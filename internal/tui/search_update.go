package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	sessionservice "github.com/YaMaiDay/sshm/internal/session"
)

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.searching = false
		m.query = ""
		m.selected = 0
	case "enter":
		if idx, ok := m.selectedRealIndex(); ok {
			m.searching = false
			cmd, cleanup := sessionservice.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Index: idx, Err: err}
			})
		}
		m.searching = false
	case " ":
		if idx, ok := m.selectedRealIndex(); ok {
			m.searching = false
			return m.openDetail(idx)
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "backspace":
		if len(m.query) > 0 {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
			m.selected = 0
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
			m.selected = 0
		}
	}
	return m, nil
}
