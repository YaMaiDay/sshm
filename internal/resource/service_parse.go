package resource

import (
	"sort"
	"strconv"
	"strings"
)

func ParseServiceDetails(output string) ([]ServiceDetail, string) {
	if strings.Contains(output, "__SSHM_SYSTEMCTL_UNAVAILABLE__") {
		return nil, "systemctl不可用"
	}
	errText := ""
	items := []ServiceDetail{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "UNIT ") || strings.HasPrefix(trimmed, "LOAD ") {
			continue
		}
		if strings.HasPrefix(trimmed, "__SSHM_SYSTEMCTL_ERROR__") {
			errText = strings.TrimSpace(strings.ReplaceAll(output, "__SSHM_SYSTEMCTL_ERROR__", ""))
			continue
		}
		if strings.HasPrefix(line, "__SSHM_SERVICE__\t") {
			parts := strings.Split(line, "\t")
			for len(parts) < 31 {
				parts = append(parts, "")
			}
			if len(parts) < 9 || !strings.HasSuffix(parts[1], ".service") {
				continue
			}
			item := ServiceDetail{
				Unit:             strings.TrimSpace(parts[1]),
				Load:             strings.TrimSpace(parts[2]),
				Active:           strings.TrimSpace(parts[3]),
				Sub:              strings.TrimSpace(parts[4]),
				Description:      strings.TrimSpace(parts[5]),
				FragmentPath:     strings.TrimSpace(parts[6]),
				WorkingDirectory: strings.TrimSpace(parts[7]),
				ExecStart:        strings.TrimSpace(parts[8]),
			}
			if len(parts) >= 12 {
				item.MainPID = normalizePID(parts[9])
				item.ExecMainPID = normalizePID(parts[10])
				item.MemoryCurrent = parseSystemdMemoryCurrent(parts[11])
			}
			if len(parts) >= 13 {
				item.ActiveSince = strings.TrimSpace(parts[12])
			}
			if len(parts) >= 31 {
				item.InactiveSince = strings.TrimSpace(parts[13])
				item.StateChangedAt = strings.TrimSpace(parts[14])
				item.ExecStartedAt = strings.TrimSpace(parts[15])
				item.ExecExitedAt = strings.TrimSpace(parts[16])
				item.UnitFileState = strings.TrimSpace(parts[17])
				item.Result = strings.TrimSpace(parts[18])
				item.ExecMainStatus = strings.TrimSpace(parts[19])
				item.NRestarts = strings.TrimSpace(parts[20])
				item.TasksCurrent = strings.TrimSpace(parts[21])
				item.ControlGroup = strings.TrimSpace(parts[22])
				item.Slice = strings.TrimSpace(parts[23])
				item.User = strings.TrimSpace(parts[24])
				item.Group = strings.TrimSpace(parts[25])
				item.Restart = strings.TrimSpace(parts[26])
				item.RestartSec = strings.TrimSpace(parts[27])
				item.ExecStop = strings.TrimSpace(parts[28])
				item.ExecReload = strings.TrimSpace(parts[29])
				item.DropInPaths = strings.TrimSpace(parts[30])
			}
			items = append(items, item)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 4 || !strings.HasSuffix(fields[0], ".service") {
			continue
		}
		item := ServiceDetail{
			Unit:   fields[0],
			Load:   fields[1],
			Active: fields[2],
			Sub:    fields[3],
		}
		if len(fields) > 4 {
			item.Description = strings.Join(fields[4:], " ")
		}
		items = append(items, item)
	}
	if len(items) == 0 && errText != "" {
		return nil, errText
	}
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := ServiceDetailKindRank(items[i]), ServiceDetailKindRank(items[j])
		if ki != kj {
			return ki < kj
		}
		return strings.ToLower(items[i].Unit) < strings.ToLower(items[j].Unit)
	})
	return items, ""
}

func ParseServiceExtraDetail(output string) (ServiceDetail, string) {
	if strings.Contains(output, "__SSHM_SYSTEMCTL_UNAVAILABLE__") {
		return ServiceDetail{}, "systemctl不可用"
	}
	errText := ""
	props := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "__SSHM_SYSTEMCTL_ERROR__") {
			errText = strings.TrimSpace(strings.ReplaceAll(output, "__SSHM_SYSTEMCTL_ERROR__", ""))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if props["Id"] == "" {
		items, parsedErr := ParseServiceDetails(output)
		if len(items) > 0 {
			return items[0], ""
		}
		if parsedErr != "" {
			return ServiceDetail{}, parsedErr
		}
		return ServiceDetail{}, errText
	}
	item := ServiceDetail{
		Unit:             props["Id"],
		Load:             props["LoadState"],
		Active:           props["ActiveState"],
		Sub:              props["SubState"],
		Description:      props["Description"],
		FragmentPath:     props["FragmentPath"],
		WorkingDirectory: props["WorkingDirectory"],
		ExecStart:        props["ExecStart"],
		MainPID:          normalizePID(props["MainPID"]),
		ExecMainPID:      normalizePID(props["ExecMainPID"]),
		MemoryCurrent:    parseSystemdMemoryCurrent(props["MemoryCurrent"]),
		ActiveSince:      props["ActiveEnterTimestamp"],
		InactiveSince:    props["InactiveEnterTimestamp"],
		StateChangedAt:   props["StateChangeTimestamp"],
		ExecStartedAt:    props["ExecMainStartTimestamp"],
		ExecExitedAt:     props["ExecMainExitTimestamp"],
		UnitFileState:    props["UnitFileState"],
		Result:           props["Result"],
		ExecMainStatus:   props["ExecMainStatus"],
		NRestarts:        props["NRestarts"],
		TasksCurrent:     props["TasksCurrent"],
		ControlGroup:     props["ControlGroup"],
		Slice:            props["Slice"],
		User:             props["User"],
		Group:            props["Group"],
		Restart:          props["Restart"],
		RestartSec:       props["RestartUSec"],
		ExecStop:         props["ExecStop"],
		ExecReload:       props["ExecReload"],
		DropInPaths:      props["DropInPaths"],
	}
	return item, ""
}

func normalizePID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return ""
	}
	return value
}

func parseSystemdMemoryCurrent(value string) uint64 {
	value = strings.TrimSpace(value)
	if value == "" || value == "[not set]" || value == "-" {
		return 0
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n == ^uint64(0) {
		return 0
	}
	return n
}

func ServiceDetailKindRank(item ServiceDetail) int {
	switch serviceDetailKind(item) {
	case "failed":
		return 0
	case "running":
		return 1
	case "active":
		return 2
	case "stopped":
		return 3
	default:
		return 4
	}
}

func serviceDetailKind(item ServiceDetail) string {
	if item.Missing {
		return "missing"
	}
	active := strings.ToLower(strings.TrimSpace(item.Active))
	sub := strings.ToLower(strings.TrimSpace(item.Sub))
	load := strings.ToLower(strings.TrimSpace(item.Load))
	if active == "failed" || sub == "failed" {
		return "failed"
	}
	if active == "active" && sub == "running" {
		return "running"
	}
	if active == "active" {
		return "active"
	}
	if active == "inactive" || load == "not-found" {
		return "stopped"
	}
	return "other"
}
