package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

const (
	settingsLanguage = iota
	settingsASCIIMode
	settingsRefreshInterval
	settingsConnectTimeout
	settingsCommandTimeout
	settingsCPUWarn
	settingsCPUCrit
	settingsMemWarn
	settingsMemCrit
	settingsDiskWarn
	settingsDiskCrit
	settingsCustomDirs
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
	CustomDirs      bool
	LocalDirs       string
	RemoteDirs      string
}

func settingsFormFromConfig(cfg config.AppConfig) settingsForm {
	cfg = config.NormalizeAppConfig(cfg)
	return settingsForm{
		Language:        cfg.Language,
		RefreshInterval: formatSettingSeconds(cfg.RefreshInterval),
		ConnectTimeout:  formatSettingSeconds(cfg.ConnectTimeout),
		CommandTimeout:  formatSettingSeconds(cfg.CommandTimeout),
		ASCIIMode:       cfg.ASCIIMode,
		CPUWarn:         formatSettingPercent(cfg.Thresholds.CPUWarn),
		CPUCrit:         formatSettingPercent(cfg.Thresholds.CPUCrit),
		MemWarn:         formatSettingPercent(cfg.Thresholds.MemWarn),
		MemCrit:         formatSettingPercent(cfg.Thresholds.MemCrit),
		DiskWarn:        formatSettingPercent(cfg.Thresholds.DiskWarn),
		DiskCrit:        formatSettingPercent(cfg.Thresholds.DiskCrit),
		CustomDirs:      cfg.CustomDirs,
		LocalDirs:       strings.Join(cfg.LocalDirs, ", "),
		RemoteDirs:      strings.Join(cfg.RemoteDirs, ", "),
	}
}

func (m Model) startSettings() Model {
	m.settings.Form = settingsFormFromConfig(m.appConfig)
	m.settings.Field = 0
	m.settings.Cursor = m.settingsValueLen()
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
	case "esc", "ctrl+c":
		m.mode = modeDashboard
		m.status = m.settingsText("Canceled.", "已取消。")
	case "tab", "down":
		m.moveSettingsField(1)
	case "shift+tab", "up":
		m.moveSettingsField(-1)
	case "left":
		if m.settings.Field == settingsLanguage || m.settings.Field == settingsASCIIMode || m.settings.Field == settingsCustomDirs {
			m.toggleSettingChoice()
		} else {
			m.moveSettingsCursor(-1)
		}
	case "right":
		if m.settings.Field == settingsLanguage || m.settings.Field == settingsASCIIMode || m.settings.Field == settingsCustomDirs {
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
		setASCIIMode(cfg.ASCIIMode)
		m.collector.Timeout = cfg.CommandDuration()
		m.collector.ConnectTimeout = cfg.ConnectDuration()
		m.mode = modeDashboard
		m.status = m.settingsText("Settings saved.", "设置已保存。")
	case "backspace":
		m.settingsBackspace()
	default:
		if len(msg.Runes) > 0 && m.settings.Field != settingsLanguage && m.settings.Field != settingsASCIIMode && m.settings.Field != settingsCustomDirs {
			m.settingsAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) settingsText(en string, zh string) string {
	return m.t(en, zh)
}

func (m *Model) moveSettingsField(delta int) {
	m.settings.Field = moveIndex(m.settings.Field, settingsFieldCount(), delta)
	m.settings.Cursor = m.settingsValueLen()
}

func settingsFieldCount() int { return settingsRemoteDirs + 1 }

func (m *Model) toggleSettingChoice() {
	switch m.settings.Field {
	case settingsLanguage:
		if m.settings.Form.Language == "zh" {
			m.settings.Form.Language = "en"
		} else {
			m.settings.Form.Language = "zh"
		}
	case settingsASCIIMode:
		m.settings.Form.ASCIIMode = !m.settings.Form.ASCIIMode
	case settingsCustomDirs:
		m.settings.Form.CustomDirs = !m.settings.Form.CustomDirs
	}
}

func (m Model) settingsValue() string {
	switch m.settings.Field {
	case settingsRefreshInterval:
		return m.settings.Form.RefreshInterval
	case settingsConnectTimeout:
		return m.settings.Form.ConnectTimeout
	case settingsCommandTimeout:
		return m.settings.Form.CommandTimeout
	case settingsCPUWarn:
		return m.settings.Form.CPUWarn
	case settingsCPUCrit:
		return m.settings.Form.CPUCrit
	case settingsMemWarn:
		return m.settings.Form.MemWarn
	case settingsMemCrit:
		return m.settings.Form.MemCrit
	case settingsDiskWarn:
		return m.settings.Form.DiskWarn
	case settingsDiskCrit:
		return m.settings.Form.DiskCrit
	case settingsLocalDirs:
		return m.settings.Form.LocalDirs
	case settingsRemoteDirs:
		return m.settings.Form.RemoteDirs
	default:
		return ""
	}
}

func (m Model) settingsValueLen() int {
	return len([]rune(m.settingsValue()))
}

func (m *Model) setSettingsValue(value string) {
	switch m.settings.Field {
	case settingsRefreshInterval:
		m.settings.Form.RefreshInterval = value
	case settingsConnectTimeout:
		m.settings.Form.ConnectTimeout = value
	case settingsCommandTimeout:
		m.settings.Form.CommandTimeout = value
	case settingsCPUWarn:
		m.settings.Form.CPUWarn = value
	case settingsCPUCrit:
		m.settings.Form.CPUCrit = value
	case settingsMemWarn:
		m.settings.Form.MemWarn = value
	case settingsMemCrit:
		m.settings.Form.MemCrit = value
	case settingsDiskWarn:
		m.settings.Form.DiskWarn = value
	case settingsDiskCrit:
		m.settings.Form.DiskCrit = value
	case settingsLocalDirs:
		m.settings.Form.LocalDirs = value
	case settingsRemoteDirs:
		m.settings.Form.RemoteDirs = value
	}
}

func (m *Model) settingsAppend(s string) {
	value := []rune(m.settingsValue())
	m.settings.Cursor = clampInt(m.settings.Cursor, 0, len(value))
	insert := []rune(s)
	next := append([]rune{}, value[:m.settings.Cursor]...)
	next = append(next, insert...)
	next = append(next, value[m.settings.Cursor:]...)
	m.setSettingsValue(string(next))
	m.settings.Cursor += len(insert)
}

func (m *Model) settingsBackspace() {
	if m.settings.Field == settingsLanguage || m.settings.Field == settingsASCIIMode || m.settings.Field == settingsCustomDirs {
		return
	}
	value := []rune(m.settingsValue())
	if m.settings.Cursor <= 0 || len(value) == 0 {
		return
	}
	m.settings.Cursor = clampInt(m.settings.Cursor, 0, len(value))
	next := append([]rune{}, value[:m.settings.Cursor-1]...)
	next = append(next, value[m.settings.Cursor:]...)
	m.setSettingsValue(string(next))
	m.settings.Cursor--
}

func (m *Model) moveSettingsCursor(delta int) {
	m.settings.Cursor = clampInt(m.settings.Cursor+delta, 0, m.settingsValueLen())
}

func (m Model) settingsConfigFromForm() (config.AppConfig, error) {
	cfg := m.appConfig
	cfg.Language = strings.TrimSpace(m.settings.Form.Language)
	if cfg.Language != "zh" && cfg.Language != "en" {
		return cfg, fmt.Errorf("%s", m.settingsText("language must be zh or en", "语言只能是 zh 或 en"))
	}
	refreshInterval, err := parseSettingSeconds(m.settingsText("refresh interval", "刷新间隔"), m.settings.Form.RefreshInterval, m.isChineseUI())
	if err != nil {
		return cfg, err
	}
	connectTimeout, err := parseSettingSeconds(m.settingsText("connect timeout", "连接超时"), m.settings.Form.ConnectTimeout, m.isChineseUI())
	if err != nil {
		return cfg, err
	}
	commandTimeout, err := parseSettingSeconds(m.settingsText("command timeout", "命令超时"), m.settings.Form.CommandTimeout, m.isChineseUI())
	if err != nil {
		return cfg, err
	}
	cfg.RefreshInterval = refreshInterval
	cfg.ConnectTimeout = connectTimeout
	cfg.CommandTimeout = commandTimeout
	cfg.ASCIIMode = m.settings.Form.ASCIIMode
	cfg.CustomDirs = m.settings.Form.CustomDirs
	if cfg.Thresholds.CPUWarn, err = parseSettingPercent(m.settingsText("CPU warn", "CPU 警告"), m.settings.Form.CPUWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.CPUCrit, err = parseSettingPercent(m.settingsText("CPU critical", "CPU 严重"), m.settings.Form.CPUCrit, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.MemWarn, err = parseSettingPercent(m.settingsText("memory warn", "内存警告"), m.settings.Form.MemWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.MemCrit, err = parseSettingPercent(m.settingsText("memory critical", "内存严重"), m.settings.Form.MemCrit, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.DiskWarn, err = parseSettingPercent(m.settingsText("disk warn", "磁盘警告"), m.settings.Form.DiskWarn, m.isChineseUI()); err != nil {
		return cfg, err
	}
	if cfg.Thresholds.DiskCrit, err = parseSettingPercent(m.settingsText("disk critical", "磁盘严重"), m.settings.Form.DiskCrit, m.isChineseUI()); err != nil {
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
	cfg.LocalDirs = splitSettingList(m.settings.Form.LocalDirs)
	cfg.RemoteDirs = splitSettingList(m.settings.Form.RemoteDirs)
	return config.NormalizeAppConfig(cfg), nil
}

func parseSettingSeconds(label string, value string, zh bool) (string, error) {
	text := strings.TrimSpace(value)
	d, err := time.ParseDuration(text)
	if err != nil && !strings.ContainsAny(text, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		seconds, parseErr := strconv.ParseFloat(text, 64)
		if parseErr == nil {
			d = time.Duration(seconds * float64(time.Second))
			err = nil
		}
	}
	if err != nil || d <= 0 {
		if !zh {
			return "", fmt.Errorf("%s must be a positive number of seconds", label)
		}
		return "", fmt.Errorf("%s需要填写大于 0 的秒数", label)
	}
	return formatDurationSeconds(d), nil
}

func formatSettingSeconds(value string) string {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || d <= 0 {
		return strings.TrimSpace(value)
	}
	seconds := d.Seconds()
	if math.Trunc(seconds) == seconds {
		return strconv.FormatInt(int64(seconds), 10)
	}
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func formatDurationSeconds(d time.Duration) string {
	seconds := d.Seconds()
	if math.Trunc(seconds) == seconds {
		return strconv.FormatInt(int64(seconds), 10) + "s"
	}
	return strconv.FormatFloat(seconds, 'f', -1, 64) + "s"
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
