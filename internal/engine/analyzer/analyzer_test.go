package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
