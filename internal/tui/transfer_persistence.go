package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m *Model) setPersistenceError(en string, zh string, err error) bool {
	if err == nil {
		return false
	}
	m.status = m.t(en+": ", zh+"：") + err.Error()
	return true
}

func (m Model) saveTransferFile(file config.TransferHistoryFile) error {
	return transferservice.SaveHistory(m.home, file)
}

func (m Model) updateTransferEntry(entry config.TransferEntry) error {
	return transferservice.UpdateEntry(m.home, entry)
}

func (m *Model) updateTransferEntryAndReload(entry config.TransferEntry) error {
	if err := m.updateTransferEntry(entry); err != nil {
		return err
	}
	m.reloadTransfers()
	return nil
}

func (m Model) appendTransferEntry(entry config.TransferEntry) error {
	return transferservice.AppendEntry(m.home, entry)
}

func (m *Model) deleteTransferEntry(id string) error {
	if err := transferservice.DeleteEntry(m.home, id); err != nil {
		return err
	}
	m.reloadTransfers()
	return nil
}

func (m *Model) completeTransferEntry(done transferservice.Completion) {
	m.setPersistenceError("Save transfer result failed", "保存传输结果失败", transferservice.CompleteJob(m.home, done))
}

func (m *Model) reloadTransfers() {
	file, _, err := transferservice.LoadHistory(m.home)
	if m.setPersistenceError("Load transfer history failed", "读取传输历史失败", err) {
		return
	}
	m.transferState.History = file
	if m.transferState.Index >= len(m.transferState.History.Entries) {
		m.transferState.Index = len(m.transferState.History.Entries) - 1
	}
	if m.transferState.Index < 0 {
		m.transferState.Index = 0
	}
	m.ensureTransferIndexVisible()
}

func (m *Model) markActiveTransferInterrupted() {
	err := transferservice.MarkRunningInterrupted(m.home, m.transferState.Active.ID)
	m.setPersistenceError("Save transfer interruption failed", "保存传输中断状态失败", err)
}
