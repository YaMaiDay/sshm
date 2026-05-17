package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) saveTransferFile(file config.TransferHistoryFile) {
	_ = transferservice.SaveHistory(m.home, file)
}

func (m Model) updateTransferEntry(entry config.TransferEntry) {
	_ = transferservice.UpdateEntry(m.home, entry)
}

func (m *Model) updateTransferEntryAndReload(entry config.TransferEntry) {
	m.updateTransferEntry(entry)
	m.reloadTransfers()
}

func (m Model) appendTransferEntry(entry config.TransferEntry) {
	_ = transferservice.AppendEntry(m.home, entry)
}

func (m *Model) deleteTransferEntry(id string) {
	_ = transferservice.DeleteEntry(m.home, id)
	m.reloadTransfers()
}
