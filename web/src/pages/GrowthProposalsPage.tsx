import { useCallback, useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api, type GrowthProposal, type Team } from '../api'
import { useI18n } from '../i18n'
import { dataFileEditorRoute, formatDateTime } from './data/DataShared'
import CustomSelect from '../components/CustomSelect'

const TEAM_SELECTION_KEY = 'vola:selected-team-id'

function teamMatches(team: Team, value: string) {
  return team.id === value || team.slug === value
}

function statusLabel(status: string, tx: (zh: string, en: string) => string) {
  if (status === 'pending_review') return tx('待审', 'Pending')
  if (status === 'accepted') return tx('已接受', 'Accepted')
  if (status === 'dismissed') return tx('已忽略', 'Dismissed')
  if (status === 'applied') return tx('已应用', 'Applied')
  return status
}

function changeLabel(kind: string, tx: (zh: string, en: string) => string) {
  if (kind === 'append_section') return tx('追加章节', 'Append section')
  if (kind === 'update_frontmatter_field') return tx('更新 frontmatter', 'Update frontmatter')
  if (kind === 'create_candidate_skill') return tx('创建候选 Skill', 'Create candidate skill')
  if (kind === 'add_verification_note') return tx('增加验证提示', 'Add verification note')
  return kind
}

export default function GrowthProposalsPage() {
  const { locale, tx } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const query = useMemo(() => new URLSearchParams(location.search), [location.search])
  const [teams, setTeams] = useState<Team[]>([])
  const [selectedTeamID, setSelectedTeamID] = useState(() => {
    if (typeof window === 'undefined') return ''
    return query.get('team') || window.localStorage.getItem(TEAM_SELECTION_KEY) || ''
  })
  const [statusFilter, setStatusFilter] = useState(query.get('status') || 'pending_review')
  const [proposals, setProposals] = useState<GrowthProposal[]>([])
  const [loading, setLoading] = useState(true)
  const [busyID, setBusyID] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const selectedTeam = useMemo(
    () => teams.find((team) => teamMatches(team, selectedTeamID)) || null,
    [selectedTeamID, teams],
  )
  const activeTeamID = selectedTeam?.id || ''

  const teamOptions = useMemo(() => {
    const list = [{ value: '', label: tx('个人空间', 'Personal') }]
    teams.forEach((t) => {
      list.push({ value: t.id, label: `${t.name} / ${t.slug}` })
    })
    return list
  }, [teams, tx])

  const statusOptions = useMemo(() => [
    { value: 'pending_review', label: tx('待审', 'Pending') },
    { value: 'accepted', label: tx('已接受', 'Accepted') },
    { value: 'applied', label: tx('已应用', 'Applied') },
    { value: 'dismissed', label: tx('已忽略', 'Dismissed') },
    { value: '', label: tx('全部', 'All') }
  ], [tx])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const teamList = await api.getTeams().catch(() => [] as Team[])
      const queryTeamID = query.get('team') || ''
      const savedTeamID = typeof window !== 'undefined' ? window.localStorage.getItem(TEAM_SELECTION_KEY) || '' : ''
      const nextTeam = teamList.find((team) => teamMatches(team, queryTeamID || selectedTeamID || savedTeamID)) || null
      const teamID = queryTeamID && nextTeam ? nextTeam.id : ''
      setTeams(teamList)
      setSelectedTeamID(teamID)
      const result = await api.getGrowthProposals(teamID || undefined, statusFilter || undefined)
      setProposals(result.proposals || [])
    } catch (err: any) {
      setError(err.message || tx('加载成长提案失败', 'Failed to load growth proposals'))
    } finally {
      setLoading(false)
    }
  }, [query, selectedTeamID, statusFilter, tx])

  useEffect(() => {
    void load()
  }, [load])

  const changeScope = (teamID: string) => {
    const params = new URLSearchParams(location.search)
    if (teamID) {
      params.set('team', teamID)
      if (typeof window !== 'undefined') window.localStorage.setItem(TEAM_SELECTION_KEY, teamID)
    } else {
      params.delete('team')
    }
    navigate(`/growth-proposals${params.toString() ? `?${params.toString()}` : ''}`, { replace: true })
    setSelectedTeamID(teamID)
  }

  const changeStatus = (status: string) => {
    const params = new URLSearchParams(location.search)
    if (status) params.set('status', status)
    else params.delete('status')
    navigate(`/growth-proposals${params.toString() ? `?${params.toString()}` : ''}`, { replace: true })
    setStatusFilter(status)
  }

  const runAction = async (proposal: GrowthProposal, action: 'accept' | 'dismiss' | 'apply') => {
    if (busyID) return
    setBusyID(`${proposal.id}:${action}`)
    setMessage('')
    setError('')
    try {
      const response = action === 'accept'
        ? await api.acceptGrowthProposal(proposal.id, activeTeamID || undefined)
        : action === 'dismiss'
          ? await api.dismissGrowthProposal(proposal.id, activeTeamID || undefined)
          : await api.applyGrowthProposal(proposal.id, activeTeamID || undefined)
      setMessage(action === 'apply' ? tx('提案已应用。', 'Proposal applied.') : tx('提案状态已更新。', 'Proposal status updated.'))
      setProposals((items) => items.map((item) => item.id === proposal.id ? response.proposal : item))
    } catch (err: any) {
      setError(err.message || tx('操作失败', 'Action failed'))
    } finally {
      setBusyID('')
    }
  }

  if (loading) {
    return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>
  }

  return (
    <div className="page materials-page">
      <section className="materials-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">Learning Engine</div>
          <h2 className="materials-title">{tx('成长提案', 'Growth proposals')}</h2>
          <p className="materials-subtitle">{tx('检查学习引擎建议的 Skill 改动，确认后再应用到文件。', 'Review suggested Skill changes before applying them to files.')}</p>
        </div>
        <div className="materials-actions" style={{ gap: '12px' }}>
          <CustomSelect
            value={activeTeamID}
            onChange={(val) => changeScope(val)}
            options={teamOptions}
            ariaLabel={tx('选择范围', 'Select scope')}
          />
          <CustomSelect
            value={statusFilter}
            onChange={(val) => changeStatus(val)}
            options={statusOptions}
            ariaLabel={tx('选择状态', 'Select status')}
          />
        </div>
      </section>

      {error && <div className="alert alert-warn">{error}</div>}
      {message && <div className="alert alert-success">{message}</div>}

      <section className="materials-section growth-proposals-section">
        <div className="materials-section-head">
          <div>
            <h3 className="materials-section-title">{tx('提案列表', 'Proposal list')}</h3>
            <p className="materials-section-copy">{tx('只支持低风险变更类型：追加章节、更新 frontmatter、创建候选 Skill、增加验证提示。', 'Supported low-risk changes: append section, update frontmatter, create candidate Skill, add verification note.')}</p>
          </div>
          <span className="materials-tile-pill">{proposals.length}</span>
        </div>

        {proposals.length === 0 ? (
          <div className="empty-action-state">
            <p>{tx('当前没有这个状态的提案。', 'No proposals in this status.')}</p>
          </div>
        ) : (
          <div className="growth-proposal-list">
            {proposals.map((proposal) => (
              <article key={proposal.id} className={`growth-proposal-card is-${proposal.status}`}>
                <div className="growth-proposal-card-head">
                  <div>
                    <strong>{proposal.type}</strong>
                    <small>{proposal.id}</small>
                  </div>
                  <span>{statusLabel(proposal.status, tx)} / {proposal.risk}</span>
                </div>
                <p>{proposal.reason}</p>
                <div className="growth-proposal-meta">
                  <button type="button" className="btn-text" onClick={() => navigate(dataFileEditorRoute(proposal.target_path, activeTeamID))}>{proposal.target_path}</button>
                  <span>{formatDateTime(proposal.created_at, locale)}</span>
                  {proposal.source_run_id ? <span>{proposal.source_run_id}</span> : null}
                </div>
                <div className="growth-proposal-changes">
                  {proposal.suggested_changes.map((change, index) => (
                    <div key={`${proposal.id}-${index}`}>
                      <strong>{changeLabel(change.kind, tx)}</strong>
                      <span>{change.field || change.heading || change.path || proposal.target_path}</span>
                      {change.value || change.content ? <p>{change.value || change.content}</p> : null}
                    </div>
                  ))}
                </div>
                <div className="materials-actions growth-proposal-actions">
                  {proposal.status === 'pending_review' ? (
                    <>
                      <button className="btn btn-sm" disabled={!!busyID} onClick={() => { void runAction(proposal, 'dismiss') }}>{busyID === `${proposal.id}:dismiss` ? tx('处理中...', 'Working...') : tx('忽略', 'Dismiss')}</button>
                      <button className="btn btn-sm" disabled={!!busyID} onClick={() => { void runAction(proposal, 'accept') }}>{busyID === `${proposal.id}:accept` ? tx('处理中...', 'Working...') : tx('接受', 'Accept')}</button>
                    </>
                  ) : null}
                  {(proposal.status === 'pending_review' || proposal.status === 'accepted') ? (
                    <button className="btn btn-sm btn-primary" disabled={!!busyID} onClick={() => { void runAction(proposal, 'apply') }}>{busyID === `${proposal.id}:apply` ? tx('应用中...', 'Applying...') : tx('应用', 'Apply')}</button>
                  ) : null}
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
