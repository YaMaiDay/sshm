package transfer

import (
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestUpdateProgressUpdatesRunningEntry(t *testing.T) {
	home := t.TempDir()
	entry := config.TransferEntry{
		ID:         "job-1",
		Kind:       "download",
		Status:     config.TransferStatusRunning,
		HostName:   "web-1",
		Source:     "/remote/file",
		TargetDir:  "/tmp",
		TotalBytes: 200,
	}
	if err := AppendEntry(home, entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	if err := UpdateProgress(home, "job-1", "100 50% 1.00MB/s 0:00:01 (xfr#1, to-chk=0/1)"); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}

	file, _, err := LoadHistory(home)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	got := file.Entries[0]
	if got.Progress == "" || got.CurrentBytes != 100 || got.DoneBytes != 0 {
		t.Fatalf("unexpected progress update: %#v", got)
	}
}

func TestCompleteJobMarksDoneAndCapsBytes(t *testing.T) {
	home := t.TempDir()
	entry := config.TransferEntry{
		ID:         "job-1",
		Kind:       "download",
		Status:     config.TransferStatusRunning,
		HostName:   "web-1",
		Source:     "/remote/file",
		TargetDir:  "/tmp",
		TotalBytes: 200,
	}
	if err := AppendEntry(home, entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	err := CompleteJob(home, Completion{
		ID:     "job-1",
		Output: "200 100% 1.00MB/s 0:00:01 (xfr#1, to-chk=0/1)",
	})
	if err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}

	file, _, err := LoadHistory(home)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	got := file.Entries[0]
	if got.Status != config.TransferStatusDone || got.Progress != "100%" || got.DoneBytes != got.TotalBytes || got.CurrentBytes != 0 {
		t.Fatalf("unexpected completed entry: %#v", got)
	}
}
