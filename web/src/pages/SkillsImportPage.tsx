import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import CustomSelect from '../components/CustomSelect'
import LanguageToggle from '../components/LanguageToggle'
import { useI18n } from '../i18n'
import {
  api,
  buildAPIErrorFromPayload,
  buildAPIErrorFromResponse,
  notifyBillingRedirect,
  type ExternalSkillAssetUploadResult,
  type ImportResult,
  type Team,
} from '../api'

interface SkillsImportRequest {
  token: string
  platform: string
  teamID: string
  error: 'missing_token' | ''
}

interface AgentUploadAuthInfo {
  auth_mode: string
  trust_level: number
  scopes?: string[]
}

interface ExternalUploadStatus {
  busy?: boolean
  error?: string
  message?: string
}

type SkillScope = 'personal' | 'team'

const TEAM_SELECTION_KEY = 'vola:selected-team-id'
const SKILL_SCOPE_KEY = 'vola:skills-scope'

function parseSkillsImportRequest(search: string): SkillsImportRequest {
  const params = new URLSearchParams(search)
  const token = (params.get('token') || '').trim()
  const platform = (params.get('platform') || 'claude-web').trim() || 'claude-web'
  const teamID = (params.get('team_id') || params.get('team') || '').trim()

  if (!token) {
    return {
      token: '',
      platform,
      teamID,
      error: 'missing_token',
    }
  }

  return {
    token,
    platform,
    teamID,
    error: '',
  }
}

function teamMatches(team: Team, value: string) {
  return team.id === value || team.slug === value
}

function isZipFile(file: File) {
  const lowerName = file.name.toLowerCase()
  return lowerName.endsWith('.zip') || file.type === 'application/zip' || file.type === 'application/x-zip-compressed'
}

function formatFileSize(bytes: number, locale: 'zh-CN' | 'en') {
  if (!Number.isFinite(bytes) || bytes <= 0) return locale === 'zh-CN' ? '未知大小' : 'Unknown size'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

function warningLabel(code: string, locale: 'zh-CN' | 'en') {
  const labels: Record<string, { zh: string; en: string }> = {
    external_reference: { zh: '外部引用', en: 'External reference' },
    large_file: { zh: '大文件', en: 'Large file' },
    secret_risk: { zh: 'Secret 风险', en: 'Secret risk' },
    binary_asset: { zh: '二进制资源', en: 'Binary asset' },
  }
  const item = labels[code]
  if (!item) return code
  return locale === 'zh-CN' ? item.zh : item.en
}

function warningMessage(entry: { code: string; path?: string; message: string }, locale: 'zh-CN' | 'en') {
  const target = entry.path || ''
  if (locale !== 'zh-CN') return entry.message
  switch (entry.code) {
    case 'external_reference':
      return `${target || 'Skill 文档'} 引用了 Skill 目录外的 Claude 路径，当前 zip 未包含。`
    case 'large_file':
      return `${target} 超过 5 MB，确认是否需要随 Skill 同步。`
    case 'secret_risk':
      return `${target} 看起来可能包含 secret，分享或推送前请确认。`
    case 'binary_asset':
      return `${target} 已作为二进制资源保存。`
    default:
      return entry.message
  }
}

function extractResponsePayload(xhr: XMLHttpRequest) {
  if (xhr.response && typeof xhr.response === 'object') {
    return xhr.response
  }
  const text = (() => {
    if (typeof xhr.response === 'string') {
      return xhr.response
    }
    if (xhr.responseType === '' || xhr.responseType === 'text') {
      return xhr.responseText
    }
    return ''
  })()
  if (!text) {
    return null
  }
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function uploadSkillsArchive(
  token: string,
  platform: string,
  file: File,
  teamID: string | undefined,
  onProgress: (percent: number) => void,
): Promise<ImportResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const params = new URLSearchParams()
    if (platform) {
      params.set('platform', platform)
    }
    if (teamID) {
      params.set('team_id', teamID)
    }

    xhr.open('POST', `/agent/import/skills?${params.toString()}`)
    xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.responseType = 'json'

    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable || event.total <= 0) return
      onProgress(Math.max(1, Math.min(99, Math.round((event.loaded / event.total) * 100))))
    }

    xhr.onerror = () => {
      reject(new Error('network error'))
    }

    xhr.onload = () => {
      const payload = extractResponsePayload(xhr)
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress(100)
        const data = payload && payload.ok === true && payload.data !== undefined ? payload.data : payload
        resolve(data as ImportResult)
        return
      }

      const error = buildAPIErrorFromPayload(payload, xhr.statusText || 'upload failed')
      notifyBillingRedirect(error)
      reject(error)
    }

    const formData = new FormData()
    formData.append('platform', platform)
    if (teamID) {
      formData.append('team_id', teamID)
    }
    formData.append('file', file)
    xhr.send(formData)
  })
}

async function uploadExternalSkillAsset(
  token: string,
  platform: string,
  skillName: string,
  sourceRef: string,
  file: File,
  teamID?: string,
): Promise<ExternalSkillAssetUploadResult> {
  const formData = new FormData()
  formData.append('platform', platform)
  if (teamID) {
    formData.append('team_id', teamID)
  }
  formData.append('skill_name', skillName)
  formData.append('source_ref', sourceRef)
  formData.append('file', file)
  const params = new URLSearchParams()
  if (teamID) {
    params.set('team_id', teamID)
  }

  const res = await fetch(`/agent/import/skills/external${params.toString() ? `?${params.toString()}` : ''}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
    },
    body: formData,
  })
  const payload = await res.json().catch(() => null)
  if (!res.ok) {
    const error = payload
      ? buildAPIErrorFromPayload(payload, res.statusText || 'asset upload failed')
      : await buildAPIErrorFromResponse(res)
    notifyBillingRedirect(error)
    throw error
  }
  const data = payload && payload.ok === true && payload.data !== undefined ? payload.data : payload
  return data as ExternalSkillAssetUploadResult
}

async function validateUploadToken(token: string): Promise<AgentUploadAuthInfo> {
  const res = await fetch('/agent/auth/whoami', {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  })

  const payload = await res.json().catch(() => null)
  if (!res.ok) {
    const error = payload
      ? buildAPIErrorFromPayload(payload, res.statusText || 'token validation failed')
      : await buildAPIErrorFromResponse(res)
    notifyBillingRedirect(error)
    throw error
  }
  const data = payload && payload.ok === true && payload.data !== undefined ? payload.data : payload
  return data as AgentUploadAuthInfo
}

function externalRefKey(skillName: string, sourceFile: string, sourceRef: string) {
  return `${skillName}\u0000${sourceFile}\u0000${sourceRef}`
}

function manifestWarningsFrom(manifests: ImportResult['skill_manifests']) {
  return (manifests || []).flatMap((manifest) => manifest.warnings || [])
}

export default function SkillsImportPage() {
  const { locale, tx } = useI18n()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const externalInputRefs = useRef<Record<string, HTMLInputElement | null>>({})
  const [request] = useState<SkillsImportRequest>(() => parseSkillsImportRequest(window.location.search))
  const [hasSessionToken, setHasSessionToken] = useState(() => Boolean(window.localStorage.getItem('token')))
  const [teams, setTeams] = useState<Team[]>([])
  const [teamsLoading, setTeamsLoading] = useState(false)
  const [skillScope, setSkillScope] = useState<SkillScope>(() => {
    if (request.teamID) return 'team'
    return window.localStorage.getItem(SKILL_SCOPE_KEY) === 'team' ? 'team' : 'personal'
  })
  const [selectedTeamID, setSelectedTeamID] = useState(() => request.teamID || window.localStorage.getItem(TEAM_SELECTION_KEY) || '')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [busy, setBusy] = useState(false)
  const [tokenChecking, setTokenChecking] = useState(false)
  const [tokenReady, setTokenReady] = useState(false)
  const [tokenError, setTokenError] = useState('')
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')
  const [result, setResult] = useState<ImportResult | null>(null)
  const [externalUploads, setExternalUploads] = useState<Record<string, ExternalUploadStatus>>({})
  const selectedTeam = useMemo(
    () => teams.find((team) => teamMatches(team, selectedTeamID)) || null,
    [selectedTeamID, teams],
  )
  const activeTeamID = skillScope === 'team' ? selectedTeam?.id || selectedTeamID : ''
  const targetLabel = skillScope === 'team'
    ? (selectedTeam ? `${selectedTeam.name} / ${selectedTeam.slug}` : tx('团队空间', 'Team space'))
    : tx('个人空间', 'Personal space')
  const canUploadWithToken = Boolean(request.token && tokenReady)
  const canUploadWithSession = Boolean(!request.token && hasSessionToken)
  const canUpload = canUploadWithToken || canUploadWithSession
  const canUploadExternalAssets = Boolean(request.token && tokenReady)
  const showUploadActions = Boolean(selectedFile) && (!result || busy)
  const dashboardSkillsHref = activeTeamID ? `/skills?team=${encodeURIComponent(activeTeamID)}` : '/skills'
  const skillManifests = result?.skill_manifests || []
  const manifestWarnings = result?.warnings || []
  const manifestSummary = skillManifests.reduce((summary, manifest) => ({
    scripts: summary.scripts + (manifest.summary?.scripts || 0),
    dependencies: summary.dependencies + (manifest.summary?.dependency_files || 0),
    resources: summary.resources + (manifest.summary?.resources || 0),
    external: summary.external + (manifest.summary?.external_references || 0),
  }), {
    scripts: 0,
    dependencies: 0,
    resources: 0,
    external: 0,
  })

  useEffect(() => {
    if (!request.token) return
    const params = new URLSearchParams(window.location.search)
    if (!params.has('token')) return
    params.delete('token')
    const nextSearch = params.toString()
    window.history.replaceState({}, '', `${window.location.pathname}${nextSearch ? `?${nextSearch}` : ''}`)
  }, [request.token])

  useEffect(() => {
    const sessionToken = Boolean(window.localStorage.getItem('token'))
    setHasSessionToken(sessionToken)
    if (!sessionToken) return

    let cancelled = false
    setTeamsLoading(true)
    void api.getTeams()
      .then((items) => {
        if (cancelled) return
        setTeams(items)
        const requestedTeamID = request.teamID || selectedTeamID
        const nextTeam = items.find((team) => teamMatches(team, requestedTeamID)) || (skillScope === 'team' ? items[0] || null : null)
        if (nextTeam) {
          setSelectedTeamID(nextTeam.id)
          if (request.teamID || skillScope === 'team') {
            setSkillScope('team')
            window.localStorage.setItem(SKILL_SCOPE_KEY, 'team')
          }
          window.localStorage.setItem(TEAM_SELECTION_KEY, nextTeam.id)
        } else if (!request.teamID && skillScope === 'team') {
          setSkillScope('personal')
          window.localStorage.setItem(SKILL_SCOPE_KEY, 'personal')
        }
      })
      .catch(() => {
        if (!cancelled) {
          setTeams([])
        }
      })
      .finally(() => {
        if (!cancelled) {
          setTeamsLoading(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [request.teamID, selectedTeamID, skillScope])

  useEffect(() => {
    if (!request.token) {
      setTokenReady(false)
      setTokenError('')
      return
    }

    let cancelled = false
    setTokenChecking(true)
    setTokenReady(false)
    setTokenError('')

    void validateUploadToken(request.token)
      .then((info) => {
        if (cancelled) return
        if (info.auth_mode !== 'scoped_token') {
          setTokenError(tx(
            '这个链接里的 token 不是上传专用 token，请重新生成上传链接。',
            'This link does not contain a scoped upload token. Please generate a fresh upload link.',
          ))
          return
        }
        if ((info.trust_level || 0) < 3) {
          setTokenError(tx(
            '这个上传 token 的信任级别不足，至少需要 L3 Work。',
            'This upload token does not have enough trust. L3 Work is required.',
          ))
          return
        }
        if (!info.scopes?.includes('write:skills')) {
          setTokenError(tx(
            '这个 token 缺少 `write:skills` 权限，请重新生成上传链接。',
            'This token is missing `write:skills`. Please generate a fresh upload link.',
          ))
          return
        }
        setTokenReady(true)
      })
      .catch((err: any) => {
        if (cancelled) return
        setTokenError(
          err?.message || tx(
            '上传 token 校验失败，请重新生成上传链接。',
            'Upload token validation failed. Please generate a fresh upload link.',
          ),
        )
      })
      .finally(() => {
        if (!cancelled) {
          setTokenChecking(false)
        }
      })

    return () => {
      cancelled = true
    }
  }, [request.token, tx])

  const requestError = useMemo(() => {
    if (request.error === 'missing_token' && !hasSessionToken) {
      return tx(
        '请先登录 Vola，或使用 Vola 生成的带 token 上传链接。',
        'Sign in to Vola first, or use a tokenized upload link generated by Vola.',
      )
    }
    return ''
  }, [hasSessionToken, request.error, tx])

  const selectPersonalScope = () => {
    setSkillScope('personal')
    setSelectedTeamID('')
    window.localStorage.setItem(SKILL_SCOPE_KEY, 'personal')
  }

  const selectTeamScope = (teamID?: string) => {
    const nextTeam = teams.find((team) => teamMatches(team, teamID || selectedTeamID)) || selectedTeam || teams[0] || null
    if (!nextTeam && !request.teamID) return
    setSkillScope('team')
    const nextTeamID = nextTeam?.id || request.teamID
    setSelectedTeamID(nextTeamID)
    window.localStorage.setItem(SKILL_SCOPE_KEY, 'team')
    if (nextTeamID) {
      window.localStorage.setItem(TEAM_SELECTION_KEY, nextTeamID)
    }
  }

  const selectFile = (file: File | null) => {
    if (!file) return

    if (!isZipFile(file)) {
      setError(tx('请上传 `.zip` 格式的 skills archive。', 'Please upload a `.zip` skills archive.'))
      setSelectedFile(null)
      setResult(null)
      setProgress(0)
      setExternalUploads({})
      return
    }

    setSelectedFile(file)
    setError('')
    setResult(null)
    setProgress(0)
    setExternalUploads({})
  }

  const handleUpload = async () => {
    if (!selectedFile || !canUpload || busy) return

    setBusy(true)
    setError('')
    setResult(null)
    setProgress(0)
    setExternalUploads({})

    try {
      const uploaded = request.token
        ? await uploadSkillsArchive(request.token, request.platform, selectedFile, activeTeamID || undefined, setProgress)
        : await api.uploadSkillsZip(selectedFile, activeTeamID || undefined)
      if (!request.token) {
        setProgress(100)
      }
      setResult(uploaded)
      if (inputRef.current) {
        inputRef.current.value = ''
      }
    } catch (err: any) {
      const message = err?.message === 'network error'
        ? tx('上传过程中发生网络错误，请重试。', 'A network error occurred during upload. Please try again.')
        : (err?.message || tx('上传失败', 'Upload failed'))
      setError(message)
      setProgress(0)
    } finally {
      setBusy(false)
    }
  }

  const handleExternalAssetUpload = async (
    skillName: string,
    sourceFile: string,
    sourceRef: string,
    file: File | null,
  ) => {
    if (!request.token || !tokenReady || !file) return

    const key = externalRefKey(skillName, sourceFile, sourceRef)
    setExternalUploads((current) => ({
      ...current,
      [key]: { busy: true },
    }))

    try {
      const uploaded = await uploadExternalSkillAsset(request.token, request.platform, skillName, sourceRef, file, activeTeamID || undefined)
      setResult((current) => {
        if (!current) return current
        const nextManifests = (current.skill_manifests || []).map((manifest) => (
          manifest.skill_name === uploaded.skill_name ? uploaded.manifest : manifest
        ))
        return {
          ...current,
          imported: current.imported + 1,
          skill_manifests: nextManifests,
          warnings: manifestWarningsFrom(nextManifests),
        }
      })
      setExternalUploads((current) => ({
        ...current,
        [key]: {
          message: tx('已导入并刷新清单。', 'Imported and refreshed.'),
        },
      }))
    } catch (err: any) {
      setExternalUploads((current) => ({
        ...current,
        [key]: {
          error: err?.message || tx('上传失败', 'Upload failed'),
        },
      }))
    } finally {
      const input = externalInputRefs.current[key]
      if (input) {
        input.value = ''
      }
    }
  }

  return (
    <div className="skills-upload-page">
      <div className="skills-upload-card">
        <div className="login-card-header">
          <LanguageToggle />
        </div>
        <div className="sync-login-eyebrow">Vola Skills</div>
        <h1 className="login-title">{tx('上传 Skills Zip', 'Upload skills zip')}</h1>
        <p className="login-desc">
          {tx(
            '把完整的 skills zip 上传到 Vola。页面会导入 `/skills`，生成技能清单，并标出脚本、依赖、assets、外部 Claude 引用和环境变量提示。',
            'Upload a complete skills zip into Vola. The page imports it into `/skills`, generates the skill manifest, and flags scripts, dependencies, assets, external Claude references, and environment variable notes.',
          )}
        </p>

        <div className="sync-login-summary">
          <div className="sync-login-summary-row">
            <span>{tx('目标目录', 'Target root')}</span>
            <strong>/skills</strong>
          </div>
          <div className="sync-login-summary-row">
            <span>{tx('目标空间', 'Target space')}</span>
            <strong>{targetLabel}</strong>
          </div>
          <div className="sync-login-summary-row">
            <span>{tx('来源平台', 'Platform')}</span>
            <strong>{request.platform}</strong>
          </div>
        </div>

        <div className="skill-scope-control skills-upload-scope-control">
          <div className="materials-toggle-group" role="tablist" aria-label={tx('上传目标', 'Upload target')}>
            <button
              type="button"
              className={`materials-toggle-item ${skillScope === 'personal' ? 'is-active' : ''}`}
              onClick={selectPersonalScope}
            >
              {tx('个人', 'Personal')}
            </button>
            <button
              type="button"
              className={`materials-toggle-item ${skillScope === 'team' ? 'is-active' : ''}`}
              disabled={teams.length === 0 && !request.teamID}
              onClick={() => selectTeamScope()}
            >
              {tx('团队', 'Team')}
            </button>
          </div>
          {skillScope === 'team' && teams.length > 0 ? (
            <CustomSelect
              value={selectedTeam?.id || ''}
              onChange={(val) => selectTeamScope(val)}
              options={teams.map((team) => ({
                value: team.id,
                label: `${team.name} / ${team.slug}`
              }))}
              ariaLabel={tx('选择团队', 'Select team')}
            />
          ) : null}
          {teamsLoading ? <span className="materials-tile-pill skill-scope-pill">{tx('读取团队...', 'Loading teams...')}</span> : null}
        </div>

        {(requestError || tokenError || error) && (
          <div className="alert alert-error">
            {requestError || tokenError || error}
          </div>
        )}

        <div
          className={`skills-upload-dropzone${isDragging ? ' is-dragging' : ''}${requestError || tokenError || tokenChecking || busy ? ' is-disabled' : ''}`}
          onClick={() => {
            if (!requestError && !tokenError && !tokenChecking && !busy) {
              inputRef.current?.click()
            }
          }}
          onDragOver={(event) => {
            if (requestError || tokenError || tokenChecking || busy) return
            event.preventDefault()
            setIsDragging(true)
          }}
          onDragLeave={(event) => {
            event.preventDefault()
            setIsDragging(false)
          }}
          onDrop={(event) => {
            if (requestError || tokenError || tokenChecking || busy) return
            event.preventDefault()
            setIsDragging(false)
            const file = event.dataTransfer.files?.[0] || null
            selectFile(file)
          }}
          role="button"
          tabIndex={requestError || tokenError || tokenChecking || busy ? -1 : 0}
          onKeyDown={(event) => {
            if (requestError || tokenError || tokenChecking || busy) return
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault()
              inputRef.current?.click()
            }
          }}
        >
          <input
            ref={inputRef}
            type="file"
            accept=".zip,application/zip,application/x-zip-compressed"
            className="skills-upload-input"
            onChange={(event) => {
              const file = event.target.files?.[0] || null
              event.target.value = ''
              selectFile(file)
            }}
            disabled={!!requestError || busy}
          />
          <div className="skills-upload-dropzone-title">
            {tx('拖拽 zip 到这里，或点击选择文件', 'Drag a zip here, or click to choose a file')}
          </div>
          <div className="skills-upload-dropzone-copy">
            {tx(
              '请上传完整的 skills archive，不要只包含 `SKILL.md`；MCP、plugin 和 hook 会作为报告内容保留，不会自动启用。',
              'Upload a complete skills archive, not a `SKILL.md`-only shortcut. MCP, plugins, and hooks are preserved for reports and are not enabled automatically.',
            )}
          </div>
        </div>

        {selectedFile && (
          <div className="skills-upload-file-meta">
            <div className="data-record-title">{selectedFile.name}</div>
            <div className="data-record-secondary">
              {tx('文件大小：', 'File size: ')}{formatFileSize(selectedFile.size, locale)}
            </div>
          </div>
        )}

        {showUploadActions && (
          <div className="skills-upload-actions">
            <button
              className="btn btn-primary"
              disabled={!selectedFile || !!requestError || !!tokenError || tokenChecking || !canUpload || busy}
              onClick={() => { void handleUpload() }}
            >
              {tokenChecking ? tx('校验 token...', 'Checking token...') : busy ? tx('上传中...', 'Uploading...') : tx('开始上传', 'Start upload')}
            </button>
            <button
              className="btn"
              disabled={busy}
              onClick={() => {
                setSelectedFile(null)
                setResult(null)
                setError('')
                setProgress(0)
                setExternalUploads({})
                if (inputRef.current) {
                  inputRef.current.value = ''
                }
              }}
            >
              {tx('清空', 'Clear')}
            </button>
          </div>
        )}

        {(busy || progress > 0) && (
          <div className="skills-upload-progress">
            <div className="skills-upload-progress-bar">
              <div className="skills-upload-progress-fill" style={{ width: `${progress}%` }} />
            </div>
            <div className="data-record-secondary">
              {busy && progress >= 99
                ? tx('文件已上传，正在导入 skills...', 'Upload finished. Importing skills...')
                : tx(`上传进度：${progress}%`, `Upload progress: ${progress}%`)}
            </div>
          </div>
        )}

        {result && (
          <div className="skills-upload-results">
            <div className="alert alert-success">
              {tx(
                `上传完成，成功导入 ${result.imported} 个文件。`,
                `Upload complete. Imported ${result.imported} files.`,
              )}
            </div>

            <div className="sync-login-summary">
              <div className="sync-login-summary-row">
                <span>{tx('导入文件', 'Imported files')}</span>
                <strong>{result.imported}</strong>
              </div>
              <div className="sync-login-summary-row">
                <span>{tx('跳过文件', 'Skipped files')}</span>
                <strong>{result.skipped}</strong>
              </div>
              <div className="sync-login-summary-row">
                <span>{tx('导入 skills', 'Imported skills')}</span>
                <strong>{result.skills?.length || 0}</strong>
              </div>
            </div>

            {result.skills && result.skills.length > 0 && (
              <div className="skills-upload-skill-list">
                {result.skills.map((skill) => (
                  <span key={skill} className="skills-upload-skill-chip">{skill}</span>
                ))}
              </div>
            )}

            {skillManifests.length > 0 && (
              <div className="skills-upload-manifest">
                <div className="data-record-title">{tx('资产完整性检查', 'Asset integrity check')}</div>
                <div className="sync-login-summary">
                  <div className="sync-login-summary-row">
                    <span>{tx('清单文件', 'Manifest files')}</span>
                    <strong>{result.manifest_files || skillManifests.length}</strong>
                  </div>
                  <div className="sync-login-summary-row">
                    <span>{tx('脚本文件', 'Script files')}</span>
                    <strong>{manifestSummary.scripts}</strong>
                  </div>
                  <div className="sync-login-summary-row">
                    <span>{tx('依赖文件', 'Dependency files')}</span>
                    <strong>{manifestSummary.dependencies}</strong>
                  </div>
                  <div className="sync-login-summary-row">
                    <span>{tx('资源文件', 'Resource files')}</span>
                    <strong>{manifestSummary.resources}</strong>
                  </div>
                  <div className="sync-login-summary-row">
                    <span>{tx('外部引用', 'External references')}</span>
                    <strong>{manifestSummary.external}</strong>
                  </div>
                </div>
                <div className="skills-upload-manifest-list">
                  {skillManifests.map((manifest) => (
                    <div key={manifest.skill_name} className="skills-upload-manifest-item">
                      <div className="skills-upload-manifest-head">
                        <strong>{manifest.skill_name}</strong>
                        <span>{tx(`${manifest.summary.files} 个文件`, `${manifest.summary.files} files`)}</span>
                      </div>
                      <div className="data-record-secondary">
                        {tx(
                          `脚本 ${manifest.summary.scripts} · 依赖 ${manifest.summary.dependency_files} · 二进制 ${manifest.summary.binary_files}`,
                          `Scripts ${manifest.summary.scripts} · Dependencies ${manifest.summary.dependency_files} · Binary ${manifest.summary.binary_files}`,
                        )}
                      </div>
                      {manifest.external_references && manifest.external_references.length > 0 && (
                        <ul className="skills-upload-error-list">
                          {manifest.external_references.map((ref) => {
                            const refKey = externalRefKey(manifest.skill_name, ref.source_file, ref.path)
                            const uploadState = externalUploads[refKey] || {}
                            return (
                              <li key={refKey} className="skills-upload-external-ref">
                                <div className="skills-upload-external-ref-copy">
                                  <code>{ref.path}</code>{tx(' 由 ', ' from ')}<code>{ref.source_file}</code>{tx(' 引用。', ' references it.')}
                                  <span className={`skills-upload-ref-status ${ref.included ? 'is-included' : 'is-missing'}`}>
                                    {ref.included ? tx('已包含', 'Included') : tx('当前 zip 未包含', 'Missing from zip')}
                                  </span>
                                </div>
                                {!ref.included && (
                                  <div className="skills-upload-external-actions">
                                    <input
                                      ref={(node) => { externalInputRefs.current[refKey] = node }}
                                      type="file"
                                      className="skills-upload-input"
                                      onChange={(event) => {
                                        const file = event.target.files?.[0] || null
                                        void handleExternalAssetUpload(manifest.skill_name, ref.source_file, ref.path, file)
                                      }}
                                      disabled={!!uploadState.busy || !canUploadExternalAssets}
                                    />
                                    <button
                                      className="btn btn-sm"
                                      disabled={!!uploadState.busy || !canUploadExternalAssets}
                                      onClick={() => { externalInputRefs.current[refKey]?.click() }}
                                    >
                                      {uploadState.busy ? tx('上传中...', 'Uploading...') : tx('选择文件上传', 'Choose file')}
                                    </button>
                                    {!canUploadExternalAssets && (
                                      <span className="skills-upload-ref-note">{tx('外部引用文件需要上传链接中的 token。', 'External reference uploads need the tokenized upload link.')}</span>
                                    )}
                                    {uploadState.error && (
                                      <span className="skills-upload-ref-note is-error">{uploadState.error}</span>
                                    )}
                                    {uploadState.message && (
                                      <span className="skills-upload-ref-note is-success">{uploadState.message}</span>
                                    )}
                                  </div>
                                )}
                              </li>
                            )
                          })}
                        </ul>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}

            {manifestWarnings.length > 0 && (
              <div className="alert alert-warn">
                <div className="data-record-title">{tx('完整性提示', 'Integrity notes')}</div>
                <ul className="skills-upload-error-list">
                  {manifestWarnings.slice(0, 8).map((entry, index) => (
                    <li key={`${entry.code}-${entry.path || index}-${entry.message}`}>
                      <strong>{warningLabel(entry.code, locale)}：</strong>{warningMessage(entry, locale)}
                    </li>
                  ))}
                </ul>
                {manifestWarnings.length > 8 && (
                  <div className="data-record-secondary">
                    {tx(`还有 ${manifestWarnings.length - 8} 条提示，可在 skill 目录的清单文件查看。`, `${manifestWarnings.length - 8} more notes are available in the skill manifest.`)}
                  </div>
                )}
              </div>
            )}

            {result.errors && result.errors.length > 0 && (
              <div className="alert alert-warn">
                <div className="data-record-title">{tx('部分文件未导入', 'Some files were not imported')}</div>
                <ul className="skills-upload-error-list">
                  {result.errors.map((entry) => (
                    <li key={entry}>{entry}</li>
                  ))}
                </ul>
              </div>
            )}

            <div className="skills-upload-actions">
              <button
                className="btn btn-primary"
                onClick={() => {
                  window.location.assign(dashboardSkillsHref)
                }}
              >
                {tx('去 Dashboard 查看', 'Open dashboard')}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
