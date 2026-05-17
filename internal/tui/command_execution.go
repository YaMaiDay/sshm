package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	commandservice "github.com/YaMaiDay/sshm/internal/command"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

func (m Model) commandOutputMaxScroll() int {
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	lines := 2
	if m.activeCommand.Running {
		lines++
	} else {
		output := strings.TrimRight(m.activeCommand.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(strings.Split(output, "\n"))
		}
		lines += 2
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) runCommand(index int, script string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result := (commandservice.Service{}).Run(ctx, m.states[index].Host, remotescript.NewUserScript(script))
		return commandDoneMsg{Result: commandResultFromCommand(result)}
	}
}

func (m *Model) recordCommandHistory(result commandResult) error {
	if m.activeCommand.HostIndex < 0 || m.activeCommand.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeCommand.HostIndex].Host
	entry := commandservice.SingleHistoryEntry(h, m.activeCommand.Name, m.activeCommand.Command, commandservice.Result{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}, time.Now())
	if err := commandservice.AppendHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func (m *Model) recordBatchCommandHistory() error {
	targets := make([]commandservice.BatchTarget, 0, len(m.batchJobs))
	for _, job := range m.batchJobs {
		if job.HostIndex < 0 || job.HostIndex >= len(m.states) {
			continue
		}
		targets = append(targets, commandservice.BatchTarget{Host: m.states[job.HostIndex].Host, Output: job.Output, ExitCode: job.ExitCode, Err: job.Err})
	}
	if len(targets) == 0 {
		return nil
	}
	entry := commandservice.BatchHistoryEntry(m.batchCommand.Name, m.batchCommand.Command, targets, time.Now())
	if err := commandservice.AppendHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func commandHistoryStatus(err error) string {
	return commandservice.Status(err)
}
