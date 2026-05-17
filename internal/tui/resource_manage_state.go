package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) resourceRefForManagedItem(item config.ManagedResource) (resourceRef, bool) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return resourceRef{}, false
	}
	state := m.states[m.resourceState.HostIndex]
	switch item.Kind {
	case config.ResourceKindService:
		for i := range state.ServiceDetails {
			if state.ServiceDetails[i].Unit == item.Name {
				return resourceRef{Kind: resourceServices, Index: i}, true
			}
		}
	case config.ResourceKindContainer:
		for i := range state.ContainerDetails {
			if state.ContainerDetails[i].Name == item.Name {
				return resourceRef{Kind: resourceContainers, Index: i}, true
			}
		}
	case config.ResourceKindProcess:
		for i := range state.PortDetails {
			if state.PortDetails[i].Process == item.Name {
				return resourceRef{Kind: resourceProcesses, Index: i}, true
			}
		}
	case config.ResourceKindPort:
		proto, port := splitManagedPortName(item.Name)
		for i := range state.PortDetails {
			if state.PortDetails[i].Protocol == proto && state.PortDetails[i].Port == port {
				return resourceRef{Kind: resourcePorts, Index: i}, true
			}
		}
	case config.ResourceKindDatabase:
		for i := range state.DatabaseDetails {
			if state.DatabaseDetails[i].Name == item.Name {
				return resourceRef{Kind: resourceDatabases, Index: i}, true
			}
		}
	}
	return resourceRef{}, false
}

func (m Model) missingResourceRefForManagedItem(item config.ManagedResource) resourceRef {
	return resourceRef{Kind: resourceKindFromConfig(item.Kind), Index: -1}
}

func (m Model) resourceMissingMetaForItem(item config.ManagedResource) string {
	if item.Kind == config.ResourceKindPort {
		return "  " + item.Name
	}
	if item.Kind == config.ResourceKindDatabase {
		meta := strings.TrimSpace(databaseManagedEndpoint(item))
		if meta == "" {
			meta = strings.TrimSpace(item.DBEngine)
		}
		if meta != "" {
			return "  " + meta
		}
	}
	return ""
}

func (m Model) resourceMetaForRef(ref resourceRef) string {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return ""
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index]
		return "  " + emptyDash(item.Status)
	case resourceServices:
		item := m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index]
		return "  " + serviceFullRawState(item)
	case resourceProcesses:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		return "  " + strings.TrimSpace(item.Protocol+"/"+item.Port)
	case resourcePorts:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		return "  " + emptyDash(portListenText(item))
	case resourceDatabases:
		item := m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index]
		return "  " + emptyDash(firstNonEmpty(item.Engine, item.Endpoint))
	default:
		return ""
	}
}

func serviceFullRawState(item resourceservice.ServiceDetail) string {
	load := strings.TrimSpace(item.Load)
	runtime := strings.TrimSpace(serviceRawState(item))
	if load == "" {
		return runtime
	}
	if runtime == "" {
		return load
	}
	return load + " " + runtime
}

func (m Model) resourceManageStatusForRef(ref resourceRef) string {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return ""
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index]
		return coloredContainerStatus(m.containerStatusLabel(item), containerDetailKind(item))
	case resourceServices:
		item := m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index]
		return coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))
	case resourceProcesses:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		return m.processStatusStyled(item)
	case resourcePorts:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		return m.portStatusStyledLabel(item, m.portStatusLabel(item))
	case resourceDatabases:
		item := m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index]
		return m.databaseStatusStyled(item)
	default:
		return ""
	}
}

func (m Model) processStatusStyled(item resourceservice.PortDetail) string {
	if item.Missing {
		return mutedStyle.Render(m.t("Not found", "未发现"))
	}
	if strings.TrimSpace(item.PID) != "" {
		return greenStyle.Render(m.t("Running", "运行"))
	}
	return yellowStyle.Render(m.t("Unknown", "未知"))
}

func (m Model) resourceManageDiscoveredRefs() []resourceRef {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return nil
	}
	refs := []resourceRef{}
	add := func(ref resourceRef) {
		if ref.Kind == resourceDatabases && m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index].Configured {
			return
		}
		if m.resourceRefAdded(ref) || m.resourceRefMissing(ref) {
			return
		}
		if !m.resourceManageQueryMatchesRef(ref) {
			return
		}
		refs = append(refs, ref)
	}
	switch m.resourceState.AddKind {
	case resourceContainers:
		for i := range m.states[m.resourceState.HostIndex].ContainerDetails {
			add(resourceRef{Kind: resourceContainers, Index: i})
		}
	case resourceServices:
		searching := strings.TrimSpace(m.resourceState.ManageQuery) != ""
		for i := range m.states[m.resourceState.HostIndex].ServiceDetails {
			if serviceNotFoundInactiveDead(m.states[m.resourceState.HostIndex].ServiceDetails[i]) && !searching {
				continue
			}
			add(resourceRef{Kind: resourceServices, Index: i})
		}
	case resourceProcesses:
		for _, ref := range m.allProcessRefs() {
			add(ref)
		}
	case resourcePorts:
		seen := map[string]bool{}
		for i := range m.states[m.resourceState.HostIndex].PortDetails {
			item := m.states[m.resourceState.HostIndex].PortDetails[i]
			key := strings.ToLower(strings.TrimSpace(item.Protocol)) + "/" + strings.TrimSpace(item.Port)
			if key == "/" || seen[key] {
				continue
			}
			seen[key] = true
			add(resourceRef{Kind: resourcePorts, Index: i})
		}
	case resourceDatabases:
		for i := range m.states[m.resourceState.HostIndex].DatabaseDetails {
			add(resourceRef{Kind: resourceDatabases, Index: i})
		}
	}
	m.sortResourceManagerRefs(refs)
	return refs
}

func (m Model) sortResourceManagerRefs(refs []resourceRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		aRank := m.resourceStatusRank(refs[i])
		bRank := m.resourceStatusRank(refs[j])
		if aRank != bRank {
			return aRank < bRank
		}
		return m.resourceSortNameValue(refs[i]) < m.resourceSortNameValue(refs[j])
	})
}

func (m Model) resourceManageFavorites() []config.ManagedResource {
	server := m.resourceServerKey(m.resourceState.HostIndex)
	kind := configResourceKind(m.resourceState.AddKind)
	query := strings.ToLower(strings.TrimSpace(m.resourceState.ManageQuery))
	items := []config.ManagedResource{}
	for _, item := range m.resourceState.File.Items {
		if item.Server == server && item.Kind == kind && item.Added {
			if item.Kind == config.ResourceKindDatabase && !managedDatabaseResourceConfigured(item) {
				continue
			}
			text := strings.Join([]string{
				item.Name, item.StartCommand, item.StopCommand, item.RestartCommand,
				item.LogCommand, item.HealthCommand, item.DBEngine, item.DBHost, item.DBPort,
				item.DBUser, item.DBName, item.DBInstance, item.DBNote,
			}, " ")
			if query != "" && !strings.Contains(strings.ToLower(text), query) {
				continue
			}
			items = append(items, item)
		}
	}
	m.sortResourceManagerItems(items)
	return items
}

func (m Model) sortResourceManagerItems(items []config.ManagedResource) {
	sort.SliceStable(items, func(i, j int) bool {
		aRank := m.resourceManagerItemStatusRank(items[i])
		bRank := m.resourceManagerItemStatusRank(items[j])
		if aRank != bRank {
			return aRank < bRank
		}
		return strings.ToLower(strings.TrimSpace(items[i].Name)) < strings.ToLower(strings.TrimSpace(items[j].Name))
	})
}

func (m Model) resourceManagerItemStatusRank(item config.ManagedResource) int {
	if ref, ok := m.resourceRefForManagedItem(item); ok {
		return m.resourceStatusRank(ref)
	}
	return 0
}

func (m Model) resourceManageQueryMatchesRef(ref resourceRef) bool {
	query := strings.ToLower(strings.TrimSpace(m.resourceState.ManageQuery))
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(m.resourceSearchText(ref)), query)
}

func (m Model) currentProcessRefs() []resourceRef {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return nil
	}
	seen := map[string]bool{}
	refs := []resourceRef{}
	for i, port := range m.states[m.resourceState.HostIndex].PortDetails {
		if !m.portLooksStandaloneProcess(port) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(port.Process)) + "/" + strings.TrimSpace(port.PID)
		if key == "/" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, resourceRef{Kind: resourceProcesses, Index: i})
	}
	return refs
}

func (m Model) allProcessRefs() []resourceRef {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return nil
	}
	seen := map[string]bool{}
	refs := []resourceRef{}
	for i, port := range m.states[m.resourceState.HostIndex].PortDetails {
		process := strings.TrimSpace(port.Process)
		if process == "" {
			continue
		}
		key := strings.ToLower(process)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, resourceRef{Kind: resourceProcesses, Index: i})
	}
	return refs
}

func (m Model) resourceNameForRef(ref resourceRef) (string, bool) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return "", false
	}
	switch ref.Kind {
	case resourceContainers:
		items := m.states[m.resourceState.HostIndex].ContainerDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Name, true
	case resourceServices:
		items := m.states[m.resourceState.HostIndex].ServiceDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Unit, true
	case resourceProcesses:
		items := m.states[m.resourceState.HostIndex].PortDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Process, true
	case resourcePorts:
		items := m.states[m.resourceState.HostIndex].PortDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return fmt.Sprintf("%s/%s", items[ref.Index].Protocol, items[ref.Index].Port), true
	case resourceDatabases:
		items := m.states[m.resourceState.HostIndex].DatabaseDetails
		if ref.Index < 0 || ref.Index >= len(items) {
			return "", false
		}
		return items[ref.Index].Name, true
	default:
		return "", false
	}
}

func (m Model) resourceRefInScope(ref resourceRef) bool {
	if m.resourceState.Scope == resourceScopeManaged {
		return m.resourceRefAdded(ref) && m.resourceRefFavorite(ref)
	}
	return m.resourceRefAdded(ref)
}

func (m Model) resourceRefManaged(ref resourceRef) bool {
	return m.resourceRefAdded(ref)
}

func (m Model) resourceRefAdded(ref resourceRef) bool {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return false
	}
	name, ok := m.resourceNameForRef(ref)
	if !ok || strings.TrimSpace(name) == "" {
		return false
	}
	kind := configResourceKind(ref.Kind)
	if kind == "" {
		return false
	}
	idx := findManagedResource(m.resourceState.File.Items, m.resourceServerKey(m.resourceState.HostIndex), kind, name)
	if idx >= 0 {
		if kind == config.ResourceKindDatabase && !managedDatabaseResourceConfigured(m.resourceState.File.Items[idx]) {
			return false
		}
		return m.resourceState.File.Items[idx].Added
	}
	switch ref.Kind {
	case resourceContainers:
		return m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index].Managed
	case resourceServices:
		return m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index].Managed
	case resourceProcesses:
		return m.states[m.resourceState.HostIndex].PortDetails[ref.Index].ProcessManaged
	case resourcePorts:
		return m.states[m.resourceState.HostIndex].PortDetails[ref.Index].Managed
	case resourceDatabases:
		return m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index].Managed
	default:
		return false
	}
}

func (m Model) resourceRefFavorite(ref resourceRef) bool {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return false
	}
	switch ref.Kind {
	case resourceContainers:
		return m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index].Favorite
	case resourceServices:
		return m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index].Favorite
	case resourceProcesses:
		return m.states[m.resourceState.HostIndex].PortDetails[ref.Index].ProcessFavorite
	case resourcePorts:
		return m.states[m.resourceState.HostIndex].PortDetails[ref.Index].Favorite
	case resourceDatabases:
		return m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index].Favorite
	default:
		return false
	}
}

func (m Model) selectedResourceManaged() bool {
	ref, ok := m.selectedResourceRef()
	if !ok {
		return false
	}
	return m.resourceRefManaged(ref)
}

func (m Model) resourceRefMissing(ref resourceRef) bool {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return false
	}
	switch ref.Kind {
	case resourceContainers:
		return m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index].Missing
	case resourceServices:
		return m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index].Missing
	case resourceProcesses:
		return false
	case resourcePorts:
		return m.states[m.resourceState.HostIndex].PortDetails[ref.Index].Missing
	case resourceDatabases:
		return m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index].Missing
	default:
		return false
	}
}
