package tui

import (
	"strings"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// studentItem adapts api.AcceptedAssignment to bubbles/list.Item.
type studentItem struct{ assignment api.AcceptedAssignment }

func (i studentItem) FilterValue() string { return i.Title() }

func (i studentItem) Title() string {
	marker := "·"
	if i.assignment.Submitted {
		marker = "✓"
	}
	if len(i.assignment.Students) == 0 {
		return marker + " (no student)"
	}
	logins := make([]string, len(i.assignment.Students))
	for j, s := range i.assignment.Students {
		logins[j] = s.Login
	}
	return marker + " " + strings.Join(logins, ", ")
}

func (i studentItem) Description() string { return i.assignment.Repository.FullName }

type StudentsPanel struct {
	list         list.Model
	loading      bool
	spinner      spinner.Model
	assignmentID int
}

func newStudentsPanel() StudentsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Students"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return StudentsPanel{list: l, spinner: s}
}

func (p *StudentsPanel) setSize(w, h int) {
	p.list.SetSize(w, h)
}

func (p *StudentsPanel) SetItems(assignmentID int, students []api.AcceptedAssignment) {
	p.assignmentID = assignmentID
	items := make([]list.Item, len(students))
	for i, s := range students {
		items[i] = studentItem{assignment: s}
	}
	p.list.SetItems(items)
	p.loading = false
}

func (p StudentsPanel) SelectedItem() *api.AcceptedAssignment {
	item, ok := p.list.SelectedItem().(studentItem)
	if !ok {
		return nil
	}
	a := item.assignment
	return &a
}

func (p StudentsPanel) Update(msg tea.Msg) (StudentsPanel, tea.Cmd) {
	var cmd tea.Cmd
	if p.loading {
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	}
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p StudentsPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)

	if p.loading {
		content := lipgloss.NewStyle().
			Width(inner).Height(innerH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View())
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	if len(p.list.Items()) == 0 {
		content := lipgloss.NewStyle().
			Width(inner).Height(innerH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No accepted assignments yet.")
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(p.list.View())
}
