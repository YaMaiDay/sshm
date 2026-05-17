package security

import (
	"strings"
	"testing"
)

func TestParseLoginRecordsFiltersNoise(t *testing.T) {
	output := strings.Join([]string{
		"reboot   system boot",
		"alice pts/0 10.0.0.1 Mon May 17 10:00 - 10:30 (00:30)",
		"wtmp begins Mon May 1 00:00:00 2026",
		"bob pts/1 10.0.0.2 Mon May 17 11:00 - 11:05 (00:05)",
	}, "\n")
	records := ParseLoginRecords(output, 1)
	if len(records) != 1 || !strings.HasPrefix(records[0], "alice pts/0") {
		t.Fatalf("records = %#v", records)
	}
}

func TestFailedLoginSummaryDetectsPermissionAndScan(t *testing.T) {
	if _, errText := FailedLoginSummary("__SSHM_LASTB_PERMISSION__"); errText == "" {
		t.Fatal("expected permission error")
	}
	output := strings.Join([]string{
		"root ssh:notty 1.1.1.1 Mon May 17 10:00 - 10:00 (00:00)",
		"admin ssh:notty 1.1.1.1 Mon May 17 10:01 - 10:01 (00:00)",
		"test ssh:notty 1.1.1.1 Mon May 17 10:02 - 10:02 (00:00)",
	}, "\n")
	rows, errText := FailedLoginSummary(output)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if got := strings.Join(rows, "\n"); !strings.Contains(got, "疑似扫描") || !strings.Contains(got, "1.1.1.1 尝试3个用户名") {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestParseSSHDSettings(t *testing.T) {
	settings := ParseSSHDSettings("PasswordAuthentication=yes\nPermitRootLogin=no\n")
	if settings["passwordauthentication"] != "yes" || settings["permitrootlogin"] != "no" {
		t.Fatalf("settings = %#v", settings)
	}
}
