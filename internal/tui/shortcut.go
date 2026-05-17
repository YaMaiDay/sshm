package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func shortcutKey(msg tea.KeyMsg) string {
	key := strings.ToLower(msg.String())
	if key == "shift+/" {
		return key
	}
	if len(msg.Runes) == 1 {
		key = normalizeShortcutRune(msg.Runes[0])
	}
	return key
}

func normalizeShortcutRune(r rune) string {
	switch {
	case r >= 'Ａ' && r <= 'Ｚ':
		return string(r - 'Ａ' + 'a')
	case r >= 'ａ' && r <= 'ｚ':
		return string(r - 'ａ' + 'a')
	case r >= '０' && r <= '９':
		return string(r - '０' + '0')
	}
	switch r {
	case '。':
		return "."
	case '？':
		return "?"
	case '／', '、':
		return "/"
	case '　':
		return " "
	default:
		return strings.ToLower(string(r))
	}
}
