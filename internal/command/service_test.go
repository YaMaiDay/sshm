package command

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestSingleHistoryEntry(t *testing.T) {
	at := time.Date(2026, 5, 17, 10, 20, 30, 0, time.UTC)
	h := host.Host{Category: "prod", Name: "api", HostName: "10.0.0.1", User: "root", Port: "22"}
	entry := SingleHistoryEntry(h, "uptime", "uptime", Result{Output: "ok\n", ExitCode: 0}, at)

	if entry.ID == "" || entry.Time != at.Format(time.RFC3339) || entry.Kind != "single" {
		t.Fatalf("unexpected metadata: %#v", entry)
	}
	if entry.Status != "success" || entry.ExitCode != 0 || len(entry.Targets) != 1 {
		t.Fatalf("unexpected status/targets: %#v", entry)
	}
	target := entry.Targets[0]
	if target.Category != "prod" || target.Name != "api" || target.Output != "ok" {
		t.Fatalf("unexpected target: %#v", target)
	}
}

func TestBatchHistoryEntryCountsFailures(t *testing.T) {
	at := time.Date(2026, 5, 17, 10, 20, 30, 0, time.UTC)
	output := strings.Repeat("line\n", 250)
	entry := BatchHistoryEntry("deploy", "deploy", []BatchTarget{
		{Host: host.Host{Name: "ok"}, Output: "done", ExitCode: 0},
		{Host: host.Host{Name: "bad"}, Output: output, ExitCode: 2, Err: errors.New("failed")},
	}, at)

	if entry.Kind != "batch" || entry.Status != "failed" || entry.ExitCode != 1 {
		t.Fatalf("unexpected batch status: %#v", entry)
	}
	if len(entry.Targets) != 2 || entry.Targets[1].Status != "failed" || entry.Targets[1].ExitCode != 2 {
		t.Fatalf("unexpected targets: %#v", entry.Targets)
	}
	if gotLines := strings.Count(entry.Targets[1].Output, "\n") + 1; gotLines != 200 {
		t.Fatalf("trimmed output lines = %d, want 200", gotLines)
	}
}
