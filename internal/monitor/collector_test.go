package monitor

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRemoteScriptSyntax(t *testing.T) {
	if out, err := exec.Command("sh", "-n", "-c", remoteScript).CombinedOutput(); err != nil {
		t.Fatalf("remoteScript syntax error: %v\n%s", err, out)
	}
}

func TestRemoteScriptDockerUsesSudoFallback(t *testing.T) {
	if !strings.Contains(remoteScript, "sudo -n docker ps -a") {
		t.Fatalf("remoteScript should use sudo fallback for docker container counts")
	}
}

func TestRemoteScriptDockerDistinguishesPermissionAndMissing(t *testing.T) {
	for _, want := range []string{"DOCKER_STATUS=ok", "DOCKER_STATUS=permission", "DOCKER_STATUS=not_installed"} {
		if !strings.Contains(remoteScript, want) {
			t.Fatalf("remoteScript missing %s", want)
		}
	}
}

func TestParseMetricsUsesAvailableMemory(t *testing.T) {
	metrics, err := parseMetrics("MEM=1000 700 400\n")
	if err != nil {
		t.Fatal(err)
	}

	if metrics.MemUsed != 600 {
		t.Fatalf("MemUsed = %d, want 600", metrics.MemUsed)
	}
	if got := metrics.MemPercent(); got != 60 {
		t.Fatalf("MemPercent = %v, want 60", got)
	}
}

func TestDiskPercentUsesDfUsableSpace(t *testing.T) {
	metrics, err := parseMetrics("DISK=1000 900 50\n")
	if err != nil {
		t.Fatal(err)
	}

	if got := metrics.DiskPercent(); got != 900.0/950.0*100 {
		t.Fatalf("DiskPercent = %v, want %v", got, 900.0/950.0*100)
	}
}

func TestDiskPercentUsesDfUsableSpaceWhenFull(t *testing.T) {
	metrics, err := parseMetrics("DISK=1000 950 0\n")
	if err != nil {
		t.Fatal(err)
	}

	if got := metrics.DiskPercent(); got != 100 {
		t.Fatalf("DiskPercent = %v, want 100", got)
	}
}

func TestParseMetricsDisksSelectsHighestRealDisk(t *testing.T) {
	metrics, err := parseMetrics(strings.Join([]string{
		"DISK=1000 100 900",
		"DISK_FS=/dev/sda1",
		"DISK_MOUNT=/",
		"DISKS=tmpfs\ttmpfs\t1000\t990\t10\t/run|/dev/sda1\text4\t1000\t400\t600\t/|/dev/sdb1\txfs\t1000\t910\t90\t/data|overlay\toverlay\t1000\t990\t10\t/var/lib/docker/overlay2/x",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}

	if len(metrics.Disks) != 2 {
		t.Fatalf("len(Disks) = %d, want 2: %+v", len(metrics.Disks), metrics.Disks)
	}
	if metrics.DiskFilesystem != "/dev/sdb1" || metrics.DiskMountpoint != "/data" {
		t.Fatalf("primary disk = %q %q, want /dev/sdb1 /data", metrics.DiskFilesystem, metrics.DiskMountpoint)
	}
	if got := metrics.DiskPercent(); got != 91 {
		t.Fatalf("DiskPercent = %v, want 91", got)
	}
}

func TestParseMetricsCPUInfo(t *testing.T) {
	metrics, err := parseMetrics("CPU_CORES=4\nCPU_MODEL=Intel Xeon Test\n")
	if err != nil {
		t.Fatal(err)
	}

	if metrics.CPUCores != 4 {
		t.Fatalf("CPUCores = %d, want 4", metrics.CPUCores)
	}
	if metrics.CPUModel != "Intel Xeon Test" {
		t.Fatalf("CPUModel = %q, want %q", metrics.CPUModel, "Intel Xeon Test")
	}
}

func TestParseMetricsDetailInfo(t *testing.T) {
	metrics, err := parseMetrics(strings.Join([]string{
		"HOSTNAME=server-1",
		"KERNEL=5.10.0-test",
		"ARCH=x86_64",
		"SWAP=2000 500 1500",
		"DISK_FS=/dev/xvda1",
		"DISK_MOUNT=/",
		"INODE=1000 250 750",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}

	if metrics.RemoteHostname != "server-1" {
		t.Fatalf("RemoteHostname = %q, want %q", metrics.RemoteHostname, "server-1")
	}
	if metrics.Kernel != "5.10.0-test" {
		t.Fatalf("Kernel = %q, want %q", metrics.Kernel, "5.10.0-test")
	}
	if metrics.Arch != "x86_64" {
		t.Fatalf("Arch = %q, want %q", metrics.Arch, "x86_64")
	}
	if metrics.SwapTotal != 2000 || metrics.SwapUsed != 500 || metrics.SwapFree != 1500 {
		t.Fatalf("swap = %d/%d/%d, want 2000/500/1500", metrics.SwapTotal, metrics.SwapUsed, metrics.SwapFree)
	}
	if metrics.DiskFilesystem != "/dev/xvda1" || metrics.DiskMountpoint != "/" {
		t.Fatalf("disk = %q %q, want /dev/xvda1 /", metrics.DiskFilesystem, metrics.DiskMountpoint)
	}
	if got := metrics.InodePercent(); got != 25 {
		t.Fatalf("InodePercent = %v, want 25", got)
	}
}

func TestHealthPortsFromListeningPorts(t *testing.T) {
	ports := healthPorts([]int{80, 5432, 80, 70000}, "22,80,8080")
	if len(ports) != 2 {
		t.Fatalf("len(ports) = %d, want 2", len(ports))
	}
	if ports[0].Port != 80 || !ports[0].Healthy {
		t.Fatalf("ports[0] = %+v, want healthy 80", ports[0])
	}
	if ports[1].Port != 5432 || ports[1].Healthy {
		t.Fatalf("ports[1] = %+v, want unhealthy 5432", ports[1])
	}
}

func TestParseMetricsDockerStates(t *testing.T) {
	metrics, err := parseMetrics(strings.Join([]string{
		"DOCKER=2",
		"SERVICE_AVAILABLE=1",
		"SERVICE_TOTAL=8",
		"SERVICE_RUNNING=6",
		"SERVICE_STOPPED=1",
		"DOCKER_AVAILABLE=1",
		"DOCKER_STATUS=ok",
		"DOCKER_TOTAL=5",
		"DOCKER_STOPPED=2",
		"DOCKER_FAILED=1",
		"DOCKER_RUNNING_NAMES=web,db",
		"DOCKER_STOPPED_NAMES=api(exited),job(created)",
		"DOCKER_FAILED_NAMES=worker(restarting)",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}

	if metrics.DockerRunning != 2 || metrics.DockerTotal != 5 {
		t.Fatalf("docker running/total = %d/%d, want 2/5", metrics.DockerRunning, metrics.DockerTotal)
	}
	if !metrics.ServiceAvailable || metrics.ServiceTotal != 8 || metrics.ServiceRunning != 6 || metrics.ServiceStopped != 1 {
		t.Fatalf("service metrics = available:%v total:%d running:%d stopped:%d", metrics.ServiceAvailable, metrics.ServiceTotal, metrics.ServiceRunning, metrics.ServiceStopped)
	}
	if !metrics.DockerAvailable || metrics.DockerStatus != "ok" {
		t.Fatalf("docker status = available:%v status:%q, want ok", metrics.DockerAvailable, metrics.DockerStatus)
	}
	if metrics.DockerStopped != 2 || metrics.DockerFailed != 1 {
		t.Fatalf("docker stopped/failed = %d/%d, want 2/1", metrics.DockerStopped, metrics.DockerFailed)
	}
	if got := metrics.DockerNonRunningCount(); got != 3 {
		t.Fatalf("DockerNonRunningCount = %d, want 3", got)
	}
	if len(metrics.DockerRunningNames) != 2 || metrics.DockerRunningNames[0] != "web" {
		t.Fatalf("DockerRunningNames = %#v, want parsed running containers", metrics.DockerRunningNames)
	}
	if len(metrics.DockerStoppedNames) != 2 || metrics.DockerStoppedNames[0] != "api(exited)" {
		t.Fatalf("DockerStoppedNames = %#v, want parsed stopped containers", metrics.DockerStoppedNames)
	}
	if len(metrics.DockerFailedNames) != 1 || metrics.DockerFailedNames[0] != "worker(restarting)" {
		t.Fatalf("DockerFailedNames = %#v, want parsed failed containers", metrics.DockerFailedNames)
	}
}

func TestParseMetricsDockerUnavailable(t *testing.T) {
	metrics, err := parseMetrics(strings.Join([]string{
		"DOCKER_AVAILABLE=0",
		"DOCKER_STATUS=not_installed",
		"DOCKER=0",
		"DOCKER_TOTAL=0",
		"DOCKER_STOPPED=0",
		"DOCKER_FAILED=0",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if metrics.DockerAvailable || metrics.DockerStatus != "not_installed" {
		t.Fatalf("docker status = available:%v status:%q, want not_installed", metrics.DockerAvailable, metrics.DockerStatus)
	}
	if metrics.DockerTotal != 0 || metrics.DockerRunning != 0 || metrics.DockerStopped != 0 || metrics.DockerFailed != 0 {
		t.Fatalf("docker counts = total:%d running:%d stopped:%d failed:%d", metrics.DockerTotal, metrics.DockerRunning, metrics.DockerStopped, metrics.DockerFailed)
	}
}

func TestParseMetricsDockerPermissionDenied(t *testing.T) {
	metrics, err := parseMetrics(strings.Join([]string{
		"DOCKER_AVAILABLE=0",
		"DOCKER_STATUS=permission",
		"DOCKER=0",
		"DOCKER_TOTAL=0",
		"DOCKER_STOPPED=0",
		"DOCKER_FAILED=0",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if metrics.DockerAvailable || metrics.DockerStatus != "permission" {
		t.Fatalf("docker status = available:%v status:%q, want permission", metrics.DockerAvailable, metrics.DockerStatus)
	}
}
