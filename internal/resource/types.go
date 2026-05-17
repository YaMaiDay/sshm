package resource

import "github.com/YaMaiDay/sshm/internal/dbmonitor"

type PortDetail struct {
	Protocol        string
	Port            string
	LocalAddress    string
	ForeignAddress  string
	State           string
	Process         string
	PID             string
	FD              string
	ServiceUnit     string
	Container       string
	ContainerPort   string
	Count           int
	Managed         bool
	Favorite        bool
	ProcessManaged  bool
	ProcessFavorite bool
	Missing         bool
}

type ServiceDetail struct {
	Unit             string
	Load             string
	Active           string
	Sub              string
	Description      string
	FragmentPath     string
	WorkingDirectory string
	ExecStart        string
	MainPID          string
	ExecMainPID      string
	MemoryCurrent    uint64
	ActiveSince      string
	InactiveSince    string
	StateChangedAt   string
	ExecStartedAt    string
	ExecExitedAt     string
	UnitFileState    string
	Result           string
	ExecMainStatus   string
	NRestarts        string
	TasksCurrent     string
	ControlGroup     string
	Slice            string
	User             string
	Group            string
	Restart          string
	RestartSec       string
	ExecStop         string
	ExecReload       string
	DropInPaths      string
	Managed          bool
	Favorite         bool
	Missing          bool
}

type ProcessExtraDetail struct {
	PID          string
	PPID         string
	User         string
	State        string
	CPU          string
	Memory       string
	RSS          string
	Elapsed      string
	Started      string
	Command      string
	CommandLine  string
	WorkingDir   string
	Executable   string
	ControlGroup string
	ServiceUnit  string
}

type ContainerDetail struct {
	Name          string
	Image         string
	Status        string
	Ports         string
	CPU           string
	Memory        string
	MemPerc       string
	CPULimitKnown bool
	NanoCpus      int64
	CPUQuota      int64
	CPUPeriod     int64
	CpusetCpus    string
	Managed       bool
	Favorite      bool
	Missing       bool
}

type DatabaseDetail struct {
	Name        string
	Engine      string
	Source      string
	Status      string
	RawStatus   string
	Endpoint    string
	ServiceUnit string
	Container   string
	Image       string
	Process     string
	PID         string
	Protocol    string
	Port        string
	Managed     bool
	Favorite    bool
	Missing     bool
	Configured  bool
}

type DatabaseExtraDetail = dbmonitor.Detail

type DatabaseTableSize = dbmonitor.TableSize

type ContainerMountDetail struct {
	Type        string
	Source      string
	Destination string
	RW          bool
}

type ContainerNetworkDetail struct {
	Name       string
	IPAddress  string
	Gateway    string
	MacAddress string
	NetworkID  string
	EndpointID string
	Aliases    []string
}

type ContainerExtraDetail struct {
	ID            string
	Created       string
	Path          string
	Args          []string
	Driver        string
	Platform      string
	RestartPolicy string
	NanoCpus      int64
	CPUQuota      int64
	CPUPeriod     int64
	CpusetCpus    string
	StateStatus   string
	StartedAt     string
	FinishedAt    string
	ExitCode      int
	HealthStatus  string
	Size          string
	VirtualSize   string
	SizeRW        uint64
	SizeRootFS    uint64
	BlockIO       string
	Mounts        []ContainerMountDetail
	Networks      []ContainerNetworkDetail
}
