package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const classroomBase = "https://classroom.github.com/api/v1"

func GetClassrooms(token string) ([]Classroom, error) {
	var classrooms []Classroom
	err := get(token, classroomBase+"/classrooms", &classrooms)
	return classrooms, err
}

func GetAssignments(token string, classroomID int) ([]Assignment, error) {
	var assignments []Assignment
	url := fmt.Sprintf("%s/classrooms/%d/assignments", classroomBase, classroomID)
	err := get(token, url, &assignments)
	return assignments, err
}

func GetAcceptedAssignments(token string, assignmentID int) ([]AcceptedAssignment, error) {
	var all []AcceptedAssignment
	page := 1
	const perPage = 100
	for {
		var page_results []AcceptedAssignment
		url := fmt.Sprintf("%s/assignments/%d/accepted_assignments?page=%d&per_page=%d",
			classroomBase, assignmentID, page, perPage)
		if err := get(token, url, &page_results); err != nil {
			return nil, err
		}
		all = append(all, page_results...)
		if len(page_results) < perPage {
			break
		}
		page++
	}
	return all, nil
}

func get(token, url string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
		reset := resp.Header.Get("X-RateLimit-Reset")
		return fmt.Errorf("rate limit reached. Resets at %s", reset)
	}
	if resp.StatusCode != http.StatusOK {
		var body [512]byte
		n, _ := resp.Body.Read(body[:])
		return fmt.Errorf("GitHub API %d: %s", resp.StatusCode, body[:n])
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
