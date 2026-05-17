package security

import (
	"fmt"
	"sort"
	"strings"
)

func ParseLoginRecords(output string, limit int) []string {
	var records []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "wtmp begins") ||
			strings.HasPrefix(lower, "btmp begins") ||
			strings.HasPrefix(lower, "reboot ") ||
			strings.HasPrefix(lower, "shutdown ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		records = append(records, strings.Join(fields, " "))
		if limit > 0 && len(records) >= limit {
			break
		}
	}
	return records
}

func ParseSSHDSettings(output string) map[string]string {
	settings := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		settings[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.TrimSpace(value))
	}
	return settings
}

func FailedLoginSummary(output string) ([]string, string) {
	if strings.Contains(output, "__SSHM_LASTB_UNAVAILABLE__") {
		return nil, "lastb不可用"
	}
	if strings.Contains(output, "__SSHM_LASTB_PERMISSION__") {
		return nil, "需要root权限（可配置sudo -n lastb）"
	}
	return LoginSummaryRows(ParseLoginRecords(output, 100)), ""
}

func LoginSummaryRows(records []string) []string {
	if len(records) == 0 {
		return nil
	}
	ipCounts := map[string]int{}
	userCounts := map[string]int{}
	ipUsers := map[string]map[string]bool{}
	for _, record := range records {
		fields := strings.Fields(record)
		if len(fields) > 0 {
			userCounts[fields[0]]++
		}
		if len(fields) > 2 {
			ipCounts[fields[2]]++
			if ipUsers[fields[2]] == nil {
				ipUsers[fields[2]] = map[string]bool{}
			}
			if len(fields) > 0 {
				ipUsers[fields[2]][fields[0]] = true
			}
		}
	}
	rows := []string{
		fmt.Sprintf("统计\t最近%d条", len(records)),
		fmt.Sprintf("来源IP\t%s", topCountsText(ipCounts, 3)),
		fmt.Sprintf("用户名\t%s", topCountsText(userCounts, 5)),
		fmt.Sprintf("最近\t%s", records[0]),
	}
	if scanText := suspiciousScanText(ipUsers); scanText != "" {
		rows = append(rows, fmt.Sprintf("疑似扫描\t%s", scanText))
	}
	return rows
}

func suspiciousScanText(ipUsers map[string]map[string]bool) string {
	type item struct {
		IP    string
		Users int
	}
	items := []item{}
	for ip, users := range ipUsers {
		if len(users) >= 3 {
			items = append(items, item{IP: ip, Users: len(users)})
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Users == items[j].Users {
			return items[i].IP < items[j].IP
		}
		return items[i].Users > items[j].Users
	})
	limit := min(3, len(items))
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s 尝试%d个用户名", items[i].IP, items[i].Users))
	}
	return strings.Join(parts, "、")
}

func topCountsText(counts map[string]int, limit int) string {
	if len(counts) == 0 {
		return "-"
	}
	type item struct {
		Value string
		Count int
	}
	items := make([]item, 0, len(counts))
	for value, count := range counts {
		if strings.TrimSpace(value) == "" {
			continue
		}
		items = append(items, item{Value: value, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Value < items[j].Value
		}
		return items[i].Count > items[j].Count
	})
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s %d次", items[i].Value, items[i].Count))
	}
	return strings.Join(parts, "、")
}
