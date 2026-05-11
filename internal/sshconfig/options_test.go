package sshconfig

import (
	"slices"
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
