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
		m.resourceServiceAt = now
	}
	if msg.Kind == resourceContainers {
		m.states[msg.Index].ContainerDetails = msg.Containers
		m.states[msg.Index].ContainerError = msg.ContainerErr
		m.resourceContainerAt = now
		if msg.ContainerErr == "" {
			_ = resourceservice.UpsertContainerCache(m.home, m.resourceServerKey(msg.Index), containerDetailsToCache(msg.Containers), now)
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
	resourceservice.AssociatePortContainers(m.states[msg.Index].PortDetails, m.states[msg.Index].ContainerDetails)
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
