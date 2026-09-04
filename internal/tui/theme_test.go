package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/titanous/json5"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()
	if theme.BorderActive != "39" {
		t.Fatalf("expected default BorderActive to be 39, got %s", theme.BorderActive)
	}
	if theme.StatusRunning != "39" {
		t.Fatalf("expected default StatusRunning to be 39, got %s", theme.StatusRunning)
	}
}

func TestLoadThemeJSON5(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "chameleon")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	t.Setenv("XDG_CONFIG_HOME", tempDir)

	json5Content := `
	{
		// Custom vibrant theme
		border_active: "205", // Hot pink
		border_inactive: "235",
		accent_color: "205",
		status_running: "205",
		/* Multi-line comment
		   for table styling */
		table_selected_fg: "231",
		table_selected_bg: "128", // Purple
	}
	`

	themePath := filepath.Join(configDir, "theme.json5")
	if err := os.WriteFile(themePath, []byte(json5Content), 0600); err != nil {
		t.Fatalf("failed to write theme.json5: %v", err)
	}

	var unmarshalErrTheme Theme
	if err := json5.Unmarshal([]byte(json5Content), &unmarshalErrTheme); err != nil {
		t.Fatalf("json5.Unmarshal error: %v", err)
	}
	theme := LoadTheme(themePath)
	if theme.BorderActive != "205" {
		t.Fatalf("expected BorderActive to be 205, got %s", theme.BorderActive)
	}
	if theme.BorderInactive != "235" {
		t.Fatalf("expected BorderInactive to be 235, got %s", theme.BorderInactive)
	}
	if theme.AccentColor != "205" {
		t.Fatalf("expected AccentColor to be 205, got %s", theme.AccentColor)
	}
	if theme.TableSelectedBg != "128" {
		t.Fatalf("expected TableSelectedBg to be 128, got %s", theme.TableSelectedBg)
	}
	// Verify unset fields retain default values
	if theme.TitleInactive != "245" {
		t.Fatalf("expected TitleInactive default 245, got %s", theme.TitleInactive)
	}
}
