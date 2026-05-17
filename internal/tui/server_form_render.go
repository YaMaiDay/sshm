package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderAddForm() string {
	title := m.t("Add Server", "添加服务器")
	if m.serverForm.Editing {
		title = m.t("Edit Server", "编辑服务器")
	} else if m.serverForm.Copying {
		title = m.t("Copy Server", "复制服务器")
	}
	width := formContentWidth(m.width)
	if m.useSingleFormPane(width) {
		width = detailFrameWidth(m.width)
	}
	help := m.t("Switch Tab  Move ↑↓  Category ←→  Save Enter  Back Esc", "切换 Tab  选择 ↑↓  分类 ←→  保存 Enter  返回 Esc")
	if m.serverForm.Pane == 1 {
		help = m.t("Back Tab  Move ↑↓  New n  Rename r  Delete x  Back Esc", "切回 Tab  选择 ↑↓  新增 n  重命名 r  删除 x  返回 Esc")
		if m.serverForm.AddingCategory {
			help = m.t("Add Enter  Back Esc", "添加 Enter  返回 Esc")
		} else if m.serverForm.RenamingCategory {
			help = m.t("Rename Enter  Back Esc", "重命名 Enter  返回 Esc")
		}
	}
	header := titleStyle.Render(title)
	if strings.TrimSpace(m.status) != "" && m.status != title {
		statusStyle := mutedStyle
		if m.hasErrorText(m.status) {
			statusStyle = redStyle
		}
		header += "  " + statusStyle.Render(fit(m.status, width-ansi.StringWidth(title)-2))
	}
	bodyHeight := m.height - 2
	if bodyHeight < 8 {
		bodyHeight = 8
	}
	body := ""
	if m.useSingleFormPane(width) {
		if m.serverForm.Pane == 1 {
			body = m.renderCategoryPane(width, bodyHeight)
		} else {
			body = m.renderServerFormPane(title, width, bodyHeight)
		}
	} else {
		gap := 1
		leftWidth := (width - gap) * 2 / 3
		rightWidth := width - gap - leftWidth
		if rightWidth < 28 {
			rightWidth = 28
			leftWidth = width - gap - rightWidth
		}
		left := m.renderServerFormPane(title, leftWidth, bodyHeight)
		right := m.renderCategoryPane(rightWidth, bodyHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	}
	lines := []string{
		header,
		body,
		renderHelp(width, help),
	}
	return strings.Join(lines, "\n")
}
