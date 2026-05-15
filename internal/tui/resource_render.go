package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderResourceList() string {
	width := contentWidth(m.width)
	if width < 44 {
		width = 44
	}
	lines := []string{fitANSI(m.resourceHeaderText(), width), ""}
	bodyHeight := m.resourceBodyHeight()
	body := m.renderResourceBody(width, bodyHeight)
	if strings.TrimSpace(body) != "" {
		lines = append(lines, strings.Split(body, "\n")...)
	}
	help := renderHelp(width, m.resourceListHelp())
	pageDots := ""
	if m.resourceView == resourceViewCards {
		pageDots = m.resourcePageDots()
	}
	reservedBottomLines := strings.Count(help, "\n") + 1
	if pageDots != "" {
		reservedBottomLines += strings.Count(pageDots, "\n") + 1
	}
	lines = padToBottom(lines, m.height, reservedBottomLines)
	if pageDots != "" {
		lines = append(lines, pageDots)
	}
	lines = append(lines, help)
	return strings.Join(lines, "\n")
}

func (m Model) resourceBodyHeight() int {
	height := m.height - 4
	if height < 1 {
		height = 1
	}
	return height
}

func (m Model) resourceHeaderText() string {
	parts := []string{
		titleStyle.Render(m.t("Resources", "资源")),
		blueStyle.Bold(true).Render(m.resourceHostTitle()),
		m.resourceScopeHeaderText(),
		m.resourceKindHeaderText(),
		mutedStyle.Render(m.resourceTotalHeaderText(len(m.currentResourceRefs()))),
		cardMutedStyle.Render(m.resourceViewName(m.resourceView)),
		m.resourceFilterHeaderText(),
	}
	if refresh := m.resourceRefreshHeaderText(); refresh != "" {
		parts = append(parts, mutedStyle.Render(refresh))
	}
	if m.resourceSearch {
		searchWidth := m.width / 3
		if searchWidth < 8 {
			searchWidth = 8
		}
		parts = append(parts, blueStyle.Render(m.t("Search ", "搜索 ")+inlineCursorText(m.resourceQuery, searchWidth, len([]rune(m.resourceQuery)))))
	} else if strings.TrimSpace(m.resourceQuery) != "" {
		parts = append(parts, blueStyle.Render(m.t("Search ", "搜索 ")+m.resourceQuery))
	}
	if m.resourceLoading {
		parts = append(parts, m.dashboardStatusHeaderText(m.status))
	} else if strings.TrimSpace(m.status) != "" && m.status != m.resourceRefreshStatus {
		parts = append(parts, m.dashboardStatusHeaderText(m.status))
	}
	return strings.Join(parts, "  ")
}

func (m Model) resourceScopeHeaderText() string {
	name := m.resourceScopeName(m.resourceScope)
	if m.resourceScope == resourceScopeManaged {
		return favoriteStyle.Render(name)
	}
	return blueStyle.Render(name)
}

func (m Model) resourceKindHeaderText() string {
	name := m.resourceKindName(m.resourceKind)
	if m.resourceKind == resourceAll {
		return detailValueStyle.Render(name)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(m.resourceKind)).Render(name)
}

func (m Model) resourceFilterHeaderText() string {
	if m.resourceKind == resourcePorts {
		name := m.resourcePortFilterName(m.resourcePortFilter)
		switch m.resourcePortFilter {
		case resourcePortFilterPublic:
			return redStyle.Render(name)
		case resourcePortFilterLoopback:
			return greenStyle.Render(name)
		case resourcePortFilterSpecific:
			return yellowStyle.Render(name)
		case resourcePortFilterContainer, resourcePortFilterProcess:
			return blueStyle.Render(name)
		default:
			return detailValueStyle.Render(name)
		}
	}
	name := m.resourceFilterName(m.resourceFilter)
	switch m.resourceFilter {
	case resourceFilterRunning:
		return greenStyle.Render(name)
	case resourceFilterProblems:
		return redStyle.Render(name)
	case resourceFilterStopped:
		return mutedStyle.Render(name)
	default:
		return detailValueStyle.Render(name)
	}
}

func (m Model) resourceTotalHeaderText(total int) string {
	if m.isChineseUI() {
		return fmt.Sprintf("%d项", total)
	}
	if total == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", total)
}

func (m Model) resourceCollectedText() string {
	t := m.resourceCollectedAt
	switch m.resourceKind {
	case resourceContainers:
		t = m.resourceContainerAt
	case resourceServices:
		t = m.resourceServiceAt
	case resourcePorts:
		t = m.resourcePortAt
	}
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04:05")
}

func (m Model) resourceRefreshHeaderText() string {
	status := strings.TrimSpace(m.resourceRefreshStatus)
	if status == "" {
		return ""
	}
	prefixes := []string{
		m.t("Manual refresh done: ", "手动刷新完成："),
		m.t("Last refresh: ", "最后刷新："),
	}
	for _, prefix := range prefixes {
		status = strings.TrimPrefix(status, prefix)
	}
	return status
}

func (m Model) renderResourceBody(width int, height int) string {
	if m.resourceLoading && len(m.filteredResourceIndexes()) == 0 {
		text := m.t("Loading resources...", "正在加载资源...")
		if m.resourceLoadingKind != resourceAll {
			text = m.t("Loading ", "正在加载") + m.resourceKindName(m.resourceLoadingKind) + "..."
		}
		if m.resourceView == resourceViewList {
			return m.renderResourceListBox([]string{mutedStyle.Render(text)}, width, height)
		}
		return padBlock(mutedStyle.Render(text), width)
	}
	indexes := m.filteredResourceIndexes()
	if len(indexes) == 0 {
		text := mutedStyle.Render(m.t("No matching resources", "没有匹配的资源"))
		if errText := m.resourceErrorText(); errText != "" {
			text = redStyle.Render(errText)
		}
		if m.resourceView == resourceViewList {
			return m.renderResourceListBox([]string{text}, width, height)
		}
		if errText := m.resourceErrorText(); errText != "" {
			return padBlock(redStyle.Render(errText), width)
		}
		return padBlock(text, width)
	}
	if m.resourceIndex >= len(indexes) {
		m.resourceIndex = len(indexes) - 1
	}
	if m.resourceView == resourceViewList {
		contentHeight := maxInt(1, height-2)
		rows, selectedRow := m.resourceListRows(indexes, width-4)
		lines := []string{}
		start, end := visibleRange(len(rows), selectedRow, contentHeight)
		for i := start; i < end; i++ {
			lines = append(lines, rows[i])
		}
		return m.renderResourceListBox(lines, width, height)
	}
	lines, selectedTop, selectedBottom := m.resourceCardLines(indexes, width)
	start, end := dashboardLineWindow(len(lines), selectedTop, selectedBottom, height)
	return strings.Join(lines[start:end], "\n")
}

func (m Model) renderResourceListBox(lines []string, width int, height int) string {
	contentHeight := maxInt(1, height-2)
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	frameWidth := width
	if frameWidth < 34 {
		frameWidth = 34
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(frameWidth).
		Render(strings.Join(lines, "\n"))
}

func (m Model) resourceErrorText() string {
	return m.resourceErrorTextForKind(m.resourceKind)
}

func (m Model) resourceErrorTextForKind(kind resourceKind) string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return ""
	}
	if kind == resourceContainers {
		return strings.TrimSpace(m.states[m.resourceHostIndex].ContainerError)
	}
	if kind == resourcePorts {
		return strings.TrimSpace(m.states[m.resourceHostIndex].PortDetailsError)
	}
	if kind == resourceProcesses {
		return strings.TrimSpace(m.states[m.resourceHostIndex].PortDetailsError)
	}
	if kind == resourceAll {
		parts := []string{}
		if errText := strings.TrimSpace(m.states[m.resourceHostIndex].ContainerError); errText != "" {
			parts = append(parts, errText)
		}
		if errText := strings.TrimSpace(m.states[m.resourceHostIndex].ServiceError); errText != "" {
			parts = append(parts, errText)
		}
		if errText := strings.TrimSpace(m.states[m.resourceHostIndex].PortDetailsError); errText != "" {
			parts = append(parts, errText)
		}
		return strings.Join(parts, " / ")
	}
	return strings.TrimSpace(m.states[m.resourceHostIndex].ServiceError)
}

func (m Model) resourceCardLines(indexes []resourceRef, width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	for i := 0; i < len(indexes); i += cols {
		rowEnd := minInt(i+cols, len(indexes))
		row := []string{}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= rowEnd {
				continue
			}
			visible := i + col
			if visible == m.resourceIndex {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.resourceCard(indexes[visible], visible == m.resourceIndex, cardWidth), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.resourceIndex >= i && m.resourceIndex < rowEnd {
			selectedBottom = len(lines)
		}
	}
	return lines, selectedTop, selectedBottom
}

func (m Model) resourceCard(ref resourceRef, selected bool, width int) string {
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	if ref.Kind == resourceContainers {
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		kind := containerDetailKind(item)
		title := m.resourceTypeBadge(ref.Kind) + " " + item.Name
		if item.Favorite {
			title = favoriteStyle.Render("★") + " " + title
		}
		dot := coloredContainerStatus("●", kind)
		meta := cardMutedStyle.Render(m.containerCardMeta(item))
		innerWidth := width - 4
		if innerWidth < 30 {
			innerWidth = 30
		}
		cpuLine := resourcePercentMetricLine("CPU", item.CPU, m.containerCPULimitTextForItem(item), innerWidth, 70, 85)
		memLine := resourcePercentMetricLine(m.t("Mem", "内存"), item.MemPerc, item.Memory, innerWidth, 70, 85)
		return strings.Join([]string{
			cardTopLine(width, fitANSI(title, maxInt(8, width-12)), meta, dot, borderStyle),
			cardContentLine(width, m.t("Status", "状态")+"  "+m.containerStatusLine(item), borderStyle),
			cardContentLine(width, cpuLine, borderStyle),
			cardContentLine(width, memLine, borderStyle),
			cardContentLine(width, m.t("Image", "镜像")+"  "+cardMutedStyle.Render(emptyDash(item.Image)), borderStyle),
			cardInnerSeparatorLine(width, borderStyle),
			cardMutedContentLine(width, fitANSI(emptyDash(simplifyDockerPorts(item.Ports)), maxInt(8, width-4)), borderStyle),
			cardBottomLine(width, borderStyle),
		}, "\n")
	}
	if ref.Kind == resourcePorts {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		title := m.resourceTypeBadge(ref.Kind) + " " + fmt.Sprintf("%s/%s", item.Protocol, item.Port)
		dot := greenStyle.Render("●")
		if item.Missing {
			dot = mutedStyle.Render("●")
		}
		if item.Favorite {
			title = favoriteStyle.Render("★") + " " + title
		}
		process := emptyDash(item.Process)
		if item.PID != "" {
			process += "  pid:" + item.PID
		}
		return strings.Join([]string{
			cardTopLine(width, fitANSI(title, maxInt(8, width-12)), "", dot, borderStyle),
			cardContentLine(width, m.t("Status", "状态")+"  "+m.portStatusLine(item), borderStyle),
			cardContentLine(width, m.t("Listen", "监听")+"  "+cardMutedStyle.Render(emptyDash(portListenText(item))), borderStyle),
			cardContentLine(width, m.t("Process", "进程")+"  "+cardMutedStyle.Render(process), borderStyle),
			cardContentLine(width, m.t("Container", "容器")+"  "+cardMutedStyle.Render(emptyDash(item.Container)), borderStyle),
			cardInnerSeparatorLine(width, borderStyle),
			cardMutedContentLine(width, fitANSI(m.portScopeText(item)+"  "+m.portRiskText(item), maxInt(8, width-4)), borderStyle),
			cardBottomLine(width, borderStyle),
		}, "\n")
	}
	if ref.Kind == resourceProcesses {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		title := m.resourceTypeBadge(ref.Kind) + " " + emptyDash(item.Process)
		if item.ProcessFavorite {
			title = favoriteStyle.Render("★") + " " + title
		}
		extra, hasExtra := m.processExtraForCard(item)
		meta := cardMutedStyle.Render(m.processCardMeta(extra, hasExtra))
		executable := m.processCardExecutable(item, extra, hasExtra)
		commandLine := m.processCardCommandLine(item, extra, hasExtra)
		return strings.Join([]string{
			cardTopLine(width, fitANSI(title, maxInt(8, width-12)), meta, m.processCardDot(extra, hasExtra), borderStyle),
			cardContentLine(width, m.t("Status", "状态")+"  "+m.processCardStatusLine(item, extra, hasExtra), borderStyle),
			cardContentLine(width, m.processCardResourceLine(item, extra, hasExtra), borderStyle),
			cardContentLine(width, m.t("Listen", "监听")+"  "+cardMutedStyle.Render(emptyDash(portListenText(item))), borderStyle),
			cardMutedContentLine(width, emptyDash(executable), borderStyle),
			cardInnerSeparatorLine(width, borderStyle),
			cardMutedContentLine(width, emptyDash(commandLine), borderStyle),
			cardBottomLine(width, borderStyle),
		}, "\n")
	}
	item := m.mergedServiceDetail(m.states[m.resourceHostIndex].ServiceDetails[ref.Index])
	kind := serviceDetailKind(item)
	title := m.resourceTypeBadge(ref.Kind) + " " + item.Unit
	if item.Favorite {
		title = favoriteStyle.Render("★") + " " + title
	}
	dot := coloredServiceStatus("●", kind)
	meta := cardMutedStyle.Render(m.serviceCardMeta(item))
	stateLine := emptyDash(serviceRawState(item))
	lines := []string{
		cardTopLine(width, fitANSI(title, maxInt(8, width-12)), meta, dot, borderStyle),
		cardContentLine(width, m.t("Status", "状态")+"  "+coloredServiceStatus(m.serviceStatusText(item), kind)+"  "+coloredServiceStatus(stateLine, kind), borderStyle),
	}
	lines = append(lines,
		cardContentLine(width, serviceCardResourceLine(m, item), borderStyle),
		cardMutedContentLine(width, fitANSI(emptyDash(serviceProgramPath(item)), maxInt(8, width-4)), borderStyle),
		cardContentLine(width, m.t("Enabled", "自启")+"  "+cardMutedStyle.Render(emptyDash(m.serviceUnitFileStateText(item.UnitFileState))), borderStyle),
		cardInnerSeparatorLine(width, borderStyle),
		cardMutedContentLine(width, fitANSI(emptyDash(item.Description), maxInt(8, width-4)), borderStyle),
		cardBottomLine(width, borderStyle),
	)
	return strings.Join(lines, "\n")
}

func (m Model) resourceTypeBadge(kind resourceKind) string {
	label := ""
	mark := "◆"
	switch kind {
	case resourceContainers:
		label = m.t("[Container]", "[容器]")
		mark = "🐳"
	case resourceServices:
		label = m.t("[Service]", "[服务]")
		mark = "●"
	case resourceProcesses:
		label = m.t("[Process]", "[进程]")
		mark = "■"
	case resourcePorts:
		label = m.t("[Port]", "[端口]")
		mark = "▲"
	default:
		label = m.t("[Resource]", "[资源]")
	}
	return lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(kind)).Render(mark + " " + label)
}

func resourceKindColor(kind resourceKind) lipgloss.Color {
	switch kind {
	case resourceContainers:
		return blue
	case resourceServices:
		return green
	case resourceProcesses:
		return lipgloss.Color("201")
	case resourcePorts:
		return yellow
	default:
		return valueGray
	}
}

func (m Model) resourcePageDots() string {
	indexes := m.filteredResourceIndexes()
	if len(indexes) == 0 {
		return ""
	}
	lines, selectedTop, selectedBottom := m.resourceCardLines(indexes, contentWidth(m.width))
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, m.resourceBodyHeight(), m.width)
}

func (m Model) resourceListRows(indexes []resourceRef, width int) ([]string, int) {
	rows := []string{}
	selectedRow := 0
	lastKind := resourceKind(-1)
	for i, ref := range indexes {
		if ref.Kind != lastKind {
			if len(rows) > 0 {
				rows = append(rows, "")
			}
			rows = append(rows, m.resourceListGroupHeader(ref.Kind, width))
			lastKind = ref.Kind
		}
		if i == m.resourceIndex {
			selectedRow = len(rows)
		}
		rows = append(rows, m.resourceListLine(ref, i == m.resourceIndex, width))
	}
	return rows, selectedRow
}

func (m Model) resourceListGroupHeader(kind resourceKind, width int) string {
	label := lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(kind)).Render(m.resourceKindName(kind))
	count := 0
	for _, ref := range m.filteredResourceIndexes() {
		if ref.Kind == kind {
			count++
		}
	}
	text := fmt.Sprintf("%s %d", label, count)
	return fitANSI(text, width)
}

func (m Model) resourceListLine(ref resourceRef, selected bool, width int) string {
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	nameWidth, statusWidth, infoWidth := resourceListColumnWidths(width)
	if ref.Kind == resourceContainers {
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		kind := containerDetailKind(item)
		displayName := m.resourceTypeBadge(ref.Kind) + " " + item.Name
		return m.resourceListColumns(prefix, item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), coloredContainerStatus(emptyDash(item.Status), kind), containerResourceText(item), firstNonEmpty(item.Image, simplifyDockerPorts(item.Ports)), width, nameWidth, statusWidth, infoWidth)
	}
	if ref.Kind == resourcePorts {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		displayName := m.resourceTypeBadge(ref.Kind) + " " + fmt.Sprintf("%s/%s", item.Protocol, item.Port)
		return m.resourceListColumns(prefix, item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), m.portStatusStyled(item), portListenText(item), portProcessDetailText(item), width, nameWidth, statusWidth, infoWidth)
	}
	if ref.Kind == resourceProcesses {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		displayName := m.resourceTypeBadge(ref.Kind) + " " + emptyDash(item.Process)
		return m.resourceListColumns(prefix, item.ProcessFavorite, nameStyle.Render(fitANSI(displayName, nameWidth)), portListenText(item), "PID "+emptyDash(item.PID), m.portRiskText(item), width, nameWidth, statusWidth, infoWidth)
	}
	item := m.states[m.resourceHostIndex].ServiceDetails[ref.Index]
	kind := serviceDetailKind(item)
	displayName := m.resourceTypeBadge(ref.Kind) + " " + item.Unit
	return m.resourceListColumns(prefix, item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), coloredServiceStatus(m.serviceStatusText(item), kind), serviceResourceText(item), m.serviceUnitFileStateText(item.UnitFileState), width, nameWidth, statusWidth, infoWidth)
}

func resourceListColumnWidths(width int) (int, int, int) {
	nameWidth := 30
	statusWidth := 18
	infoWidth := 24
	if width < 92 {
		nameWidth = 24
		statusWidth = 14
		infoWidth = 20
	}
	if width < 72 {
		nameWidth = 20
		statusWidth = 12
		infoWidth = 16
	}
	return nameWidth, statusWidth, infoWidth
}

func (m Model) resourceListColumns(prefix string, favorite bool, name string, status string, info string, extra string, width int, nameWidth int, statusWidth int, infoWidth int) string {
	favoriteMark := " "
	if favorite {
		favoriteMark = favoriteStyle.Render("★")
	}
	name = padVisible(fitANSI(name, nameWidth), nameWidth)
	status = padVisible(fitANSI(status, statusWidth), statusWidth)
	info = cardMutedStyle.Render(padVisible(fitANSI(info, infoWidth), infoWidth))
	used := 2 + 1 + 1 + 1 + 1 + nameWidth + 2 + statusWidth + 2 + infoWidth + 2
	extraWidth := maxInt(8, width-used)
	extra = cardMutedStyle.Render(fitANSI(extra, extraWidth))
	return fitANSI(fmt.Sprintf("  %s %s %s  %s  %s  %s", prefix, favoriteMark, name, status, info, extra), width)
}

func containerMemoryText(item containerDetail) string {
	memory := strings.TrimSpace(item.Memory)
	percent := strings.TrimSpace(item.MemPerc)
	switch {
	case memory != "" && percent != "":
		return memory + " " + percent
	case memory != "":
		return memory
	case percent != "":
		return percent
	default:
		return "-"
	}
}

func containerResourceText(item containerDetail) string {
	return "CPU " + emptyDash(item.CPU) + "  MEM " + containerMemoryText(item)
}

func (m Model) containerStatusLine(item containerDetail) string {
	kind := containerDetailKind(item)
	label := m.containerStatusLabel(item)
	raw := emptyDash(item.Status)
	return coloredContainerStatus(label, kind) + "  " + coloredContainerStatus(raw, kind)
}

func (m Model) containerStatusLabel(item containerDetail) string {
	switch containerDetailKind(item) {
	case "missing":
		return m.t("Not found", "未发现")
	case "failed":
		return m.t("Problem", "异常")
	case "running":
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if strings.Contains(status, "healthy") && !strings.Contains(status, "unhealthy") {
			return m.t("Healthy", "健康")
		}
		return m.t("Running", "运行")
	case "stopped":
		return m.t("Stopped", "停止")
	default:
		return m.t("Unknown", "未知")
	}
}

func containerDetailPercentText(percentText string, extra string, warn float64, crit float64) string {
	value, ok := parsePercentText(percentText)
	if !ok {
		return emptyDash(extra)
	}
	text := percentBarWithThreshold(value, warn, crit)
	if strings.TrimSpace(extra) != "" {
		text += "  " + cardMutedStyle.Render(extra)
	}
	return text
}

func (m Model) containerCPULimitText() string {
	if m.resourceContainerExtraLoading {
		return m.t("Loading", "加载中")
	}
	if strings.TrimSpace(m.resourceContainerExtraErr) != "" {
		return "-"
	}
	d := m.resourceContainerExtra
	return m.containerCPULimitTextFromFields(d.NanoCpus, d.CPUQuota, d.CPUPeriod, d.CpusetCpus)
}

func (m Model) containerCPULimitTextForItem(item containerDetail) string {
	if item.CPULimitKnown {
		return m.containerCPULimitTextFromFields(item.NanoCpus, item.CPUQuota, item.CPUPeriod, item.CpusetCpus)
	}
	if m.resourceContainerExtraName == item.Name && !m.resourceContainerExtraLoading && strings.TrimSpace(m.resourceContainerExtraErr) == "" {
		return m.containerCPULimitText()
	}
	return ""
}

func (m Model) containerCPULimitTextFromFields(nanoCpus int64, cpuQuota int64, cpuPeriod int64, cpusetCpus string) string {
	if strings.TrimSpace(cpusetCpus) != "" {
		return "CPU " + cpusetCpus
	}
	if nanoCpus > 0 {
		return m.cpuLimitCoresText(float64(nanoCpus) / 1_000_000_000)
	}
	if cpuQuota > 0 && cpuPeriod > 0 {
		return m.cpuLimitCoresText(float64(cpuQuota) / float64(cpuPeriod))
	}
	return m.t("Unlimited", "未限制")
}

func (m Model) cpuLimitCoresText(cores float64) string {
	if cores <= 0 {
		return m.t("Unlimited", "未限制")
	}
	text := fmt.Sprintf("%.2f", cores)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if m.isChineseUI() {
		return text + "核限制"
	}
	if text == "1" {
		return "1 core limit"
	}
	return text + " cores limit"
}

func (m Model) containerCardMeta(item containerDetail) string {
	raw := strings.TrimSpace(item.Status)
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "unhealthy") || strings.HasPrefix(lower, "up "):
		return m.dockerStatusDashboardMeta(raw, "Up")
	case strings.HasPrefix(lower, "restarting"), strings.HasPrefix(lower, "exited"):
		return m.dockerStatusDashboardMeta(raw, "")
	case strings.HasPrefix(lower, "created"):
		return m.dockerStatusDashboardMeta(raw, "Created")
	default:
		return ""
	}
}

func (m Model) dockerStatusDashboardMeta(status string, prefix string) string {
	d, ok := dockerStatusDuration(status, prefix)
	if !ok {
		return ""
	}
	return m.dashboardDurationShort(d)
}

func dockerStatusDuration(status string, prefix string) (time.Duration, bool) {
	status = strings.TrimSpace(status)
	status = strings.TrimPrefix(status, prefix)
	if idx := strings.Index(status, "("); idx >= 0 {
		status = status[:idx]
	}
	if idx := strings.LastIndex(status, ")"); idx >= 0 && idx < len(status)-1 {
		status = status[idx+1:]
	}
	status = strings.TrimPrefix(strings.TrimSpace(status), "Created ")
	status = strings.TrimSuffix(strings.TrimSpace(status), "ago")
	fields := strings.Fields(strings.TrimSpace(status))
	if len(fields) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 0 {
		return 0, false
	}
	unit := strings.ToLower(fields[1])
	switch {
	case strings.HasPrefix(unit, "second"):
		return time.Duration(n) * time.Second, true
	case strings.HasPrefix(unit, "minute"):
		return time.Duration(n) * time.Minute, true
	case strings.HasPrefix(unit, "hour"):
		return time.Duration(n) * time.Hour, true
	case strings.HasPrefix(unit, "day"):
		return time.Duration(n) * 24 * time.Hour, true
	case strings.HasPrefix(unit, "week"):
		return time.Duration(n) * 7 * 24 * time.Hour, true
	case strings.HasPrefix(unit, "month"):
		return time.Duration(n) * 30 * 24 * time.Hour, true
	case strings.HasPrefix(unit, "year"):
		return time.Duration(n) * 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func (m Model) dashboardDurationShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	if minutes < 60 {
		if m.isChineseUI() {
			return fmt.Sprintf("%d分", minutes)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		if m.isChineseUI() {
			return fmt.Sprintf("%d时", hours)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if m.isChineseUI() {
		return fmt.Sprintf("%d天", days)
	}
	return fmt.Sprintf("%dd", days)
}

func resourcePercentMetricLine(label string, percentText string, extra string, width int, warn float64, crit float64) string {
	value, ok := parsePercentText(percentText)
	if !ok {
		return cardMetricLine(label, cardMutedStyle.Render(" -"), emptyDash(extra), width)
	}
	barWidth := 8
	if width >= 42 {
		barWidth = 10
	}
	return cardMetricLine(label, percentBarWidthWithThreshold(value, barWidth, warn, crit), emptyDash(extra), width)
}

func parsePercentText(value string) (float64, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func servicePIDText(item serviceDetail) string {
	if strings.TrimSpace(item.MainPID) != "" {
		return item.MainPID
	}
	return item.ExecMainPID
}

func serviceMemoryText(item serviceDetail) string {
	return bytesHuman(item.MemoryCurrent)
}

func serviceResourceText(item serviceDetail) string {
	return "PID " + emptyDash(servicePIDText(item)) + "  MEM " + serviceMemoryText(item)
}

func serviceCardResourceLine(m Model, item serviceDetail) string {
	parts := []string{}
	if pid := strings.TrimSpace(servicePIDText(item)); pid != "" {
		parts = append(parts, "PID  "+cardMutedStyle.Render(pid))
	}
	if item.MemoryCurrent > 0 {
		parts = append(parts, m.t("Memory", "内存")+"  "+cardMutedStyle.Render(serviceMemoryText(item)))
	}
	if len(parts) == 0 {
		return m.t("Resource", "资源") + "  " + cardMutedStyle.Render("-")
	}
	return strings.Join(parts, "  ")
}

func (m Model) serviceCardMeta(item serviceDetail) string {
	if serviceDetailKind(item) != "running" && serviceDetailKind(item) != "active" {
		return ""
	}
	for _, value := range []string{item.ActiveSince, item.ExecStartedAt, item.StateChangedAt} {
		t, ok := parseSystemdTimestamp(value)
		if ok {
			d := time.Since(t)
			if d < 0 {
				return ""
			}
			return m.dashboardDurationShort(d)
		}
	}
	return ""
}

func (m Model) serviceStartText(item serviceDetail) string {
	for _, value := range []string{item.ActiveSince, item.ExecStartedAt, item.StateChangedAt} {
		text := shortSystemdTimestampAge(value, m.isChineseUI())
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return "-"
}

func (m Model) serviceUnitFileStateText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !m.isChineseUI() {
		return value
	}
	switch strings.ToLower(value) {
	case "enabled":
		return "已启用"
	case "disabled":
		return "未启用"
	case "static":
		return "依赖启动"
	case "masked":
		return "已屏蔽"
	case "generated":
		return "生成"
	case "transient":
		return "临时"
	case "indirect":
		return "间接"
	case "enabled-runtime":
		return "运行时启用"
	case "linked", "linked-runtime":
		return "已链接"
	default:
		return value
	}
}

func (m Model) serviceSourceText(item serviceDetail) string {
	if strings.TrimSpace(item.WorkingDirectory) != "" {
		return m.t("Dir ", "目录 ") + item.WorkingDirectory
	}
	if strings.TrimSpace(item.FragmentPath) != "" {
		return m.t("Source ", "来源 ") + item.FragmentPath
	}
	return "-"
}

func serviceProgramPath(item serviceDetail) string {
	execStart := strings.TrimSpace(item.ExecStart)
	if execStart == "" {
		return ""
	}
	normalized := strings.NewReplacer(";", " ", "{", " ", "}", " ").Replace(execStart)
	for _, token := range strings.Fields(normalized) {
		token = strings.Trim(token, "\"'(),")
		if strings.HasPrefix(token, "path=") {
			token = strings.TrimPrefix(token, "path=")
		}
		if strings.HasPrefix(token, "argv[]=") {
			token = strings.TrimPrefix(token, "argv[]=")
		}
		if strings.HasPrefix(token, "/") {
			return token
		}
	}
	return ""
}

func (m Model) renderResourceDetail() string {
	width := detailFrameWidth(m.width)
	lines := expandLines(m.resourceDetailLines())
	bodyHeight := m.height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	maxScroll := maxInt(0, len(lines)-bodyHeight)
	m.resourceScroll = clampInt(m.resourceScroll, 0, maxScroll)
	if len(lines) > bodyHeight {
		lines = lines[m.resourceScroll : m.resourceScroll+bodyHeight]
	}
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	help := renderHelp(width, m.resourceDetailHelp())
	return strings.Join([]string{titleStyle.Render(fitANSI(m.resourceDetailTitle(), width)), body, help}, "\n")
}

func (m Model) resourceDetailHelp() string {
	partsEN := []string{"Scroll ↑↓/jk"}
	partsZH := []string{"滚动 ↑↓/jk"}
	if m.selectedResourceManaged() {
		partsEN = append(partsEN, "Edit e")
		partsZH = append(partsZH, "编辑 e")
	}
	partsEN = append(partsEN, "Remove x")
	partsZH = append(partsZH, "移出 x")
	switch m.currentSelectedResourceKind() {
	case resourcePorts:
	case resourceProcesses, resourceServices, resourceContainers:
		partsEN = append(partsEN, "Logs o", "Start s", "Stop p", "Restart r")
		partsZH = append(partsZH, "日志 o", "启动 s", "停止 p", "重启 r")
	}
	partsEN = append(partsEN, "Back Space/Esc")
	partsZH = append(partsZH, "返回 Space/Esc")
	return m.t(strings.Join(partsEN, "  "), strings.Join(partsZH, "  "))
}

func (m Model) resourceDetailTitle() string {
	kind := m.currentSelectedResourceKind()
	title := fmt.Sprintf("%s  %s", m.t(m.resourceKindName(kind)+" Detail", m.resourceKindName(kind)+"详情"), m.resourceHostTitle())
	name := m.resourceDetailName
	if strings.TrimSpace(name) == "" {
		name = m.currentSelectedResourceName()
	}
	if name != "" {
		title += "  " + name
	}
	return title
}

func (m Model) currentSelectedResourceName() string {
	if ref, ok := m.selectedResourceRef(); ok {
		switch ref.Kind {
		case resourceContainers:
			if item, ok := m.selectedContainer(); ok {
				return item.Name
			}
		case resourceServices:
			if item, ok := m.selectedService(); ok {
				return item.Unit
			}
		case resourceProcesses:
			if item, ok := m.selectedProcess(); ok {
				return item.Process
			}
		case resourcePorts:
			if item, ok := m.selectedPort(); ok {
				return item.Protocol + "/" + item.Port
			}
		}
	}
	return ""
}

func (m Model) resourceDetailLines() []string {
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceContainers {
		item, ok := m.selectedContainer()
		if !ok {
			return []string{m.t("No selected container", "没有选中的容器")}
		}
		lines := []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceContainers)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Name", "名称"), item.Name),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
			m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
			m.detailRow(m.t("Status", "状态"), item.Status),
			m.detailRow(m.t("Image", "镜像"), item.Image),
			m.detailRow(m.t("Ports", "端口"), emptyDash(simplifyDockerPorts(item.Ports))),
			"",
			sectionTitle(m.t("Resources", "资源监控")),
			m.detailRow(m.t("CPU", "CPU"), containerDetailPercentText(item.CPU, m.containerCPULimitTextForItem(item), 70, 85)),
			m.detailRow(m.t("Memory", "内存"), containerDetailPercentText(item.MemPerc, item.Memory, 70, 85)),
		}
		lines = append(lines, m.containerExtraDetailLines()...)
		lines = append(lines,
			"",
			sectionTitle(m.t("Actions", "操作")),
			m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceContainers, resourceActionStart, item.Name)),
			m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceContainers, resourceActionStop, item.Name)),
			m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceContainers, resourceActionRestart, item.Name)),
			m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceContainers, item.Name, 200)),
		)
		return lines
	}
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourcePorts {
		item, ok := m.selectedPort()
		if !ok {
			return []string{m.t("No selected port", "没有选中的端口")}
		}
		return []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourcePorts)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Protocol", "协议"), item.Protocol),
			m.detailRow(m.t("Port", "端口"), item.Port),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
			m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
			m.detailRow(m.t("Status", "状态"), m.portStatusStyled(item)),
			m.detailRow(m.t("Socket state", "连接状态"), emptyDash(item.State)),
			m.detailRow(m.t("Listen", "监听地址"), emptyDash(item.LocalAddress)),
			m.detailRow(m.t("Remote", "远端地址"), emptyDash(item.ForeignAddress)),
			m.detailRow(m.t("Scope", "监听范围"), m.portScopeText(item)),
			m.detailRow(m.t("Scope note", "范围说明"), m.portScopeNote(item)),
			m.detailRow(m.t("IP version", "IP版本"), emptyDash(portIPVersion(item.LocalAddress))),
			m.detailRow(m.t("Risk", "风险"), m.portRiskText(item)),
			m.detailRow(m.t("Risk note", "风险说明"), m.portRiskNote(item)),
			"",
			sectionTitle(m.t("Process", "进程")),
			m.detailRow(m.t("Process", "进程"), emptyDash(item.Process)),
			m.detailRow("PID", emptyDash(item.PID)),
			m.detailRow("FD", emptyDash(item.FD)),
			m.detailRow(m.t("Service", "服务"), emptyDash(item.ServiceUnit)),
			m.detailRow(m.t("Instances", "实例数"), fmt.Sprintf("%d", item.Count)),
			"",
			sectionTitle("Docker"),
			m.detailRow(m.t("Container", "容器"), emptyDash(item.Container)),
			m.detailRow(m.t("Container port", "容器端口"), emptyDash(item.ContainerPort)),
		}
	}
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceProcesses {
		item, ok := m.selectedProcess()
		if !ok {
			return []string{m.t("No selected process", "没有选中的进程")}
		}
		lines := []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceProcesses)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Process", "进程"), emptyDash(item.Process)),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.ProcessFavorite)),
			m.detailRow("PID", emptyDash(item.PID)),
			m.detailRow("FD", emptyDash(item.FD)),
			m.detailRow(m.t("Service", "服务"), emptyDash(item.ServiceUnit)),
			m.detailRow(m.t("Status", "状态"), m.processStatusLine(item)),
			"",
			sectionTitle(m.t("Listener", "监听")),
			m.detailRow(m.t("Protocol", "协议"), item.Protocol),
			m.detailRow(m.t("Port", "端口"), item.Port),
			m.detailRow(m.t("Socket state", "连接状态"), emptyDash(item.State)),
			m.detailRow(m.t("Listen", "监听地址"), emptyDash(item.LocalAddress)),
			m.detailRow(m.t("Remote", "远端地址"), emptyDash(item.ForeignAddress)),
			m.detailRow(m.t("Scope", "监听范围"), m.portScopeText(item)),
			m.detailRow(m.t("Scope note", "范围说明"), m.portScopeNote(item)),
			m.detailRow(m.t("Risk", "风险"), m.portRiskText(item)),
			m.detailRow(m.t("Risk note", "风险说明"), m.portRiskNote(item)),
		}
		lines = append(lines, m.processExtraDetailLines(item)...)
		lines = append(lines,
			"",
			sectionTitle(m.t("Actions", "操作")),
			m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceProcesses, resourceActionStart, item.Process)),
			m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceProcesses, resourceActionStop, item.Process)),
			m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceProcesses, resourceActionRestart, item.Process)),
			m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceProcesses, item.Process, 200)),
		)
		return lines
	}
	item, ok := m.selectedService()
	if !ok {
		return []string{m.t("No selected service", "没有选中的服务")}
	}
	item = m.mergedServiceDetail(item)
	return m.serviceDetailLines(item)
}

func (m Model) serviceDetailLines(item serviceDetail) []string {
	lines := []string{
		sectionTitle(m.t("Basic", "基础信息")),
		m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceServices)),
		m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
		m.detailRow(m.t("Unit", "服务名"), item.Unit),
		m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
		m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
		m.detailRow(m.t("Status", "状态"), coloredServiceStatus(serviceRawState(item), serviceDetailKind(item))),
		m.detailRow(m.t("Status note", "状态说明"), m.serviceStateNote(item)),
		m.detailRow(m.t("Summary", "摘要"), coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))),
		m.detailRow(m.t("Load", "加载"), emptyDash(item.Load)),
		m.detailRow(m.t("Load note", "加载说明"), m.serviceLoadNote(item)),
	}
	if row := m.serviceDetailLoadRow(); strings.TrimSpace(row) != "" {
		lines = append(lines, row)
	}
	result := serviceResultStyle(item.Result)
	if strings.TrimSpace(result) == "" {
		result = emptyDash(item.Result)
	}
	exitCode := serviceExitStyle(item.ExecMainStatus)
	if strings.TrimSpace(exitCode) == "" {
		exitCode = emptyDash(item.ExecMainStatus)
	}
	lines = append(lines,
		m.detailRow(m.t("Enabled", "开机启动"), emptyDash(item.UnitFileState)),
		m.detailRow(m.t("Result", "结果"), result),
		m.detailRow(m.t("Exit code", "退出码"), exitCode),
		m.detailRow(m.t("Restarts", "重启次数"), emptyDash(item.NRestarts)),
		m.detailRow(m.t("Desc", "说明"), emptyDash(item.Description)),
		"",
		sectionTitle(m.t("Resources", "资源监控")),
		m.detailRow("PID", emptyDash(servicePIDText(item))),
		m.detailRow(m.t("Memory", "内存"), serviceMemoryText(item)),
		m.detailRow(m.t("Tasks", "任务数"), emptyDash(item.TasksCurrent)),
		m.detailRow(m.t("Control group", "控制组"), emptyDash(item.ControlGroup)),
		m.detailRow("Slice", emptyDash(item.Slice)),
		"",
		sectionTitle(m.t("Time", "时间")),
		m.detailRow(m.t("Active since", "启动时间"), emptyDash(formatSystemdTimestamp(item.ActiveSince))),
		m.detailRow(m.t("Inactive since", "停止时间"), emptyDash(formatSystemdTimestamp(item.InactiveSince))),
		m.detailRow(m.t("State changed", "状态变化"), emptyDash(formatSystemdTimestamp(item.StateChangedAt))),
		m.detailRow(m.t("Process start", "进程启动"), emptyDash(formatSystemdTimestamp(item.ExecStartedAt))),
		m.detailRow(m.t("Process exit", "进程退出"), emptyDash(formatSystemdTimestamp(item.ExecExitedAt))),
		"",
		sectionTitle(m.t("Startup", "启动配置")),
		m.detailRow(m.t("User", "用户"), emptyDash(item.User)),
		m.detailRow(m.t("Group", "用户组"), emptyDash(item.Group)),
		m.detailRow(m.t("Restart", "重启策略"), emptyDash(item.Restart)),
		m.detailRow(m.t("Restart delay", "重启延迟"), emptyDash(serviceRestartDelayText(item.RestartSec))),
		m.detailRow(m.t("Source", "来源"), emptyDash(item.FragmentPath)),
		m.detailRow("Drop-in", emptyDash(item.DropInPaths)),
		m.detailRow(m.t("Workdir", "工作目录"), emptyDash(item.WorkingDirectory)),
		m.detailRow(m.t("Program", "程序路径"), emptyDash(serviceProgramPath(item))),
		m.detailRow(m.t("Exec", "启动"), emptyDash(item.ExecStart)),
		m.detailRow(m.t("Stop", "停止"), emptyDash(item.ExecStop)),
		m.detailRow(m.t("Reload", "重载"), emptyDash(item.ExecReload)),
		"",
		sectionTitle(m.t("Actions", "操作")),
		m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceServices, resourceActionStart, item.Unit)),
		m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceServices, resourceActionStop, item.Unit)),
		m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceServices, resourceActionRestart, item.Unit)),
		m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceServices, item.Unit, 200)),
	)
	return lines
}

func (m Model) processExtraDetailLines(item portDetail) []string {
	if strings.TrimSpace(item.PID) == "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), "-")}
	}
	if m.resourceProcessExtraPID != item.PID {
		return nil
	}
	if m.resourceProcessExtraLoading {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(m.resourceProcessExtraErr) != "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), redStyle.Render(m.resourceProcessExtraErr))}
	}
	d := m.resourceProcessExtra
	lines := []string{
		"",
		sectionTitle(m.t("Runtime", "运行信息")),
		m.detailRow(m.t("User", "用户"), emptyDash(d.User)),
		m.detailRow(m.t("Parent PID", "父进程"), emptyDash(d.PPID)),
		m.detailRow(m.t("State", "进程状态"), emptyDash(d.State)),
		m.detailRow("CPU", emptyDash(percentSuffix(d.CPU))),
		m.detailRow(m.t("Memory", "内存"), processMemoryText(d)),
		m.detailRow("RSS", processRSSText(d.RSS)),
		m.detailRow(m.t("Elapsed", "运行时长"), emptyDash(d.Elapsed)),
		m.detailRow(m.t("Started", "启动时间"), emptyDash(d.Started)),
		m.detailRow(m.t("Command", "命令"), emptyDash(d.Command)),
		"",
		sectionTitle(m.t("Paths", "路径")),
		m.detailRow(m.t("Executable", "可执行文件"), emptyDash(d.Executable)),
		m.detailRow(m.t("Workdir", "工作目录"), emptyDash(d.WorkingDir)),
		m.detailRow(m.t("Command line", "命令行"), emptyDash(d.CommandLine)),
		"",
		sectionTitle(m.t("Ownership", "归属")),
		m.detailRow(m.t("Service", "服务"), emptyDash(firstNonEmpty(d.ServiceUnit, item.ServiceUnit))),
		m.detailRow(m.t("Control group", "控制组"), emptyDash(d.ControlGroup)),
	}
	return lines
}

func (m Model) serviceLoadNote(item serviceDetail) string {
	switch strings.ToLower(strings.TrimSpace(item.Load)) {
	case "loaded":
		return m.t("Unit file was found and loaded by systemd.", "systemd 已找到并加载这个服务配置文件。")
	case "not-found":
		return m.t("The unit file cannot be found.", "找不到这个服务对应的 unit 配置文件。")
	case "masked":
		return m.t("The service is masked and cannot be started normally.", "这个服务已被屏蔽，通常不能正常启动。")
	case "bad-setting":
		return m.t("The unit file has invalid settings.", "这个服务配置文件里有无效配置。")
	case "error":
		return m.t("systemd failed to load this unit.", "systemd 加载这个服务失败。")
	case "":
		return m.t("Load state is unavailable.", "没有获取到加载状态。")
	default:
		return m.t("This is systemd LoadState.", "这是 systemd 的 LoadState，表示配置文件加载状态。")
	}
}

func (m Model) serviceStateNote(item serviceDetail) string {
	active := strings.ToLower(strings.TrimSpace(item.Active))
	sub := strings.ToLower(strings.TrimSpace(item.Sub))
	switch {
	case active == "active" && sub == "running":
		return m.t("The service is currently running.", "服务当前正在运行。")
	case active == "active" && sub == "exited":
		return m.t("The service completed its start task and exited successfully.", "服务启动任务已完成并退出，常见于一次性服务。")
	case active == "inactive" && sub == "dead":
		return m.t("The service is stopped.", "服务当前已停止。")
	case active == "failed":
		return m.t("The service failed. Check logs for the reason.", "服务运行失败，需要查看日志确认原因。")
	case active == "activating":
		return m.t("The service is starting.", "服务正在启动中。")
	case active == "deactivating":
		return m.t("The service is stopping.", "服务正在停止中。")
	case active == "":
		return m.t("Runtime state is unavailable.", "没有获取到运行状态。")
	default:
		return m.t("This is systemd ActiveState/SubState.", "这是 systemd 的 ActiveState/SubState，表示当前运行状态。")
	}
}

func (m Model) appendOptionalDetailRow(lines *[]string, label, value, styledValue string) {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return
	}
	if strings.TrimSpace(styledValue) != "" {
		*lines = append(*lines, m.detailRow(label, styledValue))
		return
	}
	*lines = append(*lines, m.detailRow(label, value))
}

func appendDetailSection(lines []string, title string, rows []string) []string {
	rows = compactDetailRows(rows)
	if len(rows) == 0 {
		return lines
	}
	lines = append(lines, "", sectionTitle(title))
	return append(lines, rows...)
}

func compactDetailRows(rows []string) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row) == "" {
			continue
		}
		out = append(out, row)
	}
	return out
}

func isEmptyDetailValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return true
	}
	switch strings.ToLower(value) {
	case "n/a", "<nil>", "[]", "{}":
		return true
	default:
		return false
	}
}

func serviceResultStyle(value string) string {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return ""
	}
	if strings.EqualFold(value, "success") {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func serviceExitStyle(value string) string {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return ""
	}
	if value == "0" {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func (m Model) mergedServiceDetail(item serviceDetail) serviceDetail {
	if m.resourceServiceExtraLoading || strings.TrimSpace(m.resourceServiceExtraErr) != "" || m.resourceServiceExtraName != item.Unit {
		return item
	}
	extra := m.resourceServiceExtra
	if strings.TrimSpace(extra.Unit) == "" {
		return item
	}
	extra.Managed = item.Managed
	extra.Favorite = item.Favorite
	extra.Missing = item.Missing
	return extra
}

func (m Model) serviceDetailLoadRow() string {
	if m.resourceServiceExtraLoading {
		return m.detailRow(m.t("Details", "详情"), m.t("Loading", "加载中"))
	}
	if strings.TrimSpace(m.resourceServiceExtraErr) != "" {
		return m.detailRow(m.t("Details", "详情"), redStyle.Render(m.resourceServiceExtraErr))
	}
	return ""
}

func (m Model) containerExtraDetailLines() []string {
	if m.resourceContainerExtraLoading {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(m.resourceContainerExtraErr) != "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), redStyle.Render(m.resourceContainerExtraErr))}
	}
	d := m.resourceContainerExtra
	if d.ID == "" && d.Size == "" && d.VirtualSize == "" && d.BlockIO == "" && len(d.Mounts) == 0 {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), "-")}
	}
	lines := []string{
		"",
		sectionTitle(m.t("Details", "详情")),
		m.detailRow("ID", shortContainerID(d.ID)),
		m.detailRow(m.t("Created", "创建"), emptyDash(formatContainerTimestamp(d.Created))),
		m.detailRow(m.t("Started", "启动时间"), emptyDash(formatContainerTimestamp(d.StartedAt))),
		m.detailRow(m.t("Last stop", "上次停止"), emptyDash(formatContainerTimestamp(d.FinishedAt))),
		m.detailRow(m.t("State", "状态"), emptyDash(d.StateStatus)),
		m.detailRow(m.t("Health", "健康"), emptyDash(d.HealthStatus)),
		m.detailRow(m.t("Exit code", "退出码"), fmt.Sprintf("%d", d.ExitCode)),
		m.detailRow(m.t("Restart", "重启策略"), emptyDash(d.RestartPolicy)),
		m.detailRow(m.t("Driver", "驱动"), emptyDash(d.Driver)),
		m.detailRow(m.t("Platform", "平台"), emptyDash(d.Platform)),
		m.detailRow(m.t("Command", "命令"), emptyDash(containerCommandText(d))),
		"",
		sectionTitle(m.t("Storage", "存储")),
		m.detailRow(m.t("Writable layer", "可写层"), firstNonEmpty(d.Size, bytesHuman(d.SizeRW))),
		m.detailRow(m.t("Virtual size", "虚拟大小"), firstNonEmpty(d.VirtualSize, bytesHuman(d.SizeRootFS))),
		m.detailRow(m.t("Block IO", "块IO"), emptyDash(d.BlockIO)),
	}
	if len(d.Mounts) > 0 {
		lines = append(lines, "", sectionTitle(m.t("Mounts", "挂载")))
		for i, mount := range d.Mounts {
			mode := "ro"
			if mount.RW {
				mode = "rw"
			}
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				detailSubTitle(fmt.Sprintf("%02d  %s", i+1, emptyDash(mount.Destination))),
				m.detailRow(m.t("Type", "类型"), emptyDash(mount.Type)+"  "+mode),
				m.detailRow(m.t("Source", "来源"), emptyDash(mount.Source)),
			)
		}
	}
	if len(d.Networks) > 0 {
		lines = append(lines, "", sectionTitle(m.t("Networks", "网络")))
		for i, network := range d.Networks {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				detailSubTitle(fmt.Sprintf("%02d  %s", i+1, emptyDash(network.Name))),
				m.detailRow("IP", emptyDash(network.IPAddress)),
				m.detailRow(m.t("Gateway", "网关"), emptyDash(network.Gateway)),
				m.detailRow("MAC", emptyDash(network.MacAddress)),
				m.detailRow(m.t("Aliases", "别名"), emptyDash(strings.Join(network.Aliases, ", "))),
				m.detailRow(m.t("Network ID", "网络ID"), shortContainerID(network.NetworkID)),
				m.detailRow(m.t("Endpoint ID", "端点ID"), shortContainerID(network.EndpointID)),
			)
		}
	}
	return lines
}

func expandLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		out = append(out, parts...)
	}
	return out
}

func shortContainerID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	return emptyDash(value)
}

func formatContainerTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "0001-01-01") {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func formatSystemdTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return ""
	}
	t, ok := parseSystemdTimestamp(value)
	if ok {
		return t.Local().Format("2006-01-02 15:04:05")
	}
	return value
}

func parseSystemdTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return time.Time{}, false
	}
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -07",
		"Mon 2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, true
		}
	}
	fields := strings.Fields(value)
	if len(fields) >= 3 {
		trimmed := strings.Join(fields[:3], " ")
		if t, err := time.ParseInLocation("Mon 2006-01-02 15:04:05", trimmed, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func serviceRestartDelayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return "-"
	}
	if strings.HasSuffix(value, "us") {
		value = strings.TrimSuffix(value, "us")
	}
	us, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	seconds := us / 1_000_000
	text := fmt.Sprintf("%.2fs", seconds)
	return strings.TrimRight(strings.TrimRight(text, "0"), ".")
}

func containerCommandText(d containerExtraDetail) string {
	parts := []string{}
	if strings.TrimSpace(d.Path) != "" {
		parts = append(parts, d.Path)
	}
	parts = append(parts, d.Args...)
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "-" {
			return value
		}
	}
	return "-"
}

func (m Model) renderResourceLog() string {
	width := detailFrameWidth(m.width)
	bodyHeight := m.resourceLogBodyHeight()
	lines := m.resourceLogLines()
	start, end := resourceLogWindowRange(len(lines), m.resourceLogScroll, bodyHeight)
	bodyLines := fitLines(lines[start:end], width-4)
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Height(bodyHeight).
		Render(strings.Join(bodyLines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fitANSI(m.resourceLogHeader(len(lines), start, end), width)),
		body,
		renderHelp(width, m.t("Scroll ↑↓/jk  Refresh r  Back Esc", "滚动 ↑↓/jk  刷新 r  返回 Esc")),
	}, "\n")
}

func (m Model) resourceLogBodyHeight() int {
	// Header, bordered body, and help consume four terminal rows before log content.
	return maxInt(1, m.height-4)
}

func (m Model) resourceLogLines() []string {
	lines := strings.Split(strings.TrimRight(m.resourceLogOutput, "\n"), "\n")
	if len(lines) == 0 || len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return []string{m.t("No log output.", "没有日志输出。")}
	}
	return lines
}

func (m Model) resourceLogMaxScroll() int {
	return maxInt(0, len(m.resourceLogLines())-m.resourceLogBodyHeight())
}

func resourceLogWindowRange(total int, scroll int, height int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if height <= 0 {
		height = 1
	}
	start := clampInt(scroll, 0, maxInt(0, total-height))
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}

func (m Model) resourceLogHeader(total int, start int, end int) string {
	rangeText := fmt.Sprintf("%d-%d/%d", start+1, end, total)
	if total == 0 {
		rangeText = "0/0"
	}
	return strings.Join([]string{
		m.t("Resources", "资源"),
		">",
		m.t("Logs", "日志"),
		m.resourceHostTitle(),
		m.resourceKindName(m.resourceLogKind),
		m.resourceLogName,
		mutedStyle.Render(rangeText),
	}, "  ")
}

func (m Model) renderResourceCommandEdit() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 40 {
		bodyWidth = 40
	}
	lines := []string{
		detailSubTitle(m.t("Resource Commands", "资源命令")),
		m.detailRow(m.t("Resource", "资源"), m.resourceCommandForm.Name),
		m.detailRow(m.t("Type", "类型"), m.resourceKindName(m.resourceCommandForm.Kind)),
		"",
	}
	if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
		lines = append(lines, m.detailRow(m.t("Mode", "模式"), m.t("Read-only resource", "只读资源")))
	}
	for i := 0; i < resourceCommandFieldCount(m.resourceCommandForm.Kind); i++ {
		lines = append(lines, m.resourceCommandFieldLine(i, bodyWidth))
	}
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	start, end := visibleRange(len(lines), maxInt(0, m.resourceCommandField+3), bodyHeight)
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines[start:end], "\n"))
	help := renderHelp(width, m.t("Save Enter  Move Tab/↑↓  Cursor ←→  Back Esc", "保存 Enter  移动 Tab/↑↓  光标 ←→  返回 Esc"))
	return strings.Join([]string{titleStyle.Render(fitANSI(m.t("Edit Resource Commands", "编辑资源命令"), width)), body, help}, "\n")
}

func (m Model) renderResourceAdd() string {
	width := contentWidth(m.width)
	if width < 70 {
		width = 70
	}
	leftWidth := (width - 2) / 2
	rightWidth := width - leftWidth - 2
	bodyHeight := m.height - 4
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	discovered := m.resourceManageDiscoveredRefs()
	favorites := m.resourceManageFavorites()
	leftLines := m.resourceManageDiscoveredLines(discovered, leftWidth-4, bodyHeight)
	rightLines := m.resourceManageFavoriteLines(favorites, rightWidth-4, bodyHeight)
	leftTitle := m.t("Discovered", "发现")
	rightTitle := m.t("Added", "已添加")
	if m.resourceManagePane == 0 {
		leftTitle = blueStyle.Bold(true).Render(leftTitle)
	} else {
		rightTitle = blueStyle.Bold(true).Render(rightTitle)
	}
	left := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(leftWidth).
		Height(bodyHeight).
		Render(strings.Join(append([]string{leftTitle}, leftLines...), "\n"))
	right := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(rightWidth).
		Height(bodyHeight).
		Render(strings.Join(append([]string{rightTitle}, rightLines...), "\n"))
	headerParts := []string{m.t("Resource Manager", "资源管理"), m.resourceHostTitle(), m.resourceManageTypeTabs()}
	if m.resourceManageSearch {
		searchWidth := width / 3
		if searchWidth < 8 {
			searchWidth = 8
		}
		headerParts = append(headerParts, blueStyle.Render(m.t("Search ", "搜索 ")+inlineCursorText(m.resourceManageQuery, searchWidth, len([]rune(m.resourceManageQuery)))))
	} else if strings.TrimSpace(m.resourceManageQuery) != "" {
		headerParts = append(headerParts, blueStyle.Render(m.t("Search ", "搜索 ")+m.resourceManageQuery))
	}
	if collected := m.resourceManageCollectedText(); collected != "" {
		headerParts = append(headerParts, mutedStyle.Render(collected))
	}
	if m.resourceLoading {
		headerParts = append(headerParts, m.dashboardStatusHeaderText(m.status))
	} else if strings.TrimSpace(m.status) != "" && m.status != m.resourceRefreshStatus {
		headerParts = append(headerParts, m.dashboardStatusHeaderText(m.status))
	}
	header := strings.Join(headerParts, "  ")
	help := renderHelp(width, m.t("Move ↑↓/jk  Pane Tab  Type ←→/hl/g  Search /  Add Enter  Remove x  Edit e  Refresh r  Back Esc", "移动 ↑↓/jk  切栏 Tab  类型 ←→/hl/g  搜索 /  添加 Enter  移出 x  编辑 e  刷新 r  返回 Esc"))
	return strings.Join([]string{titleStyle.Render(fitANSI(header, width)), lipgloss.JoinHorizontal(lipgloss.Top, left, right), help}, "\n")
}

func (m Model) resourceManageTypeTabs() string {
	return fmt.Sprintf("%s  %s  %s", m.t("Type", "类型"), lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(m.resourceAddKind)).Render(m.resourceKindName(m.resourceAddKind)), mutedStyle.Render("←→/g"))
}

func (m Model) resourceManageCollectedText() string {
	return m.resourceRefreshHeaderText()
}

func (m Model) resourceAddFieldLine(field int, label string, value string, width int) string {
	prefix := "  "
	style := detailValueStyle
	if m.resourceAddField == field {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	return fitANSI(prefix+style.Render(padVisible(label, 12))+"  "+value, width)
}

func (m Model) resourceManageDiscoveredLines(refs []resourceRef, width int, height int) []string {
	lines := []string{}
	if len(refs) == 0 {
		if m.resourceLoading {
			lines = append(lines, mutedStyle.Render(m.t("Loading ", "正在加载")+m.resourceKindName(m.resourceAddKind)+"..."))
		} else if errText := m.resourceErrorTextForKind(m.resourceAddKind); errText != "" {
			lines = append(lines, redStyle.Render(fitANSI(errText, width)))
		} else {
			lines = append(lines, mutedStyle.Render(m.t("No discovered resources", "暂无发现资源")))
		}
	} else {
		idx := clampInt(m.resourceManageDiscoveredIndex, 0, len(refs)-1)
		start, end := visibleRange(len(refs), idx, maxInt(1, height-1))
		for i := start; i < end; i++ {
			selected := m.resourceManagePane == 0 && i == idx
			lines = append(lines, m.resourceManageRefLine(refs[i], selected, width))
		}
	}
	for len(lines) < maxInt(1, height-1) {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) resourceManageFavoriteLines(items []config.ManagedResource, width int, height int) []string {
	lines := []string{}
	if len(items) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No added resources", "暂无已添加资源")))
	} else {
		idx := clampInt(m.resourceManageFavoriteIndex, 0, len(items)-1)
		start, end := visibleRange(len(items), idx, maxInt(1, height-1))
		for i := start; i < end; i++ {
			selected := m.resourceManagePane == 1 && i == idx
			lines = append(lines, m.resourceManageFavoriteLine(items[i], selected, width))
		}
	}
	for len(lines) < maxInt(1, height-1) {
		lines = append(lines, "")
	}
	return lines
}

func (m Model) resourceManageRefLine(ref resourceRef, selected bool, width int) string {
	prefix := "  "
	style := detailValueStyle
	if selected {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	name, _ := m.resourceNameForRef(ref)
	status := m.resourceManageStatusForRef(ref)
	meta := m.resourceMetaForRef(ref)
	contentWidth := width - ansi.StringWidth(prefix)
	if contentWidth < 12 {
		contentWidth = 12
	}
	statusWidth := m.resourceManageStatusWidth()
	nameWidth := m.resourceManageNameWidth(contentWidth, statusWidth)
	metaWidth := contentWidth - statusWidth - nameWidth
	if metaWidth < 0 {
		metaWidth = 0
	}
	if nameWidth < 12 {
		nameWidth = maxInt(12, contentWidth-statusWidth-1)
		meta = ""
	}
	line := prefix + padVisible(fitANSI(status, statusWidth), statusWidth) + style.Render(padVisible(fitANSI(emptyDash(name), nameWidth), nameWidth)) + mutedStyle.Render(fitANSI(meta, metaWidth))
	return fitANSI(line, width)
}

func (m Model) resourceManageStatusWidth() int {
	if m.isChineseUI() {
		return 8
	}
	return 10
}

func (m Model) resourceManageNameWidth(contentWidth int, statusWidth int) int {
	nameWidth := 30
	if contentWidth < 70 {
		nameWidth = 26
	}
	if contentWidth < 56 {
		nameWidth = 22
	}
	maxNameWidth := contentWidth - statusWidth - 8
	if maxNameWidth < 12 {
		maxNameWidth = contentWidth - statusWidth - 1
	}
	return clampInt(nameWidth, 12, maxNameWidth)
}

func (m Model) resourceManageFavoriteLine(item config.ManagedResource, selected bool, width int) string {
	if ref, ok := m.resourceRefForManagedItem(item); ok {
		return m.resourceManageRefLine(ref, selected, width)
	}
	ref := m.missingResourceRefForManagedItem(item)
	prefix := "  "
	style := detailValueStyle
	if selected {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	status := mutedStyle.Render(m.t("Not found", "未发现"))
	meta := mutedStyle.Render(m.resourceMissingMetaForItem(item))
	contentWidth := width - ansi.StringWidth(prefix)
	statusWidth := m.resourceManageStatusWidth()
	nameWidth := contentWidth - statusWidth - ansi.StringWidth(meta) - 1
	if nameWidth < 12 {
		nameWidth = maxInt(12, contentWidth-statusWidth-1)
		meta = ""
	}
	name := item.Name
	if ref.Kind == resourcePorts {
		name = item.Name
	}
	line := prefix + padVisible(fitANSI(status, statusWidth), statusWidth) + style.Render(fitANSI(emptyDash(name), nameWidth)) + meta
	return fitANSI(line, width)
}

func (m Model) resourceRefForManagedItem(item config.ManagedResource) (resourceRef, bool) {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceRef{}, false
	}
	state := m.states[m.resourceHostIndex]
	switch item.Kind {
	case config.ResourceKindService:
		for i := range state.ServiceDetails {
			if state.ServiceDetails[i].Unit == item.Name {
				return resourceRef{Kind: resourceServices, Index: i}, true
			}
		}
	case config.ResourceKindContainer:
		for i := range state.ContainerDetails {
			if state.ContainerDetails[i].Name == item.Name {
				return resourceRef{Kind: resourceContainers, Index: i}, true
			}
		}
	case config.ResourceKindProcess:
		for i := range state.PortDetails {
			if state.PortDetails[i].Process == item.Name {
				return resourceRef{Kind: resourceProcesses, Index: i}, true
			}
		}
	case config.ResourceKindPort:
		proto, port := splitManagedPortName(item.Name)
		for i := range state.PortDetails {
			if state.PortDetails[i].Protocol == proto && state.PortDetails[i].Port == port {
				return resourceRef{Kind: resourcePorts, Index: i}, true
			}
		}
	}
	return resourceRef{}, false
}

func (m Model) missingResourceRefForManagedItem(item config.ManagedResource) resourceRef {
	return resourceRef{Kind: resourceKindFromConfig(item.Kind), Index: -1}
}

func (m Model) resourceMissingMetaForItem(item config.ManagedResource) string {
	if item.Kind == config.ResourceKindPort {
		return "  " + item.Name
	}
	return ""
}

func (m Model) resourceMetaForRef(ref resourceRef) string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return ""
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		return "  " + m.containerStatusLabel(item)
	case resourceServices:
		item := m.states[m.resourceHostIndex].ServiceDetails[ref.Index]
		return "  " + serviceRawState(item)
	case resourceProcesses:
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return "  " + strings.TrimSpace(item.Protocol+"/"+item.Port)
	case resourcePorts:
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return "  " + emptyDash(portListenText(item))
	default:
		return ""
	}
}

func (m Model) resourceManageStatusForRef(ref resourceRef) string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return ""
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		return coloredContainerStatus(m.containerStatusLabel(item), containerDetailKind(item))
	case resourceServices:
		item := m.states[m.resourceHostIndex].ServiceDetails[ref.Index]
		return coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))
	case resourceProcesses:
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return m.processStatusStyled(item)
	case resourcePorts:
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return m.portStatusStyledLabel(item, m.portStatusLabel(item))
	default:
		return ""
	}
}

func (m Model) processStatusStyled(item portDetail) string {
	if item.Missing {
		return mutedStyle.Render(m.t("Not found", "未发现"))
	}
	if strings.TrimSpace(item.PID) != "" {
		return greenStyle.Render(m.t("Running", "运行"))
	}
	return yellowStyle.Render(m.t("Unknown", "未知"))
}

func (m Model) resourceCommandFieldLine(field int, width int) string {
	label := m.resourceCommandFieldName(field)
	value := m.resourceCommandFieldValue(field)
	inputWidth := width - 18
	if inputWidth < 18 {
		inputWidth = 18
	}
	display := commandInputText(value, m.resourceCommandCursor, m.resourceCommandField == field, inputWidth)
	prefix := "  "
	style := detailValueStyle
	if m.resourceCommandField == field {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	return fitANSI(prefix+style.Render(padVisible(label, 12))+"  "+display, width)
}

func (m Model) resourceCommandFieldName(field int) string {
	if m.resourceCommandForm.Kind == resourcePorts {
		return m.t("Health", "健康检查")
	}
	switch field {
	case 0:
		return m.t("Start", "启动")
	case 1:
		return m.t("Stop", "停止")
	case 2:
		return m.t("Restart", "重启")
	case 3:
		return m.t("Logs", "日志")
	default:
		return "-"
	}
}

func (m Model) renderResourceConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	command := resourceActionCommandPreview(m.resourceActionResource, m.resourceAction, m.resourceActionName)
	lines := []string{
		m.t("Server: ", "服务器：") + m.resourceHostTitle(),
		m.resourceKindName(m.resourceActionResource) + ": " + m.resourceActionName,
		m.t("Action: ", "操作：") + m.resourceActionNameText(m.resourceAction),
		m.t("Command: ", "命令：") + command,
	}
	wrapped := []string{}
	for _, line := range lines {
		wrapped = append(wrapped, wrapPlainLine(line, bodyWidth))
	}
	return m.renderDangerConfirm(m.t("Confirm Resource Action", "确认资源操作"), wrapped, width)
}

func (m Model) renderResourceOutput() string {
	width := detailFrameWidth(m.width)
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	lines := []string{
		"$ " + m.resourceCommandPreview(m.resourceActionResource, m.resourceAction, m.resourceActionName),
		"",
	}
	if m.resourceActionRunning {
		lines = append(lines, m.t("Running...", "执行中..."))
	} else {
		if strings.TrimSpace(m.resourceActionOutput) != "" {
			lines = append(lines, strings.Split(m.resourceActionOutput, "\n")...)
		}
		lines = append(lines, "", fmt.Sprintf("%s %d", m.t("Exit code", "退出码"), m.resourceActionExitCode))
	}
	start, end := visibleRange(len(lines), m.resourceScroll, bodyHeight)
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines[start:end], "\n"))
	return strings.Join([]string{titleStyle.Render(fitANSI(m.t("Resource Output", "资源操作输出"), width)), body, renderHelp(width, m.t("Scroll ↑↓/jk  Retry r  Back Esc", "滚动 ↑↓/jk  重试 r  返回 Esc"))}, "\n")
}

func (m Model) filteredResourceIndexes() []resourceRef {
	items := m.currentResourceRefs()
	query := strings.ToLower(strings.TrimSpace(m.resourceQuery))
	indexes := []resourceRef{}
	for _, ref := range items {
		text := strings.ToLower(m.resourceSearchText(ref))
		if query != "" && !strings.Contains(text, query) {
			continue
		}
		if !m.resourceFilterMatches(ref) {
			continue
		}
		if !m.resourcePortFilterMatches(ref) {
			continue
		}
		indexes = append(indexes, ref)
	}
	return indexes
}

func (m Model) currentResourceRefs() []resourceRef {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return nil
	}
	refs := []resourceRef{}
	if m.resourceKind == resourceAll || m.resourceKind == resourceContainers {
		for i := range m.states[m.resourceHostIndex].ContainerDetails {
			ref := resourceRef{Kind: resourceContainers, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceKind == resourceAll || m.resourceKind == resourceServices {
		for i := range m.states[m.resourceHostIndex].ServiceDetails {
			if !resourceServiceVisible(m.states[m.resourceHostIndex].ServiceDetails[i]) {
				continue
			}
			ref := resourceRef{Kind: resourceServices, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceKind == resourceAll || m.resourceKind == resourceProcesses {
		for _, ref := range m.currentProcessRefs() {
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceKind == resourceAll || m.resourceKind == resourcePorts {
		for i := range m.states[m.resourceHostIndex].PortDetails {
			ref := resourceRef{Kind: resourcePorts, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

func (m Model) resourceManageDiscoveredRefs() []resourceRef {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return nil
	}
	refs := []resourceRef{}
	add := func(ref resourceRef) {
		if m.resourceRefManaged(ref) || m.resourceRefMissing(ref) {
			return
		}
		if !m.resourceManageQueryMatchesRef(ref) {
			return
		}
		refs = append(refs, ref)
	}
	switch m.resourceAddKind {
	case resourceContainers:
		for i := range m.states[m.resourceHostIndex].ContainerDetails {
			add(resourceRef{Kind: resourceContainers, Index: i})
		}
	case resourceServices:
		for i := range m.states[m.resourceHostIndex].ServiceDetails {
			add(resourceRef{Kind: resourceServices, Index: i})
		}
	case resourceProcesses:
		for _, ref := range m.allProcessRefs() {
			add(ref)
		}
	case resourcePorts:
		seen := map[string]bool{}
		for i := range m.states[m.resourceHostIndex].PortDetails {
			item := m.states[m.resourceHostIndex].PortDetails[i]
			key := strings.ToLower(strings.TrimSpace(item.Protocol)) + "/" + strings.TrimSpace(item.Port)
			if key == "/" || seen[key] {
				continue
			}
			seen[key] = true
			add(resourceRef{Kind: resourcePorts, Index: i})
		}
	}
	return refs
}

func (m Model) resourceManageFavorites() []config.ManagedResource {
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(m.resourceAddKind)
	query := strings.ToLower(strings.TrimSpace(m.resourceManageQuery))
	items := []config.ManagedResource{}
	for _, item := range m.resourceFile.Items {
		if item.Server == server && item.Kind == kind {
			if query != "" && !strings.Contains(strings.ToLower(strings.Join([]string{item.Name, item.StartCommand, item.StopCommand, item.RestartCommand, item.LogCommand, item.HealthCommand}, " ")), query) {
				continue
			}
			items = append(items, item)
		}
	}
	return items
}

func (m Model) resourceManageQueryMatchesRef(ref resourceRef) bool {
	query := strings.ToLower(strings.TrimSpace(m.resourceManageQuery))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(m.resourceSearchText(ref)), query)
}

func (m Model) currentProcessRefs() []resourceRef {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return nil
	}
	seen := map[string]bool{}
	refs := []resourceRef{}
	for i, port := range m.states[m.resourceHostIndex].PortDetails {
		if !m.portLooksStandaloneProcess(port) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(port.Process)) + "/" + strings.TrimSpace(port.PID)
		if key == "/" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, resourceRef{Kind: resourceProcesses, Index: i})
	}
	return refs
}

func (m Model) allProcessRefs() []resourceRef {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return nil
	}
	seen := map[string]bool{}
	refs := []resourceRef{}
	for i, port := range m.states[m.resourceHostIndex].PortDetails {
		process := strings.TrimSpace(port.Process)
		if process == "" {
			continue
		}
		key := strings.ToLower(process)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, resourceRef{Kind: resourceProcesses, Index: i})
	}
	return refs
}

func (m Model) resourceNameForRef(ref resourceRef) (string, bool) {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return "", false
	}
	switch ref.Kind {
	case resourceContainers:
		items := m.states[m.resourceHostIndex].ContainerDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Name, true
	case resourceServices:
		items := m.states[m.resourceHostIndex].ServiceDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Unit, true
	case resourceProcesses:
		items := m.states[m.resourceHostIndex].PortDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Process, true
	case resourcePorts:
		items := m.states[m.resourceHostIndex].PortDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return fmt.Sprintf("%s/%s", items[ref.Index].Protocol, items[ref.Index].Port), true
	default:
		return "", false
	}
}

func (m Model) resourceRefInScope(ref resourceRef) bool {
	if ref.Kind == resourceContainers && m.resourceScope != resourceScopeManaged {
		return !m.resourceRefMissing(ref) || m.resourceRefManaged(ref)
	}
	if m.resourceScope == resourceScopeManaged {
		return m.resourceRefManaged(ref) && m.resourceRefFavorite(ref)
	}
	return m.resourceRefManaged(ref)
}

func (m Model) resourceRefManaged(ref resourceRef) bool {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		return item.Managed || !item.Missing
	case resourceServices:
		return m.states[m.resourceHostIndex].ServiceDetails[ref.Index].Managed
	case resourceProcesses:
		return m.states[m.resourceHostIndex].PortDetails[ref.Index].ProcessManaged
	case resourcePorts:
		return m.states[m.resourceHostIndex].PortDetails[ref.Index].Managed
	default:
		return false
	}
}

func (m Model) resourceRefFavorite(ref resourceRef) bool {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	switch ref.Kind {
	case resourceContainers:
		return m.states[m.resourceHostIndex].ContainerDetails[ref.Index].Favorite
	case resourceServices:
		return m.states[m.resourceHostIndex].ServiceDetails[ref.Index].Favorite
	case resourceProcesses:
		return m.states[m.resourceHostIndex].PortDetails[ref.Index].ProcessFavorite
	case resourcePorts:
		return m.states[m.resourceHostIndex].PortDetails[ref.Index].Favorite
	default:
		return false
	}
}

func (m Model) selectedResourceManaged() bool {
	ref, ok := m.selectedResourceRef()
	if !ok {
		return false
	}
	return m.resourceRefManaged(ref)
}

func (m Model) resourceRefMissing(ref resourceRef) bool {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	switch ref.Kind {
	case resourceContainers:
		return m.states[m.resourceHostIndex].ContainerDetails[ref.Index].Missing
	case resourceServices:
		return m.states[m.resourceHostIndex].ServiceDetails[ref.Index].Missing
	case resourceProcesses:
		return false
	case resourcePorts:
		return m.states[m.resourceHostIndex].PortDetails[ref.Index].Missing
	default:
		return false
	}
}

func resourceServiceVisible(item serviceDetail) bool {
	if item.Managed {
		return true
	}
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(item.Load), "not-found") && strings.EqualFold(strings.TrimSpace(item.Active), "inactive") && strings.EqualFold(strings.TrimSpace(item.Sub), "dead") {
		return false
	}
	return true
}

func (m Model) resourceServiceDiscoveredVisible(item serviceDetail, searching bool) bool {
	if item.Missing {
		return false
	}
	if item.Managed {
		return true
	}
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if serviceLooksUserManaged(item) {
		return true
	}
	if serviceLooksInstalledPackage(item) {
		return true
	}
	if m.serviceOwnsListeningPort(item) {
		return true
	}
	if serviceLooksSystemManaged(item) {
		return false
	}
	if serviceDetailKind(item) == "failed" {
		return true
	}
	if !serviceHasDiscoveryMetadata(item) {
		return false
	}
	return searching
}

func (m Model) serviceOwnsListeningPort(item serviceDetail) bool {
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	for _, port := range m.states[m.resourceHostIndex].PortDetails {
		if m.serviceMatchesPort(item, port) {
			return true
		}
	}
	return false
}

func (m Model) portLooksStandaloneProcess(port portDetail) bool {
	if port.ProcessManaged {
		return true
	}
	if port.Missing || strings.TrimSpace(port.Process) == "" || strings.TrimSpace(port.Container) != "" {
		return false
	}
	process := strings.ToLower(strings.TrimSpace(port.Process))
	if serviceIsKnownInfrastructureProcess(process) {
		return false
	}
	for _, service := range m.states[m.resourceHostIndex].ServiceDetails {
		if m.serviceMatchesPort(service, port) {
			return false
		}
	}
	return true
}

func (m Model) serviceMatchesPort(item serviceDetail, port portDetail) bool {
	if item.Missing || serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if strings.TrimSpace(port.ServiceUnit) != "" && strings.TrimSpace(port.ServiceUnit) == strings.TrimSpace(item.Unit) {
		return true
	}
	process := strings.ToLower(strings.TrimSpace(port.Process))
	if process == "" || serviceIsKnownInfrastructureProcess(process) {
		return false
	}
	pids := serviceCandidatePIDs(item)
	if strings.TrimSpace(port.PID) != "" {
		if _, ok := pids[strings.TrimSpace(port.PID)]; ok {
			return true
		}
	}
	names := serviceCandidateProcessNames(item)
	_, ok := names[process]
	return ok
}

func serviceCandidatePIDs(item serviceDetail) map[string]struct{} {
	out := map[string]struct{}{}
	for _, pid := range []string{item.MainPID, item.ExecMainPID} {
		pid = strings.TrimSpace(pid)
		if pid == "" || pid == "0" || pid == "-" {
			continue
		}
		out[pid] = struct{}{}
	}
	return out
}

func serviceCandidateProcessNames(item serviceDetail) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimSuffix(value, ".service")
		if idx := strings.Index(value, "@"); idx >= 0 {
			value = value[:idx]
		}
		value = strings.Trim(value, "\"'{}();")
		if value == "" || strings.Contains(value, "/") || strings.Contains(value, " ") {
			return
		}
		out[value] = struct{}{}
	}
	add(item.Unit)
	for _, token := range strings.Fields(strings.NewReplacer(";", " ", "{", " ", "}", " ").Replace(item.ExecStart)) {
		token = strings.Trim(token, "\"'(),")
		if strings.HasPrefix(token, "path=") {
			token = strings.TrimPrefix(token, "path=")
		}
		if strings.HasPrefix(token, "argv[]=") {
			token = strings.TrimPrefix(token, "argv[]=")
		}
		if strings.Contains(token, "/") {
			if idx := strings.LastIndex(token, "/"); idx >= 0 && idx < len(token)-1 {
				add(token[idx+1:])
			}
		}
	}
	return out
}

func serviceIsKnownInfrastructureProcess(process string) bool {
	process = strings.ToLower(strings.TrimSpace(process))
	if process == "" {
		return false
	}
	exact := map[string]bool{
		"sshd":              true,
		"ssh":               true,
		"chronyd":           true,
		"systemd-network":   true,
		"systemd-networkd":  true,
		"systemd-resolve":   true,
		"systemd-resolved":  true,
		"networkmanager":    true,
		"dhclient":          true,
		"rpcbind":           true,
		"rpc.statd":         true,
		"docker-proxy":      true,
		"containerd":        true,
		"containerd-shim":   true,
		"containerd-shim-r": true,
	}
	return exact[process]
}

func serviceIsKnownInfrastructure(unit string) bool {
	unit = strings.ToLower(strings.TrimSpace(unit))
	if unit == "" {
		return false
	}
	exact := map[string]bool{
		"acpid.service":               true,
		"atd.service":                 true,
		"auditd.service":              true,
		"auth-rpcgss-module.service":  true,
		"chrony.service":              true,
		"chronyd.service":             true,
		"cloud-config.service":        true,
		"cloud-final.service":         true,
		"cloud-init-local.service":    true,
		"cloud-init.service":          true,
		"cloud-init.target":           true,
		"cloud-init-hotplugd.service": true,
		"cron.service":                true,
		"crond.service":               true,
		"dbus.service":                true,
		"dbus-broker.service":         true,
		"fstrim.service":              true,
		"gssproxy.service":            true,
		"irqbalance.service":          true,
		"libstoragemgmt.service":      true,
		"networkmanager.service":      true,
		"network.service":             true,
		"ntpd.service":                true,
		"ntpdate.service":             true,
		"rpc-statd-notify.service":    true,
		"rpcbind.service":             true,
		"sshd.service":                true,
		"ssh.service":                 true,
		"sysstat.service":             true,
	}
	if exact[unit] {
		return true
	}
	for _, prefix := range []string{
		"cloud-init@",
		"dracut-",
		"emergency.",
		"getty@",
		"initrd-",
		"plymouth-",
		"policy-routes@",
		"rescue.",
		"serial-getty@",
		"systemd-",
		"user-runtime-dir@",
		"user@",
	} {
		if strings.HasPrefix(unit, prefix) {
			return true
		}
	}
	return false
}

func serviceHasDiscoveryMetadata(item serviceDetail) bool {
	return strings.TrimSpace(item.FragmentPath) != "" || strings.TrimSpace(item.WorkingDirectory) != "" || strings.TrimSpace(item.ExecStart) != ""
}

func serviceLooksUserManaged(item serviceDetail) bool {
	fragment := strings.TrimSpace(item.FragmentPath)
	if strings.HasPrefix(fragment, "/etc/systemd/system/") || strings.HasPrefix(fragment, "/run/systemd/system/") {
		return true
	}
	text := strings.Join([]string{item.WorkingDirectory, item.ExecStart}, " ")
	for _, marker := range []string{"/opt/", "/data/", "/www/", "/var/www/", "/home/", "/srv/", "/usr/local/", "/etc/openvpn/", "/etc/nginx/", "/etc/supervisor/"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func serviceLooksInstalledPackage(item serviceDetail) bool {
	unit := strings.ToLower(strings.TrimSpace(item.Unit))
	unitBase := strings.TrimSuffix(unit, ".service")
	if idx := strings.Index(unitBase, "@"); idx >= 0 {
		unitBase = unitBase[:idx]
	}
	exact := map[string]bool{
		"caddy":             true,
		"clickhouse":        true,
		"clickhouse-server": true,
		"containerd":        true,
		"docker":            true,
		"elasticsearch":     true,
		"frpc":              true,
		"frps":              true,
		"grafana-server":    true,
		"ipsec":             true,
		"kafka":             true,
		"kibana":            true,
		"logstash":          true,
		"mariadb":           true,
		"mongod":            true,
		"mongodb":           true,
		"mysql":             true,
		"nats":              true,
		"nginx":             true,
		"node_exporter":     true,
		"openvpn":           true,
		"openvpn-server":    true,
		"postgresql":        true,
		"prometheus":        true,
		"rabbitmq-server":   true,
		"redis":             true,
		"redis-server":      true,
		"supervisord":       true,
		"x-ui":              true,
		"xl2tpd":            true,
		"xray":              true,
		"v2ray":             true,
	}
	return exact[unitBase]
}

func serviceLooksSystemManaged(item serviceDetail) bool {
	fragment := strings.TrimSpace(item.FragmentPath)
	if fragment == "" {
		return false
	}
	systemFragments := []string{
		"/usr/lib/systemd/system/",
		"/lib/systemd/system/",
	}
	inSystemDir := false
	for _, prefix := range systemFragments {
		if strings.HasPrefix(fragment, prefix) {
			inSystemDir = true
			break
		}
	}
	if !inSystemDir {
		return false
	}
	text := strings.Join([]string{item.WorkingDirectory, item.ExecStart}, " ")
	if strings.TrimSpace(text) == "" {
		return true
	}
	for _, marker := range []string{"/usr/lib/", "/lib/", "/usr/sbin/", "/usr/bin/", "/sbin/", "/bin/", "/run/", "/var/lib/", "/var/run/"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (m Model) resourceSearchText(ref resourceRef) string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return ""
	}
	if ref.Kind == resourceContainers {
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		return strings.Join([]string{item.Name, item.Image, item.Status, item.Ports}, " ")
	}
	if ref.Kind == resourcePorts {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return strings.Join([]string{item.Protocol, item.Port, item.LocalAddress, item.ForeignAddress, item.State, item.Process, item.PID, item.FD, item.ServiceUnit, item.Container, item.ContainerPort}, " ")
	}
	if ref.Kind == resourceProcesses {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return strings.Join([]string{item.Process, item.PID, item.ServiceUnit, item.Protocol, item.Port, item.LocalAddress, item.State}, " ")
	}
	item := m.states[m.resourceHostIndex].ServiceDetails[ref.Index]
	return strings.Join([]string{item.Unit, item.Load, item.Active, item.Sub, item.Description, item.FragmentPath, item.WorkingDirectory, item.ExecStart}, " ")
}

func (m Model) resourceFilterMatches(ref resourceRef) bool {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	if m.resourceKind == resourcePorts && ref.Kind == resourcePorts {
		return true
	}
	switch m.resourceFilter {
	case resourceFilterRunning:
		if ref.Kind == resourceContainers {
			return containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index]) == "running"
		}
		if ref.Kind == resourcePorts {
			return !m.states[m.resourceHostIndex].PortDetails[ref.Index].Missing
		}
		if ref.Kind == resourceProcesses {
			return true
		}
		return serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index]) == "running"
	case resourceFilterProblems:
		if ref.Kind == resourceContainers {
			kind := containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index])
			return kind == "failed" || kind == "missing"
		}
		if ref.Kind == resourcePorts {
			return m.states[m.resourceHostIndex].PortDetails[ref.Index].Missing
		}
		if ref.Kind == resourceProcesses {
			return false
		}
		kind := serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index])
		return kind == "failed" || kind == "missing"
	case resourceFilterStopped:
		if ref.Kind == resourceContainers {
			return containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index]) == "stopped"
		}
		if ref.Kind == resourcePorts {
			return false
		}
		if ref.Kind == resourceProcesses {
			return false
		}
		return serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index]) == "stopped"
	default:
		return true
	}
}

func (m Model) resourcePortFilterMatches(ref resourceRef) bool {
	if m.resourceKind != resourcePorts || ref.Kind != resourcePorts {
		return true
	}
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
	switch m.resourcePortFilter {
	case resourcePortFilterPublic:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeWildcard
	case resourcePortFilterLoopback:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeLoopback
	case resourcePortFilterSpecific:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeSpecific
	case resourcePortFilterContainer:
		return strings.TrimSpace(item.Container) != ""
	case resourcePortFilterProcess:
		return strings.TrimSpace(item.Process) != ""
	default:
		return true
	}
}

func (m Model) selectedResourceName() (string, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok {
		return "", false
	}
	if ref.Kind == resourceContainers {
		item, ok := m.selectedContainer()
		return item.Name, ok
	}
	if ref.Kind == resourcePorts {
		item, ok := m.selectedPort()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("%s/%s", item.Protocol, item.Port), true
	}
	if ref.Kind == resourceProcesses {
		item, ok := m.selectedProcess()
		return item.Process, ok
	}
	item, ok := m.selectedService()
	return item.Unit, ok
}

func (m Model) selectedResourceRef() (resourceRef, bool) {
	indexes := m.filteredResourceIndexes()
	if len(indexes) == 0 || m.resourceIndex < 0 || m.resourceIndex >= len(indexes) {
		return resourceRef{}, false
	}
	return indexes[m.resourceIndex], true
}

func (m Model) currentSelectedResourceKind() resourceKind {
	if strings.TrimSpace(m.resourceDetailName) != "" {
		return m.resourceDetailKind
	}
	if ref, ok := m.selectedResourceRef(); ok {
		return ref.Kind
	}
	if m.resourceKind == resourcePorts {
		return resourcePorts
	}
	if m.resourceKind == resourceProcesses {
		return resourceProcesses
	}
	if m.resourceKind == resourceServices {
		return resourceServices
	}
	return resourceContainers
}

func (m Model) selectedPort() (portDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourcePorts || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return portDetail{}, false
	}
	items := m.states[m.resourceHostIndex].PortDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return portDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedProcess() (portDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceProcesses || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return portDetail{}, false
	}
	items := m.states[m.resourceHostIndex].PortDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return portDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedContainer() (containerDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceContainers || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return containerDetail{}, false
	}
	items := m.states[m.resourceHostIndex].ContainerDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return containerDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedService() (serviceDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceServices || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return serviceDetail{}, false
	}
	items := m.states[m.resourceHostIndex].ServiceDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return serviceDetail{}, false
	}
	return items[real], true
}

func (m Model) resourceHostTitle() string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return "-"
	}
	return hostDisplayName(m.states[m.resourceHostIndex].Host)
}

func (m Model) resourceKindName(kind resourceKind) string {
	if kind == resourceServices {
		return m.t("Services", "服务")
	}
	if kind == resourceProcesses {
		return m.t("Processes", "进程")
	}
	if kind == resourcePorts {
		return m.t("Ports", "端口")
	}
	if kind == resourceAll {
		return m.t("All", "全部")
	}
	return m.t("Containers", "容器")
}

func (m Model) resourceViewName(view resourceViewMode) string {
	if view == resourceViewList {
		return m.t("List", "列表")
	}
	return m.t("Cards", "卡片")
}

func (m Model) resourceScopeName(scope resourceScopeMode) string {
	if scope == resourceScopeDiscovered {
		return m.t("All", "全部")
	}
	return m.t("Favorites", "收藏")
}

func (m Model) resourceFilterName(filter resourceFilterMode) string {
	switch filter {
	case resourceFilterRunning:
		return m.t("Running", "运行")
	case resourceFilterProblems:
		return m.t("Problems", "异常")
	case resourceFilterStopped:
		return m.t("Stopped", "停止")
	default:
		return m.t("All", "全部")
	}
}

func (m Model) resourcePortFilterName(filter resourcePortFilterMode) string {
	switch filter {
	case resourcePortFilterPublic:
		return m.t("Public", "公网监听")
	case resourcePortFilterLoopback:
		return m.t("Loopback", "仅本机")
	case resourcePortFilterSpecific:
		return m.t("Specific", "指定地址")
	case resourcePortFilterContainer:
		return m.t("Containers", "容器端口")
	case resourcePortFilterProcess:
		return m.t("Processes", "进程端口")
	default:
		return m.t("All", "全部")
	}
}

func (m Model) resourceActionNameText(action resourceActionKind) string {
	switch action {
	case resourceActionStart:
		return m.t("Start", "启动")
	case resourceActionStop:
		return m.t("Stop", "停止")
	case resourceActionRestart:
		return m.t("Restart", "重启")
	case resourceActionDelete:
		return m.t("Disabled", "已禁用")
	default:
		return "-"
	}
}

func (m Model) resourceActionErrorText(result actions.CommandResult) string {
	if strings.Contains(result.Output, "__SSHM_PERMISSION_DENIED__") {
		return m.t("Permission denied. Configure sudo permissions or run with an allowed account.", "权限不够。请配置 sudo 权限或使用有权限的账号。")
	}
	return fmt.Sprintf("%s %d", m.t("Resource action failed, exit", "资源操作失败，退出码"), result.ExitCode)
}

func resourceActionCommandPreview(kind resourceKind, action resourceActionKind, name string) string {
	target := shellQuoteLocal(name)
	if kind == resourceServices {
		switch action {
		case resourceActionStart:
			return "systemctl start " + target
		case resourceActionStop:
			return "systemctl stop " + target
		default:
			return "systemctl restart " + target
		}
	}
	if kind == resourceProcesses || kind == resourcePorts {
		return "-"
	}
	switch action {
	case resourceActionStart:
		return "docker start " + target
	case resourceActionStop:
		return "docker stop " + target
	case resourceActionDelete:
		return "-"
	default:
		return "docker restart " + target
	}
}

func (m Model) resourceCommandPreview(kind resourceKind, action resourceActionKind, name string) string {
	if cmd := m.managedResourceCommand(kind, action, name); cmd != "" {
		return cmd
	}
	return resourceActionCommandPreview(kind, action, name)
}

func (m Model) resourceLogCommandPreview(kind resourceKind, name string, lines int) string {
	if cmd := m.managedResourceLogCommand(kind, name); cmd != "" {
		return cmd
	}
	if kind == resourceServices {
		return fmt.Sprintf("journalctl -u %s -n %d --no-pager", shellQuoteLocal(name), lines)
	}
	if kind == resourceProcesses || kind == resourcePorts {
		return "-"
	}
	return fmt.Sprintf("docker logs --tail %d %s", lines, shellQuoteLocal(name))
}

func (m Model) resourceListHelp() string {
	kind := m.currentSelectedResourceKind()
	partsEN := []string{"Move ↑↓←→/hjkl", "Detail Space"}
	partsZH := []string{"移动 ↑↓←→/hjkl", "详情 Space"}
	partsEN = append(partsEN, "Add a")
	partsZH = append(partsZH, "添加 a")
	partsEN = append(partsEN, "Remove x")
	partsZH = append(partsZH, "移出 x")
	if m.selectedResourceManaged() {
		partsEN = append(partsEN, "Edit e")
		partsZH = append(partsZH, "编辑 e")
	}
	partsEN = append(partsEN, "Favorite f", "Favorites v")
	partsZH = append(partsZH, "收藏 f", "收藏 v")
	partsEN = append(partsEN, "Type Tab")
	partsZH = append(partsZH, "类型 Tab")
	if kind == resourcePorts {
		partsEN = append(partsEN, "Scope g")
		partsZH = append(partsZH, "范围 g")
	} else {
		partsEN = append(partsEN, "Status g")
		partsZH = append(partsZH, "状态 g")
	}
	partsEN = append(partsEN, "View z")
	partsZH = append(partsZH, "视图 z")
	if kind == resourceServices || kind == resourceContainers || kind == resourceProcesses {
		partsEN = append(partsEN, "Logs o")
		partsZH = append(partsZH, "日志 o")
	}
	partsEN = append(partsEN, "Refresh r", "Search /", "Back Esc")
	partsZH = append(partsZH, "刷新 r", "搜索 /", "返回 Esc")
	return m.t(strings.Join(partsEN, "  "), strings.Join(partsZH, "  "))
}

func (m Model) portStatusText(item portDetail) string {
	if item.Missing {
		return m.t("Not found", "未发现")
	}
	if strings.TrimSpace(item.State) != "" {
		return item.State
	}
	return m.t("Listening", "监听中")
}

func (m Model) portStatusStyled(item portDetail) string {
	if item.Missing {
		return mutedStyle.Render(m.portStatusText(item))
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	switch state {
	case "LISTEN", "UNCONN":
		return greenStyle.Render(m.portStatusText(item))
	case "":
		return greenStyle.Render(m.portStatusText(item))
	default:
		return yellowStyle.Render(m.portStatusText(item))
	}
}

func (m Model) portStatusLine(item portDetail) string {
	raw := strings.ToUpper(strings.TrimSpace(item.State))
	if raw == "" {
		raw = m.portStatusText(item)
	}
	return m.portStatusStyledLabel(item, m.portStatusLabel(item)) + "  " + m.portStatusStyledLabel(item, raw)
}

func (m Model) portStatusLabel(item portDetail) string {
	if item.Missing {
		return m.t("Not found", "未发现")
	}
	switch strings.ToUpper(strings.TrimSpace(item.State)) {
	case "LISTEN", "UNCONN", "":
		return m.t("Listening", "监听")
	case "CLOSE", "CLOSED":
		return m.t("Closed", "关闭")
	default:
		return m.t("State", "状态")
	}
}

func (m Model) portStatusStyledLabel(item portDetail, text string) string {
	if item.Missing {
		return mutedStyle.Render(text)
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	switch state {
	case "LISTEN", "UNCONN", "":
		return greenStyle.Render(text)
	case "CLOSE", "CLOSED":
		return mutedStyle.Render(text)
	default:
		return yellowStyle.Render(text)
	}
}

func (m Model) processStatusLine(item portDetail) string {
	raw := strings.ToUpper(strings.TrimSpace(item.State))
	if raw == "" {
		raw = "-"
	}
	return greenStyle.Render(m.t("Running", "运行")) + "  " + greenStyle.Render(raw)
}

func processMemoryText(item processExtraDetail) string {
	mem := percentSuffix(item.Memory)
	if strings.TrimSpace(item.RSS) == "" {
		return emptyDash(mem)
	}
	rss := processRSSText(item.RSS)
	if strings.TrimSpace(mem) == "" {
		return rss
	}
	return rss + "  " + mem
}

func (m Model) processExtraForCard(item portDetail) (processExtraDetail, bool) {
	if strings.TrimSpace(item.PID) == "" || m.resourceProcessExtraLoading || strings.TrimSpace(m.resourceProcessExtraErr) != "" {
		return processExtraDetail{}, false
	}
	if m.resourceProcessExtraPID != item.PID || m.resourceProcessExtra.PID != item.PID {
		return processExtraDetail{}, false
	}
	return m.resourceProcessExtra, true
}

func (m Model) processCardMeta(extra processExtraDetail, ok bool) string {
	if !ok {
		return ""
	}
	d, parsed := parseProcessElapsed(extra.Elapsed)
	if !parsed {
		return ""
	}
	return m.dashboardDurationShort(d)
}

func (m Model) processCardStatusLine(item portDetail, extra processExtraDetail, ok bool) string {
	state := ""
	if ok && strings.TrimSpace(extra.State) != "" {
		state = strings.TrimSpace(extra.State)
	}
	if state == "" && strings.TrimSpace(item.State) != "" && !strings.EqualFold(strings.TrimSpace(item.State), "LISTEN") && !strings.EqualFold(strings.TrimSpace(item.State), "UNCONN") {
		state = strings.TrimSpace(item.State)
	}
	status := m.t("Running", "运行")
	style := greenStyle.Render
	if processStateProblem(state) {
		style = redStyle.Render
	} else if processStateWarn(state) {
		style = yellowStyle.Render
	}
	if state == "" {
		return style(status)
	}
	return style(status) + "  " + style(state)
}

func (m Model) processCardResourceLine(item portDetail, extra processExtraDetail, ok bool) string {
	parts := []string{}
	if strings.TrimSpace(item.PID) != "" {
		parts = append(parts, "PID  "+cardMutedStyle.Render(item.PID))
	}
	memory := ""
	if ok {
		memory = processMemoryText(extra)
	}
	if strings.TrimSpace(memory) != "" && memory != "-" {
		parts = append(parts, m.t("Memory", "内存")+"  "+cardMutedStyle.Render(memory))
	}
	if len(parts) == 0 {
		return m.t("Resource", "资源") + "  " + cardMutedStyle.Render("-")
	}
	return strings.Join(parts, "  ")
}

func (m Model) processCardExecutable(item portDetail, extra processExtraDetail, ok bool) string {
	if ok && strings.TrimSpace(extra.Executable) != "" {
		return strings.TrimSpace(extra.Executable)
	}
	return strings.TrimSpace(item.Process)
}

func (m Model) processCardCommandLine(item portDetail, extra processExtraDetail, ok bool) string {
	if ok && strings.TrimSpace(extra.CommandLine) != "" {
		return strings.TrimSpace(extra.CommandLine)
	}
	if ok && strings.TrimSpace(extra.Command) != "" {
		return strings.TrimSpace(extra.Command)
	}
	if strings.TrimSpace(item.Process) != "" {
		return strings.TrimSpace(item.Process)
	}
	return strings.TrimSpace(item.Protocol + "/" + item.Port)
}

func (m Model) processCardDot(extra processExtraDetail, ok bool) string {
	state := ""
	if ok {
		state = extra.State
	}
	switch {
	case processStateProblem(state):
		return redStyle.Render("●")
	case processStateWarn(state):
		return yellowStyle.Render("●")
	default:
		return greenStyle.Render("●")
	}
}

func processStateProblem(state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	return strings.Contains(state, "Z") || strings.Contains(state, "X")
}

func processStateWarn(state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	return strings.Contains(state, "D") || strings.Contains(state, "T")
}

func parseProcessElapsed(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	days := 0
	if left, right, ok := strings.Cut(value, "-"); ok {
		n, err := strconv.Atoi(left)
		if err != nil {
			return 0, false
		}
		days = n
		value = right
	}
	parts := strings.Split(value, ":")
	total := time.Duration(days) * 24 * time.Hour
	switch len(parts) {
	case 2:
		minutes, err1 := strconv.Atoi(parts[0])
		seconds, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, false
		}
		total += time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	case 3:
		hours, err1 := strconv.Atoi(parts[0])
		minutes, err2 := strconv.Atoi(parts[1])
		seconds, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, false
		}
		total += time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	default:
		return 0, false
	}
	return total, true
}

func processRSSText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	kb, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return value
	}
	return bytesHuman(kb * 1024)
}

func percentSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, "%") {
		return value
	}
	return value + "%"
}

func portListenText(item portDetail) string {
	if strings.TrimSpace(item.LocalAddress) != "" {
		return item.LocalAddress
	}
	if strings.TrimSpace(item.Port) == "" {
		return ""
	}
	return strings.TrimSpace(item.Protocol + "/" + item.Port)
}

func portProcessDetailText(item portDetail) string {
	process := emptyDash(item.Process)
	if strings.TrimSpace(item.PID) != "" {
		process += " pid:" + strings.TrimSpace(item.PID)
	}
	if strings.TrimSpace(item.FD) != "" {
		process += " fd:" + strings.TrimSpace(item.FD)
	}
	return process
}

func (m Model) portScopeText(item portDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return mutedStyle.Render(m.t("Not found", "未发现"))
	case scope == portScopeUnknown:
		return mutedStyle.Render("-")
	case scope == portScopeLoopback:
		return greenStyle.Render(m.t("Loopback only", "仅本机"))
	case scope == portScopeWildcard:
		return redStyle.Render(m.t("All interfaces", "所有网卡"))
	default:
		return yellowStyle.Render(m.t("Specific address", "指定地址"))
	}
}

func (m Model) portRiskText(item portDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return mutedStyle.Render("-")
	case scope == portScopeUnknown:
		return mutedStyle.Render(m.t("Unknown listener", "监听地址未知"))
	case scope == portScopeLoopback:
		return greenStyle.Render(m.t("Local only", "仅本机访问"))
	case scope == portScopeWildcard:
		return redStyle.Render(m.t("Public listener", "公网监听"))
	default:
		return yellowStyle.Render(m.t("Check firewall", "检查防火墙"))
	}
}

func (m Model) portScopeNote(item portDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return m.t("The configured listener was not found.", "配置的监听端口未发现。")
	case scope == portScopeUnknown:
		return m.t("The listener address could not be parsed from ss/netstat output.", "无法从 ss/netstat 输出解析监听地址。")
	case scope == portScopeLoopback:
		return m.t("This port only listens on loopback and is normally reachable only from this server.", "当前端口只监听本机回环地址，通常只有这台服务器自己能访问。")
	case scope == portScopeWildcard:
		return m.t("This port listens on all local interfaces, including private and public NICs.", "当前端口监听所有本机网卡，包括内网网卡和公网网卡。")
	default:
		return m.t("This port listens on a specific local address only.", "当前端口只监听指定的本机地址。")
	}
}

func (m Model) portRiskNote(item portDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return "-"
	case scope == portScopeUnknown:
		return m.t("Risk cannot be judged without the listener address.", "缺少监听地址，无法判断风险。")
	case scope == portScopeLoopback:
		return m.t("External hosts normally cannot access this port unless traffic is forwarded locally.", "外部机器通常不能直接访问，除非本机做了转发。")
	case scope == portScopeWildcard:
		return m.t("If firewall or security group allows it, external hosts may access this port.", "如果防火墙或安全组放行，外部机器可能访问这个端口。")
	default:
		return m.t("Check whether this address is reachable from external networks.", "需要确认这个指定地址是否能被外部网络访问。")
	}
}

func portIPVersion(address string) string {
	host := portAddressHost(address)
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		return "IPv6"
	}
	return "IPv4"
}

func portAddressHost(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.HasPrefix(address, "[") {
		if idx := strings.LastIndex(address, "]:"); idx >= 0 {
			return strings.Trim(address[1:idx], "[]")
		}
	}
	if idx := strings.LastIndex(address, ":"); idx >= 0 {
		return strings.TrimSpace(address[:idx])
	}
	return address
}

type portScope int

const (
	portScopeUnknown portScope = iota
	portScopeLoopback
	portScopeSpecific
	portScopeWildcard
)

func portAddressScope(address string) portScope {
	scope := portScopeUnknown
	for _, part := range splitCSVValues(address) {
		host := portAddressHost(part)
		switch {
		case host == "":
			continue
		case isWildcardHost(host):
			return portScopeWildcard
		case isLoopbackHost(host):
			if scope == portScopeUnknown {
				scope = portScopeLoopback
			}
		default:
			if scope != portScopeSpecific {
				scope = portScopeSpecific
			}
		}
	}
	return scope
}

func isWildcardHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "*" || host == "0.0.0.0" || host == "::" || host == ":::" || host == ""
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func yesNoText(zh bool, value bool) string {
	if zh {
		return yesNo(value)
	}
	if value {
		return "Yes"
	}
	return "No"
}
