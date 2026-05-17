package tui

import (
	"strconv"
	"strings"
	"time"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func (m Model) portStatusText(item resourceservice.PortDetail) string {
	if item.Missing {
		return m.t("Not found", "未发现")
	}
	if strings.TrimSpace(item.State) != "" {
		return item.State
	}
	return m.t("Listening", "监听中")
}

func (m Model) portStatusStyled(item resourceservice.PortDetail) string {
	if item.Missing {
		return mutedStyle.Render(m.portStatusText(item))
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	switch state {
	case "LISTEN", "UNCONN":
		return greenStyle.Render(m.portStatusText(item))
	case "":
		return greenStyle.Render(m.portStatusText(item))
	default:
		return yellowStyle.Render(m.portStatusText(item))
	}
}

func (m Model) portStatusLine(item resourceservice.PortDetail) string {
	raw := strings.ToUpper(strings.TrimSpace(item.State))
	if raw == "" {
		raw = m.portStatusText(item)
	}
	return m.portStatusStyledLabel(item, m.portStatusLabel(item)) + "  " + m.portStatusStyledLabel(item, raw)
}

func (m Model) portStatusLabel(item resourceservice.PortDetail) string {
	if item.Missing {
		return m.t("Not found", "未发现")
	}
	switch strings.ToUpper(strings.TrimSpace(item.State)) {
	case "LISTEN", "UNCONN", "":
		return m.t("Listening", "监听")
	case "CLOSE", "CLOSED":
		return m.t("Closed", "关闭")
	default:
		return m.t("State", "状态")
	}
}

func (m Model) portStatusStyledLabel(item resourceservice.PortDetail, text string) string {
	if item.Missing {
		return mutedStyle.Render(text)
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	switch state {
	case "LISTEN", "UNCONN", "":
		return greenStyle.Render(text)
	case "CLOSE", "CLOSED":
		return mutedStyle.Render(text)
	default:
		return yellowStyle.Render(text)
	}
}

func (m Model) processStatusLine(item resourceservice.PortDetail) string {
	raw := strings.ToUpper(strings.TrimSpace(item.State))
	if raw == "" {
		raw = "-"
	}
	return greenStyle.Render(m.t("Running", "运行")) + "  " + greenStyle.Render(raw)
}

func processMemoryText(item resourceservice.ProcessExtraDetail) string {
	mem := percentSuffix(item.Memory)
	if strings.TrimSpace(item.RSS) == "" {
		return emptyDash(mem)
	}
	rss := processRSSText(item.RSS)
	if strings.TrimSpace(mem) == "" {
		return rss
	}
	return rss + "  " + mem
}

func (m Model) processExtraForCard(item resourceservice.PortDetail) (resourceservice.ProcessExtraDetail, bool) {
	if strings.TrimSpace(item.PID) == "" || m.resourceProcessExtraLoading || strings.TrimSpace(m.resourceProcessExtraErr) != "" {
		return resourceservice.ProcessExtraDetail{}, false
	}
	if m.resourceProcessExtraPID != item.PID || m.resourceProcessExtra.PID != item.PID {
		return resourceservice.ProcessExtraDetail{}, false
	}
	return m.resourceProcessExtra, true
}

func (m Model) processCardMeta(extra resourceservice.ProcessExtraDetail, ok bool) string {
	if !ok {
		return ""
	}
	d, parsed := parseProcessElapsed(extra.Elapsed)
	if !parsed {
		return ""
	}
	return m.dashboardDurationShort(d)
}

func (m Model) processCardStatusLine(item resourceservice.PortDetail, extra resourceservice.ProcessExtraDetail, ok bool) string {
	state := ""
	if ok && strings.TrimSpace(extra.State) != "" {
		state = strings.TrimSpace(extra.State)
	}
	if state == "" && strings.TrimSpace(item.State) != "" && !strings.EqualFold(strings.TrimSpace(item.State), "LISTEN") && !strings.EqualFold(strings.TrimSpace(item.State), "UNCONN") {
		state = strings.TrimSpace(item.State)
	}
	status := m.t("Running", "运行")
	style := greenStyle.Render
	if processStateProblem(state) {
		style = redStyle.Render
	} else if processStateWarn(state) {
		style = yellowStyle.Render
	}
	if state == "" {
		return style(status)
	}
	return style(status) + "  " + style(state)
}

func (m Model) processCardResourceLine(item resourceservice.PortDetail, extra resourceservice.ProcessExtraDetail, ok bool) string {
	parts := []string{}
	if strings.TrimSpace(item.PID) != "" {
		parts = append(parts, "PID  "+cardMutedStyle.Render(item.PID))
	}
	memory := ""
	if ok {
		memory = processMemoryText(extra)
	}
	if strings.TrimSpace(memory) != "" && memory != "-" {
		parts = append(parts, m.t("Memory", "内存")+"  "+cardMutedStyle.Render(memory))
	}
	if len(parts) == 0 {
		return m.t("Resource", "资源") + "  " + cardMutedStyle.Render("-")
	}
	return strings.Join(parts, "  ")
}

func (m Model) processCardExecutable(item resourceservice.PortDetail, extra resourceservice.ProcessExtraDetail, ok bool) string {
	if ok && strings.TrimSpace(extra.Executable) != "" {
		return strings.TrimSpace(extra.Executable)
	}
	return strings.TrimSpace(item.Process)
}

func (m Model) processCardCommandLine(item resourceservice.PortDetail, extra resourceservice.ProcessExtraDetail, ok bool) string {
	if ok && strings.TrimSpace(extra.CommandLine) != "" {
		return strings.TrimSpace(extra.CommandLine)
	}
	if ok && strings.TrimSpace(extra.Command) != "" {
		return strings.TrimSpace(extra.Command)
	}
	if strings.TrimSpace(item.Process) != "" {
		return strings.TrimSpace(item.Process)
	}
	return strings.TrimSpace(item.Protocol + "/" + item.Port)
}

func (m Model) processCardDot(extra resourceservice.ProcessExtraDetail, ok bool) string {
	state := ""
	if ok {
		state = extra.State
	}
	switch {
	case processStateProblem(state):
		return redStyle.Render("●")
	case processStateWarn(state):
		return yellowStyle.Render("●")
	default:
		return greenStyle.Render("●")
	}
}

func processStateProblem(state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	return strings.Contains(state, "Z") || strings.Contains(state, "X")
}

func processStateWarn(state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	return strings.Contains(state, "D") || strings.Contains(state, "T")
}

func parseProcessElapsed(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	days := 0
	if left, right, ok := strings.Cut(value, "-"); ok {
		n, err := strconv.Atoi(left)
		if err != nil {
			return 0, false
		}
		days = n
		value = right
	}
	parts := strings.Split(value, ":")
	total := time.Duration(days) * 24 * time.Hour
	switch len(parts) {
	case 2:
		minutes, err1 := strconv.Atoi(parts[0])
		seconds, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return 0, false
		}
		total += time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	case 3:
		hours, err1 := strconv.Atoi(parts[0])
		minutes, err2 := strconv.Atoi(parts[1])
		seconds, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, false
		}
		total += time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	default:
		return 0, false
	}
	return total, true
}

func processRSSText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	kb, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return value
	}
	return bytesHuman(kb * 1024)
}

func percentSuffix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, "%") {
		return value
	}
	return value + "%"
}
