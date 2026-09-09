package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestChallenger_ChannelBackpressureAndRapidShutdown tests that flooding
// Results, Files, Discovered, and Jobs beyond channel capacities (1000) followed
// by rapid Stop() completes cleanly without hangs, deadlocks, or closed-channel panics.
func TestChallenger_ChannelBackpressureAndRapidShutdown(t *testing.T) {
	for iteration := 0; iteration < 5; iteration++ {
		eng := NewEngine()
		eng.Start(8)

		// 1. Flood Results channel buffer (cap 1000) with 3000 items
		// to force 2000 goroutines into e.sendWg
		for i := 0; i < 3000; i++ {
			eng.sendResult(Result{
				Name:       fmt.Sprintf("backpressure-result-%d", i),
				Status:     "200",
				StatusCode: 200,
				URL:        fmt.Sprintf("http://flood.test/item-%d", i),
				Timestamp:  time.Now(),
			})
		}

		// 2. Flood Files channel buffer (cap 1000) with 2500 items
		for i := 0; i < 2500; i++ {
			eng.sendFile(fmt.Sprintf("output/flood.test/file-%d.html", i))
		}

		// 3. Flood Discovered channel buffer (cap 1000) with 2500 items
		for i := 0; i < 2500; i++ {
			eng.sendDiscovered(fmt.Sprintf("http://flood.test/discovered-%d", i))
		}

		// 4. Flood Jobs channel buffer (cap 1000) with 2500 jobs
		for i := 0; i < 2500; i++ {
			eng.dispatchJob(Job{
				URL:      fmt.Sprintf("http://flood.test/job-%d", i),
				Settings: Settings{Depth: 1, Speed: SpeedFast},
			})
		}

		// 5. Call Stop() and verify it completes without deadlocking
		stopped := make(chan struct{})
		go func() {
			eng.Stop()
			close(stopped)
		}()

		select {
		case <-stopped:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatalf("Iteration %d: eng.Stop() timed out under severe channel backpressure", iteration)
		}

		// 6. Adversarial post-shutdown calls: must be safe, no panic
		eng.Stop() // Idempotent Stop
		eng.sendResult(Result{Name: "post-stop-res"})
		eng.sendFile("post-stop-file")
		eng.sendDiscovered("http://post-stop")
		eng.dispatchJob(Job{URL: "http://post-stop-job"})
		eng.AnalyzeURL("http://post-stop-url")
	}
}

// TestChallenger_ConcurrentWorkersAndRapidStop stresses concurrent workers
// fetching HTTP targets while Stop() is called concurrently.
func TestChallenger_ConcurrentWorkersAndRapidStop(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	for cycle := 0; cycle < 5; cycle++ {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Brief delay to simulate network latency and keep workers busy
			time.Sleep(5 * time.Millisecond)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><a href=\"/link-%s\">Link</a></body></html>", r.URL.Path)
		}))

		eng := NewEngine()
		eng.Start(10)

		var (
			stopTrigger = make(chan struct{})
			wg          sync.WaitGroup
			jobCounter  atomic.Int64
		)

		// 10 concurrent goroutines rapidly submitting jobs, pausing, resuming, stopping
		for workerID := 0; workerID < 10; workerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < 50; i++ {
					n := jobCounter.Add(1)
					u := fmt.Sprintf("%s/job-%d-%d", ts.URL, id, n)
					eng.AddJob(u, Settings{Depth: 2, Speed: SpeedFast})

					if i%3 == 0 {
						eng.PauseJob(u)
					}
					if i%5 == 0 {
						eng.ResumeJob(u)
					}
					if i%7 == 0 {
						eng.StopJob(u)
					}
					if i%11 == 0 {
						eng.TogglePauseAll()
					}

					select {
					case <-stopTrigger:
						return
					default:
					}
				}
			}(workerID)
		}

		// Let it run under load for 15 milliseconds, then abruptly Stop()
		time.Sleep(15 * time.Millisecond)
		close(stopTrigger)

		stopDone := make(chan struct{})
		go func() {
			// Trigger concurrent Stop() calls from multiple goroutines
			var stopWg sync.WaitGroup
			for s := 0; s < 4; s++ {
				stopWg.Add(1)
				go func() {
					defer stopWg.Done()
					eng.Stop()
				}()
			}
			stopWg.Wait()
			close(stopDone)
		}()

		select {
		case <-stopDone:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatalf("Cycle %d: Concurrent Stop() deadlocked", cycle)
		}

		wg.Wait()
		ts.Close()
	}
}

// TestChallenger_ResolveSavePathBoundaryAttacks thoroughly tests sanitizeHost
// and resolveSavePath against malformed, malicious, or extreme URL boundary cases.
func TestChallenger_ResolveSavePathBoundaryAttacks(t *testing.T) {
	type testCase struct {
		name          string
		rawURL        string
		isImage       bool
		expectValid   bool   // whether it should return a non-empty safe path
		expectedSub   string // substring expected in the resolved path
		mustNotContain string // string that must NOT be present
	}

	cases := []testCase{
		// 1. Multiple Port Colons
		{
			name:          "Multiple port colons in host",
			rawURL:        "http://example.com:80:80/page.html",
			isImage:       false,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "example.com_80", "page.html"),
			mustNotContain: ":",
		},
		{
			name:          "Host ending with trailing colon",
			rawURL:        "http://example.com:/page.html",
			isImage:       false,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "example.com", "page.html"),
			mustNotContain: ":",
		},
		{
			name:          "IP with multiple port colons",
			rawURL:        "http://127.0.0.1:8080:9090/data.json",
			isImage:       false,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "127.0.0.1_8080", "data.json"),
			mustNotContain: ":",
		},

		// 2. IPv6 Brackets and Colons
		{
			name:          "Standard IPv6 loopback with port",
			rawURL:        "http://[::1]:8080/index.html",
			isImage:       false,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "__1", "index.html"),
			mustNotContain: ":",
		},
		{
			name:          "IPv6 address without port",
			rawURL:        "http://[2001:db8::1]/path/test.html",
			isImage:       false,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "2001_db8__1", "path", "test.html"),
			mustNotContain: ":",
		},
		{
			name:          "IPv4-mapped IPv6 address",
			rawURL:        "http://[::ffff:192.0.2.1]:80/img.png",
			isImage:       true,
			expectValid:   true,
			expectedSub:   filepath.Join("output", "__ffff_192.0.2.1", "img.png"),
			mustNotContain: ":",
		},

		// 3. Trailing Slashes and Empty Paths
		{
			name:        "Root path trailing slash HTML",
			rawURL:      "http://example.com/",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "index.html"),
		},
		{
			name:        "Root path trailing slash Image",
			rawURL:      "http://example.com/",
			isImage:     true,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "image"),
		},
		{
			name:        "No path HTML",
			rawURL:      "http://example.com",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "index.html"),
		},
		{
			name:        "No path Image",
			rawURL:      "http://example.com",
			isImage:     true,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "image"),
		},
		{
			name:        "Subdirectory trailing slash HTML",
			rawURL:      "http://example.com/deep/nested/dir/",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "deep", "nested", "dir", "index.html"),
		},
		{
			name:        "Multiple consecutive trailing slashes",
			rawURL:      "http://example.com/deep////",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "deep", "index.html"),
		},

		// 4. Dot Segments and Path Traversal Attacks
		{
			name:        "Double dot traversal to root",
			rawURL:      "http://example.com/../../etc/passwd",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "etc", "passwd"),
		},
		{
			name:        "Deep double dot traversal",
			rawURL:      "http://example.com/a/b/c/../../../../../../../../../../var/log/syslog",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "var", "log", "syslog"),
		},
		{
			name:        "Dot-slash normalization",
			rawURL:      "http://example.com/././a/./b/./c.html",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "a", "b", "c.html"),
		},
		{
			name:        "Multiple dots and slashes",
			rawURL:      "http://example.com/....//....//sensitive.txt",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "example.com", "....", "....", "sensitive.txt"),
		},

		// 5. Host-Level Traversal Attempts
		{
			name:        "Host consists only of dot-dot",
			rawURL:      "http://../test.html",
			isImage:     false,
			expectValid: true,
			// Host .. is sanitized to _
			expectedSub: filepath.Join("output", "_", "test.html"),
		},
		{
			name:        "Host with leading and trailing dots",
			rawURL:      "http://.attacker.com./test.html",
			isImage:     false,
			expectValid: true,
			expectedSub: filepath.Join("output", "attacker.com", "test.html"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("Failed to parse URL %q: %v", tc.rawURL, err)
			}
			resolved := resolveSavePath(u, tc.isImage)

			if !tc.expectValid {
				if resolved != "" {
					t.Errorf("Expected empty/invalid path for %q, got %q", tc.rawURL, resolved)
				}
				return
			}

			if resolved == "" {
				t.Fatalf("Expected valid path for %q, got empty string", tc.rawURL)
			}

			// Invariant 1: Path MUST reside inside output/
			cleanResolved := filepath.Clean(resolved)
			cleanOutput := filepath.Clean("output")
			if !strings.HasPrefix(cleanResolved, cleanOutput+string(filepath.Separator)) {
				t.Fatalf("PATH TRAVERSAL ESCAPE: %q resolved to %q, outside output/", tc.rawURL, cleanResolved)
			}

			// Invariant 2: Path MUST reside inside output/<host>/
			host := sanitizeHost(u)
			baseHostDir := filepath.Clean(filepath.Join("output", host))
			if cleanResolved == baseHostDir {
				t.Fatalf("PATH COLLISION: %q resolved directly to base host dir %q", tc.rawURL, cleanResolved)
			}
			if !strings.HasPrefix(cleanResolved, baseHostDir+string(filepath.Separator)) {
				t.Fatalf("HOST ESCAPE: %q resolved to %q, not prefixed by %q", tc.rawURL, cleanResolved, baseHostDir)
			}

			// Invariant 3: Expected substring match
			if tc.expectedSub != "" && !strings.Contains(cleanResolved, filepath.Clean(tc.expectedSub)) {
				t.Errorf("Expected path to contain %q, got %q", tc.expectedSub, cleanResolved)
			}

			// Invariant 4: Forbidden substring
			if tc.mustNotContain != "" && strings.Contains(cleanResolved, tc.mustNotContain) {
				t.Errorf("Resolved path %q contains forbidden token %q", cleanResolved, tc.mustNotContain)
			}
		})
	}
}

// TestChallenger_DirectoryFileCollisionScenarios tests real filesystem conflicts:
// 1. A file exists where a directory is needed for nested files.
// 2. A directory exists where a file is needed.
func TestChallenger_DirectoryFileCollisionScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	eng := NewEngine()
	eng.Start(2)
	defer eng.Stop()

	// SCENARIO 1: Existing file blocks directory creation
	// Pre-create output/collision.test/section as a regular file
	sectionDir := filepath.Join("output", "collision.test")
	if err := os.MkdirAll(sectionDir, 0755); err != nil {
		t.Fatal(err)
	}
	sectionFilePath := filepath.Join(sectionDir, "section")
	if err := os.WriteFile(sectionFilePath, []byte("existing-file-content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Now fetch a URL that requires 'section' to become a directory: http://collision.test/section/page.html
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body>Nested Page Under Section</body></html>")
	}))
	defer ts1.Close()

	// Parse server host
	u, _ := url.Parse(ts1.URL)
	mockHost := u.Hostname()

	// Pre-create a conflicting file under the mock server's output host
	mockBaseDir := filepath.Join("output", mockHost)
	if err := os.MkdirAll(mockBaseDir, 0755); err != nil {
		t.Fatal(err)
	}
	conflictingFile := filepath.Join(mockBaseDir, "subpage")
	if err := os.WriteFile(conflictingFile, []byte("conflicting-file"), 0600); err != nil {
		t.Fatal(err)
	}

	// Request a child of the conflicting file: /subpage/child.html
	eng.AddJob(ts1.URL+"/subpage/child.html", Settings{Depth: 1, Speed: SpeedFast})

	select {
	case res := <-eng.Results:
		if res.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", res.StatusCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for result in collision scenario 1")
	}

	select {
	case savedFile := <-eng.Files:
		expectedChild := filepath.Join("output", mockHost, "subpage", "child.html")
		if filepath.Clean(savedFile) != filepath.Clean(expectedChild) {
			t.Errorf("Expected saved child at %q, got %q", expectedChild, savedFile)
		}
		// Verify renamed file exists
		renamedFile := filepath.Join("output", mockHost, "subpage.file")
		if _, err := os.Stat(renamedFile); os.IsNotExist(err) {
			t.Errorf("Expected conflicting file to be renamed to %q", renamedFile)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for file emission in collision scenario 1")
	}

	// SCENARIO 2: Existing directory blocks file creation
	// Pre-create output/<mockHost>/existingdir as a directory
	existingDir := filepath.Join("output", mockHost, "existingdir")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Request URL that maps directly to 'existingdir': http://<mockHost>/existingdir
	eng.AddJob(ts1.URL+"/existingdir", Settings{Depth: 1, Speed: SpeedFast})

	select {
	case res := <-eng.Results:
		if res.StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK, got %d", res.StatusCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for result in collision scenario 2")
	}

	select {
	case savedFile := <-eng.Files:
		// Should have been routed into existingdir/index.html
		expectedIndex := filepath.Join("output", mockHost, "existingdir", "index.html")
		if filepath.Clean(savedFile) != filepath.Clean(expectedIndex) {
			t.Errorf("Expected file saved as %q, got %q", expectedIndex, savedFile)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for file emission in collision scenario 2")
	}
}

// TestChallenger_SSRFAndPrivateNetworkExclusion tests that internal/private networks
// and cloud metadata endpoints are strictly blocked from link traversal.
func TestChallenger_SSRFAndPrivateNetworkExclusion(t *testing.T) {
	parentPublic, _ := url.Parse("https://public-service.com/index.html")
	parentLocal, _ := url.Parse("http://localhost:8080/index.html")

	ssrfTargets := []struct {
		target   string
		allowed  bool
		fromHost *url.URL
	}{
		// Cloud Metadata endpoints (MUST NEVER BE ALLOWED from any host)
		{"http://169.254.169.254/latest/meta-data/", false, parentPublic},
		{"http://169.254.169.254/latest/meta-data/", false, parentLocal},
		{"http://metadata.google.internal/computeMetadata/v1/", false, parentPublic},
		{"http://metadata.google.internal/computeMetadata/v1/", false, parentLocal},

		// Cross-domain escapes from public site
		{"https://another-domain.com/data", false, parentPublic},
		{"http://127.0.0.1:8080/secret", false, parentPublic},
		{"http://10.0.0.1/admin", false, parentPublic},
		{"http://192.168.1.1/router", false, parentPublic},

		// Same host public
		{"https://public-service.com/about", true, parentPublic},
		{"https://public-service.com/contact.html", true, parentPublic},
	}

	for _, tc := range ssrfTargets {
		t.Run(tc.target, func(t *testing.T) {
			targetURL, err := url.Parse(tc.target)
			if err != nil {
				t.Fatalf("Failed to parse target %s: %v", tc.target, err)
			}
			allowed := isAllowedCrawlTarget(tc.fromHost, targetURL)
			if allowed != tc.allowed {
				t.Errorf("isAllowedCrawlTarget(%s, %s) = %v, expected %v",
					tc.fromHost, tc.target, allowed, tc.allowed)
			}
		})
	}
}
