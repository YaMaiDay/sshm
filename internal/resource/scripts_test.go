package resource

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestActionScriptBuildsServiceAndContainerCommands(t *testing.T) {
	service := ActionScript(config.ResourceKindService, "restart", "nginx.service")
	if !strings.Contains(service, "systemctl restart nginx.service") || !strings.Contains(service, "sudo -n systemctl restart nginx.service") {
		t.Fatalf("unexpected service action script: %s", service)
	}
	container := ActionScript(config.ResourceKindContainer, "stop", "web app")
	if !strings.Contains(container, "docker stop 'web app'") || !strings.Contains(container, "sudo -n docker stop 'web app'") {
		t.Fatalf("unexpected container action script: %s", container)
	}
	if got := ActionScript(config.ResourceKindPort, "stop", "8080"); got != "" {
		t.Fatalf("port action script = %q, want empty", got)
	}
	if got := ActionScript(config.ResourceKindService, "restart; touch /tmp/pwned", "nginx.service"); got != "" {
		t.Fatalf("unsafe action command produced script: %q", got)
	}
	if got := ActionPreview(config.ResourceKindContainer, "stop && rm -rf /", "web", config.ManagedResource{}); got != "-" {
		t.Fatalf("unsafe action preview = %q, want disabled marker", got)
	}
}

func TestManagedActionScriptAllowsExplicitCustomCommands(t *testing.T) {
	managed := config.ManagedResource{StartCommand: "cd /srv/app && ./start.sh"}
	got := ManagedActionScript(config.ResourceKindProcess, "start", "app", managed)
	if !strings.Contains(got, "cd /srv/app && ./start.sh") {
		t.Fatalf("managed custom action script = %q, want explicit custom command", got)
	}
}

func TestLogScriptDefaultsAndUnsupportedKinds(t *testing.T) {
	service := LogScript(config.ResourceKindService, "nginx.service", 0)
	if !strings.Contains(service, "journalctl -u nginx.service -n 200 --no-pager") {
		t.Fatalf("unexpected service log script: %s", service)
	}
	container := LogScript(config.ResourceKindContainer, "web", 50)
	if !strings.Contains(container, "docker logs --tail 50 web") {
		t.Fatalf("unexpected container log script: %s", container)
	}
	if got := LogScript(config.ResourceKindDatabase, "db", 100); got != "" {
		t.Fatalf("database log script = %q, want empty", got)
	}
}

func TestDetailScriptsContainMarkers(t *testing.T) {
	if !strings.Contains(ServiceDetailScript(), "__SSHM_SERVICE__") {
		t.Fatal("service detail script missing marker")
	}
	if !strings.Contains(ContainerDetailScript(), "__SSHM_CONTAINER__") {
		t.Fatal("container detail script missing marker")
	}
	if !strings.Contains(PortDetailScript(), "__SSHM_PORT_CGROUP__") {
		t.Fatal("port detail script missing cgroup marker")
	}
}
