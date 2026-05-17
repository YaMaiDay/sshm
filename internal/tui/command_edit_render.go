package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderCommandEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 36 {
		innerWidth = 36
	}
	title := m.t("Add Command Template", "添加命令模板")
	if m.commandEditing {
		title = m.t("Edit Command Template", "编辑命令模板")
	}
	scope := m.t("Global", "全局") + "  ←/→"
	server := "-"
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		h := m.states[m.activeCommand.HostIndex].Host
		server = config.ServerCommandKey(h.Category, h.Name)
	}
	if m.commandForm.Scope == commandScopeServer {
		scope = server + "  ←/→"
	}
	header := title
	if m.commandForm.Scope == commandScopeServer && server != "-" {
		header += "  " + server
	}
	lines := []string{}
	lines = append(lines, commandFieldLine(m, 0, m.t("Scope", "范围"), scope, innerWidth))
	lines = append(lines, commandFieldLine(m, 1, m.t("Template name", "模板名称"), commandInputText(m.commandForm.Name, m.commandCursor, m.commandField == 1, 28), innerWidth))
	lines = append(lines, "")
	help := m.t("Switch Tab  Save Enter  New line Ctrl+J  Back Esc", "切换 Tab  保存 Enter  换行 Ctrl+J  返回 Esc")
	lines = append(lines, detailSubTitle(m.t("Command", "命令内容")))
	lines = append(lines, commandTextArea(m.commandForm.Command, m.commandCursor, m.commandField == 2, innerWidth, m.commandTextAreaHeight(help)))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func commandFieldLine(m Model, index int, label string, value string, width int) string {
	prefix := " "
	style := lipgloss.NewStyle()
	if m.commandField == index {
		prefix = "▶"
		style = blueStyle.Bold(true)
	}
	labelWidth := runewidth.StringWidth("Template name")
	if m.isChineseUI() {
		labelWidth = runewidth.StringWidth("模板名称")
	}
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func commandInputText(value string, cursor int, active bool, width int) string {
	if width < 8 {
		width = 8
	}
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if active {
		return "[" + formInputText(value, width, cursor) + "]"
	}
	fitted := padVisible(value, width)
	return "[" + fitted + strings.Repeat(" ", maxInt(0, width-runewidth.StringWidth(fitted))) + "]"
}

func (m Model) commandTextAreaHeight(help string) int {
	contentLinesBeforeTextArea := 4
	textareaBorderLines := 2
	formBorderLines := 2
	externalHeaderLines := 1
	height := m.height - externalHeaderLines - contentLinesBeforeTextArea - textareaBorderLines - formBorderLines - 1
	if height < 6 {
		height = 6
	}
	return height
}

func commandTextArea(value string, cursor int, active bool, width int, height int) string {
	bodyWidth := width - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	if height < 4 {
		height = 4
	}
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	cursorLine := 0
	cursorCol := 0
	if active {
		cursorLine, cursorCol = cursorTextPosition(runes, cursor)
	}
	lines := strings.Split(value, "\n")
	start := 0
	if len(lines) > height {
		start = cursorLine - height + 1
		if start < 0 {
			start = 0
		}
		if start+height > len(lines) {
			start = len(lines) - height
		}
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	viewLines := make([]string, 0, height)
	for i := start; i < end; i++ {
		if active && i == cursorLine {
			viewLines = append(viewLines, formInputText(lines[i], bodyWidth, cursorCol))
			continue
		}
		viewLines = append(viewLines, fit(lines[i], bodyWidth))
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(viewLines, "\n"))
}

func cursorTextPosition(runes []rune, cursor int) (int, int) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	line := 0
	col := 0
	for i := 0; i < cursor; i++ {
		if runes[i] == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return line, col
}
