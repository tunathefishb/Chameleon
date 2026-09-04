package tui

import (
	"fmt"
	"strings"

	"chameleon/internal/engine"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) addQueue(url string) {
	m.queue = append(m.queue, url)
	m.needsQueueUpdate = true
}

func (m *Model) updateQueueContent() {
	if len(m.queue) == 0 {
		m.queueViewport.SetContent("  (no queued jobs)")
		return
	}

	if m.queueCursor >= len(m.queue) {
		m.queueCursor = len(m.queue) - 1
	}
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}

	badgeQueued := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusQueued)).Render("[Queued ]")
	badgeRunning := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.StatusRunning)).Render("[Running]")
	badgePaused := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.StatusPaused)).Render("[Paused ]")
	badgeStopped := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusStopped)).Render("[Stopped]")
	badgeDone := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusDone)).Render("[ Done  ]")
	badgeError := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusError)).Render("[ Error ]")
	badgeDefault := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusQueued)).Render("[Queued ]")

	styleNormalURL := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleSelectedURL := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	prefixSelected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render("▶ ")

	var b strings.Builder
	for i, q := range m.queue {
		status := m.eng.GetJobStatus(q)

		var statusBadge string
		switch status {
		case engine.StatusQueued:
			statusBadge = badgeQueued
		case engine.StatusRunning:
			statusBadge = badgeRunning
		case engine.StatusPaused:
			statusBadge = badgePaused
		case engine.StatusStopped:
			statusBadge = badgeStopped
		case engine.StatusDone:
			statusBadge = badgeDone
		case engine.StatusError:
			statusBadge = badgeError
		default:
			statusBadge = badgeDefault
		}

		isSelected := m.activePanel == PanelQueue && i == m.queueCursor
		prefix := "  "
		urlStyle := styleNormalURL

		if isSelected {
			prefix = prefixSelected
			urlStyle = styleSelectedURL
		}

		// Truncate URL to fit in queue viewport
		maxURLLen := m.queueViewport.Width - 18
		displayURL := q
		if maxURLLen > 8 && len(displayURL) > maxURLLen {
			displayURL = displayURL[:maxURLLen-3] + "..."
		}

		line := fmt.Sprintf("%s%2d. %s %s\n", prefix, i+1, statusBadge, urlStyle.Render(displayURL))
		b.WriteString(line)
	}

	m.queueViewport.SetContent(b.String())

	// Auto-scroll viewport if cursor moved out of visible window
	if m.activePanel == PanelQueue && m.queueViewport.Height > 0 {
		if m.queueCursor < m.queueViewport.YOffset {
			m.queueViewport.YOffset = m.queueCursor
		} else if m.queueCursor >= m.queueViewport.YOffset+m.queueViewport.Height {
			m.queueViewport.YOffset = m.queueCursor - m.queueViewport.Height + 1
		}
	}
}

func (m *Model) updateQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.queueCursor > 0 {
			m.queueCursor--
			m.updateQueueContent()
		}
		return *m, nil
	case "down", "j":
		if m.queueCursor < len(m.queue)-1 {
			m.queueCursor++
			m.updateQueueContent()
		}
		return *m, nil
	case "P":
		m.eng.TogglePauseAll()
		m.updateQueueContent()
		return *m, nil
	case "p", " ", "enter":
		if len(m.queue) > 0 && m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
			targetURL := m.queue[m.queueCursor]
			m.eng.TogglePauseJob(targetURL)
			m.updateQueueContent()
		}
		return *m, nil
	case "s", "d", "delete", "backspace":
		if len(m.queue) > 0 && m.queueCursor >= 0 && m.queueCursor < len(m.queue) {
			targetURL := m.queue[m.queueCursor]
			m.eng.StopJob(targetURL)
			m.updateQueueContent()
		}
		return *m, nil
	}
	var cmd tea.Cmd
	m.queueViewport, cmd = m.queueViewport.Update(msg)
	return *m, cmd
}
