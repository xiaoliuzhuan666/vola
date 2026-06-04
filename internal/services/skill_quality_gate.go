package services

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/agi-bar/vola/internal/models"
	"github.com/google/uuid"
)

type SkillQualityStats struct {
	Passed         int `json:"passed"`
	Warnings       int `json:"warnings"`
	ManualRequired int `json:"manual_required"`
	Blocked        int `json:"blocked"`
}

type SkillQualityFinding struct {
	Code     string `json:"code"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	AgentID  string `json:"agent_id,omitempty"`
}

type skillQualityAccumulator struct {
	stats    SkillQualityStats
	findings []SkillQualityFinding
}

func (a *skillQualityAccumulator) add(f SkillQualityFinding) {
	f.Status = strings.TrimSpace(f.Status)
	if f.Status == "" {
		f.Status = "warning"
	}
	switch f.Status {
	case "passed":
		a.stats.Passed++
		return
	case "blocked":
		a.stats.Blocked++
	case "manual_required":
		a.stats.ManualRequired++
	default:
		f.Status = "warning"
		a.stats.Warnings++
	}
	if f.Severity == "" {
		switch f.Status {
		case "blocked":
			f.Severity = "error"
		case "manual_required":
			f.Severity = "warning"
		default:
			f.Severity = "info"
		}
	}
	a.findings = append(a.findings, f)
}

func (a *skillQualityAccumulator) status() string {
	switch {
	case a.stats.Blocked > 0:
		return "blocked"
	case a.stats.ManualRequired > 0:
		return "manual_required"
	case a.stats.Warnings > 0:
		return "warning"
	default:
		if a.stats.Passed == 0 {
			a.stats.Passed = 1
		}
		return "passed"
	}
}

func (s *SkillLearningService) evaluateSkillQuality(ctx context.Context, userID uuid.UUID, trustLevel int, bundlePath string, manifest *skillLearningManifest, assignedAgents []string) (string, SkillQualityStats, []SkillQualityFinding) {
	acc := &skillQualityAccumulator{}
	entries, err := s.skillEntryMap(ctx, userID, trustLevel, bundlePath)
	if err != nil {
		acc.add(SkillQualityFinding{
			Code:     "skill_tree_unreadable",
			Status:   "blocked",
			Category: "structure",
			Title:    "Skill 目录不可读",
			Message:  "无法读取 Skill 目录，学习任务不能确认文件完整性。",
			Path:     bundlePath,
		})
		status := acc.status()
		return status, acc.stats, acc.findings
	}

	skillContent := ""
	if entry, ok := entries["SKILL.md"]; ok {
		skillContent = entry.Content
		acc.add(SkillQualityFinding{Code: "entry_file_present", Status: "passed", Category: "structure"})
	} else {
		acc.add(SkillQualityFinding{
			Code:     "entry_file_missing",
			Status:   "blocked",
			Category: "structure",
			Title:    "缺少入口文件",
			Message:  "Skill 目录内没有 SKILL.md，目标 Agent 无法稳定加载。",
			Path:     path.Join(bundlePath, "SKILL.md"),
		})
	}

	if manifest == nil {
		acc.add(SkillQualityFinding{
			Code:     "manifest_missing",
			Status:   "warning",
			Category: "structure",
			Title:    "缺少 manifest",
			Message:  "缺少 manifest.vola.json，暂时不能自动确认脚本、依赖和外部引用。",
			Path:     path.Join(bundlePath, "manifest.vola.json"),
		})
		status := acc.status()
		return status, acc.stats, acc.findings
	}

	if manifest.EntryFile != "" {
		if _, ok := entries[normalizeSkillRelPath(manifest.EntryFile)]; !ok {
			acc.add(SkillQualityFinding{
				Code:     "manifest_entry_missing",
				Status:   "blocked",
				Category: "structure",
				Title:    "manifest 入口不存在",
				Message:  "manifest 指向的入口文件不存在，导出或同步后可能无法使用。",
				Path:     path.Join(bundlePath, manifest.EntryFile),
			})
		}
	}

	richAsset := false
	hasManualConfig := false
	for _, warning := range manifest.Warnings {
		switch warning.Code {
		case "secret_risk":
			acc.add(SkillQualityFinding{
				Code:     "secret_risk",
				Status:   "blocked",
				Category: "security",
				Title:    "疑似密钥文件",
				Message:  firstNonEmpty(warning.Message, "manifest 标记了疑似密钥文件，同步或分享前需要处理。"),
				Path:     path.Join(bundlePath, warning.Path),
			})
		case "large_file":
			acc.add(SkillQualityFinding{
				Code:     "large_file",
				Status:   "warning",
				Category: "asset",
				Title:    "大文件资产",
				Message:  firstNonEmpty(warning.Message, "manifest 标记了大文件资产，需要确认是否应该随 Skill 分发。"),
				Path:     path.Join(bundlePath, warning.Path),
			})
		}
	}

	for _, ref := range manifest.ExternalReferences {
		richAsset = true
		refPath := strings.TrimSpace(ref.Path)
		switch {
		case !ref.Included || ref.Status == "missing" || ref.Status == "requires_confirmation":
			acc.add(SkillQualityFinding{
				Code:     "external_reference_missing",
				Status:   "blocked",
				Category: "external_reference",
				Title:    "外部引用未确认",
				Message:  "Skill 引用了外部 Claude tools/plugins 文件，但当前包内没有可确认的副本。",
				Path:     refPath,
			})
		default:
			acc.add(SkillQualityFinding{
				Code:     "external_reference_included",
				Status:   "manual_required",
				Category: "external_reference",
				Title:    "外部引用已纳入",
				Message:  "外部 Claude tools/plugins 已随 Skill 保存，启用前仍需要人工确认用途和权限。",
				Path:     refPath,
			})
		}
	}

	for _, envName := range manifest.EnvVars {
		acc.add(SkillQualityFinding{
			Code:     "env_var_required",
			Status:   "warning",
			Category: "runtime",
			Title:    "需要环境变量",
			Message:  "运行这个 Skill 前需要确认环境变量 " + envName + " 是否已配置。",
		})
	}

	scriptFiles := 0
	dependencyFiles := 0
	for _, file := range manifest.Files {
		relPath := normalizeSkillRelPath(file.Path)
		if relPath == "" {
			continue
		}
		entry, exists := entries[relPath]
		if file.Included && !exists {
			acc.add(SkillQualityFinding{
				Code:     "manifest_file_missing",
				Status:   "blocked",
				Category: "structure",
				Title:    "manifest 文件缺失",
				Message:  "manifest 记录的文件在 Skill 目录中不存在。",
				Path:     path.Join(bundlePath, relPath),
			})
			continue
		}
		if isSkillMCPConfigPath(relPath, entry.Content) {
			hasManualConfig = true
			acc.add(SkillQualityFinding{
				Code:     "mcp_config",
				Status:   "manual_required",
				Category: "runtime_config",
				Title:    "MCP 配置需要审查",
				Message:  "检测到 MCP server 配置。Vola 会保存文件，但不会自动注册或启用。",
				Path:     path.Join(bundlePath, relPath),
			})
		}
		if isSkillHookPath(relPath) {
			hasManualConfig = true
			acc.add(SkillQualityFinding{
				Code:     "hook_config",
				Status:   "manual_required",
				Category: "runtime_config",
				Title:    "Hook 需要审查",
				Message:  "检测到 hook 文件。同步前需要确认触发时机、权限和副作用。",
				Path:     path.Join(bundlePath, relPath),
			})
		}
		if isSkillPluginPath(relPath) {
			hasManualConfig = true
			acc.add(SkillQualityFinding{
				Code:     "plugin_config",
				Status:   "manual_required",
				Category: "runtime_config",
				Title:    "Plugin 需要审查",
				Message:  "检测到 plugin 元数据或外部 plugin 文件。Vola 不会自动安装或启用 plugin。",
				Path:     path.Join(bundlePath, relPath),
			})
		}
		switch file.Kind {
		case "script":
			richAsset = true
			scriptFiles++
			evaluateSkillScript(acc, bundlePath, relPath, entry)
		case "dependency":
			richAsset = true
			dependencyFiles++
			evaluateSkillDependency(acc, bundlePath, relPath, entry)
		}
	}

	if manifest.Summary.Scripts > 0 && scriptFiles == 0 {
		richAsset = true
		acc.add(SkillQualityFinding{
			Code:     "script_summary_without_files",
			Status:   "warning",
			Category: "runtime",
			Title:    "脚本清单不完整",
			Message:  "manifest 统计里有脚本，但文件列表没有标出具体脚本。",
			Path:     path.Join(bundlePath, "manifest.vola.json"),
		})
	}
	if manifest.Summary.DependencyFiles > 0 && dependencyFiles == 0 {
		richAsset = true
		acc.add(SkillQualityFinding{
			Code:     "dependency_summary_without_files",
			Status:   "warning",
			Category: "runtime",
			Title:    "依赖清单不完整",
			Message:  "manifest 统计里有依赖文件，但文件列表没有标出具体依赖。",
			Path:     path.Join(bundlePath, "manifest.vola.json"),
		})
	}

	if richAsset || hasManualConfig {
		if hasSkillVerificationSection(skillContent) {
			acc.add(SkillQualityFinding{Code: "verification_section_present", Status: "passed", Category: "runtime"})
		} else {
			acc.add(SkillQualityFinding{
				Code:     "verification_steps_missing",
				Status:   "manual_required",
				Category: "runtime",
				Title:    "缺少验证步骤",
				Message:  "Skill 含脚本、依赖、外部引用或运行配置，但 SKILL.md 没有 Verification 说明。",
				Path:     path.Join(bundlePath, "SKILL.md"),
			})
		}
		if len(assignedAgents) > 0 {
			acc.add(SkillQualityFinding{
				Code:     "agent_smoke_test_required",
				Status:   "manual_required",
				Category: "agent_runtime",
				Title:    "需要目标 Agent 试用",
				Message:  "这个 Skill 已分配给 " + strings.Join(assignedAgents, " / ") + "，同步后需要用目标 Agent 做一次实际请求验证。",
				Path:     bundlePath,
			})
		}
	}

	status := acc.status()
	return status, acc.stats, uniqueSkillQualityFindings(acc.findings)
}

func (s *SkillLearningService) skillEntryMap(ctx context.Context, userID uuid.UUID, trustLevel int, bundlePath string) (map[string]models.FileTreeEntry, error) {
	snapshot, err := s.fileTree.Snapshot(ctx, userID, bundlePath, trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return nil, err
		}
		return nil, err
	}
	out := map[string]models.FileTreeEntry{}
	prefix := strings.TrimSuffix(bundlePath, "/") + "/"
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, prefix)
		rel = normalizeSkillRelPath(rel)
		if rel == "" || strings.HasPrefix(rel, "../") {
			continue
		}
		out[rel] = entry
	}
	return out, nil
}

func evaluateSkillScript(acc *skillQualityAccumulator, bundlePath, relPath string, entry models.FileTreeEntry) {
	content := strings.TrimSpace(entry.Content)
	if content == "" {
		acc.add(SkillQualityFinding{
			Code:     "script_empty",
			Status:   "blocked",
			Category: "runtime",
			Title:    "脚本为空",
			Message:  "脚本文件为空，无法作为可复用 Skill 资产。",
			Path:     path.Join(bundlePath, relPath),
		})
		return
	}
	runtime := detectScriptRuntime(relPath, content)
	if runtime == "" {
		acc.add(SkillQualityFinding{
			Code:     "script_runtime_unknown",
			Status:   "warning",
			Category: "runtime",
			Title:    "脚本运行时未知",
			Message:  "无法从扩展名或 shebang 判断脚本运行时，请在 Verification 中写明运行方式。",
			Path:     path.Join(bundlePath, relPath),
		})
		return
	}
	acc.add(SkillQualityFinding{Code: "script_runtime_detected", Status: "passed", Category: "runtime"})
}

func evaluateSkillDependency(acc *skillQualityAccumulator, bundlePath, relPath string, entry models.FileTreeEntry) {
	content := strings.TrimSpace(entry.Content)
	if content == "" {
		acc.add(SkillQualityFinding{
			Code:     "dependency_empty",
			Status:   "warning",
			Category: "runtime",
			Title:    "依赖文件为空",
			Message:  "依赖文件为空，需要确认是否仍然有意义。",
			Path:     path.Join(bundlePath, relPath),
		})
		return
	}
	if path.Base(relPath) == "package.json" {
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(content), &payload); err != nil {
			acc.add(SkillQualityFinding{
				Code:     "dependency_package_json_invalid",
				Status:   "blocked",
				Category: "runtime",
				Title:    "package.json 无法解析",
				Message:  "package.json 不是合法 JSON，目标环境无法可靠安装依赖。",
				Path:     path.Join(bundlePath, relPath),
			})
			return
		}
	}
	acc.add(SkillQualityFinding{Code: "dependency_file_readable", Status: "passed", Category: "runtime"})
}

func normalizeSkillRelPath(value string) string {
	clean := path.Clean(strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/"))
	if clean == "." || clean == "/" {
		return ""
	}
	return clean
}

func isSkillMCPConfigPath(relPath, content string) bool {
	base := strings.ToLower(path.Base(relPath))
	if base == "mcp.json" || base == ".mcp.json" {
		return true
	}
	return strings.Contains(content, `"mcpServers"`)
}

func isSkillHookPath(relPath string) bool {
	clean := strings.ToLower(normalizeSkillRelPath(relPath))
	return strings.HasPrefix(clean, "hooks/") || strings.Contains(clean, "/hooks/")
}

func isSkillPluginPath(relPath string) bool {
	clean := strings.ToLower(normalizeSkillRelPath(relPath))
	return strings.HasPrefix(clean, ".codex-plugin/") ||
		strings.HasPrefix(clean, "external/claude-plugins/") ||
		strings.Contains(clean, "/plugin.json")
}

func detectScriptRuntime(relPath, content string) string {
	ext := strings.ToLower(path.Ext(relPath))
	switch ext {
	case ".py":
		return "python"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".js", ".mjs", ".cjs":
		return "node"
	case ".ts", ".tsx":
		return "typescript"
	case ".rb":
		return "ruby"
	case ".pl":
		return "perl"
	}
	firstLine := strings.TrimSpace(strings.SplitN(content, "\n", 2)[0])
	if strings.HasPrefix(firstLine, "#!") {
		return strings.TrimSpace(strings.TrimPrefix(firstLine, "#!"))
	}
	return ""
}

func hasSkillVerificationSection(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "\n## verification") ||
		strings.Contains(lower, "\n# verification") ||
		strings.Contains(content, "\n## 验证") ||
		strings.Contains(content, "\n# 验证")
}

func uniqueSkillQualityFindings(findings []SkillQualityFinding) []SkillQualityFinding {
	if len(findings) == 0 {
		return findings
	}
	seen := map[string]struct{}{}
	out := make([]SkillQualityFinding, 0, len(findings))
	for _, finding := range findings {
		key := strings.Join([]string{finding.Code, finding.Status, finding.Path, finding.AgentID}, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}
