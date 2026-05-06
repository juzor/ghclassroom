package tui

import (
	"fmt"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type classroomItem struct{ classroom api.Classroom }

func (i classroomItem) FilterValue() string { return i.classroom.Name }
func (i classroomItem) Title() string       { return i.classroom.Name }
func (i classroomItem) Description() string { return i.classroom.URL }

type ClassroomsPanel struct {
	list    list.Model
	loading bool
	loaded  bool
	spinner spinner.Model
	count   int
}

func newClassroomsPanel() ClassroomsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return ClassroomsPanel{list: l, spinner: s}
}

func (p *ClassroomsPanel) setSize(w, h int) {
	p.list.SetSize(w, max(0, h-2))
}

func (p *ClassroomsPanel) SetItems(classrooms []api.Classroom) {
	items := make([]list.Item, len(classrooms))
	for i, c := range classrooms {
		items[i] = classroomItem{classroom: c}
	}
	p.list.SetItems(items)
	p.count = len(classrooms)
	p.loading = false
	p.loaded = true
}

func (p ClassroomsPanel) SelectedItem() *api.Classroom {
	item, ok := p.list.SelectedItem().(classroomItem)
	if !ok {
		return nil
	}
	c := item.classroom
	return &c
}

func (p ClassroomsPanel) Update(msg tea.Msg) (ClassroomsPanel, tea.Cmd) {
	var cmd tea.Cmd
	if p.loading {
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	}
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p ClassroomsPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)
	availH := max(0, innerH-2)

	if p.loading {
		hdr := renderPanelHeader("Classrooms", "…", active, inner)
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View() + " Loading classrooms…\n" + dimStyle.Render("GET /classrooms"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	hdr := renderPanelHeader("Classrooms", fmt.Sprintf("%d", p.count), active, inner)

	if p.loaded && p.count == 0 {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No classrooms found.\n" + dimStyle.Render("Check token scopes (repo, read:org)."))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + p.list.View())
}
