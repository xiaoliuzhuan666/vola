package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/agi-bar/vola/internal/runtimecfg"
)

func TestLocalToolsStatusReportsSyncModesAndPreviewOnlyResources(t *testing.T) {
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
					Token:       "codex-token",
					ConnectedAt: "2026-06-16T00:00:00Z",
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

	ts, _, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/tools/status", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("tools status failed: status=%d body=%+v", status, env)
	}
	var resp localToolStatusResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Version != localToolStatusVersion {
		t.Fatalf("version = %q, want %q", resp.Version, localToolStatusVersion)
	}
	platforms := map[string]localToolStatusPlatform{}
	for _, item := range resp.Platforms {
		platforms[item.ID] = item
	}
	if !platforms["codex"].AutoSyncSupported || !platforms["codex"].Connected || platforms["codex"].SyncMode != "auto-sync" {
		t.Fatalf("unexpected codex status: %+v", platforms["codex"])
	}
	cursor := platforms["cursor-agent"]
	if cursor.AutoSyncSupported || !cursor.ExportSupported || cursor.SyncMode != "export-only" {
		t.Fatalf("unexpected cursor status: %+v", cursor)
	}
	gemini := platforms["gemini-cli"]
	if gemini.AutoSyncSupported || !gemini.ExportSupported || gemini.SyncMode != "manual" {
		t.Fatalf("unexpected gemini status: %+v", gemini)
	}
	if len(resp.ResourceRecommendations) == 0 {
		t.Fatalf("expected resource recommendations")
	}
	for _, item := range resp.ResourceRecommendations {
		if !item.PreviewOnly {
			t.Fatalf("resource recommendation must be preview-only: %+v", item)
		}
	}
}
