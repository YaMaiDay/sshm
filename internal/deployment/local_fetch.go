package deployment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

func runLocalFetchDeployment(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) CommandResult {
	var output strings.Builder
	pre := buildLocalFetchPreScript(app)
	preResult, cleanup := actions.RemoteCommandStreamContext(ctx, h, pre, onOutput)
	cleanup()
	output.WriteString(preResult.Output)
	if preResult.Err != nil {
		preResult.Output = output.String()
		return deploymentCommandResult(preResult)
	}
	tmp, err := os.MkdirTemp("", "sshm-deploy-*")
	if err != nil {
		return CommandResult{Output: output.String(), Err: err, ExitCode: -1}
	}
	defer os.RemoveAll(tmp)
	localResult := localFetchDeploymentResource(ctx, app, tmp, onOutput)
	output.WriteString(localResult.Output)
	if localResult.Err != nil {
		localResult.Output = output.String()
		return localResult
	}
	cmd, rsyncCleanup := actions.RsyncUploadCommandContext(ctx, h, localResultPath(tmp)+string(os.PathSeparator), app.Path)
	uploadTitle := "== 上传资源 ==\n"
	output.WriteString(uploadTitle)
	if onOutput != nil {
		onOutput(uploadTitle)
	}
	rsyncResult := actions.RunCommandStream(cmd, onOutput)
	rsyncCleanup()
	output.WriteString(rsyncResult.Output)
	if rsyncResult.Err != nil {
		return CommandResult{Output: output.String(), Err: rsyncResult.Err, ExitCode: rsyncResult.ExitCode}
	}
	post := buildLocalFetchPostScript(app)
	postResult, postCleanup := actions.RemoteCommandStreamContext(ctx, h, post, onOutput)
	postCleanup()
	output.WriteString(postResult.Output)
	postResult.Output = output.String()
	return deploymentCommandResult(postResult)
}

func buildLocalFetchPreScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + remotescript.SingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + remotescript.SingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + remotescript.SingleQuote(app.Source) + "\n")
	b.WriteString("mkdir -p " + remotescript.SingleQuote(app.Path) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新前", app.BeforeCommands)
	if app.Source == config.DeploySourceGit {
		b.WriteString("cd " + remotescript.SingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	} else {
		b.WriteString("cd " + remotescript.SingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(readlink current 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	}
	return b.String()
}

func buildLocalFetchPostScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + remotescript.SingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + remotescript.SingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + remotescript.SingleQuote(app.Source) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新", app.UpdateCommands)
	appendDeploymentCommands(&b, app.Path, "更新后", app.AfterCommands)
	appendDeploymentCommands(&b, app.Path, "健康检查", app.HealthCommands)
	return b.String()
}

func localResultPath(tmp string) string {
	return filepath.Join(tmp, "payload")
}

func localFetchDeploymentResource(ctx context.Context, app config.DeploymentApp, tmp string, onOutput func(string)) CommandResult {
	payload := localResultPath(tmp)
	if err := os.MkdirAll(payload, 0700); err != nil {
		return CommandResult{Err: err, ExitCode: -1}
	}
	if len(app.ResourceCommands) > 0 {
		return localFetchCustomResource(ctx, app, payload, onOutput)
	}
	if app.Source == config.DeploySourceRelease {
		return localFetchReleaseResource(ctx, app, payload, onOutput)
	}
	return localFetchGitResource(ctx, app, payload, onOutput)
}

func localFetchCustomResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) CommandResult {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + remotescript.SingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + remotescript.SingleQuote(payload) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + remotescript.SingleQuote(app.Source) + "\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + remotescript.SingleQuote(payload) + "\n")
	for _, command := range app.ResourceCommands {
		if strings.TrimSpace(command) != "" {
			b.WriteString(command + "\n")
		}
	}
	cmd := localShellCommand(ctx, b.String())
	cmd.Env = deploymentLocalEnv(app)
	return deploymentCommandResult(actions.RunCommandStream(cmd, onOutput))
}

func localFetchGitResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) CommandResult {
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	args := []string{"clone", "--branch", branch, app.Repo, payload}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = deploymentLocalEnv(app)
	var output strings.Builder
	stage := "== 获取资源 ==\n"
	output.WriteString(stage)
	if onOutput != nil {
		onOutput(stage)
	}
	result := actions.RunCommandStream(cmd, onOutput)
	output.WriteString(result.Output)
	result.Output = output.String()
	if result.Err != nil {
		return deploymentCommandResult(result)
	}
	versionCmd := exec.CommandContext(ctx, "git", "-C", payload, "rev-parse", "--short", "HEAD")
	versionOutput, versionErr := versionCmd.CombinedOutput()
	if versionErr == nil {
		result.Output += "SSHM_CURRENT_VERSION=" + strings.TrimSpace(string(versionOutput)) + "\n"
	}
	return deploymentCommandResult(result)
}

func localFetchReleaseResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) CommandResult {
	script := buildLocalReleaseScript(app, payload)
	cmd := localShellCommand(ctx, script)
	cmd.Env = deploymentLocalEnv(app)
	return deploymentCommandResult(actions.RunCommandStream(cmd, onOutput))
}

func buildLocalReleaseScript(app config.DeploymentApp, payload string) string {
	var b strings.Builder
	url, version, asset := deploymentReleaseValues(app)
	b.WriteString("set -eu\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + remotescript.SingleQuote(payload) + "\n")
	b.WriteString("mkdir -p packages " + remotescript.SingleQuote("releases/"+version) + "\n")
	if deploymentAssetIsPattern(asset) && strings.TrimSpace(app.ReleaseURL) == "" {
		apiURL := deploymentReleaseAPIURL(app.Repo, version)
		b.WriteString("SSHM_RELEASE_JSON=$(curl -fsL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + remotescript.SingleQuote(apiURL) + ")\n")
		b.WriteString("SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in " + shellCasePattern(asset) + ") printf '%s\\n' \"$url\"; break ;; esac; done)\n")
		b.WriteString("if [ -z \"$SSHM_RELEASE_URL\" ]; then echo " + remotescript.SingleQuote("未找到匹配的 Release 资源："+asset) + "; exit 1; fi\n")
		b.WriteString("SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}\n")
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"\n")
		appendDynamicReleaseUnpackShell(&b, "$SSHM_RELEASE_ASSET", "$SSHM_RELEASE_PACKAGE", remotescript.SingleQuote("releases/"+version), remotescript.SingleQuote("releases/"+version+"/"))
	} else {
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + remotescript.SingleQuote(url) + " -o " + remotescript.SingleQuote("packages/"+asset) + "\n")
		appendReleaseUnpackShell(&b, asset, version)
	}
	b.WriteString("ln -sfn " + remotescript.SingleQuote("releases/"+version) + " current\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=" + remotescript.SingleQuote(version) + "\n")
	return b.String()
}

func appendReleaseUnpackShell(b *strings.Builder, asset string, version string) {
	b.WriteString("case " + remotescript.SingleQuote(asset) + " in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf " + remotescript.SingleQuote("packages/"+asset) + " -C " + remotescript.SingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *.zip) unzip -o " + remotescript.SingleQuote("packages/"+asset) + " -d " + remotescript.SingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *) cp " + remotescript.SingleQuote("packages/"+asset) + " " + remotescript.SingleQuote("releases/"+version+"/") + " ;;\n")
	b.WriteString("esac\n")
}

func appendDynamicReleaseUnpackShell(b *strings.Builder, assetExpr string, packageExpr string, targetExpr string, copyTargetExpr string) {
	b.WriteString("case \"" + assetExpr + "\" in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf \"" + packageExpr + "\" -C " + targetExpr + " ;;\n")
	b.WriteString("  *.zip) unzip -o \"" + packageExpr + "\" -d " + targetExpr + " ;;\n")
	b.WriteString("  *) cp \"" + packageExpr + "\" " + copyTargetExpr + " ;;\n")
	b.WriteString("esac\n")
}

func localShellCommand(ctx context.Context, script string) *exec.Cmd {
	name := "sh"
	args := []string{"-s"}
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/C", script}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if runtime.GOOS != "windows" {
		cmd.Stdin = strings.NewReader(script)
	}
	return cmd
}

func deploymentLocalEnv(app config.DeploymentApp) []string {
	env := os.Environ()
	name := strings.TrimSpace(app.CredentialName)
	switch app.Credential {
	case config.DeployCredentialSSH:
		if name != "" {
			env = append(env, "GIT_SSH_COMMAND=ssh -i "+name+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new")
		}
	case config.DeployCredentialToken:
		tokenVar := remotescript.EnvName(name)
		if tokenVar == "" {
			tokenVar = "GITHUB_TOKEN"
		}
		token := os.Getenv(tokenVar)
		if token != "" {
			env = append(env, "SSHM_GITHUB_AUTH_HEADER=Authorization: Bearer "+token)
		}
	}
	return env
}
