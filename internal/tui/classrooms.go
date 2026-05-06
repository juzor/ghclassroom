package tui

import (
	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// classroomItem adapts api.Classroom to bubbles/list.Item.
type classroomItem struct{ classroom api.Classroom }

func (i classroomItem) FilterValue() string { return i.classroom.Name }
func (i classroomItem) Title() string       { return i.classroom.Name }
func (i classroomItem) Description() string { return i.classroom.URL }

type ClassroomsPanel struct {
	list    list.Model
	loading bool
	spinner spinner.Model
}

func newClassroomsPanel() ClassroomsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Classrooms"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // spec: "No search needed — classroom lists are short"
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return ClassroomsPanel{list: l, spinner: s}
}

func (p *ClassroomsPanel) setSize(w, h int) {
	p.list.SetSize(w, h)
}

func (p *ClassroomsPanel) SetItems(classrooms []api.Classroom) {
	items := make([]list.Item, len(classrooms))
	for i, c := range classrooms {
		items[i] = classroomItem{classroom: c}
	}
	p.list.SetItems(items)
	p.loading = false
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

	if p.loading {
		content := lipgloss.NewStyle().
			Width(inner).Height(innerH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View())
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(p.list.View())
}
