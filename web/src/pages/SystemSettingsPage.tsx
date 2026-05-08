import { useMemo, useState, useEffect } from 'react'
import {
  api,
  type GitMirrorGitHubTestResult,
  type GitMirrorSettings,
  type LocalConfigFile,
  type UpdateGitMirrorRequest,
} from '../api'
import { useI18n } from '../i18n'
import { formatDateTime, localizeGitHubAccessMessage } from './data/DataShared'

type ConfigViewMode = 'settings' | 'raw'

type LocalSettingsDraft = {
  currentTarget: string
  profilesJson: string
  listenAddr: string
  storage: string
  sqlitePath: string
  databaseURL: string
  gitMirrorPath: string
  publicBaseURL: string
  jwtSecret: string
  vaultMasterKey: string
  connectionsJson: string
}

const REMOTE_PROFILES_EXAMPLE = `${JSON.stringify({
  official: {
    api_base: 'https://neudrive.ai',
    token: 'eyJhbGciOi...',
    refresh_token: 'ndr_refresh_xxxxxxxx',
    expires_at: '2026-12-31T23:59:59Z',
    scopes: ['admin', 'offline_access'],
    auth_mode: 'oauth_session',
  },
}, null, 2)}\n`

const LOCAL_CONNECTIONS_EXAMPLE = `${JSON.stringify({
  'codex-local': {
    transport: 'stdio',
    entrypoint_type: 'binary',
    entrypoint_path: '/Users/you/.local/bin/neudrive-mcp',
    managed_paths: ['/skills', '/memory', '/projects'],
    chat_usage: ['codex'],
  },
}, null, 2)}\n`

function EyeIcon({ visible }: { visible: boolean }) {
  if (visible) {
    return (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M3 3l18 18" />
        <path d="M10.6 10.6a3 3 0 0 0 4.24 4.24" />
        <path d="M9.88 5.09A10.94 10.94 0 0 1 12 5c5 0 9.27 3.11 11 7-0.6 1.37-1.54 2.62-2.73 3.65" />
        <path d="M6.61 6.61C4.62 7.9 3.1 9.79 2 12c1.73 3.89 6 7 10 7a10.8 10.8 0 0 0 4.39-.93" />
      </svg>
    )
  }
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2 12s3.64-7 10-7 10 7 10 7-3.64 7-10 7-10-7-10-7Z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}

function isRecord(value: unknown): value is Record<string, any> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

function defaultLocalConfigObject() {
  return {
    version: 3,
    current_target: 'local',
    local: {},
  } as Record<string, any>
}

function stringifyLocalConfig(value: Record<string, any>) {
  return `${JSON.stringify(value, null, 2)}\n`
}

function parseLocalConfigObject(raw: string): { value: Record<string, any>; error: string } {
  if (!raw.trim()) {
    return { value: defaultLocalConfigObject(), error: '' }
  }
  try {
    const parsed = JSON.parse(raw)
    if (!isRecord(parsed)) {
      return { value: defaultLocalConfigObject(), error: 'config.json must contain a top-level JSON object.' }
    }
    return { value: parsed, error: '' }
  } catch (err: any) {
    return { value: defaultLocalConfigObject(), error: err?.message || 'config.json is not valid JSON.' }
  }
}

function readString(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function readPrettyObject(value: unknown) {
  if (!isRecord(value)) {
    return '{}'
  }
  return JSON.stringify(value, null, 2)
}

function normalizeTargetInput(value: string) {
  const trimmed = value.trim()
  if (!trimmed || trimmed === 'local') {
    return { target: trimmed || 'local', profile: '' }
  }
  if (trimmed.startsWith('profile:')) {
    return {
      target: trimmed,
      profile: trimmed.slice('profile:'.length).trim(),
    }
  }
  return {
    target: `profile:${trimmed}`,
    profile: trimmed,
  }
}

function draftFromRaw(raw: string): LocalSettingsDraft {
  const parsed = parseLocalConfigObject(raw)
  const root = parsed.value
  const local = isRecord(root.local) ? root.local : {}
  const currentTarget = normalizeTargetInput(readString(root.current_target) || readString(root.current_profile)).target
  return {
    currentTarget,
    profilesJson: readPrettyObject(root.profiles),
    listenAddr: readString(local.listen_addr),
    storage: readString(local.storage),
    sqlitePath: readString(local.sqlite_path),
    databaseURL: readString(local.database_url),
    gitMirrorPath: readString(local.git_mirror_path),
    publicBaseURL: readString(local.public_base_url),
    jwtSecret: readString(local.jwt_secret),
    vaultMasterKey: readString(local.vault_master_key),
    connectionsJson: readPrettyObject(local.connections),
  }
}

function parseObjectField(raw: string, fieldName: string): { value: Record<string, any>; error: string } {
  const trimmed = raw.trim()
  if (!trimmed) {
    return { value: {}, error: '' }
  }
  try {
    const parsed = JSON.parse(trimmed)
    if (!isRecord(parsed)) {
      return { value: {}, error: `${fieldName} must be a JSON object.` }
    }
    return { value: parsed, error: '' }
  } catch (err: any) {
    return { value: {}, error: err?.message || `${fieldName} is not valid JSON.` }
  }
}

function setOptionalString(target: Record<string, any>, key: string, value: string) {
  const trimmed = value.trim()
  if (trimmed) {
    target[key] = trimmed
    return
  }
  delete target[key]
}

function buildConfigFromDraft(baseRaw: string, draft: LocalSettingsDraft): { value: Record<string, any>; error: string } {
  const parsedBase = parseLocalConfigObject(baseRaw)
  if (parsedBase.error) {
    return { value: defaultLocalConfigObject(), error: parsedBase.error }
  }

  const profiles = parseObjectField(draft.profilesJson, 'profiles')
  if (profiles.error) {
    return { value: parsedBase.value, error: profiles.error }
  }

  const connections = parseObjectField(draft.connectionsJson, 'local.connections')
  if (connections.error) {
    return { value: parsedBase.value, error: connections.error }
  }

  const next = JSON.parse(JSON.stringify(parsedBase.value || defaultLocalConfigObject())) as Record<string, any>
  const local = isRecord(next.local) ? next.local : {}
  const currentTarget = normalizeTargetInput(draft.currentTarget)
  setOptionalString(next, 'current_target', currentTarget.target)
  setOptionalString(next, 'current_profile', currentTarget.profile)
  next.profiles = profiles.value

  setOptionalString(local, 'listen_addr', draft.listenAddr)
  setOptionalString(local, 'storage', draft.storage)
  setOptionalString(local, 'sqlite_path', draft.sqlitePath)
  setOptionalString(local, 'database_url', draft.databaseURL)
  setOptionalString(local, 'git_mirror_path', draft.gitMirrorPath)
  setOptionalString(local, 'public_base_url', draft.publicBaseURL)
  setOptionalString(local, 'jwt_secret', draft.jwtSecret)
  setOptionalString(local, 'vault_master_key', draft.vaultMasterKey)
  local.connections = connections.value

  next.local = local
  return { value: next, error: '' }
}

export default function SystemSettingsPage() {
  const { locale, tx } = useI18n()
  const [configViewMode, setConfigViewMode] = useState<ConfigViewMode>('settings')
  const [localConfig, setLocalConfig] = useState<LocalConfigFile | null>(null)
  const [localConfigBusy, setLocalConfigBusy] = useState(false)
  const [localConfigSaving, setLocalConfigSaving] = useState(false)
  const [localConfigError, setLocalConfigError] = useState('')
  const [localConfigMessage, setLocalConfigMessage] = useState('')
  const [localConfigRaw, setLocalConfigRaw] = useState('')
  const [settingsDraft, setSettingsDraft] = useState<LocalSettingsDraft>(() => draftFromRaw(''))
  const [visibleSecrets, setVisibleSecrets] = useState<Record<string, boolean>>({})
  const [gitMirror, setGitMirror] = useState<GitMirrorSettings | null>(null)
  const [gitMirrorBusy, setGitMirrorBusy] = useState(false)
  const [gitMirrorSaving, setGitMirrorSaving] = useState(false)
  const [gitMirrorTesting, setGitMirrorTesting] = useState(false)
  const [gitMirrorError, setGitMirrorError] = useState('')
  const [gitMirrorMessage, setGitMirrorMessage] = useState('')
  const [gitMirrorTokenInput, setGitMirrorTokenInput] = useState('')
  const [gitMirrorTokenTest, setGitMirrorTokenTest] = useState<GitMirrorGitHubTestResult | null>(null)
  const [gitMirrorDraft, setGitMirrorDraft] = useState<UpdateGitMirrorRequest>({
    auto_commit_enabled: false,
    auto_push_enabled: false,
    auth_mode: 'local_credentials',
    remote_name: 'origin',
    remote_url: '',
    remote_branch: 'main',
  })

  const syncLocalConfig = (config: LocalConfigFile) => {
    setLocalConfig(config)
    setLocalConfigRaw(config.raw || '')
    setSettingsDraft(draftFromRaw(config.raw || ''))
  }

  const loadLocalConfig = async () => {
    setLocalConfigBusy(true)
    setLocalConfigError('')
    try {
      const config = await api.getLocalConfig()
      syncLocalConfig(config)
    } catch (err: any) {
      setLocalConfigError(err.message || tx('加载 config.json 失败', 'Failed to load config.json'))
    } finally {
      setLocalConfigBusy(false)
    }
  }

  const syncGitMirrorDraft = (settings: GitMirrorSettings) => {
    setGitMirror(settings)
    setGitMirrorDraft({
      auto_commit_enabled: settings.auto_commit_enabled,
      auto_push_enabled: settings.auto_push_enabled,
      auth_mode: settings.auth_mode,
      remote_name: 'origin',
      remote_url: settings.remote_url || '',
      remote_branch: 'main',
    })
    setGitMirrorTokenInput('')
    setGitMirrorTokenTest(null)
  }

  const loadGitMirror = async () => {
    setGitMirrorBusy(true)
    setGitMirrorError('')
    try {
      const settings = await api.getLocalGitMirror()
      syncGitMirrorDraft(settings)
    } catch (err: any) {
      setGitMirrorError(err.message || tx('加载 Git Mirror 配置失败', 'Failed to load Git Mirror settings'))
    } finally {
      setGitMirrorBusy(false)
    }
  }

  useEffect(() => {
    void loadLocalConfig()
    void loadGitMirror()
  }, [])

  const localConfigRawValidationError = useMemo(() => {
    if (!localConfigRaw.trim()) {
      return tx('config.json 不能为空。', 'config.json cannot be empty.')
    }
    const parsed = parseLocalConfigObject(localConfigRaw)
    if (parsed.error) {
      return parsed.error
    }
    return ''
  }, [localConfigRaw, tx])

  const settingsValidationError = useMemo(() => {
    const result = buildConfigFromDraft(localConfigRaw, settingsDraft)
    return result.error
  }, [localConfigRaw, settingsDraft])

  const activeConfigValidationError = configViewMode === 'raw'
    ? localConfigRawValidationError
    : settingsValidationError

  const updateSettingsDraft = (patch: Partial<LocalSettingsDraft>) => {
    setSettingsDraft((prev) => ({ ...prev, ...patch }))
    setLocalConfigMessage('')
    setLocalConfigError('')
  }

  const updateGitMirrorDraft = (patch: Partial<UpdateGitMirrorRequest>) => {
    setGitMirrorDraft((prev) => {
      const next = { ...prev, ...patch, remote_name: 'origin', remote_branch: 'main' }
      if (!next.auto_commit_enabled) {
        next.auto_push_enabled = false
      }
      if (next.auto_push_enabled) {
        next.auto_commit_enabled = true
      }
      return next
    })
    setGitMirrorError('')
    setGitMirrorMessage('')
    setGitMirrorTokenTest(null)
  }

  const toggleSecretVisibility = (key: string) => {
    setVisibleSecrets((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  const handleSwitchToRaw = () => {
    const next = buildConfigFromDraft(localConfigRaw, settingsDraft)
    if (next.error) {
      setLocalConfigError(next.error)
      setLocalConfigMessage('')
      return
    }
    setLocalConfigRaw(stringifyLocalConfig(next.value))
    setConfigViewMode('raw')
    setLocalConfigError('')
    setLocalConfigMessage('')
  }

  const handleSwitchToSettings = () => {
    const parsed = parseLocalConfigObject(localConfigRaw)
    if (parsed.error) {
      setLocalConfigError(tx('当前 config.json 不是有效 JSON，修复后才能切回设置视图。', 'Fix the current config.json before switching back to the settings view.'))
      setLocalConfigMessage('')
      return
    }
    setSettingsDraft(draftFromRaw(localConfigRaw))
    setConfigViewMode('settings')
    setLocalConfigError('')
    setLocalConfigMessage('')
  }

  const handleLocalConfigSave = async () => {
    const rawToSave = configViewMode === 'raw'
      ? localConfigRaw
      : stringifyLocalConfig(buildConfigFromDraft(localConfigRaw, settingsDraft).value)

    if (activeConfigValidationError) {
      setLocalConfigError(activeConfigValidationError)
      setLocalConfigMessage('')
      return
    }

    setLocalConfigSaving(true)
    setLocalConfigError('')
    setLocalConfigMessage('')
    try {
      const saved = await api.updateLocalConfig({ raw: rawToSave })
      syncLocalConfig(saved)
      setLocalConfigMessage(tx('系统设置已保存', 'System settings saved'))
    } catch (err: any) {
      setLocalConfigError(err.message || tx('保存系统设置失败', 'Failed to save system settings'))
    } finally {
      setLocalConfigSaving(false)
    }
  }

  const handleGitMirrorTest = async () => {
    setGitMirrorTesting(true)
    setGitMirrorError('')
    setGitMirrorMessage('')
    try {
      const result = await api.testGitMirrorGitHubToken({
        remote_url: gitMirrorDraft.remote_url || '',
        github_token: gitMirrorTokenInput.trim(),
      })
      setGitMirrorTokenTest(result)
      if (result.normalized_remote_url) {
        setGitMirrorDraft((prev) => ({ ...prev, remote_url: result.normalized_remote_url || prev.remote_url }))
      }
      if (result.ok) {
        setGitMirrorMessage(localizeGitHubAccessMessage(result.message, locale) || tx('GitHub token 可用', 'GitHub token is valid'))
      } else {
        setGitMirrorError(localizeGitHubAccessMessage(result.message, locale) || tx('GitHub token 校验失败', 'GitHub token validation failed'))
      }
    } catch (err: any) {
      setGitMirrorError(err.message || tx('GitHub token 测试失败', 'Failed to test GitHub token'))
    } finally {
      setGitMirrorTesting(false)
    }
  }

  const handleGitMirrorSave = async () => {
    setGitMirrorSaving(true)
    setGitMirrorError('')
    setGitMirrorMessage('')
    try {
      const saved = await api.updateLocalGitMirror({
        ...gitMirrorDraft,
        remote_name: 'origin',
        remote_branch: 'main',
        github_token: gitMirrorTokenInput.trim() || undefined,
      })
      syncGitMirrorDraft(saved)
      setGitMirrorMessage(tx('Git Mirror 配置已保存', 'Git Mirror settings saved'))
    } catch (err: any) {
      setGitMirrorError(err.message || tx('保存 Git Mirror 配置失败', 'Failed to save Git Mirror settings'))
    } finally {
      setGitMirrorSaving(false)
    }
  }

  const handleGitMirrorClearToken = async () => {
    setGitMirrorSaving(true)
    setGitMirrorError('')
    setGitMirrorMessage('')
    try {
      const saved = await api.updateLocalGitMirror({
        ...gitMirrorDraft,
        auto_push_enabled: false,
        remote_name: 'origin',
        remote_branch: 'main',
        clear_github_token: true,
      })
      syncGitMirrorDraft(saved)
      setGitMirrorMessage(tx('已清除保存的 GitHub token', 'Saved GitHub token was cleared'))
    } catch (err: any) {
      setGitMirrorError(err.message || tx('清除 GitHub token 失败', 'Failed to clear GitHub token'))
    } finally {
      setGitMirrorSaving(false)
    }
  }

  const renderSensitiveInput = (
    id: string,
    value: string,
    onChange: (value: string) => void,
    visibilityKey: string,
    placeholder?: string,
  ) => {
    const visible = !!visibleSecrets[visibilityKey]
    const hasValue = value.trim().length > 0
    return (
      <div className="data-sync-secret-row">
        <input
          id={id}
          className="data-sync-secret-input"
          type="text"
          value={visible ? value : (hasValue ? '******' : '')}
          readOnly={!visible}
          onChange={(e) => {
            if (!visible) return
            onChange(e.target.value)
          }}
          placeholder={visible ? placeholder : (placeholder || '******')}
        />
        <button
          type="button"
          className="btn data-sync-visibility-btn"
          onClick={() => toggleSecretVisibility(visibilityKey)}
          aria-label={visible ? tx('隐藏敏感值', 'Hide sensitive value') : tx('显示敏感值', 'Show sensitive value')}
          title={visible ? tx('隐藏', 'Hide') : tx('显示', 'Show')}
        >
          <EyeIcon visible={visible} />
        </button>
      </div>
    )
  }

  return (
    <div className="page materials-page">
      <div className="materials-panel data-sync-card">
        <div className="card-header">
          <h3 className="card-title">{tx('本地配置', 'Local Configuration')}</h3>
        </div>
        {localConfig?.path && (
          <div className="data-record-secondary">
            {tx('文件位置：', 'File path: ')}<code>{localConfig.path}</code>
          </div>
        )}
        <div className="alert alert-warn" style={{ marginTop: 12 }}>
          {tx('注意：这里可能包含 token、密钥和本地路径。部分配置变更需要重启本地服务后才会生效。', 'This file can contain tokens, secrets, and local paths. Some changes require restarting the local service before they take effect.')}
        </div>
        {localConfigBusy && <div className="page-loading">{tx('加载中...', 'Loading...')}</div>}
        {!localConfigBusy && (
          <>
            {configViewMode === 'settings' ? (
              <div className="data-sync-settings-shell">
                <section className="data-sync-settings-section">
                  <h4 className="data-sync-section-title">{tx('基础配置', 'Basics')}</h4>
                  <div className="data-sync-settings-grid">
                    <div className="form-group">
                      <label htmlFor="config-current-target">{tx('当前 target', 'Current target')}</label>
                      <div className="data-sync-field-note">{tx('CLI 默认使用的 target。填 `local` 或 `profile:official`；也可以直接填 profile 名称。', 'The default CLI target. Use `local` or `profile:official`; a bare profile name also works.')}</div>
                      <input id="config-current-target" value={settingsDraft.currentTarget} onChange={(e) => updateSettingsDraft({ currentTarget: e.target.value })} />
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-listen-addr">{tx('本地监听地址', 'Local listen address')}</label>
                      <div className="data-sync-field-note">{tx('本地服务的 HTTP 监听地址。', 'The HTTP listen address for the local service.')}</div>
                      <input id="config-listen-addr" value={settingsDraft.listenAddr} onChange={(e) => updateSettingsDraft({ listenAddr: e.target.value })} placeholder="127.0.0.1:42690" />
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-storage">{tx('本地存储后端', 'Local storage backend')}</label>
                      <div className="data-sync-field-note">{tx('通常保持 sqlite；只有你明确准备好了数据库时再切到 postgres。', 'Usually keep sqlite unless you have intentionally prepared a database for postgres.')}</div>
                      <input id="config-storage" value={settingsDraft.storage} onChange={(e) => updateSettingsDraft({ storage: e.target.value })} placeholder="sqlite" />
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-sqlite-path">SQLite path</label>
                      <div className="data-sync-field-note">{tx('当 local.storage=sqlite 时使用的数据库文件位置。', 'The SQLite database file used when local.storage=sqlite.')}</div>
                      <input id="config-sqlite-path" value={settingsDraft.sqlitePath} onChange={(e) => updateSettingsDraft({ sqlitePath: e.target.value })} placeholder="~/.local/share/neudrive/neudrive.db" />
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-database-url">Database URL</label>
                      <div className="data-sync-field-note">{tx('当 local.storage=postgres 时使用的数据库连接串。', 'The database connection string used when local.storage=postgres.')}</div>
                      {renderSensitiveInput('config-database-url', settingsDraft.databaseURL, (value) => updateSettingsDraft({ databaseURL: value }), 'database-url', 'postgres://localhost:5432/neudrive?sslmode=disable')}
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-git-mirror-path">{tx('Git Mirror 默认目录', 'Default Git Mirror path')}</label>
                      <div className="data-sync-field-note">{tx('首次初始化本地 Git Mirror 时，neuDrive 会优先使用这里的目录。', 'When neuDrive initializes the local Git Mirror for the first time, it uses this directory first.')}</div>
                      <input id="config-git-mirror-path" value={settingsDraft.gitMirrorPath} onChange={(e) => updateSettingsDraft({ gitMirrorPath: e.target.value })} placeholder="./neudrive-export/git-mirror" />
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-public-base-url">Public base URL</label>
                      <div className="data-sync-field-note">{tx('Dashboard、OAuth 回调和本地 CLI 页面跳转时使用的服务地址。', 'The service URL used by the dashboard, OAuth callbacks, and local CLI browser redirects.')}</div>
                      <input id="config-public-base-url" value={settingsDraft.publicBaseURL} onChange={(e) => updateSettingsDraft({ publicBaseURL: e.target.value })} placeholder="http://127.0.0.1:42690" />
                    </div>
                  </div>
                </section>

                <section className="data-sync-settings-section">
                  <h4 className="data-sync-section-title">{tx('敏感配置', 'Sensitive settings')}</h4>
                  <div className="data-sync-settings-grid">
                    <div className="form-group">
                      <label htmlFor="config-jwt-secret">JWT secret</label>
                      <div className="data-sync-field-note">{tx('本地 auth token 签名密钥。通常只在自管部署或调试时才需要改。', 'The signing secret for local auth tokens. You usually only change this for self-managed setups or debugging.')}</div>
                      {renderSensitiveInput('config-jwt-secret', settingsDraft.jwtSecret, (value) => updateSettingsDraft({ jwtSecret: value }), 'jwt-secret')}
                    </div>
                    <div className="form-group">
                      <label htmlFor="config-vault-master-key">Vault master key</label>
                      <div className="data-sync-field-note">{tx('本地 vault 的主密钥；请谨慎修改。', 'The master key for the local vault. Change with care.')}</div>
                      {renderSensitiveInput('config-vault-master-key', settingsDraft.vaultMasterKey, (value) => updateSettingsDraft({ vaultMasterKey: value }), 'vault-master-key')}
                    </div>
                  </div>
                </section>

                <section className="data-sync-settings-section">
                  <h4 className="data-sync-section-title">{tx('高级 JSON 字段', 'Advanced JSON fields')}</h4>
                  <div className="data-sync-settings-grid data-sync-settings-grid-wide">
                    <div className="form-group data-sync-settings-span-wide">
                      <label htmlFor="config-profiles-json">profiles</label>
                      <div className="data-sync-field-note">{tx('hosted profile 集合。适合维护多个官方云端或自托管环境。', 'The set of hosted profiles. Useful when you maintain multiple official-cloud or self-hosted environments.')}</div>
                      <textarea id="config-profiles-json" className="data-sync-config-editor data-sync-json-editor" value={settingsDraft.profilesJson} onChange={(e) => updateSettingsDraft({ profilesJson: e.target.value })} />
                      <div className="data-sync-example">
                        <div className="data-sync-example-title">Example</div>
                        <pre>{REMOTE_PROFILES_EXAMPLE}</pre>
                      </div>
                    </div>
                    <div className="form-group data-sync-settings-span-wide">
                      <label htmlFor="config-connections-json">local.connections</label>
                      <div className="data-sync-field-note">{tx('本地 adapter / MCP 连接配置。', 'The local adapter / MCP connection map.')}</div>
                      <textarea id="config-connections-json" className="data-sync-config-editor data-sync-json-editor" value={settingsDraft.connectionsJson} onChange={(e) => updateSettingsDraft({ connectionsJson: e.target.value })} />
                      <div className="data-sync-example">
                        <div className="data-sync-example-title">Example</div>
                        <pre>{LOCAL_CONNECTIONS_EXAMPLE}</pre>
                      </div>
                    </div>
                  </div>
                </section>
              </div>
            ) : (
              <textarea className="data-sync-config-editor" value={localConfigRaw} onChange={(e) => { setLocalConfigRaw(e.target.value); setLocalConfigMessage(''); setLocalConfigError('') }} spellCheck={false} />
            )}

            <div className="data-sync-actions">
              <button type="button" className={`btn ${configViewMode === 'settings' ? 'data-sync-mode-button-active' : ''}`} onClick={handleSwitchToSettings}>
                {tx('设置视图', 'Settings view')}
              </button>
              <button type="button" className={`btn ${configViewMode === 'raw' ? 'data-sync-mode-button-active' : ''}`} onClick={handleSwitchToRaw}>
                config.json
              </button>
              <button type="button" className="btn btn-primary" disabled={localConfigSaving} onClick={handleLocalConfigSave}>
                {localConfigSaving ? tx('保存中...', 'Saving...') : tx('保存配置', 'Save settings')}
              </button>
            </div>
          </>
        )}
        {localConfigError && <div className="alert alert-warn" style={{ marginTop: 16 }}>{localConfigError}</div>}
        {localConfigMessage && <div className="alert alert-ok" style={{ marginTop: 16 }}>{localConfigMessage}</div>}
      </div>

      <div className="materials-panel data-sync-card">
        <div className="card-header">
          <h3 className="card-title">{tx('Git Mirror Advanced', 'Git Mirror Advanced')}</h3>
        </div>
        <p className="data-record-secondary">
          {tx('这里是本地部署的高级备份配置。普通 GitHub Backup 页面会固定使用 origin/main。', 'Advanced backup settings for local deployments. The main GitHub Backup page always uses origin/main.')}
        </p>

        {gitMirrorBusy && <div className="page-loading">{tx('加载中...', 'Loading...')}</div>}
        {!gitMirrorBusy && gitMirror && (
          <>
            <div className="data-sync-status-grid" style={{ marginTop: 16 }}>
              <div className="data-sync-status-card">
                <div className="data-record-title">{tx('镜像目录', 'Mirror path')}</div>
                <code>{gitMirror.path || tx('还没有初始化', 'Not initialized')}</code>
              </div>
              <div className="data-sync-status-card">
                <div className="data-record-title">{tx('最近同步', 'Last sync')}</div>
                <div className="data-record-secondary">
                  {gitMirror.last_synced_at ? formatDateTime(gitMirror.last_synced_at, locale) : tx('还没有', 'Not yet')}
                </div>
              </div>
              <div className="data-sync-status-card">
                <div className="data-record-title">{tx('最近推送', 'Last push')}</div>
                <div className="data-record-secondary">
                  {gitMirror.last_push_at ? formatDateTime(gitMirror.last_push_at, locale) : tx('还没有', 'Not yet')}
                </div>
              </div>
            </div>

            <div className="data-sync-settings-shell">
              <section className="data-sync-settings-section">
                <h4 className="data-sync-section-title">{tx('同步行为', 'Sync behavior')}</h4>
                <div className="data-sync-toggle-grid">
                  <label className="data-sync-toggle-card">
                    <div className="data-sync-toggle-copy">
                      <div className="data-sync-toggle-title">{tx('自动 commit', 'Auto commit')}</div>
                      <div className="data-sync-field-note">
                        {tx('每次 Hub 写入后，自动在本地 mirror 仓库创建一条提交。', 'Create a commit in the local mirror after each Hub write.')}
                      </div>
                    </div>
                    <input
                      type="checkbox"
                      checked={gitMirrorDraft.auto_commit_enabled}
                      onChange={(e) => updateGitMirrorDraft({ auto_commit_enabled: e.target.checked })}
                    />
                  </label>
                  <label className="data-sync-toggle-card">
                    <div className="data-sync-toggle-copy">
                      <div className="data-sync-toggle-title">{tx('自动 push', 'Auto push')}</div>
                      <div className="data-sync-field-note">
                        {tx('同步成功后推送到远端；开启时会自动开启 auto commit。', 'Push to the remote after sync; enabling this also enables auto commit.')}
                      </div>
                    </div>
                    <input
                      type="checkbox"
                      checked={gitMirrorDraft.auto_push_enabled}
                      onChange={(e) => updateGitMirrorDraft({ auto_push_enabled: e.target.checked, auto_commit_enabled: e.target.checked ? true : gitMirrorDraft.auto_commit_enabled })}
                    />
                  </label>
                </div>
              </section>

              <section className="data-sync-settings-section">
                <h4 className="data-sync-section-title">{tx('远端配置', 'Remote settings')}</h4>
                <div className="data-sync-settings-grid">
                  <div className="form-group">
                    <label htmlFor="settings-git-mirror-auth-mode">{tx('认证方式', 'Auth mode')}</label>
                    <select
                      id="settings-git-mirror-auth-mode"
                      value={gitMirrorDraft.auth_mode}
                      onChange={(e) => updateGitMirrorDraft({ auth_mode: e.target.value as UpdateGitMirrorRequest['auth_mode'] })}
                    >
                      <option value="local_credentials">{tx('本机 Git 凭证', 'Local Git credentials')}</option>
                      <option value="github_token">{tx('GitHub Token', 'GitHub token')}</option>
                      <option value="github_app_user">{tx('GitHub App 用户', 'GitHub App user')}</option>
                    </select>
                  </div>
                  <div className="form-group data-sync-settings-span-wide">
                    <label htmlFor="settings-git-mirror-remote-url">{tx('仓库 URL', 'Repository URL')}</label>
                    <input
                      id="settings-git-mirror-remote-url"
                      value={gitMirrorDraft.remote_url || ''}
                      onChange={(e) => updateGitMirrorDraft({ remote_url: e.target.value })}
                      placeholder="https://github.com/owner/neudrive-backup.git"
                    />
                  </div>
                </div>
                <div className="data-record-secondary" style={{ marginTop: 8 }}>
                  {tx('远端名称和分支固定为 origin/main。', 'Remote name and branch are fixed to origin/main.')}
                </div>
              </section>

              {gitMirrorDraft.auth_mode === 'github_token' && (
                <section className="data-sync-settings-section">
                  <h4 className="data-sync-section-title">{tx('GitHub Token', 'GitHub token')}</h4>
                  <div className="form-group data-sync-settings-span-wide">
                    <label htmlFor="settings-git-mirror-token">{tx('GitHub Token', 'GitHub token')}</label>
                    <div className="data-sync-field-note">
                      {gitMirror.github_token_configured
                        ? tx('已经保存过 token；这里不会回显原值。填写新 token 会替换旧值。', 'A token is already saved; its raw value is never shown. Entering a new token replaces it.')
                        : tx('当前还没有保存 GitHub token。', 'No GitHub token is saved yet.')}
                    </div>
                    <div className="data-sync-secret-row">
                      <input
                        id="settings-git-mirror-token"
                        className="data-sync-secret-input"
                        type={visibleSecrets.gitMirrorToken ? 'text' : 'password'}
                        value={gitMirrorTokenInput}
                        onChange={(e) => {
                          setGitMirrorTokenInput(e.target.value)
                          setGitMirrorTokenTest(null)
                          setGitMirrorError('')
                          setGitMirrorMessage('')
                        }}
                        placeholder={gitMirror.github_token_configured ? tx('留空以继续使用已保存的 token', 'Leave blank to keep the saved token') : 'ghp_xxx'}
                      />
                      <button
                        type="button"
                        className="btn data-sync-visibility-btn"
                        onClick={() => toggleSecretVisibility('gitMirrorToken')}
                        aria-label={visibleSecrets.gitMirrorToken ? tx('隐藏 GitHub token', 'Hide GitHub token') : tx('显示 GitHub token', 'Show GitHub token')}
                        title={visibleSecrets.gitMirrorToken ? tx('隐藏', 'Hide') : tx('显示', 'Show')}
                      >
                        <EyeIcon visible={!!visibleSecrets.gitMirrorToken} />
                      </button>
                    </div>
                  </div>
                  <div className="data-sync-actions data-sync-actions-compact">
                    <button className="btn" type="button" disabled={gitMirrorTesting || !gitMirrorDraft.remote_url || !gitMirrorTokenInput.trim()} onClick={handleGitMirrorTest}>
                      {gitMirrorTesting ? tx('测试中...', 'Testing...') : tx('测试 Token', 'Test token')}
                    </button>
                    {gitMirror.github_token_configured && (
                      <button className="btn" type="button" disabled={gitMirrorSaving} onClick={handleGitMirrorClearToken}>
                        {tx('清除已保存 Token', 'Clear saved token')}
                      </button>
                    )}
                  </div>
                  {gitMirrorTokenTest && (
                    <div className={gitMirrorTokenTest.ok ? 'alert alert-ok' : 'alert alert-warn'} style={{ marginTop: 12 }}>
                      {gitMirrorTokenTest.message}
                    </div>
                  )}
                </section>
              )}

              {gitMirrorDraft.auth_mode === 'github_app_user' && (
                <section className="data-sync-settings-section">
                  <h4 className="data-sync-section-title">{tx('GitHub App 用户', 'GitHub App user')}</h4>
                  <div className="alert alert-warn">
                    {tx('GitHub App 授权和默认仓库创建请在 GitHub Backup 页面完成。这里仅保留高级配置。', 'Complete GitHub App authorization and default repository creation on the GitHub Backup page. This area only keeps advanced settings.')}
                  </div>
                </section>
              )}
            </div>

            {gitMirrorMessage && <div className="alert alert-ok" style={{ marginTop: 12 }}>{gitMirrorMessage}</div>}
            {gitMirrorError && <div className="alert alert-warn" style={{ marginTop: 12 }}>{gitMirrorError}</div>}
            {gitMirror.last_error && <div className="alert alert-warn" style={{ marginTop: 12 }}>{gitMirror.last_error}</div>}

            <div className="data-sync-actions">
              <button className="btn btn-primary" type="button" onClick={handleGitMirrorSave} disabled={gitMirrorSaving}>
                {gitMirrorSaving ? tx('保存中...', 'Saving...') : tx('保存 Git Mirror 配置', 'Save Git Mirror settings')}
              </button>
              <button className="btn" type="button" onClick={() => { void loadGitMirror() }} disabled={gitMirrorBusy || gitMirrorSaving}>
                {tx('重新加载', 'Reload')}
              </button>
            </div>
          </>
        )}
        {!gitMirrorBusy && !gitMirror && gitMirrorError && (
          <div className="alert alert-warn" style={{ marginTop: 16 }}>{gitMirrorError}</div>
        )}
      </div>
    </div>
  )
}
