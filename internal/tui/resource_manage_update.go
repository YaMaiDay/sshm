package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

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
	m.status = m.t("Add database manually", "手动新增数据库")
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
		if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
