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
	m.commandState.File = file
	m.commandState.Items = m.commandListItems(index)
	m.commandState.Index = firstCommandItem(m.commandState.Items)
	m.commandState.Active = activeCommand{HostIndex: index}
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
	for i, command := range m.commandState.File.Server {
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
	for i, command := range m.commandState.File.Global {
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
		m.commandState.Confirm = item
		m.commandState.OutputScroll = 0
		m.mode = modeCommandConfirm
		m.status = "确认执行命令"
	}
	return m, nil
}

func (m *Model) moveCommandIndex(delta int) {
	if len(m.commandState.Items) == 0 {
		m.commandState.Index = 0
		return
	}
	for i := 0; i < len(m.commandState.Items); i++ {
		m.commandState.Index = moveIndex(m.commandState.Index, len(m.commandState.Items), delta)
		item := m.commandState.Items[m.commandState.Index]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedCommandItem() (commandItem, bool) {
	if m.commandState.Index < 0 || m.commandState.Index >= len(m.commandState.Items) {
		return commandItem{}, false
	}
	item := m.commandState.Items[m.commandState.Index]
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
	m.commandState.Form = commandEditForm{Scope: scope, Name: name, Command: body}
	m.commandState.Field = 0
	m.commandState.Cursor = len([]rune(name))
	m.commandState.Editing = editing
	m.commandState.EditItem = item
	m.mode = modeCommandEdit
	if item.Temporary {
		m.commandState.Form.Name = "临时命令"
		m.commandState.Field = 2
		m.commandState.Cursor = 0
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
		m.commandState.Field = (m.commandState.Field + 1) % 3
		m.commandState.Cursor = m.commandValueLen()
	case "shift+tab":
		m.commandState.Field--
		if m.commandState.Field < 0 {
			m.commandState.Field = 2
		}
		m.commandState.Cursor = m.commandValueLen()
	case "up":
		if m.commandState.Field == 2 {
			m.moveCommandBodyLine(-1)
		} else {
			m.commandState.Field--
			if m.commandState.Field < 0 {
				m.commandState.Field = 2
			}
			m.commandState.Cursor = m.commandValueLen()
		}
	case "down":
		if m.commandState.Field == 2 {
			m.moveCommandBodyLine(1)
		} else {
			m.commandState.Field = (m.commandState.Field + 1) % 3
			m.commandState.Cursor = m.commandValueLen()
		}
	case "left":
		if m.commandState.Field == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(-1)
		}
	case "right":
		if m.commandState.Field == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(1)
		}
	case "ctrl+j":
		if m.commandState.Field == 2 {
			m.commandAppend("\n")
		}
	case "enter":
		if strings.TrimSpace(m.commandState.Form.Command) == "" {
			m.status = "保存失败：命令内容不能为空"
			return m, nil
		}
		if m.commandState.EditItem.Temporary {
			m.commandState.Confirm = commandItem{Name: "临时命令", Command: m.commandState.Form.Command, Temporary: true}
			m.commandState.OutputScroll = 0
			m.mode = modeCommandConfirm
			m.status = "确认执行命令"
			return m, nil
		}
		if err := config.ValidateCommandTemplate(m.commandState.Form.Name, m.commandState.Form.Command); err != nil {
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
		if len(msg.Runes) > 0 && m.commandState.Field != 0 {
			m.commandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) backToCommandList(status string) Model {
	index := m.commandState.Active.HostIndex
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
	if m.commandState.Form.Scope == commandScopeGlobal {
		m.commandState.Form.Scope = commandScopeServer
	} else {
		m.commandState.Form.Scope = commandScopeGlobal
	}
}

func (m Model) commandValue() string {
	switch m.commandState.Field {
	case 1:
		return m.commandState.Form.Name
	case 2:
		return m.commandState.Form.Command
	default:
		return ""
	}
}

func (m Model) commandValueLen() int {
	return len([]rune(m.commandValue()))
}

func (m *Model) setCommandValue(value string) {
	switch m.commandState.Field {
	case 1:
		m.commandState.Form.Name = value
	case 2:
		m.commandState.Form.Command = value
	}
}

func (m *Model) commandAppend(s string) {
	value := []rune(m.commandValue())
	if m.commandState.Cursor < 0 {
		m.commandState.Cursor = 0
	}
	if m.commandState.Cursor > len(value) {
		m.commandState.Cursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.commandState.Cursor]...)
	next = append(next, insert...)
	next = append(next, value[m.commandState.Cursor:]...)
	m.setCommandValue(string(next))
	m.commandState.Cursor += len(insert)
}

func (m *Model) commandBackspace() {
	if m.commandState.Field == 0 {
		return
	}
	value := []rune(m.commandValue())
	if m.commandState.Cursor <= 0 || len(value) == 0 {
		return
	}
	if m.commandState.Cursor > len(value) {
		m.commandState.Cursor = len(value)
	}
	next := append([]rune{}, value[:m.commandState.Cursor-1]...)
	next = append(next, value[m.commandState.Cursor:]...)
	m.setCommandValue(string(next))
	m.commandState.Cursor--
}

func (m *Model) moveCommandCursor(delta int) {
	m.commandState.Cursor += delta
	if m.commandState.Cursor < 0 {
		m.commandState.Cursor = 0
	}
	if maxCursor := m.commandValueLen(); m.commandState.Cursor > maxCursor {
		m.commandState.Cursor = maxCursor
	}
}

func (m *Model) moveCommandBodyLine(delta int) {
	if m.commandState.Field != 2 {
		return
	}
	runes := []rune(m.commandState.Form.Command)
	if len(runes) == 0 {
		return
	}
	lineStart := 0
	for i := m.commandState.Cursor - 1; i >= 0 && i < len(runes); i-- {
		if runes[i] == '\n' {
			lineStart = i + 1
			break
		}
	}
	col := m.commandState.Cursor - lineStart
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
		m.commandState.Cursor = prevStart + minInt(col, prevEnd-prevStart)
		return
	}
	lineEnd := len(runes)
	for i := m.commandState.Cursor; i < len(runes); i++ {
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
	m.commandState.Cursor = nextStart + minInt(col, nextEnd-nextStart)
}

func (m Model) saveCommandTemplate() error {
	file := m.commandState.File
	name := strings.TrimSpace(m.commandState.Form.Name)
	body := strings.TrimSpace(m.commandState.Form.Command)
	serverKey := ""
	if m.commandState.Active.HostIndex >= 0 && m.commandState.Active.HostIndex < len(m.states) {
		h := m.states[m.commandState.Active.HostIndex].Host
		serverKey = config.ServerCommandKey(h.Category, h.Name)
	}
	if m.commandState.Editing {
		item := m.commandState.EditItem
		if item.Scope == commandScopeGlobal && item.Index >= 0 && item.Index < len(file.Global) {
			file.Global = append(file.Global[:item.Index], file.Global[item.Index+1:]...)
		}
		if item.Scope == commandScopeServer && item.Index >= 0 && item.Index < len(file.Server) {
			file.Server = append(file.Server[:item.Index], file.Server[item.Index+1:]...)
		}
	}
	if m.commandState.Form.Scope == commandScopeGlobal {
		file.Global = append(file.Global, config.CommandTemplate{Name: name, Command: body})
	} else {
		file.Server = append(file.Server, config.ServerCommandTemplate{Server: serverKey, Name: name, Command: body})
	}
	if err := commandservice.SaveTemplates(m.home, file); err != nil {
		return err
	}
	m.commandState.File = file
	return nil
}

func (m Model) deleteCommandTemplate(item commandItem) (tea.Model, tea.Cmd) {
	file := m.commandState.File
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
	m.commandState.File = file
	m.commandState.Items = m.commandListItems(m.commandState.Active.HostIndex)
	m.commandState.Index = firstCommandItem(m.commandState.Items)
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
		m.commandState.OutputScroll = moveClampedInt(m.commandState.OutputScroll, 1, 0, m.commandConfirmMaxScroll())
	case "k", "up":
		m.commandState.OutputScroll = moveClampedInt(m.commandState.OutputScroll, -1, 0, m.commandConfirmMaxScroll())
	case "enter":
		if m.commandState.Active.HostIndex < 0 || m.commandState.Active.HostIndex >= len(m.states) {
			m.status = "没有选中的服务器。"
			return m, nil
		}
		m.commandState.Active.Name = m.commandState.Confirm.Name
		m.commandState.Active.Command = m.commandState.Confirm.Command
		m.commandState.Active.Output = ""
		m.commandState.Active.ExitCode = 0
		m.commandState.Active.Running = true
		m.commandState.OutputScroll = 0
		m.commandState.OutputBack = modeDashboard
		m.mode = modeCommandOutput
		m.status = "正在执行命令..."
		return m, m.runCommand(m.commandState.Active.HostIndex, m.commandState.Confirm.Command)
	}
	return m, nil
}

func (m Model) updateCommandOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = m.commandState.OutputBack
		m.status = ""
	case "j", "down":
		m.commandState.OutputScroll = moveClampedInt(m.commandState.OutputScroll, 1, 0, m.commandOutputMaxScroll())
	case "k", "up":
		m.commandState.OutputScroll = moveClampedInt(m.commandState.OutputScroll, -1, 0, m.commandOutputMaxScroll())
	}
	return m, nil
}

func (m Model) batchRunning() bool {
	for _, job := range m.batchState.Jobs {
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
