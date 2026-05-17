package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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
		category = m.t("Uncategorized", "未分类")
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
	thresholds := m.metricThresholds()
	cpu := percentBarWidthWithThreshold(metrics.CPUPercent, barWidth, thresholds.CPUWarn, thresholds.CPUCrit)
	mem := percentBarWidthWithThreshold(metrics.MemPercent(), barWidth, thresholds.MemWarn, thresholds.MemCrit)
	disk := percentBarWidthWithThreshold(metrics.DiskPercent(), barWidth, thresholds.DiskWarn, thresholds.DiskCrit)

	cpuLine := cardMetricLine("CPU", cpu, m.cpuCoresText(metrics), innerWidth)
	memLine := cardMetricLine(m.t("Mem", "内存"), mem, bytesPair(metrics.MemUsed, metrics.MemTotal), innerWidth)
	diskLine := cardMetricLine(m.diskMetricLabel(metrics), disk, bytesPair(metrics.DiskUsed, metrics.DiskTotal), innerWidth)
	uptimeLabel := m.cardHeaderMeta(h, metrics)
	loadLine := fit(fmt.Sprintf("%s %s / %s / %s", m.t("Load", "负载"), emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15)), innerWidth)
	serviceLine := ansi.Truncate(m.serviceCardText(metrics), innerWidth, "…")
	riskText := m.cardRiskText(m.buildChecks(state), innerWidth)
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
	if recent := m.lastLoginCard(m.lastLogin(h)); recent != "" {
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
