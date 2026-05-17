package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
)

func (m Model) updateDeploymentConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; go back after it finishes or fails.", "部署执行中，完成或失败后再返回")
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, 1, 0, m.deploymentConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, -1, 0, m.deploymentConfirmMaxScroll())
	case "enter":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running.", "部署执行中")
			return m, nil
		}
		if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.Output != "" {
			m.status = m.t("This deployment has already run. Press r to retry, or a to redeploy.", "当前部署已执行，按 r 重试，或按 a 重新部署")
			return m, nil
		}
		queue := m.deploymentConfirmQueue
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		for _, app := range queue {
			if m.deploymentServerIndex(app.Server) < 0 {
				m.status = m.t("Deployment server does not exist: ", "部署服务器不存在：") + emptyDash(app.Server)
				return m, nil
			}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueIndex = 0
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	case "r":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; cannot retry.", "部署执行中，不能重试")
			return m, nil
		}
		if len(m.activeDeployment.Queue) == 0 || m.activeDeployment.QueueFailed < 0 || m.activeDeployment.QueueFailed >= len(m.activeDeployment.Queue) {
			m.status = m.t("No failed item to retry.", "没有失败项可重试")
			return m, nil
		}
		return m.startQueuedDeployment(m.activeDeployment.QueueFailed)
	case "a":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; cannot redeploy.", "部署执行中，不能重新部署")
			return m, nil
		}
		queue := m.activeDeployment.Queue
		if len(queue) == 0 {
			queue = m.deploymentConfirmQueue
		}
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	}
	return m, nil
}

func (m Model) startQueuedDeployment(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.activeDeployment.Queue) {
		m.activeDeployment.Running = false
		m.status = m.t("Deployment queue completed.", "部署队列完成。")
		return m, nil
	}
	app := m.activeDeployment.Queue[index]
	hostIndex := m.deploymentServerIndex(app.Server)
	if hostIndex < 0 {
		m.status = m.t("Deployment server does not exist: ", "部署服务器不存在：") + emptyDash(app.Server)
		return m, nil
	}
	m.activeDeployment.HostIndex = hostIndex
	m.activeDeployment.App = app
	m.activeDeployment.Action = config.DeployActionDeploy
	m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
	m.activeDeployment.Output = ""
	m.activeDeployment.ExitCode = 0
	m.activeDeployment.Running = true
	m.activeDeployment.PreviousVersion = ""
	m.activeDeployment.CurrentVersion = ""
	m.activeDeployment.QueueIndex = index
	m.activeDeployment.QueueFailed = -1
	m.deploymentOutputScroll = 0
	m.mode = modeDeploymentConfirm
	if len(m.activeDeployment.Queue) > 1 {
		m.status = fmt.Sprintf(m.t("Deploying %d/%d: %s", "正在部署 %d/%d：%s"), index+1, len(m.activeDeployment.Queue), app.Name)
	} else {
		m.status = m.t("Deploying...", "正在部署...")
	}
	deploymentProgressStart(m.activeDeployment.ProgressID)
	return m, tea.Batch(m.runDeployment(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
}

func (m Model) startNextQueuedDeployment() (tea.Model, tea.Cmd) {
	next := m.activeDeployment.QueueIndex + 1
	return m.startQueuedDeployment(next)
}

func deploymentQueueNextAfter(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		return deploymentQueueNextMsg{}
	}
}

func (m Model) updateDeploymentOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; go back after it finishes.", "部署执行中，完成后再返回")
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, 1, 0, m.deploymentOutputMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, -1, 0, m.deploymentOutputMaxScroll())
	case "r":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; rollback after it finishes.", "部署执行中，完成后再回滚")
			return m, nil
		}
		if len(m.activeDeployment.App.RollbackCommands) == 0 {
			m.status = m.t("No rollback commands configured.", "没有配置回滚命令")
			return m, nil
		}
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentRollbackConfirm
		m.status = m.t("Confirm Rollback", "确认回滚")
		return m, nil
	}
	return m, nil
}

func (m Model) updateDeploymentRollbackConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDeploymentOutput
	case "j", "down":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, 1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = moveClampedInt(m.deploymentOutputScroll, -1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "enter":
		m.activeDeployment.Running = true
		m.activeDeployment.Action = config.DeployActionRollback
		m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
		m.activeDeployment.Output = ""
		m.activeDeployment.ExitCode = 0
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentOutput
		m.status = m.t("Running rollback...", "正在执行回滚...")
		deploymentProgressStart(m.activeDeployment.ProgressID)
		return m, tea.Batch(m.runDeploymentRollback(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
	}
	return m, nil
}

func (m Model) runDeployment() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := commandResult{Err: fmt.Errorf("%s%s", m.t("Deployment server does not exist: ", "部署服务器不存在："), emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	onOutput := func(text string) { deploymentProgressAppend(progressID, text) }
	if app.FetchMode == config.DeployFetchLocal {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
			defer cancel()
			result := deploymentservice.Service{}.Run(ctx, h, app, onOutput)
			deploymentProgressFinish(progressID, result.Command.Output)
			return deploymentDoneMsg{ID: progressID, Result: commandResultFromDeployment(result.Command), PreviousVersion: result.PreviousVersion, CurrentVersion: result.CurrentVersion}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result := deploymentservice.Service{}.Run(ctx, h, app, onOutput)
		deploymentProgressFinish(progressID, result.Command.Output)
		return deploymentDoneMsg{ID: progressID, Result: commandResultFromDeployment(result.Command), PreviousVersion: result.PreviousVersion, CurrentVersion: result.CurrentVersion}
	}
}

func (m Model) runDeploymentRollback() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := commandResult{Err: fmt.Errorf("%s%s", m.t("Deployment server does not exist: ", "部署服务器不存在："), emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result := deploymentservice.Service{}.Rollback(ctx, h, app, func(text string) { deploymentProgressAppend(progressID, text) })
		deploymentProgressFinish(progressID, result.Output)
		return deploymentDoneMsg{ID: progressID, Result: commandResultFromDeployment(result)}
	}
}

func (m Model) handleDeploymentDone(msg deploymentDoneMsg) (tea.Model, tea.Cmd) {
	if msg.ID != "" && msg.ID != m.activeDeployment.ProgressID {
		return m, nil
	}
	m.activeDeployment.Running = false
	m.activeDeployment.Output = msg.Result.Output
	m.activeDeployment.ExitCode = msg.Result.ExitCode
	m.activeDeployment.PreviousVersion = msg.PreviousVersion
	m.activeDeployment.CurrentVersion = msg.CurrentVersion
	failed := msg.Result.Err != nil
	if failed {
		m.status = fmt.Sprintf("%s %d", m.t("Deployment failed: exit", "部署失败：退出码"), msg.Result.ExitCode)
	} else {
		m.status = m.t("Deployment completed.", "部署完成。")
	}
	if err := m.recordDeployment(msg.Result); err != nil {
		m.status += m.t(" Record save failed: ", " 记录保存失败：") + err.Error()
	}
	if msg.ID != "" {
		deploymentProgressClear(msg.ID)
	}
	if m.activeDeployment.Action == config.DeployActionDeploy && len(m.activeDeployment.Queue) > 0 {
		if failed {
			m.activeDeployment.QueueFailed = m.activeDeployment.QueueIndex
			if len(m.activeDeployment.Queue) > 1 {
				m.status = fmt.Sprintf(m.t("Deployment queue stopped: app %d failed. Press r to retry failed item, a to redeploy.", "部署队列停止：第 %d 个应用失败，按 r 重试失败项，按 a 重新部署"), m.activeDeployment.QueueIndex+1)
			} else {
				m.status = m.t("Deployment failed. Press r to retry, a to redeploy.", "部署失败，按 r 重试，按 a 重新部署")
			}
			return m, nil
		}
		next := m.activeDeployment.QueueIndex + 1
		if next < len(m.activeDeployment.Queue) {
			wait := maxInt(0, m.activeDeployment.App.WaitSeconds)
			m.status = fmt.Sprintf(m.t("Deployment completed. Waiting %d seconds before next: %s", "部署完成，等待 %d 秒后执行下一个：%s"), wait, m.activeDeployment.Queue[next].Name)
			return m, deploymentQueueNextAfter(time.Duration(wait) * time.Second)
		}
		if len(m.activeDeployment.Queue) > 1 {
			m.status = m.t("Deployment queue completed.", "部署队列完成。")
		}
	}
	return m, nil
}

func (m Model) handleDeploymentProgress(msg deploymentProgressMsg) (tea.Model, tea.Cmd) {
	if msg.ID == "" || msg.ID != m.activeDeployment.ProgressID || !m.activeDeployment.Running {
		return m, nil
	}
	m.activeDeployment.Output = msg.Output
	if msg.Done {
		return m, nil
	}
	return m, deploymentProgressAfter(msg.ID, 300*time.Millisecond)
}

func (m *Model) recordDeployment(result commandResult) error {
	if m.activeDeployment.HostIndex < 0 || m.activeDeployment.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeDeployment.HostIndex].Host
	status := config.DeployStatusSuccess
	errText := ""
	if result.Err != nil {
		status = config.DeployStatusFailed
		errText = result.Err.Error()
	}
	record := config.DeploymentRecord{
		ID:              config.NewDeploymentID(time.Now()),
		Time:            time.Now().Format(time.RFC3339),
		App:             m.activeDeployment.App.Name,
		ServerCategory:  h.Category,
		ServerName:      h.Name,
		Action:          emptyChoice(m.activeDeployment.Action, config.DeployActionDeploy),
		Source:          m.activeDeployment.App.Source,
		Status:          status,
		PreviousVersion: m.activeDeployment.PreviousVersion,
		CurrentVersion:  m.activeDeployment.CurrentVersion,
		ExitCode:        result.ExitCode,
		Output:          result.Output,
		Error:           errText,
	}
	file, err := deploymentservice.AppendRecord(m.home, record)
	if err != nil {
		return err
	}
	m.deploymentFile = file
	return nil
}
