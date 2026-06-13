package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/agi-bar/vola/internal/models"
)

func TestSkillSubscriptionsDiffAndRollback(t *testing.T) {
	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}

	// Create a token with read:skills and write:skills scopes
	skillsToken, err := store.CreateToken(ctx, user.ID, "skills-test", []string{models.ScopeReadSkills, models.ScopeWriteSkills}, models.TrustLevelWork, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken skills-test: %v", err)
	}

	// 1. Create a team
	status, teamResp := doJSON(t, http.MethodPost, ts.URL+"/api/teams", adminToken, []byte(`{
		"slug": "test-team",
		"name": "Test Team"
	}`))
	if status != http.StatusCreated || !teamResp.OK {
		t.Fatalf("create team failed: status=%d body=%+v", status, teamResp)
	}
	var teamPayload struct {
		Team models.Team `json:"team"`
	}
	if err := json.Unmarshal(teamResp.Data, &teamPayload); err != nil {
		t.Fatalf("decode team: %v", err)
	}

	teamID := teamPayload.Team.ID.String()
	teamHubUserID := teamPayload.Team.HubUserID

	// 2. Prepare team skill files (source_path)
	teamSkillDir := "/skills/demo-skill"
	_, err = store.WriteEntry(ctx, teamHubUserID, teamSkillDir+"/SKILL.md", "# Demo Skill Team\n", "text/markdown", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write team SKILL.md: %v", err)
	}
	_, err = store.WriteEntry(ctx, teamHubUserID, teamSkillDir+"/helper.py", "print('team helper')\n", "text/x-python", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write team helper.py: %v", err)
	}
	_, err = store.WriteEntry(ctx, teamHubUserID, teamSkillDir+"/config.json", `{"team": true}`, "application/json", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write team config.json: %v", err)
	}

	// 3. Prepare personal skill files (target_path)
	personalSkillDir := "/skills/my-demo-skill"
	_, err = store.WriteEntry(ctx, user.ID, personalSkillDir+"/SKILL.md", "# Demo Skill Personal\n", "text/markdown", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write personal SKILL.md: %v", err)
	}
	_, err = store.WriteEntry(ctx, user.ID, personalSkillDir+"/helper.py", "print('team helper')\n", "text/x-python", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write personal helper.py: %v", err)
	}
	_, err = store.WriteEntry(ctx, user.ID, personalSkillDir+"/local_only.txt", "my data\n", "text/plain", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write personal local_only.txt: %v", err)
	}

	// 4. Call handleSkillSubscriptionsDiff and check status
	reqBody := skillDiffRequest{
		TeamID:     teamID,
		SourcePath: teamSkillDir,
		TargetPath: personalSkillDir,
	}
	reqData, _ := json.Marshal(reqBody)
	status, diffResp := doJSON(t, http.MethodPost, ts.URL+"/api/skills/team-subscriptions/diff", skillsToken.Token, reqData)
	if status != http.StatusOK || !diffResp.OK {
		t.Fatalf("diff API failed: status=%d body=%+v", status, diffResp)
	}

	var diffResult skillDiffResponse
	if err := json.Unmarshal(diffResp.Data, &diffResult); err != nil {
		t.Fatalf("unmarshal diff data: %v", err)
	}

	statusMap := make(map[string]string)
	for _, item := range diffResult.Files {
		statusMap[item.RelPath] = item.Status
	}

	if statusMap["SKILL.md"] != "modified" {
		t.Errorf("expected SKILL.md modified, got %s", statusMap["SKILL.md"])
	}
	if statusMap["helper.py"] != "unchanged" {
		t.Errorf("expected helper.py unchanged, got %s", statusMap["helper.py"])
	}
	if statusMap["config.json"] != "added" {
		t.Errorf("expected config.json added, got %s", statusMap["config.json"])
	}
	if statusMap["local_only.txt"] != "deleted" {
		t.Errorf("expected local_only.txt deleted, got %s", statusMap["local_only.txt"])
	}

	// 5. Simulate zip backup file
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("SKILL.md")
	_, _ = w.Write([]byte("# Backed Up Skill\n"))
	w, _ = zw.Create("helper.py")
	_, _ = w.Write([]byte("print('backed up helper')\n"))
	_ = zw.Close()

	backupFilePath := "/settings/team-skill-backups/my-demo-skill/20260613T120000Z-backup.zip"
	_, err = store.WriteBinaryEntry(ctx, user.ID, backupFilePath, zipBuf.Bytes(), "application/zip", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write backup ZIP: %v", err)
	}

	// Initialize subscriptions file
	subDoc := teamSkillSubscriptionsDocument{
		Version: "vola.team-skill-subscriptions/v1",
		Subscriptions: []teamSkillSubscription{
			{
				TeamID:     teamID,
				SourcePath: teamSkillDir,
				TargetPath: personalSkillDir,
			},
		},
	}
	subDocData, _ := json.Marshal(subDoc)
	_, err = store.WriteEntry(ctx, user.ID, "/settings/team-skill-subscriptions.json", string(subDocData), "application/json", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write subscription document: %v", err)
	}

	// 6. Call handleSkillSubscriptionsRollback
	rollbackReq := skillRollbackRequest{
		TargetPath:     personalSkillDir,
		BackupFilePath: backupFilePath,
	}
	rollbackReqData, _ := json.Marshal(rollbackReq)
	status, rollbackResp := doJSON(t, http.MethodPost, ts.URL+"/api/skills/team-subscriptions/rollback", skillsToken.Token, rollbackReqData)
	if status != http.StatusOK || !rollbackResp.OK {
		t.Fatalf("rollback API failed: status=%d body=%+v", status, rollbackResp)
	}

	var rollbackResult skillRollbackResponse
	if err := json.Unmarshal(rollbackResp.Data, &rollbackResult); err != nil {
		t.Fatalf("unmarshal rollback data: %v", err)
	}

	if !rollbackResult.Success || rollbackResult.Restored != 2 {
		t.Errorf("rollback failed, restored=%d, success=%t", rollbackResult.Restored, rollbackResult.Success)
	}

	// Verify restored files
	skillMD, err := store.Read(ctx, user.ID, personalSkillDir+"/SKILL.md", models.TrustLevelWork)
	if err != nil || skillMD.Content != "# Backed Up Skill\n" {
		t.Errorf("SKILL.md was not restored correctly: content=%q, err=%v", skillMD.Content, err)
	}

	helperPy, err := store.Read(ctx, user.ID, personalSkillDir+"/helper.py", models.TrustLevelWork)
	if err != nil || helperPy.Content != "print('backed up helper')\n" {
		t.Errorf("helper.py was not restored correctly: content=%q, err=%v", helperPy.Content, err)
	}

	// local_only.txt should be deleted
	_, err = store.Read(ctx, user.ID, personalSkillDir+"/local_only.txt", models.TrustLevelWork)
	if err == nil {
		t.Error("local_only.txt was not deleted during rollback")
	}

	// Check subscriptions fingerprint update
	subNode, err := store.Read(ctx, user.ID, "/settings/team-skill-subscriptions.json", models.TrustLevelWork)
	if err != nil {
		t.Fatalf("read subscription config: %v", err)
	}
	var subUpdated teamSkillSubscriptionsDocument
	if err := json.Unmarshal([]byte(subNode.Content), &subUpdated); err != nil {
		t.Fatalf("unmarshal updated subscription config: %v", err)
	}

	if len(subUpdated.Subscriptions) != 1 || subUpdated.Subscriptions[0].SourceFingerprint == "" {
		t.Errorf("fingerprint was not updated: %+v", subUpdated)
	}
}
