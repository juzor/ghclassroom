package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type state int

const (
	stateClassrooms state = iota
	stateAssignments
	stateStudents
	stateActivity
)

type Model struct {
	state state
}

func New() Model {
	return Model{state: stateClassrooms}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return ""
}
