package tui

import (
	"fmt"
	"strings"
	"time"

	"ghclassroom/internal/api"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ActivityPanel struct {
	viewport viewport.Model
	loading  bool
	spinner  spinner.Model
	repoURL  string
	errMsg   string
}

func newActivityPanel() ActivityPanel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return ActivityPanel{
		viewport: viewport.New(0, 0),
		spinner:  s,
	}
}

func (p *ActivityPanel) setSize(w, h int) {
	p.viewport.Width = w
	p.viewport.Height = h
}

func (p *ActivityPanel) SetActivity(activity *api.RepoActivity, repoFullName, repoURL string) {
	p.repoURL = repoURL
	p.errMsg = ""
	p.viewport.SetContent(renderActivity(activity, repoFullName))
	p.viewport.GotoTop()
	p.loading = false
}

func (p *ActivityPanel) SetError(msg string) {
	p.errMsg = msg
	p.repoURL = ""
	p.loading = false
}

// RepoURL returns the HTML URL for the repo — used by the root model's o/c key handlers.
func (p ActivityPanel) RepoURL() string { return p.repoURL }

func (p ActivityPanel) Update(msg tea.Msg) (ActivityPanel, tea.Cmd) {
	var cmd tea.Cmd
	if p.loading {
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	}
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p ActivityPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)

	if p.loading {
		content := lipgloss.NewStyle().
			Width(inner).Height(innerH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View())
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	if p.errMsg != "" {
		content := lipgloss.NewStyle().
			Width(inner).Height(innerH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Could not load activity:\n" + p.errMsg)
		return panelStyle(active).Width(inner).Height(innerH).Render(content)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(p.viewport.View())
}

func renderActivity(a *api.RepoActivity, repoFullName string) string {
	if a == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "Repo:  %s\n\n", repoFullName)

	fmt.Fprintf(&b, "COMMITS (%d)\n", len(a.Commits))
	if len(a.Commits) == 0 {
		fmt.Fprintf(&b, "  No commits yet.\n")
	}
	for _, c := range a.Commits {
		sha := c.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n", sha, formatDate(c.Commit.Author.Date), truncate(firstLineOf(c.Commit.Message), 60))
	}
	b.WriteByte('\n')

	names := make([]string, len(a.Branches))
	for i, br := range a.Branches {
		names[i] = br.Name
	}
	fmt.Fprintf(&b, "BRANCHES (%d)\n", len(a.Branches))
	if len(names) > 0 {
		fmt.Fprintf(&b, "  %s\n", strings.Join(names, ", "))
	}
	b.WriteByte('\n')

	fmt.Fprintf(&b, "PULL REQUESTS (%d)\n", len(a.PullRequests))
	for _, pr := range a.PullRequests {
		fmt.Fprintf(&b, "  #%d  %s  %s\n", pr.Number, pr.State, pr.Title)
	}

	return b.String()
}

func formatDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
