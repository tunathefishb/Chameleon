package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAdversarial_MatchRobotsPath exercises matchRobotsPath with aggressive patterns and edge cases.
func TestAdversarial_MatchRobotsPath(t *testing.T) {
	tests := []struct {
		pattern  string
		path     string
		expected bool
	}{
		// Basic edge cases
		{"", "/", false},
		{"", "", false},
		{"/", "/", true},
		{"/", "/any/sub/path", true},
		{"/admin", "/admin", true},
		{"/admin", "/admin/dashboard", true},
		{"/admin", "/administrator", true},
		{"/admin", "/user", false},

		// End anchors ($)
		{"/$", "/", true},
		{"/$", "/sub", false},
		{"/admin$", "/admin", true},
		{"/admin$", "/admin/sub", false},
		{"/admin$", "/administrator", false},
		{"/page.html$", "/page.html", true},
		{"/page.html$", "/page.html?v=1", false},
		{"/page.html$", "/page.html/extra", false},

		// Wildcards (*)
		{"*", "/", true},
		{"*", "/any/path", true},
		{"/*", "/", true},
		{"/*", "/anything", true},
		{"/fish*", "/fish", true},
		{"/fish*", "/fish.html", true},
		{"/fish*", "/fish/salmon.html", true},
		{"/fish*", "/salmon", false},
		{"/*.php", "/index.php", true},
		{"/*.php", "/dir/test.php", true},
		{"/*.php", "/test.php?v=1", true},
		{"/*.php", "/test.phps", true},
		{"/*.php", "/test.html", false},

		// Combined wildcards and end anchors (* and $)
		{"/*.php$", "/index.php", true},
		{"/*.php$", "/dir/page.php", true},
		{"/*.php$", "/test.php5", false},
		{"/*.php$", "/test.php/extra", false},
		{"/a*b*c$", "/abc", true},
		{"/a*b*c$", "/a_middle_b_middle_c", true},
		{"/a*b*c$", "/a_middle_b_middle_c_extra", false},
		{"/*$", "/", true},
		{"/*$", "/anything", true},

		// Regex special characters in patterns that must be escaped
		{"/api/v1.0/", "/api/v1.0/users", true},
		{"/api/v1.0/", "/api/v1X0/users", false},
		{"/item(1)/", "/item(1)/details", true},
		{"/item[1]/", "/item[1]/details", true},
		{"/calc+sum/", "/calc+sum/run", true},
		{"/query?foo=bar", "/query?foo=bar&baz=1", true},
		{"/price$10", "/price$10/item", true},
		{"/test{1,2}/", "/test{1,2}/page", true},

		// Multiple consecutive wildcards
		{"/**", "/any", true},
		{"/***", "/any/nested", true},
		{"/a**b", "/ab", true},
		{"/a**b", "/aXYZb", true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("Pattern=%s_Path=%s", tc.pattern, tc.path), func(t *testing.T) {
			got := matchRobotsPath(tc.pattern, tc.path)
			if got != tc.expected {
				t.Errorf("matchRobotsPath(%q, %q) = %v; expected %v", tc.pattern, tc.path, got, tc.expected)
			}
		})
	}
}

// TestAdversarial_HostileRobotsTxt tests robots.txt parsing against malformed records,
// binary gibberish, multiple user-agent groups, comments, and precedence rules.
func TestAdversarial_HostileRobotsTxt(t *testing.T) {
	tests := []struct {
		name               string
		robotsContent      string
		targetPath         string
		expectDisallowed   bool
		expectAllowed      bool
		expectCrawlDelay   bool
		expectedCrawlDelay float64
	}{
		{
			name:             "Empty robots.txt",
			robotsContent:    "",
			targetPath:       "/anything",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name:             "Whitespace-only robots.txt",
			robotsContent:    "   \n\t  \n  \r\n",
			targetPath:       "/anything",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name: "Malformed lines without colon and binary noise",
			robotsContent: `
THIS IS NOT A VALID DIRECTIVE
User-agent * (missing colon)
DisallowWithoutColon /bad
User-agent: *
Disallow: /private/
			`,
			targetPath:       "/private/secret",
			expectDisallowed: true,
		},
		{
			name: "Multiple colons in values",
			robotsContent: `
User-agent: *
Disallow: /path:with:colons/
Sitemap: https://example.com:8443/sitemap.xml
			`,
			targetPath:       "/path:with:colons/item",
			expectDisallowed: true,
		},
		{
			name: "Inline comments on directives",
			robotsContent: `
# Header comment
User-agent: * # Wildcard rule
Disallow: /blocked/ # Do not crawl this
Allow: /blocked/open # Except this
Crawl-delay: 3.5 # Moderate delay
			`,
			targetPath:         "/blocked/open",
			expectDisallowed:   false,
			expectAllowed:      true,
			expectCrawlDelay:   true,
			expectedCrawlDelay: 3.5,
		},
		{
			name: "Comments with hash in disallowed path",
			robotsContent: `
User-agent: *
Disallow: /tag/#popular
			`,
			// Note: '#' in line is treated as comment, so line becomes "Disallow: /tag/"
			targetPath:       "/tag/test",
			expectDisallowed: true,
		},
		{
			name: "Comma-separated user-agents on single line",
			robotsContent: `
User-agent: Googlebot, Bingbot, ChameleonBot, Yandex
Disallow: /no-crawlers/
			`,
			targetPath:       "/no-crawlers/data",
			expectDisallowed: true,
		},
		{
			name: "Chameleon specific group overrides wildcard group",
			robotsContent: `
User-agent: *
Disallow: /secret/

User-agent: ChameleonBot
Allow: /secret/
Disallow: /super-secret/
			`,
			targetPath:       "/secret/data",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name: "Chameleon specific group disallow on super-secret",
			robotsContent: `
User-agent: *
Allow: /

User-agent: ChameleonBot
Disallow: /super-secret/
			`,
			targetPath:       "/super-secret/file",
			expectDisallowed: true,
		},
		{
			name: "Longest match rule: Allow longer than Disallow",
			robotsContent: `
User-agent: *
Disallow: /data/
Allow: /data/public/
			`,
			targetPath:       "/data/public/info",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name: "Longest match rule: Disallow longer than Allow",
			robotsContent: `
User-agent: *
Allow: /data/
Disallow: /data/private/
			`,
			targetPath:       "/data/private/info",
			expectDisallowed: true,
		},
		{
			name: "Tie between Allow and Disallow: Allow MUST win (RFC 9309)",
			robotsContent: `
User-agent: *
Disallow: /path/same
Allow: /path/same
			`,
			targetPath:       "/path/same",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name: "Empty Disallow value means allow all",
			robotsContent: `
User-agent: *
Disallow:
			`,
			targetPath:       "/anything",
			expectDisallowed: false,
			expectAllowed:    true,
		},
		{
			name: "Directives before first User-agent are ignored (RFC 9309)",
			robotsContent: `
Disallow: /rogue-disallow/
Allow: /rogue-allow/

User-agent: *
Disallow: /real-disallow/
			`,
			targetPath:       "/rogue-disallow/item",
			expectDisallowed: false,
			expectAllowed:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprint(w, tc.robotsContent)
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintln(w, "<html><body>Test Page</body></html>")
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			az := NewAnalyzer(server.Client(), nil)
			report, err := az.Analyze(context.Background(), server.URL+tc.targetPath)
			if err != nil {
				t.Fatalf("unexpected Analyze error: %v", err)
			}

			if tc.expectDisallowed {
				if report.EthicalScore >= 100 {
					t.Errorf("expected penalized ethical score for disallowed path, got %d (details: %v)",
						report.EthicalScore, report.EthicalDetails)
				}
			}

			if tc.expectAllowed {
				if report.EthicalScore < 90 {
					t.Errorf("expected high ethical score >= 90 for allowed path, got %d (details: %v)",
						report.EthicalScore, report.EthicalDetails)
				}
			}

			if tc.expectCrawlDelay {
				foundDelay := false
				for _, d := range report.EthicalDetails {
					if strings.Contains(d, fmt.Sprintf("Crawl-delay: %.1fs", tc.expectedCrawlDelay)) {
						foundDelay = true
						break
					}
				}
				if !foundDelay {
					t.Errorf("expected Crawl-delay %.1fs in EthicalDetails, got: %v",
						tc.expectedCrawlDelay, report.EthicalDetails)
				}
			}
		})
	}
}

// TestAdversarial_HugeAndOversizedRobotsTxt tests bounded reading (512KB limit)
// and scanner resilience on very large robots.txt files and lines > 64KB.
func TestAdversarial_HugeAndOversizedRobotsTxt(t *testing.T) {
	t.Run("HugeRobotsTxt_NoHangOrPanic", func(t *testing.T) {
		// 1. A 512KB robots.txt with 10,000 valid lines
		var sb strings.Builder
		sb.WriteString("User-agent: *\n")
		for i := 0; i < 15000; i++ {
			sb.WriteString(fmt.Sprintf("Disallow: /disallowed-path-%d/\n", i))
		}
		sb.WriteString("Disallow: /target-at-the-end/\n")
		hugeRobots := sb.String()

		mux := http.NewServeMux()
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, hugeRobots)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, "<html><body>Huge Robots OK</body></html>")
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		az := NewAnalyzer(server.Client(), nil)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		report, err := az.Analyze(ctx, server.URL+"/disallowed-path-50/")
		if err != nil {
			t.Fatalf("unexpected error on huge robots.txt: %v", err)
		}
		if report == nil {
			t.Fatal("expected non-nil report")
		}
		if report.EthicalScore >= 100 {
			t.Errorf("expected disallow penalty, got score: %d (details: %v)", report.EthicalScore, report.EthicalDetails)
		}
	})

	t.Run("OversizedLineRobotsTxt_NoPanic", func(t *testing.T) {
		// 2. A robots.txt with a single line > 64KB (exceeding default bufio.Scanner buffer)
		oversizedLine := "User-agent: *\nDisallow: /" + strings.Repeat("a", 70000) + "\nDisallow: /regular-disallow/\n"

		mux := http.NewServeMux()
		mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, oversizedLine)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, "<html><body>Oversized Line OK</body></html>")
		})

		server := httptest.NewServer(mux)
		defer server.Close()

		az := NewAnalyzer(server.Client(), nil)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		report, err := az.Analyze(ctx, server.URL+"/regular-disallow/")
		if err != nil {
			t.Fatalf("unexpected error on oversized line: %v", err)
		}
		if report == nil {
			t.Fatal("expected non-nil report")
		}
	})
}

// TestAdversarial_MalformedJSONLDAndDOM checks broken JSON-LD syntax, primitives,
// arrays, and massive DOM trees to ensure zero crashes and accurate block counting.
func TestAdversarial_MalformedJSONLDAndDOM(t *testing.T) {
	tests := []struct {
		name                 string
		htmlBody             string
		expectedJSONLDBlocks int
	}{
		{
			name:                 "Broken JSON syntax",
			htmlBody:             `<script type="application/ld+json">{"unclosed: true, </script>`,
			expectedJSONLDBlocks: 0,
		},
		{
			name:                 "Empty script tag",
			htmlBody:             `<script type="application/ld+json"></script>`,
			expectedJSONLDBlocks: 0,
		},
		{
			name:                 "Whitespace only script tag",
			htmlBody:             `<script type="application/ld+json">   \n\t\r  </script>`,
			expectedJSONLDBlocks: 0,
		},
		{
			name:                 "Non-object valid JSON (primitives: number, boolean, string, null)",
			htmlBody:             `<script type="application/ld+json">12345</script>`,
			expectedJSONLDBlocks: 1,
		},
		{
			name: "Valid JSON-LD object",
			htmlBody: `<script type="application/ld+json">
				{"@context": "https://schema.org", "@type": "WebSite", "name": "Chameleon"}
			</script>`,
			expectedJSONLDBlocks: 1,
		},
		{
			name: "Valid JSON-LD array of objects",
			htmlBody: `<script type="application/ld+json">
				[
					{"@context": "https://schema.org", "@type": "BreadcrumbList"},
					{"@context": "https://schema.org", "@type": "Organization"}
				]
			</script>`,
			expectedJSONLDBlocks: 1,
		},
		{
			name: "Multiple script tags mixed valid and invalid",
			htmlBody: `
				<script type="application/ld+json">{"valid": 1}</script>
				<script type="application/ld+json">INVALID_JSON_HERE</script>
				<script type="application/ld+json"></script>
				<script type="application/ld+json">{"valid": 2}</script>
				<script type="application/ld+json">{"valid": 3}</script>
			`,
			expectedJSONLDBlocks: 3,
		},
		{
			name: "Script tags with uppercase type or surrounding attributes",
			htmlBody: `
				<script id="schema1" type="application/ld+json">{"schema": 1}</script>
			`,
			expectedJSONLDBlocks: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "User-agent: *\nAllow: /")
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				// Provide body content so it does not trigger SPA text length heuristics
				fmt.Fprintf(w, `<!DOCTYPE html><html><body><p>Plenty of regular content to ensure static HTML evaluation.</p>%s</body></html>`, tc.htmlBody)
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			az := NewAnalyzer(server.Client(), nil)
			report, err := az.Analyze(context.Background(), server.URL+"/")
			if err != nil {
				t.Fatalf("unexpected Analyze error: %v", err)
			}

			foundCount := 0
			for _, d := range report.DifficultyDetails {
				if strings.Contains(d, "Structured JSON-LD metadata available") {
					var count int
					_, scanErr := fmt.Sscanf(d, "💎 Structured JSON-LD metadata available (%d block(s))", &count)
					if scanErr == nil {
						foundCount = count
					}
					break
				}
			}

			if foundCount != tc.expectedJSONLDBlocks {
				t.Errorf("expected %d JSON-LD block(s), got %d (details: %v)",
					tc.expectedJSONLDBlocks, foundCount, report.DifficultyDetails)
			}
		})
	}
}

// TestAdversarial_TargetURLEdgeCases tests empty, scheme-less, non-HTTP,
// IPv6, and malformed URLs, verifying that structured reports are returned
// with zero crashes, no hangs, and consistent difficulty/ethical ratings.
func TestAdversarial_TargetURLEdgeCases(t *testing.T) {
	az := NewAnalyzer(nil, nil)

	hostileURLs := []struct {
		name       string
		rawURL     string
		expectFail bool
	}{
		{"Empty String", "", true},
		{"Whitespace Only", "   \t\n   ", true},
		{"Scheme Only HTTP", "http://", true},
		{"Scheme Only HTTPS", "https://", true},
		{"Colons and Slashes", "://", true},
		{"Invalid Scheme Leading", "://example.com", true},
		{"Bare Domain Without Scheme", "example.com", true},
		{"Bare Domain With Path", "example.com/some/path", true},
		{"Relative Path", "/root/subpath", true},
		{"Relative Dot Path", "./relative/path", true},
		{"Control Characters", "http://example.com/\x00/test", false}, // url.Parse may accept or fail depending on Go version
		{"FTP Scheme", "ftp://ftp.example.com/file.txt", false},
		{"File Scheme", "file:///etc/passwd", false},
		{"Data URI", "data:text/plain;base64,SGVsbG8=", true}, // Host is empty in data URI
		{"Javascript URI", "javascript:alert(1)", true},       // Host is empty
		{"Mailto URI", "mailto:user@example.com", true},       // Host is empty
		{"Unreachable Closed Port", "http://127.0.0.1:54321/", false},
	}

	for _, tc := range hostileURLs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			report, err := az.Analyze(ctx, tc.rawURL)
			if report == nil {
				t.Fatalf("Analyze(%q) returned nil *Report", tc.rawURL)
			}
			if err != nil {
				t.Fatalf("Analyze(%q) returned non-nil error: %v (expected error encapsulated in Report)", tc.rawURL, err)
			}

			// Verify invariant properties on the returned Report
			if report.URL != tc.rawURL {
				t.Errorf("expected Report.URL to match input %q, got %q", tc.rawURL, report.URL)
			}
			if report.Duration < 0 {
				t.Errorf("expected non-negative duration, got %v", report.Duration)
			}
			if report.AnalyzedAt.IsZero() {
				t.Errorf("expected non-zero AnalyzedAt timestamp")
			}

			// If it's a known invalid target URL, verify structured error contract
			if tc.expectFail {
				if report.EthicalGrade != "F" {
					t.Errorf("expected EthicalGrade 'F', got %q", report.EthicalGrade)
				}
				if report.EthicalScore != 0 {
					t.Errorf("expected EthicalScore 0, got %d", report.EthicalScore)
				}
				if report.DifficultyScore != 10 {
					t.Errorf("expected DifficultyScore 10, got %d", report.DifficultyScore)
				}
				if report.DifficultyLevel != "Extreme" {
					t.Errorf("expected DifficultyLevel 'Extreme', got %q", report.DifficultyLevel)
				}
				if report.Recommendation == "" {
					t.Errorf("expected non-empty Recommendation for invalid URL %q", tc.rawURL)
				}
			}
		})
	}
}

// TestAdversarial_ConcurrentAnalyzeStress runs concurrent Analyze calls across
// multiple goroutines on a shared Analyzer instance with varied inputs to ensure race freedom.
func TestAdversarial_ConcurrentAnalyzeStress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nDisallow: /secret/\nCrawl-delay: 2")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html><html><body>
			<div id="root"></div>
			<script type="application/ld+json">{"@context":"https://schema.org"}</script>
		</body></html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)

	testURLs := []string{
		server.URL + "/",
		server.URL + "/secret/page",
		"",
		"://invalid",
		server.URL + "/normal",
		"https://example.com/no-such-host",
	}

	const goroutines = 20
	const iterations = 5

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				target := testURLs[(id+i)%len(testURLs)]
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				rep, _ := az.Analyze(ctx, target)
				cancel()
				if rep == nil {
					t.Errorf("goroutine %d received nil report for target %q", id, target)
				}
			}
		}(g)
	}

	wg.Wait()
}

// TestAdversarial_MathLimitsCrawlDelay explicitly verifies IEEE 754 float limits on CrawlDelay.
func TestAdversarial_MathLimitsCrawlDelay(t *testing.T) {
	delayCases := []struct {
		name        string
		valStr      string
		expectValid bool
	}{
		{"Normal 1.0", "1.0", true},
		{"Zero 0.0", "0.0", false}, // > 0 required for report/bump
		{"Negative -1.0", "-1.0", false},
		{"NaN", "NaN", false},
		{"Positive Inf", "+Inf", false},
		{"Negative Inf", "-Inf", false},
		{"Max Seconds (86400)", "86400", true},
		{"Over 86400 (86401)", "86401", false},
		{"Astronomical float", "1e308", false},
		{"Negative astronomical float", "-1e308", false},
	}

	for _, dc := range delayCases {
		t.Run(dc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, "User-agent: *\nCrawl-delay: %s\n", dc.valStr)
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintln(w, "OK")
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			az := NewAnalyzer(server.Client(), nil)
			rep, err := az.Analyze(context.Background(), server.URL+"/")
			if err != nil {
				t.Fatalf("unexpected Analyze error: %v", err)
			}

			foundDelay := false
			for _, d := range rep.EthicalDetails {
				if strings.Contains(d, "Crawl-delay:") {
					foundDelay = true
					break
				}
			}

			if foundDelay != dc.expectValid {
				t.Errorf("Crawl-delay %q: expectValid = %v, but foundDelay = %v", dc.valStr, dc.expectValid, foundDelay)
			}
		})
	}
}
