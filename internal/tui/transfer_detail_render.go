package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/config"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) renderTransferDetail() string {
	m.reloadTransfers()
	entry, ok := m.selectedTransferEntry()
	width := detailFrameWidth(m.width)
	if width < 34 {
		width = 34
	}
	if !ok {
		return strings.Join([]string{
			titleStyle.Render(fit(m.t("Transfer Detail", "传输详情"), width)),
			mutedStyle.Render(m.t("Current job does not exist", "当前任务不存在")),
			renderHelp(width, m.t("Back Esc", "返回 Esc")),
		}, "\n")
	}
	lines := m.transferDetailLines(entry)
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	headerText := fmt.Sprintf("%s  %s  %s", m.t("Transfer Detail", "传输详情"), transferEntryName(entry), m.transferStatusText(entry.Status))
	header := titleStyle.Render(fitANSI(headerText, width))
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	help := renderHelp(width, m.t("Scroll ↑↓/jk  Start Enter  Start all a  Pause all p  Cancel c  Delete x  Back Esc", "滚动 ↑↓/jk  开始 Enter  全部开始 a  全部暂停 p  取消 c  删除 x  返回 Esc"))
	return strings.Join([]string{header, body, help}, "\n")
}

func (m Model) selectedTransferEntry() (config.TransferEntry, bool) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return config.TransferEntry{}, false
	}
	return m.transferHistory.Entries[m.transferIndex], true
}

func (m Model) transferDetailLines(entry config.TransferEntry) []string {
	status := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true).Render(m.transferStatusText(entry.Status))
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
		m.renderDetailSectionLine(m.t("Basic", "基本信息"), sectionTitle(m.t("Basic", "基本信息"))),
		m.detailRow(m.t("Status", "状态"), status),
		m.detailRow(m.t("Type", "类型"), m.transferEntryKindText(entry)),
		m.detailRow(m.t("Direction", "方向"), m.transferDirectionText(entry)),
		m.detailRow(m.t("File", "文件"), transferEntryName(entry)),
		m.detailRow(m.t("Directory", "目录"), yesNoLang(entry.IsDir, m.isChineseUI())),
		m.detailRow(m.t("Job ID", "任务ID"), entry.ID),
		m.detailRow(m.t("Server", "服务器"), ansi.Strip(m.transferEntryHostTitle(entry))),
		m.detailRow(m.t("Connection", "连接"), transferEntryConnection(entry)),
		m.detailRow(m.t("Created", "创建时间"), transferTimeShort(entry.Time)),
		m.detailRow(m.t("Updated", "更新时间"), transferTimeShort(entry.UpdatedAt)),
		m.detailRow(m.t("Queue", "队列位置"), transferQueueText(m.transferHistory.Entries, entry)),
		m.detailRow(m.t("Method", "传输方式"), m.t("rsync, resumable, keeps partial files", "rsync，支持断点续传，保留半成品")),
		"",
		m.renderDetailSectionLine(m.t("Paths", "路径信息"), sectionTitle(m.t("Paths", "路径信息"))),
		m.detailRow(m.t("Source", "来源"), entry.Source),
		m.detailRow(m.t("Target", "目标"), transferJobTarget(entry)),
		"",
		m.renderDetailSectionLine(m.t("Progress", "传输进度"), sectionTitle(m.t("Progress", "传输进度"))),
		m.detailRow(m.t("Progress", "进度"), progress),
		m.detailRow(m.t("Percent", "百分比"), percent),
		m.detailRow(m.t("Total", "总大小"), total),
		m.detailRow(m.t("Done", "已完成"), done),
		m.detailRow(m.t("Remaining", "剩余大小"), remaining),
		m.detailRow(m.t("Speed", "速度"), emptyDash(speed)),
		m.detailRow(m.t("ETA", "剩余时间"), emptyDash(remain)),
		m.detailRow(m.t("Raw progress", "原始进度"), emptyDash(strings.Join(strings.Fields(entry.Progress), " "))),
		"",
		m.renderDetailSectionLine(m.t("Actions", "操作"), sectionTitle(m.t("Actions", "操作"))),
		m.detailRow(m.t("Available", "可操作"), m.transferActionHint(entry.Status)),
	}
	if strings.TrimSpace(entry.Error) != "" {
		lines = append(lines, "", m.renderDetailSectionLine(m.t("Error", "错误"), sectionTitle(m.t("Error", "错误"))), m.detailRow(m.t("Error", "错误"), redStyle.Render(entry.Error)))
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

func (m Model) transferDirectionText(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return m.t("Remote → Local", "远程 → 本地")
	}
	return m.t("Local → Remote", "本地 → 远程")
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

func (m Model) transferActionHint(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return m.t("Enter start, c cancel, x delete", "Enter 开始，c 取消，x 删除")
	case config.TransferStatusPending:
		return m.t("p pause all, waiting to start", "p 全部暂停，等待自动开始")
	case config.TransferStatusRunning:
		return m.t("p pause, c interrupt", "p 暂停，c 中断")
	case config.TransferStatusInterrupted:
		return m.t("Enter resume, a start all, c cancel, x delete", "Enter 继续，a 全部开始，c 取消，x 删除")
	case config.TransferStatusFailed:
		return m.t("Enter retry, x delete", "Enter 重试，x 删除")
	case config.TransferStatusCanceled:
		return m.t("x delete", "x 删除")
	case config.TransferStatusDone:
		return m.t("x delete", "x 删除")
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
		if transferservice.PercentText(field) != "" {
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
