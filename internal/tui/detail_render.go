package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderDeleteConfirm() string {
	if m.deleteIndex < 0 || m.deleteIndex >= len(m.states) {
		return m.t("No selected server", "没有选中的服务器")
	}
	h := m.states[m.deleteIndex].Host
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{
		wrapPlainLine(m.t("Server: ", "服务器：")+h.Name, bodyWidth),
		wrapPlainLine(m.t("File: ", "文件：")+h.File, bodyWidth),
		"",
		m.t("This server configuration will be deleted.", "将删除该服务器配置。"),
	}
	return m.renderDangerConfirm(m.t("Delete Server", "确认删除服务器"), lines, width)
}

func (m Model) renderConfirmAction() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{}
	for _, line := range m.confirm.Lines {
		lines = append(lines, wrapPlainLine(line, bodyWidth))
	}
	return m.renderDangerConfirm(m.confirm.Title, lines, width)
}

func (m Model) renderDangerConfirm(title string, lines []string, width int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(red).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, m.t("Confirm Enter/y  Cancel Esc/n", "确认 Enter/y  取消 Esc/n")),
	}, "\n")
}

func (m Model) renderDetail() string {
	lines, ok := m.detailLines()
	if !ok {
		return m.t("No selected server", "没有选中的服务器")
	}
	idx, _ := m.selectedRealIndex()
	width := detailFrameWidth(m.width)
	headerText := m.t("Server Detail  ", "服务器详情  ") + hostDisplayName(m.states[idx].Host)
	if checks := m.currentDetailChecks(); len(checks) > 0 {
		headerText += "  " + m.riskSummaryText(checks)
	}
	header := titleStyle.Render(fitANSI(headerText, width))
	help := renderHelp(width, m.t("Scroll ↑↓/jk  Section ←→/Tab  Login l  Command m  Upload u  Download d  Refresh r  Back q/Esc", "滚动 ↑↓/jk  分类 ←→/Tab  登录 l  命令 m  上传 u  下载 d  刷新 r  返回 q/Esc"))
	tabs := m.renderDetailSectionTabs(width)
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	bodyContent := tabs + "\n" + detailFrameSeparator(width-2) + "\n" + strings.Join(lines, "\n")
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(bodyContent)
	return strings.Join([]string{
		header,
		body,
		help,
	}, "\n")
}

func (m Model) renderDetailSectionTabs(width int) string {
	sections := m.detailSectionNames()
	activeIndex := m.detailSectionIndex
	if len(sections) > 0 && activeIndex >= len(sections) {
		activeIndex = len(sections) - 1
	}
	contentWidth := width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}
	parts := make([]string, 0, len(sections))
	for i, section := range sections {
		label := shortDetailSectionName(section)
		if i == activeIndex {
			parts = append(parts, titleStyle.Render(label))
		} else {
			parts = append(parts, mutedStyle.Render(label))
		}
	}
	value := strings.Join(parts, "  ")
	if ansi.StringWidth(value) > contentWidth && activeIndex > 0 {
		value = strings.Join(parts[activeIndex:], "  ")
	}
	line := padVisible(fitANSI(value, contentWidth), contentWidth)
	return line
}

func detailFrameSeparator(width int) string {
	if width < 1 {
		width = 1
	}
	return cardBorderStyle.Render(strings.Repeat("─", width))
}

func shortDetailSectionName(section string) string {
	switch section {
	case "基础信息":
		return "基础"
	case "资源监控":
		return "资源"
	case "服务状态":
		return "服务"
	case "登录记录":
		return "登录"
	case "风险提示":
		return "风险"
	case "Basic":
		return "Basic"
	case "Resources":
		return "Res"
	case "Services":
		return "Svc"
	case "Login Records":
		return "Login"
	case "Risks":
		return "Risk"
	case "Containers":
		return "Cont"
	case "Recent Error":
		return "Error"
	default:
		return section
	}
}

func (m Model) detailLines() ([]string, bool) {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return nil, false
	}
	state := m.states[idx]
	h := state.Host
	metrics := state.Metrics

	status := m.t("Offline", "离线")
	if state.Loading {
		status = m.t("Loading", "采集中")
	} else if metrics.Online {
		status = m.t("Online", "在线")
	}

	lines := []string{
		sectionTitle(m.t("Basic", "基础信息")),
		m.detailRow(m.t("Status", "状态"), colorStatus(status, state.Loading, metrics.Online)),
		m.detailRow(m.t("Address", "地址"), h.Address()),
		m.detailRow(m.t("User", "用户"), h.User),
		m.detailRow(m.t("Port", "端口"), h.Port),
		m.detailRow(m.t("Category", "分类"), emptyDash(h.Category)),
		m.detailRow(m.t("Favorite", "收藏"), yesNoLang(h.Favorite, m.isChineseUI())),
		m.detailRow(m.t("Pinned", "置顶"), yesNoLang(h.Pinned, m.isChineseUI())),
		m.detailRow(m.t("Auth", "认证方式"), m.authText(h)),
		m.detailRow(m.t("Bastion", "跳板机"), m.jumpDetailText(h)),
		m.detailRow(m.t("Bastion key", "跳板机密钥"), m.jumpKeyText(h)),
		m.detailRow(m.t("Hostname", "主机名"), emptyDash(metrics.RemoteHostname)),
		m.detailRow(m.t("OS", "系统"), emptyDash(metrics.OS)),
		m.detailRow(m.t("Kernel", "内核"), emptyDash(metrics.Kernel)),
		m.detailRow(m.t("Arch", "架构"), emptyDash(metrics.Arch)),
		m.detailRow(m.t("Source", "来源"), h.File),
		m.detailRow(m.t("Expire at", "到期时间"), emptyDash(h.ExpireAt)),
		m.detailRow(m.t("Remaining", "剩余时间"), m.expireDetailText(h.ExpireAt)),
		m.detailRow(m.t("Note", "备注"), emptyDash(h.Note)),
		m.detailRow(m.t("Last login", "最近登录"), m.lastLoginDetail(m.lastLogin(h))),
	}
	thresholds := m.metricThresholds()
	checks := m.buildChecks(state)
	lines = append(lines,
		"",
		sectionTitle(m.t("Resources", "资源监控")),
		detailSubTitle("CPU"),
		m.detailRow(m.t("Usage", "使用率"), percentBarWithThreshold(metrics.CPUPercent, thresholds.CPUWarn, thresholds.CPUCrit)),
		m.detailRow(m.t("Cores", "核心数"), m.cpuCoresText(metrics)),
		m.detailRow(m.t("Model", "型号"), emptyDash(metrics.CPUModel)),
		"",
		detailSubTitle(m.t("Memory", "内存")),
		m.detailRow(m.t("Usage", "使用率"), fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.MemPercent(), thresholds.MemWarn, thresholds.MemCrit), bytesHuman(metrics.MemUsed), bytesHuman(metrics.MemTotal))),
		m.detailRow(m.t("Available", "可用"), bytesHuman(metrics.MemAvailable)),
		m.detailRow("Swap", m.swapUsageText(metrics)),
		m.detailRow(m.t("Swap free", "Swap可用"), swapFreeText(metrics)),
		"",
		detailSubTitle(m.t("Disk", "磁盘")),
		m.detailRow(m.t("Mount", "挂载点"), emptyDash(metrics.DiskMountpoint)),
		m.detailRow(m.t("Filesystem", "文件系统"), emptyDash(metrics.DiskFilesystem)),
		m.detailRow(m.t("Type", "类型"), emptyDash(metrics.DiskType)),
		m.detailRow(m.t("Usage", "使用率"), fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.DiskPercent(), thresholds.DiskWarn, thresholds.DiskCrit), bytesHuman(metrics.DiskUsed), bytesHuman(metrics.DiskTotal))),
		m.detailRow(m.t("Available", "可用"), bytesHuman(metrics.DiskAvailable)),
		m.diskListText(metrics),
		m.detailRow(m.t("Inodes", "索引节点"), m.inodeUsageText(metrics)),
		m.detailRow(m.t("Free inodes", "可用节点"), countHuman(metrics.InodeAvailable)),
		"",
		detailSubTitle(m.t("System", "系统")),
		m.detailRow(m.t("Load", "负载"), fmt.Sprintf("%s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))),
		m.detailRow(m.t("Uptime", "运行时间"), m.uptimeText(metrics.Uptime)),
		"",
		sectionTitle(m.t("Services", "服务状态")),
		detailSubTitle(m.t("Health", "健康")),
		m.detailRow(m.t("Health ports", "健康端口"), healthPortsText(metrics)),
		"",
		detailSubTitle(m.t("Services", "服务")),
	)
	lines = append(lines, serviceDetailSummaryRows(m, metrics, state)...)
	lines = append(lines,
		"",
		detailSubTitle(m.t("Service details", "服务详情")),
	)
	lines = append(lines, serviceDetailRows(m, metrics, state)...)
	lines = append(lines,
		"",
		detailSubTitle(m.t("Ports", "端口")),
	)
	lines = append(lines, portDetailRows(m, state)...)
	lines = append(lines,
		"",
		sectionTitle(m.t("Containers", "容器")),
		detailSubTitle(m.t("Status", "状态")),
	)
	lines = append(lines, dockerDetailRows(m, metrics, state)...)
	lines = append(lines, "", detailSubTitle(m.t("Details", "详情")))
	lines = append(lines, containerDetailRows(m, state)...)
	if metrics.Error != "" {
		lines = append(lines, "", sectionTitle(m.t("Recent Error", "最近错误")), m.detailRow(m.t("Error", "错误"), metrics.Error))
	}
	lines = append(lines, "", sectionTitle(m.t("Login Records", "登录记录")), detailSuccessSubTitle(m.t("Success", "成功")))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.LoginSummary, state.LoginError, false)...)
	lines = append(lines, "", detailDangerSubTitle(m.t("Failed", "失败")))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.FailedLoginSummary, state.FailedLoginError, true)...)
	lines = append(lines, "", sectionTitle(m.t("Risks", "风险提示")))
	lines = append(lines, checkSuggestionRows(m, state, checks)...)
	lines = m.activeDetailSectionLines(lines)
	return lines, true
}

func (m Model) currentDetailChecks() []checkItem {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return nil
	}
	return m.buildChecks(m.states[idx])
}

func (m Model) activeDetailSectionLines(lines []string) []string {
	sections := m.detailSectionNames()
	if len(sections) == 0 {
		return lines
	}
	index := clampInt(m.detailSectionIndex, 0, len(sections)-1)
	target := sections[index]
	out := []string{}
	inSection := false
	for _, line := range lines {
		name, isSection := detailSectionNameFromLine(line)
		if isSection {
			if name == target {
				inSection = true
				out = append(out, m.renderDetailSectionLine(name, line))
				continue
			}
			if inSection {
				break
			}
			continue
		}
		if inSection {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return []string{m.renderDetailSectionLine(target, sectionTitle(target)), m.detailRow(m.t("Status", "状态"), m.t("No content", "暂无内容"))}
	}
	return out
}

func (m Model) renderDetailSectionLine(name string, fallback string) string {
	sections := m.detailSectionNames()
	selected := false
	if m.detailSectionIndex >= 0 && m.detailSectionIndex < len(sections) {
		selected = sections[m.detailSectionIndex] == name
	}
	marker := "  "
	style := detailSectionStyle
	if selected {
		marker = "▶ "
		style = blueStyle.Bold(true)
	}
	if name == "" {
		return fallback
	}
	return marker + style.Render("["+name+"]")
}

func detailSectionNameFromLine(line string) (string, bool) {
	plain := ansi.Strip(line)
	plain = strings.TrimSpace(strings.TrimPrefix(plain, "▶"))
	if !strings.HasPrefix(plain, "[") {
		return "", false
	}
	start := strings.Index(plain, "[")
	end := strings.Index(plain, "]")
	if start < 0 || end <= start {
		return "", false
	}
	return plain[start+1 : end], true
}

func (m Model) detailViewportHeight() int {
	height := m.height - 6
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) detailMaxScroll() int {
	lines, ok := m.detailLines()
	if !ok {
		return 0
	}
	maxScroll := len(lines) - m.detailViewportHeight()
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}
