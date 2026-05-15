package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/actions"
	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) startResourceList(index int, kind resourceKind, back viewMode) (tea.Model, tea.Cmd) {
	if index < 0 || index >= len(m.states) {
		return m, nil
	}
	if file, _, err := config.LoadResources(m.home); err == nil {
		m.resourceFile = file
	} else {
		m.status = m.t("Failed to read resource config: ", "读取资源配置失败：") + err.Error()
	}
	m.mode = modeResourceList
	m.resourceHostIndex = index
	m.resourceBackMode = back
	m.resourceKind = kind
	m.resourceScope = resourceScopeDiscovered
	m.resourceIndex = 0
	m.resourceScroll = 0
	m.resourceQuery = ""
	m.resourceSearch = false
	m.applyCachedResourceDetails(index, kind)
	m.applyManagedResources(index)
	m.resourceLoading = true
	m.resourceLoadingKind = kind
	m.resourceLoadingPending = resourceLoadPartCount(kind)
	m.resourceManualRefresh = false
	m.resourceRefreshStatus = ""
	m.status = m.t("Loading resources...", "正在读取资源...")
	return m, m.fetchResourceDetails(index, kind)
}

func resourceLoadPartCount(kind resourceKind) int {
	switch kind {
	case resourceAll:
		return 3
	case resourceContainers, resourcePorts:
		return 1
	default:
		return 1
	}
}

func (m *Model) applyCachedResourceDetails(index int, kind resourceKind) {
	if index < 0 || index >= len(m.states) {
		return
	}
	if kind != resourceAll && kind != resourceContainers {
		return
	}
	if len(m.states[index].ContainerDetails) > 0 {
		return
	}
	file, _, err := config.LoadResourceCache(m.home)
	if err != nil {
		return
	}
	items, ok := config.ResourceContainerCacheForServer(file, m.resourceServerKey(index))
	if !ok {
		return
	}
	m.states[index].ContainerDetails = containerDetailsFromCache(items)
	m.states[index].ContainerError = ""
	associatePortContainers(m.states[index].PortDetails, m.states[index].ContainerDetails)
}

func (m Model) fetchResourceDetails(index int, kind resourceKind) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	cmds := []tea.Cmd{}
	if kind == resourceAll || kind == resourceServices {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourceServices))
	}
	if kind == resourceAll || kind == resourceContainers {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourceContainers))
	}
	if kind == resourceAll || kind == resourcePorts {
		cmds = append(cmds, m.fetchResourcePart(index, kind, resourcePorts))
	}
	return tea.Batch(cmds...)
}

func (m Model) fetchResourcePart(index int, requested resourceKind, part resourceKind) tea.Cmd {
	if index < 0 || index >= len(m.states) {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		msg := resourceLoadMsg{Index: index, Kind: part, Requested: requested}
		switch part {
		case resourceServices:
			serviceResult, serviceCleanup := actions.RemoteCommandContext(ctx, h, serviceListScript())
			serviceCleanup()
			msg.Services, msg.ServiceErr = parseServiceDetails(serviceResult.Output)
			if serviceResult.Err != nil && msg.ServiceErr == "" {
				msg.ServiceErr = serviceResult.Err.Error()
			}
		case resourceContainers:
			containerResult, containerCleanup := actions.RemoteCommandContext(ctx, h, containerDetailScript())
			containerCleanup()
			msg.Containers, msg.ContainerErr = parseContainerDetails(containerResult.Output)
			if containerResult.Err != nil && msg.ContainerErr == "" {
				msg.ContainerErr = containerResult.Err.Error()
			}
		case resourcePorts:
			portResult, portCleanup := actions.RemoteCommandContext(ctx, h, portDetailScript())
			portCleanup()
			msg.Ports, msg.PortsErrText = parsePortDetails(portResult.Output)
			if portResult.Err != nil && msg.PortsErrText == "" {
				msg.PortsErrText = portResult.Err.Error()
			}
		}
		return msg
	}
}

func (m Model) updateResourceList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.resourceSearch {
		return m.updateResourceSearch(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = m.resourceBackMode
		m.status = ""
		m.resourceDetailName = ""
	case "tab":
		m.resourceKind = (m.resourceKind + 1) % 5
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "g":
		m.cycleResourceListFilter()
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "v":
		if m.resourceScope == resourceScopeManaged {
			m.resourceScope = resourceScopeDiscovered
		} else {
			m.resourceScope = resourceScopeManaged
		}
		m.resourceIndex = 0
		m.resourceScroll = 0
	case "f":
		return m.toggleManagedResource()
	case "x":
		return m.startSelectedResourceRemoveConfirm()
	case "j", "down":
		m.moveResourceDown()
	case "k", "up":
		m.moveResourceUp()
	case "h", "left":
		m.moveResourceLeft()
	case "l", "right":
		m.moveResourceRight()
	case "o":
		return m.openResourceLog()
	case "e":
		return m.startResourceCommandEdit()
	case "a":
		return m.startResourceAdd()
	case "/":
		m.resourceSearch = true
		m.resourceQuery = ""
		m.resourceIndex = 0
	case "z":
		if m.resourceView == resourceViewCards {
			m.resourceView = resourceViewList
		} else {
			m.resourceView = resourceViewCards
		}
		m.resourceScroll = 0
	case "s":
		m.cycleResourceListFilter()
		m.resourceIndex = 0
	case "r":
		m.resourceLoading = true
		m.resourceLoadingKind = m.resourceKind
		m.resourceLoadingPending = resourceLoadPartCount(m.resourceKind)
		m.resourceManualRefresh = true
		m.status = m.t("Refreshing resources...", "正在刷新资源...")
		return m, m.fetchResourceDetails(m.resourceHostIndex, m.resourceKind)
	case " ", "enter":
		m.mode = modeResourceDetail
		m.resourceScroll = 0
		if ref, ok := m.selectedResourceRef(); ok {
			m.resourceDetailKind = ref.Kind
			if name, nameOK := m.selectedResourceName(); nameOK {
				m.resourceDetailName = name
			} else {
				m.resourceDetailName = ""
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceContainers {
			if item, ok := m.selectedContainer(); ok {
				m.resourceContainerExtraName = item.Name
				m.resourceContainerExtra = containerExtraDetail{}
				m.resourceContainerExtraErr = ""
				m.resourceContainerExtraLoading = true
				return m, m.fetchContainerExtraDetail(m.resourceHostIndex, item.Name)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceServices {
			if item, ok := m.selectedService(); ok {
				m.resourceServiceExtraName = item.Unit
				m.resourceServiceExtra = serviceDetail{}
				m.resourceServiceExtraErr = ""
				m.resourceServiceExtraLoading = true
				return m, m.fetchServiceExtraDetail(m.resourceHostIndex, item.Unit)
			}
		}
		if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceProcesses {
			if item, ok := m.selectedProcess(); ok {
				m.resourceProcessExtraPID = item.PID
				m.resourceProcessExtra = processExtraDetail{}
				m.resourceProcessExtraErr = ""
				m.resourceProcessExtraLoading = true
				return m, m.fetchProcessExtraDetail(m.resourceHostIndex, item.PID)
			}
		}
	}
	return m, nil
}

func (m *Model) cycleResourceListFilter() {
	if m.resourceKind == resourcePorts {
		m.resourcePortFilter = (m.resourcePortFilter + 1) % 6
		return
	}
	m.resourceFilter = (m.resourceFilter + 1) % 4
}

func (m Model) updateResourceSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.resourceSearch = false
		m.resourceQuery = ""
		m.resourceIndex = 0
	case "enter":
		m.resourceSearch = false
	case "backspace":
		r := []rune(m.resourceQuery)
		if len(r) > 0 {
			m.resourceQuery = string(r[:len(r)-1])
		}
		m.resourceIndex = 0
	default:
		if len(msg.Runes) > 0 {
			m.resourceQuery += string(msg.Runes)
			m.resourceIndex = 0
		}
	}
	return m, nil
}

func (m Model) updateResourceDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", " ":
		m.mode = modeResourceList
		m.resourceDetailName = ""
	case "j", "down":
		m.resourceScroll++
	case "k", "up":
		if m.resourceScroll > 0 {
			m.resourceScroll--
		}
	case "o":
		return m.openResourceLog()
	case "e":
		return m.startResourceCommandEdit()
	case "x":
		return m.startSelectedResourceRemoveConfirm()
	case "s":
		return m.startResourceAction(resourceActionStart)
	case "p":
		return m.startResourceAction(resourceActionStop)
	case "r":
		return m.startResourceAction(resourceActionRestart)
	}
	return m, nil
}

func (m Model) fetchContainerExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, containerExtraDetailScript(name))
		cleanup()
		detail, errText := parseContainerExtraDetail(result.Output)
		if result.Err != nil && errText == "" {
			errText = result.Err.Error()
		}
		return resourceContainerDetailMsg{Index: index, Name: name, Detail: detail, Err: errText}
	}
}

func (m Model) fetchServiceExtraDetail(index int, name string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, serviceExtraDetailScript(name))
		cleanup()
		detail, errText := parseServiceExtraDetail(result.Output)
		if strings.TrimSpace(detail.Unit) != "" {
			errText = ""
		} else if result.Err != nil && errText == "" {
			errText = result.Err.Error()
		}
		if !meaningfulResourceDetailError(errText) {
			errText = ""
		}
		return resourceServiceDetailMsg{Index: index, Name: name, Detail: detail, Err: errText}
	}
}

func (m Model) fetchProcessExtraDetail(index int, pid string) tea.Cmd {
	if index < 0 || index >= len(m.states) || strings.TrimSpace(pid) == "" {
		return nil
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, processExtraDetailScript(pid))
		cleanup()
		detail, errText := parseProcessExtraDetail(result.Output)
		if result.Err != nil && errText == "" {
			errText = result.Err.Error()
		}
		return resourceProcessDetailMsg{Index: index, PID: pid, Detail: detail, Err: errText}
	}
}

func meaningfulResourceDetailError(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.Contains(value, "=") && !strings.ContainsAny(value, " \n\t:/") {
		return false
	}
	needles := []string{
		"error", "failed", "permission", "denied", "timeout", "deadline", "killed",
		"exit status", "not found", "no such", "unavailable", "cannot", "refused",
		"错误", "失败", "权限", "拒绝", "超时", "不可用", "不存在",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) || strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func (m Model) startResourceAdd() (tea.Model, tea.Cmd) {
	kind := m.resourceKind
	if kind == resourceAll || kind == resourceContainers {
		kind = resourceServices
	}
	m.resourceAddKind = kind
	m.resourceAddName = ""
	m.resourceAddField = 0
	m.resourceAddCursor = 0
	m.resourceManagePane = 0
	m.resourceManageDiscoveredIndex = 0
	m.resourceManageFavoriteIndex = 0
	m.resourceManageSearch = false
	m.resourceManageQuery = ""
	m.resourceCommandForm = resourceCommandForm{Server: m.resourceServerKey(m.resourceHostIndex), Kind: kind}
	m.mode = modeResourceAdd
	m.status = m.t("Resource manager", "资源管理")
	return m, nil
}

func (m Model) updateResourceAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.resourceManageSearch {
		return m.updateResourceManageSearch(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "ctrl+i", "shift+tab":
		m.resourceManagePane = 1 - m.resourceManagePane
	case "down", "j":
		m.moveResourceManageSelection(1)
	case "up", "k":
		m.moveResourceManageSelection(-1)
	case "left", "h":
		m.cycleResourceAddKind(-1)
		m.resetResourceManageSelection()
	case "right", "l":
		m.cycleResourceAddKind(1)
		m.resetResourceManageSelection()
	case "g":
		m.cycleResourceAddKind(1)
		m.resetResourceManageSelection()
	case "r":
		refreshKind := m.resourceAddKind
		if refreshKind == resourceProcesses {
			refreshKind = resourcePorts
		}
		m.resourceLoading = true
		m.resourceLoadingKind = refreshKind
		m.resourceLoadingPending = resourceLoadPartCount(refreshKind)
		m.resourceManualRefresh = true
		m.status = m.t("Refreshing resources...", "正在刷新资源...")
		return m, m.fetchResourceDetails(m.resourceHostIndex, refreshKind)
	case "/":
		m.resourceManageSearch = true
		m.resourceManageQuery = ""
		m.resetResourceManageSelection()
	case "enter", "f":
		return m.toggleResourceManagerFavorite()
	case "x":
		return m.startResourceManageRemoveConfirm()
	case "e":
		if m.resourceManagePane == 1 {
			return m.startResourceManageEdit()
		}
	}
	return m, nil
}

func (m Model) updateResourceManageSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc":
		m.resourceManageSearch = false
		m.resourceManageQuery = ""
		m.resetResourceManageSelection()
	case "enter":
		m.resourceManageSearch = false
	case "backspace":
		r := []rune(m.resourceManageQuery)
		if len(r) > 0 {
			m.resourceManageQuery = string(r[:len(r)-1])
		}
		m.resetResourceManageSelection()
	default:
		if len(msg.Runes) > 0 {
			m.resourceManageQuery += string(msg.Runes)
			m.resetResourceManageSelection()
		}
	}
	return m, nil
}

func (m *Model) resetResourceManageSelection() {
	m.resourceManageDiscoveredIndex = 0
	m.resourceManageFavoriteIndex = 0
}

func (m *Model) moveResourceManageSelection(delta int) {
	if m.resourceManagePane == 1 {
		count := len(m.resourceManageFavorites())
		if count == 0 {
			m.resourceManageFavoriteIndex = 0
			return
		}
		m.resourceManageFavoriteIndex = moveIndex(m.resourceManageFavoriteIndex, count, delta)
		return
	}
	count := len(m.resourceManageDiscoveredRefs())
	if count == 0 {
		m.resourceManageDiscoveredIndex = 0
		return
	}
	m.resourceManageDiscoveredIndex = moveIndex(m.resourceManageDiscoveredIndex, count, delta)
}

func (m Model) toggleResourceManagerFavorite() (tea.Model, tea.Cmd) {
	if m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(m.resourceAddKind)
	if kind == "" {
		return m, nil
	}
	if m.resourceManagePane == 1 {
		return m.startResourceManageRemoveConfirm()
	}
	refs := m.resourceManageDiscoveredRefs()
	if len(refs) == 0 {
		return m, nil
	}
	ref := refs[clampInt(m.resourceManageDiscoveredIndex, 0, len(refs)-1)]
	name, ok := m.resourceNameForRef(ref)
	if !ok || strings.TrimSpace(name) == "" {
		return m, nil
	}
	if findManagedResource(m.resourceFile.Items, server, kind, name) >= 0 {
		m.status = m.t("Resource already added: ", "资源已添加：") + name
		return m, clearStatusAfter(2 * time.Second)
	}
	item := defaultManagedResource(server, kind, name)
	m.resourceFile.Items = append(m.resourceFile.Items, item)
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	refs = m.resourceManageDiscoveredRefs()
	if m.resourceManageDiscoveredIndex >= len(refs) && m.resourceManageDiscoveredIndex > 0 {
		m.resourceManageDiscoveredIndex--
	}
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceManageRemoveConfirm() (tea.Model, tea.Cmd) {
	if m.resourceManagePane != 1 {
		return m, nil
	}
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		m.status = m.t("No added resource to remove.", "没有可移出的已添加资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := items[clampInt(m.resourceManageFavoriteIndex, 0, len(items)-1)]
	if item.Kind == config.ResourceKindContainer {
		m.status = m.dockerRemoveUnavailableText()
		return m, clearStatusAfter(2 * time.Second)
	}
	m.confirm = confirmAction{
		Kind:     confirmRemoveResource,
		Title:    m.t("Remove Resource", "确认移出资源"),
		Lines:    m.resourceRemoveConfirmLines(item),
		Back:     modeResourceAdd,
		Resource: item,
	}
	m.mode = modeConfirmAction
	m.status = m.t("Confirm remove resource", "确认移出资源")
	return m, nil
}

func (m Model) startSelectedResourceRemoveConfirm() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourceContainers {
		m.status = m.dockerRemoveUnavailableText()
		return m, clearStatusAfter(2 * time.Second)
	}
	item, ok := m.managedResource(ref.Kind, name)
	if !ok {
		m.status = m.t("This resource is not added.", "该资源未添加。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.confirm = confirmAction{
		Kind:     confirmRemoveResource,
		Title:    m.t("Remove Resource", "确认移出资源"),
		Lines:    m.resourceRemoveConfirmLines(item),
		Back:     modeResourceList,
		Resource: item,
	}
	m.mode = modeConfirmAction
	m.status = m.t("Confirm remove resource", "确认移出资源")
	return m, nil
}

func (m Model) dockerRemoveUnavailableText() string {
	return m.t("Containers cannot be deleted.", "容器无法删除。")
}

func (m Model) resourceRemoveConfirmLines(item config.ManagedResource) []string {
	return []string{
		m.t("This added resource will be removed from sshm.", "将从 sshm 已添加资源中移出该资源。"),
		m.t("Resource: ", "资源：") + item.Name,
		m.t("Type: ", "类型：") + m.resourceKindName(resourceKindFromConfig(item.Kind)),
		m.t("Server: ", "服务器：") + item.Server,
		m.t("This does not delete or stop the system resource. Docker containers can be discovered again next time.", "这不会删除或停止系统资源。Docker 容器下次仍会被自动发现。"),
	}
}

func (m Model) removeManagedResource(item config.ManagedResource) (tea.Model, tea.Cmd) {
	real := findManagedResource(m.resourceFile.Items, item.Server, item.Kind, item.Name)
	if real >= 0 {
		m.resourceFile.Items = append(m.resourceFile.Items[:real], m.resourceFile.Items[real+1:]...)
	}
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.confirm = confirmAction{}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	m.status = m.t("Removed from resources: ", "已移出资源：") + item.Name
	if m.resourceManageFavoriteIndex >= len(m.resourceManageFavorites()) && m.resourceManageFavoriteIndex > 0 {
		m.resourceManageFavoriteIndex--
	}
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceManageEdit() (tea.Model, tea.Cmd) {
	items := m.resourceManageFavorites()
	if len(items) == 0 {
		return m, nil
	}
	item := items[clampInt(m.resourceManageFavoriteIndex, 0, len(items)-1)]
	m.resourceCommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceAddKind,
		Name:           item.Name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
	}
	m.resourceCommandField = 0
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceCommandBackMode = modeResourceAdd
	m.mode = modeResourceCommandEdit
	m.status = m.t("Edit resource commands", "编辑资源命令")
	return m, nil
}

func (m *Model) moveResourceAddField(delta int) {
	m.resourceAddField = moveIndex(m.resourceAddField, resourceAddFieldCount(m.resourceAddKind), delta)
	m.resourceAddCursor = len([]rune(m.resourceAddFieldValue(m.resourceAddField)))
}

func resourceAddFieldCount(kind resourceKind) int {
	return 2 + resourceCommandFieldCount(kind)
}

func (m *Model) cycleResourceAddKind(delta int) {
	kinds := []resourceKind{resourceServices, resourceProcesses, resourcePorts}
	idx := 0
	for i, kind := range kinds {
		if kind == m.resourceAddKind {
			idx = i
			break
		}
	}
	idx = moveIndex(idx, len(kinds), delta)
	m.resourceAddKind = kinds[idx]
	m.resourceCommandForm.Kind = m.resourceAddKind
	m.applyResourceAddDefaults()
	m.resourceAddCursor = len([]rune(m.resourceAddFieldValue(m.resourceAddField)))
}

func (m *Model) applyResourceAddDefaults() {
	name := strings.TrimSpace(m.resourceAddName)
	if m.resourceAddKind == resourcePorts && name != "" && !strings.Contains(name, "/") {
		name = "tcp/" + name
	}
	item := defaultManagedResource(m.resourceServerKey(m.resourceHostIndex), configResourceKind(m.resourceAddKind), name)
	m.resourceCommandForm = resourceCommandForm{
		Server:         item.Server,
		Kind:           m.resourceAddKind,
		Name:           item.Name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
	}
}

func (m Model) resourceAddFieldValue(field int) string {
	switch field {
	case 1:
		return m.resourceAddName
	default:
		return m.resourceCommandFieldValue(field - 2)
	}
}

func (m *Model) setResourceAddFieldValue(field int, value string) {
	if field == 1 {
		m.resourceAddName = value
		m.applyResourceAddDefaults()
		return
	}
	m.setResourceCommandFieldValue(field-2, value)
}

func (m *Model) moveResourceAddCursor(delta int) {
	value := []rune(m.resourceAddFieldValue(m.resourceAddField))
	m.resourceAddCursor = clampInt(m.resourceAddCursor+delta, 0, len(value))
}

func (m *Model) resourceAddAppend(value string) {
	runes := []rune(m.resourceAddFieldValue(m.resourceAddField))
	cursor := clampInt(m.resourceAddCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceAddField, string(next))
	m.resourceAddCursor = cursor + len([]rune(value))
}

func (m *Model) resourceAddBackspace() {
	runes := []rune(m.resourceAddFieldValue(m.resourceAddField))
	cursor := clampInt(m.resourceAddCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceAddFieldValue(m.resourceAddField, string(next))
	m.resourceAddCursor = cursor - 1
}

func (m Model) saveResourceAdd() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.resourceAddName)
	if name == "" {
		m.status = m.t("Resource name cannot be empty.", "资源名称不能为空。")
		return m, nil
	}
	if m.resourceAddKind == resourcePorts && !strings.Contains(name, "/") {
		name = "tcp/" + name
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(m.resourceAddKind)
	if findManagedResource(m.resourceFile.Items, server, kind, name) >= 0 {
		m.status = m.t("Resource already added: ", "资源已添加：") + name
		return m, nil
	}
	item := defaultManagedResource(server, kind, name)
	item.StartCommand = strings.TrimSpace(m.resourceCommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceCommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceCommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceCommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceCommandForm.HealthCommand)
	m.resourceFile.Items = append(m.resourceFile.Items, item)
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	m.resourceScope = resourceScopeDiscovered
	m.resourceKind = m.resourceAddKind
	m.resourceIndex = 0
	m.resourceScroll = 0
	m.mode = modeResourceList
	m.status = m.t("Added to resources: ", "已添加资源：") + name
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) startResourceCommandEdit() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("Ports are read-only. Add the port before editing health checks.", "端口为只读。请先添加端口，再编辑健康检查。")
	}
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(ref.Kind)
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 {
		m.status = m.t("Add this resource before editing commands.", "请先添加该资源，再编辑命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	item := m.resourceFile.Items[idx]
	m.resourceCommandForm = resourceCommandForm{
		Server:         server,
		Kind:           ref.Kind,
		Name:           name,
		StartCommand:   item.StartCommand,
		StopCommand:    item.StopCommand,
		RestartCommand: item.RestartCommand,
		LogCommand:     item.LogCommand,
		HealthCommand:  item.HealthCommand,
	}
	m.resourceCommandField = 0
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(0)))
	m.resourceCommandBackMode = modeResourceList
	m.mode = modeResourceCommandEdit
	m.status = m.t("Edit resource commands", "编辑资源命令")
	return m, nil
}

func (m Model) updateResourceCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = m.resourceCommandReturnMode()
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "down", "j":
		m.moveResourceCommandField(1)
	case "shift+tab", "up", "k":
		m.moveResourceCommandField(-1)
	case "left", "h":
		m.moveResourceCommandCursor(-1)
	case "right", "l":
		m.moveResourceCommandCursor(1)
	case "backspace":
		if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
			return m, nil
		}
		m.resourceCommandBackspace()
	case "enter":
		if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
			m.mode = m.resourceCommandReturnMode()
			m.status = ""
			return m, nil
		}
		if err := m.saveResourceCommandForm(); err != nil {
			m.status = m.t("Save failed: ", "保存失败：") + err.Error()
			return m, nil
		}
		m.mode = m.resourceCommandReturnMode()
		m.status = m.t("Resource commands saved.", "资源命令已保存。")
		return m, clearStatusAfter(2 * time.Second)
	default:
		if len(msg.Runes) > 0 {
			if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
				return m, nil
			}
			m.resourceCommandAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func (m Model) resourceCommandReturnMode() viewMode {
	if m.resourceCommandBackMode == modeResourceAdd {
		return modeResourceAdd
	}
	return modeResourceList
}

func (m *Model) moveResourceCommandField(delta int) {
	if resourceCommandFieldCount(m.resourceCommandForm.Kind) == 0 {
		m.resourceCommandField = 0
		m.resourceCommandCursor = 0
		return
	}
	m.resourceCommandField = moveIndex(m.resourceCommandField, resourceCommandFieldCount(m.resourceCommandForm.Kind), delta)
	m.resourceCommandCursor = len([]rune(m.resourceCommandFieldValue(m.resourceCommandField)))
}

func resourceCommandFieldCount(kind resourceKind) int {
	if kind == resourcePorts {
		return 1
	}
	return 4
}

func (m *Model) moveResourceCommandCursor(delta int) {
	value := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	m.resourceCommandCursor = clampInt(m.resourceCommandCursor+delta, 0, len(value))
}

func (m Model) resourceCommandFieldValue(field int) string {
	switch field {
	case 0:
		if m.resourceCommandForm.Kind == resourcePorts {
			return m.resourceCommandForm.HealthCommand
		}
		return m.resourceCommandForm.StartCommand
	case 1:
		return m.resourceCommandForm.StopCommand
	case 2:
		return m.resourceCommandForm.RestartCommand
	case 3:
		return m.resourceCommandForm.LogCommand
	default:
		return ""
	}
}

func (m *Model) setResourceCommandFieldValue(field int, value string) {
	switch field {
	case 0:
		if m.resourceCommandForm.Kind == resourcePorts {
			m.resourceCommandForm.HealthCommand = value
		} else {
			m.resourceCommandForm.StartCommand = value
		}
	case 1:
		m.resourceCommandForm.StopCommand = value
	case 2:
		m.resourceCommandForm.RestartCommand = value
	case 3:
		m.resourceCommandForm.LogCommand = value
	}
}

func (m *Model) resourceCommandAppend(value string) {
	runes := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	cursor := clampInt(m.resourceCommandCursor, 0, len(runes))
	next := append([]rune{}, runes[:cursor]...)
	next = append(next, []rune(value)...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceCommandField, string(next))
	m.resourceCommandCursor = cursor + len([]rune(value))
}

func (m *Model) resourceCommandBackspace() {
	runes := []rune(m.resourceCommandFieldValue(m.resourceCommandField))
	cursor := clampInt(m.resourceCommandCursor, 0, len(runes))
	if cursor == 0 {
		return
	}
	next := append([]rune{}, runes[:cursor-1]...)
	next = append(next, runes[cursor:]...)
	m.setResourceCommandFieldValue(m.resourceCommandField, string(next))
	m.resourceCommandCursor = cursor - 1
}

func (m *Model) saveResourceCommandForm() error {
	server := m.resourceCommandForm.Server
	kind := configResourceKind(m.resourceCommandForm.Kind)
	name := m.resourceCommandForm.Name
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 {
		return fmt.Errorf("resource config not found")
	}
	item := m.resourceFile.Items[idx]
	item.StartCommand = strings.TrimSpace(m.resourceCommandForm.StartCommand)
	item.StopCommand = strings.TrimSpace(m.resourceCommandForm.StopCommand)
	item.RestartCommand = strings.TrimSpace(m.resourceCommandForm.RestartCommand)
	item.DeleteCommand = ""
	item.LogCommand = strings.TrimSpace(m.resourceCommandForm.LogCommand)
	item.HealthCommand = strings.TrimSpace(m.resourceCommandForm.HealthCommand)
	m.resourceFile.Items[idx] = item
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		return err
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	return nil
}

func (m Model) updateResourceLog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
	case "j", "down":
		m.resourceLogScroll++
	case "k", "up":
		if m.resourceLogScroll > 0 {
			m.resourceLogScroll--
		}
	case "r":
		return m.openResourceLog()
	}
	m.resourceLogScroll = clampInt(m.resourceLogScroll, 0, m.resourceLogMaxScroll())
	return m, nil
}

func (m Model) updateResourceConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "enter":
		m.mode = modeResourceOutput
		m.resourceActionRunning = true
		m.resourceActionOutput = ""
		m.resourceActionExitCode = 0
		return m, m.runResourceAction()
	}
	return m, nil
}

func (m Model) updateResourceOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q":
		m.mode = modeResourceList
		m.resourceAction = resourceActionNone
	case "j", "down":
		m.resourceScroll++
	case "k", "up":
		if m.resourceScroll > 0 {
			m.resourceScroll--
		}
	case "r":
		if !m.resourceActionRunning && m.resourceAction != resourceActionNone {
			m.mode = modeResourceOutput
			m.resourceActionRunning = true
			m.resourceActionOutput = ""
			m.resourceActionExitCode = 0
			return m, m.runResourceAction()
		}
	}
	return m, nil
}

func (m *Model) moveResourceSelection(delta int) {
	total := len(m.filteredResourceIndexes())
	if total == 0 {
		m.resourceIndex = 0
		return
	}
	m.resourceIndex = clampInt(m.resourceIndex+delta, 0, total-1)
}

func (m *Model) moveResourceDown() {
	if m.resourceView == resourceViewCards {
		m.moveResourceSelection(m.dashboardColumns())
		return
	}
	m.moveResourceSelection(1)
}

func (m *Model) moveResourceUp() {
	if m.resourceView == resourceViewCards {
		m.moveResourceSelection(-m.dashboardColumns())
		return
	}
	m.moveResourceSelection(-1)
}

func (m *Model) moveResourceLeft() {
	m.moveResourceSelection(-1)
}

func (m *Model) moveResourceRight() {
	m.moveResourceSelection(1)
}

func (m Model) startResourceAction(action resourceActionKind) (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if action == resourceActionDelete {
		m.status = m.t("Delete is disabled for resources.", "资源页已禁用删除操作。")
		return m, nil
	}
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource is read-only.", "该资源为只读信息。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.managedResourceCommand(ref.Kind, action, name)) == "" {
		m.status = m.t("Add this process and configure commands first.", "请先添加该进程并配置命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.resourceAction = action
	m.resourceActionResource = ref.Kind
	m.resourceActionName = name
	m.resourceScroll = 0
	m.mode = modeResourceConfirm
	return m, nil
}

func (m Model) runResourceAction() tea.Cmd {
	index := m.resourceHostIndex
	kind := m.resourceActionResource
	action := m.resourceAction
	name := m.resourceActionName
	if index < 0 || index >= len(m.states) || strings.TrimSpace(name) == "" {
		return func() tea.Msg {
			return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: actions.CommandResult{Err: fmt.Errorf("invalid resource"), ExitCode: -1}}
		}
	}
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		script := m.resourceActionScript(kind, action, name)
		result, cleanup := actions.RemoteCommandContext(ctx, h, script)
		cleanup()
		return resourceActionMsg{Index: index, Kind: kind, Action: action, Name: name, Result: result}
	}
}

func (m Model) openResourceLog() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	if ref.Kind == resourcePorts {
		m.status = m.t("This resource does not have managed logs.", "该资源没有托管日志。")
		return m, nil
	}
	if ref.Kind == resourceProcesses && strings.TrimSpace(m.managedResourceLogCommand(ref.Kind, name)) == "" {
		m.status = m.t("Add this process and configure log command first.", "请先添加该进程并配置日志命令。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.mode = modeResourceLog
	m.resourceLogName = name
	m.resourceLogKind = ref.Kind
	m.resourceLogOutput = m.t("Loading logs...", "正在读取日志...")
	m.resourceLogScroll = 0
	index := m.resourceHostIndex
	kind := m.resourceLogKind
	h := m.states[index].Host
	timeout := m.appConfig.CommandDuration()
	if timeout < 20*time.Second {
		timeout = 20 * time.Second
	}
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, cleanup := actions.RemoteCommandContext(ctx, h, m.resourceLogScript(kind, name, 200))
		cleanup()
		return resourceLogMsg{Index: index, Kind: kind, Name: name, Output: result.Output, Result: result}
	}
}

func (m Model) handleResourceLoad(msg resourceLoadMsg) (tea.Model, tea.Cmd) {
	if msg.Index < 0 || msg.Index >= len(m.states) {
		return m, nil
	}
	now := time.Now()
	if msg.Kind == resourceServices {
		m.states[msg.Index].ServiceDetails = msg.Services
		m.states[msg.Index].ServiceError = msg.ServiceErr
		m.resourceServiceAt = now
	}
	if msg.Kind == resourceContainers {
		m.states[msg.Index].ContainerDetails = msg.Containers
		m.states[msg.Index].ContainerError = msg.ContainerErr
		m.resourceContainerAt = now
		if msg.ContainerErr == "" {
			_ = config.UpsertResourceContainerCache(m.home, m.resourceServerKey(msg.Index), containerDetailsToCache(msg.Containers), now)
		}
	}
	if msg.Kind == resourcePorts {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	if msg.Kind == resourceServices && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	if msg.Kind == resourceContainers && len(msg.Ports) > 0 {
		m.states[msg.Index].PortDetails = msg.Ports
		m.states[msg.Index].PortDetailsError = msg.PortsErrText
		m.resourcePortAt = now
	}
	associatePortContainers(m.states[msg.Index].PortDetails, m.states[msg.Index].ContainerDetails)
	m.applyManagedResources(msg.Index)
	m.resourceCollectedAt = now
	if m.resourceLoadingPending > 0 {
		m.resourceLoadingPending--
	}
	if m.resourceLoadingPending <= 0 {
		m.resourceLoading = false
		if m.resourceManualRefresh {
			m.resourceRefreshStatus = fmt.Sprintf("%s%s", m.t("Manual refresh done: ", "手动刷新完成："), now.Format("15:04:05"))
		} else {
			m.resourceRefreshStatus = fmt.Sprintf("%s%s", m.t("Last refresh: ", "最后刷新："), now.Format("15:04:05"))
		}
		m.resourceManualRefresh = false
		m.status = m.resourceRefreshStatus
		return m, nil
	}
	m.status = m.t("Loading resources...", "正在读取资源...")
	return m, nil
}

func (m Model) handleResourceContainerDetail(msg resourceContainerDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Name != m.resourceContainerExtraName {
		return m, nil
	}
	m.resourceContainerExtraLoading = false
	m.resourceContainerExtra = msg.Detail
	m.resourceContainerExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceServiceDetail(msg resourceServiceDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Name != m.resourceServiceExtraName {
		return m, nil
	}
	m.resourceServiceExtraLoading = false
	m.resourceServiceExtra = msg.Detail
	m.resourceServiceExtraErr = msg.Err
	if strings.TrimSpace(msg.Err) == "" && strings.TrimSpace(msg.Detail.Unit) != "" {
		for i := range m.states[msg.Index].ServiceDetails {
			if m.states[msg.Index].ServiceDetails[i].Unit != msg.Detail.Unit {
				continue
			}
			managed := m.states[msg.Index].ServiceDetails[i].Managed
			favorite := m.states[msg.Index].ServiceDetails[i].Favorite
			missing := m.states[msg.Index].ServiceDetails[i].Missing
			m.states[msg.Index].ServiceDetails[i] = msg.Detail
			m.states[msg.Index].ServiceDetails[i].Managed = managed
			m.states[msg.Index].ServiceDetails[i].Favorite = favorite
			m.states[msg.Index].ServiceDetails[i].Missing = missing
			break
		}
	}
	return m, nil
}

func (m Model) handleResourceProcessDetail(msg resourceProcessDetailMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.PID != m.resourceProcessExtraPID {
		return m, nil
	}
	m.resourceProcessExtraLoading = false
	m.resourceProcessExtra = msg.Detail
	m.resourceProcessExtraErr = msg.Err
	return m, nil
}

func (m Model) handleResourceLog(msg resourceLogMsg) (tea.Model, tea.Cmd) {
	if msg.Index != m.resourceHostIndex || msg.Kind != m.resourceLogKind || msg.Name != m.resourceLogName {
		return m, nil
	}
	m.resourceLogOutput = strings.TrimRight(msg.Output, "\n")
	if m.resourceLogOutput == "" {
		m.resourceLogOutput = m.t("No log output.", "没有日志输出。")
	}
	if msg.Result.Err != nil {
		m.status = fmt.Sprintf("%s %d", m.t("Log command failed, exit", "日志命令失败，退出码"), msg.Result.ExitCode)
	}
	return m, nil
}

func (m Model) handleResourceAction(msg resourceActionMsg) (tea.Model, tea.Cmd) {
	m.resourceActionRunning = false
	m.resourceActionOutput = strings.TrimRight(msg.Result.Output, "\n")
	m.resourceActionExitCode = msg.Result.ExitCode
	if msg.Result.Err != nil {
		m.status = m.resourceActionErrorText(msg.Result)
		return m, nil
	}
	m.status = m.resourceActionNameText(msg.Action) + m.t(" complete.", "完成。")
	refreshKind := msg.Kind
	if refreshKind == resourcePorts {
		refreshKind = resourceAll
	}
	m.resourceLoading = true
	m.resourceLoadingKind = refreshKind
	m.resourceLoadingPending = resourceLoadPartCount(refreshKind)
	m.resourceManualRefresh = false
	return m, m.fetchResourceDetails(msg.Index, refreshKind)
}

func (m Model) toggleManagedResource() (tea.Model, tea.Cmd) {
	name, ok := m.selectedResourceName()
	if !ok || m.resourceHostIndex < 0 || m.resourceHostIndex >= len(m.states) {
		return m, nil
	}
	ref, _ := m.selectedResourceRef()
	server := m.resourceServerKey(m.resourceHostIndex)
	kind := configResourceKind(ref.Kind)
	if kind == "" {
		return m, nil
	}
	idx := findManagedResource(m.resourceFile.Items, server, kind, name)
	if idx < 0 {
		if ref.Kind == resourceContainers {
			item := config.ManagedResource{Server: server, Kind: kind, Name: name, Favorite: true}
			m.resourceFile.Items = append(m.resourceFile.Items, item)
			if err := config.SaveResources(m.home, m.resourceFile); err != nil {
				m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
				return m, nil
			}
			m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
			m.applyManagedResources(m.resourceHostIndex)
			m.status = m.t("Added to favorites: ", "已收藏：") + name
			return m, clearStatusAfter(2 * time.Second)
		}
		m.status = m.t("Add this resource first with a.", "请先按 a 添加该资源。")
		return m, clearStatusAfter(2 * time.Second)
	}
	m.resourceFile.Items[idx].Favorite = !m.resourceFile.Items[idx].Favorite
	if ref.Kind == resourceContainers && !m.resourceFile.Items[idx].Favorite && !containerResourceHasCustomConfig(m.resourceFile.Items[idx]) {
		m.resourceFile.Items = append(m.resourceFile.Items[:idx], m.resourceFile.Items[idx+1:]...)
		idx = -1
	}
	if err := config.SaveResources(m.home, m.resourceFile); err != nil {
		m.status = m.t("Failed to save resource config: ", "保存资源配置失败：") + err.Error()
		return m, nil
	}
	m.resourceFile.Items = config.NormalizeManagedResources(m.resourceFile.Items)
	m.applyManagedResources(m.resourceHostIndex)
	if idx >= 0 && m.resourceFile.Items[idx].Favorite {
		m.status = m.t("Added to favorites: ", "已收藏：") + name
	} else {
		m.status = m.t("Removed from favorites: ", "已取消收藏：") + name
	}
	return m, clearStatusAfter(2 * time.Second)
}

func (m Model) hasManagedResources(index int) bool {
	server := m.resourceServerKey(index)
	for _, item := range m.resourceFile.Items {
		if item.Server == server {
			return true
		}
	}
	return false
}

func (m Model) resourceServerKey(index int) string {
	if index < 0 || index >= len(m.states) {
		return ""
	}
	h := m.states[index].Host
	return config.ServerCommandKey(h.Category, h.Name)
}

func (m *Model) applyManagedResources(index int) {
	if index < 0 || index >= len(m.states) {
		return
	}
	server := m.resourceServerKey(index)
	managed := m.managedResourcesForServer(server)
	state := &m.states[index]
	for i := range state.ServiceDetails {
		state.ServiceDetails[i].Managed = false
		state.ServiceDetails[i].Favorite = false
		state.ServiceDetails[i].Missing = false
	}
	for i := range state.ContainerDetails {
		state.ContainerDetails[i].Managed = false
		state.ContainerDetails[i].Favorite = false
		state.ContainerDetails[i].Missing = false
	}
	for i := range state.PortDetails {
		state.PortDetails[i].Managed = false
		state.PortDetails[i].Favorite = false
		state.PortDetails[i].ProcessManaged = false
		state.PortDetails[i].ProcessFavorite = false
		state.PortDetails[i].Missing = false
	}
	for _, item := range managed {
		switch item.Kind {
		case config.ResourceKindService:
			found := false
			for i := range state.ServiceDetails {
				if state.ServiceDetails[i].Unit == item.Name {
					state.ServiceDetails[i].Managed = true
					state.ServiceDetails[i].Favorite = item.Favorite
					if strings.EqualFold(strings.TrimSpace(state.ServiceDetails[i].Load), "not-found") {
						state.ServiceDetails[i].Missing = true
						state.ServiceDetails[i].Description = "Managed resource not found"
					}
					found = true
					break
				}
			}
			if !found {
				state.ServiceDetails = append(state.ServiceDetails, serviceDetail{Unit: item.Name, Load: "-", Active: "missing", Sub: "missing", Description: "Managed resource not found", Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindContainer:
			found := false
			for i := range state.ContainerDetails {
				if state.ContainerDetails[i].Name == item.Name {
					if containerResourceHasCustomConfig(item) {
						state.ContainerDetails[i].Managed = true
					}
					state.ContainerDetails[i].Favorite = item.Favorite
					found = true
					break
				}
			}
			if !found && containerResourceHasCustomConfig(item) {
				state.ContainerDetails = append(state.ContainerDetails, containerDetail{Name: item.Name, Status: "missing", Managed: true, Favorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindProcess:
			found := false
			for i := range state.PortDetails {
				if state.PortDetails[i].Process == item.Name {
					state.PortDetails[i].ProcessManaged = true
					state.PortDetails[i].ProcessFavorite = item.Favorite
					found = true
				}
			}
			if !found {
				state.PortDetails = append(state.PortDetails, portDetail{Process: item.Name, Count: 0, ProcessManaged: true, ProcessFavorite: item.Favorite, Missing: true})
			}
		case config.ResourceKindPort:
			proto, port := splitManagedPortName(item.Name)
			found := false
			for i := range state.PortDetails {
				if state.PortDetails[i].Protocol == proto && state.PortDetails[i].Port == port {
					state.PortDetails[i].Managed = true
					state.PortDetails[i].Favorite = item.Favorite
					found = true
				}
			}
			if !found {
				state.PortDetails = append(state.PortDetails, portDetail{Protocol: proto, Port: port, Count: 0, Managed: true, Favorite: item.Favorite, Missing: true})
			}
		}
	}
}

func (m Model) managedResourcesForServer(server string) []config.ManagedResource {
	items := []config.ManagedResource{}
	for _, item := range m.resourceFile.Items {
		if item.Server == server {
			items = append(items, item)
		}
	}
	return items
}

func findManagedResource(items []config.ManagedResource, server string, kind string, name string) int {
	for i, item := range items {
		if item.Server == server && item.Kind == kind && item.Name == name {
			return i
		}
	}
	return -1
}

func configResourceKind(kind resourceKind) string {
	switch kind {
	case resourceServices:
		return config.ResourceKindService
	case resourceContainers:
		return config.ResourceKindContainer
	case resourceProcesses:
		return config.ResourceKindProcess
	case resourcePorts:
		return config.ResourceKindPort
	default:
		return ""
	}
}

func resourceKindFromConfig(kind string) resourceKind {
	switch kind {
	case config.ResourceKindService:
		return resourceServices
	case config.ResourceKindContainer:
		return resourceContainers
	case config.ResourceKindProcess:
		return resourceProcesses
	case config.ResourceKindPort:
		return resourcePorts
	default:
		return resourceAll
	}
}

func defaultManagedResource(server string, kind string, name string) config.ManagedResource {
	item := config.ManagedResource{Server: server, Kind: kind, Name: name}
	switch kind {
	case config.ResourceKindService:
		target := shellQuoteLocal(name)
		item.StartCommand = "systemctl start " + target
		item.StopCommand = "systemctl stop " + target
		item.RestartCommand = "systemctl restart " + target
		item.LogCommand = "journalctl -u " + target + " -n 200 --no-pager"
	case config.ResourceKindContainer:
		target := shellQuoteLocal(name)
		item.StartCommand = "docker start " + target
		item.StopCommand = "docker stop " + target
		item.RestartCommand = "docker restart " + target
		item.LogCommand = "docker logs --tail 200 " + target
	case config.ResourceKindPort:
		_, port := splitManagedPortName(name)
		item.HealthCommand = "curl -f http://127.0.0.1:" + shellQuoteLocal(port) + "/health"
	}
	return item
}

func containerResourceHasCustomConfig(item config.ManagedResource) bool {
	if item.Kind != config.ResourceKindContainer {
		return false
	}
	defaults := defaultManagedResource(item.Server, item.Kind, item.Name)
	return strings.TrimSpace(item.StartCommand) != "" && strings.TrimSpace(item.StartCommand) != defaults.StartCommand ||
		strings.TrimSpace(item.StopCommand) != "" && strings.TrimSpace(item.StopCommand) != defaults.StopCommand ||
		strings.TrimSpace(item.RestartCommand) != "" && strings.TrimSpace(item.RestartCommand) != defaults.RestartCommand ||
		strings.TrimSpace(item.LogCommand) != "" && strings.TrimSpace(item.LogCommand) != defaults.LogCommand ||
		strings.TrimSpace(item.HealthCommand) != "" ||
		strings.TrimSpace(item.DeleteCommand) != ""
}

func containerDetailsToCache(items []containerDetail) []config.ResourceContainerCache {
	out := make([]config.ResourceContainerCache, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || item.Missing {
			continue
		}
		out = append(out, config.ResourceContainerCache{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func containerDetailsFromCache(items []config.ResourceContainerCache) []containerDetail {
	out := make([]containerDetail, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		out = append(out, containerDetail{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func splitManagedPortName(name string) (string, string) {
	name = strings.TrimSpace(name)
	proto, port, ok := strings.Cut(name, "/")
	if !ok {
		return "tcp", name
	}
	proto = strings.TrimSpace(proto)
	port = strings.TrimSpace(port)
	if proto == "" {
		proto = "tcp"
	}
	return proto, port
}

func resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	if action == resourceActionDelete {
		return ""
	}
	if kind == resourceProcesses || kind == resourcePorts {
		return ""
	}
	target := shellQuoteLocal(name)
	if kind == resourceServices {
		command := "restart"
		switch action {
		case resourceActionStart:
			command = "start"
		case resourceActionStop:
			command = "stop"
		case resourceActionRestart:
			command = "restart"
		}
		return sudoFallbackScript("systemctl "+command+" "+target, "sudo -n systemctl "+command+" "+target)
	}
	command := "restart"
	switch action {
	case resourceActionStart:
		command = "start"
	case resourceActionStop:
		command = "stop"
	case resourceActionRestart:
		command = "restart"
	}
	return sudoFallbackScript("docker "+command+" "+target, "sudo -n docker "+command+" "+target)
}

func (m Model) resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	if cmd := m.managedResourceCommand(kind, action, name); cmd != "" {
		return sudoFallbackScript(cmd, "sudo -n "+cmd)
	}
	return resourceActionScript(kind, action, name)
}

func resourceLogScript(kind resourceKind, name string, lines int) string {
	if lines <= 0 {
		lines = 200
	}
	target := shellQuoteLocal(name)
	if kind == resourceServices {
		cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager", target, lines)
		sudoCmd := fmt.Sprintf("sudo -n journalctl -u %s -n %d --no-pager", target, lines)
		return sudoFallbackScript(cmd, sudoCmd)
	}
	if kind == resourceProcesses || kind == resourcePorts {
		return ""
	}
	cmd := fmt.Sprintf("docker logs --tail %d %s", lines, target)
	sudoCmd := fmt.Sprintf("sudo -n docker logs --tail %d %s", lines, target)
	return sudoFallbackScript(cmd, sudoCmd)
}

func (m Model) resourceLogScript(kind resourceKind, name string, lines int) string {
	if cmd := m.managedResourceLogCommand(kind, name); cmd != "" {
		return sudoFallbackScript(cmd, "sudo -n "+cmd)
	}
	return resourceLogScript(kind, name, lines)
}

func (m Model) managedResourceCommand(kind resourceKind, action resourceActionKind, name string) string {
	item, ok := m.managedResource(kind, name)
	if !ok {
		return ""
	}
	switch action {
	case resourceActionStart:
		return item.StartCommand
	case resourceActionStop:
		return item.StopCommand
	case resourceActionRestart:
		return item.RestartCommand
	case resourceActionDelete:
		return ""
	default:
		return ""
	}
}

func (m Model) managedResourceLogCommand(kind resourceKind, name string) string {
	item, ok := m.managedResource(kind, name)
	if !ok {
		return ""
	}
	return item.LogCommand
}

func (m Model) managedResource(kind resourceKind, name string) (config.ManagedResource, bool) {
	server := m.resourceServerKey(m.resourceHostIndex)
	configKind := configResourceKind(kind)
	if server == "" || configKind == "" {
		return config.ManagedResource{}, false
	}
	for _, item := range m.resourceFile.Items {
		if item.Server == server && item.Kind == configKind && item.Name == name {
			return item, true
		}
	}
	return config.ManagedResource{}, false
}

func sudoFallbackScript(command string, sudoCommand string) string {
	return fmt.Sprintf(`out=$(%s 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  first="$out"
  out=$(%s 2>&1)
  code=$?
  if [ "$code" -ne 0 ]; then
    case "$first $out" in
      *"permission denied"*|*"Permission denied"*|*"not in the docker group"*|*"password is required"*|*"a password is required"*|*"Authentication is required"*) echo "__SSHM_PERMISSION_DENIED__" ;;
    esac
  fi
fi
printf '%%s\n' "$out"
exit "$code"`, command, sudoCommand)
}

func shellQuoteLocal(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '/' || r == '.' || r == ':' || r == '=' || r == ',' || r == '@' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
