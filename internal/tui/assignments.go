package tui

import (
	"fmt"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type assignmentItem struct{ assignment api.Assignment }

func (i assignmentItem) FilterValue() string { return i.assignment.Title }
func (i assignmentItem) Title() string       { return i.assignment.Title }
func (i assignmentItem) Description() string {
	t := i.assignment.Type
	if i.assignment.Deadline == "" {
		return t + "  " + dimStyle.Render("no deadline")
	}
	return t + "  " + dimStyle.Render(i.assignment.Deadline)
}

type AssignmentsPanel struct {
	list        list.Model
	loading     bool
	loaded      bool
	spinner     spinner.Model
	classroomID int
	totalCount  int
	selectedURL string
}

func newAssignmentsPanel() AssignmentsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return AssignmentsPanel{list: l, spinner: s}
}

func (p *AssignmentsPanel) setSize(w, h int) {
	p.list.SetSize(w, max(0, h-3))
}

func (p *AssignmentsPanel) SetItems(classroomID int, assignments []api.Assignment) {
	p.classroomID = classroomID
	p.totalCount = len(assignments)
	items := make([]list.Item, len(assignments))
	for i, a := range assignments {
		items[i] = assignmentItem{assignment: a}
	}
	p.list.SetItems(items)
	p.loading = false
	p.loaded = true
	p.selectedURL = p.computeSelectedURL()
}

func (p AssignmentsPanel) SelectedItem() *api.Assignment {
	item, ok := p.list.SelectedItem().(assignmentItem)
	if !ok {
		return nil
	}
	a := item.assignment
	return &a
}

func (p AssignmentsPanel) SelectedReportURL() string { return p.selectedURL }

func (p AssignmentsPanel) IsFiltering() bool {
	return p.list.FilterState() == list.Filtering
}

func (p AssignmentsPanel) countStr() string {
	if p.loading {
		return "…"
	}
	if !p.loaded {
		return "—"
	}
	fs := p.list.FilterState()
	if fs == list.Filtering || fs == list.FilterApplied {
		return fmt.Sprintf("%d of %d", len(p.list.VisibleItems()), p.totalCount)
	}
	return fmt.Sprintf("%d", p.totalCount)
}

func (p AssignmentsPanel) computeSelectedURL() string {
	item, ok := p.list.SelectedItem().(assignmentItem)
	if !ok {
		return ""
	}
	return api.ReportURL(p.classroomID, item.assignment.ID)
}

func (p AssignmentsPanel) Update(msg tea.Msg) (AssignmentsPanel, tea.Cmd) {
	var cmd tea.Cmd
	if p.loading {
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	}
	p.list, cmd = p.list.Update(msg)
	p.selectedURL = p.computeSelectedURL()
	return p, cmd
}

func (p AssignmentsPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)
	availH := max(0, innerH-2)

	hdr := renderPanelHeader("Assignments", p.countStr(), active, inner)

	if p.loading {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View() + " Loading assignments…\n" + dimStyle.Render("GET /classrooms/{id}/assignments"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if !p.loaded {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(dimStyle.Render("select a classroom →"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if p.totalCount == 0 {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No assignments in this classroom.")
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	urlLine := dimStyle.Width(inner).Render("Report: " + p.selectedURL)
	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + p.list.View() + "\n" + urlLine)
}
