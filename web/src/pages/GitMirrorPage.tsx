import { useEffect, useMemo, useState } from 'react'
import {
  api,
  type BackupRun,
  type BackupRestoreApplyResult,
  type BackupRestorePreview,
  type BackupRunResult,
  type BackupTarget,
  type BackupTargetKind,
  type GitMirrorGitHubTestResult,
  type GitMirrorSettings,
  type OpsInstanceStatus,
  type OpsStatus,
  type PublicConfig,
  type SaveBackupTargetRequest,
  type UpdateGitMirrorRequest,
} from '../api'
import { useI18n } from '../i18n'
import { formatDateTime, localizeGitHubAccessMessage } from './data/DataShared'
import CustomSelect from '../components/CustomSelect'

type AuthMode = UpdateGitMirrorRequest['auth_mode']
type AuthHelp = {
  title: string
  intro: string
  steps?: string[]
  footer?: string
}

const DEFAULT_AUTH_MODE: AuthMode = 'local_credentials'
const DEFAULT_REMOTE_NAME = 'origin'
const DEFAULT_REMOTE_BRANCH = 'main'
const GITHUB_APP_PERMISSION_UPDATE_REQUIRED_CODE = 'github_app_permission_update_required'

function defaultBackupDraft(): SaveBackupTargetRequest {
  return {
    kind: 'webdav',
    name: 'WebDAV backup',
    enabled: true,
    s3_path_style: true,
    auto_backup_enabled: false,
    auto_backup_interval_hours: 24,
    retention_keep_last: 0,
    retention_keep_days: 0,
  }
}

function githubRepoLabel(remoteURL?: string) {
  const value = (remoteURL || '').trim()
  if (!value) return ''
  const sshMatch = value.match(/^git@github\.com:([^/]+\/[^.]+)(?:\.git)?$/)
  if (sshMatch) return sshMatch[1]
  const httpsMatch = value.match(/^https:\/\/github\.com\/([^/]+\/[^.]+)(?:\.git)?$/)
  if (httpsMatch) return httpsMatch[1]
  return value
}

function githubRepoHref(remoteURL?: string) {
  const label = githubRepoLabel(remoteURL)
  if (!label || label === remoteURL) return remoteURL || ''
  return `https://github.com/${label.replace(/\.git$/, '')}`
}

function latestUpdateTime(mirror: GitMirrorSettings | null) {
  return mirror?.last_push_at || mirror?.last_synced_at || ''
}

function isGitHubHTTPSURL(value: string) {
  try {
    const parsed = new URL(value.trim())
    return (parsed.protocol === 'https:' || parsed.protocol === 'http:') && parsed.hostname.toLowerCase() === 'github.com'
  } catch {
    return false
  }
}

function authModeForExecution(mode: AuthMode | undefined, executionMode: string, tokenConfigured?: boolean): AuthMode {
  if (executionMode === 'hosted' && mode === 'local_credentials') {
    return tokenConfigured ? 'github_token' : 'github_app_user'
  }
  return mode || (executionMode === 'hosted' ? 'github_app_user' : DEFAULT_AUTH_MODE)
}

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function isGitHubAppPermissionUpdateRequiredError(err: any) {
  const message = String(err?.message || '')
  return err?.code === GITHUB_APP_PERMISSION_UPDATE_REQUIRED_CODE ||
    /Repository Administration/i.test(message) ||
    /Resource not accessible by integration/i.test(message)
}

function formatBytes(value: number | undefined, locale: 'zh-CN' | 'en') {
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

function PencilIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" d="m4 20 4.6-1.1L19.5 8a2.1 2.1 0 0 0 0-3L19 4.5a2.1 2.1 0 0 0-3 0L5.1 15.4 4 20Z" />
      <path fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" d="m14.5 6.5 3 3" />
    </svg>
  )
}

function InfoIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="9" fill="none" stroke="currentColor" strokeWidth="2" />
      <path fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" d="M9.8 9a2.4 2.4 0 0 1 4.6 1c0 1.8-2.4 2-2.4 3.7" />
      <path fill="currentColor" d="M12 17.8a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Z" />
    </svg>
  )
}

export default function GitMirrorPage() {
  const { locale, tx } = useI18n()
  const [mirror, setMirror] = useState<GitMirrorSettings | null>(null)
  const [publicConfig, setPublicConfig] = useState<PublicConfig | null>(null)
  const [busy, setBusy] = useState(true)
  const [working, setWorking] = useState(false)
  const [testing, setTesting] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const [githubAppError, setGithubAppError] = useState('')
  const [githubAppReauthRequired, setGithubAppReauthRequired] = useState(false)
  const [authMode, setAuthMode] = useState<AuthMode>(DEFAULT_AUTH_MODE)
  const [authHelpOpen, setAuthHelpOpen] = useState(false)
  const [remoteURL, setRemoteURL] = useState('')
  const [urlEditing, setUrlEditing] = useState(false)
  const [tokenInput, setTokenInput] = useState('')
  const [tokenEditing, setTokenEditing] = useState(false)
  const [tokenTest, setTokenTest] = useState<GitMirrorGitHubTestResult | null>(null)
  const [backupTargets, setBackupTargets] = useState<BackupTarget[]>([])
  const [backupRuns, setBackupRuns] = useState<BackupRun[]>([])
  const [backupDraft, setBackupDraft] = useState<SaveBackupTargetRequest>(() => defaultBackupDraft())
  const [backupSaving, setBackupSaving] = useState(false)
  const [backupRunningID, setBackupRunningID] = useState('')
  const [backupError, setBackupError] = useState('')
  const [backupMessage, setBackupMessage] = useState('')
  const [backupResult, setBackupResult] = useState<BackupRunResult | null>(null)
  const [restoreFile, setRestoreFile] = useState<File | null>(null)
  const [restorePreview, setRestorePreview] = useState<BackupRestorePreview | null>(null)
  const [restoreMode, setRestoreMode] = useState<'skip' | 'overwrite'>('skip')
  const [restoreApplyResult, setRestoreApplyResult] = useState<BackupRestoreApplyResult | null>(null)
  const [restorePreviewing, setRestorePreviewing] = useState(false)
  const [restoreApplying, setRestoreApplying] = useState(false)
  const [restoreError, setRestoreError] = useState('')
  const [opsStatus, setOpsStatus] = useState<OpsStatus | null>(null)
  const [opsInstanceStatus, setOpsInstanceStatus] = useState<OpsInstanceStatus | null>(null)
  const [syncRetryUntil, setSyncRetryUntil] = useState(0)
  const [nowTick, setNowTick] = useState(Date.now())
  const [isDragging, setIsDragging] = useState(false)

  const loadPage = async () => {
    setBusy(true)
    setError('')
    try {
      const [settings, config, targets, runs, ops, instanceOps] = await Promise.all([
        api.getGitMirror(),
        api.getPublicConfig().catch(() => ({} as PublicConfig)),
        api.listBackupTargets().catch(() => [] as BackupTarget[]),
        api.listBackupRuns(20).catch(() => [] as BackupRun[]),
        api.getOpsStatus().catch(() => null as OpsStatus | null),
        api.getOpsInstanceStatus().catch(() => null as OpsInstanceStatus | null),
      ])
      const nextExecutionMode = settings.execution_mode || config.git_mirror_execution_mode || 'hosted'
      setMirror(settings)
      setPublicConfig(config)
      if (settings.github_app_user_connected) {
        setGithubAppReauthRequired(false)
      }
      setAuthMode(authModeForExecution(settings.auth_mode, nextExecutionMode, settings.github_token_configured))
      setRemoteURL(settings.remote_url || '')
      setUrlEditing(!(settings.remote_url || '').trim() && settings.auth_mode !== 'github_app_user')
      setTokenInput('')
      setTokenEditing(!settings.github_token_configured)
      setTokenTest(null)
      setBackupTargets(targets)
      setBackupRuns(runs)
      setOpsStatus(ops)
      setOpsInstanceStatus(instanceOps)
    } catch (err: any) {
      setError(err.message || tx('加载 GitHub Backup 失败', 'Failed to load GitHub Backup'))
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    void loadPage()
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const status = params.get('github_app_status')
    const callbackError = params.get('github_app_error')
    if (!status && !callbackError) return
    if (callbackError) {
      setGithubAppError(callbackError)
    } else if (status === 'connected') {
      setGithubAppReauthRequired(false)
      setMessage(tx('GitHub 已连接。现在可以创建备份仓库。', 'GitHub connected. You can create the backup repository now.'))
    }
    void loadPage()
    params.delete('github_app_status')
    params.delete('github_app_error')
    const nextSearch = params.toString()
    window.history.replaceState({}, '', `${window.location.pathname}${nextSearch ? `?${nextSearch}` : ''}`)
  }, [tx])

  useEffect(() => {
    if (!syncRetryUntil) return
    const timer = window.setInterval(() => setNowTick(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [syncRetryUntil])

  const executionMode = mirror?.execution_mode || publicConfig?.git_mirror_execution_mode || 'hosted'
  const isLocalExecution = executionMode === 'local'
  const githubAppConfigured = !!publicConfig?.github_app_enabled
  const githubAppUnavailableMessage = tx(
    '当前服务没有配置 GitHub App 授权。请设置 GITHUB_APP_CLIENT_ID、GITHUB_APP_CLIENT_SECRET、GITHUB_APP_SLUG、PUBLIC_BASE_URL 和 JWT_SECRET 后重启服务。',
    'GitHub App authorization is not configured for this service. Set GITHUB_APP_CLIENT_ID, GITHUB_APP_CLIENT_SECRET, GITHUB_APP_SLUG, PUBLIC_BASE_URL, and JWT_SECRET, then restart the service.',
  )
  const remoteRepoURL = mirror?.remote_url || ''
  const lastUpdate = latestUpdateTime(mirror)
  const authHelp: AuthHelp = authMode === 'local_credentials'
    ? {
        title: tx('本机 Git 凭证', 'Local Git credentials'),
        intro: tx(
          '仅本机模式适用。Vola 会复用这台机器已有的 Git SSH key 或 credential helper，适合你已经在本机配置好 GitHub 推送权限的情况。',
          'Available in local mode only. Vola reuses this machine’s Git SSH key or credential helper, which is best when GitHub push access is already configured locally.',
        ),
      }
    : authMode === 'github_token'
      ? {
          title: tx('GitHub Token', 'GitHub token'),
          intro: tx(
            '适合你想自己创建备份仓库，并把权限精确控制在某一个仓库里的情况。',
            'Use this when you want to create the backup repository yourself and keep the token scoped to that single repository.',
          ),
          steps: [
            tx('先在 GitHub 新建一个私有仓库，例如 vola-backup。', 'Create a private GitHub repository first, for example vola-backup.'),
            tx('进入 GitHub Settings → Developer settings → Personal access tokens → Fine-grained tokens，创建新 token。', 'Go to GitHub Settings → Developer settings → Personal access tokens → Fine-grained tokens, then generate a new token.'),
            tx('Repository access 只选择这个备份仓库。', 'For Repository access, select only this backup repository.'),
            tx('Repository permissions 里给 Contents 读写权限；Metadata 保持默认只读。', 'For Repository permissions, set Contents to Read and write; keep Metadata at the default read-only access.'),
            tx('回到这里填写仓库 URL 和 token，先测试 token，通过后保存。', 'Come back here, enter the repository URL and token, test the token, then save after it passes.'),
          ],
          footer: tx(
            'Token 只显示一次，Vola 会加密保存，不会在页面回显原值。',
            'GitHub only shows the token once. Vola stores it encrypted and will not show the raw value again.',
          ),
        }
      : {
          title: tx('GitHub App 授权', 'GitHub App authorization'),
          intro: tx(
            '推荐的一键方式。连接 GitHub 后，Vola 会自动创建或复用你的私有 vola-backup 仓库，不需要你手动保存 token。',
            'Recommended for the easiest setup. After you connect GitHub, Vola creates or reuses your private vola-backup repository without asking you to manage a token.',
          ),
          steps: [
            tx('点击连接 GitHub，在 GitHub 授权页批准 Vola App。', 'Click Connect GitHub and approve the Vola App on GitHub.'),
            tx('回到 Vola 后点击创建私有备份仓库。', 'Back in Vola, click Create private backup repo.'),
            tx('之后点击立即同步，后台会把 Vola 数据推送到这个仓库。', 'Then click Sync now; the worker pushes your Vola data to that repository.'),
          ],
          footer: tx(
            '如果 GitHub 提示批准新权限，批准后再回到这里重试。',
            'If GitHub asks you to approve updated permissions, approve them and then retry here.',
          ),
        }
  const retrySeconds = useMemo(() => {
    if (!syncRetryUntil) return 0
    return Math.max(0, Math.ceil((syncRetryUntil - nowTick) / 1000))
  }, [nowTick, syncRetryUntil])
  const syncCooldownSeconds = Math.max(0, Number(publicConfig?.git_mirror_manual_sync_cooldown_seconds || 0))
  const selectedAuthCanSync = authMode === 'github_app_user'
    ? !!mirror?.github_app_user_connected
    : authMode === 'github_token'
      ? !!mirror?.github_token_configured
      : true
  const syncDisabled = working || retrySeconds > 0 || !remoteRepoURL
  const destinationConfigured = !!remoteRepoURL
  const syncAvailable = destinationConfigured && selectedAuthCanSync
  const tokenModeNeedsTokenBeforeSave = authMode === 'github_token' && !mirror?.github_token_configured && !tokenInput.trim()
  const remotePlaceholder = authMode === 'local_credentials'
    ? 'git@github.com:owner/vola-backup.git'
    : 'https://github.com/owner/vola-backup.git'
  const storageLabel = publicConfig?.storage === 'postgres'
    ? tx('Postgres 数据库', 'Postgres database')
    : publicConfig?.storage === 'sqlite'
      ? tx('SQLite 本地数据库', 'SQLite local database')
      : tx('服务端存储', 'Server storage')
  const storageDetail = publicConfig?.local_mode
    ? tx('本地模式会把 Hub 内容写入这台机器的 SQLite 数据库。', 'Local mode stores Hub content in this machine’s SQLite database.')
    : tx('Hosted 模式会把 Hub 内容写入服务端数据库；外部备份需要单独配置。', 'Hosted mode stores Hub content in the server database; external backup must be configured separately.')
  const mirrorPathLabel = mirror?.path || (isLocalExecution
    ? tx('还没有创建本地 Git mirror', 'No local Git mirror yet')
    : tx('Hosted Git working tree', 'Hosted Git working tree'))
  const backupRepoLabel = githubRepoLabel(remoteRepoURL)
  const backupRepoLink = githubRepoHref(remoteRepoURL)
  const latestBackupLabel = mirror?.last_push_at
    ? formatDateTime(mirror.last_push_at, locale)
    : mirror?.last_synced_at
      ? tx(`已写入 Git mirror：${formatDateTime(mirror.last_synced_at, locale)}`, `Written to Git mirror: ${formatDateTime(mirror.last_synced_at, locale)}`)
      : tx('还没有备份记录', 'No backup record yet')
  const externalTargetsConfigured = backupTargets.length
  const externalTargetsWithBackup = backupTargets.filter((target) => target.last_backup_at).length
  const remoteBackupSummary = remoteRepoURL
    ? backupRepoLabel || remoteRepoURL
    : externalTargetsConfigured > 0
      ? tx(`${externalTargetsConfigured} 个外部目标`, `${externalTargetsConfigured} external target${externalTargetsConfigured === 1 ? '' : 's'}`)
      : tx('尚未配置', 'Not configured')
  const opsStatusLabel = opsStatus?.status === 'ok'
    ? tx('备份状态正常', 'Backup status OK')
    : opsStatus?.status === 'critical'
      ? tx('有同步或备份错误', 'Sync or backup error')
      : tx('备份还需要配置或执行', 'Backup needs setup or a first run')
  const opsInstanceStatusLabel = opsInstanceStatus?.status === 'ok'
    ? tx('实例备份状态正常', 'Instance backup status OK')
    : opsInstanceStatus?.status === 'critical'
      ? tx('实例里有同步或备份错误', 'Instance has a sync or backup error')
      : tx('实例备份还需要配置或执行', 'Instance backup needs setup or a first run')
  const latestOpsBackupLabel = opsInstanceStatus?.latest_external_backup_at
    ? formatDateTime(opsInstanceStatus.latest_external_backup_at, locale)
    : opsStatus?.backup.last_successful_backup_at
      ? formatDateTime(opsStatus.backup.last_successful_backup_at, locale)
    : tx('还没有外部上传记录', 'No external upload record yet')
  const latestGitOpsLabel = opsInstanceStatus?.latest_git_push_at
    ? formatDateTime(opsInstanceStatus.latest_git_push_at, locale)
    : opsStatus?.git_mirror.last_push_at
    ? formatDateTime(opsStatus.git_mirror.last_push_at, locale)
    : opsStatus?.git_mirror.last_synced_at
      ? formatDateTime(opsStatus.git_mirror.last_synced_at, locale)
      : tx('还没有 Git 同步记录', 'No Git sync record yet')
  const latestFailedBackupRun = backupRuns.find((run) => run.status === 'failed')
  const githubBackupReady = !!remoteRepoURL && selectedAuthCanSync && !mirror?.remote_conflict
  const backupFocusTone = mirror?.remote_conflict
    ? 'warn'
    : githubBackupReady
      ? 'good'
      : 'warn'
  const backupFocusTitle = mirror?.remote_conflict
    ? tx('远端仓库需要确认', 'Remote repository needs review')
    : githubBackupReady
      ? tx('GitHub 备份已经可用', 'GitHub Backup is ready')
      : tx('先完成 GitHub 备份', 'Set up GitHub Backup first')
  const backupFocusCopy = mirror?.remote_conflict
    ? tx('远端仓库出现 Vola 之外的新提交。请在下方 GitHub 备份目标里确认处理方式。', 'The remote repository has commits outside Vola. Review the GitHub backup destination below before syncing.')
    : githubBackupReady
      ? tx('你可以立即同步当前资料；需要离开服务器的 zip 备份包时，再添加 WebDAV 或 COS/S3 目标。', 'You can sync now. Add WebDAV or S3-compatible archive backup targets when you need off-server zip archives.')
      : authMode === 'github_app_user'
        ? tx('推荐使用 GitHub App：连接 GitHub，创建私有备份仓库，然后立即同步。', 'Recommended path: connect GitHub, create a private backup repository, then sync.')
        : tx('填写备份仓库和认证信息后，Vola 会把资料写入 GitHub 版本历史。', 'After repository and credentials are saved, Vola writes your data into GitHub version history.')
  const githubStepValue = remoteRepoURL
    ? tx('仓库已配置', 'Repository set')
    : authMode === 'github_app_user' && mirror?.github_app_user_connected
      ? tx('GitHub 已连接', 'GitHub connected')
      : tx('等待配置', 'Waiting for setup')
  const archiveStepValue = backupTargets.length > 0
    ? tx(`${backupTargets.length} 个目标`, `${backupTargets.length} target${backupTargets.length === 1 ? '' : 's'}`)
    : tx('可选', 'Optional')
  const recoveryStepValue = restorePreview
    ? tx('已有预览结果', 'Preview ready')
    : tx('从 zip 备份包恢复', 'Restore from zip archive')

  const setInlineMessage = (nextMessage: string) => {
    setMessage(nextMessage)
    setError('')
    setGithubAppError('')
  }

  const setInlineError = (nextError: string) => {
    setError(nextError)
    setMessage('')
  }

  const refreshOpsStatus = async () => {
    const [next, instanceNext] = await Promise.all([
      api.getOpsStatus().catch(() => null as OpsStatus | null),
      api.getOpsInstanceStatus().catch(() => null as OpsInstanceStatus | null),
    ])
    setOpsStatus(next)
    setOpsInstanceStatus(instanceNext)
  }

  const refreshBackupRuns = async () => {
    const runs = await api.listBackupRuns(20).catch(() => [] as BackupRun[])
    setBackupRuns(runs)
  }

  const hostedSyncSettledMessage = (settings: GitMirrorSettings) => {
    if (settings.sync_state === 'error' || settings.remote_conflict || settings.last_error || settings.last_push_error) {
      return settings.message || tx('最近一次后台同步需要处理。', 'The latest background sync needs attention.')
    }
    const requestedAt = Date.parse(settings.sync_requested_at || '')
    const lastSyncedAt = Date.parse(settings.last_synced_at || '')
    const lastCommitAt = Date.parse(settings.last_commit_at || '')
    const requestCompleted = Number.isFinite(requestedAt) && Number.isFinite(lastSyncedAt) && lastSyncedAt >= requestedAt
    const noNewCommit = requestCompleted && (!Number.isFinite(lastCommitAt) || lastCommitAt < requestedAt)
    if (noNewCommit) {
      return tx(
        '已检查，没有新的变更需要提交；备份仓库已是最新状态。',
        'Checked: no new changes to commit; the backup repository is up to date.',
      )
    }
    return settings.message || tx('后台同步已完成。', 'Background sync completed.')
  }

  const pollHostedSyncUntilSettled = async () => {
    for (let attempt = 0; attempt < 12; attempt += 1) {
      await delay(1500)
      const next = await api.getGitMirror()
      setMirror(next)
      const state = next.sync_state || 'idle'
      if (state !== 'queued' && state !== 'running') {
        setInlineMessage(hostedSyncSettledMessage(next))
        return
      }
    }
  }

  const handleConnectGitHubApp = async () => {
    setError('')
    setMessage('')
    setGithubAppError('')
    if (!githubAppConfigured) {
      setGithubAppError(githubAppUnavailableMessage)
      return
    }
    setWorking(true)
    try {
      const result = await api.startGitMirrorGitHubAppBrowser(window.location.pathname)
      window.location.assign(result.authorization_url)
    } catch (err: any) {
      setGithubAppError(err.message || tx('启动 GitHub 授权失败', 'Failed to start GitHub authorization'))
    } finally {
      setWorking(false)
    }
  }

  const handleCreateDefaultBackupRepo = async () => {
    setError('')
    setMessage('')
    setGithubAppError('')
    if (!githubAppConfigured) {
      setGithubAppError(githubAppUnavailableMessage)
      return
    }
    setWorking(true)
    try {
      const result = await api.createDefaultGitMirrorGitHubAppBackupRepo()
      setMirror(result.settings)
      setAuthMode(result.settings.auth_mode || 'github_app_user')
      setRemoteURL(result.settings.remote_url || '')
      setUrlEditing(false)
      setGithubAppReauthRequired(false)
      setInlineMessage(tx('备份仓库已准备好。', 'Backup repository is ready.'))
      void refreshOpsStatus()
    } catch (err: any) {
      if (isGitHubAppPermissionUpdateRequiredError(err)) {
        await loadPage()
        setAuthMode('github_app_user')
        setGithubAppReauthRequired(true)
        setGithubAppError(tx(
          'GitHub App 权限已更新，我们已自动断开旧的 GitHub Backup 授权。请先在 GitHub 批准新的 Repository Administration 读写权限，然后回到这里重新连接。',
          'GitHub App permissions were updated, so we disconnected the old GitHub Backup authorization. Approve the new Repository Administration read/write permission on GitHub, then reconnect here.',
        ))
        return
      }
      setInlineError(err.message || tx('创建备份仓库失败', 'Failed to create the backup repository'))
    } finally {
      setWorking(false)
    }
  }

  const handleDisconnectGitHubApp = async () => {
    setWorking(true)
    setError('')
    setMessage('')
    setGithubAppError('')
    try {
      await api.disconnectGitMirrorGitHubAppUser()
      await loadPage()
      setGithubAppReauthRequired(false)
      setInlineMessage(tx('GitHub 已断开连接', 'GitHub disconnected'))
    } catch (err: any) {
      setInlineError(err.message || tx('断开 GitHub 失败', 'Failed to disconnect GitHub'))
    } finally {
      setWorking(false)
    }
  }

  const handleSaveLocalDestination = async () => {
    if (!remoteURL.trim()) {
      setInlineError(tx('请填写 GitHub 仓库 URL。', 'Enter a GitHub repository URL.'))
      return
    }
    if (authMode === 'local_credentials' && isGitHubHTTPSURL(remoteURL)) {
      setInlineError(tx(
        '本机 Git 凭证模式请填写 git@github.com:owner/repo.git 形式的仓库 URL。',
        'Local Git credentials require a repository URL in the form git@github.com:owner/repo.git.',
      ))
      setUrlEditing(true)
      return
    }
    setWorking(true)
    setError('')
    setMessage('')
    try {
      const saved = await api.updateGitMirror({
        auto_commit_enabled: true,
        auto_push_enabled: true,
        auth_mode: authMode,
        remote_name: DEFAULT_REMOTE_NAME,
        remote_url: remoteURL.trim(),
        remote_branch: DEFAULT_REMOTE_BRANCH,
        github_token: authMode === 'github_token' ? tokenInput.trim() || undefined : undefined,
      })
      setMirror(saved)
      setRemoteURL(saved.remote_url || '')
      setUrlEditing(false)
      setTokenInput('')
      if (authMode === 'github_token') {
        setTokenEditing(!saved.github_token_configured)
      }
      setTokenTest(null)
      setInlineMessage(tx('备份目标已保存。', 'Backup destination saved.'))
      void refreshOpsStatus()
    } catch (err: any) {
      setInlineError(err.message || tx('保存备份目标失败', 'Failed to save the backup destination'))
    } finally {
      setWorking(false)
    }
  }

  const handleTestToken = async () => {
    setTesting(true)
    setError('')
    setMessage('')
    try {
      const result = await api.testGitMirrorGitHubTokenGeneric({
        remote_url: remoteURL.trim(),
        github_token: tokenInput.trim(),
      })
      setTokenTest(result)
      if (result.normalized_remote_url) {
        setRemoteURL(result.normalized_remote_url)
      }
    } catch (err: any) {
      setInlineError(err.message || tx('GitHub token 测试失败', 'Failed to test GitHub token'))
    } finally {
      setTesting(false)
    }
  }

  const handleSaveToken = async () => {
    if (!remoteURL.trim()) {
      setInlineError(tx('请先填写 GitHub 仓库 URL。', 'Enter the GitHub repository URL first.'))
      return
    }
    if (!tokenInput.trim()) {
      setInlineError(tx('请填写 GitHub token。', 'Enter a GitHub token.'))
      return
    }
    setWorking(true)
    setError('')
    setMessage('')
    try {
      const saved = await api.updateGitMirror({
        auto_commit_enabled: true,
        auto_push_enabled: true,
        auth_mode: 'github_token',
        remote_name: DEFAULT_REMOTE_NAME,
        remote_url: remoteURL.trim(),
        remote_branch: DEFAULT_REMOTE_BRANCH,
        github_token: tokenInput.trim(),
      })
      setMirror(saved)
      setRemoteURL(saved.remote_url || remoteURL.trim())
      setTokenInput('')
      setTokenEditing(!saved.github_token_configured)
      setTokenTest(null)
      setInlineMessage(tx('GitHub token 已保存。', 'GitHub token saved.'))
      void refreshOpsStatus()
    } catch (err: any) {
      setInlineError(err.message || tx('保存 GitHub token 失败', 'Failed to save GitHub token'))
    } finally {
      setWorking(false)
    }
  }

  const handleClearToken = async () => {
    setWorking(true)
    setError('')
    setMessage('')
    try {
      const saved = await api.updateGitMirror({
        auto_commit_enabled: true,
        auto_push_enabled: false,
        auth_mode: 'github_token',
        remote_name: DEFAULT_REMOTE_NAME,
        remote_url: remoteURL.trim(),
        remote_branch: DEFAULT_REMOTE_BRANCH,
        clear_github_token: true,
      })
      setMirror(saved)
      setTokenInput('')
      setTokenEditing(true)
      setTokenTest(null)
      setInlineMessage(tx('已清除保存的 GitHub token。', 'Saved GitHub token was cleared.'))
      void refreshOpsStatus()
    } catch (err: any) {
      setInlineError(err.message || tx('清除 GitHub token 失败', 'Failed to clear GitHub token'))
    } finally {
      setWorking(false)
    }
  }

  const handleSync = async (forceRemoteOverwrite = false) => {
    if (retrySeconds > 0) {
      setInlineError(tx(`请 ${retrySeconds} 秒后再同步。`, `Try syncing again in ${retrySeconds} seconds.`))
      return
    }
    setWorking(true)
    setError('')
    setMessage('')
    try {
      const result = await api.syncGitMirror({ force_remote_overwrite: forceRemoteOverwrite || undefined })
      await loadPage()
      setSyncRetryUntil(syncCooldownSeconds > 0 ? Date.now() + syncCooldownSeconds * 1000 : 0)
      const queuedHostedSync = result.execution_mode === 'hosted' && (result.sync_state === 'queued' || result.sync_state === 'running')
      setInlineMessage(queuedHostedSync
        ? tx('同步请求已提交，后台正在处理。完成后页面会自动更新状态。', 'Sync request submitted. The background worker is processing it, and this page will update when it finishes.')
        : result.message || tx('已触发同步。', 'Sync started.'))
      if (queuedHostedSync) {
        void pollHostedSyncUntilSettled().catch(() => undefined)
      }
    } catch (err: any) {
      if (err?.code === 'rate_limit_exceeded' && err.retry_after_sec) {
        setSyncRetryUntil(Date.now() + Number(err.retry_after_sec) * 1000)
      }
      setInlineError(err.message || tx('触发同步失败', 'Failed to start sync'))
    } finally {
      setWorking(false)
    }
  }

  const updateBackupDraft = (patch: Partial<SaveBackupTargetRequest>) => {
    setBackupDraft((current) => ({ ...current, ...patch }))
    setBackupError('')
    setBackupMessage('')
    setBackupResult(null)
  }

  const handleBackupKindChange = (kind: BackupTargetKind) => {
    updateBackupDraft({
      kind,
      name: kind === 'webdav' ? 'WebDAV backup' : 'S3-compatible backup',
      s3_path_style: true,
    })
  }

  const handleSaveBackupTarget = async () => {
    setBackupSaving(true)
    setBackupError('')
    setBackupMessage('')
    setBackupResult(null)
    try {
      const saved = await api.saveBackupTarget({
        ...backupDraft,
        enabled: true,
      })
      setBackupTargets((targets) => {
        const exists = targets.some((target) => target.id === saved.id)
        if (exists) {
          return targets.map((target) => target.id === saved.id ? saved : target)
        }
        return [...targets, saved]
      })
      setBackupDraft(defaultBackupDraft())
      setBackupMessage(tx('外部备份目标已保存。', 'External backup target saved.'))
      void refreshOpsStatus()
      void refreshBackupRuns()
    } catch (err: any) {
      setBackupError(err.message || tx('保存外部备份目标失败', 'Failed to save external backup target'))
    } finally {
      setBackupSaving(false)
    }
  }

  const handleRunBackupTarget = async (target: BackupTarget) => {
    setBackupRunningID(target.id)
    setBackupError('')
    setBackupMessage('')
    setBackupResult(null)
    try {
      const result = await api.runBackupTarget(target.id)
      setBackupResult(result)
      setBackupTargets((targets) => targets.map((item) => item.id === result.target.id ? result.target : item))
      setBackupMessage(tx('备份包已上传到外部目标。', 'Backup archive uploaded to the external target.'))
      void refreshOpsStatus()
      void refreshBackupRuns()
    } catch (err: any) {
      setBackupError(err.message || tx('上传备份包失败', 'Failed to upload the backup archive'))
      const targets = await api.listBackupTargets().catch(() => [] as BackupTarget[])
      setBackupTargets(targets)
      void refreshOpsStatus()
      void refreshBackupRuns()
    } finally {
      setBackupRunningID('')
    }
  }

  const handlePreviewBackupRestore = async () => {
    if (!restoreFile) {
      setRestoreError(tx('请选择一个备份 zip。', 'Choose a backup zip first.'))
      return
    }
    setRestorePreviewing(true)
    setRestoreError('')
    setRestorePreview(null)
    try {
      const preview = await api.previewBackupRestore(restoreFile)
      setRestorePreview(preview)
      setRestoreApplyResult(null)
    } catch (err: any) {
      setRestoreError(err.message || tx('读取备份包失败', 'Failed to read the backup archive'))
    } finally {
      setRestorePreviewing(false)
    }
  }

  const handleApplyBackupRestore = async () => {
    if (!restoreFile) {
      setRestoreError(tx('请选择一个备份 zip。', 'Choose a backup zip first.'))
      return
    }
    setRestoreApplying(true)
    setRestoreError('')
    setRestoreApplyResult(null)
    try {
      const result = await api.applyBackupRestore(restoreFile, restoreMode)
      setRestoreApplyResult(result)
      setBackupMessage(tx('备份包恢复已完成。', 'Backup restore completed.'))
    } catch (err: any) {
      setRestoreError(err.message || tx('应用恢复失败', 'Failed to apply restore'))
    } finally {
      setRestoreApplying(false)
    }
  }

  const backupKindLabel = (kind: BackupTargetKind | string) => kind === 'webdav'
    ? 'WebDAV'
    : 'S3-compatible / OSS / R2'

  const backupTargetLocation = (target: BackupTarget) => {
    if (target.kind === 'webdav') {
      return target.webdav_url || ''
    }
    const endpoint = target.s3_endpoint || ''
    const bucket = target.s3_bucket || ''
    const prefix = target.s3_prefix ? `/${target.s3_prefix}` : ''
    return bucket ? `${endpoint}/${bucket}${prefix}` : endpoint
  }

  const restoreCategoryLabel = (id: string, fallback: string) => {
    switch (id) {
      case 'identity':
        return tx('身份资料', 'Identity')
      case 'vault':
        return tx('Vault', 'Vault')
      case 'skills':
        return tx('Skills', 'Skills')
      case 'memory_profile':
        return tx('记忆资料', 'Memory profile')
      case 'projects':
        return tx('Projects', 'Projects')
      case 'scratch':
        return tx('Scratch', 'Scratch')
      case 'roles':
        return tx('Roles', 'Roles')
      case 'inbox':
        return tx('Inbox', 'Inbox')
      default:
        return fallback
    }
  }

  const renderStatus = () => (
    <div className="git-mirror-status-line">
      <span>
        {tx('最近更新时间：', 'Last update: ')}
        {lastUpdate ? formatDateTime(lastUpdate, locale) : tx('还没有同步', 'Not synced yet')}
      </span>
    </div>
  )

  const renderStorageOverview = () => (
    <div className="materials-panel data-sync-card">
      <div className="card-header">
        <h3 className="card-title">{tx('数据位置与恢复', 'Data location and recovery')}</h3>
      </div>
      <p className="data-record-secondary">
        {tx('Vola 会先把 Hub 数据写入主存储，再按配置写入 Git mirror，并推送到远端仓库作为离开当前机器或服务器的备份。', 'Vola writes Hub data to primary storage first, then mirrors it into Git and pushes to a remote repository as the off-machine or off-server backup.')}
      </p>
      <div className="data-sync-status-grid">
        <div className="data-sync-status-card">
          <div className="data-record-title">{tx('主存储', 'Primary storage')}</div>
          <div className="data-record-secondary">{storageLabel}</div>
          <div className="data-record-secondary">{storageDetail}</div>
        </div>
        <div className="data-sync-status-card">
          <div className="data-record-title">{tx('Git 工作目录', 'Git working tree')}</div>
          <div className="data-record-secondary"><code>{mirrorPathLabel}</code></div>
          <div className="data-record-secondary">
            {tx('这里保存可见文件树的 Git 版本历史，是推送到 GitHub 前的工作副本。', 'This holds Git version history for the visible file tree before it is pushed to GitHub.')}
          </div>
        </div>
        <div className="data-sync-status-card">
          <div className="data-record-title">{tx('远端备份', 'Remote backup')}</div>
          <div className="data-record-secondary">
            {remoteRepoURL ? (
              <a href={backupRepoLink} target="_blank" rel="noreferrer">{backupRepoLabel || remoteRepoURL}</a>
            ) : (
              remoteBackupSummary
            )}
          </div>
          <div className="data-record-secondary">
            {remoteRepoURL
              ? <>{tx('最近备份：', 'Latest backup: ')}{latestBackupLabel}</>
              : <>{tx('外部目标最近上传：', 'External target uploads: ')}{externalTargetsWithBackup > 0 ? tx(`${externalTargetsWithBackup} 个已有记录`, `${externalTargetsWithBackup} recorded`) : tx('还没有上传记录', 'No upload record yet')}</>}
          </div>
        </div>
        {(opsInstanceStatus || opsStatus) && (
          <div className="data-sync-status-card">
            <div className="data-record-title">{tx('生产状态', 'Production status')}</div>
            <div className="data-record-secondary">{opsInstanceStatus ? opsInstanceStatusLabel : opsStatusLabel}</div>
            {opsInstanceStatus && (
              <div className="data-record-secondary">
                {tx(
                  `实例用户：${opsInstanceStatus.users_with_remote_backup_artifact}/${opsInstanceStatus.users_total} 个已有远端备份记录`,
                  `Instance users: ${opsInstanceStatus.users_with_remote_backup_artifact}/${opsInstanceStatus.users_total} have remote backup records`,
                )}
              </div>
            )}
            <div className="data-record-secondary">
              {tx('Git 最近记录：', 'Latest Git record: ')}{latestGitOpsLabel}
            </div>
            <div className="data-record-secondary">
              {opsInstanceStatus
                ? tx(
                    `外部备份用户：${opsInstanceStatus.users_with_external_backup}/${opsInstanceStatus.users_total} · ${latestOpsBackupLabel}`,
                    `External backup users: ${opsInstanceStatus.users_with_external_backup}/${opsInstanceStatus.users_total} · ${latestOpsBackupLabel}`,
                  )
                : <>{tx('外部目标：', 'External targets: ')}{opsStatus?.backup.enabled_targets}/{opsStatus?.backup.targets_configured} · {latestOpsBackupLabel}</>}
            </div>
            {opsInstanceStatus && opsStatus && opsStatus.status !== opsInstanceStatus.status && (
              <div className="data-record-secondary">
                {tx(
                  `当前用户状态：${opsStatus.status}`,
                  `Current user status: ${opsStatus.status}`,
                )}
              </div>
            )}
          </div>
        )}
        <div className="data-sync-status-card">
          <div className="data-record-title">{tx('恢复说明', 'Recovery')}</div>
          <div className="data-record-secondary">
            {remoteRepoURL
              ? tx('需要恢复时，可以先 clone 这个仓库检查 skills、memory 和 project 文件；secret 与账号内部元数据需要从数据库或配置恢复。', 'For recovery, clone this repository to inspect skills, memory, and project files. Secrets and internal account metadata must be recovered from the database or service configuration.')
              : tx('配置远端仓库后，这里会显示可用于恢复的仓库位置。', 'After a remote repository is configured, this page shows the repository to use for recovery.')}
          </div>
        </div>
      </div>
      {!remoteRepoURL && (
        <div className="alert alert-warn" style={{ marginTop: 16 }}>
          {tx('当前还没有远端备份。服务器或本机数据丢失时，只能依赖数据库备份或手工导出的文件。', 'No remote backup is configured. If the server or local machine loses data, recovery depends on database backups or manually exported files.')}
        </div>
      )}
      <p className="data-record-secondary">
        {tx('GitHub 保存版本历史；WebDAV、S3-compatible、OSS、R2 会上传 Vola 导出 zip，适合做离开服务器的备份包。', 'GitHub keeps version history; WebDAV, S3-compatible, OSS, and R2 upload a Vola export zip as an off-server backup archive.')}
      </p>
    </div>
  )

  const renderFeedback = () => {
    if (error) {
      return <div className="alert alert-warn git-mirror-inline-feedback">{error}</div>
    }
    if (message) {
      return <div className="alert alert-ok git-mirror-inline-feedback">{message}</div>
    }
    return null
  }

  const renderPrimaryFocusAction = () => {
    if (mirror?.remote_conflict) {
      return (
        <a className="btn btn-primary" href="#github-backup">
          {tx('查看 GitHub 备份目标', 'Review GitHub destination')}
        </a>
      )
    }
    if (githubBackupReady) {
      return (
        <button className="btn btn-primary" type="button" disabled={syncDisabled} onClick={() => void handleSync(false)}>
          {working ? tx('处理中...', 'Working...') : retrySeconds > 0 ? tx(`请稍候 ${retrySeconds}s`, `Wait ${retrySeconds}s`) : tx('立即同步', 'Sync now')}
        </button>
      )
    }
    if (authMode === 'github_app_user' && !mirror?.github_app_user_connected) {
      return (
        <button className="btn btn-primary" type="button" disabled={working} onClick={handleConnectGitHubApp}>
          {working ? tx('连接中...', 'Connecting...') : tx('连接 GitHub', 'Connect GitHub')}
        </button>
      )
    }
    if (authMode === 'github_app_user' && mirror?.github_app_user_connected && !remoteRepoURL) {
      return (
        <button className="btn btn-primary" type="button" disabled={working} onClick={handleCreateDefaultBackupRepo}>
          {working ? tx('创建中...', 'Creating...') : tx('创建私有备份仓库', 'Create private backup repo')}
        </button>
      )
    }
    return (
      <a className="btn btn-primary" href="#github-backup">
        {tx('填写备份仓库', 'Enter backup repository')}
      </a>
    )
  }

  const renderBackupFocus = () => (
    <section className={`git-backup-focus-panel is-${backupFocusTone}`} aria-label={tx('备份重点', 'Backup focus')}>
      <div className="git-backup-focus-copy">
        <span>{tx('备份状态', 'Backup status')}</span>
        <h2>{backupFocusTitle}</h2>
        <p>{backupFocusCopy}</p>
        <div className="git-backup-focus-actions">
          {renderPrimaryFocusAction()}
          <a className="btn" href="#external-backup">
            {backupTargets.length > 0 ? tx('查看外部备份包', 'View archive backups') : tx('配置外部备份包', 'Set archive backup')}
          </a>
          <a className="btn" href="#restore-preview">
            {tx('恢复预览', 'Restore preview')}
          </a>
        </div>
      </div>
      <div className="git-backup-focus-status">
        <div className="git-backup-status-item">
          <span>GitHub</span>
          <strong>{githubStepValue}</strong>
          <small>{remoteRepoURL ? latestBackupLabel : tx('推荐先完成', 'Recommended first')}</small>
        </div>
        <div className="git-backup-status-item">
          <span>{tx('外部备份包', 'Archive backups')}</span>
          <strong>{archiveStepValue}</strong>
          <small>{latestOpsBackupLabel}</small>
        </div>
        <div className="git-backup-status-item">
          <span>{tx('恢复', 'Recovery')}</span>
          <strong>{recoveryStepValue}</strong>
          <small>{tx('不会自动覆盖，先预览再应用', 'Preview before applying changes')}</small>
        </div>
      </div>
    </section>
  )

  const renderRemoteConflict = () => mirror?.remote_conflict && (
    <div className="alert alert-warn" style={{ marginTop: 16 }}>
      {tx('远端仓库有 Vola 之外的新提交。普通同步已停止，确认后可以用 Vola 覆盖远端。', 'The remote repository has commits outside Vola. Normal sync is blocked; confirm overwrite to replace the remote with Vola.')}
      <div className="data-sync-actions data-sync-actions-compact" style={{ marginTop: 12 }}>
        <button className="btn btn-primary" type="button" disabled={working || retrySeconds > 0} onClick={() => void handleSync(true)}>
          {tx('用 Vola 覆盖远端', 'Overwrite remote with Vola')}
        </button>
      </div>
    </div>
  )

  const renderGitHubAppFlow = () => (
    <div className="data-sync-token-box" style={{ marginTop: 16 }}>
      <div className="data-record-title">{tx('GitHub App 授权', 'GitHub App authorization')}</div>
      <div className="data-sync-field-note">
        {tx('授权后 Vola 会创建或复用你的私有 vola-backup 仓库。', 'After authorization, Vola creates or reuses your private vola-backup repository.')}
      </div>
      {!githubAppConfigured && (
        <div className="alert alert-warn" style={{ marginTop: 12 }}>
          {githubAppUnavailableMessage}
        </div>
      )}
      {githubAppError && !githubAppReauthRequired && (
        <div className="alert alert-warn" style={{ marginTop: 12 }}>
          {githubAppError}
        </div>
      )}
      {githubAppReauthRequired && !mirror?.github_app_user_connected && (
        <div className="alert alert-warn" style={{ marginTop: 12 }}>
          <div className="data-record-title">{tx('需要重新批准 GitHub App 权限', 'GitHub App permissions need approval')}</div>
          {githubAppError && (
            <div className="data-record-secondary" style={{ marginTop: 6 }}>
              {githubAppError}
            </div>
          )}
          <div className="data-record-secondary" style={{ marginTop: 8 }}>
            {tx('我们已断开旧授权。点击下面按钮后，GitHub 会要求你批准新权限并重新授权。', 'The old authorization was disconnected. Click the button below to approve the new permissions on GitHub and reconnect.')}
          </div>
          <div className="data-sync-actions data-sync-actions-compact">
            <button
              className="btn btn-primary"
              type="button"
              disabled={working}
              onClick={handleConnectGitHubApp}
            >
              {working ? tx('连接中...', 'Connecting...') : tx('批准新权限并连接 GitHub', 'Approve permissions and connect GitHub')}
            </button>
          </div>
        </div>
      )}
      {!mirror?.github_app_user_connected ? (
        !githubAppReauthRequired && <div className="data-sync-actions data-sync-actions-compact">
          <button className="btn btn-primary" type="button" disabled={working} onClick={handleConnectGitHubApp}>
            {working ? tx('连接中...', 'Connecting...') : tx('连接 GitHub', 'Connect GitHub')}
          </button>
        </div>
      ) : (
        <>
          <div className="data-record-secondary" style={{ marginTop: 12 }}>
            GitHub {mirror.github_app_user_login || tx('已连接', 'Connected')}
          </div>
          {!remoteRepoURL && (
            <div className="data-sync-actions data-sync-actions-compact">
              <button className="btn btn-primary" type="button" disabled={working} onClick={handleCreateDefaultBackupRepo}>
                {working ? tx('创建中...', 'Creating...') : tx('创建私有备份仓库', 'Create private backup repo')}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )

  const renderLocalAuthControls = () => (
    <>
      <div className="data-sync-settings-grid git-mirror-auth-grid" style={{ marginTop: 16 }}>
        <div className="form-group git-mirror-auth-mode-field">
          <label htmlFor="git-mirror-auth-mode">{tx('认证方式', 'Auth mode')}</label>
          <div className="git-mirror-auth-control-row">
            <CustomSelect
              value={authMode}
              onChange={(val) => {
                const next = val as AuthMode
                setAuthMode(next)
                setError('')
                setMessage('')
                setGithubAppError('')
                if (next !== 'github_app_user') {
                  setGithubAppReauthRequired(false)
                }
                setTokenTest(null)
                setAuthHelpOpen(false)
                setUrlEditing(next !== 'github_app_user' && !remoteURL.trim())
                setTokenEditing(next === 'github_token' && !mirror?.github_token_configured)
              }}
              options={[
                ...(isLocalExecution ? [{ value: 'local_credentials', label: tx('本机 Git 凭证', 'Local Git credentials') }] : []),
                { value: 'github_token', label: tx('GitHub Token', 'GitHub token') },
                { value: 'github_app_user', label: tx('GitHub App 授权', 'GitHub App authorization') },
              ]}
              ariaLabel={tx('认证方式', 'Auth mode')}
            />
            <div className="git-mirror-auth-help">
              <button
                className="git-mirror-help-button"
                type="button"
                aria-label={tx('查看认证方式说明', 'Show auth mode help')}
                aria-expanded={authHelpOpen}
                onClick={() => setAuthHelpOpen((open) => !open)}
              >
                <InfoIcon />
              </button>
              {authHelpOpen && (
                <div className="git-mirror-help-popover" role="dialog">
                  <div className="data-record-title">{authHelp.title}</div>
                  <div className="data-record-secondary">{authHelp.intro}</div>
                  {authHelp.steps && (
                    <ol className="git-mirror-help-steps">
                      {authHelp.steps.map((step) => (
                        <li key={step}>{step}</li>
                      ))}
                    </ol>
                  )}
                  {authHelp.footer && (
                    <div className="data-record-secondary git-mirror-help-footer">{authHelp.footer}</div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
        {authMode !== 'github_app_user' && (
          <div className="form-group data-sync-settings-span-wide git-mirror-repo-field">
            <label htmlFor="git-mirror-remote-url">{tx('仓库 URL', 'Repository URL')}</label>
            {urlEditing || !remoteURL.trim() ? (
              <div className="git-mirror-repo-edit-row">
                <input
                  id="git-mirror-remote-url"
                  value={remoteURL}
                  autoFocus={urlEditing}
                  onChange={(event) => {
                    setRemoteURL(event.target.value)
                    setTokenTest(null)
                    setError('')
                    setMessage('')
                  }}
                  placeholder={remotePlaceholder}
                />
                <button className="btn btn-primary" type="button" disabled={working || !remoteURL.trim() || tokenModeNeedsTokenBeforeSave} onClick={handleSaveLocalDestination}>
                  {working ? tx('保存中...', 'Saving...') : tx('保存备份目标', 'Save backup destination')}
                </button>
              </div>
            ) : (
              <div className="git-mirror-repo-read-row">
                <a href={githubRepoHref(remoteURL)} target="_blank" rel="noreferrer">
                  {remoteURL}
                </a>
                <button
                  className="git-mirror-icon-button"
                  type="button"
                  aria-label={tx('编辑仓库 URL', 'Edit repository URL')}
                  title={tx('编辑仓库 URL', 'Edit repository URL')}
                  onClick={() => setUrlEditing(true)}
                >
                  <PencilIcon />
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {renderFeedback()}

      {authMode === 'github_token' && (
        <div className="data-sync-token-box git-mirror-token-box" style={{ marginTop: 12 }}>
          <div className="data-record-title">{tx('GitHub Token', 'GitHub token')}</div>
          <div className="data-sync-field-note">
            {mirror?.github_token_configured
              ? tx('Token 已配置；输入新 token 后可替换。建议使用只授权这个备份仓库的 fine-grained token。', 'A token is configured; enter a new token to replace it. A fine-grained token scoped only to this backup repository is recommended.')
              : tx('建议使用 fine-grained token，只给这个备份仓库 Contents 读写权限。具体步骤见认证方式旁边的 ?。', 'Use a fine-grained token scoped only to this backup repository with Contents read/write access. See the ? next to Auth mode for the steps.')}
          </div>
          {tokenEditing || !mirror?.github_token_configured ? (
            <>
              <div className="git-mirror-token-edit-row">
                <input
                  className="data-sync-secret-input"
                  type="password"
                  value={tokenInput}
                  onChange={(event) => {
                    setTokenInput(event.target.value)
                    setTokenTest(null)
                    setError('')
                    setMessage('')
                  }}
                  placeholder="ghp_xxx"
                />
                <button className="btn btn-primary" type="button" disabled={working || !remoteURL.trim() || !tokenInput.trim()} onClick={handleSaveToken}>
                  {working ? tx('保存中...', 'Saving...') : tx('保存 token', 'Save token')}
                </button>
              </div>
              <div className="data-sync-actions data-sync-actions-compact">
                <button className="btn" type="button" disabled={testing || !remoteURL.trim() || !tokenInput.trim()} onClick={handleTestToken}>
                  {testing ? tx('测试中...', 'Testing...') : tx('测试 token', 'Test token')}
                </button>
                {mirror?.github_token_configured && (
                  <button
                    className="btn-text"
                    type="button"
                    disabled={working}
                    onClick={() => {
                      setTokenInput('')
                      setTokenTest(null)
                      setTokenEditing(false)
                    }}
                  >
                    {tx('取消', 'Cancel')}
                  </button>
                )}
              </div>
            </>
          ) : (
            <>
              <div className="git-mirror-repo-read-row git-mirror-token-read-row">
                <span>
                  {tx('Token 已配置', 'Token configured')}
                  {mirror?.github_token_login ? ` · ${mirror.github_token_login}` : ''}
                </span>
                <button
                  className="git-mirror-icon-button"
                  type="button"
                  aria-label={tx('编辑 GitHub token', 'Edit GitHub token')}
                  title={tx('编辑 GitHub token', 'Edit GitHub token')}
                  onClick={() => setTokenEditing(true)}
                >
                  <PencilIcon />
                </button>
              </div>
              <div className="data-sync-actions data-sync-actions-compact">
                <button className="btn" type="button" disabled={working} onClick={handleClearToken}>
                  {tx('清除已保存 token', 'Clear saved token')}
                </button>
              </div>
            </>
          )}
          {tokenTest && (
            <div className={tokenTest.ok ? 'alert alert-ok' : 'alert alert-warn'} style={{ marginTop: 12 }}>
              {localizeGitHubAccessMessage(tokenTest.message, locale)}
            </div>
          )}
        </div>
      )}

      {authMode === 'github_app_user' && renderGitHubAppFlow()}
    </>
  )

  const renderExternalBackupTargets = () => (
    <div id="external-backup" className="materials-panel data-sync-card">
      <div className="card-header">
        <h3 className="card-title">{tx('外部备份目标', 'External backup targets')}</h3>
      </div>
      <p className="data-record-secondary">
        {tx('这里会把 Vola 导出 zip 上传到 WebDAV 或 S3-compatible 存储。GitHub 适合版本历史；这些目标适合做离开服务器的备份包。', 'This uploads a Vola export zip to WebDAV or S3-compatible storage. GitHub is for version history; these targets are for off-server backup archives.')}
      </p>
      {latestFailedBackupRun && (
        <div className="alert alert-warn" style={{ marginTop: 12 }}>
          {tx('最近失败：', 'Latest failure: ')}
          {latestFailedBackupRun.target_name || backupKindLabel(latestFailedBackupRun.target_kind)}
          {' · '}
          {formatDateTime(latestFailedBackupRun.started_at, locale)}
          {latestFailedBackupRun.error ? ` · ${latestFailedBackupRun.error}` : ''}
        </div>
      )}

      {backupTargets.length > 0 && (
        <div className="data-sync-status-grid">
          {backupTargets.map((target) => (
            <div className="data-sync-status-card" key={target.id}>
              <div className="data-record-title">{target.name || backupKindLabel(target.kind)}</div>
              <div className="data-record-secondary">{backupKindLabel(target.kind)}</div>
              <div className="data-record-secondary"><code>{backupTargetLocation(target) || tx('未填写位置', 'No location')}</code></div>
              <div className="data-record-secondary">
                {target.last_backup_at
                  ? tx(`最近上传：${formatDateTime(target.last_backup_at, locale)}`, `Latest upload: ${formatDateTime(target.last_backup_at, locale)}`)
                  : tx('还没有上传记录', 'No upload record yet')}
              </div>
              <div className="data-record-secondary">
                {target.auto_backup_enabled
                  ? tx(`自动备份：每 ${target.auto_backup_interval_hours || 24} 小时`, `Auto backup: every ${target.auto_backup_interval_hours || 24} hours`)
                  : tx('自动备份：关闭', 'Auto backup: off')}
              </div>
              <div className="data-record-secondary">
                {target.retention_keep_last || target.retention_keep_days
                  ? tx(
                      `保留策略：${target.retention_keep_last ? `最近 ${target.retention_keep_last} 份` : ''}${target.retention_keep_last && target.retention_keep_days ? ' / ' : ''}${target.retention_keep_days ? `${target.retention_keep_days} 天` : ''}`,
                      `Retention: ${target.retention_keep_last ? `last ${target.retention_keep_last}` : ''}${target.retention_keep_last && target.retention_keep_days ? ' / ' : ''}${target.retention_keep_days ? `${target.retention_keep_days} days` : ''}`,
                    )
                  : tx('保留策略：不自动清理远端备份包', 'Retention: remote archive cleanup is off')}
              </div>
              {target.last_auto_backup_at && (
                <div className="data-record-secondary">
                  {tx(`最近自动执行：${formatDateTime(target.last_auto_backup_at, locale)}`, `Latest scheduled run: ${formatDateTime(target.last_auto_backup_at, locale)}`)}
                </div>
              )}
              {target.last_backup_object && (
                <div className="data-record-secondary">
                  {tx('对象：', 'Object: ')}<code>{target.last_backup_object}</code>
                </div>
              )}
              {target.last_backup_error && (
                <div className="data-record-secondary">{target.last_backup_error}</div>
              )}
              <div className="data-sync-actions data-sync-actions-compact">
                <button className="btn btn-primary" type="button" disabled={backupRunningID === target.id} onClick={() => void handleRunBackupTarget(target)}>
                  {backupRunningID === target.id ? tx('上传中...', 'Uploading...') : tx('上传当前备份包', 'Upload current backup')}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="data-sync-token-box" style={{ marginTop: 16 }}>
        <div className="data-record-title">{tx('新增备份目标', 'Add backup target')}</div>
        <div className="data-sync-settings-grid" style={{ marginTop: 12 }}>
          <div className="form-group">
            <label htmlFor="backup-target-kind">{tx('目标类型', 'Target type')}</label>
            <CustomSelect
              value={backupDraft.kind}
              onChange={(val) => handleBackupKindChange(val as BackupTargetKind)}
              options={[
                { value: 'webdav', label: 'WebDAV' },
                { value: 's3', label: 'S3-compatible / OSS / R2' },
              ]}
              ariaLabel={tx('目标类型', 'Target type')}
            />
          </div>
          <div className="form-group">
            <label htmlFor="backup-target-name">{tx('显示名称', 'Display name')}</label>
            <input
              id="backup-target-name"
              value={backupDraft.name}
              onChange={(event) => updateBackupDraft({ name: event.target.value })}
              placeholder={backupDraft.kind === 'webdav' ? 'WebDAV backup' : 'S3-compatible backup'}
            />
          </div>
        </div>
        <div className="data-sync-toggle-grid" style={{ marginTop: 12 }}>
          <label className="data-sync-toggle-card">
            <div className="data-sync-toggle-copy">
              <div className="data-sync-toggle-title">{tx('自动备份', 'Auto backup')}</div>
              <div className="data-sync-field-note">
                {tx('后台会按间隔上传新的 Vola 导出 zip。', 'The scheduler uploads a new Vola export zip at the chosen interval.')}
              </div>
            </div>
            <input
              type="checkbox"
              checked={!!backupDraft.auto_backup_enabled}
              onChange={(event) => updateBackupDraft({ auto_backup_enabled: event.target.checked })}
            />
          </label>
          <div className="form-group">
            <label htmlFor="backup-auto-interval">{tx('间隔小时', 'Interval hours')}</label>
            <input
              id="backup-auto-interval"
              type="number"
              min={1}
              max={720}
              value={backupDraft.auto_backup_interval_hours || 24}
              onChange={(event) => updateBackupDraft({ auto_backup_interval_hours: Number(event.target.value || 24) })}
              disabled={!backupDraft.auto_backup_enabled}
            />
          </div>
        </div>

        <div className="data-sync-settings-grid" style={{ marginTop: 12 }}>
          <div className="form-group">
            <label htmlFor="backup-retention-last">{tx('保留最近份数', 'Keep latest runs')}</label>
            <input
              id="backup-retention-last"
              type="number"
              min={0}
              max={365}
              value={backupDraft.retention_keep_last || 0}
              onChange={(event) => updateBackupDraft({ retention_keep_last: Number(event.target.value || 0) })}
            />
            <div className="data-sync-field-note">
              {tx('0 表示不按份数自动清理。只会处理 Vola 自己生成的备份包。', '0 disables count-based cleanup. Only Vola-generated archives are eligible.')}
            </div>
          </div>
          <div className="form-group">
            <label htmlFor="backup-retention-days">{tx('保留天数', 'Keep days')}</label>
            <input
              id="backup-retention-days"
              type="number"
              min={0}
              max={3650}
              value={backupDraft.retention_keep_days || 0}
              onChange={(event) => updateBackupDraft({ retention_keep_days: Number(event.target.value || 0) })}
            />
            <div className="data-sync-field-note">
              {tx('0 表示不按时间自动清理。最近成功备份不会被清理。', '0 disables age-based cleanup. The latest successful backup is never removed.')}
            </div>
          </div>
        </div>

        {backupDraft.kind === 'webdav' ? (
          <div className="data-sync-settings-grid data-sync-settings-grid-wide" style={{ marginTop: 12 }}>
            <div className="form-group data-sync-settings-span-wide">
              <label htmlFor="backup-webdav-url">{tx('WebDAV 目录 URL', 'WebDAV folder URL')}</label>
              <input
                id="backup-webdav-url"
                value={backupDraft.webdav_url || ''}
                onChange={(event) => updateBackupDraft({ webdav_url: event.target.value })}
                placeholder="https://dav.example.com/backup/vola"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-webdav-username">{tx('用户名', 'Username')}</label>
              <input
                id="backup-webdav-username"
                value={backupDraft.webdav_username || ''}
                onChange={(event) => updateBackupDraft({ webdav_username: event.target.value })}
                placeholder="email@example.com"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-webdav-password">{tx('密码或应用密码', 'Password or app password')}</label>
              <input
                id="backup-webdav-password"
                className="data-sync-secret-input"
                type="password"
                value={backupDraft.webdav_password || ''}
                onChange={(event) => updateBackupDraft({ webdav_password: event.target.value })}
                placeholder={tx('保存后不会回显', 'Not shown again after saving')}
              />
            </div>
          </div>
        ) : (
          <div className="data-sync-settings-grid data-sync-settings-grid-wide" style={{ marginTop: 12 }}>
            <div className="form-group">
              <label htmlFor="backup-s3-endpoint">{tx('Endpoint', 'Endpoint')}</label>
              <input
                id="backup-s3-endpoint"
                value={backupDraft.s3_endpoint || ''}
                onChange={(event) => updateBackupDraft({ s3_endpoint: event.target.value })}
                placeholder="https://account.r2.cloudflarestorage.com"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-s3-bucket">{tx('Bucket', 'Bucket')}</label>
              <input
                id="backup-s3-bucket"
                value={backupDraft.s3_bucket || ''}
                onChange={(event) => updateBackupDraft({ s3_bucket: event.target.value })}
                placeholder="vola-backup"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-s3-region">{tx('Region', 'Region')}</label>
              <input
                id="backup-s3-region"
                value={backupDraft.s3_region || ''}
                onChange={(event) => updateBackupDraft({ s3_region: event.target.value })}
                placeholder="auto / us-east-1 / oss-cn-hangzhou"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-s3-prefix">{tx('Prefix', 'Prefix')}</label>
              <input
                id="backup-s3-prefix"
                value={backupDraft.s3_prefix || ''}
                onChange={(event) => updateBackupDraft({ s3_prefix: event.target.value })}
                placeholder="vola"
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-s3-access-key">{tx('Access key ID', 'Access key ID')}</label>
              <input
                id="backup-s3-access-key"
                value={backupDraft.s3_access_key_id || ''}
                onChange={(event) => updateBackupDraft({ s3_access_key_id: event.target.value })}
                placeholder="AKIA..."
              />
            </div>
            <div className="form-group">
              <label htmlFor="backup-s3-secret-key">{tx('Secret access key', 'Secret access key')}</label>
              <input
                id="backup-s3-secret-key"
                className="data-sync-secret-input"
                type="password"
                value={backupDraft.s3_secret_access_key || ''}
                onChange={(event) => updateBackupDraft({ s3_secret_access_key: event.target.value })}
                placeholder={tx('保存后不会回显', 'Not shown again after saving')}
              />
            </div>
            <label className="data-sync-toggle-card data-sync-settings-span-wide">
              <div className="data-sync-toggle-copy">
                <div className="data-sync-toggle-title">{tx('Path-style URL', 'Path-style URL')}</div>
                <div className="data-sync-field-note">
                  {tx('默认使用 endpoint/bucket/object。关闭后使用 bucket.endpoint/object，适合要求 virtual-hosted style 的服务。', 'Uses endpoint/bucket/object by default. Turn it off for bucket.endpoint/object when a provider requires virtual-hosted style.')}
                </div>
              </div>
              <input
                type="checkbox"
                checked={backupDraft.s3_path_style !== false}
                onChange={(event) => updateBackupDraft({ s3_path_style: event.target.checked })}
              />
            </label>
          </div>
        )}

        <div className="data-sync-actions data-sync-actions-compact">
          <button className="btn btn-primary" type="button" disabled={backupSaving} onClick={() => void handleSaveBackupTarget()}>
            {backupSaving ? tx('保存中...', 'Saving...') : tx('保存外部备份目标', 'Save external backup target')}
          </button>
        </div>
        {backupError && <div className="alert alert-warn" style={{ marginTop: 12 }}>{backupError}</div>}
        {backupMessage && <div className="alert alert-ok" style={{ marginTop: 12 }}>{backupMessage}</div>}
        {backupResult && (
          <div className="data-record-secondary">
            {tx('最近上传位置：', 'Latest upload location: ')}<code>{backupResult.location}</code>
          </div>
        )}
      </div>

      {backupRuns.length > 0 && (
        <div className="data-sync-token-box" style={{ marginTop: 16 }}>
          <div className="data-record-title">{tx('备份历史', 'Backup history')}</div>
          <div className="data-sync-status-grid" style={{ marginTop: 12 }}>
            {backupRuns.slice(0, 6).map((run) => (
              <div className="data-sync-status-card" key={run.id}>
                <div className="data-record-title">{run.target_name || backupKindLabel(run.target_kind)}</div>
                <div className="data-record-secondary">
                  {run.trigger === 'auto' ? tx('自动备份', 'Auto backup') : tx('手动备份', 'Manual backup')} · {run.status === 'success' ? tx('成功', 'Success') : tx('失败', 'Failed')}
                </div>
                <div className="data-record-secondary">{formatDateTime(run.started_at, locale)}</div>
                {run.object_name && <div className="data-record-secondary"><code>{run.object_name}</code></div>}
                {run.size_bytes > 0 && <div className="data-record-secondary">{formatBytes(run.size_bytes, locale)}</div>}
                {run.remote_deleted_at && <div className="data-record-secondary">{tx('已按保留策略清理远端对象', 'Remote object pruned by retention policy')}</div>}
                {run.error && <div className="data-record-secondary">{run.error}</div>}
              </div>
            ))}
          </div>
        </div>
      )}

      <div id="restore-preview" className="data-sync-token-box" style={{ marginTop: 16 }}>
        <div className="data-record-title">{tx('恢复预览', 'Restore preview')}</div>
        <div className="data-sync-settings-grid" style={{ marginTop: 12 }}>
          <div className="form-group data-sync-settings-span-wide">
            <label>{tx('备份 zip', 'Backup zip')}</label>
            <div
              className={`skills-upload-dropzone${isDragging ? ' is-dragging' : ''}`}
              onClick={() => document.getElementById('backup-restore-file')?.click()}
              onDragOver={(event) => {
                event.preventDefault()
                setIsDragging(true)
              }}
              onDragLeave={(event) => {
                event.preventDefault()
                setIsDragging(false)
              }}
              onDrop={(event) => {
                event.preventDefault()
                setIsDragging(false)
                const file = event.dataTransfer.files?.[0] || null
                if (file) {
                  setRestoreFile(file)
                  setRestorePreview(null)
                  setRestoreApplyResult(null)
                  setRestoreError('')
                }
              }}
              style={{ padding: '24px', textAlign: 'center', cursor: 'pointer', marginTop: '6px' }}
            >
              <input
                id="backup-restore-file"
                type="file"
                accept=".zip,application/zip,application/x-zip-compressed"
                style={{ display: 'none' }}
                onChange={(event) => {
                  setRestoreFile(event.target.files?.[0] || null)
                  setRestorePreview(null)
                  setRestoreApplyResult(null)
                  setRestoreError('')
                }}
              />
              <div style={{ fontWeight: 600, fontSize: '14px', marginBottom: '4px', color: 'var(--color-text)' }}>
                {restoreFile ? restoreFile.name : tx('拖拽 zip 备份包到这里，或点击选择文件', 'Drag backup zip here, or click to choose')}
              </div>
              <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
                {restoreFile ? tx('已选择。请在下方点击“预览备份包”读取内容。', 'Selected. Click "Preview backup" below to read.') : tx('支持 Vola 导出的 zip 归档文件', 'Supports zip archives exported by Vola')}
              </div>
            </div>
          </div>
          <div className="form-group">
            <label>{tx('识别结果', 'Preview result')}</label>
            <button className="btn btn-primary" type="button" disabled={restorePreviewing || !restoreFile} onClick={() => void handlePreviewBackupRestore()}>
              {restorePreviewing ? tx('读取中...', 'Reading...') : tx('预览备份包', 'Preview backup')}
            </button>
          </div>
          <div className="form-group">
            <label htmlFor="backup-restore-mode">{tx('应用策略', 'Apply mode')}</label>
            <CustomSelect
              value={restoreMode}
              onChange={(val) => setRestoreMode(val as 'skip' | 'overwrite')}
              options={[
                { value: 'skip', label: tx('跳过已有文件', 'Skip existing files') },
                { value: 'overwrite', label: tx('覆盖已有文件', 'Overwrite existing files') },
              ]}
              ariaLabel={tx('应用策略', 'Apply mode')}
            />
          </div>
        </div>
        {restoreError && <div className="alert alert-warn" style={{ marginTop: 12 }}>{restoreError}</div>}
        {restorePreview && (
          <>
            <div className={restorePreview.recognized ? 'alert alert-ok' : 'alert alert-warn'} style={{ marginTop: 12 }}>
              {restorePreview.recognized
                ? tx(`已识别 ${restorePreview.total_files} 个文件，解压后约 ${formatBytes(restorePreview.total_bytes, locale)}。`, `Recognized ${restorePreview.total_files} files, about ${formatBytes(restorePreview.total_bytes, locale)} after unzip.`)
                : tx('这个文件未被识别为 Vola 备份包。', 'This file was not recognized as a Vola backup archive.')}
            </div>
            {restorePreview.categories.length > 0 && (
              <div className="data-sync-status-grid" style={{ marginTop: 12 }}>
                {restorePreview.categories.map((category) => (
                  <div className="data-sync-status-card" key={category.id}>
                    <div className="data-record-title">{restoreCategoryLabel(category.id, category.label)}</div>
                    <div className="data-record-secondary">{tx(`${category.files} 个文件`, `${category.files} files`)}</div>
                    <div className="data-record-secondary">{formatBytes(category.bytes, locale)}</div>
                  </div>
                ))}
              </div>
            )}
            {restorePreview.warnings && restorePreview.warnings.length > 0 && (
              <div className="alert alert-warn" style={{ marginTop: 12 }}>
                {restorePreview.warnings.join(' ')}
              </div>
            )}
            {restorePreview.recognized && (
              <div className="data-sync-actions data-sync-actions-compact">
                <button className="btn btn-primary" type="button" disabled={restoreApplying} onClick={() => void handleApplyBackupRestore()}>
                  {restoreApplying ? tx('恢复中...', 'Restoring...') : tx('应用恢复', 'Apply restore')}
                </button>
              </div>
            )}
          </>
        )}
        {restoreApplyResult && (
          <div className={restoreApplyResult.errors?.length ? 'alert alert-warn' : 'alert alert-ok'} style={{ marginTop: 12 }}>
            {tx(
              `恢复完成：写入 ${restoreApplyResult.applied} 个，覆盖 ${restoreApplyResult.overwritten} 个，跳过 ${restoreApplyResult.skipped} 个。`,
              `Restore complete: ${restoreApplyResult.applied} written, ${restoreApplyResult.overwritten} overwritten, ${restoreApplyResult.skipped} skipped.`,
            )}
            {restoreApplyResult.errors?.length ? ` ${restoreApplyResult.errors.join(' ')}` : ''}
            {restoreApplyResult.warnings?.length ? ` ${restoreApplyResult.warnings.join(' ')}` : ''}
          </div>
        )}
      </div>
    </div>
  )

  if (busy) {
    return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>
  }

  return (
    <div className="page materials-page">
      {renderBackupFocus()}

      <div id="github-backup" className="materials-panel data-sync-card">
        <div className="card-header">
          <h3 className="card-title">{tx('GitHub 备份目标', 'GitHub backup destination')}</h3>
        </div>
        <p className="data-record-secondary">
          {tx('选择一个 GitHub 仓库作为 Vola 的备份位置，授权后可以一键同步并保留可恢复的版本记录。', 'Choose a GitHub repository as your Vola backup destination, then sync on demand with recoverable version history.')}
        </p>

        {renderStatus()}
        {renderRemoteConflict()}

        {renderLocalAuthControls()}

        {authMode === 'github_app_user' && mirror?.github_app_user_connected && (
          <div className="data-sync-actions data-sync-actions-compact" style={{ marginTop: 16 }}>
            <button className="btn-text" type="button" disabled={working} onClick={handleDisconnectGitHubApp}>
              {tx('断开 GitHub', 'Disconnect GitHub')}
            </button>
          </div>
        )}
      </div>

      {renderStorageOverview()}
      {renderExternalBackupTargets()}
    </div>
  )
}
