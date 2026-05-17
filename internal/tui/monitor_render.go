package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

func loginSummaryDetailRows(m Model, loading bool, summary []string, errText string, danger bool) []string {
	if loading {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(errText) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(errText))}
	}
	if len(summary) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("None found", "未发现"))}
	}
	lines := make([]string, 0, len(summary))
	for _, line := range summary {
		label, value, ok := strings.Cut(line, "\t")
		if !ok {
			label = "记录"
			value = line
		}
		value = m.loginSummaryValueText(value)
		if danger && label == "统计" {
			value = redStyle.Render(value)
		}
		lines = append(lines, m.detailRow(m.loginSummaryLabel(label), value))
	}
	return lines
}

func (m Model) loginSummaryLabel(label string) string {
	if m.isChineseUI() {
		return label
	}
	switch label {
	case "统计":
		return "Stats"
	case "来源IP":
		return "Source IP"
	case "用户名":
		return "User"
	case "最近":
		return "Latest"
	case "疑似扫描":
		return "Scan"
	case "记录":
		return "Record"
	default:
		return label
	}
}

func (m Model) loginSummaryValueText(value string) string {
	if m.isChineseUI() {
		return value
	}
	value = regexp.MustCompile(`最近(\d+)条`).ReplaceAllString(value, "last $1")
	value = regexp.MustCompile(`(\d+)次`).ReplaceAllString(value, "$1 times")
	value = regexp.MustCompile(`尝试(\d+)个用户名`).ReplaceAllString(value, "tried $1 users")
	value = strings.ReplaceAll(value, "、", ", ")
	return value
}

func (m Model) serviceCardText(metrics monitor.Metrics) string {
	total := dockerTotal(metrics)
	containerLabel := m.t("Ctr", "容器")
	serviceLabel := m.t("Svc", "服务")
	if !metrics.DockerAvailable {
		containerText := cardMutedStyle.Render(containerLabel + " " + m.dockerUnavailableShortText(metrics))
		serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		if metrics.FailedServices > 0 {
			serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		}
		serviceText := cardMutedStyle.Render(serviceLabel+" ") + serviceNumber
		return fmt.Sprintf("%s  %s", containerText, serviceText)
	}
	containerText := cardMutedStyle.Render(fmt.Sprintf("%s %d/%d/%d", containerLabel, metrics.DockerFailed, metrics.DockerRunning, total))
	if total == 0 {
		containerText = cardMutedStyle.Render(containerLabel + " 0")
	}
	if metrics.DockerFailed > 0 {
		containerText = cardMutedStyle.Render(containerLabel+" ") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	serviceText := cardMutedStyle.Render(serviceLabel+" ") + serviceNumber
	return fmt.Sprintf("%s  %s", containerText, serviceText)
}

func (m Model) dockerUnavailableText(metrics monitor.Metrics) string {
	if strings.TrimSpace(metrics.DockerStatus) == "permission" {
		return m.t("Docker permission denied", "Docker无权限")
	}
	return m.t("Docker not installed", "未安装Docker")
}

func (m Model) dockerUnavailableShortText(metrics monitor.Metrics) string {
	if strings.TrimSpace(metrics.DockerStatus) == "permission" {
		return m.t("No permission", "无权限")
	}
	return m.t("N/A", "未安装")
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
