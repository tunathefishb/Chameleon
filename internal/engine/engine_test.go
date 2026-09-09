package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEngineFetching(t *testing.T) {
	// Create a local test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, "<html><body><a href=\"/link1\">Link 1</a></body></html>")
	}))
	defer ts.Close()

	// Initialize the engine
	eng := NewEngine()
	eng.Start(1)

	// Ensure we cleanup output directory created by test
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	// Add a job pointing to the test server
	settings := Settings{
		Depth:  1,
		Images: false,
		Speed:  SpeedFast,
	}
	eng.AddJob(ts.URL, settings)

	// Verify the result is emitted via Results channel
	select {
	case res := <-eng.Results:
		if res.Status != "200" {
			t.Errorf("Expected status 200, got %s", res.Status)
		}
		if res.Type != "text/html" {
			t.Errorf("Expected type text/html, got %s", res.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Test timed out waiting for result")
	}

	if eng.GetJobStatus(ts.URL) != StatusDone {
		t.Errorf("Expected job status to be StatusDone, got %s", eng.GetJobStatus(ts.URL))
	}
}

func TestEnginePauseResume(t *testing.T) {
	eng := NewEngine()
	targetURL := "https://example.com/test-pause"
	settings := Settings{Depth: 1, Speed: SpeedSafe}

	eng.AddJob(targetURL, settings)
	if eng.GetJobStatus(targetURL) != StatusQueued {
		t.Fatalf("expected status Queued, got %s", eng.GetJobStatus(targetURL))
	}

	eng.PauseJob(targetURL)
	if eng.GetJobStatus(targetURL) != StatusPaused {
		t.Fatalf("expected status Paused, got %s", eng.GetJobStatus(targetURL))
	}

	eng.TogglePauseJob(targetURL)
	if eng.GetJobStatus(targetURL) != StatusQueued {
		t.Fatalf("expected status Queued after toggle resume, got %s", eng.GetJobStatus(targetURL))
	}
}

func TestEnginePauseAllResumeAll(t *testing.T) {
	eng := NewEngine()
	url1 := "https://example.com/pause-all-1"
	url2 := "https://example.com/pause-all-2"
	url3 := "https://example.com/pause-all-3"
	settings := Settings{Depth: 1, Speed: SpeedSafe}

	eng.AddJob(url1, settings)
	eng.AddJob(url2, settings)
	eng.AddJob(url3, settings)

	if eng.GetJobStatus(url1) != StatusQueued || eng.GetJobStatus(url2) != StatusQueued || eng.GetJobStatus(url3) != StatusQueued {
		t.Fatalf("expected all jobs to be Queued")
	}

	// Test PauseAll
	eng.PauseAll()
	if eng.GetJobStatus(url1) != StatusPaused || eng.GetJobStatus(url2) != StatusPaused || eng.GetJobStatus(url3) != StatusPaused {
		t.Fatalf("expected all jobs to be Paused after PauseAll")
	}

	// Test ResumeAll
	eng.ResumeAll()
	if eng.GetJobStatus(url1) != StatusQueued || eng.GetJobStatus(url2) != StatusQueued || eng.GetJobStatus(url3) != StatusQueued {
		t.Fatalf("expected all jobs to be Queued after ResumeAll")
	}

	// Test TogglePauseAll (when active -> pauses all)
	paused := eng.TogglePauseAll()
	if !paused {
		t.Fatalf("expected TogglePauseAll to return true when pausing active jobs")
	}
	if eng.GetJobStatus(url1) != StatusPaused || eng.GetJobStatus(url2) != StatusPaused || eng.GetJobStatus(url3) != StatusPaused {
		t.Fatalf("expected all jobs to be Paused after TogglePauseAll")
	}

	// Test TogglePauseAll (when all paused -> resumes all)
	paused = eng.TogglePauseAll()
	if paused {
		t.Fatalf("expected TogglePauseAll to return false when resuming paused jobs")
	}
	if eng.GetJobStatus(url1) != StatusQueued || eng.GetJobStatus(url2) != StatusQueued || eng.GetJobStatus(url3) != StatusQueued {
		t.Fatalf("expected all jobs to be Queued after second TogglePauseAll")
	}
}

func TestEngineStopJob(t *testing.T) {
	eng := NewEngine()
	targetURL := "https://example.com/test-stop"
	settings := Settings{Depth: 1, Speed: SpeedSafe}

	eng.AddJob(targetURL, settings)
	eng.StopJob(targetURL)
	if eng.GetJobStatus(targetURL) != StatusStopped {
		t.Fatalf("expected status Stopped, got %s", eng.GetJobStatus(targetURL))
	}
}

func TestEngineImageScrapingAndRewriting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body>
				<img id="main-img" src="/images/logo.png" />
				<picture>
					<source id="hero-source" srcset="/images/hero.webp 1x, /images/hero-2x.webp 2x" />
					<img id="fallback-img" src="/images/fallback.jpg" />
				</picture>
			</body></html>`)
		case "/images/logo.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("fake-png-data"))
		case "/images/hero.webp":
			w.Header().Set("Content-Type", "image/webp")
			_, _ = w.Write([]byte("fake-webp-data"))
		case "/images/hero-2x.webp":
			w.Header().Set("Content-Type", "image/webp")
			_, _ = w.Write([]byte("fake-webp-2x-data"))
		case "/images/fallback.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("fake-jpg-data"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	_ = os.RemoveAll("output")
	eng := NewEngine()
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

	eng.AddJob(ts.URL+"/", settings)

	// Expect 5 results: 1 HTML + 4 images
	received := 0
	receivedFiles := 0
	timeout := time.After(5 * time.Second)
	for received < 5 || receivedFiles < 5 {
		select {
		case <-eng.Results:
			received++
		case <-eng.Files:
			receivedFiles++
		case <-timeout:
			t.Fatalf("timed out waiting for results (%d/5) and files (%d/5)", received, receivedFiles)
		}
	}

	// Verify the saved files exist
	// Parse server host
	parsedURL, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	host := parsedURL.URL.Hostname()

	htmlPath := filepath.Join("output", host, "index.html")
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("failed to read saved HTML file: %v", err)
	}

	htmlContent := string(htmlBytes)
	if !strings.Contains(htmlContent, `src="images/logo.png"`) {
		t.Errorf("expected HTML to contain rewritten src=\"images/logo.png\", got:\n%s", htmlContent)
	}
	if !strings.Contains(htmlContent, `srcset="images/hero.webp 1x, images/hero-2x.webp 2x"`) {
		t.Errorf("expected HTML to contain rewritten srcset, got:\n%s", htmlContent)
	}
	if !strings.Contains(htmlContent, `src="images/fallback.jpg"`) {
		t.Errorf("expected HTML to contain rewritten src=\"images/fallback.jpg\", got:\n%s", htmlContent)
	}

	// Verify image files exist on disk
	imgFiles := []string{
		filepath.Join("output", host, "images", "logo.png"),
		filepath.Join("output", host, "images", "hero.webp"),
		filepath.Join("output", host, "images", "hero-2x.webp"),
		filepath.Join("output", host, "images", "fallback.jpg"),
	}

	for _, imgFile := range imgFiles {
		if _, err := os.Stat(imgFile); os.IsNotExist(err) {
			t.Errorf("expected image file to exist at %s", imgFile)
		}
	}
}

func TestEnginePathTraversalProtection(t *testing.T) {
	maliciousURLs := []string{
		"http://example.com/../../etc/passwd",
		"http://example.com/....//....//sensitive",
		"http://example.com/static/../../../root/secret.txt",
	}

	for _, raw := range maliciousURLs {
		parsed, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatalf("failed to parse url: %v", err)
		}
		savePath := resolveSavePath(parsed.URL, false)
		if savePath != "" {
			expectedPrefix := filepath.Clean(filepath.Join("output", "example.com"))
			if !strings.HasPrefix(filepath.Clean(savePath), expectedPrefix) {
				t.Fatalf("path traversal escape detected for %s: resolved to %s (not prefixed by %s)", raw, savePath, expectedPrefix)
			}
		}
	}
}

func TestEngineRetryStoppedJob(t *testing.T) {
	eng := NewEngine()
	targetURL := "https://example.com/retry-test"
	settings := Settings{Depth: 1, Speed: SpeedSafe}

	eng.AddJob(targetURL, settings)
	if eng.GetJobStatus(targetURL) != StatusQueued {
		t.Fatalf("expected initial status Queued, got %s", eng.GetJobStatus(targetURL))
	}

	eng.StopJob(targetURL)
	if eng.GetJobStatus(targetURL) != StatusStopped {
		t.Fatalf("expected status Stopped, got %s", eng.GetJobStatus(targetURL))
	}

	// Re-add the same stopped job
	eng.AddJob(targetURL, settings)
	if eng.GetJobStatus(targetURL) != StatusQueued {
		t.Fatalf("expected status Queued after re-adding stopped job, got %s", eng.GetJobStatus(targetURL))
	}
}

func TestEngineBoundedReading(t *testing.T) {
	// Server returns a response larger than maxResponseBodySize (10MB)
	chunk := make([]byte, 1024*1024) // 1MB
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		for range 12 { // 12MB total
			_, _ = w.Write(chunk)
		}
	}))
	defer ts.Close()

	eng := NewEngine()
	eng.Start(1)
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	eng.AddJob(ts.URL, Settings{Depth: 1, Speed: SpeedFast})

	select {
	case res := <-eng.Results:
		if res.Status != "200" {
			t.Fatalf("expected status 200, got %s", res.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for bounded read result")
	}

	var savedFilePath string
	select {
	case savedFilePath = <-eng.Files:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file to be saved")
	}

	// Verify saved file size is bounded to <= 10MB
	fi, err := os.Stat(savedFilePath)
	if err != nil {
		t.Fatalf("failed to stat saved file: %v", err)
	}
	if fi.Size() > 10*1024*1024 {
		t.Fatalf("saved file size %d exceeds 10MB bound", fi.Size())
	}
}

func TestEngineHasActiveWork(t *testing.T) {
	defer os.RemoveAll("output")
	eng := NewEngine()
	defer eng.Stop()

	if eng.HasActiveWork() {
		t.Fatalf("expected HasActiveWork to be false initially")
	}

	eng.jobStates.Store("http://example.com/test", StatusQueued)
	if !eng.HasActiveWork() {
		t.Fatalf("expected HasActiveWork to be true when a job is queued")
	}

	eng.jobStates.Store("http://example.com/test", StatusRunning)
	if !eng.HasActiveWork() {
		t.Fatalf("expected HasActiveWork to be true when a job is running")
	}

	eng.jobStates.Store("http://example.com/test", StatusDone)
	if eng.HasActiveWork() {
		t.Fatalf("expected HasActiveWork to be false when all jobs are done")
	}
}

func TestResolveSavePathSecurity(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		isImage     bool
		expectedRel string
	}{
		{
			name:        "Standard URL",
			rawURL:      "http://example.com/about/team.html",
			isImage:     false,
			expectedRel: filepath.Join("output", "example.com", "about", "team.html"),
		},
		{
			name:        "Host with standard port",
			rawURL:      "http://example.com:8080/data.json",
			isImage:     false,
			expectedRel: filepath.Join("output", "example.com", "data.json"),
		},
		{
			name:        "IPv6 host with port",
			rawURL:      "http://[::1]:8443/index",
			isImage:     false,
			expectedRel: filepath.Join("output", "__1", "index"),
		},
		{
			name:        "Empty path resolves to index.html",
			rawURL:      "http://example.com",
			isImage:     false,
			expectedRel: filepath.Join("output", "example.com", "index.html"),
		},
		{
			name:        "Trailing slash resolves to index.html",
			rawURL:      "http://example.com/docs/",
			isImage:     false,
			expectedRel: filepath.Join("output", "example.com", "docs", "index.html"),
		},
		{
			name:        "Empty path for image resolves to image",
			rawURL:      "http://example.com/",
			isImage:     true,
			expectedRel: filepath.Join("output", "example.com", "image"),
		},
		{
			name:        "Directory traversal attempt is neutralized",
			rawURL:      "http://example.com/../../etc/passwd",
			isImage:     false,
			expectedRel: filepath.Join("output", "example.com", "etc", "passwd"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("failed to parse url %s: %v", tc.rawURL, err)
			}
			got := resolveSavePath(u, tc.isImage)
			cleanExpected := filepath.Clean(tc.expectedRel)
			cleanGot := filepath.Clean(got)
			if cleanGot != cleanExpected {
				t.Errorf("expected %q, got %q", cleanExpected, cleanGot)
			}
			// Verify it never equals output/<host> directly
			baseHostDir := filepath.Clean(filepath.Join("output", sanitizeHost(u)))
			if cleanGot == baseHostDir {
				t.Errorf("path equals base directory output/<host>, should be nested inside: %q", cleanGot)
			}
		})
	}
}

func TestSymlinkTraversalProtection(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll("output/example.com", 0755); err != nil {
		t.Fatal(err)
	}

	outsideDir := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("sensitive"), 0600); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join("output", "example.com", "linked.txt")
	if err := os.Symlink(secretFile, symlinkPath); err != nil {
		t.Skipf("symlinks not supported in environment: %v", err)
	}

	if isSafeOutputPath(symlinkPath) {
		t.Errorf("expected isSafeOutputPath to reject symlink pointing outside output/")
	}

	safePath := filepath.Join("output", "example.com", "normal.txt")
	if !isSafeOutputPath(safePath) {
		t.Errorf("expected isSafeOutputPath to accept normal path inside output/")
	}
}

func TestHTTPStatusErrorAndRetry(t *testing.T) {
	statusCode := http.StatusNotFound
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Not Found", statusCode)
	}))
	defer ts.Close()

	eng := NewEngine()
	eng.Start(1)
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	settings := Settings{Depth: 1, Speed: SpeedFast}
	eng.AddJob(ts.URL, settings)

	select {
	case res := <-eng.Results:
		if res.StatusCode != 404 {
			t.Errorf("expected status code 404, got %d", res.StatusCode)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for 404 result")
	}

	st := eng.GetJobStatus(ts.URL)
	if st != StatusError {
		t.Fatalf("expected job status StatusError for 404 response, got %s", st)
	}

	eng.AddJob(ts.URL, settings)
	stAfterRetry := eng.GetJobStatus(ts.URL)
	if stAfterRetry != StatusQueued && stAfterRetry != StatusRunning && stAfterRetry != StatusError {
		t.Fatalf("expected job to be re-enqueued, got status %s", stAfterRetry)
	}
}

func TestSendResultNonBlockingAndCleanShutdown(t *testing.T) {
	eng := NewEngine()
	for i := 0; i < 1005; i++ {
		eng.sendResult(Result{
			Name:       fmt.Sprintf("item-%d", i),
			Status:     "200",
			StatusCode: 200,
		})
	}

	done := make(chan struct{})
	go func() {
		eng.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("eng.Stop() deadlocked or timed out during shutdown under channel backpressure")
	}

	eng.sendResult(Result{Name: "post-stop", Status: "200"})
}

func TestAllJobsEvictionOnStopJob(t *testing.T) {
	eng := NewEngine()
	targetURL := "https://example.com/leak-test"
	settings := Settings{Depth: 1, Speed: SpeedSafe}

	eng.AddJob(targetURL, settings)

	eng.allJobsMu.Lock()
	_, exists := eng.allJobs[targetURL]
	eng.allJobsMu.Unlock()
	if !exists {
		t.Fatalf("expected job to be in allJobs after AddJob")
	}

	eng.StopJob(targetURL)

	eng.allJobsMu.Lock()
	_, stillExists := eng.allJobs[targetURL]
	eng.allJobsMu.Unlock()
	if stillExists {
		t.Fatalf("job leaked in allJobs after StopJob")
	}
	if eng.GetJobStatus(targetURL) != StatusStopped {
		t.Fatalf("expected status Stopped, got %s", eng.GetJobStatus(targetURL))
	}
}

func TestRecursiveLinkDomainBoundingAndSSRF(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, `<html><body>
			<a href="/allowed-child">Internal Link</a>
			<a href="https://external-domain.com/unauthorized">External Link</a>
			<a href="http://169.254.169.254/latest/meta-data/">Cloud Metadata SSRF</a>
		</body></html>`)
	}))
	defer ts.Close()

	eng := NewEngine()
	eng.Start(1)
	defer func() {
		eng.Stop()
		os.RemoveAll("output")
	}()

	settings := Settings{
		Depth:  2,
		Images: false,
		Speed:  SpeedFast,
	}
	eng.AddJob(ts.URL, settings)

	discoveredURLs := make(map[string]bool)
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case u := <-eng.Discovered:
			discoveredURLs[u] = true
		case <-time.After(200 * time.Millisecond):
			break loop
		case <-timeout:
			break loop
		}
	}

	if !discoveredURLs[ts.URL+"/allowed-child"] {
		t.Errorf("expected same-host child link to be discovered, got %v", discoveredURLs)
	}
	if discoveredURLs["https://external-domain.com/unauthorized"] {
		t.Errorf("external domain should not have been crawled (SSRF / domain boundary leak)")
	}
	if discoveredURLs["http://169.254.169.254/latest/meta-data/"] {
		t.Errorf("cloud metadata IP should never have been crawled (SSRF hazard)")
	}
}

func TestAnalyzeURLFallbackErrorReport(t *testing.T) {
	eng := NewEngine()
	defer eng.Stop()

	eng.AnalyzeURL("http://invalid url with spaces")

	select {
	case report := <-eng.Analysis:
		if report.EthicalGrade != "F" {
			t.Errorf("expected EthicalGrade F, got %s", report.EthicalGrade)
		}
		if report.DifficultyLevel != "Extreme" {
			t.Errorf("expected DifficultyLevel Extreme, got %s", report.DifficultyLevel)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Analysis report fallback on error")
	}
}
