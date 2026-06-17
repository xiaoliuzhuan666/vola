package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/runtimecfg"
)

const localToolStatusVersion = "vola.local-tool-status/v1"

type localToolStatusResponse struct {
	Version                 string                            `json:"version"`
	GeneratedAt             string                            `json:"generated_at"`
	Platforms               []localToolStatusPlatform         `json:"platforms"`
	ResourceRecommendations []localToolResourceRecommendation `json:"resource_recommendations"`
}

type localToolStatusPlatform struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Installed           bool     `json:"installed"`
	Connected           bool     `json:"connected"`
	ConfigPath          string   `json:"config_path,omitempty"`
	EntrypointInstalled bool     `json:"entrypoint_installed"`
	EntrypointPath      string   `json:"entrypoint_path,omitempty"`
	AutoSyncSupported   bool     `json:"auto_sync_supported"`
	ExportSupported     bool     `json:"export_supported"`
	SyncMode            string   `json:"sync_mode"`
	StatusLabel         string   `json:"status_label"`
	NextAction          string   `json:"next_action"`
	Reasons             []string `json:"reasons,omitempty"`
	ChatUsage           []string `json:"chat_usage,omitempty"`
}

type localToolResourceRecommendation struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	PreviewOnly bool     `json:"preview_only"`
	Description string   `json:"description"`
	Steps       []string `json:"steps,omitempty"`
	Platforms   []string `json:"platforms,omitempty"`
}

func (s *Server) handleLocalToolsStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := s.checkLocalSkillSyncAccess(w, r)
	if !ok {
		return
	}
	if !s.ensureLocalPlatformMode(w) {
		return
	}

	_, cfg, err := runtimecfg.LoadConfig("")
	if err != nil {
		respondInternalError(w, err)
		return
	}
	if err := runtimecfg.EnsureLocalDefaults(cfg); err != nil {
		respondInternalError(w, err)
		return
	}

	daemonURL := strings.TrimSpace(cfg.Local.PublicBaseURL)
	if _, state, err := runtimecfg.LoadState(""); err == nil && state != nil && strings.TrimSpace(state.APIBase) != "" {
		daemonURL = strings.TrimSpace(state.APIBase)
	}

	statuses := platforms.AllStatuses(cfg, daemonURL)
	platformsOut := make([]localToolStatusPlatform, 0, 4)
	for _, status := range statuses {
		switch status.ID {
		case "codex", "claude-code", "cursor-agent", "gemini-cli":
			platformsOut = append(platformsOut, localToolStatusFromPlatformStatus(status))
		default:
			continue
		}
	}

	respondOK(w, localToolStatusResponse{
		Version:                 localToolStatusVersion,
		GeneratedAt:             time.Now().UTC().Format(time.RFC3339),
		Platforms:               platformsOut,
		ResourceRecommendations: localToolResourceRecommendations(),
	})
}

func localToolStatusFromPlatformStatus(status platforms.Status) localToolStatusPlatform {
	out := localToolStatusPlatform{
		ID:                  status.ID,
		Name:                status.DisplayName,
		Installed:           status.Installed,
		Connected:           status.Connected,
		ConfigPath:          status.ConfigPath,
		EntrypointInstalled: status.EntrypointInstalled,
		EntrypointPath:      status.EntrypointPath,
		ChatUsage:           append([]string{}, status.ChatUsage...),
	}

	switch status.ID {
	case "codex", "claude-code":
		out.AutoSyncSupported = true
		out.ExportSupported = true
		out.SyncMode = "auto-sync"
		out.Reasons = []string{
			"支持团队 MCP 本机刷新。",
			"支持团队 Skill 写入 Vola 管理目录。",
		}
		if !status.Installed {
			out.StatusLabel = "未检测到本机工具"
			out.NextAction = "安装本机工具后再连接 Vola。"
		} else if !status.Connected {
			out.StatusLabel = "未连接"
			out.NextAction = "先执行连接，再回到页面同步团队资产。"
		} else if !status.EntrypointInstalled {
			out.StatusLabel = "已连接，入口待刷新"
			out.NextAction = "点击同步到本机，刷新 Vola Skill 和团队 MCP。"
		} else {
			out.StatusLabel = "可自动同步"
			out.NextAction = "团队资产更新后点击同步到本机。"
		}
	case "cursor-agent":
		out.AutoSyncSupported = false
		out.ExportSupported = true
		out.SyncMode = "export-only"
		out.StatusLabel = "只导出"
		out.NextAction = "下载导出包，再按项目规则手工整理到 Cursor。"
		out.Reasons = []string{
			"Cursor rules 通常按项目生效。",
			"Vola 不自动修改 Cursor 全局或项目配置。",
		}
	case "gemini-cli":
		out.AutoSyncSupported = false
		out.ExportSupported = true
		out.SyncMode = "manual"
		out.StatusLabel = "手工处理"
		out.NextAction = "下载导出包，再手工引用到 GEMINI.md 或已有指南。"
		out.Reasons = []string{
			"Gemini CLI 没有 Vola 可安全管理的 Skill 目录契约。",
			"Vola 不自动修改 GEMINI.md 或 Gemini CLI 配置。",
		}
	default:
		out.SyncMode = "unknown"
		out.StatusLabel = "未支持"
	}
	return out
}

func localToolResourceRecommendations() []localToolResourceRecommendation {
	return []localToolResourceRecommendation{
		{
			ID:          "team-skill-starter-pack",
			Title:       "团队 Skill 入门资料包",
			Category:    "skill-pack",
			PreviewOnly: true,
			Description: "把评审、发布、客服知识等团队经验整理成私有 Hub 里的 Skill 集合。",
			Platforms:   []string{"codex", "claude-code", "cursor", "gemini-cli"},
			Steps: []string{
				"先在 Team Library 创建目录模板。",
				"把可运行内容放入 /skills/<name>/SKILL.md。",
				"Codex / Claude Code 走本机同步，Cursor / Gemini CLI 下载导出包。",
			},
		},
		{
			ID:          "filesystem-mcp-preview",
			Title:       "文件系统 MCP 配置预览",
			Category:    "mcp-server",
			PreviewOnly: true,
			Description: "适合团队讨论本地文件访问范围。Vola 只展示配置建议，不安装或启用第三方 server。",
			Platforms:   []string{"codex", "claude-code"},
			Steps: []string{
				"确认要暴露的目录和负责人。",
				"在 /team/mcp 记录命令、环境变量和安全说明。",
				"由成员自行安装 server 后，再通过团队 MCP 发布和本机同步入口刷新。",
			},
		},
		{
			ID:          "private-resource-index",
			Title:       "私有资源索引",
			Category:    "resource-index",
			PreviewOnly: true,
			Description: "把外部 Skill、MCP、prompt 链接作为团队资料保存，先预览再决定是否导入。",
			Platforms:   []string{"codex", "claude-code", "cursor", "gemini-cli"},
			Steps: []string{
				"把候选资源写入 /team/playbooks 或 /team/mcp。",
				"标注来源、用途、权限范围和维护人。",
				"需要启用时再走导入、转换或手工配置。",
			},
		},
	}
}
