package session

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestSSHCommandDelegatesToActionsBuilder(t *testing.T) {
	cmd, cleanup := SSHCommand(host.Host{
		HostName:     "192.0.2.10",
		User:         "root",
		Port:         "2222",
		IdentityFile: "/tmp/id_ed25519",
	})
	defer cleanup()

	if cmd.Path == "" || !strings.HasSuffix(cmd.Path, "ssh") {
		t.Fatalf("command path = %q, want ssh", cmd.Path)
	}
	args := strings.Join(cmd.Args[1:], " ")
	for _, want := range []string{
		"StrictHostKeyChecking=accept-new",
		"-p 2222",
		"-i /tmp/id_ed25519",
		"-tt root@192.0.2.10",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("ssh args missing %q in %q", want, args)
		}
	}
}
