package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"chameleon/internal/engine"
	"github.com/charmbracelet/lipgloss"
)

// TestAdversarialTruncateRunes tests truncateRunes with hostile strings and maxLens 0..10.
func TestAdversarialTruncateRunes(t *testing.T) {
	hostileStrings := []struct {
		category string
		value    string
	}{
		// 1. 4-byte emojis
		{"emoji_single", "🔥"},
		{"emoji_multi", "🔥🚀🍕🦀🌈💎🦄🎉"},
		{"emoji_with_text", "Status: 🚀 Launching... 🍕 Done!"},

		// 2. ZWJ sequences
		{"zwj_family", "👨‍👩‍👧‍👦"},
		{"zwj_flag", "🏳️‍🌈"},
		{"zwj_woman_tech", "👩‍💻"},
		{"zwj_holding_hands", "🧑‍🤝‍🧑"},
		{"zwj_polar_bear", "🐻‍❄️"},
		{"zwj_farmer_skin", "👨🏽‍🌾"},

		// 3. CJK full-width characters
		{"cjk_chinese", "你好世界，欢迎使用Chameleon爬虫！"},
		{"cjk_japanese", "こんにちは世界、日本語テキストのテスト"},
		{"cjk_korean", "안녕하세요 세계, 크롤러 테스트입니다"},
		{"cjk_fullwidth_punct", "！？【】（）《》「」『』"},
		{"cjk_fullwidth_ascii", "ＡＢＣＤＥＦＧＨＩＪＫＬＭＮ"},

		// 4. Mixed scripts
		{"mixed_latin_cjk_emoji", "Chameleon 爬虫 🚀 v1.0.0"},
		{"mixed_rtl_arabic", "Hello عالم من الزواحف 🕷️"},
		{"mixed_rtl_hebrew", "Test שלום עולם 123"},
		{"mixed_combining_diacritics", "Z\u0301\u0302\u0303a\u0300\u0308l\u0304g\u0305o\u0306"},
		{"mixed_cyrillic_greek", "Привет мир / Γειά σου κόσμε"},

		// 5. ANSI escape codes
		{"ansi_color16", "\x1b[31mRed Alert\x1b[0m"},
		{"ansi_bold_color", "\x1b[1;32;40mBoldGreenOnBlack\x1b[0m"},
		{"ansi_color256", "\x1b[38;5;196mRed256Text\x1b[0m"},
		{"ansi_truecolor", "\x1b[38;2;255;128;64mTrueColorText\x1b[0m"},
		{"ansi_cursor_clear", "\x1b[2J\x1b[HTextAfterClear"},
		{"ansi_malformed_partial", "\x1b[31mUnclosedColor"},
		{"ansi_bare_escapes", "\x1b[\x1b]bare"},

		// 6. Boundary strings
		{"empty", ""},
		{"single_space", " "},
		{"spaces_only", "    "},
		{"only_zwj", "\u200D\u200D\u200D"},
		{"combining_only", "\u0300\u0301\u0302\u0303"},
	}

	for _, tc := range hostileStrings {
		tc := tc
		t.Run(tc.category, func(t *testing.T) {
			// Test maxLen from 0 to 10 (as strictly requested) plus negative and larger values
			for maxLen := -2; maxLen <= 25; maxLen++ {
				// Assert zero slice bounds panics
				var res string
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("PANIC on input %q (%s) with maxLen %d: %v", tc.value, tc.category, maxLen, r)
						}
					}()
					res = truncateRunes(tc.value, maxLen)
				}()

				// Verify UTF-8 validity
				if !utf8.ValidString(res) {
					t.Errorf("truncateRunes(%q, %d) produced invalid UTF-8 string: %x", tc.value, maxLen, res)
				}

				// Verify zero invalid replacement bytes
				if strings.ContainsRune(res, '\ufffd') && !strings.ContainsRune(tc.value, '\ufffd') {
					t.Errorf("truncateRunes(%q, %d) introduced \\ufffd replacement character: %q", tc.value, maxLen, res)
				}

				// Verify negative or zero maxLen yields empty string
				if maxLen <= 0 {
					if res != "" {
						t.Errorf("truncateRunes(%q, %d) = %q, want empty string", tc.value, maxLen, res)
					}
					continue
				}

				// Verify strict adherence to max visual width
				w := lipgloss.Width(res)
				if w > maxLen {
					t.Errorf("truncateRunes(%q, %d) visual width = %d, strictly exceeds maxLen %d (result: %q)", tc.value, maxLen, w, maxLen, res)
				}
			}
		})
	}
}

// TestVariationSelectorVisualWidthOverflow reproduces the systematic bug where characters
// with Variation Selector-16 (U+FE0F) cause truncateRunes output to exceed maxLen by 1 column.
func TestVariationSelectorVisualWidthOverflow(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"warning_sign", "⚠️ Alert!"},
		{"heart_symbol", "❤️ Love"},
		{"rainbow_flag", "🏳️‍🌈 Rainbow"},
		{"envelope", "✉️ Mailbox"},
		{"telephone", "☎️ Support"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for maxLen := 1; maxLen <= 8; maxLen++ {
				res := truncateRunes(tc.input, maxLen)
				w := lipgloss.Width(res)
				if w > maxLen {
					t.Errorf("FAIL [BUG CONFIRMED]: truncateRunes(%q, maxLen=%d) = %q (visual width %d > maxLen %d)",
						tc.input, maxLen, res, w, maxLen)
				}
			}
		})
	}
}

// TestDanglingZWJSequenceTruncation verifies whether truncating ZWJ sequences leaves
// dangling zero-width joiners (\u200D) or variation selectors before ellipsis.
func TestDanglingZWJSequenceTruncation(t *testing.T) {
	cases := []struct {
		input  string
		maxLen int
	}{
		{"🏳️‍🌈flags", 4},
		{"👩‍💻developer", 5},
		{"👨‍👩‍👧‍👦family", 5},
	}

	for _, tc := range cases {
		res := truncateRunes(tc.input, tc.maxLen)
		cleanPrefix := strings.TrimSuffix(res, "...")
		if strings.HasSuffix(cleanPrefix, "\u200D") {
			t.Errorf("FAIL [DEFECT CONFIRMED]: truncateRunes(%q, %d) = %q leaves dangling ZWJ (\\u200D) before ellipsis",
				tc.input, tc.maxLen, res)
		}
	}
}

// TestPanelQueueURLTruncation tests internationalized URLs in panel_queue for UTF-8 validity and zero \ufffd.
func TestPanelQueueURLTruncation(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	testURLs := []string{
		"https://example.com/über/café",
		"https://中国.icom.museum/中文/路径/页面.html",
		"https://кто.рф/страница/каталог",
		"https://موقع.وزارة-الاتصالات.مصر/صفحة/رئيسية",
		"https://example.com/search?q=🚀🔥🍕&category=🦀",
		"https://example.com/family/👨‍👩‍👧‍👦/profile",
		"https://example.com/test?name=André&city=São%20Paulo&tag=日本語",
		"https://en.wikipedia.org/wiki/List_of_Unicode_characters",
		"https://sub.sub2.example.com/very/long/nested/path/to/a/deep/resource/with/query/parameters?param1=value1&param2=value2&special=こんにちは",
		"https://\x1b[31mhostile\x1b[0m.com/test",
	}

	for _, u := range testURLs {
		m.addQueue(u)
	}

	// Test various viewport widths: very narrow to wide
	widths := []int{10, 15, 20, 25, 30, 40, 60, 80, 120}

	for _, w := range widths {
		t.Run(fmt.Sprintf("viewport_width_%d", w), func(t *testing.T) {
			m.queueViewport.Width = w
			m.queueViewport.Height = 20
			m.queueCursor = 0

			// Render queue content
			m.updateQueueContent()
			view := m.queueViewport.View()

			// 1. Verify valid UTF-8
			if !utf8.ValidString(view) {
				t.Fatalf("queueViewport.View() contains invalid UTF-8 for width %d", w)
			}

			// 2. Verify zero \ufffd replacement characters introduced
			if strings.ContainsRune(view, '\ufffd') {
				t.Fatalf("queueViewport.View() contains \\ufffd replacement character for width %d:\n%s", w, view)
			}

			// 3. Test navigation through all items without panicking or corrupting UTF-8
			for cursor := 0; cursor < len(testURLs); cursor++ {
				m.queueCursor = cursor
				m.updateQueueContent()
				cView := m.queueViewport.View()

				if !utf8.ValidString(cView) {
					t.Fatalf("cursor %d, width %d produced invalid UTF-8", cursor, w)
				}
				if strings.ContainsRune(cView, '\ufffd') {
					t.Fatalf("cursor %d, width %d introduced \\ufffd", cursor, w)
				}
			}
		})
	}
}
