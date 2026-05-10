package config

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/YaMaiDay/sshm/internal/host"
)

type Loader struct {
	Home      string
	Passwords PasswordStore
	seenFiles map[string]bool
}

func LoadHosts(home string) ([]host.Host, error) {
	if hosts, ok, err := LoadServerHosts(home); ok || err != nil {
		return hosts, err
	}
	loader := Loader{
		Home:      home,
		Passwords: LoadPasswords(home),
		seenFiles: map[string]bool{},
	}
	main := filepath.Join(home, ".ssh", "config")
	hosts, err := loader.parseFile(main, "main")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	seenHosts := map[string]bool{}
	out := make([]host.Host, 0, len(hosts))
	for _, h := range hosts {
		if h.Name == "" || strings.ContainsAny(h.Name, "*?") || seenHosts[h.Name] {
			continue
		}
		if _, ok := loader.Passwords.Password(h.Name); ok {
			h.HasPassword = true
		}
		seenHosts[h.Name] = true
		out = append(out, h)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Category < out[j].Category
	})
	return out, nil
}

func (l *Loader) parseFile(path, category string) ([]host.Host, error) {
	path = expandHome(path, l.Home)
	if !filepath.IsAbs(path) {
		path = filepath.Join(l.Home, path)
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = realPath
	}
	if l.seenFiles[path] {
		return nil, nil
	}
	l.seenFiles[path] = true

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if category == "" {
		category = categoryFromPath(path)
	}

	var hosts []host.Host
	var current *host.Host

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "#") {
			continue
		}

		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		key := strings.ToLower(fields[0])

		if key == "include" && len(fields) > 1 {
			for _, pattern := range fields[1:] {
				matches, _ := filepath.Glob(expandHome(pattern, l.Home))
				for _, match := range matches {
					base := filepath.Base(match)
					if strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
						continue
					}
					if info, err := os.Stat(match); err == nil && !info.IsDir() {
						included, err := l.parseFile(match, categoryFromPath(match))
						if err == nil {
							hosts = append(hosts, included...)
						}
					}
				}
			}
			continue
		}

		if key == "host" && len(fields) > 1 {
			if current != nil {
				hosts = append(hosts, *current)
			}
			current = &host.Host{
				Name:     fields[1],
				User:     os.Getenv("USER"),
				Port:     "22",
				Category: category,
				File:     path,
			}
			continue
		}

		if current == nil || len(fields) < 2 {
			continue
		}
		value := strings.Join(fields[1:], " ")
		switch key {
		case "hostname":
			current.HostName = value
		case "user":
			current.User = value
		case "port":
			current.Port = value
		case "identityfile":
			current.IdentityFile = value
		case "proxyjump":
			current.ProxyJump = value
		}
	}
	if current != nil {
		hosts = append(hosts, *current)
	}

	return hosts, scanner.Err()
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return os.ExpandEnv(path)
}

func categoryFromPath(path string) string {
	base := filepath.Base(path)
	if base == "config" {
		return "main"
	}
	return base
}
