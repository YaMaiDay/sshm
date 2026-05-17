package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	securityservice "github.com/YaMaiDay/sshm/internal/security"
	sessionservice "github.com/YaMaiDay/sshm/internal/session"
)

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c", "b":
		m.mode = modeDashboard
		m.detailScroll = 0
	case "j", "down":
		m.detailScroll = moveClampedInt(m.detailScroll, 1, 0, m.detailMaxScroll())
	case "k", "up":
		m.detailScroll = moveClampedInt(m.detailScroll, -1, 0, m.detailMaxScroll())
	case "tab", "right":
		m.moveDetailSection(1)
	case "shift+tab", "left":
		m.moveDetailSection(-1)
	case "u":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startUpload(idx), nil
		}
	case "d":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startDownload(idx), nil
		}
	case "r":
		if idx, ok := m.selectedRealIndex(); ok {
			m.states[idx].Loading = true
			m.states[idx].LoginLoading = true
			m.states[idx].LoginError = ""
			m.states[idx].FailedLoginError = ""
			m.states[idx].SSHDSecurityError = ""
			m.states[idx].PortDetailsError = ""
			m.states[idx].ContainerError = ""
			m.states[idx].PortDetails = nil
			m.states[idx].ContainerDetails = nil
			return m, tea.Batch(m.collectOne(idx), m.fetchLoginRecords(idx))
		}
	case "f":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.toggleFavorite(idx)
		}
	case "m":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startCommandList(idx), nil
		}
	case "n":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startResourceList(idx, resourceAll, modeDetail)
		}
	case "enter":
		if idx, ok := m.selectedRealIndex(); ok {
			cmd, cleanup := sessionservice.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Index: idx, Err: err}
			})
		}
	}
	return m, nil
}

func (m Model) updateAnomalyOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.anomalyItems()
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
	case "j", "down":
		m.anomalyIndex = clampInt(m.anomalyIndex+1, 0, maxInt(0, len(items)-1))
	case "k", "up":
		m.anomalyIndex = clampInt(m.anomalyIndex-1, 0, maxInt(0, len(items)-1))
	case "f", "tab":
		m.anomalyFilter = (m.anomalyFilter + 1) % 8
		m.anomalyIndex = 0
	case "0":
		m.anomalyFilter = anomalyAll
		m.anomalyIndex = 0
	case "1":
		m.anomalyFilter = anomalySevere
		m.anomalyIndex = 0
	case "2":
		m.anomalyFilter = anomalyWarn
		m.anomalyIndex = 0
	case "3":
		m.anomalyFilter = anomalyOffline
		m.anomalyIndex = 0
	case "4":
		m.anomalyFilter = anomalyResource
		m.anomalyIndex = 0
	case "5":
		m.anomalyFilter = anomalyContainer
		m.anomalyIndex = 0
	case "6":
		m.anomalyFilter = anomalyService
		m.anomalyIndex = 0
	case "7":
		m.anomalyFilter = anomalySecurity
		m.anomalyIndex = 0
	case "enter", " ":
		if len(items) == 0 {
			return m, nil
		}
		m.anomalyIndex = clampInt(m.anomalyIndex, 0, len(items)-1)
		item := items[m.anomalyIndex]
		m.selected = m.visibleIndexForRealIndex(item.Index)
		return m.openDetailSection(item.Index, anomalyDetailSection(item.Checks))
	case "r":
		m.status = "正在刷新全部服务器..."
		m.collectRound++
		m.manualRound = m.collectRound
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, true)
	}
	return m, nil
}

func (m Model) openDetailSection(index int, section string) (tea.Model, tea.Cmd) {
	model, cmd := m.openDetail(index)
	next, ok := model.(Model)
	if !ok {
		return model, cmd
	}
	next.setDetailSection(section)
	return next, cmd
}

func (m *Model) setDetailSection(section string) {
	if strings.TrimSpace(section) == "" {
		return
	}
	for i, name := range m.detailSectionNames() {
		if name == section {
			m.detailSectionIndex = i
			m.detailScroll = 0
			return
		}
	}
}

func (m Model) visibleIndexForRealIndex(realIndex int) int {
	indexes := m.filteredIndexes()
	for i, index := range indexes {
		if index == realIndex {
			return i
		}
	}
	return clampInt(m.selected, 0, maxInt(0, len(indexes)-1))
}

func (m *Model) moveDetailSection(delta int) {
	sections := m.detailSectionNames()
	if len(sections) == 0 {
		m.detailSectionIndex = 0
		return
	}
	m.detailSectionIndex = moveIndex(m.detailSectionIndex, len(sections), delta)
	m.detailScroll = 0
}

func (m Model) detailSectionNames() []string {
	sections := []string{
		m.t("Basic", "基础信息"),
		m.t("Resources", "资源监控"),
	}
	if idx, ok := m.selectedRealIndex(); ok && strings.TrimSpace(m.states[idx].Metrics.Error) != "" {
		sections = append(sections, m.t("Recent Error", "最近错误"))
	}
	sections = append(sections, m.t("Login Records", "登录记录"), m.t("Risks", "风险提示"))
	return sections
}

func (m Model) openDetail(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.states) {
		return m, nil
	}
	m.mode = modeDetail
	m.detailScroll = 0
	if len(m.states[idx].LoginSummary) > 0 || len(m.states[idx].FailedLoginSummary) > 0 || len(m.states[idx].SSHDSecurity) > 0 || m.states[idx].LoginLoading || m.states[idx].LoginError != "" || m.states[idx].FailedLoginError != "" || m.states[idx].SSHDSecurityError != "" {
		return m, nil
	}
	m.states[idx].LoginLoading = true
	return m, m.fetchLoginRecords(idx)
}

func (m Model) fetchLoginRecords(index int) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		result := (securityservice.Service{}).FetchLoginRecords(ctx, h)
		return loginRecordsMsg{
			Index:         index,
			Summary:       result.Summary,
			ErrText:       result.ErrText,
			FailedSummary: result.FailedSummary,
			FailedErrText: result.FailedErrText,
			SSHDSecurity:  result.SSHDSecurity,
			SSHDErrText:   result.SSHDErrText,
		}
	}
}
