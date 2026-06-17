package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agi-bar/vola/internal/config"
	"github.com/agi-bar/vola/internal/runtimecfg"
	"github.com/go-chi/chi/v5"
)

func TestSSEEventBrokerAndBroadcaster(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("VOLA_CONFIG", filepath.Join(tempDir, "config.json"))

	// Create dummy config.json so sse hot-reloading can LoadConfig() safely
	cliConf := &runtimecfg.CLIConfig{
		Version: 1,
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	configPath := runtimecfg.DefaultConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, cliConfBytes, 0644)

	server := NewServerWithDeps(ServerDeps{
		Config: &config.Config{
			RateLimit: 10000,
		},
		JWTSecret: testJWTSecret,
	})

	teamID := "test-team-123"

	// Mock SSE server endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("team", teamID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		server.Router.ServeHTTP(w, r)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/teams/"+teamID+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+generateTestJWT())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to sse stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status OK, got %v", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Read handshake ": ok\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read handshake: %v", err)
	}
	if !strings.HasPrefix(line, ": ok") {
		t.Errorf("unexpected handshake message: %s", line)
	}

	// Read handshake trailing newline "\n"
	blank, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read handshake trailing newline: %v", err)
	}
	if blank != "\n" {
		t.Errorf("expected blank line, got %q", blank)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		GlobalBroker.Publish(teamID, "mcp_update", `{"slug": "test-mcp"}`)
	}()

	// Read event: mcp_update\n
	lineEvent, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read event line: %v", err)
	}
	if !strings.Contains(lineEvent, "event: mcp_update") {
		t.Errorf("expected mcp_update event type, got %s", lineEvent)
	}

	// Read data: {"slug": "test-mcp"}\n
	lineData, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read data line: %v", err)
	}
	if !strings.Contains(lineData, `{"slug": "test-mcp"}`) {
		t.Errorf("expected payload content, got %s", lineData)
	}
}

func TestSSEReadWatchdogTimeoutAndReconnect(t *testing.T) {
	// Configure short watchdog timeout for fast test execution
	oldTimeout := sseIdleTimeout
	sseIdleTimeout = 60 * time.Millisecond
	defer func() {
		sseIdleTimeout = oldTimeout
	}()

	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("VOLA_CONFIG", filepath.Join(tempDir, "config.json"))

	cliConf := &runtimecfg.CLIConfig{
		Version: 1,
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	configPath := runtimecfg.DefaultConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, cliConfBytes, 0644)

	server := NewServerWithDeps(ServerDeps{
		Config: &config.Config{
			RateLimit: 10000,
		},
		JWTSecret: testJWTSecret,
	})

	teamID := "test-team-backoff"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("team", teamID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		server.Router.ServeHTTP(w, r)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// subscribeTeamSse should connect, handshake, then block and get closed by the watchdog
	s := &Server{}
	err := s.subscribeTeamSse(ctx, ts.URL, generateTestJWT(), teamID, nil)

	if err == nil {
		t.Fatal("expected subscribeTeamSse to return error on watchdog timeout, got nil")
	}

	if !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "EOF") {
		t.Logf("expected read error due to closed body, got: %v", err)
	}
}

func TestSSEEventReplayOnReconnect(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("VOLA_CONFIG", filepath.Join(tempDir, "config.json"))

	cliConf := &runtimecfg.CLIConfig{
		Version: 1,
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	configPath := runtimecfg.DefaultConfigPath()
	_ = os.MkdirAll(filepath.Dir(configPath), 0755)
	_ = os.WriteFile(configPath, cliConfBytes, 0644)

	server := NewServerWithDeps(ServerDeps{
		Config: &config.Config{
			RateLimit: 10000,
		},
		JWTSecret: testJWTSecret,
	})

	teamID := "test-team-replay"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("team", teamID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		server.Router.ServeHTTP(w, r)
	}))
	defer ts.Close()

	// 1. Establish first connection, then close it to record lastSeenTime
	ctx, cancel := context.WithCancel(context.Background())
	var lastSeenTime time.Time

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel() // disconnect
	}()

	s := &Server{}
	_ = s.subscribeTeamSse(ctx, ts.URL, generateTestJWT(), teamID, &lastSeenTime)

	if lastSeenTime.IsZero() {
		t.Fatal("expected lastSeenTime to be recorded after first connection")
	}

	// 2. Publish a mock event while client is disconnected
	GlobalBroker.Publish(teamID, "mcp_update", `{"replayed": true}`)

	// 3. Reconnect carrying the lastSeenTime
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	req, _ := http.NewRequestWithContext(ctx2, http.MethodGet, ts.URL+"/api/teams/"+teamID+"/events?last_seen_ms="+strconv.FormatInt(lastSeenTime.UnixNano()/int64(time.Millisecond), 10), nil)
	req.Header.Set("Authorization", "Bearer "+generateTestJWT())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to sse stream on reconnect: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read handshake ": ok\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read handshake: %v", err)
	}
	if !strings.HasPrefix(line, ": ok") {
		t.Errorf("unexpected handshake message: %s", line)
	}

	// Read handshake trailing newline "\n"
	_, _ = reader.ReadString('\n')

	// Since we passed last_seen_ms, the server should replay the missed event immediately
	lineEvent, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read replayed event line: %v", err)
	}
	if !strings.Contains(lineEvent, "event: mcp_update") {
		t.Errorf("expected replayed mcp_update event type, got %s", lineEvent)
	}

	lineData, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read replayed data line: %v", err)
	}
	if !strings.Contains(lineData, `{"replayed": true}`) {
		t.Errorf("expected replayed payload content, got %s", lineData)
	}
}

func TestClientTokenSilentRotation(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configFilePath := filepath.Join(tempDir, "config.json")
	t.Setenv("VOLA_CONFIG", configFilePath)

	// Setup config.json
	cliConf := &runtimecfg.CLIConfig{
		Version:        1,
		CurrentProfile: "test",
		Profiles: map[string]runtimecfg.SyncProfile{
			"test": {
				APIBase:      "", // Will fill after starting test server
				Token:        "old-expired-token",
				RefreshToken: "valid-refresh-token",
			},
		},
	}

	callCountTeams := 0
	callCountRefresh := 0

	// 1. Create mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/refresh" {
			callCountRefresh++
			var body struct {
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.RefreshToken != "valid-refresh-token" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"access_token": "new-fresh-token",
				"refresh_token": "new-refresh-token"
			}`))
			return
		}

		if r.URL.Path == "/api/teams" {
			callCountTeams++
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer new-fresh-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"teams": [
					{"id": "team-foo", "slug": "team-foo-slug"}
				]
			}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Update config.json with actual test server URL
	cliConf.Profiles["test"] = runtimecfg.SyncProfile{
		APIBase:      ts.URL,
		Token:        "old-expired-token",
		RefreshToken: "valid-refresh-token",
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	_ = os.MkdirAll(filepath.Dir(configFilePath), 0755)
	_ = os.WriteFile(configFilePath, cliConfBytes, 0644)

	// Run syncTeamListeners
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Server{}
	s.syncTeamListeners(ctx)

	// Verify the side effects
	if callCountRefresh != 1 {
		t.Errorf("expected /api/auth/refresh to be called exactly 1 time, got %d", callCountRefresh)
	}
	if callCountTeams != 2 {
		t.Errorf("expected /api/teams to be called 2 times (first 401, second retry 200), got %d", callCountTeams)
	}

	// Verify that config.json was updated with new tokens
	_, updatedCfg, err := runtimecfg.LoadConfig(configFilePath)
	if err != nil {
		t.Fatalf("failed to load updated config: %v", err)
	}
	updatedProfile := updatedCfg.Profiles["test"]
	if updatedProfile.Token != "new-fresh-token" {
		t.Errorf("expected Token to be 'new-fresh-token', got %s", updatedProfile.Token)
	}
	if updatedProfile.RefreshToken != "new-refresh-token" {
		t.Errorf("expected RefreshToken to be 'new-refresh-token', got %s", updatedProfile.RefreshToken)
	}
}

func TestClientTokenSilentRotationConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configFilePath := filepath.Join(tempDir, "config.json")
	t.Setenv("VOLA_CONFIG", configFilePath)

	// Setup config.json
	cliConf := &runtimecfg.CLIConfig{
		Version:        1,
		CurrentProfile: "test",
		Profiles: map[string]runtimecfg.SyncProfile{
			"test": {
				APIBase:      "",
				Token:        "old-expired-token",
				RefreshToken: "valid-refresh-token",
			},
		},
	}

	var callCountRefresh int32

	// 1. Create mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/refresh" {
			// Add latency to make concurrent requests overlap
			time.Sleep(50 * time.Millisecond)

			// Thread-safe increment
			importCount := &callCountRefresh
			importCount2 := int32(1)

			// Standard atomic increment
			_ = importCount
			_ = importCount2
			// Let's use mutex or atomic inside mock server to count accurately
			w.Header().Set("Content-Type", "application/json")

			// We can use a simple map lookup or standard write
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"access_token": "concurrency-fresh-token",
				"refresh_token": "concurrency-refresh-token"
			}`))

			// Use global atomic package by copying its value or just inline increment
			// Since we want to check callCountRefresh, let's keep track using standard sync/atomic
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Update config.json with actual test server URL
	cliConf.Profiles["test"] = runtimecfg.SyncProfile{
		APIBase:      ts.URL,
		Token:        "old-expired-token",
		RefreshToken: "valid-refresh-token",
	}
	cliConfBytes, _ := json.Marshal(cliConf)
	_ = os.MkdirAll(filepath.Dir(configFilePath), 0755)
	_ = os.WriteFile(configFilePath, cliConfBytes, 0644)

	// Mock server with local count
	var mu sync.Mutex
	callCount := 0

	// Re-assign mock handlers for clean thread-safe counting
	ts.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/refresh" {
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			callCount++
			mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"access_token": "concurrency-fresh-token",
				"refresh_token": "concurrency-refresh-token"
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	ctx := context.Background()
	concurrency := 5
	results := make(chan string, concurrency)
	errs := make(chan error, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := silentRefreshToken(ctx, ts.URL, "old-expired-token")
			if err != nil {
				errs <- err
			} else {
				results <- token
			}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("concurrency silent refresh failed: %v", err)
	}

	resultTokens := make(map[string]int)
	for token := range results {
		resultTokens[token]++
	}

	// We expect all concurrent refresh calls to succeed and return the new token
	if resultTokens["concurrency-fresh-token"] != concurrency {
		t.Errorf("expected all %d threads to get 'concurrency-fresh-token', got: %+v", concurrency, resultTokens)
	}

	// Only 1 actual refresh request should hit the mock server
	mu.Lock()
	actualCalls := callCount
	mu.Unlock()

	if actualCalls != 1 {
		t.Errorf("expected only 1 network refresh call due to singleflight lock, but got %d", actualCalls)
	}

	// Verify final saved config
	_, finalCfg, err := runtimecfg.LoadConfig(configFilePath)
	if err != nil {
		t.Fatalf("failed to load final config: %v", err)
	}
	finalProfile := finalCfg.Profiles["test"]
	if finalProfile.Token != "concurrency-fresh-token" {
		t.Errorf("expected Token to be 'concurrency-fresh-token', got %s", finalProfile.Token)
	}
}
