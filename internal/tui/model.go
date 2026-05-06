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

// panelState tracks which panel is active.
type panelState int

const (
	panelClassrooms panelState = iota
	panelAssignments
	panelStudents
	panelActivity
)

// Async result messages — each carries its own key so the handler can cache
// the result without relying on the current cursor position.

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
	return loadClassroomsCmd(m.token)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := m.panelWidths()
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
		m.assignments.SetItems(msg.assignments)
		return m, nil

	case studentsLoadedMsg:
		m.students.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.students[msg.assignmentID] = msg.students
		m.students.SetItems(msg.students)
		return m, nil

	case activityLoadedMsg:
		m.activity.loading = false
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
			return m, nil
		}
		m.cache.activity[msg.repoFullName] = msg.activity
		m.activity.SetActivity(msg.activity)
		return m, nil

	case tea.KeyMsg:
		m.statusMsg = ""
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
				}
			}
			return m, nil
		case "c":
			if url := m.currentCopyURL(); url != "" {
				if err := clipboard.Init(); err != nil {
					m.statusMsg = "URL: " + url
				} else {
					clipboard.Write(clipboard.FmtText, []byte(url))
					m.statusMsg = "copied!"
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

	if m.width < 100 {
		inner := max(0, m.width-2)
		ch := max(0, m.height-3)
		content := m.activeView(inner)
		panel := activePanelStyle.Width(inner).Height(ch).Render(content)
		return panel + "\n" + statusBar
	}

	w := m.panelWidths()
	ch := max(0, m.height-3)

	p0 := panelStyle(m.state == panelClassrooms).Width(max(0, w[0]-2)).Height(ch).Render(
		m.classrooms.View(max(0, w[0]-2), m.state == panelClassrooms))
	p1 := panelStyle(m.state == panelAssignments).Width(max(0, w[1]-2)).Height(ch).Render(
		m.assignments.View(max(0, w[1]-2), m.state == panelAssignments))
	p2 := panelStyle(m.state == panelStudents).Width(max(0, w[2]-2)).Height(ch).Render(
		m.students.View(max(0, w[2]-2), m.state == panelStudents))
	p3 := panelStyle(m.state == panelActivity).Width(max(0, w[3]-2)).Height(ch).Render(
		m.activity.View(max(0, w[3]-2), m.state == panelActivity))

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

func (m Model) activeView(width int) string {
	switch m.state {
	case panelClassrooms:
		return m.classrooms.View(width, true)
	case panelAssignments:
		return m.assignments.View(width, true)
	case panelStudents:
		return m.students.View(width, true)
	case panelActivity:
		return m.activity.View(width, true)
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
			m.assignments.SetItems(cached)
			return m, nil
		}
		m.assignments.loading = true
		return m, loadAssignmentsCmd(m.token, cl.ID)

	case panelAssignments:
		a := m.assignments.SelectedItem()
		if a == nil {
			return m, nil
		}
		m.state = panelStudents
		if cached, ok := m.cache.students[a.ID]; ok {
			m.students.SetItems(cached)
			return m, nil
		}
		m.students.loading = true
		return m, loadStudentsCmd(m.token, a.ID)

	case panelStudents:
		s := m.students.SelectedItem()
		if s == nil {
			return m, nil
		}
		m.state = panelActivity
		if cached, ok := m.cache.activity[s.Repository.FullName]; ok {
			m.activity.SetActivity(cached)
			return m, nil
		}
		m.activity.loading = true
		return m, loadActivityCmd(m.token, s.Repository.FullName)
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
		return m, loadClassroomsCmd(m.token)

	case panelAssignments:
		if cl := m.classrooms.SelectedItem(); cl != nil {
			delete(m.cache.assignments, cl.ID)
			m.assignments.loading = true
			return m, loadAssignmentsCmd(m.token, cl.ID)
		}

	case panelStudents:
		if a := m.assignments.SelectedItem(); a != nil {
			delete(m.cache.students, a.ID)
			m.students.loading = true
			return m, loadStudentsCmd(m.token, a.ID)
		}

	case panelActivity:
		if s := m.students.SelectedItem(); s != nil {
			delete(m.cache.activity, s.Repository.FullName)
			m.activity.loading = true
			return m, loadActivityCmd(m.token, s.Repository.FullName)
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
		if cl := m.classrooms.SelectedItem(); cl != nil {
			if a := m.assignments.SelectedItem(); a != nil {
				return api.ReportURL(cl.ID, a.ID)
			}
		}
	case panelStudents, panelActivity:
		if s := m.students.SelectedItem(); s != nil {
			return s.Repository.HTMLURL
		}
	}
	return ""
}

// currentCopyURL returns the URL for the "c" (copy) key.
// Assignments and students panels copy the report URL; activity copies the repo URL.
func (m Model) currentCopyURL() string {
	switch m.state {
	case panelAssignments, panelStudents:
		if cl := m.classrooms.SelectedItem(); cl != nil {
			if a := m.assignments.SelectedItem(); a != nil {
				return api.ReportURL(cl.ID, a.ID)
			}
		}
	case panelActivity:
		if s := m.students.SelectedItem(); s != nil {
			return s.Repository.HTMLURL
		}
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

// Lipgloss styles.
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

func loadActivityCmd(token string, repoFullName string) tea.Cmd {
	return func() tea.Msg {
		activity, err := api.GetRepoActivity(token, repoFullName)
		return activityLoadedMsg{repoFullName: repoFullName, activity: activity, err: err}
	}
}
