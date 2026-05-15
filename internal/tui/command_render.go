package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderCommandList() string {
	width := detailFrameWidth(m.width)
	hostName := "-"
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		hostName = hostDisplayName(m.states[m.activeCommand.HostIndex].Host)
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
	if len(m.commandItems) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No command templates", "没有命令模板")))
	} else {
		start, end := visibleRange(len(m.commandItems), m.commandIndex, listHeight)
		for i := start; i < end; i++ {
			item := m.commandItems[i]
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
			if i == m.commandIndex {
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
	if len(m.batchIndexes) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No selectable servers", "没有可选择的服务器")))
	} else {
		start, end := visibleRange(len(m.batchIndexes), m.batchCursor, contentHeight)
		for i := start; i < end; i++ {
			index := m.batchIndexes[i]
			h := m.states[index].Host
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.batchCursor {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			mark := "[ ]"
			if m.batchSelected[index] {
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
	start, end := visibleRange(len(m.batchCommandItems), m.batchCommandIndex, contentHeight)
	for i := start; i < end; i++ {
		item := m.batchCommandItems[i]
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
		if i == m.batchCommandIndex {
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
	lines = append(lines, commandTextArea(m.commandForm.Command, m.commandCursor, true, innerWidth, m.batchCommandTextAreaHeight(targetLines)))
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
		modalLine(m.t("Template", "模板"), m.batchCommand.Name, bodyWidth),
		"",
		detailSubTitle(m.t("Targets", "目标")),
	}
	for _, index := range m.selectedBatchHostIndexes() {
		lines = append(lines, fit("- "+hostDisplayName(m.states[index].Host), bodyWidth))
	}
	lines = append(lines, "", detailSubTitle(m.t("Command", "命令")))
	lines = append(lines, strings.Split(wrapPlainLine(m.batchCommand.Command, bodyWidth), "\n")...)
	scroll := clampInt(m.batchOutputScroll, 0, m.batchConfirmMaxScroll())
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
	lines := 5 + len(m.selectedBatchHostIndexes()) + len(wrapDetailValue(m.batchCommand.Command, detailFrameWidth(m.width)-4))
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
	lines := make([]string, 0, len(m.batchJobs)+4)
	displayIndexes := m.batchResultDisplayIndexes()
	lastGroup := ""
	for _, i := range displayIndexes {
		if i < 0 || i >= len(m.batchJobs) {
			continue
		}
		job := m.batchJobs[i]
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
		if i == m.batchOutputIndex {
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
	indexes := make([]int, 0, len(m.batchJobs))
	for _, group := range groups {
		for i, job := range m.batchJobs {
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
	if len(m.batchJobs) == 0 || m.batchOutputIndex < 0 || m.batchOutputIndex >= len(m.batchJobs) {
		return ""
	}
	job := m.batchJobs[m.batchOutputIndex]
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
	scroll := clampInt(m.batchOutputScroll, 0, m.batchOutputMaxScroll())
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
	if len(m.batchJobs) == 0 || m.batchOutputIndex < 0 || m.batchOutputIndex >= len(m.batchJobs) {
		return 0
	}
	job := m.batchJobs[m.batchOutputIndex]
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

func (m Model) renderCommandHistory() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := m.t("Move ↑↓/jk  View Enter  Search /  Rerun r  Delete x  Back q/Esc", "移动 ↑↓/jk  查看 Enter  搜索 /  重跑 r  删除 x  返回 q/Esc")
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	lines := []string{}
	entries := m.filteredHistoryEntries()
	if len(entries) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No command history", "暂无命令历史")))
	} else {
		start, end := visibleRange(len(entries), m.historyIndex, height)
		for i := start; i < end; i++ {
			entry := entries[i]
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.historyIndex {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			status := m.historyStatusText(entry.Status)
			line := fmt.Sprintf("%s %s  %s  %s  %s", prefix, historyTimeShort(entry.Time), status, m.historyTargetsText(entry, 1), m.historyCommandName(entry))
			lines = append(lines, style.Render(fit(line, bodyWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("%s  %d%s", m.t("Command History", "命令历史"), len(entries), m.t(" records", "条"))
	if m.historySearch {
		title += "  " + m.t("Search: ", "搜索：") + inlineCursorText(m.historyQuery, width/3, len([]rune(m.historyQuery)))
	} else if strings.TrimSpace(m.historyQuery) != "" {
		title += "  " + m.t("Search: ", "搜索：") + m.historyQuery
	}
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) renderCommandHistoryDetail() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return m.t("No command history", "没有命令历史")
	}
	lines := []string{
		modalLine(m.t("Time", "时间"), historyTimeFull(entry.Time), bodyWidth),
		modalLine(m.t("Status", "状态"), m.historyStatusPlain(entry.Status), bodyWidth),
		modalLine(m.t("Type", "类型"), m.historyKindText(entry), bodyWidth),
		modalLine(m.t("Name", "名称"), m.historyCommandName(entry), bodyWidth),
		"",
		detailSubTitle(m.t("Targets", "目标")),
	}
	for _, target := range entry.Targets {
		state := m.historyStatusPlain(target.Status)
		targetText := fmt.Sprintf("%s  %s  %s%d", m.historyTargetName(target), state, m.t("exit ", "退出码"), target.ExitCode)
		lines = append(lines, fit(targetText, bodyWidth))
	}
	lines = append(lines, "", detailSubTitle(m.t("Command", "命令")))
	lines = append(lines, strings.Split(wrapPlainLine(entry.Command, bodyWidth), "\n")...)
	lines = append(lines, "", detailSubTitle(m.t("Output", "输出")))
	for _, target := range entry.Targets {
		lines = append(lines, fit("["+m.historyTargetName(target)+"]", bodyWidth))
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			output = m.t("(no output)", "(无输出)")
		}
		lines = append(lines, strings.Split(wrapPlainLine(output, bodyWidth), "\n")...)
		lines = append(lines, "")
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.historyScroll, 0, m.commandHistoryDetailMaxScroll())
	viewLines := lines
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(fitLines(viewLines, bodyWidth), "\n"))
	help := m.t("Scroll ↑↓/jk  Rerun r  Delete x  Back q/Esc", "滚动 ↑↓/jk  重跑 r  删除 x  返回 q/Esc")
	return strings.Join([]string{titleStyle.Render(fit(m.t("Command History Detail", "命令历史详情"), width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) commandHistoryDetailMaxScroll() int {
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return 0
	}
	bodyWidth := detailFrameWidth(m.width) - 4
	lines := 9 + len(entry.Targets)*3 + len(wrapDetailValue(entry.Command, bodyWidth))
	for _, target := range entry.Targets {
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(wrapDetailValue(output, bodyWidth))
		}
	}
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

func historyCommandName(entry config.CommandHistoryEntry) string {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return "临时命令"
	}
	return name
}

func (m Model) historyCommandName(entry config.CommandHistoryEntry) string {
	return m.commandDisplayName(historyCommandName(entry))
}

func (m Model) commandDisplayName(name string) string {
	if m.isChineseUI() {
		return name
	}
	switch strings.TrimSpace(name) {
	case "当前服务器":
		return "Current Server"
	case "全局":
		return "Global"
	case "临时命令":
		return "Temporary Command"
	case "学习样板-部署检查":
		return "Example - Deploy Check"
	case "学习样板-部署发布":
		return "Example - Deploy Release"
	case "学习样板-日志排查":
		return "Example - Log Troubleshooting"
	case "学习样板-Docker清理":
		return "Example - Docker Cleanup"
	default:
		return name
	}
}

func (m Model) historyKindText(entry config.CommandHistoryEntry) string {
	if entry.Kind == "batch" {
		if m.isChineseUI() {
			return fmt.Sprintf("批量命令 %d台", len(entry.Targets))
		}
		return fmt.Sprintf("Batch command %d targets", len(entry.Targets))
	}
	if m.isChineseUI() {
		return "单台命令"
	}
	return "Single command"
}

func (m Model) historyStatusText(status string) string {
	if status == "failed" {
		return redStyle.Render(m.t("Failed", "失败"))
	}
	return greenStyle.Render(m.t("Success", "成功"))
}

func (m Model) historyStatusPlain(status string) string {
	if status == "failed" {
		return m.t("Failed", "失败")
	}
	return m.t("Success", "成功")
}

func historyTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "--"
	}
	return t.Local().Format("01-02 15:04")
}

func historyTimeFull(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return value
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func (m Model) historyTargetsText(entry config.CommandHistoryEntry, limit int) string {
	if len(entry.Targets) == 0 {
		return "-"
	}
	names := make([]string, 0, len(entry.Targets))
	for _, target := range entry.Targets {
		names = append(names, m.historyTargetName(target))
	}
	if limit > 0 && len(names) > limit {
		return fmt.Sprintf("%s %s%d", names[0], m.t("and ", "等"), len(names))
	}
	if m.isChineseUI() {
		return strings.Join(names, "、")
	}
	return strings.Join(names, ", ")
}

func (m Model) historyTargetName(target config.CommandHistoryTarget) string {
	category := strings.TrimSpace(target.Category)
	name := strings.TrimSpace(target.Name)
	if category == "" {
		return name
	}
	return "[" + m.displayCategoryName(category) + "] " + name
}

func (m Model) renderHelpPanel() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	rows := []struct {
		key  string
		desc string
	}{
		{"↑↓←→ / hjkl", m.t("Move selection", "移动选择")},
		{"Enter", m.t("Login server", "登录服务器")},
		{"Space", m.t("View details", "查看详情")},
		{"m", m.t("Command templates", "命令模板")},
		{"b", m.t("Batch commands", "批量命令")},
		{"i", m.t("Command history", "命令历史")},
		{"y", m.t("Transfer jobs", "传输任务")},
		{"g", m.t("App deployment", "应用部署")},
		{"n", m.t("Container and service resources", "容器和服务资源")},
		{".", m.t("Settings", "设置")},
		{"w", m.t("Problem overview", "异常总览")},
		{"z", m.t("Switch dashboard view", "切换首页视图")},
		{"t", m.t("Pin / unpin", "置顶 / 取消置顶")},
		{"f", m.t("Favorite / unfavorite", "收藏 / 取消收藏")},
		{"v", m.t("Favorites only / clear filter", "只看收藏 / 取消筛选")},
		{"a", m.t("Add server", "添加服务器")},
		{"c", m.t("Copy server", "复制服务器")},
		{"e", m.t("Edit server", "编辑服务器")},
		{"x", m.t("Delete server", "删除服务器")},
		{"u", m.t("Upload file or directory", "上传文件或目录")},
		{"d", m.t("Download file or directory", "下载文件或目录")},
		{"r", m.t("Refresh monitoring", "刷新监控")},
		{"/", m.t("Search", "搜索")},
		{"Tab", m.t("Switch category", "切换分类")},
		{"o", m.t("Online only / clear filter", "只看在线 / 取消筛选")},
		{"p", m.t("Problems only / clear filter", "只看异常 / 取消筛选")},
		{"s", m.t("Switch sort", "切换排序")},
		{"q / Esc", m.t("Quit or go back", "退出或返回")},
		{"?", m.t("Close help", "关闭帮助")},
	}
	lines := []string{}
	for _, row := range rows {
		lines = append(lines, modalLine(row.key, row.desc, bodyWidth))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(m.t("Shortcuts", "快捷键"), width)),
		box,
		renderHelp(width, m.t("Back q/Esc/?", "返回 q/Esc/?")),
	}, "\n")
}

type anomalyItem struct {
	Index  int
	Checks []checkItem
}

func (m Model) anomalyItems() []anomalyItem {
	items := make([]anomalyItem, 0)
	for i, state := range m.states {
		checks := actionableChecks(m.buildChecks(state))
		if len(checks) == 0 {
			continue
		}
		if !m.anomalyMatchesFilter(checks) {
			continue
		}
		items = append(items, anomalyItem{Index: i, Checks: checks})
	}
	sort.SliceStable(items, func(i, j int) bool {
		aSevere, aWarn, aTip := checkCounts(items[i].Checks)
		bSevere, bWarn, bTip := checkCounts(items[j].Checks)
		if aSevere != bSevere {
			return aSevere > bSevere
		}
		if aWarn != bWarn {
			return aWarn > bWarn
		}
		if aTip != bTip {
			return aTip > bTip
		}
		aHost := m.states[items[i].Index].Host
		bHost := m.states[items[j].Index].Host
		if aHost.Category == bHost.Category {
			return aHost.Name < bHost.Name
		}
		return aHost.Category < bHost.Category
	})
	return items
}

func (m Model) anomalyMatchesFilter(checks []checkItem) bool {
	switch m.anomalyFilter {
	case anomalySevere:
		for _, check := range checks {
			if check.Level == "严重" {
				return true
			}
		}
		return false
	case anomalyWarn:
		for _, check := range checks {
			if check.Level == "警告" {
				return true
			}
		}
		return false
	case anomalyOffline:
		return checksContainKind(checks, "offline")
	case anomalyResource:
		return checksContainKind(checks, "resource")
	case anomalyContainer:
		return checksContainKind(checks, "container")
	case anomalyService:
		return checksContainKind(checks, "service")
	case anomalySecurity:
		return checksContainKind(checks, "security")
	default:
		return true
	}
}

func checksContainKind(checks []checkItem, kind string) bool {
	for _, check := range checks {
		if checkKind(check) == kind {
			return true
		}
	}
	return false
}

func actionableChecks(checks []checkItem) []checkItem {
	out := make([]checkItem, 0, len(checks))
	for _, check := range checks {
		if check.Level == "严重" || check.Level == "警告" {
			out = append(out, check)
		}
	}
	return out
}

func checkCounts(checks []checkItem) (int, int, int) {
	severe := 0
	warn := 0
	tip := 0
	for _, check := range checks {
		switch check.Level {
		case "严重":
			severe++
		case "警告":
			warn++
		case "提示":
			tip++
		}
	}
	return severe, warn, tip
}
