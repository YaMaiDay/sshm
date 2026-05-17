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
	m.settingsForm.RefreshInterval = "30"
	m.settingsForm.ConnectTimeout = "8"
	m.settingsForm.CommandTimeout = "20"
	m.settingsForm.ASCIIMode = true
	m.settingsForm.CustomDirs = true
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
	if got.mode != modeDashboard || got.status != "Settings saved." {
		t.Fatalf("mode/status = %v/%q", got.mode, got.status)
	}
	loaded := config.LoadAppConfig(home)
	if loaded.Language != "en" || loaded.RefreshInterval != "30s" || loaded.ConnectTimeout != "8s" || loaded.CommandTimeout != "20s" || !loaded.ASCIIMode || !loaded.CustomDirs {
		t.Fatalf("loaded config = %+v", loaded)
	}
	if loaded.Thresholds.DiskCrit != 92 || len(loaded.LocalDirs) != 2 || loaded.RemoteDirs[1] != "/data" {
		t.Fatalf("loaded thresholds/dirs = %+v %#v %#v", loaded.Thresholds, loaded.LocalDirs, loaded.RemoteDirs)
	}
}

func TestSettingsTextFieldsAcceptShortcutLetters(t *testing.T) {
	m := Model{appConfig: config.DefaultAppConfig()}
	m = m.startSettings()
	m.settingsField = settingsLocalDirs
	m.settingsForm.LocalDirs = "/tm"
	m.settingsCursor = len([]rune(m.settingsForm.LocalDirs))
	next, _ := m.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := next.(Model)
	if got.settingsForm.LocalDirs != "/tmp" {
		t.Fatalf("LocalDirs = %q, want /tmp", got.settingsForm.LocalDirs)
	}
	next, _ = got.updateSettings(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got = next.(Model)
	if got.settingsForm.LocalDirs != "/tmpq" || got.mode != modeSettings {
		t.Fatalf("LocalDirs/mode = %q/%v, want /tmpq/settings", got.settingsForm.LocalDirs, got.mode)
	}
}

func TestLocalRootItemsUsesAppConfig(t *testing.T) {
	home := t.TempDir()
	first := filepath.Join(home, "first")
	second := filepath.Join(home, "second")
	mustMkdir(t, first)
	mustMkdir(t, second)
	cfg := config.DefaultAppConfig()
	cfg.CustomDirs = true
	cfg.LocalDirs = []string{filepath.Join(home, "missing"), first, second, first}
	m := Model{home: home, appConfig: cfg}

	items := m.localRootItems(true)
	if len(items) != 2 || items[0].Path != first || items[1].Path != second {
		t.Fatalf("local roots = %#v", items)
	}
}

func TestLocalRootItemsUsesRootDirsWhenCustomDisabled(t *testing.T) {
	home := t.TempDir()
	cfg := config.DefaultAppConfig()
	cfg.CustomDirs = false
	cfg.LocalDirs = []string{filepath.Join(home, "missing")}
	m := Model{home: home, appConfig: cfg}

	items := m.localRootItems(true)
	foundRootChild := false
	for _, item := range items {
		if filepath.Dir(item.Path) == "/" {
			foundRootChild = true
		}
		if item.Path == filepath.Join(home, "missing") {
			t.Fatalf("custom dir was used while custom dirs disabled: %#v", items)
		}
	}
	if !foundRootChild {
		t.Fatalf("root dirs were not used: %#v", items)
	}
}

func TestLocalTreeItemsTreatsSymlinkDirectoryAsDirectoryAndDedupes(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "target")
	link := filepath.Join(home, "link")
	mustMkdir(t, target)
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}

	items := localTreeItems(home, true)
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one real directory after dedupe", items)
	}
	if !items[0].IsDir {
		t.Fatalf("symlink target should be treated as directory: %#v", items)
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

func TestShortcutKeyNormalizesPeriodForSettings(t *testing.T) {
	if got := shortcutKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}}); got != "." {
		t.Fatalf("shortcutKey(.) = %q", got)
	}
	if got := shortcutKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'。'}}); got != "." {
		t.Fatalf("shortcutKey(fullwidth period) = %q", got)
	}
}

func TestSettingsASCIIModeIsSecondSelectableField(t *testing.T) {
	m := Model{home: t.TempDir(), appConfig: config.DefaultAppConfig(), collector: monitor.NewCollector(config.PasswordStore{})}
	m = m.startSettings()
	m.moveSettingsField(1)
	if m.settingsField != settingsASCIIMode {
		t.Fatalf("second settings field = %d, want ASCII mode", m.settingsField)
	}
	next, _ := m.updateSettings(tea.KeyMsg{Type: tea.KeyRight})
	got := next.(Model)
	if !got.settingsForm.ASCIIMode {
		t.Fatal("ASCII mode should toggle on with right arrow")
	}
}

func TestSettingsDisplaysDurationAsSeconds(t *testing.T) {
	form := settingsFormFromConfig(config.AppConfig{
		Language:        "en",
		RefreshInterval: "30s",
		ConnectTimeout:  "1500ms",
		CommandTimeout:  "1m",
	})
	if form.RefreshInterval != "30" || form.ConnectTimeout != "1.5" || form.CommandTimeout != "60" {
		t.Fatalf("seconds form = %+v", form)
	}
}

func TestSettingsAcceptsSecondsWithoutUnit(t *testing.T) {
	m := Model{home: t.TempDir(), appConfig: config.DefaultAppConfig(), collector: monitor.NewCollector(config.PasswordStore{})}
	m = m.startSettings()
	m.settingsForm.RefreshInterval = "15"
	m.settingsForm.ConnectTimeout = "3.5"
	m.settingsForm.CommandTimeout = "40"
	cfg, err := m.settingsConfigFromForm()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshInterval != "15s" || cfg.ConnectTimeout != "3.5s" || cfg.CommandTimeout != "40s" {
		t.Fatalf("durations = %q %q %q", cfg.RefreshInterval, cfg.ConnectTimeout, cfg.CommandTimeout)
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
