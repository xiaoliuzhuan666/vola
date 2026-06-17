package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agi-bar/vola/internal/runtimecfg"
)

func TestLocalPlatformConnectionRefreshReusesExistingConnection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv(runtimecfg.ConfigEnv, configPath)

	cfg := &runtimecfg.CLIConfig{
		Version: 3,
		Local: runtimecfg.LocalConfig{
			PublicBaseURL: "http://127.0.0.1:42690",
			Connections: map[string]runtimecfg.LocalConnection{
				"codex": {
					Token:           "existing-token",
					ConnectedAt:     "2026-06-16T00:00:00Z",
					LastPlatformURL: "http://old.local/mcp",
				},
			},
		},
	}
	if err := runtimecfg.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if err := runtimecfg.SaveState(runtimecfg.DefaultStatePath(), &runtimecfg.RuntimeState{
		APIBase: "http://127.0.0.1:42700",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	originalRefresh := refreshLocalPlatformConnection
	t.Cleanup(func() { refreshLocalPlatformConnection = originalRefresh })
	refreshLocalPlatformConnection = func(ctx context.Context, cfg *runtimecfg.CLIConfig, platform, executable, daemonURL string) (runtimecfg.LocalConnection, error) {
		if platform != "codex" {
			t.Fatalf("platform = %q, want codex", platform)
		}
		if !strings.HasPrefix(daemonURL, "http://127.0.0.1:42700") {
			t.Fatalf("daemonURL = %q, want state APIBase", daemonURL)
		}
		if cfg.Local.Connections["codex"].Token != "existing-token" {
			t.Fatalf("token was not reused: %+v", cfg.Local.Connections["codex"])
		}
		updated := cfg.Local.Connections["codex"]
		updated.Transport = "stdio"
		updated.ConfigPath = filepath.Join(tempDir, ".codex", "config.toml")
		cfg.Local.Connections["codex"] = updated
		return updated, nil
	}

	ts, _, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/platform/connection/refresh", adminToken, []byte(`{"platform":"codex"}`))
	if status != http.StatusOK || !env.OK {
		t.Fatalf("refresh failed: status=%d env=%+v", status, env)
	}
	var resp localPlatformConnectionResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if !resp.Refreshed || resp.Platform != "codex" || resp.Name != "Codex CLI" || resp.Connection.Transport != "stdio" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	_, saved, err := runtimecfg.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if saved.Local.Connections["codex"].ConfigPath == "" {
		t.Fatalf("expected saved codex connection, got %+v", saved.Local.Connections["codex"])
	}
}

func TestLocalPlatformConnectionRefreshRejectsManualOnlyPlatforms(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	configPath := filepath.Join(tempDir, "config.json")
	t.Setenv(runtimecfg.ConfigEnv, configPath)
	if err := runtimecfg.SaveConfig(configPath, &runtimecfg.CLIConfig{Version: 3}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	ts, _, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/platform/connection/refresh", adminToken, []byte(`{"platform":"gemini-cli"}`))
	if status != http.StatusUnprocessableEntity || env.OK {
		t.Fatalf("expected validation error, status=%d env=%+v", status, env)
	}
}
