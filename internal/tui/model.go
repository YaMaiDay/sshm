package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	commandservice "github.com/YaMaiDay/sshm/internal/command"
	"github.com/YaMaiDay/sshm/internal/config"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func New(hosts []host.Host, passwords config.PasswordStore) Model {
	home, _ := os.UserHomeDir()
	appConfig := config.LoadAppConfig(home)
	appState := config.LoadState(home)
	startupWarnings := []string{}
	categories, _, err := config.LoadCategories(home)
	if err != nil {
		startupWarnings = append(startupWarnings, "categories: "+err.Error())
	}
	commandFile, _, err := commandservice.LoadTemplates(home)
	if err != nil {
		startupWarnings = append(startupWarnings, "commands: "+err.Error())
	}
	deploymentFile, _, err := deploymentservice.LoadFile(home)
	if err != nil {
		startupWarnings = append(startupWarnings, "deployments: "+err.Error())
	}
	resourceFile, _, err := resourceservice.LoadConfig(home)
	if err != nil {
		startupWarnings = append(startupWarnings, "resources: "+err.Error())
	}
	if err := transferservice.MarkRunningTransfersInterrupted(home); err != nil {
		startupWarnings = append(startupWarnings, "transfers: "+err.Error())
	}
	transferHistory, _, err := transferservice.LoadHistory(home)
	if err != nil {
		startupWarnings = append(startupWarnings, "transfer history: "+err.Error())
	}
	setASCIIMode(appConfig.ASCIIMode)
	states := make([]hostState, len(hosts))
	for i, h := range hosts {
		states[i] = hostState{Host: h, Loading: true}
	}
	pendingByRound := map[int]int{1: len(states)}
	collector := monitor.NewCollector(passwords)
	collector.Timeout = appConfig.CommandDuration()
	collector.ConnectTimeout = appConfig.ConnectDuration()
	m := Model{
		states:      states,
		collector:   collector,
		passwords:   passwords,
		appConfig:   appConfig,
		appState:    appState,
		home:        home,
		commandFile: commandFile,
		deploymentState: deploymentState{
			File:     deploymentFile,
			Progress: newDeploymentProgressStore(),
		},
		resourceState:  resourceState{File: resourceFile},
		transferState:  transferState{History: transferHistory},
		categories:     categories,
		status:         "",
		collectRound:   1,
		pendingByRound: pendingByRound,
	}
	if len(startupWarnings) > 0 {
		m.status = m.t("Config load warnings: ", "配置读取警告：") + strings.Join(startupWarnings, "; ")
	} else {
		m.status = m.t("Collecting server status...", "正在采集服务器状态...")
	}
	return m
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
		return m.updateTick()
	case collectMsg:
		return m.updateCollect(msg)
	case transferDoneMsg:
		return m.updateTransferDone(msg)
	case rsyncCheckMsg:
		return m.updateRsyncCheck(msg)
	case rsyncInstallMsg:
		return m.updateRsyncInstall(msg)
	case transferProgressMsg:
		return m.updateTransferProgress()
	case clearStatusMsg:
		return m.updateClearStatus()
	case sshDoneMsg:
		return m.updateSSHDone(msg)
	case loginRecordsMsg:
		return m.updateLoginRecords(msg)
	case resourceLoadMsg:
		return m.handleResourceLoad(msg)
	case resourceContainerDetailMsg:
		return m.handleResourceContainerDetail(msg)
	case resourceServiceDetailMsg:
		return m.handleResourceServiceDetail(msg)
	case resourceProcessDetailMsg:
		return m.handleResourceProcessDetail(msg)
	case resourceDatabaseDetailMsg:
		return m.handleResourceDatabaseDetail(msg)
	case resourceLogMsg:
		return m.handleResourceLog(msg)
	case resourceActionMsg:
		return m.handleResourceAction(msg)
	case commandDoneMsg:
		return m.updateCommandDone(msg)
	case batchCommandDoneMsg:
		return m.handleBatchCommandDone(msg)
	case deploymentDoneMsg:
		return m.handleDeploymentDone(msg)
	case deploymentQueueNextMsg:
		return m.startNextQueuedDeployment()
	case deploymentProgressMsg:
		return m.handleDeploymentProgress(msg)
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) View() string {
	if view, ok := m.viewByMode(); ok {
		return view
	}
	return m.renderDashboardView()
}
