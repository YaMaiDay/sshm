package tui

import (
	"strings"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func resourceServiceVisible(item resourceservice.ServiceDetail) bool {
	if item.Managed {
		return true
	}
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if serviceNotFoundInactiveDead(item) {
		return false
	}
	return true
}

func serviceNotFoundInactiveDead(item resourceservice.ServiceDetail) bool {
	return strings.EqualFold(strings.TrimSpace(item.Load), "not-found") &&
		strings.EqualFold(strings.TrimSpace(item.Active), "inactive") &&
		strings.EqualFold(strings.TrimSpace(item.Sub), "dead")
}

func (m Model) resourceServiceDiscoveredVisible(item resourceservice.ServiceDetail, searching bool) bool {
	if item.Missing {
		return false
	}
	if item.Managed {
		return true
	}
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if serviceLooksUserManaged(item) {
		return true
	}
	if serviceLooksInstalledPackage(item) {
		return true
	}
	if m.serviceOwnsListeningPort(item) {
		return true
	}
	if serviceLooksSystemManaged(item) {
		return false
	}
	if serviceDetailKind(item) == "failed" {
		return true
	}
	if !serviceHasDiscoveryMetadata(item) {
		return false
	}
	return searching
}

func (m Model) serviceOwnsListeningPort(item resourceservice.ServiceDetail) bool {
	if serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if m.resourceState.HostIndex < 0 || m.resourceState.HostIndex >= len(m.states) {
		return false
	}
	for _, port := range m.states[m.resourceState.HostIndex].PortDetails {
		if m.serviceMatchesPort(item, port) {
			return true
		}
	}
	return false
}

func (m Model) portLooksStandaloneProcess(port resourceservice.PortDetail) bool {
	if port.ProcessManaged {
		return true
	}
	if port.Missing || strings.TrimSpace(port.Process) == "" || strings.TrimSpace(port.Container) != "" {
		return false
	}
	process := strings.ToLower(strings.TrimSpace(port.Process))
	if serviceIsKnownInfrastructureProcess(process) {
		return false
	}
	for _, service := range m.states[m.resourceState.HostIndex].ServiceDetails {
		if m.serviceMatchesPort(service, port) {
			return false
		}
	}
	return true
}

func (m Model) serviceMatchesPort(item resourceservice.ServiceDetail, port resourceservice.PortDetail) bool {
	if item.Missing || serviceIsKnownInfrastructure(item.Unit) {
		return false
	}
	if strings.TrimSpace(port.ServiceUnit) != "" && strings.TrimSpace(port.ServiceUnit) == strings.TrimSpace(item.Unit) {
		return true
	}
	process := strings.ToLower(strings.TrimSpace(port.Process))
	if process == "" || serviceIsKnownInfrastructureProcess(process) {
		return false
	}
	pids := serviceCandidatePIDs(item)
	if strings.TrimSpace(port.PID) != "" {
		if _, ok := pids[strings.TrimSpace(port.PID)]; ok {
			return true
		}
	}
	names := serviceCandidateProcessNames(item)
	_, ok := names[process]
	return ok
}

func serviceCandidatePIDs(item resourceservice.ServiceDetail) map[string]struct{} {
	out := map[string]struct{}{}
	for _, pid := range []string{item.MainPID, item.ExecMainPID} {
		pid = strings.TrimSpace(pid)
		if pid == "" || pid == "0" || pid == "-" {
			continue
		}
		out[pid] = struct{}{}
	}
	return out
}

func serviceCandidateProcessNames(item resourceservice.ServiceDetail) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimSuffix(value, ".service")
		if idx := strings.Index(value, "@"); idx >= 0 {
			value = value[:idx]
		}
		value = strings.Trim(value, "\"'{}();")
		if value == "" || strings.Contains(value, "/") || strings.Contains(value, " ") {
			return
		}
		out[value] = struct{}{}
	}
	add(item.Unit)
	for _, token := range strings.Fields(strings.NewReplacer(";", " ", "{", " ", "}", " ").Replace(item.ExecStart)) {
		token = strings.Trim(token, "\"'(),")
		if strings.HasPrefix(token, "path=") {
			token = strings.TrimPrefix(token, "path=")
		}
		if strings.HasPrefix(token, "argv[]=") {
			token = strings.TrimPrefix(token, "argv[]=")
		}
		if strings.Contains(token, "/") {
			if idx := strings.LastIndex(token, "/"); idx >= 0 && idx < len(token)-1 {
				add(token[idx+1:])
			}
		}
	}
	return out
}

func serviceIsKnownInfrastructureProcess(process string) bool {
	process = strings.ToLower(strings.TrimSpace(process))
	if process == "" {
		return false
	}
	exact := map[string]bool{
		"sshd":              true,
		"ssh":               true,
		"chronyd":           true,
		"systemd-network":   true,
		"systemd-networkd":  true,
		"systemd-resolve":   true,
		"systemd-resolved":  true,
		"networkmanager":    true,
		"dhclient":          true,
		"rpcbind":           true,
		"rpc.statd":         true,
		"docker-proxy":      true,
		"containerd":        true,
		"containerd-shim":   true,
		"containerd-shim-r": true,
	}
	return exact[process]
}

func serviceIsKnownInfrastructure(unit string) bool {
	unit = strings.ToLower(strings.TrimSpace(unit))
	if unit == "" {
		return false
	}
	exact := map[string]bool{
		"acpid.service":               true,
		"atd.service":                 true,
		"auditd.service":              true,
		"auth-rpcgss-module.service":  true,
		"chrony.service":              true,
		"chronyd.service":             true,
		"cloud-config.service":        true,
		"cloud-final.service":         true,
		"cloud-init-local.service":    true,
		"cloud-init.service":          true,
		"cloud-init.target":           true,
		"cloud-init-hotplugd.service": true,
		"cron.service":                true,
		"crond.service":               true,
		"dbus.service":                true,
		"dbus-broker.service":         true,
		"fstrim.service":              true,
		"gssproxy.service":            true,
		"irqbalance.service":          true,
		"libstoragemgmt.service":      true,
		"networkmanager.service":      true,
		"network.service":             true,
		"ntpd.service":                true,
		"ntpdate.service":             true,
		"rpc-statd-notify.service":    true,
		"rpcbind.service":             true,
		"sshd.service":                true,
		"ssh.service":                 true,
		"sysstat.service":             true,
	}
	if exact[unit] {
		return true
	}
	for _, prefix := range []string{
		"cloud-init@",
		"dracut-",
		"emergency.",
		"getty@",
		"initrd-",
		"plymouth-",
		"policy-routes@",
		"rescue.",
		"serial-getty@",
		"systemd-",
		"user-runtime-dir@",
		"user@",
	} {
		if strings.HasPrefix(unit, prefix) {
			return true
		}
	}
	return false
}

func serviceHasDiscoveryMetadata(item resourceservice.ServiceDetail) bool {
	return strings.TrimSpace(item.FragmentPath) != "" || strings.TrimSpace(item.WorkingDirectory) != "" || strings.TrimSpace(item.ExecStart) != ""
}

func serviceLooksUserManaged(item resourceservice.ServiceDetail) bool {
	fragment := strings.TrimSpace(item.FragmentPath)
	if strings.HasPrefix(fragment, "/etc/systemd/system/") || strings.HasPrefix(fragment, "/run/systemd/system/") {
		return true
	}
	text := strings.Join([]string{item.WorkingDirectory, item.ExecStart}, " ")
	for _, marker := range []string{"/opt/", "/data/", "/www/", "/var/www/", "/home/", "/srv/", "/usr/local/", "/etc/openvpn/", "/etc/nginx/", "/etc/supervisor/"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func serviceLooksInstalledPackage(item resourceservice.ServiceDetail) bool {
	unit := strings.ToLower(strings.TrimSpace(item.Unit))
	unitBase := strings.TrimSuffix(unit, ".service")
	if idx := strings.Index(unitBase, "@"); idx >= 0 {
		unitBase = unitBase[:idx]
	}
	exact := map[string]bool{
		"caddy":             true,
		"clickhouse":        true,
		"clickhouse-server": true,
		"containerd":        true,
		"docker":            true,
		"elasticsearch":     true,
		"frpc":              true,
		"frps":              true,
		"grafana-server":    true,
		"ipsec":             true,
		"kafka":             true,
		"kibana":            true,
		"logstash":          true,
		"mariadb":           true,
		"mongod":            true,
		"mongodb":           true,
		"mysql":             true,
		"nats":              true,
		"nginx":             true,
		"node_exporter":     true,
		"openvpn":           true,
		"openvpn-server":    true,
		"postgresql":        true,
		"prometheus":        true,
		"rabbitmq-server":   true,
		"redis":             true,
		"redis-server":      true,
		"supervisord":       true,
		"x-ui":              true,
		"xl2tpd":            true,
		"xray":              true,
		"v2ray":             true,
	}
	return exact[unitBase]
}

func serviceLooksSystemManaged(item resourceservice.ServiceDetail) bool {
	fragment := strings.TrimSpace(item.FragmentPath)
	if fragment == "" {
		return false
	}
	systemFragments := []string{
		"/usr/lib/systemd/system/",
		"/lib/systemd/system/",
	}
	inSystemDir := false
	for _, prefix := range systemFragments {
		if strings.HasPrefix(fragment, prefix) {
			inSystemDir = true
			break
		}
	}
	if !inSystemDir {
		return false
	}
	text := strings.Join([]string{item.WorkingDirectory, item.ExecStart}, " ")
	if strings.TrimSpace(text) == "" {
		return true
	}
	for _, marker := range []string{"/usr/lib/", "/lib/", "/usr/sbin/", "/usr/bin/", "/sbin/", "/bin/", "/run/", "/var/lib/", "/var/run/"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
