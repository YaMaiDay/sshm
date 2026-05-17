package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) updateResourceDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", " ":
		m.mode = modeResourceList
		m.resourceDetailName = ""
	case "j":
		m = m.moveResourceDetailScroll(1)
	case "down":
		m = m.moveResourceDetailScroll(3)
	case "k":
		m = m.moveResourceDetailScroll(-1)
	case "up":
		m = m.moveResourceDetailScroll(-3)
	case "pgdown", "ctrl+d":
		m = m.moveResourceDetailScroll(maxInt(1, m.resourceDetailBodyHeight()/2))
	case "pgup", "ctrl+u":
		m = m.moveResourceDetailScroll(-maxInt(1, m.resourceDetailBodyHeight()/2))
	case "o":
		return m.openResourceLog()
	case "e":
		return m.startResourceCommandEdit()
	case "x":
		return m.startSelectedResourceRemoveConfirm()
	case "s":
		return m.startResourceAction(resourceActionStart)
	case "p":
		return m.startResourceAction(resourceActionStop)
	case "c":
		return m.startResourceAction(resourceActionRestart)
	case "r":
		m, refreshCmd := m.refreshResourceDetails(m.resourceKind)
		m, extraCmd := m.refreshSelectedResourceExtra()
		return m, tea.Batch(refreshCmd, extraCmd)
	}
	return m, nil
}

func (m Model) moveResourceDetailScroll(delta int) Model {
	maxScroll := m.resourceDetailMaxScroll()
	m.resourceScroll = moveClampedInt(m.resourceScroll, delta, 0, maxScroll)
	return m
}

func (m Model) resourceDetailBodyHeight() int {
	return maxInt(1, m.height-4)
}

func (m Model) resourceDetailMaxScroll() int {
	lines := expandLines(m.resourceDetailLines())
	return maxInt(0, len(lines)-m.resourceDetailBodyHeight())
}

func (m Model) refreshResourceDetails(kind resourceKind) (Model, tea.Cmd) {
	m.resourceLoading = true
	m.resourceLoadingKind = kind
	m.resourceLoadingPending = resourceLoadPartCount(kind)
	m.resourceManualRefresh = true
	m.status = m.t("Refreshing resources...", "正在刷新资源...")
	return m, m.fetchResourceDetails(m.resourceHostIndex, kind)
}

func (m Model) refreshSelectedResourceExtra() (Model, tea.Cmd) {
	ref, ok := m.selectedResourceRef()
	if !ok {
		return m, nil
	}
	switch ref.Kind {
	case resourceContainers:
		item, ok := m.selectedContainer()
		if !ok {
			return m, nil
		}
		m.resourceContainerExtraName = item.Name
		m.resourceContainerExtra = resourceservice.ContainerExtraDetail{}
		m.resourceContainerExtraErr = ""
		m.resourceContainerExtraLoading = true
		return m, m.fetchContainerExtraDetail(m.resourceHostIndex, item.Name)
	case resourceServices:
		item, ok := m.selectedService()
		if !ok {
			return m, nil
		}
		m.resourceServiceExtraName = item.Unit
		m.resourceServiceExtra = resourceservice.ServiceDetail{}
		m.resourceServiceExtraErr = ""
		m.resourceServiceExtraLoading = true
		return m, m.fetchServiceExtraDetail(m.resourceHostIndex, item.Unit)
	case resourceProcesses:
		item, ok := m.selectedProcess()
		if !ok {
			return m, nil
		}
		m.resourceProcessExtraPID = item.PID
		m.resourceProcessExtra = resourceservice.ProcessExtraDetail{}
		m.resourceProcessExtraErr = ""
		m.resourceProcessExtraLoading = true
		return m, m.fetchProcessExtraDetail(m.resourceHostIndex, item.PID)
	case resourceDatabases:
		item, ok := m.selectedDatabase()
		if !ok {
			return m, nil
		}
		m.resourceDatabaseExtraName = item.Name
		m.resourceDatabaseExtra = resourceservice.DatabaseExtraDetail{}
		m.resourceDatabaseExtraErr = ""
		m.resourceDatabaseExtraLoading = true
		return m, m.fetchDatabaseExtraDetail(m.resourceHostIndex, item.Name)
	default:
		return m, nil
	}
}

func (m Model) fetchContainerExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).FetchContainerDetail(ctx, h, name)
		errText := result.ErrText
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		return resourceContainerDetailMsg{Index: index, Name: name, Detail: result.Detail, Err: errText}
	}
}

func (m Model) fetchServiceExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).FetchServiceDetail(ctx, h, name)
		errText := result.ErrText
		if strings.TrimSpace(result.Detail.Unit) != "" {
			errText = ""
		} else if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		if !meaningfulResourceDetailError(errText) {
			errText = ""
		}
		return resourceServiceDetailMsg{Index: index, Name: name, Detail: result.Detail, Err: errText}
	}
}

func (m Model) fetchProcessExtraDetail(index int, pid string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(pid) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).FetchProcessDetail(ctx, h, pid)
		errText := result.ErrText
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		return resourceProcessDetailMsg{Index: index, PID: pid, Detail: result.Detail, Err: errText}
	}
}

func (m Model) fetchDatabaseExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	item, ok := m.managedResource(resourceDatabases, name)
	if !ok || strings.TrimSpace(item.DBEngine) == "" {
		return func() tea.Msg {
			return resourceDatabaseDetailMsg{Index: index, Name: name, Detail: resourceservice.DatabaseExtraDetail{}, Err: m.t("Database connection is not configured. Press e to configure it.", "未配置数据库连接。按 e 配置。")}
		}
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	db := resourceservice.DatabaseDetail{}
	for _, detail := range m.states[index].DatabaseDetails {
		if detail.Name == name {
			db = detail
			break
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := (resourceservice.Service{}).FetchDatabaseDetail(ctx, h, item, db)
		detail := result.Detail
		errText := result.ErrText
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		detail.Engine = item.DBEngine
		detail.Host = item.DBHost
		detail.Port = item.DBPort
		detail.User = item.DBUser
		detail.Database = strings.TrimSpace(item.DBName)
		if detail.Database == "" {
			detail.Database = resourceservice.DatabaseDefaultName(item.DBEngine)
		}
		if resourceservice.DatabaseMissingCredentialHint(item, errText) {
			errText = m.t("Database credentials are not configured or authentication failed. Press e to set user/password.", "未配置数据库账号密码或认证失败。按 e 配置用户和密码。")
		}
		return resourceDatabaseDetailMsg{Index: index, Name: name, Detail: detail, Err: errText}
	}
}

func (m Model) fetchDatabaseCardExtras(index int) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	if m.resourceKind != resourceDatabases && m.resourceKind != resourceAll && m.resourceAddKind != resourceDatabases {
		return nil
	}
	cmds := []tea.Cmd{}
	for _, db := range m.states[index].DatabaseDetails {
		if !db.Managed || db.Missing || strings.TrimSpace(db.Name) == "" {
			continue
		}
		if cache, ok := m.databaseExtraCache(db.Name); ok && (cache.Loading || cache.Err == "" && cache.Detail.Version != "") {
			continue
		}
		m.setDatabaseExtraCache(db.Name, resourceservice.DatabaseExtraDetail{}, "", true)
		cmds = append(cmds, m.fetchDatabaseExtraDetail(index, db.Name))
	}
	return tea.Batch(cmds...)
}

func (m Model) databaseExtraCache(name string) (databaseExtraCache, bool) {
	if m.resourceDatabaseExtraCache == nil {
		return databaseExtraCache{}, false
	}
	cache, ok := m.resourceDatabaseExtraCache[name]
	return cache, ok
}

func (m *Model) setDatabaseExtraCache(name string, detail resourceservice.DatabaseExtraDetail, errText string, loading bool) {
	if strings.TrimSpace(name) == "" {
		return
	}
	if m.resourceDatabaseExtraCache == nil {
		m.resourceDatabaseExtraCache = map[string]databaseExtraCache{}
	}
	m.resourceDatabaseExtraCache[name] = databaseExtraCache{Detail: detail, Err: strings.TrimSpace(errText), Loading: loading}
}

func meaningfulResourceDetailError(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.Contains(value, "=") && !strings.ContainsAny(value, " \n\t:/") {
		return false
	}
	needles := []string{
		"error", "failed", "permission", "denied", "timeout", "deadline", "killed",
		"exit status", "not found", "no such", "unavailable", "cannot", "refused",
		"错误", "失败", "权限", "拒绝", "超时", "不可用", "不存在",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) || strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func (m Model) resourceRemoteErrorText(err error) string {
	if err == nil {
		return ""
	}
	return m.friendlyResourceErrorText(err.Error())
}

func (m Model) friendlyResourceErrorText(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "exit status 255"):
		return m.t("SSH connection failed", "SSH连接失败")
	case strings.Contains(lower, "context deadline exceeded"):
		return m.t("Resource read timed out", "资源读取超时")
	default:
		return text
	}
}
