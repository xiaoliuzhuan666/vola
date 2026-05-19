import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode, type Team, type TeamMember, type TeamRole } from '../api'
import GitHubTreeList from '../components/GitHubTreeList'
import MaterialsTile from '../components/MaterialsTile'
import { useI18n } from '../i18n'
import { dataFileEditorRoute } from './data/DataShared'

const TEAM_ROOT = '/team'
const TEAM_SELECTION_KEY = 'neudrive:selected-team-id'

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
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [treePath, setTreePath] = useState(TEAM_ROOT)
  const [newTeamName, setNewTeamName] = useState('')
  const [newTeamSlug, setNewTeamSlug] = useState('')
  const [newTeamDescription, setNewTeamDescription] = useState('')
  const [memberSlug, setMemberSlug] = useState('')
  const [memberRole, setMemberRole] = useState<TeamRole>('member')

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
      const selected = await loadTeams()
      await loadTeamData(selected)
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

  return (
    <div className="materials-page">
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
            <select
              className="input"
              value={selectedTeam?.id || ''}
              onChange={(event) => setSelectedTeamID(event.target.value)}
              aria-label={tx('选择团队', 'Select team')}
            >
              {teams.map((team) => (
                <option key={team.id} value={team.id}>{team.name} / {team.slug}</option>
              ))}
            </select>
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
                  <select className="input" value={memberRole} onChange={(event) => setMemberRole(event.target.value as TeamRole)}>
                    <option value="member">{roleLabel('member', locale)}</option>
                    <option value="admin">{roleLabel('admin', locale)}</option>
                    <option value="viewer">{roleLabel('viewer', locale)}</option>
                  </select>
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
