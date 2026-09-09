package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		row("1, 2, 3, 4", "Switch directly to Tab (Queue, Telemetry, Files, Settings)"),
		row("[ / ]", "Cycle to previous / next tab"),
		row("Tab / Shift+Tab", "Toggle focus between active tab and bottom URL bar"),
		row("Esc", "Return focus from URL bar to active tab"),
		row("? / F1", "Toggle this help menu"),
		row("Ctrl+C", "Quit application"),
		"",
		sectionStyle.Render("Target URL (Bottom Bar)"),
		row("Enter", "Submit and start scraping job"),
		row("Ctrl+A / F2", "Analyze scrapability & ethics"),
		"",
		sectionStyle.Render("Jobs Queue (Tab 1)"),
		row("↑/↓ or j/k", "Select job in queue"),
		row("Space / p / Enter", "Pause / resume selected job"),
		row("P (Shift+P)", "Toggle pause / resume all jobs"),
		row("s / Del / Backspace", "Stop and remove job"),
		"",
		sectionStyle.Render("Telemetry & Analysis (Tab 2)"),
		row("Enter / i / v", "Inspect selected request details"),
		row("Space / a / t", "Toggle scrapability ethical report"),
		row("Esc / q", "Return to live telemetry table"),
		row("↑/↓ or j/k", "Scroll report or details"),
		"",
		sectionStyle.Render("Saved Files (Tab 3)"),
		row("↑/↓ or j/k", "Scroll saved files"),
		row("g / G (Home/End)", "Jump to top / bottom"),
		"",
		sectionStyle.Render("Engine Settings (Tab 4)"),
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

func (m Model) footerItems() (string, []footerItem, []footerItem) {
	var modeBadge string
	var contextActions []footerItem
	var globalActions []footerItem

	if m.focusTarget == FocusURLInput {
		modeBadge = "TARGET URL"
		contextActions = []footerItem{
			{Key: "Enter", Desc: "Start Crawl"},
			{Key: "Ctrl+A / F2", Desc: "Audit Ethics"},
			{Key: "Tab / Esc", Desc: "Back to Tab"},
		}
		globalActions = []footerItem{
			{Key: "F1", Desc: "Help"},
			{Key: "Ctrl+C", Desc: "Quit"},
		}
		return modeBadge, contextActions, globalActions
	}

	switch m.activeTab {
	case TabQueue:
		modeBadge = "TAB 1: QUEUE"
		if m.confirmStopURL != "" {
			contextActions = []footerItem{
				{Key: "s", Desc: "Confirm Stop"},
				{Key: "Esc", Desc: "Cancel"},
				{Key: "Tab", Desc: "Focus URL"},
			}
		} else {
			contextActions = []footerItem{
				{Key: "Space", Desc: "Pause/Resume"},
				{Key: "P", Desc: "Pause All"},
				{Key: "s", Desc: "Stop"},
				{Key: "↑/↓", Desc: "Select"},
				{Key: "Tab", Desc: "Focus URL"},
			}
		}
	case TabTelemetry:
		switch m.centerMode {
		case CenterViewVerbose:
			modeBadge = "TAB 2: DETAILS"
			contextActions = []footerItem{
				{Key: "Esc / q", Desc: "Back to Table"},
				{Key: "a", Desc: "Ethical Report"},
				{Key: "↑/↓", Desc: "Scroll"},
				{Key: "Tab", Desc: "Focus URL"},
			}
		case CenterViewReport:
			modeBadge = "TAB 2: REPORT"
			contextActions = []footerItem{
				{Key: "Esc / a", Desc: "Back to Table"},
				{Key: "↑/↓", Desc: "Scroll"},
				{Key: "Tab", Desc: "Focus URL"},
			}
		default:
			modeBadge = "TAB 2: TELEMETRY"
			contextActions = []footerItem{
				{Key: "Enter / v", Desc: "Inspect"},
				{Key: "a", Desc: "Ethical Report"},
				{Key: "↑/↓", Desc: "Select"},
				{Key: "Tab", Desc: "Focus URL"},
			}
		}
	case TabFiles:
		modeBadge = "TAB 3: FILES"
		contextActions = []footerItem{
			{Key: "↑/↓", Desc: "Scroll Files"},
			{Key: "Tab", Desc: "Focus URL"},
		}
	case TabSettings:
		modeBadge = "TAB 4: SETTINGS"
		contextActions = []footerItem{
			{Key: "↑/↓", Desc: "Select"},
			{Key: "←/→ / Space", Desc: "Adjust"},
			{Key: "Tab", Desc: "Focus URL"},
		}
	}

	globalActions = []footerItem{
		{Key: "1-4 / [ ]", Desc: "Tabs"},
		{Key: "?", Desc: "Help"},
		{Key: "Ctrl+C", Desc: "Quit"},
	}

	return modeBadge, contextActions, globalActions
}

func (m Model) renderFooter() string {
	if m.width <= 0 {
		return ""
	}

	modeBadge, contextActions, globalActions := m.footerItems()
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.FooterSep))
	sep := sepStyle.Render(" • ")

	pill := m.renderModePill(modeBadge)

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

	// 1. Try full context with as many global items as possible (at least 1)
	for g := len(renderedGlobal); g >= 1; g-- {
		line, _ := buildLine(len(renderedCtx), g)
		if line != "" {
			return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
		}
	}

	// 2. Try reducing context items while keeping at least 1 global item
	for c := len(renderedCtx) - 1; c >= 1; c-- {
		for g := len(renderedGlobal); g >= 1; g-- {
			line, _ := buildLine(c, g)
			if line != "" {
				return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
			}
		}
	}

	// 3. If still no room, drop global items completely and show as many context items as fit
	for c := len(renderedCtx); c >= 1; c-- {
		line, _ := buildLine(c, 0)
		if line != "" {
			return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
		}
	}

	line, _ := buildLine(1, 0)
	if line != "" {
		return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(line)
	}

	return lipgloss.NewStyle().MaxWidth(m.width).Inline(true).Render(pill)
}

func (m Model) renderTabBar() string {
	type tabInfo struct {
		id    ActiveTab
		title string
		badge string
	}

	tabs := []tabInfo{
		{id: TabQueue, title: "1: Queue", badge: strconv.Itoa(len(m.queue))},
		{id: TabTelemetry, title: "2: Telemetry", badge: strconv.Itoa(len(m.table.Rows()))},
		{id: TabFiles, title: "3: Files", badge: strconv.Itoa(len(m.savedFiles))},
		{id: TabSettings, title: "4: Settings", badge: ""},
	}

	var renderedTabs []string
	for _, t := range tabs {
		isActive := m.activeTab == t.id

		var label string
		if t.badge != "" {
			label = fmt.Sprintf(" %s (%s) ", t.title, t.badge)
		} else {
			label = fmt.Sprintf(" %s ", t.title)
		}

		if isActive {
			bg := m.theme.AccentColor
			if bg == "" {
				bg = "39"
			}
			st := lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color(bg)).
				Foreground(lipgloss.Color("0")).
				MarginRight(1)
			renderedTabs = append(renderedTabs, st.Render(label))
		} else {
			st := lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.theme.TitleInactive)).
				Background(lipgloss.Color(m.theme.BorderInactive)).
				MarginRight(1)
			renderedTabs = append(renderedTabs, st.Render(label))
		}
	}

	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)

	// Help indicator on the right of the tab bar
	helpPill := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.FooterDesc)).
		Italic(true).
		Render("[?: Help]")

	totalTabWidth := lipgloss.Width(tabLine)
	helpWidth := lipgloss.Width(helpPill)

	// Inset by 1 space on left and right to align perfectly with the central panel
	// borders and inner content area (columns 1 to w-2).
	leftPad := " "
	rightPad := " "
	spacerWidth := m.width - totalTabWidth - helpWidth - len(leftPad) - len(rightPad)
	if spacerWidth < 1 {
		spacerWidth = 1
	}

	headerLine := leftPad + tabLine + strings.Repeat(" ", spacerWidth) + helpPill + rightPad
	if lipgloss.Width(headerLine) > m.width {
		if lipgloss.Width(leftPad+tabLine) <= m.width {
			headerLine = leftPad + tabLine
		} else {
			headerLine = tabLine
		}
	}

	return lipgloss.NewStyle().MaxWidth(m.width).Render(headerLine)
}

func (m Model) renderTabContentPanel(title, rightTitle, body string, w, h int) string {
	isFocused := m.focusTarget == FocusTabContent

	titleStyle := lipgloss.NewStyle().
		Bold(isFocused).
		Foreground(lipgloss.Color(m.theme.TitleActive))
	if !isFocused {
		titleStyle = lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color(m.theme.TitleInactive))
	}

	styledTitle := titleStyle.Render(title)

	var header string
	innerWidth := w - 2
	if innerWidth < 0 {
		innerWidth = 0
	}

	if rightTitle != "" {
		styledRight := titleStyle.Render(rightTitle)
		padding := innerWidth - lipgloss.Width(styledTitle) - lipgloss.Width(styledRight)
		if padding < 0 {
			padding = 0
		}
		header = styledTitle + strings.Repeat(" ", padding) + styledRight
	} else {
		header = styledTitle
	}

	inner := lipgloss.JoinVertical(lipgloss.Left, header, body)
	inner = lipgloss.NewStyle().MaxWidth(innerWidth).Render(inner)

	panelSt := lipgloss.NewStyle().
		Width(innerWidth).
		Height(h - 2)

	if isFocused {
		panelSt = panelSt.
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color(m.theme.BorderActive))
	} else {
		panelSt = panelSt.
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(m.theme.BorderInactive))
	}

	return panelSt.Render(inner)
}

func (m Model) renderBottomURLBar(w int) string {
	isFocused := m.focusTarget == FocusURLInput

	borderColor := m.theme.BorderInactive
	titleColor := m.theme.TitleInactive
	if isFocused {
		borderColor = m.theme.BorderActive
		titleColor = m.theme.TitleActive
	}

	// Line 1: Top divider border with title
	titleText := " Target URL "
	styledTitle := lipgloss.NewStyle().
		Bold(isFocused).
		Foreground(lipgloss.Color(titleColor)).
		Render(titleText)

	dashStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	leftDashes := dashStyle.Render("───")
	leftLen := lipgloss.Width(leftDashes) + lipgloss.Width(styledTitle)

	rightLen := w - leftLen
	if rightLen < 0 {
		rightLen = 0
	}
	rightDashes := dashStyle.Render(strings.Repeat("─", rightLen))
	line1 := leftDashes + styledTitle + rightDashes

	// Line 2: Input prompt with cursor/input
	promptStyle := lipgloss.NewStyle().
		Bold(isFocused).
		Foreground(lipgloss.Color(m.theme.AccentColor))
	prompt := promptStyle.Render(" ❯ ")

	var errStr string
	errLen := 0
	if m.inputError != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.StatusError)).Bold(true)
		errStr = "  " + errStyle.Render(m.inputError)
		errLen = lipgloss.Width(errStr)
	}

	inputWidth := w - lipgloss.Width(prompt) - errLen - 1
	if inputWidth < 10 {
		inputWidth = 10
	}
	m.textInput.Width = inputWidth

	line2 := prompt + m.textInput.View() + errStr

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().MaxWidth(w).Render(line1),
		lipgloss.NewStyle().MaxWidth(w).Render(line2),
	)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	if m.showHelp {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.renderHelpModal())
	}

	l := m.layout
	if l.Width == 0 {
		l = calculateLayout(m.width, m.height)
	}

	// 1. Top Tab Bar (height 1: pills line)
	tabHeader := m.renderTabBar()

	// 2. Active Tab Content Panel
	var contentBox string
	switch m.activeTab {
	case TabQueue:
		contentBox = m.renderTabContentPanel(" Jobs Queue ", "("+strconv.Itoa(len(m.queue))+") ", m.queueViewport.View(), l.TabContentWidth, l.TabContentHeight)

	case TabTelemetry:
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
		contentBox = m.renderTabContentPanel(centerTitle, centerRight, centerBody, l.TabContentWidth, l.TabContentHeight)

	case TabFiles:
		contentBox = m.renderTabContentPanel(" Saved Files ", "("+strconv.Itoa(len(m.savedFiles))+") ", m.filesViewport.View(), l.TabContentWidth, l.TabContentHeight)

	case TabSettings:
		contentBox = m.renderSettingsPanel(l.TabContentWidth, l.TabContentHeight)
	}

	// 3. Bottom URL Bar (2 lines)
	urlBar := m.renderBottomURLBar(l.Width)

	// 4. Contextual Footer (1 line)
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, tabHeader, contentBox, urlBar, footer)
}
