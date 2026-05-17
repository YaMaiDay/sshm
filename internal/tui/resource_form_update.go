package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) startResourceManageEdit() (tea.Model, tea.Cmd) {
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		return m, nil
	}
	item := items[clampInt(m.resourceState.ManageFavoriteIndex, 0, len(items)-1)]
	m.resourceState.CommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceState.AddKind,
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
	m.resourceState.CommandField = 0
	m.resourceState.CommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceState.CommandBackMode = modeResourceAdd
	m.mode = modeResourceCommandEdit
	m.status = m.t("Edit resource commands", "编辑资源命令")
	return m, nil
}

func (m *Model) moveResourceAddField(delta int) {
	m.resourceState.AddField = moveIndex(m.resourceState.AddField, resourceAddFieldCount(m.resourceState.AddKind), delta)
	m.resourceState.AddCursor = len([]rune(m.resourceAddFieldValue(m.resourceState.AddField)))
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
		if kind == m.resourceState.AddKind {
			idx = i
			break
		}
	}
	idx = moveIndex(idx, len(kinds), delta)
	m.resourceState.AddKind = kinds[idx]
	m.resourceState.CommandForm.Kind = m.resourceState.AddKind
	m.applyResourceAddDefaults()
	m.resourceState.AddCursor = len([]rune(m.resourceAddFieldValue(m.resourceState.AddField)))
}

func (m *Model) applyResourceAddDefaults() {
	name := strings.TrimSpace(m.resourceState.AddName)
	if m.resourceState.AddKind == resourcePorts && name != "" && !strings.Contains(name, "/") {
		name = "tcp/" + name
	}
	item := defaultManagedResource(m.resourceServerKey(m.resourceState.HostIndex), configResourceKind(m.resourceState.AddKind), name)
	m.resourceState.CommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceState.AddKind,
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
	if m.resourceState.AddKind == resourceDatabases {
		return m.resourceCommandFieldValue(field)
	}
	switch field {
	case 0:
		return m.resourceState.AddName
	default:
		return m.resourceCommandFieldValue(field - 1)
	}
}

func (m *Model) setResourceAddFieldValue(field int, value string) {
	if m.resourceState.AddKind == resourceDatabases {
		m.setResourceCommandFieldValue(field, value)
		return
	}
	if field == 0 {
		m.resourceState.AddName = value
		if m.resourceState.AddKind != resourceDatabases {
			m.applyResourceAddDefaults()
		}
		return
	}
	m.setResourceCommandFieldValue(field-1, value)
}

func (m *Model) moveResourceAddCursor(delta int) {
	if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.AddField == 0 {
		return
	}
	value := []rune(m.resourceAddFieldValue(m.resourceState.AddField))
	m.resourceState.AddCursor = clampInt(m.resourceState.AddCursor+delta, 0, len(value))
}

func (m *Model) resourceAddAppend(value string) {
	runes := []rune(m.resourceAddFieldValue(m.resourceState.AddField))
	cursor := clampInt(m.resourceState.AddCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceState.AddField, string(next))
	m.resourceState.AddCursor = cursor + len([]rune(value))
}

func (m *Model) resourceAddBackspace() {
	runes := []rune(m.resourceAddFieldValue(m.resourceState.AddField))
	cursor := clampInt(m.resourceState.AddCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceState.AddField, string(next))
	m.resourceState.AddCursor = cursor - 1
}

func (m Model) saveResourceAdd() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.resourceState.AddName)
	server := m.resourceServerKey(m.resourceState.HostIndex)
	kind := configResourceKind(m.resourceState.AddKind)
	if m.resourceState.AddKind == resourceDatabases {
		if strings.TrimSpace(m.resourceState.CommandForm.DBEngine) == "" {
			m.resourceState.CommandForm.DBEngine = "MySQL"
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBHost) == "" {
			m.resourceState.CommandForm.DBHost = "127.0.0.1"
		}
		if strings.TrimSpace(m.resourceState.CommandForm.DBPort) == "" {
			m.resourceState.CommandForm.DBPort = resourceservice.DatabaseDefaultPort(m.resourceState.CommandForm.DBEngine)
		}
		name = strings.TrimSpace(m.resourceState.CommandForm.DBName)
		if name == "" {
			m.status = m.t("Database name cannot be empty.", "库名不能为空。")
			return m, nil
		}
	} else {
		if name == "" {
			m.status = m.t("Resource name cannot be empty.", "资源名称不能为空。")
			return m, nil
		}
		if m.resourceState.AddKind == resourcePorts && !strings.Contains(name, "/") {
			name = "tcp/" + name
		}
	}
	if idx := findManagedResource(m.resourceState.File.Items, server, kind, name); idx >= 0 {
		if m.resourceState.File.Items[idx].Added && !(m.resourceState.AddKind == resourceDatabases && !managedDatabaseResourceConfigured(m.resourceState.File.Items[idx])) {
			m.status = m.t("Resource already added: ", "资源已添加：") + name
			return m, nil
		}
		m.resourceState.File.Items[idx].Added = true
		m.resourceState.File.Items[idx].StartCommand = strings.TrimSpace(m.resourceState.CommandForm.StartCommand)
		m.resourceState.File.Items[idx].StopCommand = strings.TrimSpace(m.resourceState.CommandForm.StopCommand)
		m.resourceState.File.Items[idx].RestartCommand = strings.TrimSpace(m.resourceState.CommandForm.RestartCommand)
		m.resourceState.File.Items[idx].DeleteCommand = ""
		m.resourceState.File.Items[idx].LogCommand = strings.TrimSpace(m.resourceState.CommandForm.LogCommand)
		m.resourceState.File.Items[idx].HealthCommand = strings.TrimSpace(m.resourceState.CommandForm.HealthCommand)
		m.resourceState.File.Items[idx].DBEngine = strings.TrimSpace(m.resourceState.CommandForm.DBEngine)
		m.resourceState.File.Items[idx].DBHost = strings.TrimSpace(m.resourceState.CommandForm.DBHost)
		m.resourceState.File.Items[idx].DBPort = strings.TrimSpace(m.resourceState.CommandForm.DBPort)
		m.resourceState.File.Items[idx].DBUser = strings.TrimSpace(m.resourceState.CommandForm.DBUser)
		m.resourceState.File.Items[idx].DBPassword = strings.TrimSpace(m.resourceState.CommandForm.DBPassword)
		m.resourceState.File.Items[idx].DBName = strings.TrimSpace(m.resourceState.CommandForm.DBName)
		m.resourceState.File.Items[idx].DBInstance = strings.TrimSpace(m.resourceState.CommandForm.DBInstance)
		m.resourceState.File.Items[idx].DBNote = strings.TrimSpace(m.resourceState.CommandForm.DBNote)
		if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
			m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
			return m, nil
		}
		m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
		m.applyManagedResources(m.resourceState.HostIndex)
		m.resourceState.Scope = resourceScopeDiscovered
		m.resourceState.Kind = m.resourceState.AddKind
		m.resourceState.Index = 0
		m.resourceState.Scroll = 0
		if m.mode == modeResourceAddEdit {
			m.mode = modeResourceAdd
			m.resourceState.ManagePane = 1
		} else {
			m.mode = modeResourceList
		}
		m.status = m.t("Added to resources: ", "已添加资源：") + name
		return m, clearStatusAfter(2 * time.Second)
	}
	item := defaultManagedResource(server, kind, name)
	item.Added = true
	item.StartCommand = strings.TrimSpace(m.resourceState.CommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceState.CommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceState.CommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceState.CommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceState.CommandForm.HealthCommand)
	item.DBEngine = strings.TrimSpace(m.resourceState.CommandForm.DBEngine)
	item.DBHost = strings.TrimSpace(m.resourceState.CommandForm.DBHost)
	item.DBPort = strings.TrimSpace(m.resourceState.CommandForm.DBPort)
	item.DBUser = strings.TrimSpace(m.resourceState.CommandForm.DBUser)
	item.DBPassword = strings.TrimSpace(m.resourceState.CommandForm.DBPassword)
	item.DBName = strings.TrimSpace(m.resourceState.CommandForm.DBName)
	item.DBInstance = strings.TrimSpace(m.resourceState.CommandForm.DBInstance)
	item.DBNote = strings.TrimSpace(m.resourceState.CommandForm.DBNote)
	m.resourceState.File.Items = append(m.resourceState.File.Items, item)
	if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
	m.applyManagedResources(m.resourceState.HostIndex)
	m.resourceState.Scope = resourceScopeDiscovered
	m.resourceState.Kind = m.resourceState.AddKind
	m.resourceState.Index = 0
	m.resourceState.Scroll = 0
	if m.mode == modeResourceAddEdit {
		m.mode = modeResourceAdd
		m.resourceState.ManagePane = 1
	} else {
		m.mode = modeResourceList
	}
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceCommandEdit() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("Ports are read-only. Add the port before editing health checks.", "端口为只读。请先添加端口，再编辑健康检查。")
	}
	server := m.resourceServerKey(m.resourceState.HostIndex)
	kind := configResourceKind(ref.Kind)
	idx := findManagedResource(m.resourceState.File.Items, server, kind, name)
	if idx < 0 {
		if ref.Kind == resourceDatabases {
			item := defaultManagedResource(server, kind, name)
			if db, ok := m.selectedDatabase(); ok {
				item = defaultDatabaseManagedResource(server, db)
			}
			item.Added = true
			m.resourceState.File.Items = append(m.resourceState.File.Items, item)
			if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
				m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
				return m, nil
			}
			m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
			m.applyManagedResources(m.resourceState.HostIndex)
			idx = findManagedResource(m.resourceState.File.Items, server, kind, name)
		}
	}
	if idx < 0 {
		m.status = m.t("Add this resource before editing commands.", "请先添加该资源，再编辑命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := m.resourceState.File.Items[idx]
	if ref.Kind == resourceDatabases {
		if db, ok := m.selectedDatabase(); ok {
			item = mergeDatabaseDiscoveredDefaults(item, defaultDatabaseManagedResource(server, db))
		}
	}
	m.resourceState.CommandForm = resourceCommandForm{
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
	m.resourceState.CommandField = 0
	m.resourceState.CommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceState.CommandBackMode = modeResourceList
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
		if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.CommandField == 0 {
			m.cycleResourceCommandDatabaseEngine(-1)
			return m, nil
		}
		m.moveResourceCommandCursor(-1)
	case "right":
		if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.CommandField == 0 {
			m.cycleResourceCommandDatabaseEngine(1)
			return m, nil
		}
		m.moveResourceCommandCursor(1)
	case "backspace":
		if resourceCommandFieldCount(m.resourceState.CommandForm.Kind) == 0 {
			return m, nil
		}
		m.resourceCommandBackspace()
	case "enter":
		if resourceCommandFieldCount(m.resourceState.CommandForm.Kind) == 0 {
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
			if resourceCommandFieldCount(m.resourceState.CommandForm.Kind) == 0 {
				return m, nil
			}
			if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.CommandField == 0 {
				return m, nil
			}
			m.resourceCommandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) resourceCommandReturnMode() viewMode {
	if m.resourceState.CommandBackMode == modeResourceAdd {
		return modeResourceAdd
	}
	return modeResourceList
}

func (m *Model) moveResourceCommandField(delta int) {
	if resourceCommandFieldCount(m.resourceState.CommandForm.Kind) == 0 {
		m.resourceState.CommandField = 0
		m.resourceState.CommandCursor = 0
		return
	}
	m.resourceState.CommandField = moveIndex(m.resourceState.CommandField, resourceCommandFieldCount(m.resourceState.CommandForm.Kind), delta)
	m.resourceState.CommandCursor = len([]rune(m.resourceCommandFieldValue(m.resourceState.CommandField)))
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
	oldEngine := strings.TrimSpace(m.resourceState.CommandForm.DBEngine)
	oldDefaultPort := resourceservice.DatabaseDefaultPort(oldEngine)
	oldDefaultUser := resourceservice.DatabaseDefaultUser(oldEngine)
	oldDefaultName := resourceservice.DatabaseDefaultName(oldEngine)
	engines := resourceservice.DatabaseEngineChoices()
	idx := 0
	for i, engine := range engines {
		if strings.EqualFold(engine, oldEngine) {
			idx = i
			break
		}
	}
	idx = moveIndex(idx, len(engines), delta)
	next := engines[idx]
	m.resourceState.CommandForm.DBEngine = next
	if strings.TrimSpace(m.resourceState.CommandForm.DBPort) == "" || strings.TrimSpace(m.resourceState.CommandForm.DBPort) == oldDefaultPort {
		m.resourceState.CommandForm.DBPort = resourceservice.DatabaseDefaultPort(next)
	}
	if strings.TrimSpace(m.resourceState.CommandForm.DBUser) == "" || strings.TrimSpace(m.resourceState.CommandForm.DBUser) == oldDefaultUser {
		m.resourceState.CommandForm.DBUser = resourceservice.DatabaseDefaultUser(next)
	}
	if strings.TrimSpace(m.resourceState.CommandForm.DBName) == "" || strings.TrimSpace(m.resourceState.CommandForm.DBName) == oldDefaultName {
		m.resourceState.CommandForm.DBName = resourceservice.DatabaseDefaultName(next)
	}
	m.resourceState.CommandCursor = len([]rune(m.resourceCommandFieldValue(m.resourceState.CommandField)))
}

func (m *Model) moveResourceCommandCursor(delta int) {
	if m.resourceState.CommandForm.Kind == resourceDatabases && m.resourceState.CommandField == 0 {
		return
	}
	value := []rune(m.resourceCommandFieldValue(m.resourceState.CommandField))
	m.resourceState.CommandCursor = clampInt(m.resourceState.CommandCursor+delta, 0, len(value))
}

func (m Model) resourceCommandFieldValue(field int) string {
	switch field {
	case 0:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBEngine
		}
		if m.resourceState.CommandForm.Kind == resourcePorts {
			return m.resourceState.CommandForm.HealthCommand
		}
		return m.resourceState.CommandForm.StartCommand
	case 1:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBHost
		}
		return m.resourceState.CommandForm.StopCommand
	case 2:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBPort
		}
		return m.resourceState.CommandForm.RestartCommand
	case 3:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBUser
		}
		return m.resourceState.CommandForm.LogCommand
	case 4:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBPassword
		}
		return ""
	case 5:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBName
		}
		return ""
	case 6:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			return m.resourceState.CommandForm.DBNote
		}
		return ""
	default:
		return ""
	}
}

func (m *Model) setResourceCommandFieldValue(field int, value string) {
	switch field {
	case 0:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBEngine = resourceservice.NormalizeDatabaseEngine(value)
			return
		}
		if m.resourceState.CommandForm.Kind == resourcePorts {
			m.resourceState.CommandForm.HealthCommand = value
		} else {
			m.resourceState.CommandForm.StartCommand = value
		}
	case 1:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBHost = value
			return
		}
		m.resourceState.CommandForm.StopCommand = value
	case 2:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBPort = value
			return
		}
		m.resourceState.CommandForm.RestartCommand = value
	case 3:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBUser = value
			return
		}
		m.resourceState.CommandForm.LogCommand = value
	case 4:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBPassword = value
		}
	case 5:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBName = value
		}
	case 6:
		if m.resourceState.CommandForm.Kind == resourceDatabases {
			m.resourceState.CommandForm.DBNote = value
		}
	}
}

func (m *Model) resourceCommandAppend(value string) {
	runes := []rune(m.resourceCommandFieldValue(m.resourceState.CommandField))
	cursor := clampInt(m.resourceState.CommandCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceState.CommandField, string(next))
	m.resourceState.CommandCursor = cursor + len([]rune(value))
}

func (m *Model) resourceCommandBackspace() {
	runes := []rune(m.resourceCommandFieldValue(m.resourceState.CommandField))
	cursor := clampInt(m.resourceState.CommandCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceState.CommandField, string(next))
	m.resourceState.CommandCursor = cursor - 1
}

func (m *Model) saveResourceCommandForm() error {
	server := m.resourceState.CommandForm.Server
	kind := configResourceKind(m.resourceState.CommandForm.Kind)
	name := m.resourceState.CommandForm.Name
	idx := findManagedResource(m.resourceState.File.Items, server, kind, name)
	if idx < 0 {
		return fmt.Errorf("resource config not found")
	}
	item := m.resourceState.File.Items[idx]
	item.StartCommand = strings.TrimSpace(m.resourceState.CommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceState.CommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceState.CommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceState.CommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceState.CommandForm.HealthCommand)
	item.DBEngine = strings.TrimSpace(m.resourceState.CommandForm.DBEngine)
	item.DBHost = strings.TrimSpace(m.resourceState.CommandForm.DBHost)
	item.DBPort = strings.TrimSpace(m.resourceState.CommandForm.DBPort)
	item.DBUser = strings.TrimSpace(m.resourceState.CommandForm.DBUser)
	item.DBPassword = strings.TrimSpace(m.resourceState.CommandForm.DBPassword)
	item.DBName = strings.TrimSpace(m.resourceState.CommandForm.DBName)
	item.DBInstance = strings.TrimSpace(m.resourceState.CommandForm.DBInstance)
	item.DBNote = strings.TrimSpace(m.resourceState.CommandForm.DBNote)
	if m.resourceState.CommandForm.Kind == resourceDatabases && item.DBName != "" {
		item.Name = item.DBName
	}
	m.resourceState.File.Items[idx] = item
	if err := resourceservice.SaveConfig(m.home, m.resourceState.File); err != nil {
		return err
	}
	m.resourceState.File.Items = config.NormalizeManagedResources(m.resourceState.File.Items)
	m.applyManagedResources(m.resourceState.HostIndex)
	return nil
}
