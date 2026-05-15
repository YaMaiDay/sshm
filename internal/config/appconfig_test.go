package config

import (
	"os"
	"testing"
)

func TestSaveLoadAppConfig(t *testing.T) {
	home := t.TempDir()
	cfg := DefaultAppConfig()
	cfg.Language = "zh"
	cfg.RefreshInterval = "30s"
	cfg.ConnectTimeout = "8s"
	cfg.CommandTimeout = "20s"
	cfg.ASCIIMode = true
	cfg.CustomDirs = true
	cfg.LocalDirs = []string{"~/Downloads", "/tmp"}
	cfg.RemoteDirs = []string{"/opt", "/data"}
	cfg.Thresholds.CPUWarn = 65
	cfg.Thresholds.CPUCrit = 90
	if err := SaveAppConfig(home, cfg); err != nil {
		t.Fatal(err)
	}
	got := LoadAppConfig(home)
	if got.Language != "zh" || got.RefreshInterval != "30s" || got.ConnectTimeout != "8s" || got.CommandTimeout != "20s" || !got.ASCIIMode || !got.CustomDirs {
		t.Fatalf("loaded config = %+v", got)
	}
	if len(got.LocalDirs) != 2 || got.LocalDirs[0] != "~/Downloads" || len(got.RemoteDirs) != 2 || got.RemoteDirs[1] != "/data" {
		t.Fatalf("loaded dirs = local %#v remote %#v", got.LocalDirs, got.RemoteDirs)
	}
	info, err := os.Stat(AppConfigPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestNormalizeAppConfigDefaultsInvalidValues(t *testing.T) {
	cfg := NormalizeAppConfig(AppConfig{
		Language:        "bad",
		RefreshInterval: "bad",
		ConnectTimeout:  "0s",
		CommandTimeout:  "-1s",
	})
	defaults := DefaultAppConfig()
	if cfg.Language != defaults.Language || cfg.RefreshInterval != defaults.RefreshInterval || cfg.ConnectTimeout != defaults.ConnectTimeout || cfg.CommandTimeout != defaults.CommandTimeout {
		t.Fatalf("normalized config = %+v, defaults = %+v", cfg, defaults)
	}
	if cfg.Thresholds.CPUWarn != defaults.Thresholds.CPUWarn || cfg.Thresholds.DiskCrit != defaults.Thresholds.DiskCrit {
		t.Fatalf("thresholds should default: %+v", cfg.Thresholds)
	}
}

func TestNormalizeAppConfigKeepsEmptyDirsWhenCustomDisabled(t *testing.T) {
	cfg := NormalizeAppConfig(AppConfig{
		Language:        "en",
		RefreshInterval: "5s",
		ConnectTimeout:  "2s",
		CommandTimeout:  "6s",
		CustomDirs:      false,
	})
	if len(cfg.LocalDirs) != 0 || len(cfg.RemoteDirs) != 0 {
		t.Fatalf("dirs = local %#v remote %#v, want empty when custom dirs disabled", cfg.LocalDirs, cfg.RemoteDirs)
	}
}

func TestNormalizeAppConfigKeepsEmptyDirsWhenCustomEnabled(t *testing.T) {
	cfg := NormalizeAppConfig(AppConfig{
		Language:        "en",
		RefreshInterval: "5s",
		ConnectTimeout:  "2s",
		CommandTimeout:  "6s",
		CustomDirs:      true,
	})
	if len(cfg.LocalDirs) != 0 || len(cfg.RemoteDirs) != 0 {
		t.Fatalf("dirs = local %#v remote %#v, want empty when custom dirs enabled with empty values", cfg.LocalDirs, cfg.RemoteDirs)
	}
}
