package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type DirInput struct {
	input     textinput.Model
	active    bool
	label     string
	errMsg    string
	onConfirm func(path string)
	onCancel  func()
}

func NewDirInput() DirInput {
	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 60
	ti.Placeholder = "~/Downloads"
	return DirInput{input: ti}
}

// Open activates the prompt with the given label, pre-filled path, and callbacks.
func (d *DirInput) Open(label, defaultPath string, onConfirm func(string), onCancel func()) {
	d.label = label
	d.errMsg = ""
	d.onConfirm = onConfirm
	d.onCancel = onCancel
	d.input.SetValue(defaultPath)
	d.active = true
	d.input.Focus()
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
