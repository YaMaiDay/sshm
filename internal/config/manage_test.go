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

	if err := DeleteHost(home, host.Host{Category: "test", Name: "test-host", File: configPath}, true); err != nil {
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
	if err := EditHost(home, host.Host{Category: "old", Name: "old-host", File: oldPath}, HostInput{
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
	if err := DeleteHost(home, host.Host{Category: "prod", Name: "prod-host"}, true); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(home, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCategory(home, "default"); err == nil {
		t.Fatal("expected deleting final category to fail")
	}
}

func TestDuplicateHostNameAllowedAcrossCategories(t *testing.T) {
	home := t.TempDir()
	if err := AddCategory(home, "left"); err != nil {
		t.Fatal(err)
	}
	if err := AddCategory(home, "right"); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category: "left",
		Name:     "same-name",
		HostName: "127.0.0.1",
		User:     "root",
		Port:     "22",
	}); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category: "right",
		Name:     "same-name",
		HostName: "127.0.0.2",
		User:     "root",
		Port:     "2222",
	}); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(home, HostInput{
		Category: "left",
		Name:     "same-name",
		HostName: "127.0.0.3",
		User:     "root",
		Port:     "2223",
	}); err == nil {
		t.Fatal("expected duplicate name in same category to fail")
	}

	if err := DeleteHost(home, host.Host{Category: "left", Name: "same-name"}, true); err != nil {
		t.Fatal(err)
	}
	hosts, _, err := LoadServerHosts(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected one host after category-scoped delete, got %d", len(hosts))
	}
	if hosts[0].Category != "right" || hosts[0].Name != "same-name" {
		t.Fatalf("wrong host remained: %+v", hosts[0])
	}
}

func TestSaveAndLoadFavoriteHost(t *testing.T) {
	home := t.TempDir()
	hosts := []host.Host{
		{
			Category:    "prod",
			Name:        "favorite-host",
			HostName:    "127.0.0.1",
			User:        "root",
			Port:        "22",
			Favorite:    true,
			HealthPorts: []int{80, 5432},
		},
	}
	if err := SaveServerHosts(home, hosts); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(ServersPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "favorite = true") {
		t.Fatalf("favorite not written: %s", data)
	}
	if !strings.Contains(string(data), "health_ports = [80, 5432]") {
		t.Fatalf("health ports not written: %s", data)
	}

	loaded, _, err := LoadServerHosts(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d hosts, want 1", len(loaded))
	}
	if !loaded[0].Favorite {
		t.Fatalf("Favorite = false, want true")
	}
	if len(loaded[0].HealthPorts) != 2 || loaded[0].HealthPorts[0] != 80 || loaded[0].HealthPorts[1] != 5432 {
		t.Fatalf("HealthPorts = %#v, want [80 5432]", loaded[0].HealthPorts)
	}
}

func TestParseHealthPorts(t *testing.T) {
	ports, err := ParseHealthPorts("80, 5432 8080,80")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{80, 5432, 8080}
	if len(ports) != len(want) {
		t.Fatalf("ports = %#v, want %#v", ports, want)
	}
	for i := range want {
		if ports[i] != want[i] {
			t.Fatalf("ports = %#v, want %#v", ports, want)
		}
	}
	if _, err := ParseHealthPorts("80,abc"); err == nil {
		t.Fatal("expected invalid health port to fail")
	}
}
