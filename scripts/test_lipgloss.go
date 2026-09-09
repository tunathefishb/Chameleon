//go:build ignore

package main

import (
	"fmt"

	"chameleon/internal/engine"
	"chameleon/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	eng := engine.NewEngine()
	m := tui.InitialModel(eng)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 25})
	m = next.(tui.Model)

	// Toggle focus from URL input to Tab Content
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(tui.Model)

	// Navigate to Settings (Tab 4)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = next.(tui.Model)

	fmt.Println("=== SETTINGS TAB ===")
	fmt.Println(m.View())

	// Cycle back to Files (Tab 3) via '['
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = next.(tui.Model)
	fmt.Println("=== FILES TAB ===")
	fmt.Println(m.View())

	// Jump directly to Telemetry (Tab 2)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = next.(tui.Model)
	fmt.Println("=== TELEMETRY TAB ===")
	fmt.Println(m.View())

	// Focus bottom URL bar via Tab
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(tui.Model)
	fmt.Println("=== URL BAR FOCUSED ===")
	fmt.Println(m.View())
}
