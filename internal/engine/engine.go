package engine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"chameleon/internal/engine/analyzer"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

type JobStatus string

const (
	StatusQueued  JobStatus = "Queued"
	StatusRunning JobStatus = "Running"
	StatusPaused  JobStatus = "Paused"
	StatusStopped JobStatus = "Stopped"
	StatusDone    JobStatus = "Done"
	StatusError   JobStatus = "Error"
)

const (
	SpeedFast = "Fast"
	SpeedSafe = "Safe"
)

type Settings struct {
	Depth  int
	Images bool
	Speed  string
}

type Job struct {
	URL      string
	Settings Settings
	Depth    int // Current depth
}

type Result struct {
	Name            string
	Status          string
	Type            string
	Size            string
	Time            string
	URL             string
	Method          string
	StatusCode      int
	ResponseHeaders http.Header
	ErrorMsg        string
	Timestamp       time.Time
}

type Engine struct {
	Jobs       chan Job
	Results    chan Result
	Files      chan string
	Discovered chan string
	Analysis   chan analyzer.Report

	analyzer *analyzer.Analyzer
	limiter  *rate.Limiter
	client   *http.Client
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc

	visited sync.Map

	// Job management
	jobStates  sync.Map // url (string) -> JobStatus
	jobCancels sync.Map // url (string) -> context.CancelFunc
	pausedMu   sync.Mutex
	pausedJobs map[string]Job
	allJobsMu  sync.Mutex
	allJobs    map[string]Job
	stopOnce   sync.Once
}

func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	client := &http.Client{Timeout: 10 * time.Second}
	return &Engine{
		Jobs:       make(chan Job, 1000),
		Results:    make(chan Result, 1000),
		Files:      make(chan string, 1000),
		Discovered: make(chan string, 1000),
		Analysis:   make(chan analyzer.Report, 100),
		analyzer:   analyzer.NewAnalyzer(client, nil),
		limiter:    rate.NewLimiter(rate.Every(1*time.Second), 1), // Default 1 req/sec
		client:     client,
		ctx:        ctx,
		cancel:     cancel,
		pausedJobs: make(map[string]Job),
		allJobs:    make(map[string]Job),
	}
}

func (e *Engine) Start(workers int) {
	for range workers {
		e.wg.Add(1)
		go e.worker()
	}
}

func (e *Engine) Stop() {
	e.stopOnce.Do(func() {
		e.cancel()
		e.wg.Wait()
		close(e.Jobs)
		close(e.Results)
		close(e.Files)
		close(e.Discovered)
		close(e.Analysis)
	})
}

func (e *Engine) AddJob(rawURL string, settings Settings) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}
	targetURL := rawURL
	if parsed, err := url.Parse(rawURL); err == nil {
		parsed.Fragment = ""
		targetURL = parsed.String()
	}

	st := e.GetJobStatus(targetURL)
	if st == StatusError || st == StatusStopped {
		e.visited.Delete(targetURL)
	}

	if _, loaded := e.visited.LoadOrStore(targetURL, true); !loaded {
		job := Job{URL: targetURL, Settings: settings, Depth: 0}
		e.allJobsMu.Lock()
		e.allJobs[targetURL] = job
		e.allJobsMu.Unlock()
		e.jobStates.Store(targetURL, StatusQueued)

		select {
		case e.Jobs <- job:
		default:
			go func() {
				select {
				case <-e.ctx.Done():
				case e.Jobs <- job:
				}
			}()
		}
	}
}

func (e *Engine) GetJobStatus(url string) JobStatus {
	val, ok := e.jobStates.Load(url)
	if !ok {
		return StatusQueued
	}
	return val.(JobStatus)
}

func (e *Engine) PauseJob(url string) {
	st := e.GetJobStatus(url)
	if st == StatusDone || st == StatusStopped || st == StatusError {
		return
	}
	e.jobStates.Store(url, StatusPaused)

	// If currently running, cancel the in-flight HTTP request context
	if cancelVal, ok := e.jobCancels.Load(url); ok {
		if cancelFn, isFn := cancelVal.(context.CancelFunc); isFn {
			cancelFn()
		}
	}

	// Retain in pausedJobs
	e.allJobsMu.Lock()
	if job, ok := e.allJobs[url]; ok {
		e.pausedMu.Lock()
		e.pausedJobs[url] = job
		e.pausedMu.Unlock()
	}
	e.allJobsMu.Unlock()
}

func (e *Engine) ResumeJob(url string) {
	st := e.GetJobStatus(url)
	if st != StatusPaused {
		return
	}
	e.jobStates.Store(url, StatusQueued)

	e.pausedMu.Lock()
	job, ok := e.pausedJobs[url]
	if ok {
		delete(e.pausedJobs, url)
	}
	e.pausedMu.Unlock()

	if !ok {
		e.allJobsMu.Lock()
		job = e.allJobs[url]
		e.allJobsMu.Unlock()
	}

	if job.URL != "" {
		select {
		case e.Jobs <- job:
		default:
			go func() {
				select {
				case <-e.ctx.Done():
				case e.Jobs <- job:
				}
			}()
		}
	}
}

func (e *Engine) StopJob(url string) {
	e.jobStates.Store(url, StatusStopped)

	e.pausedMu.Lock()
	delete(e.pausedJobs, url)
	e.pausedMu.Unlock()

	e.visited.Delete(url)

	if cancelVal, ok := e.jobCancels.Load(url); ok {
		if cancelFn, isFn := cancelVal.(context.CancelFunc); isFn {
			cancelFn()
		}
	}
}

// HasActiveWork returns true if there are any jobs currently queued or running.
func (e *Engine) HasActiveWork() bool {
	hasWork := false
	e.jobStates.Range(func(key, val any) bool {
		if st, ok := val.(JobStatus); ok {
			if st == StatusQueued || st == StatusRunning {
				hasWork = true
				return false
			}
		}
		return true
	})
	return hasWork
}

func (e *Engine) TogglePauseJob(url string) {
	st := e.GetJobStatus(url)
	switch st {
	case StatusPaused:
		e.ResumeJob(url)
	case StatusQueued, StatusRunning:
		e.PauseJob(url)
	}
}

func (e *Engine) PauseAll() {
	e.allJobsMu.Lock()
	urls := make([]string, 0, len(e.allJobs))
	for u := range e.allJobs {
		urls = append(urls, u)
	}
	e.allJobsMu.Unlock()

	for _, u := range urls {
		st := e.GetJobStatus(u)
		if st == StatusQueued || st == StatusRunning {
			e.PauseJob(u)
		}
	}
}

func (e *Engine) ResumeAll() {
	e.pausedMu.Lock()
	urls := make([]string, 0, len(e.pausedJobs))
	for u := range e.pausedJobs {
		urls = append(urls, u)
	}
	e.pausedMu.Unlock()

	e.allJobsMu.Lock()
	for u := range e.allJobs {
		if e.GetJobStatus(u) == StatusPaused {
			found := false
			for _, pu := range urls {
				if pu == u {
					found = true
					break
				}
			}
			if !found {
				urls = append(urls, u)
			}
		}
	}
	e.allJobsMu.Unlock()

	for _, u := range urls {
		e.ResumeJob(u)
	}
}

func (e *Engine) TogglePauseAll() bool {
	hasActive := false
	hasPaused := false

	e.allJobsMu.Lock()
	for u := range e.allJobs {
		st := e.GetJobStatus(u)
		if st == StatusQueued || st == StatusRunning {
			hasActive = true
			break
		}
		if st == StatusPaused {
			hasPaused = true
		}
	}
	e.allJobsMu.Unlock()

	if hasActive {
		e.PauseAll()
		return true
	} else if hasPaused {
		e.ResumeAll()
		return false
	}
	return false
}

func (e *Engine) AnalyzeURL(targetURL string) {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		report, err := e.analyzer.Analyze(e.ctx, targetURL)
		if err == nil && report != nil {
			select {
			case <-e.ctx.Done():
			case e.Analysis <- *report:
			}
		}
	}()
}

func resolveSavePath(u *url.URL, isImage bool) string {
	host := u.Host
	if host == "" {
		host = "unknown"
	}
	host = strings.ReplaceAll(host, "/", "_")
	host = strings.ReplaceAll(host, "\\", "_")
	host = strings.ReplaceAll(host, "..", "_")
	host = strings.Trim(host, ". ")
	if host == "" {
		host = "unknown"
	}

	baseDir := filepath.Join("output", host)

	cleanPath := path.Clean("/" + u.Path)
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	var targetRel string
	if cleanPath == "" || strings.HasSuffix(u.Path, "/") {
		if isImage {
			targetRel = filepath.Join(filepath.FromSlash(cleanPath), "image")
		} else {
			targetRel = filepath.Join(filepath.FromSlash(cleanPath), "index.html")
		}
	} else {
		targetRel = filepath.FromSlash(cleanPath)
	}

	fullPath := filepath.Join(baseDir, targetRel)

	cleanBase := filepath.Clean(baseDir)
	cleanFull := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanFull, cleanBase+string(filepath.Separator)) && cleanFull != cleanBase {
		return ""
	}
	return cleanFull
}

func (e *Engine) worker() {
	defer e.wg.Done()
	for {
		select {
		case <-e.ctx.Done():
			return
		case job := <-e.Jobs:
			e.processJob(job)
		}
	}
}

func (e *Engine) processJob(job Job) {
	st := e.GetJobStatus(job.URL)
	if st == StatusStopped {
		return
	}
	if st == StatusPaused {
		e.pausedMu.Lock()
		e.pausedJobs[job.URL] = job
		e.pausedMu.Unlock()
		return
	}

	// Adjust rate limit based on speed
	if job.Settings.Speed == SpeedFast {
		e.limiter.SetLimit(rate.Every(200 * time.Millisecond))
	} else {
		e.limiter.SetLimit(rate.Every(1 * time.Second))
	}

	jobCtx, jobCancel := context.WithCancel(e.ctx)
	e.jobCancels.Store(job.URL, jobCancel)
	defer func() {
		jobCancel()
		e.jobCancels.Delete(job.URL)
	}()

	err := e.limiter.Wait(jobCtx)
	if err != nil {
		st = e.GetJobStatus(job.URL)
		if st == StatusPaused {
			e.pausedMu.Lock()
			e.pausedJobs[job.URL] = job
			e.pausedMu.Unlock()
		}
		return
	}

	st = e.GetJobStatus(job.URL)
	if st == StatusStopped {
		return
	}
	if st == StatusPaused {
		e.pausedMu.Lock()
		e.pausedJobs[job.URL] = job
		e.pausedMu.Unlock()
		return
	}

	e.jobStates.Store(job.URL, StatusRunning)

	start := time.Now()

	req, err := http.NewRequestWithContext(jobCtx, http.MethodGet, job.URL, nil)
	if err != nil {
		e.jobStates.Store(job.URL, StatusError)
		e.Results <- Result{
			Name:      job.URL,
			Status:    "ERR",
			Type:      "-",
			Size:      "-",
			Time:      "-",
			URL:       job.URL,
			Method:    http.MethodGet,
			ErrorMsg:  err.Error(),
			Timestamp: time.Now(),
		}
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ChameleonBot/1.0; +https://example.com/bot)")

	resp, err := e.client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		st = e.GetJobStatus(job.URL)
		if st == StatusPaused {
			e.pausedMu.Lock()
			e.pausedJobs[job.URL] = job
			e.pausedMu.Unlock()
			return
		}
		if st == StatusStopped {
			return
		}
		e.jobStates.Store(job.URL, StatusError)
		e.Results <- Result{
			Name:      job.URL,
			Status:    "ERR",
			Type:      "-",
			Size:      "-",
			Time:      fmt.Sprintf("%dms", elapsed.Milliseconds()),
			URL:       job.URL,
			Method:    http.MethodGet,
			ErrorMsg:  err.Error(),
			Timestamp: start,
		}
		return
	}
	defer resp.Body.Close()

	// 1. Report Telemetry
	status := "200"
	if resp.StatusCode != http.StatusOK {
		status = fmt.Sprintf("%d", resp.StatusCode)
	}
	contentType := resp.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	if contentType == "" {
		contentType = "unknown"
	}

	const maxResponseBodySize = 10 * 1024 * 1024 // 10MB bounded read limit
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize))
	if err != nil {
		e.jobStates.Store(job.URL, StatusError)
		e.sendResult(Result{
			Name:            job.URL,
			Status:          "ERR",
			Type:            contentType,
			Size:            "-",
			Time:            fmt.Sprintf("%dms", elapsed.Milliseconds()),
			URL:             job.URL,
			Method:          http.MethodGet,
			StatusCode:      resp.StatusCode,
			ResponseHeaders: resp.Header.Clone(),
			ErrorMsg:        fmt.Sprintf("read response body error: %v", err),
			Timestamp:       start,
		})
		return
	}

	size := fmt.Sprintf("%d B", len(bodyBytes))
	if len(bodyBytes) > 1024*1024 {
		size = fmt.Sprintf("%.1f MB", float64(len(bodyBytes))/(1024*1024))
	} else if len(bodyBytes) > 1024 {
		size = fmt.Sprintf("%d KB", len(bodyBytes)/1024)
	}

	parsedURL, _ := url.Parse(job.URL)
	name := parsedURL.Path
	if name == "" || name == "/" {
		name = "index.html"
	} else {
		name = filepath.Base(name)
	}

	e.jobStates.Store(job.URL, StatusDone)

	e.sendResult(Result{
		Name:            name,
		Status:          status,
		Type:            contentType,
		Size:            size,
		Time:            fmt.Sprintf("%dms", elapsed.Milliseconds()),
		URL:             job.URL,
		Method:          http.MethodGet,
		StatusCode:      resp.StatusCode,
		ResponseHeaders: resp.Header.Clone(),
		Timestamp:       start,
	})

	// 2. Save File & Process HTML
	if resp.StatusCode == http.StatusOK {
		isImg := strings.HasPrefix(contentType, "image/")
		savePath := resolveSavePath(parsedURL, isImg)
		if savePath == "" {
			return
		}

		// Handle directory/file name conflicts
		if fi, err := os.Stat(savePath); err == nil && fi.IsDir() {
			if isImg {
				savePath = filepath.Join(savePath, "image")
			} else {
				savePath = filepath.Join(savePath, "index.html")
			}
		}
		htmlDir := filepath.Dir(savePath)
		if fi, err := os.Stat(htmlDir); err == nil && !fi.IsDir() {
			_ = os.Rename(htmlDir, htmlDir+".file")
		}

		// Check if HTML and process links & images
		if strings.Contains(contentType, "text/html") {
			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyBytes))
			if err == nil {
				htmlModified := false

				// Process and rewrite images if enabled
				if job.Settings.Images {
					processImageURL := func(rawSrc string) (string, bool) {
						rawSrc = strings.TrimSpace(rawSrc)
						if rawSrc == "" || strings.HasPrefix(rawSrc, "data:") {
							return rawSrc, false
						}
						imgURL, err := parsedURL.Parse(rawSrc)
						if err != nil || (imgURL.Scheme != "http" && imgURL.Scheme != "https") {
							return rawSrc, false
						}
						imgURL.Fragment = ""
						imgURLStr := imgURL.String()

						imgSavePath := resolveSavePath(imgURL, true)
						if imgSavePath == "" {
							return rawSrc, false
						}

						if _, loaded := e.visited.LoadOrStore(imgURLStr, true); !loaded {
							imgJob := Job{
								URL:      imgURLStr,
								Settings: job.Settings,
								Depth:    job.Settings.Depth,
							}
							e.allJobsMu.Lock()
							e.allJobs[imgURLStr] = imgJob
							e.allJobsMu.Unlock()
							e.jobStates.Store(imgURLStr, StatusQueued)

							select {
							case e.Jobs <- imgJob:
							default:
								go func() {
									select {
									case <-e.ctx.Done():
									case e.Jobs <- imgJob:
									}
								}()
							}
							e.sendDiscovered(imgURLStr)
						}

						relPath, err := filepath.Rel(htmlDir, imgSavePath)
						if err != nil {
							return rawSrc, false
						}
						return filepath.ToSlash(relPath), true
					}

					processSrcset := func(rawSrcset string) (string, bool) {
						entries := strings.Split(rawSrcset, ",")
						var newEntries []string
						modified := false
						for _, entry := range entries {
							trimmed := strings.TrimSpace(entry)
							if trimmed == "" {
								continue
							}
							parts := strings.Fields(trimmed)
							if len(parts) > 0 {
								urlPart := parts[0]
								if newRel, ok := processImageURL(urlPart); ok {
									parts[0] = newRel
									modified = true
								}
								newEntries = append(newEntries, strings.Join(parts, " "))
							}
						}
						if modified {
							return strings.Join(newEntries, ", "), true
						}
						return rawSrcset, false
					}

					doc.Find("img").Each(func(_ int, s *goquery.Selection) {
						if src, exists := s.Attr("src"); exists {
							if newSrc, ok := processImageURL(src); ok {
								s.SetAttr("src", newSrc)
								htmlModified = true
							}
						}
						if srcset, exists := s.Attr("srcset"); exists {
							if newSrcset, ok := processSrcset(srcset); ok {
								s.SetAttr("srcset", newSrcset)
								htmlModified = true
							}
						}
					})

					doc.Find("picture source, source").Each(func(_ int, s *goquery.Selection) {
						if srcset, exists := s.Attr("srcset"); exists {
							if newSrcset, ok := processSrcset(srcset); ok {
								s.SetAttr("srcset", newSrcset)
								htmlModified = true
							}
						}
						if src, exists := s.Attr("src"); exists {
							if newSrc, ok := processImageURL(src); ok {
								s.SetAttr("src", newSrc)
								htmlModified = true
							}
						}
					})
				}

				// Extract links if depth < maxDepth
				if job.Depth < job.Settings.Depth {
					doc.Find("a").Each(func(_ int, s *goquery.Selection) {
						href, exists := s.Attr("href")
						if exists {
							// Resolve relative URL
							absoluteURL, err := parsedURL.Parse(href)
							if err == nil {
								if absoluteURL.Scheme == "http" || absoluteURL.Scheme == "https" {
									// Strip fragments for cleaner deduplication
									absoluteURL.Fragment = ""
									urlStr := absoluteURL.String()

									if _, loaded := e.visited.LoadOrStore(urlStr, true); !loaded {
										childJob := Job{
											URL:      urlStr,
											Settings: job.Settings,
											Depth:    job.Depth + 1,
										}
										e.allJobsMu.Lock()
										e.allJobs[urlStr] = childJob
										e.allJobsMu.Unlock()
										e.jobStates.Store(urlStr, StatusQueued)

										select {
										case e.Jobs <- childJob:
										default:
											go func() {
												select {
												case <-e.ctx.Done():
												case e.Jobs <- childJob:
												}
											}()
										}
										e.sendDiscovered(urlStr)
									}
								}
							}
						}
					})
				}

				if htmlModified {
					if htmlStr, err := doc.Html(); err == nil {
						bodyBytes = []byte(htmlStr)
					}
				}
			}
		}

		err := os.MkdirAll(htmlDir, 0755)
		if err == nil {
			err = os.WriteFile(savePath, bodyBytes, 0600)
			if err == nil {
				e.sendFile(savePath)
			}
		}
	}
}

func (e *Engine) sendResult(res Result) {
	select {
	case <-e.ctx.Done():
	case e.Results <- res:
	default:
		go func() {
			select {
			case <-e.ctx.Done():
			case e.Results <- res:
			}
		}()
	}
}

func (e *Engine) sendFile(path string) {
	select {
	case <-e.ctx.Done():
	case e.Files <- path:
	default:
		go func() {
			select {
			case <-e.ctx.Done():
			case e.Files <- path:
			}
		}()
	}
}

func (e *Engine) sendDiscovered(u string) {
	select {
	case <-e.ctx.Done():
	case e.Discovered <- u:
	default:
		go func() {
			select {
			case <-e.ctx.Done():
			case e.Discovered <- u:
			}
		}()
	}
}
