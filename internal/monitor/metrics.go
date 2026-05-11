package monitor

import "time"

type Metrics struct {
	Online         bool
	OS             string
	RemoteHostname string
	CPUPercent     float64
	MemUsed        uint64
	MemTotal       uint64
	MemAvailable   uint64
	DiskUsed       uint64
	DiskTotal      uint64
	DiskAvailable  uint64
	DiskAvailKnown bool
	Load1          string
	Load5          string
	Load15         string
	Uptime         string
	DockerRunning  int
	FailedServices int
	FailedUnits    []string
	Ports          string
	Error          string
	UpdatedAt      time.Time
}

func (m Metrics) MemPercent() float64 {
	if m.MemTotal == 0 {
		return 0
	}
	return float64(m.MemUsed) * 100 / float64(m.MemTotal)
}

func (m Metrics) DiskPercent() float64 {
	if m.DiskAvailKnown {
		usable := m.DiskUsed + m.DiskAvailable
		if usable == 0 {
			return 0
		}
		return float64(m.DiskUsed) * 100 / float64(usable)
	}
	if m.DiskTotal == 0 {
		return 0
	}
	return float64(m.DiskUsed) * 100 / float64(m.DiskTotal)
}
