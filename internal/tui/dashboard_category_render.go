package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/host"
)

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
			category = m.t("Uncategorized", "未分类")
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

func (m Model) dashboardCategoryHostName(h host.Host, selected bool, width int, showCategory bool, fixedMarkSlots bool) string {
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
		category = m.t("Uncategorized", "未分类")
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
	status := m.t("Offline", "离线")
	if state.Loading {
		status = m.t("Loading", "采集")
	} else if metrics.Online {
		status = m.t("Online", "在线")
	}
	nameWidth := dashboardCategoryNameWidth(width)
	name := m.dashboardCategoryHostName(h, selected, nameWidth, showCategory, fixedMarkSlots)
	statusText := colorStatus(status, state.Loading, metrics.Online)
	cpu, mem, disk := m.compactResourceTriplet(state)
	container, service := m.compactServicePair(metrics)
	timeText := m.cardHeaderMeta(h, metrics)
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
	lines := []string{titleStyle.Render(fit(m.t("Category", "分类"), contentWidth))}
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
		{Label: m.t("All", "全部"), Kind: "all", Count: len(m.states)},
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
		return m.t("All", "全部")
	}
	index := m.dashboardCategorySelectedIndex(items)
	if index < 0 || index >= len(items) {
		return m.t("All", "全部")
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
