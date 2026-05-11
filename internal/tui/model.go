package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ghclassroom/internal/api"
	"ghclassroom/internal/classifier"
	"ghclassroom/internal/config"
	"ghclassroom/internal/downloader"
	"golang.design/x/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type allActivitiesLoadedMsg struct {
	assignmentID int
	activities   map[string]*api.RepoActivity
}

type cloneResultMsg struct {
	login string
	repo  string
	err   error
}

type cloneProgressMsg struct {
	login string
	repo  string
	err   error
	done  int
	total int
}

type tickMsg time.Time

type cache struct {
	classrooms  []api.Classroom
	assignments map[int][]api.Assignment
	students    map[int][]api.AcceptedAssignment
	activity    map[string]*api.RepoActivity
	statuses    map[int][]classifier.StudentStatus
}

type Model struct {
	token              string
	clipboardAvailable bool
	thresholdDays      int
	contentFocus       bool
	width              int
	height             int
	cache              cache
	sidebar            Sidebar
	content            ContentPanel
	statusMsg             string
	allActivitiesLoading  bool
	thresholdInput        textinput.Model
	thresholdInputActive  bool
	dirInput              DirInput
	cloneMode             string // "single" or "all"
	cloneSingleURL        string
	cloneSingleLogin      string
	cloneSingleRepo       string
	cloneAllStudents      []struct{ Login, URL string }
	cloneCh               <-chan downloader.Result
	cloneTotal            int
	cloneDone             int
	cloneSucc             int
	cloneFail             int
	cloneDir              string
	lastDownloadDir       string
	rateLimitReset        time.Time
	showRateModal         bool
}

func New(token string, clipboardAvailable bool, thresholdDays int, lastDownloadDir string) Model {
	return Model{
		token:              token,
		clipboardAvailable: clipboardAvailable,
		thresholdDays:      thresholdDays,
		lastDownloadDir:    lastDownloadDir,
		cache: cache{
			assignments: make(map[int][]api.Assignment),
			students:    make(map[int][]api.AcceptedAssignment),
			activity:    make(map[string]*api.RepoActivity),
			statuses:    make(map[int][]classifier.StudentStatus),
		},
		sidebar:  newSidebar(),
		content:  newContentPanel(),
		dirInput: NewDirInput(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadClassroomsCmd(m.token), m.sidebar.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case tickMsg:
		if !m.rateLimitReset.IsZero() && time.Now().Before(m.rateLimitReset) {
			return m, tickCmd()
		}
		return m, nil

	case classroomsLoadedMsg:
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
		m.sidebar.SetClassrooms(msg.classrooms)
		return m.updateContentForSelection()

	case assignmentsLoadedMsg:
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
		m.sidebar.SetAssignments(msg.classroomID, msg.assignments)
		return m.updateContentForSelection()

	case studentsLoadedMsg:
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
		m.sidebar.SetStudents(msg.assignmentID, msg.students)
		m.allActivitiesLoading = true
		m2, contentCmd := m.updateContentForSelection()
		return m2, tea.Batch(
			contentCmd,
			loadAllActivitiesCmd(m2.token, msg.assignmentID, msg.students),
		)

	case allActivitiesLoadedMsg:
		for k, v := range msg.activities {
			if _, exists := m.cache.activity[k]; !exists {
				m.cache.activity[k] = v
			}
		}
		statuses := classifier.Classify(
			m.cache.students[msg.assignmentID],
			m.cache.activity,
			m.thresholdDays,
		)
		m.cache.statuses[msg.assignmentID] = statuses
		m.sidebar.ApplyStatuses(msg.assignmentID, statuses)
		m.allActivitiesLoading = false
		return m, nil

	case activityLoadedMsg:
		if msg.err != nil {
			if rle, ok := api.AsRateLimitError(msg.err); ok {
				m.rateLimitReset = rle.ResetAt
				m.showRateModal = true
				m.content.SetError(msg.err.Error())
				return m, tickCmd()
			}
			m.content.SetError(msg.err.Error())
			if isUnauthorized(msg.err) {
				m.statusMsg = unauthorizedMsg
			}
			return m, nil
		}
		m.cache.activity[msg.repoFullName] = msg.activity
		if n := m.sidebar.SelectedNode(); n != nil && n.kind == nodeStudent &&
			n.student.Repository.FullName == msg.repoFullName {
			m.content.ShowActivity(msg.activity, msg.repoFullName, msg.repoURL, m.statusForNode(n))
		}
		return m, nil

	case cloneResultMsg:
		if msg.err != nil {
			m.statusMsg = "Clone failed: " + msg.err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("Cloned %s to %s", msg.repo, m.cloneDir)
		}
		return m, nil

	case cloneProgressMsg:
		if msg.err == nil {
			m.cloneSucc++
		} else {
			m.cloneFail++
		}
		m.cloneDone = msg.done
		if msg.done < msg.total {
			m.statusMsg = fmt.Sprintf("Cloning repos: %d/%d  ·  %s", msg.done, msg.total, msg.login)
			return m, waitCloneProgressCmd(m.cloneCh, msg.done, msg.total)
		}
		m.statusMsg = fmt.Sprintf("Done: %d cloned, %d failed → %s", m.cloneSucc, m.cloneFail, m.cloneDir)
		m.cloneCh = nil
		return m, nil

	case tea.KeyMsg:
		// DirInput suppresses all other keybindings while active.
		if m.dirInput.Active() {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			updated, cmd := m.dirInput.Update(msg)
			m.dirInput = updated
			if path := m.dirInput.Consume(); path != "" {
				return m.handleDirInputConfirmed(path, cmd)
			}
			return m, cmd
		}

		// Threshold input suppresses everything else while active.
		if m.thresholdInputActive {
			return m.handleThresholdInputKeys(msg)
		}

		m.statusMsg = ""

		// Filter mode intercepts most keys
		if m.sidebar.filterActive {
			return m.handleFilterKeys(msg)
		}

		if m.showRateModal {
			switch msg.String() {
			case "esc":
				m.showRateModal = false
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.contentFocus = !m.contentFocus
			return m, nil
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

		if m.contentFocus {
			return m.handleContentKeys(msg)
		}
		return m.handleSidebarKeys(msg)
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	if m.dirInput.Active() {
		updated, c := m.dirInput.Update(msg)
		m.dirInput = updated
		cmds = append(cmds, c)
	}
	if m.thresholdInputActive {
		m.thresholdInput, cmd = m.thresholdInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	m.sidebar, cmd = m.sidebar.Update(msg)
	cmds = append(cmds, cmd)
	m.content, cmd = m.content.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m Model) handleFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.sidebar.ExitFilter()
		return m.updateContentForSelection()
	case "enter":
		m.sidebar.filterActive = false
		return m.updateContentForSelection()
	case "backspace", "ctrl+h":
		m.sidebar.FilterBackspace()
	case "ctrl+c":
		return m, tea.Quit
	default:
		// Append printable rune
		runes := []rune(msg.String())
		if len(runes) == 1 && runes[0] >= 32 {
			m.sidebar.FilterAppend(runes[0])
		}
	}
	return m, nil
}

func (m Model) handleSidebarKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		prev := m.sidebar.SelectedNode()
		m.sidebar.MoveUp()
		if m.sidebar.SelectedNode() != prev {
			return m.updateContentForSelection()
		}
	case "down", "j":
		prev := m.sidebar.SelectedNode()
		m.sidebar.MoveDown()
		if m.sidebar.SelectedNode() != prev {
			return m.updateContentForSelection()
		}
	case "enter", "right", "l":
		return m.handleActivate()
	case "left", "h", "esc", "-":
		m.sidebar.Collapse()
		return m.updateContentForSelection()
	case "/":
		m.sidebar.EnterFilter()
	case "t":
		n := m.sidebar.SelectedNode()
		if n != nil && n.kind == nodeStudent {
			ti := textinput.New()
			ti.CharLimit = 3
			ti.Placeholder = fmt.Sprintf("%d", m.thresholdDays)
			m.thresholdInput = ti
			m.thresholdInputActive = true
			return m, m.thresholdInput.Focus()
		}
	case "d":
		n := m.sidebar.SelectedNode()
		if n != nil && n.kind == nodeStudent {
			m.cloneMode = "single"
			m.cloneSingleURL = n.student.Repository.HTMLURL
			m.cloneSingleRepo = n.student.Repository.FullName
			m.cloneSingleLogin = ""
			if len(n.student.Students) > 0 {
				m.cloneSingleLogin = n.student.Students[0].Login
			}
			m.dirInput.Open("Clone repo to: ", m.defaultDownloadDir(), nil, nil)
			return m, m.dirInput.FocusCmd()
		}
	case "D":
		n := m.sidebar.SelectedNode()
		if n != nil && n.kind == nodeStudent && n.parent != nil {
			aID := n.parent.assignment.ID
			students := m.cache.students[aID]
			repos := make([]struct{ Login, URL string }, 0, len(students))
			for _, s := range students {
				login := ""
				if len(s.Students) > 0 {
					login = s.Students[0].Login
				}
				repos = append(repos, struct{ Login, URL string }{Login: login, URL: s.Repository.HTMLURL})
			}
			m.cloneMode = "all"
			m.cloneAllStudents = repos
			m.dirInput.Open("Clone all repos to: ", m.defaultDownloadDir(), nil, nil)
			return m, m.dirInput.FocusCmd()
		}
	}
	return m, nil
}

func (m Model) handleThresholdInputKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.thresholdInputActive = false
		return m, nil
	case "enter":
		val, err := strconv.Atoi(strings.TrimSpace(m.thresholdInput.Value()))
		if err != nil || val <= 0 {
			m.thresholdInputActive = false
			m.statusMsg = "Invalid: must be a number > 0"
			return m, nil
		}
		m.thresholdDays = val
		n := m.sidebar.SelectedNode()
		if n != nil && n.kind == nodeStudent && n.parent != nil {
			aID := n.parent.assignment.ID
			delete(m.cache.statuses, aID)
			statuses := classifier.Classify(
				m.cache.students[aID],
				m.cache.activity,
				m.thresholdDays,
			)
			m.cache.statuses[aID] = statuses
			m.sidebar.ApplyStatuses(aID, statuses)
		}
		m.thresholdInputActive = false
		m.statusMsg = fmt.Sprintf("Threshold updated to %d days", val)
		return m, nil
	default:
		var cmd tea.Cmd
		m.thresholdInput, cmd = m.thresholdInput.Update(msg)
		return m, cmd
	}
}

func (m Model) handleContentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.contentFocus = false
		return m, nil
	}
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return m, cmd
}

func (m Model) handleActivate() (tea.Model, tea.Cmd) {
	n := m.sidebar.Expand()
	if n == nil {
		return m.updateContentForSelection()
	}

	switch n.kind {
	case nodeClassroom:
		if cached, ok := m.cache.assignments[n.classroom.ID]; ok {
			n.loading = false
			m.sidebar.SetAssignments(n.classroom.ID, cached)
			return m.updateContentForSelection()
		}
		m2, contentCmd := m.updateContentForSelection()
		return m2, tea.Batch(
			loadAssignmentsCmd(m2.token, n.classroom.ID),
			m2.sidebar.spinner.Tick,
			contentCmd,
		)
	case nodeAssignment:
		if cached, ok := m.cache.students[n.assignment.ID]; ok {
			n.loading = false
			m.sidebar.SetStudents(n.assignment.ID, cached)
			return m.updateContentForSelection()
		}
		m2, contentCmd := m.updateContentForSelection()
		return m2, tea.Batch(
			loadStudentsCmd(m2.token, n.assignment.ID),
			m2.sidebar.spinner.Tick,
			contentCmd,
		)
	case nodeStudent:
		// Load activity on explicit activation (not on cursor move)
		if cached, ok := m.cache.activity[n.student.Repository.FullName]; ok {
			m.content.ShowActivity(cached, n.student.Repository.FullName, n.student.Repository.HTMLURL, m.statusForNode(n))
			return m, nil
		}
		m.content.StartLoadingActivity(n.student.Repository.HTMLURL)
		return m, tea.Batch(
			loadActivityCmd(m.token, n.student.Repository.FullName, n.student.Repository.HTMLURL),
			m.content.spinner.Tick,
		)
	}
	return m, nil
}

// updateContentForSelection refreshes the content pane to match the current cursor.
// For student nodes it shows a preview; activity loads only on explicit ↵.
func (m Model) updateContentForSelection() (Model, tea.Cmd) {
	n := m.sidebar.SelectedNode()
	if n == nil {
		return m, nil
	}
	switch n.kind {
	case nodeClassroom:
		assignments := m.cache.assignments[n.classroom.ID]
		m.content.ShowClassroom(n.classroom, len(assignments), len(assignments) > 0)
	case nodeAssignment:
		students := m.cache.students[n.assignment.ID]
		var submitted int
		for _, s := range students {
			if s.Submitted {
				submitted++
			}
		}
		classroomID := 0
		if n.parent != nil {
			classroomID = n.parent.classroom.ID
		}
		m.content.ShowAssignment(n.assignment, classroomID, len(students), submitted, len(students) > 0)
	case nodeStudent:
		// Show preview — don't auto-load activity; user presses ↵ for that
		if cached, ok := m.cache.activity[n.student.Repository.FullName]; ok {
			m.content.ShowActivity(cached, n.student.Repository.FullName, n.student.Repository.HTMLURL, m.statusForNode(n))
			return m, nil
		}
		m.content.ShowStudentPreview(n.student)
	}
	return m, nil
}

func (m Model) statusForNode(n *treeNode) *classifier.StudentStatus {
	if n == nil || n.parent == nil {
		return nil
	}
	repo := n.student.Repository.FullName
	for i := range m.cache.statuses[n.parent.assignment.ID] {
		if m.cache.statuses[n.parent.assignment.ID][i].RepoFullName == repo {
			return &m.cache.statuses[n.parent.assignment.ID][i]
		}
	}
	return nil
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	n := m.sidebar.SelectedNode()
	if n == nil {
		return m.refreshClassrooms()
	}
	switch n.kind {
	case nodeClassroom:
		return m.refreshClassrooms()
	case nodeAssignment:
		if n.parent == nil {
			return m, nil
		}
		clID := n.parent.classroom.ID
		delete(m.cache.assignments, clID)
		n.parent.loading = true
		n.parent.children = nil
		n.parent.expanded = true
		return m, tea.Batch(loadAssignmentsCmd(m.token, clID), m.sidebar.spinner.Tick)
	case nodeStudent:
		if n.parent == nil {
			return m, nil
		}
		aID := n.parent.assignment.ID
		students, ok := m.cache.students[aID]
		if !ok {
			return m, nil
		}
		for _, s := range students {
			delete(m.cache.activity, s.Repository.FullName)
		}
		delete(m.cache.statuses, aID)
		m.sidebar.ResetStatuses(aID)
		m.content.Reset()
		m.allActivitiesLoading = true
		m.statusMsg = "Refreshing activity data..."
		return m, tea.Batch(loadAllActivitiesCmd(m.token, aID, students), m.sidebar.spinner.Tick)
	}
	return m, nil
}

func (m Model) handleDirInputConfirmed(path string, priorCmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.lastDownloadDir = path
	m.cloneDir = path
	m.saveDownloadDir(path)
	switch m.cloneMode {
	case "single":
		m.statusMsg = fmt.Sprintf("Cloning %s…", m.cloneSingleRepo)
		return m, tea.Batch(priorCmd, cloneSingleCmd(m.cloneSingleURL, path, m.cloneSingleLogin, m.cloneSingleRepo))
	case "all":
		total := len(m.cloneAllStudents)
		if total == 0 {
			m.statusMsg = "No repos to clone."
			return m, priorCmd
		}
		ch := make(chan downloader.Result, total)
		m.cloneCh = ch
		m.cloneTotal = total
		m.cloneDone = 0
		m.cloneSucc = 0
		m.cloneFail = 0
		m.statusMsg = fmt.Sprintf("Cloning repos: 0/%d", total)
		go downloader.CloneAll(m.cloneAllStudents, path, ch)
		return m, tea.Batch(priorCmd, waitCloneProgressCmd(ch, 0, total))
	}
	return m, priorCmd
}

func (m Model) defaultDownloadDir() string {
	if m.lastDownloadDir != "" {
		return m.lastDownloadDir
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func (m Model) saveDownloadDir(dir string) {
	cfg := config.Config{
		Token:                   m.token,
		InactivityThresholdDays: m.thresholdDays,
		LastDownloadDir:         dir,
	}
	_ = config.SaveConfig(cfg)
}

func (m Model) refreshClassrooms() (tea.Model, tea.Cmd) {
	m.cache.classrooms = nil
	m.sidebar.roots = nil
	m.sidebar.loading = true
	m.sidebar.cursor = 0
	m.sidebar.offset = 0
	return m, tea.Batch(loadClassroomsCmd(m.token), m.sidebar.spinner.Tick)
}

func (m *Model) updateSizes() {
	ch := max(0, m.height-6)
	outerH := ch + 2
	sideW := int(float64(m.width) * 0.28)
	if sideW < 20 {
		sideW = 20
	}
	contentW := m.width - sideW
	m.sidebar.setSize(sideW, outerH)
	m.content.setSize(contentW, outerH)
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	header := m.renderHeader()
	statusBar := m.renderStatusBar()

	sidebarView := m.sidebar.View(!m.contentFocus)
	contentView := m.content.View(m.contentFocus)
	row := lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentView)

	result := header + "\n" + row + "\n" + statusBar
	if m.showRateModal {
		result = m.applyRateModal(result)
	}
	return result
}

func (m Model) renderHeader() string {
	brand := amberStyle.Bold(true).Render("ghclassroom") + dimStyle.Render(" v0.1")
	crumbs := m.breadcrumb()

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

func (m Model) breadcrumb() string {
	levels := [4]string{"Classrooms", "Assignments", "Students", "Activity"}
	active := 0
	n := m.sidebar.SelectedNode()
	if n != nil {
		switch n.kind {
		case nodeClassroom:
			active = 0
		case nodeAssignment:
			active = 1
		case nodeStudent:
			if m.content.IsActivityLoaded() {
				active = 3
			} else {
				active = 2
			}
		}
	}
	sep := dimStyle.Render(" › ")
	strs := make([]string, len(levels))
	for i, l := range levels {
		if i == active {
			strs[i] = amberStyle.Render(l)
		} else {
			strs[i] = dimStyle.Render(l)
		}
	}
	return strings.Join(strs[:], sep)
}

func (m Model) renderStatusBar() string {
	sep := dimStyle.Render(strings.Repeat("╌", m.width))

	if m.dirInput.Active() {
		k := func(key, desc string) string {
			return amberStyle.Render(key) + dimStyle.Render(" "+desc)
		}
		prompt := m.dirInput.View()
		hints := k("↵", "confirm") + dimStyle.Render("  ") + k("esc", "cancel")
		lw := lipgloss.Width(prompt)
		rw := lipgloss.Width(hints)
		gap := m.width - lw - rw
		if gap < 1 {
			gap = 1
		}
		return sep + "\n" + prompt + strings.Repeat(" ", gap) + hints
	}

	if m.thresholdInputActive {
		k := func(key, desc string) string {
			return amberStyle.Render(key) + dimStyle.Render(" "+desc)
		}
		prompt := dimStyle.Render("Inactivity threshold (days): ") + m.thresholdInput.View()
		hints := k("↵", "confirm") + dimStyle.Render("  ") + k("esc", "cancel")
		lw := lipgloss.Width(prompt)
		rw := lipgloss.Width(hints)
		gap := m.width - lw - rw
		if gap < 1 {
			gap = 1
		}
		return sep + "\n" + prompt + strings.Repeat(" ", gap) + hints
	}

	var left string
	if m.statusMsg != "" {
		left = dimStyle.Render(m.statusMsg)
	} else if m.allActivitiesLoading {
		left = dimStyle.Render("Analysing student activity…")
	} else if !m.rateLimitReset.IsZero() {
		remaining := time.Until(m.rateLimitReset)
		if remaining > 0 {
			mins := int(remaining.Minutes())
			secs := int(remaining.Seconds()) % 60
			left = redStyle.Render(fmt.Sprintf("Rate limit reached · resets in %d min %d s", mins, secs))
		} else {
			left = dimStyle.Render("Rate limit passed · press r to retry")
		}
	} else if n := m.sidebar.SelectedNode(); n != nil && n.kind == nodeStudent && !m.contentFocus {
		left = dimStyle.Render(fmt.Sprintf("Threshold: %dd", m.thresholdDays))
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

	if m.sidebar.filterActive {
		return join(k("↵", "confirm"), k("esc", "cancel"))
	}
	if m.contentFocus {
		return join(k("↑↓", "scroll"), k("tab", "sidebar"), k("q", "quit"))
	}
	n := m.sidebar.SelectedNode()
	if n == nil || m.sidebar.loading {
		return join(k("r", "retry"), k("q", "quit"))
	}
	switch n.kind {
	case nodeClassroom:
		return join(k("↵", "expand"), k("-", "collapse"), k("o", "open"), k("r", "refresh"), k("q", "quit"))
	case nodeAssignment:
		return join(k("↵", "expand"), k("-", "collapse"), k("/", "filter"), k("c", "copy URL"), k("o", "open"), k("q", "quit"))
	case nodeStudent:
		if m.content.IsActivityLoaded() {
			return join(k("↵", "view activity"), k("tab", "scroll"), k("o", "open repo"), k("c", "copy"), k("t", "threshold"), k("d", "clone"), k("D", "clone all"), k("q", "quit"))
		}
		return join(k("↵", "view activity"), k("o", "open"), k("c", "copy"), k("t", "threshold"), k("d", "clone"), k("D", "clone all"), k("q", "quit"))
	}
	return join(k("↑↓", "navigate"), k("q", "quit"))
}

func (m Model) currentURL() string {
	n := m.sidebar.SelectedNode()
	if n == nil {
		return ""
	}
	switch n.kind {
	case nodeClassroom:
		return n.classroom.URL
	case nodeAssignment:
		if n.parent != nil {
			return api.ReportURL(n.parent.classroom.ID, n.assignment.ID)
		}
	case nodeStudent:
		return n.student.Repository.HTMLURL
	}
	return ""
}

func (m Model) currentCopyURL() string {
	n := m.sidebar.SelectedNode()
	if n == nil {
		return ""
	}
	switch n.kind {
	case nodeAssignment:
		if n.parent != nil {
			return api.ReportURL(n.parent.classroom.ID, n.assignment.ID)
		}
	case nodeStudent:
		// Copy assignment report URL (parent) when on student node
		if n.parent != nil && n.parent.parent != nil {
			return api.ReportURL(n.parent.parent.classroom.ID, n.parent.assignment.ID)
		}
		return n.student.Repository.HTMLURL
	}
	return ""
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
		dimStyle.Render("Cached tree state remains available.\nPress ") + amberStyle.Render("r") + dimStyle.Render(" to retry after reset.") + "\n\n" +
		amberStyle.Render("esc") + dimStyle.Render(" dismiss  ") + amberStyle.Render("q") + dimStyle.Render(" quit")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#f87171")).
		Padding(0, 1).
		Width(innerW).
		Render(content)
}

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

func loadAllActivitiesCmd(token string, assignmentID int, students []api.AcceptedAssignment) tea.Cmd {
	return func() tea.Msg {
		jobs := make(chan api.AcceptedAssignment, len(students))
		results := make(chan struct {
			key      string
			activity *api.RepoActivity
		}, len(students))

		for i := 0; i < 5; i++ {
			go func() {
				for s := range jobs {
					activity, _ := api.GetRepoActivity(token, s.Repository.FullName)
					results <- struct {
						key      string
						activity *api.RepoActivity
					}{s.Repository.FullName, activity}
				}
			}()
		}

		for _, s := range students {
			jobs <- s
		}
		close(jobs)

		collected := map[string]*api.RepoActivity{}
		for range students {
			r := <-results
			collected[r.key] = r.activity
		}
		return allActivitiesLoadedMsg{assignmentID: assignmentID, activities: collected}
	}
}

func cloneSingleCmd(repoURL, targetDir, login, repo string) tea.Cmd {
	return func() tea.Msg {
		err := downloader.CloneRepo(repoURL, targetDir)
		return cloneResultMsg{login: login, repo: repo, err: err}
	}
}

func waitCloneProgressCmd(ch <-chan downloader.Result, done, total int) tea.Cmd {
	return func() tea.Msg {
		r := <-ch
		return cloneProgressMsg{
			login: r.Login,
			repo:  r.Repo,
			err:   r.Error,
			done:  done + 1,
			total: total,
		}
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
