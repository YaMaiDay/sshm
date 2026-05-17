package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

var asciiModeEnabled bool

func setASCIIMode(enabled bool) {
	asciiModeEnabled = enabled
}

func percentBar(value float64) string {
	return percentBarWidth(value, 8)
}

func percentBarWithThreshold(value float64, warn float64, crit float64) string {
	return percentBarWidthWithThreshold(value, 8, warn, crit)
}

func metricLine(label, value string) string {
	const labelWidth = 5
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return cardMutedStyle.Render(label) + strings.Repeat(" ", padding) + value
}

func cardMetricLine(label string, base string, extra string, width int) string {
	return metricLine(label, compactCardMetric(label, base, extra, width))
}

func compactCardMetric(label string, base string, extra string, width int) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if extra == "" || extra == "-" {
		return base
	}
	full := base + "  " + cardMutedStyle.Render(extra)
	if ansi.StringWidth(metricLine(label, full)) <= width {
		return full
	}
	return base
}

func threeMetricLine(width int, metrics monitor.Metrics) string {
	gap := 1
	colWidth := (width - gap*2) / 3
	if colWidth < 8 {
		colWidth = 8
	}
	barWidth := 4
	if colWidth >= 12 {
		barWidth = 5
	}
	if colWidth >= 15 {
		barWidth = 6
	}
	cpu := compactMetric("CPU", metrics.CPUPercent, colWidth, barWidth)
	mem := compactMetric("内存", metrics.MemPercent(), colWidth, barWidth)
	disk := compactDiskMetric(metrics, colWidth, barWidth)
	line := padVisible(cpu, colWidth) + strings.Repeat(" ", gap) + padVisible(mem, colWidth) + strings.Repeat(" ", gap) + padVisible(disk, colWidth)
	return fit(line, width)
}

func compactMetric(label string, value float64, width int, barWidth int) string {
	return compactMetricWithThreshold(label, value, width, barWidth, 70, 85)
}

func compactMetricWithThreshold(label string, value float64, width int, barWidth int, warn float64, crit float64) string {
	bar := compactPercentBarWithThreshold(value, barWidth, warn, crit)
	labelWidth := 4
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+bar, width)
}

func compactDiskMetric(metrics monitor.Metrics, width int, barWidth int) string {
	label := diskMountLabel(metrics)
	if label == "-" {
		label = "磁盘"
	}
	bar := compactPercentBarWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)
	return fit(label+" "+bar, width)
}

func compactPercentBar(value float64, total int) string {
	return compactPercentBarWithThreshold(value, total, 70, 85)
}

func compactPercentBarWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	filledChar, emptyChar := "▰", "▱"
	if asciiModeEnabled {
		filledChar, emptyChar = "#", "-"
	}
	bar := style.Render(strings.Repeat(filledChar, filled)) + barEmptyStyle.Render(strings.Repeat(emptyChar, total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func padVisible(s string, width int) string {
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "")
	}
	if ansi.StringWidth(s) < width {
		s += strings.Repeat(" ", width-ansi.StringWidth(s))
	}
	return s
}

func cardTopLine(width int, title string, meta string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffixText := dot
	if strings.TrimSpace(meta) != "" && strings.TrimSpace(meta) != "-" {
		suffixText = meta + " " + dot
	}
	suffix := " " + suffixText + " "
	fillWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if fillWidth < 1 {
		title = ansi.Truncate(title, innerWidth-ansi.StringWidth(prefix)-ansi.StringWidth(titleGap)-ansi.StringWidth(suffix)-1, "…")
		fillWidth = innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	}
	if fillWidth < 0 {
		fillWidth = 0
	}
	return left + prefix + title + titleGap + borderStyle.Render(strings.Repeat("─", fillWidth)) + suffix + right
}

func cardNoteText(note string, width int) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return fit("备注 "+note, width)
}

func cardContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := padVisible(content, contentWidth)
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardMutedContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := cardMutedStyle.Render(padVisible(content, contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardInnerSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	line := cardBorderStyle.Render(dashedLine(contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func dashedLine(width int) string {
	if width <= 0 {
		return ""
	}
	pattern := "- "
	line := strings.Repeat(pattern, (width+len(pattern)-1)/len(pattern))
	return ansi.Truncate(line, width, "")
}

func cardSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("├") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("┤")
}

func cardBottomLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("╰") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("╯")
}

func statusDot(loading bool, online bool) string {
	if loading {
		return "●"
	}
	if online {
		return "●"
	}
	return "●"
}

func percentBarWidth(value float64, total int) string {
	return percentBarWidthWithThreshold(value, total, 70, 85)
}

func percentBarWidthWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	filledChar, emptyChar := "▰", "▱"
	if asciiModeEnabled {
		filledChar, emptyChar = "#", "-"
	}
	bar := style.Render(strings.Repeat(filledChar, filled)) + barEmptyStyle.Render(strings.Repeat(emptyChar, total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func metricValueStyle(value float64, warn float64, crit float64) lipgloss.Style {
	if value >= crit {
		return redStyle
	}
	if value >= warn {
		return yellowStyle
	}
	return greenStyle
}

func colorStatus(value string, loading bool, online bool) string {
	if loading {
		return yellowStyle.Render(value)
	}
	if online {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}
