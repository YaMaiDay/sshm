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
	m.commandState.History = file
	m.commandState.HistoryIndex = clampInt(m.commandState.HistoryIndex, 0, maxInt(0, len(file.Entries)-1))
	m.commandState.HistoryScroll = 0
	m.mode = modeCommandHistory
	m.status = ""
	return m, nil
}

func (m *Model) reloadCommandHistory() {
	file, _, err := commandservice.LoadHistory(m.home)
	if err != nil {
		return
	}
	m.commandState.History = file
	m.commandState.HistoryIndex = clampInt(m.commandState.HistoryIndex, 0, maxInt(0, len(file.Entries)-1))
}

func (m Model) updateCommandHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.commandState.HistorySearch {
		switch msg.String() {
		case "esc":
			m.commandState.HistorySearch = false
			m.commandState.HistoryQuery = ""
			m.commandState.HistoryIndex = 0
		case "enter":
			m.commandState.HistorySearch = false
		case "backspace":
			runes := []rune(m.commandState.HistoryQuery)
			if len(runes) > 0 {
				m.commandState.HistoryQuery = string(runes[:len(runes)-1])
				m.commandState.HistoryIndex = 0
			}
		default:
			if len(msg.Runes) > 0 {
				m.commandState.HistoryQuery += string(msg.Runes)
				m.commandState.HistoryIndex = 0
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
		m.commandState.HistorySearch = true
		m.commandState.HistoryQuery = ""
		m.commandState.HistoryIndex = 0
	case "j", "down":
		m.commandState.HistoryIndex = clampInt(m.commandState.HistoryIndex+1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "k", "up":
		m.commandState.HistoryIndex = clampInt(m.commandState.HistoryIndex-1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "enter":
		if _, ok := m.selectedHistoryEntry(); ok {
			m.commandState.HistoryScroll = 0
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
		m.commandState.HistoryScroll = moveClampedInt(m.commandState.HistoryScroll, 1, 0, m.commandHistoryDetailMaxScroll())
	case "k", "up":
		m.commandState.HistoryScroll = moveClampedInt(m.commandState.HistoryScroll, -1, 0, m.commandHistoryDetailMaxScroll())
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
	if m.commandState.HistoryIndex < 0 || m.commandState.HistoryIndex >= len(entries) {
		return config.CommandHistoryEntry{}, false
	}
	return entries[m.commandState.HistoryIndex], true
}

func (m Model) filteredHistoryEntries() []config.CommandHistoryEntry {
	query := strings.ToLower(strings.TrimSpace(m.commandState.HistoryQuery))
	if query == "" {
		return m.commandState.History.Entries
	}
	out := make([]config.CommandHistoryEntry, 0, len(m.commandState.History.Entries))
	for _, entry := range m.commandState.History.Entries {
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
	m.commandState.HistoryIndex = clampInt(m.commandState.HistoryIndex, 0, maxInt(0, len(m.commandState.History.Entries)-1))
	m.status = m.t("Command history deleted.", "命令历史已删除。")
	if len(m.commandState.History.Entries) == 0 {
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
		m.commandState.Active = activeCommand{
			HostIndex: indexes[0],
			Name:      historyCommandName(entry),
			Command:   entry.Command,
			Running:   true,
		}
		m.commandState.OutputScroll = 0
		m.commandState.OutputBack = backMode
		m.mode = modeCommandOutput
		m.status = m.t("Rerunning command...", "正在重新执行命令...")
		return m, m.runCommand(indexes[0], entry.Command)
	}
	backMode := m.mode
	m.batchState.Selected = map[int]bool{}
	for _, index := range indexes {
		m.batchState.Selected[index] = true
	}
	m.batchState.Indexes = indexes
	m.batchState.Command = commandItem{Name: historyCommandName(entry), Command: entry.Command}
	m.prepareBatchJobs()
	if len(m.batchState.Jobs) == 0 {
		m.status = m.t("No runnable servers.", "没有可执行的服务器")
		return m, nil
	}
	m.mode = modeBatchOutput
	m.batchState.Current = 0
	m.batchState.Jobs[0].Running = true
	m.batchState.OutputIndex = 0
	m.batchState.OutputScroll = 0
	m.batchState.OutputBack = backMode
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
