package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
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
	modeHelp
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

const (
	dashboardCardInnerHeight  = 7
	dashboardCardTotalHeight  = dashboardCardInnerHeight + 2
	deploymentCardInnerHeight = 6
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
	PortDetails        []portDetail
	PortDetailsError   string
	ContainerDetails   []containerDetail
	ContainerError     string
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
	Ports         []portDetail
	PortsErrText  string
	Containers    []containerDetail
	ContainerErr  string
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
	states                 []hostState
	selected               int
	width                  int
	height                 int
	searching              bool
	query                  string
	status                 string
	refreshStatus          string
	collector              monitor.Collector
	passwords              config.PasswordStore
	appConfig              config.AppConfig
	appState               config.AppState
	home                   string
	mode                   viewMode
	transfer               transferMode
	pickIndex              int
	pickTitle              string
	choices                []choice
	remoteTree             remoteTree
	pending                pendingTransfer
	panel                  transferPanel
	form                   addForm
	formIndex              int
	formCursor             int
	formPane               int
	categories             []string
	categoryIndex          int
	addingCategory         bool
	renamingCategory       bool
	categoryDraft          string
	editing                bool
	copying                bool
	editIndex              int
	deleteIndex            int
	confirm                confirmAction
	filter                 filterMode
	sortBy                 sortMode
	dashboardMode          dashboardMode
	dashboardFocus         int
	category               string
	favoriteOnly           bool
	detailScroll           int
	detailSectionIndex     int
	activeTransfer         activeTransfer
	transferHistory        config.TransferHistoryFile
	transferIndex          int
	transferStatusFilter   int
	transferRunAll         bool
	commandFile            config.CommandsFile
	commandItems           []commandItem
	commandIndex           int
	commandForm            commandEditForm
	commandField           int
	commandCursor          int
	commandEditing         bool
	commandEditItem        commandItem
	commandConfirm         commandItem
	commandOutputScroll    int
	commandOutputBack      viewMode
	activeCommand          activeCommand
	batchIndexes           []int
	batchSelected          map[int]bool
	batchCursor            int
	batchCommandItems      []commandItem
	batchCommandIndex      int
	batchCommand           commandItem
	batchJobs              []batchJob
	batchCurrent           int
	batchOutputIndex       int
	batchOutputScroll      int
	batchOutputBack        viewMode
	commandHistory         config.CommandHistoryFile
	historyIndex           int
	historyScroll          int
	historySearch          bool
	historyQuery           string
	deploymentFile         config.DeploymentsFile
	deploymentItems        []deploymentItem
	deploymentIndex        int
	deploymentForm         deploymentForm
	deploymentField        int
	deploymentCursor       int
	deploymentEditing      bool
	deploymentEditIndex    int
	deploymentDetail       config.DeploymentApp
	deploymentConfirm      config.DeploymentApp
	deploymentConfirmQueue []config.DeploymentApp
	deploymentSelected     []int
	deploymentCategory     string
	deploymentView         deploymentViewMode
	deploymentFavoriteOnly bool
	activeDeployment       activeDeployment
	deploymentOutputScroll int
	anomalyIndex           int
	anomalyFilter          anomalyFilterMode
	transferJobsBack       viewMode
	helpBackMode           viewMode
	collectRound           int
	manualRound            int
	pendingByRound         map[int]int
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
	HealthPorts  string
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
	healthPortsFormIndex = 8
	noteFormIndex        = 9
	expireAtFormIndex    = 10
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
	Protocol  string
	Port      string
	Process   string
	PID       string
	Container string
	Count     int
}

type containerDetail struct {
	Name   string
	Image  string
	Status string
	Ports  string
}

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmDeleteCategory
	confirmDeleteCommand
	confirmDeleteHistory
	confirmDeleteDeployment
)

type confirmAction struct {
	Kind       confirmKind
	Title      string
	Lines      []string
	Back       viewMode
	Command    commandItem
	History    config.CommandHistoryEntry
	Deployment config.DeploymentApp
	Index      int
	Value      string
}

func (f addForm) fields() []formField {
	fields := []formField{
		{Label: "基础信息", Section: true},
		{ID: categoryFormIndex, Label: "分类", Value: f.Category},
		{ID: nameFormIndex, Label: "服务器名称", Value: f.Name},
		{Label: "目标服务器", Section: true},
		{ID: hostFormIndex, Label: "服务器地址", Value: f.HostName},
		{ID: userFormIndex, Label: "用户名", Value: f.User},
		{ID: portFormIndex, Label: "端口", Value: f.Port},
		{ID: identityFormIndex, Label: "服务器本地密钥文件", Value: f.IdentityFile},
		{ID: passwordFormIndex, Label: "密码", Value: f.Password},
	}
	if f.Category != config.BastionCategory {
		fields = append(fields,
			formField{Label: "跳板机", Section: true},
			formField{ID: jumpHostRefFormIndex, Label: "使用跳板机", Value: emptyChoice(f.JumpHostRef, "无")},
		)
	}
	fields = append(fields,
		formField{Label: "辅助信息", Section: true},
		formField{ID: healthPortsFormIndex, Label: "健康端口", Value: f.HealthPorts},
		formField{ID: noteFormIndex, Label: "备注", Value: f.Note},
		formField{ID: expireAtFormIndex, Label: "到期时间", Value: f.ExpireAt},
	)
	return fields
}

func New(hosts []host.Host, passwords config.PasswordStore) Model {
	home, _ := os.UserHomeDir()
	appConfig := config.LoadAppConfig(home)
	appState := config.LoadState(home)
	categories, _, _ := config.LoadCategories(home)
	commandFile, _, _ := config.LoadCommands(home)
	deploymentFile, _, _ := config.LoadDeployments(home)
	_ = config.MarkRunningTransfersInterrupted(home)
	transferHistory, _, _ := config.LoadTransfers(home)
	states := make([]hostState, len(hosts))
	for i, h := range hosts {
		states[i] = hostState{Host: h, Loading: true}
	}
	pendingByRound := map[int]int{1: len(states)}
	collector := monitor.NewCollector(passwords)
	collector.Timeout = appConfig.CommandDuration()
	collector.ConnectTimeout = appConfig.ConnectDuration()
	return Model{
		states:          states,
		collector:       collector,
		passwords:       passwords,
		appConfig:       appConfig,
		appState:        appState,
		home:            home,
		commandFile:     commandFile,
		deploymentFile:  deploymentFile,
		transferHistory: transferHistory,
		categories:      categories,
		status:          "正在采集服务器状态...",
		collectRound:    1,
		pendingByRound:  pendingByRound,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.collectAll(m.collectRound, false), tickAfter(m.appConfig.RefreshDuration()))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, tea.Batch(m.collectAll(m.collectRound, false), tickAfter(m.appConfig.RefreshDuration()))
	case collectMsg:
		if msg.Round != m.collectRound {
			return m, nil
		}
		m.applyMetrics(msg.Index, msg.Metrics)
		m.pendingByRound[msg.Round]--
		if m.pendingByRound[msg.Round] <= 0 {
			delete(m.pendingByRound, msg.Round)
			if msg.Manual && msg.Round == m.manualRound {
				m.refreshStatus = fmt.Sprintf("手动刷新完成：%s", time.Now().Format("15:04:05"))
				if !m.activeTransfer.Active {
					m.status = m.refreshStatus
				}
			} else {
				m.refreshStatus = fmt.Sprintf("最后刷新：%s", time.Now().Format("15:04:05"))
				if m.status == "正在采集服务器状态..." {
					m.status = ""
				}
			}
		}
		return m, nil
	case transferDoneMsg:
		m.activeTransfer.Active = false
		m.activeTransfer.Cancel = nil
		m.updateTransferEntryDone(msg)
		m.reloadTransfers()
		if status, ok := m.transferEntryStatus(msg.ID); ok {
			if status == config.TransferStatusInterrupted {
				m.status = msg.Kind + "已中断。"
				return m, clearStatusAfter(3 * time.Second)
			}
			if status == config.TransferStatusCanceled {
				m.status = msg.Kind + "已取消。"
				return m, clearStatusAfter(3 * time.Second)
			}
		}
		if msg.Err != nil {
			m.status = msg.Kind + "失败：" + transferErrorText(msg.Err, msg.Output)
			if m.transferRunAll {
				return m.startNextQueuedTransfer()
			}
			return m, clearStatusAfter(3 * time.Second)
		} else {
			m.status = fmt.Sprintf("%s完成：%s -> %s", msg.Kind, filepath.Base(msg.Source), msg.Target)
			if m.transferRunAll {
				return m.startNextQueuedTransfer()
			}
			return m, clearStatusAfter(3 * time.Second)
		}
	case rsyncCheckMsg:
		if msg.Missing {
			m.panel.NeedsInstall = true
			m.status = "远程未安装 rsync。按 i 尝试安装并继续，Esc 取消。"
			return m, nil
		}
		if msg.ErrText != "" {
			m.status = "检测 rsync 失败：" + msg.ErrText
			return m, nil
		}
		return m.createTransferJobsFromPanel()
	case rsyncInstallMsg:
		if msg.ErrText != "" {
			m.status = "安装 rsync 失败：" + msg.ErrText
			return m, nil
		}
		m.panel.NeedsInstall = false
		m.status = "rsync 安装成功，开始传输。"
		return m.createTransferJobsFromPanel()
	case transferProgressMsg:
		if !m.activeTransfer.Active {
			return m, nil
		}
		m.reloadTransfers()
		m.status = transferProgressText(m.activeTransfer, m.states)
		return m, transferProgressAfter(500 * time.Millisecond)
	case clearStatusMsg:
		if !m.activeTransfer.Active {
			m.status = ""
		}
		return m, nil
	case sshDoneMsg:
		if msg.Err != nil {
			m.status = fmt.Sprintf("登录退出：%v", msg.Err)
			return m, tea.Batch(clearScreen(), clearStatusAfter(3*time.Second))
		}
		if msg.Index >= 0 && msg.Index < len(m.states) {
			m.recordLastLogin(m.states[msg.Index].Host, time.Now())
		}
		m.status = "已返回监控面板"
		return m, tea.Batch(clearScreen(), clearStatusAfter(2*time.Second))
	case loginRecordsMsg:
		if msg.Index < 0 || msg.Index >= len(m.states) {
			return m, nil
		}
		m.states[msg.Index].LoginLoading = false
		m.states[msg.Index].LoginSummary = msg.Summary
		m.states[msg.Index].LoginError = msg.ErrText
		m.states[msg.Index].FailedLoginSummary = msg.FailedSummary
		m.states[msg.Index].FailedLoginError = msg.FailedErrText
		m.states[msg.Index].SSHDSecurity = msg.SSHDSecurity
		m.states[msg.Index].SSHDSecurityError = msg.SSHDErrText
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.states[msg.Index].ContainerDetails = msg.Containers
		m.states[msg.Index].ContainerError = msg.ContainerErr
		return m, nil
	case commandDoneMsg:
		m.activeCommand.Running = false
		m.activeCommand.Output = msg.Result.Output
		m.activeCommand.ExitCode = msg.Result.ExitCode
		historyErr := m.recordCommandHistory(msg.Result)
		if msg.Result.Err != nil {
			m.status = fmt.Sprintf("命令执行失败：退出码 %d", msg.Result.ExitCode)
		} else {
			m.status = "命令执行完成。"
		}
		if historyErr != nil {
			m.status += " 历史保存失败：" + historyErr.Error()
		}
		return m, nil
	case batchCommandDoneMsg:
		return m.handleBatchCommandDone(msg)
	case deploymentDoneMsg:
		return m.handleDeploymentDone(msg)
	case deploymentQueueNextMsg:
		return m.startNextQueuedDeployment()
	case deploymentProgressMsg:
		return m.handleDeploymentProgress(msg)
	case tea.KeyMsg:
		switch m.mode {
		case modeCommandList:
			return m.updateCommandList(msg)
		case modeCommandEdit:
			return m.updateCommandEdit(msg)
		case modeCommandConfirm:
			return m.updateCommandConfirm(msg)
		case modeCommandOutput:
			return m.updateCommandOutput(msg)
		case modeBatchSelect:
			return m.updateBatchSelect(msg)
		case modeBatchCommandList:
			return m.updateBatchCommandList(msg)
		case modeBatchCommandEdit:
			return m.updateBatchCommandEdit(msg)
		case modeBatchConfirm:
			return m.updateBatchConfirm(msg)
		case modeBatchOutput:
			return m.updateBatchOutput(msg)
		case modeCommandHistory:
			return m.updateCommandHistory(msg)
		case modeCommandHistoryDetail:
			return m.updateCommandHistoryDetail(msg)
		case modeAnomalyOverview:
			return m.updateAnomalyOverview(msg)
		case modeDeploymentList:
			return m.updateDeploymentList(msg)
		case modeDeploymentDetail:
			return m.updateDeploymentDetail(msg)
		case modeDeploymentEdit:
			return m.updateDeploymentEdit(msg)
		case modeDeploymentConfirm:
			return m.updateDeploymentConfirm(msg)
		case modeDeploymentRollbackConfirm:
			return m.updateDeploymentRollbackConfirm(msg)
		case modeDeploymentOutput:
			return m.updateDeploymentOutput(msg)
		case modeTransferJobs:
			return m.updateTransferJobs(msg)
		case modeTransferDetail:
			return m.updateTransferDetail(msg)
		case modeHelp:
			return m.updateHelpPanel(msg)
		}
		if m.mode == modeAddForm {
			return m.updateAddForm(msg)
		}
		if m.mode == modeDeleteConfirm {
			return m.updateDeleteConfirm(msg)
		}
		if m.mode == modeConfirmAction {
			return m.updateConfirmAction(msg)
		}
		if m.mode == modeTransferPanel {
			return m.updateTransferPanel(msg)
		}
		if m.mode != modeDashboard && m.mode != modeDetail {
			return m.updatePicker(msg)
		}
		if m.mode == modeDetail {
			return m.updateDetail(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		key := shortcutKey(msg)
		switch key {
		case "q", "esc", "ctrl+c":
			if m.activeTransfer.Active && m.activeTransfer.Cancel != nil {
				m.markActiveTransferInterrupted()
				m.activeTransfer.Cancel()
			}
			return m, tea.Quit
		case "j", "down":
			m.moveDashboardDown()
		case "k", "up":
			m.moveDashboardUp()
		case "h", "left":
			m.moveDashboardLeft()
		case "l", "right":
			m.moveDashboardRight()
		case "/":
			m.searching = true
			m.query = ""
		case "?", "shift+/":
			m.helpBackMode = modeDashboard
			m.mode = modeHelp
		case "s":
			m.sortBy = (m.sortBy + 1) % 5
			m.selected = 0
			m.status = "排序：" + m.sortName()
		case "o":
			if m.filter == filterOnline {
				m.filter = filterAll
			} else {
				m.filter = filterOnline
			}
			m.selected = 0
		case "p":
			if m.filter == filterProblem {
				m.filter = filterAll
			} else {
				m.filter = filterProblem
			}
			m.selected = 0
		case "f":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.toggleFavorite(idx)
			}
		case "t":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.togglePinned(idx)
			}
		case "y":
			m.transferJobsBack = modeDashboard
			m.mode = modeTransferJobs
			m.reloadTransfers()
		case "g":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startDeploymentList(idx), nil
			}
		case "v":
			m.favoriteOnly = !m.favoriteOnly
			m.selected = 0
			if m.favoriteOnly {
				m.status = "筛选：收藏"
			} else {
				m.status = "已取消收藏筛选"
			}
		case "tab":
			m.cycleCategory()
			m.selected = 0
		case " ":
			if m.dashboardMode == dashboardCategory && m.dashboardFocus == 0 {
				m.dashboardFocus = 1
				return m, nil
			}
			if idx, ok := m.selectedRealIndex(); ok {
				return m.openDetail(idx)
			}
		case "a":
			return m.startAddForm(), nil
		case "c":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startCopyForm(idx), nil
			}
		case "e":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startEditForm(idx), nil
			}
		case "x":
			if idx, ok := m.selectedRealIndex(); ok {
				m.deleteIndex = idx
				m.mode = modeDeleteConfirm
			}
		case "u":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startUpload(idx), nil
			}
		case "d":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startDownload(idx), nil
			}
		case "m":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startCommandList(idx), nil
			}
		case "b":
			return m.startBatchSelect(), nil
		case "i":
			return m.startCommandHistory()
		case "w":
			m.mode = modeAnomalyOverview
			m.anomalyIndex = 0
		case "z":
			switch m.dashboardMode {
			case dashboardCards:
				m.dashboardMode = dashboardGrouped
			case dashboardGrouped:
				m.dashboardMode = dashboardCategory
				m.dashboardFocus = 1
			default:
				m.dashboardMode = dashboardCards
			}
			m.status = ""
		case "r":
			m.status = "正在刷新全部服务器..."
			m.collectRound++
			m.manualRound = m.collectRound
			m.pendingByRound[m.collectRound] = len(m.states)
			return m, m.collectAll(m.collectRound, true)
		case "enter":
			if m.dashboardMode == dashboardCategory && m.dashboardFocus == 0 {
				m.dashboardFocus = 1
				return m, nil
			}
			if idx, ok := m.selectedRealIndex(); ok {
				cmd, cleanup := actions.SSHCommand(m.states[idx].Host)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					cleanup()
					return sshDoneMsg{Index: idx, Err: err}
				})
			}
		}
	}
	return m, nil
}

func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formPane == 1 {
		return m.updateCategoryPane(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.copying = false
		m.status = "已取消。"
	case "tab":
		m.formPane = 1
	case "down":
		m.formIndex = m.nextFormIndex()
		m.formCursor = m.formValueLen()
	case "shift+tab":
		m.formPane = 1
	case "up":
		m.formIndex = m.prevFormIndex()
		m.formCursor = m.formValueLen()
	case "left":
		if m.formIndex == 0 {
			m.moveCategory(-1)
		} else if m.formIndex == jumpHostRefFormIndex {
			m.moveJumpHostRef(-1)
		} else {
			m.moveFormCursor(-1)
		}
	case "right":
		if m.formIndex == 0 {
			m.moveCategory(1)
		} else if m.formIndex == jumpHostRefFormIndex {
			m.moveJumpHostRef(1)
		} else {
			m.moveFormCursor(1)
		}
	case "enter":
		healthPorts, err := config.ParseHealthPorts(m.form.HealthPorts)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		expireAt, err := normalizeExpireAtForSave(m.form.ExpireAt)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		favorite := false
		pinned := false
		pinnedOrder := int64(0)
		if m.editing {
			if m.editIndex < 0 || m.editIndex >= len(m.states) {
				m.status = "编辑失败：没有选中的服务器"
				return m, nil
			}
			favorite = m.states[m.editIndex].Host.Favorite
			pinned = m.states[m.editIndex].Host.Pinned
			pinnedOrder = m.states[m.editIndex].Host.PinnedOrder
		}
		input := config.HostInput{
			Category:     m.form.Category,
			Name:         m.form.Name,
			HostName:     m.form.HostName,
			User:         m.form.User,
			Port:         m.form.Port,
			IdentityFile: m.form.IdentityFile,
			Password:     m.form.Password,
			JumpHostRef:  m.form.JumpHostRef,
			Note:         m.form.Note,
			ExpireAt:     expireAt,
			Favorite:     favorite,
			Pinned:       pinned,
			PinnedOrder:  pinnedOrder,
			HealthPorts:  healthPorts,
		}
		if m.editing {
			if err := config.EditHost(m.home, m.states[m.editIndex].Host, input); err != nil {
				m.status = "编辑失败：" + err.Error()
				return m, nil
			}
		} else {
			if err := config.AddHost(m.home, input); err != nil {
				m.status = "添加失败：" + err.Error()
				return m, nil
			}
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		if m.editing {
			m.status = "服务器已更新。"
		} else if m.copying {
			m.status = "服务器已复制。"
		} else {
			m.status = "服务器已添加。"
		}
		m.copying = false
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	case "backspace":
		if m.formIndex == expireAtFormIndex {
			m.formExpireBackspace()
		} else {
			m.formBackspace()
		}
	default:
		if len(msg.Runes) > 0 && m.formIndex != 0 {
			if m.formIndex == expireAtFormIndex {
				m.formExpireAppend(msg.Runes)
			} else {
				m.formAppend(string(msg.Runes))
			}
		}
	}
	return m, nil
}

func (m Model) updateCategoryPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.addingCategory || m.renamingCategory {
		key := shortcutKey(msg)
		switch key {
		case "esc", "q", "ctrl+c":
			m.addingCategory = false
			m.renamingCategory = false
			m.categoryDraft = ""
		case "enter":
			if m.renamingCategory {
				oldName := ""
				if len(m.categories) > 0 {
					oldName = m.categories[m.categoryIndex]
				}
				if err := config.RenameCategory(m.home, oldName, m.categoryDraft); err != nil {
					m.status = "重命名分类失败：" + categoryErrorText(err)
				} else {
					newName := strings.TrimSpace(m.categoryDraft)
					hosts, err := config.LoadHosts(m.home)
					if err != nil {
						m.status = "重命名后重新读取失败：" + err.Error()
					} else {
						m.reloadHosts(hosts)
					}
					m.reloadCategories(newName)
					m.form.Category = m.categories[m.categoryIndex]
					if m.category == oldName {
						m.category = newName
					}
					m.status = "分类已重命名。"
				}
			} else {
				if err := config.AddCategory(m.home, m.categoryDraft); err != nil {
					m.status = "添加分类失败：" + categoryErrorText(err)
				} else {
					m.reloadCategories(m.categoryDraft)
					m.form.Category = m.categories[m.categoryIndex]
					m.status = "分类已添加。"
				}
			}
			m.addingCategory = false
			m.renamingCategory = false
			m.categoryDraft = ""
		case "backspace":
			m.categoryDraft = removeLastRune(m.categoryDraft)
		default:
			if len(msg.Runes) > 0 {
				m.categoryDraft += string(msg.Runes)
			}
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "tab", "shift+tab":
		m.formPane = 0
	case "j", "down":
		m.moveCategory(1)
	case "k", "up":
		m.moveCategory(-1)
	case "n", "a":
		m.addingCategory = true
		m.renamingCategory = false
		m.categoryDraft = ""
		m.status = "输入新分类名称。"
	case "r":
		if len(m.categories) == 0 {
			return m, nil
		}
		name := m.categories[m.categoryIndex]
		if name == config.BastionCategory {
			m.status = "跳板机分类不能重命名。"
			return m, nil
		}
		m.renamingCategory = true
		m.addingCategory = false
		m.categoryDraft = name
		m.status = "输入新的分类名称。"
	case "x":
		if len(m.categories) == 0 {
			return m, nil
		}
		name := m.categories[m.categoryIndex]
		m.confirm = confirmAction{
			Kind:  confirmDeleteCategory,
			Title: "确认删除分类",
			Lines: []string{
				"分类：" + name,
				"将删除这个空分类。",
			},
			Back:  modeAddForm,
			Value: name,
		}
		m.mode = modeConfirmAction
	}
	return m, nil
}

func (m *Model) moveCategory(delta int) {
	if len(m.categories) == 0 {
		m.categories = []string{"default"}
		m.categoryIndex = 0
		m.form.Category = "default"
		return
	}
	m.categoryIndex += delta
	if m.categoryIndex < 0 {
		m.categoryIndex = len(m.categories) - 1
	}
	if m.categoryIndex >= len(m.categories) {
		m.categoryIndex = 0
	}
	m.form.Category = m.categories[m.categoryIndex]
}

func (m *Model) moveJumpHostRef(delta int) {
	choices := append([]string{""}, m.bastionNames()...)
	if len(choices) == 0 {
		m.form.JumpHostRef = ""
		return
	}
	current := strings.TrimSpace(m.form.JumpHostRef)
	index := 0
	for i, choice := range choices {
		if choice == current {
			index = i
			break
		}
	}
	index = (index + delta) % len(choices)
	if index < 0 {
		index += len(choices)
	}
	m.form.JumpHostRef = choices[index]
}

func (m Model) bastionNames() []string {
	names := []string{}
	for _, state := range m.states {
		h := state.Host
		if h.Category != config.BastionCategory {
			continue
		}
		if m.editing && m.editIndex >= 0 && m.editIndex < len(m.states) {
			current := m.states[m.editIndex].Host
			if current.Category == h.Category && current.Name == h.Name {
				continue
			}
		}
		names = append(names, h.Name)
	}
	sort.Strings(names)
	return names
}

func (m *Model) reloadCategories(prefer string) {
	categories, _, err := config.LoadCategories(m.home)
	if err != nil || len(categories) == 0 {
		categories = []string{"default"}
	}
	m.categories = categories
	m.categoryIndex = 0
	if strings.TrimSpace(prefer) == "" {
		prefer = "default"
	}
	for i, category := range categories {
		if category == prefer {
			m.categoryIndex = i
			break
		}
	}
}

func categoryErrorText(err error) string {
	switch {
	case errors.Is(err, os.ErrInvalid):
		return "至少需要保留一个分类，或分类名称不能为空"
	case errors.Is(err, os.ErrPermission):
		return "跳板机分类不能重命名或删除，分类下面还有服务器时也不能删除"
	case errors.Is(err, os.ErrExist):
		return "分类名称已存在"
	case errors.Is(err, os.ErrNotExist):
		return "分类不存在"
	default:
		return err.Error()
	}
}

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n":
		m.mode = modeDashboard
		m.status = "已取消删除。"
	case "y", "enter":
		if m.deleteIndex < 0 || m.deleteIndex >= len(m.states) {
			m.mode = modeDashboard
			m.status = "没有选中的服务器。"
			return m, nil
		}
		h := m.states[m.deleteIndex].Host
		if err := config.DeleteHost(m.home, h, true); err != nil {
			m.mode = modeDashboard
			m.status = "删除失败：" + err.Error()
			return m, nil
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.mode = modeDashboard
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		m.status = "服务器已删除。"
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	}
	return m, nil
}

func (m Model) updateConfirmAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n", "q":
		m.mode = m.confirm.Back
		m.status = "已取消删除。"
	case "y", "enter":
		switch m.confirm.Kind {
		case confirmDeleteCategory:
			name := m.confirm.Value
			if err := config.DeleteCategory(m.home, name); err != nil {
				m.mode = m.confirm.Back
				m.status = "删除分类失败：" + categoryErrorText(err)
				return m, nil
			}
			m.reloadCategories("")
			m.form.Category = m.categories[m.categoryIndex]
			m.mode = modeAddForm
			m.status = "分类已删除。"
		case confirmDeleteCommand:
			item := m.confirm.Command
			m.mode = modeCommandList
			return m.deleteCommandTemplate(item)
		case confirmDeleteHistory:
			entry := m.confirm.History
			m.mode = modeCommandHistory
			return m.deleteCommandHistoryEntry(entry)
		case confirmDeleteDeployment:
			index := m.confirm.Index
			m.mode = modeDeploymentList
			return m.deleteDeploymentApp(index)
		}
		m.confirm = confirmAction{}
	}
	return m, nil
}

func (m Model) startAddForm() Model {
	m.reloadCategories("")
	m.mode = modeAddForm
	m.formIndex = 0
	m.formCursor = 0
	m.formPane = 0
	m.editing = false
	m.copying = false
	m.editIndex = -1
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	m.form = addForm{Category: m.categories[m.categoryIndex], User: "root", Port: "22"}
	m.status = "添加服务器"
	return m
}

func (m Model) copyHostName(category string, name string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "服务器"
	}
	candidate := base + "-副本"
	if !m.hostNameExists(category, candidate) {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = fmt.Sprintf("%s-副本%d", base, i)
		if !m.hostNameExists(category, candidate) {
			return candidate
		}
	}
}

func (m Model) hostNameExists(category string, name string) bool {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	for _, state := range m.states {
		h := state.Host
		if strings.TrimSpace(h.Category) == category && strings.TrimSpace(h.Name) == name {
			return true
		}
	}
	return false
}

func (m Model) startCopyForm(idx int) Model {
	h := m.states[idx].Host
	password, _ := m.passwords.Password(h.Name)
	input := config.InputFromHost(h, password)
	m.reloadCategories(input.Category)
	m.mode = modeAddForm
	m.formIndex = 1
	m.formCursor = 0
	m.formPane = 0
	m.editing = false
	m.copying = true
	m.editIndex = -1
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	name := m.copyHostName(input.Category, input.Name)
	m.form = addForm{
		Category:     m.categories[m.categoryIndex],
		Name:         name,
		HostName:     input.HostName,
		User:         input.User,
		Port:         input.Port,
		IdentityFile: input.IdentityFile,
		Password:     input.Password,
		JumpHostRef:  input.JumpHostRef,
		HealthPorts:  config.FormatHealthPorts(input.HealthPorts),
		ExpireAt:     input.ExpireAt,
		Note:         input.Note,
	}
	m.formCursor = len([]rune(name))
	m.status = "复制服务器"
	return m
}

func (m Model) startEditForm(idx int) Model {
	h := m.states[idx].Host
	password, _ := m.passwords.Password(h.Name)
	input := config.InputFromHost(h, password)
	m.reloadCategories(input.Category)
	m.mode = modeAddForm
	m.formIndex = 0
	m.formCursor = 0
	m.formPane = 0
	m.editing = true
	m.copying = false
	m.editIndex = idx
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	m.form = addForm{
		Category:     m.categories[m.categoryIndex],
		Name:         input.Name,
		HostName:     input.HostName,
		User:         input.User,
		Port:         input.Port,
		IdentityFile: input.IdentityFile,
		Password:     input.Password,
		JumpHostRef:  input.JumpHostRef,
		HealthPorts:  config.FormatHealthPorts(input.HealthPorts),
		ExpireAt:     input.ExpireAt,
		Note:         input.Note,
	}
	m.status = "编辑服务器"
	return m
}

func (m *Model) reloadHosts(hosts []host.Host) {
	states := make([]hostState, len(hosts))
	for i, h := range hosts {
		states[i] = hostState{Host: h, Loading: true}
	}
	m.states = states
	m.selected = 0
	m.query = ""
	m.passwords = config.PasswordsFromHosts(hosts)
	m.collector = monitor.NewCollector(m.passwords)
	m.collector.Timeout = m.appConfig.CommandDuration()
	m.collector.ConnectTimeout = m.appConfig.ConnectDuration()
	m.reloadCategories("")
}

func (m *Model) recordLastLogin(h host.Host, at time.Time) {
	config.SetLastLogin(&m.appState, h, at)
	if err := config.SaveState(m.home, m.appState); err != nil {
		m.status = "最近登录保存失败：" + err.Error()
	}
}

func (m Model) lastLogin(h host.Host) time.Time {
	return config.LastLoginFor(m.appState, h)
}

func (m Model) toggleFavorite(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	hosts := make([]host.Host, len(m.states))
	for i, state := range m.states {
		hosts[i] = state.Host
	}
	hosts[index].Favorite = !hosts[index].Favorite
	if err := config.SaveServerHosts(m.home, hosts); err != nil {
		m.status = "收藏更新失败：" + err.Error()
		return m, nil
	}
	m.states[index].Host.Favorite = hosts[index].Favorite
	if hosts[index].Favorite {
		m.status = "已收藏：" + hosts[index].Name
	} else {
		m.status = "已取消收藏：" + hosts[index].Name
	}
	if m.favoriteOnly && !hosts[index].Favorite {
		m.selected = 0
	}
	return m, nil
}

func (m Model) togglePinned(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	hosts := make([]host.Host, len(m.states))
	for i, state := range m.states {
		hosts[i] = state.Host
	}
	if hosts[index].Pinned {
		hosts[index].Pinned = false
		hosts[index].PinnedOrder = 0
	} else {
		hosts[index].Pinned = true
		hosts[index].PinnedOrder = nextPinnedOrder(hosts)
	}
	if err := config.SaveServerHosts(m.home, hosts); err != nil {
		m.status = "置顶更新失败：" + err.Error()
		return m, nil
	}
	m.states[index].Host.Pinned = hosts[index].Pinned
	m.states[index].Host.PinnedOrder = hosts[index].PinnedOrder
	if hosts[index].Pinned {
		m.status = "已置顶：" + hosts[index].Name
	} else {
		m.status = "已取消置顶：" + hosts[index].Name
	}
	return m, nil
}

func nextPinnedOrder(hosts []host.Host) int64 {
	var maxOrder int64
	for _, h := range hosts {
		if h.PinnedOrder > maxOrder {
			maxOrder = h.PinnedOrder
		}
	}
	return maxOrder + 1
}

func (m *Model) formAppend(s string) {
	if m.formIndex == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.formCursor < 0 {
		m.formCursor = 0
	}
	if m.formCursor > len(value) {
		m.formCursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.formCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.formCursor:]...)
	m.setFormValue(string(next))
	m.formCursor += len(insert)
}

func (m *Model) formBackspace() {
	if m.formIndex == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.formCursor <= 0 || len(value) == 0 {
		return
	}
	if m.formCursor > len(value) {
		m.formCursor = len(value)
	}
	next := append([]rune{}, value[:m.formCursor-1]...)
	next = append(next, value[m.formCursor:]...)
	m.setFormValue(string(next))
	m.formCursor--
}

func (m *Model) formExpireAppend(runes []rune) {
	mask := []rune(dateMask(m.form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.formCursor, 0, len(positions))
	for _, r := range runes {
		if r >= '０' && r <= '９' {
			r = r - '０' + '0'
		}
		if r < '0' || r > '9' || cursor >= len(positions) {
			continue
		}
		mask[positions[cursor]] = r
		cursor++
	}
	m.form.ExpireAt = string(mask)
	m.formCursor = cursor
}

func (m *Model) formExpireBackspace() {
	if m.formCursor <= 0 {
		return
	}
	mask := []rune(dateMask(m.form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.formCursor, 0, len(positions))
	pos := positions[cursor-1]
	mask[pos] = datePlaceholderForPosition(pos)
	m.form.ExpireAt = string(mask)
	m.formCursor = cursor - 1
}

func dateMask(value string) string {
	base := []rune("yyyy-mm-dd")
	positions := dateInputPositions()
	runes := []rune(value)
	if len(runes) == len(base) {
		for _, pos := range positions {
			r := runes[pos]
			if (r >= '0' && r <= '9') || r == datePlaceholderForPosition(pos) {
				base[pos] = r
			}
		}
		return string(base)
	}
	digits := []rune(dateDigits(value))
	for i, r := range digits {
		if i >= len(positions) {
			break
		}
		base[positions[i]] = r
	}
	return string(base)
}

func normalizeExpireAtForSave(value string) (string, error) {
	mask := []rune(dateMask(value))
	positions := dateInputPositions()
	digits := make([]rune, 0, len(positions))
	hasValue := false
	incomplete := false
	for _, pos := range positions {
		r := mask[pos]
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
			hasValue = true
			continue
		}
		incomplete = true
	}
	if !hasValue {
		return "", nil
	}
	if incomplete {
		return "", fmt.Errorf("到期时间未填写完整")
	}
	value = fmt.Sprintf("%s-%s-%s", string(digits[:4]), string(digits[4:6]), string(digits[6:8]))
	if err := config.ValidateExpireAt(value); err != nil {
		return "", err
	}
	return value, nil
}

func datePlaceholderForPosition(pos int) rune {
	switch pos {
	case 0, 1, 2, 3:
		return 'y'
	case 5, 6:
		return 'm'
	default:
		return 'd'
	}
}

func dateInputPositions() []int {
	return []int{0, 1, 2, 3, 5, 6, 8, 9}
}

func (m *Model) moveFormCursor(delta int) {
	m.formCursor += delta
	if m.formCursor < 0 {
		m.formCursor = 0
	}
	maxCursor := m.formValueLen()
	if m.formCursor > maxCursor {
		m.formCursor = maxCursor
	}
}

func (m Model) formValueLen() int {
	if m.formIndex == expireAtFormIndex {
		return dateCursorEnd(m.form.ExpireAt)
	}
	return len([]rune(m.formValue()))
}

func (m Model) nextFormIndex() int {
	ids := editableFormIDs(m.form.fields())
	for i, id := range ids {
		if id == m.formIndex {
			return ids[(i+1)%len(ids)]
		}
	}
	return ids[0]
}

func (m Model) prevFormIndex() int {
	ids := editableFormIDs(m.form.fields())
	for i, id := range ids {
		if id == m.formIndex {
			if i == 0 {
				return ids[len(ids)-1]
			}
			return ids[i-1]
		}
	}
	return ids[0]
}

func editableFormIDs(fields []formField) []int {
	ids := make([]int, 0, len(fields))
	for _, field := range fields {
		if !field.Section {
			ids = append(ids, field.ID)
		}
	}
	if len(ids) == 0 {
		return []int{categoryFormIndex}
	}
	return ids
}

func selectedFieldRow(fields []formField, id int) int {
	for i, field := range fields {
		if !field.Section && field.ID == id {
			return i
		}
	}
	return 0
}

func dateCursorEnd(value string) int {
	mask := []rune(dateMask(value))
	positions := dateInputPositions()
	for i, pos := range positions {
		r := mask[pos]
		if r < '0' || r > '9' {
			return i
		}
	}
	return len(positions)
}

func (m Model) formValue() string {
	switch m.formIndex {
	case 1:
		return m.form.Name
	case 2:
		return m.form.HostName
	case 3:
		return m.form.User
	case 4:
		return m.form.Port
	case 5:
		return m.form.IdentityFile
	case 6:
		return m.form.Password
	case 7:
		return emptyChoice(m.form.JumpHostRef, "无")
	case 8:
		return m.form.HealthPorts
	case 9:
		return m.form.Note
	case 10:
		return m.form.ExpireAt
	default:
		return ""
	}
}

func (m *Model) setFormValue(value string) {
	switch m.formIndex {
	case 1:
		m.form.Name = value
	case 2:
		m.form.HostName = value
	case 3:
		m.form.User = value
	case 4:
		m.form.Port = value
	case 5:
		m.form.IdentityFile = value
	case 6:
		m.form.Password = value
	case 7:
		m.form.JumpHostRef = strings.TrimSpace(value)
	case 8:
		m.form.HealthPorts = value
	case 9:
		m.form.Note = value
	case 10:
		m.form.ExpireAt = value
	}
}

func emptyChoice(value, empty string) string {
	if strings.TrimSpace(value) == "" {
		return empty
	}
	return value
}

func removeLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func shortcutKey(msg tea.KeyMsg) string {
	key := strings.ToLower(msg.String())
	if key == "shift+/" {
		return key
	}
	if len(msg.Runes) == 1 {
		key = normalizeShortcutRune(msg.Runes[0])
	}
	return key
}

func normalizeShortcutRune(r rune) string {
	switch {
	case r >= 'Ａ' && r <= 'Ｚ':
		return string(r - 'Ａ' + 'a')
	case r >= 'ａ' && r <= 'ｚ':
		return string(r - 'ａ' + 'a')
	case r >= '０' && r <= '９':
		return string(r - '０' + '0')
	}
	switch r {
	case '？':
		return "?"
	case '／', '、':
		return "/"
	case '　':
		return " "
	default:
		return strings.ToLower(string(r))
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.searching = false
		m.query = ""
		m.selected = 0
	case "enter":
		if idx, ok := m.selectedRealIndex(); ok {
			m.searching = false
			cmd, cleanup := actions.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Index: idx, Err: err}
			})
		}
		m.searching = false
	case " ":
		if idx, ok := m.selectedRealIndex(); ok {
			m.searching = false
			return m.openDetail(idx)
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "backspace":
		if len(m.query) > 0 {
			runes := []rune(m.query)
			m.query = string(runes[:len(runes)-1])
			m.selected = 0
		}
	default:
		if len(msg.Runes) > 0 {
			m.query += string(msg.Runes)
			m.selected = 0
		}
	}
	return m, nil
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c", "b":
		m.mode = modeDashboard
		m.detailScroll = 0
	case "j", "down":
		m.detailScroll = clampInt(m.detailScroll+1, 0, m.detailMaxScroll())
	case "k", "up":
		m.detailScroll = clampInt(m.detailScroll-1, 0, m.detailMaxScroll())
	case "tab", "right":
		m.moveDetailSection(1)
	case "shift+tab", "left":
		m.moveDetailSection(-1)
	case "u":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startUpload(idx), nil
		}
	case "d":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startDownload(idx), nil
		}
	case "r":
		if idx, ok := m.selectedRealIndex(); ok {
			m.states[idx].Loading = true
			m.states[idx].LoginLoading = true
			m.states[idx].LoginError = ""
			m.states[idx].FailedLoginError = ""
			m.states[idx].SSHDSecurityError = ""
			m.states[idx].PortDetailsError = ""
			m.states[idx].ContainerError = ""
			m.states[idx].PortDetails = nil
			m.states[idx].ContainerDetails = nil
			return m, tea.Batch(m.collectOne(idx), m.fetchLoginRecords(idx))
		}
	case "f":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.toggleFavorite(idx)
		}
	case "m":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startCommandList(idx), nil
		}
	case "l":
		if idx, ok := m.selectedRealIndex(); ok {
			cmd, cleanup := actions.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Index: idx, Err: err}
			})
		}
	}
	return m, nil
}

func (m Model) updateAnomalyOverview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.anomalyItems()
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
	case "j", "down":
		m.anomalyIndex = clampInt(m.anomalyIndex+1, 0, maxInt(0, len(items)-1))
	case "k", "up":
		m.anomalyIndex = clampInt(m.anomalyIndex-1, 0, maxInt(0, len(items)-1))
	case "f", "tab":
		m.anomalyFilter = (m.anomalyFilter + 1) % 8
		m.anomalyIndex = 0
	case "0":
		m.anomalyFilter = anomalyAll
		m.anomalyIndex = 0
	case "1":
		m.anomalyFilter = anomalySevere
		m.anomalyIndex = 0
	case "2":
		m.anomalyFilter = anomalyWarn
		m.anomalyIndex = 0
	case "3":
		m.anomalyFilter = anomalyOffline
		m.anomalyIndex = 0
	case "4":
		m.anomalyFilter = anomalyResource
		m.anomalyIndex = 0
	case "5":
		m.anomalyFilter = anomalyContainer
		m.anomalyIndex = 0
	case "6":
		m.anomalyFilter = anomalyService
		m.anomalyIndex = 0
	case "7":
		m.anomalyFilter = anomalySecurity
		m.anomalyIndex = 0
	case "enter", " ":
		if len(items) == 0 {
			return m, nil
		}
		m.anomalyIndex = clampInt(m.anomalyIndex, 0, len(items)-1)
		item := items[m.anomalyIndex]
		m.selected = m.visibleIndexForRealIndex(item.Index)
		return m.openDetailSection(item.Index, anomalyDetailSection(item.Checks))
	case "r":
		m.status = "正在刷新全部服务器..."
		m.collectRound++
		m.manualRound = m.collectRound
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, true)
	}
	return m, nil
}

func (m Model) openDetailSection(index int, section string) (tea.Model, tea.Cmd) {
	model, cmd := m.openDetail(index)
	next, ok := model.(Model)
	if !ok {
		return model, cmd
	}
	next.setDetailSection(section)
	return next, cmd
}

func (m *Model) setDetailSection(section string) {
	if strings.TrimSpace(section) == "" {
		return
	}
	for i, name := range m.detailSectionNames() {
		if name == section {
			m.detailSectionIndex = i
			m.detailScroll = 0
			return
		}
	}
}

func (m Model) visibleIndexForRealIndex(realIndex int) int {
	indexes := m.filteredIndexes()
	for i, index := range indexes {
		if index == realIndex {
			return i
		}
	}
	return clampInt(m.selected, 0, maxInt(0, len(indexes)-1))
}

func (m *Model) moveDetailSection(delta int) {
	sections := m.detailSectionNames()
	if len(sections) == 0 {
		m.detailSectionIndex = 0
		return
	}
	m.detailSectionIndex = moveIndex(m.detailSectionIndex, len(sections), delta)
	m.detailScroll = 0
}

func (m Model) detailSectionNames() []string {
	sections := []string{"基础信息", "资源监控", "服务状态", "容器"}
	if idx, ok := m.selectedRealIndex(); ok && strings.TrimSpace(m.states[idx].Metrics.Error) != "" {
		sections = append(sections, "最近错误")
	}
	sections = append(sections, "登录记录", "风险提示")
	return sections
}

func (m Model) openDetail(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(m.states) {
		return m, nil
	}
	m.mode = modeDetail
	m.detailScroll = 0
	if len(m.states[idx].LoginSummary) > 0 || len(m.states[idx].FailedLoginSummary) > 0 || len(m.states[idx].SSHDSecurity) > 0 || len(m.states[idx].PortDetails) > 0 || len(m.states[idx].ContainerDetails) > 0 || m.states[idx].LoginLoading || m.states[idx].LoginError != "" || m.states[idx].FailedLoginError != "" || m.states[idx].SSHDSecurityError != "" || m.states[idx].PortDetailsError != "" || m.states[idx].ContainerError != "" {
		return m, nil
	}
	m.states[idx].LoginLoading = true
	return m, m.fetchLoginRecords(idx)
}

func (m Model) fetchLoginRecords(index int) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, "last -n 100 2>/dev/null || true")
		cleanup()
		msg := loginRecordsMsg{Index: index}
		if result.Err != nil {
			errText := strings.TrimSpace(result.Output)
			if errText == "" {
				errText = result.Err.Error()
			}
			msg.ErrText = errText
		} else {
			msg.Summary = loginSummaryRows(parseLoginRecords(result.Output, 100))
		}
		failedResult, failedCleanup := actions.RemoteCommandContext(ctx, h, failedLoginScript())
		failedCleanup()
		if strings.TrimSpace(failedResult.Output) != "" {
			msg.FailedSummary, msg.FailedErrText = failedLoginSummary(failedResult.Output)
		}
		if failedResult.Err != nil && msg.FailedErrText == "" {
			msg.FailedErrText = failedResult.Err.Error()
		}
		sshdResult, sshdCleanup := actions.RemoteCommandContext(ctx, h, sshdSecurityScript())
		sshdCleanup()
		if strings.TrimSpace(sshdResult.Output) != "" {
			msg.SSHDSecurity = parseSSHDSettings(sshdResult.Output)
		}
		if sshdResult.Err != nil {
			msg.SSHDErrText = "sshd配置不可读"
		}
		portResult, portCleanup := actions.RemoteCommandContext(ctx, h, portDetailScript())
		portCleanup()
		msg.Ports, msg.PortsErrText = parsePortDetails(portResult.Output)
		if portResult.Err != nil && msg.PortsErrText == "" {
			msg.PortsErrText = portResult.Err.Error()
		}
		containerResult, containerCleanup := actions.RemoteCommandContext(ctx, h, containerDetailScript())
		containerCleanup()
		msg.Containers, msg.ContainerErr = parseContainerDetails(containerResult.Output)
		if containerResult.Err != nil && msg.ContainerErr == "" {
			msg.ContainerErr = containerResult.Err.Error()
		}
		associatePortContainers(msg.Ports, msg.Containers)
		return msg
	}
}

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeDashboard
		m.transfer = transferNone
		m.choices = nil
		m.remoteTree = remoteTree{}
		m.pickIndex = 0
		m.status = "已取消。"
	case "j", "down":
		m.movePick(1)
	case "k", "up":
		m.movePick(-1)
	case "l", "right":
		if m.treePickerActive() {
			return m.expandTreePick()
		}
	case "h", "left":
		if m.treePickerActive() {
			return m.collapseTreePick(), nil
		}
	case " ":
		return m.confirmPick()
	case "enter":
		if m.treePickerActive() {
			return m.toggleTreePick()
		}
		return m.confirmPick()
	}
	return m, nil
}

func transferProgressText(t activeTransfer, states []hostState) string {
	if t.Kind == "" {
		return ""
	}
	if t.Total <= 0 {
		return fmt.Sprintf("%s中：%s", t.Kind, filepath.Base(t.Source))
	}
	current := int64(0)
	if t.Kind == "上传" && t.HostIndex >= 0 && t.HostIndex < len(states) && t.RemotePath != "" {
		current = remoteSizeBytes(states[t.HostIndex].Host, t.RemotePath)
	} else {
		current = localPathSize(t.LocalPath)
	}
	percent := int(float64(current) / float64(t.Total) * 100)
	if percent < 0 {
		percent = 0
	}
	if percent > 99 {
		percent = 99
	}
	return fmt.Sprintf("%s中：%s  %d%%", t.Kind, filepath.Base(t.Source), percent)
}

func remoteJoin(dir, name string) string {
	if dir == "" || dir == "/" {
		return "/" + name
	}
	return strings.TrimRight(dir, "/") + "/" + name
}

func localPathSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (m *Model) move(delta int) {
	count := len(m.filteredIndexes())
	if count == 0 {
		m.selected = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= count {
		m.selected = count - 1
	}
}

func (m *Model) moveDashboardDown() {
	if m.dashboardMode == dashboardCategory && m.dashboardFocus == 0 {
		m.moveDashboardCategory(1)
		return
	}
	if m.dashboardMode == dashboardCategory {
		m.move(1)
		return
	}
	if m.dashboardMode == dashboardCards && !m.searching {
		m.move(m.dashboardColumns())
		return
	}
	m.move(1)
}

func (m *Model) moveDashboardUp() {
	if m.dashboardMode == dashboardCategory && m.dashboardFocus == 0 {
		m.moveDashboardCategory(-1)
		return
	}
	if m.dashboardMode == dashboardCategory {
		m.move(-1)
		return
	}
	if m.dashboardMode == dashboardCards && !m.searching {
		m.move(-m.dashboardColumns())
		return
	}
	m.move(-1)
}

func (m *Model) moveDashboardLeft() {
	if m.dashboardMode == dashboardCategory {
		m.dashboardFocus = 0
		return
	}
	m.move(-1)
}

func (m *Model) moveDashboardRight() {
	if m.dashboardMode == dashboardCategory {
		if m.dashboardFocus == 0 {
			m.dashboardFocus = 1
		}
		return
	}
	m.move(1)
}

func (m *Model) moveDashboardCategory(delta int) {
	items := m.dashboardCategoryItems()
	if len(items) == 0 {
		return
	}
	index := m.dashboardCategorySelectedIndex(items)
	index = clampInt(index+delta, 0, len(items)-1)
	m.applyDashboardCategoryItem(items[index])
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func fitLines(lines []string, width int) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, ansi.Truncate(line, width, "…"))
	}
	return out
}

func visibleRange(total int, selected int, height int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if height <= 0 || height >= total {
		return 0, total
	}
	selected = clampInt(selected, 0, total-1)
	start := selected - height + 1
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}

func (m Model) selectedRealIndex() (int, bool) {
	indexes := m.filteredIndexes()
	if len(indexes) == 0 || m.selected < 0 || m.selected >= len(indexes) {
		return 0, false
	}
	return indexes[m.selected], true
}

func (m Model) filteredIndexes() []int {
	var indexes []int
	q := strings.ToLower(strings.TrimSpace(m.query))
	for i, state := range m.states {
		h := state.Host
		if m.category != "" && h.Category != m.category {
			continue
		}
		if m.favoriteOnly && !h.Favorite {
			continue
		}
		if m.filter == filterOnline && !state.Metrics.Online {
			continue
		}
		if m.filter == filterProblem && !isProblem(state) {
			continue
		}
		text := strings.ToLower(strings.Join([]string{
			h.Name, h.HostName, h.User, h.Category, h.Note, h.ExpireAt,
		}, " "))
		if q == "" || strings.Contains(text, q) {
			indexes = append(indexes, i)
		}
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a := m.states[indexes[i]]
		b := m.states[indexes[j]]
		if m.sortCategoryBeforePinned() && a.Host.Category != b.Host.Category {
			return a.Host.Category < b.Host.Category
		}
		if a.Host.Pinned != b.Host.Pinned {
			return a.Host.Pinned
		}
		if a.Host.Pinned && b.Host.Pinned && a.Host.PinnedOrder != b.Host.PinnedOrder {
			return a.Host.PinnedOrder > b.Host.PinnedOrder
		}
		switch m.sortBy {
		case sortState:
			if a.Metrics.Online != b.Metrics.Online {
				return a.Metrics.Online
			}
		case sortCPU:
			return a.Metrics.CPUPercent > b.Metrics.CPUPercent
		case sortMem:
			return a.Metrics.MemPercent() > b.Metrics.MemPercent()
		case sortDisk:
			return a.Metrics.DiskPercent() > b.Metrics.DiskPercent()
		}
		if a.Host.Category == b.Host.Category {
			return a.Host.Name < b.Host.Name
		}
		return a.Host.Category < b.Host.Category
	})
	return indexes
}

func (m Model) sortCategoryBeforePinned() bool {
	return m.dashboardMode == dashboardGrouped || m.category != ""
}

func isProblem(state hostState) bool {
	if !state.Metrics.Online && !state.Loading {
		return true
	}
	return state.Metrics.CPUPercent >= 85 || state.Metrics.MemPercent() >= 85 || state.Metrics.DiskPercent() >= 90 || state.Metrics.FailedServices > 0
}

func (m Model) sortName() string {
	switch m.sortBy {
	case sortState:
		return "状态"
	case sortCPU:
		return "CPU"
	case sortMem:
		return "内存"
	case sortDisk:
		return "磁盘"
	default:
		return "默认"
	}
}

func (m Model) filterName() string {
	switch m.filter {
	case filterOnline:
		return "在线"
	case filterProblem:
		return "异常"
	default:
		return "全部"
	}
}

func (m *Model) cycleCategory() {
	m.favoriteOnly = false
	categories := []string{""}
	seen := map[string]bool{}
	for _, state := range m.states {
		cat := state.Host.Category
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories[1:])
	current := 0
	for i, cat := range categories {
		if cat == m.category {
			current = i
			break
		}
	}
	m.category = categories[(current+1)%len(categories)]
	if m.category == "" {
		m.status = "分类：全部"
	} else {
		m.status = "分类：" + m.category
	}
}

func (m Model) collectAll(round int, manual bool) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.states))
	for i, state := range m.states {
		index := i
		h := state.Host
		cmds = append(cmds, func() tea.Msg {
			metrics := m.collector.Collect(context.Background(), h)
			return collectMsg{Index: index, Round: round, Metrics: metrics, Manual: manual}
		})
	}
	return tea.Batch(cmds...)
}

func (m Model) collectOne(index int) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	round := m.collectRound
	return func() tea.Msg {
		metrics := m.collector.Collect(context.Background(), h)
		return collectMsg{Index: index, Round: round, Metrics: metrics}
	}
}

func (m *Model) applyMetrics(index int, metrics monitor.Metrics) {
	if index < 0 || index >= len(m.states) {
		return
	}
	state := &m.states[index]
	state.LastAttempt = time.Now()
	if metrics.Online {
		state.Metrics = metrics
		state.FailureCount = 0
	} else if state.Metrics.Online {
		state.FailureCount++
		if state.FailureCount >= 2 {
			state.Metrics = metrics
		}
	} else {
		state.Metrics = metrics
		state.FailureCount++
	}
	state.Loading = false
}

func tickAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) View() string {
	if m.mode == modeAddForm {
		return m.renderAddForm()
	}
	if m.mode == modeDeleteConfirm {
		return m.renderDeleteConfirm()
	}
	if m.mode == modeConfirmAction {
		return m.renderConfirmAction()
	}
	if m.mode == modeDetail {
		return m.renderDetail()
	}
	if m.mode == modeTransferPanel {
		return m.renderTransferPanel()
	}
	if m.mode == modeCommandList {
		return m.renderCommandList()
	}
	if m.mode == modeCommandEdit {
		return m.renderCommandEdit()
	}
	if m.mode == modeCommandConfirm {
		return m.renderCommandConfirm()
	}
	if m.mode == modeCommandOutput {
		return m.renderCommandOutput()
	}
	if m.mode == modeBatchSelect {
		return m.renderBatchSelect()
	}
	if m.mode == modeBatchCommandList {
		return m.renderBatchCommandList()
	}
	if m.mode == modeBatchCommandEdit {
		return m.renderBatchCommandEdit()
	}
	if m.mode == modeBatchConfirm {
		return m.renderBatchConfirm()
	}
	if m.mode == modeBatchOutput {
		return m.renderBatchOutput()
	}
	if m.mode == modeCommandHistory {
		return m.renderCommandHistory()
	}
	if m.mode == modeCommandHistoryDetail {
		return m.renderCommandHistoryDetail()
	}
	if m.mode == modeAnomalyOverview {
		return m.renderAnomalyOverview()
	}
	if m.mode == modeDeploymentList {
		return m.renderDeploymentList()
	}
	if m.mode == modeDeploymentDetail {
		return m.renderDeploymentDetail()
	}
	if m.mode == modeDeploymentEdit {
		return m.renderDeploymentEdit()
	}
	if m.mode == modeDeploymentConfirm {
		return m.renderDeploymentConfirm()
	}
	if m.mode == modeDeploymentRollbackConfirm {
		return m.renderDeploymentRollbackConfirm()
	}
	if m.mode == modeDeploymentOutput {
		return m.renderDeploymentOutput()
	}
	if m.mode == modeTransferJobs {
		return m.renderTransferJobs()
	}
	if m.mode == modeTransferDetail {
		return m.renderTransferDetail()
	}
	if m.mode == modeHelp {
		return m.renderHelpPanel()
	}
	if m.mode != modeDashboard {
		return m.renderPicker()
	}

	indexes := m.filteredIndexes()
	headerParts := []string{"sshm", fmt.Sprintf("服务器 %d", len(indexes))}
	headerParts = append(headerParts, "视图："+dashboardModeName(m.dashboardMode))
	if m.dashboardMode == dashboardCategory {
		headerParts = append(headerParts, "分类："+m.dashboardCategoryActiveLabel())
	}
	if m.searching {
		searchWidth := m.width / 3
		if searchWidth < 8 {
			searchWidth = 8
		}
		headerParts = append(headerParts, "搜索："+inlineCursorText(m.query, searchWidth, len([]rune(m.query))))
	} else if m.query != "" {
		headerParts = append(headerParts, "搜索："+m.query)
	}
	if m.category != "" && m.dashboardMode != dashboardCategory {
		headerParts = append(headerParts, "分类："+m.category)
	}
	if m.filter != filterAll {
		headerParts = append(headerParts, "筛选："+m.filterName())
	}
	if m.favoriteOnly {
		headerParts = append(headerParts, "只看收藏")
	}
	if m.sortBy != sortDefault {
		headerParts = append(headerParts, "排序："+m.sortName())
	}
	if m.refreshStatus != "" {
		headerParts = append(headerParts, m.refreshStatus)
	}
	if m.status != "" && m.status != m.refreshStatus {
		headerParts = append(headerParts, m.status)
	}
	headerText := strings.Join(headerParts, "  ")
	headerWidth := m.width
	if headerWidth < 1 {
		headerWidth = contentWidth(m.width)
	}
	header := titleStyle.Render(fit(headerText, headerWidth))

	var lines []string
	lines = append(lines, header)
	if m.dashboardMode != dashboardCategory {
		lines = append(lines, "")
	}

	if len(m.states) == 0 {
		lines = append(lines, mutedStyle.Render("没有服务器。按 a 添加服务器。"))
	} else if len(indexes) == 0 {
		lines = append(lines, mutedStyle.Render("没有匹配的服务器"))
	} else {
		lines = append(lines, m.renderDashboard(indexes))
	}

	helpWidth := m.width
	if helpWidth < 1 {
		helpWidth = contentWidth(m.width)
	}
	helpBlock := renderDashboardHelp(helpWidth)
	pageDots := ""
	if m.dashboardMode == dashboardCards && !m.searching {
		pageDots = m.dashboardPageDots(indexes)
	} else if m.dashboardMode == dashboardGrouped && !m.searching {
		pageDots = m.dashboardGroupedDots(indexes)
	}
	reservedBottomLines := strings.Count(helpBlock, "\n") + 1
	if pageDots != "" {
		reservedBottomLines += strings.Count(pageDots, "\n") + 1
	}
	lines = padToBottom(lines, m.height, reservedBottomLines)
	if pageDots != "" {
		lines = append(lines, pageDots)
	}
	lines = append(lines, helpBlock)
	return strings.Join(lines, "\n")
}
