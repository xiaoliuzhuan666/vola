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

const (
	GrowthProposalRoot          = "/memory/proposals/skills"
	GrowthProposalVersion       = "vola.growth-proposal/v1"
	GrowthProposalPromptVersion = "skill-growth-proposal-v1"
)

type GrowthProposalService struct {
	fileTree *FileTreeService
}

type GrowthProposal struct {
	Version          string                 `json:"version"`
	ID               string                 `json:"id"`
	Type             string                 `json:"type"`
	Status           string                 `json:"status"`
	TargetPath       string                 `json:"target_path"`
	Risk             string                 `json:"risk"`
	Reason           string                 `json:"reason"`
	SuggestedChanges []GrowthProposalChange `json:"suggested_changes"`
	SourcePaths      []string               `json:"source_paths"`
	SourceRunID      string                 `json:"source_run_id,omitempty"`
	CreatedBy        GrowthProposalCreator  `json:"created_by"`
	CreatedAt        string                 `json:"created_at"`
	UpdatedAt        string                 `json:"updated_at,omitempty"`
	AppliedAt        string                 `json:"applied_at,omitempty"`
	DismissedAt      string                 `json:"dismissed_at,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type GrowthProposalChange struct {
	Kind    string `json:"kind"`
	Heading string `json:"heading,omitempty"`
	Content string `json:"content,omitempty"`
	Field   string `json:"field,omitempty"`
	Value   string `json:"value,omitempty"`
	Path    string `json:"path,omitempty"`
}

type GrowthProposalCreator struct {
	Kind            string `json:"kind"`
	ModelProviderID string `json:"model_provider_id,omitempty"`
	Model           string `json:"model,omitempty"`
	PromptVersion   string `json:"prompt_version"`
}

type GrowthProposalWeeklyStats struct {
	Since         string `json:"since"`
	NewProposals  int    `json:"new_proposals"`
	Accepted      int    `json:"accepted"`
	Dismissed     int    `json:"dismissed"`
	Applied       int    `json:"applied"`
	PendingReview int    `json:"pending_review"`
}

func NewGrowthProposalService(fileTree *FileTreeService) *GrowthProposalService {
	return &GrowthProposalService{fileTree: fileTree}
}

func (s *GrowthProposalService) CreateNewSkillProposal(ctx context.Context, userID uuid.UUID, trustLevel int, query string) (GrowthProposal, error) {
	if s == nil || s.fileTree == nil {
		return GrowthProposal{}, fmt.Errorf("growth proposal service not configured")
	}
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		return GrowthProposal{}, fmt.Errorf("query is required")
	}
	now := time.Now().UTC()
	runDate := now.Format("2006-01-02")
	slug := growthProposalSlug(cleanQuery)
	proposal := GrowthProposal{
		Version:    GrowthProposalVersion,
		ID:         growthProposalID(runDate, "new-skill", slug),
		Type:       "new_skill",
		Status:     "pending_review",
		TargetPath: path.Join("/skills/_candidates", slug, "SKILL.md"),
		Risk:       "low",
		Reason:     "当前 Skill 库没有明显匹配的新需求，可以先生成候选 Skill，审查后再决定是否应用。",
		SuggestedChanges: []GrowthProposalChange{{
			Kind:    "create_candidate_skill",
			Path:    path.Join("/skills/_candidates", slug, "SKILL.md"),
			Content: renderCandidateSkillMarkdown(cleanQuery, slug),
		}},
		SourcePaths: []string{"/skills"},
		CreatedBy: GrowthProposalCreator{
			Kind:          "learning_engine",
			PromptVersion: GrowthProposalPromptVersion,
		},
		CreatedAt: now.Format(time.RFC3339),
	}
	if existing, _, err := s.findByID(ctx, userID, trustLevel, proposal.ID); err == nil {
		return existing, nil
	}
	if err := s.writeProposal(ctx, userID, runDate, proposal); err != nil {
		return GrowthProposal{}, err
	}
	return proposal, nil
}

func (s *GrowthProposalService) GenerateFromLearningRun(ctx context.Context, userID uuid.UUID, trustLevel int, summary SkillLearningSummary, run LearningRun) ([]GrowthProposal, error) {
	if s == nil || s.fileTree == nil {
		return nil, fmt.Errorf("growth proposal service not configured")
	}
	now := time.Now().UTC()
	runDate := now.Format("2006-01-02")
	if run.Outputs.ProposalDir != "" {
		runDate = path.Base(run.Outputs.ProposalDir)
	}
	creator := GrowthProposalCreator{
		Kind:          "learning_engine",
		PromptVersion: GrowthProposalPromptVersion,
	}
	if run.Model != nil {
		creator.ModelProviderID = run.Model.ProviderID
		creator.Model = run.Model.Model
	}

	proposals := make([]GrowthProposal, 0, 3)
	for _, item := range summary.Items {
		if len(proposals) >= 3 {
			break
		}
		targetPath := firstNonEmpty(item.PrimaryPath, path.Join(item.Path, "SKILL.md"))
		switch {
		case item.VerificationNeeded:
			proposals = append(proposals, GrowthProposal{
				Version:    GrowthProposalVersion,
				ID:         growthProposalID(runDate, "verification", item.Path),
				Type:       "improve_skill",
				Status:     "pending_review",
				TargetPath: targetPath,
				Risk:       "low",
				Reason:     "这个 Skill 含脚本、依赖或外部引用，同步前需要保留可执行的验证提示。",
				SuggestedChanges: []GrowthProposalChange{{
					Kind:    "add_verification_note",
					Heading: "Verification",
					Content: "同步或导出前，先检查脚本、依赖和外部引用是否仍可用；如果有本地命令，先执行一次预览或 dry run。",
				}},
				SourcePaths: []string{targetPath, path.Join(item.Path, "manifest.vola.json")},
				SourceRunID: run.ID,
				CreatedBy:   creator,
				CreatedAt:   now.Format(time.RFC3339),
			})
		case !item.HasWhenToUse:
			proposals = append(proposals, GrowthProposal{
				Version:    GrowthProposalVersion,
				ID:         growthProposalID(runDate, "when-to-use", item.Path),
				Type:       "improve_skill",
				Status:     "pending_review",
				TargetPath: targetPath,
				Risk:       "low",
				Reason:     "这个 Skill 缺少 when_to_use，Agent 很难判断什么时候调用。",
				SuggestedChanges: []GrowthProposalChange{{
					Kind:  "update_frontmatter_field",
					Field: "when_to_use",
					Value: "Use this skill when the request clearly matches this bundle's documented workflow.",
				}},
				SourcePaths: []string{targetPath},
				SourceRunID: run.ID,
				CreatedBy:   creator,
				CreatedAt:   now.Format(time.RFC3339),
			})
		case !item.HasSummary:
			proposals = append(proposals, GrowthProposal{
				Version:    GrowthProposalVersion,
				ID:         growthProposalID(runDate, "description", item.Path),
				Type:       "improve_skill",
				Status:     "pending_review",
				TargetPath: targetPath,
				Risk:       "low",
				Reason:     "这个 Skill 缺少简短用途摘要，列表和推荐结果里不够清晰。",
				SuggestedChanges: []GrowthProposalChange{{
					Kind:  "update_frontmatter_field",
					Field: "description",
					Value: "Reusable workflow captured from this skill bundle.",
				}},
				SourcePaths: []string{targetPath},
				SourceRunID: run.ID,
				CreatedBy:   creator,
				CreatedAt:   now.Format(time.RFC3339),
			})
		}
	}
	for _, item := range summary.Items {
		if len(proposals) >= 4 {
			break
		}
		if !(item.HasScripts && item.HasDependencies && item.HasExternalRefs) {
			continue
		}
		targetPath := firstNonEmpty(item.PrimaryPath, path.Join(item.Path, "SKILL.md"))
		proposals = append(proposals, GrowthProposal{
			Version:    GrowthProposalVersion,
			ID:         growthProposalID(runDate, "split-skill", item.Path),
			Type:       "split_skill",
			Status:     "pending_review",
			TargetPath: targetPath,
			Risk:       "medium",
			Reason:     "这个 Skill 同时包含脚本、依赖和外部引用，可能已经承担多个工作流，建议人工评估是否拆成更小的候选 Skill。",
			SuggestedChanges: []GrowthProposalChange{{
				Kind:    "append_section",
				Heading: "Split Review",
				Content: "Review whether scripts, dependencies, and external references belong to one workflow. If not, create separate candidate Skills before syncing broadly.",
			}},
			SourcePaths: []string{targetPath, path.Join(item.Path, "manifest.vola.json")},
			SourceRunID: run.ID,
			CreatedBy:   creator,
			CreatedAt:   now.Format(time.RFC3339),
		})
	}
	for _, item := range summary.Items {
		if len(proposals) >= 5 {
			break
		}
		if item.Score > 35 || len(item.AssignedAgents) > 0 {
			continue
		}
		targetPath := firstNonEmpty(item.PrimaryPath, path.Join(item.Path, "SKILL.md"))
		proposals = append(proposals, GrowthProposal{
			Version:    GrowthProposalVersion,
			ID:         growthProposalID(runDate, "archive-review", item.Path),
			Type:       "archive_or_review",
			Status:     "pending_review",
			TargetPath: targetPath,
			Risk:       "medium",
			Reason:     "这个 Skill 信息不足且未分配给任何 Agent，建议人工决定补全、保留为草稿，或移入归档。",
			SuggestedChanges: []GrowthProposalChange{{
				Kind:    "append_section",
				Heading: "Review",
				Content: "Decide whether to complete this Skill, keep it as a draft, or archive it outside the active sync set.",
			}},
			SourcePaths: []string{targetPath},
			SourceRunID: run.ID,
			CreatedBy:   creator,
			CreatedAt:   now.Format(time.RFC3339),
		})
	}
	for i := range proposals {
		if err := s.writeProposal(ctx, userID, runDate, proposals[i]); err != nil {
			return proposals[:i], err
		}
	}
	return proposals, nil
}

func (s *GrowthProposalService) List(ctx context.Context, userID uuid.UUID, trustLevel int, status string) ([]GrowthProposal, error) {
	if s == nil || s.fileTree == nil {
		return nil, fmt.Errorf("growth proposal service not configured")
	}
	snapshot, err := s.fileTree.Snapshot(ctx, userID, GrowthProposalRoot, trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return []GrowthProposal{}, nil
		}
		return nil, fmt.Errorf("growth proposals.List: snapshot: %w", err)
	}
	filterStatus := strings.TrimSpace(status)
	proposals := make([]GrowthProposal, 0)
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || !strings.HasSuffix(entry.Path, ".json") {
			continue
		}
		var proposal GrowthProposal
		if err := json.Unmarshal([]byte(entry.Content), &proposal); err != nil {
			continue
		}
		if proposal.ID == "" {
			continue
		}
		if filterStatus != "" && proposal.Status != filterStatus {
			continue
		}
		proposals = append(proposals, proposal)
	}
	sort.Slice(proposals, func(i, j int) bool {
		if proposals[i].CreatedAt != proposals[j].CreatedAt {
			return proposals[i].CreatedAt > proposals[j].CreatedAt
		}
		return proposals[i].ID < proposals[j].ID
	})
	return proposals, nil
}

func (s *GrowthProposalService) WeeklyStats(ctx context.Context, userID uuid.UUID, trustLevel int, now time.Time) (GrowthProposalWeeklyStats, error) {
	since := now.UTC().AddDate(0, 0, -7)
	stats := GrowthProposalWeeklyStats{Since: since.Format(time.RFC3339)}
	proposals, err := s.List(ctx, userID, trustLevel, "")
	if err != nil {
		return stats, err
	}
	for _, proposal := range proposals {
		createdAt, err := time.Parse(time.RFC3339, proposal.CreatedAt)
		if err != nil || createdAt.Before(since) {
			continue
		}
		stats.NewProposals++
		switch proposal.Status {
		case "accepted":
			stats.Accepted++
		case "dismissed":
			stats.Dismissed++
		case "applied":
			stats.Applied++
		case "pending_review":
			stats.PendingReview++
		}
	}
	return stats, nil
}

func (s *GrowthProposalService) Accept(ctx context.Context, userID uuid.UUID, trustLevel int, id string) (GrowthProposal, error) {
	return s.updateStatus(ctx, userID, trustLevel, id, "accepted")
}

func (s *GrowthProposalService) Dismiss(ctx context.Context, userID uuid.UUID, trustLevel int, id string) (GrowthProposal, error) {
	return s.updateStatus(ctx, userID, trustLevel, id, "dismissed")
}

func (s *GrowthProposalService) Apply(ctx context.Context, userID uuid.UUID, trustLevel int, id string) (GrowthProposal, error) {
	proposal, proposalPath, err := s.findByID(ctx, userID, trustLevel, id)
	if err != nil {
		return GrowthProposal{}, err
	}
	if proposal.Status != "pending_review" && proposal.Status != "accepted" {
		return GrowthProposal{}, fmt.Errorf("proposal %q cannot be applied from status %q", id, proposal.Status)
	}
	if len(proposal.SuggestedChanges) == 0 {
		return GrowthProposal{}, fmt.Errorf("proposal %q has no suggested changes", id)
	}
	if err := validateGrowthProposalBeforeApply(proposal); err != nil {
		return GrowthProposal{}, err
	}
	var entry *models.FileTreeEntry
	content := ""
	needsTargetRead := false
	for _, change := range proposal.SuggestedChanges {
		if change.Kind != "create_candidate_skill" {
			needsTargetRead = true
			break
		}
	}
	if needsTargetRead {
		var err error
		entry, err = s.fileTree.Read(ctx, userID, proposal.TargetPath, trustLevel)
		if err != nil {
			return GrowthProposal{}, fmt.Errorf("growth proposals.Apply: read target: %w", err)
		}
		content = entry.Content
	}
	for _, change := range proposal.SuggestedChanges {
		if change.Kind == "create_candidate_skill" {
			candidatePath := strings.TrimSpace(firstNonEmpty(change.Path, proposal.TargetPath))
			candidateContent := strings.TrimSpace(change.Content)
			if candidatePath == "" || candidateContent == "" {
				return GrowthProposal{}, fmt.Errorf("create_candidate_skill requires path and content")
			}
			if !strings.HasPrefix(candidatePath, "/skills/") {
				return GrowthProposal{}, fmt.Errorf("create_candidate_skill path must be under /skills")
			}
			if _, err := s.fileTree.WriteEntry(ctx, userID, candidatePath, candidateContent+"\n", "text/markdown", models.FileTreeWriteOptions{
				Kind: "skill",
				Metadata: map[string]interface{}{
					"source":      "growth_proposal",
					"proposal_id": proposal.ID,
				},
				MinTrustLevel: models.TrustLevelFull,
			}); err != nil {
				return GrowthProposal{}, fmt.Errorf("growth proposals.Apply: create candidate skill: %w", err)
			}
			continue
		}
		next, err := applyGrowthProposalChange(content, proposal.TargetPath, change)
		if err != nil {
			return GrowthProposal{}, err
		}
		content = next
	}
	if needsTargetRead {
		if _, err := s.fileTree.WriteEntry(ctx, userID, proposal.TargetPath, content, firstNonEmpty(entry.ContentType, "text/markdown"), models.FileTreeWriteOptions{
			Kind: "skill",
			Metadata: map[string]interface{}{
				"source":      "growth_proposal",
				"proposal_id": proposal.ID,
			},
			MinTrustLevel: models.TrustLevelFull,
		}); err != nil {
			return GrowthProposal{}, fmt.Errorf("growth proposals.Apply: write target: %w", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	proposal.Status = "applied"
	proposal.UpdatedAt = now
	proposal.AppliedAt = now
	if err := s.writeProposalAtPath(ctx, userID, proposalPath, proposal); err != nil {
		return GrowthProposal{}, err
	}
	return proposal, nil
}

func validateGrowthProposalBeforeApply(proposal GrowthProposal) error {
	if strings.TrimSpace(proposal.Risk) != "" && proposal.Risk != "low" {
		return fmt.Errorf("proposal %q risk %q requires manual editing and cannot be applied automatically", proposal.ID, proposal.Risk)
	}
	targetPath := strings.TrimSpace(proposal.TargetPath)
	for _, change := range proposal.SuggestedChanges {
		switch change.Kind {
		case "append_section", "add_verification_note":
			if strings.TrimSpace(change.Content) == "" {
				return fmt.Errorf("%s requires content", change.Kind)
			}
			if targetPath == "" || !strings.HasSuffix(targetPath, ".md") {
				return fmt.Errorf("%s requires a markdown target file", change.Kind)
			}
		case "update_frontmatter_field":
			field := strings.TrimSpace(change.Field)
			if field != "description" && field != "when_to_use" {
				return fmt.Errorf("frontmatter field %q is not allowed for automatic apply", field)
			}
			if strings.TrimSpace(change.Value) == "" {
				return fmt.Errorf("update_frontmatter_field requires value")
			}
			if targetPath == "" || !strings.HasSuffix(targetPath, ".md") {
				return fmt.Errorf("update_frontmatter_field requires a markdown target file")
			}
		case "create_candidate_skill":
			candidatePath := strings.TrimSpace(firstNonEmpty(change.Path, targetPath))
			if !strings.HasPrefix(candidatePath, "/skills/_candidates/") || !strings.HasSuffix(candidatePath, "/SKILL.md") {
				return fmt.Errorf("create_candidate_skill can only write /skills/_candidates/<name>/SKILL.md")
			}
			if strings.TrimSpace(change.Content) == "" {
				return fmt.Errorf("create_candidate_skill requires content")
			}
		default:
			return fmt.Errorf("unsupported proposal change kind %q", change.Kind)
		}
	}
	return nil
}

func (s *GrowthProposalService) updateStatus(ctx context.Context, userID uuid.UUID, trustLevel int, id, status string) (GrowthProposal, error) {
	proposal, proposalPath, err := s.findByID(ctx, userID, trustLevel, id)
	if err != nil {
		return GrowthProposal{}, err
	}
	if proposal.Status == "applied" {
		return GrowthProposal{}, fmt.Errorf("proposal %q has already been applied", id)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	proposal.Status = status
	proposal.UpdatedAt = now
	if status == "dismissed" {
		proposal.DismissedAt = now
	}
	if err := s.writeProposalAtPath(ctx, userID, proposalPath, proposal); err != nil {
		return GrowthProposal{}, err
	}
	return proposal, nil
}

func (s *GrowthProposalService) findByID(ctx context.Context, userID uuid.UUID, trustLevel int, id string) (GrowthProposal, string, error) {
	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return GrowthProposal{}, "", fmt.Errorf("proposal id is required")
	}
	snapshot, err := s.fileTree.Snapshot(ctx, userID, GrowthProposalRoot, trustLevel)
	if err != nil {
		if errors.Is(err, ErrEntryNotFound) {
			return GrowthProposal{}, "", ErrEntryNotFound
		}
		return GrowthProposal{}, "", fmt.Errorf("growth proposals.findByID: snapshot: %w", err)
	}
	for _, entry := range snapshot.Entries {
		if entry.IsDirectory || !strings.HasSuffix(entry.Path, ".json") {
			continue
		}
		var proposal GrowthProposal
		if err := json.Unmarshal([]byte(entry.Content), &proposal); err != nil {
			continue
		}
		if proposal.ID == cleanID {
			return proposal, entry.Path, nil
		}
	}
	return GrowthProposal{}, "", ErrEntryNotFound
}

func (s *GrowthProposalService) writeProposal(ctx context.Context, userID uuid.UUID, runDate string, proposal GrowthProposal) error {
	proposalPath := path.Join(GrowthProposalRoot, runDate, proposal.ID+".json")
	if _, _, err := s.findByID(ctx, userID, models.TrustLevelFull, proposal.ID); err == nil {
		return nil
	}
	if err := s.writeProposalAtPath(ctx, userID, proposalPath, proposal); err != nil {
		return err
	}
	mdPath := strings.TrimSuffix(proposalPath, ".json") + ".md"
	_, err := s.fileTree.WriteEntry(ctx, userID, mdPath, renderGrowthProposalMarkdown(proposal), "text/markdown", models.FileTreeWriteOptions{
		Kind: "growth_proposal_note",
		Metadata: map[string]interface{}{
			"source":      "learning_engine",
			"proposal_id": proposal.ID,
			"status":      proposal.Status,
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	return err
}

func (s *GrowthProposalService) writeProposalAtPath(ctx context.Context, userID uuid.UUID, proposalPath string, proposal GrowthProposal) error {
	data, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = s.fileTree.WriteEntry(ctx, userID, proposalPath, string(data), "application/json", models.FileTreeWriteOptions{
		Kind: "growth_proposal",
		Metadata: map[string]interface{}{
			"source":        "learning_engine",
			"proposal_id":   proposal.ID,
			"proposal_type": proposal.Type,
			"status":        proposal.Status,
			"source_run_id": proposal.SourceRunID,
		},
		MinTrustLevel: models.TrustLevelFull,
	})
	return err
}

func applyGrowthProposalChange(content, targetPath string, change GrowthProposalChange) (string, error) {
	switch change.Kind {
	case "append_section", "add_verification_note":
		heading := strings.TrimSpace(firstNonEmpty(change.Heading, "Notes"))
		body := strings.TrimSpace(change.Content)
		if body == "" {
			return "", fmt.Errorf("%s requires content", change.Kind)
		}
		if strings.Contains(strings.ToLower(content), strings.ToLower("## "+heading)) {
			return content, nil
		}
		return strings.TrimRight(content, "\n") + "\n\n## " + heading + "\n\n" + body + "\n", nil
	case "update_frontmatter_field":
		field := strings.TrimSpace(change.Field)
		value := strings.TrimSpace(change.Value)
		if field == "" || value == "" {
			return "", fmt.Errorf("update_frontmatter_field requires field and value")
		}
		return updateSimpleFrontmatter(content, field, value), nil
	case "create_candidate_skill":
		return content, nil
	default:
		return "", fmt.Errorf("unsupported proposal change kind %q", change.Kind)
	}
}

func updateSimpleFrontmatter(content, field, value string) string {
	lines := strings.Split(content, "\n")
	line := field + ": " + value
	if len(lines) >= 2 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				lines = append(lines[:i], append([]string{line}, lines[i:]...)...)
				return strings.Join(lines, "\n")
			}
			if key, _, ok := strings.Cut(lines[i], ":"); ok && strings.TrimSpace(key) == field {
				lines[i] = line
				return strings.Join(lines, "\n")
			}
		}
	}
	return "---\n" + line + "\n---\n\n" + strings.TrimLeft(content, "\n")
}

func renderGrowthProposalMarkdown(proposal GrowthProposal) string {
	var b strings.Builder
	b.WriteString("# Growth Proposal\n\n")
	b.WriteString("- ID: ")
	b.WriteString(proposal.ID)
	b.WriteString("\n- 状态: ")
	b.WriteString(proposal.Status)
	b.WriteString("\n- 类型: ")
	b.WriteString(proposal.Type)
	b.WriteString("\n- 目标: ")
	b.WriteString(proposal.TargetPath)
	b.WriteString("\n- 风险: ")
	b.WriteString(proposal.Risk)
	b.WriteString("\n- 来源运行: ")
	b.WriteString(proposal.SourceRunID)
	b.WriteString("\n\n## 原因\n\n")
	b.WriteString(proposal.Reason)
	b.WriteString("\n\n## 建议变更\n\n")
	for _, change := range proposal.SuggestedChanges {
		b.WriteString("- ")
		b.WriteString(change.Kind)
		if change.Field != "" {
			b.WriteString(" `")
			b.WriteString(change.Field)
			b.WriteString("`")
		}
		if change.Heading != "" {
			b.WriteString(" / ")
			b.WriteString(change.Heading)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func growthProposalID(runDate, prefix, targetPath string) string {
	clean := growthProposalSlug(targetPath)
	if len(clean) > 48 {
		clean = clean[:48]
		clean = strings.Trim(clean, "-")
	}
	return "proposal-" + runDate + "-" + prefix + "-" + clean
}

func growthProposalSlug(value string) string {
	clean := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("/", "-", "\\", "-", "_", "-", " ", "-", ".", "-", ":", "-", "#", "-", "?", "-")
	clean = replacer.Replace(clean)
	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-')
	})
	clean = strings.Join(parts, "-")
	for strings.Contains(clean, "--") {
		clean = strings.ReplaceAll(clean, "--", "-")
	}
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "candidate-skill"
	}
	if len(clean) > 48 {
		clean = strings.Trim(clean[:48], "-")
	}
	return clean
}

func renderCandidateSkillMarkdown(query, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(slug)
	b.WriteString("\n")
	b.WriteString("description: Candidate Skill proposed from a new request.\n")
	b.WriteString("when_to_use: Use this candidate after reviewing and completing its workflow.\n")
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(slug)
	b.WriteString("\n\n")
	b.WriteString("## Source request\n\n")
	b.WriteString(query)
	b.WriteString("\n\n")
	b.WriteString("## Workflow\n\n")
	b.WriteString("- Review the request and fill in the repeatable steps.\n")
	b.WriteString("- Add required tools, scripts, dependencies, and verification steps before assigning this Skill to an Agent.\n")
	b.WriteString("\n## Verification\n\n")
	b.WriteString("- Preview the candidate Skill locally before syncing it to Claude Code or Codex.\n")
	return b.String()
}
