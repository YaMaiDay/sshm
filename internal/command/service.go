package command

import (
	"context"
	"time"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/execresult"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

type BatchTarget struct {
	Host     host.Host
	Output   string
	ExitCode int
	Err      error
}

type Result = execresult.Result

type Service struct{}

func (Service) Run(ctx context.Context, h host.Host, script remotescript.UserScript) Result {
	result, cleanup := actions.RemoteCommandContext(ctx, h, script.String())
	cleanup()
	return Result{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}

func Status(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}

func SingleHistoryEntry(h host.Host, name string, script string, result Result, at time.Time) config.CommandHistoryEntry {
	status := Status(result.Err)
	return config.NormalizeCommandHistoryEntry(config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(at),
		Time:     at.Format(time.RFC3339),
		Kind:     "single",
		Name:     name,
		Command:  script,
		Status:   status,
		ExitCode: result.ExitCode,
		Targets: []config.CommandHistoryTarget{
			config.CommandHistoryTargetFromHost(h, status, result.ExitCode, result.Output),
		},
	})
}

func BatchHistoryEntry(name string, script string, targets []BatchTarget, at time.Time) config.CommandHistoryEntry {
	historyTargets := make([]config.CommandHistoryTarget, 0, len(targets))
	failCount := 0
	for _, target := range targets {
		status := Status(target.Err)
		if target.Err != nil {
			failCount++
		}
		historyTargets = append(historyTargets, config.CommandHistoryTargetFromHost(target.Host, status, target.ExitCode, target.Output))
	}
	status := "success"
	if failCount > 0 {
		status = "failed"
	}
	return config.NormalizeCommandHistoryEntry(config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(at),
		Time:     at.Format(time.RFC3339),
		Kind:     "batch",
		Name:     name,
		Command:  script,
		Status:   status,
		ExitCode: failCount,
		Targets:  historyTargets,
	})
}
