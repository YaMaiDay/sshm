package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderDeploymentConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	hostName := m.deploymentConfirmServerName()
	lines := m.deploymentConfirmLines(hostName, bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentState.OutputScroll, 0, m.deploymentConfirmMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.deploymentConfirmBorderColor()).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit(m.t("Confirm Deployment", "确认部署"), width)), box, renderHelp(width, m.deploymentConfirmHelp())}, "\n")
}

func (m Model) deploymentConfirmBorderColor() lipgloss.Color {
	if m.deploymentState.Active.Running {
		return blue
	}
	if len(m.deploymentState.Active.Queue) > 0 && m.deploymentState.Active.QueueFailed >= 0 && m.deploymentState.Active.ExitCode != 0 {
		return red
	}
	return softGray
}

func (m Model) deploymentConfirmLines(hostName string, bodyWidth int) []string {
	app := m.deploymentState.Confirm
	queue := m.deploymentState.ConfirmQueue
	if len(m.deploymentState.Active.Queue) > 0 {
		queue = m.deploymentState.Active.Queue
	}
	if len(queue) == 0 {
		queue = []config.DeploymentApp{app}
	}
	return m.deploymentQueueConfirmLines(queue, bodyWidth)
}

func (m Model) deploymentConfirmHelp() string {
	if m.deploymentState.Active.Running {
		return m.t("Scroll ↑↓/jk  Running", "滚动 ↑↓/jk  执行中")
	}
	if len(m.deploymentState.Active.Queue) > 0 && m.deploymentState.Active.QueueFailed >= 0 && m.deploymentState.Active.ExitCode != 0 {
		return m.t("Scroll ↑↓/jk  Retry failed r  Redeploy a  Back q/Esc", "滚动 ↑↓/jk  重试失败 r  重新部署 a  返回 q/Esc")
	}
	if len(m.deploymentState.Active.Queue) > 0 && m.deploymentState.Active.Output != "" {
		return m.t("Scroll ↑↓/jk  Redeploy a  Back q/Esc", "滚动 ↑↓/jk  重新部署 a  返回 q/Esc")
	}
	return m.t("Scroll ↑↓/jk  Start Enter  Retry failed r  Redeploy a  Back q/Esc", "滚动 ↑↓/jk  开始 Enter  重试失败 r  重新部署 a  返回 q/Esc")
}

func (m Model) deploymentQueueConfirmLines(queue []config.DeploymentApp, bodyWidth int) []string {
	current := m.deploymentQueueCurrentApp(queue)
	lines := []string{}
	if len(queue) == 1 {
		lines = append(lines, detailSubTitle(m.t("Deployment Info", "部署信息")))
		lines = append(lines, m.deploymentInfoLines(current, bodyWidth)...)
	} else {
		lines = append(lines,
			detailSubTitle(m.t("Deployment Queue", "部署队列")),
			mutedStyle.Render(fit(m.t("Run serially in the order below; after each app, wait its configured seconds before the next one.", "按下面顺序串行执行；每个应用完成后按自己的等待时间进入下一个。"), bodyWidth)),
			"",
		)
		for i, app := range queue {
			lines = append(lines, m.deploymentQueueLine(m.deploymentState.Active, i, app, bodyWidth))
		}
	}
	lines = append(lines, "", detailSubTitle(m.t("Current Flow", "当前流程")), fit(m.deploymentQueueFlowText(current), bodyWidth))
	if len(m.deploymentState.Active.Queue) > 0 {
		lines = append(lines, "", detailSubTitle(m.t("Output", "执行输出")))
		lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
		if !m.deploymentState.Active.Running && m.deploymentState.Active.Output != "" {
			lines = append(lines, "", fmt.Sprintf("%s %d", m.t("Exit code", "退出码"), m.deploymentState.Active.ExitCode))
		}
	}
	if len(queue) == 1 {
		records := m.deploymentRecordsForApp(current, 50)
		lines = append(lines, "", detailSubTitle(fmt.Sprintf("%s %d%s", m.t("History", "历史"), len(records), m.t(" records", "条"))))
		if len(records) == 0 {
			lines = append(lines, mutedStyle.Render(m.t("No records", "暂无记录")))
		} else {
			for _, record := range records {
				lines = append(lines, m.deploymentDetailHistoryLine(record, bodyWidth))
			}
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (m Model) deploymentQueueLine(active activeDeployment, index int, app config.DeploymentApp, width int) string {
	icon := deploymentQueueStatusStyle(active, index).Render(deploymentQueueStatusIcon(active, index))
	seq := cardMutedStyle.Render(fmt.Sprintf("%02d", index+1))
	name := deploymentQueueNameStyle(active, index).Render(padVisible(emptyDash(app.Name), 14))
	server := cardMutedStyle.Render(padVisible(emptyDash(app.Server), 18))
	source := detailValueStyle.Render(padVisible(deploySourceText(app.Source), 7))
	target := cardMutedStyle.Render(padVisible(deploymentAppTarget(app), 10))
	wait := cardMutedStyle.Render(fmt.Sprintf("%s %d%s", m.t("Wait", "等待"), maxInt(0, app.WaitSeconds), m.t("s", "秒")))
	return fitANSI(icon+" "+strings.Join([]string{seq, name, server, source, target, wait}, "  "), width)
}

func deploymentQueueNameStyle(active activeDeployment, index int) lipgloss.Style {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return blueStyle.Bold(true)
	case "done":
		return greenStyle
	case "failed":
		return redStyle
	default:
		return detailValueStyle
	}
}

func deploymentAppTarget(app config.DeploymentApp) string {
	target := app.Branch
	if app.Source == config.DeploySourceRelease {
		target = app.Version
	}
	if strings.TrimSpace(target) == "" {
		return "-"
	}
	return target
}

func (m Model) deploymentInfoLines(app config.DeploymentApp, bodyWidth int) []string {
	return []string{
		deploymentDetailRow(m.t("App", "应用"), emptyDash(app.Name), bodyWidth),
		deploymentDetailRow(m.t("Server", "服务器"), deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow(m.t("Source", "来源"), deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow(m.t("Repo", "仓库"), emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow(m.t("Path", "目录"), emptyDash(app.Path), bodyWidth),
		deploymentDetailRow(m.t("Credential", "凭证"), m.deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow(m.t("Credential param", "凭证参数"), emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow(m.t("Favorite", "收藏"), yesNoLang(app.Favorite, m.isChineseUI()), bodyWidth),
		deploymentDetailRow(m.t("Pinned", "置顶"), yesNoLang(app.Pinned, m.isChineseUI()), bodyWidth),
	}
}

func (m Model) deploymentQueueCurrentApp(queue []config.DeploymentApp) config.DeploymentApp {
	if len(m.deploymentState.Active.Queue) > 0 {
		index := clampInt(m.deploymentState.Active.QueueIndex, 0, len(m.deploymentState.Active.Queue)-1)
		return m.deploymentState.Active.Queue[index]
	}
	if len(queue) > 0 {
		return queue[0]
	}
	return config.DeploymentApp{}
}

func deploymentQueueStatusIcon(active activeDeployment, index int) string {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return "▶"
	case "done":
		return "✓"
	case "failed":
		return "✕"
	default:
		return "·"
	}
}

func deploymentQueueStatusStyle(active activeDeployment, index int) lipgloss.Style {
	status := deploymentQueueItemStatus(active, index)
	switch status {
	case "running":
		return blueStyle.Bold(true)
	case "done":
		return greenStyle
	case "failed":
		return redStyle
	default:
		return lipgloss.NewStyle()
	}
}

func deploymentQueueItemStatus(active activeDeployment, index int) string {
	if len(active.Queue) == 0 {
		return "pending"
	}
	if active.QueueFailed == index && active.ExitCode != 0 {
		return "failed"
	}
	if active.Running && active.QueueIndex == index {
		return "running"
	}
	if active.QueueFailed >= 0 && active.ExitCode != 0 {
		if index < active.QueueFailed {
			return "done"
		}
		return "pending"
	}
	if index < active.QueueIndex {
		return "done"
	}
	if !active.Running && active.QueueIndex == len(active.Queue)-1 && index == active.QueueIndex && active.Output != "" && active.ExitCode == 0 {
		return "done"
	}
	return "pending"
}

func (m Model) deploymentQueueFlowText(app config.DeploymentApp) string {
	parts := []string{}
	if len(app.BeforeCommands) > 0 {
		parts = append(parts, fmt.Sprintf("%s %d%s", m.t("Before", "更新前"), len(app.BeforeCommands), m.t(" steps", "步")))
	}
	parts = append(parts, fmt.Sprintf("%s %d%s", m.t("Fetch", "获取资源"), len(app.ResourceCommands), m.t(" steps", "步")))
	if len(app.UpdateCommands) > 0 {
		parts = append(parts, fmt.Sprintf("%s %d%s", m.t("Update", "更新"), len(app.UpdateCommands), m.t(" steps", "步")))
	}
	if len(app.AfterCommands) > 0 {
		parts = append(parts, fmt.Sprintf("%s %d%s", m.t("After", "更新后"), len(app.AfterCommands), m.t(" steps", "步")))
	}
	if len(app.HealthCommands) > 0 {
		parts = append(parts, fmt.Sprintf("%s %d%s", m.t("Health", "健康检查"), len(app.HealthCommands), m.t(" steps", "步")))
	}
	return strings.Join(parts, "  ")
}

func (m Model) deploymentHistoryLine(record config.DeploymentRecord, width int) string {
	version := deploymentRecordVersionText(record)
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = fmt.Sprintf("  %s %d", m.t("exit", "退出码"), record.ExitCode)
	}
	line := fmt.Sprintf("%s  %s  %s%s",
		padVisible(deploymentRecordDateTimeText(record.Time), 11),
		padVisible(m.deploymentRecordActionStatusText(record), 14),
		version,
		exit,
	)
	return fit(line, width)
}

func deploymentRecordVersionText(record config.DeploymentRecord) string {
	previous := strings.TrimSpace(record.PreviousVersion)
	current := strings.TrimSpace(record.CurrentVersion)
	if previous == "" {
		previous = "-"
	}
	if current == "" {
		current = "-"
	}
	return previous + " -> " + current
}

func appendWrappedCommandPreview(lines []string, command string, width int) []string {
	wrapped := strings.Split(wrapPlainLine("$ "+command, width), "\n")
	return append(lines, wrapped...)
}

func deploymentStepCountLine(label string, count int, width int) string {
	return fit(fmt.Sprintf("%s  %d步", padVisible(label, 10), count), width)
}

func (m Model) renderDeploymentRollbackConfirm() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := m.deploymentRollbackConfirmLines(bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentState.OutputScroll, 0, m.deploymentRollbackConfirmMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit(m.t("Confirm Rollback", "确认回滚"), width)), box, renderHelp(width, m.t("Scroll ↑↓/jk  Run Enter  Back Esc", "滚动 ↑↓/jk  执行 Enter  返回 Esc"))}, "\n")
}

func (m Model) deploymentRollbackConfirmLines(bodyWidth int) []string {
	app := m.deploymentState.Active.App
	lines := []string{
		modalLine(m.t("Server", "服务器"), m.activeDeploymentServerName(), bodyWidth),
		modalLine(m.t("App", "应用"), app.Name, bodyWidth),
		modalLine(m.t("Previous version", "上一版本"), emptyDash(m.deploymentState.Active.PreviousVersion), bodyWidth),
		modalLine(m.t("Current version", "当前版本"), emptyDash(m.deploymentState.Active.CurrentVersion), bodyWidth),
		"",
		detailSubTitle(m.t("Rollback commands", "回滚命令")),
	}
	for _, command := range app.RollbackCommands {
		lines = appendWrappedCommandPreview(lines, command, bodyWidth)
	}
	return lines
}

func (m Model) deploymentRollbackConfirmMaxScroll() int {
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := len(m.deploymentRollbackConfirmLines(bodyWidth))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) activeDeploymentServerName() string {
	if m.deploymentState.Active.HostIndex >= 0 && m.deploymentState.Active.HostIndex < len(m.states) {
		return hostDisplayName(m.states[m.deploymentState.Active.HostIndex].Host)
	}
	return emptyDash(m.deploymentState.Active.App.Server)
}

func (m Model) deploymentConfirmMaxScroll() int {
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	hostName := m.deploymentConfirmServerName()
	lines := len(m.deploymentConfirmLines(hostName, bodyWidth))
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) deploymentConfirmServerName() string {
	index := m.deploymentServerIndex(m.deploymentState.Confirm.Server)
	if index >= 0 && index < len(m.states) {
		return hostDisplayName(m.states[index].Host)
	}
	return emptyDash(m.deploymentState.Confirm.Server)
}

func (m Model) renderDeploymentOutput() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := m.t("Scroll ↑↓/jk  Rollback r  Back q/Esc", "滚动 ↑↓/jk  回滚 r  返回 q/Esc")
	title := m.t("Deployment Output  ", "部署输出  ") + m.deploymentState.Active.App.Name
	lines := []string{
		modalLine(m.t("App", "应用"), m.deploymentState.Active.App.Name, bodyWidth),
		modalLine(m.t("Source", "来源"), deploySourceText(m.deploymentState.Active.App.Source), bodyWidth),
		modalLine(m.t("Queue", "队列"), deploymentQueueProgressText(m.deploymentState.Active), bodyWidth),
		modalLine(m.t("Previous version", "上一版本"), emptyDash(m.deploymentState.Active.PreviousVersion), bodyWidth),
		modalLine(m.t("Current version", "当前版本"), emptyDash(m.deploymentState.Active.CurrentVersion), bodyWidth),
		"",
	}
	lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
	if !m.deploymentState.Active.Running {
		lines = append(lines, "", fmt.Sprintf("退出码 %d", m.deploymentState.Active.ExitCode))
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentState.OutputScroll, 0, m.deploymentOutputMaxScroll())
	if len(lines) > height {
		lines = lines[scroll:minInt(len(lines), scroll+height)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(fitLines(lines, bodyWidth), "\n"))
	return strings.Join([]string{titleStyle.Render(fit(title, width)), box, renderHelp(width, help)}, "\n")
}

func (m Model) deploymentOutputMaxScroll() int {
	lines := 6
	bodyWidth := detailFrameWidth(m.width) - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines += len(m.deploymentOutputContentLines(bodyWidth))
	if !m.deploymentState.Active.Running {
		lines += 2
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) deploymentOutputContentLines(width int) []string {
	stages := deploymentExecutionStages(m.deploymentState.Active.App, m.deploymentState.Active.Action)
	output := strings.TrimRight(m.deploymentState.Active.Output, "\n")
	sections, loose, lastStage := deploymentOutputSections(output)
	if len(stages) == 0 {
		if output == "" {
			return []string{mutedStyle.Render(m.t("(no output)", "(无输出)"))}
		}
		return m.deploymentOutputLines(output, width)
	}
	currentIndex := 0
	if lastStage != "" {
		for i, stage := range stages {
			if stage == lastStage {
				currentIndex = i
				break
			}
		}
	}
	lines := []string{}
	if len(loose) > 0 {
		lines = append(lines, detailSubTitle(m.t("Output", "输出")))
		for _, line := range loose {
			lines = append(lines, fit(line, width))
		}
		lines = append(lines, "")
	}
	for i, stage := range stages {
		status := "pending"
		if m.deploymentState.Active.Running {
			if i < currentIndex {
				status = "done"
			} else if i == currentIndex {
				status = "running"
			}
		} else {
			if m.deploymentState.Active.ExitCode != 0 {
				if i < currentIndex {
					status = "done"
				} else if i == currentIndex {
					status = "failed"
				}
			} else {
				status = "done"
			}
		}
		lines = append(lines, m.deploymentOutputStageLine(stage, status, width))
		stageLines := sections[stage]
		if len(stageLines) == 0 && status == "running" {
			lines = append(lines, mutedStyle.Render("  "+m.t("Running...", "正在执行...")))
		}
		for _, line := range stageLines {
			lines = append(lines, fit("  "+line, width))
		}
	}
	return lines
}

func deploymentExecutionStages(app config.DeploymentApp, action string) []string {
	if action == config.DeployActionRollback {
		if len(app.RollbackCommands) == 0 {
			return nil
		}
		return []string{"回滚"}
	}
	stages := []string{}
	if len(app.BeforeCommands) > 0 {
		stages = append(stages, "更新前")
	}
	stages = append(stages, "获取资源")
	if app.FetchMode == config.DeployFetchLocal {
		stages = append(stages, "上传资源")
	}
	if len(app.UpdateCommands) > 0 {
		stages = append(stages, "更新")
	}
	if len(app.AfterCommands) > 0 {
		stages = append(stages, "更新后")
	}
	if len(app.HealthCommands) > 0 {
		stages = append(stages, "健康检查")
	}
	return stages
}

func deploymentOutputStageLine(stage string, status string, width int) string {
	switch status {
	case "running":
		return blueStyle.Bold(true).Render(fit("▶ "+stage, width))
	case "done":
		return greenStyle.Render(fit("✓ "+stage, width))
	case "failed":
		return redStyle.Render(fit("✕ "+stage, width))
	default:
		return mutedStyle.Render(fit("· "+stage, width))
	}
}

func (m Model) deploymentOutputStageLine(stage string, status string, width int) string {
	text := m.deploymentStageText(stage)
	switch status {
	case "running":
		return blueStyle.Bold(true).Render(fit("▶ "+text, width))
	case "done":
		return greenStyle.Render(fit("✓ "+text, width))
	case "failed":
		return redStyle.Render(fit("✕ "+text, width))
	default:
		return mutedStyle.Render(fit("· "+text, width))
	}
}

func deploymentQueueProgressText(active activeDeployment) string {
	if len(active.Queue) <= 1 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", active.QueueIndex+1, len(active.Queue))
}

func deploymentOutputSections(output string) (map[string][]string, []string, string) {
	sections := map[string][]string{}
	loose := []string{}
	current := ""
	for _, line := range strings.Split(output, "\n") {
		if isDeploymentVersionMarker(line) {
			continue
		}
		if title, ok := deploymentOutputStageTitle(line); ok {
			current = title
			if _, exists := sections[current]; !exists {
				sections[current] = []string{}
			}
			continue
		}
		if current == "" {
			if strings.TrimSpace(line) != "" {
				loose = append(loose, line)
			}
			continue
		}
		sections[current] = append(sections[current], line)
	}
	return sections, loose, current
}

func (m Model) deploymentOutputLines(output string, width int) []string {
	rawLines := strings.Split(output, "\n")
	lines := []string{}
	for _, line := range rawLines {
		if isDeploymentVersionMarker(line) {
			continue
		}
		if title, ok := deploymentOutputStageTitle(line); ok {
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				lines = append(lines, "")
			}
			lines = append(lines, detailSubTitle(m.deploymentStageText(title)))
			continue
		}
		lines = append(lines, fit(line, width))
	}
	return lines
}

func (m Model) deploymentStageText(stage string) string {
	if m.isChineseUI() {
		return stage
	}
	switch stage {
	case "更新前":
		return "Before"
	case "获取资源":
		return "Fetch"
	case "上传资源":
		return "Upload resource"
	case "更新":
		return "Update"
	case "更新后":
		return "After"
	case "健康检查":
		return "Health"
	case "回滚":
		return "Rollback"
	default:
		return stage
	}
}

func isDeploymentVersionMarker(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "SSHM_PREVIOUS_VERSION=") || strings.HasPrefix(line, "SSHM_CURRENT_VERSION=")
}

func deploymentOutputStageTitle(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "== ") || !strings.HasSuffix(line, " ==") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "== "), " =="))
	if title == "" {
		return "", false
	}
	return title, true
}

func deploySourceText(source string) string {
	if source == config.DeploySourceRelease {
		return "Release"
	}
	return "Git"
}

func deployFetchModeText(value string) string {
	if value == config.DeployFetchRemote {
		return "服务器拉取"
	}
	return "本地拉取后上传"
}

func (m Model) deployFetchModeText(value string) string {
	if value == config.DeployFetchRemote {
		return m.t("Remote fetch", "服务器拉取")
	}
	return m.t("Local fetch + upload", "本地拉取后上传")
}

func deployCredentialText(value string) string {
	switch value {
	case config.DeployCredentialSSH:
		return "SSH Key"
	case config.DeployCredentialToken:
		return "Token"
	default:
		return "不配置"
	}
}

func (m Model) deployCredentialText(value string) string {
	switch value {
	case config.DeployCredentialSSH:
		return "SSH Key"
	case config.DeployCredentialToken:
		return "Token"
	default:
		return m.t("None", "不配置")
	}
}
