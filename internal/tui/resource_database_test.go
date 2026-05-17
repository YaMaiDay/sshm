package tui

import (
	"testing"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func TestDeriveDatabaseDetailsFromDockerAndPort(t *testing.T) {
	containers := []resourceservice.ContainerDetail{{
		Name:   "mysql-prod",
		Image:  "mysql:8",
		Status: "Up 2 hours (healthy)",
		Ports:  "0.0.0.0:3306->3306/tcp",
	}}
	ports := []resourceservice.PortDetail{{
		Protocol:     "tcp",
		Port:         "3306",
		LocalAddress: "0.0.0.0:3306",
		State:        "LISTEN",
		Process:      "docker-proxy",
		Container:    "mysql-prod",
	}}

	items, errText := deriveDatabaseDetails(nil, containers, ports)
	if errText != "" {
		t.Fatalf("errText = %q, want empty", errText)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one merged database", items)
	}
	item := items[0]
	if item.Engine != "MySQL" || item.Source != "Docker+port" || item.Status != "running" {
		t.Fatalf("item = %+v, want running Docker MySQL", item)
	}
	if item.Container != "mysql-prod" || item.Port != "3306" || item.Endpoint == "" {
		t.Fatalf("item = %+v, want container, port, endpoint", item)
	}
}

func TestDeriveDatabaseDetailsFromServiceAndPort(t *testing.T) {
	services := []resourceservice.ServiceDetail{{
		Unit:        "redis.service",
		Load:        "loaded",
		Active:      "active",
		Sub:         "running",
		Description: "Redis persistent key-value database",
		MainPID:     "123",
	}}
	ports := []resourceservice.PortDetail{{
		Protocol:    "tcp",
		Port:        "6379",
		State:       "LISTEN",
		Process:     "redis-server",
		PID:         "123",
		ServiceUnit: "redis.service",
	}}

	items, errText := deriveDatabaseDetails(services, nil, ports)
	if errText != "" {
		t.Fatalf("errText = %q, want empty", errText)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one merged database", items)
	}
	item := items[0]
	if item.Engine != "Redis" || item.ServiceUnit != "redis.service" || item.Port != "6379" {
		t.Fatalf("item = %+v, want Redis service merged with port", item)
	}
}

func TestDeriveDatabaseDetailsKeepsFailedService(t *testing.T) {
	services := []resourceservice.ServiceDetail{{
		Unit:   "mysqld.service",
		Load:   "loaded",
		Active: "failed",
		Sub:    "failed",
		Result: "exit-code",
	}}

	items, _ := deriveDatabaseDetails(services, nil, nil)
	if len(items) != 1 {
		t.Fatalf("items = %#v, want failed mysqld", items)
	}
	if items[0].Engine != "MySQL" || items[0].Status != "problem" {
		t.Fatalf("item = %+v, want problem MySQL", items[0])
	}
}

func TestDefaultDatabaseManagedResourceUsesDockerPublishedPort(t *testing.T) {
	item := defaultDatabaseManagedResource("prod/db", resourceservice.DatabaseDetail{
		Name:     "postgresql_8fjg-postgresql_8FJG-1",
		Engine:   "PostgreSQL",
		Endpoint: "0.0.0.0:35432->5432/tcp",
	})
	if item.Name != "postgres" || item.DBInstance != "postgresql_8fjg-postgresql_8FJG-1" ||
		item.DBEngine != "PostgreSQL" || item.DBHost != "127.0.0.1" || item.DBPort != "35432" || item.DBUser != "postgres" || item.DBName != "postgres" {
		t.Fatalf("managed database = %+v, want postgres defaults with host published port", item)
	}
}
