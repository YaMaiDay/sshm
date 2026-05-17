package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m Model) renderCommandConfirm() string {
	width := detailFrameWidth(m.width)
	help := m.t("Scroll ↑↓/jk  Run Enter  Back Esc", "滚动 ↑↓/jk  执行 Enter  返回 Esc")
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	h := m.states[m.activeCommand.HostIndex].Host
	lines := []string{
		modalLine(m.t("Server", "服务器"), hostDisplayName(h), width-4),
		modalLine(m.t("Template", "模板"), m.commandConfirm.Name, width-4),
		"",
		detailSubTitle(m.t("Command", "命令")),
	}
	lines = append(lines, strings.Split(wrapPlainLine(m.commandConfirm.Command, width-4), "\n")...)
	maxScroll := m.commandConfirmMaxScroll()
	scroll := clampInt(m.commandOutputScroll, 0, maxScroll)
	viewLines := lines
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(yellow).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(viewLines, width-4), "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(m.t("Run Command", "即将执行"), width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) commandConfirmMaxScroll() int {
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	lines := []string{
		"",
		"",
		"",
		m.t("Command", "命令"),
	}
	lines = append(lines, strings.Split(wrapPlainLine(m.commandConfirm.Command, detailFrameWidth(m.width)-4), "\n")...)
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func modalLine(label string, value string, width int) string {
	labelWidth := 8
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+value, width)
}

func (m Model) renderCommandOutput() string {
	width := detailFrameWidth(m.width)
	help := m.t("Scroll ↑↓/jk  Back q/Esc", "滚动 ↑↓/jk  返回 q/Esc")
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	title := m.t("Command Output  ", "命令输出  ") + m.activeCommand.Name
	lines := []string{"$ " + m.activeCommand.Command, ""}
	if m.activeCommand.Running {
		lines = append(lines, m.t("Running...", "正在执行..."))
	} else {
		output := strings.TrimRight(m.activeCommand.Output, "\n")
		if output == "" {
			output = m.t("(no output)", "(无输出)")
		}
		lines = append(lines, strings.Split(output, "\n")...)
		lines = append(lines, "", fmt.Sprintf("%s %d", m.t("Exit code", "退出码"), m.activeCommand.ExitCode))
	}
	viewLines := lines
	maxScroll := m.commandOutputMaxScroll()
	scroll := clampInt(m.commandOutputScroll, 0, maxScroll)
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(viewLines, width-4), "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}
