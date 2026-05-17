package tui

import (
	"fmt"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) resourceActionNameText(action resourceActionKind) string {
	switch action {
	case resourceActionStart:
		return m.t("Start", "启动")
	case resourceActionStop:
		return m.t("Stop", "停止")
	case resourceActionRestart:
		return m.t("Restart", "重启")
	case resourceActionDelete:
		return m.t("Disabled", "已禁用")
	default:
		return "-"
	}
}

func (m Model) resourceActionErrorText(result commandResult) string {
	if strings.Contains(result.Output, "__SSHM_PERMISSION_DENIED__") {
		return m.t("Permission denied. Configure sudo permissions or run with an allowed account.", "权限不够。请配置 sudo 权限或使用有权限的账号。")
	}
	return fmt.Sprintf("%s %d", m.t("Resource action failed, exit", "资源操作失败，退出码"), result.ExitCode)
}

func resourceActionCommandPreview(kind resourceKind, action resourceActionKind, name string) string {
	command, ok := resourceActionCommandName(action)
	if !ok {
		return "-"
	}
	return resourceservice.ActionPreview(configResourceKind(kind), command, name, config.ManagedResource{})
}

func (m Model) resourceCommandPreview(kind resourceKind, action resourceActionKind, name string) string {
	command, ok := resourceActionCommandName(action)
	if !ok {
		return "-"
	}
	managed, _ := m.managedResource(kind, name)
	return resourceservice.ActionPreview(configResourceKind(kind), command, name, managed)
}

func (m Model) resourceLogCommandPreview(kind resourceKind, name string, lines int) string {
	managed, _ := m.managedResource(kind, name)
	return resourceservice.LogPreview(configResourceKind(kind), name, lines, managed)
}
