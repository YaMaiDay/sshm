package host

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

func (h Host) Address() string {
	if h.HostName != "" {
		return h.HostName
	}
	return h.Name
}

func (h Host) Target() string {
	address := h.Address()
	if h.User == "" {
		return address
	}
	return h.User + "@" + address
}

func (h Host) JumpTarget() string {
	if h.JumpHost == "" {
		return ""
	}
	if h.JumpUser == "" {
		return h.JumpHost
	}
	return h.JumpUser + "@" + h.JumpHost
}
