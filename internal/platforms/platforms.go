package platforms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/runtimecfg"
	"github.com/agi-bar/vola/internal/storage/sqlite"
	"github.com/agi-bar/vola/internal/systemskills"
)

const LocalServerName = "vola-local"

var lookPath = exec.LookPath

const (
	volaSkillName           = "vola"
	managedMarkerFile       = ".vola-managed.json"
	managedCommandHeader    = "<!-- vola-managed:command -->"
	codexEntrypointDir      = "~/.agents/skills/vola"
	claudeEntrypointDir     = "~/.claude/skills/vola"
	claudeCommandEntrypoint = "~/.claude/commands/vola.md"
)

type Source = sqlite.Source

type Status struct {
	ID                  string
	DisplayName         string
	Installed           bool
	Connected           bool
	MCPInstalled        bool
	BinaryPath          string
	ConfigPath          string
	DaemonTarget        string
	EntrypointInstalled bool
	EntrypointType      string
	EntrypointPath      string
	ChatUsage           []string
	AgentMediated       string
	SupportedDomains    []string
	Sources             []Source
}

type Adapter interface {
	ID() string
	DisplayName() string
	Aliases() []string
	SupportedDomains() []string
	EntrypointType() string
	EntrypointPath() string
	ChatUsage() []string
	AgentMediatedSupport() string
	Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status
	Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error)
	Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error
	DiscoverSources() []Source
}

func Registry() []Adapter {
	return []Adapter{
		&claudeAdapter{},
		&codexAdapter{},
		&geminiAdapter{},
		&cursorAdapter{},
		&traeAdapter{},
		&codebuddyAdapter{},
		&workbuddyAdapter{},
	}
}

func Resolve(name string) (Adapter, error) {
	requested := strings.TrimSpace(strings.ToLower(name))
	for _, adapter := range Registry() {
		if adapter.ID() == requested {
			return adapter, nil
		}
		for _, alias := range adapter.Aliases() {
			if alias == requested {
				return adapter, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown platform %q", name)
}

func AllStatuses(cfg *runtimecfg.CLIConfig, daemonURL string) []Status {
	out := make([]Status, 0, len(Registry()))
	for _, adapter := range Registry() {
		out = append(out, adapter.Detect(cfg, daemonURL))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ImportIntoLocalHub(ctx context.Context, cfg *runtimecfg.CLIConfig, platform string) (*sqlite.ImportResult, error) {
	adapter, err := Resolve(platform)
	if err != nil {
		return nil, err
	}
	result, _, _, err := importPlatformData(ctx, cfg, adapter.ID(), adapter.DiscoverSources(), nil)
	return result, err
}

func ExportFromLocalHub(ctx context.Context, cfg *runtimecfg.CLIConfig, platform, outputRoot string) (*sqlite.ExportResult, error) {
	adapter, err := Resolve(platform)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(outputRoot) == "" {
		outputRoot, err = defaultExportRoot(adapter.ID())
		if err != nil {
			return nil, err
		}
	} else {
		outputRoot, err = resolveLocalPath(outputRoot)
		if err != nil {
			return nil, err
		}
	}
	var result sqlite.ExportResult
	_, err = localPlatformAPIPostJSON(ctx, cfg.Local.PublicBaseURL, cfg.Local.OwnerToken, "/agent/local/platform/export", map[string]string{
		"platform":    adapter.ID(),
		"output_root": outputRoot,
	}, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func EnsureConnection(ctx context.Context, cfg *runtimecfg.CLIConfig, platform, executable, daemonURL string) (runtimecfg.LocalConnection, error) {
	adapter, err := Resolve(platform)
	if err != nil {
		return runtimecfg.LocalConnection{}, err
	}
	if cfg.Local.Connections == nil {
		cfg.Local.Connections = map[string]runtimecfg.LocalConnection{}
	}

	connection := cfg.Local.Connections[adapter.ID()]
	if strings.TrimSpace(connection.Token) == "" {
		var tokenResp models.CreateTokenResponse
		_, err := localPlatformAPIPostJSON(ctx, daemonURL, cfg.Local.OwnerToken, "/agent/local/platform-token", map[string]interface{}{
			"platform":    adapter.ID(),
			"trust_level": models.TrustLevelWork,
		}, &tokenResp)
		if err != nil {
			return runtimecfg.LocalConnection{}, err
		}
		connection.Token = tokenResp.Token
		connection.TokenID = tokenResp.ScopedToken.ID.String()
		connection.TokenPrefix = tokenResp.TokenPrefix
		connection.Scopes = tokenResp.ScopedToken.Scopes
		connection.MaxTrustLevel = tokenResp.ScopedToken.MaxTrustLevel
		connection.ConnectedAt = time.Now().UTC().Format(time.RFC3339)
	}
	connection.LastPlatformURL = strings.TrimRight(daemonURL, "/") + "/mcp"
	updated, err := adapter.Connect(ctx, cfg, executable, daemonURL, connection)
	if err != nil {
		return runtimecfg.LocalConnection{}, err
	}
	cfg.Local.Connections[adapter.ID()] = updated
	return updated, nil
}

func RefreshConnection(ctx context.Context, cfg *runtimecfg.CLIConfig, platform, executable, daemonURL string) (runtimecfg.LocalConnection, error) {
	adapter, err := Resolve(platform)
	if err != nil {
		return runtimecfg.LocalConnection{}, err
	}
	if cfg.Local.Connections == nil {
		cfg.Local.Connections = map[string]runtimecfg.LocalConnection{}
	}
	connection := cfg.Local.Connections[adapter.ID()]
	if strings.TrimSpace(connection.Token) == "" {
		return runtimecfg.LocalConnection{}, fmt.Errorf("platform %q is not connected", platform)
	}
	connection.LastPlatformURL = strings.TrimRight(daemonURL, "/") + "/mcp"
	updated, err := adapter.Connect(ctx, cfg, executable, daemonURL, connection)
	if err != nil {
		return runtimecfg.LocalConnection{}, err
	}
	cfg.Local.Connections[adapter.ID()] = updated
	return updated, nil
}

func Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig, platform string) error {
	adapter, err := Resolve(platform)
	if err != nil {
		return err
	}
	connection, ok := cfg.Local.Connections[adapter.ID()]
	if ok && strings.TrimSpace(connection.TokenID) != "" {
		_, _ = localPlatformAPIDelete(ctx, cfg.Local.PublicBaseURL, cfg.Local.OwnerToken, "/agent/local/platform-token/"+url.PathEscape(connection.TokenID), nil)
	}
	if err := adapter.Disconnect(ctx, cfg); err != nil {
		return err
	}
	delete(cfg.Local.Connections, adapter.ID())
	return nil
}

type baseAdapter struct {
	id               string
	displayName      string
	aliases          []string
	command          string
	configPath       string
	entrypointType   string
	entrypointPath   string
	chatUsage        []string
	agentMediated    string
	supportedDomains []string
	sources          []Source
}

func (b *baseAdapter) ID() string                 { return b.id }
func (b *baseAdapter) DisplayName() string        { return b.displayName }
func (b *baseAdapter) Aliases() []string          { return b.aliases }
func (b *baseAdapter) SupportedDomains() []string { return b.supportedDomains }
func (b *baseAdapter) EntrypointType() string     { return b.entrypointType }
func (b *baseAdapter) EntrypointPath() string     { return expandUser(b.entrypointPath) }
func (b *baseAdapter) ChatUsage() []string        { return append([]string{}, b.chatUsage...) }
func (b *baseAdapter) AgentMediatedSupport() string {
	if strings.TrimSpace(b.agentMediated) == "" {
		return "planned"
	}
	return b.agentMediated
}
func (b *baseAdapter) DiscoverSources() []Source { return existingSources(b.sources) }
func (b *baseAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	binPath, _ := lookPath(b.command)
	connection, connected := cfg.Local.Connections[b.id]
	entrypointType := b.entrypointType
	if strings.TrimSpace(connection.EntrypointType) != "" {
		entrypointType = connection.EntrypointType
	}
	entrypointPath := strings.TrimSpace(connection.EntrypointPath)
	if entrypointPath == "" {
		entrypointPath = expandUser(b.entrypointPath)
	}
	entrypointInstalled := false
	if entrypointPath != "" {
		if _, err := os.Stat(entrypointPath); err == nil {
			entrypointInstalled = true
		}
	}
	chatUsage := append([]string{}, b.chatUsage...)
	if len(connection.ChatUsage) > 0 {
		chatUsage = append([]string{}, connection.ChatUsage...)
	}
	return Status{
		ID:                  b.id,
		DisplayName:         b.displayName,
		Installed:           binPath != "",
		Connected:           connected,
		MCPInstalled:        connected,
		BinaryPath:          binPath,
		ConfigPath:          expandUser(b.configPath),
		DaemonTarget:        strings.TrimRight(daemonURL, "/") + "/mcp",
		EntrypointInstalled: entrypointInstalled,
		EntrypointType:      entrypointType,
		EntrypointPath:      entrypointPath,
		ChatUsage:           chatUsage,
		AgentMediated:       b.AgentMediatedSupport(),
		SupportedDomains:    append([]string{}, b.supportedDomains...),
		Sources:             existingSources(b.sources),
	}
}

type claudeAdapter struct{ baseAdapter }
type codexAdapter struct{ baseAdapter }
type geminiAdapter struct{ baseAdapter }
type cursorAdapter struct{ baseAdapter }
type traeAdapter struct{ baseAdapter }
type codebuddyAdapter struct{ baseAdapter }
type workbuddyAdapter struct{ baseAdapter }

func newBaseAdapter(id, displayName, command, configPath, entrypointType, entrypointPath string, aliases []string, chatUsage, domains []string, sources []Source, agentMediated string) baseAdapter {
	return baseAdapter{
		id:               id,
		displayName:      displayName,
		aliases:          aliases,
		command:          command,
		configPath:       configPath,
		entrypointType:   entrypointType,
		entrypointPath:   entrypointPath,
		chatUsage:        append([]string{}, chatUsage...),
		agentMediated:    agentMediated,
		supportedDomains: domains,
		sources:          sources,
	}
}

func (a *claudeAdapter) init() *claudeAdapter {
	if a.id == "" {
		*a = claudeAdapter{newBaseAdapter(
			"claude-code",
			"Claude Code",
			"claude",
			"~/.claude.json",
			"command",
			claudeCommandEntrypoint,
			[]string{"claude"},
			[]string{"/vola ls", "/vola read profile/preferences", "/vola write memory \"Remember this\"", "/vola create project demo", "/vola import claude", "/vola token create --kind sync --purpose backup", "/vola status"},
			[]string{"connections", "profile", "memory", "skills", "projects", "prompts", "tools", "automations", "archives", "secrets"},
			claudeSources(),
			"supported",
		)}
	}
	return a
}

func (a *claudeAdapter) ID() string                 { return a.init().baseAdapter.ID() }
func (a *claudeAdapter) DisplayName() string        { return a.init().baseAdapter.DisplayName() }
func (a *claudeAdapter) Aliases() []string          { return a.init().baseAdapter.Aliases() }
func (a *claudeAdapter) SupportedDomains() []string { return a.init().baseAdapter.SupportedDomains() }
func (a *claudeAdapter) DiscoverSources() []Source  { return existingSources(claudeSources()) }
func (a *claudeAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	status := a.init().baseAdapter.Detect(cfg, daemonURL)
	status.Sources = existingSources(claudeSources())
	return status
}
func (a *claudeAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	if _, err := lookPath("claude"); err != nil {
		return connection, err
	}
	_ = run(ctx, "claude", "mcp", "remove", "--scope", "user", LocalServerName)
	if err := run(ctx, "claude", "mcp", "add", "--scope", "user", "--transport", "http", LocalServerName, strings.TrimRight(daemonURL, "/")+"/mcp", "--header", "Authorization: Bearer "+connection.Token); err != nil {
		return connection, err
	}
	// 拉取团队 MCP 并动态装配
	if mcps, err := fetchTeamMcps(ctx, daemonURL, connection.Token); err == nil {
		for _, mcp := range mcps {
			serverKey := "team-mcp-" + mcp.Slug
			_ = run(ctx, "claude", "mcp", "remove", "--scope", "user", serverKey)
			if mcp.Transport == "stdio" {
				args := []string{"claude", "mcp", "add", "--scope", "user", "--transport", "stdio", serverKey, "--", mcp.Command}
				args = append(args, mcp.Args...)
				_ = run(ctx, args[0], args[1:]...)
			} else if mcp.Transport == "http" {
				args := []string{"claude", "mcp", "add", "--scope", "user", "--transport", "http", serverKey, mcp.URL}
				for k, v := range mcp.Headers {
					args = append(args, "--header", k+": "+v)
				}
				_ = run(ctx, args[0], args[1:]...)
			}
		}
	}
	skillPath, managedPaths, err := installManagedSkill(expandUser(claudeEntrypointDir), "claude-code")
	if err != nil {
		return connection, err
	}
	commandPath, err := installManagedClaudeCommand(expandUser(claudeCommandEntrypoint))
	if err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = expandUser("~/.claude.json")
	connection.EntrypointType = "command"
	connection.EntrypointPath = commandPath
	connection.ManagedPaths = append(managedPaths, commandPath)
	connection.ChatUsage = []string{"/vola ls", "/vola read profile/preferences", "/vola write memory \"Remember this\"", "/vola create project demo", "/vola import claude", "/vola token create --kind sync --purpose backup", "/vola status"}
	_ = skillPath
	return connection, nil
}
func (a *claudeAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	for _, target := range managedPathsForPlatform(cfg, a.ID(), expandUser(claudeEntrypointDir), expandUser(claudeCommandEntrypoint)) {
		if err := removeManagedPath(target); err != nil {
			return err
		}
	}
	if mcps, err := fetchTeamMcps(ctx, cfg.Local.PublicBaseURL, cfg.Local.OwnerToken); err == nil {
		for _, mcp := range mcps {
			_ = run(ctx, "claude", "mcp", "remove", "--scope", "user", "team-mcp-"+mcp.Slug)
		}
	}
	return run(ctx, "claude", "mcp", "remove", "--scope", "user", LocalServerName)
}

func (a *codexAdapter) init() *codexAdapter {
	if a.id == "" {
		*a = codexAdapter{newBaseAdapter(
			"codex",
			"Codex CLI",
			"codex",
			"~/.codex/config.toml",
			"skill",
			codexEntrypointDir,
			nil,
			[]string{"$vola ls", "$vola read profile/preferences", "$vola write memory \"Remember this\"", "$vola create project demo", "$vola import codex", "$vola token create --kind sync --purpose backup", "$vola status"},
			[]string{"connections", "skills", "profile", "memory", "projects", "automations", "tools", "archives"},
			[]Source{
				{Domain: "profile", Label: "config.toml", Path: expandUser("~/.codex/config.toml")},
				{Domain: "profile", Label: "AGENTS.md", Path: expandUser("~/.codex/AGENTS.md")},
				{Domain: "profile", Label: "rules", Path: expandUser("~/.codex/rules"), IsDir: true},
				{Domain: "memory", Label: "memories", Path: expandUser("~/.codex/memories"), IsDir: true},
				{Domain: "skills", Label: "skills", Path: expandUser("~/.agents/skills"), IsDir: true},
				{Domain: "skills", Label: "bundled_skills", Path: expandUser("~/.codex/skills"), IsDir: true},
				{Domain: "automations", Label: "automations", Path: expandUser("~/.codex/automations"), IsDir: true},
				{Domain: "connections", Label: "auth.json", Path: expandUser("~/.codex/auth.json")},
				{Domain: "projects", Label: "sessions", Path: expandUser("~/.codex/sessions"), IsDir: true},
				{Domain: "projects", Label: "history.jsonl", Path: expandUser("~/.codex/history.jsonl")},
				{Domain: "projects", Label: "session_index.jsonl", Path: expandUser("~/.codex/session_index.jsonl")},
				{Domain: "archives", Label: "archived_sessions", Path: expandUser("~/.codex/archived_sessions"), IsDir: true},
			},
			"supported",
		)}
	}
	return a
}

func (a *codexAdapter) ID() string                 { return a.init().baseAdapter.ID() }
func (a *codexAdapter) DisplayName() string        { return a.init().baseAdapter.DisplayName() }
func (a *codexAdapter) Aliases() []string          { return a.init().baseAdapter.Aliases() }
func (a *codexAdapter) SupportedDomains() []string { return a.init().baseAdapter.SupportedDomains() }
func (a *codexAdapter) DiscoverSources() []Source  { return a.init().baseAdapter.DiscoverSources() }
func (a *codexAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *codexAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	if _, err := lookPath("codex"); err != nil {
		return connection, err
	}
	_ = run(ctx, "codex", "mcp", "remove", LocalServerName)
	if err := run(ctx,
		"codex", "mcp", "add", LocalServerName,
		"--env", "VOLA_TOKEN="+connection.Token,
		"--env", "VOLA_STORAGE="+cfg.Local.Storage,
		"--env", "VOLA_SQLITE_PATH="+cfg.Local.SQLitePath,
		"--env", "DATABASE_URL="+cfg.Local.DatabaseURL,
		"--env", "JWT_SECRET="+cfg.Local.JWTSecret,
		"--env", "VAULT_MASTER_KEY="+cfg.Local.VaultMasterKey,
		"--env", "PUBLIC_BASE_URL="+cfg.Local.PublicBaseURL,
		"--",
		executable, "mcp", "stdio", "--storage", cfg.Local.Storage, "--sqlite-path", cfg.Local.SQLitePath, "--token-env", "VOLA_TOKEN",
	); err != nil {
		return connection, err
	}
	// 拉取团队 MCP 并动态装配
	if mcps, err := fetchTeamMcps(ctx, daemonURL, connection.Token); err == nil {
		for _, mcp := range mcps {
			serverKey := "team-mcp-" + mcp.Slug
			_ = run(ctx, "codex", "mcp", "remove", serverKey)
			if mcp.Transport == "stdio" {
				args := []string{"codex", "mcp", "add", serverKey}
				for k, v := range mcp.Env {
					args = append(args, "--env", k+"="+v)
				}
				args = append(args, "--", mcp.Command)
				args = append(args, mcp.Args...)
				_ = run(ctx, args[0], args[1:]...)
			} else if mcp.Transport == "http" {
				args := []string{"codex", "mcp", "add", serverKey, "--url", mcp.URL}
				for k, v := range mcp.Headers {
					args = append(args, "--header", k+": "+v)
				}
				_ = run(ctx, args[0], args[1:]...)
			}
		}
	}
	skillPath, managedPaths, err := installManagedSkill(expandUser(codexEntrypointDir), "codex")
	if err != nil {
		return connection, err
	}
	connection.Transport = "stdio"
	connection.ConfigPath = expandUser("~/.codex/config.toml")
	connection.EntrypointType = "skill"
	connection.EntrypointPath = skillPath
	connection.ManagedPaths = managedPaths
	connection.ChatUsage = []string{"$vola ls", "$vola read profile/preferences", "$vola write memory \"Remember this\"", "$vola create project demo", "$vola import codex", "$vola token create --kind sync --purpose backup", "$vola status"}
	return connection, nil
}
func (a *codexAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	for _, target := range managedPathsForPlatform(cfg, a.ID(), expandUser(codexEntrypointDir)) {
		if err := removeManagedPath(target); err != nil {
			return err
		}
	}
	if mcps, err := fetchTeamMcps(ctx, cfg.Local.PublicBaseURL, cfg.Local.OwnerToken); err == nil {
		for _, mcp := range mcps {
			_ = run(ctx, "codex", "mcp", "remove", "team-mcp-"+mcp.Slug)
		}
	}
	return run(ctx, "codex", "mcp", "remove", LocalServerName)
}

func (a *geminiAdapter) init() *geminiAdapter {
	if a.id == "" {
		*a = geminiAdapter{newBaseAdapter(
			"gemini-cli",
			"Gemini CLI",
			"gemini",
			"~/.gemini/settings.json",
			"",
			"",
			[]string{"gemini"},
			nil,
			[]string{"connections", "profile", "projects", "prompts", "archives"},
			[]Source{
				{Domain: "profile", Label: "settings.json", Path: expandUser("~/.gemini/settings.json")},
				{Domain: "connections", Label: "mcp-oauth-tokens.json", Path: expandUser("~/.gemini/mcp-oauth-tokens.json")},
				{Domain: "projects", Label: "history", Path: expandUser("~/.gemini/history"), IsDir: true},
				{Domain: "projects", Label: "projects.json", Path: expandUser("~/.gemini/projects.json")},
				{Domain: "archives", Label: "tmp", Path: expandUser("~/.gemini/tmp"), IsDir: true},
			},
			"planned",
		)}
	}
	return a
}

func (a *geminiAdapter) ID() string                 { return a.init().baseAdapter.ID() }
func (a *geminiAdapter) DisplayName() string        { return a.init().baseAdapter.DisplayName() }
func (a *geminiAdapter) Aliases() []string          { return a.init().baseAdapter.Aliases() }
func (a *geminiAdapter) SupportedDomains() []string { return a.init().baseAdapter.SupportedDomains() }
func (a *geminiAdapter) DiscoverSources() []Source  { return a.init().baseAdapter.DiscoverSources() }
func (a *geminiAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *geminiAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	if _, err := lookPath("gemini"); err != nil {
		return connection, err
	}
	_ = run(ctx, "gemini", "mcp", "remove", "--scope", "user", LocalServerName)
	if err := run(ctx, "gemini", "mcp", "add", "--scope", "user", "--transport", "http", LocalServerName, strings.TrimRight(daemonURL, "/")+"/mcp", "--header", "Authorization: Bearer "+connection.Token); err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = expandUser("~/.gemini/settings.json")
	return connection, nil
}
func (a *geminiAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	return run(ctx, "gemini", "mcp", "remove", "--scope", "user", LocalServerName)
}

func (a *cursorAdapter) init() *cursorAdapter {
	if a.id == "" {
		*a = cursorAdapter{newBaseAdapter(
			"cursor-agent",
			"Cursor Agent",
			"cursor-agent",
			"~/.cursor/mcp.json",
			"",
			"",
			[]string{"cursor"},
			nil,
			[]string{"connections", "skills", "projects", "prompts", "archives"},
			[]Source{
				{Domain: "connections", Label: "mcp.json", Path: expandUser("~/.cursor/mcp.json")},
				{Domain: "skills", Label: "skills-cursor", Path: expandUser("~/.cursor/skills-cursor"), IsDir: true},
				{Domain: "projects", Label: "projects", Path: expandUser("~/.cursor/projects"), IsDir: true},
				{Domain: "prompts", Label: "prompt_history.json", Path: expandUser("~/.cursor/prompt_history.json")},
				{Domain: "archives", Label: "chats", Path: expandUser("~/.cursor/chats"), IsDir: true},
			},
			"planned",
		)}
	}
	return a
}

func (a *cursorAdapter) ID() string                 { return a.init().baseAdapter.ID() }
func (a *cursorAdapter) DisplayName() string        { return a.init().baseAdapter.DisplayName() }
func (a *cursorAdapter) Aliases() []string          { return a.init().baseAdapter.Aliases() }
func (a *cursorAdapter) SupportedDomains() []string { return a.init().baseAdapter.SupportedDomains() }
func (a *cursorAdapter) DiscoverSources() []Source  { return a.init().baseAdapter.DiscoverSources() }
func (a *cursorAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *cursorAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	configPath := expandUser("~/.cursor/mcp.json")
	if err := connectMcpPlatform(ctx, daemonURL, connection, configPath); err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = configPath
	return connection, nil
}
func (a *cursorAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	configPath := expandUser("~/.cursor/mcp.json")
	return disconnectMcpPlatform(configPath)
}

// ---------------- Trae Adapter ----------------

func (a *traeAdapter) init() *traeAdapter {
	if a.id == "" {
		*a = traeAdapter{newBaseAdapter(
			"trae-agent",
			"Trae Agent",
			"trae-agent",
			"~/.trae/mcp.json",
			"",
			"",
			[]string{"trae"},
			nil,
			[]string{"connections", "skills", "projects", "prompts", "archives"},
			[]Source{
				{Domain: "connections", Label: "mcp.json", Path: expandUser("~/.trae/mcp.json")},
			},
			"planned",
		)}
	}
	return a
}

func (a *traeAdapter) ID() string                 { return a.init().baseAdapter.ID() }
func (a *traeAdapter) DisplayName() string        { return a.init().baseAdapter.DisplayName() }
func (a *traeAdapter) Aliases() []string          { return a.init().baseAdapter.Aliases() }
func (a *traeAdapter) SupportedDomains() []string { return a.init().baseAdapter.SupportedDomains() }
func (a *traeAdapter) DiscoverSources() []Source  { return a.init().baseAdapter.DiscoverSources() }
func (a *traeAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *traeAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	configPath := expandUser("~/.trae/mcp.json")
	if err := connectMcpPlatform(ctx, daemonURL, connection, configPath); err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = configPath
	return connection, nil
}
func (a *traeAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	configPath := expandUser("~/.trae/mcp.json")
	return disconnectMcpPlatform(configPath)
}

// ---------------- Codebuddy Adapter ----------------

func (a *codebuddyAdapter) init() *codebuddyAdapter {
	if a.id == "" {
		*a = codebuddyAdapter{newBaseAdapter(
			"codebuddy-agent",
			"Codebuddy Agent",
			"codebuddy-agent",
			"~/.codebuddy/mcp.json",
			"",
			"",
			[]string{"codebuddy"},
			nil,
			[]string{"connections", "skills", "projects", "prompts", "archives"},
			[]Source{
				{Domain: "connections", Label: "mcp.json", Path: expandUser("~/.codebuddy/mcp.json")},
			},
			"planned",
		)}
	}
	return a
}

func (a *codebuddyAdapter) ID() string          { return a.init().baseAdapter.ID() }
func (a *codebuddyAdapter) DisplayName() string { return a.init().baseAdapter.DisplayName() }
func (a *codebuddyAdapter) Aliases() []string   { return a.init().baseAdapter.Aliases() }
func (a *codebuddyAdapter) SupportedDomains() []string {
	return a.init().baseAdapter.SupportedDomains()
}
func (a *codebuddyAdapter) DiscoverSources() []Source { return a.init().baseAdapter.DiscoverSources() }
func (a *codebuddyAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *codebuddyAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	configPath := expandUser("~/.codebuddy/mcp.json")
	if err := connectMcpPlatform(ctx, daemonURL, connection, configPath); err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = configPath
	return connection, nil
}
func (a *codebuddyAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	configPath := expandUser("~/.codebuddy/mcp.json")
	return disconnectMcpPlatform(configPath)
}

// ---------------- Workbuddy Adapter ----------------

func (a *workbuddyAdapter) init() *workbuddyAdapter {
	if a.id == "" {
		*a = workbuddyAdapter{newBaseAdapter(
			"workbuddy-agent",
			"Workbuddy Agent",
			"workbuddy-agent",
			"~/.workbuddy/mcp.json",
			"",
			"",
			[]string{"workbuddy"},
			nil,
			[]string{"connections", "skills", "projects", "prompts", "archives"},
			[]Source{
				{Domain: "connections", Label: "mcp.json", Path: expandUser("~/.workbuddy/mcp.json")},
			},
			"planned",
		)}
	}
	return a
}

func (a *workbuddyAdapter) ID() string          { return a.init().baseAdapter.ID() }
func (a *workbuddyAdapter) DisplayName() string { return a.init().baseAdapter.DisplayName() }
func (a *workbuddyAdapter) Aliases() []string   { return a.init().baseAdapter.Aliases() }
func (a *workbuddyAdapter) SupportedDomains() []string {
	return a.init().baseAdapter.SupportedDomains()
}
func (a *workbuddyAdapter) DiscoverSources() []Source { return a.init().baseAdapter.DiscoverSources() }
func (a *workbuddyAdapter) Detect(cfg *runtimecfg.CLIConfig, daemonURL string) Status {
	return a.init().baseAdapter.Detect(cfg, daemonURL)
}
func (a *workbuddyAdapter) Connect(ctx context.Context, cfg *runtimecfg.CLIConfig, executable, daemonURL string, connection runtimecfg.LocalConnection) (runtimecfg.LocalConnection, error) {
	configPath := expandUser("~/.workbuddy/mcp.json")
	if err := connectMcpPlatform(ctx, daemonURL, connection, configPath); err != nil {
		return connection, err
	}
	connection.Transport = "http"
	connection.ConfigPath = configPath
	return connection, nil
}
func (a *workbuddyAdapter) Disconnect(ctx context.Context, cfg *runtimecfg.CLIConfig) error {
	configPath := expandUser("~/.workbuddy/mcp.json")
	return disconnectMcpPlatform(configPath)
}

func claudeSources() []Source {
	return []Source{
		{Domain: "connections", Label: "claude.json", Path: expandUser("~/.claude.json")},
		{Domain: "profile", Label: "CLAUDE.md", Path: expandUser("~/.claude/CLAUDE.md")},
		{Domain: "profile", Label: "CLAUDE.local.md", Path: expandUser("~/.claude/CLAUDE.local.md")},
		{Domain: "profile", Label: "settings.json", Path: expandUser("~/.claude/settings.json")},
		{Domain: "profile", Label: "settings.local.json", Path: expandUser("~/.claude/settings.local.json")},
		{Domain: "memory", Label: "agent-memory", Path: expandUser("~/.claude/agent-memory"), IsDir: true},
		{Domain: "memory", Label: "projects", Path: expandUser("~/.claude/projects"), IsDir: true},
		{Domain: "skills", Label: "skills", Path: expandUser("~/.claude/skills"), IsDir: true},
		{Domain: "skills", Label: "agents", Path: expandUser("~/.claude/agents"), IsDir: true},
		{Domain: "skills", Label: "commands", Path: expandUser("~/.claude/commands"), IsDir: true},
		{Domain: "skills", Label: "rules", Path: expandUser("~/.claude/rules"), IsDir: true},
		{Domain: "tools", Label: "plugins", Path: expandUser("~/.claude/plugins"), IsDir: true},
		{Domain: "prompts", Label: "history", Path: expandUser("~/.claude/history.jsonl")},
		{Domain: "automations", Label: "scheduled-tasks", Path: expandUser("~/.claude/scheduled-tasks"), IsDir: true},
		{Domain: "archives", Label: "output-styles", Path: expandUser("~/.claude/output-styles"), IsDir: true},
		{Domain: "archives", Label: "hooks", Path: expandUser("~/.claude/hooks"), IsDir: true},
	}
}

func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, trimmed)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func existingSources(sources []Source) []Source {
	out := make([]Source, 0, len(sources))
	for _, source := range sources {
		if _, err := os.Stat(source.Path); err == nil {
			out = append(out, source)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain == out[j].Domain {
			return out[i].Path < out[j].Path
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

func expandUser(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

type managedInstallMarker struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Platform    string `json:"platform"`
	GeneratedAt string `json:"generated_at"`
}

func managedPathsForPlatform(cfg *runtimecfg.CLIConfig, platform string, defaults ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(defaults))
	if cfg != nil {
		if connection, ok := cfg.Local.Connections[platform]; ok {
			for _, candidate := range connection.ManagedPaths {
				candidate = strings.TrimSpace(candidate)
				if candidate == "" {
					continue
				}
				if _, ok := seen[candidate]; ok {
					continue
				}
				seen[candidate] = struct{}{}
				out = append(out, candidate)
			}
		}
	}
	for _, candidate := range defaults {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func installManagedSkill(targetDir, platform string) (string, []string, error) {
	files, err := systemskills.ExportSkillFiles(volaSkillName)
	if err != nil {
		return "", nil, err
	}
	if info, err := os.Stat(targetDir); err == nil {
		if !info.IsDir() {
			return "", nil, fmt.Errorf("%s exists and is not a directory", targetDir)
		}
		if !isManagedSkillDir(targetDir) {
			return "", nil, fmt.Errorf("%s already exists and is not managed by Vola", targetDir)
		}
		if err := os.RemoveAll(targetDir); err != nil {
			return "", nil, err
		}
	} else if !os.IsNotExist(err) {
		return "", nil, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", nil, err
	}
	for relPath, content := range files {
		target := filepath.Join(targetDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", nil, err
		}
	}
	markerData, _ := json.MarshalIndent(managedInstallMarker{
		Name:        volaSkillName,
		Kind:        "skill",
		Platform:    platform,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err := os.WriteFile(filepath.Join(targetDir, managedMarkerFile), append(markerData, '\n'), 0o644); err != nil {
		return "", nil, err
	}
	return targetDir, []string{targetDir}, nil
}

func installManagedClaudeCommand(targetPath string) (string, error) {
	if data, err := os.ReadFile(targetPath); err == nil {
		if !strings.HasPrefix(string(data), managedCommandHeader) {
			return "", fmt.Errorf("%s already exists and is not managed by Vola", targetPath)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	content := strings.Join([]string{
		managedCommandHeader,
		"---",
		"description: Route `/vola <subcommand>` through the installed Vola skill and MCP surface.",
		"---",
		"",
		"Use the installed `vola` skill at `~/.claude/skills/vola/SKILL.md`.",
		"",
		"Treat the first argument after `/vola` as the subcommand.",
		"Supported subcommands: `ls`, `read`, `write`, `search`, `create`, `log`, `import`, `token`, `stats`, `export`, `status`, `help`.",
		"Examples: `/vola ls`, `/vola read profile/preferences`, `/vola import claude`, `/vola status`.",
		"Use `/vola help` or `/vola help import` when the user needs guidance on the command surface.",
		"Use the Git Mirror page in Vola when the user wants a repo-backed mirror of the Hub.",
		"",
		"1. Read `~/.claude/skills/vola/SKILL.md`.",
		"2. Read the matching command document under `~/.claude/skills/vola/commands/`.",
		"3. Use Vola MCP tools for all Hub reads and writes.",
		"4. Use `~/.claude/skills/vola/references/platforms/claude.md` for Claude-specific routing.",
		"",
		"If no subcommand is provided, treat it as `help`.",
	}, "\n")
	if err := os.WriteFile(targetPath, []byte(content+"\n"), 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func isManagedSkillDir(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, managedMarkerFile))
	if err != nil {
		return false
	}
	var marker managedInstallMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return false
	}
	return marker.Name == volaSkillName && marker.Kind == "skill"
}

func removeManagedPath(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		if !isManagedSkillDir(target) {
			return fmt.Errorf("%s is not managed by Vola", target)
		}
		return os.RemoveAll(target)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(data), managedCommandHeader) {
		return fmt.Errorf("%s is not managed by Vola", target)
	}
	return os.Remove(target)
}

type teamMcpConfig struct {
	Slug      string            `json:"slug"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Status    string            `json:"status"`
	Tags      []string          `json:"tags,omitempty"`
}

func loadLocalVolarc() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	volarcPath := filepath.Join(cwd, ".volarc")

	// Resolve absolute physical path and evaluate symlinks
	absPath, err := filepath.Abs(volarcPath)
	if err != nil {
		return nil
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return nil
	}

	// Sandboxing check: block reading symlinks leading to sensitive system paths
	lowerPath := strings.ToLower(realPath)
	sensitivePrefixes := []string{"/etc/", "/var/", "/system/", "/private/", "/bin/", "/sbin/", "/usr/"}
	for _, pref := range sensitivePrefixes {
		if strings.HasPrefix(lowerPath, pref) {
			return nil
		}
	}

	data, err := os.ReadFile(realPath)
	if err != nil {
		return nil
	}
	var config struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(data, &config); err == nil {
		return config.Tags
	}
	return nil
}

func matchTags(mcpTags []string, filterTags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	for _, ft := range filterTags {
		for _, mt := range mcpTags {
			if strings.EqualFold(ft, mt) {
				return true
			}
		}
	}
	return false
}

func fetchTeamMcps(ctx context.Context, apiBase, token string) ([]teamMcpConfig, error) {
	var teamsResp struct {
		Teams []struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"teams"`
	}
	_, err := localPlatformAPIJSON(ctx, http.MethodGet, apiBase, token, "/api/teams", nil, &teamsResp)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var allMcps []teamMcpConfig
	var wg sync.WaitGroup

	for _, t := range teamsResp.Teams {
		wg.Add(1)
		go func(teamID string) {
			defer wg.Done()
			var mcpsResp struct {
				Mcps []teamMcpConfig `json:"mcps"`
			}
			_, err := localPlatformAPIJSON(ctx, http.MethodGet, apiBase, token, "/api/teams/"+teamID+"/mcps", nil, &mcpsResp)
			if err == nil {
				mu.Lock()
				for _, mcp := range mcpsResp.Mcps {
					if mcp.Status == "published" {
						allMcps = append(allMcps, mcp)
					}
				}
				mu.Unlock()
			}
		}(t.ID)
	}
	wg.Wait()

	return allMcps, nil
}

func SafeUpdateMcpConfig(configPath string, modifyFunc func(current map[string]any) error) error {
	lockPath := filepath.Join(filepath.Dir(configPath), "."+filepath.Base(configPath)+".lock")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	var lockFile *os.File
	var err error
	success := false
	for i := 0; i < 30; i++ {
		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			success = true
			break
		}
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create lock file: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !success {
		return fmt.Errorf("timeout waiting for lock on %s", configPath)
	}
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}()

	current := map[string]any{"mcpServers": map[string]any{}}
	if rawData, err := os.ReadFile(configPath); err == nil {
		bakPath := configPath + ".vola.bak"
		if _, statErr := os.Stat(bakPath); os.IsNotExist(statErr) {
			_ = os.WriteFile(bakPath, rawData, 0o644)
		}
		unmarshalErr := json.Unmarshal(rawData, &current)
		if unmarshalErr != nil {
			// Try to heal from backup
			if bakData, bakErr := os.ReadFile(bakPath); bakErr == nil {
				var bakCurrent map[string]any
				if json.Unmarshal(bakData, &bakCurrent) == nil {
					current = bakCurrent
					_ = os.WriteFile(configPath, bakData, 0o644)
				}
			}
		}
	}

	if err := modifyFunc(current); err != nil {
		return err
	}

	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

func connectMcpPlatform(ctx context.Context, daemonURL string, connection runtimecfg.LocalConnection, configPath string) error {
	return SafeUpdateMcpConfig(configPath, func(current map[string]any) error {
		servers, _ := current["mcpServers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		servers[LocalServerName] = map[string]any{
			"url": strings.TrimRight(daemonURL, "/") + "/mcp",
			"headers": map[string]string{
				"Authorization": "Bearer " + connection.Token,
			},
		}

		if mcps, err := fetchTeamMcps(ctx, daemonURL, connection.Token); err == nil {
			projectTags := loadLocalVolarc()
			var mergedTags []string
			mergedTags = append(mergedTags, connection.Tags...)
			mergedTags = append(mergedTags, projectTags...)

			for _, mcp := range mcps {
				if !matchTags(mcp.Tags, mergedTags) {
					continue
				}

				serverKey := "team-mcp-" + mcp.Slug
				if mcp.Transport == "stdio" {
					servers[serverKey] = map[string]any{
						"command": mcp.Command,
						"args":    mcp.Args,
						"env":     mcp.Env,
					}
				} else if mcp.Transport == "http" {
					servers[serverKey] = map[string]any{
						"url":     mcp.URL,
						"headers": mcp.Headers,
					}
				}
			}
		}

		current["mcpServers"] = servers
		return nil
	})
}

func disconnectMcpPlatform(configPath string) error {
	return SafeUpdateMcpConfig(configPath, func(current map[string]any) error {
		servers, _ := current["mcpServers"].(map[string]any)
		if servers == nil {
			return nil
		}
		delete(servers, LocalServerName)
		for key := range servers {
			if strings.HasPrefix(key, "team-mcp-") {
				delete(servers, key)
			}
		}
		current["mcpServers"] = servers
		return nil
	})
}
