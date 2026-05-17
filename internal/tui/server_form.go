package tui

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (f addForm) fields() []formField {
	fields := []formField{
		{Label: "基础信息", Section: true},
		{ID: categoryFormIndex, Label: "分类", Value: f.Category},
		{ID: nameFormIndex, Label: "服务器名称", Value: f.Name},
		{Label: "目标服务器", Section: true},
		{ID: hostFormIndex, Label: "服务器地址", Value: f.HostName},
		{ID: userFormIndex, Label: "用户名", Value: f.User},
		{ID: portFormIndex, Label: "端口", Value: f.Port},
		{ID: identityFormIndex, Label: "服务器本地密钥文件", Value: f.IdentityFile},
		{ID: passwordFormIndex, Label: "密码", Value: f.Password},
	}
	if f.Category != config.BastionCategory {
		fields = append(fields,
			formField{Label: "跳板机", Section: true},
			formField{ID: jumpHostRefFormIndex, Label: "使用跳板机", Value: emptyChoice(f.JumpHostRef, "无")},
		)
	}
	fields = append(fields,
		formField{Label: "辅助信息", Section: true},
		formField{ID: noteFormIndex, Label: "备注", Value: f.Note},
		formField{ID: expireAtFormIndex, Label: "到期时间", Value: f.ExpireAt},
	)
	return fields
}

func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formPane == 1 {
		return m.updateCategoryPane(msg)
	}
	key := shortcutKey(msg)
	switch key {
	case "esc", "ctrl+c":
		m.mode = modeDashboard
		m.copying = false
		m.status = m.t("Canceled.", "已取消。")
	case "tab":
		m.formPane = 1
	case "down":
		m.formIndex = m.nextFormIndex()
		m.formCursor = m.formValueLen()
	case "shift+tab":
		m.formPane = 1
	case "up":
		m.formIndex = m.prevFormIndex()
		m.formCursor = m.formValueLen()
	case "left":
		if m.formIndex == 0 {
			m.moveCategory(-1)
		} else if m.formIndex == jumpHostRefFormIndex {
			m.moveJumpHostRef(-1)
		} else {
			m.moveFormCursor(-1)
		}
	case "right":
		if m.formIndex == 0 {
			m.moveCategory(1)
		} else if m.formIndex == jumpHostRefFormIndex {
			m.moveJumpHostRef(1)
		} else {
			m.moveFormCursor(1)
		}
	case "enter":
		expireAt, err := normalizeExpireAtForSave(m.form.ExpireAt)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		favorite := false
		pinned := false
		pinnedOrder := int64(0)
		if m.editing {
			if m.editIndex < 0 || m.editIndex >= len(m.states) {
				m.status = "编辑失败：没有选中的服务器"
				return m, nil
			}
			favorite = m.states[m.editIndex].Host.Favorite
			pinned = m.states[m.editIndex].Host.Pinned
			pinnedOrder = m.states[m.editIndex].Host.PinnedOrder
		}
		input := config.HostInput{
			Category:     m.form.Category,
			Name:         m.form.Name,
			HostName:     m.form.HostName,
			User:         m.form.User,
			Port:         m.form.Port,
			IdentityFile: m.form.IdentityFile,
			Password:     m.form.Password,
			JumpHostRef:  m.form.JumpHostRef,
			Note:         m.form.Note,
			ExpireAt:     expireAt,
			Favorite:     favorite,
			Pinned:       pinned,
			PinnedOrder:  pinnedOrder,
		}
		if m.editing {
			if err := config.EditHost(m.home, m.states[m.editIndex].Host, input); err != nil {
				m.status = "编辑失败：" + err.Error()
				return m, nil
			}
		} else {
			if err := config.AddHost(m.home, input); err != nil {
				m.status = "添加失败：" + err.Error()
				return m, nil
			}
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		if m.editing {
			m.status = "服务器已更新。"
		} else if m.copying {
			m.status = "服务器已复制。"
		} else {
			m.status = "服务器已添加。"
		}
		m.copying = false
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	case "backspace":
		if m.formIndex == expireAtFormIndex {
			m.formExpireBackspace()
		} else {
			m.formBackspace()
		}
	default:
		if len(msg.Runes) > 0 && m.formIndex != 0 {
			if m.formIndex == expireAtFormIndex {
				m.formExpireAppend(msg.Runes)
			} else {
				m.formAppend(string(msg.Runes))
			}
		}
	}
	return m, nil
}

func (m Model) updateCategoryPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.addingCategory || m.renamingCategory {
		key := shortcutKey(msg)
		switch key {
		case "esc", "ctrl+c":
			m.addingCategory = false
			m.renamingCategory = false
			m.categoryDraft = ""
		case "enter":
			if m.renamingCategory {
				oldName := ""
				if len(m.categories) > 0 {
					oldName = m.categories[m.categoryIndex]
				}
				if err := config.RenameCategory(m.home, oldName, m.categoryDraft); err != nil {
					m.status = m.t("Rename category failed: ", "重命名分类失败：") + m.categoryErrorText(err)
				} else {
					newName := strings.TrimSpace(m.categoryDraft)
					hosts, err := config.LoadHosts(m.home)
					if err != nil {
						m.status = m.t("Reload after rename failed: ", "重命名后重新读取失败：") + err.Error()
					} else {
						m.reloadHosts(hosts)
					}
					m.reloadCategories(newName)
					m.form.Category = m.categories[m.categoryIndex]
					if m.category == oldName {
						m.category = newName
					}
					m.status = m.t("Category renamed.", "分类已重命名。")
				}
			} else {
				if err := config.AddCategory(m.home, m.categoryDraft); err != nil {
					m.status = m.t("Add category failed: ", "添加分类失败：") + m.categoryErrorText(err)
				} else {
					m.reloadCategories(m.categoryDraft)
					m.form.Category = m.categories[m.categoryIndex]
					m.status = m.t("Category added.", "分类已添加。")
				}
			}
			m.addingCategory = false
			m.renamingCategory = false
			m.categoryDraft = ""
		case "backspace":
			m.categoryDraft = removeLastRune(m.categoryDraft)
		default:
			if len(msg.Runes) > 0 {
				m.categoryDraft += string(msg.Runes)
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
		m.formPane = 0
	case "j", "down":
		m.moveCategory(1)
	case "k", "up":
		m.moveCategory(-1)
	case "n", "a":
		m.addingCategory = true
		m.renamingCategory = false
		m.categoryDraft = ""
		m.status = m.t("Enter new category name.", "输入新分类名称。")
	case "r":
		if len(m.categories) == 0 {
			return m, nil
		}
		name := m.categories[m.categoryIndex]
		if name == config.BastionCategory {
			m.status = m.t("The bastion category cannot be renamed.", "跳板机分类不能重命名。")
			return m, nil
		}
		m.renamingCategory = true
		m.addingCategory = false
		m.categoryDraft = name
		m.status = m.t("Enter the new category name.", "输入新的分类名称。")
	case "x":
		if len(m.categories) == 0 {
			return m, nil
		}
		name := m.categories[m.categoryIndex]
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
	if len(m.categories) == 0 {
		m.categories = []string{"default"}
		m.categoryIndex = 0
		m.form.Category = "default"
		return
	}
	m.categoryIndex += delta
	if m.categoryIndex < 0 {
		m.categoryIndex = len(m.categories) - 1
	}
	if m.categoryIndex >= len(m.categories) {
		m.categoryIndex = 0
	}
	m.form.Category = m.categories[m.categoryIndex]
}

func (m *Model) moveJumpHostRef(delta int) {
	choices := append([]string{""}, m.bastionNames()...)
	if len(choices) == 0 {
		m.form.JumpHostRef = ""
		return
	}
	current := strings.TrimSpace(m.form.JumpHostRef)
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
	m.form.JumpHostRef = choices[index]
}

func (m Model) bastionNames() []string {
	names := []string{}
	for _, state := range m.states {
		h := state.Host
		if h.Category != config.BastionCategory {
			continue
		}
		if m.editing && m.editIndex >= 0 && m.editIndex < len(m.states) {
			current := m.states[m.editIndex].Host
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
	m.categories = categories
	m.categoryIndex = 0
	if strings.TrimSpace(prefer) == "" {
		prefer = "default"
	}
	for i, category := range categories {
		if category == prefer {
			m.categoryIndex = i
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

func (m Model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n":
		m.mode = modeDashboard
		m.status = "已取消删除。"
	case "y", "enter":
		if m.deleteIndex < 0 || m.deleteIndex >= len(m.states) {
			m.mode = modeDashboard
			m.status = "没有选中的服务器。"
			return m, nil
		}
		h := m.states[m.deleteIndex].Host
		if err := config.DeleteHost(m.home, h, true); err != nil {
			m.mode = modeDashboard
			m.status = "删除失败：" + err.Error()
			return m, nil
		}
		hosts, err := config.LoadHosts(m.home)
		if err != nil {
			m.mode = modeDashboard
			m.status = "重新读取失败：" + err.Error()
			return m, nil
		}
		m.reloadHosts(hosts)
		m.mode = modeDashboard
		m.status = "服务器已删除。"
		m.collectRound++
		m.pendingByRound[m.collectRound] = len(m.states)
		return m, m.collectAll(m.collectRound, false)
	}
	return m, nil
}

func (m Model) updateConfirmAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "n", "q":
		m.mode = m.confirm.Back
		if m.confirm.Kind == confirmRemoveResource {
			m.status = m.t("Canceled.", "已取消。")
		} else {
			m.status = "已取消删除。"
		}
	case "y", "enter":
		switch m.confirm.Kind {
		case confirmDeleteCategory:
			name := m.confirm.Value
			if err := config.DeleteCategory(m.home, name); err != nil {
				m.mode = m.confirm.Back
				m.status = m.t("Delete category failed: ", "删除分类失败：") + m.categoryErrorText(err)
				return m, nil
			}
			m.reloadCategories("")
			m.form.Category = m.categories[m.categoryIndex]
			m.mode = modeAddForm
			m.status = m.t("Category deleted.", "分类已删除。")
		case confirmDeleteCommand:
			item := m.confirm.Command
			m.mode = modeCommandList
			return m.deleteCommandTemplate(item)
		case confirmDeleteHistory:
			entry := m.confirm.History
			m.mode = modeCommandHistory
			return m.deleteCommandHistoryEntry(entry)
		case confirmDeleteDeployment:
			index := m.confirm.Index
			m.mode = modeDeploymentList
			return m.deleteDeploymentApp(index)
		case confirmRemoveResource:
			item := m.confirm.Resource
			m.mode = m.confirm.Back
			return m.removeManagedResource(item)
		}
		m.confirm = confirmAction{}
	}
	return m, nil
}

func (m Model) startAddForm() Model {
	m.reloadCategories("")
	m.mode = modeAddForm
	m.formIndex = 0
	m.formCursor = 0
	m.formPane = 0
	m.editing = false
	m.copying = false
	m.editIndex = -1
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	m.form = addForm{Category: m.categories[m.categoryIndex], User: "root", Port: "22"}
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
	m.formIndex = 1
	m.formCursor = 0
	m.formPane = 0
	m.editing = false
	m.copying = true
	m.editIndex = -1
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	name := m.copyHostName(input.Category, input.Name)
	m.form = addForm{
		Category:     m.categories[m.categoryIndex],
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
	m.formCursor = len([]rune(name))
	m.status = m.t("Copy Server", "复制服务器")
	return m
}

func (m Model) startEditForm(idx int) Model {
	h := m.states[idx].Host
	password, _ := m.passwords.Password(h.Name)
	input := config.InputFromHost(h, password)
	m.reloadCategories(input.Category)
	m.mode = modeAddForm
	m.formIndex = 0
	m.formCursor = 0
	m.formPane = 0
	m.editing = true
	m.copying = false
	m.editIndex = idx
	m.addingCategory = false
	m.renamingCategory = false
	m.categoryDraft = ""
	m.form = addForm{
		Category:     m.categories[m.categoryIndex],
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

func (m *Model) formAppend(s string) {
	if m.formIndex == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.formCursor < 0 {
		m.formCursor = 0
	}
	if m.formCursor > len(value) {
		m.formCursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.formCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.formCursor:]...)
	m.setFormValue(string(next))
	m.formCursor += len(insert)
}

func (m *Model) formBackspace() {
	if m.formIndex == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.formCursor <= 0 || len(value) == 0 {
		return
	}
	if m.formCursor > len(value) {
		m.formCursor = len(value)
	}
	next := append([]rune{}, value[:m.formCursor-1]...)
	next = append(next, value[m.formCursor:]...)
	m.setFormValue(string(next))
	m.formCursor--
}

func (m *Model) formExpireAppend(runes []rune) {
	mask := []rune(dateMask(m.form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.formCursor, 0, len(positions))
	for _, r := range runes {
		if r >= '０' && r <= '９' {
			r = r - '０' + '0'
		}
		if r < '0' || r > '9' || cursor >= len(positions) {
			continue
		}
		mask[positions[cursor]] = r
		cursor++
	}
	m.form.ExpireAt = string(mask)
	m.formCursor = cursor
}

func (m *Model) formExpireBackspace() {
	if m.formCursor <= 0 {
		return
	}
	mask := []rune(dateMask(m.form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.formCursor, 0, len(positions))
	pos := positions[cursor-1]
	mask[pos] = datePlaceholderForPosition(pos)
	m.form.ExpireAt = string(mask)
	m.formCursor = cursor - 1
}

func dateMask(value string) string {
	base := []rune("yyyy-mm-dd")
	positions := dateInputPositions()
	runes := []rune(value)
	if len(runes) == len(base) {
		for _, pos := range positions {
			r := runes[pos]
			if (r >= '0' && r <= '9') || r == datePlaceholderForPosition(pos) {
				base[pos] = r
			}
		}
		return string(base)
	}
	digits := []rune(dateDigits(value))
	for i, r := range digits {
		if i >= len(positions) {
			break
		}
		base[positions[i]] = r
	}
	return string(base)
}

func normalizeExpireAtForSave(value string) (string, error) {
	mask := []rune(dateMask(value))
	positions := dateInputPositions()
	digits := make([]rune, 0, len(positions))
	hasValue := false
	incomplete := false
	for _, pos := range positions {
		r := mask[pos]
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
			hasValue = true
			continue
		}
		incomplete = true
	}
	if !hasValue {
		return "", nil
	}
	if incomplete {
		return "", fmt.Errorf("到期时间未填写完整")
	}
	value = fmt.Sprintf("%s-%s-%s", string(digits[:4]), string(digits[4:6]), string(digits[6:8]))
	if err := config.ValidateExpireAt(value); err != nil {
		return "", err
	}
	return value, nil
}

func datePlaceholderForPosition(pos int) rune {
	switch pos {
	case 0, 1, 2, 3:
		return 'y'
	case 5, 6:
		return 'm'
	default:
		return 'd'
	}
}

func dateInputPositions() []int {
	return []int{0, 1, 2, 3, 5, 6, 8, 9}
}

func (m *Model) moveFormCursor(delta int) {
	m.formCursor += delta
	if m.formCursor < 0 {
		m.formCursor = 0
	}
	maxCursor := m.formValueLen()
	if m.formCursor > maxCursor {
		m.formCursor = maxCursor
	}
}

func (m Model) formValueLen() int {
	if m.formIndex == expireAtFormIndex {
		return dateCursorEnd(m.form.ExpireAt)
	}
	return len([]rune(m.formValue()))
}

func (m Model) nextFormIndex() int {
	ids := editableFormIDs(m.form.fields())
	for i, id := range ids {
		if id == m.formIndex {
			return ids[(i+1)%len(ids)]
		}
	}
	return ids[0]
}

func (m Model) prevFormIndex() int {
	ids := editableFormIDs(m.form.fields())
	for i, id := range ids {
		if id == m.formIndex {
			if i == 0 {
				return ids[len(ids)-1]
			}
			return ids[i-1]
		}
	}
	return ids[0]
}

func editableFormIDs(fields []formField) []int {
	ids := make([]int, 0, len(fields))
	for _, field := range fields {
		if !field.Section {
			ids = append(ids, field.ID)
		}
	}
	if len(ids) == 0 {
		return []int{categoryFormIndex}
	}
	return ids
}

func selectedFieldRow(fields []formField, id int) int {
	for i, field := range fields {
		if !field.Section && field.ID == id {
			return i
		}
	}
	return 0
}

func dateCursorEnd(value string) int {
	mask := []rune(dateMask(value))
	positions := dateInputPositions()
	for i, pos := range positions {
		r := mask[pos]
		if r < '0' || r > '9' {
			return i
		}
	}
	return len(positions)
}

func (m Model) formValue() string {
	switch m.formIndex {
	case 1:
		return m.form.Name
	case 2:
		return m.form.HostName
	case 3:
		return m.form.User
	case 4:
		return m.form.Port
	case 5:
		return m.form.IdentityFile
	case 6:
		return m.form.Password
	case 7:
		return emptyChoice(m.form.JumpHostRef, "无")
	case 8:
		return m.form.Note
	case 9:
		return m.form.ExpireAt
	default:
		return ""
	}
}

func (m *Model) setFormValue(value string) {
	switch m.formIndex {
	case 1:
		m.form.Name = value
	case 2:
		m.form.HostName = value
	case 3:
		m.form.User = value
	case 4:
		m.form.Port = value
	case 5:
		m.form.IdentityFile = value
	case 6:
		m.form.Password = value
	case 7:
		m.form.JumpHostRef = strings.TrimSpace(value)
	case 8:
		m.form.Note = value
	case 9:
		m.form.ExpireAt = value
	}
}

func emptyChoice(value, empty string) string {
	if strings.TrimSpace(value) == "" {
		return empty
	}
	return value
}

func removeLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}
