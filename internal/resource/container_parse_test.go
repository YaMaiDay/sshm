package resource

import (
	"strings"
	"testing"
)

func TestParseContainerDetailsWithDockerStats(t *testing.T) {
	out := strings.Join([]string{
		"__SSHM_CONTAINER__\tapi\tapp:latest\tUp 2 minutes\t80->80/tcp",
		"__SSHM_CONTAINER_STATS__\tapi\t0.12%\t32.4MiB / 1.9GiB\t1.65%",
		"__SSHM_CONTAINER_LIMIT__\t/api\t1500000000\t0\t0\t",
	}, "\n")
	items, errText := ParseContainerDetails(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want 1", items)
	}
	if items[0].CPU != "0.12%" || items[0].Memory != "32.4M/1.9G" || items[0].MemPerc != "1.65%" {
		t.Fatalf("container stats = %+v", items[0])
	}
	if !items[0].CPULimitKnown || items[0].NanoCpus != 1500000000 {
		t.Fatalf("container cpu limit = %+v", items[0])
	}
}

func TestParseContainerExtraDetail(t *testing.T) {
	inspect := `{"Id":"abcdef1234567890","Created":"2026-05-15T10:00:00Z","Path":"/app","Args":["serve"],"Driver":"overlay2","Platform":"linux","SizeRw":1234,"SizeRootFs":5678,"State":{"Status":"running","StartedAt":"2026-05-15T10:01:00Z","FinishedAt":"0001-01-01T00:00:00Z","ExitCode":0,"Health":{"Status":"healthy"}},"HostConfig":{"RestartPolicy":{"Name":"unless-stopped"},"NanoCpus":2000000000,"CpuQuota":0,"CpuPeriod":0,"CpusetCpus":""},"Mounts":[{"Type":"bind","Source":"/data/app","Destination":"/app/data","RW":true}],"NetworkSettings":{"Networks":{"customer_default":{"IPAddress":"172.18.0.3","Gateway":"172.18.0.1","MacAddress":"02:42:ac:12:00:03","NetworkID":"networkabcdef123456","EndpointID":"endpointabcdef123456","Aliases":["api","customer-app-1"]}}}}`
	out := strings.Join([]string{
		"__SSHM_CONTAINER_INSPECT__\t" + inspect,
		"__SSHM_CONTAINER_SIZE__\t12.3MB (virtual 1.2GB)",
		"__SSHM_CONTAINER_BLOCKIO__\t1.2MB / 3.4MB",
	}, "\n")
	detail, errText := ParseContainerExtraDetail(out)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if detail.ID != "abcdef1234567890" || detail.Size != "12.3MB" || detail.VirtualSize != "1.2GB" || detail.BlockIO != "1.2MB/3.4MB" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.SizeRW != 1234 || detail.SizeRootFS != 5678 || detail.HealthStatus != "healthy" || detail.RestartPolicy != "unless-stopped" {
		t.Fatalf("inspect fields = %+v", detail)
	}
	if detail.NanoCpus != 2000000000 {
		t.Fatalf("cpu limit fields = %+v", detail)
	}
	if len(detail.Mounts) != 1 || detail.Mounts[0].Source != "/data/app" || !detail.Mounts[0].RW {
		t.Fatalf("mounts = %+v", detail.Mounts)
	}
	if len(detail.Networks) != 1 || detail.Networks[0].Name != "customer_default" || detail.Networks[0].IPAddress != "172.18.0.3" || len(detail.Networks[0].Aliases) != 2 {
		t.Fatalf("networks = %+v", detail.Networks)
	}
}
