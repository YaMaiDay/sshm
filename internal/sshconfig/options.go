package sshconfig

import (
	"os/exec"
	"strings"
	"sync"

	"github.com/YaMaiDay/sshm/internal/host"
)

var (
	warnWeakCryptoOnce      sync.Once
	warnWeakCryptoSupported bool
)

func WarnWeakCryptoNoPQKexArgs() []string {
	warnWeakCryptoOnce.Do(func() {
		cmd := exec.Command("ssh", "-G", "-o", "WarnWeakCrypto=no-pq-kex", "example.com")
		warnWeakCryptoSupported = cmd.Run() == nil
	})
	if !warnWeakCryptoSupported {
		return nil
	}
	return []string{"-o", "WarnWeakCrypto=no-pq-kex"}
}

func StrictSSHArgs(h host.Host) []string {
	args := []string{
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
	}
	if strings.TrimSpace(h.IdentityFile) != "" || strings.TrimSpace(h.JumpKeyPath) != "" {
		args = append(args,
			"-o", "IdentitiesOnly=yes",
			"-o", "IdentityAgent=none",
		)
	}
	return args
}

func PasswordAuthArgs(h host.Host) []string {
	authMethods := "password,keyboard-interactive"
	if strings.TrimSpace(h.IdentityFile) != "" || strings.TrimSpace(h.JumpKeyPath) != "" {
		authMethods = "publickey,password,keyboard-interactive"
	}
	args := []string{
		"-o", "PreferredAuthentications=" + authMethods,
		"-o", "PasswordAuthentication=yes",
		"-o", "KbdInteractiveAuthentication=yes",
	}
	if strings.TrimSpace(h.IdentityFile) == "" && strings.TrimSpace(h.JumpKeyPath) == "" {
		args = append(args, "-o", "PubkeyAuthentication=no")
	}
	return args
}
