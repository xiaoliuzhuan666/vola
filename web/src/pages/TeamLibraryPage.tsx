import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode, type Team, type TeamAgentAsset, type TeamMember, type TeamRole, type TeamSkillReviewEvent, type TeamSkillSubscriptionReportResponse, type TeamSkillUpdateNotification, type TeamSkillUpdateNotificationsResponse } from '../api'
import CustomSelect from '../components/CustomSelect'
import GitHubTreeList from '../components/GitHubTreeList'
import MaterialsTile from '../components/MaterialsTile'
import { useI18n } from '../i18n'
import { dataFileEditorRoute } from './data/DataShared'

const TEAM_ROOT = '/team'
const TEAM_SELECTION_KEY = 'vola:selected-team-id'

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
      return
    }
    const [teamResult, skillsResult, snapshotResult, membersResult, agentsResult, reviewHistoryResult, subscriptionReportResult, notificationsResult] = await Promise.allSettled([
      api.getTeamTree(team.id, TEAM_ROOT),
      api.getTeamTree(team.id, '/skills'),
      api.getTeamTreeSnapshot(team.id, '/'),
      api.getTeamMembers(team.id),
      api.getTeamAgents(team.id),
      api.getTeamSkillReviewHistory(team.id),
      team.can_manage_members ? api.getTeamSkillSubscriptionReport(team.id) : Promise.resolve(null),
      team.can_manage_members ? api.getTeamSkillUpdateNotifications(team.id) : Promise.resolve(null),
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
    setSkillReviewHistory(reviewHistoryResult.status === 'fulfilled' ? reviewHistoryResult.value.events || [] : [])
    setSkillSubscriptionReport(subscriptionReportResult.status === 'fulfilled' ? subscriptionReportResult.value : null)
    const notificationsResponse = notificationsResult.status === 'fulfilled' ? notificationsResult.value : null
    setSkillUpdateNotifications(notificationsResponse?.notifications || [])
    setSkillUpdateNotificationInfo(notificationsResponse ? {
      storage_path: notificationsResponse.storage_path,
      updated_at: notificationsResponse.updated_at,
      last_checked_at: notificationsResponse.last_checked_at,
    } : null)
  }, [])

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

  const categories = useMemo(() => {
    return [
      {
        title: tx('团队 Skill', 'Team Skills'),
        subtitle: '/skills',
        desc: tx('归属当前团队的 Skill，不会和成员个人 Skill 混在一起。', 'Skills owned by this team, separate from personal skills.'),
        count: childCount(skillsRoot),
        icon: 'icon-stack',
        action: <button className="btn btn-sm" type="button" onClick={() => setTreePath('/skills')}>{tx('打开', 'Open')}</button>,
      },
      {
        title: 'MCP',
        subtitle: '/team/mcp',
        desc: tx('内部 MCP server、配置样例、负责人和安全说明。', 'Internal MCP servers, config examples, owners, and security notes.'),
        count: teamCounts.mcp || 0,
        icon: 'icon-device',
        action: selectedTeam ? <Link className="btn btn-sm" to={dataFileEditorRoute('/team/mcp/README.md', selectedTeam.id)}>{tx('打开', 'Open')}</Link> : null,
      },
      {
        title: tx('Agent 配方', 'Agent Recipes'),
        subtitle: '/team/agents',
        desc: tx('团队共用的 Agent 说明、默认 Skill、模型和权限建议。', 'Shared agent instructions, default skills, models, and permission notes.'),
        count: teamCounts.agents || 0,
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
  }, [locale, selectedTeam, skillsRoot, teamCounts, tx])

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
      setMessage(tx('成员已加入团队。', 'Member added to the team.'))
    } catch (err: any) {
      setError(err?.message || tx('添加成员失败', 'Failed to add member'))
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

  const isLocalTeamSimulation = !currentUser || !currentUser.slug || currentUser.slug === 'local'

  return (
    <div className="materials-page" style={{ paddingLeft: '24px', paddingRight: '24px', margin: '0 auto', maxWidth: '1240px' }}>
      <section className="materials-hero">
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

      <section className="materials-section">
        <div className="materials-section-head">
          <div>
            <h3 className="materials-section-title">{tx('团队', 'Teams')}</h3>
            <p className="materials-section-copy">
              {tx('创建团队后，把同事加入对应资料库。当前阶段用于共享资料，不作为企业级组织管理后台。', 'Create a team, then add teammates to the shared library. This phase is for shared material, not enterprise org administration.')}
            </p>
          </div>
        </div>

        <div className="materials-grid materials-grid-wide">
          {teams.map((team) => (
            <MaterialsTile
              key={team.id}
              iconClassName="icon-stack"
              title={team.name}
              subtitle={`/${team.slug}`}
              description={team.description || tx('团队空间', 'Team space')}
              selected={selectedTeam?.id === team.id}
              footerStart={roleLabel(team.role, locale)}
              footerEnd={team.can_write ? tx('可写入', 'Can write') : tx('只读', 'Read only')}
              onOpen={() => setSelectedTeamID(team.id)}
            />
          ))}
          <MaterialsTile
            iconClassName="icon-file"
            title={tx('新团队', 'New team')}
            subtitle={tx('独立资料空间', 'Separate space')}
            description={tx('适合部门、项目组或客户项目。', 'For departments, project groups, or client work.')}
            footerStart={tx('Owner 自动设为当前账号', 'Current account becomes owner')}
          >
            <div className="form-grid">
              <input className="input" placeholder={tx('团队名称', 'Team name')} value={newTeamName} onChange={(event) => {
                setNewTeamName(event.target.value)
                if (!newTeamSlug) setNewTeamSlug(normalizeSlugInput(event.target.value))
              }} />
              <input className="input" placeholder="slug" value={newTeamSlug} onChange={(event) => setNewTeamSlug(normalizeSlugInput(event.target.value))} />
              <input className="input" placeholder={tx('描述', 'Description')} value={newTeamDescription} onChange={(event) => setNewTeamDescription(event.target.value)} />
              <button className="btn btn-sm" type="button" disabled={working} onClick={handleCreateTeam}>{tx('创建', 'Create')}</button>
            </div>
          </MaterialsTile>
        </div>
      </section>

      {selectedTeam && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
            <h3 className="materials-section-title">{tx('成员', 'Members')}</h3>
              <p className="materials-section-copy">
                {tx('Owner/Admin 管理成员并处理审查；Member 可写团队资料并提交审查；Viewer 只读。', 'Owners/Admins manage members and reviews; Members can write team files and request reviews; Viewers are read-only.')}
              </p>
            </div>
          </div>

          <div className="materials-grid materials-grid-wide">
            {members.map((member) => (
              <MaterialsTile
                key={member.user_id}
                iconClassName="icon-file"
                title={member.display_name || member.user_slug}
                subtitle={`@${member.user_slug}`}
                description={member.email}
                footerStart={roleLabel(member.role, locale)}
              />
            ))}
            {selectedTeam.can_manage_members && (
              <MaterialsTile
                iconClassName="icon-file"
                title={tx('添加成员', 'Add member')}
                subtitle={tx('通过用户 slug 添加', 'Add by user slug')}
                description={tx('被添加的同事仍保留自己的个人空间。', 'The teammate keeps their personal space.')}
              >
                <div className="form-grid">
                  <input className="input" placeholder={tx('用户 slug', 'User slug')} value={memberSlug} onChange={(event) => setMemberSlug(event.target.value)} />
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
                  <button className="btn btn-sm" type="button" disabled={working || !memberSlug.trim()} onClick={handleAddMember}>{tx('添加', 'Add')}</button>
                </div>
              </MaterialsTile>
            )}
          </div>
        </section>
      )}

      {selectedTeam && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('资料类型', 'Library Types')}</h3>
              <p className="materials-section-copy">
                {tx('当前查看的是所选团队的独立 Hub 空间。', 'You are viewing the selected team-owned Hub space.')}
              </p>
            </div>
          </div>

          <div className="materials-grid materials-grid-wide">
            {categories.map((category) => (
              <MaterialsTile
                key={category.subtitle}
                iconClassName={category.icon}
                title={category.title}
                subtitle={category.subtitle}
                description={category.desc}
                footerStart={loading ? tx('读取中', 'Loading') : formatCategoryCount(category.count, locale)}
                actions={category.action}
              />
            ))}
          </div>
        </section>
      )}

      {selectedTeam && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('团队共享管理', 'Team Sharing Management')}</h3>
              <p className="materials-section-copy">
                {tx('查看团队 Skill 审查记录、成员订阅状态和更新提醒。', 'Review team skill history, member subscription status, and update notices.')}
              </p>
            </div>
          </div>

          <div className="materials-grid materials-grid-wide">
            {selectedTeam.can_manage_members ? (
              <MaterialsTile
                iconClassName="icon-stack"
                title={tx('订阅报表', 'Subscription Report')}
                subtitle={tx(`${subscriptionSummary.skills} 个团队 Skill`, `${subscriptionSummary.skills} team skills`)}
                description={tx('按成员查看哪些团队 Skill 有个人副本、未安装或需要更新。', 'See which team skills have personal copies, are missing, or need updates by member.')}
                footerStart={tx(`个人副本 ${subscriptionSummary.personalCopies}`, `${subscriptionSummary.personalCopies} personal copies`)}
                footerEnd={tx(`待更新 ${subscriptionSummary.updates}`, `${subscriptionSummary.updates} updates`)}
                actions={(
                  <button className="btn btn-sm btn-primary" type="button" disabled={sharingWorking} onClick={() => { void handleCheckTeamSkillSubscriptions() }}>
                    {sharingWorking ? tx('检查中...', 'Checking...') : tx('检查更新', 'Check updates')}
                  </button>
                )}
              >
                <div className="team-admin-list">
                  {(skillSubscriptionReport?.skills || []).filter((skill) => skill.update_available_count > 0 || skill.source_missing_count > 0 || skill.not_installed_count > 0).slice(0, 4).map((skill) => (
                    <div key={skill.skill_path} className="team-admin-row">
                      <span>{skill.skill_path}</span>
                      <small>{tx(`未安装 ${skill.not_installed_count} / 更新 ${skill.update_available_count}`, `${skill.not_installed_count} missing / ${skill.update_available_count} updates`)}</small>
                    </div>
                  ))}
                  {!skillSubscriptionReport?.skills?.length ? (
                    <div className="team-admin-empty">{tx('暂无订阅数据。', 'No subscription data yet.')}</div>
                  ) : null}
                </div>
              </MaterialsTile>
            ) : null}

            {selectedTeam.can_manage_members ? (
              <MaterialsTile
                iconClassName="icon-file"
                title={tx('更新通知', 'Update Notices')}
                subtitle={tx(`最近检查：${formatDateTime(skillUpdateNotificationInfo?.last_checked_at || skillUpdateNotifications[0]?.created_at, locale)}`, `Last checked: ${formatDateTime(skillUpdateNotificationInfo?.last_checked_at || skillUpdateNotifications[0]?.created_at, locale)}`)}
                description={tx('检查结果会保存到团队 Hub，供管理员后续处理。', 'Check results are saved in the team Hub for admins to review.')}
                footerStart={tx(`${skillUpdateNotifications.length} 条提醒`, `${skillUpdateNotifications.length} notices`)}
              >
                <div className="team-admin-list">
                  {skillUpdateNotifications.slice(0, 4).map((item) => (
                    <div key={item.id} className="team-admin-row">
                      <span>{item.skill_path}</span>
                      <small>{item.display_name || item.user_slug} · {item.status}</small>
                    </div>
                  ))}
                  {skillUpdateNotifications.length === 0 ? (
                    <div className="team-admin-empty">{tx('暂无更新提醒。', 'No update notices yet.')}</div>
                  ) : null}
                </div>
              </MaterialsTile>
            ) : null}

            <MaterialsTile
              iconClassName="icon-file"
              title={tx('审查历史', 'Review History')}
              subtitle={tx(`${skillReviewHistory.length} 条记录`, `${skillReviewHistory.length} records`)}
              description={tx('记录 Skill 和 Agent 配方的提交、通过、要求修改、发布和归档。', 'Records review requests, approvals, requested changes, publishing, and archiving for skills and agent recipes.')}
              footerStart={tx(`提交审查 ${reviewEventCount}`, `${reviewEventCount} review requests`)}
            >
              <div className="team-admin-list">
                {skillReviewHistory.slice(0, 5).map((event) => (
                  <div key={event.id} className="team-admin-row">
                    <span>{event.skill_path || event.agent_slug || event.asset_type}</span>
                    <small>{reviewActionLabel(event.action, locale)} · {roleLabel(event.actor_role as TeamRole, locale)} · {formatDateTime(event.created_at, locale)}</small>
                  </div>
                ))}
                {skillReviewHistory.length === 0 ? (
                  <div className="team-admin-empty">{tx('暂无审查记录。', 'No review history yet.')}</div>
                ) : null}
              </div>
            </MaterialsTile>
          </div>
        </section>
      )}

      {selectedTeam && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('团队 Agent 配方', 'Team Agent Recipes')}</h3>
              <p className="materials-section-copy">
                {tx('这里保存可共享的 Agent 配置：默认 Skill、模型、权限和审批动作。Vola 保存配方，不直接运行 Agent。', 'Shared agent recipes store default skills, models, permissions, and approval notes. Vola stores the recipe and does not run the agent.')}
              </p>
            </div>
          </div>

          <div className="materials-grid materials-grid-wide">
            {teamAgents.map((agent) => (
              <MaterialsTile
                key={agent.slug}
                iconClassName="icon-device"
                title={agent.name}
                subtitle={`/${agent.slug}`}
                description={agent.description || agent.instructions || tx('团队 Agent 配方', 'Team agent recipe')}
                footerStart={agentStatusLabel(agent.status, agent.visibility, locale)}
                footerEnd={agent.review_status === 'requested'
                  ? tx('待审查', 'Review requested')
                  : agent.review_status === 'changes_requested'
                    ? tx('需修改', 'Changes requested')
                    : (agent.default_skill_paths || []).length
                      ? tx(`${agent.default_skill_paths?.length || 0} 个默认 Skill`, `${agent.default_skill_paths?.length || 0} default skills`)
                      : tx('可安装到个人', 'Installable')}
                actions={(
                  <>
                    {agent.status === 'published' && agent.visibility === 'team' ? (
                      <button className="btn btn-sm" type="button" disabled={installingAgentSlug === agent.slug} onClick={() => { void handleInstallAgent(agent) }}>
                        {installingAgentSlug === agent.slug ? tx('安装中...', 'Installing...') : tx('安装到个人', 'Install')}
                      </button>
                    ) : null}
                    {selectedTeam.can_write && !selectedTeam.can_manage_members ? (
                      <button className="btn btn-sm" type="button" disabled={agentWorking || agent.review_status === 'requested'} onClick={() => { void handleRequestAgentReview(agent) }}>
                        {agent.review_status === 'requested' ? tx('待审查', 'Review requested') : tx('提交审查', 'Request review')}
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
                  </>
                )}
              />
            ))}
            {selectedTeam.can_write && (
              <MaterialsTile
                iconClassName="icon-file"
                title={tx('新 Agent 配方', 'New Agent Recipe')}
                subtitle={tx('默认保存为团队资产', 'Saved as a team asset')}
                description={tx('给 Codex、Claude、Cursor 或 Gemini CLI 复用的 Agent 说明。', 'Agent instructions reusable by Codex, Claude, Cursor, or Gemini CLI.')}
                footerStart={selectedTeam.can_manage_members ? tx('创建后发布', 'Publish on create') : tx('创建为草稿', 'Create as draft')}
              >
                <div className="form-grid">
                  <input className="input" placeholder={tx('Agent 名称', 'Agent name')} value={newAgentName} onChange={(event) => {
                    setNewAgentName(event.target.value)
                    if (!newAgentSlug) setNewAgentSlug(normalizeSlugInput(event.target.value))
                  }} />
                  <input className="input" placeholder="slug" value={newAgentSlug} onChange={(event) => setNewAgentSlug(normalizeSlugInput(event.target.value))} />
                  <input className="input" placeholder={tx('用途说明', 'Description')} value={newAgentDescription} onChange={(event) => setNewAgentDescription(event.target.value)} />
                  <textarea className="input" rows={4} placeholder={tx('Agent 指令、默认工作方式和审批边界', 'Agent instructions, default workflow, and approval boundaries')} value={newAgentInstructions} onChange={(event) => setNewAgentInstructions(event.target.value)} />
                  <button className="btn btn-sm" type="button" disabled={agentWorking || !newAgentName.trim()} onClick={handleCreateAgent}>
                    {agentWorking ? tx('保存中...', 'Saving...') : tx('保存配方', 'Save recipe')}
                  </button>
                </div>
              </MaterialsTile>
            )}
          </div>
        </section>
      )}

      {!selectedTeam ? (
        <section className="dashboard-file-panel github-tree-list">
          <div className="dashboard-section-head">
            <div>
              <h3>{tx('团队文件', 'Team Files')}</h3>
              <p>{tx('还没有团队。创建一个团队后再添加资料。', 'No team yet. Create a team to add shared material.')}</p>
            </div>
          </div>
          <div className="dashboard-file-empty">{tx('暂无团队文件。', 'No team files yet.')}</div>
        </section>
      ) : !loading && !teamRoot ? (
        <section className="dashboard-file-panel github-tree-list">
          <div className="dashboard-section-head">
            <div>
              <h3>{tx('团队文件', 'Team Files')}</h3>
              <p>{tx('团队资料目录尚未创建，点击上方按钮生成 /team/mcp、/team/prompts 和 /team/playbooks。', 'The team library folders have not been created yet. Use the button above to create /team/mcp, /team/prompts, and /team/playbooks.')}</p>
            </div>
          </div>
          <div className="dashboard-file-empty">{tx('暂无团队文件。', 'No team files yet.')}</div>
        </section>
      ) : (
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
      )}
    </div>
  )
}
