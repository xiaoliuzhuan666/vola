package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

type McpHealthStatus struct {
	Status    string    `json:"status"` // "online" or "offline"
	LatencyMs int64     `json:"latency_ms"`
	LastCheck time.Time `json:"last_check"`
}

var (
	mcpHealthCache    sync.Map // key: string (mcp server key), value: McpHealthStatus
	onceHealth        sync.Once
	healthFailures    = make(map[string]int) // key: mcp server key, value: consecutive failures count
	healthFailuresMu  sync.Mutex
	checkCycleCount   int64
	lastManualRefresh time.Time
	manualRefreshMu   sync.Mutex
)

// StartMcpHealthChecker starts a background goroutine to poll HTTP MCP servers' health.
func StartMcpHealthChecker(ctx context.Context) {
	onceHealth.Do(func() {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			// Run immediately on start
			checkAllMcpServers(ctx)

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					checkAllMcpServers(ctx)
				}
			}
		}()
	})
}

func checkAllMcpServers(ctx context.Context) {
	_, cfg, err := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath())
	if err != nil {
		return
	}

	healthFailuresMu.Lock()
	cycle := checkCycleCount
	checkCycleCount++
	healthFailuresMu.Unlock()

	// 1. Gather all configured HTTP MCP URLs and track them for pruning
	configuredMcpKeys := make(map[string]bool)
	urlsToCheck := make(map[string]string) // key: mcp server key, value: url

	statuses := platforms.AllStatuses(cfg, "")
	for _, status := range statuses {
		if status.ConfigPath == "" {
			continue
		}
		if _, err := os.Stat(status.ConfigPath); os.IsNotExist(err) {
			continue
		}
		data, err := os.ReadFile(status.ConfigPath)
		if err != nil {
			continue
		}
		var mcpConf struct {
			McpServers map[string]struct {
				URL string `json:"url"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &mcpConf); err == nil {
			for name, srv := range mcpConf.McpServers {
				if strings.HasPrefix(name, "team-mcp-") && srv.URL != "" {
					configuredMcpKeys[name] = true

					// Failure Backoff Control:
					healthFailuresMu.Lock()
					fails := healthFailures[name]
					healthFailuresMu.Unlock()

					shouldCheck := true
					if fails > 0 {
						var skipFactor int64 = 1
						if fails == 1 {
							skipFactor = 2
						} else if fails == 2 {
							skipFactor = 4
						} else {
							skipFactor = 8
						}
						if cycle%skipFactor != 0 {
							shouldCheck = false
						}
					}

					if shouldCheck {
						urlsToCheck[name] = srv.URL
					}
				}
			}
		}
	}

	// 2. Concurrently check health for servers targeted in this cycle
	var wg sync.WaitGroup
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	for key, url := range urlsToCheck {
		wg.Add(1)
		go func(mcpKey, mcpURL string) {
			defer wg.Done()
			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, mcpURL, nil)
			var resp *http.Response
			if err == nil {
				resp, err = client.Do(req)
			}

			// Fallback to GET if HEAD fails
			if err == nil && resp.StatusCode == http.StatusMethodNotAllowed {
				_ = resp.Body.Close()
				reqGet, errGet := http.NewRequestWithContext(ctx, http.MethodGet, mcpURL, nil)
				if errGet == nil {
					resp, err = client.Do(reqGet)
				}
			}

			latency := time.Since(start).Milliseconds()
			status := "offline"
			if err == nil {
				_ = resp.Body.Close()
				status = "online"
			}

			mcpHealthCache.Store(mcpKey, McpHealthStatus{
				Status:    status,
				LatencyMs: latency,
				LastCheck: time.Now(),
			})

			// Update failures tracker
			healthFailuresMu.Lock()
			if status == "online" {
				healthFailures[mcpKey] = 0
			} else {
				healthFailures[mcpKey]++
			}
			healthFailuresMu.Unlock()
		}(key, url)
	}

	wg.Wait()

	// 3. Prune deleted servers from cache and track map
	mcpHealthCache.Range(func(key, value any) bool {
		mcpKey, ok := key.(string)
		if ok {
			if !configuredMcpKeys[mcpKey] {
				mcpHealthCache.Delete(mcpKey)
				healthFailuresMu.Lock()
				delete(healthFailures, mcpKey)
				healthFailuresMu.Unlock()
			}
		}
		return true
	})
}

// handleLocalMcpHealth returns the health state of local/team HTTP MCPs.
func (s *Server) handleLocalMcpHealth(w http.ResponseWriter, r *http.Request) {
	// On-Demand Wakeup: if last manual check was > 5s ago, force background refresh
	manualRefreshMu.Lock()
	if time.Since(lastManualRefresh) > 5*time.Second {
		lastManualRefresh = time.Now()

		// Reset failure backoffs to force immediate refresh check
		healthFailuresMu.Lock()
		for k := range healthFailures {
			healthFailures[k] = 0
		}
		healthFailuresMu.Unlock()

		go checkAllMcpServers(context.Background())
	}
	manualRefreshMu.Unlock()

	data := make(map[string]McpHealthStatus)
	mcpHealthCache.Range(func(key, value any) bool {
		if k, ok := key.(string); ok {
			if val, ok := value.(McpHealthStatus); ok {
				data[k] = val
			}
		}
		return true
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"data": data,
	})
}
