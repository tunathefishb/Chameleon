package tui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTabNavigation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.setFocusTarget(FocusTabContent)

	// Direct tab jump 2: Telemetry
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = next.(Model)
	if m.activeTab != TabTelemetry {
		t.Fatalf("expected active tab to be TabTelemetry, got %d", m.activeTab)
	}

	// Direct tab jump 3: Files
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = next.(Model)
	if m.activeTab != TabFiles {
		t.Fatalf("expected active tab to be TabFiles, got %d", m.activeTab)
	}

	// Direct tab jump 4: Settings
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = next.(Model)
	if m.activeTab != TabSettings {
		t.Fatalf("expected active tab to be TabSettings, got %d", m.activeTab)
	}

	// Direct tab jump 1: Queue
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = next.(Model)
	if m.activeTab != TabQueue {
		t.Fatalf("expected active tab to be TabQueue, got %d", m.activeTab)
	}

	// Cycle forward with ]
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = next.(Model)
	if m.activeTab != TabTelemetry {
		t.Fatalf("expected active tab to be TabTelemetry after ], got %d", m.activeTab)
	}

	// Cycle backward with [
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = next.(Model)
	if m.activeTab != TabQueue {
		t.Fatalf("expected active tab to be TabQueue after [, got %d", m.activeTab)
	}

	// Wrap around backward with [
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = next.(Model)
	if m.activeTab != TabSettings {
		t.Fatalf("expected active tab to wrap around to TabSettings after [, got %d", m.activeTab)
	}
}

func TestFocusToggle(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.setFocusTarget(FocusURLInput)
	if !m.textInput.Focused() {
		t.Fatalf("expected textInput to be focused initially")
	}

	// Tab toggles focus to TabContent
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focusTarget != FocusTabContent {
		t.Fatalf("expected focusTarget to be FocusTabContent, got %d", m.focusTarget)
	}
	if m.textInput.Focused() {
		t.Fatalf("expected textInput to be blurred when tab content is focused")
	}

	// Tab toggles focus back to URLInput
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.focusTarget != FocusURLInput {
		t.Fatalf("expected focusTarget to be FocusURLInput, got %d", m.focusTarget)
	}
	if !m.textInput.Focused() {
		t.Fatalf("expected textInput to be focused")
	}

	// Esc returns focus to TabContent
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.focusTarget != FocusTabContent {
		t.Fatalf("expected Esc to return focus to FocusTabContent, got %d", m.focusTarget)
	}

	// Shift+Tab toggles focus
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.focusTarget != FocusURLInput {
		t.Fatalf("expected Shift+Tab to focus URLInput, got %d", m.focusTarget)
	}
}

func TestSettingsInteraction(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

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

	// Test category jump with PageDown (jumps 4 items from 0 -> 4 Workers)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = next.(Model)
	if m.settingIndex != 4 {
		t.Fatalf("expected settingIndex 4 after pgdown, got %d", m.settingIndex)
	}

	// Move down to Speed (Index 5)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.settingIndex != 5 {
		t.Fatalf("expected settingIndex 5, got %d", m.settingIndex)
	}

	// Press enter -> toggle speed to Fast
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.settings.Speed != engine.SpeedFast {
		t.Fatalf("expected Speed to be Fast, got %s", m.settings.Speed)
	}

	// Jump to Output & Appearance category (Index 12: Images)
	m.settingIndex = 12
	if m.settings.Images {
		t.Fatalf("expected initial Images to be false")
	}

	// Press space -> toggle images
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if !m.settings.Images {
		t.Fatalf("expected Images to be true")
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
	m.focusTab(TabTelemetry)
	m.setFocusTarget(FocusTabContent)
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
	if !strings.Contains(view, "1: Queue") {
		t.Errorf("expected view to contain '1: Queue'")
	}
	if !strings.Contains(view, "2: Telemetry") {
		t.Errorf("expected view to contain '2: Telemetry'")
	}
	if !strings.Contains(view, "3: Files") {
		t.Errorf("expected view to contain '3: Files'")
	}
	if !strings.Contains(view, "4: Settings") {
		t.Errorf("expected view to contain '4: Settings'")
	}
	if !strings.Contains(view, "Jobs Queue") {
		t.Errorf("expected view to contain 'Jobs Queue'")
	}
	if !strings.Contains(view, "Target URL") {
		t.Errorf("expected view to contain 'Target URL'")
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

	// Focus TabQueue
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

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

	// Focus TabQueue
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

	url1 := "https://example.com/job1"
	url2 := "https://example.com/job2"
	m.eng.AddJob(url1, m.settings)
	m.addQueue(url1)
	m.eng.AddJob(url2, m.settings)
	m.addQueue(url2)

	if m.eng.GetJobStatus(url1) != engine.StatusQueued || m.eng.GetJobStatus(url2) != engine.StatusQueued {
		t.Fatalf("expected both jobs to be Queued initially")
	}

	// Press 'P' (Shift+P) in TabQueue to pause all jobs
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

	// Focus TabTelemetry
	m.focusTab(TabTelemetry)
	m.setFocusTarget(FocusTabContent)

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

	// In initial state (URL input focused), '?' should go to textInput
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(Model)
	if m.showHelp {
		t.Fatalf("expected showHelp to be false when typing '?' in textInput")
	}
	if !strings.Contains(m.textInput.Value(), "?") {
		t.Fatalf("expected textInput to receive '?'")
	}

	// Pressing F1 in URL input should open help modal
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
	if strings.Contains(view, "Grid") {
		t.Errorf("expected help modal not to contain 'Grid'")
	}

	// Pressing Esc should close help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.showHelp {
		t.Fatalf("expected showHelp to be false after pressing Esc")
	}

	// Switch to TabQueue
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

	// Pressing '?' in TabQueue should open help modal
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = next.(Model)
	if !m.showHelp {
		t.Fatalf("expected showHelp to be true after pressing '?' in TabQueue")
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

	// 1. FocusURLInput
	m.setFocusTarget(FocusURLInput)
	footer := m.renderFooter()
	if !strings.Contains(footer, "TARGET URL") {
		t.Errorf("expected URL input footer to have 'TARGET URL' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Start Crawl") {
		t.Errorf("expected URL input footer to have 'Start Crawl', got: %s", footer)
	}
	if !strings.Contains(footer, "Audit Ethics") {
		t.Errorf("expected URL input footer to have 'Audit Ethics', got: %s", footer)
	}
	if !strings.Contains(footer, "[F1]") {
		t.Errorf("expected URL input footer to have '[F1]', got: %s", footer)
	}

	// 2. TabQueue
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 1: QUEUE") {
		t.Errorf("expected TabQueue footer to have 'TAB 1: QUEUE' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Pause/Resume") {
		t.Errorf("expected TabQueue footer to have 'Pause/Resume', got: %s", footer)
	}
	if !strings.Contains(footer, "Pause All") {
		t.Errorf("expected TabQueue footer to have 'Pause All', got: %s", footer)
	}
	if !strings.Contains(footer, "Stop") {
		t.Errorf("expected TabQueue footer to have 'Stop', got: %s", footer)
	}

	// 3. TabTelemetry (Telemetry Mode)
	m.focusTab(TabTelemetry)
	m.setFocusTarget(FocusTabContent)
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 2: TELEMETRY") {
		t.Errorf("expected TabTelemetry footer to have 'TAB 2: TELEMETRY' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Inspect") {
		t.Errorf("expected TabTelemetry footer to have 'Inspect', got: %s", footer)
	}
	if !strings.Contains(footer, "Ethical Report") {
		t.Errorf("expected TabTelemetry footer to have 'Ethical Report', got: %s", footer)
	}

	// 4. TabTelemetry (Report Mode)
	m.centerMode = CenterViewReport
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 2: REPORT") {
		t.Errorf("expected Report mode footer to have 'TAB 2: REPORT' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Back to Table") {
		t.Errorf("expected Report mode footer to have 'Back to Table', got: %s", footer)
	}

	// 5. TabTelemetry (Verbose Mode)
	m.centerMode = CenterViewVerbose
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 2: DETAILS") {
		t.Errorf("expected Verbose mode footer to have 'TAB 2: DETAILS' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Back to Table") {
		t.Errorf("expected Verbose mode footer to have 'Back to Table', got: %s", footer)
	}

	// 6. TabFiles
	m.focusTab(TabFiles)
	m.setFocusTarget(FocusTabContent)
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 3: FILES") {
		t.Errorf("expected TabFiles footer to have 'TAB 3: FILES' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Scroll Files") {
		t.Errorf("expected TabFiles footer to have 'Scroll Files', got: %s", footer)
	}

	// 7. TabSettings
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)
	footer = m.renderFooter()
	if !strings.Contains(footer, "TAB 4: SETTINGS") {
		t.Errorf("expected TabSettings footer to have 'TAB 4: SETTINGS' badge, got: %s", footer)
	}
	if !strings.Contains(footer, "Select") {
		t.Errorf("expected TabSettings footer to have 'Select', got: %s", footer)
	}
	if !strings.Contains(footer, "Adjust") {
		t.Errorf("expected TabSettings footer to have 'Adjust', got: %s", footer)
	}
}

func TestFooterResponsiveTruncation(t *testing.T) {
	eng := engine.NewEngine()
	widths := []int{120, 100, 80, 60, 50, 40}

	for _, w := range widths {
		// Test URL input
		m := InitialModel(eng)
		next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
		m = next.(Model)
		m.setFocusTarget(FocusURLInput)
		footer := m.renderFooter()
		if strings.Contains(footer, "\n") {
			t.Fatalf("footer contains newline at width %d for URL input: %q", w, footer)
		}
		badge, _, _ := m.footerItems()
		if !strings.Contains(footer, badge) {
			t.Errorf("expected footer at width %d to contain mode badge %q, got: %q", w, badge, footer)
		}

		// Test each tab
		tabs := []ActiveTab{TabQueue, TabTelemetry, TabFiles, TabSettings}
		for _, tab := range tabs {
			m.focusTab(tab)
			m.setFocusTarget(FocusTabContent)
			footer = m.renderFooter()
			if strings.Contains(footer, "\n") {
				t.Fatalf("footer contains newline at width %d for tab %d: %q", w, tab, footer)
			}
			badge, _, _ = m.footerItems()
			if !strings.Contains(footer, badge) {
				t.Errorf("expected footer at width %d to contain mode badge %q, got: %q", w, badge, footer)
			}
		}
	}
}

func TestTabbedViewRendering(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	// Add item to queue to check badge count
	m.addQueue("https://example.com/item1")

	rendered := m.View()

	// Verify tab pills and count badges
	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected multiple lines in rendered view, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "1: Queue") {
		t.Fatalf("expected first line to contain '1: Queue', got %q", lines[0])
	}
	if !strings.HasPrefix(lines[0], " ") {
		t.Fatalf("expected first line to have leading space padding for panel border alignment, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "┌") {
		t.Fatalf("expected second line directly beneath tabs to be panel top border, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "Jobs Queue") {
		t.Fatalf("expected third line to contain panel header 'Jobs Queue', got %q", lines[2])
	}
	if m.layout.TabHeaderHeight != 1 {
		t.Fatalf("expected TabHeaderHeight to be 1, got %d", m.layout.TabHeaderHeight)
	}
	if !strings.Contains(rendered, "1: Queue") {
		t.Fatalf("expected rendered view to contain '1: Queue'")
	}
	if !strings.Contains(rendered, "(1)") {
		t.Fatalf("expected rendered view to contain badge '(1)' for queue")
	}
	if !strings.Contains(rendered, "2: Telemetry") {
		t.Fatalf("expected rendered view to contain '2: Telemetry'")
	}
	if !strings.Contains(rendered, "3: Files") {
		t.Fatalf("expected rendered view to contain '3: Files'")
	}
	if !strings.Contains(rendered, "4: Settings") {
		t.Fatalf("expected rendered view to contain '4: Settings'")
	}

	// Verify bottom 2-line URL bar
	if !strings.Contains(rendered, "Target URL") {
		t.Fatalf("expected rendered view to contain 'Target URL'")
	}
	if !strings.Contains(rendered, "❯") {
		t.Fatalf("expected rendered view to contain input prompt '❯'")
	}
	if strings.Contains(rendered, "❯ >") {
		t.Fatalf("expected rendered view not to contain double prompt '❯ >'")
	}

	// Verify help indicator on the right of the header
	if !strings.Contains(rendered, "[?: Help]") {
		t.Fatalf("expected rendered view to contain '[?: Help]' in header")
	}
	if strings.Contains(rendered, "Grid") {
		t.Fatalf("expected rendered view not to contain 'Grid'")
	}
}

func TestTabbedInteractions(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.setFocusTarget(FocusTabContent)
	m.focusTab(TabSettings)

	// In Settings tab, adjust Depth
	if m.settings.Depth != 1 {
		t.Fatalf("expected initial depth 1, got %d", m.settings.Depth)
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.settings.Depth != 2 {
		t.Fatalf("expected depth 2 after KeyRight, got %d", m.settings.Depth)
	}

	// Switch focus to URL input and submit URL
	m.setFocusTarget(FocusURLInput)
	m.textInput.SetValue("https://testcrawl.com")

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if len(m.queue) == 0 {
		t.Fatalf("expected job to be added to queue on Enter")
	}
	if m.textInput.Value() != "" {
		t.Fatalf("expected textInput to be cleared after Enter")
	}
}

func TestTabbedQueueSelectionAndCursor(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	m.addQueue("https://example.com/one")
	m.addQueue("https://example.com/two")
	m.setFocusTarget(FocusTabContent)
	m.focusTab(TabQueue)

	m.updateQueueContent()
	content := m.queueViewport.View()

	// In TabQueue focused, the selected item must display the cursor '▶'
	if !strings.Contains(content, "▶") {
		t.Fatalf("expected queue viewport content to contain '▶' cursor, got:\n%s", content)
	}

	// Move cursor down
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.queueCursor != 1 {
		t.Fatalf("expected queueCursor 1, got %d", m.queueCursor)
	}
	content = m.queueViewport.View()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) >= 2 && !strings.Contains(lines[1], "▶") {
		t.Fatalf("expected second line to have '▶' cursor after moving down, got:\n%s", content)
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

func TestNormalizeAndValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantNorm    string
		wantHost    string
		expectError bool
	}{
		{
			name:        "bare domain defaults to https",
			input:       "example.com",
			wantNorm:    "https://example.com",
			wantHost:    "example.com",
			expectError: false,
		},
		{
			name:        "explicit http scheme preserved",
			input:       "http://example.com/path",
			wantNorm:    "http://example.com/path",
			wantHost:    "example.com",
			expectError: false,
		},
		{
			name:        "explicit https with port and query",
			input:       "https://sub.domain.org:8080/test?a=1&b=2",
			wantNorm:    "https://sub.domain.org:8080/test?a=1&b=2",
			wantHost:    "sub.domain.org:8080",
			expectError: false,
		},
		{
			name:        "localhost supported",
			input:       "localhost:8080",
			wantNorm:    "https://localhost:8080",
			wantHost:    "localhost:8080",
			expectError: false,
		},
		{
			name:        "http localhost supported",
			input:       "http://localhost",
			wantNorm:    "http://localhost",
			wantHost:    "localhost",
			expectError: false,
		},
		{
			name:        "trimmed whitespace",
			input:       "   https://example.com   ",
			wantNorm:    "https://example.com",
			wantHost:    "example.com",
			expectError: false,
		},
		{
			name:        "empty string fails",
			input:       "",
			expectError: true,
		},
		{
			name:        "whitespace only fails",
			input:       "   ",
			expectError: true,
		},
		{
			name:        "domain without dot or localhost fails",
			input:       "example",
			expectError: true,
		},
		{
			name:        "spaces in host fails",
			input:       "http://foo bar.com",
			expectError: true,
		},
		{
			name:        "pure IPv6 loopback supported",
			input:       "http://[::1]",
			wantNorm:    "http://[::1]",
			wantHost:    "[::1]",
			expectError: false,
		},
		{
			name:        "pure IPv6 with port supported",
			input:       "https://[::1]:8080",
			wantNorm:    "https://[::1]:8080",
			wantHost:    "[::1]:8080",
			expectError: false,
		},
		{
			name:        "schemeless pure IPv6 with port supported",
			input:       "[::1]:8080",
			wantNorm:    "https://[::1]:8080",
			wantHost:    "[::1]:8080",
			expectError: false,
		},
		{
			name:        "mailto scheme fails",
			input:       "mailto:admin@example.com",
			expectError: true,
		},
		{
			name:        "ftp scheme fails",
			input:       "ftp://ftp.example.com/file.zip",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			norm, parsed, err := normalizeAndValidateURL(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("normalizeAndValidateURL(%q) expected error, got norm=%q", tt.input, norm)
				}
			} else {
				if err != nil {
					t.Errorf("normalizeAndValidateURL(%q) unexpected error: %v", tt.input, err)
				}
				if norm != tt.wantNorm {
					t.Errorf("normalizeAndValidateURL(%q) norm = %q, want %q", tt.input, norm, tt.wantNorm)
				}
				if parsed == nil || parsed.Host != tt.wantHost {
					t.Errorf("normalizeAndValidateURL(%q) host = %v, want %q", tt.input, parsed, tt.wantHost)
				}
			}
		})
	}
}

func TestQueueUTF8Truncation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Set viewport width narrow enough to force truncation
	m.queueViewport.Width = 35
	m.queueViewport.Height = 10
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

	// Add multi-byte UTF-8 URL with German, French, Spanish, Cyrillic, and Japanese
	longUTF8URL := "https://example.com/über/café/españa/кириллица/日本語/test"
	m.addQueue(longUTF8URL)
	m.updateQueueContent()

	content := m.queueViewport.View()

	// 1. Must be strictly valid UTF-8
	if !utf8.ValidString(content) {
		t.Fatalf("queue viewport content contains invalid UTF-8 bytes: %q", content)
	}

	// 2. Must not contain Unicode replacement character \ufffd
	if strings.ContainsRune(content, '\ufffd') {
		t.Fatalf("queue viewport content contains unicode replacement char: %q", content)
	}

	// 3. Must contain ellipsis
	if !strings.Contains(content, "...") {
		t.Fatalf("expected truncated URL in queue to contain '...', got:\n%s", content)
	}
}

func TestAddQueueDirtyFlagBatching(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	if m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be initially false")
	}

	// addQueue should only append to queue and set needsQueueUpdate = true
	// without synchronously updating queueViewport content
	testURL := "https://example.com/batch-test"
	m.addQueue(testURL)

	if !m.needsQueueUpdate {
		t.Fatalf("expected addQueue to set needsQueueUpdate = true")
	}

	// Viewport content has not updated synchronously yet
	if strings.Contains(m.queueViewport.View(), testURL) {
		t.Fatalf("expected queueViewport not to be synchronously rendered before tick")
	}

	// Simulate spinner.TickMsg which batches the update
	next, _ = m.Update(m.spinner.Tick())
	m = next.(Model)

	if m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be reset to false after TickMsg")
	}

	if !strings.Contains(m.queueViewport.View(), testURL) {
		t.Fatalf("expected queueViewport to contain %q after TickMsg, got:\n%s", testURL, m.queueViewport.View())
	}
}

