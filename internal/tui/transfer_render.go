package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func transferProgressBarLine(entry config.TransferEntry, width int) string {
	percent, ok := transferProgressPercent(entry)
	if !ok && entry.Status == config.TransferStatusDone {
		percent = 100
		ok = true
	}
	label := "--"
	if ok {
		label = fmt.Sprintf("%3d%%", percent)
	}
	style := transferProgressStyle(entry.Status)
	suffix := style.Render(label)
	if detail := transferProgressDetail(entry); detail != "" {
		maxDetail := width - 8 - runewidth.StringWidth(label) - 2
		if maxDetail > 4 {
			suffix += " " + cardMutedStyle.Render(fit(detail, maxDetail))
		}
	}
	barWidth := width - ansi.StringWidth(suffix) - 1
	if barWidth < 8 {
		barWidth = 8
	}
	filled := 0
	if ok {
		filled = int(float64(barWidth) * float64(percent) / 100)
		if percent > 0 && filled == 0 {
			filled = 1
		}
		if filled > barWidth {
			filled = barWidth
		}
	}
	filledChar, emptyChar := "▰", "▱"
	if asciiModeEnabled {
		filledChar, emptyChar = "#", "-"
	}
	bar := style.Render(strings.Repeat(filledChar, filled)) + barEmptyStyle.Render(strings.Repeat(emptyChar, barWidth-filled))
	return bar + " " + suffix
}

func transferProgressPercent(entry config.TransferEntry) (int, bool) {
	if entry.TotalBytes > 0 {
		done := transferProgressDoneBytes(entry)
		percent := int(float64(done) * 100 / float64(entry.TotalBytes))
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		return percent, true
	}
	percentText := transferservice.PercentText(entry.Progress)
	if percentText == "" {
		return 0, false
	}
	value := strings.TrimSuffix(percentText, "%")
	percent, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return percent, true
}

func transferProgressDoneBytes(entry config.TransferEntry) int64 {
	done := entry.DoneBytes + entry.CurrentBytes
	if entry.TotalBytes > 0 && done > entry.TotalBytes {
		return entry.TotalBytes
	}
	if done < 0 {
		return 0
	}
	return done
}

func transferProgressDetail(entry config.TransferEntry) string {
	sizeText := ""
	if entry.TotalBytes > 0 {
		sizeText = bytesPair(uint64(transferProgressDoneBytes(entry)), uint64(entry.TotalBytes))
	}
	progress := strings.Join(strings.Fields(strings.TrimSpace(entry.Progress)), " ")
	percent := transferservice.PercentText(progress)
	if progress == "" || percent == "" || progress == percent {
		return sizeText
	}
	if idx := strings.Index(progress, " ("); idx >= 0 {
		progress = strings.TrimSpace(progress[:idx])
	}
	idx := strings.Index(progress, percent)
	if idx < 0 {
		return ""
	}
	before := strings.TrimSpace(progress[:idx])
	after := strings.TrimSpace(progress[idx+len(percent):])
	rsyncText := strings.TrimSpace(before + " " + after)
	if sizeText == "" {
		return rsyncText
	}
	if after == "" {
		return sizeText
	}
	return strings.TrimSpace(sizeText + " " + after)
}

func transferProgressStyle(status string) lipgloss.Style {
	switch status {
	case config.TransferStatusQueued:
		return detailSubTitleStyle
	case config.TransferStatusPending:
		return blueStyle
	case config.TransferStatusRunning:
		return blueStyle
	case config.TransferStatusDone:
		return greenStyle
	case config.TransferStatusFailed:
		return redStyle
	case config.TransferStatusInterrupted:
		return yellowStyle
	case config.TransferStatusCanceled:
		return mutedStyle
	default:
		return mutedStyle
	}
}

func transferStatusColor(status string) lipgloss.Color {
	switch status {
	case config.TransferStatusQueued:
		return cyan
	case config.TransferStatusPending:
		return blue
	case config.TransferStatusRunning:
		return blue
	case config.TransferStatusDone:
		return green
	case config.TransferStatusFailed:
		return red
	case config.TransferStatusInterrupted:
		return yellow
	case config.TransferStatusCanceled:
		return textGray
	default:
		return textGray
	}
}

func (m Model) transferStatusText(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return m.t("Queued", "等待中")
	case config.TransferStatusPending:
		return m.t("Pending", "排队中")
	case config.TransferStatusRunning:
		return m.t("Running", "运行中")
	case config.TransferStatusDone:
		return m.t("Done", "已完成")
	case config.TransferStatusFailed:
		return m.t("Failed", "失败")
	case config.TransferStatusCanceled:
		return m.t("Canceled", "已取消")
	case config.TransferStatusInterrupted:
		return m.t("Interrupted", "中断")
	default:
		return status
	}
}

func (m Model) transferEntryKindText(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return m.t("Download", "下载")
	}
	return m.t("Upload", "上传")
}

func transferTimeText(entry config.TransferEntry) string {
	if entry.UpdatedAt != "" {
		return transferTimeShort(entry.UpdatedAt)
	}
	return transferTimeShort(entry.Time)
}

func transferTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return t.Local().Format("01-02 15:04")
}

func transferStatusCounts(entries []config.TransferEntry) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Status]++
	}
	return counts
}

func transferUnfinishedCount(entries []config.TransferEntry) int {
	total := 0
	for _, entry := range entries {
		if entry.Status == config.TransferStatusQueued || entry.Status == config.TransferStatusPending || entry.Status == config.TransferStatusRunning || entry.Status == config.TransferStatusInterrupted {
			total++
		}
	}
	return total
}
