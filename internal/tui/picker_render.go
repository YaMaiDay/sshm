package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderPicker() string {
	header := m.pickTitle
	if m.status != "" && m.status != m.pickTitle {
		header += "  " + m.status
	}
	width := detailFrameWidth(m.width)
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.choices) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的项目"))
	} else {
		maxRows := m.height - 5
		if maxRows < 5 {
			maxRows = 5
		}
		start := 0
		if m.pickIndex >= maxRows {
			start = m.pickIndex - maxRows + 1
		}
		end := start + maxRows
		if end > len(m.choices) {
			end = len(m.choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.pickIndex {
				prefix = "▶"
				style = lipgloss.NewStyle().Foreground(blue).Bold(true)
			}
			label := m.choices[i].Label
			if m.treePickerActive() && m.choices[i].IsDir {
				label = blueStyle.Render(label)
			}
			lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s", prefix, label), width)))
		}
	}
	help := "移动 ↑↓/jk  选择 Enter  返回 Esc"
	if m.treePickerActive() {
		help = "移动 ↑↓/jk  展开 Enter  选择 Space  返回 Esc"
	}
	lines = append(lines, "", renderHelp(width, help))
	return strings.Join(lines, "\n")
}
