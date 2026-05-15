package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
)

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
