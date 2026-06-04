package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/agi-bar/vola/internal/models"
	"github.com/google/uuid"
)

const skillAssignmentsPath = "/settings/agent-skill-assignments.json"

const (
	SkillLearningRunsRoot       = "/memory/learning/runs"
	SkillVerificationRunsRoot   = "/memory/learning/verifications"
	SkillLearningRunVersion     = "vola.learning-run/v1"
	SkillLearningPromptVersion  = "skill-learning-report-v1"
	SkillVerificationRunVersion = "vola.skill-verification-run/v1"
)

var skillLearningAgentTargets = []struct {
	ID   string
	Name string
}{
	{ID: "claude-code", Name: "Claude Code"},
	{ID: "codex", Name: "Codex"},
	{ID: "cursor", Name: "Cursor"},
	{ID: "gemini-cli", Name: "Gemini CLI"},
}

type SkillLearningService struct {
	fileTree        *FileTreeService
	modelProviders  *ModelProviderService
	growthProposals *GrowthProposalService
}

type SkillLearningStats struct {
	Skills                int `json:"skills"`
	Ready                 int `json:"ready"`
	NeedsSummary          int `json:"needs_summary"`
	NeedsValidation       int `json:"needs_validation"`
	RichAssets            int `json:"rich_assets"`
	Assigned              int `json:"assigned"`
	SyncRisk              int `json:"sync_risk"`
	QualityBlocked        int `json:"quality_blocked"`
	QualityManualRequired int `json:"quality_manual_required"`
	QualityWarnings       int `json:"quality_warnings"`
}

type SkillLearningItem struct {
	Name               string                `json:"name"`
	Path               string                `json:"path"`
	PrimaryPath        string                `json:"primary_path,omitempty"`
	Source             string                `json:"source"`
	Status             string                `json:"status"`
	Score              int                   `json:"score"`
	HasSummary         bool                  `json:"has_summary"`
	HasWhenToUse       bool                  `json:"has_when_to_use"`
	HasManifest        bool                  `json:"has_manifest"`
	HasScripts         bool                  `json:"has_scripts"`
	HasDependencies    bool                  `json:"has_dependencies"`
	HasExternalRefs    bool                  `json:"has_external_refs"`
	Tags               []string              `json:"tags"`
	AssignedAgents     []string              `json:"assigned_agents,omitempty"`
	Recommendations    []string              `json:"recommendations,omitempty"`
	VerificationNeeded bool                  `json:"verification_needed"`
	VerificationStatus string                `json:"verification_status"`
	QualityStatus      string                `json:"quality_status"`
	QualityStats       SkillQualityStats     `json:"quality_stats"`
	QualityFindings    []SkillQualityFinding `json:"quality_findings,omitempty"`
	UpdatedAt          string                `json:"updated_at,omitempty"`
	MatchScore         int                   `json:"match_score,omitempty"`
	MatchReasons       []string              `json:"match_reasons,omitempty"`
	Notes              []SkillLearningNote   `json:"notes,omitempty"`
}

type SkillLearningAction struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Count   int    `json:"count"`
	Message string `json:"message"`
}

type SkillLearningSummary struct {
	Stats         SkillLearningStats         `json:"stats"`
	Items         []SkillLearningItem        `json:"items"`
	Actions       []SkillLearningAction      `json:"actions"`
	ProposalStats *GrowthProposalWeeklyStats `json:"proposal_stats,omitempty"`
}

type SkillLearningMap struct {
	Version     string                `json:"version"`
	GeneratedAt string                `json:"generated_at"`
	Stats       SkillLearningStats    `json:"stats"`
	Items       []SkillLearningItem   `json:"items"`
	Actions     []SkillLearningAction `json:"actions"`
}

type SkillVerificationRun struct {
	Version     string                     `json:"version"`
	ID          string                     `json:"id"`
	GeneratedAt string                     `json:"generated_at"`
	Stats       SkillLearningStats         `json:"stats"`
	Items       []SkillVerificationRunItem `json:"items"`
}

type SkillVerificationRunItem struct {
	Name               string                `json:"name"`
	Path               string                `json:"path"`
	PrimaryPath        string                `json:"primary_path,omitempty"`
	AssignedAgents     []string              `json:"assigned_agents,omitempty"`
	VerificationNeeded bool                  `json:"verification_needed"`
	VerificationStatus string                `json:"verification_status"`
	QualityStatus      string                `json:"quality_status"`
	QualityStats       SkillQualityStats     `json:"quality_stats"`
	QualityFindings    []SkillQualityFinding `json:"quality_findings,omitempty"`
	UpdatedAt          string                `json:"updated_at,omitempty"`
}

type SkillLearningNote struct {
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LearningRun struct {
	Version    string            `json:"version"`
	ID         string            `json:"id"`
	Status     string            `json:"status"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at,omitempty"`
	Steps      []LearningRunStep `json:"steps"`
	InputPaths []string          `json:"input_paths"`
	Model      *LearningRunModel `json:"model,omitempty"`
	Outputs    LearningRunOutput `json:"outputs"`
	Error      string            `json:"error,omitempty"`
}

type LearningRunStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type LearningRunModel struct {
	ProviderID    string `json:"provider_id"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"prompt_version"`
}

type LearningRunOutput struct {
	RunPath          string `json:"run_path"`
	ReportPath       string `json:"report_path"`
	LegacyPath       string `json:"legacy_path,omitempty"`
	ProposalDir      string `json:"proposal_dir,omitempty"`
	SkillMapPath     string `json:"skill_map_path,omitempty"`
	VerificationPath string `json:"verification_path,omitempty"`
}

func NewSkillLearningService(fileTree *FileTreeService) *SkillLearningService {
	return &SkillLearningService{fileTree: fileTree}
}

func NewSkillLearningServiceWithModelProvider(fileTree *FileTreeService, modelProviders *ModelProviderService) *SkillLearningService {
	return &SkillLearningService{fileTree: fileTree, modelProviders: modelProviders}
}

func NewSkillLearningServiceWithDeps(fileTree *FileTreeService, modelProviders *ModelProviderService, growthProposals *GrowthProposalService) *SkillLearningService {
	return &SkillLearningService{fileTree: fileTree, modelProviders: modelProviders, growthProposals: growthProposals}
}

func (s *SkillLearningService) LoadSummary(ctx context.Context, userID uuid.UUID, trustLevel int) (SkillLearningSummary, error) {
	if s == nil || s.fileTree == nil {
		return SkillLearningSummary{}, fmt.Errorf("skill learning service not configured")
	}
	skills, err := s.fileTree.ListSkillSummaries(ctx, userID, trustLevel)
	if err != nil {
		return SkillLearningSummary{}, fmt.Errorf("skill learning.LoadSummary: list skills: %w", err)
	}
	assignments, err := s.readAssignments(ctx, userID)
	if err != nil {
		return SkillLearningSummary{}, err
	}
	assignedBySkill := skillLearningAssignedAgents(assignments)

	items := make([]SkillLearningItem, 0, len(skills))
	stats := SkillLearningStats{}
	for _, skill := range skills {
		if skill.Source == "system" {
			continue
		}
		if !strings.HasPrefix(normalizeAssignedSkillPath(firstNonEmpty(skill.BundlePath, skill.Path)), "/skills/") {
			continue
		}
		item := s.buildItem(ctx, userID, trustLevel, skill, assignedBySkill)
		items = append(items, item)
		stats.Skills++
		if item.Status == "ready" {
			stats.Ready++
		}
		if !item.HasSummary || !item.HasWhenToUse {
			stats.NeedsSummary++
		}
		if item.VerificationNeeded {
			stats.NeedsValidation++
		}
		if item.HasScripts || item.HasDependencies || item.HasExternalRefs {
			stats.RichAssets++
		}
		if len(item.AssignedAgents) > 0 {
			stats.Assigned++
		}
		if item.VerificationNeeded {
			stats.SyncRisk++
		}
		stats.QualityBlocked += item.QualityStats.Blocked
		stats.QualityManualRequired += item.QualityStats.ManualRequired
		stats.QualityWarnings += item.QualityStats.Warnings
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Status != items[j].Status {
			return skillLearningStatusRank(items[i].Status) < skillLearningStatusRank(items[j].Status)
		}
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		return items[i].Name < items[j].Name
	})

	summary := SkillLearningSummary{
		Stats:   stats,
		Items:   items,
		Actions: skillLearningActions(stats),
	}
	if s.growthProposals != nil {
		proposalStats, err := s.growthProposals.WeeklyStats(ctx, userID, trustLevel, time.Now().UTC())
		if err == nil {
			summary.ProposalStats = &proposalStats
		}
	}
	return summary, nil
}

func (s *SkillLearningService) Recommend(ctx context.Context, userID uuid.UUID, trustLevel int, query string) (SkillLearningSummary, error) {
	summary, err := s.LoadSummary(ctx, userID, trustLevel)
	if err != nil {
		return SkillLearningSummary{}, err
	}
	q := normalizeQuery(query)
	if q == "" {
		return summary, nil
	}

	filtered := make([]SkillLearningItem, 0, len(summary.Items))
	for _, item := range summary.Items {
		score, reasons := scoreSkillMatch(item, q)
		if score <= 0 {
			continue
		}
		item.MatchScore = score
		item.MatchReasons = uniqueLearningStrings(reasons)
		filtered = append(filtered, item)
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].MatchScore != filtered[j].MatchScore {
			return filtered[i].MatchScore > filtered[j].MatchScore
		}
		if filtered[i].Status != filtered[j].Status {
			return skillLearningStatusRank(filtered[i].Status) < skillLearningStatusRank(filtered[j].Status)
		}
		if filtered[i].Score != filtered[j].Score {
			return filtered[i].Score > filtered[j].Score
		}
		return filtered[i].Name < filtered[j].Name
	})

	out := summary
	out.Items = filtered
	return out, nil
}

func (s *SkillLearningService) RecommendWithNotes(ctx context.Context, userID uuid.UUID, trustLevel int, query string, notes []SkillLearningNote) (SkillLearningSummary, error) {
	summary, err := s.Recommend(ctx, userID, trustLevel, query)
	if err != nil {
		return SkillLearningSummary{}, err
	}
	notesByPath := map[string]SkillLearningNote{}
	for _, note := range notes {
		notesByPath[note.Path] = note
	}
	for i := range summary.Items {
		bundle := normalizeAssignedSkillPath(summary.Items[i].Path)
		if note, ok := notesByPath[path.Join("/memory/learning/skills", bundle, "summary.md")]; ok {
			summary.Items[i].Notes = []SkillLearningNote{note}
		}
	}
	return summary, nil
}

func (s *SkillLearningService) WriteDailyNote(ctx context.Context, userID uuid.UUID, trustLevel int) (*models.FileTreeEntry, SkillLearningSummary, error) {
	report, summary, _, err := s.WriteDailyLearningRun(ctx, userID, trustLevel)
	return report, summary, err
}

func (s *SkillLearningService) WriteDailyLearningRun(ctx context.Context, userID uuid.UUID, trustLevel int) (*models.FileTreeEntry, SkillLearningSummary, LearningRun, error) {
	started := time.Now().UTC()
	runDate := started.Format("2006-01-02")
	runID := runDate + "-skill-learning"
	runDir := path.Join(SkillLearningRunsRoot, runDate)
	runPath := path.Join(runDir, "run.json")
	reportPath := path.Join(runDir, "report.md")
	skillMapPath := path.Join(runDir, "skill-map.json")
	verificationPath := path.Join(SkillVerificationRunsRoot, runDate, "result.json")
	legacyPath := path.Join("/memory/learning/skills", runDate, "summary.md")
	run := LearningRun{
		Version:    SkillLearningRunVersion,
		ID:         runID,
		Status:     "running",
		StartedAt:  started.Format(time.RFC3339),
		Steps:      []LearningRunStep{{Name: "scan", Status: "running"}},
		InputPaths: []string{"/skills", skillAssignmentsPath, ModelProvidersPath},
		Outputs: LearningRunOutput{
			RunPath:          runPath,
			ReportPath:       reportPath,
			LegacyPath:       legacyPath,
			ProposalDir:      path.Join("/memory/proposals/skills", runDate),
			SkillMapPath:     skillMapPath,
			VerificationPath: verificationPath,
		},
	}

	summary, err := s.LoadSummary(ctx, userID, trustLevel)
	if err != nil {
		run.Status = "failed"
		run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		run.Error = err.Error()
		run.Steps = []LearningRunStep{{Name: "scan", Status: "failed", Error: err.Error()}}
		_ = s.writeLearningRun(ctx, userID, run)
		return nil, SkillLearningSummary{}, run, err
	}
	run.Steps = []LearningRunStep{
		{Name: "scan", Status: "completed"},
		{Name: "summarize", Status: "running"},
		{Name: "propose", Status: "pending"},
		{Name: "write_outputs", Status: "pending"},
	}

	content := renderDailyLearningNote(started, summary)
	summarizeStep := LearningRunStep{Name: "summarize", Status: "completed"}
	if aiContent, modelInfo, err := s.generateDailyLearningInsight(ctx, userID, trustLevel, started, summary); err == nil && strings.TrimSpace(aiContent) != "" {
		content = aiContent
		run.Model = modelInfo
	} else if err != nil {
		summarizeStep.Error = err.Error()
	}

	run.Steps[1] = summarizeStep
	run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "running"}
	if err := s.writeSkillMap(ctx, userID, skillMapPath, started, summary); err != nil {
		run.Status = "failed"
		run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		run.Error = err.Error()
		run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "failed", Error: err.Error()}
		_ = s.writeLearningRun(ctx, userID, run)
		return nil, SkillLearningSummary{}, run, fmt.Errorf("skill learning.WriteDailyLearningRun: write skill map: %w", err)
	}
	if err := s.writeVerificationRun(ctx, userID, verificationPath, run.ID, started, summary); err != nil {
		run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "running", Error: err.Error()}
	}
	entry, err := s.fileTree.WriteEntry(ctx, userID, reportPath, content, "text/markdown", models.FileTreeWriteOptions{
		Kind: "skill_learning_note",
		Metadata: map[string]interface{}{
			"source":         "scheduler",
			"summary_type":   "skill_learning",
			"generated_date": runDate,
			"run_id":         run.ID,
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	if err != nil {
		run.Status = "failed"
		run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		run.Error = err.Error()
		run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "failed", Error: err.Error()}
		_ = s.writeLearningRun(ctx, userID, run)
		return nil, SkillLearningSummary{}, run, fmt.Errorf("skill learning.WriteDailyLearningRun: write report: %w", err)
	}
	if _, err := s.fileTree.WriteEntry(ctx, userID, legacyPath, content, "text/markdown", models.FileTreeWriteOptions{
		Kind: "skill_learning_note",
		Metadata: map[string]interface{}{
			"source":         "scheduler",
			"summary_type":   "skill_learning",
			"generated_date": runDate,
			"run_id":         run.ID,
			"canonical_path": reportPath,
		},
		MinTrustLevel: models.TrustLevelFull,
	}); err != nil {
		run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "completed", Error: err.Error()}
	} else {
		run.Steps[3] = LearningRunStep{Name: "write_outputs", Status: "completed"}
	}
	run.Steps[2] = LearningRunStep{Name: "propose", Status: "completed"}
	if s.growthProposals != nil {
		if _, err := s.growthProposals.GenerateFromLearningRun(ctx, userID, trustLevel, summary, run); err != nil {
			run.Steps[2] = LearningRunStep{Name: "propose", Status: "completed", Error: err.Error()}
		}
	}
	run.Status = "completed"
	run.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.writeLearningRun(ctx, userID, run); err != nil {
		return entry, summary, run, err
	}
	return entry, summary, run, nil
}

func (s *SkillLearningService) LoadLatestLearningRun(ctx context.Context, userID uuid.UUID, trustLevel int) (*LearningRun, error) {
	if s == nil || s.fileTree == nil {
		return nil, fmt.Errorf("skill learning service not configured")
	}
	snapshot, err := s.fileTree.Snapshot(ctx, userID, SkillLearningRunsRoot, trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("skill learning.LoadLatestLearningRun: snapshot: %w", err)
	}
	runs := make([]LearningRun, 0)
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || path.Base(entry.Path) != "run.json" {
			continue
		}
		var run LearningRun
		if err := json.Unmarshal([]byte(entry.Content), &run); err != nil {
			continue
		}
		if run.ID == "" {
			run.ID = path.Base(path.Dir(entry.Path))
		}
		if run.Outputs.RunPath == "" {
			run.Outputs.RunPath = entry.Path
		}
		runs = append(runs, run)
	}
	if len(runs) == 0 {
		return nil, nil
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].StartedAt != runs[j].StartedAt {
			return runs[i].StartedAt > runs[j].StartedAt
		}
		return runs[i].ID > runs[j].ID
	})
	return &runs[0], nil
}

func (s *SkillLearningService) writeSkillMap(ctx context.Context, userID uuid.UUID, skillMapPath string, generatedAt time.Time, summary SkillLearningSummary) error {
	payload := SkillLearningMap{
		Version:     "vola.skill-map/v1",
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Stats:       summary.Stats,
		Items:       summary.Items,
		Actions:     summary.Actions,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.fileTree.WriteEntry(ctx, userID, skillMapPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "skill_learning_skill_map",
		Metadata: map[string]interface{}{
			"source":       "scheduler",
			"capture_mode": "skill-learning",
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	return err
}

func (s *SkillLearningService) writeVerificationRun(ctx context.Context, userID uuid.UUID, verificationPath, runID string, generatedAt time.Time, summary SkillLearningSummary) error {
	payload := SkillVerificationRun{
		Version:     SkillVerificationRunVersion,
		ID:          runID,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Stats:       summary.Stats,
		Items:       make([]SkillVerificationRunItem, 0, len(summary.Items)),
	}
	for _, item := range summary.Items {
		payload.Items = append(payload.Items, SkillVerificationRunItem{
			Name:               item.Name,
			Path:               item.Path,
			PrimaryPath:        item.PrimaryPath,
			AssignedAgents:     append([]string{}, item.AssignedAgents...),
			VerificationNeeded: item.VerificationNeeded,
			VerificationStatus: item.VerificationStatus,
			QualityStatus:      item.QualityStatus,
			QualityStats:       item.QualityStats,
			QualityFindings:    append([]SkillQualityFinding{}, item.QualityFindings...),
			UpdatedAt:          item.UpdatedAt,
		})
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.fileTree.WriteEntry(ctx, userID, verificationPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "skill_verification_run",
		Metadata: map[string]interface{}{
			"source":       "scheduler",
			"capture_mode": "skill-verification",
			"run_id":       runID,
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	return err
}

func (s *SkillLearningService) writeLearningRun(ctx context.Context, userID uuid.UUID, run LearningRun) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.fileTree.WriteEntry(ctx, userID, run.Outputs.RunPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "skill_learning_run",
		Metadata: map[string]interface{}{
			"source":       "scheduler",
			"capture_mode": "skill-learning-run",
			"run_id":       run.ID,
			"status":       run.Status,
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	return err
}

func (s *SkillLearningService) ListRecentNotes(ctx context.Context, userID uuid.UUID, trustLevel int, days int) ([]SkillLearningNote, error) {
	if s == nil || s.fileTree == nil {
		return nil, fmt.Errorf("skill learning service not configured")
	}
	if days <= 0 {
		days = 14
	}
	snapshot, err := s.fileTree.Snapshot(ctx, userID, "/memory/learning/skills", trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return []SkillLearningNote{}, nil
		}
		return nil, fmt.Errorf("skill learning.ListRecentNotes: snapshot: %w", err)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	notes := make([]SkillLearningNote, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || !strings.HasSuffix(entry.Path, ".md") {
			continue
		}
		if entry.CreatedAt.Before(cutoff) && entry.UpdatedAt.Before(cutoff) {
			continue
		}
		notes = append(notes, SkillLearningNote{
			Path:      entry.Path,
			Title:     firstMarkdownHeading(entry.Content),
			Source:    EntrySource(&entry),
			Content:   entry.Content,
			UpdatedAt: entry.UpdatedAt,
		})
	}

	sort.Slice(notes, func(i, j int) bool {
		if !notes[i].UpdatedAt.Equal(notes[j].UpdatedAt) {
			return notes[i].UpdatedAt.After(notes[j].UpdatedAt)
		}
		return notes[i].Path < notes[j].Path
	})
	if len(notes) > 10 {
		notes = notes[:10]
	}
	return notes, nil
}

func (s *SkillLearningService) buildItem(ctx context.Context, userID uuid.UUID, trustLevel int, skill models.SkillSummary, assignedBySkill map[string][]string) SkillLearningItem {
	bundlePath := normalizeAssignedSkillPath(firstNonEmpty(skill.BundlePath, path.Dir(skill.Path)))
	item := SkillLearningItem{
		Name:         skill.Name,
		Path:         bundlePath,
		PrimaryPath:  skill.PrimaryPath,
		Source:       skill.Source,
		Status:       "ready",
		Score:        100,
		HasSummary:   strings.TrimSpace(skill.Description) != "",
		HasWhenToUse: strings.TrimSpace(skill.WhenToUse) != "",
		Tags:         append([]string{}, skill.Tags...),
	}
	if entry, err := s.fileTree.Read(ctx, userID, skill.PrimaryPath, trustLevel); err == nil && entry != nil {
		item.UpdatedAt = entry.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	item.AssignedAgents = assignedBySkill[bundlePath]

	manifest := s.readManifest(ctx, userID, trustLevel, bundlePath)
	if manifest != nil {
		item.HasManifest = true
		item.HasScripts = manifest.Summary.Scripts > 0 || manifestHasKind(manifest.Files, "script")
		item.HasDependencies = manifest.Summary.DependencyFiles > 0 || manifestHasKind(manifest.Files, "dependency")
		item.HasExternalRefs = manifest.Summary.ExternalReferences > 0 || len(manifest.ExternalReferences) > 0
		if manifest.Summary.SecretRiskFiles > 0 {
			item.VerificationNeeded = true
			item.Recommendations = append(item.Recommendations, "检查 manifest 中的 secret 风险文件")
		}
		if manifest.Summary.LargeFiles > 0 {
			item.Recommendations = append(item.Recommendations, "确认大文件是否应作为 Skill 资产保留")
		}
		for _, ref := range manifest.ExternalReferences {
			if !ref.Included || ref.Status == "missing" {
				item.VerificationNeeded = true
				item.Recommendations = append(item.Recommendations, "补齐外部引用文件："+ref.Path)
				break
			}
		}
	}
	item.QualityStatus, item.QualityStats, item.QualityFindings = s.evaluateSkillQuality(ctx, userID, trustLevel, bundlePath, manifest, item.AssignedAgents)
	if item.QualityStatus != "" && item.QualityStatus != "passed" {
		for _, finding := range item.QualityFindings {
			if finding.Message != "" {
				item.Recommendations = append(item.Recommendations, finding.Message)
			}
			if len(item.Recommendations) >= 5 {
				break
			}
		}
	}
	if item.QualityStatus == "blocked" || item.QualityStatus == "manual_required" {
		item.VerificationNeeded = true
	}

	if !item.HasSummary {
		item.Score -= 25
		item.Recommendations = append(item.Recommendations, "补一句用途摘要")
	}
	if !item.HasWhenToUse {
		item.Score -= 20
		item.Recommendations = append(item.Recommendations, "补 when_to_use，让 Agent 知道何时调用")
	}
	if !item.HasManifest {
		item.Score -= 15
		item.Recommendations = append(item.Recommendations, "重新导入或生成 manifest，记录脚本、依赖和资源")
	}
	if len(item.AssignedAgents) == 0 {
		item.Score -= 10
		item.Recommendations = append(item.Recommendations, "分配给至少一个 Agent 或保留为未启用草稿")
	}
	if item.HasScripts || item.HasDependencies || item.HasExternalRefs {
		item.VerificationNeeded = true
	}
	if item.VerificationNeeded {
		item.Score -= 10
		item.Recommendations = append(item.Recommendations, "同步或导出前先做一次本地预览")
	}
	if item.Score < 0 {
		item.Score = 0
	}
	item.Recommendations = uniqueLearningStrings(item.Recommendations)
	switch {
	case !item.HasSummary || !item.HasWhenToUse:
		item.Status = "needs_summary"
		item.VerificationStatus = "incomplete"
	case item.VerificationNeeded:
		item.Status = "needs_validation"
		item.VerificationStatus = firstNonEmpty(item.QualityStatus, "required")
	default:
		item.Status = "ready"
		item.VerificationStatus = "verified"
	}
	return item
}

func (s *SkillLearningService) readManifest(ctx context.Context, userID uuid.UUID, trustLevel int, bundlePath string) *skillLearningManifest {
	entry, err := s.fileTree.Read(ctx, userID, path.Join(bundlePath, "manifest.vola.json"), trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return nil
		}
		return nil
	}
	var manifest skillLearningManifest
	if err := json.Unmarshal([]byte(entry.Content), &manifest); err != nil {
		return nil
	}
	return &manifest
}

type skillLearningManifest struct {
	Files              []skillLearningManifestFile      `json:"files"`
	ExternalReferences []skillLearningExternalReference `json:"external_references"`
	EnvVars            []string                         `json:"env_vars"`
	Warnings           []skillLearningManifestWarning   `json:"warnings"`
	Summary            skillLearningManifestSummary     `json:"summary"`
	EntryFile          string                           `json:"entry_file"`
	Version            string                           `json:"version"`
}

type skillLearningManifestFile struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Included    bool   `json:"included"`
	SizeBytes   int    `json:"size_bytes"`
	ContentType string `json:"content_type,omitempty"`
}

type skillLearningExternalReference struct {
	Path       string `json:"path"`
	SourceFile string `json:"source_file"`
	Scope      string `json:"scope"`
	Included   bool   `json:"included"`
	Status     string `json:"status"`
}

type skillLearningManifestWarning struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

type skillLearningManifestSummary struct {
	Scripts            int `json:"scripts"`
	DependencyFiles    int `json:"dependency_files"`
	ExternalReferences int `json:"external_references"`
	SecretRiskFiles    int `json:"secret_risk_files"`
	LargeFiles         int `json:"large_files"`
}

func skillLearningAssignedAgents(assignments []skillAgentAssignment) map[string][]string {
	out := map[string][]string{}
	names := map[string]string{}
	for _, agent := range skillLearningAgentTargets {
		names[agent.ID] = agent.Name
	}
	for _, assignment := range normalizeSkillAssignments(assignments) {
		agentName := firstNonEmpty(names[assignment.AgentID], assignment.AgentID)
		for _, skillPath := range assignment.SkillPaths {
			normalized := normalizeAssignedSkillPath(skillPath)
			if normalized == "" {
				continue
			}
			out[normalized] = append(out[normalized], agentName)
		}
	}
	for skillPath := range out {
		sort.Strings(out[skillPath])
	}
	return out
}

type skillAgentAssignment struct {
	AgentID    string   `json:"agent_id"`
	SkillPaths []string `json:"skill_paths"`
}

func (s *SkillLearningService) readAssignments(ctx context.Context, userID uuid.UUID) ([]skillAgentAssignment, error) {
	entry, err := s.fileTree.Read(ctx, userID, skillAssignmentsPath, models.TrustLevelFull)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return []skillAgentAssignment{}, nil
		}
		return nil, fmt.Errorf("skill learning.readAssignments: %w", err)
	}
	var payload struct {
		Assignments []skillAgentAssignment `json:"assignments"`
	}
	if err := json.Unmarshal([]byte(entry.Content), &payload); err != nil {
		return nil, fmt.Errorf("skill learning.readAssignments: decode: %w", err)
	}
	return normalizeSkillAssignments(payload.Assignments), nil
}

func normalizeSkillAssignments(items []skillAgentAssignment) []skillAgentAssignment {
	knownAgents := map[string]struct{}{}
	for _, target := range skillLearningAgentTargets {
		knownAgents[target.ID] = struct{}{}
	}
	grouped := map[string]map[string]struct{}{}
	for _, item := range items {
		agentID := strings.TrimSpace(item.AgentID)
		if _, ok := knownAgents[agentID]; !ok {
			continue
		}
		if grouped[agentID] == nil {
			grouped[agentID] = map[string]struct{}{}
		}
		for _, skillPath := range item.SkillPaths {
			if normalized := normalizeAssignedSkillPath(skillPath); normalized != "" {
				grouped[agentID][normalized] = struct{}{}
			}
		}
	}

	out := make([]skillAgentAssignment, 0, len(skillLearningAgentTargets))
	for _, target := range skillLearningAgentTargets {
		paths := sortedSkillLearningStringSet(grouped[target.ID])
		out = append(out, skillAgentAssignment{
			AgentID:    target.ID,
			SkillPaths: paths,
		})
	}
	return out
}

func normalizeAssignedSkillPath(value string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if clean == "" {
		return ""
	}
	if strings.HasSuffix(clean, "/SKILL.md") {
		clean = path.Dir(clean)
	}
	clean = strings.TrimSuffix(clean, "/")
	if clean == "" || clean == "." {
		return ""
	}
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	if !strings.HasPrefix(clean, "/skills/") {
		clean = "/skills/" + strings.TrimPrefix(clean, "/")
	}
	clean = path.Clean(clean)
	if clean != "/skills" && !strings.HasPrefix(clean, "/skills/") {
		return ""
	}
	return clean
}

func skillLearningStatusRank(status string) int {
	switch status {
	case "needs_summary":
		return 0
	case "needs_validation":
		return 1
	case "ready":
		return 2
	default:
		return 3
	}
}

func skillLearningActions(stats SkillLearningStats) []SkillLearningAction {
	actions := []SkillLearningAction{}
	if stats.QualityBlocked > 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "resolve-quality-blockers",
			Label:   "处理阻断项",
			Count:   stats.QualityBlocked,
			Message: "先处理缺失文件、外部引用、密钥风险或无法解析的依赖文件。",
		})
	}
	if stats.QualityManualRequired > 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "review-runtime-config",
			Label:   "审查运行配置",
			Count:   stats.QualityManualRequired,
			Message: "MCP、plugin、hook 和目标 Agent 试用需要人工确认后再同步。",
		})
	}
	if stats.NeedsSummary > 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "complete-summary",
			Label:   "补全用途说明",
			Count:   stats.NeedsSummary,
			Message: "优先补 description 和 when_to_use，Agent 才能知道何时调用。",
		})
	}
	if stats.NeedsValidation > 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "preview-validation",
			Label:   "做本地预览",
			Count:   stats.NeedsValidation,
			Message: "含脚本、依赖或外部引用的 Skill，先预览再应用。",
		})
	}
	unassigned := stats.Skills - stats.Assigned
	if unassigned > 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "assign-agent",
			Label:   "分配给 Agent",
			Count:   unassigned,
			Message: "未分配的 Skill 不会进入 Claude Code / Codex 本地同步。",
		})
	}
	if len(actions) == 0 {
		actions = append(actions, SkillLearningAction{
			Code:    "ready",
			Label:   "当前可用",
			Count:   stats.Ready,
			Message: "当前 Skill 元数据和分配状态较完整，可以按需同步或导出。",
		})
	}
	return actions
}

func normalizeQuery(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return strings.Join(strings.Fields(value), " ")
}

func renderDailyLearningNote(now time.Time, summary SkillLearningSummary) string {
	var b strings.Builder
	b.WriteString("# Skill 学习记录\n\n")
	b.WriteString("- 生成时间: ")
	b.WriteString(now.UTC().Format(time.RFC3339))
	b.WriteString("\n")
	b.WriteString("- Skill 总数: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.Skills))
	b.WriteString("\n")
	b.WriteString("- 可直接使用: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.Ready))
	b.WriteString("\n")
	b.WriteString("- 缺用途说明: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.NeedsSummary))
	b.WriteString("\n")
	b.WriteString("- 需验证: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.NeedsValidation))
	b.WriteString("\n")
	b.WriteString("- 质量阻断项: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.QualityBlocked))
	b.WriteString("\n")
	b.WriteString("- 需人工审查: ")
	b.WriteString(fmt.Sprintf("%d", summary.Stats.QualityManualRequired))
	b.WriteString("\n\n")

	b.WriteString("## 今日动作\n")
	for _, action := range summary.Actions {
		b.WriteString("- ")
		b.WriteString(action.Label)
		b.WriteString(" (")
		b.WriteString(fmt.Sprintf("%d", action.Count))
		b.WriteString("): ")
		b.WriteString(action.Message)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	top := summary.Items
	if len(top) > 5 {
		top = top[:5]
	}
	if len(top) > 0 {
		b.WriteString("## 重点 Skill\n")
		for _, item := range top {
			b.WriteString("- ")
			b.WriteString(item.Name)
			b.WriteString(" · ")
			b.WriteString(item.Status)
			b.WriteString(" · ")
			b.WriteString(fmt.Sprintf("%d", item.Score))
			if len(item.Recommendations) > 0 {
				b.WriteString(" · ")
				b.WriteString(item.Recommendations[0])
			}
			if len(item.QualityFindings) > 0 {
				b.WriteString(" · ")
				b.WriteString(item.QualityFindings[0].Title)
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (s *SkillLearningService) generateDailyLearningInsight(ctx context.Context, userID uuid.UUID, trustLevel int, now time.Time, summary SkillLearningSummary) (string, *LearningRunModel, error) {
	if s == nil || s.modelProviders == nil {
		return "", nil, nil
	}
	doc, err := s.modelProviders.Load(ctx, userID, trustLevel)
	if err != nil {
		return "", nil, err
	}
	providerID := firstNonEmpty(doc.DefaultSummaryProviderID, doc.DefaultProposalProviderID)
	if providerID == "" {
		return "", nil, nil
	}
	var provider ModelProvider
	for _, item := range doc.Providers {
		if item.ID == providerID {
			provider = item
			break
		}
	}
	if provider.ID == "" || !provider.Enabled {
		return "", nil, nil
	}
	prompt := renderDailyLearningPrompt(now, summary)
	model := firstNonEmpty(provider.Models.Summary, provider.Models.JSON, provider.Models.Proposal)
	modelInfo := &LearningRunModel{ProviderID: provider.ID, Model: model, PromptVersion: SkillLearningPromptVersion}
	text, err := s.modelProviders.GenerateText(ctx, userID, trustLevel, GenerateRequest{
		ProviderID: provider.ID,
		Model:      model,
		Prompt:     prompt,
	})
	if err != nil {
		return "", modelInfo, err
	}
	clean := strings.TrimSpace(stripMarkdownCodeFence(text))
	if clean == "" {
		return "", modelInfo, nil
	}
	if !strings.HasPrefix(clean, "#") {
		clean = "# Skill 学习记录\n\n" + clean
	}
	clean += "\n\n---\n\n"
	clean += renderDailyLearningNote(now, summary)
	return clean, modelInfo, nil
}

func renderDailyLearningPrompt(now time.Time, summary SkillLearningSummary) string {
	var b strings.Builder
	b.WriteString("你是 Vola 的本地 Skill 学习引擎。请根据下面的结构化数据，输出一份中文 Markdown 日报。\n")
	b.WriteString("要求：\n")
	b.WriteString("- 不要夸张宣传，不要编造不存在的能力。\n")
	b.WriteString("- 重点写今天 Skill 知识库有哪些变化风险、应该补什么、哪些 Skill 可以更好地复用。\n")
	b.WriteString("- 输出包含：今日判断、优先改进、可复用知识点、明日建议。\n")
	b.WriteString("- 不要输出 JSON。\n\n")
	b.WriteString("生成日期: ")
	b.WriteString(now.UTC().Format("2006-01-02"))
	b.WriteString("\n\n")
	b.WriteString("统计:\n")
	b.WriteString(fmt.Sprintf("- Skill 总数: %d\n", summary.Stats.Skills))
	b.WriteString(fmt.Sprintf("- 可直接使用: %d\n", summary.Stats.Ready))
	b.WriteString(fmt.Sprintf("- 缺用途说明: %d\n", summary.Stats.NeedsSummary))
	b.WriteString(fmt.Sprintf("- 需验证: %d\n", summary.Stats.NeedsValidation))
	b.WriteString(fmt.Sprintf("- 有脚本/依赖/外部引用: %d\n", summary.Stats.RichAssets))
	b.WriteString(fmt.Sprintf("- 已分配给 Agent: %d\n\n", summary.Stats.Assigned))
	b.WriteString(fmt.Sprintf("- 质量阻断项: %d\n", summary.Stats.QualityBlocked))
	b.WriteString(fmt.Sprintf("- 需人工审查: %d\n", summary.Stats.QualityManualRequired))
	b.WriteString(fmt.Sprintf("- 质量提醒: %d\n\n", summary.Stats.QualityWarnings))
	b.WriteString("系统建议动作:\n")
	for _, action := range summary.Actions {
		b.WriteString(fmt.Sprintf("- %s (%d): %s\n", action.Label, action.Count, action.Message))
	}
	b.WriteString("\n重点 Skill:\n")
	items := summary.Items
	if len(items) > 12 {
		items = items[:12]
	}
	for _, item := range items {
		recommendations := strings.Join(item.Recommendations, "; ")
		if recommendations == "" {
			recommendations = "无"
		}
		agents := strings.Join(item.AssignedAgents, ", ")
		if agents == "" {
			agents = "未分配"
		}
		quality := item.QualityStatus
		if quality == "" {
			quality = "unknown"
		}
		finding := "无"
		if len(item.QualityFindings) > 0 {
			finding = item.QualityFindings[0].Title + ": " + item.QualityFindings[0].Message
		}
		b.WriteString(fmt.Sprintf("- 名称: %s\n  路径: %s\n  状态: %s\n  质量: %s\n  评分: %d\n  Agent: %s\n  建议: %s\n  质量项: %s\n", item.Name, item.Path, item.Status, quality, item.Score, agents, recommendations, finding))
	}
	return b.String()
}

func stripMarkdownCodeFence(value string) string {
	clean := strings.TrimSpace(value)
	if !strings.HasPrefix(clean, "```") {
		return clean
	}
	lines := strings.Split(clean, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return strings.Join(lines[1:len(lines)-1], "\n")
	}
	return clean
}

func scoreSkillMatch(item SkillLearningItem, query string) (int, []string) {
	if strings.TrimSpace(query) == "" {
		return item.Score, nil
	}
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	combined := strings.ToLower(strings.Join([]string{
		item.Name,
		item.Path,
		item.Source,
		strings.Join(item.Tags, " "),
		strings.Join(item.Recommendations, " "),
	}, " "))

	score := item.Score / 2
	reasons := []string{}

	if strings.Contains(strings.ToLower(item.Name), lowerQuery) {
		score += 50
		reasons = append(reasons, "名称命中")
	}
	if strings.Contains(strings.ToLower(item.PrimaryPath), lowerQuery) || strings.Contains(strings.ToLower(item.Path), lowerQuery) {
		score += 20
		reasons = append(reasons, "路径命中")
	}
	if strings.Contains(combined, lowerQuery) {
		score += 30
		reasons = append(reasons, "说明命中")
	}

	tokens := tokenizeSkillQuery(lowerQuery)
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(strings.ToLower(item.Name), token) {
			score += 16
			reasons = append(reasons, "名称包含 "+token)
		}
		if strings.Contains(strings.ToLower(item.Path), token) {
			score += 10
			reasons = append(reasons, "路径包含 "+token)
		}
		for _, tag := range item.Tags {
			if strings.Contains(strings.ToLower(tag), token) {
				score += 14
				reasons = append(reasons, "标签包含 "+tag)
				break
			}
		}
	}

	switch item.Status {
	case "ready":
		score += 12
		reasons = append(reasons, "可直接使用")
	case "needs_validation":
		score += 4
		reasons = append(reasons, "需要验证")
	case "needs_summary":
		score -= 6
		reasons = append(reasons, "信息不完整")
	}

	if len(item.AssignedAgents) > 0 {
		score += 6
		reasons = append(reasons, "已分配 Agent")
	}

	if score < 0 {
		score = 0
	}
	return score, uniqueLearningStrings(reasons)
}

func tokenizeSkillQuery(query string) []string {
	splitter := func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		default:
			return true
		}
	}
	parts := strings.FieldsFunc(query, splitter)
	if len(parts) == 0 {
		return []string{query}
	}
	return parts
}

func uniqueLearningStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func manifestHasKind(files []skillLearningManifestFile, kind string) bool {
	for _, file := range files {
		if file.Included && strings.EqualFold(strings.TrimSpace(file.Kind), kind) {
			return true
		}
	}
	return false
}

func sortedSkillLearningStringSet(values map[string]struct{}) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
