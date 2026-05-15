package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderAnomalyOverview() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 42 {
		bodyWidth = 42
	}
	items := m.anomalyItems()
	m.anomalyIndex = clampInt(m.anomalyIndex, 0, maxInt(0, len(items)-1))
	totalSevere, totalWarn := 0, 0
	for _, item := range items {
		severe, warn, _ := checkCounts(item.Checks)
		totalSevere += severe
		totalWarn += warn
	}
	title := fmt.Sprintf("异常总览  %d台  %s", len(items), anomalyFilterName(m.anomalyFilter))
	if totalSevere > 0 {
		title += "  " + redStyle.Render(fmt.Sprintf("严重%d", totalSevere))
	}
	if totalWarn > 0 {
		title += "  " + yellowStyle.Render(fmt.Sprintf("警告%d", totalWarn))
	}
	if m.refreshStatus != "" {
		title += "  " + m.refreshStatus
	}
	contentHeight := m.height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	lines := []string{}
	if len(items) == 0 {
		lines = append(lines, greenStyle.Render("没有发现严重或警告级别的问题。"))
		lines = append(lines, mutedStyle.Render("提示级别的问题仍可在服务器详情的风险页查看。"))
	} else {
		itemHeight := 3
		rowsVisible := contentHeight / itemHeight
		if rowsVisible < 1 {
			rowsVisible = 1
		}
		start, end := visibleRange(len(items), m.anomalyIndex, rowsVisible)
		if end <= start {
			end = minInt(len(items), start+1)
		}
		lastGroup := ""
		for i := start; i < end; i++ {
			group := anomalyGroupName(items[i].Checks)
			if group != lastGroup {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, anomalyGroupTitle(group))
				lastGroup = group
			}
			if len(lines)+itemHeight > contentHeight {
				break
			}
			lines = append(lines, m.anomalyItemLines(items[i], i == m.anomalyIndex, bodyWidth)...)
			lines = append(lines, "")
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fitANSI(title, width)),
		box,
		renderHelp(width, "移动 ↑↓/jk  详情 Enter/Space  筛选 f/Tab  全部0 严重1 警告2 离线3 资源4 容器5 服务6 安全7  刷新 r  返回 q/Esc"),
	}, "\n")
}

func (m Model) anomalyItemLines(item anomalyItem, selected bool, width int) []string {
	state := m.states[item.Index]
	h := state.Host
	metrics := state.Metrics
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	severe, warn, _ := checkCounts(item.Checks)
	summary := []string{}
	if severe > 0 {
		summary = append(summary, redStyle.Render(fmt.Sprintf("严重%d", severe)))
	}
	if warn > 0 {
		summary = append(summary, yellowStyle.Render(fmt.Sprintf("警告%d", warn)))
	}
	name := hostDisplayName(h)
	status := "离线"
	if state.Loading {
		status = "采集中"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := 30
	if width < 90 {
		nameWidth = 24
	}
	if width < 72 {
		nameWidth = 18
	}
	nameText := nameStyle.Render(padVisible(fitANSI(name, nameWidth), nameWidth))
	riskText := padVisible(strings.Join(summary, " "), 10)
	statusText := padVisible(colorStatus(status, state.Loading, metrics.Online), 6)
	mainLine := fmt.Sprintf("%s %s  %s  %s  %s  %s",
		prefix,
		nameText,
		statusText,
		riskText,
		anomalyResourceText(state),
		serviceCardText(metrics),
	)
	reasons := make([]string, 0, minInt(3, len(item.Checks)))
	for _, check := range item.Checks {
		reasons = append(reasons, stripCheckPrefix(check.Text))
		if len(reasons) >= 3 {
			break
		}
	}
	reasonLine := "  " + mutedStyle.Render("问题 ") + detailValueStyle.Render(strings.Join(reasons, "；"))
	return []string{
		fitANSI(mainLine, width),
		fitANSI(reasonLine, width),
	}
}

func anomalyGroupName(checks []checkItem) string {
	severe, _, _ := checkCounts(checks)
	if severe > 0 {
		return "严重"
	}
	return "警告"
}

func anomalyGroupTitle(group string) string {
	if group == "严重" {
		return detailDangerSubTitle("严重")
	}
	return detailSubTitle("警告")
}

func anomalyFilterName(filter anomalyFilterMode) string {
	switch filter {
	case anomalySevere:
		return "严重"
	case anomalyWarn:
		return "警告"
	case anomalyOffline:
		return "离线"
	case anomalyResource:
		return "资源"
	case anomalyContainer:
		return "容器"
	case anomalyService:
		return "服务"
	case anomalySecurity:
		return "安全"
	default:
		return "全部"
	}
}

func anomalyResourceText(state hostState) string {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render("CPU -  内存 -  磁盘 -")
	}
	return strings.Join([]string{
		"CPU " + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%.0f%%", metrics.CPUPercent)),
		"内存 " + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%.0f%%", metrics.MemPercent())),
		"磁盘 " + diskMountPercentText(metrics),
	}, "  ")
}

func stripCheckPrefix(value string) string {
	value = strings.TrimSpace(value)
	for _, sep := range []string{"：风险，", "：警告，", "：提示，"} {
		if strings.Contains(value, sep) {
			parts := strings.SplitN(value, sep, 2)
			return strings.TrimSpace(parts[0] + "：" + parts[1])
		}
	}
	return value
}

func anomalyDetailSection(checks []checkItem) string {
	priority := []struct {
		Kind    string
		Section string
	}{
		{"offline", "基础信息"},
		{"expire", "基础信息"},
		{"container", "容器"},
		{"service", "服务状态"},
		{"security", "登录记录"},
		{"resource", "资源监控"},
	}
	for _, item := range priority {
		for _, check := range checks {
			if checkKind(check) == item.Kind {
				return item.Section
			}
		}
	}
	return "风险提示"
}

func checkKind(check checkItem) string {
	text := strings.TrimSpace(check.Text)
	switch {
	case strings.HasPrefix(text, "服务器到期："):
		return "expire"
	case strings.HasPrefix(text, "服务器状态："):
		return "offline"
	case strings.HasPrefix(text, "CPU使用：") ||
		strings.HasPrefix(text, "内存使用：") ||
		strings.HasPrefix(text, "磁盘容量："):
		return "resource"
	case strings.HasPrefix(text, "容器状态：") ||
		strings.HasPrefix(text, "容器详情："):
		return "container"
	case strings.HasPrefix(text, "系统服务：") ||
		strings.HasPrefix(text, "健康端口：") ||
		strings.HasPrefix(text, "端口详情："):
		return "service"
	case strings.HasPrefix(text, "允许密码登录：") ||
		strings.HasPrefix(text, "允许root登录：") ||
		strings.HasPrefix(text, "密钥登录：") ||
		strings.HasPrefix(text, "SSH端口：") ||
		strings.HasPrefix(text, "SSH配置检查：") ||
		strings.HasPrefix(text, "失败登录来源IP过多："):
		return "security"
	default:
		return "other"
	}
}
