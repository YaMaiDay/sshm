package tui

import (
	"fmt"
	"strings"
	"time"
)

func shortSystemdTimestampAge(value string, chinese bool) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "n/a" {
		return ""
	}
	layouts := []string{
		"Mon 2006-01-02 15:04:05 MST",
		"Mon 2006-01-02 15:04:05 -07",
		"Mon 2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04:05 -0700",
	}
	var parsed time.Time
	ok := false
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			parsed = t
			ok = true
			break
		}
	}
	if !ok {
		fields := strings.Fields(value)
		if len(fields) >= 3 {
			trimmed := strings.Join(fields[:3], " ")
			if t, err := time.ParseInLocation("Mon 2006-01-02 15:04:05", trimmed, time.Local); err == nil {
				parsed = t
				ok = true
			}
		}
	}
	if !ok {
		return ""
	}
	d := time.Since(parsed)
	if d < 0 {
		return ""
	}
	return shortDurationAge(d, chinese)
}

func shortDurationAge(d time.Duration, chinese bool) string {
	switch {
	case d < time.Minute:
		n := int(d.Seconds())
		if n < 1 {
			n = 1
		}
		if chinese {
			return fmt.Sprintf("%d秒", n)
		}
		return fmt.Sprintf("%ds", n)
	case d < time.Hour:
		n := int(d.Minutes())
		if chinese {
			return fmt.Sprintf("%d分", n)
		}
		return fmt.Sprintf("%dm", n)
	case d < 24*time.Hour:
		n := int(d.Hours())
		if chinese {
			return fmt.Sprintf("%d时", n)
		}
		return fmt.Sprintf("%dh", n)
	case d < 7*24*time.Hour:
		n := int(d.Hours() / 24)
		if chinese {
			return fmt.Sprintf("%d天", n)
		}
		return fmt.Sprintf("%dd", n)
	case d < 30*24*time.Hour:
		n := int(d.Hours() / 24 / 7)
		if chinese {
			return fmt.Sprintf("%d周", n)
		}
		return fmt.Sprintf("%dw", n)
	case d < 365*24*time.Hour:
		n := int(d.Hours() / 24 / 30)
		if chinese {
			return fmt.Sprintf("%d月", n)
		}
		return fmt.Sprintf("%dmo", n)
	default:
		n := int(d.Hours() / 24 / 365)
		if chinese {
			return fmt.Sprintf("%d年", n)
		}
		return fmt.Sprintf("%dy", n)
	}
}
