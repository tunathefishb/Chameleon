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

	// Focus Settings
	// Cycle: Input (0) -> Queue (1) -> Table (2) -> Files (3) -> Settings (4)
	for range 4 {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = next.(tui.Model)
	}

	fmt.Println("=== SETTINGS FOCUSED ===")
	fmt.Println(m.View())

	// Shift+Tab back to Files (3)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(tui.Model)
	fmt.Println("=== FILES FOCUSED ===")
	fmt.Println(m.View())

	// Shift+Tab back to Telemetry (2)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(tui.Model)
	fmt.Println("=== TELEMETRY FOCUSED ===")
	fmt.Println(m.View())
}
