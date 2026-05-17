package actions

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/YaMaiDay/sshm/internal/execresult"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/remotescript"
	"github.com/YaMaiDay/sshm/internal/sshconfig"
)

type Cleanup func()

type CommandResult = execresult.Result

func SSHCommand(h host.Host) (*exec.Cmd, Cleanup) {
	args, target, cleanup := sshArgs(h)
	args = append(args, "-tt", target)
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "ssh", passwordSSHOptions(h), args); ok {
		baseCleanup := cleanup
		cleanup = func() { passCleanup(); baseCleanup() }
		return interactiveCommand("sshpass", fullArgs...), cleanup
	}
	return interactiveCommand("ssh", args...), cleanup
}

func SCPUploadCommand(h host.Host, localPath, remoteDir string, recursive bool) (*exec.Cmd, Cleanup) {
	return SCPUploadCommandContext(context.Background(), h, localPath, remoteDir, recursive)
}

func SCPUploadCommandContext(ctx context.Context, h host.Host, localPath, remoteDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args, target, cleanup := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, localPath, target+":"+remoteDir+"/")
	return scpCommand(ctx, h, args, cleanup)
}

func SCPDownloadCommand(h host.Host, remotePath, localDir string, recursive bool) (*exec.Cmd, Cleanup) {
	return SCPDownloadCommandContext(context.Background(), h, remotePath, localDir, recursive)
}

func SCPDownloadCommandContext(ctx context.Context, h host.Host, remotePath, localDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args, target, cleanup := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, target+":"+remotePath, localDir+"/")
	return scpCommand(ctx, h, args, cleanup)
}

func RsyncUploadCommandContext(ctx context.Context, h host.Host, localPath, remoteDir string) (*exec.Cmd, Cleanup) {
	args, target, cleanup := rsyncArgs(h)
	args = append(args, ensureRsyncSource(localPath), target+":"+ensureRemoteDir(remoteDir))
	return rsyncCommand(ctx, h, args, cleanup)
}

func RsyncDownloadCommandContext(ctx context.Context, h host.Host, remotePath, localDir string) (*exec.Cmd, Cleanup) {
	args, target, cleanup := rsyncArgs(h)
	args = append(args, target+":"+remotePath, ensureLocalDir(localDir))
	return rsyncCommand(ctx, h, args, cleanup)
}

func RemoteRsyncCheckCommand(ctx context.Context, h host.Host) (*exec.Cmd, Cleanup) {
	return remoteShellCommand(ctx, h, "command -v rsync >/dev/null 2>&1")
}

func RemoteRsyncInstallCommand(ctx context.Context, h host.Host) (*exec.Cmd, Cleanup) {
	script := `set -eu
if command -v rsync >/dev/null 2>&1; then
  exit 0
fi
if command -v apt-get >/dev/null 2>&1; then
  sudo -n apt-get update && sudo -n apt-get install -y rsync
elif command -v dnf >/dev/null 2>&1; then
  sudo -n dnf install -y rsync
elif command -v yum >/dev/null 2>&1; then
  sudo -n yum install -y rsync
elif command -v apk >/dev/null 2>&1; then
  sudo -n apk add rsync
else
  echo "__SSHM_UNSUPPORTED_PACKAGE_MANAGER__"
  exit 1
fi`
	return remoteShellCommand(ctx, h, script)
}

func RemoteSizeCommand(h host.Host, remotePath string) (*exec.Cmd, Cleanup) {
	script := `p=$1
if [ -d "$p" ]; then
  du -sk "$p" 2>/dev/null | awk '{print $1 * 1024}'
elif [ -f "$p" ]; then
  wc -c < "$p" 2>/dev/null
fi`
	args, target, cleanup := sshArgs(h)
	args = append(args, "-o", "LogLevel=ERROR", target, "sh", "-s", "--", remotePath)
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "ssh", passwordSSHOptions(h), args); ok {
		baseCleanup := cleanup
		cleanup = func() { passCleanup(); baseCleanup() }
		cmd := exec.Command("sshpass", fullArgs...)
		cmd.Stdin = strings.NewReader(script)
		return cmd, cleanup
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

func RemoteCommandStreamContext(ctx context.Context, h host.Host, script string, onOutput func(string)) (CommandResult, Cleanup) {
	cmd, cleanup := remoteShellCommand(ctx, h, script)
	return RunCommandStream(cmd, onOutput), cleanup
}

func RunCommandStream(cmd *exec.Cmd, onOutput func(string)) CommandResult {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return CommandResult{Err: err, ExitCode: -1}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return CommandResult{Err: err, ExitCode: -1}
	}
	if err := cmd.Start(); err != nil {
		return CommandResult{Err: err, ExitCode: -1}
	}
	var mu sync.Mutex
	var output strings.Builder
	collect := func(r io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				text := string(buf[:n])
				mu.Lock()
				output.WriteString(text)
				mu.Unlock()
				if onOutput != nil {
					onOutput(text)
				}
			}
			if readErr != nil {
				return
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		collect(stdout)
	}()
	go func() {
		defer wg.Done()
		collect(stderr)
	}()
	err = cmd.Wait()
	wg.Wait()
	result := CommandResult{Output: output.String(), Err: err}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	return result
}

func remoteShellCommand(ctx context.Context, h host.Host, script string) (*exec.Cmd, Cleanup) {
	args, target, cleanup := sshArgs(h)
	args = append(args, "-o", "LogLevel=ERROR", target, "sh", "-s")
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "ssh", passwordSSHOptions(h), args); ok {
		baseCleanup := cleanup
		cleanup = func() { passCleanup(); baseCleanup() }
		cmd := exec.CommandContext(ctx, "sshpass", fullArgs...)
		cmd.Stdin = strings.NewReader(script)
		return cmd, cleanup
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = strings.NewReader(script)
	return cmd, cleanup
}

func scpCommand(ctx context.Context, h host.Host, args []string, cleanup Cleanup) (*exec.Cmd, Cleanup) {
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "scp", passwordSSHOptions(h), args); ok {
		baseCleanup := cleanup
		cleanup = func() { passCleanup(); baseCleanup() }
		return exec.CommandContext(ctx, "sshpass", fullArgs...), cleanup
	}
	return exec.CommandContext(ctx, "scp", args...), cleanup
}

func rsyncCommand(ctx context.Context, h host.Host, args []string, cleanup Cleanup) (*exec.Cmd, Cleanup) {
	if fullArgs, passCleanup, ok := sshconfig.SSHPassArgs(h.Password, "rsync", nil, args); ok {
		baseCleanup := cleanup
		cleanup = func() { passCleanup(); baseCleanup() }
		return exec.CommandContext(ctx, "sshpass", fullArgs...), cleanup
	}
	return exec.CommandContext(ctx, "rsync", args...), cleanup
}

func interactiveCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	return cmd
}

func sshArgs(h host.Host) ([]string, string, Cleanup) {
	return sshconfig.SSHArgs(h)
}

func scpArgs(h host.Host) ([]string, string, Cleanup) {
	return sshconfig.SCPArgs(h)
}

func rsyncArgs(h host.Host) ([]string, string, Cleanup) {
	args := []string{"-az", "--partial", "--append", "--progress"}
	sshArgs, target, cleanup := sshArgs(h)
	args = append(args, "-e", "ssh "+strings.Join(shellQuoteArgs(sshArgs), " "))
	return args, target, cleanup
}

func passwordSSHOptions(h host.Host) []string {
	args := sshconfig.PasswordAuthArgs(h)
	if h.JumpEnabled {
		args = append(args, "-o", "BatchMode=no")
	}
	return args
}

func shellQuoteArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, shellQuote(arg))
	}
	return out
}

func shellQuote(value string) string {
	return remotescript.Quote(value)
}

func ensureRsyncSource(path string) string {
	return path
}

func ensureRemoteDir(path string) string {
	return strings.TrimRight(path, "/") + "/"
}

func ensureLocalDir(path string) string {
	return strings.TrimRight(path, string(os.PathSeparator)) + string(os.PathSeparator)
}
