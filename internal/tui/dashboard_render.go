package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderDashboard(indexes []int) string {
	if m.dashboardMode == dashboardCategory {
		return m.renderDashboardCategory(indexes)
	}
	if m.dashboardMode == dashboardGrouped {
		return m.renderDashboardGrouped(indexes)
	}
	return m.renderDashboardGrid(indexes)
}

func (m Model) dashboardModeName(mode dashboardMode) string {
	switch mode {
	case dashboardCategory:
		return m.t("Category", "分类")
	case dashboardGrouped:
		return m.t("Group", "分组")
	default:
		return m.t("Cards", "卡片")
	}
}

func (m Model) renderDashboardGrid(indexes []int) string {
	width := m.dashboardGridWidth()
	height := m.dashboardGridHeight()
	lines, selectedTop, selectedBottom := m.dashboardGridLines(indexes, width)
	start, end := dashboardLineWindow(len(lines), selectedTop, selectedBottom, height)
	return strings.Join(lines[start:end], "\n")
}

func (m Model) dashboardGridWidth() int {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 34 {
		width = 34
	}
	return width
}

func (m Model) dashboardGridHeight() int {
	height := m.height - 4
	if height < 1 {
		height = 1
	}
	return height
}

func (m Model) dashboardGridLines(indexes []int, width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	for i := 0; i < len(indexes); i += cols {
		rowEnd := i + cols
		if rowEnd > len(indexes) {
			rowEnd = len(indexes)
		}
		rowHasNote := false
		for j := i; j < rowEnd; j++ {
			if strings.TrimSpace(indexesHostNote(m.states, indexes[j])) != "" {
				rowHasNote = true
				break
			}
		}
		var row []string
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= len(indexes) {
				row = append(row, padBlock(blankCard(cardWidth, rowHasNote), cardWidth))
				continue
			}
			visibleIndex := i + col
			realIndex := indexes[visibleIndex]
			if visibleIndex == m.selected {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.renderCard(realIndex, visibleIndex == m.selected, cardWidth, rowHasNote), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.selected >= i && m.selected < rowEnd {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func indexesHostNote(states []hostState, index int) string {
	if index < 0 || index >= len(states) {
		return ""
	}
	return states[index].Host.Note
}

func (m Model) dashboardPageDots(indexes []int) string {
	if len(indexes) == 0 {
		return ""
	}
	lines, selectedTop, selectedBottom := m.dashboardGridLines(indexes, m.dashboardGridWidth())
	height := m.dashboardGridHeight()
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, height, m.width)
}

func (m Model) dashboardGroupedDots(indexes []int) string {
	if len(indexes) == 0 {
		return ""
	}
	width := contentWidth(m.width)
	if width <= 0 {
		width = m.width
	}
	if width < 34 {
		width = 34
	}
	lines, selectedTop, selectedBottom := m.groupedLines(indexes, width)
	height := m.dashboardGroupedHeight()
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, height, m.width)
}

func dashboardLineWindow(totalLines int, selectedTop int, selectedBottom int, height int) (int, int) {
	if height <= 0 {
		return 0, 0
	}
	start := selectedBottom - height
	if start < 0 {
		start = 0
	}
	if selectedTop < start {
		start = selectedTop
	}
	if start+height > totalLines {
		start = totalLines - height
		if start < 0 {
			start = 0
		}
	}
	end := start + height
	if end > totalLines {
		end = totalLines
	}
	return start, end
}

func dashboardLineDots(totalLines int, selectedTop int, selectedBottom int, height int, width int) string {
	if height <= 0 || totalLines <= 0 {
		return ""
	}
	totalPages := (totalLines + height - 1) / height
	if totalPages <= 1 {
		return ""
	}
	_, windowEnd := dashboardLineWindow(totalLines, selectedTop, selectedBottom, height)
	currentPage := (windowEnd - 1) / height
	if currentPage >= totalPages {
		currentPage = totalPages - 1
	}
	if currentPage < 0 {
		currentPage = 0
	}
	start := 0
	dotCount := totalPages
	showNumber := false
	if totalPages > 9 {
		dotCount = 7
		showNumber = true
		start = currentPage - dotCount/2
		if start < 0 {
			start = 0
		}
		if start+dotCount > totalPages {
			start = totalPages - dotCount
		}
	}
	parts := make([]string, 0, dotCount+1)
	for i := 0; i < dotCount; i++ {
		page := start + i
		dot := cardBorderStyle.Render("●")
		if page == currentPage {
			dot = titleStyle.Render("●")
		}
		parts = append(parts, dot)
	}
	if showNumber {
		parts = append(parts, mutedStyle.Render(fmt.Sprintf("%d/%d", currentPage+1, totalPages)))
	}
	line := strings.Join(parts, " ")
	if width <= 0 {
		width = 80
	}
	padding := (width - ansi.StringWidth(line)) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + line
}

func padBlock(block string, width int) string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth > width {
			lines[i] = ansi.Truncate(lines[i], width, "")
			lineWidth = ansi.StringWidth(lines[i])
		}
		if lineWidth < width {
			lines[i] += strings.Repeat(" ", width-lineWidth)
		}
	}
	return strings.Join(lines, "\n")
}

func distributeWidths(totalWidth, cols int) []int {
	if cols <= 0 {
		return []int{totalWidth}
	}
	base := totalWidth / cols
	remainder := totalWidth % cols
	widths := make([]int, cols)
	for i := 0; i < cols; i++ {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
		if widths[i] < 34 {
			widths[i] = 34
		}
	}
	return widths
}

func (m Model) dashboardColumns() int {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	switch {
	case width >= 190:
		return 5
	case width >= 148:
		return 4
	case width >= 108:
		return 3
	case width >= 72:
		return 2
	default:
		return 1
	}
}

func withVerticalNav(content string, totalWidth, totalItems, cols, startRow, rowsVisible int) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	targetWidth := totalWidth - 1
	if targetWidth < 1 {
		targetWidth = 1
	}
	totalRows := (totalItems + cols - 1) / cols
	height := len(lines)
	track := make([]string, height)
	for i := range track {
		track[i] = " "
	}
	if totalRows <= rowsVisible {
		for i := range track {
			track[i] = cardBorderStyle.Render("▌")
		}
	} else {
		thumbHeight := height * rowsVisible / totalRows
		if thumbHeight < 1 {
			thumbHeight = 1
		}
		if thumbHeight > height {
			thumbHeight = height
		}
		maxStart := height - thumbHeight
		thumbStart := startRow * maxStart / (totalRows - rowsVisible)
		for i := thumbStart; i < thumbStart+thumbHeight && i < height; i++ {
			track[i] = cardBorderStyle.Render("▌")
		}
	}
	for i := range lines {
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth > targetWidth {
			lines[i] = ansi.Truncate(lines[i], targetWidth, "")
			lineWidth = ansi.StringWidth(lines[i])
		}
		if lineWidth < targetWidth {
			lines[i] += strings.Repeat(" ", targetWidth-lineWidth)
		}
		if track[i] != " " {
			lines[i] += track[i]
		}
	}
	return strings.Join(lines, "\n")
}
