import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode, type LocalSkillSyncResponse, type LocalToolStatusResponse, type Team, type TeamAgentAsset, type TeamMember, type TeamRole, type TeamSkillReviewEvent, type TeamSkillSubscriptionReportResponse, type TeamSkillUpdateNotification, type TeamSkillUpdateNotificationsResponse, type TeamMcpAsset, type TeamSkillPublication, type TeamSkillSubscriptionStatus } from '../api'
import CustomSelect from '../components/CustomSelect'
import GitHubTreeList from '../components/GitHubTreeList'
import { useI18n } from '../i18n'
import { dataFileEditorRoute } from './data/DataShared'

const TEAM_ROOT = '/team'
const TEAM_SELECTION_KEY = 'vola:selected-team-id'
type TeamDialog = '' | 'create-team' | 'add-member' | 'create-agent' | 'create-mcp' | 'skill-publication'
type TeamView = 'overview' | 'assets' | 'sync' | 'files'

const TEAM_TEMPLATE = [
  { path: '/team', isDir: true, content: '', mimeType: 'directory' },
  { path: '/team/mcp', isDir: true, content: '', mimeType: 'directory' },
  { path: '/team/agents', isDir: true, content: '', mimeType: 'directory' },
  { path: '/team/prompts', isDir: true, content: '', mimeType: 'directory' },
  { path: '/team/playbooks', isDir: true, content: '', mimeType: 'directory' },
  {
    path: '/team/README.md',
    isDir: false,
    mimeType: 'text/markdown',
    content: `# 团队 AI 资料库

这个目录用于保存团队内部 AI 资料。

推荐结构：

- \`/skills/<name>/...\`：可运行 Skill，需要进入团队 Skill 列表、MCP \`list_skills\`、同步、转换和 Agent 分配时放这里。
- \`/team/mcp/...\`：内部 MCP server 记录、配置样例、负责人和安全说明。
- \`/team/prompts/...\`：团队复用的 prompt 模板。
- \`/team/playbooks/...\`：AI 使用技巧、评审方法和团队协作方法。
`,
  },
  {
    path: '/team/mcp/README.md',
    isDir: false,
    mimeType: 'text/markdown',
    content: `# 内部 MCP 目录

- 负责人：
- 用途：
- Server URL 或 stdio command：
- 必需环境变量：
- 数据访问范围：
- 配置说明：
- 风险说明：
`,
  },
  {
    path: '/team/agents/README.md',
    isDir: false,
    mimeType: 'text/markdown',
    content: `# 团队 Agent 配方

这里保存团队共享的 Agent 配置说明，不直接安装或运行 Agent。

推荐记录：

- 名称：
- 适用场景：
- 默认模型：
- 默认 Skill：
- 可访问数据范围：
- 需要人工审批的动作：
- Codex / Claude / Cursor / Gemini CLI 配置说明：
- 维护人：
`,
  },
  {
    path: '/team/prompts/README.md',
    isDir: false,
    mimeType: 'text/markdown',
    content: `# 提示词库

这里保存还没有升级成 Skill 的可复用 prompt 模板。
`,
  },
  {
    path: '/team/playbooks/README.md',
    isDir: false,
    mimeType: 'text/markdown',
    content: `# AI 使用技巧

这里保存团队工作方法、评审流程、模型选择经验和日常 AI 使用技巧。
`,
  },
]

function childCount(node: FileNode | null) {
  return node?.children?.length || 0
}

function formatCategoryCount(count: number, locale: string) {
  if (locale === 'zh-CN') return `${count} 项`
  return `${count} ${count === 1 ? 'item' : 'items'}`
}

function normalizeTemplatePath(path: string) {
  const normalized = path.startsWith('/') ? path : `/${path}`
  return normalized === '/' ? '/' : normalized.replace(/\/+$/g, '')
}

function hasSnapshotPath(paths: Set<string>, path: string) {
  return paths.has(normalizeTemplatePath(path))
}

function parentTemplatePath(path: string) {
  const normalized = normalizeTemplatePath(path)
  if (normalized === '/') return ''
  const index = normalized.lastIndexOf('/')
  return index <= 0 ? '/' : normalized.slice(0, index)
}

function countDirectChildren(entries: FileNode[], path: string) {
  const normalizedPath = normalizeTemplatePath(path)
  return entries.filter((entry) => parentTemplatePath(entry.path) === normalizedPath).length
}

function roleLabel(role: TeamRole | undefined, locale: string) {
  switch (role) {
    case 'owner':
      return locale === 'zh-CN' ? 'Owner' : 'Owner'
    case 'admin':
      return locale === 'zh-CN' ? '管理员' : 'Admin'
    case 'member':
      return locale === 'zh-CN' ? '成员' : 'Member'
    case 'viewer':
      return locale === 'zh-CN' ? '只读' : 'Viewer'
    default:
      return '-'
  }
}

function agentStatusLabel(status: string | undefined, visibility: string | undefined, locale: string) {
  if (status === 'archived') return locale === 'zh-CN' ? '已归档' : 'Archived'
  if (status === 'published' && visibility === 'team') return locale === 'zh-CN' ? '团队可见' : 'Team visible'
  return locale === 'zh-CN' ? '草稿' : 'Draft'
}

function reviewActionLabel(action: string, locale: string) {
  if (locale !== 'zh-CN') {
    if (action === 'request_review') return 'Review requested'
    if (action === 'approved') return 'Approved'
    if (action === 'changes_requested') return 'Changes requested'
    if (action === 'publish' || action === 'publish_agent') return 'Published'
    if (action === 'archive' || action === 'archive_agent') return 'Archived'
    return 'Draft updated'
  }
  if (action === 'request_review') return '提交审查'
  if (action === 'approved') return '审查通过'
  if (action === 'changes_requested') return '要求修改'
  if (action === 'publish' || action === 'publish_agent') return '发布'
  if (action === 'archive' || action === 'archive_agent') return '归档'
  return '更新草稿'
}

function syncResponseTotal(response: LocalSkillSyncResponse | null) {
  if (!response) return { add: 0, update: 0, conflict: 0, removable: 0, export: 0, written: 0, blocked: 0, manual: 0 }
  return response.agents.reduce((acc, agent) => {
    acc.add += agent.summary.add
    acc.update += agent.summary.update
    acc.conflict += agent.summary.conflict
    acc.removable += agent.summary.removable
    acc.export += agent.summary.export
    acc.written += agent.summary.written
    acc.blocked += agent.summary.blocked || 0
    acc.manual += agent.summary.manual_required || 0
    return acc
  }, { add: 0, update: 0, conflict: 0, removable: 0, export: 0, written: 0, blocked: 0, manual: 0 })
}

function syncActionClass(action: string) {
  if (action === 'add') return 'preview-action preview-action-create'
  if (action === 'update') return 'preview-action preview-action-update'
  if (action === 'delete') return 'preview-action preview-action-delete'
  if (action === 'export') return 'preview-action preview-action-update'
  if (action === 'conflict') return 'preview-action preview-action-conflict'
  return 'preview-action preview-action-skip'
}

function syncActionLabel(action: string, tx: (zh: string, en: string) => string) {
  if (action === 'add') return tx('新增', 'add')
  if (action === 'update') return tx('更新', 'update')
  if (action === 'unchanged') return tx('相同', 'same')
  if (action === 'missing') return tx('本地多出', 'extra')
  if (action === 'conflict') return tx('冲突', 'conflict')
  if (action === 'delete') return tx('清理', 'clean')
  if (action === 'export') return tx('导出', 'export')
  return action
}

function syncAgentSummaryText(agent: LocalSkillSyncResponse['agents'][number], tx: (zh: string, en: string) => string) {
  const summary = agent.summary
  return tx(
    `新增 ${summary.add} / 更新 ${summary.update} / 冲突 ${summary.conflict} / 写入 ${summary.written} / 可导出 ${summary.export}`,
    `add ${summary.add} / update ${summary.update} / conflicts ${summary.conflict} / written ${summary.written} / export ${summary.export}`,
  )
}

function publicationStatusLabel(publication: TeamSkillPublication, tx: (zh: string, en: string) => string) {
  if (publication.status === 'archived') return tx('已归档', 'Archived')
  if (publication.status === 'published' && publication.visibility === 'team') return tx('已发布', 'Published')
  return tx('草稿', 'Draft')
}

function publicationReviewLabel(publication: TeamSkillPublication, tx: (zh: string, en: string) => string) {
  if (publication.review_status === 'requested') return tx('待审查', 'Review requested')
  if (publication.review_status === 'approved') return tx('已通过', 'Approved')
  if (publication.review_status === 'changes_requested') return tx('需修改', 'Changes requested')
  return ''
}

function publicationDraftsFromPublications(publications: TeamSkillPublication[]) {
  return publications.reduce<Record<string, { version: string; release_note: string; note: string }>>((acc, pub) => {
    acc[pub.skill_path] = {
      version: pub.version || '',
      release_note: pub.release_note || '',
      note: pub.note || '',
    }
    return acc
  }, {})
}

function formatDateTime(value: string | undefined, locale: string) {
  if (!value) return locale === 'zh-CN' ? '暂无' : 'None'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function normalizeSlugInput(value: string) {
  return value.trim().toLowerCase().replace(/[^a-z0-9-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 64)
}

function teamMemberErrorMessage(err: any, tx: (zh: string, en: string) => string) {
  const message = `${err?.message || ''}`.trim().toLowerCase()
  if (message === 'user not found' || message === 'not found: user' || message === 'user') {
    return tx('没有找到这个用户 slug。请确认对方已经注册云账号，并填写个人资料里的 slug。', 'No user with that slug was found. Make sure the teammate has a cloud account and use their profile slug.')
  }
  if (message.includes('already') || message.includes('duplicate')) {
    return tx('这个成员已经在团队里。', 'This member is already in the team.')
  }
  return err?.message || tx('添加成员失败', 'Failed to add member')
}

export default function TeamLibraryPage() {
  const { locale, tx } = useI18n()
  const [teams, setTeams] = useState<Team[]>([])
  const [selectedTeamID, setSelectedTeamID] = useState('')
  const [teamRoot, setTeamRoot] = useState<FileNode | null>(null)
  const [skillsRoot, setSkillsRoot] = useState<FileNode | null>(null)
  const [teamCounts, setTeamCounts] = useState<Record<string, number>>({})
  const [members, setMembers] = useState<TeamMember[]>([])
  const [teamAgents, setTeamAgents] = useState<TeamAgentAsset[]>([])
  const [skillSubscriptionReport, setSkillSubscriptionReport] = useState<TeamSkillSubscriptionReportResponse | null>(null)
  const [skillUpdateNotifications, setSkillUpdateNotifications] = useState<TeamSkillUpdateNotification[]>([])
  const [skillUpdateNotificationInfo, setSkillUpdateNotificationInfo] = useState<Pick<TeamSkillUpdateNotificationsResponse, 'storage_path' | 'updated_at' | 'last_checked_at'> | null>(null)
  const [skillReviewHistory, setSkillReviewHistory] = useState<TeamSkillReviewEvent[]>([])
  const [skillPublications, setSkillPublications] = useState<TeamSkillPublication[]>([])
  const [skillPublicationDrafts, setSkillPublicationDrafts] = useState<Record<string, { version: string; release_note: string; note: string }>>({})
  const [subscriptions, setSubscriptions] = useState<TeamSkillSubscriptionStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [working, setWorking] = useState(false)
  const [agentWorking, setAgentWorking] = useState(false)
  const [sharingWorking, setSharingWorking] = useState(false)
  const [installingAgentSlug, setInstallingAgentSlug] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [treePath, setTreePath] = useState(TEAM_ROOT)
  const [newTeamName, setNewTeamName] = useState('')
  const [newTeamSlug, setNewTeamSlug] = useState('')
  const [newTeamDescription, setNewTeamDescription] = useState('')
  const [memberSlug, setMemberSlug] = useState('')
  const [memberRole, setMemberRole] = useState<TeamRole>('member')
  const [currentUser, setCurrentUser] = useState<any>(null)
  const [newAgentName, setNewAgentName] = useState('')
  const [newAgentSlug, setNewAgentSlug] = useState('')
  const [newAgentDescription, setNewAgentDescription] = useState('')
  const [newAgentInstructions, setNewAgentInstructions] = useState('')
  const [teamMcps, setTeamMcps] = useState<TeamMcpAsset[]>([])
  const [newMcpName, setNewMcpName] = useState('')
  const [newMcpSlug, setNewMcpSlug] = useState('')
  const [newMcpDescription, setNewMcpDescription] = useState('')
  const [newMcpTransport, setNewMcpTransport] = useState<'stdio' | 'http'>('stdio')
  const [newMcpCommand, setNewMcpCommand] = useState('')
  const [newMcpArgs, setNewMcpArgs] = useState('')
  const [newMcpEnv, setNewMcpEnv] = useState('')
  const [newMcpURL, setNewMcpURL] = useState('')
  const [newMcpHeaders, setNewMcpHeaders] = useState('')
  const [newMcpTags, setNewMcpTags] = useState('')
  const [mcpWorking, setMcpWorking] = useState(false)
  const [mcpHealth, setMcpHealth] = useState<Record<string, { status: 'online' | 'offline'; latency_ms: number }>>({})
  const [teamSkillSyncPreview, setTeamSkillSyncPreview] = useState<LocalSkillSyncResponse | null>(null)
  const [teamSkillSyncBusy, setTeamSkillSyncBusy] = useState<'preview' | 'apply' | ''>('')
  const [teamSkillExportBusy, setTeamSkillExportBusy] = useState('')
  const [platformRefreshBusy, setPlatformRefreshBusy] = useState('')
  const [localToolStatus, setLocalToolStatus] = useState<LocalToolStatusResponse | null>(null)
  const [localToolStatusLoading, setLocalToolStatusLoading] = useState(false)
  const [teamDialog, setTeamDialog] = useState<TeamDialog>('')
  const [editingPublicationPath, setEditingPublicationPath] = useState('')
  const [teamView, setTeamView] = useState<TeamView>('overview')

  const selectedTeam = useMemo(
    () => teams.find((team) => team.id === selectedTeamID) || teams[0] || null,
    [selectedTeamID, teams],
  )

  const loadTeams = useCallback(async () => {
    const list = await api.getTeams()
    setTeams(list)
    const saved = window.localStorage.getItem(TEAM_SELECTION_KEY) || ''
    const nextSelected = list.find((team) => team.id === selectedTeamID)?.id
      || list.find((team) => team.id === saved)?.id
      || list[0]?.id
      || ''
    setSelectedTeamID(nextSelected)
    if (nextSelected) window.localStorage.setItem(TEAM_SELECTION_KEY, nextSelected)
    return list.find((team) => team.id === nextSelected) || null
  }, [selectedTeamID])

  const loadTeamData = useCallback(async (team: Team | null) => {
    if (!team) {
      setTeamRoot(null)
      setSkillsRoot(null)
      setTeamCounts({})
      setMembers([])
      setTeamAgents([])
      setSkillSubscriptionReport(null)
      setSkillUpdateNotifications([])
      setSkillUpdateNotificationInfo(null)
      setSkillReviewHistory([])
      setSkillPublications([])
      setSkillPublicationDrafts({})
      setSubscriptions([])
      setTeamSkillSyncPreview(null)
      setTeamSkillSyncBusy('')
      setTeamSkillExportBusy('')
      setPlatformRefreshBusy('')
      setLocalToolStatus(null)
      return
    }
    setLocalToolStatusLoading(true)
    const [
      teamResult,
      skillsResult,
      snapshotResult,
      membersResult,
      agentsResult,
      mcpsResult,
      reviewHistoryResult,
      subscriptionReportResult,
      notificationsResult,
      publicationsResult,
      subscriptionsResult,
    ] = await Promise.allSettled([
      api.getTeamTree(team.id, TEAM_ROOT),
      api.getTeamTree(team.id, '/skills'),
      api.getTeamTreeSnapshot(team.id, '/'),
      api.getTeamMembers(team.id),
      api.getTeamAgents(team.id),
      api.fetchTeamMcps(team.id),
      api.getTeamSkillReviewHistory(team.id),
      team.can_manage_members ? api.getTeamSkillSubscriptionReport(team.id) : Promise.resolve(null),
      team.can_manage_members ? api.getTeamSkillUpdateNotifications(team.id) : Promise.resolve(null),
      api.getTeamSkillPublications(team.id),
      api.getTeamSkillSubscriptions(team.id),
    ])
    const entries = snapshotResult.status === 'fulfilled' ? snapshotResult.value.entries || [] : []
    const existingPaths = new Set(entries.map((entry) => normalizeTemplatePath(entry.path)))
    setTeamRoot(teamResult.status === 'fulfilled' && hasSnapshotPath(existingPaths, TEAM_ROOT) ? teamResult.value : null)
    setSkillsRoot(skillsResult.status === 'fulfilled' ? skillsResult.value : null)
    setTeamCounts({
      mcp: countDirectChildren(entries, '/team/mcp'),
      agents: countDirectChildren(entries, '/team/agents'),
      prompts: countDirectChildren(entries, '/team/prompts'),
      playbooks: countDirectChildren(entries, '/team/playbooks'),
    })
    setMembers(membersResult.status === 'fulfilled' ? membersResult.value : [])
    setTeamAgents(agentsResult.status === 'fulfilled' ? agentsResult.value.agents || [] : [])
    setTeamMcps(mcpsResult.status === 'fulfilled' ? mcpsResult.value.mcps || [] : [])
    setSkillReviewHistory(reviewHistoryResult.status === 'fulfilled' ? reviewHistoryResult.value.events || [] : [])
    setSkillSubscriptionReport(subscriptionReportResult.status === 'fulfilled' ? subscriptionReportResult.value : null)
    const notificationsResponse = notificationsResult.status === 'fulfilled' ? notificationsResult.value : null
    setSkillUpdateNotifications(notificationsResponse?.notifications || [])
    setSkillUpdateNotificationInfo(notificationsResponse ? {
      storage_path: notificationsResponse.storage_path,
      updated_at: notificationsResponse.updated_at,
      last_checked_at: notificationsResponse.last_checked_at,
    } : null)
    const nextSkillPublications = publicationsResult.status === 'fulfilled' ? publicationsResult.value.publications || [] : []
    setSkillPublications(nextSkillPublications)
    setSkillPublicationDrafts(publicationDraftsFromPublications(nextSkillPublications))
    setSubscriptions(subscriptionsResult.status === 'fulfilled' ? subscriptionsResult.value.subscriptions || [] : [])
    loadLocalToolsStatus()
  }, [])

  const loadLocalToolsStatus = () => {
    setLocalToolStatusLoading(true)
    return api.getLocalToolsStatus()
      .then((result) => setLocalToolStatus(result))
      .catch(() => setLocalToolStatus(null))
      .finally(() => setLocalToolStatusLoading(false))
  }

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [me, selected] = await Promise.all([
        api.getMe().catch(() => null),
        loadTeams().catch(() => null),
      ])
      setCurrentUser(me)
      if (selected) {
        await loadTeamData(selected)
      }
    } catch (err: any) {
      setError(err?.message || tx('读取团队失败', 'Failed to load teams'))
    } finally {
      setLoading(false)
    }
  }, [loadTeamData, loadTeams, tx])

  useEffect(() => {
    const fetchHealth = () => {
      api.getLocalMcpHealth()
        .then((res) => {
          if (res) {
            setMcpHealth(res)
          }
        })
        .catch(() => {})
    }
    fetchHealth()
    const timer = setInterval(fetchHealth, 15000)
    return () => clearInterval(timer)
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    setMessage('')
  }, [locale])

  useEffect(() => {
    if (!selectedTeam) return
    window.localStorage.setItem(TEAM_SELECTION_KEY, selectedTeam.id)
    setTreePath(TEAM_ROOT)
    void loadTeamData(selectedTeam)
  }, [loadTeamData, selectedTeam])

  const subscriptionSummary = useMemo(() => {
    return (skillSubscriptionReport?.skills || []).reduce((acc, skill) => {
      acc.skills += 1
      acc.personalCopies += skill.installed_count + skill.update_available_count + skill.source_missing_count
      acc.notInstalled += skill.not_installed_count
      acc.updates += skill.update_available_count
      acc.missing += skill.source_missing_count
      return acc
    }, { skills: 0, personalCopies: 0, notInstalled: 0, updates: 0, missing: 0 })
  }, [skillSubscriptionReport])

  const reviewEventCount = useMemo(() => (
    skillReviewHistory.filter((event) => event.action === 'request_review').length
  ), [skillReviewHistory])

  const teamLocalSyncTotal = useMemo(() => syncResponseTotal(teamSkillSyncPreview), [teamSkillSyncPreview])
  const localToolPlatformMap = useMemo(() => {
    return (localToolStatus?.platforms || []).reduce<Record<string, LocalToolStatusResponse['platforms'][number]>>((acc, item) => {
      acc[item.id] = item
      return acc
    }, {})
  }, [localToolStatus])
  const teamSkillSyncSummary = useMemo(() => {
    if (!teamSkillSyncPreview) return tx('尚未生成预览。', 'No preview yet.')
    return tx(
      `新增 ${teamLocalSyncTotal.add}，更新 ${teamLocalSyncTotal.update}，冲突 ${teamLocalSyncTotal.conflict}，可导出 ${teamLocalSyncTotal.export}。`,
      `add ${teamLocalSyncTotal.add}, update ${teamLocalSyncTotal.update}, conflicts ${teamLocalSyncTotal.conflict}, export ${teamLocalSyncTotal.export}.`,
    )
  }, [teamLocalSyncTotal, teamSkillSyncPreview, tx])
  const editingPublication = useMemo(
    () => skillPublications.find((pub) => pub.skill_path === editingPublicationPath) || null,
    [editingPublicationPath, skillPublications],
  )

  const openSkillPublicationDialog = (skillPath: string) => {
    setEditingPublicationPath(skillPath)
    setTeamDialog('skill-publication')
  }

  const closeTeamDialog = () => {
    setTeamDialog('')
    setEditingPublicationPath('')
  }

  const handleCheckTeamSkillSubscriptions = async () => {
    if (!selectedTeam || sharingWorking) return
    setSharingWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.checkTeamSkillSubscriptions(selectedTeam.id)
      if (response.report) setSkillSubscriptionReport(response.report)
      setSkillUpdateNotifications(response.notifications || [])
      setSkillUpdateNotificationInfo({
        storage_path: response.storage_path,
        updated_at: response.updated_at,
        last_checked_at: response.last_checked_at,
      })
      setMessage(response.notifications?.length
        ? tx(`发现 ${response.notifications.length} 条团队 Skill 更新提醒。`, `${response.notifications.length} team skill update notices found.`)
        : tx('已检查团队 Skill 订阅，没有新的更新提醒。', 'Team skill subscriptions checked. No new update notices.'))
    } catch (err: any) {
      setError(err?.message || tx('检查团队 Skill 更新失败', 'Failed to check team skill updates'))
    } finally {
      setSharingWorking(false)
    }
  }

  const handleTeamSkillSync = async (mode: 'preview' | 'apply') => {
    if (!selectedTeam || teamSkillSyncBusy) return
    if (mode === 'apply' && teamLocalSyncTotal.blocked > 0) {
      setError(tx('本地同步里还有阻断项，先看预览再应用。', 'Blocked local sync items need review before applying.'))
      return
    }
    setTeamSkillSyncBusy(mode)
    setError('')
    setMessage('')
    try {
      const response = mode === 'preview'
        ? await api.previewLocalSkillSync(selectedTeam.id)
        : await api.applyLocalSkillSync(selectedTeam.id, teamLocalSyncTotal.manual > 0)
      setTeamSkillSyncPreview(response)
      const total = syncResponseTotal(response)
      setMessage(mode === 'preview'
        ? tx(`已生成本机预览：新增 ${total.add}，更新 ${total.update}，冲突 ${total.conflict}。`, `Local preview ready: ${total.add} add, ${total.update} update, ${total.conflict} conflicts.`)
        : tx(`已应用到本机：写入 ${total.written} 个文件。`, `Applied locally: ${total.written} files written.`))
    } catch (err: any) {
      setError(err?.message || tx('团队 Skill 本机同步失败', 'Failed to sync team skills locally'))
    } finally {
      setTeamSkillSyncBusy('')
    }
  }

  const handleTeamSkillExport = async (agentId: string) => {
    if (!selectedTeam || teamSkillExportBusy) return
    setTeamSkillExportBusy(agentId)
    setError('')
    setMessage('')
    try {
      await api.downloadLocalSkillSyncExport(agentId, selectedTeam.id)
      setMessage(tx('导出包已生成。', 'Export package created.'))
    } catch (err: any) {
      setError(err?.message || tx('生成导出包失败', 'Failed to create export package'))
    } finally {
      setTeamSkillExportBusy('')
    }
  }

  const handleRefreshLocalPlatform = async (platform: 'codex' | 'claude-code') => {
    if (platformRefreshBusy || !selectedTeam) return
    setPlatformRefreshBusy(platform)
    setError('')
    setMessage('')
    try {
      const result = await api.refreshLocalPlatformConnection(platform)
      setMessage(tx(
        `${result.name || result.platform} 已刷新本机连接，已发布的团队 MCP 会同步到本机配置。`,
        `${result.name || result.platform} refreshed locally; published team MCPs were synced to the local config.`,
      ))
      void loadLocalToolsStatus()
    } catch (err: any) {
      setError(err?.message || tx('刷新本机连接失败', 'Failed to refresh local connection'))
    } finally {
      setPlatformRefreshBusy('')
    }
  }

  const localToolPlatformOrder = ['codex', 'claude-code', 'cursor-agent', 'gemini-cli']
  const localToolPlatformLabel = (platform: string) => {
    if (platform === 'codex') return 'Codex'
    if (platform === 'claude-code') return 'Claude Code'
    if (platform === 'cursor-agent') return 'Cursor'
    if (platform === 'gemini-cli') return 'Gemini CLI'
    return platform
  }

  const localToolSyncModeLabel = (platform: LocalToolStatusResponse['platforms'][number]) => {
    if (platform.auto_sync_supported) return tx('可自动同步', 'Auto sync')
    if (platform.id === 'gemini-cli') return tx('手工处理', 'Manual')
    if (platform.export_supported) return tx('只导出', 'Export only')
    return tx('未支持', 'Unsupported')
  }
  const overviewStats = useMemo(() => [
    {
      label: tx('成员', 'Members'),
      value: members.length,
      detail: selectedTeam ? roleLabel(selectedTeam.role, locale) : tx('未选择', 'No team'),
    },
    {
      label: tx('Skill', 'Skills'),
      value: childCount(skillsRoot),
      detail: tx('团队 Skill', 'Team skills'),
    },
    {
      label: 'Agent',
      value: teamAgents.length,
      detail: tx('团队配方', 'Team recipes'),
    },
    {
      label: 'MCP',
      value: teamMcps.length,
      detail: tx('共享配置', 'Shared configs'),
    },
  ], [locale, members.length, selectedTeam, skillsRoot, teamAgents.length, teamMcps.length, tx])
  const teamTabs = useMemo(() => [
    { id: 'overview' as const, label: tx('概览', 'Overview') },
    { id: 'assets' as const, label: tx('资产', 'Assets') },
    { id: 'sync' as const, label: tx('同步', 'Sync') },
    { id: 'files' as const, label: tx('文件', 'Files') },
  ], [tx])

  const publicationSavePayload = (pub: TeamSkillPublication, overrides: Partial<TeamSkillPublication> = {}) => ({
    skill_path: pub.skill_path,
    status: overrides.status || pub.status,
    visibility: overrides.visibility || pub.visibility,
    version: overrides.version ?? pub.version,
    release_note: overrides.release_note ?? pub.release_note,
    note: overrides.note ?? pub.note,
  })

  const refreshSkillPublications = async () => {
    if (!selectedTeam) return
    const updatedPubs = await api.getTeamSkillPublications(selectedTeam.id)
    const next = updatedPubs.publications || []
    setSkillPublications(next)
    setSkillPublicationDrafts(publicationDraftsFromPublications(next))
  }

  const updateSkillPublicationDraft = (skillPath: string, key: 'version' | 'release_note' | 'note', value: string) => {
    setSkillPublicationDrafts((current) => ({
      ...current,
      [skillPath]: {
        version: current[skillPath]?.version || '',
        release_note: current[skillPath]?.release_note || '',
        note: current[skillPath]?.note || '',
        [key]: value,
      },
    }))
  }

  const handleSaveSkillPublicationMeta = async (pub: TeamSkillPublication) => {
    if (!selectedTeam || sharingWorking) return false
    const draft = skillPublicationDrafts[pub.skill_path] || {
      version: pub.version || '',
      release_note: pub.release_note || '',
      note: pub.note || '',
    }
    setSharingWorking(true)
    setError('')
    setMessage('')
    try {
      const response = await api.saveTeamSkillPublication(selectedTeam.id, publicationSavePayload(pub, draft))
      const next = response.publications || []
      setSkillPublications(next)
      setSkillPublicationDrafts(publicationDraftsFromPublications(next))
      setMessage(tx('发布信息已保存。', 'Publication details saved.'))
      return true
    } catch (err: any) {
      setError(err?.message || tx('保存发布信息失败', 'Failed to save publication details'))
      return false
    } finally {
      setSharingWorking(false)
    }
  }

  const categories = useMemo(() => {
    return [
      {
        title: tx('团队 Skill', 'Team Skills'),
        subtitle: '/skills',
        desc: tx('归属当前团队的 Skill，不会和成员个人 Skill 混在一起。', 'Skills owned by this team, separate from personal skills.'),
        count: childCount(skillsRoot),
        icon: 'icon-stack',
        action: <button className="btn btn-sm" type="button" onClick={() => { setTreePath('/skills'); setTeamView('files') }}>{tx('打开', 'Open')}</button>,
      },
      {
        title: 'MCP',
        subtitle: '/team/mcp',
        desc: tx('内部 MCP server、配置样例、负责人和安全说明。', 'Internal MCP servers, config examples, owners, and security notes.'),
        count: teamMcps.length,
        icon: 'icon-device',
        action: selectedTeam ? <Link className="btn btn-sm" to={dataFileEditorRoute('/team/mcp/README.md', selectedTeam.id)}>{tx('打开', 'Open')}</Link> : null,
      },
      {
        title: tx('Agent 配方', 'Agent Recipes'),
        subtitle: '/team/agents',
        desc: tx('团队共用的 Agent 说明、默认 Skill、模型和权限建议。', 'Shared agent instructions, default skills, models, and permission notes.'),
        count: teamAgents.length,
        icon: 'icon-device',
        action: selectedTeam ? <Link className="btn btn-sm" to={dataFileEditorRoute('/team/agents/README.md', selectedTeam.id)}>{tx('打开', 'Open')}</Link> : null,
      },
      {
        title: tx('提示词', 'Prompts'),
        subtitle: '/team/prompts',
        desc: tx('团队复用的 prompt 模板，适合还没变成 Skill 的内容。', 'Reusable prompt templates before they become full skills.'),
        count: teamCounts.prompts || 0,
        icon: 'icon-file',
        action: selectedTeam ? <Link className="btn btn-sm" to={dataFileEditorRoute('/team/prompts/README.md', selectedTeam.id)}>{tx('打开', 'Open')}</Link> : null,
      },
      {
        title: tx('AI 使用技巧', 'AI Playbooks'),
        subtitle: '/team/playbooks',
        desc: tx('评审方法、模型使用经验、协作流程和日常技巧。', 'Review routines, model notes, collaboration methods, and daily usage tips.'),
        count: teamCounts.playbooks || 0,
        icon: 'icon-folder',
        action: selectedTeam ? <Link className="btn btn-sm" to={dataFileEditorRoute('/team/playbooks/README.md', selectedTeam.id)}>{tx('打开', 'Open')}</Link> : null,
      },
    ]
  }, [locale, selectedTeam, skillsRoot, teamAgents.length, teamCounts, teamMcps.length, tx])

  const teamTreeLoader = useCallback((path: string) => {
    if (!selectedTeam) return Promise.reject(new Error('team not selected'))
    return api.getTeamTree(selectedTeam.id, path)
  }, [selectedTeam])

  const currentTreeRoot = treePath.startsWith('/skills') ? '/skills' : TEAM_ROOT

  const teamFileRoute = useCallback((path: string) => {
    return selectedTeam ? dataFileEditorRoute(path, selectedTeam.id) : dataFileEditorRoute(path)
  }, [selectedTeam])

  const handleCreateTeam = async () => {
    const slug = normalizeSlugInput(newTeamSlug || newTeamName)
    const name = newTeamName.trim() || slug
    if (!slug || !name) {
      setError(tx('请填写团队名称和 slug。', 'Enter a team name and slug.'))
      return
    }
    setWorking(true)
    setMessage('')
    setError('')
    try {
      const team = await api.createTeam({ slug, name, description: newTeamDescription })
      setTeams((current) => [team, ...current.filter((item) => item.id !== team.id)])
      setSelectedTeamID(team.id)
      setNewTeamName('')
      setNewTeamSlug('')
      setNewTeamDescription('')
      closeTeamDialog()
      setMessage(tx('团队已创建。', 'Team created.'))
      await loadTeamData(team)
    } catch (err: any) {
      setError(err?.message || tx('创建团队失败', 'Failed to create team'))
    } finally {
      setWorking(false)
    }
  }

  const handleCreateTemplate = async () => {
    if (!selectedTeam) return
    setWorking(true)
    setMessage('')
    setError('')
    try {
      let created = 0
      let skipped = 0
      const snapshot = await api.getTeamTreeSnapshot(selectedTeam.id, '/')
      const existingPaths = new Set((snapshot.entries || []).map((entry) => normalizeTemplatePath(entry.path)))
      for (const item of TEAM_TEMPLATE) {
        if (hasSnapshotPath(existingPaths, item.path)) {
          skipped += 1
          continue
        }
        await api.writeTeamTree(selectedTeam.id, item.path, {
          content: item.content,
          mimeType: item.mimeType,
          isDir: item.isDir,
          metadata: { source: 'manual', library: 'team-ai', team_slug: selectedTeam.slug },
          minTrustLevel: 2,
        })
        existingPaths.add(normalizeTemplatePath(item.path))
        created += 1
      }
      setMessage(
        created > 0
          ? tx(`已创建 ${created} 个目录/文件，${skipped} 个已有项目未改。`, `Created ${created} folders/files. ${skipped} existing items were left unchanged.`)
          : tx('目录模板已存在，没有改动。', 'The folder template already exists. Nothing changed.'),
      )
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('创建团队资料库失败', 'Failed to create team library'))
    } finally {
      setWorking(false)
    }
  }

  const handleAddMember = async () => {
    if (!selectedTeam) return
    const slug = memberSlug.trim()
    if (!slug) return
    setWorking(true)
    setMessage('')
    setError('')
    try {
      const member = await api.addTeamMember(selectedTeam.id, slug, memberRole)
      setMembers((current) => [member, ...current.filter((item) => item.user_id !== member.user_id)])
      setMemberSlug('')
      closeTeamDialog()
      setMessage(tx('成员已加入团队。', 'Member added to the team.'))
    } catch (err: any) {
      setError(teamMemberErrorMessage(err, tx))
    } finally {
      setWorking(false)
    }
  }

  const handleCreateAgent = async () => {
    if (!selectedTeam) return
    const slug = normalizeSlugInput(newAgentSlug || newAgentName)
    const name = newAgentName.trim() || slug
    if (!slug || !name) {
      setError(tx('请填写 Agent 名称和 slug。', 'Enter an agent name and slug.'))
      return
    }
    setAgentWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.saveTeamAgent(selectedTeam.id, {
        slug,
        name,
        description: newAgentDescription,
        instructions: newAgentInstructions,
        status: selectedTeam.can_manage_members ? 'published' : 'draft',
        visibility: selectedTeam.can_manage_members ? 'team' : 'private',
        target_agents: ['codex', 'claude-code', 'cursor', 'gemini-cli'],
      })
      setTeamAgents(response.agents || [])
      if (!selectedTeam.can_manage_members) {
        const history = await api.requestTeamSkillReview(selectedTeam.id, {
          asset_type: 'agent',
          agent_slug: slug,
          note: 'Created by a team member.',
        })
        setSkillReviewHistory(history.events || [])
      }
      setNewAgentName('')
      setNewAgentSlug('')
      setNewAgentDescription('')
      setNewAgentInstructions('')
      closeTeamDialog()
      setMessage(selectedTeam.can_manage_members
        ? tx('Agent 配方已发布给团队。', 'Agent recipe published to the team.')
        : tx('Agent 配方已保存为草稿，等待管理员发布。', 'Agent recipe saved as draft for an admin to publish.'))
    } catch (err: any) {
      setError(err?.message || tx('保存 Agent 配方失败', 'Failed to save agent recipe'))
    } finally {
      setAgentWorking(false)
    }
  }

  const handlePublishAgent = async (agent: TeamAgentAsset, status: 'draft' | 'published' | 'archived') => {
    if (!selectedTeam) return
    setAgentWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.saveTeamAgent(selectedTeam.id, {
        slug: agent.slug,
        name: agent.name,
        description: agent.description,
        instructions: agent.instructions,
        default_skill_paths: agent.default_skill_paths || [],
        target_agents: agent.target_agents || [],
        model: agent.model,
        permissions: agent.permissions || [],
        approval_required: agent.approval_required || [],
        maintainer: agent.maintainer,
        status,
        visibility: status === 'published' ? 'team' : 'private',
      })
      setTeamAgents(response.agents || [])
      setMessage(status === 'published'
        ? tx('Agent 配方已发布。', 'Agent recipe published.')
        : status === 'archived'
          ? tx('Agent 配方已归档。', 'Agent recipe archived.')
          : tx('Agent 配方已转为草稿。', 'Agent recipe moved to draft.'))
    } catch (err: any) {
      setError(err?.message || tx('更新 Agent 配方失败', 'Failed to update agent recipe'))
    } finally {
      setAgentWorking(false)
    }
  }

  const handleRequestAgentReview = async (agent: TeamAgentAsset) => {
    if (!selectedTeam) return
    setAgentWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.requestTeamSkillReview(selectedTeam.id, {
        asset_type: 'agent',
        agent_slug: agent.slug,
      })
      setSkillReviewHistory(response.events || [])
      setMessage(tx('Agent 配方已提交给管理员审查。', 'Agent recipe review requested for admins.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('提交 Agent 审查失败', 'Failed to request agent review'))
    } finally {
      setAgentWorking(false)
    }
  }

  const handleResolveAgentReview = async (agent: TeamAgentAsset, decision: 'approved' | 'changes_requested') => {
    if (!selectedTeam) return
    setAgentWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.resolveTeamSkillReview(selectedTeam.id, {
        asset_type: 'agent',
        agent_slug: agent.slug,
        decision,
      })
      setSkillReviewHistory(response.events || [])
      setMessage(decision === 'approved'
        ? tx('Agent 配方审查已通过。', 'Agent recipe review approved.')
        : tx('已要求修改 Agent 配方。', 'Changes requested for this agent recipe.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('处理 Agent 审查失败', 'Failed to resolve agent review'))
    } finally {
      setAgentWorking(false)
    }
  }

  const handleInstallAgent = async (agent: TeamAgentAsset) => {
    if (!selectedTeam || installingAgentSlug) return
    setInstallingAgentSlug(agent.slug)
    setMessage('')
    setError('')
    try {
      await api.installTeamAgent(selectedTeam.id, agent.slug)
      setMessage(tx(`已安装到个人 Agent 配方：${agent.name}`, `Installed personal agent recipe: ${agent.name}`))
    } catch (err: any) {
      setError(err?.message || tx('安装 Agent 配方失败', 'Failed to install agent recipe'))
    } finally {
      setInstallingAgentSlug('')
    }
  }

  const handleCreateMcp = async () => {
    if (!selectedTeam) return
    const slug = normalizeSlugInput(newMcpSlug || newMcpName)
    const name = newMcpName.trim() || slug
    if (!slug || !name) {
      setError(tx('请填写 MCP 名称和 slug。', 'Enter an MCP name and slug.'))
      return
    }
    setMcpWorking(true)
    setMessage('')
    setError('')
    try {
      const args = newMcpArgs.split(',').map(s => s.trim()).filter(Boolean)
      const env: Record<string, string> = {}
      newMcpEnv.split('\n').forEach(line => {
        const parts = line.split('=')
        if (parts.length >= 2) {
          env[parts[0].trim()] = parts.slice(1).join('=').trim()
        }
      })
      const headers: Record<string, string> = {}
      newMcpHeaders.split('\n').forEach(line => {
        const parts = line.split(':')
        if (parts.length >= 2) {
          headers[parts[0].trim()] = parts.slice(1).join(':').trim()
        }
      })

      const tags = newMcpTags.split(',').map(s => s.trim()).filter(Boolean)

      const response = await api.saveTeamMcp(selectedTeam.id, {
        slug,
        name,
        description: newMcpDescription,
        transport: newMcpTransport,
        command: newMcpCommand,
        args: args.length > 0 ? args : undefined,
        env: Object.keys(env).length > 0 ? env : undefined,
        url: newMcpURL || undefined,
        headers: Object.keys(headers).length > 0 ? headers : undefined,
        status: selectedTeam.can_manage_members ? 'published' : 'draft',
        visibility: selectedTeam.can_manage_members ? 'team' : 'private',
        tags: tags.length > 0 ? tags : undefined,
      })
      setTeamMcps(response.data.mcps || [])
      if (!selectedTeam.can_manage_members) {
        await api.submitTeamMcpReview(selectedTeam.id, slug, 'Created by a team member.')
      }
      setNewMcpName('')
      setNewMcpSlug('')
      setNewMcpDescription('')
      setNewMcpCommand('')
      setNewMcpTags('')
      setNewMcpArgs('')
      setNewMcpEnv('')
      setNewMcpURL('')
      setNewMcpHeaders('')
      closeTeamDialog()
      setMessage(selectedTeam.can_manage_members
        ? tx('团队 MCP 已发布。', 'Team MCP configuration published.')
        : tx('团队 MCP 已保存为草稿，等待管理员发布。', 'Team MCP saved as draft for an admin to publish.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('保存团队 MCP 失败', 'Failed to save team MCP'))
    } finally {
      setMcpWorking(false)
    }
  }

  const handlePublishMcp = async (mcp: TeamMcpAsset, status: 'draft' | 'published' | 'archived') => {
    if (!selectedTeam) return
    setMcpWorking(true)
    setMessage('')
    setError('')
    try {
      const response = await api.saveTeamMcp(selectedTeam.id, {
        slug: mcp.slug,
        name: mcp.name,
        description: mcp.description,
        transport: mcp.transport,
        command: mcp.command,
        args: mcp.args,
        env: mcp.env,
        url: mcp.url,
        headers: mcp.headers,
        status,
        visibility: status === 'published' ? 'team' : 'private',
      })
      setTeamMcps(response.data.mcps || [])
      setMessage(status === 'published'
        ? tx('团队 MCP 已发布。', 'Team MCP published.')
        : status === 'archived'
          ? tx('团队 MCP 已归档。', 'Team MCP archived.')
          : tx('团队 MCP 已转为草稿。', 'Team MCP moved to draft.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('更新团队 MCP 失败', 'Failed to update team MCP'))
    } finally {
      setMcpWorking(false)
    }
  }

  const handleRequestMcpReview = async (mcp: TeamMcpAsset) => {
    if (!selectedTeam) return
    setMcpWorking(true)
    setMessage('')
    setError('')
    try {
      await api.submitTeamMcpReview(selectedTeam.id, mcp.slug, 'Request review.')
      setMessage(tx('团队 MCP 已提交审查。', 'Team MCP review requested.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('提交团队 MCP 审查失败', 'Failed to request team MCP review'))
    } finally {
      setMcpWorking(false)
    }
  }

  const handleResolveMcpReview = async (mcp: TeamMcpAsset, decision: 'approved' | 'rejected') => {
    if (!selectedTeam) return
    setMcpWorking(true)
    setMessage('')
    setError('')
    try {
      await api.resolveTeamMcpReview(selectedTeam.id, mcp.slug, decision === 'approved' ? 'approve' : 'reject')
      setMessage(decision === 'approved'
        ? tx('团队 MCP 审查已通过并发布。', 'Team MCP review approved.')
        : tx('已拒绝团队 MCP 审查。', 'Team MCP review rejected.'))
      await loadTeamData(selectedTeam)
    } catch (err: any) {
      setError(err?.message || tx('处理团队 MCP 审查失败', 'Failed to resolve team MCP review'))
    } finally {
      setMcpWorking(false)
    }
  }

  const isLocalTeamSimulation = !currentUser || !currentUser.slug || currentUser.slug === 'local'

  return (
    <div className="materials-page team-library-page">
      <section className="materials-hero team-library-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">{tx('团队资料', 'Team Library')}</div>
          <h2 className="materials-title">{tx('团队 AI 资料库', 'Team AI Library')}</h2>
          <p className="materials-subtitle">
            {tx(
              '个人账号保留自己的 Skill；小团队可共享 Skill、MCP 配置说明、提示词和使用技巧。',
              'Personal skills stay private; small teams can share skills, MCP configuration notes, prompts, and playbooks.',
            )}
          </p>
        </div>
        <div className="materials-actions">
          {teams.length > 0 && (
            <CustomSelect
              value={selectedTeam?.id || ''}
              onChange={(val) => setSelectedTeamID(val)}
              options={teams.map((team) => ({
                value: team.id,
                label: `${team.name} / ${team.slug}`
              }))}
              ariaLabel={tx('选择团队', 'Select team')}
            />
          )}
          <button className="btn btn-primary" type="button" onClick={handleCreateTemplate} disabled={working || !selectedTeam}>
            {working ? tx('处理中...', 'Working...') : tx('创建目录模板', 'Create folders')}
          </button>
          <button className="btn" type="button" onClick={() => setTeamDialog('create-team')}>
            {tx('新团队', 'New team')}
          </button>
        </div>
      </section>

      {message && <div className="alert alert-success">{message}</div>}
      {error && <div className="alert alert-warn">{error}</div>}
      {isLocalTeamSimulation && (
        <div className="alert alert-warn">
          {tx(
            '当前是本地团队模拟：团队、成员、Skill 和审查记录只保存在这台设备的本地数据里，适合演示和离线验证；真实多人同步仍需要云端账号。',
            'Local team simulation is active. Teams, members, skills, and review records are stored only on this device for demos and offline testing. Real multi-user sync still needs a cloud account.',
          )}
        </div>
      )}

      <nav className="team-page-tabs" aria-label={tx('团队页导航', 'Team page navigation')}>
        {teamTabs.map((item) => (
          <button
            key={item.id}
            type="button"
            className={teamView === item.id ? 'is-active' : ''}
            aria-current={teamView === item.id ? 'page' : undefined}
            onClick={() => setTeamView(item.id)}
          >
            {item.label}
          </button>
        ))}
      </nav>

      <div className="team-tab-panel" data-view={teamView}>
      {teamView === 'overview' && selectedTeam && (
        <section className="team-overview-panel" id="team-overview">
          <div>
            <span>{tx('当前团队', 'Current team')}</span>
            <strong>{selectedTeam.name}</strong>
            <small>/{selectedTeam.slug}</small>
          </div>
          <div className="team-overview-stats">
            {overviewStats.map((item) => (
              <div key={item.label} className="team-overview-stat">
                <span>{item.label}</span>
                <strong>{item.value}</strong>
                <small>{item.detail}</small>
              </div>
            ))}
          </div>
        </section>
      )}

      {teamView === 'overview' && (
      <section className="materials-section" id="team-setup">
        <div className="materials-section-head">
          <div>
            <h3 className="materials-section-title">{tx('团队', 'Teams')}</h3>
            <p className="materials-section-copy">
              {tx('创建团队后，把同事加入对应资料库。当前阶段用于共享资料，不作为企业级组织管理后台。', 'Create a team, then add teammates to the shared library. This phase is for shared material, not enterprise org administration.')}
            </p>
          </div>
        </div>

        <div className="team-compact-list team-list">
          {teams.map((team) => (
            <button
              key={team.id}
              className={`team-compact-row team-select-row${selectedTeam?.id === team.id ? ' is-selected' : ''}`}
              type="button"
              onClick={() => setSelectedTeamID(team.id)}
            >
              <span className="materials-tile-icon icon-stack" aria-hidden="true" />
              <div>
                <strong>{team.name}</strong>
                <small>/{team.slug}{team.description ? ` / ${team.description}` : ''}</small>
              </div>
              <span className="team-row-meta">
                <span className="materials-tile-pill">{roleLabel(team.role, locale)}</span>
                <span className="materials-tile-pill">{team.can_write ? tx('可写入', 'Can write') : tx('只读', 'Read only')}</span>
              </span>
            </button>
          ))}
          {teams.length === 0 ? (
            <button className="team-action-row" type="button" onClick={() => setTeamDialog('create-team')}>
              <span>{tx('还没有团队', 'No teams yet')}</span>
              <strong>{tx('创建第一个团队资料库', 'Create the first team library')}</strong>
            </button>
          ) : null}
        </div>
      </section>
      )}

      {teamView === 'overview' && selectedTeam && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('成员', 'Members')}</h3>
              <p className="materials-section-copy">
                {tx('Owner/Admin 管理成员并处理审查；Member 可写团队资料并提交审查；Viewer 只读。', 'Owners/Admins manage members and reviews; Members can write team files and request reviews; Viewers are read-only.')}
              </p>
            </div>
            {selectedTeam.can_manage_members ? (
              <div className="materials-actions">
                <button className="btn btn-sm btn-primary" type="button" onClick={() => setTeamDialog('add-member')}>
                  {tx('添加成员', 'Add member')}
                </button>
              </div>
            ) : null}
          </div>

          <div className="team-compact-list">
            {members.map((member) => (
              <div
                key={member.user_id}
                className="team-compact-row"
              >
                <span className="materials-tile-icon icon-file" aria-hidden="true" />
                <div>
                  <strong>{member.display_name || member.user_slug}</strong>
                  <small>@{member.user_slug}{member.email ? ` / ${member.email}` : ''}</small>
                </div>
                <span className="materials-tile-pill">{roleLabel(member.role, locale)}</span>
              </div>
            ))}
            {members.length === 0 ? (
              <div className="team-admin-empty">{tx('暂无成员。', 'No members yet.')}</div>
            ) : null}
          </div>
        </section>
      )}

      {teamView === 'assets' && selectedTeam && (
        <section className="team-assets-workspace" id="team-assets">
          <aside className="team-assets-rail" aria-label={tx('团队资料目录', 'Team library')}>
            <div className="team-assets-rail-head">
              <strong>{tx('团队资料', 'Team library')}</strong>
              <span>{selectedTeam.name}</span>
            </div>
            <div className="team-assets-rail-list">
              {categories.map((category) => (
                <div key={category.subtitle} className="team-assets-rail-item">
                  <span className={`materials-tile-icon ${category.icon}`} aria-hidden="true" />
                  <div>
                    <strong>{category.title}</strong>
                    <small>{category.subtitle}</small>
                  </div>
                  <em>{loading ? tx('读取中', 'Loading') : formatCategoryCount(category.count, locale)}</em>
                  {category.action}
                </div>
              ))}
            </div>
          </aside>

          <div className="team-assets-main">
            <div className="team-assets-main-head">
              <div>
                <h3>{tx('共享 Skill 与 MCP', 'Shared Skills and MCP')}</h3>
                <p>{tx('团队协作的主路径只保留 Skill、MCP、本机同步和审查状态。其他资料放在左侧目录里。', 'The team workflow focuses on Skill, MCP, local sync, and review state. Other material stays in the directory rail.')}</p>
              </div>
              <div className="team-assets-head-actions">
                <Link className="btn btn-sm" to={selectedTeam ? `/data/skills?team=${encodeURIComponent(selectedTeam.id)}` : '/data/skills'}>
                  {tx('管理 Skill', 'Manage skills')}
                </Link>
                {selectedTeam.can_write ? (
                  <button className="btn btn-sm btn-primary" type="button" onClick={() => setTeamDialog('create-mcp')}>
                    {tx('新 MCP', 'New MCP')}
                  </button>
                ) : null}
              </div>
            </div>

            <div className="team-assets-summary-grid">
              <div className="team-assets-summary-item is-primary">
                <span>Skill</span>
                <strong>{childCount(skillsRoot)}</strong>
                <p>{tx(`${skillPublications.length} 个共享发布，${subscriptions.length} 个已装配`, `${skillPublications.length} shared publications, ${subscriptions.length} installed`)}</p>
              </div>
              <div className="team-assets-summary-item">
                <span>MCP</span>
                <strong>{teamMcps.length}</strong>
                <p>{tx('发布后可同步到 Codex 和 Claude Code。', 'Published items can sync to Codex and Claude Code.')}</p>
              </div>
              <div className="team-assets-summary-item">
                <span>{tx('待审查', 'Review')}</span>
                <strong>{reviewEventCount}</strong>
                <p>{skillReviewHistory[0]
                  ? `${skillReviewHistory[0].skill_path || skillReviewHistory[0].agent_slug || skillReviewHistory[0].asset_type} / ${reviewActionLabel(skillReviewHistory[0].action, locale)}`
                  : tx('暂无审查记录。', 'No review history yet.')}
                </p>
              </div>
            </div>
          </div>

          <aside className="team-assets-side">
            <div className="team-assets-side-block">
              <strong>{tx('本机同步', 'Local sync')}</strong>
              <p>{tx('刷新后，已发布的团队 MCP 和 Skill 会进入本机工具链。', 'After refresh, published team MCP and Skills enter the local toolchain.')}</p>
              <div className="team-assets-side-actions">
                <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'codex'} onClick={() => { void handleRefreshLocalPlatform('codex') }}>
                  {platformRefreshBusy === 'codex' ? tx('刷新中...', 'Refreshing...') : 'Codex'}
                </button>
                <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'claude-code'} onClick={() => { void handleRefreshLocalPlatform('claude-code') }}>
                  {platformRefreshBusy === 'claude-code' ? tx('刷新中...', 'Refreshing...') : 'Claude Code'}
                </button>
              </div>
            </div>
            {selectedTeam.can_manage_members ? (
              <div className="team-assets-side-block">
                <strong>{tx('订阅更新', 'Subscription updates')}</strong>
                <p>{tx(`待更新 ${subscriptionSummary.updates}，未安装 ${subscriptionSummary.notInstalled}，提醒 ${skillUpdateNotifications.length}。`, `${subscriptionSummary.updates} updates, ${subscriptionSummary.notInstalled} missing, ${skillUpdateNotifications.length} notices.`)}</p>
                <button className="btn btn-sm btn-primary" type="button" disabled={sharingWorking} onClick={() => { void handleCheckTeamSkillSubscriptions() }}>
                  {sharingWorking ? tx('检查中...', 'Checking...') : tx('检查更新', 'Check updates')}
                </button>
              </div>
            ) : null}
          </aside>
        </section>
      )}

      {teamView === 'sync' && selectedTeam && (
        <section className="materials-section" id="team-sync">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('本机同步入口', 'Local Sync')}</h3>
              <p className="materials-section-copy">
                {tx('Codex 和 Claude Code 可直接刷新本机连接；团队 Skill 也可先预览再应用。Cursor 和 Gemini CLI 继续只导出。', 'Codex and Claude Code can refresh local connections directly; team skills can also be previewed before applying. Cursor and Gemini CLI remain export-only.')}
              </p>
            </div>
            <div className="materials-actions">
              <button className="btn btn-sm" type="button" disabled={Boolean(teamSkillSyncBusy)} onClick={() => { void handleTeamSkillSync('preview') }}>
                {teamSkillSyncBusy === 'preview' ? tx('预览中...', 'Previewing...') : tx('预览 Skill', 'Preview skills')}
              </button>
              <button className="btn btn-sm btn-primary" type="button" disabled={Boolean(teamSkillSyncBusy)} onClick={() => { void handleTeamSkillSync('apply') }}>
                {teamSkillSyncBusy === 'apply' ? tx('应用中...', 'Applying...') : tx('应用到本机', 'Apply locally')}
              </button>
              <button className="btn btn-sm" type="button" disabled={teamSkillExportBusy === 'cursor'} onClick={() => { void handleTeamSkillExport('cursor') }}>
                {teamSkillExportBusy === 'cursor' ? tx('生成中...', 'Creating...') : tx('导出 Cursor', 'Export Cursor')}
              </button>
              <button className="btn btn-sm" type="button" disabled={teamSkillExportBusy === 'gemini-cli'} onClick={() => { void handleTeamSkillExport('gemini-cli') }}>
                {teamSkillExportBusy === 'gemini-cli' ? tx('生成中...', 'Creating...') : tx('导出 Gemini', 'Export Gemini')}
              </button>
            </div>
          </div>

          <div className="local-tool-status-panel">
            <div className="local-tool-status-grid">
              {localToolStatusLoading ? (
                <div className="local-tool-status-card">
                  <div className="local-tool-status-card-head">
                    <strong>{tx('正在检测本机工具', 'Checking local tools')}</strong>
                    <span className="materials-tile-pill">{tx('检测中', 'Checking')}</span>
                  </div>
                  <p>{tx('Vola 正在读取本机连接状态和同步能力。', 'Vola is reading local connection status and sync capabilities.')}</p>
                </div>
              ) : localToolPlatformOrder.map((platformID) => {
                const platform = localToolPlatformMap[platformID]
                if (!platform) {
                  return (
                    <div key={platformID} className="local-tool-status-card">
                      <div className="local-tool-status-card-head">
                        <strong>{localToolPlatformLabel(platformID)}</strong>
                        <span className="materials-tile-pill team-skill-publication-pill is-draft">{tx('未检测', 'Unknown')}</span>
                      </div>
                      <p>{tx('当前环境没有返回这个工具的状态。', 'No status was returned for this tool in the current environment.')}</p>
                    </div>
                  )
                }
                return (
                  <div key={platform.id} className="local-tool-status-card">
                    <div className="local-tool-status-card-head">
                      <strong>{platform.name || localToolPlatformLabel(platform.id)}</strong>
                      <span className={`materials-tile-pill ${platform.auto_sync_supported ? 'team-skill-publication-pill is-published' : 'team-skill-publication-pill is-draft'}`}>
                        {localToolSyncModeLabel(platform)}
                      </span>
                    </div>
                    <div className="local-tool-status-line">
                      <span>{platform.status_label}</span>
                      <small>{platform.next_action}</small>
                    </div>
                    {platform.config_path ? <code>{platform.config_path}</code> : null}
                    {platform.entrypoint_path ? <code>{platform.entrypoint_path}</code> : null}
                    {platform.reasons?.length ? (
                      <ul className="local-tool-status-notes">
                        {platform.reasons.map((reason) => <li key={reason}>{reason}</li>)}
                      </ul>
                    ) : null}
                  </div>
                )
              })}
            </div>
          </div>

          {teamSkillSyncPreview && (
            <div className="alert alert-success">{teamSkillSyncSummary}</div>
          )}
          {teamSkillSyncPreview && teamLocalSyncTotal.manual > 0 && (
            <div className="alert alert-warn">
              {tx(`有 ${teamLocalSyncTotal.manual} 个条目需要人工确认后再应用。`, `${teamLocalSyncTotal.manual} items need manual review before applying.`)}
            </div>
          )}
          {teamSkillSyncPreview && teamLocalSyncTotal.conflict > 0 && (
            <div className="alert alert-warn">
              {tx('本机已有同名但未由 Vola 管理的目录，刷新不会覆盖它们。', 'Existing non-Vola-managed local folders will not be overwritten.')}
            </div>
          )}
          <div className="materials-section-copy">
            {tx('本机同步只会修改当前机器的 Vola 托管目录或客户端配置；如果当前会话没有本机管理权限，系统会拒绝写入。', 'Local sync only changes Vola-managed directories or client config on this machine; without local admin access, the API refuses the write.')}
          </div>

          {teamSkillSyncPreview && (
            <div className="data-sync-preview skill-local-sync-preview">
              <div className="data-sync-preview-sections">
                {teamSkillSyncPreview.agents.map((agent) => (
                  <details
                    key={agent.agent_id}
                    className="data-sync-preview-section"
                    open={agent.summary.add > 0 || agent.summary.update > 0 || agent.summary.conflict > 0 || agent.summary.export > 0}
                  >
                    <summary className="data-sync-preview-summary">
                      <span>
                        <strong>{agent.name}</strong>
                        <small>{agent.target_root || agent.export_file_name || agent.support_status}</small>
                      </span>
                      <span>{syncAgentSummaryText(agent, tx)}</span>
                    </summary>
                    {agent.message ? <p className="dashboard-empty-copy">{agent.message}</p> : null}
                    {agent.auto_apply_reason ? <p className="dashboard-empty-copy">{agent.auto_apply_reason}</p> : null}
                    {agent.detected_roots?.length ? (
                      <div className="data-sync-preview-list">
                        {agent.detected_roots.map((root, index) => (
                          <div key={`${agent.agent_id}-root-${index}`} className="data-sync-preview-entry">
                            <span className={root.exists ? 'preview-action preview-action-update' : 'preview-action preview-action-skip'}>
                              {root.exists ? tx('已发现', 'found') : tx('未发现', 'missing')}
                            </span>
                            <span className="skill-local-sync-path">{root.path}</span>
                            <small>{root.role || root.message || ''}</small>
                          </div>
                        ))}
                      </div>
                    ) : null}
                    {agent.changes.length === 0 ? (
                      <p className="dashboard-empty-copy">{tx('没有需要处理的变化。', 'No changes to process.')}</p>
                    ) : (
                      <div className="data-sync-preview-list">
                        {agent.changes.filter((change) => change.action !== 'marker').slice(0, 20).map((change, index) => (
                          <div key={`${agent.agent_id}-${change.target_path || change.skill_path}-${index}`} className={`data-sync-preview-entry ${change.action === 'conflict' ? 'is-danger' : ''}`}>
                            <span className={syncActionClass(change.action)}>{syncActionLabel(change.action, tx)}</span>
                            <span className="skill-local-sync-path">
                              {change.skill_path}{change.rel_path ? `/${change.rel_path}` : ''}
                            </span>
                            <small>{change.target_path || change.reason || agent.export_file_name || ''}</small>
                          </div>
                        ))}
                        {agent.changes.filter((change) => change.action !== 'marker').length > 20 ? (
                          <p className="dashboard-empty-copy">{tx('只显示前 20 条变化。', 'Showing the first 20 changes only.')}</p>
                        ) : null}
                      </div>
                    )}
                    {agent.export_available ? (
                      <div className="materials-actions">
                        <button className="btn btn-sm" type="button" disabled={Boolean(teamSkillExportBusy)} onClick={() => { void handleTeamSkillExport(agent.agent_id) }}>
                          {teamSkillExportBusy === agent.agent_id ? tx('生成中...', 'Creating...') : tx('下载导出包', 'Download export')}
                        </button>
                      </div>
                    ) : null}
                  </details>
                ))}
              </div>
            </div>
          )}
        </section>
      )}

      {teamView === 'assets' && selectedTeam && (
        <section className="materials-section team-assets-list-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('团队 Skills 共享', 'Team Skills Sharing')}</h3>
              <p className="materials-section-copy">
                {tx('团队内共享的 AI 技能 (Skills) 列表。管理员可设置全员共享与审核推荐，成员可一键快速装配到本地。', 'Shared AI skills in the team. Admins can share to all and review recommendations, and members can install with one click.')}
              </p>
            </div>
            <div className="materials-actions">
              <Link className="btn btn-sm" to={selectedTeam ? `/data/skills?team=${encodeURIComponent(selectedTeam.id)}` : '/data/skills'}>
                {tx('管理 Skill', 'Manage skills')}
              </Link>
            </div>
          </div>

          <div className="team-asset-list team-skill-row-list">
            {skillPublications.map((pub) => {
              const sub = subscriptions.find(s => s.source_path === pub.skill_path)
              const isInstalled = !!sub
              const hasUpdate = sub?.update_available
              const reviewLabel = publicationReviewLabel(pub, tx)
              const draft = skillPublicationDrafts[pub.skill_path] || {
                version: pub.version || '',
                release_note: pub.release_note || '',
                note: pub.note || '',
              }

              return (
                <div key={pub.skill_path} className="team-asset-row team-skill-row">
                  <span className="materials-tile-icon icon-stack" aria-hidden="true" />
                  <div className="team-asset-row-main">
                    <strong>{pub.skill_path.replace(/^\/skills\//, '')}</strong>
                    <small>{pub.skill_path}</small>
                    <p>{pub.release_note || pub.note || tx('团队共享技能', 'Shared team skill')}</p>
                  </div>
                  <div className="team-asset-row-meta">
                    <span className={`materials-tile-pill team-skill-publication-pill is-${pub.status}`}>
                      {publicationStatusLabel(pub, tx)}
                    </span>
                    {reviewLabel ? (
                      <span className={`materials-tile-pill team-skill-review-pill is-${pub.review_status || 'none'}`}>
                        {reviewLabel}
                      </span>
                    ) : null}
                    {pub.version ? (
                      <span className="materials-tile-pill team-skill-version-pill">{pub.version}</span>
                    ) : null}
                    {isInstalled ? (
                      <span className="materials-tile-pill team-skill-install-pill is-installed">{tx('已装配', 'Installed')}</span>
                    ) : (
                      <span className="materials-tile-pill team-skill-install-pill is-not-installed">{tx('未装配', 'Not Installed')}</span>
                    )}
                    {selectedTeam.can_manage_members ? (
                      <label className="team-inline-toggle">
                        <input
                          type="checkbox"
                          checked={pub.visibility === 'team'}
                          disabled={sharingWorking}
                          onChange={async (e) => {
                            setSharingWorking(true)
                            try {
                              await api.saveTeamSkillPublication(selectedTeam.id, publicationSavePayload(pub, {
                                visibility: e.target.checked ? 'team' : 'private',
                                ...draft,
                              }))
                              setMessage(tx('共享设置已更新', 'Sharing settings updated'))
                              await refreshSkillPublications()
                            } catch (err: any) {
                              setError(err?.message || tx('更新共享状态失败', 'Failed to update sharing status'))
                            } finally {
                              setSharingWorking(false)
                            }
                          }}
                        />
                        {tx('全员共享', 'Share with all')}
                      </label>
                    ) : (
                      pub.visibility === 'team' ? (
                        <span className="team-inline-status is-visible">{tx('全员可见', 'Visible to all')}</span>
                      ) : pub.review_status === 'requested' ? (
                        <span className="team-inline-status">{tx('待审查', 'Review requested')}</span>
                      ) : (
                        <span className="team-inline-status">{tx('推荐中', 'Recommended')}</span>
                      )
                    )}
                  </div>
                  <div className="team-asset-row-actions">
                    {selectedTeam.can_manage_members ? (
                      <button
                        className="btn btn-sm"
                        type="button"
                        onClick={() => openSkillPublicationDialog(pub.skill_path)}
                      >
                        {tx('发布信息', 'Details')}
                      </button>
                    ) : null}
                    {!isInstalled ? (
                      <button
                        className="btn btn-sm btn-primary"
                        type="button"
                        disabled={sharingWorking}
                        onClick={async () => {
                          setSharingWorking(true)
                          try {
                            await api.copyTeamSkillToPersonal(selectedTeam.id, pub.skill_path)
                            setMessage(tx('技能装配成功！', 'Skill installed successfully!'))
                            const res = await api.getTeamSkillSubscriptions(selectedTeam.id)
                            setSubscriptions(res.subscriptions || [])
                          } catch (err: any) {
                            setError(err?.message || tx('装配技能失败', 'Failed to install skill'))
                          } finally {
                            setSharingWorking(false)
                          }
                        }}
                      >
                        + {tx('快速装配', 'Install')}
                      </button>
                    ) : hasUpdate ? (
                      <button
                        className="btn btn-sm btn-primary"
                        type="button"
                        disabled={sharingWorking}
                        onClick={async () => {
                          setSharingWorking(true)
                          try {
                            await api.copyTeamSkillToPersonal(selectedTeam.id, pub.skill_path, undefined, true)
                            setMessage(tx('技能已同步到最新版！', 'Skill updated successfully!'))
                            const res = await api.getTeamSkillSubscriptions(selectedTeam.id)
                            setSubscriptions(res.subscriptions || [])
                          } catch (err: any) {
                            setError(err?.message || tx('更新同步失败', 'Failed to update skill'))
                          } finally {
                            setSharingWorking(false)
                          }
                        }}
                      >
                        {tx('一键同步', 'Sync Update')}
                      </button>
                    ) : null}

                    {(!selectedTeam.can_manage_members && pub.review_status !== 'requested' && pub.visibility !== 'team') && (
                      <button
                        className="btn btn-sm"
                        type="button"
                        disabled={sharingWorking}
                        onClick={async () => {
                          setSharingWorking(true)
                          try {
                            await api.requestTeamSkillReview(selectedTeam.id, {
                              asset_type: 'skill',
                              skill_path: pub.skill_path,
                              note: 'Recommended by team member.'
                            })
                            setMessage(tx('推荐成功，已提交管理员审核！', 'Recommended successfully, submitted to admin for review.'))
                            await refreshSkillPublications()
                          } catch (err: any) {
                            setError(err?.message || tx('提交推荐失败', 'Failed to recommend'))
                          } finally {
                            setSharingWorking(false)
                          }
                        }}
                      >
                        {tx('推荐', 'Recommend')}
                      </button>
                    )}

                    {selectedTeam.can_manage_members && pub.review_status === 'requested' ? (
                      <>
                        <button
                          className="btn btn-sm btn-primary"
                          type="button"
                          disabled={sharingWorking}
                          onClick={async () => {
                            setSharingWorking(true)
                            try {
                              await api.resolveTeamSkillReview(selectedTeam.id, {
                                asset_type: 'skill',
                                skill_path: pub.skill_path,
                                decision: 'approved',
                                note: 'Approved by admin.'
                              })
                              await api.saveTeamSkillPublication(selectedTeam.id, publicationSavePayload(pub, {
                                status: 'published',
                                visibility: 'team',
                                ...draft,
                              }))
                              setMessage(tx('已通过审查并全员共享！', 'Approved and shared with all!'))
                              await refreshSkillPublications()
                            } catch (err: any) {
                              setError(err?.message || tx('审核操作失败', 'Failed to resolve review'))
                            } finally {
                              setSharingWorking(false)
                            }
                          }}
                        >
                          {tx('通过审查', 'Approve')}
                        </button>
                        <button
                          className="btn btn-sm"
                          type="button"
                          disabled={sharingWorking}
                          onClick={async () => {
                            setSharingWorking(true)
                            try {
                              await api.resolveTeamSkillReview(selectedTeam.id, {
                                asset_type: 'skill',
                                skill_path: pub.skill_path,
                                decision: 'changes_requested',
                                note: 'Changes requested by admin.'
                              })
                              setMessage(tx('已要求修改。', 'Changes requested.'))
                              await refreshSkillPublications()
                            } catch (err: any) {
                              setError(err?.message || tx('审核操作失败', 'Failed to resolve review'))
                            } finally {
                              setSharingWorking(false)
                            }
                          }}
                        >
                          {tx('要求修改', 'Request Changes')}
                        </button>
                      </>
                    ) : null}
                  </div>
                </div>
              )
            })}
            {skillPublications.length === 0 && (
              <div className="team-admin-empty">
                {tx('暂无团队共享的 Skills。', 'No shared team skills yet.')}
              </div>
            )}
          </div>
        </section>
      )}

      {teamView === 'assets' && selectedTeam && (
        <details className="team-secondary-assets">
          <summary>
            <span>
              <strong>{tx('Agent 配方', 'Agent recipes')}</strong>
              <small>{tx(`${teamAgents.length} 个配置，作为 Skill/MCP 之外的辅助资料。`, `${teamAgents.length} recipes, kept as supporting material outside Skill/MCP.`)}</small>
            </span>
            {selectedTeam.can_write ? (
              <button className="btn btn-sm" type="button" onClick={(event) => { event.preventDefault(); setTeamDialog('create-agent') }}>
                {tx('新 Agent', 'New agent')}
              </button>
            ) : null}
          </summary>

          <div className="team-asset-list team-secondary-list">
            {teamAgents.map((agent) => (
              <div
                key={agent.slug}
                className="team-asset-row"
              >
                <span className="materials-tile-icon icon-device" aria-hidden="true" />
                <div className="team-asset-row-main">
                  <strong>{agent.name}</strong>
                  <small>/{agent.slug}</small>
                  <p>{agent.description || agent.instructions || tx('团队 Agent 配方', 'Team agent recipe')}</p>
                </div>
                <div className="team-asset-row-meta">
                  <span className="materials-tile-pill">{agentStatusLabel(agent.status, agent.visibility, locale)}</span>
                  {selectedTeam.can_manage_members ? (
                    <label className="team-inline-toggle">
                      <input
                        type="checkbox"
                        checked={agent.visibility === 'team'}
                        disabled={agentWorking}
                        onChange={async (e) => {
                          setAgentWorking(true)
                          try {
                            await api.saveTeamAgent(selectedTeam.id, {
                              slug: agent.slug,
                              name: agent.name,
                              description: agent.description,
                              instructions: agent.instructions,
                              default_skill_paths: agent.default_skill_paths || [],
                              target_agents: agent.target_agents || [],
                              model: agent.model,
                              permissions: agent.permissions || [],
                              approval_required: agent.approval_required || [],
                              maintainer: agent.maintainer,
                              status: agent.status,
                              visibility: e.target.checked ? 'team' : 'private',
                            })
                            setMessage(tx('Agent 共享设置已更新', 'Agent sharing settings updated'))
                            const res = await api.getTeamAgents(selectedTeam.id)
                            setTeamAgents(res.agents || [])
                          } catch (err: any) {
                            setError(err?.message || tx('更新 Agent 共享状态失败', 'Failed to update agent sharing status'))
                          } finally {
                            setAgentWorking(false)
                          }
                        }}
                      />
                      {tx('全员共享', 'Share with all')}
                    </label>
                  ) : (
                    agent.visibility === 'team' ? (
                      <span className="team-inline-status is-visible">{tx('全员可见', 'Visible to all')}</span>
                    ) : agent.review_status === 'requested' ? (
                      <span className="team-inline-status">{tx('待审查', 'Review requested')}</span>
                    ) : (
                      <span className="team-inline-status">{tx('推荐中', 'Recommended')}</span>
                    )
                  )}
                </div>
                <div className="team-asset-row-actions">
                  {agent.status === 'published' && agent.visibility === 'team' ? (
                    <button className="btn btn-sm" type="button" disabled={installingAgentSlug === agent.slug} onClick={() => { void handleInstallAgent(agent) }}>
                      {installingAgentSlug === agent.slug ? tx('安装中...', 'Installing...') : `+ ${tx('快速装配', 'Install')}`}
                    </button>
                  ) : null}
                  {selectedTeam.can_write && !selectedTeam.can_manage_members && agent.review_status !== 'requested' && agent.visibility !== 'team' ? (
                    <button className="btn btn-sm" type="button" disabled={agentWorking} onClick={async () => {
                      setAgentWorking(true)
                      try {
                        await api.requestTeamSkillReview(selectedTeam.id, {
                          asset_type: 'agent',
                          agent_slug: agent.slug,
                          note: 'Recommended by team member.'
                        })
                        setMessage(tx('推荐成功，已提交管理员审核！', 'Recommended successfully, submitted to admin for review.'))
                        const res = await api.getTeamAgents(selectedTeam.id)
                        setTeamAgents(res.agents || [])
                      } catch (err: any) {
                        setError(err?.message || tx('提交推荐失败', 'Failed to recommend agent'))
                      } finally {
                        setAgentWorking(false)
                      }
                    }}>
                      {tx('推荐', 'Recommend')}
                    </button>
                  ) : null}
                  {selectedTeam.can_manage_members && agent.review_status === 'requested' ? (
                    <>
                      <button className="btn btn-sm btn-primary" type="button" disabled={agentWorking} onClick={() => { void handleResolveAgentReview(agent, 'approved') }}>
                        {tx('通过审查', 'Approve review')}
                      </button>
                      <button className="btn btn-sm" type="button" disabled={agentWorking} onClick={() => { void handleResolveAgentReview(agent, 'changes_requested') }}>
                        {tx('要求修改', 'Request changes')}
                      </button>
                    </>
                  ) : null}
                  {selectedTeam.can_manage_members ? (
                    agent.status === 'published' && agent.visibility === 'team' ? (
                      <button className="btn btn-sm" type="button" disabled={agentWorking} onClick={() => { void handlePublishAgent(agent, 'draft') }}>
                        {tx('转草稿', 'Draft')}
                      </button>
                    ) : (
                      <button className="btn btn-sm btn-primary" type="button" disabled={agentWorking} onClick={() => { void handlePublishAgent(agent, 'published') }}>
                        {tx('发布', 'Publish')}
                      </button>
                    )
                  ) : null}
                  {selectedTeam.can_manage_members && agent.status !== 'archived' ? (
                      <button className="btn btn-sm materials-toolbar-control is-danger" type="button" disabled={agentWorking} onClick={() => { void handlePublishAgent(agent, 'archived') }}>
                        {tx('归档', 'Archive')}
                      </button>
                    ) : null}
                </div>
              </div>
            ))}
            {teamAgents.length === 0 ? (
              <div className="team-admin-empty">{tx('暂无团队 Agent 配方。', 'No team agent recipes yet.')}</div>
            ) : null}
          </div>
        </details>
      )}

      {teamView === 'assets' && selectedTeam && (
        <section className="materials-section team-assets-list-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('团队共享 MCP 配置', 'Team MCP Configurations')}</h3>
              <p className="materials-section-copy">
                {tx('将数据库、服务 API 或专用工具包装为团队共享 MCP。Codex 和 Claude Code 刷新本机连接后会同步拉起已发布条目，Cursor 和 Gemini CLI 仍按导出或手动方式处理。', 'Share databases, service APIs, or custom tools as team MCPs. Codex and Claude Code sync published items after a local connection refresh, while Cursor and Gemini CLI stay export-only or manual.')}
              </p>
            </div>
            <div className="materials-actions">
              {selectedTeam.can_write ? (
                <button className="btn btn-sm btn-primary" type="button" onClick={() => setTeamDialog('create-mcp')}>
                  {tx('新 MCP', 'New MCP')}
                </button>
              ) : null}
              <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'codex'} onClick={() => { void handleRefreshLocalPlatform('codex') }}>
                {platformRefreshBusy === 'codex' ? tx('刷新中...', 'Refreshing...') : tx('同步到 Codex', 'Sync to Codex')}
              </button>
              <button className="btn btn-sm" type="button" disabled={platformRefreshBusy === 'claude-code'} onClick={() => { void handleRefreshLocalPlatform('claude-code') }}>
                {platformRefreshBusy === 'claude-code' ? tx('刷新中...', 'Refreshing...') : tx('同步到 Claude Code', 'Sync to Claude Code')}
              </button>
            </div>
          </div>

          <div className="team-asset-list">
            {teamMcps.map((mcp) => {
              const health = mcpHealth[`team-mcp-${mcp.slug}`]
              return (
                <div key={mcp.slug} className="team-asset-row">
                  <span className="materials-tile-icon icon-gear" aria-hidden="true" />
                  <div className="team-asset-row-main">
                    <strong>{mcp.name}</strong>
                    <small>
                      /{mcp.slug} / {mcp.transport}
                      {mcp.transport === 'http' && health ? ` / ${health.status === 'online' ? `Online ${health.latency_ms}ms` : 'Offline'}` : ''}
                    </small>
                    <p>{mcp.description || (mcp.transport === 'stdio' ? `${mcp.command} ${mcp.args?.join(' ')}` : mcp.url)}</p>
                  </div>
                  <div className="team-asset-row-meta">
                    <span className="materials-tile-pill">{agentStatusLabel(mcp.status, mcp.visibility, locale)}</span>
                    <span className={`materials-tile-pill ${mcp.review_status === 'requested' ? 'team-skill-review-pill is-requested' : mcp.review_status === 'rejected' ? 'team-skill-review-pill is-changes_requested' : ''}`}>
                      {mcp.review_status === 'requested'
                        ? tx('待审查', 'Review requested')
                        : mcp.review_status === 'rejected'
                          ? tx('需修改', 'Changes requested')
                          : tx('本机可用', 'Local ready')}
                    </span>
                  </div>
                  <div className="team-asset-row-actions">
                    {selectedTeam.can_write && !selectedTeam.can_manage_members ? (
                      <button className="btn btn-sm" type="button" disabled={mcpWorking || mcp.review_status === 'requested'} onClick={() => { void handleRequestMcpReview(mcp) }}>
                        {mcp.review_status === 'requested' ? tx('待审查', 'Review requested') : tx('提交审查', 'Request review')}
                      </button>
                    ) : null}
                    {selectedTeam.can_manage_members && mcp.review_status === 'requested' ? (
                      <>
                        <button className="btn btn-sm btn-primary" type="button" disabled={mcpWorking} onClick={() => { void handleResolveMcpReview(mcp, 'approved') }}>
                          {tx('通过审查', 'Approve review')}
                        </button>
                        <button className="btn btn-sm" type="button" disabled={mcpWorking} onClick={() => { void handleResolveMcpReview(mcp, 'rejected') }}>
                          {tx('要求修改', 'Request changes')}
                        </button>
                      </>
                    ) : null}
                    {selectedTeam.can_manage_members ? (
                      mcp.status === 'published' && mcp.visibility === 'team' ? (
                        <button className="btn btn-sm" type="button" disabled={mcpWorking} onClick={() => { void handlePublishMcp(mcp, 'draft') }}>
                          {tx('转草稿', 'Draft')}
                        </button>
                      ) : (
                        <button className="btn btn-sm btn-primary" type="button" disabled={mcpWorking} onClick={() => { void handlePublishMcp(mcp, 'published') }}>
                          {tx('发布', 'Publish')}
                        </button>
                      )
                    ) : null}
                    {selectedTeam.can_manage_members && mcp.status !== 'archived' ? (
                      <button className="btn btn-sm materials-toolbar-control is-danger" type="button" disabled={mcpWorking} onClick={() => { void handlePublishMcp(mcp, 'archived') }}>
                        {tx('归档', 'Archive')}
                      </button>
                    ) : null}
                  </div>
                </div>
              )
            })}
            {teamMcps.length === 0 ? (
              <div className="team-admin-empty">{tx('暂无团队共享 MCP。', 'No shared team MCP yet.')}</div>
            ) : null}
          </div>
        </section>
      )}

      {teamView === 'files' && (!selectedTeam ? (
        <section className="dashboard-file-panel github-tree-list" id="team-files">
          <div className="dashboard-section-head">
            <div>
              <h3>{tx('团队文件', 'Team Files')}</h3>
              <p>{tx('还没有团队。创建一个团队后再添加资料。', 'No team yet. Create a team to add shared material.')}</p>
            </div>
          </div>
          <div className="dashboard-file-empty">{tx('暂无团队文件。', 'No team files yet.')}</div>
        </section>
      ) : !loading && !teamRoot ? (
        <section className="dashboard-file-panel github-tree-list" id="team-files">
          <div className="dashboard-section-head">
            <div>
              <h3>{tx('团队文件', 'Team Files')}</h3>
              <p>{tx('团队资料目录尚未创建，点击上方按钮生成 /team/mcp、/team/prompts 和 /team/playbooks。', 'The team library folders have not been created yet. Use the button above to create /team/mcp, /team/prompts, and /team/playbooks.')}</p>
            </div>
          </div>
          <div className="dashboard-file-empty">{tx('暂无团队文件。', 'No team files yet.')}</div>
        </section>
      ) : (
        <div id="team-files">
          <GitHubTreeList
            rootPath={currentTreeRoot}
            path={treePath}
            rootLabel={currentTreeRoot === '/skills' ? tx('团队 Skill', 'Team Skills') : tx('团队资料', 'Team')}
            title={tx('团队文件', 'Team Files')}
            description={selectedTeam ? `${selectedTeam.name} / ${selectedTeam.slug}` : undefined}
            loadNode={teamTreeLoader}
            fileRoute={teamFileRoute}
            onPathChange={setTreePath}
          />
        </div>
      ))}
      </div>

      {teamDialog && (
        <div
          className="materials-modal-backdrop"
          onClick={closeTeamDialog}
        >
          <section
            className="materials-modal app-modal team-action-modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="team-action-title"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="app-modal-head">
              <div>
                <div className="app-modal-kicker">Team</div>
                <h3 id="team-action-title">
                  {teamDialog === 'create-team'
                    ? tx('创建团队', 'Create team')
                    : teamDialog === 'add-member'
                      ? tx('添加成员', 'Add member')
                      : teamDialog === 'create-agent'
                        ? tx('新 Agent 配方', 'New agent recipe')
                        : teamDialog === 'create-mcp'
                          ? tx('新建共享 MCP', 'New shared MCP')
                          : tx('发布信息', 'Publication details')}
                </h3>
                <p>
                  {teamDialog === 'create-team'
                    ? tx('团队会有独立资料空间，当前账号会成为 Owner。', 'The team gets its own material space. The current account becomes owner.')
                    : teamDialog === 'add-member'
                      ? tx('用对方的用户 slug 添加，成员仍保留自己的个人空间。', 'Add a teammate by user slug. Their personal space stays separate.')
                      : teamDialog === 'create-agent'
                        ? tx('保存给 Codex、Claude、Cursor 或 Gemini CLI 复用的 Agent 说明。', 'Save reusable agent instructions for Codex, Claude, Cursor, or Gemini CLI.')
                        : teamDialog === 'create-mcp'
                          ? tx('把数据库、服务 API 或本地命令登记为团队 MCP 配置。', 'Register a database, service API, or local command as a team MCP config.')
                          : tx('维护版本、发布说明和内部备注。', 'Maintain version, release note, and internal note.')}
                </p>
              </div>
              <button
                className="modal-icon-btn"
                type="button"
                aria-label={tx('关闭', 'Close')}
                onClick={closeTeamDialog}
              >
                ×
              </button>
            </div>

            {teamDialog === 'create-team' ? (
              <div className="modal-form-grid">
                <label className="profile-edit-field">
                  <span>{tx('团队名称', 'Team name')}</span>
                  <input className="input" value={newTeamName} onChange={(event) => {
                    setNewTeamName(event.target.value)
                    if (!newTeamSlug) setNewTeamSlug(normalizeSlugInput(event.target.value))
                  }} />
                </label>
                <label className="profile-edit-field">
                  <span>slug</span>
                  <input className="input" value={newTeamSlug} onChange={(event) => setNewTeamSlug(normalizeSlugInput(event.target.value))} />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('描述', 'Description')}</span>
                  <input className="input" value={newTeamDescription} onChange={(event) => setNewTeamDescription(event.target.value)} />
                </label>
                <div className="modal-actions">
                  <button className="btn" type="button" onClick={closeTeamDialog}>{tx('取消', 'Cancel')}</button>
                  <button className="btn btn-primary" type="button" disabled={working} onClick={() => { void handleCreateTeam() }}>
                    {working ? tx('创建中...', 'Creating...') : tx('创建团队', 'Create team')}
                  </button>
                </div>
              </div>
            ) : null}

            {teamDialog === 'add-member' ? (
              <div className="modal-form-grid">
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('用户 slug', 'User slug')}</span>
                  <input className="input" value={memberSlug} onChange={(event) => setMemberSlug(event.target.value)} />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('角色', 'Role')}</span>
                  <CustomSelect
                    value={memberRole}
                    onChange={(val) => setMemberRole(val as TeamRole)}
                    options={[
                      { value: 'member', label: roleLabel('member', locale) },
                      { value: 'admin', label: roleLabel('admin', locale) },
                      { value: 'viewer', label: roleLabel('viewer', locale) },
                    ]}
                    ariaLabel={tx('选择角色', 'Select role')}
                  />
                </label>
                <div className="modal-actions">
                  <button className="btn" type="button" onClick={closeTeamDialog}>{tx('取消', 'Cancel')}</button>
                  <button className="btn btn-primary" type="button" disabled={working || !memberSlug.trim()} onClick={() => { void handleAddMember() }}>
                    {working ? tx('添加中...', 'Adding...') : tx('添加成员', 'Add member')}
                  </button>
                </div>
              </div>
            ) : null}

            {teamDialog === 'create-agent' ? (
              <div className="modal-form-grid">
                <label className="profile-edit-field">
                  <span>{tx('Agent 名称', 'Agent name')}</span>
                  <input className="input" value={newAgentName} onChange={(event) => {
                    setNewAgentName(event.target.value)
                    if (!newAgentSlug) setNewAgentSlug(normalizeSlugInput(event.target.value))
                  }} />
                </label>
                <label className="profile-edit-field">
                  <span>slug</span>
                  <input className="input" value={newAgentSlug} onChange={(event) => setNewAgentSlug(normalizeSlugInput(event.target.value))} />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('用途说明', 'Description')}</span>
                  <input className="input" value={newAgentDescription} onChange={(event) => setNewAgentDescription(event.target.value)} />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('Agent 指令', 'Agent instructions')}</span>
                  <textarea className="input" rows={5} value={newAgentInstructions} onChange={(event) => setNewAgentInstructions(event.target.value)} />
                </label>
                <div className="modal-actions">
                  <button className="btn" type="button" onClick={closeTeamDialog}>{tx('取消', 'Cancel')}</button>
                  <button className="btn btn-primary" type="button" disabled={agentWorking || !newAgentName.trim()} onClick={() => { void handleCreateAgent() }}>
                    {agentWorking ? tx('保存中...', 'Saving...') : tx('保存配方', 'Save recipe')}
                  </button>
                </div>
              </div>
            ) : null}

            {teamDialog === 'create-mcp' ? (
              <div className="modal-form-grid">
                <label className="profile-edit-field">
                  <span>{tx('MCP 名称', 'MCP name')}</span>
                  <input className="input" value={newMcpName} onChange={(event) => {
                    setNewMcpName(event.target.value)
                    if (!newMcpSlug) setNewMcpSlug(normalizeSlugInput(event.target.value))
                  }} />
                </label>
                <label className="profile-edit-field">
                  <span>slug</span>
                  <input className="input" value={newMcpSlug} onChange={(event) => setNewMcpSlug(normalizeSlugInput(event.target.value))} />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('用途说明', 'Description')}</span>
                  <input className="input" value={newMcpDescription} onChange={(event) => setNewMcpDescription(event.target.value)} />
                </label>
                <label className="profile-edit-field">
                  <span>{tx('标签', 'Tags')}</span>
                  <input className="input" value={newMcpTags} onChange={(event) => setNewMcpTags(event.target.value)} />
                </label>
                <label className="profile-edit-field">
                  <span>{tx('连接方式', 'Transport')}</span>
                  <select className="input" value={newMcpTransport} onChange={(event) => setNewMcpTransport(event.target.value as 'stdio' | 'http')}>
                    <option value="stdio">stdio</option>
                    <option value="http">http</option>
                  </select>
                </label>
                {newMcpTransport === 'stdio' ? (
                  <>
                    <label className="profile-edit-field">
                      <span>Command</span>
                      <input className="input" value={newMcpCommand} onChange={(event) => setNewMcpCommand(event.target.value)} />
                    </label>
                    <label className="profile-edit-field">
                      <span>Args</span>
                      <input className="input" value={newMcpArgs} onChange={(event) => setNewMcpArgs(event.target.value)} />
                    </label>
                    <label className="profile-edit-field profile-long-field">
                      <span>Env</span>
                      <textarea className="input" rows={3} value={newMcpEnv} onChange={(event) => setNewMcpEnv(event.target.value)} />
                    </label>
                  </>
                ) : (
                  <>
                    <label className="profile-edit-field profile-long-field">
                      <span>URL</span>
                      <input className="input" value={newMcpURL} onChange={(event) => setNewMcpURL(event.target.value)} />
                    </label>
                    <label className="profile-edit-field profile-long-field">
                      <span>Headers</span>
                      <textarea className="input" rows={3} value={newMcpHeaders} onChange={(event) => setNewMcpHeaders(event.target.value)} />
                    </label>
                  </>
                )}
                <div className="modal-actions">
                  <button className="btn" type="button" onClick={closeTeamDialog}>{tx('取消', 'Cancel')}</button>
                  <button className="btn btn-primary" type="button" disabled={mcpWorking} onClick={() => { void handleCreateMcp() }}>
                    {mcpWorking ? tx('保存中...', 'Saving...') : tx('创建并保存', 'Create & Save')}
                  </button>
                </div>
              </div>
            ) : null}

            {teamDialog === 'skill-publication' && editingPublication ? (
              <div className="modal-form-grid">
                <div className="team-publication-dialog-path profile-long-field">
                  <span>{tx('Skill 路径', 'Skill path')}</span>
                  <code>{editingPublication.skill_path}</code>
                </div>
                <label className="profile-edit-field">
                  <span>{tx('版本', 'Version')}</span>
                  <input
                    className="input"
                    value={skillPublicationDrafts[editingPublication.skill_path]?.version || ''}
                    onChange={(event) => updateSkillPublicationDraft(editingPublication.skill_path, 'version', event.target.value)}
                  />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('发布说明', 'Release note')}</span>
                  <input
                    className="input"
                    value={skillPublicationDrafts[editingPublication.skill_path]?.release_note || ''}
                    onChange={(event) => updateSkillPublicationDraft(editingPublication.skill_path, 'release_note', event.target.value)}
                  />
                </label>
                <label className="profile-edit-field profile-long-field">
                  <span>{tx('内部备注', 'Internal note')}</span>
                  <textarea
                    className="input"
                    rows={4}
                    value={skillPublicationDrafts[editingPublication.skill_path]?.note || ''}
                    onChange={(event) => updateSkillPublicationDraft(editingPublication.skill_path, 'note', event.target.value)}
                  />
                </label>
                <div className="modal-actions">
                  <button
                    className="btn"
                    type="button"
                    onClick={closeTeamDialog}
                  >
                    {tx('取消', 'Cancel')}
                  </button>
                  <button className="btn btn-primary" type="button" disabled={sharingWorking} onClick={() => {
                    void handleSaveSkillPublicationMeta(editingPublication).then((saved) => {
                      if (saved) closeTeamDialog()
                    })
                  }}>
                    {sharingWorking ? tx('保存中...', 'Saving...') : tx('保存发布信息', 'Save details')}
                  </button>
                </div>
              </div>
            ) : null}
          </section>
        </div>
      )}
    </div>
  )
}
