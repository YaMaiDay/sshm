package monitor

import "testing"

func TestParseMetricsUsesAvailableMemory(t *testing.T) {
	metrics, err := parseMetrics("MEM=1000 700 400\n")
	if err != nil {
		t.Fatal(err)
	}

	if metrics.MemUsed != 600 {
		t.Fatalf("MemUsed = %d, want 600", metrics.MemUsed)
	}
	if got := metrics.MemPercent(); got != 60 {
		t.Fatalf("MemPercent = %v, want 60", got)
	}
}

func TestDiskPercentUsesDfUsableSpace(t *testing.T) {
	metrics, err := parseMetrics("DISK=1000 900 50\n")
	if err != nil {
		t.Fatal(err)
	}

	if got := metrics.DiskPercent(); got != 900.0/950.0*100 {
		t.Fatalf("DiskPercent = %v, want %v", got, 900.0/950.0*100)
	}
}

func TestDiskPercentUsesDfUsableSpaceWhenFull(t *testing.T) {
	metrics, err := parseMetrics("DISK=1000 950 0\n")
	if err != nil {
		t.Fatal(err)
	}

	if got := metrics.DiskPercent(); got != 100 {
		t.Fatalf("DiskPercent = %v, want 100", got)
	}
}
