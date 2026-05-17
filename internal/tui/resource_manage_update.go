package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) startResourceAdd() (tea.Model, tea.Cmd) {
	kind := m.resourceState.Kind
	if kind == resourceAll || kind == resourceContainers {
		kind = resourceServices
	}
	m.resourceState.AddKind = kind
	m.resourceState.AddName = ""
	m.resourceState.AddField = 0
	m.resourceState.AddCursor = 0
	m.resourceState.ManagePane = 0
	m.resourceState.ManageDiscoveredIndex = 0
	m.resourceState.ManageFavoriteIndex = 0
	m.resourceState.ManageSearch = false
	m.resourceState.ManageQuery = ""
	m.resourceState.CommandForm = resourceCommandForm{Server: m.resourceServerKey(m.resourceState.HostIndex), Kind: kind}
	m.mode = modeResourceAdd
	m.status = m.t("Resource manager", "资源管理")
	return m, nil
}

func (m Model) updateResourceAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.resourceState.ManageSearch {
		return m.updateResourceManageSearch(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "ctrl+i", "shift+tab":
		m.resourceState.ManagePane = 1 - m.resourceState.ManagePane
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
		refreshKind := m.resourceState.AddKind
		if refreshKind == resourceProcesses {
			refreshKind = resourcePorts
		}
		m.resourceState.Loading = true
		m.resourceState.LoadingKind = refreshKind
		m.resourceState.LoadingPending = resourceLoadPartCount(refreshKind)
		m.resourceState.ManualRefresh = true
		m.status = m.t("Refreshing resources...", "正在刷新资源...")
		return m, m.fetchResourceDetails(m.resourceState.HostIndex, refreshKind)
	case "/":
		m.resourceState.ManageSearch = true
		m.resourceState.ManageQuery = ""
		m.resetResourceManageSelection()
	case "enter", "f":
		return m.toggleResourceManagerFavorite()
	case "n":
		return m.startResourceExternalDatabaseAdd()
	case "x":
		return m.startResourceManageRemoveConfirm()
	case "e":
		if m.resourceState.ManagePane == 1 {
			return m.startResourceManageEdit()
		}
	}
	return m, nil
}

func (m Model) startResourceExternalDatabaseAdd() (tea.Model, tea.Cmd) {
	if m.resourceState.AddKind != resourceDatabases {
		m.status = m.t("Manual add is available for databases only.", "手动新增仅支持数据库。")
		return m, clearStatusAfter(2 * time.Second)
	}
	server := m.resourceServerKey(m.resourceState.HostIndex)
	item := defaultManagedResource(server, config.ResourceKindDatabase, "")
	m.resourceState.AddName = ""
	m.resourceState.AddField = 0
	m.resourceState.AddCursor = 0
	m.resourceState.CommandForm = resourceCommandForm{
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
	if ref.Kind != resourceDatabases || m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return m, nil
	}
	items := m.states[m.resourceState.HostIndex].DatabaseDetails
	if ref.Index < 0 || ref.Index >= len(items) {
		return m, nil
	}
	db := items[ref.Index]
	item := defaultDatabaseManagedResource(m.resourceServerKey(m.resourceState.HostIndex), db)
	m.resourceState.AddName = ""
	m.resourceState.AddField = 0
	m.resourceState.AddCursor = 0
	m.resourceState.CommandForm = resourceCommandForm{
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
		if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.AddField == 0 {
			m.cycleResourceCommandDatabaseEngine(-1)
			return m, nil
		}
		m.moveResourceAddCursor(-1)
	case "right":
		if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.AddField == 0 {
			m.cycleResourceCommandDatabaseEngine(1)
			return m, nil
		}
		m.moveResourceAddCursor(1)
	case "backspace":
		if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.AddField == 0 {
			return m, nil
		}
		m.resourceAddBackspace()
	case "enter":
		return m.saveResourceAdd()
	default:
		if len(msg.Runes) > 0 {
			if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.AddField == 0 {
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
		m.resourceState.ManageSearch = false
		m.resourceState.ManageQuery = ""
		m.resetResourceManageSelection()
	case "enter":
		m.resourceState.ManageSearch = false
	case "backspace":
		r := []rune(m.resourceState.ManageQuery)
		if len(r) > 0 {
			m.resourceState.ManageQuery = string(r[:len(r)-1])
		}
		m.resetResourceManageSelection()
	default:
		if len(msg.Runes) > 0 {
			m.resourceState.ManageQuery += string(msg.Runes)
			m.resetResourceManageSelection()
		}
	}
	return m, nil
}

func (m *Model) resetResourceManageSelection() {
	m.resourceState.ManageDiscoveredIndex = 0
	m.resourceState.ManageFavoriteIndex = 0
}

func (m *Model) moveResourceManageSelection(delta int) {
	if m.resourceState.ManagePane == 1 {
		count := len(m.resourceManageFavorites())
		if count == 0 {
			m.resourceState.ManageFavoriteIndex = 0
			return
		}
		m.resourceState.ManageFavoriteIndex = clampInt(m.resourceState.ManageFavoriteIndex+delta, 0, count-1)
		return
	}
	count := len(m.resourceManageDiscoveredRefs())
	if count == 0 {
		m.resourceState.ManageDiscoveredIndex = 0
		return
	}
	m.resourceState.ManageDiscoveredIndex = clampInt(m.resourceState.ManageDiscoveredIndex+delta, 0, count-1)
}

func (m Model) toggleResourceManagerFavorite() (tea.Model, tea.Cmd) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return m, nil
	}
	server := m.resourceServerKey(m.resourceState.HostIndex)
	kind := configResourceKind(m.resourceState.AddKind)
	if kind == "" {
		return m, nil
	}
	if m.resourceState.ManagePane == 1 {
		return m.startResourceManageRemoveConfirm()
	}
	refs := m.resourceManageDiscoveredRefs()
	if len(refs) == 0 {
		return m, nil
	}
	ref := refs[clampInt(m.resourceState.ManageDiscoveredIndex, 0, len(refs)-1)]
	if ref.Kind == resourceDatabases {
		return m.startResourceDatabaseDiscoveredAdd(ref)
	}
	name, ok := m.resourceNameForRef(ref)
	if !ok || strings.TrimSpace(name) == "" {
		return m, nil
	}
	if idx := findManagedResource(m.resourceState.File.Items, server, kind, name); idx >= 0 {
		if m.resourceState.File.Items[idx].Added {
			m.status = m.t("Resource already added: ", "资源已添加：") + name
			return m, clearStatusAfter(2 * time.Second)
		}
		m.resourceState.File.Items[idx].Added = true
		if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
			m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
			return m, nil
		}
		m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
		m.applyManagedResources(m.resourceState.HostIndex)
		m.status = m.t("Added to resources: ", "已添加资源：") + name
		return m, clearStatusAfter(2 * time.Second)
	}
	item := defaultManagedResource(server, kind, name)
	item.Added = true
	m.resourceState.File.Items = append(m.resourceState.File.Items, item)
	if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
	m.applyManagedResources(m.resourceState.HostIndex)
	refs = m.resourceManageDiscoveredRefs()
	if m.resourceState.ManageDiscoveredIndex >= len(refs) && m.resourceState.ManageDiscoveredIndex > 0 {
		m.resourceState.ManageDiscoveredIndex--
	}
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceManageRemoveConfirm() (tea.Model, tea.Cmd) {
	if m.resourceState.ManagePane != 1 {
		return m, nil
	}
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		m.status = m.t("No added resource to remove.", "没有可移出的已添加资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := items[clampInt(m.resourceState.ManageFavoriteIndex, 0, len(items)-1)]
	return m.removeManagedResource(item)
}

func (m Model) startSelectedResourceRemoveConfirm() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
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
	real := findManagedResource(m.resourceState.File.Items, item.Server, item.Kind, item.Name)
	if real >= 0 {
		m.resourceState.File.Items = append(m.resourceState.File.Items[:real], m.resourceState.File.Items[real+1:]...)
	}
	if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.confirm = confirmAction{}
	m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
	m.applyManagedResources(m.resourceState.HostIndex)
	m.status = m.t("Removed from resources: ", "已移出资源：") + item.Name
	if m.resourceState.ManageFavoriteIndex >= len(m.resourceManageFavorites()) && m.resourceState.ManageFavoriteIndex > 0 {
		m.resourceState.ManageFavoriteIndex--
	}
	return m, clearStatusAfter(2 * time.Second)
}
