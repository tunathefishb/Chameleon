package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// TestAdversarialSSRFAndDomainScopingUnit tests isAllowedCrawlTarget and isDisallowedHostOrIP
// with comprehensive SSRF vectors, external domains, and parser bypass attempts.
func TestAdversarialSSRFAndDomainScopingUnit(t *testing.T) {
	parentPublic, err := url.Parse("https://crawler-target.com/index.html")
	if err != nil {
		t.Fatalf("failed to parse parent url: %v", err)
	}

	tests := []struct {
		name      string
		parent    *url.URL
		rawTarget string
		wantAllow bool
	}{
		// 1. Private IPv4 SSRF vectors
		{"Private IP 10.0.0.1", parentPublic, "http://10.0.0.1/admin", false},
		{"Private IP 192.168.1.1", parentPublic, "http://192.168.1.1/setup", false},
		{"Private IP 172.16.0.1", parentPublic, "http://172.16.0.1/internal", false},
		{"Private IP 172.31.255.255", parentPublic, "http://172.31.255.255/api", false},
		{"Loopback IPv4 127.0.0.1", parentPublic, "http://127.0.0.1:8080/secret", false},
		{"Loopback IPv4 127.0.0.2", parentPublic, "http://127.0.0.2/secret", false},
		{"Unspecified IPv4 0.0.0.0", parentPublic, "http://0.0.0.0/", false},

		// 2. Cloud metadata vectors
		{"AWS/GCP/Azure Metadata IP", parentPublic, "http://169.254.169.254/latest/meta-data/", false},
		{"Google Cloud Metadata Host", parentPublic, "http://metadata.google.internal/computeMetadata/v1/", false},
		{"Metadata with Port", parentPublic, "http://169.254.169.254:80/latest/meta-data/", false},
		{"Localhost name", parentPublic, "http://localhost:3000/env", false},
		{"Subdomain of localhost", parentPublic, "http://api.localhost/debug", false},

		// 3. IPv6 SSRF vectors
		{"IPv6 Loopback ::1", parentPublic, "http://[::1]:8080/status", false},
		{"IPv6 Unique Local / Private fc00::1", parentPublic, "http://[fc00::1]/admin", false},
		{"IPv6 Link Local fe80::1", parentPublic, "http://[fe80::1]/info", false},

		// 4. External domain scoping
		{"External Domain attacker.com", parentPublic, "https://attacker.com/steal", false},
		{"External Domain other.org", parentPublic, "http://other.org/", false},
		{"Subdomain not same host", parentPublic, "https://sub.crawler-target.com/login", false},
		{"Domain suffix spoof", parentPublic, "https://crawler-target.com.evil.com/phish", false},
		{"Domain prefix spoof", parentPublic, "https://not-crawler-target.com/path", false},

		// 5. URL parser trick vectors
		{"Protocol relative to private IP", parentPublic, "//10.0.0.1/test", false},
		{"Userinfo spoofing target domain", parentPublic, "http://crawler-target.com@10.0.0.1/bypass", false},
		{"Userinfo spoofing metadata", parentPublic, "http://crawler-target.com@169.254.169.254/bypass", false},
		{"Hash fragment host spoof", parentPublic, "http://10.0.0.1#crawler-target.com", false},

		// 6. Legitimate same-domain crawl targets (must ALLOW)
		{"Same domain relative path", parentPublic, "https://crawler-target.com/about", true},
		{"Same domain nested path", parentPublic, "https://crawler-target.com/blog/2026/09/post.html", true},
		{"Same domain with port", parentPublic, "https://crawler-target.com:443/docs", true},
		{"Same domain mixed case", parentPublic, "https://CRAWLER-TARGET.COM/Contact", true},
		{"Same domain query params", parentPublic, "https://crawler-target.com/search?q=chameleon", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			targetU, err := tc.parent.Parse(tc.rawTarget)
			if err != nil {
				t.Fatalf("failed to parse target %s: %v", tc.rawTarget, err)
			}
			got := isAllowedCrawlTarget(tc.parent, targetU)
			if got != tc.wantAllow {
				t.Errorf("isAllowedCrawlTarget(%s, %s) = %v; want %v", tc.parent.String(), targetU.String(), got, tc.wantAllow)
			}
		})
	}
}

// TestAdversarialRecursiveLinkExtractionScopingEndToEnd crawls a live mock server
// whose HTML is heavily populated with SSRF and external links, verifying that
// child crawl jobs strictly respect domain boundaries and reject all SSRF vectors.
func TestAdversarialRecursiveLinkExtractionScopingEndToEnd(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body>
				<h1>Root Page</h1>
				<!-- Valid Internal Links -->
				<a href="/allowed-child-1">Allowed Child 1</a>
				<a href="/allowed-child-2">Allowed Child 2</a>

				<!-- Private IP SSRF vectors -->
				<a href="http://10.0.0.1/admin">SSRF 10.0.0.1</a>
				<a href="http://192.168.1.1/setup">SSRF 192.168.1.1</a>
				<a href="http://172.16.0.1/vault">SSRF 172.16.0.1</a>
				<a href="http://127.0.0.2/secret">SSRF 127.0.0.2</a>

				<!-- Cloud Metadata vectors -->
				<a href="http://169.254.169.254/latest/meta-data/">Cloud Metadata AWS/GCP</a>
				<a href="http://metadata.google.internal/computeMetadata/v1/">GCP Metadata</a>

				<!-- External Domain vectors -->
				<a href="https://external-target.com/data">External Target</a>
				<a href="https://evil-attacker.org/exfil">Attacker Link</a>
				<a href="http://127.0.0.1@10.0.0.1/userinfo-bypass">Userinfo Bypass</a>
				<a href="//10.0.0.1/proto-relative">Proto Relative</a>
			</body></html>`)
		case "/allowed-child-1":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body><a href="/allowed-grandchild">Grandchild</a></body></html>`)
		case "/allowed-child-2":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body><p>Leaf node</p></body></html>`)
		case "/allowed-grandchild":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body><p>Grandchild node</p></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	eng := NewEngine()
	eng.Start(2)
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	settings := Settings{
		Depth:  2,
		Images: false,
		Speed:  SpeedFast,
	}

	eng.AddJob(ts.URL+"/", settings)

	discovered := make(map[string]bool)
	results := make(map[string]bool)
	var mu sync.Mutex

	timeout := time.After(4 * time.Second)
	doneChan := make(chan struct{})

	go func() {
		for {
			select {
			case <-timeout:
				close(doneChan)
				return
			case u, ok := <-eng.Discovered:
				if !ok {
					return
				}
				mu.Lock()
				discovered[u] = true
				mu.Unlock()
			case res, ok := <-eng.Results:
				if !ok {
					return
				}
				mu.Lock()
				results[res.URL] = true
				mu.Unlock()
			}
		}
	}()

	<-doneChan

	mu.Lock()
	defer mu.Unlock()

	// 1. Verify permitted child links were discovered
	expectedChildren := []string{
		ts.URL + "/allowed-child-1",
		ts.URL + "/allowed-child-2",
	}
	for _, expected := range expectedChildren {
		if !discovered[expected] {
			t.Errorf("expected legitimate internal URL %s to be discovered, discovered: %v", expected, discovered)
		}
	}

	// 2. Adversarial SSRF and domain breakout checks:
	// None of the prohibited URLs must ever appear in discovered or results.
	forbiddenPrefixes := []string{
		"http://10.0.0.1",
		"http://192.168.1.1",
		"http://172.16.0.1",
		"http://127.0.0.2",
		"http://169.254.169.254",
		"http://metadata.google.internal",
		"https://external-target.com",
		"https://evil-attacker.org",
	}

	for discURL := range discovered {
		for _, prefix := range forbiddenPrefixes {
			if strings.HasPrefix(discURL, prefix) {
				t.Fatalf("CRITICAL SSRF / DOMAIN BREAKOUT: prohibited URL %q was discovered and queued!", discURL)
			}
		}
	}

	for resURL := range results {
		for _, prefix := range forbiddenPrefixes {
			if strings.HasPrefix(resURL, prefix) {
				t.Fatalf("CRITICAL SSRF / DOMAIN BREAKOUT: prohibited URL %q was fetched!", resURL)
			}
		}
	}
}

// TestAdversarialPublicDomainImageSSRFScoping tests image scraping when crawling a simulated public domain.
// It verifies that image tags pointing to private IPs or cloud metadata are rejected.
func TestAdversarialPublicDomainImageSSRFScoping(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body>
				<img src="http://169.254.169.254/leak.png" />
				<img src="http://10.0.0.1/token.png" />
				<img src="http://192.168.1.1/router.png" />
				<img src="http://127.0.0.1:8080/internal.png" />
				<img src="http://public-target.com/images/safe.png" />
			</body></html>`)
		case "/images/safe.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("safe-image-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	eng := NewEngine()
	// Route requests for "public-target.com" to our local test server
	serverAddr := ts.Listener.Addr().String()
	eng.client.Transport = &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, serverAddr)
		},
	}

	eng.Start(2)
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	settings := Settings{
		Depth:  1,
		Images: true,
		Speed:  SpeedFast,
	}

	eng.AddJob("http://public-target.com/", settings)

	discovered := make(map[string]bool)
	var mu sync.Mutex

	timeout := time.After(3 * time.Second)
	doneChan := make(chan struct{})

	go func() {
		for {
			select {
			case <-timeout:
				close(doneChan)
				return
			case u, ok := <-eng.Discovered:
				if !ok {
					return
				}
				mu.Lock()
				discovered[u] = true
				mu.Unlock()
			case <-eng.Results:
			case <-eng.Files:
			}
		}
	}()

	<-doneChan

	mu.Lock()
	defer mu.Unlock()

	// Prohibited image SSRF URLs
	forbidden := []string{
		"http://169.254.169.254/leak.png",
		"http://10.0.0.1/token.png",
		"http://192.168.1.1/router.png",
		"http://127.0.0.1:8080/internal.png",
	}

	for _, f := range forbidden {
		if discovered[f] {
			t.Errorf("IMAGE SSRF VULNERABILITY: prohibited image %q was discovered and queued from public domain!", f)
		}
	}

	if !discovered["http://public-target.com/images/safe.png"] {
		t.Errorf("expected legitimate image http://public-target.com/images/safe.png to be discovered, got %v", discovered)
	}
}

// TestAdversarialLimiterConcurrencyStress concurrently hammers SetLimit with mixed speeds
// across multiple goroutines while running active crawling jobs, verifying that limiter
// synchronization is thread-safe and free from data races or deadlocks under -race.
func TestAdversarialLimiterConcurrencyStress(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	eng := NewEngine()
	eng.Start(8) // 8 concurrent workers
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	const numJobs = 60
	const numMutators = 12

	var wg sync.WaitGroup

	// Concurrently enqueue jobs alternating between SpeedFast and SpeedSafe
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numJobs; i++ {
			speed := SpeedFast
			if i%2 == 0 {
				speed = SpeedSafe
			}
			jobURL := fmt.Sprintf("%s/job-%d", ts.URL, i)
			eng.AddJob(jobURL, Settings{
				Depth:  1,
				Images: false,
				Speed:  speed,
			})
			if i%10 == 0 {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	// Concurrently mutate the limiter rate across multiple goroutines
	for m := 0; m < numMutators; m++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				eng.limiterMu.Lock()
				if (id+i)%2 == 0 {
					eng.limiter.SetLimit(rate.Every(10 * time.Millisecond))
				} else {
					eng.limiter.SetLimit(rate.Every(50 * time.Millisecond))
				}
				eng.limiterMu.Unlock()
				time.Sleep(2 * time.Millisecond)
			}
		}(m)
	}

	// Concurrently consume from engine channels to prevent backpressure stalls
	resultsCount := 0
	stopConsuming := make(chan struct{})
	var resultsMu sync.Mutex

	go func() {
		for {
			select {
			case <-stopConsuming:
				return
			case <-eng.Results:
				resultsMu.Lock()
				resultsCount++
				resultsMu.Unlock()
			case <-eng.Files:
			case <-eng.Discovered:
			}
		}
	}()

	wg.Wait()

	// Wait up to 3 seconds for active jobs to complete
	deadline := time.After(3 * time.Second)
waitLoop:
	for {
		resultsMu.Lock()
		count := resultsCount
		resultsMu.Unlock()
		if count >= numJobs {
			break waitLoop
		}
		select {
		case <-deadline:
			break waitLoop
		case <-time.After(50 * time.Millisecond):
		}
	}

	close(stopConsuming)
}
