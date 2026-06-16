export let API_BASE = "/api";

function normalizeApiBase(value?: string | null) {
  const raw = `${value || ""}`.trim().replace(/\/+$/, "");
  if (!raw) return "/api";
  if (raw === "/api" || raw.endsWith("/api")) return raw;
  if (/^https?:\/\//i.test(raw)) return `${raw}/api`;
  return raw;
}

function detectTauriRuntime() {
  if (typeof window === "undefined") return false;
  const protocol = window.location?.protocol || "";
  const hostname = window.location?.hostname || "";
  const runtime = window as any;
  return !!runtime.__TAURI__ || !!runtime.__TAURI_INTERNALS__ || protocol === "tauri:" || hostname === "tauri.localhost";
}

export const isTauri = detectTauriRuntime();

export let apiBasePromise: Promise<string> | null = null;

type TauriInvoke = <T>(command: string, args?: Record<string, unknown>) => Promise<T>;
type AuthStorageKey = "token" | "refresh_token";
let memoryAccessToken = "";
let localOwnerTokenPromise: Promise<string | null> | null = null;

export interface CliToolsInstallResult {
  source_path: string;
  install_dir: string;
  commands: string[];
  command_paths: string[];
  path_updated: boolean;
  rc_file?: string | null;
  shell_reload_command?: string | null;
}

function sanitizeAuthValue(value?: string | null) {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed || !/^[\x21-\x7E]+$/.test(trimmed)) {
    return null;
  }
  return trimmed;
}

function readHeaderSafeAuthValue(key: AuthStorageKey) {
  const value = localStorage.getItem(key);
  const trimmed = sanitizeAuthValue(value);
  if (!trimmed) {
    localStorage.removeItem(key);
    if (key === "token" && memoryAccessToken) return memoryAccessToken;
    return null;
  }
  if (trimmed !== value) localStorage.setItem(key, trimmed);
  return trimmed;
}

function storeAccessToken(token?: string | null) {
  const trimmed = sanitizeAuthValue(token);
  if (!trimmed) {
    localStorage.removeItem("token");
    memoryAccessToken = "";
    return null;
  }
  memoryAccessToken = trimmed;
  localStorage.setItem("token", trimmed);
  return trimmed;
}

function clearStoredAuth() {
  memoryAccessToken = "";
  localStorage.removeItem("token");
  localStorage.removeItem("refresh_token");
}

function authHeadersForToken(token: string | null): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

function authHeaders(): Record<string, string> {
  return authHeadersForToken(readHeaderSafeAuthValue("token"));
}

function getGlobalTauriInvoke(): TauriInvoke | null {
  if (typeof window === "undefined") return null;
  const tauri = (window as any).__TAURI__;
  return tauri?.core?.invoke || tauri?.invoke || null;
}

export async function installCliTools(): Promise<CliToolsInstallResult> {
  if (!isTauri) {
    throw new Error("CLI installer is only available in the Vola desktop app.");
  }
  const globalInvoke = getGlobalTauriInvoke();
  const invoke = globalInvoke || (await import('@tauri-apps/api/core')).invoke;
  return invoke<CliToolsInstallResult>("install_cli_tools");
}

async function resolveTauriApiBase() {
  try {
    const globalInvoke = getGlobalTauriInvoke();
    const invoke = globalInvoke || (await import('@tauri-apps/api/core')).invoke;
    const url = await invoke<string>("get_api_base");
    API_BASE = normalizeApiBase(url);
    console.log("Tauri backend API base initialized:", API_BASE);
    return API_BASE;
  } catch (err) {
    console.error("Failed to get API base from Tauri:", err);
    throw err;
  }
}

async function ensureTauriApiBase() {
  if (!isTauri) return;
  if (!apiBasePromise) apiBasePromise = resolveTauriApiBase();
  try {
    await apiBasePromise;
  } catch (err) {
    apiBasePromise = null;
    throw err;
  }
}

function shouldAutoBootstrapLocalOwner(path: string) {
  if (!isTauri) return false;
  if (path === "/config" || path === "/local/owner-token") return false;
  if (path === "/auth/login" || path === "/auth/register" || path === "/auth/refresh" || path === "/auth/logout") return false;
  if (path.startsWith("/auth/providers/")) return false;
  return true;
}

async function bootstrapLocalOwnerAccessToken() {
  if (!isTauri) return null;
  if (!localOwnerTokenPromise) {
    localOwnerTokenPromise = (async () => {
      await ensureTauriApiBase();
      const res = await fetch(`${API_BASE}/local/owner-token`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      if (!res.ok) return null;
      const json = await res.json().catch(() => null);
      const token = json?.data?.token || json?.token || null;
      return storeAccessToken(token);
    })().finally(() => {
      localOwnerTokenPromise = null;
    });
  }
  return localOwnerTokenPromise;
}


function parseDownloadFilename(
  contentDisposition: string | null,
  fallback: string,
) {
  if (!contentDisposition) return fallback;
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]);
    } catch {
      return utf8Match[1];
    }
  }
  const quotedMatch = contentDisposition.match(/filename="([^"]+)"/i);
  if (quotedMatch?.[1]) return quotedMatch[1];
  const plainMatch = contentDisposition.match(/filename=([^;]+)/i);
  if (plainMatch?.[1]) return plainMatch[1].trim();
  return fallback;
}

function triggerBrowserDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}

// ---------------------------------------------------------------------------
// Auth types
// ---------------------------------------------------------------------------

export interface RegisterRequest {
  email: string;
  password: string;
  display_name: string;
  slug: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface AuthProvider {
  id: string;
  kind: string;
  display_name: string;
  enabled: boolean;
}

export type AuthProviderAction = "login" | "signup";

export interface StartAuthProviderResponse {
  authorization_url: string;
}

export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user: any;
}

export interface Session {
  id: string;
  user_id: string;
  user_agent: string;
  ip_address: string;
  expires_at: string;
  created_at: string;
}

export interface ConnectionResponse {
  id: string;
  name: string;
  platform: string;
  trust_level: number;
  api_key_prefix?: string;
  last_used_at?: string;
  created_at?: string;
}

export interface OAuthAppResponse {
  id: string;
  name: string;
  client_id: string;
  redirect_uris: string[];
  scopes: string[];
  description: string;
  logo_url?: string;
  is_active: boolean;
  created_at: string;
}

export interface OAuthGrantResponse {
  id: string;
  app: OAuthAppResponse;
  scopes: string[];
  created_at: string;
}

export interface FileNode {
  path: string;
  name: string;
  is_dir: boolean;
  source?: string;
  kind?: string;
  content?: string;
  mime_type?: string;
  size?: number;
  version?: number;
  checksum?: string;
  metadata?: Record<string, any>;
  bundle_context?: BundleContext;
  min_trust_level?: number;
  children?: FileNode[];
  created_at?: string;
  updated_at?: string;
  deleted_at?: string;
}

export interface TreeSnapshotResponse {
  path: string;
  cursor: number;
  root_checksum: string;
  entries: FileNode[];
}

export interface DashboardActivity {
  platform: string;
  count: number;
}

export interface DashboardPending {
  type: string;
  count: number;
  message: string;
}

export interface PublicConfig {
  github_client_id?: string;
  github_enabled?: boolean;
  github_app_enabled?: boolean;
  github_app_slug?: string;
  billing_enabled?: boolean;
  public_registration_enabled?: boolean;
  storage?: string;
  local_mode?: boolean;
  system_settings_enabled?: boolean;
  git_mirror_execution_mode?: "local" | "hosted";
  git_mirror_manual_sync_cooldown_seconds?: number;
}

export interface BillingPlan {
  code: BillingPlanCode
  name: string
  currency: string
  price_cents: number
  interval: string
  storage_limit_bytes: number
}

export type BillingPlanCode = 'free' | 'pro_monthly' | 'pro_yearly'
export type PaidBillingPlanCode = Exclude<BillingPlanCode, 'free'>
export type BillingAccessSource = 'free' | 'stripe'

export interface BillingStatus {
  current_plan: BillingPlanCode
  entitlement_status: 'active' | 'grace'
  subscription_status?: string
  current_period_end?: string
  access_source: BillingAccessSource
  used_bytes: number
  limit_bytes: number
  usage_measured_at?: string
  account_read_only: boolean
  plans: BillingPlan[]
  can_checkout: boolean
  can_manage_portal: boolean
}

export interface DashboardStats {
  connections: number;
  files: number;
  memory: number;
  profile: number;
  conversations: number;
  skills: number;
  projects: number;
  weekly_activity: DashboardActivity[];
  pending: DashboardPending[];
}

export interface LocalLibraryScanRequest {
  roots?: string[];
  max_markdown?: number;
  max_projects?: number;
}

export interface LocalLibraryRoot {
  path: string;
  exists: boolean;
  scanned: boolean;
  error?: string;
}

export interface LocalLibraryStats {
  roots_requested: number;
  roots_scanned: number;
  markdown_found: number;
  markdown_shown: number;
  projects_found: number;
  projects_shown: number;
  dirs_skipped: number;
  files_scanned: number;
  sensitive_files: number;
}

export interface LocalLibraryProjectCandidate {
  name: string;
  path: string;
  score: number;
  markers: string[];
  reasons: string[];
  markdown_count: number;
  updated_at?: string;
}

export interface LocalLibraryMarkdownCandidate {
  title: string;
  path: string;
  project_name?: string;
  project_path?: string;
  category: string;
  generic_candidate: boolean;
  sensitive_candidate: boolean;
  size_bytes: number;
  updated_at?: string;
  score: number;
  headings?: string[];
  excerpt?: string;
}

export interface LocalLibraryScanResponse {
  version: string;
  generated_at: string;
  roots: LocalLibraryRoot[];
  stats: LocalLibraryStats;
  projects: LocalLibraryProjectCandidate[];
  markdown: LocalLibraryMarkdownCandidate[];
  warnings?: string[];
}

export interface LocalLibraryImportResponse {
  version: string;
  generated_at: string;
  project_name: string;
  paths: string[];
  stats: LocalLibraryStats;
  warnings?: string[];
}

export function normalizeDashboardStats(stats?: Partial<DashboardStats> | null): DashboardStats {
  return {
    connections: Number(stats?.connections || 0),
    files: Number(stats?.files || 0),
    memory: Number(stats?.memory || 0),
    profile: Number(stats?.profile || 0),
    conversations: Number(stats?.conversations || 0),
    skills: Number(stats?.skills || 0),
    projects: Number(stats?.projects || 0),
    weekly_activity: Array.isArray(stats?.weekly_activity) ? stats.weekly_activity : [],
    pending: Array.isArray(stats?.pending) ? stats.pending : [],
  };
}

export interface SkillSummary {
  name: string;
  path: string;
  bundle_path?: string;
  primary_path?: string;
  source: string;
  read_only?: boolean;
  description?: string;
  when_to_use?: string;
  capabilities?: string[];
  tags?: string[];
  min_trust_level?: number;
}

export interface SkillLearningStats {
  skills: number;
  ready: number;
  needs_summary: number;
  needs_validation: number;
  rich_assets: number;
  assigned: number;
  sync_risk?: number;
  quality_blocked?: number;
  quality_manual_required?: number;
  quality_warnings?: number;
}

export interface SkillQualityStats {
  passed: number;
  warnings: number;
  manual_required: number;
  blocked: number;
}

export interface SkillQualityFinding {
  code: string;
  status: string;
  severity: string;
  category: string;
  title: string;
  message: string;
  path?: string;
  agent_id?: string;
}

export interface SkillLearningItem {
  name: string;
  path: string;
  primary_path?: string;
  source: string;
  status: "ready" | "needs_summary" | "needs_validation" | string;
  score: number;
  has_summary: boolean;
  has_when_to_use: boolean;
  has_manifest: boolean;
  has_scripts: boolean;
  has_dependencies: boolean;
  has_external_refs: boolean;
  assigned_agents?: string[];
  recommendations?: string[];
  verification_needed: boolean;
  verification_status?: string;
  quality_status?: string;
  quality_stats?: SkillQualityStats;
  quality_findings?: SkillQualityFinding[];
  updated_at?: string;
  match_score?: number;
  match_reasons?: string[];
  notes?: SkillLearningNote[];
}

export interface SkillLearningAction {
  code: string;
  label: string;
  count: number;
  message: string;
}

export interface LearningRunStep {
  name: string;
  status: string;
  error?: string;
}

export interface LearningRunModel {
  provider_id: string;
  model?: string;
  prompt_version: string;
}

export interface LearningRunOutput {
  run_path: string;
  report_path: string;
  legacy_path?: string;
  proposal_dir?: string;
  skill_map_path?: string;
}

export interface LearningRun {
  version: string;
  id: string;
  status: string;
  started_at: string;
  finished_at?: string;
  steps: LearningRunStep[];
  input_paths: string[];
  model?: LearningRunModel;
  outputs: LearningRunOutput;
  error?: string;
}

export interface SkillLearningSummary {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  stats: SkillLearningStats;
  items: SkillLearningItem[];
  actions: SkillLearningAction[];
  latest_run?: LearningRun;
  candidate_proposal?: GrowthProposal;
}

export interface SkillLearningRunResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  run: LearningRun;
  summary: {
    stats: SkillLearningStats;
    items: SkillLearningItem[];
    actions: SkillLearningAction[];
  };
}

export interface GrowthProposalChange {
  kind: string;
  heading?: string;
  content?: string;
  field?: string;
  value?: string;
  path?: string;
}

export interface GrowthProposalCreator {
  kind: string;
  model_provider_id?: string;
  model?: string;
  prompt_version: string;
}

export interface GrowthProposal {
  version: string;
  id: string;
  type: string;
  status: string;
  target_path: string;
  risk: string;
  reason: string;
  suggested_changes: GrowthProposalChange[];
  source_paths: string[];
  source_run_id?: string;
  created_by: GrowthProposalCreator;
  created_at: string;
  updated_at?: string;
  applied_at?: string;
  dismissed_at?: string;
  metadata?: Record<string, any>;
}

export interface GrowthProposalsResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  proposals: GrowthProposal[];
}

export interface GrowthProposalResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  proposal: GrowthProposal;
}

export interface SkillLearningNote {
  path: string;
  title: string;
  source: string;
  content: string;
  updated_at: string;
}

export interface SkillLearningNotesResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  notes: SkillLearningNote[];
}

export type ModelProviderType =
  | "openai-compatible"
  | "openai"
  | "ollama"
  | "anthropic"
  | "gemini";

export interface ModelProviderModels {
  summary?: string;
  proposal?: string;
  json?: string;
}

export interface ModelProvider {
  id: string;
  type: ModelProviderType | string;
  name: string;
  base_url?: string;
  api_key_ref?: string;
  models: ModelProviderModels;
  enabled: boolean;
  last_verified_at?: string;
  last_error?: string;
  metadata?: Record<string, any>;
}

export interface ModelProviderSaveRequest {
  id: string;
  type: ModelProviderType | string;
  name: string;
  base_url?: string;
  api_key?: string;
  api_key_ref?: string;
  models: ModelProviderModels;
  enabled: boolean;
}

export interface SaveModelProvidersRequest {
  default_summary_provider_id?: string;
  default_proposal_provider_id?: string;
  providers: ModelProviderSaveRequest[];
}

export interface ModelProviderSupportedType {
  type: ModelProviderType | string;
  name: string;
  requires_api_key: boolean;
  default_base_url?: string;
  live_test_supported: boolean;
}

export interface ModelProvidersResponse {
  version: string;
  storage_path: string;
  updated_at?: string;
  default_summary_provider_id?: string;
  default_proposal_provider_id?: string;
  providers: ModelProvider[];
  supported_types: ModelProviderSupportedType[];
}

export interface ModelProviderTestRequest {
  provider_id?: string;
  provider?: ModelProviderSaveRequest;
}

export interface ModelProviderTestResult {
  ok: boolean;
  provider_id?: string;
  type?: string;
  message: string;
  tested_at: string;
}

export type TeamRole = "owner" | "admin" | "member" | "viewer";

export interface Team {
  id: string;
  slug: string;
  name: string;
  description?: string;
  role?: TeamRole;
  can_manage_members: boolean;
  can_write: boolean;
  storage_used_bytes?: number;
  storage_quota_bytes?: number | null;
  created_at?: string;
  updated_at?: string;
}

export interface TeamMember {
  team_id: string;
  user_id: string;
  user_slug: string;
  display_name: string;
  email?: string;
  role: TeamRole;
  created_at?: string;
  updated_at?: string;
}

export interface SkillAgentTarget {
  id: string;
  name: string;
  platform: string;
  install_path_hint?: string;
  supports_apply: boolean;
  support_status?: string;
  apply_mode?: string;
  export_supported?: boolean;
  auto_apply_reason?: string;
  docs_path?: string;
  directory_rules?: string[];
}

export interface SkillAgentAssignment {
  agent_id: string;
  skill_paths: string[];
}

export interface SkillAssignmentsState {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  storage_path: string;
  updated_at?: string;
  agents: SkillAgentTarget[];
  assignments: SkillAgentAssignment[];
}

export interface LocalSkillSyncSummary {
  add: number;
  update: number;
  unchanged: number;
  missing: number;
  conflict: number;
  removable: number;
  export: number;
  written: number;
  deleted: number;
  sync_risk: number;
  blocked?: number;
  manual_required?: number;
}

export interface LocalSkillSyncChange {
  action: "add" | "update" | "unchanged" | "missing" | "conflict" | "delete" | "marker" | "export";
  skill_path?: string;
  rel_path?: string;
  target_path?: string;
  reason?: string;
  size_bytes?: number;
  verification_status?: string;
  verification_required?: boolean;
  verification_message?: string;
}

export interface LocalSkillDetectedRoot {
  path: string;
  role?: string;
  exists: boolean;
  writable: boolean;
  is_dir: boolean;
  message?: string;
}

export interface LocalSkillSyncAgentPlan {
  agent_id: string;
  name: string;
  target_root?: string;
  supported: boolean;
  support_status: string;
  apply_mode?: string;
  export_supported: boolean;
  export_available: boolean;
  export_file_name?: string;
  auto_apply_reason?: string;
  docs_path?: string;
  directory_rules?: string[];
  detected_roots?: LocalSkillDetectedRoot[];
  message?: string;
  assigned_skill_paths: string[];
  summary: LocalSkillSyncSummary;
  changes: LocalSkillSyncChange[];
  errors?: string[];
}

export interface LocalSkillSyncResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  mode: "preview" | "apply" | "cleanup";
  applied: boolean;
  cleanup: boolean;
  updated_at: string;
  agents: LocalSkillSyncAgentPlan[];
  blocked?: boolean;
}

export interface SkillConversionRequest {
  source_path: string;
  source_platform?: "claude-code" | "codex";
  target_platform: "claude-code" | "codex";
  target_path?: string;
  overwrite?: boolean;
  team_id?: string;
}

export interface SkillConversionResponse {
  version: string;
  scope?: "personal" | "team";
  team?: Team;
  applied: boolean;
  converted_at: string;
  source_path: string;
  target_path: string;
  source_platform: "claude-code" | "codex";
  target_platform: "claude-code" | "codex";
  summary: SkillConversionSummary;
  files: SkillConversionFileChange[];
  auto_items?: SkillConversionReportItem[];
  manual_items?: SkillConversionReportItem[];
  unsupported?: SkillConversionReportItem[];
  warnings?: SkillConversionReportItem[];
}

export interface SkillCopyToPersonalResponse {
  version: string;
  applied: boolean;
  copied_at: string;
  team?: Team;
  source_path: string;
  target_path: string;
  files: number;
  bytes: number;
  overwrite: boolean;
}

export interface SkillDiffFileItem {
  rel_path: string;
  status: "added" | "modified" | "deleted" | "unchanged";
  source_content?: string;
  target_content?: string;
}

export interface SkillDiffResponse {
  version: string;
  files: SkillDiffFileItem[];
}

export interface SkillRollbackResponse {
  version: string;
  success: boolean;
  restored: number;
}

export interface TeamMcpAsset {
  version: string;
  slug: string;
  name: string;
  description?: string;
  transport: "stdio" | "http";
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  status: string;
  visibility: string;
  review_status?: string;
  review_note?: string;
  review_requested_at?: string;
  review_requested_by?: string;
  review_requested_by_role?: string;
  reviewed_at?: string;
  reviewed_by?: string;
  reviewed_by_role?: string;
  source_team_id?: string;
  source_team_slug?: string;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  archived_at?: string;
  path?: string;
  tags?: string[];
}

export interface TeamMcpsResponse {
  version: string;
  team?: Team;
  mcps: TeamMcpAsset[];
}

export interface TeamMcpSaveRequest {
  slug: string;
  name: string;
  description?: string;
  transport: "stdio" | "http";
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  status?: string;
  visibility?: string;
  tags?: string[];
}

export interface TeamSkillPublication {
  skill_path: string;
  status: "draft" | "published" | "archived" | string;
  visibility: "private" | "team" | string;
  note?: string;
  review_status?: "requested" | "approved" | "changes_requested" | string;
  review_note?: string;
  review_requested_at?: string;
  review_requested_by?: string;
  review_requested_by_role?: string;
  reviewed_at?: string;
  reviewed_by?: string;
  reviewed_by_role?: string;
  updated_at?: string;
  published_at?: string;
  archived_at?: string;
  implicit?: boolean;
}

export interface TeamSkillPublicationsResponse {
  version: string;
  team?: Team;
  storage_path: string;
  updated_at?: string;
  publications: TeamSkillPublication[];
}

export interface TeamSkillSubscriptionStatus {
  team_id: string;
  team_slug?: string;
  team_name?: string;
  source_path: string;
  target_path: string;
  source_fingerprint?: string;
  source_current_fingerprint?: string;
  files: number;
  bytes: number;
  installed_at?: string;
  updated_at?: string;
  checked_at?: string;
  update_available: boolean;
  source_missing: boolean;
  error?: string;
}

export interface TeamSkillSubscriptionsResponse {
  version: string;
  storage_path: string;
  updated_at?: string;
  checked_at?: string;
  subscriptions: TeamSkillSubscriptionStatus[];
}

export interface TeamSkillReviewEvent {
  id: string;
  asset_type: "skill" | "agent" | string;
  skill_path?: string;
  agent_slug?: string;
  action: string;
  status?: string;
  visibility?: string;
  note?: string;
  actor_id?: string;
  actor_role?: string;
  created_at: string;
}

export interface TeamSkillReviewHistoryResponse {
  version: string;
  team?: Team;
  storage_path: string;
  updated_at?: string;
  events: TeamSkillReviewEvent[];
}

export interface TeamSkillSubscriptionMemberReport {
  user_id: string;
  user_slug?: string;
  display_name?: string;
  role?: string;
  status: "not_installed" | "installed" | "update_available" | "source_missing" | string;
  target_path?: string;
  source_fingerprint?: string;
  source_current_fingerprint?: string;
  files?: number;
  bytes?: number;
  installed_at?: string;
  updated_at?: string;
  checked_at?: string;
  update_available: boolean;
  source_missing: boolean;
  error?: string;
}

export interface TeamSkillSubscriptionSkillReport {
  skill_path: string;
  status: string;
  visibility: string;
  source_fingerprint?: string;
  source_missing: boolean;
  installed_count: number;
  update_available_count: number;
  source_missing_count: number;
  not_installed_count: number;
  members: TeamSkillSubscriptionMemberReport[];
}

export interface TeamSkillSubscriptionReportResponse {
  version: string;
  team?: Team;
  generated_at: string;
  skills: TeamSkillSubscriptionSkillReport[];
}

export interface TeamSkillUpdateNotification {
  id: string;
  kind: string;
  team_id?: string;
  team_slug?: string;
  skill_path?: string;
  user_id?: string;
  user_slug?: string;
  display_name?: string;
  member_role?: string;
  status: string;
  message: string;
  source_fingerprint?: string;
  installed_fingerprint?: string;
  created_at: string;
}

export interface TeamSkillUpdateNotificationsResponse {
  version: string;
  team?: Team;
  storage_path: string;
  updated_at?: string;
  last_checked_at?: string;
  notifications: TeamSkillUpdateNotification[];
  report?: TeamSkillSubscriptionReportResponse;
}

export interface TeamAgentAsset {
  version: string;
  slug: string;
  name: string;
  description?: string;
  instructions?: string;
  status: "draft" | "published" | "archived" | string;
  visibility: "private" | "team" | string;
  default_skill_paths?: string[];
  target_agents?: string[];
  model?: string;
  permissions?: string[];
  approval_required?: string[];
  maintainer?: string;
  review_status?: "requested" | "approved" | "changes_requested" | string;
  review_note?: string;
  review_requested_at?: string;
  review_requested_by?: string;
  review_requested_by_role?: string;
  reviewed_at?: string;
  reviewed_by?: string;
  reviewed_by_role?: string;
  source_team_id?: string;
  source_team_slug?: string;
  installed_from_team_id?: string;
  created_at?: string;
  updated_at?: string;
  published_at?: string;
  archived_at?: string;
  path?: string;
  readme_path?: string;
}

export interface TeamAgentsResponse {
  version: string;
  team?: Team;
  agents: TeamAgentAsset[];
}

export interface TeamAgentSaveRequest {
  slug: string;
  name: string;
  description?: string;
  instructions?: string;
  status?: string;
  visibility?: string;
  default_skill_paths?: string[];
  target_agents?: string[];
  model?: string;
  permissions?: string[];
  approval_required?: string[];
  maintainer?: string;
}

export interface SkillConversionSummary {
  converted: number;
  copied: number;
  generated: number;
  conflicts: number;
  auto: number;
  manual: number;
  warnings: number;
}

export interface SkillConversionFileChange {
  action: "convert" | "copy" | "generate" | "conflict";
  source_path?: string;
  target_path: string;
  rel_path: string;
  reason?: string;
  size_bytes?: number;
}

export interface SkillConversionReportItem {
  code: string;
  severity: string;
  path?: string;
  message: string;
}

export interface BundleContext {
  kind: "skill" | "project" | "conversation";
  name: string;
  path: string;
  source?: string;
  read_only?: boolean;
  description?: string;
  when_to_use?: string;
  status?: string;
  primary_path?: string;
  log_path?: string;
  capabilities?: string[];
  allowed_tools?: string[];
  tags?: string[];
  arguments?: Record<string, any>;
  activation?: Record<string, any>;
  min_trust_level?: number;
  relative_path?: string;
}

export interface LocalGitSyncInfo {
  enabled: boolean;
  path?: string;
  execution_mode?: "local" | "hosted";
  sync_state?: "idle" | "queued" | "running" | "error";
  sync_requested_at?: string;
  sync_started_at?: string;
  sync_next_attempt_at?: string;
  sync_attempt_count?: number;
  synced: boolean;
  last_synced_at?: string;
  message?: string;
  last_error?: string;
  auto_commit_enabled?: boolean;
  auto_push_enabled?: boolean;
  auth_mode?: string;
  remote_name?: string;
  remote_branch?: string;
  last_commit_at?: string;
  last_commit_hash?: string;
  last_push_at?: string;
  last_push_error?: string;
  remote_conflict?: boolean;
  force_remote_overwrite?: boolean;
  commit_created?: boolean;
  push_attempted?: boolean;
  push_succeeded?: boolean;
}

export interface RequestEnvelope<T> {
  data: T;
  localGitSync?: LocalGitSyncInfo;
}

export type OpsStatusLevel = "ok" | "warning" | "critical";

export interface OpsCheck {
  id: string;
  status: OpsStatusLevel;
  message: string;
  action?: string;
}

export interface OpsGitMirrorStatus {
  service_configured: boolean;
  execution_mode?: "local" | "hosted";
  hosted_root?: string;
  hosted_root_set: boolean;
  remote_url?: string;
  auto_push_enabled: boolean;
  sync_state?: string;
  last_synced_at?: string;
  last_push_at?: string;
  last_error?: string;
  last_push_error?: string;
  remote_conflict?: boolean;
  github_app_connected: boolean;
  github_token_set: boolean;
}

export interface OpsBackupTargetState {
  id: string;
  kind: BackupTargetKind;
  name: string;
  enabled: boolean;
  secret_configured: boolean;
  auto_backup_enabled?: boolean;
  auto_backup_interval_hours?: number;
  retention_keep_last?: number;
  retention_keep_days?: number;
  last_auto_backup_at?: string;
  last_backup_at?: string;
  last_backup_object?: string;
  last_backup_error?: string;
}

export interface OpsBackupRunState {
  id: string;
  target_id: string;
  target_name: string;
  target_kind: BackupTargetKind;
  trigger: "manual" | "auto";
  status: "success" | "failed";
  object_name?: string;
  size_bytes: number;
  started_at: string;
  completed_at?: string;
  duration_ms: number;
  error?: string;
  remote_deleted_at?: string;
}

export interface OpsBackupStatus {
  service_configured: boolean;
  targets_configured: number;
  enabled_targets: number;
  targets_with_secrets: number;
  targets_with_last_backup: number;
  history_count: number;
  last_successful_backup_at?: string;
  last_backup_object?: string;
  last_run_status?: string;
  last_run_at?: string;
  last_error?: string;
  targets: OpsBackupTargetState[];
  recent_runs: OpsBackupRunState[];
}

export interface OpsStatus {
  status: OpsStatusLevel;
  generated_at: string;
  storage?: string;
  local_mode: boolean;
  public_url?: string;
  git_mirror: OpsGitMirrorStatus;
  backup: OpsBackupStatus;
  checks: OpsCheck[];
  docs: Array<{ title: string; path: string }>;
}

export interface OpsInstanceUserState {
  user_id: string;
  user_slug: string;
  display_name?: string;
  status: OpsStatusLevel;
  git_mirror: OpsGitMirrorStatus;
  backup: OpsBackupStatus;
  checks: OpsCheck[];
  has_git_backup: boolean;
  has_external_backup: boolean;
  has_remote_artifact: boolean;
}

export interface OpsInstanceStatus {
  status: OpsStatusLevel;
  generated_at: string;
  storage?: string;
  local_mode: boolean;
  public_url?: string;
  users_total: number;
  users_with_git_backup: number;
  users_with_external_backup: number;
  users_with_remote_backup_artifact: number;
  users_with_critical_backup_status: number;
  latest_git_push_at?: string;
  latest_external_backup_at?: string;
  latest_external_backup_object?: string;
  subjects: OpsInstanceUserState[];
  checks: OpsCheck[];
  docs: Array<{ title: string; path: string }>;
}

export interface BillingRedirectDetail {
  code?: string
  message: string
  plan?: string
  upgrade_url?: string
  used_bytes?: number
  limit_bytes?: number
  retry_after_sec?: number
}

type APIErrorPayload = {
  code?: string
  message?: string
  error?: string
  plan?: string
  upgrade_url?: string
  used_bytes?: number
  delta_bytes?: number
  limit_bytes?: number
  retry_after_sec?: number
}

export const BILLING_REDIRECT_EVENT = 'vola:billing-redirect'

export class BillingAPIError extends Error {
  code?: string
  plan?: string
  upgrade_url?: string
  used_bytes?: number
  delta_bytes?: number
  limit_bytes?: number
  retry_after_sec?: number

  constructor(payload: APIErrorPayload, fallbackMessage: string) {
    super(payload.message || payload.error || fallbackMessage)
    this.name = 'BillingAPIError'
    this.code = payload.code
    this.plan = payload.plan
    this.upgrade_url = payload.upgrade_url
    this.used_bytes = payload.used_bytes
    this.delta_bytes = payload.delta_bytes
    this.limit_bytes = payload.limit_bytes
    this.retry_after_sec = payload.retry_after_sec
  }
}

function extractAPIErrorPayload(raw: any): APIErrorPayload {
  if (!raw || typeof raw !== 'object') {
    return {}
  }
  if (raw.error && typeof raw.error === 'object') {
    return raw.error as APIErrorPayload
  }
  if (typeof raw.error === 'string') {
    return { ...raw, message: raw.message || raw.error }
  }
  return raw as APIErrorPayload
}

function buildAPIError(payload: any, fallbackMessage: string): BillingAPIError {
  return new BillingAPIError(extractAPIErrorPayload(payload), fallbackMessage)
}

export function buildAPIErrorFromPayload(payload: any, fallbackMessage: string): BillingAPIError {
  return buildAPIError(payload, fallbackMessage)
}

export async function buildAPIErrorFromResponse(res: Response): Promise<BillingAPIError> {
  const contentType = res.headers.get('content-type') || ''
  const payload = contentType.toLowerCase().includes('application/json')
    ? await res.json().catch(() => ({ error: res.statusText }))
    : { error: await res.text().then((text) => text.trim().slice(0, 160)).catch(() => res.statusText) || res.statusText }
  return buildAPIError(payload, res.statusText)
}

async function parseAPIJsonResponse(res: Response, path: string) {
  const contentType = res.headers.get('content-type') || ''
  if (!contentType.toLowerCase().includes('application/json')) {
    const preview = await res.text()
      .then((text) => text.replace(/\s+/g, ' ').trim().slice(0, 120))
      .catch(() => '')
    throw new Error(preview ? `接口返回的不是 JSON：${path} (${preview})` : `接口返回的不是 JSON：${path}`)
  }
  return res.json()
}

export function notifyBillingRedirect(error: unknown) {
  if (!(error instanceof BillingAPIError)) return
  if (error.code !== 'quota_exceeded' && error.code !== 'account_read_only') return
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent<BillingRedirectDetail>(BILLING_REDIRECT_EVENT, {
    detail: {
      code: error.code,
      message: error.message,
      plan: error.plan,
      upgrade_url: error.upgrade_url,
      used_bytes: error.used_bytes,
      limit_bytes: error.limit_bytes,
      retry_after_sec: error.retry_after_sec,
    },
  }))
}

// ---------------------------------------------------------------------------
// Token refresh logic
// ---------------------------------------------------------------------------

let isRefreshing = false;
let refreshPromise: Promise<AuthResponse | null> | null = null;

async function doRefreshToken(): Promise<AuthResponse | null> {
  const refreshToken = readHeaderSafeAuthValue("refresh_token");
  if (!refreshToken) return null;

  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) {
      clearStoredAuth();
      return null;
    }
    const data: AuthResponse = await res.json();
    storeAccessToken(data.access_token);
    localStorage.setItem("refresh_token", data.refresh_token);
    return data;
  } catch {
    clearStoredAuth();
    return null;
  }
}

// ---------------------------------------------------------------------------
// Core request function with automatic 401 refresh
// ---------------------------------------------------------------------------

async function requestWithMetadata<T>(
  path: string,
  options?: RequestInit,
): Promise<RequestEnvelope<T>> {
  await ensureTauriApiBase();
  let token = readHeaderSafeAuthValue("token");
  if (!token && shouldAutoBootstrapLocalOwner(path)) {
    token = await bootstrapLocalOwnerAccessToken();
  }
  const hasExplicitContentType =
    !!options?.headers &&
    typeof options.headers === "object" &&
    !Array.isArray(options.headers) &&
    "Content-Type" in options.headers;
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      ...(!hasExplicitContentType && !(options?.body instanceof FormData)
        ? { "Content-Type": "application/json" }
        : {}),
      ...authHeadersForToken(token),
      ...options?.headers,
    },
  });

  if (res.status === 401 && shouldAutoBootstrapLocalOwner(path)) {
    clearStoredAuth();
    const localToken = await bootstrapLocalOwnerAccessToken();
    if (localToken) {
      const retryRes = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: {
          ...(!hasExplicitContentType && !(options?.body instanceof FormData)
            ? { "Content-Type": "application/json" }
            : {}),
          ...authHeadersForToken(localToken),
          ...options?.headers,
        },
      });
      if (!retryRes.ok) {
        const err = await buildAPIErrorFromResponse(retryRes)
        notifyBillingRedirect(err)
        throw err
      }
      const retryJson = await parseAPIJsonResponse(retryRes, path);
      if (retryJson && retryJson.ok === true && retryJson.data !== undefined) {
        return {
          data: retryJson.data,
          localGitSync: retryJson.local_git_sync,
        };
      }
      return { data: retryJson };
    }
  }

  // If 401, try to refresh the token once
  if (res.status === 401 && readHeaderSafeAuthValue("refresh_token")) {
    if (!isRefreshing) {
      isRefreshing = true;
      refreshPromise = doRefreshToken().finally(() => {
        isRefreshing = false;
        refreshPromise = null;
      });
    }

    const refreshResult = await (refreshPromise || doRefreshToken());
    if (refreshResult) {
      // Retry the original request with the new token
      const retryRes = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers: {
          ...(!hasExplicitContentType && !(options?.body instanceof FormData)
            ? { "Content-Type": "application/json" }
            : {}),
          Authorization: `Bearer ${refreshResult.access_token}`,
          ...options?.headers,
        },
      });
      if (!retryRes.ok) {
        const err = await buildAPIErrorFromResponse(retryRes)
        notifyBillingRedirect(err)
        throw err
      }
      const retryJson = await parseAPIJsonResponse(retryRes, path);
      if (retryJson && retryJson.ok === true && retryJson.data !== undefined) {
        return {
          data: retryJson.data,
          localGitSync: retryJson.local_git_sync,
        };
      }
      return { data: retryJson };
    }
    throw new Error("session expired");
  }

  if (!res.ok) {
    const err = await buildAPIErrorFromResponse(res)
    notifyBillingRedirect(err)
    throw err
  }
  const json = await parseAPIJsonResponse(res, path);
  // Unwrap APISuccess envelope: {ok: true, data: {...}} → return data
  if (json && json.ok === true && json.data !== undefined) {
    return {
      data: json.data,
      localGitSync: json.local_git_sync,
    };
  }
  return { data: json };
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const result = await requestWithMetadata<T>(path, options);
  return result.data;
}

async function requestEnvelope<T>(
  path: string,
  options?: RequestInit,
): Promise<RequestEnvelope<T>> {
  return requestWithMetadata<T>(path, options);
}

async function agentRequest<T>(
  path: string,
  token: string,
  options?: RequestInit,
): Promise<T> {
  const hasExplicitContentType =
    !!options?.headers &&
    typeof options.headers === "object" &&
    !Array.isArray(options.headers) &&
    "Content-Type" in options.headers;
  const res = await fetch(path, {
    ...options,
    headers: {
      Authorization: `Bearer ${token}`,
      ...(!hasExplicitContentType && !(options?.body instanceof FormData)
        ? { "Content-Type": "application/json" }
        : {}),
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const err = await buildAPIErrorFromResponse(res)
    notifyBillingRedirect(err)
    throw err
  }
  if (res.status === 204) {
    return undefined as T;
  }
  const json = await parseAPIJsonResponse(res, path);
  if (json && json.ok === true && json.data !== undefined) {
    return json.data;
  }
  return json;
}

async function agentRequestBytes(path: string, token: string): Promise<Blob> {
  const res = await fetch(path, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  if (!res.ok) {
    const err = await buildAPIErrorFromResponse(res)
    notifyBillingRedirect(err)
    throw err
  }
  return res.blob();
}

export const api = {
  // Auth
  getAuthProviders: (): Promise<AuthProvider[]> =>
    request<AuthProvider[]>("/auth/providers"),

  register: (data: RegisterRequest): Promise<AuthResponse> =>
    request<AuthResponse>("/auth/register", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  login: (data: LoginRequest): Promise<AuthResponse> =>
    request<AuthResponse>("/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  startAuthProvider: (
    provider: string,
    redirectUrl?: string,
    action: AuthProviderAction = "login",
  ): Promise<StartAuthProviderResponse> =>
    request<StartAuthProviderResponse>(
      `/auth/providers/${encodeURIComponent(provider)}/start`,
      {
        method: "POST",
        body: JSON.stringify({ redirect_url: redirectUrl || "", action }),
      },
    ),

  refreshToken: (refreshToken: string): Promise<AuthResponse> =>
    request<AuthResponse>("/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  logout: async (): Promise<void> => {
    const refreshToken = readHeaderSafeAuthValue("refresh_token");
    if (refreshToken) {
      try {
        await request<any>("/auth/logout", {
          method: "POST",
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
      } catch {
        // Ignore errors on logout
      }
    }
    localStorage.removeItem("token");
    localStorage.removeItem("refresh_token");
  },

  getPublicConfig: (): Promise<PublicConfig> =>
    request<PublicConfig>("/config"),

  bootstrapLocalOwnerToken: (): Promise<CreateTokenResponse> =>
    request<CreateTokenResponse>("/local/owner-token", {
      method: "POST",
    }).then((created) => {
      storeAccessToken(created?.token);
      return created;
    }),

  getLocalMcpHealth: (): Promise<Record<string, { status: "online" | "offline"; latency_ms: number; last_check: string }>> =>
    request<Record<string, { status: "online" | "offline"; latency_ms: number; last_check: string }>>("/local/mcp/health"),

  getSessions: (): Promise<Session[]> => request<Session[]>("/auth/sessions"),

  revokeSession: (id: string): Promise<void> =>
    request<void>(`/auth/sessions/${id}`, { method: "DELETE" }),

  devLogin: (slug: string) =>
    request<{ token: string; user: any }>("/auth/token/dev", {
      method: "POST",
      body: JSON.stringify({ slug }),
    }),

  getMe: () => request<any>("/auth/me"),

  updateMe: (data: {
    display_name: string;
    bio: string;
    timezone: string;
    language: string;
  }) =>
    request<any>("/auth/me", {
      method: "PUT",
      body: JSON.stringify(data),
    }),

  // Dashboard
  getStats: () =>
    request<DashboardStats>("/dashboard/stats").then(normalizeDashboardStats),

  getDashboardActivities: () =>
    request<any[]>("/dashboard/activities"),

  // Billing
  getBillingStatus: () => request<BillingStatus>('/billing/status'),

  createBillingCheckout: (planCode?: PaidBillingPlanCode) =>
    request<{ ok: true; checkout_url: string }>('/billing/checkout', {
      method: 'POST',
      ...(planCode ? { body: JSON.stringify({ plan_code: planCode }) } : {}),
    }),

  createBillingPortal: () =>
    request<{ ok: true; portal_url: string }>('/billing/portal', {
      method: 'POST',
    }),

  // Local MCP Clients
  getLocalMCPClients: (): Promise<Array<{
    id: string;
    name: string;
    installed: boolean;
    registered: boolean;
    config_path?: string;
  }>> => request("/local/mcp/clients"),

  registerLocalMCPClient: (clientId: string): Promise<{ success: boolean }> =>
    request("/local/mcp/clients/register", {
      method: "POST",
      body: JSON.stringify({ client_id: clientId }),
    }),

  unregisterLocalMCPClient: (clientId: string): Promise<{ success: boolean }> =>
    request("/local/mcp/clients/unregister", {
      method: "POST",
      body: JSON.stringify({ client_id: clientId }),
    }),

  // Connections
  getConnections: () =>
    request<{ connections: ConnectionResponse[] }>("/connections").then(
      (r) => r.connections || [],
    ),
  createConnection: (data: any) =>
    request<any>("/connections", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateConnection: (id: string, data: any) =>
    request<any>(`/connections/${id}`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  deleteConnection: (id: string) =>
    request<void>(`/connections/${id}`, { method: "DELETE" }),

  getOAuthGrants: () =>
    request<{ grants: OAuthGrantResponse[] }>("/oauth/grants").then(
      (r) => r.grants || [],
    ),
  revokeOAuthGrant: (id: string) =>
    request<void>(`/oauth/grants/${id}`, { method: "DELETE" }),

  // Memory
  getProfile: () => request<any>("/memory/profile"),
  upsertProfile: (data: any) =>
    request<any>("/memory/profile", {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  getScratchMemory: () => request<any>("/memory/scratch"),
  writeScratchMemory: (data: any) =>
    request<any>("/memory/scratch", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  // Projects
  getProjects: () =>
    request<{ projects: any[] }>("/projects").then((r) => r.projects),
  getProject: (name: string) => request<any>(`/projects/${name}`),
  createProject: (name: string) =>
    request<any>("/projects", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  archiveProject: (name: string) =>
    request<any>(`/projects/${name}/archive`, { method: "PUT" }),
  appendProjectLog: (
    name: string,
    data: { source: string; action: string; summary: string; tags?: string[] },
  ) =>
    request<any>(`/projects/${name}/log`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  scanLocalLibrary: (data?: LocalLibraryScanRequest) =>
    request<LocalLibraryScanResponse>("/local/library/scan", {
      method: "POST",
      body: JSON.stringify(data || {}),
    }),
  importLocalLibrary: (data?: LocalLibraryScanRequest) =>
    request<LocalLibraryImportResponse>("/local/library/import", {
      method: "POST",
      body: JSON.stringify(data || {}),
    }),

  // Skills
  getSkills: (teamID?: string) =>
    request<{ skills: SkillSummary[] }>(
      teamID ? `/skills?team_id=${encodeURIComponent(teamID)}` : "/skills",
    ).then((r) => r.skills || []),
  getSkillLearningSummary: (teamID?: string) =>
    request<SkillLearningSummary>(
      teamID ? `/skills/learning-summary?team_id=${encodeURIComponent(teamID)}` : "/skills/learning-summary",
    ),
  getSkillLearningNotes: (teamID?: string, days = 14) =>
    request<SkillLearningNotesResponse>(
      teamID
        ? `/skills/learning-notes?team_id=${encodeURIComponent(teamID)}&days=${encodeURIComponent(String(days))}`
        : `/skills/learning-notes?days=${encodeURIComponent(String(days))}`,
    ),
  recommendSkills: (query: string, teamID?: string) => {
    const params = new URLSearchParams()
    params.set("q", query)
    if (teamID) params.set("team_id", teamID)
    return request<SkillLearningSummary>(`/skills/learning-recommend?${params.toString()}`)
  },
  createSkillLearningRun: (teamID?: string) =>
    request<SkillLearningRunResponse>(
      teamID ? `/skills/learning-runs?team_id=${encodeURIComponent(teamID)}` : "/skills/learning-runs",
      { method: "POST" },
    ),
  getGrowthProposals: (teamID?: string, status?: string) => {
    const params = new URLSearchParams()
    if (teamID) params.set("team_id", teamID)
    if (status) params.set("status", status)
    const suffix = params.toString()
    return request<GrowthProposalsResponse>(`/growth-proposals${suffix ? `?${suffix}` : ""}`)
  },
  acceptGrowthProposal: (id: string, teamID?: string) =>
    request<GrowthProposalResponse>(
      `/growth-proposals/${encodeURIComponent(id)}/accept${teamID ? `?team_id=${encodeURIComponent(teamID)}` : ""}`,
      { method: "POST" },
    ),
  dismissGrowthProposal: (id: string, teamID?: string) =>
    request<GrowthProposalResponse>(
      `/growth-proposals/${encodeURIComponent(id)}/dismiss${teamID ? `?team_id=${encodeURIComponent(teamID)}` : ""}`,
      { method: "POST" },
    ),
  applyGrowthProposal: (id: string, teamID?: string) =>
    request<GrowthProposalResponse>(
      `/growth-proposals/${encodeURIComponent(id)}/apply${teamID ? `?team_id=${encodeURIComponent(teamID)}` : ""}`,
      { method: "POST" },
    ),
  getModelProviders: () =>
    request<ModelProvidersResponse>("/model-providers"),
  saveModelProviders: (data: SaveModelProvidersRequest) =>
    requestEnvelope<ModelProvidersResponse>("/model-providers", {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  testModelProvider: (data: ModelProviderTestRequest) =>
    request<ModelProviderTestResult>("/model-providers/test", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  getTeams: () =>
    request<{ teams: Team[] }>("/teams").then((r) => r.teams || []),
  createTeam: (params: {
    slug: string;
    name: string;
    description?: string;
    storageQuotaBytes?: number | null;
  }) =>
    request<{ team: Team }>("/teams", {
      method: "POST",
      body: JSON.stringify({
        slug: params.slug,
        name: params.name,
        description: params.description || "",
        storage_quota_bytes: params.storageQuotaBytes ?? null,
      }),
    }).then((r) => r.team),
  getTeamMembers: (teamID: string) =>
    request<{ members: TeamMember[] }>(`/teams/${encodeURIComponent(teamID)}/members`).then((r) => r.members || []),
  addTeamMember: (teamID: string, userSlug: string, role: TeamRole) =>
    request<{ member: TeamMember }>(`/teams/${encodeURIComponent(teamID)}/members`, {
      method: "POST",
      body: JSON.stringify({ user_slug: userSlug, role }),
    }).then((r) => r.member),
  updateTeamMember: (teamID: string, userID: string, role: TeamRole) =>
    request<{ member: TeamMember }>(`/teams/${encodeURIComponent(teamID)}/members/${encodeURIComponent(userID)}`, {
      method: "PUT",
      body: JSON.stringify({ role }),
    }).then((r) => r.member),
  removeTeamMember: (teamID: string, userID: string) =>
    request<{ status: string }>(`/teams/${encodeURIComponent(teamID)}/members/${encodeURIComponent(userID)}`, {
      method: "DELETE",
    }),
  getTeamSkills: (teamID: string) =>
    request<{ skills: SkillSummary[] }>(`/teams/${encodeURIComponent(teamID)}/skills`).then((r) => r.skills || []),
  getTeamSkillPublications: (teamID: string) =>
    request<TeamSkillPublicationsResponse>(`/teams/${encodeURIComponent(teamID)}/skill-publications`),
  saveTeamSkillPublication: (teamID: string, data: {
    skill_path: string;
    status: string;
    visibility: string;
    note?: string;
  }) =>
    request<TeamSkillPublicationsResponse>(`/teams/${encodeURIComponent(teamID)}/skill-publications`, {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  getTeamSkillReviewHistory: (teamID: string, params?: { asset_type?: string; skill_path?: string; agent_slug?: string }) => {
    const query = new URLSearchParams()
    if (params?.asset_type) query.set("asset_type", params.asset_type)
    if (params?.skill_path) query.set("skill_path", params.skill_path)
    if (params?.agent_slug) query.set("agent_slug", params.agent_slug)
    const suffix = query.toString()
    return request<TeamSkillReviewHistoryResponse>(`/teams/${encodeURIComponent(teamID)}/skill-review-history${suffix ? `?${suffix}` : ""}`)
  },
  requestTeamSkillReview: (teamID: string, data: { asset_type?: string; skill_path?: string; agent_slug?: string; note?: string }) =>
    request<TeamSkillReviewHistoryResponse>(`/teams/${encodeURIComponent(teamID)}/skill-review-requests`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  resolveTeamSkillReview: (teamID: string, data: { asset_type?: string; skill_path?: string; agent_slug?: string; decision: "approved" | "changes_requested"; note?: string }) =>
    request<TeamSkillReviewHistoryResponse>(`/teams/${encodeURIComponent(teamID)}/skill-review-requests/resolve`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  getTeamSkillSubscriptionReport: (teamID: string) =>
    request<TeamSkillSubscriptionReportResponse>(`/teams/${encodeURIComponent(teamID)}/skill-subscription-report`),
  checkTeamSkillSubscriptions: (teamID: string) =>
    request<TeamSkillUpdateNotificationsResponse>(`/teams/${encodeURIComponent(teamID)}/skill-subscriptions/check`, {
      method: "POST",
    }),
  getTeamSkillUpdateNotifications: (teamID: string) =>
    request<TeamSkillUpdateNotificationsResponse>(`/teams/${encodeURIComponent(teamID)}/skill-update-notifications`),
  getTeamSkillSubscriptions: (teamID?: string) =>
    request<TeamSkillSubscriptionsResponse>(
      teamID ? `/skills/team-subscriptions?team_id=${encodeURIComponent(teamID)}` : "/skills/team-subscriptions",
    ),
  getTeamAgents: (teamID: string) =>
    request<TeamAgentsResponse>(`/teams/${encodeURIComponent(teamID)}/agents`),
  saveTeamAgent: (teamID: string, data: TeamAgentSaveRequest) =>
    request<TeamAgentsResponse>(`/teams/${encodeURIComponent(teamID)}/agents`, {
      method: "POST",
      body: JSON.stringify(data),
    }),
  installTeamAgent: (teamID: string, slug: string) =>
    requestEnvelope<{ agent: TeamAgentAsset }>(`/teams/${encodeURIComponent(teamID)}/agents/${encodeURIComponent(slug)}/install`, {
      method: "POST",
    }),
  getSkillAssignments: (teamID?: string) =>
    request<SkillAssignmentsState>(
      teamID ? `/skills/assignments?team_id=${encodeURIComponent(teamID)}` : "/skills/assignments",
    ),
  saveSkillAssignments: (assignments: SkillAgentAssignment[], teamID?: string) =>
    requestEnvelope<SkillAssignmentsState>(teamID ? `/skills/assignments?team_id=${encodeURIComponent(teamID)}` : "/skills/assignments", {
      method: "PUT",
      body: JSON.stringify({ assignments, team_id: teamID || undefined }),
    }),
  previewLocalSkillSync: (teamID?: string) =>
    request<LocalSkillSyncResponse>("/local/skills/sync/preview", {
      method: "POST",
      body: JSON.stringify({ team_id: teamID || undefined }),
    }),
  applyLocalSkillSync: (teamID?: string, ackQualityReview = false) =>
    request<LocalSkillSyncResponse>("/local/skills/sync/apply", {
      method: "POST",
      body: JSON.stringify({ team_id: teamID || undefined, ack_quality_review: ackQualityReview }),
    }),
  cleanupLocalSkillSync: (teamID?: string) =>
    request<LocalSkillSyncResponse>("/local/skills/sync/cleanup", {
      method: "POST",
      body: JSON.stringify({ team_id: teamID || undefined }),
    }),
  downloadLocalSkillSyncExport: async (agentId: string, teamID?: string) => {
    const res = await fetch(`${API_BASE}/local/skills/sync/export`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...authHeaders(),
      },
      body: JSON.stringify({ agent_id: agentId, team_id: teamID || undefined }),
    });
    if (!res.ok) {
      const err = await buildAPIErrorFromResponse(res)
      notifyBillingRedirect(err)
      throw err
    }
    const blob = await res.blob();
    const filename = parseDownloadFilename(
      res.headers.get("Content-Disposition"),
      `vola-skills-${agentId}.zip`,
    );
    triggerBrowserDownload(blob, filename);
  },
  previewSkillConversion: (req: SkillConversionRequest) =>
    request<SkillConversionResponse>("/skills/convert/preview", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  applySkillConversion: (req: SkillConversionRequest) =>
    requestEnvelope<SkillConversionResponse>("/skills/convert/apply", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  copyTeamSkillToPersonal: (teamID: string, sourcePath: string, targetPath?: string, overwrite = false) =>
    requestEnvelope<SkillCopyToPersonalResponse>("/skills/copy-to-personal", {
      method: "POST",
      body: JSON.stringify({
        team_id: teamID,
        source_path: sourcePath,
        target_path: targetPath || undefined,
        overwrite,
      }),
    }),
  diffTeamSkillSubscription: (teamId: string, sourcePath: string, targetPath: string): Promise<SkillDiffResponse> =>
    request<SkillDiffResponse>("/skills/team-subscriptions/diff", {
      method: "POST",
      body: JSON.stringify({ team_id: teamId, source_path: sourcePath, target_path: targetPath }),
    }),
  rollbackTeamSkillSubscription: (targetPath: string, backupFilePath: string): Promise<RequestEnvelope<SkillRollbackResponse>> =>
    requestEnvelope<SkillRollbackResponse>("/skills/team-subscriptions/rollback", {
      method: "POST",
      body: JSON.stringify({ target_path: targetPath, backup_file_path: backupFilePath }),
    }),
  fetchTeamMcps: (teamId: string): Promise<TeamMcpsResponse> =>
    request<TeamMcpsResponse>(`/teams/${teamId}/mcps`),
  saveTeamMcp: (teamId: string, mcp: TeamMcpSaveRequest): Promise<RequestEnvelope<TeamMcpsResponse>> =>
    requestEnvelope<TeamMcpsResponse>(`/teams/${teamId}/mcps`, {
      method: "POST",
      body: JSON.stringify(mcp),
    }),
  submitTeamMcpReview: (teamId: string, mcpSlug: string, note?: string): Promise<RequestEnvelope<TeamMcpsResponse>> =>
    requestEnvelope<TeamMcpsResponse>(`/teams/${teamId}/mcps/${mcpSlug}/review`, {
      method: "POST",
      body: JSON.stringify({ note }),
    }),
  resolveTeamMcpReview: (teamId: string, mcpSlug: string, decision: "approve" | "reject", note?: string): Promise<RequestEnvelope<TeamMcpsResponse>> =>
    requestEnvelope<TeamMcpsResponse>(`/teams/${teamId}/mcps/${mcpSlug}/resolve`, {
      method: "POST",
      body: JSON.stringify({ decision, note }),
    }),

  // File tree
  getTree: (path = "/"): Promise<FileNode> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    return request<FileNode>(`/tree${normalized}`);
  },
  downloadTreeZip: async (path: string) => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    const fallbackName = `${normalized.split("/").filter(Boolean).slice(-1)[0] || "root"}.zip`;
    const res = await fetch(
      `${API_BASE}/tree/archive?path=${encodeURIComponent(normalized)}`,
      {
        headers: {
          ...authHeaders(),
        },
      },
    );
    if (!res.ok) {
      const err = await buildAPIErrorFromResponse(res)
      notifyBillingRedirect(err)
      throw err
    }
    const blob = await res.blob();
    const filename = parseDownloadFilename(
      res.headers.get("Content-Disposition"),
      fallbackName,
    );
    triggerBrowserDownload(blob, filename);
  },
  downloadTeamTreeZip: async (teamID: string, path: string) => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    const fallbackName = `${normalized.split("/").filter(Boolean).slice(-1)[0] || "root"}.zip`;
    const res = await fetch(
      `${API_BASE}/teams/${encodeURIComponent(teamID)}/tree/archive?path=${encodeURIComponent(normalized)}`,
      {
        headers: {
          ...authHeaders(),
        },
      },
    );
    if (!res.ok) {
      const err = await buildAPIErrorFromResponse(res)
      notifyBillingRedirect(err)
      throw err
    }
    const blob = await res.blob();
    const filename = parseDownloadFilename(
      res.headers.get("Content-Disposition"),
      fallbackName,
    );
    triggerBrowserDownload(blob, filename);
  },
  getTreeSnapshot: (path = "/"): Promise<TreeSnapshotResponse> =>
    request<TreeSnapshotResponse>(
      `/tree/snapshot?path=${encodeURIComponent(path)}`,
    ),
  getTeamTree: (teamID: string, path = "/"): Promise<FileNode> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    return request<FileNode>(`/teams/${encodeURIComponent(teamID)}/tree${normalized}`);
  },
  getTeamTreeSnapshot: (teamID: string, path = "/"): Promise<TreeSnapshotResponse> =>
    request<TreeSnapshotResponse>(
      `/teams/${encodeURIComponent(teamID)}/tree/snapshot?path=${encodeURIComponent(path)}`,
    ),

  search: (
    q: string,
  ): Promise<
    {
      path: string;
      name: string;
      source?: string;
      kind?: string;
      content?: string;
      updated_at?: string;
    }[]
  > =>
    request<{ query: string; results: any[] }>(
      `/search?q=${encodeURIComponent(q)}`,
    ).then((r) =>
      (r.results || []).map((x: any) => ({
        path: x.path,
        name: x.name || x.path.split("/").pop() || x.path,
        source: x.source,
        kind: x.kind,
        content: x.content,
        updated_at: x.updated_at,
      })),
    ),

  writeTree: (
    path: string,
    params: {
      content: string;
      mimeType?: string;
      isDir?: boolean;
      metadata?: Record<string, any>;
      expectedVersion?: number;
      expectedChecksum?: string;
      minTrustLevel?: number;
    },
  ): Promise<FileNode> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    const body = {
      content: params.content,
      mime_type: params.mimeType || "text/plain",
      is_dir: params.isDir || false,
      metadata: params.metadata,
      expected_version: params.expectedVersion,
      expected_checksum: params.expectedChecksum,
      min_trust_level: params.minTrustLevel,
    };
    return request<FileNode>(`/tree${normalized}` as string, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  },
  writeTeamTree: (
    teamID: string,
    path: string,
    params: {
      content: string;
      mimeType?: string;
      isDir?: boolean;
      metadata?: Record<string, any>;
      expectedVersion?: number;
      expectedChecksum?: string;
      minTrustLevel?: number;
    },
  ): Promise<FileNode> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    const body = {
      content: params.content,
      mime_type: params.mimeType || "text/plain",
      is_dir: params.isDir || false,
      metadata: params.metadata,
      expected_version: params.expectedVersion,
      expected_checksum: params.expectedChecksum,
      min_trust_level: params.minTrustLevel,
    };
    return request<FileNode>(`/teams/${encodeURIComponent(teamID)}/tree${normalized}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  },
  deleteTree: (path: string): Promise<{ status: string; path: string }> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    return request<{ status: string; path: string }>(`/tree${normalized}`, {
      method: "DELETE",
    });
  },
  deleteTeamTree: (teamID: string, path: string): Promise<{ status: string; path: string }> => {
    const normalized = path.startsWith("/") ? path : `/${path}`;
    return request<{ status: string; path: string }>(`/teams/${encodeURIComponent(teamID)}/tree${normalized}`, {
      method: "DELETE",
    });
  },

  // Memory conflicts
  getConflicts: () =>
    request<{ conflicts: MemoryConflict[] }>("/memory/conflicts").then(
      (r) => r.conflicts,
    ),
  resolveConflict: (id: string, resolution: string) =>
    request<{ status: string; resolution: string }>(
      `/memory/conflicts/${id}/resolve`,
      {
        method: "POST",
        body: JSON.stringify({ resolution }),
      },
    ),

  // Vault
  getVaultScopes: () =>
    request<{ scopes: any[] }>("/vault/scopes").then((r) => r.scopes || []),

  requestEnvelope,

  // Import / Export
  importSkills: (skills: SkillFile[], teamID?: string) =>
    request<ImportResult>(teamID ? `/import/skills?team_id=${encodeURIComponent(teamID)}` : "/import/skills", {
      method: "POST",
      body: JSON.stringify({ skills, team_id: teamID || undefined }),
    }),
  importClaudeMemory: (memories: ClaudeMemoryItem[]) =>
    request<ImportResult>("/import/claude-memory", {
      method: "POST",
      body: JSON.stringify({ memories }),
    }),
  importClaudeData: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<ClaudeDataImportResult>('/import/claude-data', {
      method: 'POST',
      body: form,
    })
  },
  importProfile: (profile: ImportProfileRequest) =>
    request<ImportResult>("/import/profile", {
      method: "POST",
      body: JSON.stringify(profile),
    }),
  importVault: (secrets: VaultSecretImport[]) =>
    request<ImportResult>("/import/vault", {
      method: "POST",
      body: JSON.stringify({ secrets }),
    }),
  importFull: (data: FullHubExport) =>
    request<ImportResult>("/import/full", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  exportFull: () => request<FullHubExport>("/export/full"),
  exportZip: async () => {
    const res = await fetch(`${API_BASE}/export/zip`, {
      headers: {
        ...authHeaders(),
      },
    });
    if (!res.ok) {
      const err = await buildAPIErrorFromResponse(res)
      notifyBillingRedirect(err)
      throw err
    }
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `vola-export-${new Date().toISOString().slice(0, 10)}.zip`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  },
  exportJSON: () => request<any>("/export/json"),

  // Token management
  getTokens: (): Promise<ScopedTokenResponse[]> =>
    request<{ tokens: ScopedTokenResponse[] }>("/tokens").then((r) => r.tokens),

  createToken: (req: CreateTokenRequest): Promise<CreateTokenResponse> =>
    request<CreateTokenResponse>("/tokens", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  createSyncToken: (req: SyncTokenRequest): Promise<SyncTokenResponse> =>
    request<SyncTokenResponse>("/tokens/sync", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  updateToken: (
    id: string,
    req: UpdateTokenRequest,
  ): Promise<ScopedTokenResponse> =>
    request<ScopedTokenResponse>(`/tokens/${id}`, {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  revokeToken: (id: string): Promise<void> =>
    request<void>(`/tokens/${id}`, { method: "DELETE" }),

  getTokenScopes: (): Promise<{
    scopes: string[];
    categories: Record<string, string[]>;
    bundles: Record<string, string[]>;
  }> => request("/tokens/scopes"),
  uploadSkillsZip: (file: File, teamID?: string) => {
    const formData = new FormData();
    formData.append("file", file);
    if (teamID) formData.append("team_id", teamID);
    const query = teamID ? `?team_id=${encodeURIComponent(teamID)}` : "";
    return fetch(`${API_BASE}/import/skills${query}`, {
      method: "POST",
      headers: {
        ...authHeaders(),
      },
      body: formData,
    }).then(async (res) => {
      if (!res.ok) {
        const err = await buildAPIErrorFromResponse(res)
        notifyBillingRedirect(err)
        throw err
      }
      const payload = await parseAPIJsonResponse(res, `/import/skills${query}`);
      return (payload && payload.ok === true && payload.data !== undefined ? payload.data : payload) as ImportResult;
    });
  },

  previewBundle: (token: string, payload: any): Promise<BundlePreviewResult> =>
    agentRequest("/agent/import/preview", token, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  importBundle: (token: string, bundle: any): Promise<any> =>
    agentRequest("/agent/import/bundle", token, {
      method: "POST",
      body: JSON.stringify(bundle),
    }),

  startSyncSession: (token: string, payload: any): Promise<SyncSessionStatus> =>
    agentRequest("/agent/import/session", token, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  uploadSyncPart: (
    token: string,
    sessionId: string,
    index: number,
    data: Uint8Array,
  ): Promise<SyncSessionStatus> =>
    agentRequest(`/agent/import/session/${sessionId}/parts/${index}`, token, {
      method: "PUT",
      headers: {
        "Content-Type": "application/octet-stream",
      },
      body: data as BodyInit,
    }),

  getSyncSession: (
    token: string,
    sessionId: string,
  ): Promise<SyncSessionStatus> =>
    agentRequest(`/agent/import/session/${sessionId}`, token),

  commitSyncSession: (
    token: string,
    sessionId: string,
    previewFingerprint?: string,
  ): Promise<any> =>
    agentRequest(`/agent/import/session/${sessionId}/commit`, token, {
      method: "POST",
      body: JSON.stringify(
        previewFingerprint ? { preview_fingerprint: previewFingerprint } : {},
      ),
    }),

  abortSyncSession: (token: string, sessionId: string): Promise<any> =>
    agentRequest(`/agent/import/session/${sessionId}`, token, {
      method: "DELETE",
    }),

  listSyncJobs: (token: string): Promise<SyncJob[]> =>
    agentRequest<{ jobs: SyncJob[] }>("/agent/sync/jobs", token).then(
      (data) => data.jobs || [],
    ),

  getSyncJob: (token: string, jobId: string): Promise<SyncJob> =>
    agentRequest(`/agent/sync/jobs/${jobId}`, token),

  exportBundle: (
    token: string,
    format: "json" | "archive",
    filters?: BundleFilters,
  ): Promise<any> => {
    const params = new URLSearchParams();
    params.set("format", format);
    for (const domain of filters?.include_domains || [])
      params.append("include_domain", domain);
    for (const skill of filters?.include_skills || [])
      params.append("include_skill", skill);
    for (const skill of filters?.exclude_skills || [])
      params.append("exclude_skill", skill);
    const path = `/agent/export/bundle?${params.toString()}`;
    if (format === "archive") return agentRequestBytes(path, token);
    return agentRequest(path, token);
  },

  getLocalConfig: (): Promise<LocalConfigFile> =>
    request<LocalConfigFile>("/local/config"),

  updateLocalConfig: (
    req: UpdateLocalConfigRequest,
  ): Promise<LocalConfigFile> =>
    request<LocalConfigFile>("/local/config", {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  updateLocalActiveWorkspace: (req: { active_team_id: string }): Promise<void> =>
    request<void>("/local/workspace/active", {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  getLocalGitMirror: (): Promise<GitMirrorSettings> =>
    request<GitMirrorSettings>("/local/git-mirror"),

  updateLocalGitMirror: (
    req: UpdateGitMirrorRequest,
  ): Promise<GitMirrorSettings> =>
    request<GitMirrorSettings>("/local/git-mirror", {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  getLocalPlatformImportPreviewTask: (req: {
    platform: string;
    mode: "agent" | "files" | "all";
  }): Promise<LocalPlatformImportPreviewTask> =>
    request<LocalPlatformImportPreviewTask>(
      `/local/platform/preview-task?platform=${encodeURIComponent(req.platform)}&mode=${encodeURIComponent(req.mode)}`,
    ),

  startLocalPlatformImportPreviewTask: (req: {
    platform: string;
    mode: "agent" | "files" | "all";
  }): Promise<LocalPlatformImportPreviewTask> =>
    request<LocalPlatformImportPreviewTask>("/local/platform/preview-task", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getLocalPlatformImportPreviewCache: (req: {
    platform: string;
    mode: "agent" | "files" | "all";
  }): Promise<LocalPlatformImportPreview | null> =>
    request<LocalPlatformImportPreview | null>(
      `/local/platform/preview-cache?platform=${encodeURIComponent(req.platform)}&mode=${encodeURIComponent(req.mode)}`,
    ),

  previewLocalPlatformImport: (req: {
    platform: string;
    mode: "agent" | "files" | "all";
  }): Promise<LocalPlatformImportPreview> =>
    request<LocalPlatformImportPreview>("/local/platform/preview", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  importLocalPlatform: (req: {
    platform: string;
    mode: "agent" | "files" | "all";
  }): Promise<RequestEnvelope<LocalPlatformImportSummary>> =>
    requestEnvelope<LocalPlatformImportSummary>("/local/platform/import", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getCodexConsole: (): Promise<CodexConsoleResponse> =>
    request<CodexConsoleResponse>("/local/codex-console"),
  saveCodexConsoleArtifacts: (data: CodexConsoleArtifactRegistrySaveRequest = {}): Promise<CodexConsoleArtifactRegistrySaveResponse> =>
    request<CodexConsoleArtifactRegistrySaveResponse>("/local/codex-console/artifacts/save", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  syncCodexConsoleMemory: (data: CodexConsoleMemorySyncRequest): Promise<CodexConsoleMemorySyncResponse> =>
    request<CodexConsoleMemorySyncResponse>("/local/codex-console/memory-sync", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  reviewCodexConsoleMemory: (data: CodexConsoleMemoryReviewRequest): Promise<CodexConsoleMemoryReviewResponse> =>
    request<CodexConsoleMemoryReviewResponse>("/local/codex-console/memory-review", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  resolveCodexConsoleMemoryConflict: (data: CodexConsoleMemoryConflictResolveRequest): Promise<CodexConsoleMemoryConflictResolveResponse> =>
    request<CodexConsoleMemoryConflictResolveResponse>("/local/codex-console/memory-conflict/resolve", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  saveCodexConsoleHandover: (data: CodexConsoleHandoverSaveRequest): Promise<CodexConsoleHandoverSaveResponse> =>
    request<CodexConsoleHandoverSaveResponse>("/local/codex-console/handovers/save", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  saveCodexConsoleSkillCandidate: (data: CodexConsoleSkillCandidateSaveRequest): Promise<CodexConsoleSkillCandidateSaveResponse> =>
    request<CodexConsoleSkillCandidateSaveResponse>("/local/codex-console/skill-candidates/save", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  assignCodexConsoleSkillCandidate: (data: CodexConsoleSkillCandidateAssignPreviewRequest): Promise<CodexConsoleSkillCandidateAssignPreviewResponse> =>
    request<CodexConsoleSkillCandidateAssignPreviewResponse>("/local/codex-console/skill-candidates/assign-preview", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  updateCodexConsoleSkillCandidateStatus: (data: CodexConsoleSkillCandidateStatusRequest): Promise<CodexConsoleSkillCandidateStatusResponse> =>
    request<CodexConsoleSkillCandidateStatusResponse>("/local/codex-console/skill-candidates/status", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  testGitMirrorGitHubToken: (
    req: GitMirrorGitHubTestRequest,
  ): Promise<GitMirrorGitHubTestResult> =>
    request<GitMirrorGitHubTestResult>("/local/git-mirror/github/test", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getGitMirror: (): Promise<GitMirrorSettings> =>
    request<GitMirrorSettings>("/git-mirror"),

  updateGitMirror: (req: UpdateGitMirrorRequest): Promise<GitMirrorSettings> =>
    request<GitMirrorSettings>("/git-mirror", {
      method: "PUT",
      body: JSON.stringify(req),
    }),

  syncGitMirror: (req: SyncGitMirrorRequest = {}): Promise<LocalGitSyncInfo> =>
    request<LocalGitSyncInfo>("/git-mirror/sync", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  getOpsStatus: (): Promise<OpsStatus> =>
    request<OpsStatus>("/ops/status"),

  getOpsInstanceStatus: (): Promise<OpsInstanceStatus> =>
    request<OpsInstanceStatus>("/ops/instance-status"),

  testGitMirrorGitHubTokenGeneric: (
    req: GitMirrorGitHubTestRequest,
  ): Promise<GitMirrorGitHubTestResult> =>
    request<GitMirrorGitHubTestResult>("/git-mirror/github/test", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  startGitMirrorGitHubAppBrowser: (
    returnTo: string,
  ): Promise<GitMirrorGitHubAppBrowserStartResult> =>
    request<GitMirrorGitHubAppBrowserStartResult>(
      "/git-mirror/github-app/browser/start",
      {
        method: "POST",
        body: JSON.stringify({ return_to: returnTo }),
      },
    ),

  disconnectGitMirrorGitHubAppUser: (): Promise<{ status: string }> =>
    request<{ status: string }>("/git-mirror/github-app/disconnect", {
      method: "POST",
      body: JSON.stringify({}),
    }),

  listGitMirrorGitHubAppRepos: (): Promise<GitMirrorRepo[]> =>
    request<GitMirrorRepo[]>("/git-mirror/github-app/repos"),

  createGitMirrorGitHubAppRepo: (
    req: CreateGitMirrorRepoRequest,
  ): Promise<GitMirrorRepo> =>
    request<GitMirrorRepo>("/git-mirror/github-app/repos", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  createDefaultGitMirrorGitHubAppBackupRepo:
    (): Promise<GitMirrorDefaultBackupRepoResult> =>
      request<GitMirrorDefaultBackupRepoResult>(
        "/git-mirror/github-app/default-backup-repo",
        {
          method: "POST",
          body: JSON.stringify({}),
        },
      ),

  listBackupTargets: (): Promise<BackupTarget[]> =>
    request<BackupTarget[]>("/backup/targets"),

  saveBackupTarget: (req: SaveBackupTargetRequest): Promise<BackupTarget> =>
    request<BackupTarget>("/backup/targets", {
      method: "POST",
      body: JSON.stringify(req),
    }),

  runBackupTarget: (id: string): Promise<BackupRunResult> =>
    request<BackupRunResult>(`/backup/targets/${encodeURIComponent(id)}/run`, {
      method: "POST",
      body: JSON.stringify({}),
    }),

  listBackupRuns: (limit = 20): Promise<BackupRun[]> =>
    request<BackupRun[]>(`/backup/runs?limit=${encodeURIComponent(String(limit))}`),

  previewBackupRestore: (file: File): Promise<BackupRestorePreview> => {
    const form = new FormData();
    form.append("file", file);
    return request<BackupRestorePreview>("/backup/restore/preview", {
      method: "POST",
      body: form,
    });
  },

  applyBackupRestore: (
    file: File,
    mode: "skip" | "overwrite",
  ): Promise<BackupRestoreApplyResult> => {
    const form = new FormData();
    form.append("file", file);
    form.append("mode", mode);
    return request<BackupRestoreApplyResult>("/backup/restore/apply", {
      method: "POST",
      body: form,
    });
  },
};

// ---------------------------------------------------------------------------
// Import / Export types
// ---------------------------------------------------------------------------

export interface SkillFile {
  path: string;
  content: string;
  content_type?: string;
}

export interface ClaudeMemoryItem {
  content: string;
  source: string;
  created_at?: string;
}

export interface ImportProfileRequest {
  preferences?: string;
  relationships?: string;
  principles?: string;
}

export interface VaultSecretImport {
  scope: string;
  value: string;
  description: string;
  min_trust_level?: number;
}

export interface ProjectExport {
  name: string;
  status: string;
  context_md: string;
}

export interface FullHubExport {
  version: string;
  exported_at: string;
  user: any;
  profile: Record<string, string>;
  skills: SkillFile[];
  projects: ProjectExport[];
  vault_scopes: string[];
}

export interface ImportResult {
  imported: number;
  skipped: number;
  manifest_files?: number;
  errors?: string[];
  skills?: string[];
  skill_manifests?: SkillManifest[];
  warnings?: SkillManifestWarning[];
}

export interface ExternalSkillAssetUploadResult {
  skill_name: string;
  path: string;
  manifest: SkillManifest;
  warnings?: SkillManifestWarning[];
}

export interface SkillManifest {
  version: string;
  skill_name: string;
  entry_file: string;
  source_platform?: string;
  source_archive?: string;
  files: SkillManifestFile[];
  external_references?: SkillExternalReference[];
  env_vars?: string[];
  supported_platforms?: string[];
  warnings?: SkillManifestWarning[];
  summary: SkillManifestSummary;
}

export interface SkillManifestFile {
  path: string;
  kind: string;
  size_bytes: number;
  content_type?: string;
  sha256?: string;
  included: boolean;
}

export interface SkillExternalReference {
  path: string;
  source_file: string;
  scope: string;
  included: boolean;
  status: string;
}

export interface SkillManifestWarning {
  code: string;
  severity: string;
  path?: string;
  message: string;
}

export interface SkillManifestSummary {
  files: number;
  scripts: number;
  dependency_files: number;
  resources: number;
  binary_files: number;
  large_files: number;
  secret_risk_files: number;
  external_references: number;
}

export interface LocalPlatformPreviewCategory {
  name: string;
  discovered: number;
  importable: number;
  archived: number;
  blocked: number;
}

export interface LocalPlatformSensitiveFinding {
  title: string;
  detail: string;
  severity: string;
  source_paths?: string[];
  redacted_example?: string;
}

export interface LocalPlatformVaultCandidate {
  scope: string;
  description: string;
  source_paths?: string[];
}

export interface LocalPlatformImportPreview {
  platform: string;
  display_name: string;
  mode: "agent" | "files" | "all";
  started_at?: string;
  completed_at?: string;
  duration_ms?: number;
  categories: LocalPlatformPreviewCategory[];
  sensitive_findings: LocalPlatformSensitiveFinding[];
  vault_candidates: LocalPlatformVaultCandidate[];
  notes: string[];
  next_command: string;
}

export interface LocalPlatformImportPreviewTaskStatus {
  version: string;
  job_id?: string;
  platform: string;
  display_name?: string;
  mode: "agent" | "files" | "all";
  state: "idle" | "running" | "succeeded" | "failed";
  started_at?: string;
  updated_at?: string;
  completed_at?: string;
  duration_ms?: number;
  error_message?: string;
  result_saved_at?: string;
}

export interface LocalPlatformImportPreviewTask {
  status?: LocalPlatformImportPreviewTaskStatus | null;
  preview?: LocalPlatformImportPreview | null;
}

export interface LocalPlatformFilesImportResult {
  platform: string;
  files: number;
  bytes: number;
  paths: string[];
}

export interface LocalPlatformAgentImportResult {
  platform: string;
  profile_categories: number;
  memory_items: number;
  projects: number;
  project_files: number;
  bundles: number;
  conversations: number;
  artifacts: number;
  imported: number;
  archived: number;
  blocked: number;
  sensitive_findings: number;
  vault_candidates: number;
  paths: string[];
}

export interface LocalPlatformImportSummary {
  platform: string;
  mode: "agent" | "files" | "all";
  files?: LocalPlatformFilesImportResult;
  agent?: LocalPlatformAgentImportResult;
}

export interface CodexConsoleWorkspace {
  name: string;
  threads: number;
  last_activity?: string;
}

export interface CodexConsoleOverview {
  threads: number;
  goals: number;
  automations: number;
  runs: number;
  artifacts: number;
  hooks: number;
  memory_candidates: number;
  handovers: number;
  skill_candidates: number;
  memory_review_required: number;
  memory_accepted: number;
  memory_ignored: number;
  memory_deferred: number;
  memory_synced: number;
  projects: number;
  skills: number;
  tools: number;
  sensitive_findings: number;
  vault_candidates: number;
  last_activity?: string;
  workspaces?: CodexConsoleWorkspace[];
}

export interface CodexConsoleThread {
  id: string;
  title: string;
  summary?: string;
  project?: string;
  started_at?: string;
  updated_at?: string;
  archived: boolean;
  source_path?: string;
  message_count: number;
  user_turns: number;
  assistant_turns: number;
  tool_calls: number;
  tool_results: number;
  thinking_events: number;
  attachment_count: number;
  artifact_count: number;
}

export interface CodexConsoleGoal {
  id: string;
  title: string;
  status: string;
  thread_id?: string;
  thread_title?: string;
  project?: string;
  source_path?: string;
  observed_at?: string;
  description?: string;
}

export interface CodexConsoleAutomation {
  id: string;
  name: string;
  kind?: string;
  status?: string;
  schedule?: string;
  prompt?: string;
  source_path?: string;
}

export interface CodexConsoleRunEvent {
  at?: string;
  type: string;
  title: string;
  detail?: string;
}

export interface CodexConsoleRun {
  id: string;
  thread_id: string;
  thread_title: string;
  project?: string;
  started_at?: string;
  updated_at?: string;
  source_path?: string;
  tool_calls: number;
  tool_results: number;
  browser_actions: number;
  computer_actions: number;
  approvals: number;
  errors: number;
  artifacts: number;
  events?: CodexConsoleRunEvent[];
}

export interface CodexConsoleArtifact {
  id: string;
  name: string;
  kind: string;
  role?: string;
  thread_id?: string;
  thread_title?: string;
  project?: string;
  source_path?: string;
  detail?: string;
  handoff_note?: string;
  agent_instruction?: string;
}

export interface CodexConsoleArtifactRegistrySummary {
  status?: string;
  path?: string;
  saved_at?: string;
  artifact_count?: number;
  project_count?: number;
  project_summaries?: CodexConsoleArtifactProjectSummary[];
}

export interface CodexConsoleArtifactProjectSummary {
  project: string;
  artifact_count: number;
  roles?: CodexConsoleArtifactRoleCount[];
  primary_artifacts?: CodexConsoleArtifactHandoff[];
}

export interface CodexConsoleArtifactRoleCount {
  role: string;
  count: number;
}

export interface CodexConsoleArtifactHandoff {
  id: string;
  name: string;
  role?: string;
  handoff_note?: string;
  agent_instruction?: string;
}

export interface CodexConsoleArtifactRegistrySaveRequest {
  overwrite?: boolean;
}

export interface CodexConsoleArtifactRegistrySaveResponse {
  status: string;
  path: string;
  saved_at?: string;
  artifact_count: number;
  project_count: number;
  message?: string;
}

export interface CodexConsoleHookRisk {
  id: string;
  name: string;
  kind: string;
  bundle?: string;
  status: string;
  risk_level?: string;
  shebang?: string;
  env_vars?: string[];
  risk_signals?: string[];
  write_path_hints?: string[];
  source_path?: string;
  detail?: string;
}

export interface CodexConsoleMemoryCandidate {
  id: string;
  title: string;
  kind: string;
  content: string;
  source_path?: string;
  confidence?: number;
  review_status: string;
  review_note?: string;
  reviewed_at?: string;
  memory_path?: string;
  conflict?: CodexConsoleMemoryConflictHint;
}

export interface CodexConsoleHandoverItem {
  id: string;
  title: string;
  kind?: string;
  detail?: string;
  at?: string;
  source_path?: string;
}

export interface CodexConsoleHandoverSummary {
  id: string;
  project: string;
  summary: string;
  latest_activity?: string;
  thread_count: number;
  run_count: number;
  artifact_count: number;
  memory_candidate_count: number;
  recent_threads?: CodexConsoleHandoverItem[];
  recent_runs?: CodexConsoleHandoverItem[];
  recent_artifacts?: CodexConsoleHandoverItem[];
  memory_candidates?: CodexConsoleHandoverItem[];
  status?: string;
  path?: string;
  saved_at?: string;
  version?: number;
  saved_content?: string;
}

export interface CodexConsoleHandoverSaveRequest {
  id: string;
  overwrite?: boolean;
  content_override?: string;
}

export interface CodexConsoleHandoverSaveResponse {
  id: string;
  project: string;
  status: string;
  path: string;
  saved_at?: string;
  version?: number;
  edited?: boolean;
  message?: string;
}

export interface CodexConsoleSkillCandidate {
  id: string;
  name: string;
  title: string;
  project?: string;
  thread_id?: string;
  thread_title?: string;
  updated_at?: string;
  source_path?: string;
  confidence?: number;
  tool_calls: number;
  artifact_count: number;
  signals?: string[];
  rationale?: string;
  draft: string;
  status?: string;
  status_note?: string;
  status_updated_at?: string;
  skill_path?: string;
  saved_at?: string;
  edited?: boolean;
  metadata_path?: string;
  manifest_path?: string;
}

export interface CodexConsoleSkillCandidateSaveRequest {
  id: string;
  overwrite?: boolean;
  draft_override?: string;
}

export interface CodexConsoleSkillCandidateSaveResponse {
  id: string;
  name: string;
  status: string;
  skill_path: string;
  path: string;
  metadata_path: string;
  manifest_path: string;
  saved_at?: string;
  edited?: boolean;
  files?: string[];
  message?: string;
}

export interface CodexConsoleSkillCandidateAssignPreviewRequest {
  id: string;
  agent_ids?: string[];
  target_roots?: Record<string, string>;
}

export interface CodexConsoleSkillCandidateStatusRequest {
  id: string;
  status: "draft" | "ready" | "archived";
  note?: string;
}

export interface CodexConsoleSkillCandidateAssignPreviewResponse {
  id: string;
  status: string;
  skill_path: string;
  agent_ids: string[];
  assignments?: SkillAgentAssignment[];
  sync_preview?: LocalSkillSyncResponse;
  message?: string;
}

export interface CodexConsoleSkillCandidateStatusResponse {
  id: string;
  status: string;
  skill_path: string;
  metadata_path: string;
  manifest_path: string;
  status_updated_at?: string;
  message?: string;
}

export interface CodexConsoleMemoryConflictHint {
  status: string;
  target: string;
  category: string;
  path: string;
  existing_source?: string;
  existing_updated_at?: string;
  existing_content?: string;
  candidate_content?: string;
  message: string;
}

export interface CodexConsoleMemorySyncRequest {
  ids?: string[];
  all?: boolean;
  target?: "profile" | "project";
  project?: string;
  content_overrides?: Record<string, string>;
}

export interface CodexConsoleMemorySyncItem {
  id: string;
  title?: string;
  category?: string;
  path?: string;
  target?: string;
  project?: string;
  edited?: boolean;
  status: string;
  message?: string;
}

export interface CodexConsoleMemorySyncResponse {
  target: string;
  project?: string;
  requested: number;
  synced: number;
  skipped: number;
  failed: number;
  items: CodexConsoleMemorySyncItem[];
  paths?: string[];
}

export interface CodexConsoleMemoryReviewRequest {
  ids: string[];
  status: "review_required" | "accepted" | "ignored" | "deferred";
  note?: string;
}

export interface CodexConsoleMemoryReviewItem {
  id: string;
  title?: string;
  status: string;
  message?: string;
}

export interface CodexConsoleMemoryReviewResponse {
  path: string;
  status: string;
  updated: number;
  failed: number;
  items: CodexConsoleMemoryReviewItem[];
}

export type CodexConsoleMemoryConflictResolution = "keep_existing" | "use_candidate" | "keep_both" | "merge";

export interface CodexConsoleMemoryConflictResolveRequest {
  id: string;
  resolution: CodexConsoleMemoryConflictResolution;
  merged_content?: string;
}

export interface CodexConsoleMemoryConflictResolveResponse {
  id: string;
  title?: string;
  category: string;
  resolution: CodexConsoleMemoryConflictResolution;
  status: string;
  path?: string;
  existing_path?: string;
  candidate_path?: string;
  review_path?: string;
  message?: string;
}

export interface CodexConsoleResponse {
  platform: string;
  updated_at: string;
  overview: CodexConsoleOverview;
  threads: CodexConsoleThread[];
  goals: CodexConsoleGoal[];
  automations: CodexConsoleAutomation[];
  runs: CodexConsoleRun[];
  artifacts: CodexConsoleArtifact[];
  artifact_registry: CodexConsoleArtifactRegistrySummary;
  hooks: CodexConsoleHookRisk[];
  memory_candidates: CodexConsoleMemoryCandidate[];
  handovers: CodexConsoleHandoverSummary[];
  skill_candidates: CodexConsoleSkillCandidate[];
  sensitive_findings?: LocalPlatformSensitiveFinding[];
  vault_candidates?: LocalPlatformVaultCandidate[];
  notes?: string[];
}

export interface ClaudeDataImportResult {
  memories_imported: number;
  conversations_imported: number;
  projects_imported: number;
  files_written: number;
}

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

export interface ScopedTokenResponse {
  id: string;
  user_id: string;
  name: string;
  token_prefix: string;
  scopes: string[];
  max_trust_level: number;
  expires_at: string;
  rate_limit: number;
  request_count: number;
  last_used_at?: string;
  last_used_ip?: string;
  created_at: string;
  revoked_at?: string;
  is_expired: boolean;
  is_revoked: boolean;
}

export interface CreateTokenRequest {
  name: string;
  scopes: string[];
  max_trust_level: number;
  expires_in_days: number;
}

export interface UpdateTokenRequest {
  name: string;
}

export interface CreateTokenResponse {
  token: string;
  token_prefix: string;
  scoped_token: ScopedTokenResponse;
}

export interface SyncTokenRequest {
  access: "push" | "pull" | "both";
  ttl_minutes: number;
}

export interface SyncTokenResponse {
  token: string;
  expires_at: string;
  api_base: string;
  scopes: string[];
  usage: string;
}

export interface BundleFilters {
  include_domains?: string[];
  include_skills?: string[];
  exclude_skills?: string[];
}

export interface BundlePreviewResult {
  version: string;
  mode: string;
  fingerprint?: string;
  summary: Record<string, number>;
  profile?: Array<{ path: string; action: string; kind?: string }>;
  memory?: Array<{ path: string; action: string; kind?: string }>;
  skills?: Record<
    string,
    {
      summary: Record<string, number>;
      files?: Array<{ path: string; action: string; kind?: string }>;
    }
  >;
}

export interface SyncSessionStatus {
  session_id: string;
  job_id: string;
  status: string;
  chunk_size_bytes: number;
  total_parts: number;
  expires_at: string;
  mode?: string;
  summary: Record<string, any>;
  received_parts?: number[];
  missing_parts?: number[];
}

export interface SyncJob {
  id: string;
  user_id: string;
  session_id?: string;
  direction: string;
  transport: string;
  status: string;
  source?: string;
  mode?: string;
  filters?: BundleFilters;
  summary?: Record<string, any>;
  error?: string;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface GitMirrorSettings {
  enabled: boolean;
  path?: string;
  execution_mode?: "local" | "hosted";
  sync_state?: "idle" | "queued" | "running" | "error";
  sync_requested_at?: string;
  sync_started_at?: string;
  sync_next_attempt_at?: string;
  sync_attempt_count?: number;
  auto_commit_enabled: boolean;
  auto_push_enabled: boolean;
  auth_mode: "local_credentials" | "github_token" | "github_app_user";
  remote_name: string;
  remote_url?: string;
  remote_branch: string;
  last_synced_at?: string;
  last_error?: string;
  last_commit_at?: string;
  last_commit_hash?: string;
  last_push_at?: string;
  last_push_error?: string;
  remote_conflict?: boolean;
  force_remote_overwrite?: boolean;
  github_token_configured: boolean;
  github_token_verified_at?: string;
  github_token_login?: string;
  github_repo_permission?: string;
  github_app_user_connected: boolean;
  github_app_user_login?: string;
  github_app_user_authorized_at?: string;
  github_app_user_refresh_expires_at?: string;
  message?: string;
}

export interface LocalConfigFile {
  path: string;
  raw: string;
}

export interface UpdateLocalConfigRequest {
  raw: string;
}

export interface UpdateGitMirrorRequest {
  auto_commit_enabled: boolean;
  auto_push_enabled: boolean;
  auth_mode: "local_credentials" | "github_token" | "github_app_user";
  remote_name?: string;
  remote_url?: string;
  remote_branch?: string;
  github_token?: string;
  clear_github_token?: boolean;
}

export interface SyncGitMirrorRequest {
  force_remote_overwrite?: boolean;
}

export interface GitMirrorGitHubAppBrowserStartResult {
  authorization_url: string;
}

export interface GitMirrorDefaultBackupRepoResult {
  settings: GitMirrorSettings;
  repo: GitMirrorRepo;
}

export interface GitMirrorRepo {
  owner_login: string;
  owner_type: string;
  repo_name: string;
  full_name: string;
  default_branch?: string;
  clone_url: string;
  viewer_permission?: string;
}

export interface CreateGitMirrorRepoRequest {
  owner_login: string;
  repo_name: string;
  description?: string;
  private: boolean;
  remote_name?: string;
  remote_branch?: string;
}

export interface GitMirrorGitHubTestRequest {
  remote_url: string;
  github_token: string;
}

export interface GitMirrorGitHubTestResult {
  ok: boolean;
  login?: string;
  repo?: string;
  normalized_remote_url?: string;
  permission?: string;
  message?: string;
}

export type BackupTargetKind = "webdav" | "s3";

export interface BackupTarget {
  id: string;
  kind: BackupTargetKind;
  name: string;
  enabled: boolean;
  webdav_url?: string;
  webdav_username?: string;
  s3_endpoint?: string;
  s3_bucket?: string;
  s3_region?: string;
  s3_prefix?: string;
  s3_access_key_id?: string;
  s3_path_style?: boolean;
  secret_configured: boolean;
  auto_backup_enabled?: boolean;
  auto_backup_interval_hours?: number;
  retention_keep_last?: number;
  retention_keep_days?: number;
  last_auto_backup_at?: string;
  last_backup_at?: string;
  last_backup_object?: string;
  last_backup_error?: string;
  created_at?: string;
  updated_at?: string;
}

export interface SaveBackupTargetRequest {
  id?: string;
  kind: BackupTargetKind;
  name: string;
  enabled: boolean;
  webdav_url?: string;
  webdav_username?: string;
  webdav_password?: string;
  s3_endpoint?: string;
  s3_bucket?: string;
  s3_region?: string;
  s3_prefix?: string;
  s3_access_key_id?: string;
  s3_secret_access_key?: string;
  s3_path_style?: boolean;
  auto_backup_enabled?: boolean;
  auto_backup_interval_hours?: number;
  retention_keep_last?: number;
  retention_keep_days?: number;
}

export interface BackupRunResult {
  target: BackupTarget;
  run: BackupRun;
  object_name: string;
  location: string;
  size_bytes: number;
  completed_at: string;
  message?: string;
}

export interface BackupRun {
  id: string;
  target_id: string;
  target_name: string;
  target_kind: BackupTargetKind;
  trigger: "manual" | "auto";
  status: "success" | "failed";
  object_name?: string;
  location?: string;
  size_bytes: number;
  started_at: string;
  completed_at?: string;
  duration_ms: number;
  error?: string;
  remote_deleted_at?: string;
  remote_delete_error?: string;
}

export interface BackupRestoreCategory {
  id: string;
  label: string;
  files: number;
  bytes: number;
}

export interface BackupRestorePreview {
  recognized: boolean;
  file_name?: string;
  size_bytes: number;
  total_files: number;
  total_bytes: number;
  categories: BackupRestoreCategory[];
  warnings?: string[];
}

export interface BackupRestoreAppliedEntry {
  path: string;
  zip_path: string;
  category: string;
  action: "created" | "overwritten" | "skipped" | "error";
  bytes: number;
  error?: string;
}

export interface BackupRestoreApplyResult {
  recognized: boolean;
  mode: "skip" | "overwrite";
  applied: number;
  skipped: number;
  overwritten: number;
  errors?: string[];
  warnings?: string[];
  entries: BackupRestoreAppliedEntry[];
}

// ---------------------------------------------------------------------------
// Memory conflict types
// ---------------------------------------------------------------------------

export interface MemoryConflict {
  id: string;
  user_id: string;
  category: string;
  source_a: string;
  content_a: string;
  source_b: string;
  content_b: string;
  status: string;
  resolved_at?: string;
  created_at: string;
}
