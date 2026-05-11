package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type DirInput struct {
	input         textinput.Model
	active        bool
	label         string
	errMsg        string
	confirmedPath string
	focusCmd      tea.Cmd
	onConfirm     func(path string)
	onCancel      func()
}

func NewDirInput() DirInput {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 60
	ti.Placeholder = "~/Downloads"
	return DirInput{input: ti}
}

// Open activates the prompt. Callers must return FocusCmd() from their Update
// to start the cursor blink animation.
func (d *DirInput) Open(label, defaultPath string, onConfirm func(string), onCancel func()) {
	d.label = label
	d.errMsg = ""
	d.confirmedPath = ""
	d.onConfirm = onConfirm
	d.onCancel = onCancel
	d.input.SetValue(defaultPath)
	d.active = true
	d.focusCmd = d.input.Focus()
}

// FocusCmd returns the cursor-blink cmd produced by Open. Call it once and
// return it from the model's Update alongside other cmds.
func (d *DirInput) FocusCmd() tea.Cmd {
	cmd := d.focusCmd
	d.focusCmd = nil
	return cmd
}

// Consume returns the confirmed path if enter was just accepted, then clears
// it. Returns "" if no confirmation is pending.
func (d *DirInput) Consume() string {
	p := d.confirmedPath
	d.confirmedPath = ""
	return p
}

func (d *DirInput) Update(msg tea.Msg) (DirInput, tea.Cmd) {
	if !d.active {
		return *d, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		return *d, cmd
	}

	switch key.String() {
	case "enter":
		path := strings.TrimSpace(d.input.Value())
		if path == "~" {
			if home, err := os.UserHomeDir(); err == nil {
				path = home
			}
		} else if strings.HasPrefix(path, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				path = home + path[1:]
			}
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			d.errMsg = "Cannot create directory: " + err.Error()
			return *d, nil
		}
		d.active = false
		d.errMsg = ""
		d.confirmedPath = path
		if d.onConfirm != nil {
			d.onConfirm(path)
		}
		return *d, nil

	case "esc":
		d.active = false
		d.errMsg = ""
		if d.onCancel != nil {
			d.onCancel()
		}
		return *d, nil

	default:
		var cmd tea.Cmd
		d.input, cmd = d.input.Update(msg)
		return *d, cmd
	}
}

// View returns the status-bar content when active, or "" when inactive.
func (d *DirInput) View() string {
	if !d.active {
		return ""
	}
	if d.errMsg != "" {
		return redStyle.Render(d.errMsg) + "  " + d.input.View()
	}
	return dimStyle.Render(d.label) + d.input.View()
}

func (d *DirInput) Active() bool {
	return d.active
}
