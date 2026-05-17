package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
)

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
