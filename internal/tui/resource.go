package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startResourceList(index int, kind resourceKind, back viewMode) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	if file, _, err := config.LoadResources(m.home); err == nil {
		m.resourceFile = file
	} else {
		m.status = m.t("Failed to read resource config: ", "读取资源配置失败：") + err.Error()
	}
	m.mode = modeResourceList
	m.resourceHostIndex = index
	m.resourceBackMode = back
	m.resourceKind = kind
	m.resourceScope = resourceScopeDiscovered
	m.resourceIndex = 0
	m.resourceScroll = 0
	m.resourceQuery = ""
	m.resourceSearch = false
	m.applyCachedResourceDetails(index, kind)
	m.applyManagedResources(index)
	m.resourceLoading = true
	m.resourceLoadingKind = kind
	m.resourceLoadingPending = resourceLoadPartCount(kind)
	m.resourceManualRefresh = false
	m.resourceRefreshStatus = ""
	m.status = m.t("Loading resources...", "正在读取资源...")
	return m, m.fetchResourceDetails(index, kind)
}

func resourceLoadPartCount(kind resourceKind) int {
	switch kind {
	case resourceAll:
		return 3
	case resourceDatabases:
		return 3
	case resourceContainers, resourcePorts:
		return 1
	default:
		return 1
	}
}

func (m *Model) applyCachedResourceDetails(index int, kind resourceKind) {
	if index < 0 || index >= len(m.states) {
		return
	}
	if kind != resourceAll && kind != resourceContainers && kind != resourceDatabases {
		return
	}
	if len(m.states[index].ContainerDetails) > 0 {
		return
	}
	file, _, err := config.LoadResourceCache(m.home)
	if err != nil {
		return
	}
	items, ok := config.ResourceContainerCacheForServer(file, m.resourceServerKey(index))
	if !ok {
		return
	}
	m.states[index].ContainerDetails = containerDetailsFromCache(items)
	m.states[index].ContainerError = ""
	associatePortContainers(m.states[index].PortDetails, m.states[index].ContainerDetails)
	m.states[index].DatabaseDetails, m.states[index].DatabaseError = deriveDatabaseDetails(m.states[index].ServiceDetails, m.states[index].ContainerDetails, m.states[index].PortDetails)
}

func (m Model) fetchResourceDetails(index int, kind resourceKind) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	cmds := []tea.Cmd{}
	if kind == resourceAll || kind == resourceServices {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourceServices))
	}
	if kind == resourceAll || kind == resourceContainers {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourceContainers))
	}
	if kind == resourceAll || kind == resourcePorts {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourcePorts))
	}
	if kind == resourceDatabases {
		cmds = append(cmds,
			m.fetchResourcePart(index, kind, resourceServices),
			m.fetchResourcePart(index, kind, resourceContainers),
			m.fetchResourcePart(index, kind, resourcePorts),
		)
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchResourcePart(index int, requested resourceKind, part resourceKind) tea.Cmd {
	if index < 0 || index >= len(m.states) {
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
		msg := resourceLoadMsg{Index: index, Kind: part, Requested: requested}
		switch part {
		case resourceServices:
			serviceResult, serviceCleanup := actions.RemoteCommandContext(ctx, h, serviceListScript())
			serviceCleanup()
			msg.Services, msg.ServiceErr = parseServiceDetails(serviceResult.Output)
			if serviceResult.Err != nil && msg.ServiceErr == "" {
				msg.ServiceErr = m.resourceRemoteErrorText(serviceResult.Err)
			}
		case resourceContainers:
			containerResult, containerCleanup := actions.RemoteCommandContext(ctx, h, containerDetailScript())
			containerCleanup()
			msg.Containers, msg.ContainerErr = parseContainerDetails(containerResult.Output)
			if containerResult.Err != nil && msg.ContainerErr == "" {
				msg.ContainerErr = m.resourceRemoteErrorText(containerResult.Err)
			}
		case resourcePorts:
			portResult, portCleanup := actions.RemoteCommandContext(ctx, h, portDetailScript())
			portCleanup()
			msg.Ports, msg.PortsErrText = parsePortDetails(portResult.Output)
			if portResult.Err != nil && msg.PortsErrText == "" {
				msg.PortsErrText = m.resourceRemoteErrorText(portResult.Err)
			}
		}
		return msg
	}
}

func (m Model) updateResourceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.resourceSearch {
		return m.updateResourceSearch(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = m.resourceBackMode
		m.status = ""
		m.resourceDetailName = ""
	case "tab":
		m.resourceKind = (m.resourceKind + 1) % 6
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "g":
		m.cycleResourceListFilter()
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "v":
		if m.resourceScope == resourceScopeManaged {
			m.resourceScope = resourceScopeDiscovered
		} else {
			m.resourceScope = resourceScopeManaged
		}
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "f":
		return m.toggleManagedResource()
	case "t":
		return m.toggleResourcePinned()
	case "y":
		m.resourceSort = (m.resourceSort + 1) % 6
		m.resourceIndex = 0
		m.resourceScroll = 0
		m.status = m.t("Sort: ", "排序：") + m.resourceSortName(m.resourceSort)
	case "x":
		return m.startSelectedResourceRemoveConfirm()
	case "j", "down":
		m.moveResourceDown()
	case "k", "up":
		m.moveResourceUp()
	case "h", "left":
		m.moveResourceLeft()
	case "l", "right":
		m.moveResourceRight()
	case "o":
		return m.openResourceLog()
	case "e":
		return m.startResourceCommandEdit()
	case "a":
		return m.startResourceAdd()
	case "/":
		m.resourceSearch = true
		m.resourceQuery = ""
		m.resourceIndex = 0
	case "z":
		if m.resourceView == resourceViewCards {
			m.resourceView = resourceViewList
		} else {
			m.resourceView = resourceViewCards
		}
		m.resourceScroll = 0
	case "s":
		return m.startResourceAction(resourceActionStart)
	case "p":
		return m.startResourceAction(resourceActionStop)
	case "c":
		return m.startResourceAction(resourceActionRestart)
	case "r":
		return m.refreshResourceDetails(m.resourceKind)
	case " ", "enter":
		m.mode = modeResourceDetail
		m.resourceScroll = 0
		if ref, ok := m.selectedResourceRef(); ok {
			m.resourceDetailKind = ref.Kind
			if name, nameOK := m.selectedResourceName(); nameOK {
				m.resourceDetailName = name
			} else {
				m.resourceDetailName = ""
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceContainers {
			if item, ok := m.selectedContainer(); ok {
				m.resourceContainerExtraName = item.Name
				m.resourceContainerExtra = containerExtraDetail{}
				m.resourceContainerExtraErr = ""
				m.resourceContainerExtraLoading = true
				return m, m.fetchContainerExtraDetail(m.resourceHostIndex, item.Name)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceServices {
			if item, ok := m.selectedService(); ok {
				m.resourceServiceExtraName = item.Unit
				m.resourceServiceExtra = serviceDetail{}
				m.resourceServiceExtraErr = ""
				m.resourceServiceExtraLoading = true
				return m, m.fetchServiceExtraDetail(m.resourceHostIndex, item.Unit)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceProcesses {
			if item, ok := m.selectedProcess(); ok {
				m.resourceProcessExtraPID = item.PID
				m.resourceProcessExtra = processExtraDetail{}
				m.resourceProcessExtraErr = ""
				m.resourceProcessExtraLoading = true
				return m, m.fetchProcessExtraDetail(m.resourceHostIndex, item.PID)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceDatabases {
			if item, ok := m.selectedDatabase(); ok {
				m.resourceDatabaseExtraName = item.Name
				m.resourceDatabaseExtra = databaseExtraDetail{}
				m.resourceDatabaseExtraErr = ""
				m.resourceDatabaseExtraLoading = true
				return m, m.fetchDatabaseExtraDetail(m.resourceHostIndex, item.Name)
			}
		}
	}
	return m, nil
}

func (m *Model) cycleResourceListFilter() {
	if m.resourceKind == resourcePorts {
		m.resourcePortFilter = (m.resourcePortFilter + 1) % 6
		return
	}
	m.resourceFilter = (m.resourceFilter + 1) % 4
}

func (m Model) updateResourceSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.resourceSearch = false
		m.resourceQuery = ""
		m.resourceIndex = 0
	case "enter":
		m.resourceSearch = false
	case "backspace":
		r := []rune(m.resourceQuery)
		if len(r) > 0 {
			m.resourceQuery = string(r[:len(r)-1])
		}
		m.resourceIndex = 0
	default:
		if len(msg.Runes) > 0 {
			m.resourceQuery += string(msg.Runes)
			m.resourceIndex = 0
		}
	}
	return m, nil
}

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
		m.resourceContainerExtra = containerExtraDetail{}
		m.resourceContainerExtraErr = ""
		m.resourceContainerExtraLoading = true
		return m, m.fetchContainerExtraDetail(m.resourceHostIndex, item.Name)
	case resourceServices:
		item, ok := m.selectedService()
		if !ok {
			return m, nil
		}
		m.resourceServiceExtraName = item.Unit
		m.resourceServiceExtra = serviceDetail{}
		m.resourceServiceExtraErr = ""
		m.resourceServiceExtraLoading = true
		return m, m.fetchServiceExtraDetail(m.resourceHostIndex, item.Unit)
	case resourceProcesses:
		item, ok := m.selectedProcess()
		if !ok {
			return m, nil
		}
		m.resourceProcessExtraPID = item.PID
		m.resourceProcessExtra = processExtraDetail{}
		m.resourceProcessExtraErr = ""
		m.resourceProcessExtraLoading = true
		return m, m.fetchProcessExtraDetail(m.resourceHostIndex, item.PID)
	case resourceDatabases:
		item, ok := m.selectedDatabase()
		if !ok {
			return m, nil
		}
		m.resourceDatabaseExtraName = item.Name
		m.resourceDatabaseExtra = databaseExtraDetail{}
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
		result, cleanup := actions.RemoteCommandContext(ctx, h, containerExtraDetailScript(name))
		cleanup()
		detail, errText := parseContainerExtraDetail(result.Output)
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		return resourceContainerDetailMsg{Index: index, Name: name, Detail: detail, Err: errText}
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
		result, cleanup := actions.RemoteCommandContext(ctx, h, serviceExtraDetailScript(name))
		cleanup()
		detail, errText := parseServiceExtraDetail(result.Output)
		if strings.TrimSpace(detail.Unit) != "" {
			errText = ""
		} else if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		if !meaningfulResourceDetailError(errText) {
			errText = ""
		}
		return resourceServiceDetailMsg{Index: index, Name: name, Detail: detail, Err: errText}
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
		result, cleanup := actions.RemoteCommandContext(ctx, h, processExtraDetailScript(pid))
		cleanup()
		detail, errText := parseProcessExtraDetail(result.Output)
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		return resourceProcessDetailMsg{Index: index, PID: pid, Detail: detail, Err: errText}
	}
}

func (m Model) fetchDatabaseExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	item, ok := m.managedResource(resourceDatabases, name)
	if !ok || strings.TrimSpace(item.DBEngine) == "" {
		return func() tea.Msg {
			return resourceDatabaseDetailMsg{Index: index, Name: name, Detail: databaseExtraDetail{}, Err: m.t("Database connection is not configured. Press e to configure it.", "未配置数据库连接。按 e 配置。")}
		}
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	db := databaseDetail{}
	for _, detail := range m.states[index].DatabaseDetails {
		if detail.Name == name {
			db = detail
			break
		}
	}
	script := databaseMetricScriptForDetail(item, db)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, script)
		cleanup()
		detail, errText := parseDatabaseExtraDetail(result.Output)
		if result.Err != nil && errText == "" {
			errText = m.resourceRemoteErrorText(result.Err)
		}
		detail.Engine = item.DBEngine
		detail.Host = item.DBHost
		detail.Port = item.DBPort
		detail.User = item.DBUser
		detail.Database = strings.TrimSpace(item.DBName)
		if detail.Database == "" {
			detail.Database = databaseDefaultName(item.DBEngine)
		}
		if databaseMissingCredentialHint(item, errText) {
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
		m.setDatabaseExtraCache(db.Name, databaseExtraDetail{}, "", true)
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

func (m *Model) setDatabaseExtraCache(name string, detail databaseExtraDetail, errText string, loading bool) {
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

func (m Model) startResourceAdd() (tea.Model, tea.Cmd) {
	kind := m.resourceKind
	if kind == resourceAll || kind == resourceContainers {
		kind = resourceServices
	}
	m.resourceAddKind = kind
	m.resourceAddName = ""
	m.resourceAddField = 0
	m.resourceAddCursor = 0
	m.resourceManagePane = 0
	m.resourceManageDiscoveredIndex = 0
	m.resourceManageFavoriteIndex = 0
	m.resourceManageSearch = false
	m.resourceManageQuery = ""
	m.resourceCommandForm = resourceCommandForm{Server: m.resourceServerKey(m.resourceHostIndex), Kind: kind}
	m.mode = modeResourceAdd
	m.status = m.t("Resource manager", "资源管理")
	return m, nil
}

func (m Model) updateResourceAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.resourceManageSearch {
		return m.updateResourceManageSearch(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "ctrl+i", "shift+tab":
		m.resourceManagePane = 1 - m.resourceManagePane
	case "down", "j":
		m.moveResourceManageSelection(1)
	case "up", "k":
		m.moveResourceManageSelection(-1)
	case "left", "h":
		m.cycleResourceAddKind(-1)
		m.resetResourceManageSelection()
	case "right", "l":
		m.cycleResourceAddKind(1)
		m.resetResourceManageSelection()
	case "g":
		m.cycleResourceAddKind(1)
		m.resetResourceManageSelection()
	case "r":
		refreshKind := m.resourceAddKind
		if refreshKind == resourceProcesses {
			refreshKind = resourcePorts
		}
		m.resourceLoading = true
		m.resourceLoadingKind = refreshKind
		m.resourceLoadingPending = resourceLoadPartCount(refreshKind)
		m.resourceManualRefresh = true
		m.status = m.t("Refreshing resources...", "正在刷新资源...")
		return m, m.fetchResourceDetails(m.resourceHostIndex, refreshKind)
	case "/":
		m.resourceManageSearch = true
		m.resourceManageQuery = ""
		m.resetResourceManageSelection()
	case "enter", "f":
		return m.toggleResourceManagerFavorite()
	case "n":
		return m.startResourceExternalDatabaseAdd()
	case "x":
		return m.startResourceManageRemoveConfirm()
	case "e":
		if m.resourceManagePane == 1 {
			return m.startResourceManageEdit()
		}
	}
	return m, nil
}

func (m Model) startResourceExternalDatabaseAdd() (tea.Model, tea.Cmd) {
	if m.resourceAddKind != resourceDatabases {
		m.status = m.t("Manual add is available for databases only.", "手动新增仅支持数据库。")
		return m, clearStatusAfter(2 * time.Second)
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	item := defaultManagedResource(server, config.ResourceKindDatabase, "")
	m.resourceAddName = ""
	m.resourceAddField = 0
	m.resourceAddCursor = 0
	m.resourceCommandForm = resourceCommandForm{
		Server:     server,
		Kind:       resourceDatabases,
		Name:       "",
		DBEngine:   item.DBEngine,
		DBHost:     item.DBHost,
		DBPort:     item.DBPort,
		DBUser:     item.DBUser,
		DBPassword: item.DBPassword,
		DBName:     item.DBName,
		DBInstance: item.DBInstance,
		DBNote:     item.DBNote,
	}
	m.mode = modeResourceAddEdit
	m.status = m.t("Add external database", "新增外部数据库")
	return m, nil
}

func (m Model) startResourceDatabaseDiscoveredAdd(ref resourceRef) (tea.Model, tea.Cmd) {
	if ref.Kind != resourceDatabases || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	items := m.states[m.resourceHostIndex].DatabaseDetails
	if ref.Index < 0 || ref.Index >= len(items) {
		return m, nil
	}
	db := items[ref.Index]
	item := defaultDatabaseManagedResource(m.resourceServerKey(m.resourceHostIndex), db)
	m.resourceAddName = ""
	m.resourceAddField = 0
	m.resourceAddCursor = 0
	m.resourceCommandForm = resourceCommandForm{
		Server:        item.Server,
		Kind:          resourceDatabases,
		Name:          item.Name,
		DBEngine:      item.DBEngine,
		DBHost:        item.DBHost,
		DBPort:        item.DBPort,
		DBUser:        item.DBUser,
		DBPassword:    item.DBPassword,
		DBName:        item.DBName,
		DBInstance:    item.DBInstance,
		DBNote:        item.DBNote,
		DBSource:      db.Source,
		DBStatus:      db.Status,
		DBRawStatus:   db.RawStatus,
		DBEndpoint:    db.Endpoint,
		DBContainer:   db.Container,
		DBImage:       db.Image,
		DBServiceUnit: db.ServiceUnit,
		DBProcess:     db.Process,
		DBPID:         db.PID,
	}
	m.mode = modeResourceAddEdit
	m.status = m.t("Configure database", "配置数据库")
	return m, nil
}

func (m Model) updateResourceAddEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.mode = modeResourceAdd
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "down":
		m.moveResourceAddField(1)
	case "shift+tab", "up":
		m.moveResourceAddField(-1)
	case "left":
		if m.resourceCommandForm.Kind == resourceDatabases && m.resourceAddField == 0 {
			m.cycleResourceCommandDatabaseEngine(-1)
			return m, nil
		}
		m.moveResourceAddCursor(-1)
	case "right":
		if m.resourceCommandForm.Kind == resourceDatabases && m.resourceAddField == 0 {
			m.cycleResourceCommandDatabaseEngine(1)
			return m, nil
		}
		m.moveResourceAddCursor(1)
	case "backspace":
		if m.resourceCommandForm.Kind == resourceDatabases && m.resourceAddField == 0 {
			return m, nil
		}
		m.resourceAddBackspace()
	case "enter":
		return m.saveResourceAdd()
	default:
		if len(msg.Runes) > 0 {
			if m.resourceCommandForm.Kind == resourceDatabases && m.resourceAddField == 0 {
				return m, nil
			}
			m.resourceAddAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) updateResourceManageSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.resourceManageSearch = false
		m.resourceManageQuery = ""
		m.resetResourceManageSelection()
	case "enter":
		m.resourceManageSearch = false
	case "backspace":
		r := []rune(m.resourceManageQuery)
		if len(r) > 0 {
			m.resourceManageQuery = string(r[:len(r)-1])
		}
		m.resetResourceManageSelection()
	default:
		if len(msg.Runes) > 0 {
			m.resourceManageQuery += string(msg.Runes)
			m.resetResourceManageSelection()
		}
	}
	return m, nil
}

func (m *Model) resetResourceManageSelection() {
	m.resourceManageDiscoveredIndex = 0
	m.resourceManageFavoriteIndex = 0
}

func (m *Model) moveResourceManageSelection(delta int) {
	if m.resourceManagePane == 1 {
		count := len(m.resourceManageFavorites())
		if count == 0 {
			m.resourceManageFavoriteIndex = 0
			return
		}
		m.resourceManageFavoriteIndex = clampInt(m.resourceManageFavoriteIndex+delta, 0, count-1)
		return
	}
	count := len(m.resourceManageDiscoveredRefs())
	if count == 0 {
		m.resourceManageDiscoveredIndex = 0
		return
	}
	m.resourceManageDiscoveredIndex = clampInt(m.resourceManageDiscoveredIndex+delta, 0, count-1)
}

func (m Model) toggleResourceManagerFavorite() (tea.Model, tea.Cmd) {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(m.resourceAddKind)
	if kind == "" {
		return m, nil
	}
	if m.resourceManagePane == 1 {
		return m.startResourceManageRemoveConfirm()
	}
	refs := m.resourceManageDiscoveredRefs()
	if len(refs) == 0 {
		return m, nil
	}
	ref := refs[clampInt(m.resourceManageDiscoveredIndex, 0, len(refs)-1)]
	if ref.Kind == resourceDatabases {
		return m.startResourceDatabaseDiscoveredAdd(ref)
	}
	name, ok := m.resourceNameForRef(ref)
	if !ok || strings.TrimSpace(name) == "" {
		return m, nil
	}
	if idx := findManagedResource(m.resourceFile.Items, server, kind, name); idx >= 0 {
		if m.resourceFile.Items[idx].Added {
			m.status = m.t("Resource already added: ", "资源已添加：") + name
			return m, clearStatusAfter(2 * time.Second)
		}
		m.resourceFile.Items[idx].Added = true
		if err := config.SaveResources(m.home, m.resourceFile); err != nil {
			m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
			return m, nil
		}
		m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
		m.applyManagedResources(m.resourceHostIndex)
		m.status = m.t("Added to resources: ", "已添加资源：") + name
		return m, clearStatusAfter(2 * time.Second)
	}
	item := defaultManagedResource(server, kind, name)
	item.Added = true
	m.resourceFile.Items = append(m.resourceFile.Items, item)
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	refs = m.resourceManageDiscoveredRefs()
	if m.resourceManageDiscoveredIndex >= len(refs) && m.resourceManageDiscoveredIndex > 0 {
		m.resourceManageDiscoveredIndex--
	}
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceManageRemoveConfirm() (tea.Model, tea.Cmd) {
	if m.resourceManagePane != 1 {
		return m, nil
	}
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		m.status = m.t("No added resource to remove.", "没有可移出的已添加资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := items[clampInt(m.resourceManageFavoriteIndex, 0, len(items)-1)]
	return m.removeManagedResource(item)
}

func (m Model) startSelectedResourceRemoveConfirm() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	item, ok := m.managedResource(ref.Kind, name)
	if !ok {
		m.status = m.t("This resource is not added.", "该资源未添加。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.confirm = confirmAction{
		Kind:     confirmRemoveResource,
		Title:    m.t("Remove Resource", "确认移出资源"),
		Lines:    m.resourceRemoveConfirmLines(item),
		Back:     modeResourceList,
		Resource: item,
	}
	m.mode = modeConfirmAction
	m.status = m.t("Confirm remove resource", "确认移出资源")
	return m, nil
}

func (m Model) resourceRemoveConfirmLines(item config.ManagedResource) []string {
	return []string{
		m.t("This added resource will be removed from sshm.", "将从 sshm 已添加资源中移出该资源。"),
		m.t("Resource: ", "资源：") + item.Name,
		m.t("Type: ", "类型：") + m.resourceKindName(resourceKindFromConfig(item.Kind)),
		m.t("Server: ", "服务器：") + item.Server,
		m.t("This does not delete or stop the system resource. Docker containers can be discovered again next time.", "这不会删除或停止系统资源。Docker 容器下次仍会被自动发现。"),
	}
}

func (m Model) removeManagedResource(item config.ManagedResource) (tea.Model, tea.Cmd) {
	real := findManagedResource(m.resourceFile.Items, item.Server, item.Kind, item.Name)
	if real >= 0 {
		m.resourceFile.Items = append(m.resourceFile.Items[:real], m.resourceFile.Items[real+1:]...)
	}
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.confirm = confirmAction{}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	m.status = m.t("Removed from resources: ", "已移出资源：") + item.Name
	if m.resourceManageFavoriteIndex >= len(m.resourceManageFavorites()) && m.resourceManageFavoriteIndex > 0 {
		m.resourceManageFavoriteIndex--
	}
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceManageEdit() (tea.Model, tea.Cmd) {
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		return m, nil
	}
	item := items[clampInt(m.resourceManageFavoriteIndex, 0, len(items)-1)]
	m.resourceCommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceAddKind,
		Name:           item.Name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
		DBEngine:       item.DBEngine,
		DBHost:         item.DBHost,
		DBPort:         item.DBPort,
		DBUser:         item.DBUser,
		DBPassword:     item.DBPassword,
		DBName:         item.DBName,
		DBInstance:     item.DBInstance,
		DBNote:         item.DBNote,
	}
	m.resourceCommandField = 0
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceCommandBackMode = modeResourceAdd
	m.mode = modeResourceCommandEdit
	m.status = m.t("Edit resource commands", "编辑资源命令")
	return m, nil
}

func (m *Model) moveResourceAddField(delta int) {
	m.resourceAddField = moveIndex(m.resourceAddField, resourceAddFieldCount(m.resourceAddKind), delta)
	m.resourceAddCursor = len([]rune(m.resourceAddFieldValue(m.resourceAddField)))
}

func resourceAddFieldCount(kind resourceKind) int {
	if kind == resourceDatabases {
		return resourceCommandFieldCount(kind)
	}
	return 1 + resourceCommandFieldCount(kind)
}

func (m *Model) cycleResourceAddKind(delta int) {
	kinds := []resourceKind{resourceContainers, resourceServices, resourceProcesses, resourcePorts, resourceDatabases}
	idx := 0
	for i, kind := range kinds {
		if kind == m.resourceAddKind {
			idx = i
			break
		}
	}
	idx = moveIndex(idx, len(kinds), delta)
	m.resourceAddKind = kinds[idx]
	m.resourceCommandForm.Kind = m.resourceAddKind
	m.applyResourceAddDefaults()
	m.resourceAddCursor = len([]rune(m.resourceAddFieldValue(m.resourceAddField)))
}

func (m *Model) applyResourceAddDefaults() {
	name := strings.TrimSpace(m.resourceAddName)
	if m.resourceAddKind == resourcePorts && name != "" && !strings.Contains(name, "/") {
		name = "tcp/" + name
	}
	item := defaultManagedResource(m.resourceServerKey(m.resourceHostIndex), configResourceKind(m.resourceAddKind), name)
	m.resourceCommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceAddKind,
		Name:           item.Name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
		DBEngine:       item.DBEngine,
		DBHost:         item.DBHost,
		DBPort:         item.DBPort,
		DBUser:         item.DBUser,
		DBPassword:     item.DBPassword,
		DBName:         item.DBName,
		DBInstance:     item.DBInstance,
		DBNote:         item.DBNote,
	}
}

func (m Model) resourceAddFieldValue(field int) string {
	if m.resourceAddKind == resourceDatabases {
		return m.resourceCommandFieldValue(field)
	}
	switch field {
	case 0:
		return m.resourceAddName
	default:
		return m.resourceCommandFieldValue(field - 1)
	}
}

func (m *Model) setResourceAddFieldValue(field int, value string) {
	if m.resourceAddKind == resourceDatabases {
		m.setResourceCommandFieldValue(field, value)
		return
	}
	if field == 0 {
		m.resourceAddName = value
		if m.resourceAddKind != resourceDatabases {
			m.applyResourceAddDefaults()
		}
		return
	}
	m.setResourceCommandFieldValue(field-1, value)
}

func (m *Model) moveResourceAddCursor(delta int) {
	if m.resourceCommandForm.Kind == resourceDatabases && m.resourceAddField == 0 {
		return
	}
	value := []rune(m.resourceAddFieldValue(m.resourceAddField))
	m.resourceAddCursor = clampInt(m.resourceAddCursor+delta, 0, len(value))
}

func (m *Model) resourceAddAppend(value string) {
	runes := []rune(m.resourceAddFieldValue(m.resourceAddField))
	cursor := clampInt(m.resourceAddCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceAddField, string(next))
	m.resourceAddCursor = cursor + len([]rune(value))
}

func (m *Model) resourceAddBackspace() {
	runes := []rune(m.resourceAddFieldValue(m.resourceAddField))
	cursor := clampInt(m.resourceAddCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceAddField, string(next))
	m.resourceAddCursor = cursor - 1
}

func (m Model) saveResourceAdd() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.resourceAddName)
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(m.resourceAddKind)
	if m.resourceAddKind == resourceDatabases {
		if strings.TrimSpace(m.resourceCommandForm.DBEngine) == "" {
			m.resourceCommandForm.DBEngine = "MySQL"
		}
		if strings.TrimSpace(m.resourceCommandForm.DBHost) == "" {
			m.resourceCommandForm.DBHost = "127.0.0.1"
		}
		if strings.TrimSpace(m.resourceCommandForm.DBPort) == "" {
			m.resourceCommandForm.DBPort = databaseDefaultPort(m.resourceCommandForm.DBEngine)
		}
		name = strings.TrimSpace(m.resourceCommandForm.DBName)
		if name == "" {
			m.status = m.t("Database name cannot be empty.", "库名不能为空。")
			return m, nil
		}
	} else {
		if name == "" {
			m.status = m.t("Resource name cannot be empty.", "资源名称不能为空。")
			return m, nil
		}
		if m.resourceAddKind == resourcePorts && !strings.Contains(name, "/") {
			name = "tcp/" + name
		}
	}
	if idx := findManagedResource(m.resourceFile.Items, server, kind, name); idx >= 0 {
		if m.resourceFile.Items[idx].Added && !(m.resourceAddKind == resourceDatabases && !managedDatabaseResourceConfigured(m.resourceFile.Items[idx])) {
			m.status = m.t("Resource already added: ", "资源已添加：") + name
			return m, nil
		}
		m.resourceFile.Items[idx].Added = true
		m.resourceFile.Items[idx].StartCommand = strings.TrimSpace(m.resourceCommandForm.StartCommand)
		m.resourceFile.Items[idx].StopCommand = strings.TrimSpace(m.resourceCommandForm.StopCommand)
		m.resourceFile.Items[idx].RestartCommand = strings.TrimSpace(m.resourceCommandForm.RestartCommand)
		m.resourceFile.Items[idx].DeleteCommand = ""
		m.resourceFile.Items[idx].LogCommand = strings.TrimSpace(m.resourceCommandForm.LogCommand)
		m.resourceFile.Items[idx].HealthCommand = strings.TrimSpace(m.resourceCommandForm.HealthCommand)
		m.resourceFile.Items[idx].DBEngine = strings.TrimSpace(m.resourceCommandForm.DBEngine)
		m.resourceFile.Items[idx].DBHost = strings.TrimSpace(m.resourceCommandForm.DBHost)
		m.resourceFile.Items[idx].DBPort = strings.TrimSpace(m.resourceCommandForm.DBPort)
		m.resourceFile.Items[idx].DBUser = strings.TrimSpace(m.resourceCommandForm.DBUser)
		m.resourceFile.Items[idx].DBPassword = strings.TrimSpace(m.resourceCommandForm.DBPassword)
		m.resourceFile.Items[idx].DBName = strings.TrimSpace(m.resourceCommandForm.DBName)
		m.resourceFile.Items[idx].DBInstance = strings.TrimSpace(m.resourceCommandForm.DBInstance)
		m.resourceFile.Items[idx].DBNote = strings.TrimSpace(m.resourceCommandForm.DBNote)
		if err := config.SaveResources(m.home, m.resourceFile); err != nil {
			m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
			return m, nil
		}
		m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
		m.applyManagedResources(m.resourceHostIndex)
		m.resourceScope = resourceScopeDiscovered
		m.resourceKind = m.resourceAddKind
		m.resourceIndex = 0
		m.resourceScroll = 0
		if m.mode == modeResourceAddEdit {
			m.mode = modeResourceAdd
			m.resourceManagePane = 1
		} else {
			m.mode = modeResourceList
		}
		m.status = m.t("Added to resources: ", "已添加资源：") + name
		return m, clearStatusAfter(2 * time.Second)
	}
	item := defaultManagedResource(server, kind, name)
	item.Added = true
	item.StartCommand = strings.TrimSpace(m.resourceCommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceCommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceCommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceCommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceCommandForm.HealthCommand)
	item.DBEngine = strings.TrimSpace(m.resourceCommandForm.DBEngine)
	item.DBHost = strings.TrimSpace(m.resourceCommandForm.DBHost)
	item.DBPort = strings.TrimSpace(m.resourceCommandForm.DBPort)
	item.DBUser = strings.TrimSpace(m.resourceCommandForm.DBUser)
	item.DBPassword = strings.TrimSpace(m.resourceCommandForm.DBPassword)
	item.DBName = strings.TrimSpace(m.resourceCommandForm.DBName)
	item.DBInstance = strings.TrimSpace(m.resourceCommandForm.DBInstance)
	item.DBNote = strings.TrimSpace(m.resourceCommandForm.DBNote)
	m.resourceFile.Items = append(m.resourceFile.Items, item)
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	m.resourceScope = resourceScopeDiscovered
	m.resourceKind = m.resourceAddKind
	m.resourceIndex = 0
	m.resourceScroll = 0
	if m.mode == modeResourceAddEdit {
		m.mode = modeResourceAdd
		m.resourceManagePane = 1
	} else {
		m.mode = modeResourceList
	}
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceCommandEdit() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("Ports are read-only. Add the port before editing health checks.", "端口为只读。请先添加端口，再编辑健康检查。")
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(ref.Kind)
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 {
		if ref.Kind == resourceDatabases {
			item := defaultManagedResource(server, kind, name)
			if db, ok := m.selectedDatabase(); ok {
				item = defaultDatabaseManagedResource(server, db)
			}
			item.Added = true
			m.resourceFile.Items = append(m.resourceFile.Items, item)
			if err := config.SaveResources(m.home, m.resourceFile); err != nil {
				m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
				return m, nil
			}
			m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
			m.applyManagedResources(m.resourceHostIndex)
			idx = findManagedResource(m.resourceFile.Items, server, kind, name)
		}
	}
	if idx < 0 {
		m.status = m.t("Add this resource before editing commands.", "请先添加该资源，再编辑命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := m.resourceFile.Items[idx]
	if ref.Kind == resourceDatabases {
		if db, ok := m.selectedDatabase(); ok {
			item = mergeDatabaseDiscoveredDefaults(item, defaultDatabaseManagedResource(server, db))
		}
	}
	m.resourceCommandForm = resourceCommandForm{
		Server:         server,
		Kind:           ref.Kind,
		Name:           name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
		DBEngine:       item.DBEngine,
		DBHost:         item.DBHost,
		DBPort:         item.DBPort,
		DBUser:         item.DBUser,
		DBPassword:     item.DBPassword,
		DBName:         item.DBName,
		DBInstance:     item.DBInstance,
		DBNote:         item.DBNote,
	}
	m.resourceCommandField = 0
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceCommandBackMode = modeResourceList
	m.mode = modeResourceCommandEdit
	m.status = m.t("Edit resource commands", "编辑资源命令")
	return m, nil
}

func (m Model) updateResourceCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.mode = m.resourceCommandReturnMode()
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "down":
		m.moveResourceCommandField(1)
	case "shift+tab", "up":
		m.moveResourceCommandField(-1)
	case "left":
		if m.resourceCommandForm.Kind == resourceDatabases && m.resourceCommandField == 0 {
			m.cycleResourceCommandDatabaseEngine(-1)
			return m, nil
		}
		m.moveResourceCommandCursor(-1)
	case "right":
		if m.resourceCommandForm.Kind == resourceDatabases && m.resourceCommandField == 0 {
			m.cycleResourceCommandDatabaseEngine(1)
			return m, nil
		}
		m.moveResourceCommandCursor(1)
	case "backspace":
		if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
			return m, nil
		}
		m.resourceCommandBackspace()
	case "enter":
		if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
			m.mode = m.resourceCommandReturnMode()
			m.status = ""
			return m, nil
		}
		if err := m.saveResourceCommandForm(); err != nil {
			m.status = m.t("Save failed: ", "保存失败：") + err.Error()
			return m, nil
		}
		m.mode = m.resourceCommandReturnMode()
		m.status = m.t("Resource commands saved.", "资源命令已保存。")
		return m, clearStatusAfter(2 * time.Second)
	default:
		if len(msg.Runes) > 0 {
			if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
				return m, nil
			}
			if m.resourceCommandForm.Kind == resourceDatabases && m.resourceCommandField == 0 {
				return m, nil
			}
			m.resourceCommandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) resourceCommandReturnMode() viewMode {
	if m.resourceCommandBackMode == modeResourceAdd {
		return modeResourceAdd
	}
	return modeResourceList
}

func (m *Model) moveResourceCommandField(delta int) {
	if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
		m.resourceCommandField = 0
		m.resourceCommandCursor = 0
		return
	}
	m.resourceCommandField = moveIndex(m.resourceCommandField, resourceCommandFieldCount(m.resourceCommandForm.Kind), delta)
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(m.resourceCommandField)))
}

func resourceCommandFieldCount(kind resourceKind) int {
	if kind == resourcePorts {
		return 1
	}
	if kind == resourceDatabases {
		return 7
	}
	return 4
}

func (m *Model) cycleResourceCommandDatabaseEngine(delta int) {
	oldEngine := strings.TrimSpace(m.resourceCommandForm.DBEngine)
	oldDefaultPort := databaseDefaultPort(oldEngine)
	oldDefaultUser := databaseDefaultUser(oldEngine)
	oldDefaultName := databaseDefaultName(oldEngine)
	engines := databaseEngineChoices()
	idx := 0
	for i, engine := range engines {
		if strings.EqualFold(engine, oldEngine) {
			idx = i
			break
		}
	}
	idx = moveIndex(idx, len(engines), delta)
	next := engines[idx]
	m.resourceCommandForm.DBEngine = next
	if strings.TrimSpace(m.resourceCommandForm.DBPort) == "" || strings.TrimSpace(m.resourceCommandForm.DBPort) == oldDefaultPort {
		m.resourceCommandForm.DBPort = databaseDefaultPort(next)
	}
	if strings.TrimSpace(m.resourceCommandForm.DBUser) == "" || strings.TrimSpace(m.resourceCommandForm.DBUser) == oldDefaultUser {
		m.resourceCommandForm.DBUser = databaseDefaultUser(next)
	}
	if strings.TrimSpace(m.resourceCommandForm.DBName) == "" || strings.TrimSpace(m.resourceCommandForm.DBName) == oldDefaultName {
		m.resourceCommandForm.DBName = databaseDefaultName(next)
	}
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(m.resourceCommandField)))
}

func (m *Model) moveResourceCommandCursor(delta int) {
	if m.resourceCommandForm.Kind == resourceDatabases && m.resourceCommandField == 0 {
		return
	}
	value := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	m.resourceCommandCursor = clampInt(m.resourceCommandCursor+delta, 0, len(value))
}

func (m Model) resourceCommandFieldValue(field int) string {
	switch field {
	case 0:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBEngine
		}
		if m.resourceCommandForm.Kind == resourcePorts {
			return m.resourceCommandForm.HealthCommand
		}
		return m.resourceCommandForm.StartCommand
	case 1:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBHost
		}
		return m.resourceCommandForm.StopCommand
	case 2:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBPort
		}
		return m.resourceCommandForm.RestartCommand
	case 3:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBUser
		}
		return m.resourceCommandForm.LogCommand
	case 4:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBPassword
		}
		return ""
	case 5:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBName
		}
		return ""
	case 6:
		if m.resourceCommandForm.Kind == resourceDatabases {
			return m.resourceCommandForm.DBNote
		}
		return ""
	default:
		return ""
	}
}

func (m *Model) setResourceCommandFieldValue(field int, value string) {
	switch field {
	case 0:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBEngine = normalizeDatabaseEngine(value)
			return
		}
		if m.resourceCommandForm.Kind == resourcePorts {
			m.resourceCommandForm.HealthCommand = value
		} else {
			m.resourceCommandForm.StartCommand = value
		}
	case 1:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBHost = value
			return
		}
		m.resourceCommandForm.StopCommand = value
	case 2:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBPort = value
			return
		}
		m.resourceCommandForm.RestartCommand = value
	case 3:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBUser = value
			return
		}
		m.resourceCommandForm.LogCommand = value
	case 4:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBPassword = value
		}
	case 5:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBName = value
		}
	case 6:
		if m.resourceCommandForm.Kind == resourceDatabases {
			m.resourceCommandForm.DBNote = value
		}
	}
}

func (m *Model) resourceCommandAppend(value string) {
	runes := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	cursor := clampInt(m.resourceCommandCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceCommandField, string(next))
	m.resourceCommandCursor = cursor + len([]rune(value))
}

func (m *Model) resourceCommandBackspace() {
	runes := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	cursor := clampInt(m.resourceCommandCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceCommandField, string(next))
	m.resourceCommandCursor = cursor - 1
}

func (m *Model) saveResourceCommandForm() error {
	server := m.resourceCommandForm.Server
	kind := configResourceKind(m.resourceCommandForm.Kind)
	name := m.resourceCommandForm.Name
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 {
		return fmt.Errorf("resource config not found")
	}
	item := m.resourceFile.Items[idx]
	item.StartCommand = strings.TrimSpace(m.resourceCommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceCommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceCommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceCommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceCommandForm.HealthCommand)
	item.DBEngine = strings.TrimSpace(m.resourceCommandForm.DBEngine)
	item.DBHost = strings.TrimSpace(m.resourceCommandForm.DBHost)
	item.DBPort = strings.TrimSpace(m.resourceCommandForm.DBPort)
	item.DBUser = strings.TrimSpace(m.resourceCommandForm.DBUser)
	item.DBPassword = strings.TrimSpace(m.resourceCommandForm.DBPassword)
	item.DBName = strings.TrimSpace(m.resourceCommandForm.DBName)
	item.DBInstance = strings.TrimSpace(m.resourceCommandForm.DBInstance)
	item.DBNote = strings.TrimSpace(m.resourceCommandForm.DBNote)
	if m.resourceCommandForm.Kind == resourceDatabases && item.DBName != "" {
		item.Name = item.DBName
	}
	m.resourceFile.Items[idx] = item
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		return err
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	return nil
}

func (m Model) updateResourceLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
	case "j", "down":
		m.resourceLogScroll = moveClampedInt(m.resourceLogScroll, 1, 0, m.resourceLogMaxScroll())
	case "k", "up":
		m.resourceLogScroll = moveClampedInt(m.resourceLogScroll, -1, 0, m.resourceLogMaxScroll())
	case "r":
		return m.openResourceLog()
	}
	m.resourceLogScroll = clampInt(m.resourceLogScroll, 0, m.resourceLogMaxScroll())
	return m, nil
}

func (m Model) updateResourceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "enter":
		m.mode = modeResourceOutput
		m.resourceActionRunning = true
		m.resourceActionOutput = ""
		m.resourceActionExitCode = 0
		return m, m.runResourceAction()
	}
	return m, nil
}

func (m Model) updateResourceOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "j", "down":
		m.resourceScroll = moveClampedInt(m.resourceScroll, 1, 0, m.resourceOutputMaxScroll())
	case "k", "up":
		m.resourceScroll = moveClampedInt(m.resourceScroll, -1, 0, m.resourceOutputMaxScroll())
	case "r":
		if !m.resourceActionRunning && m.resourceAction != resourceActionNone {
			m.mode = modeResourceOutput
			m.resourceActionRunning = true
			m.resourceActionOutput = ""
			m.resourceActionExitCode = 0
			return m, m.runResourceAction()
		}
	}
	return m, nil
}

func (m *Model) moveResourceSelection(delta int) {
	total := len(m.filteredResourceIndexes())
	if total == 0 {
		m.resourceIndex = 0
		return
	}
	m.resourceIndex = clampInt(m.resourceIndex+delta, 0, total-1)
}

func (m *Model) moveResourceDown() {
	if m.resourceView == resourceViewCards {
		m.moveResourceSelection(m.dashboardColumns())
		return
	}
	m.moveResourceSelection(1)
}

func (m *Model) moveResourceUp() {
	if m.resourceView == resourceViewCards {
		m.moveResourceSelection(-m.dashboardColumns())
		return
	}
	m.moveResourceSelection(-1)
}

func (m *Model) moveResourceLeft() {
	m.moveResourceSelection(-1)
}

func (m *Model) moveResourceRight() {
	m.moveResourceSelection(1)
}

func (m Model) startResourceAction(action resourceActionKind) (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if action == resourceActionDelete {
		m.status = m.t("Delete is disabled for resources.", "资源页已禁用删除操作。")
		return m, nil
	}
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource is read-only.", "该资源为只读信息。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.managedResourceCommand(ref.Kind, action, name)) == "" {
		m.status = m.t("Add this process and configure commands first.", "请先添加该进程并配置命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.resourceAction = action
	m.resourceActionResource = ref.Kind
	m.resourceActionName = name
	m.resourceScroll = 0
	m.mode = modeResourceConfirm
	return m, nil
}

func (m Model) runResourceAction() tea.Cmd {
	index := m.resourceHostIndex
	kind := m.resourceActionResource
	action := m.resourceAction
	name := m.resourceActionName
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return func() tea.Msg {
			return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: actions.CommandResult{Err: fmt.Errorf("invalid resource"), ExitCode: -1}}
		}
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		script := m.resourceActionScript(kind, action, name)
		result, cleanup := actions.RemoteCommandContext(ctx, h, script)
		cleanup()
		return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: result}
	}
}

func (m Model) openResourceLog() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource does not have managed logs.", "该资源没有托管日志。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.managedResourceLogCommand(ref.Kind, name)) == "" {
		m.status = m.t("Add this process and configure log command first.", "请先添加该进程并配置日志命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeResourceLog
	m.resourceLogName = name
	m.resourceLogKind = ref.Kind
	m.resourceLogOutput = m.t("Loading logs...", "正在读取日志...")
	m.resourceLogScroll = 0
	index := m.resourceHostIndex
	kind := m.resourceLogKind
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, m.resourceLogScript(kind, name, 200))
		cleanup()
		return resourceLogMsg{Index: index, Kind: kind, Name: name, Output: result.Output, Result: result}
	}
}

func (m Model) handleResourceLoad(msg resourceLoadMsg) (tea.Model, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.states) {
		return m, nil
	}
	now := time.Now()
	if msg.Kind == resourceServices {
		m.states[msg.Index].ServiceDetails = msg.Services
		m.states[msg.Index].ServiceError = msg.ServiceErr
		m.resourceServiceAt = now
	}
	if msg.Kind == resourceContainers {
		m.states[msg.Index].ContainerDetails = msg.Containers
		m.states[msg.Index].ContainerError = msg.ContainerErr
		m.resourceContainerAt = now
		if msg.ContainerErr == "" {
			_ = config.UpsertResourceContainerCache(m.home, m.resourceServerKey(msg.Index), containerDetailsToCache(msg.Containers), now)
		}
	}
	if msg.Kind == resourcePorts {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	if msg.Kind == resourceServices && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	if msg.Kind == resourceContainers && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	associatePortContainers(m.states[msg.Index].PortDetails, m.states[msg.Index].ContainerDetails)
	m.states[msg.Index].DatabaseDetails, m.states[msg.Index].DatabaseError = deriveDatabaseDetails(m.states[msg.Index].ServiceDetails, m.states[msg.Index].ContainerDetails, m.states[msg.Index].PortDetails)
	m.applyManagedResources(msg.Index)
	m.resourceCollectedAt = now
	if m.resourceLoadingPending > 0 {
		m.resourceLoadingPending--
	}
	if m.resourceLoadingPending <= 0 {
		m.resourceLoading = false
		if m.resourceManualRefresh {
			m.resourceRefreshStatus = fmt.Sprintf("%s%s", m.t("Manual refresh done: ", "手动刷新完成："), now.Format("15:04:05"))
		} else {
			m.resourceRefreshStatus = fmt.Sprintf("%s%s", m.t("Last refresh: ", "最后刷新："), now.Format("15:04:05"))
		}
		m.resourceManualRefresh = false
		m.status = m.resourceRefreshStatus
		return m, m.fetchDatabaseCardExtras(msg.Index)
	}
	m.status = m.t("Loading resources...", "正在读取资源...")
	return m, nil
}

func (m Model) handleResourceContainerDetail(msg resourceContainerDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Name != m.resourceContainerExtraName {
		return m, nil
	}
	m.resourceContainerExtraLoading = false
	m.resourceContainerExtra = msg.Detail
	m.resourceContainerExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceServiceDetail(msg resourceServiceDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Name != m.resourceServiceExtraName {
		return m, nil
	}
	m.resourceServiceExtraLoading = false
	m.resourceServiceExtra = msg.Detail
	m.resourceServiceExtraErr = msg.Err
	if strings.TrimSpace(msg.Err) == "" && strings.TrimSpace(msg.Detail.Unit) != "" {
		for i := range m.states[msg.Index].ServiceDetails {
			if m.states[msg.Index].ServiceDetails[i].Unit != msg.Detail.Unit {
				continue
			}
			managed := m.states[msg.Index].ServiceDetails[i].Managed
			favorite := m.states[msg.Index].ServiceDetails[i].Favorite
			missing := m.states[msg.Index].ServiceDetails[i].Missing
			m.states[msg.Index].ServiceDetails[i] = msg.Detail
			m.states[msg.Index].ServiceDetails[i].Managed = managed
			m.states[msg.Index].ServiceDetails[i].Favorite = favorite
			m.states[msg.Index].ServiceDetails[i].Missing = missing
			break
		}
	}
	return m, nil
}

func (m Model) handleResourceProcessDetail(msg resourceProcessDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.PID != m.resourceProcessExtraPID {
		return m, nil
	}
	m.resourceProcessExtraLoading = false
	m.resourceProcessExtra = msg.Detail
	m.resourceProcessExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceDatabaseDetail(msg resourceDatabaseDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex {
		return m, nil
	}
	m.setDatabaseExtraCache(msg.Name, msg.Detail, msg.Err, false)
	if msg.Name == m.resourceDatabaseExtraName {
		m.resourceDatabaseExtraLoading = false
		m.resourceDatabaseExtra = msg.Detail
		m.resourceDatabaseExtraErr = msg.Err
	}
	return m, nil
}

func (m Model) handleResourceLog(msg resourceLogMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Kind != m.resourceLogKind || msg.Name != m.resourceLogName {
		return m, nil
	}
	m.resourceLogOutput = strings.TrimRight(msg.Output, "\n")
	if m.resourceLogOutput == "" {
		m.resourceLogOutput = m.t("No log output.", "没有日志输出。")
	}
	if msg.Result.Err != nil {
		m.status = fmt.Sprintf("%s %d", m.t("Log command failed, exit", "日志命令失败，退出码"), msg.Result.ExitCode)
	}
	return m, nil
}

func (m Model) handleResourceAction(msg resourceActionMsg) (tea.Model, tea.Cmd) {
	m.resourceActionRunning = false
	m.resourceActionOutput = strings.TrimRight(msg.Result.Output, "\n")
	m.resourceActionExitCode = msg.Result.ExitCode
	if msg.Result.Err != nil {
		m.status = m.resourceActionErrorText(msg.Result)
		return m, nil
	}
	m.status = m.resourceActionNameText(msg.Action) + m.t(" complete.", "完成。")
	refreshKind := msg.Kind
	if refreshKind == resourcePorts {
		refreshKind = resourceAll
	}
	m.resourceLoading = true
	m.resourceLoadingKind = refreshKind
	m.resourceLoadingPending = resourceLoadPartCount(refreshKind)
	m.resourceManualRefresh = false
	return m, m.fetchResourceDetails(msg.Index, refreshKind)
}

func (m Model) toggleManagedResource() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(ref.Kind)
	if kind == "" {
		return m, nil
	}
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 || !m.resourceFile.Items[idx].Added {
		m.status = m.t("Add this resource first with a.", "请先按 a 添加该资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.resourceFile.Items[idx].Favorite = !m.resourceFile.Items[idx].Favorite
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	if idx >= 0 && m.resourceFile.Items[idx].Favorite {
		m.status = m.t("Added to favorites: ", "已收藏：") + name
	} else {
		m.status = m.t("Removed from favorites: ", "已取消收藏：") + name
	}
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) toggleResourcePinned() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(ref.Kind)
	if kind == "" {
		return m, nil
	}
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	pinnedNow := false
	if idx < 0 || !m.resourceFile.Items[idx].Added {
		m.status = m.t("Add this resource first with a.", "请先按 a 添加该资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	if m.resourceFile.Items[idx].Pinned {
		m.resourceFile.Items[idx].Pinned = false
		m.resourceFile.Items[idx].PinnedOrder = 0
	} else {
		m.resourceFile.Items[idx].Pinned = true
		m.resourceFile.Items[idx].PinnedOrder = nextResourcePinnedOrder(m.resourceFile.Items)
		pinnedNow = true
	}
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to update pin: ", "置顶更新失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	if pinnedNow {
		m.status = m.t("Pinned: ", "已置顶：") + name
	} else {
		m.status = m.t("Unpinned: ", "已取消置顶：") + name
	}
	return m, clearStatusAfter(2 * time.Second)
}

func nextResourcePinnedOrder(items []config.ManagedResource) int64 {
	var maxOrder int64
	for _, item := range items {
		if item.PinnedOrder > maxOrder {
			maxOrder = item.PinnedOrder
		}
	}
	return maxOrder + 1
}

func (m Model) hasManagedResources(index int) bool {
	server := m.resourceServerKey(index)
	for _, item := range m.resourceFile.Items {
		if item.Server == server && item.Added {
			return true
		}
	}
	return false
}

func (m Model) resourceServerKey(index int) string {
	if index < 0 || index >= len(m.states) {
		return ""
	}
	h := m.states[index].Host
	return config.ServerCommandKey(h.Category, h.Name)
}

func (m *Model) applyManagedResources(index int) {
	if index < 0 || index >= len(m.states) {
		return
	}
	server := m.resourceServerKey(index)
	managed := m.managedResourcesForServer(server)
	state := &m.states[index]
	state.DatabaseDetails = removeConfiguredDatabaseDetails(state.DatabaseDetails)
	for i := range state.ServiceDetails {
		state.ServiceDetails[i].Managed = false
		state.ServiceDetails[i].Favorite = false
		state.ServiceDetails[i].Missing = false
	}
	for i := range state.ContainerDetails {
		state.ContainerDetails[i].Managed = false
		state.ContainerDetails[i].Favorite = false
		state.ContainerDetails[i].Missing = false
	}
	for i := range state.PortDetails {
		state.PortDetails[i].Managed = false
		state.PortDetails[i].Favorite = false
		state.PortDetails[i].ProcessManaged = false
		state.PortDetails[i].ProcessFavorite = false
		state.PortDetails[i].Missing = false
	}
	for i := range state.DatabaseDetails {
		state.DatabaseDetails[i].Managed = false
		state.DatabaseDetails[i].Favorite = false
		state.DatabaseDetails[i].Missing = false
	}
	for _, item := range managed {
		if !item.Added {
			continue
		}
		switch item.Kind {
		case config.ResourceKindService:
			found := false
			for i := range state.ServiceDetails {
				if state.ServiceDetails[i].Unit == item.Name {
					state.ServiceDetails[i].Managed = true
					state.ServiceDetails[i].Favorite = item.Favorite
					if strings.EqualFold(strings.TrimSpace(state.ServiceDetails[i].Load), "not-found") {
						state.ServiceDetails[i].Missing = true
						state.ServiceDetails[i].Description = "Managed resource not found"
					}
					found = true
					break
				}
			}
			if !found {
				state.ServiceDetails = append(state.ServiceDetails, serviceDetail{Unit: item.Name, Load: "-", Active: "missing", Sub: "missing", Description: "Managed resource not found", Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindContainer:
			found := false
			for i := range state.ContainerDetails {
				if state.ContainerDetails[i].Name == item.Name {
					if containerResourceHasCustomConfig(item) {
						state.ContainerDetails[i].Managed = true
					}
					state.ContainerDetails[i].Favorite = item.Favorite
					found = true
					break
				}
			}
			if !found && containerResourceHasCustomConfig(item) {
				state.ContainerDetails = append(state.ContainerDetails, containerDetail{Name: item.Name, Status: "missing", Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindProcess:
			found := false
			for i := range state.PortDetails {
				if state.PortDetails[i].Process == item.Name {
					state.PortDetails[i].ProcessManaged = true
					state.PortDetails[i].ProcessFavorite = item.Favorite
					found = true
				}
			}
			if !found {
				state.PortDetails = append(state.PortDetails, portDetail{Process: item.Name, Count: 0, ProcessManaged: true, ProcessFavorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindPort:
			proto, port := splitManagedPortName(item.Name)
			found := false
			for i := range state.PortDetails {
				if state.PortDetails[i].Protocol == proto && state.PortDetails[i].Port == port {
					state.PortDetails[i].Managed = true
					state.PortDetails[i].Favorite = item.Favorite
					found = true
				}
			}
			if !found {
				state.PortDetails = append(state.PortDetails, portDetail{Protocol: proto, Port: port, Count: 0, Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindDatabase:
			if !managedDatabaseResourceConfigured(item) {
				continue
			}
			state.DatabaseDetails = append(state.DatabaseDetails, m.databaseDetailForManagedResource(item, state.DatabaseDetails))
		}
	}
}

func removeConfiguredDatabaseDetails(items []databaseDetail) []databaseDetail {
	out := items[:0]
	for _, item := range items {
		if item.Configured {
			continue
		}
		out = append(out, item)
	}
	return out
}

func managedDatabaseResourceConfigured(item config.ManagedResource) bool {
	if item.Kind != config.ResourceKindDatabase || !item.Added {
		return false
	}
	return strings.TrimSpace(item.DBEngine) != "" &&
		strings.TrimSpace(item.DBHost) != "" &&
		strings.TrimSpace(item.DBPort) != "" &&
		strings.TrimSpace(item.DBName) != ""
}

func (m Model) databaseDetailForManagedResource(item config.ManagedResource, discovered []databaseDetail) databaseDetail {
	detail := databaseDetail{
		Name:       firstNonEmpty(item.Name, item.DBName),
		Engine:     firstNonEmpty(item.DBEngine, "Database"),
		Source:     m.t("Configured", "配置"),
		Status:     "unknown",
		RawStatus:  m.t("Configured", "已配置"),
		Endpoint:   databaseManagedEndpoint(item),
		Protocol:   "tcp",
		Port:       strings.TrimSpace(item.DBPort),
		Managed:    true,
		Favorite:   item.Favorite,
		Configured: true,
	}
	if match, ok := managedDatabaseInstanceDetail(item, discovered); ok {
		detail.Source = match.Source
		detail.Status = match.Status
		detail.RawStatus = match.RawStatus
		detail.Endpoint = firstNonEmpty(databaseManagedEndpoint(item), match.Endpoint)
		detail.ServiceUnit = match.ServiceUnit
		detail.Container = match.Container
		detail.Image = match.Image
		detail.Process = match.Process
		detail.PID = match.PID
		detail.Protocol = firstNonEmpty(match.Protocol, detail.Protocol)
		detail.Port = firstNonEmpty(strings.TrimSpace(item.DBPort), match.Port)
	}
	return detail
}

func managedDatabaseInstanceDetail(item config.ManagedResource, discovered []databaseDetail) (databaseDetail, bool) {
	instance := strings.TrimSpace(item.DBInstance)
	for _, db := range discovered {
		if instance != "" && strings.EqualFold(db.Name, instance) {
			return db, true
		}
	}
	port := strings.TrimSpace(item.DBPort)
	for _, db := range discovered {
		if port != "" && strings.Contains(db.Endpoint, port) && strings.EqualFold(normalizeDatabaseEngine(db.Engine), normalizeDatabaseEngine(item.DBEngine)) {
			return db, true
		}
	}
	return databaseDetail{}, false
}

func (m Model) managedResourcesForServer(server string) []config.ManagedResource {
	items := []config.ManagedResource{}
	for _, item := range m.resourceFile.Items {
		if item.Server == server {
			items = append(items, item)
		}
	}
	return items
}

func findManagedResource(items []config.ManagedResource, server string, kind string, name string) int {
	for i, item := range items {
		if item.Server == server && item.Kind == kind && item.Name == name {
			return i
		}
	}
	return -1
}

func configResourceKind(kind resourceKind) string {
	switch kind {
	case resourceServices:
		return config.ResourceKindService
	case resourceContainers:
		return config.ResourceKindContainer
	case resourceProcesses:
		return config.ResourceKindProcess
	case resourcePorts:
		return config.ResourceKindPort
	case resourceDatabases:
		return config.ResourceKindDatabase
	default:
		return ""
	}
}

func resourceKindFromConfig(kind string) resourceKind {
	switch kind {
	case config.ResourceKindService:
		return resourceServices
	case config.ResourceKindContainer:
		return resourceContainers
	case config.ResourceKindProcess:
		return resourceProcesses
	case config.ResourceKindPort:
		return resourcePorts
	case config.ResourceKindDatabase:
		return resourceDatabases
	default:
		return resourceAll
	}
}

func defaultManagedResource(server string, kind string, name string) config.ManagedResource {
	item := config.ManagedResource{Server: server, Kind: kind, Name: name}
	switch kind {
	case config.ResourceKindService:
		target := shellQuoteLocal(name)
		item.StartCommand = "systemctl start " + target
		item.StopCommand = "systemctl stop " + target
		item.RestartCommand = "systemctl restart " + target
		item.LogCommand = "journalctl -u " + target + " -n 200 --no-pager"
	case config.ResourceKindContainer:
		target := shellQuoteLocal(name)
		item.StartCommand = "docker start " + target
		item.StopCommand = "docker stop " + target
		item.RestartCommand = "docker restart " + target
		item.LogCommand = "docker logs --tail 200 " + target
	case config.ResourceKindPort:
		_, port := splitManagedPortName(name)
		item.HealthCommand = "curl -f http://127.0.0.1:" + shellQuoteLocal(port) + "/health"
	case config.ResourceKindDatabase:
		item.DBEngine = "MySQL"
		item.DBHost = "127.0.0.1"
		item.DBPort = "3306"
		item.DBUser = "root"
	}
	return item
}

func defaultDatabaseManagedResource(server string, db databaseDetail) config.ManagedResource {
	item := config.ManagedResource{Server: server, Kind: config.ResourceKindDatabase, Added: true}
	item.DBEngine = firstNonEmpty(normalizeDatabaseEngine(db.Engine), "MySQL")
	item.DBHost = databaseDefaultHost(db)
	item.DBPort = databaseDefaultPortForDetail(db)
	item.DBUser = databaseDefaultUser(item.DBEngine)
	item.DBName = databaseDefaultName(item.DBEngine)
	item.Name = firstNonEmpty(item.DBName, db.Name)
	item.DBInstance = strings.TrimSpace(db.Name)
	return item
}

func mergeDatabaseDiscoveredDefaults(item config.ManagedResource, defaults config.ManagedResource) config.ManagedResource {
	if strings.TrimSpace(item.DBEngine) == "" || databaseConnectionLooksGenericDefault(item, defaults) {
		item.DBEngine = defaults.DBEngine
		item.DBHost = defaults.DBHost
		item.DBPort = defaults.DBPort
		item.DBUser = defaults.DBUser
		item.DBName = defaults.DBName
		item.DBInstance = defaults.DBInstance
		return item
	}
	if strings.TrimSpace(item.DBHost) == "" {
		item.DBHost = defaults.DBHost
	}
	if strings.TrimSpace(item.DBPort) == "" {
		item.DBPort = defaults.DBPort
	}
	if strings.TrimSpace(item.DBUser) == "" {
		item.DBUser = defaults.DBUser
	}
	if strings.TrimSpace(item.DBName) == "" {
		item.DBName = defaults.DBName
	}
	if strings.TrimSpace(item.DBInstance) == "" {
		item.DBInstance = defaults.DBInstance
	}
	return item
}

func databaseConnectionLooksGenericDefault(item config.ManagedResource, defaults config.ManagedResource) bool {
	if strings.EqualFold(strings.TrimSpace(item.DBEngine), strings.TrimSpace(defaults.DBEngine)) {
		return false
	}
	generic := defaultManagedResource(item.Server, config.ResourceKindDatabase, item.Name)
	return strings.EqualFold(strings.TrimSpace(item.DBEngine), strings.TrimSpace(generic.DBEngine)) &&
		strings.TrimSpace(item.DBHost) == strings.TrimSpace(generic.DBHost) &&
		strings.TrimSpace(item.DBPort) == strings.TrimSpace(generic.DBPort) &&
		strings.TrimSpace(item.DBUser) == strings.TrimSpace(generic.DBUser) &&
		strings.TrimSpace(item.DBName) == strings.TrimSpace(generic.DBName)
}

func databaseManagedEndpoint(item config.ManagedResource) string {
	host := strings.TrimSpace(item.DBHost)
	port := strings.TrimSpace(item.DBPort)
	if host == "" {
		return ""
	}
	if port == "" {
		return host
	}
	return host + ":" + port
}

func containerResourceHasCustomConfig(item config.ManagedResource) bool {
	if item.Kind != config.ResourceKindContainer {
		return false
	}
	defaults := defaultManagedResource(item.Server, item.Kind, item.Name)
	return strings.TrimSpace(item.StartCommand) != "" && strings.TrimSpace(item.StartCommand) != defaults.StartCommand ||
		strings.TrimSpace(item.StopCommand) != "" && strings.TrimSpace(item.StopCommand) != defaults.StopCommand ||
		strings.TrimSpace(item.RestartCommand) != "" && strings.TrimSpace(item.RestartCommand) != defaults.RestartCommand ||
		strings.TrimSpace(item.LogCommand) != "" && strings.TrimSpace(item.LogCommand) != defaults.LogCommand ||
		strings.TrimSpace(item.HealthCommand) != "" ||
		strings.TrimSpace(item.DeleteCommand) != ""
}

func containerDetailsToCache(items []containerDetail) []config.ResourceContainerCache {
	out := make([]config.ResourceContainerCache, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || item.Missing {
			continue
		}
		out = append(out, config.ResourceContainerCache{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func containerDetailsFromCache(items []config.ResourceContainerCache) []containerDetail {
	out := make([]containerDetail, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		out = append(out, containerDetail{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func splitManagedPortName(name string) (string, string) {
	name = strings.TrimSpace(name)
	proto, port, ok := strings.Cut(name, "/")
	if !ok {
		return "tcp", name
	}
	proto = strings.TrimSpace(proto)
	port = strings.TrimSpace(port)
	if proto == "" {
		proto = "tcp"
	}
	return proto, port
}

func resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	if action == resourceActionDelete {
		return ""
	}
	if kind == resourceProcesses || kind == resourcePorts || kind == resourceDatabases {
		return ""
	}
	target := shellQuoteLocal(name)
	if kind == resourceServices {
		command := "restart"
		switch action {
		case resourceActionStart:
			command = "start"
		case resourceActionStop:
			command = "stop"
		case resourceActionRestart:
			command = "restart"
		}
		return sudoFallbackScript("systemctl "+command+" "+target, "sudo -n systemctl "+command+" "+target)
	}
	command := "restart"
	switch action {
	case resourceActionStart:
		command = "start"
	case resourceActionStop:
		command = "stop"
	case resourceActionRestart:
		command = "restart"
	}
	return sudoFallbackScript("docker "+command+" "+target, "sudo -n docker "+command+" "+target)
}

func (m Model) resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	if cmd := m.managedResourceCommand(kind, action, name); cmd != "" {
		return sudoFallbackScript(cmd, "sudo -n "+cmd)
	}
	return resourceActionScript(kind, action, name)
}

func resourceLogScript(kind resourceKind, name string, lines int) string {
	if lines <= 0 {
		lines = 200
	}
	target := shellQuoteLocal(name)
	if kind == resourceServices {
		cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager", target, lines)
		sudoCmd := fmt.Sprintf("sudo -n journalctl -u %s -n %d --no-pager", target, lines)
		return sudoFallbackScript(cmd, sudoCmd)
	}
	if kind == resourceProcesses || kind == resourcePorts || kind == resourceDatabases {
		return ""
	}
	cmd := fmt.Sprintf("docker logs --tail %d %s", lines, target)
	sudoCmd := fmt.Sprintf("sudo -n docker logs --tail %d %s", lines, target)
	return sudoFallbackScript(cmd, sudoCmd)
}

func (m Model) resourceLogScript(kind resourceKind, name string, lines int) string {
	if cmd := m.managedResourceLogCommand(kind, name); cmd != "" {
		return sudoFallbackScript(cmd, "sudo -n "+cmd)
	}
	return resourceLogScript(kind, name, lines)
}

func (m Model) managedResourceCommand(kind resourceKind, action resourceActionKind, name string) string {
	item, ok := m.managedResource(kind, name)
	if !ok {
		return ""
	}
	switch action {
	case resourceActionStart:
		return item.StartCommand
	case resourceActionStop:
		return item.StopCommand
	case resourceActionRestart:
		return item.RestartCommand
	case resourceActionDelete:
		return ""
	default:
		return ""
	}
}

func (m Model) managedResourceLogCommand(kind resourceKind, name string) string {
	item, ok := m.managedResource(kind, name)
	if !ok {
		return ""
	}
	return item.LogCommand
}

func (m Model) managedResource(kind resourceKind, name string) (config.ManagedResource, bool) {
	server := m.resourceServerKey(m.resourceHostIndex)
	configKind := configResourceKind(kind)
	if server == "" || configKind == "" {
		return config.ManagedResource{}, false
	}
	for _, item := range m.resourceFile.Items {
		if item.Server == server && item.Kind == configKind && item.Name == name && item.Added {
			if item.Kind == config.ResourceKindDatabase && !managedDatabaseResourceConfigured(item) {
				continue
			}
			return item, true
		}
	}
	return config.ManagedResource{}, false
}

func sudoFallbackScript(command string, sudoCommand string) string {
	return fmt.Sprintf(`out=$(%s 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  first="$out"
  out=$(%s 2>&1)
  code=$?
  if [ "$code" -ne 0 ]; then
    case "$first $out" in
      *"permission denied"*|*"Permission denied"*|*"not in the docker group"*|*"password is required"*|*"a password is required"*|*"Authentication is required"*) echo "__SSHM_PERMISSION_DENIED__" ;;
    esac
  fi
fi
printf '%%s\n' "$out"
exit "$code"`, command, sudoCommand)
}

func shellQuoteLocal(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '/' || r == '.' || r == ':' || r == '=' || r == ',' || r == '@' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
