package tui

import (
	sessionservice "github.com/YaMaiDay/sshm/internal/session"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if handled, model, cmd := m.updateModeKey(msg); handled {
		return model, cmd
	}
	if m.searching {
		return m.updateSearch(msg)
	}
	return m.updateDashboardKey(msg)
}

func (m Model) updateModeKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	switch m.mode {
	case modeCommandList:
		model, cmd := m.updateCommandList(msg)
		return true, model, cmd
	case modeCommandEdit:
		model, cmd := m.updateCommandEdit(msg)
		return true, model, cmd
	case modeCommandConfirm:
		model, cmd := m.updateCommandConfirm(msg)
		return true, model, cmd
	case modeCommandOutput:
		model, cmd := m.updateCommandOutput(msg)
		return true, model, cmd
	case modeBatchSelect:
		model, cmd := m.updateBatchSelect(msg)
		return true, model, cmd
	case modeBatchCommandList:
		model, cmd := m.updateBatchCommandList(msg)
		return true, model, cmd
	case modeBatchCommandEdit:
		model, cmd := m.updateBatchCommandEdit(msg)
		return true, model, cmd
	case modeBatchConfirm:
		model, cmd := m.updateBatchConfirm(msg)
		return true, model, cmd
	case modeBatchOutput:
		model, cmd := m.updateBatchOutput(msg)
		return true, model, cmd
	case modeCommandHistory:
		model, cmd := m.updateCommandHistory(msg)
		return true, model, cmd
	case modeCommandHistoryDetail:
		model, cmd := m.updateCommandHistoryDetail(msg)
		return true, model, cmd
	case modeAnomalyOverview:
		model, cmd := m.updateAnomalyOverview(msg)
		return true, model, cmd
	case modeDeploymentList:
		model, cmd := m.updateDeploymentList(msg)
		return true, model, cmd
	case modeDeploymentDetail:
		model, cmd := m.updateDeploymentDetail(msg)
		return true, model, cmd
	case modeDeploymentEdit:
		model, cmd := m.updateDeploymentEdit(msg)
		return true, model, cmd
	case modeDeploymentConfirm:
		model, cmd := m.updateDeploymentConfirm(msg)
		return true, model, cmd
	case modeDeploymentRollbackConfirm:
		model, cmd := m.updateDeploymentRollbackConfirm(msg)
		return true, model, cmd
	case modeDeploymentOutput:
		model, cmd := m.updateDeploymentOutput(msg)
		return true, model, cmd
	case modeSettings:
		model, cmd := m.updateSettings(msg)
		return true, model, cmd
	case modeTransferJobs:
		model, cmd := m.updateTransferJobs(msg)
		return true, model, cmd
	case modeTransferDetail:
		model, cmd := m.updateTransferDetail(msg)
		return true, model, cmd
	case modeHelp:
		model, cmd := m.updateHelpPanel(msg)
		return true, model, cmd
	case modeResourceList:
		model, cmd := m.updateResourceList(msg)
		return true, model, cmd
	case modeResourceDetail:
		model, cmd := m.updateResourceDetail(msg)
		return true, model, cmd
	case modeResourceAdd:
		model, cmd := m.updateResourceAdd(msg)
		return true, model, cmd
	case modeResourceAddEdit:
		model, cmd := m.updateResourceAddEdit(msg)
		return true, model, cmd
	case modeResourceLog:
		model, cmd := m.updateResourceLog(msg)
		return true, model, cmd
	case modeResourceCommandEdit:
		model, cmd := m.updateResourceCommandEdit(msg)
		return true, model, cmd
	case modeResourceConfirm:
		model, cmd := m.updateResourceConfirm(msg)
		return true, model, cmd
	case modeResourceOutput:
		model, cmd := m.updateResourceOutput(msg)
		return true, model, cmd
	case modeAddForm:
		model, cmd := m.updateAddForm(msg)
		return true, model, cmd
	case modeDeleteConfirm:
		model, cmd := m.updateDeleteConfirm(msg)
		return true, model, cmd
	case modeConfirmAction:
		model, cmd := m.updateConfirmAction(msg)
		return true, model, cmd
	case modeTransferPanel:
		model, cmd := m.updateTransferPanel(msg)
		return true, model, cmd
	case modeDetail:
		model, cmd := m.updateDetail(msg)
		return true, model, cmd
	}
	if m.mode != modeDashboard {
		model, cmd := m.updatePicker(msg)
		return true, model, cmd
	}
	return false, m, nil
}

func (m Model) updateDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "q", "esc", "ctrl+c":
		if m.activeTransfer.Active && m.activeTransfer.Cancel != nil {
			m.markActiveTransferInterrupted()
			m.activeTransfer.Cancel()
		}
		return m, tea.Quit
	case "j", "down":
		m.moveDashboardDown()
	case "k", "up":
		m.moveDashboardUp()
	case "h", "left":
		m.moveDashboardLeft()
	case "l", "right":
		m.moveDashboardRight()
	case "/":
		m.searching = true
		m.query = ""
	case "?", "shift+/":
		m.helpBackMode = modeDashboard
		m.mode = modeHelp
	case "s":
		m.sortBy = (m.sortBy + 1) % 5
		m.selected = 0
		m.status = m.t("Sort: ", "排序：") + m.sortName()
	case "o":
		if m.filter == filterOnline {
			m.filter = filterAll
		} else {
			m.filter = filterOnline
		}
		m.selected = 0
	case "p":
		if m.filter == filterProblem {
			m.filter = filterAll
		} else {
			m.filter = filterProblem
		}
		m.selected = 0
	case "f":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.toggleFavorite(idx)
		}
	case "t":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.togglePinned(idx)
		}
	case "y":
		m.transferJobsBack = modeDashboard
		m.mode = modeTransferJobs
		m.reloadTransfers()
	case "g":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startDeploymentList(idx), nil
		}
	case ".":
		return m.startSettings(), nil
	case "v":
		m.favoriteOnly = !m.favoriteOnly
		m.selected = 0
		if m.favoriteOnly {
			m.status = m.t("Filter: favorites", "筛选：收藏")
		} else {
			m.status = m.t("Favorites filter cleared", "已取消收藏筛选")
		}
	case "tab":
		m.cycleCategory()
		m.selected = 0
	case " ":
		if m.dashboard.Mode == dashboardCategory && m.dashboard.Focus == 0 {
			m.dashboard.Focus = 1
			return m, nil
		}
		if idx, ok := m.selectedRealIndex(); ok {
			return m.openDetail(idx)
		}
	case "a":
		return m.startAddForm(), nil
	case "c":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startCopyForm(idx), nil
		}
	case "e":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startEditForm(idx), nil
		}
	case "x":
		if idx, ok := m.selectedRealIndex(); ok {
			m.deleteIndex = idx
			m.mode = modeDeleteConfirm
		}
	case "u":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startUpload(idx), nil
		}
	case "d":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startDownload(idx), nil
		}
	case "m":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startCommandList(idx), nil
		}
	case "b":
		return m.startBatchSelect(), nil
	case "i":
		return m.startCommandHistory()
	case "w":
		m.mode = modeAnomalyOverview
		m.anomaly.Index = 0
	case "n":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startResourceList(idx, resourceAll, modeDashboard)
		}
	case "z":
		switch m.dashboard.Mode {
		case dashboardCards:
			m.dashboard.Mode = dashboardGrouped
		case dashboardGrouped:
			m.dashboard.Mode = dashboardCategory
			m.dashboard.Focus = 1
		default:
			m.dashboard.Mode = dashboardCards
		}
		m.status = ""
	case "r":
		m.status = m.t("Refreshing all servers...", "正在刷新全部服务器...")
		m.collectRound++
		m.manualRound = m.collectRound
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, true)
	case "enter":
		if m.dashboard.Mode == dashboardCategory && m.dashboard.Focus == 0 {
			m.dashboard.Focus = 1
			return m, nil
		}
		if idx, ok := m.selectedRealIndex(); ok {
			cmd, cleanup := sessionservice.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Index: idx, Err: err}
			})
		}
	}
	return m, nil
}
