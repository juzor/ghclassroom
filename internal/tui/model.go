package tui

import (
	"fmt"
	"os/exec"
	"runtime"

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

// Async result messages — each carries its own cache key so the handler
// does not depend on the cursor position at the time the response arrives.

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

type cache struct {
	classrooms  []api.Classroom
	assignments map[int][]api.Assignment
	students    map[int][]api.AcceptedAssignment
	activity    map[string]*api.RepoActivity
}

type Model struct {
	token       string
	state       panelState
	width       int
	height      int
	cache       cache
	classrooms  ClassroomsPanel
	assignments AssignmentsPanel
	students    StudentsPanel
	activity    ActivityPanel
	statusMsg   string
}

func New(token string) Model {
	m := Model{
		token: token,
		state: panelClassrooms,
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
		// setSize receives inner content dimensions (border = 2 per axis).
		ch := max(0, m.height-3)
		m.classrooms.setSize(max(0, w[0]-2), ch)
		m.assignments.setSize(max(0, w[1]-2), ch)
		m.students.setSize(max(0, w[2]-2), ch)
		m.activity.setSize(max(0, w[3]-2), ch)
		return m, nil

	case classroomsLoadedMsg:
		m.classrooms.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.classrooms = msg.classrooms
		m.classrooms.SetItems(msg.classrooms)
		return m, nil

	case assignmentsLoadedMsg:
		m.assignments.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.assignments[msg.classroomID] = msg.assignments
		m.assignments.SetItems(msg.classroomID, msg.assignments)
		return m, nil

	case studentsLoadedMsg:
		m.students.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.students[msg.assignmentID] = msg.students
		m.students.SetItems(msg.assignmentID, msg.students)
		return m, nil

	case activityLoadedMsg:
		m.activity.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.activity[msg.repoFullName] = msg.activity
		m.activity.SetActivity(msg.activity, msg.repoFullName, msg.repoURL)
		return m, nil

	case tea.KeyMsg:
		m.statusMsg = ""
		// While the assignments filter is active, let the list consume all
		// keystrokes (esc exits filter mode, enter applies it, etc.).
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
				if err := clipboard.Init(); err != nil {
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

	statusBar := m.renderStatusBar()
	// ch = inner content height; panels receive outer height = ch+2.
	ch := max(0, m.height-3)
	outerH := ch + 2

	if m.width < 100 {
		panel := m.activePanel(m.width, outerH)
		return panel + "\n" + statusBar
	}

	w := m.panelWidths()
	p0 := m.classrooms.View(m.state == panelClassrooms, w[0], outerH)
	p1 := m.assignments.View(m.state == panelAssignments, w[1], outerH)
	p2 := m.students.View(m.state == panelStudents, w[2], outerH)
	p3 := m.activity.View(m.state == panelActivity, w[3], outerH)

	row := lipgloss.JoinHorizontal(lipgloss.Top, p0, p1, p2, p3)
	return row + "\n" + statusBar
}

// panelWidths returns the outer column count allocated to each panel.
// Ratios: 20% / 25% / 25% / 30%; last panel absorbs rounding remainder.
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

// activePanel renders only the active panel (used when width < 100).
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

func (m Model) renderStatusBar() string {
	msg := m.statusMsg
	if msg == "" {
		msg = "↑↓ navigate  enter select  esc back  o open  c copy  r refresh  q quit"
	}
	return statusBarStyle.Width(m.width).Render(msg)
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
		m.activity.loading = true
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
			m.activity.loading = true
			return m, tea.Batch(
				loadActivityCmd(m.token, s.Repository.FullName, s.Repository.HTMLURL),
				m.activity.spinner.Tick,
			)
		}
	}

	return m, nil
}

// currentURL returns the URL for the "o" (open in browser) key.
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

// currentCopyURL returns the URL for the "c" (copy) key.
// Assignments and students panels copy the report URL; activity copies the repo URL.
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

// Package-level styles used by all panels and the status bar.
var (
	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	inactivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Foreground(lipgloss.Color("240"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// dimStyle is used inline in panel content (description text, URL line).
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

func panelStyle(active bool) lipgloss.Style {
	if active {
		return activePanelStyle
	}
	return inactivePanelStyle
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

// Async commands — each wraps its identifying key in the result message.

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
