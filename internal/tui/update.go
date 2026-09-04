package tui

import (
	"strings"
	"time"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+t", "f3":
			m.manualUIMode = true
			if m.uiMode == UIModeGrid {
				m.uiMode = UIModeTabbed
				m.setFocusTarget(FocusURLInput)
			} else {
				m.uiMode = UIModeGrid
				m.focusPanel(PanelInput)
			}
			m.syncDimensions()
			return m, nil
		}

		if m.showHelp {
			switch msg.String() {
			case "esc", "q", "?", "f1", "enter", " ":
				m.showHelp = false
			}
			return m, nil
		}

		if m.uiMode == UIModeTabbed {
			return m.updateTabbedKey(msg)
		}

		switch msg.String() {
		case "f1":
			m.showHelp = true
			return m, nil
		case "?":
			if m.activePanel != PanelInput {
				m.showHelp = true
				return m, nil
			}
		case "tab":
			m.nextPanel()
			return m, nil
		case "shift+tab":
			m.prevPanel()
			return m, nil
		case "ctrl+a", "f2":
			if m.activePanel == PanelInput {
				val := strings.TrimSpace(m.textInput.Value())
				if val != "" {
					m.startAnalysis(val)
					return m, nil
				}
			}
			// Toggle center view mode
			if m.centerMode == CenterViewTelemetry {
				m.centerMode = CenterViewReport
			} else {
				m.centerMode = CenterViewTelemetry
			}
			return m, nil
		}

		switch m.activePanel {
		case PanelInput:
			return m.updateInput(msg)

		case PanelQueue:
			return m.updateQueueKey(msg)

		case PanelTable:
			return m.updateTable(msg)

		case PanelFiles:
			m.filesViewport, cmd = m.filesViewport.Update(msg)
			cmds = append(cmds, cmd)

		case PanelSettings:
			return m.updateSettings(msg)
		}

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

	case engineFileMsg:
		m.addFile(string(msg))
		cmds = append(cmds, waitForFile(m.eng.Files))

	case engineDiscoveredMsg:
		m.addQueue(string(msg))
		cmds = append(cmds, waitForDiscovered(m.eng.Discovered))

	case engineAnalysisMsg:
		r := analyzer.Report(msg)
		m.latestReport = &r
		m.isAnalyzing = false
		m.centerMode = CenterViewReport
		m.updateReportContent()
		cmds = append(cmds, waitForAnalysis(m.eng.Analysis))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.needsQueueUpdate {
			m.updateQueueContent()
			m.needsQueueUpdate = false
		}
		if m.needsFilesUpdate {
			m.updateFilesContent()
			m.needsFilesUpdate = false
		}
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.manualUIMode {
			if m.width < 120 || m.height < 28 {
				m.uiMode = UIModeTabbed
			} else {
				m.uiMode = UIModeGrid
			}
		}
		m.syncDimensions()
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) syncDimensions() {
	if m.width == 0 || m.height == 0 {
		return
	}
	m.layout = calculateLayout(m.width, m.height)

	if m.uiMode == UIModeTabbed {
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
	} else {
		// Grid mode
		m.queueViewport.Width = m.layout.LeftWidth - 2
		m.queueViewport.Height = m.layout.TopHeight - 3

		m.filesViewport.Width = m.layout.RightWidth - 2
		m.filesViewport.Height = m.layout.TopHeight - 3

		tableWidth := m.layout.CenterWidth - 4
		if tableWidth < 20 {
			tableWidth = 20
		}
		m.table.SetWidth(tableWidth)
		m.table.SetHeight(m.layout.TopHeight - 3)

		m.reportViewport.Width = m.layout.CenterWidth - 2
		m.reportViewport.Height = m.layout.TopHeight - 3

		m.verboseViewport.Width = m.layout.CenterWidth - 2
		m.verboseViewport.Height = m.layout.TopHeight - 3

		m.updateTableColumns(tableWidth)
		m.textInput.Width = m.layout.BottomLeftWidth - 8
	}
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

func (m *Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		val := strings.TrimSpace(m.textInput.Value())
		if val != "" {
			if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
				val = "https://" + val
			}
			m.eng.AddJob(val, m.settings)
			m.addQueue(val)
			m.textInput.SetValue("")
		}
		return *m, nil
	}
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
				m.selectedItem = &selected
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
	switch msg.String() {
	case "up", "k":
		m.settingIndex = (m.settingIndex - 1 + 3) % 3
	case "down", "j":
		m.settingIndex = (m.settingIndex + 1) % 3
	case "left":
		switch m.settingIndex {
		case 0:
			if m.settings.Depth > 1 {
				m.settings.Depth--
			}
		case 1:
			m.settings.Images = !m.settings.Images
		case 2:
			if m.settings.Speed == engine.SpeedFast {
				m.settings.Speed = engine.SpeedSafe
			} else {
				m.settings.Speed = engine.SpeedFast
			}
		}
	case "right", "enter", " ":
		switch m.settingIndex {
		case 0:
			if msg.String() == "right" {
				if m.settings.Depth < 5 {
					m.settings.Depth++
				}
			} else {
				m.settings.Depth = (m.settings.Depth % 5) + 1
			}
		case 1:
			m.settings.Images = !m.settings.Images
		case 2:
			if m.settings.Speed == engine.SpeedSafe {
				m.settings.Speed = engine.SpeedFast
			} else {
				m.settings.Speed = engine.SpeedSafe
			}
		}
	}
	return *m, nil
}
