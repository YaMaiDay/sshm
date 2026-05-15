package tui

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestBuildDeploymentScriptRollbackOnlyRunsRollbackCommands(t *testing.T) {
	script := buildDeploymentScript(config.DeploymentApp{
		Name:             "api",
		Source:           config.DeploySourceGit,
		Repo:             "git@github.com:owner/api.git",
		Branch:           "main",
		Path:             "/data/api",
		UpdateCommands:   []string{"make deploy"},
		HealthCommands:   []string{"curl -fsS http://127.0.0.1:8080/health"},
		RollbackCommands: []string{"ln -sfn releases/old current", "systemctl restart api"},
	}, true)

	for _, want := range []string{
		"== 回滚 ==",
		"cd '/data/api'",
		"ln -sfn releases/old current",
		"systemctl restart api",
	} {
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

	pre := buildLocalFetchPreScript(app)
	for _, want := range []string{
		"mkdir -p '/srv/api'",
		"== 更新前 ==",
		"systemctl stop api",
		"SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
	} {
		if !strings.Contains(pre, want) {
			t.Fatalf("pre script missing %q:\n%s", want, pre)
		}
	}

	post := buildLocalFetchPostScript(app)
	for _, want := range []string{
		"== 更新 ==",
		"make install",
		"== 更新后 ==",
		"systemctl restart api",
		"== 健康检查 ==",
		"curl -fsS http://127.0.0.1:8080/health",
	} {
		if !strings.Contains(post, want) {
			t.Fatalf("post script missing %q:\n%s", want, post)
		}
	}
	if strings.Contains(post, "== 获取资源 ==") {
		t.Fatalf("post script should not fetch resources:\n%s", post)
	}
}

func TestDeploymentLocalEnvCredentialModes(t *testing.T) {
	sshEnv := deploymentLocalEnv(config.DeploymentApp{
		Credential:     config.DeployCredentialSSH,
		CredentialName: "/home/me/.ssh/deploy_key",
	})
	if !envContainsPrefix(sshEnv, "GIT_SSH_COMMAND=ssh -i /home/me/.ssh/deploy_key ") {
		t.Fatalf("ssh env missing GIT_SSH_COMMAND: %#v", sshEnv)
	}

	t.Setenv("SSHM_TEST_TOKEN", "secret")
	tokenEnv := deploymentLocalEnv(config.DeploymentApp{
		Credential:     config.DeployCredentialToken,
		CredentialName: "SSHM_TEST_TOKEN",
	})
	if !envContains(tokenEnv, "SSHM_GITHUB_AUTH_HEADER=Authorization: Bearer secret") {
		t.Fatalf("token env missing auth header: %#v", tokenEnv)
	}

	t.Setenv("SSHM_MISSING_TOKEN", "")
	missingTokenEnv := deploymentLocalEnv(config.DeploymentApp{
		Credential:     config.DeployCredentialToken,
		CredentialName: "SSHM_MISSING_TOKEN",
	})
	if envContainsPrefix(missingTokenEnv, "SSHM_GITHUB_AUTH_HEADER=") {
		t.Fatalf("missing token should not add auth header: %#v", missingTokenEnv)
	}
}

func TestParseDeploymentVersionsUsesLastMarkers(t *testing.T) {
	prev, curr := parseDeploymentVersions(strings.Join([]string{
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
