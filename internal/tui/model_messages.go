package tui

import (
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) updateTick() (tea.Model, tea.Cmd) {
	m.collectRound++
	m.pendingByRound[m.collectRound] = len(m.states)
	return m, tea.Batch(m.collectAll(m.collectRound, false), tickAfter(m.appConfig.RefreshDuration()))
}

func (m Model) updateCollect(msg collectMsg) (tea.Model, tea.Cmd) {
	if msg.Round != m.collectRound {
		return m, nil
	}
	m.applyMetrics(msg.Index, msg.Metrics)
	m.pendingByRound[msg.Round]--
	if m.pendingByRound[msg.Round] <= 0 {
		delete(m.pendingByRound, msg.Round)
		if msg.Manual && msg.Round == m.manualRound {
			m.refreshStatus = fmt.Sprintf("%s%s", m.t("Manual refresh done: ", "手动刷新完成："), time.Now().Format("15:04:05"))
			if !m.transferState.Active.Active {
				m.status = m.refreshStatus
			}
		} else {
			m.refreshStatus = fmt.Sprintf("%s%s", m.t("Last refresh: ", "最后刷新："), time.Now().Format("15:04:05"))
			if m.status == m.t("Collecting server status...", "正在采集服务器状态...") {
				m.status = ""
			}
		}
	}
	return m, nil
}

func (m Model) updateTransferDone(msg transferDoneMsg) (tea.Model, tea.Cmd) {
	m.transferState.Active.Active = false
	m.transferState.Active.Cancel = nil
	m.updateTransferEntryDone(msg)
	m.reloadTransfers()
	if status, ok := m.transferEntryStatus(msg.ID); ok {
		if status == config.TransferStatusInterrupted {
			m.status = fmt.Sprintf(m.t("%s interrupted.", "%s已中断。"), msg.Kind)
			return m, clearStatusAfter(3 * time.Second)
		}
		if status == config.TransferStatusCanceled {
			m.status = fmt.Sprintf(m.t("%s canceled.", "%s已取消。"), msg.Kind)
			return m, clearStatusAfter(3 * time.Second)
		}
	}
	if msg.Err != nil {
		m.status = fmt.Sprintf(m.t("%s failed: %s", "%s失败：%s"), msg.Kind, transferErrorText(msg.Err, msg.Output))
		if m.transferState.RunAll {
			return m.startNextQueuedTransfer()
		}
		return m, clearStatusAfter(3 * time.Second)
	}
	m.status = fmt.Sprintf(m.t("%s complete: %s -> %s", "%s完成：%s -> %s"), msg.Kind, filepath.Base(msg.Source), msg.Target)
	if m.transferState.RunAll {
		return m.startNextQueuedTransfer()
	}
	return m, clearStatusAfter(3 * time.Second)
}

func (m Model) updateRsyncCheck(msg rsyncCheckMsg) (tea.Model, tea.Cmd) {
	if msg.Missing {
		m.transferState.Panel.NeedsInstall = true
		m.status = m.t("Remote rsync is not installed. Press i to install and continue, Esc to cancel.", "远程未安装 rsync。按 i 尝试安装并继续，Esc 取消。")
		return m, nil
	}
	if msg.ErrText != "" {
		m.status = m.t("Rsync check failed: ", "检测 rsync 失败：") + msg.ErrText
		return m, nil
	}
	return m.createTransferJobsFromPanel()
}

func (m Model) updateRsyncInstall(msg rsyncInstallMsg) (tea.Model, tea.Cmd) {
	if msg.ErrText != "" {
		m.status = m.t("Rsync install failed: ", "安装 rsync 失败：") + msg.ErrText
		return m, nil
	}
	m.transferState.Panel.NeedsInstall = false
	m.status = m.t("Rsync installed, starting transfer.", "rsync 安装成功，开始传输。")
	return m.createTransferJobsFromPanel()
}

func (m Model) updateTransferProgress() (tea.Model, tea.Cmd) {
	if !m.transferState.Active.Active {
		return m, nil
	}
	m.reloadTransfers()
	m.status = m.transferProgressText(m.transferState.Active)
	return m, transferProgressAfter(500 * time.Millisecond)
}

func (m Model) updateClearStatus() (tea.Model, tea.Cmd) {
	if !m.transferState.Active.Active {
		m.status = ""
	}
	return m, nil
}

func (m Model) updateSSHDone(msg sshDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.status = fmt.Sprintf("登录退出：%v", msg.Err)
		return m, tea.Batch(clearScreen(), clearStatusAfter(3*time.Second))
	}
	if msg.Index >= 0 && msg.Index < len(m.states) {
		m.recordLastLogin(m.states[msg.Index].Host, time.Now())
	}
	m.status = "已返回监控面板"
	return m, tea.Batch(clearScreen(), clearStatusAfter(2*time.Second))
}

func (m Model) updateLoginRecords(msg loginRecordsMsg) (tea.Model, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.states) {
		return m, nil
	}
	m.states[msg.Index].LoginLoading = false
	m.states[msg.Index].LoginSummary = msg.Summary
	m.states[msg.Index].LoginError = msg.ErrText
	m.states[msg.Index].FailedLoginSummary = msg.FailedSummary
	m.states[msg.Index].FailedLoginError = msg.FailedErrText
	m.states[msg.Index].SSHDSecurity = msg.SSHDSecurity
	m.states[msg.Index].SSHDSecurityError = msg.SSHDErrText
	return m, nil
}

func (m Model) updateCommandDone(msg commandDoneMsg) (tea.Model, tea.Cmd) {
	m.commandState.Active.Running = false
	m.commandState.Active.Output = msg.Result.Output
	m.commandState.Active.ExitCode = msg.Result.ExitCode
	historyErr := m.recordCommandHistory(msg.Result)
	if msg.Result.Err != nil {
		m.status = fmt.Sprintf("命令执行失败：退出码 %d", msg.Result.ExitCode)
	} else {
		m.status = "命令执行完成。"
	}
	if historyErr != nil {
		m.status += " 历史保存失败：" + historyErr.Error()
	}
	return m, nil
}
