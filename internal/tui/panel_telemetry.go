package tui

import (
	"fmt"
	"sort"
	"strings"

	"chameleon/internal/engine"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateVerboseContent(res engine.Result) {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.AccentColor)).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	b.WriteString("\n  ")
	b.WriteString(headerStyle.Render("⚡ REQUEST & TELEMETRY DETAILS"))
	b.WriteString("\n\n")

	displayURL := res.URL
	if displayURL == "" {
		displayURL = res.Name
	}
	method := res.Method
	if method == "" {
		method = "GET"
	}

	writeField := func(label, val string) {
		b.WriteString("  ")
		b.WriteString(labelStyle.Render(label))
		b.WriteString(val)
		b.WriteByte('\n')
	}

	writeField("URL:        ", urlStyle.Render(displayURL))
	writeField("Method:     ", valueStyle.Render(method))

	var statusBadge string
	if res.Status == "ERR" || res.StatusCode >= 400 {
		statusBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render(res.Status)
	} else {
		statusBadge = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("46")).Render(res.Status)
	}
	writeField("Status:     ", statusBadge)
	writeField("Content:    ", valueStyle.Render(res.Type))
	writeField("Size:       ", valueStyle.Render(res.Size))
	writeField("Duration:   ", valueStyle.Render(res.Time))
	if !res.Timestamp.IsZero() {
		writeField("Timestamp:  ", valueStyle.Render(res.Timestamp.Format("2006-01-02 15:04:05.000")))
	}

	if res.ErrorMsg != "" {
		b.WriteString("\n  ")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("─ Error Details ─────────────────────────────────────────"))
		b.WriteString("\n   ")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(res.ErrorMsg))
		b.WriteByte('\n')
	}

	if len(res.ResponseHeaders) > 0 {
		b.WriteString("\n  ")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("248")).Render("─ Response Headers ──────────────────────────────────────"))
		b.WriteString("\n")

		keys := make([]string, 0, len(res.ResponseHeaders))
		for k := range res.ResponseHeaders {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			vals := res.ResponseHeaders[k]
			for _, v := range vals {
				b.WriteString(fmt.Sprintf("   %s: %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.AccentColor)).Render(k), valueStyle.Render(v)))
			}
		}
	}

	b.WriteString("\n  ")
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	b.WriteString(hintStyle.Render("[Esc / Enter / q: Back to Telemetry Table]"))
	b.WriteString("\n")

	m.verboseViewport.SetContent(b.String())
	m.verboseViewport.GotoTop()
}
