package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
)

func (m Model) startDeploymentList(index int) Model {
	file, _, err := deploymentservice.LoadFile(m.home)
	if err != nil {
		m.status = m.t("Failed to read deployment config: ", "读取部署配置失败：") + err.Error()
		return m
	}
	m.deploymentState.File = file
	if m.deploymentState.Category == "" && index >= 0 && index < len(m.states) && m.category != "" {
		m.deploymentState.Category = m.category
	}
	m.deploymentState.Items = m.deploymentListItems()
	m.deploymentState.Index = firstDeploymentItem(m.deploymentState.Items)
	m.deploymentState.Active = activeDeployment{HostIndex: index}
	m.deploymentState.Selected = filterDeploymentSelection(m.deploymentState.Selected, file.Apps)
	m.deploymentState.OutputScroll = 0
	m.mode = modeDeploymentList
	m.status = m.t("Deployments", "应用部署")
	return m
}

func (m Model) deploymentListItems() []deploymentItem {
	items := []deploymentItem{}
	for i, app := range m.deploymentState.File.Apps {
		if m.deploymentState.Category != "" && deploymentAppCategory(app) != m.deploymentState.Category {
			continue
		}
		if m.deploymentState.FavoriteOnly && !app.Favorite {
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
			m.deploymentState.Detail = item.App
			m.deploymentState.OutputScroll = 0
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
			if item.Index >= 0 && item.Index < len(m.deploymentState.File.Apps) {
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
		m.deploymentState.ConfirmQueue = queue
		m.deploymentState.Confirm = queue[0]
		m.deploymentState.Active = activeDeployment{HostIndex: m.deploymentState.Active.HostIndex}
		m.deploymentState.OutputScroll = 0
		m.mode = modeDeploymentConfirm
		m.status = m.t("Confirm Deployment", "确认部署")
	}
	return m, nil
}

func (m Model) deploymentMoveStep(delta int, horizontal bool) int {
	if m.deploymentState.View != deploymentViewCards || horizontal {
		return delta
	}
	return delta * m.dashboardColumns()
}

func (m *Model) toggleDeploymentView() {
	if m.deploymentState.View == deploymentViewCards {
		m.deploymentState.View = deploymentViewList
		m.status = ""
		return
	}
	m.deploymentState.View = deploymentViewCards
	m.status = ""
}

func (m *Model) toggleDeploymentFavoriteFilter() {
	m.deploymentState.FavoriteOnly = !m.deploymentState.FavoriteOnly
	m.deploymentState.Items = m.deploymentListItems()
	m.deploymentState.Index = firstDeploymentItem(m.deploymentState.Items)
	if m.deploymentState.FavoriteOnly {
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
	file := m.deploymentState.File
	if item.Index < 0 || item.Index >= len(file.Apps) {
		m.status = m.t("Deployment app does not exist", "部署应用不存在")
		return m, nil
	}
	file.Apps[item.Index].Favorite = !file.Apps[item.Index].Favorite
	if err := deploymentservice.SaveFile(m.home, file); err != nil {
		m.status = m.t("Failed to update favorite: ", "收藏更新失败：") + err.Error()
		return m, nil
	}
	m.deploymentState.File = file
	m.deploymentState.Items = m.deploymentListItems()
	m.deploymentState.Index = m.deploymentVisibleIndex(item.Index)
	if file.Apps[item.Index].Favorite {
		m.status = m.t("Favorited: ", "已收藏：") + file.Apps[item.Index].Name
	} else {
		m.status = m.t("Unfavorited: ", "已取消收藏：") + file.Apps[item.Index].Name
	}
	if m.deploymentState.FavoriteOnly && !file.Apps[item.Index].Favorite {
		m.deploymentState.Index = firstDeploymentItem(m.deploymentState.Items)
	}
	return m, nil
}

func (m Model) toggleDeploymentPinned() (tea.Model, tea.Cmd) {
	item, ok := m.selectedDeploymentItem()
	if !ok {
		m.status = m.t("No deployment app to pin", "没有可置顶的部署应用")
		return m, nil
	}
	file := m.deploymentState.File
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
	if err := deploymentservice.SaveFile(m.home, file); err != nil {
		m.status = m.t("Failed to update pin: ", "置顶更新失败：") + err.Error()
		return m, nil
	}
	m.deploymentState.File = file
	m.deploymentState.Items = m.deploymentListItems()
	m.deploymentState.Index = m.deploymentVisibleIndex(item.Index)
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
	for i, item := range m.deploymentState.Items {
		if item.Index == appIndex {
			return i
		}
	}
	return firstDeploymentItem(m.deploymentState.Items)
}

func (m Model) updateDeploymentDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDeploymentList
	case "j", "down":
		m.deploymentState.OutputScroll = moveClampedInt(m.deploymentState.OutputScroll, 1, 0, m.deploymentDetailMaxScroll())
	case "k", "up":
		m.deploymentState.OutputScroll = moveClampedInt(m.deploymentState.OutputScroll, -1, 0, m.deploymentDetailMaxScroll())
	case "e":
		return m.startDeploymentEdit(m.deploymentState.Detail, true), nil
	case "enter":
		app := deploymentAppWithResourceDefaults(m.deploymentState.Detail)
		m.deploymentState.ConfirmQueue = []config.DeploymentApp{app}
		m.deploymentState.Confirm = app
		m.deploymentState.Active = activeDeployment{HostIndex: m.deploymentState.Active.HostIndex}
		m.deploymentState.OutputScroll = 0
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
	file := m.deploymentState.File
	if index < 0 || index >= len(file.Apps) {
		m.status = m.t("No deployment app to delete.", "没有可删除的部署应用。")
		return m, nil
	}
	file, deleted, err := deploymentservice.DeleteApp(m.home, file, index)
	if err != nil {
		m.status = m.t("Failed to delete deployment app: ", "删除部署应用失败：") + err.Error()
		return m, nil
	}
	if !deleted {
		m.status = m.t("No deployment app to delete.", "没有可删除的部署应用。")
		return m, nil
	}
	m.deploymentState.File = file
	m.confirm = confirmAction{}
	m.deploymentState.Selected = removeDeploymentSelection(m.deploymentState.Selected, index)
	m = m.startDeploymentList(m.deploymentState.Active.HostIndex)
	m.status = m.t("Deployment app deleted.", "部署应用已删除。")
	return m, nil
}

func (m *Model) toggleDeploymentSelection(index int) {
	for i, selected := range m.deploymentState.Selected {
		if selected == index {
			m.deploymentState.Selected = append(m.deploymentState.Selected[:i], m.deploymentState.Selected[i+1:]...)
			m.status = m.t("Selection canceled", "已取消选择")
			return
		}
	}
	m.deploymentState.Selected = append(m.deploymentState.Selected, index)
	if m.isChineseUI() {
		m.status = fmt.Sprintf("已选择第 %d 个部署应用", len(m.deploymentState.Selected))
	} else {
		m.status = fmt.Sprintf("Selected deployment app %d", len(m.deploymentState.Selected))
	}
}

func (m Model) deploymentSelectionOrder(index int) int {
	for i, selected := range m.deploymentState.Selected {
		if selected == index {
			return i + 1
		}
	}
	return 0
}

func (m Model) selectedDeploymentQueue() []config.DeploymentApp {
	queue := []config.DeploymentApp{}
	for _, index := range m.deploymentState.Selected {
		if index >= 0 && index < len(m.deploymentState.File.Apps) {
			app := m.deploymentState.File.Apps[index]
			if m.deploymentState.Category == "" || deploymentAppCategory(app) == m.deploymentState.Category {
				queue = append(queue, app)
			}
		}
	}
	return queue
}

func (m *Model) cycleDeploymentCategory(delta int) {
	categories := []string{""}
	seen := map[string]bool{}
	for _, app := range m.deploymentState.File.Apps {
		cat := deploymentAppCategory(app)
		if cat != "" && !seen[cat] {
			seen[cat] = true
			categories = append(categories, cat)
		}
	}
	sort.Strings(categories[1:])
	if len(categories) <= 1 {
		m.deploymentState.Category = ""
		m.deploymentState.Items = m.deploymentListItems()
		m.deploymentState.Index = firstDeploymentItem(m.deploymentState.Items)
		m.status = m.t("No deployment category to switch", "没有可切换的部署分类")
		return
	}
	current := 0
	for i, category := range categories {
		if category == m.deploymentState.Category {
			current = i
			break
		}
	}
	current = moveIndex(current, len(categories), delta)
	m.deploymentState.Category = categories[current]
	m.deploymentState.Items = m.deploymentListItems()
	m.deploymentState.Index = firstDeploymentItem(m.deploymentState.Items)
	if m.deploymentState.Category == "" {
		m.status = m.t("Deployment category: all", "部署分类：全部")
	} else {
		m.status = m.t("Deployment category: ", "部署分类：") + m.deploymentState.Category
	}
}

func (m *Model) moveDeploymentIndex(delta int) {
	if len(m.deploymentState.Items) == 0 {
		m.deploymentState.Index = 0
		return
	}
	for i := 0; i < len(m.deploymentState.Items); i++ {
		m.deploymentState.Index = moveIndex(m.deploymentState.Index, len(m.deploymentState.Items), delta)
		item := m.deploymentState.Items[m.deploymentState.Index]
		if !item.Header && !item.Spacer {
			return
		}
	}
}

func (m Model) selectedDeploymentItem() (deploymentItem, bool) {
	if m.deploymentState.Index < 0 || m.deploymentState.Index >= len(m.deploymentState.Items) {
		return deploymentItem{}, false
	}
	item := m.deploymentState.Items[m.deploymentState.Index]
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
