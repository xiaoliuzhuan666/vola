package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AgentAuditMiddleware intercepts requests going to the /agent/ group and audits them.
func (s *Server) AgentAuditMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Skip preflight CORS requests.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		slog.Info("AgentAuditMiddleware triggered", "path", r.URL.Path, "method", r.Method)

		// 2. Resolve userID from context.
		rawUID := r.Context().Value(ctxKeyUserID)
		slog.Info("Resolved ctxKeyUserID from context", "value", rawUID, "type", fmt.Sprintf("%T", rawUID))

		userID, ok := userIDFromCtx(r.Context())
		if !ok {
			slog.Warn("AgentAuditMiddleware userIDFromCtx failed", "ok", ok)
			// If not yet authenticated (should not happen if apiKeyMiddleware is registered), let next handle it.
			next.ServeHTTP(w, r)
			return
		}

		// 3. Resolve connection or token metadata from context.
		conn := connectionFromCtx(r.Context())
		token := scopedTokenFromCtx(r.Context())

		agentName := "Unknown Agent"
		var connectionID *uuid.UUID

		if conn != nil {
			agentName = conn.Name
			connectionID = &conn.ID
		} else if token != nil {
			agentName = "Scoped Token: " + token.Name
		}

		// 4. Resolve accessed resource and evaluate risk.
		requestPath := r.URL.Path
		accessedResource := requestPath
		if strings.HasPrefix(accessedResource, "/agent/tree/") {
			accessedResource = strings.TrimPrefix(accessedResource, "/agent/tree/")
		}
		isScopedVaultEndpoint := strings.HasPrefix(requestPath, "/agent/vault/")

		riskLevel := "LOW"
		isSensitive := false
		sensitiveKeywords := []string{".env", "id_rsa", "config.json", "credentials", "session", "cookie", "vault"}
		for _, kw := range sensitiveKeywords {
			if strings.Contains(strings.ToLower(accessedResource), kw) {
				riskLevel = "HIGH"
				isSensitive = true
				break
			}
		}

		// 5. Active defense firewall block if high risk.
		if isSensitive && !isScopedVaultEndpoint {
			riskLevel = "BLOCKED"
			// Record the blocked log to database.
			_ = s.DashboardService.LogActivity(r.Context(), userID, connectionID, r.Method, requestPath, map[string]interface{}{
				"request_method":    r.Method,
				"request_path":      requestPath,
				"accessed_resource": accessedResource,
				"risk_level":        riskLevel,
				"status_code":       http.StatusForbidden,
				"agent_name":        agentName,
			})
			respondError(w, http.StatusForbidden, ErrCodeForbidden, "Access blocked by Vola Security Firewall (PII/Credential protection)")
			return
		} else if isSensitive {
			riskLevel = "HIGH"
		}

		// 6. Pass through to next handler and record success log.
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(ww, r)

		// Record the successful log asynchronously.
		go func() {
			_ = s.DashboardService.LogActivity(context.Background(), userID, connectionID, r.Method, requestPath, map[string]interface{}{
				"request_method":    r.Method,
				"request_path":      requestPath,
				"accessed_resource": accessedResource,
				"risk_level":        riskLevel,
				"status_code":       ww.statusCode,
				"agent_name":        agentName,
			})
		}()
	})
}

type dashboardActivityResponse struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	ConnectionID string                 `json:"connection_id,omitempty"`
	Action       string                 `json:"action"`
	Path         string                 `json:"path"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
}

// handleGetDashboardActivities retrieves recent audited agent activities.
func (s *Server) handleGetDashboardActivities(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromCtx(r.Context())
	if !ok {
		respondUnauthorized(w)
		return
	}

	acts, err := s.DashboardService.GetActivities(r.Context(), userID, 25)
	if err != nil {
		respondInternalError(w, err)
		return
	}

	activities := make([]dashboardActivityResponse, len(acts))
	for i, act := range acts {
		connID := ""
		if act.ConnectionID != uuid.Nil {
			connID = act.ConnectionID.String()
		}

		activities[i] = dashboardActivityResponse{
			ID:           act.ID.String(),
			UserID:       act.UserID.String(),
			ConnectionID: connID,
			Action:       act.Action,
			Path:         act.Path,
			Metadata:     act.Metadata,
			CreatedAt:    act.CreatedAt,
		}
	}

	respondOK(w, activities)
}
