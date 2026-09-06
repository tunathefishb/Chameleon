package tui

import (
	"strings"
	"testing"
	"time"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPanelCycling(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	if m.activePanel != PanelInput {
		t.Fatalf("expected initial panel to be PanelInput, got %d", m.activePanel)
	}

	// Tab to Queue
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activePanel != PanelQueue {
		t.Fatalf("expected active panel to be PanelQueue, got %d", m.activePanel)
	}

	// Tab to Table
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activePanel != PanelTable {
		t.Fatalf("expected active panel to be PanelTable, got %d", m.activePanel)
	}
	if !m.table.Focused() {
		t.Fatalf("expected table to be focused")
	}

	// Tab to Files
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activePanel != PanelFiles {
		t.Fatalf("expected active panel to be PanelFiles, got %d", m.activePanel)
	}
	if m.table.Focused() {
		t.Fatalf("expected table to be unfocused")
	}

	// Tab to Settings
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activePanel != PanelSettings {
		t.Fatalf("expected active panel to be PanelSettings, got %d", m.activePanel)
	}

	// Tab back to Input
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.activePanel != PanelInput {
		t.Fatalf("expected active panel to be PanelInput, got %d", m.activePanel)
	}
	if !m.textInput.Focused() {
		t.Fatalf("expected textInput to be focused")
	}

	// Shift+Tab back to Settings
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.activePanel != PanelSettings {
		t.Fatalf("expected active panel to be PanelSettings, got %d", m.activePanel)
	}
}

func TestSettingsInteraction(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusPanel(PanelSettings)

	// Initial settingIndex is 0 (Depth)
	if m.settings.Depth != 1 {
		t.Fatalf("expected depth 1, got %d", m.settings.Depth)
	}

	// Press right arrow -> Depth increments to 2
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.settings.Depth != 2 {
		t.Fatalf("expected depth 2, got %d", m.settings.Depth)
	}

	// Move down to Images
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.settingIndex != 1 {
		t.Fatalf("expected settingIndex 1, got %d", m.settingIndex)
	}

	// Press space -> toggle images
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if !m.settings.Images {
		t.Fatalf("expected Images to be true")
	}

	// Move down to Speed
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.settingIndex != 2 {
		t.Fatalf("expected settingIndex 2, got %d", m.settingIndex)
	}

	// Press enter -> toggle speed
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.settings.Speed != engine.SpeedFast {
		t.Fatalf("expected Speed to be Fast, got %s", m.settings.Speed)
	}
}

func TestAnalysisViewAndToggling(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// Set URL and trigger analysis via Ctrl+A
	m.textInput.SetValue("example.com")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = next.(Model)

	if !m.isAnalyzing {
		t.Errorf("expected isAnalyzing to be true")
	}
	if m.centerMode != CenterViewReport {
		t.Errorf("expected centerMode to be CenterViewReport")
	}

	// Simulate receiving analysis report message
	reportMsg := engineAnalysisMsg(analyzer.Report{
		URL:               "https://example.com",
		EthicalGrade:      "A",
		EthicalScore:      95,
		EthicalDetails:    []string{"✅ robots.txt allows crawling"},
		DifficultyScore:   2,
		DifficultyLevel:   "Easy",
		DifficultyDetails: []string{"📄 Static / SSR HTML detected"},
		RateLimitInfo:     "Passive: No rate limit headers detected",
		Recommendation:    "Direct HTTP Scraping Recommended",
		Duration:          50 * time.Millisecond,
	})

	next, _ = m.Update(reportMsg)
	m = next.(Model)

	if m.isAnalyzing {
		t.Errorf("expected isAnalyzing to be false after report received")
	}
	if m.latestReport == nil || m.latestReport.EthicalGrade != "A" {
		t.Errorf("expected latestReport with grade A")
	}

	view := m.View()
	if !strings.Contains(view, "ETHICAL GRADE: A") {
		t.Errorf("expected view to render 'ETHICAL GRADE: A'")
	}
	if !strings.Contains(view, "Scrapability & Ethical Report") {
		t.Errorf("expected view title to contain 'Scrapability & Ethical Report'")
	}

	// Toggle back to Telemetry
	m.focusPanel(PanelTable)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(Model)
	if m.centerMode != CenterViewTelemetry {
		t.Errorf("expected centerMode to toggle back to CenterViewTelemetry")
	}
}

func TestViewRendering(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		t.Errorf("expected view height <= %d, got %d lines", m.height, len(lines))
	}
	if !strings.Contains(view, "Jobs Queue") {
		t.Errorf("expected view to contain 'Jobs Queue'")
	}
	if !strings.Contains(view, "Telemetry Live") {
		t.Errorf("expected view to contain 'Telemetry Live'")
	}
	if !strings.Contains(view, "Engine Settings") {
		t.Errorf("expected view to contain 'Engine Settings'")
	}
	if !strings.Contains(view, "❯") {
		t.Errorf("expected view to contain styled input prompt '❯'")
	}
	if strings.Contains(view, "❯ >") {
		t.Errorf("expected view not to contain duplicate prompt '❯ >'")
	}
}

func TestQueueJobManagement(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// Focus PanelQueue
	m.focusPanel(PanelQueue)

	// Add test jobs
	url1 := "https://example.com/job1"
	url2 := "https://example.com/job2"
	m.eng.AddJob(url1, m.settings)
	m.addQueue(url1)
	m.eng.AddJob(url2, m.settings)
	m.addQueue(url2)

	if len(m.queue) != 2 {
		t.Fatalf("expected 2 jobs in queue, got %d", len(m.queue))
	}
	if m.queueCursor != 0 {
		t.Fatalf("expected queueCursor 0, got %d", m.queueCursor)
	}

	// Move cursor down to job2
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.queueCursor != 1 {
		t.Fatalf("expected queueCursor 1 after down arrow, got %d", m.queueCursor)
	}

	// Toggle pause for job2 via space
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if m.eng.GetJobStatus(url2) != engine.StatusPaused {
		t.Fatalf("expected job2 status Paused, got %s", m.eng.GetJobStatus(url2))
	}

	view := m.View()
	if !strings.Contains(view, "Paused") {
		t.Errorf("expected view to contain 'Paused'")
	}

	// Toggle resume for job2 via 'p'
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = next.(Model)
	if m.eng.GetJobStatus(url2) != engine.StatusQueued {
		t.Fatalf("expected job2 status Queued after resume, got %s", m.eng.GetJobStatus(url2))
	}

	// Move cursor up to job1 and stop it via 's' (with confirmation safeguard)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if m.queueCursor != 0 {
		t.Fatalf("expected queueCursor 0 after up arrow, got %d", m.queueCursor)
	}

	// First 's' primes confirmation safeguard
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.confirmStopURL != url1 {
		t.Fatalf("expected confirmStopURL %s, got %s", url1, m.confirmStopURL)
	}
	if !strings.Contains(m.View(), "Confirm Stop") {
		t.Errorf("expected view to display confirmation prompt")
	}
	if m.eng.GetJobStatus(url1) == engine.StatusStopped {
		t.Fatalf("job1 should not be stopped before confirmation")
	}

	// Second 's' confirms stop
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.eng.GetJobStatus(url1) != engine.StatusStopped {
		t.Fatalf("expected job1 status Stopped after confirmation, got %s", m.eng.GetJobStatus(url1))
	}

	view = m.View()
	if !strings.Contains(view, "Stopped") {
		t.Errorf("expected view to contain 'Stopped'")
	}
}

func TestQueuePauseAll(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// Focus PanelQueue
	m.focusPanel(PanelQueue)

	url1 := "https://example.com/job1"
	url2 := "https://example.com/job2"
	m.eng.AddJob(url1, m.settings)
	m.addQueue(url1)
	m.eng.AddJob(url2, m.settings)
	m.addQueue(url2)

	if m.eng.GetJobStatus(url1) != engine.StatusQueued || m.eng.GetJobStatus(url2) != engine.StatusQueued {
		t.Fatalf("expected both jobs to be Queued initially")
	}

	// Press 'P' (Shift+P) in PanelQueue to pause all jobs
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = next.(Model)

	if m.eng.GetJobStatus(url1) != engine.StatusPaused || m.eng.GetJobStatus(url2) != engine.StatusPaused {
		t.Fatalf("expected both jobs to be Paused after pressing 'P'")
	}

	// Press 'P' again to resume all jobs
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	m = next.(Model)

	if m.eng.GetJobStatus(url1) != engine.StatusQueued || m.eng.GetJobStatus(url2) != engine.StatusQueued {
		t.Fatalf("expected both jobs to be Queued after second 'P'")
	}
}

func TestVerboseTelemetryDetails(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// Simulate receiving telemetry results
	headers := make(map[string][]string)
	headers["Content-Type"] = []string{"application/json"}
	headers["Server"] = []string{"ChameleonTestServer/1.0"}

	res := engine.Result{
		Name:            "api-endpoint",
		Status:          "200",
		Type:            "application/json",
		Size:            "1.2 KB",
		Time:            "45ms",
		URL:             "https://example.com/api/v1/telemetry",
		Method:          "GET",
		StatusCode:      200,
		ResponseHeaders: headers,
		Timestamp:       time.Now(),
	}

	next, _ = m.Update(engineResultMsg(res))
	m = next.(Model)

	if len(m.telemetryItems) != 1 {
		t.Fatalf("expected 1 telemetry item, got %d", len(m.telemetryItems))
	}

	// Focus PanelTable
	m.focusPanel(PanelTable)

	// Press Enter to inspect the focused telemetry row
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.centerMode != CenterViewVerbose {
		t.Fatalf("expected centerMode to be CenterViewVerbose, got %d", m.centerMode)
	}

	view := m.View()
	if !strings.Contains(view, "Request Details") {
		t.Errorf("expected view title to contain 'Request Details'")
	}
	if !strings.Contains(view, "https://example.com/api/v1/telemetry") {
		t.Errorf("expected view to contain full URL")
	}
	if !strings.Contains(view, "Server: ChameleonTestServer/1.0") {
		t.Errorf("expected view to contain header Server: ChameleonTestServer/1.0")
	}
	if !strings.Contains(view, "Content-Type: application/json") {
		t.Errorf("expected view to contain header Content-Type: application/json")
	}

	// Press Esc to dismiss verbose view
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.centerMode != CenterViewTelemetry {
		t.Fatalf("expected centerMode to be CenterViewTelemetry after Esc, got %d", m.centerMode)
	}

	// Also verify 'i' key works
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = next.(Model)

	if m.centerMode != CenterViewVerbose {
		t.Fatalf("expected centerMode to be CenterViewVerbose after pressing 'i', got %d", m.centerMode)
	}

	// Press 'q' to dismiss
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = next.(Model)

	if m.centerMode != CenterViewTelemetry {
		t.Fatalf("expected centerMode to be CenterViewTelemetry after 'q', got %d", m.centerMode)
	}
}

func TestHelpModal(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// In initial state (PanelInput), '?' should go to textInput
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(Model)
	if m.showHelp {
		t.Fatalf("expected showHelp to be false when typing '?' in textInput")
	}
	if !strings.Contains(m.textInput.Value(), "?") {
		t.Fatalf("expected textInput to receive '?'")
	}

	// Pressing F1 in PanelInput should open help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyF1})
	m = next.(Model)
	if !m.showHelp {
		t.Fatalf("expected showHelp to be true after pressing F1")
	}

	view := m.View()
	if !strings.Contains(view, "Chameleon — Help & Keybindings") {
		t.Fatalf("expected view to contain help modal title")
	}
	if !strings.Contains(view, "Press Esc, q, or ? to close help") {
		t.Fatalf("expected view to contain close hint")
	}

	// Pressing Esc should close help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.showHelp {
		t.Fatalf("expected showHelp to be false after pressing Esc")
	}

	// Switch to PanelQueue
	m.focusPanel(PanelQueue)

	// Pressing '?' in PanelQueue should open help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(Model)
	if !m.showHelp {
		t.Fatalf("expected showHelp to be true after pressing '?' in PanelQueue")
	}

	// Pressing 'q' should close help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = next.(Model)
	if m.showHelp {
		t.Fatalf("expected showHelp to be false after pressing 'q'")
	}
}

func TestFooterContextualRendering(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	// 1. PanelInput
	footer := m.renderFooter()
	if !strings.Contains(footer, "TARGET URL") {
		t.Errorf("expected PanelInput footer to have 'TARGET URL' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Start Crawl") {
		t.Errorf("expected PanelInput footer to have 'Start Crawl', got: %s", footer)
	}
	if !strings.Contains(footer, "Audit Ethics") {
		t.Errorf("expected PanelInput footer to have 'Audit Ethics', got: %s", footer)
	}
	if !strings.Contains(footer, "[F1]") {
		t.Errorf("expected PanelInput footer to have '[F1]', got: %s", footer)
	}

	// 2. PanelQueue
	m.focusPanel(PanelQueue)
	footer = m.renderFooter()
	if !strings.Contains(footer, "JOBS QUEUE") {
		t.Errorf("expected PanelQueue footer to have 'JOBS QUEUE' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Pause/Resume") {
		t.Errorf("expected PanelQueue footer to have 'Pause/Resume', got: %s", footer)
	}
	if !strings.Contains(footer, "Pause All") {
		t.Errorf("expected PanelQueue footer to have 'Pause All', got: %s", footer)
	}
	if !strings.Contains(footer, "Stop") {
		t.Errorf("expected PanelQueue footer to have 'Stop', got: %s", footer)
	}

	// 3. PanelTable (Telemetry)
	m.focusPanel(PanelTable)
	footer = m.renderFooter()
	if !strings.Contains(footer, "TELEMETRY") {
		t.Errorf("expected PanelTable footer to have 'TELEMETRY' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Inspect") {
		t.Errorf("expected PanelTable footer to have 'Inspect', got: %s", footer)
	}
	if !strings.Contains(footer, "Ethical Report") {
		t.Errorf("expected PanelTable footer to have 'Ethical Report', got: %s", footer)
	}

	// 4. PanelTable (Report Mode)
	m.centerMode = CenterViewReport
	footer = m.renderFooter()
	if !strings.Contains(footer, "ETHICAL REPORT") {
		t.Errorf("expected Report mode footer to have 'ETHICAL REPORT' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Back to Table") {
		t.Errorf("expected Report mode footer to have 'Back to Table', got: %s", footer)
	}

	// 5. PanelTable (Verbose Mode)
	m.centerMode = CenterViewVerbose
	footer = m.renderFooter()
	if !strings.Contains(footer, "REQUEST DETAILS") {
		t.Errorf("expected Verbose mode footer to have 'REQUEST DETAILS' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Back to Table") {
		t.Errorf("expected Verbose mode footer to have 'Back to Table', got: %s", footer)
	}

	// 6. PanelFiles
	m.focusPanel(PanelFiles)
	footer = m.renderFooter()
	if !strings.Contains(footer, "SAVED FILES") {
		t.Errorf("expected PanelFiles footer to have 'SAVED FILES' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Scroll Files") {
		t.Errorf("expected PanelFiles footer to have 'Scroll Files', got: %s", footer)
	}

	// 7. PanelSettings
	m.focusPanel(PanelSettings)
	footer = m.renderFooter()
	if !strings.Contains(footer, "SETTINGS") {
		t.Errorf("expected PanelSettings footer to have 'SETTINGS' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Select Setting") {
		t.Errorf("expected PanelSettings footer to have 'Select Setting', got: %s", footer)
	}
	if !strings.Contains(footer, "Adjust Value") {
		t.Errorf("expected PanelSettings footer to have 'Adjust Value', got: %s", footer)
	}
}

func TestFooterResponsiveTruncation(t *testing.T) {
	eng := engine.NewEngine()
	panels := []Panel{PanelInput, PanelQueue, PanelTable, PanelFiles, PanelSettings}
	widths := []int{120, 100, 80, 60, 50, 40}

	for _, w := range widths {
		for _, p := range panels {
			m := InitialModel(eng)
			next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
			m = next.(Model)
			m.focusPanel(p)

			footer := m.renderFooter()
			if strings.Contains(footer, "\n") {
				t.Fatalf("footer contains newline at width %d for panel %d: %q", w, p, footer)
			}

			// Verify mode badge is retained even at narrow widths
			badge, _, _ := m.contextFooterItems()
			if !strings.Contains(footer, badge) {
				t.Errorf("expected footer at width %d to contain mode badge %q, got: %q", w, badge, footer)
			}
		}
	}
}

func TestCommandsChannelClosed(t *testing.T) {
	eng := engine.NewEngine()
	eng.Stop() // closes all channels

	cmdResult := waitForResult(eng.Results)
	if msg := cmdResult(); msg != nil {
		t.Fatalf("expected waitForResult to return nil on closed channel, got %#v", msg)
	}

	cmdFile := waitForFile(eng.Files)
	if msg := cmdFile(); msg != nil {
		t.Fatalf("expected waitForFile to return nil on closed channel, got %#v", msg)
	}

	cmdDiscovered := waitForDiscovered(eng.Discovered)
	if msg := cmdDiscovered(); msg != nil {
		t.Fatalf("expected waitForDiscovered to return nil on closed channel, got %#v", msg)
	}

	cmdAnalysis := waitForAnalysis(eng.Analysis)
	if msg := cmdAnalysis(); msg != nil {
		t.Fatalf("expected waitForAnalysis to return nil on closed channel, got %#v", msg)
	}
}
