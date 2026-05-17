package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

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
			if visible == m.resourceState.Index {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.resourceCard(indexes[visible], visible == m.resourceState.Index, cardWidth), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.resourceState.Index >= i && m.resourceState.Index < rowEnd {
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
		item := m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index]
		kind := containerDetailKind(item)
		title := m.resourceTypeBadge(ref.Kind) + " " + item.Name
		title = m.resourceMarkedTitle(ref, title, item.Favorite)
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
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		title := m.resourceTypeBadge(ref.Kind) + " " + fmt.Sprintf("%s/%s", item.Protocol, item.Port)
		dot := greenStyle.Render("●")
		if item.Missing {
			dot = mutedStyle.Render("●")
		}
		title = m.resourceMarkedTitle(ref, title, item.Favorite)
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
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		title := m.resourceTypeBadge(ref.Kind) + " " + emptyDash(item.Process)
		title = m.resourceMarkedTitle(ref, title, item.ProcessFavorite)
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
	if ref.Kind == resourceDatabases {
		item := m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index]
		title := databaseTitleMark() + " " + databaseDisplayTitle(item)
		title = m.resourceMarkedTitle(ref, title, item.Favorite)
		meta := cardMutedStyle.Render(m.databaseCardMeta(item))
		return strings.Join([]string{
			cardTopLine(width, fitANSI(title, maxInt(8, width-12)), meta, m.databaseStatusDot(item), borderStyle),
			cardContentLine(width, m.t("Status", "状态")+"  "+m.databaseStatusLine(item), borderStyle),
			cardContentLine(width, m.t("Endpoint", "地址")+"  "+cardMutedStyle.Render(emptyDash(item.Endpoint)), borderStyle),
			cardContentLine(width, m.databaseCardOwnerLine(item), borderStyle),
			cardInnerSeparatorLine(width, borderStyle),
			cardContentLine(width, m.databaseCardStorageLine(item, width), borderStyle),
			cardContentLine(width, m.databaseCardCountLine(item), borderStyle),
			cardBottomLine(width, borderStyle),
		}, "\n")
	}
	item := m.mergedServiceDetail(m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index])
	kind := serviceDetailKind(item)
	title := m.resourceTypeBadge(ref.Kind) + " " + item.Unit
	title = m.resourceMarkedTitle(ref, title, item.Favorite)
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

func (m Model) resourceMarkedTitle(ref resourceRef, title string, favorite bool) string {
	marks := ""
	if pinned, _ := m.resourceRefPinned(ref); pinned {
		marks += pinnedStyle.Render("▲") + " "
	}
	if favorite {
		marks += favoriteStyle.Render("★") + " "
	}
	return marks + title
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
		mark = "□"
	case resourceProcesses:
		label = m.t("[Process]", "[进程]")
		mark = "◇"
	case resourcePorts:
		label = m.t("[Port]", "[端口]")
		mark = "◌"
	case resourceDatabases:
		label = m.t("[Database]", "[库]")
		mark = "▤"
	default:
		label = m.t("[Resource]", "[资源]")
	}
	return lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(kind)).Render(mark + " " + label)
}

func databaseDisplayTitle(item resourceservice.DatabaseDetail) string {
	engine := strings.TrimSpace(item.Engine)
	name := strings.TrimSpace(item.Name)
	if engine == "" {
		return detailValueStyle.Render(emptyDash(name))
	}
	engineLabel := lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(resourceDatabases)).Render("[" + engine + "]")
	if name == "" {
		return engineLabel
	}
	return engineLabel + " " + detailValueStyle.Render(name)
}

func databaseTitleMark() string {
	return lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(resourceDatabases)).Render("▤")
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
	case resourceDatabases:
		return lipgloss.Color("208")
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
		if i == m.resourceState.Index {
			selectedRow = len(rows)
		}
		rows = append(rows, m.resourceListLine(ref, i == m.resourceState.Index, width))
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
		item := m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index]
		kind := containerDetailKind(item)
		displayName := m.resourceTypeBadge(ref.Kind) + " " + item.Name
		return m.resourceListColumns(prefix, m.resourceRefPinnedOnly(ref), item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), coloredContainerStatus(emptyDash(item.Status), kind), containerResourceText(item), firstNonEmpty(item.Image, simplifyDockerPorts(item.Ports)), width, nameWidth, statusWidth, infoWidth)
	}
	if ref.Kind == resourcePorts {
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		displayName := m.resourceTypeBadge(ref.Kind) + " " + fmt.Sprintf("%s/%s", item.Protocol, item.Port)
		return m.resourceListColumns(prefix, m.resourceRefPinnedOnly(ref), item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), m.portStatusStyled(item), portListenText(item), portProcessDetailText(item), width, nameWidth, statusWidth, infoWidth)
	}
	if ref.Kind == resourceProcesses {
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		displayName := m.resourceTypeBadge(ref.Kind) + " " + emptyDash(item.Process)
		return m.resourceListColumns(prefix, m.resourceRefPinnedOnly(ref), item.ProcessFavorite, nameStyle.Render(fitANSI(displayName, nameWidth)), portListenText(item), "PID "+emptyDash(item.PID), m.portRiskText(item), width, nameWidth, statusWidth, infoWidth)
	}
	if ref.Kind == resourceDatabases {
		item := m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index]
		displayName := databaseTitleMark() + " " + databaseDisplayTitle(item)
		return m.resourceListColumns(prefix, m.resourceRefPinnedOnly(ref), item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), m.databaseStatusStyled(item), item.Engine+"  "+emptyDash(item.Source), firstNonEmpty(item.Endpoint, item.RawStatus), width, nameWidth, statusWidth, infoWidth)
	}
	item := m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index]
	kind := serviceDetailKind(item)
	displayName := m.resourceTypeBadge(ref.Kind) + " " + item.Unit
	return m.resourceListColumns(prefix, m.resourceRefPinnedOnly(ref), item.Favorite, nameStyle.Render(fitANSI(displayName, nameWidth)), coloredServiceStatus(m.serviceStatusText(item), kind), serviceResourceText(item), m.serviceUnitFileStateText(item.UnitFileState), width, nameWidth, statusWidth, infoWidth)
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

func (m Model) resourceRefPinnedOnly(ref resourceRef) bool {
	pinned, _ := m.resourceRefPinned(ref)
	return pinned
}

func (m Model) resourceListColumns(prefix string, pinned bool, favorite bool, name string, status string, info string, extra string, width int, nameWidth int, statusWidth int, infoWidth int) string {
	pinnedMark := " "
	if pinned {
		pinnedMark = pinnedStyle.Render("▲")
	}
	favoriteMark := " "
	if favorite {
		favoriteMark = favoriteStyle.Render("★")
	}
	name = padVisible(fitANSI(name, nameWidth), nameWidth)
	status = padVisible(fitANSI(status, statusWidth), statusWidth)
	info = cardMutedStyle.Render(padVisible(fitANSI(info, infoWidth), infoWidth))
	used := 2 + 1 + 1 + 1 + 1 + 1 + 1 + nameWidth + 2 + statusWidth + 2 + infoWidth + 2
	extraWidth := maxInt(8, width-used)
	extra = cardMutedStyle.Render(fitANSI(extra, extraWidth))
	return fitANSI(fmt.Sprintf("  %s %s %s %s  %s  %s  %s", prefix, pinnedMark, favoriteMark, name, status, info, extra), width)
}
