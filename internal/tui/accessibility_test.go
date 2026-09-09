package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	tea "github.com/charmbracelet/bubbletea"
)

func TestURLInputValidationAccessibility(t *testing.T) {
	eng := engine.NewEngine()
	defer eng.Stop()

	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)
	m.setFocusTarget(FocusURLInput)

	// 1. Test invalid URL submission (spaces)
	m.textInput.SetValue("not a valid url")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil {
		t.Fatalf("expected nil cmd for invalid URL submission")
	}
	if m.inputError == "" {
		t.Fatalf("expected inputError to be populated for invalid URL")
	}
	if len(m.queue) != 0 {
		t.Fatalf("expected queue to remain empty, got %d items", len(m.queue))
	}
	view := m.View()
	if !strings.Contains(view, "Please enter a valid URL") {
		t.Errorf("expected view to contain accessible error message, got view:\n%s", view)
	}

	// 2. Typing clears error
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = next.(Model)
	if m.inputError != "" {
		t.Errorf("expected inputError to clear upon typing, got %q", m.inputError)
	}

	// 3. Test valid URL submission
	m.textInput.SetValue("example.com")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.inputError != "" {
		t.Errorf("expected no inputError for valid URL, got %q", m.inputError)
	}
	if len(m.queue) != 1 {
		t.Fatalf("expected 1 queued item, got %d", len(m.queue))
	}
	if m.queue[0] != "https://example.com" {
		t.Errorf("expected normalized URL https://example.com, got %s", m.queue[0])
	}
}

func TestQueueStopConfirmationSafeguard(t *testing.T) {
	eng := engine.NewEngine()
	defer eng.Stop()

	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	url1 := "https://example.com/one"
	url2 := "https://example.com/two"
	m.addQueue(url1)
	m.addQueue(url2)
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

	// Move to url1
	m.queueCursor = 0

	// 1. First 's' primes confirmation
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.confirmStopURL != url1 {
		t.Fatalf("expected confirmStopURL to be %s, got %s", url1, m.confirmStopURL)
	}
	if m.eng.GetJobStatus(url1) == engine.StatusStopped {
		t.Fatalf("job should not be stopped yet")
	}
	view := m.View()
	if !strings.Contains(view, "Confirm Stop") {
		t.Errorf("expected view to indicate confirmation state, got:\n%s", view)
	}

	// 2. Pressing Esc cancels confirmation
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.confirmStopURL != "" {
		t.Fatalf("expected confirmStopURL to be cleared after Esc, got %s", m.confirmStopURL)
	}

	// 3. First 's' primes, down arrow cancels
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.confirmStopURL != url1 {
		t.Fatalf("expected confirmStopURL to be %s", url1)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.confirmStopURL != "" {
		t.Fatalf("expected confirmStopURL to be cleared after navigation, got %s", m.confirmStopURL)
	}

	// 4. Two consecutive 's' presses stop the job
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.confirmStopURL != url2 {
		t.Fatalf("expected confirmStopURL to be %s", url2)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.confirmStopURL != "" {
		t.Fatalf("expected confirmStopURL to be cleared after stop execution")
	}
	if m.eng.GetJobStatus(url2) != engine.StatusStopped {
		t.Fatalf("expected url2 status to be Stopped, got %s", m.eng.GetJobStatus(url2))
	}
}

func TestSpinnerIdleGatingAndReducedMotion(t *testing.T) {
	eng := engine.NewEngine()
	defer eng.Stop()

	// 1. Idle gating: when no jobs are active, spinner tick stops
	m := InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = next.(Model)

	if m.hasActiveWork() {
		t.Fatalf("expected hasActiveWork to be false initially")
	}

	// Simulate a tick when idle: spinnerRunning should be false and no new tick command returned
	next, _ = m.Update(tea.Msg(m.spinner.Tick()))
	m = next.(Model)
	if m.spinnerRunning {
		t.Fatalf("expected spinnerRunning to be false when idle")
	}

	// 2. When analysis starts, spinner starts
	m.isAnalyzing = true
	if !m.hasActiveWork() {
		t.Fatalf("expected hasActiveWork to be true while analyzing")
	}

	// 3. When analysis finishes, idle is restored
	reportMsg := engineAnalysisMsg(analyzer.Report{
		URL:          "https://example.com",
		EthicalGrade: "A",
		EthicalScore: 90,
	})
	next, _ = m.Update(reportMsg)
	m = next.(Model)
	if m.isAnalyzing {
		t.Fatalf("expected isAnalyzing to be false after report")
	}

	// 4. Reduced Motion Mode
	m.reducedMotion = true
	m.focusTab(TabTelemetry)
	m.centerMode = CenterViewTelemetry
	if cmd := m.startSpinnerCmd(); cmd != nil {
		t.Fatalf("expected nil cmd when reducedMotion is active")
	}
	m.lastTelemetry = time.Now() // simulate recent activity
	view := m.View()
	if !strings.Contains(view, "[Active]") {
		t.Errorf("expected view to contain static [Active] badge in reduced motion mode, got:\n%s", view)
	}
}

func TestAccessibleThemeTokens(t *testing.T) {
	theme := DefaultTheme()

	// Verify contrast enhancements in DefaultTheme
	if theme.FooterSep != "246" {
		t.Errorf("expected FooterSep to be 246 (>= 4.5:1 contrast vs #000000), got %s", theme.FooterSep)
	}
	if theme.StatusStopped != "208" {
		t.Errorf("expected StatusStopped to be 208 (orange distinct from error red), got %s", theme.StatusStopped)
	}

	// Find config path from current working directory
	lightThemePath := "configs/theme.light.json5"
	if _, err := os.Stat(lightThemePath); err != nil {
		lightThemePath = "../../configs/theme.light.json5"
	}
	if _, err := os.Stat(lightThemePath); err != nil {
		t.Fatalf("configs/theme.light.json5 not found: %v", err)
	}
	lightTheme := LoadTheme(lightThemePath)
	if lightTheme.BorderActive != "24" {
		t.Errorf("expected light theme BorderActive 24, got %s", lightTheme.BorderActive)
	}
	if lightTheme.StatusDone != "28" {
		t.Errorf("expected light theme StatusDone 28, got %s", lightTheme.StatusDone)
	}

	// Verify Accessible High Contrast Theme file exists and loads
	accThemePath := "configs/theme.accessible.json5"
	if _, err := os.Stat(accThemePath); err != nil {
		accThemePath = "../../configs/theme.accessible.json5"
	}
	if _, err := os.Stat(accThemePath); err != nil {
		t.Fatalf("configs/theme.accessible.json5 not found: %v", err)
	}
	accTheme := LoadTheme(accThemePath)
	if accTheme.BorderActive != "15" {
		t.Errorf("expected accessible theme BorderActive 15, got %s", accTheme.BorderActive)
	}
	if accTheme.TableSelectedFg != "0" || accTheme.TableSelectedBg != "15" {
		t.Errorf("expected high contrast inverted selection (0 on 15), got %s on %s",
			accTheme.TableSelectedFg, accTheme.TableSelectedBg)
	}
}
