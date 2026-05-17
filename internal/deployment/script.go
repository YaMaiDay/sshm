package deployment

import (
	"path/filepath"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

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
		appendDynamicReleaseUnpackShell(b, "$SSHM_RELEASE_ASSET", "$SSHM_RELEASE_PACKAGE", "\"$SSHM_RELEASE_TARGET\"", "\"$SSHM_RELEASE_TARGET/\"")
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
	appendReleaseUnpackShell(b, asset, version)
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

func shellSingleQuote(value string) string {
	return remotescript.SingleQuote(value)
}

func shellEnvName(value string) string {
	return remotescript.EnvName(value)
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
