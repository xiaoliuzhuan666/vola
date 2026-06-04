package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agi-bar/vola/internal/models"
	"github.com/agi-bar/vola/internal/services"
	sqlitestorage "github.com/agi-bar/vola/internal/storage/sqlite"
	"github.com/agi-bar/vola/internal/vault"
	"github.com/google/uuid"
)

func TestModelProviderGenerateTextSupportsOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"# Skill 学习记录\n\nAI insight"}`))
	}))
	t.Cleanup(server.Close)

	ctx, fileTree, modelProviders, userID := newSkillLearningServiceTestDeps(t)
	if _, err := modelProviders.Save(ctx, userID, services.SaveModelProvidersRequest{
		DefaultSummaryProviderID: "ollama-main",
		Providers: []services.ModelProviderSaveRequest{
			{
				ID:      "ollama-main",
				Type:    "ollama",
				Name:    "Ollama Main",
				BaseURL: server.URL,
				Models:  services.ModelProviderModels{Summary: "llama3.1"},
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("Save model providers: %v", err)
	}

	text, err := modelProviders.GenerateText(ctx, userID, models.TrustLevelFull, services.GenerateRequest{
		ProviderID: "ollama-main",
		Prompt:     "summarize",
	})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if !strings.Contains(text, "AI insight") {
		t.Fatalf("GenerateText response = %q", text)
	}
	if _, err := fileTree.Read(ctx, userID, services.ModelProvidersPath, models.TrustLevelFull); err != nil {
		t.Fatalf("Read model providers file: %v", err)
	}
}

func TestModelProviderGenerateTextSupportsAnthropic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "anthropic-key" {
			t.Fatalf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatalf("missing anthropic-version header")
		}
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "claude-test" || len(body.Messages) != 1 || body.Messages[0].Content == "" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"Anthropic OK"}]}`))
	}))
	t.Cleanup(server.Close)

	ctx, _, modelProviders, userID := newSkillLearningServiceTestDeps(t)
	if _, err := modelProviders.Save(ctx, userID, services.SaveModelProvidersRequest{
		DefaultSummaryProviderID: "anthropic-main",
		Providers: []services.ModelProviderSaveRequest{
			{
				ID:      "anthropic-main",
				Type:    "anthropic",
				Name:    "Anthropic Main",
				BaseURL: server.URL,
				APIKey:  "anthropic-key",
				Models:  services.ModelProviderModels{Summary: "claude-test"},
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("Save model providers: %v", err)
	}

	text, err := modelProviders.GenerateText(ctx, userID, models.TrustLevelFull, services.GenerateRequest{
		ProviderID: "anthropic-main",
		Prompt:     "summarize",
	})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if text != "Anthropic OK" {
		t.Fatalf("GenerateText response = %q", text)
	}
}

func TestModelProviderGenerateTextSupportsGemini(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-test:generateContent" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "gemini-key" {
			t.Fatalf("x-goog-api-key = %q", got)
		}
		var body struct {
			Contents []struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(body.Contents) != 1 || len(body.Contents[0].Parts) != 1 || body.Contents[0].Parts[0].Text == "" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"Gemini OK"}]}}]}`))
	}))
	t.Cleanup(server.Close)

	ctx, _, modelProviders, userID := newSkillLearningServiceTestDeps(t)
	if _, err := modelProviders.Save(ctx, userID, services.SaveModelProvidersRequest{
		DefaultSummaryProviderID: "gemini-main",
		Providers: []services.ModelProviderSaveRequest{
			{
				ID:      "gemini-main",
				Type:    "gemini",
				Name:    "Gemini Main",
				BaseURL: server.URL,
				APIKey:  "gemini-key",
				Models:  services.ModelProviderModels{Summary: "models/gemini-test"},
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("Save model providers: %v", err)
	}

	text, err := modelProviders.GenerateText(ctx, userID, models.TrustLevelFull, services.GenerateRequest{
		ProviderID: "gemini-main",
		Prompt:     "summarize",
	})
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if text != "Gemini OK" {
		t.Fatalf("GenerateText response = %q", text)
	}
}

func TestSkillLearningWriteDailyNoteUsesConfiguredModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":"# Skill 学习记录\n\n## 今日判断\nAI 生成的学习总结"}`))
	}))
	t.Cleanup(server.Close)

	ctx, fileTree, modelProviders, userID := newSkillLearningServiceTestDeps(t)
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/demo/SKILL.md", "# demo\n\nUse for demo work.\n", "text/markdown", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write skill: %v", err)
	}
	if _, err := modelProviders.Save(ctx, userID, services.SaveModelProvidersRequest{
		DefaultSummaryProviderID: "ollama-main",
		Providers: []services.ModelProviderSaveRequest{
			{
				ID:      "ollama-main",
				Type:    "ollama",
				Name:    "Ollama Main",
				BaseURL: server.URL,
				Models:  services.ModelProviderModels{Summary: "llama3.1"},
				Enabled: true,
			},
		},
	}); err != nil {
		t.Fatalf("Save model providers: %v", err)
	}

	svc := services.NewSkillLearningServiceWithModelProvider(fileTree, modelProviders)
	entry, _, err := svc.WriteDailyNote(ctx, userID, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("WriteDailyNote: %v", err)
	}
	if !strings.Contains(entry.Content, "AI 生成的学习总结") {
		t.Fatalf("daily note did not include AI content:\n%s", entry.Content)
	}
	if !strings.Contains(entry.Content, "---") || !strings.Contains(entry.Content, "Skill 总数") {
		t.Fatalf("daily note did not include rule-based appendix:\n%s", entry.Content)
	}
	latestRun, err := svc.LoadLatestLearningRun(ctx, userID, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("LoadLatestLearningRun: %v", err)
	}
	if latestRun == nil {
		t.Fatalf("expected latest learning run")
	}
	if latestRun.Status != "completed" {
		t.Fatalf("latest run status = %q", latestRun.Status)
	}
	if latestRun.Model == nil || latestRun.Model.ProviderID != "ollama-main" {
		t.Fatalf("latest run model = %+v", latestRun.Model)
	}
	if _, err := fileTree.Read(ctx, userID, latestRun.Outputs.RunPath, models.TrustLevelFull); err != nil {
		t.Fatalf("Read run json: %v", err)
	}
	report, err := fileTree.Read(ctx, userID, latestRun.Outputs.ReportPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read report: %v", err)
	}
	if !strings.Contains(report.Content, "AI 生成的学习总结") {
		t.Fatalf("report content = %q", report.Content)
	}
	skillMap, err := fileTree.Read(ctx, userID, latestRun.Outputs.SkillMapPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read skill map: %v", err)
	}
	if !strings.Contains(skillMap.Content, `"version": "vola.skill-map/v1"`) {
		t.Fatalf("skill map content = %q", skillMap.Content)
	}
	if latestRun.Outputs.VerificationPath == "" {
		t.Fatalf("expected verification output path in run: %+v", latestRun.Outputs)
	}
	verification, err := fileTree.Read(ctx, userID, latestRun.Outputs.VerificationPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read verification result: %v", err)
	}
	if !strings.Contains(verification.Content, `"version": "vola.skill-verification-run/v1"`) || !strings.Contains(verification.Content, `"quality_status"`) {
		t.Fatalf("verification result content = %q", verification.Content)
	}
}

func TestSkillLearningRunGeneratesGrowthProposal(t *testing.T) {
	ctx, fileTree, modelProviders, userID := newSkillLearningServiceTestDeps(t)
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/risky/SKILL.md", "---\ndescription: Risky skill\nwhen_to_use: Use for risky workflows\n---\n# Risky\n", "text/markdown", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write skill: %v", err)
	}
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/risky/manifest.vola.json", `{"summary":{"scripts":1,"dependency_files":0,"external_references":0}}`+"\n", "application/json", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write manifest: %v", err)
	}

	growth := services.NewGrowthProposalService(fileTree)
	svc := services.NewSkillLearningServiceWithDeps(fileTree, modelProviders, growth)
	_, _, run, err := svc.WriteDailyLearningRun(ctx, userID, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("WriteDailyLearningRun: %v", err)
	}
	proposals, err := growth.List(ctx, userID, models.TrustLevelFull, "pending_review")
	if err != nil {
		t.Fatalf("List proposals: %v", err)
	}
	if len(proposals) == 0 {
		t.Fatalf("expected growth proposal")
	}
	if proposals[0].SourceRunID != run.ID {
		t.Fatalf("proposal SourceRunID = %q, want %q", proposals[0].SourceRunID, run.ID)
	}
	if proposals[0].SuggestedChanges[0].Kind != "add_verification_note" {
		t.Fatalf("proposal change kind = %q", proposals[0].SuggestedChanges[0].Kind)
	}
}

func TestSkillLearningQualityGateDetectsRuntimeConfigs(t *testing.T) {
	ctx, fileTree, _, userID := newSkillLearningServiceTestDeps(t)
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/complex/SKILL.md", "---\ndescription: Complex skill\nwhen_to_use: Use for complex workflows\n---\n# Complex\n", "text/markdown", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write skill: %v", err)
	}
	files := map[string]string{
		"/skills/complex/scripts/run.py":                              "print('ok')\n",
		"/skills/complex/package.json":                                "{invalid json\n",
		"/skills/complex/mcp.json":                                    `{"mcpServers":{"demo":{}}}` + "\n",
		"/skills/complex/hooks/preflight.sh":                          "#!/bin/sh\necho preflight\n",
		"/skills/complex/external/claude-plugins/release/plugin.json": `{"name":"release"}` + "\n",
	}
	for path, content := range files {
		if _, err := fileTree.WriteEntry(ctx, userID, path, content, "text/plain", models.FileTreeWriteOptions{
			MinTrustLevel: models.TrustLevelGuest,
		}); err != nil {
			t.Fatalf("Write %s: %v", path, err)
		}
	}
	manifest := `{
		"version":"vola.skill-manifest/v1",
		"entry_file":"SKILL.md",
		"files":[
			{"path":"SKILL.md","kind":"entry","included":true},
			{"path":"scripts/run.py","kind":"script","included":true},
			{"path":"package.json","kind":"dependency","included":true},
			{"path":"mcp.json","kind":"config","included":true},
			{"path":"hooks/preflight.sh","kind":"script","included":true},
			{"path":"external/claude-plugins/release/plugin.json","kind":"config","included":true}
		],
		"summary":{"scripts":2,"dependency_files":1}
	}` + "\n"
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/complex/manifest.vola.json", manifest, "application/json", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write manifest: %v", err)
	}

	svc := services.NewSkillLearningService(fileTree)
	summary, err := svc.LoadSummary(ctx, userID, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("LoadSummary: %v", err)
	}
	if summary.Stats.QualityBlocked == 0 || summary.Stats.QualityManualRequired < 3 {
		t.Fatalf("quality stats = %+v", summary.Stats)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("items = %+v", summary.Items)
	}
	item := summary.Items[0]
	if item.QualityStatus != "blocked" || item.VerificationStatus != "blocked" {
		t.Fatalf("item quality status = %q verification = %q findings=%+v", item.QualityStatus, item.VerificationStatus, item.QualityFindings)
	}
	wantCodes := map[string]bool{
		"dependency_package_json_invalid": false,
		"mcp_config":                      false,
		"hook_config":                     false,
		"plugin_config":                   false,
		"verification_steps_missing":      false,
	}
	for _, finding := range item.QualityFindings {
		if _, ok := wantCodes[finding.Code]; ok {
			wantCodes[finding.Code] = true
		}
	}
	for code, seen := range wantCodes {
		if !seen {
			t.Fatalf("missing quality finding %s in %+v", code, item.QualityFindings)
		}
	}
}

func TestGrowthProposalApplyAppendSection(t *testing.T) {
	ctx, fileTree, _, userID := newSkillLearningServiceTestDeps(t)
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/demo/SKILL.md", "# Demo\n", "text/markdown", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write skill: %v", err)
	}
	growth := services.NewGrowthProposalService(fileTree)
	proposal := services.GrowthProposal{
		Version:    services.GrowthProposalVersion,
		ID:         "proposal-demo-append",
		Type:       "improve_skill",
		Status:     "pending_review",
		TargetPath: "/skills/demo/SKILL.md",
		Risk:       "low",
		Reason:     "Add verification notes.",
		SuggestedChanges: []services.GrowthProposalChange{{
			Kind:    "append_section",
			Heading: "Verification",
			Content: "Run the preview before syncing.",
		}},
		SourcePaths: []string{"/skills/demo/SKILL.md"},
		SourceRunID: "test-run",
		CreatedBy: services.GrowthProposalCreator{
			Kind:          "learning_engine",
			PromptVersion: services.GrowthProposalPromptVersion,
		},
		CreatedAt: "2026-05-24T00:00:00Z",
	}
	if _, err := fileTree.WriteEntry(ctx, userID, "/memory/proposals/skills/2026-05-24/proposal-demo-append.json", mustJSON(t, proposal), "application/json", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelFull,
	}); err != nil {
		t.Fatalf("Write proposal: %v", err)
	}

	applied, err := growth.Apply(ctx, userID, models.TrustLevelFull, proposal.ID)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Status != "applied" {
		t.Fatalf("applied status = %q", applied.Status)
	}
	entry, err := fileTree.Read(ctx, userID, "/skills/demo/SKILL.md", models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read skill: %v", err)
	}
	if !strings.Contains(entry.Content, "## Verification") || !strings.Contains(entry.Content, "Run the preview before syncing.") {
		t.Fatalf("skill content after apply:\n%s", entry.Content)
	}
}

func TestGrowthProposalCreateAndApplyCandidateSkill(t *testing.T) {
	ctx, fileTree, _, userID := newSkillLearningServiceTestDeps(t)
	growth := services.NewGrowthProposalService(fileTree)
	proposal, err := growth.CreateNewSkillProposal(ctx, userID, models.TrustLevelFull, "build a repeatable PDF redaction workflow")
	if err != nil {
		t.Fatalf("CreateNewSkillProposal: %v", err)
	}
	if proposal.Type != "new_skill" || proposal.Status != "pending_review" {
		t.Fatalf("proposal = %+v", proposal)
	}
	if !strings.HasPrefix(proposal.TargetPath, "/skills/_candidates/") {
		t.Fatalf("candidate target path = %q", proposal.TargetPath)
	}
	applied, err := growth.Apply(ctx, userID, models.TrustLevelFull, proposal.ID)
	if err != nil {
		t.Fatalf("Apply candidate: %v", err)
	}
	if applied.Status != "applied" {
		t.Fatalf("applied status = %q", applied.Status)
	}
	entry, err := fileTree.Read(ctx, userID, proposal.TargetPath, models.TrustLevelFull)
	if err != nil {
		t.Fatalf("Read candidate skill: %v", err)
	}
	if !strings.Contains(entry.Content, "build a repeatable PDF redaction workflow") {
		t.Fatalf("candidate content:\n%s", entry.Content)
	}
}

func TestGrowthProposalApplyRejectsMediumRisk(t *testing.T) {
	ctx, fileTree, _, userID := newSkillLearningServiceTestDeps(t)
	if _, err := fileTree.WriteEntry(ctx, userID, "/skills/demo/SKILL.md", "# Demo\n", "text/markdown", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelGuest,
	}); err != nil {
		t.Fatalf("Write skill: %v", err)
	}
	growth := services.NewGrowthProposalService(fileTree)
	proposal := services.GrowthProposal{
		Version:    services.GrowthProposalVersion,
		ID:         "proposal-demo-medium-risk",
		Type:       "split_skill",
		Status:     "accepted",
		TargetPath: "/skills/demo/SKILL.md",
		Risk:       "medium",
		Reason:     "Needs manual review before editing.",
		SuggestedChanges: []services.GrowthProposalChange{{
			Kind:    "append_section",
			Heading: "Split Review",
			Content: "Review whether this skill should be split.",
		}},
		SourcePaths: []string{"/skills/demo/SKILL.md"},
		SourceRunID: "test-run",
		CreatedBy: services.GrowthProposalCreator{
			Kind:          "learning_engine",
			PromptVersion: services.GrowthProposalPromptVersion,
		},
		CreatedAt: "2026-05-24T00:00:00Z",
		UpdatedAt: "2026-05-24T00:00:00Z",
	}
	if _, err := fileTree.WriteEntry(ctx, userID, "/memory/proposals/skills/2026-05-24/proposal-demo-medium-risk.json", mustJSON(t, proposal), "application/json", models.FileTreeWriteOptions{
		MinTrustLevel: models.TrustLevelFull,
	}); err != nil {
		t.Fatalf("Write proposal: %v", err)
	}

	if _, err := growth.Apply(ctx, userID, models.TrustLevelFull, proposal.ID); err == nil || !strings.Contains(err.Error(), "requires manual editing") {
		t.Fatalf("expected medium-risk apply rejection, got %v", err)
	}
}

func mustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(data)
}

func newSkillLearningServiceTestDeps(t *testing.T) (context.Context, *services.FileTreeService, *services.ModelProviderService, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	store, err := sqlitestorage.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatalf("Open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}
	v, err := vault.NewVault(strings.Repeat("0", 64))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	fileTree := services.NewFileTreeServiceWithRepo(sqlitestorage.NewFileTreeRepo(store))
	vaultSvc := services.NewVaultServiceWithRepo(sqlitestorage.NewVaultRepo(store), v)
	return ctx, fileTree, services.NewModelProviderService(fileTree, vaultSvc), user.ID
}
