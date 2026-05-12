package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type CommandTemplate struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type ServerCommandTemplate struct {
	Server  string `toml:"server"`
	Name    string `toml:"name"`
	Command string `toml:"command"`
}

type CommandsFile struct {
	Global []CommandTemplate       `toml:"global"`
	Server []ServerCommandTemplate `toml:"server"`
}

func CommandsPath(home string) string {
	return filepath.Join(home, ".config", "sshm", "commands.toml")
}

func LoadCommands(home string) (CommandsFile, bool, error) {
	path := CommandsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CommandsFile{}, false, nil
		}
		return CommandsFile{}, false, err
	}
	var file CommandsFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return CommandsFile{}, true, err
	}
	file.Global = normalizeCommandTemplates(file.Global)
	file.Server = normalizeServerCommandTemplates(file.Server)
	return file, true, nil
}

func SaveCommands(home string, file CommandsFile) error {
	path := CommandsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file.Global = normalizeCommandTemplates(file.Global)
	file.Server = normalizeServerCommandTemplates(file.Server)
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	return writeFile0600(path, data)
}

func ValidateCommandTemplate(name, command string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("命令内容不能为空")
	}
	return nil
}

func ServerCommandKey(category, name string) string {
	category = strings.TrimSpace(category)
	name = strings.TrimSpace(name)
	if category == "" {
		return name
	}
	return category + "/" + name
}

func normalizeCommandTemplates(commands []CommandTemplate) []CommandTemplate {
	out := make([]CommandTemplate, 0, len(commands))
	for _, command := range commands {
		name := strings.TrimSpace(command.Name)
		body := strings.TrimSpace(command.Command)
		if name == "" || body == "" {
			continue
		}
		out = append(out, CommandTemplate{Name: name, Command: body})
	}
	return out
}

func normalizeServerCommandTemplates(commands []ServerCommandTemplate) []ServerCommandTemplate {
	out := make([]ServerCommandTemplate, 0, len(commands))
	for _, command := range commands {
		server := strings.TrimSpace(command.Server)
		name := strings.TrimSpace(command.Name)
		body := strings.TrimSpace(command.Command)
		if server == "" || name == "" || body == "" {
			continue
		}
		out = append(out, ServerCommandTemplate{Server: server, Name: name, Command: body})
	}
	return out
}
