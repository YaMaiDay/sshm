package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	commandservice "github.com/YaMaiDay/sshm/internal/command"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startCommandList(index int) Model {
	file, _, err := commandservice.LoadTemplates(m.home)
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
	if err := commandservice.SaveTemplates(m.home, file); err != nil {
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
	if err := commandservice.SaveTemplates(m.home, file); err != nil {
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
