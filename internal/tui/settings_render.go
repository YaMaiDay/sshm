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
		titleStyle.Render(fit(m.settingsHeader(width), width)),
		box,
		renderHelp(width, m.settingsHelp()),
	}, "\n")
}

func (m Model) settingsHeader(width int) string {
	title := "设置"
	if !m.isChineseUI() {
		title = "Settings"
	}
	status := strings.TrimSpace(m.status)
	if status == "" || status == title {
		return title
	}
	statusStyle := mutedStyle
	lowerStatus := strings.ToLower(status)
	if strings.Contains(status, "失败") || strings.Contains(status, "不能") || strings.Contains(status, "需要") ||
		strings.Contains(lowerStatus, "failed") || strings.Contains(lowerStatus, "cannot") || strings.Contains(lowerStatus, "must") {
		statusStyle = redStyle
	}
	return title + "  " + statusStyle.Render(fit(status, width-ansi.StringWidth(title)-2))
}

func (m Model) settingsLines(width int) []string {
	if !m.isChineseUI() {
		return []string{
			deploymentSectionTitle("Interface"),
			settingsChoiceLine(m, settingsLanguage, "Language", settingsLanguageText(m.settingsForm.Language), width),
			settingsChoiceLine(m, settingsASCIIMode, "ASCII mode", yesNoLang(m.settingsForm.ASCIIMode, false), width),
			"",
			deploymentSectionTitle("Monitoring"),
			settingsInputLine(m, settingsRefreshInterval, "Refresh interval", m.settingsForm.RefreshInterval, width, "e.g. 5s, 30s, 1m"),
			settingsInputLine(m, settingsConnectTimeout, "Connect timeout", m.settingsForm.ConnectTimeout, width, "e.g. 2s, 5s"),
			settingsInputLine(m, settingsCommandTimeout, "Command timeout", m.settingsForm.CommandTimeout, width, "e.g. 6s, 30s"),
			"",
			deploymentSectionTitle("Alert Thresholds"),
			settingsInputLine(m, settingsCPUWarn, "CPU warn", m.settingsForm.CPUWarn, width, "percent"),
			settingsInputLine(m, settingsCPUCrit, "CPU critical", m.settingsForm.CPUCrit, width, "percent"),
			settingsInputLine(m, settingsMemWarn, "Memory warn", m.settingsForm.MemWarn, width, "percent"),
			settingsInputLine(m, settingsMemCrit, "Memory critical", m.settingsForm.MemCrit, width, "percent"),
			settingsInputLine(m, settingsDiskWarn, "Disk warn", m.settingsForm.DiskWarn, width, "percent"),
			settingsInputLine(m, settingsDiskCrit, "Disk critical", m.settingsForm.DiskCrit, width, "percent"),
			"",
			deploymentSectionTitle("Directories"),
			settingsInputLine(m, settingsLocalDirs, "Local dirs", m.settingsForm.LocalDirs, width, "comma separated"),
			settingsInputLine(m, settingsRemoteDirs, "Remote dirs", m.settingsForm.RemoteDirs, width, "comma separated"),
			"",
			mutedStyle.Render(fit("Note: English is the default language. Chinese remains available as a secondary language.", width)),
		}
	}
	return []string{
		deploymentSectionTitle("界面"),
		settingsChoiceLine(m, settingsLanguage, "语言", settingsLanguageText(m.settingsForm.Language), width),
		settingsChoiceLine(m, settingsASCIIMode, "ASCII 模式", yesNoLang(m.settingsForm.ASCIIMode, true), width),
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

func yesNoLang(value bool, zh bool) string {
	if zh {
		return yesNo(value)
	}
	if value {
		return "Yes"
	}
	return "No"
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
	labelWidth := runewidth.StringWidth("Refresh interval")
	if m.isChineseUI() {
		labelWidth = runewidth.StringWidth("刷新间隔")
	}
	padding := labelWidth - runewidth.StringWidth(label) + 2
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func settingsInputWidth(width int) int {
	inputWidth := width - runewidth.StringWidth("▶ Refresh interval  ") - 2
	if inputWidth > 58 {
		inputWidth = 58
	}
	if inputWidth < 18 {
		inputWidth = 18
	}
	return inputWidth
}

func (m Model) settingsHelp() string {
	if m.isChineseUI() {
		return "切换 ↑↓/jk/Tab  修改 ←→/输入  保存 Enter  返回 Esc"
	}
	return "Move ↑↓/jk/Tab  Change ←→/type  Save Enter  Back Esc"
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
