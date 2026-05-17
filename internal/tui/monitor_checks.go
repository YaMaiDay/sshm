package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func checkSuggestionRows(m Model, state hostState, checks []checkItem) []string {
	if state.LoginLoading {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("Checking", "检查中"))}
	}
	rows := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Level == "正常" {
			continue
		}
		rows = append(rows, m.detailRow(m.checkLevelText(check.Level), m.styleCheck(check.Level, m.checkText(check.Text))))
	}
	if len(rows) == 0 {
		rows = append(rows, m.detailRow(m.t("OK", "正常"), m.t("No obvious risks found", "未发现明显风险")))
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

func (m Model) riskSummaryText(checks []checkItem) string {
	if m.isChineseUI() {
		return riskSummaryText(checks)
	}
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 && counts["提示"] == 0 {
		return greenStyle.Render("OK")
	}
	parts := []string{}
	if counts["严重"] > 0 {
		parts = append(parts, redStyle.Render(fmt.Sprintf("Critical%d", counts["严重"])))
	}
	if counts["警告"] > 0 {
		parts = append(parts, yellowStyle.Render(fmt.Sprintf("Warn%d", counts["警告"])))
	}
	if counts["提示"] > 0 {
		parts = append(parts, detailValueStyle.Render(fmt.Sprintf("Info%d", counts["提示"])))
	}
	return strings.Join(parts, "  ")
}

func (m Model) cardRiskText(checks []checkItem, width int) string {
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 {
		return ""
	}
	text := cardMutedStyle.Render(m.t("Risk ", "风险 "))
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

type metricThresholds struct {
	CPUWarn  float64
	CPUCrit  float64
	MemWarn  float64
	MemCrit  float64
	DiskWarn float64
	DiskCrit float64
}

func (m Model) metricThresholds() metricThresholds {
	cfg := m.appConfig
	return metricThresholds{
		CPUWarn:  cfg.Thresholds.CPUWarn,
		CPUCrit:  cfg.Thresholds.CPUCrit,
		MemWarn:  cfg.Thresholds.MemWarn,
		MemCrit:  cfg.Thresholds.MemCrit,
		DiskWarn: cfg.Thresholds.DiskWarn,
		DiskCrit: cfg.Thresholds.DiskCrit,
	}
}

func (m Model) buildChecks(state hostState) []checkItem {
	return buildChecksWithThresholds(state, m.metricThresholds())
}

func buildChecksWithThresholds(state hostState, thresholds metricThresholds) []checkItem {
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
	if metrics.DiskPercent() >= thresholds.DiskCrit {
		add("严重", fmt.Sprintf("磁盘容量：风险，使用率%.0f%%，建议尽快清理", metrics.DiskPercent()))
	} else if metrics.DiskPercent() >= thresholds.DiskWarn {
		add("警告", fmt.Sprintf("磁盘容量：警告，使用率%.0f%%，建议关注容量", metrics.DiskPercent()))
	}
	if metrics.MemPercent() >= thresholds.MemCrit {
		add("严重", fmt.Sprintf("内存使用：风险，使用率%.0f%%，建议尽快排查进程", metrics.MemPercent()))
	} else if metrics.MemPercent() >= thresholds.MemWarn {
		add("警告", fmt.Sprintf("内存使用：警告，使用率%.0f%%，建议排查进程", metrics.MemPercent()))
	}
	if metrics.CPUPercent >= thresholds.CPUCrit {
		add("严重", fmt.Sprintf("CPU使用：风险，使用率%.0f%%，建议尽快排查负载", metrics.CPUPercent))
	} else if metrics.CPUPercent >= thresholds.CPUWarn {
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
	if len(state.PortDetails) > 0 {
		publicDockerPorts := publicDockerProxyPorts(state.PortDetails)
		if publicDockerPorts > 0 {
			add("提示", fmt.Sprintf("公网端口：提示，发现%d个Docker映射端口，建议只开放必要端口", publicDockerPorts))
		}
	}
	return checks
}

func publicDockerProxyPorts(ports []resourceservice.PortDetail) int {
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

func (m Model) styleCheck(level string, text string) string {
	return styleCheck(level, text)
}

func (m Model) checkLevelText(level string) string {
	if m.isChineseUI() {
		return level
	}
	switch level {
	case "严重":
		return "Critical"
	case "警告":
		return "Warn"
	case "提示":
		return "Info"
	case "正常":
		return "OK"
	default:
		return level
	}
}

func (m Model) checkText(text string) string {
	if m.isChineseUI() {
		return text
	}
	if strings.Contains(text, "允许密码登录：") {
		return "Password login: risk, disable PasswordAuthentication"
	}
	if strings.Contains(text, "允许root登录：风险") {
		return "Root login: risk, set PermitRootLogin no"
	}
	if strings.Contains(text, "允许root登录：警告") {
		return "Root login: warning, not fully disabled; set PermitRootLogin no"
	}
	if strings.Contains(text, "密钥登录：") {
		return "Key login: warning, SSH key login is disabled"
	}
	if strings.Contains(text, "SSH配置检查：") {
		return "SSH config: " + afterLastColon(text)
	}
	if strings.Contains(text, "SSH端口：") {
		port := extractFirstNumber(text)
		if port == "" {
			port = "-"
		}
		return "SSH port: current port " + port + ", restrict security group to your IP"
	}
	if strings.Contains(text, "失败登录来源IP过多：") {
		num := extractFirstNumber(text)
		if strings.Contains(text, "fail2ban") {
			return "Failed logins: risk, " + num + " recent failed logins; restrict security group or enable fail2ban"
		}
		if strings.Contains(text, "来源IP") {
			return "Failed logins: warning, " + num + " source IPs found"
		}
		return "Failed logins: warning, " + num + " recent failed logins"
	}
	if strings.Contains(text, "磁盘容量：") {
		return "Disk usage: " + englishRiskSeverity(text) + ", usage " + extractPercent(text)
	}
	if strings.Contains(text, "内存使用：") {
		return "Memory usage: warning, usage " + extractPercent(text)
	}
	if strings.Contains(text, "CPU使用：") {
		return "CPU usage: warning, usage " + extractPercent(text)
	}
	if strings.Contains(text, "容器状态：") {
		num := extractFirstNumber(text)
		if strings.Contains(text, "故障容器") {
			return "Containers: warning, " + num + " failed containers"
		}
		return "Containers: info, " + num + " stopped containers"
	}
	if strings.Contains(text, "容器详情：") {
		return "Container details: " + afterLastColon(text)
	}
	if strings.Contains(text, "端口详情：") {
		return "Port details: " + afterLastColon(text)
	}
	if strings.Contains(text, "系统服务：") {
		return "System services: warning, " + extractFirstNumber(text) + " failed services"
	}
	if strings.Contains(text, "公网端口：") {
		return "Public ports: info, " + extractFirstNumber(text) + " Docker mapped ports found"
	}
	if strings.Contains(text, "服务器到期：") {
		return "Server expiry: " + englishRiskSeverity(text) + ", " + strings.ReplaceAll(afterLastColon(text), "天", "d")
	}
	if strings.Contains(text, "服务器状态：") {
		return "Server status: offline, monitoring data unavailable"
	}
	return text
}

func afterLastColon(value string) string {
	if idx := strings.LastIndex(value, "："); idx >= 0 && idx < len(value)-len("：") {
		return value[idx+len("："):]
	}
	return value
}

func extractFirstNumber(value string) string {
	return regexp.MustCompile(`\d+`).FindString(value)
}

func extractPercent(value string) string {
	if got := regexp.MustCompile(`\d+%`).FindString(value); got != "" {
		return got
	}
	return "-"
}

func extractHealthRatio(value string) string {
	if got := regexp.MustCompile(`\d+/\d+`).FindString(value); got != "" {
		return got
	}
	return "-"
}

func englishRiskSeverity(value string) string {
	switch {
	case strings.Contains(value, "风险"):
		return "risk"
	case strings.Contains(value, "警告"):
		return "warning"
	case strings.Contains(value, "提示"):
		return "info"
	default:
		return "info"
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
