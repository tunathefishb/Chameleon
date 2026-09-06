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

	return theme
}
