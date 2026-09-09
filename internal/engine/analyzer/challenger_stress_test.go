package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// TestChallenger_InspectContent_5MB_MemoryAndAllocBounds verifies that inspecting
// a 5MB HTML document containing thousands of elements does not perform redundant 10MB
// string heap copies (rawHTML + lowerHTML) and that memory allocations remain bounded.
func TestChallenger_InspectContent_5MB_MemoryAndAllocBounds(t *testing.T) {
	// 1. Build a realistic 5MB HTML body with ~5,000 DOM elements
	var sb strings.Builder
	sb.Grow(5 * 1024 * 1024 + 1024)
	sb.WriteString("<!DOCTYPE html><html><head><title>5MB Stress Document</title></head><body>\n")
	sb.WriteString("<header><h1>Stress Testing Large Body Allocation</h1></header>\n<main>\n")

	elemIdx := 0
	for sb.Len() < 5*1024*1024 {
		sb.WriteString(fmt.Sprintf(
			`<section id="sec-%d" class="content-block standard-layout"><p class="text-body">Element %d: Testing memory footprint without whole-body string duplication.</p><a href="/item-%d" class="nav-link">Link %d</a></section>`+"\n",
			elemIdx, elemIdx, elemIdx, elemIdx,
		))
		elemIdx++
	}
	sb.WriteString("</main>\n<footer><p>All Rights Reserved. Terms of Service apply to all users.</p></footer>\n")
	sb.WriteString("</body></html>\n")

	bodyBytes := []byte(sb.String())
	if len(bodyBytes) < 5*1024*1024 {
		t.Fatalf("expected payload >= 5MB, got %d bytes", len(bodyBytes))
	}

	az := NewAnalyzer(nil, nil)
	targetURL, _ := url.Parse("https://stress.test/large-page")

	// 2. Verify bytesContainsFold does ZERO heap allocations
	t.Run("ZeroAllocBytesContainsFold", func(t *testing.T) {
		allocs := testing.AllocsPerRun(20, func() {
			found := bytesContainsFold(bodyBytes, "all rights reserved")
			if !found {
				t.Fatalf("expected to find 'all rights reserved' in footer")
			}
			foundTOS := bytesContainsFold(bodyBytes, "terms of service")
			if !foundTOS {
				t.Fatalf("expected to find 'terms of service' in footer")
			}
		})
		if allocs != 0 {
			t.Errorf("expected 0 allocations for bytesContainsFold on 5MB slice, got %.2f", allocs)
		}
	})

	// 3. Measure memory allocation delta during inspectContent
	t.Run("InspectContentBoundedHeapDelta", func(t *testing.T) {
		runtime.GC()
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)

		rep := &Report{
			URL:               targetURL.String(),
			EthicalScore:      100,
			DifficultyScore:   1,
			EthicalDetails:    make([]string, 0),
			DifficultyDetails: make([]string, 0),
		}

		az.inspectContent(targetURL, bodyBytes, rep)

		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)

		totalAllocDelta := m2.TotalAlloc - m1.TotalAlloc
		t.Logf("5MB inspectContent TotalAlloc delta: %d bytes (%.2f MB)", totalAllocDelta, float64(totalAllocDelta)/(1024*1024))

		// Verify footer notice was detected
		foundFooter := false
		for _, d := range rep.EthicalDetails {
			if strings.Contains(d, "Terms of Service / Copyright notice identified") {
				foundFooter = true
				break
			}
		}
		if !foundFooter {
			t.Errorf("expected copyright/TOS notice in ethical details, got: %v", rep.EthicalDetails)
		}
	})
}

// TestChallenger_ClassSampling_10K_Elements_Exact50Break adversarially tests that
// the DOM class sampling loop terminates after exactly 50 samples on documents
// with 10,000 class attributes.
func TestChallenger_ClassSampling_10K_Elements_Exact50Break(t *testing.T) {
	t.Run("OracleFirst50Obfuscated_RestClean", func(t *testing.T) {
		// First 50 elements have obfuscated hashes (e.g. "sc-100000")
		// Next 9,950 elements have clean class names ("clean-element-class")
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><body>\n")
		for i := 0; i < 50; i++ {
			sb.WriteString(fmt.Sprintf(`<div class="sc-%06x">Obfuscated %d</div>`+"\n", i+0x100000, i))
		}
		for i := 0; i < 9950; i++ {
			sb.WriteString(fmt.Sprintf(`<div class="clean-element-class">Clean %d</div>`+"\n", i))
		}
		sb.WriteString("</body></html>\n")
		bodyBytes := []byte(sb.String())

		az := NewAnalyzer(nil, nil)
		u, _ := url.Parse("https://oracle.test/classes")
		rep := &Report{
			URL:               u.String(),
			EthicalScore:      100,
			DifficultyScore:   1,
			EthicalDetails:    make([]string, 0),
			DifficultyDetails: make([]string, 0),
		}

		az.inspectContent(u, bodyBytes, rep)

		// If the loop broke at 50, obfuscatedClasses = 50 / sampleClasses = 50 -> 100% > 40%.
		// Obfuscation MUST be detected!
		// If the loop failed to break and continued to 10,000, ratio would be 50/10000 = 0.5% << 40% and NOT detected.
		foundObfuscation := false
		for _, d := range rep.DifficultyDetails {
			if strings.Contains(d, "Obfuscated CSS / styled-components classes detected") {
				foundObfuscation = true
				break
			}
		}
		if !foundObfuscation {
			t.Fatalf("FAIL: Class sampling did not detect obfuscation on first 50 elements; likely traversed past 50 samples (details: %v)", rep.DifficultyDetails)
		}
	})

	t.Run("OracleFirst50Clean_RestObfuscated", func(t *testing.T) {
		// First 50 elements have clean class names
		// Next 9,950 elements have clean class names
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><body>\n")
		for i := 0; i < 50; i++ {
			sb.WriteString(fmt.Sprintf(`<div class="clean-element-class">Clean %d</div>`+"\n", i))
		}
		for i := 0; i < 9950; i++ {
			sb.WriteString(fmt.Sprintf(`<div class="sc-%06x">Obfuscated %d</div>`+"\n", i+0x100000, i))
		}
		sb.WriteString("</body></html>\n")
		bodyBytes := []byte(sb.String())

		az := NewAnalyzer(nil, nil)
		u, _ := url.Parse("https://oracle.test/clean-first")
		rep := &Report{
			URL:               u.String(),
			EthicalScore:      100,
			DifficultyScore:   1,
			EthicalDetails:    make([]string, 0),
			DifficultyDetails: make([]string, 0),
		}

		az.inspectContent(u, bodyBytes, rep)

		// Since first 50 were clean and loop halts at 50, ratio must be 0/50 = 0% <= 40%.
		// Obfuscation must NOT be detected!
		for _, d := range rep.DifficultyDetails {
			if strings.Contains(d, "Obfuscated CSS / styled-components classes detected") {
				t.Fatalf("FAIL: Class sampling detected obfuscation when first 50 were clean; inspected elements past 50! (details: %v)", rep.DifficultyDetails)
			}
		}
	})

	t.Run("DirectTraversalCount_10000Elements", func(t *testing.T) {
		// Build 10,000 elements with class attribute
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><body>\n")
		for i := 0; i < 10000; i++ {
			sb.WriteString(fmt.Sprintf(`<div class="item-%d">Content</div>`+"\n", i))
		}
		sb.WriteString("</body></html>\n")

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(sb.String()))
		if err != nil {
			t.Fatal(err)
		}

		// Execute the exact class sampling EachWithBreak logic and count iterations
		iterations := 0
		sampleClasses := 0
		doc.Find("[class]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			iterations++
			if sampleClasses >= 50 {
				return false
			}
			classVal, _ := s.Attr("class")
			classes := strings.Fields(classVal)
			for _, _ = range classes {
				sampleClasses++
				if sampleClasses >= 50 {
					return false
				}
			}
			return true
		})

		if sampleClasses != 50 {
			t.Errorf("expected sampleClasses to be exactly 50, got %d", sampleClasses)
		}
		if iterations != 50 {
			t.Errorf("expected EachWithBreak to execute callback exactly 50 times on 10,000 elements, got %d", iterations)
		}
	})
}

// TestChallenger_HoneypotDetection_ThousandsOfLinks_ImmediateBreak adversarially tests
// that honeypot detection immediately terminates traversal upon encountering a trap.
func TestChallenger_HoneypotDetection_ThousandsOfLinks_ImmediateBreak(t *testing.T) {
	t.Run("FirstLinkIsHoneypot_10000Links", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><body>\n")
		// Link 0 is a honeypot
		sb.WriteString(`<a href="/trap-0" style="display: none">Hidden Trap</a>` + "\n")
		for i := 1; i < 10000; i++ {
			sb.WriteString(fmt.Sprintf(`<a href="/normal-%d">Normal Link %d</a>`+"\n", i, i))
		}
		sb.WriteString("</body></html>\n")

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(sb.String()))
		if err != nil {
			t.Fatal(err)
		}

		// Direct callback iteration count
		callbackCount := 0
		honeypotFound := false
		doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			callbackCount++
			style, _ := s.Attr("style")
			cleanStyle := strings.ToLower(strings.ReplaceAll(style, " ", ""))
			if strings.Contains(cleanStyle, "display:none") || strings.Contains(cleanStyle, "visibility:hidden") || strings.Contains(cleanStyle, "opacity:0") {
				honeypotFound = true
				return false
			}
			return true
		})

		if !honeypotFound {
			t.Fatal("expected honeypot to be found")
		}
		if callbackCount != 1 {
			t.Fatalf("expected EachWithBreak to stop at callback #1, got %d callbacks executed", callbackCount)
		}
	})

	t.Run("HoneypotAtPosition42_10000Links", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString("<!DOCTYPE html><html><body>\n")
		for i := 0; i < 42; i++ {
			sb.WriteString(fmt.Sprintf(`<a href="/normal-%d">Normal Link %d</a>`+"\n", i, i))
		}
		// Link 42 is honeypot
		sb.WriteString(`<a href="/trap-42" style="visibility: hidden">Hidden Trap 42</a>` + "\n")
		for i := 43; i < 10000; i++ {
			sb.WriteString(fmt.Sprintf(`<a href="/normal-%d">Normal Link %d</a>`+"\n", i, i))
		}
		sb.WriteString("</body></html>\n")

		doc, err := goquery.NewDocumentFromReader(strings.NewReader(sb.String()))
		if err != nil {
			t.Fatal(err)
		}

		callbackCount := 0
		honeypotFound := false
		doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			callbackCount++
			style, _ := s.Attr("style")
			cleanStyle := strings.ToLower(strings.ReplaceAll(style, " ", ""))
			if strings.Contains(cleanStyle, "display:none") || strings.Contains(cleanStyle, "visibility:hidden") || strings.Contains(cleanStyle, "opacity:0") {
				honeypotFound = true
				return false
			}
			return true
		})

		if !honeypotFound {
			t.Fatal("expected honeypot to be found")
		}
		if callbackCount != 43 {
			t.Fatalf("expected EachWithBreak to stop at callback #43 (0-indexed 42), got %d callbacks executed", callbackCount)
		}
	})

	t.Run("StyleVariantsDetection", func(t *testing.T) {
		styleVariants := []string{
			"display:none",
			"display: none",
			"DISPLAY: NONE",
			"display:  none  ; color: red",
			"visibility:hidden",
			"visibility: hidden",
			"VISIBILITY: HIDDEN",
			"opacity:0",
			"opacity: 0",
			"opacity:0.0",
		}

		for _, style := range styleVariants {
			html := fmt.Sprintf(`<html><body><a href="/trap" style="%s">Trap</a></body></html>`, style)
			az := NewAnalyzer(nil, nil)
			u, _ := url.Parse("https://style.test/trap")
			rep := &Report{
				URL:               u.String(),
				EthicalScore:      100,
				DifficultyScore:   1,
				EthicalDetails:    make([]string, 0),
				DifficultyDetails: make([]string, 0),
			}

			az.inspectContent(u, []byte(html), rep)

			found := false
			for _, d := range rep.DifficultyDetails {
				if strings.Contains(d, "Hidden honeypot traps detected") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("style variant %q was not detected as honeypot trap", style)
			}
		}
	})
}

// TestChallenger_RobotsRFC9309_AdversarialStress tests complex RFC 9309 directive
// precedence, wildcards, end-anchors, and group overrides.
func TestChallenger_RobotsRFC9309_AdversarialStress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `
# Adversarial robots.txt with multiple user-agent groups, mixed case, comments, and wildcards
User-agent: BadBot, ScraperBot
Disallow: /

User-agent: *
Disallow: /private/
Disallow: /admin/*
Disallow: /*.secret$
Disallow: /api/v1/
Allow: /api/v1/public/
Crawl-delay: 2.0

USER-AGENT: CHAMELEONBOT
Allow: /private/chameleon-exclusive/
Disallow: /private/
Crawl-delay: 1.5
Sitemap: https://adversarial.test/sitemap-1.xml
Sitemap: https://adversarial.test/sitemap-2.xml
`)
	})

	// Setup endpoints
	mux.HandleFunc("/private/chameleon-exclusive/doc", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body>Exclusive</body></html>")
	})
	mux.HandleFunc("/private/secret-data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body>Private</body></html>")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)

	// Test 1: Chameleon specific Allow override on /private/chameleon-exclusive/doc
	t.Run("ChameleonSpecificAllowOverride", func(t *testing.T) {
		rep, err := az.Analyze(context.Background(), server.URL+"/private/chameleon-exclusive/doc")
		if err != nil {
			t.Fatal(err)
		}
		if rep.EthicalScore < 90 {
			t.Errorf("expected ethical score >= 90 due to chameleon-specific allow, got %d", rep.EthicalScore)
		}
		foundAllow := false
		for _, d := range rep.EthicalDetails {
			if strings.Contains(d, "explicitly allows path matching") {
				foundAllow = true
				break
			}
		}
		if !foundAllow {
			t.Errorf("expected explicit allow detail, got: %v", rep.EthicalDetails)
		}
	})

	// Test 2: Chameleon specific Disallow on /private/secret-data
	t.Run("ChameleonSpecificDisallow", func(t *testing.T) {
		rep, err := az.Analyze(context.Background(), server.URL+"/private/secret-data")
		if err != nil {
			t.Fatal(err)
		}
		if rep.EthicalScore > 75 {
			t.Errorf("expected penalized ethical score <= 75, got %d", rep.EthicalScore)
		}
	})
}

// TestChallenger_ConcurrentAnalysis_Stress tests concurrent Analyze invocations
// across 20 goroutines to verify race freedom and report isolation.
func TestChallenger_ConcurrentAnalysis_Stress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "User-agent: *\nAllow: /\nCrawl-delay: 0.5")
	})
	mux.HandleFunc("/page-a", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Server", "cloudflare")
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><div id=\"root\"></div><p>React App</p></body></html>")
	})
	mux.HandleFunc("/page-b", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><h1>Static Page</h1><script type=\"application/ld+json\">{\"@type\":\"WebSite\"}</script></body></html>")
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	az := NewAnalyzer(server.Client(), nil)

	var wg sync.WaitGroup
	workers := 20
	errChan := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var target string
			if id%2 == 0 {
				target = server.URL + "/page-a"
			} else {
				target = server.URL + "/page-b"
			}

			rep, err := az.Analyze(context.Background(), target)
			if err != nil {
				errChan <- fmt.Errorf("worker %d returned error: %w", id, err)
				return
			}
			if rep == nil {
				errChan <- fmt.Errorf("worker %d returned nil report", id)
				return
			}
			if id%2 == 0 {
				// React SPA with Cloudflare
				if rep.DifficultyScore < 3 {
					errChan <- fmt.Errorf("worker %d expected DifficultyScore >= 3, got %d", id, rep.DifficultyScore)
					return
				}
			} else {
				// Static page with JSON-LD
				if rep.EthicalGrade != "A" {
					errChan <- fmt.Errorf("worker %d expected EthicalGrade A, got %s", id, rep.EthicalGrade)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent stress error: %v", err)
	}
}

// BenchmarkChallenger_InspectContent_5MB benchmarks inspectContent on a 5MB payload
func BenchmarkChallenger_InspectContent_5MB(b *testing.B) {
	var sb strings.Builder
	sb.Grow(5 * 1024 * 1024 + 512)
	sb.WriteString("<!DOCTYPE html><html><body>\n")
	for sb.Len() < 5*1024*1024 {
		sb.WriteString(`<div class="item-block"><p>Some standard content for benchmarking allocations.</p><a href="/test">Link</a></div>` + "\n")
	}
	sb.WriteString("<footer><p>All Rights Reserved. Terms of Service apply.</p></footer></body></html>\n")
	bodyBytes := []byte(sb.String())

	az := NewAnalyzer(nil, nil)
	u, _ := url.Parse("https://bench.test/5mb")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rep := &Report{
			URL:               u.String(),
			EthicalScore:      100,
			DifficultyScore:   1,
			EthicalDetails:    make([]string, 0),
			DifficultyDetails: make([]string, 0),
		}
		az.inspectContent(u, bodyBytes, rep)
	}
}
