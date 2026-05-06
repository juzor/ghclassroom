package api

import "testing"

func TestReportURL(t *testing.T) {
	got := ReportURL(42, 7)
	want := "https://classroom.github.com/classrooms/42/assignments/7"
	if got != want {
		t.Errorf("ReportURL(42, 7) = %q, want %q", got, want)
	}
}
