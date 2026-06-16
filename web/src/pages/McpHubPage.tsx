import { useEffect, useState } from 'react'
import { api } from '../api'
import { useI18n } from '../i18n'

interface MCPClientStatus {
  id: string
  name: string
  installed: boolean
  registered: boolean
  config_path?: string
}

interface ConfigMCPServer {
  id: string
  name: string
  enabled: boolean
  command: string
  args: string[]
  env: Record<string, string>
}

interface MCPServersConfig {
  version: string
  servers: ConfigMCPServer[]
}

function localPlatformLabel(platform: string) {
  if (platform === 'claude-code' || platform === 'claude') return 'Claude Code'
  if (platform === 'codex') return 'Codex'
  return platform
}

export default function McpHubPage() {
  const { tx } = useI18n()

  // Clients Status State
  const [clients, setClients] = useState<MCPClientStatus[]>([])
  const [clientsLoading, setClientsLoading] = useState(true)

  // Config State
  const [config, setConfig] = useState<MCPServersConfig>({ version: '1.0.0', servers: [] })
  const [configLoading, setConfigLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  // Form State
  const [editingServer, setEditingServer] = useState<ConfigMCPServer | null>(null)
  const [formId, setFormId] = useState('')
  const [formName, setFormName] = useState('')
  const [formCommand, setFormCommand] = useState('')
  const [formArgsText, setFormArgsText] = useState('')
  const [formEnvText, setFormEnvText] = useState('')
  const [formEnabled, setFormEnabled] = useState(true)
  const [formError, setFormError] = useState('')

  // Messages
  const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null)
  const [platformRefreshBusy, setPlatformRefreshBusy] = useState('')

  const showMessage = (text: string, type: 'success' | 'error' = 'success') => {
    setMessage({ text, type })
    window.setTimeout(() => setMessage(null), 4000)
  }

  const handleRefreshLocalPlatform = async (platform: 'codex' | 'claude-code') => {
    if (platformRefreshBusy) return
    setPlatformRefreshBusy(platform)
    try {
      const result = await api.refreshLocalPlatformConnection(platform)
      showMessage(tx(
        `${result.name || localPlatformLabel(result.platform)} 已刷新，本地配置会包含已发布的团队 MCP。`,
        `${result.name || localPlatformLabel(result.platform)} refreshed; the local config includes published team MCPs.`,
      ))
    } catch (e: any) {
      showMessage(tx('刷新失败：', 'Refresh failed: ') + (e.message || e), 'error')
    } finally {
      setPlatformRefreshBusy('')
    }
  }

  // Load Claude clients status
  const loadClients = async () => {
    setClientsLoading(true)
    try {
      const data = await api.getLocalMCPClients()
      setClients(data)
    } catch (e) {
      console.error('Failed to load local MCP clients:', e)
    } finally {
      setClientsLoading(false)
    }
  }

  // Load external servers config from FileTree
  const loadConfig = async () => {
    setConfigLoading(true)
    try {
      const res = await api.getTree('/settings/mcp-servers.json')
      if (res && res.content) {
        const parsed = JSON.parse(res.content) as MCPServersConfig
        setConfig(parsed)
      }
    } catch (e: any) {
      // 404 is expected when config doesn't exist yet
      if (e.status === 404 || e.code === 'not_found' || /not found|file not found/i.test(String(e.message || ''))) {
        setConfig({ version: '1.0.0', servers: [] })
      } else {
        console.error('Failed to load mcp servers config:', e)
      }
    } finally {
      setConfigLoading(false)
    }
  }

  useEffect(() => {
    void loadClients()
    void loadConfig()
  }, [])

  // Save current config state to FileTree
  const saveConfig = async (newConfig: MCPServersConfig) => {
    setSaving(true)
    try {
      await api.writeTree('/settings/mcp-servers.json', {
        content: JSON.stringify(newConfig, null, 2),
        mimeType: 'application/json',
      })
      setConfig(newConfig)
      showMessage(tx('配置保存成功！', 'Configuration saved successfully!'))
    } catch (e) {
      console.error('Failed to save config:', e)
      showMessage(tx('保存配置失败', 'Failed to save configuration'), 'error')
    } finally {
      setSaving(false)
    }
  }

  // Handle client register
  const handleRegister = async (clientId: string) => {
    try {
      const res = await api.registerLocalMCPClient(clientId)
      if (res.success) {
        showMessage(tx('成功注册到本地客户端！重启客户端即可生效。', 'Registered successfully! Restart the client to take effect.'))
        void loadClients()
      }
    } catch (e: any) {
      console.error('Register client failed:', e)
      showMessage(tx('注册失败：', 'Registration failed: ') + (e.message || e), 'error')
    }
  }

  // Handle client unregister
  const handleUnregister = async (clientId: string) => {
    try {
      const res = await api.unregisterLocalMCPClient(clientId)
      if (res.success) {
        showMessage(tx('注销成功！', 'Unregistered successfully!'))
        void loadClients()
      }
    } catch (e: any) {
      console.error('Unregister client failed:', e)
      showMessage(tx('注销失败：', 'Unregistration failed: ') + (e.message || e), 'error')
    }
  }

  // Toggle server enabled state
  const handleToggleServer = async (serverId: string) => {
    const updatedServers = config.servers.map((s) => {
      if (s.id === serverId) {
        return { ...s, enabled: !s.enabled }
      }
      return s
    })
    const newConfig = { ...config, servers: updatedServers }
    await saveConfig(newConfig)
  }

  // Delete server
  const handleDeleteServer = async (serverId: string) => {
    if (!window.confirm(tx(`确定要删除 MCP 服务 "${serverId}" 吗？`, `Are you sure you want to delete MCP server "${serverId}"?`))) {
      return
    }
    const updatedServers = config.servers.filter((s) => s.id !== serverId)
    const newConfig = { ...config, servers: updatedServers }
    await saveConfig(newConfig)
    if (editingServer?.id === serverId) {
      cancelEdit()
    }
  }

  // Start creating new server
  const startCreate = () => {
    setEditingServer(null)
    setFormId('')
    setFormName('')
    setFormCommand('')
    setFormArgsText('')
    setFormEnvText('')
    setFormEnabled(true)
    setFormError('')
  }

  // Start editing existing server
  const startEdit = (server: ConfigMCPServer) => {
    setEditingServer(server)
    setFormId(server.id)
    setFormName(server.name)
    setFormCommand(server.command)
    setFormArgsText(server.args.join('\n'))

    const envLines = Object.entries(server.env || {})
      .map(([k, v]) => `${k}=${v}`)
      .join('\n')
    setFormEnvText(envLines)
    setFormEnabled(server.enabled)
    setFormError('')
  }

  const cancelEdit = () => {
    setEditingServer(null)
    setFormId('')
    setFormName('')
    setFormCommand('')
    setFormArgsText('')
    setFormEnvText('')
    setFormEnabled(true)
    setFormError('')
  }

  // Submit form
  const handleSaveForm = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError('')

    const id = formId.trim()
    const name = formName.trim()
    const command = formCommand.trim()

    // Validation
    if (!id) {
      setFormError(tx('ID 不能为空', 'ID cannot be empty'))
      return
    }
    if (!/^[a-zA-Z0-9_-]+$/.test(id)) {
      setFormError(tx('ID 只能包含英文字母、数字、下划线和连字符', 'ID can only contain alphanumeric characters, underscores, and hyphens'))
      return
    }
    if (!name) {
      setFormError(tx('名称不能为空', 'Name cannot be empty'))
      return
    }
    if (!command) {
      setFormError(tx('执行命令不能为空', 'Command cannot be empty'))
      return
    }

    // Check duplicate ID
    const isNew = editingServer === null
    if (isNew && config.servers.some((s) => s.id === id)) {
      setFormError(tx('已存在相同 ID 的 MCP 服务', 'An MCP server with the same ID already exists'))
      return
    }

    // Parse args (split by newlines, filter empty)
    const args = formArgsText
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0)

    // Parse env key-value pairs
    const env: Record<string, string> = {}
    const envLines = formEnvText.split('\n')
    for (const line of envLines) {
      const trimmed = line.trim()
      if (!trimmed) continue
      const eqIdx = trimmed.indexOf('=')
      if (eqIdx <= 0) {
        setFormError(tx(`环境变量格式错误: "${trimmed}"。必须是 KEY=VALUE 格式。`, `Invalid environment variable format: "${trimmed}". Must be KEY=VALUE.`))
        return
      }
      const k = trimmed.substring(0, eqIdx).trim()
      const v = trimmed.substring(eqIdx + 1).trim()
      if (!k) {
        setFormError(tx(`环境变量的 Key 不能为空`, 'Environment variable Key cannot be empty'))
        return
      }
      env[k] = v
    }

    const newServer: ConfigMCPServer = {
      id,
      name,
      enabled: formEnabled,
      command,
      args,
      env,
    }

    let updatedServers: ConfigMCPServer[]
    if (isNew) {
      updatedServers = [...config.servers, newServer]
    } else {
      updatedServers = config.servers.map((s) => (s.id === editingServer.id ? newServer : s))
    }

    const newConfig = { ...config, servers: updatedServers }
    await saveConfig(newConfig)
    cancelEdit()
  }

  return (
    <div className="page materials-page mcp-hub-page">
      {/* Title Hero */}
      <section className="materials-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">MCP Gateway</div>
          <h2 className="materials-title">{tx('MCP 控制中心', 'MCP Hub')}</h2>
          <p className="materials-subtitle">
            {tx(
              '一键将 Vola 连接到本地客户端（例如 Claude Desktop），或在此托管装配第三方的 Stdio MCP 服务，实现“单一连接，聚合全部本地 MCP 资源”。',
              'Register Vola into local clients (such as Claude Desktop), or host third-party Stdio MCP servers here to aggregate all local MCP resources.'
            )}
          </p>
        </div>
      </section>

      {/* Global alert messages */}
      {message && (
        <div className={`notification-banner ${message.type === 'error' ? 'banner-error' : 'banner-success'}`}>
          <div className="banner-content">
            <span className="banner-icon">{message.type === 'error' ? '⚠️' : '✓'}</span>
            <p className="banner-text">{message.text}</p>
          </div>
        </div>
      )}

      <section className="materials-panel">
        <div className="materials-panel-head">
          <div>
            <h3 className="card-title">{tx('本机同步入口', 'Local Sync Entry')}</h3>
            <p className="materials-section-copy">
              {tx(
                'Codex 和 Claude Code 可以直接刷新本机连接；Cursor 和 Gemini CLI 继续只导出，不会在这里改本机配置。',
                'Codex and Claude Code can refresh local connections directly; Cursor and Gemini CLI stay export-only and do not change local config here.',
              )}
            </p>
          </div>
          <div className="mcp-actions-wrap">
            <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'codex'} onClick={() => { void handleRefreshLocalPlatform('codex') }}>
              {platformRefreshBusy === 'codex' ? tx('刷新中...', 'Refreshing...') : tx('刷新 Codex', 'Refresh Codex')}
            </button>
            <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'claude-code'} onClick={() => { void handleRefreshLocalPlatform('claude-code') }}>
              {platformRefreshBusy === 'claude-code' ? tx('刷新中...', 'Refreshing...') : tx('刷新 Claude Code', 'Refresh Claude Code')}
            </button>
          </div>
        </div>
      </section>

      {/* Part 1: Local Client Integration */}
      <section className="materials-panel">
        <div className="materials-panel-head">
          <div>
            <h3 className="card-title">{tx('本地客户端集成', 'Local Client Integration')}</h3>
            <p className="materials-section-copy">
              {tx(
                '在本地配置文件中注册/注销 Vola 的 MCP 端点。',
                'Register or unregister Vola\'s MCP endpoint in your local client configurations.'
              )}
            </p>
          </div>
          <button className="btn btn-sm" type="button" onClick={() => void loadClients()}>
            {tx('刷新状态', 'Refresh')}
          </button>
        </div>

        {clientsLoading ? (
          <div className="mcp-loader-container">
            <div className="loading-spinner" />
          </div>
        ) : (
          <div className="mcp-client-list">
            {clients.length === 0 ? (
              <p className="mcp-empty-text">
                {tx('未检测到本地兼容的客户端安装（例如 Claude Desktop）。', 'No compatible local clients (e.g. Claude Desktop) detected.')}
              </p>
            ) : (
              clients.map((client) => (
                <div className="mcp-client-item" key={client.id}>
                  <div className="mcp-client-info">
                    <div className="mcp-client-main">
                      <span className="mcp-client-name">{client.name}</span>
                      <div className="mcp-client-badges">
                        {client.installed ? (
                          <span className="badge badge-success">{tx('已安装', 'Installed')}</span>
                        ) : (
                          <span className="badge badge-secondary">{tx('未检测到安装', 'Not Installed')}</span>
                        )}
                        {client.registered ? (
                          <span className="badge badge-info">{tx('已注册', 'Registered')}</span>
                        ) : (
                          <span className="badge badge-warning">{tx('未注册', 'Not Registered')}</span>
                        )}
                      </div>
                    </div>
                    {client.config_path && (
                      <span className="mcp-client-path">
                        {tx('配置文件：', 'Config path: ')}<code>{client.config_path}</code>
                      </span>
                    )}
                  </div>
                  <div className="mcp-client-actions">
                    {!client.registered ? (
                      <button
                        className="btn btn-primary"
                        type="button"
                        disabled={!client.installed}
                        onClick={() => handleRegister(client.id)}
                      >
                        {tx('一键注册', 'One-click Register')}
                      </button>
                    ) : (
                      <button
                        className="btn btn-danger"
                        type="button"
                        onClick={() => handleUnregister(client.id)}
                      >
                        {tx('解除注册', 'Unregister')}
                      </button>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </section>

      {/* Part 2: Third-party Server Configuration */}
      <section className="materials-panel">
        <div className="materials-panel-head">
          <div>
            <h3 className="card-title">{tx('第三方 MCP 服务装配', 'Third-party MCP Servers')}</h3>
            <p className="materials-section-copy">
              {tx(
                '添加外部 StdIO 协议的 MCP 进程。开启后，Vola 后端将自动维护其运行状态，并将所有 Tools 前缀隔离后聚合代理。',
                'Assemble external Stdio-based MCP processes. When enabled, Vola automatically spawns and manages them, proxying all tools with namespace isolation.'
              )}
            </p>
          </div>
          <div className="mcp-actions-wrap">
            <button
              className="btn btn-primary"
              type="button"
              onClick={startCreate}
              disabled={editingServer === null && formId === '' && formName === '' && formCommand === ''}
            >
              {tx('新建 MCP', 'Add MCP Server')}
            </button>
          </div>
        </div>

        {configLoading ? (
          <div className="mcp-loader-container">
            <div className="loading-spinner" />
          </div>
        ) : (
          <div className="mcp-server-container">
            {/* Servers List */}
            <div className="mcp-server-list">
              {config.servers.length === 0 ? (
                <div className="mcp-empty-state">
                  <span className="mcp-empty-icon">🔌</span>
                  <p className="mcp-empty-title">{tx('暂无装配的 MCP 服务', 'No custom MCP servers configured')}</p>
                  <p className="mcp-empty-desc">
                    {tx(
                      '点击右上角“新建 MCP”按钮，您可以添加类似 filesystem, memory 等本地或三方的 MCP 服务。',
                      'Click "Add MCP Server" to link external node/python Stdio processes.'
                    )}
                  </p>
                </div>
              ) : (
                config.servers.map((server) => (
                  <div
                    key={server.id}
                    className={`mcp-server-item ${server.enabled ? 'enabled' : 'disabled'} ${
                      editingServer?.id === server.id ? 'editing' : ''
                    }`}
                  >
                    <div className="mcp-server-main-info">
                      <div className="mcp-server-head">
                        <span className="mcp-server-name">{server.name}</span>
                        <code className="mcp-server-id">ID: {server.id}</code>
                        <span className={`badge ${server.enabled ? 'badge-success' : 'badge-secondary'}`}>
                          {server.enabled ? tx('已启用', 'Enabled') : tx('已禁用', 'Disabled')}
                        </span>
                      </div>
                      <div className="mcp-server-command">
                        <code>
                          {server.command} {server.args.join(' ')}
                        </code>
                      </div>
                      {Object.keys(server.env || {}).length > 0 && (
                        <div className="mcp-server-env-badges">
                          {Object.keys(server.env).map((k) => (
                            <span className="badge-env" key={k}>
                              {k}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="mcp-server-controls">
                      <div className="toggle-switch-container">
                        <label className="toggle-switch">
                          <input
                            type="checkbox"
                            checked={server.enabled}
                            onChange={() => handleToggleServer(server.id)}
                            disabled={saving}
                          />
                          <span className="toggle-slider"></span>
                        </label>
                      </div>

                      <button
                        className="btn btn-sm btn-edit"
                        type="button"
                        onClick={() => startEdit(server)}
                      >
                        {tx('编辑', 'Edit')}
                      </button>

                      <button
                        className="btn btn-sm btn-danger"
                        type="button"
                        onClick={() => handleDeleteServer(server.id)}
                        disabled={saving}
                      >
                        {tx('删除', 'Delete')}
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>

            {/* Server Editor Form (Inline sidebar/drawer or inline panel) */}
            {(editingServer !== null || formId !== '' || formName !== '' || formCommand !== '') && (
              <div className="mcp-server-form-panel">
                <h4 className="card-title">
                  {editingServer ? tx(`编辑 MCP: ${editingServer.id}`, `Edit MCP: ${editingServer.id}`) : tx('新建 MCP 服务', 'New MCP Server')}
                </h4>

                <form onSubmit={(e) => void handleSaveForm(e)}>
                  {formError && <div className="form-error-banner">{formError}</div>}

                  <div className="form-group">
                    <label className="form-label">
                      {tx('服务 ID (前缀隔离标识)', 'Server ID (Prefix Identifier)')} <span className="req">*</span>
                    </label>
                    <input
                      className="form-control"
                      type="text"
                      placeholder="e.g. filesystem"
                      value={formId}
                      onChange={(e) => setFormId(e.target.value)}
                      disabled={editingServer !== null} // ID cannot be edited once created
                      required
                    />
                    <p className="form-helper">
                      {tx('此 ID 会作为 Tools 调用时的前缀（例如 filesystem__read_file），必须唯一，只能由字母数字、下划线及连字符组成。', 'Used as namespace prefix for your tools, e.g. serverid__toolname. Alphanumeric, underscores, hyphens only.')}
                    </p>
                  </div>

                  <div className="form-group">
                    <label className="form-label">{tx('展示名称', 'Display Name')} <span className="req">*</span></label>
                    <input
                      className="form-control"
                      type="text"
                      placeholder="e.g. Filesystem MCP"
                      value={formName}
                      onChange={(e) => setFormName(e.target.value)}
                      required
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">{tx('执行命令', 'Command')} <span className="req">*</span></label>
                    <input
                      className="form-control"
                      type="text"
                      placeholder="e.g. npx or node"
                      value={formCommand}
                      onChange={(e) => setFormCommand(e.target.value)}
                      required
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">{tx('命令行参数 (每行一个)', 'Arguments (one per line)')}</label>
                    <textarea
                      className="form-control code-textarea"
                      rows={4}
                      placeholder={tx("-y\n@modelcontextprotocol/server-filesystem\n/path/to/directory", "-y\n@modelcontextprotocol/server-filesystem\n/path/to/directory")}
                      value={formArgsText}
                      onChange={(e) => setFormArgsText(e.target.value)}
                    />
                  </div>

                  <div className="form-group">
                    <label className="form-label">{tx('环境变量 (每行一个，格式为 KEY=VALUE)', 'Environment Variables (KEY=VALUE)')}</label>
                    <textarea
                      className="form-control code-textarea"
                      rows={3}
                      placeholder="API_KEY=your_key_here&#10;PORT=3000"
                      value={formEnvText}
                      onChange={(e) => setFormEnvText(e.target.value)}
                    />
                  </div>

                  <div className="form-group checkbox-group">
                    <label className="form-label-checkbox">
                      <input
                        type="checkbox"
                        checked={formEnabled}
                        onChange={(e) => setFormEnabled(e.target.checked)}
                      />
                      <span>{tx('立即启用该服务', 'Enable this server')}</span>
                    </label>
                  </div>

                  <div className="form-actions">
                    <button className="btn btn-primary" type="submit" disabled={saving}>
                      {saving ? tx('保存中...', 'Saving...') : tx('保存', 'Save')}
                    </button>
                    <button className="btn btn-secondary" type="button" onClick={cancelEdit}>
                      {tx('取消', 'Cancel')}
                    </button>
                  </div>
                </form>
              </div>
            )}
          </div>
        )}
      </section>
    </div>
  )
}
