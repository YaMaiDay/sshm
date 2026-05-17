package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) startSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	switch entry.Status {
	case config.TransferStatusQueued:
		return m.startTransferEntry(entry)
	case config.TransferStatusFailed, config.TransferStatusInterrupted:
		transferservice.SetEntryStatus(&entry, config.TransferStatusQueued, "")
		m.updateTransferEntryAndReload(entry)
		return m.startTransferEntry(entry)
	default:
		m.status = m.t("This job cannot be started now.", "该任务当前不可开始。")
		return m, nil
	}
}

func (m Model) startAllQueuedTransfers() (tea.Model, tea.Cmd) {
	file := m.transferHistory
	count := 0
	now := time.Now().Format(time.RFC3339)
	for i := range file.Entries {
		if file.Entries[i].Status == config.TransferStatusQueued || file.Entries[i].Status == config.TransferStatusInterrupted {
			transferservice.SetEntryStatusAt(&file.Entries[i], config.TransferStatusPending, "", now)
			count++
		}
	}
	if count == 0 {
		m.status = m.t("No pending or interrupted jobs.", "没有等待中或中断的任务。")
		return m, nil
	}
	m.saveTransferFile(file)
	m.transferStatusFilter = 0
	m.reloadTransfers()
	m.transferRunAll = true
	if m.activeTransfer.Active {
		m.status = fmt.Sprintf(m.t("Added to start all: %d queued.", "已加入全部开始：排队中 %d 个。"), count)
		return m, nil
	}
	return m.startNextQueuedTransfer()
}

func (m Model) transferEntryStatus(id string) (string, bool) {
	for _, entry := range m.transferHistory.Entries {
		if entry.ID == id {
			return entry.Status, true
		}
	}
	return "", false
}

func (m Model) pauseRunningTransfers() (tea.Model, tea.Cmd) {
	file := m.transferHistory
	changed := false
	now := time.Now().Format(time.RFC3339)
	for i := range file.Entries {
		switch file.Entries[i].Status {
		case config.TransferStatusRunning:
			transferservice.SetEntryStatusAt(&file.Entries[i], config.TransferStatusInterrupted, file.Entries[i].Error, now)
			changed = true
		case config.TransferStatusPending:
			transferservice.SetEntryStatusAt(&file.Entries[i], config.TransferStatusQueued, file.Entries[i].Error, now)
			changed = true
		}
	}
	if !changed {
		m.status = m.t("No running or queued jobs.", "没有运行中或排队中的任务。")
		return m, nil
	}
	m.transferRunAll = false
	m.saveTransferFile(file)
	m.reloadTransfers()
	if m.activeTransfer.Active && m.activeTransfer.Cancel != nil {
		m.activeTransfer.Cancel()
	}
	m.status = m.t("Paused running jobs; queued jobs were moved back to pending.", "已暂停运行中任务，排队中任务已退回等待中。")
	return m, nil
}

func (m Model) deleteSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	if entry.Status == config.TransferStatusRunning {
		m.status = m.t("Running jobs cannot be deleted.", "运行中的任务不能删除。")
		return m, nil
	}
	m.deleteTransferEntry(entry.ID)
	return m, nil
}

func (m Model) cancelSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	if entry.Status == config.TransferStatusQueued {
		transferservice.SetEntryStatus(&entry, config.TransferStatusCanceled, entry.Error)
		m.updateTransferEntryAndReload(entry)
		return m, nil
	}
	if entry.Status == config.TransferStatusRunning && m.activeTransfer.ID == entry.ID && m.activeTransfer.Cancel != nil {
		transferservice.SetEntryStatus(&entry, config.TransferStatusInterrupted, entry.Error)
		m.updateTransferEntryAndReload(entry)
		m.activeTransfer.Cancel()
		m.status = m.t("Transfer interrupted. Press c again to cancel it.", "已中断当前传输。再次按 c 可取消该任务。")
		return m, nil
	}
	if entry.Status == config.TransferStatusInterrupted {
		transferservice.SetEntryStatus(&entry, config.TransferStatusCanceled, entry.Error)
		m.updateTransferEntryAndReload(entry)
		m.status = m.t("Canceled interrupted transfer.", "已取消当前中断任务。")
		return m, nil
	}
	m.status = m.t("This job cannot be canceled now.", "该任务当前不可取消。")
	return m, nil
}

func (m Model) findTransferHost(entry config.TransferEntry) (host.Host, int, bool) {
	for i, state := range m.states {
		h := state.Host
		if h.Name == entry.HostName && h.Category == entry.HostCategory {
			return h, i, true
		}
	}
	return host.Host{}, -1, false
}

func (m *Model) updateTransferEntryDone(msg transferDoneMsg) {
	completion := transferservice.Completion{
		ID:     msg.ID,
		Output: msg.Output,
	}
	if msg.Err != nil {
		completion.Failed = true
		completion.ErrorText = transferErrorText(msg.Err, msg.Output)
	}
	_ = transferservice.CompleteJob(m.home, completion)
}

func updateTransferProgress(home string, id string, progress string) {
	_ = transferservice.UpdateProgress(home, id, progress)
}

func (m Model) markActiveTransferInterrupted() {
	_ = transferservice.MarkRunningInterrupted(m.home, m.activeTransfer.ID)
}
