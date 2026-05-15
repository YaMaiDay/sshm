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
	title := fmt.Sprintf("%s  %d%s  %s", m.t("Anomaly Overview", "异常总览"), len(items), m.t(" servers", "台"), m.anomalyFilterName(m.anomalyFilter))
	if totalSevere > 0 {
		title += "  " + redStyle.Render(fmt.Sprintf("%s%d", m.t("Critical", "严重"), totalSevere))
	}
	if totalWarn > 0 {
		title += "  " + yellowStyle.Render(fmt.Sprintf("%s%d", m.t("Warn", "警告"), totalWarn))
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
		lines = append(lines, greenStyle.Render(m.t("No critical or warning issues found.", "没有发现严重或警告级别的问题。")))
		lines = append(lines, mutedStyle.Render(m.t("Info-level issues are still available on the server detail risk page.", "提示级别的问题仍可在服务器详情的风险页查看。")))
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
				lines = append(lines, m.anomalyGroupTitle(group))
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
		renderHelp(width, m.t("Move ↑↓/jk  Detail Enter/Space  Filter f/Tab  All 0  Critical 1  Warn 2  Offline 3  Resource 4  Container 5  Service 6  Security 7  Refresh r  Back q/Esc", "移动 ↑↓/jk  详情 Enter/Space  筛选 f/Tab  全部0 严重1 警告2 离线3 资源4 容器5 服务6 安全7  刷新 r  返回 q/Esc")),
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
		summary = append(summary, redStyle.Render(fmt.Sprintf("%s%d", m.t("Critical", "严重"), severe)))
	}
	if warn > 0 {
		summary = append(summary, yellowStyle.Render(fmt.Sprintf("%s%d", m.t("Warn", "警告"), warn)))
	}
	name := hostDisplayName(h)
	status := m.t("Offline", "离线")
	if state.Loading {
		status = m.t("Loading", "采集中")
	} else if metrics.Online {
		status = m.t("Online", "在线")
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
		m.anomalyResourceText(state),
		m.serviceCardText(metrics),
	)
	reasons := make([]string, 0, minInt(3, len(item.Checks)))
	for _, check := range item.Checks {
		reasons = append(reasons, m.anomalyCheckReason(check.Text))
		if len(reasons) >= 3 {
			break
		}
	}
	reasonLine := "  " + mutedStyle.Render(m.t("Issue ", "问题 ")) + detailValueStyle.Render(strings.Join(reasons, m.t("; ", "；")))
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

func (m Model) anomalyGroupTitle(group string) string {
	if group == "严重" {
		return detailDangerSubTitle(m.t("Critical", "严重"))
	}
	return detailSubTitle(m.t("Warn", "警告"))
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

func (m Model) anomalyFilterName(filter anomalyFilterMode) string {
	if m.isChineseUI() {
		return anomalyFilterName(filter)
	}
	switch filter {
	case anomalySevere:
		return "Critical"
	case anomalyWarn:
		return "Warn"
	case anomalyOffline:
		return "Offline"
	case anomalyResource:
		return "Resource"
	case anomalyContainer:
		return "Container"
	case anomalyService:
		return "Service"
	case anomalySecurity:
		return "Security"
	default:
		return "All"
	}
}

func (m Model) anomalyResourceText(state hostState) string {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render(m.t("CPU -  Mem -  Disk -", "CPU -  内存 -  磁盘 -"))
	}
	thresholds := m.metricThresholds()
	return strings.Join([]string{
		"CPU " + metricValueStyle(metrics.CPUPercent, thresholds.CPUWarn, thresholds.CPUCrit).Render(fmt.Sprintf("%.0f%%", metrics.CPUPercent)),
		m.t("Mem ", "内存 ") + metricValueStyle(metrics.MemPercent(), thresholds.MemWarn, thresholds.MemCrit).Render(fmt.Sprintf("%.0f%%", metrics.MemPercent())),
		m.t("Disk ", "磁盘 ") + m.diskMountPercentText(metrics),
	}, "  ")
}

func (m Model) anomalyCheckReason(value string) string {
	if m.isChineseUI() {
		return stripCheckPrefix(value)
	}
	return m.checkText(value)
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
