package resource

import (
	"strings"
	"testing"
)

func TestParseServiceDetailsSortsFailedFirst(t *testing.T) {
	out := strings.Join([]string{
		"nginx.service loaded active running A high performance web server",
		"redis.service loaded failed failed Redis server",
		"cron.service loaded active exited Regular background program processing daemon",
		"old.service loaded inactive dead Old service",
	}, "\n")
	services, errText := ParseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 4 {
		t.Fatalf("services = %#v", services)
	}
	if services[0].Unit != "redis.service" || ServiceDetailKindRank(services[0]) != 0 {
		t.Fatalf("first service = %+v, want failed redis first", services[0])
	}
}

func TestParseServiceDetailsWithDiscoveryMetadata(t *testing.T) {
	out := "__SSHM_SERVICE__\tapi.service\tloaded\tactive\trunning\tAPI Service\t/etc/systemd/system/api.service\t/data/api\t/data/api/server\t123\t0\t86016\tFri 2026-05-15 10:00:00 UTC\tFri 2026-05-14 10:00:00 UTC\tFri 2026-05-15 10:00:00 UTC\tFri 2026-05-15 10:00:01 UTC\tFri 2026-05-14 10:00:00 UTC\tenabled\tsuccess\t0\t2\t6\t/system.slice/api.service\tsystem.slice\tapp\tapp\talways\t5000000\t/bin/stop\t/bin/reload\t/etc/systemd/system/api.service.d/override.conf"
	services, errText := ParseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 1 {
		t.Fatalf("services = %#v, want 1", services)
	}
	item := services[0]
	if item.Unit != "api.service" || item.FragmentPath != "/etc/systemd/system/api.service" || item.WorkingDirectory != "/data/api" || item.ExecStart != "/data/api/server" {
		t.Fatalf("service metadata = %+v", item)
	}
	if item.MainPID != "123" || item.ExecMainPID != "" || item.MemoryCurrent != 86016 || item.ActiveSince != "Fri 2026-05-15 10:00:00 UTC" {
		t.Fatalf("service resource fields = %+v", item)
	}
	if item.UnitFileState != "enabled" || item.Result != "success" || item.NRestarts != "2" || item.TasksCurrent != "6" || item.User != "app" || item.Restart != "always" || item.RestartSec != "5000000" || item.ExecStop != "/bin/stop" || item.ExecReload != "/bin/reload" {
		t.Fatalf("service extended fields = %+v", item)
	}
}

func TestParseServiceDetailsIgnoresInvalidMemoryCurrent(t *testing.T) {
	out := "__SSHM_SERVICE__\tapi.service\tloaded\tactive\trunning\tAPI Service\t/etc/systemd/system/api.service\t/data/api\t/data/api/server\t0\t456\t18446744073709551615"
	services, errText := ParseServiceDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(services) != 1 {
		t.Fatalf("services = %#v, want 1", services)
	}
	if services[0].MainPID != "" || services[0].ExecMainPID != "456" || services[0].MemoryCurrent != 0 {
		t.Fatalf("invalid memory should be hidden: %+v", services[0])
	}
}

func TestParseServiceExtraDetailRawSystemctlShow(t *testing.T) {
	out := strings.Join([]string{
		"Id=postfix.service",
		"LoadState=loaded",
		"ActiveState=active",
		"SubState=running",
		"Description=Postfix Mail Transport Agent",
		"FragmentPath=/usr/lib/systemd/system/postfix.service",
		"ExecStart={ path=/usr/sbin/postfix ; argv[]=/usr/sbin/postfix start ; status=0 }",
		"MainPID=3137",
		"ExecMainPID=0",
		"MemoryCurrent=7969177",
		"ActiveEnterTimestamp=Tue 2026-03-17 11:01:19 CST",
		"UnitFileState=enabled",
		"Result=success",
		"ExecMainStatus=0",
		"NRestarts=0",
		"TasksCurrent=4",
		"ControlGroup=/system.slice/postfix.service",
		"Slice=system.slice",
		"Restart=no",
		"RestartUSec=100ms",
	}, "\n")
	item, errText := ParseServiceExtraDetail(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if item.Unit != "postfix.service" || item.UnitFileState != "enabled" || item.Result != "success" || item.ExecMainStatus != "0" || item.NRestarts != "0" {
		t.Fatalf("basic extra fields = %+v", item)
	}
	if item.TasksCurrent != "4" || item.ControlGroup != "/system.slice/postfix.service" || item.Slice != "system.slice" || item.Restart != "no" || item.RestartSec != "100ms" {
		t.Fatalf("extended fields = %+v", item)
	}
	if !strings.Contains(item.ExecStart, "/usr/sbin/postfix") {
		t.Fatalf("exec start = %q, want postfix path", item.ExecStart)
	}
}
