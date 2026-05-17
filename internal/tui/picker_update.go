package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeDashboard
		m.transferState.Mode = transferNone
		m.transferState.Choices = nil
		m.transferState.RemoteTree = remoteTree{}
		m.transferState.PickIndex = 0
		m.status = m.t("Canceled.", "已取消。")
	case "j", "down":
		m.movePick(1)
	case "k", "up":
		m.movePick(-1)
	case "l", "right":
		if m.treePickerActive() {
			return m.expandTreePick()
		}
	case "h", "left":
		if m.treePickerActive() {
			return m.collapseTreePick(), nil
		}
	case " ":
		return m.confirmPick()
	case "enter":
		if m.treePickerActive() {
			return m.toggleTreePick()
		}
		return m.confirmPick()
	}
	return m, nil
}
