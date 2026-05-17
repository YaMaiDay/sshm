package tui

import "strings"

func (m Model) viewByMode() (string, bool) {
	switch m.mode {
	case modeAddForm:
		return m.renderAddForm(), true
	case modeDeleteConfirm:
		return m.renderDeleteConfirm(), true
	case modeConfirmAction:
		return m.renderConfirmAction(), true
	case modeDetail:
		return m.renderDetail(), true
	case modeTransferPanel:
		return m.renderTransferPanel(), true
	case modeCommandList:
		return m.renderCommandList(), true
	case modeCommandEdit:
		return m.renderCommandEdit(), true
	case modeCommandConfirm:
		return m.renderCommandConfirm(), true
	case modeCommandOutput:
		return m.renderCommandOutput(), true
	case modeBatchSelect:
		return m.renderBatchSelect(), true
	case modeBatchCommandList:
		return m.renderBatchCommandList(), true
	case modeBatchCommandEdit:
		return m.renderBatchCommandEdit(), true
	case modeBatchConfirm:
		return m.renderBatchConfirm(), true
	case modeBatchOutput:
		return m.renderBatchOutput(), true
	case modeCommandHistory:
		return m.renderCommandHistory(), true
	case modeCommandHistoryDetail:
		return m.renderCommandHistoryDetail(), true
	case modeAnomalyOverview:
		return m.renderAnomalyOverview(), true
	case modeDeploymentList:
		return m.renderDeploymentList(), true
	case modeDeploymentDetail:
		return m.renderDeploymentDetail(), true
	case modeDeploymentEdit:
		return m.renderDeploymentEdit(), true
	case modeDeploymentConfirm:
		return m.renderDeploymentConfirm(), true
	case modeDeploymentRollbackConfirm:
		return m.renderDeploymentRollbackConfirm(), true
	case modeDeploymentOutput:
		return m.renderDeploymentOutput(), true
	case modeSettings:
		return m.renderSettings(), true
	case modeTransferJobs:
		return m.renderTransferJobs(), true
	case modeTransferDetail:
		return m.renderTransferDetail(), true
	case modeHelp:
		return m.renderHelpPanel(), true
	case modeResourceList:
		return m.renderResourceList(), true
	case modeResourceDetail:
		return m.renderResourceDetail(), true
	case modeResourceAdd:
		return m.renderResourceAdd(), true
	case modeResourceAddEdit:
		return m.renderResourceAddEdit(), true
	case modeResourceLog:
		return m.renderResourceLog(), true
	case modeResourceCommandEdit:
		return m.renderResourceCommandEdit(), true
	case modeResourceConfirm:
		return m.renderResourceConfirm(), true
	case modeResourceOutput:
		return m.renderResourceOutput(), true
	}
	if m.mode != modeDashboard {
		return m.renderPicker(), true
	}
	return "", false
}

func (m Model) renderDashboardView() string {
	indexes := m.filteredIndexes()
	headerWidth := m.width
	if headerWidth < 1 {
		headerWidth = contentWidth(m.width)
	}
	header := fitANSI(m.dashboardHeaderText(indexes), headerWidth)

	var lines []string
	lines = append(lines, header)
	if m.dashboard.Mode != dashboardCategory {
		lines = append(lines, "")
	}

	if len(m.states) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No servers. Press a to add one.", "没有服务器。按 a 添加服务器。")))
	} else if len(indexes) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No matching servers", "没有匹配的服务器")))
	} else {
		lines = append(lines, m.renderDashboard(indexes))
	}

	helpWidth := m.width
	if helpWidth < 1 {
		helpWidth = contentWidth(m.width)
	}
	helpBlock := m.renderDashboardHelp(helpWidth)
	pageDots := ""
	if m.dashboard.Mode == dashboardCards {
		pageDots = m.dashboardPageDots(indexes)
	} else if m.dashboard.Mode == dashboardGrouped {
		pageDots = m.dashboardGroupedDots(indexes)
	}
	reservedBottomLines := strings.Count(helpBlock, "\n") + 1
	if pageDots != "" {
		reservedBottomLines += strings.Count(pageDots, "\n") + 1
	}
	lines = padToBottom(lines, m.height, reservedBottomLines)
	if pageDots != "" {
		lines = append(lines, pageDots)
	}
	lines = append(lines, helpBlock)
	return strings.Join(lines, "\n")
}
