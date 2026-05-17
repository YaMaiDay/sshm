package host

import "testing"

func TestEndpointTarget(t *testing.T) {
	h := Host{Name: "alias", HostName: "192.0.2.10", User: " root ", Port: " 2222 ", IdentityFile: " /tmp/key "}

	endpoint := h.Endpoint()
	if endpoint.Address != "192.0.2.10" || endpoint.User != "root" || endpoint.Port != "2222" || endpoint.IdentityFile != "/tmp/key" {
		t.Fatalf("Endpoint() = %#v", endpoint)
	}
	if endpoint.Target() != "root@192.0.2.10" {
		t.Fatalf("Target() = %q", endpoint.Target())
	}
}

func TestJumpEndpointTarget(t *testing.T) {
	h := Host{JumpHost: " 198.51.100.10 ", JumpUser: " jump "}

	if got := h.JumpTarget(); got != "jump@198.51.100.10" {
		t.Fatalf("JumpTarget() = %q", got)
	}
}
