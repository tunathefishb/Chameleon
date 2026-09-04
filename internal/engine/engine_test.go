package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	host := parsedURL.URL.Host

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
