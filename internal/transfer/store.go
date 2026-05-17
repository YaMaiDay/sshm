package transfer

import (
	"time"

	"github.com/YaMaiDay/sshm/internal/config"
)

type Completion struct {
	ID        string
	Output    string
	Failed    bool
	ErrorText string
}

func LoadHistory(home string) (config.TransferHistoryFile, bool, error) {
	return config.LoadTransfers(home)
}

func SaveHistory(home string, file config.TransferHistoryFile) error {
	return config.SaveTransfers(home, file)
}

func AppendEntry(home string, entry config.TransferEntry) error {
	return config.AppendTransfer(home, entry)
}

func UpdateEntry(home string, entry config.TransferEntry) error {
	return config.UpdateTransfer(home, entry)
}

func DeleteEntry(home string, id string) error {
	return config.DeleteTransfer(home, id)
}

func MarkRunningTransfersInterrupted(home string) error {
	return config.MarkRunningTransfersInterrupted(home)
}

func SetEntryStatus(entry *config.TransferEntry, status string, errorText string) {
	SetEntryStatusAt(entry, status, errorText, time.Now().Format(time.RFC3339))
}

func SetEntryStatusAt(entry *config.TransferEntry, status string, errorText string, updatedAt string) {
	entry.Status = status
	entry.Error = errorText
	entry.UpdatedAt = updatedAt
}

func CompleteJob(home string, done Completion) error {
	file, _, err := config.LoadTransfers(home)
	if err != nil {
		return err
	}
	for i := range file.Entries {
		if file.Entries[i].ID != done.ID {
			continue
		}
		if file.Entries[i].Status == config.TransferStatusCanceled || file.Entries[i].Status == config.TransferStatusInterrupted {
			return config.SaveTransfers(home, file)
		}
		file.Entries[i].Progress = LastProgressLine(done.Output)
		UpdateProgressBytes(&file.Entries[i], file.Entries[i].Progress)
		if done.Failed {
			SetEntryStatus(&file.Entries[i], config.TransferStatusFailed, done.ErrorText)
		} else {
			SetEntryStatus(&file.Entries[i], config.TransferStatusDone, "")
			file.Entries[i].Progress = "100%"
			if file.Entries[i].TotalBytes > 0 {
				file.Entries[i].DoneBytes = file.Entries[i].TotalBytes
				file.Entries[i].CurrentBytes = 0
			}
		}
		return config.SaveTransfers(home, file)
	}
	return nil
}

func UpdateProgress(home string, id string, progress string) error {
	if id == "" || progress == "" {
		return nil
	}
	_, err := config.UpdateRunningTransferProgress(home, id, func(entry *config.TransferEntry) {
		entry.Progress = progress
		UpdateProgressBytes(entry, progress)
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
	})
	return err
}

func UpdateProgressBytes(entry *config.TransferEntry, progress string) {
	bytes, percent, seq, ok := ProgressValues(progress)
	if !ok {
		return
	}
	if percent >= 100 && seq > 0 && seq > entry.ProgressSeq {
		entry.DoneBytes += bytes
		entry.CurrentBytes = 0
		entry.ProgressSeq = seq
	} else if percent >= 100 && entry.TotalBytes > 0 && bytes >= entry.TotalBytes {
		entry.DoneBytes = entry.TotalBytes
		entry.CurrentBytes = 0
	} else {
		entry.CurrentBytes = bytes
	}
	if entry.TotalBytes > 0 && entry.DoneBytes > entry.TotalBytes {
		entry.DoneBytes = entry.TotalBytes
	}
}

func MarkRunningInterrupted(home string, id string) error {
	if id == "" {
		return nil
	}
	file, _, err := config.LoadTransfers(home)
	if err != nil {
		return err
	}
	for i := range file.Entries {
		if file.Entries[i].ID == id && file.Entries[i].Status == config.TransferStatusRunning {
			SetEntryStatus(&file.Entries[i], config.TransferStatusInterrupted, file.Entries[i].Error)
			return config.SaveTransfers(home, file)
		}
	}
	return nil
}
