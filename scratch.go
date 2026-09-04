package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	header := "12345678901234567890"
	body := "Row 1 is very long and might wrap\nRow 2 is also long"

	inner := lipgloss.JoinVertical(lipgloss.Left, header, body)
	inner = lipgloss.NewStyle().MaxWidth(15).Render(inner)

	panel := lipgloss.NewStyle().Width(15).Border(lipgloss.NormalBorder()).Render(inner)
	fmt.Println("Panel:")
	fmt.Println(panel)
}
