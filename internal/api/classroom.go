package api

import (
	"fmt"
	"net/http"
)

const classroomBase = "https://api.github.com"

func GetClassrooms(token string) ([]Classroom, error) {
	var out []Classroom
	err := doRequest(token, http.MethodGet, classroomBase+"/classrooms", &out)
	return out, err
}

func GetAssignments(token string, classroomID int) ([]Assignment, error) {
	var out []Assignment
	url := fmt.Sprintf("%s/classrooms/%d/assignments", classroomBase, classroomID)
	err := doRequest(token, http.MethodGet, url, &out)
	return out, err
}

func GetAcceptedAssignments(token string, assignmentID int) ([]AcceptedAssignment, error) {
	var all []AcceptedAssignment
	const perPage = 100
	for pageNum := 1; ; pageNum++ {
		var results []AcceptedAssignment
		url := fmt.Sprintf("%s/assignments/%d/accepted_assignments?page=%d&per_page=%d",
			classroomBase, assignmentID, pageNum, perPage)
		if err := doRequest(token, http.MethodGet, url, &results); err != nil {
			return nil, err
		}
		all = append(all, results...)
		if len(results) < perPage {
			break
		}
	}
	return all, nil
}

func ReportURL(classroomID, assignmentID int) string {
	return fmt.Sprintf("https://classroom.github.com/classrooms/%d/assignments/%d",
		classroomID, assignmentID)
}
