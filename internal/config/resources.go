package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	ResourceKindService   = "service"
	ResourceKindContainer = "container"
	ResourceKindProcess   = "process"
	ResourceKindPort      = "port"
)

type ResourcesFile struct {
	Items []ManagedResource `toml:"items"`
}

type ManagedResource struct {
	Server         string `toml:"server"`
	Kind           string `toml:"kind"`
	Name           string `toml:"name"`
	Favorite       bool   `toml:"favorite,omitempty"`
	Pinned         bool   `toml:"pinned,omitempty"`
	PinnedOrder    int64  `toml:"pinned_order,omitempty"`
	StartCommand   string `toml:"start_command,omitempty"`
	StopCommand    string `toml:"stop_command,omitempty"`
	RestartCommand string `toml:"restart_command,omitempty"`
	DeleteCommand  string `toml:"delete_command,omitempty"`
	LogCommand     string `toml:"log_command,omitempty"`
	HealthCommand  string `toml:"health_command,omitempty"`
}

func ResourcesPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "resources.toml")
}

func LoadResources(home string) (ResourcesFile, bool, error) {
	path := ResourcesPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ResourcesFile{}, false, nil
		}
		return ResourcesFile{}, false, err
	}
	var file ResourcesFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return ResourcesFile{}, true, err
	}
	file.Items = NormalizeManagedResources(file.Items)
	return file, true, nil
}

func SaveResources(home string, file ResourcesFile) error {
	path := ResourcesPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Items = NormalizeManagedResources(file.Items)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func NormalizeManagedResources(items []ManagedResource) []ManagedResource {
	out := make([]ManagedResource, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item.Server = strings.TrimSpace(item.Server)
		item.Kind = normalizeResourceKind(item.Kind)
		item.Name = strings.TrimSpace(item.Name)
		item.StartCommand = strings.TrimSpace(item.StartCommand)
		item.StopCommand = strings.TrimSpace(item.StopCommand)
		item.RestartCommand = strings.TrimSpace(item.RestartCommand)
		item.DeleteCommand = strings.TrimSpace(item.DeleteCommand)
		item.LogCommand = strings.TrimSpace(item.LogCommand)
		item.HealthCommand = strings.TrimSpace(item.HealthCommand)
		if !item.Pinned {
			item.PinnedOrder = 0
		}
		if item.Server == "" || item.Kind == "" || item.Name == "" {
			continue
		}
		key := item.Server + "\x00" + item.Kind + "\x00" + item.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func normalizeResourceKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ResourceKindService, "services":
		return ResourceKindService
	case ResourceKindContainer, "containers", "docker":
		return ResourceKindContainer
	case ResourceKindProcess, "processes", "proc":
		return ResourceKindProcess
	case ResourceKindPort, "ports":
		return ResourceKindPort
	default:
		return ""
	}
}
