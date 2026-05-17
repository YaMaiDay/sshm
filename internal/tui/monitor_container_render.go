package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func containerDetailRows(m Model, state hostState) []string {
	if strings.TrimSpace(state.ContainerError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(m.t("Collection failed", "采集失败")))}
	}
	if len(state.ContainerDetails) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("None found", "未发现"))}
	}
	lines := []string{}
	groups := []struct {
		Title string
		Kind  string
		Style lipgloss.Style
	}{
		{m.t("Failed", "故障"), "failed", detailDangerStyle},
		{m.t("Running", "运行"), "running", detailSubTitleStyle},
		{m.t("Stopped", "停止"), "stopped", detailSubTitleStyle},
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

func containerProblemRows(m Model, state hostState) []string {
	if strings.TrimSpace(state.ContainerError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("Cannot list problems while container collection failed.", "容器采集失败，无法列出异常项。"))}
	}
	if len(state.ContainerDetails) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	items := filterContainersByKind(state.ContainerDetails, "failed")
	if len(items) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	limit := 5
	lines := []string{detailDangerStyle.Render(fmt.Sprintf("· %s %d", m.t("Failed", "故障"), len(items)))}
	nameWidth := containerNameWidth(items)
	for i, item := range items {
		if i >= limit {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  %s %d", m.t("More in Resources:", "更多请到资源页查看："), len(items)-limit)))
			break
		}
		lines = append(lines, containerProblemLine(m, item, nameWidth, i+1))
	}
	return lines
}

func containerProblemLine(m Model, item resourceservice.ContainerDetail, nameWidth int, index int) string {
	state := coloredContainerStatus(emptyDash(item.Status), containerDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	name := detailValueStyle.Render(padVisible(fit(item.Name, nameWidth), nameWidth))
	meta := containerResourceText(item)
	if strings.TrimSpace(item.Image) != "" {
		meta += "  " + item.Image
	}
	return fitANSI(prefix+name+"  "+state+"  "+cardMutedStyle.Render(fitANSI(meta, maxInt(12, m.detailContentWidth()-nameWidth-12))), m.detailContentWidth())
}

func containerNameWidth(items []resourceservice.ContainerDetail) int {
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

func containerDetailItemRows(m Model, item resourceservice.ContainerDetail, nameWidth int, index int) []string {
	status := item.Status
	if status == "" {
		status = "-"
	}
	ports := item.Ports
	state := coloredContainerStatus(status, containerDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	name := detailValueStyle.Render(padVisible(fit(item.Name, nameWidth), nameWidth))
	line := fitANSI(prefix+name+"  "+state, m.detailContentWidth())
	lines := []string{line}
	indent := strings.Repeat(" ", 4)
	if strings.TrimSpace(item.Status) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Status", "状态"), item.Status))
	}
	if strings.TrimSpace(item.CPU) != "" || strings.TrimSpace(item.Memory) != "" || strings.TrimSpace(item.MemPerc) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Resource", "资源"), containerResourceText(item)))
	}
	if strings.TrimSpace(item.Image) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Image", "镜像"), item.Image))
	}
	if simplified := simplifyDockerPorts(ports); simplified != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Ports", "端口"), simplified))
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
	case lower == "missing":
		return "未发现"
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

func (m Model) containerStatusSummary(status string) string {
	if m.isChineseUI() {
		return containerStatusSummary(status)
	}
	raw := strings.TrimSpace(status)
	lower := strings.ToLower(raw)
	switch {
	case lower == "missing":
		return "Not found"
	case strings.Contains(lower, "unhealthy"):
		return strings.TrimSpace("Unhealthy " + dockerStatusAgeEN(raw, "Up"))
	case strings.HasPrefix(lower, "up "):
		age := dockerStatusAgeEN(raw, "Up")
		if strings.Contains(lower, "healthy") {
			return strings.TrimSpace("Healthy " + age)
		}
		return strings.TrimSpace("Running " + age)
	case strings.HasPrefix(lower, "restarting"):
		return strings.TrimSpace("Restarting " + dockerStatusAgoEN(raw))
	case strings.HasPrefix(lower, "exited"):
		return strings.TrimSpace("Exited " + dockerStatusAgoEN(raw))
	case strings.HasPrefix(lower, "created"):
		return strings.TrimSpace("Created " + dockerStatusAgoEN(raw))
	default:
		return raw
	}
}

func dockerStatusAgeEN(status string, prefix string) string {
	status = strings.TrimSpace(status)
	status = strings.TrimPrefix(status, prefix)
	if idx := strings.Index(status, "("); idx >= 0 {
		status = status[:idx]
	}
	return shortDockerDurationEN(status)
}

func dockerStatusAgoEN(status string) string {
	status = strings.TrimSpace(status)
	if idx := strings.LastIndex(status, ")"); idx >= 0 && idx < len(status)-1 {
		status = status[idx+1:]
	}
	status = strings.TrimSuffix(strings.TrimSpace(status), "ago")
	return shortDockerDurationEN(status)
}

func shortDockerDurationEN(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Created ")
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return value
	}
	unit := fields[1]
	switch {
	case strings.HasPrefix(unit, "second"):
		unit = "s"
	case strings.HasPrefix(unit, "minute"):
		unit = "m"
	case strings.HasPrefix(unit, "hour"):
		unit = "h"
	case strings.HasPrefix(unit, "day"):
		unit = "d"
	case strings.HasPrefix(unit, "week"):
		unit = "w"
	case strings.HasPrefix(unit, "month"):
		unit = "mo"
	case strings.HasPrefix(unit, "year"):
		unit = "y"
	}
	return fields[0] + unit
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
	hostPort := resourceservice.PortFromAddress(left)
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
	case "missing":
		return mutedStyle.Render(status)
	case "running":
		return greenStyle.Render(status)
	case "stopped":
		return mutedStyle.Render(status)
	default:
		return detailValueStyle.Render(status)
	}
}

func filterContainersByKind(items []resourceservice.ContainerDetail, kind string) []resourceservice.ContainerDetail {
	out := []resourceservice.ContainerDetail{}
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

func containerDetailKind(item resourceservice.ContainerDetail) string {
	if item.Missing {
		return "missing"
	}
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

func containerDetailCounts(items []resourceservice.ContainerDetail) (int, int, int) {
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
