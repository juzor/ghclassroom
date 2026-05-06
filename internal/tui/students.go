package tui

import (
	"ghclassroom/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type StudentsPanel struct {
	loading bool
	items   []api.AcceptedAssignment
	cursor  int
	width   int
	height  int
}

func newStudentsPanel() StudentsPanel {
	return StudentsPanel{}
}

func (p *StudentsPanel) setSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *StudentsPanel) SetItems(items []api.AcceptedAssignment) {
	p.items = items
	p.loading = false
	p.cursor = 0
}

func (p StudentsPanel) SelectedItem() *api.AcceptedAssignment {
	if len(p.items) == 0 || p.cursor >= len(p.items) {
		return nil
	}
	return &p.items[p.cursor]
}

func (p StudentsPanel) Update(msg tea.Msg) (StudentsPanel, tea.Cmd) {
	return p, nil
}

func (p StudentsPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)
	content := ""
	if p.loading {
		content = "Loading…"
	}
	return panelStyle(active).Width(inner).Height(innerH).Render(content)
}
