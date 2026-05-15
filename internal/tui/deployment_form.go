package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
)

func (m Model) defaultDeploymentServer() string {
	if m.activeDeployment.HostIndex >= 0 && m.activeDeployment.HostIndex < len(m.states) {
		h := m.states[m.activeDeployment.HostIndex].Host
		return config.ServerCommandKey(h.Category, h.Name)
	}
	if len(m.states) > 0 {
		h := m.states[0].Host
		return config.ServerCommandKey(h.Category, h.Name)
	}
	return ""
}

func (m Model) deploymentServerIndex(server string) int {
	server = strings.TrimSpace(server)
	for i, state := range m.states {
		if config.ServerCommandKey(state.Host.Category, state.Host.Name) == server {
			return i
		}
	}
	return -1
}

func (m *Model) cycleDeploymentServer(delta int) {
	if len(m.states) == 0 {
		m.deploymentForm.Server = ""
		return
	}
	index := m.deploymentServerIndex(m.deploymentForm.Server)
	if index < 0 {
		index = 0
	} else {
		index = moveIndex(index, len(m.states), delta)
	}
	h := m.states[index].Host
	m.deploymentForm.Server = config.ServerCommandKey(h.Category, h.Name)
}

func (m Model) startDeploymentEdit(app config.DeploymentApp, editing bool) Model {
	if !editing {
		app.Source = config.DeploySourceGit
		app.FetchMode = config.DeployFetchLocal
		app.Credential = config.DeployCredentialSSH
		app.Branch = "main"
		app.Server = m.defaultDeploymentServer()
	}
	m.deploymentForm = deploymentFormFromApp(app)
	m.deploymentField = 0
	m.deploymentCursor = len([]rune(m.deploymentForm.Name))
	m.deploymentEditing = editing
	m.deploymentEditIndex = -1
	if editing {
		if item, ok := m.selectedDeploymentItem(); ok {
			m.deploymentEditIndex = item.Index
		}
	}
	m.mode = modeDeploymentEdit
	if editing {
		m.status = m.t("Edit Deployment App", "编辑部署应用")
	} else {
		m.status = m.t("Add Deployment App", "添加部署应用")
	}
	return m
}

func deploymentFormFromApp(app config.DeploymentApp) deploymentForm {
	return deploymentForm{
		Name:             app.Name,
		Server:           app.Server,
		Source:           emptyChoice(app.Source, config.DeploySourceGit),
		FetchMode:        emptyChoice(app.FetchMode, config.DeployFetchLocal),
		Repo:             app.Repo,
		Branch:           app.Branch,
		Version:          app.Version,
		Asset:            app.Asset,
		Path:             app.Path,
		ReleaseURL:       app.ReleaseURL,
		Credential:       emptyChoice(app.Credential, config.DeployCredentialNone),
		CredentialName:   app.CredentialName,
		WaitSeconds:      strconv.Itoa(maxInt(0, app.WaitSeconds)),
		BeforeCommands:   strings.Join(app.BeforeCommands, "\n"),
		ResourceCommands: deploymentResourceCommandsText(app),
		UpdateCommands:   strings.Join(app.UpdateCommands, "\n"),
		AfterCommands:    strings.Join(app.AfterCommands, "\n"),
		HealthCommands:   strings.Join(app.HealthCommands, "\n"),
		RollbackCommands: strings.Join(app.RollbackCommands, "\n"),
	}
}

func deploymentResourceCommandsText(app config.DeploymentApp) string {
	if len(app.ResourceCommands) > 0 {
		return strings.Join(app.ResourceCommands, "\n")
	}
	return strings.Join(deploymentResourceDefaultCommands(app), "\n")
}

func deploymentAppWithResourceDefaults(app config.DeploymentApp) config.DeploymentApp {
	if len(app.ResourceCommands) == 0 {
		app.ResourceCommands = deploymentResourceDefaultCommands(app)
	}
	return app
}

func (m Model) deploymentAppFromForm() config.DeploymentApp {
	return config.DeploymentApp{
		Name:             strings.TrimSpace(m.deploymentForm.Name),
		Server:           strings.TrimSpace(m.deploymentForm.Server),
		Source:           strings.TrimSpace(m.deploymentForm.Source),
		FetchMode:        strings.TrimSpace(m.deploymentForm.FetchMode),
		Repo:             strings.TrimSpace(m.deploymentForm.Repo),
		Branch:           strings.TrimSpace(m.deploymentForm.Branch),
		Version:          strings.TrimSpace(m.deploymentForm.Version),
		Asset:            strings.TrimSpace(m.deploymentForm.Asset),
		Path:             strings.TrimSpace(m.deploymentForm.Path),
		ReleaseURL:       strings.TrimSpace(m.deploymentForm.ReleaseURL),
		Credential:       strings.TrimSpace(m.deploymentForm.Credential),
		CredentialName:   strings.TrimSpace(m.deploymentForm.CredentialName),
		WaitSeconds:      parseNonNegativeInt(m.deploymentForm.WaitSeconds),
		BeforeCommands:   splitCommandBlock(m.deploymentForm.BeforeCommands),
		ResourceCommands: splitCommandBlock(m.deploymentForm.ResourceCommands),
		UpdateCommands:   splitCommandBlock(m.deploymentForm.UpdateCommands),
		AfterCommands:    splitCommandBlock(m.deploymentForm.AfterCommands),
		HealthCommands:   splitCommandBlock(m.deploymentForm.HealthCommands),
		RollbackCommands: splitCommandBlock(m.deploymentForm.RollbackCommands),
	}
}

func splitCommandBlock(value string) []string {
	lines := []string{}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseNonNegativeInt(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (m Model) updateDeploymentEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := shortcutKey(msg)
	switch key {
	case "esc", "q", "ctrl+c":
		return m.startDeploymentList(m.activeDeployment.HostIndex), nil
	case "tab", "down":
		m.deploymentField = deploymentNextField(m.deploymentField, 1, m.deploymentForm.Source)
		m.deploymentCursor = m.deploymentValueLen()
	case "shift+tab", "up":
		m.deploymentField = deploymentNextField(m.deploymentField, -1, m.deploymentForm.Source)
		m.deploymentCursor = m.deploymentValueLen()
	case "left":
		if m.deploymentField == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentField == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentField == 2 {
			m.cycleDeploymentServer(-1)
		} else if m.deploymentField == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(-1)
		}
	case "right":
		if m.deploymentField == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentField == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentField == 2 {
			m.cycleDeploymentServer(1)
		} else if m.deploymentField == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(1)
		}
	case "ctrl+j":
		if deploymentFieldIsCommand(m.deploymentField) {
			m.deploymentAppend("\n")
		}
	case "enter":
		app := m.deploymentAppFromForm()
		if strings.TrimSpace(app.Server) == "" {
			m.status = "保存失败：部署服务器不能为空"
			return m, nil
		}
		if err := config.ValidateDeploymentApp(app); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		file := m.deploymentFile
		if m.deploymentEditing && m.deploymentEditIndex >= 0 && m.deploymentEditIndex < len(file.Apps) {
			file.Apps[m.deploymentEditIndex] = app
		} else {
			file.Apps = append(file.Apps, app)
		}
		if err := config.SaveDeployments(m.home, file); err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		m.deploymentFile = file
		m = m.startDeploymentList(m.activeDeployment.HostIndex)
		m.status = m.t("Deployment app saved.", "部署应用已保存。")
		return m, nil
	case "backspace":
		m.deploymentBackspace()
	default:
		if len(msg.Runes) > 0 && m.deploymentField != 0 && m.deploymentField != 1 && m.deploymentField != 2 && m.deploymentField != 10 {
			m.deploymentAppend(string(msg.Runes))
		}
	}
	return m, nil
}

func deploymentFieldCount() int { return 19 }

func deploymentFieldIsCommand(field int) bool { return field >= 13 }

func deploymentVisibleFields(source string) []int {
	if source == config.DeploySourceRelease {
		return []int{0, 1, 2, 3, 4, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
	}
	return []int{0, 1, 2, 3, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18}
}

func deploymentNextField(current int, delta int, source string) int {
	fields := deploymentVisibleFields(source)
	if len(fields) == 0 {
		return 0
	}
	pos := 0
	for i, field := range fields {
		if field == current {
			pos = i
			break
		}
	}
	return fields[moveIndex(pos, len(fields), delta)]
}

func (m *Model) toggleDeploymentSource() {
	if m.deploymentForm.Source == config.DeploySourceGit {
		m.deploymentForm.Source = config.DeploySourceRelease
	} else {
		m.deploymentForm.Source = config.DeploySourceGit
	}
}

func (m *Model) toggleDeploymentFetchMode() {
	if m.deploymentForm.FetchMode == config.DeployFetchRemote {
		m.deploymentForm.FetchMode = config.DeployFetchLocal
	} else {
		m.deploymentForm.FetchMode = config.DeployFetchRemote
	}
}

func (m *Model) toggleDeploymentCredential() {
	switch m.deploymentForm.Credential {
	case config.DeployCredentialNone:
		m.deploymentForm.Credential = config.DeployCredentialSSH
	case config.DeployCredentialSSH:
		m.deploymentForm.Credential = config.DeployCredentialToken
	default:
		m.deploymentForm.Credential = config.DeployCredentialNone
	}
}

func (m Model) deploymentValue() string {
	switch m.deploymentField {
	case 1:
		return ""
	case 2:
		return m.deploymentForm.Server
	case 3:
		return m.deploymentForm.Name
	case 4:
		return m.deploymentForm.Repo
	case 5:
		return m.deploymentForm.Branch
	case 6:
		return m.deploymentForm.Version
	case 7:
		return m.deploymentForm.Asset
	case 8:
		return m.deploymentForm.Path
	case 9:
		return m.deploymentForm.ReleaseURL
	case 11:
		return m.deploymentForm.CredentialName
	case 12:
		return m.deploymentForm.WaitSeconds
	case 13:
		return m.deploymentForm.BeforeCommands
	case 14:
		return m.deploymentForm.ResourceCommands
	case 15:
		return m.deploymentForm.UpdateCommands
	case 16:
		return m.deploymentForm.AfterCommands
	case 17:
		return m.deploymentForm.HealthCommands
	case 18:
		return m.deploymentForm.RollbackCommands
	default:
		return ""
	}
}

func (m Model) deploymentValueLen() int {
	return len([]rune(m.deploymentValue()))
}

func (m *Model) setDeploymentValue(value string) {
	switch m.deploymentField {
	case 2:
		m.deploymentForm.Server = value
	case 3:
		m.deploymentForm.Name = value
	case 4:
		m.deploymentForm.Repo = value
	case 5:
		m.deploymentForm.Branch = value
	case 6:
		m.deploymentForm.Version = value
	case 7:
		m.deploymentForm.Asset = value
	case 8:
		m.deploymentForm.Path = value
	case 9:
		m.deploymentForm.ReleaseURL = value
	case 11:
		m.deploymentForm.CredentialName = value
	case 12:
		m.deploymentForm.WaitSeconds = value
	case 13:
		m.deploymentForm.BeforeCommands = value
	case 14:
		m.deploymentForm.ResourceCommands = value
	case 15:
		m.deploymentForm.UpdateCommands = value
	case 16:
		m.deploymentForm.AfterCommands = value
	case 17:
		m.deploymentForm.HealthCommands = value
	case 18:
		m.deploymentForm.RollbackCommands = value
	}
}

func (m *Model) deploymentAppend(s string) {
	value := []rune(m.deploymentValue())
	m.deploymentCursor = clampInt(m.deploymentCursor, 0, len(value))
	insert := []rune(s)
	next := append([]rune{}, value[:m.deploymentCursor]...)
	next = append(next, insert...)
	next = append(next, value[m.deploymentCursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentCursor += len(insert)
}

func (m *Model) deploymentBackspace() {
	if m.deploymentField == 0 || m.deploymentField == 1 || m.deploymentField == 2 || m.deploymentField == 10 {
		return
	}
	value := []rune(m.deploymentValue())
	if m.deploymentCursor <= 0 || len(value) == 0 {
		return
	}
	m.deploymentCursor = clampInt(m.deploymentCursor, 0, len(value))
	next := append([]rune{}, value[:m.deploymentCursor-1]...)
	next = append(next, value[m.deploymentCursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentCursor--
}

func (m *Model) moveDeploymentCursor(delta int) {
	m.deploymentCursor = clampInt(m.deploymentCursor+delta, 0, m.deploymentValueLen())
}
