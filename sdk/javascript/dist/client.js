"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.NeuDriveError = exports.NeuDrive = exports.Vola = exports.VolaError = void 0;
/**
 * VolaError is thrown when the API returns a non-2xx response.
 */
class VolaError extends Error {
    constructor(status, body) {
        const msg = typeof body === 'object' && body !== null && 'error' in body
            ? body.error
            : `HTTP ${status}`;
        super(msg);
        this.status = status;
        this.body = body;
        this.name = 'VolaError';
    }
}
exports.VolaError = VolaError;
exports.NeuDriveError = VolaError;
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
class Vola {
    constructor(config) {
        if (!config.baseURL)
            throw new Error('Vola: baseURL is required');
        if (!config.token)
            throw new Error('Vola: token is required');
        this.baseURL = config.baseURL.replace(/\/+$/, '');
        this.token = config.token;
    }
    // -------------------------------------------------------------------------
    // Internal helpers
    // -------------------------------------------------------------------------
    headers(extra) {
        return {
            Authorization: `Bearer ${this.token}`,
            'Content-Type': 'application/json',
            ...extra,
        };
    }
    async request(method, path, body, initOverride) {
        const url = `${this.baseURL}${path}`;
        const init = {
            method,
            headers: this.headers(),
            ...initOverride,
        };
        if (body !== undefined) {
            init.body = JSON.stringify(body);
        }
        const res = await fetch(url, init);
        if (!res.ok) {
            let errBody;
            try {
                errBody = await res.json();
            }
            catch {
                errBody = await res.text();
            }
            throw new VolaError(res.status, errBody);
        }
        // Some endpoints return 204 No Content
        if (res.status === 204)
            return undefined;
        const data = (await res.json());
        if (data && typeof data === 'object' && 'ok' in data && 'data' in data) {
            return data.data;
        }
        return data;
    }
    get(path) {
        return this.request('GET', path);
    }
    post(path, body) {
        return this.request('POST', path, body);
    }
    put(path, body) {
        return this.request('PUT', path, body);
    }
    async requestBytes(path) {
        const url = `${this.baseURL}${path}`;
        const res = await fetch(url, {
            method: 'GET',
            headers: {
                Authorization: `Bearer ${this.token}`,
            },
        });
        if (!res.ok) {
            let errBody;
            try {
                errBody = await res.json();
            }
            catch {
                errBody = await res.text();
            }
            throw new VolaError(res.status, errBody);
        }
        return new Uint8Array(await res.arrayBuffer());
    }
    // -------------------------------------------------------------------------
    // Profile
    // -------------------------------------------------------------------------
    /**
     * Get user profile entries, optionally filtered by category.
     */
    async getProfile(category) {
        const qs = category ? `?category=${encodeURIComponent(category)}` : '';
        const res = await this.get(`/agent/memory/profile${qs}`);
        return res.profiles ?? [];
    }
    /**
     * Update (upsert) a profile category.
     */
    async updateProfile(category, content) {
        await this.put('/agent/memory/profile', { category, content });
    }
    // -------------------------------------------------------------------------
    // Memory / Search
    // -------------------------------------------------------------------------
    /**
     * Search memory, inbox, or both.
     */
    async searchMemory(query, scope = 'all') {
        const qs = `?q=${encodeURIComponent(query)}&scope=${encodeURIComponent(scope)}`;
        const res = await this.get(`/agent/search${qs}`);
        return res.results ?? [];
    }
    // -------------------------------------------------------------------------
    // Projects
    // -------------------------------------------------------------------------
    /**
     * List all projects for the authenticated user.
     */
    async listProjects() {
        const res = await this.get('/agent/projects');
        return res.projects ?? [];
    }
    /**
     * Get a single project with its logs.
     */
    async getProject(name) {
        return this.get(`/agent/projects/${encodeURIComponent(name)}`);
    }
    /**
     * Append an action log entry to a project.
     */
    async logAction(project, action, summary, tags) {
        await this.post(`/agent/projects/${encodeURIComponent(project)}/log`, {
            action,
            summary,
            tags,
        });
    }
    // -------------------------------------------------------------------------
    // File Tree
    // -------------------------------------------------------------------------
    /**
     * List directory contents at the given path.
     */
    async listDirectory(path) {
        const safePath = this.directoryPath(path);
        const res = await this.get(`/agent/tree${safePath}`);
        return res.children ?? [];
    }
    /**
     * Read a file's content from the file tree.
     */
    async readFile(path) {
        const safePath = this.filePath(path);
        const res = await this.get(`/agent/tree${safePath}`);
        return res.content ?? '';
    }
    /**
     * Write (create or overwrite) a file in the file tree.
     */
    async writeFile(path, content, options) {
        const safePath = this.filePath(path);
        await this.put(`/agent/tree${safePath}`, {
            content,
            content_type: options?.mime_type,
            metadata: options?.metadata,
            min_trust_level: options?.min_trust_level,
            expected_version: options?.expected_version,
            expected_checksum: options?.expected_checksum,
        });
    }
    /**
     * Snapshot a subtree for sync/bootstrap.
     */
    async snapshot(path = '/') {
        const qs = `?path=${encodeURIComponent(path)}`;
        return this.get(`/agent/tree/snapshot${qs}`);
    }
    /**
     * Fetch incremental changes after a cursor.
     */
    async changes(cursor, path = '/') {
        const qs = `?cursor=${encodeURIComponent(String(cursor))}&path=${encodeURIComponent(path)}`;
        return this.get(`/agent/tree/changes${qs}`);
    }
    // -------------------------------------------------------------------------
    // Vault
    // -------------------------------------------------------------------------
    /**
     * List all vault scopes visible to the current trust level.
     */
    async listSecrets() {
        const res = await this.get('/agent/vault/scopes');
        return res.scopes ?? [];
    }
    /**
     * Read a secret from the vault by scope name.
     */
    async readSecret(scope) {
        const res = await this.get(`/agent/vault/${encodeURIComponent(scope)}`);
        return res.data ?? '';
    }
    // -------------------------------------------------------------------------
    // Skills
    // -------------------------------------------------------------------------
    /**
     * List available skills.
     */
    async listSkills() {
        const res = await this.get('/agent/skills');
        return res.skills ?? [];
    }
    /**
     * Read a skill's content by name.
     */
    async readSkill(name) {
        const res = await this.get(`/agent/tree/skills/${encodeURIComponent(name)}/SKILL.md`);
        return res.content ?? '';
    }
    // -------------------------------------------------------------------------
    // Inbox
    // -------------------------------------------------------------------------
    /**
     * Send a message to another agent or role.
     */
    async sendMessage(to, subject, body, opts) {
        await this.post('/agent/inbox/send', {
            to,
            subject,
            body,
            domain: opts?.domain,
            tags: opts?.tags,
        });
    }
    /**
     * Read inbox messages, optionally filtered by role and/or status.
     */
    async readInbox(role = 'default', status) {
        const qs = status ? `?status=${encodeURIComponent(status)}` : '';
        const res = await this.get(`/agent/inbox/${encodeURIComponent(role)}${qs}`);
        return res.messages ?? [];
    }
    // -------------------------------------------------------------------------
    // Import
    // -------------------------------------------------------------------------
    /**
     * Import a skill (one or more files).
     */
    async importSkill(name, files) {
        const res = await this.post('/agent/import/skill', { name, files });
        return res.data;
    }
    /**
     * Import Claude-format memory entries.
     */
    async importClaudeMemory(memories) {
        const res = await this.post('/agent/import/claude-memory', { memories });
        return res.data;
    }
    /**
     * Import profile fields (preferences, relationships, principles).
     */
    async importProfile(profile) {
        const res = await this.post('/agent/import/profile', profile);
        return res.data;
    }
    /**
     * Export all user data.
     */
    async exportAll() {
        return this.get('/agent/export/all');
    }
    // -------------------------------------------------------------------------
    // Dashboard
    // -------------------------------------------------------------------------
    /**
     * Get dashboard statistics.
     */
    async getStats() {
        return this.get('/agent/dashboard/stats');
    }
    // -------------------------------------------------------------------------
    // Bundle Sync
    // -------------------------------------------------------------------------
    async getAuthInfo() {
        return this.get('/agent/auth/whoami');
    }
    async previewBundle(payload) {
        return this.post('/agent/import/preview', payload);
    }
    async importBundle(bundle) {
        return this.post('/agent/import/bundle', bundle);
    }
    async exportBundle(format = 'json', filters) {
        const qs = this.bundleFilterQuery(filters);
        const prefix = qs ? `&${qs}` : '';
        if (format === 'archive') {
            return this.requestBytes(`/agent/export/bundle?format=archive${prefix}`);
        }
        return this.get(`/agent/export/bundle?format=json${prefix}`);
    }
    async startSyncSession(request) {
        return this.post('/agent/import/session', request);
    }
    async uploadPart(sessionId, index, data) {
        return this.request('PUT', `/agent/import/session/${encodeURIComponent(sessionId)}/parts/${index}`, undefined, {
            headers: {
                Authorization: `Bearer ${this.token}`,
                'Content-Type': 'application/octet-stream',
            },
            body: data,
        });
    }
    async getSyncSession(sessionId) {
        return this.get(`/agent/import/session/${encodeURIComponent(sessionId)}`);
    }
    async commitSession(sessionId, previewFingerprint) {
        return this.post(`/agent/import/session/${encodeURIComponent(sessionId)}/commit`, previewFingerprint ? { preview_fingerprint: previewFingerprint } : {});
    }
    async abortSession(sessionId) {
        return this.request('DELETE', `/agent/import/session/${encodeURIComponent(sessionId)}`);
    }
    async resumeSession(sessionId, archive) {
        let state = await this.getSyncSession(sessionId);
        const chunkSize = Math.max(state.chunk_size_bytes || 1, 1);
        for (const index of state.missing_parts ?? []) {
            const start = index * chunkSize;
            const end = Math.min(archive.length, start + chunkSize);
            state = await this.uploadPart(sessionId, index, archive.slice(start, end));
        }
        return state;
    }
    async listSyncJobs() {
        const res = await this.get('/agent/sync/jobs');
        return res.jobs ?? [];
    }
    async getSyncJob(jobId) {
        return this.get(`/agent/sync/jobs/${encodeURIComponent(jobId)}`);
    }
    filePath(path) {
        return path.startsWith('/') ? path : `/${path}`;
    }
    bundleFilterQuery(filters) {
        const params = new URLSearchParams();
        for (const domain of filters?.include_domains ?? [])
            params.append('include_domain', domain);
        for (const skill of filters?.include_skills ?? [])
            params.append('include_skill', skill);
        for (const skill of filters?.exclude_skills ?? [])
            params.append('exclude_skill', skill);
        return params.toString();
    }
    directoryPath(path) {
        const safePath = this.filePath(path);
        if (safePath === '/')
            return safePath;
        return safePath.endsWith('/') ? safePath : `${safePath}/`;
    }
}
exports.Vola = Vola;
exports.NeuDrive = Vola;
//# sourceMappingURL=client.js.map