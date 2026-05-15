package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

const (
	settingsLanguage = iota
	settingsRefreshInterval
	settingsConnectTimeout
	settingsCommandTimeout
	settingsASCIIMode
	settingsCPUWarn
	settingsCPUCrit
	settingsMemWarn
	settingsMemCrit
	settingsDiskWarn
	settingsDiskCrit
	settingsLocalDirs
	settingsRemoteDirs
)

type settingsForm struct {
	Language        string
	RefreshInterval string
	ConnectTimeout  string
	CommandTimeout  string
	ASCIIMode       bool
	CPUWarn         string
	CPUCrit         string
	MemWarn         string
	MemCrit         string
	DiskWarn        string
	DiskCrit        string
	LocalDirs       string
	RemoteDirs      string
}

func settingsFormFromConfig(cfg config.AppConfig) settingsForm {
	cfg = config.NormalizeAppConfig(cfg)
	return settingsForm{
		Language:        cfg.Language,
		RefreshInterval: cfg.RefreshInterval,
		ConnectTimeout:  cfg.ConnectTimeout,
		CommandTimeout:  cfg.CommandTimeout,
		ASCIIMode:       cfg.ASCIIMode,
		CPUWarn:         formatSettingPercent(cfg.Thresholds.CPUWarn),
		CPUCrit:         formatSettingPercent(cfg.Thresholds.CPUCrit),
		MemWarn:         formatSettingPercent(cfg.Thresholds.MemWarn),
		MemCrit:         formatSettingPercent(cfg.Thresholds.MemCrit),
		DiskWarn:        formatSettingPercent(cfg.Thresholds.DiskWarn),
		DiskCrit:        formatSettingPercent(cfg.Thresholds.DiskCrit),
		LocalDirs:       strings.Join(cfg.LocalDirs, ", "),
		RemoteDirs:      strings.Join(cfg.RemoteDirs, ", "),
	}
}

func (m Model) startSettings() Model {
	m.settingsForm = settingsFormFromConfig(m.appConfig)
	m.settingsField = 0
	m.settingsCursor = m.settingsValueLen()
	m.mode = modeSettings
	if m.isChineseUI() {
		m.status = "设置"
	} else {
		m.status = "Settings"
	}
	return m
}

func (m Model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = m.settingsText("Canceled.", "已取消。")
	case "tab", "down", "j":
		m.moveSettingsField(1)
	case "shift+tab", "up", "k":
		m.moveSettingsField(-1)
	case "left", "h":
		if m.settingsField == settingsLanguage || m.settingsField == settingsASCIIMode {
			m.toggleSettingChoice()
		} else {
			m.moveSettingsCursor(-1)
		}
	case "right", "l":
		if m.settingsField == settingsLanguage || m.settingsField == settingsASCIIMode {
			m.toggleSettingChoice()
		} else {
			m.moveSettingsCursor(1)
		}
	case "enter":
		cfg, err := m.settingsConfigFromForm()
		if err != nil {
			m.status = m.settingsText("Save failed: ", "保存失败：") + err.Error()
			return m, nil
		}
		if err := config.SaveAppConfig(m.home, cfg); err != nil {
			m.status = m.settingsText("Save failed: ", "保存失败：") + err.Error()
			return m, nil
		}
		m.appConfig = cfg
		m.collector.Timeout = cfg.CommandDuration()
		m.collector.ConnectTimeout = cfg.ConnectDuration()
		m.mode = modeDashboard
		m.status = m.settingsText("Settings saved.", "设置已保存。")
	case "backspace":
		m.settingsBackspace()
	default:
		if len(msg.Runes) > 0 && m.settingsField != settingsLanguage && m.settingsField != settingsASCIIMode {
			m.settingsAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) settingsText(en string, zh string) string {
	if m.isChineseUI() {
		return zh
	}
	return en
}

func (m *Model) moveSettingsField(delta int) {
	m.settingsField = moveIndex(m.settingsField, settingsFieldCount(), delta)
	m.settingsCursor = m.settingsValueLen()
}

func settingsFieldCount() int { return settingsRemoteDirs + 1 }

func (m *Model) toggleSettingChoice() {
	switch m.settingsField {
	case settingsLanguage:
		if m.settingsForm.Language == "zh" {
			m.settingsForm.Language = "en"
		} else {
			m.settingsForm.Language = "zh"
		}
	case settingsASCIIMode:
		m.settingsForm.ASCIIMode = !m.settingsForm.ASCIIMode
	}
}

func (m Model) settingsValue() string {
	switch m.settingsField {
	case settingsRefreshInterval:
		return m.settingsForm.RefreshInterval
	case settingsConnectTimeout:
		return m.settingsForm.ConnectTimeout
	case settingsCommandTimeout:
		return m.settingsForm.CommandTimeout
	case settingsCPUWarn:
		return m.settingsForm.CPUWarn
	case settingsCPUCrit:
		return m.settingsForm.CPUCrit
	case settingsMemWarn:
		return m.settingsForm.MemWarn
	case settingsMemCrit:
		return m.settingsForm.MemCrit
	case settingsDiskWarn:
		return m.settingsForm.DiskWarn
	case settingsDiskCrit:
		return m.settingsForm.DiskCrit
	case settingsLocalDirs:
		return m.settingsForm.LocalDirs
	case settingsRemoteDirs:
		return m.settingsForm.RemoteDirs
	default:
		return ""
	}
}

func (m Model) settingsValueLen() int {
	return len([]rune(m.settingsValue()))
}

func (m *Model) setSettingsValue(value string) {
	switch m.settingsField {
	case settingsRefreshInterval:
		m.settingsForm.RefreshInterval = value
	case settingsConnectTimeout:
		m.settingsForm.ConnectTimeout = value
	case settingsCommandTimeout:
		m.settingsForm.CommandTimeout = value
	case settingsCPUWarn:
		m.settingsForm.CPUWarn = value
	case settingsCPUCrit:
		m.settingsForm.CPUCrit = value
	case settingsMemWarn:
		m.settingsForm.MemWarn = value
	case settingsMemCrit:
		m.settingsForm.MemCrit = value
	case settingsDiskWarn:
		m.settingsForm.DiskWarn = value
	case settingsDiskCrit:
		m.settingsForm.DiskCrit = value
	case settingsLocalDirs:
		m.settingsForm.LocalDirs = value
	case settingsRemoteDirs:
		m.settingsForm.RemoteDirs = value
	}
}

func (m *Model) settingsAppend(s string) {
	value := []rune(m.settingsValue())
	m.settingsCursor = clampInt(m.settingsCursor, 0, len(value))
	insert := []rune(s)
	next := append([]rune{}, value[:m.settingsCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.settingsCursor:]...)
	m.setSettingsValue(string(next))
	m.settingsCursor += len(insert)
}

func (m *Model) settingsBackspace() {
	if m.settingsField == settingsLanguage || m.settingsField == settingsASCIIMode {
		return
	}
	value := []rune(m.settingsValue())
	if m.settingsCursor <= 0 || len(value) == 0 {
		return
	}
	m.settingsCursor = clampInt(m.settingsCursor, 0, len(value))
	next := append([]rune{}, value[:m.settingsCursor-1]...)
	next = append(next, value[m.settingsCursor:]...)
	m.setSettingsValue(string(next))
	m.settingsCursor--
}

func (m *Model) moveSettingsCursor(delta int) {
	m.settingsCursor = clampInt(m.settingsCursor+delta, 0, m.settingsValueLen())
}

func (m Model) settingsConfigFromForm() (config.AppConfig, error) {
	cfg := m.appConfig
	cfg.Language = strings.TrimSpace(m.settingsForm.Language)
	if cfg.Language != "zh" && cfg.Language != "en" {
		return cfg, fmt.Errorf("%s", m.settingsText("language must be zh or en", "语言只能是 zh 或 en"))
	}
	if err := validateSettingDuration(m.settingsText("refresh interval", "刷新间隔"), m.settingsForm.RefreshInterval, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if err := validateSettingDuration(m.settingsText("connect timeout", "连接超时"), m.settingsForm.ConnectTimeout, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if err := validateSettingDuration(m.settingsText("command timeout", "命令超时"), m.settingsForm.CommandTimeout, m.isChineseUI()); err != nil {
		return cfg, err
	}
	cfg.RefreshInterval = strings.TrimSpace(m.settingsForm.RefreshInterval)
	cfg.ConnectTimeout = strings.TrimSpace(m.settingsForm.ConnectTimeout)
	cfg.CommandTimeout = strings.TrimSpace(m.settingsForm.CommandTimeout)
	cfg.ASCIIMode = m.settingsForm.ASCIIMode
	var err error
	if cfg.Thresholds.CPUWarn, err = parseSettingPercent(m.settingsText("CPU warn", "CPU 警告"), m.settingsForm.CPUWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.CPUCrit, err = parseSettingPercent(m.settingsText("CPU critical", "CPU 严重"), m.settingsForm.CPUCrit, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.MemWarn, err = parseSettingPercent(m.settingsText("memory warn", "内存警告"), m.settingsForm.MemWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.MemCrit, err = parseSettingPercent(m.settingsText("memory critical", "内存严重"), m.settingsForm.MemCrit, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.DiskWarn, err = parseSettingPercent(m.settingsText("disk warn", "磁盘警告"), m.settingsForm.DiskWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.DiskCrit, err = parseSettingPercent(m.settingsText("disk critical", "磁盘严重"), m.settingsForm.DiskCrit, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.CPUWarn > cfg.Thresholds.CPUCrit {
		return cfg, fmt.Errorf("%s", m.settingsText("CPU warn threshold cannot exceed critical threshold", "CPU 警告阈值不能大于严重阈值"))
	}
	if cfg.Thresholds.MemWarn > cfg.Thresholds.MemCrit {
		return cfg, fmt.Errorf("%s", m.settingsText("memory warn threshold cannot exceed critical threshold", "内存警告阈值不能大于严重阈值"))
	}
	if cfg.Thresholds.DiskWarn > cfg.Thresholds.DiskCrit {
		return cfg, fmt.Errorf("%s", m.settingsText("disk warn threshold cannot exceed critical threshold", "磁盘警告阈值不能大于严重阈值"))
	}
	defaults := config.DefaultAppConfig()
	cfg.LocalDirs = splitSettingList(m.settingsForm.LocalDirs)
	if len(cfg.LocalDirs) == 0 {
		cfg.LocalDirs = defaults.LocalDirs
	}
	cfg.RemoteDirs = splitSettingList(m.settingsForm.RemoteDirs)
	if len(cfg.RemoteDirs) == 0 {
		cfg.RemoteDirs = defaults.RemoteDirs
	}
	return config.NormalizeAppConfig(cfg), nil
}

func validateSettingDuration(label string, value string, zh bool) error {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || d <= 0 {
		if !zh {
			return fmt.Errorf("%s must be a valid duration, for example 5s, 30s, or 1m", label)
		}
		return fmt.Errorf("%s需要填写有效时间，例如 5s、30s、1m", label)
	}
	return nil
}

func parseSettingPercent(label string, value string, zh bool) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, "%")), 64)
	if err != nil || n <= 0 || n > 100 {
		if !zh {
			return 0, fmt.Errorf("%s must be 1-100", label)
		}
		return 0, fmt.Errorf("%s需要填写 1-100", label)
	}
	return n, nil
}

func splitSettingList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func formatSettingPercent(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}
