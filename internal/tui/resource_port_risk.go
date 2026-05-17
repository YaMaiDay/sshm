package tui

import (
	"strings"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func portListenText(item resourceservice.PortDetail) string {
	if strings.TrimSpace(item.LocalAddress) != "" {
		return item.LocalAddress
	}
	if strings.TrimSpace(item.Port) == "" {
		return ""
	}
	return strings.TrimSpace(item.Protocol + "/" + item.Port)
}

func portProcessDetailText(item resourceservice.PortDetail) string {
	process := emptyDash(item.Process)
	if strings.TrimSpace(item.PID) != "" {
		process += " pid:" + strings.TrimSpace(item.PID)
	}
	if strings.TrimSpace(item.FD) != "" {
		process += " fd:" + strings.TrimSpace(item.FD)
	}
	return process
}

func (m Model) portScopeText(item resourceservice.PortDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return mutedStyle.Render(m.t("Not found", "未发现"))
	case scope == portScopeUnknown:
		return mutedStyle.Render("-")
	case scope == portScopeLoopback:
		return greenStyle.Render(m.t("Loopback only", "仅本机"))
	case scope == portScopeWildcard:
		return redStyle.Render(m.t("All interfaces", "所有网卡"))
	default:
		return yellowStyle.Render(m.t("Specific address", "指定地址"))
	}
}

func (m Model) portRiskText(item resourceservice.PortDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return mutedStyle.Render("-")
	case scope == portScopeUnknown:
		return mutedStyle.Render(m.t("Unknown listener", "监听地址未知"))
	case scope == portScopeLoopback:
		return greenStyle.Render(m.t("Local only", "仅本机访问"))
	case scope == portScopeWildcard:
		return redStyle.Render(m.t("Public listener", "公网监听"))
	default:
		return yellowStyle.Render(m.t("Check firewall", "检查防火墙"))
	}
}

func (m Model) portScopeNote(item resourceservice.PortDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return m.t("The configured listener was not found.", "配置的监听端口未发现。")
	case scope == portScopeUnknown:
		return m.t("The listener address could not be parsed from ss/netstat output.", "无法从 ss/netstat 输出解析监听地址。")
	case scope == portScopeLoopback:
		return m.t("This port only listens on loopback and is normally reachable only from this server.", "当前端口只监听本机回环地址，通常只有这台服务器自己能访问。")
	case scope == portScopeWildcard:
		return m.t("This port listens on all local interfaces, including private and public NICs.", "当前端口监听所有本机网卡，包括内网网卡和公网网卡。")
	default:
		return m.t("This port listens on a specific local address only.", "当前端口只监听指定的本机地址。")
	}
}

func (m Model) portRiskNote(item resourceservice.PortDetail) string {
	scope := portAddressScope(item.LocalAddress)
	switch {
	case item.Missing:
		return "-"
	case scope == portScopeUnknown:
		return m.t("Risk cannot be judged without the listener address.", "缺少监听地址，无法判断风险。")
	case scope == portScopeLoopback:
		return m.t("External hosts normally cannot access this port unless traffic is forwarded locally.", "外部机器通常不能直接访问，除非本机做了转发。")
	case scope == portScopeWildcard:
		return m.t("If firewall or security group allows it, external hosts may access this port.", "如果防火墙或安全组放行，外部机器可能访问这个端口。")
	default:
		return m.t("Check whether this address is reachable from external networks.", "需要确认这个指定地址是否能被外部网络访问。")
	}
}

func portIPVersion(address string) string {
	host := portAddressHost(address)
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		return "IPv6"
	}
	return "IPv4"
}

func portAddressHost(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if strings.HasPrefix(address, "[") {
		if idx := strings.LastIndex(address, "]:"); idx >= 0 {
			return strings.Trim(address[1:idx], "[]")
		}
	}
	if idx := strings.LastIndex(address, ":"); idx >= 0 {
		return strings.TrimSpace(address[:idx])
	}
	return address
}

type portScope int

const (
	portScopeUnknown portScope = iota
	portScopeLoopback
	portScopeSpecific
	portScopeWildcard
)

func portAddressScope(address string) portScope {
	scope := portScopeUnknown
	for _, part := range resourceservice.SplitCSVValues(address) {
		host := portAddressHost(part)
		switch {
		case host == "":
			continue
		case isWildcardHost(host):
			return portScopeWildcard
		case isLoopbackHost(host):
			if scope == portScopeUnknown {
				scope = portScopeLoopback
			}
		default:
			if scope != portScopeSpecific {
				scope = portScopeSpecific
			}
		}
	}
	return scope
}

func isWildcardHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "*" || host == "0.0.0.0" || host == "::" || host == ":::" || host == ""
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
