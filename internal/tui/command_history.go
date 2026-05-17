package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	commandservice "github.com/YaMaiDay/sshm/internal/command"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startCommandHistory() (tea.Model, tea.Cmd) {
	file, _, err := commandservice.LoadHistory(m.home)
	if err != nil {
		m.status = m.t("Failed to read command history: ", "读取命令历史失败：") + err.Error()
		return m, nil
	}
	m.commandHistory = file
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(file.Entries)-1))
	m.historyScroll = 0
	m.mode = modeCommandHistory
	m.status = ""
	return m, nil
}

func (m *Model) reloadCommandHistory() {
	file, _, err := commandservice.LoadHistory(m.home)
	if err != nil {
		return
	}
	m.commandHistory = file
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(file.Entries)-1))
}

func (m Model) updateCommandHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.historySearch {
		switch msg.String() {
		case "esc":
			m.historySearch = false
			m.historyQuery = ""
			m.historyIndex = 0
		case "enter":
			m.historySearch = false
		case "backspace":
			runes := []rune(m.historyQuery)
			if len(runes) > 0 {
				m.historyQuery = string(runes[:len(runes)-1])
				m.historyIndex = 0
			}
		default:
			if len(msg.Runes) > 0 {
				m.historyQuery += string(msg.Runes)
				m.historyIndex = 0
			}
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = ""
	case "/":
		m.historySearch = true
		m.historyQuery = ""
		m.historyIndex = 0
	case "j", "down":
		m.historyIndex = clampInt(m.historyIndex+1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "k", "up":
		m.historyIndex = clampInt(m.historyIndex-1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "enter":
		if _, ok := m.selectedHistoryEntry(); ok {
			m.historyScroll = 0
			m.mode = modeCommandHistoryDetail
		}
	case "r":
		if entry, ok := m.selectedHistoryEntry(); ok {
			return m.rerunHistoryEntry(entry)
		}
	case "x":
		if entry, ok := m.selectedHistoryEntry(); ok {
			m.confirm = confirmAction{
				Kind:    confirmDeleteHistory,
				Title:   m.t("Delete Command History", "确认删除命令历史"),
				Lines:   []string{m.t("This command history record will be deleted.", "将删除该命令历史记录。"), m.t("Command: ", "命令：") + m.historyCommandName(entry)},
				Back:    modeCommandHistory,
				History: entry,
			}
			m.mode = modeConfirmAction
		}
	}
	return m, nil
}

func (m Model) updateCommandHistoryDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeCommandHistory
	case "j", "down":
		m.historyScroll = moveClampedInt(m.historyScroll, 1, 0, m.commandHistoryDetailMaxScroll())
	case "k", "up":
		m.historyScroll = moveClampedInt(m.historyScroll, -1, 0, m.commandHistoryDetailMaxScroll())
	case "r":
		if entry, ok := m.selectedHistoryEntry(); ok {
			return m.rerunHistoryEntry(entry)
		}
	case "x":
		if entry, ok := m.selectedHistoryEntry(); ok {
			m.confirm = confirmAction{
				Kind:    confirmDeleteHistory,
				Title:   m.t("Delete Command History", "确认删除命令历史"),
				Lines:   []string{m.t("This command history record will be deleted.", "将删除该命令历史记录。"), m.t("Command: ", "命令：") + m.historyCommandName(entry)},
				Back:    modeCommandHistoryDetail,
				History: entry,
			}
			m.mode = modeConfirmAction
		}
	}
	return m, nil
}

func (m Model) selectedHistoryEntry() (config.CommandHistoryEntry, bool) {
	entries := m.filteredHistoryEntries()
	if m.historyIndex < 0 || m.historyIndex >= len(entries) {
		return config.CommandHistoryEntry{}, false
	}
	return entries[m.historyIndex], true
}

func (m Model) filteredHistoryEntries() []config.CommandHistoryEntry {
	query := strings.ToLower(strings.TrimSpace(m.historyQuery))
	if query == "" {
		return m.commandHistory.Entries
	}
	out := make([]config.CommandHistoryEntry, 0, len(m.commandHistory.Entries))
	for _, entry := range m.commandHistory.Entries {
		if historyEntryMatches(entry, query) {
			out = append(out, entry)
		}
	}
	return out
}

func historyEntryMatches(entry config.CommandHistoryEntry, query string) bool {
	values := []string{entry.Name, entry.Command, entry.Kind, entry.Status}
	for _, target := range entry.Targets {
		values = append(values, target.Category, target.Name, target.HostName, target.User)
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (m Model) deleteCommandHistoryEntry(entry config.CommandHistoryEntry) (tea.Model, tea.Cmd) {
	if err := commandservice.DeleteHistoryEntry(m.home, entry.ID); err != nil {
		m.status = m.t("Failed to delete command history: ", "删除命令历史失败：") + err.Error()
		return m, nil
	}
	m.reloadCommandHistory()
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(m.commandHistory.Entries)-1))
	m.status = m.t("Command history deleted.", "命令历史已删除。")
	if len(m.commandHistory.Entries) == 0 {
		m.mode = modeCommandHistory
	}
	return m, nil
}

func (m Model) rerunHistoryEntry(entry config.CommandHistoryEntry) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(entry.Command) == "" {
		m.status = m.t("History command is empty and cannot be rerun.", "历史命令为空，不能重新执行。")
		return m, nil
	}
	indexes := m.historyTargetIndexes(entry)
	if len(indexes) == 0 {
		m.status = m.t("Server no longer exists; cannot rerun.", "服务器不存在，不能重新执行。")
		return m, nil
	}
	if len(indexes) == 1 {
		backMode := m.mode
		m.activeCommand = activeCommand{
			HostIndex: indexes[0],
			Name:      historyCommandName(entry),
			Command:   entry.Command,
			Running:   true,
		}
		m.commandOutputScroll = 0
		m.commandOutputBack = backMode
		m.mode = modeCommandOutput
		m.status = m.t("Rerunning command...", "正在重新执行命令...")
		return m, m.runCommand(indexes[0], entry.Command)
	}
	backMode := m.mode
	m.batchSelected = map[int]bool{}
	for _, index := range indexes {
		m.batchSelected[index] = true
	}
	m.batchIndexes = indexes
	m.batchCommand = commandItem{Name: historyCommandName(entry), Command: entry.Command}
	m.prepareBatchJobs()
	if len(m.batchJobs) == 0 {
		m.status = m.t("No runnable servers.", "没有可执行的服务器")
		return m, nil
	}
	m.mode = modeBatchOutput
	m.batchCurrent = 0
	m.batchJobs[0].Running = true
	m.batchOutputIndex = 0
	m.batchOutputScroll = 0
	m.batchOutputBack = backMode
	m.status = m.t("Rerunning batch command...", "正在重新批量执行...")
	return m, m.runBatchJob(0)
}

func (m Model) historyTargetIndexes(entry config.CommandHistoryEntry) []int {
	indexes := []int{}
	seen := map[int]bool{}
	for _, target := range entry.Targets {
		if index, ok := m.findHostByHistoryTarget(target); ok && !seen[index] {
			indexes = append(indexes, index)
			seen[index] = true
		}
	}
	return indexes
}

func (m Model) findHostByHistoryTarget(target config.CommandHistoryTarget) (int, bool) {
	for i, state := range m.states {
		h := state.Host
		if strings.TrimSpace(h.Category) == strings.TrimSpace(target.Category) &&
			strings.TrimSpace(h.Name) == strings.TrimSpace(target.Name) {
			return i, true
		}
	}
	return 0, false
}
