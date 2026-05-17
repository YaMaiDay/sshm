package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) serviceLoadNote(item resourceservice.ServiceDetail) string {
	switch strings.ToLower(strings.TrimSpace(item.Load)) {
	case "loaded":
		return m.t("Unit file was found and loaded by systemd.", "systemd 已找到并加载这个服务配置文件。")
	case "not-found":
		return m.t("The unit file cannot be found.", "找不到这个服务对应的 unit 配置文件。")
	case "masked":
		return m.t("The service is masked and cannot be started normally.", "这个服务已被屏蔽，通常不能正常启动。")
	case "bad-setting":
		return m.t("The unit file has invalid settings.", "这个服务配置文件里有无效配置。")
	case "error":
		return m.t("systemd failed to load this unit.", "systemd 加载这个服务失败。")
	case "":
		return m.t("Load state is unavailable.", "没有获取到加载状态。")
	default:
		return m.t("This is systemd LoadState.", "这是 systemd 的 LoadState，表示配置文件加载状态。")
	}
}

func (m Model) serviceStateNote(item resourceservice.ServiceDetail) string {
	active := strings.ToLower(strings.TrimSpace(item.Active))
	sub := strings.ToLower(strings.TrimSpace(item.Sub))
	switch {
	case active == "active" && sub == "running":
		return m.t("The service is currently running.", "服务当前正在运行。")
	case active == "active" && sub == "exited":
		return m.t("The service completed its start task and exited successfully.", "服务启动任务已完成并退出，常见于一次性服务。")
	case active == "inactive" && sub == "dead":
		return m.t("The service is stopped.", "服务当前已停止。")
	case active == "failed":
		return m.t("The service failed. Check logs for the reason.", "服务运行失败，需要查看日志确认原因。")
	case active == "activating":
		return m.t("The service is starting.", "服务正在启动中。")
	case active == "deactivating":
		return m.t("The service is stopping.", "服务正在停止中。")
	case active == "":
		return m.t("Runtime state is unavailable.", "没有获取到运行状态。")
	default:
		return m.t("This is systemd ActiveState/SubState.", "这是 systemd 的 ActiveState/SubState，表示当前运行状态。")
	}
}

func (m Model) appendOptionalDetailRow(lines *[]string, label, value, styledValue string) {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return
	}
	if strings.TrimSpace(styledValue) != "" {
		*lines = append(*lines, m.detailRow(label, styledValue))
		return
	}
	*lines = append(*lines, m.detailRow(label, value))
}

func appendDetailSection(lines []string, title string, rows []string) []string {
	rows = compactDetailRows(rows)
	if len(rows) == 0 {
		return lines
	}
	lines = append(lines, "", sectionTitle(title))
	return append(lines, rows...)
}

func compactDetailRows(rows []string) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row) == "" {
			continue
		}
		out = append(out, row)
	}
	return out
}

func isEmptyDetailValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return true
	}
	switch strings.ToLower(value) {
	case "n/a", "<nil>", "[]", "{}":
		return true
	default:
		return false
	}
}

func serviceResultStyle(value string) string {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return ""
	}
	if strings.EqualFold(value, "success") {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func serviceExitStyle(value string) string {
	value = strings.TrimSpace(value)
	if isEmptyDetailValue(value) {
		return ""
	}
	if value == "0" {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func (m Model) mergedServiceDetail(item resourceservice.ServiceDetail) resourceservice.ServiceDetail {
	if m.resourceServiceExtraLoading || strings.TrimSpace(m.resourceServiceExtraErr) != "" || m.resourceServiceExtraName != item.Unit {
		return item
	}
	extra := m.resourceServiceExtra
	if strings.TrimSpace(extra.Unit) == "" {
		return item
	}
	extra.Managed = item.Managed
	extra.Favorite = item.Favorite
	extra.Missing = item.Missing
	return extra
}

func (m Model) serviceDetailLoadRow() string {
	if m.resourceServiceExtraLoading {
		return m.detailRow(m.t("Details", "详情"), m.t("Loading", "加载中"))
	}
	if strings.TrimSpace(m.resourceServiceExtraErr) != "" {
		return m.detailRow(m.t("Details", "详情"), redStyle.Render(m.resourceServiceExtraErr))
	}
	return ""
}

func (m Model) databaseTableTopLine(table resourceservice.DatabaseTableSize) string {
	const labelWidth = 10
	nameWidth := 44
	if m.detailContentWidth() < 72 {
		nameWidth = 34
	}
	name := fitANSI(emptyDash(table.Name), nameWidth)
	size := "-"
	if table.Size > 0 {
		size = bytesHuman(table.Size)
	}
	return strings.Repeat(" ", labelWidth) +
		detailValueStyle.Render(padVisible(name, nameWidth)+"  ") +
		databaseSizeValue(size)
}

func (m Model) databaseStorageUsageText(d resourceservice.DatabaseExtraDetail) string {
	used := d.SizeBytes
	total := d.TotalBytes
	if used == 0 {
		return m.t("Not collected", "未获取")
	}
	if total == 0 {
		return databaseSizeValue(bytesHuman(used))
	}
	percent := float64(used) * 100 / float64(total)
	thresholds := m.metricThresholds()
	return fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(percent, thresholds.DiskWarn, thresholds.DiskCrit), bytesHuman(used), bytesHuman(total))
}

func databaseSizeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return detailValueStyle.Render(emptyDash(value))
	}
	return detailSizeStyle.Render(value)
}

func expandLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "\n")
		out = append(out, parts...)
	}
	return out
}

func shortContainerID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	return emptyDash(value)
}

func formatContainerTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "0001-01-01") {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return value
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func formatSystemdTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return ""
	}
	t, ok := parseSystemdTimestamp(value)
	if ok {
		return t.Local().Format("2006-01-02 15:04:05")
	}
	return value
}

func parseSystemdTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return time.Time{}, false
	}
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -07",
		"Mon 2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, true
		}
	}
	fields := strings.Fields(value)
	if len(fields) >= 3 {
		trimmed := strings.Join(fields[:3], " ")
		if t, err := time.ParseInLocation("Mon 2006-01-02 15:04:05", trimmed, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func serviceRestartDelayText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return "-"
	}
	if strings.HasSuffix(value, "us") {
		value = strings.TrimSuffix(value, "us")
	}
	us, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	seconds := us / 1_000_000
	text := fmt.Sprintf("%.2fs", seconds)
	return strings.TrimRight(strings.TrimRight(text, "0"), ".")
}

func containerCommandText(d resourceservice.ContainerExtraDetail) string {
	parts := []string{}
	if strings.TrimSpace(d.Path) != "" {
		parts = append(parts, d.Path)
	}
	parts = append(parts, d.Args...)
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "-" {
			return value
		}
	}
	return "-"
}
