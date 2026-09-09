# AGENTS.md — Chameleon Developer & AI Agent Operating Guide

Welcome to the **Chameleon** repository. This document serves as the authoritative guide for AI coding agents and human contributors working on Chameleon. It details the architecture, design principles, command workflows, concurrency patterns, and conventions required to safely and effectively extend the codebase.

---

## 1. Project Overview

**Chameleon** is a modern, concurrent, terminal-based web scraper and scrapability/ethical audit suite written in **Go 1.25**.

Key capabilities:
- **Terminal User Interface (TUI)**: Interactive dashboard built with [Charm Bubbletea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles). Features a **Tabbed Layout Architecture**:
  - **Tabbed Interface**: Focused tabbed view designed for all terminal viewports, featuring live item count badges, direct tab jumping (`1`-`4`, `[` / `]`), a dedicated 2-line bottom URL entry bar, and seamless focus toggling (`Tab` / `Esc`).
  - **Safeguards & UX Protections**: Inline accessible URL validation error banner, two-step stop confirmation safeguard (`[Confirm Stop? 's']`), and CPU-efficient idle spinner gating.
- **Enhanced Saved Files Explorer**: Interactive file manager with rich metadata tracking (`SavedFileEntry`: host, relative path, extension, categorized type badges, exact and compact byte sizes, timestamps), responsive layout (wide tabular view with breakdown metrics vs. compact list on narrow viewports), and keyboard navigation (`↑`/`↓`/`j`/`k`, `Home`/`g`, `End`/`G`).
- **Concurrent Scraping Engine**: Multi-worker asynchronous crawling pipeline with link extraction and asset rewriting powered by [Goquery](https://github.com/PuerkitoBio/goquery) and token-bucket rate limiting via `golang.org/x/time/rate`. Supports depth-bounded recursive crawling, single-job and global queue pause/resume/stop lifecycles, job retry capabilities, bounded response reading (10MB limit), and context-guarded non-blocking channel dispatching.
- **Scrapability & Ethical Audit Suite**: Pre-flight and on-demand heuristic analyzer evaluating RFC 9309 `robots.txt` compliance (longest prefix match with `Allow` override support), sitemaps, WAF/bot protections (Cloudflare, Akamai, CloudFront, DataDome, Imperva), Single Page Application (SPA) / CSR framework markers, honeypot traps, and passive rate-limit headers with actionable recommendations.
- **Accessibility & Reduced Motion Suite**: WCAG 2.1 AA compliant light theme (`theme.light.json5`), high-contrast 16 ANSI accessible theme (`theme.accessible.json5`), colorblind-distinct status tokens (Vibrant Orange for Stopped vs Red for Error), screen reader environment detection (`ACCESSIBILITY_ENABLED=1`), and reduced motion mode (`REDUCED_MOTION=1` or `NO_ANIMATIONS=1` replacing animated spinners with static text indicators).
- **Dynamic Asset Mirroring & Traversal Protection**: Saves scraped HTML and downloaded images to disk (`output/<host>/...`) with path traversal guards (`resolveSavePath`) while rewriting asset paths locally (`src` and `srcset` relative paths).
- **Theming System**: User-configurable ANSI and 256/hex color themes powered by JSON5 (`github.com/titanous/json5`).

---

## 2. Repository Layout & Architecture

```
chameleon/
├── cmd/
│   └── chameleon/
│       └── main.go                    # Application entrypoint: launches Engine and TUI
├── configs/
│   ├── theme.accessible.json5        # High-contrast 16-color ANSI accessible theme
│   ├── theme.example.json5           # Default dark JSON5 theme configuration
│   └── theme.light.json5             # WCAG 2.1 AA compliant light background theme
├── internal/
│   ├── engine/
│   │   ├── analyzer/
│   │   │   ├── analyzer.go           # Scrapability and ethical diagnostic audit engine
│   │   │   └── analyzer_test.go      # Tests for analyzer heuristics (friendly, protected, headless, robots)
│   │   ├── engine.go                 # Core scraping engine, workers, rate limiting, and disk writing
│   │   └── engine_test.go            # Tests for fetching, pause/resume/stop, path traversal, and image rewriting
│   └── tui/
│       ├── accessibility_test.go     # Tests for URL validation, stop confirmations, idle gating, and motion
│       ├── commands.go               # Bubbletea async channel listener commands with closed-channel protection
│       ├── layout.go                 # Tabbed layout dimension calculations
│       ├── panel_files.go            # Saved files panel, metadata parser, wide/narrow table rendering, cursor
│       ├── panel_files_test.go       # Unit tests for file explorer parsing, formatting, and rendering
│       ├── panel_queue.go            # Job queue list, status badges, stop confirmation, and cursor controls
│       ├── panel_report.go           # Scrapability & ethical report card viewport
│       ├── panel_telemetry.go        # Verbose request/response inspector view
│       ├── theme.go                  # JSON5 theme loading with fallback paths
│       ├── theme_test.go             # Theme parsing and default fallback tests
│       ├── tui.go                    # Model struct, enums (ActiveTab, FocusArea), focus helpers
│       ├── tui_test.go               # Comprehensive UI interaction, keybinding, and viewport tests
│       ├── update.go                 # Central Bubbletea Update() loop, key event routing, and window resize
│       └── view.go                   # Top-level View(), tab bar, content panels, URL bar, help modal, footer
├── scripts/                          # Diagnostic scripts and mock testing servers
│   ├── test_lipgloss.go              # Headless UI layout verification script
│   ├── test_server.py                # Python mock HTTP server for testing edge cases
│   └── test_url.go                   # Quick URL parsing verification script
├── Makefile                          # Build, run, test, and clean targets
├── go.mod                            # Go module definition (Go 1.25)
├── go.sum                            # Dependency lockfile
├── README.md                         # User documentation
└── AGENTS.md                         # This file (Agent operating manual)
```

### Architectural Separation of Concerns

1. **`internal/engine` is strictly decoupled from `internal/tui`**:
   - The engine must **never** import `bubbletea`, `lipgloss`, or any TUI types.
   - The engine communicates with the external world and UI strictly via Go channels (`Jobs`, `Results`, `Files`, `Discovered`, `Analysis`).
2. **`internal/engine/analyzer` is a modular diagnostic subsystem**:
   - Operates independently or as a component of `engine.Engine`.
   - Exposes a pluggable `HeadlessBrowser` interface for optional headless browser integration (e.g., Chromedp or Playwright).
3. **`internal/tui` manages state presentation and user inputs**:
   - Adheres to the Elm Architecture: `Model`, `Init()`, `Update(tea.Msg)`, `View()`.
   - **Presentation Pipeline**:
     - `View()` renders the top tab bar (`renderTabBar`), active tab content panel (`renderTabContentPanel`), bottom URL bar (`renderBottomURLBar`), and adaptive footer (`renderFooter`).
   - **Input Routing**:
     - Dispatches key events via `updateKey()`, handling focus toggling (`Tab`/`Esc`) between tab content and bottom URL bar, tab navigation (`1`-`4`, `[` / `]`), or routing to component-specific handlers (`updateQueueKey`, `updateTable`, `updateFilesKey`, `updateSettings`, `updateInput`).
   - **Asynchronous Event Loop**:
     - Manages background engine events by returning `tea.Cmd` listeners (`waitForResult`, `waitForFile`, `waitForDiscovered`, `waitForAnalysis`) that consume engine channels without blocking the Bubbletea main loop.

---

## 3. Essential Commands & Development Workflows

### Build & Run
```bash
# Run the application interactively
make run
# or
go run ./cmd/chameleon

# Compile the binary to build/chameleon
make build
# or
go build -o build/chameleon ./cmd/chameleon

# Clean build artifacts
make clean
```

### Testing & Verification
```bash
# Run all tests with verbose output
make test
# or
go test -v ./...

# Run tests for a specific package
go test -v ./internal/engine
go test -v ./internal/engine/analyzer
go test -v ./internal/tui

# Run specific unit test suites
go test -v -run TestEngineImageScrapingAndRewriting ./internal/engine
go test -v -run TestEnginePathTraversalProtection ./internal/engine
go test -v -run TestEngineBoundedReading ./internal/engine
go test -v -run TestEngineRetryStoppedJob ./internal/engine
go test -v -run TestAnalyzer_RobotsAllowOverride ./internal/engine/analyzer
go test -v -run TestAnalysisViewAndToggling ./internal/tui
go test -v -run TestTabNavigation ./internal/tui
go test -v -run TestFooterResponsiveTruncation ./internal/tui
go test -v -run TestSavedFilesTable ./internal/tui
go test -v -run TestURLInputValidationAccessibility ./internal/tui
go test -v -run TestQueueStopConfirmationSafeguard ./internal/tui
go test -v -run TestSpinnerIdleGatingAndReducedMotion ./internal/tui
go test -v -run TestAccessibleThemeTokens ./internal/tui
go test -v -run TestCommandsChannelClosed ./internal/tui

# Run tests with race detection (recommended when altering concurrency)
go test -race ./...

# Static analysis and verification
go vet ./...
make lint   # Runs golangci-lint if installed locally
```

### Headless Verification Scripts
When working in headless or containerized environments without an interactive terminal:
```bash
# Verify Lipgloss styling and panel layout without launching a TUI session
go run scripts/test_lipgloss.go

# Start the local Python mock server for edge-case HTTP response testing
python3 scripts/test_server.py

# Verify URL resolution and parsing logic
go run scripts/test_url.go
```

> [!IMPORTANT]
> **Headless / Non-Interactive Execution**:
> `make run` or `go run ./cmd/chameleon` initializes Bubbletea's alternate screen (`tea.WithAltScreen()`) and expects an interactive terminal. In headless AI agent environments, do **not** run `make run` as a long-running background command expecting TUI interaction. Always verify changes using `go test -v ./...` or isolated test scripts in `scripts/`.

---

## 4. Key Subsystem Mechanics

### 4.1. Scraping Engine (`internal/engine`)

- **Worker Pool**: `Engine.Start(workers)` launches background worker goroutines. The default in `main.go` is 5 workers.
- **Channels**:
  - `Jobs chan Job` (cap 1000): Inbound queue of crawl targets.
  - `Results chan Result` (cap 1000): Outbound stream of completed HTTP requests with status, timing, content type, headers, and error messages.
  - `Files chan string` (cap 1000): Outbound notifications of files written to `output/`.
  - `Discovered chan string` (cap 1000): Outbound notifications of discovered URLs found in parsed HTML.
  - `Analysis chan analyzer.Report` (cap 100): Outbound results of scrapability audits.
- **Job Lifecycle & States**:
  - `StatusQueued`: Pending worker pickup.
  - `StatusRunning`: Worker actively performing HTTP fetch.
  - `StatusPaused`: Suspended; worker context canceled if in-flight, job retained in `pausedJobs`.
  - `StatusStopped`: Aborted permanently; worker context canceled if in-flight, removed from active work.
  - `StatusDone`: Completed successfully.
  - `StatusError`: Encountered network, HTTP, or parsing failure.
  - **Job Retries**: `AddJob(rawURL, settings)` clears `visited` deduplication if the target URL was previously in `StatusError` or `StatusStopped`, allowing users to retry failed or stopped tasks.
- **Concurrency & State Synchronization**:
  - `jobStates sync.Map` (URL -> `JobStatus`): Lockless status lookups for UI queue badges.
  - `jobCancels sync.Map` (URL -> `context.CancelFunc`): Immediate abort of in-flight HTTP sockets upon pause or stop.
  - `pausedJobs map[string]Job` (guarded by `pausedMu sync.Mutex`): Preserves paused jobs for later resumption.
  - `allJobs map[string]Job` (guarded by `allJobsMu sync.Mutex`): Registry of all jobs for global pause/resume iteration.
  - `visited sync.Map`: Deduplication cache preventing duplicate crawling or asset downloads. URL fragments (`#hash`) are stripped before checking.
  - `Engine.HasActiveWork() bool`: Lockless helper checking whether any jobs in `jobStates` are in `StatusQueued` or `StatusRunning`, enabling the TUI to sleep idle spinners.
  - `sendResult`, `sendFile`, `sendDiscovered`: Context-guarded non-blocking channel senders with fallback goroutines bounded by `<-e.ctx.Done()` to prevent goroutine leaks upon engine shutdown.
- **Pause & Resume Controls**:
  - `TogglePauseJob(url)` / `PauseJob(url)` / `ResumeJob(url)`: Controls individual jobs.
  - `TogglePauseAll()` / `PauseAll()` / `ResumeAll()`: Controls all queued and running jobs across the engine.
  - `StopJob(url)`: Permanently cancels in-flight requests, removes the job from `pausedJobs`, and deletes the URL from `visited`.
- **Rate Limiting**: Uses `rate.Limiter` from `golang.org/x/time/rate`. `SpeedSafe` sets the interval to `1 req/sec`; `SpeedFast` sets it to `200ms` (5 req/sec).
- **Recursive Crawling & Depth Traversal**:
  - `Settings.Depth`: Configurable traversal depth (1 to 5 levels). Depth 1 crawls only the initial page.
  - If `job.Depth < job.Settings.Depth`, `doc.Find("a")` extracts links. Relative URLs are resolved against the page URL, checked for valid `http`/`https` scheme, deduplicated via `visited.LoadOrStore`, and enqueued as new child jobs with `job.Depth + 1`.
- **Asset Scraping & Relative Rewriting**:
  - If `job.Settings.Images` is true, `doc.Find("img")`, `<picture><source>`, and `<source>` elements are scanned for `src` and `srcset` attributes.
  - Image URLs are parsed, normalized (fragments stripped), deduplicated, and queued for download.
  - Attributes (`src`, `srcset`) are rewritten locally using `filepath.Rel(htmlDir, imgSavePath)` and converted to web-safe forward slashes with `filepath.ToSlash`.
- **Disk Storage Hierarchy & Path Traversal Security**:
  - `resolveSavePath(u *url.URL, isImage bool)` protects against directory traversal:
    - Strips directory traversal segments (`..`), slashes, and leading/trailing dots/spaces from hostnames.
    - Normalizes URL paths with `path.Clean("/" + u.Path)` and forces paths to remain within `output/<host>/`.
    - Automatically resolves trailing slashes or empty paths to `index.html` (or `image` for images).
    - Resolves filesystem conflicts where a directory name matches an existing file by renaming to `<path>.file`.
  - Directories are created with `0755` permissions; files are written with `0600`.
- **Bounded Response Body Reading**:
  - Protects against memory exhaustion attacks by bounding body reads with `io.LimitReader(resp.Body, 10*1024*1024)` (10MB limit) before parsing HTML or writing to disk.

### 4.2. Scrapability & Ethical Analyzer (`internal/engine/analyzer`)

The analyzer performs multi-stage passive diagnostics on a target URL:
1. **Robots & Sitemap (RFC 9309 Compliant)**: Fetches `<scheme>://<host>/robots.txt` (bounded 512KB read). Evaluates rules for both wildcard (`*`) and Chameleon-specific (`chameleon`) agents. Implements **longest prefix matching** between `Disallow` and `Allow` directives: if an `Allow` rule has a path length greater than or equal to a matching `Disallow` rule, access is permitted and recorded with positive ethical scoring. Evaluates crawl delays and sitemaps.
2. **Target Inspection**: Performs a bounded 5MB GET request with desktop User-Agent (`Mozilla/5.0 (compatible; ChameleonBot/1.0)`).
3. **Header Inspection & Bot Defenses**: Identifies bot defenses and WAFs via status codes (403, 429, 503) and response headers:
   - Cloudflare (`cf-ray`, `cf-mitigated`, `Server: cloudflare`)
   - Akamai (`Server: akamaighost`, `X-Akamai-Transformed`)
   - AWS CloudFront (`X-Amz-Cf-Id`, `cloudfront`)
   - DataDome (`X-DataDome`, `X-DataDome-CID`)
   - Imperva / Incapsula (`X-Icdn`, `incapsula`)
   - Passive rate-limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `Retry-After`)
4. **DOM & Content Inspection**:
   - Zero-copy parsing via `bytes.NewReader(bodyBytes)` to prevent redundant memory allocation.
   - SPA framework markers (`#root`, `#app`, `#__next`, `#__nuxt`, `ng-version`).
   - Class obfuscation using precompiled package regexes (`classHashRegex`, `hexHashRegex`) to prevent regex recompilation overhead in tight loops.
   - Hidden honeypot traps (`display:none`, `visibility:hidden`, `opacity:0`, zero dimension links).
   - Meta robots tags (`noindex`, `nofollow`, `noarchive`).
   - Structured JSON-LD metadata.
5. **Pluggable Headless Engine**:
   - Implements `HeadlessBrowser` interface (`CheckPage(ctx, targetURL) (*HeadlessResult, error)`).
   - Allows drop-in dynamic DOM mutation analysis, network call counts, canvas fingerprint detection, and JS execution checks.
6. **Scoring & Grading**:
   - `EthicalScore` (0–100) -> `EthicalGrade` (A, B, C, D, F).
   - `DifficultyScore` (1–10) -> `DifficultyLevel` (Easy, Moderate, Hard, Extreme).
   - Generates actionable scraping recommendations (e.g. rate-limit compliance, headless requirement warnings).

### 4.3. Terminal User Interface (`internal/tui`)

#### Tabbed UI Architecture
- Structure:
  - **Top Tab Bar (1 line)**: Tab pills with active highlights and dynamic item count badges (`1: Queue (N)`, `2: Telemetry (N)`, `3: Files (N)`, `4: Settings`) plus a right-aligned `[?: Help]` badge.
  - **Active Tab Content Panel**: Displays the focused tab's viewport, table, or settings within a bordered container.
  - **Bottom URL Bar (2 lines)**: Dedicated persistent entry bar (Line 1: `─── Target URL ───`, Line 2: ` ❯ <input>`).
  - **Footer (1 line)**: Adaptive contextual action bar with mode badge, context actions, and global controls.

#### Tabbed Mode Focus Management (`FocusArea` enum)
- **`FocusTabContent` (0)**: Focus is on the active tab's content (viewport, table, or settings list).
- **`FocusURLInput` (1)**: Focus is on the bottom 2-line Target URL input prompt.
- **Switching Focus**:
  - Pressing `Tab` / `Shift+Tab` toggles focus between the active tab content and the bottom URL bar.
  - Pressing `Esc` inside the URL bar immediately returns focus to the active tab.

#### Center Panel View Modes (`CenterViewMode` enum)
Active within Tab 2 (Telemetry):
- **`CenterViewTelemetry` (0)**: Live HTTP telemetry table with spinning loader, URL/name, status, content-type, size, and response latency.
- **`CenterViewReport` (1)**: Scrapability & Ethical Report Card displaying ethical grades, difficulty meters, policy audits, technical anti-bot findings, and recommendations.
- **`CenterViewVerbose` (2)**: Full request/response inspector displaying HTTP method, response headers, status codes, timestamps, and error details.

#### Enhanced Saved Files Explorer (`panel_files.go`)
- **Metadata Extraction (`SavedFileEntry`)**: Extracts host, relative path, file extension, typed classification (`HTML`, `PNG`, `JPG`, `WEBP`, `SVG`, `GIF`, `CSS`, `JS`, `JSON`, `TXT`, `FILE`), formatted file sizes (`formatBytes`, `formatBytesCompact`), and modification timestamps.
- **Dual-Mode Presentation**:
  - **Wide Table Mode (`vw >= 65`)**: Tabular layout with structured headers (`#`, `TYPE`, `HOST`, `PATH`, `SIZE`, `SAVED`), aggregate summary metrics header (`📂 X Files (Y) ─── 📄 A HTML  🖼️ B Images  📦 C Other`), and active selection indicator (`▶`).
  - **Narrow Card Mode (`vw < 65`)**: Compact card view designed for narrow viewports showing type badges, truncated filenames, and compact size notations.
- **Interactive Navigation & Auto-Scroll**:
  - `↑` / `k` (move up), `↓` / `j` (move down), `Home` / `g` (jump to top), `End` / `G` (jump to bottom).
  - Automatically synchronizes viewport `YOffset` to ensure the highlighted file remains within visible boundaries.

#### User Input Validation & Safeguards
- **URL Input Validation**: Validates URL structure, scheme, and hostname before submitting crawl jobs or audits. Displays an accessible inline error message `⚠️ Please enter a valid URL (e.g. example.com)` that automatically clears upon user keystrokes.
- **Two-Step Stop Confirmation Safeguard**: Stopping an active or queued job requires deliberate confirmation. Pressing `s` / `d` / `Del` enters confirmation mode with badge `[Confirm Stop? 's']` and contextual footer prompts (`[s] Confirm Stop • [Esc] Cancel`). Pressing `s` a second time confirms termination; pressing `Esc` or moving cursor cancels.

#### Accessibility & Motion System
- **Environment Flags**:
  - `REDUCED_MOTION=1` or `NO_ANIMATIONS=1`: Replaces animated spinner ticks with static `[Active]` / `[Idle]` text indicators.
- **Idle-Gated Spinner**: Halts spinner tick loop when `!m.hasActiveWork()` to eliminate idle CPU consumption. Re-activates automatically upon receiving new jobs or starting audits via `m.startSpinnerCmd()`.
- **Tab Bar Inset Alignment**: Top tab bar header incorporates 1-character left/right padding to align with the panel borders and content canvas.

#### Key Navigation Reference

| Keybinding | Context | Action |
| :--- | :--- | :--- |
| `?` / `F1` | Global | Open modal help overlay |
| `Ctrl+C` | Global | Quit application |
| `1`, `2`, `3`, `4` | Tab Focused | Jump directly to Tab (Queue, Telemetry, Files, Settings) |
| `[` / `]` | Tab Focused | Cycle to previous / next tab (wraps around) |
| `Tab` / `Shift+Tab` | Global | Toggle focus between active tab content and bottom URL bar |
| `Esc` | URL Bar Focused | Return focus from URL bar to active tab content |
| `Enter` | URL Bar Focused | Submit target URL to crawling engine |
| `Ctrl+A` / `F2` | URL Bar Focused | Trigger scrapability & ethical audit on URL |
| `Ctrl+A` / `F2` | Telemetry Tab | Toggle between Telemetry table and Ethical Report Card |
| `Space` / `p` / `Enter` | Queue Tab | Pause / resume selected job |
| `P` (`Shift+P`) | Queue Tab | Pause / resume all queued and running jobs |
| `s` / `d` / `Del` / `Backspace` | Queue Tab | Prompt stop confirmation (`[Confirm Stop? 's']`) |
| `s` | Queue (confirmation active) | Confirm stop and remove job from queue |
| `Esc` | Queue (confirmation active) | Cancel stop confirmation |
| `Enter` / `i` / `v` | Telemetry Tab | Inspect selected request headers and verbose details |
| `Esc` / `q` / `backspace` | Request Details | Return to live telemetry table |
| `a` / `t` / `Space` | Telemetry / Details | View scrapability & ethical report card |
| `↑` / `↓` or `j` / `k` | Queue / Report | Navigate or scroll viewport content |
| `↑` / `↓` or `j` / `k` | Files Tab | Navigate file entries |
| `Home` / `g` | Files Tab | Jump to first file entry |
| `End` / `G` | Files Tab | Jump to last file entry |
| `↑` / `↓` or `j` / `k` | Settings Tab | Select setting (Depth, Images, Speed) |
| `←` / `→` / `Space` / `Enter` | Settings Tab | Adjust or toggle selected setting value |

#### Responsive Contextual Footer Architecture
Implemented in `renderFooter`:
- Displays an active mode pill badge (e.g. `TARGET URL`, `TAB 1: QUEUE`, `TAB 2: TELEMETRY`, `REQUEST DETAILS`, `SETTINGS`).
- Context actions relevant to the focused panel/tab are displayed on the left; global actions are displayed on the right.
- **Progressive Collapsing Logic (`buildLine`)**:
  1. Attempts to render full context actions with all global actions.
  2. If horizontal width is constrained, progressively drops global actions.
  3. If still constrained, progressively sheds secondary context actions.
  4. Collapses to just the mode pill on extremely narrow terminals to prevent line wrapping.

#### Layout Dimension Calculations (`calculateLayout`)
Implemented in `internal/tui/layout.go`:
- Clamps minimum terminal constraints to **40 columns × 16 rows**.
- `TabHeaderHeight = 1` (tab pills line)
- `URLBarHeight = 2` (top border title + prompt line)
- `FooterHeight = 1`
- `TabContentHeight = height - TabHeaderHeight - URLBarHeight - FooterHeight` (clamped to min 6)
- `TabContentWidth = width`

### 4.4. Theming (`internal/tui/theme.go`)

- Loads configuration in JSON5 format (supporting comments, trailing commas, and unquoted keys).
- Pre-configured Themes in `configs/`:
  - `configs/theme.example.json5`: Default dark theme.
  - `configs/theme.accessible.json5`: High-contrast accessible theme utilizing standard 16 ANSI colors for maximum visibility and screen-reader compatibility.
  - `configs/theme.light.json5`: Light theme optimized specifically for white/light terminal backgrounds adhering to WCAG 2.1 AA (contrast >= 4.5:1 against `#ffffff`).
- Search precedence:
  1. Explicitly passed custom paths
  2. `./theme.json5` or `./theme.json`
  3. `$XDG_CONFIG_HOME/chameleon/theme.json5` (or `~/.config/chameleon/theme.json5`)
  4. `~/.chameleon/theme.json5`
  5. `DefaultTheme()`
- Default Color Tokens:
  - Panels & Borders: `BorderActive` (`39`), `BorderInactive` (`240`), `TitleActive` (`39`), `TitleInactive` (`245`)
  - Table: `TableHeaderBorder` (`240`), `TableSelectedFg` (`229`), `TableSelectedBg` (`57`)
  - Accent: `AccentColor` (`39`)
  - Footer: `FooterKey` (`39`), `FooterDesc` (`244`), `FooterSep` (`246`)
  - Job Status Badges: `StatusRunning` (`39`), `StatusPaused` (`220`), `StatusStopped` (`208`), `StatusDone` (`46`), `StatusError` (`196`), `StatusQueued` (`244`)
  *(Note: `StatusStopped` uses Vibrant Orange `208` to distinguish it from `StatusError` Red `196` for colorblind accessibility).*

---

## 5. Coding Standards & Guidelines for AI Agents

When contributing or refactoring code in Chameleon, you must adhere to the following rules:

### Go Style & Concurrency Rules
1. **Concurrency Safety**:
   - Never access non-concurrent maps without proper mutex locks.
   - Use `sync.Map` for read-heavy key-value caches (`visited`, `jobStates`, `jobCancels`).
   - Use `sync.Mutex` for collections with composite operations (`pausedJobs`, `allJobs`).
   - Prevent goroutine leaks: every spawned goroutine must have a clean exit path via `context.Context` or bounded channel reads.
2. **Non-Blocking Channel Operations & Safe Senders**:
   - When emitting to buffered channels that may experience backpressure from the UI, use context-guarded non-blocking sends with fallbacks:
     ```go
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
     ```
3. **Safe Channel Consumption**:
   - When consuming engine channels in Bubbletea commands, always check the `ok` channel closure boolean to prevent panic or infinite message spinning on engine shutdown:
     ```go
     res, ok := <-c
     if !ok {
         return nil
     }
     return engineResultMsg(res)
     ```
4. **Resource Management & Bounded Reads**:
   - Always close HTTP response bodies (`defer resp.Body.Close()`).
   - Bounded reading: Never use unbounded `io.ReadAll(resp.Body)` on untrusted public web pages. Use `io.LimitReader` (e.g. 10MB for crawler responses, 5MB for analyzer HTML, 512KB for robots.txt) to protect against memory exhaustion.
5. **File System Operations & Path Traversal Protection**:
   - Always sanitize and resolve output paths via `resolveSavePath(u, isImage)`. Verify target file paths remain strictly contained within `output/<host>/`.
   - Use `0755` for directories (`os.MkdirAll`) and `0600` for created files (`os.WriteFile`).
   - Always clean up test files in `defer` hooks: `defer os.RemoveAll("output")`.

### Bubbletea & Lipgloss Rules
1. **Pure `Update()` Function**:
   - Do not perform network calls, long sleeps, or disk I/O directly inside `Update()`.
   - Dispatch background work using `tea.Cmd`.
2. **Unified Tabbed Architecture**:
   - The UI is unified around the tabbed layout. New tabs or views should integrate into `ActiveTab`, `updateKey()`, and `renderTabContentPanel()`.
   - Maintain accurate viewport sizing in `syncDimensions()`.
3. **Dimension & Boundary Calculation**:
   - In Lipgloss, a border adds `+2` characters to the width and `+2` lines to the height.
   - Inner viewports must have width `w - 2` and height `h - 2` (or less if headers or footers are rendered inside the border).
   - In `tea.WindowSizeMsg`, always update viewport widths/heights, table column widths, and input box widths.
4. **Responsive Footer Integrity**:
   - Never render a static footer string wider than `m.width`.
   - When adding footer shortcuts, integrate them into `contextActions` or `globalActions` in `footerItems()` to leverage progressive collapsing.
5. **Rate-Limited Viewport Re-Rendering & Idle Gating**:
   - Set dirty flags (`needsQueueUpdate`, `needsFilesUpdate`) on high-frequency channel messages and only recompute viewport text strings during `spinner.TickMsg` ticks to prevent CPU churn.
   - Stop spinner animation ticks when the engine has no active jobs (`!m.hasActiveWork()`), restarting ticks via `m.startSpinnerCmd()` on new activity.
6. **Accessibility & Reduced Motion**:
   - Check `m.reducedMotion` before starting spinner animations. When active, display static state text (e.g. `[Active]` / `[Idle]`).
   - Provide clear, accessible error messaging on input validation failures instead of silently ignoring invalid input.
   - Protect destructive actions with two-step confirmation safeguards.
7. **Color Compatibility**:
   - Use theme token properties (`m.theme.BorderActive`, etc.) rather than hardcoded ANSI strings.
   - Support both ANSI 256 strings (`"39"`) and 24-bit hex strings (`"#00d7ff"`). Ensure contrast ratio is considered for colorblind and light-theme users.

---

## 6. Verification Checklist for Agents

Before completing any task, ensure the following steps pass:
- [ ] Code builds without errors: `go build ./...`
- [ ] All unit and integration tests pass: `go test -v ./...`
- [ ] Tabbed UI tests pass: `go test -v -run TestTabbed ./internal/tui` and `go test -v -run TestTabNavigation ./internal/tui`
- [ ] Responsive footer test passes: `go test -v -run TestFooter ./internal/tui`
- [ ] Saved files explorer tests pass: `go test -v -run TestSavedFiles ./internal/tui`
- [ ] Accessibility & safeguard tests pass: `go test -v -run TestURLInputValidationAccessibility ./internal/tui` and `go test -v -run TestQueueStopConfirmationSafeguard ./internal/tui`
- [ ] Path traversal security tests pass: `go test -v -run TestEnginePathTraversalProtection ./internal/engine`
- [ ] Channel resilience tests pass: `go test -v -run TestCommandsChannelClosed ./internal/tui`
- [ ] No concurrency race conditions: `go test -race ./...`
- [ ] Code is formatted with standard Go style: `gofmt -s -w .`
- [ ] Static analysis passes: `go vet ./...`
- [ ] Any created temporary files or `output/` directories in tests are cleaned up.
- [ ] No unhandled errors or silently ignored error returns in new functionality.
