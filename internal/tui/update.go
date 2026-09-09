package tui

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.showHelp {
			switch msg.String() {
			case "esc", "q", "?", "f1", "enter", " ":
				m.showHelp = false
			}
			return m, nil
		}

		return m.updateKey(msg)

	case engineResultMsg:
		m.totalTelemetry++
		m.lastTelemetry = time.Now()
		res := engine.Result(msg)
		m.telemetryItems = append([]engine.Result{res}, m.telemetryItems...)
		if len(m.telemetryItems) > 100 {
			m.telemetryItems = m.telemetryItems[:100]
		}
		rows := m.table.Rows()
		newRow := table.Row{res.Name, res.Status, res.Type, res.Size, res.Time}
		rows = append([]table.Row{newRow}, rows...)
		if len(rows) > 100 {
			rows = rows[:100]
		}
		m.table.SetRows(rows)
		m.needsQueueUpdate = true
		cmds = append(cmds, waitForResult(m.eng.Results))
		if cmd := m.startSpinnerCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case engineFileMsg:
		m.addFile(string(msg))
		cmds = append(cmds, waitForFile(m.eng.Files))
		if cmd := m.startSpinnerCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case engineDiscoveredMsg:
		m.addQueue(string(msg))
		cmds = append(cmds, waitForDiscovered(m.eng.Discovered))
		if cmd := m.startSpinnerCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case engineAnalysisMsg:
		r := analyzer.Report(msg)
		m.latestReport = &r
		m.isAnalyzing = false
		m.centerMode = CenterViewReport
		m.updateReportContent()
		cmds = append(cmds, waitForAnalysis(m.eng.Analysis))

	case spinner.TickMsg:
		if m.needsQueueUpdate {
			m.updateQueueContent()
			m.needsQueueUpdate = false
		}
		if m.needsFilesUpdate {
			m.updateFilesContent()
			m.needsFilesUpdate = false
		}
		if !m.reducedMotion && m.hasActiveWork() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			m.spinnerRunning = true
			cmds = append(cmds, cmd)
		} else {
			m.spinnerRunning = false
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.syncDimensions()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "f1":
		m.showHelp = true
		return *m, nil
	}

	// Focus switching between active tab and bottom URL bar
	if msg.String() == "tab" || msg.String() == "shift+tab" {
		if m.focusTarget == FocusTabContent {
			m.setFocusTarget(FocusURLInput)
		} else {
			m.setFocusTarget(FocusTabContent)
		}
		return *m, nil
	}

	// When URL input is focused
	if m.focusTarget == FocusURLInput {
		switch msg.String() {
		case "esc":
			m.setFocusTarget(FocusTabContent)
			return *m, nil

		case "ctrl+a", "f2":
			val := strings.TrimSpace(m.textInput.Value())
			if val != "" {
				normalized, _, err := normalizeAndValidateURL(val)
				if err != nil {
					m.inputError = "⚠️ Please enter a valid URL (e.g. example.com)"
					return *m, nil
				}
				m.inputError = ""
				cmd := m.startAnalysis(normalized)
				m.activeTab = TabTelemetry
				m.centerMode = CenterViewReport
				m.setFocusTarget(FocusTabContent)
				return *m, cmd
			}
		}

		return m.updateInput(msg)
	}

	// When active tab content is focused
	switch msg.String() {
	case "?":
		m.showHelp = true
		return *m, nil

	case "1":
		m.focusTab(TabQueue)
		return *m, nil

	case "2":
		m.focusTab(TabTelemetry)
		return *m, nil

	case "3":
		m.focusTab(TabFiles)
		return *m, nil

	case "4":
		m.focusTab(TabSettings)
		return *m, nil

	case "[":
		m.prevTab()
		return *m, nil

	case "]":
		m.nextTab()
		return *m, nil

	case "ctrl+a", "f2":
		if m.activeTab == TabTelemetry {
			if m.centerMode == CenterViewTelemetry {
				m.centerMode = CenterViewReport
			} else {
				m.centerMode = CenterViewTelemetry
			}
			return *m, nil
		}
	}

	// Delegate to active tab component
	switch m.activeTab {
	case TabQueue:
		return m.updateQueueKey(msg)

	case TabTelemetry:
		return m.updateTable(msg)

	case TabFiles:
		return m.updateFilesKey(msg)

	case TabSettings:
		return m.updateSettings(msg)
	}

	return *m, nil
}

func (m *Model) syncDimensions() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.layout = calculateLayout(m.width, m.height)

	contentW := m.layout.TabContentWidth
	contentH := m.layout.TabContentHeight

	m.queueViewport.Width = contentW - 2
	m.queueViewport.Height = contentH - 3

	m.filesViewport.Width = contentW - 2
	m.filesViewport.Height = contentH - 3

	m.reportViewport.Width = contentW - 2
	m.reportViewport.Height = contentH - 3

	m.verboseViewport.Width = contentW - 2
	m.verboseViewport.Height = contentH - 3

	tableWidth := contentW - 4
	if tableWidth < 20 {
		tableWidth = 20
	}
	m.table.SetWidth(tableWidth)
	m.table.SetHeight(contentH - 3)

	m.updateTableColumns(tableWidth)
	m.textInput.Width = m.layout.Width - 6

	m.updateFilesContent()
	m.updateQueueContent()
}

func (m *Model) updateTableColumns(tableWidth int) {
	availableWidth := tableWidth - 10
	if availableWidth < 10 {
		availableWidth = 10
	}

	nameWidth := int(float64(availableWidth) * 0.38)
	statusWidth := int(float64(availableWidth) * 0.15)
	typeWidth := int(float64(availableWidth) * 0.17)
	sizeWidth := int(float64(availableWidth) * 0.15)
	timeWidth := availableWidth - nameWidth - statusWidth - typeWidth - sizeWidth
	if timeWidth < 6 {
		timeWidth = 6
	}

	m.table.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Size", Width: sizeWidth},
		{Title: "Time", Width: timeWidth},
	})
}

var nonHTTPSchemes = map[string]bool{
	"mailto":     true,
	"ftp":        true,
	"javascript": true,
	"file":       true,
	"data":       true,
	"tel":        true,
	"vbscript":   true,
	"ws":         true,
	"wss":        true,
	"ssh":        true,
	"git":        true,
	"about":      true,
	"blob":       true,
	"irc":        true,
	"news":       true,
	"gopher":     true,
	"ldap":       true,
}

func normalizeAndValidateURL(raw string) (string, *url.URL, error) {
	val := strings.TrimSpace(raw)
	if val == "" {
		return "", nil, errors.New("empty URL")
	}
	if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
		if colonIdx := strings.Index(val, ":"); colonIdx != -1 {
			if !strings.HasPrefix(val, "[") {
				scheme := strings.ToLower(val[:colonIdx])
				if strings.HasPrefix(val[colonIdx:], "://") || nonHTTPSchemes[scheme] {
					return "", nil, errors.New("invalid URL structure")
				}
			}
		}
	}
	normalized := val
	if !strings.HasPrefix(normalized, "http://") && !strings.HasPrefix(normalized, "https://") {
		normalized = "https://" + normalized
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" || strings.ContainsAny(parsed.Host, " \t\r\n") {
		return "", nil, errors.New("invalid URL structure")
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		hostname = parsed.Host
	}
	if !strings.Contains(hostname, ".") && hostname != "localhost" && net.ParseIP(hostname) == nil {
		return "", nil, errors.New("invalid URL structure")
	}
	return normalized, parsed, nil
}

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			normalized, _, err := normalizeAndValidateURL(val)
			if err != nil {
				m.inputError = "⚠️ Please enter a valid URL (e.g. example.com)"
				return *m, nil
			}

			m.inputError = ""
			m.eng.AddJob(normalized, m.settings)
			m.addQueue(normalized)
			m.textInput.SetValue("")
			if cmd := m.startSpinnerCmd(); cmd != nil {
				return *m, cmd
			}
		}
		return *m, nil
	}

	m.inputError = ""
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return *m, cmd
}

func (m *Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.centerMode {
	case CenterViewVerbose:
		switch msg.String() {
		case "esc", "q", "enter", "backspace":
			m.centerMode = CenterViewTelemetry
			return *m, nil
		case "a":
			m.centerMode = CenterViewReport
			return *m, nil
		case "t":
			m.centerMode = CenterViewTelemetry
			return *m, nil
		}
		m.verboseViewport, cmd = m.verboseViewport.Update(msg)
		return *m, cmd

	case CenterViewReport:
		switch msg.String() {
		case "a", "t", " ", "esc":
			m.centerMode = CenterViewTelemetry
			return *m, nil
		}
		m.reportViewport, cmd = m.reportViewport.Update(msg)
		return *m, cmd

	case CenterViewTelemetry:
		switch msg.String() {
		case "enter", "i", "v":
			cursor := m.table.Cursor()
			if len(m.telemetryItems) > 0 && cursor >= 0 && cursor < len(m.telemetryItems) {
				selected := m.telemetryItems[cursor]
				m.updateVerboseContent(selected)
				m.centerMode = CenterViewVerbose
				return *m, nil
			}
		case "a", "t", " ":
			m.centerMode = CenterViewReport
			return *m, nil
		}
		m.table, cmd = m.table.Update(msg)
		return *m, cmd
	}
	return *m, nil
}

func (m *Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "up", "k":
		m.settingIndex = (m.settingIndex - 1 + 16) % 16
	case "down", "j":
		m.settingIndex = (m.settingIndex + 1) % 16
	case "pgup", "b":
		// Jump to previous category (4 items)
		m.settingIndex = (m.settingIndex - 4 + 16) % 16
	case "pgdown", "f":
		// Jump to next category (4 items)
		m.settingIndex = (m.settingIndex + 4) % 16
	case "home", "g":
		m.settingIndex = 0
	case "end", "G":
		m.settingIndex = 15
	case "left", "h":
		m.settingsState.Adjust(m.settingIndex, -1)
		cmd = m.applySettingSideEffects()
	case "right", "l", "enter", " ":
		m.settingsState.Adjust(m.settingIndex, 1)
		cmd = m.applySettingSideEffects()
	}

	return *m, cmd
}

// applySettingSideEffects applies live modifications (theme, motion, engine settings).
func (m *Model) applySettingSideEffects() tea.Cmd {
	var cmd tea.Cmd

	// 1. Sync engine settings (Depth, Images, Speed)
	m.syncSettingsToEngine()

	// 2. Live Theme Switching
	switch m.settingsState.ThemeIndex {
	case 0:
		m.theme = DefaultTheme()
	case 1:
		m.theme = LightTheme()
	case 2:
		m.theme = AccessibleTheme()
	}

	// Update table styles to match new theme
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(m.theme.TableHeaderBorder)).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color(m.theme.TableSelectedFg)).
		Background(lipgloss.Color(m.theme.TableSelectedBg)).
		Bold(false)
	m.table.SetStyles(s)
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.AccentColor))

	// 3. Live Motion Switching
	m.reducedMotion = m.settingsState.ReducedMotion
	if m.reducedMotion {
		m.spinnerRunning = false
	} else if m.hasActiveWork() && !m.spinnerRunning {
		m.spinnerRunning = true
		cmd = m.spinner.Tick
	}

	return cmd
}
