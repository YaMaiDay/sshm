package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

func (m Model) renderDashboardList(indexes []int, width int) string {
	if width <= 0 {
		width = contentWidth(m.width)
	}
	height := m.dashboardListHeight()
	start, end := visibleRange(len(indexes), m.selected, height)
	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		realIndex := indexes[i]
		lines = append(lines, m.dashboardListLine(realIndex, i == m.selected, width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardListHeight() int {
	height := m.height - 4
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) dashboardListLine(index int, selected bool, width int) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	status := m.t("Offline", "离线")
	if state.Loading {
		status = m.t("Loading", "采集")
	} else if metrics.Online {
		status = m.t("Online", "在线")
	}
	nameWidth := 28
	if width < 110 {
		nameWidth = 22
	}
	if width < 78 {
		nameWidth = 16
	}
	name := nameStyle.Render(padVisible(fitANSI(dashboardHostDisplayName(h), nameWidth), nameWidth))
	statusText := padVisible(colorStatus(status, state.Loading, metrics.Online), 6)
	cpu, mem, disk := m.dashboardListResourceColumns(state)
	containerText, serviceText := m.dashboardListServiceColumns(metrics)
	expire := padVisible(m.expireCardTextOrDash(h.ExpireAt), 10)
	addressWidth := 22
	if width < 100 {
		addressWidth = 16
	}
	address := cardMutedStyle.Render(padVisible(fit(h.Address(), addressWidth), addressWidth))
	line := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s  %s  %s", prefix, name, statusText, cpu, mem, disk, containerText, serviceText, expire, address)
	return fitANSI(line, width)
}

func (m Model) dashboardListResourceColumns(state hostState) (string, string, string) {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render(padVisible("CPU -", 7)),
			detailValueStyle.Render(padVisible(m.t("Mem -", "内存 -"), 8)),
			detailValueStyle.Render(padVisible(m.t("Disk -", "磁盘 -"), 8))
	}
	thresholds := m.metricThresholds()
	cpu := "CPU " + metricValueStyle(metrics.CPUPercent, thresholds.CPUWarn, thresholds.CPUCrit).Render(fmt.Sprintf("%3.0f%%", metrics.CPUPercent))
	mem := m.t("Mem ", "内存 ") + metricValueStyle(metrics.MemPercent(), thresholds.MemWarn, thresholds.MemCrit).Render(fmt.Sprintf("%3.0f%%", metrics.MemPercent()))
	disk := m.t("Disk ", "磁盘 ") + m.diskMountPercentText(metrics)
	return padVisible(cpu, 7), padVisible(mem, 8), padVisible(disk, 14)
}

func (m Model) dashboardListServiceColumns(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	containerLabel := m.t("Ctr", "容器")
	serviceLabel := m.t("Svc", "服务")
	if !metrics.DockerAvailable {
		containerText := cardMutedStyle.Render(containerLabel + " " + m.dockerUnavailableShortText(metrics))
		serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		if metrics.FailedServices > 0 {
			serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		}
		service := cardMutedStyle.Render(serviceLabel+" ") + serviceNumber
		return padVisible(containerText, ansi.StringWidth(containerLabel+" "+m.dockerUnavailableShortText(metrics))), padVisible(service, 7)
	}
	containerRaw := fmt.Sprintf("%s %d/%d/%d", containerLabel, metrics.DockerFailed, metrics.DockerRunning, total)
	if total == 0 {
		containerRaw = containerLabel + " 0"
	}
	container := cardMutedStyle.Render(containerLabel + " ")
	if metrics.DockerFailed > 0 {
		container += redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	} else if total == 0 {
		container += cardMutedStyle.Render("0")
	} else {
		container += cardMutedStyle.Render(fmt.Sprintf("0/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	service := cardMutedStyle.Render(serviceLabel+" ") + serviceNumber
	return padVisible(container, maxInt(12, ansi.StringWidth(containerRaw))), padVisible(service, 7)
}

func (m Model) compactResourceTriplet(state hostState) (string, string, string) {
	metrics := state.Metrics
	memLabel := m.t("M", "内")
	diskLabel := m.t("D", "磁")
	if state.Loading || !metrics.Online {
		return cardMutedStyle.Render("CPU") + detailValueStyle.Render("-"),
			cardMutedStyle.Render(memLabel) + detailValueStyle.Render("-"),
			cardMutedStyle.Render(diskLabel) + detailValueStyle.Render("-")
	}
	thresholds := m.metricThresholds()
	return cardMutedStyle.Render("CPU") + metricValueStyle(metrics.CPUPercent, thresholds.CPUWarn, thresholds.CPUCrit).Render(fmt.Sprintf("%.0f", metrics.CPUPercent)),
		cardMutedStyle.Render(memLabel) + metricValueStyle(metrics.MemPercent(), thresholds.MemWarn, thresholds.MemCrit).Render(fmt.Sprintf("%.0f", metrics.MemPercent())),
		cardMutedStyle.Render(diskLabel) + metricValueStyle(metrics.DiskPercent(), thresholds.DiskWarn, thresholds.DiskCrit).Render(fmt.Sprintf("%.0f", metrics.DiskPercent()))
}

func (m Model) compactServicePair(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	containerLabel := m.t("Ctr", "容器")
	serviceLabel := m.t("Svc", "服务")
	if !metrics.DockerAvailable {
		serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		if metrics.FailedServices > 0 {
			serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
		}
		return cardMutedStyle.Render(containerLabel + m.dockerUnavailableShortText(metrics)), cardMutedStyle.Render(serviceLabel) + serviceNumber
	}
	container := containerLabel + "0"
	if total > 0 {
		if metrics.DockerFailed > 0 {
			container = cardMutedStyle.Render(containerLabel) + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
		} else {
			container = cardMutedStyle.Render(fmt.Sprintf("%s0/%d/%d", containerLabel, metrics.DockerRunning, total))
		}
	} else {
		container = cardMutedStyle.Render(container)
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	return container, cardMutedStyle.Render(serviceLabel) + serviceNumber
}

func (m Model) compactExpireText(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render(m.t("Exp-", "到期-"))
	}
	return m.expireCardText(value)
}

func (m Model) expireCardTextOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render(m.t("Exp -", "到期 -"))
	}
	return m.expireCardText(value)
}
