package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type AppConfig struct {
	Language        string   `toml:"language"`
	RefreshInterval string   `toml:"refresh_interval"`
	ConnectTimeout  string   `toml:"connect_timeout"`
	CommandTimeout  string   `toml:"command_timeout"`
	ASCIIMode       bool     `toml:"ascii_mode"`
	CustomDirs      bool     `toml:"custom_dirs"`
	LocalDirs       []string `toml:"local_dirs"`
	RemoteDirs      []string `toml:"remote_dirs"`
	Thresholds      struct {
		CPUWarn  float64 `toml:"cpu_warn"`
		CPUCrit  float64 `toml:"cpu_crit"`
		MemWarn  float64 `toml:"mem_warn"`
		MemCrit  float64 `toml:"mem_crit"`
		DiskWarn float64 `toml:"disk_warn"`
		DiskCrit float64 `toml:"disk_crit"`
	} `toml:"thresholds"`
}

func DefaultAppConfig() AppConfig {
	cfg := AppConfig{
		Language:        "en",
		RefreshInterval: "5s",
		ConnectTimeout:  "2s",
		CommandTimeout:  "6s",
		LocalDirs:       []string{".", "~/Downloads", "~/Desktop", "~/Documents", "~"},
		RemoteDirs:      []string{"$HOME", "/home", "/opt", "/var/www", "/www", "/data", "/tmp"},
	}
	cfg.Thresholds.CPUWarn = 70
	cfg.Thresholds.CPUCrit = 85
	cfg.Thresholds.MemWarn = 70
	cfg.Thresholds.MemCrit = 85
	cfg.Thresholds.DiskWarn = 75
	cfg.Thresholds.DiskCrit = 90
	return cfg
}

func LoadAppConfig(home string) AppConfig {
	cfg := DefaultAppConfig()
	data, err := os.ReadFile(AppConfigPath(home))
	if err != nil {
		return cfg
	}
	_ = toml.Unmarshal(data, &cfg)
	cfg = NormalizeAppConfig(cfg)
	return cfg
}

func NormalizeAppConfig(cfg AppConfig) AppConfig {
	defaults := DefaultAppConfig()
	if cfg.Language != "en" && cfg.Language != "zh" {
		cfg.Language = defaults.Language
	}
	if !validDuration(cfg.RefreshInterval) {
		cfg.RefreshInterval = defaults.RefreshInterval
	}
	if !validDuration(cfg.ConnectTimeout) {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}
	if !validDuration(cfg.CommandTimeout) {
		cfg.CommandTimeout = defaults.CommandTimeout
	}
	if cfg.Thresholds.CPUWarn <= 0 {
		cfg.Thresholds.CPUWarn = defaults.Thresholds.CPUWarn
	}
	if cfg.Thresholds.CPUCrit <= 0 {
		cfg.Thresholds.CPUCrit = defaults.Thresholds.CPUCrit
	}
	if cfg.Thresholds.MemWarn <= 0 {
		cfg.Thresholds.MemWarn = defaults.Thresholds.MemWarn
	}
	if cfg.Thresholds.MemCrit <= 0 {
		cfg.Thresholds.MemCrit = defaults.Thresholds.MemCrit
	}
	if cfg.Thresholds.DiskWarn <= 0 {
		cfg.Thresholds.DiskWarn = defaults.Thresholds.DiskWarn
	}
	if cfg.Thresholds.DiskCrit <= 0 {
		cfg.Thresholds.DiskCrit = defaults.Thresholds.DiskCrit
	}
	return cfg
}

func validDuration(value string) bool {
	d, err := time.ParseDuration(value)
	return err == nil && d > 0
}

func SaveAppConfig(home string, cfg AppConfig) error {
	cfg = NormalizeAppConfig(cfg)
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := AppConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func AppConfigPath(home string) string {
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "sshm", "config.toml")
		}
	}
	return filepath.Join(home, ".config", "sshm", "config.toml")
}

func (c AppConfig) RefreshDuration() time.Duration {
	d, err := time.ParseDuration(c.RefreshInterval)
	if err != nil || d <= 0 {
		return 5 * time.Second
	}
	return d
}

func (c AppConfig) CommandDuration() time.Duration {
	d, err := time.ParseDuration(c.CommandTimeout)
	if err != nil || d <= 0 {
		return 6 * time.Second
	}
	return d
}

func (c AppConfig) ConnectDuration() time.Duration {
	d, err := time.ParseDuration(c.ConnectTimeout)
	if err != nil || d <= 0 {
		return 3 * time.Second
	}
	return d
}
