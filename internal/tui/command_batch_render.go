package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderBatchSelect() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := m.t("Move ↑↓/jk  Select Space  All a  Clear x  Next Enter  Back Esc", "移动 ↑↓/jk  选择 Space  全选 a  清空 x  下一步 Enter  返回 Esc")
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	contentHeight := bodyHeight - 2
	if contentHeight < 3 {
		contentHeight = 3
	}
	lines := []string{}
	if len(m.batchState.Indexes) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No selectable servers", "没有可选择的服务器")))
	} else {
		start, end := visibleRange(len(m.batchState.Indexes), m.batchState.Cursor, contentHeight)
		for i := start; i < end; i++ {
			index := m.batchState.Indexes[i]
			h := m.states[index].Host
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.batchState.Cursor {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			mark := "[ ]"
			if m.batchState.Selected[index] {
				mark = "[x]"
			}
			lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s %s", prefix, mark, hostDisplayName(h)), bodyWidth)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("%s  %s %d", m.t("Batch Select Servers", "批量选择服务器"), m.t("Selected", "已选"), m.batchSelectedCount())
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) renderBatchCommandList() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := m.t("Move ↑↓/jk  Select Enter  Back Esc", "移动 ↑↓/jk  选择 Enter  返回 Esc")
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	targets := m.batchTargetsHeader(width)
	targetLines := strings.Count(targets, "\n") + 1
	contentHeight := bodyHeight - 2 - targetLines
	if contentHeight < 3 {
		contentHeight = 3
	}
	lines := []string{}
	start, end := visibleRange(len(m.batchState.CommandItems), m.batchState.CommandIndex, contentHeight)
	for i := start; i < end; i++ {
		item := m.batchState.CommandItems[i]
		if item.Header {
			lines = append(lines, detailSubTitle(m.commandDisplayName(item.Name)))
			continue
		}
		if item.Spacer {
			lines = append(lines, "")
			continue
		}
		prefix := " "
		style := lipgloss.NewStyle()
		if i == m.batchState.CommandIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		label := m.commandDisplayName(item.Name)
		if item.Temporary {
			label = "+ " + label
		}
		lines = append(lines, style.Render(fit(prefix+" "+label, bodyWidth)))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("%s  %d %s", m.t("Select Batch Command", "选择批量命令"), m.batchSelectedCount(), m.t("servers", "台服务器"))
	return strings.Join([]string{titleStyle.Render(fit(title, width)), targets, box, renderHelp(width, help)}, "\n")
}

func (m Model) renderBatchCommandEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 36 {
		innerWidth = 36
	}
	help := m.t("Save Enter  Newline Ctrl+J  Back Esc", "保存 Enter  换行 Ctrl+J  返回 Esc")
	targets := m.batchTargetsHeader(width)
	targetLines := strings.Count(targets, "\n") + 1
	lines := []string{detailSubTitle(m.t("Command", "命令内容"))}
	lines = append(lines, commandTextArea(m.commandState.Form.Command, m.commandState.Cursor, true, innerWidth, m.batchCommandTextAreaHeight(targetLines)))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(blue).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{titleStyle.Render(fit(m.t("Batch Temporary Command", "批量临时命令"), width)), targets, box, renderHelp(width, help)}, "\n")
}

func (m Model) batchCommandTextAreaHeight(targetLines int) int {
	height := m.height - targetLines - 7
	if height < 4 {
		height = 4
	}
	return height
}

func (m Model) batchTargetsHeader(width int) string {
	names := make([]string, 0, m.batchSelectedCount())
	for _, index := range m.selectedBatchHostIndexes() {
		if index >= 0 && index < len(m.states) {
			names = append(names, hostDisplayName(m.states[index].Host))
		}
	}
	if len(names) == 0 {
		return mutedStyle.Render(m.t("Targets", "目标") + " -")
	}
	sep := ", "
	if m.isChineseUI() {
		sep = "、"
	}
	return mutedStyle.Render(wrapPlainLine(m.t("Targets ", "目标 ")+strings.Join(names, sep), width))
}

func (m Model) renderBatchConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{
		modalLine(m.t("Servers", "服务器"), fmt.Sprintf("%d%s", m.batchSelectedCount(), m.t(" servers", "台")), bodyWidth),
		modalLine(m.t("Template", "模板"), m.batchState.Command.Name, bodyWidth),
		"",
		detailSubTitle(m.t("Targets", "目标")),
	}
	for _, index := range m.selectedBatchHostIndexes() {
		lines = append(lines, fit("- "+hostDisplayName(m.states[index].Host), bodyWidth))
	}
	lines = append(lines, "", detailSubTitle(m.t("Command", "命令")))
	lines = append(lines, strings.Split(wrapPlainLine(m.batchState.Command.Command, bodyWidth), "\n")...)
	scroll := clampInt(m.batchState.OutputScroll, 0, m.batchConfirmMaxScroll())
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(yellow).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{titleStyle.Render(fit(m.t("Confirm Batch Run", "确认批量执行"), width)), box, renderHelp(width, m.t("Scroll ↑↓/jk  Confirm Enter  Back Esc", "滚动 ↑↓/jk  确认 Enter  返回 Esc"))}, "\n")
}

func (m Model) batchConfirmMaxScroll() int {
	lines := 5 + len(m.selectedBatchHostIndexes()) + len(wrapDetailValue(m.batchState.Command.Command, detailFrameWidth(m.width)-4))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) renderBatchOutput() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	leftWidth := bodyWidth / 2
	if leftWidth < 28 {
		leftWidth = 28
	}
	rightWidth := bodyWidth - leftWidth - 2
	if rightWidth < 24 {
		rightWidth = 24
	}
	left := m.batchResultList(leftWidth)
	right := m.batchOutputView(rightWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(body)
	title := fmt.Sprintf("%s  %s%d  %s%d", m.t("Batch Results", "批量执行结果"), m.t("Success", "成功"), m.batchSuccessCount(), m.t("Failed", "失败"), m.batchFailCount())
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, m.t("Select ↑↓/jk  Output ←→/hl  Retry failed r  Back q/Esc", "选择 ↑↓/jk  输出 ←→/hl  重试失败 r  返回 q/Esc"))}, "\n")
}

func (m Model) batchResultList(width int) string {
	lines := make([]string, 0, len(m.batchState.Jobs)+4)
	displayIndexes := m.batchResultDisplayIndexes()
	lastGroup := ""
	for _, i := range displayIndexes {
		if i < 0 || i >= len(m.batchState.Jobs) {
			continue
		}
		job := m.batchState.Jobs[i]
		group := batchJobGroup(job)
		if group != lastGroup {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.batchJobGroupTitle(group))
			lastGroup = group
		}
		prefix := " "
		style := lipgloss.NewStyle()
		if i == m.batchState.OutputIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		state := m.t("Waiting", "等待")
		if job.Running {
			state = m.t("Running", "执行中")
		} else if job.Done && job.Err == nil {
			state = greenStyle.Render(m.t("Success", "成功"))
		} else if job.Done && job.Err != nil {
			state = redStyle.Render(m.t("Failed", "失败"))
		}
		name := "-"
		if job.HostIndex >= 0 && job.HostIndex < len(m.states) {
			name = hostDisplayName(m.states[job.HostIndex].Host)
		}
		lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s  %s", prefix, state, name), width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) batchResultDisplayIndexes() []int {
	groups := []string{"failed", "running", "waiting", "success"}
	indexes := make([]int, 0, len(m.batchState.Jobs))
	for _, group := range groups {
		for i, job := range m.batchState.Jobs {
			if batchJobGroup(job) == group {
				indexes = append(indexes, i)
			}
		}
	}
	return indexes
}

func batchJobGroup(job batchJob) string {
	switch {
	case job.Done && job.Err != nil:
		return "failed"
	case job.Running:
		return "running"
	case !job.Done:
		return "waiting"
	default:
		return "success"
	}
}

func (m Model) batchJobGroupTitle(group string) string {
	switch group {
	case "failed":
		return detailDangerSubTitle(m.t("Failed", "失败"))
	case "running":
		return detailSubTitle(m.t("Running", "执行中"))
	case "waiting":
		return detailSubTitle(m.t("Waiting", "等待"))
	default:
		return detailSuccessSubTitle(m.t("Success", "成功"))
	}
}

func (m Model) batchOutputView(width int) string {
	if len(m.batchState.Jobs) == 0 || m.batchState.OutputIndex < 0 || m.batchState.OutputIndex >= len(m.batchState.Jobs) {
		return ""
	}
	job := m.batchState.Jobs[m.batchState.OutputIndex]
	lines := []string{}
	if job.Running {
		lines = append(lines, m.t("Running...", "执行中..."))
	} else if !job.Done {
		lines = append(lines, m.t("Waiting to run", "等待执行"))
	} else {
		output := strings.TrimRight(job.Output, "\n")
		if output == "" {
			output = m.t("(no output)", "(无输出)")
		}
		lines = append(lines, strings.Split(output, "\n")...)
		lines = append(lines, "", fmt.Sprintf("%s %d", m.t("Exit code", "退出码"), job.ExitCode))
	}
	scroll := clampInt(m.batchState.OutputScroll, 0, m.batchOutputMaxScroll())
	height := m.height - 6
	if height < 6 {
		height = 6
	}
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	return strings.Join(fitLines(lines, width), "\n")
}

func (m Model) batchOutputMaxScroll() int {
	if len(m.batchState.Jobs) == 0 || m.batchState.OutputIndex < 0 || m.batchState.OutputIndex >= len(m.batchState.Jobs) {
		return 0
	}
	job := m.batchState.Jobs[m.batchState.OutputIndex]
	lines := 1
	if job.Done {
		if output := strings.TrimRight(job.Output, "\n"); output != "" {
			lines = len(strings.Split(output, "\n")) + 2
		} else {
			lines = 3
		}
	}
	height := m.height - 6
	if height < 6 {
		height = 6
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}
