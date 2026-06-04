import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode, type Team, type TeamMember, type TeamRole } from '../api'
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
  const [loading, setLoading] = useState(true)
  const [working, setWorking] = useState(false)
  const [activeCardHover, setActiveCardHover] = useState(false)
  const [hoveredFeature, setHoveredFeature] = useState<number | null>(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [treePath, setTreePath] = useState(TEAM_ROOT)
  const [newTeamName, setNewTeamName] = useState('')
  const [newTeamSlug, setNewTeamSlug] = useState('')
  const [newTeamDescription, setNewTeamDescription] = useState('')
  const [memberSlug, setMemberSlug] = useState('')
  const [memberRole, setMemberRole] = useState<TeamRole>('member')
  const [currentUser, setCurrentUser] = useState<any>(null)

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
      return
    }
    const [teamResult, skillsResult, snapshotResult, membersResult] = await Promise.allSettled([
      api.getTeamTree(team.id, TEAM_ROOT),
      api.getTeamTree(team.id, '/skills'),
      api.getTeamTreeSnapshot(team.id, '/'),
      api.getTeamMembers(team.id),
    ])
    const entries = snapshotResult.status === 'fulfilled' ? snapshotResult.value.entries || [] : []
    const existingPaths = new Set(entries.map((entry) => normalizeTemplatePath(entry.path)))
    setTeamRoot(teamResult.status === 'fulfilled' && hasSnapshotPath(existingPaths, TEAM_ROOT) ? teamResult.value : null)
    setSkillsRoot(skillsResult.status === 'fulfilled' ? skillsResult.value : null)
    setTeamCounts({
      mcp: countDirectChildren(entries, '/team/mcp'),
      prompts: countDirectChildren(entries, '/team/prompts'),
      playbooks: countDirectChildren(entries, '/team/playbooks'),
    })
    setMembers(membersResult.status === 'fulfilled' ? membersResult.value : [])
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

  const isLocalOnly = !currentUser || !currentUser.slug || currentUser.slug === 'local'

  if (!loading && isLocalOnly) {
    return (
      <div className="materials-page" style={{ paddingLeft: '24px', paddingRight: '24px', margin: '0 auto', maxWidth: '1240px' }}>
        <section style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '30px',
          padding: '36px 40px',
          marginBottom: '24px',
          border: '1px solid rgba(65, 77, 136, 0.08)',
          borderRadius: '24px',
          background: 'linear-gradient(135deg, rgba(255, 255, 255, 0.95) 0%, rgba(248, 250, 252, 0.85) 100%)',
          boxShadow: '0 10px 30px rgba(65, 77, 136, 0.03), inset 0 1px 0 #ffffff',
          backdropFilter: 'blur(8px)',
        }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{
              fontSize: '11px',
              fontWeight: 800,
              letterSpacing: '0.15em',
              textTransform: 'uppercase',
              color: '#6366f1',
              marginBottom: '8px',
            }}>{tx('团队资料', 'Team Library')}</div>
            <h2 style={{
              fontSize: '28px',
              fontWeight: 800,
              color: '#12192d',
              margin: 0,
              letterSpacing: '-0.03em',
              lineHeight: '1.2',
            }}>{tx('团队 AI 资料库', 'Team AI Library')}</h2>
            <p style={{
              marginTop: '12px',
              fontSize: '14px',
              color: '#506074',
              lineHeight: '1.6',
              maxWidth: '640px',
              marginRight: 0,
              marginBottom: 0,
            }}>
              {tx(
                '小团队可共享专属 Skill、MCP 服务的配置样例与快捷提示词库，打破成员间的信息孤岛，共同沉淀团队高效的 AI 最佳实践。',
                'Small teams can share custom skills, MCP server guides, prompts, and playbooks in a private workspace to elevate collective AI workflow.'
              )}
            </p>
          </div>
          <svg viewBox="0 0 120 100" width="120" height="100" style={{ flexShrink: 0, display: 'block' }}>
            <defs>
              <linearGradient id="docGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stopColor="#818cf8" stopOpacity="0.85" />
                <stop offset="100%" stopColor="#c084fc" stopOpacity="0.85" />
              </linearGradient>
              <linearGradient id="docGrad2" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stopColor="#38bdf8" stopOpacity="0.75" />
                <stop offset="100%" stopColor="#818cf8" stopOpacity="0.75" />
              </linearGradient>
              <filter id="glow">
                <feGaussianBlur stdDeviation="2" result="blur" />
                <feComposite in="SourceGraphic" in2="blur" operator="over" />
              </filter>
            </defs>
            <circle cx="85" cy="50" r="30" stroke="rgba(99, 102, 241, 0.1)" strokeWidth="1" strokeDasharray="3 3" fill="none" />
            <path d="M45 70 Q 60 55 85 50" stroke="rgba(99, 102, 241, 0.15)" strokeWidth="1.5" strokeDasharray="3 3" fill="none" />
            <circle cx="85" cy="20" r="2.5" fill="#818cf8" opacity="0.6" filter="url(#glow)" />
            <circle cx="115" cy="50" r="2" fill="#38bdf8" opacity="0.6" />
            <rect x="32" y="18" width="40" height="52" rx="6" fill="url(#docGrad)" transform="rotate(-8 52 44)" style={{ filter: 'drop-shadow(0 8px 16px rgba(129, 140, 248, 0.12))' }} />
            <line x1="40" y1="31" x2="60" y2="28" stroke="#ffffff" strokeWidth="2" strokeLinecap="round" transform="rotate(-8 52 44)" />
            <line x1="40" y1="39" x2="52" y2="37" stroke="#ffffff" strokeWidth="2" strokeLinecap="round" transform="rotate(-8 52 44)" />
            <rect x="52" y="26" width="40" height="52" rx="6" fill="url(#docGrad2)" transform="rotate(6 72 52)" style={{ filter: 'drop-shadow(0 8px 16px rgba(56, 189, 248, 0.15))' }} />
            <line x1="60" y1="39" x2="80" y2="41" stroke="#ffffff" strokeWidth="2" strokeLinecap="round" transform="rotate(6 72 52)" />
            <line x1="60" y1="47" x2="75" y2="48" stroke="#ffffff" strokeWidth="2" strokeLinecap="round" transform="rotate(6 72 52)" />
            <line x1="60" y1="55" x2="68" y2="56" stroke="#ffffff" strokeWidth="2" strokeLinecap="round" transform="rotate(6 72 52)" />
            <circle cx="85" cy="50" r="4.5" fill="#6366f1" style={{ filter: 'drop-shadow(0 0 6px rgba(99, 102, 241, 0.6))' }} />
          </svg>
        </section>

        <section className="card" style={{
          marginTop: '24px',
          background: 'linear-gradient(135deg, rgba(99, 102, 241, 0.03) 0%, rgba(139, 92, 246, 0.03) 50%, rgba(6, 182, 212, 0.02) 100%)',
          border: '1px solid rgba(99, 102, 241, 0.12)',
          borderRadius: '24px',
          boxShadow: '0 20px 40px rgba(99, 102, 241, 0.04), inset 0 1px 0 rgba(255, 255, 255, 0.6)',
        }}>
          <div className="card-body" style={{ padding: '60px 40px', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <svg viewBox="0 0 64 64" width="80" height="80" style={{ marginBottom: '24px' }}>
              <defs>
                <linearGradient id="cloudGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#6366f1" />
                  <stop offset="100%" stopColor="#a855f7" />
                </linearGradient>
                <linearGradient id="nodeGrad" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#06b6d4" />
                  <stop offset="100%" stopColor="#6366f1" />
                </linearGradient>
              </defs>
              <path d="M16 32h32M32 16v32M20 20l24 24M44 20L20 44" stroke="rgba(99, 102, 241, 0.15)" strokeWidth="2" strokeDasharray="4 4" />
              <circle cx="16" cy="32" r="4" fill="url(#nodeGrad)" />
              <circle cx="48" cy="32" r="4" fill="url(#nodeGrad)" />
              <circle cx="32" cy="16" r="4" fill="url(#nodeGrad)" />
              <circle cx="32" cy="48" r="4" fill="url(#nodeGrad)" />
              <path d="M46 38a7 7 0 0 0-5.59-11.24 10 10 0 0 0-18.82-3.76A8 8 0 0 0 16 38a6 6 0 0 0 6 6h24a5 5 0 0 0 0-10z" fill="url(#cloudGrad)" />
              <path d="M26 38c0-1.5 2-2.5 4-2.5s4 1 4 2.5v1h-8zm4-3a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zm6 3c0-1.2 1.5-2 3-2s3 .8 3 2v1h-6zm3-3a1.2 1.2 0 1 0 0-2.4 1.2 1.2 0 0 0 0 2.4z" fill="#ffffff" />
            </svg>
            <h3 style={{ fontSize: '22px', fontWeight: 800, color: '#12192d', marginBottom: '10px', letterSpacing: '-0.02em' }}>
              {tx('激活团队协作空间', 'Activate Team Collaboration')}
            </h3>
            <p style={{ fontSize: '14.5px', color: '#506074', maxWidth: '560px', margin: '0 auto 28px auto', lineHeight: '1.7' }}>
              {tx(
                '您当前处于本地安全单机模式。在此状态下，您的所有文件和 AI 技能都安全地保存在本地设备中。为了与团队成员实时共享和同步 AI 技能、MCP 配置以及提示词库，请前往个人中心绑定云端账户。',
                'You are currently in safe Local-Only mode. All your files and AI skills stay securely on this device. To share and sync AI skills, MCP configurations, and prompt libraries with your team, please go to My Profile to bind a cloud account.'
              )}
            </p>
            <Link
              to="/settings/profile"
              className="btn btn-primary"
              onMouseEnter={() => setActiveCardHover(true)}
              onMouseLeave={() => setActiveCardHover(false)}
              style={{
                padding: '12px 28px',
                borderRadius: '12px',
                fontWeight: 600,
                background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%)',
                border: 'none',
                boxShadow: activeCardHover ? '0 10px 25px rgba(99, 102, 241, 0.35)' : '0 6px 18px rgba(99, 102, 241, 0.2)',
                transform: activeCardHover ? 'translateY(-1px) scale(1.02)' : 'none',
                transition: 'all 0.2s cubic-bezier(0.16, 1, 0.3, 1)',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '8px',
                color: '#ffffff',
                textDecoration: 'none',
              }}
            >
              {tx('立即前往绑定云账号', 'Go to Bind Cloud Account')}
              <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round" style={{ transition: 'transform 0.2s', transform: activeCardHover ? 'translateX(4px)' : 'none' }}>
                <line x1="5" y1="12" x2="19" y2="12" />
                <polyline points="12 5 19 12 12 19" />
              </svg>
            </Link>

            {/* 协作优势三列网格 */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
              gap: '20px',
              width: '100%',
              marginTop: '54px',
              textAlign: 'left'
            }}>
              {[
                {
                  title: tx('多端实时同步', 'Multi-device Realtime Sync'),
                  desc: tx('激活云服务后，您在多台设备上使用的 AI 技能、文件与配置均可实时无缝同步。', 'All your AI skills, files, and configurations seamlessly sync across devices in real time.'),
                  icon: (
                    <svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" style={{ color: '#6366f1' }}>
                      <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67" />
                    </svg>
                  ),
                },
                {
                  title: tx('团队协作共享', 'Teammate Collaborative Sharing'),
                  desc: tx('邀请团队成员加入，共同维护与共享团队 Skill、MCP 服务描述与快捷提示词库。', 'Invite colleagues to co-create and share custom skills, MCP connections, and prompt libraries.'),
                  icon: (
                    <svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" style={{ color: '#8b5cf6' }}>
                      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
                      <circle cx="9" cy="7" r="4" />
                      <path d="M23 21v-2a4 4 0 0 0-3-3.87" />
                      <path d="M16 3.13a4 4 0 0 1 0 7.75" />
                    </svg>
                  ),
                },
                {
                  title: tx('安全云端存储', 'Secure Cloud Storage'),
                  desc: tx('云端数据全程采用金融级高强度加密，无惧本地硬件损坏或误删风险。', 'End-to-end encryption secures your core data assets from hardware issues or accidental deletions.'),
                  icon: (
                    <svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" strokeLinejoin="round" style={{ color: '#06b6d4' }}>
                      <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                      <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                    </svg>
                  ),
                },
              ].map((feat, idx) => (
                <div
                  key={idx}
                  onMouseEnter={() => setHoveredFeature(idx)}
                  onMouseLeave={() => setHoveredFeature(null)}
                  style={{
                    padding: '24px',
                    background: '#ffffff',
                    border: '1px solid',
                    borderColor: hoveredFeature === idx ? 'rgba(99, 102, 241, 0.24)' : 'rgba(65, 77, 136, 0.08)',
                    borderRadius: '18px',
                    boxShadow: hoveredFeature === idx ? '0 16px 32px rgba(99, 102, 241, 0.06)' : '0 4px 12px rgba(65, 77, 136, 0.02)',
                    transform: hoveredFeature === idx ? 'translateY(-3px)' : 'none',
                    transition: 'all 0.25s cubic-bezier(0.16, 1, 0.3, 1)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '12px',
                  }}
                >
                  <div style={{
                    width: '40px',
                    height: '40px',
                    borderRadius: '10px',
                    background: 'rgba(99, 102, 241, 0.06)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}>
                    {feat.icon}
                  </div>
                  <h4 style={{ fontSize: '15px', fontWeight: 700, color: '#12192d', margin: 0 }}>
                    {feat.title}
                  </h4>
                  <p style={{ fontSize: '13px', color: '#506074', margin: 0, lineHeight: '1.6' }}>
                    {feat.desc}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>
    )
  }

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
                {tx('Owner/Admin 管理成员；Member 可写团队资料；Viewer 只读。审计、SSO 和审批流不在当前阶段。', 'Owners/Admins manage members; Members can write team files; Viewers are read-only. Audit, SSO, and approval workflows are outside this phase.')}
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
