package dbmonitor

import (
	"strings"
	"testing"

	"github.com/YaMaiDay/sshm/internal/config"
)

func TestParseMetrics(t *testing.T) {
	detail, errText := Parse(strings.Join([]string{
		"__SSHM_DB__\tVERSION\tPostgreSQL 16",
		"__SSHM_DB__\tUptime\t3661",
		"__SSHM_DB__\tThreads_connected\t12",
		"__SSHM_DB__\tmax_connections\t200",
		"__SSHM_DB__\tCONN_ACTIVE\t3",
		"__SSHM_DB__\tCONN_IDLE\t9",
		"__SSHM_DB__\tDATABASE_COUNT\t4",
		"__SSHM_DB__\tDATABASE_TOTAL_BYTES\t8192",
		"__SSHM_DB__\tDATABASE_TOP\tfreedex=4096",
		"__SSHM_DB__\tCACHE_HIT\t99.12",
		"__SSHM_DB__\tLOCK_WAITS\t1",
		"__SSHM_DB__\tLONG_TX\t2",
		"__SSHM_DB__\tDEADLOCKS\t5",
		"__SSHM_DB__\tSIZE_BYTES\t1024",
		"__SSHM_DB__\tTOTAL_BYTES\t4096",
		"__SSHM_DB__\tDATA_BYTES\t768",
		"__SSHM_DB__\tINDEX_BYTES\t256",
		"__SSHM_DB__\tINDEX_TOTAL_BYTES\t512",
		"__SSHM_DB__\tTABLE_COUNT\t42",
		"__SSHM_DB__\tTABLE_TOP\tpublic.orders=2048",
	}, "\n"))
	if errText != "" {
		t.Fatalf("errText = %q, want empty", errText)
	}
	if detail.Version != "PostgreSQL 16" || detail.Uptime != "1小时1分钟" || detail.Connections != "12" || detail.MaxConnections != "200" {
		t.Fatalf("detail = %+v, want version and connections", detail)
	}
	if detail.SizeBytes != 1024 || detail.TotalBytes != 4096 || detail.DataBytes != 768 || detail.IndexBytes != 256 {
		t.Fatalf("sizes = %d/%d/%d/%d, want 1024/4096/768/256", detail.SizeBytes, detail.TotalBytes, detail.DataBytes, detail.IndexBytes)
	}
	if detail.DatabaseCount != "4" || detail.DBTotalBytes != 8192 || detail.IndexTotalBytes != 512 || detail.TableCount != "42" {
		t.Fatalf("detail = %+v, want parsed metrics", detail)
	}
	if len(detail.DatabaseTop) != 1 || detail.DatabaseTop[0].Name != "freedex" || detail.DatabaseTop[0].Size != 4096 {
		t.Fatalf("database top = %+v, want freedex size", detail.DatabaseTop)
	}
	if len(detail.TableTop) != 1 || detail.TableTop[0].Name != "public.orders" || detail.TableTop[0].Size != 2048 {
		t.Fatalf("table top = %+v, want public.orders size", detail.TableTop)
	}
	if detail.ActiveConns != "3" || detail.IdleConns != "9" || detail.CacheHit != "99.12%" || detail.LockWaits != "1" || detail.LongTx != "2" || detail.Deadlocks != "5" {
		t.Fatalf("postgres performance extras = %+v", detail)
	}
}

func TestParseShowsRawFailure(t *testing.T) {
	detail, errText := Parse("Access denied for user monitor\n")
	if detail.Version != "" {
		t.Fatalf("detail = %+v, want empty detail", detail)
	}
	if !strings.Contains(errText, "Access denied") {
		t.Fatalf("errText = %q, want raw failure", errText)
	}
}

func TestParseIgnoresMariaDBDeprecationOnly(t *testing.T) {
	_, errText := Parse("mysql: Deprecated program name. It will be removed in a future release, use '/usr/bin/mariadb' instead\n")
	if errText != "没有获取到数据库指标" {
		t.Fatalf("errText = %q, want generic metrics error", errText)
	}
}

func TestMetricScriptUsesRedisCLI(t *testing.T) {
	script := MetricScript(config.ManagedResource{DBEngine: "Redis", DBHost: "127.0.0.1", DBPort: "6379", DBPassword: "secret"})
	if !strings.Contains(script, "redis-cli") || !strings.Contains(script, "-a secret") {
		t.Fatalf("script = %s, want redis-cli with auth", script)
	}
}

func TestMetricScriptUsesMySQLOrMariaDBClient(t *testing.T) {
	script := MetricScript(config.ManagedResource{DBEngine: "MySQL", DBHost: "127.0.0.1", DBPort: "3306", DBUser: "monitor"})
	if !strings.Contains(script, "command -v mysql") || !strings.Contains(script, "command -v mariadb") || !strings.Contains(script, `"$DB_CLIENT"`) {
		t.Fatalf("script = %s, want mysql/mariadb client fallback", script)
	}
}

func TestMetricScriptQuotesDatabaseConnectionFields(t *testing.T) {
	script := MetricScript(config.ManagedResource{
		DBEngine:   "MySQL",
		DBHost:     "127.0.0.1; touch /tmp/pwned",
		DBPort:     "3306; touch /tmp/pwned",
		DBUser:     "monitor user",
		DBPassword: "pa' ss",
	})
	for _, want := range []string{
		"'127.0.0.1; touch /tmp/pwned'",
		"'3306; touch /tmp/pwned'",
		"'monitor user'",
		"'pa'\\'' ss'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing quoted value %q:\n%s", want, script)
		}
	}
}

func TestPostgresMetricScriptIncludesUptime(t *testing.T) {
	script := MetricScript(config.ManagedResource{DBEngine: "PostgreSQL", DBHost: "127.0.0.1", DBPort: "5432", DBUser: "postgres", DBName: "postgres"})
	if !strings.Contains(script, "pg_postmaster_start_time") || !strings.Contains(script, "Uptime") {
		t.Fatalf("script = %s, want postgres uptime query", script)
	}
}

func TestMetricScriptUsesMongoClient(t *testing.T) {
	script := MetricScript(config.ManagedResource{DBEngine: "MongoDB", DBHost: "127.0.0.1", DBPort: "27017", DBUser: "monitor", DBPassword: "secret", DBName: "admin"})
	if !strings.Contains(script, "command -v mongosh") ||
		!strings.Contains(script, "command -v mongo") ||
		!strings.Contains(script, "serverStatus") ||
		!strings.Contains(script, "Uptime") {
		t.Fatalf("script = %s, want mongo client metrics script", script)
	}
}

func TestMetricScriptForRuntimePrefersPostgresContainer(t *testing.T) {
	script := MetricScriptForRuntime(
		config.ManagedResource{DBEngine: "PostgreSQL", DBHost: "127.0.0.1", DBPort: "35432", DBUser: "postgres", DBName: "postgres"},
		Runtime{Container: "postgresql_8fjg-postgresql_8FJG-1"},
	)
	for _, want := range []string{"docker exec", "command -v psql", "pg_postmaster_start_time", "postgresql_8fjg-postgresql_8FJG-1", "-p 5432"} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	if strings.Index(script, "docker exec") > strings.Index(script, "command -v psql") {
		t.Fatalf("script should try docker exec before host psql:\n%s", script)
	}
}

func TestDefaultsAndCredentialHint(t *testing.T) {
	if NormalizeEngine("pgsql") != "PostgreSQL" || DefaultPort("PostgreSQL") != "5432" || DefaultUser("PostgreSQL") != "postgres" || DefaultName("PostgreSQL") != "postgres" {
		t.Fatal("postgres defaults not normalized")
	}
	item := config.ManagedResource{DBEngine: "MySQL", DBUser: "root"}
	if !MissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("empty MySQL password should show credential hint")
	}
	item.DBPassword = "secret"
	if MissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("configured password should keep original error")
	}
	item = config.ManagedResource{DBEngine: "Redis"}
	if MissingCredentialHint(item, "没有获取到数据库指标") {
		t.Fatal("redis should not show SQL credential hint")
	}
}

func TestNormalizeEngine(t *testing.T) {
	cases := map[string]string{
		"postgres": "PostgreSQL",
		"pgsql":    "PostgreSQL",
		"mariadb":  "MariaDB",
		"redis":    "Redis",
		"mongo":    "MongoDB",
		"mysql":    "MySQL",
	}
	for input, want := range cases {
		if got := NormalizeEngine(input); got != want {
			t.Fatalf("NormalizeEngine(%q) = %q, want %q", input, got, want)
		}
	}
}
