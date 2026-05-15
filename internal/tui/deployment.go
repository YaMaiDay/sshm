package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startDeploymentList(index int) Model {
	file, _, err := config.LoadDeployments(m.home)
	if err != nil {
		m.status = m.t("Failed to read deployment config: ", "读取部署配置失败：") + err.Error()
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
	m.status = m.t("Deployments", "应用部署")
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
			m.status = m.t("Deployment Detail", "部署详情")
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
		m.status = m.t("No editable deployment app. Press a to add one.", "没有可编辑的部署应用。按 a 新增。")
	case "x":
		item, ok := m.selectedDeploymentItem()
		if ok {
			if item.Index >= 0 && item.Index < len(m.deploymentFile.Apps) {
				m.confirm = confirmAction{
					Kind:       confirmDeleteDeployment,
					Title:      m.t("Delete Deployment App", "确认删除部署应用"),
					Lines:      []string{m.t("This deployment app will be deleted.", "将删除该部署应用。"), m.t("App: ", "应用：") + item.App.Name, m.t("Server: ", "服务器：") + item.App.Server},
					Back:       modeDeploymentList,
					Deployment: item.App,
					Index:      item.Index,
				}
				m.mode = modeConfirmAction
				m.status = m.t("Delete Deployment App", "确认删除部署应用")
				return m, nil
			}
		}
		m.status = m.t("No deployment app to delete.", "没有可删除的部署应用。")
	case "enter":
		item, ok := m.selectedDeploymentItem()
		if !ok {
			m.status = m.t("No deployable app. Press a to add one, save it, then press Enter.", "没有可部署的应用。按 a 新增，保存后再按 Enter。")
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
		m.status = m.t("Confirm Deployment", "确认部署")
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
		m.status = m.t("Filter: favorite deployments", "筛选：收藏部署")
	} else {
		m.status = m.t("Favorites filter cleared", "已取消收藏筛选")
	}
}

func (m Model) toggleDeploymentFavorite() (tea.Model, tea.Cmd) {
	item, ok := m.selectedDeploymentItem()
	if !ok {
		m.status = m.t("No deployment app to favorite", "没有可收藏的部署应用")
		return m, nil
	}
	file := m.deploymentFile
	if item.Index < 0 || item.Index >= len(file.Apps) {
		m.status = m.t("Deployment app does not exist", "部署应用不存在")
		return m, nil
	}
	file.Apps[item.Index].Favorite = !file.Apps[item.Index].Favorite
	if err := config.SaveDeployments(m.home, file); err != nil {
		m.status = m.t("Failed to update favorite: ", "收藏更新失败：") + err.Error()
		return m, nil
	}
	m.deploymentFile = file
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = m.deploymentVisibleIndex(item.Index)
	if file.Apps[item.Index].Favorite {
		m.status = m.t("Favorited: ", "已收藏：") + file.Apps[item.Index].Name
	} else {
		m.status = m.t("Unfavorited: ", "已取消收藏：") + file.Apps[item.Index].Name
	}
	if m.deploymentFavoriteOnly && !file.Apps[item.Index].Favorite {
		m.deploymentIndex = firstDeploymentItem(m.deploymentItems)
	}
	return m, nil
}

func (m Model) toggleDeploymentPinned() (tea.Model, tea.Cmd) {
	item, ok := m.selectedDeploymentItem()
	if !ok {
		m.status = m.t("No deployment app to pin", "没有可置顶的部署应用")
		return m, nil
	}
	file := m.deploymentFile
	if item.Index < 0 || item.Index >= len(file.Apps) {
		m.status = m.t("Deployment app does not exist", "部署应用不存在")
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
		m.status = m.t("Failed to update pin: ", "置顶更新失败：") + err.Error()
		return m, nil
	}
	m.deploymentFile = file
	m.deploymentItems = m.deploymentListItems()
	m.deploymentIndex = m.deploymentVisibleIndex(item.Index)
	if file.Apps[item.Index].Pinned {
		m.status = m.t("Pinned: ", "已置顶：") + file.Apps[item.Index].Name
	} else {
		m.status = m.t("Unpinned: ", "已取消置顶：") + file.Apps[item.Index].Name
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
		m.status = m.t("Confirm Deployment", "确认部署")
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
		m.status = m.t("No deployment app to delete.", "没有可删除的部署应用。")
		return m, nil
	}
	file.Apps = append(file.Apps[:index], file.Apps[index+1:]...)
	if err := config.SaveDeployments(m.home, file); err != nil {
		m.status = m.t("Failed to delete deployment app: ", "删除部署应用失败：") + err.Error()
		return m, nil
	}
	m.confirm = confirmAction{}
	m.deploymentSelected = removeDeploymentSelection(m.deploymentSelected, index)
	m = m.startDeploymentList(m.activeDeployment.HostIndex)
	m.status = m.t("Deployment app deleted.", "部署应用已删除。")
	return m, nil
}

func (m *Model) toggleDeploymentSelection(index int) {
	for i, selected := range m.deploymentSelected {
		if selected == index {
			m.deploymentSelected = append(m.deploymentSelected[:i], m.deploymentSelected[i+1:]...)
			m.status = m.t("Selection canceled", "已取消选择")
			return
		}
	}
	m.deploymentSelected = append(m.deploymentSelected, index)
	if m.isChineseUI() {
		m.status = fmt.Sprintf("已选择第 %d 个部署应用", len(m.deploymentSelected))
	} else {
		m.status = fmt.Sprintf("Selected deployment app %d", len(m.deploymentSelected))
	}
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
		m.status = m.t("No deployment category to switch", "没有可切换的部署分类")
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
		m.status = m.t("Deployment category: all", "部署分类：全部")
	} else {
		m.status = m.t("Deployment category: ", "部署分类：") + m.deploymentCategory
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
