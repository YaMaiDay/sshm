package sshconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/YaMaiDay/sshm/internal/host"
)

type Connection struct {
	Args    []string
	Target  string
	Cleanup func()
}

func SSHArgs(h host.Host, extra ...string) ([]string, string, func()) {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
	args = append(args, extra...)
	args = append(args, WarnWeakCryptoNoPQKexArgs()...)
	args = append(args, StrictSSHArgs(h)...)
	connection, err := BuildConnection(h)
	if err == nil {
		args = append(args, connection.Args...)
	} else {
		return append(args, "-o", "LogLevel=ERROR"), h.Target(), func() {}
	}
	if !h.JumpEnabled && h.Port != "" {
		args = append(args, "-p", h.Port)
	}
	if !h.JumpEnabled && h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	target := h.Target()
	if connection.Target != "" {
		target = connection.Target
	}
	return args, target, connection.Cleanup
}

func SCPArgs(h host.Host, extra ...string) ([]string, string, func()) {
	args := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "LogLevel=ERROR",
	}
	args = append(args, extra...)
	args = append(args, WarnWeakCryptoNoPQKexArgs()...)
	args = append(args, StrictSSHArgs(h)...)
	connection, err := BuildConnection(h)
	if err == nil {
		args = append(args, connection.Args...)
	} else {
		return append(args, "-o", "LogLevel=ERROR"), h.Target(), func() {}
	}
	if !h.JumpEnabled && h.Port != "" {
		args = append(args, "-P", h.Port)
	}
	if !h.JumpEnabled && h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	target := h.Target()
	if connection.Target != "" {
		target = connection.Target
	}
	return args, target, connection.Cleanup
}

func BuildConnection(h host.Host) (Connection, error) {
	if !h.JumpEnabled {
		return Connection{Target: h.Target(), Cleanup: func() {}}, nil
	}
	file, err := os.CreateTemp("", "sshm-ssh-config-*")
	if err != nil {
		return Connection{}, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		cleanup()
		return Connection{}, err
	}
	config := renderJumpConfig(h)
	if _, err := file.WriteString(config); err != nil {
		_ = file.Close()
		cleanup()
		return Connection{}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return Connection{}, err
	}
	return Connection{
		Args:    []string{"-F", file.Name()},
		Target:  "sshm-target",
		Cleanup: cleanup,
	}, nil
}

func renderJumpConfig(h host.Host) string {
	target := h.Endpoint()
	jump := h.JumpEndpoint()

	var b strings.Builder
	b.WriteString("Host sshm-jump\n")
	writeConfigValue(&b, "HostName", jump.Address)
	writeConfigValue(&b, "User", jump.User)
	writeConfigValue(&b, "Port", defaultPort(jump.Port))
	writeConfigValue(&b, "IdentityFile", jump.IdentityFile)
	if jump.IdentityFile != "" {
		b.WriteString("    IdentitiesOnly yes\n")
		b.WriteString("    IdentityAgent none\n")
	}
	b.WriteString("    StrictHostKeyChecking accept-new\n")
	b.WriteString("    ControlMaster no\n")
	b.WriteString("    ControlPath none\n\n")

	b.WriteString("Host sshm-target\n")
	writeConfigValue(&b, "HostName", target.Address)
	writeConfigValue(&b, "User", target.User)
	writeConfigValue(&b, "Port", defaultPort(target.Port))
	writeConfigValue(&b, "IdentityFile", target.IdentityFile)
	if target.IdentityFile != "" {
		b.WriteString("    IdentitiesOnly yes\n")
		b.WriteString("    IdentityAgent none\n")
	}
	b.WriteString("    ProxyJump sshm-jump\n")
	b.WriteString("    StrictHostKeyChecking accept-new\n")
	b.WriteString("    ControlMaster no\n")
	b.WriteString("    ControlPath none\n")
	return b.String()
}

func writeConfigValue(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "    %s %s\n", key, quoteConfigValue(value))
}

func quoteConfigValue(value string) string {
	if strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '"' || r == '\\'
	}) == -1 {
		return value
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func defaultPort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		return "22"
	}
	return port
}
