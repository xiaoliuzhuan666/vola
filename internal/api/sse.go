package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type sseEventRecord struct {
	TeamID    string
	EventType string
	Data      string
	Timestamp time.Time
}

type EventBroker struct {
	mu        sync.RWMutex
	listeners map[string][]chan string // teamID -> channels
	history   []sseEventRecord
}

var GlobalBroker = &EventBroker{
	listeners: make(map[string][]chan string),
}

func (b *EventBroker) Subscribe(teamID string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 32)
	b.listeners[teamID] = append(b.listeners[teamID], ch)
	return ch
}

func (b *EventBroker) Unsubscribe(teamID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.listeners[teamID]
	for i, c := range list {
		if c == ch {
			b.listeners[teamID] = append(list[:i], list[i+1:]...)
			close(ch)
			break
		}
	}
}

func (b *EventBroker) Publish(teamID string, eventType string, data string) {
	b.mu.Lock()
	record := sseEventRecord{
		TeamID:    teamID,
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now(),
	}
	b.history = append(b.history, record)

	// Keep history only for the last 5 minutes
	cutoff := time.Now().Add(-5 * time.Minute)
	prunedIdx := -1
	for i, r := range b.history {
		if r.Timestamp.After(cutoff) {
			prunedIdx = i
			break
		}
	}
	if prunedIdx > 0 {
		b.history = b.history[prunedIdx:]
	}
	b.mu.Unlock()

	b.mu.RLock()
	defer b.mu.RUnlock()
	payload := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, data)
	for _, ch := range b.listeners[teamID] {
		select {
		case ch <- payload:
		default:
			// Non-blocking
		}
	}
}

func (b *EventBroker) GetHistory(teamID string, since time.Time) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []string
	for _, r := range b.history {
		if r.TeamID == teamID && r.Timestamp.After(since) {
			out = append(out, fmt.Sprintf("event: %s\ndata: %s\n\n", r.EventType, r.Data))
		}
	}
	return out
}

// handleTeamEvents serves the SSE stream for a team.
func (s *Server) handleTeamEvents(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team")
	if teamID == "" {
		http.Error(w, "missing team id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send an initial handshake comment to establish connection
	_, _ = fmt.Fprint(w, ": ok\n\n")
	flusher.Flush()

	// Replay missed events during brief disconnection
	if lastSeenStr := r.URL.Query().Get("last_seen_ms"); lastSeenStr != "" {
		if ms, err := strconv.ParseInt(lastSeenStr, 10, 64); err == nil && ms > 0 {
			since := time.Unix(0, ms*int64(time.Millisecond))
			missedEvents := GlobalBroker.GetHistory(teamID, since)
			for _, msg := range missedEvents {
				_, _ = fmt.Fprint(w, msg)
			}
			flusher.Flush()
		}
	}

	ch := GlobalBroker.Subscribe(teamID)
	defer GlobalBroker.Unsubscribe(teamID, ch)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprint(w, msg)
			flusher.Flush()
		case <-ticker.C:
			// Heartbeat comment
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

var (
	activeListeners   = make(map[string]context.CancelFunc)
	activeListenersMu sync.Mutex
	sseIdleTimeout    = 45 * time.Second
)

func (s *Server) StartTeamEventsListener(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		// Run once immediately on start
		s.syncTeamListeners(ctx)

		for {
			select {
			case <-ctx.Done():
				activeListenersMu.Lock()
				for _, cancel := range activeListeners {
					cancel()
				}
				activeListeners = make(map[string]context.CancelFunc)
				activeListenersMu.Unlock()
				return
			case <-ticker.C:
				s.syncTeamListeners(ctx)
			}
		}
	}()
}

func (s *Server) syncTeamListeners(ctx context.Context) {
	_, cfg, err := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath())
	if err != nil {
		return
	}
	profile := cfg.Profiles[cfg.CurrentProfile]
	if profile.APIBase == "" || profile.Token == "" {
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(profile.APIBase, "/")+"/api/teams", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+profile.Token)
	resp, err := client.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode == http.StatusUnauthorized && profile.RefreshToken != "" {
		newToken, refreshErr := silentRefreshToken(ctx, profile.APIBase, profile.Token)
		if refreshErr == nil {
			resp.Body.Close()
			client2 := &http.Client{Timeout: 10 * time.Second}
			req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(profile.APIBase, "/")+"/api/teams", nil)
			req2.Header.Set("Authorization", "Bearer "+newToken)
			resp2, err2 := client2.Do(req2)
			if err2 == nil {
				resp = resp2
			} else {
				return
			}
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var teamsResp struct {
		Teams []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"teams"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&teamsResp); err != nil {
		return
	}

	activeListenersMu.Lock()
	defer activeListenersMu.Unlock()

	currentTeams := make(map[string]bool)
	for _, team := range teamsResp.Teams {
		currentTeams[team.ID] = true
		if _, ok := activeListeners[team.ID]; !ok {
			teamCtx, cancel := context.WithCancel(ctx)
			activeListeners[team.ID] = cancel
			go s.runTeamSseLoop(teamCtx, profile.APIBase, profile.Token, team.ID)
		}
	}

	// Clean up loops for teams we are no longer part of
	for teamID, cancel := range activeListeners {
		if !currentTeams[teamID] {
			cancel()
			delete(activeListeners, teamID)
		}
	}
}

func (s *Server) runTeamSseLoop(ctx context.Context, apiBase, token, teamID string) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	var lastSeenTime time.Time

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := s.subscribeTeamSse(ctx, apiBase, token, teamID, &lastSeenTime)
			if err != nil {
				// Log or handle error if needed
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}
}

func (s *Server) subscribeTeamSse(ctx context.Context, apiBase, token, teamID string, lastSeenTime *time.Time) error {
	client := &http.Client{Timeout: 0}

	var lastSeenMs int64 = 0
	if lastSeenTime != nil && !lastSeenTime.IsZero() {
		lastSeenMs = lastSeenTime.UnixNano() / int64(time.Millisecond)
	}

	url := fmt.Sprintf("%s/api/teams/%s/events", strings.TrimRight(apiBase, "/"), teamID)
	if lastSeenMs > 0 {
		url = fmt.Sprintf("%s?last_seen_ms=%d", url, lastSeenMs)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		_, cfg, loadErr := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath())
		if loadErr == nil {
			profile := cfg.Profiles[cfg.CurrentProfile]
			if profile.RefreshToken != "" {
				newToken, refreshErr := silentRefreshToken(ctx, apiBase, token)
				if refreshErr == nil {
					resp.Body.Close()
					url2 := fmt.Sprintf("%s/api/teams/%s/events", strings.TrimRight(apiBase, "/"), teamID)
					if lastSeenMs > 0 {
						url2 = fmt.Sprintf("%s?last_seen_ms=%d", url2, lastSeenMs)
					}
					req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, url2, nil)
					req2.Header.Set("Authorization", "Bearer "+newToken)
					req2.Header.Set("Accept", "text/event-stream")
					resp2, err2 := client.Do(req2)
					if err2 == nil {
						resp = resp2
					} else {
						return err2
					}
				}
			}
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected SSE status code: %v", resp.StatusCode)
	}

	// Watchdog to detect half-open TCP connections (idle timeout)
	var mu sync.Mutex
	lastSeen := time.Now()

	watchdogCtx, watchdogCancel := context.WithCancel(ctx)
	defer watchdogCancel()

	go func() {
		tickerInterval := sseIdleTimeout / 3
		if tickerInterval < 5*time.Millisecond {
			tickerInterval = 5 * time.Millisecond
		}
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-watchdogCtx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				idleTime := time.Since(lastSeen)
				mu.Unlock()
				if idleTime > sseIdleTimeout {
					// silence -> connection dead. Force close to trigger reconnect.
					_ = resp.Body.Close()
					return
				}
			}
		}
	}()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}

		now := time.Now()
		mu.Lock()
		lastSeen = now
		mu.Unlock()
		if lastSeenTime != nil {
			*lastSeenTime = now
		}

		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event:") {
			eventType := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			dataLine, err := reader.ReadString('\n')
			if err != nil {
				return err
			}

			now = time.Now()
			mu.Lock()
			lastSeen = now
			mu.Unlock()
			if lastSeenTime != nil {
				*lastSeenTime = now
			}

			dataLine = strings.TrimSpace(dataLine)
			if strings.HasPrefix(dataLine, "data:") {
				s.handleSseEvent(ctx, teamID, eventType)
			}
		}
	}
}

func (s *Server) handleSseEvent(ctx context.Context, teamID string, eventType string) {
	if eventType == "mcp_update" {
		_, cfg, err := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath())
		if err != nil {
			return
		}
		for id, conn := range cfg.Local.Connections {
			adapter, err := platforms.Resolve(id)
			if err == nil {
				_, _ = adapter.Connect(ctx, cfg, id, conn.LastPlatformURL, conn)
			}
		}
	} else if eventType == "skill_update" || eventType == "skill_publish" {
		if !s.isLocalMode() {
			return
		}
		if s.TeamService == nil || s.FileTreeService == nil {
			return
		}

		teamUUID, err := uuid.Parse(teamID)
		if err != nil {
			return
		}

		go func() {
			time.Sleep(1 * time.Second)

			syncCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			team, err := s.TeamService.GetForUser(syncCtx, s.LocalOwnerID, teamUUID)
			if err != nil {
				return
			}

			target := scopedHubTarget{
				Scope:  "team",
				UserID: team.HubUserID,
				Team:   team,
			}

			req := localSkillSyncRequest{
				TeamID:           teamID,
				AckQualityReview: true,
			}
			if _, cfg, err := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath()); err == nil {
				for id := range cfg.Local.Connections {
					req.AgentIDs = append(req.AgentIDs, id)
				}
			}

			resp, err := s.buildLocalSkillSyncResponse(syncCtx, target, req, true, false)
			if err != nil {
				slog.Error("SSE auto sync failed", "err", err)
			} else {
				if resp.Blocked {
					slog.Warn("SSE auto sync blocked by quality gates")
				} else {
					slog.Info("SSE auto sync succeeded", "agents", len(resp.Agents))
				}
			}
		}()
	}
}

var refreshMu sync.Mutex

func silentRefreshToken(ctx context.Context, apiBase string, currentToken string) (string, error) {
	refreshMu.Lock()
	defer refreshMu.Unlock()

	// Load the latest config from disk inside the critical section
	configPath, cfg, err := runtimecfg.LoadConfig(runtimecfg.DefaultConfigPath())
	if err != nil {
		return "", err
	}
	profile := cfg.Profiles[cfg.CurrentProfile]

	// If the token in config.json is already different from the expired token we hold,
	// it means another goroutine has refreshed it. We can just reuse it.
	if profile.Token != currentToken && profile.Token != "" {
		return profile.Token, nil
	}

	if profile.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/api/auth/refresh", strings.TrimRight(apiBase, "/"))
	
	reqBody, _ := json.Marshal(map[string]string{
		"refresh_token": profile.RefreshToken,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(reqBody)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh failed: status %v", resp.StatusCode)
	}

	var refreshResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refreshResp); err != nil {
		return "", err
	}

	// Update local config.json file
	profile.Token = refreshResp.AccessToken
	if refreshResp.RefreshToken != "" {
		profile.RefreshToken = refreshResp.RefreshToken
	}
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	cfg.Profiles[cfg.CurrentProfile] = profile
	_ = runtimecfg.SaveConfig(configPath, cfg)

	return refreshResp.AccessToken, nil
}

