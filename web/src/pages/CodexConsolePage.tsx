import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  api,
  type CodexConsoleAutomation,
  type CodexConsoleArtifact,
  type CodexConsoleArtifactRegistrySaveResponse,
  type CodexConsoleArtifactRegistrySummary,
  type CodexConsoleHandoverItem,
  type CodexConsoleHandoverSaveResponse,
  type CodexConsoleHandoverSummary,
  type CodexConsoleHookRisk,
  type CodexConsoleMemoryCandidate,
  type CodexConsoleMemoryConflictResolution,
  type CodexConsoleResponse,
  type CodexConsoleRun,
  type CodexConsoleSkillCandidate,
  type CodexConsoleSkillCandidateAssignPreviewResponse,
  type CodexConsoleSkillCandidateSaveResponse,
  type CodexConsoleThread,
  type LocalSkillSyncAgentPlan,
  type SkillAgentTarget,
} from "../api";
import { useI18n } from "../i18n";

type ConsoleView = "brief" | "threads" | "runs" | "automations" | "artifacts" | "hooks" | "memory" | "handovers" | "skill_candidates";
type SkillCandidateAgentID = "claude-code" | "codex" | "cursor" | "gemini-cli";
type SkillCandidateAgentOption = {
  id: SkillCandidateAgentID;
  name: string;
  modeZh: string;
  modeEn: string;
  noteZh: string;
  noteEn: string;
};

const SKILL_CANDIDATE_AGENT_OPTIONS: SkillCandidateAgentOption[] = [
  {
    id: "codex",
    name: "Codex",
    modeZh: "本地同步预览",
    modeEn: "Local sync preview",
    noteZh: "面向 ~/.agents/skills，当前不会写入本地。",
    noteEn: "Targets ~/.agents/skills; nothing is written locally here.",
  },
  {
    id: "claude-code",
    name: "Claude Code",
    modeZh: "本地同步预览",
    modeEn: "Local sync preview",
    noteZh: "面向 ~/.claude/skills，当前不会写入本地。",
    noteEn: "Targets ~/.claude/skills; nothing is written locally here.",
  },
  {
    id: "cursor",
    name: "Cursor",
    modeZh: "导出预览",
    modeEn: "Export preview",
    noteZh: "保留分配关系，生成规则素材导出预览。",
    noteEn: "Keeps the assignment and previews an export package.",
  },
  {
    id: "gemini-cli",
    name: "Gemini CLI",
    modeZh: "导出预览",
    modeEn: "Export preview",
    noteZh: "保留分配关系，供 GEMINI.md 等说明文件引用。",
    noteEn: "Keeps the assignment for manual use from GEMINI.md-style guidance.",
  },
];

function getAgentOptionsFromTarget(target: SkillAgentTarget, tx: (zh: string, en: string) => string): SkillCandidateAgentOption {
  let modeZh = "导出预览";
  let modeEn = "Export preview";
  if (target.supports_apply) {
    modeZh = "本地同步预览";
    modeEn = "Local sync preview";
  }

  let noteZh = "";
  let noteEn = "";
  if (target.supports_apply) {
    const pathHint = target.install_path_hint || "";
    noteZh = `面向 ${pathHint}，当前不会写入本地。`;
    noteEn = `Targets ${pathHint}; nothing is written locally here.`;
  } else {
    if (target.id === "cursor") {
      noteZh = "保留分配关系，生成规则素材导出预览。";
      noteEn = "Keeps the assignment and previews an export package.";
    } else if (target.id === "gemini-cli") {
      noteZh = "保留分配关系，供 GEMINI.md 等说明文件引用。";
      noteEn = "Keeps the assignment for manual use from GEMINI.md-style guidance.";
    } else {
      noteZh = target.auto_apply_reason || "保留分配关系。";
      noteEn = target.auto_apply_reason || "Keeps the assignment.";
    }
  }

  return {
    id: target.id as SkillCandidateAgentID,
    name: target.name,
    modeZh,
    modeEn,
    noteZh,
    noteEn,
  };
}

function formatDateTime(value: string | undefined, locale: "zh-CN" | "en") {
  if (!value) return "—";
  const time = new Date(value);
  if (Number.isNaN(time.getTime())) return value;
  return time.toLocaleString(locale === "zh-CN" ? "zh-CN" : "en-US");
}

function metricLabel(value: number | undefined) {
  return Number.isFinite(value) ? String(value || 0) : "0";
}

function countLabel(value: number | undefined) {
  const safeValue = Number.isFinite(value) ? value || 0 : 0;
  return safeValue.toLocaleString("en-US");
}

function sourceShortPath(path: string | undefined) {
  if (!path) return "";
  const parts = path.split("/").filter(Boolean);
  if (parts.length <= 4) return path;
  return `…/${parts.slice(-4).join("/")}`;
}

function statusTone(status: string | undefined) {
  const value = (status || "").toLowerCase();
  if (value === "active" || value === "ok" || value === "succeeded" || value === "synced" || value === "accepted" || value === "low") return "ok";
  if (value === "paused" || value === "warning" || value === "deferred" || value === "review_required" || value === "manual_required" || value === "possible" || value === "medium") return "warn";
  if (value === "failed" || value === "error" || value === "high") return "bad";
  return "neutral";
}

function confidenceLabel(value: number | undefined) {
  return Number.isFinite(value) ? `${Math.round((value || 0) * 100)}%` : "—";
}

function confidenceTone(value: number | undefined) {
  if (!Number.isFinite(value)) return "neutral";
  if ((value || 0) >= 0.75) return "ok";
  if ((value || 0) >= 0.6) return "warn";
  return "neutral";
}

function cleanPreviewText(value: string | undefined) {
  return (value || "")
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line && !line.includes("\uFFFD"))
    .join("\n")
    .trim();
}

function shortPreview(value: string | undefined, limit = 150) {
  const text = cleanPreviewText(value);
  if (!text) return "";
  const runes = Array.from(text);
  if (runes.length <= limit) return text;
  return `${runes.slice(0, limit - 1).join("").trim()}…`;
}

function isMemoryActionable(item: CodexConsoleMemoryCandidate) {
  const status = (item.review_status || "review_required").toLowerCase();
  return status === "review_required" || status === "accepted" || status === "deferred";
}

function skillCandidateSaveMessage(
  result: CodexConsoleSkillCandidateSaveResponse,
  tx: (zh: string, en: string) => string,
) {
  if (result.status === "exists") {
    return result.message || tx("目标路径已有 Skill 草稿，Vola 没有覆盖。", "A skill draft already exists at the target path; Vola did not overwrite it.");
  }
  if (result.message?.includes("already saved")) {
    return tx(`Skill 草稿已存在：${result.skill_path}`, `Skill draft already exists: ${result.skill_path}`);
  }
  if (result.edited) {
    return tx(`已保存编辑后的 Skill 草稿：${result.skill_path}`, `Saved edited skill draft: ${result.skill_path}`);
  }
  return tx(`已保存为 Vola Skill 草稿：${result.skill_path}`, `Saved as Vola skill draft: ${result.skill_path}`);
}

function skillCandidateAssignMessage(
  result: CodexConsoleSkillCandidateAssignPreviewResponse,
  tx: (zh: string, en: string) => string,
) {
  if (result.status !== "assigned") {
    return result.message || tx("请先保存 Skill 草稿，再生成分配预览。", "Save the skill draft before creating an assignment preview.");
  }
  const agents = result.sync_preview?.agents || [];
  const managed = agents.filter((agent) => agent.supported).length;
  const exportOnly = agents.filter((agent) => !agent.supported || agent.support_status === "export_only").length;
  const conflict = agents.reduce((total, agent) => total + (agent.summary.conflict || 0), 0);
  return tx(
    `已加入 ${result.agent_ids.length || agents.length} 个 Agent 分配：${managed} 个可本地同步预览，${exportOnly} 个导出预览，冲突 ${conflict}。尚未写入本地目录。`,
    `Assigned to ${result.agent_ids.length || agents.length} Agent targets: ${managed} local sync preview, ${exportOnly} export preview, ${conflict} conflict. Nothing was written locally.`,
  );
}

function skillCandidateAgentName(agentID: string) {
  return SKILL_CANDIDATE_AGENT_OPTIONS.find((agent) => agent.id === agentID)?.name || agentID;
}

function orderedSkillCandidateAgentIDs(agentIDs: string[]) {
  const selected = new Set(agentIDs);
  return SKILL_CANDIDATE_AGENT_OPTIONS.filter((agent) => selected.has(agent.id)).map((agent) => agent.id);
}

function isArchivedSkillCandidate(item: CodexConsoleSkillCandidate) {
  return (item.status || "").toLowerCase() === "archived";
}

function skillCandidateStatusLabel(item: CodexConsoleSkillCandidate, tx: (zh: string, en: string) => string) {
  const status = (item.status || "").toLowerCase();
  if (!item.skill_path) return tx("未保存", "Not saved");
  if (status === "ready") return tx("已确认", "Ready");
  if (status === "archived") return tx("已归档", "Archived");
  if (item.edited) return tx("已编辑草稿", "Edited draft");
  return tx("Vola 草稿", "Vola draft");
}

function skillCandidateStatusTone(item: CodexConsoleSkillCandidate) {
  const status = (item.status || "").toLowerCase();
  if (status === "ready") return "ok";
  if (status === "archived") return "neutral";
  if (item.skill_path) return "warn";
  return "neutral";
}

function syncActionLabel(action: string, tx: (zh: string, en: string) => string) {
  if (action === "add") return tx("新增", "add");
  if (action === "update") return tx("更新", "update");
  if (action === "unchanged") return tx("相同", "same");
  if (action === "missing") return tx("本地多出", "extra");
  if (action === "conflict") return tx("冲突", "conflict");
  if (action === "delete") return tx("清理", "clean");
  if (action === "export") return tx("导出", "export");
  return action;
}

function handoverSaveMessage(
  result: CodexConsoleHandoverSaveResponse,
  tx: (zh: string, en: string) => string,
) {
  if (result.edited) {
    return tx(`已保存编辑后的项目交接文件：${result.path}`, `Saved edited project handover: ${result.path}`);
  }
  return result.message || tx(`已保存项目交接文件：${result.path}`, `Saved project handover: ${result.path}`);
}

function artifactRegistrySaveMessage(
  result: CodexConsoleArtifactRegistrySaveResponse,
  tx: (zh: string, en: string) => string,
) {
  if (result.message?.includes("already saved")) {
    return tx(`交付物索引已存在：${result.path}`, `Artifact registry already exists: ${result.path}`);
  }
  return result.message || tx(`已保存交付物索引：${result.path}`, `Saved artifact registry: ${result.path}`);
}

function artifactRegistryStatusLabel(
  status: string | undefined,
  tx: (zh: string, en: string) => string,
) {
  switch ((status || "not_saved").toLowerCase()) {
    case "saved":
      return tx("已保存到 Vola", "Saved to Vola");
    case "invalid":
      return tx("索引文件需要重建", "Registry file needs rebuilding");
    case "unsupported":
      return tx("索引版本不兼容", "Registry version is unsupported");
    case "exists":
      return tx("已有索引文件", "Registry file exists");
    case "failed":
      return tx("保存失败", "Save failed");
    default:
      return tx("尚未保存", "Not saved");
  }
}

function artifactRoleLabel(role: string | undefined, tx: (zh: string, en: string) => string) {
  switch ((role || "").toLowerCase()) {
    case "handoff-document":
      return tx("交接文档", "Handoff document");
    case "visual-evidence":
      return tx("视觉证据", "Visual evidence");
    case "preview-output":
      return tx("预览输出", "Preview output");
    case "structured-data":
      return tx("结构化数据", "Structured data");
    case "run-evidence":
      return tx("执行证据", "Run evidence");
    case "attachment":
      return tx("附件", "Attachment");
    default:
      return tx("项目文件", "Project file");
  }
}

function artifactHandoffPrompt(item: CodexConsoleArtifact, tx: (zh: string, en: string) => string) {
  if (item.agent_instruction) return item.agent_instruction;
  const role = artifactRoleLabel(item.role, tx);
  return tx(
    `请先查看交付物「${item.name}」，把它当作${role}使用。继续任务前说明它是否是最终交付物、验证证据还是中间文件。`,
    `Review artifact "${item.name}" as ${role}. Before continuing, explain whether it is a final deliverable, verification evidence, or intermediate file.`,
  );
}

export default function CodexConsolePage() {
  const { locale, tx } = useI18n();
  const [data, setData] = useState<CodexConsoleResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [view, setView] = useState<ConsoleView>("brief");
  const [selectedThreadID, setSelectedThreadID] = useState("");
  const [selectedRunID, setSelectedRunID] = useState("");
  const [selectedAutomationID, setSelectedAutomationID] = useState("");
  const [selectedArtifactID, setSelectedArtifactID] = useState("");
  const [selectedHookID, setSelectedHookID] = useState("");
  const [selectedMemoryID, setSelectedMemoryID] = useState("");
  const [selectedHandoverID, setSelectedHandoverID] = useState("");
  const [selectedSkillCandidateID, setSelectedSkillCandidateID] = useState("");
  const [copied, setCopied] = useState("");
  const [memorySyncing, setMemorySyncing] = useState<"" | "selected" | "all">("");
  const [memorySyncTarget, setMemorySyncTarget] = useState<"profile" | "project">("profile");
  const [memorySyncProject, setMemorySyncProject] = useState("");
  const [memoryDraftContent, setMemoryDraftContent] = useState("");
  const [memoryConflictDraft, setMemoryConflictDraft] = useState("");
  const [memoryReviewing, setMemoryReviewing] = useState("");
  const [memoryConflictResolving, setMemoryConflictResolving] = useState<"" | CodexConsoleMemoryConflictResolution>("");
  const [memorySyncMessage, setMemorySyncMessage] = useState("");
  const [memorySyncError, setMemorySyncError] = useState("");
  const [noticeTarget, setNoticeTarget] = useState<"" | "memory" | "skills" | "projects" | "platforms">("");
  const [artifactSaving, setArtifactSaving] = useState(false);
  const [handoverSaving, setHandoverSaving] = useState("");
  const [handoverDraftContent, setHandoverDraftContent] = useState("");
  const [skillSaving, setSkillSaving] = useState("");
  const [skillStatusUpdating, setSkillStatusUpdating] = useState("");
  const [skillDraftContent, setSkillDraftContent] = useState("");
  const [skillAssigning, setSkillAssigning] = useState("");
  const [skillAssignPreview, setSkillAssignPreview] = useState<CodexConsoleSkillCandidateAssignPreviewResponse | null>(null);
  const [skillAssignAgentIDs, setSkillAssignAgentIDs] = useState<SkillCandidateAgentID[]>(["codex"]);
  const [showArchivedSkillCandidates, setShowArchivedSkillCandidates] = useState(false);
  const [availableAgents, setAvailableAgents] = useState<SkillAgentTarget[]>([
    { id: "codex", name: "Codex", platform: "codex", install_path_hint: "~/.agents/skills", supports_apply: true, export_supported: true },
    { id: "claude-code", name: "Claude Code", platform: "claude-code", install_path_hint: "~/.claude/skills", supports_apply: true, export_supported: true },
    { id: "cursor", name: "Cursor", platform: "cursor", install_path_hint: ".cursor/rules", supports_apply: false, export_supported: true },
    { id: "gemini-cli", name: "Gemini CLI", platform: "gemini-cli", install_path_hint: "Export package; manual GEMINI.md integration", supports_apply: false, export_supported: true },
  ]);

  const agentOptions = useMemo(() => {
    return availableAgents.map((agent) => getAgentOptionsFromTarget(agent, tx));
  }, [availableAgents, locale]);

  const load = async (quiet = false) => {
    if (quiet) setRefreshing(true);
    else setLoading(true);
    setError("");
    try {
      const next = await api.getCodexConsole();
      setData(next);
      try {
        const assignmentsData = await api.getSkillAssignments();
        if (assignmentsData && assignmentsData.agents) {
          setAvailableAgents(assignmentsData.agents);
        }
      } catch (e) {
        console.warn("Failed to load skill assignments in CodexConsole", e);
      }
      setSelectedThreadID((current) => current || next.threads[0]?.id || "");
      setSelectedRunID((current) => current || next.runs[0]?.id || "");
      setSelectedAutomationID((current) => current || next.automations[0]?.id || "");
      setSelectedArtifactID((current) => current || next.artifacts[0]?.id || "");
      setSelectedHookID((current) => current || next.hooks[0]?.id || "");
      setSelectedMemoryID((current) => current || next.memory_candidates[0]?.id || "");
      setSelectedHandoverID((current) => current || next.handovers[0]?.id || "");
      setSelectedSkillCandidateID((current) => current || next.skill_candidates.find((item) => !isArchivedSkillCandidate(item))?.id || next.skill_candidates[0]?.id || "");
    } catch (err: any) {
      setError(err?.message || tx("加载 Codex Console 失败", "Failed to load Codex Console"));
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const selectedThread = useMemo(
    () => data?.threads.find((item) => item.id === selectedThreadID) || data?.threads[0] || null,
    [data?.threads, selectedThreadID],
  );
  const selectedRun = useMemo(
    () => data?.runs.find((item) => item.id === selectedRunID) || data?.runs[0] || null,
    [data?.runs, selectedRunID],
  );
  const selectedAutomation = useMemo(
    () => data?.automations.find((item) => item.id === selectedAutomationID) || data?.automations[0] || null,
    [data?.automations, selectedAutomationID],
  );
  const selectedArtifact = useMemo(
    () => data?.artifacts.find((item) => item.id === selectedArtifactID) || data?.artifacts[0] || null,
    [data?.artifacts, selectedArtifactID],
  );
  const selectedHook = useMemo(
    () => data?.hooks.find((item) => item.id === selectedHookID) || data?.hooks[0] || null,
    [data?.hooks, selectedHookID],
  );
  const selectedMemory = useMemo(
    () => data?.memory_candidates.find((item) => item.id === selectedMemoryID) || data?.memory_candidates[0] || null,
    [data?.memory_candidates, selectedMemoryID],
  );
  useEffect(() => {
    setMemoryDraftContent(cleanPreviewText(selectedMemory?.content));
  }, [selectedMemory?.id, selectedMemory?.content]);
  useEffect(() => {
    setMemoryConflictDraft(selectedMemory?.conflict ? buildMemoryConflictDraft(selectedMemory, tx) : "");
  }, [
    selectedMemory?.id,
    selectedMemory?.conflict?.existing_content,
    selectedMemory?.conflict?.candidate_content,
    selectedMemory?.content,
    locale,
  ]);
  const selectedHandover = useMemo(
    () => data?.handovers.find((item) => item.id === selectedHandoverID) || data?.handovers[0] || null,
    [data?.handovers, selectedHandoverID],
  );
  useEffect(() => {
    setHandoverDraftContent(selectedHandover ? buildHandoverDraft(selectedHandover, tx) : "");
  }, [
    selectedHandover?.id,
    selectedHandover?.saved_content,
    selectedHandover?.summary,
    selectedHandover?.latest_activity,
    locale,
  ]);
  const visibleSkillCandidates = useMemo(
    () => (data?.skill_candidates || []).filter((item) => showArchivedSkillCandidates || !isArchivedSkillCandidate(item)),
    [data?.skill_candidates, showArchivedSkillCandidates],
  );
  const archivedSkillCandidateCount = useMemo(
    () => (data?.skill_candidates || []).filter(isArchivedSkillCandidate).length,
    [data?.skill_candidates],
  );
  const selectedSkillCandidate = useMemo(
    () => visibleSkillCandidates.find((item) => item.id === selectedSkillCandidateID) || visibleSkillCandidates[0] || null,
    [visibleSkillCandidates, selectedSkillCandidateID],
  );
  useEffect(() => {
    setSkillDraftContent(cleanSkillDraftText(selectedSkillCandidate?.draft));
    setSkillAssignPreview(null);
  }, [selectedSkillCandidate?.id, selectedSkillCandidate?.draft]);
  const copySource = async (path: string | undefined) => {
    if (!path) return;
    await navigator.clipboard?.writeText(path);
    setCopied(path);
    window.setTimeout(() => setCopied(""), 1400);
  };

  const copyDraft = async (id: string, value: string) => {
    if (!value) return;
    const marker = `skill-draft:${id}`;
    await navigator.clipboard?.writeText(value);
    setCopied(marker);
    window.setTimeout(() => setCopied(""), 1400);
  };

  const saveArtifactRegistry = async () => {
    setArtifactSaving(true);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.saveCodexConsoleArtifacts({ overwrite: true });
      setMemorySyncMessage(artifactRegistrySaveMessage(result, tx));
      setNoticeTarget("platforms");
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("保存交付物索引失败", "Failed to save artifact registry"));
    } finally {
      setArtifactSaving(false);
    }
  };

  const saveSkillCandidate = async (id: string, draftOverride: string) => {
    if (!id) return;
    setSkillSaving(id);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.saveCodexConsoleSkillCandidate({
        id,
        draft_override: cleanSkillDraftText(draftOverride),
        overwrite: !!selectedSkillCandidate?.skill_path,
      });
      setMemorySyncMessage(skillCandidateSaveMessage(result, tx));
      setNoticeTarget("skills");
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("保存 Skill 草稿失败", "Failed to save skill draft"));
    } finally {
      setSkillSaving("");
    }
  };

  const updateSkillCandidateStatus = async (id: string, status: "draft" | "ready" | "archived") => {
    if (!id) return;
    setSkillStatusUpdating(status);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.updateCodexConsoleSkillCandidateStatus({ id, status });
      if (result.status === "not_saved") {
        setMemorySyncError(result.message || tx("请先保存 Skill 草稿。", "Save the skill draft first."));
        return;
      }
      const message =
        status === "ready"
          ? tx("已标记为可分配 Skill。", "Marked as ready for assignment.")
          : status === "archived"
            ? tx("已归档。默认列表会隐藏这条草稿，但不会删除文件。", "Archived. The default list hides this draft without deleting files.")
            : tx("已恢复为草稿状态。", "Restored to draft status.");
      setMemorySyncMessage(message);
      setNoticeTarget("skills");
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("更新 Skill 草稿状态失败", "Failed to update skill draft status"));
    } finally {
      setSkillStatusUpdating("");
    }
  };

  const toggleSkillAssignAgent = (agentID: SkillCandidateAgentID) => {
    setSkillAssignPreview(null);
    setSkillAssignAgentIDs((current) => {
      if (current.includes(agentID)) {
        if (current.length <= 1) return current;
        return current.filter((id) => id !== agentID);
      }
      return orderedSkillCandidateAgentIDs([...current, agentID]) as SkillCandidateAgentID[];
    });
  };

  const assignSkillCandidateToAgents = async (id: string, agentIDs: string[]) => {
    if (!id) return;
    const selectedAgentIDs = orderedSkillCandidateAgentIDs(agentIDs);
    if (selectedAgentIDs.length === 0) {
      setMemorySyncError(tx("至少选择一个 Agent。", "Select at least one Agent."));
      return;
    }
    setSkillAssigning(id);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.assignCodexConsoleSkillCandidate({ id, agent_ids: selectedAgentIDs });
      setSkillAssignPreview(result);
      setMemorySyncMessage(skillCandidateAssignMessage(result, tx));
      setNoticeTarget("skills");
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("生成 Skill 分配预览失败", "Failed to create skill assignment preview"));
    } finally {
      setSkillAssigning("");
    }
  };

  const saveHandover = async (id: string, contentOverride: string) => {
    if (!id) return;
    setHandoverSaving(id);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.saveCodexConsoleHandover({ id, content_override: cleanHandoverDraftText(contentOverride) });
      setMemorySyncMessage(handoverSaveMessage(result, tx));
      setNoticeTarget("projects");
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("保存项目交接文件失败", "Failed to save project handover"));
    } finally {
      setHandoverSaving("");
    }
  };

  const syncMemoryCandidates = async (all: boolean) => {
    if (!all && !selectedMemory?.id) return;
    const project = memorySyncProject.trim();
    const draft = memoryDraftContent.trim();
    const original = cleanPreviewText(selectedMemory?.content).trim();
    if (memorySyncTarget === "project" && !project) {
      setMemorySyncError(tx("同步到项目时需要填写项目名。", "Project name is required when syncing to project context."));
      return;
    }
    if (!all && !draft) {
      setMemorySyncError(tx("候选记忆内容不能为空。", "Memory candidate content cannot be empty."));
      return;
    }
    setMemorySyncing(all ? "all" : "selected");
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.syncCodexConsoleMemory({
        ...(all ? { all: true } : { ids: [selectedMemory?.id || ""] }),
        target: memorySyncTarget,
        ...(memorySyncTarget === "project" ? { project } : {}),
        ...(!all && selectedMemory?.id && draft !== original ? { content_overrides: { [selectedMemory.id]: draft } } : {}),
      });
      const skipped = result.skipped > 0 ? tx(`，${result.skipped} 条跳过`, `, ${result.skipped} skipped`) : "";
      const failed = result.failed > 0 ? tx(`，${result.failed} 条失败`, `, ${result.failed} failed`) : "";
      const targetText = memorySyncTarget === "project" ? tx("项目上下文", "project context") : tx("长期记忆", "profile memory");
      setMemorySyncMessage(tx(
        `已同步 ${result.synced} 条到${targetText}${skipped}${failed}。`,
        `Synced ${result.synced} items to ${targetText}${skipped}${failed}.`,
      ));
      setNoticeTarget("memory");
      const firstIssue = result.items.find((item) => item.status !== "synced" && item.message);
      if (firstIssue?.message) {
        setMemorySyncError(firstIssue.message);
      }
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("同步记忆失败", "Failed to sync memory"));
    } finally {
      setMemorySyncing("");
    }
  };

  const reviewMemoryCandidate = async (status: "accepted" | "ignored" | "deferred" | "review_required") => {
    if (!selectedMemory?.id) return;
    setMemoryReviewing(status);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.reviewCodexConsoleMemory({ ids: [selectedMemory.id], status });
      const failed = result.failed > 0 ? tx(`，${result.failed} 条失败`, `, ${result.failed} failed`) : "";
      setMemorySyncMessage(tx(`已更新 ${result.updated} 条记忆候选${failed}。`, `Updated ${result.updated} memory candidates${failed}.`));
      setNoticeTarget("memory");
      const firstIssue = result.items.find((item) => item.status === "failed" && item.message);
      if (firstIssue?.message) setMemorySyncError(firstIssue.message);
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("更新记忆候选失败", "Failed to update memory candidate"));
    } finally {
      setMemoryReviewing("");
    }
  };

  const resolveMemoryConflict = async (resolution: CodexConsoleMemoryConflictResolution) => {
    if (!selectedMemory?.id || !selectedMemory.conflict) return;
    const mergedContent = memoryConflictDraft.trim();
    if (resolution === "merge" && !mergedContent) {
      setMemorySyncError(tx("合并后的记忆内容不能为空。", "Merged memory content cannot be empty."));
      return;
    }
    setMemoryConflictResolving(resolution);
    setMemorySyncMessage("");
    setNoticeTarget("");
    setMemorySyncError("");
    try {
      const result = await api.resolveCodexConsoleMemoryConflict({
        id: selectedMemory.id,
        resolution,
        ...(resolution === "merge" ? { merged_content: mergedContent } : {}),
      });
      const labels: Record<CodexConsoleMemoryConflictResolution, string> = {
        keep_existing: tx("保留已有", "kept existing"),
        use_candidate: tx("采用候选", "used candidate"),
        keep_both: tx("两者共存", "kept both"),
        merge: tx("合并", "merged"),
      };
      setMemorySyncMessage(result.message || tx(`已处理记忆冲突：${labels[resolution]}。`, `Resolved memory conflict: ${labels[resolution]}.`));
      setNoticeTarget("memory");
      await load(true);
    } catch (err: any) {
      setMemorySyncError(err?.message || tx("处理记忆冲突失败", "Failed to resolve memory conflict"));
    } finally {
      setMemoryConflictResolving("");
    }
  };

  if (loading) return <div className="page-loading">{tx("加载中...", "Loading...")}</div>;

  const overview = data?.overview;
  const hasConsoleData = !!data && (
    data.threads.length > 0 ||
    data.automations.length > 0 ||
    data.artifacts.length > 0 ||
    data.hooks.length > 0 ||
    (data.handovers || []).length > 0 ||
    (data.skill_candidates || []).length > 0 ||
    data.memory_candidates.length > 0 ||
    data.runs.length > 0
  );

  return (
    <div className="page codex-console-page">
      <div className="page-header compact-header">
        <div>
          <h2>Codex Console</h2>
          <p className="page-subtitle">
            {tx("把最近 Codex 工作整理成待确认项、交接材料和可复用提示词。", "Turn recent Codex work into review items, handoff context, and reusable prompts.")}
          </p>
        </div>
        <div className="page-actions">
          <button className="btn" type="button" disabled={refreshing} onClick={() => void load(true)}>
            {refreshing ? tx("整理中...", "Refreshing...") : tx("重新整理", "Refresh")}
          </button>
          <Link className="btn btn-primary" to="/imports/local-apps?platform=codex">
            {tx("导入 Codex 数据", "Import Codex data")}
          </Link>
        </div>
      </div>

      {error ? <div className="alert alert-warn">{error}</div> : null}
      {memorySyncMessage ? (
        <div className="alert alert-success">
          {memorySyncMessage}
          {noticeTarget === "memory" ? <> <Link to="/memory">{tx("查看长期记忆", "View profile memory")}</Link></> : null}
          {noticeTarget === "skills" ? <> <Link to="/skills">{tx("查看 Skills", "View skills")}</Link></> : null}
          {noticeTarget === "projects" ? <> <Link to="/projects">{tx("查看项目", "View projects")}</Link></> : null}
          {noticeTarget === "platforms" ? <> <Link to="/">{tx("查看数据概览", "View data overview")}</Link></> : null}
        </div>
      ) : null}
      {memorySyncError ? <div className="alert alert-warn">{memorySyncError}</div> : null}

      {!hasConsoleData ? (
        <div className="empty-action-state">
          <p>{tx("还没有可展示的 Codex 本地资料。", "No local Codex data is available yet.")}</p>
          <Link className="btn btn-primary" to="/imports/local-apps?platform=codex">
            {tx("扫描 Codex", "Scan Codex")}
          </Link>
        </div>
      ) : (
        <>
          {view === "brief" ? (
            <section className="codex-console-brief-shell">
              <BriefDashboard
                data={data}
                overview={overview}
                tx={tx}
                onOpenAdvanced={(nextView, selectedID) => {
                  if (nextView === "memory" && selectedID) setSelectedMemoryID(selectedID);
                  if (nextView === "skill_candidates" && selectedID) setSelectedSkillCandidateID(selectedID);
                  if (nextView === "handovers" && selectedID) setSelectedHandoverID(selectedID);
                  if (nextView === "artifacts" && selectedID) setSelectedArtifactID(selectedID);
                  if (nextView === "hooks" && selectedID) setSelectedHookID(selectedID);
                  if (nextView === "runs" && selectedID) setSelectedRunID(selectedID);
                  if (nextView === "automations" && selectedID) setSelectedAutomationID(selectedID);
                  setView(nextView);
                }}
              />
            </section>
          ) : (
            <section className="codex-console-layout">
              <div className="codex-console-main">
                <div className="codex-console-tabs" role="tablist" aria-label="Codex Console views">
                  {([
                    ["brief", tx("简报", "Brief"), 0],
                    ["memory", tx("记忆候选", "Memory"), data?.memory_candidates.length || 0],
                    ["handovers", tx("项目交接", "Handovers"), data?.handovers?.length || 0],
                    ["skill_candidates", tx("Skill 草稿", "Skill Drafts"), visibleSkillCandidates.length],
                    ["runs", tx("执行记录", "Runs"), data?.runs.length || 0],
                    ["threads", tx("原始线程", "Threads"), data?.threads.length || 0],
                    ["artifacts", tx("交付物", "Artifacts"), data?.artifacts.length || 0],
                    ["automations", tx("自动化", "Automations"), data?.automations.length || 0],
                    ["hooks", tx("Hook 审查", "Hooks"), data?.hooks.length || 0],
                  ] as Array<[ConsoleView, string, number]>).map(([key, label, count]) => (
                    <button key={key} type="button" role="tab" aria-selected={view === key} className={view === key ? "is-active" : ""} onClick={() => setView(key)}>
                      <span>{label}</span>
                      {key === "brief" ? null : <strong>{count}</strong>}
                    </button>
                  ))}
                </div>
                {view === "threads" ? (
                  <ThreadTable items={data?.threads || []} selectedID={selectedThread?.id || ""} onSelect={setSelectedThreadID} tx={tx} locale={locale} />
                ) : null}
                {view === "runs" ? (
                  <RunTable items={data?.runs || []} selectedID={selectedRun?.id || ""} onSelect={setSelectedRunID} tx={tx} locale={locale} />
                ) : null}
                {view === "automations" ? (
                  <AutomationList items={data?.automations || []} selectedID={selectedAutomation?.id || ""} onSelect={setSelectedAutomationID} tx={tx} />
                ) : null}
                {view === "artifacts" ? (
                  <ArtifactList items={data?.artifacts || []} selectedID={selectedArtifact?.id || ""} onSelect={setSelectedArtifactID} tx={tx} />
                ) : null}
                {view === "hooks" ? (
                  <HookList items={data?.hooks || []} selectedID={selectedHook?.id || ""} onSelect={setSelectedHookID} tx={tx} />
                ) : null}
                {view === "memory" ? (
                  <MemoryList items={data?.memory_candidates || []} selectedID={selectedMemory?.id || ""} onSelect={setSelectedMemoryID} tx={tx} />
                ) : null}
                {view === "handovers" ? (
                  <HandoverList items={data?.handovers || []} selectedID={selectedHandover?.id || ""} onSelect={setSelectedHandoverID} tx={tx} locale={locale} />
                ) : null}
                {view === "skill_candidates" ? (
                  <SkillCandidateList
                    items={visibleSkillCandidates}
                    selectedID={selectedSkillCandidate?.id || ""}
                    onSelect={setSelectedSkillCandidateID}
                    tx={tx}
                    locale={locale}
                    archivedCount={archivedSkillCandidateCount}
                    showArchived={showArchivedSkillCandidates}
                    onShowArchivedChange={setShowArchivedSkillCandidates}
                  />
                ) : null}
              </div>

              <aside className="codex-console-detail">
                {view === "threads" && selectedThread ? (
                  <ThreadDetail item={selectedThread} locale={locale} tx={tx} copied={copied} onCopy={copySource} />
                ) : null}
                {view === "runs" && selectedRun ? (
                  <RunDetail item={selectedRun} locale={locale} tx={tx} copied={copied} onCopy={copySource} />
                ) : null}
                {view === "automations" && selectedAutomation ? (
                  <AutomationDetail item={selectedAutomation} tx={tx} copied={copied} onCopy={copySource} />
                ) : null}
                {view === "artifacts" && selectedArtifact ? (
                  <ArtifactDetail
                    item={selectedArtifact}
                    registry={data?.artifact_registry}
                    locale={locale}
                    tx={tx}
                    copied={copied}
                    saving={artifactSaving}
                    onCopy={copySource}
                    onSave={() => void saveArtifactRegistry()}
                  />
                ) : null}
                {view === "hooks" && selectedHook ? (
                  <HookDetail item={selectedHook} tx={tx} copied={copied} onCopy={copySource} />
                ) : null}
                {view === "memory" && selectedMemory ? (
                  <MemoryDetail
                    item={selectedMemory}
                    tx={tx}
                    locale={locale}
                    copied={copied}
                    syncing={memorySyncing}
                    syncTarget={memorySyncTarget}
                    syncProject={memorySyncProject}
                    draftContent={memoryDraftContent}
                    conflictDraftContent={memoryConflictDraft}
                    reviewing={memoryReviewing}
                    conflictResolving={memoryConflictResolving}
                    onCopy={copySource}
                    onSyncTargetChange={setMemorySyncTarget}
                    onSyncProjectChange={setMemorySyncProject}
                    onDraftContentChange={setMemoryDraftContent}
                    onDraftReset={() => setMemoryDraftContent(cleanPreviewText(selectedMemory.content))}
                    onConflictDraftChange={setMemoryConflictDraft}
                    onConflictDraftReset={() => setMemoryConflictDraft(buildMemoryConflictDraft(selectedMemory, tx))}
                    onSyncSelected={() => void syncMemoryCandidates(false)}
                    onSyncAll={() => void syncMemoryCandidates(true)}
                    onReview={(status) => void reviewMemoryCandidate(status)}
                    onResolveConflict={(resolution) => void resolveMemoryConflict(resolution)}
                  />
                ) : null}
                {view === "handovers" && selectedHandover ? (
                  <HandoverDetail
                    item={selectedHandover}
                    tx={tx}
                    locale={locale}
                    saving={handoverSaving === selectedHandover.id}
                    draftContent={handoverDraftContent}
                    onDraftChange={setHandoverDraftContent}
                    onDraftReset={() => setHandoverDraftContent(buildHandoverDraft(selectedHandover, tx))}
                    onSave={(id) => void saveHandover(id, handoverDraftContent)}
                  />
                ) : null}
                {view === "skill_candidates" && selectedSkillCandidate ? (
                  <SkillCandidateDetail
                    item={selectedSkillCandidate}
                    tx={tx}
                    locale={locale}
                    copied={copied}
                    saving={skillSaving === selectedSkillCandidate.id}
                    statusUpdating={skillStatusUpdating}
                    draftContent={skillDraftContent}
                    assigning={skillAssigning === selectedSkillCandidate.id}
                    assignPreview={skillAssignPreview?.id === selectedSkillCandidate.id ? skillAssignPreview : null}
                    agentOptions={agentOptions}
                    selectedAgentIDs={skillAssignAgentIDs}
                    onDraftChange={setSkillDraftContent}
                    onDraftReset={() => setSkillDraftContent(cleanSkillDraftText(selectedSkillCandidate.draft))}
                    onSave={(id) => void saveSkillCandidate(id, skillDraftContent)}
                    onAgentToggle={toggleSkillAssignAgent}
                    onAssignAgents={(id, agentIDs) => void assignSkillCandidateToAgents(id, agentIDs)}
                    onStatusUpdate={(id, status) => void updateSkillCandidateStatus(id, status)}
                    onCopyDraft={copyDraft}
                    onCopy={copySource}
                  />
                ) : null}
              </aside>
            </section>
          )}

          {view !== "brief" ? (
            <section className="codex-console-overview">
              <div className="dashboard-section-head compact">
                <div>
                  <h3>{tx("工作区", "Workspaces")}</h3>
                  <p>{tx("专业用户追溯原始 Codex session 时使用。", "For expert tracing of raw Codex sessions.")}</p>
                </div>
                <span className="dashboard-card-link-muted">{formatDateTime(overview?.last_activity, locale)}</span>
              </div>
              <div className="codex-console-workspaces">
                {(overview?.workspaces || []).slice(0, 6).map((workspace) => (
                  <div key={workspace.name} className="codex-console-workspace">
                    <strong>{workspace.name}</strong>
                    <span>{tx(`${workspace.threads} 条线程`, `${workspace.threads} threads`)}</span>
                    <small>{formatDateTime(workspace.last_activity, locale)}</small>
                  </div>
                ))}
              </div>
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}

function codexBriefStats(data: CodexConsoleResponse | null) {
  const memoryCandidates = data?.memory_candidates || [];
  const runs = data?.runs || [];
  const hooks = data?.hooks || [];
  const failedRuns = runs.filter((item) => item.errors > 0);
  const failedProjects = new Set(failedRuns.map((item) => item.project || item.thread_title).filter(Boolean));
  const artifactProjects = data?.artifact_registry?.project_count || new Set((data?.artifacts || []).map((item) => item.project).filter(Boolean)).size;
  return {
    pendingMemory: memoryCandidates.filter((item) => !item.conflict && isMemoryActionable(item)).length,
    memoryConflicts: memoryCandidates.filter((item) => item.conflict && isMemoryActionable(item)).length,
    failedRuns: failedRuns.length,
    failedProjects: failedProjects.size,
    hookReview: hooks.filter((item) => item.status === "manual_required" || item.risk_level === "high").length,
    skillDrafts: data?.skill_candidates.length || 0,
    handovers: data?.handovers.length || 0,
    artifacts: data?.artifacts.length || 0,
    artifactProjects,
    artifactRegistrySaved: data?.artifact_registry?.status === "saved",
    threads: data?.threads.length || 0,
  };
}

function codexBriefRecentText(data: CodexConsoleResponse | null) {
  const chunks: string[] = [];
  for (const thread of (data?.threads || []).slice(0, 20)) {
    chunks.push(thread.title, thread.summary || "", thread.project || "");
  }
  for (const run of (data?.runs || []).slice(0, 20)) {
    chunks.push(run.thread_title, run.project || "");
    for (const event of (run.events || []).slice(0, 4)) {
      chunks.push(event.title, event.detail || "");
    }
  }
  return chunks.join("\n").toLowerCase();
}

function BriefDashboard({
  data,
  overview,
  tx,
  onOpenAdvanced,
}: {
  data: CodexConsoleResponse | null;
  overview: CodexConsoleResponse["overview"] | undefined;
  tx: (zh: string, en: string) => string;
  onOpenAdvanced: (view: ConsoleView, selectedID?: string) => void;
}) {
  const stats = codexBriefStats(data);
  const latestProject = overview?.workspaces?.[0];
  const promptSuggestions = buildPromptSuggestions(data, stats, tx);
  const memoryHighlights = (data?.memory_candidates || [])
    .filter(isMemoryActionable)
    .sort((left, right) => Number(!!right.conflict) - Number(!!left.conflict))
    .slice(0, 1);
  const preparedItems = buildPreparedAgentItems(data, stats, latestProject?.name, tx);
  const allTaskItems = buildBriefTaskItems(data, stats, tx);
  const taskItems = allTaskItems.slice(0, 4);
  const actionableTaskCount = allTaskItems.filter((item) => !item.disabled && !item.isEmpty).length;
  const nextPromptTemplate = tx(
    `请先读取 Vola 里${latestProject?.name ? ` ${latestProject.name} 的` : ""}项目交接、长期记忆候选和交付物索引；如果目标是桌面端，请验证 macOS Vola.app。完成后说明改了哪些文件、验证结果和未验证项。`,
    `Read Vola${latestProject?.name ? ` ${latestProject.name}` : ""} project handoff, memory candidates, and artifact registry first. If the target is desktop, verify the macOS Vola.app. Finish with changed files, verification results, and anything not verified.`,
  );
  return (
    <div className="codex-console-brief-board">
      <section className="codex-console-workbench" aria-label={tx("Codex Console 工作台", "Codex Console workbench")}>
        <div className="codex-console-action-queue">
          <div className="codex-console-action-head">
            <div>
              <h3>{tx("现在要处理的事", "What needs attention")}</h3>
              <p>{tx("只放会影响后续 AI 使用效果的项目。", "Only items that affect later AI usefulness are shown here.")}</p>
            </div>
            <span>{actionableTaskCount ? tx(`${countLabel(actionableTaskCount)} 项待处理`, `${countLabel(actionableTaskCount)} to review`) : tx("暂无待处理", "Nothing pending")}</span>
          </div>
          <div className="codex-console-task-list">
            {taskItems.map((item) => (
              <BriefTaskCard
                key={item.title}
                title={item.title}
                body={item.body}
                meta={item.meta}
                result={item.result}
                action={item.action}
                tone={item.tone}
                disabled={item.disabled}
                isEmpty={item.isEmpty}
                onAction={() => onOpenAdvanced(item.view, item.selectedID)}
              />
            ))}
          </div>
        </div>

        <aside className="codex-console-next-panel">
          <div className="dashboard-section-head compact">
            <div>
              <h3>{tx("给下个 AI 的提示词", "Prompt for the next AI")}</h3>
              <p>{tx("复制后直接贴到新会话，先让它读取 Vola 已整理的资料。", "Copy this into a new session so it reads the context Vola already organized.")}</p>
            </div>
          </div>
          <PromptTemplateBox text={nextPromptTemplate} tx={tx} />
          <div className="codex-console-brief-metrics" aria-label={tx("资料概览", "Context overview")}>
            <button type="button" onClick={() => onOpenAdvanced("memory")}>
              <strong>{countLabel(stats.memoryConflicts + stats.pendingMemory)}</strong>
              <span>{tx("记忆候选", "Memory")}</span>
            </button>
            <button type="button" onClick={() => onOpenAdvanced("handovers")}>
              <strong>{countLabel(stats.handovers)}</strong>
              <span>{tx("交接材料", "Handovers")}</span>
            </button>
            <button type="button" onClick={() => onOpenAdvanced("artifacts")}>
              <strong>{countLabel(stats.artifacts)}</strong>
              <span>{tx("交付物", "Artifacts")}</span>
            </button>
            <button type="button" onClick={() => onOpenAdvanced("skill_candidates")}>
              <strong>{countLabel(stats.skillDrafts)}</strong>
              <span>{tx("Skill 草稿", "Skill drafts")}</span>
            </button>
          </div>
        </aside>
      </section>

      <section className="codex-console-brief-section is-compact">
        <div className="dashboard-section-head compact">
          <div>
            <h3>{tx("常用资料入口", "Common context")}</h3>
            <p>{tx("需要交接、保存索引或审查候选时从这里进入。", "Use these when you need handoff context, saved indexes, or candidate review.")}</p>
          </div>
        </div>
        <div className="codex-console-prepared-list is-compact">
          {preparedItems.map((item) => (
            <BriefPreparedItem
              key={item.title}
              title={item.title}
              body={item.body}
              state={item.state}
              action={item.action}
              disabled={item.disabled}
              onAction={() => onOpenAdvanced(item.view)}
            />
          ))}
        </div>
      </section>

      <div className="codex-console-brief-secondary-grid">
        <section className="codex-console-brief-section">
          <div className="dashboard-section-head compact">
            <div>
              <h3>{tx("提示词改进建议", "Prompt improvements")}</h3>
              <p>{tx("保留最容易影响结果的几条。", "The few points most likely to affect the result.")}</p>
            </div>
          </div>
          <div className="codex-console-prompt-list">
            {promptSuggestions.map((item) => (
              <PromptSuggestionCard
                key={item.title}
                title={item.title}
                body={item.body}
                example={item.example}
                source={item.source}
                tx={tx}
              />
            ))}
          </div>
        </section>

        <section className="codex-console-brief-section">
          <div className="dashboard-section-head compact">
            <div>
              <h3>{tx("长期记忆", "Long-term memory")}</h3>
              <p>{tx("确认前不会写入长期记忆。", "Nothing is saved as long-term memory before review.")}</p>
            </div>
            <span className="dashboard-card-link-muted">{stats.memoryConflicts ? tx("有相似记忆", "possible overlap") : stats.pendingMemory ? tx("值得确认", "worth reviewing") : tx("暂无新建议", "nothing new")}</span>
          </div>
          <div className="codex-console-memory-value-list">
            {memoryHighlights.length ? (
              memoryHighlights.map((item) => <MemoryValueCard key={item.id} item={item} tx={tx} />)
            ) : (
              <MemoryValueCard tx={tx} />
            )}
          </div>
        </section>
      </div>

      <section className="codex-console-brief-section is-expert">
        <details className="codex-console-expert-details">
          <summary>
            <span>
              <strong>{tx("专业资料和编辑入口", "Expert records and editing")}</strong>
              <small>{tx("需要审查、编辑或追溯时再打开。", "Open only when you need review, editing, or traceability.")}</small>
            </span>
          </summary>
          <AdvancedViewButtons data={data} tx={tx} onOpen={onOpenAdvanced} />
          <div className="codex-console-brief-guardrails">
            <strong>{tx("自动处理边界", "Automation boundary")}</strong>
            <p>{tx("Vola 会整理、预览和生成草稿；长期记忆写入、Skill 安装、Hook 启用仍需要人工确认。", "Vola organizes, previews, and drafts; saving long-term memory, installing skills, and enabling hooks still require human approval.")}</p>
          </div>
        </details>
      </section>
    </div>
  );
}

type BriefTaskTone = "bad" | "warn" | "neutral" | "ok";

type BriefTaskItem = {
  title: string;
  body: string;
  meta: string;
  result: string;
  action: string;
  view: ConsoleView;
  selectedID?: string;
  tone: BriefTaskTone;
  disabled: boolean;
  isEmpty?: boolean;
};

function buildBriefTaskItems(
  data: CodexConsoleResponse | null,
  stats: ReturnType<typeof codexBriefStats>,
  tx: (zh: string, en: string) => string,
): BriefTaskItem[] {
  const items: BriefTaskItem[] = [];
  const memoryCandidates = data?.memory_candidates || [];
  const memoryConflict = memoryCandidates.find((item) => item.conflict && isMemoryActionable(item));
  const pendingMemory = memoryCandidates.find((item) => !item.conflict && isMemoryActionable(item));
  const skillDrafts = (data?.skill_candidates || []).filter((item) => {
    if (isArchivedSkillCandidate(item)) return false;
    const status = (item.status || "").toLowerCase();
    return status !== "ready" || !item.skill_path;
  });
  const unsavedHandover = (data?.handovers || []).find((item) => !item.path);
  const hookReview = (data?.hooks || []).find((item) => item.status === "manual_required" || item.risk_level === "high");
  const failedRun = (data?.runs || []).find((item) => item.errors > 0);

  if (memoryConflict) {
    items.push({
      title: tx("处理相似记忆", "Review overlapping memory"),
      body: tx("有候选记忆和已有长期记忆相似。确认合并方式后，再让后续 AI 使用这些背景。", "A candidate overlaps with existing long-term memory. Choose how to merge it before later AI uses the context."),
      meta: tx(`${countLabel(stats.memoryConflicts)} 条需要确认`, `${countLabel(stats.memoryConflicts)} need review`),
      result: tx("后续 AI 不会读到互相打架的背景。", "Later AI will not read conflicting context."),
      action: tx("处理记忆", "Review memory"),
      view: "memory",
      selectedID: memoryConflict.id,
      tone: "bad",
      disabled: false,
    });
  }

  if (pendingMemory) {
    items.push({
      title: tx("确认长期记忆候选", "Review memory candidates"),
      body: tx("这些内容可能是稳定偏好、项目背景或固定流程。确认前不会写入长期记忆。", "These may be stable preferences, project context, or repeat workflows. Nothing is saved before review."),
      meta: tx(`${countLabel(stats.pendingMemory)} 条可确认`, `${countLabel(stats.pendingMemory)} reviewable`),
      result: tx("后续 AI 会少问背景，少按旧前提处理任务。", "Later AI can ask less and avoid stale assumptions."),
      action: tx("查看候选", "View candidates"),
      view: "memory",
      selectedID: pendingMemory.id,
      tone: "warn",
      disabled: false,
    });
  }

  if (hookReview) {
    items.push({
      title: tx("审查 Hook 风险", "Review hook risk"),
      body: tx("发现需要人工确认的 Hook 或高风险脚本。Vola 不会自动启用它们。", "A hook or high-risk script needs human review. Vola will not enable it automatically."),
      meta: tx(`${countLabel(stats.hookReview)} 项需审查`, `${countLabel(stats.hookReview)} to review`),
      result: tx("启用脚本前先确认权限和影响范围。", "Permissions and impact are reviewed before scripts are enabled."),
      action: tx("查看 Hook", "View hooks"),
      view: "hooks",
      selectedID: hookReview.id,
      tone: "bad",
      disabled: false,
    });
  }

  if (failedRun) {
    items.push({
      title: tx("查看失败执行记录", "Check failed run"),
      body: tx("最近执行记录里有失败信号。继续同类任务前，建议先看失败发生在哪里。", "A recent run has failure signals. Check where it failed before continuing similar work."),
      meta: tx(`${countLabel(stats.failedRuns)} 条失败记录`, `${countLabel(stats.failedRuns)} failed runs`),
      result: tx("继续同类任务时可以避开已知失败点。", "Similar work can avoid known failure points."),
      action: tx("查看记录", "View run"),
      view: "runs",
      selectedID: failedRun.id,
      tone: "warn",
      disabled: false,
    });
  }

  if (skillDrafts.length) {
    items.push({
      title: tx("审查 Skill 草稿", "Review skill drafts"),
      body: tx("先看命名、适用范围和同步目标，再决定是否分配给 Codex、Claude Code、Cursor 或 Gemini CLI。", "Review the name, scope, and sync target before assigning it to Codex, Claude Code, Cursor, or Gemini CLI."),
      meta: tx(`${countLabel(skillDrafts.length)} 条待审查`, `${countLabel(skillDrafts.length)} to review`),
      result: tx("常用流程会变成可复用规则，不用每次重讲。", "Repeat workflows become reusable rules."),
      action: tx("查看草稿", "View drafts"),
      view: "skill_candidates",
      selectedID: skillDrafts[0]?.id,
      tone: "warn",
      disabled: false,
    });
  }

  if (unsavedHandover) {
    items.push({
      title: tx("保存项目交接", "Save project handoff"),
      body: tx(`${unsavedHandover.project} 已生成交接摘要，保存后后续 AI 可以直接读取项目状态。`, `${unsavedHandover.project} has a handoff summary. Save it so later AI can read the project state directly.`),
      meta: tx(`${countLabel(stats.handovers)} 份交接材料`, `${countLabel(stats.handovers)} handoffs`),
      result: tx("换会话或换同事时不用翻旧记录。", "New sessions and teammates can start without digging through old records."),
      action: tx("查看交接", "View handoff"),
      view: "handovers",
      selectedID: unsavedHandover.id,
      tone: "neutral",
      disabled: false,
    });
  }

  if (stats.artifacts > 0 && !stats.artifactRegistrySaved) {
    items.push({
      title: tx("保存交付物索引", "Save artifact registry"),
      body: tx("报告、截图、文档和生成文件已经识别出来。保存索引后，后续 AI 更容易区分结果和验证证据。", "Reports, screenshots, documents, and generated files were found. Saving the registry helps later AI distinguish outputs from evidence."),
      meta: tx(`${countLabel(stats.artifacts)} 个交付物`, `${countLabel(stats.artifacts)} artifacts`),
      result: tx("下次接手能直接知道哪些文件能交付。", "The next session can tell which files are deliverables."),
      action: tx("查看交付物", "View artifacts"),
      view: "artifacts",
      selectedID: data?.artifacts[0]?.id,
      tone: "neutral",
      disabled: false,
    });
  }

  if (!items.length) {
    items.push({
      title: tx("暂无必须处理的项", "Nothing needs review"),
      body: tx("可以直接复制右侧提示词开始新会话；需要追溯时再打开下面的专业入口。", "You can copy the prompt on the right into a new session. Open expert views only when tracing details."),
      meta: tx("资料已整理", "Context organized"),
      result: tx("当前不需要额外审查。", "No extra review is needed now."),
      action: tx("无需操作", "No action needed"),
      view: "brief",
      tone: "ok",
      disabled: true,
      isEmpty: true,
    });
  }

  return items;
}

function BriefTaskCard({
  title,
  body,
  meta,
  result,
  action,
  tone,
  disabled,
  isEmpty,
  onAction,
}: {
  title: string;
  body: string;
  meta: string;
  result: string;
  action: string;
  tone: BriefTaskTone;
  disabled: boolean;
  isEmpty?: boolean;
  onAction: () => void;
}) {
  return (
    <article className={`codex-console-task-card tone-${tone}${isEmpty ? " is-empty" : ""}`}>
      <small>{meta}</small>
      <strong>{title}</strong>
      <p>{body}</p>
      <span className="codex-console-task-result">{result}</span>
      {isEmpty ? (
        <b>{action}</b>
      ) : (
        <button type="button" disabled={disabled} onClick={onAction}>
          {action}
        </button>
      )}
    </article>
  );
}

function buildPromptSuggestions(
  data: CodexConsoleResponse | null,
  stats: ReturnType<typeof codexBriefStats>,
  tx: (zh: string, en: string) => string,
) {
  const recentText = codexBriefRecentText(data);
  const hasDesktopSignals = recentText.includes("desktop") ||
    recentText.includes("桌面") ||
    recentText.includes("vola.app") ||
    recentText.includes("tauri");
  const hasVerificationSignals = recentText.includes("验证") ||
    recentText.includes("test") ||
    recentText.includes("build") ||
    recentText.includes("screenshot") ||
    stats.failedRuns > 0;
  const items: Array<{
    title: string;
    body: string;
    example: string;
    source: string;
  }> = [];
  items.push({
    title: hasDesktopSignals ? tx("你最近在区分桌面端和网页版", "You are distinguishing desktop from web") : tx("先说明要验证哪个端", "Name the target surface first"),
    body: hasDesktopSignals
      ? tx("最近记录里出现桌面端 / Tauri / Vola.app 相关工作，下次提示词直接点名桌面端，可以减少测错对象。", "Recent records mention desktop / Tauri / Vola.app work. Naming the desktop app directly reduces wrong-surface checks.")
      : tx("如果任务涉及桌面或网页，开头就写清楚目标端，AI 更不容易测错地方。", "If a task involves desktop or web, name the target surface up front so the AI checks the right place."),
    example: hasDesktopSignals
      ? tx("请以桌面端为准，打开新构建的 macOS Vola.app 验证，不要只验证网页版。", "Use the desktop app as the source of truth. Open the newly built macOS Vola.app, not just the web page.")
      : tx("请先确认本次要验证的是桌面端、网页版还是两者都要。", "First confirm whether this should be verified on desktop, web, or both."),
    source: hasDesktopSignals ? tx("依据：最近 Codex 记录里有桌面端相关信号", "Based on recent desktop-related Codex signals") : tx("依据：当前工作包含多端入口", "Based on the multi-surface app context"),
  });
  items.push({
    title: tx("把长期事实交给 Vola 记忆", "Put long-term facts into Vola memory"),
    body: stats.memoryConflicts > 0
      ? tx("当前有相似记忆，提示词里可以要求先展示候选和冲突，再由你确认。", "There is overlapping memory now; ask the AI to show candidates and conflicts before saving.")
      : tx("项目背景、个人偏好和固定工作方式适合写成候选记忆，后续 AI 会少问很多。", "Project context, personal preferences, and stable workflows are good memory candidates and reduce repeated questions."),
    example: tx("如果发现长期偏好、项目背景或固定流程，请作为 Vola 记忆候选展示；冲突时先让我确认。", "If you find stable preferences, project context, or repeatable workflows, show them as Vola memory candidates; ask me before resolving conflicts."),
    source: stats.pendingMemory || stats.memoryConflicts
      ? tx("依据：当前已有可审查的记忆候选", "Based on current memory candidates")
      : tx("依据：Vola 会持续从 Codex 记录里提取长期事实", "Based on Vola's long-term fact extraction"),
  });
  if (hasVerificationSignals) {
    items.push({
      title: tx("把验收标准写清楚", "State the acceptance check"),
      body: stats.failedRuns > 0
        ? tx("历史执行里有失败信号，提示词最好写明要跑哪些测试、看哪个端、哪些没验证必须说明。", "Past runs contain failure signals. Specify tests, target surface, and unverified areas.")
        : tx("最近记录里有验证、构建或截图相关工作，提示词里写清验收方式会减少返工。", "Recent records include verification, build, or screenshot work. Stating acceptance checks reduces rework."),
      example: tx("完成后请列出验证命令、实际结果、桌面端截图或观察，以及没有验证的部分。", "After finishing, list verification commands, results, desktop observations or screenshots, and anything not verified."),
      source: stats.failedRuns > 0
        ? tx("依据：当前执行记录里有失败信号", "Based on failure signals in runs")
        : tx("依据：最近 Codex 记录里有验证相关信号", "Based on recent verification-related Codex signals"),
    });
  }
  if (stats.artifacts > 0) {
    items.push({
      title: tx("要求说明交付物用途", "Ask for artifact purpose"),
      body: tx("让 AI 最后说明改了哪些文件、哪些是最终交付物，后续接手会容易很多。", "Ask the AI to explain changed files and final deliverables so later handoff is easier."),
      example: tx("最后请说明生成或修改了哪些文件，每个文件给用户或下一个 Agent 的用途是什么。", "At the end, explain which files were created or changed and how each one helps the user or next agent."),
      source: tx("依据：Codex 记录里已识别到交付物引用", "Based on artifact references found in Codex records"),
    });
  }
  return items.slice(0, 3);
}

function BriefPreparedItem({
  title,
  body,
  state,
  action,
  disabled,
  onAction,
}: {
  title: string;
  body: string;
  state: string;
  action: string;
  disabled: boolean;
  onAction: () => void;
}) {
  return (
    <article className="codex-console-prepared-item">
      <div>
        <strong>{title}</strong>
        <p>{body}</p>
      </div>
      <div>
        <small>{state}</small>
        <button type="button" disabled={disabled} onClick={onAction}>{action}</button>
      </div>
    </article>
  );
}

function PromptSuggestionCard({
  title,
  body,
  example,
  source,
  tx,
}: {
  title: string;
  body: string;
  example: string;
  source: string;
  tx: (zh: string, en: string) => string;
}) {
  return (
    <article className="codex-console-prompt-card">
      <div className="codex-console-prompt-card-head">
        <span>{tx("建议", "Advice")}</span>
        <strong>{title}</strong>
      </div>
      <p>{body}</p>
      <div className="codex-console-prompt-example">
        <span>{tx("可以这样写", "Try this wording")}</span>
        <p>{example}</p>
      </div>
      <small>{source}</small>
    </article>
  );
}

function cleanMemoryValuePreview(
  value: string | undefined,
  fallback: string | undefined,
  tx: (zh: string, en: string) => string,
) {
  const raw = cleanPreviewText(value || fallback);
  const taskGroup = raw.match(/Task Group:\s*([^#\n]+?)(?:\s+scope:|$)/i);
  if (taskGroup?.[1]) {
    const group = shortPreview(taskGroup[1].replace(/\s+/g, " ").trim(), 90);
    return tx(
      `这条记录描述了 ${group} 的项目背景和处理范围。`,
      `This records project context and workflow scope for ${group}.`,
    );
  }
  if (/ad-hoc notes/i.test(raw) || /memories\/extensions/i.test(raw)) {
    return tx(
      "这条记录来自 Codex 记忆扩展，包含某个项目的长期说明；确认后可以合并进 Vola 记忆。",
      "This comes from a Codex memory extension and contains long-term project notes; after review it can be merged into Vola memory.",
    );
  }
  const text = raw
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/#{1,6}\s*/g, "")
    .replace(/\/Users\/[^\s，。；,;]+/g, "本机项目路径")
    .replace(/\b[\w.-]+\/(?:[\w.@-]+\/){1,}[\w.@-]+\b/g, "相关资料")
    .replace(/\s+/g, " ")
    .trim();
  return shortPreview(text, 180);
}

function memoryValueReason(item: CodexConsoleMemoryCandidate, tx: (zh: string, en: string) => string) {
  if (item.conflict) {
    return tx("它和已有长期记忆相似，先确认合并方式，可以避免后续 AI 读到互相冲突的背景。", "It overlaps with existing memory. Reviewing the merge avoids later AI reading conflicting context.");
  }
  if (item.kind === "chronicle") {
    return tx("它记录的是反复出现的工作方式，写入后后续 AI 可以直接沿用，不用每次重新解释。", "It captures a repeated workflow, so later AI can reuse it without asking again.");
  }
  return tx("它像项目背景或稳定偏好，写入后后续 AI 会少问一次，也更不容易按错误前提处理任务。", "It looks like project context or a stable preference, so later AI can ask less and avoid wrong assumptions.");
}

function buildMemoryConflictDraft(
  item: CodexConsoleMemoryCandidate,
  tx: (zh: string, en: string) => string,
) {
  const title = cleanPreviewText(item.title) || tx("Codex 记忆", "Codex memory");
  const existing = cleanPreviewText(item.conflict?.existing_content);
  const candidate = cleanPreviewText(item.conflict?.candidate_content || item.content);
  const lines = [
    `# ${tx("长期记忆：", "Long-term memory: ")}${title}`,
    "",
    `## ${tx("已确认背景", "Confirmed context")}`,
    "",
    existing || tx("这里填写已经确认的旧记忆。", "Add the confirmed existing memory here."),
    "",
    `## ${tx("这次新增背景", "New context from this candidate")}`,
    "",
    candidate || tx("这里填写这次候选里值得保留的内容。", "Add the useful candidate content here."),
    "",
    `## ${tx("后续 AI 应该怎么用", "How later AI should use this")}`,
    "",
    tx(
      "- 后续处理相关任务时先读取这条长期记忆；如果任务和本条不相关，不要强行套用。",
      "- Later AI should read this memory for related tasks; if the task is unrelated, do not force it.",
    ),
  ];
  return lines.join("\n").trim();
}

function MemoryValueCard({
  item,
  tx,
}: {
  item?: CodexConsoleMemoryCandidate;
  tx: (zh: string, en: string) => string;
}) {
  if (!item) {
    return (
      <div className="codex-console-memory-value-card is-empty">
        <strong>{tx("暂无新的长期记忆建议", "No new memory suggestions")}</strong>
        <p>{tx("当 Codex 记录里出现稳定偏好、项目背景或工作方式时，Vola 会把它放到候选池。", "When Codex records stable preferences, project context, or work habits, Vola will put them in the candidate pool.")}</p>
        <small>{tx("不用操作。", "No action needed.")}</small>
      </div>
    );
  }
  const value = memoryValueReason(item, tx);
  return (
    <article className={item.conflict ? "codex-console-memory-value-card has-conflict" : "codex-console-memory-value-card"}>
      <div className="codex-console-memory-value-head">
        <span>{item.conflict ? tx("需要确认", "Needs review") : tx("值得记住", "Worth saving")}</span>
        <strong>{item.conflict ? tx("有一条候选记忆和旧记忆相似", "One candidate overlaps with existing memory") : tx("建议确认一条长期记忆", "Suggested long-term memory")}</strong>
      </div>
      <p>{cleanMemoryValuePreview(item.content, item.title, tx) || item.title}</p>
      <div className="codex-console-memory-reason">
        <span>{tx("为什么有用", "Why it matters")}</span>
        <p>{value}</p>
      </div>
    </article>
  );
}

function buildPreparedAgentItems(
  data: CodexConsoleResponse | null,
  stats: ReturnType<typeof codexBriefStats>,
  latestProject: string | undefined,
  tx: (zh: string, en: string) => string,
) {
  return [
    {
      title: tx("项目接手材料", "Project handoff"),
      body: latestProject
        ? tx(`已把 ${latestProject} 的线程、执行记录和交付物整理成接手摘要，后续 AI 可以先读这里。`, `Threads, runs, and artifacts for ${latestProject} are organized into a handoff the next AI can read first.`)
        : tx("已把最近项目的线程、执行记录和交付物整理成接手摘要，后续 AI 可以先读这里。", "Recent project threads, runs, and artifacts are organized into handoffs the next AI can read first."),
      state: stats.handovers ? tx("已准备好", "Prepared") : tx("等待更多记录", "Waiting for more records"),
      action: tx("查看交接", "View handovers"),
      view: "handovers" as ConsoleView,
      disabled: stats.handovers === 0,
    },
    {
      title: tx("交付物用途说明", "Artifact usage"),
      body: tx("已把报告、截图、HTML、文档和生成文件按用途归类，后续 AI 可以知道哪些是结果、哪些是验证证据。", "Reports, screenshots, HTML, documents, and generated files are grouped by purpose so later AI can tell outputs from evidence."),
      state: stats.artifacts ? (stats.artifactRegistrySaved ? tx("索引已保存", "Registry saved") : tx("已识别交付物", "Artifacts found")) : tx("暂无交付物", "No artifacts yet"),
      action: tx("查看交付物", "View artifacts"),
      view: "artifacts" as ConsoleView,
      disabled: stats.artifacts === 0,
    },
    {
      title: tx("长期记忆候选", "Long-term memory candidates"),
      body: stats.memoryConflicts
        ? tx("发现和旧记忆相似的候选。Vola 会先展示差异，避免后续 AI 读到互相冲突的背景。", "Overlapping memory candidates were found. Vola shows the differences first so later AI does not read conflicting context.")
        : tx("发现可能会长期有用的项目背景、偏好或流程。确认后，后续 AI 可以少问、少误解。", "Potentially useful project context, preferences, or workflows were found. After review, later AI can ask less and avoid misunderstanding."),
      state: stats.memoryConflicts ? tx("需要确认", "Needs review") : stats.pendingMemory ? tx("可确认", "Reviewable") : tx("暂无新建议", "Nothing new"),
      action: tx("查看记忆", "View memory"),
      view: "memory" as ConsoleView,
      disabled: stats.pendingMemory === 0 && stats.memoryConflicts === 0,
    },
    {
      title: tx("可复用流程草稿", "Reusable workflow drafts"),
      body: tx("成功完成的 Codex 线程会变成 Skill 草稿，专业用户审查后再分配给 Codex、Claude Code、Cursor 或 Gemini CLI。", "Successful Codex threads become skill drafts that experts can review before assigning to Codex, Claude Code, Cursor, or Gemini CLI."),
      state: stats.skillDrafts ? tx("待审查", "Needs review") : tx("暂无草稿", "No drafts yet"),
      action: tx("查看草稿", "View drafts"),
      view: "skill_candidates" as ConsoleView,
      disabled: stats.skillDrafts === 0,
    },
    {
      title: tx("自动化和 Hook 风险", "Automations and hook risks"),
      body: tx("自动化和 hook 只进入清单与风险审查。Vola 不会自动启用脚本，也不会替用户授权高风险行为。", "Automations and hooks only enter inventory and risk review. Vola does not enable scripts or authorize high-risk actions automatically."),
      state: (data?.hooks.length || 0) > 0 || (data?.automations.length || 0) > 0 ? tx("已纳入审查", "In review") : tx("暂无风险项", "No risk items"),
      action: (data?.hooks.length || 0) > 0 ? tx("查看 Hook", "View hooks") : tx("查看自动化", "View automations"),
      view: ((data?.hooks.length || 0) > 0 ? "hooks" : "automations") as ConsoleView,
      disabled: (data?.hooks.length || 0) === 0 && (data?.automations.length || 0) === 0,
    },
  ];
}

function PromptTemplateBox({
  text,
  tx,
}: {
  text: string;
  tx: (zh: string, en: string) => string;
}) {
  const [copiedTemplate, setCopiedTemplate] = useState(false);
  const copy = async () => {
    await navigator.clipboard?.writeText(text);
    setCopiedTemplate(true);
    window.setTimeout(() => setCopiedTemplate(false), 1400);
  };
  return (
    <div className="codex-console-prompt-template">
      <p>{text}</p>
      <button className="btn btn-outline" type="button" onClick={() => void copy()}>
        {copiedTemplate ? tx("已复制", "Copied") : tx("复制提示词", "Copy prompt")}
      </button>
    </div>
  );
}

function AdvancedViewButtons({
  data,
  tx,
  onOpen,
  compact = false,
}: {
  data: CodexConsoleResponse | null;
  tx: (zh: string, en: string) => string;
  onOpen: (view: ConsoleView) => void;
  compact?: boolean;
}) {
  const items: Array<[ConsoleView, string, number]> = [
    ["memory", tx("记忆候选", "Memory"), data?.memory_candidates.length || 0],
    ["handovers", tx("项目交接", "Handovers"), data?.handovers.length || 0],
    ["skill_candidates", tx("Skill 草稿", "Skill drafts"), data?.skill_candidates.length || 0],
    ["runs", tx("执行记录", "Runs"), data?.runs.length || 0],
    ["threads", tx("原始线程", "Threads"), data?.threads.length || 0],
    ["artifacts", tx("交付物", "Artifacts"), data?.artifacts.length || 0],
    ["automations", tx("自动化", "Automations"), data?.automations.length || 0],
    ["hooks", tx("Hook 审查", "Hooks"), data?.hooks.length || 0],
  ];
  return (
    <div className={compact ? "codex-console-advanced-buttons is-compact" : "codex-console-advanced-buttons"}>
      {items.map(([view, label, count]) => (
        <button key={view} type="button" onClick={() => onOpen(view)}>
          <span>{label}</span>
          <strong>{advancedViewStatus(view, count, data, tx)}</strong>
        </button>
      ))}
    </div>
  );
}

function advancedViewStatus(
  view: ConsoleView,
  count: number,
  data: CodexConsoleResponse | null,
  tx: (zh: string, en: string) => string,
) {
  if (!count) return tx("暂无", "None");
  if (view === "memory") {
    const stats = codexBriefStats(data);
    if (stats.memoryConflicts) return tx("需确认", "Review");
    return tx("可查看", "Ready");
  }
  if (view === "handovers") return tx("可接手", "Ready");
  if (view === "skill_candidates") return tx("待审查", "Review");
  if (view === "artifacts") return tx("已整理", "Organized");
  if (view === "hooks") return tx("需审查", "Review");
  return tx("可追溯", "Traceable");
}

function ThreadTable({
  items,
  selectedID,
  onSelect,
  tx,
  locale,
}: {
  items: CodexConsoleThread[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
}) {
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有线程。", "No threads yet.")}</p></div>;
  return (
    <table className="codex-console-table">
      <thead>
        <tr>
          <th>{tx("线程", "Thread")}</th>
          <th>{tx("项目", "Project")}</th>
          <th>{tx("事件", "Events")}</th>
          <th>{tx("更新时间", "Updated")}</th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => (
          <tr key={item.id} className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
            <td><strong>{item.title}</strong><small>{shortPreview(item.summary, 110) || sourceShortPath(item.source_path)}</small></td>
            <td>{item.project || "—"}</td>
            <td>{item.tool_calls + item.tool_results} · {item.message_count}</td>
            <td>{formatDateTime(item.updated_at || item.started_at, locale)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function RunTable({
  items,
  selectedID,
  onSelect,
  tx,
  locale,
}: {
  items: CodexConsoleRun[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
}) {
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有 Runs。", "No runs yet.")}</p></div>;
  return (
    <table className="codex-console-table">
      <thead>
        <tr>
          <th>{tx("Run", "Run")}</th>
          <th>{tx("工具", "Tools")}</th>
          <th>{tx("浏览器 / 电脑", "Browser / Computer")}</th>
          <th>{tx("更新时间", "Updated")}</th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => (
          <tr key={item.id} className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
            <td><strong>{item.thread_title}</strong><small>{item.project || sourceShortPath(item.source_path)}</small></td>
            <td>{item.tool_calls} / {item.tool_results}</td>
            <td>{item.browser_actions} / {item.computer_actions}</td>
            <td>{formatDateTime(item.updated_at || item.started_at, locale)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function AutomationList({
  items,
  selectedID,
  onSelect,
  tx,
}: {
  items: CodexConsoleAutomation[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
}) {
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有自动化。", "No automations.")}</p></div>;
  return (
    <div className="codex-console-card-list">
      {items.map((item) => (
        <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
          <span className={`codex-console-pill tone-${statusTone(item.status)}`}>{item.status || "unknown"}</span>
          <strong>{item.name}</strong>
          <small>{item.kind || "automation"} · {item.schedule || item.id}</small>
        </button>
      ))}
    </div>
  );
}

function ArtifactList({
  items,
  selectedID,
  onSelect,
  tx,
}: {
  items: CodexConsoleArtifact[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
}) {
  const [projectFilter, setProjectFilter] = useState("all");
  const [roleFilter, setRoleFilter] = useState("all");
  const [query, setQuery] = useState("");
  const projectOptions = useMemo(() => {
    const projects = new Set(items.map((item) => item.project || "unassigned"));
    return Array.from(projects).sort((left, right) => left.localeCompare(right));
  }, [items]);
  const roleOptions = useMemo(() => {
    const roles = new Set(items.map((item) => item.role || "file-reference"));
    return Array.from(roles).sort((left, right) => artifactRoleLabel(left, tx).localeCompare(artifactRoleLabel(right, tx)));
  }, [items, tx]);
  const filteredItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter((item) => {
      const project = item.project || "unassigned";
      const role = item.role || "file-reference";
      if (projectFilter !== "all" && project !== projectFilter) return false;
      if (roleFilter !== "all" && role !== roleFilter) return false;
      if (!needle) return true;
      return [
        item.name,
        item.project,
        item.thread_title,
        item.handoff_note,
        item.agent_instruction,
        item.source_path,
      ].some((value) => (value || "").toLowerCase().includes(needle));
    });
  }, [items, projectFilter, query, roleFilter]);
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有交付物。", "No artifacts.")}</p></div>;
  const visibleItems = filteredItems.slice(0, 300);
  return (
    <div className="codex-console-card-list">
      <div className="codex-console-artifact-filters">
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={tx("搜索交付物、项目或用途", "Search artifacts, projects, or usage")}
        />
        <select value={projectFilter} onChange={(event) => setProjectFilter(event.target.value)}>
          <option value="all">{tx("全部项目", "All projects")}</option>
          {projectOptions.map((project) => (
            <option key={project} value={project}>{project === "unassigned" ? tx("未归属项目", "Unassigned") : project}</option>
          ))}
        </select>
        <select value={roleFilter} onChange={(event) => setRoleFilter(event.target.value)}>
          <option value="all">{tx("全部用途", "All roles")}</option>
          {roleOptions.map((role) => (
            <option key={role} value={role}>{artifactRoleLabel(role, tx)}</option>
          ))}
        </select>
      </div>
      {filteredItems.length > visibleItems.length || filteredItems.length !== items.length ? (
        <div className="dashboard-card-link-muted">
          {tx(`显示 ${visibleItems.length} / ${filteredItems.length} 条，来源 ${items.length} 条`, `Showing ${visibleItems.length} / ${filteredItems.length}, from ${items.length}`)}
        </div>
      ) : null}
      {!filteredItems.length ? (
        <div className="empty-action-state"><p>{tx("没有符合条件的交付物。", "No artifacts match the filters.")}</p></div>
      ) : null}
      {visibleItems.map((item) => (
        <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
          <span className="codex-console-pill tone-neutral">{artifactRoleLabel(item.role, tx)}</span>
          <strong>{item.name}</strong>
          <small>{item.handoff_note || item.project || item.thread_title || sourceShortPath(item.source_path)}</small>
        </button>
      ))}
    </div>
  );
}

function HookList({
  items,
  selectedID,
  onSelect,
  tx,
}: {
  items: CodexConsoleHookRisk[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
}) {
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有 hook 风险资产。", "No hook risk assets.")}</p></div>;
  return (
    <div className="codex-console-card-list">
      {items.map((item) => (
        <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
          <span className={`codex-console-pill tone-${statusTone(item.risk_level || item.status)}`}>{item.risk_level || item.status}</span>
          <strong>{item.name}</strong>
          <small>{item.risk_signals?.length ? item.risk_signals.join(", ") : item.bundle || item.kind}</small>
        </button>
      ))}
    </div>
  );
}

function memoryReviewStatusLabel(status: string | undefined, tx: (zh: string, en: string) => string) {
  const value = (status || "review_required").toLowerCase();
  if (value === "accepted") return tx("已接受", "Accepted");
  if (value === "ignored") return tx("已忽略", "Ignored");
  if (value === "deferred") return tx("已延后", "Deferred");
  return tx("待确认", "Needs review");
}

function memoryCandidateDisplayTitle(item: CodexConsoleMemoryCandidate) {
  const raw = cleanPreviewText(item.content);
  const taskGroup = raw.match(/Task Group:\s*([^#\n]+?)(?:\s+scope:|$)/i);
  if (taskGroup?.[1]) return shortPreview(taskGroup[1].replace(/\s+/g, " ").trim(), 96);

  const frontmatterName = raw.match(/^---[\s\S]*?\nname:\s*["']?([^"'\n]+)["']?/i);
  if (frontmatterName?.[1]) return shortPreview(frontmatterName[1].replace(/[_-]+/g, " ").trim(), 96);

  const heading = raw.match(/^\s{0,3}#{1,3}\s+(.+)$/m);
  if (heading?.[1]) {
    const cleanHeading = heading[1].replace(/`/g, "").replace(/\s+/g, " ").trim();
    if (cleanHeading && !/^raw memories$/i.test(cleanHeading) && !/^ad-hoc notes$/i.test(cleanHeading)) {
      return shortPreview(cleanHeading, 96);
    }
  }

  const title = cleanPreviewText(item.title)
    .replace(/^memories\/(?:rollout_summaries|extensions\/ad_hoc\/notes|skills)\//, "")
    .replace(/^memories\//, "")
    .replace(/\/SKILL$/, "")
    .replace(/^20\d{2}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-[A-Za-z0-9]+-/, "")
    .split("/")
    .pop();
  return shortPreview((title || item.title || "Memory candidate").replace(/[_-]+/g, " ").trim(), 96);
}

function MemoryList({
  items,
  selectedID,
  onSelect,
  tx,
}: {
  items: CodexConsoleMemoryCandidate[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
}) {
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState<"actionable" | "conflict" | "all">("actionable");
  const entries = useMemo(() => {
    return items
      .map((item) => ({
        item,
        title: memoryCandidateDisplayTitle(item),
        summary: cleanMemoryValuePreview(item.content, item.title, tx),
      }))
      .sort((left, right) => {
        const leftPriority = Number(!!left.item.conflict) * 2 + Number(isMemoryActionable(left.item));
        const rightPriority = Number(!!right.item.conflict) * 2 + Number(isMemoryActionable(right.item));
        if (rightPriority !== leftPriority) return rightPriority - leftPriority;
        return left.title.localeCompare(right.title);
      });
  }, [items, tx]);
  const filteredEntries = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return entries.filter((entry) => {
      if (mode === "actionable" && !isMemoryActionable(entry.item)) return false;
      if (mode === "conflict" && !entry.item.conflict) return false;
      if (!needle) return true;
      return [
        entry.title,
        entry.summary,
        entry.item.title,
        entry.item.kind,
        entry.item.source_path,
      ].some((value) => (value || "").toLowerCase().includes(needle));
    });
  }, [entries, mode, query]);
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有记忆候选。", "No memory candidates.")}</p></div>;
  return (
    <>
      <div className="codex-console-list-tools is-stacked">
        <div>
          <strong>{tx("记忆候选", "Memory candidates")}</strong>
          <span>{tx("确认前不会写入长期记忆。", "Nothing is saved as long-term memory before review.")}</span>
        </div>
        <div className="codex-console-memory-filters">
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={tx("搜索候选、项目或关键词", "Search candidates, projects, or keywords")}
          />
          <select value={mode} onChange={(event) => setMode(event.target.value as "actionable" | "conflict" | "all")}>
            <option value="actionable">{tx("待处理", "Actionable")}</option>
            <option value="conflict">{tx("相似记忆", "Overlaps")}</option>
            <option value="all">{tx("全部", "All")}</option>
          </select>
        </div>
      </div>
      <div className="codex-console-card-list">
        {!filteredEntries.length ? (
          <div className="empty-action-state"><p>{tx("没有符合条件的记忆候选。", "No memory candidates match the filters.")}</p></div>
        ) : null}
        {filteredEntries.map(({ item, title, summary }) => (
          <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
            <span className={`codex-console-pill tone-${statusTone(item.review_status)}`}>{memoryReviewStatusLabel(item.review_status, tx)}</span>
            {item.conflict ? <span className="codex-console-pill tone-warn">{tx("相似记忆", "Overlap")}</span> : null}
            <strong>{title}</strong>
            <small>{item.conflict ? item.conflict.message : summary}</small>
          </button>
        ))}
      </div>
    </>
  );
}

function SkillCandidateList({
  items,
  selectedID,
  onSelect,
  tx,
  locale,
  archivedCount,
  showArchived,
  onShowArchivedChange,
}: {
  items: CodexConsoleSkillCandidate[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
  archivedCount: number;
  showArchived: boolean;
  onShowArchivedChange: (value: boolean) => void;
}) {
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"open" | "unsaved" | "draft" | "ready" | "archived" | "all">("open");
  const [projectFilter, setProjectFilter] = useState("all");
  const projectOptions = useMemo(() => {
    const projects = new Set(items.map((item) => item.project || "unassigned"));
    return Array.from(projects).sort((left, right) => left.localeCompare(right));
  }, [items]);
  const visibleItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter((item) => {
      const status = (item.status || "").toLowerCase();
      const project = item.project || "unassigned";
      if (projectFilter !== "all" && project !== projectFilter) return false;
      if (statusFilter === "open" && (status === "ready" || status === "archived")) return false;
      if (statusFilter === "unsaved" && item.skill_path) return false;
      if (statusFilter === "draft" && (!item.skill_path || status === "ready" || status === "archived")) return false;
      if (statusFilter === "ready" && status !== "ready") return false;
      if (statusFilter === "archived" && status !== "archived") return false;
      if (!needle) return true;
      return [
        item.title,
        item.name,
        item.project,
        item.thread_title,
        ...(item.signals || []),
      ].some((value) => (value || "").toLowerCase().includes(needle));
    });
  }, [items, projectFilter, query, statusFilter]);
  useEffect(() => {
    if (!showArchived && statusFilter === "archived") {
      setStatusFilter("open");
    }
  }, [showArchived, statusFilter]);
  useEffect(() => {
    if (showArchived && statusFilter === "open" && items.length > 0 && items.every(isArchivedSkillCandidate)) {
      setStatusFilter("archived");
    }
  }, [items, showArchived, statusFilter]);
  useEffect(() => {
    if (!visibleItems.length) return;
    if (!visibleItems.some((item) => item.id === selectedID)) {
      onSelect(visibleItems[0].id);
    }
  }, [onSelect, selectedID, visibleItems]);
  if (!items.length) {
    return (
      <div className="empty-action-state">
        <p>{archivedCount ? tx("当前只剩已归档草稿。", "Only archived drafts remain.") : tx("还没有候选 Skill。", "No skill candidates.")}</p>
        {archivedCount ? (
          <button className="btn btn-outline" type="button" onClick={() => onShowArchivedChange(true)}>
            {tx("显示已归档", "Show archived")}
          </button>
        ) : null}
      </div>
    );
  }
  return (
    <>
      <div className="codex-console-list-tools is-stacked">
        <div>
          <strong>{tx("Skill 草稿", "Skill drafts")}</strong>
          <span>
            {archivedCount && !showArchived
              ? tx(`当前显示 ${countLabel(items.length)} 条，另有 ${countLabel(archivedCount)} 条归档草稿隐藏`, `Showing ${countLabel(items.length)} drafts, with ${countLabel(archivedCount)} archived hidden`)
              : tx(`当前显示 ${countLabel(items.length)} 条草稿`, `Showing ${countLabel(items.length)} drafts`)}
          </span>
        </div>
        <div className="codex-console-skill-filters">
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={tx("搜索草稿、项目或信号", "Search drafts, projects, or signals")}
          />
          <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as "open" | "unsaved" | "draft" | "ready" | "archived" | "all")}>
            <option value="open">{tx("待审查", "Needs review")}</option>
            <option value="unsaved">{tx("未保存", "Not saved")}</option>
            <option value="draft">{tx("已保存草稿", "Saved drafts")}</option>
            <option value="ready">{tx("已确认", "Ready")}</option>
            {showArchived ? <option value="archived">{tx("已归档", "Archived")}</option> : null}
            <option value="all">{tx("全部状态", "All statuses")}</option>
          </select>
          <select value={projectFilter} onChange={(event) => setProjectFilter(event.target.value)}>
            <option value="all">{tx("全部项目", "All projects")}</option>
            {projectOptions.map((project) => (
              <option key={project} value={project}>{project === "unassigned" ? tx("未归属项目", "Unassigned") : project}</option>
            ))}
          </select>
          {archivedCount ? (
            <button className="btn btn-outline" type="button" onClick={() => onShowArchivedChange(!showArchived)}>
              {showArchived ? tx("隐藏归档", "Hide archived") : tx("显示归档", "Show archived")}
            </button>
          ) : null}
        </div>
      </div>
      <div className="codex-console-card-list">
        {visibleItems.length !== items.length ? (
          <div className="dashboard-card-link-muted">
            {tx(`显示 ${countLabel(visibleItems.length)} / ${countLabel(items.length)} 条草稿`, `Showing ${countLabel(visibleItems.length)} / ${countLabel(items.length)} drafts`)}
          </div>
        ) : null}
        {!visibleItems.length ? (
          <div className="empty-action-state"><p>{tx("没有符合条件的 Skill 草稿。", "No skill drafts match the filters.")}</p></div>
        ) : null}
        {visibleItems.map((item) => (
          <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
            <span className={`codex-console-pill tone-${confidenceTone(item.confidence)}`}>{confidenceLabel(item.confidence)}</span>
            <span className={`codex-console-pill tone-${skillCandidateStatusTone(item)}`}>{skillCandidateStatusLabel(item, tx)}</span>
            <strong>{item.title || item.name}</strong>
            <small>
              {tx(
                `${item.project || "unassigned"} · ${item.tool_calls} 次工具调用 · ${item.artifact_count} 个交付物 · ${formatDateTime(item.updated_at, locale)}`,
                `${item.project || "unassigned"} · ${item.tool_calls} tool calls · ${item.artifact_count} artifacts · ${formatDateTime(item.updated_at, locale)}`,
              )}
            </small>
            {item.signals?.length ? <small>{item.signals.join(" · ")}</small> : null}
          </button>
        ))}
      </div>
    </>
  );
}

function HandoverList({
  items,
  selectedID,
  onSelect,
  tx,
  locale,
}: {
  items: CodexConsoleHandoverSummary[];
  selectedID: string;
  onSelect: (id: string) => void;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
}) {
  if (!items.length) return <div className="empty-action-state"><p>{tx("还没有项目交接摘要。", "No project handover summaries.")}</p></div>;
  return (
    <div className="codex-console-card-list">
      {items.map((item) => (
        <button key={item.id} type="button" className={selectedID === item.id ? "is-selected" : ""} onClick={() => onSelect(item.id)}>
          <span className="codex-console-pill tone-neutral">{item.project}</span>
          {item.path ? <span className="codex-console-pill tone-ok">{tx("已保存", "Saved")}</span> : null}
          <strong>{item.project}</strong>
          <small>
            {tx(
              `${item.thread_count} 线程 · ${item.run_count} 执行 · ${item.artifact_count} 交付物 · ${formatDateTime(item.latest_activity, locale)}`,
              `${item.thread_count} threads · ${item.run_count} runs · ${item.artifact_count} artifacts · ${formatDateTime(item.latest_activity, locale)}`,
            )}
          </small>
        </button>
      ))}
    </div>
  );
}

function ThreadDetail({
  item,
  locale,
  tx,
  copied,
  onCopy,
}: {
  item: CodexConsoleThread;
  locale: "zh-CN" | "en";
  tx: (zh: string, en: string) => string;
  copied: string;
  onCopy: (path?: string) => void;
}) {
  return (
    <>
      <h3>{item.title}</h3>
      <p>{cleanPreviewText(item.summary) || tx("没有摘要。", "No summary.")}</p>
      <dl className="preview-meta">
        <div><dt>Project</dt><dd>{item.project || "—"}</dd></div>
        <div><dt>Messages</dt><dd>{item.message_count}</dd></div>
        <div><dt>Tools</dt><dd>{item.tool_calls} / {item.tool_results}</dd></div>
        <div><dt>Artifacts</dt><dd>{item.artifact_count}</dd></div>
        <div><dt>Updated</dt><dd>{formatDateTime(item.updated_at || item.started_at, locale)}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
      </dl>
      <div className="drawer-actions">
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function RunDetail({
  item,
  locale,
  tx,
  copied,
  onCopy,
}: {
  item: CodexConsoleRun;
  locale: "zh-CN" | "en";
  tx: (zh: string, en: string) => string;
  copied: string;
  onCopy: (path?: string) => void;
}) {
  return (
    <>
      <h3>{item.thread_title}</h3>
      <p>{item.project || formatDateTime(item.updated_at || item.started_at, locale)}</p>
      <dl className="preview-meta">
        <div><dt>Tool calls</dt><dd>{item.tool_calls}</dd></div>
        <div><dt>Tool results</dt><dd>{item.tool_results}</dd></div>
        <div><dt>Browser</dt><dd>{item.browser_actions}</dd></div>
        <div><dt>Computer</dt><dd>{item.computer_actions}</dd></div>
        <div><dt>Errors</dt><dd>{item.errors}</dd></div>
      </dl>
      <div className="codex-console-event-list">
        {(item.events || []).map((event, index) => (
          <div key={`${event.type}-${event.title}-${index}`} className="codex-console-event">
            <strong>{event.title}</strong>
            <span>{event.type} · {formatDateTime(event.at, locale)}</span>
            {event.detail ? <p>{cleanPreviewText(event.detail)}</p> : null}
          </div>
        ))}
      </div>
      <div className="drawer-actions">
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function AutomationDetail({
  item,
  tx,
  copied,
  onCopy,
}: {
  item: CodexConsoleAutomation;
  tx: (zh: string, en: string) => string;
  copied: string;
  onCopy: (path?: string) => void;
}) {
  return (
    <>
      <h3>{item.name}</h3>
      <p>{item.prompt || tx("没有 prompt 摘要。", "No prompt summary.")}</p>
      <dl className="preview-meta">
        <div><dt>Kind</dt><dd>{item.kind || "—"}</dd></div>
        <div><dt>Status</dt><dd>{item.status || "—"}</dd></div>
        <div><dt>Schedule</dt><dd>{item.schedule || "—"}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
      </dl>
      <div className="drawer-actions">
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function ArtifactDetail({
  item,
  registry,
  locale,
  tx,
  copied,
  saving,
  onCopy,
  onSave,
}: {
  item: CodexConsoleArtifact;
  registry: CodexConsoleArtifactRegistrySummary | undefined;
  locale: "zh-CN" | "en";
  tx: (zh: string, en: string) => string;
  copied: string;
  saving: boolean;
  onCopy: (path?: string) => void;
  onSave: () => void;
}) {
  const registryStatus = registry?.status || "not_saved";
  const canSave = (registry?.artifact_count || 0) > 0;
  const handoffPrompt = artifactHandoffPrompt(item, tx);
  return (
    <>
      <h3>{item.name}</h3>
      <p>{item.handoff_note || item.detail || item.thread_title || tx("没有详情。", "No detail.")}</p>
      <div className="codex-console-artifact-guidance">
        <strong>{tx("给下一个 Agent 的用法", "How the next agent should use this")}</strong>
        <p>{item.handoff_note || tx("先确认它是最终交付物、验证证据还是中间文件，再继续任务。", "Confirm whether it is a final deliverable, verification evidence, or intermediate file before continuing.")}</p>
        <PromptTemplateBox text={handoffPrompt} tx={tx} />
      </div>
      <dl className="preview-meta">
        <div><dt>Kind</dt><dd>{item.kind}</dd></div>
        <div><dt>Role</dt><dd>{artifactRoleLabel(item.role, tx)}</dd></div>
        <div><dt>Project</dt><dd>{item.project || "—"}</dd></div>
        <div><dt>Thread</dt><dd>{item.thread_title || "—"}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
        <div><dt>Registry</dt><dd>{artifactRegistryStatusLabel(registryStatus, tx)}</dd></div>
        <div><dt>Registry path</dt><dd><code>{sourceShortPath(registry?.path) || "—"}</code></dd></div>
        <div><dt>Saved</dt><dd>{formatDateTime(registry?.saved_at, locale)}</dd></div>
        <div><dt>Indexed</dt><dd>{tx(`${registry?.artifact_count || 0} 个交付物 / ${registry?.project_count || 0} 个项目`, `${registry?.artifact_count || 0} artifacts / ${registry?.project_count || 0} projects`)}</dd></div>
      </dl>
      <div className="drawer-actions">
        <button className="btn btn-primary" type="button" disabled={!canSave || saving} onClick={onSave}>
          {saving
            ? tx("保存中...", "Saving...")
            : registryStatus === "saved"
              ? tx("更新交付物索引", "Update artifact registry")
              : tx("保存交付物索引", "Save artifact registry")}
        </button>
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function HookDetail({
  item,
  tx,
  copied,
  onCopy,
}: {
  item: CodexConsoleHookRisk;
  tx: (zh: string, en: string) => string;
  copied: string;
  onCopy: (path?: string) => void;
}) {
  return (
    <>
      <h3>{item.name}</h3>
      <p>{item.detail || tx("需要人工审查。", "Manual review required.")}</p>
      <dl className="preview-meta">
        <div><dt>Kind</dt><dd>{item.kind}</dd></div>
        <div><dt>Status</dt><dd>{item.status}</dd></div>
        <div><dt>Risk</dt><dd>{item.risk_level || "—"}</dd></div>
        <div><dt>Shebang</dt><dd><code>{item.shebang || "—"}</code></dd></div>
        <div><dt>Bundle</dt><dd>{item.bundle || "—"}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
      </dl>
      <div className="codex-console-event-list">
        <HookSignalBlock title="Signals" values={item.risk_signals || []} empty="—" />
        <HookSignalBlock title="Env vars" values={item.env_vars || []} empty="—" />
        <HookSignalBlock title="Write paths" values={item.write_path_hints || []} empty="—" />
      </div>
      <div className="drawer-actions">
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function HookSignalBlock({ title, values, empty }: { title: string; values: string[]; empty: string }) {
  return (
    <div className="codex-console-event">
      <strong>{title}</strong>
      <span>{values.length ? values.join(", ") : empty}</span>
    </div>
  );
}

function MemoryDetail({
  item,
  tx,
  locale,
  copied,
  syncing,
  syncTarget,
  syncProject,
  draftContent,
  conflictDraftContent,
  reviewing,
  conflictResolving,
  onCopy,
  onSyncTargetChange,
  onSyncProjectChange,
  onDraftContentChange,
  onDraftReset,
  onConflictDraftChange,
  onConflictDraftReset,
  onSyncSelected,
  onSyncAll,
  onReview,
  onResolveConflict,
}: {
  item: CodexConsoleMemoryCandidate;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
  copied: string;
  syncing: "" | "selected" | "all";
  syncTarget: "profile" | "project";
  syncProject: string;
  draftContent: string;
  conflictDraftContent: string;
  reviewing: string;
  conflictResolving: "" | CodexConsoleMemoryConflictResolution;
  onCopy: (path?: string) => void;
  onSyncTargetChange: (target: "profile" | "project") => void;
  onSyncProjectChange: (project: string) => void;
  onDraftContentChange: (content: string) => void;
  onDraftReset: () => void;
  onConflictDraftChange: (content: string) => void;
  onConflictDraftReset: () => void;
  onSyncSelected: () => void;
  onSyncAll: () => void;
  onReview: (status: "accepted" | "ignored" | "deferred" | "review_required") => void;
  onResolveConflict: (resolution: CodexConsoleMemoryConflictResolution) => void;
}) {
  const busy = !!syncing || !!reviewing || !!conflictResolving;
  const profileConflict = syncTarget === "profile" && !!item.conflict;
  return (
    <>
      <h3>{item.title}</h3>
      <p>{cleanMemoryValuePreview(item.content, item.title, tx)}</p>
      {item.conflict ? (
        <div className="codex-console-conflict-review">
          <div className="alert alert-warn">
            {tx("Profile memory 已有不同来源内容。", "Profile memory already has content from another source.")}
          </div>
          <div className="codex-console-conflict-grid">
            <div>
              <strong>{tx("已有内容", "Existing")}</strong>
              <pre>{item.conflict.existing_content || "—"}</pre>
            </div>
            <div>
              <strong>{tx("候选内容", "Candidate")}</strong>
              <pre>{item.conflict.candidate_content || item.content || "—"}</pre>
            </div>
          </div>
          <div className="codex-console-memory-editor is-conflict">
            <div>
              <strong>{tx("合并后写入内容", "Merged memory to save")}</strong>
              <button className="btn btn-outline" type="button" disabled={busy} onClick={onConflictDraftReset}>
                {tx("恢复合并草稿", "Reset merge draft")}
              </button>
            </div>
            <textarea
              value={conflictDraftContent}
              disabled={busy}
              onChange={(event) => onConflictDraftChange(event.target.value)}
              rows={10}
            />
            <small>{tx("点击“合并并保存”时，这段内容会写入 Profile memory；旧记忆和 Codex 原始记录不会被直接改写。", "When you choose merge and save, this content is written to profile memory; the old memory and original Codex record are not edited directly.")}</small>
          </div>
          <div className="drawer-actions">
            <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onResolveConflict("keep_existing")}>
              {conflictResolving === "keep_existing" ? tx("处理中...", "Resolving...") : tx("保留已有", "Keep existing")}
            </button>
            <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onResolveConflict("use_candidate")}>
              {conflictResolving === "use_candidate" ? tx("处理中...", "Resolving...") : tx("采用候选", "Use candidate")}
            </button>
            <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onResolveConflict("keep_both")}>
              {conflictResolving === "keep_both" ? tx("处理中...", "Resolving...") : tx("两者共存", "Keep both")}
            </button>
            <button className="btn btn-primary" type="button" disabled={busy || !conflictDraftContent.trim()} onClick={() => onResolveConflict("merge")}>
              {conflictResolving === "merge" ? tx("处理中...", "Resolving...") : tx("合并并保存", "Merge and save")}
            </button>
          </div>
        </div>
      ) : null}
      <dl className="preview-meta">
        <div><dt>Kind</dt><dd>{item.kind}</dd></div>
        <div><dt>Status</dt><dd>{item.review_status || "review_required"}</dd></div>
        <div><dt>Confidence</dt><dd>{item.confidence ? `${Math.round(item.confidence * 100)}%` : "—"}</dd></div>
        <div><dt>Memory path</dt><dd><code>{item.memory_path || "—"}</code></dd></div>
        <div><dt>Conflict</dt><dd>{item.conflict ? item.conflict.category : "—"}</dd></div>
        <div><dt>Reviewed</dt><dd>{item.reviewed_at ? formatDateTime(item.reviewed_at, locale) : "—"}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
      </dl>
      {item.review_note ? <p>{item.review_note}</p> : null}
      <div className="codex-console-memory-editor">
        <div>
          <strong>{tx("写入前可编辑", "Edit before saving")}</strong>
          <button className="btn btn-outline" type="button" disabled={busy} onClick={onDraftReset}>
            {tx("恢复候选原文", "Reset candidate")}
          </button>
        </div>
        <textarea
          value={draftContent}
          disabled={busy}
          onChange={(event) => onDraftContentChange(event.target.value)}
          rows={8}
        />
        <small>{tx("只影响这次同步，不会改 Codex 原始记录。", "Only affects this sync; it does not change the original Codex record.")}</small>
      </div>
      <div className="codex-console-sync-target">
        <label>
          <span>{tx("同步目标", "Sync target")}</span>
          <select value={syncTarget} onChange={(event) => onSyncTargetChange(event.target.value === "project" ? "project" : "profile")}>
            <option value="profile">{tx("Profile memory", "Profile memory")}</option>
            <option value="project">{tx("Project context", "Project context")}</option>
          </select>
        </label>
        {syncTarget === "project" ? (
          <label>
            <span>{tx("项目名", "Project")}</span>
            <input value={syncProject} onChange={(event) => onSyncProjectChange(event.target.value)} placeholder="vola" />
          </label>
        ) : null}
      </div>
      <div className="drawer-actions">
        <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onReview("accepted")}>
          {reviewing === "accepted" ? tx("更新中...", "Updating...") : tx("接受候选", "Accept")}
        </button>
        <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onReview("deferred")}>
          {reviewing === "deferred" ? tx("更新中...", "Updating...") : tx("延后", "Defer")}
        </button>
        <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onReview("ignored")}>
          {reviewing === "ignored" ? tx("更新中...", "Updating...") : tx("忽略", "Ignore")}
        </button>
        {(item.review_status || "review_required") !== "review_required" ? (
          <button className="btn btn-outline" type="button" disabled={busy} onClick={() => onReview("review_required")}>
            {reviewing === "review_required" ? tx("更新中...", "Updating...") : tx("重新待审", "Mark pending")}
          </button>
        ) : null}
        <button className="btn btn-primary" type="button" disabled={busy || profileConflict} onClick={onSyncSelected}>
          {syncing === "selected" ? tx("同步中...", "Syncing...") : tx("同步选中到长期记忆", "Sync selected")}
        </button>
        <button className="btn btn-outline" type="button" disabled={busy} onClick={onSyncAll}>
          {syncing === "all" ? tx("同步中...", "Syncing...") : tx("同步全部候选", "Sync all")}
        </button>
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function cleanHandoverDraftText(value: string | undefined) {
  return (value || "")
    .replace(/\uFFFD/g, "")
    .trim();
}

function cleanSkillDraftText(value: string | undefined) {
  return (value || "")
    .replace(/\uFFFD/g, "")
    .trim();
}

function buildHandoverDraft(item: CodexConsoleHandoverSummary, tx: (zh: string, en: string) => string) {
  if (cleanHandoverDraftText(item.saved_content)) {
    return cleanHandoverDraftText(item.saved_content);
  }
  const lines = [
    `# Codex handover: ${item.project || "unassigned"}`,
    "",
    "## Next agent briefing",
    "",
    cleanPreviewText(item.summary) || tx("请在这里写清楚当前项目状态、已完成内容和下一步要注意的地方。", "Describe current project status, completed work, and what the next agent should watch."),
    "",
    "## Current state",
    "",
    `- Latest activity: ${item.latest_activity || "unknown"}`,
    `- Threads: ${item.thread_count}`,
    `- Runs: ${item.run_count}`,
    `- Artifacts: ${item.artifact_count}`,
    `- Memory candidates: ${item.memory_candidate_count}`,
    "",
  ];
  appendHandoverDraftGroup(lines, "Recent threads", item.recent_threads || [], "No recent threads.");
  appendHandoverDraftGroup(lines, "Runs", item.recent_runs || [], "No runs.");
  appendHandoverDraftGroup(lines, "Artifacts", item.recent_artifacts || [], "No artifacts.");
  appendHandoverDraftGroup(lines, "Memory candidates", item.memory_candidates || [], "No memory candidates.");
  lines.push(
    "## Manual notes",
    "",
    tx("这里写给下一个 Agent 的人工说明，例如不要重复验证什么、先看哪个文件、哪些判断还没确认。", "Add human notes for the next agent, such as what not to re-check, which file to read first, and which assumptions are still open."),
  );
  return lines.join("\n").trim();
}

function appendHandoverDraftGroup(lines: string[], title: string, items: CodexConsoleHandoverItem[], empty: string) {
  lines.push(`## ${title}`, "");
  if (!items.length) {
    lines.push(`- ${empty}`, "");
    return;
  }
  for (const item of items) {
    const parts = [item.kind, item.at].filter(Boolean).join(", ");
    lines.push(`- ${cleanPreviewText(item.title) || item.id}${parts ? ` (${parts})` : ""}`);
    if (cleanPreviewText(item.detail)) lines.push(`  - Detail: ${cleanPreviewText(item.detail)}`);
    if (item.source_path) lines.push(`  - Source: \`${item.source_path.replace(/`/g, "'")}\``);
  }
  lines.push("");
}

function HandoverDetail({
  item,
  tx,
  locale,
  saving,
  draftContent,
  onDraftChange,
  onDraftReset,
  onSave,
}: {
  item: CodexConsoleHandoverSummary;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
  saving: boolean;
  draftContent: string;
  onDraftChange: (value: string) => void;
  onDraftReset: () => void;
  onSave: (id: string) => void;
}) {
  return (
    <>
      <h3>{item.project}</h3>
      <p>{cleanPreviewText(item.summary)}</p>
      <dl className="preview-meta">
        <div><dt>Threads</dt><dd>{item.thread_count}</dd></div>
        <div><dt>Runs</dt><dd>{item.run_count}</dd></div>
        <div><dt>Artifacts</dt><dd>{item.artifact_count}</dd></div>
        <div><dt>Memory</dt><dd>{item.memory_candidate_count}</dd></div>
        <div><dt>Latest</dt><dd>{formatDateTime(item.latest_activity, locale)}</dd></div>
        <div><dt>Status</dt><dd>{item.path ? tx("已保存", "Saved") : tx("未保存", "Not saved")}</dd></div>
        <div><dt>Version</dt><dd>{item.version ? String(item.version) : "—"}</dd></div>
        <div><dt>Path</dt><dd><code>{item.path || "—"}</code></dd></div>
        <div><dt>Saved</dt><dd>{formatDateTime(item.saved_at, locale)}</dd></div>
      </dl>
      <div className="codex-console-memory-editor">
        <div>
          <strong>{tx("交接文件内容", "Handover file content")}</strong>
          <button className="btn btn-outline" type="button" onClick={onDraftReset}>{tx("恢复当前草稿", "Reset draft")}</button>
        </div>
        <textarea
          value={draftContent}
          onChange={(event) => onDraftChange(event.target.value)}
          aria-label={tx("编辑项目交接文件内容", "Edit project handover file content")}
        />
        <small>{tx("保存后会写入项目资料，并保留 FileTree 版本记录。", "Saving writes this into project data and keeps FileTree version history.")}</small>
      </div>
      <div className="codex-console-event-list">
        <HandoverItemGroup title={tx("最近线程", "Recent threads")} items={item.recent_threads || []} locale={locale} empty={tx("没有线程。", "No threads.")} />
        <HandoverItemGroup title={tx("执行记录", "Runs")} items={item.recent_runs || []} locale={locale} empty={tx("没有执行记录。", "No runs.")} />
        <HandoverItemGroup title={tx("交付物", "Artifacts")} items={item.recent_artifacts || []} locale={locale} empty={tx("没有交付物。", "No artifacts.")} />
        <HandoverItemGroup title={tx("记忆候选", "Memory candidates")} items={item.memory_candidates || []} locale={locale} empty={tx("没有记忆候选。", "No memory candidates.")} />
      </div>
      <div className="drawer-actions">
        <button className="btn btn-primary" type="button" disabled={saving || !item.id || !cleanHandoverDraftText(draftContent)} onClick={() => onSave(item.id)}>
          {saving ? tx("保存中...", "Saving...") : item.path ? tx("保存编辑后的交接文件", "Save edited handover") : tx("保存为项目交接文件", "Save project handover")}
        </button>
      </div>
    </>
  );
}

function SkillCandidateDetail({
  item,
  tx,
  locale,
  copied,
  saving,
  statusUpdating,
  draftContent,
  assigning,
  assignPreview,
  agentOptions,
  selectedAgentIDs,
  onDraftChange,
  onDraftReset,
  onSave,
  onAgentToggle,
  onAssignAgents,
  onStatusUpdate,
  onCopyDraft,
  onCopy,
}: {
  item: CodexConsoleSkillCandidate;
  tx: (zh: string, en: string) => string;
  locale: "zh-CN" | "en";
  copied: string;
  saving: boolean;
  statusUpdating: string;
  draftContent: string;
  assigning: boolean;
  assignPreview: CodexConsoleSkillCandidateAssignPreviewResponse | null;
  agentOptions: SkillCandidateAgentOption[];
  selectedAgentIDs: SkillCandidateAgentID[];
  onDraftChange: (value: string) => void;
  onDraftReset: () => void;
  onSave: (id: string) => void;
  onAgentToggle: (agentID: SkillCandidateAgentID) => void;
  onAssignAgents: (id: string, agentIDs: string[]) => void;
  onStatusUpdate: (id: string, status: "draft" | "ready" | "archived") => void;
  onCopyDraft: (id: string, draft: string) => void;
  onCopy: (path?: string) => void;
}) {
  const draftMarker = `skill-draft:${item.id}`;
  const saved = !!item.skill_path;
  const selectedCount = selectedAgentIDs.length;
  const currentStatus = (item.status || "").toLowerCase();
  return (
    <>
      <h3>{item.title || item.name}</h3>
      <p>{cleanPreviewText(item.rationale) || tx("来自成功完成的 Codex 线程，安装前需要人工审查。", "Generated from a successful Codex thread and needs review before installation.")}</p>
      <dl className="preview-meta">
        <div><dt>Name</dt><dd><code>{item.name || "—"}</code></dd></div>
        <div><dt>Status</dt><dd>{skillCandidateStatusLabel(item, tx)}</dd></div>
        <div><dt>Skill path</dt><dd><code>{item.skill_path || "—"}</code></dd></div>
        <div><dt>Project</dt><dd>{item.project || "—"}</dd></div>
        <div><dt>Thread</dt><dd>{item.thread_title || item.thread_id || "—"}</dd></div>
        <div><dt>Confidence</dt><dd>{confidenceLabel(item.confidence)}</dd></div>
        <div><dt>Tool calls</dt><dd>{item.tool_calls}</dd></div>
        <div><dt>Artifacts</dt><dd>{item.artifact_count}</dd></div>
        <div><dt>Updated</dt><dd>{formatDateTime(item.updated_at, locale)}</dd></div>
        <div><dt>Saved</dt><dd>{formatDateTime(item.saved_at, locale)}</dd></div>
        <div><dt>Source</dt><dd><code>{sourceShortPath(item.source_path) || "—"}</code></dd></div>
      </dl>
      <div className="codex-console-event-list">
        <HookSignalBlock title="Signals" values={item.signals || []} empty="—" />
      </div>
      <div className="codex-console-skill-draft">
        <div className="codex-console-skill-draft-head">
          <strong>{tx("Skill 草稿内容", "Skill draft content")}</strong>
          <button className="btn btn-outline" type="button" disabled={saving} onClick={onDraftReset}>
            {tx("恢复当前草稿", "Reset draft")}
          </button>
        </div>
        <textarea
          value={draftContent}
          disabled={saving}
          onChange={(event) => onDraftChange(event.target.value)}
          aria-label={tx("编辑 Skill 草稿内容", "Edit skill draft content")}
          rows={16}
        />
        <small>{tx("保存后会写入 Hub 候选 Skill，不会自动安装到 Codex 或启用给 Agent。", "Saving writes this into Hub skill candidates; it is not installed into Codex or enabled for any Agent automatically.")}</small>
      </div>
      <div className="codex-console-skill-review">
        <div>
          <strong>{tx("审查状态", "Review status")}</strong>
          <span>{saved ? skillCandidateStatusLabel(item, tx) : tx("先保存草稿", "Save draft first")}</span>
        </div>
        <p>{tx("确认有价值的草稿可以标为可分配；不值得继续看的草稿可以归档。归档只隐藏默认列表，不删除 Hub 文件。", "Useful drafts can be marked ready; low-value drafts can be archived. Archiving only hides them from the default list and does not delete Hub files.")}</p>
        {item.status_note ? <small>{item.status_note}</small> : null}
        {item.status_updated_at ? <small>{tx(`状态更新时间：${formatDateTime(item.status_updated_at, locale)}`, `Status updated: ${formatDateTime(item.status_updated_at, locale)}`)}</small> : null}
        <div className="drawer-actions">
          <button className="btn btn-outline" type="button" disabled={!saved || saving || !!statusUpdating || currentStatus === "ready"} onClick={() => onStatusUpdate(item.id, "ready")}>
            {statusUpdating === "ready" ? tx("更新中...", "Updating...") : tx("标为可分配", "Mark ready")}
          </button>
          <button className="btn btn-outline" type="button" disabled={!saved || saving || !!statusUpdating || currentStatus === "draft" || !currentStatus} onClick={() => onStatusUpdate(item.id, "draft")}>
            {statusUpdating === "draft" ? tx("更新中...", "Updating...") : tx("恢复草稿", "Restore draft")}
          </button>
          <button className="btn btn-outline" type="button" disabled={!saved || saving || !!statusUpdating || currentStatus === "archived"} onClick={() => onStatusUpdate(item.id, "archived")}>
            {statusUpdating === "archived" ? tx("归档中...", "Archiving...") : tx("归档草稿", "Archive draft")}
          </button>
        </div>
      </div>
      <div className="codex-console-skill-assignment">
        <div>
          <strong>{tx("Agent 分配预览", "Agent assignment preview")}</strong>
          <span>{saved ? tx(`已选择 ${selectedCount} 个目标`, `${selectedCount} selected`) : tx("先保存草稿", "Save draft first")}</span>
        </div>
        <p>{tx("Vola 会把这份 Skill 作为同一份资产分配给所选 Agent。Codex 和 Claude Code 生成本地同步预览，Cursor 和 Gemini CLI 生成导出预览；这里不会写入本机目录。", "Vola assigns this Skill asset to the selected Agents. Codex and Claude Code get local sync previews; Cursor and Gemini CLI get export previews. Nothing is written locally here.")}</p>
        <div className="codex-console-agent-options" role="group" aria-label={tx("选择 Agent 目标", "Select Agent targets")}>
          {agentOptions.map((agent) => {
            const checked = selectedAgentIDs.includes(agent.id);
            const disableLastChecked = checked && selectedAgentIDs.length <= 1;
            return (
              <label className={`codex-console-agent-option${checked ? " selected" : ""}`} key={agent.id}>
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={assigning || disableLastChecked}
                  onChange={() => onAgentToggle(agent.id)}
                />
                <span>
                  <strong>{agent.name}</strong>
                  <em>{tx(agent.modeZh, agent.modeEn)}</em>
                  <small>{tx(agent.noteZh, agent.noteEn)}</small>
                </span>
              </label>
            );
          })}
        </div>
        {assignPreview ? <SkillCandidateAssignPreview result={assignPreview} tx={tx} /> : null}
        <div className="drawer-actions">
          <button className="btn btn-outline" type="button" disabled={!saved || saving || assigning || selectedCount === 0} onClick={() => onAssignAgents(item.id, selectedAgentIDs)}>
            {assigning ? tx("预览中...", "Previewing...") : tx("加入所选 Agent 分配并预览", "Assign selected Agents and preview")}
          </button>
          <Link className="btn btn-outline" to="/skills">
            {tx("打开 Skill 分配页", "Open skill assignments")}
          </Link>
        </div>
      </div>
      <div className="drawer-actions">
        <button className="btn btn-primary" type="button" disabled={saving || !item.id || !cleanSkillDraftText(draftContent)} onClick={() => onSave(item.id)}>
          {saving ? tx("保存中...", "Saving...") : saved ? tx("保存编辑后的 Skill 草稿", "Save edited skill draft") : tx("保存为 Vola Skill 草稿", "Save as Vola skill draft")}
        </button>
        <button className="btn btn-outline" type="button" disabled={!cleanSkillDraftText(draftContent)} onClick={() => onCopyDraft(item.id, cleanSkillDraftText(draftContent))}>
          {copied === draftMarker ? tx("已复制", "Copied") : tx("复制草稿", "Copy draft")}
        </button>
        <button className="btn btn-outline" type="button" disabled={!item.source_path} onClick={() => onCopy(item.source_path)}>
          {copied === item.source_path ? tx("已复制", "Copied") : tx("复制来源路径", "Copy source path")}
        </button>
      </div>
    </>
  );
}

function SkillCandidateAssignPreview({
  result,
  tx,
}: {
  result: CodexConsoleSkillCandidateAssignPreviewResponse;
  tx: (zh: string, en: string) => string;
}) {
  const agents = result.sync_preview?.agents || [];
  if (agents.length === 0) {
    return <div className="alert alert-warn">{result.message || tx("没有生成 Agent 预览。", "No Agent preview was generated.")}</div>;
  }
  return (
    <div className="codex-console-skill-preview">
      <div className="codex-console-skill-preview-head">
        <strong>{tx("已生成多 Agent 预览", "Multi-Agent preview ready")}</strong>
        <span>{tx(result.sync_preview?.applied ? "已写入" : "未写入本地", result.sync_preview?.applied ? "Applied" : "Not written locally")}</span>
      </div>
      <div className="codex-console-skill-agent-preview-list">
        {agents.map((agent) => (
          <SkillCandidateAgentPreview agent={agent} tx={tx} key={agent.agent_id} />
        ))}
      </div>
      <small>{tx("这里只更新 Vola 的分配关系并展示结果。要写入 Codex 或 Claude Code 的本地目录，请到 Skill 分配页执行本地同步；Cursor 和 Gemini CLI 走导出包。", "This updates Vola's assignment state and shows the result. Use the Skill assignments page to write Codex or Claude Code local directories; Cursor and Gemini CLI use export packages.")}</small>
    </div>
  );
}

function SkillCandidateAgentPreview({
  agent,
  tx,
}: {
  agent: LocalSkillSyncAgentPlan;
  tx: (zh: string, en: string) => string;
}) {
  const visibleChanges = agent.changes.filter((change) => change.action !== "marker").slice(0, 5);
  const mode = agent.supported ? tx("本地同步", "Local sync") : tx("导出包", "Export package");
  const target = agent.target_root || agent.export_file_name || agent.detected_roots?.[0]?.path || "—";
  const manual = agent.summary.manual_required || 0;
  return (
    <section className="codex-console-skill-agent-preview">
      <div className="codex-console-skill-agent-preview-head">
        <strong>{agent.name || skillCandidateAgentName(agent.agent_id)}</strong>
        <span>{mode}</span>
      </div>
      <dl className="preview-meta">
        <div><dt>{tx("目标", "Target")}</dt><dd><code>{target}</code></dd></div>
        <div><dt>{tx("新增", "Add")}</dt><dd>{agent.summary.add}</dd></div>
        <div><dt>{tx("更新", "Update")}</dt><dd>{agent.summary.update}</dd></div>
        <div><dt>{tx("导出", "Export")}</dt><dd>{agent.summary.export}</dd></div>
        <div><dt>{tx("冲突", "Conflict")}</dt><dd>{agent.summary.conflict}</dd></div>
        <div><dt>{tx("人工审查", "Review")}</dt><dd>{manual}</dd></div>
      </dl>
      {!agent.supported && agent.auto_apply_reason ? (
        <p>{agent.auto_apply_reason}</p>
      ) : null}
      {agent.summary.conflict > 0 ? (
        <div className="alert alert-warn">{tx("存在同名本地内容。Vola 预览不会覆盖未由它管理的目录。", "A local name conflict exists. Vola will not overwrite directories it does not manage.")}</div>
      ) : null}
      {visibleChanges.length > 0 ? (
        <div className="codex-console-skill-preview-list">
          {visibleChanges.map((change, index) => (
            <div key={`${agent.agent_id}-${change.action}-${change.rel_path || change.target_path || index}`}>
              <span>{syncActionLabel(change.action, tx)}</span>
              <code>{[change.skill_path, change.rel_path].filter(Boolean).join("/") || change.target_path || "—"}</code>
            </div>
          ))}
        </div>
      ) : (
        <small>{agent.message || tx("没有需要展示的变更。", "No changes to show.")}</small>
      )}
      {agent.errors?.length ? (
        <div className="alert alert-error">{agent.errors.join("; ")}</div>
      ) : null}
    </section>
  );
}

function HandoverItemGroup({
  title,
  items,
  locale,
  empty,
}: {
  title: string;
  items: CodexConsoleHandoverItem[];
  locale: "zh-CN" | "en";
  empty: string;
}) {
  return (
    <div className="codex-console-event">
      <strong>{title}</strong>
      {items.length ? (
        <div className="codex-console-handover-items">
          {items.map((item) => (
            <div key={item.id} className="codex-console-handover-item">
              <span>{item.title}</span>
              <small>{[item.kind, formatDateTime(item.at, locale), item.detail].filter(Boolean).join(" · ")}</small>
              {item.source_path ? <code>{sourceShortPath(item.source_path)}</code> : null}
            </div>
          ))}
        </div>
      ) : (
        <span>{empty}</span>
      )}
    </div>
  );
}
