package deployment

import (
	"context"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/execresult"
	"github.com/YaMaiDay/sshm/internal/host"
)

type Result struct {
	Command         CommandResult
	PreviousVersion string
	CurrentVersion  string
}

type CommandResult = execresult.Result

type Service struct{}

func (Service) Run(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) Result {
	var result CommandResult
	if app.FetchMode == config.DeployFetchLocal {
		result = runLocalFetchDeployment(ctx, h, app, onOutput)
	} else {
		script := BuildScript(app, false)
		actionResult, cleanup := actions.RemoteCommandStreamContext(ctx, h, script, onOutput)
		result = deploymentCommandResult(actionResult)
		cleanup()
	}
	prev, curr := ParseVersions(result.Output)
	return Result{Command: result, PreviousVersion: prev, CurrentVersion: curr}
}

func (Service) Rollback(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) CommandResult {
	result, cleanup := actions.RemoteCommandStreamContext(ctx, h, BuildScript(app, true), onOutput)
	cleanup()
	return deploymentCommandResult(result)
}

func BuildScript(app config.DeploymentApp, rollback bool) string {
	return buildDeploymentScript(app, rollback)
}
func BuildRemoteScript(app config.DeploymentApp, rollback bool, includeResource bool) string {
	return buildRemoteDeploymentScript(app, rollback, includeResource)
}
func RunLocalFetch(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) CommandResult {
	return runLocalFetchDeployment(ctx, h, app, onOutput)
}
func LocalEnv(app config.DeploymentApp) []string   { return deploymentLocalEnv(app) }
func ParseVersions(output string) (string, string) { return parseDeploymentVersions(output) }
func ResourcePreviewCommands(app config.DeploymentApp) []string {
	return deploymentResourcePreviewCommands(app)
}
func ResourceDefaultCommands(app config.DeploymentApp) []string {
	return deploymentResourceDefaultCommands(app)
}
func BuildLocalFetchPreScript(app config.DeploymentApp) string {
	return buildLocalFetchPreScript(app)
}
func BuildLocalFetchPostScript(app config.DeploymentApp) string {
	return buildLocalFetchPostScript(app)
}

func deploymentCommandResult(result actions.CommandResult) CommandResult {
	return CommandResult{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}
