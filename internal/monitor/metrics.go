package monitor

import "time"

type HealthPort struct {
	Port    int
	Healthy bool
}

type DiskMetric struct {
	Filesystem string
	Type       string
	Mountpoint string
	Used       uint64
	Total      uint64
	Available  uint64
	AvailKnown bool
}

func (d DiskMetric) Percent() float64 {
	if d.AvailKnown {
		usable := d.Used + d.Available
		if usable == 0 {
			return 0
		}
		return float64(d.Used) * 100 / float64(usable)
	}
	if d.Total == 0 {
		return 0
	}
	return float64(d.Used) * 100 / float64(d.Total)
}

type Metrics struct {
	Online             bool
	OS                 string
	RemoteHostname     string
	Kernel             string
	Arch               string
	CPUPercent         float64
	CPUCores           int
	CPUModel           string
	MemUsed            uint64
	MemTotal           uint64
	MemAvailable       uint64
	SwapUsed           uint64
	SwapTotal          uint64
	SwapFree           uint64
	DiskUsed           uint64
	DiskTotal          uint64
	DiskAvailable      uint64
	DiskAvailKnown     bool
	DiskFilesystem     string
	DiskType           string
	DiskMountpoint     string
	Disks              []DiskMetric
	InodeUsed          uint64
	InodeTotal         uint64
	InodeAvailable     uint64
	Load1              string
	Load5              string
	Load15             string
	Uptime             string
	DockerRunning      int
	DockerTotal        int
	DockerStopped      int
	DockerFailed       int
	DockerRunningNames []string
	DockerStoppedNames []string
	DockerFailedNames  []string
	FailedServices     int
	FailedUnits        []string
	Ports              string
	HealthPorts        []HealthPort
	Error              string
	UpdatedAt          time.Time
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

func (m Metrics) SwapPercent() float64 {
	if m.SwapTotal == 0 {
		return 0
	}
	return float64(m.SwapUsed) * 100 / float64(m.SwapTotal)
}

func (m Metrics) InodePercent() float64 {
	usable := m.InodeUsed + m.InodeAvailable
	if usable > 0 {
		return float64(m.InodeUsed) * 100 / float64(usable)
	}
	if m.InodeTotal == 0 {
		return 0
	}
	return float64(m.InodeUsed) * 100 / float64(m.InodeTotal)
}

func (m Metrics) HealthTotal() int {
	return len(m.HealthPorts)
}

func (m Metrics) HealthOK() int {
	total := 0
	for _, port := range m.HealthPorts {
		if port.Healthy {
			total++
		}
	}
	return total
}

func (m Metrics) DockerNonRunningCount() int {
	if m.DockerTotal <= 0 {
		return 0
	}
	problems := m.DockerTotal - m.DockerRunning
	if problems < 0 {
		return 0
	}
	return problems
}
