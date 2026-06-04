import type { VolaConfig, Profile, Project, ProjectLog, VaultScope, InboxMessage, ImportResult, FileTreeEntry, SearchResult, Skill, BundleFilters, BundlePreviewResult, DashboardStats, AgentAuthInfo, SyncJob, SyncSessionStatus, TreeSnapshot, TreeChanges, WriteFileOptions } from './types';
/**
 * VolaError is thrown when the API returns a non-2xx response.
 */
export declare class VolaError extends Error {
    readonly status: number;
    readonly body: unknown;
    constructor(status: number, body: unknown);
}
/**
 * Main client for Vola.
 *
 * Uses the `/agent/*` API surface authenticated via a scoped token
 * (ndt_xxxxx) sent as `Authorization: Bearer <token>`.
 *
 * @example
 * ```ts
 * const hub = new Vola({ baseURL: 'https://vola.ai', token: 'ndt_xxxxx' })
 * const profile = await hub.getProfile('preferences')
 * ```
 */
export declare class Vola {
    private readonly baseURL;
    private readonly token;
    constructor(config: VolaConfig);
    private headers;
    private request;
    private get;
    private post;
    private put;
    private requestBytes;
    /**
     * Get user profile entries, optionally filtered by category.
     */
    getProfile(category?: string): Promise<Profile[]>;
    /**
     * Update (upsert) a profile category.
     */
    updateProfile(category: string, content: string): Promise<void>;
    /**
     * Search memory, inbox, or both.
     */
    searchMemory(query: string, scope?: 'memory' | 'inbox' | 'all'): Promise<SearchResult[]>;
    /**
     * List all projects for the authenticated user.
     */
    listProjects(): Promise<Project[]>;
    /**
     * Get a single project with its logs.
     */
    getProject(name: string): Promise<{
        project: Project;
        logs: ProjectLog[];
    }>;
    /**
     * Append an action log entry to a project.
     */
    logAction(project: string, action: string, summary: string, tags?: string[]): Promise<void>;
    /**
     * List directory contents at the given path.
     */
    listDirectory(path: string): Promise<FileTreeEntry[]>;
    /**
     * Read a file's content from the file tree.
     */
    readFile(path: string): Promise<string>;
    /**
     * Write (create or overwrite) a file in the file tree.
     */
    writeFile(path: string, content: string, options?: WriteFileOptions): Promise<void>;
    /**
     * Snapshot a subtree for sync/bootstrap.
     */
    snapshot(path?: string): Promise<TreeSnapshot>;
    /**
     * Fetch incremental changes after a cursor.
     */
    changes(cursor: number, path?: string): Promise<TreeChanges>;
    /**
     * List all vault scopes visible to the current trust level.
     */
    listSecrets(): Promise<VaultScope[]>;
    /**
     * Read a secret from the vault by scope name.
     */
    readSecret(scope: string): Promise<string>;
    /**
     * List available skills.
     */
    listSkills(): Promise<Skill[]>;
    /**
     * Read a skill's content by name.
     */
    readSkill(name: string): Promise<string>;
    /**
     * Send a message to another agent or role.
     */
    sendMessage(to: string, subject: string, body: string, opts?: {
        domain?: string;
        tags?: string[];
    }): Promise<void>;
    /**
     * Read inbox messages, optionally filtered by role and/or status.
     */
    readInbox(role?: string, status?: string): Promise<InboxMessage[]>;
    /**
     * Import a skill (one or more files).
     */
    importSkill(name: string, files: Record<string, string>): Promise<ImportResult>;
    /**
     * Import Claude-format memory entries.
     */
    importClaudeMemory(memories: Array<{
        content: string;
        type?: string;
        created_at?: string;
    }>): Promise<ImportResult>;
    /**
     * Import profile fields (preferences, relationships, principles).
     */
    importProfile(profile: {
        preferences?: string;
        relationships?: string;
        principles?: string;
    }): Promise<ImportResult>;
    /**
     * Export all user data.
     */
    exportAll(): Promise<unknown>;
    /**
     * Get dashboard statistics.
     */
    getStats(): Promise<DashboardStats>;
    getAuthInfo(): Promise<AgentAuthInfo>;
    previewBundle(payload: Record<string, unknown>): Promise<BundlePreviewResult>;
    importBundle(bundle: Record<string, unknown>): Promise<Record<string, unknown>>;
    exportBundle(format?: 'json' | 'archive', filters?: BundleFilters): Promise<Record<string, unknown> | Uint8Array>;
    startSyncSession(request: Record<string, unknown>): Promise<SyncSessionStatus>;
    uploadPart(sessionId: string, index: number, data: Uint8Array): Promise<SyncSessionStatus>;
    getSyncSession(sessionId: string): Promise<SyncSessionStatus>;
    commitSession(sessionId: string, previewFingerprint?: string): Promise<Record<string, unknown>>;
    abortSession(sessionId: string): Promise<Record<string, unknown>>;
    resumeSession(sessionId: string, archive: Uint8Array): Promise<SyncSessionStatus>;
    listSyncJobs(): Promise<SyncJob[]>;
    getSyncJob(jobId: string): Promise<SyncJob>;
    private filePath;
    private bundleFilterQuery;
    private directoryPath;
}
export { Vola as NeuDrive, VolaError as NeuDriveError };
//# sourceMappingURL=client.d.ts.map