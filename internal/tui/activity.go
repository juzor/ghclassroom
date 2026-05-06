package tui

import (
	"ghclassroom/internal/api"
	tea "github.com/charmbracelet/bubbletea"
)

type ActivityPanel struct {
	loading  bool
	activity *api.RepoActivity
	width    int
	height   int
}

func newActivityPanel() ActivityPanel {
	return ActivityPanel{}
}

func (p *ActivityPanel) setSize(w, h int) {
	p.width = w
	p.height = h
}

func (p *ActivityPanel) SetActivity(activity *api.RepoActivity) {
	p.activity = activity
	p.loading = false
}

func (p ActivityPanel) Update(msg tea.Msg) (ActivityPanel, tea.Cmd) {
	return p, nil
}

func (p ActivityPanel) View(width int, active bool) string {
	if p.loading {
		return "Loading…"
	}
	return ""
}
