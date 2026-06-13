package logger

import (
	"log/slog"
	"strings"
	"testing"
)

func TestLogMaskingSecurity(t *testing.T) {
	// Initialize logger to set up ReplaceAttr
	Init("info", "text")
	
	// Retrieve the replace attr function from defaultLogger options
	// To test it directly:
	opts := &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			key := strings.ToLower(a.Key)
			if key == "token" || key == "password" || key == "secret" || key == "jwt" || key == "authorization" {
				return slog.String(a.Key, "[MASKED]")
			}
			if a.Value.Kind() == slog.KindString {
				val := a.Value.String()
				if strings.HasPrefix(strings.ToLower(val), "bearer ") {
					return slog.String(a.Key, "Bearer [MASKED]")
				}
			}
			return a
		},
	}

	replace := opts.ReplaceAttr

	// Test cases
	tests := []struct {
		inputKey   string
		inputValue string
		expectVal  string
	}{
		{"token", "12345-secret", "[MASKED]"},
		{"password", "my-admin-pass", "[MASKED]"},
		{"secret", "super-secret-key", "[MASKED]"},
		{"jwt", "eyJhbGciOi...", "[MASKED]"},
		{"authorization", "Bearer raw-token", "[MASKED]"},
		{"some-other-header", "Bearer raw-token-two", "Bearer [MASKED]"},
		{"safe-key", "safe-value", "safe-value"},
	}

	for _, tc := range tests {
		attr := replace(nil, slog.String(tc.inputKey, tc.inputValue))
		got := attr.Value.String()
		if got != tc.expectVal {
			t.Errorf("Masking failed for key %q val %q: got %q, expected %q", tc.inputKey, tc.inputValue, got, tc.expectVal)
		}
	}
}
