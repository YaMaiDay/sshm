package tui

import (
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/remotescript"
	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) managedResourcesForServer(server string) []config.ManagedResource {
	items := []config.ManagedResource{}
	for _, item := range m.resourceState.File.Items {
		if item.Server == server {
			items = append(items, item)
		}
	}
	return items
}

func findManagedResource(items []config.ManagedResource, server string, kind string, name string) int {
	for i, item := range items {
		if item.Server == server && item.Kind == kind && item.Name == name {
			return i
		}
	}
	return -1
}

func configResourceKind(kind resourceKind) string {
	switch kind {
	case resourceServices:
		return config.ResourceKindService
	case resourceContainers:
		return config.ResourceKindContainer
	case resourceProcesses:
		return config.ResourceKindProcess
	case resourcePorts:
		return config.ResourceKindPort
	case resourceDatabases:
		return config.ResourceKindDatabase
	default:
		return ""
	}
}

func resourceKindFromConfig(kind string) resourceKind {
	switch kind {
	case config.ResourceKindService:
		return resourceServices
	case config.ResourceKindContainer:
		return resourceContainers
	case config.ResourceKindProcess:
		return resourceProcesses
	case config.ResourceKindPort:
		return resourcePorts
	case config.ResourceKindDatabase:
		return resourceDatabases
	default:
		return resourceAll
	}
}

func defaultManagedResource(server string, kind string, name string) config.ManagedResource {
	item := config.ManagedResource{Server: server, Kind: kind, Name: name}
	switch kind {
	case config.ResourceKindService:
		target := remotescript.Quote(name)
		item.StartCommand = "systemctl start " + target
		item.StopCommand = "systemctl stop " + target
		item.RestartCommand = "systemctl restart " + target
		item.LogCommand = "journalctl -u " + target + " -n 200 --no-pager"
	case config.ResourceKindContainer:
		target := remotescript.Quote(name)
		item.StartCommand = "docker start " + target
		item.StopCommand = "docker stop " + target
		item.RestartCommand = "docker restart " + target
		item.LogCommand = "docker logs --tail 200 " + target
	case config.ResourceKindPort:
		_, port := splitManagedPortName(name)
		item.HealthCommand = "curl -f http://127.0.0.1:" + remotescript.Quote(port) + "/health"
	case config.ResourceKindDatabase:
		item.DBEngine = "MySQL"
		item.DBHost = "127.0.0.1"
		item.DBPort = "3306"
		item.DBUser = "root"
	}
	return item
}

func defaultDatabaseManagedResource(server string, db resourceservice.DatabaseDetail) config.ManagedResource {
	item := config.ManagedResource{Server: server, Kind: config.ResourceKindDatabase, Added: true}
	item.DBEngine = firstNonEmpty(resourceservice.NormalizeDatabaseEngine(db.Engine), "MySQL")
	item.DBHost = databaseDefaultHost(db)
	item.DBPort = databaseDefaultPortForDetail(db)
	item.DBUser = resourceservice.DatabaseDefaultUser(item.DBEngine)
	item.DBName = resourceservice.DatabaseDefaultName(item.DBEngine)
	item.Name = firstNonEmpty(item.DBName, db.Name)
	item.DBInstance = strings.TrimSpace(db.Name)
	return item
}

func mergeDatabaseDiscoveredDefaults(item config.ManagedResource, defaults config.ManagedResource) config.ManagedResource {
	if strings.TrimSpace(item.DBEngine) == "" || databaseConnectionLooksGenericDefault(item, defaults) {
		item.DBEngine = defaults.DBEngine
		item.DBHost = defaults.DBHost
		item.DBPort = defaults.DBPort
		item.DBUser = defaults.DBUser
		item.DBName = defaults.DBName
		item.DBInstance = defaults.DBInstance
		return item
	}
	if strings.TrimSpace(item.DBHost) == "" {
		item.DBHost = defaults.DBHost
	}
	if strings.TrimSpace(item.DBPort) == "" {
		item.DBPort = defaults.DBPort
	}
	if strings.TrimSpace(item.DBUser) == "" {
		item.DBUser = defaults.DBUser
	}
	if strings.TrimSpace(item.DBName) == "" {
		item.DBName = defaults.DBName
	}
	if strings.TrimSpace(item.DBInstance) == "" {
		item.DBInstance = defaults.DBInstance
	}
	return item
}

func databaseConnectionLooksGenericDefault(item config.ManagedResource, defaults config.ManagedResource) bool {
	if strings.EqualFold(strings.TrimSpace(item.DBEngine), strings.TrimSpace(defaults.DBEngine)) {
		return false
	}
	generic := defaultManagedResource(item.Server, config.ResourceKindDatabase, item.Name)
	return strings.EqualFold(strings.TrimSpace(item.DBEngine), strings.TrimSpace(generic.DBEngine)) &&
		strings.TrimSpace(item.DBHost) == strings.TrimSpace(generic.DBHost) &&
		strings.TrimSpace(item.DBPort) == strings.TrimSpace(generic.DBPort) &&
		strings.TrimSpace(item.DBUser) == strings.TrimSpace(generic.DBUser) &&
		strings.TrimSpace(item.DBName) == strings.TrimSpace(generic.DBName)
}

func databaseManagedEndpoint(item config.ManagedResource) string {
	host := strings.TrimSpace(item.DBHost)
	port := strings.TrimSpace(item.DBPort)
	if host == "" {
		return ""
	}
	if port == "" {
		return host
	}
	return host + ":" + port
}

func containerResourceHasCustomConfig(item config.ManagedResource) bool {
	if item.Kind != config.ResourceKindContainer {
		return false
	}
	defaults := defaultManagedResource(item.Server, item.Kind, item.Name)
	return strings.TrimSpace(item.StartCommand) != "" && strings.TrimSpace(item.StartCommand) != defaults.StartCommand ||
		strings.TrimSpace(item.StopCommand) != "" && strings.TrimSpace(item.StopCommand) != defaults.StopCommand ||
		strings.TrimSpace(item.RestartCommand) != "" && strings.TrimSpace(item.RestartCommand) != defaults.RestartCommand ||
		strings.TrimSpace(item.LogCommand) != "" && strings.TrimSpace(item.LogCommand) != defaults.LogCommand ||
		strings.TrimSpace(item.HealthCommand) != "" ||
		strings.TrimSpace(item.DeleteCommand) != ""
}

func containerDetailsToCache(items []resourceservice.ContainerDetail) []config.ResourceContainerCache {
	out := make([]config.ResourceContainerCache, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" || item.Missing {
			continue
		}
		out = append(out, config.ResourceContainerCache{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func containerDetailsFromCache(items []config.ResourceContainerCache) []resourceservice.ContainerDetail {
	out := make([]resourceservice.ContainerDetail, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		out = append(out, resourceservice.ContainerDetail{
			Name:    item.Name,
			Image:   item.Image,
			Status:  item.Status,
			Ports:   item.Ports,
			CPU:     item.CPU,
			Memory:  item.Memory,
			MemPerc: item.MemPerc,
		})
	}
	return out
}

func splitManagedPortName(name string) (string, string) {
	name = strings.TrimSpace(name)
	proto, port, ok := strings.Cut(name, "/")
	if !ok {
		return "tcp", name
	}
	proto = strings.TrimSpace(proto)
	port = strings.TrimSpace(port)
	if proto == "" {
		proto = "tcp"
	}
	return proto, port
}
