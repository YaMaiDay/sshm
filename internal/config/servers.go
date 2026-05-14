package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/YaMaiDay/sshm/internal/host"
)

type serversFile struct {
	Categories []string      `toml:"categories"`
	Servers    []serverEntry `toml:"servers"`
}

type serverEntry struct {
	Category    string `toml:"category"`
	Name        string `toml:"name"`
	Host        string `toml:"host"`
	User        string `toml:"user"`
	Port        int    `toml:"port"`
	KeyPath     string `toml:"key_path"`
	Password    string `toml:"password"`
	JumpHostRef string `toml:"jump_host_ref,omitempty"`
	Note        string `toml:"note,omitempty"`
	ExpireAt    string `toml:"expire_at,omitempty"`
	Favorite    bool   `toml:"favorite,omitempty"`
	Pinned      bool   `toml:"pinned,omitempty"`
	PinnedOrder int64  `toml:"pinned_order,omitempty"`
	HealthPorts []int  `toml:"health_ports,omitempty"`
}

const BastionCategory = "跳板机"

func ServersPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "servers.toml")
}

func LoadServerHosts(home string) ([]host.Host, bool, error) {
	path := ServersPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var file serversFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, true, err
	}
	hosts := make([]host.Host, 0, len(file.Servers))
	for _, entry := range file.Servers {
		port := "22"
		if entry.Port > 0 {
			port = strconv.Itoa(entry.Port)
		}
		password := strings.TrimSpace(entry.Password)
		category := strings.TrimSpace(entry.Category)
		if category == "" {
			category = "default"
		}
		hosts = append(hosts, host.Host{
			Name:         strings.TrimSpace(entry.Name),
			HostName:     strings.TrimSpace(entry.Host),
			User:         strings.TrimSpace(entry.User),
			Port:         port,
			IdentityFile: strings.TrimSpace(entry.KeyPath),
			Password:     password,
			JumpHostRef:  strings.TrimSpace(entry.JumpHostRef),
			Category:     category,
			Note:         strings.TrimSpace(entry.Note),
			ExpireAt:     strings.TrimSpace(entry.ExpireAt),
			Favorite:     entry.Favorite,
			Pinned:       entry.Pinned,
			PinnedOrder:  entry.PinnedOrder,
			HealthPorts:  normalizeHealthPorts(entry.HealthPorts),
			File:         path,
			HasPassword:  password != "",
		})
	}
	return resolveBastionRefs(hosts), true, nil
}

func SaveServerHosts(home string, hosts []host.Host) error {
	categories, _, _ := LoadCategories(home)
	return SaveServerData(home, categoriesFromHosts(categories, hosts), hosts)
}

func LoadCategories(home string) ([]string, bool, error) {
	path := ServersPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{BastionCategory, "default"}, false, nil
		}
		return nil, false, err
	}
	var file serversFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, true, err
	}
	return normalizeCategories(file.Categories, file.Servers), true, nil
}

func SaveCategories(home string, categories []string) error {
	hosts, _, err := LoadServerHosts(home)
	if err != nil {
		return err
	}
	return SaveServerData(home, normalizeCategoryNames(categories), hosts)
}

func AddCategory(home, name string) error {
	categories, _, err := LoadCategories(home)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == BastionCategory {
		return os.ErrPermission
	}
	if name == "" {
		return os.ErrInvalid
	}
	for _, category := range categories {
		if category == name {
			return nil
		}
	}
	return SaveCategories(home, append(categories, name))
}

func DeleteCategory(home, name string) error {
	name = strings.TrimSpace(name)
	categories, _, err := LoadCategories(home)
	if err != nil {
		return err
	}
	nonSystemCategories := 0
	for _, category := range categories {
		if category != BastionCategory {
			nonSystemCategories++
		}
	}
	if nonSystemCategories <= 1 && name != BastionCategory {
		return os.ErrInvalid
	}
	hosts, _, err := LoadServerHosts(home)
	if err != nil {
		return err
	}
	for _, h := range hosts {
		if h.Category == name {
			return os.ErrPermission
		}
	}
	next := make([]string, 0, len(categories)-1)
	for _, category := range categories {
		if category != name {
			next = append(next, category)
		}
	}
	if len(next) == len(categories) {
		return nil
	}
	return SaveServerData(home, next, hosts)
}

func RenameCategory(home, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return os.ErrInvalid
	}
	if oldName == BastionCategory || newName == BastionCategory {
		return os.ErrPermission
	}
	categories, _, err := LoadCategories(home)
	if err != nil {
		return err
	}
	found := false
	for _, category := range categories {
		if category == newName {
			return os.ErrExist
		}
		if category == oldName {
			found = true
		}
	}
	if !found {
		return os.ErrNotExist
	}
	hosts, _, err := LoadServerHosts(home)
	if err != nil {
		return err
	}
	for i, category := range categories {
		if category == oldName {
			categories[i] = newName
		}
	}
	for i := range hosts {
		if hosts[i].Category == oldName {
			hosts[i].Category = newName
		}
	}
	return SaveServerData(home, categories, hosts)
}

func SaveServerData(home string, categories []string, hosts []host.Host) error {
	path := ServersPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file := serversFile{
		Categories: normalizeCategoryNames(categories),
		Servers:    make([]serverEntry, 0, len(hosts)),
	}
	for _, h := range hosts {
		port, _ := strconv.Atoi(strings.TrimSpace(h.Port))
		if port <= 0 {
			port = 22
		}
		category := strings.TrimSpace(h.Category)
		if category == "" {
			category = "default"
		}
		file.Servers = append(file.Servers, serverEntry{
			Category:    category,
			Name:        strings.TrimSpace(h.Name),
			Host:        strings.TrimSpace(h.HostName),
			User:        strings.TrimSpace(h.User),
			Port:        port,
			KeyPath:     strings.TrimSpace(h.IdentityFile),
			Password:    strings.TrimSpace(h.Password),
			JumpHostRef: strings.TrimSpace(h.JumpHostRef),
			Note:        strings.TrimSpace(h.Note),
			ExpireAt:    strings.TrimSpace(h.ExpireAt),
			Favorite:    h.Favorite,
			Pinned:      h.Pinned,
			PinnedOrder: h.PinnedOrder,
			HealthPorts: normalizeHealthPorts(h.HealthPorts),
		})
	}
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func normalizeCategories(categories []string, servers []serverEntry) []string {
	out := normalizeCategoryNames(categories)
	if !containsCategory(out, BastionCategory) {
		out = append([]string{BastionCategory}, out...)
	}
	for _, server := range servers {
		category := strings.TrimSpace(server.Category)
		if category == "" {
			category = "default"
		}
		exists := false
		for _, current := range out {
			if current == category {
				exists = true
				break
			}
		}
		if !exists {
			out = append(out, category)
		}
	}
	if len(out) == 0 {
		out = []string{BastionCategory, "default"}
	}
	return out
}

func normalizeCategoryNames(categories []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(categories)+1)
	for _, category := range categories {
		category = strings.TrimSpace(category)
		if category == "" || seen[category] {
			continue
		}
		seen[category] = true
		out = append(out, category)
	}
	if len(out) == 0 {
		out = append(out, BastionCategory, "default")
	} else if !seen[BastionCategory] {
		out = append([]string{BastionCategory}, out...)
	}
	return out
}

func containsCategory(categories []string, name string) bool {
	for _, category := range categories {
		if category == name {
			return true
		}
	}
	return false
}

func resolveBastionRefs(hosts []host.Host) []host.Host {
	bastions := map[string]host.Host{}
	for _, h := range hosts {
		if h.Category == BastionCategory {
			bastions[h.Name] = h
		}
	}
	for i := range hosts {
		ref := strings.TrimSpace(hosts[i].JumpHostRef)
		if ref == "" {
			hosts[i].JumpEnabled = false
			continue
		}
		bastion, ok := bastions[ref]
		if !ok {
			hosts[i].JumpEnabled = true
			continue
		}
		hosts[i].JumpEnabled = true
		hosts[i].JumpHost = bastion.HostName
		hosts[i].JumpUser = bastion.User
		hosts[i].JumpPort = bastion.Port
		hosts[i].JumpKeyPath = bastion.IdentityFile
	}
	return hosts
}

func categoriesFromHosts(categories []string, hosts []host.Host) []string {
	entries := make([]serverEntry, 0, len(hosts))
	for _, h := range hosts {
		entries = append(entries, serverEntry{Category: h.Category})
	}
	return normalizeCategories(categories, entries)
}

func MigrateServersFile(home string, hosts []host.Host, passwords PasswordStore) error {
	next := make([]host.Host, 0, len(hosts))
	for _, h := range hosts {
		password, _ := passwords.Password(h.Name)
		h.Password = password
		h.HasPassword = password != ""
		h.File = ServersPath(home)
		next = append(next, h)
	}
	return SaveServerHosts(home, next)
}

func PasswordsFromHosts(hosts []host.Host) PasswordStore {
	store := PasswordStore{ByHost: map[string]string{}}
	for _, h := range hosts {
		if strings.TrimSpace(h.Password) != "" {
			store.ByHost[h.Name] = h.Password
		}
	}
	return store
}
