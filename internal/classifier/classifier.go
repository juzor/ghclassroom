package classifier

import (
	"time"

	"ghclassroom/internal/api"
)

type Signal int

const (
	SignalNoCommits    Signal = iota
	SignalInactive
	SignalSingleCommit
	SignalNeverPushed
)

func (s Signal) String() string {
	switch s {
	case SignalNoCommits:
		return "no commits"
	case SignalInactive:
		return "inactive"
	case SignalSingleCommit:
		return "single commit, no recent activity"
	case SignalNeverPushed:
		return "accepted but never pushed"
	default:
		return "unknown"
	}
}

type StudentStatus struct {
	Login        string
	RepoFullName string
	Signals      []Signal
	LastCommit   time.Time
	CommitCount  int
	NeedsAttention bool
}

func Classify(
	students []api.AcceptedAssignment,
	activities map[string]*api.RepoActivity,
	thresholdDays int,
) []StudentStatus {
	threshold := time.Duration(thresholdDays) * 24 * time.Hour
	var results []StudentStatus

	for _, s := range students {
		repo := s.Repository.FullName
		activity, ok := activities[repo]
		if !ok || activity == nil {
			continue
		}

		login := ""
		if len(s.Students) > 0 {
			login = s.Students[0].Login
		}

		commitCount := len(activity.Commits)
		var lastCommit time.Time
		if commitCount > 0 {
			t, err := time.Parse(time.RFC3339, activity.Commits[0].Commit.Author.Date)
			if err == nil {
				lastCommit = t
			}
		}

		var signals []Signal

		if commitCount == 0 {
			signals = append(signals, SignalNoCommits)
		}

		if commitCount > 0 && lastCommit.IsZero() {
			signals = append(signals, SignalNeverPushed)
		}

		if commitCount == 1 && !lastCommit.IsZero() && time.Since(lastCommit) > threshold {
			signals = append(signals, SignalSingleCommit)
		}

		if commitCount > 1 && !lastCommit.IsZero() && time.Since(lastCommit) > threshold {
			signals = append(signals, SignalInactive)
		}

		results = append(results, StudentStatus{
			Login:          login,
			RepoFullName:   repo,
			Signals:        signals,
			LastCommit:     lastCommit,
			CommitCount:    commitCount,
			NeedsAttention: len(signals) > 0,
		})
	}

	return results
}
