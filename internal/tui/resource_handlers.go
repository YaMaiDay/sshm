package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) handleResourceLoad(msg resourceLoadMsg) (tea.Model, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.states) {
		return m, nil
	}
	now := time.Now()
	if msg.Kind == resourceServices {
		m.states[msg.Index].ServiceDetails = msg.Services
		m.states[msg.Index].ServiceError = msg.ServiceErr
		m.resourceState.ServiceAt = now
	}
	if msg.Kind == resourceContainers {
		m.states[msg.Index].ContainerDetails = msg.Containers
		m.states[msg.Index].ContainerError = msg.ContainerErr
		m.resourceState.ContainerAt = now
		if msg.ContainerErr == "" {
			if err := resourceservice.UpsertContainerCache(m.home, m.resourceServerKey(msg.Index), containerDetailsToCache(msg.Containers), now); err != nil {
				m.resourceState.CacheWarning = m.t("Resource cache save failed: ", "资源缓存保存失败：") + err.Error()
			}
		}
	}
	if msg.Kind == resourcePorts {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourceState.PortAt = now
	}
	if msg.Kind == resourceServices && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourceState.PortAt = now
	}
	if msg.Kind == resourceContainers && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourceState.PortAt = now
	}
	resourceservice.AssociatePortContainers(m.states[msg.Index].PortDetails, m.states[msg.Index].ContainerDetails)
	m.states[msg.Index].DatabaseDetails, m.states[msg.Index].DatabaseError = deriveDatabaseDetails(m.states[msg.Index].ServiceDetails, m.states[msg.Index].ContainerDetails, m.states[msg.Index].PortDetails)
	m.applyManagedResources(msg.Index)
	m.resourceState.CollectedAt = now
	if m.resourceState.LoadingPending > 0 {
		m.resourceState.LoadingPending--
	}
	if m.resourceState.LoadingPending <= 0 {
		m.resourceState.Loading = false
		if m.resourceState.ManualRefresh {
			m.resourceState.RefreshStatus = fmt.Sprintf("%s%s", m.t("Manual refresh done: ", "手动刷新完成："), now.Format("15:04:05"))
		} else {
			m.resourceState.RefreshStatus = fmt.Sprintf("%s%s", m.t("Last refresh: ", "最后刷新："), now.Format("15:04:05"))
		}
		if strings.TrimSpace(m.resourceState.CacheWarning) != "" {
			m.resourceState.RefreshStatus += " · " + m.resourceState.CacheWarning
		}
		m.resourceState.ManualRefresh = false
		m.status = m.resourceState.RefreshStatus
		return m, m.fetchDatabaseCardExtras(msg.Index)
	}
	m.status = m.t("Loading resources...", "正在读取资源...")
	return m, nil
}

func (m Model) handleResourceContainerDetail(msg resourceContainerDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceState.HostIndex || msg.Name != m.resourceState.ContainerExtraName {
		return m, nil
	}
	m.resourceState.ContainerExtraLoading = false
	m.resourceState.ContainerExtra = msg.Detail
	m.resourceState.ContainerExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceServiceDetail(msg resourceServiceDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceState.HostIndex || msg.Name != m.resourceState.ServiceExtraName {
		return m, nil
	}
	m.resourceState.ServiceExtraLoading = false
	m.resourceState.ServiceExtra = msg.Detail
	m.resourceState.ServiceExtraErr = msg.Err
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
	if msg.Index != m.resourceState.HostIndex || msg.PID != m.resourceState.ProcessExtraPID {
		return m, nil
	}
	m.resourceState.ProcessExtraLoading = false
	m.resourceState.ProcessExtra = msg.Detail
	m.resourceState.ProcessExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceDatabaseDetail(msg resourceDatabaseDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceState.HostIndex {
		return m, nil
	}
	m.setDatabaseExtraCache(msg.Name, msg.Detail, msg.Err, false)
	if msg.Name == m.resourceState.DatabaseExtraName {
		m.resourceState.DatabaseExtraLoading = false
		m.resourceState.DatabaseExtra = msg.Detail
		m.resourceState.DatabaseExtraErr = msg.Err
	}
	return m, nil
}

func (m Model) handleResourceLog(msg resourceLogMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceState.HostIndex || msg.Kind != m.resourceState.LogKind || msg.Name != m.resourceState.LogName {
		return m, nil
	}
	m.resourceState.LogOutput = strings.TrimRight(msg.Output, "\n")
	if m.resourceState.LogOutput == "" {
		m.resourceState.LogOutput = m.t("No log output.", "没有日志输出。")
	}
	if msg.Result.Err != nil {
		m.status = fmt.Sprintf("%s %d", m.t("Log command failed, exit", "日志命令失败，退出码"), msg.Result.ExitCode)
	}
	return m, nil
}

func (m Model) handleResourceAction(msg resourceActionMsg) (tea.Model, tea.Cmd) {
	m.resourceState.ActionRunning = false
	m.resourceState.ActionOutput = strings.TrimRight(msg.Result.Output, "\n")
	m.resourceState.ActionExitCode = msg.Result.ExitCode
	if msg.Result.Err != nil {
		m.status = m.resourceActionErrorText(msg.Result)
		return m, nil
	}
	m.status = m.resourceActionNameText(msg.Action) + m.t(" complete.", "完成。")
	refreshKind := msg.Kind
	if refreshKind == resourcePorts {
		refreshKind = resourceAll
	}
	m.resourceState.Loading = true
	m.resourceState.LoadingKind = refreshKind
	m.resourceState.LoadingPending = resourceLoadPartCount(refreshKind)
	m.resourceState.ManualRefresh = false
	m.resourceState.CacheWarning = ""
	return m, m.fetchResourceDetails(msg.Index, refreshKind)
}
