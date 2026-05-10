package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/host"
)

func TestAddAndDeleteHost(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := AddCategory(home, "test"); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category:  "test",
		Name:      "test-host",
		HostName:  "127.0.0.1",
		User:      "root",
		Port:      "22",
		ProxyJump: "jump-host",
		Password:  "secret",
	}); err != nil {
		t.Fatal(err)
	}

	configPath := ServersPath(home)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `name = 'test-host'`) && !strings.Contains(string(data), `name = "test-host"`) {
		t.Fatalf("server not written: %s", data)
	}
	if !strings.Contains(string(data), `password = 'secret'`) && !strings.Contains(string(data), `password = "secret"`) {
		t.Fatalf("password not written: %s", data)
	}
	if !strings.Contains(string(data), `proxy_jump = 'jump-host'`) && !strings.Contains(string(data), `proxy_jump = "jump-host"`) {
		t.Fatalf("proxy jump not written: %s", data)
	}

	if err := DeleteHost(home, host.Host{Name: "test-host", File: configPath}, true); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Host test-host") {
		t.Fatalf("host block not deleted: %s", data)
	}
	if strings.Contains(string(data), "test-host") {
		t.Fatalf("server not deleted: %s", data)
	}
}

func TestEditHostMoveCategoryAndPassword(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := AddCategory(home, "old"); err != nil {
		t.Fatal(err)
	}
	if err := AddCategory(home, "new"); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category: "old",
		Name:     "old-host",
		HostName: "127.0.0.1",
		User:     "root",
		Port:     "22",
		Password: "oldpass",
	}); err != nil {
		t.Fatal(err)
	}

	oldPath := ServersPath(home)
	if err := EditHost(home, host.Host{Name: "old-host", File: oldPath}, HostInput{
		Category: "new",
		Name:     "new-host",
		HostName: "10.0.0.1",
		User:     "ubuntu",
		Port:     "2222",
		Password: "newpass",
	}); err != nil {
		t.Fatal(err)
	}

	newData, err := os.ReadFile(ServersPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(newData), "old-host") {
		t.Fatalf("old host still present: %s", newData)
	}
	if !strings.Contains(string(newData), `name = 'new-host'`) && !strings.Contains(string(newData), `name = "new-host"`) {
		t.Fatalf("new host not written correctly: %s", newData)
	}
	if !strings.Contains(string(newData), "2222") || !strings.Contains(string(newData), "newpass") {
		t.Fatalf("new host not written correctly: %s", newData)
	}
}

func TestDeleteCategoryRules(t *testing.T) {
	home := t.TempDir()
	if err := AddCategory(home, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category: "prod",
		Name:     "prod-host",
		HostName: "127.0.0.1",
		User:     "root",
		Port:     "22",
	}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(home, "prod"); err == nil {
		t.Fatal("expected deleting category with servers to fail")
	}
	if err := DeleteHost(home, host.Host{Name: "prod-host"}, true); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(home, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(home, "default"); err == nil {
		t.Fatal("expected deleting final category to fail")
	}
}
