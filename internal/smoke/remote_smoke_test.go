package smoke

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
	"github.com/YaMaiDay/sshm/internal/host"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func TestRemoteSSHTransferDeploymentSmoke(t *testing.T) {
	if os.Getenv("SSHM_REMOTE_SMOKE") != "1" {
		t.Skip("set SSHM_REMOTE_SMOKE=1 to run real remote smoke")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	hosts, ok, err := config.LoadServerHosts(home)
	if err != nil {
		t.Fatalf("load hosts: %v", err)
	}
	if !ok || len(hosts) == 0 {
		t.Fatal("no sshm hosts configured")
	}
	h, ok := smokeHost(hosts)
	if !ok {
		t.Fatal("configured smoke host was not found")
	}
	t.Logf("remote smoke host: %s/%s", h.Category, h.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	runRemoteScript(t, ctx, h, "printf sshm-smoke")
	runRsyncSmoke(t, ctx, h)
	runDeploymentSmoke(t, ctx, h)
}

func smokeHost(hosts []host.Host) (host.Host, bool) {
	category := strings.TrimSpace(os.Getenv("SSHM_SMOKE_HOST_CATEGORY"))
	name := strings.TrimSpace(os.Getenv("SSHM_SMOKE_HOST_NAME"))
	for _, h := range hosts {
		if category != "" && h.Category != category {
			continue
		}
		if name != "" && h.Name != name {
			continue
		}
		return h, true
	}
	if category != "" || name != "" {
		return host.Host{}, false
	}
	return hosts[0], true
}

func runRemoteScript(t *testing.T, ctx context.Context, h host.Host, script string) string {
	t.Helper()
	result, cleanup := actions.RemoteCommandContext(ctx, h, script)
	cleanup()
	if result.Err != nil {
		t.Fatalf("remote command failed: %v\n%s", result.Err, strings.TrimSpace(result.Output))
	}
	return result.Output
}

func runRsyncSmoke(t *testing.T, ctx context.Context, h host.Host) {
	t.Helper()
	service := transferservice.Service{}
	if err := service.CheckRsync(ctx, h); err != nil {
		t.Fatalf("remote rsync check failed: %v", err)
	}
	localDir := t.TempDir()
	localFile := filepath.Join(localDir, "payload.txt")
	if err := os.WriteFile(localFile, []byte("sshm-rsync-smoke\n"), 0o600); err != nil {
		t.Fatalf("write local payload: %v", err)
	}
	remoteDir := "/tmp/sshm-smoke-rsync"
	runRemoteScript(t, ctx, h, "rm -rf /tmp/sshm-smoke-rsync && mkdir -p /tmp/sshm-smoke-rsync")
	entry := transferservice.BuildEntry(h, transferservice.EntrySpec{
		Kind:      "upload",
		Status:    config.TransferStatusRunning,
		Source:    localFile,
		TargetDir: remoteDir,
	})
	result := service.RunJob(ctx, h, entry, nil)
	if result.Err != nil {
		t.Fatalf("rsync upload failed: %v\n%s", result.Err, strings.TrimSpace(result.Output))
	}
	out := runRemoteScript(t, ctx, h, "cat /tmp/sshm-smoke-rsync/payload.txt && rm -rf /tmp/sshm-smoke-rsync")
	if strings.TrimSpace(out) != "sshm-rsync-smoke" {
		t.Fatalf("unexpected rsync payload: %q", strings.TrimSpace(out))
	}
}

func runDeploymentSmoke(t *testing.T, ctx context.Context, h host.Host) {
	t.Helper()
	app := config.DeploymentApp{
		Name:             "sshm-smoke",
		Server:           h.Category + "/" + h.Name,
		Source:           config.DeploySourceGit,
		FetchMode:        config.DeployFetchRemote,
		Path:             "/tmp/sshm-smoke-deploy",
		ResourceCommands: []string{"echo resource-ok"},
		UpdateCommands:   []string{"printf deployment-ok > smoke.txt", "cat smoke.txt"},
		AfterCommands:    []string{"rm -rf /tmp/sshm-smoke-deploy"},
	}
	result := deploymentservice.Service{}.Run(ctx, h, app, nil)
	if result.Command.Err != nil {
		t.Fatalf("deployment smoke failed: %v\n%s", result.Command.Err, strings.TrimSpace(result.Command.Output))
	}
	if !strings.Contains(result.Command.Output, "deployment-ok") {
		t.Fatalf("deployment output missing marker:\n%s", result.Command.Output)
	}
}
