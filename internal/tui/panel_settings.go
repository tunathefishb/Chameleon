package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SettingsState holds the configuration values for all 16 settings.
type SettingsState struct {
	// Category 0: Crawl & Scope (Indices 0 - 3)
	DepthIndex       int  // 0: 1, 1: 2, 2: 4, 3: 8
	PageBudgetIndex  int  // 0: 16, 1: 64, 2: 256, 3: 1024, 4: Unlimited
	DomainScopeIndex int  // 0: Same Host, 1: Subdomains, 2: Same TLD, 3: Unrestricted
	StripQuery       bool // true: Enabled, false: Disabled

	// Category 1: Network & Engine (Indices 4 - 7)
	WorkersIndex   int  // 0: 2, 1: 4, 2: 8, 3: 16
	SpeedIndex     int  // 0: 1 req/s (Safe), 1: 2 req/s, 2: 4 req/s, 3: 8 req/s (Aggressive)
	TimeoutIndex   int  // 0: 4s, 1: 8s, 2: 16s, 3: 32s
	UserAgentIndex int  // 0: ChameleonBot, 1: Chrome Desktop, 2: Mobile Safari, 3: Googlebot

	// Category 2: Ethics & Defense (Indices 8 - 11)
	RobotsPolicyIndex int  // 0: Strict (RFC 9309), 1: Warn in Telemetry, 2: Ignore
	Honeypots         bool // true: Filter Traps, false: Allow Traps
	AutoBackoff       bool // true: Honor 429/Retry-After, false: Disabled
	MaxBodySizeIndex  int  // 0: 2 MB, 1: 4 MB, 2: 8 MB, 3: 16 MB

	// Category 3: Output & Appearance (Indices 12 - 15)
	Images            bool // true: Download & Rewrite, false: Disabled
	ExportFormatIndex int  // 0: HTML Mirror, 1: Clean Markdown (LLM), 2: JSON-LD Only
	ThemeIndex        int  // 0: Chameleon Dark, 1: WCAG Light AA, 2: High-Contrast ANSI
	ReducedMotion     bool // true: Reduced Motion, false: Animated Spinners
}

// Option value arrays (all scaled by powers of 2 where applicable)
var (
	DepthOptions       = []int{1, 2, 4, 8}
	PageBudgetOptions  = []string{"16 Pages", "64 Pages", "256 Pages", "1024 Pages", "Unlimited"}
	DomainScopeOptions = []string{"Same Host", "Subdomains", "Same TLD", "Unrestricted"}
	WorkersOptions     = []int{2, 4, 8, 16}
	SpeedOptions       = []string{"1 req/s (Safe)", "2 req/s (Fast)", "4 req/s (Rapid)", "8 req/s (Aggressive)"}
	TimeoutOptions     = []string{"4s", "8s", "16s", "32s"}
	UserAgentOptions   = []string{"ChameleonBot (Ethical)", "Chrome Desktop", "Mobile Safari (iOS)", "Googlebot (Crawler)"}
	RobotsOptions      = []string{"Strict (RFC 9309)", "Warn in Telemetry", "Ignore"}
	MaxBodySizeOptions = []string{"2 MB", "4 MB", "8 MB", "16 MB"}
	ExportOptions      = []string{"HTML Mirror", "Clean Markdown (LLM)", "JSON-LD Only"}
	ThemeOptions       = []string{"Chameleon Dark", "WCAG Light AA", "High-Contrast ANSI"}
)

// DefaultSettingsState returns initialized settings with power-of-two defaults.
func DefaultSettingsState() SettingsState {
	return SettingsState{
		DepthIndex:        0,    // 1 level
		PageBudgetIndex:   1,    // 64 pages
		DomainScopeIndex:  0,    // Same Host
		StripQuery:        true, // Enabled
		WorkersIndex:      1,    // 4 workers
		SpeedIndex:        0,    // 1 req/s
		TimeoutIndex:      1,    // 8s
		UserAgentIndex:    0,    // ChameleonBot
		RobotsPolicyIndex: 0,    // Strict
		Honeypots:         true, // Filter traps
		AutoBackoff:       true, // Auto-throttle
		MaxBodySizeIndex:  2,    // 8 MB
		Images:            false,// Off
		ExportFormatIndex: 0,    // HTML Mirror
		ThemeIndex:        0,    // Dark
		ReducedMotion:     false,
	}
}

// SettingsItem represents a single setting definition for display and navigation.
type SettingsItem struct {
	Index        int
	Category     int
	CategoryName string
	Label        string
	Value        string
	Description  string
}

// GetItem retrieves item metadata by its 0-15 global index.
func (s *SettingsState) GetItem(index int) SettingsItem {
	switch index {
	// Category 0: Crawl & Scope
	case 0:
		depth := DepthOptions[s.DepthIndex]
		valStr := fmt.Sprintf("< %d >", depth)
		if depth == 1 {
			valStr += " (Direct only)"
		} else {
			valStr += fmt.Sprintf(" (%d levels)", depth)
		}
		return SettingsItem{
			Index: 0, Category: 0, CategoryName: "[1] Crawl & Scope",
			Label: "Depth:", Value: valStr,
			Description: "[Depth] Link traversal limit for recursive crawling. Depth 1 crawls only the seed page; higher depths discover internal links.",
		}
	case 1:
		return SettingsItem{
			Index: 1, Category: 0, CategoryName: "[1] Crawl & Scope",
			Label: "Page Budget:", Value: fmt.Sprintf("< %s >", PageBudgetOptions[s.PageBudgetIndex]),
			Description: "[Page Budget] Hard ceiling on visited pages. Prevents recursive traversal from running indefinitely on large domains.",
		}
	case 2:
		return SettingsItem{
			Index: 2, Category: 0, CategoryName: "[1] Crawl & Scope",
			Label: "Domain Scope:", Value: fmt.Sprintf("< %s >", DomainScopeOptions[s.DomainScopeIndex]),
			Description: "[Domain Scope] Traversal boundary. Restricts crawler to the same host, allows subdomains, or enables external link following.",
		}
	case 3:
		valStr := "[ ] Disabled"
		if s.StripQuery {
			valStr = "[x] Enabled"
		}
		return SettingsItem{
			Index: 3, Category: 0, CategoryName: "[1] Crawl & Scope",
			Label: "Strip Query:", Value: valStr,
			Description: "[Strip Query] Normalizes URLs by removing query strings (?utm_*, session IDs) to avoid crawler traps and redundant downloads.",
		}

	// Category 1: Network & Engine
	case 4:
		return SettingsItem{
			Index: 4, Category: 1, CategoryName: "[2] Network & Engine",
			Label: "Workers:", Value: fmt.Sprintf("< %d Workers >", WorkersOptions[s.WorkersIndex]),
			Description: "[Workers] Concurrency level for asynchronous HTTP requests executed by the background engine worker pool.",
		}
	case 5:
		return SettingsItem{
			Index: 5, Category: 1, CategoryName: "[2] Network & Engine",
			Label: "Speed:", Value: fmt.Sprintf("< %s >", SpeedOptions[s.SpeedIndex]),
			Description: "[Speed] Token bucket request dispatch rate limiter. Lower rates are gentler on target servers and avoid rate limits.",
		}
	case 6:
		return SettingsItem{
			Index: 6, Category: 1, CategoryName: "[2] Network & Engine",
			Label: "Timeout:", Value: fmt.Sprintf("< %s >", TimeoutOptions[s.TimeoutIndex]),
			Description: "[Timeout] Maximum duration to wait for socket connections, TLS handshakes, and response headers before aborting.",
		}
	case 7:
		return SettingsItem{
			Index: 7, Category: 1, CategoryName: "[2] Network & Engine",
			Label: "User-Agent:", Value: fmt.Sprintf("< %s >", UserAgentOptions[s.UserAgentIndex]),
			Description: "[User-Agent] HTTP User-Agent persona sent to servers. Toggle between ethical bot identification and browser fingerprints.",
		}

	// Category 2: Ethics & Defense
	case 8:
		return SettingsItem{
			Index: 8, Category: 2, CategoryName: "[3] Ethics & Defense",
			Label: "Robots.txt:", Value: fmt.Sprintf("< %s >", RobotsOptions[s.RobotsPolicyIndex]),
			Description: "[Robots.txt] RFC 9309 compliance policy. Strictly obeys Disallow and Crawl-Delay directives before requesting pages.",
		}
	case 9:
		valStr := "[ ] Allow Traps"
		if s.Honeypots {
			valStr = "[x] Filter Traps"
		}
		return SettingsItem{
			Index: 9, Category: 2, CategoryName: "[3] Ethics & Defense",
			Label: "Honeypots:", Value: valStr,
			Description: "[Honeypots] Automatically skips links hidden with display:none, opacity:0, or zero dimensions detected by the analyzer.",
		}
	case 10:
		valStr := "[ ] Disabled"
		if s.AutoBackoff {
			valStr = "[x] Auto-throttle"
		}
		return SettingsItem{
			Index: 10, Category: 2, CategoryName: "[3] Ethics & Defense",
			Label: "Auto-Backoff:", Value: valStr,
			Description: "[Auto-Backoff] Automatically suspends and throttles worker requests when encountering HTTP 429 or Retry-After headers.",
		}
	case 11:
		return SettingsItem{
			Index: 11, Category: 2, CategoryName: "[3] Ethics & Defense",
			Label: "Max Body Size:", Value: fmt.Sprintf("< %s >", MaxBodySizeOptions[s.MaxBodySizeIndex]),
			Description: "[Max Body Size] Response memory boundary. Guards against memory bloat and compression bomb attacks on large downloads.",
		}

	// Category 3: Output & Appearance
	case 12:
		valStr := "[ ] Download Images"
		if s.Images {
			valStr = "[x] Download Images"
		}
		return SettingsItem{
			Index: 12, Category: 3, CategoryName: "[4] Output & Appearance",
			Label: "Images:", Value: valStr,
			Description: "[Images] Discovers, downloads, and locally mirrors linked images (<img src>, <picture>, srcset) with rewritten relative paths.",
		}
	case 13:
		return SettingsItem{
			Index: 13, Category: 3, CategoryName: "[4] Output & Appearance",
			Label: "Export Format:", Value: fmt.Sprintf("< %s >", ExportOptions[s.ExportFormatIndex]),
			Description: "[Export Format] Output representation saved to disk. Clean Markdown extracts boilerplate-free text tailored for LLMs.",
		}
	case 14:
		return SettingsItem{
			Index: 14, Category: 3, CategoryName: "[4] Output & Appearance",
			Label: "Theme:", Value: fmt.Sprintf("< %s >", ThemeOptions[s.ThemeIndex]),
			Description: "[Theme] Switches active TUI theme palette live between Chameleon Dark, WCAG Light AA, and High-Contrast ANSI.",
		}
	case 15:
		valStr := "[ ] Animated Spinners"
		if s.ReducedMotion {
			valStr = "[x] Reduced Motion"
		}
		return SettingsItem{
			Index: 15, Category: 3, CategoryName: "[4] Output & Appearance",
			Label: "Motion:", Value: valStr,
			Description: "[Reduced Motion] Disables animated spinner tick loops and replaces motion elements with accessible static indicators.",
		}
	default:
		return SettingsItem{}
	}
}

// Adjust steps the selected setting forward (direction > 0) or backward (direction < 0).
func (s *SettingsState) Adjust(index int, direction int) {
	if direction == 0 {
		direction = 1
	}

	cycle := func(current, length int) int {
		res := (current + direction) % length
		if res < 0 {
			res += length
		}
		return res
	}

	switch index {
	case 0:
		s.DepthIndex = cycle(s.DepthIndex, len(DepthOptions))
	case 1:
		s.PageBudgetIndex = cycle(s.PageBudgetIndex, len(PageBudgetOptions))
	case 2:
		s.DomainScopeIndex = cycle(s.DomainScopeIndex, len(DomainScopeOptions))
	case 3:
		s.StripQuery = !s.StripQuery
	case 4:
		s.WorkersIndex = cycle(s.WorkersIndex, len(WorkersOptions))
	case 5:
		s.SpeedIndex = cycle(s.SpeedIndex, len(SpeedOptions))
	case 6:
		s.TimeoutIndex = cycle(s.TimeoutIndex, len(TimeoutOptions))
	case 7:
		s.UserAgentIndex = cycle(s.UserAgentIndex, len(UserAgentOptions))
	case 8:
		s.RobotsPolicyIndex = cycle(s.RobotsPolicyIndex, len(RobotsOptions))
	case 9:
		s.Honeypots = !s.Honeypots
	case 10:
		s.AutoBackoff = !s.AutoBackoff
	case 11:
		s.MaxBodySizeIndex = cycle(s.MaxBodySizeIndex, len(MaxBodySizeOptions))
	case 12:
		s.Images = !s.Images
	case 13:
		s.ExportFormatIndex = cycle(s.ExportFormatIndex, len(ExportOptions))
	case 14:
		s.ThemeIndex = cycle(s.ThemeIndex, len(ThemeOptions))
	case 15:
		s.ReducedMotion = !s.ReducedMotion
	}
}

// CategoryTitles defines the 4 category names.
var CategoryTitles = [4]string{
	"[1] Crawl & Scope",
	"[2] Network & Engine",
	"[3] Ethics & Defense",
	"[4] Output & Appearance",
}

// renderSettingsPanel builds the dual-column or responsive settings dashboard.
func (m Model) renderSettingsPanel(w, h int) string {
	innerWidth := w - 2
	if innerWidth < 30 {
		innerWidth = 30
	}

	isFocused := m.focusTarget == FocusTabContent
	activeItem := m.settingsState.GetItem(m.settingIndex)

	// Build the 4 category boxes
	categoryBoxes := make([]string, 4)
	isWide := innerWidth >= 76

	// Determine width for cards
	cardWidth := innerWidth - 4
	if isWide {
		cardWidth = (innerWidth - 6) / 2
	}
	if cardWidth < 28 {
		cardWidth = 28
	}

	for cat := 0; cat < 4; cat++ {
		var lines []string

		// Category header
		hasActiveSetting := isFocused && (m.settingIndex/4 == cat)
		headerStyle := lipgloss.NewStyle().Bold(true)
		if hasActiveSetting {
			headerStyle = headerStyle.Foreground(lipgloss.Color(m.theme.AccentColor))
		} else {
			headerStyle = headerStyle.Foreground(lipgloss.Color(m.theme.TitleInactive))
		}
		lines = append(lines, headerStyle.Render(CategoryTitles[cat]))

		// 4 items per category
		for i := 0; i < 4; i++ {
			itemIdx := cat*4 + i
			item := m.settingsState.GetItem(itemIdx)

			isSelected := isFocused && (m.settingIndex == itemIdx)
			cursor := "  "
			if isSelected {
				cursor = "▶ "
			}

			labelStr := item.Label
			valStr := item.Value

			// Format row with fixed alignment
			itemLine := fmt.Sprintf("%s%-14s %s", cursor, labelStr, valStr)

			if isSelected {
				itemLine = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color(m.theme.AccentColor)).
					Render(itemLine)
			} else {
				itemLine = lipgloss.NewStyle().
					Foreground(lipgloss.Color(m.theme.TitleInactive)).
					Render(itemLine)
			}
			lines = append(lines, itemLine)
		}

		cardBorderColor := m.theme.BorderInactive
		if hasActiveSetting {
			cardBorderColor = m.theme.BorderActive
		}

		cardInner := strings.Join(lines, "\n")
		cardBox := lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(cardBorderColor)).
			Width(cardWidth - 2).
			Render(cardInner)

		categoryBoxes[cat] = cardBox
	}

	// Assemble layout
	var grid string
	var gridWidth int
	if isWide {
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, categoryBoxes[0], " ", categoryBoxes[1])
		bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, categoryBoxes[2], " ", categoryBoxes[3])
		grid = lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
		gridWidth = lipgloss.Width(topRow)
	} else {
		grid = lipgloss.JoinVertical(lipgloss.Left, categoryBoxes[0], categoryBoxes[1], categoryBoxes[2], categoryBoxes[3])
		gridWidth = lipgloss.Width(categoryBoxes[0])
	}

	// Description callout box matching grid width
	descInnerWidth := gridWidth - 2
	if descInnerWidth < 28 {
		descInnerWidth = 28
	}

	descHeader := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.theme.AccentColor)).
		Render("Setting Info:")

	descText := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.TitleActive)).
		Render(activeItem.Description)

	descHint := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.theme.FooterDesc)).
		Render("Navigate: [↑/↓/j/k] • Adjust: [←/→/Space/Enter] • Jump: [PgUp/PgDn]")

	descInner := fmt.Sprintf("%s %s\n%s", descHeader, descText, descHint)
	descBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.theme.BorderInactive)).
		Width(descInnerWidth).
		Render(descInner)

	settingsBody := lipgloss.JoinVertical(lipgloss.Left, grid, descBox)
	return m.renderTabContentPanel(" Engine Settings ", "(16 Configs) ", settingsBody, w, h)
}
