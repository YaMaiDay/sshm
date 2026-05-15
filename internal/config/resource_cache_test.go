package config

import (
	"testing"
	"time"
)

func TestResourceContainerCacheRoundTrip(t *testing.T) {
	home := t.TempDir()
	err := UpsertResourceContainerCache(home, "prod/api-01", []ResourceContainerCache{
		{Name: "api", Image: "app:latest", Status: "Up 1 second", Ports: "80/tcp", CPU: "0.10%", Memory: "12MiB/1GiB", MemPerc: "1.2%"},
	}, time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	file, ok, err := LoadResourceCache(home)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("cache file not found")
	}
	items, ok := ResourceContainerCacheForServer(file, "prod/api-01")
	if !ok || len(items) != 1 {
		t.Fatalf("cache items = %#v, ok=%v", items, ok)
	}
	if items[0].Name != "api" || items[0].Image != "app:latest" || items[0].CPU != "0.10%" {
		t.Fatalf("cache item = %#v", items[0])
	}
}
