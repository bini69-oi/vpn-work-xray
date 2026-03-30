package logging

import "testing"

func TestRedactSensitive(t *testing.T) {
	msg := "token=abc123 password=secret privateKey=pk replace-me"
	out := redactSensitive(msg)
	if out == msg {
		t.Fatal("expected sensitive values to be redacted")
	}
	if containsAny(out, "abc123", "secret", "replace-me") {
		t.Fatalf("redaction failed: %s", out)
	}
}

func containsAny(s string, values ...string) bool {
	for _, v := range values {
		if v != "" && len(s) > 0 && stringContains(s, v) {
			return true
		}
	}
	return false
}

func stringContains(s, sub string) bool {
	return len(sub) <= len(s) && (s == sub || (len(s) > len(sub) && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

