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

func (m Model) renderDeploymentList() string {
	pageWidth := m.width
	if pageWidth <= 0 {
		pageWidth = contentWidth(m.width)
	}
	if pageWidth < 34 {
		pageWidth = 34
	}
	frameWidth := detailFrameWidth(pageWidth)
	bodyWidth := frameWidth - 4
	if bodyWidth < 32 {
		bodyWidth = 32
	}
	help := "移动 ↑↓←→/hjkl  详情 Space  选择 s  分类 Tab  视图 z  置顶 t  收藏 f  只看收藏 v  部署 Enter  新增 a  编辑 e  删除 x  返回 Esc"
	header := titleStyle.Render(fit(strings.Join(m.deploymentHeaderParts(), "  "), pageWidth))
	if m.deploymentView == deploymentViewCards {
		bodyHeight := deploymentCardsHeight(m.height, false)
		pageDots := m.deploymentPageDots(pageWidth, bodyHeight)
		if pageDots != "" {
			bodyHeight = deploymentCardsHeight(m.height, true)
			pageDots = m.deploymentPageDots(pageWidth, bodyHeight)
		}
		lines := []string{header, "", m.renderDeploymentCards(pageWidth, bodyHeight)}
		if pageDots != "" {
			lines = append(lines, pageDots)
		}
		lines = append(lines, renderHelp(pageWidth, help))
		return strings.Join(lines, "\n")
	}
	contentHeight := m.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}
	lines := []string{}
	if len(m.deploymentItems) == 0 {
		lines = append(lines, mutedStyle.Render("没有部署应用。按 a 添加。"))
	} else {
		start, end := visibleRange(len(m.deploymentItems), m.deploymentIndex, contentHeight)
		for i := start; i < end; i++ {
			item := m.deploymentItems[i]
			lines = append(lines, m.deploymentAppListLine(item, bodyWidth, i == m.deploymentIndex))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(frameWidth).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{header, box, renderHelp(pageWidth, help)}, "\n")
}

func (m Model) deploymentHeaderParts() []string {
	parts := []string{"应用部署", fmt.Sprintf("应用 %d", len(m.deploymentItems)), "视图：" + deploymentViewName(m.deploymentView)}
	if m.deploymentCategory != "" {
		parts = append(parts, "分类："+m.deploymentCategory)
	}
	if len(m.deploymentSelected) > 0 {
		parts = append(parts, fmt.Sprintf("已选 %d", len(m.deploymentSelected)))
	}
	if m.deploymentFavoriteOnly {
		parts = append(parts, "只看收藏")
	}
	if m.status != "" && m.status != "应用部署" && !strings.HasPrefix(m.status, "部署视图：") {
		parts = append(parts, m.status)
	}
	return parts
}

func deploymentViewName(view deploymentViewMode) string {
	if view == deploymentViewList {
		return "列表"
	}
	return "卡片"
}

func deploymentCardsHeight(totalHeight int, withDots bool) int {
	bodyHeight := totalHeight - 3
	if withDots {
		bodyHeight--
	}
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	return bodyHeight
}

func (m Model) renderDeploymentCards(width int, bodyHeight int) string {
	lines := []string{}
	if len(m.deploymentItems) == 0 {
		lines = append(lines, mutedStyle.Render("没有部署应用。按 a 添加。"))
		for len(lines) < bodyHeight {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	cardLines, selectedTop, selectedBottom := m.deploymentCardLines(width)
	cardHeight := bodyHeight
	if cardHeight < 1 {
		cardHeight = 1
	}
	start, end := dashboardLineWindow(len(cardLines), selectedTop, selectedBottom, cardHeight)
	lines = append(lines, cardLines[start:end]...)
	for len(lines) < bodyHeight {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) deploymentPageDots(width int, bodyHeight int) string {
	if len(m.deploymentItems) == 0 || bodyHeight <= 0 {
		return ""
	}
	lines, selectedTop, selectedBottom := m.deploymentCardLines(width)
	return dashboardLineDots(len(lines), selectedTop, selectedBottom, bodyHeight, width)
}

func (m Model) deploymentCardLines(width int) ([]string, int, int) {
	cols := m.dashboardColumns()
	cardWidths := distributeWidths(width, cols)
	lines := []string{}
	selectedTop := 0
	selectedBottom := 0
	for i := 0; i < len(m.deploymentItems); i += cols {
		rowEnd := minInt(i+cols, len(m.deploymentItems))
		row := []string{}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= rowEnd {
				row = append(row, padBlock(blankDeploymentCard(cardWidth), cardWidth))
				continue
			}
			itemIndex := i + col
			if itemIndex == m.deploymentIndex {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.renderDeploymentAppCard(m.deploymentItems[itemIndex], cardWidth, itemIndex == m.deploymentIndex), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.deploymentIndex >= i && m.deploymentIndex < rowEnd {
			selectedBottom = len(lines)
		}
	}
	if selectedBottom == 0 {
		selectedBottom = selectedTop
	}
	return lines, selectedTop, selectedBottom
}

func deploymentCardColumns(width int) int {
	switch {
	case width >= 148:
		return 3
	case width >= 96:
		return 2
	default:
		return 1
	}
}

func blankDeploymentCard(width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.HiddenBorder()).
		Padding(0, 1).
		Width(maxInt(30, width-4)).
		Height(deploymentCardInnerHeight).
		Render("")
}

func (m Model) renderDeploymentAppCard(item deploymentItem, width int, selected bool) string {
	app := item.App
	cardWidth := maxInt(34, width)
	innerWidth := cardWidth - 4
	borderStyle := cardBorderStyle
	if selected {
		borderStyle = selectedCardBorderStyle
	}
	order := m.deploymentSelectionOrder(item.Index)
	mark := ""
	if order > 0 {
		mark = blueStyle.Bold(true).Render(fmt.Sprintf("%02d ", order))
	}
	title := deploymentAppMarks(app) + mark + detailValueStyle.Render(emptyDash(app.Name))
	meta := m.deploymentLastRecordMeta(app)
	dot := m.deploymentLastRecordDot(app)
	sourceLine := deploymentCardSourceLine(app, innerWidth)
	serverLine := deploymentCardServerLine(app.Server, innerWidth)
	pathLine := cardMutedStyle.Render("目录 ") + detailValueStyle.Render(emptyDash(app.Path))
	fetchLine := cardMutedStyle.Render("方式 ") + detailValueStyle.Render(deployFetchModeText(app.FetchMode))
	timeLine := m.deploymentLastRecordTimeLine(app, innerWidth)
	lines := []string{
		cardTopLine(cardWidth, title, meta, dot, borderStyle),
		cardContentLine(cardWidth, serverLine, borderStyle),
		cardContentLine(cardWidth, sourceLine, borderStyle),
		cardContentLine(cardWidth, pathLine, borderStyle),
		cardContentLine(cardWidth, fetchLine, borderStyle),
		cardInnerSeparatorLine(cardWidth, borderStyle),
		cardContentLine(cardWidth, timeLine, borderStyle),
		cardBottomLine(cardWidth, borderStyle),
	}
	return strings.Join(lines, "\n")
}

func deploymentCardSourceLine(app config.DeploymentApp, width int) string {
	left := cardMutedStyle.Render(deploySourceText(app.Source)+" ") + detailValueStyle.Render(deploymentAppTarget(app))
	right := cardMutedStyle.Render(deployCredentialText(app.Credential))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 2 {
		maxLeft := width - ansi.StringWidth(right) - 2
		if maxLeft < 8 {
			return fitANSI(left, width)
		}
		left = fitANSI(left, maxLeft)
		gap = width - ansi.StringWidth(left) - ansi.StringWidth(right)
	}
	return left + strings.Repeat(" ", maxInt(1, gap)) + right
}

func deploymentCardServerLine(server string, width int) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return transferPathLine("→", "-")
	}
	category := ""
	name := server
	if idx := strings.Index(server, "/"); idx >= 0 {
		category = strings.TrimSpace(server[:idx])
		name = strings.TrimSpace(server[idx+1:])
	}
	if name == "" {
		name = server
	}
	line := blueStyle.Render("→ ")
	if category != "" {
		line += cardMutedStyle.Render("["+category+"]") + " "
	}
	line += detailValueStyle.Render(name)
	return fitANSI(line, width)
}

func (m Model) deploymentLastRecordMeta(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("未部署")
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(deploymentRecordActionStatusText(record))
}

func (m Model) deploymentLastRecordDot(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("●")
	}
	if record.Status == config.DeployStatusFailed {
		return redStyle.Render("●")
	}
	return greenStyle.Render("●")
}

func (m Model) deploymentLastRecordTimeLine(app config.DeploymentApp, width int) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("时间 暂无记录")
	}
	return cardMutedStyle.Render("时间 ") + detailValueStyle.Render(fit(deploymentRecordTimeText(record.Time), width-5))
}

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
	return strings.Join([]string{titleStyle.Render(fit("部署详情", width)), box, renderHelp(width, "滚动 ↑↓/jk  部署 Enter  编辑 e  返回 Esc")}, "\n")
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
		detailSubTitle("基础"),
		deploymentDetailRow("应用", emptyDash(app.Name), bodyWidth),
		deploymentDetailRow("服务器", deploymentDisplayServerText(app.Server), bodyWidth),
		deploymentDetailRow("来源", deploySourceText(app.Source), bodyWidth),
		deploymentDetailRow("获取方式", deployFetchModeText(app.FetchMode), bodyWidth),
		deploymentDetailRow("仓库", emptyDash(app.Repo), bodyWidth),
		deploymentDetailRow("目标", deploymentAppTarget(app), bodyWidth),
		deploymentDetailRow("资源匹配", emptyDash(app.Asset), bodyWidth),
		deploymentDetailRow("下载地址", emptyDash(app.ReleaseURL), bodyWidth),
		deploymentDetailRow("目录", emptyDash(app.Path), bodyWidth),
		deploymentDetailRow("凭证", deployCredentialText(app.Credential), bodyWidth),
		deploymentDetailRow("凭证参数", emptyDash(app.CredentialName), bodyWidth),
		deploymentDetailRow("等待", fmt.Sprintf("%d秒", maxInt(0, app.WaitSeconds)), bodyWidth),
		deploymentDetailRow("收藏", yesNo(app.Favorite), bodyWidth),
		deploymentDetailRow("置顶", yesNo(app.Pinned), bodyWidth),
		"",
		detailSubTitle("流程代码"),
	}
	records := m.deploymentRecordsForApp(app, 50)
	lines = appendDeploymentDetailCommands(lines, "更新前", app.BeforeCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "获取资源", app.ResourceCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "更新", app.UpdateCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "更新后", app.AfterCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "健康检查", app.HealthCommands, bodyWidth)
	lines = appendDeploymentDetailCommands(lines, "回滚", app.RollbackCommands, bodyWidth)
	lines = append(lines, "", detailSubTitle(fmt.Sprintf("历史 %d条", len(records))))
	if len(records) == 0 {
		lines = append(lines, mutedStyle.Render("暂无记录"))
		return lines
	}
	for _, record := range records {
		lines = append(lines, deploymentDetailHistoryLine(record, bodyWidth))
	}
	return lines
}

func appendDeploymentDetailCommands(lines []string, title string, commands []string, width int) []string {
	lines = append(lines, deploymentDetailStageLine(title, len(commands), width))
	if len(commands) == 0 {
		lines = append(lines, mutedStyle.Render("    未配置"))
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
	countText := cardMutedStyle.Render(fmt.Sprintf("%d步", count))
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

func deploymentDetailHistoryLine(record config.DeploymentRecord, width int) string {
	statusText := deploymentRecordActionStatusText(record)
	statusStyle := greenStyle
	if record.Status == config.DeployStatusFailed {
		statusStyle = redStyle
	}
	version := cardMutedStyle.Render(deploymentRecordVersionText(record))
	exit := ""
	if record.Status == config.DeployStatusFailed && record.ExitCode != 0 {
		exit = "  " + redStyle.Render(fmt.Sprintf("退出码 %d", record.ExitCode))
	}
	line := cardMutedStyle.Render(padVisible(deploymentRecordDateTimeText(record.Time), 11)) +
		"  " + statusStyle.Render(padVisible(statusText, 8)) +
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
	credential := cardMutedStyle.Render(padVisible(deployCredentialText(app.Credential), 8))
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
		return "未部署"
	}
	return deploymentRecordActionStatusText(record) + "  " + deploymentRecordTimeText(record.Time)
}

func (m Model) deploymentLastRecordListText(app config.DeploymentApp) string {
	record, ok := m.latestDeploymentRecord(app)
	if !ok {
		return cardMutedStyle.Render("未部署")
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(deploymentRecordActionStatusText(record)) + "  " + cardMutedStyle.Render(deploymentRecordTimeText(record.Time))
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

func deploymentRecordActionStatusText(record config.DeploymentRecord) string {
	action := "部署"
	if record.Action == config.DeployActionRollback {
		action = "回滚"
	}
	status := "成功"
	if record.Status == config.DeployStatusFailed {
		status = "失败"
	}
	if record.Status == config.DeployStatusRunning {
		status = "运行中"
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

func deploymentRecordTimeText(value string) string {
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
		return "刚刚"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d分钟前", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d小时前", int(d.Hours()))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%d天前", int(d.Hours()/24))
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

func (m Model) renderDeploymentEdit() string {
	width := detailFrameWidth(m.width)
	innerWidth := width - 4
	if innerWidth < 42 {
		innerWidth = 42
	}
	help := "切换 Tab  保存 Enter  换行 Ctrl+J  服务器/来源/凭证 ←→  返回 Esc"
	title := "添加部署应用"
	if m.deploymentEditing {
		title = "编辑部署应用"
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if strings.Contains(m.status, "失败") || strings.Contains(m.status, "不能为空") || strings.Contains(m.status, "需要填写") {
			statusStyle = redStyle
		}
		header += "  " + statusStyle.Render(fit(m.status, width-ansi.StringWidth(title)-2))
	}
	contentHeight := m.height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	lines := m.deploymentEditLines(innerWidth, contentHeight)
	if !deploymentFieldIsCommand(m.deploymentField) && len(lines) > contentHeight {
		selected := selectedDeploymentEditRow(m.deploymentField)
		start := selected - contentHeight + 4
		if start < 0 {
			start = 0
		}
		if start+contentHeight > len(lines) {
			start = len(lines) - contentHeight
			if start < 0 {
				start = 0
			}
		}
		lines = lines[start:minInt(len(lines), start+contentHeight)]
	}
	for blockLineCount(strings.Join(lines, "\n")) < contentHeight {
		lines = append(lines, "")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(blue).
		Padding(0, 1).
		Width(width).
		Render(strings.Join(lines, "\n"))
	return strings.Join([]string{fit(header, width), box, renderHelp(width, help)}, "\n")
}

func (m Model) deploymentEditLines(innerWidth int, contentHeight int) []string {
	if deploymentFieldIsCommand(m.deploymentField) {
		lines := []string{
			deploymentSectionTitle("部署流程"),
			deploymentCommandSummaryLine(m, 13, "更新前", m.deploymentForm.BeforeCommands, innerWidth),
			deploymentCommandSummaryLine(m, 14, "获取资源", m.deploymentForm.ResourceCommands, innerWidth),
			deploymentCommandSummaryLine(m, 15, "更新命令", m.deploymentForm.UpdateCommands, innerWidth),
			deploymentCommandSummaryLine(m, 16, "更新后", m.deploymentForm.AfterCommands, innerWidth),
			deploymentCommandSummaryLine(m, 17, "健康检查", m.deploymentForm.HealthCommands, innerWidth),
			"",
			deploymentSectionTitle("回滚流程"),
			deploymentCommandSummaryLine(m, 18, "回滚命令", m.deploymentForm.RollbackCommands, innerWidth),
			"",
			deploymentSectionTitle(deploymentFieldName(m.deploymentField)),
		}
		textAreaHeight := contentHeight - len(lines) - 2
		if textAreaHeight < 4 {
			textAreaHeight = 4
		}
		lines = append(lines, commandTextArea(m.deploymentValue(), m.deploymentCursor, true, innerWidth, textAreaHeight))
		return lines
	}
	lines := []string{
		deploymentSectionTitle("资源来源"),
		deploymentFieldLine(m, 0, "来源", deploySourceText(m.deploymentForm.Source)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 1, "获取方式", deployFetchModeText(m.deploymentForm.FetchMode)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 2, "服务器", m.deploymentServerText(innerWidth), innerWidth),
		deploymentFieldLine(m, 3, "应用名称", m.deploymentInputText(3, deploymentInputWidth()), innerWidth),
		deploymentFieldLine(m, 4, "仓库", m.deploymentInputText(4, deploymentInputWidth()), innerWidth),
	}
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines,
			deploymentFieldLine(m, 6, "版本", m.deploymentInputText(6, deploymentInputWidth()), innerWidth),
			deploymentFieldLine(m, 7, "资源文件/匹配", m.deploymentInputText(7, deploymentInputWidth()), innerWidth),
		)
	} else {
		lines = append(lines, deploymentFieldLine(m, 5, "分支", m.deploymentInputText(5, deploymentInputWidth()), innerWidth))
	}
	lines = append(lines, deploymentFieldLine(m, 8, "项目目录", m.deploymentInputText(8, deploymentInputWidth()), innerWidth))
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines, deploymentFieldLine(m, 9, "下载地址", m.deploymentInputText(9, deploymentInputWidth()), innerWidth))
		lines = append(lines, deploymentReleaseHintLines(innerWidth)...)
	}
	lines = append(lines,
		"",
		deploymentSectionTitle("GitHub 凭证"),
		deploymentFieldLine(m, 10, "凭证类型", deployCredentialText(m.deploymentForm.Credential)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 11, "凭证参数", m.deploymentInputText(11, deploymentInputWidth()), innerWidth),
		"",
		deploymentSectionTitle("串行部署"),
		deploymentFieldLine(m, 12, "等待时间", m.deploymentInputText(12, deploymentInputWidth())+"  秒", innerWidth),
		mutedStyle.Render(fit("说明：多选部署时，此应用完成后等待该秒数再执行下一个。", innerWidth)),
		"",
		deploymentSectionTitle("部署流程"),
		deploymentCommandSummaryLine(m, 13, "更新前", m.deploymentForm.BeforeCommands, innerWidth),
		deploymentCommandSummaryLine(m, 14, "获取资源", m.deploymentForm.ResourceCommands, innerWidth),
		deploymentCommandSummaryLine(m, 15, "更新命令", m.deploymentForm.UpdateCommands, innerWidth),
		deploymentCommandSummaryLine(m, 16, "更新后", m.deploymentForm.AfterCommands, innerWidth),
		deploymentCommandSummaryLine(m, 17, "健康检查", m.deploymentForm.HealthCommands, innerWidth),
		"",
		deploymentSectionTitle("回滚流程"),
		deploymentCommandSummaryLine(m, 18, "回滚命令", m.deploymentForm.RollbackCommands, innerWidth),
	)
	return lines
}

func deploymentSectionTitle(value string) string {
	return "  " + sectionTitle(value)
}

func deploymentInputWidth() int {
	return 34
}

func deploymentReleaseHintLines(width int) []string {
	return fitLines([]string{
		mutedStyle.Render("说明：版本留空或 latest 表示最新 Release；填 v1.0.0 表示固定版本。"),
		mutedStyle.Render("说明：资源不带 * 是固定文件名；带 * 会在 Release 资源里自动匹配。"),
		mutedStyle.Render("说明：下载地址可选；填写后优先使用完整下载地址。"),
	}, width)
}

func (m Model) deploymentServerText(width int) string {
	value := deploymentDisplayServerText(m.deploymentForm.Server)
	index := m.deploymentServerIndex(m.deploymentForm.Server)
	if index >= 0 {
		h := m.states[index].Host
		value = deploymentDisplayServerName(h.Category, h.Name)
	} else if strings.TrimSpace(m.deploymentForm.Server) != "" {
		value += "  未找到"
	}
	value += "  ←/→"
	return fit(value, width)
}

func deploymentDisplayServerText(server string) string {
	server = strings.TrimSpace(server)
	if server == "" {
		return "-"
	}
	category := ""
	name := server
	if idx := strings.Index(server, "/"); idx >= 0 {
		category = strings.TrimSpace(server[:idx])
		name = strings.TrimSpace(server[idx+1:])
	}
	return deploymentDisplayServerName(category, name)
}

func deploymentDisplayServerName(category string, name string) string {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "-"
	}
	if category == "" {
		return name
	}
	return "[" + category + "] " + name
}

func (m Model) deploymentInputText(field int, width int) string {
	value := m.deploymentFieldValue(field)
	if value != "" || m.deploymentField == field {
		return commandInputText(value, m.deploymentCursor, m.deploymentField == field, width)
	}
	placeholder := deploymentFieldPlaceholder(field, m.deploymentForm.Source, m.deploymentForm.Credential)
	if placeholder == "" {
		return commandInputText(value, m.deploymentCursor, m.deploymentField == field, width)
	}
	return "[" + fit(placeholder, width) + strings.Repeat(" ", maxInt(0, width-ansi.StringWidth(placeholder))) + "]"
}

func (m Model) deploymentFieldValue(field int) string {
	m.deploymentField = field
	return m.deploymentValue()
}

func deploymentFieldPlaceholder(field int, source string, credential string) string {
	switch field {
	case 3:
		return "例如 api"
	case 4:
		if source == config.DeploySourceRelease {
			return "owner/repo"
		}
		return "git@github.com:owner/repo.git"
	case 5:
		return "main"
	case 6:
		return "latest 或 v1.0.0"
	case 7:
		return "app.tar.gz 或 app-*"
	case 8:
		return "/opt/app"
	case 9:
		return "可选：完整下载地址"
	case 11:
		switch credential {
		case config.DeployCredentialSSH:
			return "本地或目标服务器私钥路径"
		case config.DeployCredentialToken:
			return "本地或目标服务器环境变量名"
		default:
			return "公开仓库或服务器已配置认证"
		}
	case 12:
		return "0"
	default:
		return ""
	}
}

func selectedDeploymentEditRow(field int) int {
	if field <= 12 {
		return field + 1
	}
	return 19 + field - 13
}

func deploymentFieldLine(m Model, index int, label string, value string, width int) string {
	prefix := " "
	style := lipgloss.NewStyle()
	if m.deploymentField == index {
		prefix = "▶"
		style = blueStyle.Bold(true)
	}
	labelWidth := runewidth.StringWidth("资源文件/匹配")
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func deploymentCommandSummaryLine(m Model, index int, label string, value string, width int) string {
	count := len(splitCommandBlock(value))
	summary := fmt.Sprintf("%d条", count)
	if count == 0 {
		summary = "未配置"
	}
	return deploymentFieldLine(m, index, label, summary, width)
}

func deploymentFieldName(field int) string {
	switch field {
	case 13:
		return "更新前命令"
	case 14:
		return "获取资源命令"
	case 15:
		return "更新命令"
	case 16:
		return "更新后命令"
	case 17:
		return "健康检查命令"
	case 18:
		return "回滚命令"
	default:
		return "命令"
	}
}

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
