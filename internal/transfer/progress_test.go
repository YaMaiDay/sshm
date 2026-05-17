package transfer

import "testing"

func TestProgressTextNormalizesRsyncProgress(t *testing.T) {
	got := ProgressText("     1,024  50%   10.00kB/s    0:00:01 (xfer#2, to-check=1/3)")
	want := "1,024 50% 10.00kB/s 0:00:01 (xfer#2, to-check=1/3)"
	if got != want {
		t.Fatalf("ProgressText = %q, want %q", got, want)
	}
}

func TestProgressValues(t *testing.T) {
	bytes, percent, seq, ok := ProgressValues("1,024 50% 10.00kB/s 0:00:01 (xfer#2, to-check=1/3)")
	if !ok || bytes != 1024 || percent != 50 || seq != 2 {
		t.Fatalf("ProgressValues = %d %d %d %v, want 1024 50 2 true", bytes, percent, seq, ok)
	}
}

func TestLastProgressLine(t *testing.T) {
	got := LastProgressLine("noise\n1,024 50% xfer#1\n2,048 100% xfer#2\n")
	if got != "2,048 100% xfer#2" {
		t.Fatalf("LastProgressLine = %q", got)
	}
}
