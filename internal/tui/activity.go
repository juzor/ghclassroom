package tui

import (
	"fmt"
	"strings"
	"time"

	"ghclassroom/internal/api"
	"ghclassroom/internal/classifier"
	"github.com/charmbracelet/lipgloss"
)

func renderClassifierBadge(status *classifier.StudentStatus) string {
	if status == nil || !status.NeedsAttention {
		return ""
	}
	sigStrs := make([]string, len(status.Signals))
	for i, sig := range status.Signals {
		sigStrs[i] = sig.String()
	}
	header := redStyle.Render("⚠") + "  " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("NEEDS ATTENTION")
	signals := "   " + strings.Join(sigStrs, " · ")
	lines := []string{header, signals}
	if !status.LastCommit.IsZero() {
		lines = append(lines, "   Last commit: "+status.LastCommit.UTC().Format("02-01-2006"))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func renderActivity(a *api.RepoActivity, repoFullName string, status *classifier.StudentStatus) string {
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

	if badge := renderClassifierBadge(status); badge != "" {
		fmt.Fprintln(&b, badge)
		fmt.Fprintln(&b)
	}

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
	return t.UTC().Format("02-01-2006 15:04 UTC")
}

func shortDate(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.UTC().Format("02-01-06")
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
