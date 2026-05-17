package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderTransferJobs() string {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 34 {
		width = 34
	}
	help := m.renderTransferJobsHelp(width)
	reservedBottomLines := strings.Count(help, "\n") + 1
	counts := transferStatusCounts(m.transferHistory.Entries)
	filtered := m.filteredTransferIndexes()
	header := fmt.Sprintf("%s  %s %s  %s %d/%d  %s %d  %s %d  %s %d",
		m.t("Transfer Jobs", "传输任务"),
		m.t("Status", "状态"), m.transferStatusFilterName(),
		m.t("Showing", "显示"), len(filtered), len(m.transferHistory.Entries),
		m.t("Running", "运行"), counts[config.TransferStatusRunning],
		m.t("Open", "未完成"), transferUnfinishedCount(m.transferHistory.Entries),
		m.t("Done", "已完成"), counts[config.TransferStatusDone])
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.transferHistory.Entries) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No transfer records", "暂无传输记录")))
	} else if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No transfer jobs for this status", "当前状态没有传输任务")))
	} else {
		bodyLines := m.height - reservedBottomLines - 2
		if bodyLines < 1 {
			bodyLines = 1
		}
		cardLines, selectedTop, selectedBottom := m.transferJobGridLines(width)
		start, end := dashboardLineWindow(len(cardLines), selectedTop, selectedBottom, bodyLines)
		lines = append(lines, cardLines[start:end]...)
	}
	lines = padToBottom(lines, m.height, reservedBottomLines)
	lines = append(lines, help)
	return strings.Join(lines, "\n")
}

func (m Model) renderTransferJobsHelp(width int) string {
	if width < 1 {
		width = 1
	}
	help := strings.Join([]string{
		m.t("Status Tab", "状态 Tab"),
		m.t("Move ↑↓←→/hjkl", "移动 ↑↓←→/hjkl"),
		m.t("Start Enter", "开始 Enter"),
		m.t("Detail Space", "详情 Space"),
		m.t("Start all a", "全部开始 a"),
		m.t("Pause all p", "全部暂停 p"),
		m.t("Cancel c", "取消 c"),
		m.t("Delete x", "删除 x"),
		m.t("Back Esc", "返回 Esc"),
	}, "  ")
	return helpStyle.Render(fit(help, width))
}

func transferStatusFilterOptions() []string {
	return []string{
		"",
		config.TransferStatusQueued,
		config.TransferStatusPending,
		config.TransferStatusRunning,
		config.TransferStatusDone,
		config.TransferStatusFailed,
		config.TransferStatusCanceled,
		config.TransferStatusInterrupted,
	}
}

func (m Model) transferStatusFilterValue() string {
	options := transferStatusFilterOptions()
	if m.transferStatusFilter < 0 || m.transferStatusFilter >= len(options) {
		return ""
	}
	return options[m.transferStatusFilter]
}

func (m Model) transferStatusFilterName() string {
	status := m.transferStatusFilterValue()
	if status == "" {
		return m.t("All", "全部")
	}
	return m.transferStatusText(status)
}

func (m Model) filteredTransferIndexes() []int {
	status := m.transferStatusFilterValue()
	indexes := make([]int, 0, len(m.transferHistory.Entries))
	for i, entry := range m.transferHistory.Entries {
		if status == "" || entry.Status == status {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m Model) transferJobGridLines(width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	indexes := m.filteredTransferIndexes()
	for i := 0; i < len(indexes); i += cols {
		rowEnd := i + cols
		if rowEnd > len(indexes) {
			rowEnd = len(indexes)
		}
		rowBlocks := make([]string, cols)
		rowHeight := 0
		rowHasError := false
		for j := i; j < rowEnd; j++ {
			entry := m.transferHistory.Entries[indexes[j]]
			if strings.TrimSpace(entry.Error) != "" {
				rowHasError = true
				break
			}
		}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			visibleIndex := i + col
			if visibleIndex >= rowEnd {
				continue
			}
			entryIndex := indexes[visibleIndex]
			if entryIndex == m.transferIndex {
				selectedTop = len(lines)
			}
			block := m.renderTransferJobCard(m.transferHistory.Entries[entryIndex], cardWidth, entryIndex == m.transferIndex, rowHasError)
			rowBlocks[col] = block
			if height := blockLineCount(block); height > rowHeight {
				rowHeight = height
			}
		}
		if rowHeight == 0 {
			continue
		}
		for col := 0; col < cols; col++ {
			if rowBlocks[col] == "" {
				rowBlocks[col] = blankTransferJobBlock(cardWidths[col], rowHeight)
			} else {
				rowBlocks[col] = padBlockHeight(rowBlocks[col], cardWidths[col], rowHeight)
			}
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, rowBlocks...), "\n")
		lines = append(lines, rowLines...)
		for _, index := range indexes[i:rowEnd] {
			if index == m.transferIndex {
				selectedBottom = len(lines)
				break
			}
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func blockLineCount(block string) int {
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

func blankTransferJobBlock(width int, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	return strings.Join(lines, "\n")
}

func padBlockHeight(block string, width int, height int) string {
	lines := strings.Split(padBlock(block, width), "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTransferJobCard(entry config.TransferEntry, width int, selected bool, reserveErrorLine bool) string {
	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}

	title := m.transferEntryHostTitle(entry)
	meta := m.transferJobMeta(entry)
	dot := transferJobDot(entry.Status)
	nameLine := m.transferFileLine(entry, cardWidth-4, selected)
	sourceLine := transferPathLine(transferSourceSymbol(entry), entry.Source)
	targetLine := transferPathLine("→", transferJobTarget(entry))

	lines := []string{
		cardTopLine(cardWidth, title, meta, dot, borderStyle),
		cardContentLine(cardWidth, nameLine, borderStyle),
		cardContentLine(cardWidth, sourceLine, borderStyle),
		cardContentLine(cardWidth, targetLine, borderStyle),
		cardContentLine(cardWidth, transferProgressBarLine(entry, cardWidth-4), borderStyle),
	}
	if errorLine := transferJobError(entry); errorLine != "" || reserveErrorLine {
		lines = append(lines, cardContentLine(cardWidth, errorLine, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func (m Model) transferEntryHostTitle(entry config.TransferEntry) string {
	category := strings.TrimSpace(entry.HostCategory)
	if category == "" {
		category = m.t("Uncategorized", "未分类")
	}
	name := strings.TrimSpace(entry.HostName)
	if name == "" {
		name = m.t("Server", "服务器")
	}
	return cardMutedStyle.Render("["+category+"]") + " " + detailValueStyle.Render(name)
}

func transferEntryName(entry config.TransferEntry) string {
	name := filepath.Base(strings.TrimRight(entry.Source, "/"))
	if name == "." || name == "/" || name == "" {
		name = entry.Source
	}
	if entry.IsDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

func transferSourceSymbol(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return "↓"
	}
	return "↑"
}

func transferJobTarget(entry config.TransferEntry) string {
	if entry.Kind == "upload" {
		return entry.HostName + ":" + entry.TargetDir
	}
	return entry.TargetDir
}

func (m Model) transferJobMeta(entry config.TransferEntry) string {
	style := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true)
	return style.Render(m.transferStatusText(entry.Status))
}

func transferJobDot(status string) string {
	return lipgloss.NewStyle().Foreground(transferStatusColor(status)).Render("●")
}

func transferFieldLine(label string, value string) string {
	return cardMutedStyle.Render(label+" ") + detailValueStyle.Render(value)
}

func transferPathLine(label string, value string) string {
	return transferArrowStyle(label).Render(label+" ") + cardMutedStyle.Render(value)
}

func transferArrowStyle(label string) lipgloss.Style {
	switch label {
	case "↑", "↓", "→":
		return blueStyle
	default:
		return cardMutedStyle
	}
}

func (m Model) transferFileLine(entry config.TransferEntry, width int, selected bool) string {
	nameStyle := detailValueStyle
	if selected {
		nameStyle = blueStyle.Bold(true)
	}
	left := cardMutedStyle.Render(m.transferEntryTypeLabel(entry)+" ") + nameStyle.Render(transferEntryName(entry))
	right := cardMutedStyle.Render(m.transferEntryKindText(entry) + " " + transferTimeText(entry))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		maxLeft := width - ansi.StringWidth(right) - 2
		if maxLeft < 8 {
			return left
		}
		left = fitANSI(left, maxLeft)
		gap = width - ansi.StringWidth(left) - ansi.StringWidth(right)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) transferEntryTypeLabel(entry config.TransferEntry) string {
	if entry.IsDir {
		return m.t("Dir", "目录")
	}
	return m.t("File", "文件")
}

func transferJobError(entry config.TransferEntry) string {
	if entry.Error != "" {
		return cardMutedStyle.Render("错误 ") + redStyle.Render(entry.Error)
	}
	return ""
}
