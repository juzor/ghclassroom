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

func (p ActivityPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)
	content := ""
	if p.loading {
		content = "Loading…"
	}
	return panelStyle(active).Width(inner).Height(innerH).Render(content)
}
