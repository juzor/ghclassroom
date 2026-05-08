package tui

import (
	"fmt"
	"strings"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ContentPanel struct {
	viewport       viewport.Model
	spinner        spinner.Model
	loading        bool
	hasContent     bool // any content (preview or activity)
	activityLoaded bool // full activity (not just preview)
	errMsg         string
	repoURL        string
	reportURL      string
	kind           nodeKind
	width          int
	height         int
}

func newContentPanel() ContentPanel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return ContentPanel{
		viewport: viewport.New(0, 0),
		spinner:  s,
	}
}

func (p *ContentPanel) setSize(w, h int) {
	p.width = w
	p.height = h
	inner := max(0, w-2)
	innerH := max(0, h-2)
	availH := max(0, innerH-2)
	p.viewport.Width = inner
	p.viewport.Height = availH
}

func (p *ContentPanel) ShowClassroom(c api.Classroom, assignmentCount int, loaded bool) {
	p.loading = false
	p.hasContent = true
	p.activityLoaded = false
	p.errMsg = ""
	p.repoURL = c.URL
	p.reportURL = ""
	p.kind = nodeClassroom

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n\n",
		dimStyle.Render("Classroom"), amberStyle.Bold(true).Render(c.Name))
	fmt.Fprintf(&b, "%s  %s\n\n",
		dimStyle.Render("URL      "), dimStyle.Render(c.URL))
	if loaded {
		fmt.Fprintf(&b, "%s  %d\n\n",
			dimStyle.Render("Assignments"), assignmentCount)
	}
	fmt.Fprintf(&b, "%s\n",
		dimStyle.Render("Press ↵ or → to expand assignments in the tree."))
	fmt.Fprintf(&b, "%s\n",
		dimStyle.Render("Press o to open the classroom in your browser."))
	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()
}

func (p *ContentPanel) ShowAssignment(a api.Assignment, classroomID int, totalStudents, submitted int, studentsLoaded bool) {
	p.loading = false
	p.hasContent = true
	p.activityLoaded = false
	p.errMsg = ""
	p.repoURL = ""
	p.reportURL = api.ReportURL(classroomID, a.ID)
	p.kind = nodeAssignment

	deadline := a.Deadline
	if deadline == "" {
		deadline = "—"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n",
		dimStyle.Render("Title     "), amberStyle.Bold(true).Render(a.Title))
	fmt.Fprintf(&b, "%s  %s\n",
		dimStyle.Render("Type      "), a.Type)
	fmt.Fprintf(&b, "%s  %s\n",
		dimStyle.Render("Deadline  "), deadline)
	if studentsLoaded {
		fmt.Fprintf(&b, "%s  %d total  %s\n",
			dimStyle.Render("Students  "), totalStudents,
			dimStyle.Render(fmt.Sprintf("· %d submitted", submitted)))
	}
	fmt.Fprintf(&b, "\n%s  %s\n\n",
		dimStyle.Render("Report URL"), amberStyle.Render(p.reportURL))
	if !studentsLoaded {
		fmt.Fprintf(&b, "%s\n",
			dimStyle.Render("Press ↵ to expand students in the tree."))
	}
	fmt.Fprintf(&b, "%s\n",
		dimStyle.Render("Press c to copy the report URL."))
	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()
}

func (p *ContentPanel) ShowStudentPreview(s api.AcceptedAssignment) {
	p.loading = false
	p.hasContent = true
	p.activityLoaded = false
	p.errMsg = ""
	p.repoURL = s.Repository.HTMLURL
	p.reportURL = ""
	p.kind = nodeStudent

	logins := make([]string, len(s.Students))
	for i, st := range s.Students {
		logins[i] = st.Login
	}
	student := strings.Join(logins, ", ")
	if student == "" {
		student = "(no student)"
	}

	submittedStr := dimStyle.Render("· not yet")
	if s.Submitted {
		submittedStr = cyanStyle.Render("✓ yes")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n",
		dimStyle.Render("Student   "), amberStyle.Bold(true).Render(student))
	fmt.Fprintf(&b, "%s  %s\n",
		dimStyle.Render("Repo      "), s.Repository.FullName)
	fmt.Fprintf(&b, "%s  %s\n\n",
		dimStyle.Render("Submitted "), submittedStr)
	fmt.Fprintf(&b, "%s\n",
		dimStyle.Render("Press ↵ to load commits, branches, and pull requests."))
	fmt.Fprintf(&b, "%s\n",
		dimStyle.Render("Press o to open the repo in your browser."))
	p.viewport.SetContent(b.String())
	p.viewport.GotoTop()
}

func (p *ContentPanel) ShowActivity(activity *api.RepoActivity, repoFullName, repoURL string) {
	p.loading = false
	p.hasContent = true
	p.activityLoaded = true
	p.errMsg = ""
	p.repoURL = repoURL
	p.reportURL = ""
	p.kind = nodeStudent
	p.viewport.SetContent(renderActivity(activity, repoFullName))
	p.viewport.GotoTop()
}

func (p *ContentPanel) StartLoadingActivity(repoURL string) {
	p.loading = true
	p.hasContent = true
	p.activityLoaded = false
	p.errMsg = ""
	p.repoURL = repoURL
	p.reportURL = ""
	p.kind = nodeStudent
}

func (p *ContentPanel) SetError(msg string) {
	p.loading = false
	p.errMsg = msg
}

func (p ContentPanel) RepoURL() string    { return p.repoURL }
func (p ContentPanel) ReportURL() string  { return p.reportURL }
func (p ContentPanel) IsActivityLoaded() bool { return p.activityLoaded }

func (p ContentPanel) Update(msg tea.Msg) (ContentPanel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	p.spinner, cmd = p.spinner.Update(msg)
	cmds = append(cmds, cmd)
	if p.hasContent && p.errMsg == "" && !p.loading {
		p.viewport, cmd = p.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	return p, tea.Batch(cmds...)
}

func (p ContentPanel) headerName() string {
	if p.activityLoaded {
		return "Activity"
	}
	return "Preview"
}

func (p ContentPanel) countStr() string {
	switch {
	case p.loading:
		return "…"
	case p.errMsg != "":
		return "!"
	case !p.hasContent:
		return "—"
	case p.activityLoaded:
		return "live"
	case p.kind == nodeStudent:
		return "repo"
	case p.kind == nodeAssignment:
		return "assignment"
	case p.kind == nodeClassroom:
		return "classroom"
	}
	return "—"
}

func (p ContentPanel) View(active bool) string {
	w := p.width
	h := p.height
	inner := max(0, w-2)
	innerH := max(0, h-2)
	availH := max(0, innerH-2)

	hdr := renderPanelHeader(p.headerName(), p.countStr(), active, inner)

	if p.loading {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View() + " Loading activity…\n" +
				dimStyle.Render("GET /commits · /branches · /pulls"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if p.errMsg != "" {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Could not load activity:\n" + p.errMsg)
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if !p.hasContent {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(dimStyle.Render("nothing selected yet"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + p.viewport.View())
}
