package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SavedFileEntry represents a saved file with extracted metadata for display.
type SavedFileEntry struct {
	FullPath string
	Host     string
	RelPath  string
	Ext      string
	Type     string
	Size     int64
	SizeStr  string
	SavedAt  time.Time
}

func parseSavedFile(rawPath string) SavedFileEntry {
	cleaned := filepath.Clean(rawPath)
	slashPath := filepath.ToSlash(cleaned)

	var host, relPath string
	trimmed := strings.TrimPrefix(slashPath, "output/")
	trimmed = strings.TrimPrefix(trimmed, "output")
	trimmed = strings.TrimPrefix(trimmed, "/")

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 2 {
		host = parts[0]
		relPath = "/" + parts[1]
	} else if len(parts) == 1 && parts[0] != "" {
		host = parts[0]
		relPath = "/"
	} else {
		host = "local"
		relPath = "/" + slashPath
	}

	ext := strings.ToLower(filepath.Ext(cleaned))
	fileType := "FILE"
	switch ext {
	case ".html", ".htm":
		fileType = "HTML"
	case ".png":
		fileType = "PNG"
	case ".jpg", ".jpeg":
		fileType = "JPG"
	case ".webp":
		fileType = "WEBP"
	case ".svg":
		fileType = "SVG"
	case ".gif":
		fileType = "GIF"
	case ".css":
		fileType = "CSS"
	case ".js":
		fileType = "JS"
	case ".json":
		fileType = "JSON"
	case ".xml", ".txt":
		fileType = "TXT"
	}

	var size int64
	savedAt := time.Now()
	if fi, err := os.Stat(rawPath); err == nil {
		size = fi.Size()
		savedAt = fi.ModTime()
	}

	return SavedFileEntry{
		FullPath: rawPath,
		Host:     host,
		RelPath:  relPath,
		Ext:      ext,
		Type:     fileType,
		Size:     size,
		SizeStr:  formatBytes(size),
		SavedAt:  savedAt,
	}
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := float64(b) / float64(div)
	switch exp {
	case 0:
		return fmt.Sprintf("%.1f KB", val)
	case 1:
		return fmt.Sprintf("%.1f MB", val)
	case 2:
		return fmt.Sprintf("%.1f GB", val)
	default:
		return fmt.Sprintf("%.1f TB", val)
	}
}

func formatBytesCompact(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	val := float64(b) / float64(div)
	switch exp {
	case 0:
		return fmt.Sprintf("%.1fK", val)
	case 1:
		return fmt.Sprintf("%.1fM", val)
	case 2:
		return fmt.Sprintf("%.1fG", val)
	default:
		return fmt.Sprintf("%.1fT", val)
	}
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func padLeft(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

func truncateRunes(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string([]rune(s)[:maxLen])
	}
	runes := []rune(s)
	if len(runes) > maxLen-3 {
		return string(runes[:maxLen-3]) + "..."
	}
	return s
}

func (m *Model) addFile(path string) {
	m.files = append(m.files, path)
	entry := parseSavedFile(path)
	m.savedFiles = append(m.savedFiles, entry)
	m.totalSavedSize += entry.Size
	m.needsFilesUpdate = true
}

func (m Model) isFilesFocused() bool {
	if m.uiMode == UIModeTabbed {
		return m.activeTab == TabFiles && m.focusTarget == FocusTabContent
	}
	return m.activePanel == PanelFiles
}

func (m *Model) updateFilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.filesCursor > 0 {
			m.filesCursor--
			m.updateFilesContent()
		}
		return *m, nil
	case "down", "j":
		if m.filesCursor < len(m.savedFiles)-1 {
			m.filesCursor++
			m.updateFilesContent()
		}
		return *m, nil
	case "home", "g":
		if len(m.savedFiles) > 0 {
			m.filesCursor = 0
			m.updateFilesContent()
		}
		return *m, nil
	case "end", "G":
		if len(m.savedFiles) > 0 {
			m.filesCursor = len(m.savedFiles) - 1
			m.updateFilesContent()
		}
		return *m, nil
	}
	var cmd tea.Cmd
	m.filesViewport, cmd = m.filesViewport.Update(msg)
	return *m, cmd
}

func (m *Model) updateFilesContent() {
	if len(m.savedFiles) == 0 {
		m.filesViewport.SetContent("  (no saved files)")
		return
	}

	if m.filesCursor >= len(m.savedFiles) {
		m.filesCursor = len(m.savedFiles) - 1
	}
	if m.filesCursor < 0 {
		m.filesCursor = 0
	}

	vw := m.filesViewport.Width
	if vw <= 0 {
		vw = 40
	}

	// Calculate aggregate statistics
	var totalSize int64
	var htmlCount, imgCount, otherCount int
	for _, f := range m.savedFiles {
		totalSize += f.Size
		switch f.Type {
		case "HTML":
			htmlCount++
		case "PNG", "JPG", "WEBP", "SVG", "GIF":
			imgCount++
		default:
			otherCount++
		}
	}

	badgeStyle := func(fileType string) string {
		switch fileType {
		case "HTML":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render("[HTML]")
		case "PNG":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")).Render("[PNG ]")
		case "JPG":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("206")).Render("[JPG ]")
		case "WEBP":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("177")).Render("[WEBP]")
		case "SVG":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219")).Render("[SVG ]")
		case "GIF":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("183")).Render("[GIF ]")
		case "CSS":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220")).Render("[CSS ]")
		case "JS":
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("46")).Render("[JS  ]")
		case "JSON", "TXT":
			return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[DATA]")
		default:
			return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("[FILE]")
		}
	}

	var b strings.Builder
	var headerOffset int

	if vw >= 70 {
		// 1. Storage Summary Card
		boxW := vw - 2
		if boxW > 86 {
			boxW = 86
		}
		if boxW < 50 {
			boxW = 50
		}

		cardBorder := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TableHeaderBorder))
		cardTitle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render(" 📂 STORAGE SUMMARY ")

		titleLen := lipgloss.Width(cardTitle)
		topDashes := boxW - 3 - titleLen
		if topDashes < 2 {
			topDashes = 2
		}
		topLine := cardBorder.Render("┌─") + cardTitle + cardBorder.Render(strings.Repeat("─", topDashes)+"┐") + "\n"

		statContent := fmt.Sprintf("Total: %d files (%s)  •  HTML: %d  •  Images: %d", len(m.savedFiles), formatBytes(totalSize), htmlCount, imgCount)
		if otherCount > 0 {
			statContent += fmt.Sprintf("  •  Other: %d", otherCount)
		}
		statStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(statContent)
		statLen := lipgloss.Width(statStyled)
		statSpaces := boxW - 3 - statLen
		if statSpaces < 1 {
			statSpaces = 1
		}
		midLine := cardBorder.Render("│ ") + statStyled + strings.Repeat(" ", statSpaces) + cardBorder.Render("│") + "\n"
		botLine := cardBorder.Render("└"+strings.Repeat("─", boxW-2)+"┘") + "\n\n"

		b.WriteString(topLine)
		b.WriteString(midLine)
		b.WriteString(botLine)

		// 2. Table Header
		colIdx := 4
		colType := 6
		colSize := 10
		colTime := 8

		tableWidth := vw - 2
		fixed := colIdx + colType + colSize + colTime + 10
		remaining := tableWidth - fixed
		if remaining < 20 {
			remaining = 20
		}
		colHost := int(float64(remaining) * 0.38)
		if colHost < 14 {
			colHost = 14
		}
		colPath := remaining - colHost
		if colPath < 16 {
			colPath = 16
		}

		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
		hdrLine := fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			padRight("#", colIdx),
			padRight("TYPE", colType),
			padRight("HOST", colHost),
			padRight("PATH / FILENAME", colPath),
			padLeft("SIZE", colSize),
			padRight("SAVED", colTime),
		)
		b.WriteString(headerStyle.Render(hdrLine) + "\n")

		divStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TableHeaderBorder))
		divLine := fmt.Sprintf("%s  %s  %s  %s  %s  %s",
			strings.Repeat("─", colIdx),
			strings.Repeat("─", colType),
			strings.Repeat("─", colHost),
			strings.Repeat("─", colPath),
			strings.Repeat("─", colSize),
			strings.Repeat("─", colTime),
		)
		b.WriteString(divStyle.Render(divLine) + "\n")

		headerOffset = 5

		// 3. Rows
		for i, f := range m.savedFiles {
			isSelected := (i == m.filesCursor && m.isFilesFocused())

			var idxStr string
			if isSelected {
				idxStr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render("▶" + padLeft(strconv.Itoa(i+1)+".", colIdx-1))
			} else {
				idxStr = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(padLeft(strconv.Itoa(i+1)+".", colIdx))
			}

			typeBadge := badgeStyle(f.Type)

			displayHost := truncateRunes(f.Host, colHost)
			hostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("253"))
			if isSelected {
				hostStyle = hostStyle.Bold(true).Foreground(lipgloss.Color("255"))
			}
			hostStr := padRight(hostStyle.Render(displayHost), colHost)

			displayPath := truncateRunes(f.RelPath, colPath)
			pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
			if isSelected {
				pathStyle = pathStyle.Bold(true).Foreground(lipgloss.Color("255"))
			}
			pathStr := padRight(pathStyle.Render(displayPath), colPath)

			sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
			if isSelected {
				sizeStyle = sizeStyle.Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor))
			}
			sizeStr := padLeft(sizeStyle.Render(f.SizeStr), colSize)

			timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
			timeStr := padRight(timeStyle.Render(f.SavedAt.Format("15:04:05")), colTime)

			rowLine := fmt.Sprintf("%s  %s  %s  %s  %s  %s", idxStr, typeBadge, hostStr, pathStr, sizeStr, timeStr)
			if isSelected {
				rowLine = lipgloss.NewStyle().Background(lipgloss.Color(m.theme.TableSelectedBg)).Render(rowLine)
			}
			b.WriteString(rowLine + "\n")
		}
	} else {
		// Narrow mode (Grid mode / small viewports)
		headerOffset = 2
		cardBorder := lipgloss.NewStyle().Foreground(lipgloss.Color(m.theme.TableHeaderBorder))
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor))

		summaryLine := fmt.Sprintf(" 📂 %d Files (%s)", len(m.savedFiles), formatBytes(totalSize))
		b.WriteString(headerStyle.Render(summaryLine) + "\n")

		divWidth := vw - 2
		if divWidth < 10 {
			divWidth = 10
		}
		b.WriteString(cardBorder.Render(strings.Repeat("─", divWidth)) + "\n")

		for i, f := range m.savedFiles {
			isSelected := (i == m.filesCursor && m.isFilesFocused())

			prefix := "  "
			if isSelected {
				prefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor)).Render("▶ ")
			}

			typeBadge := badgeStyle(f.Type)
			compactSize := formatBytesCompact(f.Size)

			used := 2 + 6 + 1 + len(compactSize) + 1
			avail := divWidth - used
			if avail < 4 {
				avail = 4
			}

			filename := filepath.Base(f.RelPath)
			if filename == "/" || filename == "." {
				filename = f.Host
			}
			filename = truncateRunes(filename, avail)

			fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
			if isSelected {
				fileStyle = fileStyle.Bold(true).Foreground(lipgloss.Color("255"))
			}

			sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
			if isSelected {
				sizeStyle = sizeStyle.Bold(true).Foreground(lipgloss.Color(m.theme.AccentColor))
			}

			paddedFile := padRight(fileStyle.Render(filename), avail)
			rowLine := fmt.Sprintf("%s%s %s %s", prefix, typeBadge, paddedFile, sizeStyle.Render(compactSize))
			if isSelected {
				rowLine = lipgloss.NewStyle().Background(lipgloss.Color(m.theme.TableSelectedBg)).Render(rowLine)
			}
			b.WriteString(rowLine + "\n")
		}
	}

	m.filesViewport.SetContent(strings.TrimRight(b.String(), "\n"))

	if m.isFilesFocused() && m.filesViewport.Height > 0 {
		if m.filesCursor == 0 {
			m.filesViewport.YOffset = 0
		} else {
			cursorLine := headerOffset + m.filesCursor
			if cursorLine < m.filesViewport.YOffset {
				m.filesViewport.YOffset = cursorLine
			} else if cursorLine >= m.filesViewport.YOffset+m.filesViewport.Height {
				m.filesViewport.YOffset = cursorLine - m.filesViewport.Height + 1
			}
		}
	}
}
