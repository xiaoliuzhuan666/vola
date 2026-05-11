import { useEffect, useMemo, useState } from 'react'
import { api, type ScopedTokenResponse } from '../api'
import { useI18n } from '../i18n'
import { formatDateTime } from './data/DataShared'

type TokenPurpose = 'cli' | 'api' | 'custom'
type TokenFilter = 'active' | 'all' | 'revoked'
type AccessChoice = 'readonly' | 'readwrite' | 'custom'

const accessPresets: Record<AccessChoice, {
  label: { zh: string; en: string }
  description: { zh: string; en: string }
  trust: number
  scopes: string[]
}> = {
  readonly: {
    label: { zh: '只读', en: 'Read-only' },
    description: { zh: '允许 Agent 读取你的 Profile、Memory、文件树和技能信息。', en: 'Allows the agent to read your profile, memory, file tree, and skills.' },
    trust: 3,
    scopes: ['read:profile', 'read:memory', 'read:tree', 'read:skills', 'read:projects', 'search'],
  },
  readwrite: {
    label: { zh: '读+写', en: 'Read + write' },
    description: { zh: '允许 Agent 读取资料，并写入 Memory、Projects 和文件树。', en: 'Allows the agent to read data and write memory, projects, and file tree entries.' },
    trust: 4,
    scopes: ['read:profile', 'read:memory', 'write:memory', 'read:skills', 'read:vault.auth', 'read:projects', 'write:projects', 'read:tree', 'write:tree', 'search'],
  },
  custom: {
    label: { zh: '自定义', en: 'Custom' },
    description: { zh: '手动选择这个 Agent 需要的权限范围。', en: 'Manually choose the scopes this agent needs.' },
    trust: 3,
    scopes: ['read:profile', 'read:memory', 'read:tree', 'search'],
  },
}

const purposePresets: Record<TokenPurpose, {
  name: string
  days: number
}> = {
  cli: {
    name: 'CLI Local App',
    days: 90,
  },
  api: {
    name: 'REST API Token',
    days: 90,
  },
  custom: {
    name: 'Custom Agent',
    days: 30,
  },
}

function sameScopes(left: string[], right: string[]) {
  if (left.length !== right.length) return false
  const rightSet = new Set(right)
  return left.every((scope) => rightSet.has(scope))
}

function accessChoiceForScopes(scopes: string[]): AccessChoice {
  if (sameScopes(scopes, accessPresets.readonly.scopes)) return 'readonly'
  if (sameScopes(scopes, accessPresets.readwrite.scopes)) return 'readwrite'
  return 'custom'
}

function copyText(value: string) {
  if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(value)
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
  return Promise.resolve()
}

function tokenStatus(token: ScopedTokenResponse) {
  if (token.is_revoked) return 'Revoked'
  if (token.is_expired) return 'Expired'
  return 'Active'
}

function tokenPurpose(scopes: string[]) {
  if (scopes.includes('write:projects')) return 'CLI / Local app'
  if (scopes.includes('write:tree')) return 'API / Local app'
  if (scopes.length === 0) return 'Unknown'
  return 'Custom Agent'
}

export default function DeveloperAccessPage() {
  const { locale, tx } = useI18n()
  const [tokens, setTokens] = useState<ScopedTokenResponse[]>([])
  const [name, setName] = useState(purposePresets.custom.name)
  const [accessChoice, setAccessChoice] = useState<AccessChoice>('readonly')
  const [days, setDays] = useState(purposePresets.custom.days)
  const [scopes, setScopes] = useState<string[]>(accessPresets.readonly.scopes)
  const [availableScopes, setAvailableScopes] = useState<string[]>([])
  const [newToken, setNewToken] = useState('')
  const [tokenCopied, setTokenCopied] = useState('')
  const [filter, setFilter] = useState<TokenFilter>('active')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    setLoading(true)
    const [tokenResult, scopesResult] = await Promise.allSettled([
      api.getTokens(),
      api.getTokenScopes(),
    ])
    if (tokenResult.status === 'fulfilled') setTokens(tokenResult.value || [])
    else setError(tokenResult.reason?.message || tx('Token 加载失败', 'Failed to load tokens'))
    if (scopesResult.status === 'fulfilled') setAvailableScopes(scopesResult.value.scopes || [])
    setLoading(false)
  }

  useEffect(() => {
    void load()
  }, [])

  const filteredTokens = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    return tokens.filter((token) => {
      if (filter === 'active' && (token.is_revoked || token.is_expired)) return false
      if (filter === 'revoked' && !token.is_revoked && !token.is_expired) return false
      if (!normalizedQuery) return true
      const haystack = `${token.name} ${token.token_prefix} ${token.scopes.join(' ')}`.toLowerCase()
      return haystack.includes(normalizedQuery)
    })
  }, [filter, query, tokens])

  const selectedAccess = accessPresets[accessChoice]
  const isCustomAccess = accessChoice === 'custom'
  const trust = selectedAccess.trust

  const updateAccessChoice = (next: AccessChoice) => {
    setAccessChoice(next)
    setScopes(accessPresets[next].scopes)
  }

  const createToken = async () => {
    setCreating(true)
    setError('')
    setNewToken('')
    try {
      const response = await api.createToken({
        name,
        scopes,
        max_trust_level: trust,
        expires_in_days: days,
      })
      setNewToken(response.token)
      await load()
      setFilter('active')
    } catch (err: any) {
      setError(err?.message || tx('创建 token 失败', 'Failed to create token'))
    } finally {
      setCreating(false)
    }
  }

  const revoke = async (token: ScopedTokenResponse) => {
    if (!window.confirm(tx(`吊销 ${token.name}？`, `Revoke ${token.name}?`))) return
    try {
      await api.revokeToken(token.id)
      await load()
    } catch (err: any) {
      setError(err?.message || tx('吊销失败', 'Failed to revoke token'))
    }
  }

  const rename = async (token: ScopedTokenResponse) => {
    const nextName = window.prompt(tx('新的 token 名称', 'New token name'), token.name)
    if (!nextName || nextName === token.name) return
    try {
      await api.updateToken(token.id, { name: nextName })
      await load()
    } catch (err: any) {
      setError(err?.message || tx('改名失败', 'Failed to rename token'))
    }
  }

  const handleCopy = async (value: string, key: string) => {
    try {
      await copyText(value)
    } finally {
      setTokenCopied(key)
      window.setTimeout(() => setTokenCopied(''), 1600)
    }
  }

  if (loading) return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>

  return (
    <div className="page developer-page developer-access-page">
      {error && <div className="alert alert-warn">{error}</div>}

      <section className="developer-manual-card">
        <div>
          <h2>{tx('开发者访问用于什么？', 'What is Developer Access for?')}</h2>
          <p>
            {tx(
              '这里用来为命令行工具、本地应用或自定义 Agent 创建访问凭证。',
              'Create access credentials here for CLI tools, local apps, or custom agents.',
            )}
          </p>
        </div>
        <ol>
          <li>{tx('选择只读、读+写或自定义权限。', 'Choose read-only, read + write, or custom permissions.')}</li>
          <li>{tx('创建后立即复制访问凭证；它只会显示一次。', 'Copy the credential immediately after creation; it is shown only once.')}</li>
          <li>{tx('不再使用时，在列表里吊销。', 'Revoke it from the list when it is no longer needed.')}</li>
        </ol>
      </section>

      <section className="developer-create-card">
        <div className="developer-section-title">
          <h3>{tx('创建 Token', 'Create token')}</h3>
        </div>

        <div className="developer-create-grid">
          <div className="developer-form-card">
            <div className="developer-form">
              <label>
                {tx('名称', 'Name')}
                <input className="input" value={name} onChange={(event) => setName(event.target.value)} />
              </label>
              <label>
                {tx('信任等级', 'Trust level')}
                <select value={accessChoice} onChange={(event) => updateAccessChoice(event.target.value as AccessChoice)}>
                  <option value="readonly">{tx('只读', 'Read-only')}</option>
                  <option value="readwrite">{tx('读+写', 'Read + write')}</option>
                  <option value="custom">{tx('自定义', 'Custom')}</option>
                </select>
              </label>
              <label>
                {tx('有效期', 'Expires')}
                <select value={days} onChange={(event) => setDays(Number(event.target.value))}>
                  <option value={7}>7 days</option>
                  <option value={30}>30 days</option>
                  <option value={90}>90 days</option>
                  <option value={365}>365 days</option>
                  <option value={0}>{tx('永不过期', 'Never')}</option>
                </select>
              </label>
            </div>

            <button className="btn btn-primary developer-create-button" disabled={creating || !name.trim() || scopes.length === 0} onClick={() => { void createToken() }}>
              {creating ? tx('生成中...', 'Creating...') : tx('创建 token', 'Create token')}
            </button>
          </div>

          <aside className="developer-permission-card">
            <div className="developer-permission-head">
              <strong>{tx(`${selectedAccess.label.zh}权限范围`, `${selectedAccess.label.en} scopes`)}</strong>
            </div>
            <p>{tx(selectedAccess.description.zh, selectedAccess.description.en)}</p>
            <div className="developer-scope-summary">
              <span>{tx(`${scopes.length} 项权限`, `${scopes.length} scopes`)}</span>
              <span>{days === 0 ? tx('永不过期', 'Never expires') : tx(`${days} 天`, `${days} days`)}</span>
            </div>
            <div className="developer-scope-preview">
              {scopes.map((scope) => <code key={scope}>{scope}</code>)}
            </div>
            {isCustomAccess && (
              <div className="scope-chip-grid developer-scope-grid">
                {availableScopes.map((scope) => (
                  <label key={scope} className="scope-chip">
                    <input type="checkbox" checked={scopes.includes(scope)} onChange={() => setScopes((current) => current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope])} />
                    {scope}
                  </label>
                ))}
              </div>
            )}
          </aside>
        </div>

        {newToken && (
          <div className="token-once-card developer-token-once-card">
            <strong>{tx('现在复制这个 token。关闭后将无法再次查看。', 'Copy this token now. You will not be able to see it again.')}</strong>
            <code>{newToken}</code>
            <div className="token-copy-actions">
              <button className="btn btn-primary" onClick={() => { void handleCopy(newToken, 'token') }}>
                {tokenCopied === 'token' ? tx('已复制 ✓', 'Copied ✓') : tx('复制 token', 'Copy token')}
              </button>
              <button className="btn btn-outline" onClick={() => { void handleCopy(`export NEUDRIVE_TOKEN=${newToken}`, 'env') }}>
                {tokenCopied === 'env' ? tx('已复制 ✓', 'Copied ✓') : tx('复制环境变量', 'Copy env var')}
              </button>
            </div>
          </div>
        )}
      </section>

      <section className="developer-token-card">
        <div className="developer-token-toolbar">
          <div>
            <h3>{tx('Token 列表', 'Tokens')}</h3>
          </div>
          <div className="developer-token-controls">
            <input className="input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={tx('搜索 token', 'Search tokens')} />
            <div className="segmented-control">
              {([
                ['active', tx('可用', 'Active')],
                ['all', tx('全部', 'All')],
                ['revoked', tx('停用', 'Inactive')],
              ] as [TokenFilter, string][]).map(([key, label]) => (
                <button key={key} className={filter === key ? 'active' : ''} onClick={() => setFilter(key)}>{label}</button>
              ))}
            </div>
          </div>
        </div>

        {filteredTokens.length > 0 ? (
          <div className="developer-token-table-wrap">
            <table className="data-table developer-token-table">
              <thead>
                <tr>
                  <th>{tx('名称', 'Name')}</th>
                  <th>{tx('状态', 'Status')}</th>
                  <th>{tx('用途', 'Purpose')}</th>
                  <th>{tx('信任等级', 'Trust level')}</th>
                  <th>{tx('请求', 'Requests')}</th>
                  <th>{tx('过期时间', 'Expires')}</th>
                  <th>{tx('最后使用', 'Last used')}</th>
                  <th>{tx('操作', 'Actions')}</th>
                </tr>
              </thead>
              <tbody>
                {filteredTokens.map((token) => (
                  <tr key={token.id} className={token.is_expired || token.is_revoked ? 'is-muted' : ''}>
                    <td><strong>{token.name}</strong><small>{token.token_prefix}...</small></td>
                    <td><span className={token.is_expired || token.is_revoked ? 'status-pill' : 'status-pill connected'}>{tokenStatus(token)}</span></td>
                    <td>{tokenPurpose(token.scopes)}</td>
                    <td>{tx(accessPresets[accessChoiceForScopes(token.scopes)].label.zh, accessPresets[accessChoiceForScopes(token.scopes)].label.en)}</td>
                    <td>{token.request_count || 0}</td>
                    <td>{token.expires_at ? formatDateTime(token.expires_at, locale) : tx('永不过期', 'Never')}</td>
                    <td>{formatDateTime(token.last_used_at, locale)}</td>
                    <td>
                      <div className="table-actions">
                        <button className="btn-text" onClick={() => { void rename(token) }}>{tx('改名', 'Rename')}</button>
                        {!token.is_revoked && <button className="btn-text danger" onClick={() => { void revoke(token) }}>{tx('吊销', 'Revoke')}</button>}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="empty-action-state"><p>{tx('没有匹配的 token。', 'No matching tokens.')}</p></div>
        )}
      </section>
    </div>
  )
}
