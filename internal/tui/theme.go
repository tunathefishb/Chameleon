package tui

import (
	"os"
	"path/filepath"

	"github.com/titanous/json5"
)

type Theme struct {
	// Panel & Border Styling
	BorderActive   string `json:"border_active"`
	BorderInactive string `json:"border_inactive"`
	TitleActive    string `json:"title_active"`
	TitleInactive  string `json:"title_inactive"`

	// Table Styling
	TableHeaderBorder string `json:"table_header_border"`
	TableSelectedFg   string `json:"table_selected_fg"`
	TableSelectedBg   string `json:"table_selected_bg"`

	// Settings & Highlights
	AccentColor string `json:"accent_color"`

	// Footer Styling
	FooterKey  string `json:"footer_key"`
	FooterDesc string `json:"footer_desc"`
	FooterSep  string `json:"footer_sep"`

	// Job Status Badges
	StatusRunning string `json:"status_running"`
	StatusPaused  string `json:"status_paused"`
	StatusStopped string `json:"status_stopped"`
	StatusDone    string `json:"status_done"`
	StatusError   string `json:"status_error"`
	StatusQueued  string `json:"status_queued"`

	// Status theme tokens
	Error   string `json:"error,omitempty"`
	Success string `json:"success,omitempty"`
}

// DefaultTheme returns the default Chameleon theme colors.
func DefaultTheme() Theme {
	return Theme{
		BorderActive:      "39",
		BorderInactive:    "240",
		TitleActive:       "39",
		TitleInactive:     "245",
		TableHeaderBorder: "240",
		TableSelectedFg:   "229",
		TableSelectedBg:   "57",
		AccentColor:       "39",
		FooterKey:         "39",
		FooterDesc:        "244",
		FooterSep:         "246",
		StatusRunning:     "39",
		StatusPaused:      "220",
		StatusStopped:     "208",
		StatusDone:        "46",
		StatusError:       "196",
		StatusQueued:      "244",
		Error:             "196",
		Success:           "46",
	}
}

// AccessibleTheme returns the high-contrast 16-ANSI color theme.
func AccessibleTheme() Theme {
	return Theme{
		BorderActive:      "15",
		BorderInactive:    "7",
		TitleActive:       "15",
		TitleInactive:     "7",
		TableHeaderBorder: "7",
		TableSelectedFg:   "0",
		TableSelectedBg:   "15",
		AccentColor:       "14",
		FooterKey:         "14",
		FooterDesc:        "15",
		FooterSep:         "7",
		StatusRunning:     "14",
		StatusPaused:      "11",
		StatusStopped:     "208",
		StatusDone:        "10",
		StatusError:       "9",
		StatusQueued:      "7",
		Error:             "9",
		Success:           "10",
	}
}

// LightTheme returns the WCAG 2.1 AA compliant light background theme.
func LightTheme() Theme {
	return Theme{
		BorderActive:      "24",
		BorderInactive:    "244",
		TitleActive:       "24",
		TitleInactive:     "240",
		TableHeaderBorder: "244",
		TableSelectedFg:   "255",
		TableSelectedBg:   "24",
		AccentColor:       "24",
		FooterKey:         "24",
		FooterDesc:        "240",
		FooterSep:         "244",
		StatusRunning:     "24",
		StatusPaused:      "130",
		StatusStopped:     "166",
		StatusDone:        "28",
		StatusError:       "160",
		StatusQueued:      "240",
		Error:             "160",
		Success:           "28",
	}
}

// LoadTheme attempts to read and unmarshal a theme from customPaths or default locations
// (~/.config/chameleon/theme.json5 or ./theme.json5), falling back gracefully to DefaultTheme().
func LoadTheme(customPaths ...string) Theme {
	theme := DefaultTheme()

	var configPaths []string
	configPaths = append(configPaths, customPaths...)

	// Check local directory
	configPaths = append(configPaths, "theme.json5", "theme.json")

	// Check user config directory
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		configPaths = append(configPaths, filepath.Join(userConfigDir, "chameleon", "theme.json5"))
		configPaths = append(configPaths, filepath.Join(userConfigDir, "chameleon", "theme.json"))
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		configPaths = append(configPaths, filepath.Join(homeDir, ".config", "chameleon", "theme.json5"))
		configPaths = append(configPaths, filepath.Join(homeDir, ".chameleon", "theme.json5"))
	}

	for _, p := range configPaths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err == nil {
			_ = json5.Unmarshal(data, &theme)
			break
		}
	}

	if theme.Error == "" {
		theme.Error = theme.StatusError
	}
	if theme.Success == "" {
		theme.Success = theme.StatusDone
	}
	if theme.StatusError == "" {
		theme.StatusError = theme.Error
	}
	if theme.StatusDone == "" {
		theme.StatusDone = theme.Success
	}

	return theme
}
