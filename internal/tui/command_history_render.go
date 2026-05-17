package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderCommandHistory() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := m.t("Move ↑↓/jk  View Enter  Search /  Rerun r  Delete x  Back q/Esc", "移动 ↑↓/jk  查看 Enter  搜索 /  重跑 r  删除 x  返回 q/Esc")
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	lines := []string{}
	entries := m.filteredHistoryEntries()
	if len(entries) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No command history", "暂无命令历史")))
	} else {
		start, end := visibleRange(len(entries), m.historyIndex, height)
		for i := start; i < end; i++ {
			entry := entries[i]
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.historyIndex {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			status := m.historyStatusText(entry.Status)
			line := fmt.Sprintf("%s %s  %s  %s  %s", prefix, historyTimeShort(entry.Time), status, m.historyTargetsText(entry, 1), m.historyCommandName(entry))
			lines = append(lines, style.Render(fit(line, bodyWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("%s  %d%s", m.t("Command History", "命令历史"), len(entries), m.t(" records", "条"))
	if m.historySearch {
		title += "  " + m.t("Search: ", "搜索：") + inlineCursorText(m.historyQuery, width/3, len([]rune(m.historyQuery)))
	} else if strings.TrimSpace(m.historyQuery) != "" {
		title += "  " + m.t("Search: ", "搜索：") + m.historyQuery
	}
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) renderCommandHistoryDetail() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return m.t("No command history", "没有命令历史")
	}
	lines := []string{
		modalLine(m.t("Time", "时间"), historyTimeFull(entry.Time), bodyWidth),
		modalLine(m.t("Status", "状态"), m.historyStatusPlain(entry.Status), bodyWidth),
		modalLine(m.t("Type", "类型"), m.historyKindText(entry), bodyWidth),
		modalLine(m.t("Name", "名称"), m.historyCommandName(entry), bodyWidth),
		"",
		detailSubTitle(m.t("Targets", "目标")),
	}
	for _, target := range entry.Targets {
		state := m.historyStatusPlain(target.Status)
		targetText := fmt.Sprintf("%s  %s  %s%d", m.historyTargetName(target), state, m.t("exit ", "退出码"), target.ExitCode)
		lines = append(lines, fit(targetText, bodyWidth))
	}
	lines = append(lines, "", detailSubTitle(m.t("Command", "命令")))
	lines = append(lines, strings.Split(wrapPlainLine(entry.Command, bodyWidth), "\n")...)
	lines = append(lines, "", detailSubTitle(m.t("Output", "输出")))
	for _, target := range entry.Targets {
		lines = append(lines, fit("["+m.historyTargetName(target)+"]", bodyWidth))
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			output = m.t("(no output)", "(无输出)")
		}
		lines = append(lines, strings.Split(wrapPlainLine(output, bodyWidth), "\n")...)
		lines = append(lines, "")
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.historyScroll, 0, m.commandHistoryDetailMaxScroll())
	viewLines := lines
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(fitLines(viewLines, bodyWidth), "\n"))
	help := m.t("Scroll ↑↓/jk  Rerun r  Delete x  Back q/Esc", "滚动 ↑↓/jk  重跑 r  删除 x  返回 q/Esc")
	return strings.Join([]string{titleStyle.Render(fit(m.t("Command History Detail", "命令历史详情"), width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) commandHistoryDetailMaxScroll() int {
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return 0
	}
	bodyWidth := detailFrameWidth(m.width) - 4
	lines := 9 + len(entry.Targets)*3 + len(wrapDetailValue(entry.Command, bodyWidth))
	for _, target := range entry.Targets {
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(wrapDetailValue(output, bodyWidth))
		}
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func historyCommandName(entry config.CommandHistoryEntry) string {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return "临时命令"
	}
	return name
}

func (m Model) historyCommandName(entry config.CommandHistoryEntry) string {
	return m.commandDisplayName(historyCommandName(entry))
}

func (m Model) commandDisplayName(name string) string {
	if m.isChineseUI() {
		return name
	}
	switch strings.TrimSpace(name) {
	case "当前服务器":
		return "Current Server"
	case "全局":
		return "Global"
	case "临时命令":
		return "Temporary Command"
	case "学习样板-部署检查":
		return "Example - Deploy Check"
	case "学习样板-部署发布":
		return "Example - Deploy Release"
	case "学习样板-日志排查":
		return "Example - Log Troubleshooting"
	case "学习样板-Docker清理":
		return "Example - Docker Cleanup"
	default:
		return name
	}
}

func (m Model) historyKindText(entry config.CommandHistoryEntry) string {
	if entry.Kind == "batch" {
		if m.isChineseUI() {
			return fmt.Sprintf("批量命令 %d台", len(entry.Targets))
		}
		return fmt.Sprintf("Batch command %d targets", len(entry.Targets))
	}
	if m.isChineseUI() {
		return "单台命令"
	}
	return "Single command"
}

func (m Model) historyStatusText(status string) string {
	if status == "failed" {
		return redStyle.Render(m.t("Failed", "失败"))
	}
	return greenStyle.Render(m.t("Success", "成功"))
}

func (m Model) historyStatusPlain(status string) string {
	if status == "failed" {
		return m.t("Failed", "失败")
	}
	return m.t("Success", "成功")
}

func historyTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "--"
	}
	return t.Local().Format("01-02 15:04")
}

func historyTimeFull(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return value
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func (m Model) historyTargetsText(entry config.CommandHistoryEntry, limit int) string {
	if len(entry.Targets) == 0 {
		return "-"
	}
	names := make([]string, 0, len(entry.Targets))
	for _, target := range entry.Targets {
		names = append(names, m.historyTargetName(target))
	}
	if limit > 0 && len(names) > limit {
		return fmt.Sprintf("%s %s%d", names[0], m.t("and ", "等"), len(names))
	}
	if m.isChineseUI() {
		return strings.Join(names, "、")
	}
	return strings.Join(names, ", ")
}

func (m Model) historyTargetName(target config.CommandHistoryTarget) string {
	category := strings.TrimSpace(target.Category)
	name := strings.TrimSpace(target.Name)
	if category == "" {
		return name
	}
	return "[" + m.displayCategoryName(category) + "] " + name
}

func (m Model) renderHelpPanel() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	rows := []struct {
		key  string
		desc string
	}{
		{"↑↓←→ / hjkl", m.t("Move selection", "移动选择")},
		{"Enter", m.t("Login server", "登录服务器")},
		{"Space", m.t("View details", "查看详情")},
		{"m", m.t("Command templates", "命令模板")},
		{"b", m.t("Batch commands", "批量命令")},
		{"i", m.t("Command history", "命令历史")},
		{"y", m.t("Transfer jobs", "传输任务")},
		{"g", m.t("App deployment", "应用部署")},
		{"n", m.t("Container and service resources", "容器和服务资源")},
		{".", m.t("Settings", "设置")},
		{"w", m.t("Problem overview", "异常总览")},
		{"z", m.t("Switch dashboard view", "切换首页视图")},
		{"t", m.t("Pin / unpin", "置顶 / 取消置顶")},
		{"f", m.t("Favorite / unfavorite", "收藏 / 取消收藏")},
		{"v", m.t("Favorites only / clear filter", "只看收藏 / 取消筛选")},
		{"a", m.t("Add server", "添加服务器")},
		{"c", m.t("Copy server", "复制服务器")},
		{"e", m.t("Edit server", "编辑服务器")},
		{"x", m.t("Delete server", "删除服务器")},
		{"u", m.t("Upload file or directory", "上传文件或目录")},
		{"d", m.t("Download file or directory", "下载文件或目录")},
		{"r", m.t("Refresh monitoring", "刷新监控")},
		{"/", m.t("Search", "搜索")},
		{"Tab", m.t("Switch category", "切换分类")},
		{"o", m.t("Online only / clear filter", "只看在线 / 取消筛选")},
		{"p", m.t("Problems only / clear filter", "只看异常 / 取消筛选")},
		{"s", m.t("Switch sort", "切换排序")},
		{"q / Esc", m.t("Quit or go back", "退出或返回")},
		{"?", m.t("Close help", "关闭帮助")},
	}
	lines := []string{}
	for _, row := range rows {
		lines = append(lines, modalLine(row.key, row.desc, bodyWidth))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(m.t("Shortcuts", "快捷键"), width)),
		box,
		renderHelp(width, m.t("Back q/Esc/?", "返回 q/Esc/?")),
	}, "\n")
}

type anomalyItem struct {
	Index  int
	Checks []checkItem
}

func (m Model) anomalyItems() []anomalyItem {
	items := make([]anomalyItem, 0)
	for i, state := range m.states {
		checks := actionableChecks(m.buildChecks(state))
		if len(checks) == 0 {
			continue
		}
		if !m.anomalyMatchesFilter(checks) {
			continue
		}
		items = append(items, anomalyItem{Index: i, Checks: checks})
	}
	sort.SliceStable(items, func(i, j int) bool {
		aSevere, aWarn, aTip := checkCounts(items[i].Checks)
		bSevere, bWarn, bTip := checkCounts(items[j].Checks)
		if aSevere != bSevere {
			return aSevere > bSevere
		}
		if aWarn != bWarn {
			return aWarn > bWarn
		}
		if aTip != bTip {
			return aTip > bTip
		}
		aHost := m.states[items[i].Index].Host
		bHost := m.states[items[j].Index].Host
		if aHost.Category == bHost.Category {
			return aHost.Name < bHost.Name
		}
		return aHost.Category < bHost.Category
	})
	return items
}

func (m Model) anomalyMatchesFilter(checks []checkItem) bool {
	switch m.anomaly.Filter {
	case anomalySevere:
		for _, check := range checks {
			if check.Level == "严重" {
				return true
			}
		}
		return false
	case anomalyWarn:
		for _, check := range checks {
			if check.Level == "警告" {
				return true
			}
		}
		return false
	case anomalyOffline:
		return checksContainKind(checks, "offline")
	case anomalyResource:
		return checksContainKind(checks, "resource")
	case anomalyContainer:
		return checksContainKind(checks, "container")
	case anomalyService:
		return checksContainKind(checks, "service")
	case anomalySecurity:
		return checksContainKind(checks, "security")
	default:
		return true
	}
}

func checksContainKind(checks []checkItem, kind string) bool {
	for _, check := range checks {
		if checkKind(check) == kind {
			return true
		}
	}
	return false
}

func actionableChecks(checks []checkItem) []checkItem {
	out := make([]checkItem, 0, len(checks))
	for _, check := range checks {
		if check.Level == "严重" || check.Level == "警告" {
			out = append(out, check)
		}
	}
	return out
}

func checkCounts(checks []checkItem) (int, int, int) {
	severe := 0
	warn := 0
	tip := 0
	for _, check := range checks {
		switch check.Level {
		case "严重":
			severe++
		case "警告":
			warn++
		case "提示":
			tip++
		}
	}
	return severe, warn, tip
}
