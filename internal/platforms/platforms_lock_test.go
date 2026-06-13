package platforms

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agi-bar/vola/internal/runtimecfg"
)

func TestMcpConfigSafeAtomicWriteAndLock(t *testing.T) {
	tempDir := t.TempDir()

	t.Setenv("HOME", tempDir)

	// Create fake .trae directory and empty mcp.json
	traeDir := filepath.Join(tempDir, ".trae")
	err := os.MkdirAll(traeDir, 0755)
	if err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	initial := map[string]any{"mcpServers": map[string]any{}}
	data, _ := json.Marshal(initial)
	traeConfigPath := filepath.Join(traeDir, "mcp.json")
	_ = os.WriteFile(traeConfigPath, data, 0644)

	adapter, err := Resolve("trae")
	if err != nil {
		t.Fatalf("Resolve trae failed: %v", err)
	}

	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"trae-agent": {
					Token: "test-token",
				},
			},
		},
	}

	var wg sync.WaitGroup
	workers := 8

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := adapter.Connect(context.Background(), cfg, "trae", "http://127.0.0.1:42690", cfg.Local.Connections["trae-agent"])
			if err != nil {
				t.Logf("Worker %d Connect error (expected if locking timeouts occur): %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	readData, err := os.ReadFile(traeConfigPath)
	if err != nil {
		t.Fatalf("config file was deleted or cannot be read: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(readData, &parsed); err != nil {
		t.Fatalf("JSON format was corrupted under concurrency: %v. Raw data: %s", err, string(readData))
	}

	bakPath := traeConfigPath + ".vola.bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Error("Vola backup file was not created")
	}
}

func TestMcpConfigSelfHealing(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	traeDir := filepath.Join(tempDir, ".trae")
	_ = os.MkdirAll(traeDir, 0755)

	traeConfigPath := filepath.Join(traeDir, "mcp.json")
	initial := map[string]any{
		"mcpServers": map[string]any{
			"team-mcp-online-test": map[string]any{
				"url": "http://127.0.0.1:42690/mcp",
			},
		},
	}
	initialData, _ := json.MarshalIndent(initial, "", "  ")
	_ = os.WriteFile(traeConfigPath, initialData, 0644)

	adapter, _ := Resolve("trae")
	cfg := &runtimecfg.CLIConfig{
		Local: runtimecfg.LocalConfig{
			Connections: map[string]runtimecfg.LocalConnection{
				"trae-agent": {
					Token: "test-token",
				},
			},
		},
	}

	// 1. Run Connect to generate initial config and its backup file
	_, err := adapter.Connect(context.Background(), cfg, "trae", "http://127.0.0.1:42690", cfg.Local.Connections["trae-agent"])
	if err != nil {
		t.Fatalf("initial Connect failed: %v", err)
	}

	// Confirm backup file is created
	bakPath := traeConfigPath + ".vola.bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		t.Fatal("expected .vola.bak file to be created, but it does not exist")
	}

	// 2. Damage config file manually with invalid json syntax
	_ = os.WriteFile(traeConfigPath, []byte("{{{ corrupted invalid json structure!"), 0644)

	// 3. Connect again. It should detect the syntax error, load from backup and self-heal
	_, err = adapter.Connect(context.Background(), cfg, "trae", "http://127.0.0.1:42690", cfg.Local.Connections["trae-agent"])
	if err != nil {
		t.Fatalf("Connect on damaged config failed: %v", err)
	}

	// Read healed config and verify it is valid JSON
	healedData, err := os.ReadFile(traeConfigPath)
	if err != nil {
		t.Fatalf("failed to read healed file: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(healedData, &parsed); err != nil {
		t.Fatalf("config file is still corrupted: %v. Data: %s", err, string(healedData))
	}

	servers, ok := parsed["mcpServers"].(map[string]any)
	if !ok || servers["team-mcp-online-test"] == nil {
		t.Errorf("healed mcpServers do not contain backed up servers: %+v", parsed)
	}
}

func TestVolarcPathSandboxing(t *testing.T) {
	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get old cwd: %v", err)
	}

	tempDir := t.TempDir()
	
	// Create symlink `.volarc` pointing to system sensitive path /etc/hosts
	err = os.Symlink("/etc/hosts", filepath.Join(tempDir, ".volarc"))
	if err != nil {
		t.Skipf("skipping symlink test: %v", err)
	}

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldCwd)
	})

	tags := loadLocalVolarc()
	if tags != nil {
		t.Errorf("expected loadLocalVolarc to reject symlink pointing to system sensitive paths, got: %v", tags)
	}
}


