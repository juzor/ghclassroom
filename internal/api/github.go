package api

import (
	"fmt"
	"net/http"
)

const githubBase = "https://api.github.com"

func GetRecentCommits(token, repoFullName string) ([]Commit, error) {
	var out []Commit
	url := fmt.Sprintf("%s/repos/%s/commits?per_page=10", githubBase, repoFullName)
	err := doRequest(token, http.MethodGet, url, &out)
	return out, err
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
	commits, err := GetRecentCommits(token, repoFullName)
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
