package tui

import (
	"strings"
	"testing"

	"chameleon/internal/engine"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUIModeAutoDetection(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Default uninitialized is Grid
	if m.uiMode != UIModeGrid {
		t.Fatalf("expected initial uiMode to be UIModeGrid, got %d", m.uiMode)
	}

	// Small screen: 80x24 -> Tabbed
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected 80x24 to trigger UIModeTabbed, got %d", m.uiMode)
	}

	// Medium screen: 100x24 (width < 120) -> Tabbed
	next, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = next.(Model)
	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected 100x24 to trigger UIModeTabbed, got %d", m.uiMode)
	}

	// Short screen: 140x20 (height < 28) -> Tabbed
	next, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 20})
	m = next.(Model)
	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected 140x20 to trigger UIModeTabbed, got %d", m.uiMode)
	}

	// Large display: 140x40 -> Grid
	next, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(Model)
	if m.uiMode != UIModeGrid {
		t.Fatalf("expected 140x40 to trigger UIModeGrid, got %d", m.uiMode)
	}
}

func TestUIModeManualToggle(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Set large window
	next, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = next.(Model)
	if m.uiMode != UIModeGrid {
		t.Fatalf("expected Grid mode for large window")
	}

	// Manual toggle with Ctrl+T
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	m = next.(Model)
	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected Ctrl+T to switch to Tabbed mode, got %d", m.uiMode)
	}
	if !m.manualUIMode {
		t.Fatalf("expected manualUIMode to be true after manual toggle")
	}

	// Subsequent resize should NOT override manual preference
	next, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	m = next.(Model)
	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected Tabbed mode to be preserved despite large resize")
	}

	// Manual toggle with F3
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyF3})
	m = next.(Model)
	if m.uiMode != UIModeGrid {
		t.Fatalf("expected F3 to toggle back to Grid mode, got %d", m.uiMode)
	}
}

func TestTabbedNavigation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Switch to Tabbed mode and focus tab content
	m.uiMode = UIModeTabbed
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

func TestTabbedFocusToggle(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.uiMode = UIModeTabbed
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
}

func TestTabbedViewRendering(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Configure window for tabbed mode
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	if m.uiMode != UIModeTabbed {
		t.Fatalf("expected Tabbed mode on 80x24")
	}

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

	// Verify Ctrl+T toggle indicator in header
	if !strings.Contains(rendered, "[Ctrl+T: Grid]") {
		t.Fatalf("expected rendered view to contain '[Ctrl+T: Grid]' in header")
	}

	// At width 100, verify full footer includes 'Grid View'
	next, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = next.(Model)
	rendered100 := m.View()
	if !strings.Contains(rendered100, "Grid View") {
		t.Fatalf("expected rendered view at width 100 to contain 'Grid View' in footer")
	}
}

func TestTabbedInteractions(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.uiMode = UIModeTabbed
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

func TestTabbedShiftTabFocusToggle(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.uiMode = UIModeTabbed
	m.setFocusTarget(FocusURLInput)

	// Shift+Tab toggles focus to TabContent
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.focusTarget != FocusTabContent {
		t.Fatalf("expected Shift+Tab to focus TabContent, got %d", m.focusTarget)
	}

	// Shift+Tab toggles focus back to URLInput
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(Model)
	if m.focusTarget != FocusURLInput {
		t.Fatalf("expected Shift+Tab to focus URLInput, got %d", m.focusTarget)
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

	// In Tabbed mode with TabQueue focused, the selected item must display the cursor '▶'
	if !strings.Contains(content, "▶") {
		t.Fatalf("expected queue viewport content to contain '▶' cursor in Tabbed mode, got:\n%s", content)
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
