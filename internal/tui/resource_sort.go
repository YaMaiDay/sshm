package tui

import (
	"sort"
	"strconv"
	"strings"
)

func (m Model) filteredResourceIndexes() []resourceRef {
	items := m.currentResourceRefs()
	query := strings.ToLower(strings.TrimSpace(m.resourceState.Query))
	indexes := []resourceRef{}
	for _, ref := range items {
		text := strings.ToLower(m.resourceSearchText(ref))
		if query != "" && !strings.Contains(text, query) {
			continue
		}
		if !m.resourceFilterMatches(ref) {
			continue
		}
		if !m.resourcePortFilterMatches(ref) {
			continue
		}
		indexes = append(indexes, ref)
	}
	m.sortResourceRefs(indexes)
	return indexes
}

func (m Model) sortResourceRefs(refs []resourceRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		a := refs[i]
		b := refs[j]
		aPinned, aOrder := m.resourceRefPinned(a)
		bPinned, bOrder := m.resourceRefPinned(b)
		if aPinned != bPinned {
			return aPinned
		}
		if aPinned && bPinned && aOrder != bOrder {
			return aOrder > bOrder
		}
		if m.resourceState.Kind == resourceAll && a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if m.resourceState.Sort == resourceSortDefault {
			return false
		}
		switch m.resourceState.Sort {
		case resourceSortStatus:
			ar := m.resourceStatusRank(a)
			br := m.resourceStatusRank(b)
			if ar != br {
				return ar < br
			}
		case resourceSortName:
			return m.resourceSortNameValue(a) < m.resourceSortNameValue(b)
		case resourceSortCPU:
			av, aok := m.resourceCPUValue(a)
			bv, bok := m.resourceCPUValue(b)
			if aok && bok && av != bv {
				return av > bv
			}
			if aok != bok {
				return aok
			}
		case resourceSortMemory:
			av, aok := m.resourceMemoryValue(a)
			bv, bok := m.resourceMemoryValue(b)
			if aok && bok && av != bv {
				return av > bv
			}
			if aok != bok {
				return aok
			}
		case resourceSortPort:
			ap, aok := m.resourcePortValue(a)
			bp, bok := m.resourcePortValue(b)
			if aok && bok && ap != bp {
				return ap < bp
			}
			if aok != bok {
				return aok
			}
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return m.resourceSortNameValue(a) < m.resourceSortNameValue(b)
	})
}

func (m Model) resourceRefPinned(ref resourceRef) (bool, int64) {
	name, ok := m.resourceNameForRef(ref)
	if !ok {
		return false, 0
	}
	item, ok := m.managedResource(ref.Kind, name)
	if !ok {
		return false, 0
	}
	return item.Pinned, item.PinnedOrder
}

func (m Model) resourceSortNameValue(ref resourceRef) string {
	name, _ := m.resourceNameForRef(ref)
	return strings.ToLower(strings.TrimSpace(name))
}

func (m Model) resourceStatusRank(ref resourceRef) int {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return 9
	}
	switch ref.Kind {
	case resourceContainers:
		item := m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index]
		switch containerDetailKind(item) {
		case "failed", "missing":
			return 0
		case "running":
			return 1
		case "stopped":
			return 2
		default:
			return 3
		}
	case resourceServices:
		item := m.states[m.resourceState.HostIndex].ServiceDetails[ref.Index]
		switch serviceDetailKind(item) {
		case "failed", "missing":
			return 0
		case "running":
			return 1
		case "active":
			return 2
		case "stopped":
			return 3
		default:
			return 4
		}
	case resourceProcesses:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		if item.Missing || strings.TrimSpace(item.PID) == "" {
			return 2
		}
		return 1
	case resourcePorts:
		item := m.states[m.resourceState.HostIndex].PortDetails[ref.Index]
		if item.Missing {
			return 2
		}
		if strings.EqualFold(strings.TrimSpace(item.State), "LISTEN") || strings.TrimSpace(item.State) == "" {
			return 1
		}
		return 3
	case resourceDatabases:
		item := m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index]
		return databaseStatusRank(item)
	default:
		return 9
	}
}

func (m Model) resourceCPUValue(ref resourceRef) (float64, bool) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return 0, false
	}
	switch ref.Kind {
	case resourceContainers:
		return parsePercentText(m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index].CPU)
	default:
		return 0, false
	}
}

func (m Model) resourceMemoryValue(ref resourceRef) (float64, bool) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return 0, false
	}
	switch ref.Kind {
	case resourceContainers:
		return parsePercentText(m.states[m.resourceState.HostIndex].ContainerDetails[ref.Index].MemPerc)
	default:
		return 0, false
	}
}

func (m Model) resourcePortValue(ref resourceRef) (int, bool) {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return 0, false
	}
	if ref.Kind == resourcePorts || ref.Kind == resourceProcesses {
		port := strings.TrimSpace(m.states[m.resourceState.HostIndex].PortDetails[ref.Index].Port)
		n, err := strconv.Atoi(port)
		return n, err == nil
	}
	if ref.Kind == resourceDatabases {
		port := strings.TrimSpace(m.states[m.resourceState.HostIndex].DatabaseDetails[ref.Index].Port)
		n, err := strconv.Atoi(port)
		return n, err == nil
	}
	return 0, false
}

func (m Model) currentResourceRefs() []resourceRef {
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return nil
	}
	refs := []resourceRef{}
	if m.resourceState.Kind == resourceAll || m.resourceState.Kind == resourceContainers {
		for i := range m.states[m.resourceState.HostIndex].ContainerDetails {
			ref := resourceRef{Kind: resourceContainers, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceState.Kind == resourceAll || m.resourceState.Kind == resourceServices {
		for i := range m.states[m.resourceState.HostIndex].ServiceDetails {
			if !resourceServiceVisible(m.states[m.resourceState.HostIndex].ServiceDetails[i]) {
				continue
			}
			ref := resourceRef{Kind: resourceServices, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceState.Kind == resourceAll || m.resourceState.Kind == resourceProcesses {
		for _, ref := range m.currentProcessRefs() {
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceState.Kind == resourceAll || m.resourceState.Kind == resourcePorts {
		for i := range m.states[m.resourceState.HostIndex].PortDetails {
			ref := resourceRef{Kind: resourcePorts, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	if m.resourceState.Kind == resourceAll || m.resourceState.Kind == resourceDatabases {
		for i := range m.states[m.resourceState.HostIndex].DatabaseDetails {
			ref := resourceRef{Kind: resourceDatabases, Index: i}
			if m.resourceRefInScope(ref) {
				refs = append(refs, ref)
			}
		}
	}
	return refs
}
