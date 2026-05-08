package tui

import (
	"strings"

	"ghclassroom/internal/api"
	"ghclassroom/internal/classifier"
)

type nodeKind int

const (
	nodeClassroom  nodeKind = iota
	nodeAssignment
	nodeStudent
	nodeEmpty // placeholder leaf for empty assignment
)

type treeNode struct {
	kind              nodeKind
	classroom         api.Classroom
	assignment        api.Assignment
	student           api.AcceptedAssignment
	status            *classifier.StudentStatus
	classifierApplied bool // true once ApplyStatuses has run for this assignment node
	expanded          bool
	loading           bool
	children          []*treeNode
	parent            *treeNode
	depth             int
}

func (n *treeNode) label() string {
	switch n.kind {
	case nodeClassroom:
		return n.classroom.Name
	case nodeAssignment:
		return n.assignment.Title
	case nodeStudent:
		return studentNodeLabel(n.student)
	case nodeEmpty:
		return "no accepted assignments"
	}
	return "?"
}

func (n *treeNode) meta() string {
	switch n.kind {
	case nodeAssignment:
		if n.assignment.Deadline == "" {
			return n.assignment.Type
		}
		d := n.assignment.Deadline
		if len(d) >= 10 {
			d = d[5:10] // MM-DD from ISO date
		}
		return n.assignment.Type + " · " + d
	}
	return ""
}

func studentNodeLabel(s api.AcceptedAssignment) string {
	marker := "·"
	if s.Submitted {
		marker = "✓"
	}
	if len(s.Students) == 0 {
		return marker + " (no student)"
	}
	logins := make([]string, len(s.Students))
	for i, st := range s.Students {
		logins[i] = st.Login
	}
	return marker + " " + strings.Join(logins, ", ")
}

func collectVisible(roots []*treeNode) []*treeNode {
	var out []*treeNode
	for _, n := range roots {
		collectVisibleRec(n, &out)
	}
	return out
}

func collectVisibleRec(n *treeNode, out *[]*treeNode) {
	*out = append(*out, n)
	if n.expanded {
		for _, c := range n.children {
			collectVisibleRec(c, out)
		}
	}
}
