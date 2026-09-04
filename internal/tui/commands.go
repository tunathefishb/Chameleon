package tui

import (
	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	tea "github.com/charmbracelet/bubbletea"
)

// Custom Messages
type engineResultMsg engine.Result
type engineFileMsg string
type engineDiscoveredMsg string
type engineAnalysisMsg analyzer.Report

func waitForResult(c chan engine.Result) tea.Cmd {
	return func() tea.Msg {
		return engineResultMsg(<-c)
	}
}

func waitForFile(c chan string) tea.Cmd {
	return func() tea.Msg {
		return engineFileMsg(<-c)
	}
}

func waitForDiscovered(c chan string) tea.Cmd {
	return func() tea.Msg {
		return engineDiscoveredMsg(<-c)
	}
}

func waitForAnalysis(c chan analyzer.Report) tea.Cmd {
	return func() tea.Msg {
		return engineAnalysisMsg(<-c)
	}
}
