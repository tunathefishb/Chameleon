package main

import (
	"fmt"
	"os"

	"chameleon/internal/engine"
	"chameleon/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	scraperEngine := engine.NewEngine()
	scraperEngine.Start(5) // 5 concurrent workers

	theme := tui.LoadTheme()
	p := tea.NewProgram(tui.InitialModel(scraperEngine, theme), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	scraperEngine.Stop()
}
