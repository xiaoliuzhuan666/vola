package sqlite

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agi-bar/neudrive/internal/models"
	"github.com/agi-bar/neudrive/internal/skillsarchive"
	"github.com/google/uuid"
)

func TestImportAgentExportClaudeConversationWritesCanonicalArchive(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	client := &Client{store: store, userID: user.ID}

	payload := AgentExportPayload{
		Claude: &ClaudeInventory{
			Conversations: []ClaudeConversation{{
				Name:        "Demo Chat",
				SessionID:   "sess-123",
				ProjectName: "demo-project",
				Summary:     "Imported from Claude Code scan",
				StartedAt:   "2026-04-16T20:00:00Z",
				Exactness:   "exact",
				SourcePaths: []string{"/tmp/demo-chat.jsonl"},
				Messages: []ClaudeConversationMessage{
					{
						ID:        "msg-1",
						Role:      "user",
						Content:   "Hello from Claude Code",
						Timestamp: "2026-04-16T20:00:01Z",
						Kind:      "message",
					},
					{
						ID:        "msg-2",
						ParentID:  "msg-1",
						Role:      "assistant",
						Content:   "Hi there",
						Timestamp: "2026-04-16T20:00:02Z",
						Kind:      "message",
					},
				},
			}},
		},
	}

	result, err := client.ImportAgentExport(ctx, "claude-code", payload)
	if err != nil {
		t.Fatalf("ImportAgentExport: %v", err)
	}
	if result.Conversations != 1 {
		t.Fatalf("Conversations = %d, want 1", result.Conversations)
	}

	rootPath := "/conversations/claude-code/2026-04-16-demo-chat-sess-123-compact"
	transcriptPath := rootPath + "/conversation.md"
	conversationPath := rootPath + "/conversation.json"
	indexPath := "/conversations/claude-code/index.json"

	root, err := store.Read(ctx, user.ID, rootPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read conversation root: %v", err)
	}
	for key, want := range map[string]interface{}{
		"conversation_title":           "Demo Chat",
		"source_platform":              "claude-code",
		"source_conversation_id":       "sess-123",
		"conversation_started_at":      "2026-04-16T20:00:00Z",
		"conversation_ended_at":        "2026-04-16T20:00:02Z",
		"conversation_project_name":    "demo-project",
		"conversation_message_count":   float64(2),
		"message_count":                float64(2),
		"turn_count":                   float64(2),
		"bundle_primary_path":          transcriptPath,
		"conversation_transcript_path": transcriptPath,
		"conversation_path":            conversationPath,
	} {
		if got := root.Metadata[key]; got != want {
			t.Fatalf("root metadata[%s] = %#v, want %#v", key, got, want)
		}
	}

	transcript, err := store.Read(ctx, user.ID, transcriptPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read transcript: %v", err)
	}
	if !strings.Contains(transcript.Content, "# Demo Chat") {
		t.Fatalf("transcript missing title: %q", transcript.Content)
	}
	if !strings.Contains(transcript.Content, "## User 1") || !strings.Contains(transcript.Content, "## Assistant 2") {
		t.Fatalf("transcript missing rendered turns: %q", transcript.Content)
	}

	conversation, err := store.Read(ctx, user.ID, conversationPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read conversation sidecar: %v", err)
	}
	for _, want := range []string{
		`"version": "neudrive.conversation/v1"`,
		`"import_strategy": "claude-code-local-scan"`,
		`"source_conversation_id": "sess-123"`,
		`"transcript_path": "` + transcriptPath + `"`,
		`"message_count": 2`,
	} {
		if !strings.Contains(conversation.Content, want) {
			t.Fatalf("conversation sidecar missing %s: %q", want, conversation.Content)
		}
	}

	index, err := store.Read(ctx, user.ID, indexPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read index: %v", err)
	}
	for _, want := range []string{
		`"root_path": "` + rootPath + `"`,
		`"transcript_path": "` + transcriptPath + `"`,
		`"conversation_path": "` + conversationPath + `"`,
	} {
		if !strings.Contains(index.Content, want) {
			t.Fatalf("index missing %s: %q", want, index.Content)
		}
	}
}

func TestImportAgentExportClaudeConversationPreservesStructuredParts(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	client := &Client{store: store, userID: user.ID}

	payload := AgentExportPayload{
		Claude: &ClaudeInventory{
			Conversations: []ClaudeConversation{{
				Name:      "Structured Demo",
				SessionID: "structured-123",
				StartedAt: "2026-04-16T20:00:00Z",
				Messages: []ClaudeConversationMessage{
					{
						ID:        "msg-1",
						Role:      "assistant",
						Content:   "I inspected the repo.",
						Timestamp: "2026-04-16T20:00:01Z",
						Kind:      "assistant",
						Parts: []NormalizedPart{
							{Type: "thinking", Text: "Need to inspect files first"},
							{Type: "text", Text: "I inspected the repo."},
							{Type: "tool_call", Name: "bash", ArgsText: "{\n  \"command\": \"ls -la\"\n}"},
							{Type: "tool_result", Text: "total 8"},
						},
					},
				},
			}},
		},
	}

	if _, err := client.ImportAgentExport(ctx, "claude-code", payload); err != nil {
		t.Fatalf("ImportAgentExport: %v", err)
	}

	rootPath := "/conversations/claude-code/2026-04-16-structured-demo-structured-123-compact"
	transcriptPath := rootPath + "/conversation.md"
	conversationPath := rootPath + "/conversation.json"

	transcript, err := store.Read(ctx, user.ID, transcriptPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read transcript: %v", err)
	}
	for _, want := range []string{
		"Thinking (condensed)",
		"### Tool Call: `bash`",
		"### Tool Result",
	} {
		if !strings.Contains(transcript.Content, want) {
			t.Fatalf("transcript missing %s: %q", want, transcript.Content)
		}
	}

	conversation, err := store.Read(ctx, user.ID, conversationPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read conversation sidecar: %v", err)
	}
	for _, want := range []string{
		`"type": "thinking"`,
		`"type": "tool_call"`,
		`"name": "bash"`,
		`"type": "tool_result"`,
	} {
		if !strings.Contains(conversation.Content, want) {
			t.Fatalf("conversation sidecar missing %s: %q", want, conversation.Content)
		}
	}
}

func TestImportAgentExportSkillBundlesWritePortableManifest(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	client := &Client{store: store, userID: user.ID}

	logo := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	payload := AgentExportPayload{
		Claude: &ClaudeInventory{
			Bundles: []ClaudeBundle{{
				Name:        "release-helper",
				Kind:        "skill",
				Description: "Complex Claude Code skill",
				Exactness:   "exact",
				SourcePaths: []string{"/tmp/.claude/skills/release-helper"},
				Files: []ClaudeFileRecord{
					{
						Path:        "SKILL.md",
						Content:     "# Release Helper\n\nRun ~/.claude/tools/release.py and ~/.claude/plugins/release/plugin.json with $RELEASE_TOKEN.\n",
						ContentType: "text/markdown",
					},
					{Path: "scripts/ship.sh", Content: "#!/bin/sh\necho release\n", ContentType: "text/x-shellscript"},
					{Path: "requirements.txt", Content: "requests==2.32.0\n", ContentType: "text/plain"},
					{Path: "package.json", Content: `{"scripts":{"check":"node check.js"}}` + "\n", ContentType: "application/json"},
					{Path: "mcp.json", Content: `{"mcpServers":{"demo":{}}}` + "\n", ContentType: "application/json"},
					{Path: "hooks/preflight.sh", Content: "#!/bin/sh\necho preflight\n", ContentType: "text/x-shellscript"},
					{Path: "external/claude-tools/release.py", Content: "print('release')\n", ContentType: "text/x-python"},
					{Path: "external/claude-plugins/release/plugin.json", Content: `{"name":"release"}` + "\n", ContentType: "application/json"},
					{Path: "assets/logo.png", ContentBase64: base64.StdEncoding.EncodeToString(logo), ContentType: "image/png"},
				},
			}},
		},
	}

	result, err := client.ImportAgentExport(ctx, "claude-code", payload)
	if err != nil {
		t.Fatalf("ImportAgentExport: %v", err)
	}
	if result.Bundles != 1 {
		t.Fatalf("Bundles = %d, want 1", result.Bundles)
	}

	claudeManifest := readSkillManifestForTest(t, store, user.ID, "/skills/release-helper/manifest.neudrive.json")
	if claudeManifest.SourcePlatform != "claude-code" {
		t.Fatalf("claude manifest platform = %q", claudeManifest.SourcePlatform)
	}
	if claudeManifest.Summary.Scripts < 3 || claudeManifest.Summary.DependencyFiles != 2 || claudeManifest.Summary.BinaryFiles != 1 {
		t.Fatalf("unexpected claude manifest summary: %+v", claudeManifest.Summary)
	}
	if len(claudeManifest.EnvVars) != 1 || claudeManifest.EnvVars[0] != "RELEASE_TOKEN" {
		t.Fatalf("unexpected env vars: %+v", claudeManifest.EnvVars)
	}
	if len(claudeManifest.ExternalReferences) != 2 {
		t.Fatalf("unexpected external refs: %+v", claudeManifest.ExternalReferences)
	}
	for _, ref := range claudeManifest.ExternalReferences {
		if !ref.Included || ref.Status != "included" {
			t.Fatalf("expected included external ref: %+v", ref)
		}
	}
	binaryData, _, err := store.ReadBinary(ctx, user.ID, "/skills/release-helper/assets/logo.png", models.TrustLevelWork)
	if err != nil {
		t.Fatalf("ReadBinary logo: %v", err)
	}
	if string(binaryData) != string(logo) {
		t.Fatal("binary logo changed during import")
	}

	codexResult, err := client.ImportAgentExport(ctx, "codex", AgentExportPayload{
		Codex: &CodexInventory{
			Bundles: []ClaudeBundle{{
				Name:        "codex-audit",
				Kind:        "skill",
				Description: "Complex Codex skill",
				Files: []ClaudeFileRecord{
					{Path: "SKILL.md", Content: "# Codex Audit\n\nUse the packaged plugin metadata as reference only.\n", ContentType: "text/markdown"},
					{Path: ".codex-plugin/plugin.json", Content: `{"name":"audit","mcpServers":{"demo":{}}}` + "\n", ContentType: "application/json"},
					{Path: "scripts/audit.py", Content: "print('audit')\n", ContentType: "text/x-python"},
					{Path: "assets/icon.png", ContentBase64: base64.StdEncoding.EncodeToString(logo), ContentType: "image/png"},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("ImportAgentExport codex: %v", err)
	}
	if codexResult.Bundles != 1 {
		t.Fatalf("Codex Bundles = %d, want 1", codexResult.Bundles)
	}
	codexManifest := readSkillManifestForTest(t, store, user.ID, "/skills/codex-audit/manifest.neudrive.json")
	if codexManifest.SourcePlatform != "codex" {
		t.Fatalf("codex manifest platform = %q", codexManifest.SourcePlatform)
	}
	if codexManifest.Summary.Scripts != 1 || codexManifest.Summary.BinaryFiles != 1 {
		t.Fatalf("unexpected codex manifest summary: %+v", codexManifest.Summary)
	}
	if !manifestHasFile(codexManifest, ".codex-plugin/plugin.json") {
		t.Fatalf("codex manifest missing plugin metadata: %+v", codexManifest.Files)
	}
}

func readSkillManifestForTest(t *testing.T, store *Store, userID uuid.UUID, manifestPath string) skillsarchive.SkillManifest {
	t.Helper()
	entry, err := store.Read(context.Background(), userID, manifestPath, models.TrustLevelWork)
	if err != nil {
		t.Fatalf("Read %s: %v", manifestPath, err)
	}
	var manifest skillsarchive.SkillManifest
	if err := json.Unmarshal([]byte(entry.Content), &manifest); err != nil {
		t.Fatalf("Unmarshal %s: %v", manifestPath, err)
	}
	return manifest
}

func manifestHasFile(manifest skillsarchive.SkillManifest, relPath string) bool {
	for _, file := range manifest.Files {
		if file.Path == relPath && file.Included {
			return true
		}
	}
	return false
}
