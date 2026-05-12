package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const MaxTransferHistoryEntries = 100

var transferFileMu sync.Mutex

const (
	TransferStatusQueued      = "queued"
	TransferStatusPending     = "pending"
	TransferStatusRunning     = "running"
	TransferStatusDone        = "done"
	TransferStatusFailed      = "failed"
	TransferStatusCanceled    = "canceled"
	TransferStatusInterrupted = "interrupted"
)

type TransferHistoryFile struct {
	Entries []TransferEntry `toml:"entries"`
}

type TransferEntry struct {
	ID           string `toml:"id"`
	Time         string `toml:"time"`
	Kind         string `toml:"kind"`
	Status       string `toml:"status"`
	HostCategory string `toml:"host_category"`
	HostName     string `toml:"host_name"`
	Host         string `toml:"host"`
	User         string `toml:"user"`
	Port         string `toml:"port"`
	Source       string `toml:"source"`
	TargetDir    string `toml:"target_dir"`
	IsDir        bool   `toml:"is_dir"`
	Error        string `toml:"error,omitempty"`
	Progress     string `toml:"progress,omitempty"`
	TotalBytes   int64  `toml:"total_bytes,omitempty"`
	DoneBytes    int64  `toml:"done_bytes,omitempty"`
	CurrentBytes int64  `toml:"current_bytes,omitempty"`
	ProgressSeq  int    `toml:"progress_seq,omitempty"`
	UpdatedAt    string `toml:"updated_at,omitempty"`
}

func TransfersPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "transfers.toml")
}

func LoadTransfers(home string) (TransferHistoryFile, bool, error) {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	return loadTransfersUnlocked(home)
}

func loadTransfersUnlocked(home string) (TransferHistoryFile, bool, error) {
	path := TransfersPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return TransferHistoryFile{}, false, nil
		}
		return TransferHistoryFile{}, false, err
	}
	var file TransferHistoryFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return TransferHistoryFile{}, true, err
	}
	file.Entries = normalizeTransferEntries(file.Entries)
	return file, true, nil
}

func SaveTransfers(home string, file TransferHistoryFile) error {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	return saveTransfersUnlocked(home, file)
}

func saveTransfersUnlocked(home string, file TransferHistoryFile) error {
	path := TransfersPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Entries = normalizeTransferEntries(file.Entries)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func AppendTransfer(home string, entry TransferEntry) error {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	file, _, err := loadTransfersUnlocked(home)
	if err != nil {
		return err
	}
	entry = NormalizeTransferEntry(entry)
	file.Entries = append([]TransferEntry{entry}, file.Entries...)
	return saveTransfersUnlocked(home, file)
}

func UpdateTransfer(home string, entry TransferEntry) error {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	file, _, err := loadTransfersUnlocked(home)
	if err != nil {
		return err
	}
	entry = NormalizeTransferEntry(entry)
	for i := range file.Entries {
		if file.Entries[i].ID == entry.ID {
			file.Entries[i] = entry
			return saveTransfersUnlocked(home, file)
		}
	}
	file.Entries = append([]TransferEntry{entry}, file.Entries...)
	return saveTransfersUnlocked(home, file)
}

func DeleteTransfer(home, id string) error {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	file, _, err := loadTransfersUnlocked(home)
	if err != nil {
		return err
	}
	next := make([]TransferEntry, 0, len(file.Entries))
	for _, entry := range file.Entries {
		if entry.ID != id {
			next = append(next, entry)
		}
	}
	file.Entries = next
	return saveTransfersUnlocked(home, file)
}

func MarkRunningTransfersInterrupted(home string) error {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	file, ok, err := loadTransfersUnlocked(home)
	if err != nil || !ok {
		return err
	}
	changed := false
	now := time.Now().Format(time.RFC3339)
	for i := range file.Entries {
		if file.Entries[i].Status == TransferStatusRunning {
			file.Entries[i].Status = TransferStatusInterrupted
			file.Entries[i].UpdatedAt = now
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return saveTransfersUnlocked(home, file)
}

func UpdateRunningTransferProgress(home string, id string, update func(*TransferEntry)) (bool, error) {
	transferFileMu.Lock()
	defer transferFileMu.Unlock()
	file, _, err := loadTransfersUnlocked(home)
	if err != nil {
		return false, err
	}
	for i := range file.Entries {
		if file.Entries[i].ID == id && file.Entries[i].Status == TransferStatusRunning {
			update(&file.Entries[i])
			return true, saveTransfersUnlocked(home, file)
		}
	}
	return false, nil
}

func NewTransferID(at time.Time) string {
	return fmt.Sprintf("%d", at.UnixNano())
}

func NormalizeTransferEntry(entry TransferEntry) TransferEntry {
	now := time.Now().Format(time.RFC3339)
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = NewTransferID(time.Now())
	}
	if strings.TrimSpace(entry.Time) == "" {
		entry.Time = now
	}
	if strings.TrimSpace(entry.UpdatedAt) == "" {
		entry.UpdatedAt = entry.Time
	}
	entry.Kind = strings.TrimSpace(entry.Kind)
	entry.Status = normalizeTransferStatus(entry.Status)
	entry.HostCategory = strings.TrimSpace(entry.HostCategory)
	entry.HostName = strings.TrimSpace(entry.HostName)
	entry.Host = strings.TrimSpace(entry.Host)
	entry.User = strings.TrimSpace(entry.User)
	entry.Port = strings.TrimSpace(entry.Port)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.TargetDir = strings.TrimSpace(entry.TargetDir)
	entry.Error = strings.TrimSpace(entry.Error)
	entry.Progress = strings.TrimSpace(entry.Progress)
	if entry.TotalBytes < 0 {
		entry.TotalBytes = 0
	}
	if entry.DoneBytes < 0 {
		entry.DoneBytes = 0
	}
	if entry.CurrentBytes < 0 {
		entry.CurrentBytes = 0
	}
	if entry.ProgressSeq < 0 {
		entry.ProgressSeq = 0
	}
	return entry
}

func normalizeTransferStatus(status string) string {
	switch strings.TrimSpace(status) {
	case TransferStatusQueued, TransferStatusPending, TransferStatusRunning, TransferStatusDone, TransferStatusFailed, TransferStatusCanceled, TransferStatusInterrupted:
		return strings.TrimSpace(status)
	default:
		return TransferStatusQueued
	}
}

func normalizeTransferEntries(entries []TransferEntry) []TransferEntry {
	unfinished := make([]TransferEntry, 0, len(entries))
	finished := make([]TransferEntry, 0, len(entries))
	for _, entry := range entries {
		entry = NormalizeTransferEntry(entry)
		if entry.Source == "" || entry.TargetDir == "" || entry.HostName == "" {
			continue
		}
		if transferFinished(entry.Status) {
			finished = append(finished, entry)
			continue
		}
		unfinished = append(unfinished, entry)
	}
	if len(finished) > MaxTransferHistoryEntries {
		finished = finished[:MaxTransferHistoryEntries]
	}
	return append(unfinished, finished...)
}

func transferFinished(status string) bool {
	return status == TransferStatusDone || status == TransferStatusFailed || status == TransferStatusCanceled
}
