package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

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

func (m Model) startCommandList(index int) Model {
	file, _, err := config.LoadCommands(m.home)
	if err != nil {
		m.status = "读取命令模板失败：" + err.Error()
		return m
	}
	m.commandFile = file
	m.commandItems = m.commandListItems(index)
	m.commandIndex = firstCommandItem(m.commandItems)
	m.activeCommand = activeCommand{HostIndex: index}
	m.mode = modeCommandList
	m.status = "命令模板"
	return m
}

func (m Model) startBatchSelect() Model {
	indexes := m.filteredIndexes()
	m.batchIndexes = indexes
	m.batchSelected = map[int]bool{}
	m.batchCursor = 0
	for _, index := range indexes {
		if index == m.selectedRealIndexOrZero() {
			m.batchSelected[index] = true
			break
		}
	}
	m.mode = modeBatchSelect
	m.status = "批量选择服务器"
	return m
}

func (m Model) selectedRealIndexOrZero() int {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return -1
	}
	return idx
}

func (m Model) commandListItems(index int) []commandItem {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	serverKey := config.ServerCommandKey(h.Category, h.Name)
	items := []commandItem{{Name: "当前服务器", Header: true}}
	for i, command := range m.commandFile.Server {
		if command.Server == serverKey {
			items = append(items, commandItem{
				Scope:   commandScopeServer,
				Index:   i,
				Server:  command.Server,
				Name:    command.Name,
				Command: command.Command,
			})
		}
	}
	items = append(items, commandItem{Name: "全局", Header: true})
	for i, command := range m.commandFile.Global {
		items = append(items, commandItem{
			Scope:   commandScopeGlobal,
			Index:   i,
			Name:    command.Name,
			Command: command.Command,
		})
	}
	items = append(items, commandItem{Spacer: true}, commandItem{Name: "临时命令", Command: "", Temporary: true})
	return items
}

func firstCommandItem(items []commandItem) int {
	for i, item := range items {
		if !item.Header {
			return i
		}
	}
	return 0
}

func (m Model) updateCommandList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "down":
		m.moveCommandIndex(1)
	case "k", "up":
		m.moveCommandIndex(-1)
	case "a":
		return m.startCommandEdit(commandItem{}, false), nil
	case "e":
		item, ok := m.selectedCommandItem()
		if ok && !item.Temporary {
			return m.startCommandEdit(item, true), nil
		}
	case "x":
		item, ok := m.selectedCommandItem()
		if ok && !item.Temporary {
			m.confirm = confirmAction{
				Kind:  confirmDeleteCommand,
				Title: "确认删除命令模板",
				Lines: []string{
					"模板：" + item.Name,
					"将删除这个命令模板。",
				},
				Back:    modeCommandList,
				Command: item,
			}
			m.mode = modeConfirmAction
		}
	case "enter":
		item, ok := m.selectedCommandItem()
		if !ok {
			return m, nil
		}
		if item.Temporary {
			return m.startCommandEdit(item, false), nil
		}
		m.commandConfirm = item
		m.commandOutputScroll = 0
		m.mode = modeCommandConfirm
		m.status = "确认执行命令"
	}
	return m, nil
}

func (m *Model) moveCommandIndex(delta int) {
	if len(m.commandItems) == 0 {
		m.commandIndex = 0
		return
	}
	for i := 0; i < len(m.commandItems); i++ {
		m.commandIndex = moveIndex(m.commandIndex, len(m.commandItems), delta)
		item := m.commandItems[m.commandIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedCommandItem() (commandItem, bool) {
	if m.commandIndex < 0 || m.commandIndex >= len(m.commandItems) {
		return commandItem{}, false
	}
	item := m.commandItems[m.commandIndex]
	if item.Header || item.Spacer {
		return commandItem{}, false
	}
	return item, true
}

func (m Model) startCommandEdit(item commandItem, editing bool) Model {
	scope := commandScopeServer
	name := ""
	body := ""
	if editing {
		scope = item.Scope
	}
	if item.Temporary {
		scope = commandScopeServer
	}
	if editing {
		name = item.Name
		body = item.Command
	}
	m.commandForm = commandEditForm{Scope: scope, Name: name, Command: body}
	m.commandField = 0
	m.commandCursor = len([]rune(name))
	m.commandEditing = editing
	m.commandEditItem = item
	m.mode = modeCommandEdit
	if item.Temporary {
		m.commandForm.Name = "临时命令"
		m.commandField = 2
		m.commandCursor = 0
	}
	m.status = "编辑命令模板"
	if !editing {
		m.status = "添加命令模板"
	}
	return m
}

func (m Model) updateCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		return m.backToCommandList("已取消。"), nil
	case "tab":
		m.commandField = (m.commandField + 1) % 3
		m.commandCursor = m.commandValueLen()
	case "shift+tab":
		m.commandField--
		if m.commandField < 0 {
			m.commandField = 2
		}
		m.commandCursor = m.commandValueLen()
	case "up":
		if m.commandField == 2 {
			m.moveCommandBodyLine(-1)
		} else {
			m.commandField--
			if m.commandField < 0 {
				m.commandField = 2
			}
			m.commandCursor = m.commandValueLen()
		}
	case "down":
		if m.commandField == 2 {
			m.moveCommandBodyLine(1)
		} else {
			m.commandField = (m.commandField + 1) % 3
			m.commandCursor = m.commandValueLen()
		}
	case "left":
		if m.commandField == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(-1)
		}
	case "right":
		if m.commandField == 0 {
			m.toggleCommandScope()
		} else {
			m.moveCommandCursor(1)
		}
	case "ctrl+j":
		if m.commandField == 2 {
			m.commandAppend("\n")
		}
	case "enter":
		if strings.TrimSpace(m.commandForm.Command) == "" {
			m.status = "保存失败：命令内容不能为空"
			return m, nil
		}
		if m.commandEditItem.Temporary {
			m.commandConfirm = commandItem{Name: "临时命令", Command: m.commandForm.Command, Temporary: true}
			m.commandOutputScroll = 0
			m.mode = modeCommandConfirm
			m.status = "确认执行命令"
			return m, nil
		}
		if err := config.ValidateCommandTemplate(m.commandForm.Name, m.commandForm.Command); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		if err := m.saveCommandTemplate(); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		return m.backToCommandList("命令模板已保存。"), nil
	case "backspace":
		m.commandBackspace()
	default:
		if len(msg.Runes) > 0 && m.commandField != 0 {
			m.commandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) backToCommandList(status string) Model {
	index := m.activeCommand.HostIndex
	if index < 0 {
		if selected, ok := m.selectedRealIndex(); ok {
			index = selected
		}
	}
	m = m.startCommandList(index)
	m.status = status
	return m
}

func (m *Model) toggleCommandScope() {
	if m.commandForm.Scope == commandScopeGlobal {
		m.commandForm.Scope = commandScopeServer
	} else {
		m.commandForm.Scope = commandScopeGlobal
	}
}

func (m Model) commandValue() string {
	switch m.commandField {
	case 1:
		return m.commandForm.Name
	case 2:
		return m.commandForm.Command
	default:
		return ""
	}
}

func (m Model) commandValueLen() int {
	return len([]rune(m.commandValue()))
}

func (m *Model) setCommandValue(value string) {
	switch m.commandField {
	case 1:
		m.commandForm.Name = value
	case 2:
		m.commandForm.Command = value
	}
}

func (m *Model) commandAppend(s string) {
	value := []rune(m.commandValue())
	if m.commandCursor < 0 {
		m.commandCursor = 0
	}
	if m.commandCursor > len(value) {
		m.commandCursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.commandCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.commandCursor:]...)
	m.setCommandValue(string(next))
	m.commandCursor += len(insert)
}

func (m *Model) commandBackspace() {
	if m.commandField == 0 {
		return
	}
	value := []rune(m.commandValue())
	if m.commandCursor <= 0 || len(value) == 0 {
		return
	}
	if m.commandCursor > len(value) {
		m.commandCursor = len(value)
	}
	next := append([]rune{}, value[:m.commandCursor-1]...)
	next = append(next, value[m.commandCursor:]...)
	m.setCommandValue(string(next))
	m.commandCursor--
}

func (m *Model) moveCommandCursor(delta int) {
	m.commandCursor += delta
	if m.commandCursor < 0 {
		m.commandCursor = 0
	}
	if maxCursor := m.commandValueLen(); m.commandCursor > maxCursor {
		m.commandCursor = maxCursor
	}
}

func (m *Model) moveCommandBodyLine(delta int) {
	if m.commandField != 2 {
		return
	}
	runes := []rune(m.commandForm.Command)
	if len(runes) == 0 {
		return
	}
	lineStart := 0
	for i := m.commandCursor - 1; i >= 0 && i < len(runes); i-- {
		if runes[i] == '\n' {
			lineStart = i + 1
			break
		}
	}
	col := m.commandCursor - lineStart
	if delta < 0 {
		if lineStart == 0 {
			return
		}
		prevEnd := lineStart - 1
		prevStart := 0
		for i := prevEnd - 1; i >= 0; i-- {
			if runes[i] == '\n' {
				prevStart = i + 1
				break
			}
		}
		m.commandCursor = prevStart + minInt(col, prevEnd-prevStart)
		return
	}
	lineEnd := len(runes)
	for i := m.commandCursor; i < len(runes); i++ {
		if runes[i] == '\n' {
			lineEnd = i
			break
		}
	}
	if lineEnd >= len(runes) {
		return
	}
	nextStart := lineEnd + 1
	nextEnd := len(runes)
	for i := nextStart; i < len(runes); i++ {
		if runes[i] == '\n' {
			nextEnd = i
			break
		}
	}
	m.commandCursor = nextStart + minInt(col, nextEnd-nextStart)
}

func (m Model) saveCommandTemplate() error {
	file := m.commandFile
	name := strings.TrimSpace(m.commandForm.Name)
	body := strings.TrimSpace(m.commandForm.Command)
	serverKey := ""
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		h := m.states[m.activeCommand.HostIndex].Host
		serverKey = config.ServerCommandKey(h.Category, h.Name)
	}
	if m.commandEditing {
		item := m.commandEditItem
		if item.Scope == commandScopeGlobal && item.Index >= 0 && item.Index < len(file.Global) {
			file.Global = append(file.Global[:item.Index], file.Global[item.Index+1:]...)
		}
		if item.Scope == commandScopeServer && item.Index >= 0 && item.Index < len(file.Server) {
			file.Server = append(file.Server[:item.Index], file.Server[item.Index+1:]...)
		}
	}
	if m.commandForm.Scope == commandScopeGlobal {
		file.Global = append(file.Global, config.CommandTemplate{Name: name, Command: body})
	} else {
		file.Server = append(file.Server, config.ServerCommandTemplate{Server: serverKey, Name: name, Command: body})
	}
	if err := config.SaveCommands(m.home, file); err != nil {
		return err
	}
	m.commandFile = file
	return nil
}

func (m Model) deleteCommandTemplate(item commandItem) (tea.Model, tea.Cmd) {
	file := m.commandFile
	if item.Scope == commandScopeGlobal && item.Index >= 0 && item.Index < len(file.Global) {
		file.Global = append(file.Global[:item.Index], file.Global[item.Index+1:]...)
	}
	if item.Scope == commandScopeServer && item.Index >= 0 && item.Index < len(file.Server) {
		file.Server = append(file.Server[:item.Index], file.Server[item.Index+1:]...)
	}
	if err := config.SaveCommands(m.home, file); err != nil {
		m.status = "删除失败：" + err.Error()
		return m, nil
	}
	m.commandFile = file
	m.commandItems = m.commandListItems(m.activeCommand.HostIndex)
	m.commandIndex = firstCommandItem(m.commandItems)
	m.status = "命令模板已删除。"
	return m, nil
}

func (m Model) updateCommandConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeCommandList
		m.status = "已取消。"
	case "j", "down":
		m.commandOutputScroll = clampInt(m.commandOutputScroll+1, 0, m.commandConfirmMaxScroll())
	case "k", "up":
		m.commandOutputScroll = clampInt(m.commandOutputScroll-1, 0, m.commandConfirmMaxScroll())
	case "enter":
		if m.activeCommand.HostIndex < 0 || m.activeCommand.HostIndex >= len(m.states) {
			m.status = "没有选中的服务器。"
			return m, nil
		}
		m.activeCommand.Name = m.commandConfirm.Name
		m.activeCommand.Command = m.commandConfirm.Command
		m.activeCommand.Output = ""
		m.activeCommand.ExitCode = 0
		m.activeCommand.Running = true
		m.commandOutputScroll = 0
		m.commandOutputBack = modeDashboard
		m.mode = modeCommandOutput
		m.status = "正在执行命令..."
		return m, m.runCommand(m.activeCommand.HostIndex, m.commandConfirm.Command)
	}
	return m, nil
}

func (m Model) updateCommandOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = m.commandOutputBack
		m.status = ""
	case "j", "down":
		m.commandOutputScroll = clampInt(m.commandOutputScroll+1, 0, m.commandOutputMaxScroll())
	case "k", "up":
		m.commandOutputScroll = clampInt(m.commandOutputScroll-1, 0, m.commandOutputMaxScroll())
	}
	return m, nil
}

func (m Model) updateBatchSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "down":
		m.batchCursor = clampInt(m.batchCursor+1, 0, maxInt(0, len(m.batchIndexes)-1))
	case "k", "up":
		m.batchCursor = clampInt(m.batchCursor-1, 0, maxInt(0, len(m.batchIndexes)-1))
	case " ":
		if m.batchCursor >= 0 && m.batchCursor < len(m.batchIndexes) {
			index := m.batchIndexes[m.batchCursor]
			if m.batchSelected[index] {
				delete(m.batchSelected, index)
			} else {
				m.batchSelected[index] = true
			}
		}
	case "a":
		for _, index := range m.batchIndexes {
			m.batchSelected[index] = true
		}
	case "x":
		m.batchSelected = map[int]bool{}
	case "enter":
		if m.batchSelectedCount() == 0 {
			m.status = "请至少选择一台服务器"
			return m, nil
		}
		return m.startBatchCommandList()
	}
	return m, nil
}

func (m Model) startBatchCommandList() (tea.Model, tea.Cmd) {
	file, _, err := config.LoadCommands(m.home)
	if err != nil {
		m.status = "读取命令模板失败：" + err.Error()
		return m, nil
	}
	m.commandFile = file
	m.batchCommandItems = m.batchGlobalCommandItems()
	m.batchCommandIndex = firstCommandItem(m.batchCommandItems)
	m.mode = modeBatchCommandList
	m.status = "选择批量命令模板"
	return m, nil
}

func (m Model) batchGlobalCommandItems() []commandItem {
	items := []commandItem{{Name: "全局", Header: true}}
	for i, command := range m.commandFile.Global {
		items = append(items, commandItem{
			Scope:   commandScopeGlobal,
			Index:   i,
			Name:    command.Name,
			Command: command.Command,
		})
	}
	items = append(items, commandItem{Spacer: true}, commandItem{Name: "临时命令", Temporary: true})
	return items
}

func (m Model) updateBatchCommandList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeBatchSelect
	case "j", "down":
		m.moveBatchCommandIndex(1)
	case "k", "up":
		m.moveBatchCommandIndex(-1)
	case "enter":
		item, ok := m.selectedBatchCommandItem()
		if !ok {
			return m, nil
		}
		if item.Temporary {
			m.commandForm = commandEditForm{Name: "临时命令"}
			m.commandField = 2
			m.commandCursor = 0
			m.mode = modeBatchCommandEdit
			m.status = "输入批量临时命令"
			return m, nil
		}
		m.batchCommand = item
		m.mode = modeBatchConfirm
		m.batchOutputScroll = 0
		m.status = "确认批量执行"
	}
	return m, nil
}

func (m *Model) moveBatchCommandIndex(delta int) {
	if len(m.batchCommandItems) == 0 {
		m.batchCommandIndex = 0
		return
	}
	for i := 0; i < len(m.batchCommandItems); i++ {
		m.batchCommandIndex = moveIndex(m.batchCommandIndex, len(m.batchCommandItems), delta)
		item := m.batchCommandItems[m.batchCommandIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedBatchCommandItem() (commandItem, bool) {
	if m.batchCommandIndex < 0 || m.batchCommandIndex >= len(m.batchCommandItems) {
		return commandItem{}, false
	}
	item := m.batchCommandItems[m.batchCommandIndex]
	if item.Header || item.Spacer {
		return commandItem{}, false
	}
	return item, true
}

func (m Model) updateBatchCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeBatchCommandList
		m.status = "已取消。"
	case "ctrl+j":
		m.commandAppend("\n")
	case "up":
		m.moveCommandBodyLine(-1)
	case "down":
		m.moveCommandBodyLine(1)
	case "left":
		m.moveCommandCursor(-1)
	case "right":
		m.moveCommandCursor(1)
	case "backspace":
		m.commandBackspace()
	case "enter":
		if strings.TrimSpace(m.commandForm.Command) == "" {
			m.status = "命令内容不能为空"
			return m, nil
		}
		m.batchCommand = commandItem{Name: "临时命令", Command: m.commandForm.Command, Temporary: true}
		m.mode = modeBatchConfirm
		m.status = "确认批量执行"
	default:
		if len(msg.Runes) > 0 {
			m.commandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) updateBatchConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.batchCommand.Temporary {
			m.commandForm = commandEditForm{Name: "临时命令", Command: m.batchCommand.Command}
			m.commandField = 2
			m.commandCursor = len([]rune(m.commandForm.Command))
			m.mode = modeBatchCommandEdit
		} else {
			m.mode = modeBatchCommandList
		}
	case "j", "down":
		m.batchOutputScroll = clampInt(m.batchOutputScroll+1, 0, m.batchConfirmMaxScroll())
	case "k", "up":
		m.batchOutputScroll = clampInt(m.batchOutputScroll-1, 0, m.batchConfirmMaxScroll())
	case "enter":
		m.prepareBatchJobs()
		if len(m.batchJobs) == 0 {
			m.status = "没有可执行的服务器"
			return m, nil
		}
		m.mode = modeBatchOutput
		m.batchCurrent = 0
		m.batchJobs[0].Running = true
		m.batchOutputIndex = 0
		m.batchOutputScroll = 0
		m.batchOutputBack = modeBatchCommandList
		m.status = "批量命令执行中..."
		return m, m.runBatchJob(0)
	}
	return m, nil
}

func (m Model) updateBatchOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.batchRunning() {
			m.status = "批量命令执行中，完成后再返回"
			return m, nil
		}
		m.mode = m.batchOutputBack
		if m.mode == modeBatchCommandList {
			m.status = "可继续选择批量命令"
		} else {
			m.status = ""
		}
	case "j", "down":
		m.moveBatchOutputIndex(1)
		m.batchOutputScroll = 0
	case "k", "up":
		m.moveBatchOutputIndex(-1)
		m.batchOutputScroll = 0
	case "right", "l":
		m.batchOutputScroll = clampInt(m.batchOutputScroll+1, 0, m.batchOutputMaxScroll())
	case "left", "h":
		m.batchOutputScroll = clampInt(m.batchOutputScroll-1, 0, m.batchOutputMaxScroll())
	case "r":
		if m.batchRunning() {
			m.status = "批量命令执行中，完成后再重试"
			return m, nil
		}
		return m.retryFailedBatchJobs()
	}
	return m, nil
}

func (m *Model) moveBatchOutputIndex(delta int) {
	indexes := m.batchResultDisplayIndexes()
	if len(indexes) == 0 {
		m.batchOutputIndex = 0
		return
	}
	pos := 0
	for i, index := range indexes {
		if index == m.batchOutputIndex {
			pos = i
			break
		}
	}
	pos = clampInt(pos+delta, 0, len(indexes)-1)
	m.batchOutputIndex = indexes[pos]
}

func (m Model) retryFailedBatchJobs() (tea.Model, tea.Cmd) {
	jobs := make([]batchJob, 0)
	for _, job := range m.batchJobs {
		if job.Done && job.Err != nil {
			jobs = append(jobs, batchJob{HostIndex: job.HostIndex})
		}
	}
	if len(jobs) == 0 {
		m.status = "没有失败的服务器需要重试"
		return m, nil
	}
	m.batchJobs = jobs
	m.batchCurrent = 0
	m.batchJobs[0].Running = true
	m.batchOutputIndex = 0
	m.batchOutputScroll = 0
	m.status = "正在重试失败服务器..."
	return m, m.runBatchJob(0)
}

func (m Model) startCommandHistory() (tea.Model, tea.Cmd) {
	file, _, err := config.LoadCommandHistory(m.home)
	if err != nil {
		m.status = "读取命令历史失败：" + err.Error()
		return m, nil
	}
	m.commandHistory = file
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(file.Entries)-1))
	m.historyScroll = 0
	m.mode = modeCommandHistory
	m.status = ""
	return m, nil
}

func (m *Model) reloadCommandHistory() {
	file, _, err := config.LoadCommandHistory(m.home)
	if err != nil {
		return
	}
	m.commandHistory = file
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(file.Entries)-1))
}

func (m Model) updateCommandHistory(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.historySearch {
		switch msg.String() {
		case "esc":
			m.historySearch = false
			m.historyQuery = ""
			m.historyIndex = 0
		case "enter":
			m.historySearch = false
		case "backspace":
			runes := []rune(m.historyQuery)
			if len(runes) > 0 {
				m.historyQuery = string(runes[:len(runes)-1])
				m.historyIndex = 0
			}
		default:
			if len(msg.Runes) > 0 {
				m.historyQuery += string(msg.Runes)
				m.historyIndex = 0
			}
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = ""
	case "/":
		m.historySearch = true
		m.historyQuery = ""
		m.historyIndex = 0
	case "j", "down":
		m.historyIndex = clampInt(m.historyIndex+1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "k", "up":
		m.historyIndex = clampInt(m.historyIndex-1, 0, maxInt(0, len(m.filteredHistoryEntries())-1))
	case "enter":
		if _, ok := m.selectedHistoryEntry(); ok {
			m.historyScroll = 0
			m.mode = modeCommandHistoryDetail
		}
	case "r":
		if entry, ok := m.selectedHistoryEntry(); ok {
			return m.rerunHistoryEntry(entry)
		}
	case "x":
		if entry, ok := m.selectedHistoryEntry(); ok {
			m.confirm = confirmAction{
				Kind:    confirmDeleteHistory,
				Title:   "确认删除命令历史",
				Lines:   []string{"将删除该命令历史记录。", "命令：" + entry.Name},
				Back:    modeCommandHistory,
				History: entry,
			}
			m.mode = modeConfirmAction
		}
	}
	return m, nil
}

func (m Model) updateCommandHistoryDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeCommandHistory
	case "j", "down":
		m.historyScroll = clampInt(m.historyScroll+1, 0, m.commandHistoryDetailMaxScroll())
	case "k", "up":
		m.historyScroll = clampInt(m.historyScroll-1, 0, m.commandHistoryDetailMaxScroll())
	case "r":
		if entry, ok := m.selectedHistoryEntry(); ok {
			return m.rerunHistoryEntry(entry)
		}
	case "x":
		if entry, ok := m.selectedHistoryEntry(); ok {
			m.confirm = confirmAction{
				Kind:    confirmDeleteHistory,
				Title:   "确认删除命令历史",
				Lines:   []string{"将删除该命令历史记录。", "命令：" + entry.Name},
				Back:    modeCommandHistoryDetail,
				History: entry,
			}
			m.mode = modeConfirmAction
		}
	}
	return m, nil
}

func (m Model) selectedHistoryEntry() (config.CommandHistoryEntry, bool) {
	entries := m.filteredHistoryEntries()
	if m.historyIndex < 0 || m.historyIndex >= len(entries) {
		return config.CommandHistoryEntry{}, false
	}
	return entries[m.historyIndex], true
}

func (m Model) filteredHistoryEntries() []config.CommandHistoryEntry {
	query := strings.ToLower(strings.TrimSpace(m.historyQuery))
	if query == "" {
		return m.commandHistory.Entries
	}
	out := make([]config.CommandHistoryEntry, 0, len(m.commandHistory.Entries))
	for _, entry := range m.commandHistory.Entries {
		if historyEntryMatches(entry, query) {
			out = append(out, entry)
		}
	}
	return out
}

func historyEntryMatches(entry config.CommandHistoryEntry, query string) bool {
	values := []string{entry.Name, entry.Command, entry.Kind, entry.Status}
	for _, target := range entry.Targets {
		values = append(values, target.Category, target.Name, target.HostName, target.User)
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (m Model) deleteCommandHistoryEntry(entry config.CommandHistoryEntry) (tea.Model, tea.Cmd) {
	if err := config.DeleteCommandHistoryEntry(m.home, entry.ID); err != nil {
		m.status = "删除命令历史失败：" + err.Error()
		return m, nil
	}
	m.reloadCommandHistory()
	m.historyIndex = clampInt(m.historyIndex, 0, maxInt(0, len(m.commandHistory.Entries)-1))
	m.status = "命令历史已删除。"
	if len(m.commandHistory.Entries) == 0 {
		m.mode = modeCommandHistory
	}
	return m, nil
}

func (m Model) rerunHistoryEntry(entry config.CommandHistoryEntry) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(entry.Command) == "" {
		m.status = "历史命令为空，不能重新执行。"
		return m, nil
	}
	indexes := m.historyTargetIndexes(entry)
	if len(indexes) == 0 {
		m.status = "服务器不存在，不能重新执行。"
		return m, nil
	}
	if len(indexes) == 1 {
		backMode := m.mode
		m.activeCommand = activeCommand{
			HostIndex: indexes[0],
			Name:      historyCommandName(entry),
			Command:   entry.Command,
			Running:   true,
		}
		m.commandOutputScroll = 0
		m.commandOutputBack = backMode
		m.mode = modeCommandOutput
		m.status = "正在重新执行命令..."
		return m, m.runCommand(indexes[0], entry.Command)
	}
	backMode := m.mode
	m.batchSelected = map[int]bool{}
	for _, index := range indexes {
		m.batchSelected[index] = true
	}
	m.batchIndexes = indexes
	m.batchCommand = commandItem{Name: historyCommandName(entry), Command: entry.Command}
	m.prepareBatchJobs()
	if len(m.batchJobs) == 0 {
		m.status = "没有可执行的服务器"
		return m, nil
	}
	m.mode = modeBatchOutput
	m.batchCurrent = 0
	m.batchJobs[0].Running = true
	m.batchOutputIndex = 0
	m.batchOutputScroll = 0
	m.batchOutputBack = backMode
	m.status = "正在重新批量执行..."
	return m, m.runBatchJob(0)
}

func (m Model) historyTargetIndexes(entry config.CommandHistoryEntry) []int {
	indexes := []int{}
	seen := map[int]bool{}
	for _, target := range entry.Targets {
		if index, ok := m.findHostByHistoryTarget(target); ok && !seen[index] {
			indexes = append(indexes, index)
			seen[index] = true
		}
	}
	return indexes
}

func (m Model) findHostByHistoryTarget(target config.CommandHistoryTarget) (int, bool) {
	for i, state := range m.states {
		h := state.Host
		if strings.TrimSpace(h.Category) == strings.TrimSpace(target.Category) &&
			strings.TrimSpace(h.Name) == strings.TrimSpace(target.Name) {
			return i, true
		}
	}
	return 0, false
}

func (m Model) batchRunning() bool {
	for _, job := range m.batchJobs {
		if job.Running {
			return true
		}
	}
	return false
}

func (m Model) updateHelpPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "?":
		if m.helpBackMode == 0 {
			m.helpBackMode = modeDashboard
		}
		m.mode = m.helpBackMode
	}
	return m, nil
}

func (m Model) commandOutputMaxScroll() int {
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	lines := 2
	if m.activeCommand.Running {
		lines++
	} else {
		output := strings.TrimRight(m.activeCommand.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(strings.Split(output, "\n"))
		}
		lines += 2
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) runCommand(index int, script string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, m.states[index].Host, script)
		cleanup()
		return commandDoneMsg{Result: result}
	}
}

func (m Model) batchSelectedCount() int {
	count := 0
	for _, selected := range m.batchSelected {
		if selected {
			count++
		}
	}
	return count
}

func (m Model) selectedBatchHostIndexes() []int {
	indexes := make([]int, 0, m.batchSelectedCount())
	for _, index := range m.batchIndexes {
		if m.batchSelected[index] {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (m *Model) prepareBatchJobs() {
	indexes := m.selectedBatchHostIndexes()
	m.batchJobs = make([]batchJob, 0, len(indexes))
	for _, index := range indexes {
		m.batchJobs = append(m.batchJobs, batchJob{HostIndex: index})
	}
}

func (m Model) runBatchJob(job int) tea.Cmd {
	if job < 0 || job >= len(m.batchJobs) {
		return nil
	}
	hostIndex := m.batchJobs[job].HostIndex
	if hostIndex < 0 || hostIndex >= len(m.states) {
		return func() tea.Msg {
			return batchCommandDoneMsg{Job: job, Result: actions.CommandResult{ExitCode: -1, Err: fmt.Errorf("服务器索引无效")}}
		}
	}
	h := m.states[hostIndex].Host
	script := m.batchCommand.Command
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, script)
		cleanup()
		return batchCommandDoneMsg{Job: job, Result: result}
	}
}

func (m Model) handleBatchCommandDone(msg batchCommandDoneMsg) (tea.Model, tea.Cmd) {
	if msg.Job < 0 || msg.Job >= len(m.batchJobs) {
		return m, nil
	}
	m.batchJobs[msg.Job].Running = false
	m.batchJobs[msg.Job].Done = true
	m.batchJobs[msg.Job].Output = msg.Result.Output
	m.batchJobs[msg.Job].ExitCode = msg.Result.ExitCode
	m.batchJobs[msg.Job].Err = msg.Result.Err
	next := msg.Job + 1
	if next < len(m.batchJobs) {
		m.batchCurrent = next
		m.batchJobs[next].Running = true
		m.batchOutputIndex = next
		m.batchOutputScroll = 0
		return m, m.runBatchJob(next)
	}
	m.batchCurrent = len(m.batchJobs)
	m.status = fmt.Sprintf("批量执行完成：成功%d  失败%d", m.batchSuccessCount(), m.batchFailCount())
	if err := m.recordBatchCommandHistory(); err != nil {
		m.status += " 历史保存失败：" + err.Error()
	}
	return m, nil
}

func (m *Model) recordCommandHistory(result actions.CommandResult) error {
	if m.activeCommand.HostIndex < 0 || m.activeCommand.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeCommand.HostIndex].Host
	status := commandHistoryStatus(result.Err)
	entry := config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(time.Now()),
		Time:     time.Now().Format(time.RFC3339),
		Kind:     "single",
		Name:     m.activeCommand.Name,
		Command:  m.activeCommand.Command,
		Status:   status,
		ExitCode: result.ExitCode,
		Targets: []config.CommandHistoryTarget{
			config.CommandHistoryTargetFromHost(h, status, result.ExitCode, result.Output),
		},
	}
	if err := config.AppendCommandHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func (m *Model) recordBatchCommandHistory() error {
	targets := make([]config.CommandHistoryTarget, 0, len(m.batchJobs))
	failCount := 0
	for _, job := range m.batchJobs {
		if job.HostIndex < 0 || job.HostIndex >= len(m.states) {
			continue
		}
		status := commandHistoryStatus(job.Err)
		if job.Err != nil {
			failCount++
		}
		targets = append(targets, config.CommandHistoryTargetFromHost(m.states[job.HostIndex].Host, status, job.ExitCode, job.Output))
	}
	if len(targets) == 0 {
		return nil
	}
	status := "success"
	if failCount > 0 {
		status = "failed"
	}
	entry := config.CommandHistoryEntry{
		ID:       config.NewCommandHistoryID(time.Now()),
		Time:     time.Now().Format(time.RFC3339),
		Kind:     "batch",
		Name:     m.batchCommand.Name,
		Command:  m.batchCommand.Command,
		Status:   status,
		ExitCode: failCount,
		Targets:  targets,
	}
	if err := config.AppendCommandHistory(m.home, entry); err != nil {
		return err
	}
	m.reloadCommandHistory()
	return nil
}

func commandHistoryStatus(err error) string {
	if err != nil {
		return "failed"
	}
	return "success"
}

func (m Model) startDeploymentList(index int) Model {
	file, _, err := config.LoadDeployments(m.home)
	if err != nil {
		m.status = "读取部署配置失败：" + err.Error()
		return m
	}
	m.deploymentFile = file
	if m.deploymentCategory == "" && index >= 0 && index < len(m.states) && m.category != "" {
		m.deploymentCategory = m.category
	}
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
	m.activeDeployment = activeDeployment{HostIndex: index}
	m.deploymentSelected = filterDeploymentSelection(m.deploymentSelected, file.Apps)
	m.deploymentOutputScroll = 0
	m.mode = modeDeploymentList
	m.status = "应用部署"
	return m
}

func (m Model) deploymentListItems() []deploymentItem {
	items := []deploymentItem{}
	for i, app := range m.deploymentFile.Apps {
		if m.deploymentCategory != "" && deploymentAppCategory(app) != m.deploymentCategory {
			continue
		}
		if m.deploymentFavoriteOnly && !app.Favorite {
			continue
		}
		items = append(items, deploymentItem{Index: i, App: app})
	}
	sort.SliceStable(items, func(i, j int) bool {
		a := items[i].App
		b := items[j].App
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if a.Pinned && b.Pinned && a.PinnedOrder != b.PinnedOrder {
			return a.PinnedOrder > b.PinnedOrder
		}
		return items[i].Index < items[j].Index
	})
	return items
}

func deploymentAppCategory(app config.DeploymentApp) string {
	server := strings.TrimSpace(app.Server)
	if idx := strings.Index(server, "/"); idx >= 0 {
		return server[:idx]
	}
	return ""
}

func firstDeploymentItem(items []deploymentItem) int {
	for i, item := range items {
		if !item.Header && !item.Spacer {
			return i
		}
	}
	return 0
}

func (m Model) updateDeploymentList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = ""
	case "j", "down":
		m.moveDeploymentIndex(m.deploymentMoveStep(1, false))
	case "k", "up":
		m.moveDeploymentIndex(m.deploymentMoveStep(-1, false))
	case "h", "left":
		m.moveDeploymentIndex(-1)
	case "l", "right":
		m.moveDeploymentIndex(1)
	case " ":
		item, ok := m.selectedDeploymentItem()
		if ok {
			m.deploymentDetail = item.App
			m.deploymentOutputScroll = 0
			m.mode = modeDeploymentDetail
			m.status = "部署详情"
		}
	case "s":
		item, ok := m.selectedDeploymentItem()
		if ok {
			m.toggleDeploymentSelection(item.Index)
		}
	case "tab":
		m.cycleDeploymentCategory(1)
	case "z":
		m.toggleDeploymentView()
	case "f":
		return m.toggleDeploymentFavorite()
	case "v":
		m.toggleDeploymentFavoriteFilter()
	case "t":
		return m.toggleDeploymentPinned()
	case "a":
		return m.startDeploymentEdit(config.DeploymentApp{}, false), nil
	case "e":
		item, ok := m.selectedDeploymentItem()
		if ok {
			return m.startDeploymentEdit(item.App, true), nil
		}
		m.status = "没有可编辑的部署应用。按 a 新增。"
	case "x":
		item, ok := m.selectedDeploymentItem()
		if ok {
			if item.Index >= 0 && item.Index < len(m.deploymentFile.Apps) {
				m.confirm = confirmAction{
					Kind:       confirmDeleteDeployment,
					Title:      "确认删除部署应用",
					Lines:      []string{"将删除该部署应用。", "应用：" + item.App.Name, "服务器：" + item.App.Server},
					Back:       modeDeploymentList,
					Deployment: item.App,
					Index:      item.Index,
				}
				m.mode = modeConfirmAction
				m.status = "确认删除部署应用"
				return m, nil
			}
		}
		m.status = "没有可删除的部署应用。"
	case "enter":
		item, ok := m.selectedDeploymentItem()
		if !ok {
			m.status = "没有可部署的应用。按 a 新增，保存后再按 Enter。"
			return m, nil
		}
		queue := m.selectedDeploymentQueue()
		if len(queue) == 0 {
			queue = []config.DeploymentApp{item.App}
		}
		for i := range queue {
			queue[i] = deploymentAppWithResourceDefaults(queue[i])
		}
		m.deploymentConfirmQueue = queue
		m.deploymentConfirm = queue[0]
		m.activeDeployment = activeDeployment{HostIndex: m.activeDeployment.HostIndex}
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentConfirm
		m.status = "确认部署"
	}
	return m, nil
}

func (m Model) deploymentMoveStep(delta int, horizontal bool) int {
	if m.deploymentView != deploymentViewCards || horizontal {
		return delta
	}
	return delta * m.dashboardColumns()
}

func (m *Model) toggleDeploymentView() {
	if m.deploymentView == deploymentViewCards {
		m.deploymentView = deploymentViewList
		m.status = ""
		return
	}
	m.deploymentView = deploymentViewCards
	m.status = ""
}

func (m *Model) toggleDeploymentFavoriteFilter() {
	m.deploymentFavoriteOnly = !m.deploymentFavoriteOnly
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
	if m.deploymentFavoriteOnly {
		m.status = "筛选：收藏部署"
	} else {
		m.status = "已取消收藏筛选"
	}
}

func (m Model) toggleDeploymentFavorite() (tea.Model, tea.Cmd) {
	item, ok := m.selectedDeploymentItem()
	if !ok {
		m.status = "没有可收藏的部署应用"
		return m, nil
	}
	file := m.deploymentFile
	if item.Index < 0 || item.Index >= len(file.Apps) {
		m.status = "部署应用不存在"
		return m, nil
	}
	file.Apps[item.Index].Favorite = !file.Apps[item.Index].Favorite
	if err := config.SaveDeployments(m.home, file); err != nil {
		m.status = "收藏更新失败：" + err.Error()
		return m, nil
	}
	m.deploymentFile = file
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = m.deploymentVisibleIndex(item.Index)
	if file.Apps[item.Index].Favorite {
		m.status = "已收藏：" + file.Apps[item.Index].Name
	} else {
		m.status = "已取消收藏：" + file.Apps[item.Index].Name
	}
	if m.deploymentFavoriteOnly && !file.Apps[item.Index].Favorite {
		m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
	}
	return m, nil
}

func (m Model) toggleDeploymentPinned() (tea.Model, tea.Cmd) {
	item, ok := m.selectedDeploymentItem()
	if !ok {
		m.status = "没有可置顶的部署应用"
		return m, nil
	}
	file := m.deploymentFile
	if item.Index < 0 || item.Index >= len(file.Apps) {
		m.status = "部署应用不存在"
		return m, nil
	}
	if file.Apps[item.Index].Pinned {
		file.Apps[item.Index].Pinned = false
		file.Apps[item.Index].PinnedOrder = 0
	} else {
		file.Apps[item.Index].Pinned = true
		file.Apps[item.Index].PinnedOrder = nextDeploymentPinnedOrder(file.Apps)
	}
	if err := config.SaveDeployments(m.home, file); err != nil {
		m.status = "置顶更新失败：" + err.Error()
		return m, nil
	}
	m.deploymentFile = file
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = m.deploymentVisibleIndex(item.Index)
	if file.Apps[item.Index].Pinned {
		m.status = "已置顶：" + file.Apps[item.Index].Name
	} else {
		m.status = "已取消置顶：" + file.Apps[item.Index].Name
	}
	return m, nil
}

func nextDeploymentPinnedOrder(apps []config.DeploymentApp) int64 {
	var maxOrder int64
	for _, app := range apps {
		if app.PinnedOrder > maxOrder {
			maxOrder = app.PinnedOrder
		}
	}
	return maxOrder + 1
}

func (m Model) deploymentVisibleIndex(appIndex int) int {
	for i, item := range m.deploymentItems {
		if item.Index == appIndex {
			return i
		}
	}
	return firstDeploymentItem(m.deploymentItems)
}

func (m Model) updateDeploymentDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentDetailMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentDetailMaxScroll())
	case "e":
		return m.startDeploymentEdit(m.deploymentDetail, true), nil
	case "enter":
		app := deploymentAppWithResourceDefaults(m.deploymentDetail)
		m.deploymentConfirmQueue = []config.DeploymentApp{app}
		m.deploymentConfirm = app
		m.activeDeployment = activeDeployment{HostIndex: m.activeDeployment.HostIndex}
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentConfirm
		m.status = "确认部署"
	}
	return m, nil
}

func filterDeploymentSelection(selection []int, apps []config.DeploymentApp) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, index := range selection {
		if index >= 0 && index < len(apps) && !seen[index] {
			seen[index] = true
			out = append(out, index)
		}
	}
	return out
}

func removeDeploymentSelection(selection []int, removed int) []int {
	out := []int{}
	for _, index := range selection {
		if index == removed {
			continue
		}
		if index > removed {
			index--
		}
		out = append(out, index)
	}
	return out
}

func (m Model) deleteDeploymentApp(index int) (tea.Model, tea.Cmd) {
	file := m.deploymentFile
	if index < 0 || index >= len(file.Apps) {
		m.status = "没有可删除的部署应用。"
		return m, nil
	}
	file.Apps = append(file.Apps[:index], file.Apps[index+1:]...)
	if err := config.SaveDeployments(m.home, file); err != nil {
		m.status = "删除部署应用失败：" + err.Error()
		return m, nil
	}
	m.confirm = confirmAction{}
	m.deploymentSelected = removeDeploymentSelection(m.deploymentSelected, index)
	m = m.startDeploymentList(m.activeDeployment.HostIndex)
	m.status = "部署应用已删除。"
	return m, nil
}

func (m *Model) toggleDeploymentSelection(index int) {
	for i, selected := range m.deploymentSelected {
		if selected == index {
			m.deploymentSelected = append(m.deploymentSelected[:i], m.deploymentSelected[i+1:]...)
			m.status = "已取消选择"
			return
		}
	}
	m.deploymentSelected = append(m.deploymentSelected, index)
	m.status = fmt.Sprintf("已选择第 %d 个部署应用", len(m.deploymentSelected))
}

func (m Model) deploymentSelectionOrder(index int) int {
	for i, selected := range m.deploymentSelected {
		if selected == index {
			return i + 1
		}
	}
	return 0
}

func (m Model) selectedDeploymentQueue() []config.DeploymentApp {
	queue := []config.DeploymentApp{}
	for _, index := range m.deploymentSelected {
		if index >= 0 && index < len(m.deploymentFile.Apps) {
			app := m.deploymentFile.Apps[index]
			if m.deploymentCategory == "" || deploymentAppCategory(app) == m.deploymentCategory {
				queue = append(queue, app)
			}
		}
	}
	return queue
}

func (m *Model) cycleDeploymentCategory(delta int) {
	categories := []string{""}
	seen := map[string]bool{}
	for _, app := range m.deploymentFile.Apps {
		cat := deploymentAppCategory(app)
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories[1:])
	if len(categories) <= 1 {
		m.deploymentCategory = ""
		m.deploymentItems = m.deploymentListItems()
		m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
		m.status = "没有可切换的部署分类"
		return
	}
	current := 0
	for i, category := range categories {
		if category == m.deploymentCategory {
			current = i
			break
		}
	}
	current = moveIndex(current, len(categories), delta)
	m.deploymentCategory = categories[current]
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
	if m.deploymentCategory == "" {
		m.status = "部署分类：全部"
	} else {
		m.status = "部署分类：" + m.deploymentCategory
	}
}

func (m *Model) moveDeploymentIndex(delta int) {
	if len(m.deploymentItems) == 0 {
		m.deploymentIndex = 0
		return
	}
	for i := 0; i < len(m.deploymentItems); i++ {
		m.deploymentIndex = moveIndex(m.deploymentIndex, len(m.deploymentItems), delta)
		item := m.deploymentItems[m.deploymentIndex]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedDeploymentItem() (deploymentItem, bool) {
	if m.deploymentIndex < 0 || m.deploymentIndex >= len(m.deploymentItems) {
		return deploymentItem{}, false
	}
	item := m.deploymentItems[m.deploymentIndex]
	if item.Header || item.Spacer {
		return deploymentItem{}, false
	}
	return item, true
}

func (m Model) defaultDeploymentServer() string {
	if m.activeDeployment.HostIndex >= 0 && m.activeDeployment.HostIndex < len(m.states) {
		h := m.states[m.activeDeployment.HostIndex].Host
		return config.ServerCommandKey(h.Category, h.Name)
	}
	if len(m.states) > 0 {
		h := m.states[0].Host
		return config.ServerCommandKey(h.Category, h.Name)
	}
	return ""
}

func (m Model) deploymentServerIndex(server string) int {
	server = strings.TrimSpace(server)
	for i, state := range m.states {
		if config.ServerCommandKey(state.Host.Category, state.Host.Name) == server {
			return i
		}
	}
	return -1
}

func (m *Model) cycleDeploymentServer(delta int) {
	if len(m.states) == 0 {
		m.deploymentForm.Server = ""
		return
	}
	index := m.deploymentServerIndex(m.deploymentForm.Server)
	if index < 0 {
		index = 0
	} else {
		index = moveIndex(index, len(m.states), delta)
	}
	h := m.states[index].Host
	m.deploymentForm.Server = config.ServerCommandKey(h.Category, h.Name)
}

func (m Model) startDeploymentEdit(app config.DeploymentApp, editing bool) Model {
	if !editing {
		app.Source = config.DeploySourceGit
		app.FetchMode = config.DeployFetchLocal
		app.Credential = config.DeployCredentialSSH
		app.Branch = "main"
		app.Server = m.defaultDeploymentServer()
	}
	m.deploymentForm = deploymentFormFromApp(app)
	m.deploymentField = 0
	m.deploymentCursor = len([]rune(m.deploymentForm.Name))
	m.deploymentEditing = editing
	m.deploymentEditIndex = -1
	if editing {
		if item, ok := m.selectedDeploymentItem(); ok {
			m.deploymentEditIndex = item.Index
		}
	}
	m.mode = modeDeploymentEdit
	if editing {
		m.status = "编辑部署应用"
	} else {
		m.status = "添加部署应用"
	}
	return m
}

func deploymentFormFromApp(app config.DeploymentApp) deploymentForm {
	return deploymentForm{
		Name:             app.Name,
		Server:           app.Server,
		Source:           emptyChoice(app.Source, config.DeploySourceGit),
		FetchMode:        emptyChoice(app.FetchMode, config.DeployFetchLocal),
		Repo:             app.Repo,
		Branch:           app.Branch,
		Version:          app.Version,
		Asset:            app.Asset,
		Path:             app.Path,
		ReleaseURL:       app.ReleaseURL,
		Credential:       emptyChoice(app.Credential, config.DeployCredentialNone),
		CredentialName:   app.CredentialName,
		WaitSeconds:      strconv.Itoa(maxInt(0, app.WaitSeconds)),
		BeforeCommands:   strings.Join(app.BeforeCommands, "\n"),
		ResourceCommands: deploymentResourceCommandsText(app),
		UpdateCommands:   strings.Join(app.UpdateCommands, "\n"),
		AfterCommands:    strings.Join(app.AfterCommands, "\n"),
		HealthCommands:   strings.Join(app.HealthCommands, "\n"),
		RollbackCommands: strings.Join(app.RollbackCommands, "\n"),
	}
}

func deploymentResourceCommandsText(app config.DeploymentApp) string {
	if len(app.ResourceCommands) > 0 {
		return strings.Join(app.ResourceCommands, "\n")
	}
	return strings.Join(deploymentResourceDefaultCommands(app), "\n")
}

func deploymentAppWithResourceDefaults(app config.DeploymentApp) config.DeploymentApp {
	if len(app.ResourceCommands) == 0 {
		app.ResourceCommands = deploymentResourceDefaultCommands(app)
	}
	return app
}

func (m Model) deploymentAppFromForm() config.DeploymentApp {
	return config.DeploymentApp{
		Name:             strings.TrimSpace(m.deploymentForm.Name),
		Server:           strings.TrimSpace(m.deploymentForm.Server),
		Source:           strings.TrimSpace(m.deploymentForm.Source),
		FetchMode:        strings.TrimSpace(m.deploymentForm.FetchMode),
		Repo:             strings.TrimSpace(m.deploymentForm.Repo),
		Branch:           strings.TrimSpace(m.deploymentForm.Branch),
		Version:          strings.TrimSpace(m.deploymentForm.Version),
		Asset:            strings.TrimSpace(m.deploymentForm.Asset),
		Path:             strings.TrimSpace(m.deploymentForm.Path),
		ReleaseURL:       strings.TrimSpace(m.deploymentForm.ReleaseURL),
		Credential:       strings.TrimSpace(m.deploymentForm.Credential),
		CredentialName:   strings.TrimSpace(m.deploymentForm.CredentialName),
		WaitSeconds:      parseNonNegativeInt(m.deploymentForm.WaitSeconds),
		BeforeCommands:   splitCommandBlock(m.deploymentForm.BeforeCommands),
		ResourceCommands: splitCommandBlock(m.deploymentForm.ResourceCommands),
		UpdateCommands:   splitCommandBlock(m.deploymentForm.UpdateCommands),
		AfterCommands:    splitCommandBlock(m.deploymentForm.AfterCommands),
		HealthCommands:   splitCommandBlock(m.deploymentForm.HealthCommands),
		RollbackCommands: splitCommandBlock(m.deploymentForm.RollbackCommands),
	}
}

func splitCommandBlock(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseNonNegativeInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (m Model) updateDeploymentEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		return m.startDeploymentList(m.activeDeployment.HostIndex), nil
	case "tab", "down":
		m.deploymentField = deploymentNextField(m.deploymentField, 1, m.deploymentForm.Source)
		m.deploymentCursor = m.deploymentValueLen()
	case "shift+tab", "up":
		m.deploymentField = deploymentNextField(m.deploymentField, -1, m.deploymentForm.Source)
		m.deploymentCursor = m.deploymentValueLen()
	case "left":
		if m.deploymentField == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentField == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentField == 2 {
			m.cycleDeploymentServer(-1)
		} else if m.deploymentField == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(-1)
		}
	case "right":
		if m.deploymentField == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentField == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentField == 2 {
			m.cycleDeploymentServer(1)
		} else if m.deploymentField == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(1)
		}
	case "ctrl+j":
		if deploymentFieldIsCommand(m.deploymentField) {
			m.deploymentAppend("\n")
		}
	case "enter":
		app := m.deploymentAppFromForm()
		if strings.TrimSpace(app.Server) == "" {
			m.status = "保存失败：部署服务器不能为空"
			return m, nil
		}
		if err := config.ValidateDeploymentApp(app); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		file := m.deploymentFile
		if m.deploymentEditing && m.deploymentEditIndex >= 0 && m.deploymentEditIndex < len(file.Apps) {
			file.Apps[m.deploymentEditIndex] = app
		} else {
			file.Apps = append(file.Apps, app)
		}
		if err := config.SaveDeployments(m.home, file); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		m.deploymentFile = file
		m = m.startDeploymentList(m.activeDeployment.HostIndex)
		m.status = "部署应用已保存。"
		return m, nil
	case "backspace":
		m.deploymentBackspace()
	default:
		if len(msg.Runes) > 0 && m.deploymentField != 0 && m.deploymentField != 1 && m.deploymentField != 2 && m.deploymentField != 10 {
			m.deploymentAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func deploymentFieldCount() int { return 19 }

func deploymentFieldIsCommand(field int) bool { return field >= 13 }

func deploymentVisibleFields(source string) []int {
	if source == config.DeploySourceRelease {
		return []int{0, 1, 2, 3, 4, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
	}
	return []int{0, 1, 2, 3, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18}
}

func deploymentNextField(current int, delta int, source string) int {
	fields := deploymentVisibleFields(source)
	if len(fields) == 0 {
		return 0
	}
	pos := 0
	for i, field := range fields {
		if field == current {
			pos = i
			break
		}
	}
	return fields[moveIndex(pos, len(fields), delta)]
}

func (m *Model) toggleDeploymentSource() {
	if m.deploymentForm.Source == config.DeploySourceGit {
		m.deploymentForm.Source = config.DeploySourceRelease
	} else {
		m.deploymentForm.Source = config.DeploySourceGit
	}
}

func (m *Model) toggleDeploymentFetchMode() {
	if m.deploymentForm.FetchMode == config.DeployFetchRemote {
		m.deploymentForm.FetchMode = config.DeployFetchLocal
	} else {
		m.deploymentForm.FetchMode = config.DeployFetchRemote
	}
}

func (m *Model) toggleDeploymentCredential() {
	switch m.deploymentForm.Credential {
	case config.DeployCredentialNone:
		m.deploymentForm.Credential = config.DeployCredentialSSH
	case config.DeployCredentialSSH:
		m.deploymentForm.Credential = config.DeployCredentialToken
	default:
		m.deploymentForm.Credential = config.DeployCredentialNone
	}
}

func (m Model) deploymentValue() string {
	switch m.deploymentField {
	case 1:
		return ""
	case 2:
		return m.deploymentForm.Server
	case 3:
		return m.deploymentForm.Name
	case 4:
		return m.deploymentForm.Repo
	case 5:
		return m.deploymentForm.Branch
	case 6:
		return m.deploymentForm.Version
	case 7:
		return m.deploymentForm.Asset
	case 8:
		return m.deploymentForm.Path
	case 9:
		return m.deploymentForm.ReleaseURL
	case 11:
		return m.deploymentForm.CredentialName
	case 12:
		return m.deploymentForm.WaitSeconds
	case 13:
		return m.deploymentForm.BeforeCommands
	case 14:
		return m.deploymentForm.ResourceCommands
	case 15:
		return m.deploymentForm.UpdateCommands
	case 16:
		return m.deploymentForm.AfterCommands
	case 17:
		return m.deploymentForm.HealthCommands
	case 18:
		return m.deploymentForm.RollbackCommands
	default:
		return ""
	}
}

func (m Model) deploymentValueLen() int {
	return len([]rune(m.deploymentValue()))
}

func (m *Model) setDeploymentValue(value string) {
	switch m.deploymentField {
	case 2:
		m.deploymentForm.Server = value
	case 3:
		m.deploymentForm.Name = value
	case 4:
		m.deploymentForm.Repo = value
	case 5:
		m.deploymentForm.Branch = value
	case 6:
		m.deploymentForm.Version = value
	case 7:
		m.deploymentForm.Asset = value
	case 8:
		m.deploymentForm.Path = value
	case 9:
		m.deploymentForm.ReleaseURL = value
	case 11:
		m.deploymentForm.CredentialName = value
	case 12:
		m.deploymentForm.WaitSeconds = value
	case 13:
		m.deploymentForm.BeforeCommands = value
	case 14:
		m.deploymentForm.ResourceCommands = value
	case 15:
		m.deploymentForm.UpdateCommands = value
	case 16:
		m.deploymentForm.AfterCommands = value
	case 17:
		m.deploymentForm.HealthCommands = value
	case 18:
		m.deploymentForm.RollbackCommands = value
	}
}

func (m *Model) deploymentAppend(s string) {
	value := []rune(m.deploymentValue())
	m.deploymentCursor = clampInt(m.deploymentCursor, 0, len(value))
	insert := []rune(s)
	next := append([]rune{}, value[:m.deploymentCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.deploymentCursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentCursor += len(insert)
}

func (m *Model) deploymentBackspace() {
	if m.deploymentField == 0 || m.deploymentField == 1 || m.deploymentField == 2 || m.deploymentField == 10 {
		return
	}
	value := []rune(m.deploymentValue())
	if m.deploymentCursor <= 0 || len(value) == 0 {
		return
	}
	m.deploymentCursor = clampInt(m.deploymentCursor, 0, len(value))
	next := append([]rune{}, value[:m.deploymentCursor-1]...)
	next = append(next, value[m.deploymentCursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentCursor--
}

func (m *Model) moveDeploymentCursor(delta int) {
	m.deploymentCursor = clampInt(m.deploymentCursor+delta, 0, m.deploymentValueLen())
}

func (m Model) updateDeploymentConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = "部署执行中，完成或失败后再返回"
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentConfirmMaxScroll())
	case "enter":
		if m.activeDeployment.Running {
			m.status = "部署执行中"
			return m, nil
		}
		if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.Output != "" {
			m.status = "当前部署已执行，按 r 重试，或按 a 重新部署"
			return m, nil
		}
		queue := m.deploymentConfirmQueue
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		for _, app := range queue {
			if m.deploymentServerIndex(app.Server) < 0 {
				m.status = "部署服务器不存在：" + emptyDash(app.Server)
				return m, nil
			}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueIndex = 0
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	case "r":
		if m.activeDeployment.Running {
			m.status = "部署执行中，不能重试"
			return m, nil
		}
		if len(m.activeDeployment.Queue) == 0 || m.activeDeployment.QueueFailed < 0 || m.activeDeployment.QueueFailed >= len(m.activeDeployment.Queue) {
			m.status = "没有失败项可重试"
			return m, nil
		}
		return m.startQueuedDeployment(m.activeDeployment.QueueFailed)
	case "a":
		if m.activeDeployment.Running {
			m.status = "部署执行中，不能重新部署"
			return m, nil
		}
		queue := m.activeDeployment.Queue
		if len(queue) == 0 {
			queue = m.deploymentConfirmQueue
		}
		if len(queue) == 0 {
			queue = []config.DeploymentApp{m.deploymentConfirm}
		}
		m.activeDeployment.Queue = queue
		m.activeDeployment.QueueFailed = -1
		return m.startQueuedDeployment(0)
	}
	return m, nil
}

func (m Model) startQueuedDeployment(index int) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.activeDeployment.Queue) {
		m.activeDeployment.Running = false
		m.status = "部署队列完成。"
		return m, nil
	}
	app := m.activeDeployment.Queue[index]
	hostIndex := m.deploymentServerIndex(app.Server)
	if hostIndex < 0 {
		m.status = "部署服务器不存在：" + emptyDash(app.Server)
		return m, nil
	}
	m.activeDeployment.HostIndex = hostIndex
	m.activeDeployment.App = app
	m.activeDeployment.Action = config.DeployActionDeploy
	m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
	m.activeDeployment.Output = ""
	m.activeDeployment.ExitCode = 0
	m.activeDeployment.Running = true
	m.activeDeployment.PreviousVersion = ""
	m.activeDeployment.CurrentVersion = ""
	m.activeDeployment.QueueIndex = index
	m.activeDeployment.QueueFailed = -1
	m.deploymentOutputScroll = 0
	m.mode = modeDeploymentConfirm
	if len(m.activeDeployment.Queue) > 1 {
		m.status = fmt.Sprintf("正在部署 %d/%d：%s", index+1, len(m.activeDeployment.Queue), app.Name)
	} else {
		m.status = "正在部署..."
	}
	deploymentProgressStart(m.activeDeployment.ProgressID)
	return m, tea.Batch(m.runDeployment(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
}

func (m Model) startNextQueuedDeployment() (tea.Model, tea.Cmd) {
	next := m.activeDeployment.QueueIndex + 1
	return m.startQueuedDeployment(next)
}

func deploymentQueueNextAfter(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay)
		}
		return deploymentQueueNextMsg{}
	}
}

func (m Model) updateDeploymentOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		if m.activeDeployment.Running {
			m.status = "部署执行中，完成后再返回"
			return m, nil
		}
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentOutputMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentOutputMaxScroll())
	case "r":
		if m.activeDeployment.Running {
			m.status = "部署执行中，完成后再回滚"
			return m, nil
		}
		if len(m.activeDeployment.App.RollbackCommands) == 0 {
			m.status = "没有配置回滚命令"
			return m, nil
		}
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentRollbackConfirm
		m.status = "确认回滚"
		return m, nil
	}
	return m, nil
}

func (m Model) updateDeploymentRollbackConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDeploymentOutput
	case "j", "down":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll+1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "k", "up":
		m.deploymentOutputScroll = clampInt(m.deploymentOutputScroll-1, 0, m.deploymentRollbackConfirmMaxScroll())
	case "enter":
		m.activeDeployment.Running = true
		m.activeDeployment.Action = config.DeployActionRollback
		m.activeDeployment.ProgressID = config.NewDeploymentID(time.Now())
		m.activeDeployment.Output = ""
		m.activeDeployment.ExitCode = 0
		m.deploymentOutputScroll = 0
		m.mode = modeDeploymentOutput
		m.status = "正在执行回滚..."
		deploymentProgressStart(m.activeDeployment.ProgressID)
		return m, tea.Batch(m.runDeploymentRollback(), deploymentProgressAfter(m.activeDeployment.ProgressID, 200*time.Millisecond))
	}
	return m, nil
}

func (m Model) runDeployment() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := actions.CommandResult{Err: fmt.Errorf("部署服务器不存在：%s", emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	onOutput := func(text string) { deploymentProgressAppend(progressID, text) }
	if app.FetchMode == config.DeployFetchLocal {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			result := runLocalFetchDeployment(ctx, h, app, onOutput)
			prev, curr := parseDeploymentVersions(result.Output)
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result, PreviousVersion: prev, CurrentVersion: curr}
		}
	}
	script := buildDeploymentScript(app, false)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, cleanup := actions.RemoteCommandStreamContext(ctx, h, script, onOutput)
		cleanup()
		prev, curr := parseDeploymentVersions(result.Output)
		deploymentProgressFinish(progressID, result.Output)
		return deploymentDoneMsg{ID: progressID, Result: result, PreviousVersion: prev, CurrentVersion: curr}
	}
}

func (m Model) runDeploymentRollback() tea.Cmd {
	index := m.activeDeployment.HostIndex
	app := m.activeDeployment.App
	progressID := m.activeDeployment.ProgressID
	if index < 0 || index >= len(m.states) {
		return func() tea.Msg {
			result := actions.CommandResult{Err: fmt.Errorf("部署服务器不存在：%s", emptyDash(app.Server)), ExitCode: -1}
			deploymentProgressFinish(progressID, result.Output)
			return deploymentDoneMsg{ID: progressID, Result: result}
		}
	}
	h := m.states[index].Host
	script := buildDeploymentScript(app, true)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		result, cleanup := actions.RemoteCommandStreamContext(ctx, h, script, func(text string) { deploymentProgressAppend(progressID, text) })
		cleanup()
		deploymentProgressFinish(progressID, result.Output)
		return deploymentDoneMsg{ID: progressID, Result: result}
	}
}

func (m Model) handleDeploymentDone(msg deploymentDoneMsg) (tea.Model, tea.Cmd) {
	if msg.ID != "" && msg.ID != m.activeDeployment.ProgressID {
		return m, nil
	}
	m.activeDeployment.Running = false
	m.activeDeployment.Output = msg.Result.Output
	m.activeDeployment.ExitCode = msg.Result.ExitCode
	m.activeDeployment.PreviousVersion = msg.PreviousVersion
	m.activeDeployment.CurrentVersion = msg.CurrentVersion
	failed := msg.Result.Err != nil
	if failed {
		m.status = fmt.Sprintf("部署失败：退出码 %d", msg.Result.ExitCode)
	} else {
		m.status = "部署完成。"
	}
	if err := m.recordDeployment(msg.Result); err != nil {
		m.status += " 记录保存失败：" + err.Error()
	}
	if msg.ID != "" {
		deploymentProgressClear(msg.ID)
	}
	if m.activeDeployment.Action == config.DeployActionDeploy && len(m.activeDeployment.Queue) > 0 {
		if failed {
			m.activeDeployment.QueueFailed = m.activeDeployment.QueueIndex
			if len(m.activeDeployment.Queue) > 1 {
				m.status = fmt.Sprintf("部署队列停止：第 %d 个应用失败，按 r 重试失败项，按 a 重新部署", m.activeDeployment.QueueIndex+1)
			} else {
				m.status = "部署失败，按 r 重试，按 a 重新部署"
			}
			return m, nil
		}
		next := m.activeDeployment.QueueIndex + 1
		if next < len(m.activeDeployment.Queue) {
			wait := maxInt(0, m.activeDeployment.App.WaitSeconds)
			m.status = fmt.Sprintf("部署完成，等待 %d 秒后执行下一个：%s", wait, m.activeDeployment.Queue[next].Name)
			return m, deploymentQueueNextAfter(time.Duration(wait) * time.Second)
		}
		if len(m.activeDeployment.Queue) > 1 {
			m.status = "部署队列完成。"
		}
	}
	return m, nil
}

func (m Model) handleDeploymentProgress(msg deploymentProgressMsg) (tea.Model, tea.Cmd) {
	if msg.ID == "" || msg.ID != m.activeDeployment.ProgressID || !m.activeDeployment.Running {
		return m, nil
	}
	m.activeDeployment.Output = msg.Output
	if msg.Done {
		return m, nil
	}
	return m, deploymentProgressAfter(msg.ID, 300*time.Millisecond)
}

func (m *Model) recordDeployment(result actions.CommandResult) error {
	if m.activeDeployment.HostIndex < 0 || m.activeDeployment.HostIndex >= len(m.states) {
		return nil
	}
	h := m.states[m.activeDeployment.HostIndex].Host
	status := config.DeployStatusSuccess
	errText := ""
	if result.Err != nil {
		status = config.DeployStatusFailed
		errText = result.Err.Error()
	}
	record := config.DeploymentRecord{
		ID:              config.NewDeploymentID(time.Now()),
		Time:            time.Now().Format(time.RFC3339),
		App:             m.activeDeployment.App.Name,
		ServerCategory:  h.Category,
		ServerName:      h.Name,
		Action:          emptyChoice(m.activeDeployment.Action, config.DeployActionDeploy),
		Source:          m.activeDeployment.App.Source,
		Status:          status,
		PreviousVersion: m.activeDeployment.PreviousVersion,
		CurrentVersion:  m.activeDeployment.CurrentVersion,
		ExitCode:        result.ExitCode,
		Output:          result.Output,
		Error:           errText,
	}
	if err := config.AppendDeploymentRecord(m.home, record); err != nil {
		return err
	}
	file, _, err := config.LoadDeployments(m.home)
	if err == nil {
		m.deploymentFile = file
	}
	return nil
}

func buildDeploymentScript(app config.DeploymentApp, rollback bool) string {
	return buildRemoteDeploymentScript(app, rollback, true)
}

func buildRemoteDeploymentScript(app config.DeploymentApp, rollback bool, includeResource bool) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentCredentialScript(&b, app)
	b.WriteString("mkdir -p " + shellSingleQuote(app.Path) + "\n")
	if rollback {
		appendDeploymentCommands(&b, app.Path, "回滚", app.RollbackCommands)
		return b.String()
	}
	appendDeploymentCommands(&b, app.Path, "更新前", app.BeforeCommands)
	if includeResource && len(app.ResourceCommands) > 0 {
		appendDeploymentCommands(&b, app.Path, "获取资源", app.ResourceCommands)
	} else if includeResource {
		appendDeploymentStageTitle(&b, "获取资源")
		switch app.Source {
		case config.DeploySourceRelease:
			appendReleaseDeploymentScript(&b, app)
		default:
			appendGitDeploymentScript(&b, app)
		}
	}
	appendDeploymentCommands(&b, app.Path, "更新", app.UpdateCommands)
	appendDeploymentCommands(&b, app.Path, "更新后", app.AfterCommands)
	appendDeploymentCommands(&b, app.Path, "健康检查", app.HealthCommands)
	return b.String()
}

func runLocalFetchDeployment(ctx context.Context, h host.Host, app config.DeploymentApp, onOutput func(string)) actions.CommandResult {
	var output strings.Builder
	pre := buildLocalFetchPreScript(app)
	preResult, cleanup := actions.RemoteCommandStreamContext(ctx, h, pre, onOutput)
	cleanup()
	output.WriteString(preResult.Output)
	if preResult.Err != nil {
		preResult.Output = output.String()
		return preResult
	}
	tmp, err := os.MkdirTemp("", "sshm-deploy-*")
	if err != nil {
		return actions.CommandResult{Output: output.String(), Err: err, ExitCode: -1}
	}
	defer os.RemoveAll(tmp)
	localResult := localFetchDeploymentResource(ctx, app, tmp, onOutput)
	output.WriteString(localResult.Output)
	if localResult.Err != nil {
		localResult.Output = output.String()
		return localResult
	}
	cmd, rsyncCleanup := actions.RsyncUploadCommandContext(ctx, h, localResultPath(tmp)+string(os.PathSeparator), app.Path)
	uploadTitle := "== 上传资源 ==\n"
	output.WriteString(uploadTitle)
	if onOutput != nil {
		onOutput(uploadTitle)
	}
	rsyncResult := actions.RunCommandStream(cmd, onOutput)
	rsyncCleanup()
	output.WriteString(rsyncResult.Output)
	if rsyncResult.Err != nil {
		return actions.CommandResult{Output: output.String(), Err: rsyncResult.Err, ExitCode: rsyncResult.ExitCode}
	}
	post := buildLocalFetchPostScript(app)
	postResult, postCleanup := actions.RemoteCommandStreamContext(ctx, h, post, onOutput)
	postCleanup()
	output.WriteString(postResult.Output)
	postResult.Output = output.String()
	return postResult
}

func buildLocalFetchPreScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	b.WriteString("mkdir -p " + shellSingleQuote(app.Path) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新前", app.BeforeCommands)
	if app.Source == config.DeploySourceGit {
		b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	} else {
		b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
		b.WriteString("SSHM_PREVIOUS_VERSION=$(readlink current 2>/dev/null || true)\n")
		b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	}
	return b.String()
}

func buildLocalFetchPostScript(app config.DeploymentApp) string {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(app.Path) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentCommands(&b, app.Path, "更新", app.UpdateCommands)
	appendDeploymentCommands(&b, app.Path, "更新后", app.AfterCommands)
	appendDeploymentCommands(&b, app.Path, "健康检查", app.HealthCommands)
	return b.String()
}

func localResultPath(tmp string) string {
	return filepath.Join(tmp, "payload")
}

func localFetchDeploymentResource(ctx context.Context, app config.DeploymentApp, tmp string, onOutput func(string)) actions.CommandResult {
	payload := localResultPath(tmp)
	if err := os.MkdirAll(payload, 0700); err != nil {
		return actions.CommandResult{Err: err, ExitCode: -1}
	}
	if len(app.ResourceCommands) > 0 {
		return localFetchCustomResource(ctx, app, payload, onOutput)
	}
	if app.Source == config.DeploySourceRelease {
		return localFetchReleaseResource(ctx, app, payload, onOutput)
	}
	return localFetchGitResource(ctx, app, payload, onOutput)
}

func localFetchCustomResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("export SSHM_DEPLOY_APP=" + shellSingleQuote(app.Name) + "\n")
	b.WriteString("export SSHM_DEPLOY_PATH=" + shellSingleQuote(payload) + "\n")
	b.WriteString("export SSHM_DEPLOY_SOURCE=" + shellSingleQuote(app.Source) + "\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + shellSingleQuote(payload) + "\n")
	for _, command := range app.ResourceCommands {
		if strings.TrimSpace(command) != "" {
			b.WriteString(command + "\n")
		}
	}
	cmd := localShellCommand(ctx, b.String())
	cmd.Env = deploymentLocalEnv(app)
	return actions.RunCommandStream(cmd, onOutput)
}

func localFetchGitResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	args := []string{"clone", "--branch", branch, app.Repo, payload}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = deploymentLocalEnv(app)
	var output strings.Builder
	stage := "== 获取资源 ==\n"
	output.WriteString(stage)
	if onOutput != nil {
		onOutput(stage)
	}
	result := actions.RunCommandStream(cmd, onOutput)
	output.WriteString(result.Output)
	result.Output = output.String()
	if result.Err != nil {
		return result
	}
	versionCmd := exec.CommandContext(ctx, "git", "-C", payload, "rev-parse", "--short", "HEAD")
	versionOutput, versionErr := versionCmd.CombinedOutput()
	if versionErr == nil {
		result.Output += "SSHM_CURRENT_VERSION=" + strings.TrimSpace(string(versionOutput)) + "\n"
	}
	return result
}

func localFetchReleaseResource(ctx context.Context, app config.DeploymentApp, payload string, onOutput func(string)) actions.CommandResult {
	script := buildLocalReleaseScript(app, payload)
	cmd := localShellCommand(ctx, script)
	cmd.Env = deploymentLocalEnv(app)
	return actions.RunCommandStream(cmd, onOutput)
}

func buildLocalReleaseScript(app config.DeploymentApp, payload string) string {
	var b strings.Builder
	url, version, asset := deploymentReleaseValues(app)
	b.WriteString("set -eu\n")
	appendDeploymentStageTitle(&b, "获取资源")
	b.WriteString("cd " + shellSingleQuote(payload) + "\n")
	b.WriteString("mkdir -p packages " + shellSingleQuote("releases/"+version) + "\n")
	if deploymentAssetIsPattern(asset) && strings.TrimSpace(app.ReleaseURL) == "" {
		apiURL := deploymentReleaseAPIURL(app.Repo, version)
		b.WriteString("SSHM_RELEASE_JSON=$(curl -fsL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + shellSingleQuote(apiURL) + ")\n")
		b.WriteString("SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in " + shellCasePattern(asset) + ") printf '%s\\n' \"$url\"; break ;; esac; done)\n")
		b.WriteString("if [ -z \"$SSHM_RELEASE_URL\" ]; then echo " + shellSingleQuote("未找到匹配的 Release 资源："+asset) + "; exit 1; fi\n")
		b.WriteString("SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}\n")
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("case \"$SSHM_RELEASE_ASSET\" in\n")
		b.WriteString("  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C " + shellSingleQuote("releases/"+version) + " ;;\n")
		b.WriteString("  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d " + shellSingleQuote("releases/"+version) + " ;;\n")
		b.WriteString("  *) cp \"$SSHM_RELEASE_PACKAGE\" " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
		b.WriteString("esac\n")
	} else {
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
		appendReleaseUnpackShell(&b, asset, version)
	}
	b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
	return b.String()
}

func appendReleaseUnpackShell(b *strings.Builder, asset string, version string) {
	b.WriteString("case " + shellSingleQuote(asset) + " in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf " + shellSingleQuote("packages/"+asset) + " -C " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *.zip) unzip -o " + shellSingleQuote("packages/"+asset) + " -d " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *) cp " + shellSingleQuote("packages/"+asset) + " " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
	b.WriteString("esac\n")
}

func localShellCommand(ctx context.Context, script string) *exec.Cmd {
	name := "sh"
	args := []string{"-s"}
	if runtime.GOOS == "windows" {
		name = "cmd"
		args = []string{"/C", script}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if runtime.GOOS != "windows" {
		cmd.Stdin = strings.NewReader(script)
	}
	return cmd
}

func deploymentLocalEnv(app config.DeploymentApp) []string {
	env := os.Environ()
	name := strings.TrimSpace(app.CredentialName)
	switch app.Credential {
	case config.DeployCredentialSSH:
		if name != "" {
			env = append(env, "GIT_SSH_COMMAND=ssh -i "+name+" -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new")
		}
	case config.DeployCredentialToken:
		tokenVar := shellEnvName(name)
		if tokenVar == "" {
			tokenVar = "GITHUB_TOKEN"
		}
		token := os.Getenv(tokenVar)
		if token != "" {
			env = append(env, "SSHM_GITHUB_AUTH_HEADER=Authorization: Bearer "+token)
		}
	}
	return env
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func appendGitDeploymentScript(b *strings.Builder, app config.DeploymentApp) {
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	parent := filepath.Dir(strings.TrimRight(app.Path, "/"))
	b.WriteString("echo '== 获取 Git 代码 =='\n")
	b.WriteString("if [ ! -d " + shellSingleQuote(app.Path+"/.git") + " ]; then\n")
	b.WriteString("  mkdir -p " + shellSingleQuote(parent) + "\n")
	b.WriteString("  git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("fi\n")
	b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	b.WriteString("git fetch --all --prune\n")
	b.WriteString("git checkout " + shellSingleQuote(branch) + "\n")
	b.WriteString("git pull --ff-only\n")
	b.WriteString("SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
}

func appendReleaseDeploymentScript(b *strings.Builder, app config.DeploymentApp) {
	url, version, asset := deploymentReleaseValues(app)
	assetIsPattern := deploymentAssetIsPattern(asset)
	b.WriteString("echo '== 获取 Release 资源 =='\n")
	b.WriteString("cd " + shellSingleQuote(app.Path) + "\n")
	b.WriteString("SSHM_PREVIOUS_VERSION=$(readlink current 2>/dev/null || true)\n")
	b.WriteString("echo SSHM_PREVIOUS_VERSION=$SSHM_PREVIOUS_VERSION\n")
	b.WriteString("mkdir -p packages " + shellSingleQuote("releases/"+version) + "\n")
	if assetIsPattern && strings.TrimSpace(app.ReleaseURL) == "" {
		apiURL := deploymentReleaseAPIURL(app.Repo, version)
		b.WriteString("SSHM_RELEASE_API=" + shellSingleQuote(apiURL) + "\n")
		b.WriteString("if [ -n \"${SSHM_GITHUB_AUTH_HEADER:-}\" ]; then\n")
		b.WriteString("  SSHM_RELEASE_JSON=$(curl -fsL -H \"$SSHM_GITHUB_AUTH_HEADER\" \"$SSHM_RELEASE_API\")\n")
		b.WriteString("else\n")
		b.WriteString("  SSHM_RELEASE_JSON=$(curl -fsL \"$SSHM_RELEASE_API\")\n")
		b.WriteString("fi\n")
		b.WriteString("SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in " + shellCasePattern(asset) + ") printf '%s\\n' \"$url\"; break ;; esac; done)\n")
		b.WriteString("if [ -z \"$SSHM_RELEASE_URL\" ]; then echo " + shellSingleQuote("未找到匹配的 Release 资源："+asset) + "; exit 1; fi\n")
		b.WriteString("SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}\n")
		b.WriteString("curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"\n")
		b.WriteString("SSHM_RELEASE_TARGET=" + shellSingleQuote("releases/"+version) + "\n")
		b.WriteString("case \"$SSHM_RELEASE_ASSET\" in\n")
		b.WriteString("  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C \"$SSHM_RELEASE_TARGET\" ;;\n")
		b.WriteString("  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d \"$SSHM_RELEASE_TARGET\" ;;\n")
		b.WriteString("  *) cp \"$SSHM_RELEASE_PACKAGE\" \"$SSHM_RELEASE_TARGET/\" ;;\n")
		b.WriteString("esac\n")
		b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
		b.WriteString("SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
		b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
		return
	}
	b.WriteString("if [ -n \"${SSHM_GITHUB_AUTH_HEADER:-}\" ]; then\n")
	b.WriteString("  curl -fL -H \"$SSHM_GITHUB_AUTH_HEADER\" " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
	b.WriteString("else\n")
	b.WriteString("  curl -fL " + shellSingleQuote(url) + " -o " + shellSingleQuote("packages/"+asset) + "\n")
	b.WriteString("fi\n")
	b.WriteString("case " + shellSingleQuote(asset) + " in\n")
	b.WriteString("  *.tar.gz|*.tgz) tar -xzf " + shellSingleQuote("packages/"+asset) + " -C " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *.zip) unzip -o " + shellSingleQuote("packages/"+asset) + " -d " + shellSingleQuote("releases/"+version) + " ;;\n")
	b.WriteString("  *) cp " + shellSingleQuote("packages/"+asset) + " " + shellSingleQuote("releases/"+version+"/") + " ;;\n")
	b.WriteString("esac\n")
	b.WriteString("ln -sfn " + shellSingleQuote("releases/"+version) + " current\n")
	b.WriteString("SSHM_CURRENT_VERSION=" + shellSingleQuote(version) + "\n")
	b.WriteString("echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION\n")
}

func deploymentReleaseValues(app config.DeploymentApp) (string, string, string) {
	url := strings.TrimSpace(app.ReleaseURL)
	version := strings.TrimSpace(app.Version)
	if version == "" {
		version = "latest"
	}
	asset := strings.TrimSpace(app.Asset)
	if asset == "" {
		asset = filepath.Base(url)
	}
	if url == "" {
		repo := strings.Trim(strings.TrimSpace(app.Repo), "/")
		if version == "latest" {
			url = "https://github.com/" + repo + "/releases/latest/download/" + asset
		} else {
			url = "https://github.com/" + repo + "/releases/download/" + version + "/" + asset
		}
	}
	return url, version, asset
}

func deploymentReleaseAPIURL(repo string, version string) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	if strings.TrimSpace(version) == "" || version == "latest" {
		return "https://api.github.com/repos/" + repo + "/releases/latest"
	}
	return "https://api.github.com/repos/" + repo + "/releases/tags/" + version
}

func deploymentAssetIsPattern(asset string) bool {
	return strings.Contains(asset, "*")
}

func shellCasePattern(value string) string {
	if value == "" {
		return "''"
	}
	var b strings.Builder
	var literal strings.Builder
	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		b.WriteString(shellSingleQuote(literal.String()))
		literal.Reset()
	}
	for _, r := range value {
		if r == '*' {
			flushLiteral()
			b.WriteRune('*')
			continue
		}
		literal.WriteRune(r)
	}
	flushLiteral()
	if b.Len() == 0 {
		return "''"
	}
	return b.String()
}

func deploymentResourcePreviewCommands(app config.DeploymentApp) []string {
	return deploymentResourceDefaultCommands(app)
}

func deploymentResourceDefaultCommands(app config.DeploymentApp) []string {
	localFetch := app.FetchMode == config.DeployFetchLocal
	if app.Source == config.DeploySourceRelease {
		url, version, asset := deploymentReleaseValues(app)
		commands := []string{}
		if !localFetch {
			commands = append(commands, "cd "+shellSingleQuote(app.Path))
		}
		commands = append(commands, "mkdir -p packages "+shellSingleQuote("releases/"+version))
		if deploymentAssetIsPattern(asset) && strings.TrimSpace(app.ReleaseURL) == "" {
			commands = append(commands,
				"SSHM_RELEASE_JSON=$(curl -fsL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} "+shellSingleQuote(deploymentReleaseAPIURL(app.Repo, version))+")",
				"SSHM_RELEASE_URL=$(printf '%s\\n' \"$SSHM_RELEASE_JSON\" | awk -F '\"' '/\"browser_download_url\":/ {print $4}' | while IFS= read -r url; do name=${url##*/}; case \"$name\" in "+shellCasePattern(asset)+") printf '%s\\n' \"$url\"; break ;; esac; done)",
				"if [ -z \"$SSHM_RELEASE_URL\" ]; then echo "+shellSingleQuote("未找到匹配的 Release 资源："+asset)+"; exit 1; fi",
				"SSHM_RELEASE_ASSET=${SSHM_RELEASE_URL##*/}",
				"curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} \"$SSHM_RELEASE_URL\" -o \"packages/$SSHM_RELEASE_ASSET\"",
				"SSHM_RELEASE_PACKAGE=\"packages/$SSHM_RELEASE_ASSET\"",
			)
			commands = appendReleaseDynamicUnpackPreview(commands, version)
			return append(commands, "ln -sfn "+shellSingleQuote("releases/"+version)+" current")
		}
		commands = append(commands, "curl -fL ${SSHM_GITHUB_AUTH_HEADER:+-H \"$SSHM_GITHUB_AUTH_HEADER\"} "+shellSingleQuote(url)+" -o "+shellSingleQuote("packages/"+asset))
		commands = appendReleaseUnpackPreview(commands, asset, version)
		commands = append(commands, "ln -sfn "+shellSingleQuote("releases/"+version)+" current")
		return commands
	}
	branch := strings.TrimSpace(app.Branch)
	if branch == "" {
		branch = "main"
	}
	if localFetch {
		return []string{
			"git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " .",
			"SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
			"echo SSHM_CURRENT_VERSION=$SSHM_CURRENT_VERSION",
		}
	}
	parent := filepath.Dir(strings.TrimRight(app.Path, "/"))
	return []string{
		"if [ ! -d " + shellSingleQuote(app.Path+"/.git") + " ]; then mkdir -p " + shellSingleQuote(parent) + " && git clone --branch " + shellSingleQuote(branch) + " " + shellSingleQuote(app.Repo) + " " + shellSingleQuote(app.Path) + "; fi",
		"cd " + shellSingleQuote(app.Path),
		"SSHM_PREVIOUS_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
		"git fetch --all --prune",
		"git checkout " + shellSingleQuote(branch),
		"git pull --ff-only",
		"SSHM_CURRENT_VERSION=$(git rev-parse --short HEAD 2>/dev/null || true)",
	}
}

func appendReleaseDynamicUnpackPreview(commands []string, version string) []string {
	return append(commands,
		"case \"$SSHM_RELEASE_ASSET\" in",
		"  *.tar.gz|*.tgz) tar -xzf \"$SSHM_RELEASE_PACKAGE\" -C "+shellSingleQuote("releases/"+version)+" ;;",
		"  *.zip) unzip -o \"$SSHM_RELEASE_PACKAGE\" -d "+shellSingleQuote("releases/"+version)+" ;;",
		"  *) cp \"$SSHM_RELEASE_PACKAGE\" "+shellSingleQuote("releases/"+version+"/")+" ;;",
		"esac",
	)
}

func appendReleaseUnpackPreview(commands []string, asset string, version string) []string {
	switch {
	case strings.HasSuffix(asset, ".tar.gz") || strings.HasSuffix(asset, ".tgz"):
		return append(commands, "tar -xzf "+shellSingleQuote("packages/"+asset)+" -C "+shellSingleQuote("releases/"+version))
	case strings.HasSuffix(asset, ".zip"):
		return append(commands, "unzip -o "+shellSingleQuote("packages/"+asset)+" -d "+shellSingleQuote("releases/"+version))
	default:
		return append(commands, "cp "+shellSingleQuote("packages/"+asset)+" "+shellSingleQuote("releases/"+version+"/"))
	}
}

func appendDeploymentCredentialScript(b *strings.Builder, app config.DeploymentApp) {
	name := strings.TrimSpace(app.CredentialName)
	switch app.Credential {
	case config.DeployCredentialSSH:
		if name == "" {
			return
		}
		gitSSHCommand := "ssh -i " + shellSingleQuote(name) + " -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new"
		b.WriteString("export GIT_SSH_COMMAND=" + shellSingleQuote(gitSSHCommand) + "\n")
	case config.DeployCredentialToken:
		tokenVar := shellEnvName(name)
		if tokenVar == "" {
			tokenVar = "GITHUB_TOKEN"
		}
		b.WriteString("SSHM_GITHUB_AUTH_HEADER=\n")
		b.WriteString("if [ -n \"${" + tokenVar + ":-}\" ]; then\n")
		b.WriteString("  SSHM_GITHUB_AUTH_HEADER=\"Authorization: Bearer ${" + tokenVar + "}\"\n")
		b.WriteString("fi\n")
	}
}

func appendDeploymentCommands(b *strings.Builder, path string, title string, commands []string) {
	if len(commands) == 0 {
		return
	}
	appendDeploymentStageTitle(b, title)
	b.WriteString("cd " + shellSingleQuote(path) + "\n")
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			b.WriteString(command + "\n")
		}
	}
}

func appendDeploymentStageTitle(b *strings.Builder, title string) {
	b.WriteString("echo " + shellSingleQuote("== "+title+" ==") + "\n")
}

func deploymentStageOutput(title string, output string) string {
	if strings.TrimSpace(output) == "" {
		return "== " + title + " ==\n"
	}
	return "== " + title + " ==\n" + output
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellEnvName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for i, r := range value {
		if i == 0 {
			if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return ""
			}
			continue
		}
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return ""
		}
	}
	return value
}

func parseDeploymentVersions(output string) (string, string) {
	prev := ""
	curr := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SSHM_PREVIOUS_VERSION=") {
			prev = strings.TrimPrefix(line, "SSHM_PREVIOUS_VERSION=")
		}
		if strings.HasPrefix(line, "SSHM_CURRENT_VERSION=") {
			curr = strings.TrimPrefix(line, "SSHM_CURRENT_VERSION=")
		}
	}
	return prev, curr
}

func (m Model) batchSuccessCount() int {
	count := 0
	for _, job := range m.batchJobs {
		if job.Done && job.Err == nil {
			count++
		}
	}
	return count
}

func (m Model) batchFailCount() int {
	count := 0
	for _, job := range m.batchJobs {
		if job.Done && job.Err != nil {
			count++
		}
	}
	return count
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

func (m Model) updateTransferPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panel.NeedsInstall {
		key := shortcutKey(msg)
		switch key {
		case "i":
			m.status = "正在远程安装 rsync..."
			return m, m.installRemoteRsync(m.panel.HostIndex)
		case "esc", "q":
			m.panel.NeedsInstall = false
			m.status = "已取消。"
			return m, nil
		}
		return m, nil
	}
	if m.panel.Confirming && msg.String() != "enter" {
		m.panel.Confirming = false
		m.status = transferPanelStatus(m.panel.Mode)
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeDashboard
		m.transfer = transferNone
		m.panel = transferPanel{}
		m.status = "已取消。"
	case "tab":
		m.cancelTransferConfirm()
		if m.panel.ActivePane == 0 {
			m.panel.ActivePane = 1
		} else {
			m.panel.ActivePane = 0
		}
	case "j", "down":
		m.cancelTransferConfirm()
		m.movePanel(1)
	case "k", "up":
		m.cancelTransferConfirm()
		m.movePanel(-1)
	case "enter":
		if m.panel.Confirming {
			return m.confirmTransferPanel()
		}
		m.cancelTransferConfirm()
		m.togglePanelTree()
	case " ":
		m.cancelTransferConfirm()
		m.togglePanelSelection()
	case "s":
		m.cancelTransferConfirm()
		return m.confirmTransferPanel()
	}
	return m, nil
}

func (m Model) updateTransferJobs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panel.NeedsInstall {
		key := shortcutKey(msg)
		switch key {
		case "i":
			m.status = "正在远程安装 rsync..."
			return m, m.installRemoteRsync(m.panel.HostIndex)
		case "esc", "q":
			m.panel.NeedsInstall = false
			m.status = "已取消安装 rsync。"
			return m, nil
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = m.transferJobsBack
		if m.mode == 0 {
			m.mode = modeDashboard
		}
	case "j", "down":
		m.moveTransferIndex(m.dashboardColumns())
	case "k", "up":
		m.moveTransferIndex(-m.dashboardColumns())
	case "h", "left":
		m.moveTransferIndex(-1)
	case "l", "right":
		m.moveTransferIndex(1)
	case "tab":
		m.cycleTransferStatusFilter()
	case "enter":
		m.transferRunAll = false
		return m.startSelectedTransfer()
	case " ":
		return m.openTransferDetail(), nil
	case "a":
		return m.startAllQueuedTransfers()
	case "p":
		return m.pauseRunningTransfers()
	case "c":
		return m.cancelSelectedTransfer()
	case "x":
		return m.deleteSelectedTransfer()
	}
	return m, nil
}

func (m Model) openTransferDetail() Model {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m
	}
	m.mode = modeTransferDetail
	m.detailScroll = 0
	return m
}

func (m Model) updateTransferDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c", "b":
		m.mode = modeTransferJobs
		m.detailScroll = 0
	case "j", "down":
		m.detailScroll = clampInt(m.detailScroll+1, 0, m.transferDetailMaxScroll())
	case "k", "up":
		m.detailScroll = clampInt(m.detailScroll-1, 0, m.transferDetailMaxScroll())
	case "enter":
		m.transferRunAll = false
		return m.startSelectedTransfer()
	case "a":
		return m.startAllQueuedTransfers()
	case "p":
		return m.pauseRunningTransfers()
	case "c":
		return m.cancelSelectedTransfer()
	case "x":
		return m.deleteSelectedTransfer()
	}
	return m, nil
}

func (m *Model) moveTransferIndex(delta int) {
	indexes := m.filteredTransferIndexes()
	if len(indexes) == 0 {
		m.transferIndex = 0
		return
	}
	pos := 0
	for i, index := range indexes {
		if index == m.transferIndex {
			pos = i
			break
		}
	}
	pos = clampInt(pos+delta, 0, len(indexes)-1)
	m.transferIndex = indexes[pos]
}

func (m *Model) cycleTransferStatusFilter() {
	m.transferStatusFilter++
	if m.transferStatusFilter >= len(transferStatusFilterOptions()) {
		m.transferStatusFilter = 0
	}
	m.ensureTransferIndexVisible()
}

func (m Model) startSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	switch entry.Status {
	case config.TransferStatusQueued:
		return m.startTransferEntry(entry)
	case config.TransferStatusFailed, config.TransferStatusInterrupted:
		entry.Status = config.TransferStatusQueued
		entry.Error = ""
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = config.UpdateTransfer(m.home, entry)
		m.reloadTransfers()
		return m.startTransferEntry(entry)
	default:
		m.status = "该任务当前不可开始。"
		return m, nil
	}
}

func (m Model) startAllQueuedTransfers() (tea.Model, tea.Cmd) {
	file := m.transferHistory
	count := 0
	now := time.Now().Format(time.RFC3339)
	for i := range file.Entries {
		if file.Entries[i].Status == config.TransferStatusQueued || file.Entries[i].Status == config.TransferStatusInterrupted {
			file.Entries[i].Status = config.TransferStatusPending
			file.Entries[i].Error = ""
			file.Entries[i].UpdatedAt = now
			count++
		}
	}
	if count == 0 {
		m.status = "没有等待中或中断的任务。"
		return m, nil
	}
	_ = config.SaveTransfers(m.home, file)
	m.transferStatusFilter = 0
	m.reloadTransfers()
	m.transferRunAll = true
	if m.activeTransfer.Active {
		m.status = fmt.Sprintf("已加入全部开始：排队中 %d 个。", count)
		return m, nil
	}
	return m.startNextQueuedTransfer()
}

func (m Model) transferEntryStatus(id string) (string, bool) {
	for _, entry := range m.transferHistory.Entries {
		if entry.ID == id {
			return entry.Status, true
		}
	}
	return "", false
}

func (m Model) pauseRunningTransfers() (tea.Model, tea.Cmd) {
	file := m.transferHistory
	changed := false
	now := time.Now().Format(time.RFC3339)
	for i := range file.Entries {
		switch file.Entries[i].Status {
		case config.TransferStatusRunning:
			file.Entries[i].Status = config.TransferStatusInterrupted
			file.Entries[i].UpdatedAt = now
			changed = true
		case config.TransferStatusPending:
			file.Entries[i].Status = config.TransferStatusQueued
			file.Entries[i].UpdatedAt = now
			changed = true
		}
	}
	if !changed {
		m.status = "没有运行中或排队中的任务。"
		return m, nil
	}
	m.transferRunAll = false
	_ = config.SaveTransfers(m.home, file)
	m.reloadTransfers()
	if m.activeTransfer.Active && m.activeTransfer.Cancel != nil {
		m.activeTransfer.Cancel()
	}
	m.status = "已暂停运行中任务，排队中任务已退回等待中。"
	return m, nil
}

func (m Model) deleteSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	if entry.Status == config.TransferStatusRunning {
		m.status = "运行中的任务不能删除。"
		return m, nil
	}
	_ = config.DeleteTransfer(m.home, entry.ID)
	m.reloadTransfers()
	return m, nil
}

func (m Model) cancelSelectedTransfer() (tea.Model, tea.Cmd) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return m, nil
	}
	entry := m.transferHistory.Entries[m.transferIndex]
	if entry.Status == config.TransferStatusQueued {
		entry.Status = config.TransferStatusCanceled
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = config.UpdateTransfer(m.home, entry)
		m.reloadTransfers()
		return m, nil
	}
	if entry.Status == config.TransferStatusRunning && m.activeTransfer.ID == entry.ID && m.activeTransfer.Cancel != nil {
		entry.Status = config.TransferStatusInterrupted
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = config.UpdateTransfer(m.home, entry)
		m.reloadTransfers()
		m.activeTransfer.Cancel()
		m.status = "已中断当前传输。再次按 c 可取消该任务。"
		return m, nil
	}
	if entry.Status == config.TransferStatusInterrupted {
		entry.Status = config.TransferStatusCanceled
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = config.UpdateTransfer(m.home, entry)
		m.reloadTransfers()
		m.status = "已取消当前中断任务。"
		return m, nil
	}
	m.status = "该任务当前不可取消。"
	return m, nil
}

func (m *Model) cancelTransferConfirm() {
	if m.panel.Confirming {
		m.panel.Confirming = false
		m.status = transferPanelStatus(m.panel.Mode)
	}
}

func (m *Model) movePanel(delta int) {
	if m.panel.ActivePane == 0 {
		m.panel.LeftIndex = moveIndex(m.panel.LeftIndex, len(m.panel.LeftChoices), delta)
		return
	}
	m.panel.RightIndex = moveIndex(m.panel.RightIndex, len(m.panel.RightChoices), delta)
}

func (m *Model) togglePanelSelection() {
	if m.panel.ActivePane != 0 {
		return
	}
	if len(m.panel.LeftChoices) == 0 || m.panel.LeftIndex < 0 || m.panel.LeftIndex >= len(m.panel.LeftChoices) {
		return
	}
	pick := m.panel.LeftChoices[m.panel.LeftIndex]
	if m.panel.LeftSelected == nil {
		m.panel.LeftSelected = map[string]bool{}
	}
	if m.panel.LeftSelected[pick.Value] {
		delete(m.panel.LeftSelected, pick.Value)
	} else {
		m.panel.LeftSelected[pick.Value] = true
	}
}

func (m Model) selectedTransferSources() []choice {
	if len(m.panel.LeftSelected) == 0 {
		return nil
	}
	out := make([]choice, 0, len(m.panel.LeftSelected))
	for path := range m.panel.LeftSelected {
		node := m.panel.LeftTree.Nodes[path]
		if node == nil {
			continue
		}
		out = append(out, choice{
			Label: treeLabel(node),
			Value: node.Item.Path,
			IsDir: node.Item.IsDir,
			Depth: node.Depth,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Value) < strings.ToLower(out[j].Value)
	})
	return out
}

func moveIndex(index, count, delta int) int {
	if count == 0 {
		return 0
	}
	index += delta
	if index < 0 {
		index = count - 1
	}
	if index >= count {
		index = 0
	}
	return index
}

func (m *Model) togglePanelTree() {
	tree, choices, index := m.activePanelTree()
	if tree == nil || len(*choices) == 0 || *index < 0 || *index >= len(*choices) {
		return
	}
	pick := (*choices)[*index]
	node := tree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return
	}
	if node.Expanded {
		node.Expanded = false
	} else {
		if !node.Loaded {
			loadTreeNodeFor(tree, node, m.states)
		}
		if len(node.Children) == 0 {
			m.status = "没有子目录：" + node.Item.Path
			return
		}
		node.Expanded = true
	}
	*choices = flattenTree(*tree)
	if *index >= len(*choices) {
		*index = len(*choices) - 1
	}
	if *index < 0 {
		*index = 0
	}
}

func (m *Model) activePanelTree() (*remoteTree, *[]choice, *int) {
	if m.panel.ActivePane == 0 {
		return &m.panel.LeftTree, &m.panel.LeftChoices, &m.panel.LeftIndex
	}
	return &m.panel.RightTree, &m.panel.RightChoices, &m.panel.RightIndex
}

func (m Model) confirmTransferPanel() (tea.Model, tea.Cmd) {
	m.panel.Confirming = false
	if len(m.selectedTransferSources()) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = "左侧至少选择一个文件或目录，右侧选择目标目录。"
		return m, nil
	}
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = "右侧必须选择目录。"
		return m, nil
	}
	m.transferJobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.status = "正在检测远程 rsync..."
	return m, m.checkRemoteRsync(m.panel.HostIndex)
}

func (m Model) prepareTransferConfirm() (tea.Model, tea.Cmd) {
	selected := m.selectedTransferSources()
	if len(selected) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = "左侧至少选择一个文件或目录，右侧选择目标目录。"
		return m, nil
	}
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = "右侧必须选择目录。"
		return m, nil
	}
	h := m.states[m.panel.HostIndex].Host
	m.panel.Confirming = true
	if m.panel.Mode == transferUpload {
		m.status = fmt.Sprintf("上传 Enter：%d 项 -> %s:%s/  取消 Esc", len(selected), hostDisplayName(h), right.Value)
		return m, nil
	}
	m.status = fmt.Sprintf("下载 Enter：%d 项 -> %s/  取消 Esc", len(selected), right.Value)
	return m, nil
}

func (m *Model) movePick(delta int) {
	if len(m.choices) == 0 {
		m.pickIndex = 0
		return
	}
	m.pickIndex += delta
	if m.pickIndex < 0 {
		m.pickIndex = len(m.choices) - 1
	}
	if m.pickIndex >= len(m.choices) {
		m.pickIndex = 0
	}
}

func (m Model) confirmPick() (tea.Model, tea.Cmd) {
	if len(m.choices) == 0 || m.pickIndex < 0 || m.pickIndex >= len(m.choices) {
		m.status = "没有可选择的项目。"
		return m, nil
	}
	pick := m.choices[m.pickIndex]
	switch m.mode {
	case modePickLocalRoot:
		m.pending.LocalRoot = pick.Value
		m.setChoices("选择本地文件/目录", modePickLocalItem, localItemChoices(fsselect.LocalItems(pick.Value)))
	case modePickLocalItem:
		m.pending.LocalPath = pick.Value
		m.pending.LocalIsDir = pick.IsDir
		h := m.states[m.pending.HostIndex].Host
		m.startRemoteTree("选择远程目录", modePickRemoteDir, h, true)
	case modePickRemoteDir:
		m.pending.RemoteDir = pick.Value
		return m.startUploadTransfer()
	case modePickRemoteItem:
		m.pending.RemotePath = pick.Value
		m.pending.RemoteIsDir = pick.IsDir
		m.startLocalTree("选择本地保存目录", modePickSaveDir, true)
	case modePickSaveDir:
		m.pending.SaveDir = pick.Value
		return m.startDownloadTransfer()
	}
	return m, nil
}

func (m *Model) setChoices(title string, mode viewMode, choices []choice) {
	m.pickTitle = title
	m.mode = mode
	m.choices = choices
	m.pickIndex = 0
	if len(choices) == 0 {
		m.status = title + "：没有可选择的项目"
	} else {
		m.status = title
	}
}

func (m Model) startTransfer(status string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.mode = modeDashboard
	m.transfer = transferNone
	m.choices = nil
	m.remoteTree = remoteTree{}
	m.pickIndex = 0
	m.status = status
	return m, cmd
}

func (m Model) startUploadTransfer() (tea.Model, tea.Cmd) {
	h := m.states[m.pending.HostIndex].Host
	localPath := m.pending.LocalPath
	remoteDir := m.pending.RemoteDir
	remotePath := remoteJoin(remoteDir, filepath.Base(localPath))
	total := localPathSize(localPath)
	ctx, cancel := context.WithCancel(context.Background())
	m.mode = modeDashboard
	m.transfer = transferNone
	m.choices = nil
	m.remoteTree = remoteTree{}
	m.pickIndex = 0
	m.activeTransfer = activeTransfer{
		Kind:       "上传",
		Source:     localPath,
		Target:     h.Name + ":" + remoteDir + "/",
		LocalPath:  localPath,
		RemotePath: remotePath,
		HostIndex:  m.pending.HostIndex,
		Total:      total,
		Active:     true,
		Cancel:     cancel,
	}
	m.status = transferProgressText(m.activeTransfer, m.states)
	return m, tea.Batch(m.runUpload(ctx), transferProgressAfter(500*time.Millisecond))
}

func (m Model) startDownloadTransfer() (tea.Model, tea.Cmd) {
	h := m.states[m.pending.HostIndex].Host
	remotePath := m.pending.RemotePath
	saveDir := m.pending.SaveDir
	localPath := filepath.Join(saveDir, filepath.Base(remotePath))
	total := remoteSizeBytes(h, remotePath)
	if total < 0 {
		total = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mode = modeDashboard
	m.transfer = transferNone
	m.choices = nil
	m.remoteTree = remoteTree{}
	m.pickIndex = 0
	m.activeTransfer = activeTransfer{
		Kind:      "下载",
		Source:    remotePath,
		Target:    saveDir + "/",
		LocalPath: localPath,
		HostIndex: m.pending.HostIndex,
		Total:     total,
		Active:    true,
		Cancel:    cancel,
	}
	m.status = transferProgressText(m.activeTransfer, m.states)
	return m, tea.Batch(m.runDownload(ctx), transferProgressAfter(500*time.Millisecond))
}

func (m Model) checkRemoteRsync(index int) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.states) {
			return rsyncCheckMsg{HostIndex: index, ErrText: "服务器索引无效"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		cmd, cleanup := actions.RemoteRsyncCheckCommand(ctx, m.states[index].Host)
		defer cleanup()
		err := cmd.Run()
		if err == nil {
			return rsyncCheckMsg{HostIndex: index}
		}
		return rsyncCheckMsg{HostIndex: index, Missing: true}
	}
}

func (m Model) installRemoteRsync(index int) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.states) {
			return rsyncInstallMsg{HostIndex: index, ErrText: "服务器索引无效"}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd, cleanup := actions.RemoteRsyncInstallCommand(ctx, m.states[index].Host)
		defer cleanup()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return rsyncInstallMsg{HostIndex: index, ErrText: transferErrorText(err, string(output))}
		}
		return rsyncInstallMsg{HostIndex: index}
	}
}

func (m Model) createTransferJobsFromPanel() (tea.Model, tea.Cmd) {
	selected := m.selectedTransferSources()
	if len(selected) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = "没有可传输的项目。"
		return m, nil
	}
	target := m.panel.RightChoices[m.panel.RightIndex]
	h := m.states[m.panel.HostIndex].Host
	now := time.Now()
	for i, item := range selected {
		totalBytes := int64(0)
		if m.panel.Mode == transferDownload {
			totalBytes = remoteSizeBytes(h, item.Value)
		} else {
			totalBytes = localSizeBytes(item.Value)
		}
		entry := config.TransferEntry{
			ID:           config.NewTransferID(now.Add(time.Duration(i))),
			Time:         now.Format(time.RFC3339),
			Kind:         transferKindString(m.panel.Mode),
			Status:       config.TransferStatusQueued,
			HostCategory: h.Category,
			HostName:     h.Name,
			Host:         h.HostName,
			User:         h.User,
			Port:         h.Port,
			Source:       item.Value,
			TargetDir:    target.Value,
			IsDir:        item.IsDir,
			TotalBytes:   totalBytes,
			UpdatedAt:    now.Format(time.RFC3339),
		}
		_ = config.AppendTransfer(m.home, entry)
	}
	m.reloadTransfers()
	m.transferJobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.transfer = transferNone
	m.status = fmt.Sprintf("已创建 %d 个传输任务。", len(selected))
	return m, nil
}

func transferKindString(mode transferMode) string {
	if mode == transferDownload {
		return "download"
	}
	return "upload"
}

func (m *Model) reloadTransfers() {
	file, _, _ := config.LoadTransfers(m.home)
	m.transferHistory = file
	if m.transferIndex >= len(m.transferHistory.Entries) {
		m.transferIndex = len(m.transferHistory.Entries) - 1
	}
	if m.transferIndex < 0 {
		m.transferIndex = 0
	}
	m.ensureTransferIndexVisible()
}

func (m *Model) ensureTransferIndexVisible() {
	indexes := m.filteredTransferIndexes()
	if len(indexes) == 0 {
		m.transferIndex = 0
		return
	}
	for _, index := range indexes {
		if index == m.transferIndex {
			return
		}
	}
	m.transferIndex = indexes[0]
}

func (m *Model) startLocalTree(title string, mode viewMode, dirsOnly bool) {
	m.startTree(title, mode, localTreeItems("/", true), -1, dirsOnly, true)
}

func newTree(roots []fsselect.Item, hostIndex int, dirsOnly bool, local bool) remoteTree {
	tree := remoteTree{
		HostIndex: hostIndex,
		Local:     local,
		DirsOnly:  dirsOnly,
		Nodes:     map[string]*remoteTreeNode{},
	}
	for _, item := range roots {
		if item.Path == "" || (dirsOnly && !item.IsDir) {
			continue
		}
		if _, ok := tree.Nodes[item.Path]; ok {
			continue
		}
		tree.Roots = append(tree.Roots, item.Path)
		tree.Nodes[item.Path] = &remoteTreeNode{Item: item}
	}
	sort.Strings(tree.Roots)
	return tree
}

func (m *Model) startRemoteTree(title string, mode viewMode, h host.Host, dirsOnly bool) {
	m.startTree(title, mode, fsselect.RemoteRootItems(h), m.pending.HostIndex, dirsOnly, false)
}

func (m *Model) startRemoteTreeAt(title string, mode viewMode, h host.Host, root string, dirsOnly bool) {
	if root == "" {
		m.startRemoteTree(title, mode, h, dirsOnly)
		return
	}
	m.startTree(title, mode, []fsselect.Item{{Path: root, IsDir: true}}, m.pending.HostIndex, dirsOnly, false)
	if len(m.choices) > 0 {
		_, _ = m.expandTreePick()
	}
}

func (m *Model) startTree(title string, mode viewMode, roots []fsselect.Item, hostIndex int, dirsOnly bool, local bool) {
	tree := newTree(roots, hostIndex, dirsOnly, local)
	m.remoteTree = tree
	m.pickTitle = title
	m.mode = mode
	m.pickIndex = 0
	m.refreshTreeChoices()
	if len(m.choices) == 0 {
		m.status = title + "：没有可选择的项目"
	} else {
		m.status = title
	}
}

func (m Model) treePickerActive() bool {
	switch m.mode {
	case modePickLocalItem, modePickRemoteDir, modePickRemoteItem, modePickSaveDir:
		return m.remoteTree.Nodes != nil
	default:
		return false
	}
}

func (m *Model) refreshTreeChoices() {
	var choices []choice
	for _, root := range m.remoteTree.Roots {
		m.appendTreeChoice(&choices, root)
	}
	m.choices = choices
	if m.pickIndex >= len(m.choices) {
		m.pickIndex = len(m.choices) - 1
	}
	if m.pickIndex < 0 {
		m.pickIndex = 0
	}
}

func (m *Model) appendTreeChoice(choices *[]choice, path string) {
	node := m.remoteTree.Nodes[path]
	if node == nil {
		return
	}
	label := treeLabel(node)
	*choices = append(*choices, choice{Label: label, Value: node.Item.Path, IsDir: node.Item.IsDir, Depth: node.Depth})
	if !node.Expanded {
		return
	}
	for _, child := range node.Children {
		m.appendTreeChoice(choices, child)
	}
}

func flattenTree(tree remoteTree) []choice {
	var choices []choice
	for _, root := range tree.Roots {
		appendTreeChoiceTo(&choices, tree, root)
	}
	return choices
}

func appendTreeChoiceTo(choices *[]choice, tree remoteTree, path string) {
	node := tree.Nodes[path]
	if node == nil {
		return
	}
	*choices = append(*choices, choice{Label: treeLabel(node), Value: node.Item.Path, IsDir: node.Item.IsDir, Depth: node.Depth})
	if !node.Expanded {
		return
	}
	for _, child := range node.Children {
		appendTreeChoiceTo(choices, tree, child)
	}
}

func (m Model) expandTreePick() (tea.Model, tea.Cmd) {
	if len(m.choices) == 0 || m.pickIndex < 0 || m.pickIndex >= len(m.choices) {
		return m, nil
	}
	pick := m.choices[m.pickIndex]
	node := m.remoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m, nil
	}
	if !node.Loaded {
		m.loadTreeNode(node)
	}
	if len(node.Children) == 0 {
		if m.remoteTree.DirsOnly {
			m.status = "没有子目录：" + node.Item.Path + "。按空格可选择当前目录。"
		} else {
			m.status = "目录为空或没有权限：" + node.Item.Path
		}
		return m, nil
	}
	node.Expanded = true
	m.refreshTreeChoices()
	return m, nil
}

func (m Model) toggleTreePick() (tea.Model, tea.Cmd) {
	if len(m.choices) == 0 || m.pickIndex < 0 || m.pickIndex >= len(m.choices) {
		return m, nil
	}
	pick := m.choices[m.pickIndex]
	node := m.remoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m.confirmPick()
	}
	if node.Expanded {
		node.Expanded = false
		m.refreshTreeChoices()
		return m, nil
	}
	return m.expandTreePick()
}

func (m Model) collapseTreePick() Model {
	if len(m.choices) == 0 || m.pickIndex < 0 || m.pickIndex >= len(m.choices) {
		return m
	}
	pick := m.choices[m.pickIndex]
	node := m.remoteTree.Nodes[pick.Value]
	if node == nil || !node.Item.IsDir {
		return m
	}
	if node.Expanded {
		node.Expanded = false
		m.refreshTreeChoices()
		return m
	}
	parent := filepath.Dir(node.Item.Path)
	for parent != "." && parent != "/" {
		if _, ok := m.remoteTree.Nodes[parent]; ok {
			for i, choice := range m.choices {
				if choice.Value == parent {
					m.pickIndex = i
					return m
				}
			}
		}
		parent = filepath.Dir(parent)
	}
	return m
}

func (m *Model) loadTreeNode(node *remoteTreeNode) {
	loadTreeNodeFor(&m.remoteTree, node, m.states)
}

func loadTreeNodeFor(tree *remoteTree, node *remoteTreeNode, states []hostState) {
	var items []fsselect.Item
	if tree.Local {
		items = localTreeItems(node.Item.Path, tree.DirsOnly)
	} else if tree.HostIndex >= 0 && tree.HostIndex < len(states) {
		h := states[tree.HostIndex].Host
		if tree.DirsOnly {
			items = fsselect.RemoteDirItems(h, node.Item.Path)
		} else {
			items = fsselect.RemoteItems(h, node.Item.Path)
		}
	}
	node.Children = nil
	seen := map[string]bool{}
	for _, item := range items {
		if item.Path == "" || item.Path == node.Item.Path {
			continue
		}
		if seen[item.Path] {
			continue
		}
		seen[item.Path] = true
		if tree.DirsOnly && !item.IsDir {
			continue
		}
		tree.Nodes[item.Path] = &remoteTreeNode{Item: item, Depth: node.Depth + 1}
		node.Children = append(node.Children, item.Path)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		a := tree.Nodes[node.Children[i]].Item
		b := tree.Nodes[node.Children[j]].Item
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(filepath.Base(a.Path)) < strings.ToLower(filepath.Base(b.Path))
	})
	node.Loaded = true
}

func localTreeItems(dir string, dirsOnly bool) []fsselect.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	items := make([]fsselect.Item, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if dirsOnly && !entry.IsDir() {
			continue
		}
		items = append(items, fsselect.Item{Path: filepath.Join(dir, entry.Name()), IsDir: entry.IsDir()})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(filepath.Base(items[i].Path)) < strings.ToLower(filepath.Base(items[j].Path))
	})
	return items
}

func treeLabel(node *remoteTreeNode) string {
	indent := strings.Repeat("  ", node.Depth)
	name := node.Item.Path
	if node.Depth > 0 {
		name = filepath.Base(node.Item.Path)
	}
	return indent + name
}

func (m Model) startTransferPanel(idx int, mode transferMode) Model {
	h := m.states[idx].Host
	remoteTitle := "远程 " + hostDisplayName(h)
	panel := transferPanel{Mode: mode, HostIndex: idx, LeftSelected: map[string]bool{}}
	if mode == transferUpload {
		panel.LeftTitle = "本地"
		panel.RightTitle = remoteTitle
		panel.LeftTree = newTree(localTreeItems("/", true), -1, false, true)
		panel.RightTree = newTree(fsselect.RemoteRootItems(h), idx, true, false)
		m.status = transferPanelStatus(mode)
	} else {
		panel.LeftTitle = remoteTitle
		panel.RightTitle = "本地"
		panel.LeftTree = newTree(fsselect.RemoteRootItems(h), idx, false, false)
		panel.RightTree = newTree(localTreeItems("/", true), -1, true, true)
		m.status = transferPanelStatus(mode)
	}
	panel.LeftChoices = flattenTree(panel.LeftTree)
	panel.RightChoices = flattenTree(panel.RightTree)
	m.mode = modeTransferPanel
	m.transfer = mode
	m.pending = pendingTransfer{HostIndex: idx}
	m.panel = panel
	return m
}

func hostDisplayName(h host.Host) string {
	return "[" + strings.TrimSpace(h.Category) + "] " + h.Name
}

func dashboardHostDisplayName(h host.Host) string {
	parts := make([]string, 0, 3)
	if h.Pinned {
		parts = append(parts, "▲")
	}
	if h.Favorite {
		parts = append(parts, "★")
	}
	parts = append(parts, hostDisplayName(h))
	return strings.Join(parts, " ")
}

func transferPanelStatus(mode transferMode) string {
	if mode == transferUpload {
		return "上传：左侧多选本地文件/目录，右侧选择远程目录，s 开始。"
	}
	return "下载：左侧多选远程文件/目录，右侧选择本地目录，s 开始。"
}

func (m Model) startUpload(idx int) Model {
	return m.startTransferPanel(idx, transferUpload)
}

func (m Model) startDownload(idx int) Model {
	return m.startTransferPanel(idx, transferDownload)
}

func (m Model) runUpload(ctx context.Context) tea.Cmd {
	h := m.states[m.pending.HostIndex].Host
	localPath := m.pending.LocalPath
	remoteDir := m.pending.RemoteDir
	recursive := m.pending.LocalIsDir
	cmd, cleanup := actions.SCPUploadCommandContext(ctx, h, localPath, remoteDir, recursive)
	return func() tea.Msg {
		output, err := cmd.CombinedOutput()
		cleanup()
		return transferDoneMsg{Kind: "上传", Source: localPath, Target: h.Name + ":" + remoteDir + "/", Err: err, Output: string(output)}
	}
}

func (m Model) runDownload(ctx context.Context) tea.Cmd {
	h := m.states[m.pending.HostIndex].Host
	remotePath := m.pending.RemotePath
	saveDir := m.pending.SaveDir
	recursive := m.pending.RemoteIsDir
	cmd, cleanup := actions.SCPDownloadCommandContext(ctx, h, remotePath, saveDir, recursive)
	return func() tea.Msg {
		output, err := cmd.CombinedOutput()
		cleanup()
		return transferDoneMsg{Kind: "下载", Source: remotePath, Target: saveDir + "/", Err: err, Output: string(output)}
	}
}

func (m Model) startNextQueuedTransfer() (tea.Model, tea.Cmd) {
	if m.activeTransfer.Active {
		return m, nil
	}
	for _, entry := range m.transferHistory.Entries {
		if entry.Status == config.TransferStatusPending {
			return m.startTransferEntry(entry)
		}
	}
	m.transferRunAll = false
	return m, clearStatusAfter(3 * time.Second)
}

func (m Model) startTransferEntry(entry config.TransferEntry) (tea.Model, tea.Cmd) {
	h, index, ok := m.findTransferHost(entry)
	if !ok {
		entry.Status = config.TransferStatusFailed
		entry.Error = "找不到服务器：" + entry.HostName
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
		_ = config.UpdateTransfer(m.home, entry)
		m.reloadTransfers()
		return m, clearStatusAfter(3 * time.Second)
	}
	ctx, cancel := context.WithCancel(context.Background())
	entry.Status = config.TransferStatusRunning
	entry.Error = ""
	entry.UpdatedAt = time.Now().Format(time.RFC3339)
	_ = config.UpdateTransfer(m.home, entry)
	m.activeTransfer = activeTransfer{
		ID:        entry.ID,
		Kind:      transferEntryKindText(entry),
		Source:    entry.Source,
		Target:    entry.TargetDir,
		HostIndex: index,
		Active:    true,
		Cancel:    cancel,
	}
	m.reloadTransfers()
	m.status = transferProgressText(m.activeTransfer, m.states)
	cmd := func() tea.Msg {
		cmd, cleanup := m.rsyncCommandForEntry(ctx, h, entry)
		output, err := runRsyncWithProgress(cmd, m.home, entry.ID)
		cleanup()
		cancel()
		return transferDoneMsg{ID: entry.ID, Kind: transferEntryKindText(entry), Source: entry.Source, Target: entry.TargetDir, Err: err, Output: string(output)}
	}
	return m, tea.Batch(cmd, transferProgressAfter(500*time.Millisecond))
}

func runRsyncWithProgress(cmd *exec.Cmd, home string, id string) (string, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var mu sync.Mutex
	var output strings.Builder
	lastProgress := ""
	collect := func(text string) {
		progress := ""
		mu.Lock()
		output.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			output.WriteString("\n")
		}
		if progressText := rsyncProgressText(text); progressText != "" && progressText != lastProgress {
			lastProgress = progressText
			progress = progressText
		}
		mu.Unlock()
		if progress != "" {
			updateTransferProgress(home, id, progress)
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		readRsyncProgress(stdout, collect)
	}()
	go func() {
		defer wg.Done()
		readRsyncProgress(stderr, collect)
	}()
	err = cmd.Wait()
	wg.Wait()
	mu.Lock()
	text := output.String()
	mu.Unlock()
	return text, err
}

func readRsyncProgress(r io.Reader, collect func(string)) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitRsyncProgress)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			collect(text)
		}
	}
}

func splitRsyncProgress(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (m Model) rsyncCommandForEntry(ctx context.Context, h host.Host, entry config.TransferEntry) (*exec.Cmd, actions.Cleanup) {
	if entry.Kind == "download" {
		return actions.RsyncDownloadCommandContext(ctx, h, entry.Source, entry.TargetDir)
	}
	return actions.RsyncUploadCommandContext(ctx, h, entry.Source, entry.TargetDir)
}

func (m Model) findTransferHost(entry config.TransferEntry) (host.Host, int, bool) {
	for i, state := range m.states {
		h := state.Host
		if h.Name == entry.HostName && h.Category == entry.HostCategory {
			return h, i, true
		}
	}
	return host.Host{}, -1, false
}

func transferEntryKindText(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return "下载"
	}
	return "上传"
}

func (m *Model) updateTransferEntryDone(msg transferDoneMsg) {
	file, _, err := config.LoadTransfers(m.home)
	if err != nil {
		return
	}
	for i := range file.Entries {
		if file.Entries[i].ID != msg.ID {
			continue
		}
		if file.Entries[i].Status == config.TransferStatusCanceled || file.Entries[i].Status == config.TransferStatusInterrupted {
			_ = config.SaveTransfers(m.home, file)
			return
		}
		file.Entries[i].UpdatedAt = time.Now().Format(time.RFC3339)
		file.Entries[i].Progress = lastRsyncProgressLine(msg.Output)
		updateTransferProgressBytes(&file.Entries[i], file.Entries[i].Progress)
		if msg.Err != nil {
			file.Entries[i].Status = config.TransferStatusFailed
			file.Entries[i].Error = transferErrorText(msg.Err, msg.Output)
		} else {
			file.Entries[i].Status = config.TransferStatusDone
			file.Entries[i].Progress = "100%"
			if file.Entries[i].TotalBytes > 0 {
				file.Entries[i].DoneBytes = file.Entries[i].TotalBytes
				file.Entries[i].CurrentBytes = 0
			}
			file.Entries[i].Error = ""
		}
		_ = config.SaveTransfers(m.home, file)
		return
	}
}

func updateTransferProgress(home string, id string, progress string) {
	if id == "" || progress == "" {
		return
	}
	_, _ = config.UpdateRunningTransferProgress(home, id, func(entry *config.TransferEntry) {
		entry.Progress = progress
		updateTransferProgressBytes(entry, progress)
		entry.UpdatedAt = time.Now().Format(time.RFC3339)
	})
}

func updateTransferProgressBytes(entry *config.TransferEntry, progress string) {
	bytes, percent, seq, ok := parseRsyncProgressValues(progress)
	if !ok {
		return
	}
	if percent >= 100 && seq > 0 && seq > entry.ProgressSeq {
		entry.DoneBytes += bytes
		entry.CurrentBytes = 0
		entry.ProgressSeq = seq
	} else if percent >= 100 && entry.TotalBytes > 0 && bytes >= entry.TotalBytes {
		entry.DoneBytes = entry.TotalBytes
		entry.CurrentBytes = 0
	} else {
		entry.CurrentBytes = bytes
	}
	if entry.TotalBytes > 0 && entry.DoneBytes > entry.TotalBytes {
		entry.DoneBytes = entry.TotalBytes
	}
}

func (m Model) markActiveTransferInterrupted() {
	if m.activeTransfer.ID == "" {
		return
	}
	file, _, err := config.LoadTransfers(m.home)
	if err != nil {
		return
	}
	for i := range file.Entries {
		if file.Entries[i].ID == m.activeTransfer.ID && file.Entries[i].Status == config.TransferStatusRunning {
			file.Entries[i].Status = config.TransferStatusInterrupted
			file.Entries[i].UpdatedAt = time.Now().Format(time.RFC3339)
			_ = config.SaveTransfers(m.home, file)
			return
		}
	}
}

func lastRsyncProgressLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if progress := rsyncProgressText(line); progress != "" {
			return progress
		}
	}
	return ""
}

var rsyncPercentPattern = regexp.MustCompile(`\b([0-9]{1,3})%`)
var rsyncXferPattern = regexp.MustCompile(`xfer#([0-9]+)`)

func rsyncProgressText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || rsyncPercentText(value) == "" {
		return ""
	}
	return value
}

func rsyncPercentText(value string) string {
	match := rsyncPercentPattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	percent, err := strconv.Atoi(match[1])
	if err != nil {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return fmt.Sprintf("%d%%", percent)
}

func parseRsyncProgressValues(value string) (int64, int, int, bool) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) < 2 {
		return 0, 0, 0, false
	}
	bytesText := strings.ReplaceAll(fields[0], ",", "")
	bytes, err := strconv.ParseInt(bytesText, 10, 64)
	if err != nil || bytes < 0 {
		return 0, 0, 0, false
	}
	percentText := strings.TrimSuffix(fields[1], "%")
	percent, err := strconv.Atoi(percentText)
	if err != nil {
		return 0, 0, 0, false
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	seq := 0
	if match := rsyncXferPattern.FindStringSubmatch(value); len(match) == 2 {
		seq, _ = strconv.Atoi(match[1])
	}
	return bytes, percent, seq, true
}

func remoteSizeBytes(h host.Host, remotePath string) int64 {
	cmd, cleanup := actions.RemoteSizeCommand(h, remotePath)
	defer cleanup()
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || size < 0 {
		return 0
	}
	return size
}

func localSizeBytes(path string) int64 {
	info, err := os.Lstat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.WalkDir(path, func(itemPath string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func transferProgressAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return transferProgressMsg(t)
	})
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func clearScreen() tea.Cmd {
	return func() tea.Msg {
		return tea.ClearScreen()
	}
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

func renderDashboardHelp(width int) string {
	if width < 1 {
		width = 1
	}
	help := strings.Join([]string{
		"更多 ?",
		"移动 ↑↓←→/hjkl",
		"登录 Enter",
		"详情 Space",
		"命令 m",
		"批量 b",
		"历史 i",
		"传输 y",
		"部署 g",
		"总览 w",
		"视图 z",
		"置顶 t",
		"收藏 f",
		"收藏 v",
		"添加 a",
		"复制 c",
		"编辑 e",
		"删除 x",
		"上传 u",
		"下载 d",
		"刷新 r",
		"搜索 /",
		"分类 Tab",
		"在线 o",
		"异常 p",
		"排序 s",
		"退出 q",
	}, "  ")
	return helpStyle.Render(fit(help, width))
}

func (m Model) renderAddForm() string {
	title := "添加服务器"
	if m.editing {
		title = "编辑服务器"
	} else if m.copying {
		title = "复制服务器"
	}
	width := formContentWidth(m.width)
	if m.useSingleFormPane(width) {
		width = detailFrameWidth(m.width)
	}
	help := "切换 Tab  选择 ↑↓  分类 ←→  保存 Enter  返回 Esc"
	if m.formPane == 1 {
		help = "切回 Tab  选择 ↑↓  新增 n  重命名 r  删除 x  返回 Esc"
		if m.addingCategory {
			help = "添加 Enter  返回 Esc"
		} else if m.renamingCategory {
			help = "重命名 Enter  返回 Esc"
		}
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if strings.Contains(m.status, "失败") || strings.Contains(m.status, "不能") {
			statusStyle = redStyle
		}
		header += "  " + statusStyle.Render(fit(m.status, width-ansi.StringWidth(title)-2))
	}
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	body := ""
	if m.useSingleFormPane(width) {
		if m.formPane == 1 {
			body = m.renderCategoryPane(width, bodyHeight)
		} else {
			body = m.renderServerFormPane(title, width, bodyHeight)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) * 2 / 3
		rightWidth := width - gap - leftWidth
		if rightWidth < 28 {
			rightWidth = 28
			leftWidth = width - gap - rightWidth
		}
		left := m.renderServerFormPane(title, leftWidth, bodyHeight)
		right := m.renderCategoryPane(rightWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	lines := []string{
		header,
		body,
		renderHelp(width, help),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDeploymentList() string {
	pageWidth := m.width
	if pageWidth <= 0 {
		pageWidth = contentWidth(m.width)
	}
	if pageWidth < 34 {
		pageWidth = 34
	}
	frameWidth := detailFrameWidth(pageWidth)
	bodyWidth := frameWidth - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "移动 ↑↓←→/hjkl  详情 Space  选择 s  分类 Tab  视图 z  置顶 t  收藏 f  只看收藏 v  部署 Enter  新增 a  编辑 e  删除 x  返回 Esc"
	header := titleStyle.Render(fit(strings.Join(m.deploymentHeaderParts(), "  "), pageWidth))
	if m.deploymentView == deploymentViewCards {
		bodyHeight := deploymentCardsHeight(m.height, false)
		pageDots := m.deploymentPageDots(pageWidth, bodyHeight)
		if pageDots != "" {
			bodyHeight = deploymentCardsHeight(m.height, true)
			pageDots = m.deploymentPageDots(pageWidth, bodyHeight)
		}
		lines := []string{header, "", m.renderDeploymentCards(pageWidth, bodyHeight)}
		if pageDots != "" {
			lines = append(lines, pageDots)
		}
		lines = append(lines, renderHelp(pageWidth, help))
		return strings.Join(lines, "\n")
	}
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}
	lines := []string{}
	if len(m.deploymentItems) == 0 {
		lines = append(lines, mutedStyle.Render("没有部署应用。按 a 添加。"))
	} else {
		start, end := visibleRange(len(m.deploymentItems), m.deploymentIndex, contentHeight)
		for i := start; i < end; i++ {
			item := m.deploymentItems[i]
			lines = append(lines, m.deploymentAppListLine(item, bodyWidth, i == m.deploymentIndex))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(frameWidth).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{header, box, renderHelp(pageWidth, help)}, "\n")
}

func (m Model) deploymentHeaderParts() []string {
	parts := []string{"应用部署", fmt.Sprintf("应用 %d", len(m.deploymentItems)), "视图：" + deploymentViewName(m.deploymentView)}
	if m.deploymentCategory != "" {
		parts = append(parts, "分类："+m.deploymentCategory)
	}
	if len(m.deploymentSelected) > 0 {
		parts = append(parts, fmt.Sprintf("已选 %d", len(m.deploymentSelected)))
	}
	if m.deploymentFavoriteOnly {
		parts = append(parts, "只看收藏")
	}
	if m.status != "" && m.status != "应用部署" && !strings.HasPrefix(m.status, "部署视图：") {
		parts = append(parts, m.status)
	}
	return parts
}

func deploymentViewName(view deploymentViewMode) string {
	if view == deploymentViewList {
		return "列表"
	}
	return "卡片"
}

func deploymentCardsHeight(totalHeight int, withDots bool) int {
	bodyHeight := totalHeight - 3
	if withDots {
		bodyHeight--
	}
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	return bodyHeight
}

func (m Model) renderDeploymentCards(width int, bodyHeight int) string {
	lines := []string{}
	if len(m.deploymentItems) == 0 {
		lines = append(lines, mutedStyle.Render("没有部署应用。按 a 添加。"))
		for len(lines) < bodyHeight {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	cardLines, selectedTop, selectedBottom := m.deploymentCardLines(width)
	cardHeight := bodyHeight
	if cardHeight < 1 {
		cardHeight = 1
	}
	start, end := dashboardLineWindow(len(cardLines), selectedTop, selectedBottom, cardHeight)
	lines = append(lines, cardLines[start:end]...)
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentPageDots(width int, bodyHeight int) string {
	if len(m.deploymentItems) == 0 || bodyHeight <= 0 {
		return ""
	}
	lines, selectedTop, selectedBottom := m.deploymentCardLines(width)
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, bodyHeight, width)
}

func (m Model) deploymentCardLines(width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	for i := 0; i < len(m.deploymentItems); i += cols {
		rowEnd := minInt(i+cols, len(m.deploymentItems))
		row := []string{}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= rowEnd {
				row = append(row, padBlock(blankDeploymentCard(cardWidth), cardWidth))
				continue
			}
			itemIndex := i + col
			if itemIndex == m.deploymentIndex {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.renderDeploymentAppCard(m.deploymentItems[itemIndex], cardWidth, itemIndex == m.deploymentIndex), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.deploymentIndex >= i && m.deploymentIndex < rowEnd {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func deploymentCardColumns(width int) int {
	switch {
	case width >= 148:
		return 3
	case width >= 96:
		return 2
	default:
		return 1
	}
}

func blankDeploymentCard(width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder()).
		Padding(0, 1).
		Width(maxInt(30, width-4)).
		Height(deploymentCardInnerHeight).
		Render("")
}

func (m Model) renderDeploymentAppCard(item deploymentItem, width int, selected bool) string {
	app := item.App
	cardWidth := maxInt(34, width)
	innerWidth := cardWidth - 4
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	order := m.deploymentSelectionOrder(item.Index)
	mark := ""
	if order > 0 {
		mark = blueStyle.Bold(true).Render(fmt.Sprintf("%02d ", order))
	}
	title := deploymentAppMarks(app) + mark + detailValueStyle.Render(emptyDash(app.Name))
	meta := m.deploymentLastRecordMeta(app)
	dot := m.deploymentLastRecordDot(app)
	sourceLine := deploymentCardSourceLine(app, innerWidth)
	serverLine := deploymentCardServerLine(app.Server, innerWidth)
	pathLine := cardMutedStyle.Render("目录 ") + detailValueStyle.Render(emptyDash(app.Path))
	fetchLine := cardMutedStyle.Render("方式 ") + detailValueStyle.Render(deployFetchModeText(app.FetchMode))
	timeLine := m.deploymentLastRecordTimeLine(app, innerWidth)
	lines := []string{
		cardTopLine(cardWidth, title, meta, dot, borderStyle),
		cardContentLine(cardWidth, serverLine, borderStyle),
		cardContentLine(cardWidth, sourceLine, borderStyle),
		cardContentLine(cardWidth, pathLine, borderStyle),
		cardContentLine(cardWidth, fetchLine, borderStyle),
		cardInnerSeparatorLine(cardWidth, borderStyle),
		cardContentLine(cardWidth, timeLine, borderStyle),
		cardBottomLine(cardWidth, borderStyle),
	}
	return strings.Join(lines, "\n")
}

func deploymentCardSourceLine(app config.DeploymentApp, width int) string {
	left := cardMutedStyle.Render(deploySourceText(app.Source)+" ") + detailValueStyle.Render(deploymentAppTarget(app))
	right := cardMutedStyle.Render(deployCredentialText(app.Credential))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		maxLeft := width - ansi.StringWidth(right) - 2
		if maxLeft < 8 {
			return fitANSI(left, width)
		}
		left = fitANSI(left, maxLeft)
		gap = width - ansi.StringWidth(left) - ansi.StringWidth(right)
	}
	return left + strings.Repeat(" ", maxInt(1, gap)) + right
}

func deploymentCardServerLine(server string, width int) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return transferPathLine("→", "-")
	}
	category := ""
	name := server
	if idx := strings.Index(server, "/"); idx >= 0 {
		category = strings.TrimSpace(server[:idx])
		name = strings.TrimSpace(server[idx+1:])
	}
	if name == "" {
		name = server
	}
	line := blueStyle.Render("→ ")
	if category != "" {
		line += cardMutedStyle.Render("["+category+"]") + " "
	}
	line += detailValueStyle.Render(name)
	return fitANSI(line, width)
}

func (m Model) deploymentLastRecordMeta(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("未部署")
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(deploymentRecordActionStatusText(record))
}

func (m Model) deploymentLastRecordDot(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("●")
	}
	if record.Status == config.DeployStatusFailed {
		return redStyle.Render("●")
	}
	return greenStyle.Render("●")
}

func (m Model) deploymentLastRecordTimeLine(app config.DeploymentApp, width int) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("时间 暂无记录")
	}
	return cardMutedStyle.Render("时间 ") + detailValueStyle.Render(fit(deploymentRecordTimeText(record.Time), width-5))
}

func (m Model) renderDeploymentDetail() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := m.deploymentDetailLines(bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentDetailMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit("部署详情", width)), box, renderHelp(width, "滚动 ↑↓/jk  部署 Enter  编辑 e  返回 Esc")}, "\n")
}

func (m Model) deploymentDetailMaxScroll() int {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, len(m.deploymentDetailLines(bodyWidth))-height)
}

func (m Model) deploymentDetailLines(bodyWidth int) []string {
	app := deploymentAppWithResourceDefaults(m.deploymentDetail)
	lines := []string{
		detailSubTitle("基础"),
		deploymentDetailRow("应用", emptyDash(app.Name), bodyWidth),
		deploymentDetailRow("服务器", deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow("来源", deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow("获取方式", deployFetchModeText(app.FetchMode), bodyWidth),
		deploymentDetailRow("仓库", emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow("目标", deploymentAppTarget(app), bodyWidth),
		deploymentDetailRow("资源匹配", emptyDash(app.Asset), bodyWidth),
		deploymentDetailRow("下载地址", emptyDash(app.ReleaseURL), bodyWidth),
		deploymentDetailRow("目录", emptyDash(app.Path), bodyWidth),
		deploymentDetailRow("凭证", deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow("凭证参数", emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow("等待", fmt.Sprintf("%d秒", maxInt(0, app.WaitSeconds)), bodyWidth),
		deploymentDetailRow("收藏", yesNo(app.Favorite), bodyWidth),
		deploymentDetailRow("置顶", yesNo(app.Pinned), bodyWidth),
		"",
		detailSubTitle("流程代码"),
	}
	records := m.deploymentRecordsForApp(app, 50)
	lines = appendDeploymentDetailCommands(lines, "更新前", app.BeforeCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "获取资源", app.ResourceCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "更新", app.UpdateCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "更新后", app.AfterCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "健康检查", app.HealthCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "回滚", app.RollbackCommands, bodyWidth)
	lines = append(lines, "", detailSubTitle(fmt.Sprintf("历史 %d条", len(records))))
	if len(records) == 0 {
		lines = append(lines, mutedStyle.Render("暂无记录"))
		return lines
	}
	for _, record := range records {
		lines = append(lines, deploymentDetailHistoryLine(record, bodyWidth))
	}
	return lines
}

func appendDeploymentDetailCommands(lines []string, title string, commands []string, width int) []string {
	lines = append(lines, deploymentDetailStageLine(title, len(commands), width))
	if len(commands) == 0 {
		lines = append(lines, mutedStyle.Render("    未配置"))
		return append(lines, "")
	}
	for _, command := range commands {
		lines = appendWrappedCommandIndented(lines, command, width)
	}
	return append(lines, "")
}

func deploymentDetailRow(label string, value string, width int) string {
	labelWidth := 10
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	return fitANSI(prefix+detailValueStyle.Render(value), width)
}

func deploymentDetailStageLine(title string, count int, width int) string {
	label := detailLabelStyle.Render(padVisible(title, 10))
	countText := cardMutedStyle.Render(fmt.Sprintf("%d步", count))
	return fitANSI(label+countText, width)
}

func appendWrappedCommandIndented(lines []string, command string, width int) []string {
	prefix := cardMutedStyle.Render("    $ ")
	wrapped := strings.Split(wrapPlainLine(command, width-ansi.StringWidth(prefix)), "\n")
	for _, line := range wrapped {
		lines = append(lines, prefix+detailValueStyle.Render(line))
	}
	return lines
}

func deploymentDetailHistoryLine(record config.DeploymentRecord, width int) string {
	statusText := deploymentRecordActionStatusText(record)
	statusStyle := greenStyle
	if record.Status == config.DeployStatusFailed {
		statusStyle = redStyle
	}
	version := cardMutedStyle.Render(deploymentRecordVersionText(record))
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = "  " + redStyle.Render(fmt.Sprintf("退出码 %d", record.ExitCode))
	}
	line := cardMutedStyle.Render(padVisible(deploymentRecordDateTimeText(record.Time), 11)) +
		"  " + statusStyle.Render(padVisible(statusText, 8)) +
		"  " + version + exit
	return fitANSI(line, width)
}

func (m Model) deploymentAppListLine(item deploymentItem, width int, selected bool) string {
	app := item.App
	prefix := cardMutedStyle.Render(" ")
	nameStyle := detailValueStyle
	if selected {
		prefix = blueStyle.Bold(true).Render("▶")
		nameStyle = blueStyle.Bold(true)
	}
	mark := "  "
	if order := m.deploymentSelectionOrder(item.Index); order > 0 {
		mark = blueStyle.Bold(true).Render(fmt.Sprintf("%02d", order))
	} else {
		mark = cardMutedStyle.Render(mark)
	}
	flags := deploymentAppListMarks(app)
	name := nameStyle.Render(padVisible(emptyDash(app.Name), 14))
	server := cardMutedStyle.Render(padVisible(deploymentDisplayServerText(app.Server), 18))
	source := detailValueStyle.Render(padVisible(deploySourceText(app.Source), 7))
	target := cardMutedStyle.Render(padVisible(deploymentAppTarget(app), 10))
	credential := cardMutedStyle.Render(padVisible(deployCredentialText(app.Credential), 8))
	record := m.deploymentLastRecordListText(app)
	return fitANSI(strings.Join([]string{prefix, mark, flags, name, server, source, target, credential, record}, "  "), width)
}

func deploymentAppMarks(app config.DeploymentApp) string {
	marks := ""
	if app.Pinned {
		marks += pinnedStyle.Render("▲") + " "
	}
	if app.Favorite {
		marks += favoriteStyle.Render("★") + " "
	}
	return marks
}

func deploymentAppListMarks(app config.DeploymentApp) string {
	marks := ""
	if app.Pinned {
		marks += pinnedStyle.Render("▲")
	} else {
		marks += " "
	}
	marks += " "
	if app.Favorite {
		marks += favoriteStyle.Render("★")
	} else {
		marks += " "
	}
	return padVisible(marks, 3)
}

func (m Model) deploymentLastRecordText(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return "未部署"
	}
	return deploymentRecordActionStatusText(record) + "  " + deploymentRecordTimeText(record.Time)
}

func (m Model) deploymentLastRecordListText(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("未部署")
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(deploymentRecordActionStatusText(record)) + "  " + cardMutedStyle.Render(deploymentRecordTimeText(record.Time))
}

func (m Model) latestDeploymentRecord(app config.DeploymentApp) (config.DeploymentRecord, bool) {
	records := m.deploymentRecordsForApp(app, 1)
	if len(records) == 0 {
		return config.DeploymentRecord{}, false
	}
	return records[0], true
}

func (m Model) deploymentRecordsForApp(app config.DeploymentApp, limit int) []config.DeploymentRecord {
	category, name := splitDeploymentServer(app.Server)
	records := []config.DeploymentRecord{}
	for _, record := range m.deploymentFile.Records {
		if record.App == app.Name && record.ServerCategory == category && record.ServerName == name {
			records = append(records, record)
			if limit > 0 && len(records) >= limit {
				break
			}
		}
	}
	return records
}

func deploymentRecordActionStatusText(record config.DeploymentRecord) string {
	action := "部署"
	if record.Action == config.DeployActionRollback {
		action = "回滚"
	}
	status := "成功"
	if record.Status == config.DeployStatusFailed {
		status = "失败"
	}
	if record.Status == config.DeployStatusRunning {
		status = "运行中"
	}
	return action + status
}

func splitDeploymentServer(server string) (string, string) {
	server = strings.TrimSpace(server)
	if idx := strings.Index(server, "/"); idx >= 0 {
		return server[:idx], server[idx+1:]
	}
	return "", server
}

func deploymentRecordTimeText(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	now := time.Now()
	if t.After(now) {
		t = now
	}
	d := now.Sub(t)
	if d < time.Minute {
		return "刚刚"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟前", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时前", int(d.Hours()))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%d天前", int(d.Hours()/24))
	}
	if t.Year() == now.Year() {
		return t.Format("01-02")
	}
	return t.Format("2006-01-02")
}

func deploymentRecordDateTimeText(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	return t.Local().Format("01-02 15:04")
}

func (m Model) renderDeploymentEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 42 {
		innerWidth = 42
	}
	help := "切换 Tab  保存 Enter  换行 Ctrl+J  服务器/来源/凭证 ←→  返回 Esc"
	title := "添加部署应用"
	if m.deploymentEditing {
		title = "编辑部署应用"
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if strings.Contains(m.status, "失败") || strings.Contains(m.status, "不能为空") || strings.Contains(m.status, "需要填写") {
			statusStyle = redStyle
		}
		header += "  " + statusStyle.Render(fit(m.status, width-ansi.StringWidth(title)-2))
	}
	contentHeight := m.height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	lines := m.deploymentEditLines(innerWidth, contentHeight)
	if !deploymentFieldIsCommand(m.deploymentField) && len(lines) > contentHeight {
		selected := selectedDeploymentEditRow(m.deploymentField)
		start := selected - contentHeight + 4
		if start < 0 {
			start = 0
		}
		if start+contentHeight > len(lines) {
			start = len(lines) - contentHeight
			if start < 0 {
				start = 0
			}
		}
		lines = lines[start:minInt(len(lines), start+contentHeight)]
	}
	for blockLineCount(strings.Join(lines, "\n")) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{fit(header, width), box, renderHelp(width, help)}, "\n")
}

func (m Model) deploymentEditLines(innerWidth int, contentHeight int) []string {
	if deploymentFieldIsCommand(m.deploymentField) {
		lines := []string{
			deploymentSectionTitle("部署流程"),
			deploymentCommandSummaryLine(m, 13, "更新前", m.deploymentForm.BeforeCommands, innerWidth),
			deploymentCommandSummaryLine(m, 14, "获取资源", m.deploymentForm.ResourceCommands, innerWidth),
			deploymentCommandSummaryLine(m, 15, "更新命令", m.deploymentForm.UpdateCommands, innerWidth),
			deploymentCommandSummaryLine(m, 16, "更新后", m.deploymentForm.AfterCommands, innerWidth),
			deploymentCommandSummaryLine(m, 17, "健康检查", m.deploymentForm.HealthCommands, innerWidth),
			"",
			deploymentSectionTitle("回滚流程"),
			deploymentCommandSummaryLine(m, 18, "回滚命令", m.deploymentForm.RollbackCommands, innerWidth),
			"",
			deploymentSectionTitle(deploymentFieldName(m.deploymentField)),
		}
		textAreaHeight := contentHeight - len(lines) - 2
		if textAreaHeight < 4 {
			textAreaHeight = 4
		}
		lines = append(lines, commandTextArea(m.deploymentValue(), m.deploymentCursor, true, innerWidth, textAreaHeight))
		return lines
	}
	lines := []string{
		deploymentSectionTitle("资源来源"),
		deploymentFieldLine(m, 0, "来源", deploySourceText(m.deploymentForm.Source)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 1, "获取方式", deployFetchModeText(m.deploymentForm.FetchMode)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 2, "服务器", m.deploymentServerText(innerWidth), innerWidth),
		deploymentFieldLine(m, 3, "应用名称", m.deploymentInputText(3, 28), innerWidth),
		deploymentFieldLine(m, 4, "仓库", m.deploymentInputText(4, 34), innerWidth),
	}
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines,
			deploymentFieldLine(m, 6, "版本", m.deploymentInputText(6, 18), innerWidth),
			deploymentFieldLine(m, 7, "资源文件/匹配", m.deploymentInputText(7, 28), innerWidth),
		)
	} else {
		lines = append(lines, deploymentFieldLine(m, 5, "分支", m.deploymentInputText(5, 18), innerWidth))
	}
	lines = append(lines, deploymentFieldLine(m, 8, "项目目录", m.deploymentInputText(8, 34), innerWidth))
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines, deploymentFieldLine(m, 9, "下载地址", m.deploymentInputText(9, 34), innerWidth))
		lines = append(lines, deploymentReleaseHintLines(innerWidth)...)
	}
	lines = append(lines,
		"",
		deploymentSectionTitle("GitHub 凭证"),
		deploymentFieldLine(m, 10, "凭证类型", deployCredentialText(m.deploymentForm.Credential)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 11, "凭证参数", m.deploymentInputText(11, 28), innerWidth),
		"",
		deploymentSectionTitle("串行部署"),
		deploymentFieldLine(m, 12, "等待时间", m.deploymentInputText(12, 12)+"  秒", innerWidth),
		mutedStyle.Render(fit("说明：多选部署时，此应用完成后等待该秒数再执行下一个。", innerWidth)),
		"",
		deploymentSectionTitle("部署流程"),
		deploymentCommandSummaryLine(m, 13, "更新前", m.deploymentForm.BeforeCommands, innerWidth),
		deploymentCommandSummaryLine(m, 14, "获取资源", m.deploymentForm.ResourceCommands, innerWidth),
		deploymentCommandSummaryLine(m, 15, "更新命令", m.deploymentForm.UpdateCommands, innerWidth),
		deploymentCommandSummaryLine(m, 16, "更新后", m.deploymentForm.AfterCommands, innerWidth),
		deploymentCommandSummaryLine(m, 17, "健康检查", m.deploymentForm.HealthCommands, innerWidth),
		"",
		deploymentSectionTitle("回滚流程"),
		deploymentCommandSummaryLine(m, 18, "回滚命令", m.deploymentForm.RollbackCommands, innerWidth),
	)
	return lines
}

func deploymentSectionTitle(value string) string {
	return "  " + sectionTitle(value)
}

func deploymentReleaseHintLines(width int) []string {
	return fitLines([]string{
		mutedStyle.Render("说明：版本留空或 latest 表示最新 Release；填 v1.0.0 表示固定版本。"),
		mutedStyle.Render("说明：资源不带 * 是固定文件名；带 * 会在 Release 资源里自动匹配。"),
		mutedStyle.Render("说明：下载地址可选；填写后优先使用完整下载地址。"),
	}, width)
}

func (m Model) deploymentServerText(width int) string {
	value := deploymentDisplayServerText(m.deploymentForm.Server)
	index := m.deploymentServerIndex(m.deploymentForm.Server)
	if index >= 0 {
		h := m.states[index].Host
		value = deploymentDisplayServerName(h.Category, h.Name)
	} else if strings.TrimSpace(m.deploymentForm.Server) != "" {
		value += "  未找到"
	}
	value += "  ←/→"
	return fit(value, width)
}

func deploymentDisplayServerText(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return "-"
	}
	category := ""
	name := server
	if idx := strings.Index(server, "/"); idx >= 0 {
		category = strings.TrimSpace(server[:idx])
		name = strings.TrimSpace(server[idx+1:])
	}
	return deploymentDisplayServerName(category, name)
}

func deploymentDisplayServerName(category string, name string) string {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "-"
	}
	if category == "" {
		return name
	}
	return "[" + category + "] " + name
}

func (m Model) deploymentInputText(field int, width int) string {
	value := m.deploymentFieldValue(field)
	if value != "" || m.deploymentField == field {
		return commandInputText(value, m.deploymentCursor, m.deploymentField == field, width)
	}
	placeholder := deploymentFieldPlaceholder(field, m.deploymentForm.Source, m.deploymentForm.Credential)
	if placeholder == "" {
		return commandInputText(value, m.deploymentCursor, m.deploymentField == field, width)
	}
	return "[" + fit(placeholder, width) + strings.Repeat(" ", maxInt(0, width-ansi.StringWidth(placeholder))) + "]"
}

func (m Model) deploymentFieldValue(field int) string {
	m.deploymentField = field
	return m.deploymentValue()
}

func deploymentFieldPlaceholder(field int, source string, credential string) string {
	switch field {
	case 3:
		return "例如 api"
	case 4:
		if source == config.DeploySourceRelease {
			return "owner/repo"
		}
		return "git@github.com:owner/repo.git"
	case 5:
		return "main"
	case 6:
		return "latest 或 v1.0.0"
	case 7:
		return "app.tar.gz 或 app-*"
	case 8:
		return "/opt/app"
	case 9:
		return "可选：完整下载地址"
	case 11:
		switch credential {
		case config.DeployCredentialSSH:
			return "本地或目标服务器私钥路径"
		case config.DeployCredentialToken:
			return "本地或目标服务器环境变量名"
		default:
			return "公开仓库或服务器已配置认证"
		}
	case 12:
		return "0"
	default:
		return ""
	}
}

func selectedDeploymentEditRow(field int) int {
	if field <= 12 {
		return field + 1
	}
	return 19 + field - 13
}

func deploymentFieldLine(m Model, index int, label string, value string, width int) string {
	prefix := " "
	style := lipgloss.NewStyle()
	if m.deploymentField == index {
		prefix = "▶"
		style = blueStyle.Bold(true)
	}
	labelWidth := runewidth.StringWidth("资源文件/匹配")
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func deploymentCommandSummaryLine(m Model, index int, label string, value string, width int) string {
	count := len(splitCommandBlock(value))
	summary := fmt.Sprintf("%d条", count)
	if count == 0 {
		summary = "未配置"
	}
	return deploymentFieldLine(m, index, label, summary, width)
}

func deploymentFieldName(field int) string {
	switch field {
	case 13:
		return "更新前命令"
	case 14:
		return "获取资源命令"
	case 15:
		return "更新命令"
	case 16:
		return "更新后命令"
	case 17:
		return "健康检查命令"
	case 18:
		return "回滚命令"
	default:
		return "命令"
	}
}

func (m Model) renderDeploymentConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	hostName := m.deploymentConfirmServerName()
	lines := m.deploymentConfirmLines(hostName, bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentConfirmMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.deploymentConfirmBorderColor()).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit("确认部署", width)), box, renderHelp(width, m.deploymentConfirmHelp())}, "\n")
}

func (m Model) deploymentConfirmBorderColor() lipgloss.Color {
	if m.activeDeployment.Running {
		return blue
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.QueueFailed >= 0 && m.activeDeployment.ExitCode != 0 {
		return red
	}
	return softGray
}

func (m Model) deploymentConfirmLines(hostName string, bodyWidth int) []string {
	app := m.deploymentConfirm
	queue := m.deploymentConfirmQueue
	if len(m.activeDeployment.Queue) > 0 {
		queue = m.activeDeployment.Queue
	}
	if len(queue) == 0 {
		queue = []config.DeploymentApp{app}
	}
	return m.deploymentQueueConfirmLines(queue, bodyWidth)
}

func (m Model) deploymentConfirmHelp() string {
	if m.activeDeployment.Running {
		return "滚动 ↑↓/jk  执行中"
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.QueueFailed >= 0 && m.activeDeployment.ExitCode != 0 {
		return "滚动 ↑↓/jk  重试失败 r  重新部署 a  返回 q/Esc"
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.Output != "" {
		return "滚动 ↑↓/jk  重新部署 a  返回 q/Esc"
	}
	return "滚动 ↑↓/jk  开始 Enter  重试失败 r  重新部署 a  返回 q/Esc"
}

func (m Model) deploymentQueueConfirmLines(queue []config.DeploymentApp, bodyWidth int) []string {
	current := m.deploymentQueueCurrentApp(queue)
	lines := []string{}
	if len(queue) == 1 {
		lines = append(lines, detailSubTitle("部署信息"))
		lines = append(lines, deploymentInfoLines(current, bodyWidth)...)
	} else {
		lines = append(lines,
			detailSubTitle("部署队列"),
			mutedStyle.Render(fit("按下面顺序串行执行；每个应用完成后按自己的等待时间进入下一个。", bodyWidth)),
			"",
		)
		for i, app := range queue {
			lines = append(lines, deploymentQueueLine(m.activeDeployment, i, app, bodyWidth))
		}
	}
	lines = append(lines, "", detailSubTitle("当前流程"), fit(deploymentQueueFlowText(current), bodyWidth))
	if len(m.activeDeployment.Queue) > 0 {
		lines = append(lines, "", detailSubTitle("执行输出"))
		lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
		if !m.activeDeployment.Running && m.activeDeployment.Output != "" {
			lines = append(lines, "", fmt.Sprintf("退出码 %d", m.activeDeployment.ExitCode))
		}
	}
	if len(queue) == 1 {
		records := m.deploymentRecordsForApp(current, 50)
		lines = append(lines, "", detailSubTitle(fmt.Sprintf("历史 %d条", len(records))))
		if len(records) == 0 {
			lines = append(lines, mutedStyle.Render("暂无记录"))
		} else {
			for _, record := range records {
				lines = append(lines, deploymentDetailHistoryLine(record, bodyWidth))
			}
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func deploymentQueueLine(active activeDeployment, index int, app config.DeploymentApp, width int) string {
	icon := deploymentQueueStatusStyle(active, index).Render(deploymentQueueStatusIcon(active, index))
	seq := cardMutedStyle.Render(fmt.Sprintf("%02d", index+1))
	name := deploymentQueueNameStyle(active, index).Render(padVisible(emptyDash(app.Name), 14))
	server := cardMutedStyle.Render(padVisible(emptyDash(app.Server), 18))
	source := detailValueStyle.Render(padVisible(deploySourceText(app.Source), 7))
	target := cardMutedStyle.Render(padVisible(deploymentAppTarget(app), 10))
	wait := cardMutedStyle.Render(fmt.Sprintf("等待 %d秒", maxInt(0, app.WaitSeconds)))
	return fitANSI(icon+" "+strings.Join([]string{seq, name, server, source, target, wait}, "  "), width)
}

func deploymentQueueNameStyle(active activeDeployment, index int) lipgloss.Style {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return blueStyle.Bold(true)
	case "done":
		return greenStyle
	case "failed":
		return redStyle
	default:
		return detailValueStyle
	}
}

func deploymentAppTarget(app config.DeploymentApp) string {
	target := app.Branch
	if app.Source == config.DeploySourceRelease {
		target = app.Version
	}
	if strings.TrimSpace(target) == "" {
		return "-"
	}
	return target
}

func deploymentInfoLines(app config.DeploymentApp, bodyWidth int) []string {
	return []string{
		deploymentDetailRow("应用", emptyDash(app.Name), bodyWidth),
		deploymentDetailRow("服务器", deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow("来源", deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow("仓库", emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow("目录", emptyDash(app.Path), bodyWidth),
		deploymentDetailRow("凭证", deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow("凭证参数", emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow("收藏", yesNo(app.Favorite), bodyWidth),
		deploymentDetailRow("置顶", yesNo(app.Pinned), bodyWidth),
	}
}

func (m Model) deploymentQueueCurrentApp(queue []config.DeploymentApp) config.DeploymentApp {
	if len(m.activeDeployment.Queue) > 0 {
		index := clampInt(m.activeDeployment.QueueIndex, 0, len(m.activeDeployment.Queue)-1)
		return m.activeDeployment.Queue[index]
	}
	if len(queue) > 0 {
		return queue[0]
	}
	return config.DeploymentApp{}
}

func deploymentQueueStatusIcon(active activeDeployment, index int) string {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return "▶"
	case "done":
		return "✓"
	case "failed":
		return "✕"
	default:
		return "·"
	}
}

func deploymentQueueStatusStyle(active activeDeployment, index int) lipgloss.Style {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return blueStyle.Bold(true)
	case "done":
		return greenStyle
	case "failed":
		return redStyle
	default:
		return lipgloss.NewStyle()
	}
}

func deploymentQueueItemStatus(active activeDeployment, index int) string {
	if len(active.Queue) == 0 {
		return "pending"
	}
	if active.QueueFailed == index && active.ExitCode != 0 {
		return "failed"
	}
	if active.Running && active.QueueIndex == index {
		return "running"
	}
	if active.QueueFailed >= 0 && active.ExitCode != 0 {
		if index < active.QueueFailed {
			return "done"
		}
		return "pending"
	}
	if index < active.QueueIndex {
		return "done"
	}
	if !active.Running && active.QueueIndex == len(active.Queue)-1 && index == active.QueueIndex && active.Output != "" && active.ExitCode == 0 {
		return "done"
	}
	return "pending"
}

func deploymentQueueFlowText(app config.DeploymentApp) string {
	parts := []string{}
	if len(app.BeforeCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新前 %d步", len(app.BeforeCommands)))
	}
	parts = append(parts, fmt.Sprintf("获取资源 %d步", len(app.ResourceCommands)))
	if len(app.UpdateCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新 %d步", len(app.UpdateCommands)))
	}
	if len(app.AfterCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新后 %d步", len(app.AfterCommands)))
	}
	if len(app.HealthCommands) > 0 {
		parts = append(parts, fmt.Sprintf("健康检查 %d步", len(app.HealthCommands)))
	}
	return strings.Join(parts, "  ")
}

func deploymentHistoryLine(record config.DeploymentRecord, width int) string {
	version := deploymentRecordVersionText(record)
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = fmt.Sprintf("  退出码 %d", record.ExitCode)
	}
	line := fmt.Sprintf("%s  %s  %s%s",
		padVisible(deploymentRecordDateTimeText(record.Time), 11),
		padVisible(deploymentRecordActionStatusText(record), 8),
		version,
		exit,
	)
	return fit(line, width)
}

func deploymentRecordVersionText(record config.DeploymentRecord) string {
	previous := strings.TrimSpace(record.PreviousVersion)
	current := strings.TrimSpace(record.CurrentVersion)
	if previous == "" {
		previous = "-"
	}
	if current == "" {
		current = "-"
	}
	return previous + " -> " + current
}

func appendWrappedCommandPreview(lines []string, command string, width int) []string {
	wrapped := strings.Split(wrapPlainLine("$ "+command, width), "\n")
	return append(lines, wrapped...)
}

func deploymentStepCountLine(label string, count int, width int) string {
	return fit(fmt.Sprintf("%s  %d步", padVisible(label, 10), count), width)
}

func (m Model) renderDeploymentRollbackConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := m.deploymentRollbackConfirmLines(bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentRollbackConfirmMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit("确认回滚", width)), box, renderHelp(width, "滚动 ↑↓/jk  执行 Enter  返回 Esc")}, "\n")
}

func (m Model) deploymentRollbackConfirmLines(bodyWidth int) []string {
	app := m.activeDeployment.App
	lines := []string{
		modalLine("服务器", m.activeDeploymentServerName(), bodyWidth),
		modalLine("应用", app.Name, bodyWidth),
		modalLine("上一版本", emptyDash(m.activeDeployment.PreviousVersion), bodyWidth),
		modalLine("当前版本", emptyDash(m.activeDeployment.CurrentVersion), bodyWidth),
		"",
		detailSubTitle("回滚命令"),
	}
	for _, command := range app.RollbackCommands {
		lines = appendWrappedCommandPreview(lines, command, bodyWidth)
	}
	return lines
}

func (m Model) deploymentRollbackConfirmMaxScroll() int {
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := len(m.deploymentRollbackConfirmLines(bodyWidth))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) activeDeploymentServerName() string {
	if m.activeDeployment.HostIndex >= 0 && m.activeDeployment.HostIndex < len(m.states) {
		return hostDisplayName(m.states[m.activeDeployment.HostIndex].Host)
	}
	return emptyDash(m.activeDeployment.App.Server)
}

func (m Model) deploymentConfirmMaxScroll() int {
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	hostName := m.deploymentConfirmServerName()
	lines := len(m.deploymentConfirmLines(hostName, bodyWidth))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) deploymentConfirmServerName() string {
	index := m.deploymentServerIndex(m.deploymentConfirm.Server)
	if index >= 0 && index < len(m.states) {
		return hostDisplayName(m.states[index].Host)
	}
	return emptyDash(m.deploymentConfirm.Server)
}

func (m Model) renderDeploymentOutput() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "滚动 ↑↓/jk  回滚 r  返回 q/Esc"
	title := "部署输出  " + m.activeDeployment.App.Name
	lines := []string{
		modalLine("应用", m.activeDeployment.App.Name, bodyWidth),
		modalLine("来源", deploySourceText(m.activeDeployment.App.Source), bodyWidth),
		modalLine("队列", deploymentQueueProgressText(m.activeDeployment), bodyWidth),
		modalLine("上一版本", emptyDash(m.activeDeployment.PreviousVersion), bodyWidth),
		modalLine("当前版本", emptyDash(m.activeDeployment.CurrentVersion), bodyWidth),
		"",
	}
	lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
	if !m.activeDeployment.Running {
		lines = append(lines, "", fmt.Sprintf("退出码 %d", m.activeDeployment.ExitCode))
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentOutputMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) deploymentOutputMaxScroll() int {
	lines := 6
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines += len(m.deploymentOutputContentLines(bodyWidth))
	if !m.activeDeployment.Running {
		lines += 2
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) deploymentOutputContentLines(width int) []string {
	stages := deploymentExecutionStages(m.activeDeployment.App, m.activeDeployment.Action)
	output := strings.TrimRight(m.activeDeployment.Output, "\n")
	sections, loose, lastStage := deploymentOutputSections(output)
	if len(stages) == 0 {
		if output == "" {
			return []string{mutedStyle.Render("(无输出)")}
		}
		return deploymentOutputLines(output, width)
	}
	currentIndex := 0
	if lastStage != "" {
		for i, stage := range stages {
			if stage == lastStage {
				currentIndex = i
				break
			}
		}
	}
	lines := []string{}
	if len(loose) > 0 {
		lines = append(lines, detailSubTitle("输出"))
		for _, line := range loose {
			lines = append(lines, fit(line, width))
		}
		lines = append(lines, "")
	}
	for i, stage := range stages {
		status := "pending"
		if m.activeDeployment.Running {
			if i < currentIndex {
				status = "done"
			} else if i == currentIndex {
				status = "running"
			}
		} else {
			if m.activeDeployment.ExitCode != 0 {
				if i < currentIndex {
					status = "done"
				} else if i == currentIndex {
					status = "failed"
				}
			} else {
				status = "done"
			}
		}
		lines = append(lines, deploymentOutputStageLine(stage, status, width))
		stageLines := sections[stage]
		if len(stageLines) == 0 && status == "running" {
			lines = append(lines, mutedStyle.Render("  正在执行..."))
		}
		for _, line := range stageLines {
			lines = append(lines, fit("  "+line, width))
		}
	}
	return lines
}

func deploymentExecutionStages(app config.DeploymentApp, action string) []string {
	if action == config.DeployActionRollback {
		if len(app.RollbackCommands) == 0 {
			return nil
		}
		return []string{"回滚"}
	}
	stages := []string{}
	if len(app.BeforeCommands) > 0 {
		stages = append(stages, "更新前")
	}
	stages = append(stages, "获取资源")
	if app.FetchMode == config.DeployFetchLocal {
		stages = append(stages, "上传资源")
	}
	if len(app.UpdateCommands) > 0 {
		stages = append(stages, "更新")
	}
	if len(app.AfterCommands) > 0 {
		stages = append(stages, "更新后")
	}
	if len(app.HealthCommands) > 0 {
		stages = append(stages, "健康检查")
	}
	return stages
}

func deploymentOutputStageLine(stage string, status string, width int) string {
	switch status {
	case "running":
		return blueStyle.Bold(true).Render(fit("▶ "+stage, width))
	case "done":
		return greenStyle.Render(fit("✓ "+stage, width))
	case "failed":
		return redStyle.Render(fit("✕ "+stage, width))
	default:
		return mutedStyle.Render(fit("· "+stage, width))
	}
}

func deploymentQueueProgressText(active activeDeployment) string {
	if len(active.Queue) <= 1 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", active.QueueIndex+1, len(active.Queue))
}

func deploymentOutputSections(output string) (map[string][]string, []string, string) {
	sections := map[string][]string{}
	loose := []string{}
	current := ""
	for _, line := range strings.Split(output, "\n") {
		if isDeploymentVersionMarker(line) {
			continue
		}
		if title, ok := deploymentOutputStageTitle(line); ok {
			current = title
			if _, exists := sections[current]; !exists {
				sections[current] = []string{}
			}
			continue
		}
		if current == "" {
			if strings.TrimSpace(line) != "" {
				loose = append(loose, line)
			}
			continue
		}
		sections[current] = append(sections[current], line)
	}
	return sections, loose, current
}

func deploymentOutputLines(output string, width int) []string {
	rawLines := strings.Split(output, "\n")
	lines := []string{}
	for _, line := range rawLines {
		if isDeploymentVersionMarker(line) {
			continue
		}
		if title, ok := deploymentOutputStageTitle(line); ok {
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				lines = append(lines, "")
			}
			lines = append(lines, detailSubTitle(title))
			continue
		}
		lines = append(lines, fit(line, width))
	}
	return lines
}

func isDeploymentVersionMarker(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "SSHM_PREVIOUS_VERSION=") || strings.HasPrefix(line, "SSHM_CURRENT_VERSION=")
}

func deploymentOutputStageTitle(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "== ") || !strings.HasSuffix(line, " ==") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "== "), " =="))
	if title == "" {
		return "", false
	}
	return title, true
}

func deploySourceText(source string) string {
	if source == config.DeploySourceRelease {
		return "Release"
	}
	return "Git"
}

func deployFetchModeText(value string) string {
	if value == config.DeployFetchRemote {
		return "服务器拉取"
	}
	return "本地拉取后上传"
}

func deployCredentialText(value string) string {
	switch value {
	case config.DeployCredentialSSH:
		return "SSH Key"
	case config.DeployCredentialToken:
		return "Token"
	default:
		return "不配置"
	}
}

func (m Model) renderCommandList() string {
	width := detailFrameWidth(m.width)
	hostName := "-"
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		hostName = hostDisplayName(m.states[m.activeCommand.HostIndex].Host)
	}
	title := "命令模板  " + hostName
	bodyWidth := width - 4
	if bodyWidth < 30 {
		bodyWidth = 30
	}
	help := "选择 ↑↓/jk  执行 Enter  新增 a  编辑 e  删除 x  返回 Esc"
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	contentHeight := bodyHeight - 2
	if contentHeight < 4 {
		contentHeight = 4
	}
	lines := []string{}
	listHeight := contentHeight
	if listHeight < 1 {
		listHeight = 1
	}
	if len(m.commandItems) == 0 {
		lines = append(lines, mutedStyle.Render("没有命令模板"))
	} else {
		start, end := visibleRange(len(m.commandItems), m.commandIndex, listHeight)
		for i := start; i < end; i++ {
			item := m.commandItems[i]
			if item.Header {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, detailSubTitle(item.Name))
				continue
			}
			if item.Spacer {
				lines = append(lines, "")
				continue
			}
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.commandIndex {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			label := item.Name
			if item.Temporary {
				label = "+ " + label
			}
			lines = append(lines, style.Render(fit(prefix+" "+label, bodyWidth)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) renderCommandEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 36 {
		innerWidth = 36
	}
	title := "添加命令模板"
	if m.commandEditing {
		title = "编辑命令模板"
	}
	scope := "全局  ←/→"
	server := "-"
	if m.activeCommand.HostIndex >= 0 && m.activeCommand.HostIndex < len(m.states) {
		h := m.states[m.activeCommand.HostIndex].Host
		server = config.ServerCommandKey(h.Category, h.Name)
	}
	if m.commandForm.Scope == commandScopeServer {
		scope = server + "  ←/→"
	}
	header := title
	if m.commandForm.Scope == commandScopeServer && server != "-" {
		header += "  " + server
	}
	lines := []string{}
	lines = append(lines, commandFieldLine(m, 0, "范围", scope, innerWidth))
	lines = append(lines, commandFieldLine(m, 1, "模板名称", commandInputText(m.commandForm.Name, m.commandCursor, m.commandField == 1, 28), innerWidth))
	lines = append(lines, "")
	help := "切换 Tab  保存 Enter  换行 Ctrl+J  返回 Esc"
	lines = append(lines, detailSubTitle("命令内容"))
	lines = append(lines, commandTextArea(m.commandForm.Command, m.commandCursor, m.commandField == 2, innerWidth, m.commandTextAreaHeight(help)))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func commandFieldLine(m Model, index int, label string, value string, width int) string {
	prefix := " "
	style := lipgloss.NewStyle()
	if m.commandField == index {
		prefix = "▶"
		style = blueStyle.Bold(true)
	}
	labelWidth := runewidth.StringWidth("模板名称")
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func commandInputText(value string, cursor int, active bool, width int) string {
	if width < 8 {
		width = 8
	}
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if active {
		return "[" + formInputText(value, width, cursor) + "]"
	}
	fitted := padVisible(value, width)
	return "[" + fitted + strings.Repeat(" ", maxInt(0, width-runewidth.StringWidth(fitted))) + "]"
}

func (m Model) commandTextAreaHeight(help string) int {
	contentLinesBeforeTextArea := 4
	textareaBorderLines := 2
	formBorderLines := 2
	externalHeaderLines := 1
	height := m.height - externalHeaderLines - contentLinesBeforeTextArea - textareaBorderLines - formBorderLines - 1
	if height < 6 {
		height = 6
	}
	return height
}

func commandTextArea(value string, cursor int, active bool, width int, height int) string {
	bodyWidth := width - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	if height < 4 {
		height = 4
	}
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	cursorLine := 0
	cursorCol := 0
	if active {
		cursorLine, cursorCol = cursorTextPosition(runes, cursor)
	}
	lines := strings.Split(value, "\n")
	start := 0
	if len(lines) > height {
		start = cursorLine - height + 1
		if start < 0 {
			start = 0
		}
		if start+height > len(lines) {
			start = len(lines) - height
		}
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	viewLines := make([]string, 0, height)
	for i := start; i < end; i++ {
		if active && i == cursorLine {
			viewLines = append(viewLines, formInputText(lines[i], bodyWidth, cursorCol))
			continue
		}
		viewLines = append(viewLines, fit(lines[i], bodyWidth))
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(viewLines, "\n"))
}

func cursorTextPosition(runes []rune, cursor int) (int, int) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	line := 0
	col := 0
	for i := 0; i < cursor; i++ {
		if runes[i] == '\n' {
			line++
			col = 0
			continue
		}
		col++
	}
	return line, col
}

func (m Model) renderCommandConfirm() string {
	width := detailFrameWidth(m.width)
	help := "滚动 ↑↓/jk  执行 Enter  返回 Esc"
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	h := m.states[m.activeCommand.HostIndex].Host
	lines := []string{
		modalLine("服务器", hostDisplayName(h), width-4),
		modalLine("模板", m.commandConfirm.Name, width-4),
		"",
		detailSubTitle("命令"),
	}
	lines = append(lines, strings.Split(wrapPlainLine(m.commandConfirm.Command, width-4), "\n")...)
	maxScroll := m.commandConfirmMaxScroll()
	scroll := clampInt(m.commandOutputScroll, 0, maxScroll)
	viewLines := lines
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(yellow).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(viewLines, width-4), "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit("即将执行", width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) commandConfirmMaxScroll() int {
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	lines := []string{
		"",
		"",
		"",
		"命令",
	}
	lines = append(lines, strings.Split(wrapPlainLine(m.commandConfirm.Command, detailFrameWidth(m.width)-4), "\n")...)
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func modalLine(label string, value string, width int) string {
	labelWidth := 8
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+value, width)
}

func (m Model) renderCommandOutput() string {
	width := detailFrameWidth(m.width)
	help := "滚动 ↑↓/jk  返回 q/Esc"
	height := m.height - 4
	if height < 6 {
		height = 6
	}
	title := "命令输出  " + m.activeCommand.Name
	lines := []string{"$ " + m.activeCommand.Command, ""}
	if m.activeCommand.Running {
		lines = append(lines, "正在执行...")
	} else {
		output := strings.TrimRight(m.activeCommand.Output, "\n")
		if output == "" {
			output = "(无输出)"
		}
		lines = append(lines, strings.Split(output, "\n")...)
		lines = append(lines, "", fmt.Sprintf("退出码 %d", m.activeCommand.ExitCode))
	}
	viewLines := lines
	maxScroll := m.commandOutputMaxScroll()
	scroll := clampInt(m.commandOutputScroll, 0, maxScroll)
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(viewLines, width-4), "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(title, width)),
		box,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) renderBatchSelect() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "移动 ↑↓/jk  选择 Space  全选 a  清空 x  下一步 Enter  返回 Esc"
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	contentHeight := bodyHeight - 2
	if contentHeight < 3 {
		contentHeight = 3
	}
	lines := []string{}
	if len(m.batchIndexes) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的服务器"))
	} else {
		start, end := visibleRange(len(m.batchIndexes), m.batchCursor, contentHeight)
		for i := start; i < end; i++ {
			index := m.batchIndexes[i]
			h := m.states[index].Host
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.batchCursor {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			mark := "[ ]"
			if m.batchSelected[index] {
				mark = "[x]"
			}
			lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s %s", prefix, mark, hostDisplayName(h)), bodyWidth)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("批量选择服务器  已选%d台", m.batchSelectedCount())
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) renderBatchCommandList() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "移动 ↑↓/jk  选择 Enter  返回 Esc"
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	targets := m.batchTargetsHeader(width)
	targetLines := strings.Count(targets, "\n") + 1
	contentHeight := bodyHeight - 2 - targetLines
	if contentHeight < 3 {
		contentHeight = 3
	}
	lines := []string{}
	start, end := visibleRange(len(m.batchCommandItems), m.batchCommandIndex, contentHeight)
	for i := start; i < end; i++ {
		item := m.batchCommandItems[i]
		if item.Header {
			lines = append(lines, detailSubTitle(item.Name))
			continue
		}
		if item.Spacer {
			lines = append(lines, "")
			continue
		}
		prefix := " "
		style := lipgloss.NewStyle()
		if i == m.batchCommandIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		label := item.Name
		if item.Temporary {
			label = "+ " + label
		}
		lines = append(lines, style.Render(fit(prefix+" "+label, bodyWidth)))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("选择批量命令  %d台服务器", m.batchSelectedCount())
	return strings.Join([]string{titleStyle.Render(fit(title, width)), targets, box, renderHelp(width, help)}, "\n")
}

func (m Model) renderBatchCommandEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 36 {
		innerWidth = 36
	}
	help := "保存 Enter  换行 Ctrl+J  返回 Esc"
	targets := m.batchTargetsHeader(width)
	targetLines := strings.Count(targets, "\n") + 1
	lines := []string{detailSubTitle("命令内容")}
	lines = append(lines, commandTextArea(m.commandForm.Command, m.commandCursor, true, innerWidth, m.batchCommandTextAreaHeight(targetLines)))
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(blue).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{titleStyle.Render(fit("批量临时命令", width)), targets, box, renderHelp(width, help)}, "\n")
}

func (m Model) batchCommandTextAreaHeight(targetLines int) int {
	height := m.height - targetLines - 7
	if height < 4 {
		height = 4
	}
	return height
}

func (m Model) batchTargetsHeader(width int) string {
	names := make([]string, 0, m.batchSelectedCount())
	for _, index := range m.selectedBatchHostIndexes() {
		if index >= 0 && index < len(m.states) {
			names = append(names, hostDisplayName(m.states[index].Host))
		}
	}
	if len(names) == 0 {
		return mutedStyle.Render("目标 -")
	}
	return mutedStyle.Render(wrapPlainLine("目标 "+strings.Join(names, "、"), width))
}

func (m Model) renderBatchConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{
		modalLine("服务器", fmt.Sprintf("%d台", m.batchSelectedCount()), bodyWidth),
		modalLine("模板", m.batchCommand.Name, bodyWidth),
		"",
		detailSubTitle("目标"),
	}
	for _, index := range m.selectedBatchHostIndexes() {
		lines = append(lines, fit("- "+hostDisplayName(m.states[index].Host), bodyWidth))
	}
	lines = append(lines, "", detailSubTitle("命令"))
	lines = append(lines, strings.Split(wrapPlainLine(m.batchCommand.Command, bodyWidth), "\n")...)
	scroll := clampInt(m.batchOutputScroll, 0, m.batchConfirmMaxScroll())
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(yellow).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	return strings.Join([]string{titleStyle.Render(fit("确认批量执行", width)), box, renderHelp(width, "滚动 ↑↓/jk  确认 Enter  返回 Esc")}, "\n")
}

func (m Model) batchConfirmMaxScroll() int {
	lines := 5 + len(m.selectedBatchHostIndexes()) + len(wrapDetailValue(m.batchCommand.Command, detailFrameWidth(m.width)-4))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) renderBatchOutput() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	leftWidth := bodyWidth / 2
	if leftWidth < 28 {
		leftWidth = 28
	}
	rightWidth := bodyWidth - leftWidth - 2
	if rightWidth < 24 {
		rightWidth = 24
	}
	left := m.batchResultList(leftWidth)
	right := m.batchOutputView(rightWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(body)
	title := fmt.Sprintf("批量执行结果  成功%d  失败%d", m.batchSuccessCount(), m.batchFailCount())
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, "选择 ↑↓/jk  输出 ←→/hl  重试失败 r  返回 q/Esc")}, "\n")
}

func (m Model) batchResultList(width int) string {
	lines := make([]string, 0, len(m.batchJobs)+4)
	displayIndexes := m.batchResultDisplayIndexes()
	lastGroup := ""
	for _, i := range displayIndexes {
		if i < 0 || i >= len(m.batchJobs) {
			continue
		}
		job := m.batchJobs[i]
		group := batchJobGroup(job)
		if group != lastGroup {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, batchJobGroupTitle(group))
			lastGroup = group
		}
		prefix := " "
		style := lipgloss.NewStyle()
		if i == m.batchOutputIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		state := "等待"
		if job.Running {
			state = "执行中"
		} else if job.Done && job.Err == nil {
			state = greenStyle.Render("成功")
		} else if job.Done && job.Err != nil {
			state = redStyle.Render("失败")
		}
		name := "-"
		if job.HostIndex >= 0 && job.HostIndex < len(m.states) {
			name = hostDisplayName(m.states[job.HostIndex].Host)
		}
		lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s  %s", prefix, state, name), width)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) batchResultDisplayIndexes() []int {
	groups := []string{"failed", "running", "waiting", "success"}
	indexes := make([]int, 0, len(m.batchJobs))
	for _, group := range groups {
		for i, job := range m.batchJobs {
			if batchJobGroup(job) == group {
				indexes = append(indexes, i)
			}
		}
	}
	return indexes
}

func batchJobGroup(job batchJob) string {
	switch {
	case job.Done && job.Err != nil:
		return "failed"
	case job.Running:
		return "running"
	case !job.Done:
		return "waiting"
	default:
		return "success"
	}
}

func batchJobGroupTitle(group string) string {
	switch group {
	case "failed":
		return detailDangerSubTitle("失败")
	case "running":
		return detailSubTitle("执行中")
	case "waiting":
		return detailSubTitle("等待")
	default:
		return detailSuccessSubTitle("成功")
	}
}

func (m Model) batchOutputView(width int) string {
	if len(m.batchJobs) == 0 || m.batchOutputIndex < 0 || m.batchOutputIndex >= len(m.batchJobs) {
		return ""
	}
	job := m.batchJobs[m.batchOutputIndex]
	lines := []string{}
	if job.Running {
		lines = append(lines, "执行中...")
	} else if !job.Done {
		lines = append(lines, "等待执行")
	} else {
		output := strings.TrimRight(job.Output, "\n")
		if output == "" {
			output = "(无输出)"
		}
		lines = append(lines, strings.Split(output, "\n")...)
		lines = append(lines, "", fmt.Sprintf("退出码 %d", job.ExitCode))
	}
	scroll := clampInt(m.batchOutputScroll, 0, m.batchOutputMaxScroll())
	height := m.height - 6
	if height < 6 {
		height = 6
	}
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	return strings.Join(fitLines(lines, width), "\n")
}

func (m Model) batchOutputMaxScroll() int {
	if len(m.batchJobs) == 0 || m.batchOutputIndex < 0 || m.batchOutputIndex >= len(m.batchJobs) {
		return 0
	}
	job := m.batchJobs[m.batchOutputIndex]
	lines := 1
	if job.Done {
		if output := strings.TrimRight(job.Output, "\n"); output != "" {
			lines = len(strings.Split(output, "\n")) + 2
		} else {
			lines = 3
		}
	}
	height := m.height - 6
	if height < 6 {
		height = 6
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) renderCommandHistory() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "移动 ↑↓/jk  查看 Enter  搜索 /  重跑 r  删除 x  返回 q/Esc"
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	lines := []string{}
	entries := m.filteredHistoryEntries()
	if len(entries) == 0 {
		lines = append(lines, mutedStyle.Render("暂无命令历史"))
	} else {
		start, end := visibleRange(len(entries), m.historyIndex, height)
		for i := start; i < end; i++ {
			entry := entries[i]
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.historyIndex {
				prefix = "▶"
				style = blueStyle.Bold(true)
			}
			status := historyStatusText(entry.Status)
			line := fmt.Sprintf("%s %s  %s  %s  %s", prefix, historyTimeShort(entry.Time), status, historyTargetsText(entry, 1), historyCommandName(entry))
			lines = append(lines, style.Render(fit(line, bodyWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(lines, "\n"))
	title := fmt.Sprintf("命令历史  %d条", len(entries))
	if m.historySearch {
		title += "  搜索：" + inlineCursorText(m.historyQuery, width/3, len([]rune(m.historyQuery)))
	} else if strings.TrimSpace(m.historyQuery) != "" {
		title += "  搜索：" + m.historyQuery
	}
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) renderCommandHistoryDetail() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return "没有命令历史"
	}
	lines := []string{
		modalLine("时间", historyTimeFull(entry.Time), bodyWidth),
		modalLine("状态", historyStatusPlain(entry.Status), bodyWidth),
		modalLine("类型", historyKindText(entry), bodyWidth),
		modalLine("名称", historyCommandName(entry), bodyWidth),
		"",
		detailSubTitle("目标"),
	}
	for _, target := range entry.Targets {
		state := historyStatusPlain(target.Status)
		targetText := fmt.Sprintf("%s  %s  退出码%d", historyTargetName(target), state, target.ExitCode)
		lines = append(lines, fit(targetText, bodyWidth))
	}
	lines = append(lines, "", detailSubTitle("命令"))
	lines = append(lines, strings.Split(wrapPlainLine(entry.Command, bodyWidth), "\n")...)
	lines = append(lines, "", detailSubTitle("输出"))
	for _, target := range entry.Targets {
		lines = append(lines, fit("["+historyTargetName(target)+"]", bodyWidth))
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			output = "(无输出)"
		}
		lines = append(lines, strings.Split(wrapPlainLine(output, bodyWidth), "\n")...)
		lines = append(lines, "")
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.historyScroll, 0, m.commandHistoryDetailMaxScroll())
	viewLines := lines
	if len(lines) > height {
		viewLines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(viewLines) < height {
		viewLines = append(viewLines, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width).Render(strings.Join(fitLines(viewLines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit("命令历史详情", width)), box, renderHelp(width, "滚动 ↑↓/jk  重跑 r  删除 x  返回 q/Esc")}, "\n")
}

func (m Model) commandHistoryDetailMaxScroll() int {
	entry, ok := m.selectedHistoryEntry()
	if !ok {
		return 0
	}
	bodyWidth := detailFrameWidth(m.width) - 4
	lines := 9 + len(entry.Targets)*3 + len(wrapDetailValue(entry.Command, bodyWidth))
	for _, target := range entry.Targets {
		output := strings.TrimRight(target.Output, "\n")
		if output == "" {
			lines++
		} else {
			lines += len(wrapDetailValue(output, bodyWidth))
		}
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	maxScroll := lines - height
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func historyCommandName(entry config.CommandHistoryEntry) string {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		return "临时命令"
	}
	return name
}

func historyKindText(entry config.CommandHistoryEntry) string {
	if entry.Kind == "batch" {
		return fmt.Sprintf("批量命令 %d台", len(entry.Targets))
	}
	return "单台命令"
}

func historyStatusText(status string) string {
	if status == "failed" {
		return redStyle.Render("失败")
	}
	return greenStyle.Render("成功")
}

func historyStatusPlain(status string) string {
	if status == "failed" {
		return "失败"
	}
	return "成功"
}

func historyTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "--"
	}
	return t.Local().Format("01-02 15:04")
}

func historyTimeFull(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return value
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func historyTargetsText(entry config.CommandHistoryEntry, limit int) string {
	if len(entry.Targets) == 0 {
		return "-"
	}
	names := make([]string, 0, len(entry.Targets))
	for _, target := range entry.Targets {
		names = append(names, historyTargetName(target))
	}
	if limit > 0 && len(names) > limit {
		return fmt.Sprintf("%s 等%d台", names[0], len(names))
	}
	return strings.Join(names, "、")
}

func historyTargetName(target config.CommandHistoryTarget) string {
	category := strings.TrimSpace(target.Category)
	name := strings.TrimSpace(target.Name)
	if category == "" {
		return name
	}
	return "[" + category + "] " + name
}

func (m Model) renderHelpPanel() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	rows := []struct {
		key  string
		desc string
	}{
		{"↑↓←→ / hjkl", "移动选择"},
		{"Enter", "登录服务器"},
		{"Space", "查看详情"},
		{"m", "命令模板"},
		{"b", "批量命令"},
		{"i", "命令历史"},
		{"y", "传输任务"},
		{"g", "应用部署"},
		{"w", "异常总览"},
		{"z", "切换首页视图"},
		{"t", "置顶 / 取消置顶"},
		{"f", "收藏 / 取消收藏"},
		{"v", "只看收藏 / 取消筛选"},
		{"a", "添加服务器"},
		{"c", "复制服务器"},
		{"e", "编辑服务器"},
		{"x", "删除服务器"},
		{"u", "上传文件或目录"},
		{"d", "下载文件或目录"},
		{"r", "刷新监控"},
		{"/", "搜索"},
		{"Tab", "切换分类"},
		{"o", "只看在线 / 取消筛选"},
		{"p", "只看异常 / 取消筛选"},
		{"s", "切换排序"},
		{"q / Esc", "退出或返回"},
		{"?", "关闭帮助"},
	}
	lines := []string{}
	for _, row := range rows {
		lines = append(lines, modalLine(row.key, row.desc, bodyWidth))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit("快捷键", width)),
		box,
		renderHelp(width, "返回 q/Esc/?"),
	}, "\n")
}

type anomalyItem struct {
	Index  int
	Checks []checkItem
}

func (m Model) anomalyItems() []anomalyItem {
	items := make([]anomalyItem, 0)
	for i, state := range m.states {
		checks := actionableChecks(buildChecks(state))
		if len(checks) == 0 {
			continue
		}
		if !m.anomalyMatchesFilter(checks) {
			continue
		}
		items = append(items, anomalyItem{Index: i, Checks: checks})
	}
	sort.SliceStable(items, func(i, j int) bool {
		aSevere, aWarn, aTip := checkCounts(items[i].Checks)
		bSevere, bWarn, bTip := checkCounts(items[j].Checks)
		if aSevere != bSevere {
			return aSevere > bSevere
		}
		if aWarn != bWarn {
			return aWarn > bWarn
		}
		if aTip != bTip {
			return aTip > bTip
		}
		aHost := m.states[items[i].Index].Host
		bHost := m.states[items[j].Index].Host
		if aHost.Category == bHost.Category {
			return aHost.Name < bHost.Name
		}
		return aHost.Category < bHost.Category
	})
	return items
}

func (m Model) anomalyMatchesFilter(checks []checkItem) bool {
	switch m.anomalyFilter {
	case anomalySevere:
		for _, check := range checks {
			if check.Level == "严重" {
				return true
			}
		}
		return false
	case anomalyWarn:
		for _, check := range checks {
			if check.Level == "警告" {
				return true
			}
		}
		return false
	case anomalyOffline:
		return checksContainKind(checks, "offline")
	case anomalyResource:
		return checksContainKind(checks, "resource")
	case anomalyContainer:
		return checksContainKind(checks, "container")
	case anomalyService:
		return checksContainKind(checks, "service")
	case anomalySecurity:
		return checksContainKind(checks, "security")
	default:
		return true
	}
}

func checksContainKind(checks []checkItem, kind string) bool {
	for _, check := range checks {
		if checkKind(check) == kind {
			return true
		}
	}
	return false
}

func actionableChecks(checks []checkItem) []checkItem {
	out := make([]checkItem, 0, len(checks))
	for _, check := range checks {
		if check.Level == "严重" || check.Level == "警告" {
			out = append(out, check)
		}
	}
	return out
}

func checkCounts(checks []checkItem) (int, int, int) {
	severe := 0
	warn := 0
	tip := 0
	for _, check := range checks {
		switch check.Level {
		case "严重":
			severe++
		case "警告":
			warn++
		case "提示":
			tip++
		}
	}
	return severe, warn, tip
}

func (m Model) renderAnomalyOverview() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 42 {
		bodyWidth = 42
	}
	items := m.anomalyItems()
	m.anomalyIndex = clampInt(m.anomalyIndex, 0, maxInt(0, len(items)-1))
	totalSevere, totalWarn := 0, 0
	for _, item := range items {
		severe, warn, _ := checkCounts(item.Checks)
		totalSevere += severe
		totalWarn += warn
	}
	title := fmt.Sprintf("异常总览  %d台  %s", len(items), anomalyFilterName(m.anomalyFilter))
	if totalSevere > 0 {
		title += "  " + redStyle.Render(fmt.Sprintf("严重%d", totalSevere))
	}
	if totalWarn > 0 {
		title += "  " + yellowStyle.Render(fmt.Sprintf("警告%d", totalWarn))
	}
	if m.refreshStatus != "" {
		title += "  " + m.refreshStatus
	}
	contentHeight := m.height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	lines := []string{}
	if len(items) == 0 {
		lines = append(lines, greenStyle.Render("没有发现严重或警告级别的问题。"))
		lines = append(lines, mutedStyle.Render("提示级别的问题仍可在服务器详情的风险页查看。"))
	} else {
		itemHeight := 3
		rowsVisible := contentHeight / itemHeight
		if rowsVisible < 1 {
			rowsVisible = 1
		}
		start, end := visibleRange(len(items), m.anomalyIndex, rowsVisible)
		if end <= start {
			end = minInt(len(items), start+1)
		}
		lastGroup := ""
		for i := start; i < end; i++ {
			group := anomalyGroupName(items[i].Checks)
			if group != lastGroup {
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, anomalyGroupTitle(group))
				lastGroup = group
			}
			if len(lines)+itemHeight > contentHeight {
				break
			}
			lines = append(lines, m.anomalyItemLines(items[i], i == m.anomalyIndex, bodyWidth)...)
			lines = append(lines, "")
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fitANSI(title, width)),
		box,
		renderHelp(width, "移动 ↑↓/jk  详情 Enter/Space  筛选 f/Tab  全部0 严重1 警告2 离线3 资源4 容器5 服务6 安全7  刷新 r  返回 q/Esc"),
	}, "\n")
}

func (m Model) anomalyItemLines(item anomalyItem, selected bool, width int) []string {
	state := m.states[item.Index]
	h := state.Host
	metrics := state.Metrics
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	severe, warn, _ := checkCounts(item.Checks)
	summary := []string{}
	if severe > 0 {
		summary = append(summary, redStyle.Render(fmt.Sprintf("严重%d", severe)))
	}
	if warn > 0 {
		summary = append(summary, yellowStyle.Render(fmt.Sprintf("警告%d", warn)))
	}
	name := hostDisplayName(h)
	status := "离线"
	if state.Loading {
		status = "采集中"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := 30
	if width < 90 {
		nameWidth = 24
	}
	if width < 72 {
		nameWidth = 18
	}
	nameText := nameStyle.Render(padVisible(fitANSI(name, nameWidth), nameWidth))
	riskText := padVisible(strings.Join(summary, " "), 10)
	statusText := padVisible(colorStatus(status, state.Loading, metrics.Online), 6)
	mainLine := fmt.Sprintf("%s %s  %s  %s  %s  %s",
		prefix,
		nameText,
		statusText,
		riskText,
		anomalyResourceText(state),
		serviceCardText(metrics),
	)
	reasons := make([]string, 0, minInt(3, len(item.Checks)))
	for _, check := range item.Checks {
		reasons = append(reasons, stripCheckPrefix(check.Text))
		if len(reasons) >= 3 {
			break
		}
	}
	reasonLine := "  " + mutedStyle.Render("问题 ") + detailValueStyle.Render(strings.Join(reasons, "；"))
	return []string{
		fitANSI(mainLine, width),
		fitANSI(reasonLine, width),
	}
}

func anomalyGroupName(checks []checkItem) string {
	severe, _, _ := checkCounts(checks)
	if severe > 0 {
		return "严重"
	}
	return "警告"
}

func anomalyGroupTitle(group string) string {
	if group == "严重" {
		return detailDangerSubTitle("严重")
	}
	return detailSubTitle("警告")
}

func anomalyFilterName(filter anomalyFilterMode) string {
	switch filter {
	case anomalySevere:
		return "严重"
	case anomalyWarn:
		return "警告"
	case anomalyOffline:
		return "离线"
	case anomalyResource:
		return "资源"
	case anomalyContainer:
		return "容器"
	case anomalyService:
		return "服务"
	case anomalySecurity:
		return "安全"
	default:
		return "全部"
	}
}

func anomalyResourceText(state hostState) string {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render("CPU -  内存 -  磁盘 -")
	}
	return strings.Join([]string{
		"CPU " + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%.0f%%", metrics.CPUPercent)),
		"内存 " + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%.0f%%", metrics.MemPercent())),
		"磁盘 " + diskMountPercentText(metrics),
	}, "  ")
}

func stripCheckPrefix(value string) string {
	value = strings.TrimSpace(value)
	for _, sep := range []string{"：风险，", "：警告，", "：提示，"} {
		if strings.Contains(value, sep) {
			parts := strings.SplitN(value, sep, 2)
			return strings.TrimSpace(parts[0] + "：" + parts[1])
		}
	}
	return value
}

func anomalyDetailSection(checks []checkItem) string {
	priority := []struct {
		Kind    string
		Section string
	}{
		{"offline", "基础信息"},
		{"expire", "基础信息"},
		{"container", "容器"},
		{"service", "服务状态"},
		{"security", "登录记录"},
		{"resource", "资源监控"},
	}
	for _, item := range priority {
		for _, check := range checks {
			if checkKind(check) == item.Kind {
				return item.Section
			}
		}
	}
	return "风险提示"
}

func checkKind(check checkItem) string {
	text := strings.TrimSpace(check.Text)
	switch {
	case strings.HasPrefix(text, "服务器到期："):
		return "expire"
	case strings.HasPrefix(text, "服务器状态："):
		return "offline"
	case strings.HasPrefix(text, "CPU使用：") ||
		strings.HasPrefix(text, "内存使用：") ||
		strings.HasPrefix(text, "磁盘容量："):
		return "resource"
	case strings.HasPrefix(text, "容器状态：") ||
		strings.HasPrefix(text, "容器详情："):
		return "container"
	case strings.HasPrefix(text, "系统服务：") ||
		strings.HasPrefix(text, "健康端口：") ||
		strings.HasPrefix(text, "端口详情："):
		return "service"
	case strings.HasPrefix(text, "允许密码登录：") ||
		strings.HasPrefix(text, "允许root登录：") ||
		strings.HasPrefix(text, "密钥登录：") ||
		strings.HasPrefix(text, "SSH端口：") ||
		strings.HasPrefix(text, "SSH配置检查：") ||
		strings.HasPrefix(text, "失败登录来源IP过多："):
		return "security"
	default:
		return "other"
	}
}

func (m Model) useSingleFormPane(width int) bool {
	return width < 96
}

func (m Model) renderServerFormPane(title string, width int, height int) string {
	fields := m.form.fields()
	lines := make([]string, 0, len(fields)+2)
	lines = append(lines, titleStyle.Render("服务器"))
	innerWidth := width - 4
	if innerWidth < 24 {
		innerWidth = 24
	}
	contentHeight := height - 2
	if contentHeight < 4 {
		contentHeight = 4
	}
	if m.formIndex == jumpHostRefFormIndex || strings.TrimSpace(m.form.JumpHostRef) != "" {
		lines = append(lines, mutedStyle.Render("跳板机只转发连接，密钥文件都在本地。"))
	}
	fieldHeight := contentHeight - len(lines)
	if fieldHeight < 1 {
		fieldHeight = 1
	}
	selectedRow := selectedFieldRow(fields, m.formIndex)
	start, end := visibleRange(len(fields), selectedRow, fieldHeight)
	for i := start; i < end; i++ {
		field := fields[i]
		if field.Section {
			if len(lines) > 1 {
				lines = append(lines, "")
			}
			lines = append(lines, sectionTitle(field.Label))
			continue
		}
		prefix := " "
		style := lipgloss.NewStyle()
		value := field.Value
		if field.ID == categoryFormIndex {
			value = m.form.Category
			if value == "" && len(m.categories) > 0 {
				value = m.categories[m.categoryIndex]
			}
			value += mutedStyle.Render("  ←/→")
		} else if field.ID == expireAtFormIndex {
			value = dateInputText(m.form.ExpireAt, m.formCursor, m.formPane == 0 && field.ID == m.formIndex)
		} else if field.ID == jumpHostRefFormIndex {
			value += mutedStyle.Render("  ←/→")
		}
		if m.formPane == 0 && field.ID == m.formIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		if field.ID == expireAtFormIndex || field.ID == jumpHostRefFormIndex {
			lines = append(lines, style.Render(formFieldLine(prefix, field.Label, value, innerWidth, false, false, m.formCursor)))
		} else {
			lines = append(lines, style.Render(formFieldLine(prefix, field.Label, value, innerWidth, field.ID != categoryFormIndex, m.formPane == 0 && field.ID == m.formIndex, m.formCursor)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width)
	if m.formPane == 0 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCategoryPane(width int, height int) string {
	lines := []string{titleStyle.Render("分类")}
	innerWidth := width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	contentHeight := height - 2
	if contentHeight < 5 {
		contentHeight = 5
	}
	bottomLineCount := 0
	listHeight := contentHeight - 1 - bottomLineCount
	if listHeight < 1 {
		listHeight = 1
	}
	if len(m.categories) == 0 {
		lines = append(lines, mutedStyle.Render("没有分类"))
	} else {
		start, end := visibleRange(len(m.categories), m.categoryIndex, listHeight)
		for i := start; i < end; i++ {
			category := m.categories[i]
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.categoryIndex {
				prefix = "▶"
				if m.formPane == 1 && !m.addingCategory {
					style = blueStyle.Bold(true)
				}
			}
			count := m.categoryHostCount(category)
			lines = append(lines, style.Render(categoryLine(prefix, category, count, innerWidth)))
		}
	}
	for len(lines) < 1+listHeight {
		lines = append(lines, "")
	}
	if m.addingCategory || m.renamingCategory {
		label := "新分类 "
		if m.renamingCategory {
			label = "重命名 "
		}
		lines = append(lines, blueStyle.Bold(true).Render(prefixedCursorText(label, m.categoryDraft, innerWidth)))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width)
	if m.formPane == 1 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func formFieldLine(prefix string, label string, value string, width int, boxed bool, active bool, cursor int) string {
	const labelWidth = 12
	labelText := prefix + " " + padVisible(label, labelWidth)
	valueWidth := width - ansi.StringWidth(labelText) - 1
	if valueWidth < 8 {
		valueWidth = 8
	}
	if boxed {
		if valueWidth > 32 {
			valueWidth = 32
		}
		boxWidth := valueWidth - 2
		if boxWidth < 4 {
			boxWidth = 4
		}
		if active {
			value = "[" + formInputText(value, boxWidth, cursor) + "]"
		} else {
			value = "[" + padVisible(value, boxWidth) + "]"
		}
	} else {
		value = fit(value, valueWidth)
	}
	return fit(labelText+" "+value, width)
}

func dateInputText(value string, cursor int, active bool) string {
	runes := []rune(dateMask(value))
	positions := dateInputPositions()
	if active {
		cursor = clampInt(cursor, 0, len(positions))
		if cursor < len(positions) {
			pos := positions[cursor]
			runes = append(runes[:pos], append([]rune{'│'}, runes[pos:]...)...)
		} else {
			runes = append(runes, '│')
		}
	}
	return "[" + string(runes) + "]"
}

func dateDigits(value string) string {
	var out []rune
	for _, r := range value {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return string(out)
}

func formInputText(value string, width int, cursor int) string {
	return padVisible(inlineCursorText(value, width, cursor), width)
}

func inlineCursorText(value string, width int, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if width < 1 {
		width = 1
	}
	contentWidth := width - 1
	before := visibleTailByWidth(runes[:cursor], contentWidth)
	remaining := contentWidth - runewidth.StringWidth(before)
	if remaining < 0 {
		remaining = 0
	}
	after := visibleHeadByWidth(runes[cursor:], remaining)
	return before + "│" + after
}

func prefixedCursorText(prefix string, value string, width int) string {
	inputWidth := width - runewidth.StringWidth(prefix)
	if inputWidth < 1 {
		inputWidth = 1
	}
	return fit(prefix+inlineCursorText(value, inputWidth, len([]rune(value))), width)
}

func visibleTailByWidth(runes []rune, width int) string {
	if width <= 0 || len(runes) == 0 {
		return ""
	}
	used := 0
	start := len(runes)
	for start > 0 {
		r := runes[start-1]
		rw := runewidth.RuneWidth(r)
		if used+rw > width {
			break
		}
		used += rw
		start--
	}
	return string(runes[start:])
}

func visibleHeadByWidth(runes []rune, width int) string {
	if width <= 0 || len(runes) == 0 {
		return ""
	}
	used := 0
	end := 0
	for end < len(runes) {
		rw := runewidth.RuneWidth(runes[end])
		if used+rw > width {
			break
		}
		used += rw
		end++
	}
	return string(runes[:end])
}

func categoryLine(prefix string, category string, count int, width int) string {
	countText := ""
	if count > 0 {
		countText = fmt.Sprintf("%d台", count)
	}
	prefixText := prefix + " "
	nameWidth := width - ansi.StringWidth(prefixText) - ansi.StringWidth(countText)
	if countText != "" {
		nameWidth--
	}
	if nameWidth < 6 {
		nameWidth = 6
	}
	line := prefixText + fit(category, nameWidth)
	if countText != "" {
		spaces := width - ansi.StringWidth(line) - ansi.StringWidth(countText)
		if spaces < 1 {
			spaces = 1
		}
		line += strings.Repeat(" ", spaces) + countText
	}
	return fit(line, width)
}

func (m Model) categoryHostCount(category string) int {
	count := 0
	for _, state := range m.states {
		if state.Host.Category == category {
			count++
		}
	}
	return count
}

func (m Model) renderDeleteConfirm() string {
	if m.deleteIndex < 0 || m.deleteIndex >= len(m.states) {
		return "没有选中的服务器"
	}
	h := m.states[m.deleteIndex].Host
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{
		wrapPlainLine("服务器："+h.Name, bodyWidth),
		wrapPlainLine("文件："+h.File, bodyWidth),
		"",
		"将删除该服务器配置。",
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(red).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit("确认删除服务器", width)),
		box,
		renderHelp(width, "确认 Enter/y  取消 Esc/n"),
	}, "\n")
}

func (m Model) renderConfirmAction() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := []string{}
	for _, line := range m.confirm.Lines {
		lines = append(lines, wrapPlainLine(line, bodyWidth))
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(red).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		titleStyle.Render(fit(m.confirm.Title, width)),
		box,
		renderHelp(width, "确认 Enter/y  取消 Esc/n"),
	}, "\n")
}

func (m Model) renderDetail() string {
	lines, ok := m.detailLines()
	if !ok {
		return "没有选中的服务器"
	}
	idx, _ := m.selectedRealIndex()
	width := detailFrameWidth(m.width)
	headerText := "服务器详情  " + hostDisplayName(m.states[idx].Host)
	if checks := m.currentDetailChecks(); len(checks) > 0 {
		headerText += "  " + riskSummaryText(checks)
	}
	header := titleStyle.Render(fitANSI(headerText, width))
	help := renderHelp(width, "滚动 ↑↓/jk  分类 ←→/Tab  登录 l  命令 m  上传 u  下载 d  刷新 r  返回 q/Esc")
	tabs := m.renderDetailSectionTabs(width)
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	bodyContent := tabs + "\n" + detailFrameSeparator(width-2) + "\n" + strings.Join(lines, "\n")
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(bodyContent)
	return strings.Join([]string{
		header,
		body,
		help,
	}, "\n")
}

func (m Model) renderDetailSectionTabs(width int) string {
	sections := m.detailSectionNames()
	activeIndex := m.detailSectionIndex
	if len(sections) > 0 && activeIndex >= len(sections) {
		activeIndex = len(sections) - 1
	}
	contentWidth := width - 2
	if contentWidth < 10 {
		contentWidth = 10
	}
	parts := make([]string, 0, len(sections))
	for i, section := range sections {
		label := shortDetailSectionName(section)
		if i == activeIndex {
			parts = append(parts, titleStyle.Render(label))
		} else {
			parts = append(parts, mutedStyle.Render(label))
		}
	}
	value := strings.Join(parts, "  ")
	if ansi.StringWidth(value) > contentWidth && activeIndex > 0 {
		value = strings.Join(parts[activeIndex:], "  ")
	}
	line := padVisible(fitANSI(value, contentWidth), contentWidth)
	return line
}

func detailFrameSeparator(width int) string {
	if width < 1 {
		width = 1
	}
	return cardBorderStyle.Render(strings.Repeat("─", width))
}

func shortDetailSectionName(section string) string {
	switch section {
	case "基础信息":
		return "基础"
	case "资源监控":
		return "资源"
	case "服务状态":
		return "服务"
	case "登录记录":
		return "登录"
	case "风险提示":
		return "风险"
	default:
		return section
	}
}

func (m Model) detailLines() ([]string, bool) {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return nil, false
	}
	state := m.states[idx]
	h := state.Host
	metrics := state.Metrics

	status := "离线"
	if state.Loading {
		status = "采集中"
	} else if metrics.Online {
		status = "在线"
	}

	lines := []string{
		sectionTitle("基础信息"),
		m.detailRow("状态", colorStatus(status, state.Loading, metrics.Online)),
		m.detailRow("地址", h.Address()),
		m.detailRow("用户", h.User),
		m.detailRow("端口", h.Port),
		m.detailRow("分类", emptyDash(h.Category)),
		m.detailRow("收藏", yesNo(h.Favorite)),
		m.detailRow("置顶", yesNo(h.Pinned)),
		m.detailRow("认证方式", authText(h)),
		m.detailRow("跳板机", jumpDetailText(h)),
		m.detailRow("跳板机密钥", jumpKeyText(h)),
		m.detailRow("主机名", emptyDash(metrics.RemoteHostname)),
		m.detailRow("系统", emptyDash(metrics.OS)),
		m.detailRow("内核", emptyDash(metrics.Kernel)),
		m.detailRow("架构", emptyDash(metrics.Arch)),
		m.detailRow("来源", h.File),
		m.detailRow("到期时间", emptyDash(h.ExpireAt)),
		m.detailRow("剩余时间", expireDetailText(h.ExpireAt)),
		m.detailRow("备注", emptyDash(h.Note)),
		m.detailRow("最近登录", lastLoginDetail(m.lastLogin(h))),
	}
	checks := buildChecks(state)
	lines = append(lines,
		"",
		sectionTitle("资源监控"),
		detailSubTitle("CPU"),
		m.detailRow("使用率", percentBar(metrics.CPUPercent)),
		m.detailRow("核心数", cpuCoresText(metrics)),
		m.detailRow("型号", emptyDash(metrics.CPUModel)),
		"",
		detailSubTitle("内存"),
		m.detailRow("使用率", fmt.Sprintf("%s  %s / %s", percentBar(metrics.MemPercent()), bytesHuman(metrics.MemUsed), bytesHuman(metrics.MemTotal))),
		m.detailRow("可用", bytesHuman(metrics.MemAvailable)),
		m.detailRow("Swap", swapUsageText(metrics)),
		m.detailRow("Swap可用", swapFreeText(metrics)),
		"",
		detailSubTitle("磁盘"),
		m.detailRow("挂载点", emptyDash(metrics.DiskMountpoint)),
		m.detailRow("文件系统", emptyDash(metrics.DiskFilesystem)),
		m.detailRow("类型", emptyDash(metrics.DiskType)),
		m.detailRow("使用率", fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.DiskPercent(), 80, 90), bytesHuman(metrics.DiskUsed), bytesHuman(metrics.DiskTotal))),
		m.detailRow("可用", bytesHuman(metrics.DiskAvailable)),
		m.diskListText(metrics),
		m.detailRow("索引节点", inodeUsageText(metrics)),
		m.detailRow("可用节点", countHuman(metrics.InodeAvailable)),
		"",
		detailSubTitle("系统"),
		m.detailRow("负载", fmt.Sprintf("%s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))),
		m.detailRow("运行时间", uptimeCN(metrics.Uptime)),
		"",
		sectionTitle("服务状态"),
		detailSubTitle("健康"),
		m.detailRow("健康端口", healthPortsText(metrics)),
		"",
		detailSubTitle("端口"),
	)
	lines = append(lines, portDetailRows(m, state)...)
	lines = append(lines,
		"",
		detailSubTitle("异常"),
		m.detailRow("异常服务", failedServiceText(metrics, 8)),
		"",
		sectionTitle("容器"),
		detailSubTitle("状态"),
	)
	lines = append(lines, dockerDetailRows(m, metrics, state)...)
	lines = append(lines, "", detailSubTitle("详情"))
	lines = append(lines, containerDetailRows(m, state)...)
	if metrics.Error != "" {
		lines = append(lines, "", sectionTitle("最近错误"), m.detailRow("错误", metrics.Error))
	}
	lines = append(lines, "", sectionTitle("登录记录"), detailSuccessSubTitle("成功"))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.LoginSummary, state.LoginError, false)...)
	lines = append(lines, "", detailDangerSubTitle("失败"))
	lines = append(lines, loginSummaryDetailRows(m, state.LoginLoading, state.FailedLoginSummary, state.FailedLoginError, true)...)
	lines = append(lines, "", sectionTitle("风险提示"))
	lines = append(lines, checkSuggestionRows(m, state, checks)...)
	lines = m.activeDetailSectionLines(lines)
	return lines, true
}

func (m Model) currentDetailChecks() []checkItem {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return nil
	}
	return buildChecks(m.states[idx])
}

func (m Model) activeDetailSectionLines(lines []string) []string {
	sections := m.detailSectionNames()
	if len(sections) == 0 {
		return lines
	}
	index := clampInt(m.detailSectionIndex, 0, len(sections)-1)
	target := sections[index]
	out := []string{}
	inSection := false
	for _, line := range lines {
		name, isSection := detailSectionNameFromLine(line)
		if isSection {
			if name == target {
				inSection = true
				out = append(out, m.renderDetailSectionLine(name, line))
				continue
			}
			if inSection {
				break
			}
			continue
		}
		if inSection {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return []string{m.renderDetailSectionLine(target, sectionTitle(target)), m.detailRow("状态", "暂无内容")}
	}
	return out
}

func (m Model) renderDetailSectionLine(name string, fallback string) string {
	sections := m.detailSectionNames()
	selected := false
	if m.detailSectionIndex >= 0 && m.detailSectionIndex < len(sections) {
		selected = sections[m.detailSectionIndex] == name
	}
	marker := "  "
	style := detailSectionStyle
	if selected {
		marker = "▶ "
		style = blueStyle.Bold(true)
	}
	if name == "" {
		return fallback
	}
	return marker + style.Render("["+name+"]")
}

func detailSectionNameFromLine(line string) (string, bool) {
	plain := ansi.Strip(line)
	plain = strings.TrimSpace(strings.TrimPrefix(plain, "▶"))
	if !strings.HasPrefix(plain, "[") {
		return "", false
	}
	start := strings.Index(plain, "[")
	end := strings.Index(plain, "]")
	if start < 0 || end <= start {
		return "", false
	}
	return plain[start+1 : end], true
}

func (m Model) detailViewportHeight() int {
	height := m.height - 6
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) detailMaxScroll() int {
	lines, ok := m.detailLines()
	if !ok {
		return 0
	}
	maxScroll := len(lines) - m.detailViewportHeight()
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (m Model) renderPicker() string {
	header := m.pickTitle
	if m.status != "" && m.status != m.pickTitle {
		header += "  " + m.status
	}
	width := detailFrameWidth(m.width)
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.choices) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的项目"))
	} else {
		maxRows := m.height - 5
		if maxRows < 5 {
			maxRows = 5
		}
		start := 0
		if m.pickIndex >= maxRows {
			start = m.pickIndex - maxRows + 1
		}
		end := start + maxRows
		if end > len(m.choices) {
			end = len(m.choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.pickIndex {
				prefix = "▶"
				style = lipgloss.NewStyle().Foreground(blue).Bold(true)
			}
			label := m.choices[i].Label
			if m.treePickerActive() && m.choices[i].IsDir {
				label = blueStyle.Render(label)
			}
			lines = append(lines, style.Render(fit(fmt.Sprintf("%s %s", prefix, label), width)))
		}
	}
	help := "移动 ↑↓/jk  选择 Enter  返回 Esc"
	if m.treePickerActive() {
		help = "移动 ↑↓/jk  展开 Enter  选择 Space  返回 Esc"
	}
	lines = append(lines, "", renderHelp(width, help))
	return strings.Join(lines, "\n")
}

func (m Model) renderTransferPanel() string {
	title := "上传文件"
	if m.panel.Mode == transferDownload {
		title = "下载文件"
	}
	header := title
	if m.status != "" {
		header += "  " + m.status
	}
	width := formContentWidth(m.width)
	help := "切换 Tab  移动 ↑↓/jk  展开 Enter  选择 Space  任务 s  返回 Esc"
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	body := ""
	if m.useSingleTransferPane(width) {
		if m.panel.ActivePane == 0 {
			body = renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, width, height, true, m.panel.LeftSelected)
		} else {
			body = renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, width, height, true, nil)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) / 2
		rightWidth := width - gap - leftWidth
		left := renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, leftWidth, height, m.panel.ActivePane == 0, m.panel.LeftSelected)
		right := renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, rightWidth, height, m.panel.ActivePane == 1, nil)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		body,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) renderTransferJobs() string {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 34 {
		width = 34
	}
	help := renderTransferJobsHelp(width)
	reservedBottomLines := strings.Count(help, "\n") + 1
	counts := transferStatusCounts(m.transferHistory.Entries)
	filtered := m.filteredTransferIndexes()
	header := fmt.Sprintf("传输任务  状态 %s  显示 %d/%d  运行 %d  未完成 %d  已完成 %d", m.transferStatusFilterName(), len(filtered), len(m.transferHistory.Entries), counts[config.TransferStatusRunning], transferUnfinishedCount(m.transferHistory.Entries), counts[config.TransferStatusDone])
	lines := []string{titleStyle.Render(fit(header, width)), ""}
	if len(m.transferHistory.Entries) == 0 {
		lines = append(lines, mutedStyle.Render("暂无传输记录"))
	} else if len(filtered) == 0 {
		lines = append(lines, mutedStyle.Render("当前状态没有传输任务"))
	} else {
		bodyLines := m.height - reservedBottomLines - 2
		if bodyLines < 1 {
			bodyLines = 1
		}
		cardLines, selectedTop, selectedBottom := m.transferJobGridLines(width)
		start, end := dashboardLineWindow(len(cardLines), selectedTop, selectedBottom, bodyLines)
		lines = append(lines, cardLines[start:end]...)
	}
	lines = padToBottom(lines, m.height, reservedBottomLines)
	lines = append(lines, help)
	return strings.Join(lines, "\n")
}

func (m Model) renderTransferDetail() string {
	m.reloadTransfers()
	entry, ok := m.selectedTransferEntry()
	width := detailFrameWidth(m.width)
	if width < 34 {
		width = 34
	}
	if !ok {
		return strings.Join([]string{
			titleStyle.Render(fit("传输详情", width)),
			mutedStyle.Render("当前任务不存在"),
			renderHelp(width, "返回 Esc"),
		}, "\n")
	}
	lines := m.transferDetailLines(entry)
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	headerText := fmt.Sprintf("传输详情  %s  %s", transferEntryName(entry), transferStatusText(entry.Status))
	header := titleStyle.Render(fitANSI(headerText, width))
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	help := renderHelp(width, "滚动 ↑↓/jk  开始 Enter  全部开始 a  全部暂停 p  取消 c  删除 x  返回 Esc")
	return strings.Join([]string{header, body, help}, "\n")
}

func (m Model) selectedTransferEntry() (config.TransferEntry, bool) {
	if len(m.transferHistory.Entries) == 0 || m.transferIndex < 0 || m.transferIndex >= len(m.transferHistory.Entries) {
		return config.TransferEntry{}, false
	}
	return m.transferHistory.Entries[m.transferIndex], true
}

func (m Model) transferDetailLines(entry config.TransferEntry) []string {
	status := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true).Render(transferStatusText(entry.Status))
	total := "-"
	if entry.TotalBytes > 0 {
		total = bytesHuman(uint64(entry.TotalBytes))
	}
	done := "-"
	if entry.TotalBytes > 0 || transferProgressDoneBytes(entry) > 0 {
		done = bytesHuman(uint64(transferProgressDoneBytes(entry)))
	}
	remaining := transferRemainingBytesText(entry)
	speed, remain := transferProgressSpeedRemain(entry.Progress)
	percent := transferPercentText(entry)
	progress := transferProgressBarLine(entry, m.detailContentWidth())
	lines := []string{
		m.renderDetailSectionLine("基本信息", sectionTitle("基本信息")),
		m.detailRow("状态", status),
		m.detailRow("类型", transferEntryKindText(entry)),
		m.detailRow("方向", transferDirectionText(entry)),
		m.detailRow("文件", transferEntryName(entry)),
		m.detailRow("目录", yesNo(entry.IsDir)),
		m.detailRow("任务ID", entry.ID),
		m.detailRow("服务器", ansi.Strip(transferEntryHostTitle(entry))),
		m.detailRow("连接", transferEntryConnection(entry)),
		m.detailRow("创建时间", transferTimeShort(entry.Time)),
		m.detailRow("更新时间", transferTimeShort(entry.UpdatedAt)),
		m.detailRow("队列位置", transferQueueText(m.transferHistory.Entries, entry)),
		m.detailRow("传输方式", "rsync，支持断点续传，保留半成品"),
		"",
		m.renderDetailSectionLine("路径信息", sectionTitle("路径信息")),
		m.detailRow("来源", entry.Source),
		m.detailRow("目标", transferJobTarget(entry)),
		"",
		m.renderDetailSectionLine("传输进度", sectionTitle("传输进度")),
		m.detailRow("进度", progress),
		m.detailRow("百分比", percent),
		m.detailRow("总大小", total),
		m.detailRow("已完成", done),
		m.detailRow("剩余大小", remaining),
		m.detailRow("速度", emptyDash(speed)),
		m.detailRow("剩余时间", emptyDash(remain)),
		m.detailRow("原始进度", emptyDash(strings.Join(strings.Fields(entry.Progress), " "))),
		"",
		m.renderDetailSectionLine("操作", sectionTitle("操作")),
		m.detailRow("可操作", transferActionHint(entry.Status)),
	}
	if strings.TrimSpace(entry.Error) != "" {
		lines = append(lines, "", m.renderDetailSectionLine("错误", sectionTitle("错误")), m.detailRow("错误", redStyle.Render(entry.Error)))
	}
	return lines
}

func (m Model) transferDetailMaxScroll() int {
	entry, ok := m.selectedTransferEntry()
	if !ok {
		return 0
	}
	maxScroll := len(m.transferDetailLines(entry)) - m.detailViewportHeight()
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func transferEntryConnection(entry config.TransferEntry) string {
	user := strings.TrimSpace(entry.User)
	if user == "" {
		user = "-"
	}
	host := strings.TrimSpace(entry.Host)
	if host == "" {
		host = "-"
	}
	port := strings.TrimSpace(entry.Port)
	if port == "" {
		port = "22"
	}
	return fmt.Sprintf("%s@%s:%s", user, host, port)
}

func transferDirectionText(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return "远程 → 本地"
	}
	return "本地 → 远程"
}

func transferRemotePath(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return entry.Source
	}
	return transferJobTarget(entry)
}

func transferLocalPath(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return entry.TargetDir
	}
	return entry.Source
}

func transferPercentText(entry config.TransferEntry) string {
	percent, ok := transferProgressPercent(entry)
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%d%%", percent)
}

func transferRemainingBytesText(entry config.TransferEntry) string {
	if entry.TotalBytes <= 0 {
		return "-"
	}
	done := transferProgressDoneBytes(entry)
	if done >= entry.TotalBytes {
		return "0B"
	}
	return bytesHuman(uint64(entry.TotalBytes - done))
}

func transferQueueText(entries []config.TransferEntry, entry config.TransferEntry) string {
	if entry.Status == config.TransferStatusRunning {
		return "当前运行"
	}
	if entry.Status != config.TransferStatusPending && entry.Status != config.TransferStatusQueued {
		return "-"
	}
	position := 0
	total := 0
	for _, item := range entries {
		if item.Status != entry.Status {
			continue
		}
		total++
		if item.ID == entry.ID {
			position = total
		}
	}
	if position == 0 || total == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", position, total)
}

func transferActionHint(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return "Enter 开始，c 取消，x 删除"
	case config.TransferStatusPending:
		return "p 全部暂停，等待自动开始"
	case config.TransferStatusRunning:
		return "p 暂停，c 中断"
	case config.TransferStatusInterrupted:
		return "Enter 继续，a 全部开始，c 取消，x 删除"
	case config.TransferStatusFailed:
		return "Enter 重试，x 删除"
	case config.TransferStatusCanceled:
		return "x 删除"
	case config.TransferStatusDone:
		return "x 删除"
	default:
		return "-"
	}
}

func transferProgressSpeedRemain(progress string) (string, string) {
	fields := strings.Fields(strings.TrimSpace(progress))
	if len(fields) == 0 {
		return "", ""
	}
	percentIndex := -1
	for i, field := range fields {
		if rsyncPercentText(field) != "" {
			percentIndex = i
			break
		}
	}
	if percentIndex < 0 {
		return "", ""
	}
	speed := ""
	remain := ""
	for _, field := range fields[percentIndex+1:] {
		cleaned := strings.Trim(field, "()")
		if speed == "" && strings.Contains(cleaned, "/s") {
			speed = cleaned
			continue
		}
		if remain == "" && strings.Count(cleaned, ":") >= 2 {
			remain = cleaned
		}
	}
	return speed, remain
}

func renderTransferJobsHelp(width int) string {
	if width < 1 {
		width = 1
	}
	help := strings.Join([]string{
		"状态 Tab",
		"移动 ↑↓←→/hjkl",
		"开始 Enter",
		"详情 Space",
		"全部开始 a",
		"全部暂停 p",
		"取消 c",
		"删除 x",
		"返回 Esc",
	}, "  ")
	return helpStyle.Render(fit(help, width))
}

func transferStatusFilterOptions() []string {
	return []string{
		"",
		config.TransferStatusQueued,
		config.TransferStatusPending,
		config.TransferStatusRunning,
		config.TransferStatusDone,
		config.TransferStatusFailed,
		config.TransferStatusCanceled,
		config.TransferStatusInterrupted,
	}
}

func (m Model) transferStatusFilterValue() string {
	options := transferStatusFilterOptions()
	if m.transferStatusFilter < 0 || m.transferStatusFilter >= len(options) {
		return ""
	}
	return options[m.transferStatusFilter]
}

func (m Model) transferStatusFilterName() string {
	status := m.transferStatusFilterValue()
	if status == "" {
		return "全部"
	}
	return transferStatusText(status)
}

func (m Model) filteredTransferIndexes() []int {
	status := m.transferStatusFilterValue()
	indexes := make([]int, 0, len(m.transferHistory.Entries))
	for i, entry := range m.transferHistory.Entries {
		if status == "" || entry.Status == status {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m Model) transferJobGridLines(width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	indexes := m.filteredTransferIndexes()
	for i := 0; i < len(indexes); i += cols {
		rowEnd := i + cols
		if rowEnd > len(indexes) {
			rowEnd = len(indexes)
		}
		rowBlocks := make([]string, cols)
		rowHeight := 0
		rowHasError := false
		for j := i; j < rowEnd; j++ {
			entry := m.transferHistory.Entries[indexes[j]]
			if strings.TrimSpace(entry.Error) != "" {
				rowHasError = true
				break
			}
		}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			visibleIndex := i + col
			if visibleIndex >= rowEnd {
				continue
			}
			entryIndex := indexes[visibleIndex]
			if entryIndex == m.transferIndex {
				selectedTop = len(lines)
			}
			block := renderTransferJobCard(m.transferHistory.Entries[entryIndex], cardWidth, entryIndex == m.transferIndex, rowHasError)
			rowBlocks[col] = block
			if height := blockLineCount(block); height > rowHeight {
				rowHeight = height
			}
		}
		if rowHeight == 0 {
			continue
		}
		for col := 0; col < cols; col++ {
			if rowBlocks[col] == "" {
				rowBlocks[col] = blankTransferJobBlock(cardWidths[col], rowHeight)
			} else {
				rowBlocks[col] = padBlockHeight(rowBlocks[col], cardWidths[col], rowHeight)
			}
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, rowBlocks...), "\n")
		lines = append(lines, rowLines...)
		for _, index := range indexes[i:rowEnd] {
			if index == m.transferIndex {
				selectedBottom = len(lines)
				break
			}
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func blockLineCount(block string) int {
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

func blankTransferJobBlock(width int, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	return strings.Join(lines, "\n")
}

func padBlockHeight(block string, width int, height int) string {
	lines := strings.Split(padBlock(block, width), "\n")
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func renderTransferJobCard(entry config.TransferEntry, width int, selected bool, reserveErrorLine bool) string {
	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}

	title := transferEntryHostTitle(entry)
	meta := transferJobMeta(entry)
	dot := transferJobDot(entry.Status)
	nameLine := transferFileLine(entry, cardWidth-4, selected)
	sourceLine := transferPathLine(transferSourceSymbol(entry), entry.Source)
	targetLine := transferPathLine("→", transferJobTarget(entry))

	lines := []string{
		cardTopLine(cardWidth, title, meta, dot, borderStyle),
		cardContentLine(cardWidth, nameLine, borderStyle),
		cardContentLine(cardWidth, sourceLine, borderStyle),
		cardContentLine(cardWidth, targetLine, borderStyle),
		cardContentLine(cardWidth, transferProgressBarLine(entry, cardWidth-4), borderStyle),
	}
	if errorLine := transferJobError(entry); errorLine != "" || reserveErrorLine {
		lines = append(lines, cardContentLine(cardWidth, errorLine, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func transferEntryHostTitle(entry config.TransferEntry) string {
	category := strings.TrimSpace(entry.HostCategory)
	if category == "" {
		category = "未分类"
	}
	name := strings.TrimSpace(entry.HostName)
	if name == "" {
		name = "服务器"
	}
	return cardMutedStyle.Render("["+category+"]") + " " + detailValueStyle.Render(name)
}

func transferEntryName(entry config.TransferEntry) string {
	name := filepath.Base(strings.TrimRight(entry.Source, "/"))
	if name == "." || name == "/" || name == "" {
		name = entry.Source
	}
	if entry.IsDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name
}

func transferSourceSymbol(entry config.TransferEntry) string {
	if entry.Kind == "download" {
		return "↓"
	}
	return "↑"
}

func transferJobTarget(entry config.TransferEntry) string {
	if entry.Kind == "upload" {
		return entry.HostName + ":" + entry.TargetDir
	}
	return entry.TargetDir
}

func transferJobMeta(entry config.TransferEntry) string {
	style := lipgloss.NewStyle().Foreground(transferStatusColor(entry.Status)).Bold(true)
	return style.Render(transferStatusText(entry.Status))
}

func transferJobDot(status string) string {
	return lipgloss.NewStyle().Foreground(transferStatusColor(status)).Render("●")
}

func transferFieldLine(label string, value string) string {
	return cardMutedStyle.Render(label+" ") + detailValueStyle.Render(value)
}

func transferPathLine(label string, value string) string {
	return transferArrowStyle(label).Render(label+" ") + cardMutedStyle.Render(value)
}

func transferArrowStyle(label string) lipgloss.Style {
	switch label {
	case "↑", "↓", "→":
		return blueStyle
	default:
		return cardMutedStyle
	}
}

func transferFileLine(entry config.TransferEntry, width int, selected bool) string {
	nameStyle := detailValueStyle
	if selected {
		nameStyle = blueStyle.Bold(true)
	}
	left := cardMutedStyle.Render(transferEntryTypeLabel(entry)+" ") + nameStyle.Render(transferEntryName(entry))
	right := cardMutedStyle.Render(transferEntryKindText(entry) + " " + transferTimeText(entry))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		maxLeft := width - ansi.StringWidth(right) - 2
		if maxLeft < 8 {
			return left
		}
		left = fitANSI(left, maxLeft)
		gap = width - ansi.StringWidth(left) - ansi.StringWidth(right)
	}
	return left + strings.Repeat(" ", gap) + right
}

func transferEntryTypeLabel(entry config.TransferEntry) string {
	if entry.IsDir {
		return "目录"
	}
	return "文件"
}

func transferJobError(entry config.TransferEntry) string {
	if entry.Error != "" {
		return cardMutedStyle.Render("错误 ") + redStyle.Render(entry.Error)
	}
	return ""
}

func transferProgressBarLine(entry config.TransferEntry, width int) string {
	percent, ok := transferProgressPercent(entry)
	if !ok && entry.Status == config.TransferStatusDone {
		percent = 100
		ok = true
	}
	label := "--"
	if ok {
		label = fmt.Sprintf("%3d%%", percent)
	}
	style := transferProgressStyle(entry.Status)
	suffix := style.Render(label)
	if detail := transferProgressDetail(entry); detail != "" {
		maxDetail := width - 8 - runewidth.StringWidth(label) - 2
		if maxDetail > 4 {
			suffix += " " + cardMutedStyle.Render(fit(detail, maxDetail))
		}
	}
	barWidth := width - ansi.StringWidth(suffix) - 1
	if barWidth < 8 {
		barWidth = 8
	}
	filled := 0
	if ok {
		filled = int(float64(barWidth) * float64(percent) / 100)
		if percent > 0 && filled == 0 {
			filled = 1
		}
		if filled > barWidth {
			filled = barWidth
		}
	}
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", barWidth-filled))
	return bar + " " + suffix
}

func transferProgressPercent(entry config.TransferEntry) (int, bool) {
	if entry.TotalBytes > 0 {
		done := transferProgressDoneBytes(entry)
		percent := int(float64(done) * 100 / float64(entry.TotalBytes))
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		return percent, true
	}
	percentText := rsyncPercentText(entry.Progress)
	if percentText == "" {
		return 0, false
	}
	value := strings.TrimSuffix(percentText, "%")
	percent, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return percent, true
}

func transferProgressDoneBytes(entry config.TransferEntry) int64 {
	done := entry.DoneBytes + entry.CurrentBytes
	if entry.TotalBytes > 0 && done > entry.TotalBytes {
		return entry.TotalBytes
	}
	if done < 0 {
		return 0
	}
	return done
}

func transferProgressDetail(entry config.TransferEntry) string {
	sizeText := ""
	if entry.TotalBytes > 0 {
		sizeText = bytesPair(uint64(transferProgressDoneBytes(entry)), uint64(entry.TotalBytes))
	}
	progress := strings.Join(strings.Fields(strings.TrimSpace(entry.Progress)), " ")
	percent := rsyncPercentText(progress)
	if progress == "" || percent == "" || progress == percent {
		return sizeText
	}
	if idx := strings.Index(progress, " ("); idx >= 0 {
		progress = strings.TrimSpace(progress[:idx])
	}
	idx := strings.Index(progress, percent)
	if idx < 0 {
		return ""
	}
	before := strings.TrimSpace(progress[:idx])
	after := strings.TrimSpace(progress[idx+len(percent):])
	rsyncText := strings.TrimSpace(before + " " + after)
	if sizeText == "" {
		return rsyncText
	}
	if after == "" {
		return sizeText
	}
	return strings.TrimSpace(sizeText + " " + after)
}

func transferProgressStyle(status string) lipgloss.Style {
	switch status {
	case config.TransferStatusQueued:
		return detailSubTitleStyle
	case config.TransferStatusPending:
		return blueStyle
	case config.TransferStatusRunning:
		return blueStyle
	case config.TransferStatusDone:
		return greenStyle
	case config.TransferStatusFailed:
		return redStyle
	case config.TransferStatusInterrupted:
		return yellowStyle
	case config.TransferStatusCanceled:
		return mutedStyle
	default:
		return mutedStyle
	}
}

func transferStatusColor(status string) lipgloss.Color {
	switch status {
	case config.TransferStatusQueued:
		return cyan
	case config.TransferStatusPending:
		return blue
	case config.TransferStatusRunning:
		return blue
	case config.TransferStatusDone:
		return green
	case config.TransferStatusFailed:
		return red
	case config.TransferStatusInterrupted:
		return yellow
	case config.TransferStatusCanceled:
		return textGray
	default:
		return textGray
	}
}

func transferStatusText(status string) string {
	switch status {
	case config.TransferStatusQueued:
		return "等待中"
	case config.TransferStatusPending:
		return "排队中"
	case config.TransferStatusRunning:
		return "运行中"
	case config.TransferStatusDone:
		return "已完成"
	case config.TransferStatusFailed:
		return "失败"
	case config.TransferStatusCanceled:
		return "已取消"
	case config.TransferStatusInterrupted:
		return "中断"
	default:
		return status
	}
}

func transferTimeText(entry config.TransferEntry) string {
	if entry.UpdatedAt != "" {
		return transferTimeShort(entry.UpdatedAt)
	}
	return transferTimeShort(entry.Time)
}

func transferTimeShort(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return t.Local().Format("01-02 15:04")
}

func transferStatusCounts(entries []config.TransferEntry) map[string]int {
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.Status]++
	}
	return counts
}

func transferUnfinishedCount(entries []config.TransferEntry) int {
	total := 0
	for _, entry := range entries {
		if entry.Status == config.TransferStatusQueued || entry.Status == config.TransferStatusPending || entry.Status == config.TransferStatusRunning || entry.Status == config.TransferStatusInterrupted {
			total++
		}
	}
	return total
}

func (m Model) useSingleTransferPane(width int) bool {
	return width < 70
}

func renderTransferPane(title string, choices []choice, index int, width int, height int, active bool, selected map[string]bool) string {
	if width < 34 {
		width = 34
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(softGray).Padding(0, 1).Width(width)
	if active {
		style = style.BorderForeground(blue)
	}
	innerWidth := width - 4
	lines := []string{titleStyle.Render(title)}
	if len(choices) == 0 {
		lines = append(lines, mutedStyle.Render("没有可选择的项目"))
	} else {
		maxRows := height - 2
		if maxRows < 3 {
			maxRows = 3
		}
		start := 0
		if index >= maxRows {
			start = index - maxRows + 1
		}
		end := start + maxRows
		if end > len(choices) {
			end = len(choices)
		}
		for i := start; i < end; i++ {
			prefix := " "
			lineStyle := lipgloss.NewStyle()
			if choices[i].IsDir {
				lineStyle = blueStyle
			}
			if i == index {
				prefix = "▶"
				lineStyle = lineStyle.Bold(true)
			}
			mark := " "
			if selected != nil && selected[choices[i].Value] {
				mark = "✓"
			}
			lines = append(lines, lineStyle.Render(fit(prefix+" "+mark+" "+choices[i].Label, innerWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) renderDashboard(indexes []int) string {
	if m.searching {
		return m.renderDashboardList(indexes, m.width)
	}
	if m.dashboardMode == dashboardCategory {
		return m.renderDashboardCategory(indexes)
	}
	if m.dashboardMode == dashboardGrouped {
		return m.renderDashboardGrouped(indexes)
	}
	return m.renderDashboardGrid(indexes)
}

func dashboardModeName(mode dashboardMode) string {
	switch mode {
	case dashboardCategory:
		return "分类"
	case dashboardGrouped:
		return "分组"
	default:
		return "卡片"
	}
}

func (m Model) renderDashboardGrid(indexes []int) string {
	width := m.dashboardGridWidth()
	height := m.dashboardGridHeight()
	lines, selectedTop, selectedBottom := m.dashboardGridLines(indexes, width)
	start, end := dashboardLineWindow(len(lines), selectedTop, selectedBottom, height)
	return strings.Join(lines[start:end], "\n")
}

func (m Model) dashboardGridWidth() int {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 34 {
		width = 34
	}
	return width
}

func (m Model) dashboardGridHeight() int {
	height := m.height - 4
	if height < 1 {
		height = 1
	}
	return height
}

func (m Model) dashboardGridLines(indexes []int, width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	for i := 0; i < len(indexes); i += cols {
		rowEnd := i + cols
		if rowEnd > len(indexes) {
			rowEnd = len(indexes)
		}
		rowHasNote := false
		for j := i; j < rowEnd; j++ {
			if strings.TrimSpace(indexesHostNote(m.states, indexes[j])) != "" {
				rowHasNote = true
				break
			}
		}
		var row []string
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= len(indexes) {
				row = append(row, padBlock(blankCard(cardWidth, rowHasNote), cardWidth))
				continue
			}
			visibleIndex := i + col
			realIndex := indexes[visibleIndex]
			if visibleIndex == m.selected {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.renderCard(realIndex, visibleIndex == m.selected, cardWidth, rowHasNote), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.selected >= i && m.selected < rowEnd {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func (m Model) renderDashboardList(indexes []int, width int) string {
	if width <= 0 {
		width = contentWidth(m.width)
	}
	height := m.dashboardListHeight()
	start, end := visibleRange(len(indexes), m.selected, height)
	lines := make([]string, 0, height)
	for i := start; i < end; i++ {
		realIndex := indexes[i]
		lines = append(lines, m.dashboardListLine(realIndex, i == m.selected, width))
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardListHeight() int {
	height := m.height - 4
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) dashboardListLine(index int, selected bool, width int) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	prefix := " "
	nameStyle := detailValueStyle
	if selected {
		prefix = "▶"
		nameStyle = blueStyle.Bold(true)
	}
	status := "离线"
	if state.Loading {
		status = "采集"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := 28
	if width < 110 {
		nameWidth = 22
	}
	if width < 78 {
		nameWidth = 16
	}
	name := nameStyle.Render(padVisible(fitANSI(dashboardHostDisplayName(h), nameWidth), nameWidth))
	statusText := padVisible(colorStatus(status, state.Loading, metrics.Online), 6)
	cpu, mem, disk := dashboardListResourceColumns(state)
	containerText, serviceText := dashboardListServiceColumns(metrics)
	expire := padVisible(expireCardTextOrDash(h.ExpireAt), 10)
	addressWidth := 22
	if width < 100 {
		addressWidth = 16
	}
	address := cardMutedStyle.Render(padVisible(fit(h.Address(), addressWidth), addressWidth))
	line := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s  %s  %s", prefix, name, statusText, cpu, mem, disk, containerText, serviceText, expire, address)
	return fitANSI(line, width)
}

func (m Model) renderDashboardGrouped(indexes []int) string {
	width := contentWidth(m.width)
	if width <= 0 {
		width = m.width
	}
	if width < 34 {
		width = 34
	}
	height := m.dashboardGroupedHeight()
	allLines, selectedTop, selectedBottom := m.groupedLines(indexes, width)
	start, end := dashboardLineWindow(len(allLines), selectedTop, selectedBottom, height)
	lines := append([]string{}, allLines[start:end]...)
	return strings.Join(lines, "\n")
}

func (m Model) dashboardGroupedHeight() int {
	height := m.height - 4
	if height < dashboardGroupedCardHeight() {
		height = dashboardGroupedCardHeight()
	}
	return height
}

func (m Model) groupedLines(indexes []int, width int) ([]string, int, int) {
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	lastCategory := ""
	for i, index := range indexes {
		category := strings.TrimSpace(m.states[index].Host.Category)
		if category == "" {
			category = "未分类"
		}
		if i == 0 || category != lastCategory {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.groupedCategoryHeader(category, indexes, width))
			lastCategory = category
		}
		if i == m.selected {
			selectedTop = len(lines)
		}
		cardLines := strings.Split(m.renderGroupedCard(index, i == m.selected, width), "\n")
		lines = append(lines, cardLines...)
		if i == m.selected {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func dashboardGroupedCardHeight() int {
	return 6
}

func (m Model) groupedCategoryHeader(category string, indexes []int, width int) string {
	count := 0
	for _, index := range indexes {
		cat := strings.TrimSpace(m.states[index].Host.Category)
		if cat == "" {
			cat = "未分类"
		}
		if cat == category {
			count++
		}
	}
	countText := fmt.Sprintf("%d台", count)
	nameWidth := width - runewidth.StringWidth(countText) - 2
	if nameWidth < 1 {
		nameWidth = 1
	}
	label := fit(category, nameWidth)
	spaces := width - runewidth.StringWidth(label) - runewidth.StringWidth(countText)
	if spaces < 1 {
		spaces = 1
	}
	return titleStyle.Render(label + strings.Repeat(" ", spaces) + countText)
}

func (m Model) renderGroupedCard(index int, selected bool, width int) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	innerWidth := cardWidth - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	favoriteMark := ""
	if h.Favorite {
		favoriteMark = favoriteStyle.Render("★") + " "
	}
	pinnedMark := ""
	if h.Pinned {
		pinnedMark = pinnedStyle.Render("▲") + " "
	}
	title := pinnedMark + favoriteMark + h.Name
	recentLabel := ""
	if recent := lastLoginCard(m.lastLogin(h)); recent != "" {
		recentLabel = cardMutedStyle.Render(recent)
	}
	uptimeLabel := cardHeaderMeta(h, metrics)
	stateMark := colorStatus("●", state.Loading, metrics.Online)

	userPort := h.User
	if userPort == "" {
		userPort = "-"
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	addressLine := fmt.Sprintf("%s %s:%s", h.Address(), userPort, port)

	barWidth := 8
	cpuLine := groupedMetricText("CPU", metrics.CPUPercent, cpuCoresText(metrics), barWidth, 70, 85)
	memLine := groupedMetricText("内存", metrics.MemPercent(), bytesPair(metrics.MemUsed, metrics.MemTotal), barWidth, 70, 85)
	diskLine := groupedMetricText("磁盘", metrics.DiskPercent(), diskSummaryText(metrics), barWidth, 80, 90)
	loadLine := fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))
	serviceLine := serviceCardText(metrics)
	if riskText := cardRiskText(buildChecks(state), innerWidth); riskText != "" {
		serviceLine += "  " + riskText
	}
	noteLine := groupedNoteText(h.Note)

	lines := []string{groupedCardTopLine(cardWidth, title, recentLabel, uptimeLabel, stateMark, borderStyle)}
	contentParts := []groupedAdaptivePart{
		{Text: cardMutedStyle.Render(addressLine), Width: 26},
		{Text: cpuLine, Width: 24},
		{Text: memLine, Width: 36},
		{Text: diskLine, Width: 36},
		{Text: cardMutedStyle.Render(loadLine), Width: 25},
		{Text: serviceLine, Width: 26},
	}
	if noteLine != "" {
		contentParts = append(contentParts, groupedAdaptivePart{Text: cardMutedStyle.Render(noteLine), Width: 30})
	}
	for _, line := range groupedAdaptiveContentLines(innerWidth, contentParts) {
		lines = append(lines, cardContentLine(cardWidth, line, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func groupedMetricText(label string, value float64, extra string, barWidth int, warn float64, crit float64) string {
	return fmt.Sprintf("%s %s %s",
		cardMutedStyle.Render(label),
		percentBarWidthWithThreshold(value, barWidth, warn, crit),
		cardMutedStyle.Render(emptyDash(extra)),
	)
}

func groupedCardTopLine(width int, title string, middle string, meta string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffixText := dot
	if strings.TrimSpace(meta) != "" && strings.TrimSpace(meta) != "-" {
		suffixText = meta + " " + dot
	}
	suffix := " " + suffixText + " "
	baseWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if baseWidth < 1 {
		baseWidth = 1
	}
	if ansi.StringWidth(title) > baseWidth {
		title = ansi.Truncate(title, baseWidth, "…")
	}
	fillWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if fillWidth < 0 {
		fillWidth = 0
	}
	fill := borderStyle.Render(strings.Repeat("─", fillWidth))
	middle = strings.TrimSpace(middle)
	if middle != "" && fillWidth > ansi.StringWidth(middle)+2 {
		middleWidth := ansi.StringWidth(middle)
		fillStart := ansi.StringWidth(prefix) + ansi.StringWidth(title) + ansi.StringWidth(titleGap)
		targetStart := (innerWidth - middleWidth) / 2
		leftFill := targetStart - fillStart - 1
		if leftFill < 0 {
			leftFill = 0
		}
		if leftFill+middleWidth+2 > fillWidth {
			leftFill = fillWidth - middleWidth - 2
		}
		if leftFill < 0 {
			leftFill = 0
		}
		rightFill := fillWidth - ansi.StringWidth(middle) - 2 - leftFill
		if rightFill < 0 {
			rightFill = 0
		}
		fill = borderStyle.Render(strings.Repeat("─", leftFill)) + " " + middle + " " + borderStyle.Render(strings.Repeat("─", rightFill))
	}
	return left + prefix + title + titleGap + fill + suffix + right
}

func groupedNoteText(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return "备注 " + note
}

type groupedAdaptivePart struct {
	Text  string
	Width int
}

func groupedAdaptiveContentLines(width int, parts []groupedAdaptivePart) []string {
	if width < 1 {
		width = 1
	}
	const minTrailingWidth = 10
	lines := []string{}
	for i := 0; i < len(parts); {
		rowStart := i
		rowWidth := 0
		for i < len(parts) {
			partWidth := parts[i].Width
			if partWidth < 1 {
				partWidth = ansi.StringWidth(strings.TrimSpace(parts[i].Text))
			}
			if partWidth > width {
				partWidth = width
			}
			nextWidth := partWidth
			if i > rowStart {
				nextWidth += 2
			}
			if i > rowStart && rowWidth+nextWidth > width {
				remaining := width - rowWidth - 2
				if remaining >= minTrailingWidth {
					i++
				}
				break
			}
			rowWidth += nextWidth
			i++
		}
		lines = append(lines, groupedAdaptiveLine(width, parts[rowStart:i]))
	}
	return lines
}

func groupedAdaptiveLine(width int, parts []groupedAdaptivePart) string {
	line := ""
	used := 0
	for i, part := range parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		partWidth := part.Width
		if partWidth < 1 {
			partWidth = ansi.StringWidth(text)
		}
		if used > 0 {
			if used+2 >= width {
				break
			}
			line += "  "
			used += 2
		}
		if used+partWidth > width {
			partWidth = width - used
		}
		if i == len(parts)-1 {
			partWidth = width - used
		}
		if partWidth <= 0 {
			break
		}
		tail := ""
		if i == len(parts)-1 {
			tail = "…"
		}
		text = ansi.Truncate(text, partWidth, tail)
		line += padVisible(text, partWidth)
		used += partWidth
	}
	return line
}

func dashboardListResourceColumns(state hostState) (string, string, string) {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return detailValueStyle.Render(padVisible("CPU -", 7)),
			detailValueStyle.Render(padVisible("内存 -", 8)),
			detailValueStyle.Render(padVisible("磁盘 -", 8))
	}
	cpu := "CPU " + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%3.0f%%", metrics.CPUPercent))
	mem := "内存 " + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%3.0f%%", metrics.MemPercent()))
	disk := "磁盘 " + diskMountPercentText(metrics)
	return padVisible(cpu, 7), padVisible(mem, 8), padVisible(disk, 14)
}

func dashboardListServiceColumns(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	containerRaw := fmt.Sprintf("容器 %d/%d/%d", metrics.DockerFailed, metrics.DockerRunning, total)
	if total == 0 {
		containerRaw = "容器 0"
	}
	container := cardMutedStyle.Render("容器 ")
	if metrics.DockerFailed > 0 {
		container += redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	} else if total == 0 {
		container += cardMutedStyle.Render("0")
	} else {
		container += cardMutedStyle.Render(fmt.Sprintf("0/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	service := cardMutedStyle.Render("服务 ") + serviceNumber
	return padVisible(container, maxInt(12, ansi.StringWidth(containerRaw))), padVisible(service, 7)
}

func compactResourceTriplet(state hostState) (string, string, string) {
	metrics := state.Metrics
	if state.Loading || !metrics.Online {
		return cardMutedStyle.Render("CPU") + detailValueStyle.Render("-"),
			cardMutedStyle.Render("内") + detailValueStyle.Render("-"),
			cardMutedStyle.Render("磁") + detailValueStyle.Render("-")
	}
	return cardMutedStyle.Render("CPU") + metricValueStyle(metrics.CPUPercent, 70, 85).Render(fmt.Sprintf("%.0f", metrics.CPUPercent)),
		cardMutedStyle.Render("内") + metricValueStyle(metrics.MemPercent(), 70, 85).Render(fmt.Sprintf("%.0f", metrics.MemPercent())),
		cardMutedStyle.Render("磁") + metricValueStyle(metrics.DiskPercent(), 80, 90).Render(fmt.Sprintf("%.0f", metrics.DiskPercent()))
}

func compactServicePair(metrics monitor.Metrics) (string, string) {
	total := dockerTotal(metrics)
	container := "容器0"
	if total > 0 {
		if metrics.DockerFailed > 0 {
			container = cardMutedStyle.Render("容器") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
		} else {
			container = cardMutedStyle.Render(fmt.Sprintf("容器0/%d/%d", metrics.DockerRunning, total))
		}
	} else {
		container = cardMutedStyle.Render(container)
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	return container, cardMutedStyle.Render("服务") + serviceNumber
}

func compactExpireText(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render("到期-")
	}
	return expireCardText(value)
}

func expireCardTextOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return cardMutedStyle.Render("到期 -")
	}
	return expireCardText(value)
}

func (m Model) renderDashboardCategory(indexes []int) string {
	width := contentWidth(m.width)
	if width <= 0 {
		width = contentWidth(m.width)
	}
	if width < 100 {
		return m.renderDashboardCategoryTop(indexes, width)
	}
	leftWidth := 24
	if width >= 120 {
		leftWidth = 28
	}
	gap := 0
	height := m.dashboardCategoryBodyHeight()
	rightWidth := width - leftWidth - gap
	if rightWidth < 34 {
		return m.renderDashboardCategoryTop(indexes, width)
	}
	left := m.renderDashboardCategoryPane(leftWidth, height)
	right := m.renderDashboardCategoryServerPane(indexes, rightWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
}

func (m Model) renderDashboardCategoryTop(indexes []int, width int) string {
	if width < 34 {
		width = 34
	}
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	bar := m.renderDashboardCategoryTopBar(contentWidth)
	height := m.dashboardCategoryBodyHeight() - 2
	if height < 3 {
		height = 3
	}
	list := m.renderDashboardCategoryServers(indexes, contentWidth, height)
	content := strings.Join([]string{
		bar,
		detailFrameSeparator(contentWidth),
		list,
	}, "\n")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width - 2).
		Render(content)
}

func (m Model) renderDashboardCategoryTopBar(width int) string {
	items := m.dashboardCategoryItems()
	selected := m.dashboardCategorySelectedIndex(items)
	if width < 10 {
		width = 10
	}
	parts := make([]string, 0, len(items))
	for i, item := range items {
		label := fmt.Sprintf("%s %d", item.Label, item.Count)
		if i == selected {
			label = titleStyle.Render(label)
		} else {
			label = mutedStyle.Render(label)
		}
		parts = append(parts, label)
	}
	value := ""
	if len(parts) > 0 {
		value = strings.Join(parts, "  ")
		if ansi.StringWidth(value) > width && selected > 0 {
			value = strings.Join(parts[selected:], "  ")
		}
	}
	return padVisible(fitANSI(value, width), width)
}

func (m Model) dashboardCategoryBodyHeight() int {
	height := m.height - 4
	if height < 5 {
		height = 5
	}
	return height
}

func (m Model) renderDashboardCategoryServers(indexes []int, width int, height int) string {
	if m.dashboardCategoryShowsGroupedServers() {
		return m.renderDashboardCategoryGroupedServers(indexes, width, height)
	}
	start, end := visibleRange(len(indexes), m.selected, height)
	lines := []string{}
	for i := start; i < end; i++ {
		lines = append(lines, padVisible(m.dashboardCategoryServerLine(indexes[i], i == m.selected, width), width))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardCategoryShowsGroupedServers() bool {
	return m.dashboardMode == dashboardCategory && m.filter == filterAll && !m.favoriteOnly && strings.TrimSpace(m.query) == ""
}

func (m Model) renderDashboardCategoryGroupedServers(indexes []int, width int, height int) string {
	allLines, selectedLine := m.dashboardCategoryGroupedServerLines(indexes, width)
	start := selectedLine - height + 1
	if start < 0 {
		start = 0
	}
	if selectedLine < start {
		start = selectedLine
	}
	if start+height > len(allLines) {
		start = len(allLines) - height
		if start < 0 {
			start = 0
		}
	}
	end := start + height
	if end > len(allLines) {
		end = len(allLines)
	}
	lines := append([]string{}, allLines[start:end]...)
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) dashboardCategoryGroupedServerLines(indexes []int, width int) ([]string, int) {
	lines := []string{}
	selectedLine := 0
	lastCategory := ""
	for i, index := range indexes {
		category := strings.TrimSpace(m.states[index].Host.Category)
		if category == "" {
			category = "未分类"
		}
		if i == 0 || category != lastCategory {
			if len(lines) > 0 {
				lines = append(lines, strings.Repeat(" ", width))
			}
			lines = append(lines, m.dashboardCategoryGroupHeader(category, indexes, width))
			lastCategory = category
		}
		if i == m.selected {
			selectedLine = len(lines)
		}
		line := m.dashboardCategoryServerLineWithOptions(index, i == m.selected, width, false, true)
		lines = append(lines, padVisible(fitANSI(line, width), width))
	}
	if len(lines) == 0 {
		return []string{}, 0
	}
	return lines, selectedLine
}

func (m Model) dashboardCategoryGroupHeader(category string, indexes []int, width int) string {
	count := 0
	for _, index := range indexes {
		cat := strings.TrimSpace(m.states[index].Host.Category)
		if cat == "" {
			cat = "未分类"
		}
		if cat == category {
			count++
		}
	}
	countText := fmt.Sprintf("%d台", count)
	nameWidth := width - runewidth.StringWidth(countText) - 2
	if nameWidth < 1 {
		nameWidth = 1
	}
	label := cardMutedStyle.Render(fit(category, nameWidth))
	spaces := width - ansi.StringWidth(label) - runewidth.StringWidth(countText)
	if spaces < 1 {
		spaces = 1
	}
	return padVisible(label+strings.Repeat(" ", spaces)+cardMutedStyle.Render(countText), width)
}

func (m Model) renderDashboardCategoryServerPane(indexes []int, width int, height int) string {
	border := softGray
	styleWidth := width - 2
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(styleWidth).
		Render(m.renderDashboardCategoryServers(indexes, contentWidth, height))
}

func dashboardCategoryNameWidth(width int) int {
	nameWidth := 24
	if width < 92 {
		nameWidth = 20
	}
	if width < 74 {
		nameWidth = 16
	}
	return nameWidth
}

func dashboardCategoryHostName(h host.Host, selected bool, width int, showCategory bool, fixedMarkSlots bool) string {
	marks := ""
	if fixedMarkSlots {
		if h.Pinned {
			marks += pinnedStyle.Render("▲")
		} else {
			marks += " "
		}
		marks += " "
		if h.Favorite {
			marks += favoriteStyle.Render("★")
		} else {
			marks += " "
		}
		marks += " "
	} else {
		if h.Pinned {
			marks += pinnedStyle.Render("▲") + " "
		}
		if h.Favorite {
			marks += favoriteStyle.Render("★") + " "
		}
	}
	category := strings.TrimSpace(h.Category)
	if category == "" {
		category = "未分类"
	}
	categoryText := ""
	if showCategory {
		categoryText = cardMutedStyle.Render("[" + category + "]")
	}
	nameStyle := detailValueStyle
	if selected {
		nameStyle = blueStyle.Bold(true)
	}
	name := strings.TrimSpace(h.Name)
	if name == "" {
		name = h.Address()
	}
	marksWidth := ansi.StringWidth(marks)
	categoryWidth := 0
	if showCategory {
		categoryWidth = runewidth.StringWidth("[" + category + "]")
	}
	nameMinWidth := 8
	if width < marksWidth+categoryWidth+1+nameMinWidth {
		categoryText = ""
		categoryWidth = 0
	}
	nameWidth := width - marksWidth - categoryWidth
	if categoryText != "" {
		nameWidth--
	}
	if nameWidth < 1 {
		nameWidth = 1
	}
	text := marks
	if categoryText != "" {
		text += categoryText + " "
	}
	text += nameStyle.Render(fitANSI(name, nameWidth))
	return padVisible(text, width)
}

func (m Model) dashboardCategoryServerLine(index int, selected bool, width int) string {
	return m.dashboardCategoryServerLineWithOptions(index, selected, width, true, false)
}

func (m Model) dashboardCategoryServerLineWithOptions(index int, selected bool, width int, showCategory bool, fixedMarkSlots bool) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics
	status := "离线"
	if state.Loading {
		status = "采集"
	} else if metrics.Online {
		status = "在线"
	}
	nameWidth := dashboardCategoryNameWidth(width)
	name := dashboardCategoryHostName(h, selected, nameWidth, showCategory, fixedMarkSlots)
	statusText := colorStatus(status, state.Loading, metrics.Online)
	cpu, mem, disk := compactResourceTriplet(state)
	container, service := compactServicePair(metrics)
	timeText := cardHeaderMeta(h, metrics)
	cell := func(value string, cellWidth int) string {
		return padVisible(fitANSI(value, cellWidth), cellWidth)
	}
	fields := []string{
		name,
		cell(statusText, 4),
		cell(cpu, 6),
		cell(mem, 5),
		cell(disk, 5),
		cell(container, 11),
		cell(service, 7),
	}
	fields = append(fields, cell(timeText, 8))
	line := strings.Join(fields, " ")
	used := ansi.StringWidth(line)
	if remaining := width - used - 1; remaining >= 8 {
		line += " " + cell(cardMutedStyle.Render(h.Address()), remaining)
	}
	return fitANSI(line, width)
}

func (m Model) renderDashboardCategoryPane(width int, height int) string {
	items := m.dashboardCategoryItems()
	active := m.dashboardMode == dashboardCategory && m.dashboardFocus == 0
	selected := m.dashboardCategorySelectedIndex(items)
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	lines := []string{titleStyle.Render(fit("分类", contentWidth))}
	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}
	start, end := visibleRange(len(items), selected, listHeight)
	for i := start; i < end; i++ {
		item := items[i]
		prefix := " "
		style := detailValueStyle
		if i == selected {
			prefix = "▶"
			if active {
				style = blueStyle.Bold(true)
			}
		}
		count := mutedStyle.Render(fmt.Sprintf("%d", item.Count))
		countWidth := ansi.StringWidth(count)
		labelWidth := contentWidth - countWidth - 3
		if labelWidth < 4 {
			labelWidth = 4
		}
		label := style.Render(fit(item.Label, labelWidth))
		line := prefix + " " + label
		spaces := contentWidth - ansi.StringWidth(line) - countWidth
		if spaces < 1 {
			spaces = 1
		}
		lines = append(lines, padVisible(line+strings.Repeat(" ", spaces)+count, contentWidth))
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}
	border := softGray
	if active {
		border = blue
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Width(width - 2).Render(strings.Join(lines, "\n"))
}

type dashboardCategoryItem struct {
	Label string
	Kind  string
	Value string
	Count int
}

func (m Model) dashboardCategoryItems() []dashboardCategoryItem {
	items := []dashboardCategoryItem{
		{Label: "全部", Kind: "all", Count: len(m.states)},
	}
	seen := map[string]bool{}
	categories := []string{}
	for _, state := range m.states {
		cat := state.Host.Category
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories)
	for _, category := range categories {
		cat := category
		items = append(items, dashboardCategoryItem{
			Label: category,
			Kind:  "category",
			Value: category,
			Count: m.countHosts(func(state hostState) bool { return state.Host.Category == cat }),
		})
	}
	return items
}

func (m Model) countHosts(match func(hostState) bool) int {
	count := 0
	for _, state := range m.states {
		if match(state) {
			count++
		}
	}
	return count
}

func (m Model) dashboardCategorySelectedIndex(items []dashboardCategoryItem) int {
	for i, item := range items {
		switch item.Kind {
		case "problem":
			if m.filter == filterProblem {
				return i
			}
		case "online":
			if m.filter == filterOnline {
				return i
			}
		case "category":
			if m.filter == filterAll && m.category == item.Value {
				return i
			}
		case "all":
			if m.filter == filterAll && m.category == "" {
				return i
			}
		}
	}
	return 0
}

func (m Model) dashboardCategoryActiveLabel() string {
	items := m.dashboardCategoryItems()
	if len(items) == 0 {
		return "全部"
	}
	index := m.dashboardCategorySelectedIndex(items)
	if index < 0 || index >= len(items) {
		return "全部"
	}
	return items[index].Label
}

func (m *Model) applyDashboardCategoryItem(item dashboardCategoryItem) {
	m.favoriteOnly = false
	m.filter = filterAll
	m.category = ""
	switch item.Kind {
	case "problem":
		m.filter = filterProblem
	case "online":
		m.filter = filterOnline
	case "category":
		m.category = item.Value
	}
	m.selected = 0
}

func indexesHostNote(states []hostState, index int) string {
	if index < 0 || index >= len(states) {
		return ""
	}
	return states[index].Host.Note
}

func (m Model) dashboardPageDots(indexes []int) string {
	if len(indexes) == 0 {
		return ""
	}
	lines, selectedTop, selectedBottom := m.dashboardGridLines(indexes, m.dashboardGridWidth())
	height := m.dashboardGridHeight()
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, height, m.width)
}

func (m Model) dashboardGroupedDots(indexes []int) string {
	if len(indexes) == 0 {
		return ""
	}
	width := contentWidth(m.width)
	if width <= 0 {
		width = m.width
	}
	if width < 34 {
		width = 34
	}
	lines, selectedTop, selectedBottom := m.groupedLines(indexes, width)
	height := m.dashboardGroupedHeight()
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, height, m.width)
}

func dashboardLineWindow(totalLines int, selectedTop int, selectedBottom int, height int) (int, int) {
	if height <= 0 {
		return 0, 0
	}
	start := selectedBottom - height
	if start < 0 {
		start = 0
	}
	if selectedTop < start {
		start = selectedTop
	}
	if start+height > totalLines {
		start = totalLines - height
		if start < 0 {
			start = 0
		}
	}
	end := start + height
	if end > totalLines {
		end = totalLines
	}
	return start, end
}

func dashboardLineDots(totalLines int, selectedTop int, selectedBottom int, height int, width int) string {
	if height <= 0 || totalLines <= 0 {
		return ""
	}
	totalPages := (totalLines + height - 1) / height
	if totalPages <= 1 {
		return ""
	}
	_, windowEnd := dashboardLineWindow(totalLines, selectedTop, selectedBottom, height)
	currentPage := (windowEnd - 1) / height
	if currentPage >= totalPages {
		currentPage = totalPages - 1
	}
	if currentPage < 0 {
		currentPage = 0
	}
	start := 0
	dotCount := totalPages
	showNumber := false
	if totalPages > 9 {
		dotCount = 7
		showNumber = true
		start = currentPage - dotCount/2
		if start < 0 {
			start = 0
		}
		if start+dotCount > totalPages {
			start = totalPages - dotCount
		}
	}
	parts := make([]string, 0, dotCount+1)
	for i := 0; i < dotCount; i++ {
		page := start + i
		dot := cardBorderStyle.Render("●")
		if page == currentPage {
			dot = titleStyle.Render("●")
		}
		parts = append(parts, dot)
	}
	if showNumber {
		parts = append(parts, mutedStyle.Render(fmt.Sprintf("%d/%d", currentPage+1, totalPages)))
	}
	line := strings.Join(parts, " ")
	if width <= 0 {
		width = 80
	}
	padding := (width - ansi.StringWidth(line)) / 2
	if padding < 0 {
		padding = 0
	}
	return strings.Repeat(" ", padding) + line
}

func padBlock(block string, width int) string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth > width {
			lines[i] = ansi.Truncate(lines[i], width, "")
			lineWidth = ansi.StringWidth(lines[i])
		}
		if lineWidth < width {
			lines[i] += strings.Repeat(" ", width-lineWidth)
		}
	}
	return strings.Join(lines, "\n")
}

func distributeWidths(totalWidth, cols int) []int {
	if cols <= 0 {
		return []int{totalWidth}
	}
	base := totalWidth / cols
	remainder := totalWidth % cols
	widths := make([]int, cols)
	for i := 0; i < cols; i++ {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
		if widths[i] < 34 {
			widths[i] = 34
		}
	}
	return widths
}

func (m Model) dashboardColumns() int {
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
	}
	switch {
	case width >= 190:
		return 5
	case width >= 148:
		return 4
	case width >= 108:
		return 3
	case width >= 72:
		return 2
	default:
		return 1
	}
}

func withVerticalNav(content string, totalWidth, totalItems, cols, startRow, rowsVisible int) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}
	targetWidth := totalWidth - 1
	if targetWidth < 1 {
		targetWidth = 1
	}
	totalRows := (totalItems + cols - 1) / cols
	height := len(lines)
	track := make([]string, height)
	for i := range track {
		track[i] = " "
	}
	if totalRows <= rowsVisible {
		for i := range track {
			track[i] = cardBorderStyle.Render("▌")
		}
	} else {
		thumbHeight := height * rowsVisible / totalRows
		if thumbHeight < 1 {
			thumbHeight = 1
		}
		if thumbHeight > height {
			thumbHeight = height
		}
		maxStart := height - thumbHeight
		thumbStart := startRow * maxStart / (totalRows - rowsVisible)
		for i := thumbStart; i < thumbStart+thumbHeight && i < height; i++ {
			track[i] = cardBorderStyle.Render("▌")
		}
	}
	for i := range lines {
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth > targetWidth {
			lines[i] = ansi.Truncate(lines[i], targetWidth, "")
			lineWidth = ansi.StringWidth(lines[i])
		}
		if lineWidth < targetWidth {
			lines[i] += strings.Repeat(" ", targetWidth-lineWidth)
		}
		if track[i] != " " {
			lines[i] += track[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCard(index int, selected bool, width int, reserveNoteLine bool) string {
	state := m.states[index]
	h := state.Host
	metrics := state.Metrics

	innerWidth := width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	category := h.Category
	if category == "" {
		category = "未分类"
	}
	favoriteMark := ""
	if h.Favorite {
		favoriteMark = favoriteStyle.Render("★") + " "
	}
	pinnedMark := ""
	if h.Pinned {
		pinnedMark = pinnedStyle.Render("▲") + " "
	}
	prefixMarks := pinnedMark + favoriteMark
	categoryLabel := "[" + category + "]"
	titleText := prefixMarks + categoryLabel + " " + h.Name
	if ansi.StringWidth(titleText) > innerWidth {
		prefixMarks = ""
		titleText = categoryLabel + " " + h.Name
	}
	barWidth := 12
	if innerWidth < 42 {
		barWidth = 8
	}
	cpu := percentBarWidth(metrics.CPUPercent, barWidth)
	mem := percentBarWidth(metrics.MemPercent(), barWidth)
	disk := percentBarWidthWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)

	cpuLine := cardMetricLine("CPU", cpu, cpuCoresText(metrics), innerWidth)
	memLine := cardMetricLine("内存", mem, bytesPair(metrics.MemUsed, metrics.MemTotal), innerWidth)
	diskLine := cardMetricLine(diskMetricLabel(metrics), disk, bytesPair(metrics.DiskUsed, metrics.DiskTotal), innerWidth)
	uptimeLabel := cardHeaderMeta(h, metrics)
	loadLine := fit(fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15)), innerWidth)
	serviceLine := ansi.Truncate(serviceCardText(metrics), innerWidth, "…")
	riskText := cardRiskText(buildChecks(state), innerWidth)
	if riskText != "" {
		serviceLine = ansi.Truncate(serviceLine+"  "+riskText, innerWidth, "…")
	}
	title := titleText

	cardWidth := width
	if cardWidth < 34 {
		cardWidth = 34
	}
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	userPort := h.User
	if userPort == "" {
		userPort = "-"
	}
	port := h.Port
	if port == "" {
		port = "22"
	}
	userPort += ":" + port
	addressText := fmt.Sprintf("%s %s", h.Address(), userPort)
	if recent := lastLoginCard(m.lastLogin(h)); recent != "" {
		addressText += "  " + recent
	}
	addressLine := fit(addressText, innerWidth)
	noteLine := cardNoteText(h.Note, innerWidth)
	stateMark := colorStatus("●", state.Loading, metrics.Online)
	lines := []string{
		cardTopLine(cardWidth, title, uptimeLabel, stateMark, borderStyle),
		cardMutedContentLine(cardWidth, addressLine, borderStyle),
		cardContentLine(cardWidth, cpuLine, borderStyle),
		cardContentLine(cardWidth, memLine, borderStyle),
		cardContentLine(cardWidth, diskLine, borderStyle),
		cardInnerSeparatorLine(cardWidth, borderStyle),
		cardMutedContentLine(cardWidth, loadLine, borderStyle),
		cardContentLine(cardWidth, serviceLine, borderStyle),
	}
	if noteLine != "" || reserveNoteLine {
		lines = append(lines, cardMutedContentLine(cardWidth, noteLine, borderStyle))
	}
	lines = append(lines, cardBottomLine(cardWidth, borderStyle))
	return strings.Join(lines, "\n")
}

func blankCard(width int, reserveNoteLine bool) string {
	innerWidth := width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	height := dashboardCardInnerHeight
	if reserveNoteLine {
		height++
	}
	return lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder()).
		Padding(0, 1).
		Width(innerWidth).
		Height(height).
		Render("")
}

func percentBar(value float64) string {
	return percentBarWidth(value, 8)
}

func percentBarWithThreshold(value float64, warn float64, crit float64) string {
	return percentBarWidthWithThreshold(value, 8, warn, crit)
}

func metricLine(label, value string) string {
	const labelWidth = 5
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return cardMutedStyle.Render(label) + strings.Repeat(" ", padding) + value
}

func cardMetricLine(label string, base string, extra string, width int) string {
	return metricLine(label, compactCardMetric(label, base, extra, width))
}

func compactCardMetric(label string, base string, extra string, width int) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	if extra == "" || extra == "-" {
		return base
	}
	full := base + "  " + cardMutedStyle.Render(extra)
	if ansi.StringWidth(metricLine(label, full)) <= width {
		return full
	}
	return base
}

func threeMetricLine(width int, metrics monitor.Metrics) string {
	gap := 1
	colWidth := (width - gap*2) / 3
	if colWidth < 8 {
		colWidth = 8
	}
	barWidth := 4
	if colWidth >= 12 {
		barWidth = 5
	}
	if colWidth >= 15 {
		barWidth = 6
	}
	cpu := compactMetric("CPU", metrics.CPUPercent, colWidth, barWidth)
	mem := compactMetric("内存", metrics.MemPercent(), colWidth, barWidth)
	disk := compactDiskMetric(metrics, colWidth, barWidth)
	line := padVisible(cpu, colWidth) + strings.Repeat(" ", gap) + padVisible(mem, colWidth) + strings.Repeat(" ", gap) + padVisible(disk, colWidth)
	return fit(line, width)
}

func compactMetric(label string, value float64, width int, barWidth int) string {
	return compactMetricWithThreshold(label, value, width, barWidth, 70, 85)
}

func compactMetricWithThreshold(label string, value float64, width int, barWidth int, warn float64, crit float64) string {
	bar := compactPercentBarWithThreshold(value, barWidth, warn, crit)
	labelWidth := 4
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+bar, width)
}

func compactDiskMetric(metrics monitor.Metrics, width int, barWidth int) string {
	label := diskMountLabel(metrics)
	if label == "-" {
		label = "磁盘"
	}
	bar := compactPercentBarWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)
	return fit(label+" "+bar, width)
}

func compactPercentBar(value float64, total int) string {
	return compactPercentBarWithThreshold(value, total, 70, 85)
}

func compactPercentBarWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func padVisible(s string, width int) string {
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "")
	}
	if ansi.StringWidth(s) < width {
		s += strings.Repeat(" ", width-ansi.StringWidth(s))
	}
	return s
}

func cardTopLine(width int, title string, meta string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffixText := dot
	if strings.TrimSpace(meta) != "" && strings.TrimSpace(meta) != "-" {
		suffixText = meta + " " + dot
	}
	suffix := " " + suffixText + " "
	fillWidth := innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	if fillWidth < 1 {
		title = ansi.Truncate(title, innerWidth-ansi.StringWidth(prefix)-ansi.StringWidth(titleGap)-ansi.StringWidth(suffix)-1, "…")
		fillWidth = innerWidth - ansi.StringWidth(prefix) - ansi.StringWidth(title) - ansi.StringWidth(titleGap) - ansi.StringWidth(suffix)
	}
	if fillWidth < 0 {
		fillWidth = 0
	}
	return left + prefix + title + titleGap + borderStyle.Render(strings.Repeat("─", fillWidth)) + suffix + right
}

func cardNoteText(note string, width int) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return fit("备注 "+note, width)
}

func cardHeaderMeta(h host.Host, metrics monitor.Metrics) string {
	if strings.TrimSpace(h.ExpireAt) != "" {
		return expireCardText(h.ExpireAt)
	}
	return cardMutedStyle.Render(cardUptimeShort(metrics.Uptime))
}

func expireCardText(value string) string {
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("到期格式错")
	}
	switch {
	case days < 0:
		return redStyle.Render("已过期")
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("到期%d天", days))
	default:
		return cardMutedStyle.Render(fmt.Sprintf("到期%d天", days))
	}
}

func expireDetailText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("格式错误")
	}
	switch {
	case days < 0:
		return redStyle.Render(fmt.Sprintf("已过期%d天", -days))
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("剩余%d天", days))
	default:
		return fmt.Sprintf("剩余%d天", days)
	}
}

func expireDays(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	expire, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return 0, false
	}
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return int(expire.Sub(today).Hours() / 24), true
}

func cardContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := padVisible(content, contentWidth)
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardMutedContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := cardMutedStyle.Render(padVisible(content, contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardInnerSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	line := cardBorderStyle.Render(dashedLine(contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func dashedLine(width int) string {
	if width <= 0 {
		return ""
	}
	pattern := "- "
	line := strings.Repeat(pattern, (width+len(pattern)-1)/len(pattern))
	return ansi.Truncate(line, width, "")
}

func cardSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("├") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("┤")
}

func cardBottomLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	return borderStyle.Render("╰") + borderStyle.Render(strings.Repeat("─", innerWidth)) + borderStyle.Render("╯")
}

func statusDot(loading bool, online bool) string {
	if loading {
		return "●"
	}
	if online {
		return "●"
	}
	return "●"
}

func percentBarWidth(value float64, total int) string {
	return percentBarWidthWithThreshold(value, total, 70, 85)
}

func percentBarWidthWithThreshold(value float64, total int, warn float64, crit float64) string {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	if total < 3 {
		total = 3
	}
	filled := int(value / 100 * float64(total))
	if value > 0 && filled == 0 {
		filled = 1
	}
	style := metricValueStyle(value, warn, crit)
	bar := style.Render(strings.Repeat("▰", filled)) + barEmptyStyle.Render(strings.Repeat("▱", total-filled))
	return fmt.Sprintf("%s %s", bar, style.Render(fmt.Sprintf("%3.0f%%", value)))
}

func metricValueStyle(value float64, warn float64, crit float64) lipgloss.Style {
	if value >= crit {
		return redStyle
	}
	if value >= warn {
		return yellowStyle
	}
	return greenStyle
}

func colorStatus(value string, loading bool, online bool) string {
	if loading {
		return yellowStyle.Render(value)
	}
	if online {
		return greenStyle.Render(value)
	}
	return redStyle.Render(value)
}

func contentWidth(width int) int {
	if width <= 0 {
		return 100
	}
	return width
}

func detailFrameWidth(width int) int {
	if width <= 0 {
		return 100
	}
	if width < 44 {
		return 42
	}
	return width - 2
}

func formContentWidth(width int) int {
	if width <= 0 {
		return 100
	}
	if width < 44 {
		return 42
	}
	return width - 4
}

func stringChoices(values []string, dirs bool) []choice {
	out := make([]choice, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		label := value
		if dirs {
			label = "[目录] " + value
		}
		out = append(out, choice{Label: label, Value: value, IsDir: dirs})
	}
	return out
}

func localItemChoices(items []fsselect.Item) []choice {
	return itemChoices(items)
}

func itemChoices(items []fsselect.Item) []choice {
	out := make([]choice, 0, len(items))
	for _, item := range items {
		kind := "[文件] "
		if item.IsDir {
			kind = "[目录] "
		}
		out = append(out, choice{
			Label: kind + item.Path,
			Value: item.Path,
			IsDir: item.IsDir,
		})
	}
	return out
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func yesNo(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func authText(h host.Host) string {
	hasKey := strings.TrimSpace(h.IdentityFile) != ""
	hasPassword := h.HasPassword || strings.TrimSpace(h.Password) != ""
	switch {
	case hasKey && hasPassword:
		return "密钥：" + filepath.Base(h.IdentityFile) + "，密码"
	case hasKey:
		return "密钥：" + filepath.Base(h.IdentityFile)
	case hasPassword:
		return "密码"
	default:
		return "系统 SSH 默认"
	}
}

func jumpDetailText(h host.Host) string {
	if !h.JumpEnabled {
		return "未启用"
	}
	if strings.TrimSpace(h.JumpHostRef) != "" {
		return h.JumpHostRef + "，仅转发，本地密钥认证"
	}
	port := strings.TrimSpace(h.JumpPort)
	if port == "" {
		port = "22"
	}
	target := h.JumpHost
	if strings.TrimSpace(h.JumpUser) != "" {
		target = h.JumpUser + "@" + target
	}
	return target + ":" + port + "，仅转发，本地密钥认证"
}

func jumpKeyText(h host.Host) string {
	if !h.JumpEnabled {
		return "-"
	}
	if strings.TrimSpace(h.JumpKeyPath) == "" {
		return "系统 SSH 默认"
	}
	return filepath.Base(h.JumpKeyPath) + "（本地）"
}

func transferErrorText(err error, output string) string {
	text := cleanTransferOutput(output)
	if text != "" {
		return text
	}
	if err == nil {
		return "未知错误"
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("命令退出码 %d", exitErr.ExitCode())
	}
	return err.Error()
}

func cleanTransferOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "** WARNING:") ||
			strings.HasPrefix(line, "** This session") ||
			strings.HasPrefix(line, "** The server") {
			continue
		}
		if rsyncProgressText(line) != "" {
			continue
		}
		return line
	}
	return ""
}

func sectionTitle(value string) string {
	return detailSectionStyle.Render("[" + value + "]")
}

func detailSubTitle(value string) string {
	return detailSubTitleStyle.Render("· " + value)
}

func detailSuccessSubTitle(value string) string {
	return detailSuccessStyle.Render("· " + value)
}

func detailDangerSubTitle(value string) string {
	return detailDangerStyle.Render("· " + value)
}

func (m Model) detailRow(label, value string) string {
	const labelWidth = 10
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	continuationPrefix := strings.Repeat(" ", labelWidth)
	valueWidth := m.detailContentWidth() - labelWidth
	if valueWidth < 12 {
		valueWidth = 12
	}
	parts := wrapDetailValue(value, valueWidth)
	if len(parts) == 0 {
		return prefix
	}
	lines := make([]string, 0, len(parts))
	lines = append(lines, prefix+detailValue(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, continuationPrefix+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func detailValue(value string) string {
	if strings.Contains(value, "\x1b[") {
		return value
	}
	return detailValueStyle.Render(value)
}

func (m Model) detailContentWidth() int {
	width := detailFrameWidth(m.width) - 6
	if width < 24 {
		width = 24
	}
	return width
}

func wrapDetailValue(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	if ansi.StringWidth(value) <= width {
		return []string{value}
	}
	if strings.Contains(value, "\x1b") {
		return []string{ansi.Truncate(value, width, "…")}
	}
	var lines []string
	current := ""
	for _, token := range splitWrapTokens(value) {
		if current == "" {
			current = token
			continue
		}
		if ansi.StringWidth(current+token) <= width {
			current += token
			continue
		}
		lines = appendWrappedLine(lines, current, width)
		current = strings.TrimLeft(token, " ")
	}
	if current != "" {
		lines = appendWrappedLine(lines, current, width)
	}
	return lines
}

func wrapPlainLine(value string, width int) string {
	return strings.Join(wrapDetailValue(value, width), "\n")
}

func renderHelp(width int, value string) string {
	return helpStyle.Render(fit(value, width))
}

func splitWrapTokens(value string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range value {
		current.WriteRune(r)
		if r == ',' || r == '/' || r == ' ' {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func appendWrappedLine(lines []string, value string, width int) []string {
	value = strings.TrimSpace(value)
	for ansi.StringWidth(value) > width {
		runes := []rune(value)
		cut := 0
		for cut < len(runes) && runewidth.StringWidth(string(runes[:cut+1])) <= width {
			cut++
		}
		if cut <= 0 {
			cut = 1
		}
		lines = append(lines, string(runes[:cut]))
		value = strings.TrimSpace(string(runes[cut:]))
	}
	if value != "" {
		lines = append(lines, value)
	}
	return lines
}

func failedServiceText(metrics monitor.Metrics, limit int) string {
	if metrics.FailedServices <= 0 {
		return "0"
	}
	if len(metrics.FailedUnits) == 0 {
		return fmt.Sprintf("%d", metrics.FailedServices)
	}
	if limit <= 0 || limit > len(metrics.FailedUnits) {
		limit = len(metrics.FailedUnits)
	}
	names := append([]string{}, metrics.FailedUnits[:limit]...)
	if len(metrics.FailedUnits) > limit {
		names = append(names, fmt.Sprintf("等%d个", metrics.FailedServices))
	}
	return fmt.Sprintf("%d（%s）", metrics.FailedServices, strings.Join(names, "、"))
}

func dockerTotal(metrics monitor.Metrics) int {
	if metrics.DockerTotal > 0 {
		return metrics.DockerTotal
	}
	return metrics.DockerRunning
}

func dockerRunningText(metrics monitor.Metrics, limit int) string {
	if metrics.DockerRunning <= 0 {
		return "-"
	}
	if len(metrics.DockerRunningNames) == 0 {
		return fmt.Sprintf("%d", metrics.DockerRunning)
	}
	if limit <= 0 || limit > len(metrics.DockerRunningNames) {
		limit = len(metrics.DockerRunningNames)
	}
	names := append([]string{}, metrics.DockerRunningNames[:limit]...)
	if len(metrics.DockerRunningNames) > limit {
		names = append(names, fmt.Sprintf("等%d个", metrics.DockerRunning))
	}
	return strings.Join(names, "、")
}

func dockerStoppedText(metrics monitor.Metrics, limit int) string {
	return limitedDockerNames(metrics.DockerStoppedNames, metrics.DockerStopped, limit)
}

func dockerFailedText(metrics monitor.Metrics, limit int) string {
	return limitedDockerNames(metrics.DockerFailedNames, metrics.DockerFailed, limit)
}

func limitedDockerNames(names []string, count int, limit int) string {
	if count <= 0 {
		return "-"
	}
	if len(names) == 0 {
		return fmt.Sprintf("%d", count)
	}
	if limit <= 0 || limit > len(names) {
		limit = len(names)
	}
	out := append([]string{}, names[:limit]...)
	if len(names) > limit {
		out = append(out, fmt.Sprintf("等%d个", count))
	}
	return strings.Join(out, "、")
}

func dockerDetailRows(m Model, metrics monitor.Metrics, state hostState) []string {
	total := dockerTotal(metrics)
	if len(state.ContainerDetails) > 0 {
		total = len(state.ContainerDetails)
	}
	lines := []string{}
	if total == 0 {
		lines = append(lines, m.detailRow("状态", "未发现"))
	} else {
		running, stopped, failed := containerDetailCounts(state.ContainerDetails)
		if len(state.ContainerDetails) == 0 && (metrics.DockerRunning > 0 || metrics.DockerStopped > 0 || metrics.DockerFailed > 0) {
			running = metrics.DockerRunning
			stopped = metrics.DockerStopped
			failed = metrics.DockerFailed
		}
		lines = append(lines,
			m.detailRow("总数", fmt.Sprintf("%d", total)),
			m.detailRow("运行", fmt.Sprintf("%d", running)),
			m.detailRow("停止", fmt.Sprintf("%d", stopped)),
			m.detailRow("故障", fmt.Sprintf("%d", failed)),
		)
	}
	return lines
}

func portDetailRows(m Model, state hostState) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(state.PortDetailsError) != "" {
		return []string{m.detailRow("状态", redStyle.Render(state.PortDetailsError))}
	}
	if len(state.PortDetails) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	groups := groupedPortDetails(state.PortDetails)
	lines := []string{}
	groupDefs := []struct {
		Title string
		Key   string
	}{
		{"系统端口", "system"},
		{"Docker端口", "docker"},
		{"应用端口", "app"},
	}
	first := true
	for _, group := range groupDefs {
		items := groups[group.Key]
		if len(items) == 0 {
			continue
		}
		if !first {
			lines = append(lines, "")
		}
		first = false
		lines = append(lines, detailSubTitle(fmt.Sprintf("%s %d", group.Title, len(items))))
		lines = append(lines, portDetailItemRows(m, items)...)
	}
	return lines
}

func portDetailItemRows(m Model, items []portDetail) []string {
	labelWidth := len("tcp/10000")
	processWidth := len("docker-proxy")
	for _, item := range items {
		label := strings.TrimSpace(item.Protocol + "/" + item.Port)
		if width := runewidth.StringWidth(label); width > labelWidth {
			labelWidth = width
		}
		process := portProcessText(item)
		if process == "" {
			process = "-"
		}
		if width := runewidth.StringWidth(process); width > processWidth {
			processWidth = width
		}
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		process := portProcessText(item)
		if process == "" {
			process = "-"
		}
		pid := item.PID
		if pid == "" {
			pid = "-"
		}
		if item.Count > 1 {
			pid = fmt.Sprintf("%s 等%d个", pid, item.Count)
		}
		label := strings.TrimSpace(item.Protocol + "/" + item.Port)
		processPadding := processWidth - runewidth.StringWidth(process) + 2
		if processPadding < 1 {
			processPadding = 1
		}
		value := process + strings.Repeat(" ", processPadding) + "pid:" + pid
		lines = append(lines, detailAlignedRow(m, label, value, labelWidth))
	}
	return lines
}

func groupedPortDetails(items []portDetail) map[string][]portDetail {
	groups := map[string][]portDetail{"system": {}, "docker": {}, "app": {}}
	for _, item := range items {
		group := portDetailGroup(item)
		groups[group] = append(groups[group], item)
	}
	return groups
}

func portDetailGroup(item portDetail) string {
	if strings.TrimSpace(item.Container) != "" || strings.TrimSpace(item.Process) == "docker-proxy" {
		return "docker"
	}
	port, _ := strconv.Atoi(item.Port)
	if port > 0 && port < 1024 {
		return "system"
	}
	return "app"
}

func portProcessText(item portDetail) string {
	container := strings.TrimSpace(item.Container)
	if container != "" && strings.TrimSpace(item.Process) == "docker-proxy" {
		return container
	}
	return strings.TrimSpace(item.Process)
}

func detailAlignedRow(m Model, label, value string, labelWidth int) string {
	padding := labelWidth - runewidth.StringWidth(label) + 2
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	continuationPrefix := strings.Repeat(" ", labelWidth+2)
	valueWidth := m.detailContentWidth() - labelWidth - 2
	if valueWidth < 12 {
		valueWidth = 12
	}
	parts := wrapDetailValue(value, valueWidth)
	if len(parts) == 0 {
		return prefix
	}
	lines := make([]string, 0, len(parts))
	lines = append(lines, prefix+detailValue(parts[0]))
	for _, part := range parts[1:] {
		lines = append(lines, continuationPrefix+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func containerDetailRows(m Model, state hostState) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(state.ContainerError) != "" {
		return []string{m.detailRow("状态", redStyle.Render(state.ContainerError))}
	}
	if len(state.ContainerDetails) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	lines := []string{}
	groups := []struct {
		Title string
		Kind  string
		Style lipgloss.Style
	}{
		{"故障", "failed", detailDangerStyle},
		{"运行", "running", detailSubTitleStyle},
		{"停止", "stopped", detailSubTitleStyle},
	}
	firstGroup := true
	for _, group := range groups {
		items := filterContainersByKind(state.ContainerDetails, group.Kind)
		if len(items) == 0 {
			continue
		}
		if !firstGroup {
			lines = append(lines, "")
		}
		firstGroup = false
		lines = append(lines, group.Style.Render(fmt.Sprintf("· %s %d", group.Title, len(items))))
		nameWidth := containerNameWidth(items)
		for i, item := range items {
			lines = append(lines, containerDetailItemRows(m, item, nameWidth, i+1)...)
		}
	}
	return lines
}

func containerNameWidth(items []containerDetail) int {
	width := 10
	for _, item := range items {
		if w := runewidth.StringWidth(item.Name); w > width {
			width = w
		}
	}
	if width > 28 {
		width = 28
	}
	return width
}

func containerDetailItemRows(m Model, item containerDetail, nameWidth int, index int) []string {
	status := item.Status
	if status == "" {
		status = "-"
	}
	ports := item.Ports
	state := coloredContainerStatus(containerStatusSummary(status), containerDetailKind(item))
	prefix := detailLabelStyle.Render(fmt.Sprintf("%02d  ", index))
	name := detailValueStyle.Render(padVisible(fit(item.Name, nameWidth), nameWidth))
	line := fitANSI(prefix+name+"  "+state, m.detailContentWidth())
	lines := []string{line}
	indent := strings.Repeat(" ", 4)
	if strings.TrimSpace(item.Image) != "" {
		lines = append(lines, containerIndentedLine(m, indent, "镜像", item.Image))
	}
	if simplified := simplifyDockerPorts(ports); simplified != "" {
		lines = append(lines, containerIndentedLine(m, indent, "端口", simplified))
	}
	return lines
}

func containerIndentedLine(m Model, indent string, label string, value string) string {
	prefixText := indent + label + " "
	prefix := detailLabelStyle.Render(prefixText)
	width := m.detailContentWidth() - ansi.StringWidth(prefixText)
	if width < 12 {
		width = 12
	}
	parts := wrapDetailValue(value, width)
	if len(parts) == 0 {
		return prefix
	}
	lines := []string{prefix + detailValue(parts[0])}
	continuation := strings.Repeat(" ", ansi.StringWidth(prefixText))
	for _, part := range parts[1:] {
		lines = append(lines, continuation+detailValue(part))
	}
	return strings.Join(lines, "\n")
}

func containerStatusSummary(status string) string {
	raw := strings.TrimSpace(status)
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "unhealthy"):
		return strings.TrimSpace("异常 " + dockerStatusAge(raw, "Up"))
	case strings.HasPrefix(lower, "up "):
		age := dockerStatusAge(raw, "Up")
		if strings.Contains(lower, "healthy") {
			return strings.TrimSpace("健康 " + age)
		}
		return strings.TrimSpace("运行 " + age)
	case strings.HasPrefix(lower, "restarting"):
		return strings.TrimSpace("重启中 " + dockerStatusAgo(raw))
	case strings.HasPrefix(lower, "exited"):
		return strings.TrimSpace("退出 " + dockerStatusAgo(raw))
	case strings.HasPrefix(lower, "created"):
		return strings.TrimSpace("已创建 " + dockerStatusAgo(raw))
	default:
		return raw
	}
}

func dockerStatusAge(status string, prefix string) string {
	status = strings.TrimSpace(status)
	status = strings.TrimPrefix(status, prefix)
	if idx := strings.Index(status, "("); idx >= 0 {
		status = status[:idx]
	}
	return shortDockerDuration(status)
}

func dockerStatusAgo(status string) string {
	status = strings.TrimSpace(status)
	if idx := strings.LastIndex(status, ")"); idx >= 0 && idx < len(status)-1 {
		status = status[idx+1:]
	}
	status = strings.TrimSuffix(strings.TrimSpace(status), "ago")
	return shortDockerDuration(status)
}

func shortDockerDuration(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Created ")
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return value
	}
	unit := fields[1]
	switch {
	case strings.HasPrefix(unit, "second"):
		unit = "秒"
	case strings.HasPrefix(unit, "minute"):
		unit = "分"
	case strings.HasPrefix(unit, "hour"):
		unit = "时"
	case strings.HasPrefix(unit, "day"):
		unit = "天"
	case strings.HasPrefix(unit, "week"):
		unit = "周"
	case strings.HasPrefix(unit, "month"):
		unit = "月"
	case strings.HasPrefix(unit, "year"):
		unit = "年"
	}
	return fields[0] + unit
}

func simplifyDockerPorts(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		text := simplifyDockerPort(strings.TrimSpace(part))
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return strings.Join(out, ", ")
}

func simplifyDockerPort(value string) string {
	if value == "" {
		return ""
	}
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return value
	}
	hostPort := portFromAddress(left)
	if hostPort == "" {
		return value
	}
	host := dockerPortHost(left)
	target := strings.TrimSpace(right)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return hostPort + "->" + target
	}
	return host + ":" + hostPort + "->" + target
}

func dockerPortHost(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") {
		if idx := strings.LastIndex(value, "]:"); idx >= 0 {
			return strings.Trim(value[:idx+1], "[]")
		}
	}
	if idx := strings.LastIndex(value, ":"); idx >= 0 {
		return value[:idx]
	}
	return ""
}

func coloredContainerStatus(status string, kind string) string {
	switch kind {
	case "failed":
		return redStyle.Render(status)
	default:
		return detailValueStyle.Render(status)
	}
}

func filterContainersByKind(items []containerDetail, kind string) []containerDetail {
	out := []containerDetail{}
	for _, item := range items {
		if containerDetailKind(item) == kind {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a := strings.ToLower(out[i].Name)
		b := strings.ToLower(out[j].Name)
		if a == b {
			return strings.ToLower(out[i].Image) < strings.ToLower(out[j].Image)
		}
		return a < b
	})
	return out
}

func containerDetailKind(item containerDetail) string {
	status := strings.ToLower(item.Status)
	switch {
	case strings.Contains(status, "restarting") || strings.Contains(status, "dead") || strings.Contains(status, "unhealthy"):
		return "failed"
	case strings.HasPrefix(status, "up "):
		return "running"
	default:
		return "stopped"
	}
}

func containerDetailCounts(items []containerDetail) (int, int, int) {
	running := 0
	stopped := 0
	failed := 0
	for _, item := range items {
		switch containerDetailKind(item) {
		case "running":
			running++
		case "failed":
			failed++
		default:
			stopped++
		}
	}
	return running, stopped, failed
}

func loginSummaryDetailRows(m Model, loading bool, summary []string, errText string, danger bool) []string {
	if loading {
		return []string{m.detailRow("状态", "加载中")}
	}
	if strings.TrimSpace(errText) != "" {
		return []string{m.detailRow("状态", redStyle.Render(errText))}
	}
	if len(summary) == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	lines := make([]string, 0, len(summary))
	for _, line := range summary {
		label, value, ok := strings.Cut(line, "\t")
		if !ok {
			label = "记录"
			value = line
		}
		if danger && label == "统计" {
			value = redStyle.Render(value)
		}
		lines = append(lines, m.detailRow(label, value))
	}
	return lines
}

func checkSuggestionRows(m Model, state hostState, checks []checkItem) []string {
	if state.LoginLoading {
		return []string{m.detailRow("状态", "检查中")}
	}
	rows := make([]string, 0, len(checks))
	for _, check := range checks {
		if check.Level == "正常" {
			continue
		}
		rows = append(rows, m.detailRow(check.Level, styleCheck(check.Level, check.Text)))
	}
	if len(rows) == 0 {
		rows = append(rows, m.detailRow("正常", "未发现明显风险"))
	}
	return rows
}

func riskSummaryText(checks []checkItem) string {
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 && counts["提示"] == 0 {
		return greenStyle.Render("正常")
	}
	parts := []string{}
	if counts["严重"] > 0 {
		parts = append(parts, redStyle.Render(fmt.Sprintf("严重%d", counts["严重"])))
	}
	if counts["警告"] > 0 {
		parts = append(parts, yellowStyle.Render(fmt.Sprintf("警告%d", counts["警告"])))
	}
	if counts["提示"] > 0 {
		parts = append(parts, detailValueStyle.Render(fmt.Sprintf("提示%d", counts["提示"])))
	}
	return strings.Join(parts, "  ")
}

func cardRiskText(checks []checkItem, width int) string {
	counts := map[string]int{}
	for _, check := range checks {
		counts[check.Level]++
	}
	if counts["严重"] == 0 && counts["警告"] == 0 {
		return ""
	}
	text := cardMutedStyle.Render("风险 ")
	if counts["严重"] > 0 {
		text += redStyle.Render(fmt.Sprintf("%d", counts["严重"]))
	}
	if counts["严重"] > 0 && counts["警告"] > 0 {
		text += cardMutedStyle.Render("/")
	}
	if counts["警告"] > 0 {
		text += yellowStyle.Render(fmt.Sprintf("%d", counts["警告"]))
	}
	return ansi.Truncate(text, width, "…")
}

type checkItem struct {
	Level string
	Text  string
}

func buildChecks(state hostState) []checkItem {
	metrics := state.Metrics
	var checks []checkItem
	add := func(level string, text string) {
		checks = append(checks, checkItem{Level: level, Text: text})
	}
	if strings.TrimSpace(state.Host.ExpireAt) != "" {
		if days, ok := expireDays(state.Host.ExpireAt); ok {
			switch {
			case days < 0:
				add("严重", fmt.Sprintf("服务器到期：风险，已过期%d天，建议确认续费或下线", -days))
			case days == 0:
				add("严重", "服务器到期：风险，今天到期，建议立即续费")
			case days <= 7:
				add("警告", fmt.Sprintf("服务器到期：警告，剩余%d天，建议提前续费", days))
			case days <= 30:
				add("提示", fmt.Sprintf("服务器到期：提示，剩余%d天", days))
			}
		} else {
			add("警告", "服务器到期：警告，到期时间格式错误，应为 YYYY-MM-DD")
		}
	}
	if !metrics.Online {
		add("严重", "服务器状态：风险，当前离线，监控数据不可用")
		return checks
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["passwordauthentication"])); value == "yes" {
		add("严重", "允许密码登录：风险，建议关闭 PasswordAuthentication")
	} else if value == "no" {
		add("正常", "SSH密码登录已关闭")
	} else if state.SSHDSecurityError != "" {
		add("提示", "SSH配置检查：提示，"+state.SSHDSecurityError)
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["permitrootlogin"])); value == "yes" {
		add("严重", "允许root登录：风险，建议设置 PermitRootLogin no")
	} else if value == "without-password" || value == "prohibit-password" {
		add("警告", "允许root登录：警告，未完全禁用，建议设置 PermitRootLogin no")
	} else if value == "no" {
		add("正常", "Root登录已关闭")
	}
	if value := strings.ToLower(strings.TrimSpace(state.SSHDSecurity["pubkeyauthentication"])); value == "no" {
		add("警告", "密钥登录：警告，SSH密钥登录已关闭，建议确认是否符合预期")
	}
	sshPort := strings.TrimSpace(state.Host.Port)
	if sshPort == "" {
		sshPort = "22"
	}
	add("提示", fmt.Sprintf("SSH端口：提示，当前端口%s，建议安全组只允许你的IP连接", sshPort))
	failedCount := loginSummaryCount(state.FailedLoginSummary)
	failedSourceCount := loginSummaryUniqueSourceCount(state.FailedLoginSummary)
	failedScan := loginSummaryValue(state.FailedLoginSummary, "疑似扫描")
	if failedCount >= 100 {
		add("严重", fmt.Sprintf("失败登录来源IP过多：风险，最近%d条失败登录，建议限制安全组或启用fail2ban", failedCount))
	} else if failedCount >= 20 {
		add("警告", fmt.Sprintf("失败登录来源IP过多：警告，最近%d条失败登录，建议关注来源IP", failedCount))
	} else if failedSourceCount >= 3 {
		add("警告", fmt.Sprintf("失败登录来源IP过多：警告，发现%d个来源IP，建议确认是否为扫描", failedSourceCount))
	}
	if failedScan != "" && failedScan != "-" {
		add("警告", "失败登录来源IP过多：警告，"+failedScan)
	}
	if metrics.DiskPercent() >= 90 {
		add("严重", fmt.Sprintf("磁盘容量：风险，使用率%.0f%%，建议尽快清理", metrics.DiskPercent()))
	} else if metrics.DiskPercent() >= 80 {
		add("警告", fmt.Sprintf("磁盘容量：警告，使用率%.0f%%，建议关注容量", metrics.DiskPercent()))
	}
	if metrics.MemPercent() >= 90 {
		add("警告", fmt.Sprintf("内存使用：警告，使用率%.0f%%，建议排查进程", metrics.MemPercent()))
	}
	if metrics.CPUPercent >= 90 {
		add("警告", fmt.Sprintf("CPU使用：警告，使用率%.0f%%，建议排查负载", metrics.CPUPercent))
	}
	_, detailStopped, detailFailed := containerDetailCounts(state.ContainerDetails)
	dockerFailed := metrics.DockerFailed
	if dockerFailed == 0 {
		dockerFailed = detailFailed
	}
	if dockerFailed > 0 {
		add("警告", fmt.Sprintf("容器状态：警告，存在%d个故障容器，建议查看容器详情", dockerFailed))
	}
	if metrics.DockerTotal == 0 && len(state.ContainerDetails) > 0 && detailStopped > 0 {
		add("提示", fmt.Sprintf("容器状态：提示，存在%d个停止容器", detailStopped))
	}
	if strings.TrimSpace(state.ContainerError) != "" {
		add("提示", "容器详情：提示，"+state.ContainerError)
	}
	if strings.TrimSpace(state.PortDetailsError) != "" {
		add("提示", "端口详情：提示，"+state.PortDetailsError)
	}
	if metrics.FailedServices > 0 {
		add("警告", fmt.Sprintf("系统服务：警告，存在%d个异常服务", metrics.FailedServices))
	}
	if metrics.HealthTotal() > 0 && metrics.HealthOK() < metrics.HealthTotal() {
		add("警告", fmt.Sprintf("健康端口：警告，%d/%d正常", metrics.HealthOK(), metrics.HealthTotal()))
	}
	if len(state.PortDetails) > 0 {
		publicDockerPorts := publicDockerProxyPorts(state.PortDetails)
		if publicDockerPorts > 0 {
			add("提示", fmt.Sprintf("公网端口：提示，发现%d个Docker映射端口，建议只开放必要端口", publicDockerPorts))
		}
	}
	return checks
}

func publicDockerProxyPorts(ports []portDetail) int {
	count := 0
	for _, port := range ports {
		if strings.TrimSpace(port.Container) != "" || strings.TrimSpace(port.Process) == "docker-proxy" {
			count++
		}
	}
	return count
}

func styleCheck(level string, text string) string {
	switch level {
	case "严重":
		return redStyle.Render(text)
	case "警告":
		return yellowStyle.Render(text)
	case "正常":
		return greenStyle.Render(text)
	default:
		return detailValueStyle.Render(text)
	}
}

func loginSummaryCount(summary []string) int {
	for _, row := range summary {
		label, value, ok := strings.Cut(row, "\t")
		if !ok || label != "统计" {
			continue
		}
		re := regexp.MustCompile(`\d+`)
		match := re.FindString(value)
		if match == "" {
			return 0
		}
		n, _ := strconv.Atoi(match)
		return n
	}
	return 0
}

func loginSummaryUniqueSourceCount(summary []string) int {
	for _, row := range summary {
		label, value, ok := strings.Cut(row, "\t")
		if !ok || label != "来源IP" {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" || value == "-" {
			return 0
		}
		count := 0
		for _, part := range strings.Split(value, "、") {
			if strings.TrimSpace(part) != "" {
				count++
			}
		}
		return count
	}
	return 0
}

func loginSummaryValue(summary []string, label string) string {
	for _, row := range summary {
		got, value, ok := strings.Cut(row, "\t")
		if ok && got == label {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func serviceCardText(metrics monitor.Metrics) string {
	total := dockerTotal(metrics)
	containerText := cardMutedStyle.Render(fmt.Sprintf("容器 %d/%d/%d", metrics.DockerFailed, metrics.DockerRunning, total))
	if total == 0 {
		containerText = cardMutedStyle.Render("容器 0")
	}
	if metrics.DockerFailed > 0 {
		containerText = cardMutedStyle.Render("容器 ") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + cardMutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := cardMutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	serviceText := cardMutedStyle.Render("服务 ") + serviceNumber
	if metrics.HealthTotal() > 0 {
		healthNumber := cardMutedStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		switch {
		case metrics.HealthOK() == metrics.HealthTotal():
			healthNumber = greenStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		case metrics.HealthOK() == 0:
			healthNumber = redStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		default:
			healthNumber = yellowStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		}
		healthText := cardMutedStyle.Render("健康 ") + healthNumber
		return fmt.Sprintf("%s  %s  %s", healthText, containerText, serviceText)
	}
	return fmt.Sprintf("%s  %s", containerText, serviceText)
}

func healthPortsText(metrics monitor.Metrics) string {
	if metrics.HealthTotal() == 0 {
		return "-"
	}
	parts := make([]string, 0, len(metrics.HealthPorts))
	for _, port := range metrics.HealthPorts {
		status := "失败"
		if port.Healthy {
			status = "正常"
		}
		parts = append(parts, fmt.Sprintf("%d%s", port.Port, status))
	}
	return strings.Join(parts, "  ")
}

func padToBottom(lines []string, height int, reservedBottomLines int) []string {
	if height <= 0 {
		return lines
	}
	used := 0
	for _, line := range lines {
		used += strings.Count(line, "\n") + 1
	}
	target := height - reservedBottomLines
	for used < target {
		lines = append(lines, "")
		used++
	}
	return lines
}

func uptimeCN(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(value)
	value = normalizeWeeksToDays(value)
	replacer := strings.NewReplacer(
		" days", "天",
		" day", "天",
		" hours", "小时",
		" hour", "小时",
		" minutes", "分钟",
		" minute", "分钟",
		", ", "",
		" ago", "前",
	)
	value = replacer.Replace(value)
	return value
}

func cardUptimeShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	if days > 0 {
		return fmt.Sprintf("%d天", days)
	}
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	if hours > 0 {
		return fmt.Sprintf("%d时", hours)
	}
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%d分", minutes)
}

func lastLoginDetail(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	relative := relativeTime(value)
	if relative != "刚刚" {
		relative += "前"
	}
	return value.Format("2006-01-02 15:04") + "（" + relative + "）"
}

func lastLoginCard(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	relative := relativeTime(value)
	if relative == "刚刚" {
		return relative
	}
	return relative + "前"
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	d := time.Since(value)
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "刚刚"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d分", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d时", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%d天", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d月", months)
	}
	return fmt.Sprintf("%d年", days/365)
}

func parseLoginRecords(output string, limit int) []string {
	var records []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "wtmp begins") ||
			strings.HasPrefix(lower, "btmp begins") ||
			strings.HasPrefix(lower, "reboot ") ||
			strings.HasPrefix(lower, "shutdown ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		records = append(records, strings.Join(fields, " "))
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records
}

func failedLoginScript() string {
	return `if ! command -v lastb >/dev/null 2>&1; then
  echo "__SSHM_LASTB_UNAVAILABLE__"
  exit 0
fi
out=$(lastb -n 100 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n lastb -n 100 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_LASTB_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func portDetailScript() string {
	return `if ! command -v ss >/dev/null 2>&1; then
  echo "__SSHM_SS_UNAVAILABLE__"
  exit 0
fi
out=$(ss -H -tulnp 2>&1)
code=$?
if [ "$code" -eq 0 ] && ! printf '%s\n' "$out" | grep -q 'users:('; then
  sudo_out=$(sudo -n ss -H -tulnp 2>&1)
  sudo_code=$?
  if [ "$sudo_code" -eq 0 ]; then
    out="$sudo_out"
  fi
fi
if [ "$code" -ne 0 ]; then
  sudo_out=$(sudo -n ss -H -tulnp 2>&1)
  sudo_code=$?
  if [ "$sudo_code" -ne 0 ]; then
    echo "__SSHM_SS_PERMISSION__"
    printf '%s\n' "$sudo_out"
    exit 0
  fi
  out="$sudo_out"
fi
printf '%s\n' "$out"`
}

func containerDetailScript() string {
	return `if ! command -v docker >/dev/null 2>&1; then
  echo "__SSHM_DOCKER_UNAVAILABLE__"
  exit 0
fi
out=$(docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_DOCKER_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"`
}

func sshdSecurityScript() string {
	return `if command -v sshd >/dev/null 2>&1; then
  sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
elif [ -x /usr/sbin/sshd ]; then
  /usr/sbin/sshd -T 2>/dev/null | awk '/^(passwordauthentication|permitrootlogin|pubkeyauthentication) / {print $1"="$2}'
fi`
}

func parseSSHDSettings(output string) map[string]string {
	settings := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		settings[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
	}
	return settings
}

func parsePortDetails(output string) ([]portDetail, string) {
	if strings.Contains(output, "__SSHM_SS_UNAVAILABLE__") {
		return nil, "ss不可用"
	}
	if strings.Contains(output, "__SSHM_SS_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n ss）"
	}
	lines := strings.Split(output, "\n")
	grouped := map[string]*portDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Netid") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[4]
		port := portFromAddress(local)
		if port == "" || port == "*" {
			continue
		}
		processText := ""
		if len(fields) > 6 {
			processText = strings.Join(fields[6:], " ")
		}
		process, pid := processFromSS(processText)
		key := fields[0] + "/" + port + "/" + process
		if item, ok := grouped[key]; ok {
			item.Count++
			if item.PID == "" && pid != "" {
				item.PID = pid
			}
			continue
		}
		grouped[key] = &portDetail{Protocol: fields[0], Port: port, Process: process, PID: pid, Count: 1}
	}
	out := make([]portDetail, 0, len(grouped))
	for _, item := range grouped {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		pi, _ := strconv.Atoi(out[i].Port)
		pj, _ := strconv.Atoi(out[j].Port)
		if pi == pj {
			return out[i].Protocol < out[j].Protocol
		}
		return pi < pj
	})
	return out, ""
}

func portFromAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if idx := strings.LastIndex(value, "]:"); idx >= 0 {
			return strings.TrimSpace(value[idx+2:])
		}
	}
	idx := strings.LastIndex(value, ":")
	if idx < 0 || idx == len(value)-1 {
		return ""
	}
	return strings.TrimSpace(value[idx+1:])
}

func processFromSS(value string) (string, string) {
	name := ""
	pid := ""
	if idx := strings.Index(value, "\""); idx >= 0 {
		rest := value[idx+1:]
		if end := strings.Index(rest, "\""); end >= 0 {
			name = rest[:end]
		}
	}
	if idx := strings.Index(value, "pid="); idx >= 0 {
		rest := value[idx+4:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		pid = rest[:end]
	}
	return name, pid
}

func parseContainerDetails(output string) ([]containerDetail, string) {
	if strings.Contains(output, "__SSHM_DOCKER_UNAVAILABLE__") {
		return nil, "未安装Docker"
	}
	if strings.Contains(output, "__SSHM_DOCKER_PERMISSION__") {
		return nil, "需要Docker权限（可配置sudo -n docker）"
	}
	lines := strings.Split(output, "\n")
	out := make([]containerDetail, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		item := containerDetail{
			Name:   strings.TrimSpace(parts[0]),
			Image:  strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
		}
		if len(parts) >= 4 {
			item.Ports = strings.TrimSpace(parts[3])
		}
		if item.Name != "" {
			out = append(out, item)
		}
	}
	return out, ""
}

func associatePortContainers(ports []portDetail, containers []containerDetail) {
	portMap := containerPublishedPortMap(containers)
	for i := range ports {
		key := strings.ToLower(ports[i].Protocol) + "/" + ports[i].Port
		if names := portMap[key]; len(names) > 0 {
			ports[i].Container = strings.Join(names, "、")
		}
	}
}

func containerPublishedPortMap(containers []containerDetail) map[string][]string {
	out := map[string][]string{}
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		for _, part := range strings.Split(container.Ports, ",") {
			hostPort, proto, ok := parseDockerPublishedPort(part)
			if !ok {
				continue
			}
			key := proto + "/" + hostPort
			if !stringSliceContains(out[key], name) {
				out[key] = append(out[key], name)
			}
		}
	}
	return out
}

func parseDockerPublishedPort(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return "", "", false
	}
	hostPort := portFromAddress(left)
	if hostPort == "" {
		return "", "", false
	}
	proto := "tcp"
	if idx := strings.LastIndex(right, "/"); idx >= 0 && idx < len(right)-1 {
		proto = strings.ToLower(strings.TrimSpace(right[idx+1:]))
	}
	if proto != "tcp" && proto != "udp" {
		proto = "tcp"
	}
	return hostPort, proto, true
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func failedLoginSummary(output string) ([]string, string) {
	if strings.Contains(output, "__SSHM_LASTB_UNAVAILABLE__") {
		return nil, "lastb不可用"
	}
	if strings.Contains(output, "__SSHM_LASTB_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n lastb）"
	}
	return loginSummaryRows(parseLoginRecords(output, 100)), ""
}

func loginSummaryRows(records []string) []string {
	if len(records) == 0 {
		return nil
	}
	ipCounts := map[string]int{}
	userCounts := map[string]int{}
	ipUsers := map[string]map[string]bool{}
	for _, record := range records {
		fields := strings.Fields(record)
		if len(fields) > 0 {
			userCounts[fields[0]]++
		}
		if len(fields) > 2 {
			ipCounts[fields[2]]++
			if ipUsers[fields[2]] == nil {
				ipUsers[fields[2]] = map[string]bool{}
			}
			if len(fields) > 0 {
				ipUsers[fields[2]][fields[0]] = true
			}
		}
	}
	rows := []string{
		fmt.Sprintf("统计\t最近%d条", len(records)),
		fmt.Sprintf("来源IP\t%s", topCountsText(ipCounts, 3)),
		fmt.Sprintf("用户名\t%s", topCountsText(userCounts, 5)),
		fmt.Sprintf("最近\t%s", records[0]),
	}
	if scanText := suspiciousScanText(ipUsers); scanText != "" {
		rows = append(rows, fmt.Sprintf("疑似扫描\t%s", scanText))
	}
	return rows
}

func suspiciousScanText(ipUsers map[string]map[string]bool) string {
	type item struct {
		IP    string
		Users int
	}
	items := []item{}
	for ip, users := range ipUsers {
		if len(users) >= 3 {
			items = append(items, item{IP: ip, Users: len(users)})
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Users == items[j].Users {
			return items[i].IP < items[j].IP
		}
		return items[i].Users > items[j].Users
	})
	limit := minInt(3, len(items))
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s 尝试%d个用户名", items[i].IP, items[i].Users))
	}
	return strings.Join(parts, "、")
}

func topCountsText(counts map[string]int, limit int) string {
	if len(counts) == 0 {
		return "-"
	}
	type item struct {
		Value string
		Count int
	}
	items := make([]item, 0, len(counts))
	for value, count := range counts {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items = append(items, item{Value: value, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Value < items[j].Value
		}
		return items[i].Count > items[j].Count
	})
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s %d次", items[i].Value, items[i].Count))
	}
	return strings.Join(parts, "、")
}

func firstUptimeNumber(value string, pattern string) int {
	re := regexp.MustCompile(pattern)
	parts := re.FindStringSubmatch(value)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}

func normalizeWeeksToDays(value string) string {
	re := regexp.MustCompile(`(\d+)\s+weeks?(?:,\s*(\d+)\s+days?)?`)
	return re.ReplaceAllStringFunc(value, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 0 {
			return match
		}
		weeks, _ := strconv.Atoi(parts[1])
		days := 0
		if len(parts) > 2 && parts[2] != "" {
			days, _ = strconv.Atoi(parts[2])
		}
		return fmt.Sprintf("%d days", weeks*7+days)
	})
}

func bytesHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(value)
	unit := 0
	for f >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f%s", f, units[unit])
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func bytesPair(used uint64, total uint64) string {
	if used == 0 && total == 0 {
		return ""
	}
	return fmt.Sprintf("%s/%s", bytesHuman(used), bytesHuman(total))
}

func diskMountLabel(metrics monitor.Metrics) string {
	mountpoint := strings.TrimSpace(metrics.DiskMountpoint)
	if mountpoint == "" {
		return "-"
	}
	return mountpoint
}

func diskMetricLabel(metrics monitor.Metrics) string {
	mountpoint := diskMountLabel(metrics)
	if mountpoint == "-" || mountpoint == "/" {
		return "磁盘"
	}
	return "磁盘" + mountpoint
}

func diskMountPercentText(metrics monitor.Metrics) string {
	label := diskMountLabel(metrics)
	percent := metricValueStyle(metrics.DiskPercent(), 80, 90).Render(fmt.Sprintf("%.0f%%", metrics.DiskPercent()))
	if label == "-" {
		return percent
	}
	return fit(label+" "+percent, 18)
}

func diskSummaryText(metrics monitor.Metrics) string {
	label := diskMountLabel(metrics)
	size := bytesPair(metrics.DiskUsed, metrics.DiskTotal)
	if label == "-" {
		return size
	}
	if size == "" {
		return label
	}
	return label + " " + size
}

func (m Model) diskListText(metrics monitor.Metrics) string {
	if len(metrics.Disks) == 0 {
		return ""
	}
	disks := append([]monitor.DiskMetric(nil), metrics.Disks...)
	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Percent() > disks[j].Percent()
	})
	mountWidth := 8
	for _, disk := range disks {
		if width := ansi.StringWidth(emptyDash(disk.Mountpoint)); width > mountWidth {
			mountWidth = width
		}
	}
	rows := []string{"", "分区"}
	for i, disk := range disks {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, m.diskPartitionInfoLine(i+1, disk, mountWidth))
		rows = append(rows, m.diskPartitionUsageLine(disk))
	}
	rows = append(rows, "")
	return strings.Join(rows, "\n")
}

func (m Model) diskPartitionInfoLine(index int, disk monitor.DiskMetric, mountWidth int) string {
	width := m.detailContentWidth()
	indexText := detailLabelStyle.Render(fmt.Sprintf("%02d", index))
	mount := emptyDash(disk.Mountpoint)
	filesystem := strings.TrimSpace(disk.Filesystem)
	diskType := strings.TrimSpace(disk.Type)
	if filesystem == "" {
		filesystem = "-"
	}
	if diskType == "" {
		diskType = "-"
	}
	suffixRaw := "  类型 " + diskType
	if mountWidth > 24 {
		mountWidth = 24
	}
	prefixRaw := fmt.Sprintf("%02d  %s  设备 ", index, padVisible(fit(mount, mountWidth), mountWidth))
	filesystemWidth := width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	if filesystemWidth < 12 {
		mountWidth = width - ansi.StringWidth(fmt.Sprintf("%02d  ", index)) - ansi.StringWidth("  设备 ") - ansi.StringWidth(suffixRaw) - 12
		if mountWidth < 8 {
			mountWidth = 8
		}
		mount = fit(mount, mountWidth)
		prefixRaw = fmt.Sprintf("%02d  %s  设备 ", index, padVisible(mount, mountWidth))
		filesystemWidth = width - ansi.StringWidth(prefixRaw) - ansi.StringWidth(suffixRaw)
	}
	if filesystemWidth < 8 {
		filesystemWidth = 8
	}
	mount = padVisible(fit(mount, mountWidth), mountWidth)
	line := indexText +
		"  " + detailValueStyle.Render(mount) +
		"  " + mutedStyle.Render("设备") + " " + detailValueStyle.Render(fit(filesystem, filesystemWidth)) +
		"  " + mutedStyle.Render("类型") + " " + detailValueStyle.Render(diskType)
	if ansi.StringWidth(line) > width {
		return fitANSI(line, width)
	}
	return line
}

func (m Model) diskPartitionUsageLine(disk monitor.DiskMetric) string {
	parts := []string{percentBarWithThreshold(disk.Percent(), 80, 90)}
	if size := bytesPair(disk.Used, disk.Total); size != "" {
		parts = append(parts, detailValueStyle.Render(size))
	}
	if disk.AvailKnown {
		parts = append(parts, mutedStyle.Render("可用")+" "+detailValueStyle.Render(bytesHuman(disk.Available)))
	}
	line := strings.Repeat(" ", 10) + strings.Join(parts, "  ")
	if ansi.StringWidth(line) > m.detailContentWidth() {
		return fitANSI(line, m.detailContentWidth())
	}
	return line
}

func swapUsageText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "未配置"
	}
	return fmt.Sprintf("%s  %s / %s", percentBar(metrics.SwapPercent()), bytesHuman(metrics.SwapUsed), bytesHuman(metrics.SwapTotal))
}

func swapFreeText(metrics monitor.Metrics) string {
	if metrics.SwapTotal == 0 {
		return "-"
	}
	return bytesHuman(metrics.SwapFree)
}

func inodeUsageText(metrics monitor.Metrics) string {
	if metrics.InodeTotal == 0 && metrics.InodeUsed == 0 && metrics.InodeAvailable == 0 {
		return "-"
	}
	return fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.InodePercent(), 80, 90), countHuman(metrics.InodeUsed), countHuman(metrics.InodeTotal))
}

func countHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"", "K", "M", "B"}
	f := float64(value)
	unit := 0
	for f >= 1000 && unit < len(units)-1 {
		f /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d核", metrics.CPUCores)
}

func fit(s string, width int) string {
	if runewidth.StringWidth(s) <= width {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && runewidth.StringWidth(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func fitANSI(s string, width int) string {
	return ansi.Truncate(s, width, "…")
}

var (
	green     = lipgloss.Color("42")
	yellow    = lipgloss.Color("214")
	red       = lipgloss.Color("196")
	blue      = lipgloss.Color("39")
	textGray  = lipgloss.Color("245")
	valueGray = lipgloss.Color("252")
	cyan      = lipgloss.Color("45")
	softGray  = lipgloss.Color("235")
	lineGray  = lipgloss.Color("234")

	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(blue)
	mutedStyle          = lipgloss.NewStyle().Foreground(textGray)
	cardMutedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	helpStyle           = lipgloss.NewStyle().Foreground(textGray)
	navStyle            = lipgloss.NewStyle().Foreground(textGray)
	barEmptyStyle       = lipgloss.NewStyle().Foreground(softGray)
	subtleLineStyle     = lipgloss.NewStyle().Foreground(lineGray)
	detailSectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(blue)
	detailSubTitleStyle = lipgloss.NewStyle().Foreground(cyan)
	detailSuccessStyle  = lipgloss.NewStyle().Foreground(green)
	detailDangerStyle   = lipgloss.NewStyle().Foreground(red)
	detailLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	detailValueStyle    = lipgloss.NewStyle().Foreground(valueGray)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(softGray).
			Padding(0, 1).
			MarginBottom(0)
	selectedCardStyle       = cardStyle.BorderForeground(blue)
	cardBorderStyle         = lipgloss.NewStyle().Foreground(softGray)
	selectedCardBorderStyle = lipgloss.NewStyle().Foreground(blue)
	detailStyle             = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(blue).
				Padding(1, 2)

	greenStyle    = lipgloss.NewStyle().Foreground(green)
	yellowStyle   = lipgloss.NewStyle().Foreground(yellow)
	favoriteStyle = lipgloss.NewStyle().Bold(true).Foreground(yellow)
	pinnedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("201"))
	redStyle      = lipgloss.NewStyle().Foreground(red)
	blueStyle     = lipgloss.NewStyle().Foreground(blue)
)

func Run(hosts []host.Host, passwords config.PasswordStore) error {
	model := New(hosts, passwords)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func Fatal(message string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	} else {
		fmt.Fprintln(os.Stderr, message)
	}
	os.Exit(1)
}
