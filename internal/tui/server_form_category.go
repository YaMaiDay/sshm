package tui

import (
	"errors"
	"github.com/YaMaiDay/sshm/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"sort"
	"strings"
)

func (m Model) updateCategoryPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.serverForm.AddingCategory || m.serverForm.RenamingCategory {
		key := shortcutKey(msg)
		switch key {
		case "esc", "ctrl+c":
			m.serverForm.AddingCategory = false
			m.serverForm.RenamingCategory = false
			m.serverForm.CategoryDraft = ""
		case "enter":
			if m.serverForm.RenamingCategory {
				oldName := ""
				if len(m.serverForm.Categories) > 0 {
					oldName = m.serverForm.Categories[m.serverForm.CategoryIndex]
				}
				if err := config.RenameCategory(m.home, oldName, m.serverForm.CategoryDraft); err != nil {
					m.status = m.t("Rename category failed: ", "重命名分类失败：") + m.categoryErrorText(err)
				} else {
					newName := strings.TrimSpace(m.serverForm.CategoryDraft)
					hosts, err := config.LoadHosts(m.home)
					if err != nil {
						m.status = m.t("Reload after rename failed: ", "重命名后重新读取失败：") + err.Error()
					} else {
						m.reloadHosts(hosts)
					}
					m.reloadCategories(newName)
					m.serverForm.Form.Category = m.serverForm.Categories[m.serverForm.CategoryIndex]
					if m.category == oldName {
						m.category = newName
					}
					m.status = m.t("Category renamed.", "分类已重命名。")
				}
			} else {
				if err := config.AddCategory(m.home, m.serverForm.CategoryDraft); err != nil {
					m.status = m.t("Add category failed: ", "添加分类失败：") + m.categoryErrorText(err)
				} else {
					m.reloadCategories(m.serverForm.CategoryDraft)
					m.serverForm.Form.Category = m.serverForm.Categories[m.serverForm.CategoryIndex]
					m.status = m.t("Category added.", "分类已添加。")
				}
			}
			m.serverForm.AddingCategory = false
			m.serverForm.RenamingCategory = false
			m.serverForm.CategoryDraft = ""
		case "backspace":
			m.serverForm.CategoryDraft = removeLastRune(m.serverForm.CategoryDraft)
		default:
			if len(msg.Runes) > 0 {
				m.serverForm.CategoryDraft += string(msg.Runes)
			}
		}
		return m, nil
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		m.mode = modeDashboard
		m.status = m.t("Canceled.", "已取消。")
	case "tab", "shift+tab":
		m.serverForm.Pane = 0
	case "j", "down":
		m.moveCategory(1)
	case "k", "up":
		m.moveCategory(-1)
	case "n", "a":
		m.serverForm.AddingCategory = true
		m.serverForm.RenamingCategory = false
		m.serverForm.CategoryDraft = ""
		m.status = m.t("Enter new category name.", "输入新分类名称。")
	case "r":
		if len(m.serverForm.Categories) == 0 {
			return m, nil
		}
		name := m.serverForm.Categories[m.serverForm.CategoryIndex]
		if name == config.BastionCategory {
			m.status = m.t("The bastion category cannot be renamed.", "跳板机分类不能重命名。")
			return m, nil
		}
		m.serverForm.RenamingCategory = true
		m.serverForm.AddingCategory = false
		m.serverForm.CategoryDraft = name
		m.status = m.t("Enter the new category name.", "输入新的分类名称。")
	case "x":
		if len(m.serverForm.Categories) == 0 {
			return m, nil
		}
		name := m.serverForm.Categories[m.serverForm.CategoryIndex]
		m.confirm = confirmAction{
			Kind:  confirmDeleteCategory,
			Title: m.t("Delete Category", "确认删除分类"),
			Lines: []string{
				m.t("Category: ", "分类：") + name,
				m.t("This empty category will be deleted.", "将删除这个空分类。"),
			},
			Back:  modeAddForm,
			Value: name,
		}
		m.mode = modeConfirmAction
	}
	return m, nil
}

func (m *Model) moveCategory(delta int) {
	if len(m.serverForm.Categories) == 0 {
		m.serverForm.Categories = []string{"default"}
		m.serverForm.CategoryIndex = 0
		m.serverForm.Form.Category = "default"
		return
	}
	m.serverForm.CategoryIndex += delta
	if m.serverForm.CategoryIndex < 0 {
		m.serverForm.CategoryIndex = len(m.serverForm.Categories) - 1
	}
	if m.serverForm.CategoryIndex >= len(m.serverForm.Categories) {
		m.serverForm.CategoryIndex = 0
	}
	m.serverForm.Form.Category = m.serverForm.Categories[m.serverForm.CategoryIndex]
}

func (m *Model) moveJumpHostRef(delta int) {
	choices := append([]string{""}, m.bastionNames()...)
	if len(choices) == 0 {
		m.serverForm.Form.JumpHostRef = ""
		return
	}
	current := strings.TrimSpace(m.serverForm.Form.JumpHostRef)
	index := 0
	for i, choice := range choices {
		if choice == current {
			index = i
			break
		}
	}
	index = (index + delta) % len(choices)
	if index < 0 {
		index += len(choices)
	}
	m.serverForm.Form.JumpHostRef = choices[index]
}

func (m Model) bastionNames() []string {
	names := []string{}
	for _, state := range m.states {
		h := state.Host
		if h.Category != config.BastionCategory {
			continue
		}
		if m.serverForm.Editing && m.serverForm.EditIndex >= 0 && m.serverForm.EditIndex < len(m.states) {
			current := m.states[m.serverForm.EditIndex].Host
			if current.Category == h.Category && current.Name == h.Name {
				continue
			}
		}
		names = append(names, h.Name)
	}
	sort.Strings(names)
	return names
}

func (m *Model) reloadCategories(prefer string) {
	categories, _, err := config.LoadCategories(m.home)
	if err != nil || len(categories) == 0 {
		categories = []string{"default"}
	}
	m.serverForm.Categories = categories
	m.serverForm.CategoryIndex = 0
	if strings.TrimSpace(prefer) == "" {
		prefer = "default"
	}
	for i, category := range categories {
		if category == prefer {
			m.serverForm.CategoryIndex = i
			break
		}
	}
}

func (m Model) categoryErrorText(err error) string {
	switch {
	case errors.Is(err, os.ErrInvalid):
		return m.t("At least one category is required, and the category name cannot be empty", "至少需要保留一个分类，或分类名称不能为空")
	case errors.Is(err, os.ErrPermission):
		return m.t("The bastion category cannot be renamed or deleted; non-empty categories cannot be deleted", "跳板机分类不能重命名或删除，分类下面还有服务器时也不能删除")
	case errors.Is(err, os.ErrExist):
		return m.t("Category already exists", "分类名称已存在")
	case errors.Is(err, os.ErrNotExist):
		return m.t("Category does not exist", "分类不存在")
	default:
		return err.Error()
	}
}
