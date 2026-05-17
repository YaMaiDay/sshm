package transfer

import (
	"testing"
	"time"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

func TestBuildEntryFillsHostAndTransferFields(t *testing.T) {
	at := time.Date(2026, 5, 17, 10, 20, 30, 0, time.UTC)
	entry := BuildEntry(host.Host{
		Category: "prod",
		Name:     "web-1",
		HostName: "10.0.0.1",
		User:     "root",
		Port:     "2222",
	}, EntrySpec{
		ID:         "job-1",
		Time:       at,
		Kind:       "download",
		Source:     "/var/log/app.log",
		TargetDir:  "/tmp",
		IsDir:      false,
		TotalBytes: 42,
	})

	if entry.ID != "job-1" || entry.Time != at.Format(time.RFC3339) || entry.UpdatedAt != entry.Time {
		t.Fatalf("unexpected timestamps/id: %#v", entry)
	}
	if entry.Status != config.TransferStatusQueued {
		t.Fatalf("status = %q, want %q", entry.Status, config.TransferStatusQueued)
	}
	if entry.HostCategory != "prod" || entry.HostName != "web-1" || entry.Host != "10.0.0.1" || entry.User != "root" || entry.Port != "2222" {
		t.Fatalf("unexpected host fields: %#v", entry)
	}
	if entry.Kind != "download" || entry.Source != "/var/log/app.log" || entry.TargetDir != "/tmp" || entry.IsDir || entry.TotalBytes != 42 {
		t.Fatalf("unexpected transfer fields: %#v", entry)
	}
}

func TestBuildEntryUsesDefaults(t *testing.T) {
	entry := BuildEntry(host.Host{Name: "web-1"}, EntrySpec{})
	if entry.ID == "" || entry.Time == "" || entry.UpdatedAt == "" {
		t.Fatalf("expected generated id and timestamps: %#v", entry)
	}
	if entry.Status != config.TransferStatusQueued {
		t.Fatalf("status = %q, want %q", entry.Status, config.TransferStatusQueued)
	}
}
