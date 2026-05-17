package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/YaMaiDay/sshm/internal/config"
	deploymentservice "github.com/YaMaiDay/sshm/internal/deployment"
)

func (m Model) defaultDeploymentServer() string {
	if m.deploymentState.Active.HostIndex >= 0 && m.deploymentState.Active.HostIndex < len(m.states) {
		h := m.states[m.deploymentState.Active.HostIndex].Host
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
		m.deploymentState.Form.Server = ""
		return
	}
	index := m.deploymentServerIndex(m.deploymentState.Form.Server)
	if index < 0 {
		index = 0
	} else {
		index = moveIndex(index, len(m.states), delta)
	}
	h := m.states[index].Host
	m.deploymentState.Form.Server = config.ServerCommandKey(h.Category, h.Name)
}

func (m Model) startDeploymentEdit(app config.DeploymentApp, editing bool) Model {
	if !editing {
		app.Source = config.DeploySourceGit
		app.FetchMode = config.DeployFetchLocal
		app.Credential = config.DeployCredentialSSH
		app.Branch = "main"
		app.Server = m.defaultDeploymentServer()
	}
	m.deploymentState.Form = deploymentFormFromApp(app)
	m.deploymentState.Field = 0
	m.deploymentState.Cursor = len([]rune(m.deploymentState.Form.Name))
	m.deploymentState.Editing = editing
	m.deploymentState.EditIndex = -1
	if editing {
		if item, ok := m.selectedDeploymentItem(); ok {
			m.deploymentState.EditIndex = item.Index
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
	return strings.Join(deploymentservice.ResourceDefaultCommands(app), "\n")
}

func deploymentAppWithResourceDefaults(app config.DeploymentApp) config.DeploymentApp {
	if len(app.ResourceCommands) == 0 {
		app.ResourceCommands = deploymentservice.ResourceDefaultCommands(app)
	}
	return app
}

func (m Model) deploymentAppFromForm() config.DeploymentApp {
	return config.DeploymentApp{
		Name:             strings.TrimSpace(m.deploymentState.Form.Name),
		Server:           strings.TrimSpace(m.deploymentState.Form.Server),
		Source:           strings.TrimSpace(m.deploymentState.Form.Source),
		FetchMode:        strings.TrimSpace(m.deploymentState.Form.FetchMode),
		Repo:             strings.TrimSpace(m.deploymentState.Form.Repo),
		Branch:           strings.TrimSpace(m.deploymentState.Form.Branch),
		Version:          strings.TrimSpace(m.deploymentState.Form.Version),
		Asset:            strings.TrimSpace(m.deploymentState.Form.Asset),
		Path:             strings.TrimSpace(m.deploymentState.Form.Path),
		ReleaseURL:       strings.TrimSpace(m.deploymentState.Form.ReleaseURL),
		Credential:       strings.TrimSpace(m.deploymentState.Form.Credential),
		CredentialName:   strings.TrimSpace(m.deploymentState.Form.CredentialName),
		WaitSeconds:      parseNonNegativeInt(m.deploymentState.Form.WaitSeconds),
		BeforeCommands:   splitCommandBlock(m.deploymentState.Form.BeforeCommands),
		ResourceCommands: splitCommandBlock(m.deploymentState.Form.ResourceCommands),
		UpdateCommands:   splitCommandBlock(m.deploymentState.Form.UpdateCommands),
		AfterCommands:    splitCommandBlock(m.deploymentState.Form.AfterCommands),
		HealthCommands:   splitCommandBlock(m.deploymentState.Form.HealthCommands),
		RollbackCommands: splitCommandBlock(m.deploymentState.Form.RollbackCommands),
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
	case "esc", "ctrl+c":
		return m.startDeploymentList(m.deploymentState.Active.HostIndex), nil
	case "tab", "down":
		m.deploymentState.Field = deploymentNextField(m.deploymentState.Field, 1, m.deploymentState.Form.Source)
		m.deploymentState.Cursor = m.deploymentValueLen()
	case "shift+tab", "up":
		m.deploymentState.Field = deploymentNextField(m.deploymentState.Field, -1, m.deploymentState.Form.Source)
		m.deploymentState.Cursor = m.deploymentValueLen()
	case "left":
		if m.deploymentState.Field == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentState.Field == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentState.Field == 2 {
			m.cycleDeploymentServer(-1)
		} else if m.deploymentState.Field == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(-1)
		}
	case "right":
		if m.deploymentState.Field == 0 {
			m.toggleDeploymentSource()
		} else if m.deploymentState.Field == 1 {
			m.toggleDeploymentFetchMode()
		} else if m.deploymentState.Field == 2 {
			m.cycleDeploymentServer(1)
		} else if m.deploymentState.Field == 10 {
			m.toggleDeploymentCredential()
		} else {
			m.moveDeploymentCursor(1)
		}
	case "ctrl+j":
		if deploymentFieldIsCommand(m.deploymentState.Field) {
			m.deploymentAppend("\n")
		}
	case "enter":
		app := m.deploymentAppFromForm()
		if strings.TrimSpace(app.Server) == "" {
			m.status = "保存失败：部署服务器不能为空"
			return m, nil
		}
		index := -1
		if m.deploymentState.Editing && m.deploymentState.EditIndex >= 0 && m.deploymentState.EditIndex < len(m.deploymentState.File.Apps) {
			index = m.deploymentState.EditIndex
		}
		file, err := deploymentservice.SaveApp(m.home, m.deploymentState.File, index, app)
		if err != nil {
			m.status = "保存失败：" + err.Error()
			return m, nil
		}
		m.deploymentState.File = file
		m = m.startDeploymentList(m.deploymentState.Active.HostIndex)
		m.status = m.t("Deployment app saved.", "部署应用已保存。")
		return m, nil
	case "backspace":
		m.deploymentBackspace()
	default:
		if len(msg.Runes) > 0 && m.deploymentState.Field != 0 && m.deploymentState.Field != 1 && m.deploymentState.Field != 2 && m.deploymentState.Field != 10 {
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
	if m.deploymentState.Form.Source == config.DeploySourceGit {
		m.deploymentState.Form.Source = config.DeploySourceRelease
	} else {
		m.deploymentState.Form.Source = config.DeploySourceGit
	}
}

func (m *Model) toggleDeploymentFetchMode() {
	if m.deploymentState.Form.FetchMode == config.DeployFetchRemote {
		m.deploymentState.Form.FetchMode = config.DeployFetchLocal
	} else {
		m.deploymentState.Form.FetchMode = config.DeployFetchRemote
	}
}

func (m *Model) toggleDeploymentCredential() {
	switch m.deploymentState.Form.Credential {
	case config.DeployCredentialNone:
		m.deploymentState.Form.Credential = config.DeployCredentialSSH
	case config.DeployCredentialSSH:
		m.deploymentState.Form.Credential = config.DeployCredentialToken
	default:
		m.deploymentState.Form.Credential = config.DeployCredentialNone
	}
}

func (m Model) deploymentValue() string {
	switch m.deploymentState.Field {
	case 1:
		return ""
	case 2:
		return m.deploymentState.Form.Server
	case 3:
		return m.deploymentState.Form.Name
	case 4:
		return m.deploymentState.Form.Repo
	case 5:
		return m.deploymentState.Form.Branch
	case 6:
		return m.deploymentState.Form.Version
	case 7:
		return m.deploymentState.Form.Asset
	case 8:
		return m.deploymentState.Form.Path
	case 9:
		return m.deploymentState.Form.ReleaseURL
	case 11:
		return m.deploymentState.Form.CredentialName
	case 12:
		return m.deploymentState.Form.WaitSeconds
	case 13:
		return m.deploymentState.Form.BeforeCommands
	case 14:
		return m.deploymentState.Form.ResourceCommands
	case 15:
		return m.deploymentState.Form.UpdateCommands
	case 16:
		return m.deploymentState.Form.AfterCommands
	case 17:
		return m.deploymentState.Form.HealthCommands
	case 18:
		return m.deploymentState.Form.RollbackCommands
	default:
		return ""
	}
}

func (m Model) deploymentValueLen() int {
	return len([]rune(m.deploymentValue()))
}

func (m *Model) setDeploymentValue(value string) {
	switch m.deploymentState.Field {
	case 2:
		m.deploymentState.Form.Server = value
	case 3:
		m.deploymentState.Form.Name = value
	case 4:
		m.deploymentState.Form.Repo = value
	case 5:
		m.deploymentState.Form.Branch = value
	case 6:
		m.deploymentState.Form.Version = value
	case 7:
		m.deploymentState.Form.Asset = value
	case 8:
		m.deploymentState.Form.Path = value
	case 9:
		m.deploymentState.Form.ReleaseURL = value
	case 11:
		m.deploymentState.Form.CredentialName = value
	case 12:
		m.deploymentState.Form.WaitSeconds = value
	case 13:
		m.deploymentState.Form.BeforeCommands = value
	case 14:
		m.deploymentState.Form.ResourceCommands = value
	case 15:
		m.deploymentState.Form.UpdateCommands = value
	case 16:
		m.deploymentState.Form.AfterCommands = value
	case 17:
		m.deploymentState.Form.HealthCommands = value
	case 18:
		m.deploymentState.Form.RollbackCommands = value
	}
}

func (m *Model) deploymentAppend(s string) {
	value := []rune(m.deploymentValue())
	m.deploymentState.Cursor = clampInt(m.deploymentState.Cursor, 0, len(value))
	insert := []rune(s)
	next := append([]rune{}, value[:m.deploymentState.Cursor]...)
	next = append(next, insert...)
	next = append(next, value[m.deploymentState.Cursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentState.Cursor += len(insert)
}

func (m *Model) deploymentBackspace() {
	if m.deploymentState.Field == 0 || m.deploymentState.Field == 1 || m.deploymentState.Field == 2 || m.deploymentState.Field == 10 {
		return
	}
	value := []rune(m.deploymentValue())
	if m.deploymentState.Cursor <= 0 || len(value) == 0 {
		return
	}
	m.deploymentState.Cursor = clampInt(m.deploymentState.Cursor, 0, len(value))
	next := append([]rune{}, value[:m.deploymentState.Cursor-1]...)
	next = append(next, value[m.deploymentState.Cursor:]...)
	m.setDeploymentValue(string(next))
	m.deploymentState.Cursor--
}

func (m *Model) moveDeploymentCursor(delta int) {
	m.deploymentState.Cursor = clampInt(m.deploymentState.Cursor+delta, 0, m.deploymentValueLen())
}
