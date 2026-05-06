package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"ghclassroom/internal/api"
	"golang.design/x/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type panelState int

const (
	panelClassrooms panelState = iota
	panelAssignments
	panelStudents
	panelActivity
)

type classroomsLoadedMsg struct {
	classrooms []api.Classroom
	err        error
}

type assignmentsLoadedMsg struct {
	classroomID int
	assignments []api.Assignment
	err         error
}

type studentsLoadedMsg struct {
	assignmentID int
	students     []api.AcceptedAssignment
	err          error
}

type activityLoadedMsg struct {
	repoFullName string
	repoURL      string
	activity     *api.RepoActivity
	err          error
}

type tickMsg time.Time

type cache struct {
	classrooms  []api.Classroom
	assignments map[int][]api.Assignment
	students    map[int][]api.AcceptedAssignment
	activity    map[string]*api.RepoActivity
}

type Model struct {
	token              string
	clipboardAvailable bool
	state              panelState
	width              int
	height             int
	cache              cache
	classrooms         ClassroomsPanel
	assignments        AssignmentsPanel
	students           StudentsPanel
	activity           ActivityPanel
	statusMsg          string
	rateLimitReset     time.Time
	showRateModal      bool
}

func New(token string, clipboardAvailable bool) Model {
	m := Model{
		token:              token,
		clipboardAvailable: clipboardAvailable,
		state:              panelClassrooms,
		cache: cache{
			assignments: make(map[int][]api.Assignment),
			students:    make(map[int][]api.AcceptedAssignment),
			activity:    make(map[string]*api.RepoActivity),
		},
		classrooms:  newClassroomsPanel(),
		assignments: newAssignmentsPanel(),
		students:    newStudentsPanel(),
		activity:    newActivityPanel(),
	}
	m.classrooms.loading = true
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadClassroomsCmd(m.token), m.classrooms.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := m.panelWidths()
		ch := max(0, m.height-6)
		m.classrooms.setSize(max(0, w[0]-2), ch)
		m.assignments.setSize(max(0, w[1]-2), ch)
		m.students.setSize(max(0, w[2]-2), ch)
		m.activity.setSize(max(0, w[3]-2), ch)
		return m, nil

	case tickMsg:
		if !m.rateLimitReset.IsZero() && time.Now().Before(m.rateLimitReset) {
			return m, tickCmd()
		}
		return m, nil

	case classroomsLoadedMsg:
		m.classrooms.loading = false
		if msg.err != nil {
			if rle, ok := api.AsRateLimitError(msg.err); ok {
				m.rateLimitReset = rle.ResetAt
				m.showRateModal = true
				return m, tickCmd()
			}
			m.statusMsg = loadErrMsg(msg.err)
			return m, nil
		}
		m.cache.classrooms = msg.classrooms
		m.classrooms.SetItems(msg.classrooms)
		return m, nil

	case assignmentsLoadedMsg:
		m.assignments.loading = false
		if msg.err != nil {
			if rle, ok := api.AsRateLimitError(msg.err); ok {
				m.rateLimitReset = rle.ResetAt
				m.showRateModal = true
				return m, tickCmd()
			}
			m.statusMsg = loadErrMsg(msg.err)
			return m, nil
		}
		m.cache.assignments[msg.classroomID] = msg.assignments
		m.assignments.SetItems(msg.classroomID, msg.assignments)
		return m, nil

	case studentsLoadedMsg:
		m.students.loading = false
		if msg.err != nil {
			if rle, ok := api.AsRateLimitError(msg.err); ok {
				m.rateLimitReset = rle.ResetAt
				m.showRateModal = true
				return m, tickCmd()
			}
			m.statusMsg = loadErrMsg(msg.err)
			return m, nil
		}
		m.cache.students[msg.assignmentID] = msg.students
		m.students.SetItems(msg.assignmentID, msg.students)
		return m, nil

	case activityLoadedMsg:
		m.activity.loading = false
		if msg.err != nil {
			if rle, ok := api.AsRateLimitError(msg.err); ok {
				m.rateLimitReset = rle.ResetAt
				m.showRateModal = true
				m.activity.SetError(msg.err.Error())
				return m, tickCmd()
			}
			m.activity.SetError(msg.err.Error())
			if isUnauthorized(msg.err) {
				m.statusMsg = unauthorizedMsg
			}
			return m, nil
		}
		m.cache.activity[msg.repoFullName] = msg.activity
		m.activity.SetActivity(msg.activity, msg.repoFullName, msg.repoURL)
		return m, nil

	case tea.KeyMsg:
		m.statusMsg = ""
		if m.showRateModal && msg.String() == "esc" {
			m.showRateModal = false
			return m, nil
		}
		if m.state == panelAssignments && m.assignments.IsFiltering() {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m.routeToActivePanel(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "right", "enter":
			return m.handleForward()
		case "left", "esc":
			return m.handleBack()
		case "r":
			return m.handleRefresh()
		case "o":
			if url := m.currentURL(); url != "" {
				if err := openURL(url); err != nil {
					m.statusMsg = "cannot open browser: " + err.Error()
				} else {
					m.statusMsg = "Opened in browser"
				}
			}
			return m, nil
		case "c":
			if url := m.currentCopyURL(); url != "" {
				if !m.clipboardAvailable {
					m.statusMsg = "URL: " + url
				} else {
					clipboard.Write(clipboard.FmtText, []byte(url))
					m.statusMsg = "Copied to clipboard"
				}
			}
			return m, nil
		}
	}

	return m.routeToActivePanel(msg)
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	statusBar := m.renderStatusBar()
	ch := max(0, m.height-6)
	outerH := ch + 2

	var row string
	if m.width < 100 {
		row = m.activePanel(m.width, outerH)
	} else {
		w := m.panelWidths()
		p0 := m.classrooms.View(m.state == panelClassrooms, w[0], outerH)
		p1 := m.assignments.View(m.state == panelAssignments, w[1], outerH)
		p2 := m.students.View(m.state == panelStudents, w[2], outerH)
		p3 := m.activity.View(m.state == panelActivity, w[3], outerH)
		row = lipgloss.JoinHorizontal(lipgloss.Top, p0, p1, p2, p3)
	}

	result := header + "\n" + row + "\n" + statusBar
	if m.showRateModal {
		result = m.applyRateModal(result)
	}
	return result
}

func (m Model) renderHeader() string {
	brand := amberStyle.Bold(true).Render("ghclassroom") + dimStyle.Render(" v0.1")

	names := []string{"Classrooms", "Assignments", "Students", "Activity"}
	states := []panelState{panelClassrooms, panelAssignments, panelStudents, panelActivity}
	parts := make([]string, len(names))
	for i, name := range names {
		if states[i] == m.state {
			parts[i] = amberStyle.Render(name)
		} else {
			parts[i] = dimStyle.Render(name)
		}
	}
	crumbs := strings.Join(parts, dimStyle.Render(" › "))

	var right string
	if !m.rateLimitReset.IsZero() {
		right = redStyle.Render("⚠ rate limited")
	}

	bw := lipgloss.Width(brand)
	cw := lipgloss.Width(crumbs)
	rw := lipgloss.Width(right)
	space := m.width - bw - cw - rw
	if space < 2 {
		space = 2
	}
	leftPad := space / 2
	rightPad := space - leftPad

	line := brand + strings.Repeat(" ", leftPad) + crumbs + strings.Repeat(" ", rightPad) + right
	sep := dimStyle.Render(strings.Repeat("╌", m.width))
	return line + "\n" + sep
}

func (m Model) renderStatusBar() string {
	sep := dimStyle.Render(strings.Repeat("╌", m.width))

	var left string
	if m.statusMsg != "" {
		left = dimStyle.Render(m.statusMsg)
	} else if !m.rateLimitReset.IsZero() {
		remaining := time.Until(m.rateLimitReset)
		if remaining > 0 {
			mins := int(remaining.Minutes())
			secs := int(remaining.Seconds()) % 60
			left = redStyle.Render(fmt.Sprintf("Rate limit reached · resets in %d min %d s", mins, secs))
		} else {
			left = dimStyle.Render("Rate limit passed · press r to retry")
		}
	} else if url := m.currentURL(); url != "" {
		left = amberStyle.Render(url)
	}

	right := m.keyHints()

	lw := lipgloss.Width(left)
	rw := lipgloss.Width(right)
	gap := m.width - lw - rw
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + right
	return sep + "\n" + bar
}

func (m Model) keyHints() string {
	k := func(key, desc string) string {
		return amberStyle.Render(key) + dimStyle.Render(" "+desc)
	}
	join := func(hints ...string) string {
		return strings.Join(hints, dimStyle.Render("  "))
	}

	if m.classrooms.loading {
		return k("q", "quit")
	}
	switch m.state {
	case panelClassrooms:
		return join(k("↵", "drill in"), k("o", "open"), k("r", "refresh"), k("q", "quit"))
	case panelAssignments:
		return join(k("↵", "drill in"), k("/", "filter"), k("c", "copy"), k("o", "open"), k("esc", "back"))
	case panelStudents:
		return join(k("↵", "view activity"), k("c", "copy repo"), k("o", "open"), k("esc", "back"))
	case panelActivity:
		return join(k("o", "open repo"), k("c", "copy URL"), k("r", "refresh"), k("esc", "back"))
	}
	return k("q", "quit")
}

func (m Model) renderModalBox() string {
	resetAt := m.rateLimitReset.UTC().Format("2006-01-02 15:04 UTC")
	remaining := time.Until(m.rateLimitReset)

	var countdown string
	if remaining > 0 {
		mins := int(remaining.Minutes())
		secs := int(remaining.Seconds()) % 60
		countdown = amberStyle.Render(fmt.Sprintf("(in %d min %d s)", mins, secs))
	} else {
		countdown = amberStyle.Render("(ready to retry)")
	}

	innerW := min(54, m.width-8)
	content := redStyle.Bold(true).Render("⚠  Rate limit reached") + "\n\n" +
		"GitHub API responded " + redStyle.Render("403") + "  " + dimStyle.Render("X-RateLimit-Remaining: 0") + "\n" +
		"Resets at " + amberStyle.Render(resetAt) + "  " + countdown + "\n\n" +
		dimStyle.Render("Cached panels remain available.\nPress ") + amberStyle.Render("r") + dimStyle.Render(" to retry after reset.") + "\n\n" +
		amberStyle.Render("esc") + dimStyle.Render(" dismiss  ") + amberStyle.Render("q") + dimStyle.Render(" quit")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f87171")).
		Padding(0, 1).
		Width(innerW).
		Render(content)
}

// applyRateModal overlays the rate-limit modal box centered over the rendered view
// using ANSI cursor-positioning sequences appended to the view string.
func (m Model) applyRateModal(base string) string {
	box := m.renderModalBox()
	boxLines := strings.Split(box, "\n")
	boxH := len(boxLines)
	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}

	startRow := (m.height-boxH)/2 + 1
	startCol := (m.width - boxW) / 2
	if startRow < 3 {
		startRow = 3
	}
	if startCol < 0 {
		startCol = 0
	}

	var sb strings.Builder
	sb.WriteString(base)
	for i, line := range boxLines {
		sb.WriteString(fmt.Sprintf("\033[%d;%dH", startRow+i+1, startCol+1))
		sb.WriteString(line)
	}
	return sb.String()
}

func (m Model) panelWidths() [4]int {
	w := [4]int{
		int(float64(m.width) * 0.20),
		int(float64(m.width) * 0.25),
		int(float64(m.width) * 0.25),
		0,
	}
	w[3] = m.width - w[0] - w[1] - w[2]
	return w
}

func (m Model) activePanel(width, height int) string {
	switch m.state {
	case panelClassrooms:
		return m.classrooms.View(true, width, height)
	case panelAssignments:
		return m.assignments.View(true, width, height)
	case panelStudents:
		return m.students.View(true, width, height)
	case panelActivity:
		return m.activity.View(true, width, height)
	}
	return ""
}

func (m Model) handleForward() (tea.Model, tea.Cmd) {
	switch m.state {
	case panelClassrooms:
		cl := m.classrooms.SelectedItem()
		if cl == nil {
			return m, nil
		}
		m.state = panelAssignments
		if cached, ok := m.cache.assignments[cl.ID]; ok {
			m.assignments.SetItems(cl.ID, cached)
			return m, nil
		}
		m.assignments.loading = true
		return m, tea.Batch(loadAssignmentsCmd(m.token, cl.ID), m.assignments.spinner.Tick)

	case panelAssignments:
		a := m.assignments.SelectedItem()
		if a == nil {
			return m, nil
		}
		m.state = panelStudents
		if cached, ok := m.cache.students[a.ID]; ok {
			m.students.SetItems(a.ID, cached)
			return m, nil
		}
		m.students.loading = true
		return m, tea.Batch(loadStudentsCmd(m.token, a.ID), m.students.spinner.Tick)

	case panelStudents:
		s := m.students.SelectedItem()
		if s == nil {
			return m, nil
		}
		m.state = panelActivity
		if cached, ok := m.cache.activity[s.Repository.FullName]; ok {
			m.activity.SetActivity(cached, s.Repository.FullName, s.Repository.HTMLURL)
			return m, nil
		}
		m.activity.startLoading()
		return m, tea.Batch(
			loadActivityCmd(m.token, s.Repository.FullName, s.Repository.HTMLURL),
			m.activity.spinner.Tick,
		)
	}

	return m, nil
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case panelAssignments:
		m.state = panelClassrooms
	case panelStudents:
		m.state = panelAssignments
	case panelActivity:
		m.state = panelStudents
	}
	return m, nil
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	switch m.state {
	case panelClassrooms:
		m.cache.classrooms = nil
		m.classrooms.loading = true
		return m, tea.Batch(loadClassroomsCmd(m.token), m.classrooms.spinner.Tick)

	case panelAssignments:
		if cl := m.classrooms.SelectedItem(); cl != nil {
			delete(m.cache.assignments, cl.ID)
			m.assignments.loading = true
			return m, tea.Batch(loadAssignmentsCmd(m.token, cl.ID), m.assignments.spinner.Tick)
		}

	case panelStudents:
		if a := m.assignments.SelectedItem(); a != nil {
			delete(m.cache.students, a.ID)
			m.students.loading = true
			return m, tea.Batch(loadStudentsCmd(m.token, a.ID), m.students.spinner.Tick)
		}

	case panelActivity:
		if s := m.students.SelectedItem(); s != nil {
			delete(m.cache.activity, s.Repository.FullName)
			m.activity.startLoading()
			return m, tea.Batch(
				loadActivityCmd(m.token, s.Repository.FullName, s.Repository.HTMLURL),
				m.activity.spinner.Tick,
			)
		}
	}

	return m, nil
}

func (m Model) currentURL() string {
	switch m.state {
	case panelClassrooms:
		if cl := m.classrooms.SelectedItem(); cl != nil {
			return cl.URL
		}
	case panelAssignments:
		return m.assignments.SelectedReportURL()
	case panelStudents:
		if s := m.students.SelectedItem(); s != nil {
			return s.Repository.HTMLURL
		}
	case panelActivity:
		return m.activity.RepoURL()
	}
	return ""
}

func (m Model) currentCopyURL() string {
	switch m.state {
	case panelAssignments, panelStudents:
		return m.assignments.SelectedReportURL()
	case panelActivity:
		return m.activity.RepoURL()
	}
	return ""
}

func (m Model) routeToActivePanel(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case panelClassrooms:
		m.classrooms, cmd = m.classrooms.Update(msg)
	case panelAssignments:
		m.assignments, cmd = m.assignments.Update(msg)
	case panelStudents:
		m.students, cmd = m.students.Update(msg)
	case panelActivity:
		m.activity, cmd = m.activity.Update(msg)
	}
	return m, cmd
}

var (
	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#f59e0b"))

	inactivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	amberStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#67e8f9"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	blueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#60a5fa"))
	purpleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#c084fc"))
)

func panelStyle(active bool) lipgloss.Style {
	if active {
		return activePanelStyle
	}
	return inactivePanelStyle
}

// renderPanelHeader produces a 2-row panel title (NAME + count / dashed separator + trailing newline).
func renderPanelHeader(name, count string, active bool, w int) string {
	var nameStr string
	if active {
		nameStr = amberStyle.Bold(true).Render(strings.ToUpper(name))
	} else {
		nameStr = dimStyle.Render(strings.ToUpper(name))
	}
	countStr := dimStyle.Render(count)

	nw := lipgloss.Width(nameStr)
	cw := lipgloss.Width(countStr)
	pad := w - nw - cw
	if pad < 1 {
		pad = 1
	}
	row := nameStr + strings.Repeat(" ", pad) + countStr
	sep := dimStyle.Render(strings.Repeat("╌", w))
	return row + "\n" + sep + "\n"
}

const unauthorizedMsg = "Token unauthorized. Run ghclassroom with --reconfigure to reset."

func isUnauthorized(err error) bool {
	return strings.HasPrefix(err.Error(), "GitHub API 401")
}

func loadErrMsg(err error) string {
	if isUnauthorized(err) {
		return unauthorizedMsg
	}
	return err.Error()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func openURL(url string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return exec.Command(cmd, url).Start()
}

func loadClassroomsCmd(token string) tea.Cmd {
	return func() tea.Msg {
		classrooms, err := api.GetClassrooms(token)
		return classroomsLoadedMsg{classrooms: classrooms, err: err}
	}
}

func loadAssignmentsCmd(token string, classroomID int) tea.Cmd {
	return func() tea.Msg {
		assignments, err := api.GetAssignments(token, classroomID)
		return assignmentsLoadedMsg{classroomID: classroomID, assignments: assignments, err: err}
	}
}

func loadStudentsCmd(token string, assignmentID int) tea.Cmd {
	return func() tea.Msg {
		students, err := api.GetAcceptedAssignments(token, assignmentID)
		return studentsLoadedMsg{assignmentID: assignmentID, students: students, err: err}
	}
}

func loadActivityCmd(token, repoFullName, repoURL string) tea.Cmd {
	return func() tea.Msg {
		activity, err := api.GetRepoActivity(token, repoFullName)
		return activityLoadedMsg{
			repoFullName: repoFullName,
			repoURL:      repoURL,
			activity:     activity,
			err:          err,
		}
	}
}
