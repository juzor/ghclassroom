package tui

import (
	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// assignmentItem adapts api.Assignment to bubbles/list.Item.
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
	spinner     spinner.Model
	classroomID int
	selectedURL string // report URL for the highlighted assignment
}

func newAssignmentsPanel() AssignmentsPanel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Assignments"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	// Filtering enabled by default — do not disable it (spec core UX feature)

	s := spinner.New()
	s.Spinner = spinner.Dot

	return AssignmentsPanel{list: l, spinner: s}
}

func (p *AssignmentsPanel) setSize(w, h int) {
	// Reserve 1 row below the list for the report URL status line.
	p.list.SetSize(w, max(0, h-1))
}

func (p *AssignmentsPanel) SetItems(classroomID int, assignments []api.Assignment) {
	p.classroomID = classroomID
	items := make([]list.Item, len(assignments))
	for i, a := range assignments {
		items[i] = assignmentItem{assignment: a}
	}
	p.list.SetItems(items)
	p.loading = false
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

// SelectedReportURL returns the report URL for the currently highlighted assignment.
func (p AssignmentsPanel) SelectedReportURL() string {
	return p.selectedURL
}

// IsFiltering reports whether the built-in fuzzy filter is active.
// Used by the root model to decide whether to pass key events through.
func (p AssignmentsPanel) IsFiltering() bool {
	return p.list.FilterState() == list.Filtering
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
			Render("No assignments in this classroom.")
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	urlLine := dimStyle.Width(inner).Render("Report URL: " + p.selectedURL)
	content := p.list.View() + "\n" + urlLine
	return panelStyle(active).Width(inner).Height(innerH).Render(content)
}
