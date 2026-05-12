package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/YaMaiDay/sshm/internal/host"
)

type AppState struct {
	Servers []ServerState `toml:"servers,omitempty"`
}

type ServerState struct {
	Category  string `toml:"category"`
	Name      string `toml:"name"`
	LastLogin string `toml:"last_login,omitempty"`
}

func StatePath(home string) string {
	return filepath.Join(home, ".config", "sshm", "state.toml")
}

func LoadState(home string) AppState {
	state := AppState{}
	data, err := os.ReadFile(StatePath(home))
	if err != nil {
		return state
	}
	_ = toml.Unmarshal(data, &state)
	return state
}

func SaveState(home string, state AppState) error {
	path := StatePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := toml.Marshal(state)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func ServerStateKey(h host.Host) string {
	category := strings.TrimSpace(h.Category)
	if category == "" {
		category = "default"
	}
	return category + "/" + strings.TrimSpace(h.Name)
}

func LastLoginFor(state AppState, h host.Host) time.Time {
	category := normalizedStateCategory(h.Category)
	name := strings.TrimSpace(h.Name)
	for _, server := range state.Servers {
		if strings.TrimSpace(server.Category) == category && strings.TrimSpace(server.Name) == name {
			return parseStateTime(server.LastLogin)
		}
	}
	return time.Time{}
}

func SetLastLogin(state *AppState, h host.Host, at time.Time) {
	category := normalizedStateCategory(h.Category)
	name := strings.TrimSpace(h.Name)
	value := at.UTC().Format(time.RFC3339)
	for i := range state.Servers {
		if strings.TrimSpace(state.Servers[i].Category) == category && strings.TrimSpace(state.Servers[i].Name) == name {
			state.Servers[i].LastLogin = value
			return
		}
	}
	state.Servers = append(state.Servers, ServerState{
		Category:  category,
		Name:      name,
		LastLogin: value,
	})
}

func normalizedStateCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return "default"
	}
	return category
}

func parseStateTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
