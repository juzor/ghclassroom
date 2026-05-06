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
	loaded   bool
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
	p.viewport.Height = max(0, h-2)
}

func (p *ActivityPanel) startLoading() {
	p.loading = true
	p.errMsg = ""
}

func (p *ActivityPanel) SetActivity(activity *api.RepoActivity, repoFullName, repoURL string) {
	p.repoURL = repoURL
	p.errMsg = ""
	p.loaded = true
	p.viewport.SetContent(renderActivity(activity, repoFullName))
	p.viewport.GotoTop()
	p.loading = false
}

func (p *ActivityPanel) SetError(msg string) {
	p.errMsg = msg
	p.repoURL = ""
	p.loading = false
}

func (p ActivityPanel) RepoURL() string { return p.repoURL }

func (p ActivityPanel) Update(msg tea.Msg) (ActivityPanel, tea.Cmd) {
	var cmd tea.Cmd
	if p.loading {
		p.spinner, cmd = p.spinner.Update(msg)
		return p, cmd
	}
	if p.loaded && p.errMsg == "" {
		p.viewport, cmd = p.viewport.Update(msg)
	}
	return p, cmd
}

func (p ActivityPanel) countStr() string {
	if p.loading {
		return "…"
	}
	if p.errMsg != "" {
		return "!"
	}
	if !p.loaded {
		return "—"
	}
	return "live"
}

func (p ActivityPanel) View(active bool, width, height int) string {
	inner := max(0, width-2)
	innerH := max(0, height-2)
	hdr := renderPanelHeader("Activity", p.countStr(), active, inner)
	availH := max(0, innerH-2)

	if p.loading {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(p.spinner.View() + " Loading activity…\n" +
				dimStyle.Render("GET /commits, /branches, /pulls"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if p.errMsg != "" {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Could not load activity:\n" + p.errMsg)
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	if !p.loaded {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(dimStyle.Render("—"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + p.viewport.View())
}

func renderActivity(a *api.RepoActivity, repoFullName string) string {
	if a == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "%s  %s\n", dimStyle.Render("Repo     "), repoFullName)
	if len(a.Commits) > 0 {
		lastDate := formatDate(a.Commits[0].Commit.Author.Date)
		rel := relativeTime(a.Commits[0].Commit.Author.Date)
		fmt.Fprintf(&b, "%s  %s %s\n", dimStyle.Render("Last push"), lastDate, dimStyle.Render("· "+rel))
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, amberStyle.Render(fmt.Sprintf("COMMITS (%d)", len(a.Commits))))
	fmt.Fprintln(&b, dimStyle.Render(strings.Repeat("╌", 60)))
	if len(a.Commits) == 0 {
		fmt.Fprintln(&b, dimStyle.Render("  No commits yet."))
	}
	for _, c := range a.Commits {
		sha := c.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n",
			cyanStyle.Render(sha),
			dimStyle.Render(shortDate(c.Commit.Author.Date)),
			truncate(firstLineOf(c.Commit.Message), 50))
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, amberStyle.Render(fmt.Sprintf("BRANCHES (%d)", len(a.Branches))))
	fmt.Fprintln(&b, dimStyle.Render(strings.Repeat("╌", 60)))
	if len(a.Branches) > 0 {
		names := make([]string, len(a.Branches))
		for i, br := range a.Branches {
			names[i] = greenStyle.Render(br.Name)
		}
		fmt.Fprintf(&b, "  %s\n", strings.Join(names, dimStyle.Render(", ")))
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, amberStyle.Render(fmt.Sprintf("PULL REQUESTS (%d)", len(a.PullRequests))))
	fmt.Fprintln(&b, dimStyle.Render(strings.Repeat("╌", 60)))
	for _, pr := range a.PullRequests {
		var stateStr string
		switch pr.State {
		case "open":
			stateStr = greenStyle.Render("open  ")
		case "closed":
			stateStr = redStyle.Render("closed")
		case "merged":
			stateStr = purpleStyle.Render("merged")
		default:
			stateStr = dimStyle.Render(pr.State)
		}
		fmt.Fprintf(&b, "  %s  %s  %s\n",
			blueStyle.Render(fmt.Sprintf("#%-3d", pr.Number)),
			stateStr,
			truncate(pr.Title, 50))
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

func shortDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format("01-02")
}

func relativeTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
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
