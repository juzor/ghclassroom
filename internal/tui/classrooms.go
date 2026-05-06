package tui

import (
	"ghclassroom/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type ClassroomsPanel struct {
	loading bool
	items   []api.Classroom
	cursor  int
	width   int
	height  int
}

func newClassroomsPanel() ClassroomsPanel {
	return ClassroomsPanel{}
}

func (p *ClassroomsPanel) setSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ClassroomsPanel) SetItems(items []api.Classroom) {
	p.items = items
	p.loading = false
	p.cursor = 0
}

func (p ClassroomsPanel) SelectedItem() *api.Classroom {
	if len(p.items) == 0 || p.cursor >= len(p.items) {
		return nil
	}
	return &p.items[p.cursor]
}

func (p ClassroomsPanel) Update(msg tea.Msg) (ClassroomsPanel, tea.Cmd) {
	return p, nil
}

func (p ClassroomsPanel) View(width int, active bool) string {
	if p.loading {
		return "Loading…"
	}
	return ""
}
