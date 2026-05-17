package resource

import (
	"time"

	"github.com/YaMaiDay/sshm/internal/config"
)

func LoadConfig(home string) (config.ResourcesFile, bool, error) {
	return config.LoadResources(home)
}

func SaveConfig(home string, file config.ResourcesFile) error {
	return config.SaveResources(home, file)
}

func LoadCache(home string) (config.ResourceCacheFile, bool, error) {
	return config.LoadResourceCache(home)
}

func UpsertContainerCache(home string, server string, items []config.ResourceContainerCache, at time.Time) error {
	return config.UpsertResourceContainerCache(home, server, items, at)
}
