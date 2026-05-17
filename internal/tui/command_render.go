package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderCommandList() string {
	width := detailFrameWidth(m.width)
	hostName := "-"
	if m.commandState.Active.HostIndex >= 0 && m.commandState.Active.HostIndex < len(m.states) {
		hostName = hostDisplayName(m.states[m.commandState.Active.HostIndex].Host)
	}
	title := m.t("Command Templates  ", "命令模板  ") + hostName
	bodyWidth := width - 4
	if bodyWidth < 30 {
		bodyWidth = 30
	}
	help := m.t("Move ↑↓/jk  Run Enter  Add a  Edit e  Delete x  Back Esc", "选择 ↑↓/jk  执行 Enter  新增 a  编辑 e  删除 x  返回 Esc")
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	contentHeight := bodyHeight - 2
	if contentHeight < 4 {
		contentHeight = 4
	}
	lines := []string{}
	listHeight := contentHeight
	if listHeight < 1 {
		listHeight = 1
	}
	if len(m.commandState.Items) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No command templates", "没有命令模板")))
	} else {
		start, end := visibleRange(len(m.commandState.Items), m.commandState.Index, listHeight)
		for i := start; i < end; i++ {
			item := m.commandState.Items[i]
			if item.Header {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, detailSubTitle(m.commandDisplayName(item.Name)))
				continue
			}
			if item.Spacer {
				lines = append(lines, "")
				continue
			}
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.commandState.Index {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			label := m.commandDisplayName(item.Name)
			if item.Temporary {
				label = "+ " + label
			}
			lines = append(lines, style.Render(fit(prefix+" "+label, bodyWidth)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}
