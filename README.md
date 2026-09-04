# Chameleon

Chameleon is a modern, fast, and concurrent Terminal User Interface (TUI) web scraper written in Go. 

Designed for ease of use directly from your terminal, it leverages [Bubbletea](https://github.com/charmbracelet/bubbletea) for a rich, responsive interface and [Goquery](https://github.com/PuerkitoBio/goquery) for robust HTML parsing and link discovery.

## Features

- **Rich Terminal UI**: A beautiful, split-pane dashboard powered by Bubbletea and Lipgloss, featuring real-time updates.
- **Scrapability & Ethical Audit**: Instant diagnostics evaluating robots.txt, sitemaps, WAF/Cloudflare bot defenses, Single Page App (SPA) dependencies, CSS obfuscation, honeypots, and passive rate limits with actionable recommendations.
- **Concurrent Engine**: Scrape pages asynchronously using multiple worker goroutines.
- **Live Telemetry**: Watch HTTP status codes, response sizes, content types, and response times update live in a data table.
- **Auto-Saving**: Automatically saves scraped HTML and assets to the local `output/` directory, mirroring the remote site's directory structure.
- **Recursive Crawling**: Configurable link discovery and depth traversal.
- **Rate Limiting**: Built-in request rate limiting (Safe/Fast modes) to respect target servers.

## Installation

Ensure you have [Go](https://go.dev/) 1.25 or later installed.

Clone the repository and install the dependencies:

```bash
git clone https://github.com/yourusername/chameleon.git
cd chameleon
go mod download
```

## Usage

You can run Chameleon directly using `make run` (or `go run`):

```bash
make run
```

### Navigating the TUI

Once launched, you will be presented with the Chameleon dashboard:

- **Switch Focus (`Tab` / `Shift+Tab`)**: Cycle focus between all panels. The active panel is highlighted with a thick vibrant border.
- **Input Panel**: Type the target URL:
  - Press `Enter` to queue and scrape.
  - Press `Ctrl+A` (or `F2`) to trigger a **Scrapability & Ethical Audit**.
- **Center Panel (Telemetry / Report Card)**:
  - Press `Ctrl+A` or `A` to toggle between the **Live Telemetry Table** and the **Scrapability & Ethical Report Card**.
- **Queue Panel (Left)**: 
  - Scroll through pending/discovered URLs with `↑` / `↓` / `PgUp` / `PgDn` / `j` / `k`.
  - Press `Space` / `p` / `Enter` to pause or resume the selected job.
  - Press `P` (`Shift+P`) to pause or resume all jobs in the queue.
  - Press `s` / `d` / `Del` to stop the selected job.
- **Files Panel (Right)**: Scroll through saved file paths with `↑` / `↓` / `PgUp` / `PgDn` / `j` / `k`.
- **Settings Panel (Bottom Right)**: Use `↑` / `↓` to select Depth, Images, or Speed, and `←` / `→` / `Space` / `Enter` to configure or toggle.
- **Quit**: Press `Ctrl+C` to exit gracefully.

## Project Structure

```
chameleon/
├── cmd/
│   └── chameleon/            # Application entrypoint
│       └── main.go
├── configs/                  # Example and template configurations
├── internal/
│   ├── engine/               # Core web scraping and parsing logic
│   │   ├── analyzer/         # Scrapability and ethical analysis audit suite
│   │   │   ├── analyzer.go
│   │   │   └── analyzer_test.go
│   │   ├── engine.go
│   │   └── engine_test.go
│   └── tui/                  # Bubbletea terminal interface components
│       ├── tui.go
│       └── tui_test.go
├── output/                   # Directory where scraped files are saved
├── scripts/                  # Development scripts and mock servers
├── go.mod                    # Go module definition
└── README.md                 # This file
```

## Dependencies

- [Bubbletea](https://github.com/charmbracelet/bubbletea) - The fun, functional and stateful way to build terminal apps.
- [Bubbles](https://github.com/charmbracelet/bubbles) - Some standard Bubbletea components (Text Input, Table).
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Style definitions for nice terminal layouts.
- [Goquery](https://github.com/PuerkitoBio/goquery) - jQuery-like HTML DOM parsing.
- [x/time/rate](https://golang.org/x/time) - Rate limiting for the scraping engine.

## License

MIT License
