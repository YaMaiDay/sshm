package tui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/dbmonitor"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

type viewMode int

const (
	modeDashboard viewMode = iota
	modeDetail
	modeAddForm
	modeDeleteConfirm
	modeConfirmAction
	modePickLocalRoot
	modePickLocalItem
	modePickRemoteDir
	modePickRemoteItem
	modePickSaveDir
	modeTransferPanel
	modeTransferJobs
	modeTransferDetail
	modeCommandList
	modeCommandEdit
	modeCommandConfirm
	modeCommandOutput
	modeBatchSelect
	modeBatchCommandList
	modeBatchCommandEdit
	modeBatchConfirm
	modeBatchOutput
	modeCommandHistory
	modeCommandHistoryDetail
	modeAnomalyOverview
	modeDeploymentList
	modeDeploymentDetail
	modeDeploymentEdit
	modeDeploymentConfirm
	modeDeploymentRollbackConfirm
	modeDeploymentOutput
	modeSettings
	modeHelp
	modeResourceList
	modeResourceDetail
	modeResourceAdd
	modeResourceAddEdit
	modeResourceLog
	modeResourceCommandEdit
	modeResourceConfirm
	modeResourceOutput
)

type transferMode int

const (
	transferNone transferMode = iota
	transferUpload
	transferDownload
)

type commandScope int

const (
	commandScopeGlobal commandScope = iota
	commandScopeServer
)

type filterMode int

const (
	filterAll filterMode = iota
	filterOnline
	filterProblem
)

type sortMode int

const (
	sortDefault sortMode = iota
	sortState
	sortCPU
	sortMem
	sortDisk
)

type dashboardMode int

const (
	dashboardCards dashboardMode = iota
	dashboardCategory
	dashboardGrouped
)

type deploymentViewMode int

const (
	deploymentViewCards deploymentViewMode = iota
	deploymentViewList
)

type anomalyFilterMode int

const (
	anomalyAll anomalyFilterMode = iota
	anomalySevere
	anomalyWarn
	anomalyOffline
	anomalyResource
	anomalyContainer
	anomalyService
	anomalySecurity
)

type resourceKind int

const (
	resourceAll resourceKind = iota
	resourceContainers
	resourceServices
	resourceProcesses
	resourcePorts
	resourceDatabases
)

type resourceViewMode int

const (
	resourceViewCards resourceViewMode = iota
	resourceViewList
)

type resourceScopeMode int

const (
	resourceScopeDiscovered resourceScopeMode = iota
	resourceScopeManaged
)

type resourceFilterMode int

const (
	resourceFilterAll resourceFilterMode = iota
	resourceFilterRunning
	resourceFilterProblems
	resourceFilterStopped
)

type resourceSortMode int

const (
	resourceSortDefault resourceSortMode = iota
	resourceSortStatus
	resourceSortName
	resourceSortCPU
	resourceSortMemory
	resourceSortPort
)

type resourcePortFilterMode int

const (
	resourcePortFilterAll resourcePortFilterMode = iota
	resourcePortFilterPublic
	resourcePortFilterLoopback
	resourcePortFilterSpecific
	resourcePortFilterContainer
	resourcePortFilterProcess
)

type resourceActionKind int

const (
	resourceActionNone resourceActionKind = iota
	resourceActionStart
	resourceActionStop
	resourceActionRestart
	resourceActionDelete
)

type resourceRef struct {
	Kind  resourceKind
	Index int
}

const (
	dashboardCardInnerHeight  = 7
	dashboardCardTotalHeight  = dashboardCardInnerHeight + 2
	deploymentCardInnerHeight = 6
	resourceCardInnerHeight   = 4
)

type hostState struct {
	Host               host.Host
	Metrics            monitor.Metrics
	Loading            bool
	FailureCount       int
	LastAttempt        time.Time
	LoginLoading       bool
	LoginSummary       []string
	LoginError         string
	FailedLoginSummary []string
	FailedLoginError   string
	SSHDSecurity       map[string]string
	SSHDSecurityError  string
	ServiceDetails     []serviceDetail
	ServiceError       string
	PortDetails        []portDetail
	PortDetailsError   string
	ContainerDetails   []containerDetail
	ContainerError     string
	DatabaseDetails    []databaseDetail
	DatabaseError      string
}

type collectMsg struct {
	Index   int
	Round   int
	Metrics monitor.Metrics
	Manual  bool
}

type tickMsg time.Time

type transferDoneMsg struct {
	ID     string
	Kind   string
	Source string
	Target string
	Err    error
	Output string
}

type rsyncCheckMsg struct {
	HostIndex int
	Missing   bool
	ErrText   string
}

type rsyncInstallMsg struct {
	HostIndex int
	ErrText   string
}

type transferProgressMsg time.Time

type clearStatusMsg struct{}

type sshDoneMsg struct {
	Index int
	Err   error
}

type loginRecordsMsg struct {
	Index         int
	Summary       []string
	ErrText       string
	FailedSummary []string
	FailedErrText string
	SSHDSecurity  map[string]string
	SSHDErrText   string
}

type commandDoneMsg struct {
	Result actions.CommandResult
}

type batchCommandDoneMsg struct {
	Job    int
	Result actions.CommandResult
}

type deploymentDoneMsg struct {
	ID              string
	Result          actions.CommandResult
	PreviousVersion string
	CurrentVersion  string
}

type deploymentQueueNextMsg struct{}

type deploymentProgressMsg struct {
	ID     string
	Output string
	Done   bool
}

type resourceLoadMsg struct {
	Index        int
	Kind         resourceKind
	Requested    resourceKind
	Services     []serviceDetail
	ServiceErr   string
	Containers   []containerDetail
	ContainerErr string
	Ports        []portDetail
	PortsErrText string
}

type resourceContainerDetailMsg struct {
	Index  int
	Name   string
	Detail containerExtraDetail
	Err    string
}

type resourceServiceDetailMsg struct {
	Index  int
	Name   string
	Detail serviceDetail
	Err    string
}

type resourceProcessDetailMsg struct {
	Index  int
	PID    string
	Detail processExtraDetail
	Err    string
}

type resourceDatabaseDetailMsg struct {
	Index  int
	Name   string
	Detail databaseExtraDetail
	Err    string
}

type resourceLogMsg struct {
	Index  int
	Kind   resourceKind
	Name   string
	Output string
	Result actions.CommandResult
}

type resourceActionMsg struct {
	Index  int
	Kind   resourceKind
	Action resourceActionKind
	Name   string
	Result actions.CommandResult
}

type activeTransfer struct {
	ID         string
	Kind       string
	Source     string
	Target     string
	LocalPath  string
	RemotePath string
	HostIndex  int
	Total      int64
	Active     bool
	Cancel     context.CancelFunc
}

type Model struct {
	states                        []hostState
	selected                      int
	width                         int
	height                        int
	searching                     bool
	query                         string
	status                        string
	refreshStatus                 string
	collector                     monitor.Collector
	passwords                     config.PasswordStore
	appConfig                     config.AppConfig
	appState                      config.AppState
	home                          string
	mode                          viewMode
	transfer                      transferMode
	pickIndex                     int
	pickTitle                     string
	choices                       []choice
	remoteTree                    remoteTree
	pending                       pendingTransfer
	panel                         transferPanel
	form                          addForm
	formIndex                     int
	formCursor                    int
	formPane                      int
	categories                    []string
	categoryIndex                 int
	addingCategory                bool
	renamingCategory              bool
	categoryDraft                 string
	editing                       bool
	copying                       bool
	editIndex                     int
	deleteIndex                   int
	confirm                       confirmAction
	filter                        filterMode
	sortBy                        sortMode
	dashboardMode                 dashboardMode
	dashboardFocus                int
	category                      string
	favoriteOnly                  bool
	detailScroll                  int
	detailSectionIndex            int
	activeTransfer                activeTransfer
	transferHistory               config.TransferHistoryFile
	transferIndex                 int
	transferStatusFilter          int
	transferRunAll                bool
	commandFile                   config.CommandsFile
	commandItems                  []commandItem
	commandIndex                  int
	commandForm                   commandEditForm
	commandField                  int
	commandCursor                 int
	commandEditing                bool
	commandEditItem               commandItem
	commandConfirm                commandItem
	commandOutputScroll           int
	commandOutputBack             viewMode
	activeCommand                 activeCommand
	batchIndexes                  []int
	batchSelected                 map[int]bool
	batchCursor                   int
	batchCommandItems             []commandItem
	batchCommandIndex             int
	batchCommand                  commandItem
	batchJobs                     []batchJob
	batchCurrent                  int
	batchOutputIndex              int
	batchOutputScroll             int
	batchOutputBack               viewMode
	commandHistory                config.CommandHistoryFile
	historyIndex                  int
	historyScroll                 int
	historySearch                 bool
	historyQuery                  string
	deploymentFile                config.DeploymentsFile
	resourceFile                  config.ResourcesFile
	deploymentItems               []deploymentItem
	deploymentIndex               int
	deploymentForm                deploymentForm
	deploymentField               int
	deploymentCursor              int
	deploymentEditing             bool
	deploymentEditIndex           int
	deploymentDetail              config.DeploymentApp
	deploymentConfirm             config.DeploymentApp
	deploymentConfirmQueue        []config.DeploymentApp
	deploymentSelected            []int
	deploymentCategory            string
	deploymentView                deploymentViewMode
	deploymentFavoriteOnly        bool
	activeDeployment              activeDeployment
	deploymentOutputScroll        int
	settingsForm                  settingsForm
	settingsField                 int
	settingsCursor                int
	anomalyIndex                  int
	anomalyFilter                 anomalyFilterMode
	transferJobsBack              viewMode
	helpBackMode                  viewMode
	collectRound                  int
	manualRound                   int
	pendingByRound                map[int]int
	resourceHostIndex             int
	resourceBackMode              viewMode
	resourceKind                  resourceKind
	resourceScope                 resourceScopeMode
	resourceView                  resourceViewMode
	resourceFilter                resourceFilterMode
	resourceSort                  resourceSortMode
	resourcePortFilter            resourcePortFilterMode
	resourceIndex                 int
	resourceScroll                int
	resourceDetailKind            resourceKind
	resourceDetailName            string
	resourceSearch                bool
	resourceQuery                 string
	resourceLoading               bool
	resourceLoadingKind           resourceKind
	resourceLoadingPending        int
	resourceManualRefresh         bool
	resourceRefreshStatus         string
	resourceCollectedAt           time.Time
	resourceContainerAt           time.Time
	resourceServiceAt             time.Time
	resourcePortAt                time.Time
	resourceAction                resourceActionKind
	resourceActionResource        resourceKind
	resourceActionName            string
	resourceActionOutput          string
	resourceActionExitCode        int
	resourceActionRunning         bool
	resourceLogName               string
	resourceLogKind               resourceKind
	resourceLogOutput             string
	resourceLogScroll             int
	resourceAddKind               resourceKind
	resourceAddName               string
	resourceAddField              int
	resourceAddCursor             int
	resourceManagePane            int
	resourceManageDiscoveredIndex int
	resourceManageFavoriteIndex   int
	resourceManageSearch          bool
	resourceManageQuery           string
	resourceCommandForm           resourceCommandForm
	resourceCommandBackMode       viewMode
	resourceCommandField          int
	resourceCommandCursor         int
	resourceContainerExtraName    string
	resourceContainerExtra        containerExtraDetail
	resourceContainerExtraLoading bool
	resourceContainerExtraErr     string
	resourceServiceExtraName      string
	resourceServiceExtra          serviceDetail
	resourceServiceExtraLoading   bool
	resourceServiceExtraErr       string
	resourceProcessExtraPID       string
	resourceProcessExtra          processExtraDetail
	resourceProcessExtraLoading   bool
	resourceProcessExtraErr       string
	resourceDatabaseExtraName     string
	resourceDatabaseExtra         databaseExtraDetail
	resourceDatabaseExtraLoading  bool
	resourceDatabaseExtraErr      string
	resourceDatabaseExtraCache    map[string]databaseExtraCache
}

type databaseExtraCache struct {
	Detail  databaseExtraDetail
	Err     string
	Loading bool
}

type resourceCommandForm struct {
	Server         string
	Kind           resourceKind
	Name           string
	StartCommand   string
	StopCommand    string
	RestartCommand string
	DeleteCommand  string
	LogCommand     string
	HealthCommand  string
	DBEngine       string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBInstance     string
	DBNote         string
	DBSource       string
	DBStatus       string
	DBRawStatus    string
	DBEndpoint     string
	DBContainer    string
	DBImage        string
	DBServiceUnit  string
	DBProcess      string
	DBPID          string
}

type choice struct {
	Label string
	Value string
	IsDir bool
	Depth int
}

type remoteTree struct {
	HostIndex int
	Local     bool
	DirsOnly  bool
	Roots     []string
	Nodes     map[string]*remoteTreeNode
}

type remoteTreeNode struct {
	Item     fsselect.Item
	Depth    int
	Loaded   bool
	Expanded bool
	Children []string
}

type transferPanel struct {
	Mode         transferMode
	HostIndex    int
	ActivePane   int
	LeftTitle    string
	RightTitle   string
	LeftTree     remoteTree
	RightTree    remoteTree
	LeftChoices  []choice
	RightChoices []choice
	LeftSelected map[string]bool
	LeftIndex    int
	RightIndex   int
	Confirming   bool
	NeedsInstall bool
}

type pendingTransfer struct {
	HostIndex   int
	LocalRoot   string
	LocalPath   string
	LocalIsDir  bool
	RemoteDir   string
	RemotePath  string
	RemoteIsDir bool
	SaveDir     string
}

type addForm struct {
	Category     string
	Name         string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	Password     string
	JumpHostRef  string
	ExpireAt     string
	Note         string
}

const (
	categoryFormIndex    = 0
	nameFormIndex        = 1
	hostFormIndex        = 2
	userFormIndex        = 3
	portFormIndex        = 4
	identityFormIndex    = 5
	passwordFormIndex    = 6
	jumpHostRefFormIndex = 7
	noteFormIndex        = 8
	expireAtFormIndex    = 9
)

type formField struct {
	ID      int
	Label   string
	Value   string
	Section bool
}

type commandItem struct {
	Scope     commandScope
	Index     int
	Name      string
	Command   string
	Server    string
	Header    bool
	Spacer    bool
	Temporary bool
}

type commandEditForm struct {
	Scope   commandScope
	Name    string
	Command string
}

type deploymentItem struct {
	Index  int
	App    config.DeploymentApp
	Header bool
	Spacer bool
}

type deploymentForm struct {
	Name             string
	Server           string
	Source           string
	FetchMode        string
	Repo             string
	Branch           string
	Version          string
	Asset            string
	Path             string
	ReleaseURL       string
	Credential       string
	CredentialName   string
	WaitSeconds      string
	BeforeCommands   string
	ResourceCommands string
	UpdateCommands   string
	AfterCommands    string
	HealthCommands   string
	RollbackCommands string
}

type activeDeployment struct {
	HostIndex       int
	App             config.DeploymentApp
	Action          string
	ProgressID      string
	Output          string
	ExitCode        int
	Running         bool
	PreviousVersion string
	CurrentVersion  string
	Queue           []config.DeploymentApp
	QueueIndex      int
	QueueFailed     int
}

type deploymentProgressState struct {
	Output string
	Done   bool
}

var deploymentProgressStore = struct {
	sync.Mutex
	items map[string]deploymentProgressState
}{items: map[string]deploymentProgressState{}}

func deploymentProgressStart(id string) {
	if id == "" {
		return
	}
	deploymentProgressStore.Lock()
	deploymentProgressStore.items[id] = deploymentProgressState{}
	deploymentProgressStore.Unlock()
}

func deploymentProgressAppend(id string, text string) {
	if id == "" || text == "" {
		return
	}
	deploymentProgressStore.Lock()
	state := deploymentProgressStore.items[id]
	state.Output += text
	deploymentProgressStore.items[id] = state
	deploymentProgressStore.Unlock()
}

func deploymentProgressFinish(id string, output string) {
	if id == "" {
		return
	}
	deploymentProgressStore.Lock()
	state := deploymentProgressStore.items[id]
	state.Output = output
	state.Done = true
	deploymentProgressStore.items[id] = state
	deploymentProgressStore.Unlock()
}

func deploymentProgressSnapshot(id string) deploymentProgressState {
	deploymentProgressStore.Lock()
	defer deploymentProgressStore.Unlock()
	return deploymentProgressStore.items[id]
}

func deploymentProgressClear(id string) {
	if id == "" {
		return
	}
	deploymentProgressStore.Lock()
	delete(deploymentProgressStore.items, id)
	deploymentProgressStore.Unlock()
}

func deploymentProgressAfter(id string, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(delay)
		state := deploymentProgressSnapshot(id)
		return deploymentProgressMsg{ID: id, Output: state.Output, Done: state.Done}
	}
}

type activeCommand struct {
	HostIndex int
	Name      string
	Command   string
	Output    string
	ExitCode  int
	Running   bool
}

type batchJob struct {
	HostIndex int
	Output    string
	ExitCode  int
	Err       error
	Running   bool
	Done      bool
}

type portDetail struct {
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

type serviceDetail struct {
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

type processExtraDetail struct {
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

type containerDetail struct {
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

type databaseDetail struct {
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

type databaseExtraDetail = dbmonitor.Detail

type databaseTableSize = dbmonitor.TableSize

type containerMountDetail struct {
	Type        string
	Source      string
	Destination string
	RW          bool
}

type containerNetworkDetail struct {
	Name       string
	IPAddress  string
	Gateway    string
	MacAddress string
	NetworkID  string
	EndpointID string
	Aliases    []string
}

type containerExtraDetail struct {
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
	Mounts        []containerMountDetail
	Networks      []containerNetworkDetail
}

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmDeleteCategory
	confirmDeleteCommand
	confirmDeleteHistory
	confirmDeleteDeployment
	confirmRemoveResource
)

type confirmAction struct {
	Kind       confirmKind
	Title      string
	Lines      []string
	Back       viewMode
	Command    commandItem
	History    config.CommandHistoryEntry
	Deployment config.DeploymentApp
	Resource   config.ManagedResource
	Index      int
	Value      string
}
