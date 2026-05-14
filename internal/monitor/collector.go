package monitor

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/sshconfig"
)

type Collector struct {
	Passwords      config.PasswordStore
	Timeout        time.Duration
	ConnectTimeout time.Duration
}

func NewCollector(passwords config.PasswordStore) Collector {
	return Collector{Passwords: passwords, Timeout: 6 * time.Second, ConnectTimeout: 3 * time.Second}
}

func (c Collector) Collect(ctx context.Context, h host.Host) Metrics {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	args, target, cleanup := sshconfig.SSHArgs(h,
		"-o", "ConnectTimeout="+sshSeconds(c.ConnectTimeout),
		"-o", "LogLevel=ERROR",
	)
	defer cleanup()
	args = append(args, target, remoteScript)

	var cmd *exec.Cmd
	var tempFile string
	password := strings.TrimSpace(h.Password)
	if password == "" {
		password, _ = c.Passwords.Password(h.Name)
	}
	if password != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			file, err := sshconfig.TempPasswordFile(password)
			if err == nil {
				tempFile = file
				defer os.Remove(tempFile)
				passwordArgs := append(passwordSSHOptions(h), args...)
				fullArgs := append([]string{"-f", tempFile, "ssh"}, passwordArgs...)
				cmd = exec.CommandContext(ctx, "sshpass", fullArgs...)
			}
		}
	}
	if cmd == nil {
		cmd = exec.CommandContext(ctx, "ssh", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return Metrics{Online: false, Error: cleanError(err, output), UpdatedAt: time.Now()}
	}
	metrics, err := parseMetrics(string(output))
	if err != nil {
		return Metrics{Online: false, Error: err.Error(), UpdatedAt: time.Now()}
	}
	metrics.Online = true
	metrics.HealthPorts = healthPorts(h.HealthPorts, metrics.Ports)
	metrics.UpdatedAt = time.Now()
	return metrics
}

func passwordSSHOptions(h host.Host) []string {
	return sshconfig.PasswordAuthArgs(h)
}

func sshSeconds(d time.Duration) string {
	if d <= 0 {
		d = 3 * time.Second
	}
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("%d", seconds)
}

func cleanError(err error, output []byte) string {
	text := strings.TrimSpace(string(output))
	if text != "" {
		lines := strings.Split(text, "\n")
		return lines[len(lines)-1]
	}
	if err != nil {
		return err.Error()
	}
	return "未知错误"
}

func parseMetrics(output string) (Metrics, error) {
	m := Metrics{}
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		values[parts[0]] = parts[1]
	}

	m.RemoteHostname = values["HOSTNAME"]
	m.OS = values["OS"]
	m.Kernel = values["KERNEL"]
	m.Arch = values["ARCH"]
	load := strings.Fields(values["LOAD"])
	if len(load) >= 3 {
		m.Load1, m.Load5, m.Load15 = load[0], load[1], load[2]
	}
	mem := strings.Fields(values["MEM"])
	if len(mem) >= 2 {
		m.MemTotal, _ = strconv.ParseUint(mem[0], 10, 64)
		m.MemUsed, _ = strconv.ParseUint(mem[1], 10, 64)
		if len(mem) >= 3 {
			m.MemAvailable, _ = strconv.ParseUint(mem[2], 10, 64)
			if m.MemTotal >= m.MemAvailable {
				m.MemUsed = m.MemTotal - m.MemAvailable
			}
		}
	}
	swap := strings.Fields(values["SWAP"])
	if len(swap) >= 3 {
		m.SwapTotal, _ = strconv.ParseUint(swap[0], 10, 64)
		m.SwapUsed, _ = strconv.ParseUint(swap[1], 10, 64)
		m.SwapFree, _ = strconv.ParseUint(swap[2], 10, 64)
	}
	disk := strings.Fields(values["DISK"])
	if len(disk) >= 2 {
		m.DiskTotal, _ = strconv.ParseUint(disk[0], 10, 64)
		m.DiskUsed, _ = strconv.ParseUint(disk[1], 10, 64)
		if len(disk) >= 3 {
			m.DiskAvailable, _ = strconv.ParseUint(disk[2], 10, 64)
			m.DiskAvailKnown = true
		}
	}
	m.DiskFilesystem = values["DISK_FS"]
	m.DiskMountpoint = values["DISK_MOUNT"]
	m.Disks = parseDisks(values["DISKS"])
	if len(m.Disks) > 0 {
		primary := m.Disks[0]
		for _, disk := range m.Disks[1:] {
			if disk.Percent() > primary.Percent() {
				primary = disk
			}
		}
		m.DiskTotal = primary.Total
		m.DiskUsed = primary.Used
		m.DiskAvailable = primary.Available
		m.DiskAvailKnown = primary.AvailKnown
		m.DiskFilesystem = primary.Filesystem
		m.DiskType = primary.Type
		m.DiskMountpoint = primary.Mountpoint
	}
	inode := strings.Fields(values["INODE"])
	if len(inode) >= 3 {
		m.InodeTotal, _ = strconv.ParseUint(inode[0], 10, 64)
		m.InodeUsed, _ = strconv.ParseUint(inode[1], 10, 64)
		m.InodeAvailable, _ = strconv.ParseUint(inode[2], 10, 64)
	}
	m.CPUPercent = cpuPercent(values["CPU1"], values["CPU2"])
	m.CPUCores, _ = strconv.Atoi(strings.TrimSpace(values["CPU_CORES"]))
	m.CPUModel = values["CPU_MODEL"]
	m.Uptime = values["UPTIME"]
	m.DockerRunning, _ = strconv.Atoi(strings.TrimSpace(values["DOCKER"]))
	m.DockerTotal, _ = strconv.Atoi(strings.TrimSpace(values["DOCKER_TOTAL"]))
	m.DockerStopped, _ = strconv.Atoi(strings.TrimSpace(values["DOCKER_STOPPED"]))
	m.DockerFailed, _ = strconv.Atoi(strings.TrimSpace(values["DOCKER_FAILED"]))
	m.DockerRunningNames = splitList(values["DOCKER_RUNNING_NAMES"])
	m.DockerStoppedNames = splitList(values["DOCKER_STOPPED_NAMES"])
	m.DockerFailedNames = splitList(values["DOCKER_FAILED_NAMES"])
	if m.DockerTotal == 0 && m.DockerRunning > 0 {
		m.DockerTotal = m.DockerRunning
	}
	m.FailedServices, _ = strconv.Atoi(strings.TrimSpace(values["FAILED"]))
	m.FailedUnits = splitList(values["FAILED_UNITS"])
	m.Ports = values["PORTS"]
	return m, nil
}

func splitList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseDisks(value string) []DiskMetric {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	out := []DiskMetric{}
	for _, row := range strings.Split(value, "|") {
		fields := strings.Split(row, "\t")
		if len(fields) < 6 {
			continue
		}
		diskType := strings.TrimSpace(fields[1])
		filesystem := strings.TrimSpace(fields[0])
		if !realDiskFilesystem(filesystem, diskType) {
			continue
		}
		total, _ := strconv.ParseUint(fields[2], 10, 64)
		used, _ := strconv.ParseUint(fields[3], 10, 64)
		available, _ := strconv.ParseUint(fields[4], 10, 64)
		mountpoint := strings.TrimSpace(fields[5])
		if total == 0 || mountpoint == "" {
			continue
		}
		out = append(out, DiskMetric{
			Filesystem: filesystem,
			Type:       diskType,
			Mountpoint: mountpoint,
			Total:      total,
			Used:       used,
			Available:  available,
			AvailKnown: true,
		})
	}
	return out
}

func realDiskFilesystem(filesystem, diskType string) bool {
	diskType = strings.ToLower(strings.TrimSpace(diskType))
	switch diskType {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "zfs", "f2fs", "jfs", "reiserfs":
		return true
	case "tmpfs", "devtmpfs", "proc", "sysfs", "cgroup", "cgroup2", "overlay", "squashfs", "ramfs", "debugfs", "tracefs", "fusectl", "nsfs", "autofs", "binfmt_misc", "securityfs", "pstore", "efivarfs", "configfs", "hugetlbfs", "mqueue":
		return false
	}
	filesystem = strings.TrimSpace(filesystem)
	return strings.HasPrefix(filesystem, "/dev/") || strings.HasPrefix(filesystem, "UUID=")
}

func healthPorts(ports []int, listening string) []HealthPort {
	if len(ports) == 0 {
		return nil
	}
	listeningPorts := map[int]bool{}
	for _, part := range strings.Split(listening, ",") {
		port, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil {
			listeningPorts[port] = true
		}
	}
	out := make([]HealthPort, 0, len(ports))
	seen := map[int]bool{}
	for _, port := range ports {
		if port < 1 || port > 65535 || seen[port] {
			continue
		}
		out = append(out, HealthPort{Port: port, Healthy: listeningPorts[port]})
		seen[port] = true
	}
	return out
}

func cpuPercent(a, b string) float64 {
	first := parseCPU(a)
	second := parseCPU(b)
	if len(first) < 5 || len(second) < 5 {
		return 0
	}
	idle1 := first[3] + first[4]
	idle2 := second[3] + second[4]
	total1 := sum(first)
	total2 := sum(second)
	totalDelta := total2 - total1
	idleDelta := idle2 - idle1
	if totalDelta <= 0 {
		return 0
	}
	return float64(totalDelta-idleDelta) * 100 / float64(totalDelta)
}

func parseCPU(line string) []uint64 {
	fields := strings.Fields(line)
	if len(fields) > 0 && fields[0] == "cpu" {
		fields = fields[1:]
	}
	out := make([]uint64, 0, len(fields))
	for _, field := range fields {
		value, _ := strconv.ParseUint(field, 10, 64)
		out = append(out, value)
	}
	return out
}

func sum(values []uint64) uint64 {
	var total uint64
	for _, value := range values {
		total += value
	}
	return total
}

const remoteScript = `sh -c '
echo HOSTNAME=$(hostname 2>/dev/null)
if [ -r /etc/os-release ]; then . /etc/os-release; echo OS="${PRETTY_NAME:-$NAME}"; else echo OS="$(uname -s 2>/dev/null)"; fi
echo KERNEL="$(uname -r 2>/dev/null)"
echo ARCH="$(uname -m 2>/dev/null)"
echo CPU1="$(awk '"'"'/^cpu /{print}'"'"' /proc/stat 2>/dev/null)"
sleep 0.5
echo CPU2="$(awk '"'"'/^cpu /{print}'"'"' /proc/stat 2>/dev/null)"
echo CPU_CORES="$(nproc 2>/dev/null || grep -c "^processor" /proc/cpuinfo 2>/dev/null)"
CPU_MODEL_VALUE="$(grep -m1 -E "model name|Hardware|Processor" /proc/cpuinfo 2>/dev/null | cut -d: -f2- | sed "s/^[[:space:]]*//")"
echo CPU_MODEL="$CPU_MODEL_VALUE"
echo LOAD="$(cat /proc/loadavg 2>/dev/null | awk '"'"'{print $1" "$2" "$3}'"'"')"
echo MEM="$(free -b 2>/dev/null | awk '"'"'/^Mem:/{print $2" "$3" "$7}'"'"')"
echo SWAP="$(free -b 2>/dev/null | awk '"'"'/^Swap:/{print $2" "$3" "$4}'"'"')"
echo DISK="$(df -P -B1 / 2>/dev/null | awk '"'"'NR==2{print $2" "$3" "$4}'"'"')"
echo DISK_FS="$(df -P -B1 / 2>/dev/null | awk '"'"'NR==2{print $1}'"'"')"
echo DISK_MOUNT="$(df -P -B1 / 2>/dev/null | awk '"'"'NR==2{print $6}'"'"')"
echo DISKS="$(df -PT -B1 2>/dev/null | awk '"'"'NR>1{printf "%s%s\t%s\t%s\t%s\t%s\t%s", sep, $1, $2, $3, $4, $5, $7; sep="|"}'"'"')"
echo INODE="$(df -Pi / 2>/dev/null | awk '"'"'NR==2{print $2" "$3" "$4}'"'"')"
echo UPTIME="$(uptime -p 2>/dev/null || uptime 2>/dev/null)"
echo DOCKER="$(docker ps -q 2>/dev/null | wc -l | tr -d '"'"' '"'"')"
DOCKER_LINES="$(docker ps -a --format '"'"'{{.Names}}|{{.State}}'"'"' 2>/dev/null)"
echo DOCKER_TOTAL="$(printf "%s\n" "$DOCKER_LINES" | awk '"'"'NF{c++} END{print c+0}'"'"')"
echo DOCKER_STOPPED="$(printf "%s\n" "$DOCKER_LINES" | awk -F"|" '"'"'$2=="exited" || $2=="created" || $2=="paused"{c++} END{print c+0}'"'"')"
echo DOCKER_FAILED="$(printf "%s\n" "$DOCKER_LINES" | awk -F"|" '"'"'$2=="restarting" || $2=="dead"{c++} END{print c+0}'"'"')"
DOCKER_RUNNING_NAMES_VALUE="$(printf "%s\n" "$DOCKER_LINES" | awk -F"|" '"'"'$2=="running"{print $1}'"'"' | paste -sd, - 2>/dev/null)"
echo DOCKER_RUNNING_NAMES="$DOCKER_RUNNING_NAMES_VALUE"
DOCKER_STOPPED_NAMES_VALUE="$(printf "%s\n" "$DOCKER_LINES" | awk -F"|" '"'"'$2=="exited" || $2=="created" || $2=="paused"{print $1"("$2")"}'"'"' | paste -sd, - 2>/dev/null)"
echo DOCKER_STOPPED_NAMES="$DOCKER_STOPPED_NAMES_VALUE"
DOCKER_FAILED_NAMES_VALUE="$(printf "%s\n" "$DOCKER_LINES" | awk -F"|" '"'"'$2=="restarting" || $2=="dead"{print $1"("$2")"}'"'"' | paste -sd, - 2>/dev/null)"
echo DOCKER_FAILED_NAMES="$DOCKER_FAILED_NAMES_VALUE"
FAILED_UNITS_VALUE="$(systemctl --failed --no-legend --plain 2>/dev/null | awk '"'"'{print $1}'"'"' | paste -sd, - 2>/dev/null)"
if [ -n "$FAILED_UNITS_VALUE" ]; then FAILED_COUNT="$(printf "%s" "$FAILED_UNITS_VALUE" | awk -F, '"'"'{print NF}'"'"')"; else FAILED_COUNT=0; fi
echo FAILED="$FAILED_COUNT"
echo FAILED_UNITS="$FAILED_UNITS_VALUE"
echo PORTS="$(ss -tlnH 2>/dev/null | awk '"'"'{print $4}'"'"' | sed '"'"'s/.*://'"'"' | sort -n | uniq | paste -sd, - 2>/dev/null)"
'`
