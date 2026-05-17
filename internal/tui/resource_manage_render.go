package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

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
	if m.resourceState.ManagePane == 0 {
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
	if m.resourceState.ManageSearch {
		searchWidth := width / 3
		if searchWidth < 8 {
			searchWidth = 8
		}
		headerParts = append(headerParts, blueStyle.Render(m.t("Search ", "搜索 ")+inlineCursorText(m.resourceState.ManageQuery, searchWidth, len([]rune(m.resourceState.ManageQuery)))))
	} else if strings.TrimSpace(m.resourceState.ManageQuery) != "" {
		headerParts = append(headerParts, blueStyle.Render(m.t("Search ", "搜索 ")+m.resourceState.ManageQuery))
	}
	if collected := m.resourceManageCollectedText(); collected != "" {
		headerParts = append(headerParts, mutedStyle.Render(collected))
	}
	if m.resourceState.Loading {
		headerParts = append(headerParts, m.dashboardStatusHeaderText(m.status))
	} else if strings.TrimSpace(m.status) != "" && m.status != m.resourceState.RefreshStatus {
		headerParts = append(headerParts, m.dashboardStatusHeaderText(m.status))
	}
	header := strings.Join(headerParts, "  ")
	help := renderHelp(width, m.resourceManageHelp())
	return strings.Join([]string{titleStyle.Render(fitANSI(header, width)), lipgloss.JoinHorizontal(lipgloss.Top, left, right), help}, "\n")
}

func (m Model) resourceManageHelp() string {
	partsEN := []string{"Move ↑↓/jk", "Pane Tab", "Type ←→/g", "Search /"}
	partsZH := []string{"移动 ↑↓/jk", "切栏 Tab", "类型 ←→/g", "搜索 /"}
	if m.resourceState.ManagePane == 0 {
		if m.resourceState.AddKind == resourceDatabases {
			partsEN = append(partsEN, "Config Enter/f")
			partsZH = append(partsZH, "配置 Enter/f")
			partsEN = append(partsEN, "New n")
			partsZH = append(partsZH, "新建 n")
		} else {
			partsEN = append(partsEN, "Add Enter/f")
			partsZH = append(partsZH, "添加 Enter/f")
		}
	} else {
		partsEN = append(partsEN, "Remove Enter/x", "Edit e")
		partsZH = append(partsZH, "移出 Enter/x", "编辑 e")
	}
	partsEN = append(partsEN, "Refresh r", "Back q/Esc")
	partsZH = append(partsZH, "刷新 r", "返回 q/Esc")
	return m.t(strings.Join(partsEN, "  "), strings.Join(partsZH, "  "))
}

func (m Model) renderResourceAddEdit() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 40 {
		bodyWidth = 40
	}
	lines := []string{
		detailSubTitle(m.resourceAddDatabaseConfigTitle()),
	}
	lines = append(lines, m.resourceAddDatabaseInstanceLines()...)
	lines = append(lines, "")
	for i := 0; i < resourceAddFieldCount(m.resourceState.AddKind); i++ {
		lines = append(lines, m.resourceAddEditFieldLine(i, bodyWidth))
	}
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	start, end := visibleRange(len(lines), maxInt(0, m.resourceState.AddField+5), bodyHeight)
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines[start:end], "\n"))
	help := renderHelp(width, m.t("Save Enter  Move Tab/↑↓  Cursor ←→  Back Esc", "保存 Enter  移动 Tab/↑↓  光标 ←→  返回 Esc"))
	return strings.Join([]string{titleStyle.Render(fitANSI(m.resourceAddDatabaseConfigTitle(), width)), body, help}, "\n")
}

func (m Model) resourceAddDatabaseConfigTitle() string {
	if strings.TrimSpace(m.resourceState.CommandForm.DBInstance) != "" {
		return m.t("Configure Database", "配置数据库")
	}
	return m.t("Add Database Manually", "手动新增数据库")
}

func (m Model) resourceAddDatabaseInstanceLines() []string {
	lines := []string{}
	if strings.TrimSpace(m.resourceState.CommandForm.DBInstance) != "" {
		lines = append(lines,
			sectionTitle(m.t("Discovered instance", "发现实例")),
			m.detailRow(m.t("Instance", "实例"), m.resourceState.CommandForm.DBInstance),
			m.detailRow(m.t("Engine", "数据库"), emptyDash(m.resourceState.CommandForm.DBEngine)),
			m.detailRow(m.t("Status", "状态"), m.resourceAddDatabaseStatusLine()),
			m.detailRow(m.t("Source", "来源"), emptyDash(m.resourceState.CommandForm.DBSource)),
			m.detailRow(m.t("Endpoint", "地址"), emptyDash(m.resourceState.CommandForm.DBEndpoint)),
		)
		if strings.TrimSpace(m.resourceState.CommandForm.DBContainer) != "" {
			lines = append(lines, m.detailRow(m.t("Container", "容器"), m.resourceState.CommandForm.DBContainer))
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBImage) != "" {
			lines = append(lines, m.detailRow(m.t("Image", "镜像"), m.resourceState.CommandForm.DBImage))
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBServiceUnit) != "" {
			lines = append(lines, m.detailRow(m.t("Service", "服务"), m.resourceState.CommandForm.DBServiceUnit))
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBProcess) != "" {
			lines = append(lines, m.detailRow(m.t("Process", "进程"), m.resourceState.CommandForm.DBProcess))
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBPID) != "" {
			lines = append(lines, m.detailRow("PID", m.resourceState.CommandForm.DBPID))
		}
	} else {
		lines = append(lines,
			sectionTitle(m.t("Connection", "连接")),
			m.detailRow(m.t("Source", "来源"), m.t("External", "外部")),
		)
	}
	lines = append(lines,
		m.detailRow(m.t("Connection", "连接方式"), m.t("Run from server", "通过服务器执行")),
		m.detailRow(m.t("Runner", "执行位置"), m.resourceHostTitle()),
		m.detailRow(m.t("Jump host", "跳板机"), m.resourceHostJumpText(m.resourceState.HostIndex)),
		"",
		sectionTitle(m.t("Database schema", "库配置")),
	)
	return lines
}

func (m Model) resourceAddDatabaseStatusLine() string {
	raw := strings.TrimSpace(m.resourceState.CommandForm.DBRawStatus)
	item := resourceservice.DatabaseDetail{Status: m.resourceState.CommandForm.DBStatus, RawStatus: raw}
	return m.databaseStatusLine(item)
}

func (m Model) resourceAddEditFieldLine(field int, width int) string {
	label := m.resourceAddEditFieldName(field)
	value := m.resourceAddFieldValue(field)
	inputWidth := width - 18
	if inputWidth < 18 {
		inputWidth = 18
	}
	display := commandInputText(value, m.resourceState.AddCursor, m.resourceState.AddField == field, inputWidth)
	if m.resourceState.CommandForm.Kind == resourceDatabases && field == 0 {
		display = m.resourceAddDatabaseEngineDisplay(inputWidth)
	}
	prefix := "  "
	style := detailValueStyle
	if m.resourceState.AddField == field {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	return fitANSI(prefix+style.Render(padVisible(label, 12))+"  "+display, width)
}

func (m Model) resourceAddDatabaseEngineDisplay(width int) string {
	value := resourceservice.NormalizeDatabaseEngine(m.resourceState.CommandForm.DBEngine)
	if value == "" {
		value = "MySQL"
	}
	text := value
	if m.resourceState.AddField == 0 {
		text = "← " + text + " →"
	}
	return fitANSI(text, width)
}

func (m Model) resourceAddEditFieldName(field int) string {
	if m.resourceState.AddKind == resourceDatabases {
		return m.resourceCommandFieldName(field)
	}
	if field == 0 {
		return m.t("Name", "名称")
	}
	return m.resourceCommandFieldName(field - 1)
}

func (m Model) resourceManageTypeTabs() string {
	return fmt.Sprintf("%s  %s  %s", m.t("Type", "类型"), lipgloss.NewStyle().Bold(true).Foreground(resourceKindColor(m.resourceState.AddKind)).Render(m.resourceKindName(m.resourceState.AddKind)), mutedStyle.Render("←→/g"))
}

func (m Model) resourceManageCollectedText() string {
	return m.resourceRefreshHeaderText()
}

func (m Model) resourceHostJumpText(index int) string {
	if index < 0 || index >= len(m.states) {
		return m.t("None", "无")
	}
	h := m.states[index].Host
	if strings.TrimSpace(h.JumpHostRef) != "" {
		return h.JumpHostRef
	}
	if strings.TrimSpace(h.JumpTarget()) != "" {
		return h.JumpTarget()
	}
	return m.t("None", "无")
}

func (m Model) resourceManageDiscoveredLines(refs []resourceRef, width int, height int) []string {
	lines := []string{}
	if len(refs) == 0 {
		if m.resourceState.Loading {
			lines = append(lines, mutedStyle.Render(m.t("Loading ", "正在加载")+m.resourceKindName(m.resourceState.AddKind)+"..."))
		} else if errText := m.resourceErrorTextForKind(m.resourceState.AddKind); errText != "" {
			lines = append(lines, redStyle.Render(fitANSI(errText, width)))
		} else {
			lines = append(lines, mutedStyle.Render(m.t("No discovered resources", "暂无发现资源")))
		}
	} else {
		idx := clampInt(m.resourceState.ManageDiscoveredIndex, 0, len(refs)-1)
		start, end := visibleRange(len(refs), idx, maxInt(1, height-1))
		for i := start; i < end; i++ {
			selected := m.resourceState.ManagePane == 0 && i == idx
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
		idx := clampInt(m.resourceState.ManageFavoriteIndex, 0, len(items)-1)
		start, end := visibleRange(len(items), idx, maxInt(1, height-1))
		for i := start; i < end; i++ {
			selected := m.resourceState.ManagePane == 1 && i == idx
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
