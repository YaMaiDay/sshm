package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
)

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
