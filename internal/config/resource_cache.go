package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type ResourceCacheFile struct {
	Servers []ResourceServerCache `toml:"servers"`
}

type ResourceServerCache struct {
	Server     string                   `toml:"server"`
	UpdatedAt  string                   `toml:"updated_at,omitempty"`
	Containers []ResourceContainerCache `toml:"containers,omitempty"`
}

type ResourceContainerCache struct {
	Name    string `toml:"name"`
	Image   string `toml:"image,omitempty"`
	Status  string `toml:"status,omitempty"`
	Ports   string `toml:"ports,omitempty"`
	CPU     string `toml:"cpu,omitempty"`
	Memory  string `toml:"memory,omitempty"`
	MemPerc string `toml:"mem_perc,omitempty"`
}

func ResourceCachePath(home string) string {
	return filepath.Join(home, ".config", "sshm", "resource_cache.toml")
}

func LoadResourceCache(home string) (ResourceCacheFile, bool, error) {
	path := ResourceCachePath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ResourceCacheFile{}, false, nil
		}
		return ResourceCacheFile{}, false, err
	}
	var file ResourceCacheFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return ResourceCacheFile{}, true, err
	}
	file.Servers = normalizeResourceServerCaches(file.Servers)
	return file, true, nil
}

func SaveResourceCache(home string, file ResourceCacheFile) error {
	path := ResourceCachePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Servers = normalizeResourceServerCaches(file.Servers)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func UpsertResourceContainerCache(home string, server string, containers []ResourceContainerCache, at time.Time) error {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil
	}
	file, _, err := LoadResourceCache(home)
	if err != nil {
		return err
	}
	entry := ResourceServerCache{Server: server, UpdatedAt: at.Format(time.RFC3339), Containers: normalizeResourceContainerCaches(containers)}
	replaced := false
	for i := range file.Servers {
		if file.Servers[i].Server == server {
			file.Servers[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		file.Servers = append(file.Servers, entry)
	}
	return SaveResourceCache(home, file)
}

func ResourceContainerCacheForServer(file ResourceCacheFile, server string) ([]ResourceContainerCache, bool) {
	server = strings.TrimSpace(server)
	for _, entry := range file.Servers {
		if entry.Server == server && len(entry.Containers) > 0 {
			return append([]ResourceContainerCache(nil), entry.Containers...), true
		}
	}
	return nil, false
}

func normalizeResourceServerCaches(items []ResourceServerCache) []ResourceServerCache {
	out := make([]ResourceServerCache, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item.Server = strings.TrimSpace(item.Server)
		if item.Server == "" || seen[item.Server] {
			continue
		}
		seen[item.Server] = true
		item.Containers = normalizeResourceContainerCaches(item.Containers)
		out = append(out, item)
	}
	return out
}

func normalizeResourceContainerCaches(items []ResourceContainerCache) []ResourceContainerCache {
	out := make([]ResourceContainerCache, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		item.Image = strings.TrimSpace(item.Image)
		item.Status = strings.TrimSpace(item.Status)
		item.Ports = strings.TrimSpace(item.Ports)
		item.CPU = strings.TrimSpace(item.CPU)
		item.Memory = strings.TrimSpace(item.Memory)
		item.MemPerc = strings.TrimSpace(item.MemPerc)
		if item.Name == "" || seen[item.Name] {
			continue
		}
		seen[item.Name] = true
		out = append(out, item)
	}
	return out
}
