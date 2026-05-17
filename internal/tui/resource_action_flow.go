package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) updateResourceLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
	case "j", "down":
		m.resourceState.LogScroll = moveClampedInt(m.resourceState.LogScroll, 1, 0, m.resourceLogMaxScroll())
	case "k", "up":
		m.resourceState.LogScroll = moveClampedInt(m.resourceState.LogScroll, -1, 0, m.resourceLogMaxScroll())
	case "r":
		return m.openResourceLog()
	}
	m.resourceState.LogScroll = clampInt(m.resourceState.LogScroll, 0, m.resourceLogMaxScroll())
	return m, nil
}

func (m Model) updateResourceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceState.Action = resourceActionNone
	case "enter":
		m.mode = modeResourceOutput
		m.resourceState.ActionRunning = true
		m.resourceState.ActionOutput = ""
		m.resourceState.ActionExitCode = 0
		return m, m.runResourceAction()
	}
	return m, nil
}

func (m Model) updateResourceOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceState.Action = resourceActionNone
	case "j", "down":
		m.resourceState.Scroll = moveClampedInt(m.resourceState.Scroll, 1, 0, m.resourceOutputMaxScroll())
	case "k", "up":
		m.resourceState.Scroll = moveClampedInt(m.resourceState.Scroll, -1, 0, m.resourceOutputMaxScroll())
	case "r":
		if !m.resourceState.ActionRunning && m.resourceState.Action != resourceActionNone {
			m.mode = modeResourceOutput
			m.resourceState.ActionRunning = true
			m.resourceState.ActionOutput = ""
			m.resourceState.ActionExitCode = 0
			return m, m.runResourceAction()
		}
	}
	return m, nil
}

func (m Model) startResourceAction(action resourceActionKind) (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if action == resourceActionDelete {
		m.status = m.t("Delete is disabled for resources.", "资源页已禁用删除操作。")
		return m, nil
	}
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource is read-only.", "该资源为只读信息。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.resourceActionScript(ref.Kind, action, name)) == "" {
		m.status = m.t("Add this process and configure commands first.", "请先添加该进程并配置命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.resourceState.Action = action
	m.resourceState.ActionResource = ref.Kind
	m.resourceState.ActionName = name
	m.resourceState.Scroll = 0
	m.mode = modeResourceConfirm
	return m, nil
}

func (m Model) runResourceAction() tea.Cmd {
	index := m.resourceState.HostIndex
	kind := m.resourceState.ActionResource
	action := m.resourceState.Action
	name := m.resourceState.ActionName
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return func() tea.Msg {
			return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: commandResult{Err: fmt.Errorf("invalid resource"), ExitCode: -1}}
		}
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		script := m.resourceActionScript(kind, action, name)
		result := (resourceservice.Service{}).ExecuteScript(ctx, h, script)
		return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: result}
	}
}

func (m Model) openResourceLog() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource does not have managed logs.", "该资源没有托管日志。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.resourceLogScript(ref.Kind, name, 200)) == "" {
		m.status = m.t("Add this process and configure log command first.", "请先添加该进程并配置日志命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeResourceLog
	m.resourceState.LogName = name
	m.resourceState.LogKind = ref.Kind
	m.resourceState.LogOutput = m.t("Loading logs...", "正在读取日志...")
	m.resourceState.LogScroll = 0
	index := m.resourceState.HostIndex
	kind := m.resourceState.LogKind
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).ExecuteScript(ctx, h, m.resourceLogScript(kind, name, 200))
		return resourceLogMsg{Index: index, Kind: kind, Name: name, Output: result.Output, Result: result}
	}
}
