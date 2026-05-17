package tui

import (
	"github.com/YaMaiDay/sshm/internal/config"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	command, ok := resourceActionCommandName(action)
	if !ok {
		return ""
	}
	return resourceservice.ActionScript(configResourceKind(kind), command, name)
}

func resourceActionCommandName(action resourceActionKind) (string, bool) {
	switch action {
	case resourceActionStart:
		return "start", true
	case resourceActionStop:
		return "stop", true
	case resourceActionRestart:
		return "restart", true
	default:
		return "", false
	}
}

func (m Model) resourceActionScript(kind resourceKind, action resourceActionKind, name string) string {
	command, ok := resourceActionCommandName(action)
	if !ok {
		return ""
	}
	managed, _ := m.managedResource(kind, name)
	return resourceservice.ManagedActionScript(configResourceKind(kind), command, name, managed)
}

func resourceLogScript(kind resourceKind, name string, lines int) string {
	return resourceservice.LogScript(configResourceKind(kind), name, lines)
}

func (m Model) resourceLogScript(kind resourceKind, name string, lines int) string {
	managed, _ := m.managedResource(kind, name)
	return resourceservice.ManagedLogScript(configResourceKind(kind), name, lines, managed)
}

func (m Model) managedResource(kind resourceKind, name string) (config.ManagedResource, bool) {
	server := m.resourceServerKey(m.resourceHostIndex)
	configKind := configResourceKind(kind)
	if server == "" || configKind == "" {
		return config.ManagedResource{}, false
	}
	for _, item := range m.resourceFile.Items {
		if item.Server == server && item.Kind == configKind && item.Name == name && item.Added {
			if item.Kind == config.ResourceKindDatabase && !managedDatabaseResourceConfigured(item) {
				continue
			}
			return item, true
		}
	}
	return config.ManagedResource{}, false
}

func sudoFallbackScript(command string, sudoCommand string) string {
	return resourceservice.SudoFallbackScript(command, sudoCommand)
}

func shellQuoteLocal(value string) string {
	return resourceservice.QuoteShell(value)
}
