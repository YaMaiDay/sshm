package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func (m *Model) reloadHosts(hosts []host.Host) {
	states := make([]hostState, len(hosts))
	for i, h := range hosts {
		states[i] = hostState{Host: h, Loading: true}
	}
	m.states = states
	m.selected = 0
	m.query = ""
	m.passwords = config.PasswordsFromHosts(hosts)
	m.collector = monitor.NewCollector(m.passwords)
	m.collector.Timeout = m.appConfig.CommandDuration()
	m.collector.ConnectTimeout = m.appConfig.ConnectDuration()
	m.reloadCategories("")
}

func (m *Model) recordLastLogin(h host.Host, at time.Time) {
	config.SetLastLogin(&m.appState, h, at)
	if err := config.SaveState(m.home, m.appState); err != nil {
		m.status = m.t("Failed to save last login: ", "最近登录保存失败：") + err.Error()
	}
}

func (m Model) lastLogin(h host.Host) time.Time {
	return config.LastLoginFor(m.appState, h)
}

func (m Model) toggleFavorite(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	hosts := make([]host.Host, len(m.states))
	for i, state := range m.states {
		hosts[i] = state.Host
	}
	hosts[index].Favorite = !hosts[index].Favorite
	if err := config.SaveServerHosts(m.home, hosts); err != nil {
		m.status = m.t("Failed to update favorite: ", "收藏更新失败：") + err.Error()
		return m, nil
	}
	m.states[index].Host.Favorite = hosts[index].Favorite
	if hosts[index].Favorite {
		m.status = m.t("Favorited: ", "已收藏：") + hosts[index].Name
	} else {
		m.status = m.t("Unfavorited: ", "已取消收藏：") + hosts[index].Name
	}
	if m.favoriteOnly && !hosts[index].Favorite {
		m.selected = 0
	}
	return m, nil
}

func (m Model) togglePinned(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	hosts := make([]host.Host, len(m.states))
	for i, state := range m.states {
		hosts[i] = state.Host
	}
	if hosts[index].Pinned {
		hosts[index].Pinned = false
		hosts[index].PinnedOrder = 0
	} else {
		hosts[index].Pinned = true
		hosts[index].PinnedOrder = nextPinnedOrder(hosts)
	}
	if err := config.SaveServerHosts(m.home, hosts); err != nil {
		m.status = m.t("Failed to update pin: ", "置顶更新失败：") + err.Error()
		return m, nil
	}
	m.states[index].Host.Pinned = hosts[index].Pinned
	m.states[index].Host.PinnedOrder = hosts[index].PinnedOrder
	if hosts[index].Pinned {
		m.status = m.t("Pinned: ", "已置顶：") + hosts[index].Name
	} else {
		m.status = m.t("Unpinned: ", "已取消置顶：") + hosts[index].Name
	}
	return m, nil
}

func nextPinnedOrder(hosts []host.Host) int64 {
	var maxOrder int64
	for _, h := range hosts {
		if h.PinnedOrder > maxOrder {
			maxOrder = h.PinnedOrder
		}
	}
	return maxOrder + 1
}
