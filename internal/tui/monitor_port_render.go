package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func portDetailRows(m Model, state hostState) []string {
	if state.LoginLoading {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(state.PortDetailsError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(m.t("Collection failed", "采集失败")))}
	}
	if len(state.PortDetails) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("None found", "未发现"))}
	}
	groups := groupedPortDetails(state.PortDetails)
	lines := []string{}
	groupDefs := []struct {
		Title string
		Key   string
	}{
		{m.t("System ports", "系统端口"), "system"},
		{m.t("Docker ports", "Docker端口"), "docker"},
		{m.t("App ports", "应用端口"), "app"},
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

func portDetailItemRows(m Model, items []resourceservice.PortDetail) []string {
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
			pid = fmt.Sprintf("%s %s", pid, m.t(fmt.Sprintf("%d total", item.Count), fmt.Sprintf("等%d个", item.Count)))
		}
		label := strings.TrimSpace(item.Protocol + "/" + item.Port)
		processPadding := processWidth - runewidth.StringWidth(process) + 2
		if processPadding < 1 {
			processPadding = 1
		}
		value := process + strings.Repeat(" ", processPadding) + "pid:" + pid
		if strings.TrimSpace(item.LocalAddress) != "" {
			value += "  " + m.t("listen ", "监听 ") + item.LocalAddress
		}
		lines = append(lines, detailAlignedRow(m, label, value, labelWidth))
	}
	return lines
}

func groupedPortDetails(items []resourceservice.PortDetail) map[string][]resourceservice.PortDetail {
	groups := map[string][]resourceservice.PortDetail{"system": {}, "docker": {}, "app": {}}
	for _, item := range items {
		group := portDetailGroup(item)
		groups[group] = append(groups[group], item)
	}
	return groups
}

func portDetailGroup(item resourceservice.PortDetail) string {
	if strings.TrimSpace(item.Container) != "" || strings.TrimSpace(item.Process) == "docker-proxy" {
		return "docker"
	}
	port, _ := strconv.Atoi(item.Port)
	if port > 0 && port < 1024 {
		return "system"
	}
	return "app"
}

func portProcessText(item resourceservice.PortDetail) string {
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
