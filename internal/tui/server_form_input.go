package tui

import (
	"fmt"
	"github.com/YaMaiDay/sshm/internal/config"
	"strings"
)

func (m *Model) formAppend(s string) {
	if m.serverForm.Index == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.serverForm.Cursor < 0 {
		m.serverForm.Cursor = 0
	}
	if m.serverForm.Cursor > len(value) {
		m.serverForm.Cursor = len(value)
	}
	insert := []rune(s)
	next := append([]rune{}, value[:m.serverForm.Cursor]...)
	next = append(next, insert...)
	next = append(next, value[m.serverForm.Cursor:]...)
	m.setFormValue(string(next))
	m.serverForm.Cursor += len(insert)
}

func (m *Model) formBackspace() {
	if m.serverForm.Index == 0 {
		return
	}
	value := []rune(m.formValue())
	if m.serverForm.Cursor <= 0 || len(value) == 0 {
		return
	}
	if m.serverForm.Cursor > len(value) {
		m.serverForm.Cursor = len(value)
	}
	next := append([]rune{}, value[:m.serverForm.Cursor-1]...)
	next = append(next, value[m.serverForm.Cursor:]...)
	m.setFormValue(string(next))
	m.serverForm.Cursor--
}

func (m *Model) formExpireAppend(runes []rune) {
	mask := []rune(dateMask(m.serverForm.Form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.serverForm.Cursor, 0, len(positions))
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
	m.serverForm.Form.ExpireAt = string(mask)
	m.serverForm.Cursor = cursor
}

func (m *Model) formExpireBackspace() {
	if m.serverForm.Cursor <= 0 {
		return
	}
	mask := []rune(dateMask(m.serverForm.Form.ExpireAt))
	positions := dateInputPositions()
	cursor := clampInt(m.serverForm.Cursor, 0, len(positions))
	pos := positions[cursor-1]
	mask[pos] = datePlaceholderForPosition(pos)
	m.serverForm.Form.ExpireAt = string(mask)
	m.serverForm.Cursor = cursor - 1
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
	m.serverForm.Cursor += delta
	if m.serverForm.Cursor < 0 {
		m.serverForm.Cursor = 0
	}
	maxCursor := m.formValueLen()
	if m.serverForm.Cursor > maxCursor {
		m.serverForm.Cursor = maxCursor
	}
}

func (m Model) formValueLen() int {
	if m.serverForm.Index == expireAtFormIndex {
		return dateCursorEnd(m.serverForm.Form.ExpireAt)
	}
	return len([]rune(m.formValue()))
}

func (m Model) nextFormIndex() int {
	ids := editableFormIDs(m.serverForm.Form.fields())
	for i, id := range ids {
		if id == m.serverForm.Index {
			return ids[(i+1)%len(ids)]
		}
	}
	return ids[0]
}

func (m Model) prevFormIndex() int {
	ids := editableFormIDs(m.serverForm.Form.fields())
	for i, id := range ids {
		if id == m.serverForm.Index {
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
	switch m.serverForm.Index {
	case 1:
		return m.serverForm.Form.Name
	case 2:
		return m.serverForm.Form.HostName
	case 3:
		return m.serverForm.Form.User
	case 4:
		return m.serverForm.Form.Port
	case 5:
		return m.serverForm.Form.IdentityFile
	case 6:
		return m.serverForm.Form.Password
	case 7:
		return emptyChoice(m.serverForm.Form.JumpHostRef, "无")
	case 8:
		return m.serverForm.Form.Note
	case 9:
		return m.serverForm.Form.ExpireAt
	default:
		return ""
	}
}

func (m *Model) setFormValue(value string) {
	switch m.serverForm.Index {
	case 1:
		m.serverForm.Form.Name = value
	case 2:
		m.serverForm.Form.HostName = value
	case 3:
		m.serverForm.Form.User = value
	case 4:
		m.serverForm.Form.Port = value
	case 5:
		m.serverForm.Form.IdentityFile = value
	case 6:
		m.serverForm.Form.Password = value
	case 7:
		m.serverForm.Form.JumpHostRef = strings.TrimSpace(value)
	case 8:
		m.serverForm.Form.Note = value
	case 9:
		m.serverForm.Form.ExpireAt = value
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
