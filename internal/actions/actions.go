package actions

import (
	"os"
	"os/exec"
	"strings"

	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/sshconfig"
)

type Cleanup func()

func SSHCommand(h host.Host) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	args := sshArgs(h)
	args = append(args, h.Target())
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "ssh", "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no"}, args...)
				return attachTerminal(exec.Command("sshpass", fullArgs...)), cleanup
			}
		}
	}
	return attachTerminal(exec.Command("ssh", args...)), cleanup
}

func SCPUploadCommand(h host.Host, localPath, remoteDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, localPath, h.Target()+":"+remoteDir+"/")
	return scpCommand(h, args)
}

func SCPDownloadCommand(h host.Host, remotePath, localDir string, recursive bool) (*exec.Cmd, Cleanup) {
	args := scpArgs(h)
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, h.Target()+":"+remotePath, localDir+"/")
	return scpCommand(h, args)
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
				fullArgs := append([]string{"-f", file, "ssh", "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no"}, args...)
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

func scpCommand(h host.Host, args []string) (*exec.Cmd, Cleanup) {
	cleanup := func() {}
	if strings.TrimSpace(h.Password) != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := tempPasswordFile(h.Password)
			if err == nil {
				cleanup = func() { _ = os.Remove(file) }
				fullArgs := append([]string{"-f", file, "scp", "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no"}, args...)
				return exec.Command("sshpass", fullArgs...), cleanup
			}
		}
	}
	return exec.Command("scp", args...), cleanup
}

func attachTerminal(cmd *exec.Cmd) *exec.Cmd {
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func sshArgs(h host.Host) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
	args = append(args, sshconfig.WarnWeakCryptoNoPQKexArgs()...)
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
