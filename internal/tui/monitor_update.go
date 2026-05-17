package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/monitor"
)

func (m Model) collectAll(round int, manual bool) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.states))
	for i, state := range m.states {
		index := i
		h := state.Host
		cmds = append(cmds, func() tea.Msg {
			metrics := m.collector.Collect(context.Background(), h)
			return collectMsg{Index: index, Round: round, Metrics: metrics, Manual: manual}
		})
	}
	return tea.Batch(cmds...)
}

func (m Model) collectOne(index int) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	round := m.collectRound
	return func() tea.Msg {
		metrics := m.collector.Collect(context.Background(), h)
		return collectMsg{Index: index, Round: round, Metrics: metrics}
	}
}

func (m *Model) applyMetrics(index int, metrics monitor.Metrics) {
	if index < 0 || index >= len(m.states) {
		return
	}
	state := &m.states[index]
	state.LastAttempt = time.Now()
	if metrics.Online {
		state.Metrics = metrics
		state.FailureCount = 0
	} else if state.Metrics.Online {
		state.FailureCount++
		if state.FailureCount >= 2 {
			state.Metrics = metrics
		}
	} else {
		state.Metrics = metrics
		state.FailureCount++
	}
	state.Loading = false
}

func tickAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
