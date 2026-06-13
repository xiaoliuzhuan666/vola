package platforms_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

func TestCursorMcpTeamInjectionAndCleanup(t *testing.T) {
	// 1. Setup mock HOME directory
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Create fake .cursor directory and mock mcp.json with a user private server
	cursorDir := filepath.Join(tempDir, ".cursor")
	err := os.MkdirAll(cursorDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"user-private-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"private.js"},
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	configPath := filepath.Join(cursorDir, "mcp.json")
	err = os.WriteFile(configPath, initialData, 0644)
	if err != nil {
		t.Fatalf("WriteFile initial config failed: %v", err)
	}

	// 2. Setup mock daemon server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/teams" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"teams": [
						{"id": "team-foo", "slug": "team-foo"}
					]
				}
			}`))
			return
		}
		if r.URL.Path == "/api/teams/team-foo/mcps" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"mcps": [
						{
							"slug": "mcp-stdio",
							"name": "MCP Stdio",
							"transport": "stdio",
							"command": "python3",
							"args": ["server.py"],
							"env": {"DEBUG": "1"},
							"status": "published"
						},
						{
							"slug": "mcp-http",
							"name": "MCP Http",
							"transport": "http",
							"url": "http://127.0.0.1:9000",
							"headers": {"X-Custom": "test"},
							"status": "published"
						},
						{
							"slug": "mcp-draft",
							"name": "MCP Draft",
							"transport": "http",
							"url": "http://127.0.0.1:9001",
							"status": "draft"
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 3. Mock runtime config
	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"cursor-agent": {
					Token: "test-token-123",
				},
			},
		},
	}

	adapter, err := platforms.Resolve("cursor")
	if err != nil {
		t.Fatalf("Resolve cursor: %v", err)
	}

	// 4. Call Connect to inject team MCPs
	conn, err := adapter.Connect(context.Background(), cfg, "cursor", ts.URL, cfg.Local.Connections["cursor-agent"])
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	cfg.Local.Connections["cursor-agent"] = conn

	// 5. Verify config file content
	writtenData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	var parsedConfig map[string]interface{}
	if err := json.Unmarshal(writtenData, &parsedConfig); err != nil {
		t.Fatalf("Unmarshal written config failed: %v", err)
	}

	servers, ok := parsedConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers not found in written config: %s", string(writtenData))
	}

	// Verify vola-local
	volaLocal, ok := servers["vola-local"].(map[string]interface{})
	if !ok {
		t.Error("vola-local not found in mcpServers")
	} else {
		if !strings.HasSuffix(volaLocal["url"].(string), "/mcp") {
			t.Errorf("unexpected vola-local url: %v", volaLocal["url"])
		}
	}

	// Verify team-mcp-mcp-stdio
	mcpStdio, ok := servers["team-mcp-mcp-stdio"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-stdio not injected")
	} else {
		if mcpStdio["command"] != "python3" {
			t.Errorf("expected command python3, got %v", mcpStdio["command"])
		}
		args := mcpStdio["args"].([]interface{})
		if len(args) != 1 || args[0] != "server.py" {
			t.Errorf("unexpected args: %v", args)
		}
		env := mcpStdio["env"].(map[string]interface{})
		if env["DEBUG"] != "1" {
			t.Errorf("expected env DEBUG=1, got %v", env["DEBUG"])
		}
	}

	// Verify team-mcp-mcp-http
	mcpHttp, ok := servers["team-mcp-mcp-http"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-http not injected")
	} else {
		if mcpHttp["url"] != "http://127.0.0.1:9000" {
			t.Errorf("expected url http://127.0.0.1:9000, got %v", mcpHttp["url"])
		}
		headers := mcpHttp["headers"].(map[string]interface{})
		if headers["X-Custom"] != "test" {
			t.Errorf("expected header X-Custom=test, got %v", headers["X-Custom"])
		}
	}

	// Draft mcp-draft should not be injected
	if _, ok := servers["team-mcp-mcp-draft"]; ok {
		t.Error("team-mcp-mcp-draft (draft) should not be injected")
	}

	// Private server must be preserved
	if _, ok := servers["user-private-server"]; !ok {
		t.Error("user-private-server was lost")
	}

	// 6. Call Disconnect to cleanup
	err = adapter.Disconnect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// 7. Verify cleanup results
	cleanedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config after disconnect: %v", err)
	}

	var cleanedConfig map[string]interface{}
	_ = json.Unmarshal(cleanedData, &cleanedConfig)
	cleanedServers := cleanedConfig["mcpServers"].(map[string]interface{})

	if _, ok := cleanedServers["vola-local"]; ok {
		t.Error("vola-local was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-stdio"]; ok {
		t.Error("team-mcp-mcp-stdio was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-http"]; ok {
		t.Error("team-mcp-mcp-http was not cleaned up")
	}

	// User private server must still exist
	if _, ok := cleanedServers["user-private-server"]; !ok {
		t.Error("user-private-server was deleted during cleanup")
	}
}

func TestTraeMcpTeamInjectionAndCleanup(t *testing.T) {
	// 1. Setup mock HOME directory
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Create fake .trae directory and mock mcp.json with a user private server
	traeDir := filepath.Join(tempDir, ".trae")
	err := os.MkdirAll(traeDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"user-private-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"private.js"},
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	configPath := filepath.Join(traeDir, "mcp.json")
	err = os.WriteFile(configPath, initialData, 0644)
	if err != nil {
		t.Fatalf("WriteFile initial config failed: %v", err)
	}

	// 2. Setup mock daemon server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/teams" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"teams": [
						{"id": "team-foo", "slug": "team-foo"}
					]
				}
			}`))
			return
		}
		if r.URL.Path == "/api/teams/team-foo/mcps" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"mcps": [
						{
							"slug": "mcp-stdio",
							"name": "MCP Stdio",
							"transport": "stdio",
							"command": "python3",
							"args": ["server.py"],
							"env": {"DEBUG": "1"},
							"status": "published"
						},
						{
							"slug": "mcp-http",
							"name": "MCP Http",
							"transport": "http",
							"url": "http://127.0.0.1:9000",
							"headers": {"X-Custom": "test"},
							"status": "published"
						},
						{
							"slug": "mcp-draft",
							"name": "MCP Draft",
							"transport": "http",
							"url": "http://127.0.0.1:9001",
							"status": "draft"
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 3. Mock runtime config
	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"trae-agent": {
					Token: "test-token-123",
				},
			},
		},
	}

	adapter, err := platforms.Resolve("trae")
	if err != nil {
		t.Fatalf("Resolve trae: %v", err)
	}

	// 4. Call Connect to inject team MCPs
	conn, err := adapter.Connect(context.Background(), cfg, "trae", ts.URL, cfg.Local.Connections["trae-agent"])
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	cfg.Local.Connections["trae-agent"] = conn

	// 5. Verify config file content
	writtenData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	var parsedConfig map[string]interface{}
	if err := json.Unmarshal(writtenData, &parsedConfig); err != nil {
		t.Fatalf("Unmarshal written config failed: %v", err)
	}

	servers, ok := parsedConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers not found in written config: %s", string(writtenData))
	}

	// Verify vola-local
	volaLocal, ok := servers["vola-local"].(map[string]interface{})
	if !ok {
		t.Error("vola-local not found in mcpServers")
	} else {
		if !strings.HasSuffix(volaLocal["url"].(string), "/mcp") {
			t.Errorf("unexpected vola-local url: %v", volaLocal["url"])
		}
	}

	// Verify team-mcp-mcp-stdio
	mcpStdio, ok := servers["team-mcp-mcp-stdio"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-stdio not injected")
	} else {
		if mcpStdio["command"] != "python3" {
			t.Errorf("expected command python3, got %v", mcpStdio["command"])
		}
		args := mcpStdio["args"].([]interface{})
		if len(args) != 1 || args[0] != "server.py" {
			t.Errorf("unexpected args: %v", args)
		}
		env := mcpStdio["env"].(map[string]interface{})
		if env["DEBUG"] != "1" {
			t.Errorf("expected env DEBUG=1, got %v", env["DEBUG"])
		}
	}

	// Verify team-mcp-mcp-http
	mcpHttp, ok := servers["team-mcp-mcp-http"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-http not injected")
	} else {
		if mcpHttp["url"] != "http://127.0.0.1:9000" {
			t.Errorf("expected url http://127.0.0.1:9000, got %v", mcpHttp["url"])
		}
		headers := mcpHttp["headers"].(map[string]interface{})
		if headers["X-Custom"] != "test" {
			t.Errorf("expected header X-Custom=test, got %v", headers["X-Custom"])
		}
	}

	// Draft mcp-draft should not be injected
	if _, ok := servers["team-mcp-mcp-draft"]; ok {
		t.Error("team-mcp-mcp-draft (draft) should not be injected")
	}

	// Private server must be preserved
	if _, ok := servers["user-private-server"]; !ok {
		t.Error("user-private-server was lost")
	}

	// 6. Call Disconnect to cleanup
	err = adapter.Disconnect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// 7. Verify cleanup results
	cleanedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config after disconnect: %v", err)
	}

	var cleanedConfig map[string]interface{}
	_ = json.Unmarshal(cleanedData, &cleanedConfig)
	cleanedServers := cleanedConfig["mcpServers"].(map[string]interface{})

	if _, ok := cleanedServers["vola-local"]; ok {
		t.Error("vola-local was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-stdio"]; ok {
		t.Error("team-mcp-mcp-stdio was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-http"]; ok {
		t.Error("team-mcp-mcp-http was not cleaned up")
	}

	// User private server must still exist
	if _, ok := cleanedServers["user-private-server"]; !ok {
		t.Error("user-private-server was deleted during cleanup")
	}
}

func TestCodebuddyMcpTeamInjectionAndCleanup(t *testing.T) {
	// 1. Setup mock HOME directory
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Create fake .codebuddy directory and mock mcp.json with a user private server
	codebuddyDir := filepath.Join(tempDir, ".codebuddy")
	err := os.MkdirAll(codebuddyDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"user-private-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"private.js"},
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	configPath := filepath.Join(codebuddyDir, "mcp.json")
	err = os.WriteFile(configPath, initialData, 0644)
	if err != nil {
		t.Fatalf("WriteFile initial config failed: %v", err)
	}

	// 2. Setup mock daemon server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/teams" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"teams": [
						{"id": "team-foo", "slug": "team-foo"}
					]
				}
			}`))
			return
		}
		if r.URL.Path == "/api/teams/team-foo/mcps" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"mcps": [
						{
							"slug": "mcp-stdio",
							"name": "MCP Stdio",
							"transport": "stdio",
							"command": "python3",
							"args": ["server.py"],
							"env": {"DEBUG": "1"},
							"status": "published"
						},
						{
							"slug": "mcp-http",
							"name": "MCP Http",
							"transport": "http",
							"url": "http://127.0.0.1:9000",
							"headers": {"X-Custom": "test"},
							"status": "published"
						},
						{
							"slug": "mcp-draft",
							"name": "MCP Draft",
							"transport": "http",
							"url": "http://127.0.0.1:9001",
							"status": "draft"
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 3. Mock runtime config
	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"codebuddy-agent": {
					Token: "test-token-123",
				},
			},
		},
	}

	adapter, err := platforms.Resolve("codebuddy")
	if err != nil {
		t.Fatalf("Resolve codebuddy: %v", err)
	}

	// 4. Call Connect to inject team MCPs
	conn, err := adapter.Connect(context.Background(), cfg, "codebuddy", ts.URL, cfg.Local.Connections["codebuddy-agent"])
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	cfg.Local.Connections["codebuddy-agent"] = conn

	// 5. Verify config file content
	writtenData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	var parsedConfig map[string]interface{}
	if err := json.Unmarshal(writtenData, &parsedConfig); err != nil {
		t.Fatalf("Unmarshal written config failed: %v", err)
	}

	servers, ok := parsedConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers not found in written config: %s", string(writtenData))
	}

	// Verify vola-local
	volaLocal, ok := servers["vola-local"].(map[string]interface{})
	if !ok {
		t.Error("vola-local not found in mcpServers")
	} else {
		if !strings.HasSuffix(volaLocal["url"].(string), "/mcp") {
			t.Errorf("unexpected vola-local url: %v", volaLocal["url"])
		}
	}

	// Verify team-mcp-mcp-stdio
	mcpStdio, ok := servers["team-mcp-mcp-stdio"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-stdio not injected")
	} else {
		if mcpStdio["command"] != "python3" {
			t.Errorf("expected command python3, got %v", mcpStdio["command"])
		}
		args := mcpStdio["args"].([]interface{})
		if len(args) != 1 || args[0] != "server.py" {
			t.Errorf("unexpected args: %v", args)
		}
		env := mcpStdio["env"].(map[string]interface{})
		if env["DEBUG"] != "1" {
			t.Errorf("expected env DEBUG=1, got %v", env["DEBUG"])
		}
	}

	// Verify team-mcp-mcp-http
	mcpHttp, ok := servers["team-mcp-mcp-http"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-http not injected")
	} else {
		if mcpHttp["url"] != "http://127.0.0.1:9000" {
			t.Errorf("expected url http://127.0.0.1:9000, got %v", mcpHttp["url"])
		}
		headers := mcpHttp["headers"].(map[string]interface{})
		if headers["X-Custom"] != "test" {
			t.Errorf("expected header X-Custom=test, got %v", headers["X-Custom"])
		}
	}

	// Draft mcp-draft should not be injected
	if _, ok := servers["team-mcp-mcp-draft"]; ok {
		t.Error("team-mcp-mcp-draft (draft) should not be injected")
	}

	// Private server must be preserved
	if _, ok := servers["user-private-server"]; !ok {
		t.Error("user-private-server was lost")
	}

	// 6. Call Disconnect to cleanup
	err = adapter.Disconnect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// 7. Verify cleanup results
	cleanedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config after disconnect: %v", err)
	}

	var cleanedConfig map[string]interface{}
	_ = json.Unmarshal(cleanedData, &cleanedConfig)
	cleanedServers := cleanedConfig["mcpServers"].(map[string]interface{})

	if _, ok := cleanedServers["vola-local"]; ok {
		t.Error("vola-local was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-stdio"]; ok {
		t.Error("team-mcp-mcp-stdio was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-http"]; ok {
		t.Error("team-mcp-mcp-http was not cleaned up")
	}

	// User private server must still exist
	if _, ok := cleanedServers["user-private-server"]; !ok {
		t.Error("user-private-server was deleted during cleanup")
	}
}

func TestWorkbuddyMcpTeamInjectionAndCleanup(t *testing.T) {
	// 1. Setup mock HOME directory
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	// Create fake .workbuddy directory and mock mcp.json with a user private server
	workbuddyDir := filepath.Join(tempDir, ".workbuddy")
	err := os.MkdirAll(workbuddyDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initialConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"user-private-server": map[string]interface{}{
				"command": "node",
				"args":    []string{"private.js"},
			},
		},
	}
	initialData, _ := json.Marshal(initialConfig)
	configPath := filepath.Join(workbuddyDir, "mcp.json")
	err = os.WriteFile(configPath, initialData, 0644)
	if err != nil {
		t.Fatalf("WriteFile initial config failed: %v", err)
	}

	// 2. Setup mock daemon server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/teams" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"teams": [
						{"id": "team-foo", "slug": "team-foo"}
					]
				}
			}`))
			return
		}
		if r.URL.Path == "/api/teams/team-foo/mcps" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"data": {
					"mcps": [
						{
							"slug": "mcp-stdio",
							"name": "MCP Stdio",
							"transport": "stdio",
							"command": "python3",
							"args": ["server.py"],
							"env": {"DEBUG": "1"},
							"status": "published"
						},
						{
							"slug": "mcp-http",
							"name": "MCP Http",
							"transport": "http",
							"url": "http://127.0.0.1:9000",
							"headers": {"X-Custom": "test"},
							"status": "published"
						},
						{
							"slug": "mcp-draft",
							"name": "MCP Draft",
							"transport": "http",
							"url": "http://127.0.0.1:9001",
							"status": "draft"
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 3. Mock runtime config
	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"workbuddy-agent": {
					Token: "test-token-123",
				},
			},
		},
	}

	adapter, err := platforms.Resolve("workbuddy")
	if err != nil {
		t.Fatalf("Resolve workbuddy: %v", err)
	}

	// 4. Call Connect to inject team MCPs
	conn, err := adapter.Connect(context.Background(), cfg, "workbuddy", ts.URL, cfg.Local.Connections["workbuddy-agent"])
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	cfg.Local.Connections["workbuddy-agent"] = conn

	// 5. Verify config file content
	writtenData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config failed: %v", err)
	}

	var parsedConfig map[string]interface{}
	if err := json.Unmarshal(writtenData, &parsedConfig); err != nil {
		t.Fatalf("Unmarshal written config failed: %v", err)
	}

	servers, ok := parsedConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("mcpServers not found in written config: %s", string(writtenData))
	}

	// Verify vola-local
	volaLocal, ok := servers["vola-local"].(map[string]interface{})
	if !ok {
		t.Error("vola-local not found in mcpServers")
	} else {
		if !strings.HasSuffix(volaLocal["url"].(string), "/mcp") {
			t.Errorf("unexpected vola-local url: %v", volaLocal["url"])
		}
	}

	// Verify team-mcp-mcp-stdio
	mcpStdio, ok := servers["team-mcp-mcp-stdio"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-stdio not injected")
	} else {
		if mcpStdio["command"] != "python3" {
			t.Errorf("expected command python3, got %v", mcpStdio["command"])
		}
		args := mcpStdio["args"].([]interface{})
		if len(args) != 1 || args[0] != "server.py" {
			t.Errorf("unexpected args: %v", args)
		}
		env := mcpStdio["env"].(map[string]interface{})
		if env["DEBUG"] != "1" {
			t.Errorf("expected env DEBUG=1, got %v", env["DEBUG"])
		}
	}

	// Verify team-mcp-mcp-http
	mcpHttp, ok := servers["team-mcp-mcp-http"].(map[string]interface{})
	if !ok {
		t.Error("team-mcp-mcp-http not injected")
	} else {
		if mcpHttp["url"] != "http://127.0.0.1:9000" {
			t.Errorf("expected url http://127.0.0.1:9000, got %v", mcpHttp["url"])
		}
		headers := mcpHttp["headers"].(map[string]interface{})
		if headers["X-Custom"] != "test" {
			t.Errorf("expected header X-Custom=test, got %v", headers["X-Custom"])
		}
	}

	// Draft mcp-draft should not be injected
	if _, ok := servers["team-mcp-mcp-draft"]; ok {
		t.Error("team-mcp-mcp-draft (draft) should not be injected")
	}

	// Private server must be preserved
	if _, ok := servers["user-private-server"]; !ok {
		t.Error("user-private-server was lost")
	}

	// 6. Call Disconnect to cleanup
	err = adapter.Disconnect(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Disconnect failed: %v", err)
	}

	// 7. Verify cleanup results
	cleanedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile config after disconnect: %v", err)
	}

	var cleanedConfig map[string]interface{}
	_ = json.Unmarshal(cleanedData, &cleanedConfig)
	cleanedServers := cleanedConfig["mcpServers"].(map[string]interface{})

	if _, ok := cleanedServers["vola-local"]; ok {
		t.Error("vola-local was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-stdio"]; ok {
		t.Error("team-mcp-mcp-stdio was not cleaned up")
	}
	if _, ok := cleanedServers["team-mcp-mcp-http"]; ok {
		t.Error("team-mcp-mcp-http was not cleaned up")
	}

	// User private server must still exist
	if _, ok := cleanedServers["user-private-server"]; !ok {
		t.Error("user-private-server was deleted during cleanup")
	}
}
