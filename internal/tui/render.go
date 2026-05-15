package tui

import (
	"encoding/json"
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

var asciiModeEnabled bool

func setASCIIMode(enabled bool) {
	asciiModeEnabled = enabled
}

func (m Model) renderDashboardHelp(width int) string {
	if width < 1 {
		width = 1
	}
	parts := []string{
		"More ?",
		"Move ↑↓←→/hjkl",
		"Login Enter",
		"Details Space",
		"Command m",
		"Batch b",
		"History i",
		"Transfer y",
		"Deploy g",
		"Resources n",
		"Settings .",
		"Overview w",
		"View z",
		"Pin t",
		"Favorite f",
		"Favorites v",
		"Add a",
		"Copy c",
		"Edit e",
		"Delete x",
		"Upload u",
		"Download d",
		"Refresh r",
		"Search /",
		"Category Tab",
		"Online o",
		"Problems p",
		"Sort s",
		"Quit q",
	}
	if m.isChineseUI() {
		parts = []string{
			"更多 ?",
			"移动 ↑↓←→/hjkl",
			"登录 Enter",
			"详情 Space",
			"命令 m",
			"批量 b",
			"历史 i",
			"传输 y",
			"部署 g",
			"资源 n",
			"设置 .",
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
		}
	}
	help := strings.Join(parts, "  ")
	return helpStyle.Render(fit(help, width))
}

func (m Model) renderAddForm() string {
	title := m.t("Add Server", "添加服务器")
	if m.editing {
		title = m.t("Edit Server", "编辑服务器")
	} else if m.copying {
		title = m.t("Copy Server", "复制服务器")
	}
	width := formContentWidth(m.width)
	if m.useSingleFormPane(width) {
		width = detailFrameWidth(m.width)
	}
	help := m.t("Switch Tab  Move ↑↓  Category ←→  Save Enter  Back Esc", "切换 Tab  选择 ↑↓  分类 ←→  保存 Enter  返回 Esc")
	if m.formPane == 1 {
		help = m.t("Back Tab  Move ↑↓  New n  Rename r  Delete x  Back Esc", "切回 Tab  选择 ↑↓  新增 n  重命名 r  删除 x  返回 Esc")
		if m.addingCategory {
			help = m.t("Add Enter  Back Esc", "添加 Enter  返回 Esc")
		} else if m.renamingCategory {
			help = m.t("Rename Enter  Back Esc", "重命名 Enter  返回 Esc")
		}
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if m.hasErrorText(m.status) {
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
	filledChar, emptyChar := "▰", "▱"
	if asciiModeEnabled {
		filledChar, emptyChar = "#", "-"
	}
	bar := style.Render(strings.Repeat(filledChar, filled)) + barEmptyStyle.Render(strings.Repeat(emptyChar, total-filled))
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

func (m Model) cardHeaderMeta(h host.Host, metrics monitor.Metrics) string {
	if strings.TrimSpace(h.ExpireAt) != "" {
		return m.expireCardText(h.ExpireAt)
	}
	return cardMutedStyle.Render(m.cardUptimeShort(metrics.Uptime))
}

func (m Model) expireCardText(value string) string {
	if m.isChineseUI() {
		return expireCardText(value)
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("Bad expiry")
	}
	switch {
	case days < 0:
		return redStyle.Render("Expired")
	case days == 0:
		return redStyle.Render("Expires today")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("Exp %dd", days))
	default:
		return cardMutedStyle.Render(fmt.Sprintf("Exp %dd", days))
	}
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

func (m Model) expireDetailText(value string) string {
	if m.isChineseUI() {
		return expireDetailText(value)
	}
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("Invalid")
	}
	switch {
	case days < 0:
		return redStyle.Render(fmt.Sprintf("Expired %dd", -days))
	case days == 0:
		return redStyle.Render("Expires today")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("%dd left", days))
	default:
		return fmt.Sprintf("%dd left", days)
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
	filledChar, emptyChar := "▰", "▱"
	if asciiModeEnabled {
		filledChar, emptyChar = "#", "-"
	}
	bar := style.Render(strings.Repeat(filledChar, filled)) + barEmptyStyle.Render(strings.Repeat(emptyChar, total-filled))
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

func (m Model) authText(h host.Host) string {
	hasKey := strings.TrimSpace(h.IdentityFile) != ""
	hasPassword := h.HasPassword || strings.TrimSpace(h.Password) != ""
	switch {
	case hasKey && hasPassword:
		return m.t("Key: ", "密钥：") + filepath.Base(h.IdentityFile) + m.t(", password", "，密码")
	case hasKey:
		return m.t("Key: ", "密钥：") + filepath.Base(h.IdentityFile)
	case hasPassword:
		return m.t("Password", "密码")
	default:
		return m.t("System SSH default", "系统 SSH 默认")
	}
}

func (m Model) jumpDetailText(h host.Host) string {
	if !h.JumpEnabled {
		return m.t("Disabled", "未启用")
	}
	if strings.TrimSpace(h.JumpHostRef) != "" {
		return h.JumpHostRef + m.t(", forwarding only, local key auth", "，仅转发，本地密钥认证")
	}
	port := strings.TrimSpace(h.JumpPort)
	if port == "" {
		port = "22"
	}
	target := h.JumpHost
	if strings.TrimSpace(h.JumpUser) != "" {
		target = h.JumpUser + "@" + target
	}
	return target + ":" + port + m.t(", forwarding only, local key auth", "，仅转发，本地密钥认证")
}

func (m Model) jumpKeyText(h host.Host) string {
	if !h.JumpEnabled {
		return "-"
	}
	if strings.TrimSpace(h.JumpKeyPath) == "" {
		return m.t("System SSH default", "系统 SSH 默认")
	}
	return filepath.Base(h.JumpKeyPath) + m.t(" (local)", "（本地）")
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

func (m Model) cardUptimeShort(value string) string {
	if m.isChineseUI() {
		return cardUptimeShort(value)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%dm", minutes)
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

func (m Model) lastLoginDetail(value time.Time) string {
	if m.isChineseUI() {
		return lastLoginDetail(value)
	}
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04") + " (" + m.lastLoginCard(value) + ")"
}

func (m Model) uptimeText(value string) string {
	if m.isChineseUI() {
		return uptimeCN(value)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	parts := []string{}
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		return m.cardUptimeShort(value)
	}
	return strings.Join(parts, " ")
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

func (m Model) lastLoginCard(value time.Time) string {
	if m.isChineseUI() {
		return lastLoginCard(value)
	}
	if value.IsZero() {
		return ""
	}
	relative := m.relativeTime(value)
	if relative == "now" {
		return relative
	}
	return relative + " ago"
}

func (m Model) relativeTime(value time.Time) string {
	if m.isChineseUI() {
		return relativeTime(value)
	}
	if value.IsZero() {
		return "-"
	}
	d := time.Since(value)
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "now"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", days/365)
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
units=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>/dev/null | awk '$1 ~ /\.service$/ {print $1}')
if [ -z "$units" ]; then
  out=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
  code=$?
  if [ "$code" -ne 0 ]; then
    echo "__SSHM_SYSTEMCTL_ERROR__"
    printf '%s\n' "$out"
    exit 0
  fi
  printf '%s\n' "$out"
  exit 0
fi
parsed=$(for unit in $units; do
  props=$(systemctl show "$unit" -p Id -p LoadState -p ActiveState -p SubState -p Description -p FragmentPath -p WorkingDirectory -p ExecStart -p MainPID -p ExecMainPID -p MemoryCurrent -p ActiveEnterTimestamp -p InactiveEnterTimestamp -p StateChangeTimestamp -p ExecMainStartTimestamp -p ExecMainExitTimestamp -p UnitFileState --no-pager 2>/dev/null)
  [ -n "$props" ] || continue
  get_prop() { printf '%s\n' "$props" | awk -F= -v key="$1" '$1==key{print substr($0, index($0,"=")+1); exit}'; }
  id=$(get_prop Id)
  [ -n "$id" ] || continue
  load=$(get_prop LoadState)
  active=$(get_prop ActiveState)
  sub=$(get_prop SubState)
  desc=$(get_prop Description)
  fragment=$(get_prop FragmentPath)
  workdir=$(get_prop WorkingDirectory)
  execstart=$(get_prop ExecStart)
  mainpid=$(get_prop MainPID)
  execmainpid=$(get_prop ExecMainPID)
  memorycurrent=$(get_prop MemoryCurrent)
  activeenter=$(get_prop ActiveEnterTimestamp)
  inactiveenter=$(get_prop InactiveEnterTimestamp)
  statechange=$(get_prop StateChangeTimestamp)
  execmainstart=$(get_prop ExecMainStartTimestamp)
  execmainexit=$(get_prop ExecMainExitTimestamp)
  unitfilestate=$(get_prop UnitFileState)
  pid="$mainpid"
  [ -n "$pid" ] && [ "$pid" != "0" ] || pid="$execmainpid"
  if { [ -z "$memorycurrent" ] || [ "$memorycurrent" = "[not set]" ] || [ "$memorycurrent" = "18446744073709551615" ]; } && [ -n "$pid" ] && [ "$pid" != "0" ]; then
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | awk 'NR==1{print $1}')
    case "$rss" in
      ''|*[!0-9]*) ;;
      *) memorycurrent=$((rss * 1024)) ;;
    esac
  fi
  if [ -z "$activeenter" ] || [ "$activeenter" = "n/a" ]; then
    activeenter="$execmainstart"
  fi
  if [ -z "$activeenter" ] || [ "$activeenter" = "n/a" ]; then
    activeenter="$statechange"
  fi
  printf "__SSHM_SERVICE__\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" "$id" "$load" "$active" "$sub" "$desc" "$fragment" "$workdir" "$execstart" "$mainpid" "$execmainpid" "$memorycurrent" "$activeenter" "$inactiveenter" "$statechange" "$execmainstart" "$execmainexit" "$unitfilestate"
done)
if [ -n "$parsed" ]; then
  printf '%s\n' "$parsed"
  exit 0
fi
fallback=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
fallback_code=$?
if [ "$fallback_code" -ne 0 ]; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$fallback"
exit 0`
}

func serviceListScript() string {
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
printf '%s\n' "$out"
exit 0`
}

func serviceExtraDetailScript(unit string) string {
	quoted := shellQuoteLocal(unit)
	return fmt.Sprintf(`if ! command -v systemctl >/dev/null 2>&1; then
  echo "__SSHM_SYSTEMCTL_UNAVAILABLE__"
  exit 0
fi
props=$(systemctl show %s -p Id -p LoadState -p ActiveState -p SubState -p Description -p FragmentPath -p WorkingDirectory -p ExecStart -p ExecStop -p ExecReload -p MainPID -p ExecMainPID -p MemoryCurrent -p ActiveEnterTimestamp -p InactiveEnterTimestamp -p StateChangeTimestamp -p ExecMainStartTimestamp -p ExecMainExitTimestamp -p UnitFileState -p Result -p ExecMainStatus -p NRestarts -p TasksCurrent -p ControlGroup -p Slice -p User -p Group -p Restart -p RestartUSec -p DropInPaths --no-pager 2>&1)
code=$?
if [ "$code" -ne 0 ] && ! printf '%%s\n' "$props" | grep -q '^Id='; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%%s\n' "$props"
  exit 0
fi
get_prop() { printf '%%s\n' "$props" | awk -F= -v key="$1" '$1==key{print substr($0, index($0,"=")+1); exit}'; }
printf '%%s\n' "$props"`, quoted)
}

func portDetailScript() string {
	return `run_ports() {
  if command -v ss >/dev/null 2>&1; then
    ss -H -tulnp 2>&1 || ss -tulnp 2>&1
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -tulnp 2>&1
    return $?
  fi
  return 127
}
run_ports_sudo() {
  if command -v ss >/dev/null 2>&1; then
    sudo -n ss -H -tulnp 2>&1 || sudo -n ss -tulnp 2>&1
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    sudo -n netstat -tulnp 2>&1
    return $?
  fi
  return 127
}
if ! command -v ss >/dev/null 2>&1 && ! command -v netstat >/dev/null 2>&1; then
  echo "__SSHM_SS_UNAVAILABLE__"
  exit 0
fi
out=$(run_ports)
code=$?
if [ "$code" -eq 0 ] && ! printf '%s\n' "$out" | grep -q 'users:(' && ! printf '%s\n' "$out" | grep -Eq '[0-9]+/[^[:space:]]+'; then
  sudo_out=$(run_ports_sudo)
  sudo_code=$?
  if [ "$sudo_code" -eq 0 ]; then
    out="$sudo_out"
  fi
fi
if [ "$code" -ne 0 ]; then
  sudo_out=$(run_ports_sudo)
  sudo_code=$?
  if [ "$sudo_code" -ne 0 ]; then
    echo "__SSHM_SS_PERMISSION__"
    printf '%s\n' "$sudo_out"
    exit 0
  fi
  out="$sudo_out"
fi
printf '%s\n' "$out"
printf '%s\n' "$out" | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p; s/.*[[:space:]]\([0-9][0-9]*\)\/[^[:space:]]*.*/\1/p' | sort -u | while read -r pid; do
  [ -n "$pid" ] || continue
  unit=$(cat "/proc/$pid/cgroup" 2>/dev/null | sed -n 's|.*[:/]\([^/:]*\.service\).*|\1|p' | head -n 1)
  [ -n "$unit" ] && printf '__SSHM_PORT_CGROUP__\t%s\t%s\n' "$pid" "$unit"
done`
}

func processExtraDetailScript(pid string) string {
	quoted := shellQuoteLocal(pid)
	return fmt.Sprintf(`pid=%s
case "$pid" in
  ''|*[!0-9]*)
    echo "__SSHM_PROCESS_INVALID__"
    exit 0
    ;;
esac
if [ ! -d "/proc/$pid" ]; then
  echo "__SSHM_PROCESS_NOT_FOUND__"
  exit 0
fi
ps_value() {
  ps -p "$pid" -o "$1=" 2>/dev/null | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | head -n 1
}
ppid=$(ps_value ppid)
user=$(ps_value user)
state=$(ps_value stat)
cpu=$(ps_value pcpu)
mem=$(ps_value pmem)
rss=$(ps_value rss)
elapsed=$(ps_value etime)
started=$(ps_value lstart)
comm=$(ps_value comm)
cmdline=$(tr '\000' ' ' <"/proc/$pid/cmdline" 2>/dev/null | sed -e 's/[[:space:]]*$//')
[ -z "$cmdline" ] && cmdline=$(ps_value args)
cwd=$(readlink "/proc/$pid/cwd" 2>/dev/null || true)
exe=$(readlink "/proc/$pid/exe" 2>/dev/null || true)
cgroup=$(cat "/proc/$pid/cgroup" 2>/dev/null | paste -sd ';' -)
unit=$(printf '%%s\n' "$cgroup" | sed -n 's|.*[:/]\([^/:;]*\.service\).*|\1|p' | head -n 1)
printf '__SSHM_PROCESS__\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\n' "$pid" "$ppid" "$user" "$state" "$cpu" "$mem" "$rss" "$elapsed" "$started" "$comm" "$cmdline" "$cwd" "$exe" "$cgroup" "$unit"`, quoted)
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
printf '%s\n' "$out" | while IFS= read -r line; do
  [ -z "$line" ] && continue
  printf '__SSHM_CONTAINER__\t%s\n' "$line"
done
stats=$(docker stats --no-stream --format '{{.Name}}	{{.CPUPerc}}	{{.MemUsage}}	{{.MemPerc}}' 2>/dev/null)
stats_code=$?
if [ "$stats_code" -ne 0 ]; then
  stats=$(sudo -n docker stats --no-stream --format '{{.Name}}	{{.CPUPerc}}	{{.MemUsage}}	{{.MemPerc}}' 2>/dev/null)
  stats_code=$?
fi
if [ "$stats_code" -eq 0 ]; then
  printf '%s\n' "$stats" | while IFS= read -r line; do
    [ -z "$line" ] && continue
    printf '__SSHM_CONTAINER_STATS__\t%s\n' "$line"
  done
fi
ids=$(docker ps -aq 2>/dev/null)
limits_code=$?
if [ "$limits_code" -ne 0 ]; then
  ids=$(sudo -n docker ps -aq 2>/dev/null)
  limits_code=$?
fi
if [ "$limits_code" -eq 0 ] && [ -n "$ids" ]; then
  limits=$(docker inspect --format '{{.Name}}	{{.HostConfig.NanoCpus}}	{{.HostConfig.CpuQuota}}	{{.HostConfig.CpuPeriod}}	{{.HostConfig.CpusetCpus}}' $ids 2>/dev/null)
  limits_code=$?
  if [ "$limits_code" -ne 0 ]; then
    limits=$(sudo -n docker inspect --format '{{.Name}}	{{.HostConfig.NanoCpus}}	{{.HostConfig.CpuQuota}}	{{.HostConfig.CpuPeriod}}	{{.HostConfig.CpusetCpus}}' $ids 2>/dev/null)
    limits_code=$?
  fi
  if [ "$limits_code" -eq 0 ]; then
    printf '%s\n' "$limits" | while IFS= read -r line; do
      [ -z "$line" ] && continue
      printf '__SSHM_CONTAINER_LIMIT__\t%s\n' "$line"
    done
  fi
fi`
}

func containerExtraDetailScript(name string) string {
	quoted := shellQuoteLocal(name)
	filter := shellQuoteLocal("name=^/" + name + "$")
	return fmt.Sprintf(`if ! command -v docker >/dev/null 2>&1; then
  echo "__SSHM_DOCKER_UNAVAILABLE__"
  exit 0
fi
run_docker() {
  docker "$@" 2>&1
}
run_docker_sudo() {
  sudo -n docker "$@" 2>&1
}
inspect=$(run_docker inspect --size --format '{{json .}}' %s)
code=$?
if [ "$code" -ne 0 ]; then
  inspect=$(run_docker_sudo inspect --size --format '{{json .}}' %s)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_DOCKER_PERMISSION__"
  printf '%%s\n' "$inspect"
  exit 0
fi
printf '__SSHM_CONTAINER_INSPECT__\t%%s\n' "$inspect"
size=$(run_docker ps -a --filter %s --size --format '{{.Size}}')
code=$?
if [ "$code" -ne 0 ]; then
  size=$(run_docker_sudo ps -a --filter %s --size --format '{{.Size}}')
fi
[ -n "$size" ] && printf '__SSHM_CONTAINER_SIZE__\t%%s\n' "$size"
blockio=$(run_docker stats --no-stream --format '{{.BlockIO}}' %s)
code=$?
if [ "$code" -ne 0 ]; then
  blockio=$(run_docker_sudo stats --no-stream --format '{{.BlockIO}}' %s)
fi
[ -n "$blockio" ] && printf '__SSHM_CONTAINER_BLOCKIO__\t%%s\n' "$blockio"`, quoted, quoted, filter, filter, quoted, quoted)
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
	errText := ""
	items := []serviceDetail{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "UNIT ") || strings.HasPrefix(trimmed, "LOAD ") {
			continue
		}
		if strings.HasPrefix(trimmed, "__SSHM_SYSTEMCTL_ERROR__") {
			errText = strings.TrimSpace(strings.ReplaceAll(output, "__SSHM_SYSTEMCTL_ERROR__", ""))
			continue
		}
		if strings.HasPrefix(line, "__SSHM_SERVICE__\t") {
			parts := strings.Split(line, "\t")
			for len(parts) < 31 {
				parts = append(parts, "")
			}
			if len(parts) < 9 || !strings.HasSuffix(parts[1], ".service") {
				continue
			}
			item := serviceDetail{
				Unit:             strings.TrimSpace(parts[1]),
				Load:             strings.TrimSpace(parts[2]),
				Active:           strings.TrimSpace(parts[3]),
				Sub:              strings.TrimSpace(parts[4]),
				Description:      strings.TrimSpace(parts[5]),
				FragmentPath:     strings.TrimSpace(parts[6]),
				WorkingDirectory: strings.TrimSpace(parts[7]),
				ExecStart:        strings.TrimSpace(parts[8]),
			}
			if len(parts) >= 12 {
				item.MainPID = normalizePID(parts[9])
				item.ExecMainPID = normalizePID(parts[10])
				item.MemoryCurrent = parseSystemdMemoryCurrent(parts[11])
			}
			if len(parts) >= 13 {
				item.ActiveSince = strings.TrimSpace(parts[12])
			}
			if len(parts) >= 31 {
				item.InactiveSince = strings.TrimSpace(parts[13])
				item.StateChangedAt = strings.TrimSpace(parts[14])
				item.ExecStartedAt = strings.TrimSpace(parts[15])
				item.ExecExitedAt = strings.TrimSpace(parts[16])
				item.UnitFileState = strings.TrimSpace(parts[17])
				item.Result = strings.TrimSpace(parts[18])
				item.ExecMainStatus = strings.TrimSpace(parts[19])
				item.NRestarts = strings.TrimSpace(parts[20])
				item.TasksCurrent = strings.TrimSpace(parts[21])
				item.ControlGroup = strings.TrimSpace(parts[22])
				item.Slice = strings.TrimSpace(parts[23])
				item.User = strings.TrimSpace(parts[24])
				item.Group = strings.TrimSpace(parts[25])
				item.Restart = strings.TrimSpace(parts[26])
				item.RestartSec = strings.TrimSpace(parts[27])
				item.ExecStop = strings.TrimSpace(parts[28])
				item.ExecReload = strings.TrimSpace(parts[29])
				item.DropInPaths = strings.TrimSpace(parts[30])
			}
			items = append(items, item)
			continue
		}
		fields := strings.Fields(trimmed)
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
	if len(items) == 0 && errText != "" {
		return nil, errText
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

func parseServiceExtraDetail(output string) (serviceDetail, string) {
	if strings.Contains(output, "__SSHM_SYSTEMCTL_UNAVAILABLE__") {
		return serviceDetail{}, "systemctl不可用"
	}
	errText := ""
	props := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "__SSHM_SYSTEMCTL_ERROR__") {
			errText = strings.TrimSpace(strings.ReplaceAll(output, "__SSHM_SYSTEMCTL_ERROR__", ""))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if props["Id"] == "" {
		items, parsedErr := parseServiceDetails(output)
		if len(items) > 0 {
			return items[0], ""
		}
		if parsedErr != "" {
			return serviceDetail{}, parsedErr
		}
		return serviceDetail{}, errText
	}
	item := serviceDetail{
		Unit:             props["Id"],
		Load:             props["LoadState"],
		Active:           props["ActiveState"],
		Sub:              props["SubState"],
		Description:      props["Description"],
		FragmentPath:     props["FragmentPath"],
		WorkingDirectory: props["WorkingDirectory"],
		ExecStart:        props["ExecStart"],
		MainPID:          normalizePID(props["MainPID"]),
		ExecMainPID:      normalizePID(props["ExecMainPID"]),
		MemoryCurrent:    parseSystemdMemoryCurrent(props["MemoryCurrent"]),
		ActiveSince:      props["ActiveEnterTimestamp"],
		InactiveSince:    props["InactiveEnterTimestamp"],
		StateChangedAt:   props["StateChangeTimestamp"],
		ExecStartedAt:    props["ExecMainStartTimestamp"],
		ExecExitedAt:     props["ExecMainExitTimestamp"],
		UnitFileState:    props["UnitFileState"],
		Result:           props["Result"],
		ExecMainStatus:   props["ExecMainStatus"],
		NRestarts:        props["NRestarts"],
		TasksCurrent:     props["TasksCurrent"],
		ControlGroup:     props["ControlGroup"],
		Slice:            props["Slice"],
		User:             props["User"],
		Group:            props["Group"],
		Restart:          props["Restart"],
		RestartSec:       props["RestartUSec"],
		ExecStop:         props["ExecStop"],
		ExecReload:       props["ExecReload"],
		DropInPaths:      props["DropInPaths"],
	}
	return item, ""
}

func parsePortDetails(output string) ([]portDetail, string) {
	if strings.Contains(output, "__SSHM_SS_UNAVAILABLE__") {
		return nil, "ss不可用"
	}
	if strings.Contains(output, "__SSHM_SS_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n ss）"
	}
	lines := strings.Split(output, "\n")
	cgroups := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "__SSHM_PORT_CGROUP__\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			cgroups[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[2])
		}
	}
	grouped := map[string]*portDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "__SSHM_PORT_CGROUP__\t") || strings.HasPrefix(line, "Netid") || strings.HasPrefix(line, "Proto") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		info := portLineInfo(fields)
		port := portFromAddress(info.LocalAddress)
		if port == "" || port == "*" {
			continue
		}
		process, pid, fd := processFromSS(info.ProcessText)
		serviceUnit := cgroups[strings.TrimSpace(pid)]
		protocol := normalizePortProtocol(fields[0])
		key := strings.Join([]string{protocol, port, info.State, process, serviceUnit}, "/")
		if item, ok := grouped[key]; ok {
			item.Count++
			item.LocalAddress = appendUniqueCSV(item.LocalAddress, info.LocalAddress)
			item.ForeignAddress = appendUniqueCSV(item.ForeignAddress, info.ForeignAddress)
			item.PID = appendUniqueCSV(item.PID, pid)
			item.FD = appendUniqueCSV(item.FD, fd)
			item.ServiceUnit = appendUniqueCSV(item.ServiceUnit, serviceUnit)
			continue
		}
		grouped[key] = &portDetail{
			Protocol:       protocol,
			Port:           port,
			LocalAddress:   info.LocalAddress,
			ForeignAddress: info.ForeignAddress,
			State:          info.State,
			Process:        process,
			PID:            pid,
			FD:             fd,
			ServiceUnit:    serviceUnit,
			Count:          1,
		}
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

func normalizePortProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "tcp6":
		return "tcp"
	case "udp6":
		return "udp"
	default:
		return value
	}
}

func appendUniqueCSV(current string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return current
	}
	values := splitCSVValues(current)
	for _, existing := range values {
		if existing == value {
			return current
		}
	}
	values = append(values, value)
	return strings.Join(values, ", ")
}

func splitCSVValues(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseProcessExtraDetail(output string) (processExtraDetail, string) {
	if strings.Contains(output, "__SSHM_PROCESS_INVALID__") {
		return processExtraDetail{}, "PID无效"
	}
	if strings.Contains(output, "__SSHM_PROCESS_NOT_FOUND__") {
		return processExtraDetail{}, "进程不存在"
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "__SSHM_PROCESS__\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < 16 {
			parts = append(parts, "")
		}
		return processExtraDetail{
			PID:          strings.TrimSpace(parts[1]),
			PPID:         strings.TrimSpace(parts[2]),
			User:         strings.TrimSpace(parts[3]),
			State:        strings.TrimSpace(parts[4]),
			CPU:          strings.TrimSpace(parts[5]),
			Memory:       strings.TrimSpace(parts[6]),
			RSS:          strings.TrimSpace(parts[7]),
			Elapsed:      strings.TrimSpace(parts[8]),
			Started:      strings.TrimSpace(parts[9]),
			Command:      strings.TrimSpace(parts[10]),
			CommandLine:  strings.TrimSpace(parts[11]),
			WorkingDir:   strings.TrimSpace(parts[12]),
			Executable:   strings.TrimSpace(parts[13]),
			ControlGroup: strings.TrimSpace(parts[14]),
			ServiceUnit:  strings.TrimSpace(parts[15]),
		}, ""
	}
	return processExtraDetail{}, ""
}

type portLineParseInfo struct {
	LocalAddress   string
	ForeignAddress string
	State          string
	ProcessText    string
}

func portLineInfo(fields []string) portLineParseInfo {
	processStart := len(fields)
	for i, field := range fields {
		if strings.HasPrefix(field, "users:") || strings.Contains(field, "users:(") {
			processStart = i
			break
		}
	}
	if processStart == len(fields) {
		for i := len(fields) - 1; i >= 0; i-- {
			if netstatProcessField(fields[i]) {
				processStart = i
				break
			}
		}
	}
	processText := ""
	if processStart < len(fields) {
		processText = strings.Join(fields[processStart:], " ")
	}
	limit := processStart
	if limit > len(fields) {
		limit = len(fields)
	}
	info := portLineParseInfo{ProcessText: processText}
	if len(fields) > 1 && !numericText(fields[1]) {
		info.State = fields[1]
	}
	localIndex := -1
	for i := 3; i < limit; i++ {
		port := portFromAddress(fields[i])
		if port == "" || port == "*" {
			continue
		}
		info.LocalAddress = fields[i]
		localIndex = i
		break
	}
	if localIndex >= 0 && localIndex+1 < limit {
		next := fields[localIndex+1]
		if portFromAddress(next) != "" {
			info.ForeignAddress = next
		}
	}
	if info.State == "" && localIndex >= 0 {
		for i := localIndex + 1; i < limit; i++ {
			field := fields[i]
			if portFromAddress(field) != "" || numericText(field) || netstatProcessField(field) {
				continue
			}
			info.State = field
			break
		}
	}
	return info
}

func numericText(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func netstatProcessField(value string) bool {
	if value == "-" {
		return true
	}
	pid, _, ok := strings.Cut(value, "/")
	if !ok || pid == "" {
		return false
	}
	for _, r := range pid {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

func processFromSS(value string) (string, string, string) {
	name := ""
	pid := ""
	fd := ""
	for _, field := range strings.Fields(value) {
		if field == "-" {
			return "", "", ""
		}
		left, right, ok := strings.Cut(field, "/")
		if ok && left != "" && right != "" {
			digits := true
			for _, r := range left {
				if r < '0' || r > '9' {
					digits = false
					break
				}
			}
			if digits {
				return right, left, ""
			}
		}
	}
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
	if idx := strings.Index(value, "fd="); idx >= 0 {
		rest := value[idx+3:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		fd = rest[:end]
	}
	return name, pid, fd
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
	stats := map[string]containerDetail{}
	limits := map[string]containerDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "__SSHM_CONTAINER_STATS__\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 5 {
				name := strings.TrimSpace(parts[1])
				stats[name] = containerDetail{
					Name:    name,
					CPU:     strings.TrimSpace(parts[2]),
					Memory:  normalizeDockerMemory(strings.TrimSpace(parts[3])),
					MemPerc: strings.TrimSpace(parts[4]),
				}
			}
			continue
		}
		if strings.HasPrefix(line, "__SSHM_CONTAINER_LIMIT__\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 5 {
				name := strings.TrimPrefix(strings.TrimSpace(parts[1]), "/")
				limits[name] = containerDetail{
					Name:          name,
					CPULimitKnown: true,
					NanoCpus:      parseContainerLimitInt(parts[2]),
					CPUQuota:      parseContainerLimitInt(parts[3]),
					CPUPeriod:     parseContainerLimitInt(parts[4]),
				}
				if len(parts) >= 6 {
					limit := limits[name]
					limit.CpusetCpus = strings.TrimSpace(parts[5])
					limits[name] = limit
				}
			}
			continue
		}
		line = strings.TrimPrefix(line, "__SSHM_CONTAINER__\t")
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
		if stat, ok := stats[item.Name]; ok {
			item.CPU = stat.CPU
			item.Memory = stat.Memory
			item.MemPerc = stat.MemPerc
		}
		if limit, ok := limits[item.Name]; ok {
			item.CPULimitKnown = limit.CPULimitKnown
			item.NanoCpus = limit.NanoCpus
			item.CPUQuota = limit.CPUQuota
			item.CPUPeriod = limit.CPUPeriod
			item.CpusetCpus = limit.CpusetCpus
		}
		if item.Name != "" {
			out = append(out, item)
		}
	}
	for i := range out {
		if stat, ok := stats[out[i].Name]; ok {
			out[i].CPU = stat.CPU
			out[i].Memory = stat.Memory
			out[i].MemPerc = stat.MemPerc
		}
		if limit, ok := limits[out[i].Name]; ok {
			out[i].CPULimitKnown = limit.CPULimitKnown
			out[i].NanoCpus = limit.NanoCpus
			out[i].CPUQuota = limit.CPUQuota
			out[i].CPUPeriod = limit.CPUPeriod
			out[i].CpusetCpus = limit.CpusetCpus
		}
	}
	return out, ""
}

func parseContainerLimitInt(value string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func parseContainerExtraDetail(output string) (containerExtraDetail, string) {
	if strings.Contains(output, "__SSHM_DOCKER_UNAVAILABLE__") {
		return containerExtraDetail{}, "未安装Docker"
	}
	if strings.Contains(output, "__SSHM_DOCKER_PERMISSION__") {
		return containerExtraDetail{}, "需要Docker权限（可配置sudo -n docker）"
	}
	detail := containerExtraDetail{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "__SSHM_CONTAINER_INSPECT__\t"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_INSPECT__\t"))
			if err := applyContainerInspectJSON(raw, &detail); err != nil {
				return detail, err.Error()
			}
		case strings.HasPrefix(line, "__SSHM_CONTAINER_SIZE__\t"):
			size := strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_SIZE__\t"))
			detail.Size, detail.VirtualSize = splitDockerSize(size)
		case strings.HasPrefix(line, "__SSHM_CONTAINER_BLOCKIO__\t"):
			detail.BlockIO = normalizeDockerMemory(strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_BLOCKIO__\t")))
		}
	}
	return detail, ""
}

func applyContainerInspectJSON(raw string, detail *containerExtraDetail) error {
	type inspectItem struct {
		ID         string   `json:"Id"`
		Created    string   `json:"Created"`
		Path       string   `json:"Path"`
		Args       []string `json:"Args"`
		Driver     string   `json:"Driver"`
		Platform   string   `json:"Platform"`
		SizeRw     int64    `json:"SizeRw"`
		SizeRootFS int64    `json:"SizeRootFs"`
		State      struct {
			Status     string `json:"Status"`
			StartedAt  string `json:"StartedAt"`
			FinishedAt string `json:"FinishedAt"`
			ExitCode   int    `json:"ExitCode"`
			Health     *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		HostConfig struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
			NanoCpus   int64  `json:"NanoCpus"`
			CPUQuota   int64  `json:"CpuQuota"`
			CPUPeriod  int64  `json:"CpuPeriod"`
			CpusetCpus string `json:"CpusetCpus"`
		} `json:"HostConfig"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress  string   `json:"IPAddress"`
				Gateway    string   `json:"Gateway"`
				MacAddress string   `json:"MacAddress"`
				NetworkID  string   `json:"NetworkID"`
				EndpointID string   `json:"EndpointID"`
				Aliases    []string `json:"Aliases"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	var items []inspectItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		var single inspectItem
		if singleErr := json.Unmarshal([]byte(raw), &single); singleErr != nil {
			return err
		}
		items = []inspectItem{single}
	}
	if len(items) == 0 {
		return nil
	}
	item := items[0]
	detail.ID = item.ID
	detail.Created = item.Created
	detail.Path = item.Path
	detail.Args = item.Args
	detail.Driver = item.Driver
	detail.Platform = item.Platform
	detail.StateStatus = item.State.Status
	detail.StartedAt = item.State.StartedAt
	detail.FinishedAt = item.State.FinishedAt
	detail.ExitCode = item.State.ExitCode
	if item.State.Health != nil {
		detail.HealthStatus = item.State.Health.Status
	}
	detail.RestartPolicy = item.HostConfig.RestartPolicy.Name
	detail.NanoCpus = item.HostConfig.NanoCpus
	detail.CPUQuota = item.HostConfig.CPUQuota
	detail.CPUPeriod = item.HostConfig.CPUPeriod
	detail.CpusetCpus = item.HostConfig.CpusetCpus
	if item.SizeRw > 0 {
		detail.SizeRW = uint64(item.SizeRw)
	}
	if item.SizeRootFS > 0 {
		detail.SizeRootFS = uint64(item.SizeRootFS)
	}
	for _, mount := range item.Mounts {
		detail.Mounts = append(detail.Mounts, containerMountDetail{
			Type:        mount.Type,
			Source:      mount.Source,
			Destination: mount.Destination,
			RW:          mount.RW,
		})
	}
	for name, network := range item.NetworkSettings.Networks {
		detail.Networks = append(detail.Networks, containerNetworkDetail{
			Name:       name,
			IPAddress:  network.IPAddress,
			Gateway:    network.Gateway,
			MacAddress: network.MacAddress,
			NetworkID:  network.NetworkID,
			EndpointID: network.EndpointID,
			Aliases:    network.Aliases,
		})
	}
	sort.Slice(detail.Networks, func(i, j int) bool {
		return detail.Networks[i].Name < detail.Networks[j].Name
	})
	return nil
}

func splitDockerSize(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if left, right, ok := strings.Cut(value, "(virtual "); ok {
		return strings.TrimSpace(left), strings.TrimSuffix(strings.TrimSpace(right), ")")
	}
	return value, ""
}

func normalizePID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return ""
	}
	return value
}

func parseSystemdMemoryCurrent(value string) uint64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "[not set]" || value == "-" {
		return 0
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n == ^uint64(0) {
		return 0
	}
	return n
}

func normalizeDockerMemory(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "GiB", "G")
	value = strings.ReplaceAll(value, "MiB", "M")
	value = strings.ReplaceAll(value, "KiB", "K")
	value = strings.ReplaceAll(value, "TiB", "T")
	value = strings.ReplaceAll(value, " / ", "/")
	value = strings.ReplaceAll(value, "B /", "B/")
	value = strings.ReplaceAll(value, "/ ", "/")
	return value
}

func shortSystemdTimestampAge(value string, chinese bool) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return ""
	}
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -07",
		"Mon 2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
	}
	var parsed time.Time
	ok := false
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			parsed = t
			ok = true
			break
		}
	}
	if !ok {
		fields := strings.Fields(value)
		if len(fields) >= 3 {
			trimmed := strings.Join(fields[:3], " ")
			if t, err := time.ParseInLocation("Mon 2006-01-02 15:04:05", trimmed, time.Local); err == nil {
				parsed = t
				ok = true
			}
		}
	}
	if !ok {
		return ""
	}
	d := time.Since(parsed)
	if d < 0 {
		return ""
	}
	return shortDurationAge(d, chinese)
}

func shortDurationAge(d time.Duration, chinese bool) string {
	switch {
	case d < time.Minute:
		n := int(d.Seconds())
		if n < 1 {
			n = 1
		}
		if chinese {
			return fmt.Sprintf("%d秒", n)
		}
		return fmt.Sprintf("%ds", n)
	case d < time.Hour:
		n := int(d.Minutes())
		if chinese {
			return fmt.Sprintf("%d分", n)
		}
		return fmt.Sprintf("%dm", n)
	case d < 24*time.Hour:
		n := int(d.Hours())
		if chinese {
			return fmt.Sprintf("%d时", n)
		}
		return fmt.Sprintf("%dh", n)
	case d < 7*24*time.Hour:
		n := int(d.Hours() / 24)
		if chinese {
			return fmt.Sprintf("%d天", n)
		}
		return fmt.Sprintf("%dd", n)
	case d < 30*24*time.Hour:
		n := int(d.Hours() / 24 / 7)
		if chinese {
			return fmt.Sprintf("%d周", n)
		}
		return fmt.Sprintf("%dw", n)
	case d < 365*24*time.Hour:
		n := int(d.Hours() / 24 / 30)
		if chinese {
			return fmt.Sprintf("%d月", n)
		}
		return fmt.Sprintf("%dmo", n)
	default:
		n := int(d.Hours() / 24 / 365)
		if chinese {
			return fmt.Sprintf("%d年", n)
		}
		return fmt.Sprintf("%dy", n)
	}
}

func associatePortContainers(ports []portDetail, containers []containerDetail) {
	portMap := containerPublishedPortMap(containers)
	for i := range ports {
		key := strings.ToLower(ports[i].Protocol) + "/" + ports[i].Port
		if mappings := portMap[key]; len(mappings) > 0 {
			names := make([]string, 0, len(mappings))
			targets := make([]string, 0, len(mappings))
			for _, mapping := range mappings {
				if !stringSliceContains(names, mapping.Name) {
					names = append(names, mapping.Name)
				}
				if mapping.Target != "" && !stringSliceContains(targets, mapping.Target) {
					targets = append(targets, mapping.Target)
				}
			}
			ports[i].Container = strings.Join(names, "、")
			ports[i].ContainerPort = strings.Join(targets, "、")
		}
	}
}

type containerPortMapping struct {
	Name   string
	Target string
}

func containerPublishedPortMap(containers []containerDetail) map[string][]containerPortMapping {
	out := map[string][]containerPortMapping{}
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		for _, part := range strings.Split(container.Ports, ",") {
			hostPort, targetPort, proto, ok := parseDockerPublishedPort(part)
			if !ok {
				continue
			}
			key := proto + "/" + hostPort
			exists := false
			for _, existing := range out[key] {
				if existing.Name == name && existing.Target == targetPort {
					exists = true
					break
				}
			}
			if !exists {
				out[key] = append(out[key], containerPortMapping{Name: name, Target: targetPort})
			}
		}
	}
	return out
}

func parseDockerPublishedPort(value string) (string, string, string, bool) {
	value = strings.TrimSpace(value)
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return "", "", "", false
	}
	hostPort := portFromAddress(left)
	if hostPort == "" {
		return "", "", "", false
	}
	proto := "tcp"
	targetPort := strings.TrimSpace(right)
	if idx := strings.LastIndex(right, "/"); idx >= 0 && idx < len(right)-1 {
		proto = strings.ToLower(strings.TrimSpace(right[idx+1:]))
		targetPort = strings.TrimSpace(right[:idx])
	}
	if proto != "tcp" && proto != "udp" {
		proto = "tcp"
	}
	return hostPort, targetPort, proto, true
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

func (m Model) diskMetricLabel(metrics monitor.Metrics) string {
	if m.isChineseUI() {
		return diskMetricLabel(metrics)
	}
	mountpoint := diskMountLabel(metrics)
	if mountpoint == "-" || mountpoint == "/" {
		return "Disk"
	}
	return "Disk" + mountpoint
}

func (m Model) diskMountPercentText(metrics monitor.Metrics) string {
	thresholds := m.metricThresholds()
	label := diskMountLabel(metrics)
	percent := metricValueStyle(metrics.DiskPercent(), thresholds.DiskWarn, thresholds.DiskCrit).Render(fmt.Sprintf("%.0f%%", metrics.DiskPercent()))
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
	rows := []string{"", m.t("Partitions", "分区")}
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
	deviceLabel := m.t("Device", "设备")
	typeLabel := m.t("Type", "类型")
	suffixRaw := "  " + typeLabel + " " + diskType
	if mountWidth > 24 {
		mountWidth = 24
	}
	prefixRaw := fmt.Sprintf("%02d  %s  %s ", index, padVisible(fit(mount, mountWidth), mountWidth), deviceLabel)
	filesystemWidth := width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	if filesystemWidth < 12 {
		mountWidth = width - ansi.StringWidth(fmt.Sprintf("%02d  ", index)) - ansi.StringWidth("  "+deviceLabel+" ") - ansi.StringWidth(suffixRaw) - 12
		if mountWidth < 8 {
			mountWidth = 8
		}
		mount = fit(mount, mountWidth)
		prefixRaw = fmt.Sprintf("%02d  %s  %s ", index, padVisible(mount, mountWidth), deviceLabel)
		filesystemWidth = width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	}
	if filesystemWidth < 8 {
		filesystemWidth = 8
	}
	mount = padVisible(fit(mount, mountWidth), mountWidth)
	line := indexText +
		"  " + detailValueStyle.Render(mount) +
		"  " + mutedStyle.Render(deviceLabel) + " " + detailValueStyle.Render(fit(filesystem, filesystemWidth)) +
		"  " + mutedStyle.Render(typeLabel) + " " + detailValueStyle.Render(diskType)
	if ansi.StringWidth(line) > width {
		return fitANSI(line, width)
	}
	return line
}

func (m Model) diskPartitionUsageLine(disk monitor.DiskMetric) string {
	thresholds := m.metricThresholds()
	parts := []string{percentBarWithThreshold(disk.Percent(), thresholds.DiskWarn, thresholds.DiskCrit)}
	if size := bytesPair(disk.Used, disk.Total); size != "" {
		parts = append(parts, detailValueStyle.Render(size))
	}
	if disk.AvailKnown {
		parts = append(parts, mutedStyle.Render(m.t("Avail", "可用"))+" "+detailValueStyle.Render(bytesHuman(disk.Available)))
	}
	line := strings.Repeat(" ", 10) + strings.Join(parts, "  ")
	if ansi.StringWidth(line) > m.detailContentWidth() {
		return fitANSI(line, m.detailContentWidth())
	}
	return line
}

func (m Model) swapUsageText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return m.t("Not configured", "未配置")
	}
	return fmt.Sprintf("%s  %s / %s", percentBar(metrics.SwapPercent()), bytesHuman(metrics.SwapUsed), bytesHuman(metrics.SwapTotal))
}

func swapFreeText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "-"
	}
	return bytesHuman(metrics.SwapFree)
}

func (m Model) inodeUsageText(metrics monitor.Metrics) string {
	if metrics.InodeTotal == 0 && metrics.InodeUsed == 0 && metrics.InodeAvailable == 0 {
		return "-"
	}
	thresholds := m.metricThresholds()
	return fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.InodePercent(), thresholds.DiskWarn, thresholds.DiskCrit), countHuman(metrics.InodeUsed), countHuman(metrics.InodeTotal))
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

func (m Model) cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	if m.isChineseUI() {
		return fmt.Sprintf("%d核", metrics.CPUCores)
	}
	return fmt.Sprintf("%d cores", metrics.CPUCores)
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
