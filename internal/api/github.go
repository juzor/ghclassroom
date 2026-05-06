package api

import (
	"fmt"
)

const githubBase = "https://api.github.com"

func GetCommits(token, fullName string) ([]Commit, error) {
	var commits []Commit
	url := fmt.Sprintf("%s/repos/%s/commits?per_page=10", githubBase, fullName)
	err := get(token, url, &commits)
	return commits, err
}

func GetBranches(token, fullName string) ([]Branch, error) {
	var branches []Branch
	url := fmt.Sprintf("%s/repos/%s/branches", githubBase, fullName)
	err := get(token, url, &branches)
	return branches, err
}

func GetPullRequests(token, fullName string) ([]PullRequest, error) {
	var prs []PullRequest
	url := fmt.Sprintf("%s/repos/%s/pulls?state=all&per_page=10", githubBase, fullName)
	err := get(token, url, &prs)
	return prs, err
}
