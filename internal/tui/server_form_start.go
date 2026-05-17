package tui

import (
	"fmt"
	"github.com/YaMaiDay/sshm/internal/config"
	"strings"
)

func (m Model) startAddForm() Model {
	m.reloadCategories("")
	m.mode = modeAddForm
	m.serverForm.Index = 0
	m.serverForm.Cursor = 0
	m.serverForm.Pane = 0
	m.serverForm.Editing = false
	m.serverForm.Copying = false
	m.serverForm.EditIndex = -1
	m.serverForm.AddingCategory = false
	m.serverForm.RenamingCategory = false
	m.serverForm.CategoryDraft = ""
	m.serverForm.Form = addForm{Category: m.serverForm.Categories[m.serverForm.CategoryIndex], User: "root", Port: "22"}
	m.status = m.t("Add Server", "添加服务器")
	return m
}

func (m Model) copyHostName(category string, name string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = m.t("server", "服务器")
	}
	candidate := base + m.t("-copy", "-副本")
	if !m.hostNameExists(category, candidate) {
		return candidate
	}
	for i := 2; ; i++ {
		candidate = fmt.Sprintf("%s%s%d", base, m.t("-copy", "-副本"), i)
		if !m.hostNameExists(category, candidate) {
			return candidate
		}
	}
}

func (m Model) hostNameExists(category string, name string) bool {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	for _, state := range m.states {
		h := state.Host
		if strings.TrimSpace(h.Category) == category && strings.TrimSpace(h.Name) == name {
			return true
		}
	}
	return false
}

func (m Model) startCopyForm(idx int) Model {
	h := m.states[idx].Host
	password, _ := m.passwords.Password(h.Name)
	input := config.InputFromHost(h, password)
	m.reloadCategories(input.Category)
	m.mode = modeAddForm
	m.serverForm.Index = 1
	m.serverForm.Cursor = 0
	m.serverForm.Pane = 0
	m.serverForm.Editing = false
	m.serverForm.Copying = true
	m.serverForm.EditIndex = -1
	m.serverForm.AddingCategory = false
	m.serverForm.RenamingCategory = false
	m.serverForm.CategoryDraft = ""
	name := m.copyHostName(input.Category, input.Name)
	m.serverForm.Form = addForm{
		Category:     m.serverForm.Categories[m.serverForm.CategoryIndex],
		Name:         name,
		HostName:     input.HostName,
		User:         input.User,
		Port:         input.Port,
		IdentityFile: input.IdentityFile,
		Password:     input.Password,
		JumpHostRef:  input.JumpHostRef,
		ExpireAt:     input.ExpireAt,
		Note:         input.Note,
	}
	m.serverForm.Cursor = len([]rune(name))
	m.status = m.t("Copy Server", "复制服务器")
	return m
}

func (m Model) startEditForm(idx int) Model {
	h := m.states[idx].Host
	password, _ := m.passwords.Password(h.Name)
	input := config.InputFromHost(h, password)
	m.reloadCategories(input.Category)
	m.mode = modeAddForm
	m.serverForm.Index = 0
	m.serverForm.Cursor = 0
	m.serverForm.Pane = 0
	m.serverForm.Editing = true
	m.serverForm.Copying = false
	m.serverForm.EditIndex = idx
	m.serverForm.AddingCategory = false
	m.serverForm.RenamingCategory = false
	m.serverForm.CategoryDraft = ""
	m.serverForm.Form = addForm{
		Category:     m.serverForm.Categories[m.serverForm.CategoryIndex],
		Name:         input.Name,
		HostName:     input.HostName,
		User:         input.User,
		Port:         input.Port,
		IdentityFile: input.IdentityFile,
		Password:     input.Password,
		JumpHostRef:  input.JumpHostRef,
		ExpireAt:     input.ExpireAt,
		Note:         input.Note,
	}
	m.status = m.t("Edit Server", "编辑服务器")
	return m
}
