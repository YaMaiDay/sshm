package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.serverForm.Pane == 1 {
		return m.updateCategoryPane(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "ctrl+c":
		m.mode = modeDashboard
		m.serverForm.Copying = false
		m.status = m.t("Canceled.", "已取消。")
	case "tab":
		m.serverForm.Pane = 1
	case "down":
		m.serverForm.Index = m.nextFormIndex()
		m.serverForm.Cursor = m.formValueLen()
	case "shift+tab":
		m.serverForm.Pane = 1
	case "up":
		m.serverForm.Index = m.prevFormIndex()
		m.serverForm.Cursor = m.formValueLen()
	case "left":
		if m.serverForm.Index == 0 {
			m.moveCategory(-1)
		} else if m.serverForm.Index == jumpHostRefFormIndex {
			m.moveJumpHostRef(-1)
		} else {
			m.moveFormCursor(-1)
		}
	case "right":
		if m.serverForm.Index == 0 {
			m.moveCategory(1)
		} else if m.serverForm.Index == jumpHostRefFormIndex {
			m.moveJumpHostRef(1)
		} else {
			m.moveFormCursor(1)
		}
	case "enter":
		expireAt, err := normalizeExpireAtForSave(m.serverForm.Form.ExpireAt)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		favorite := false
		pinned := false
		pinnedOrder := int64(0)
		if m.serverForm.Editing {
			if m.serverForm.EditIndex < 0 || m.serverForm.EditIndex >= len(m.states) {
				m.status = "编辑失败：没有选中的服务器"
				return m, nil
			}
			favorite = m.states[m.serverForm.EditIndex].Host.Favorite
			pinned = m.states[m.serverForm.EditIndex].Host.Pinned
			pinnedOrder = m.states[m.serverForm.EditIndex].Host.PinnedOrder
		}
		input := config.HostInput{
			Category:     m.serverForm.Form.Category,
			Name:         m.serverForm.Form.Name,
			HostName:     m.serverForm.Form.HostName,
			User:         m.serverForm.Form.User,
			Port:         m.serverForm.Form.Port,
			IdentityFile: m.serverForm.Form.IdentityFile,
			Password:     m.serverForm.Form.Password,
			JumpHostRef:  m.serverForm.Form.JumpHostRef,
			Note:         m.serverForm.Form.Note,
			ExpireAt:     expireAt,
			Favorite:     favorite,
			Pinned:       pinned,
			PinnedOrder:  pinnedOrder,
		}
		if m.serverForm.Editing {
			if err := config.EditHost(m.home, m.states[m.serverForm.EditIndex].Host, input); err != nil {
				m.status = "编辑失败：" + err.Error()
				return m, nil
			}
		} else {
			if err := config.AddHost(m.home, input); err != nil {
				m.status = "添加失败：" + err.Error()
				return m, nil
			}
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		if m.serverForm.Editing {
			m.status = "服务器已更新。"
		} else if m.serverForm.Copying {
			m.status = "服务器已复制。"
		} else {
			m.status = "服务器已添加。"
		}
		m.serverForm.Copying = false
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	case "backspace":
		if m.serverForm.Index == expireAtFormIndex {
			m.formExpireBackspace()
		} else {
			m.formBackspace()
		}
	default:
		if len(msg.Runes) > 0 && m.serverForm.Index != 0 {
			if m.serverForm.Index == expireAtFormIndex {
				m.formExpireAppend(msg.Runes)
			} else {
				m.formAppend(string(msg.Runes))
			}
		}
	}
	return m, nil
}
