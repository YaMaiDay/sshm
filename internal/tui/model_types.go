package tui

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/execresult"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
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
	ServiceDetails     []resourceservice.ServiceDetail
	ServiceError       string
	PortDetails        []resourceservice.PortDetail
	PortDetailsError   string
	ContainerDetails   []resourceservice.ContainerDetail
	ContainerError     string
	DatabaseDetails    []resourceservice.DatabaseDetail
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

type commandResult = execresult.Result

type commandDoneMsg struct {
	Result commandResult
}

type batchCommandDoneMsg struct {
	Job    int
	Result commandResult
}

type deploymentDoneMsg struct {
	ID              string
	Result          commandResult
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
	Services     []resourceservice.ServiceDetail
	ServiceErr   string
	Containers   []resourceservice.ContainerDetail
	ContainerErr string
	Ports        []resourceservice.PortDetail
	PortsErrText string
}

type resourceContainerDetailMsg struct {
	Index  int
	Name   string
	Detail resourceservice.ContainerExtraDetail
	Err    string
}

type resourceServiceDetailMsg struct {
	Index  int
	Name   string
	Detail resourceservice.ServiceDetail
	Err    string
}

type resourceProcessDetailMsg struct {
	Index  int
	PID    string
	Detail resourceservice.ProcessExtraDetail
	Err    string
}

type resourceDatabaseDetailMsg struct {
	Index  int
	Name   string
	Detail resourceservice.DatabaseExtraDetail
	Err    string
}

type resourceLogMsg struct {
	Index  int
	Kind   resourceKind
	Name   string
	Output string
	Result commandResult
}

type resourceActionMsg struct {
	Index  int
	Kind   resourceKind
	Action resourceActionKind
	Name   string
	Result commandResult
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

type transferState struct {
	Mode         transferMode
	PickIndex    int
	PickTitle    string
	Choices      []choice
	RemoteTree   remoteTree
	Pending      pendingTransfer
	Panel        transferPanel
	Active       activeTransfer
	History      config.TransferHistoryFile
	Index        int
	StatusFilter int
	RunAll       bool
	JobsBack     viewMode
}

type settingsState struct {
	Form   settingsForm
	Field  int
	Cursor int
}

type dashboardState struct {
	Mode  dashboardMode
	Focus int
}

type anomalyState struct {
	Index  int
	Filter anomalyFilterMode
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
	transferState                 transferState
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
	dashboard                     dashboardState
	category                      string
	favoriteOnly                  bool
	detailScroll                  int
	detailSectionIndex            int
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
	deploymentProgress            *deploymentProgressStore
	deploymentOutputScroll        int
	settings                      settingsState
	anomaly                       anomalyState
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
	resourceContainerExtra        resourceservice.ContainerExtraDetail
	resourceContainerExtraLoading bool
	resourceContainerExtraErr     string
	resourceServiceExtraName      string
	resourceServiceExtra          resourceservice.ServiceDetail
	resourceServiceExtraLoading   bool
	resourceServiceExtraErr       string
	resourceProcessExtraPID       string
	resourceProcessExtra          resourceservice.ProcessExtraDetail
	resourceProcessExtraLoading   bool
	resourceProcessExtraErr       string
	resourceDatabaseExtraName     string
	resourceDatabaseExtra         resourceservice.DatabaseExtraDetail
	resourceDatabaseExtraLoading  bool
	resourceDatabaseExtraErr      string
	resourceDatabaseExtraCache    map[string]databaseExtraCache
}

type databaseExtraCache struct {
	Detail  resourceservice.DatabaseExtraDetail
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

type deploymentProgressStore struct {
	sync.Mutex
	items map[string]deploymentProgressState
}

func newDeploymentProgressStore() *deploymentProgressStore {
	return &deploymentProgressStore{items: map[string]deploymentProgressState{}}
}

func (s *deploymentProgressStore) ensure() {
	if s.items == nil {
		s.items = map[string]deploymentProgressState{}
	}
}

func (s *deploymentProgressStore) start(id string) {
	if id == "" {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.ensure()
	s.items[id] = deploymentProgressState{}
}

func (s *deploymentProgressStore) append(id string, text string) {
	if id == "" || text == "" {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.ensure()
	state := s.items[id]
	state.Output += text
	s.items[id] = state
}

func (s *deploymentProgressStore) finish(id string, output string) {
	if id == "" {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.ensure()
	state := s.items[id]
	state.Output = output
	state.Done = true
	s.items[id] = state
}

func (s *deploymentProgressStore) snapshot(id string) deploymentProgressState {
	s.Lock()
	defer s.Unlock()
	s.ensure()
	return s.items[id]
}

func (s *deploymentProgressStore) clear(id string) {
	if id == "" {
		return
	}
	s.Lock()
	defer s.Unlock()
	s.ensure()
	delete(s.items, id)
}

func deploymentProgressAfter(store *deploymentProgressStore, id string, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(delay)
		state := store.snapshot(id)
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
