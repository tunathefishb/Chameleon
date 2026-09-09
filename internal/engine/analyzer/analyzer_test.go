package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAnalyzer_FriendlySite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nAllow: /\nSitemap: https://example.com/sitemap.xml")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head><title>Open Encyclopedia</title></head>
<body>
  <h1>Public Articles</h1>
  <p>Here is plenty of semantic text that can be easily parsed by any scraper without Javascript rendering required.</p>
  <script type="application/ld+json">{"@context": "https://schema.org", "@type": "NewsArticle"}</script>
</body>
</html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)
	report, err := az.Analyze(context.Background(), server.URL+"/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.EthicalGrade != "A" {
		t.Errorf("expected EthicalGrade A, got %s (score: %d)", report.EthicalGrade, report.EthicalScore)
	}
	if report.DifficultyLevel != "Easy" {
		t.Errorf("expected DifficultyLevel Easy, got %s (score: %d)", report.DifficultyLevel, report.DifficultyScore)
	}
	if report.DifficultyScore > 2 {
		t.Errorf("expected low difficulty score <= 2, got %d", report.DifficultyScore)
	}
}

func TestAnalyzer_ProtectedSite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nDisallow: /\nCrawl-delay: 10")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.Header().Set("cf-ray", "8f3123abc456")
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head>
  <title>App</title>
  <meta name="robots" content="noindex, nofollow">
</head>
<body>
  <div id="root"></div>
  <a href="/trap" style="display: none">Invisible Honeypot</a>
  <script src="/static/js/main.123abc45.js"></script>
</body>
</html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)
	report, err := az.Analyze(context.Background(), server.URL+"/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.EthicalGrade != "F" && report.EthicalGrade != "D" {
		t.Errorf("expected poor ethical grade (D or F), got %s (score: %d)", report.EthicalGrade, report.EthicalScore)
	}
	if report.DifficultyScore < 5 {
		t.Errorf("expected high difficulty score >= 5, got %d", report.DifficultyScore)
	}
	if report.RateLimitInfo == "" {
		t.Errorf("expected rate limit info, got empty string")
	}
}

type mockHeadlessDriver struct{}

func (m *mockHeadlessDriver) CheckPage(_ context.Context, _ string) (*HeadlessResult, error) {
	return &HeadlessResult{
		RenderedHTML: "<html><body>Rendered</body></html>",
		DOMMutations: 15,
		ExecutedJS:   true,
	}, nil
}

func TestAnalyzer_WithHeadless(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "<html><body>Hello</body></html>")
	}))
	defer server.Close()

	az := NewAnalyzer(server.Client(), &mockHeadlessDriver{})
	report, err := az.Analyze(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.HeadlessTested {
		t.Errorf("expected HeadlessTested to be true")
	}
}

func TestAnalyzer_RobotsAllowOverride(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nDisallow: /\nAllow: /public/")
	})
	mux.HandleFunc("/public/article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><p>Public Content</p></body></html>")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)
	report, err := az.Analyze(context.Background(), server.URL+"/public/article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.EthicalScore < 90 {
		t.Errorf("expected high ethical score >= 90 due to Allow rule override, got %d", report.EthicalScore)
	}
	foundAllow := false
	for _, detail := range report.EthicalDetails {
		if strings.Contains(detail, "explicitly allows path matching") {
			foundAllow = true
			break
		}
	}
	if !foundAllow {
		t.Errorf("expected ethical details to mention explicit allow, got: %v", report.EthicalDetails)
	}
}

func TestAnalyzer_InvalidURLs(t *testing.T) {
	az := NewAnalyzer(nil, nil)
	invalidURLs := []string{
		"",
		"://invalid-url",
		"example.com",
		"http://",
		"https://",
		"/just/a/path",
		"not-a-valid-url-at-all",
	}

	for _, u := range invalidURLs {
		t.Run("URL="+u, func(t *testing.T) {
			report, err := az.Analyze(context.Background(), u)
			if report == nil {
				t.Fatalf("expected non-nil Report for invalid URL %q, got nil", u)
			}
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
			if len(report.DifficultyDetails) == 0 {
				t.Errorf("expected non-empty DifficultyDetails for invalid URL %q", u)
			}
			if report.Recommendation == "" {
				t.Errorf("expected non-empty Recommendation for invalid URL %q", u)
			}
			if report.AnalyzedAt.IsZero() {
				t.Errorf("expected non-zero AnalyzedAt")
			}
			_ = err
		})
	}
}

func TestAnalyzer_BytesContainsFold(t *testing.T) {
	cases := []struct {
		body     string
		sub      string
		expected bool
	}{
		{"Copyright 2026. All Rights Reserved.", "all rights reserved", true},
		{"TERMS OF SERVICE apply here", "terms of service", true},
		{"Subject to Terms of Use.", "terms of use", true},
		{"No copyright notice at all.", "all rights reserved", false},
		{"", "all rights reserved", false},
		{"Short", "longer target phrase", false},
		{"all rights reserved", "all rights reserved", true},
		{"ALL RIGHTS RESERVED", "all rights reserved", true},
		{"mixEd Case TERMS OF use here", "terms of use", true},
	}

	for _, tc := range cases {
		got := bytesContainsFold([]byte(tc.body), tc.sub)
		if got != tc.expected {
			t.Errorf("bytesContainsFold(%q, %q) = %v, expected %v", tc.body, tc.sub, got, tc.expected)
		}
	}
}

func TestAnalyzer_InspectContentMemoryEfficiency(t *testing.T) {
	// Generate a 2MB HTML payload
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html><html><body>")
	sb.WriteString("<h1>Large Test Page</h1>")
	chunk := "<p>This is standard paragraph content without SPA markers.</p>\n"
	for sb.Len() < 2*1024*1024 {
		sb.WriteString(chunk)
	}
	sb.WriteString("<footer><p>All Rights Reserved. Terms of Service apply.</p></footer>")
	sb.WriteString("</body></html>")
	bodyBytes := []byte(sb.String())

	az := NewAnalyzer(nil, nil)
	u, _ := url.Parse("https://example.com/test")

	allocs := testing.AllocsPerRun(5, func() {
		rep := &Report{
			URL:               "https://example.com/test",
			EthicalScore:      100,
			DifficultyScore:   1,
			EthicalDetails:    make([]string, 0),
			DifficultyDetails: make([]string, 0),
		}
		az.inspectContent(u, bodyBytes, rep)
		if len(rep.EthicalDetails) == 0 {
			t.Errorf("expected ethical details from footer notice")
		}
	})

	t.Logf("Allocations per inspectContent run on 2MB body: %.0f", allocs)
}

func TestAnalyzer_DOMTraversalEarlyBreak(t *testing.T) {
	// Build an HTML document with 2,000 class elements and 1,000 links
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html><html><body>")
	// The first link is a honeypot
	sb.WriteString(`<a href="/trap" style="display: none">Honeypot Trap</a>`)
	for i := 0; i < 1000; i++ {
		sb.WriteString(fmt.Sprintf(`<a href="/link-%d">Normal Link %d</a>`, i, i))
	}
	// Add 2,000 elements with classes
	for i := 0; i < 2000; i++ {
		sb.WriteString(fmt.Sprintf(`<div class="item-%d normal-class">Content %d</div>`, i, i))
	}
	sb.WriteString("</body></html>")
	bodyBytes := []byte(sb.String())

	az := NewAnalyzer(nil, nil)
	u, _ := url.Parse("https://example.com/bigdom")
	rep := &Report{
		URL:               "https://example.com/bigdom",
		EthicalScore:      100,
		DifficultyScore:   1,
		EthicalDetails:    make([]string, 0),
		DifficultyDetails: make([]string, 0),
	}

	az.inspectContent(u, bodyBytes, rep)

	// Honeypot should be detected
	foundHoneypot := false
	for _, d := range rep.DifficultyDetails {
		if strings.Contains(d, "Hidden honeypot traps detected") {
			foundHoneypot = true
			break
		}
	}
	if !foundHoneypot {
		t.Errorf("expected honeypot detection in DifficultyDetails, got: %v", rep.DifficultyDetails)
	}
}

func TestAnalyzer_RobotsCrawlDelayEdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		crawlDelayLine    string
		expectDelayReport bool
		expectedDelayStr  string
		expectScoreBump   bool
	}{
		{
			name:              "Negative Crawl-Delay",
			crawlDelayLine:    "Crawl-delay: -10",
			expectDelayReport: false,
		},
		{
			name:              "NaN Crawl-Delay",
			crawlDelayLine:    "Crawl-delay: NaN",
			expectDelayReport: false,
		},
		{
			name:              "Inf Crawl-Delay",
			crawlDelayLine:    "Crawl-delay: +Inf",
			expectDelayReport: false,
		},
		{
			name:              "Absurdly Large Crawl-Delay",
			crawlDelayLine:    "Crawl-delay: 999999999",
			expectDelayReport: false,
		},
		{
			name:              "Valid Moderate Crawl-Delay",
			crawlDelayLine:    "Crawl-delay: 2.5",
			expectDelayReport: true,
			expectedDelayStr:  "2.5s",
			expectScoreBump:   false,
		},
		{
			name:              "Valid High Crawl-Delay (>5s)",
			crawlDelayLine:    "Crawl-delay: 10.0",
			expectDelayReport: true,
			expectedDelayStr:  "10.0s",
			expectScoreBump:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, "User-agent: *\n%s\n", tc.crawlDelayLine)
			})
			mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintln(w, "<html><body>OK</body></html>")
			})

			server := httptest.NewServer(mux)
			defer server.Close()

			az := NewAnalyzer(server.Client(), nil)
			report, err := az.Analyze(context.Background(), server.URL+"/")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			foundDelay := false
			for _, detail := range report.EthicalDetails {
				if strings.Contains(detail, "Crawl-delay:") {
					foundDelay = true
					if tc.expectedDelayStr != "" && !strings.Contains(detail, tc.expectedDelayStr) {
						t.Errorf("expected Crawl-delay detail to contain %q, got %q", tc.expectedDelayStr, detail)
					}
					break
				}
			}

			if foundDelay != tc.expectDelayReport {
				t.Errorf("expectDelayReport = %v, but foundDelay = %v (details: %v)", tc.expectDelayReport, foundDelay, report.EthicalDetails)
			}

			if tc.expectScoreBump && report.DifficultyScore < 2 {
				t.Errorf("expected DifficultyScore bump for high crawl delay, got %d", report.DifficultyScore)
			}
		})
	}
}

func TestAnalyzer_InvalidAndValidJSONLD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nAllow: /")
	})
	mux.HandleFunc("/mixed-jsonld", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head>
  <script type="application/ld+json">{"@context":"https://schema.org","@type":"Article","headline":"Valid"}</script>
  <script type="application/ld+json"></script>
  <script type="application/ld+json">    </script>
  <script type="application/ld+json">{"unclosed json: true, </script>
</head>
<body><p>Mixed JSON-LD test page with plenty of content to avoid SPA flag.</p></body>
</html>`)
	})

	mux.HandleFunc("/invalid-only-jsonld", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head>
  <script type="application/ld+json"></script>
  <script type="application/ld+json">{bad json}</script>
</head>
<body><p>Only invalid JSON-LD content on this page.</p></body>
</html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)

	t.Run("MixedJSONLD", func(t *testing.T) {
		report, err := az.Analyze(context.Background(), server.URL+"/mixed-jsonld")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		foundValidBlock := false
		for _, detail := range report.DifficultyDetails {
			if strings.Contains(detail, "Structured JSON-LD metadata available (1 block(s))") {
				foundValidBlock = true
				break
			}
		}
		if !foundValidBlock {
			t.Errorf("expected exactly 1 block of JSON-LD detected, got details: %v", report.DifficultyDetails)
		}
	})

	t.Run("InvalidOnlyJSONLD", func(t *testing.T) {
		report, err := az.Analyze(context.Background(), server.URL+"/invalid-only-jsonld")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, detail := range report.DifficultyDetails {
			if strings.Contains(detail, "Structured JSON-LD") {
				t.Errorf("did not expect any structured JSON-LD reported, got: %s", detail)
			}
		}
	})
}

func TestAnalyzer_RobotsCaseInsensitiveAndGroups(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `
# Group 1: OtherBot and wildcard
User-agent: OtherBot
User-agent: *
Disallow: /blocked-for-all/
Disallow: /*.secret$

# Group 2: ChameleonBot specific override
USER-AGENT: CHAMELEONBOT
Allow: /blocked-for-all/allowed-for-chameleon
Disallow: /chameleon-private/*
`)
	})
	mux.HandleFunc("/blocked-for-all/allowed-for-chameleon", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body>Allowed for Chameleon</body></html>")
	})
	mux.HandleFunc("/chameleon-private/data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body>Private data</body></html>")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)

	t.Run("ChameleonSpecificAllow", func(t *testing.T) {
		report, err := az.Analyze(context.Background(), server.URL+"/blocked-for-all/allowed-for-chameleon")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.EthicalScore < 90 {
			t.Errorf("expected high ethical score >= 90, got %d (details: %v)", report.EthicalScore, report.EthicalDetails)
		}
	})

	t.Run("ChameleonWildcardDisallow", func(t *testing.T) {
		report, err := az.Analyze(context.Background(), server.URL+"/chameleon-private/data")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.EthicalScore > 75 {
			t.Errorf("expected penalized ethical score <= 75 for disallowed path, got %d", report.EthicalScore)
		}
	})
}
