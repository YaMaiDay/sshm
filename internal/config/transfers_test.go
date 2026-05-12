package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadTransfers(t *testing.T) {
	home := t.TempDir()
	file := TransferHistoryFile{Entries: []TransferEntry{{
		ID:        "1",
		Time:      "2026-05-13T00:00:00Z",
		Kind:      "upload",
		Status:    TransferStatusDone,
		HostName:  "demo",
		Source:    "/tmp/a.txt",
		TargetDir: "/home/demo",
	}}}
	if err := SaveTransfers(home, file); err != nil {
		t.Fatalf("SaveTransfers: %v", err)
	}
	if info, err := os.Stat(filepath.Join(home, ".config", "sshm", "transfers.toml")); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("transfers.toml mode = %o, want 600", got)
	}
	got, ok, err := LoadTransfers(home)
	if err != nil {
		t.Fatalf("LoadTransfers: %v", err)
	}
	if !ok {
		t.Fatal("LoadTransfers ok = false, want true")
	}
	if len(got.Entries) != 1 || got.Entries[0].Source != "/tmp/a.txt" {
		t.Fatalf("loaded transfers = %#v", got)
	}
}

func TestNormalizeTransfersKeepsUnfinishedAndLimitsFinished(t *testing.T) {
	entries := []TransferEntry{{
		ID:        "running",
		Kind:      "upload",
		Status:    TransferStatusRunning,
		HostName:  "demo",
		Source:    "/tmp/running",
		TargetDir: "/tmp",
	}}
	for i := 0; i < MaxTransferHistoryEntries+5; i++ {
		entries = append(entries, TransferEntry{
			ID:        NewTransferID(time.Unix(int64(i), 0)),
			Kind:      "download",
			Status:    TransferStatusDone,
			HostName:  "demo",
			Source:    "/tmp/source",
			TargetDir: "/tmp/target",
		})
	}
	got := normalizeTransferEntries(entries)
	if len(got) != MaxTransferHistoryEntries+1 {
		t.Fatalf("entry count = %d, want %d", len(got), MaxTransferHistoryEntries+1)
	}
	if got[0].ID != "running" {
		t.Fatalf("first entry = %q, want running unfinished entry", got[0].ID)
	}
}

func TestMarkRunningTransfersInterrupted(t *testing.T) {
	home := t.TempDir()
	if err := SaveTransfers(home, TransferHistoryFile{Entries: []TransferEntry{{
		ID:        "1",
		Kind:      "upload",
		Status:    TransferStatusRunning,
		HostName:  "demo",
		Source:    "/tmp/a",
		TargetDir: "/tmp",
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := MarkRunningTransfersInterrupted(home); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadTransfers(home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Entries[0].Status != TransferStatusInterrupted {
		t.Fatalf("status = %q, want interrupted", got.Entries[0].Status)
	}
}
