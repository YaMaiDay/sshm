package resource

import (
	"fmt"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/remotescript"
)

func ServiceDetailScript() string {
	return `if ! command -v systemctl >/dev/null 2>&1; then
  echo "__SSHM_SYSTEMCTL_UNAVAILABLE__"
  exit 0
fi
units=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>/dev/null | awk '$1 ~ /\.service$/ {print $1}')
if [ -z "$units" ]; then
  out=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
  code=$?
  if [ "$code" -ne 0 ]; then
    echo "__SSHM_SYSTEMCTL_ERROR__"
    printf '%s\n' "$out"
    exit 0
  fi
  printf '%s\n' "$out"
  exit 0
fi
parsed=$(for unit in $units; do
  props=$(systemctl show "$unit" -p Id -p LoadState -p ActiveState -p SubState -p Description -p FragmentPath -p WorkingDirectory -p ExecStart -p MainPID -p ExecMainPID -p MemoryCurrent -p ActiveEnterTimestamp -p InactiveEnterTimestamp -p StateChangeTimestamp -p ExecMainStartTimestamp -p ExecMainExitTimestamp -p UnitFileState --no-pager 2>/dev/null)
  [ -n "$props" ] || continue
  get_prop() { printf '%s\n' "$props" | awk -F= -v key="$1" '$1==key{print substr($0, index($0,"=")+1); exit}'; }
  id=$(get_prop Id)
  [ -n "$id" ] || continue
  load=$(get_prop LoadState)
  active=$(get_prop ActiveState)
  sub=$(get_prop SubState)
  desc=$(get_prop Description)
  fragment=$(get_prop FragmentPath)
  workdir=$(get_prop WorkingDirectory)
  execstart=$(get_prop ExecStart)
  mainpid=$(get_prop MainPID)
  execmainpid=$(get_prop ExecMainPID)
  memorycurrent=$(get_prop MemoryCurrent)
  activeenter=$(get_prop ActiveEnterTimestamp)
  inactiveenter=$(get_prop InactiveEnterTimestamp)
  statechange=$(get_prop StateChangeTimestamp)
  execmainstart=$(get_prop ExecMainStartTimestamp)
  execmainexit=$(get_prop ExecMainExitTimestamp)
  unitfilestate=$(get_prop UnitFileState)
  pid="$mainpid"
  [ -n "$pid" ] && [ "$pid" != "0" ] || pid="$execmainpid"
  if { [ -z "$memorycurrent" ] || [ "$memorycurrent" = "[not set]" ] || [ "$memorycurrent" = "18446744073709551615" ]; } && [ -n "$pid" ] && [ "$pid" != "0" ]; then
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | awk 'NR==1{print $1}')
    case "$rss" in
      ''|*[!0-9]*) ;;
      *) memorycurrent=$((rss * 1024)) ;;
    esac
  fi
  if [ -z "$activeenter" ] || [ "$activeenter" = "n/a" ]; then
    activeenter="$execmainstart"
  fi
  if [ -z "$activeenter" ] || [ "$activeenter" = "n/a" ]; then
    activeenter="$statechange"
  fi
  printf "__SSHM_SERVICE__\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n" "$id" "$load" "$active" "$sub" "$desc" "$fragment" "$workdir" "$execstart" "$mainpid" "$execmainpid" "$memorycurrent" "$activeenter" "$inactiveenter" "$statechange" "$execmainstart" "$execmainexit" "$unitfilestate"
done)
if [ -n "$parsed" ]; then
  printf '%s\n' "$parsed"
  exit 0
fi
fallback=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
fallback_code=$?
if [ "$fallback_code" -ne 0 ]; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$fallback"
exit 0`
}

func ServiceListScript() string {
	return `if ! command -v systemctl >/dev/null 2>&1; then
  echo "__SSHM_SYSTEMCTL_UNAVAILABLE__"
  exit 0
fi
out=$(systemctl list-units --type=service --all --no-legend --plain --no-pager 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out"
exit 0`
}

func ServiceExtraDetailScript(unit string) string {
	quoted := remotescript.Quote(unit)
	return fmt.Sprintf(`if ! command -v systemctl >/dev/null 2>&1; then
  echo "__SSHM_SYSTEMCTL_UNAVAILABLE__"
  exit 0
fi
props=$(systemctl show %s -p Id -p LoadState -p ActiveState -p SubState -p Description -p FragmentPath -p WorkingDirectory -p ExecStart -p ExecStop -p ExecReload -p MainPID -p ExecMainPID -p MemoryCurrent -p ActiveEnterTimestamp -p InactiveEnterTimestamp -p StateChangeTimestamp -p ExecMainStartTimestamp -p ExecMainExitTimestamp -p UnitFileState -p Result -p ExecMainStatus -p NRestarts -p TasksCurrent -p ControlGroup -p Slice -p User -p Group -p Restart -p RestartUSec -p DropInPaths --no-pager 2>&1)
code=$?
if [ "$code" -ne 0 ] && ! printf '%%s\n' "$props" | grep -q '^Id='; then
  echo "__SSHM_SYSTEMCTL_ERROR__"
  printf '%%s\n' "$props"
  exit 0
fi
get_prop() { printf '%%s\n' "$props" | awk -F= -v key="$1" '$1==key{print substr($0, index($0,"=")+1); exit}'; }
printf '%%s\n' "$props"`, quoted)
}

func PortDetailScript() string {
	return `run_ports() {
  if command -v ss >/dev/null 2>&1; then
    ss -H -tulnp 2>&1 || ss -tulnp 2>&1
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -tulnp 2>&1
    return $?
  fi
  return 127
}
run_ports_sudo() {
  if command -v ss >/dev/null 2>&1; then
    sudo -n ss -H -tulnp 2>&1 || sudo -n ss -tulnp 2>&1
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    sudo -n netstat -tulnp 2>&1
    return $?
  fi
  return 127
}
if ! command -v ss >/dev/null 2>&1 && ! command -v netstat >/dev/null 2>&1; then
  echo "__SSHM_SS_UNAVAILABLE__"
  exit 0
fi
out=$(run_ports)
code=$?
if [ "$code" -eq 0 ] && ! printf '%s\n' "$out" | grep -q 'users:(' && ! printf '%s\n' "$out" | grep -Eq '[0-9]+/[^[:space:]]+'; then
  sudo_out=$(run_ports_sudo)
  sudo_code=$?
  if [ "$sudo_code" -eq 0 ]; then
    out="$sudo_out"
  fi
fi
if [ "$code" -ne 0 ]; then
  sudo_out=$(run_ports_sudo)
  sudo_code=$?
  if [ "$sudo_code" -ne 0 ]; then
    echo "__SSHM_SS_PERMISSION__"
    printf '%s\n' "$sudo_out"
    exit 0
  fi
  out="$sudo_out"
fi
printf '%s\n' "$out"
printf '%s\n' "$out" | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p; s/.*[[:space:]]\([0-9][0-9]*\)\/[^[:space:]]*.*/\1/p' | sort -u | while read -r pid; do
  [ -n "$pid" ] || continue
  unit=$(cat "/proc/$pid/cgroup" 2>/dev/null | sed -n 's|.*[:/]\([^/:]*\.service\).*|\1|p' | head -n 1)
  [ -n "$unit" ] && printf '__SSHM_PORT_CGROUP__\t%s\t%s\n' "$pid" "$unit"
done`
}

func ProcessExtraDetailScript(pid string) string {
	quoted := remotescript.Quote(pid)
	return fmt.Sprintf(`pid=%s
case "$pid" in
  ''|*[!0-9]*)
    echo "__SSHM_PROCESS_INVALID__"
    exit 0
    ;;
esac
if [ ! -d "/proc/$pid" ]; then
  echo "__SSHM_PROCESS_NOT_FOUND__"
  exit 0
fi
ps_value() {
  ps -p "$pid" -o "$1=" 2>/dev/null | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' | head -n 1
}
ppid=$(ps_value ppid)
user=$(ps_value user)
state=$(ps_value stat)
cpu=$(ps_value pcpu)
mem=$(ps_value pmem)
rss=$(ps_value rss)
elapsed=$(ps_value etime)
started=$(ps_value lstart)
comm=$(ps_value comm)
cmdline=$(tr '\000' ' ' <"/proc/$pid/cmdline" 2>/dev/null | sed -e 's/[[:space:]]*$//')
[ -z "$cmdline" ] && cmdline=$(ps_value args)
cwd=$(readlink "/proc/$pid/cwd" 2>/dev/null || true)
exe=$(readlink "/proc/$pid/exe" 2>/dev/null || true)
cgroup=$(cat "/proc/$pid/cgroup" 2>/dev/null | paste -sd ';' -)
unit=$(printf '%%s\n' "$cgroup" | sed -n 's|.*[:/]\([^/:;]*\.service\).*|\1|p' | head -n 1)
printf '__SSHM_PROCESS__\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\t%%s\n' "$pid" "$ppid" "$user" "$state" "$cpu" "$mem" "$rss" "$elapsed" "$started" "$comm" "$cmdline" "$cwd" "$exe" "$cgroup" "$unit"`, quoted)
}

func ContainerDetailScript() string {
	return `if ! command -v docker >/dev/null 2>&1; then
  echo "__SSHM_DOCKER_UNAVAILABLE__"
  exit 0
fi
out=$(docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
code=$?
if [ "$code" -ne 0 ]; then
  out=$(sudo -n docker ps -a --format '{{.Names}}	{{.Image}}	{{.Status}}	{{.Ports}}' 2>&1)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_DOCKER_PERMISSION__"
  printf '%s\n' "$out"
  exit 0
fi
printf '%s\n' "$out" | while IFS= read -r line; do
  [ -z "$line" ] && continue
  printf '__SSHM_CONTAINER__\t%s\n' "$line"
done
stats=$(docker stats --no-stream --format '{{.Name}}	{{.CPUPerc}}	{{.MemUsage}}	{{.MemPerc}}' 2>/dev/null)
stats_code=$?
if [ "$stats_code" -ne 0 ]; then
  stats=$(sudo -n docker stats --no-stream --format '{{.Name}}	{{.CPUPerc}}	{{.MemUsage}}	{{.MemPerc}}' 2>/dev/null)
  stats_code=$?
fi
if [ "$stats_code" -eq 0 ]; then
  printf '%s\n' "$stats" | while IFS= read -r line; do
    [ -z "$line" ] && continue
    printf '__SSHM_CONTAINER_STATS__\t%s\n' "$line"
  done
fi
ids=$(docker ps -aq 2>/dev/null)
limits_code=$?
if [ "$limits_code" -ne 0 ]; then
  ids=$(sudo -n docker ps -aq 2>/dev/null)
  limits_code=$?
fi
if [ "$limits_code" -eq 0 ] && [ -n "$ids" ]; then
  limits=$(docker inspect --format '{{.Name}}	{{.HostConfig.NanoCpus}}	{{.HostConfig.CpuQuota}}	{{.HostConfig.CpuPeriod}}	{{.HostConfig.CpusetCpus}}' $ids 2>/dev/null)
  limits_code=$?
  if [ "$limits_code" -ne 0 ]; then
    limits=$(sudo -n docker inspect --format '{{.Name}}	{{.HostConfig.NanoCpus}}	{{.HostConfig.CpuQuota}}	{{.HostConfig.CpuPeriod}}	{{.HostConfig.CpusetCpus}}' $ids 2>/dev/null)
    limits_code=$?
  fi
  if [ "$limits_code" -eq 0 ]; then
    printf '%s\n' "$limits" | while IFS= read -r line; do
      [ -z "$line" ] && continue
      printf '__SSHM_CONTAINER_LIMIT__\t%s\n' "$line"
    done
  fi
fi`
}

func ContainerExtraDetailScript(name string) string {
	quoted := remotescript.Quote(name)
	filter := remotescript.Quote("name=^/" + name + "$")
	return fmt.Sprintf(`if ! command -v docker >/dev/null 2>&1; then
  echo "__SSHM_DOCKER_UNAVAILABLE__"
  exit 0
fi
run_docker() {
  docker "$@" 2>&1
}
run_docker_sudo() {
  sudo -n docker "$@" 2>&1
}
inspect=$(run_docker inspect --size --format '{{json .}}' %s)
code=$?
if [ "$code" -ne 0 ]; then
  inspect=$(run_docker_sudo inspect --size --format '{{json .}}' %s)
  code=$?
fi
if [ "$code" -ne 0 ]; then
  echo "__SSHM_DOCKER_PERMISSION__"
  printf '%%s\n' "$inspect"
  exit 0
fi
printf '__SSHM_CONTAINER_INSPECT__\t%%s\n' "$inspect"
size=$(run_docker ps -a --filter %s --size --format '{{.Size}}')
code=$?
if [ "$code" -ne 0 ]; then
  size=$(run_docker_sudo ps -a --filter %s --size --format '{{.Size}}')
fi
[ -n "$size" ] && printf '__SSHM_CONTAINER_SIZE__\t%%s\n' "$size"
blockio=$(run_docker stats --no-stream --format '{{.BlockIO}}' %s)
code=$?
if [ "$code" -ne 0 ]; then
  blockio=$(run_docker_sudo stats --no-stream --format '{{.BlockIO}}' %s)
fi
[ -n "$blockio" ] && printf '__SSHM_CONTAINER_BLOCKIO__\t%%s\n' "$blockio"`, quoted, quoted, filter, filter, quoted, quoted)
}

func ActionScript(kind string, command string, name string) string {
	if !isDefaultActionCommand(command) {
		return ""
	}
	if kind == config.ResourceKindProcess || kind == config.ResourceKindPort || kind == config.ResourceKindDatabase {
		return ""
	}
	target := remotescript.Quote(name)
	if kind == config.ResourceKindService {
		return remotescript.SudoFallback("systemctl "+command+" "+target, "sudo -n systemctl "+command+" "+target)
	}
	if kind == config.ResourceKindContainer {
		return remotescript.SudoFallback("docker "+command+" "+target, "sudo -n docker "+command+" "+target)
	}
	return ""
}

func ManagedActionScript(kind string, command string, name string, managed config.ManagedResource) string {
	if cmd := managedActionCommand(command, managed); cmd != "" {
		return remotescript.SudoFallback(cmd, "sudo -n "+cmd)
	}
	return ActionScript(kind, command, name)
}

func ActionPreview(kind string, command string, name string, managed config.ManagedResource) string {
	if cmd := managedActionCommand(command, managed); cmd != "" {
		return cmd
	}
	if !isDefaultActionCommand(command) {
		return "-"
	}
	target := remotescript.Quote(name)
	if kind == config.ResourceKindService {
		return "systemctl " + command + " " + target
	}
	if kind == config.ResourceKindProcess || kind == config.ResourceKindPort || kind == config.ResourceKindDatabase {
		return "-"
	}
	if kind == config.ResourceKindContainer {
		return "docker " + command + " " + target
	}
	return "-"
}

func isDefaultActionCommand(command string) bool {
	switch command {
	case "start", "stop", "restart":
		return true
	default:
		return false
	}
}

func managedActionCommand(command string, managed config.ManagedResource) string {
	switch command {
	case "start":
		return remotescript.UserCommand(managed.StartCommand)
	case "stop":
		return remotescript.UserCommand(managed.StopCommand)
	case "restart":
		return remotescript.UserCommand(managed.RestartCommand)
	default:
		return ""
	}
}

func LogScript(kind string, name string, lines int) string {
	if lines <= 0 {
		lines = 200
	}
	target := remotescript.Quote(name)
	if kind == config.ResourceKindService {
		cmd := fmt.Sprintf("journalctl -u %s -n %d --no-pager", target, lines)
		sudoCmd := fmt.Sprintf("sudo -n journalctl -u %s -n %d --no-pager", target, lines)
		return remotescript.SudoFallback(cmd, sudoCmd)
	}
	if kind == config.ResourceKindProcess || kind == config.ResourceKindPort || kind == config.ResourceKindDatabase {
		return ""
	}
	if kind == config.ResourceKindContainer {
		cmd := fmt.Sprintf("docker logs --tail %d %s", lines, target)
		sudoCmd := fmt.Sprintf("sudo -n docker logs --tail %d %s", lines, target)
		return remotescript.SudoFallback(cmd, sudoCmd)
	}
	return ""
}

func ManagedLogScript(kind string, name string, lines int, managed config.ManagedResource) string {
	if cmd := remotescript.UserCommand(managed.LogCommand); cmd != "" {
		return remotescript.SudoFallback(cmd, "sudo -n "+cmd)
	}
	return LogScript(kind, name, lines)
}

func LogPreview(kind string, name string, lines int, managed config.ManagedResource) string {
	if cmd := remotescript.UserCommand(managed.LogCommand); cmd != "" {
		return cmd
	}
	if lines <= 0 {
		lines = 200
	}
	target := remotescript.Quote(name)
	if kind == config.ResourceKindService {
		return fmt.Sprintf("journalctl -u %s -n %d --no-pager", target, lines)
	}
	if kind == config.ResourceKindProcess || kind == config.ResourceKindPort || kind == config.ResourceKindDatabase {
		return "-"
	}
	if kind == config.ResourceKindContainer {
		return fmt.Sprintf("docker logs --tail %d %s", lines, target)
	}
	return "-"
}
