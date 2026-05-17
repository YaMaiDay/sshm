package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func (m Model) renderDashboardGrouped(indexes []int) string {
	width := contentWidth(m.width)
	if width <= 0 {
		width = m.width
	}
	if width < 34 {
		width = 34
	}
	height := m.dashboardGroupedHeight()
	allLines, selectedTop, selectedBottom := m.groupedLines(indexes, width)
	start, end := dashboardLineWindow(len(allLines), selectedTop, selectedBottom, height)
	lines := append([]string{}, allLines[start:end]...)
	return strings.Join(lines, "\n")
}

func (m Model) dashboardGroupedHeight() int {
	height := m.height - 4
	if height < dashboardGroupedCardHeight() {
		height = dashboardGroupedCardHeight()
	}
	return height
}

func (m Model) groupedLines(indexes []int, width int) ([]string, int, int) {
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	lastCategory := ""
	for i, index := range indexes {
		category := strings.TrimSpace(m.states[index].Host.Category)
		if category == "" {
			category = m.t("Uncategorized", "未分类")
		}
		if i == 0 || category != lastCategory {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.groupedCategoryHeader(category, indexes, width))
			lastCategory = category
		}
		if i == m.selected {
			selectedTop = len(lines)
		}
		cardLines := strings.Split(m.renderGroupedCard(index, i == m.selected, width), "\n")
		lines = append(lines, cardLines...)
		if i == m.selected {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func dashboardGroupedCardHeight() int {
	return 6
}

func (m Model) groupedCategoryHeader(category string, indexes []int, width int) string {
	count := 0
	for _, index := range indexes {
		cat := strings.TrimSpace(m.states[index].Host.Category)
		if cat == "" {
			cat = m.t("Uncategorized", "未分类")
		}
		if cat == category {
			count++
		}
	}
	countText := fmt.Sprintf("%d%s", count, m.t(" servers", "台"))
	nameWidth := width - runewidth.StringWidth(countText) - 2
	if nameWidth < 1 {
		nameWidth = 1
	}
	label := fit(category, nameWidth)
	spaces := width - runewidth.StringWidth(label) - runewidth.StringWidth(countText)
	if spaces < 1 {
		spaces = 1
	}
	return titleStyle.Render(label + strings.Repeat(" ", spaces) + countText)
}

func (m Model) renderGroupedCard(index int, selected bool, width int) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	innerWidth := cardWidth - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	favoriteMark := ""
	if h.Favorite {
		favoriteMark = favoriteStyle.Render("★") + " "
	}
	pinnedMark := ""
	if h.Pinned {
		pinnedMark = pinnedStyle.Render("▲") + " "
	}
	title := pinnedMark + favoriteMark + h.Name
	recentLabel := ""
	if recent := m.lastLoginCard(m.lastLogin(h)); recent != "" {
		recentLabel = cardMutedStyle.Render(recent)
	}
	uptimeLabel := m.cardHeaderMeta(h, metrics)
	stateMark := colorStatus("●", state.Loading, metrics.Online)

	userPort := h.User
	if userPort == "" {
		userPort = "-"
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	addressLine := fmt.Sprintf("%s %s:%s", h.Address(), userPort, port)

	barWidth := 8
	thresholds := m.metricThresholds()
	cpuLine := groupedMetricText("CPU", metrics.CPUPercent, m.cpuCoresText(metrics), barWidth, thresholds.CPUWarn, thresholds.CPUCrit)
	memLine := groupedMetricText(m.t("Mem", "内存"), metrics.MemPercent(), bytesPair(metrics.MemUsed, metrics.MemTotal), barWidth, thresholds.MemWarn, thresholds.MemCrit)
	diskLine := groupedMetricText(m.t("Disk", "磁盘"), metrics.DiskPercent(), diskSummaryText(metrics), barWidth, thresholds.DiskWarn, thresholds.DiskCrit)
	loadLine := fmt.Sprintf("%s %s / %s / %s", m.t("Load", "负载"), emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))
	serviceLine := m.serviceCardText(metrics)
	if riskText := m.cardRiskText(m.buildChecks(state), innerWidth); riskText != "" {
		serviceLine += "  " + riskText
	}
	noteLine := groupedNoteText(h.Note)

	lines := []string{groupedCardTopLine(cardWidth, title, recentLabel, uptimeLabel, stateMark, borderStyle)}
	contentParts := []groupedAdaptivePart{
		{Text: cardMutedStyle.Render(addressLine), Width: 26},
		{Text: cpuLine, Width: 24},
		{Text: memLine, Width: 36},
		{Text: diskLine, Width: 36},
		{Text: cardMutedStyle.Render(loadLine), Width: 25},
		{Text: serviceLine, Width: 26},
	}
	if noteLine != "" {
		contentParts = append(contentParts, groupedAdaptivePart{Text: cardMutedStyle.Render(noteLine), Width: 30})
	}
	for _, line := range groupedAdaptiveContentLines(innerWidth, contentParts) {
		lines = append(lines, cardContentLine(cardWidth, line, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func groupedMetricText(label string, value float64, extra string, barWidth int, warn float64, crit float64) string {
	return fmt.Sprintf("%s %s %s",
		cardMutedStyle.Render(label),
		percentBarWidthWithThreshold(value, barWidth, warn, crit),
		cardMutedStyle.Render(emptyDash(extra)),
	)
}

func groupedCardTopLine(width int, title string, middle string, meta string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffixText := dot
	if strings.TrimSpace(meta) != "" && strings.TrimSpace(meta) != "-" {
		suffixText = meta + " " + dot
	}
	suffix := " " + suffixText + " "
	baseWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if baseWidth < 1 {
		baseWidth = 1
	}
	if ansi.StringWidth(title) > baseWidth {
		title = ansi.Truncate(title, baseWidth, "…")
	}
	fillWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if fillWidth < 0 {
		fillWidth = 0
	}
	fill := borderStyle.Render(strings.Repeat("─", fillWidth))
	middle = strings.TrimSpace(middle)
	if middle != "" && fillWidth > ansi.StringWidth(middle)+2 {
		middleWidth := ansi.StringWidth(middle)
		fillStart := ansi.StringWidth(prefix) + ansi.StringWidth(title) + ansi.StringWidth(titleGap)
		targetStart := (innerWidth - middleWidth) / 2
		leftFill := targetStart - fillStart - 1
		if leftFill < 0 {
			leftFill = 0
		}
		if leftFill+middleWidth+2 > fillWidth {
			leftFill = fillWidth - middleWidth - 2
		}
		if leftFill < 0 {
			leftFill = 0
		}
		rightFill := fillWidth - ansi.StringWidth(middle) - 2 - leftFill
		if rightFill < 0 {
			rightFill = 0
		}
		fill = borderStyle.Render(strings.Repeat("─", leftFill)) + " " + middle + " " + borderStyle.Render(strings.Repeat("─", rightFill))
	}
	return left + prefix + title + titleGap + fill + suffix + right
}

func groupedNoteText(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return "备注 " + note
}

type groupedAdaptivePart struct {
	Text  string
	Width int
}

func groupedAdaptiveContentLines(width int, parts []groupedAdaptivePart) []string {
	if width < 1 {
		width = 1
	}
	const minTrailingWidth = 10
	lines := []string{}
	for i := 0; i < len(parts); {
		rowStart := i
		rowWidth := 0
		for i < len(parts) {
			partWidth := parts[i].Width
			if partWidth < 1 {
				partWidth = ansi.StringWidth(strings.TrimSpace(parts[i].Text))
			}
			if partWidth > width {
				partWidth = width
			}
			nextWidth := partWidth
			if i > rowStart {
				nextWidth += 2
			}
			if i > rowStart && rowWidth+nextWidth > width {
				remaining := width - rowWidth - 2
				if remaining >= minTrailingWidth {
					i++
				}
				break
			}
			rowWidth += nextWidth
			i++
		}
		lines = append(lines, groupedAdaptiveLine(width, parts[rowStart:i]))
	}
	return lines
}

func groupedAdaptiveLine(width int, parts []groupedAdaptivePart) string {
	line := ""
	used := 0
	for i, part := range parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		partWidth := part.Width
		if partWidth < 1 {
			partWidth = ansi.StringWidth(text)
		}
		if used > 0 {
			if used+2 >= width {
				break
			}
			line += "  "
			used += 2
		}
		if used+partWidth > width {
			partWidth = width - used
		}
		if i == len(parts)-1 {
			partWidth = width - used
		}
		if partWidth <= 0 {
			break
		}
		tail := ""
		if i == len(parts)-1 {
			tail = "…"
		}
		text = ansi.Truncate(text, partWidth, tail)
		line += padVisible(text, partWidth)
		used += partWidth
	}
	return line
}
