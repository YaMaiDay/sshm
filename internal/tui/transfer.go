package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) updateTransferPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.panel.NeedsInstall {
		key := shortcutKey(msg)
		switch key {
		case "i":
			m.status = m.t("Installing rsync on remote host...", "正在远程安装 rsync...")
			return m, m.installRemoteRsync(m.panel.HostIndex)
		case "esc", "q":
			m.panel.NeedsInstall = false
			m.status = m.t("Canceled.", "已取消。")
			return m, nil
		}
		return m, nil
	}
	if m.panel.Confirming && msg.String() != "enter" {
		m.panel.Confirming = false
		m.status = m.transferPanelStatus(m.panel.Mode)
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeDashboard
		m.transfer = transferNone
		m.panel = transferPanel{}
		m.status = m.t("Canceled.", "已取消。")
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
			m.status = m.t("Installing rsync on remote host...", "正在远程安装 rsync...")
			return m, m.installRemoteRsync(m.panel.HostIndex)
		case "esc", "q":
			m.panel.NeedsInstall = false
			m.status = m.t("Canceled rsync install.", "已取消安装 rsync。")
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
		m.detailScroll = moveClampedInt(m.detailScroll, 1, 0, m.transferDetailMaxScroll())
	case "k", "up":
		m.detailScroll = moveClampedInt(m.detailScroll, -1, 0, m.transferDetailMaxScroll())
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

func (m *Model) cancelTransferConfirm() {
	if m.panel.Confirming {
		m.panel.Confirming = false
		m.status = m.transferPanelStatus(m.panel.Mode)
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
			m.status = m.t("No subdirectories: ", "没有子目录：") + node.Item.Path
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
		m.status = m.t("Select at least one source on the left and a target directory on the right.", "左侧至少选择一个文件或目录，右侧选择目标目录。")
		return m, nil
	}
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = m.t("The right side must be a directory.", "右侧必须选择目录。")
		return m, nil
	}
	m.transferJobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.status = m.t("Checking remote rsync...", "正在检测远程 rsync...")
	return m, m.checkRemoteRsync(m.panel.HostIndex)
}

func (m Model) prepareTransferConfirm() (tea.Model, tea.Cmd) {
	selected := m.selectedTransferSources()
	if len(selected) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = m.t("Select at least one source on the left and a target directory on the right.", "左侧至少选择一个文件或目录，右侧选择目标目录。")
		return m, nil
	}
	right := m.panel.RightChoices[m.panel.RightIndex]
	if !right.IsDir {
		m.status = m.t("The right side must be a directory.", "右侧必须选择目录。")
		return m, nil
	}
	h := m.states[m.panel.HostIndex].Host
	m.panel.Confirming = true
	if m.panel.Mode == transferUpload {
		m.status = fmt.Sprintf(m.t("Upload Enter: %d items -> %s:%s/  Cancel Esc", "上传 Enter：%d 项 -> %s:%s/  取消 Esc"), len(selected), hostDisplayName(h), right.Value)
		return m, nil
	}
	m.status = fmt.Sprintf(m.t("Download Enter: %d items -> %s/  Cancel Esc", "下载 Enter：%d 项 -> %s/  取消 Esc"), len(selected), right.Value)
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
		m.status = m.t("No selectable item.", "没有可选择的项目。")
		return m, nil
	}
	pick := m.choices[m.pickIndex]
	switch m.mode {
	case modePickLocalRoot:
		m.pending.LocalRoot = pick.Value
		m.setChoices(m.t("Select local file/dir", "选择本地文件/目录"), modePickLocalItem, localItemChoices(fsselect.LocalItems(pick.Value)))
	case modePickLocalItem:
		m.pending.LocalPath = pick.Value
		m.pending.LocalIsDir = pick.IsDir
		h := m.states[m.pending.HostIndex].Host
		m.startRemoteTree(m.t("Select remote dir", "选择远程目录"), modePickRemoteDir, h, true)
	case modePickRemoteDir:
		m.pending.RemoteDir = pick.Value
		return m.startUploadTransfer()
	case modePickRemoteItem:
		m.pending.RemotePath = pick.Value
		m.pending.RemoteIsDir = pick.IsDir
		m.startLocalTree(m.t("Select local save dir", "选择本地保存目录"), modePickSaveDir, true)
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
		m.status = title + m.t(": no selectable items", "：没有可选择的项目")
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
	total := transferservice.LocalSizeBytes(localPath)
	ctx, cancel := context.WithCancel(context.Background())
	m.mode = modeDashboard
	m.transfer = transferNone
	m.choices = nil
	m.remoteTree = remoteTree{}
	m.pickIndex = 0
	m.activeTransfer = activeTransfer{
		Kind:       m.t("Upload", "上传"),
		Source:     localPath,
		Target:     h.Name + ":" + remoteDir + "/",
		LocalPath:  localPath,
		RemotePath: remotePath,
		HostIndex:  m.pending.HostIndex,
		Total:      total,
		Active:     true,
		Cancel:     cancel,
	}
	m.status = m.transferProgressText(m.activeTransfer)
	return m, tea.Batch(m.runUpload(ctx), transferProgressAfter(500*time.Millisecond))
}

func (m Model) startDownloadTransfer() (tea.Model, tea.Cmd) {
	h := m.states[m.pending.HostIndex].Host
	remotePath := m.pending.RemotePath
	saveDir := m.pending.SaveDir
	localPath := filepath.Join(saveDir, filepath.Base(remotePath))
	total := transferservice.RemoteSizeBytes(h, remotePath)
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
		Kind:      m.t("Download", "下载"),
		Source:    remotePath,
		Target:    saveDir + "/",
		LocalPath: localPath,
		HostIndex: m.pending.HostIndex,
		Total:     total,
		Active:    true,
		Cancel:    cancel,
	}
	m.status = m.transferProgressText(m.activeTransfer)
	return m, tea.Batch(m.runDownload(ctx), transferProgressAfter(500*time.Millisecond))
}

func (m Model) checkRemoteRsync(index int) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.states) {
			return rsyncCheckMsg{HostIndex: index, ErrText: m.t("Invalid server index", "服务器索引无效")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), m.appConfig.CommandDuration())
		defer cancel()
		if err := (transferservice.Service{}).CheckRsync(ctx, m.states[index].Host); err == nil {
			return rsyncCheckMsg{HostIndex: index}
		}
		return rsyncCheckMsg{HostIndex: index, Missing: true}
	}
}

func (m Model) installRemoteRsync(index int) tea.Cmd {
	return func() tea.Msg {
		if index < 0 || index >= len(m.states) {
			return rsyncInstallMsg{HostIndex: index, ErrText: m.t("Invalid server index", "服务器索引无效")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		output, err := (transferservice.Service{}).InstallRsync(ctx, m.states[index].Host)
		if err != nil {
			return rsyncInstallMsg{HostIndex: index, ErrText: transferErrorText(err, output)}
		}
		return rsyncInstallMsg{HostIndex: index}
	}
}

func (m Model) createTransferJobsFromPanel() (tea.Model, tea.Cmd) {
	selected := m.selectedTransferSources()
	if len(selected) == 0 || len(m.panel.RightChoices) == 0 {
		m.status = m.t("No transferable items.", "没有可传输的项目。")
		return m, nil
	}
	target := m.panel.RightChoices[m.panel.RightIndex]
	h := m.states[m.panel.HostIndex].Host
	now := time.Now()
	for i, item := range selected {
		totalBytes := int64(0)
		if m.panel.Mode == transferDownload {
			totalBytes = transferservice.RemoteSizeBytes(h, item.Value)
		} else {
			totalBytes = transferservice.LocalSizeBytes(item.Value)
		}
		entry := transferservice.BuildEntry(h, transferservice.EntrySpec{
			ID:         config.NewTransferID(now.Add(time.Duration(i))),
			Time:       now,
			Kind:       transferKindString(m.panel.Mode),
			Source:     item.Value,
			TargetDir:  target.Value,
			IsDir:      item.IsDir,
			TotalBytes: totalBytes,
		})
		m.appendTransferEntry(entry)
	}
	m.reloadTransfers()
	m.transferJobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.transfer = transferNone
	m.status = fmt.Sprintf(m.t("Created %d transfer jobs.", "已创建 %d 个传输任务。"), len(selected))
	return m, nil
}

func transferKindString(mode transferMode) string {
	if mode == transferDownload {
		return "download"
	}
	return "upload"
}

func (m *Model) reloadTransfers() {
	file, _, _ := transferservice.LoadHistory(m.home)
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

func (m Model) startTransferPanel(idx int, mode transferMode) Model {
	h := m.states[idx].Host
	remoteTitle := m.t("Remote ", "远程 ") + hostDisplayName(h)
	panel := transferPanel{Mode: mode, HostIndex: idx, LeftSelected: map[string]bool{}}
	if mode == transferUpload {
		panel.LeftTitle = m.t("Local", "本地")
		panel.RightTitle = remoteTitle
		panel.LeftTree = newTree(m.localRootItems(false), -1, false, true)
		panel.RightTree = newTree(m.remoteRootItems(h), idx, true, false)
		m.status = m.transferPanelStatus(mode)
	} else {
		panel.LeftTitle = remoteTitle
		panel.RightTitle = m.t("Local", "本地")
		panel.LeftTree = newTree(m.remoteRootItems(h), idx, false, false)
		panel.RightTree = newTree(m.localRootItems(true), -1, true, true)
		m.status = m.transferPanelStatus(mode)
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

func (m Model) transferPanelStatus(mode transferMode) string {
	if mode == transferUpload {
		return m.t("Upload: select local files/dirs on the left, choose remote dir on the right, press s to start.", "上传：左侧多选本地文件/目录，右侧选择远程目录，s 开始。")
	}
	return m.t("Download: select remote files/dirs on the left, choose local dir on the right, press s to start.", "下载：左侧多选远程文件/目录，右侧选择本地目录，s 开始。")
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
	return func() tea.Msg {
		result := (transferservice.Service{}).Upload(ctx, h, localPath, remoteDir, recursive)
		return transferDoneMsg{Kind: m.t("Upload", "上传"), Source: localPath, Target: h.Name + ":" + remoteDir + "/", Err: result.Err, Output: result.Output}
	}
}

func (m Model) runDownload(ctx context.Context) tea.Cmd {
	h := m.states[m.pending.HostIndex].Host
	remotePath := m.pending.RemotePath
	saveDir := m.pending.SaveDir
	recursive := m.pending.RemoteIsDir
	return func() tea.Msg {
		result := (transferservice.Service{}).Download(ctx, h, remotePath, saveDir, recursive)
		return transferDoneMsg{Kind: m.t("Download", "下载"), Source: remotePath, Target: saveDir + "/", Err: result.Err, Output: result.Output}
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
		transferservice.SetEntryStatus(&entry, config.TransferStatusFailed, m.t("Server not found: ", "找不到服务器：")+entry.HostName)
		m.updateTransferEntryAndReload(entry)
		return m, clearStatusAfter(3 * time.Second)
	}
	ctx, cancel := context.WithCancel(context.Background())
	transferservice.SetEntryStatus(&entry, config.TransferStatusRunning, "")
	m.updateTransferEntry(entry)
	m.activeTransfer = activeTransfer{
		ID:        entry.ID,
		Kind:      m.transferEntryKindText(entry),
		Source:    entry.Source,
		Target:    entry.TargetDir,
		HostIndex: index,
		Active:    true,
		Cancel:    cancel,
	}
	m.reloadTransfers()
	m.status = m.transferProgressText(m.activeTransfer)
	cmd := func() tea.Msg {
		result := (transferservice.Service{}).RunJob(ctx, h, entry, func(progress string) {
			updateTransferProgress(m.home, entry.ID, progress)
		})
		cancel()
		return transferDoneMsg{ID: entry.ID, Kind: m.transferEntryKindText(entry), Source: entry.Source, Target: entry.TargetDir, Err: result.Err, Output: result.Output}
	}
	return m, tea.Batch(cmd, transferProgressAfter(500*time.Millisecond))
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
