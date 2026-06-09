package services

import (
	"net/http"
	"testing"
)

func TestApplyWebhookHeadersSendsVolaAndLegacyNames(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com/webhook", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	applyWebhookHeaders(req, "inbox.new", "abc123")

	for _, tc := range []struct {
		header string
		want   string
	}{
		{header: "Content-Type", want: "application/json"},
		{header: "X-Vola-Event", want: "inbox.new"},
		{header: "X-Vola-Signature", want: "sha256=abc123"},
		{header: "X-NeuDrive-Event", want: "inbox.new"},
		{header: "X-NeuDrive-Signature", want: "sha256=abc123"},
	} {
		if got := req.Header.Get(tc.header); got != tc.want {
			t.Fatalf("%s=%q want %q", tc.header, got, tc.want)
		}
	}
}
