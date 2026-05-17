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
		m.resourceLogScroll = moveClampedInt(m.resourceLogScroll, 1, 0, m.resourceLogMaxScroll())
	case "k", "up":
		m.resourceLogScroll = moveClampedInt(m.resourceLogScroll, -1, 0, m.resourceLogMaxScroll())
	case "r":
		return m.openResourceLog()
	}
	m.resourceLogScroll = clampInt(m.resourceLogScroll, 0, m.resourceLogMaxScroll())
	return m, nil
}

func (m Model) updateResourceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "enter":
		m.mode = modeResourceOutput
		m.resourceActionRunning = true
		m.resourceActionOutput = ""
		m.resourceActionExitCode = 0
		return m, m.runResourceAction()
	}
	return m, nil
}

func (m Model) updateResourceOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "j", "down":
		m.resourceScroll = moveClampedInt(m.resourceScroll, 1, 0, m.resourceOutputMaxScroll())
	case "k", "up":
		m.resourceScroll = moveClampedInt(m.resourceScroll, -1, 0, m.resourceOutputMaxScroll())
	case "r":
		if !m.resourceActionRunning && m.resourceAction != resourceActionNone {
			m.mode = modeResourceOutput
			m.resourceActionRunning = true
			m.resourceActionOutput = ""
			m.resourceActionExitCode = 0
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
	m.resourceAction = action
	m.resourceActionResource = ref.Kind
	m.resourceActionName = name
	m.resourceScroll = 0
	m.mode = modeResourceConfirm
	return m, nil
}

func (m Model) runResourceAction() tea.Cmd {
	index := m.resourceHostIndex
	kind := m.resourceActionResource
	action := m.resourceAction
	name := m.resourceActionName
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
		return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: commandResultFromResource(result)}
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
	m.resourceLogName = name
	m.resourceLogKind = ref.Kind
	m.resourceLogOutput = m.t("Loading logs...", "正在读取日志...")
	m.resourceLogScroll = 0
	index := m.resourceHostIndex
	kind := m.resourceLogKind
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).ExecuteScript(ctx, h, m.resourceLogScript(kind, name, 200))
		return resourceLogMsg{Index: index, Kind: kind, Name: name, Output: result.Output, Result: commandResultFromResource(result)}
	}
}
