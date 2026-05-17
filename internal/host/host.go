package host

import "strings"

type Host struct {
	Name         string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	Password     string
	JumpHostRef  string
	JumpEnabled  bool
	JumpHost     string
	JumpUser     string
	JumpPort     string
	JumpKeyPath  string
	Category     string
	Note         string
	ExpireAt     string
	Favorite     bool
	Pinned       bool
	PinnedOrder  int64
	HealthPorts  []int
	File         string
	HasPassword  bool
}

type Endpoint struct {
	Address      string
	User         string
	Port         string
	IdentityFile string
}

func (e Endpoint) Target() string {
	if e.User == "" {
		return e.Address
	}
	return e.User + "@" + e.Address
}

func (h Host) Address() string {
	if h.HostName != "" {
		return h.HostName
	}
	return h.Name
}

func (h Host) Endpoint() Endpoint {
	return Endpoint{
		Address:      strings.TrimSpace(h.Address()),
		User:         strings.TrimSpace(h.User),
		Port:         strings.TrimSpace(h.Port),
		IdentityFile: strings.TrimSpace(h.IdentityFile),
	}
}

func (h Host) JumpEndpoint() Endpoint {
	return Endpoint{
		Address:      strings.TrimSpace(h.JumpHost),
		User:         strings.TrimSpace(h.JumpUser),
		Port:         strings.TrimSpace(h.JumpPort),
		IdentityFile: strings.TrimSpace(h.JumpKeyPath),
	}
}

func (h Host) Target() string {
	return h.Endpoint().Target()
}

func (h Host) JumpTarget() string {
	endpoint := h.JumpEndpoint()
	if endpoint.Address == "" {
		return ""
	}
	return endpoint.Target()
}
