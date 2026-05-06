package tui

import (
	"fmt"
	"strings"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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
	list           list.Model
	loading        bool
	loaded         bool
	spinner        spinner.Model
	assignmentID   int
	totalCount     int
	submittedCount int
}

func newStudentsPanel() StudentsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return StudentsPanel{list: l, spinner: s}
}

func (p *StudentsPanel) setSize(w, h int) {
	p.list.SetSize(w, max(0, h-2))
}

func (p *StudentsPanel) SetItems(assignmentID int, students []api.AcceptedAssignment) {
	p.assignmentID = assignmentID
	p.totalCount = len(students)
	p.submittedCount = 0
	items := make([]list.Item, len(students))
	for i, s := range students {
		if s.Submitted {
			p.submittedCount++
		}
		items[i] = studentItem{assignment: s}
	}
	p.list.SetItems(items)
	p.loading = false
	p.loaded = true
}

func (p StudentsPanel) SelectedItem() *api.AcceptedAssignment {
	item, ok := p.list.SelectedItem().(studentItem)
	if !ok {
		return nil
	}
	a := item.assignment
	return &a
}

func (p StudentsPanel) countStr() string {
	if p.loading {
		return "…"
	}
	if !p.loaded {
		return "—"
	}
	if p.submittedCount < p.totalCount {
		return fmt.Sprintf("%d · %d submitted", p.totalCount, p.submittedCount)
	}
	return fmt.Sprintf("%d", p.totalCount)
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
	availH := max(0, innerH-2)

	hdr := renderPanelHeader("Students", p.countStr(), active, inner)

	if p.loading {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View() + " Loading students…\n" + dimStyle.Render("GET /assignments/{id}/accepted_assignments"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if !p.loaded {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(dimStyle.Render("—"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if p.totalCount == 0 {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No accepted assignments yet.")
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + p.list.View())
}
