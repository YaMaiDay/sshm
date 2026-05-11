package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
	modeCommandList
	modeCommandEdit
	modeCommandConfirm
	modeCommandOutput
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

const (
	dashboardCardInnerHeight = 7
	dashboardCardTotalHeight = dashboardCardInnerHeight + 2
)

type hostState struct {
	Host         host.Host
	Metrics      monitor.Metrics
	Loading      bool
	FailureCount int
	LastAttempt  time.Time
}

type collectMsg struct {
	Index   int
	Round   int
	Metrics monitor.Metrics
	Manual  bool
}

type tickMsg time.Time

type transferDoneMsg struct {
	Kind   string
	Source string
	Target string
	Err    error
	Output string
}

type transferProgressMsg time.Time

type clearStatusMsg struct{}

type sshDoneMsg struct {
	Err error
}

type commandDoneMsg struct {
	Result actions.CommandResult
}

type activeTransfer struct {
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
	states              []hostState
	selected            int
	width               int
	height              int
	searching           bool
	query               string
	status              string
	refreshStatus       string
	collector           monitor.Collector
	passwords           config.PasswordStore
	appConfig           config.AppConfig
	home                string
	mode                viewMode
	transfer            transferMode
	pickIndex           int
	pickTitle           string
	choices             []choice
	remoteTree          remoteTree
	pending             pendingTransfer
	panel               transferPanel
	form                addForm
	formIndex           int
	formCursor          int
	formPane            int
	categories          []string
	categoryIndex       int
	addingCategory      bool
	categoryDraft       string
	editing             bool
	editIndex           int
	deleteIndex         int
	confirm             confirmAction
	filter              filterMode
	sortBy              sortMode
	category            string
	favoriteOnly        bool
	detailScroll        int
	activeTransfer      activeTransfer
	commandFile         config.CommandsFile
	commandItems        []commandItem
	commandIndex        int
	commandForm         commandEditForm
	commandField        int
	commandCursor       int
	commandEditing      bool
	commandEditItem     commandItem
	commandConfirm      commandItem
	commandOutputScroll int
	activeCommand       activeCommand
	helpBackMode        viewMode
	collectRound        int
	manualRound         int
	pendingByRound      map[int]int
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
	LeftIndex    int
	RightIndex   int
	Confirming   bool
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
	ProxyJump    string
	Password     string
	HealthPorts  string
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

type activeCommand struct {
	HostIndex int
	Name      string
	Command   string
	Output    string
	ExitCode  int
	Running   bool
}

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmDeleteCategory
	confirmDeleteCommand
)

type confirmAction struct {
	Kind    confirmKind
	Title   string
	Lines   []string
	Back    viewMode
	Command commandItem
	Value   string
}

func (f addForm) fields() []struct {
	label string
	value string
} {
	return []struct {
		label string
		value string
	}{
		{"分类", f.Category},
		{"服务器名称", f.Name},
		{"服务器地址", f.HostName},
		{"用户名", f.User},
		{"端口", f.Port},
		{"密钥文件", f.IdentityFile},
		{"密码", f.Password},
		{"跳板机", f.ProxyJump},
		{"健康端口", f.HealthPorts},
	}
}

func New(hosts []host.Host, passwords config.PasswordStore) Model {
	home, _ := os.UserHomeDir()
	appConfig := config.LoadAppConfig(home)
	categories, _, _ := config.LoadCategories(home)
	commandFile, _, _ := config.LoadCommands(home)
	states := make([]hostState, len(hosts))
	for i, h := range hosts {
		states[i] = hostState{Host: h, Loading: true}
	}
	pendingByRound := map[int]int{1: len(states)}
	collector := monitor.NewCollector(passwords)
	collector.Timeout = appConfig.CommandDuration()
	collector.ConnectTimeout = appConfig.ConnectDuration()
	return Model{
		states:         states,
		collector:      collector,
		passwords:      passwords,
		appConfig:      appConfig,
		home:           home,
		commandFile:    commandFile,
		categories:     categories,
		status:         "正在采集服务器状态...",
		collectRound:   1,
		pendingByRound: pendingByRound,
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
		if msg.Err != nil {
			m.status = msg.Kind + "失败：" + transferErrorText(msg.Err, msg.Output)
			return m, clearStatusAfter(3 * time.Second)
		} else {
			m.status = fmt.Sprintf("%s完成：%s -> %s", msg.Kind, filepath.Base(msg.Source), msg.Target)
			return m, clearStatusAfter(3 * time.Second)
		}
	case transferProgressMsg:
		if !m.activeTransfer.Active {
			return m, nil
		}
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
		m.status = "已返回监控面板"
		return m, tea.Batch(clearScreen(), clearStatusAfter(2*time.Second))
	case commandDoneMsg:
		m.activeCommand.Running = false
		m.activeCommand.Output = msg.Result.Output
		m.activeCommand.ExitCode = msg.Result.ExitCode
		if msg.Result.Err != nil {
			m.status = fmt.Sprintf("命令执行失败：退出码 %d", msg.Result.ExitCode)
		} else {
			m.status = "命令执行完成。"
		}
		return m, nil
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
		switch msg.String() {
		case "q", "Q", "esc", "ctrl+c":
			if m.activeTransfer.Active && m.activeTransfer.Cancel != nil {
				m.activeTransfer.Cancel()
			}
			return m, tea.Quit
		case "j", "J", "down":
			m.move(m.dashboardColumns())
		case "k", "K", "up":
			m.move(-m.dashboardColumns())
		case "h", "H", "left":
			m.move(-1)
		case "l", "L", "right":
			m.move(1)
		case "/":
			m.searching = true
			m.query = ""
		case "?":
			m.helpBackMode = modeDashboard
			m.mode = modeHelp
		case "s", "S":
			m.sortBy = (m.sortBy + 1) % 5
			m.selected = 0
			m.status = "排序：" + m.sortName()
		case "o", "O":
			if m.filter == filterOnline {
				m.filter = filterAll
			} else {
				m.filter = filterOnline
			}
			m.selected = 0
		case "p", "P":
			if m.filter == filterProblem {
				m.filter = filterAll
			} else {
				m.filter = filterProblem
			}
			m.selected = 0
		case "f", "F":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.toggleFavorite(idx)
			}
		case "v", "V":
			m.favoriteOnly = !m.favoriteOnly
			m.selected = 0
			if m.favoriteOnly {
				m.status = "筛选：收藏"
			} else {
				m.status = "已取消收藏筛选"
			}
		case "t", "T":
			m.cycleCategory()
			m.selected = 0
		case " ":
			if _, ok := m.selectedRealIndex(); ok {
				m.mode = modeDetail
				m.detailScroll = 0
			}
		case "a", "A":
			return m.startAddForm(), nil
		case "e", "E":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startEditForm(idx), nil
			}
		case "x", "X", "delete":
			if idx, ok := m.selectedRealIndex(); ok {
				m.deleteIndex = idx
				m.mode = modeDeleteConfirm
			}
		case "u", "U":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startUpload(idx), nil
			}
		case "d", "D":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startDownload(idx), nil
			}
		case "m", "M":
			if idx, ok := m.selectedRealIndex(); ok {
				return m.startCommandList(idx), nil
			}
		case "r", "R":
			m.status = "正在刷新全部服务器..."
			m.collectRound++
			m.manualRound = m.collectRound
			m.pendingByRound[m.collectRound] = len(m.states)
			return m, m.collectAll(m.collectRound, true)
		case "enter":
			if idx, ok := m.selectedRealIndex(); ok {
				cmd, cleanup := actions.SSHCommand(m.states[idx].Host)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					cleanup()
					return sshDoneMsg{Err: err}
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
	fieldCount := len(m.form.fields())
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "tab":
		m.formPane = 1
	case "down":
		m.formIndex = (m.formIndex + 1) % fieldCount
		m.formCursor = m.formValueLen()
	case "shift+tab":
		m.formPane = 1
	case "up":
		m.formIndex--
		if m.formIndex < 0 {
			m.formIndex = fieldCount - 1
		}
		m.formCursor = m.formValueLen()
	case "left":
		if m.formIndex == 0 {
			m.moveCategory(-1)
		} else {
			m.moveFormCursor(-1)
		}
	case "right":
		if m.formIndex == 0 {
			m.moveCategory(1)
		} else {
			m.moveFormCursor(1)
		}
	case "enter":
		healthPorts, err := config.ParseHealthPorts(m.form.HealthPorts)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		favorite := false
		if m.editing {
			if m.editIndex < 0 || m.editIndex >= len(m.states) {
				m.status = "编辑失败：没有选中的服务器"
				return m, nil
			}
			favorite = m.states[m.editIndex].Host.Favorite
		}
		input := config.HostInput{
			Category:     m.form.Category,
			Name:         m.form.Name,
			HostName:     m.form.HostName,
			User:         m.form.User,
			Port:         m.form.Port,
			IdentityFile: m.form.IdentityFile,
			ProxyJump:    m.form.ProxyJump,
			Password:     m.form.Password,
			Favorite:     favorite,
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
		} else {
			m.status = "服务器已添加。"
		}
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	case "backspace":
		m.formBackspace()
	default:
		if len(msg.Runes) > 0 && m.formIndex != 0 {
			m.formAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) updateCategoryPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.addingCategory {
		switch msg.String() {
		case "esc", "q", "Q", "ctrl+c":
			m.addingCategory = false
			m.categoryDraft = ""
		case "enter":
			if err := config.AddCategory(m.home, m.categoryDraft); err != nil {
				m.status = "添加分类失败：" + categoryErrorText(err)
			} else {
				m.reloadCategories(m.categoryDraft)
				m.form.Category = m.categories[m.categoryIndex]
				m.status = "分类已添加。"
			}
			m.addingCategory = false
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "tab", "shift+tab":
		m.formPane = 0
	case "j", "J", "down":
		m.moveCategory(1)
	case "k", "K", "up":
		m.moveCategory(-1)
	case "n", "N", "a", "A":
		m.addingCategory = true
		m.categoryDraft = ""
		m.status = "输入新分类名称。"
	case "x", "X", "delete":
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

func (m *Model) reloadCategories(prefer string) {
	categories, _, err := config.LoadCategories(m.home)
	if err != nil || len(categories) == 0 {
		categories = []string{"default"}
	}
	m.categories = categories
	m.categoryIndex = 0
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
		return "分类下面还有服务器，不能删除"
	default:
		return err.Error()
	}
}

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n", "N":
		m.mode = modeDashboard
		m.status = "已取消删除。"
	case "y", "Y", "enter":
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
	switch msg.String() {
	case "esc", "n", "N", "q", "Q":
		m.mode = m.confirm.Back
		m.status = "已取消删除。"
	case "y", "Y", "enter":
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
	m.editIndex = -1
	m.addingCategory = false
	m.categoryDraft = ""
	m.form = addForm{Category: m.categories[m.categoryIndex], User: "root", Port: "22"}
	m.status = "添加服务器"
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
	m.editIndex = idx
	m.addingCategory = false
	m.categoryDraft = ""
	m.form = addForm{
		Category:     m.categories[m.categoryIndex],
		Name:         input.Name,
		HostName:     input.HostName,
		User:         input.User,
		Port:         input.Port,
		IdentityFile: input.IdentityFile,
		ProxyJump:    input.ProxyJump,
		Password:     input.Password,
		HealthPorts:  config.FormatHealthPorts(input.HealthPorts),
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "j", "J", "down":
		m.moveCommandIndex(1)
	case "k", "K", "up":
		m.moveCommandIndex(-1)
	case "a", "A":
		return m.startCommandEdit(commandItem{}, false), nil
	case "e", "E":
		item, ok := m.selectedCommandItem()
		if ok && !item.Temporary {
			return m.startCommandEdit(item, true), nil
		}
	case "x", "X", "delete":
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeCommandList
		m.status = "已取消。"
	case "j", "J", "down":
		m.commandOutputScroll = clampInt(m.commandOutputScroll+1, 0, m.commandConfirmMaxScroll())
	case "k", "K", "up":
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
		m.mode = modeCommandOutput
		m.status = "正在执行命令..."
		return m, m.runCommand(m.activeCommand.HostIndex, m.commandConfirm.Command)
	}
	return m, nil
}

func (m Model) updateCommandOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeDashboard
		m.status = ""
	case "j", "J", "down":
		m.commandOutputScroll = clampInt(m.commandOutputScroll+1, 0, m.commandOutputMaxScroll())
	case "k", "K", "up":
		m.commandOutputScroll = clampInt(m.commandOutputScroll-1, 0, m.commandOutputMaxScroll())
	}
	return m, nil
}

func (m Model) updateHelpPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "Q", "?":
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
	return len([]rune(m.formValue()))
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
		return m.form.ProxyJump
	case 8:
		return m.form.HealthPorts
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
		m.form.ProxyJump = value
	case 8:
		m.form.HealthPorts = value
	}
}

func removeLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searching = false
		m.query = ""
		m.selected = 0
	case "enter":
		m.searching = false
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c", "b", "B", "left":
		m.mode = modeDashboard
		m.detailScroll = 0
	case "j", "J", "down":
		m.detailScroll = clampInt(m.detailScroll+1, 0, m.detailMaxScroll())
	case "k", "K", "up":
		m.detailScroll = clampInt(m.detailScroll-1, 0, m.detailMaxScroll())
	case "u", "U":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startUpload(idx), nil
		}
	case "d", "D":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startDownload(idx), nil
		}
	case "r", "R":
		if idx, ok := m.selectedRealIndex(); ok {
			m.states[idx].Loading = true
			return m, m.collectOne(idx)
		}
	case "f", "F":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.toggleFavorite(idx)
		}
	case "m", "M":
		if idx, ok := m.selectedRealIndex(); ok {
			return m.startCommandList(idx), nil
		}
	case "enter":
		if idx, ok := m.selectedRealIndex(); ok {
			cmd, cleanup := actions.SSHCommand(m.states[idx].Host)
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				cleanup()
				return sshDoneMsg{Err: err}
			})
		}
	}
	return m, nil
}

func (m Model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
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
	if m.panel.Confirming && msg.String() != "enter" {
		m.panel.Confirming = false
		m.status = transferPanelStatus(m.panel.Mode)
		return m, nil
	}
	switch msg.String() {
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
		return m.prepareTransferConfirm()
	}
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
	if len(m.panel.LeftChoices) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = "左右两边都需要选择。"
		return m, nil
	}
	left := m.panel.LeftChoices[m.panel.LeftIndex]
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = "右侧必须选择目录。"
		return m, nil
	}
	m.pending = pendingTransfer{HostIndex: m.panel.HostIndex}
	if m.panel.Mode == transferUpload {
		m.pending.LocalPath = left.Value
		m.pending.LocalIsDir = left.IsDir
		m.pending.RemoteDir = right.Value
		return m.startUploadTransfer()
	}
	m.pending.RemotePath = left.Value
	m.pending.RemoteIsDir = left.IsDir
	m.pending.SaveDir = right.Value
	return m.startDownloadTransfer()
}

func (m Model) prepareTransferConfirm() (tea.Model, tea.Cmd) {
	if len(m.panel.LeftChoices) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = "左右两边都需要选择。"
		return m, nil
	}
	left := m.panel.LeftChoices[m.panel.LeftIndex]
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = "右侧必须选择目录。"
		return m, nil
	}
	h := m.states[m.panel.HostIndex].Host
	m.panel.Confirming = true
	if m.panel.Mode == transferUpload {
		m.status = fmt.Sprintf("上传 Enter：%s -> %s:%s/  取消 Esc", filepath.Base(left.Value), hostDisplayName(h), right.Value)
		return m, nil
	}
	m.status = fmt.Sprintf("下载 Enter：%s:%s -> %s/  取消 Esc", hostDisplayName(h), left.Value, right.Value)
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
	panel := transferPanel{Mode: mode, HostIndex: idx}
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

func transferPanelStatus(mode transferMode) string {
	if mode == transferUpload {
		return "上传：左侧选择本地文件/目录，右侧选择远程目录，空格确认。"
	}
	return "下载：左侧选择远程文件/目录，右侧选择本地目录，空格确认。"
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
			h.Name, h.HostName, h.User, h.Category,
		}, " "))
		if q == "" || strings.Contains(text, q) {
			indexes = append(indexes, i)
		}
	}
	sort.SliceStable(indexes, func(i, j int) bool {
		a := m.states[indexes[i]]
		b := m.states[indexes[j]]
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
	if m.mode == modeHelp {
		return m.renderHelpPanel()
	}
	if m.mode != modeDashboard {
		return m.renderPicker()
	}

	indexes := m.filteredIndexes()
	headerParts := []string{"sshm", fmt.Sprintf("服务器 %d", len(indexes))}
	if m.searching {
		headerParts = append(headerParts, "搜索："+m.query+"█")
	} else if m.query != "" {
		headerParts = append(headerParts, "搜索："+m.query)
	}
	if m.category != "" {
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
	lines = append(lines, "")

	if len(m.states) == 0 {
		lines = append(lines, mutedStyle.Render("没有服务器。按 a 添加服务器。"))
	} else if len(indexes) == 0 {
		lines = append(lines, mutedStyle.Render("没有匹配的服务器"))
	} else {
		lines = append(lines, m.renderDashboardGrid(indexes))
	}

	helpWidth := m.width
	if helpWidth < 1 {
		helpWidth = contentWidth(m.width)
	}
	helpBlock := renderDashboardHelp(helpWidth)
	pageDots := m.dashboardPageDots(len(indexes))
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
		"收藏 f",
		"收藏 v",
		"添加 a",
		"编辑 e",
		"删除 x",
		"上传 u",
		"下载 d",
		"刷新 r",
		"搜索 /",
		"分类 t",
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
	}
	width := formContentWidth(m.width)
	if m.useSingleFormPane(width) {
		width = detailFrameWidth(m.width)
	}
	help := "切换 Tab  选择 ↑↓  分类 ←→  保存 Enter  返回 Esc"
	if m.formPane == 1 {
		help = "切回 Tab  选择 ↑↓  新增 n  删除 x  返回 Esc"
		if m.addingCategory {
			help = "添加 Enter  返回 Esc"
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
	display := value
	if active {
		display = string(runes[:cursor]) + "│" + string(runes[cursor:])
	}
	fitted := fit(display, width)
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
	text := value
	if active {
		text = string(runes[:cursor]) + "│" + string(runes[cursor:])
	}
	lines := strings.Split(text, "\n")
	cursorLine := 0
	if active {
		cursorLine = strings.Count(string(runes[:cursor]), "\n")
	}
	if len(lines) < height {
		for len(lines) < height {
			lines = append(lines, "")
		}
	}
	if len(lines) > height {
		start := cursorLine - height + 1
		if start < 0 {
			start = 0
		}
		if start+height > len(lines) {
			start = len(lines) - height
		}
		lines = lines[start : start+height]
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, width-4), "\n"))
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
		{"f", "收藏 / 取消收藏"},
		{"v", "只看收藏 / 取消筛选"},
		{"a", "添加服务器"},
		{"e", "编辑服务器"},
		{"x", "删除服务器"},
		{"u", "上传文件或目录"},
		{"d", "下载文件或目录"},
		{"r", "刷新监控"},
		{"/", "搜索"},
		{"t", "切换分类"},
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
	fieldHeight := contentHeight - 1
	if fieldHeight < 1 {
		fieldHeight = 1
	}
	start, end := visibleRange(len(fields), m.formIndex, fieldHeight)
	for i := start; i < end; i++ {
		field := fields[i]
		prefix := " "
		style := lipgloss.NewStyle()
		value := field.value
		if i == 0 {
			value = m.form.Category
			if value == "" && len(m.categories) > 0 {
				value = m.categories[m.categoryIndex]
			}
			value += mutedStyle.Render("  ←/→")
		}
		if m.formPane == 0 && i == m.formIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		lines = append(lines, style.Render(formFieldLine(prefix, field.label, value, innerWidth, i != 0, m.formPane == 0 && i == m.formIndex, m.formCursor)))
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
	if m.addingCategory {
		lines = append(lines, blueStyle.Bold(true).Render(fit("新分类 "+m.categoryDraft+"█", innerWidth)))
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

func formInputText(value string, width int, cursor int) string {
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
	if contentWidth < 0 {
		contentWidth = 0
	}
	start := 0
	if cursor > contentWidth {
		start = cursor - contentWidth
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := start + contentWidth
	if end > len(runes) {
		end = len(runes)
	}
	visible := string(runes[start:end])
	cursorPos := cursor - start
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos > len([]rune(visible)) {
		cursorPos = len([]rune(visible))
	}
	visibleRunes := []rune(visible)
	withCursor := string(visibleRunes[:cursorPos]) + "│" + string(visibleRunes[cursorPos:])
	return padVisible(withCursor, width)
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
	header := titleStyle.Render(fit("服务器详情  "+m.states[idx].Host.Name, width))
	help := renderHelp(width, "滚动 ↑↓/jk  登录 Enter  命令 m  上传 u  下载 d  刷新 r  返回 q/Esc")
	viewportHeight := m.detailViewportHeight()
	if viewportHeight < len(lines) {
		maxScroll := len(lines) - viewportHeight
		scroll := clampInt(m.detailScroll, 0, maxScroll)
		lines = lines[scroll : scroll+viewportHeight]
	}
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{
		header,
		body,
		help,
	}, "\n")
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
		m.detailRow("认证方式", authText(h)),
		m.detailRow("主机名", emptyDash(metrics.RemoteHostname)),
		m.detailRow("系统", emptyDash(metrics.OS)),
		m.detailRow("内核", emptyDash(metrics.Kernel)),
		m.detailRow("架构", emptyDash(metrics.Arch)),
		m.detailRow("来源", h.File),
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
		m.detailRow("使用率", fmt.Sprintf("%s  %s / %s", percentBarWithThreshold(metrics.DiskPercent(), 80, 90), bytesHuman(metrics.DiskUsed), bytesHuman(metrics.DiskTotal))),
		m.detailRow("可用", bytesHuman(metrics.DiskAvailable)),
		m.detailRow("inode", inodeUsageText(metrics)),
		m.detailRow("inode可用", countHuman(metrics.InodeAvailable)),
		"",
		detailSubTitle("系统"),
		m.detailRow("负载", fmt.Sprintf("%s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))),
		m.detailRow("运行时间", uptimeCN(metrics.Uptime)),
		"",
		sectionTitle("服务状态"),
		detailSubTitle("健康"),
		m.detailRow("健康端口", healthPortsText(metrics)),
		m.detailRow("监听端口", emptyDash(metrics.Ports)),
		"",
		detailSubTitle("容器"),
	}
	lines = append(lines, dockerDetailRows(m, metrics)...)
	lines = append(lines,
		"",
		detailSubTitle("异常"),
		m.detailRow("异常服务", failedServiceText(metrics, 8)),
	)
	if metrics.Error != "" {
		lines = append(lines, "", sectionTitle("最近错误"), m.detailRow("错误", metrics.Error))
	}
	return lines, true
}

func (m Model) detailViewportHeight() int {
	height := m.height - 4
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
	help := "切换 Tab  移动 ↑↓/jk  展开 Enter  确认 Space  返回 Esc"
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	body := ""
	if m.useSingleTransferPane(width) {
		if m.panel.ActivePane == 0 {
			body = renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, width, height, true)
		} else {
			body = renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, width, height, true)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) / 2
		rightWidth := width - gap - leftWidth
		left := renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, leftWidth, height, m.panel.ActivePane == 0)
		right := renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, rightWidth, height, m.panel.ActivePane == 1)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		body,
		renderHelp(width, help),
	}, "\n")
}

func (m Model) useSingleTransferPane(width int) bool {
	return width < 70
}

func renderTransferPane(title string, choices []choice, index int, width int, height int, active bool) string {
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
			lines = append(lines, lineStyle.Render(fit(prefix+" "+choices[i].Label, innerWidth)))
		}
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) renderDashboardGrid(indexes []int) string {
	totalWidth := m.width
	if totalWidth <= 0 {
		totalWidth = contentWidth(m.width)
	}
	width := totalWidth
	if width < 34 {
		width = 34
	}
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)

	rowsVisible := (m.height - 4) / dashboardCardTotalHeight
	if rowsVisible < 1 {
		rowsVisible = 1
	}
	selectedRow := m.selected / cols
	startRow := selectedRow - rowsVisible + 1
	if startRow < 0 {
		startRow = 0
	}
	start := startRow * cols
	end := start + rowsVisible*cols
	if end > len(indexes) {
		end = len(indexes)
	}

	var out []string
	for i := start; i < end; i += cols {
		var row []string
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= end {
				row = append(row, padBlock(blankCard(cardWidth), cardWidth))
				continue
			}
			visibleIndex := i + col
			realIndex := indexes[visibleIndex]
			row = append(row, padBlock(m.renderCard(realIndex, visibleIndex == m.selected, cardWidth), cardWidth))
		}
		out = append(out, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(out, "\n")
}

func (m Model) dashboardPageDots(totalItems int) string {
	if totalItems <= 0 {
		return ""
	}
	cols := m.dashboardColumns()
	rowsVisible := (m.height - 4) / dashboardCardTotalHeight
	if rowsVisible < 1 {
		rowsVisible = 1
	}
	perPage := cols * rowsVisible
	if perPage <= 0 {
		return ""
	}
	totalPages := (totalItems + perPage - 1) / perPage
	if totalPages <= 1 {
		return ""
	}
	currentPage := m.selected / perPage
	if currentPage >= totalPages {
		currentPage = totalPages - 1
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
	width := m.width
	if width <= 0 {
		width = contentWidth(m.width)
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

func (m Model) renderCard(index int, selected bool, width int) string {
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
		favoriteMark = yellowStyle.Render("★") + " "
	}
	categoryLabel := "[" + category + "]"
	categoryWidth := runewidth.StringWidth(categoryLabel)
	nameWidth := innerWidth - categoryWidth - ansi.StringWidth(favoriteMark) - 2
	if nameWidth < 8 {
		nameWidth = innerWidth
		categoryLabel = ""
		favoriteMark = ""
	}
	name := fit(h.Name, nameWidth)
	barWidth := 12
	if innerWidth < 42 {
		barWidth = 8
	}
	cpu := percentBarWidth(metrics.CPUPercent, barWidth)
	mem := percentBarWidth(metrics.MemPercent(), barWidth)
	disk := percentBarWidthWithThreshold(metrics.DiskPercent(), barWidth, 80, 90)

	cpuLine := cardMetricLine("CPU", cpu, cpuCoresText(metrics), innerWidth)
	memLine := cardMetricLine("内存", mem, bytesPair(metrics.MemUsed, metrics.MemTotal), innerWidth)
	diskLine := cardMetricLine("磁盘", disk, bytesPair(metrics.DiskUsed, metrics.DiskTotal), innerWidth)
	uptimeLabel := mutedStyle.Render(cardUptimeShort(metrics.Uptime))
	loadLine := fit(fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15)), innerWidth)
	serviceLine := ansi.Truncate(serviceCardText(metrics), innerWidth, "…")
	titleText := name
	if categoryLabel != "" {
		titleText = favoriteMark + categoryLabel + " " + name
	}
	title := fit(titleText, innerWidth)

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
	addressLine := fit(fmt.Sprintf("%s %s", h.Address(), userPort), innerWidth)
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
		cardBottomLine(cardWidth, borderStyle),
	}
	return strings.Join(lines, "\n")
}

func blankCard(width int) string {
	innerWidth := width - 4
	if innerWidth < 30 {
		innerWidth = 30
	}
	return lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder()).
		Padding(0, 1).
		Width(innerWidth).
		Height(dashboardCardInnerHeight).
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
	return mutedStyle.Render(label) + strings.Repeat(" ", padding) + value
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
	full := base + "  " + mutedStyle.Render(extra)
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
	disk := compactMetricWithThreshold("磁盘", metrics.DiskPercent(), colWidth, barWidth, 80, 90)
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

func cardContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := padVisible(content, contentWidth)
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardMutedContentLine(width int, content string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	line := mutedStyle.Render(padVisible(content, contentWidth))
	return borderStyle.Render("│") + " " + line + " " + borderStyle.Render("│")
}

func cardInnerSeparatorLine(width int, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	contentWidth := innerWidth - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	line := subtleLineStyle.Render(dashedLine(contentWidth))
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
		return line
	}
	return ""
}

func sectionTitle(value string) string {
	return blueStyle.Bold(true).Render("[" + value + "]")
}

func detailSubTitle(value string) string {
	return blueStyle.Render("· " + value)
}

func (m Model) detailRow(label, value string) string {
	const labelWidth = 10
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	prefix := mutedStyle.Render(label) + strings.Repeat(" ", padding)
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
	lines = append(lines, prefix+parts[0])
	for _, part := range parts[1:] {
		lines = append(lines, continuationPrefix+part)
	}
	return strings.Join(lines, "\n")
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

func dockerDetailRows(m Model, metrics monitor.Metrics) []string {
	total := dockerTotal(metrics)
	if total == 0 {
		return []string{m.detailRow("状态", "未发现")}
	}
	return []string{
		m.detailRow("总数", fmt.Sprintf("%d", total)),
		m.detailRow("运行", fmt.Sprintf("%d", metrics.DockerRunning)),
		m.detailRow("停止", fmt.Sprintf("%d", metrics.DockerStopped)),
		m.detailRow("故障", fmt.Sprintf("%d", metrics.DockerFailed)),
		m.detailRow("运行容器", dockerRunningText(metrics, 8)),
		m.detailRow("停止容器", dockerStoppedText(metrics, 8)),
		m.detailRow("故障容器", dockerFailedText(metrics, 8)),
	}
}

func serviceCardText(metrics monitor.Metrics) string {
	total := dockerTotal(metrics)
	containerText := mutedStyle.Render(fmt.Sprintf("容器 %d/%d/%d", metrics.DockerFailed, metrics.DockerRunning, total))
	if total == 0 {
		containerText = mutedStyle.Render("容器 0")
	}
	if metrics.DockerFailed > 0 {
		containerText = mutedStyle.Render("容器 ") + redStyle.Render(fmt.Sprintf("%d", metrics.DockerFailed)) + mutedStyle.Render(fmt.Sprintf("/%d/%d", metrics.DockerRunning, total))
	}
	serviceNumber := mutedStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	if metrics.FailedServices > 0 {
		serviceNumber = redStyle.Render(fmt.Sprintf("%d", metrics.FailedServices))
	}
	serviceText := mutedStyle.Render("服务 ") + serviceNumber
	if metrics.HealthTotal() > 0 {
		healthNumber := mutedStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		switch {
		case metrics.HealthOK() == metrics.HealthTotal():
			healthNumber = greenStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		case metrics.HealthOK() == 0:
			healthNumber = redStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		default:
			healthNumber = yellowStyle.Render(fmt.Sprintf("%d/%d", metrics.HealthOK(), metrics.HealthTotal()))
		}
		healthText := mutedStyle.Render("健康 ") + healthNumber
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

var (
	green    = lipgloss.Color("42")
	yellow   = lipgloss.Color("214")
	red      = lipgloss.Color("196")
	blue     = lipgloss.Color("39")
	textGray = lipgloss.Color("245")
	softGray = lipgloss.Color("235")
	lineGray = lipgloss.Color("234")

	titleStyle      = lipgloss.NewStyle().Bold(true).Foreground(blue)
	mutedStyle      = lipgloss.NewStyle().Foreground(textGray)
	helpStyle       = lipgloss.NewStyle().Foreground(textGray)
	navStyle        = lipgloss.NewStyle().Foreground(textGray)
	barEmptyStyle   = lipgloss.NewStyle().Foreground(softGray)
	subtleLineStyle = lipgloss.NewStyle().Foreground(lineGray)

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

	greenStyle  = lipgloss.NewStyle().Foreground(green)
	yellowStyle = lipgloss.NewStyle().Foreground(yellow)
	redStyle    = lipgloss.NewStyle().Foreground(red)
	blueStyle   = lipgloss.NewStyle().Foreground(blue)
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
