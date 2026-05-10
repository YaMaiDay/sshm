package host

type Host struct {
	Name         string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
	Password     string
	Category     string
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
