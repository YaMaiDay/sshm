package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	help := m.t("Move ↑↓←→/hjkl  Detail Space  Select s  Category Tab  View z  Pin t  Favorite f  Favorites v  Deploy Enter  Add a  Edit e  Delete x  Back Esc", "移动 ↑↓←→/hjkl  详情 Space  选择 s  分类 Tab  视图 z  置顶 t  收藏 f  只看收藏 v  部署 Enter  新增 a  编辑 e  删除 x  返回 Esc")
	header := titleStyle.Render(fit(strings.Join(m.deploymentHeaderParts(), "  "), pageWidth))
	if m.deploymentState.View == deploymentViewCards {
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
	if len(m.deploymentState.Items) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No deployment apps. Press a to add one.", "没有部署应用。按 a 添加。")))
	} else {
		start, end := visibleRange(len(m.deploymentState.Items), m.deploymentState.Index, contentHeight)
		for i := start; i < end; i++ {
			item := m.deploymentState.Items[i]
			lines = append(lines, m.deploymentAppListLine(item, bodyWidth, i == m.deploymentState.Index))
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
	parts := []string{m.t("Deployments", "应用部署"), fmt.Sprintf("%s %d", m.t("Apps", "应用"), len(m.deploymentState.Items)), m.t("View: ", "视图：") + m.deploymentViewName(m.deploymentState.View)}
	if m.deploymentState.Category != "" {
		parts = append(parts, m.t("Category: ", "分类：")+m.deploymentState.Category)
	}
	if len(m.deploymentState.Selected) > 0 {
		parts = append(parts, fmt.Sprintf("%s %d", m.t("Selected", "已选"), len(m.deploymentState.Selected)))
	}
	if m.deploymentState.FavoriteOnly {
		parts = append(parts, m.t("Favorites only", "只看收藏"))
	}
	if m.status != "" && m.status != m.t("Deployments", "应用部署") && !strings.HasPrefix(m.status, "部署视图：") {
		parts = append(parts, m.status)
	}
	return parts
}

func (m Model) deploymentViewName(view deploymentViewMode) string {
	if view == deploymentViewList {
		return m.t("List", "列表")
	}
	return m.t("Cards", "卡片")
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
	if len(m.deploymentState.Items) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No deployment apps. Press a to add one.", "没有部署应用。按 a 添加。")))
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
	if len(m.deploymentState.Items) == 0 || bodyHeight <= 0 {
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
	for i := 0; i < len(m.deploymentState.Items); i += cols {
		rowEnd := minInt(i+cols, len(m.deploymentState.Items))
		row := []string{}
		for col := 0; col < cols; col++ {
			cardWidth := cardWidths[col]
			if i+col >= rowEnd {
				row = append(row, padBlock(blankDeploymentCard(cardWidth), cardWidth))
				continue
			}
			itemIndex := i + col
			if itemIndex == m.deploymentState.Index {
				selectedTop = len(lines)
			}
			row = append(row, padBlock(m.renderDeploymentAppCard(m.deploymentState.Items[itemIndex], cardWidth, itemIndex == m.deploymentState.Index), cardWidth))
		}
		rowLines := strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, row...), "\n")
		lines = append(lines, rowLines...)
		if m.deploymentState.Index >= i && m.deploymentState.Index < rowEnd {
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
	sourceLine := m.deploymentCardSourceLine(app, innerWidth)
	serverLine := deploymentCardServerLine(app.Server, innerWidth)
	pathLine := cardMutedStyle.Render(m.t("Path ", "目录 ")) + detailValueStyle.Render(emptyDash(app.Path))
	fetchLine := cardMutedStyle.Render(m.t("Mode ", "方式 ")) + detailValueStyle.Render(m.deployFetchModeText(app.FetchMode))
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

func (m Model) deploymentCardSourceLine(app config.DeploymentApp, width int) string {
	left := cardMutedStyle.Render(deploySourceText(app.Source)+" ") + detailValueStyle.Render(deploymentAppTarget(app))
	right := cardMutedStyle.Render(m.deployCredentialText(app.Credential))
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
		return cardMutedStyle.Render(m.t("Not deployed", "未部署"))
	}
	style := greenStyle
	if record.Status == config.DeployStatusFailed {
		style = redStyle
	}
	return style.Render(m.deploymentRecordActionStatusText(record))
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
		return cardMutedStyle.Render(m.t("Time no records", "时间 暂无记录"))
	}
	return cardMutedStyle.Render(m.t("Time ", "时间 ")) + detailValueStyle.Render(fit(m.deploymentRecordTimeText(record.Time), width-5))
}
