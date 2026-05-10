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

	args := []string{
		"-o", "ConnectTimeout=" + sshSeconds(c.ConnectTimeout),
		"-o", "LogLevel=ERROR",
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if h.Port != "" {
		args = append(args, "-p", h.Port)
	}
	if h.ProxyJump != "" {
		args = append(args, "-J", h.ProxyJump)
	}
	if h.IdentityFile != "" {
		args = append(args, "-i", h.IdentityFile)
	}
	args = append(args, h.Target(), remoteScript)

	var cmd *exec.Cmd
	var tempFile string
	password := strings.TrimSpace(h.Password)
	if password == "" {
		password, _ = c.Passwords.Password(h.Name)
	}
	if password != "" {
		if _, err := exec.LookPath("sshpass"); err == nil {
			f, err := os.CreateTemp("", "sshm-pass-*")
			if err == nil {
				tempFile = f.Name()
				_ = f.Chmod(0600)
				_, _ = f.WriteString(password + "\n")
				_ = f.Close()
				defer os.Remove(tempFile)
				passwordArgs := append([]string{
					"-o", "PreferredAuthentications=password",
					"-o", "PubkeyAuthentication=no",
				}, args...)
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
	metrics.UpdatedAt = time.Now()
	return metrics
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
	load := strings.Fields(values["LOAD"])
	if len(load) >= 3 {
		m.Load1, m.Load5, m.Load15 = load[0], load[1], load[2]
	}
	mem := strings.Fields(values["MEM"])
	if len(mem) >= 2 {
		m.MemTotal, _ = strconv.ParseUint(mem[0], 10, 64)
		m.MemUsed, _ = strconv.ParseUint(mem[1], 10, 64)
	}
	disk := strings.Fields(values["DISK"])
	if len(disk) >= 2 {
		m.DiskTotal, _ = strconv.ParseUint(disk[0], 10, 64)
		m.DiskUsed, _ = strconv.ParseUint(disk[1], 10, 64)
	}
	m.CPUPercent = cpuPercent(values["CPU1"], values["CPU2"])
	m.Uptime = values["UPTIME"]
	m.DockerRunning, _ = strconv.Atoi(strings.TrimSpace(values["DOCKER"]))
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
echo CPU1="$(awk '"'"'/^cpu /{print}'"'"' /proc/stat 2>/dev/null)"
sleep 0.25
echo CPU2="$(awk '"'"'/^cpu /{print}'"'"' /proc/stat 2>/dev/null)"
echo LOAD="$(cat /proc/loadavg 2>/dev/null | awk '"'"'{print $1" "$2" "$3}'"'"')"
echo MEM="$(free -b 2>/dev/null | awk '"'"'/^Mem:/{print $2" "$3}'"'"')"
echo DISK="$(df -P -B1 / 2>/dev/null | awk '"'"'NR==2{print $2" "$3}'"'"')"
echo UPTIME="$(uptime -p 2>/dev/null || uptime 2>/dev/null)"
echo DOCKER="$(docker ps -q 2>/dev/null | wc -l | tr -d '"'"' '"'"')"
FAILED_UNITS_VALUE="$(systemctl --failed --no-legend --plain 2>/dev/null | awk '"'"'{print $1}'"'"' | paste -sd, - 2>/dev/null)"
if [ -n "$FAILED_UNITS_VALUE" ]; then FAILED_COUNT="$(printf "%s" "$FAILED_UNITS_VALUE" | awk -F, '"'"'{print NF}'"'"')"; else FAILED_COUNT=0; fi
echo FAILED="$FAILED_COUNT"
echo FAILED_UNITS="$FAILED_UNITS_VALUE"
echo PORTS="$(ss -tlnH 2>/dev/null | awk '"'"'{print $4}'"'"' | sed '"'"'s/.*://'"'"' | sort -n | uniq | paste -sd, - 2>/dev/null)"
'`
