package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) startResourceList(index int, kind resourceKind, back viewMode) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	if file, _, err := resourceservice.LoadConfig(m.home); err == nil {
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
	file, _, err := resourceservice.LoadCache(m.home)
	if err != nil {
		return
	}
	items, ok := config.ResourceContainerCacheForServer(file, m.resourceServerKey(index))
	if !ok {
		return
	}
	m.states[index].ContainerDetails = containerDetailsFromCache(items)
	m.states[index].ContainerError = ""
	resourceservice.AssociatePortContainers(m.states[index].PortDetails, m.states[index].ContainerDetails)
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
		result := (resourceservice.Service{}).FetchPart(ctx, h, resourceServiceKind(part))
		switch part {
		case resourceServices:
			msg.Services = result.Services
			msg.ServiceErr = resourceErrText(result, m.resourceRemoteErrorText)
		case resourceContainers:
			msg.Containers = result.Containers
			msg.ContainerErr = resourceErrText(result, m.resourceRemoteErrorText)
		case resourcePorts:
			msg.Ports = result.Ports
			msg.PortsErrText = resourceErrText(result, m.resourceRemoteErrorText)
		}
		return msg
	}
}

func resourceServiceKind(kind resourceKind) resourceservice.Kind {
	switch kind {
	case resourceServices:
		return resourceservice.KindServices
	case resourceContainers:
		return resourceservice.KindContainers
	case resourcePorts:
		return resourceservice.KindPorts
	default:
		return ""
	}
}

func resourceErrText(result resourceservice.PartResult, remoteErrText func(error) string) string {
	if result.ErrText != "" {
		return result.ErrText
	}
	if result.Err != nil {
		return remoteErrText(result.Err)
	}
	return ""
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
				m.resourceContainerExtra = resourceservice.ContainerExtraDetail{}
				m.resourceContainerExtraErr = ""
				m.resourceContainerExtraLoading = true
				return m, m.fetchContainerExtraDetail(m.resourceHostIndex, item.Name)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceServices {
			if item, ok := m.selectedService(); ok {
				m.resourceServiceExtraName = item.Unit
				m.resourceServiceExtra = resourceservice.ServiceDetail{}
				m.resourceServiceExtraErr = ""
				m.resourceServiceExtraLoading = true
				return m, m.fetchServiceExtraDetail(m.resourceHostIndex, item.Unit)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceProcesses {
			if item, ok := m.selectedProcess(); ok {
				m.resourceProcessExtraPID = item.PID
				m.resourceProcessExtra = resourceservice.ProcessExtraDetail{}
				m.resourceProcessExtraErr = ""
				m.resourceProcessExtraLoading = true
				return m, m.fetchProcessExtraDetail(m.resourceHostIndex, item.PID)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceDatabases {
			if item, ok := m.selectedDatabase(); ok {
				m.resourceDatabaseExtraName = item.Name
				m.resourceDatabaseExtra = resourceservice.DatabaseExtraDetail{}
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
	if err := resourceservice.SaveConfig(m.home, m.resourceFile); err != nil {
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
				state.ServiceDetails = append(state.ServiceDetails, resourceservice.ServiceDetail{Unit: item.Name, Load: "-", Active: "missing", Sub: "missing", Description: "Managed resource not found", Managed: true, Favorite: item.Favorite, Missing: true})
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
				state.ContainerDetails = append(state.ContainerDetails, resourceservice.ContainerDetail{Name: item.Name, Status: "missing", Managed: true, Favorite: item.Favorite, Missing: true})
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
				state.PortDetails = append(state.PortDetails, resourceservice.PortDetail{Process: item.Name, Count: 0, ProcessManaged: true, ProcessFavorite: item.Favorite, Missing: true})
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
				state.PortDetails = append(state.PortDetails, resourceservice.PortDetail{Protocol: proto, Port: port, Count: 0, Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindDatabase:
			if !managedDatabaseResourceConfigured(item) {
				continue
			}
			state.DatabaseDetails = append(state.DatabaseDetails, m.databaseDetailForManagedResource(item, state.DatabaseDetails))
		}
	}
}

func removeConfiguredDatabaseDetails(items []resourceservice.DatabaseDetail) []resourceservice.DatabaseDetail {
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

func (m Model) databaseDetailForManagedResource(item config.ManagedResource, discovered []resourceservice.DatabaseDetail) resourceservice.DatabaseDetail {
	detail := resourceservice.DatabaseDetail{
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

func managedDatabaseInstanceDetail(item config.ManagedResource, discovered []resourceservice.DatabaseDetail) (resourceservice.DatabaseDetail, bool) {
	instance := strings.TrimSpace(item.DBInstance)
	for _, db := range discovered {
		if instance != "" && strings.EqualFold(db.Name, instance) {
			return db, true
		}
	}
	port := strings.TrimSpace(item.DBPort)
	for _, db := range discovered {
		if port != "" && strings.Contains(db.Endpoint, port) && strings.EqualFold(resourceservice.NormalizeDatabaseEngine(db.Engine), resourceservice.NormalizeDatabaseEngine(item.DBEngine)) {
			return db, true
		}
	}
	return resourceservice.DatabaseDetail{}, false
}
