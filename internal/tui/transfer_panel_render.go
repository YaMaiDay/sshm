package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderTransferPanel() string {
	title := m.t("Upload File", "上传文件")
	if m.panel.Mode == transferDownload {
		title = m.t("Download File", "下载文件")
	}
	header := title
	if m.status != "" {
		header += "  " + m.status
	}
	width := formContentWidth(m.width)
	help := m.t("Switch Tab  Move ↑↓/jk  Expand Enter  Select Space  Jobs s  Back Esc", "切换 Tab  移动 ↑↓/jk  展开 Enter  选择 Space  任务 s  返回 Esc")
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	body := ""
	if m.useSingleTransferPane(width) {
		if m.panel.ActivePane == 0 {
			body = m.renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, width, height, true, m.panel.LeftSelected)
		} else {
			body = m.renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, width, height, true, nil)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) / 2
		rightWidth := width - gap - leftWidth
		left := m.renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, leftWidth, height, m.panel.ActivePane == 0, m.panel.LeftSelected)
		right := m.renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, rightWidth, height, m.panel.ActivePane == 1, nil)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		body,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) useSingleTransferPane(width int) bool {
	return width < 70
}

func (m Model) renderTransferPane(title string, choices []choice, index int, width int, height int, active bool, selected map[string]bool) string {
	if width < 34 {
		width = 34
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width)
	if active {
		style = style.BorderForeground(blue)
	}
	innerWidth := width - 4
	lines := []string{titleStyle.Render(title)}
	if len(choices) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No selectable items", "没有可选择的项目")))
	} else {
		maxRows := height - 2
		if maxRows < 3 {
			maxRows = 3
		}
		start := 0
		if index >= maxRows {
			start = index - maxRows + 1
		}
		end := start + maxRows
		if end > len(choices) {
			end = len(choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			lineStyle := lipgloss.NewStyle()
			if choices[i].IsDir {
				lineStyle = blueStyle
			}
			if i == index {
				prefix = "▶"
				lineStyle = lineStyle.Bold(true)
			}
			mark := " "
			if selected != nil && selected[choices[i].Value] {
				mark = "✓"
			}
			lines = append(lines, lineStyle.Render(fit(prefix+" "+mark+" "+choices[i].Label, innerWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return style.Render(strings.Join(lines, "\n"))
}
