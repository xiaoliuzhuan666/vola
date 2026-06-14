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

func TestTeamSkillRollbackZipSlipPrevention(t *testing.T) {
	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}

	// Create a token with write:skills scope
	skillsToken, err := store.CreateToken(ctx, user.ID, "skills-test", []string{models.ScopeWriteSkills}, models.TrustLevelWork, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken skills-test: %v", err)
	}

	// Create a team
	status, teamResp := doJSON(t, http.MethodPost, ts.URL+"/api/teams", adminToken, []byte(`{
		"slug": "test-team-zip-slip",
		"name": "Test Team Zip Slip"
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

	personalSkillDir := "/skills/prevent-zip-slip"

	// Prepare subscriptions file
	subDoc := teamSkillSubscriptionsDocument{
		Version: "vola.team-skill-subscriptions/v1",
		Subscriptions: []teamSkillSubscription{
			{
				TeamID:     teamID,
				SourcePath: "/skills/team-source",
				TargetPath: personalSkillDir,
			},
		},
	}
	subDocData, _ := json.Marshal(subDoc)
	_, err = store.WriteEntry(ctx, user.ID, "/settings/team-skill-subscriptions.json", string(subDocData), "application/json", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write subscription document: %v", err)
	}

	// Create a malicious zip file containing path traversal
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	
	// Valid file
	w, _ := zw.Create("SKILL.md")
	_, _ = w.Write([]byte("valid skill file"))
	
	// Malicious file 1: relative traversal
	w, _ = zw.Create("../malicious.txt")
	_, _ = w.Write([]byte("malicious content"))

	// Malicious file 2: absolute path traversal
	w, _ = zw.Create("/skills/malicious_abs.txt")
	_, _ = w.Write([]byte("malicious absolute content"))

	// Malicious file 3: nested relative traversal
	w, _ = zw.Create("nested/../../malicious_nested.txt")
	_, _ = w.Write([]byte("malicious nested content"))

	_ = zw.Close()

	backupFilePath := "/settings/team-skill-backups/prevent-zip-slip/20260613T120000Z-backup.zip"
	_, err = store.WriteBinaryEntry(ctx, user.ID, backupFilePath, zipBuf.Bytes(), "application/zip", models.FileTreeWriteOptions{})
	if err != nil {
		t.Fatalf("write backup ZIP: %v", err)
	}

	// Call rollback API
	rollbackReq := skillRollbackRequest{
		TargetPath:     personalSkillDir,
		BackupFilePath: backupFilePath,
	}
	rollbackReqData, _ := json.Marshal(rollbackReq)
	status, rollbackResp := doJSON(t, http.MethodPost, ts.URL+"/api/skills/team-subscriptions/rollback", skillsToken.Token, rollbackReqData)
	if status != http.StatusBadRequest || rollbackResp.OK {
		t.Fatalf("expected rollback to fail with 400 Bad Request on Zip Slip pre-scan, got status=%d body=%+v", status, rollbackResp)
	}

	// Verify that SKILL.md was NOT created/restored
	_, err = store.Read(ctx, user.ID, personalSkillDir+"/SKILL.md", models.TrustLevelWork)
	if err == nil {
		t.Error("expected SKILL.md not to be created due to abort")
	}

	// Verify that traversal files were NOT created
	_, err = store.Read(ctx, user.ID, "/skills/malicious.txt", models.TrustLevelWork)
	if err == nil {
		t.Error("Zip Slip vulnerability: /skills/malicious.txt was created")
	}

	_, err = store.Read(ctx, user.ID, "/skills/malicious_abs.txt", models.TrustLevelWork)
	if err == nil {
		t.Error("Zip Slip vulnerability: /skills/malicious_abs.txt was created")
	}

	_, err = store.Read(ctx, user.ID, "/skills/malicious_nested.txt", models.TrustLevelWork)
	if err == nil {
		t.Error("Zip Slip vulnerability: /skills/malicious_nested.txt was created")
	}
}

func TestTeamSkillRollbackPreScanAndZipBombDefense(t *testing.T) {
	// Save original limits and restore them after test
	oldMaxSingle := MaxRollbackSingleFileSize
	oldMaxTotal := MaxRollbackTotalSize
	defer func() {
		MaxRollbackSingleFileSize = oldMaxSingle
		MaxRollbackTotalSize = oldMaxTotal
	}()

	// Limit to extremely small sizes for easy testing
	MaxRollbackSingleFileSize = 100 // 100 bytes
	MaxRollbackTotalSize = 200      // 200 bytes

	ts, store, adminToken, _, _ := newTestHTTPServer(t)
	ctx := context.Background()
	user, err := store.EnsureOwner(ctx)
	if err != nil {
		t.Fatalf("EnsureOwner: %v", err)
	}

	skillsToken, err := store.CreateToken(ctx, user.ID, "skills-test", []string{models.ScopeWriteSkills}, models.TrustLevelWork, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// 1. Prepare original skill files
	personalSkillDir := "/skills/pre-scan-skill"
	_, _ = store.WriteEntry(ctx, user.ID, personalSkillDir+"/SKILL.md", "# Original SKILL\n", "text/markdown", models.FileTreeWriteOptions{})
	_, _ = store.WriteEntry(ctx, user.ID, personalSkillDir+"/original.py", "print('original')\n", "text/x-python", models.FileTreeWriteOptions{})

	// Create a team
	status, teamResp := doJSON(t, http.MethodPost, ts.URL+"/api/teams", adminToken, []byte(`{
		"slug": "test-team-pre-scan",
		"name": "Test Team Pre-Scan"
	}`))
	if status != http.StatusCreated || !teamResp.OK {
		t.Fatalf("create team failed")
	}
	var teamPayload struct {
		Team models.Team `json:"team"`
	}
	_ = json.Unmarshal(teamResp.Data, &teamPayload)
	teamID := teamPayload.Team.ID.String()

	// Prepare subscriptions file
	subDoc := teamSkillSubscriptionsDocument{
		Version: "vola.team-skill-subscriptions/v1",
		Subscriptions: []teamSkillSubscription{
			{
				TeamID:     teamID,
				SourcePath: "/skills/team-source",
				TargetPath: personalSkillDir,
			},
		},
	}
	subDocData, _ := json.Marshal(subDoc)
	_, _ = store.WriteEntry(ctx, user.ID, "/settings/team-skill-subscriptions.json", string(subDocData), "application/json", models.FileTreeWriteOptions{})

	// 2. Test Case A: Zip Bomb exceeding single file limit (101 bytes)
	var zipBufA bytes.Buffer
	zwA := zip.NewWriter(&zipBufA)
	wA, _ := zwA.Create("huge_file.txt")
	_, _ = wA.Write(make([]byte, 101)) // Write 101 bytes (exceeds 100 limit)
	_ = zwA.Close()

	backupPathA := "/settings/team-skill-backups/pre-scan-skill/20260613T120000Z-bomb-backup.zip"
	_, _ = store.WriteBinaryEntry(ctx, user.ID, backupPathA, zipBufA.Bytes(), "application/zip", models.FileTreeWriteOptions{})

	rollbackReqA := skillRollbackRequest{
		TargetPath:     personalSkillDir,
		BackupFilePath: backupPathA,
	}
	rollbackReqDataA, _ := json.Marshal(rollbackReqA)
	status, rollbackRespA := doJSON(t, http.MethodPost, ts.URL+"/api/skills/team-subscriptions/rollback", skillsToken.Token, rollbackReqDataA)

	// Verify rollback failed and returned BadRequest
	if status != http.StatusBadRequest || rollbackRespA.OK {
		t.Errorf("expected rollback to fail with 400 Bad Request, got status %d, OK=%t", status, rollbackRespA.OK)
	}

	// Verify that original files were NOT deleted
	skillMDA, err := store.Read(ctx, user.ID, personalSkillDir+"/SKILL.md", models.TrustLevelWork)
	if err != nil || skillMDA.Content != "# Original SKILL\n" {
		t.Errorf("original SKILL.md was deleted or modified: err=%v", err)
	}
	originalPyA, err := store.Read(ctx, user.ID, personalSkillDir+"/original.py", models.TrustLevelWork)
	if err != nil || originalPyA.Content != "print('original')\n" {
		t.Errorf("original original.py was deleted or modified: err=%v", err)
	}

	// 3. Test Case B: Zip Slip detected during Pre-Scan
	var zipBufB bytes.Buffer
	zwB := zip.NewWriter(&zipBufB)
	wB, _ := zwB.Create("../escaped_pre_scan.txt")
	_, _ = wB.Write([]byte("escaped"))
	_ = zwB.Close()

	backupPathB := "/settings/team-skill-backups/pre-scan-skill/20260613T120000Z-slip-backup.zip"
	_, _ = store.WriteBinaryEntry(ctx, user.ID, backupPathB, zipBufB.Bytes(), "application/zip", models.FileTreeWriteOptions{})

	rollbackReqB := skillRollbackRequest{
		TargetPath:     personalSkillDir,
		BackupFilePath: backupPathB,
	}
	rollbackReqDataB, _ := json.Marshal(rollbackReqB)
	status, rollbackRespB := doJSON(t, http.MethodPost, ts.URL+"/api/skills/team-subscriptions/rollback", skillsToken.Token, rollbackReqDataB)

	// Verify rollback failed and returned BadRequest
	if status != http.StatusBadRequest || rollbackRespB.OK {
		t.Errorf("expected Zip Slip rollback to fail with 400, got status %d", status)
	}

	// Verify that original files were still NOT deleted
	skillMDB, err := store.Read(ctx, user.ID, personalSkillDir+"/SKILL.md", models.TrustLevelWork)
	if err != nil || skillMDB.Content != "# Original SKILL\n" {
		t.Errorf("original SKILL.md was deleted under Zip Slip case: err=%v", err)
	}
}
