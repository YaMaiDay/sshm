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
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentConfirmMaxScroll())
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
	return strings.Join([]string{titleStyle.Render(fit("确认部署", width)), box, renderHelp(width, m.deploymentConfirmHelp())}, "\n")
}

func (m Model) deploymentConfirmBorderColor() lipgloss.Color {
	if m.activeDeployment.Running {
		return blue
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.QueueFailed >= 0 && m.activeDeployment.ExitCode != 0 {
		return red
	}
	return softGray
}

func (m Model) deploymentConfirmLines(hostName string, bodyWidth int) []string {
	app := m.deploymentConfirm
	queue := m.deploymentConfirmQueue
	if len(m.activeDeployment.Queue) > 0 {
		queue = m.activeDeployment.Queue
	}
	if len(queue) == 0 {
		queue = []config.DeploymentApp{app}
	}
	return m.deploymentQueueConfirmLines(queue, bodyWidth)
}

func (m Model) deploymentConfirmHelp() string {
	if m.activeDeployment.Running {
		return "滚动 ↑↓/jk  执行中"
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.QueueFailed >= 0 && m.activeDeployment.ExitCode != 0 {
		return "滚动 ↑↓/jk  重试失败 r  重新部署 a  返回 q/Esc"
	}
	if len(m.activeDeployment.Queue) > 0 && m.activeDeployment.Output != "" {
		return "滚动 ↑↓/jk  重新部署 a  返回 q/Esc"
	}
	return "滚动 ↑↓/jk  开始 Enter  重试失败 r  重新部署 a  返回 q/Esc"
}

func (m Model) deploymentQueueConfirmLines(queue []config.DeploymentApp, bodyWidth int) []string {
	current := m.deploymentQueueCurrentApp(queue)
	lines := []string{}
	if len(queue) == 1 {
		lines = append(lines, detailSubTitle("部署信息"))
		lines = append(lines, deploymentInfoLines(current, bodyWidth)...)
	} else {
		lines = append(lines,
			detailSubTitle("部署队列"),
			mutedStyle.Render(fit("按下面顺序串行执行；每个应用完成后按自己的等待时间进入下一个。", bodyWidth)),
			"",
		)
		for i, app := range queue {
			lines = append(lines, deploymentQueueLine(m.activeDeployment, i, app, bodyWidth))
		}
	}
	lines = append(lines, "", detailSubTitle("当前流程"), fit(deploymentQueueFlowText(current), bodyWidth))
	if len(m.activeDeployment.Queue) > 0 {
		lines = append(lines, "", detailSubTitle("执行输出"))
		lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
		if !m.activeDeployment.Running && m.activeDeployment.Output != "" {
			lines = append(lines, "", fmt.Sprintf("退出码 %d", m.activeDeployment.ExitCode))
		}
	}
	if len(queue) == 1 {
		records := m.deploymentRecordsForApp(current, 50)
		lines = append(lines, "", detailSubTitle(fmt.Sprintf("历史 %d条", len(records))))
		if len(records) == 0 {
			lines = append(lines, mutedStyle.Render("暂无记录"))
		} else {
			for _, record := range records {
				lines = append(lines, deploymentDetailHistoryLine(record, bodyWidth))
			}
		}
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func deploymentQueueLine(active activeDeployment, index int, app config.DeploymentApp, width int) string {
	icon := deploymentQueueStatusStyle(active, index).Render(deploymentQueueStatusIcon(active, index))
	seq := cardMutedStyle.Render(fmt.Sprintf("%02d", index+1))
	name := deploymentQueueNameStyle(active, index).Render(padVisible(emptyDash(app.Name), 14))
	server := cardMutedStyle.Render(padVisible(emptyDash(app.Server), 18))
	source := detailValueStyle.Render(padVisible(deploySourceText(app.Source), 7))
	target := cardMutedStyle.Render(padVisible(deploymentAppTarget(app), 10))
	wait := cardMutedStyle.Render(fmt.Sprintf("等待 %d秒", maxInt(0, app.WaitSeconds)))
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

func deploymentInfoLines(app config.DeploymentApp, bodyWidth int) []string {
	return []string{
		deploymentDetailRow("应用", emptyDash(app.Name), bodyWidth),
		deploymentDetailRow("服务器", deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow("来源", deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow("仓库", emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow("目录", emptyDash(app.Path), bodyWidth),
		deploymentDetailRow("凭证", deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow("凭证参数", emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow("收藏", yesNo(app.Favorite), bodyWidth),
		deploymentDetailRow("置顶", yesNo(app.Pinned), bodyWidth),
	}
}

func (m Model) deploymentQueueCurrentApp(queue []config.DeploymentApp) config.DeploymentApp {
	if len(m.activeDeployment.Queue) > 0 {
		index := clampInt(m.activeDeployment.QueueIndex, 0, len(m.activeDeployment.Queue)-1)
		return m.activeDeployment.Queue[index]
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

func deploymentQueueFlowText(app config.DeploymentApp) string {
	parts := []string{}
	if len(app.BeforeCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新前 %d步", len(app.BeforeCommands)))
	}
	parts = append(parts, fmt.Sprintf("获取资源 %d步", len(app.ResourceCommands)))
	if len(app.UpdateCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新 %d步", len(app.UpdateCommands)))
	}
	if len(app.AfterCommands) > 0 {
		parts = append(parts, fmt.Sprintf("更新后 %d步", len(app.AfterCommands)))
	}
	if len(app.HealthCommands) > 0 {
		parts = append(parts, fmt.Sprintf("健康检查 %d步", len(app.HealthCommands)))
	}
	return strings.Join(parts, "  ")
}

func deploymentHistoryLine(record config.DeploymentRecord, width int) string {
	version := deploymentRecordVersionText(record)
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = fmt.Sprintf("  退出码 %d", record.ExitCode)
	}
	line := fmt.Sprintf("%s  %s  %s%s",
		padVisible(deploymentRecordDateTimeText(record.Time), 11),
		padVisible(deploymentRecordActionStatusText(record), 8),
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
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentRollbackConfirmMaxScroll())
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
	return strings.Join([]string{titleStyle.Render(fit("确认回滚", width)), box, renderHelp(width, "滚动 ↑↓/jk  执行 Enter  返回 Esc")}, "\n")
}

func (m Model) deploymentRollbackConfirmLines(bodyWidth int) []string {
	app := m.activeDeployment.App
	lines := []string{
		modalLine("服务器", m.activeDeploymentServerName(), bodyWidth),
		modalLine("应用", app.Name, bodyWidth),
		modalLine("上一版本", emptyDash(m.activeDeployment.PreviousVersion), bodyWidth),
		modalLine("当前版本", emptyDash(m.activeDeployment.CurrentVersion), bodyWidth),
		"",
		detailSubTitle("回滚命令"),
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
	if m.activeDeployment.HostIndex >= 0 && m.activeDeployment.HostIndex < len(m.states) {
		return hostDisplayName(m.states[m.activeDeployment.HostIndex].Host)
	}
	return emptyDash(m.activeDeployment.App.Server)
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
	index := m.deploymentServerIndex(m.deploymentConfirm.Server)
	if index >= 0 && index < len(m.states) {
		return hostDisplayName(m.states[index].Host)
	}
	return emptyDash(m.deploymentConfirm.Server)
}

func (m Model) renderDeploymentOutput() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "滚动 ↑↓/jk  回滚 r  返回 q/Esc"
	title := "部署输出  " + m.activeDeployment.App.Name
	lines := []string{
		modalLine("应用", m.activeDeployment.App.Name, bodyWidth),
		modalLine("来源", deploySourceText(m.activeDeployment.App.Source), bodyWidth),
		modalLine("队列", deploymentQueueProgressText(m.activeDeployment), bodyWidth),
		modalLine("上一版本", emptyDash(m.activeDeployment.PreviousVersion), bodyWidth),
		modalLine("当前版本", emptyDash(m.activeDeployment.CurrentVersion), bodyWidth),
		"",
	}
	lines = append(lines, m.deploymentOutputContentLines(bodyWidth)...)
	if !m.activeDeployment.Running {
		lines = append(lines, "", fmt.Sprintf("退出码 %d", m.activeDeployment.ExitCode))
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentOutputMaxScroll())
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
	if !m.activeDeployment.Running {
		lines += 2
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, lines-height)
}

func (m Model) deploymentOutputContentLines(width int) []string {
	stages := deploymentExecutionStages(m.activeDeployment.App, m.activeDeployment.Action)
	output := strings.TrimRight(m.activeDeployment.Output, "\n")
	sections, loose, lastStage := deploymentOutputSections(output)
	if len(stages) == 0 {
		if output == "" {
			return []string{mutedStyle.Render("(无输出)")}
		}
		return deploymentOutputLines(output, width)
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
		lines = append(lines, detailSubTitle("输出"))
		for _, line := range loose {
			lines = append(lines, fit(line, width))
		}
		lines = append(lines, "")
	}
	for i, stage := range stages {
		status := "pending"
		if m.activeDeployment.Running {
			if i < currentIndex {
				status = "done"
			} else if i == currentIndex {
				status = "running"
			}
		} else {
			if m.activeDeployment.ExitCode != 0 {
				if i < currentIndex {
					status = "done"
				} else if i == currentIndex {
					status = "failed"
				}
			} else {
				status = "done"
			}
		}
		lines = append(lines, deploymentOutputStageLine(stage, status, width))
		stageLines := sections[stage]
		if len(stageLines) == 0 && status == "running" {
			lines = append(lines, mutedStyle.Render("  正在执行..."))
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

func deploymentOutputLines(output string, width int) []string {
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
			lines = append(lines, detailSubTitle(title))
			continue
		}
		lines = append(lines, fit(line, width))
	}
	return lines
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
