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
	modePickLocalRoot
	modePickLocalItem
	modePickRemoteDir
	modePickRemoteItem
	modePickSaveDir
	modeTransferPanel
)

type transferMode int

const (
	transferNone transferMode = iota
	transferUpload
	transferDownload
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
	states         []hostState
	selected       int
	width          int
	height         int
	searching      bool
	query          string
	status         string
	refreshStatus  string
	collector      monitor.Collector
	passwords      config.PasswordStore
	appConfig      config.AppConfig
	home           string
	mode           viewMode
	transfer       transferMode
	pickIndex      int
	pickTitle      string
	choices        []choice
	remoteTree     remoteTree
	pending        pendingTransfer
	panel          transferPanel
	form           addForm
	formIndex      int
	formPane       int
	categories     []string
	categoryIndex  int
	addingCategory bool
	categoryDraft  string
	editing        bool
	editIndex      int
	deleteIndex    int
	filter         filterMode
	sortBy         sortMode
	category       string
	activeTransfer activeTransfer
	collectRound   int
	manualRound    int
	pendingByRound map[int]int
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
	}
}

func New(hosts []host.Host, passwords config.PasswordStore) Model {
	home, _ := os.UserHomeDir()
	appConfig := config.LoadAppConfig(home)
	categories, _, _ := config.LoadCategories(home)
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
	case tea.KeyMsg:
		if m.mode == modeAddForm {
			return m.updateAddForm(msg)
		}
		if m.mode == modeDeleteConfirm {
			return m.updateDeleteConfirm(msg)
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
		case "t", "T":
			m.cycleCategory()
			m.selected = 0
		case " ":
			if _, ok := m.selectedRealIndex(); ok {
				m.mode = modeDetail
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
	switch msg.String() {
	case "esc", "q", "Q", "ctrl+c":
		m.mode = modeDashboard
		m.status = "已取消。"
	case "tab":
		m.formPane = 1
	case "down":
		m.formIndex = (m.formIndex + 1) % 8
	case "shift+tab":
		m.formPane = 1
	case "up":
		m.formIndex--
		if m.formIndex < 0 {
			m.formIndex = 7
		}
	case "left":
		if m.formIndex == 0 {
			m.moveCategory(-1)
		}
	case "right":
		if m.formIndex == 0 {
			m.moveCategory(1)
		}
	case "enter":
		input := config.HostInput{
			Category:     m.form.Category,
			Name:         m.form.Name,
			HostName:     m.form.HostName,
			User:         m.form.User,
			Port:         m.form.Port,
			IdentityFile: m.form.IdentityFile,
			ProxyJump:    m.form.ProxyJump,
			Password:     m.form.Password,
		}
		if m.editing {
			if m.editIndex < 0 || m.editIndex >= len(m.states) {
				m.status = "编辑失败：没有选中的服务器"
				return m, nil
			}
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
		if err := config.DeleteCategory(m.home, name); err != nil {
			m.status = "删除分类失败：" + categoryErrorText(err)
		} else {
			m.reloadCategories("")
			m.form.Category = m.categories[m.categoryIndex]
			m.status = "分类已删除。"
		}
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

func (m Model) startAddForm() Model {
	m.reloadCategories("")
	m.mode = modeAddForm
	m.formIndex = 0
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

func (m *Model) formAppend(s string) {
	switch m.formIndex {
	case 0:
		m.form.Category += s
	case 1:
		m.form.Name += s
	case 2:
		m.form.HostName += s
	case 3:
		m.form.User += s
	case 4:
		m.form.Port += s
	case 5:
		m.form.IdentityFile += s
	case 6:
		m.form.Password += s
	case 7:
		m.form.ProxyJump += s
	}
}

func (m *Model) formBackspace() {
	switch m.formIndex {
	case 0:
		return
	case 1:
		m.form.Name = removeLastRune(m.form.Name)
	case 2:
		m.form.HostName = removeLastRune(m.form.HostName)
	case 3:
		m.form.User = removeLastRune(m.form.User)
	case 4:
		m.form.Port = removeLastRune(m.form.Port)
	case 5:
		m.form.IdentityFile = removeLastRune(m.form.IdentityFile)
	case 6:
		m.form.Password = removeLastRune(m.form.Password)
	case 7:
		m.form.ProxyJump = removeLastRune(m.form.ProxyJump)
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
	switch msg.String() {
	case "esc", "q":
		m.mode = modeDashboard
		m.transfer = transferNone
		m.panel = transferPanel{}
		m.status = "已取消。"
	case "tab":
		if m.panel.ActivePane == 0 {
			m.panel.ActivePane = 1
		} else {
			m.panel.ActivePane = 0
		}
	case "j", "down":
		m.movePanel(1)
	case "k", "up":
		m.movePanel(-1)
	case "enter":
		m.togglePanelTree()
	case " ":
		return m.confirmTransferPanel()
	}
	return m, nil
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
	panel := transferPanel{Mode: mode, HostIndex: idx}
	if mode == transferUpload {
		panel.LeftTitle = "本地"
		panel.RightTitle = "远程"
		panel.LeftTree = newTree(localTreeItems("/", true), -1, false, true)
		panel.RightTree = newTree(fsselect.RemoteRootItems(h), idx, true, false)
		m.status = "上传：左侧选择本地文件/目录，右侧选择远程目录，空格开始。"
	} else {
		panel.LeftTitle = "远程"
		panel.RightTitle = "本地"
		panel.LeftTree = newTree(fsselect.RemoteRootItems(h), idx, false, false)
		panel.RightTree = newTree(localTreeItems("/", true), -1, true, true)
		m.status = "下载：左侧选择远程文件/目录，右侧选择本地目录，空格开始。"
	}
	panel.LeftChoices = flattenTree(panel.LeftTree)
	panel.RightChoices = flattenTree(panel.RightTree)
	m.mode = modeTransferPanel
	m.transfer = mode
	m.pending = pendingTransfer{HostIndex: idx}
	m.panel = panel
	return m
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
	if m.mode == modeDetail {
		return m.renderDetail()
	}
	if m.mode == modeTransferPanel {
		return m.renderTransferPanel()
	}
	if m.mode != modeDashboard {
		return m.renderPicker()
	}

	headerParts := []string{"sshm 服务器监控"}
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
	header := titleStyle.Render(fit(headerText, contentWidth(m.width)))

	var lines []string
	lines = append(lines, header)

	indexes := m.filteredIndexes()
	if len(m.states) == 0 {
		lines = append(lines, mutedStyle.Render("没有服务器。按 a 添加服务器。"))
	} else if len(indexes) == 0 {
		lines = append(lines, mutedStyle.Render("没有匹配的服务器"))
	} else {
		lines = append(lines, m.renderDashboardGrid(indexes))
	}

	lines = padToBottom(lines, m.height, 1)
	help := "↑↓←→/hjkl 移动  回车登录  空格详情  a添加  e编辑  x删除  u上传  d下载  r刷新  /搜索  t分类  o在线  p异常  s排序  q退出"
	lines = append(lines, helpStyle.Render(fit(help, contentWidth(m.width))))
	return strings.Join(lines, "\n")
}

func (m Model) renderAddForm() string {
	title := "添加服务器"
	if m.editing {
		title = "编辑服务器"
	}
	width := contentWidth(m.width)
	if width < 80 {
		width = 80
	}
	gap := 2
	leftWidth := (width - gap) * 2 / 3
	rightWidth := width - gap - leftWidth
	if rightWidth < 28 {
		rightWidth = 28
		leftWidth = width - gap - rightWidth
	}
	left := m.renderServerFormPane(title, leftWidth)
	right := m.renderCategoryPane(rightWidth)
	help := "Tab 切换区域  ↑↓选择  ←→切换分类  回车保存服务器  退出键取消"
	if m.formPane == 1 {
		help = "Tab 切回服务器  ↑↓选择分类  n添加分类  x删除分类  退出键取消"
		if m.addingCategory {
			help = "输入分类名称  回车添加  退出键取消输入"
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
	lines := []string{
		header,
		lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right),
		helpStyle.Render(fit(help, width)),
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderServerFormPane(title string, width int) string {
	fields := m.form.fields()
	lines := make([]string, 0, len(fields)+2)
	lines = append(lines, titleStyle.Render("服务器"))
	for i, field := range fields {
		prefix := " "
		style := lipgloss.NewStyle()
		value := field.value
		if i == 0 {
			value = m.form.Category
			if value == "" && len(m.categories) > 0 {
				value = m.categories[m.categoryIndex]
			}
			value += mutedStyle.Render("  ←/→")
		} else if m.formPane == 0 && i == m.formIndex {
			value += "█"
		}
		if m.formPane == 0 && i == m.formIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		lines = append(lines, style.Render(fit(fmt.Sprintf("%s %-10s %s", prefix, field.label, value), width-4)))
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(width)
	if m.formPane == 0 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCategoryPane(width int) string {
	lines := []string{titleStyle.Render("分类")}
	if len(m.categories) == 0 {
		lines = append(lines, mutedStyle.Render("没有分类"))
	} else {
		for i, category := range m.categories {
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.categoryIndex {
				prefix = "▶"
				if m.formPane == 1 && !m.addingCategory {
					style = blueStyle.Bold(true)
				}
			}
			count := m.categoryHostCount(category)
			label := category
			if count > 0 {
				label = fmt.Sprintf("%s  %d台", category, count)
			}
			lines = append(lines, style.Render(fit(prefix+" "+label, width-4)))
		}
	}
	lines = append(lines, "")
	if m.addingCategory {
		lines = append(lines, blueStyle.Bold(true).Render(fit("新分类 "+m.categoryDraft+"█", width-4)))
	} else {
		lines = append(lines, helpStyle.Render(fit("n 添加  x 删除", width-4)))
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).
		Width(width)
	if m.formPane == 1 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
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
	lines := []string{
		titleStyle.Render("确认删除服务器"),
		"",
		"服务器：" + h.Name,
		"文件：" + h.File,
		"",
		"将删除该服务器配置。",
		"",
		helpStyle.Render("回车/是 确认删除  退出键/否 取消"),
	}
	return detailStyle.BorderForeground(red).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDetail() string {
	idx, ok := m.selectedRealIndex()
	if !ok {
		return "没有选中的服务器"
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
		titleStyle.Render("服务器详情  " + h.Name),
		"",
		sectionTitle("基础信息"),
		detailRow("状态", colorStatus(status, state.Loading, metrics.Online)),
		detailRow("地址", h.Address()),
		detailRow("用户", h.User),
		detailRow("端口", h.Port),
		detailRow("分类", emptyDash(h.Category)),
		detailRow("系统", emptyDash(metrics.OS)),
		detailRow("来源", h.File),
		"",
		sectionTitle("资源监控"),
		detailRow("CPU", percentBar(metrics.CPUPercent)),
		detailRow("内存", fmt.Sprintf("%s  %s / %s", percentBar(metrics.MemPercent()), bytesHuman(metrics.MemUsed), bytesHuman(metrics.MemTotal))),
		detailRow("磁盘", fmt.Sprintf("%s  %s / %s", percentBar(metrics.DiskPercent()), bytesHuman(metrics.DiskUsed), bytesHuman(metrics.DiskTotal))),
		detailRow("负载", fmt.Sprintf("%s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15))),
		detailRow("运行", uptimeCN(metrics.Uptime)),
		"",
		sectionTitle("服务状态"),
		detailRow("容器", fmt.Sprintf("%d 运行中", metrics.DockerRunning)),
		detailRow("异常服务", failedServiceText(metrics, 8)),
		detailRow("监听端口", emptyDash(metrics.Ports)),
	}
	if metrics.Error != "" {
		lines = append(lines, "", sectionTitle("最近错误"), detailRow("错误", metrics.Error))
	}
	lines = append(lines, "", helpStyle.Render("回车 登录  u上传  d下载  r刷新  q/退出键 返回"))
	return detailStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderPicker() string {
	header := m.pickTitle
	if m.status != "" && m.status != m.pickTitle {
		header += "  " + m.status
	}
	lines := []string{titleStyle.Render(header), ""}
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
			lines = append(lines, style.Render(fmt.Sprintf("%s %s", prefix, label)))
		}
	}
	help := "↑↓/jk 移动  回车 选择  退出键 取消"
	if m.treePickerActive() {
		help = "↑↓/jk 移动  回车 展开/收起  空格 选择  退出键 取消"
	}
	lines = append(lines, "", helpStyle.Render(help))
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
	width := contentWidth(m.width)
	if width < 70 {
		width = 70
	}
	gap := 2
	paneWidth := (width - gap) / 2
	height := m.height - 5
	if height < 8 {
		height = 8
	}
	left := renderTransferPane(m.panel.LeftTitle, m.panel.LeftChoices, m.panel.LeftIndex, paneWidth, height, m.panel.ActivePane == 0)
	right := renderTransferPane(m.panel.RightTitle, m.panel.RightChoices, m.panel.RightIndex, paneWidth, height, m.panel.ActivePane == 1)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	help := "Tab 切换  ↑↓/jk 移动  回车 展开/收起  空格 开始  退出键 取消"
	return strings.Join([]string{
		titleStyle.Render(fit(header, width)),
		"",
		body,
		helpStyle.Render(fit(help, width)),
	}, "\n")
}

func renderTransferPane(title string, choices []choice, index int, width int, height int, active bool) string {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1).Width(width - 4)
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
	width := totalWidth - 1
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
	return withVerticalNav(strings.Join(out, "\n"), totalWidth, len(indexes), cols, startRow, rowsVisible)
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
	width := m.width - 1
	if width <= 0 {
		width = contentWidth(m.width) - 1
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
			track[i] = navStyle.Render("▌")
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
			track[i] = navStyle.Render("▌")
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
	categoryLabel := "[" + category + "]"
	categoryWidth := runewidth.StringWidth(categoryLabel)
	nameWidth := innerWidth - categoryWidth - 2
	if nameWidth < 8 {
		nameWidth = innerWidth
		categoryLabel = ""
	}
	name := fit(h.Name, nameWidth)
	barWidth := 32
	if innerWidth < 42 {
		barWidth = 22
	}
	cpu := percentBarWidth(metrics.CPUPercent, barWidth)
	mem := percentBarWidth(metrics.MemPercent(), barWidth)
	disk := percentBarWidth(metrics.DiskPercent(), barWidth)

	cpuLine := metricLine("CPU", cpu)
	memLine := metricLine("内存", mem)
	diskLine := metricLine("磁盘", disk)
	runLine := fit("⏱️ "+uptimeCN(metrics.Uptime), innerWidth)
	loadLine := fit(fmt.Sprintf("负载 %s / %s / %s", emptyDash(metrics.Load1), emptyDash(metrics.Load5), emptyDash(metrics.Load15)), innerWidth)
	serviceLine := fit(fmt.Sprintf("🐳 %d 运行中  ⚠️ %d", metrics.DockerRunning, metrics.FailedServices), innerWidth)
	titleText := name
	if categoryLabel != "" {
		titleText = categoryLabel + " " + name
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
		cardTopLine(cardWidth, title, stateMark, borderStyle),
		cardContentLine(cardWidth, addressLine, borderStyle),
		cardContentLine(cardWidth, cpuLine, borderStyle),
		cardContentLine(cardWidth, memLine, borderStyle),
		cardContentLine(cardWidth, diskLine, borderStyle),
		cardContentLine(cardWidth, loadLine, borderStyle),
		cardContentLine(cardWidth, serviceLine, borderStyle),
		cardContentLine(cardWidth, runLine, borderStyle),
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

func metricLine(label, value string) string {
	const labelWidth = 4
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return label + strings.Repeat(" ", padding) + value
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
	disk := compactMetric("磁盘", metrics.DiskPercent(), colWidth, barWidth)
	line := padVisible(cpu, colWidth) + strings.Repeat(" ", gap) + padVisible(mem, colWidth) + strings.Repeat(" ", gap) + padVisible(disk, colWidth)
	return fit(line, width)
}

func compactMetric(label string, value float64, width int, barWidth int) string {
	bar := compactPercentBar(value, barWidth)
	labelWidth := 4
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return fit(label+strings.Repeat(" ", padding)+bar, width)
}

func compactPercentBar(value float64, total int) string {
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
	bar := strings.Repeat("█", filled) + strings.Repeat("░", total-filled)
	return fmt.Sprintf("%s %3.0f%%", bar, value)
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

func cardTopLine(width int, title string, dot string, borderStyle lipgloss.Style) string {
	innerWidth := width - 2
	left := borderStyle.Render("╭")
	right := borderStyle.Render("╮")
	prefix := borderStyle.Render("─ ")
	titleGap := " "
	suffix := " " + dot + " "
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
	style := greenStyle
	if value >= 85 {
		style = redStyle
	} else if value >= 70 {
		style = yellowStyle
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", total-filled)
	return style.Render(fmt.Sprintf("%s %3.0f%%", bar, value))
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

func detailRow(label, value string) string {
	const labelWidth = 8
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	return mutedStyle.Render(label) + strings.Repeat(" ", padding) + value
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
	green  = lipgloss.Color("42")
	yellow = lipgloss.Color("214")
	red    = lipgloss.Color("196")
	blue   = lipgloss.Color("39")
	gray   = lipgloss.Color("245")
	dim    = lipgloss.Color("238")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(blue)
	mutedStyle = lipgloss.NewStyle().Foreground(gray)
	helpStyle  = lipgloss.NewStyle().Foreground(gray)
	navStyle   = lipgloss.NewStyle().Foreground(dim)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1).
			MarginBottom(0)
	selectedCardStyle       = cardStyle.BorderForeground(blue)
	cardBorderStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
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
