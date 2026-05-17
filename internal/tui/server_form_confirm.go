package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n":
		m.mode = modeDashboard
		m.status = "已取消删除。"
	case "y", "enter":
		if m.serverForm.DeleteIndex < 0 || m.serverForm.DeleteIndex >= len(m.states) {
			m.mode = modeDashboard
			m.status = "没有选中的服务器。"
			return m, nil
		}
		h := m.states[m.serverForm.DeleteIndex].Host
		if err := config.DeleteHost(m.home, h, true); err != nil {
			m.mode = modeDashboard
			m.status = "删除失败：" + err.Error()
			return m, nil
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.mode = modeDashboard
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		m.status = "服务器已删除。"
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	}
	return m, nil
}

func (m Model) updateConfirmAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n", "q":
		m.mode = m.confirm.Back
		if m.confirm.Kind == confirmRemoveResource {
			m.status = m.t("Canceled.", "已取消。")
		} else {
			m.status = "已取消删除。"
		}
	case "y", "enter":
		switch m.confirm.Kind {
		case confirmDeleteCategory:
			name := m.confirm.Value
			if err := config.DeleteCategory(m.home, name); err != nil {
				m.mode = m.confirm.Back
				m.status = m.t("Delete category failed: ", "删除分类失败：") + m.categoryErrorText(err)
				return m, nil
			}
			m.reloadCategories("")
			m.serverForm.Form.Category = m.serverForm.Categories[m.serverForm.CategoryIndex]
			m.mode = modeAddForm
			m.status = m.t("Category deleted.", "分类已删除。")
		case confirmDeleteCommand:
			item := m.confirm.Command
			m.mode = modeCommandList
			return m.deleteCommandTemplate(item)
		case confirmDeleteHistory:
			entry := m.confirm.History
			m.mode = modeCommandHistory
			return m.deleteCommandHistoryEntry(entry)
		case confirmDeleteDeployment:
			index := m.confirm.Index
			m.mode = modeDeploymentList
			return m.deleteDeploymentApp(index)
		case confirmRemoveResource:
			item := m.confirm.Resource
			m.mode = m.confirm.Back
			return m.removeManagedResource(item)
		}
		m.confirm = confirmAction{}
	}
	return m, nil
}
