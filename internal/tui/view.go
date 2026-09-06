package tui

import (
	"fmt"
	"strconv"
	"strings"

	"chameleon/internal/engine"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) panelStyle(p Panel, w, h int) lipgloss.Style {
	isFocused := m.activePanel == p
	st := lipgloss.NewStyle().
		Width(w - 2).
		Height(h - 2)

	if isFocused {
		return st.
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(m.theme.BorderActive))
	}
	return st.
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(m.theme.BorderInactive))
}

func (m Model) titleStyle(p Panel) lipgloss.Style {
	if m.activePanel == p {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(m.theme.TitleActive))
	}
	return lipgloss.NewStyle().
		Bold(false).
		Foreground(lipgloss.Color(m.theme.TitleInactive))
}

func (m Model) renderPanel(p Panel, title string, rightTitle string, body string, w, h int) string {
	style := m.titleStyle(p)
	styledTitle := style.Render(title)

	var header string
	if rightTitle != "" {
		styledRight := style.Render(rightTitle)
		innerWidth := w - 2
		padding := innerWidth - lipgloss.Width(styledTitle) - lipgloss.Width(styledRight)
		if padding < 0 {
			padding = 0
		}
		header = styledTitle + strings.Repeat(" ", padding) + styledRight
	} else {
		header = styledTitle
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, header, body)

	innerWidth := w - 2
	if innerWidth < 0 {
		innerWidth = 0
	}
	inner = lipgloss.NewStyle().MaxWidth(innerWidth).Render(inner)

	return m.panelStyle(p, w, h).Render(inner)
}

func (m Model) renderHelpModal() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.TitleActive))

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.AccentColor))

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.FooterKey))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	hintStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color(m.theme.FooterDesc))

	row := func(key, desc string) string {
		return fmt.Sprintf("  %-22s %s", keyStyle.Render(key), descStyle.Render(desc))
	}

	content := strings.Join([]string{
		titleStyle.Render(" Chameleon — Help & Keybindings"),
		"",
		sectionStyle.Render("Navigation & Global"),
		row("Ctrl+T / F3", "Toggle layout (Grid vs Tabbed UI)"),
		row("Tab / Shift+Tab", "Cycle focus across panels (Grid mode)"),
		row("? / F1", "Toggle this help menu"),
		row("Ctrl+C", "Quit application"),
		"",
		sectionStyle.Render("Tabbed UI Mode"),
		row("1, 2, 3, 4", "Switch directly to Tab (Queue, Telemetry, Files, Settings)"),
		row("[ / ]", "Cycle to previous / next tab"),
		row("Tab", "Toggle focus between active tab and bottom URL bar"),
		row("Esc", "Return focus from URL bar to active tab"),
		"",
		sectionStyle.Render("Target URL Panel"),
		row("Enter", "Submit and start scraping job"),
		row("Ctrl+A / F2", "Analyze scrapability & ethics"),
		"",
		sectionStyle.Render("Jobs Queue Panel"),
		row("↑/↓ or j/k", "Select job in queue"),
		row("Space / p / Enter", "Pause / resume selected job"),
		row("P (Shift+P)", "Toggle pause / resume all jobs"),
		row("s / Del / Backspace", "Stop and remove job"),
		"",
		sectionStyle.Render("Telemetry & Analysis Reports"),
		row("Enter / i / v", "Inspect selected request details"),
		row("Space / a / t", "Toggle scrapability ethical report"),
		row("Esc / q", "Return to live telemetry table"),
		row("↑/↓ or j/k", "Scroll report or details"),
		"",
		sectionStyle.Render("Engine Settings Panel"),
		row("↑/↓ or j/k", "Select setting item"),
		row("←/→ / Space / Enter", "Adjust or toggle setting value"),
		"",
		hintStyle.Render("Press Esc, q, or ? to close help"),
	}, "\n")

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.BorderActive)).
		Padding(1, 2)

	return modalStyle.Render(content)
}

type footerItem struct {
	Key  string
	Desc string
}

func (m Model) renderModePill(mode string) string {
	bg := m.theme.TableSelectedBg
	if bg == "" {
		bg = "57"
	}
	fg := m.theme.TableSelectedFg
	if fg == "" {
		fg = "229"
	}
	return lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color(bg)).
		Foreground(lipgloss.Color(fg)).
		Padding(0, 1).
		Render(mode)
}

func (m Model) renderFooterItem(item footerItem) string {
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.FooterKey))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.FooterDesc))
	return keyStyle.Render("["+item.Key+"]") + " " + descStyle.Render(item.Desc)
}

func (m Model) contextFooterItems() (string, []footerItem, []footerItem) {
	var modeBadge string
	var contextActions []footerItem
	var globalActions []footerItem

	switch m.activePanel {
	case PanelInput:
		modeBadge = "TARGET URL"
		contextActions = []footerItem{
			{Key: "Enter", Desc: "Start Crawl"},
			{Key: "Ctrl+A / F2", Desc: "Audit Ethics"},
		}
		globalActions = []footerItem{
			{Key: "Tab", Desc: "Next Panel"},
			{Key: "Ctrl+T", Desc: "Tabs View"},
			{Key: "F1", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}

	case PanelQueue:
		modeBadge = "JOBS QUEUE"
		if m.confirmStopURL != "" {
			contextActions = []footerItem{
				{Key: "s", Desc: "Confirm Stop"},
				{Key: "Esc", Desc: "Cancel"},
			}
		} else {
			contextActions = []footerItem{
				{Key: "Space", Desc: "Pause/Resume"},
				{Key: "P", Desc: "Pause All"},
				{Key: "s", Desc: "Stop"},
				{Key: "↑/↓", Desc: "Select"},
			}
		}
		globalActions = []footerItem{
			{Key: "Tab", Desc: "Next Panel"},
			{Key: "Ctrl+T", Desc: "Tabs View"},
			{Key: "?", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}

	case PanelTable:
		switch m.centerMode {
		case CenterViewVerbose:
			modeBadge = "REQUEST DETAILS"
			contextActions = []footerItem{
				{Key: "Esc / q", Desc: "Back to Table"},
				{Key: "a", Desc: "Ethical Report"},
				{Key: "↑/↓", Desc: "Scroll"},
			}
		case CenterViewReport:
			modeBadge = "ETHICAL REPORT"
			contextActions = []footerItem{
				{Key: "Esc / a", Desc: "Back to Table"},
				{Key: "↑/↓", Desc: "Scroll"},
			}
		default: // CenterViewTelemetry
			modeBadge = "TELEMETRY"
			contextActions = []footerItem{
				{Key: "Enter / v", Desc: "Inspect"},
				{Key: "a", Desc: "Ethical Report"},
				{Key: "↑/↓", Desc: "Select"},
			}
		}
		globalActions = []footerItem{
			{Key: "Tab", Desc: "Next Panel"},
			{Key: "Ctrl+T", Desc: "Tabs View"},
			{Key: "?", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}

	case PanelFiles:
		modeBadge = "SAVED FILES"
		contextActions = []footerItem{
			{Key: "↑/↓", Desc: "Scroll Files"},
		}
		globalActions = []footerItem{
			{Key: "Tab", Desc: "Next Panel"},
			{Key: "Ctrl+T", Desc: "Tabs View"},
			{Key: "?", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}

	case PanelSettings:
		modeBadge = "SETTINGS"
		contextActions = []footerItem{
			{Key: "↑/↓", Desc: "Select Setting"},
			{Key: "←/→ / Space", Desc: "Adjust Value"},
		}
		globalActions = []footerItem{
			{Key: "Tab", Desc: "Next Panel"},
			{Key: "Ctrl+T", Desc: "Tabs View"},
			{Key: "?", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}
	}

	return modeBadge, contextActions, globalActions
}

func (m Model) renderFooter() string {
	if m.width <= 0 {
		return ""
	}

	modeBadge, contextActions, globalActions := m.contextFooterItems()
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.FooterSep))
	sep := sepStyle.Render(" • ")

	pill := m.renderModePill(modeBadge)

	// Render items to string slices
	renderItems := func(items []footerItem) []string {
		res := make([]string, len(items))
		for i, it := range items {
			res[i] = m.renderFooterItem(it)
		}
		return res
	}

	renderedCtx := renderItems(contextActions)
	renderedGlobal := renderItems(globalActions)

	buildLine := func(ctxCount, globCount int) (string, int) {
		var leftParts []string
		leftParts = append(leftParts, pill)
		if ctxCount > 0 && ctxCount <= len(renderedCtx) {
			leftParts = append(leftParts, strings.Join(renderedCtx[:ctxCount], sep))
		}
		leftStr := strings.Join(leftParts, "  ")

		var rightStr string
		if globCount > 0 && globCount <= len(renderedGlobal) {
			rightStr = strings.Join(renderedGlobal[:globCount], sep)
		}

		wL := lipgloss.Width(leftStr)
		wR := lipgloss.Width(rightStr)
		totalW := wL
		if wR > 0 {
			totalW += 2 + wR
		}

		if totalW <= m.width {
			spacerWidth := m.width - wL - wR
			if spacerWidth < 0 {
				spacerWidth = 0
			}
			return leftStr + strings.Repeat(" ", spacerWidth) + rightStr, totalW
		}
		return "", totalW
	}

	// 1. Try full context and global items
	for g := len(renderedGlobal); g >= 0; g-- {
		line, _ := buildLine(len(renderedCtx), g)
		if line != "" {
			return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
		}
	}

	// 2. If right side is stripped completely, progressively drop secondary context items
	for c := len(renderedCtx) - 1; c >= 1; c-- {
		line, _ := buildLine(c, 0)
		if line != "" {
			return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
		}
	}

	// 3. Just the pill and primary action, or only the pill if extremely narrow
	line, _ := buildLine(1, 0)
	if line != "" {
		return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
	}

	return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(pill)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelpModal())
	}

	if m.uiMode == UIModeTabbed {
		return m.renderTabbedView()
	}

	l := m.layout
	if l.Width == 0 {
		l = calculateLayout(m.width, m.height)
	}

	// 1. Queue Box
	queueBox := m.renderPanel(PanelQueue, " Jobs Queue ", "("+strconv.Itoa(len(m.queue))+") ", m.queueViewport.View(), l.LeftWidth, l.TopHeight)

	// 2. Center Panel: Telemetry Table, Report Card Box, or Verbose Details Box
	var centerTitle string
	var centerRight string
	var centerBody string
	switch m.centerMode {
	case CenterViewVerbose:
		centerTitle = " Request Details "
		centerBody = m.verboseViewport.View()
	case CenterViewTelemetry:
		var spinStr string
		if m.reducedMotion {
			if m.hasActiveWork() {
				spinStr = "[Active] "
			} else {
				spinStr = "[Idle]   "
			}
		} else if m.hasActiveWork() {
			spinStr = m.spinner.View() + " "
		} else {
			spinStr = "    "
		}
		centerTitle = fmt.Sprintf(" Telemetry Live %s", spinStr)
		centerRight = fmt.Sprintf("(%d/%d Total) ", len(m.table.Rows()), m.totalTelemetry)
		centerBody = m.table.View()
	default:
		centerTitle = " Scrapability & Ethical Report "
		centerBody = m.reportViewport.View()
	}
	telemetryBox := m.renderPanel(PanelTable, centerTitle, centerRight, centerBody, l.CenterWidth, l.TopHeight)

	// 3. Files Box
	filesBox := m.renderPanel(PanelFiles, " Saved Files ", "("+strconv.Itoa(len(m.files))+") ", m.filesViewport.View(), l.RightWidth, l.TopHeight)

	// 4. Input Box
	inputPromptStyle := lipgloss.NewStyle().
		Bold(m.activePanel == PanelInput).
		Foreground(lipgloss.Color(m.theme.AccentColor))
	inputPrompt := inputPromptStyle.Render(" ❯ ") + m.textInput.View()
	var inputInner string
	if m.inputError != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusError)).Bold(true)
		inputInner = lipgloss.JoinVertical(lipgloss.Left, "", inputPrompt, " "+errStyle.Render(m.inputError))
	} else {
		inputInner = lipgloss.JoinVertical(lipgloss.Left, "", inputPrompt)
	}
	inputBox := m.renderPanel(PanelInput, " Target URL ", "", inputInner, l.BottomLeftWidth, l.BottomHeight)

	// 5. Settings Box
	depthStr := fmt.Sprintf("Depth:  < %d >", m.settings.Depth)
	if m.settings.Depth == 1 {
		depthStr += " (Direct page only)"
	} else {
		depthStr += fmt.Sprintf(" (%d levels deep)", m.settings.Depth)
	}

	imgCheck := "[ ] Download Images"
	if m.settings.Images {
		imgCheck = "[x] Download Images"
	}
	imgStr := "Images: " + imgCheck

	speedStr := fmt.Sprintf("Speed:  < %s >", m.settings.Speed)
	if m.settings.Speed == engine.SpeedSafe {
		speedStr += " (1 req/s - Friendly)"
	} else {
		speedStr += " (5 req/s - Aggressive)"
	}

	settingRows := []string{depthStr, imgStr, speedStr}
	for i, r := range settingRows {
		if m.activePanel == PanelSettings && m.settingIndex == i {
			settingRows[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.AccentColor)).Bold(true).Render(" ▶ " + r)
		} else {
			settingRows[i] = "   " + r
		}
	}
	settingsInner := strings.Join(settingRows, "\n")
	settingsBox := m.renderPanel(PanelSettings, " Engine Settings ", "", settingsInner, l.BottomRightWidth, l.BottomHeight)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, queueBox, telemetryBox, filesBox)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, inputBox, settingsBox)
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow, footer)
}
