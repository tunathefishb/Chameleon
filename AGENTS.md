# AGENTS.md — Chameleon Developer & AI Agent Operating Guide

Welcome to the **Chameleon** repository. This document serves as the authoritative guide for AI coding agents and human contributors working on Chameleon. It details the architecture, design principles, command workflows, concurrency patterns, and conventions required to safely and effectively extend the codebase.

---

## 1. Project Overview

**Chameleon** is a modern, concurrent, terminal-based web scraper and scrapability/ethical audit suite written in **Go 1.25**.

Key capabilities:
- **Terminal User Interface (TUI)**: Interactive dashboard built with [Charm Bubbletea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles). Features a **Dual-Mode Layout Architecture**:
  - **Grid Mode**: Multi-panel dashboard for standard/wide displays showing the Jobs Queue, Center View (Telemetry / Report / Details), Saved Files, URL Input, and Engine Settings simultaneously.
  - **Tabbed Mode**: Compact, focused single-panel tabbed view designed for smaller terminal viewports or distraction-free workflows, featuring live item count badges, a dedicated 2-line bottom URL entry bar, and fast tab jumping.
  - **Responsive Auto-Detection & Manual Toggle**: Automatically switches between Grid and Tabbed modes based on terminal window dimensions, with persistent manual override via `Ctrl+T` or `F3`.
- **Concurrent Scraping Engine**: Multi-worker asynchronous crawling pipeline with link extraction and asset rewriting powered by [Goquery](https://github.com/PuerkitoBio/goquery) and token-bucket rate limiting via `golang.org/x/time/rate`. Supports depth-bounded recursive crawling, single-job and global queue pause/resume/stop lifecycles, and in-flight request socket cancellation.
- **Scrapability & Ethical Audit Suite**: Pre-flight and on-demand heuristic analyzer evaluating `robots.txt`, sitemaps, WAF/bot protections (Cloudflare, Akamai, CloudFront, DataDome, Imperva), Single Page Application (SPA) / CSR framework markers, honeypot traps, and passive rate-limit headers with actionable recommendations.
- **Dynamic Asset Mirroring**: Saves scraped HTML and downloaded images to disk (`output/<host>/...`) while rewriting asset paths locally (`src` and `srcset` relative paths).
- **Theming System**: User-configurable ANSI and 256/hex color themes powered by JSON5 (`github.com/titanous/json5`).

---

## 2. Repository Layout & Architecture

```
chameleon/
├── cmd/
│   └── chameleon/
│       └── main.go                    # Application entrypoint: launches Engine and TUI
├── configs/
│   └── theme.example.json5           # Example JSON5 theme configuration
├── internal/
│   ├── engine/
│   │   ├── analyzer/
│   │   │   ├── analyzer.go           # Scrapability and ethical diagnostic audit engine
│   │   │   └── analyzer_test.go      # Tests for analyzer heuristics (friendly, protected, headless)
│   │   ├── engine.go                 # Core scraping engine, workers, rate limiting, and disk writing
│   │   └── engine_test.go            # Tests for fetching, pause/resume/stop, and image rewriting
│   └── tui/
│       ├── commands.go               # Bubbletea async channel listener commands
│       ├── layout.go                 # Responsive grid and tabbed layout dimension calculations
│       ├── panel_files.go            # Saved files panel viewport rendering
│       ├── panel_queue.go            # Job queue list, status badges, and cursor controls
│       ├── panel_report.go           # Scrapability & ethical report card viewport
│       ├── panel_telemetry.go        # Verbose request/response inspector view
│       ├── tab_test.go               # Unit tests for tab navigation, responsive layout switching, and focus
│       ├── tab_update.go             # Key handling and input routing for tabbed UI mode
│       ├── tab_view.go               # Tab bar, tab content panels, bottom URL bar, and tabbed footer
│       ├── theme.go                  # JSON5 theme loading with fallback paths
│       ├── theme_test.go             # Theme parsing and default fallback tests
│       ├── tui.go                    # Model struct, enums (Panel, UIMode, ActiveTab, FocusArea), focus helpers
│       ├── tui_test.go               # Comprehensive UI interaction, keybinding, and viewport tests
│       ├── update.go                 # Central Bubbletea Update() loop, window resize & mode switching
│       └── view.go                   # Top-level Grid View(), help modal, and responsive contextual footer
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
   - **Dual-Layout Presentation Pipeline**:
     - When `m.uiMode == UIModeTabbed`, `View()` delegates to `renderTabbedView()` (`tab_view.go`), rendering the top tab bar (`renderTabBar`), active tab content panel (`renderTabContentPanel`), bottom URL bar (`renderBottomURLBar`), and adaptive footer (`renderTabbedFooter`).
     - When `m.uiMode == UIModeGrid`, `View()` renders the 5-panel split dashboard (`view.go`).
   - **Input Routing**:
     - Dispatches key events to `updateTabbedKey()` when in `UIModeTabbed`, or routes to panel-specific handlers (`updateInput`, `updateQueueKey`, `updateTable`, `updateSettings`) in `UIModeGrid`.
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
go test -v -run TestAnalysisViewAndToggling ./internal/tui
go test -v -run TestUIModeAutoDetection ./internal/tui
go test -v -run TestTabbedNavigation ./internal/tui
go test -v -run TestFooterResponsiveTruncation ./internal/tui

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
- **Concurrency & State Synchronization**:
  - `jobStates sync.Map` (URL -> `JobStatus`): Lockless status lookups for UI queue badges.
  - `jobCancels sync.Map` (URL -> `context.CancelFunc`): Immediate abort of in-flight HTTP sockets upon pause or stop.
  - `pausedJobs map[string]Job` (guarded by `pausedMu sync.Mutex`): Preserves paused jobs for later resumption.
  - `allJobs map[string]Job` (guarded by `allJobsMu sync.Mutex`): Registry of all jobs for global pause/resume iteration.
  - `visited sync.Map`: Deduplication cache preventing duplicate crawling or asset downloads. URL fragments (`#hash`) are stripped before checking.
- **Pause & Resume Controls**:
  - `TogglePauseJob(url)` / `PauseJob(url)` / `ResumeJob(url)`: Controls individual jobs.
  - `TogglePauseAll()` / `PauseAll()` / `ResumeAll()`: Controls all queued and running jobs across the engine.
  - `StopJob(url)`: Permanently cancels in-flight requests and removes the job from `pausedJobs`.
- **Rate Limiting**: Uses `rate.Limiter` from `golang.org/x/time/rate`. `SpeedSafe` sets the interval to `1 req/sec`; `SpeedFast` sets it to `200ms` (5 req/sec).
- **Recursive Crawling & Depth Traversal**:
  - `Settings.Depth`: Configurable traversal depth (1 to 5 levels). Depth 1 crawls only the initial page.
  - If `job.Depth < job.Settings.Depth`, `doc.Find("a")` extracts links. Relative URLs are resolved against the page URL, checked for valid `http`/`https` scheme, deduplicated via `visited.LoadOrStore`, and enqueued as new child jobs with `job.Depth + 1`.
- **Asset Scraping & Relative Rewriting**:
  - If `job.Settings.Images` is true, `doc.Find("img")`, `<picture><source>`, and `<source>` elements are scanned for `src` and `srcset` attributes.
  - Image URLs are parsed, normalized (fragments stripped), deduplicated, and queued for download.
  - Attributes (`src`, `srcset`) are rewritten locally using `filepath.Rel(htmlDir, imgSavePath)` and converted to web-safe forward slashes with `filepath.ToSlash`.
- **Disk Storage Hierarchy**:
  - Files are written to `output/<host>/<path>`.
  - URLs with trailing slashes or empty paths default to `index.html`. Images without explicit filename extensions default to `image`.
  - Directories are created with `0755` permissions; files are written with `0600`.

### 4.2. Scrapability & Ethical Analyzer (`internal/engine/analyzer`)

The analyzer performs multi-stage passive diagnostics on a target URL:
1. **Robots & Sitemap**: Fetches `<scheme>://<host>/robots.txt` (bounded 512KB read). Evaluates wildcard rules (`*`), crawl delays, path disallow rules, and sitemap entries.
2. **Target Inspection**: Performs a bounded 5MB GET request with desktop User-Agent (`Mozilla/5.0 (compatible; ChameleonBot/1.0)`).
3. **Header Inspection & Bot Defenses**: Identifies bot defenses and WAFs via status codes (403, 429, 503) and response headers:
   - Cloudflare (`cf-ray`, `cf-mitigated`, `Server: cloudflare`)
   - Akamai (`Server: akamaighost`, `X-Akamai-Transformed`)
   - AWS CloudFront (`X-Amz-Cf-Id`, `cloudfront`)
   - DataDome (`X-DataDome`, `X-DataDome-CID`)
   - Imperva / Incapsula (`X-Icdn`, `incapsula`)
   - Passive rate-limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `Retry-After`)
4. **DOM & Content Inspection**:
   - SPA framework markers (`#root`, `#app`, `#__next`, `#__nuxt`, `ng-version`).
   - Class obfuscation (regex-based detection of hashed and styled-component class names).
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

#### Dual UI Layout Modes (`UIMode` enum)
- **`UIModeGrid` (0)**:
  - Default multi-panel dashboard for standard and wide terminal displays.
  - Split layout displaying:
    - **Top Row**: Left (25% Jobs Queue), Center (50% Telemetry/Report/Details), Right (25% Saved Files).
    - **Bottom Row**: Left (50% URL Input), Right (50% Engine Settings).
    - **Footer**: 1-line contextual action bar.
  - Focus is cycled using `Tab` / `Shift+Tab` through `PanelInput` (0), `PanelQueue` (1), `PanelTable` (2), `PanelFiles` (3), and `PanelSettings` (4).
- **`UIModeTabbed` (1)**:
  - Compact single-panel layout tailored for narrow viewports (< 120 cols), short viewports (< 28 rows), or focused workflows.
  - Structure:
    - **Top Tab Bar (2 lines)**: Tab pills with active highlights and dynamic item count badges (`1: Queue (N)`, `2: Telemetry (N)`, `3: Files (N)`, `4: Settings`) plus a right-aligned `[Ctrl+T: Grid]` mode pill.
    - **Active Tab Content Panel**: Displays the focused tab's viewport or table within a bordered container.
    - **Bottom URL Bar (2 lines)**: Dedicated persistent entry bar (Line 1: `─── Target URL ───`, Line 2: ` ❯ <input>`).
    - **Footer (1 line)**: Adaptive contextual action bar.
- **Responsive Auto-Detection & Manual Override**:
  - In `Update(tea.WindowSizeMsg)`, the UI automatically sets `UIModeTabbed` if `width < 120` or `height < 28`, and `UIModeGrid` otherwise.
  - Pressing `Ctrl+T` or `F3` manually toggles the mode and sets `m.manualUIMode = true`, ensuring window resize events will not override user preference.

#### Tabbed Mode Focus Management (`FocusArea` enum)
- **`FocusTabContent` (0)**: Focus is on the active tab's content (viewport, table, or settings list).
- **`FocusURLInput` (1)**: Focus is on the bottom 2-line Target URL input prompt.
- **Switching Focus**:
  - Pressing `Tab` toggles focus between the active tab content and the bottom URL bar.
  - Pressing `Esc` inside the URL bar immediately returns focus to the active tab.

#### Center Panel View Modes (`CenterViewMode` enum)
Active across both Grid mode (Center panel) and Tabbed mode (Tab 2: Telemetry):
- **`CenterViewTelemetry` (0)**: Live HTTP telemetry table with spinning loader, URL/name, status, content-type, size, and response latency.
- **`CenterViewReport` (1)**: Scrapability & Ethical Report Card displaying ethical grades, difficulty meters, policy audits, technical anti-bot findings, and recommendations.
- **`CenterViewVerbose` (2)**: Full request/response inspector displaying HTTP method, response headers, status codes, timestamps, and error details.

#### Key Navigation Reference

| Keybinding | Context | Action |
| :--- | :--- | :--- |
| `Ctrl+T` / `F3` | Global | Toggle between **Grid** and **Tabbed** UI layouts |
| `?` / `F1` | Global | Open modal help overlay |
| `Ctrl+C` | Global | Quit application |
| `Tab` / `Shift+Tab` | Grid Mode | Cycle focus forward / backward through all 5 panels |
| `1`, `2`, `3`, `4` | Tabbed Mode (Tab focused) | Jump directly to Tab (Queue, Telemetry, Files, Settings) |
| `[` / `]` | Tabbed Mode (Tab focused) | Cycle to previous / next tab (wraps around) |
| `Tab` | Tabbed Mode | Toggle focus between active tab content and bottom URL bar |
| `Esc` | Tabbed Mode (URL focused) | Return focus from URL bar to active tab content |
| `Enter` | Input / URL bar | Submit target URL to crawling engine |
| `Ctrl+A` / `F2` | Input / URL bar | Trigger scrapability & ethical audit on URL |
| `Ctrl+A` / `F2` | Telemetry / Report | Toggle between Telemetry table and Ethical Report Card |
| `Space` / `p` / `Enter` | Queue (focused) | Pause / resume selected job |
| `P` (`Shift+P`) | Queue (focused) | Pause / resume all queued and running jobs |
| `s` / `d` / `Del` / `Backspace` | Queue (focused) | Stop and permanently remove selected job |
| `Enter` / `i` / `v` | Telemetry (focused) | Inspect selected request headers and verbose details |
| `Esc` / `q` / `backspace` | Request Details | Return to live telemetry table |
| `a` / `t` / `Space` | Telemetry / Details | View scrapability & ethical report card |
| `↑` / `↓` or `j` / `k` | Queue / Files / Report | Navigate or scroll viewport content |
| `↑` / `↓` or `j` / `k` | Settings (focused) | Select setting (Depth, Images, Speed) |
| `←` / `→` / `Space` / `Enter` | Settings (focused) | Adjust or toggle selected setting value |

#### Responsive Contextual Footer Architecture
Both Grid mode (`renderFooter`) and Tabbed mode (`renderTabbedFooter`) implement an adaptive layout algorithm:
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
- **Grid Layout**:
  - `BottomHeight = 6`, `FooterHeight = 1`
  - `TopHeight = height - BottomHeight - FooterHeight` (clamped to min 8)
  - `LeftWidth = width / 4`, `RightWidth = width / 4`, `CenterWidth = width - LeftWidth - RightWidth`
  - `BottomLeftWidth = width / 2`, `BottomRightWidth = width - BottomLeftWidth`
- **Tabbed Layout**:
  - `TabHeaderHeight = 1` (tab pills line)
  - `URLBarHeight = 2` (top border title + prompt line)
  - `FooterHeight = 1`
  - `TabContentHeight = height - TabHeaderHeight - URLBarHeight - FooterHeight` (clamped to min 6)
  - `TabContentWidth = width`

### 4.4. Theming (`internal/tui/theme.go`)

- Loads configuration in JSON5 format (supporting comments, trailing commas, and unquoted keys).
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
  - Footer: `FooterKey` (`39`), `FooterDesc` (`244`), `FooterSep` (`238`)
  - Job Status Badges: `StatusRunning` (`39`), `StatusPaused` (`220`), `StatusStopped` (`196`), `StatusDone` (`46`), `StatusError` (`196`), `StatusQueued` (`244`)

---

## 5. Coding Standards & Guidelines for AI Agents

When contributing or refactoring code in Chameleon, you must adhere to the following rules:

### Go Style & Concurrency Rules
1. **Concurrency Safety**:
   - Never access non-concurrent maps without proper mutex locks.
   - Use `sync.Map` for read-heavy key-value caches (`visited`, `jobStates`, `jobCancels`).
   - Use `sync.Mutex` for collections with composite operations (`pausedJobs`, `allJobs`).
   - Prevent goroutine leaks: every spawned goroutine must have a clean exit path via `context.Context` or bounded channel reads.
2. **Non-Blocking Channel Operations**:
   - When emitting to buffered channels that may experience backpressure from the UI, use non-blocking sends with fallbacks:
     ```go
     select {
     case e.Jobs <- job:
     default:
         go func() { e.Jobs <- job }()
     }
     ```
3. **Resource Management**:
   - Always close HTTP response bodies (`defer resp.Body.Close()`).
   - Bounded reading: Never use unbounded `io.ReadAll(resp.Body)` on untrusted public web pages. Use `io.LimitReader` (e.g. 5MB for HTML, 512KB for robots.txt) to protect against memory exhaustion.
4. **File System Operations**:
   - Use `0755` for directories (`os.MkdirAll`) and `0600` for created files (`os.WriteFile`).
   - Always clean up test files in `defer` hooks: `defer os.RemoveAll("output")`.

### Bubbletea & Lipgloss Rules
1. **Pure `Update()` Function**:
   - Do not perform network calls, long sleeps, or disk I/O directly inside `Update()`.
   - Dispatch background work using `tea.Cmd`.
2. **Dual-Layout Symmetry**:
   - When adding new panels, settings, or interactive states, always implement handlers in **both** `UIModeGrid` (`update.go`, `view.go`) and `UIModeTabbed` (`tab_update.go`, `tab_view.go`).
   - Maintain synchronized viewport sizing in `syncDimensions()` for both Grid and Tabbed dimensions.
3. **Dimension & Boundary Calculation**:
   - In Lipgloss, a border adds `+2` characters to the width and `+2` lines to the height.
   - Inner viewports must have width `w - 2` and height `h - 2` (or less if headers or footers are rendered inside the border).
   - In `tea.WindowSizeMsg`, always update viewport widths/heights, table column widths, and input box widths.
4. **Responsive Footer Integrity**:
   - Never render a static footer string wider than `m.width`.
   - When adding footer shortcuts, integrate them into `contextActions` or `globalActions` in `contextFooterItems()` / `tabbedFooterItems()` to leverage progressive collapsing.
5. **Rate-Limited Viewport Re-Rendering**:
   - Set dirty flags (`needsQueueUpdate`, `needsFilesUpdate`) on high-frequency channel messages and only recompute viewport text strings during `spinner.TickMsg` ticks to prevent CPU churn.
6. **Color Compatibility**:
   - Use theme token properties (`m.theme.BorderActive`, etc.) rather than hardcoded ANSI strings.
   - Support both ANSI 256 strings (`"39"`) and 24-bit hex strings (`"#00d7ff"`).

---

## 6. Verification Checklist for Agents

Before completing any task, ensure the following steps pass:
- [ ] Code builds without errors: `go build ./...`
- [ ] All unit and integration tests pass: `go test -v ./...`
- [ ] Tabbed and Grid UI mode tests pass: `go test -v -run TestTabbed ./internal/tui` and `go test -v -run TestUIMode ./internal/tui`
- [ ] Responsive footer test passes: `go test -v -run TestFooter ./internal/tui`
- [ ] No concurrency race conditions: `go test -race ./...`
- [ ] Code is formatted with standard Go style: `gofmt -s -w .`
- [ ] Static analysis passes: `go vet ./...`
- [ ] Any created temporary files or `output/` directories in tests are cleaned up.
- [ ] No unhandled errors or silently ignored error returns in new functionality.
