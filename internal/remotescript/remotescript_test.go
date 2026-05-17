package remotescript

import (
	"strings"
	"testing"
)

func TestQuote(t *testing.T) {
	tests := map[string]string{
		"simple-name_1": "simple-name_1",
		"/var/log/app":  "/var/log/app",
		"needs space":   "'needs space'",
		"it's":          "'it'\\''s'",
		"":              "''",
	}
	for input, want := range tests {
		if got := Quote(input); got != want {
			t.Fatalf("Quote(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSingleQuoteAlwaysQuotes(t *testing.T) {
	if got := SingleQuote("plain"); got != "'plain'" {
		t.Fatalf("SingleQuote() = %q, want quoted plain value", got)
	}
	if got := SingleQuote("it's"); got != "'it'\\''s'" {
		t.Fatalf("SingleQuote() = %q, want escaped quote", got)
	}
}

func TestEnvName(t *testing.T) {
	for _, value := range []string{"GITHUB_TOKEN", "_TOKEN1", "token"} {
		if got := EnvName(value); got != value {
			t.Fatalf("EnvName(%q) = %q, want same", value, got)
		}
	}
	for _, value := range []string{"", "1TOKEN", "TOKEN-NAME", "TOKEN;touch"} {
		if got := EnvName(value); got != "" {
			t.Fatalf("EnvName(%q) = %q, want empty", value, got)
		}
	}
}

func TestSudoFallback(t *testing.T) {
	got := SudoFallback("docker ps", "sudo -n docker ps")
	if got == "" || !containsAll(got, "docker ps", "sudo -n docker ps", "__SSHM_PERMISSION_DENIED__") {
		t.Fatalf("SudoFallback() = %q", got)
	}
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}
