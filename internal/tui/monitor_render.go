package tui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

func failedServiceText(metrics monitor.Metrics, limit int) string {
	if metrics.FailedServices <= 0 {
		return "0"
	}
	if len(metrics.FailedUnits) == 0 {
		return fmt.Sprintf("%d", metrics.FailedServices)
	}
	if limit <= 0 || limit > len(metrics.FailedUnits) {
		limit = len(metrics.FailedUnits)
	}
	names := append([]string{}, metrics.FailedUnits[:limit]...)
	if len(metrics.FailedUnits) > limit {
		names = append(names, fmt.Sprintf("等%d个", metrics.FailedServices))
	}
	return fmt.Sprintf("%d（%s）", metrics.FailedServices, strings.Join(names, "、"))
}

func dockerTotal(metrics monitor.Metrics) int {
	if metrics.DockerTotal > 0 {
		return metrics.DockerTotal
	}
	return metrics.DockerRunning
}

func dockerRunningText(metrics monitor.Metrics, limit int) string {
	if metrics.DockerRunning <= 0 {
		return "-"
	}
	if len(metrics.DockerRunningNames) == 0 {
		return fmt.Sprintf("%d", metrics.DockerRunning)
	}
	if limit <= 0 || limit > len(metrics.DockerRunningNames) {
		limit = len(metrics.DockerRunningNames)
	}
	names := append([]string{}, metrics.DockerRunningNames[:limit]...)
	if len(metrics.DockerRunningNames) > limit {
		names = append(names, fmt.Sprintf("等%d个", metrics.DockerRunning))
	}
	return strings.Join(names, "、")
}

func dockerStoppedText(metrics monitor.Metrics, limit int) string {
	return limitedDockerNames(metrics.DockerStoppedNames, metrics.DockerStopped, limit)
}

func dockerFailedText(metrics monitor.Metrics, limit int) string {
	return limitedDockerNames(metrics.DockerFailedNames, metrics.DockerFailed, limit)
}

func limitedDockerNames(names []string, count int, limit int) string {
	if count <= 0 {
		return "-"
	}
	if len(names) == 0 {
		return fmt.Sprintf("%d", count)
	}
	if limit <= 0 || limit > len(names) {
		limit = len(names)
	}
	out := append([]string{}, names[:limit]...)
	if len(names) > limit {
		out = append(out, fmt.Sprintf("等%d个", count))
	}
	return strings.Join(out, "、")
}

func dockerDetailRows(m Model, metrics monitor.Metrics, state hostState) []string {
	total := dockerTotal(metrics)
	if len(state.ContainerDetails) > 0 {
		total = len(state.ContainerDetails)
	}
	lines := []string{}
	if total == 0 {
		lines = append(lines, m.detailRow("状态", "未发现"))
	} else {
		running, stopped, failed := containerDetailCounts(state.ContainerDetails)
		if len(state.ContainerDetails) == 0 && (metrics.DockerRunning > 0 || metrics.DockerStopped > 0 || metrics.DockerFailed > 0) {
			running = metrics.DockerRunning
			stopped = metrics.DockerStopped
			failed = metrics.DockerFailed
		}
		lines = append(lines,
			m.detailRow("总数", fmt.Sprintf("%d", total)),
			m.detailRow("运行", fmt.Sprintf("%d", running)),
			m.detailRow("停止", fmt.Sprintf("%d", stopped)),
			m.detailRow("故障", fmt.Sprintf("%d", failed)),
		)
	}
	return lines
}

func portDetailRows(m Model, state hostState) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(state.PortDetailsError) != "" {
		return []string{m.detailRow("状态", redStyle.Render(state.PortDetailsError))}
	}
	if len(state.PortDetails) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	groups := groupedPortDetails(state.PortDetails)
	lines := []string{}
	groupDefs := []struct {
		Title string
		Key   string
	}{
		{"系统端口", "system"},
		{"Docker端口", "docker"},
		{"应用端口", "app"},
	}
	first := true
	for _, group := range groupDefs {
		items := groups[group.Key]
		if len(items) == 0 {
			continue
		}
		if !first {
			lines = append(lines, "")
		}
		first = false
		lines = append(lines, detailSubTitle(fmt.Sprintf("%s %d", group.Title, len(items))))
		lines = append(lines, portDetailItemRows(m, items)...)
	}
	return lines
}

func portDetailItemRows(m Model, items []portDetail) []string {
	labelWidth := len("tcp/10000")
	processWidth := len("docker-proxy")
	for _, item := range items {
		label := strings.TrimSpace(item.Protocol + "/" + item.Port)
		if width := runewidth.StringWidth(label); width > labelWidth {
			labelWidth = width
		}
		process := portProcessText(item)
		if process == "" {
			process = "-"
		}
		if width := runewidth.StringWidth(process); width > processWidth {
			processWidth = width
		}
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		process := portProcessText(item)
		if process == "" {
			process = "-"
		}
		pid := item.PID
		if pid == "" {
			pid = "-"
		}
		if item.Count > 1 {
			pid = fmt.Sprintf("%s 等%d个", pid, item.Count)
		}
		label := strings.TrimSpace(item.Protocol + "/" + item.Port)
		processPadding := processWidth - runewidth.StringWidth(process) + 2
		if processPadding < 1 {
			processPadding = 1
		}
		value := process + strings.Repeat(" ", processPadding) + "pid:" + pid
		lines = append(lines, detailAlignedRow(m, label, value, labelWidth))
	}
	return lines
}

func groupedPortDetails(items []portDetail) map[string][]portDetail {
	groups := map[string][]portDetail{"system": {}, "docker": {}, "app": {}}
	for _, item := range items {
		group := portDetailGroup(item)
		groups[group] = append(groups[group], item)
	}
	return groups
}

func portDetailGroup(item portDetail) string {
	if strings.TrimSpace(item.Container) != "" || strings.TrimSpace(item.Process) == "docker-proxy" {
		return "docker"
	}
	port, _ := strconv.Atoi(item.Port)
	if port > 0 && port < 1024 {
		return "system"
	}
	return "app"
}

func portProcessText(item portDetail) string {
	container := strings.TrimSpace(item.Container)
	if container != "" && strings.TrimSpace(item.Process) == "docker-proxy" {
		return container
	}
	return strings.TrimSpace(item.Process)
}

func detailAlignedRow(m Model, label, value string, labelWidth int) string {
	padding := labelWidth - runewidth.StringWidth(label) + 2
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	continuationPrefix := strings.Repeat(" ", labelWidth+2)
	valueWidth := m.detailContentWidth() - labelWidth - 2
	if valueWidth < 12 {
		valueWidth = 12
	}
	parts := wrapDetailValue(value, valueWidth)
	if len(parts) == 0 {
		return prefix
	}
	lines := make([]string, 0, len(parts))
	lines = append(lines, prefix+detailValue(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, continuationPrefix+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func containerDetailRows(m Model, state hostState) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(state.ContainerError) != "" {
		return []string{m.detailRow("状态", redStyle.Render(state.ContainerError))}
	}
	if len(state.ContainerDetails) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	lines := []string{}
	groups := []struct {
		Title string
		Kind  string
		Style lipgloss.Style
	}{
		{"故障", "failed", detailDangerStyle},
		{"运行", "running", detailSubTitleStyle},
		{"停止", "stopped", detailSubTitleStyle},
	}
	firstGroup := true
	for _, group := range groups {
		items := filterContainersByKind(state.ContainerDetails, group.Kind)
		if len(items) == 0 {
			continue
		}
		if !firstGroup {
			lines = append(lines, "")
		}
		firstGroup = false
		lines = append(lines, group.Style.Render(fmt.Sprintf("· %s %d", group.Title, len(items))))
		nameWidth := containerNameWidth(items)
		for i, item := range items {
			lines = append(lines, containerDetailItemRows(m, item, nameWidth, i+1)...)
		}
	}
	return lines
}

func containerNameWidth(items []containerDetail) int {
	width := 10
	for _, item := range items {
		if w := runewidth.StringWidth(item.Name); w > width {
			width = w
		}
	}
	if width > 28 {
		width = 28
	}
	return width
}

func containerDetailItemRows(m Model, item containerDetail, nameWidth int, index int) []string {
	status := item.Status
	if status == "" {
		status = "-"
	}
	ports := item.Ports
	state := coloredContainerStatus(containerStatusSummary(status), containerDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	name := detailValueStyle.Render(padVisible(fit(item.Name, nameWidth), nameWidth))
	line := fitANSI(prefix+name+"  "+state, m.detailContentWidth())
	lines := []string{line}
	indent := strings.Repeat(" ", 4)
	if strings.TrimSpace(item.Status) != "" {
		lines = append(lines, containerIndentedLine(m, indent, "状态", item.Status))
	}
	if strings.TrimSpace(item.Image) != "" {
		lines = append(lines, containerIndentedLine(m, indent, "镜像", item.Image))
	}
	if simplified := simplifyDockerPorts(ports); simplified != "" {
		lines = append(lines, containerIndentedLine(m, indent, "端口", simplified))
	}
	return lines
}

func containerIndentedLine(m Model, indent string, label string, value string) string {
	prefixText := indent + label + " "
	prefix := detailLabelStyle.Render(prefixText)
	width := m.detailContentWidth() - ansi.StringWidth(prefixText)
	if width < 12 {
		width = 12
	}
	parts := wrapDetailValue(value, width)
	if len(parts) == 0 {
		return prefix
	}
	lines := []string{prefix + detailValue(parts[0])}
	continuation := strings.Repeat(" ", ansi.StringWidth(prefixText))
	for _, part := range parts[1:] {
		lines = append(lines, continuation+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func containerStatusSummary(status string) string {
	raw := strings.TrimSpace(status)
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "unhealthy"):
		return strings.TrimSpace("异常 " + dockerStatusAge(raw, "Up"))
	case strings.HasPrefix(lower, "up "):
		age := dockerStatusAge(raw, "Up")
		if strings.Contains(lower, "healthy") {
			return strings.TrimSpace("健康 " + age)
		}
		return strings.TrimSpace("运行 " + age)
	case strings.HasPrefix(lower, "restarting"):
		return strings.TrimSpace("重启中 " + dockerStatusAgo(raw))
	case strings.HasPrefix(lower, "exited"):
		return strings.TrimSpace("退出 " + dockerStatusAgo(raw))
	case strings.HasPrefix(lower, "created"):
		return strings.TrimSpace("已创建 " + dockerStatusAgo(raw))
	default:
		return raw
	}
}

func dockerStatusAge(status string, prefix string) string {
	status = strings.TrimSpace(status)
	status = strings.TrimPrefix(status, prefix)
	if idx := strings.Index(status, "("); idx >= 0 {
		status = status[:idx]
	}
	return shortDockerDuration(status)
}

func dockerStatusAgo(status string) string {
	status = strings.TrimSpace(status)
	if idx := strings.LastIndex(status, ")"); idx >= 0 && idx < len(status)-1 {
		status = status[idx+1:]
	}
	status = strings.TrimSuffix(strings.TrimSpace(status), "ago")
	return shortDockerDuration(status)
}

func shortDockerDuration(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Created ")
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return value
	}
	unit := fields[1]
	switch {
	case strings.HasPrefix(unit, "second"):
		unit = "秒"
	case strings.HasPrefix(unit, "minute"):
		unit = "分"
	case strings.HasPrefix(unit, "hour"):
		unit = "时"
	case strings.HasPrefix(unit, "day"):
		unit = "天"
	case strings.HasPrefix(unit, "week"):
		unit = "周"
	case strings.HasPrefix(unit, "month"):
		unit = "月"
	case strings.HasPrefix(unit, "year"):
		unit = "年"
	}
	return fields[0] + unit
}

func simplifyDockerPorts(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		text := simplifyDockerPort(strings.TrimSpace(part))
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return strings.Join(out, ", ")
}

func simplifyDockerPort(value string) string {
	if value == "" {
		return ""
	}
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return value
	}
	hostPort := portFromAddress(left)
	if hostPort == "" {
		return value
	}
	host := dockerPortHost(left)
	target := strings.TrimSpace(right)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return hostPort + "->" + target
	}
	return host + ":" + hostPort + "->" + target
}

func dockerPortHost(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") {
		if idx := strings.LastIndex(value, "]:"); idx >= 0 {
			return strings.Trim(value[:idx+1], "[]")
		}
	}
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		return value[:idx]
	}
	return ""
}

func coloredContainerStatus(status string, kind string) string {
	switch kind {
	case "failed":
		return redStyle.Render(status)
	default:
		return detailValueStyle.Render(status)
	}
}

func filterContainersByKind(items []containerDetail, kind string) []containerDetail {
	out := []containerDetail{}
	for _, item := range items {
		if containerDetailKind(item) == kind {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a := strings.ToLower(out[i].Name)
		b := strings.ToLower(out[j].Name)
		if a == b {
			return strings.ToLower(out[i].Image) < strings.ToLower(out[j].Image)
		}
		return a < b
	})
	return out
}

func containerDetailKind(item containerDetail) string {
	status := strings.ToLower(item.Status)
	switch {
	case strings.Contains(status, "restarting") || strings.Contains(status, "dead") || strings.Contains(status, "unhealthy"):
		return "failed"
	case strings.HasPrefix(status, "up "):
		return "running"
	default:
		return "stopped"
	}
}

func containerDetailCounts(items []containerDetail) (int, int, int) {
	running := 0
	stopped := 0
	failed := 0
	for _, item := range items {
		switch containerDetailKind(item) {
		case "running":
			running++
		case "failed":
			failed++
		default:
			stopped++
		}
	}
	return running, stopped, failed
}

func loginSummaryDetailRows(m Model, loading bool, summary []string, errText string, danger bool) []string {
	if loading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(errText) != "" {
		return []string{m.detailRow("状态", redStyle.Render(errText))}
	}
	if len(summary) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	lines := make([]string, 0, len(summary))
	for _, line := range summary {
		label, value, ok := strings.Cut(line, "\t")
		if !ok {
			label = "记录"
			value = line
		}
		if danger && label == "统计" {
			value = redStyle.Render(value)
		}
		lines = append(lines, m.detailRow(label, value))
	}
	return lines
}

func checkSuggestionRows(m Model, state hostState, checks []checkItem) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "检查中")}
	}
	rows := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Level == "正常" {
			continue
		}
		rows = append(rows, m.detailRow(check.Level, styleCheck(check.Level, check.Text)))
	}
	if len(rows) == 0 {
		rows = append(rows, m.detailRow("正常", "未发现明显风险"))
	}
	return rows
}

func riskSummaryText(checks []checkItem) string {
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 && counts["提示"] == 0 {
		return greenStyle.Render("正常")
	}
	parts := []string{}
	if counts["严重"] > 0 {
		parts = append(parts, redStyle.Render(fmt.Sprintf("严重%d", counts["严重"])))
	}
	if counts["警告"] > 0 {
		parts = append(parts, yellowStyle.Render(fmt.Sprintf("警告%d", counts["警告"])))
	}
	if counts["提示"] > 0 {
		parts = append(parts, detailValueStyle.Render(fmt.Sprintf("提示%d", counts["提示"])))
	}
	return strings.Join(parts, "  ")
}

func cardRiskText(checks []checkItem, width int) string {
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 {
		return ""
	}
	text := cardMutedStyle.Render("风险 ")
	if counts["严重"] > 0 {
		text += redStyle.Render(fmt.Sprintf("%d", counts["严重"]))
	}
	if counts["严重"] > 0 && counts["警告"] > 0 {
		text += cardMutedStyle.Render("/")
	}
	if counts["警告"] > 0 {
		text += yellowStyle.Render(fmt.Sprintf("%d", counts["警告"]))
	}
	return ansi.Truncate(text, width, "…")
}

type checkItem struct {
	Level string
	Text  string
}

func buildChecks(state hostState) []checkItem {
	metrics := state.Metrics
	var checks []checkItem
	add := func(level string, text string) {
		checks = append(checks, checkItem{Level: level, Text: text})
	}
	if strings.TrimSpace(state.Host.ExpireAt) != "" {
		if days, ok := expireDays(state.Host.ExpireAt); ok {
			switch {
			case days < 0:
				add("严重", fmt.Sprintf("服务器到期：风险，已过期%d天，建议确认续费或下线", -days))
			case days == 0:
				add("严重", "服务器到期：风险，今天到期，建议立即续费")
			case days <= 7:
				add("警告", fmt.Sprintf("服务器到期：警告，剩余%d天，建议提前续费", days))
			case days <= 30:
				add("提示", fmt.Sprintf("服务器到期：提示，剩余%d天", days))
			}
		} else {
			add("警告", "服务器到期：警告，到期时间格式错误，应为 YYYY-MM-DD")
		}
	}
	if !metrics.Online {
		add("严重", "服务器状态：风险，当前离线，监控数据不可用")
		return checks
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["passwordauthentication"])); value == "yes" {
		add("严重", "允许密码登录：风险，建议关闭 PasswordAuthentication")
	} else if value == "no" {
		add("正常", "SSH密码登录已关闭")
	} else if state.SSHDSecurityError != "" {
		add("提示", "SSH配置检查：提示，"+state.SSHDSecurityError)
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["permitrootlogin"])); value == "yes" {
		add("严重", "允许root登录：风险，建议设置 PermitRootLogin no")
	} else if value == "without-password" || value == "prohibit-password" {
		add("警告", "允许root登录：警告，未完全禁用，建议设置 PermitRootLogin no")
	} else if value == "no" {
		add("正常", "Root登录已关闭")
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["pubkeyauthentication"])); value == "no" {
		add("警告", "密钥登录：警告，SSH密钥登录已关闭，建议确认是否符合预期")
	}
	sshPort := strings.TrimSpace(state.Host.Port)
	if sshPort == "" {
		sshPort = "22"
	}
	add("提示", fmt.Sprintf("SSH端口：提示，当前端口%s，建议安全组只允许你的IP连接", sshPort))
	failedCount := loginSummaryCount(state.FailedLoginSummary)
	failedSourceCount := loginSummaryUniqueSourceCount(state.FailedLoginSummary)
	failedScan := loginSummaryValue(state.FailedLoginSummary, "疑似扫描")
	if failedCount >= 100 {
		add("严重", fmt.Sprintf("失败登录来源IP过多：风险，最近%d条失败登录，建议限制安全组或启用fail2ban", failedCount))
	} else if failedCount >= 20 {
		add("警告", fmt.Sprintf("失败登录来源IP过多：警告，最近%d条失败登录，建议关注来源IP", failedCount))
	} else if failedSourceCount >= 3 {
		add("警告", fmt.Sprintf("失败登录来源IP过多：警告，发现%d个来源IP，建议确认是否为扫描", failedSourceCount))
	}
	if failedScan != "" && failedScan != "-" {
		add("警告", "失败登录来源IP过多：警告，"+failedScan)
	}
	if metrics.DiskPercent() >= 90 {
		add("严重", fmt.Sprintf("磁盘容量：风险，使用率%.0f%%，建议尽快清理", metrics.DiskPercent()))
	} else if metrics.DiskPercent() >= 80 {
		add("警告", fmt.Sprintf("磁盘容量：警告，使用率%.0f%%，建议关注容量", metrics.DiskPercent()))
	}
	if metrics.MemPercent() >= 90 {
		add("警告", fmt.Sprintf("内存使用：警告，使用率%.0f%%，建议排查进程", metrics.MemPercent()))
	}
	if metrics.CPUPercent >= 90 {
		add("警告", fmt.Sprintf("CPU使用：警告，使用率%.0f%%，建议排查负载", metrics.CPUPercent))
	}
	_, detailStopped, detailFailed := containerDetailCounts(state.ContainerDetails)
	dockerFailed := metrics.DockerFailed
	if dockerFailed == 0 {
		dockerFailed = detailFailed
	}
	if dockerFailed > 0 {
		add("警告", fmt.Sprintf("容器状态：警告，存在%d个故障容器，建议查看容器详情", dockerFailed))
	}
	if metrics.DockerTotal == 0 && len(state.ContainerDetails) > 0 && detailStopped > 0 {
		add("提示", fmt.Sprintf("容器状态：提示，存在%d个停止容器", detailStopped))
	}
	if strings.TrimSpace(state.ContainerError) != "" {
		add("提示", "容器详情：提示，"+state.ContainerError)
	}
	if strings.TrimSpace(state.PortDetailsError) != "" {
		add("提示", "端口详情：提示，"+state.PortDetailsError)
	}
	if metrics.FailedServices > 0 {
		add("警告", fmt.Sprintf("系统服务：警告，存在%d个异常服务", metrics.FailedServices))
	}
	if metrics.HealthTotal() > 0 && metrics.HealthOK() < metrics.HealthTotal() {
		add("警告", fmt.Sprintf("健康端口：警告，%d/%d正常", metrics.HealthOK(), metrics.HealthTotal()))
	}
	if len(state.PortDetails) > 0 {
		publicDockerPorts := publicDockerProxyPorts(state.PortDetails)
		if publicDockerPorts > 0 {
			add("提示", fmt.Sprintf("公网端口：提示，发现%d个Docker映射端口，建议只开放必要端口", publicDockerPorts))
		}
	}
	return checks
}

func publicDockerProxyPorts(ports []portDetail) int {
	count := 0
	for _, port := range ports {
		if strings.TrimSpace(port.Container) != "" || strings.TrimSpace(port.Process) == "docker-proxy" {
			count++
		}
	}
	return count
}

func styleCheck(level string, text string) string {
	switch level {
	case "严重":
		return redStyle.Render(text)
	case "警告":
		return yellowStyle.Render(text)
	case "正常":
		return greenStyle.Render(text)
	default:
		return detailValueStyle.Render(text)
	}
}

func loginSummaryCount(summary []string) int {
	for _, row := range summary {
		label, value, ok := strings.Cut(row, "\t")
		if !ok || label != "统计" {
			continue
		}
		re := regexp.MustCompile(`\d+`)
		match := re.FindString(value)
		if match == "" {
			return 0
		}
		n, _ := strconv.Atoi(match)
		return n
	}
	return 0
}

func loginSummaryUniqueSourceCount(summary []string) int {
	for _, row := range summary {
		label, value, ok := strings.Cut(row, "\t")
		if !ok || label != "来源IP" {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			return 0
		}
		count := 0
		for _, part := range strings.Split(value, "、") {
			if strings.TrimSpace(part) != "" {
				count++
			}
		}
		return count
	}
	return 0
}

func loginSummaryValue(summary []string, label string) string {
	for _, row := range summary {
		got, value, ok := strings.Cut(row, "\t")
		if ok && got == label {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func serviceCardText(metrics monitor.Metrics) string {
	total := dockerTotal(metrics)
	containerText := cardMutedStyle.Render(fmt.Sprintf("容器 %d/%d/%d", metrics.DockerFailed, metrics.DockerRunning, total))
	if total == 0 {
		containerText = cardMutedStyle.Render("容器 0")
	}
	if metrics.DockerFailed > 0 {
		containerText = cardMutedStyle.Render("容器 ") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	serviceText := cardMutedStyle.Render("服务 ") + serviceNumber
	if metrics.HealthTotal() > 0 {
		healthNumber := cardMutedStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		switch {
		case metrics.HealthOK() == metrics.HealthTotal():
			healthNumber = greenStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		case metrics.HealthOK() == 0:
			healthNumber = redStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		default:
			healthNumber = yellowStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		}
		healthText := cardMutedStyle.Render("健康 ") + healthNumber
		return fmt.Sprintf("%s  %s  %s", healthText, containerText, serviceText)
	}
	return fmt.Sprintf("%s  %s", containerText, serviceText)
}

func healthPortsText(metrics monitor.Metrics) string {
	if metrics.HealthTotal() == 0 {
		return "-"
	}
	parts := make([]string, 0, len(metrics.HealthPorts))
	for _, port := range metrics.HealthPorts {
		status := "失败"
		if port.Healthy {
			status = "正常"
		}
		parts = append(parts, fmt.Sprintf("%d%s", port.Port, status))
	}
	return strings.Join(parts, "  ")
}

func padToBottom(lines []string, height int, reservedBottomLines int) []string {
	if height <= 0 {
		return lines
	}
	used := 0
	for _, line := range lines {
		used += strings.Count(line, "\n") + 1
	}
	target := height - reservedBottomLines
	for used < target {
		lines = append(lines, "")
		used++
	}
	return lines
}
