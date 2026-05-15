package actions

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestSSHCommandBuildsBasicArgs(t *testing.T) {
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

func TestSCPUploadDownloadArgs(t *testing.T) {
	h := host.Host{HostName: "example.com", User: "deploy", Port: "2200"}

	upload, cleanup := SCPUploadCommandContext(context.Background(), h, "/tmp/app", "/srv/app", true)
	defer cleanup()
	uploadArgs := strings.Join(upload.Args[1:], " ")
	for _, want := range []string{"-P 2200", "-r", "/tmp/app", "deploy@example.com:/srv/app/"} {
		if !strings.Contains(uploadArgs, want) {
			t.Fatalf("scp upload args missing %q in %q", want, uploadArgs)
		}
	}

	download, cleanup := SCPDownloadCommandContext(context.Background(), h, "/srv/app/log.txt", "/tmp/out", false)
	defer cleanup()
	downloadArgs := strings.Join(download.Args[1:], " ")
	for _, want := range []string{"-P 2200", "deploy@example.com:/srv/app/log.txt", "/tmp/out/"} {
		if !strings.Contains(downloadArgs, want) {
			t.Fatalf("scp download args missing %q in %q", want, downloadArgs)
		}
	}
	if strings.Contains(downloadArgs, " -r ") {
		t.Fatalf("scp download args unexpectedly recursive: %q", downloadArgs)
	}
}

func TestRsyncUploadDownloadArgs(t *testing.T) {
	h := host.Host{HostName: "example.com", User: "deploy", Port: "2200", IdentityFile: "/tmp/id_ed25519"}

	upload, cleanup := RsyncUploadCommandContext(context.Background(), h, "/tmp/app/", "/srv/app")
	defer cleanup()
	uploadArgs := strings.Join(upload.Args[1:], " ")
	for _, want := range []string{"-az", "--partial", "--append", "--progress", "-e", "ssh", "-p 2200", "-i /tmp/id_ed25519", "/tmp/app/", "deploy@example.com:/srv/app/"} {
		if !strings.Contains(uploadArgs, want) {
			t.Fatalf("rsync upload args missing %q in %q", want, uploadArgs)
		}
	}

	download, cleanup := RsyncDownloadCommandContext(context.Background(), h, "/srv/app/", "/tmp/out")
	defer cleanup()
	downloadArgs := strings.Join(download.Args[1:], " ")
	for _, want := range []string{"deploy@example.com:/srv/app/", "/tmp/out/"} {
		if !strings.Contains(downloadArgs, want) {
			t.Fatalf("rsync download args missing %q in %q", want, downloadArgs)
		}
	}
}

func TestProxyJumpUsesTempSSHConfig(t *testing.T) {
	cmd, cleanup := SSHCommand(host.Host{
		HostName:     "10.0.0.5",
		User:         "app",
		Port:         "2222",
		IdentityFile: "/tmp/app key",
		JumpEnabled:  true,
		JumpHost:     "198.51.100.10",
		JumpUser:     "jump",
		JumpPort:     "6000",
		JumpKeyPath:  "/tmp/jump key",
	})
	defer cleanup()

	args := cmd.Args[1:]
	configPath := ""
	for i, arg := range args {
		if arg == "-F" && i+1 < len(args) {
			configPath = args[i+1]
			break
		}
	}
	if configPath == "" {
		t.Fatalf("ssh args missing -F temp config: %#v", args)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read temp config: %v", err)
	}
	config := string(data)
	for _, want := range []string{
		"Host sshm-jump",
		"HostName 198.51.100.10",
		"User jump",
		"Port 6000",
		`IdentityFile "/tmp/jump key"`,
		"Host sshm-target",
		"HostName 10.0.0.5",
		"User app",
		"Port 2222",
		`IdentityFile "/tmp/app key"`,
		"ProxyJump sshm-jump",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("temp config missing %q in:\n%s", want, config)
		}
	}
	if !strings.Contains(strings.Join(args, " "), "-tt sshm-target") {
		t.Fatalf("ssh args should target sshm-target through temp config: %#v", args)
	}

	cleanup()
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove temp config %q, err=%v", configPath, err)
	}
}

func TestShellQuoteArgs(t *testing.T) {
	got := strings.Join(shellQuoteArgs([]string{"-i", "/tmp/key with space", "plain=value"}), " ")
	want := "-i '/tmp/key with space' plain=value"
	if got != want {
		t.Fatalf("shellQuoteArgs = %q, want %q", got, want)
	}
}
