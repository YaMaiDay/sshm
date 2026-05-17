package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func containerMemoryText(item resourceservice.ContainerDetail) string {
	memory := strings.TrimSpace(item.Memory)
	percent := strings.TrimSpace(item.MemPerc)
	switch {
	case memory != "" && percent != "":
		return memory + " " + percent
	case memory != "":
		return memory
	case percent != "":
		return percent
	default:
		return "-"
	}
}

func containerResourceText(item resourceservice.ContainerDetail) string {
	return "CPU " + emptyDash(item.CPU) + "  MEM " + containerMemoryText(item)
}

func (m Model) containerStatusLine(item resourceservice.ContainerDetail) string {
	kind := containerDetailKind(item)
	label := m.containerStatusLabel(item)
	raw := emptyDash(item.Status)
	return coloredContainerStatus(label, kind) + "  " + coloredContainerStatus(raw, kind)
}

func (m Model) containerStatusLabel(item resourceservice.ContainerDetail) string {
	switch containerDetailKind(item) {
	case "missing":
		return m.t("Not found", "未发现")
	case "failed":
		return m.t("Problem", "异常")
	case "running":
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if strings.Contains(status, "healthy") && !strings.Contains(status, "unhealthy") {
			return m.t("Healthy", "健康")
		}
		return m.t("Running", "运行")
	case "stopped":
		return m.t("Stopped", "停止")
	default:
		return m.t("Unknown", "未知")
	}
}

func (m Model) databaseStatusLabel(item resourceservice.DatabaseDetail) string {
	if m.databaseConnectionAvailable(item) {
		return m.t("Running", "运行")
	}
	switch item.Status {
	case "missing":
		return m.t("Not found", "未发现")
	case "problem":
		return m.t("Problem", "异常")
	case "running":
		return m.t("Running", "运行")
	case "stopped":
		return m.t("Stopped", "停止")
	default:
		return m.t("Unknown", "未知")
	}
}

func (m Model) databaseStatusStyled(item resourceservice.DatabaseDetail) string {
	if m.databaseConnectionAvailable(item) {
		return greenStyle.Render(m.databaseStatusLabel(item))
	}
	switch item.Status {
	case "running":
		return greenStyle.Render(m.databaseStatusLabel(item))
	case "problem", "missing":
		return redStyle.Render(m.databaseStatusLabel(item))
	case "stopped":
		return mutedStyle.Render(m.databaseStatusLabel(item))
	default:
		return yellowStyle.Render(m.databaseStatusLabel(item))
	}
}

func (m Model) databaseStatusLine(item resourceservice.DatabaseDetail) string {
	raw := strings.TrimSpace(item.RawStatus)
	if m.databaseConnectionAvailable(item) {
		if raw == "" || strings.EqualFold(raw, m.t("Configured", "已配置")) {
			raw = m.t("Connected", "已连接")
		}
		return greenStyle.Render(m.databaseStatusLabel(item)) + "  " + greenStyle.Render(raw)
	}
	if raw == "" {
		raw = m.databaseStatusLabel(item)
	}
	switch item.Status {
	case "running":
		return greenStyle.Render(m.databaseStatusLabel(item)) + "  " + greenStyle.Render(raw)
	case "problem", "missing":
		return redStyle.Render(m.databaseStatusLabel(item)) + "  " + redStyle.Render(raw)
	case "stopped":
		return mutedStyle.Render(m.databaseStatusLabel(item)) + "  " + mutedStyle.Render(raw)
	default:
		return yellowStyle.Render(m.databaseStatusLabel(item)) + "  " + yellowStyle.Render(raw)
	}
}

func (m Model) databaseStatusDot(item resourceservice.DatabaseDetail) string {
	if m.databaseConnectionAvailable(item) {
		return greenStyle.Render("●")
	}
	switch item.Status {
	case "running":
		return greenStyle.Render("●")
	case "problem", "missing":
		return redStyle.Render("●")
	case "stopped":
		return mutedStyle.Render("●")
	default:
		return yellowStyle.Render("●")
	}
}

func (m Model) databaseConnectionAvailable(item resourceservice.DatabaseDetail) bool {
	if item.Status == "running" {
		return false
	}
	d, errText, ok := m.databaseCardExtra(item)
	if !ok || strings.TrimSpace(errText) != "" {
		return false
	}
	return strings.TrimSpace(d.Version) != ""
}

func (m Model) databaseFoundText(item resourceservice.DatabaseDetail, instance string) string {
	if item.Missing {
		return yesNoText(m.isChineseUI(), false)
	}
	if strings.TrimSpace(instance) != "" ||
		strings.TrimSpace(item.Container) != "" ||
		strings.TrimSpace(item.ServiceUnit) != "" ||
		strings.TrimSpace(item.Process) != "" {
		return yesNoText(m.isChineseUI(), true)
	}
	if item.Configured {
		return m.t("External", "外部")
	}
	return yesNoText(m.isChineseUI(), true)
}

func (m Model) databaseCardOwnerLine(item resourceservice.DatabaseDetail) string {
	if strings.TrimSpace(item.Container) != "" {
		return m.t("Container", "容器") + "  " + cardMutedStyle.Render(item.Container)
	}
	if strings.TrimSpace(item.Process) != "" {
		return m.t("Process", "进程") + "  " + cardMutedStyle.Render(item.Process)
	}
	if strings.TrimSpace(item.ServiceUnit) != "" {
		return m.t("Service", "服务") + "  " + cardMutedStyle.Render(item.ServiceUnit)
	}
	return m.t("Source", "来源") + "  " + cardMutedStyle.Render(emptyDash(item.Source))
}

func (m Model) databaseCardStorageLine(item resourceservice.DatabaseDetail, width int) string {
	label := m.t("Storage", "存储")
	d, errText, ok := m.databaseCardExtra(item)
	if !ok {
		return label + "  " + cardMutedStyle.Render(m.t("Loading", "读取中"))
	}
	if errText != "" {
		return label + "  " + m.databaseCardErrorText(errText)
	}
	if resourceservice.NormalizeDatabaseEngine(item.Engine) == "Redis" {
		return label + "  " + detailValueStyle.Render(emptyDash(firstNonEmpty(d.MemoryUsed, bytesHuman(d.SizeBytes))))
	}
	if d.SizeBytes > 0 && d.TotalBytes > 0 {
		percent := float64(d.SizeBytes) * 100 / float64(d.TotalBytes)
		thresholds := m.metricThresholds()
		barWidth := 5
		if width >= 42 {
			barWidth = 6
		}
		return label + "  " + percentBarWidthWithThreshold(percent, barWidth, thresholds.DiskWarn, thresholds.DiskCrit) + "  " + bytesHuman(d.SizeBytes) + "/" + bytesHuman(d.TotalBytes)
	}
	if d.SizeBytes > 0 {
		return label + "  " + detailValueStyle.Render(bytesHuman(d.SizeBytes))
	}
	return label + "  " + cardMutedStyle.Render("-")
}

func (m Model) databaseCardCountLine(item resourceservice.DatabaseDetail) string {
	d, errText, ok := m.databaseCardExtra(item)
	if !ok || errText != "" {
		return m.t("Structure", "结构") + "  " + cardMutedStyle.Render("-")
	}
	if resourceservice.NormalizeDatabaseEngine(item.Engine) == "Redis" {
		return m.t("Structure", "结构") + "  " + cardMutedStyle.Render("Key ") + detailValueStyle.Render(emptyDash(d.Keyspace))
	}
	parts := []string{}
	if d.DatabaseCount != "" {
		parts = append(parts, cardMutedStyle.Render(m.t("DB ", "库 "))+detailValueStyle.Render(d.DatabaseCount))
	}
	if d.TableCount != "" {
		parts = append(parts, cardMutedStyle.Render(m.t("Tables ", "表 "))+detailValueStyle.Render(d.TableCount))
	}
	if d.IndexTotalBytes > 0 {
		parts = append(parts, cardMutedStyle.Render(m.t("Index ", "索引 "))+detailValueStyle.Render(bytesHuman(d.IndexTotalBytes)))
	}
	if len(parts) == 0 {
		parts = append(parts, cardMutedStyle.Render("-"))
	}
	return m.t("Structure", "结构") + "  " + strings.Join(parts, cardMutedStyle.Render("  "))
}

func (m Model) databaseCardMeta(item resourceservice.DatabaseDetail) string {
	if d, errText, ok := m.databaseCardExtra(item); ok && errText == "" {
		if seconds := parseUnsignedText(d.Raw["Uptime"]); seconds > 0 {
			return m.dashboardDurationShort(time.Duration(seconds) * time.Second)
		}
	}
	raw := strings.TrimSpace(item.RawStatus)
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "up "):
		return m.dockerStatusDashboardMeta(raw, "Up")
	case strings.HasPrefix(lower, "created"):
		return m.dockerStatusDashboardMeta(raw, "Created")
	default:
		return ""
	}
}

func parseUnsignedText(value string) uint64 {
	n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func (m Model) databaseCardExtra(item resourceservice.DatabaseDetail) (resourceservice.DatabaseExtraDetail, string, bool) {
	if cache, ok := m.databaseExtraCache(item.Name); ok {
		if cache.Loading {
			return resourceservice.DatabaseExtraDetail{}, "", false
		}
		return cache.Detail, cache.Err, true
	}
	if m.resourceState.DatabaseExtraName != item.Name || m.resourceState.DatabaseExtraLoading {
		return resourceservice.DatabaseExtraDetail{}, "", false
	}
	return m.resourceState.DatabaseExtra, strings.TrimSpace(m.resourceState.DatabaseExtraErr), true
}

func (m Model) databaseCardErrorText(errText string) string {
	lower := strings.ToLower(errText)
	switch {
	case strings.Contains(errText, "未配置") || strings.Contains(lower, "credential") || strings.Contains(lower, "authentication"):
		return yellowStyle.Render(m.t("Need credentials", "未配置账号"))
	case strings.Contains(errText, "客户端不可用") || strings.Contains(lower, "client"):
		return yellowStyle.Render(m.t("Client unavailable", "客户端不可用"))
	default:
		return redStyle.Render(m.t("Failed", "获取失败"))
	}
}

func containerDetailPercentText(percentText string, extra string, warn float64, crit float64) string {
	value, ok := parsePercentText(percentText)
	if !ok {
		return emptyDash(extra)
	}
	text := percentBarWithThreshold(value, warn, crit)
	if strings.TrimSpace(extra) != "" {
		text += "  " + cardMutedStyle.Render(extra)
	}
	return text
}

func (m Model) containerCPULimitText() string {
	if m.resourceState.ContainerExtraLoading {
		return m.t("Loading", "加载中")
	}
	if strings.TrimSpace(m.resourceState.ContainerExtraErr) != "" {
		return "-"
	}
	d := m.resourceState.ContainerExtra
	return m.containerCPULimitTextFromFields(d.NanoCpus, d.CPUQuota, d.CPUPeriod, d.CpusetCpus)
}

func (m Model) containerCPULimitTextForItem(item resourceservice.ContainerDetail) string {
	if item.CPULimitKnown {
		return m.containerCPULimitTextFromFields(item.NanoCpus, item.CPUQuota, item.CPUPeriod, item.CpusetCpus)
	}
	if m.resourceState.ContainerExtraName == item.Name && !m.resourceState.ContainerExtraLoading && strings.TrimSpace(m.resourceState.ContainerExtraErr) == "" {
		return m.containerCPULimitText()
	}
	return ""
}

func (m Model) containerCPULimitTextFromFields(nanoCpus int64, cpuQuota int64, cpuPeriod int64, cpusetCpus string) string {
	if strings.TrimSpace(cpusetCpus) != "" {
		return "CPU " + cpusetCpus
	}
	if nanoCpus > 0 {
		return m.cpuLimitCoresText(float64(nanoCpus) / 1_000_000_000)
	}
	if cpuQuota > 0 && cpuPeriod > 0 {
		return m.cpuLimitCoresText(float64(cpuQuota) / float64(cpuPeriod))
	}
	return m.t("Unlimited", "未限制")
}

func (m Model) cpuLimitCoresText(cores float64) string {
	if cores <= 0 {
		return m.t("Unlimited", "未限制")
	}
	text := fmt.Sprintf("%.2f", cores)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if m.isChineseUI() {
		return text + "核限制"
	}
	if text == "1" {
		return "1 core limit"
	}
	return text + " cores limit"
}

func (m Model) containerCardMeta(item resourceservice.ContainerDetail) string {
	raw := strings.TrimSpace(item.Status)
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "unhealthy") || strings.HasPrefix(lower, "up "):
		return m.dockerStatusDashboardMeta(raw, "Up")
	case strings.HasPrefix(lower, "restarting"), strings.HasPrefix(lower, "exited"):
		return m.dockerStatusDashboardMeta(raw, "")
	case strings.HasPrefix(lower, "created"):
		return m.dockerStatusDashboardMeta(raw, "Created")
	default:
		return ""
	}
}

func (m Model) dockerStatusDashboardMeta(status string, prefix string) string {
	d, ok := dockerStatusDuration(status, prefix)
	if !ok {
		return ""
	}
	return m.dashboardDurationShort(d)
}

func dockerStatusDuration(status string, prefix string) (time.Duration, bool) {
	status = strings.TrimSpace(status)
	status = strings.TrimPrefix(status, prefix)
	if idx := strings.Index(status, "("); idx >= 0 {
		status = status[:idx]
	}
	if idx := strings.LastIndex(status, ")"); idx >= 0 && idx < len(status)-1 {
		status = status[idx+1:]
	}
	status = strings.TrimPrefix(strings.TrimSpace(status), "Created ")
	status = strings.TrimSuffix(strings.TrimSpace(status), "ago")
	fields := strings.Fields(strings.TrimSpace(status))
	if len(fields) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil || n < 0 {
		return 0, false
	}
	unit := strings.ToLower(fields[1])
	switch {
	case strings.HasPrefix(unit, "second"):
		return time.Duration(n) * time.Second, true
	case strings.HasPrefix(unit, "minute"):
		return time.Duration(n) * time.Minute, true
	case strings.HasPrefix(unit, "hour"):
		return time.Duration(n) * time.Hour, true
	case strings.HasPrefix(unit, "day"):
		return time.Duration(n) * 24 * time.Hour, true
	case strings.HasPrefix(unit, "week"):
		return time.Duration(n) * 7 * 24 * time.Hour, true
	case strings.HasPrefix(unit, "month"):
		return time.Duration(n) * 30 * 24 * time.Hour, true
	case strings.HasPrefix(unit, "year"):
		return time.Duration(n) * 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func (m Model) dashboardDurationShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	if minutes < 60 {
		if m.isChineseUI() {
			return fmt.Sprintf("%d分", minutes)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		if m.isChineseUI() {
			return fmt.Sprintf("%d时", hours)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if m.isChineseUI() {
		return fmt.Sprintf("%d天", days)
	}
	return fmt.Sprintf("%dd", days)
}

func resourcePercentMetricLine(label string, percentText string, extra string, width int, warn float64, crit float64) string {
	value, ok := parsePercentText(percentText)
	if !ok {
		return cardMetricLine(label, cardMutedStyle.Render(" -"), emptyDash(extra), width)
	}
	barWidth := 8
	if width >= 42 {
		barWidth = 10
	}
	return cardMetricLine(label, percentBarWidthWithThreshold(value, barWidth, warn, crit), emptyDash(extra), width)
}

func parsePercentText(value string) (float64, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func servicePIDText(item resourceservice.ServiceDetail) string {
	if strings.TrimSpace(item.MainPID) != "" {
		return item.MainPID
	}
	return item.ExecMainPID
}

func serviceMemoryText(item resourceservice.ServiceDetail) string {
	return bytesHuman(item.MemoryCurrent)
}

func serviceResourceText(item resourceservice.ServiceDetail) string {
	return "PID " + emptyDash(servicePIDText(item)) + "  MEM " + serviceMemoryText(item)
}

func serviceCardResourceLine(m Model, item resourceservice.ServiceDetail) string {
	parts := []string{}
	if pid := strings.TrimSpace(servicePIDText(item)); pid != "" {
		parts = append(parts, "PID  "+cardMutedStyle.Render(pid))
	}
	if item.MemoryCurrent > 0 {
		parts = append(parts, m.t("Memory", "内存")+"  "+cardMutedStyle.Render(serviceMemoryText(item)))
	}
	if len(parts) == 0 {
		return m.t("Resource", "资源") + "  " + cardMutedStyle.Render("-")
	}
	return strings.Join(parts, "  ")
}

func (m Model) serviceCardMeta(item resourceservice.ServiceDetail) string {
	if serviceDetailKind(item) != "running" && serviceDetailKind(item) != "active" {
		return ""
	}
	for _, value := range []string{item.ActiveSince, item.ExecStartedAt, item.StateChangedAt} {
		t, ok := parseSystemdTimestamp(value)
		if ok {
			d := time.Since(t)
			if d < 0 {
				return ""
			}
			return m.dashboardDurationShort(d)
		}
	}
	return ""
}

func (m Model) serviceStartText(item resourceservice.ServiceDetail) string {
	for _, value := range []string{item.ActiveSince, item.ExecStartedAt, item.StateChangedAt} {
		text := shortSystemdTimestampAge(value, m.isChineseUI())
		if strings.TrimSpace(text) != "" {
			return text
		}
	}
	return "-"
}

func (m Model) serviceUnitFileStateText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !m.isChineseUI() {
		return value
	}
	switch strings.ToLower(value) {
	case "enabled":
		return "已启用"
	case "disabled":
		return "未启用"
	case "static":
		return "依赖启动"
	case "masked":
		return "已屏蔽"
	case "generated":
		return "生成"
	case "transient":
		return "临时"
	case "indirect":
		return "间接"
	case "enabled-runtime":
		return "运行时启用"
	case "linked", "linked-runtime":
		return "已链接"
	default:
		return value
	}
}

func (m Model) serviceSourceText(item resourceservice.ServiceDetail) string {
	if strings.TrimSpace(item.WorkingDirectory) != "" {
		return m.t("Dir ", "目录 ") + item.WorkingDirectory
	}
	if strings.TrimSpace(item.FragmentPath) != "" {
		return m.t("Source ", "来源 ") + item.FragmentPath
	}
	return "-"
}

func serviceProgramPath(item resourceservice.ServiceDetail) string {
	execStart := strings.TrimSpace(item.ExecStart)
	if execStart == "" {
		return ""
	}
	normalized := strings.NewReplacer(";", " ", "{", " ", "}", " ").Replace(execStart)
	for _, token := range strings.Fields(normalized) {
		token = strings.Trim(token, "\"'(),")
		if strings.HasPrefix(token, "path=") {
			token = strings.TrimPrefix(token, "path=")
		}
		if strings.HasPrefix(token, "argv[]=") {
			token = strings.TrimPrefix(token, "argv[]=")
		}
		if strings.HasPrefix(token, "/") {
			return token
		}
	}
	return ""
}
