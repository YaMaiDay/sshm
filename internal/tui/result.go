package tui

import (
	commandservice "github.com/YaMaiDay/sshm/internal/command"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func commandResultFromCommand(result commandservice.Result) commandResult {
	return commandResult{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}

func commandResultFromDeployment(result deploymentservice.CommandResult) commandResult {
	return commandResult{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}

func commandResultFromResource(result resourceservice.CommandResult) commandResult {
	return commandResult{Output: result.Output, Err: result.Err, ExitCode: result.ExitCode}
}
