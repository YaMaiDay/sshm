package deployment

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestBuildScriptRollbackOnlyRunsRollbackCommands(t *testing.T) {
	script := BuildScript(config.DeploymentApp{
		Name:             "api",
		Source:           config.DeploySourceGit,
		Repo:             "git@github.com:owner/api.git",
		Branch:           "main",
		Path:             "/data/api",
		UpdateCommands:   []string{"make deploy"},
		HealthCommands:   []string{"curl -fsS http://127.0.0.1:8080/health"},
		RollbackCommands: []string{"ln -sfn releases/old current", "systemctl restart api"},
	}, true)

	for _, want := range []string{"== 回滚 ==", "cd '/data/api'", "ln -sfn releases/old current", "systemctl restart api"} {
		if !strings.Contains(script, want) {
			t.Fatalf("rollback script missing %q:\n%s", want, script)
		}
	}
	for _, notWant := range []string{"== 获取资源 ==", "make deploy", "curl -fsS"} {
		if strings.Contains(script, notWant) {
			t.Fatalf("rollback script should not include %q:\n%s", notWant, script)
		}
	}
}

func TestBuildScriptGitIncludesPipeline(t *testing.T) {
	script := BuildScript(config.DeploymentApp{
		Name:           "api",
		Source:         config.DeploySourceGit,
		Repo:           "git@github.com:owner/api.git",
		Branch:         "main",
		Path:           "/data/api",
		Credential:     config.DeployCredentialSSH,
		CredentialName: "/home/deploy/.ssh/api_deploy_key",
		BeforeCommands: []string{"systemctl stop api"},
		UpdateCommands: []string{"go build ./cmd/api"},
		AfterCommands:  []string{"systemctl restart api"},
		HealthCommands: []string{"curl -fsS http://127.0.0.1:8080/health"},
	}, false)

	for _, want := range []string{
		"== 更新前 ==",
		"== 获取资源 ==",
		"== 更新 ==",
		"== 更新后 ==",
		"== 健康检查 ==",
		"export GIT_SSH_COMMAND=",
		"/home/deploy/.ssh/api_deploy_key",
		"IdentitiesOnly=yes",
		"git clone --branch 'main' 'git@github.com:owner/api.git' '/data/api'",
		"git pull --ff-only",
		"systemctl stop api",
		"go build ./cmd/api",
		"systemctl restart api",
		"curl -fsS http://127.0.0.1:8080/health",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildScriptReleaseIncludesDownloadAndUnpack(t *testing.T) {
	script := BuildScript(config.DeploymentApp{
		Name:           "web",
		Source:         config.DeploySourceRelease,
		Repo:           "owner/web",
		Version:        "v1.2.3",
		Asset:          "web.tar.gz",
		Path:           "/data/web",
		Credential:     config.DeployCredentialToken,
		CredentialName: "GH_RELEASE_TOKEN",
	}, false)

	for _, want := range []string{
		"if [ -n \"${GH_RELEASE_TOKEN:-}\" ]; then",
		"SSHM_GITHUB_AUTH_HEADER=\"Authorization: Bearer ${GH_RELEASE_TOKEN}\"",
		"curl -fL -H \"$SSHM_GITHUB_AUTH_HEADER\" 'https://github.com/owner/web/releases/download/v1.2.3/web.tar.gz'",
		"tar -xzf 'packages/web.tar.gz' -C 'releases/v1.2.3'",
		"ln -sfn 'releases/v1.2.3' current",
		"SSHM_CURRENT_VERSION='v1.2.3'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildScriptReleaseLatestFixedAsset(t *testing.T) {
	script := BuildScript(config.DeploymentApp{
		Name:   "web",
		Source: config.DeploySourceRelease,
		Repo:   "owner/web",
		Asset:  "web.tar.gz",
		Path:   "/data/web",
	}, false)

	for _, want := range []string{
		"curl -fL 'https://github.com/owner/web/releases/latest/download/web.tar.gz'",
		"tar -xzf 'packages/web.tar.gz' -C 'releases/latest'",
		"ln -sfn 'releases/latest' current",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildScriptReleasePatternUsesGitHubAPI(t *testing.T) {
	script := BuildScript(config.DeploymentApp{
		Name:    "kernel",
		Source:  config.DeploySourceRelease,
		Repo:    "owner/kernel",
		Version: "latest",
		Asset:   "freedex-trade-kernel-amd64-*",
		Path:    "/data/kernel",
	}, false)

	for _, want := range []string{
		"SSHM_RELEASE_API='https://api.github.com/repos/owner/kernel/releases/latest'",
		"\"browser_download_url\"",
		"case \"$name\" in 'freedex-trade-kernel-amd64-'*)",
		"未找到匹配的 Release 资源：freedex-trade-kernel-amd64-*",
		"SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
}

func TestBuildLocalFetchPrePostScripts(t *testing.T) {
	app := config.DeploymentApp{
		Name:           "api",
		Source:         config.DeploySourceGit,
		Path:           "/srv/api",
		BeforeCommands: []string{"systemctl stop api"},
		UpdateCommands: []string{"make install"},
		AfterCommands:  []string{"systemctl restart api"},
		HealthCommands: []string{"curl -fsS http://127.0.0.1:8080/health"},
	}

	pre := BuildLocalFetchPreScript(app)
	for _, want := range []string{"mkdir -p '/srv/api'", "== 更新前 ==", "systemctl stop api", "SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)"} {
		if !strings.Contains(pre, want) {
			t.Fatalf("pre script missing %q:\n%s", want, pre)
		}
	}

	post := BuildLocalFetchPostScript(app)
	for _, want := range []string{"== 更新 ==", "make install", "== 更新后 ==", "systemctl restart api", "== 健康检查 ==", "curl -fsS http://127.0.0.1:8080/health"} {
		if !strings.Contains(post, want) {
			t.Fatalf("post script missing %q:\n%s", want, post)
		}
	}
	if strings.Contains(post, "== 获取资源 ==") {
		t.Fatalf("post script should not fetch resources:\n%s", post)
	}
}

func TestLocalEnvCredentialModes(t *testing.T) {
	sshEnv := LocalEnv(config.DeploymentApp{Credential: config.DeployCredentialSSH, CredentialName: "/home/me/.ssh/deploy_key"})
	if !envContainsPrefix(sshEnv, "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/deploy_key ") {
		t.Fatalf("ssh env missing GIT_SSH_COMMAND: %#v", sshEnv)
	}

	t.Setenv("SSHM_TEST_TOKEN", "secret")
	tokenEnv := LocalEnv(config.DeploymentApp{Credential: config.DeployCredentialToken, CredentialName: "SSHM_TEST_TOKEN"})
	if !envContains(tokenEnv, "SSHM_GITHUB_AUTH_HEADER=Authorization: Bearer secret") {
		t.Fatalf("token env missing auth header: %#v", tokenEnv)
	}

	t.Setenv("SSHM_MISSING_TOKEN", "")
	missingTokenEnv := LocalEnv(config.DeploymentApp{Credential: config.DeployCredentialToken, CredentialName: "SSHM_MISSING_TOKEN"})
	if envContainsPrefix(missingTokenEnv, "SSHM_GITHUB_AUTH_HEADER=") {
		t.Fatalf("missing token should not add auth header: %#v", missingTokenEnv)
	}
}

func TestParseVersionsUsesLastMarkers(t *testing.T) {
	prev, curr := ParseVersions(strings.Join([]string{
		"SSHM_PREVIOUS_VERSION=old-a",
		"SSHM_CURRENT_VERSION=new-a",
		"noise",
		"SSHM_PREVIOUS_VERSION=old-b",
		"SSHM_CURRENT_VERSION=new-b",
	}, "\n"))
	if prev != "old-b" || curr != "new-b" {
		t.Fatalf("versions = %q -> %q, want old-b -> new-b", prev, curr)
	}
}

func envContains(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}

func envContainsPrefix(env []string, prefix string) bool {
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}
