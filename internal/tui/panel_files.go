package tui

import (
	"fmt"
	"strings"
)

func (m *Model) addFile(path string) {
	m.files = append(m.files, path)
	m.needsFilesUpdate = true
}

func (m *Model) updateFilesContent() {
	if len(m.files) == 0 {
		m.filesViewport.SetContent("  (no saved files)")
		return
	}
	var b strings.Builder
	for i, f := range m.files {
		b.WriteString(fmt.Sprintf("%2d. %s\n", i+1, f))
	}
	m.filesViewport.SetContent(strings.TrimRight(b.String(), "\n"))
	m.filesViewport.GotoBottom()
}
