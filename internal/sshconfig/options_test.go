package sshconfig

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestStrictSSHArgsWithIdentityFile(t *testing.T) {
	args := StrictSSHArgs(host.Host{IdentityFile: "/tmp/test-key"})

	for _, want := range []string{"ControlMaster=no", "ControlPath=none", "IdentitiesOnly=yes", "IdentityAgent=none"} {
		if !slices.Contains(args, want) {
			t.Fatalf("StrictSSHArgs() = %#v, want %q", args, want)
		}
	}
}

func TestStrictSSHArgsWithoutIdentityFile(t *testing.T) {
	args := StrictSSHArgs(host.Host{})

	for _, want := range []string{"ControlMaster=no", "ControlPath=none"} {
		if !slices.Contains(args, want) {
			t.Fatalf("StrictSSHArgs() = %#v, want %q", args, want)
		}
	}
	for _, unwanted := range []string{"IdentitiesOnly=yes", "IdentityAgent=none"} {
		if slices.Contains(args, unwanted) {
			t.Fatalf("StrictSSHArgs() = %#v, did not want %q", args, unwanted)
		}
	}
}

func TestPasswordAuthArgsWithoutIdentityFileDisablePubkey(t *testing.T) {
	args := PasswordAuthArgs(host.Host{})

	for _, want := range []string{
		"PreferredAuthentications=password,keyboard-interactive",
		"PasswordAuthentication=yes",
		"KbdInteractiveAuthentication=yes",
		"PubkeyAuthentication=no",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("PasswordAuthArgs() = %#v, want %q", args, want)
		}
	}
}

func TestPasswordAuthArgsWithIdentityFilePreferConfiguredKey(t *testing.T) {
	args := PasswordAuthArgs(host.Host{IdentityFile: "/tmp/test-key"})

	if !slices.Contains(args, "PreferredAuthentications=publickey,password,keyboard-interactive") {
		t.Fatalf("PasswordAuthArgs() = %#v, want publickey first", args)
	}
	if slices.Contains(args, "PubkeyAuthentication=no") {
		t.Fatalf("PasswordAuthArgs() = %#v, did not want pubkey disabled", args)
	}
}

func TestBuildConnectionWithoutJumpUsesDirectTarget(t *testing.T) {
	conn, err := BuildConnection(host.Host{HostName: "192.0.2.10", User: "root"})
	if err != nil {
		t.Fatalf("BuildConnection() error = %v", err)
	}
	defer conn.Cleanup()

	if conn.Target != "root@192.0.2.10" {
		t.Fatalf("target = %q, want root@192.0.2.10", conn.Target)
	}
	if len(conn.Args) != 0 {
		t.Fatalf("args = %#v, want empty direct args", conn.Args)
	}
}

func TestBuildConnectionWithJumpCreatesTempConfig(t *testing.T) {
	conn, err := BuildConnection(host.Host{
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
	if err != nil {
		t.Fatalf("BuildConnection() error = %v", err)
	}
	if conn.Target != "sshm-target" {
		t.Fatalf("target = %q, want sshm-target", conn.Target)
	}
	if len(conn.Args) != 2 || conn.Args[0] != "-F" {
		t.Fatalf("args = %#v, want -F temp config", conn.Args)
	}
	data, err := os.ReadFile(conn.Args[1])
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
		"IdentityAgent none",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q in:\n%s", want, config)
		}
	}
	conn.Cleanup()
	if _, err := os.Stat(conn.Args[1]); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove temp config, err=%v", err)
	}
}

func TestSSHAndSCPArgsUseDirectPortAndIdentity(t *testing.T) {
	h := host.Host{HostName: "example.com", User: "deploy", Port: "2200", IdentityFile: "/tmp/id_ed25519"}

	sshArgs, target, cleanup := SSHArgs(h, "-o", "ConnectTimeout=5")
	defer cleanup()
	if target != "deploy@example.com" {
		t.Fatalf("ssh target = %q", target)
	}
	for _, want := range []string{"ConnectTimeout=5", "-p", "2200", "-i", "/tmp/id_ed25519"} {
		if !slices.Contains(sshArgs, want) {
			t.Fatalf("SSHArgs() = %#v, want %q", sshArgs, want)
		}
	}

	scpArgs, target, cleanup := SCPArgs(h)
	defer cleanup()
	if target != "deploy@example.com" {
		t.Fatalf("scp target = %q", target)
	}
	for _, want := range []string{"-P", "2200", "-i", "/tmp/id_ed25519", "LogLevel=ERROR"} {
		if !slices.Contains(scpArgs, want) {
			t.Fatalf("SCPArgs() = %#v, want %q", scpArgs, want)
		}
	}
}

func TestTempPasswordFilePermissionsAndContent(t *testing.T) {
	path, err := TempPasswordFile("secret")
	if err != nil {
		t.Fatalf("TempPasswordFile() error = %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read password file: %v", err)
	}
	if string(data) != "secret\n" {
		t.Fatalf("password file content = %q", string(data))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat password file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("password file mode = %v, want 0600", got)
	}
}
