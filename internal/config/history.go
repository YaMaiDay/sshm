package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/YaMaiDay/sshm/internal/host"
)

const (
	MaxCommandHistoryEntries = 100
	MaxCommandHistoryLines   = 200
)

type CommandHistoryFile struct {
	Entries []CommandHistoryEntry `toml:"entries"`
}

type CommandHistoryEntry struct {
	ID       string                 `toml:"id"`
	Time     string                 `toml:"time"`
	Kind     string                 `toml:"kind"`
	Name     string                 `toml:"name"`
	Command  string                 `toml:"command"`
	Status   string                 `toml:"status"`
	ExitCode int                    `toml:"exit_code"`
	Targets  []CommandHistoryTarget `toml:"targets"`
}

type CommandHistoryTarget struct {
	Category string `toml:"category"`
	Name     string `toml:"name"`
	HostName string `toml:"host"`
	User     string `toml:"user"`
	Port     string `toml:"port"`
	Status   string `toml:"status"`
	ExitCode int    `toml:"exit_code"`
	Output   string `toml:"output"`
}

func CommandHistoryPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "history.toml")
}

func LoadCommandHistory(home string) (CommandHistoryFile, bool, error) {
	path := CommandHistoryPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CommandHistoryFile{}, false, nil
		}
		return CommandHistoryFile{}, false, err
	}
	var file CommandHistoryFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return CommandHistoryFile{}, true, err
	}
	return file, true, nil
}

func SaveCommandHistory(home string, file CommandHistoryFile) error {
	path := CommandHistoryPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Entries = normalizeCommandHistory(file.Entries)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func AppendCommandHistory(home string, entry CommandHistoryEntry) error {
	file, _, err := LoadCommandHistory(home)
	if err != nil {
		return err
	}
	entry = NormalizeCommandHistoryEntry(entry)
	file.Entries = append([]CommandHistoryEntry{entry}, file.Entries...)
	return SaveCommandHistory(home, file)
}

func DeleteCommandHistoryEntry(home string, id string) error {
	file, _, err := LoadCommandHistory(home)
	if err != nil {
		return err
	}
	next := make([]CommandHistoryEntry, 0, len(file.Entries))
	for _, entry := range file.Entries {
		if entry.ID == id {
			continue
		}
		next = append(next, entry)
	}
	file.Entries = next
	return SaveCommandHistory(home, file)
}

func NewCommandHistoryID(at time.Time) string {
	return fmt.Sprintf("%d", at.UnixNano())
}

func CommandHistoryTargetFromHost(h host.Host, status string, exitCode int, output string) CommandHistoryTarget {
	return CommandHistoryTarget{
		Category: strings.TrimSpace(h.Category),
		Name:     strings.TrimSpace(h.Name),
		HostName: strings.TrimSpace(h.HostName),
		User:     strings.TrimSpace(h.User),
		Port:     strings.TrimSpace(h.Port),
		Status:   strings.TrimSpace(status),
		ExitCode: exitCode,
		Output:   TrimCommandHistoryOutput(output),
	}
}

func NormalizeCommandHistoryEntry(entry CommandHistoryEntry) CommandHistoryEntry {
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = NewCommandHistoryID(time.Now())
	}
	if strings.TrimSpace(entry.Time) == "" {
		entry.Time = time.Now().Format(time.RFC3339)
	}
	entry.Kind = strings.TrimSpace(entry.Kind)
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Command = strings.TrimSpace(entry.Command)
	entry.Status = strings.TrimSpace(entry.Status)
	for i := range entry.Targets {
		entry.Targets[i].Category = strings.TrimSpace(entry.Targets[i].Category)
		entry.Targets[i].Name = strings.TrimSpace(entry.Targets[i].Name)
		entry.Targets[i].HostName = strings.TrimSpace(entry.Targets[i].HostName)
		entry.Targets[i].User = strings.TrimSpace(entry.Targets[i].User)
		entry.Targets[i].Port = strings.TrimSpace(entry.Targets[i].Port)
		entry.Targets[i].Status = strings.TrimSpace(entry.Targets[i].Status)
		entry.Targets[i].Output = TrimCommandHistoryOutput(entry.Targets[i].Output)
	}
	return entry
}

func TrimCommandHistoryOutput(output string) string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	if len(lines) > MaxCommandHistoryLines {
		lines = lines[len(lines)-MaxCommandHistoryLines:]
	}
	return strings.Join(lines, "\n")
}

func normalizeCommandHistory(entries []CommandHistoryEntry) []CommandHistoryEntry {
	out := make([]CommandHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		entry = NormalizeCommandHistoryEntry(entry)
		if entry.Command == "" || len(entry.Targets) == 0 {
			continue
		}
		out = append(out, entry)
		if len(out) >= MaxCommandHistoryEntries {
			break
		}
	}
	return out
}
