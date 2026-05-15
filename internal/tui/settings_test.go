package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func TestSettingsSaveAppConfig(t *testing.T) {
	home := t.TempDir()
	cfg := config.DefaultAppConfig()
	m := Model{
		home:      home,
		appConfig: cfg,
		collector: monitor.NewCollector(config.PasswordStore{}),
	}
	m = m.startSettings()
	m.settingsForm.Language = "en"
	m.settingsForm.RefreshInterval = "30s"
	m.settingsForm.ConnectTimeout = "8s"
	m.settingsForm.CommandTimeout = "20s"
	m.settingsForm.ASCIIMode = true
	m.settingsForm.CPUWarn = "60"
	m.settingsForm.CPUCrit = "90"
	m.settingsForm.MemWarn = "61"
	m.settingsForm.MemCrit = "91"
	m.settingsForm.DiskWarn = "62"
	m.settingsForm.DiskCrit = "92"
	m.settingsForm.LocalDirs = "~/Downloads, /tmp"
	m.settingsForm.RemoteDirs = "/opt, /data"

	next, _ := m.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.mode != modeDashboard || got.status != "设置已保存。" {
		t.Fatalf("mode/status = %v/%q", got.mode, got.status)
	}
	loaded := config.LoadAppConfig(home)
	if loaded.Language != "en" || loaded.RefreshInterval != "30s" || loaded.ConnectTimeout != "8s" || loaded.CommandTimeout != "20s" || !loaded.ASCIIMode {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if loaded.Thresholds.DiskCrit != 92 || len(loaded.LocalDirs) != 2 || loaded.RemoteDirs[1] != "/data" {
		t.Fatalf("loaded thresholds/dirs = %+v %#v %#v", loaded.Thresholds, loaded.LocalDirs, loaded.RemoteDirs)
	}
}

func TestLocalRootItemsUsesAppConfig(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(home, "first")
	second := filepath.Join(home, "second")
	mustMkdir(t, first)
	mustMkdir(t, second)
	cfg := config.DefaultAppConfig()
	cfg.LocalDirs = []string{filepath.Join(home, "missing"), first, second, first}
	m := Model{home: home, appConfig: cfg}

	items := m.localRootItems(true)
	if len(items) != 2 || items[0].Path != first || items[1].Path != second {
		t.Fatalf("local roots = %#v", items)
	}
}

func TestShortcutKeyKeepsCaseAndWidthInsensitiveRunes(t *testing.T) {
	if got := shortcutKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}); got != "s" {
		t.Fatalf("shortcutKey(S) = %q", got)
	}
	if got := shortcutKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Ｓ'}}); got != "s" {
		t.Fatalf("shortcutKey(fullwidth S) = %q", got)
	}
}

func TestShortcutKeySupportsSettingsF2(t *testing.T) {
	if got := shortcutKey(tea.KeyMsg{Type: tea.KeyF2}); got != "f2" {
		t.Fatalf("shortcutKey(F2) = %q", got)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsRejectsInvalidThresholdOrder(t *testing.T) {
	m := Model{home: t.TempDir(), appConfig: config.DefaultAppConfig(), collector: monitor.NewCollector(config.PasswordStore{})}
	m = m.startSettings()
	m.settingsForm.CPUWarn = "95"
	m.settingsForm.CPUCrit = "90"
	next, _ := m.updateSettings(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.mode != modeSettings || got.status == "" {
		t.Fatalf("settings should stay open with validation status: mode=%v status=%q", got.mode, got.status)
	}
}
