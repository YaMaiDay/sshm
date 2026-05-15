package config

import (
	"os"
	"testing"
)

func TestSaveLoadResources(t *testing.T) {
	home := t.TempDir()
	file := ResourcesFile{Items: []ManagedResource{
		{Server: "prod/api-01", Kind: ResourceKindService, Name: "api.service", StartCommand: "systemctl start api.service"},
		{Server: "prod/api-01", Kind: ResourceKindService, Name: "api.service"},
		{Server: "prod/api-01", Kind: ResourceKindContainer, Name: "api"},
	}}
	if err := SaveResources(home, file); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadResources(home)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("resources file not found after save")
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %#v, want deduped 2", got.Items)
	}
	info, err := os.Stat(ResourcesPath(home))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}
