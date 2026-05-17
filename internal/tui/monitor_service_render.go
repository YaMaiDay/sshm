package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/monitor"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
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

func servicePreviewRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if metrics.ServiceTotal > 0 {
		failed := metrics.FailedServices
		stopped := metrics.ServiceStopped
		if failed > 0 && stopped >= failed {
			stopped -= failed
		}
		return previewCountRows(m, metrics.ServiceTotal, metrics.ServiceRunning, stopped, failed)
	}
	if metrics.FailedServices > 0 {
		return []string{m.detailRow(m.t("Failed", "故障"), previewCountStyle(metrics.FailedServices, "failed"))}
	}
	if !metrics.ServiceAvailable {
		return []string{m.detailRow(m.t("Status", "状态"), mutedStyle.Render(m.t("Unavailable", "不可用")))}
	}
	return previewCountRows(m, 0, 0, 0, 0)
}

func containerPreviewRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if !metrics.DockerAvailable {
		return []string{m.detailRow(m.t("Status", "状态"), mutedStyle.Render(m.dockerUnavailableText(metrics)))}
	}
	total := dockerTotal(metrics)
	running := metrics.DockerRunning
	stopped := metrics.DockerStopped
	failed := metrics.DockerFailed
	return previewCountRows(m, total, running, stopped, failed)
}

func previewCountRows(m Model, total, running, stopped, failed int) []string {
	return []string{
		m.detailRow(m.t("Total", "总数"), detailValueStyle.Render(fmt.Sprintf("%d", total))),
		m.detailRow(m.t("Running", "运行"), previewCountStyle(running, "running")),
		m.detailRow(m.t("Stopped", "停止"), previewCountStyle(stopped, "stopped")),
		m.detailRow(m.t("Failed", "故障"), previewCountStyle(failed, "failed")),
	}
}

func previewCountStyle(count int, kind string) string {
	text := fmt.Sprintf("%d", count)
	switch kind {
	case "running":
		if count > 0 {
			return greenStyle.Render(text)
		}
	case "stopped":
		if count > 0 {
			return yellowStyle.Render(text)
		}
	case "failed":
		if count > 0 {
			return redStyle.Render(text)
		}
	}
	return mutedStyle.Render(text)
}

func dockerDetailRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if strings.TrimSpace(state.ContainerError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(m.t("Collection failed", "采集失败")))}
	}
	total := dockerTotal(metrics)
	if len(state.ContainerDetails) > 0 {
		total = len(state.ContainerDetails)
	}
	lines := []string{}
	if total == 0 {
		lines = append(lines, m.detailRow(m.t("Status", "状态"), m.t("None found", "未发现")))
	} else {
		running, stopped, failed := containerDetailCounts(state.ContainerDetails)
		if len(state.ContainerDetails) == 0 && (metrics.DockerRunning > 0 || metrics.DockerStopped > 0 || metrics.DockerFailed > 0) {
			running = metrics.DockerRunning
			stopped = metrics.DockerStopped
			failed = metrics.DockerFailed
		}
		lines = append(lines,
			m.detailRow(m.t("Total", "总数"), fmt.Sprintf("%d", total)),
			m.detailRow(m.t("Running", "运行"), fmt.Sprintf("%d", running)),
			m.detailRow(m.t("Stopped", "停止"), fmt.Sprintf("%d", stopped)),
			m.detailRow(m.t("Failed", "故障"), fmt.Sprintf("%d", failed)),
		)
	}
	return lines
}

func serviceDetailSummaryRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if strings.TrimSpace(state.ServiceError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(m.t("Collection failed", "采集失败")))}
	}
	failed, running, active, stopped := serviceDetailCounts(state.ServiceDetails)
	if len(state.ServiceDetails) == 0 {
		if metrics.FailedServices > 0 {
			return []string{m.detailRow(m.t("Failed", "异常"), redStyle.Render(failedServiceText(metrics, 8)))}
		}
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	other := len(state.ServiceDetails) - failed - running - active - stopped
	lines := []string{
		m.detailRow(m.t("Total", "总数"), fmt.Sprintf("%d", len(state.ServiceDetails))),
		m.detailRow(m.t("Failed", "异常"), serviceCountText(failed, true)),
		m.detailRow(m.t("Running", "运行"), serviceCountText(running, false)),
		m.detailRow(m.t("Active", "活动"), serviceCountText(active, false)),
		m.detailRow(m.t("Stopped", "停止"), serviceCountText(stopped, false)),
	}
	if other > 0 {
		lines = append(lines, m.detailRow(m.t("Other", "其他"), fmt.Sprintf("%d", other)))
	}
	return lines
}

func serviceCountText(count int, danger bool) string {
	text := fmt.Sprintf("%d", count)
	if danger && count > 0 {
		return redStyle.Render(text)
	}
	return text
}

func serviceDetailRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if strings.TrimSpace(state.ServiceError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), redStyle.Render(state.ServiceError))}
	}
	if len(state.ServiceDetails) == 0 {
		if metrics.FailedServices > 0 {
			return failedServiceFallbackRows(m, metrics)
		}
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	lines := []string{}
	groups := []struct {
		Title string
		Kind  string
		Style lipgloss.Style
	}{
		{m.t("Failed", "异常"), "failed", detailDangerStyle},
		{m.t("Running", "运行"), "running", detailSubTitleStyle},
		{m.t("Active", "活动"), "active", detailSubTitleStyle},
		{m.t("Stopped", "停止"), "stopped", detailSubTitleStyle},
		{m.t("Other", "其他"), "other", detailSubTitleStyle},
	}
	firstGroup := true
	for _, group := range groups {
		items := filterServicesByKind(state.ServiceDetails, group.Kind)
		if len(items) == 0 {
			continue
		}
		if !firstGroup {
			lines = append(lines, "")
		}
		firstGroup = false
		lines = append(lines, group.Style.Render(fmt.Sprintf("· %s %d", group.Title, len(items))))
		unitWidth := serviceUnitWidth(items)
		for i, item := range items {
			lines = append(lines, serviceDetailItemRows(m, item, unitWidth, i+1)...)
		}
	}
	return lines
}

func serviceProblemRows(m Model, metrics monitor.Metrics, state hostState) []string {
	if strings.TrimSpace(state.ServiceError) != "" {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("Cannot list problems while service collection failed.", "服务采集失败，无法列出异常项。"))}
	}
	if len(state.ServiceDetails) == 0 {
		if metrics.FailedServices > 0 {
			return failedServiceFallbackRows(m, metrics)
		}
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	items := filterServicesByKind(state.ServiceDetails, "failed")
	if len(items) == 0 {
		return []string{m.detailRow(m.t("Status", "状态"), m.t("No problems found", "未发现异常"))}
	}
	limit := 5
	lines := []string{detailDangerStyle.Render(fmt.Sprintf("· %s %d", m.t("Failed", "异常"), len(items)))}
	unitWidth := serviceUnitWidth(items)
	for i, item := range items {
		if i >= limit {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("  %s %d", m.t("More in Resources:", "更多请到资源页查看："), len(items)-limit)))
			break
		}
		lines = append(lines, serviceProblemLine(m, item, unitWidth, i+1))
	}
	return lines
}

func serviceProblemLine(m Model, item resourceservice.ServiceDetail, unitWidth int, index int) string {
	state := coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	unit := detailValueStyle.Render(padVisible(fit(item.Unit, unitWidth), unitWidth))
	meta := strings.TrimSpace(strings.Join([]string{serviceRawState(item), servicePIDText(item), serviceMemoryText(item)}, "  "))
	if meta != "" && meta != "-" {
		meta = "  " + cardMutedStyle.Render(fitANSI(meta, maxInt(12, m.detailContentWidth()-unitWidth-12)))
	}
	return fitANSI(prefix+unit+"  "+state+meta, m.detailContentWidth())
}

func failedServiceFallbackRows(m Model, metrics monitor.Metrics) []string {
	lines := []string{detailDangerStyle.Render(fmt.Sprintf("· %s %d", m.t("Failed", "异常"), metrics.FailedServices))}
	if len(metrics.FailedUnits) == 0 {
		lines = append(lines, m.detailRow(m.t("Failed services", "异常服务"), failedServiceText(metrics, 8)))
		return lines
	}
	unitWidth := 10
	for _, unit := range metrics.FailedUnits {
		if w := runewidth.StringWidth(unit); w > unitWidth {
			unitWidth = w
		}
	}
	if unitWidth > 36 {
		unitWidth = 36
	}
	for i, unit := range metrics.FailedUnits {
		item := resourceservice.ServiceDetail{Unit: unit, Active: "failed", Sub: "failed"}
		lines = append(lines, serviceDetailItemRows(m, item, unitWidth, i+1)...)
	}
	return lines
}

func filterServicesByKind(items []resourceservice.ServiceDetail, kind string) []resourceservice.ServiceDetail {
	out := []resourceservice.ServiceDetail{}
	for _, item := range items {
		if serviceDetailKind(item) == kind {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Unit) < strings.ToLower(out[j].Unit)
	})
	return out
}

func serviceUnitWidth(items []resourceservice.ServiceDetail) int {
	width := 10
	for _, item := range items {
		if w := runewidth.StringWidth(item.Unit); w > width {
			width = w
		}
	}
	if width > 36 {
		width = 36
	}
	return width
}

func serviceDetailItemRows(m Model, item resourceservice.ServiceDetail, unitWidth int, index int) []string {
	state := coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	unit := detailValueStyle.Render(padVisible(fit(item.Unit, unitWidth), unitWidth))
	line := fitANSI(prefix+unit+"  "+state, m.detailContentWidth())
	lines := []string{line}
	indent := strings.Repeat(" ", 4)
	if serviceRawState(item) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("State", "状态"), serviceRawState(item)))
	}
	if pid := servicePIDText(item); pid != "" {
		lines = append(lines, containerIndentedLine(m, indent, "PID", pid))
	}
	if item.MemoryCurrent > 0 {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Memory", "内存"), serviceMemoryText(item)))
	}
	if strings.TrimSpace(item.Load) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Load", "加载"), item.Load))
	}
	if strings.TrimSpace(item.Description) != "" {
		lines = append(lines, containerIndentedLine(m, indent, m.t("Desc", "说明"), item.Description))
	}
	return lines
}

func serviceDetailCounts(items []resourceservice.ServiceDetail) (int, int, int, int) {
	failed := 0
	running := 0
	active := 0
	stopped := 0
	for _, item := range items {
		switch serviceDetailKind(item) {
		case "failed":
			failed++
		case "running":
			running++
		case "active":
			active++
		case "stopped":
			stopped++
		}
	}
	return failed, running, active, stopped
}

func serviceDetailKindRank(item resourceservice.ServiceDetail) int {
	switch serviceDetailKind(item) {
	case "failed":
		return 0
	case "running":
		return 1
	case "active":
		return 2
	case "stopped":
		return 3
	default:
		return 4
	}
}

func serviceDetailKind(item resourceservice.ServiceDetail) string {
	if item.Missing {
		return "missing"
	}
	active := strings.ToLower(strings.TrimSpace(item.Active))
	sub := strings.ToLower(strings.TrimSpace(item.Sub))
	load := strings.ToLower(strings.TrimSpace(item.Load))
	switch {
	case active == "failed" || sub == "failed" || load == "not-found" || load == "error":
		return "failed"
	case active == "active" && sub == "running":
		return "running"
	case active == "active":
		return "active"
	case active == "inactive" || sub == "dead":
		return "stopped"
	default:
		return "other"
	}
}

func serviceStatusText(item resourceservice.ServiceDetail) string {
	switch serviceDetailKind(item) {
	case "missing":
		return "未发现"
	case "failed":
		return "异常"
	case "running":
		return "运行"
	case "active":
		return "活动"
	case "stopped":
		return "停止"
	default:
		state := serviceRawState(item)
		if state == "" {
			return "未知"
		}
		return state
	}
}

func (m Model) serviceStatusText(item resourceservice.ServiceDetail) string {
	if m.isChineseUI() {
		return serviceStatusText(item)
	}
	switch serviceDetailKind(item) {
	case "missing":
		return "Not found"
	case "failed":
		return "Failed"
	case "running":
		return "Running"
	case "active":
		return "Active"
	case "stopped":
		return "Stopped"
	default:
		state := serviceRawState(item)
		if state == "" {
			return "Unknown"
		}
		return state
	}
}

func serviceRawState(item resourceservice.ServiceDetail) string {
	parts := []string{}
	if strings.TrimSpace(item.Active) != "" {
		parts = append(parts, item.Active)
	}
	if strings.TrimSpace(item.Sub) != "" {
		parts = append(parts, item.Sub)
	}
	return strings.Join(parts, "/")
}

func coloredServiceStatus(status string, kind string) string {
	switch kind {
	case "failed":
		return redStyle.Render(status)
	case "missing", "stopped":
		return mutedStyle.Render(status)
	case "running":
		return greenStyle.Render(status)
	case "active":
		return yellowStyle.Render(status)
	default:
		return detailValueStyle.Render(status)
	}
}
