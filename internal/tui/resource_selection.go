package tui

import (
	"fmt"
	"strings"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) resourceSearchText(ref resourceRef) string {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return ""
	}
	if ref.Kind == resourceContainers {
		item := m.states[m.resourceHostIndex].ContainerDetails[ref.Index]
		return strings.Join([]string{item.Name, item.Image, item.Status, item.Ports}, " ")
	}
	if ref.Kind == resourcePorts {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return strings.Join([]string{item.Protocol, item.Port, item.LocalAddress, item.ForeignAddress, item.State, item.Process, item.PID, item.FD, item.ServiceUnit, item.Container, item.ContainerPort}, " ")
	}
	if ref.Kind == resourceProcesses {
		item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
		return strings.Join([]string{item.Process, item.PID, item.ServiceUnit, item.Protocol, item.Port, item.LocalAddress, item.State}, " ")
	}
	if ref.Kind == resourceDatabases {
		item := m.states[m.resourceHostIndex].DatabaseDetails[ref.Index]
		return strings.Join([]string{item.Name, item.Engine, item.Source, item.Status, item.RawStatus, item.Endpoint, item.ServiceUnit, item.Container, item.Image, item.Process, item.PID, item.Port}, " ")
	}
	item := m.states[m.resourceHostIndex].ServiceDetails[ref.Index]
	return strings.Join([]string{item.Unit, item.Load, item.Active, item.Sub, item.Description, item.FragmentPath, item.WorkingDirectory, item.ExecStart}, " ")
}

func (m Model) resourceFilterMatches(ref resourceRef) bool {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	if m.resourceKind == resourcePorts && ref.Kind == resourcePorts {
		return true
	}
	switch m.resourceFilter {
	case resourceFilterRunning:
		if ref.Kind == resourceContainers {
			return containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index]) == "running"
		}
		if ref.Kind == resourcePorts {
			return !m.states[m.resourceHostIndex].PortDetails[ref.Index].Missing
		}
		if ref.Kind == resourceProcesses {
			return true
		}
		if ref.Kind == resourceDatabases {
			return m.states[m.resourceHostIndex].DatabaseDetails[ref.Index].Status == "running"
		}
		return serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index]) == "running"
	case resourceFilterProblems:
		if ref.Kind == resourceContainers {
			kind := containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index])
			return kind == "failed" || kind == "missing"
		}
		if ref.Kind == resourcePorts {
			return m.states[m.resourceHostIndex].PortDetails[ref.Index].Missing
		}
		if ref.Kind == resourceProcesses {
			return false
		}
		if ref.Kind == resourceDatabases {
			status := m.states[m.resourceHostIndex].DatabaseDetails[ref.Index].Status
			return status == "problem" || status == "missing"
		}
		kind := serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index])
		return kind == "failed" || kind == "missing"
	case resourceFilterStopped:
		if ref.Kind == resourceContainers {
			return containerDetailKind(m.states[m.resourceHostIndex].ContainerDetails[ref.Index]) == "stopped"
		}
		if ref.Kind == resourcePorts {
			return false
		}
		if ref.Kind == resourceProcesses {
			return false
		}
		if ref.Kind == resourceDatabases {
			return m.states[m.resourceHostIndex].DatabaseDetails[ref.Index].Status == "stopped"
		}
		return serviceDetailKind(m.states[m.resourceHostIndex].ServiceDetails[ref.Index]) == "stopped"
	default:
		return true
	}
}

func (m Model) resourcePortFilterMatches(ref resourceRef) bool {
	if m.resourceKind != resourcePorts || ref.Kind != resourcePorts {
		return true
	}
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return false
	}
	item := m.states[m.resourceHostIndex].PortDetails[ref.Index]
	switch m.resourcePortFilter {
	case resourcePortFilterPublic:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeWildcard
	case resourcePortFilterLoopback:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeLoopback
	case resourcePortFilterSpecific:
		return !item.Missing && portAddressScope(item.LocalAddress) == portScopeSpecific
	case resourcePortFilterContainer:
		return strings.TrimSpace(item.Container) != ""
	case resourcePortFilterProcess:
		return strings.TrimSpace(item.Process) != ""
	default:
		return true
	}
}

func (m Model) selectedResourceName() (string, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok {
		return "", false
	}
	if ref.Kind == resourceContainers {
		item, ok := m.selectedContainer()
		return item.Name, ok
	}
	if ref.Kind == resourcePorts {
		item, ok := m.selectedPort()
		if !ok {
			return "", false
		}
		return fmt.Sprintf("%s/%s", item.Protocol, item.Port), true
	}
	if ref.Kind == resourceProcesses {
		item, ok := m.selectedProcess()
		return item.Process, ok
	}
	if ref.Kind == resourceDatabases {
		item, ok := m.selectedDatabase()
		return item.Name, ok
	}
	item, ok := m.selectedService()
	return item.Unit, ok
}

func (m Model) selectedResourceRef() (resourceRef, bool) {
	indexes := m.filteredResourceIndexes()
	if len(indexes) == 0 || m.resourceIndex < 0 || m.resourceIndex >= len(indexes) {
		return resourceRef{}, false
	}
	return indexes[m.resourceIndex], true
}

func (m Model) currentSelectedResourceKind() resourceKind {
	if strings.TrimSpace(m.resourceDetailName) != "" {
		return m.resourceDetailKind
	}
	if ref, ok := m.selectedResourceRef(); ok {
		return ref.Kind
	}
	if m.resourceKind == resourcePorts {
		return resourcePorts
	}
	if m.resourceKind == resourceProcesses {
		return resourceProcesses
	}
	if m.resourceKind == resourceServices {
		return resourceServices
	}
	if m.resourceKind == resourceDatabases {
		return resourceDatabases
	}
	return resourceContainers
}

func (m Model) selectedDatabase() (resourceservice.DatabaseDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceDatabases || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceservice.DatabaseDetail{}, false
	}
	items := m.states[m.resourceHostIndex].DatabaseDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return resourceservice.DatabaseDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedPort() (resourceservice.PortDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourcePorts || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceservice.PortDetail{}, false
	}
	items := m.states[m.resourceHostIndex].PortDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return resourceservice.PortDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedProcess() (resourceservice.PortDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceProcesses || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceservice.PortDetail{}, false
	}
	items := m.states[m.resourceHostIndex].PortDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return resourceservice.PortDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedContainer() (resourceservice.ContainerDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceContainers || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceservice.ContainerDetail{}, false
	}
	items := m.states[m.resourceHostIndex].ContainerDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return resourceservice.ContainerDetail{}, false
	}
	return items[real], true
}

func (m Model) selectedService() (resourceservice.ServiceDetail, bool) {
	ref, ok := m.selectedResourceRef()
	if !ok || ref.Kind != resourceServices || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return resourceservice.ServiceDetail{}, false
	}
	items := m.states[m.resourceHostIndex].ServiceDetails
	real := ref.Index
	if real < 0 || real >= len(items) {
		return resourceservice.ServiceDetail{}, false
	}
	return items[real], true
}
