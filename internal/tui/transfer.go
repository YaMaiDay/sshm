package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	transferservice "github.com/YaMaiDay/sshm/internal/transfer"
)

func (m Model) updateTransferPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.transferState.Panel.NeedsInstall {
		key := shortcutKey(msg)
		switch key {
		case "i":
			m.status = m.t("Installing rsync on remote host...", "正在远程安装 rsync...")
			return m, m.installRemoteRsync(m.transferState.Panel.HostIndex)
		case "esc", "q":
			m.transferState.Panel.NeedsInstall = false
			m.status = m.t("Canceled.", "已取消。")
			return m, nil
		}
		return m, nil
	}
	if m.transferState.Panel.Confirming && msg.String() != "enter" {
		m.transferState.Panel.Confirming = false
		m.status = m.transferPanelStatus(m.transferState.Panel.Mode)
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeDashboard
		m.transferState.Mode = transferNone
		m.transferState.Panel = transferPanel{}
		m.status = m.t("Canceled.", "已取消。")
	case "tab":
		m.cancelTransferConfirm()
		if m.transferState.Panel.ActivePane == 0 {
			m.transferState.Panel.ActivePane = 1
		} else {
			m.transferState.Panel.ActivePane = 0
		}
	case "j", "down":
		m.cancelTransferConfirm()
		m.movePanel(1)
	case "k", "up":
		m.cancelTransferConfirm()
		m.movePanel(-1)
	case "enter":
		if m.transferState.Panel.Confirming {
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
	if m.transferState.Panel.NeedsInstall {
		key := shortcutKey(msg)
		switch key {
		case "i":
			m.status = m.t("Installing rsync on remote host...", "正在远程安装 rsync...")
			return m, m.installRemoteRsync(m.transferState.Panel.HostIndex)
		case "esc", "q":
			m.transferState.Panel.NeedsInstall = false
			m.status = m.t("Canceled rsync install.", "已取消安装 rsync。")
			return m, nil
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = m.transferState.JobsBack
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
		m.transferState.RunAll = false
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
	if len(m.transferState.History.Entries) == 0 || m.transferState.Index < 0 || m.transferState.Index >= len(m.transferState.History.Entries) {
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
		m.transferState.RunAll = false
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
		m.transferState.Index = 0
		return
	}
	pos := 0
	for i, index := range indexes {
		if index == m.transferState.Index {
			pos = i
			break
		}
	}
	pos = clampInt(pos+delta, 0, len(indexes)-1)
	m.transferState.Index = indexes[pos]
}

func (m *Model) cycleTransferStatusFilter() {
	m.transferState.StatusFilter++
	if m.transferState.StatusFilter >= len(transferStatusFilterOptions()) {
		m.transferState.StatusFilter = 0
	}
	m.ensureTransferIndexVisible()
}

func (m *Model) cancelTransferConfirm() {
	if m.transferState.Panel.Confirming {
		m.transferState.Panel.Confirming = false
		m.status = m.transferPanelStatus(m.transferState.Panel.Mode)
	}
}

func (m *Model) movePanel(delta int) {
	if m.transferState.Panel.ActivePane == 0 {
		m.transferState.Panel.LeftIndex = moveIndex(m.transferState.Panel.LeftIndex, len(m.transferState.Panel.LeftChoices), delta)
		return
	}
	m.transferState.Panel.RightIndex = moveIndex(m.transferState.Panel.RightIndex, len(m.transferState.Panel.RightChoices), delta)
}

func (m *Model) togglePanelSelection() {
	if m.transferState.Panel.ActivePane != 0 {
		return
	}
	if len(m.transferState.Panel.LeftChoices) == 0 || m.transferState.Panel.LeftIndex < 0 || m.transferState.Panel.LeftIndex >= len(m.transferState.Panel.LeftChoices) {
		return
	}
	pick := m.transferState.Panel.LeftChoices[m.transferState.Panel.LeftIndex]
	if m.transferState.Panel.LeftSelected == nil {
		m.transferState.Panel.LeftSelected = map[string]bool{}
	}
	if m.transferState.Panel.LeftSelected[pick.Value] {
		delete(m.transferState.Panel.LeftSelected, pick.Value)
	} else {
		m.transferState.Panel.LeftSelected[pick.Value] = true
	}
}

func (m Model) selectedTransferSources() []choice {
	if len(m.transferState.Panel.LeftSelected) == 0 {
		return nil
	}
	out := make([]choice, 0, len(m.transferState.Panel.LeftSelected))
	for path := range m.transferState.Panel.LeftSelected {
		node := m.transferState.Panel.LeftTree.Nodes[path]
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
	if m.transferState.Panel.ActivePane == 0 {
		return &m.transferState.Panel.LeftTree, &m.transferState.Panel.LeftChoices, &m.transferState.Panel.LeftIndex
	}
	return &m.transferState.Panel.RightTree, &m.transferState.Panel.RightChoices, &m.transferState.Panel.RightIndex
}

func (m Model) confirmTransferPanel() (tea.Model, tea.Cmd) {
	m.transferState.Panel.Confirming = false
	if len(m.selectedTransferSources()) == 0 || len(m.transferState.Panel.RightChoices) == 0 {
		m.status = m.t("Select at least one source on the left and a target directory on the right.", "左侧至少选择一个文件或目录，右侧选择目标目录。")
		return m, nil
	}
	right := m.transferState.Panel.RightChoices[m.transferState.Panel.RightIndex]
	if !right.IsDir {
		m.status = m.t("The right side must be a directory.", "右侧必须选择目录。")
		return m, nil
	}
	m.transferState.JobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.status = m.t("Checking remote rsync...", "正在检测远程 rsync...")
	return m, m.checkRemoteRsync(m.transferState.Panel.HostIndex)
}

func (m Model) prepareTransferConfirm() (tea.Model, tea.Cmd) {
	selected := m.selectedTransferSources()
	if len(selected) == 0 || len(m.transferState.Panel.RightChoices) == 0 {
		m.status = m.t("Select at least one source on the left and a target directory on the right.", "左侧至少选择一个文件或目录，右侧选择目标目录。")
		return m, nil
	}
	right := m.transferState.Panel.RightChoices[m.transferState.Panel.RightIndex]
	if !right.IsDir {
		m.status = m.t("The right side must be a directory.", "右侧必须选择目录。")
		return m, nil
	}
	h := m.states[m.transferState.Panel.HostIndex].Host
	m.transferState.Panel.Confirming = true
	if m.transferState.Panel.Mode == transferUpload {
		m.status = fmt.Sprintf(m.t("Upload Enter: %d items -> %s:%s/  Cancel Esc", "上传 Enter：%d 项 -> %s:%s/  取消 Esc"), len(selected), hostDisplayName(h), right.Value)
		return m, nil
	}
	m.status = fmt.Sprintf(m.t("Download Enter: %d items -> %s/  Cancel Esc", "下载 Enter：%d 项 -> %s/  取消 Esc"), len(selected), right.Value)
	return m, nil
}

func (m *Model) movePick(delta int) {
	if len(m.transferState.Choices) == 0 {
		m.transferState.PickIndex = 0
		return
	}
	m.transferState.PickIndex += delta
	if m.transferState.PickIndex < 0 {
		m.transferState.PickIndex = len(m.transferState.Choices) - 1
	}
	if m.transferState.PickIndex >= len(m.transferState.Choices) {
		m.transferState.PickIndex = 0
	}
}

func (m Model) confirmPick() (tea.Model, tea.Cmd) {
	if len(m.transferState.Choices) == 0 || m.transferState.PickIndex < 0 || m.transferState.PickIndex >= len(m.transferState.Choices) {
		m.status = m.t("No selectable item.", "没有可选择的项目。")
		return m, nil
	}
	pick := m.transferState.Choices[m.transferState.PickIndex]
	switch m.mode {
	case modePickLocalRoot:
		m.transferState.Pending.LocalRoot = pick.Value
		m.setChoices(m.t("Select local file/dir", "选择本地文件/目录"), modePickLocalItem, localItemChoices(fsselect.LocalItems(pick.Value)))
	case modePickLocalItem:
		m.transferState.Pending.LocalPath = pick.Value
		m.transferState.Pending.LocalIsDir = pick.IsDir
		h := m.states[m.transferState.Pending.HostIndex].Host
		m.startRemoteTree(m.t("Select remote dir", "选择远程目录"), modePickRemoteDir, h, true)
	case modePickRemoteDir:
		m.transferState.Pending.RemoteDir = pick.Value
		return m.startUploadTransfer()
	case modePickRemoteItem:
		m.transferState.Pending.RemotePath = pick.Value
		m.transferState.Pending.RemoteIsDir = pick.IsDir
		m.startLocalTree(m.t("Select local save dir", "选择本地保存目录"), modePickSaveDir, true)
	case modePickSaveDir:
		m.transferState.Pending.SaveDir = pick.Value
		return m.startDownloadTransfer()
	}
	return m, nil
}

func (m *Model) setChoices(title string, mode viewMode, choices []choice) {
	m.transferState.PickTitle = title
	m.mode = mode
	m.transferState.Choices = choices
	m.transferState.PickIndex = 0
	if len(choices) == 0 {
		m.status = title + m.t(": no selectable items", "：没有可选择的项目")
	} else {
		m.status = title
	}
}

func (m Model) startTransfer(status string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.mode = modeDashboard
	m.transferState.Mode = transferNone
	m.transferState.Choices = nil
	m.transferState.RemoteTree = remoteTree{}
	m.transferState.PickIndex = 0
	m.status = status
	return m, cmd
}

func (m Model) startUploadTransfer() (tea.Model, tea.Cmd) {
	h := m.states[m.transferState.Pending.HostIndex].Host
	localPath := m.transferState.Pending.LocalPath
	remoteDir := m.transferState.Pending.RemoteDir
	remotePath := remoteJoin(remoteDir, filepath.Base(localPath))
	total := transferservice.LocalSizeBytes(localPath)
	ctx, cancel := context.WithCancel(context.Background())
	m.mode = modeDashboard
	m.transferState.Mode = transferNone
	m.transferState.Choices = nil
	m.transferState.RemoteTree = remoteTree{}
	m.transferState.PickIndex = 0
	m.transferState.Active = activeTransfer{
		Kind:       m.t("Upload", "上传"),
		Source:     localPath,
		Target:     h.Name + ":" + remoteDir + "/",
		LocalPath:  localPath,
		RemotePath: remotePath,
		HostIndex:  m.transferState.Pending.HostIndex,
		Total:      total,
		Active:     true,
		Cancel:     cancel,
	}
	m.status = m.transferProgressText(m.transferState.Active)
	return m, tea.Batch(m.runUpload(ctx), transferProgressAfter(500*time.Millisecond))
}

func (m Model) startDownloadTransfer() (tea.Model, tea.Cmd) {
	h := m.states[m.transferState.Pending.HostIndex].Host
	remotePath := m.transferState.Pending.RemotePath
	saveDir := m.transferState.Pending.SaveDir
	localPath := filepath.Join(saveDir, filepath.Base(remotePath))
	total := transferservice.RemoteSizeBytes(h, remotePath)
	if total < 0 {
		total = 0
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mode = modeDashboard
	m.transferState.Mode = transferNone
	m.transferState.Choices = nil
	m.transferState.RemoteTree = remoteTree{}
	m.transferState.PickIndex = 0
	m.transferState.Active = activeTransfer{
		Kind:      m.t("Download", "下载"),
		Source:    remotePath,
		Target:    saveDir + "/",
		LocalPath: localPath,
		HostIndex: m.transferState.Pending.HostIndex,
		Total:     total,
		Active:    true,
		Cancel:    cancel,
	}
	m.status = m.transferProgressText(m.transferState.Active)
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
	if len(selected) == 0 || len(m.transferState.Panel.RightChoices) == 0 {
		m.status = m.t("No transferable items.", "没有可传输的项目。")
		return m, nil
	}
	target := m.transferState.Panel.RightChoices[m.transferState.Panel.RightIndex]
	h := m.states[m.transferState.Panel.HostIndex].Host
	now := time.Now()
	for i, item := range selected {
		totalBytes := int64(0)
		if m.transferState.Panel.Mode == transferDownload {
			totalBytes = transferservice.RemoteSizeBytes(h, item.Value)
		} else {
			totalBytes = transferservice.LocalSizeBytes(item.Value)
		}
		entry := transferservice.BuildEntry(h, transferservice.EntrySpec{
			ID:         config.NewTransferID(now.Add(time.Duration(i))),
			Time:       now,
			Kind:       transferKindString(m.transferState.Panel.Mode),
			Source:     item.Value,
			TargetDir:  target.Value,
			IsDir:      item.IsDir,
			TotalBytes: totalBytes,
		})
		if err := m.appendTransferEntry(entry); err != nil {
			m.setPersistenceError("Save transfer job failed", "保存传输任务失败", err)
			return m, nil
		}
	}
	m.reloadTransfers()
	m.transferState.JobsBack = modeTransferPanel
	m.mode = modeTransferJobs
	m.transferState.Mode = transferNone
	m.status = fmt.Sprintf(m.t("Created %d transfer jobs.", "已创建 %d 个传输任务。"), len(selected))
	return m, nil
}

func transferKindString(mode transferMode) string {
	if mode == transferDownload {
		return "download"
	}
	return "upload"
}

func (m *Model) ensureTransferIndexVisible() {
	indexes := m.filteredTransferIndexes()
	if len(indexes) == 0 {
		m.transferState.Index = 0
		return
	}
	for _, index := range indexes {
		if index == m.transferState.Index {
			return
		}
	}
	m.transferState.Index = indexes[0]
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
	m.transferState.Mode = mode
	m.transferState.Pending = pendingTransfer{HostIndex: idx}
	m.transferState.Panel = panel
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
	h := m.states[m.transferState.Pending.HostIndex].Host
	localPath := m.transferState.Pending.LocalPath
	remoteDir := m.transferState.Pending.RemoteDir
	recursive := m.transferState.Pending.LocalIsDir
	return func() tea.Msg {
		result := (transferservice.Service{}).Upload(ctx, h, localPath, remoteDir, recursive)
		return transferDoneMsg{Kind: m.t("Upload", "上传"), Source: localPath, Target: h.Name + ":" + remoteDir + "/", Err: result.Err, Output: result.Output}
	}
}

func (m Model) runDownload(ctx context.Context) tea.Cmd {
	h := m.states[m.transferState.Pending.HostIndex].Host
	remotePath := m.transferState.Pending.RemotePath
	saveDir := m.transferState.Pending.SaveDir
	recursive := m.transferState.Pending.RemoteIsDir
	return func() tea.Msg {
		result := (transferservice.Service{}).Download(ctx, h, remotePath, saveDir, recursive)
		return transferDoneMsg{Kind: m.t("Download", "下载"), Source: remotePath, Target: saveDir + "/", Err: result.Err, Output: result.Output}
	}
}

func (m Model) startNextQueuedTransfer() (tea.Model, tea.Cmd) {
	if m.transferState.Active.Active {
		return m, nil
	}
	for _, entry := range m.transferState.History.Entries {
		if entry.Status == config.TransferStatusPending {
			return m.startTransferEntry(entry)
		}
	}
	m.transferState.RunAll = false
	return m, clearStatusAfter(3 * time.Second)
}

func (m Model) startTransferEntry(entry config.TransferEntry) (tea.Model, tea.Cmd) {
	h, index, ok := m.findTransferHost(entry)
	if !ok {
		transferservice.SetEntryStatus(&entry, config.TransferStatusFailed, m.t("Server not found: ", "找不到服务器：")+entry.HostName)
		if err := m.updateTransferEntryAndReload(entry); err != nil {
			m.setPersistenceError("Save transfer job failed", "保存传输任务失败", err)
		}
		return m, clearStatusAfter(3 * time.Second)
	}
	ctx, cancel := context.WithCancel(context.Background())
	transferservice.SetEntryStatus(&entry, config.TransferStatusRunning, "")
	if err := m.updateTransferEntry(entry); err != nil {
		cancel()
		m.setPersistenceError("Save transfer job failed", "保存传输任务失败", err)
		return m, nil
	}
	m.transferState.Active = activeTransfer{
		ID:        entry.ID,
		Kind:      m.transferEntryKindText(entry),
		Source:    entry.Source,
		Target:    entry.TargetDir,
		HostIndex: index,
		Active:    true,
		Cancel:    cancel,
	}
	m.reloadTransfers()
	m.status = m.transferProgressText(m.transferState.Active)
	cmd := func() tea.Msg {
		var progressMu sync.Mutex
		var progressErr error
		result := (transferservice.Service{}).RunJob(ctx, h, entry, func(progress string) {
			if err := updateTransferProgress(m.home, entry.ID, progress); err != nil {
				progressMu.Lock()
				if progressErr == nil {
					progressErr = err
				}
				progressMu.Unlock()
			}
		})
		progressMu.Lock()
		persistenceErr := ""
		if progressErr != nil {
			persistenceErr = progressErr.Error()
		}
		progressMu.Unlock()
		cancel()
		return transferDoneMsg{ID: entry.ID, Kind: m.transferEntryKindText(entry), Source: entry.Source, Target: entry.TargetDir, Err: result.Err, Output: result.Output, PersistenceErr: persistenceErr}
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
