package tui

import (
	"os"
	"time"

	"chameleon/internal/engine"
	"chameleon/internal/engine/analyzer"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ActiveTab int

const (
	TabQueue ActiveTab = iota
	TabTelemetry
	TabFiles
	TabSettings
)

const totalTabs = 4

type FocusArea int

const (
	FocusTabContent FocusArea = iota
	FocusURLInput
)

type CenterViewMode int

const (
	CenterViewTelemetry CenterViewMode = iota
	CenterViewReport
	CenterViewVerbose
)

type Model struct {
	width  int
	height int
	layout Layout

	activeTab    ActiveTab
	focusTarget  FocusArea
	centerMode   CenterViewMode
	settingIndex int
	queueCursor  int

	textInput       textinput.Model
	table           table.Model
	queueViewport   viewport.Model
	filesViewport   viewport.Model
	reportViewport  viewport.Model
	verboseViewport viewport.Model

	settings         engine.Settings
	settingsState    SettingsState
	eng              *engine.Engine
	queue            []string
	savedFiles       []SavedFileEntry
	filesCursor      int
	telemetryItems   []engine.Result
	latestReport     *analyzer.Report
	isAnalyzing      bool
	analyzingURL     string
	totalTelemetry   int
	lastTelemetry    time.Time
	spinner          spinner.Model
	theme            Theme
	showHelp         bool
	needsQueueUpdate bool
	needsFilesUpdate bool
	reducedMotion    bool
	spinnerRunning   bool
	confirmStopURL   string
	inputError       string
}

func (m Model) hasActiveWork() bool {
	if m.isAnalyzing {
		return true
	}
	if m.eng != nil && m.eng.HasActiveWork() {
		return true
	}
	if time.Since(m.lastTelemetry) < 2*time.Second {
		return true
	}
	return false
}

func (m *Model) startSpinnerCmd() tea.Cmd {
	if !m.reducedMotion && !m.spinnerRunning {
		m.spinnerRunning = true
		return m.spinner.Tick
	}
	return nil
}

func InitialModel(eng *engine.Engine, themes ...Theme) Model {
	theme := DefaultTheme()
	if len(themes) > 0 {
		theme = themes[0]
	}

	reducedMotion := os.Getenv("REDUCED_MOTION") == "1" || os.Getenv("NO_ANIMATIONS") == "1"

	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "https://example.com"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 30

	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 8},
		{Title: "Type", Width: 10},
		{Title: "Size", Width: 8},
		{Title: "Time", Width: 8},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(theme.TableHeaderBorder)).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color(theme.TableSelectedFg)).
		Background(lipgloss.Color(theme.TableSelectedBg)).
		Bold(false)
	t.SetStyles(s)

	qVp := viewport.New(0, 0)
	qVp.SetContent("  (no queued jobs)")

	fVp := viewport.New(0, 0)
	fVp.SetContent("  (no saved files)")

	rVp := viewport.New(0, 0)
	rVp.SetContent("  (No analysis run yet. Enter a URL and press Ctrl+A to test scrapability & ethics)")

	vVp := viewport.New(0, 0)
	vVp.SetContent("  (No telemetry entry selected)")

	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentColor))

	return Model{
		activeTab:       TabQueue,
		focusTarget:     FocusURLInput,
		centerMode:      CenterViewTelemetry,
		settingIndex:    0,
		queueCursor:     0,
		textInput:       ti,
		table:           t,
		queueViewport:   qVp,
		filesViewport:   fVp,
		reportViewport:  rVp,
		verboseViewport: vVp,
		settings: engine.Settings{
			Depth:  1,
			Images: false,
			Speed:  engine.SpeedSafe,
		},
		settingsState:  DefaultSettingsState(),
		eng:            eng,
		queue:          []string{},
		savedFiles:     []SavedFileEntry{},
		filesCursor:    0,
		telemetryItems: []engine.Result{},
		totalTelemetry: 0,
		spinner:        sp,
		theme:          theme,
		reducedMotion:  reducedMotion,
		spinnerRunning: false,
	}
}

func (m *Model) syncSettingsToEngine() {
	if m.settingsState.DepthIndex >= 0 && m.settingsState.DepthIndex < len(DepthOptions) {
		m.settings.Depth = DepthOptions[m.settingsState.DepthIndex]
	}
	m.settings.Images = m.settingsState.Images
	if m.settingsState.SpeedIndex == 0 {
		m.settings.Speed = engine.SpeedSafe
	} else {
		m.settings.Speed = engine.SpeedFast
	}
}


func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		textinput.Blink,
		waitForResult(m.eng.Results),
		waitForFile(m.eng.Files),
		waitForDiscovered(m.eng.Discovered),
		waitForAnalysis(m.eng.Analysis),
	}
	if !m.reducedMotion && m.hasActiveWork() {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

func (m *Model) focusTab(t ActiveTab) {
	m.activeTab = t
	if m.focusTarget == FocusTabContent {
		if t == TabTelemetry && m.centerMode == CenterViewTelemetry {
			m.table.Focus()
		} else {
			m.table.Blur()
		}
	}
	m.updateQueueContent()
	m.updateFilesContent()
}

func (m *Model) nextTab() {
	m.focusTab((m.activeTab + 1) % totalTabs)
}

func (m *Model) prevTab() {
	m.focusTab((m.activeTab - 1 + totalTabs) % totalTabs)
}

func (m *Model) setFocusTarget(target FocusArea) {
	m.focusTarget = target
	if target == FocusURLInput {
		m.textInput.Focus()
		m.table.Blur()
	} else {
		m.textInput.Blur()
		if m.activeTab == TabTelemetry && m.centerMode == CenterViewTelemetry {
			m.table.Focus()
		} else {
			m.table.Blur()
		}
	}
	m.updateQueueContent()
	m.updateFilesContent()
}
