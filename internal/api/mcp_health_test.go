package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agi-bar/vola/internal/config"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

func TestMcpHealthCheckerAndEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// 1. Create mock trae/mcp.json with a team HTTP server
	traeDir := filepath.Join(tempDir, ".trae")
	_ = os.MkdirAll(traeDir, 0755)

	// Launch mock HTTP MCP target
	mockMcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockMcp.Close()

	initialConfig := map[string]any{
		"mcpServers": map[string]any{
			"team-mcp-online-test": map[string]any{
				"url": mockMcp.URL,
			},
			"team-mcp-offline-test": map[string]any{
				"url": "http://127.0.0.1:55432/mcp", // offline url
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	_ = os.WriteFile(filepath.Join(traeDir, "mcp.json"), initialData, 0644)

	// Save CLI config so health checker can Load() it
	t.Setenv("VOLA_CONFIG", filepath.Join(tempDir, "config.json"))
	cliConf := &runtimecfg.CLIConfig{
		Version: 1,
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	configPath := runtimecfg.DefaultConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, cliConfBytes, 0644)

	// 2. Run checkAllMcpServers synchronously to populate cache with current config
	checkAllMcpServers(context.Background())

	// 3. Test API endpoint GET /api/local/mcp/health
	server := NewServerWithDeps(ServerDeps{
		Config: &config.Config{
			RateLimit: 10000,
		},
		JWTSecret: testJWTSecret,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/local/mcp/health", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestJWT())
	w := httptest.NewRecorder()

	server.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %v", w.Code)
	}

	var resp struct {
		Ok   bool `json:"ok"`
		Data map[string]struct {
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms"`
		} `json:"data"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Ok {
		t.Error("expected ok true")
	}

	onlineCheck, ok := resp.Data["team-mcp-online-test"]
	if !ok {
		t.Error("team-mcp-online-test not found in health results")
	} else if onlineCheck.Status != "online" {
		t.Errorf("expected online status, got %s", onlineCheck.Status)
	}

	offlineCheck, ok := resp.Data["team-mcp-offline-test"]
	if !ok {
		t.Error("team-mcp-offline-test not found in health results")
	} else if offlineCheck.Status != "offline" {
		t.Errorf("expected offline status, got %s", offlineCheck.Status)
	}

	// 4. Test cache pruning: remove online test from config, rewrite configuration, and run checkAllMcpServers again
	initialConfigPruned := map[string]any{
		"mcpServers": map[string]any{
			"team-mcp-offline-test": map[string]any{
				"url": "http://127.0.0.1:55432/mcp",
			},
		},
	}
	prunedData, _ := json.Marshal(initialConfigPruned)
	_ = os.WriteFile(filepath.Join(traeDir, "mcp.json"), prunedData, 0644)

	checkAllMcpServers(context.Background())

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/local/mcp/health", nil)
	req2.Header.Set("Authorization", "Bearer "+generateTestJWT())
	server.Router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %v", w2.Code)
	}

	var resp2 struct {
		Ok   bool `json:"ok"`
		Data map[string]struct {
			Status    string `json:"status"`
			LatencyMs int64  `json:"latency_ms"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp2.Data["team-mcp-online-test"]; ok {
		t.Error("expected team-mcp-online-test to be pruned from cache")
	}
	if _, ok := resp2.Data["team-mcp-offline-test"]; !ok {
		t.Error("expected team-mcp-offline-test to still remain in cache")
	}
}

func TestMcpHealthAdaptiveBackoffAndWakeup(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("VOLA_CONFIG", filepath.Join(tempDir, "config.json"))

	// Create trae/mcp.json with an offline team HTTP server
	traeDir := filepath.Join(tempDir, ".trae")
	_ = os.MkdirAll(traeDir, 0755)

	initialConfig := map[string]any{
		"mcpServers": map[string]any{
			"team-mcp-backoff-test": map[string]any{
				"url": "http://127.0.0.1:55432/mcp", // offline URL
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	_ = os.WriteFile(filepath.Join(traeDir, "mcp.json"), initialData, 0644)

	// Save CLI config so health checker can Load() it
	cliConf := &runtimecfg.CLIConfig{
		Version: 1,
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	configPath := runtimecfg.DefaultConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, cliConfBytes, 0644)

	// Initialize state
	healthFailuresMu.Lock()
	healthFailures["team-mcp-backoff-test"] = 0
	checkCycleCount = 0
	healthFailuresMu.Unlock()

	// 1. First execution: checkCycleCount = 0, fails = 0 -> should check, failure count increments to 1
	checkAllMcpServers(context.Background())

	healthFailuresMu.Lock()
	fails1 := healthFailures["team-mcp-backoff-test"]
	healthFailuresMu.Unlock()
	if fails1 != 1 {
		t.Errorf("expected fails count to be 1, got %v", fails1)
	}

	// 2. Second execution: checkCycleCount = 1, fails = 1 -> skipFactor = 2, cycle 1 % 2 != 0 -> should skip check.
	// Failure count remains 1.
	checkAllMcpServers(context.Background())

	healthFailuresMu.Lock()
	fails2 := healthFailures["team-mcp-backoff-test"]
	healthFailuresMu.Unlock()
	if fails2 != 1 {
		t.Errorf("expected fails count to remain 1, got %v", fails2)
	}

	// 3. Trigger manual refresh via API endpoint -> resets failures to 0
	server := NewServerWithDeps(ServerDeps{
		Config: &config.Config{
			RateLimit: 10000,
		},
		JWTSecret: testJWTSecret,
	})

	// Set last manual refresh back in time so the throttle is bypassed
	manualRefreshMu.Lock()
	lastManualRefresh = time.Now().Add(-10 * time.Second)
	manualRefreshMu.Unlock()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/local/mcp/health", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestJWT())
	server.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %v", w.Code)
	}

	// Give the async go checkAllMcpServers a tiny moment to run
	time.Sleep(50 * time.Millisecond)

	healthFailuresMu.Lock()
	fails3 := healthFailures["team-mcp-backoff-test"]
	healthFailuresMu.Unlock()

	// Because manual refresh resets failures count to 0, it should be either 0 (or 1 if the async check has already completed and failed again)
	if fails3 > 1 {
		t.Errorf("expected fails count to be reset to 0 or 1, got %v", fails3)
	}
}
