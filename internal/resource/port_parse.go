package resource

import (
	"sort"
	"strconv"
	"strings"
)

func ParsePortDetails(output string) ([]PortDetail, string) {
	if strings.Contains(output, "__SSHM_SS_UNAVAILABLE__") {
		return nil, "ss不可用"
	}
	if strings.Contains(output, "__SSHM_SS_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n ss）"
	}
	lines := strings.Split(output, "\n")
	cgroups := map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "__SSHM_PORT_CGROUP__\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			cgroups[strings.TrimSpace(parts[1])] = strings.TrimSpace(parts[2])
		}
	}
	grouped := map[string]*PortDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "__SSHM_PORT_CGROUP__\t") || strings.HasPrefix(line, "Netid") || strings.HasPrefix(line, "Proto") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		info := portLineInfo(fields)
		port := PortFromAddress(info.LocalAddress)
		if port == "" || port == "*" {
			continue
		}
		process, pid, fd := processFromSS(info.ProcessText)
		serviceUnit := cgroups[strings.TrimSpace(pid)]
		protocol := normalizePortProtocol(fields[0])
		key := strings.Join([]string{protocol, port, info.State, process, serviceUnit}, "/")
		if item, ok := grouped[key]; ok {
			item.Count++
			item.LocalAddress = appendUniqueCSV(item.LocalAddress, info.LocalAddress)
			item.ForeignAddress = appendUniqueCSV(item.ForeignAddress, info.ForeignAddress)
			item.PID = appendUniqueCSV(item.PID, pid)
			item.FD = appendUniqueCSV(item.FD, fd)
			item.ServiceUnit = appendUniqueCSV(item.ServiceUnit, serviceUnit)
			continue
		}
		grouped[key] = &PortDetail{
			Protocol:       protocol,
			Port:           port,
			LocalAddress:   info.LocalAddress,
			ForeignAddress: info.ForeignAddress,
			State:          info.State,
			Process:        process,
			PID:            pid,
			FD:             fd,
			ServiceUnit:    serviceUnit,
			Count:          1,
		}
	}
	out := make([]PortDetail, 0, len(grouped))
	for _, item := range grouped {
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		pi, _ := strconv.Atoi(out[i].Port)
		pj, _ := strconv.Atoi(out[j].Port)
		if pi == pj {
			return out[i].Protocol < out[j].Protocol
		}
		return pi < pj
	})
	return out, ""
}

func normalizePortProtocol(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "tcp6":
		return "tcp"
	case "udp6":
		return "udp"
	default:
		return value
	}
}

func appendUniqueCSV(current string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return current
	}
	values := SplitCSVValues(current)
	for _, existing := range values {
		if existing == value {
			return current
		}
	}
	values = append(values, value)
	return strings.Join(values, ", ")
}

func SplitCSVValues(value string) []string {
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

type portLineParseInfo struct {
	LocalAddress   string
	ForeignAddress string
	State          string
	ProcessText    string
}

func portLineInfo(fields []string) portLineParseInfo {
	processStart := len(fields)
	for i, field := range fields {
		if strings.HasPrefix(field, "users:") || strings.Contains(field, "users:(") {
			processStart = i
			break
		}
	}
	if processStart == len(fields) {
		for i := len(fields) - 1; i >= 0; i-- {
			if netstatProcessField(fields[i]) {
				processStart = i
				break
			}
		}
	}
	processText := ""
	if processStart < len(fields) {
		processText = strings.Join(fields[processStart:], " ")
	}
	limit := processStart
	if limit > len(fields) {
		limit = len(fields)
	}
	info := portLineParseInfo{ProcessText: processText}
	if len(fields) > 1 && !numericText(fields[1]) {
		info.State = fields[1]
	}
	localIndex := -1
	for i := 3; i < limit; i++ {
		port := PortFromAddress(fields[i])
		if port == "" || port == "*" {
			continue
		}
		info.LocalAddress = fields[i]
		localIndex = i
		break
	}
	if localIndex >= 0 && localIndex+1 < limit {
		next := fields[localIndex+1]
		if PortFromAddress(next) != "" {
			info.ForeignAddress = next
		}
	}
	if info.State == "" && localIndex >= 0 {
		for i := localIndex + 1; i < limit; i++ {
			field := fields[i]
			if PortFromAddress(field) != "" || numericText(field) || netstatProcessField(field) {
				continue
			}
			info.State = field
			break
		}
	}
	return info
}

func numericText(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func netstatProcessField(value string) bool {
	if value == "-" {
		return true
	}
	pid, _, ok := strings.Cut(value, "/")
	if !ok || pid == "" {
		return false
	}
	for _, r := range pid {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func PortFromAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if idx := strings.LastIndex(value, "]:"); idx >= 0 {
			return strings.TrimSpace(value[idx+2:])
		}
	}
	idx := strings.LastIndex(value, ":")
	if idx < 0 || idx == len(value)-1 {
		return ""
	}
	return strings.TrimSpace(value[idx+1:])
}

func processFromSS(value string) (string, string, string) {
	name := ""
	pid := ""
	fd := ""
	for _, field := range strings.Fields(value) {
		if field == "-" {
			return "", "", ""
		}
		left, right, ok := strings.Cut(field, "/")
		if ok && left != "" && right != "" {
			digits := true
			for _, r := range left {
				if r < '0' || r > '9' {
					digits = false
					break
				}
			}
			if digits {
				return right, left, ""
			}
		}
	}
	if idx := strings.Index(value, "\""); idx >= 0 {
		rest := value[idx+1:]
		if end := strings.Index(rest, "\""); end >= 0 {
			name = rest[:end]
		}
	}
	if idx := strings.Index(value, "pid="); idx >= 0 {
		rest := value[idx+4:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		pid = rest[:end]
	}
	if idx := strings.Index(value, "fd="); idx >= 0 {
		rest := value[idx+3:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		fd = rest[:end]
	}
	return name, pid, fd
}
