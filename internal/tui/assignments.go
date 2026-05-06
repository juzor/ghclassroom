package tui

import (
	"ghclassroom/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type AssignmentsPanel struct {
	loading bool
	items   []api.Assignment
	cursor  int
	width   int
	height  int
}

func newAssignmentsPanel() AssignmentsPanel {
	return AssignmentsPanel{}
}

func (p *AssignmentsPanel) setSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *AssignmentsPanel) SetItems(items []api.Assignment) {
	p.items = items
	p.loading = false
	p.cursor = 0
}

func (p AssignmentsPanel) SelectedItem() *api.Assignment {
	if len(p.items) == 0 || p.cursor >= len(p.items) {
		return nil
	}
	return &p.items[p.cursor]
}

func (p AssignmentsPanel) Update(msg tea.Msg) (AssignmentsPanel, tea.Cmd) {
	return p, nil
}

func (p AssignmentsPanel) View(width int, active bool) string {
	if p.loading {
		return "Loading…"
	}
	return ""
}
