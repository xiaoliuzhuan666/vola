import { useEffect, useState, type FormEvent } from 'react'
import { useLocation } from 'react-router-dom'
import { api, isTauri, type FileNode, type LocalCloudPushResponse, type LocalCloudStatus, type MemoryConflict } from '../api'
import { useI18n } from '../i18n'
import CustomSelect from '../components/CustomSelect'

const profileFields = [
  { key: 'preferences', label: { zh: '工作偏好', en: 'Work preferences' } },
  { key: 'writing_style', label: { zh: '写作风格', en: 'Writing style' } },
  { key: 'communication', label: { zh: '沟通偏好', en: 'Communication preferences' } },
  { key: 'principles', label: { zh: '决策风格', en: 'Decision style' } },
]

const commonLanguages = [
  { value: 'zh-CN', label: '中文（简体）' },
  { value: 'zh-TW', label: '中文（繁體）' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
  { value: 'fr', label: 'Français' },
  { value: 'de', label: 'Deutsch' },
  { value: 'es', label: 'Español' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'it', label: 'Italiano' },
  { value: 'nl', label: 'Nederlands' },
  { value: 'ru', label: 'Русский' },
  { value: 'ar', label: 'العربية' },
  { value: 'hi', label: 'हिन्दी' },
  { value: 'id', label: 'Bahasa Indonesia' },
  { value: 'vi', label: 'Tiếng Việt' },
  { value: 'th', label: 'ไทย' },
  { value: 'tr', label: 'Türkçe' },
  { value: 'pl', label: 'Polski' },
]

type DesktopUpdateStatus = 'idle' | 'checking' | 'downloading' | 'ready' | 'up-to-date' | 'error'

function formatBytes(value: number | undefined, locale: string) {
  const bytes = Number(value || 0)
  if (!Number.isFinite(bytes) || bytes <= 0) return locale === 'zh-CN' ? '0 字节' : '0 bytes'
  const units = locale === 'zh-CN' ? ['字节', 'KB', 'MB', 'GB'] : ['bytes', 'KB', 'MB', 'GB']
  let current = bytes
  let unitIndex = 0
  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024
    unitIndex += 1
  }
  return `${current >= 10 || unitIndex === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[unitIndex]}`
}

function quotaPercent(status: LocalCloudStatus | null) {
  const used = Number(status?.quota?.storage_used_bytes || 0)
  const limit = Number(status?.quota?.effective_storage_quota_bytes || 0)
  if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return 0
  return Math.min(100, Math.max(0, (used / limit) * 100))
}

export default function InfoPage() {
  const { locale, tx } = useI18n()
  const location = useLocation()
  const [values, setValues] = useState<Record<string, string>>({})
  const [userProfile, setUserProfile] = useState<Record<string, any>>({})
  const [entries, setEntries] = useState<FileNode[]>([])
  const [conflicts, setConflicts] = useState<MemoryConflict[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  // Cloud login states
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [syncLoading, setSyncLoading] = useState(false)
  const [pushLoading, setPushLoading] = useState(false)
  const [localMode, setLocalMode] = useState(false)
  const [cloudStatus, setCloudStatus] = useState<LocalCloudStatus | null>(null)
  const [lastPush, setLastPush] = useState<LocalCloudPushResponse | null>(null)
  const [isSignUpMode, setIsSignUpMode] = useState(false)
  const [signupSlug, setSignupSlug] = useState('')
  const [signupDisplayName, setSignupDisplayName] = useState('')
  const [desktopUpdateStatus, setDesktopUpdateStatus] = useState<DesktopUpdateStatus>('idle')
  const [desktopUpdateDetail, setDesktopUpdateDetail] = useState('')
  const [desktopUpdateProgress, setDesktopUpdateProgress] = useState(0)

  const load = async () => {
    setLoading(true)
    setError('')
    const [meResult, profileResult, snapshotResult, conflictsResult, configResult, cloudResult] = await Promise.allSettled([
      api.getMe(),
      api.getProfile(),
      api.getTreeSnapshot('/'),
      api.getConflicts(),
      api.getPublicConfig(),
      api.getLocalCloudStatus(),
    ])
    const me = meResult.status === 'fulfilled' ? meResult.value || {} : {}
    if (meResult.status === 'fulfilled') setUserProfile(me)
    if (profileResult.status === 'fulfilled') {
      const next = profileResult.value || {}
      const prefs = next.preferences || {}
      setValues({
        display_name: String(me.display_name || next.display_name || prefs.display_name || ''),
        language: String(me.language || locale),
        preferences: String(prefs.preferences || ''),
        writing_style: String(prefs.writing_style || ''),
        communication: String(prefs.communication || ''),
        principles: String(prefs.principles || ''),
      })
    }
    if (snapshotResult.status === 'fulfilled') setEntries(snapshotResult.value.entries)
    if (conflictsResult.status === 'fulfilled') setConflicts(conflictsResult.value || [])
    if (configResult.status === 'fulfilled') {
      setLocalMode(!!configResult.value?.local_mode)
    }
    if (cloudResult.status === 'fulfilled') {
      setCloudStatus(cloudResult.value || null)
    }
    setLoading(false)
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    if (loading || location.hash !== '#cloud-account') return
    const scrollToCloudAccount = () => {
      document.getElementById('cloud-account')?.scrollIntoView({ block: 'start', inline: 'nearest' })
    }
    const frame = window.requestAnimationFrame(() => {
      scrollToCloudAccount()
    })
    const timeout = window.setTimeout(scrollToCloudAccount, 180)
    return () => {
      window.cancelAnimationFrame(frame)
      window.clearTimeout(timeout)
    }
  }, [loading, location.hash])

  const saveProfile = async () => {
    setSaving('profile')
    setError('')
    try {
      await Promise.all([
        api.updateMe({
          display_name: values.display_name || '',
          bio: String(userProfile.bio || ''),
          timezone: String(userProfile.timezone || 'UTC'),
          language: values.language || locale,
        }),
        api.upsertProfile({
          preferences: {
            preferences: values.preferences || '',
            writing_style: values.writing_style || '',
            communication: values.communication || '',
            principles: values.principles || '',
          },
        }),
      ])
      setMessage(tx('Profile 已保存。', 'Profile saved.'))
      await load()
    } catch (err: any) {
      setError(err?.message || tx('保存失败', 'Save failed'))
    } finally {
      setSaving('')
    }
  }

  const resolveConflict = async (id: string, resolution: string) => {
    try {
      await api.resolveConflict(id, resolution)
      setConflicts((current) => current.filter((item) => item.id !== id))
      setMessage(tx('冲突已解决。', 'Conflict resolved.'))
    } catch (err: any) {
      setError(err?.message || tx('解决冲突失败', 'Failed to resolve conflict'))
    }
  }

  const exportAll = async () => {
    await api.exportZip()
    setMessage(tx('导出已开始。', 'Export started.'))
  }

  const deleteImportedConversations = async () => {
    if (!window.confirm(tx('删除所有导入会话？此操作不可撤销。', 'Delete all imported conversations? This cannot be undone.'))) return
    const conversations = entries.filter((entry) => entry.path.startsWith('/conversations/') && (entry.is_dir || entry.name.endsWith('.md')))
    for (const entry of conversations) {
      await api.deleteTree(entry.path)
    }
    setMessage(tx('导入会话已删除。', 'Imported conversations deleted.'))
    await load()
  }

  const clearMemory = async () => {
    if (!window.confirm(tx('清空 Memory？Profile 之外的记忆会被删除。', 'Clear memory? Memory outside Profile will be deleted.'))) return
    const memory = entries.filter((entry) => entry.path.startsWith('/memory/') && !entry.path.startsWith('/memory/profile/'))
    for (const entry of memory) {
      await api.deleteTree(entry.path)
    }
    setMessage(tx('Memory 已清空。', 'Memory cleared.'))
    await load()
  }

  const revokeAllTokens = async () => {
    if (!window.confirm(tx('撤销所有 token？所有外部 Agent 将失去访问权限。', 'Revoke all tokens? External agents will lose access.'))) return
    const tokens = await api.getTokens()
    for (const token of tokens) {
      if (!token.is_revoked) await api.revokeToken(token.id)
    }
    setMessage(tx('Token 已全部撤销。', 'All tokens revoked.'))
  }

  const handleCloudLogin = async (e: FormEvent) => {
    e.preventDefault()
    if (syncLoading) return
    setSyncLoading(true)
    setError('')
    setMessage('')
    try {
      const status = await api.loginLocalCloud({ email, password })
      setCloudStatus(status)
      setPassword('')
      setMessage(tx('官方云账号已连接。现在可以把本机资料上传到云端。', 'Official cloud account connected. You can now upload local data to the cloud.'))
    } catch (err: any) {
      setError(err?.message || tx('登录失败，请检查您的邮箱和密码。', 'Login failed. Please check your email and password.'))
    } finally {
      setSyncLoading(false)
    }
  }

  const handleCloudSignup = async (e: FormEvent) => {
    e.preventDefault()
    if (syncLoading) return
    setSyncLoading(true)
    setError('')
    setMessage('')
    if (password !== confirmPassword) {
      setError(tx('两次输入的密码不一致，请重新输入。', 'Passwords do not match. Please try again.'))
      setSyncLoading(false)
      return
    }
    const accountSlug = (signupSlug.trim() || slugFromEmail(email)).toLowerCase()
    if (!accountSlug) {
      setError(tx('请填写账户名。', 'Please enter an account name.'))
      setSyncLoading(false)
      return
    }
    try {
      const status = await api.registerLocalCloud({
        email,
        password,
        slug: accountSlug,
        display_name: signupDisplayName.trim() || accountSlug,
      })
      setCloudStatus(status)
      setPassword('')
      setConfirmPassword('')
      setMessage(tx('官方云账号已创建，初始额度已返回。可以上传本机资料查看用量变化。', 'Official cloud account created and quota returned. Upload local data to see usage change.'))
    } catch (err: any) {
      setError(err?.message || tx('注册并绑定失败，请检查表单信息。', 'Failed to register and bind account. Check form fields.'))
    } finally {
      setSyncLoading(false)
    }
  }

  const handleCloudLogout = async () => {
    if (!window.confirm(tx('确定要解除这台电脑上的官方云账号绑定吗？云端账号和云端数据不会被删除。', 'Disconnect the official cloud account on this computer? The cloud account and cloud data will not be deleted.'))) return
    setSyncLoading(true)
    setError('')
    setMessage('')
    try {
      const status = await api.disconnectLocalCloud()
      setCloudStatus(status)
      setLastPush(null)
      setMessage(tx('已解除这台电脑上的官方云账号绑定。', 'Official cloud account disconnected on this computer.'))
    } catch (err: any) {
      setError(err?.message || tx('解除绑定失败。', 'Failed to disconnect cloud account.'))
    } finally {
      setSyncLoading(false)
    }
  }

  const handleCloudPush = async () => {
    if (pushLoading) return
    setPushLoading(true)
    setError('')
    setMessage('')
    try {
      const result = await api.pushLocalCloud()
      setLastPush(result)
      setCloudStatus(result.status)
      const quota = result.status.quota
      const usage = quota
        ? tx(`云端已用 ${formatBytes(quota.storage_used_bytes, locale)} / ${formatBytes(quota.effective_storage_quota_bytes, locale)}。`, `Cloud usage is ${formatBytes(quota.storage_used_bytes, locale)} / ${formatBytes(quota.effective_storage_quota_bytes, locale)}.`)
        : ''
      if (result.warning || result.confirmed === false) {
        setMessage(tx(`云端用量已更新。${usage}`, `Cloud usage updated. ${usage}`))
        setError(result.warning || tx('云端已收到资料，但导入确认响应超时。请稍后刷新确认导入明细。', 'Cloud received the data, but import confirmation timed out. Refresh later to confirm import details.'))
      } else {
        setMessage(tx(`本机资料已上传到官方云端。${usage}`, `Local data uploaded to official cloud. ${usage}`))
      }
    } catch (err: any) {
      try {
        const status = await api.getLocalCloudStatus()
        setCloudStatus(status)
      } catch {}
      setError(err?.message || tx('上传到云端失败。', 'Failed to upload to cloud.'))
    } finally {
      setPushLoading(false)
    }
  }

  const checkDesktopUpdate = async () => {
    if (!isTauri || desktopUpdateStatus === 'checking' || desktopUpdateStatus === 'downloading') return
    setError('')
    setMessage('')
    setDesktopUpdateStatus('checking')
    setDesktopUpdateDetail(tx('正在检查桌面版本...', 'Checking desktop version...'))
    setDesktopUpdateProgress(0)

    let updateHandle: any = null
    try {
      const { check } = await import('@tauri-apps/plugin-updater')
      updateHandle = await check()
      if (!updateHandle) {
        setDesktopUpdateStatus('up-to-date')
        setDesktopUpdateDetail(tx('当前已经是最新版本。', 'You are already on the latest version.'))
        return
      }

      const nextVersion = String(updateHandle.version || '')
      let downloaded = 0
      let contentLength = 0
      setDesktopUpdateStatus('downloading')
      setDesktopUpdateDetail(tx(`正在下载 v${nextVersion}...`, `Downloading v${nextVersion}...`))

      await updateHandle.downloadAndInstall((event: any) => {
        if (event.event === 'Started') {
          contentLength = Number(event.data?.contentLength || 0)
          downloaded = 0
          setDesktopUpdateProgress(0)
          return
        }
        if (event.event === 'Progress') {
          downloaded += Number(event.data?.chunkLength || 0)
          setDesktopUpdateProgress((current) => {
            if (contentLength > 0) {
              return Math.min(100, Math.round((downloaded / contentLength) * 100))
            }
            return Math.min(95, current + 1)
          })
          return
        }
        if (event.event === 'Finished') {
          setDesktopUpdateProgress(100)
        }
      })

      setDesktopUpdateStatus('ready')
      setDesktopUpdateDetail(tx(`v${nextVersion} 已安装，重启后生效。`, `v${nextVersion} is installed and will apply after restart.`))
    } catch (err: any) {
      setDesktopUpdateStatus('error')
      setDesktopUpdateDetail(err?.message || String(err || tx('更新检查失败。', 'Update check failed.')))
    } finally {
      if (updateHandle) {
        await updateHandle.close().catch(() => {})
      }
    }
  }

  const restartDesktopApp = async () => {
    try {
      const { relaunch } = await import('@tauri-apps/plugin-process')
      await relaunch()
    } catch (err: any) {
      setDesktopUpdateStatus('error')
      setDesktopUpdateDetail(err?.message || String(err || tx('重启失败。', 'Relaunch failed.')))
    }
  }

  if (loading) return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>

  const cloudConnected = !!cloudStatus?.connected
  const cloudAccount = cloudStatus?.account || {}
  const cloudQuota = cloudStatus?.quota || null
  const cloudUsagePercent = quotaPercent(cloudStatus)
  const cloudAccountLabel = cloudAccount.email || cloudAccount.display_name || cloudAccount.slug || tx('未连接', 'Not connected')

  return (
    <div className="page profile-page">
      {message && <div className="alert alert-success">{message}</div>}
      {error && <div className="alert alert-warn">{error}</div>}

      <section className="card profile-main-card">
        <div className="card-header">
          <h3 className="card-title">{tx('Vola 了解你的信息', 'What Vola knows about you')}</h3>
          <button className="btn btn-primary" disabled={saving !== ''} onClick={() => { void saveProfile() }}>{saving ? tx('保存中...', 'Saving...') : tx('保存', 'Save Profile')}</button>
        </div>
        <div className="profile-field-grid">
          <label className="profile-edit-field">
            <span>{tx('名称', 'Name')}</span>
            <input className="input" value={values.display_name || ''} onChange={(event) => setValues({ ...values, display_name: event.target.value })} />
          </label>
          <label className="profile-edit-field">
            <span>{tx('语言偏好', 'Language preference')}</span>
            <CustomSelect
              value={values.language || locale}
              onChange={(val) => setValues({ ...values, language: val })}
              options={commonLanguages}
              ariaLabel={tx('语言偏好', 'Language preference')}
            />
          </label>
          {profileFields.map((field) => (
            <label key={field.key} className="profile-edit-field profile-long-field">
              <span>{tx(field.label.zh, field.label.en)}</span>
              <textarea value={values[field.key] || ''} onChange={(event) => setValues({ ...values, [field.key]: event.target.value })} />
            </label>
          ))}
        </div>
      </section>

      {conflicts.length > 0 && (
        <section className="card">
          <div className="card-header">
            <h3 className="card-title">{tx('记忆冲突', 'Memory conflicts')}</h3>
          </div>
          <div className="conflict-list">
            {conflicts.map((conflict) => (
              <div key={conflict.id} className="conflict-card">
                <strong>{conflict.category}</strong>
                <div className="conflict-options">
                  <div><span>{conflict.source_a}</span><p>{conflict.content_a}</p><button className="btn btn-outline" onClick={() => { void resolveConflict(conflict.id, 'keep_a') }}>{tx('保留', 'Keep')}</button></div>
                  <div><span>{conflict.source_b}</span><p>{conflict.content_b}</p><button className="btn btn-outline" onClick={() => { void resolveConflict(conflict.id, 'keep_b') }}>{tx('保留', 'Keep')}</button></div>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {localMode && (
        <section id="cloud-account" className="card cloud-sync-card">
          <div className="card-header">
            <h3 className="card-title">{tx('云同步与团队协作', 'Cloud Sync & Team Collaboration')}</h3>
          </div>
          <div className="card-body cloud-sync-body">
            {!cloudConnected ? (
              <div className="cloud-bind-flow">
                <p className="cloud-sync-copy">
                  {tx(
                    '当前资料仍保存在本机。连接官方云账号后，可以把本机资料上传到云端，并查看云端额度与使用量。',
                    'Your data is still stored locally. Connect an official cloud account to upload local data and view cloud quota usage.'
                  )}
                </p>
                {cloudStatus?.error && <div className="alert alert-warn">{cloudStatus.error}</div>}

                <div className="cloud-auth-tabs">
                  <button
                    type="button"
                    onClick={() => { setIsSignUpMode(false); setError(''); }}
                    className={!isSignUpMode ? 'cloud-auth-tab active' : 'cloud-auth-tab'}
                  >
                    {tx('登录已有账号', 'Sign In')}
                  </button>
                  <button
                    type="button"
                    onClick={() => { setIsSignUpMode(true); setError(''); }}
                    className={isSignUpMode ? 'cloud-auth-tab active' : 'cloud-auth-tab'}
                  >
                    {tx('创建新云账号', 'Create Account')}
                  </button>
                </div>

                <form onSubmit={(e) => { void (isSignUpMode ? handleCloudSignup(e) : handleCloudLogin(e)) }} className="cloud-auth-form">
                  <div className="form-group cloud-form-field">
                    <label className="form-label">{tx('邮箱地址', 'Email Address')}</label>
                    <input
                      className="input"
                      type="email"
                      placeholder="name@example.com"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      required
                    />
                  </div>
                  <div className="form-group cloud-form-field">
                    <label className="form-label">{tx('登录密码', 'Password')}</label>
                    <input
                      className="input"
                      type="password"
                      placeholder={isSignUpMode ? tx('至少 8 位密码', 'At least 8 characters') : '••••••••'}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      required
                      minLength={8}
                    />
                  </div>

                  {isSignUpMode && (
                    <>
                      <div className="form-group cloud-form-field">
                        <label className="form-label">{tx('确认密码', 'Confirm Password')}</label>
                        <input
                          className="input"
                          type="password"
                          placeholder={tx('再次输入密码', 'Repeat password')}
                          value={confirmPassword}
                          onChange={(e) => setConfirmPassword(e.target.value)}
                          required
                          minLength={8}
                        />
                        <small className="cloud-form-hint">
                          {tx('需要和上面的登录密码完全一致。', 'Must match the password above.')}
                        </small>
                      </div>
                      <div className="form-group cloud-form-field">
                        <label className="form-label">{tx('账户名 (Slug)', 'Account Name (Slug)')}</label>
                        <input
                          className="input"
                          type="text"
                          placeholder={slugFromEmail(email) || 'my-account'}
                          value={signupSlug}
                          onChange={(e) => setSignupSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
                          minLength={3}
                          required
                        />
                      </div>
                      <div className="form-group cloud-form-field">
                        <label className="form-label">{tx('显示名称', 'Display Name')}</label>
                        <input
                          className="input"
                          type="text"
                          placeholder={tx('例如：张三', 'e.g. John Doe')}
                          value={signupDisplayName}
                          onChange={(e) => setSignupDisplayName(e.target.value)}
                        />
                      </div>
                    </>
                  )}

                  <div className="cloud-auth-actions">
                    <button className="btn btn-primary" type="submit" disabled={syncLoading}>
                      {syncLoading
                        ? (isSignUpMode ? tx('注册中...', 'Registering...') : tx('登录中...', 'Signing in...'))
                        : (isSignUpMode ? tx('创建并连接', 'Create & Connect') : tx('登录并连接', 'Sign In & Connect'))
                      }
                    </button>

                    <button
                      type="button"
                      onClick={() => { setIsSignUpMode(!isSignUpMode); setError(''); }}
                      className="cloud-auth-link"
                    >
                      {isSignUpMode ? tx('已有账户？立即登录', 'Already have an account? Sign In') : tx('还没有账户？立即注册', 'Need an account? Register')}
                    </button>
                  </div>
                </form>
              </div>
            ) : (
              <div className="cloud-unbind-flow">
                <div className="cloud-status-head">
                  <span className="cloud-status-icon">C</span>
                  <div>
                    <div className="cloud-status-title">
                      {tx('已连接官方云账号', 'Official Cloud Connected')}
                    </div>
                    <div className="cloud-status-subtitle">
                      {tx(`账号：${cloudAccountLabel}`, `Account: ${cloudAccountLabel}`)}
                    </div>
                  </div>
                </div>
                <div className="cloud-quota-panel">
                  <div className="cloud-quota-row">
                    <span>{tx('官方云额度', 'Cloud quota')}</span>
                    <strong>
                      {cloudQuota
                        ? `${formatBytes(cloudQuota.storage_used_bytes, locale)} / ${formatBytes(cloudQuota.effective_storage_quota_bytes, locale)}`
                        : tx('等待云端返回', 'Waiting for cloud')}
                    </strong>
                  </div>
                  <div className="cloud-quota-bar" aria-hidden="true">
                    <div className="cloud-quota-fill" style={{ width: `${cloudUsagePercent}%` }} />
                  </div>
                  <div className="cloud-quota-meta">
                    <span>{cloudStatus?.api_base || ''}</span>
                    <span>{cloudQuota ? `${cloudUsagePercent.toFixed(cloudUsagePercent >= 10 ? 0 : 1)}%` : ''}</span>
                  </div>
                </div>

                {lastPush && (
                  <div className="cloud-push-summary">
                    <strong>{tx('上次上传', 'Last upload')}</strong>
                    <span>
                      {lastPush.import_result ? tx(
                        `上传 ${lastPush.bundle_stats.total_files || 0} 个文件，导入 ${lastPush.import_result.files_written || 0} 个文件、${lastPush.import_result.memory_imported || 0} 条记忆。`,
                        `Uploaded ${lastPush.bundle_stats.total_files || 0} files, imported ${lastPush.import_result.files_written || 0} files and ${lastPush.import_result.memory_imported || 0} memories.`
                      ) : tx(
                        `上传 ${lastPush.bundle_stats.total_files || 0} 个文件，云端用量已变化，导入明细等待确认。`,
                        `Uploaded ${lastPush.bundle_stats.total_files || 0} files. Cloud usage changed; import details are pending confirmation.`
                      )}
                    </span>
                  </div>
                )}

                {cloudStatus?.error && <div className="alert alert-warn">{cloudStatus.error}</div>}

                <div className="page-actions">
                  <button className="btn btn-primary" type="button" onClick={() => { void handleCloudPush() }} disabled={pushLoading || syncLoading}>
                    {pushLoading ? tx('上传中...', 'Uploading...') : tx('上传本机资料到云端', 'Upload Local Data')}
                  </button>
                  <button className="btn btn-danger" type="button" onClick={handleCloudLogout} disabled={syncLoading || pushLoading}>
                    {syncLoading ? tx('解除绑定中...', 'Disconnecting...') : tx('解除云账号绑定', 'Disconnect')}
                  </button>
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {isTauri && (
        <section className="card desktop-update-card">
          <div className="card-header">
            <h3 className="card-title">{tx('桌面应用更新', 'Desktop App Update')}</h3>
          </div>
          <div className="card-body" style={{ padding: '20px' }}>
            <p className="data-record-secondary" style={{ marginBottom: '14px' }}>
              {desktopUpdateDetail || tx('当前桌面版会从 GitHub Release 获取签名更新包。', 'The desktop app checks signed updates from GitHub Releases.')}
            </p>
            {desktopUpdateStatus === 'downloading' && (
              <div style={{ height: '8px', maxWidth: '360px', borderRadius: '999px', background: 'rgba(63, 76, 133, 0.12)', overflow: 'hidden', marginBottom: '14px' }}>
                <div style={{ width: `${desktopUpdateProgress}%`, height: '100%', background: '#414d88', transition: 'width 0.18s ease' }} />
              </div>
            )}
            <div className="page-actions">
              <button
                className="btn btn-primary"
                type="button"
                disabled={desktopUpdateStatus === 'checking' || desktopUpdateStatus === 'downloading'}
                onClick={() => { void checkDesktopUpdate() }}
              >
                {desktopUpdateStatus === 'checking'
                  ? tx('检查中...', 'Checking...')
                  : desktopUpdateStatus === 'downloading'
                    ? tx('下载中...', 'Downloading...')
                    : tx('检查更新', 'Check for Updates')}
              </button>
              {desktopUpdateStatus === 'ready' && (
                <button className="btn btn-outline" type="button" onClick={() => { void restartDesktopApp() }}>
                  {tx('重启应用', 'Restart App')}
                </button>
              )}
            </div>
          </div>
        </section>
      )}

      <section className="card privacy-actions">
        <div className="card-header">
          <h3 className="card-title">{tx('隐私操作', 'Privacy Actions')}</h3>
        </div>
        <div className="page-actions">
          <button className="btn btn-outline" onClick={() => { void exportAll() }}>{tx('导出全部数据', 'Export all data')}</button>
          <button className="btn btn-outline" onClick={() => { void deleteImportedConversations() }}>{tx('删除导入会话', 'Delete imported conversations')}</button>
          <button className="btn btn-outline" onClick={() => { void clearMemory() }}>{tx('清空记忆', 'Clear memory')}</button>
          <button className="btn btn-danger" onClick={() => { void revokeAllTokens() }}>{tx('撤销全部 token', 'Revoke all tokens')}</button>
        </div>
      </section>
    </div>
  )
}

function slugFromEmail(value: string) {
  const prefix = value.trim().toLowerCase().split('@')[0] || ''
  return prefix.replace(/[^a-z0-9_-]/g, '').slice(0, 32)
}
