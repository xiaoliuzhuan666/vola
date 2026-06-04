package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

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

	// 3. 读取并修改 config
	var configMap map[string]interface{}
	data, err := os.ReadFile(claudePath)
	if err == nil {
		_ = json.Unmarshal(data, &configMap)
	}
	if configMap == nil {
		configMap = make(map[string]interface{})
	}

	mcpServers, ok := configMap["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		configMap["mcpServers"] = mcpServers
	}

	mcpServers["vola"] = map[string]interface{}{
		"command": exePath,
		"args":    []string{"mcp", "stdio", "--token", ownerToken},
	}

	// 4. 写回配置文件，确保父目录存在
	if err := os.MkdirAll(filepath.Dir(claudePath), 0755); err != nil {
		respondInternalError(w, err)
		return
	}

	newData, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		respondInternalError(w, err)
		return
	}

	if err := os.WriteFile(claudePath, newData, 0644); err != nil {
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

	data, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			respondOK(w, map[string]interface{}{"success": true})
			return
		}
		respondInternalError(w, err)
		return
	}

	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		respondInternalError(w, err)
		return
	}

	mcpServers, ok := configMap["mcpServers"].(map[string]interface{})
	if ok {
		delete(mcpServers, "vola")
	}

	newData, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		respondInternalError(w, err)
		return
	}

	if err := os.WriteFile(claudePath, newData, 0644); err != nil {
		respondInternalError(w, err)
		return
	}

	respondOK(w, map[string]interface{}{"success": true})
}
