package cli

import (
	"os"
	"strings"
	"testing"
)

func TestUsageLineUsesInvokedCommandName(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
	})

	for _, name := range []string{"neu", "vola", "vol", "neudrive", "xlzdrive"} {
		t.Run(name, func(t *testing.T) {
			os.Args = []string{name}
			want := "usage: " + name + " connect <platform>"
			if got := usageLine("connect <platform>"); got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestRootUsageRendersShortCommandWhenInvokedAsNeu(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
	})

	os.Args = []string{"neu"}
	stdout, stderr, code := captureRunForTest(t, func() int {
		return Run(nil)
	})
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "neu status") {
		t.Fatalf("expected short command in help output: %q", stdout)
	}
	if strings.Contains(stdout, "vola status") {
		t.Fatalf("did not expect canonical command in short help output: %q", stdout)
	}
}

func TestRootUsageRendersCompatibilityCommandNames(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
	})

	for _, name := range []string{"vol", "neudrive", "xlzdrive"} {
		t.Run(name, func(t *testing.T) {
			os.Args = []string{name}
			stdout, stderr, code := captureRunForTest(t, func() int {
				return Run(nil)
			})
			if code != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if !strings.Contains(stdout, name+" status") {
				t.Fatalf("expected %s command in help output: %q", name, stdout)
			}
			if !strings.Contains(stdout, "Recommended command name: neu.") {
				t.Fatalf("expected recommended command note in help output: %q", stdout)
			}
		})
	}
}
