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
	help := m.t("Switch Tab  Save Enter  Newline Ctrl+J  Server/source/credential ←→  Back Esc", "切换 Tab  保存 Enter  换行 Ctrl+J  服务器/来源/凭证 ←→  返回 Esc")
	title := m.t("Add Deployment App", "添加部署应用")
	if m.deploymentEditing {
		title = m.t("Edit Deployment App", "编辑部署应用")
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
			deploymentSectionTitle(m.t("Deploy Flow", "部署流程")),
			deploymentCommandSummaryLine(m, 13, m.t("Before", "更新前"), m.deploymentForm.BeforeCommands, innerWidth),
			deploymentCommandSummaryLine(m, 14, m.t("Fetch", "获取资源"), m.deploymentForm.ResourceCommands, innerWidth),
			deploymentCommandSummaryLine(m, 15, m.t("Update", "更新命令"), m.deploymentForm.UpdateCommands, innerWidth),
			deploymentCommandSummaryLine(m, 16, m.t("After", "更新后"), m.deploymentForm.AfterCommands, innerWidth),
			deploymentCommandSummaryLine(m, 17, m.t("Health", "健康检查"), m.deploymentForm.HealthCommands, innerWidth),
			"",
			deploymentSectionTitle(m.t("Rollback Flow", "回滚流程")),
			deploymentCommandSummaryLine(m, 18, m.t("Rollback", "回滚命令"), m.deploymentForm.RollbackCommands, innerWidth),
			"",
			deploymentSectionTitle(m.deploymentFieldName(m.deploymentField)),
		}
		textAreaHeight := contentHeight - len(lines) - 2
		if textAreaHeight < 4 {
			textAreaHeight = 4
		}
		lines = append(lines, commandTextArea(m.deploymentValue(), m.deploymentCursor, true, innerWidth, textAreaHeight))
		return lines
	}
	lines := []string{
		deploymentSectionTitle(m.t("Resource Source", "资源来源")),
		deploymentFieldLine(m, 0, m.t("Source", "来源"), deploySourceText(m.deploymentForm.Source)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 1, m.t("Fetch mode", "获取方式"), m.deployFetchModeText(m.deploymentForm.FetchMode)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 2, m.t("Server", "服务器"), m.deploymentServerText(innerWidth), innerWidth),
		deploymentFieldLine(m, 3, m.t("App name", "应用名称"), m.deploymentInputText(3, deploymentInputWidth()), innerWidth),
		deploymentFieldLine(m, 4, m.t("Repo", "仓库"), m.deploymentInputText(4, deploymentInputWidth()), innerWidth),
	}
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines,
			deploymentFieldLine(m, 6, m.t("Version", "版本"), m.deploymentInputText(6, deploymentInputWidth()), innerWidth),
			deploymentFieldLine(m, 7, m.t("Asset/match", "资源文件/匹配"), m.deploymentInputText(7, deploymentInputWidth()), innerWidth),
		)
	} else {
		lines = append(lines, deploymentFieldLine(m, 5, m.t("Branch", "分支"), m.deploymentInputText(5, deploymentInputWidth()), innerWidth))
	}
	lines = append(lines, deploymentFieldLine(m, 8, m.t("App dir", "项目目录"), m.deploymentInputText(8, deploymentInputWidth()), innerWidth))
	if m.deploymentForm.Source == config.DeploySourceRelease {
		lines = append(lines, deploymentFieldLine(m, 9, m.t("Download URL", "下载地址"), m.deploymentInputText(9, deploymentInputWidth()), innerWidth))
		lines = append(lines, m.deploymentReleaseHintLines(innerWidth)...)
	}
	lines = append(lines,
		"",
		deploymentSectionTitle(m.t("GitHub Credential", "GitHub 凭证")),
		deploymentFieldLine(m, 10, m.t("Cred type", "凭证类型"), m.deployCredentialText(m.deploymentForm.Credential)+"  ←/→", innerWidth),
		deploymentFieldLine(m, 11, m.t("Cred param", "凭证参数"), m.deploymentInputText(11, deploymentInputWidth()), innerWidth),
		"",
		deploymentSectionTitle(m.t("Serial Deploy", "串行部署")),
		deploymentFieldLine(m, 12, m.t("Wait", "等待时间"), m.deploymentInputText(12, deploymentInputWidth())+"  "+m.t("s", "秒"), innerWidth),
		mutedStyle.Render(fit(m.t("Note: during multi-app deployment, wait this many seconds after this app before starting the next one.", "说明：多选部署时，此应用完成后等待该秒数再执行下一个。"), innerWidth)),
		"",
		deploymentSectionTitle(m.t("Deploy Flow", "部署流程")),
		deploymentCommandSummaryLine(m, 13, m.t("Before", "更新前"), m.deploymentForm.BeforeCommands, innerWidth),
		deploymentCommandSummaryLine(m, 14, m.t("Fetch", "获取资源"), m.deploymentForm.ResourceCommands, innerWidth),
		deploymentCommandSummaryLine(m, 15, m.t("Update", "更新命令"), m.deploymentForm.UpdateCommands, innerWidth),
		deploymentCommandSummaryLine(m, 16, m.t("After", "更新后"), m.deploymentForm.AfterCommands, innerWidth),
		deploymentCommandSummaryLine(m, 17, m.t("Health", "健康检查"), m.deploymentForm.HealthCommands, innerWidth),
		"",
		deploymentSectionTitle(m.t("Rollback Flow", "回滚流程")),
		deploymentCommandSummaryLine(m, 18, m.t("Rollback", "回滚命令"), m.deploymentForm.RollbackCommands, innerWidth),
	)
	return lines
}

func deploymentSectionTitle(value string) string {
	return "  " + sectionTitle(value)
}

func deploymentInputWidth() int {
	return 34
}

func (m Model) deploymentReleaseHintLines(width int) []string {
	return fitLines([]string{
		mutedStyle.Render(m.t("Note: empty version or latest means the latest Release; v1.0.0 pins a fixed version.", "说明：版本留空或 latest 表示最新 Release；填 v1.0.0 表示固定版本。")),
		mutedStyle.Render(m.t("Note: an asset without * is an exact filename; with * it is matched from Release assets.", "说明：资源不带 * 是固定文件名；带 * 会在 Release 资源里自动匹配。")),
		mutedStyle.Render(m.t("Note: download URL is optional; when set, it is used first.", "说明：下载地址可选；填写后优先使用完整下载地址。")),
	}, width)
}

func (m Model) deploymentServerText(width int) string {
	value := deploymentDisplayServerText(m.deploymentForm.Server)
	index := m.deploymentServerIndex(m.deploymentForm.Server)
	if index >= 0 {
		h := m.states[index].Host
		value = deploymentDisplayServerName(h.Category, h.Name)
	} else if strings.TrimSpace(m.deploymentForm.Server) != "" {
		value += "  " + m.t("not found", "未找到")
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
	placeholder := m.deploymentFieldPlaceholder(field, m.deploymentForm.Source, m.deploymentForm.Credential)
	if placeholder == "" {
		return commandInputText(value, m.deploymentCursor, m.deploymentField == field, width)
	}
	return "[" + fit(placeholder, width) + strings.Repeat(" ", maxInt(0, width-ansi.StringWidth(placeholder))) + "]"
}

func (m Model) deploymentFieldValue(field int) string {
	m.deploymentField = field
	return m.deploymentValue()
}

func (m Model) deploymentFieldPlaceholder(field int, source string, credential string) string {
	switch field {
	case 3:
		return m.t("e.g. api", "例如 api")
	case 4:
		if source == config.DeploySourceRelease {
			return "owner/repo"
		}
		return "git@github.com:owner/repo.git"
	case 5:
		return "main"
	case 6:
		return m.t("latest or v1.0.0", "latest 或 v1.0.0")
	case 7:
		return m.t("app.tar.gz or app-*", "app.tar.gz 或 app-*")
	case 8:
		return "/opt/app"
	case 9:
		return m.t("optional: full download URL", "可选：完整下载地址")
	case 11:
		switch credential {
		case config.DeployCredentialSSH:
			return m.t("local or target private key path", "本地或目标服务器私钥路径")
		case config.DeployCredentialToken:
			return m.t("local or target env var name", "本地或目标服务器环境变量名")
		default:
			return m.t("public repo or server auth is configured", "公开仓库或服务器已配置认证")
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
	labelWidth := runewidth.StringWidth(m.t("Asset/match", "资源文件/匹配"))
	padding := labelWidth - runewidth.StringWidth(label) + 1
	if padding < 1 {
		padding = 1
	}
	return style.Render(fit(prefix+" "+label+strings.Repeat(" ", padding)+value, width))
}

func deploymentCommandSummaryLine(m Model, index int, label string, value string, width int) string {
	count := len(splitCommandBlock(value))
	summary := fmt.Sprintf("%d%s", count, m.t(" lines", "条"))
	if count == 0 {
		summary = m.t("Not configured", "未配置")
	}
	return deploymentFieldLine(m, index, label, summary, width)
}

func (m Model) deploymentFieldName(field int) string {
	switch field {
	case 13:
		return m.t("Before commands", "更新前命令")
	case 14:
		return m.t("Fetch commands", "获取资源命令")
	case 15:
		return m.t("Update commands", "更新命令")
	case 16:
		return m.t("After commands", "更新后命令")
	case 17:
		return m.t("Health check commands", "健康检查命令")
	case 18:
		return m.t("Rollback commands", "回滚命令")
	default:
		return m.t("Commands", "命令")
	}
}
