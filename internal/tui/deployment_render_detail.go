package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) renderDeploymentDetail() string {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	lines := m.deploymentDetailLines(bodyWidth)
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	scroll := clampInt(m.deploymentOutputScroll, 0, m.deploymentDetailMaxScroll())
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
	return strings.Join([]string{titleStyle.Render(fit(m.t("Deployment Detail", "部署详情"), width)), box, renderHelp(width, m.t("Scroll ↑↓/jk  Deploy Enter  Edit e  Back Esc", "滚动 ↑↓/jk  部署 Enter  编辑 e  返回 Esc"))}, "\n")
}

func (m Model) deploymentDetailMaxScroll() int {
	width := detailFrameWidth(m.width)
	bodyWidth := width - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	height := m.height - 4
	if height < 8 {
		height = 8
	}
	return maxInt(0, len(m.deploymentDetailLines(bodyWidth))-height)
}

func (m Model) deploymentDetailLines(bodyWidth int) []string {
	app := deploymentAppWithResourceDefaults(m.deploymentDetail)
	lines := []string{
		detailSubTitle(m.t("Basic", "基础")),
		deploymentDetailRow(m.t("App", "应用"), emptyDash(app.Name), bodyWidth),
		deploymentDetailRow(m.t("Server", "服务器"), deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow(m.t("Source", "来源"), deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow(m.t("Fetch mode", "获取方式"), m.deployFetchModeText(app.FetchMode), bodyWidth),
		deploymentDetailRow(m.t("Repo", "仓库"), emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow(m.t("Target", "目标"), deploymentAppTarget(app), bodyWidth),
		deploymentDetailRow(m.t("Asset match", "资源匹配"), emptyDash(app.Asset), bodyWidth),
		deploymentDetailRow(m.t("Download URL", "下载地址"), emptyDash(app.ReleaseURL), bodyWidth),
		deploymentDetailRow(m.t("Path", "目录"), emptyDash(app.Path), bodyWidth),
		deploymentDetailRow(m.t("Credential", "凭证"), m.deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow(m.t("Credential param", "凭证参数"), emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow(m.t("Wait", "等待"), fmt.Sprintf("%d%s", maxInt(0, app.WaitSeconds), m.t("s", "秒")), bodyWidth),
		deploymentDetailRow(m.t("Favorite", "收藏"), yesNoLang(app.Favorite, m.isChineseUI()), bodyWidth),
		deploymentDetailRow(m.t("Pinned", "置顶"), yesNoLang(app.Pinned, m.isChineseUI()), bodyWidth),
		"",
		detailSubTitle(m.t("Flow commands", "流程代码")),
	}
	records := m.deploymentRecordsForApp(app, 50)
	lines = m.appendDeploymentDetailCommands(lines, m.t("Before update", "更新前"), app.BeforeCommands, bodyWidth)
	lines = m.appendDeploymentDetailCommands(lines, m.t("Fetch resource", "获取资源"), app.ResourceCommands, bodyWidth)
	lines = m.appendDeploymentDetailCommands(lines, m.t("Update", "更新"), app.UpdateCommands, bodyWidth)
	lines = m.appendDeploymentDetailCommands(lines, m.t("After update", "更新后"), app.AfterCommands, bodyWidth)
	lines = m.appendDeploymentDetailCommands(lines, m.t("Health check", "健康检查"), app.HealthCommands, bodyWidth)
	lines = m.appendDeploymentDetailCommands(lines, m.t("Rollback", "回滚"), app.RollbackCommands, bodyWidth)
	lines = append(lines, "", detailSubTitle(fmt.Sprintf("%s %d", m.t("History", "历史"), len(records))))
	if len(records) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No records", "暂无记录")))
		return lines
	}
	for _, record := range records {
		lines = append(lines, m.deploymentDetailHistoryLine(record, bodyWidth))
	}
	return lines
}

func (m Model) appendDeploymentDetailCommands(lines []string, title string, commands []string, width int) []string {
	lines = append(lines, deploymentDetailStageLine(title, len(commands), width))
	if len(commands) == 0 {
		lines = append(lines, mutedStyle.Render("    "+m.t("Not configured", "未配置")))
		return append(lines, "")
	}
	for _, command := range commands {
		lines = appendWrappedCommandIndented(lines, command, width)
	}
	return append(lines, "")
}

func deploymentDetailRow(label string, value string, width int) string {
	labelWidth := 10
	padding := labelWidth - runewidth.StringWidth(label)
	if padding < 1 {
		padding = 1
	}
	prefix := detailLabelStyle.Render(label) + strings.Repeat(" ", padding)
	return fitANSI(prefix+detailValueStyle.Render(value), width)
}

func deploymentDetailStageLine(title string, count int, width int) string {
	label := detailLabelStyle.Render(padVisible(title, 10))
	countText := cardMutedStyle.Render(fmt.Sprintf("%d", count))
	return fitANSI(label+countText, width)
}

func appendWrappedCommandIndented(lines []string, command string, width int) []string {
	prefix := cardMutedStyle.Render("    $ ")
	wrapped := strings.Split(wrapPlainLine(command, width-ansi.StringWidth(prefix)), "\n")
	for _, line := range wrapped {
		lines = append(lines, prefix+detailValueStyle.Render(line))
	}
	return lines
}

func (m Model) deploymentDetailHistoryLine(record config.DeploymentRecord, width int) string {
	statusText := m.deploymentRecordActionStatusText(record)
	statusStyle := greenStyle
	if record.Status == config.DeployStatusFailed {
		statusStyle = redStyle
	}
	version := cardMutedStyle.Render(deploymentRecordVersionText(record))
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = "  " + redStyle.Render(fmt.Sprintf("%s %d", m.t("exit", "退出码"), record.ExitCode))
	}
	line := cardMutedStyle.Render(padVisible(deploymentRecordDateTimeText(record.Time), 11)) +
		"  " + statusStyle.Render(padVisible(statusText, 14)) +
		"  " + version + exit
	return fitANSI(line, width)
}

func (m Model) deploymentAppListLine(item deploymentItem, width int, selected bool) string {
	app := item.App
	prefix := cardMutedStyle.Render(" ")
	nameStyle := detailValueStyle
	if selected {
		prefix = blueStyle.Bold(true).Render("▶")
		nameStyle = blueStyle.Bold(true)
	}
	mark := "  "
	if order := m.deploymentSelectionOrder(item.Index); order > 0 {
		mark = blueStyle.Bold(true).Render(fmt.Sprintf("%02d", order))
	} else {
		mark = cardMutedStyle.Render(mark)
	}
	flags := deploymentAppListMarks(app)
	name := nameStyle.Render(padVisible(emptyDash(app.Name), 14))
	server := cardMutedStyle.Render(padVisible(deploymentDisplayServerText(app.Server), 18))
	source := detailValueStyle.Render(padVisible(deploySourceText(app.Source), 7))
	target := cardMutedStyle.Render(padVisible(deploymentAppTarget(app), 10))
	credential := cardMutedStyle.Render(padVisible(m.deployCredentialText(app.Credential), 8))
	record := m.deploymentLastRecordListText(app)
	return fitANSI(strings.Join([]string{prefix, mark, flags, name, server, source, target, credential, record}, "  "), width)
}

func deploymentAppMarks(app config.DeploymentApp) string {
	marks := ""
	if app.Pinned {
		marks += pinnedStyle.Render("▲") + " "
	}
	if app.Favorite {
		marks += favoriteStyle.Render("★") + " "
	}
	return marks
}

func deploymentAppListMarks(app config.DeploymentApp) string {
	marks := ""
	if app.Pinned {
		marks += pinnedStyle.Render("▲")
	} else {
		marks += " "
	}
	marks += " "
	if app.Favorite {
		marks += favoriteStyle.Render("★")
	} else {
		marks += " "
	}
	return padVisible(marks, 3)
}

func (m Model) deploymentLastRecordText(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return m.t("Not deployed", "未部署")
	}
	return m.deploymentRecordActionStatusText(record) + "  " + m.deploymentRecordTimeText(record.Time)
}

func (m Model) deploymentLastRecordListText(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render(m.t("Not deployed", "未部署"))
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(m.deploymentRecordActionStatusText(record)) + "  " + cardMutedStyle.Render(m.deploymentRecordTimeText(record.Time))
}

func (m Model) latestDeploymentRecord(app config.DeploymentApp) (config.DeploymentRecord, bool) {
	records := m.deploymentRecordsForApp(app, 1)
	if len(records) == 0 {
		return config.DeploymentRecord{}, false
	}
	return records[0], true
}

func (m Model) deploymentRecordsForApp(app config.DeploymentApp, limit int) []config.DeploymentRecord {
	category, name := splitDeploymentServer(app.Server)
	records := []config.DeploymentRecord{}
	for _, record := range m.deploymentFile.Records {
		if record.App == app.Name && record.ServerCategory == category && record.ServerName == name {
			records = append(records, record)
			if limit > 0 && len(records) >= limit {
				break
			}
		}
	}
	return records
}

func (m Model) deploymentRecordActionStatusText(record config.DeploymentRecord) string {
	action := m.t("Deploy", "部署")
	if record.Action == config.DeployActionRollback {
		action = m.t("Rollback", "回滚")
	}
	status := m.t(" success", "成功")
	if record.Status == config.DeployStatusFailed {
		status = m.t(" failed", "失败")
	}
	if record.Status == config.DeployStatusRunning {
		status = m.t(" running", "运行中")
	}
	return action + status
}

func splitDeploymentServer(server string) (string, string) {
	server = strings.TrimSpace(server)
	if idx := strings.Index(server, "/"); idx >= 0 {
		return server[:idx], server[idx+1:]
	}
	return "", server
}

func (m Model) deploymentRecordTimeText(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	now := time.Now()
	if t.After(now) {
		t = now
	}
	d := now.Sub(t)
	if d < time.Minute {
		return m.t("just now", "刚刚")
	}
	if d < time.Hour {
		return fmt.Sprintf("%d%s", int(d.Minutes()), m.t("m ago", "分钟前"))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d%s", int(d.Hours()), m.t("h ago", "小时前"))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%d%s", int(d.Hours()/24), m.t("d ago", "天前"))
	}
	if t.Year() == now.Year() {
		return t.Format("01-02")
	}
	return t.Format("2006-01-02")
}

func deploymentRecordDateTimeText(value string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return "-"
	}
	return t.Local().Format("01-02 15:04")
}
