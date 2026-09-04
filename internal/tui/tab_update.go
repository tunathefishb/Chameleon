package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateTabbedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "f1":
		m.showHelp = true
		return *m, nil
	}

	// Focus switching between active tab and bottom URL bar
	if msg.String() == "tab" {
		if m.focusTarget == FocusTabContent {
			m.setFocusTarget(FocusURLInput)
		} else {
			m.setFocusTarget(FocusTabContent)
		}
		return *m, nil
	}

	// When URL input is focused
	if m.focusTarget == FocusURLInput {
		switch msg.String() {
		case "esc":
			m.setFocusTarget(FocusTabContent)
			return *m, nil

		case "ctrl+a", "f2":
			val := strings.TrimSpace(m.textInput.Value())
			if val != "" {
				m.startAnalysis(val)
				m.activeTab = TabTelemetry
				m.centerMode = CenterViewReport
				m.setFocusTarget(FocusTabContent)
				return *m, nil
			}
		}

		return m.updateInput(msg)
	}

	// When active tab content is focused
	switch msg.String() {
	case "?":
		m.showHelp = true
		return *m, nil

	case "1":
		m.focusTab(TabQueue)
		return *m, nil

	case "2":
		m.focusTab(TabTelemetry)
		return *m, nil

	case "3":
		m.focusTab(TabFiles)
		return *m, nil

	case "4":
		m.focusTab(TabSettings)
		return *m, nil

	case "[":
		m.prevTab()
		return *m, nil

	case "]":
		m.nextTab()
		return *m, nil

	case "ctrl+a", "f2":
		if m.activeTab == TabTelemetry {
			if m.centerMode == CenterViewTelemetry {
				m.centerMode = CenterViewReport
			} else {
				m.centerMode = CenterViewTelemetry
			}
			return *m, nil
		}
	}

	// Delegate to active tab component
	switch m.activeTab {
	case TabQueue:
		return m.updateQueueKey(msg)

	case TabTelemetry:
		return m.updateTable(msg)

	case TabFiles:
		m.filesViewport, cmd = m.filesViewport.Update(msg)
		return *m, cmd

	case TabSettings:
		return m.updateSettings(msg)
	}

	return *m, nil
}
