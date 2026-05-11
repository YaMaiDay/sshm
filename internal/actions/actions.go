package actions

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/sshconfig"
)

type Cleanup func()

type CommandResult struct {
	Output   string
	ExitCode int
	Err      error
}

func SSHCommand(h host.Host) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	args := sshArgs(h)
	args = append(args, "-tt", h.Target())
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "ssh"}, passwordSSHOptions(h)...)
				fullArgs = append(fullArgs, args...)
				return interactiveCommand("sshpass", fullArgs...), cleanup
			}
		}
	}
	return interactiveCommand("ssh", args...), cleanup
}

func SCPUploadCommand(h host.Host, localPath, remoteDir string, recursive bool) (*exec.Cmd, Cleanup) {
	return SCPUploadCommandContext(context.Background(), h, localPath, remoteDir, recursive)
}

func SCPUploadCommandContext(ctx context.Context, h host.Host, localPath, remoteDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, localPath, h.Target()+":"+remoteDir+"/")
	return scpCommand(ctx, h, args)
}

func SCPDownloadCommand(h host.Host, remotePath, localDir string, recursive bool) (*exec.Cmd, Cleanup) {
	return SCPDownloadCommandContext(context.Background(), h, remotePath, localDir, recursive)
}

func SCPDownloadCommandContext(ctx context.Context, h host.Host, remotePath, localDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, h.Target()+":"+remotePath, localDir+"/")
	return scpCommand(ctx, h, args)
}

func RemoteSizeCommand(h host.Host, remotePath string) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	script := `p=$1
if [ -d "$p" ]; then
  du -sk "$p" 2>/dev/null | awk '{print $1 * 1024}'
elif [ -f "$p" ]; then
  wc -c < "$p" 2>/dev/null
fi`
	args := append(sshArgs(h), "-o", "LogLevel=ERROR", h.Target(), "sh", "-s", "--", remotePath)
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "ssh"}, passwordSSHOptions(h)...)
				fullArgs = append(fullArgs, args...)
				cmd := exec.Command("sshpass", fullArgs...)
				cmd.Stdin = strings.NewReader(script)
				return cmd, cleanup
			}
		}
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	return cmd, cleanup
}

func RemoteCommandContext(ctx context.Context, h host.Host, script string) (CommandResult, Cleanup) {
	cmd, cleanup := remoteShellCommand(ctx, h, script)
	output, err := cmd.CombinedOutput()
	result := CommandResult{Output: string(output), Err: err}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	return result, cleanup
}

func remoteShellCommand(ctx context.Context, h host.Host, script string) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	args := append(sshArgs(h), "-o", "LogLevel=ERROR", h.Target(), "sh", "-s")
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "ssh"}, passwordSSHOptions(h)...)
				fullArgs = append(fullArgs, args...)
				cmd := exec.CommandContext(ctx, "sshpass", fullArgs...)
				cmd.Stdin = strings.NewReader(script)
				return cmd, cleanup
			}
		}
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	return cmd, cleanup
}

func scpCommand(ctx context.Context, h host.Host, args []string) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "scp"}, passwordSSHOptions(h)...)
				fullArgs = append(fullArgs, args...)
				return exec.CommandContext(ctx, "sshpass", fullArgs...), cleanup
			}
		}
	}
	return exec.CommandContext(ctx, "scp", args...), cleanup
}

func interactiveCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return cmd
}

func sshArgs(h host.Host) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
	args = append(args, sshconfig.WarnWeakCryptoNoPQKexArgs()...)
	args = append(args, sshconfig.StrictSSHArgs(h)...)
	if h.Port != "" {
		args = append(args, "-p", h.Port)
	}
	if h.ProxyJump != "" {
		args = append(args, "-J", h.ProxyJump)
	}
	if h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	return args
}

func scpArgs(h host.Host) []string {
	args := []string{
		"-q",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "LogLevel=ERROR",
	}
	args = append(args, sshconfig.WarnWeakCryptoNoPQKexArgs()...)
	args = append(args, sshconfig.StrictSSHArgs(h)...)
	if h.Port != "" {
		args = append(args, "-P", h.Port)
	}
	if h.ProxyJump != "" {
		args = append(args, "-o", "ProxyJump="+h.ProxyJump)
	}
	if h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	return args
}

func passwordSSHOptions(h host.Host) []string {
	return sshconfig.PasswordAuthArgs(h)
}

func tempPasswordFile(password string) (string, error) {
	file, err := os.CreateTemp("", "sshm-pass-*")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	if _, err := file.WriteString(password + "\n"); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
