package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderDeleteConfirm() string {
	if m.deleteIndex < 0 || m.deleteIndex >= len(m.states) {
		return "没有选中的服务器"
	}
	h := m.states[m.deleteIndex].Host
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{
		wrapPlainLine("服务器："+h.Name, bodyWidth),
		wrapPlainLine("文件："+h.File, bodyWidth),
		"",
		"将删除该服务器配置。",
	}
	return renderDangerConfirm("确认删除服务器", lines, width)
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
	return renderDangerConfirm(m.confirm.Title, lines, width)
}

func renderDangerConfirm(title string, lines []string, width int) string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(red).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, "确认 Enter/y  取消 Esc/n"),
	}, "\n")
}

func (m Model) renderDetail() string {
	lines, ok := m.detailLines()
	if !ok {
		return "没有选中的服务器"
	}
	idx, _ := m.selectedRealIndex()
	width := detailFrameWidth(m.width)
	headerText := "服务器详情  " + hostDisplayName(m.states[idx].Host)
	if checks := m.currentDetailChecks(); len(checks) > 0 {
		headerText += "  " + riskSummaryText(checks)
	}
	header := titleStyle.Render(fitANSI(headerText, width))
	help := renderHelp(width, "滚动 ↑↓/jk  分类 ←→/Tab  登录 l  命令 m  上传 u  下载 d  刷新 r  返回 q/Esc")
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

	status := "离线"
	if state.Loading {
		status = "采集中"
	} else if metrics.Online {
		status = "在线"
	}

	lines := []string{
		sectionTitle("基础信息"),
		m.detailRow("状态", colorStatus(status, state.Loading, metrics.Online)),
		m.detailRow("地址", h.Address()),
		m.detailRow("用户", h.User),
		m.detailRow("端口", h.Port),
		m.detailRow("分类", emptyDash(h.Category)),
		m.detailRow("收藏", yesNo(h.Favorite)),
		m.detailRow("置顶", yesNo(h.Pinned)),
		m.detailRow("认证方式", authText(h)),
		m.detailRow("跳板机", jumpDetailText(h)),
		m.detailRow("跳板机密钥", jumpKeyText(h)),
		m.detailRow("主机名", emptyDash(metrics.RemoteHostname)),
		m.detailRow("系统", emptyDash(metrics.OS)),
		m.detailRow("内核", emptyDash(metrics.Kernel)),
		m.detailRow("架构", emptyDash(metrics.Arch)),
		m.detailRow("来源", h.File),
		m.detailRow("到期时间", emptyDash(h.ExpireAt)),
		m.detailRow("剩余时间", expireDetailText(h.ExpireAt)),
		m.detailRow("备注", emptyDash(h.Note)),
		m.detailRow("最近登录", lastLoginDetail(m.lastLogin(h))),
	}
	checks := buildChecks(state)
	lines = append(lines,
		"",
		sectionTitle("资源监控"),
		detailSubTitle("CPU"),
		m.detailRow("使用率", percentBar(metrics.CPUPercent)),
		m.detailRow("核心数", cpuCoresText(metrics)),
		m.detailRow("型号", emptyDash(metrics.CPUModel)),
		"",
		detailSubTitle("内存"),
		m.detailRow("使用率", fmt.Sprintf("%s  %s / %s", percentBar(metrics.MemPercent()), bytesHuman(metrics.MemUsed), bytesHuman(metrics.MemTotal))),
		m.detailRow("可用", bytesHuman(metrics.MemAvailable)),
		m.detailRow("Swap", swapUsageText(metrics)),
		m.detailRow("Swap可用", swapFreeText(metrics)),
		"",
		detailSubTitle("磁盘"),
		m.detailRow("挂载点", emptyDash(metrics.DiskMountpoint)),
		m.detailRow("文件系统", emptyDash(metrics.DiskFilesystem)),
		m.detailRow("类型", emptyDash(metrics.DiskType)),
		m.detailRow("使用率", fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.DiskPercent(), 80, 90), bytesHuman(metrics.DiskUsed), bytesHuman(metrics.DiskTotal))),
		m.detailRow("可用", bytesHuman(metrics.DiskAvailable)),
		m.diskListText(metrics),
		m.detailRow("索引节点", inodeUsageText(metrics)),
		m.detailRow("可用节点", countHuman(metrics.InodeAvailable)),
		"",
		detailSubTitle("系统"),
		m.detailRow("负载", fmt.Sprintf("%s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))),
		m.detailRow("运行时间", uptimeCN(metrics.Uptime)),
		"",
		sectionTitle("服务状态"),
		detailSubTitle("健康"),
		m.detailRow("健康端口", healthPortsText(metrics)),
		"",
		detailSubTitle("服务"),
	)
	lines = append(lines, serviceDetailSummaryRows(m, metrics, state)...)
	lines = append(lines,
		"",
		detailSubTitle("服务详情"),
	)
	lines = append(lines, serviceDetailRows(m, metrics, state)...)
	lines = append(lines,
		"",
		detailSubTitle("端口"),
	)
	lines = append(lines, portDetailRows(m, state)...)
	lines = append(lines,
		"",
		sectionTitle("容器"),
		detailSubTitle("状态"),
	)
	lines = append(lines, dockerDetailRows(m, metrics, state)...)
	lines = append(lines, "", detailSubTitle("详情"))
	lines = append(lines, containerDetailRows(m, state)...)
	if metrics.Error != "" {
		lines = append(lines, "", sectionTitle("最近错误"), m.detailRow("错误", metrics.Error))
	}
	lines = append(lines, "", sectionTitle("登录记录"), detailSuccessSubTitle("成功"))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.LoginSummary, state.LoginError, false)...)
	lines = append(lines, "", detailDangerSubTitle("失败"))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.FailedLoginSummary, state.FailedLoginError, true)...)
	lines = append(lines, "", sectionTitle("风险提示"))
	lines = append(lines, checkSuggestionRows(m, state, checks)...)
	lines = m.activeDetailSectionLines(lines)
	return lines, true
}

func (m Model) currentDetailChecks() []checkItem {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return nil
	}
	return buildChecks(m.states[idx])
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
		return []string{m.renderDetailSectionLine(target, sectionTitle(target)), m.detailRow("状态", "暂无内容")}
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
