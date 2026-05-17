package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
)

func (m Model) cardHeaderMeta(h host.Host, metrics monitor.Metrics) string {
	if strings.TrimSpace(h.ExpireAt) != "" {
		return m.expireCardText(h.ExpireAt)
	}
	return cardMutedStyle.Render(m.cardUptimeShort(metrics.Uptime))
}

func (m Model) expireCardText(value string) string {
	if m.isChineseUI() {
		return expireCardText(value)
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("Bad expiry")
	}
	switch {
	case days < 0:
		return redStyle.Render("Expired")
	case days == 0:
		return redStyle.Render("Expires today")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("Exp %dd", days))
	default:
		return cardMutedStyle.Render(fmt.Sprintf("Exp %dd", days))
	}
}

func expireCardText(value string) string {
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("到期格式错")
	}
	switch {
	case days < 0:
		return redStyle.Render("已过期")
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("到期%d天", days))
	default:
		return cardMutedStyle.Render(fmt.Sprintf("到期%d天", days))
	}
}

func expireDetailText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("格式错误")
	}
	switch {
	case days < 0:
		return redStyle.Render(fmt.Sprintf("已过期%d天", -days))
	case days == 0:
		return redStyle.Render("今天到期")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("剩余%d天", days))
	default:
		return fmt.Sprintf("剩余%d天", days)
	}
}

func (m Model) expireDetailText(value string) string {
	if m.isChineseUI() {
		return expireDetailText(value)
	}
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	days, ok := expireDays(value)
	if !ok {
		return redStyle.Render("Invalid")
	}
	switch {
	case days < 0:
		return redStyle.Render(fmt.Sprintf("Expired %dd", -days))
	case days == 0:
		return redStyle.Render("Expires today")
	case days <= 7:
		return yellowStyle.Render(fmt.Sprintf("%dd left", days))
	default:
		return fmt.Sprintf("%dd left", days)
	}
}

func expireDays(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	expire, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return 0, false
	}
	now := time.Now().In(time.Local)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return int(expire.Sub(today).Hours() / 24), true
}

func uptimeCN(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(value)
	value = normalizeWeeksToDays(value)
	replacer := strings.NewReplacer(
		" days", "天",
		" day", "天",
		" hours", "小时",
		" hour", "小时",
		" minutes", "分钟",
		" minute", "分钟",
		", ", "",
		" ago", "前",
	)
	value = replacer.Replace(value)
	return value
}

func cardUptimeShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	if days > 0 {
		return fmt.Sprintf("%d天", days)
	}
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	if hours > 0 {
		return fmt.Sprintf("%d时", hours)
	}
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%d分", minutes)
}

func (m Model) cardUptimeShort(value string) string {
	if m.isChineseUI() {
		return cardUptimeShort(value)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if minutes < 1 {
		minutes = 1
	}
	return fmt.Sprintf("%dm", minutes)
}

func lastLoginDetail(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	relative := relativeTime(value)
	if relative != "刚刚" {
		relative += "前"
	}
	return value.Format("2006-01-02 15:04") + "（" + relative + "）"
}

func (m Model) lastLoginDetail(value time.Time) string {
	if m.isChineseUI() {
		return lastLoginDetail(value)
	}
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04") + " (" + m.lastLoginCard(value) + ")"
}

func (m Model) uptimeText(value string) string {
	if m.isChineseUI() {
		return uptimeCN(value)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.TrimPrefix(value, "up ")
	value = strings.TrimSpace(normalizeWeeksToDays(value))
	parts := []string{}
	days := firstUptimeNumber(value, `(\d+)\s+days?`)
	hours := firstUptimeNumber(value, `(\d+)\s+hours?`)
	minutes := firstUptimeNumber(value, `(\d+)\s+minutes?`)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if len(parts) == 0 {
		return m.cardUptimeShort(value)
	}
	return strings.Join(parts, " ")
}

func lastLoginCard(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	relative := relativeTime(value)
	if relative == "刚刚" {
		return relative
	}
	return relative + "前"
}

func (m Model) lastLoginCard(value time.Time) string {
	if m.isChineseUI() {
		return lastLoginCard(value)
	}
	if value.IsZero() {
		return ""
	}
	relative := m.relativeTime(value)
	if relative == "now" {
		return relative
	}
	return relative + " ago"
}

func (m Model) relativeTime(value time.Time) string {
	if m.isChineseUI() {
		return relativeTime(value)
	}
	if value.IsZero() {
		return "-"
	}
	d := time.Since(value)
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "now"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%dh", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%dd", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo", months)
	}
	return fmt.Sprintf("%dy", days/365)
}

func relativeTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	d := time.Since(value)
	if d < 0 {
		d = 0
	}
	minutes := int(d.Minutes())
	if minutes < 1 {
		return "刚刚"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d分", minutes)
	}
	hours := int(d.Hours())
	if hours < 24 {
		return fmt.Sprintf("%d时", hours)
	}
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%d天", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d月", months)
	}
	return fmt.Sprintf("%d年", days/365)
}

func firstUptimeNumber(value string, pattern string) int {
	re := regexp.MustCompile(pattern)
	parts := re.FindStringSubmatch(value)
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[1])
	return n
}

func normalizeWeeksToDays(value string) string {
	re := regexp.MustCompile(`(\d+)\s+weeks?(?:,\s*(\d+)\s+days?)?`)
	return re.ReplaceAllStringFunc(value, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) == 0 {
			return match
		}
		weeks, _ := strconv.Atoi(parts[1])
		days := 0
		if len(parts) > 2 && parts[2] != "" {
			days, _ = strconv.Atoi(parts[2])
		}
		return fmt.Sprintf("%d days", weeks*7+days)
	})
}
