package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadCommands(t *testing.T) {
	home := t.TempDir()
	file := CommandsFile{
		Global: []CommandTemplate{{
			Name:    "查看磁盘",
			Command: "df -h",
		}},
		Server: []ServerCommandTemplate{{
			Server:  "aws/demo",
			Name:    "更新项目",
			Command: "cd /home/app\ngit pull",
		}},
	}

	if err := SaveCommands(home, file); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".config", "sshm", "commands.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[[global]]") || !strings.Contains(string(data), "[[server]]") {
		t.Fatalf("commands.toml = %s, want global and server sections", data)
	}
	if strings.Contains(string(data), "Name") || strings.Contains(string(data), "Command") || strings.Contains(string(data), "Server") {
		t.Fatalf("commands.toml = %s, want lowercase toml keys", data)
	}
	if !strings.Contains(string(data), "name") || !strings.Contains(string(data), "command") || !strings.Contains(string(data), "server") {
		t.Fatalf("commands.toml = %s, want name, command and server keys", data)
	}
	if info, err := os.Stat(filepath.Join(home, ".config", "sshm", "commands.toml")); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("commands.toml mode = %o, want 600", got)
	}
	if matches, err := filepath.Glob(filepath.Join(home, ".config", "sshm", ".commands.toml.tmp-*")); err != nil {
		t.Fatal(err)
	} else if len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}

	got, ok, err := LoadCommands(home)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("LoadCommands ok = false, want true")
	}
	if len(got.Global) != 1 || got.Global[0].Name != "查看磁盘" {
		t.Fatalf("Global = %#v, want saved command", got.Global)
	}
	if len(got.Server) != 1 || got.Server[0].Command != "cd /home/app\ngit pull" {
		t.Fatalf("Server = %#v, want saved server command", got.Server)
	}
}

func TestServerCommandKey(t *testing.T) {
	if got := ServerCommandKey("aws", "demo"); got != "aws/demo" {
		t.Fatalf("ServerCommandKey = %q, want aws/demo", got)
	}
}
