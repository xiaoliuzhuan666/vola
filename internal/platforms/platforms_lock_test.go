package platforms_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/agi-bar/vola/internal/platforms"
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

	adapter, err := platforms.Resolve("trae")
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
