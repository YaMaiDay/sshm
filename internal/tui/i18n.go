package tui

import "strings"

func (m Model) t(en string, zh string) string {
	if m.isChineseUI() {
		return zh
	}
	return en
}

func (m Model) isChineseUI() bool {
	return strings.TrimSpace(m.appConfig.Language) == "zh"
}

func (m Model) hasErrorText(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(value, "失败") ||
		strings.Contains(value, "不能") ||
		strings.Contains(value, "需要") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "cannot") ||
		strings.Contains(lower, "must")
}
