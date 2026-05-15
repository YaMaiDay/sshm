package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderTransferPanel() string {
	title := "上传文件"
	if m.panel.Mode == transferDownload {
		title = "下载文件"
	}
	header := title
	if m.status != "" {
		header += "  " + m.status
	}
	width := formContentWidth(m.width)
	help := "切换 Tab  移动 ↑↓/jk  展开 Enter  选择 Space  任务 s  返回 Esc"
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	body := ""
	if m.useSingleTransferPane(width) {
		if m.panel.ActivePane == 0 {
			body = renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, width, height, true, m.panel.LeftSelected)
		} else {
			body = renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, width, height, true, nil)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) / 2
		rightWidth := width - gap - leftWidth
		left := renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, leftWidth, height, m.panel.ActivePane == 0, m.panel.LeftSelected)
		right := renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, rightWidth, height, m.panel.ActivePane == 1, nil)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		body,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) renderTransferJobs() string {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 34 {
		width = 34
	}
	help := renderTransferJobsHelp(width)
	reservedBottomLines := strings.Count(help, "\n") + 1
	counts := transferStatusCounts(m.transferHistory.Entries)
	filtered := m.filteredTransferIndexes()
	header := fmt.Sprintf("传输任务  状态 %s  显示 %d/%d  运行 %d  未完成 %d  已完成 %d", m.transferStatusFilterName(), len(filtered), len(m.transferHistory.Entries), counts[config.TransferStatusRunning], transferUnfinishedCount(m.transferHistory.Entries), counts[config.TransferStatusDone])
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.transferHistory.Entries) == 0 {
		lines = append(lines, mutedStyle.Render("暂无传输记录"))
	} else if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("当前状态没有传输任务"))
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

func (m Model) renderTransferDetail() string {
	m.reloadTransfers()
	entry, ok := m.selectedTransferEntry()
	width := detailFrameWidth(m.width)
	if width < 34 {
		width = 34
	}
	if !ok {
		return strings.Join([]string{
			titleStyle.Render(fit("传输详情", width)),
			mutedStyle.Render("当前任务不存在"),
			renderHelp(width, "返回 Esc"),
		}, "\n")
	}
	lines := m.transferDetailLines(entry)
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	headerText := fmt.Sprintf("传输详情  %s  %s", transferEntryName(entry), transferStatusText(entry.Status))
	header := titleStyle.Render(fitANSI(headerText, width))
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	help := renderHelp(width, "滚动 ↑↓/jk  开始 Enter  全部开始 a  全部暂停 p  取消 c  删除 x  返回 Esc")
	return strings.Join([]string{header, body, help}, "\n")
}

func (m Model) selectedTransferEntry() (config.TransferEntry, bool) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return config.TransferEntry{}, false
	}
	return m.transferHistory.Entries[m.transferIndex], true
}

func (m Model) transferDetailLines(entry config.TransferEntry) []string {
	status := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true).Render(transferStatusText(entry.Status))
	total := "-"
	if entry.TotalBytes > 0 {
		total = bytesHuman(uint64(entry.TotalBytes))
	}
	done := "-"
	if entry.TotalBytes > 0 || transferProgressDoneBytes(entry) > 0 {
		done = bytesHuman(uint64(transferProgressDoneBytes(entry)))
	}
	remaining := transferRemainingBytesText(entry)
	speed, remain := transferProgressSpeedRemain(entry.Progress)
	percent := transferPercentText(entry)
	progress := transferProgressBarLine(entry, m.detailContentWidth())
	lines := []string{
		m.renderDetailSectionLine("基本信息", sectionTitle("基本信息")),
		m.detailRow("状态", status),
		m.detailRow("类型", transferEntryKindText(entry)),
		m.detailRow("方向", transferDirectionText(entry)),
		m.detailRow("文件", transferEntryName(entry)),
		m.detailRow("目录", yesNo(entry.IsDir)),
		m.detailRow("任务ID", entry.ID),
		m.detailRow("服务器", ansi.Strip(transferEntryHostTitle(entry))),
		m.detailRow("连接", transferEntryConnection(entry)),
		m.detailRow("创建时间", transferTimeShort(entry.Time)),
		m.detailRow("更新时间", transferTimeShort(entry.UpdatedAt)),
		m.detailRow("队列位置", transferQueueText(m.transferHistory.Entries, entry)),
		m.detailRow("传输方式", "rsync，支持断点续传，保留半成品"),
		"",
		m.renderDetailSectionLine("路径信息", sectionTitle("路径信息")),
		m.detailRow("来源", entry.Source),
		m.detailRow("目标", transferJobTarget(entry)),
		"",
		m.renderDetailSectionLine("传输进度", sectionTitle("传输进度")),
		m.detailRow("进度", progress),
		m.detailRow("百分比", percent),
		m.detailRow("总大小", total),
		m.detailRow("已完成", done),
		m.detailRow("剩余大小", remaining),
		m.detailRow("速度", emptyDash(speed)),
		m.detailRow("剩余时间", emptyDash(remain)),
		m.detailRow("原始进度", emptyDash(strings.Join(strings.Fields(entry.Progress), " "))),
		"",
		m.renderDetailSectionLine("操作", sectionTitle("操作")),
		m.detailRow("可操作", transferActionHint(entry.Status)),
	}
	if strings.TrimSpace(entry.Error) != "" {
		lines = append(lines, "", m.renderDetailSectionLine("错误", sectionTitle("错误")), m.detailRow("错误", redStyle.Render(entry.Error)))
	}
	return lines
}

func (m Model) transferDetailMaxScroll() int {
	entry, ok := m.selectedTransferEntry()
	if !ok {
		return 0
	}
	maxScroll := len(m.transferDetailLines(entry)) - m.detailViewportHeight()
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func transferEntryConnection(entry config.TransferEntry) string {
	user := strings.TrimSpace(entry.User)
	if user == "" {
		user = "-"
	}
	host := strings.TrimSpace(entry.Host)
	if host == "" {
		host = "-"
	}
	port := strings.TrimSpace(entry.Port)
	if port == "" {
		port = "22"
	}
	return fmt.Sprintf("%s@%s:%s", user, host, port)
}

func transferDirectionText(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return "远程 → 本地"
	}
	return "本地 → 远程"
}

func transferRemotePath(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return entry.Source
	}
	return transferJobTarget(entry)
}

func transferLocalPath(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return entry.TargetDir
	}
	return entry.Source
}

func transferPercentText(entry config.TransferEntry) string {
	percent, ok := transferProgressPercent(entry)
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%d%%", percent)
}

func transferRemainingBytesText(entry config.TransferEntry) string {
	if entry.TotalBytes <= 0 {
		return "-"
	}
	done := transferProgressDoneBytes(entry)
	if done >= entry.TotalBytes {
		return "0B"
	}
	return bytesHuman(uint64(entry.TotalBytes - done))
}

func transferQueueText(entries []config.TransferEntry, entry config.TransferEntry) string {
	if entry.Status == config.TransferStatusRunning {
		return "当前运行"
	}
	if entry.Status != config.TransferStatusPending && entry.Status != config.TransferStatusQueued {
		return "-"
	}
	position := 0
	total := 0
	for _, item := range entries {
		if item.Status != entry.Status {
			continue
		}
		total++
		if item.ID == entry.ID {
			position = total
		}
	}
	if position == 0 || total == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", position, total)
}

func transferActionHint(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return "Enter 开始，c 取消，x 删除"
	case config.TransferStatusPending:
		return "p 全部暂停，等待自动开始"
	case config.TransferStatusRunning:
		return "p 暂停，c 中断"
	case config.TransferStatusInterrupted:
		return "Enter 继续，a 全部开始，c 取消，x 删除"
	case config.TransferStatusFailed:
		return "Enter 重试，x 删除"
	case config.TransferStatusCanceled:
		return "x 删除"
	case config.TransferStatusDone:
		return "x 删除"
	default:
		return "-"
	}
}

func transferProgressSpeedRemain(progress string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(progress))
	if len(fields) == 0 {
		return "", ""
	}
	percentIndex := -1
	for i, field := range fields {
		if rsyncPercentText(field) != "" {
			percentIndex = i
			break
		}
	}
	if percentIndex < 0 {
		return "", ""
	}
	speed := ""
	remain := ""
	for _, field := range fields[percentIndex+1:] {
		cleaned := strings.Trim(field, "()")
		if speed == "" && strings.Contains(cleaned, "/s") {
			speed = cleaned
			continue
		}
		if remain == "" && strings.Count(cleaned, ":") >= 2 {
			remain = cleaned
		}
	}
	return speed, remain
}

func renderTransferJobsHelp(width int) string {
	if width < 1 {
		width = 1
	}
	help := strings.Join([]string{
		"状态 Tab",
		"移动 ↑↓←→/hjkl",
		"开始 Enter",
		"详情 Space",
		"全部开始 a",
		"全部暂停 p",
		"取消 c",
		"删除 x",
		"返回 Esc",
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
		return "全部"
	}
	return transferStatusText(status)
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
			block := renderTransferJobCard(m.transferHistory.Entries[entryIndex], cardWidth, entryIndex == m.transferIndex, rowHasError)
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

func renderTransferJobCard(entry config.TransferEntry, width int, selected bool, reserveErrorLine bool) string {
	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}

	title := transferEntryHostTitle(entry)
	meta := transferJobMeta(entry)
	dot := transferJobDot(entry.Status)
	nameLine := transferFileLine(entry, cardWidth-4, selected)
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

func transferEntryHostTitle(entry config.TransferEntry) string {
	category := strings.TrimSpace(entry.HostCategory)
	if category == "" {
		category = "未分类"
	}
	name := strings.TrimSpace(entry.HostName)
	if name == "" {
		name = "服务器"
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

func transferJobMeta(entry config.TransferEntry) string {
	style := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true)
	return style.Render(transferStatusText(entry.Status))
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

func transferFileLine(entry config.TransferEntry, width int, selected bool) string {
	nameStyle := detailValueStyle
	if selected {
		nameStyle = blueStyle.Bold(true)
	}
	left := cardMutedStyle.Render(transferEntryTypeLabel(entry)+" ") + nameStyle.Render(transferEntryName(entry))
	right := cardMutedStyle.Render(transferEntryKindText(entry) + " " + transferTimeText(entry))
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

func transferEntryTypeLabel(entry config.TransferEntry) string {
	if entry.IsDir {
		return "目录"
	}
	return "文件"
}

func transferJobError(entry config.TransferEntry) string {
	if entry.Error != "" {
		return cardMutedStyle.Render("错误 ") + redStyle.Render(entry.Error)
	}
	return ""
}

func transferProgressBarLine(entry config.TransferEntry, width int) string {
	percent, ok := transferProgressPercent(entry)
	if !ok && entry.Status == config.TransferStatusDone {
		percent = 100
		ok = true
	}
	label := "--"
	if ok {
		label = fmt.Sprintf("%3d%%", percent)
	}
	style := transferProgressStyle(entry.Status)
	suffix := style.Render(label)
	if detail := transferProgressDetail(entry); detail != "" {
		maxDetail := width - 8 - runewidth.StringWidth(label) - 2
		if maxDetail > 4 {
			suffix += " " + cardMutedStyle.Render(fit(detail, maxDetail))
		}
	}
	barWidth := width - ansi.StringWidth(suffix) - 1
	if barWidth < 8 {
		barWidth = 8
	}
	filled := 0
	if ok {
		filled = int(float64(barWidth) * float64(percent) / 100)
		if percent > 0 && filled == 0 {
			filled = 1
		}
		if filled > barWidth {
			filled = barWidth
		}
	}
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", barWidth-filled))
	return bar + " " + suffix
}

func transferProgressPercent(entry config.TransferEntry) (int, bool) {
	if entry.TotalBytes > 0 {
		done := transferProgressDoneBytes(entry)
		percent := int(float64(done) * 100 / float64(entry.TotalBytes))
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		return percent, true
	}
	percentText := rsyncPercentText(entry.Progress)
	if percentText == "" {
		return 0, false
	}
	value := strings.TrimSuffix(percentText, "%")
	percent, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return percent, true
}

func transferProgressDoneBytes(entry config.TransferEntry) int64 {
	done := entry.DoneBytes + entry.CurrentBytes
	if entry.TotalBytes > 0 && done > entry.TotalBytes {
		return entry.TotalBytes
	}
	if done < 0 {
		return 0
	}
	return done
}

func transferProgressDetail(entry config.TransferEntry) string {
	sizeText := ""
	if entry.TotalBytes > 0 {
		sizeText = bytesPair(uint64(transferProgressDoneBytes(entry)), uint64(entry.TotalBytes))
	}
	progress := strings.Join(strings.Fields(strings.TrimSpace(entry.Progress)), " ")
	percent := rsyncPercentText(progress)
	if progress == "" || percent == "" || progress == percent {
		return sizeText
	}
	if idx := strings.Index(progress, " ("); idx >= 0 {
		progress = strings.TrimSpace(progress[:idx])
	}
	idx := strings.Index(progress, percent)
	if idx < 0 {
		return ""
	}
	before := strings.TrimSpace(progress[:idx])
	after := strings.TrimSpace(progress[idx+len(percent):])
	rsyncText := strings.TrimSpace(before + " " + after)
	if sizeText == "" {
		return rsyncText
	}
	if after == "" {
		return sizeText
	}
	return strings.TrimSpace(sizeText + " " + after)
}

func transferProgressStyle(status string) lipgloss.Style {
	switch status {
	case config.TransferStatusQueued:
		return detailSubTitleStyle
	case config.TransferStatusPending:
		return blueStyle
	case config.TransferStatusRunning:
		return blueStyle
	case config.TransferStatusDone:
		return greenStyle
	case config.TransferStatusFailed:
		return redStyle
	case config.TransferStatusInterrupted:
		return yellowStyle
	case config.TransferStatusCanceled:
		return mutedStyle
	default:
		return mutedStyle
	}
}

func transferStatusColor(status string) lipgloss.Color {
	switch status {
	case config.TransferStatusQueued:
		return cyan
	case config.TransferStatusPending:
		return blue
	case config.TransferStatusRunning:
		return blue
	case config.TransferStatusDone:
		return green
	case config.TransferStatusFailed:
		return red
	case config.TransferStatusInterrupted:
		return yellow
	case config.TransferStatusCanceled:
		return textGray
	default:
		return textGray
	}
}

func transferStatusText(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return "等待中"
	case config.TransferStatusPending:
		return "排队中"
	case config.TransferStatusRunning:
		return "运行中"
	case config.TransferStatusDone:
		return "已完成"
	case config.TransferStatusFailed:
		return "失败"
	case config.TransferStatusCanceled:
		return "已取消"
	case config.TransferStatusInterrupted:
		return "中断"
	default:
		return status
	}
}

func transferTimeText(entry config.TransferEntry) string {
	if entry.UpdatedAt != "" {
		return transferTimeShort(entry.UpdatedAt)
	}
	return transferTimeShort(entry.Time)
}

func transferTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return t.Local().Format("01-02 15:04")
}

func transferStatusCounts(entries []config.TransferEntry) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Status]++
	}
	return counts
}

func transferUnfinishedCount(entries []config.TransferEntry) int {
	total := 0
	for _, entry := range entries {
		if entry.Status == config.TransferStatusQueued || entry.Status == config.TransferStatusPending || entry.Status == config.TransferStatusRunning || entry.Status == config.TransferStatusInterrupted {
			total++
		}
	}
	return total
}

func (m Model) useSingleTransferPane(width int) bool {
	return width < 70
}

func renderTransferPane(title string, choices []choice, index int, width int, height int, active bool, selected map[string]bool) string {
	if width < 34 {
		width = 34
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width)
	if active {
		style = style.BorderForeground(blue)
	}
	innerWidth := width - 4
	lines := []string{titleStyle.Render(title)}
	if len(choices) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的项目"))
	} else {
		maxRows := height - 2
		if maxRows < 3 {
			maxRows = 3
		}
		start := 0
		if index >= maxRows {
			start = index - maxRows + 1
		}
		end := start + maxRows
		if end > len(choices) {
			end = len(choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			lineStyle := lipgloss.NewStyle()
			if choices[i].IsDir {
				lineStyle = blueStyle
			}
			if i == index {
				prefix = "▶"
				lineStyle = lineStyle.Bold(true)
			}
			mark := " "
			if selected != nil && selected[choices[i].Value] {
				mark = "✓"
			}
			lines = append(lines, lineStyle.Render(fit(prefix+" "+mark+" "+choices[i].Label, innerWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return style.Render(strings.Join(lines, "\n"))
}
