package config

import (
	"testing"
	"time"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestSaveAndLoadState(t *testing.T) {
	home := t.TempDir()
	h := host.Host{Category: "aws", Name: "demo"}
	at := time.Date(2026, 5, 12, 10, 30, 0, 0, time.UTC)
	state := AppState{}
	SetLastLogin(&state, h, at)
	if err := SaveState(home, state); err != nil {
		t.Fatal(err)
	}

	loaded := LoadState(home)
	got := LastLoginFor(loaded, h)
	if !got.Equal(at) {
		t.Fatalf("LastLogin = %s, want %s", got, at)
	}
}
