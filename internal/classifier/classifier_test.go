package classifier

import (
	"fmt"
	"testing"
	"time"

	"ghclassroom/internal/api"
)

func makeStudent(login, repo string) api.AcceptedAssignment {
	return api.AcceptedAssignment{
		Students:   []struct{ Login string `json:"login"` }{{Login: login}},
		Repository: struct {
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		}{FullName: repo},
	}
}

func makeCommit(date string) api.Commit {
	c := api.Commit{}
	c.Commit.Author.Date = date
	return c
}

func rfc3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func TestClassify(t *testing.T) {
	threshold := 7 // days

	recentDate := rfc3339(time.Now().Add(-2 * 24 * time.Hour))   // 2 days ago, within threshold
	oldDate := rfc3339(time.Now().Add(-30 * 24 * time.Hour))     // 30 days ago, beyond threshold

	tests := []struct {
		name           string
		students       []api.AcceptedAssignment
		activities     map[string]*api.RepoActivity
		wantLen        int
		wantLogin      string
		wantSignals    []Signal
		wantNeedsAttn  bool
		wantCommits    int
	}{
		{
			name:     "zero commits → SignalNoCommits",
			students: []api.AcceptedAssignment{makeStudent("alice", "org/repo-alice")},
			activities: map[string]*api.RepoActivity{
				"org/repo-alice": {Commits: []api.Commit{}},
			},
			wantLen:       1,
			wantLogin:     "alice",
			wantSignals:   []Signal{SignalNoCommits},
			wantNeedsAttn: true,
			wantCommits:   0,
		},
		{
			name:     "commits within threshold → no signals",
			students: []api.AcceptedAssignment{makeStudent("bob", "org/repo-bob")},
			activities: map[string]*api.RepoActivity{
				"org/repo-bob": {Commits: []api.Commit{
					makeCommit(recentDate),
					makeCommit(recentDate),
				}},
			},
			wantLen:       1,
			wantLogin:     "bob",
			wantSignals:   nil,
			wantNeedsAttn: false,
			wantCommits:   2,
		},
		{
			name:     "single commit older than threshold → SignalSingleCommit",
			students: []api.AcceptedAssignment{makeStudent("carol", "org/repo-carol")},
			activities: map[string]*api.RepoActivity{
				"org/repo-carol": {Commits: []api.Commit{makeCommit(oldDate)}},
			},
			wantLen:       1,
			wantLogin:     "carol",
			wantSignals:   []Signal{SignalSingleCommit},
			wantNeedsAttn: true,
			wantCommits:   1,
		},
		{
			name:     "multiple commits, last older than threshold → SignalInactive",
			students: []api.AcceptedAssignment{makeStudent("dan", "org/repo-dan")},
			activities: map[string]*api.RepoActivity{
				"org/repo-dan": {Commits: []api.Commit{
					makeCommit(oldDate),
					makeCommit(oldDate),
				}},
			},
			wantLen:       1,
			wantLogin:     "dan",
			wantSignals:   []Signal{SignalInactive},
			wantNeedsAttn: true,
			wantCommits:   2,
		},
		{
			name:     "nil activity entry → student skipped",
			students: []api.AcceptedAssignment{makeStudent("eve", "org/repo-eve")},
			activities: map[string]*api.RepoActivity{
				"org/repo-eve": nil,
			},
			wantLen: 0,
		},
		{
			name:     "missing activity entry → student skipped",
			students: []api.AcceptedAssignment{makeStudent("frank", "org/repo-frank")},
			activities: map[string]*api.RepoActivity{},
			wantLen:  0,
		},
		{
			name:     "unparseable commit date → zero LastCommit → SignalNeverPushed",
			students: []api.AcceptedAssignment{makeStudent("grace", "org/repo-grace")},
			activities: map[string]*api.RepoActivity{
				"org/repo-grace": {Commits: []api.Commit{makeCommit("not-a-date")}},
			},
			wantLen:       1,
			wantLogin:     "grace",
			wantSignals:   []Signal{SignalNeverPushed},
			wantNeedsAttn: true,
			wantCommits:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.students, tc.activities, threshold)

			if len(got) != tc.wantLen {
				t.Fatalf("len(results) = %d, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}

			s := got[0]
			if s.Login != tc.wantLogin {
				t.Errorf("Login = %q, want %q", s.Login, tc.wantLogin)
			}
			if s.CommitCount != tc.wantCommits {
				t.Errorf("CommitCount = %d, want %d", s.CommitCount, tc.wantCommits)
			}
			if s.NeedsAttention != tc.wantNeedsAttn {
				t.Errorf("NeedsAttention = %v, want %v", s.NeedsAttention, tc.wantNeedsAttn)
			}
			if len(s.Signals) != len(tc.wantSignals) {
				t.Fatalf("Signals = %v, want %v", s.Signals, tc.wantSignals)
			}
			for i, sig := range tc.wantSignals {
				if s.Signals[i] != sig {
					t.Errorf("Signals[%d] = %v, want %v", i, s.Signals[i], sig)
				}
			}
		})
	}
}

func TestSignalString(t *testing.T) {
	cases := []struct {
		s    Signal
		want string
	}{
		{SignalNoCommits, "no commits"},
		{SignalInactive, "inactive"},
		{SignalSingleCommit, "single commit, no recent activity"},
		{SignalNeverPushed, "accepted but never pushed"},
		{Signal(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Signal(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

func TestClassifyMultipleStudents(t *testing.T) {
	threshold := 7
	oldDate := rfc3339(time.Now().Add(-30 * 24 * time.Hour))
	recentDate := rfc3339(time.Now().Add(-1 * 24 * time.Hour))

	students := []api.AcceptedAssignment{
		makeStudent("a", "org/a"),
		makeStudent("b", "org/b"), // nil → skipped
		makeStudent("c", "org/c"),
	}
	activities := map[string]*api.RepoActivity{
		"org/a": {Commits: []api.Commit{makeCommit(oldDate), makeCommit(oldDate)}},
		"org/b": nil,
		"org/c": {Commits: []api.Commit{makeCommit(recentDate)}},
	}

	got := Classify(students, activities, threshold)

	if len(got) != 2 {
		t.Fatalf("expected 2 results (b skipped), got %d", len(got))
	}

	byLogin := make(map[string]StudentStatus)
	for _, s := range got {
		byLogin[s.Login] = s
	}

	if sa, ok := byLogin["a"]; !ok {
		t.Error("missing student a")
	} else if !sa.NeedsAttention || len(sa.Signals) == 0 || sa.Signals[0] != SignalInactive {
		t.Errorf("student a: want SignalInactive, got %v", sa.Signals)
	}

	if sc, ok := byLogin["c"]; !ok {
		t.Error("missing student c")
	} else if sc.NeedsAttention {
		t.Errorf("student c: want no attention needed, got signals %v", sc.Signals)
	}

	if _, ok := byLogin["b"]; ok {
		t.Error("student b (nil activity) should be skipped")
	}

	_ = fmt.Sprintf // suppress unused import if any
}
