package api

import (
	"errors"
	"time"
)

type RateLimitError struct {
	ResetAt time.Time
}

func (e *RateLimitError) Error() string {
	return "rate limit reached, resets at " + e.ResetAt.UTC().Format("2006-01-02 15:04 UTC")
}

func AsRateLimitError(err error) (*RateLimitError, bool) {
	var rle *RateLimitError
	return rle, errors.As(err, &rle)
}

type Classroom struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Assignment struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Deadline string `json:"deadline"`
}

type AcceptedAssignment struct {
	ID        int `json:"id"`
	Submitted bool `json:"submitted"`
	Students  []struct {
		Login string `json:"login"`
	} `json:"students"`
	Repository struct {
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
	} `json:"repository"`
}

type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type Branch struct {
	Name string `json:"name"`
}

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

type RepoActivity struct {
	Commits      []Commit
	Branches     []Branch
	PullRequests []PullRequest
}
