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
		res, ok := <-c
		if !ok {
			return nil
		}
		return engineResultMsg(res)
	}
}

func waitForFile(c chan string) tea.Cmd {
	return func() tea.Msg {
		file, ok := <-c
		if !ok {
			return nil
		}
		return engineFileMsg(file)
	}
}

func waitForDiscovered(c chan string) tea.Cmd {
	return func() tea.Msg {
		url, ok := <-c
		if !ok {
			return nil
		}
		return engineDiscoveredMsg(url)
	}
}

func waitForAnalysis(c chan analyzer.Report) tea.Cmd {
	return func() tea.Msg {
		report, ok := <-c
		if !ok {
			return nil
		}
		return engineAnalysisMsg(report)
	}
}
