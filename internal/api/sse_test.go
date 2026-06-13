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
	err := subscribeTeamSse(ctx, ts.URL, generateTestJWT(), teamID, nil)

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

	_ = subscribeTeamSse(ctx, ts.URL, generateTestJWT(), teamID, &lastSeenTime)

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

