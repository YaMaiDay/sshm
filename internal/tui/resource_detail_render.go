package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) renderResourceDetail() string {
	width := detailFrameWidth(m.width)
	lines := expandLines(m.resourceDetailLines())
	bodyHeight := m.resourceDetailBodyHeight()
	maxScroll := maxInt(0, len(lines)-bodyHeight)
	m.resourceState.Scroll = clampInt(m.resourceState.Scroll, 0, maxScroll)
	if len(lines) > bodyHeight {
		lines = lines[m.resourceState.Scroll : m.resourceState.Scroll+bodyHeight]
	}
	body := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	help := renderHelp(width, m.resourceDetailHelp())
	return strings.Join([]string{titleStyle.Render(fitANSI(m.resourceDetailTitle(), width)), body, help}, "\n")
}

func (m Model) resourceDetailHelp() string {
	partsEN := []string{"Scroll ↑↓/jk", "Page Ctrl+u/d"}
	partsZH := []string{"滚动 ↑↓/jk", "翻页 Ctrl+u/d"}
	if m.selectedResourceManaged() || m.currentSelectedResourceKind() == resourceDatabases {
		if m.currentSelectedResourceKind() == resourceDatabases {
			partsEN = append(partsEN, "Config e")
			partsZH = append(partsZH, "配置 e")
		} else {
			partsEN = append(partsEN, "Edit e")
			partsZH = append(partsZH, "编辑 e")
		}
	}
	partsEN = append(partsEN, "Remove x")
	partsZH = append(partsZH, "移出 x")
	switch m.currentSelectedResourceKind() {
	case resourcePorts:
	case resourceProcesses, resourceServices, resourceContainers:
		partsEN = append(partsEN, "Logs o", "Start s", "Stop p", "Restart c")
		partsZH = append(partsZH, "日志 o", "启动 s", "停止 p", "重启 c")
	}
	partsEN = append(partsEN, "Refresh r", "Back Space/Esc")
	partsZH = append(partsZH, "刷新 r", "返回 Space/Esc")
	return m.t(strings.Join(partsEN, "  "), strings.Join(partsZH, "  "))
}

func (m Model) resourceDetailTitle() string {
	kind := m.currentSelectedResourceKind()
	title := fmt.Sprintf("%s  %s", m.t(m.resourceKindName(kind)+" Detail", m.resourceKindName(kind)+"详情"), m.resourceHostTitle())
	name := m.resourceState.DetailName
	if strings.TrimSpace(name) == "" {
		name = m.currentSelectedResourceName()
	}
	if name != "" {
		title += "  " + name
	}
	return title
}

func (m Model) currentSelectedResourceName() string {
	if ref, ok := m.selectedResourceRef(); ok {
		switch ref.Kind {
		case resourceContainers:
			if item, ok := m.selectedContainer(); ok {
				return item.Name
			}
		case resourceServices:
			if item, ok := m.selectedService(); ok {
				return item.Unit
			}
		case resourceProcesses:
			if item, ok := m.selectedProcess(); ok {
				return item.Process
			}
		case resourcePorts:
			if item, ok := m.selectedPort(); ok {
				return item.Protocol + "/" + item.Port
			}
		case resourceDatabases:
			if item, ok := m.selectedDatabase(); ok {
				return item.Name
			}
		}
	}
	return ""
}

func (m Model) resourceDetailLines() []string {
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceContainers {
		item, ok := m.selectedContainer()
		if !ok {
			return []string{m.t("No selected container", "没有选中的容器")}
		}
		lines := []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceContainers)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Name", "名称"), item.Name),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
			m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
			m.detailRow(m.t("Status", "状态"), item.Status),
			m.detailRow(m.t("Image", "镜像"), item.Image),
			m.detailRow(m.t("Ports", "端口"), emptyDash(simplifyDockerPorts(item.Ports))),
			"",
			sectionTitle(m.t("Resources", "资源监控")),
			m.detailRow(m.t("CPU", "CPU"), containerDetailPercentText(item.CPU, m.containerCPULimitTextForItem(item), 70, 85)),
			m.detailRow(m.t("Memory", "内存"), containerDetailPercentText(item.MemPerc, item.Memory, 70, 85)),
		}
		lines = append(lines, m.containerExtraDetailLines()...)
		lines = append(lines,
			"",
			sectionTitle(m.t("Actions", "操作")),
			m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceContainers, resourceActionStart, item.Name)),
			m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceContainers, resourceActionStop, item.Name)),
			m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceContainers, resourceActionRestart, item.Name)),
			m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceContainers, item.Name, 200)),
		)
		return lines
	}
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourcePorts {
		item, ok := m.selectedPort()
		if !ok {
			return []string{m.t("No selected port", "没有选中的端口")}
		}
		return []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourcePorts)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Protocol", "协议"), item.Protocol),
			m.detailRow(m.t("Port", "端口"), item.Port),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
			m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
			m.detailRow(m.t("Status", "状态"), m.portStatusStyled(item)),
			m.detailRow(m.t("Socket state", "连接状态"), emptyDash(item.State)),
			m.detailRow(m.t("Listen", "监听地址"), emptyDash(item.LocalAddress)),
			m.detailRow(m.t("Remote", "远端地址"), emptyDash(item.ForeignAddress)),
			m.detailRow(m.t("Scope", "监听范围"), m.portScopeText(item)),
			m.detailRow(m.t("Scope note", "范围说明"), m.portScopeNote(item)),
			m.detailRow(m.t("IP version", "IP版本"), emptyDash(portIPVersion(item.LocalAddress))),
			m.detailRow(m.t("Risk", "风险"), m.portRiskText(item)),
			m.detailRow(m.t("Risk note", "风险说明"), m.portRiskNote(item)),
			"",
			sectionTitle(m.t("Process", "进程")),
			m.detailRow(m.t("Process", "进程"), emptyDash(item.Process)),
			m.detailRow("PID", emptyDash(item.PID)),
			m.detailRow("FD", emptyDash(item.FD)),
			m.detailRow(m.t("Service", "服务"), emptyDash(item.ServiceUnit)),
			m.detailRow(m.t("Instances", "实例数"), fmt.Sprintf("%d", item.Count)),
			"",
			sectionTitle("Docker"),
			m.detailRow(m.t("Container", "容器"), emptyDash(item.Container)),
			m.detailRow(m.t("Container port", "容器端口"), emptyDash(item.ContainerPort)),
		}
	}
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceProcesses {
		item, ok := m.selectedProcess()
		if !ok {
			return []string{m.t("No selected process", "没有选中的进程")}
		}
		lines := []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceProcesses)),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Process", "进程"), emptyDash(item.Process)),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.ProcessFavorite)),
			m.detailRow("PID", emptyDash(item.PID)),
			m.detailRow("FD", emptyDash(item.FD)),
			m.detailRow(m.t("Service", "服务"), emptyDash(item.ServiceUnit)),
			m.detailRow(m.t("Status", "状态"), m.processStatusLine(item)),
			"",
			sectionTitle(m.t("Listener", "监听")),
			m.detailRow(m.t("Protocol", "协议"), item.Protocol),
			m.detailRow(m.t("Port", "端口"), item.Port),
			m.detailRow(m.t("Socket state", "连接状态"), emptyDash(item.State)),
			m.detailRow(m.t("Listen", "监听地址"), emptyDash(item.LocalAddress)),
			m.detailRow(m.t("Remote", "远端地址"), emptyDash(item.ForeignAddress)),
			m.detailRow(m.t("Scope", "监听范围"), m.portScopeText(item)),
			m.detailRow(m.t("Scope note", "范围说明"), m.portScopeNote(item)),
			m.detailRow(m.t("Risk", "风险"), m.portRiskText(item)),
			m.detailRow(m.t("Risk note", "风险说明"), m.portRiskNote(item)),
		}
		lines = append(lines, m.processExtraDetailLines(item)...)
		lines = append(lines,
			"",
			sectionTitle(m.t("Actions", "操作")),
			m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceProcesses, resourceActionStart, item.Process)),
			m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceProcesses, resourceActionStop, item.Process)),
			m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceProcesses, resourceActionRestart, item.Process)),
			m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceProcesses, item.Process, 200)),
		)
		return lines
	}
	if ref, ok := m.selectedResourceRef(); ok && ref.Kind == resourceDatabases {
		item, ok := m.selectedDatabase()
		if !ok {
			return []string{m.t("No selected database", "没有选中的数据库")}
		}
		instance := ""
		note := ""
		if managed, ok := m.managedResource(resourceDatabases, item.Name); ok {
			instance = managed.DBInstance
			note = managed.DBNote
		}
		lines := []string{
			sectionTitle(m.t("Basic", "基础信息")),
			m.detailRow(m.t("Type", "类型"), m.t("Database schema", "库")),
			m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
			m.detailRow(m.t("Database", "库名"), item.Name),
			m.detailRow(m.t("Instance", "实例"), emptyDash(instance)),
			m.detailRow(m.t("Engine", "数据库"), item.Engine),
			m.detailRow(m.t("Note", "备注"), emptyDash(note)),
			m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
			m.detailRow(m.t("Found", "发现"), m.databaseFoundText(item, instance)),
			m.detailRow(m.t("Status", "状态"), m.databaseStatusLine(item)),
			m.detailRow(m.t("Source", "来源"), emptyDash(item.Source)),
			m.detailRow(m.t("Endpoint", "地址"), emptyDash(item.Endpoint)),
			m.detailRow(m.t("Connection", "连接方式"), m.t("Run from server", "通过服务器执行")),
			m.detailRow(m.t("Runner", "执行位置"), m.resourceHostTitle()),
			m.detailRow(m.t("Jump host", "跳板机"), m.resourceHostJumpText(m.resourceState.HostIndex)),
			"",
			sectionTitle(m.t("Runtime", "运行信息")),
			m.detailRow(m.t("Container", "容器"), emptyDash(item.Container)),
			m.detailRow(m.t("Image", "镜像"), emptyDash(item.Image)),
			m.detailRow(m.t("Service", "服务"), emptyDash(item.ServiceUnit)),
			m.detailRow(m.t("Process", "进程"), emptyDash(item.Process)),
			m.detailRow("PID", emptyDash(item.PID)),
			m.detailRow(m.t("Protocol", "协议"), emptyDash(item.Protocol)),
			m.detailRow(m.t("Port", "端口"), emptyDash(item.Port)),
			"",
		}
		lines = append(lines, m.databaseExtraDetailLines(item)...)
		return lines
	}
	item, ok := m.selectedService()
	if !ok {
		return []string{m.t("No selected service", "没有选中的服务")}
	}
	item = m.mergedServiceDetail(item)
	return m.serviceDetailLines(item)
}

func (m Model) serviceDetailLines(item resourceservice.ServiceDetail) []string {
	lines := []string{
		sectionTitle(m.t("Basic", "基础信息")),
		m.detailRow(m.t("Type", "类型"), m.resourceKindName(resourceServices)),
		m.detailRow(m.t("Server", "服务器"), m.resourceHostTitle()),
		m.detailRow(m.t("Unit", "服务名"), item.Unit),
		m.detailRow(m.t("Favorites", "收藏"), yesNoText(m.isChineseUI(), item.Favorite)),
		m.detailRow(m.t("Found", "发现"), yesNoText(m.isChineseUI(), !item.Missing)),
		m.detailRow(m.t("Status", "状态"), coloredServiceStatus(serviceRawState(item), serviceDetailKind(item))),
		m.detailRow(m.t("Status note", "状态说明"), m.serviceStateNote(item)),
		m.detailRow(m.t("Summary", "摘要"), coloredServiceStatus(m.serviceStatusText(item), serviceDetailKind(item))),
		m.detailRow(m.t("Load", "加载"), emptyDash(item.Load)),
		m.detailRow(m.t("Load note", "加载说明"), m.serviceLoadNote(item)),
	}
	if row := m.serviceDetailLoadRow(); strings.TrimSpace(row) != "" {
		lines = append(lines, row)
	}
	result := serviceResultStyle(item.Result)
	if strings.TrimSpace(result) == "" {
		result = emptyDash(item.Result)
	}
	exitCode := serviceExitStyle(item.ExecMainStatus)
	if strings.TrimSpace(exitCode) == "" {
		exitCode = emptyDash(item.ExecMainStatus)
	}
	lines = append(lines,
		m.detailRow(m.t("Enabled", "开机启动"), emptyDash(item.UnitFileState)),
		m.detailRow(m.t("Result", "结果"), result),
		m.detailRow(m.t("Exit code", "退出码"), exitCode),
		m.detailRow(m.t("Restarts", "重启次数"), emptyDash(item.NRestarts)),
		m.detailRow(m.t("Desc", "说明"), emptyDash(item.Description)),
		"",
		sectionTitle(m.t("Resources", "资源监控")),
		m.detailRow("PID", emptyDash(servicePIDText(item))),
		m.detailRow(m.t("Memory", "内存"), serviceMemoryText(item)),
		m.detailRow(m.t("Tasks", "任务数"), emptyDash(item.TasksCurrent)),
		m.detailRow(m.t("Control group", "控制组"), emptyDash(item.ControlGroup)),
		m.detailRow("Slice", emptyDash(item.Slice)),
		"",
		sectionTitle(m.t("Time", "时间")),
		m.detailRow(m.t("Active since", "启动时间"), emptyDash(formatSystemdTimestamp(item.ActiveSince))),
		m.detailRow(m.t("Inactive since", "停止时间"), emptyDash(formatSystemdTimestamp(item.InactiveSince))),
		m.detailRow(m.t("State changed", "状态变化"), emptyDash(formatSystemdTimestamp(item.StateChangedAt))),
		m.detailRow(m.t("Process start", "进程启动"), emptyDash(formatSystemdTimestamp(item.ExecStartedAt))),
		m.detailRow(m.t("Process exit", "进程退出"), emptyDash(formatSystemdTimestamp(item.ExecExitedAt))),
		"",
		sectionTitle(m.t("Startup", "启动配置")),
		m.detailRow(m.t("User", "用户"), emptyDash(item.User)),
		m.detailRow(m.t("Group", "用户组"), emptyDash(item.Group)),
		m.detailRow(m.t("Restart", "重启策略"), emptyDash(item.Restart)),
		m.detailRow(m.t("Restart delay", "重启延迟"), emptyDash(serviceRestartDelayText(item.RestartSec))),
		m.detailRow(m.t("Source", "来源"), emptyDash(item.FragmentPath)),
		m.detailRow("Drop-in", emptyDash(item.DropInPaths)),
		m.detailRow(m.t("Workdir", "工作目录"), emptyDash(item.WorkingDirectory)),
		m.detailRow(m.t("Program", "程序路径"), emptyDash(serviceProgramPath(item))),
		m.detailRow(m.t("Exec", "启动"), emptyDash(item.ExecStart)),
		m.detailRow(m.t("Stop", "停止"), emptyDash(item.ExecStop)),
		m.detailRow(m.t("Reload", "重载"), emptyDash(item.ExecReload)),
		"",
		sectionTitle(m.t("Actions", "操作")),
		m.detailRow(m.t("Start", "启动"), "s  "+m.resourceCommandPreview(resourceServices, resourceActionStart, item.Unit)),
		m.detailRow(m.t("Stop", "停止"), "p  "+m.resourceCommandPreview(resourceServices, resourceActionStop, item.Unit)),
		m.detailRow(m.t("Restart", "重启"), "r  "+m.resourceCommandPreview(resourceServices, resourceActionRestart, item.Unit)),
		m.detailRow(m.t("Logs", "日志"), "o  "+m.resourceLogCommandPreview(resourceServices, item.Unit, 200)),
	)
	return lines
}

func (m Model) processExtraDetailLines(item resourceservice.PortDetail) []string {
	if strings.TrimSpace(item.PID) == "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), "-")}
	}
	if m.resourceState.ProcessExtraPID != item.PID {
		return nil
	}
	if m.resourceState.ProcessExtraLoading {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(m.resourceState.ProcessExtraErr) != "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), redStyle.Render(m.resourceState.ProcessExtraErr))}
	}
	d := m.resourceState.ProcessExtra
	lines := []string{
		"",
		sectionTitle(m.t("Runtime", "运行信息")),
		m.detailRow(m.t("User", "用户"), emptyDash(d.User)),
		m.detailRow(m.t("Parent PID", "父进程"), emptyDash(d.PPID)),
		m.detailRow(m.t("State", "进程状态"), emptyDash(d.State)),
		m.detailRow("CPU", emptyDash(percentSuffix(d.CPU))),
		m.detailRow(m.t("Memory", "内存"), processMemoryText(d)),
		m.detailRow("RSS", processRSSText(d.RSS)),
		m.detailRow(m.t("Elapsed", "运行时长"), emptyDash(d.Elapsed)),
		m.detailRow(m.t("Started", "启动时间"), emptyDash(d.Started)),
		m.detailRow(m.t("Command", "命令"), emptyDash(d.Command)),
		"",
		sectionTitle(m.t("Paths", "路径")),
		m.detailRow(m.t("Executable", "可执行文件"), emptyDash(d.Executable)),
		m.detailRow(m.t("Workdir", "工作目录"), emptyDash(d.WorkingDir)),
		m.detailRow(m.t("Command line", "命令行"), emptyDash(d.CommandLine)),
		"",
		sectionTitle(m.t("Ownership", "归属")),
		m.detailRow(m.t("Service", "服务"), emptyDash(firstNonEmpty(d.ServiceUnit, item.ServiceUnit))),
		m.detailRow(m.t("Control group", "控制组"), emptyDash(d.ControlGroup)),
	}
	return lines
}

func (m Model) containerExtraDetailLines() []string {
	if m.resourceState.ContainerExtraLoading {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中"))}
	}
	if strings.TrimSpace(m.resourceState.ContainerExtraErr) != "" {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), redStyle.Render(m.resourceState.ContainerExtraErr))}
	}
	d := m.resourceState.ContainerExtra
	if d.ID == "" && d.Size == "" && d.VirtualSize == "" && d.BlockIO == "" && len(d.Mounts) == 0 {
		return []string{"", sectionTitle(m.t("Details", "详情")), m.detailRow(m.t("Status", "状态"), "-")}
	}
	lines := []string{
		"",
		sectionTitle(m.t("Details", "详情")),
		m.detailRow("ID", shortContainerID(d.ID)),
		m.detailRow(m.t("Created", "创建"), emptyDash(formatContainerTimestamp(d.Created))),
		m.detailRow(m.t("Started", "启动时间"), emptyDash(formatContainerTimestamp(d.StartedAt))),
		m.detailRow(m.t("Last stop", "上次停止"), emptyDash(formatContainerTimestamp(d.FinishedAt))),
		m.detailRow(m.t("State", "状态"), emptyDash(d.StateStatus)),
		m.detailRow(m.t("Health", "健康"), emptyDash(d.HealthStatus)),
		m.detailRow(m.t("Exit code", "退出码"), fmt.Sprintf("%d", d.ExitCode)),
		m.detailRow(m.t("Restart", "重启策略"), emptyDash(d.RestartPolicy)),
		m.detailRow(m.t("Driver", "驱动"), emptyDash(d.Driver)),
		m.detailRow(m.t("Platform", "平台"), emptyDash(d.Platform)),
		m.detailRow(m.t("Command", "命令"), emptyDash(containerCommandText(d))),
		"",
		sectionTitle(m.t("Storage", "存储")),
		m.detailRow(m.t("Writable layer", "可写层"), firstNonEmpty(d.Size, bytesHuman(d.SizeRW))),
		m.detailRow(m.t("Virtual size", "虚拟大小"), firstNonEmpty(d.VirtualSize, bytesHuman(d.SizeRootFS))),
		m.detailRow(m.t("Block IO", "块IO"), emptyDash(d.BlockIO)),
	}
	if len(d.Mounts) > 0 {
		lines = append(lines, "", sectionTitle(m.t("Mounts", "挂载")))
		for i, mount := range d.Mounts {
			mode := "ro"
			if mount.RW {
				mode = "rw"
			}
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				detailSubTitle(fmt.Sprintf("%02d  %s", i+1, emptyDash(mount.Destination))),
				m.detailRow(m.t("Type", "类型"), emptyDash(mount.Type)+"  "+mode),
				m.detailRow(m.t("Source", "来源"), emptyDash(mount.Source)),
			)
		}
	}
	if len(d.Networks) > 0 {
		lines = append(lines, "", sectionTitle(m.t("Networks", "网络")))
		for i, network := range d.Networks {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines,
				detailSubTitle(fmt.Sprintf("%02d  %s", i+1, emptyDash(network.Name))),
				m.detailRow("IP", emptyDash(network.IPAddress)),
				m.detailRow(m.t("Gateway", "网关"), emptyDash(network.Gateway)),
				m.detailRow("MAC", emptyDash(network.MacAddress)),
				m.detailRow(m.t("Aliases", "别名"), emptyDash(strings.Join(network.Aliases, ", "))),
				m.detailRow(m.t("Network ID", "网络ID"), shortContainerID(network.NetworkID)),
				m.detailRow(m.t("Endpoint ID", "端点ID"), shortContainerID(network.EndpointID)),
			)
		}
	}
	return lines
}

func (m Model) databaseExtraDetailLines(item resourceservice.DatabaseDetail) []string {
	lines := []string{"", sectionTitle(m.t("Deep metrics", "深度指标"))}
	if m.resourceState.DatabaseExtraName != item.Name {
		return append(lines, m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中")))
	}
	if m.resourceState.DatabaseExtraLoading {
		return append(lines, m.detailRow(m.t("Status", "状态"), m.t("Loading", "加载中")))
	}
	if strings.TrimSpace(m.resourceState.DatabaseExtraErr) != "" {
		lines = append(lines,
			m.detailRow(m.t("Status", "状态"), redStyle.Render(m.resourceState.DatabaseExtraErr)),
			m.detailRow(m.t("Config", "配置"), m.t("Press e to configure or update database connection.", "按 e 配置或更新数据库连接。")),
		)
		return lines
	}
	d := m.resourceState.DatabaseExtra
	lines = append(lines,
		m.detailRow(m.t("Version", "版本"), emptyDash(d.Version)),
		m.detailRow(m.t("Uptime", "运行时间"), emptyDash(d.Uptime)),
	)
	lines = append(lines, "", sectionTitle(m.t("Storage", "存储")))
	if d.SizeBytes > 0 || d.DataBytes > 0 || d.IndexBytes > 0 || d.DBTotalBytes > 0 || d.DatabaseCount != "" || len(d.DatabaseTop) > 0 || d.TableCount != "" || len(d.TableTop) > 0 {
		if d.DatabaseCount != "" || d.DBTotalBytes > 0 {
			total := detailValueStyle.Render(emptyDash(d.DatabaseCount))
			if d.DBTotalBytes > 0 {
				total += "  " + databaseSizeValue(bytesHuman(d.DBTotalBytes))
			}
			lines = append(lines, m.detailRow(m.t("Databases", "数据库"), total))
		}
		if len(d.DatabaseTop) > 0 {
			lines = append(lines, m.detailRow(m.t("Database Top 10", "库排行 Top 10"), ""))
			for _, database := range d.DatabaseTop {
				lines = append(lines, m.databaseTableTopLine(database))
			}
		}
		if strings.TrimSpace(d.Database) != "" {
			lines = append(lines, m.detailRow(m.t("Current DB", "当前库"), d.Database))
		}
		lines = append(lines, m.detailRow(m.t("Used", "占用"), m.databaseStorageUsageText(d)))
		if d.DataBytes > 0 {
			lines = append(lines, m.detailRow(m.t("Data", "数据"), databaseSizeValue(bytesHuman(d.DataBytes))))
		}
		if d.IndexBytes > 0 {
			lines = append(lines, m.detailRow(m.t("Index", "索引"), databaseSizeValue(bytesHuman(d.IndexBytes))))
		}
		if d.IndexTotalBytes > 0 {
			lines = append(lines, m.detailRow(m.t("Total index", "总索引"), databaseSizeValue(bytesHuman(d.IndexTotalBytes))))
		}
		if d.TableCount != "" {
			lines = append(lines, m.detailRow(m.t("Tables", "表总数"), d.TableCount))
		}
		if len(d.TableTop) > 0 {
			lines = append(lines, m.detailRow(m.t("Table Top 10", "表排行 Top 10"), ""))
			for _, table := range d.TableTop {
				lines = append(lines, m.databaseTableTopLine(table))
			}
		}
	} else if d.MemoryUsed != "" {
		lines = append(lines,
			m.detailRow(m.t("Used", "占用"), databaseSizeValue(d.MemoryUsed)),
			m.detailRow(m.t("Peak", "峰值"), databaseSizeValue(emptyDash(d.MemoryPeak))),
		)
	} else {
		lines = append(lines, m.detailRow(m.t("Used", "占用"), m.t("Not collected", "未获取")))
	}
	lines = append(lines, "", sectionTitle(m.t("Performance", "性能")))
	if d.Connections != "" || d.MaxConnections != "" {
		conn := emptyDash(d.Connections)
		if d.MaxConnections != "" {
			conn += " / " + d.MaxConnections
		}
		lines = append(lines, m.detailRow(m.t("Connections", "连接"), conn))
	}
	if d.ActiveConns != "" || d.IdleConns != "" {
		lines = append(lines, m.detailRow(m.t("Connection state", "连接状态"), fmt.Sprintf("%s %s  %s %s", m.t("active", "活跃"), emptyDash(d.ActiveConns), m.t("idle", "空闲"), emptyDash(d.IdleConns))))
	}
	if d.CacheHit != "" {
		lines = append(lines, m.detailRow(m.t("Cache hit", "缓存命中"), d.CacheHit))
	}
	if d.LockWaits != "" || d.LongTx != "" || d.Deadlocks != "" {
		lines = append(lines,
			m.detailRow(m.t("Lock waits", "锁等待"), emptyDash(d.LockWaits)),
			m.detailRow(m.t("Long tx", "长事务"), emptyDash(d.LongTx)),
			m.detailRow(m.t("Deadlocks", "死锁"), emptyDash(d.Deadlocks)),
		)
	}
	if d.Questions != "" {
		lines = append(lines, m.detailRow("Questions", d.Questions))
	}
	if d.SlowQueries != "" {
		lines = append(lines, m.detailRow(m.t("Slow queries", "慢查询"), d.SlowQueries))
	}
	if d.OpsPerSec != "" || d.Clients != "" || d.Keyspace != "" {
		lines = append(lines,
			m.detailRow("OPS", emptyDash(d.OpsPerSec)),
			m.detailRow(m.t("Clients", "客户端"), emptyDash(d.Clients)),
			m.detailRow("Keyspace", emptyDash(d.Keyspace)),
		)
	}
	return lines
}
