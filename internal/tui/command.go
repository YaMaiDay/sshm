package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startCommandList(index int) Model {
	file, _, err := config.LoadCommands(m.home)
	if err != nil {
		m.status = "读取命令模板失败：" + err.Error()
		return m
	}
	m.commandFile = file
	m.commandItems = m.commandListItems(index)
	m.commandIndex = firstCommandItem(m.commandItems)
	m.activeCommand = activeCommand{HostIndex: index}
	m.mode = modeCommandList
	m.status = "命令模板"
	return m
}

func (m Model) startBatchSelect() Model {
	indexes := m.filteredIndexes()
	m.batchIndexes = indexes
	m.batchSelected = map[int]bool{}
	m.batchCursor = 0
	for _, index := range indexes {
		if index == m.selectedRealIndexOrZero() {
			m.batchSelected[index] = true
			break
		}
	}
	m.mode = modeBatchSelect
	m.status = m.t("Batch Select Servers", "批量选择服务器")
	return m
}

func (m Model) selectedRealIndexOrZero() int {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return -1
	}
	return idx
}

func (m Model) commandListItems(index int) []commandItem {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	serverKey := config.ServerCommandKey(h.Category, h.Name)
	items := []commandItem{{Name: "当前服务器", Header: true}}
	for i, command := range m.commandFile.Server {
		if command.Server == serverKey {
			items = append(items, commandItem{
				Scope:   commandScopeServer,
				Index:   i,
				Server:  command.Server,
				Name:    command.Name,
				Command: command.Command,
			})
		}
	}
	items = append(items, commandItem{Name: "全局", Header: true})
	for i, command := range m.commandFile.Global {
		items = append(items, commandItem{
			Scope:   commandScopeGlobal,
			Index:   i,
			Name:    command.Name,
			Command: command.Command,
		})
	}
	items = append(items, commandItem{Spacer: true}, commandItem{Name: "临时命令", Command: "", Temporary: true})
	return items
}

func firstCommandItem(items []commandItem) int {
	for i, item := range items {
		if !item.Header {
			return i
		}
	}
	return 0
}

func (m Model) updateCommandList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "down":
		m.moveCommandIndex(1)
	case "k", "up":
		m.moveCommandIndex(-1)
	case "a":
		return m.startCommandEdit(commandItem{}, false), nil
	case "e":
		item, ok := m.selectedCommandItem()
		if ok && !item.Temporary {
			return m.startCommandEdit(item, true), nil
		}
	case "x":
		item, ok := m.selectedCommandItem()
		if ok && !item.Temporary {
			m.confirm = confirmAction{
				Kind:  confirmDeleteCommand,
				Title: "确认删除命令模板",
				Lines: []string{
					"模板：" + item.Name,
					"将删除这个命令模板。",
				},
				Back:    modeCommandList,
				Command: item,
			}
			m.mode = modeConfirmAction
		}
	case "enter":
		item, ok := m.selectedCommandItem()
		if !ok {
			return m, nil
		}
		if item.Temporary {
			return m.startCommandEdit(item, false), nil
		}
		m.commandConfirm = item
		m.commandOutputScroll = 0
		m.mode = modeCommandConfirm
		m.status = "确认执行命令"
	}
	return m, nil
}

func (m *Model) moveCommandIndex(delta int) {
	if len(m.commandItems) == 0 {
		m.commandIndex = 0
		return
	}
	for i := 0; i < len(m.commandItems); i++ {
		m.commandIndex = moveIndex(m.commandIndex, len(m.commandItems), delta)
		item := m.commandItems[m.commandIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedCommandItem() (commandItem, bool) {
	if m.commandIndex < 0 || m.commandIndex >= len(m.commandItems) {
		return commandItem{}, false
	}
	item := m.commandItems[m.commandIndex]
	if item.Header || item.Spacer {
		return commandItem{}, false
	}
	return item, true
}

func (m Model) startCommandEdit(item commandItem, editing bool) Model {
	scope := commandScopeServer
	name := ""
	body := ""
	if editing {
		scope = item.Scope
	}
	if item.Temporary {
		scope = commandScopeServer
	}
	if editing {
		name = item.Name
		body = item.Command
	}
	m.commandForm = commandEditForm{Scope: scope, Name: name, Command: body}
	m.commandField = 0
	m.commandCursor = len([]rune(name))
	m.commandEditing = editing
	m.commandEditItem = item
	m.mode = modeCommandEdit
	if item.Temporary {
		m.commandForm.Name = "临时命令"
		m.commandField = 2
		m.commandCursor = 0
	}
	m.status = "编辑命令模板"
	if !editing {
		m.status = "添加命令模板"
	}
	return m
}

func (m Model) updateCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "ctrl+c":
		return m.backToCommandList("已取消。"), nil
	case "tab":
		m.commandField = (m.commandField + 1) % 3
		m.commandCursor = m.commandValueLen()
	case "shift+tab":
		m.commandField--
		if m.commandField < 0 {
			m.commandField = 2
		}
		m.commandCursor = m.commandValueLen()
	case "up":
		if m.commandField == 2 {
			m.moveCommandBodyLine(-1)
		} else {
			m.commandField--
			if m.commandField < 0 {
				m.commandField = 2
			}
			m.commandCursor = m.commandValueLen()
		}
	case "down":
		if m.commandField == 2 {
			m.moveCommandBodyLine(1)
		} else {
			m.commandField = (m.commandField + 1) % 3
			m.commandCursor = m.commandValueLen()
		}
	case "left":
		if m.commandField == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(-1)
		}
	case "right":
		if m.commandField == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(1)
		}
	case "ctrl+j":
		if m.commandField == 2 {
			m.commandAppend("\n")
		}
	case "enter":
		if strings.TrimSpace(m.commandForm.Command) == "" {
			m.status = "保存失败：命令内容不能为空"
			return m, nil
		}
		if m.commandEditItem.Temporary {
			m.commandConfirm = commandItem{Name: "临时命令", Command: m.commandForm.Command, Temporary: true}
			m.commandOutputScroll = 0
			m.mode = modeCommandConfirm
			m.status = "确认执行命令"
			return m, nil
		}
		if err := config.ValidateCommandTemplate(m.commandForm.Name, m.commandForm.Command); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		if err := m.saveCommandTemplate(); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		return m.backToCommandList("命令模板已保存。"), nil
	case "backspace":
		m.commandBackspace()
	default:
		if len(msg.Runes) > 0 && m.commandField != 0 {
			m.commandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) backToCommandList(status string) Model {
	index := m.activeCommand.HostIndex
	if index < 0 {
		if selected, ok := m.selectedRealIndex(); ok {
			index = selected
		}
	}
	m = m.startCommandList(index)
	m.status = status
	return m
}

func (m *Model) toggleCommandScope() {
	if m.commandForm.Scope == commandScopeGlobal {
		m.commandForm.Scope = commandScopeServer
	} else {
		m.commandForm.Scope = commandScopeGlobal
	}
}

func (m Model) commandValue() string {
	switch m.commandField {
	case 1:
		return m.commandForm.Name
	case 2:
		return m.commandForm.Command
	default:
		return ""
	}
}

func (m Model) commandValueLen() int {
	return len([]rune(m.commandValue()))
}

func (m *Model) setCommandValue(value string) {
	switch m.commandField {
	case 1:
		m.commandForm.Name = value
	case 2:
		m.commandForm.Command = value
	}
}

func (m *Model) commandAppend(s string) {
	value := []rune(m.commandValue())
	if m.commandCursor < 0 {
		m.commandCursor = 0
	}
	if m.commandCursor > len(value) {
		m.commandCursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.commandCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.commandCursor:]...)
	m.setCommandValue(string(next))
	m.commandCursor += len(insert)
}

func (m *Model) commandBackspace() {
	if m.commandField == 0 {
		return
	}
	value := []rune(m.commandValue())
	if m.commandCursor <= 0 || len(value) == 0 {
		return
	}
	if m.commandCursor > len(value) {
		m.commandCursor = len(value)
	}
	next := append([]rune{}, value[:m.commandCursor-1]...)
	next = append(next, value[m.commandCursor:]...)
	m.setCommandValue(string(next))
	m.commandCursor--
}

func (m *Model) moveCommandCursor(delta int) {
	m.commandCursor += delta
	if m.commandCursor < 0 {
		m.commandCursor = 0
	}
	if maxCursor := m.commandValueLen(); m.commandCursor > maxCursor {
		m.commandCursor = maxCursor
	}
}

func (m *Model) moveCommandBodyLine(delta int) {
	if m.commandField != 2 {
		return
	}
	runes := []rune(m.commandForm.Command)
	if len(runes) == 0 {
		return
	}
	lineStart := 0
	for i := m.commandCursor - 1; i >= 0 && i < len(runes); i-- {
		if runes[i] == '\n' {
			lineStart = i + 1
			break
		}
	}
	col := m.commandCursor - lineStart
	if delta < 0 {
		if lineStart == 0 {
			return
		}
		prevEnd := lineStart - 1
		prevStart := 0
		for i := prevEnd - 1; i >= 0; i-- {
			if runes[i] == '\n' {
				prevStart = i + 1
				break
			}
		}
		m.commandCursor = prevStart + minInt(col, prevEnd-prevStart)
		return
	}
	lineEnd := len(runes)
	for i := m.commandCursor; i < len(runes); i++ {
		if runes[i] == '\n' {
			lineEnd = i
			break
		}
	}
	if lineEnd >= len(runes) {
		return
	}
	nextStart := lineEnd + 1
	nextEnd := len(runes)
	for i := nextStart; i < len(runes); i++ {
		if runes[i] == '\n' {
			nextEnd = i
			break
		}
	}
	m.commandCursor = nextStart + minInt(col, nextEnd-nextStart)
}

func (m Model) saveCommandTemplate() error {
	file := m.commandFile
	name := strings.TrimSpace(m.commandForm.Name)
	body := strings.TrimSpace(m.commandForm.Command)
	serverKey := ""
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		h := m.states[m.activeCommand.HostIndex].Host
		serverKey = config.ServerCommandKey(h.Category, h.Name)
	}
	if m.commandEditing {
		item := m.commandEditItem
		if item.Scope == commandScopeGlobal && item.Index >= 0 && item.Index < len(file.Global) {
			file.Global = append(file.Global[:item.Index], file.Global[item.Index+1:]...)
		}
		if item.Scope == commandScopeServer && item.Index >= 0 && item.Index < len(file.Server) {
			file.Server = append(file.Server[:item.Index], file.Server[item.Index+1:]...)
		}
	}
	if m.commandForm.Scope == commandScopeGlobal {
		file.Global = append(file.Global, config.CommandTemplate{Name: name, Command: body})
	} else {
		file.Server = append(file.Server, config.ServerCommandTemplate{Server: serverKey, Name: name, Command: body})
	}
	if err := config.SaveCommands(m.home, file); err != nil {
		return err
	}
	m.commandFile = file
	return nil
}

func (m Model) deleteCommandTemplate(item commandItem) (tea.Model, tea.Cmd) {
	file := m.commandFile
	if item.Scope == commandScopeGlobal && item.Index >= 0 && item.Index < len(file.Global) {
		file.Global = append(file.Global[:item.Index], file.Global[item.Index+1:]...)
	}
	if item.Scope == commandScopeServer && item.Index >= 0 && item.Index < len(file.Server) {
		file.Server = append(file.Server[:item.Index], file.Server[item.Index+1:]...)
	}
	if err := config.SaveCommands(m.home, file); err != nil {
		m.status = "删除失败：" + err.Error()
		return m, nil
	}
	m.commandFile = file
	m.commandItems = m.commandListItems(m.activeCommand.HostIndex)
	m.commandIndex = firstCommandItem(m.commandItems)
	m.status = "命令模板已删除。"
	return m, nil
}

func (m Model) updateCommandConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeCommandList
		m.status = "已取消。"
	case "j", "down":
		m.commandOutputScroll = moveClampedInt(m.commandOutputScroll, 1, 0, m.commandConfirmMaxScroll())
	case "k", "up":
		m.commandOutputScroll = moveClampedInt(m.commandOutputScroll, -1, 0, m.commandConfirmMaxScroll())
	case "enter":
		if m.activeCommand.HostIndex < 0 || m.activeCommand.HostIndex >= len(m.states) {
			m.status = "没有选中的服务器。"
			return m, nil
		}
		m.activeCommand.Name = m.commandConfirm.Name
		m.activeCommand.Command = m.commandConfirm.Command
		m.activeCommand.Output = ""
		m.activeCommand.ExitCode = 0
		m.activeCommand.Running = true
		m.commandOutputScroll = 0
		m.commandOutputBack = modeDashboard
		m.mode = modeCommandOutput
		m.status = "正在执行命令..."
		return m, m.runCommand(m.activeCommand.HostIndex, m.commandConfirm.Command)
	}
	return m, nil
}

func (m Model) updateCommandOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = m.commandOutputBack
		m.status = ""
	case "j", "down":
		m.commandOutputScroll = moveClampedInt(m.commandOutputScroll, 1, 0, m.commandOutputMaxScroll())
	case "k", "up":
		m.commandOutputScroll = moveClampedInt(m.commandOutputScroll, -1, 0, m.commandOutputMaxScroll())
	}
	return m, nil
}

func (m Model) updateBatchSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "down":
		m.batchCursor = clampInt(m.batchCursor+1, 0, maxInt(0, len(m.batchIndexes)-1))
	case "k", "up":
		m.batchCursor = clampInt(m.batchCursor-1, 0, maxInt(0, len(m.batchIndexes)-1))
	case " ":
		if m.batchCursor >= 0 && m.batchCursor < len(m.batchIndexes) {
			index := m.batchIndexes[m.batchCursor]
			if m.batchSelected[index] {
				delete(m.batchSelected, index)
			} else {
				m.batchSelected[index] = true
			}
		}
	case "a":
		for _, index := range m.batchIndexes {
			m.batchSelected[index] = true
		}
	case "x":
		m.batchSelected = map[int]bool{}
	case "enter":
		if m.batchSelectedCount() == 0 {
			m.status = m.t("Select at least one server", "请至少选择一台服务器")
			return m, nil
		}
		return m.startBatchCommandList()
	}
	return m, nil
}

func (m Model) startBatchCommandList() (tea.Model, tea.Cmd) {
	file, _, err := config.LoadCommands(m.home)
	if err != nil {
		m.status = m.t("Failed to read command templates: ", "读取命令模板失败：") + err.Error()
		return m, nil
	}
	m.commandFile = file
	m.batchCommandItems = m.batchGlobalCommandItems()
	m.batchCommandIndex = firstCommandItem(m.batchCommandItems)
	m.mode = modeBatchCommandList
	m.status = m.t("Select Batch Command Template", "选择批量命令模板")
	return m, nil
}

func (m Model) batchGlobalCommandItems() []commandItem {
	items := []commandItem{{Name: "全局", Header: true}}
	for i, command := range m.commandFile.Global {
		items = append(items, commandItem{
			Scope:   commandScopeGlobal,
			Index:   i,
			Name:    command.Name,
			Command: command.Command,
		})
	}
	items = append(items, commandItem{Spacer: true}, commandItem{Name: "临时命令", Temporary: true})
	return items
}

func (m Model) updateBatchCommandList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeBatchSelect
	case "j", "down":
		m.moveBatchCommandIndex(1)
	case "k", "up":
		m.moveBatchCommandIndex(-1)
	case "enter":
		item, ok := m.selectedBatchCommandItem()
		if !ok {
			return m, nil
		}
		if item.Temporary {
			m.commandForm = commandEditForm{Name: "临时命令"}
			m.commandField = 2
			m.commandCursor = 0
			m.mode = modeBatchCommandEdit
			m.status = m.t("Enter batch temporary command", "输入批量临时命令")
			return m, nil
		}
		m.batchCommand = item
		m.mode = modeBatchConfirm
		m.batchOutputScroll = 0
		m.status = m.t("Confirm Batch Run", "确认批量执行")
	}
	return m, nil
}

func (m *Model) moveBatchCommandIndex(delta int) {
	if len(m.batchCommandItems) == 0 {
		m.batchCommandIndex = 0
		return
	}
	for i := 0; i < len(m.batchCommandItems); i++ {
		m.batchCommandIndex = moveIndex(m.batchCommandIndex, len(m.batchCommandItems), delta)
		item := m.batchCommandItems[m.batchCommandIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedBatchCommandItem() (commandItem, bool) {
	if m.batchCommandIndex < 0 || m.batchCommandIndex >= len(m.batchCommandItems) {
		return commandItem{}, false
	}
	item := m.batchCommandItems[m.batchCommandIndex]
	if item.Header || item.Spacer {
		return commandItem{}, false
	}
	return item, true
}

func (m Model) updateBatchCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "ctrl+c":
		m.mode = modeBatchCommandList
		m.status = "已取消。"
	case "ctrl+j":
		m.commandAppend("\n")
	case "up":
		m.moveCommandBodyLine(-1)
	case "down":
		m.moveCommandBodyLine(1)
	case "left":
		m.moveCommandCursor(-1)
	case "right":
		m.moveCommandCursor(1)
	case "backspace":
		m.commandBackspace()
	case "enter":
		if strings.TrimSpace(m.commandForm.Command) == "" {
			m.status = "命令内容不能为空"
			return m, nil
		}
		m.batchCommand = commandItem{Name: "临时命令", Command: m.commandForm.Command, Temporary: true}
		m.mode = modeBatchConfirm
		m.status = "确认批量执行"
	default:
		if len(msg.Runes) > 0 {
			m.commandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) updateBatchConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.batchCommand.Temporary {
			m.commandForm = commandEditForm{Name: "临时命令", Command: m.batchCommand.Command}
			m.commandField = 2
			m.commandCursor = len([]rune(m.commandForm.Command))
			m.mode = modeBatchCommandEdit
		} else {
			m.mode = modeBatchCommandList
		}
	case "j", "down":
		m.batchOutputScroll = moveClampedInt(m.batchOutputScroll, 1, 0, m.batchConfirmMaxScroll())
	case "k", "up":
		m.batchOutputScroll = moveClampedInt(m.batchOutputScroll, -1, 0, m.batchConfirmMaxScroll())
	case "enter":
		m.prepareBatchJobs()
		if len(m.batchJobs) == 0 {
			m.status = "没有可执行的服务器"
			return m, nil
		}
		m.mode = modeBatchOutput
		m.batchCurrent = 0
		m.batchJobs[0].Running = true
		m.batchOutputIndex = 0
		m.batchOutputScroll = 0
		m.batchOutputBack = modeBatchCommandList
		m.status = "批量命令执行中..."
		return m, m.runBatchJob(0)
	}
	return m, nil
}

func (m Model) updateBatchOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.batchRunning() {
			m.status = "批量命令执行中，完成后再返回"
			return m, nil
		}
		m.mode = m.batchOutputBack
		if m.mode == modeBatchCommandList {
			m.status = "可继续选择批量命令"
		} else {
			m.status = ""
		}
	case "j", "down":
		m.moveBatchOutputIndex(1)
		m.batchOutputScroll = 0
	case "k", "up":
		m.moveBatchOutputIndex(-1)
		m.batchOutputScroll = 0
	case "right", "l":
		m.batchOutputScroll = moveClampedInt(m.batchOutputScroll, 1, 0, m.batchOutputMaxScroll())
	case "left", "h":
		m.batchOutputScroll = moveClampedInt(m.batchOutputScroll, -1, 0, m.batchOutputMaxScroll())
	case "r":
		if m.batchRunning() {
			m.status = "批量命令执行中，完成后再重试"
			return m, nil
		}
		return m.retryFailedBatchJobs()
	}
	return m, nil
}

func (m *Model) moveBatchOutputIndex(delta int) {
	indexes := m.batchResultDisplayIndexes()
	if len(indexes) == 0 {
		m.batchOutputIndex = 0
		return
	}
	pos := 0
	for i, index := range indexes {
		if index == m.batchOutputIndex {
			pos = i
			break
		}
	}
	pos = clampInt(pos+delta, 0, len(indexes)-1)
	m.batchOutputIndex = indexes[pos]
}

func (m Model) retryFailedBatchJobs() (tea.Model, tea.Cmd) {
	jobs := make([]batchJob, 0)
	for _, job := range m.batchJobs {
		if job.Done && job.Err != nil {
			jobs = append(jobs, batchJob{HostIndex: job.HostIndex})
		}
	}
	if len(jobs) == 0 {
		m.status = m.t("No failed servers to retry.", "没有失败的服务器需要重试")
		return m, nil
	}
	m.batchJobs = jobs
	m.batchCurrent = 0
	m.batchJobs[0].Running = true
	m.batchOutputIndex = 0
	m.batchOutputScroll = 0
	m.status = m.t("Retrying failed servers...", "正在重试失败服务器...")
	return m, m.runBatchJob(0)
}

func (m Model) startCommandHistory() (tea.Model, tea.Cmd) {
	file, _, err := config.LoadCommandHistory(m.home)
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
	file, _, err := config.LoadCommandHistory(m.home)
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
	if err := config.DeleteCommandHistoryEntry(m.home, entry.ID); err != nil {
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

func (m Model) batchRunning() bool {
	for _, job := range m.batchJobs {
		if job.Running {
			return true
		}
	}
	return false
}

func (m Model) updateHelpPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "?":
		if m.helpBackMode == 0 {
			m.helpBackMode = modeDashboard
		}
		m.mode = m.helpBackMode
	}
	return m, nil
}

func (m Model) commandOutputMaxScroll() int {
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	lines := 2
	if m.activeCommand.Running {
		lines++
	} else {
		output := strings.TrimRight(m.activeCommand.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(strings.Split(output, "\n"))
		}
		lines += 2
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) runCommand(index int, script string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, m.states[index].Host, script)
		cleanup()
		return commandDoneMsg{Result: result}
	}
}

func (m Model) batchSelectedCount() int {
	count := 0
	for _, selected := range m.batchSelected {
		if selected {
			count++
		}
	}
	return count
}

func (m Model) selectedBatchHostIndexes() []int {
	indexes := make([]int, 0, m.batchSelectedCount())
	for _, index := range m.batchIndexes {
		if m.batchSelected[index] {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (m *Model) prepareBatchJobs() {
	indexes := m.selectedBatchHostIndexes()
	m.batchJobs = make([]batchJob, 0, len(indexes))
	for _, index := range indexes {
		m.batchJobs = append(m.batchJobs, batchJob{HostIndex: index})
	}
}

func (m Model) runBatchJob(job int) tea.Cmd {
	if job < 0 || job >= len(m.batchJobs) {
		return nil
	}
	hostIndex := m.batchJobs[job].HostIndex
	if hostIndex < 0 || hostIndex >= len(m.states) {
		return func() tea.Msg {
			return batchCommandDoneMsg{Job: job, Result: actions.CommandResult{ExitCode: -1, Err: fmt.Errorf("服务器索引无效")}}
		}
	}
	h := m.states[hostIndex].Host
	script := m.batchCommand.Command
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, script)
		cleanup()
		return batchCommandDoneMsg{Job: job, Result: result}
	}
}

func (m Model) handleBatchCommandDone(msg batchCommandDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Job < 0 || msg.Job >= len(m.batchJobs) {
		return m, nil
	}
	m.batchJobs[msg.Job].Running = false
	m.batchJobs[msg.Job].Done = true
	m.batchJobs[msg.Job].Output = msg.Result.Output
	m.batchJobs[msg.Job].ExitCode = msg.Result.ExitCode
	m.batchJobs[msg.Job].Err = msg.Result.Err
	next := msg.Job + 1
	if next < len(m.batchJobs) {
		m.batchCurrent = next
		m.batchJobs[next].Running = true
		m.batchOutputIndex = next
		m.batchOutputScroll = 0
		return m, m.runBatchJob(next)
	}
	m.batchCurrent = len(m.batchJobs)
	m.status = fmt.Sprintf("批量执行完成：成功%d  失败%d", m.batchSuccessCount(), m.batchFailCount())
	if err := m.recordBatchCommandHistory(); err != nil {
		m.status += " 历史保存失败：" + err.Error()
	}
	return m, nil
}

func (m *Model) recordCommandHistory(result actions.CommandResult) error {
	if m.activeCommand.HostIndex < 0 || m.activeCommand.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeCommand.HostIndex].Host
	status := commandHistoryStatus(result.Err)
	entry := config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(time.Now()),
		Time:     time.Now().Format(time.RFC3339),
		Kind:     "single",
		Name:     m.activeCommand.Name,
		Command:  m.activeCommand.Command,
		Status:   status,
		ExitCode: result.ExitCode,
		Targets: []config.CommandHistoryTarget{
			config.CommandHistoryTargetFromHost(h, status, result.ExitCode, result.Output),
		},
	}
	if err := config.AppendCommandHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func (m *Model) recordBatchCommandHistory() error {
	targets := make([]config.CommandHistoryTarget, 0, len(m.batchJobs))
	failCount := 0
	for _, job := range m.batchJobs {
		if job.HostIndex < 0 || job.HostIndex >= len(m.states) {
			continue
		}
		status := commandHistoryStatus(job.Err)
		if job.Err != nil {
			failCount++
		}
		targets = append(targets, config.CommandHistoryTargetFromHost(m.states[job.HostIndex].Host, status, job.ExitCode, job.Output))
	}
	if len(targets) == 0 {
		return nil
	}
	status := "success"
	if failCount > 0 {
		status = "failed"
	}
	entry := config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(time.Now()),
		Time:     time.Now().Format(time.RFC3339),
		Kind:     "batch",
		Name:     m.batchCommand.Name,
		Command:  m.batchCommand.Command,
		Status:   status,
		ExitCode: failCount,
		Targets:  targets,
	}
	if err := config.AppendCommandHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func commandHistoryStatus(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}
