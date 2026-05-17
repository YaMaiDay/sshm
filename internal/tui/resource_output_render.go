package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) renderResourceLog() string {
	width := detailFrameWidth(m.width)
	bodyHeight := m.resourceLogBodyHeight()
	lines := m.resourceLogLines()
	start, end := resourceLogWindowRange(len(lines), m.resourceState.LogScroll, bodyHeight)
	bodyLines := fitLines(lines[start:end], width-4)
	for len(bodyLines) < bodyHeight {
		bodyLines = append(bodyLines, "")
	}
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Height(bodyHeight).
		Render(strings.Join(bodyLines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fitANSI(m.resourceLogHeader(len(lines), start, end), width)),
		body,
		renderHelp(width, m.t("Scroll ↑↓/jk  Refresh r  Back Esc", "滚动 ↑↓/jk  刷新 r  返回 Esc")),
	}, "\n")
}

func (m Model) resourceLogBodyHeight() int {
	// Header, bordered body, and help consume four terminal rows before log content.
	return maxInt(1, m.height-4)
}

func (m Model) resourceLogLines() []string {
	lines := strings.Split(strings.TrimRight(m.resourceState.LogOutput, "\n"), "\n")
	if len(lines) == 0 || len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return []string{m.t("No log output.", "没有日志输出。")}
	}
	return lines
}

func (m Model) resourceLogMaxScroll() int {
	return maxInt(0, len(m.resourceLogLines())-m.resourceLogBodyHeight())
}

func resourceLogWindowRange(total int, scroll int, height int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if height <= 0 {
		height = 1
	}
	start := clampInt(scroll, 0, maxInt(0, total-height))
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}

func (m Model) resourceLogHeader(total int, start int, end int) string {
	rangeText := fmt.Sprintf("%d-%d/%d", start+1, end, total)
	if total == 0 {
		rangeText = "0/0"
	}
	return strings.Join([]string{
		m.t("Resources", "资源"),
		">",
		m.t("Logs", "日志"),
		m.resourceHostTitle(),
		m.resourceKindName(m.resourceState.LogKind),
		m.resourceState.LogName,
		mutedStyle.Render(rangeText),
	}, "  ")
}

func (m Model) renderResourceCommandEdit() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 40 {
		bodyWidth = 40
	}
	lines := []string{
		detailSubTitle(m.resourceCommandEditHeading()),
		m.detailRow(m.t("Resource", "资源"), m.resourceState.CommandForm.Name),
		m.detailRow(m.t("Type", "类型"), m.resourceKindName(m.resourceState.CommandForm.Kind)),
		"",
	}
	if resourceCommandFieldCount(m.resourceState.CommandForm.Kind) == 0 {
		lines = append(lines, m.detailRow(m.t("Mode", "模式"), m.t("Read-only resource", "只读资源")))
	}
	for i := 0; i < resourceCommandFieldCount(m.resourceState.CommandForm.Kind); i++ {
		lines = append(lines, m.resourceCommandFieldLine(i, bodyWidth))
	}
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	start, end := visibleRange(len(lines), maxInt(0, m.resourceState.CommandField+3), bodyHeight)
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines[start:end], "\n"))
	help := renderHelp(width, m.t("Save Enter  Move Tab/↑↓  Cursor ←→  Back Esc", "保存 Enter  移动 Tab/↑↓  光标 ←→  返回 Esc"))
	return strings.Join([]string{titleStyle.Render(fitANSI(m.resourceCommandEditTitle(), width)), body, help}, "\n")
}

func (m Model) resourceCommandEditHeading() string {
	if m.resourceState.CommandForm.Kind == resourceDatabases {
		return m.t("Database Connection", "数据库连接")
	}
	return m.t("Resource Commands", "资源命令")
}

func (m Model) resourceCommandEditTitle() string {
	if m.resourceState.CommandForm.Kind == resourceDatabases {
		return m.t("Configure Database", "配置数据库")
	}
	return m.t("Edit Resource Commands", "编辑资源命令")
}

func (m Model) resourceCommandFieldLine(field int, width int) string {
	label := m.resourceCommandFieldName(field)
	value := m.resourceCommandFieldValue(field)
	inputWidth := width - 18
	if inputWidth < 18 {
		inputWidth = 18
	}
	display := commandInputText(value, m.resourceState.CommandCursor, m.resourceState.CommandField == field, inputWidth)
	if m.resourceState.CommandForm.Kind == resourceDatabases && field == 0 {
		display = m.resourceCommandDatabaseEngineDisplay(inputWidth)
	}
	prefix := "  "
	style := detailValueStyle
	if m.resourceState.CommandField == field {
		prefix = "▶ "
		style = blueStyle.Bold(true)
	}
	return fitANSI(prefix+style.Render(padVisible(label, 12))+"  "+display, width)
}

func (m Model) resourceCommandDatabaseEngineDisplay(width int) string {
	value := resourceservice.NormalizeDatabaseEngine(m.resourceState.CommandForm.DBEngine)
	if value == "" {
		value = "MySQL"
	}
	text := value
	if m.resourceState.CommandField == 0 {
		text = "← " + text + " →"
	}
	return fitANSI(text, width)
}

func (m Model) resourceCommandFieldName(field int) string {
	if m.resourceState.CommandForm.Kind == resourcePorts {
		return m.t("Health", "健康检查")
	}
	if m.resourceState.CommandForm.Kind == resourceDatabases {
		switch field {
		case 0:
			return m.t("Engine", "数据库")
		case 1:
			return m.t("Host", "地址")
		case 2:
			return m.t("Port", "端口")
		case 3:
			return m.t("User", "用户")
		case 4:
			return m.t("Password", "密码")
		case 5:
			return m.t("Database", "库名")
		case 6:
			return m.t("Note", "备注")
		default:
			return "-"
		}
	}
	switch field {
	case 0:
		return m.t("Start", "启动")
	case 1:
		return m.t("Stop", "停止")
	case 2:
		return m.t("Restart", "重启")
	case 3:
		return m.t("Logs", "日志")
	default:
		return "-"
	}
}

func (m Model) renderResourceConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	command := resourceActionCommandPreview(m.resourceState.ActionResource, m.resourceState.Action, m.resourceState.ActionName)
	lines := []string{
		m.t("Server: ", "服务器：") + m.resourceHostTitle(),
		m.resourceKindName(m.resourceState.ActionResource) + ": " + m.resourceState.ActionName,
		m.t("Action: ", "操作：") + m.resourceActionNameText(m.resourceState.Action),
		m.t("Command: ", "命令：") + command,
	}
	wrapped := []string{}
	for _, line := range lines {
		wrapped = append(wrapped, wrapPlainLine(line, bodyWidth))
	}
	return m.renderDangerConfirm(m.t("Confirm Resource Action", "确认资源操作"), wrapped, width)
}

func (m Model) renderResourceOutput() string {
	width := detailFrameWidth(m.width)
	bodyHeight := m.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	lines := m.resourceOutputLines()
	start, end := visibleRange(len(lines), m.resourceState.Scroll, bodyHeight)
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines[start:end], "\n"))
	return strings.Join([]string{titleStyle.Render(fitANSI(m.t("Resource Output", "资源操作输出"), width)), body, renderHelp(width, m.t("Scroll ↑↓/jk  Retry r  Back Esc", "滚动 ↑↓/jk  重试 r  返回 Esc"))}, "\n")
}

func (m Model) resourceOutputLines() []string {
	lines := []string{
		"$ " + m.resourceCommandPreview(m.resourceState.ActionResource, m.resourceState.Action, m.resourceState.ActionName),
		"",
	}
	if m.resourceState.ActionRunning {
		lines = append(lines, m.t("Running...", "执行中..."))
	} else {
		if strings.TrimSpace(m.resourceState.ActionOutput) != "" {
			lines = append(lines, strings.Split(m.resourceState.ActionOutput, "\n")...)
		}
		lines = append(lines, "", fmt.Sprintf("%s %d", m.t("Exit code", "退出码"), m.resourceState.ActionExitCode))
	}
	return lines
}

func (m Model) resourceOutputMaxScroll() int {
	bodyHeight := maxInt(1, m.height-3)
	return maxInt(0, len(m.resourceOutputLines())-bodyHeight)
}
