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
		"__SSHM_DB__\tSIZE_BYTES\t1024",
		"__SSHM_DB__\tTOTAL_BYTES\t4096",
		"__SSHM_DB__\tTABLE_COUNT\t42",
		"__SSHM_DB__\tTABLE_TOP\tpublic.orders=2048",
	}, "\n"))
	if errText != "" {
		t.Fatalf("errText = %q, want empty", errText)
	}
	if detail.Version != "PostgreSQL 16" || detail.Uptime != "1小时1分钟" || detail.SizeBytes != 1024 || detail.TotalBytes != 4096 || detail.TableCount != "42" {
		t.Fatalf("detail = %+v, want parsed metrics", detail)
	}
	if len(detail.TableTop) != 1 || detail.TableTop[0].Name != "public.orders" || detail.TableTop[0].Size != 2048 {
		t.Fatalf("table top = %+v, want public.orders size", detail.TableTop)
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
}
