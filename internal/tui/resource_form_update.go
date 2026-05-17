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
			m.resourceCommandForm.DBPort = resourceservice.DatabaseDefaultPort(m.resourceCommandForm.DBEngine)
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
		if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
			if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
	m.resourceCommandForm.DBEngine = next
	if strings.TrimSpace(m.resourceCommandForm.DBPort) == "" || strings.TrimSpace(m.resourceCommandForm.DBPort) == oldDefaultPort {
		m.resourceCommandForm.DBPort = resourceservice.DatabaseDefaultPort(next)
	}
	if strings.TrimSpace(m.resourceCommandForm.DBUser) == "" || strings.TrimSpace(m.resourceCommandForm.DBUser) == oldDefaultUser {
		m.resourceCommandForm.DBUser = resourceservice.DatabaseDefaultUser(next)
	}
	if strings.TrimSpace(m.resourceCommandForm.DBName) == "" || strings.TrimSpace(m.resourceCommandForm.DBName) == oldDefaultName {
		m.resourceCommandForm.DBName = resourceservice.DatabaseDefaultName(next)
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
			m.resourceCommandForm.DBEngine = resourceservice.NormalizeDatabaseEngine(value)
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
		return err
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	return nil
}
