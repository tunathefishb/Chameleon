package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) startAnalysis(targetURL string) tea.Cmd {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return nil
	}
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}
	m.isAnalyzing = true
	m.analyzingURL = targetURL
	m.centerMode = CenterViewReport
	m.updateReportContent()
	m.eng.AnalyzeURL(targetURL)
	return m.startSpinnerCmd()
}

func (m *Model) updateReportContent() {
	if m.isAnalyzing {
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render("  ⚡ RUNNING SCRAPABILITY & ETHICAL AUDIT"))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("  Target: " + m.analyzingURL))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render("  • [1/5] Checking robots.txt rules & sitemap index...\n"))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render("  • [2/5] Inspecting response headers for WAF & Bot defenses...\n"))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render("  • [3/5] Testing DOM structure for Single Page App (SPA) dependencies...\n"))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render("  • [4/5] Scanning for class obfuscation & crawler honeypots...\n"))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render("  • [5/5] Checking passive rate limit telemetry...\n\n"))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("  Please wait while asynchronous checks complete..."))
		m.reportViewport.SetContent(b.String())
		return
	}

	if m.latestReport == nil {
		m.reportViewport.SetContent("\n  (No analysis run yet. Enter a URL and press Ctrl+A to test scrapability & ethics)")
		return
	}

	r := m.latestReport
	var b strings.Builder

	// Header banner
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.AccentColor)).Underline(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	b.WriteString("\n  ")
	b.WriteString(headerStyle.Render("TARGET: "))
	b.WriteString(urlStyle.Render(r.URL))
	b.WriteString("  ")
	b.WriteString(metaStyle.Render(fmt.Sprintf("(%dms)", r.Duration.Milliseconds())))
	b.WriteString("\n\n")

	// Metric Badges
	// 1. Grade Badge
	var gradeColor string
	switch r.EthicalGrade {
	case "A":
		gradeColor = "46" // Bright Green
	case "B":
		gradeColor = "82" // Light Green
	case "C":
		gradeColor = "220" // Yellow
	case "D":
		gradeColor = "208" // Orange
	default:
		gradeColor = "196" // Red
	}

	gradeBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color(gradeColor)).
		Padding(0, 1).
		Render(fmt.Sprintf("ETHICAL GRADE: %s (%d/100)", r.EthicalGrade, r.EthicalScore))

	// 2. Difficulty Meter
	var diffColor string
	switch r.DifficultyLevel {
	case "Easy":
		diffColor = "46"
	case "Moderate":
		diffColor = "220"
	case "Hard":
		diffColor = "208"
	default:
		diffColor = "196"
	}

	meterFilled := strings.Repeat("■", r.DifficultyScore)
	meterEmpty := strings.Repeat("□", 10-r.DifficultyScore)
	diffBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(diffColor)).
		Render(fmt.Sprintf("DIFFICULTY: [%s%s] %d/10 (%s)", meterFilled, meterEmpty, r.DifficultyScore, r.DifficultyLevel))

	b.WriteString("  ")
	b.WriteString(gradeBadge)
	b.WriteString("   ")
	b.WriteString(diffBadge)
	b.WriteString("\n\n")

	// Section 1: Ethical & Policy Audit
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("248")).Render("─ Ethical & Access Policies ─────────────────────────────"))
	b.WriteString("\n")
	for _, item := range r.EthicalDetails {
		b.WriteString("   ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Section 2: Technical & Anti-Bot Defenses
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("248")).Render("─ Technical Defenses & DOM Structure ────────────────────"))
	b.WriteString("\n")
	for _, item := range r.DifficultyDetails {
		b.WriteString("   ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("   ℹ️ ")
	b.WriteString(r.RateLimitInfo)
	b.WriteString("\n\n")

	// Section 3: Recommendation
	recBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.AccentColor)).
		Padding(0, 1).
		Foreground(lipgloss.Color("230")).
		Render("💡 RECOMMENDATION:\n" + r.Recommendation)

	b.WriteString("  ")
	b.WriteString(recBox)
	b.WriteString("\n")

	m.reportViewport.SetContent(b.String())
}
