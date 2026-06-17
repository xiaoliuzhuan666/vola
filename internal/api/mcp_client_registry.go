package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

type mcpClientStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Installed  bool   `json:"installed"`
	Registered bool   `json:"registered"`
	ConfigPath string `json:"config_path,omitempty"`
}

type registerClientRequest struct {
	ClientID string `json:"client_id"`
}

func getClaudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
}

func (s *Server) handleLocalMCPClientsList(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}

	clients := []mcpClientStatus{}

	// 探测 Claude Desktop
	claudePath, err := getClaudeDesktopConfigPath()
	if err == nil {
		status := mcpClientStatus{
			ID:         "claude-desktop",
			Name:       "Claude Desktop",
			ConfigPath: claudePath,
		}
		if _, statErr := os.Stat(claudePath); statErr == nil {
			status.Installed = true
			if data, readErr := os.ReadFile(claudePath); readErr == nil {
				var configMap map[string]interface{}
				if json.Unmarshal(data, &configMap) == nil {
					if mcpServers, ok := configMap["mcpServers"].(map[string]interface{}); ok {
						if _, exists := mcpServers["vola"]; exists {
							status.Registered = true
						}
					}
				}
			}
		}
		clients = append(clients, status)
	}

	respondOK(w, clients)
}

func (s *Server) handleLocalMCPClientsRegister(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}

	var req registerClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.ClientID != "claude-desktop" {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "unsupported client_id")
		return
	}

	claudePath, err := getClaudeDesktopConfigPath()
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 1. 读取 config.json 以获得 OwnerToken
	_, runCfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	ownerToken := runCfg.Local.OwnerToken
	if ownerToken == "" {
		respondInternalError(w, errors.New("owner_token not found in local config"))
		return
	}

	// 2. 获取当前运行的 Go 二进制路径
	exePath, err := os.Executable()
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 3. 使用 SafeUpdateMcpConfig 安全修改 config 并不把明文 token 曝露在 args 命令行里
	err = platforms.SafeUpdateMcpConfig(claudePath, func(configMap map[string]interface{}) error {
		mcpServers, ok := configMap["mcpServers"].(map[string]interface{})
		if !ok {
			mcpServers = make(map[string]interface{})
			configMap["mcpServers"] = mcpServers
		}
		mcpServers["vola"] = map[string]interface{}{
			"command": exePath,
			"args":    []string{"mcp", "stdio", "--token-env", "VOLA_TOKEN"},
			"env": map[string]string{
				"VOLA_TOKEN": ownerToken,
			},
		}
		return nil
	})
	if err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{"success": true})
}

func (s *Server) handleLocalMCPClientsUnregister(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}

	var req registerClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid request body")
		return
	}

	if req.ClientID != "claude-desktop" {
		respondError(w, http.StatusBadRequest, ErrCodeBadRequest, "unsupported client_id")
		return
	}

	claudePath, err := getClaudeDesktopConfigPath()
	if err != nil {
		respondInternalError(w, err)
		return
	}

	// 使用 SafeUpdateMcpConfig 安全注销
	err = platforms.SafeUpdateMcpConfig(claudePath, func(configMap map[string]interface{}) error {
		mcpServers, ok := configMap["mcpServers"].(map[string]interface{})
		if ok {
			delete(mcpServers, "vola")
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			respondOK(w, map[string]interface{}{"success": true})
			return
		}
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{"success": true})
}
