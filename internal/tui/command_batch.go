package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	commandservice "github.com/YaMaiDay/sshm/internal/command"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

func (m Model) startBatchSelect() Model {
	indexes := m.filteredIndexes()
	m.batchState.Indexes = indexes
	m.batchState.Selected = map[int]bool{}
	m.batchState.Cursor = 0
	for _, index := range indexes {
		if index == m.selectedRealIndexOrZero() {
			m.batchState.Selected[index] = true
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

func (m Model) updateBatchSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "down":
		m.batchState.Cursor = clampInt(m.batchState.Cursor+1, 0, maxInt(0, len(m.batchState.Indexes)-1))
	case "k", "up":
		m.batchState.Cursor = clampInt(m.batchState.Cursor-1, 0, maxInt(0, len(m.batchState.Indexes)-1))
	case " ":
		if m.batchState.Cursor >= 0 && m.batchState.Cursor < len(m.batchState.Indexes) {
			index := m.batchState.Indexes[m.batchState.Cursor]
			if m.batchState.Selected[index] {
				delete(m.batchState.Selected, index)
			} else {
				m.batchState.Selected[index] = true
			}
		}
	case "a":
		for _, index := range m.batchState.Indexes {
			m.batchState.Selected[index] = true
		}
	case "x":
		m.batchState.Selected = map[int]bool{}
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
	file, _, err := commandservice.LoadTemplates(m.home)
	if err != nil {
		m.status = m.t("Failed to read command templates: ", "读取命令模板失败：") + err.Error()
		return m, nil
	}
	m.commandState.File = file
	m.batchState.CommandItems = m.batchGlobalCommandItems()
	m.batchState.CommandIndex = firstCommandItem(m.batchState.CommandItems)
	m.mode = modeBatchCommandList
	m.status = m.t("Select Batch Command Template", "选择批量命令模板")
	return m, nil
}

func (m Model) batchGlobalCommandItems() []commandItem {
	items := []commandItem{{Name: "全局", Header: true}}
	for i, command := range m.commandState.File.Global {
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
			m.commandState.Form = commandEditForm{Name: "临时命令"}
			m.commandState.Field = 2
			m.commandState.Cursor = 0
			m.mode = modeBatchCommandEdit
			m.status = m.t("Enter batch temporary command", "输入批量临时命令")
			return m, nil
		}
		m.batchState.Command = item
		m.mode = modeBatchConfirm
		m.batchState.OutputScroll = 0
		m.status = m.t("Confirm Batch Run", "确认批量执行")
	}
	return m, nil
}

func (m *Model) moveBatchCommandIndex(delta int) {
	if len(m.batchState.CommandItems) == 0 {
		m.batchState.CommandIndex = 0
		return
	}
	for i := 0; i < len(m.batchState.CommandItems); i++ {
		m.batchState.CommandIndex = moveIndex(m.batchState.CommandIndex, len(m.batchState.CommandItems), delta)
		item := m.batchState.CommandItems[m.batchState.CommandIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedBatchCommandItem() (commandItem, bool) {
	if m.batchState.CommandIndex < 0 || m.batchState.CommandIndex >= len(m.batchState.CommandItems) {
		return commandItem{}, false
	}
	item := m.batchState.CommandItems[m.batchState.CommandIndex]
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
		if strings.TrimSpace(m.commandState.Form.Command) == "" {
			m.status = "命令内容不能为空"
			return m, nil
		}
		m.batchState.Command = commandItem{Name: "临时命令", Command: m.commandState.Form.Command, Temporary: true}
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
		if m.batchState.Command.Temporary {
			m.commandState.Form = commandEditForm{Name: "临时命令", Command: m.batchState.Command.Command}
			m.commandState.Field = 2
			m.commandState.Cursor = len([]rune(m.commandState.Form.Command))
			m.mode = modeBatchCommandEdit
		} else {
			m.mode = modeBatchCommandList
		}
	case "j", "down":
		m.batchState.OutputScroll = moveClampedInt(m.batchState.OutputScroll, 1, 0, m.batchConfirmMaxScroll())
	case "k", "up":
		m.batchState.OutputScroll = moveClampedInt(m.batchState.OutputScroll, -1, 0, m.batchConfirmMaxScroll())
	case "enter":
		m.prepareBatchJobs()
		if len(m.batchState.Jobs) == 0 {
			m.status = "没有可执行的服务器"
			return m, nil
		}
		m.mode = modeBatchOutput
		m.batchState.Current = 0
		m.batchState.Jobs[0].Running = true
		m.batchState.OutputIndex = 0
		m.batchState.OutputScroll = 0
		m.batchState.OutputBack = modeBatchCommandList
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
		m.mode = m.batchState.OutputBack
		if m.mode == modeBatchCommandList {
			m.status = "可继续选择批量命令"
		} else {
			m.status = ""
		}
	case "j", "down":
		m.moveBatchOutputIndex(1)
		m.batchState.OutputScroll = 0
	case "k", "up":
		m.moveBatchOutputIndex(-1)
		m.batchState.OutputScroll = 0
	case "right", "l":
		m.batchState.OutputScroll = moveClampedInt(m.batchState.OutputScroll, 1, 0, m.batchOutputMaxScroll())
	case "left", "h":
		m.batchState.OutputScroll = moveClampedInt(m.batchState.OutputScroll, -1, 0, m.batchOutputMaxScroll())
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
		m.batchState.OutputIndex = 0
		return
	}
	pos := 0
	for i, index := range indexes {
		if index == m.batchState.OutputIndex {
			pos = i
			break
		}
	}
	pos = clampInt(pos+delta, 0, len(indexes)-1)
	m.batchState.OutputIndex = indexes[pos]
}

func (m Model) retryFailedBatchJobs() (tea.Model, tea.Cmd) {
	jobs := make([]batchJob, 0)
	for _, job := range m.batchState.Jobs {
		if job.Done && job.Err != nil {
			jobs = append(jobs, batchJob{HostIndex: job.HostIndex})
		}
	}
	if len(jobs) == 0 {
		m.status = m.t("No failed servers to retry.", "没有失败的服务器需要重试")
		return m, nil
	}
	m.batchState.Jobs = jobs
	m.batchState.Current = 0
	m.batchState.Jobs[0].Running = true
	m.batchState.OutputIndex = 0
	m.batchState.OutputScroll = 0
	m.status = m.t("Retrying failed servers...", "正在重试失败服务器...")
	return m, m.runBatchJob(0)
}

func (m Model) batchSelectedCount() int {
	count := 0
	for _, selected := range m.batchState.Selected {
		if selected {
			count++
		}
	}
	return count
}

func (m Model) selectedBatchHostIndexes() []int {
	indexes := make([]int, 0, m.batchSelectedCount())
	for _, index := range m.batchState.Indexes {
		if m.batchState.Selected[index] {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (m *Model) prepareBatchJobs() {
	indexes := m.selectedBatchHostIndexes()
	m.batchState.Jobs = make([]batchJob, 0, len(indexes))
	for _, index := range indexes {
		m.batchState.Jobs = append(m.batchState.Jobs, batchJob{HostIndex: index})
	}
}

func (m Model) runBatchJob(job int) tea.Cmd {
	if job < 0 || job >= len(m.batchState.Jobs) {
		return nil
	}
	hostIndex := m.batchState.Jobs[job].HostIndex
	if hostIndex < 0 || hostIndex >= len(m.states) {
		return func() tea.Msg {
			return batchCommandDoneMsg{Job: job, Result: commandResult{ExitCode: -1, Err: fmt.Errorf("服务器索引无效")}}
		}
	}
	h := m.states[hostIndex].Host
	script := m.batchState.Command.Command
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result := (commandservice.Service{}).Run(ctx, h, remotescript.NewUserScript(script))
		return batchCommandDoneMsg{Job: job, Result: result}
	}
}

func (m Model) handleBatchCommandDone(msg batchCommandDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Job < 0 || msg.Job >= len(m.batchState.Jobs) {
		return m, nil
	}
	m.batchState.Jobs[msg.Job].Running = false
	m.batchState.Jobs[msg.Job].Done = true
	m.batchState.Jobs[msg.Job].Output = msg.Result.Output
	m.batchState.Jobs[msg.Job].ExitCode = msg.Result.ExitCode
	m.batchState.Jobs[msg.Job].Err = msg.Result.Err
	next := msg.Job + 1
	if next < len(m.batchState.Jobs) {
		m.batchState.Current = next
		m.batchState.Jobs[next].Running = true
		m.batchState.OutputIndex = next
		m.batchState.OutputScroll = 0
		return m, m.runBatchJob(next)
	}
	m.batchState.Current = len(m.batchState.Jobs)
	m.status = fmt.Sprintf("批量执行完成：成功%d  失败%d", m.batchSuccessCount(), m.batchFailCount())
	if err := m.recordBatchCommandHistory(); err != nil {
		m.status += " 历史保存失败：" + err.Error()
	}
	return m, nil
}
