package transfer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var rsyncPercentPattern = regexp.MustCompile(`\b([0-9]{1,3})%`)
var rsyncXferPattern = regexp.MustCompile(`xfer#([0-9]+)`)

func ProgressText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" || PercentText(value) == "" {
		return ""
	}
	return value
}

func PercentText(value string) string {
	match := rsyncPercentPattern.FindStringSubmatch(value)
	if len(match) < 2 {
		return ""
	}
	percent, err := strconv.Atoi(match[1])
	if err != nil {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return fmt.Sprintf("%d%%", percent)
}

func ProgressValues(value string) (int64, int, int, bool) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) < 2 {
		return 0, 0, 0, false
	}
	bytesText := strings.ReplaceAll(fields[0], ",", "")
	bytes, err := strconv.ParseInt(bytesText, 10, 64)
	if err != nil || bytes < 0 {
		return 0, 0, 0, false
	}
	percentText := strings.TrimSuffix(fields[1], "%")
	percent, err := strconv.Atoi(percentText)
	if err != nil {
		return 0, 0, 0, false
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	seq := 0
	if match := rsyncXferPattern.FindStringSubmatch(value); len(match) == 2 {
		seq, _ = strconv.Atoi(match[1])
	}
	return bytes, percent, seq, true
}

func LastProgressLine(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if progress := ProgressText(line); progress != "" {
			return progress
		}
	}
	return ""
}
