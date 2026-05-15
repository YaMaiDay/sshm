package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/YaMaiDay/sshm/internal/config"
)

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
