package tui

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestDeriveDatabaseDetailsFromDockerAndPort(t *testing.T) {
	containers := []containerDetail{{
		Name:   "mysql-prod",
		Image:  "mysql:8",
		Status: "Up 2 hours (healthy)",
		Ports:  "0.0.0.0:3306->3306/tcp",
	}}
	ports := []portDetail{{
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
	services := []serviceDetail{{
		Unit:        "redis.service",
		Load:        "loaded",
		Active:      "active",
		Sub:         "running",
		Description: "Redis persistent key-value database",
		MainPID:     "123",
	}}
	ports := []portDetail{{
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
	services := []serviceDetail{{
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

func TestParseDatabaseExtraDetailMySQL(t *testing.T) {
	output := "__SSHM_DB__\tVERSION\t8.0.36\n" +
		"__SSHM_DB__\tUptime\t3661\n" +
		"__SSHM_DB__\tThreads_connected\t12\n" +
		"__SSHM_DB__\tmax_connections\t200\n" +
		"__SSHM_DB__\tCONN_ACTIVE\t3\n" +
		"__SSHM_DB__\tCONN_IDLE\t9\n" +
		"__SSHM_DB__\tDATABASE_COUNT\t4\n" +
		"__SSHM_DB__\tDATABASE_TOTAL_BYTES\t8192\n" +
		"__SSHM_DB__\tDATABASE_TOP\tfreedex=4096\n" +
		"__SSHM_DB__\tCACHE_HIT\t99.12\n" +
		"__SSHM_DB__\tLOCK_WAITS\t1\n" +
		"__SSHM_DB__\tLONG_TX\t2\n" +
		"__SSHM_DB__\tDEADLOCKS\t5\n" +
		"__SSHM_DB__\tSIZE_BYTES\t1024\n" +
		"__SSHM_DB__\tTOTAL_BYTES\t4096\n" +
		"__SSHM_DB__\tDATA_BYTES\t768\n" +
		"__SSHM_DB__\tINDEX_BYTES\t256\n" +
		"__SSHM_DB__\tINDEX_TOTAL_BYTES\t512\n" +
		"__SSHM_DB__\tTABLE_COUNT\t42\n" +
		"__SSHM_DB__\tTABLE_TOP\tpublic.orders=2048\n"
	detail, errText := parseDatabaseExtraDetail(output)
	if errText != "" {
		t.Fatalf("errText = %q, want empty", errText)
	}
	if detail.Version != "8.0.36" || detail.Uptime != "1小时1分钟" || detail.Connections != "12" || detail.MaxConnections != "200" {
		t.Fatalf("detail = %+v, want version and connections", detail)
	}
	if detail.SizeBytes != 1024 || detail.TotalBytes != 4096 || detail.DataBytes != 768 || detail.IndexBytes != 256 {
		t.Fatalf("sizes = %d/%d/%d/%d, want 1024/4096/768/256", detail.SizeBytes, detail.TotalBytes, detail.DataBytes, detail.IndexBytes)
	}
	if detail.DatabaseCount != "4" || detail.DBTotalBytes != 8192 || detail.IndexTotalBytes != 512 || detail.TableCount != "42" ||
		len(detail.DatabaseTop) != 1 || detail.DatabaseTop[0].Name != "freedex" || detail.DatabaseTop[0].Size != 4096 ||
		len(detail.TableTop) != 1 || detail.TableTop[0].Name != "public.orders" || detail.TableTop[0].Size != 2048 {
		t.Fatalf("postgres storage extras = %+v", detail)
	}
	if detail.ActiveConns != "3" || detail.IdleConns != "9" || detail.CacheHit != "99.12%" || detail.LockWaits != "1" || detail.LongTx != "2" || detail.Deadlocks != "5" {
		t.Fatalf("postgres performance extras = %+v", detail)
	}
}

func TestParseDatabaseExtraDetailShowsRawFailure(t *testing.T) {
	detail, errText := parseDatabaseExtraDetail("Access denied for user monitor\n")
	if detail.Version != "" {
		t.Fatalf("detail = %+v, want empty detail", detail)
	}
	if !strings.Contains(errText, "Access denied") {
		t.Fatalf("errText = %q, want raw failure", errText)
	}
}

func TestParseDatabaseExtraDetailIgnoresMariaDBDeprecationOnly(t *testing.T) {
	_, errText := parseDatabaseExtraDetail("mysql: Deprecated program name. It will be removed in a future release, use '/usr/bin/mariadb' instead\n")
	if errText != "没有获取到数据库指标" {
		t.Fatalf("errText = %q, want generic metrics error", errText)
	}
}

func TestDatabaseMissingCredentialHint(t *testing.T) {
	item := config.ManagedResource{DBEngine: "MySQL", DBUser: "root"}
	if !databaseMissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("empty MySQL password with generic metrics error should show credential hint")
	}
	item.DBPassword = "secret"
	if databaseMissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("configured password should keep original error")
	}
	item = config.ManagedResource{DBEngine: "Redis"}
	if databaseMissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("redis should not show SQL credential hint")
	}
}

func TestDatabaseMetricScriptUsesRedisCLI(t *testing.T) {
	script := databaseMetricScript(config.ManagedResource{DBEngine: "Redis", DBHost: "127.0.0.1", DBPort: "6379", DBPassword: "secret"})
	if !strings.Contains(script, "redis-cli") || !strings.Contains(script, "-a secret") {
		t.Fatalf("script = %s, want redis-cli with auth", script)
	}
}

func TestDatabaseMetricScriptUsesMySQLOrMariaDBClient(t *testing.T) {
	script := databaseMetricScript(config.ManagedResource{DBEngine: "MySQL", DBHost: "127.0.0.1", DBPort: "3306", DBUser: "monitor"})
	if !strings.Contains(script, "command -v mysql") || !strings.Contains(script, "command -v mariadb") || !strings.Contains(script, `"$DB_CLIENT"`) {
		t.Fatalf("script = %s, want mysql/mariadb client fallback", script)
	}
}

func TestPostgresMetricScriptIncludesUptime(t *testing.T) {
	script := databaseMetricScript(config.ManagedResource{DBEngine: "PostgreSQL", DBHost: "127.0.0.1", DBPort: "5432", DBUser: "postgres", DBName: "postgres"})
	if !strings.Contains(script, "pg_postmaster_start_time") || !strings.Contains(script, "Uptime") {
		t.Fatalf("script = %s, want postgres uptime query", script)
	}
}

func TestDatabaseMetricScriptUsesMongoClient(t *testing.T) {
	script := databaseMetricScript(config.ManagedResource{DBEngine: "MongoDB", DBHost: "127.0.0.1", DBPort: "27017", DBUser: "monitor", DBPassword: "secret", DBName: "admin"})
	if !strings.Contains(script, "command -v mongosh") ||
		!strings.Contains(script, "command -v mongo") ||
		!strings.Contains(script, "serverStatus") ||
		!strings.Contains(script, "Uptime") {
		t.Fatalf("script = %s, want mongo client metrics script", script)
	}
}

func TestPostgresMetricScriptPrefersDockerContainer(t *testing.T) {
	script := databaseMetricScriptForDetail(
		config.ManagedResource{DBEngine: "PostgreSQL", DBHost: "127.0.0.1", DBPort: "35432", DBUser: "postgres", DBName: "postgres"},
		databaseDetail{Container: "postgresql_8fjg-postgresql_8FJG-1"},
	)
	if !strings.Contains(script, "command -v psql") ||
		!strings.Contains(script, "docker \"$@\"") ||
		!strings.Contains(script, "docker exec") ||
		!strings.Contains(script, "DATA_DIR") ||
		!strings.Contains(script, "TOTAL_BYTES") ||
		!strings.Contains(script, "TABLE_COUNT") ||
		!strings.Contains(script, "DATABASE_TOP") ||
		!strings.Contains(script, "LIMIT 10") ||
		!strings.Contains(script, "postgresql_8fjg-postgresql_8FJG-1") ||
		!strings.Contains(script, "-p 5432") {
		t.Fatalf("script = %s, want docker exec with host psql fallback", script)
	}
	if strings.Index(script, "docker exec") > strings.Index(script, "command -v psql") {
		t.Fatalf("script should prefer docker exec before host psql:\n%s", script)
	}
}

func TestDefaultDatabaseManagedResourceUsesDockerPublishedPort(t *testing.T) {
	item := defaultDatabaseManagedResource("prod/db", databaseDetail{
		Name:     "postgresql_8fjg-postgresql_8FJG-1",
		Engine:   "PostgreSQL",
		Endpoint: "0.0.0.0:35432->5432/tcp",
	})
	if item.Name != "postgres" || item.DBInstance != "postgresql_8fjg-postgresql_8FJG-1" ||
		item.DBEngine != "PostgreSQL" || item.DBHost != "127.0.0.1" || item.DBPort != "35432" || item.DBUser != "postgres" || item.DBName != "postgres" {
		t.Fatalf("managed database = %+v, want postgres defaults with host published port", item)
	}
}

func TestNormalizeDatabaseEngine(t *testing.T) {
	cases := map[string]string{
		"postgres": "PostgreSQL",
		"pgsql":    "PostgreSQL",
		"mariadb":  "MariaDB",
		"redis":    "Redis",
		"mongo":    "MongoDB",
		"mysql":    "MySQL",
	}
	for input, want := range cases {
		if got := normalizeDatabaseEngine(input); got != want {
			t.Fatalf("normalizeDatabaseEngine(%q) = %q, want %q", input, got, want)
		}
	}
}
