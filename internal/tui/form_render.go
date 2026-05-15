package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func (m Model) useSingleFormPane(width int) bool {
	return width < 96
}

func (m Model) renderServerFormPane(title string, width int, height int) string {
	fields := m.form.fields()
	for i := range fields {
		fields[i].Label = m.formLabel(fields[i].Label)
		if fields[i].ID == jumpHostRefFormIndex && strings.TrimSpace(fields[i].Value) == "无" {
			fields[i].Value = m.t("None", "无")
		}
	}
	lines := make([]string, 0, len(fields)+2)
	lines = append(lines, titleStyle.Render(m.t("Server", "服务器")))
	innerWidth := width - 4
	if innerWidth < 24 {
		innerWidth = 24
	}
	contentHeight := height - 2
	if contentHeight < 4 {
		contentHeight = 4
	}
	if m.formIndex == jumpHostRefFormIndex || strings.TrimSpace(m.form.JumpHostRef) != "" {
		lines = append(lines, mutedStyle.Render(m.t("The bastion only forwards connections. Private keys stay local.", "跳板机只转发连接，密钥文件都在本地。")))
	}
	fieldHeight := contentHeight - len(lines)
	if fieldHeight < 1 {
		fieldHeight = 1
	}
	selectedRow := selectedFieldRow(fields, m.formIndex)
	start, end := visibleRange(len(fields), selectedRow, fieldHeight)
	for i := start; i < end; i++ {
		field := fields[i]
		if field.Section {
			if len(lines) > 1 {
				lines = append(lines, "")
			}
			lines = append(lines, sectionTitle(field.Label))
			continue
		}
		prefix := " "
		style := lipgloss.NewStyle()
		value := field.Value
		if field.ID == categoryFormIndex {
			value = m.form.Category
			if value == "" && len(m.categories) > 0 {
				value = m.categories[m.categoryIndex]
			}
			value = m.displayCategoryName(value)
			value += mutedStyle.Render("  ←/→")
		} else if field.ID == expireAtFormIndex {
			value = dateInputText(m.form.ExpireAt, m.formCursor, m.formPane == 0 && field.ID == m.formIndex)
		} else if field.ID == jumpHostRefFormIndex {
			value += mutedStyle.Render("  ←/→")
		}
		if m.formPane == 0 && field.ID == m.formIndex {
			prefix = "▶"
			style = blueStyle.Bold(true)
		}
		if field.ID == expireAtFormIndex || field.ID == jumpHostRefFormIndex {
			lines = append(lines, style.Render(formFieldLine(prefix, field.Label, value, innerWidth, false, false, m.formCursor)))
		} else {
			lines = append(lines, style.Render(formFieldLine(prefix, field.Label, value, innerWidth, field.ID != categoryFormIndex, m.formPane == 0 && field.ID == m.formIndex, m.formCursor)))
		}
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width)
	if m.formPane == 0 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) renderCategoryPane(width int, height int) string {
	lines := []string{titleStyle.Render(m.t("Categories", "分类"))}
	innerWidth := width - 4
	if innerWidth < 20 {
		innerWidth = 20
	}
	contentHeight := height - 2
	if contentHeight < 5 {
		contentHeight = 5
	}
	bottomLineCount := 0
	listHeight := contentHeight - 1 - bottomLineCount
	if listHeight < 1 {
		listHeight = 1
	}
	if len(m.categories) == 0 {
		lines = append(lines, mutedStyle.Render(m.t("No categories", "没有分类")))
	} else {
		start, end := visibleRange(len(m.categories), m.categoryIndex, listHeight)
		for i := start; i < end; i++ {
			category := m.categories[i]
			prefix := " "
			style := lipgloss.NewStyle()
			if i == m.categoryIndex {
				prefix = "▶"
				if m.formPane == 1 && !m.addingCategory {
					style = blueStyle.Bold(true)
				}
			}
			count := m.categoryHostCount(category)
			lines = append(lines, style.Render(m.categoryLine(prefix, category, count, innerWidth)))
		}
	}
	for len(lines) < 1+listHeight {
		lines = append(lines, "")
	}
	if m.addingCategory || m.renamingCategory {
		label := "新分类 "
		if m.renamingCategory {
			label = "重命名 "
		}
		if !m.isChineseUI() {
			label = "New "
			if m.renamingCategory {
				label = "Rename "
			}
		}
		lines = append(lines, blueStyle.Bold(true).Render(prefixedCursorText(label, m.categoryDraft, innerWidth)))
	}
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(softGray).
		Padding(0, 1).
		Width(width)
	if m.formPane == 1 {
		style = style.BorderForeground(blue)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) formLabel(label string) string {
	switch label {
	case "基础信息":
		return m.t("Basic", "基础信息")
	case "分类":
		return m.t("Category", "分类")
	case "服务器名称":
		return m.t("Server name", "服务器名称")
	case "目标服务器":
		return m.t("Target Server", "目标服务器")
	case "服务器地址":
		return m.t("Host", "服务器地址")
	case "用户名":
		return m.t("User", "用户名")
	case "端口":
		return m.t("Port", "端口")
	case "服务器本地密钥文件":
		return m.t("Local key file", "服务器本地密钥文件")
	case "密码":
		return m.t("Password", "密码")
	case "跳板机":
		return m.t("Bastion", "跳板机")
	case "使用跳板机":
		return m.t("Use bastion", "使用跳板机")
	case "辅助信息":
		return m.t("Extra", "辅助信息")
	case "备注":
		return m.t("Note", "备注")
	case "到期时间":
		return m.t("Expire date", "到期时间")
	default:
		return label
	}
}

func formFieldLine(prefix string, label string, value string, width int, boxed bool, active bool, cursor int) string {
	const labelWidth = 12
	labelText := prefix + " " + padVisible(label, labelWidth)
	valueWidth := width - ansi.StringWidth(labelText) - 1
	if valueWidth < 8 {
		valueWidth = 8
	}
	if boxed {
		if valueWidth > 32 {
			valueWidth = 32
		}
		boxWidth := valueWidth - 2
		if boxWidth < 4 {
			boxWidth = 4
		}
		if active {
			value = "[" + formInputText(value, boxWidth, cursor) + "]"
		} else {
			value = "[" + padVisible(value, boxWidth) + "]"
		}
	} else {
		value = fit(value, valueWidth)
	}
	return fit(labelText+" "+value, width)
}

func dateInputText(value string, cursor int, active bool) string {
	runes := []rune(dateMask(value))
	positions := dateInputPositions()
	if active {
		cursor = clampInt(cursor, 0, len(positions))
		if cursor < len(positions) {
			pos := positions[cursor]
			runes = append(runes[:pos], append([]rune{'│'}, runes[pos:]...)...)
		} else {
			runes = append(runes, '│')
		}
	}
	return "[" + string(runes) + "]"
}

func dateDigits(value string) string {
	var out []rune
	for _, r := range value {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return string(out)
}

func formInputText(value string, width int, cursor int) string {
	return padVisible(inlineCursorText(value, width, cursor), width)
}

func inlineCursorText(value string, width int, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	if width < 1 {
		width = 1
	}
	contentWidth := width - 1
	before := visibleTailByWidth(runes[:cursor], contentWidth)
	remaining := contentWidth - runewidth.StringWidth(before)
	if remaining < 0 {
		remaining = 0
	}
	after := visibleHeadByWidth(runes[cursor:], remaining)
	return before + "│" + after
}

func prefixedCursorText(prefix string, value string, width int) string {
	inputWidth := width - runewidth.StringWidth(prefix)
	if inputWidth < 1 {
		inputWidth = 1
	}
	return fit(prefix+inlineCursorText(value, inputWidth, len([]rune(value))), width)
}

func visibleTailByWidth(runes []rune, width int) string {
	if width <= 0 || len(runes) == 0 {
		return ""
	}
	used := 0
	start := len(runes)
	for start > 0 {
		r := runes[start-1]
		rw := runewidth.RuneWidth(r)
		if used+rw > width {
			break
		}
		used += rw
		start--
	}
	return string(runes[start:])
}

func visibleHeadByWidth(runes []rune, width int) string {
	if width <= 0 || len(runes) == 0 {
		return ""
	}
	used := 0
	end := 0
	for end < len(runes) {
		rw := runewidth.RuneWidth(runes[end])
		if used+rw > width {
			break
		}
		used += rw
		end++
	}
	return string(runes[:end])
}

func (m Model) categoryLine(prefix string, category string, count int, width int) string {
	countText := ""
	if count > 0 {
		countText = fmt.Sprintf("%d%s", count, m.t(" servers", "台"))
	}
	prefixText := prefix + " "
	nameWidth := width - ansi.StringWidth(prefixText) - ansi.StringWidth(countText)
	if countText != "" {
		nameWidth--
	}
	if nameWidth < 6 {
		nameWidth = 6
	}
	line := prefixText + fit(m.displayCategoryName(category), nameWidth)
	if countText != "" {
		spaces := width - ansi.StringWidth(line) - ansi.StringWidth(countText)
		if spaces < 1 {
			spaces = 1
		}
		line += strings.Repeat(" ", spaces) + countText
	}
	return fit(line, width)
}

func (m Model) displayCategoryName(category string) string {
	if strings.TrimSpace(category) == "跳板机" && !m.isChineseUI() {
		return "Bastion"
	}
	return category
}

func (m Model) categoryHostCount(category string) int {
	count := 0
	for _, state := range m.states {
		if state.Host.Category == category {
			count++
		}
	}
	return count
}
