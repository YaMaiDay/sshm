package config

import (
	"strings"
	"testing"
)

func TestTrimCommandHistoryOutputKeepsLastLines(t *testing.T) {
	lines := make([]string, 0, MaxCommandHistoryLines+5)
	for i := 0; i < MaxCommandHistoryLines+5; i++ {
		lines = append(lines, "line")
	}
	got := TrimCommandHistoryOutput(strings.Join(lines, "\n"))
	if count := len(strings.Split(got, "\n")); count != MaxCommandHistoryLines {
		t.Fatalf("line count = %d, want %d", count, MaxCommandHistoryLines)
	}
}

func TestSaveAndLoadCommandHistory(t *testing.T) {
	home := t.TempDir()
	file := CommandHistoryFile{Entries: []CommandHistoryEntry{{
		ID:      "1",
		Time:    "2026-05-12T00:00:00Z",
		Kind:    "single",
		Name:    "部署",
		Command: "git pull",
		Status:  "success",
		Targets: []CommandHistoryTarget{{
			Category: "aws",
			Name:     "demo",
			Status:   "success",
			Output:   "ok",
		}},
	}}}
	if err := SaveCommandHistory(home, file); err != nil {
		t.Fatalf("SaveCommandHistory: %v", err)
	}
	got, ok, err := LoadCommandHistory(home)
	if err != nil {
		t.Fatalf("LoadCommandHistory: %v", err)
	}
	if !ok {
		t.Fatal("LoadCommandHistory ok = false, want true")
	}
	if len(got.Entries) != 1 || got.Entries[0].Command != "git pull" {
		t.Fatalf("loaded history = %#v", got)
	}
}
