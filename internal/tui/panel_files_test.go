package tui

import (
	"strings"
	"testing"

	"chameleon/internal/engine"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestParseSavedFile(t *testing.T) {
	tests := []struct {
		path     string
		wantHost string
		wantRel  string
		wantType string
	}{
		{
			path:     "output/en.wikipedia.org/index.html",
			wantHost: "en.wikipedia.org",
			wantRel:  "/index.html",
			wantType: "HTML",
		},
		{
			path:     "output/en.wikipedia.org/static/logo.png",
			wantHost: "en.wikipedia.org",
			wantRel:  "/static/logo.png",
			wantType: "PNG",
		},
		{
			path:     "output/example.com/assets/banner.jpg",
			wantHost: "example.com",
			wantRel:  "/assets/banner.jpg",
			wantType: "JPG",
		},
		{
			path:     "output/example.com/styles.css",
			wantHost: "example.com",
			wantRel:  "/styles.css",
			wantType: "CSS",
		},
		{
			path:     "output/example.com/app.js",
			wantHost: "example.com",
			wantRel:  "/app.js",
			wantType: "JS",
		},
		{
			path:     "output/example.com/data.json",
			wantHost: "example.com",
			wantRel:  "/data.json",
			wantType: "JSON",
		},
	}

	for _, tt := range tests {
		entry := parseSavedFile(tt.path)
		if entry.Host != tt.wantHost {
			t.Errorf("parseSavedFile(%q) host = %q, want %q", tt.path, entry.Host, tt.wantHost)
		}
		if entry.RelPath != tt.wantRel {
			t.Errorf("parseSavedFile(%q) relPath = %q, want %q", tt.path, entry.RelPath, tt.wantRel)
		}
		if entry.Type != tt.wantType {
			t.Errorf("parseSavedFile(%q) type = %q, want %q", tt.path, entry.Type, tt.wantType)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(500); got != "500 B" {
		t.Errorf("formatBytes(500) = %q, want 500 B", got)
	}
	if got := formatBytes(1024); got != "1.0 KB" {
		t.Errorf("formatBytes(1024) = %q, want 1.0 KB", got)
	}
	if got := formatBytes(43100); got != "42.1 KB" {
		t.Errorf("formatBytes(43100) = %q, want 42.1 KB", got)
	}
	if got := formatBytes(1887436); got != "1.8 MB" {
		t.Errorf("formatBytes(1887436) = %q, want 1.8 MB", got)
	}
}

func TestSavedFilesTableRenderingWide(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.filesViewport.Width = 100
	m.filesViewport.Height = 20

	m.addFile("output/en.wikipedia.org/index.html")
	m.addFile("output/en.wikipedia.org/static/logo.png")
	m.addFile("output/ja.wikipedia.org/index.html")

	m.focusTab(TabFiles)
	m.setFocusTarget(FocusTabContent)
	m.updateFilesContent()

	content := m.filesViewport.View()

	// Verify Storage Summary Card
	if !strings.Contains(content, "STORAGE SUMMARY") {
		t.Errorf("expected content to contain STORAGE SUMMARY, got: %s", content)
	}
	if !strings.Contains(content, "3 files") {
		t.Errorf("expected content to mention 3 files, got: %s", content)
	}

	// Verify Table Header & Columns
	if !strings.Contains(content, "TYPE") || !strings.Contains(content, "HOST") || !strings.Contains(content, "PATH / FILENAME") {
		t.Errorf("expected content to contain table header columns, got: %s", content)
	}

	// Verify File Rows and Badges
	if !strings.Contains(content, "HTML") || !strings.Contains(content, "PNG") {
		t.Errorf("expected badges HTML and PNG, got: %s", content)
	}
	if !strings.Contains(content, "en.wikipedia.org") || !strings.Contains(content, "ja.wikipedia.org") {
		t.Errorf("expected hostnames in content, got: %s", content)
	}
}

func TestSavedFilesTableRenderingNarrow(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.filesViewport.Width = 30
	m.filesViewport.Height = 15

	m.addFile("output/en.wikipedia.org/index.html")
	m.addFile("output/en.wikipedia.org/static/logo.png")

	m.focusTab(TabFiles)
	m.setFocusTarget(FocusTabContent)
	m.updateFilesContent()

	content := m.filesViewport.View()

	// Verify compact summary header
	if !strings.Contains(content, "2 Files") {
		t.Errorf("expected compact summary with 2 Files, got: %s", content)
	}

	// Verify compact entries
	if !strings.Contains(content, "index.html") {
		t.Errorf("expected index.html in narrow output, got: %s", content)
	}
}

func TestSavedFilesCursorNavigation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	m.filesViewport.Width = 90
	m.filesViewport.Height = 10

	m.addFile("output/en.wikipedia.org/index.html")
	m.addFile("output/en.wikipedia.org/about.html")
	m.addFile("output/en.wikipedia.org/static/logo.png")

	m.focusTab(TabFiles)
	m.setFocusTarget(FocusTabContent)

	if m.filesCursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.filesCursor)
	}

	// Navigate Down
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.filesCursor != 1 {
		t.Fatalf("expected cursor 1 after down key, got %d", m.filesCursor)
	}

	// Navigate Down with 'j'
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = next.(Model)
	if m.filesCursor != 2 {
		t.Fatalf("expected cursor 2 after 'j', got %d", m.filesCursor)
	}

	// Down at boundary should clamp at 2
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if m.filesCursor != 2 {
		t.Fatalf("expected cursor clamped at 2, got %d", m.filesCursor)
	}

	// Navigate Up with 'k'
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = next.(Model)
	if m.filesCursor != 1 {
		t.Fatalf("expected cursor 1 after 'k', got %d", m.filesCursor)
	}

	// Jump to Home / End
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m = next.(Model)
	if m.filesCursor != 0 {
		t.Fatalf("expected cursor 0 after Home, got %d", m.filesCursor)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = next.(Model)
	if m.filesCursor != 2 {
		t.Fatalf("expected cursor 2 after End, got %d", m.filesCursor)
	}
}

func TestVisualPreview(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Simulate window size in Tabbed mode
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m = next.(Model)

	// Jump to Tab 3: Files and focus tab content
	m.focusTab(TabFiles)
	m.setFocusTarget(FocusTabContent)

	// Add files like in the user's screenshot
	m.addFile("output/wikipedia.org/index.html")
	m.addFile("output/en.wikipedia.org/index.html")
	m.addFile("output/en.wikipedia.org/static/logo.png")
	m.addFile("output/ja.wikipedia.org/index.html")
	m.addFile("output/de.wikipedia.org/index.html")
	m.addFile("output/ru.wikipedia.org/index.html")

	// Focus tab content so cursor is highlighted
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)

	preview := m.View()
	t.Logf("\n=== FILES TAB PREVIEW ===\n%s\n", preview)
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "negative maxLen",
			input:  "hello",
			maxLen: -1,
			want:   "",
		},
		{
			name:   "zero maxLen",
			input:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "string fits within maxLen",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact fit",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "standard ascii truncation",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "maxLen 3 with no space for ellipsis",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen 2 with no space for ellipsis",
			input:  "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen 1 with no space for ellipsis",
			input:  "hello",
			maxLen: 1,
			want:   "h",
		},
		{
			name:   "cjk wide characters truncated safely",
			input:  "你好世界",
			maxLen: 5,
			want:   "你...",
		},
		{
			name:   "regression: slice bounds out of range with wide chars",
			input:  "你好",
			maxLen: 3,
			want:   "你",
		},
		{
			name:   "regression: emoji with maxLen 3",
			input:  "🔥🔥",
			maxLen: 3,
			want:   "🔥",
		},
		{
			name:   "regression: emoji with maxLen 2",
			input:  "🔥🔥",
			maxLen: 2,
			want:   "🔥",
		},
		{
			name:   "emojis with room for ellipsis",
			input:  "🔥🔥🔥🔥",
			maxLen: 7,
			want:   "🔥🔥...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRunes(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			if tt.maxLen > 0 {
				w := lipgloss.Width(got)
				if w > tt.maxLen {
					t.Errorf("truncateRunes(%q, %d) visual width = %d, exceeds maxLen %d", tt.input, tt.maxLen, w, tt.maxLen)
				}
			}
		})
	}
}

func TestFormatBytesCompact(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{500, "500B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{43100, "42.1K"},
		{1887436, "1.8M"},
		{1073741824, "1.0G"},
		{1099511627776, "1.0T"},
	}

	for _, tt := range tests {
		if got := formatBytesCompact(tt.bytes); got != tt.want {
			t.Errorf("formatBytesCompact(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

