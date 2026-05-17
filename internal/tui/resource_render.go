package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	if m.resourceSort != resourceSortDefault {
		parts = append(parts, mutedStyle.Render(m.resourceSortName(m.resourceSort)))
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

func (m Model) resourceSortName(sortMode resourceSortMode) string {
	switch sortMode {
	case resourceSortStatus:
		return m.t("Status", "状态")
	case resourceSortName:
		return m.t("Name", "名称")
	case resourceSortCPU:
		return "CPU"
	case resourceSortMemory:
		return m.t("Memory", "内存")
	case resourceSortPort:
		return m.t("Port", "端口")
	default:
		return m.t("Default", "默认")
	}
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
	if m.resourceLoading && len(m.filteredResourceIndexes()) == 0 && m.hasManagedResources(m.resourceHostIndex) {
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
		text := mutedStyle.Render(m.resourceEmptyText())
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

func (m Model) resourceEmptyText() string {
	if strings.TrimSpace(m.resourceQuery) != "" || m.resourceFilter != resourceFilterAll || (m.resourceKind == resourcePorts && m.resourcePortFilter != resourcePortFilterAll) {
		return m.t("No matching resources", "没有匹配的资源")
	}
	if m.resourceScope == resourceScopeManaged {
		return m.t("No favorite resources. Press f to favorite an added resource.", "暂无收藏资源。选中已添加资源后按 f 收藏。")
	}
	return m.t("No added resources. Press a to open Resource Manager and add one.", "暂无已添加资源。按 a 进入资源管理添加。")
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
		return m.friendlyResourceErrorText(m.states[m.resourceHostIndex].ContainerError)
	}
	if kind == resourcePorts {
		return m.friendlyResourceErrorText(m.states[m.resourceHostIndex].PortDetailsError)
	}
	if kind == resourceProcesses {
		return m.friendlyResourceErrorText(m.states[m.resourceHostIndex].PortDetailsError)
	}
	if kind == resourceDatabases {
		return m.friendlyResourceErrorText(m.states[m.resourceHostIndex].DatabaseError)
	}
	if kind == resourceAll {
		parts := []string{}
		seen := map[string]bool{}
		add := func(errText string) {
			errText = strings.TrimSpace(errText)
			if errText == "" || seen[errText] {
				return
			}
			seen[errText] = true
			parts = append(parts, errText)
		}
		if errText := m.friendlyResourceErrorText(m.states[m.resourceHostIndex].ContainerError); errText != "" {
			add(errText)
		}
		if errText := m.friendlyResourceErrorText(m.states[m.resourceHostIndex].ServiceError); errText != "" {
			add(errText)
		}
		if errText := m.friendlyResourceErrorText(m.states[m.resourceHostIndex].PortDetailsError); errText != "" {
			add(errText)
		}
		return strings.Join(parts, " / ")
	}
	return m.friendlyResourceErrorText(m.states[m.resourceHostIndex].ServiceError)
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
	if kind == resourceDatabases {
		return m.t("Databases", "数据库")
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
		return m.t("Added", "已添加")
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

func (m Model) resourceListHelp() string {
	kind := m.currentSelectedResourceKind()
	partsEN := []string{"Move ↑↓←→/hjkl", "Detail Space"}
	partsZH := []string{"移动 ↑↓←→/hjkl", "详情 Space"}
	partsEN = append(partsEN, "Add a")
	partsZH = append(partsZH, "添加 a")
	partsEN = append(partsEN, "Remove x")
	partsZH = append(partsZH, "移出 x")
	if m.selectedResourceManaged() || kind == resourceDatabases {
		if kind == resourceDatabases {
			partsEN = append(partsEN, "Config e")
			partsZH = append(partsZH, "配置 e")
		} else {
			partsEN = append(partsEN, "Edit e")
			partsZH = append(partsZH, "编辑 e")
		}
	}
	partsEN = append(partsEN, "Pin t", "Favorite f", "Favorites v")
	partsZH = append(partsZH, "置顶 t", "收藏 f", "收藏 v")
	partsEN = append(partsEN, "Type Tab")
	partsZH = append(partsZH, "类型 Tab")
	if kind == resourcePorts {
		partsEN = append(partsEN, "Scope g")
		partsZH = append(partsZH, "范围 g")
	} else {
		partsEN = append(partsEN, "Status g")
		partsZH = append(partsZH, "状态 g")
	}
	partsEN = append(partsEN, "View z", "Sort y")
	partsZH = append(partsZH, "视图 z", "排序 y")
	if kind == resourceServices || kind == resourceContainers || kind == resourceProcesses {
		partsEN = append(partsEN, "Logs o", "Start s", "Stop p", "Restart c")
		partsZH = append(partsZH, "日志 o", "启动 s", "停止 p", "重启 c")
	}
	partsEN = append(partsEN, "Refresh r", "Search /", "Back Esc")
	partsZH = append(partsZH, "刷新 r", "搜索 /", "返回 Esc")
	return m.t(strings.Join(partsEN, "  "), strings.Join(partsZH, "  "))
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
