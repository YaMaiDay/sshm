package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func (m Model) renderDashboard(indexes []int) string {
	if m.searching {
		return m.renderDashboardList(indexes, m.width)
	}
	if m.dashboardMode == dashboardCategory {
		return m.renderDashboardCategory(indexes)
	}
	if m.dashboardMode == dashboardGrouped {
		return m.renderDashboardGrouped(indexes)
	}
	return m.renderDashboardGrid(indexes)
}

func dashboardModeName(mode dashboardMode) string {
	switch mode {
	case dashboardCategory:
		return "分类"
	case dashboardGrouped:
		return "分组"
	default:
		return "卡片"
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

func (m Model) renderDashboardList(indexes []int, width int) string {
	if width <= 0 {
		width = contentWidth(m.width)
	}
	height := m.dashboardListHeight()
	start, end := visibleRange(len(indexes), m.selected, height)
	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		realIndex := indexes[i]
		lines = append(lines, m.dashboardListLine(realIndex, i == m.selected, width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardListHeight() int {
	height := m.height - 4
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) dashboardListLine(index int, selected bool, width int) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	status := "离线"
	if state.Loading {
		status = "采集"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := 28
	if width < 110 {
		nameWidth = 22
	}
	if width < 78 {
		nameWidth = 16
	}
	name := nameStyle.Render(padVisible(fitANSI(dashboardHostDisplayName(h), nameWidth), nameWidth))
	statusText := padVisible(colorStatus(status, state.Loading, metrics.Online), 6)
	cpu, mem, disk := dashboardListResourceColumns(state)
	containerText, serviceText := dashboardListServiceColumns(metrics)
	expire := padVisible(expireCardTextOrDash(h.ExpireAt), 10)
	addressWidth := 22
	if width < 100 {
		addressWidth = 16
	}
	address := cardMutedStyle.Render(padVisible(fit(h.Address(), addressWidth), addressWidth))
	line := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s  %s  %s", prefix, name, statusText, cpu, mem, disk, containerText, serviceText, expire, address)
	return fitANSI(line, width)
}

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
			category = "未分类"
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
			cat = "未分类"
		}
		if cat == category {
			count++
		}
	}
	countText := fmt.Sprintf("%d台", count)
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
	if recent := lastLoginCard(m.lastLogin(h)); recent != "" {
		recentLabel = cardMutedStyle.Render(recent)
	}
	uptimeLabel := cardHeaderMeta(h, metrics)
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
	cpuLine := groupedMetricText("CPU", metrics.CPUPercent, cpuCoresText(metrics), barWidth, 70, 85)
	memLine := groupedMetricText("内存", metrics.MemPercent(), bytesPair(metrics.MemUsed, metrics.MemTotal), barWidth, 70, 85)
	diskLine := groupedMetricText("磁盘", metrics.DiskPercent(), diskSummaryText(metrics), barWidth, 80, 90)
	loadLine := fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))
	serviceLine := serviceCardText(metrics)
	if riskText := cardRiskText(buildChecks(state), innerWidth); riskText != "" {
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

func dashboardListResourceColumns(state hostState) (string, string, string) {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render(padVisible("CPU -", 7)),
			detailValueStyle.Render(padVisible("内存 -", 8)),
			detailValueStyle.Render(padVisible("磁盘 -", 8))
	}
	cpu := "CPU " + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%3.0f%%", metrics.CPUPercent))
	mem := "内存 " + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%3.0f%%", metrics.MemPercent()))
	disk := "磁盘 " + diskMountPercentText(metrics)
	return padVisible(cpu, 7), padVisible(mem, 8), padVisible(disk, 14)
}

func dashboardListServiceColumns(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	containerRaw := fmt.Sprintf("容器 %d/%d/%d", metrics.DockerFailed, metrics.DockerRunning, total)
	if total == 0 {
		containerRaw = "容器 0"
	}
	container := cardMutedStyle.Render("容器 ")
	if metrics.DockerFailed > 0 {
		container += redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	} else if total == 0 {
		container += cardMutedStyle.Render("0")
	} else {
		container += cardMutedStyle.Render(fmt.Sprintf("0/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	service := cardMutedStyle.Render("服务 ") + serviceNumber
	return padVisible(container, maxInt(12, ansi.StringWidth(containerRaw))), padVisible(service, 7)
}

func compactResourceTriplet(state hostState) (string, string, string) {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return cardMutedStyle.Render("CPU") + detailValueStyle.Render("-"),
			cardMutedStyle.Render("内") + detailValueStyle.Render("-"),
			cardMutedStyle.Render("磁") + detailValueStyle.Render("-")
	}
	return cardMutedStyle.Render("CPU") + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%.0f", metrics.CPUPercent)),
		cardMutedStyle.Render("内") + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%.0f", metrics.MemPercent())),
		cardMutedStyle.Render("磁") + metricValueStyle(metrics.DiskPercent(), 80, 90).Render(fmt.Sprintf("%.0f", metrics.DiskPercent()))
}

func compactServicePair(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	container := "容器0"
	if total > 0 {
		if metrics.DockerFailed > 0 {
			container = cardMutedStyle.Render("容器") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
		} else {
			container = cardMutedStyle.Render(fmt.Sprintf("容器0/%d/%d", metrics.DockerRunning, total))
		}
	} else {
		container = cardMutedStyle.Render(container)
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	return container, cardMutedStyle.Render("服务") + serviceNumber
}

func compactExpireText(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render("到期-")
	}
	return expireCardText(value)
}

func expireCardTextOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render("到期 -")
	}
	return expireCardText(value)
}

func (m Model) renderDashboardCategory(indexes []int) string {
	width := contentWidth(m.width)
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 100 {
		return m.renderDashboardCategoryTop(indexes, width)
	}
	leftWidth := 24
	if width >= 120 {
		leftWidth = 28
	}
	gap := 0
	height := m.dashboardCategoryBodyHeight()
	rightWidth := width - leftWidth - gap
	if rightWidth < 34 {
		return m.renderDashboardCategoryTop(indexes, width)
	}
	left := m.renderDashboardCategoryPane(leftWidth, height)
	right := m.renderDashboardCategoryServerPane(indexes, rightWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
}

func (m Model) renderDashboardCategoryTop(indexes []int, width int) string {
	if width < 34 {
		width = 34
	}
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	bar := m.renderDashboardCategoryTopBar(contentWidth)
	height := m.dashboardCategoryBodyHeight() - 2
	if height < 3 {
		height = 3
	}
	list := m.renderDashboardCategoryServers(indexes, contentWidth, height)
	content := strings.Join([]string{
		bar,
		detailFrameSeparator(contentWidth),
		list,
	}, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width - 2).
		Render(content)
}

func (m Model) renderDashboardCategoryTopBar(width int) string {
	items := m.dashboardCategoryItems()
	selected := m.dashboardCategorySelectedIndex(items)
	if width < 10 {
		width = 10
	}
	parts := make([]string, 0, len(items))
	for i, item := range items {
		label := fmt.Sprintf("%s %d", item.Label, item.Count)
		if i == selected {
			label = titleStyle.Render(label)
		} else {
			label = mutedStyle.Render(label)
		}
		parts = append(parts, label)
	}
	value := ""
	if len(parts) > 0 {
		value = strings.Join(parts, "  ")
		if ansi.StringWidth(value) > width && selected > 0 {
			value = strings.Join(parts[selected:], "  ")
		}
	}
	return padVisible(fitANSI(value, width), width)
}

func (m Model) dashboardCategoryBodyHeight() int {
	height := m.height - 4
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) renderDashboardCategoryServers(indexes []int, width int, height int) string {
	if m.dashboardCategoryShowsGroupedServers() {
		return m.renderDashboardCategoryGroupedServers(indexes, width, height)
	}
	start, end := visibleRange(len(indexes), m.selected, height)
	lines := []string{}
	for i := start; i < end; i++ {
		lines = append(lines, padVisible(m.dashboardCategoryServerLine(indexes[i], i == m.selected, width), width))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardCategoryShowsGroupedServers() bool {
	return m.dashboardMode == dashboardCategory && m.filter == filterAll && !m.favoriteOnly && strings.TrimSpace(m.query) == ""
}

func (m Model) renderDashboardCategoryGroupedServers(indexes []int, width int, height int) string {
	allLines, selectedLine := m.dashboardCategoryGroupedServerLines(indexes, width)
	start := selectedLine - height + 1
	if start < 0 {
		start = 0
	}
	if selectedLine < start {
		start = selectedLine
	}
	if start+height > len(allLines) {
		start = len(allLines) - height
		if start < 0 {
			start = 0
		}
	}
	end := start + height
	if end > len(allLines) {
		end = len(allLines)
	}
	lines := append([]string{}, allLines[start:end]...)
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardCategoryGroupedServerLines(indexes []int, width int) ([]string, int) {
	lines := []string{}
	selectedLine := 0
	lastCategory := ""
	for i, index := range indexes {
		category := strings.TrimSpace(m.states[index].Host.Category)
		if category == "" {
			category = "未分类"
		}
		if i == 0 || category != lastCategory {
			if len(lines) > 0 {
				lines = append(lines, strings.Repeat(" ", width))
			}
			lines = append(lines, m.dashboardCategoryGroupHeader(category, indexes, width))
			lastCategory = category
		}
		if i == m.selected {
			selectedLine = len(lines)
		}
		line := m.dashboardCategoryServerLineWithOptions(index, i == m.selected, width, false, true)
		lines = append(lines, padVisible(fitANSI(line, width), width))
	}
	if len(lines) == 0 {
		return []string{}, 0
	}
	return lines, selectedLine
}

func (m Model) dashboardCategoryGroupHeader(category string, indexes []int, width int) string {
	count := 0
	for _, index := range indexes {
		cat := strings.TrimSpace(m.states[index].Host.Category)
		if cat == "" {
			cat = "未分类"
		}
		if cat == category {
			count++
		}
	}
	countText := fmt.Sprintf("%d台", count)
	nameWidth := width - runewidth.StringWidth(countText) - 2
	if nameWidth < 1 {
		nameWidth = 1
	}
	label := cardMutedStyle.Render(fit(category, nameWidth))
	spaces := width - ansi.StringWidth(label) - runewidth.StringWidth(countText)
	if spaces < 1 {
		spaces = 1
	}
	return padVisible(label+strings.Repeat(" ", spaces)+cardMutedStyle.Render(countText), width)
}

func (m Model) renderDashboardCategoryServerPane(indexes []int, width int, height int) string {
	border := softGray
	styleWidth := width - 2
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(styleWidth).
		Render(m.renderDashboardCategoryServers(indexes, contentWidth, height))
}

func dashboardCategoryNameWidth(width int) int {
	nameWidth := 24
	if width < 92 {
		nameWidth = 20
	}
	if width < 74 {
		nameWidth = 16
	}
	return nameWidth
}

func dashboardCategoryHostName(h host.Host, selected bool, width int, showCategory bool, fixedMarkSlots bool) string {
	marks := ""
	if fixedMarkSlots {
		if h.Pinned {
			marks += pinnedStyle.Render("▲")
		} else {
			marks += " "
		}
		marks += " "
		if h.Favorite {
			marks += favoriteStyle.Render("★")
		} else {
			marks += " "
		}
		marks += " "
	} else {
		if h.Pinned {
			marks += pinnedStyle.Render("▲") + " "
		}
		if h.Favorite {
			marks += favoriteStyle.Render("★") + " "
		}
	}
	category := strings.TrimSpace(h.Category)
	if category == "" {
		category = "未分类"
	}
	categoryText := ""
	if showCategory {
		categoryText = cardMutedStyle.Render("[" + category + "]")
	}
	nameStyle := detailValueStyle
	if selected {
		nameStyle = blueStyle.Bold(true)
	}
	name := strings.TrimSpace(h.Name)
	if name == "" {
		name = h.Address()
	}
	marksWidth := ansi.StringWidth(marks)
	categoryWidth := 0
	if showCategory {
		categoryWidth = runewidth.StringWidth("[" + category + "]")
	}
	nameMinWidth := 8
	if width < marksWidth+categoryWidth+1+nameMinWidth {
		categoryText = ""
		categoryWidth = 0
	}
	nameWidth := width - marksWidth - categoryWidth
	if categoryText != "" {
		nameWidth--
	}
	if nameWidth < 1 {
		nameWidth = 1
	}
	text := marks
	if categoryText != "" {
		text += categoryText + " "
	}
	text += nameStyle.Render(fitANSI(name, nameWidth))
	return padVisible(text, width)
}

func (m Model) dashboardCategoryServerLine(index int, selected bool, width int) string {
	return m.dashboardCategoryServerLineWithOptions(index, selected, width, true, false)
}

func (m Model) dashboardCategoryServerLineWithOptions(index int, selected bool, width int, showCategory bool, fixedMarkSlots bool) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	status := "离线"
	if state.Loading {
		status = "采集"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := dashboardCategoryNameWidth(width)
	name := dashboardCategoryHostName(h, selected, nameWidth, showCategory, fixedMarkSlots)
	statusText := colorStatus(status, state.Loading, metrics.Online)
	cpu, mem, disk := compactResourceTriplet(state)
	container, service := compactServicePair(metrics)
	timeText := cardHeaderMeta(h, metrics)
	cell := func(value string, cellWidth int) string {
		return padVisible(fitANSI(value, cellWidth), cellWidth)
	}
	fields := []string{
		name,
		cell(statusText, 4),
		cell(cpu, 6),
		cell(mem, 5),
		cell(disk, 5),
		cell(container, 11),
		cell(service, 7),
	}
	fields = append(fields, cell(timeText, 8))
	line := strings.Join(fields, " ")
	used := ansi.StringWidth(line)
	if remaining := width - used - 1; remaining >= 8 {
		line += " " + cell(cardMutedStyle.Render(h.Address()), remaining)
	}
	return fitANSI(line, width)
}

func (m Model) renderDashboardCategoryPane(width int, height int) string {
	items := m.dashboardCategoryItems()
	active := m.dashboardMode == dashboardCategory && m.dashboardFocus == 0
	selected := m.dashboardCategorySelectedIndex(items)
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	lines := []string{titleStyle.Render(fit("分类", contentWidth))}
	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}
	start, end := visibleRange(len(items), selected, listHeight)
	for i := start; i < end; i++ {
		item := items[i]
		prefix := " "
		style := detailValueStyle
		if i == selected {
			prefix = "▶"
			if active {
				style = blueStyle.Bold(true)
			}
		}
		count := mutedStyle.Render(fmt.Sprintf("%d", item.Count))
		countWidth := ansi.StringWidth(count)
		labelWidth := contentWidth - countWidth - 3
		if labelWidth < 4 {
			labelWidth = 4
		}
		label := style.Render(fit(item.Label, labelWidth))
		line := prefix + " " + label
		spaces := contentWidth - ansi.StringWidth(line) - countWidth
		if spaces < 1 {
			spaces = 1
		}
		lines = append(lines, padVisible(line+strings.Repeat(" ", spaces)+count, contentWidth))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}
	border := softGray
	if active {
		border = blue
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
}

type dashboardCategoryItem struct {
	Label string
	Kind  string
	Value string
	Count int
}

func (m Model) dashboardCategoryItems() []dashboardCategoryItem {
	items := []dashboardCategoryItem{
		{Label: "全部", Kind: "all", Count: len(m.states)},
	}
	seen := map[string]bool{}
	categories := []string{}
	for _, state := range m.states {
		cat := state.Host.Category
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories)
	for _, category := range categories {
		cat := category
		items = append(items, dashboardCategoryItem{
			Label: category,
			Kind:  "category",
			Value: category,
			Count: m.countHosts(func(state hostState) bool { return state.Host.Category == cat }),
		})
	}
	return items
}

func (m Model) countHosts(match func(hostState) bool) int {
	count := 0
	for _, state := range m.states {
		if match(state) {
			count++
		}
	}
	return count
}

func (m Model) dashboardCategorySelectedIndex(items []dashboardCategoryItem) int {
	for i, item := range items {
		switch item.Kind {
		case "problem":
			if m.filter == filterProblem {
				return i
			}
		case "online":
			if m.filter == filterOnline {
				return i
			}
		case "category":
			if m.filter == filterAll && m.category == item.Value {
				return i
			}
		case "all":
			if m.filter == filterAll && m.category == "" {
				return i
			}
		}
	}
	return 0
}

func (m Model) dashboardCategoryActiveLabel() string {
	items := m.dashboardCategoryItems()
	if len(items) == 0 {
		return "全部"
	}
	index := m.dashboardCategorySelectedIndex(items)
	if index < 0 || index >= len(items) {
		return "全部"
	}
	return items[index].Label
}

func (m *Model) applyDashboardCategoryItem(item dashboardCategoryItem) {
	m.favoriteOnly = false
	m.filter = filterAll
	m.category = ""
	switch item.Kind {
	case "problem":
		m.filter = filterProblem
	case "online":
		m.filter = filterOnline
	case "category":
		m.category = item.Value
	}
	m.selected = 0
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

func (m Model) renderCard(index int, selected bool, width int, reserveNoteLine bool) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics

	innerWidth := width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	category := h.Category
	if category == "" {
		category = "未分类"
	}
	favoriteMark := ""
	if h.Favorite {
		favoriteMark = favoriteStyle.Render("★") + " "
	}
	pinnedMark := ""
	if h.Pinned {
		pinnedMark = pinnedStyle.Render("▲") + " "
	}
	prefixMarks := pinnedMark + favoriteMark
	categoryLabel := "[" + category + "]"
	titleText := prefixMarks + categoryLabel + " " + h.Name
	if ansi.StringWidth(titleText) > innerWidth {
		prefixMarks = ""
		titleText = categoryLabel + " " + h.Name
	}
	barWidth := 12
	if innerWidth < 42 {
		barWidth = 8
	}
	cpu := percentBarWidth(metrics.CPUPercent, barWidth)
	mem := percentBarWidth(metrics.MemPercent(), barWidth)
	disk := percentBarWidthWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)

	cpuLine := cardMetricLine("CPU", cpu, cpuCoresText(metrics), innerWidth)
	memLine := cardMetricLine("内存", mem, bytesPair(metrics.MemUsed, metrics.MemTotal), innerWidth)
	diskLine := cardMetricLine(diskMetricLabel(metrics), disk, bytesPair(metrics.DiskUsed, metrics.DiskTotal), innerWidth)
	uptimeLabel := cardHeaderMeta(h, metrics)
	loadLine := fit(fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15)), innerWidth)
	serviceLine := ansi.Truncate(serviceCardText(metrics), innerWidth, "…")
	riskText := cardRiskText(buildChecks(state), innerWidth)
	if riskText != "" {
		serviceLine = ansi.Truncate(serviceLine+"  "+riskText, innerWidth, "…")
	}
	title := titleText

	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	userPort := h.User
	if userPort == "" {
		userPort = "-"
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	userPort += ":" + port
	addressText := fmt.Sprintf("%s %s", h.Address(), userPort)
	if recent := lastLoginCard(m.lastLogin(h)); recent != "" {
		addressText += "  " + recent
	}
	addressLine := fit(addressText, innerWidth)
	noteLine := cardNoteText(h.Note, innerWidth)
	stateMark := colorStatus("●", state.Loading, metrics.Online)
	lines := []string{
		cardTopLine(cardWidth, title, uptimeLabel, stateMark, borderStyle),
		cardMutedContentLine(cardWidth, addressLine, borderStyle),
		cardContentLine(cardWidth, cpuLine, borderStyle),
		cardContentLine(cardWidth, memLine, borderStyle),
		cardContentLine(cardWidth, diskLine, borderStyle),
		cardInnerSeparatorLine(cardWidth, borderStyle),
		cardMutedContentLine(cardWidth, loadLine, borderStyle),
		cardContentLine(cardWidth, serviceLine, borderStyle),
	}
	if noteLine != "" || reserveNoteLine {
		lines = append(lines, cardMutedContentLine(cardWidth, noteLine, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func blankCard(width int, reserveNoteLine bool) string {
	innerWidth := width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	height := dashboardCardInnerHeight
	if reserveNoteLine {
		height++
	}
	return lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder()).
		Padding(0, 1).
		Width(innerWidth).
		Height(height).
		Render("")
}
