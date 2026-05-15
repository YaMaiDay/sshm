package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func renderDashboardHelp(width int) string {
	if width < 1 {
		width = 1
	}
	help := strings.Join([]string{
		"更多 ?",
		"移动 ↑↓←→/hjkl",
		"登录 Enter",
		"详情 Space",
		"命令 m",
		"批量 b",
		"历史 i",
		"传输 y",
		"部署 g",
		"设置 F2",
		"总览 w",
		"视图 z",
		"置顶 t",
		"收藏 f",
		"收藏 v",
		"添加 a",
		"复制 c",
		"编辑 e",
		"删除 x",
		"上传 u",
		"下载 d",
		"刷新 r",
		"搜索 /",
		"分类 Tab",
		"在线 o",
		"异常 p",
		"排序 s",
		"退出 q",
	}, "  ")
	return helpStyle.Render(fit(help, width))
}

func (m Model) renderAddForm() string {
	title := "添加服务器"
	if m.editing {
		title = "编辑服务器"
	} else if m.copying {
		title = "复制服务器"
	}
	width := formContentWidth(m.width)
	if m.useSingleFormPane(width) {
		width = detailFrameWidth(m.width)
	}
	help := "切换 Tab  选择 ↑↓  分类 ←→  保存 Enter  返回 Esc"
	if m.formPane == 1 {
		help = "切回 Tab  选择 ↑↓  新增 n  重命名 r  删除 x  返回 Esc"
		if m.addingCategory {
			help = "添加 Enter  返回 Esc"
		} else if m.renamingCategory {
			help = "重命名 Enter  返回 Esc"
		}
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if strings.Contains(m.status, "失败") || strings.Contains(m.status, "不能") {
			statusStyle = redStyle
		}
		header += "  " + statusStyle.Render(fit(m.status, width-ansi.StringWidth(title)-2))
	}
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	body := ""
	if m.useSingleFormPane(width) {
		if m.formPane == 1 {
			body = m.renderCategoryPane(width, bodyHeight)
		} else {
			body = m.renderServerFormPane(title, width, bodyHeight)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) * 2 / 3
		rightWidth := width - gap - leftWidth
		if rightWidth < 28 {
			rightWidth = 28
			leftWidth = width - gap - rightWidth
		}
		left := m.renderServerFormPane(title, leftWidth, bodyHeight)
		right := m.renderCategoryPane(rightWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	lines := []string{
		header,
		body,
		renderHelp(width, help),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderPicker() string {
	header := m.pickTitle
	if m.status != "" && m.status != m.pickTitle {
		header += "  " + m.status
	}
	width := detailFrameWidth(m.width)
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.choices) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的项目"))
	} else {
		maxRows := m.height - 5
		if maxRows < 5 {
			maxRows = 5
		}
		start := 0
		if m.pickIndex >= maxRows {
			start = m.pickIndex - maxRows + 1
		}
		end := start + maxRows
		if end > len(m.choices) {
			end = len(m.choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.pickIndex {
				prefix = "▶"
				style = lipgloss.NewStyle().Foreground(blue).Bold(true)
			}
			label := m.choices[i].Label
			if m.treePickerActive() && m.choices[i].IsDir {
				label = blueStyle.Render(label)
			}
			lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s", prefix, label), width)))
		}
	}
	help := "移动 ↑↓/jk  选择 Enter  返回 Esc"
	if m.treePickerActive() {
		help = "移动 ↑↓/jk  展开 Enter  选择 Space  返回 Esc"
	}
	lines = append(lines, "", renderHelp(width, help))
	return strings.Join(lines, "\n")
}

func percentBar(value float64) string {
	return percentBarWidth(value, 8)
}

func percentBarWithThreshold(value float64, warn float64, crit float64) string {
	return percentBarWidthWithThreshold(value, 8, warn, crit)
}

func metricLine(label, value string) string {
	const labelWidth = 5
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return cardMutedStyle.Render(label) + strings.Repeat(" ", padding) + value
}

func cardMetricLine(label string, base string, extra string, width int) string {
	return metricLine(label, compactCardMetric(label, base, extra, width))
}

func compactCardMetric(label string, base string, extra string, width int) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if extra == "" || extra == "-" {
		return base
	}
	full := base + "  " + cardMutedStyle.Render(extra)
	if ansi.StringWidth(metricLine(label, full)) <= width {
		return full
	}
	return base
}

func threeMetricLine(width int, metrics monitor.Metrics) string {
	gap := 1
	colWidth := (width - gap*2) / 3
	if colWidth < 8 {
		colWidth = 8
	}
	barWidth := 4
	if colWidth >= 12 {
		barWidth = 5
	}
	if colWidth >= 15 {
		barWidth = 6
	}
	cpu := compactMetric("CPU", metrics.CPUPercent, colWidth, barWidth)
	mem := compactMetric("内存", metrics.MemPercent(), colWidth, barWidth)
	disk := compactDiskMetric(metrics, colWidth, barWidth)
	line := padVisible(cpu, colWidth) + strings.Repeat(" ", gap) + padVisible(mem, colWidth) + strings.Repeat(" ", gap) + padVisible(disk, colWidth)
	return fit(line, width)
}

func compactMetric(label string, value float64, width int, barWidth int) string {
	return compactMetricWithThreshold(label, value, width, barWidth, 70, 85)
}

func compactMetricWithThreshold(label string, value float64, width int, barWidth int, warn float64, crit float64) string {
	bar := compactPercentBarWithThreshold(value, barWidth, warn, crit)
	labelWidth := 4
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+bar, width)
}

func compactDiskMetric(metrics monitor.Metrics, width int, barWidth int) string {
	label := diskMountLabel(metrics)
	if label == "-" {
		label = "磁盘"
	}
	bar := compactPercentBarWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)
	return fit(label+" "+bar, width)
}

func compactPercentBar(value float64, total int) string {
	return compactPercentBarWithThreshold(value, total, 70, 85)
}

func compactPercentBarWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func padVisible(s string, width int) string {
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "")
	}
	if ansi.StringWidth(s) < width {
		s += strings.Repeat(" ", width-ansi.StringWidth(s))
	}
	return s
}

func cardTopLine(width int, title string, meta string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffixText := dot
	if strings.TrimSpace(meta) != "" && strings.TrimSpace(meta) != "-" {
		suffixText = meta + " " + dot
	}
	suffix := " " + suffixText + " "
	fillWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if fillWidth < 1 {
		title = ansi.Truncate(title, innerWidth-ansi.StringWidth(prefix)-ansi.StringWidth(titleGap)-ansi.StringWidth(suffix)-1, "…")
		fillWidth = innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	}
	if fillWidth < 0 {
		fillWidth = 0
	}
	return left + prefix + title + titleGap + borderStyle.Render(strings.Repeat("─", fillWidth)) + suffix + right
}

func cardNoteText(note string, width int) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return fit("备注 "+note, width)
}

func cardHeaderMeta(h host.Host, metrics monitor.Metrics) string {
	if strings.TrimSpace(h.ExpireAt) != "" {
		return expireCardText(h.ExpireAt)
	}
	return cardMutedStyle.Render(cardUptimeShort(metrics.Uptime))
}

func expireCardText(value string) string {
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("到期格式错")
	}
	switch {
	case days < 0:
		return redStyle.Render("已过期")
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("到期%d天", days))
	default:
		return cardMutedStyle.Render(fmt.Sprintf("到期%d天", days))
	}
}

func expireDetailText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("格式错误")
	}
	switch {
	case days < 0:
		return redStyle.Render(fmt.Sprintf("已过期%d天", -days))
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("剩余%d天", days))
	default:
		return fmt.Sprintf("剩余%d天", days)
	}
}

func expireDays(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	expire, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return 0, false
	}
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return int(expire.Sub(today).Hours() / 24), true
}

func cardContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := padVisible(content, contentWidth)
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardMutedContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := cardMutedStyle.Render(padVisible(content, contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardInnerSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	line := cardBorderStyle.Render(dashedLine(contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func dashedLine(width int) string {
	if width <= 0 {
		return ""
	}
	pattern := "- "
	line := strings.Repeat(pattern, (width+len(pattern)-1)/len(pattern))
	return ansi.Truncate(line, width, "")
}

func cardSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("├") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("┤")
}

func cardBottomLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("╰") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("╯")
}

func statusDot(loading bool, online bool) string {
	if loading {
		return "●"
	}
	if online {
		return "●"
	}
	return "●"
}

func percentBarWidth(value float64, total int) string {
	return percentBarWidthWithThreshold(value, total, 70, 85)
}

func percentBarWidthWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func metricValueStyle(value float64, warn float64, crit float64) lipgloss.Style {
	if value >= crit {
		return redStyle
	}
	if value >= warn {
		return yellowStyle
	}
	return greenStyle
}

func colorStatus(value string, loading bool, online bool) string {
	if loading {
		return yellowStyle.Render(value)
	}
	if online {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func contentWidth(width int) int {
	if width <= 0 {
		return 100
	}
	return width
}

func detailFrameWidth(width int) int {
	if width <= 0 {
		return 100
	}
	if width < 44 {
		return 42
	}
	return width - 2
}

func formContentWidth(width int) int {
	if width <= 0 {
		return 100
	}
	if width < 44 {
		return 42
	}
	return width - 4
}

func stringChoices(values []string, dirs bool) []choice {
	out := make([]choice, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		label := value
		if dirs {
			label = "[目录] " + value
		}
		out = append(out, choice{Label: label, Value: value, IsDir: dirs})
	}
	return out
}

func localItemChoices(items []fsselect.Item) []choice {
	return itemChoices(items)
}

func itemChoices(items []fsselect.Item) []choice {
	out := make([]choice, 0, len(items))
	for _, item := range items {
		kind := "[文件] "
		if item.IsDir {
			kind = "[目录] "
		}
		out = append(out, choice{
			Label: kind + item.Path,
			Value: item.Path,
			IsDir: item.IsDir,
		})
	}
	return out
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func authText(h host.Host) string {
	hasKey := strings.TrimSpace(h.IdentityFile) != ""
	hasPassword := h.HasPassword || strings.TrimSpace(h.Password) != ""
	switch {
	case hasKey && hasPassword:
		return "密钥：" + filepath.Base(h.IdentityFile) + "，密码"
	case hasKey:
		return "密钥：" + filepath.Base(h.IdentityFile)
	case hasPassword:
		return "密码"
	default:
		return "系统 SSH 默认"
	}
}

func jumpDetailText(h host.Host) string {
	if !h.JumpEnabled {
		return "未启用"
	}
	if strings.TrimSpace(h.JumpHostRef) != "" {
		return h.JumpHostRef + "，仅转发，本地密钥认证"
	}
	port := strings.TrimSpace(h.JumpPort)
	if port == "" {
		port = "22"
	}
	target := h.JumpHost
	if strings.TrimSpace(h.JumpUser) != "" {
		target = h.JumpUser + "@" + target
	}
	return target + ":" + port + "，仅转发，本地密钥认证"
}

func jumpKeyText(h host.Host) string {
	if !h.JumpEnabled {
		return "-"
	}
	if strings.TrimSpace(h.JumpKeyPath) == "" {
		return "系统 SSH 默认"
	}
	return filepath.Base(h.JumpKeyPath) + "（本地）"
}

func transferErrorText(err error, output string) string {
	text := cleanTransferOutput(output)
	if text != "" {
		return text
	}
	if err == nil {
		return "未知错误"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("命令退出码 %d", exitErr.ExitCode())
	}
	return err.Error()
}

func cleanTransferOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "** WARNING:") ||
			strings.HasPrefix(line, "** This session") ||
			strings.HasPrefix(line, "** The server") {
			continue
		}
		if rsyncProgressText(line) != "" {
			continue
		}
		return line
	}
	return ""
}

func sectionTitle(value string) string {
	return detailSectionStyle.Render("[" + value + "]")
}

func detailSubTitle(value string) string {
	return detailSubTitleStyle.Render("· " + value)
}

func detailSuccessSubTitle(value string) string {
	return detailSuccessStyle.Render("· " + value)
}

func detailDangerSubTitle(value string) string {
	return detailDangerStyle.Render("· " + value)
}

func (m Model) detailRow(label, value string) string {
	const labelWidth = 10
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	continuationPrefix := strings.Repeat(" ", labelWidth)
	valueWidth := m.detailContentWidth() - labelWidth
	if valueWidth < 12 {
		valueWidth = 12
	}
	parts := wrapDetailValue(value, valueWidth)
	if len(parts) == 0 {
		return prefix
	}
	lines := make([]string, 0, len(parts))
	lines = append(lines, prefix+detailValue(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, continuationPrefix+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func detailValue(value string) string {
	if strings.Contains(value, "\x1b[") {
		return value
	}
	return detailValueStyle.Render(value)
}

func (m Model) detailContentWidth() int {
	width := detailFrameWidth(m.width) - 6
	if width < 24 {
		width = 24
	}
	return width
}

func wrapDetailValue(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	if ansi.StringWidth(value) <= width {
		return []string{value}
	}
	if strings.Contains(value, "\x1b") {
		return []string{ansi.Truncate(value, width, "…")}
	}
	var lines []string
	current := ""
	for _, token := range splitWrapTokens(value) {
		if current == "" {
			current = token
			continue
		}
		if ansi.StringWidth(current+token) <= width {
			current += token
			continue
		}
		lines = appendWrappedLine(lines, current, width)
		current = strings.TrimLeft(token, " ")
	}
	if current != "" {
		lines = appendWrappedLine(lines, current, width)
	}
	return lines
}

func wrapPlainLine(value string, width int) string {
	return strings.Join(wrapDetailValue(value, width), "\n")
}

func renderHelp(width int, value string) string {
	return helpStyle.Render(fit(value, width))
}

func splitWrapTokens(value string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range value {
		current.WriteRune(r)
		if r == ',' || r == '/' || r == ' ' {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func appendWrappedLine(lines []string, value string, width int) []string {
	value = strings.TrimSpace(value)
	for ansi.StringWidth(value) > width {
		runes := []rune(value)
		cut := 0
		for cut < len(runes) && runewidth.StringWidth(string(runes[:cut+1])) <= width {
			cut++
		}
		if cut <= 0 {
			cut = 1
		}
		lines = append(lines, string(runes[:cut]))
		value = strings.TrimSpace(string(runes[cut:]))
	}
	if value != "" {
		lines = append(lines, value)
	}
	return lines
}

func uptimeCN(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(value)
	value = normalizeWeeksToDays(value)
	replacer := strings.NewReplacer(
		" days", "天",
		" day", "天",
		" hours", "小时",
		" hour", "小时",
		" minutes", "分钟",
		" minute", "分钟",
		", ", "",
		" ago", "前",
	)
	value = replacer.Replace(value)
	return value
}

func cardUptimeShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	if days > 0 {
		return fmt.Sprintf("%d天", days)
	}
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	if hours > 0 {
		return fmt.Sprintf("%d时", hours)
	}
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%d分", minutes)
}

func lastLoginDetail(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	relative := relativeTime(value)
	if relative != "刚刚" {
		relative += "前"
	}
	return value.Format("2006-01-02 15:04") + "（" + relative + "）"
}

func lastLoginCard(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	relative := relativeTime(value)
	if relative == "刚刚" {
		return relative
	}
	return relative + "前"
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	d := time.Since(value)
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "刚刚"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d分", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d时", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%d天", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d月", months)
	}
	return fmt.Sprintf("%d年", days/365)
}

func parseLoginRecords(output string, limit int) []string {
	var records []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "wtmp begins") ||
			strings.HasPrefix(lower, "btmp begins") ||
			strings.HasPrefix(lower, "reboot ") ||
			strings.HasPrefix(lower, "shutdown ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		records = append(records, strings.Join(fields, " "))
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records
}

func failedLoginScript() string {
	return `if ! command -v lastb >/dev/null 2>&1; then
  echo "__SSHM_LASTB_UNAVAILABLE__"
  exit 0
fi
out=$(lastb -n 100 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n lastb -n 100 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_LASTB_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func serviceDetailScript() string {
	return `if ! command -v systemctl >/dev/null 2>&1; then
  echo "__SSHM_SYSTEMCTL_UNAVAILABLE__"
  exit 0
fi
out=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func portDetailScript() string {
	return `if ! command -v ss >/dev/null 2>&1; then
  echo "__SSHM_SS_UNAVAILABLE__"
  exit 0
fi
out=$(ss -H -tulnp 2>&1)
code=$?
if [ "$code" -eq 0 ] && ! printf '%s\n' "$out" | grep -q 'users:('; then
  sudo_out=$(sudo -n ss -H -tulnp 2>&1)
  sudo_code=$?
  if [ "$sudo_code" -eq 0 ]; then
    out="$sudo_out"
  fi
fi
if [ "$code" -ne 0 ]; then
  sudo_out=$(sudo -n ss -H -tulnp 2>&1)
  sudo_code=$?
  if [ "$sudo_code" -ne 0 ]; then
    echo "__SSHM_SS_PERMISSION__"
    printf '%s\n' "$sudo_out"
    exit 0
  fi
  out="$sudo_out"
fi
printf '%s\n' "$out"`
}

func containerDetailScript() string {
	return `if ! command -v docker >/dev/null 2>&1; then
  echo "__SSHM_DOCKER_UNAVAILABLE__"
  exit 0
fi
out=$(docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_DOCKER_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func sshdSecurityScript() string {
	return `if command -v sshd >/dev/null 2>&1; then
  sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
elif [ -x /usr/sbin/sshd ]; then
  /usr/sbin/sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
fi`
}

func parseSSHDSettings(output string) map[string]string {
	settings := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		settings[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
	}
	return settings
}

func parseServiceDetails(output string) ([]serviceDetail, string) {
	if strings.Contains(output, "__SSHM_SYSTEMCTL_UNAVAILABLE__") {
		return nil, "systemctl不可用"
	}
	if strings.Contains(output, "__SSHM_SYSTEMCTL_ERROR__") {
		return nil, strings.TrimSpace(strings.ReplaceAll(output, "__SSHM_SYSTEMCTL_ERROR__", ""))
	}
	items := []serviceDetail{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "UNIT ") || strings.HasPrefix(line, "LOAD ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		item := serviceDetail{
			Unit:   fields[0],
			Load:   fields[1],
			Active: fields[2],
			Sub:    fields[3],
		}
		if len(fields) > 4 {
			item.Description = strings.Join(fields[4:], " ")
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := serviceDetailKindRank(items[i]), serviceDetailKindRank(items[j])
		if ki != kj {
			return ki < kj
		}
		return strings.ToLower(items[i].Unit) < strings.ToLower(items[j].Unit)
	})
	return items, ""
}

func parsePortDetails(output string) ([]portDetail, string) {
	if strings.Contains(output, "__SSHM_SS_UNAVAILABLE__") {
		return nil, "ss不可用"
	}
	if strings.Contains(output, "__SSHM_SS_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n ss）"
	}
	lines := strings.Split(output, "\n")
	grouped := map[string]*portDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Netid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[4]
		port := portFromAddress(local)
		if port == "" || port == "*" {
			continue
		}
		processText := ""
		if len(fields) > 6 {
			processText = strings.Join(fields[6:], " ")
		}
		process, pid := processFromSS(processText)
		key := fields[0] + "/" + port + "/" + process
		if item, ok := grouped[key]; ok {
			item.Count++
			if item.PID == "" && pid != "" {
				item.PID = pid
			}
			continue
		}
		grouped[key] = &portDetail{Protocol: fields[0], Port: port, Process: process, PID: pid, Count: 1}
	}
	out := make([]portDetail, 0, len(grouped))
	for _, item := range grouped {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		pi, _ := strconv.Atoi(out[i].Port)
		pj, _ := strconv.Atoi(out[j].Port)
		if pi == pj {
			return out[i].Protocol < out[j].Protocol
		}
		return pi < pj
	})
	return out, ""
}

func portFromAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if idx := strings.LastIndex(value, "]:"); idx >= 0 {
			return strings.TrimSpace(value[idx+2:])
		}
	}
	idx := strings.LastIndex(value, ":")
	if idx < 0 || idx == len(value)-1 {
		return ""
	}
	return strings.TrimSpace(value[idx+1:])
}

func processFromSS(value string) (string, string) {
	name := ""
	pid := ""
	if idx := strings.Index(value, "\""); idx >= 0 {
		rest := value[idx+1:]
		if end := strings.Index(rest, "\""); end >= 0 {
			name = rest[:end]
		}
	}
	if idx := strings.Index(value, "pid="); idx >= 0 {
		rest := value[idx+4:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		pid = rest[:end]
	}
	return name, pid
}

func parseContainerDetails(output string) ([]containerDetail, string) {
	if strings.Contains(output, "__SSHM_DOCKER_UNAVAILABLE__") {
		return nil, "未安装Docker"
	}
	if strings.Contains(output, "__SSHM_DOCKER_PERMISSION__") {
		return nil, "需要Docker权限（可配置sudo -n docker）"
	}
	lines := strings.Split(output, "\n")
	out := make([]containerDetail, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		item := containerDetail{
			Name:   strings.TrimSpace(parts[0]),
			Image:  strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
		}
		if len(parts) >= 4 {
			item.Ports = strings.TrimSpace(parts[3])
		}
		if item.Name != "" {
			out = append(out, item)
		}
	}
	return out, ""
}

func associatePortContainers(ports []portDetail, containers []containerDetail) {
	portMap := containerPublishedPortMap(containers)
	for i := range ports {
		key := strings.ToLower(ports[i].Protocol) + "/" + ports[i].Port
		if names := portMap[key]; len(names) > 0 {
			ports[i].Container = strings.Join(names, "、")
		}
	}
}

func containerPublishedPortMap(containers []containerDetail) map[string][]string {
	out := map[string][]string{}
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		for _, part := range strings.Split(container.Ports, ",") {
			hostPort, proto, ok := parseDockerPublishedPort(part)
			if !ok {
				continue
			}
			key := proto + "/" + hostPort
			if !stringSliceContains(out[key], name) {
				out[key] = append(out[key], name)
			}
		}
	}
	return out
}

func parseDockerPublishedPort(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return "", "", false
	}
	hostPort := portFromAddress(left)
	if hostPort == "" {
		return "", "", false
	}
	proto := "tcp"
	if idx := strings.LastIndex(right, "/"); idx >= 0 && idx < len(right)-1 {
		proto = strings.ToLower(strings.TrimSpace(right[idx+1:]))
	}
	if proto != "tcp" && proto != "udp" {
		proto = "tcp"
	}
	return hostPort, proto, true
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func failedLoginSummary(output string) ([]string, string) {
	if strings.Contains(output, "__SSHM_LASTB_UNAVAILABLE__") {
		return nil, "lastb不可用"
	}
	if strings.Contains(output, "__SSHM_LASTB_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n lastb）"
	}
	return loginSummaryRows(parseLoginRecords(output, 100)), ""
}

func loginSummaryRows(records []string) []string {
	if len(records) == 0 {
		return nil
	}
	ipCounts := map[string]int{}
	userCounts := map[string]int{}
	ipUsers := map[string]map[string]bool{}
	for _, record := range records {
		fields := strings.Fields(record)
		if len(fields) > 0 {
			userCounts[fields[0]]++
		}
		if len(fields) > 2 {
			ipCounts[fields[2]]++
			if ipUsers[fields[2]] == nil {
				ipUsers[fields[2]] = map[string]bool{}
			}
			if len(fields) > 0 {
				ipUsers[fields[2]][fields[0]] = true
			}
		}
	}
	rows := []string{
		fmt.Sprintf("统计\t最近%d条", len(records)),
		fmt.Sprintf("来源IP\t%s", topCountsText(ipCounts, 3)),
		fmt.Sprintf("用户名\t%s", topCountsText(userCounts, 5)),
		fmt.Sprintf("最近\t%s", records[0]),
	}
	if scanText := suspiciousScanText(ipUsers); scanText != "" {
		rows = append(rows, fmt.Sprintf("疑似扫描\t%s", scanText))
	}
	return rows
}

func suspiciousScanText(ipUsers map[string]map[string]bool) string {
	type item struct {
		IP    string
		Users int
	}
	items := []item{}
	for ip, users := range ipUsers {
		if len(users) >= 3 {
			items = append(items, item{IP: ip, Users: len(users)})
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Users == items[j].Users {
			return items[i].IP < items[j].IP
		}
		return items[i].Users > items[j].Users
	})
	limit := minInt(3, len(items))
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s 尝试%d个用户名", items[i].IP, items[i].Users))
	}
	return strings.Join(parts, "、")
}

func topCountsText(counts map[string]int, limit int) string {
	if len(counts) == 0 {
		return "-"
	}
	type item struct {
		Value string
		Count int
	}
	items := make([]item, 0, len(counts))
	for value, count := range counts {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items = append(items, item{Value: value, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Value < items[j].Value
		}
		return items[i].Count > items[j].Count
	})
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s %d次", items[i].Value, items[i].Count))
	}
	return strings.Join(parts, "、")
}

func firstUptimeNumber(value string, pattern string) int {
	re := regexp.MustCompile(pattern)
	parts := re.FindStringSubmatch(value)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}

func normalizeWeeksToDays(value string) string {
	re := regexp.MustCompile(`(\d+)\s+weeks?(?:,\s*(\d+)\s+days?)?`)
	return re.ReplaceAllStringFunc(value, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 0 {
			return match
		}
		weeks, _ := strconv.Atoi(parts[1])
		days := 0
		if len(parts) > 2 && parts[2] != "" {
			days, _ = strconv.Atoi(parts[2])
		}
		return fmt.Sprintf("%d days", weeks*7+days)
	})
}

func bytesHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(value)
	unit := 0
	for f >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f%s", f, units[unit])
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func bytesPair(used uint64, total uint64) string {
	if used == 0 && total == 0 {
		return ""
	}
	return fmt.Sprintf("%s/%s", bytesHuman(used), bytesHuman(total))
}

func diskMountLabel(metrics monitor.Metrics) string {
	mountpoint := strings.TrimSpace(metrics.DiskMountpoint)
	if mountpoint == "" {
		return "-"
	}
	return mountpoint
}

func diskMetricLabel(metrics monitor.Metrics) string {
	mountpoint := diskMountLabel(metrics)
	if mountpoint == "-" || mountpoint == "/" {
		return "磁盘"
	}
	return "磁盘" + mountpoint
}

func diskMountPercentText(metrics monitor.Metrics) string {
	label := diskMountLabel(metrics)
	percent := metricValueStyle(metrics.DiskPercent(), 80, 90).Render(fmt.Sprintf("%.0f%%", metrics.DiskPercent()))
	if label == "-" {
		return percent
	}
	return fit(label+" "+percent, 18)
}

func diskSummaryText(metrics monitor.Metrics) string {
	label := diskMountLabel(metrics)
	size := bytesPair(metrics.DiskUsed, metrics.DiskTotal)
	if label == "-" {
		return size
	}
	if size == "" {
		return label
	}
	return label + " " + size
}

func (m Model) diskListText(metrics monitor.Metrics) string {
	if len(metrics.Disks) == 0 {
		return ""
	}
	disks := append([]monitor.DiskMetric(nil), metrics.Disks...)
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Percent() > disks[j].Percent()
	})
	mountWidth := 8
	for _, disk := range disks {
		if width := ansi.StringWidth(emptyDash(disk.Mountpoint)); width > mountWidth {
			mountWidth = width
		}
	}
	rows := []string{"", "分区"}
	for i, disk := range disks {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, m.diskPartitionInfoLine(i+1, disk, mountWidth))
		rows = append(rows, m.diskPartitionUsageLine(disk))
	}
	rows = append(rows, "")
	return strings.Join(rows, "\n")
}

func (m Model) diskPartitionInfoLine(index int, disk monitor.DiskMetric, mountWidth int) string {
	width := m.detailContentWidth()
	indexText := detailLabelStyle.Render(fmt.Sprintf("%02d", index))
	mount := emptyDash(disk.Mountpoint)
	filesystem := strings.TrimSpace(disk.Filesystem)
	diskType := strings.TrimSpace(disk.Type)
	if filesystem == "" {
		filesystem = "-"
	}
	if diskType == "" {
		diskType = "-"
	}
	suffixRaw := "  类型 " + diskType
	if mountWidth > 24 {
		mountWidth = 24
	}
	prefixRaw := fmt.Sprintf("%02d  %s  设备 ", index, padVisible(fit(mount, mountWidth), mountWidth))
	filesystemWidth := width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	if filesystemWidth < 12 {
		mountWidth = width - ansi.StringWidth(fmt.Sprintf("%02d  ", index)) - ansi.StringWidth("  设备 ") - ansi.StringWidth(suffixRaw) - 12
		if mountWidth < 8 {
			mountWidth = 8
		}
		mount = fit(mount, mountWidth)
		prefixRaw = fmt.Sprintf("%02d  %s  设备 ", index, padVisible(mount, mountWidth))
		filesystemWidth = width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	}
	if filesystemWidth < 8 {
		filesystemWidth = 8
	}
	mount = padVisible(fit(mount, mountWidth), mountWidth)
	line := indexText +
		"  " + detailValueStyle.Render(mount) +
		"  " + mutedStyle.Render("设备") + " " + detailValueStyle.Render(fit(filesystem, filesystemWidth)) +
		"  " + mutedStyle.Render("类型") + " " + detailValueStyle.Render(diskType)
	if ansi.StringWidth(line) > width {
		return fitANSI(line, width)
	}
	return line
}

func (m Model) diskPartitionUsageLine(disk monitor.DiskMetric) string {
	parts := []string{percentBarWithThreshold(disk.Percent(), 80, 90)}
	if size := bytesPair(disk.Used, disk.Total); size != "" {
		parts = append(parts, detailValueStyle.Render(size))
	}
	if disk.AvailKnown {
		parts = append(parts, mutedStyle.Render("可用")+" "+detailValueStyle.Render(bytesHuman(disk.Available)))
	}
	line := strings.Repeat(" ", 10) + strings.Join(parts, "  ")
	if ansi.StringWidth(line) > m.detailContentWidth() {
		return fitANSI(line, m.detailContentWidth())
	}
	return line
}

func swapUsageText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "未配置"
	}
	return fmt.Sprintf("%s  %s / %s", percentBar(metrics.SwapPercent()), bytesHuman(metrics.SwapUsed), bytesHuman(metrics.SwapTotal))
}

func swapFreeText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "-"
	}
	return bytesHuman(metrics.SwapFree)
}

func inodeUsageText(metrics monitor.Metrics) string {
	if metrics.InodeTotal == 0 && metrics.InodeUsed == 0 && metrics.InodeAvailable == 0 {
		return "-"
	}
	return fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.InodePercent(), 80, 90), countHuman(metrics.InodeUsed), countHuman(metrics.InodeTotal))
}

func countHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"", "K", "M", "B"}
	f := float64(value)
	unit := 0
	for f >= 1000 && unit < len(units)-1 {
		f /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d核", metrics.CPUCores)
}

func fit(s string, width int) string {
	if runewidth.StringWidth(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && runewidth.StringWidth(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func fitANSI(s string, width int) string {
	return ansi.Truncate(s, width, "…")
}
