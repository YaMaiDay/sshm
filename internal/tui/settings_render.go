package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func (m Model) renderSettings() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 48 {
		bodyWidth = 48
	}
	height := m.height - 4
	if height < 12 {
		height = 12
	}
	lines := m.settingsLines(bodyWidth)
	start, end := visibleRange(len(lines), selectedSettingsRow(m.settingsField), height)
	lines = lines[start:end]
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(settingsHeader(m.status, width), width)),
		box,
		renderHelp(width, "切换 ↑↓/jk/Tab  修改 ←→/输入  保存 Enter  返回 Esc"),
	}, "\n")
}

func settingsHeader(status string, width int) string {
	title := "设置"
	if strings.TrimSpace(status) == "" || status == title {
		return title
	}
	statusStyle := mutedStyle
	if strings.Contains(status, "失败") || strings.Contains(status, "不能") || strings.Contains(status, "需要") {
		statusStyle = redStyle
	}
	return title + "  " + statusStyle.Render(fit(status, width-ansi.StringWidth(title)-2))
}

func (m Model) settingsLines(width int) []string {
	return []string{
		deploymentSectionTitle("界面"),
		settingsChoiceLine(m, settingsLanguage, "语言", settingsLanguageText(m.settingsForm.Language), width),
		settingsChoiceLine(m, settingsASCIIMode, "ASCII 模式", yesNo(m.settingsForm.ASCIIMode), width),
		"",
		deploymentSectionTitle("监控"),
		settingsInputLine(m, settingsRefreshInterval, "刷新间隔", m.settingsForm.RefreshInterval, width, "例如 5s、30s、1m"),
		settingsInputLine(m, settingsConnectTimeout, "连接超时", m.settingsForm.ConnectTimeout, width, "例如 2s、5s"),
		settingsInputLine(m, settingsCommandTimeout, "命令超时", m.settingsForm.CommandTimeout, width, "例如 6s、30s"),
		"",
		deploymentSectionTitle("告警阈值"),
		settingsInputLine(m, settingsCPUWarn, "CPU 警告", m.settingsForm.CPUWarn, width, "百分比"),
		settingsInputLine(m, settingsCPUCrit, "CPU 严重", m.settingsForm.CPUCrit, width, "百分比"),
		settingsInputLine(m, settingsMemWarn, "内存警告", m.settingsForm.MemWarn, width, "百分比"),
		settingsInputLine(m, settingsMemCrit, "内存严重", m.settingsForm.MemCrit, width, "百分比"),
		settingsInputLine(m, settingsDiskWarn, "磁盘警告", m.settingsForm.DiskWarn, width, "百分比"),
		settingsInputLine(m, settingsDiskCrit, "磁盘严重", m.settingsForm.DiskCrit, width, "百分比"),
		"",
		deploymentSectionTitle("常用目录"),
		settingsInputLine(m, settingsLocalDirs, "本地目录", m.settingsForm.LocalDirs, width, "逗号分隔"),
		settingsInputLine(m, settingsRemoteDirs, "远程目录", m.settingsForm.RemoteDirs, width, "逗号分隔"),
		"",
		mutedStyle.Render(fit("说明：英文是默认语言，中文作为第二语言保留。", width)),
	}
}

func settingsLanguageText(value string) string {
	if value == "zh" {
		return "中文"
	}
	return "English"
}

func settingsChoiceLine(m Model, field int, label string, value string, width int) string {
	return settingsFieldLine(m, field, label, value+"  ←/→", width)
}

func settingsInputLine(m Model, field int, label string, value string, width int, hint string) string {
	inputWidth := settingsInputWidth(width)
	display := commandInputText(value, m.settingsCursor, m.settingsField == field, inputWidth)
	if m.settingsField != field && strings.TrimSpace(value) == "" {
		display = "[" + fit(hint, inputWidth) + strings.Repeat(" ", maxInt(0, inputWidth-ansi.StringWidth(hint))) + "]"
	}
	return settingsFieldLine(m, field, label, display, width)
}

func settingsFieldLine(m Model, field int, label string, value string, width int) string {
	prefix := " "
	style := lipgloss.NewStyle()
	if m.settingsField == field {
		prefix = "▶"
		style = blueStyle.Bold(true)
	}
	labelWidth := runewidth.StringWidth("刷新间隔")
	padding := labelWidth - runewidth.StringWidth(label) + 2
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func settingsInputWidth(width int) int {
	inputWidth := width - runewidth.StringWidth("▶ 远程目录  ") - 2
	if inputWidth > 58 {
		inputWidth = 58
	}
	if inputWidth < 18 {
		inputWidth = 18
	}
	return inputWidth
}

func selectedSettingsRow(field int) int {
	switch field {
	case settingsLanguage:
		return 1
	case settingsASCIIMode:
		return 2
	case settingsRefreshInterval:
		return 5
	case settingsConnectTimeout:
		return 6
	case settingsCommandTimeout:
		return 7
	case settingsCPUWarn:
		return 10
	case settingsCPUCrit:
		return 11
	case settingsMemWarn:
		return 12
	case settingsMemCrit:
		return 13
	case settingsDiskWarn:
		return 14
	case settingsDiskCrit:
		return 15
	case settingsLocalDirs:
		return 18
	case settingsRemoteDirs:
		return 19
	default:
		return 0
	}
}
