package analyzer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	classHashRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{5,10}$`)
	hexHashRegex   = regexp.MustCompile(`^[a-f0-9]{6,}$`)
)

// Report contains the results of the scrapability and ethical analysis.
type Report struct {
	URL               string
	EthicalGrade      string   // "A", "B", "C", "D", "F"
	EthicalScore      int      // 0 - 100
	EthicalDetails    []string // e.g. ["✅ robots.txt allows crawling", ...]
	DifficultyScore   int      // 1 - 10
	DifficultyLevel   string   // "Easy", "Moderate", "Hard", "Extreme"
	DifficultyDetails []string // e.g. ["🛡️ Cloudflare WAF detected", ...]
	RateLimitInfo     string   // Passive rate-limiting info
	Recommendation    string   // Summary recommendation
	Duration          time.Duration
	HeadlessTested    bool
	AnalyzedAt        time.Time
}

// HeadlessBrowser defines an interface for executing or inspecting pages using a headless browser engine.
type HeadlessBrowser interface {
	CheckPage(ctx context.Context, targetURL string) (*HeadlessResult, error)
}

// HeadlessResult holds observations made by a headless browser run.
type HeadlessResult struct {
	RenderedHTML      string
	DOMMutations      int
	NetworkCalls      int
	CanvasFingerprint bool
	ExecutedJS        bool
}

// Analyzer performs static and dynamic evaluations of target websites.
type Analyzer struct {
	client  *http.Client
	browser HeadlessBrowser
}

// NewAnalyzer creates a new Analyzer instance.
func NewAnalyzer(client *http.Client, browser HeadlessBrowser) *Analyzer {
	if client == nil {
		client = &http.Client{
			Timeout: 12 * time.Second,
		}
	}
	return &Analyzer{
		client:  client,
		browser: browser,
	}
}

// SetBrowser configures or updates the headless browser driver.
func (a *Analyzer) SetBrowser(b HeadlessBrowser) {
	a.browser = b
}

// Analyze performs passive ethical and technical scrapability checks on targetURL.
func (a *Analyzer) Analyze(ctx context.Context, targetURL string) (*Report, error) {
	start := time.Now()

	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		parseErr := err
		if parseErr == nil {
			parseErr = fmt.Errorf("missing scheme or host in %s", targetURL)
		}
		return &Report{
			URL:               targetURL,
			EthicalGrade:      "F",
			EthicalScore:      0,
			EthicalDetails:    []string{"❌ Target URL parsing failed"},
			DifficultyScore:   10,
			DifficultyLevel:   "Extreme",
			DifficultyDetails: []string{fmt.Sprintf("❌ Invalid URL: %v", parseErr)},
			RateLimitInfo:     "Passive: No rate limit headers detected",
			Recommendation:    fmt.Sprintf("Invalid target URL (%s). Please provide a valid HTTP/HTTPS URL.", targetURL),
			Duration:          time.Since(start),
			AnalyzedAt:        time.Now(),
		}, nil
	}

	report := &Report{
		URL:               targetURL,
		EthicalScore:      100,
		DifficultyScore:   1,
		EthicalDetails:    make([]string, 0),
		DifficultyDetails: make([]string, 0),
		RateLimitInfo:     "Passive: No rate limit headers detected",
		AnalyzedAt:        time.Now(),
	}

	// 1. Check robots.txt and Sitemaps
	a.checkRobotsAndSitemap(ctx, parsedURL, report)

	// 2. Fetch main page (passively inspect response headers, status, and raw body)
	targetResp, fetchErr := a.fetchTarget(ctx, targetURL)
	if fetchErr != nil {
		report.DifficultyScore = 10
		report.DifficultyLevel = "Extreme"
		report.DifficultyDetails = append(report.DifficultyDetails, fmt.Sprintf("❌ Connection failed: %v", fetchErr))
		report.EthicalGrade = "F"
		report.Recommendation = "Unable to reach server. Check URL or network configuration."
		report.Duration = time.Since(start)
		return report, nil
	}

	// 3. Inspect HTTP Headers for WAF, Bot Protections, and Rate Limits
	a.inspectHeaders(targetResp.statusCode, targetResp.header, report)

	// 4. Inspect Content (DOM, SPA indicators, Obfuscation, Honeypots, Copyright/TOS)
	if len(targetResp.bodyBytes) > 0 {
		a.inspectContent(parsedURL, targetResp.bodyBytes, report)
	}

	// 5. Optional Headless Browser verification if configured
	if a.browser != nil {
		hRes, hErr := a.browser.CheckPage(ctx, targetURL)
		if hErr == nil && hRes != nil {
			report.HeadlessTested = true
			if hRes.DOMMutations > 0 {
				report.DifficultyDetails = append(report.DifficultyDetails, fmt.Sprintf("⚡ Headless Browser verified: %d dynamic DOM mutations after load", hRes.DOMMutations))
			}
		}
	}

	// 6. Compute Final Grades and Recommendations
	a.computeGrades(report)
	report.Duration = time.Since(start)

	return report, nil
}

type targetResponse struct {
	statusCode int
	header     http.Header
	bodyBytes  []byte
}

func (a *Analyzer) fetchTarget(ctx context.Context, targetURL string) (*targetResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB max
	if err != nil {
		return nil, err
	}

	return &targetResponse{
		statusCode: resp.StatusCode,
		header:     resp.Header,
		bodyBytes:  bodyBytes,
	}, nil
}

// matchRobotsPath matches targetPath against a robots.txt rule pattern supporting RFC 9309 wildcards (*) and end anchors ($).
func matchRobotsPath(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, "$") {
		return strings.HasPrefix(path, pattern)
	}

	var b strings.Builder
	b.WriteString("^")
	hasEndAnchor := strings.HasSuffix(pattern, "$")
	patToConvert := pattern
	if hasEndAnchor {
		patToConvert = pattern[:len(pattern)-1]
	}
	parts := strings.Split(patToConvert, "*")
	for i, p := range parts {
		if i > 0 {
			b.WriteString(".*")
		}
		b.WriteString(regexp.QuoteMeta(p))
	}
	if hasEndAnchor {
		b.WriteString("$")
	}
	re, err := regexp.Compile(b.String())
	if err != nil {
		return strings.HasPrefix(path, pattern)
	}
	return re.MatchString(path)
}

func (a *Analyzer) checkRobotsAndSitemap(ctx context.Context, u *url.URL, report *Report) {
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "ChameleonScraper/1.0")

	resp, err := a.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		report.EthicalDetails = append(report.EthicalDetails, "ℹ️ No robots.txt found (crawling unrestricted by default)")
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return
	}

	type ruleGroup struct {
		matchedDisallow string
		matchedAllow    string
		crawlDelay      float64
		hasCrawlDelay   bool
		hasDirectives   bool
	}

	var specificRules ruleGroup
	var specificGroupActive bool
	var hasSpecificGroup bool

	var globalRules ruleGroup
	var globalGroupActive bool
	var hasGlobalGroup bool

	inRules := false
	hasSitemap := false

	targetPath := u.Path
	if targetPath == "" {
		targetPath = "/"
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		if key == "sitemap" {
			hasSitemap = true
			continue
		}

		if key == "user-agent" {
			if inRules {
				specificGroupActive = false
				globalGroupActive = false
				inRules = false
			}
			agent := strings.ToLower(val)
			tokens := strings.FieldsFunc(agent, func(r rune) bool {
				return r == ',' || r == ' ' || r == '\t'
			})
			for _, tok := range tokens {
				tok = strings.TrimSpace(tok)
				if tok == "*" {
					globalGroupActive = true
					hasGlobalGroup = true
				}
				if strings.Contains(tok, "chameleon") {
					specificGroupActive = true
					hasSpecificGroup = true
				}
			}
			continue
		}

		if specificGroupActive || globalGroupActive {
			inRules = true
			applyDirective := func(rg *ruleGroup) {
				rg.hasDirectives = true
				switch key {
				case "disallow":
					if val != "" && matchRobotsPath(val, targetPath) {
						if len(val) >= len(rg.matchedDisallow) {
							rg.matchedDisallow = val
						}
					}
				case "allow":
					if val != "" && matchRobotsPath(val, targetPath) {
						if len(val) >= len(rg.matchedAllow) {
							rg.matchedAllow = val
						}
					}
				case "crawl-delay":
					if d, err := strconv.ParseFloat(val, 64); err == nil {
						if !math.IsNaN(d) && !math.IsInf(d, 0) && d >= 0 && d <= 86400 {
							rg.crawlDelay = d
							rg.hasCrawlDelay = true
						}
					}
				}
			}

			if specificGroupActive {
				applyDirective(&specificRules)
			}
			if globalGroupActive {
				applyDirective(&globalRules)
			}
		}
	}

	var matchedDisallow string
	var matchedAllow string
	var crawlDelay float64

	if hasSpecificGroup && specificRules.hasDirectives {
		matchedDisallow = specificRules.matchedDisallow
		matchedAllow = specificRules.matchedAllow
		if specificRules.hasCrawlDelay {
			crawlDelay = specificRules.crawlDelay
		}
	} else if hasGlobalGroup {
		matchedDisallow = globalRules.matchedDisallow
		matchedAllow = globalRules.matchedAllow
		if globalRules.hasCrawlDelay {
			crawlDelay = globalRules.crawlDelay
		}
	}

	isDisallowed := false
	if matchedDisallow != "" {
		if matchedAllow == "" || len(matchedDisallow) > len(matchedAllow) {
			isDisallowed = true
		}
	}

	if isDisallowed {
		if matchedDisallow == "/" {
			report.EthicalScore -= 60
			report.EthicalDetails = append(report.EthicalDetails, "❌ robots.txt strictly disallows all crawling ('Disallow: /')")
			report.DifficultyScore++
		} else {
			report.EthicalScore -= 35
			report.EthicalDetails = append(report.EthicalDetails, fmt.Sprintf("⚠️ robots.txt disallows path matching '%s'", matchedDisallow))
		}
	} else {
		if matchedAllow != "" {
			report.EthicalDetails = append(report.EthicalDetails, fmt.Sprintf("✅ robots.txt explicitly allows path matching '%s'", matchedAllow))
		} else {
			report.EthicalDetails = append(report.EthicalDetails, "✅ robots.txt allows crawling target path")
		}
	}

	if crawlDelay > 0 {
		report.EthicalDetails = append(report.EthicalDetails, fmt.Sprintf("⏱️ robots.txt requests Crawl-delay: %.1fs", crawlDelay))
		if crawlDelay > 5 {
			report.DifficultyScore++
		}
	}

	if hasSitemap {
		report.EthicalDetails = append(report.EthicalDetails, "🗺️ Sitemap declared in robots.txt (structured discovery available)")
	}
}

func (a *Analyzer) inspectHeaders(statusCode int, header http.Header, report *Report) {
	// Status code check
	switch statusCode {
	case http.StatusForbidden:
		report.DifficultyScore += 4
		report.DifficultyDetails = append(report.DifficultyDetails, "🚫 HTTP 403 Forbidden received (Immediate Anti-Bot/WAF Block)")
	case http.StatusTooManyRequests:
		report.DifficultyScore += 4
		report.DifficultyDetails = append(report.DifficultyDetails, "⚠️ HTTP 429 Too Many Requests received (Aggressive Rate Limiter)")
	case http.StatusServiceUnavailable:
		report.DifficultyScore += 3
		report.DifficultyDetails = append(report.DifficultyDetails, "⚠️ HTTP 503 Service Unavailable (Possible Cloudflare Challenge/DDoS Gate)")
	}

	serverHeader := strings.ToLower(header.Get("Server"))
	cfRay := header.Get("cf-ray")
	cfMitigated := header.Get("cf-mitigated")

	if strings.Contains(serverHeader, "cloudflare") || cfRay != "" || cfMitigated != "" {
		report.DifficultyScore += 3
		report.DifficultyDetails = append(report.DifficultyDetails, "🛡️ Cloudflare WAF / CDN detected (Challenge/Turnstile protection likely)")
	}

	if strings.Contains(serverHeader, "akamaighost") || header.Get("X-Akamai-Transformed") != "" {
		report.DifficultyScore += 3
		report.DifficultyDetails = append(report.DifficultyDetails, "🛡️ Akamai Edge / Bot Manager detected")
	}

	if header.Get("X-Amz-Cf-Id") != "" || strings.Contains(serverHeader, "cloudfront") {
		report.DifficultyScore++
		report.DifficultyDetails = append(report.DifficultyDetails, "☁️ AWS CloudFront CDN detected")
	}

	if header.Get("X-DataDome") != "" || header.Get("X-DataDome-CID") != "" {
		report.DifficultyScore += 4
		report.DifficultyDetails = append(report.DifficultyDetails, "🛡️ DataDome Bot Protection detected")
	}

	if header.Get("X-Icdn") != "" || strings.Contains(serverHeader, "incapsula") {
		report.DifficultyScore += 3
		report.DifficultyDetails = append(report.DifficultyDetails, "🛡️ Imperva / Incapsula WAF detected")
	}

	// Passive Rate-Limit Header checks
	rateLimitHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"RateLimit-Limit",
		"RateLimit-Remaining",
		"Retry-After",
	}

	foundRateLimits := make([]string, 0)
	for _, h := range rateLimitHeaders {
		if val := header.Get(h); val != "" {
			foundRateLimits = append(foundRateLimits, fmt.Sprintf("%s: %s", h, val))
		}
	}

	if len(foundRateLimits) > 0 {
		report.RateLimitInfo = fmt.Sprintf("Passive: %s", strings.Join(foundRateLimits, ", "))
		report.DifficultyDetails = append(report.DifficultyDetails, fmt.Sprintf("⏱️ Rate limit headers present (%s)", strings.Join(foundRateLimits, ", ")))
	}
}

// bytesContainsFold reports whether sub (lowercase ASCII) is present in b case-insensitively.
func bytesContainsFold(b []byte, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(b) < len(sub) {
		return false
	}
	subLen := len(sub)
	firstLower := sub[0]
	firstUpper := firstLower
	if firstLower >= 'a' && firstLower <= 'z' {
		firstUpper = firstLower - ('a' - 'A')
	}
	maxIdx := len(b) - subLen
	for i := 0; i <= maxIdx; i++ {
		c := b[i]
		if c == firstLower || c == firstUpper {
			match := true
			for j := 1; j < subLen; j++ {
				bj := b[i+j]
				sj := sub[j]
				if sj >= 'a' && sj <= 'z' {
					if bj != sj && bj != sj-('a'-'A') {
						match = false
						break
					}
				} else if bj != sj {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

func (a *Analyzer) inspectContent(_ *url.URL, bodyBytes []byte, report *Report) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}

	// 1. SPA / JavaScript Rendering check
	isSPA := false
	spaFramework := ""

	switch {
	case bytes.Contains(bodyBytes, []byte(`id="root"`)) || bytes.Contains(bodyBytes, []byte(`id="__next"`)):
		isSPA = true
		spaFramework = "React / Next.js"
	case bytes.Contains(bodyBytes, []byte(`id="app"`)) || bytes.Contains(bodyBytes, []byte(`id="__nuxt"`)):
		isSPA = true
		spaFramework = "Vue / Nuxt"
	case bytes.Contains(bodyBytes, []byte("ng-version")) || bytes.Contains(bodyBytes, []byte("app-root")):
		isSPA = true
		spaFramework = "Angular"
	}

	// Measure body text vs script tags
	bodyText := strings.TrimSpace(doc.Find("body").Text())
	scriptCount := doc.Find("script").Length()

	if isSPA || (len(bodyText) < 150 && scriptCount > 3 && len(bodyBytes) > 2000) {
		report.DifficultyScore += 3
		name := "Single Page Application (SPA)"
		if spaFramework != "" {
			name = fmt.Sprintf("%s SPA", spaFramework)
		}
		report.DifficultyDetails = append(report.DifficultyDetails, fmt.Sprintf("⚡ %s detected (dynamic client-side rendering required)", name))
	} else {
		report.DifficultyDetails = append(report.DifficultyDetails, "📄 Static / SSR HTML detected (direct HTTP scraping feasible)")
	}

	// 2. Class Obfuscation / CSS Hashes
	obfuscatedClasses := 0
	sampleClasses := 0

	doc.Find("[class]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if sampleClasses >= 50 {
			return false
		}
		classVal, _ := s.Attr("class")
		classes := strings.Fields(classVal)
		for _, c := range classes {
			sampleClasses++
			if hexHashRegex.MatchString(c) || strings.HasPrefix(c, "css-") || strings.HasPrefix(c, "sc-") || (len(c) >= 5 && len(c) <= 10 && classHashRegex.MatchString(c)) {
				obfuscatedClasses++
			}
			if sampleClasses >= 50 {
				return false
			}
		}
		return true
	})

	if sampleClasses > 10 && float64(obfuscatedClasses)/float64(sampleClasses) > 0.4 {
		report.DifficultyScore += 2
		report.DifficultyDetails = append(report.DifficultyDetails, "🔍 Obfuscated CSS / styled-components classes detected (fragile selectors)")
	}

	// 3. Honeypot Links detection
	honeypotFound := false
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		style, _ := s.Attr("style")
		cleanStyle := strings.ToLower(strings.ReplaceAll(style, " ", ""))
		if strings.Contains(cleanStyle, "display:none") || strings.Contains(cleanStyle, "visibility:hidden") || strings.Contains(cleanStyle, "opacity:0") {
			honeypotFound = true
			return false
		}
		return true
	})

	if honeypotFound {
		report.DifficultyScore += 2
		report.DifficultyDetails = append(report.DifficultyDetails, "🍯 Hidden honeypot traps detected (crawler trap risk)")
	}

	// 4. Meta Robots & Ethical hints
	doc.Find("meta[name='robots']").Each(func(_ int, s *goquery.Selection) {
		content, _ := s.Attr("content")
		content = strings.ToLower(content)
		if strings.Contains(content, "noindex") || strings.Contains(content, "nofollow") || strings.Contains(content, "noarchive") {
			report.EthicalScore -= 20
			report.EthicalDetails = append(report.EthicalDetails, fmt.Sprintf("⚠️ Meta robots tag prohibits indexing/archiving: '%s'", content))
		}
	})

	// 5. Copyright / Terms of Service indicators
	if bytesContainsFold(bodyBytes, "all rights reserved") || bytesContainsFold(bodyBytes, "terms of service") || bytesContainsFold(bodyBytes, "terms of use") {
		report.EthicalDetails = append(report.EthicalDetails, "📜 Terms of Service / Copyright notice identified in page footer")
	}

	// 6. JSON-LD / Structured Data (Makes scraping much easier!)
	jsonLDCount := 0
	doc.Find("script[type='application/ld+json']").Each(func(_ int, s *goquery.Selection) {
		content := strings.TrimSpace(s.Text())
		if content != "" && json.Valid([]byte(content)) {
			jsonLDCount++
		}
	})
	if jsonLDCount > 0 {
		report.DifficultyDetails = append(report.DifficultyDetails, fmt.Sprintf("💎 Structured JSON-LD metadata available (%d block(s))", jsonLDCount))
		if report.DifficultyScore > 1 {
			report.DifficultyScore--
		}
	}
}

func (a *Analyzer) computeGrades(report *Report) {
	// Clamp difficulty score between 1 and 10
	if report.DifficultyScore < 1 {
		report.DifficultyScore = 1
	}
	if report.DifficultyScore > 10 {
		report.DifficultyScore = 10
	}

	switch {
	case report.DifficultyScore <= 2:
		report.DifficultyLevel = "Easy"
	case report.DifficultyScore <= 5:
		report.DifficultyLevel = "Moderate"
	case report.DifficultyScore <= 7:
		report.DifficultyLevel = "Hard"
	default:
		report.DifficultyLevel = "Extreme"
	}

	// Clamp ethical score between 0 and 100
	if report.EthicalScore < 0 {
		report.EthicalScore = 0
	}
	if report.EthicalScore > 100 {
		report.EthicalScore = 100
	}

	switch {
	case report.EthicalScore >= 90:
		report.EthicalGrade = "A"
	case report.EthicalScore >= 75:
		report.EthicalGrade = "B"
	case report.EthicalScore >= 60:
		report.EthicalGrade = "C"
	case report.EthicalScore >= 40:
		report.EthicalGrade = "D"
	default:
		report.EthicalGrade = "F"
	}

	// Formulate actionable recommendation
	switch {
	case report.DifficultyScore >= 8:
		report.Recommendation = "High-defense target. Use Headless Browser (Chromedp/Playwright) with stealth plugins, randomized intervals, and IP proxies."
	case report.DifficultyScore >= 5:
		report.Recommendation = "Dynamic or protected target. Recommended to use Headless Browser or inspect network requests for internal JSON APIs."
	case report.EthicalGrade == "F" || report.EthicalGrade == "D":
		report.Recommendation = "Technically accessible, but site policies (robots.txt/meta) explicitly restrict scraping. Proceed respectfully with low request rates."
	default:
		report.Recommendation = "Ideal target! Standard HTTP client scraping with Goquery/Colly is fully sufficient and polite."
	}
}
