package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

func (m Model) updateDeploymentConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; go back after it finishes or fails.", "部署执行中，完成或失败后再返回")
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentConfirmMaxScroll())
	case "enter":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running.", "部署执行中")
			return m, nil
		}
		if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.Output != "" {
			m.status = m.t("This deployment has already run. Press r to retry, or a to redeploy.", "当前部署已执行，按 r 重试，或按 a 重新部署")
			return m, nil
		}
		queue := m.deploymentConfirmQueue
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		for _, app := range queue {
			if m.deploymentServerIndex(app.Server) < 0 {
				m.status = m.t("Deployment server does not exist: ", "部署服务器不存在：") + emptyDash(app.Server)
				return m, nil
			}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueIndex = 0
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	case "r":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; cannot retry.", "部署执行中，不能重试")
			return m, nil
		}
		if len(m.activeDeployment.Queue) == 0 || m.activeDeployment.QueueFailed < 0 || m.activeDeployment.QueueFailed >= len(m.activeDeployment.Queue) {
			m.status = m.t("No failed item to retry.", "没有失败项可重试")
			return m, nil
		}
		return m.startQueuedDeployment(m.activeDeployment.QueueFailed)
	case "a":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; cannot redeploy.", "部署执行中，不能重新部署")
			return m, nil
		}
		queue := m.activeDeployment.Queue
		if len(queue) == 0 {
			queue = m.deploymentConfirmQueue
		}
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	}
	return m, nil
}

func (m Model) startQueuedDeployment(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.activeDeployment.Queue) {
		m.activeDeployment.Running = false
		m.status = m.t("Deployment queue completed.", "部署队列完成。")
		return m, nil
	}
	app := m.activeDeployment.Queue[index]
	hostIndex := m.deploymentServerIndex(app.Server)
	if hostIndex < 0 {
		m.status = m.t("Deployment server does not exist: ", "部署服务器不存在：") + emptyDash(app.Server)
		return m, nil
	}
	m.activeDeployment.HostIndex = hostIndex
	m.activeDeployment.App = app
	m.activeDeployment.Action = config.DeployActionDeploy
	m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
	m.activeDeployment.Output = ""
	m.activeDeployment.ExitCode = 0
	m.activeDeployment.Running = true
	m.activeDeployment.PreviousVersion = ""
	m.activeDeployment.CurrentVersion = ""
	m.activeDeployment.QueueIndex = index
	m.activeDeployment.QueueFailed = -1
	m.deploymentOutputScroll = 0
	m.mode = modeDeploymentConfirm
	if len(m.activeDeployment.Queue) > 1 {
		m.status = fmt.Sprintf(m.t("Deploying %d/%d: %s", "正在部署 %d/%d：%s"), index+1, len(m.activeDeployment.Queue), app.Name)
	} else {
		m.status = m.t("Deploying...", "正在部署...")
	}
	deploymentProgressStart(m.activeDeployment.ProgressID)
	return m, tea.Batch(m.runDeployment(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
}

func (m Model) startNextQueuedDeployment() (tea.Model, tea.Cmd) {
	next := m.activeDeployment.QueueIndex + 1
	return m.startQueuedDeployment(next)
}

func deploymentQueueNextAfter(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		return deploymentQueueNextMsg{}
	}
}

func (m Model) updateDeploymentOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; go back after it finishes.", "部署执行中，完成后再返回")
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentOutputMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentOutputMaxScroll())
	case "r":
		if m.activeDeployment.Running {
			m.status = m.t("Deployment is running; rollback after it finishes.", "部署执行中，完成后再回滚")
			return m, nil
		}
		if len(m.activeDeployment.App.RollbackCommands) == 0 {
			m.status = m.t("No rollback commands configured.", "没有配置回滚命令")
			return m, nil
		}
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentRollbackConfirm
		m.status = m.t("Confirm Rollback", "确认回滚")
		return m, nil
	}
	return m, nil
}

func (m Model) updateDeploymentRollbackConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDeploymentOutput
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "enter":
		m.activeDeployment.Running = true
		m.activeDeployment.Action = config.DeployActionRollback
		m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
		m.activeDeployment.Output = ""
		m.activeDeployment.ExitCode = 0
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentOutput
		m.status = m.t("Running rollback...", "正在执行回滚...")
		deploymentProgressStart(m.activeDeployment.ProgressID)
		return m, tea.Batch(m.runDeploymentRollback(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
	}
	return m, nil
}

func (m Model) runDeployment() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := actions.CommandResult{Err: fmt.Errorf("%s%s", m.t("Deployment server does not exist: ", "部署服务器不存在："), emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	onOutput := func(text string) { deploymentProgressAppend(progressID, text) }
	if app.FetchMode == config.DeployFetchLocal {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
			defer cancel()
			result := m.runLocalFetchDeployment(ctx, h, app, onOutput)
			prev, curr := parseDeploymentVersions(result.Output)
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result, PreviousVersion: prev, CurrentVersion: curr}
		}
	}
	script := buildDeploymentScript(app, false)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result, cleanup := actions.RemoteCommandStreamContext(ctx, h, script, onOutput)
		cleanup()
		prev, curr := parseDeploymentVersions(result.Output)
		deploymentProgressFinish(progressID, result.Output)
		return deploymentDoneMsg{ID: progressID, Result: result, PreviousVersion: prev, CurrentVersion: curr}
	}
}

func (m Model) runDeploymentRollback() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := actions.CommandResult{Err: fmt.Errorf("%s%s", m.t("Deployment server does not exist: ", "部署服务器不存在："), emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	script := buildDeploymentScript(app, true)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		result, cleanup := actions.RemoteCommandStreamContext(ctx, h, script, func(text string) { deploymentProgressAppend(progressID, text) })
		cleanup()
		deploymentProgressFinish(progressID, result.Output)
		return deploymentDoneMsg{ID: progressID, Result: result}
	}
}

func (m Model) handleDeploymentDone(msg deploymentDoneMsg) (tea.Model, tea.Cmd) {
	if msg.ID != "" && msg.ID != m.activeDeployment.ProgressID {
		return m, nil
	}
	m.activeDeployment.Running = false
	m.activeDeployment.Output = msg.Result.Output
	m.activeDeployment.ExitCode = msg.Result.ExitCode
	m.activeDeployment.PreviousVersion = msg.PreviousVersion
	m.activeDeployment.CurrentVersion = msg.CurrentVersion
	failed := msg.Result.Err != nil
	if failed {
		m.status = fmt.Sprintf("%s %d", m.t("Deployment failed: exit", "部署失败：退出码"), msg.Result.ExitCode)
	} else {
		m.status = m.t("Deployment completed.", "部署完成。")
	}
	if err := m.recordDeployment(msg.Result); err != nil {
		m.status += m.t(" Record save failed: ", " 记录保存失败：") + err.Error()
	}
	if msg.ID != "" {
		deploymentProgressClear(msg.ID)
	}
	if m.activeDeployment.Action == config.DeployActionDeploy && len(m.activeDeployment.Queue) > 0 {
		if failed {
			m.activeDeployment.QueueFailed = m.activeDeployment.QueueIndex
			if len(m.activeDeployment.Queue) > 1 {
				m.status = fmt.Sprintf(m.t("Deployment queue stopped: app %d failed. Press r to retry failed item, a to redeploy.", "部署队列停止：第 %d 个应用失败，按 r 重试失败项，按 a 重新部署"), m.activeDeployment.QueueIndex+1)
			} else {
				m.status = m.t("Deployment failed. Press r to retry, a to redeploy.", "部署失败，按 r 重试，按 a 重新部署")
			}
			return m, nil
		}
		next := m.activeDeployment.QueueIndex + 1
		if next < len(m.activeDeployment.Queue) {
			wait := maxInt(0, m.activeDeployment.App.WaitSeconds)
			m.status = fmt.Sprintf(m.t("Deployment completed. Waiting %d seconds before next: %s", "部署完成，等待 %d 秒后执行下一个：%s"), wait, m.activeDeployment.Queue[next].Name)
			return m, deploymentQueueNextAfter(time.Duration(wait) * time.Second)
		}
		if len(m.activeDeployment.Queue) > 1 {
			m.status = m.t("Deployment queue completed.", "部署队列完成。")
		}
	}
	return m, nil
}

func (m Model) handleDeploymentProgress(msg deploymentProgressMsg) (tea.Model, tea.Cmd) {
	if msg.ID == "" || msg.ID != m.activeDeployment.ProgressID || !m.activeDeployment.Running {
		return m, nil
	}
	m.activeDeployment.Output = msg.Output
	if msg.Done {
		return m, nil
	}
	return m, deploymentProgressAfter(msg.ID, 300*time.Millisecond)
}

func (m *Model) recordDeployment(result actions.CommandResult) error {
	if m.activeDeployment.HostIndex < 0 || m.activeDeployment.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeDeployment.HostIndex].Host
	status := config.DeployStatusSuccess
	errText := ""
	if result.Err != nil {
		status = config.DeployStatusFailed
		errText = result.Err.Error()
	}
	record := config.DeploymentRecord{
		ID:              config.NewDeploymentID(time.Now()),
		Time:            time.Now().Format(time.RFC3339),
		App:             m.activeDeployment.App.Name,
		ServerCategory:  h.Category,
		ServerName:      h.Name,
		Action:          emptyChoice(m.activeDeployment.Action, config.DeployActionDeploy),
		Source:          m.activeDeployment.App.Source,
		Status:          status,
		PreviousVersion: m.activeDeployment.PreviousVersion,
		CurrentVersion:  m.activeDeployment.CurrentVersion,
		ExitCode:        result.ExitCode,
		Output:          result.Output,
		Error:           errText,
	}
	if err := config.AppendDeploymentRecord(m.home, record); err != nil {
		return err
	}
	file, _, err := config.LoadDeployments(m.home)
	if err == nil {
		m.deploymentFile = file
	}
	return nil
}

func buildDeploymentScript(app config.DeploymentApp, rollback bool) string {
	return buildRemoteDeploymentScript(app, rollback, true)
}

func buildRemoteDeploymentScript(app config.DeploymentApp, rollback bool, includeResource bool) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentCredentialScript(&b, app)
	b.WriteString("mkdir -p " + shellSingleQuote(app.Path) + "\n")
	if rollback {
		appendDeploymentCommands(&b, app.Path, "回滚", app.RollbackCommands)
		return b.String()
	}
	appendDeploymentCommands(&b, app.Path, "更新前", app.BeforeCommands)
	if includeResource && len(app.ResourceCommands) > 0 {
		appendDeploymentCommands(&b, app.Path, "获取资源", app.ResourceCommands)
	} else if includeResource {
		appendDeploymentStageTitle(&b, "获取资源")
		switch app.Source {
		case config.DeploySourceRelease:
			appendReleaseDeploymentScript(&b, app)
		default:
			appendGitDeploymentScript(&b, app)
		}
	}
	appendDeploymentCommands(&b, app.Path, "更新", app.UpdateCommands)
	appendDeploymentCommands(&b, app.Path, "更新后", app.AfterCommands)
	appendDeploymentCommands(&b, app.Path, "健康检查", app.HealthCommands)
	return b.String()
}

func (m Model) runLocalFetchDeployment(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) actions.CommandResult {
	var output strings.Builder
	pre := buildLocalFetchPreScript(app)
	preResult, cleanup := actions.RemoteCommandStreamContext(ctx, h, pre, onOutput)
	cleanup()
	output.WriteString(preResult.Output)
	if preResult.Err != nil {
		preResult.Output = output.String()
		return preResult
	}
	tmp, err := os.MkdirTemp("", "sshm-deploy-*")
	if err != nil {
		return actions.CommandResult{Output: output.String(), Err: err, ExitCode: -1}
	}
	defer os.RemoveAll(tmp)
	localResult := localFetchDeploymentResource(ctx, app, tmp, onOutput)
	output.WriteString(localResult.Output)
	if localResult.Err != nil {
		localResult.Output = output.String()
		return localResult
	}
	cmd, rsyncCleanup := actions.RsyncUploadCommandContext(ctx, h, localResultPath(tmp)+string(os.PathSeparator), app.Path)
	uploadTitle := "== " + m.t("Upload resource", "上传资源") + " ==\n"
	output.WriteString(uploadTitle)
	if onOutput != nil {
		onOutput(uploadTitle)
	}
	rsyncResult := actions.RunCommandStream(cmd, onOutput)
	rsyncCleanup()
	output.WriteString(rsyncResult.Output)
	if rsyncResult.Err != nil {
		return actions.CommandResult{Output: output.String(), Err: rsyncResult.Err, ExitCode: rsyncResult.ExitCode}
	}
	post := buildLocalFetchPostScript(app)
	postResult, postCleanup := actions.RemoteCommandStreamContext(ctx, h, post, onOutput)
	postCleanup()
	output.WriteString(postResult.Output)
	postResult.Output = output.String()
	return postResult
}

func buildLocalFetchPreScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	b.WriteString("mkdir -p " + shellSingleQuote(app.Path) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新前", app.BeforeCommands)
	if app.Source == config.DeploySourceGit {
		b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	} else {
		b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(readlink current 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	}
	return b.String()
}

func buildLocalFetchPostScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新", app.UpdateCommands)
	appendDeploymentCommands(&b, app.Path, "更新后", app.AfterCommands)
	appendDeploymentCommands(&b, app.Path, "健康检查", app.HealthCommands)
	return b.String()
}

func localResultPath(tmp string) string {
	return filepath.Join(tmp, "payload")
}

func localFetchDeploymentResource(ctx context.Context, app config.DeploymentApp, tmp string, onOutput func(string)) actions.CommandResult {
	payload := localResultPath(tmp)
	if err := os.MkdirAll(payload, 0700); err != nil {
		return actions.CommandResult{Err: err, ExitCode: -1}
	}
	if len(app.ResourceCommands) > 0 {
		return localFetchCustomResource(ctx, app, payload, onOutput)
	}
	if app.Source == config.DeploySourceRelease {
		return localFetchReleaseResource(ctx, app, payload, onOutput)
	}
	return localFetchGitResource(ctx, app, payload, onOutput)
}

func localFetchCustomResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(payload) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + shellSingleQuote(payload) + "\n")
	for _, command := range app.ResourceCommands {
		if strings.TrimSpace(command) != "" {
			b.WriteString(command + "\n")
		}
	}
	cmd := localShellCommand(ctx, b.String())
	cmd.Env = deploymentLocalEnv(app)
	return actions.RunCommandStream(cmd, onOutput)
}

func localFetchGitResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
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
		return result
	}
	versionCmd := exec.CommandContext(ctx, "git", "-C", payload, "rev-parse", "--short", "HEAD")
	versionOutput, versionErr := versionCmd.CombinedOutput()
	if versionErr == nil {
		result.Output += "SSHM_CURRENT_VERSION=" + strings.TrimSpace(string(versionOutput)) + "\n"
	}
	return result
}

func localFetchReleaseResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
	script := buildLocalReleaseScript(app, payload)
	cmd := localShellCommand(ctx, script)
	cmd.Env = deploymentLocalEnv(app)
	return actions.RunCommandStream(cmd, onOutput)
}

func buildLocalReleaseScript(app config.DeploymentApp, payload string) string {
	var b strings.Builder
	url, version, asset := deploymentReleaseValues(app)
	b.WriteString("set -eu\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + shellSingleQuote(payload) + "\n")
	b.WriteString("mkdir -p packages " + shellSingleQuote("releases/"+version) + "\n")
	if deploymentAssetIsPattern(asset) && strings.TrimSpace(app.ReleaseURL) == "" {
		apiURL := deploymentReleaseAPIURL(app.Repo, version)
		b.WriteString("SSHM_RELEASE_JSON=$(curl -fsL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + shellSingleQuote(apiURL) + ")\n")
		b.WriteString("SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in " + shellCasePattern(asset) + ") printf '%s\\n' \"$url\"; break ;; esac; done)\n")
		b.WriteString("if [ -z \"$SSHM_RELEASE_URL\" ]; then echo " + shellSingleQuote("未找到匹配的 Release 资源："+asset) + "; exit 1; fi\n")
		b.WriteString("SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}\n")
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("case \"$SSHM_RELEASE_ASSET\" in\n")
		b.WriteString("  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C " + shellSingleQuote("releases/"+version) + " ;;\n")
		b.WriteString("  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d " + shellSingleQuote("releases/"+version) + " ;;\n")
		b.WriteString("  *) cp \"$SSHM_RELEASE_PACKAGE\" " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
		b.WriteString("esac\n")
	} else {
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
		appendReleaseUnpackShell(&b, asset, version)
	}
	b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
	return b.String()
}

func appendReleaseUnpackShell(b *strings.Builder, asset string, version string) {
	b.WriteString("case " + shellSingleQuote(asset) + " in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf " + shellSingleQuote("packages/"+asset) + " -C " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *.zip) unzip -o " + shellSingleQuote("packages/"+asset) + " -d " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *) cp " + shellSingleQuote("packages/"+asset) + " " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
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
		tokenVar := shellEnvName(name)
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

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func appendGitDeploymentScript(b *strings.Builder, app config.DeploymentApp) {
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	parent := filepath.Dir(strings.TrimRight(app.Path, "/"))
	b.WriteString("echo '== 获取 Git 代码 =='\n")
	b.WriteString("if [ ! -d " + shellSingleQuote(app.Path+"/.git") + " ]; then\n")
	b.WriteString("  mkdir -p " + shellSingleQuote(parent) + "\n")
	b.WriteString("  git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("fi\n")
	b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	b.WriteString("git fetch --all --prune\n")
	b.WriteString("git checkout " + shellSingleQuote(branch) + "\n")
	b.WriteString("git pull --ff-only\n")
	b.WriteString("SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
}

func appendReleaseDeploymentScript(b *strings.Builder, app config.DeploymentApp) {
	url, version, asset := deploymentReleaseValues(app)
	assetIsPattern := deploymentAssetIsPattern(asset)
	b.WriteString("echo '== 获取 Release 资源 =='\n")
	b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("SSHM_PREVIOUS_VERSION=$(readlink current 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	b.WriteString("mkdir -p packages " + shellSingleQuote("releases/"+version) + "\n")
	if assetIsPattern && strings.TrimSpace(app.ReleaseURL) == "" {
		apiURL := deploymentReleaseAPIURL(app.Repo, version)
		b.WriteString("SSHM_RELEASE_API=" + shellSingleQuote(apiURL) + "\n")
		b.WriteString("if [ -n \"${SSHM_GITHUB_AUTH_HEADER:-}\" ]; then\n")
		b.WriteString("  SSHM_RELEASE_JSON=$(curl -fsL -H \"$SSHM_GITHUB_AUTH_HEADER\" \"$SSHM_RELEASE_API\")\n")
		b.WriteString("else\n")
		b.WriteString("  SSHM_RELEASE_JSON=$(curl -fsL \"$SSHM_RELEASE_API\")\n")
		b.WriteString("fi\n")
		b.WriteString("SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in " + shellCasePattern(asset) + ") printf '%s\\n' \"$url\"; break ;; esac; done)\n")
		b.WriteString("if [ -z \"$SSHM_RELEASE_URL\" ]; then echo " + shellSingleQuote("未找到匹配的 Release 资源："+asset) + "; exit 1; fi\n")
		b.WriteString("SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}\n")
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_TARGET=" + shellSingleQuote("releases/"+version) + "\n")
		b.WriteString("case \"$SSHM_RELEASE_ASSET\" in\n")
		b.WriteString("  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C \"$SSHM_RELEASE_TARGET\" ;;\n")
		b.WriteString("  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d \"$SSHM_RELEASE_TARGET\" ;;\n")
		b.WriteString("  *) cp \"$SSHM_RELEASE_PACKAGE\" \"$SSHM_RELEASE_TARGET/\" ;;\n")
		b.WriteString("esac\n")
		b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
		b.WriteString("SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
		b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
		return
	}
	b.WriteString("if [ -n \"${SSHM_GITHUB_AUTH_HEADER:-}\" ]; then\n")
	b.WriteString("  curl -fL -H \"$SSHM_GITHUB_AUTH_HEADER\" " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
	b.WriteString("else\n")
	b.WriteString("  curl -fL " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
	b.WriteString("fi\n")
	b.WriteString("case " + shellSingleQuote(asset) + " in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf " + shellSingleQuote("packages/"+asset) + " -C " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *.zip) unzip -o " + shellSingleQuote("packages/"+asset) + " -d " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *) cp " + shellSingleQuote("packages/"+asset) + " " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
	b.WriteString("esac\n")
	b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
	b.WriteString("SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
}

func deploymentReleaseValues(app config.DeploymentApp) (string, string, string) {
	url := strings.TrimSpace(app.ReleaseURL)
	version := strings.TrimSpace(app.Version)
	if version == "" {
		version = "latest"
	}
	asset := strings.TrimSpace(app.Asset)
	if asset == "" {
		asset = filepath.Base(url)
	}
	if url == "" {
		repo := strings.Trim(strings.TrimSpace(app.Repo), "/")
		if version == "latest" {
			url = "https://github.com/" + repo + "/releases/latest/download/" + asset
		} else {
			url = "https://github.com/" + repo + "/releases/download/" + version + "/" + asset
		}
	}
	return url, version, asset
}

func deploymentReleaseAPIURL(repo string, version string) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	if strings.TrimSpace(version) == "" || version == "latest" {
		return "https://api.github.com/repos/" + repo + "/releases/latest"
	}
	return "https://api.github.com/repos/" + repo + "/releases/tags/" + version
}

func deploymentAssetIsPattern(asset string) bool {
	return strings.Contains(asset, "*")
}

func shellCasePattern(value string) string {
	if value == "" {
		return "''"
	}
	var b strings.Builder
	var literal strings.Builder
	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		b.WriteString(shellSingleQuote(literal.String()))
		literal.Reset()
	}
	for _, r := range value {
		if r == '*' {
			flushLiteral()
			b.WriteRune('*')
			continue
		}
		literal.WriteRune(r)
	}
	flushLiteral()
	if b.Len() == 0 {
		return "''"
	}
	return b.String()
}

func deploymentResourcePreviewCommands(app config.DeploymentApp) []string {
	return deploymentResourceDefaultCommands(app)
}

func deploymentResourceDefaultCommands(app config.DeploymentApp) []string {
	localFetch := app.FetchMode == config.DeployFetchLocal
	if app.Source == config.DeploySourceRelease {
		url, version, asset := deploymentReleaseValues(app)
		commands := []string{}
		if !localFetch {
			commands = append(commands, "cd "+shellSingleQuote(app.Path))
		}
		commands = append(commands, "mkdir -p packages "+shellSingleQuote("releases/"+version))
		if deploymentAssetIsPattern(asset) && strings.TrimSpace(app.ReleaseURL) == "" {
			commands = append(commands,
				"SSHM_RELEASE_JSON=$(curl -fsL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} "+shellSingleQuote(deploymentReleaseAPIURL(app.Repo, version))+")",
				"SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in "+shellCasePattern(asset)+") printf '%s\\n' \"$url\"; break ;; esac; done)",
				"if [ -z \"$SSHM_RELEASE_URL\" ]; then echo "+shellSingleQuote("未找到匹配的 Release 资源："+asset)+"; exit 1; fi",
				"SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}",
				"curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"",
				"SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"",
			)
			commands = appendReleaseDynamicUnpackPreview(commands, version)
			return append(commands, "ln -sfn "+shellSingleQuote("releases/"+version)+" current")
		}
		commands = append(commands, "curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} "+shellSingleQuote(url)+" -o "+shellSingleQuote("packages/"+asset))
		commands = appendReleaseUnpackPreview(commands, asset, version)
		commands = append(commands, "ln -sfn "+shellSingleQuote("releases/"+version)+" current")
		return commands
	}
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	if localFetch {
		return []string{
			"git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " .",
			"SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
			"echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION",
		}
	}
	parent := filepath.Dir(strings.TrimRight(app.Path, "/"))
	return []string{
		"if [ ! -d " + shellSingleQuote(app.Path+"/.git") + " ]; then mkdir -p " + shellSingleQuote(parent) + " && git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " " + shellSingleQuote(app.Path) + "; fi",
		"cd " + shellSingleQuote(app.Path),
		"SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
		"git fetch --all --prune",
		"git checkout " + shellSingleQuote(branch),
		"git pull --ff-only",
		"SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
	}
}

func appendReleaseDynamicUnpackPreview(commands []string, version string) []string {
	return append(commands,
		"case \"$SSHM_RELEASE_ASSET\" in",
		"  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C "+shellSingleQuote("releases/"+version)+" ;;",
		"  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d "+shellSingleQuote("releases/"+version)+" ;;",
		"  *) cp \"$SSHM_RELEASE_PACKAGE\" "+shellSingleQuote("releases/"+version+"/")+" ;;",
		"esac",
	)
}

func appendReleaseUnpackPreview(commands []string, asset string, version string) []string {
	switch {
	case strings.HasSuffix(asset, ".tar.gz") || strings.HasSuffix(asset, ".tgz"):
		return append(commands, "tar -xzf "+shellSingleQuote("packages/"+asset)+" -C "+shellSingleQuote("releases/"+version))
	case strings.HasSuffix(asset, ".zip"):
		return append(commands, "unzip -o "+shellSingleQuote("packages/"+asset)+" -d "+shellSingleQuote("releases/"+version))
	default:
		return append(commands, "cp "+shellSingleQuote("packages/"+asset)+" "+shellSingleQuote("releases/"+version+"/"))
	}
}

func appendDeploymentCredentialScript(b *strings.Builder, app config.DeploymentApp) {
	name := strings.TrimSpace(app.CredentialName)
	switch app.Credential {
	case config.DeployCredentialSSH:
		if name == "" {
			return
		}
		gitSSHCommand := "ssh -i " + shellSingleQuote(name) + " -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
		b.WriteString("export GIT_SSH_COMMAND=" + shellSingleQuote(gitSSHCommand) + "\n")
	case config.DeployCredentialToken:
		tokenVar := shellEnvName(name)
		if tokenVar == "" {
			tokenVar = "GITHUB_TOKEN"
		}
		b.WriteString("SSHM_GITHUB_AUTH_HEADER=\n")
		b.WriteString("if [ -n \"${" + tokenVar + ":-}\" ]; then\n")
		b.WriteString("  SSHM_GITHUB_AUTH_HEADER=\"Authorization: Bearer ${" + tokenVar + "}\"\n")
		b.WriteString("fi\n")
	}
}

func appendDeploymentCommands(b *strings.Builder, path string, title string, commands []string) {
	if len(commands) == 0 {
		return
	}
	appendDeploymentStageTitle(b, title)
	b.WriteString("cd " + shellSingleQuote(path) + "\n")
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			b.WriteString(command + "\n")
		}
	}
}

func appendDeploymentStageTitle(b *strings.Builder, title string) {
	b.WriteString("echo " + shellSingleQuote("== "+title+" ==") + "\n")
}

func deploymentStageOutput(title string, output string) string {
	if strings.TrimSpace(output) == "" {
		return "== " + title + " ==\n"
	}
	return "== " + title + " ==\n" + output
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellEnvName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for i, r := range value {
		if i == 0 {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return ""
			}
			continue
		}
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return value
}

func parseDeploymentVersions(output string) (string, string) {
	prev := ""
	curr := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SSHM_PREVIOUS_VERSION=") {
			prev = strings.TrimPrefix(line, "SSHM_PREVIOUS_VERSION=")
		}
		if strings.HasPrefix(line, "SSHM_CURRENT_VERSION=") {
			curr = strings.TrimPrefix(line, "SSHM_CURRENT_VERSION=")
		}
	}
	return prev, curr
}
