package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

func bytesHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(value)
	unit := 0
	for f >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f%s", f, units[unit])
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func bytesPair(used uint64, total uint64) string {
	if used == 0 && total == 0 {
		return ""
	}
	return fmt.Sprintf("%s/%s", bytesHuman(used), bytesHuman(total))
}

func diskMountLabel(metrics monitor.Metrics) string {
	mountpoint := strings.TrimSpace(metrics.DiskMountpoint)
	if mountpoint == "" {
		return "-"
	}
	return mountpoint
}

func diskMetricLabel(metrics monitor.Metrics) string {
	mountpoint := diskMountLabel(metrics)
	if mountpoint == "-" || mountpoint == "/" {
		return "磁盘"
	}
	return "磁盘" + mountpoint
}

func (m Model) diskMetricLabel(metrics monitor.Metrics) string {
	if m.isChineseUI() {
		return diskMetricLabel(metrics)
	}
	mountpoint := diskMountLabel(metrics)
	if mountpoint == "-" || mountpoint == "/" {
		return "Disk"
	}
	return "Disk" + mountpoint
}

func (m Model) diskMountPercentText(metrics monitor.Metrics) string {
	thresholds := m.metricThresholds()
	label := diskMountLabel(metrics)
	percent := metricValueStyle(metrics.DiskPercent(), thresholds.DiskWarn, thresholds.DiskCrit).Render(fmt.Sprintf("%.0f%%", metrics.DiskPercent()))
	if label == "-" {
		return percent
	}
	return fit(label+" "+percent, 18)
}

func diskSummaryText(metrics monitor.Metrics) string {
	label := diskMountLabel(metrics)
	size := bytesPair(metrics.DiskUsed, metrics.DiskTotal)
	if label == "-" {
		return size
	}
	if size == "" {
		return label
	}
	return label + " " + size
}

func (m Model) diskListText(metrics monitor.Metrics) string {
	if len(metrics.Disks) == 0 {
		return ""
	}
	disks := append([]monitor.DiskMetric(nil), metrics.Disks...)
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Percent() > disks[j].Percent()
	})
	mountWidth := 8
	for _, disk := range disks {
		if width := ansi.StringWidth(emptyDash(disk.Mountpoint)); width > mountWidth {
			mountWidth = width
		}
	}
	rows := []string{"", m.t("Partitions", "分区")}
	for i, disk := range disks {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, m.diskPartitionInfoLine(i+1, disk, mountWidth))
		rows = append(rows, m.diskPartitionUsageLine(disk))
	}
	rows = append(rows, "")
	return strings.Join(rows, "\n")
}

func (m Model) diskPartitionInfoLine(index int, disk monitor.DiskMetric, mountWidth int) string {
	width := m.detailContentWidth()
	indexText := detailLabelStyle.Render(fmt.Sprintf("%02d", index))
	mount := emptyDash(disk.Mountpoint)
	filesystem := strings.TrimSpace(disk.Filesystem)
	diskType := strings.TrimSpace(disk.Type)
	if filesystem == "" {
		filesystem = "-"
	}
	if diskType == "" {
		diskType = "-"
	}
	deviceLabel := m.t("Device", "设备")
	typeLabel := m.t("Type", "类型")
	suffixRaw := "  " + typeLabel + " " + diskType
	if mountWidth > 24 {
		mountWidth = 24
	}
	prefixRaw := fmt.Sprintf("%02d  %s  %s ", index, padVisible(fit(mount, mountWidth), mountWidth), deviceLabel)
	filesystemWidth := width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	if filesystemWidth < 12 {
		mountWidth = width - ansi.StringWidth(fmt.Sprintf("%02d  ", index)) - ansi.StringWidth("  "+deviceLabel+" ") - ansi.StringWidth(suffixRaw) - 12
		if mountWidth < 8 {
			mountWidth = 8
		}
		mount = fit(mount, mountWidth)
		prefixRaw = fmt.Sprintf("%02d  %s  %s ", index, padVisible(mount, mountWidth), deviceLabel)
		filesystemWidth = width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	}
	if filesystemWidth < 8 {
		filesystemWidth = 8
	}
	mount = padVisible(fit(mount, mountWidth), mountWidth)
	line := indexText +
		"  " + detailValueStyle.Render(mount) +
		"  " + mutedStyle.Render(deviceLabel) + " " + detailValueStyle.Render(fit(filesystem, filesystemWidth)) +
		"  " + mutedStyle.Render(typeLabel) + " " + detailValueStyle.Render(diskType)
	if ansi.StringWidth(line) > width {
		return fitANSI(line, width)
	}
	return line
}

func (m Model) diskPartitionUsageLine(disk monitor.DiskMetric) string {
	thresholds := m.metricThresholds()
	parts := []string{percentBarWithThreshold(disk.Percent(), thresholds.DiskWarn, thresholds.DiskCrit)}
	if size := bytesPair(disk.Used, disk.Total); size != "" {
		parts = append(parts, detailValueStyle.Render(size))
	}
	if disk.AvailKnown {
		parts = append(parts, mutedStyle.Render(m.t("Avail", "可用"))+" "+detailValueStyle.Render(bytesHuman(disk.Available)))
	}
	line := strings.Repeat(" ", 10) + strings.Join(parts, "  ")
	if ansi.StringWidth(line) > m.detailContentWidth() {
		return fitANSI(line, m.detailContentWidth())
	}
	return line
}

func (m Model) swapUsageText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return m.t("Not configured", "未配置")
	}
	return fmt.Sprintf("%s  %s / %s", percentBar(metrics.SwapPercent()), bytesHuman(metrics.SwapUsed), bytesHuman(metrics.SwapTotal))
}

func swapFreeText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "-"
	}
	return bytesHuman(metrics.SwapFree)
}

func (m Model) inodeUsageText(metrics monitor.Metrics) string {
	if metrics.InodeTotal == 0 && metrics.InodeUsed == 0 && metrics.InodeAvailable == 0 {
		return "-"
	}
	thresholds := m.metricThresholds()
	return fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.InodePercent(), thresholds.DiskWarn, thresholds.DiskCrit), countHuman(metrics.InodeUsed), countHuman(metrics.InodeTotal))
}

func countHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"", "K", "M", "B"}
	f := float64(value)
	unit := 0
	for f >= 1000 && unit < len(units)-1 {
		f /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func (m Model) cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	if m.isChineseUI() {
		return fmt.Sprintf("%d核", metrics.CPUCores)
	}
	return fmt.Sprintf("%d cores", metrics.CPUCores)
}
