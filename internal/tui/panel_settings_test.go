package tui

import (
	"strings"
	"testing"

	"chameleon/internal/engine"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsAll16ItemsNavigation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	if m.settingIndex != 0 {
		t.Fatalf("expected initial settingIndex 0, got %d", m.settingIndex)
	}

	// Move through all 16 items with down arrow
	for expected := 1; expected < 16; expected++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
		if m.settingIndex != expected {
			t.Fatalf("expected settingIndex %d, got %d", expected, m.settingIndex)
		}
	}

	// Wrap around to 0
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.settingIndex != 0 {
		t.Fatalf("expected wrap around to 0, got %d", m.settingIndex)
	}

	// Move backwards with up arrow to 15
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	if m.settingIndex != 15 {
		t.Fatalf("expected wrap around to 15, got %d", m.settingIndex)
	}

	// Jump to top with home / g
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)
	if m.settingIndex != 0 {
		t.Fatalf("expected g to jump to index 0, got %d", m.settingIndex)
	}

	// Jump to bottom with end / G
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(Model)
	if m.settingIndex != 15 {
		t.Fatalf("expected G to jump to index 15, got %d", m.settingIndex)
	}
}

func TestSettingsCategoryJumps(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	// PgDown jumps by 4 items (next category)
	expectedSequence := []int{4, 8, 12, 0}
	for _, expected := range expectedSequence {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(Model)
		if m.settingIndex != expected {
			t.Fatalf("expected settingIndex %d after pgdown, got %d", expected, m.settingIndex)
		}
	}

	// PgUp jumps back by 4 items (previous category)
	expectedBackSequence := []int{12, 8, 4, 0}
	for _, expected := range expectedBackSequence {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m = next.(Model)
		if m.settingIndex != expected {
			t.Fatalf("expected settingIndex %d after pgup, got %d", expected, m.settingIndex)
		}
	}
}

func TestSettingsLiveThemeSwitching(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	// Setting 14 is Theme
	m.settingIndex = 14
	if m.settingsState.ThemeIndex != 0 {
		t.Fatalf("expected initial theme index 0, got %d", m.settingsState.ThemeIndex)
	}
	initialBorder := m.theme.BorderActive

	// Press Right -> switch to WCAG Light AA (Index 1)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.settingsState.ThemeIndex != 1 {
		t.Fatalf("expected theme index 1, got %d", m.settingsState.ThemeIndex)
	}
	if m.theme.BorderActive == initialBorder {
		t.Fatalf("expected theme to update, but BorderActive remained %s", initialBorder)
	}

	// Press Right -> switch to High-Contrast ANSI (Index 2)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if m.settingsState.ThemeIndex != 2 {
		t.Fatalf("expected theme index 2, got %d", m.settingsState.ThemeIndex)
	}
	if m.theme.BorderActive != "15" {
		t.Fatalf("expected accessible theme border active '15', got %s", m.theme.BorderActive)
	}
}

func TestSettingsLiveReducedMotionToggle(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	// Setting 15 is Motion
	m.settingIndex = 15
	if m.reducedMotion {
		t.Fatalf("expected initial reducedMotion to be false")
	}

	// Press Space -> toggle on
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if !m.reducedMotion {
		t.Fatalf("expected reducedMotion to be true")
	}

	// Press Space -> toggle off
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if m.reducedMotion {
		t.Fatalf("expected reducedMotion to be false")
	}
}

func TestSettingsAdjustAllItems(t *testing.T) {
	s := DefaultSettingsState()

	// Adjust all 16 items forward and backward
	for i := 0; i < 16; i++ {
		itemBefore := s.GetItem(i)
		s.Adjust(i, 1)
		itemAfter := s.GetItem(i)
		if itemBefore.Value == itemAfter.Value {
			t.Errorf("expected item %d (%s) value to change after Adjust(1)", i, itemBefore.Label)
		}

		s.Adjust(i, -1)
		itemReverted := s.GetItem(i)
		if itemBefore.Value != itemReverted.Value {
			t.Errorf("expected item %d (%s) value to revert after Adjust(-1), before: %s, reverted: %s",
				i, itemBefore.Label, itemBefore.Value, itemReverted.Value)
		}
	}
}

func TestSettingsRenderingWideAndNarrow(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	// Wide render (100 cols, 25 rows)
	wideRender := m.renderSettingsPanel(100, 25)
	if !strings.Contains(wideRender, "Engine Settings") {
		t.Errorf("expected wide render to contain 'Engine Settings'")
	}
	if !strings.Contains(wideRender, "[1] Crawl & Scope") {
		t.Errorf("expected wide render to contain '[1] Crawl & Scope'")
	}
	if !strings.Contains(wideRender, "[4] Output & Appearance") {
		t.Errorf("expected wide render to contain '[4] Output & Appearance'")
	}
	if !strings.Contains(wideRender, "Setting Info:") {
		t.Errorf("expected wide render to contain 'Setting Info:'")
	}

	// Narrow render (60 cols, 25 rows)
	narrowRender := m.renderSettingsPanel(60, 25)
	if !strings.Contains(narrowRender, "Engine Settings") {
		t.Errorf("expected narrow render to contain 'Engine Settings'")
	}
}

func TestVisualSettingsPreview(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.width = 100
	m.height = 25
	m.focusTab(TabSettings)
	m.setFocusTarget(FocusTabContent)

	view := m.View()
	t.Logf("\n=== SETTINGS TAB VISUAL PREVIEW ===\n%s\n", view)

	if !strings.Contains(view, "4: Settings") {
		t.Errorf("expected view to contain '4: Settings'")
	}
	if !strings.Contains(view, "[1] Crawl & Scope") {
		t.Errorf("expected view to contain category 1")
	}
}
