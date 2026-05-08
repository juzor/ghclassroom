package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ghclassroom/internal/api"
	"ghclassroom/internal/classifier"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	cursorNodeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3a352a")).
			Foreground(lipgloss.Color("#f59e0b")).
			Bold(true)
)

type Sidebar struct {
	roots        []*treeNode
	cursor       int
	offset       int
	width        int
	height       int
	spinner      spinner.Model
	loading      bool
	filterActive bool
	filterQuery  string
}

func newSidebar() Sidebar {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return Sidebar{spinner: s, loading: true}
}

func (s *Sidebar) setSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *Sidebar) SetClassrooms(classrooms []api.Classroom) {
	s.roots = make([]*treeNode, len(classrooms))
	for i, c := range classrooms {
		s.roots[i] = &treeNode{kind: nodeClassroom, classroom: c}
	}
	s.loading = false
}

func (s Sidebar) visibleNodes() []*treeNode {
	if s.filterActive && s.filterQuery != "" {
		return s.filteredNodes()
	}
	return collectVisible(s.roots)
}

func (s Sidebar) filteredNodes() []*treeNode {
	q := strings.ToLower(s.filterQuery)
	var out []*treeNode
	for _, root := range s.roots {
		var matchingAssignments []*treeNode
		for _, aNode := range root.children {
			if aNode.kind == nodeAssignment &&
				strings.Contains(strings.ToLower(aNode.assignment.Title), q) {
				matchingAssignments = append(matchingAssignments, aNode)
			}
		}
		out = append(out, root)
		out = append(out, matchingAssignments...)
	}
	return out
}

func (s Sidebar) filterStats() (totalAssignments, matchingAssignments int) {
	q := strings.ToLower(s.filterQuery)
	for _, root := range s.roots {
		for _, aNode := range root.children {
			if aNode.kind == nodeAssignment {
				totalAssignments++
				if strings.Contains(strings.ToLower(aNode.assignment.Title), q) {
					matchingAssignments++
				}
			}
		}
	}
	return
}

func (s Sidebar) SelectedNode() *treeNode {
	visible := s.visibleNodes()
	if s.cursor >= 0 && s.cursor < len(visible) {
		n := visible[s.cursor]
		if n.kind == nodeEmpty {
			return nil
		}
		return n
	}
	return nil
}

func (s *Sidebar) MoveUp() {
	if s.cursor > 0 {
		s.cursor--
		if s.cursor < s.offset {
			s.offset = s.cursor
		}
	}
}

func (s *Sidebar) MoveDown() {
	visible := s.visibleNodes()
	if s.cursor < len(visible)-1 {
		s.cursor++
		avail := s.availH()
		if s.cursor >= s.offset+avail {
			s.offset = s.cursor - avail + 1
		}
	}
}

// Expand expands or activates the selected node. Returns the node if children need loading.
func (s *Sidebar) Expand() *treeNode {
	n := s.SelectedNode()
	if n == nil {
		return nil
	}
	if n.kind == nodeStudent {
		return n
	}
	if n.expanded {
		s.MoveDown()
		return nil
	}
	n.expanded = true
	if !n.loading && len(n.children) == 0 {
		n.loading = true
		return n
	}
	return nil
}

// Collapse collapses the selected node or moves cursor to parent.
func (s *Sidebar) Collapse() {
	n := s.SelectedNode()
	if n == nil {
		return
	}
	if n.expanded {
		n.expanded = false
		return
	}
	if n.parent != nil {
		visible := collectVisible(s.roots)
		for i, v := range visible {
			if v == n.parent {
				s.cursor = i
				if s.cursor < s.offset {
					s.offset = s.cursor
				}
				return
			}
		}
	}
}

func (s *Sidebar) EnterFilter() {
	s.filterActive = true
	s.filterQuery = ""
	s.cursor = 0
	s.offset = 0
}

func (s *Sidebar) FilterAppend(r rune) {
	s.filterQuery += string(r)
	s.cursor = 0
	s.offset = 0
}

func (s *Sidebar) FilterBackspace() {
	if s.filterQuery == "" {
		return
	}
	runes := []rune(s.filterQuery)
	s.filterQuery = string(runes[:len(runes)-1])
	s.cursor = 0
	s.offset = 0
}

func (s *Sidebar) ExitFilter() {
	s.filterActive = false
	s.filterQuery = ""
	s.cursor = 0
	s.offset = 0
}

func (s *Sidebar) SetAssignments(classroomID int, assignments []api.Assignment) {
	for _, root := range s.roots {
		if root.classroom.ID == classroomID {
			root.loading = false
			root.children = make([]*treeNode, len(assignments))
			for i, a := range assignments {
				root.children[i] = &treeNode{
					kind:       nodeAssignment,
					assignment: a,
					parent:     root,
					depth:      1,
				}
			}
			return
		}
	}
}

func (s *Sidebar) SetStudents(assignmentID int, students []api.AcceptedAssignment) {
	for _, root := range s.roots {
		for _, aNode := range root.children {
			if aNode.kind == nodeAssignment && aNode.assignment.ID == assignmentID {
				aNode.loading = false
				if len(students) == 0 {
					aNode.children = []*treeNode{{
						kind:   nodeEmpty,
						parent: aNode,
						depth:  2,
					}}
					return
				}
				aNode.children = make([]*treeNode, len(students))
				for i, st := range students {
					aNode.children[i] = &treeNode{
						kind:    nodeStudent,
						student: st,
						parent:  aNode,
						depth:   2,
					}
				}
				return
			}
		}
	}
}

func (s *Sidebar) ApplyStatuses(assignmentID int, statuses []classifier.StudentStatus) {
	byRepo := make(map[string]*classifier.StudentStatus, len(statuses))
	for i := range statuses {
		byRepo[statuses[i].RepoFullName] = &statuses[i]
	}
	for _, root := range s.roots {
		for _, aNode := range root.children {
			if aNode.kind == nodeAssignment && aNode.assignment.ID == assignmentID {
				aNode.classifierApplied = true
				for _, sNode := range aNode.children {
					if sNode.kind == nodeStudent {
						sNode.status = byRepo[sNode.student.Repository.FullName]
					}
				}
				return
			}
		}
	}
}

func (s *Sidebar) ResetStatuses(assignmentID int) {
	for _, root := range s.roots {
		for _, aNode := range root.children {
			if aNode.kind == nodeAssignment && aNode.assignment.ID == assignmentID {
				aNode.classifierApplied = false
				for _, sNode := range aNode.children {
					sNode.status = nil
				}
				return
			}
		}
	}
}

func (s Sidebar) Update(msg tea.Msg) (Sidebar, tea.Cmd) {
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

func (s Sidebar) availH() int {
	innerH := max(0, s.height-2)
	base := max(0, innerH-2)
	if s.filterActive {
		return max(0, base-1)
	}
	return base
}

func (s Sidebar) countStr() string {
	if s.loading {
		return "…"
	}
	if s.filterActive && s.filterQuery != "" {
		total, matching := s.filterStats()
		return fmt.Sprintf("%d of %d match", matching, total)
	}
	n := s.SelectedNode()
	if n == nil {
		if len(s.roots) == 0 {
			return "0"
		}
		return fmt.Sprintf("%d classrooms", len(s.roots))
	}
	switch n.kind {
	case nodeClassroom:
		return fmt.Sprintf("%d classrooms", len(s.roots))
	case nodeAssignment:
		if n.parent != nil {
			return fmt.Sprintf("%d assignments", len(n.parent.children))
		}
		return fmt.Sprintf("%d classrooms", len(s.roots))
	case nodeStudent:
		if n.parent != nil {
			total := 0
			submitted := 0
			for _, c := range n.parent.children {
				if c.kind == nodeStudent {
					total++
					if c.student.Submitted {
						submitted++
					}
				}
			}
			return fmt.Sprintf("%d students · %d submitted", total, submitted)
		}
	}
	return fmt.Sprintf("%d classrooms", len(s.roots))
}

func (s Sidebar) View(active bool) string {
	w := s.width
	h := s.height
	inner := max(0, w-2)
	innerH := max(0, h-2)
	availH := s.availH()

	hdr := renderPanelHeader("Tree", s.countStr(), active, inner)

	if s.loading {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render(s.spinner.View() + " Loading classrooms…\n" + dimStyle.Render("GET /classrooms"))
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
	}

	visible := s.visibleNodes()
	if len(visible) == 0 {
		body := lipgloss.NewStyle().Width(inner).Height(availH).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No classrooms found.\n" + dimStyle.Render("Check token scopes (repo, read:org)."))
		filterBar := ""
		if s.filterActive {
			filterBar = s.renderFilterBar(inner)
		}
		return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body + filterBar)
	}

	var sb strings.Builder
	end := s.offset + availH
	if end > len(visible) {
		end = len(visible)
	}

	for i := s.offset; i < end; i++ {
		n := visible[i]
		isCursor := active && i == s.cursor
		line := s.renderNode(n, isCursor, inner)
		lw := lipgloss.Width(line)
		if lw < inner {
			line += strings.Repeat(" ", inner-lw)
		}
		sb.WriteString(line + "\n")
	}

	for j := end - s.offset; j < availH; j++ {
		sb.WriteString(strings.Repeat(" ", inner) + "\n")
	}

	body := sb.String()
	if s.filterActive {
		body += s.renderFilterBar(inner)
	}

	return panelStyle(active).Width(inner).Height(innerH).Render(hdr + body)
}

func (s Sidebar) renderFilterBar(w int) string {
	prompt := amberStyle.Bold(true).Render("/")
	query := s.filterQuery
	caret := amberStyle.Render("▊")
	hint := dimStyle.Render("esc cancel · ↵ confirm")

	left := prompt + " " + query + caret
	lw := lipgloss.Width(left)
	rw := lipgloss.Width(hint)
	gap := w - lw - rw
	if gap < 1 {
		gap = 1
	}
	sep := dimStyle.Render(strings.Repeat("╌", w))
	bar := left + strings.Repeat(" ", gap) + hint
	return "\n" + sep + "\n" + bar
}

func (s Sidebar) renderNode(n *treeNode, cursor bool, w int) string {
	indent := strings.Repeat("  ", n.depth)

	// Build indicator
	var indicator string
	switch {
	case n.loading:
		indicator = s.spinner.View() + " "
	case n.kind == nodeEmpty:
		indicator = dimStyle.Render("·") + " "
	case n.kind == nodeStudent:
		classifierRan := n.parent != nil && n.parent.classifierApplied
		switch {
		case classifierRan && n.status != nil && n.status.NeedsAttention:
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚠") + " "
		case classifierRan && n.status != nil && !n.status.NeedsAttention:
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓") + " "
		case n.student.Submitted:
			indicator = cyanStyle.Render("✓") + " "
		default:
			indicator = dimStyle.Render("·") + " "
		}
	case n.expanded:
		indicator = dimStyle.Render("▾") + " "
	default:
		indicator = dimStyle.Render("▸") + " "
	}

	// Compute meta (right-aligned for classroom/assignment)
	meta := n.meta()
	if n.kind == nodeClassroom {
		meta = fmt.Sprintf("%d", len(n.children))
	}

	// Width budget
	prefixW := lipgloss.Width(indent + indicator)
	metaW := 0
	if meta != "" {
		metaW = lipgloss.Width(meta) + 1 // +1 gap
	}
	maxLabelW := w - prefixW - metaW
	if maxLabelW < 4 {
		maxLabelW = 4
	}

	// Truncate label to fit
	label := n.label()
	labelRunes := []rune(label)
	for lipgloss.Width(string(labelRunes)) > maxLabelW && len(labelRunes) > 1 {
		labelRunes = labelRunes[:len(labelRunes)-1]
	}
	if utf8.RuneCountInString(label) > len(labelRunes) {
		label = string(labelRunes[:max(0, len(labelRunes)-1)]) + "…"
	} else {
		label = string(labelRunes)
	}

	// Highlight filter match in label
	if s.filterActive && s.filterQuery != "" && (n.kind == nodeAssignment) {
		label = highlightMatch(label, s.filterQuery)
	}

	// Build meta string styled
	var metaStr string
	if meta != "" {
		metaStr = dimStyle.Render(meta)
	}

	if cursor {
		// Cursor row: amber bg + bold
		rawLabel := n.label()
		if len([]rune(rawLabel)) > len(labelRunes) {
			rawLabel = string(labelRunes[:max(0, len(labelRunes)-1)]) + "…"
		}
		labelW := lipgloss.Width(rawLabel)
		padW := maxLabelW - labelW
		if padW < 0 {
			padW = 0
		}
		labelPadded := rawLabel + strings.Repeat(" ", padW)
		rendered := indent + indicator + cursorNodeStyle.Render(labelPadded)
		if metaStr != "" {
			rendered += " " + dimStyle.Render(meta)
		}
		return rendered
	}

	// Style label by node kind
	var styledLabel string
	switch n.kind {
	case nodeClassroom:
		styledLabel = label
	case nodeEmpty:
		styledLabel = dimStyle.Italic(true).Render(label)
	default:
		styledLabel = dimStyle.Render(label)
	}

	if metaStr == "" {
		return indent + indicator + styledLabel
	}

	// Pad label to fill space before meta
	labelW := lipgloss.Width(label)
	padW := maxLabelW - labelW
	if padW < 0 {
		padW = 0
	}
	return indent + indicator + styledLabel + strings.Repeat(" ", padW) + " " + metaStr
}

func highlightMatch(s, q string) string {
	if q == "" {
		return dimStyle.Render(s)
	}
	lower := strings.ToLower(s)
	lq := strings.ToLower(q)
	idx := strings.Index(lower, lq)
	if idx < 0 {
		return dimStyle.Render(s)
	}
	before := s[:idx]
	match := s[idx : idx+len(q)]
	after := s[idx+len(q):]
	return dimStyle.Render(before) + amberStyle.Bold(true).Render(match) + dimStyle.Render(after)
}
