package tui

import (
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

type Panel int

const (
	PanelInput Panel = iota
	PanelQueue
	PanelTable
	PanelFiles
	PanelSettings
)

const totalPanels = 5

type UIMode int

const (
	UIModeGrid UIMode = iota
	UIModeTabbed
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

	uiMode       UIMode
	manualUIMode bool
	activeTab    ActiveTab
	focusTarget  FocusArea

	activePanel  Panel
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
	eng              *engine.Engine
	queue            []string
	files            []string
	telemetryItems   []engine.Result
	selectedItem     *engine.Result
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
}

func InitialModel(eng *engine.Engine, themes ...Theme) Model {
	theme := DefaultTheme()
	if len(themes) > 0 {
		theme = themes[0]
	}

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
		uiMode:          UIModeGrid,
		manualUIMode:    false,
		activeTab:       TabQueue,
		focusTarget:     FocusURLInput,
		activePanel:     PanelInput,
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
		eng:            eng,
		queue:          []string{},
		files:          []string{},
		telemetryItems: []engine.Result{},
		totalTelemetry: 0,
		spinner:        sp,
		theme:          theme,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		waitForResult(m.eng.Results),
		waitForFile(m.eng.Files),
		waitForDiscovered(m.eng.Discovered),
		waitForAnalysis(m.eng.Analysis),
	)
}

func (m *Model) focusPanel(p Panel) {
	m.activePanel = p
	if p == PanelInput {
		m.textInput.Focus()
	} else {
		m.textInput.Blur()
	}

	if p == PanelTable {
		m.table.Focus()
	} else {
		m.table.Blur()
	}

	m.updateQueueContent()
}

func (m *Model) nextPanel() {
	m.focusPanel((m.activePanel + 1) % totalPanels)
}

func (m *Model) prevPanel() {
	m.focusPanel((m.activePanel - 1 + totalPanels) % totalPanels)
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
}
