package api

import (
	"fmt"
	"net/http"
)

const githubBase = "https://api.github.com"

// GetAllCommits fetches every commit for the repository by following GitHub's
// pagination Link headers, requesting 100 per page.
func GetAllCommits(token, repoFullName string) ([]Commit, error) {
	var all []Commit
	url := fmt.Sprintf("%s/repos/%s/commits?per_page=100", githubBase, repoFullName)
	for url != "" {
		var page []Commit
		next, err := doRequestWithNext(token, http.MethodGet, url, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		url = next
	}
	return all, nil
}

func GetBranches(token, repoFullName string) ([]Branch, error) {
	var out []Branch
	url := fmt.Sprintf("%s/repos/%s/branches", githubBase, repoFullName)
	err := doRequest(token, http.MethodGet, url, &out)
	return out, err
}

func GetPullRequests(token, repoFullName string) ([]PullRequest, error) {
	var out []PullRequest
	url := fmt.Sprintf("%s/repos/%s/pulls?state=all&per_page=10", githubBase, repoFullName)
	err := doRequest(token, http.MethodGet, url, &out)
	return out, err
}

func GetRepoActivity(token, repoFullName string) (*RepoActivity, error) {
	commits, err := GetAllCommits(token, repoFullName)
	if err != nil {
		return nil, err
	}
	branches, err := GetBranches(token, repoFullName)
	if err != nil {
		return nil, err
	}
	prs, err := GetPullRequests(token, repoFullName)
	if err != nil {
		return nil, err
	}
	return &RepoActivity{
		Commits:      commits,
		Branches:     branches,
		PullRequests: prs,
	}, nil
}
