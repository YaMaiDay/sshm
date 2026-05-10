package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type PasswordStore struct {
	ByHost map[string]string
	Dir    string
}

func LoadPasswords(home string) PasswordStore {
	store := PasswordStore{
		ByHost: map[string]string{},
		Dir:    filepath.Join(home, ".ssh", "passwords"),
	}

	path := filepath.Join(home, ".ssh", "passwords.txt")
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimRight(scanner.Text(), "\r\n")
			if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
				store.ByHost[parts[0]] = parts[1]
			}
		}
	}

	if entries, err := os.ReadDir(store.Dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if _, ok := store.ByHost[name]; ok {
				continue
			}
			if data, err := os.ReadFile(filepath.Join(store.Dir, name)); err == nil {
				store.ByHost[name] = strings.TrimRight(string(data), "\r\n")
			}
		}
	}

	return store
}

func (p PasswordStore) Password(host string) (string, bool) {
	value, ok := p.ByHost[host]
	return value, ok && value != ""
}
