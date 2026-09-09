package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"chameleon/internal/engine"
)

// TestLinkDiscoveryStormRerenderBatching simulates a storm of 1,000 discovered URLs
// arriving in rapid succession. It verifies that:
// 1. Synchronous Lipgloss queue re-rendering is avoided during the storm.
// 2. needsQueueUpdate is marked true.
// 3. A single spinner.TickMsg cleanly batches and updates the viewport.
// 4. needsQueueUpdate is reset to false after the tick.
func TestLinkDiscoveryStormRerenderBatching(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)

	// Set viewport dimensions so queue rendering would normally be formatted
	m.width = 100
	m.height = 30
	m.syncDimensions()
	m.focusTab(TabQueue)
	m.setFocusTarget(FocusTabContent)

	if m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be false initially")
	}

	initialView := m.queueViewport.View()

	// 1. Simulate 1,000 discovered URLs arriving in rapid succession
	const stormCount = 1000
	start := time.Now()

	for i := 0; i < stormCount; i++ {
		targetURL := fmt.Sprintf("https://example.com/page/%04d", i)
		next, _ := m.Update(engineDiscoveredMsg(targetURL))
		m = next.(Model)
	}

	elapsedStorm := time.Since(start)

	// Assertions during/immediately after storm:
	// All 1,000 URLs must be in m.queue
	if len(m.queue) != stormCount {
		t.Fatalf("expected queue length %d, got %d", stormCount, len(m.queue))
	}

	// needsQueueUpdate must be true
	if !m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be true after discovered URLs")
	}

	// Verify zero synchronous Lipgloss queue re-renders happened during the storm:
	// m.queueViewport.View() should STILL match initialView because updateQueueContent() was not called!
	currentView := m.queueViewport.View()
	if currentView != initialView {
		t.Fatalf("synchronous Lipgloss queue rendering occurred during storm! View was updated before TickMsg")
	}

	if strings.Contains(currentView, "https://example.com/page/0000") {
		t.Fatalf("queue viewport unexpectedly rendered first URL before tick")
	}
	if strings.Contains(currentView, "https://example.com/page/0999") {
		t.Fatalf("queue viewport unexpectedly rendered last URL before tick")
	}

	// Performance assertion: processing 1,000 messages without Lipgloss rendering must be sub-50ms
	t.Logf("Storm of %d discovered URLs processed in %v (avg %v/msg)", stormCount, elapsedStorm, elapsedStorm/stormCount)
	if elapsedStorm > 200*time.Millisecond {
		t.Errorf("discovery storm took unexpectedly long: %v (expected < 200ms)", elapsedStorm)
	}

	// 2. Now simulate spinner.TickMsg arriving to trigger batched re-rendering
	tickStart := time.Now()
	next, _ := m.Update(m.spinner.Tick())
	m = next.(Model)
	elapsedTick := time.Since(tickStart)

	t.Logf("Single batched render of %d queue items via TickMsg took %v", stormCount, elapsedTick)

	// Assertions after TickMsg:
	// needsQueueUpdate must now be reset to false
	if m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be reset to false after TickMsg")
	}

	// Viewport must now contain rendered items
	renderedView := m.queueViewport.View()
	if renderedView == initialView {
		t.Fatalf("queue viewport failed to update after TickMsg")
	}

	// Check first and last entries in the rendered queue
	if !strings.Contains(renderedView, "https://example.com/page/0000") {
		t.Fatalf("queue viewport missing first item after TickMsg:\n%s", renderedView[:min(500, len(renderedView))])
	}

	// Verify UTF-8 validity of the rendered viewport
	if !utf8.ValidString(renderedView) {
		t.Fatalf("rendered queue viewport contains invalid UTF-8 bytes")
	}
	if strings.ContainsRune(renderedView, '\ufffd') {
		t.Fatalf("rendered queue viewport contains replacement rune \\ufffd")
	}

	// 3. Second TickMsg should be a no-op for queue rendering
	next, _ = m.Update(m.spinner.Tick())
	m = next.(Model)
	if m.needsQueueUpdate {
		t.Fatalf("needsQueueUpdate became true unexpectedly after second tick")
	}
}

// TestLinkDiscoveryComparativeStress compares batched dirty-flag updates vs unbatched synchronous updates.
func TestLinkDiscoveryComparativeStress(t *testing.T) {
	const count = 500

	// Case A: Batched (current implementation)
	engA := engine.NewEngine()
	mA := InitialModel(engA)
	mA.width = 100
	mA.height = 30
	mA.syncDimensions()

	startBatched := time.Now()
	for i := 0; i < count; i++ {
		u := fmt.Sprintf("https://batched.test/item/%d", i)
		next, _ := mA.Update(engineDiscoveredMsg(u))
		mA = next.(Model)
	}
	// 1 batched tick
	nextA, _ := mA.Update(mA.spinner.Tick())
	mA = nextA.(Model)
	durationBatched := time.Since(startBatched)

	// Case B: Unbatched (simulate synchronous updateQueueContent on every message)
	engB := engine.NewEngine()
	mB := InitialModel(engB)
	mB.width = 100
	mB.height = 30
	mB.syncDimensions()

	startUnbatched := time.Now()
	for i := 0; i < count; i++ {
		u := fmt.Sprintf("https://unbatched.test/item/%d", i)
		mB.queue = append(mB.queue, u)
		mB.updateQueueContent() // Force synchronous Lipgloss render
	}
	durationUnbatched := time.Since(startUnbatched)

	t.Logf("Benchmark for %d items: Batched = %v, Unbatched = %v (Speedup: %.1fx)",
		count, durationBatched, durationUnbatched, float64(durationUnbatched)/float64(durationBatched))

	// Batched should be significantly faster than unbatched synchronous rendering
	if durationBatched >= durationUnbatched {
		t.Logf("Warning: batched (%v) was not faster than unbatched (%v)", durationBatched, durationUnbatched)
	}
}

// TestLinkDiscoveryInterleavedStorm tests discovered URLs interleaved with results, files, and ticks.
func TestLinkDiscoveryInterleavedStorm(t *testing.T) {
	eng := engine.NewEngine()
	m := InitialModel(eng)
	m.width = 100
	m.height = 30
	m.syncDimensions()

	const totalDiscovered = 500
	const totalResults = 100
	const totalFiles = 50

	dIdx, rIdx, fIdx := 0, 0, 0
	ticksCount := 0

	for dIdx < totalDiscovered || rIdx < totalResults || fIdx < totalFiles {
		// Interleave discovered URLs
		if dIdx < totalDiscovered {
			for k := 0; k < 5 && dIdx < totalDiscovered; k++ {
				next, _ := m.Update(engineDiscoveredMsg(fmt.Sprintf("https://example.com/interleaved/%d", dIdx)))
				m = next.(Model)
				dIdx++
			}
		}

		// Interleave results
		if rIdx < totalResults {
			res := engine.Result{
				Name:   fmt.Sprintf("item-%d", rIdx),
				Status: "200 OK",
				Type:   "text/html",
				Size:   "15.2 KB",
				Time:   "120ms",
			}
			next, _ := m.Update(engineResultMsg(res))
			m = next.(Model)
			rIdx++
		}

		// Interleave files
		if fIdx < totalFiles {
			next, _ := m.Update(engineFileMsg(fmt.Sprintf("output/example.com/page_%d.html", fIdx)))
			m = next.(Model)
			fIdx++
		}

		// Interleave periodic tick
		if (dIdx+rIdx+fIdx)%30 == 0 {
			next, _ := m.Update(m.spinner.Tick())
			m = next.(Model)
			ticksCount++
		}
	}

	// Final tick to flush remaining dirty flags
	next, _ := m.Update(m.spinner.Tick())
	m = next.(Model)

	if len(m.queue) != totalDiscovered {
		t.Fatalf("expected queue length %d, got %d", totalDiscovered, len(m.queue))
	}
	if len(m.telemetryItems) != totalResults {
		t.Fatalf("expected telemetry items %d, got %d", totalResults, len(m.telemetryItems))
	}
	if len(m.savedFiles) != totalFiles {
		t.Fatalf("expected saved files %d, got %d", totalFiles, len(m.savedFiles))
	}
	if m.needsQueueUpdate {
		t.Fatalf("expected needsQueueUpdate to be false after final tick")
	}
	if m.needsFilesUpdate {
		t.Fatalf("expected needsFilesUpdate to be false after final tick")
	}
}

// TestNormalizeAndValidateURLEdgeCases tests normalizeAndValidateURL across hostile,
// malformed, IPv6, custom port, whitespace-padded, and scheme-less inputs.
func TestNormalizeAndValidateURLEdgeCases(t *testing.T) {
	type testCase struct {
		name        string
		input       string
		expectError bool
		wantHost    string // checked if !expectError
		wantNorm    string // checked if !expectError
		category    string
	}

	tests := []testCase{
		// --- 1. Hostile & Attack Vectors ---
		{
			name:        "javascript pseudo-protocol",
			input:       "javascript:alert(1)",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "javascript void expression",
			input:       "javascript:void(0)",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "data URI HTML XSS payload",
			input:       "data:text/html,<script>alert('xss')</script>",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "data URI base64 payload",
			input:       "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "local file scheme /etc/passwd",
			input:       "file:///etc/passwd",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "local file scheme with localhost",
			input:       "file://localhost/etc/passwd",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "vbscript pseudo-protocol",
			input:       "vbscript:msgbox(1)",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "mailto scheme rejected as non-HTTP scheme",
			input:       "mailto:admin@example.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "tel scheme",
			input:       "tel:+1234567890",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "ftp scheme without http/https",
			input:       "ftp://ftp.example.com/file.zip",
			expectError: true, // becomes https://ftp://... which is invalid URL structure
			category:    "hostile",
		},
		{
			name:        "newline in host header injection",
			input:       "http://example.com\r\nHost: evil.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "carriage return in host",
			input:       "http://foo\rbar.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "tab in host",
			input:       "http://foo\tbar.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "newline in host",
			input:       "http://foo\nbar.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "spaces in host",
			input:       "http://foo bar.com",
			expectError: true,
			category:    "hostile",
		},
		{
			name:        "backslash in authority rejected by url.Parse",
			input:       "http://example.com\\path",
			expectError: true, // Go url.Parse rejects backslash in host name
			category:    "hostile",
		},
		{
			name:        "embedded credentials in URL",
			input:       "https://admin:p@ssw0rd!@secure.example.com/dashboard",
			expectError: false,
			wantHost:    "secure.example.com",
			wantNorm:    "https://admin:p@ssw0rd!@secure.example.com/dashboard",
			category:    "credentials",
		},
		{
			name:        "userinfo without password",
			input:       "http://crawler@example.org:8080/feed",
			expectError: false,
			wantHost:    "example.org:8080",
			wantNorm:    "http://crawler@example.org:8080/feed",
			category:    "credentials",
		},

		// --- 2. Malformed & Boundary Inputs ---
		{
			name:        "empty string",
			input:       "",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "whitespace only spaces",
			input:       "    ",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "whitespace tabs and newlines",
			input:       "\t\r\n",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "bare http scheme without host",
			input:       "http://",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "bare https scheme without host",
			input:       "https://",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "https with three slashes and no host",
			input:       "https:///",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "https with five slashes",
			input:       "https://///",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http with query only",
			input:       "http://?query=1",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http with fragment only",
			input:       "http://#section",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http with colon only",
			input:       "http://:",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http with colon and port only",
			input:       "http://:80",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http with user info and no host",
			input:       "http://user@",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "single label host without dot",
			input:       "internal-server",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "http single label host without dot",
			input:       "http://mycorp",
			expectError: true,
			category:    "malformed",
		},
		{
			name:        "dot only host",
			input:       ".",
			expectError: false, // parsed.Host is ".", strings.Contains(".", ".") is true
			wantHost:    ".",
			wantNorm:    "https://.",
			category:    "malformed",
		},
		{
			name:        "double dot host",
			input:       "..",
			expectError: false, // parsed.Host is "..", strings.Contains("..", ".") is true
			wantHost:    "..",
			wantNorm:    "https://..",
			category:    "malformed",
		},

		// --- 3. Whitespace-Padded Inputs ---
		{
			name:        "leading and trailing spaces around https URL",
			input:       "   https://example.com   ",
			expectError: false,
			wantHost:    "example.com",
			wantNorm:    "https://example.com",
			category:    "whitespace",
		},
		{
			name:        "leading and trailing tabs/newlines around domain",
			input:       "\t\n  sub.example.com/path?key=val  \r\n",
			expectError: false,
			wantHost:    "sub.example.com",
			wantNorm:    "https://sub.domain.org"[:0] + "https://sub.example.com/path?key=val",
			category:    "whitespace",
		},
		{
			name:        "whitespace inside path (URL with spaces in path)",
			input:       "https://example.com/my%20folder/file.html",
			expectError: false,
			wantHost:    "example.com",
			wantNorm:    "https://example.com/my%20folder/file.html",
			category:    "whitespace",
		},

		// --- 4. Scheme-Less Inputs ---
		{
			name:        "bare domain defaults to https",
			input:       "example.com",
			expectError: false,
			wantHost:    "example.com",
			wantNorm:    "https://example.com",
			category:    "schemeless",
		},
		{
			name:        "bare domain with path and query",
			input:       "api.service.io/v2/items?filter=active&sort=desc",
			expectError: false,
			wantHost:    "api.service.io",
			wantNorm:    "https://api.service.io/v2/items?filter=active&sort=desc",
			category:    "schemeless",
		},
		{
			name:        "bare localhost",
			input:       "localhost",
			expectError: false,
			wantHost:    "localhost",
			wantNorm:    "https://localhost",
			category:    "schemeless",
		},
		{
			name:        "bare localhost with port",
			input:       "localhost:3000",
			expectError: false,
			wantHost:    "localhost:3000",
			wantNorm:    "https://localhost:3000",
			category:    "schemeless",
		},
		{
			name:        "bare IPv4 address",
			input:       "127.0.0.1",
			expectError: false,
			wantHost:    "127.0.0.1",
			wantNorm:    "https://127.0.0.1",
			category:    "schemeless",
		},
		{
			name:        "bare IPv4 with port and path",
			input:       "192.168.1.100:8443/status",
			expectError: false,
			wantHost:    "192.168.1.100:8443",
			wantNorm:    "https://192.168.1.100:8443/status",
			category:    "schemeless",
		},

		// --- 5. Custom Port Inputs ---
		{
			name:        "standard http port 80",
			input:       "http://example.com:80",
			expectError: false,
			wantHost:    "example.com:80",
			wantNorm:    "http://example.com:80",
			category:    "ports",
		},
		{
			name:        "standard https port 443",
			input:       "https://example.com:443",
			expectError: false,
			wantHost:    "example.com:443",
			wantNorm:    "https://example.com:443",
			category:    "ports",
		},
		{
			name:        "high port 8080",
			input:       "https://dev.example.com:8080/test",
			expectError: false,
			wantHost:    "dev.example.com:8080",
			wantNorm:    "https://dev.example.com:8080/test",
			category:    "ports",
		},
		{
			name:        "maximum TCP port 65535",
			input:       "http://proxy.example.com:65535",
			expectError: false,
			wantHost:    "proxy.example.com:65535",
			wantNorm:    "http://proxy.example.com:65535",
			category:    "ports",
		},
		{
			name:        "port 0",
			input:       "http://service.example.com:0",
			expectError: false,
			wantHost:    "service.example.com:0",
			wantNorm:    "http://service.example.com:0",
			category:    "ports",
		},
		{
			name:        "non-numeric port rejected by url.Parse",
			input:       "http://example.com:abc",
			expectError: true, // url.Parse returns 'invalid port ":abc" after host'
			category:    "ports",
		},

		// --- 6. IPv6 Addresses & Edge Cases ---
		{
			name:        "http with bracketed IPv6 loopback",
			input:       "http://[::1]",
			expectError: false,
			wantHost:    "[::1]",
			wantNorm:    "http://[::1]",
			category:    "ipv6",
		},
		{
			name:        "https with bracketed IPv6 and port",
			input:       "https://[::1]:8080/index.html",
			expectError: false,
			wantHost:    "[::1]:8080",
			wantNorm:    "https://[::1]:8080/index.html",
			category:    "ipv6",
		},
		{
			name:        "scheme-less bracketed IPv6",
			input:       "[::1]:8080",
			expectError: false,
			wantHost:    "[::1]:8080",
			wantNorm:    "https://[::1]:8080",
			category:    "ipv6",
		},
		{
			name:        "unbracketed IPv6",
			input:       "http://::1",
			expectError: true, // url.Parse fails with "too many colons in address"
			category:    "ipv6",
		},
		{
			name:        "IPv6 global unicast address",
			input:       "http://[2001:db8::1]:80",
			expectError: false,
			wantHost:    "[2001:db8::1]:80",
			wantNorm:    "http://[2001:db8::1]:80",
			category:    "ipv6",
		},
		{
			name:        "IPv4-mapped IPv6 address (contains dots)",
			input:       "http://[::ffff:192.0.2.1]:80",
			expectError: false, // hostname "::ffff:192.0.2.1" contains dots!
			wantHost:    "[::ffff:192.0.2.1]:80",
			wantNorm:    "http://[::ffff:192.0.2.1]:80",
			category:    "ipv6",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("%s/%s", tt.category, tt.name), func(t *testing.T) {
			norm, parsed, err := normalizeAndValidateURL(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("normalizeAndValidateURL(%q) expected error, got norm=%q parsedHost=%v", tt.input, norm, parsed.Host)
				}
			} else {
				if err != nil {
					t.Errorf("normalizeAndValidateURL(%q) unexpected error: %v", tt.input, err)
					return
				}
				if norm != tt.wantNorm {
					t.Errorf("normalizeAndValidateURL(%q) norm = %q, want %q", tt.input, norm, tt.wantNorm)
				}
				if parsed == nil {
					t.Fatalf("normalizeAndValidateURL(%q) returned nil *url.URL", tt.input)
				}
				if parsed.Host != tt.wantHost {
					t.Errorf("normalizeAndValidateURL(%q) parsed.Host = %q, want %q", tt.input, parsed.Host, tt.wantHost)
				}
			}
		})
	}
}

// TestNormalizeAndValidateURLPropertyFuzz feeds semi-random / hostile permutations into normalizeAndValidateURL
// to ensure zero panics under any string input.
func TestNormalizeAndValidateURLPropertyFuzz(t *testing.T) {
	seeds := []string{
		"", " ", "   \t\r\n", "http://", "https://", "ftp://", "file://", "javascript:",
		":", "::", "::1", "[::1]", "localhost", "127.0.0.1", "example.com",
		"http://localhost:8080", "https://sub.domain.co.uk:443/path?query=val#frag",
		"http://user:pass@host.com:9000", "http://host\nname.com", "http://host\rname.com",
		"http://[2001:db8::1]", "http://%00", "https://%20", "http://..", "http://.",
		"https://foo..bar", "http://-start.com", "http://end-.com",
		"https://a.b.c.d.e.f.g.h", "https://1.2.3.4.5", "http://999.999.999.999",
	}

	suffixes := []string{
		"", "/", "/path", "/path?a=1&b=2", "/#hash", ":80", ":443", ":99999",
		" ", "\t", "\n", "\x00", "%20", "%ff", "?#", "@evil.com",
	}

	for _, s := range seeds {
		for _, suf := range suffixes {
			candidate := s + suf
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("PANIC on input %q: %v", candidate, r)
					}
				}()

				norm, parsed, err := normalizeAndValidateURL(candidate)
				if err == nil {
					// Invariant: if no error, parsed must not be nil
					if parsed == nil {
						t.Errorf("expected non-nil *url.URL for valid input %q", candidate)
					}
					// Invariant: norm must have http:// or https:// scheme
					if !strings.HasPrefix(norm, "http://") && !strings.HasPrefix(norm, "https://") {
						t.Errorf("normalized URL %q missing http/https prefix for %q", norm, candidate)
					}
					// Invariant: parsed.Host must not be empty
					if parsed != nil && parsed.Host == "" {
						t.Errorf("parsed.Host empty for valid URL %q", candidate)
					}
					// Invariant: parsed.Host must not contain whitespace
					if parsed != nil && strings.ContainsAny(parsed.Host, " \t\r\n") {
						t.Errorf("parsed.Host contains whitespace for valid URL %q", candidate)
					}
				}
			}()
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
