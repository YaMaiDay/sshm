package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

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
