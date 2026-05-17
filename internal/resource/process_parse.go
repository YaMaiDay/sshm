package resource

import "strings"

func ParseProcessExtraDetail(output string) (ProcessExtraDetail, string) {
	if strings.Contains(output, "__SSHM_PROCESS_INVALID__") {
		return ProcessExtraDetail{}, "PID无效"
	}
	if strings.Contains(output, "__SSHM_PROCESS_NOT_FOUND__") {
		return ProcessExtraDetail{}, "进程不存在"
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "__SSHM_PROCESS__\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		for len(parts) < 16 {
			parts = append(parts, "")
		}
		return ProcessExtraDetail{
			PID:          strings.TrimSpace(parts[1]),
			PPID:         strings.TrimSpace(parts[2]),
			User:         strings.TrimSpace(parts[3]),
			State:        strings.TrimSpace(parts[4]),
			CPU:          strings.TrimSpace(parts[5]),
			Memory:       strings.TrimSpace(parts[6]),
			RSS:          strings.TrimSpace(parts[7]),
			Elapsed:      strings.TrimSpace(parts[8]),
			Started:      strings.TrimSpace(parts[9]),
			Command:      strings.TrimSpace(parts[10]),
			CommandLine:  strings.TrimSpace(parts[11]),
			WorkingDir:   strings.TrimSpace(parts[12]),
			Executable:   strings.TrimSpace(parts[13]),
			ControlGroup: strings.TrimSpace(parts[14]),
			ServiceUnit:  strings.TrimSpace(parts[15]),
		}, ""
	}
	return ProcessExtraDetail{}, ""
}
