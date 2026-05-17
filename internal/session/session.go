package session

import (
	"os/exec"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/host"
)

func SSHCommand(h host.Host) (*exec.Cmd, func()) {
	return actions.SSHCommand(h)
}
