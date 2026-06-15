import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { api, type FileNode, type LocalSkillSyncResponse, type SkillAgentAssignment, type SkillAssignmentsState, type SkillConversionRequest, type SkillConversionResponse, type SkillLearningNote, type SkillLearningSummary, type SkillSummary, type Team, type TeamSkillPublication, type TeamSkillSubscriptionStatus, type SkillDiffFileItem } from '../../api'
import GitHubTreeList from '../../components/GitHubTreeList'
import MaterialsSectionToolbar from '../../components/MaterialsSectionToolbar'
import FileMaterialsTile from '../../components/FileMaterialsTile'
import ResourceActionMenu from '../../components/ResourceActionMenu'
import ResourceConfirmDialog from '../../components/ResourceConfirmDialog'
import SourceFilterBar from '../../components/SourceFilterBar'
import useResourceCardMenu from '../../hooks/useResourceCardMenu'
import useTreeDeleteDialog from '../../hooks/useTreeDeleteDialog'
import { useI18n } from '../../i18n'
import CustomSelect from '../../components/CustomSelect'
import {
  getMaterialsSortOptions,
  buildFileTileModel,
  buildSourceFilterOptions,
  buildSkillBundleTileModel,
  bundleBrowsePath,
  bundleRelativeDirFromPath,
  dataFileEditorRoute,
  dataSkillBundleRoute,
  fileNodeSource,
  formatDateTime,
  isTextLikeFile,
  matchesSourceFilter,
  normalizeBundleRelativeDir,
  normalizeHubPath,
  sourceLabel,
  skillSource,
  sortMaterialsItems,
  skillBundlePathFromSkillPath,
  type MaterialsSortDir,
  type MaterialsSortKey,
} from './DataShared'

type SkillBundle = SkillSummary & {
  bundleId: string
  bundlePath: string
  created_at?: string
  updated_at?: string
}

type SkillScope = 'personal' | 'team'
type TeamSkillInstallStatus = 'not_installed' | 'installed' | 'update_available' | 'personal_newer'
type PersonalSkillCopyInfo = {
  bundlePath: string
  created_at?: string
  updated_at?: string
}

const TEAM_SELECTION_KEY = 'vola:selected-team-id'
const SKILL_SCOPE_KEY = 'vola:skills-scope'

function titleFromPath(path: string) {
  const name = path.split('/').filter(Boolean).pop() || path
  return name.replace(/\.[^.]+$/, '').replace(/[-_]/g, ' ')
}

function bundleIdFromSkillPath(path: string) {
  return skillBundlePathFromSkillPath(path).replace(/^\/skills\/?/, '')
}

function isEditableFile(entry: FileNode) {
  if (entry.is_dir) return false
  return isTextLikeFile(entry.name, entry.mime_type)
}

function normalizeBundleName(value: string) {
  return value.trim().replace(/^\/+|\/+$/g, '').replace(/\s+/g, '-')
}

function skillStarterMarkdown(name: string) {
  return `---
name: ${name}
description:
---

# ${name}

## Use when

Describe when this skill should be used.

## Avoid when

Describe when this skill should not be used.
`
}

function normalizeSkillPath(path: string) {
  return skillBundlePathFromSkillPath(path).replace(/\/+$/g, '')
}

function dateMillis(value?: string) {
  if (!value) return 0
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function personalSkillCopyLookup(skills: SkillSummary[], skillsRoot: FileNode | null) {
  const folderLookup = (skillsRoot?.children || []).reduce<Record<string, FileNode>>((acc, child) => {
    acc[normalizeSkillPath(child.path)] = child
    return acc
  }, {})
  return skills
    .filter((skill) => skill.path.startsWith('/skills/'))
    .reduce<Record<string, PersonalSkillCopyInfo>>((acc, skill) => {
      const bundlePath = normalizeSkillPath(skill.bundle_path || skillBundlePathFromSkillPath(skill.path))
      const folder = folderLookup[bundlePath]
      acc[bundlePath] = {
        bundlePath,
        created_at: folder?.created_at,
        updated_at: folder?.updated_at || folder?.created_at,
      }
      return acc
    }, {})
}

function teamSkillSubscriptionLookup(items: TeamSkillSubscriptionStatus[]) {
  return items.reduce<Record<string, TeamSkillSubscriptionStatus>>((acc, item) => {
    acc[normalizeSkillPath(item.source_path)] = item
    return acc
  }, {})
}

function teamSkillPublicationLookup(items: TeamSkillPublication[]) {
  return items.reduce<Record<string, TeamSkillPublication>>((acc, item) => {
    acc[normalizeSkillPath(item.skill_path)] = item
    return acc
  }, {})
}

function defaultTeamSkillPublication(skill: SkillBundle): TeamSkillPublication {
  return {
    skill_path: normalizeSkillPath(skill.bundlePath || skill.path),
    status: 'published',
    visibility: 'team',
    implicit: true,
  }
}

function teamSkillInstallStatus(
  skill: SkillBundle,
  personalLookup: Record<string, PersonalSkillCopyInfo>,
  subscriptions: Record<string, TeamSkillSubscriptionStatus> = {},
): TeamSkillInstallStatus {
  const bundlePath = normalizeSkillPath(skill.bundlePath || skill.path)
  const subscription = subscriptions[bundlePath]
  if (subscription) {
    if (subscription.update_available) return 'update_available'
    return 'installed'
  }
  const personal = personalLookup[bundlePath]
  if (!personal) return 'not_installed'
  const teamUpdatedAt = dateMillis(skill.updated_at || skill.created_at)
  const personalUpdatedAt = dateMillis(personal.updated_at || personal.created_at)
  if (teamUpdatedAt && personalUpdatedAt && teamUpdatedAt > personalUpdatedAt + 1000) return 'update_available'
  if (teamUpdatedAt && personalUpdatedAt && personalUpdatedAt > teamUpdatedAt + 1000) return 'personal_newer'
  return 'installed'
}

function teamSkillInstallLabel(status: TeamSkillInstallStatus, tx: (zh: string, en: string) => string) {
  if (status === 'not_installed') return tx('未安装到个人', 'Not installed')
  if (status === 'update_available') return tx('团队有新版', 'Team update available')
  if (status === 'personal_newer') return tx('个人副本较新', 'Personal copy newer')
  return tx('已安装到个人', 'Installed')
}

function teamSkillInstallHint(status: TeamSkillInstallStatus, tx: (zh: string, en: string) => string) {
  if (status === 'not_installed') return tx('安装后可在个人空间分配给 Codex、Claude、Cursor 等工具。', 'Install it into your personal space, then assign it to Codex, Claude, Cursor, and other tools.')
  if (status === 'update_available') return tx('团队版本更新了，可以把个人副本更新到团队版本。', 'The team version changed; update your personal copy when ready.')
  if (status === 'personal_newer') return tx('你的个人副本比团队版本新，建议人工确认后再覆盖。', 'Your personal copy is newer than the team version. Review before overwriting.')
  return tx('个人空间已有同名 Skill，可直接用于 Agent 分配。', 'A matching personal skill is available for agent assignment.')
}

function teamSkillInstallActionLabel(status: TeamSkillInstallStatus, tx: (zh: string, en: string) => string) {
  if (status === 'not_installed') return tx('安装到个人', 'Install')
  if (status === 'update_available') return tx('更新个人副本', 'Update copy')
  return ''
}

function teamSkillPublicationLabel(publication: TeamSkillPublication, tx: (zh: string, en: string) => string) {
  if (publication.status === 'archived') return tx('已归档', 'Archived')
  if (publication.status === 'published' && publication.visibility === 'team') return publication.implicit ? tx('团队可见', 'Team visible') : tx('已发布', 'Published')
  return tx('草稿', 'Draft')
}

function teamSkillReviewLabel(publication: TeamSkillPublication, tx: (zh: string, en: string) => string) {
  if (publication.review_status === 'requested') return tx('待审查', 'Review requested')
  if (publication.review_status === 'approved') return tx('已通过', 'Approved')
  if (publication.review_status === 'changes_requested') return tx('需修改', 'Changes requested')
  return ''
}

function assignmentLookup(assignments: SkillAgentAssignment[]) {
  return assignments.reduce<Record<string, Set<string>>>((acc, item) => {
    acc[item.agent_id] = new Set((item.skill_paths || []).map(normalizeSkillPath))
    return acc
  }, {})
}

function normalizeAssignments(assignments: SkillAgentAssignment[], agentIds: string[]) {
  const lookup = assignmentLookup(assignments)
  return agentIds.map((agentId) => ({
    agent_id: agentId,
    skill_paths: Array.from(lookup[agentId] || []).sort(),
  }))
}

function diffSkillPaths(saved: Set<string>, draft: Set<string>) {
  const added: string[] = []
  const removed: string[] = []
  const kept: string[] = []
  draft.forEach((path) => {
    if (saved.has(path)) kept.push(path)
    else added.push(path)
  })
  saved.forEach((path) => {
    if (!draft.has(path)) removed.push(path)
  })
  return {
    added: added.sort(),
    removed: removed.sort(),
    kept: kept.sort(),
  }
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
    `新增 ${summary.add} / 更新 ${summary.update} / 冲突 ${summary.conflict} / 需验证 ${summary.sync_risk || 0} / 可清理 ${summary.removable} / 可导出 ${summary.export}`,
    `add ${summary.add} / update ${summary.update} / conflicts ${summary.conflict} / verify ${summary.sync_risk || 0} / removable ${summary.removable} / export ${summary.export}`,
  )
}

function syncResponseTotal(response: LocalSkillSyncResponse | null) {
  if (!response) return { add: 0, update: 0, conflict: 0, removable: 0, export: 0, written: 0, deleted: 0, syncRisk: 0, blocked: 0, manual: 0 }
  return response.agents.reduce((acc, agent) => {
    acc.add += agent.summary.add
    acc.update += agent.summary.update
    acc.conflict += agent.summary.conflict
    acc.removable += agent.summary.removable
    acc.export += agent.summary.export
    acc.written += agent.summary.written
    acc.deleted += agent.summary.deleted
    acc.syncRisk += agent.summary.sync_risk || 0
    acc.blocked += agent.summary.blocked || 0
    acc.manual += agent.summary.manual_required || 0
    return acc
  }, { add: 0, update: 0, conflict: 0, removable: 0, export: 0, written: 0, deleted: 0, syncRisk: 0, blocked: 0, manual: 0 })
}

function learningStatusLabel(status: string, tx: (zh: string, en: string) => string) {
  if (status === 'needs_summary') return tx('缺用途说明', 'Needs summary')
  if (status === 'needs_validation') return tx('需验证', 'Needs validation')
  if (status === 'ready') return tx('可用', 'Ready')
  return status
}

function qualityStatusLabel(status: string | undefined, tx: (zh: string, en: string) => string) {
  if (status === 'blocked') return tx('阻断', 'Blocked')
  if (status === 'manual_required') return tx('需审查', 'Review')
  if (status === 'warning') return tx('提醒', 'Warning')
  if (status === 'passed') return tx('通过', 'Passed')
  return tx('未评估', 'Unknown')
}

function learningRunStatusLabel(status: string, tx: (zh: string, en: string) => string) {
  if (status === 'completed') return tx('已完成', 'Completed')
  if (status === 'running') return tx('生成中', 'Running')
  if (status === 'failed') return tx('失败', 'Failed')
  return status
}

function conversionActionClass(action: string) {
  if (action === 'convert') return 'preview-action preview-action-update'
  if (action === 'copy') return 'preview-action preview-action-create'
  if (action === 'generate') return 'preview-action preview-action-skip'
  if (action === 'conflict') return 'preview-action preview-action-conflict'
  return 'preview-action preview-action-skip'
}

function conversionActionLabel(action: string, tx: (zh: string, en: string) => string) {
  if (action === 'convert') return tx('转换', 'convert')
  if (action === 'copy') return tx('复制', 'copy')
  if (action === 'generate') return tx('生成', 'generate')
  if (action === 'conflict') return tx('冲突', 'conflict')
  return action
}

function defaultConversionTargetPath(sourcePath: string, targetPlatform: 'claude-code' | 'codex') {
  const clean = normalizeSkillPath(sourcePath || '/skills/skill')
  const suffix = targetPlatform === 'codex' ? 'codex' : 'claude'
  return `${clean}-${suffix}`
}

function emptyTreeNode(path: string): FileNode {
  const normalized = path.startsWith('/') ? path : `/${path}`
  return {
    path: normalized,
    name: normalized.split('/').filter(Boolean).pop() || '/',
    is_dir: true,
    children: [],
  }
}

function teamMatches(team: Team, value: string) {
  return team.id === value || team.slug === value
}

function dataSkillsRoute(teamID?: string) {
  return teamID ? `/skills?team=${encodeURIComponent(teamID)}` : '/skills'
}

export default function DataSkillsPage() {
  const { locale, tx } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const params = useParams()
  const bundleKey = (params.bundleKey || '').trim()
  const query = useMemo(() => new URLSearchParams(location.search), [location.search])
  const currentRelativeDir = normalizeBundleRelativeDir(query.get('dir'))
  const currentBundlePath = bundleKey ? `/skills/${bundleKey}` : ''
  const currentBrowsePath = currentBundlePath ? bundleBrowsePath(currentBundlePath, currentRelativeDir) : ''
  const isBundleView = Boolean(currentBundlePath)

  const [skills, setSkills] = useState<SkillBundle[]>([])
  const [personalSkillCopies, setPersonalSkillCopies] = useState<Record<string, PersonalSkillCopyInfo>>({})
  const [teamSkillPublications, setTeamSkillPublications] = useState<TeamSkillPublication[]>([])
  const [teamSkillSubscriptions, setTeamSkillSubscriptions] = useState<TeamSkillSubscriptionStatus[]>([])
  const [bundleNode, setBundleNode] = useState<FileNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedBundlePath, setSelectedBundlePath] = useState<string | null>(null)
  const [selectedEntryPath, setSelectedEntryPath] = useState<string | null>(null)
  const [showNewForm, setShowNewForm] = useState(false)
  const [newBundleName, setNewBundleName] = useState('new-skill')
  const [creating, setCreating] = useState(false)
  const [sortKey, setSortKey] = useState<MaterialsSortKey>('updated_at')
  const [sortDir, setSortDir] = useState<MaterialsSortDir>('desc')
  const [sourceFilter, setSourceFilter] = useState('all')
  const [teams, setTeams] = useState<Team[]>([])
  const [skillScope, setSkillScope] = useState<SkillScope>(() => {
    if (typeof window === 'undefined') return 'personal'
    const queryTeam = new URLSearchParams(window.location.search).get('team')
    if (queryTeam) return 'team'
    return window.localStorage.getItem(SKILL_SCOPE_KEY) === 'team' ? 'team' : 'personal'
  })
  const [selectedTeamID, setSelectedTeamID] = useState(() => {
    if (typeof window === 'undefined') return ''
    return new URLSearchParams(window.location.search).get('team') || window.localStorage.getItem(TEAM_SELECTION_KEY) || ''
  })
  const [assignmentState, setAssignmentState] = useState<SkillAssignmentsState | null>(null)
  const [assignmentDraft, setAssignmentDraft] = useState<SkillAgentAssignment[]>([])
  const [learningSummary, setLearningSummary] = useState<SkillLearningSummary | null>(null)
  const [learningNotes, setLearningNotes] = useState<SkillLearningNote[]>([])
  const [learningRunBusy, setLearningRunBusy] = useState(false)
  const [learningRunMessage, setLearningRunMessage] = useState('')
  const [learningRunError, setLearningRunError] = useState('')
  const [pendingGrowthProposalCount, setPendingGrowthProposalCount] = useState(0)
  const [recommendQuery, setRecommendQuery] = useState('')
  const [recommendResult, setRecommendResult] = useState<SkillLearningSummary | null>(null)
  const [recommendBusy, setRecommendBusy] = useState(false)
  const [recommendError, setRecommendError] = useState('')
  const [assignmentSaving, setAssignmentSaving] = useState(false)
  const [assignmentMessage, setAssignmentMessage] = useState('')
  const [assignmentError, setAssignmentError] = useState('')
  const [localMode, setLocalMode] = useState(false)
  const [skillSyncPreview, setSkillSyncPreview] = useState<LocalSkillSyncResponse | null>(null)
  const [skillSyncBusy, setSkillSyncBusy] = useState<'preview' | 'apply' | 'cleanup' | ''>('')
  const [skillExportBusy, setSkillExportBusy] = useState('')
  const [skillSyncMessage, setSkillSyncMessage] = useState('')
  const [skillSyncError, setSkillSyncError] = useState('')
  const [qualityReviewAcknowledged, setQualityReviewAcknowledged] = useState(false)
  const [scopeMessage, setScopeMessage] = useState('')
  const [copyingSkillPath, setCopyingSkillPath] = useState('')
  const [teamSkillPublishingPath, setTeamSkillPublishingPath] = useState('')
  const [showDiffModal, setShowDiffModal] = useState(false)
  const [diffFiles, setDiffFiles] = useState<SkillDiffFileItem[]>([])
  const [diffTargetSkillPath, setDiffTargetSkillPath] = useState('')
  const [diffSourceSkillPath, setDiffSourceSkillPath] = useState('')
  const [backupsList, setBackupsList] = useState<any[]>([])
  const [rollbackSkillPath, setRollbackSkillPath] = useState('')
  const [showRollbackModal, setShowRollbackModal] = useState(false)
  const [rollingBack, setRollingBack] = useState(false)
  const [bulkUpdatingTeamSkills, setBulkUpdatingTeamSkills] = useState(false)
  const [conversionSourcePath, setConversionSourcePath] = useState('')
  const [conversionSourcePlatform, setConversionSourcePlatform] = useState<'claude-code' | 'codex'>('claude-code')
  const [conversionTargetPlatform, setConversionTargetPlatform] = useState<'claude-code' | 'codex'>('codex')
  const [conversionTargetPath, setConversionTargetPath] = useState('')
  const [conversionOverwrite, setConversionOverwrite] = useState(false)
  const [conversionPreview, setConversionPreview] = useState<SkillConversionResponse | null>(null)
  const [conversionBusy, setConversionBusy] = useState<'preview' | 'apply' | ''>('')
  const [conversionMessage, setConversionMessage] = useState('')
  const [conversionError, setConversionError] = useState('')
  const { activeMenuId, closeMenu, isMenuOpen, toggleMenu } = useResourceCardMenu()
  const [activeTab, setActiveTab] = useState<'library' | 'learning' | 'assignments'>('library')

  const selectedTeam = useMemo(
    () => teams.find((team) => teamMatches(team, selectedTeamID)) || null,
    [selectedTeamID, teams],
  )
  const activeTeamID = skillScope === 'team' ? selectedTeam?.id || '' : ''
  const activeTeamCanWrite = skillScope !== 'team' || Boolean(selectedTeam?.can_write)
  const skillScopeLabel = skillScope === 'team' && selectedTeam
    ? `${selectedTeam.name} / ${selectedTeam.slug}`
    : tx('个人空间', 'Personal')
  const canWriteCurrentScope = activeTeamCanWrite

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const searchParams = new URLSearchParams(location.search)
      const queryTeamID = searchParams.get('team') || ''
      const teamList = await api.getTeams().catch(() => [] as Team[])
      const requestedTeamID = queryTeamID || selectedTeamID || (typeof window !== 'undefined' ? window.localStorage.getItem(TEAM_SELECTION_KEY) || '' : '')
      const requestedScope: SkillScope = queryTeamID ? 'team' : skillScope
      const nextTeam = teamList.find((team) => teamMatches(team, requestedTeamID)) || (requestedScope === 'team' ? teamList[0] || null : null)
      const nextScope: SkillScope = requestedScope === 'team' && nextTeam ? 'team' : 'personal'
      const teamID = nextScope === 'team' && nextTeam ? nextTeam.id : ''

      setTeams(teamList)
      setSkillScope(nextScope)
      setSelectedTeamID(teamID)
      if (typeof window !== 'undefined') {
        window.localStorage.setItem(SKILL_SCOPE_KEY, nextScope)
        if (teamID) window.localStorage.setItem(TEAM_SELECTION_KEY, teamID)
      }

      if (isBundleView) {
        const node = teamID ? await api.getTeamTree(teamID, currentBrowsePath) : await api.getTree(currentBrowsePath)
        setBundleNode(node)
        setSkills([])
        setAssignmentState(null)
        setAssignmentDraft([])
        setLearningSummary(null)
        closeMenu()
        setSelectedEntryPath(null)
        return
      }

      const [skillData, skillsRootResult, assignments, learning, notes, growthProposals, publicConfig, personalSkillData, personalSkillsRoot, publications, subscriptions] = await Promise.all([
        teamID ? api.getTeamSkills(teamID) : api.getSkills(),
        (teamID ? api.getTeamTree(teamID, '/skills') : api.getTree('/skills')).catch(() => emptyTreeNode('/skills')),
        api.getSkillAssignments(teamID || undefined),
        api.getSkillLearningSummary(teamID || undefined),
        api.getSkillLearningNotes(teamID || undefined, 14).catch(() => ({ notes: [] })),
        api.getGrowthProposals(teamID || undefined, 'pending_review').catch(() => ({ proposals: [] })),
        api.getPublicConfig(),
        teamID ? api.getSkills().catch(() => [] as SkillSummary[]) : Promise.resolve([] as SkillSummary[]),
        teamID ? api.getTree('/skills').catch(() => emptyTreeNode('/skills')) : Promise.resolve(emptyTreeNode('/skills')),
        teamID ? api.getTeamSkillPublications(teamID).catch(() => ({ publications: [] as TeamSkillPublication[] })) : Promise.resolve({ publications: [] as TeamSkillPublication[] }),
        teamID ? api.getTeamSkillSubscriptions(teamID).catch(() => ({ subscriptions: [] as TeamSkillSubscriptionStatus[] })) : Promise.resolve({ subscriptions: [] as TeamSkillSubscriptionStatus[] }),
      ])
      const skillsRoot = skillsRootResult || emptyTreeNode('/skills')

      const folderLookup = (skillsRoot.children || []).reduce<Record<string, FileNode>>((acc, child) => {
        acc[child.path] = child
        return acc
      }, {})

      const bundles = skillData
        .filter((skill) => skill.path.startsWith('/skills/'))
        .map((skill) => {
          const bundlePath = skillBundlePathFromSkillPath(skill.path)
          const folder = folderLookup[bundlePath]
          return {
            ...skill,
            bundleId: bundleIdFromSkillPath(skill.path),
            bundlePath,
            created_at: folder?.created_at,
            updated_at: folder?.updated_at || folder?.created_at,
          }
        })

      const firstBundlePath = bundles[0] ? normalizeSkillPath(bundles[0].bundlePath || bundles[0].path) : ''

      setSkills(bundles)
      setPersonalSkillCopies(teamID ? personalSkillCopyLookup(personalSkillData, personalSkillsRoot) : {})
      setTeamSkillPublications(teamID ? publications.publications || [] : [])
      setTeamSkillSubscriptions(teamID ? subscriptions.subscriptions || [] : [])
      setLocalMode(Boolean(publicConfig.local_mode))
      setAssignmentState(assignments)
      setLearningSummary(learning)
      setLearningNotes(notes.notes || [])
      setPendingGrowthProposalCount(growthProposals.proposals?.length || 0)
      setLearningRunMessage('')
      setLearningRunError('')
      setRecommendResult(null)
      setRecommendError('')
      setAssignmentDraft(normalizeAssignments(assignments.assignments || [], (assignments.agents || []).map((agent) => agent.id)))
      setConversionSourcePath((value) => value || firstBundlePath)
      setConversionTargetPath((value) => value || (firstBundlePath ? defaultConversionTargetPath(firstBundlePath, 'codex') : ''))
      setAssignmentMessage('')
      setAssignmentError('')
      setSkillSyncPreview(null)
      setSkillSyncMessage('')
      setSkillSyncError('')
      setQualityReviewAcknowledged(false)
      setScopeMessage('')
      setTeamSkillPublishingPath('')
      setBundleNode(null)
      closeMenu()
      setSelectedBundlePath(null)
    } catch (err: any) {
      setError(err.message || tx('加载技能失败', 'Failed to load skills'))
    } finally {
      setLoading(false)
    }
  }, [closeMenu, currentBrowsePath, isBundleView, location.search, selectedTeamID, skillScope, tx])

  const scopedGetNode = useCallback((path: string) => (
    activeTeamID ? api.getTeamTree(activeTeamID, path) : api.getTree(path)
  ), [activeTeamID])

  const scopedDeleteNode = useCallback((path: string) => (
    activeTeamID ? api.deleteTeamTree(activeTeamID, path) : api.deleteTree(path)
  ), [activeTeamID])

  const clearScopeMessages = useCallback(() => {
    setError('')
    setAssignmentMessage('')
    setAssignmentError('')
    setSkillSyncPreview(null)
    setSkillSyncMessage('')
    setSkillSyncError('')
    setQualityReviewAcknowledged(false)
    setScopeMessage('')
    setConversionPreview(null)
    setConversionMessage('')
    setConversionError('')
    setRecommendResult(null)
    setRecommendError('')
    setLearningRunMessage('')
    setLearningRunError('')
    closeMenu()
  }, [closeMenu])

  const navigateToCurrentView = useCallback((teamID?: string) => {
    if (isBundleView) {
      navigate(dataSkillBundleRoute(bundleKey, currentRelativeDir, teamID), { replace: true })
      return
    }
    navigate(dataSkillsRoute(teamID), { replace: true })
  }, [bundleKey, currentRelativeDir, isBundleView, navigate])

  const selectPersonalScope = useCallback(() => {
    setSkillScope('personal')
    setSelectedTeamID('')
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(SKILL_SCOPE_KEY, 'personal')
    }
    clearScopeMessages()
    navigateToCurrentView()
  }, [clearScopeMessages, navigateToCurrentView])

  const selectTeamScope = useCallback((teamID?: string) => {
    const savedTeamID = typeof window !== 'undefined' ? window.localStorage.getItem(TEAM_SELECTION_KEY) || '' : ''
    const nextTeam = teams.find((team) => teamMatches(team, teamID || selectedTeamID || savedTeamID)) || selectedTeam || teams[0] || null
    if (!nextTeam) {
      setError(tx('还没有可用团队。', 'No team is available yet.'))
      return
    }
    setSkillScope('team')
    setSelectedTeamID(nextTeam.id)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(SKILL_SCOPE_KEY, 'team')
      window.localStorage.setItem(TEAM_SELECTION_KEY, nextTeam.id)
    }
    clearScopeMessages()
    navigateToCurrentView(nextTeam.id)
  }, [clearScopeMessages, navigateToCurrentView, selectedTeam, selectedTeamID, teams, tx])

  const {
    closeDialog: closeDeleteDialog,
    confirmDelete,
    dialog: deleteDialog,
    requestDelete,
    submitting: deleteSubmitting,
  } = useTreeDeleteDialog({ tx, onDeleted: load, getNode: scopedGetNode, deleteNode: scopedDeleteNode })

  useEffect(() => {
    void load()
  }, [load])

  const currentBundleContext = bundleNode?.bundle_context
  const bundleEntries = bundleNode?.children || []
  const selectedBundle = selectedBundlePath
    ? skills.find((skill) => skill.bundlePath === selectedBundlePath) || null
    : null
  const selectedDeletePath = isBundleView ? selectedEntryPath : selectedBundle?.bundlePath || null
  const canDeleteSelection = Boolean(
    selectedDeletePath && canWriteCurrentScope && !(isBundleView ? currentBundleContext?.read_only : selectedBundle?.read_only),
  )

  const sortedSkills = useMemo(
    () =>
      sortMaterialsItems({
        items: skills,
        sortKey,
        sortDir,
        getName: (skill) => skill.name,
        getUpdatedAt: (skill) => skill.updated_at || skill.created_at,
      }),
    [skills, sortDir, sortKey],
  )
  const filteredSkills = useMemo(
    () => sortedSkills.filter((skill) => matchesSourceFilter(skillSource(skill), sourceFilter)),
    [sortedSkills, sourceFilter],
  )
  const teamSkillSubscriptionByPath = useMemo(
    () => teamSkillSubscriptionLookup(teamSkillSubscriptions),
    [teamSkillSubscriptions],
  )
  const teamSkillPublicationByPath = useMemo(
    () => teamSkillPublicationLookup(teamSkillPublications),
    [teamSkillPublications],
  )
  const teamInstallSummary = useMemo(() => {
    return skills.reduce((acc, skill) => {
      if (!activeTeamID) return acc
      const status = teamSkillInstallStatus(skill, personalSkillCopies, teamSkillSubscriptionByPath)
      acc[status] += 1
      return acc
    }, {
      not_installed: 0,
      installed: 0,
      update_available: 0,
      personal_newer: 0,
    } as Record<TeamSkillInstallStatus, number>)
  }, [activeTeamID, personalSkillCopies, skills, teamSkillSubscriptionByPath])
  const teamPublicationSummary = useMemo(() => {
    return skills.reduce((acc, skill) => {
      if (!activeTeamID) return acc
      const publication = teamSkillPublicationByPath[normalizeSkillPath(skill.bundlePath || skill.path)] || defaultTeamSkillPublication(skill)
      if (publication.status === 'archived') acc.archived += 1
      else if (publication.status === 'published' && publication.visibility === 'team') acc.published += 1
      else acc.draft += 1
      return acc
    }, { published: 0, draft: 0, archived: 0 })
  }, [activeTeamID, skills, teamSkillPublicationByPath])
  const skillSourceOptions = useMemo(
    () => buildSourceFilterOptions(skills, skillSource, locale),
    [locale, skills],
  )
  const savedAssignmentLookup = useMemo(
    () => assignmentLookup(assignmentState?.assignments || []),
    [assignmentState],
  )
  const draftAssignmentLookup = useMemo(
    () => assignmentLookup(assignmentDraft),
    [assignmentDraft],
  )
  const skillNameByPath = useMemo(
    () => skills.reduce<Record<string, string>>((acc, skill) => {
      acc[normalizeSkillPath(skill.bundlePath || skill.path)] = skill.name
      return acc
    }, {}),
    [skills],
  )
  const assignmentChanged = useMemo(() => {
    const agentIds = assignmentState?.agents?.map((agent) => agent.id) || []
    return agentIds.some((agentId) => {
      const saved = savedAssignmentLookup[agentId] || new Set<string>()
      const draft = draftAssignmentLookup[agentId] || new Set<string>()
      if (saved.size !== draft.size) return true
      for (const skillPath of draft) {
        if (!saved.has(skillPath)) return true
      }
      return false
    })
  }, [assignmentState, draftAssignmentLookup, savedAssignmentLookup])

  const sortedBundleEntries = useMemo(
    () =>
      sortMaterialsItems({
        items: bundleEntries,
        sortKey,
        sortDir,
        getName: (entry) => entry.name,
        getUpdatedAt: (entry) => entry.updated_at || entry.created_at,
        groupComparator: (left, right) => {
          if (left.name === 'SKILL.md' && right.name !== 'SKILL.md') return -1
          if (right.name === 'SKILL.md' && left.name !== 'SKILL.md') return 1
          if (left.is_dir !== right.is_dir) return left.is_dir ? -1 : 1
          return 0
        },
      }),
    [bundleEntries, sortDir, sortKey],
  )
  const filteredBundleEntries = useMemo(
    () => sortedBundleEntries.filter((entry) => matchesSourceFilter(fileNodeSource(entry), sourceFilter)),
    [sortedBundleEntries, sourceFilter],
  )
  const bundleSourceOptions = useMemo(
    () => buildSourceFilterOptions(bundleEntries, fileNodeSource, locale),
    [bundleEntries, locale],
  )

  const openBundleDetail = useCallback((bundleId: string, relativeDir = '') => {
    closeMenu()
    navigate(dataSkillBundleRoute(bundleId, relativeDir, activeTeamID))
  }, [activeTeamID, closeMenu, navigate])

  const openFileEditor = useCallback((path: string) => {
    closeMenu()
    navigate(dataFileEditorRoute(path, activeTeamID))
  }, [activeTeamID, closeMenu, navigate])

  const openBundleFolder = useCallback((path: string) => {
    if (!bundleKey) return
    openBundleDetail(bundleKey, bundleRelativeDirFromPath(currentBundlePath, path))
  }, [bundleKey, currentBundlePath, openBundleDetail])

  const handleDownloadZip = useCallback(async (path: string) => {
    closeMenu()
    try {
      if (activeTeamID) await api.downloadTeamTreeZip(activeTeamID, path)
      else await api.downloadTreeZip(path)
    } catch (err: any) {
      setError(err.message || tx('下载 ZIP 失败', 'Failed to download ZIP'))
    }
  }, [activeTeamID, closeMenu, tx])

  const copyTeamSkillToPersonal = useCallback(async (sourcePath: string, overwrite = false) => {
    if (!activeTeamID || copyingSkillPath) return
    const normalizedPath = normalizeSkillPath(sourcePath)
    setCopyingSkillPath(normalizedPath)
    setError('')
    setScopeMessage('')
    try {
      const response = await api.copyTeamSkillToPersonal(activeTeamID, normalizedPath, undefined, overwrite)
      setScopeMessage(tx(
        overwrite
          ? `已更新个人副本：${response.data.target_path}`
          : `已安装到个人空间：${response.data.target_path}`,
        overwrite
          ? `Personal copy updated: ${response.data.target_path}`
          : `Installed to personal space: ${response.data.target_path}`,
      ))
      void load()
    } catch (err: any) {
      setError(err.message || tx('安装到个人空间失败', 'Failed to install to personal space'))
    } finally {
      setCopyingSkillPath('')
    }
  }, [activeTeamID, copyingSkillPath, load, tx])

  const handleCopySkillAction = useCallback(async (sourcePath: string, overwrite: boolean) => {
    if (overwrite && activeTeamID) {
      const targetPath = normalizeSkillPath(sourcePath)
      setCopyingSkillPath(targetPath)
      try {
        const diffData = await api.diffTeamSkillSubscription(activeTeamID, targetPath, targetPath)
        setDiffFiles(diffData.files)
        setDiffTargetSkillPath(targetPath)
        setDiffSourceSkillPath(sourcePath)
        setShowDiffModal(true)
      } catch (err: any) {
        setError(err.message || tx('获取版本差异失败', 'Failed to fetch version diff'))
      } finally {
        setCopyingSkillPath('')
      }
    } else {
      void copyTeamSkillToPersonal(sourcePath, false)
    }
  }, [activeTeamID, copyTeamSkillToPersonal, tx])

  const handleOpenRollback = useCallback(async (targetPath: string) => {
    const skillName = targetPath.split('/').filter(Boolean).pop() || ''
    setError('')
    try {
      const tree = await api.getTree(`/settings/team-skill-backups/${skillName}`)
      const list = (tree.children || []).filter(item => !item.is_dir && item.name.endsWith('-backup.zip'))
      setBackupsList(list)
      setRollbackSkillPath(targetPath)
      setShowRollbackModal(true)
    } catch (err: any) {
      setError(tx('暂无该技能的历史备份记录', 'No backup records found for this skill'))
    }
  }, [tx])

  const handleRollbackExecute = useCallback(async (backupFilePath: string) => {
    if (!rollbackSkillPath) return
    setError('')
    setScopeMessage('')
    try {
      const resp = await api.rollbackTeamSkillSubscription(rollbackSkillPath, backupFilePath)
      if (resp.data.success) {
        setScopeMessage(tx('成功回滚到历史版本！', 'Successfully rolled back to historical version!'))
        setShowRollbackModal(false)
        void load()
      }
    } catch (err: any) {
      setError(err.message || tx('回滚失败', 'Failed to rollback'))
    }
  }, [rollbackSkillPath, load, tx])

  const updateTeamSkillPublication = useCallback(async (sourcePath: string, status: 'draft' | 'published' | 'archived') => {
    if (!activeTeamID || teamSkillPublishingPath) return
    const normalizedPath = normalizeSkillPath(sourcePath)
    setTeamSkillPublishingPath(normalizedPath)
    setError('')
    setScopeMessage('')
    try {
      const response = await api.saveTeamSkillPublication(activeTeamID, {
        skill_path: normalizedPath,
        status,
        visibility: status === 'published' ? 'team' : 'private',
      })
      setTeamSkillPublications(response.publications || [])
      setScopeMessage(status === 'published'
        ? tx('已发布给团队成员。', 'Published for team members.')
        : status === 'archived'
          ? tx('已归档，普通成员不会再看到它。', 'Archived. Regular members will no longer see it.')
          : tx('已转为草稿，仅管理员可见。', 'Moved to draft. Only admins can see it.'))
    } catch (err: any) {
      setError(err.message || tx('更新团队可见性失败', 'Failed to update team visibility'))
    } finally {
      setTeamSkillPublishingPath('')
    }
  }, [activeTeamID, teamSkillPublishingPath, tx])

  const requestTeamSkillReview = useCallback(async (sourcePath: string) => {
    if (!activeTeamID || teamSkillPublishingPath) return
    const normalizedPath = normalizeSkillPath(sourcePath)
    setTeamSkillPublishingPath(normalizedPath)
    setError('')
    setScopeMessage('')
    try {
      await api.requestTeamSkillReview(activeTeamID, {
        asset_type: 'skill',
        skill_path: normalizedPath,
      })
      setScopeMessage(tx('已提交给管理员审查。', 'Review requested for admins.'))
      await load()
    } catch (err: any) {
      setError(err.message || tx('提交审查失败', 'Failed to request review'))
    } finally {
      setTeamSkillPublishingPath('')
    }
  }, [activeTeamID, load, teamSkillPublishingPath, tx])

  const resolveTeamSkillReview = useCallback(async (sourcePath: string, decision: 'approved' | 'changes_requested') => {
    if (!activeTeamID || teamSkillPublishingPath) return
    const normalizedPath = normalizeSkillPath(sourcePath)
    setTeamSkillPublishingPath(normalizedPath)
    setError('')
    setScopeMessage('')
    try {
      await api.resolveTeamSkillReview(activeTeamID, {
        asset_type: 'skill',
        skill_path: normalizedPath,
        decision,
      })
      setScopeMessage(decision === 'approved'
        ? tx('审查已通过，团队成员可见。', 'Review approved. Team members can see it.')
        : tx('已要求修改，普通成员暂不可见。', 'Changes requested. Regular members cannot see it yet.'))
      await load()
    } catch (err: any) {
      setError(err.message || tx('处理审查失败', 'Failed to resolve review'))
    } finally {
      setTeamSkillPublishingPath('')
    }
  }, [activeTeamID, load, teamSkillPublishingPath, tx])

  const updateAllTeamSkillCopies = useCallback(async () => {
    if (!activeTeamID || bulkUpdatingTeamSkills) return
    const updates = Object.values(teamSkillSubscriptionByPath).filter((item) => item.update_available && !item.source_missing)
    if (updates.length === 0) return
    setBulkUpdatingTeamSkills(true)
    setError('')
    setScopeMessage('')
    try {
      for (const item of updates) {
        await api.copyTeamSkillToPersonal(activeTeamID, item.source_path, item.target_path, true)
      }
      setScopeMessage(tx(`已更新 ${updates.length} 个个人 Skill 副本。`, `Updated ${updates.length} personal skill copies.`))
      await load()
    } catch (err: any) {
      setError(err.message || tx('批量更新个人副本失败', 'Failed to update personal copies'))
    } finally {
      setBulkUpdatingTeamSkills(false)
    }
  }, [activeTeamID, bulkUpdatingTeamSkills, load, teamSkillSubscriptionByPath, tx])

  const handleCreateSkill = async (event: FormEvent) => {
    event.preventDefault()
    const bundleName = normalizeBundleName(newBundleName)
    if (!bundleName) return
    if (!canWriteCurrentScope) {
      setError(tx('当前团队角色只能查看，不能创建 Skill。', 'Your current team role can only view skills.'))
      return
    }

    setCreating(true)
    setError('')
    try {
      const path = `/skills/${bundleName}/SKILL.md`
      if (activeTeamID) {
        await api.writeTeamTree(activeTeamID, path, {
          content: skillStarterMarkdown(bundleName),
          mimeType: 'text/markdown',
          metadata: { source: 'manual', team_id: activeTeamID },
        })
        if (selectedTeam?.can_manage_members) {
          await api.saveTeamSkillPublication(activeTeamID, {
            skill_path: `/skills/${bundleName}`,
            status: 'draft',
            visibility: 'private',
          }).catch(() => null)
        } else {
          await api.requestTeamSkillReview(activeTeamID, {
            asset_type: 'skill',
            skill_path: `/skills/${bundleName}`,
            note: 'Created by a team member.',
          }).catch(() => null)
        }
      } else {
        await api.writeTree(path, {
          content: skillStarterMarkdown(bundleName),
          mimeType: 'text/markdown',
          metadata: { source: 'manual' },
        })
      }
      setShowNewForm(false)
      setNewBundleName('new-skill')
      navigate(dataFileEditorRoute(path, activeTeamID))
    } catch (err: any) {
      setError(err.message || tx('新建技能失败', 'Failed to create skill'))
    } finally {
      setCreating(false)
    }
  }

  const toggleSkillAssignment = (agentId: string, skillPath: string) => {
    if (!canWriteCurrentScope) return
    const normalizedPath = normalizeSkillPath(skillPath)
    setAssignmentMessage('')
    setAssignmentError('')
    setSkillSyncPreview(null)
    setSkillSyncMessage('')
    setSkillSyncError('')
    setAssignmentDraft((current) => {
      const agentIds = assignmentState?.agents?.map((agent) => agent.id) || []
      const next = normalizeAssignments(current, agentIds)
      const index = next.findIndex((item) => item.agent_id === agentId)
      if (index < 0) return next
      const paths = new Set(next[index].skill_paths)
      if (paths.has(normalizedPath)) {
        paths.delete(normalizedPath)
      } else {
        paths.add(normalizedPath)
      }
      next[index] = {
        ...next[index],
        skill_paths: Array.from(paths).sort(),
      }
      return next
    })
  }

  const resetSkillAssignments = () => {
    const agentIds = assignmentState?.agents?.map((agent) => agent.id) || []
    setAssignmentDraft(normalizeAssignments(assignmentState?.assignments || [], agentIds))
    setAssignmentMessage('')
    setAssignmentError('')
    setSkillSyncPreview(null)
    setSkillSyncMessage('')
    setSkillSyncError('')
  }

  const runSkillRecommendation = async (event: FormEvent) => {
    event.preventDefault()
    const queryText = recommendQuery.trim()
    if (!queryText) return
    setRecommendBusy(true)
    setRecommendError('')
    try {
      const result = await api.recommendSkills(queryText, activeTeamID || undefined)
      setRecommendResult(result)
    } catch (err: any) {
      setRecommendError(err.message || tx('推荐 Skill 失败', 'Failed to recommend skills'))
    } finally {
      setRecommendBusy(false)
    }
  }

  const createLearningRun = async () => {
    if (learningRunBusy) return
    setLearningRunBusy(true)
    setLearningRunError('')
    setLearningRunMessage('')
    try {
      const response = await api.createSkillLearningRun(activeTeamID || undefined)
      setLearningSummary((current) => ({
        version: response.version,
        scope: response.scope,
        team: response.team,
        stats: response.summary.stats,
        items: response.summary.items,
        actions: response.summary.actions,
        latest_run: response.run,
      }))
      const notes = await api.getSkillLearningNotes(activeTeamID || undefined, 14).catch(() => ({ notes: [] }))
      const growthProposals = await api.getGrowthProposals(activeTeamID || undefined, 'pending_review').catch(() => ({ proposals: [] }))
      setLearningNotes(notes.notes || [])
      setPendingGrowthProposalCount(growthProposals.proposals?.length || 0)
      setLearningRunMessage(tx('学习报告已生成。', 'Learning report created.'))
    } catch (err: any) {
      setLearningRunError(err.message || tx('生成学习报告失败', 'Failed to create learning report'))
    } finally {
      setLearningRunBusy(false)
    }
  }

  const saveSkillAssignments = async () => {
    if (!assignmentState || assignmentSaving) return
    setAssignmentSaving(true)
    setAssignmentError('')
    setAssignmentMessage('')
    try {
      const normalized = normalizeAssignments(assignmentDraft, assignmentState.agents.map((agent) => agent.id))
      const saved = await api.saveSkillAssignments(normalized, activeTeamID || undefined)
      setAssignmentState(saved.data)
      setAssignmentDraft(normalizeAssignments(saved.data.assignments || [], saved.data.agents.map((agent) => agent.id)))
      setAssignmentMessage(tx('分配已保存。', 'Assignments saved.'))
      setSkillSyncPreview(null)
      setSkillSyncMessage('')
      setSkillSyncError('')
      setQualityReviewAcknowledged(false)
    } catch (err: any) {
      setAssignmentError(err.message || tx('保存分配失败', 'Failed to save assignments'))
    } finally {
      setAssignmentSaving(false)
    }
  }

  const runLocalSkillSync = async (mode: 'preview' | 'apply' | 'cleanup') => {
    if (assignmentChanged || skillSyncBusy) return
    const previewTotal = syncResponseTotal(skillSyncPreview)
    if (mode === 'apply' && previewTotal.blocked > 0) {
      setSkillSyncError(tx(
        `有 ${previewTotal.blocked} 个阻断项，处理后再应用到本地。`,
        `${previewTotal.blocked} blocked quality findings must be fixed before applying locally.`,
      ))
      return
    }
    if (mode === 'apply' && previewTotal.manual > 0 && !qualityReviewAcknowledged) {
      const confirmed = window.confirm(tx(
        `有 ${previewTotal.manual} 个需审查项。确认你已经看过同步预览和质量提示，再继续应用？`,
        `${previewTotal.manual} items require manual review. Confirm that you reviewed the preview and quality notes before applying?`,
      ))
      if (!confirmed) return
      setQualityReviewAcknowledged(true)
    }
    setSkillSyncBusy(mode)
    setSkillSyncError('')
    setSkillSyncMessage('')
    try {
      const response = mode === 'preview'
        ? await api.previewLocalSkillSync(activeTeamID || undefined)
        : mode === 'apply'
          ? await api.applyLocalSkillSync(activeTeamID || undefined, qualityReviewAcknowledged || previewTotal.manual > 0)
          : await api.cleanupLocalSkillSync(activeTeamID || undefined)
      setSkillSyncPreview(response)
      const total = syncResponseTotal(response)
      if (mode === 'preview') {
        setQualityReviewAcknowledged(false)
        setSkillSyncMessage(tx(
          `已生成预览：新增 ${total.add}，更新 ${total.update}，冲突 ${total.conflict}，可清理 ${total.removable}，可导出 ${total.export}。`,
          `Preview ready: ${total.add} add, ${total.update} update, ${total.conflict} conflicts, ${total.removable} removable, ${total.export} export.`,
        ))
      } else if (mode === 'apply') {
        if (response.blocked || total.written === 0 && (total.blocked > 0 || total.manual > 0)) {
          setSkillSyncError(tx(
            `未应用到本地：阻断 ${total.blocked}，需审查 ${total.manual}。`,
            `Not applied locally: ${total.blocked} blocked, ${total.manual} manual review.`,
          ))
        } else {
          setSkillSyncMessage(tx(
            `已应用到本地：写入 ${total.written} 个文件。`,
            `Applied locally: ${total.written} files written.`,
          ))
        }
      } else {
        setSkillSyncMessage(tx(
          `已清理 Vola 管理的未分配 Skill：${total.deleted} 个目录。`,
          `Cleaned ${total.deleted} Vola-managed unassigned skill folders.`,
        ))
      }
    } catch (err: any) {
      setSkillSyncError(err.message || tx('本地 Skill 同步失败', 'Local skill sync failed'))
    } finally {
      setSkillSyncBusy('')
    }
  }

  const downloadLocalSkillExport = async (agentId: string) => {
    if (assignmentChanged || skillExportBusy) return
    setSkillExportBusy(agentId)
    setSkillSyncError('')
    setSkillSyncMessage('')
    try {
      await api.downloadLocalSkillSyncExport(agentId, activeTeamID || undefined)
      setSkillSyncMessage(tx('导出包已生成。', 'Export package created.'))
    } catch (err: any) {
      setSkillSyncError(err.message || tx('生成导出包失败', 'Failed to create export package'))
    } finally {
      setSkillExportBusy('')
    }
  }

  const buildConversionRequest = (): SkillConversionRequest => ({
    source_path: conversionSourcePath,
    source_platform: conversionSourcePlatform,
    target_platform: conversionTargetPlatform,
    target_path: conversionTargetPath,
    overwrite: conversionOverwrite,
    team_id: activeTeamID || undefined,
  })

  const runSkillConversion = async (mode: 'preview' | 'apply') => {
    if (!conversionSourcePath || conversionBusy) return
    setConversionBusy(mode)
    setConversionError('')
    setConversionMessage('')
    try {
      const req = buildConversionRequest()
      const response = mode === 'preview'
        ? await api.previewSkillConversion(req)
        : (await api.applySkillConversion(req)).data
      setConversionPreview(response)
      if (mode === 'preview') {
        setConversionMessage(tx(
          `转换预览已生成：转换 ${response.summary.converted}，复制 ${response.summary.copied}，冲突 ${response.summary.conflicts}，需处理 ${response.summary.manual}。`,
          `Conversion preview ready: ${response.summary.converted} converted, ${response.summary.copied} copied, ${response.summary.conflicts} conflicts, ${response.summary.manual} need review.`,
        ))
      } else {
        setConversionMessage(tx(
          `已生成转换副本：${response.target_path}`,
          `Converted copy created: ${response.target_path}`,
        ))
        void load()
      }
    } catch (err: any) {
      setConversionError(err.message || tx('Skill 转换失败', 'Skill conversion failed'))
    } finally {
      setConversionBusy('')
    }
  }

  const sortOptions = getMaterialsSortOptions(locale)
  const relativeSegments = currentRelativeDir.split('/').filter(Boolean)
  const skillScopeControl = (
    <div className="skill-scope-control">
      <div className="materials-toggle-group" role="tablist" aria-label={tx('Skill 范围', 'Skill scope')}>
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
          disabled={teams.length === 0}
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
      <span className="materials-tile-pill skill-scope-pill">{skillScopeLabel}</span>
    </div>
  )

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (deleteDialog || activeMenuId) return
      if (event.key === 'Escape') {
        if (isBundleView) setSelectedEntryPath(null)
        else setSelectedBundlePath(null)
        return
      }
      if (event.key === 'Delete' && selectedDeletePath && canDeleteSelection) {
        event.preventDefault()
        void requestDelete([selectedDeletePath])
        return
      }
      if (event.key !== 'Enter') return
      if (isBundleView && selectedEntryPath) {
        const entry = bundleEntries.find((item) => item.path === selectedEntryPath)
        if (!entry) return
        if (entry.is_dir) {
          openBundleFolder(entry.path)
          return
        }
        if (isEditableFile(entry)) {
          openFileEditor(entry.path)
        }
        return
      }
      if (!isBundleView && selectedBundle) {
        openBundleDetail(selectedBundle.bundleId)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [
    activeMenuId,
    bundleEntries,
    canDeleteSelection,
    deleteDialog,
    isBundleView,
    openBundleDetail,
    openBundleFolder,
    openFileEditor,
    requestDelete,
    selectedBundle,
    selectedDeletePath,
    selectedEntryPath,
  ])

  if (loading) {
    return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>
  }

  if (isBundleView) {
    return (
      <div className="page materials-page">
        <section className="materials-hero">
          <div className="materials-hero-copy">
            <nav aria-label={tx('面包屑', 'Breadcrumbs')} className="materials-breadcrumbs">
              <button className="btn-text" onClick={() => navigate(dataSkillsRoute(activeTeamID))}>{tx('技能', 'Skills')}</button>
              {currentBundleContext ? (
                <>
                  <span className="breadcrumbs-sep">/</span>
                  <button className="btn-text" onClick={() => openBundleDetail(bundleKey)}>{currentBundleContext.name}</button>
                </>
              ) : null}
              {relativeSegments.map((segment, index) => {
                const relative = relativeSegments.slice(0, index + 1).join('/')
                return (
                  <span key={relative}>
                    <span className="breadcrumbs-sep">/</span>
                    <button className="btn-text" onClick={() => openBundleDetail(bundleKey, relative)}>{segment}</button>
                  </span>
                )
              })}
            </nav>
            <div className="materials-kicker">Vola Data</div>
            <h2 className="materials-title">{currentBundleContext?.name || bundleKey}</h2>
            <p className="materials-subtitle">
              {tx('在技能包内继续下钻时，顶部会持续显示该技能包的上下文。', 'The bundle context stays visible while you browse deeper inside this skill bundle.')}
            </p>
          </div>
          <div className="materials-actions">
            {skillScopeControl}
            {activeTeamID && currentBundleContext ? (
              personalSkillCopies[normalizeSkillPath(currentBundlePath)] ? (
                <span className="materials-tile-pill team-skill-install-pill is-installed">{tx('已安装到个人', 'Installed')}</span>
              ) : (
              <button
                className="btn"
                disabled={copyingSkillPath === normalizeSkillPath(currentBundlePath)}
                onClick={() => { void copyTeamSkillToPersonal(currentBundlePath) }}
              >
                {copyingSkillPath === normalizeSkillPath(currentBundlePath) ? tx('安装中...', 'Installing...') : tx('安装到个人空间', 'Install to personal')}
              </button>
              )
            ) : null}
          </div>
        </section>

        {error && <div className="alert alert-warn">{error}</div>}
        {scopeMessage && <div className="alert alert-success">{scopeMessage}</div>}
        {!error && !currentBundleContext && (
          <div className="alert alert-warn">{tx('没有找到这个技能包。', 'This skill bundle could not be found.')}</div>
        )}

        {currentBundleContext ? (
          <GitHubTreeList
            key={`${activeTeamID || 'personal'}:${currentBundlePath}`}
            rootPath={currentBundlePath}
            rootLabel={currentBundleContext.name}
            initialPath={currentBrowsePath || currentBundlePath}
            title={tx('技能文件', 'Skill files')}
            description={tx('按 GitHub 文件列表样式浏览这个技能包的所有文件。', 'Browse every file in this skill bundle with the GitHub-style file list.')}
            actionHref="/?type=Skill"
            actionLabel={tx('在首页查看', 'View on Home')}
            loadNode={scopedGetNode}
            fileRoute={(path) => dataFileEditorRoute(path, activeTeamID)}
          />
        ) : null}

        {currentBundleContext ? (
          <section className="materials-section">
            <div className="materials-section-head">
              <div>
                <h3 className="materials-section-title">{tx('Bundle 内容', 'Bundle contents')}</h3>
                <p className="materials-section-copy">{tx('这个 bundle 里的文件和文件夹按同一套文件卡片规则展示。', 'Files and folders inside this bundle use the same file-card system as the browser.')}</p>
              </div>
              <MaterialsSectionToolbar
                count={filteredBundleEntries.length}
                sortKey={sortKey}
                sortOptions={sortOptions}
                sortDir={sortDir}
                onSortKeyChange={(value) => setSortKey(value as MaterialsSortKey)}
                onSortDirToggle={() => setSortDir((value) => (value === 'desc' ? 'asc' : 'desc'))}
              >
                <button
                  className="btn btn-sm materials-toolbar-control is-danger"
                  disabled={!canDeleteSelection}
                  onClick={() => {
                    if (selectedDeletePath) void requestDelete([selectedDeletePath])
                  }}
                >
                  {tx('删除', 'Delete')}
                </button>
              </MaterialsSectionToolbar>
            </div>

            {(bundleSourceOptions.length > 1 || sourceFilter !== 'all') && (
              <SourceFilterBar options={bundleSourceOptions} value={sourceFilter} onChange={setSourceFilter} />
            )}

            {filteredBundleEntries.length === 0 ? (
              <p className="dashboard-empty-copy">{tx('这个 bundle 目录目前还是空的。', 'This bundle is currently empty.')}</p>
            ) : (
              <div className="materials-grid">
                {filteredBundleEntries.map((entry) => {
                  const tile = buildFileTileModel({
                    node: entry,
                    variant: 'bundle-entry',
                    bundleLabel: currentBundleContext.name,
                    locale,
                  })
                  return (
                    <FileMaterialsTile
                      key={entry.path}
                      node={tile.node}
                      subtitle={tile.subtitle}
                      description={tile.description}
                      extraPills={tile.source ? <span className="materials-tile-pill materials-source-pill">{sourceLabel(tile.source, locale)}</span> : undefined}
                      path={tile.path}
                      footerStart={tile.footerStart}
                      footerEnd={tile.footerEnd}
                      selected={selectedEntryPath === entry.path}
                      menuOpen={isMenuOpen(entry.path)}
                      menuButtonAriaLabel={tx(`打开 ${entry.name} 的工具菜单`, `Open tools menu for ${entry.name}`)}
                      menuPanel={(
                        <ResourceActionMenu
                          items={[
                            ...((entry.is_dir || isEditableFile(entry))
                              ? [{
                                  key: 'open',
                                  label: entry.is_dir ? tx('进入目录', 'Open folder') : tx('打开文件', 'Open file'),
                                  onSelect: () => {
                                    closeMenu()
                                    if (entry.is_dir) {
                                      openBundleFolder(entry.path)
                                    } else {
                                      openFileEditor(entry.path)
                                    }
                                  },
                                }]
                              : []),
                            {
                              key: 'download',
                              label: tx('下载 ZIP', 'Download ZIP'),
                              onSelect: () => {
                                void handleDownloadZip(entry.path)
                              },
                            },
                            {
                              key: 'select',
                              label: selectedEntryPath === entry.path ? tx('取消选中', 'Unselect') : tx('加入选择', 'Select'),
                              onSelect: () => {
                                closeMenu()
                                setSelectedEntryPath((value) => (value === entry.path ? null : entry.path))
                              },
                            },
                            ...(currentBundleContext.read_only || !canWriteCurrentScope
                              ? []
                              : [{
                                  key: 'delete',
                                  label: tx('删除', 'Delete'),
                                  tone: 'danger' as const,
                                  onSelect: () => {
                                    closeMenu()
                                    void requestDelete([entry.path])
                                  },
                                }]),
                          ]}
                        />
                      )}
                      onMenuToggle={() => toggleMenu(entry.path)}
                      onSelect={() => setSelectedEntryPath(entry.path)}
                      onOpen={entry.is_dir ? () => openBundleFolder(entry.path) : (isEditableFile(entry) ? () => openFileEditor(entry.path) : undefined)}
                    />
                  )
                })}
              </div>
            )}
          </section>
        ) : null}

        <ResourceConfirmDialog
          open={Boolean(deleteDialog)}
          kicker={tx('删除确认', 'Delete confirmation')}
          title={deleteDialog?.nonEmptyDirectories.length ? tx('这些目录不是空的', 'These folders are not empty') : tx('确认删除选中条目', 'Confirm deletion')}
          description={deleteDialog?.nonEmptyDirectories.length
            ? tx('确认后会递归删除其中所有可写文件和文件夹。只读内容不会被删除，可能会继续保留。', 'Continuing will recursively delete all writable files and folders inside. Read-only content will not be deleted and may remain in place.')
            : tx('这个操作会删除选中的技能文件或 bundle，且不可撤销。', 'This will delete the selected skill file or bundle and cannot be undone.')}
          cancelLabel={tx('取消', 'Cancel')}
          confirmLabel={deleteSubmitting ? tx('删除中...', 'Deleting...') : tx('确认删除', 'Delete')}
          tone="danger"
          submitting={deleteSubmitting}
          onCancel={closeDeleteDialog}
          onConfirm={() => void confirmDelete()}
        />
      </div>
    )
  }

  return (
    <div className="page materials-page">
      <section className="materials-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">{tx('AI 工具箱', 'AI Toolbox')}</div>
          <h2 className="materials-title">{tx('技能', 'Skills')}</h2>
          <p className="materials-subtitle">{tx('Skills 是你的 AI 工具箱，可以被 Claude、ChatGPT、Cursor 等工具复用。', 'Reusable instructions and tools for your AI agents across Claude, ChatGPT, Cursor, and more.')}</p>
        </div>
        <div className="materials-actions">
          {skillScopeControl}
          <button className="btn btn-primary" disabled={!canWriteCurrentScope} onClick={() => setShowNewForm(true)}>{tx('创建 Skill', 'Create skill')}</button>
          <button className="btn" onClick={() => navigate(activeTeamID ? `/import/skills?team=${encodeURIComponent(activeTeamID)}` : '/import/skills')}>{tx('导入 Skill', 'Import skill')}</button>
        </div>
      </section>

      {error && <div className="alert alert-warn">{error}</div>}
      {scopeMessage && <div className="alert alert-success">{scopeMessage}</div>}

      {!isBundleView && (
        <div className="tab-strip" style={{ marginTop: '8px', marginBottom: '24px' }}>
          <button
            type="button"
            className={activeTab === 'library' ? 'active' : ''}
            onClick={() => setActiveTab('library')}
          >
            {tx('技能库', 'Skills Library')}
          </button>
          <button
            type="button"
            className={activeTab === 'learning' ? 'active' : ''}
            onClick={() => setActiveTab('learning')}
          >
            {tx('学习与自进化', 'Insights & Learning')}
            {pendingGrowthProposalCount > 0 && (
              <span style={{ marginLeft: 6, display: 'inline-block', width: 8, height: 8, borderRadius: '50%', background: '#dc2626' }} />
            )}
          </button>
          <button
            type="button"
            className={activeTab === 'assignments' ? 'active' : ''}
            onClick={() => setActiveTab('assignments')}
          >
            {tx('Agent 分配与同步', 'Agent Assignments')}
          </button>
        </div>
      )}

      {!isBundleView && activeTab === 'learning' && learningSummary && (
        <section className="materials-section skill-learning-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('学习摘要', 'Learning summary')}</h3>
              <p className="materials-section-copy">{tx('根据用途说明、manifest、资产完整度、分配状态和验证风险，整理当前 Skill 库。', 'Summarizes the current skill library from metadata, manifests, assets, assignments, and validation risk.')}</p>
            </div>
            <div className="materials-actions">
              {learningSummary.latest_run?.outputs?.report_path ? (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => navigate(dataFileEditorRoute(learningSummary.latest_run?.outputs.report_path || '', activeTeamID))}
                >
                  {tx('查看报告', 'View report')}
                </button>
              ) : null}
              <button
                type="button"
                className="btn btn-sm btn-primary"
                disabled={learningRunBusy}
                onClick={() => { void createLearningRun() }}
              >
                {learningRunBusy ? tx('生成中...', 'Creating...') : tx('生成学习报告', 'Create report')}
              </button>
              <button
                type="button"
                className="btn btn-sm"
                onClick={() => navigate(activeTeamID ? `/growth-proposals?team=${encodeURIComponent(activeTeamID)}` : '/growth-proposals')}
              >
                {tx(`待审提案 ${pendingGrowthProposalCount}`, `Pending ${pendingGrowthProposalCount}`)}
              </button>
              <div className="skill-learning-score">
                <strong>{learningSummary.stats.ready}</strong>
                <span>{tx('可直接使用', 'ready')}</span>
              </div>
            </div>
          </div>
          {learningRunError && <div className="alert alert-warn">{learningRunError}</div>}
          {learningRunMessage && <div className="alert alert-success">{learningRunMessage}</div>}
          {learningSummary.latest_run ? (
            <div className={`skill-learning-run is-${learningSummary.latest_run.status}`}>
              <div>
                <strong>{learningRunStatusLabel(learningSummary.latest_run.status, tx)}</strong>
                <span>{formatDateTime(learningSummary.latest_run.finished_at || learningSummary.latest_run.started_at, locale)}</span>
              </div>
              <div>
                {learningSummary.latest_run.model
                  ? <span>{learningSummary.latest_run.model.provider_id}{learningSummary.latest_run.model.model ? ` / ${learningSummary.latest_run.model.model}` : ''}</span>
                  : <span>{tx('未配置模型，已生成结构化报告', 'No model configured, structured report created')}</span>}
                {learningSummary.latest_run.steps?.some((step) => step.error) ? (
                  <em>{tx('部分步骤有错误，报告仍已保留', 'Some steps had errors, report was still kept')}</em>
                ) : null}
              </div>
            </div>
          ) : null}
          <div className="skill-learning-stats">
            <div>
              <strong>{learningSummary.stats.skills}</strong>
              <span>{tx('Skill 总数', 'Skills')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.needs_summary}</strong>
              <span>{tx('缺用途说明', 'Need summary')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.needs_validation}</strong>
              <span>{tx('需预览验证', 'Need validation')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.rich_assets}</strong>
              <span>{tx('含脚本/依赖', 'Rich assets')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.assigned}</strong>
              <span>{tx('已分配', 'Assigned')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.quality_blocked || 0}</strong>
              <span>{tx('阻断项', 'Blocked')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.quality_manual_required || 0}</strong>
              <span>{tx('需审查', 'Review')}</span>
            </div>
            <div>
              <strong>{learningSummary.stats.quality_warnings || 0}</strong>
              <span>{tx('提醒', 'Warnings')}</span>
            </div>
          </div>
          <div className="skill-learning-actions">
            {learningSummary.actions.map((action) => (
              <div key={action.code} className="skill-learning-action">
                <strong>{action.label}</strong>
                <span>{tx(`${action.count} 项`, `${action.count}`)}</span>
                <p>{action.message}</p>
              </div>
            ))}
          </div>
          <form className="skill-learning-recommend" onSubmit={runSkillRecommendation}>
            <input
              className="input"
              value={recommendQuery}
              onChange={(event) => setRecommendQuery(event.target.value)}
              placeholder={tx('输入新需求，例如：部署 Docker、分析 PDF、生成 PPT', 'Enter a request, for example: deploy Docker, analyze PDF, create slides')}
            />
            <button className="btn btn-primary" disabled={recommendBusy || !recommendQuery.trim()}>
              {recommendBusy ? tx('查找中...', 'Searching...') : tx('找相关 Skill', 'Find skills')}
            </button>
          </form>
          {recommendError && <div className="alert alert-warn">{recommendError}</div>}
          {recommendResult && (
            <div className="skill-learning-list">
              {(recommendResult.items || []).slice(0, 5).map((item) => (
                <div key={`recommend-${item.path}`} className={`skill-learning-item is-${item.status}`}>
                  <div className="skill-learning-item-head">
                    <div>
                      <strong>{item.name}</strong>
                      <small>{item.path}</small>
                    </div>
                    <em>{tx('匹配', 'match')} {item.match_score || item.score}</em>
                  </div>
                  <div className="skill-learning-badges">
                    {(item.match_reasons || []).slice(0, 3).map((reason) => <span key={reason}>{reason}</span>)}
                    {item.assigned_agents?.length ? <span>{item.assigned_agents.join(' / ')}</span> : <span>{tx('未分配', 'unassigned')}</span>}
                    <span>{qualityStatusLabel(item.quality_status, tx)}</span>
                  </div>
                  <p>{(item.quality_findings || []).slice(0, 2).map((finding) => `${finding.title}: ${finding.message}`).join('；') || item.recommendations?.slice(0, 2).join('；') || tx('可以作为这个需求的候选 Skill。', 'Candidate skill for this request.')}</p>
                </div>
              ))}
              {recommendResult.items.length === 0 && (
                <div className="empty-action-state"><p>{tx('没找到明显匹配的 Skill。', 'No strong matching skill found.')}</p></div>
              )}
              {recommendResult.candidate_proposal ? (
                <div className="skill-learning-item is-needs_summary">
                  <div className="skill-learning-item-head">
                    <div>
                      <strong>{tx('建议新建候选 Skill', 'Suggested candidate Skill')}</strong>
                      <small>{recommendResult.candidate_proposal.target_path}</small>
                    </div>
                    <em>{tx('待审提案', 'proposal')}</em>
                  </div>
                  <p>{recommendResult.candidate_proposal.reason}</p>
                  <div className="materials-actions">
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => navigate(activeTeamID ? `/growth-proposals?team=${encodeURIComponent(activeTeamID)}&status=pending_review` : '/growth-proposals?status=pending_review')}
                    >
                      {tx('查看提案', 'Review proposal')}
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
          )}
          {learningNotes.length > 0 && (
            <div className="skill-learning-notes">
              <h4>{tx('最近学习记录', 'Recent learning notes')}</h4>
              {learningNotes.slice(0, 3).map((note) => (
                <button key={note.path} type="button" className="skill-learning-note" onClick={() => navigate(dataFileEditorRoute(note.path, activeTeamID))}>
                  <strong>{note.title || titleFromPath(note.path)}</strong>
                  <span>{formatDateTime(note.updated_at, locale)}</span>
                </button>
              ))}
            </div>
          )}
          {learningSummary.items.filter(item => item.quality_findings && item.quality_findings.length > 0).length > 0 && (
            <div className="skill-learning-reflexions" style={{ marginTop: '24px', marginBottom: '24px' }}>
              <h4 style={{ marginBottom: '12px', fontSize: '15px', color: '#27335f', fontWeight: 'bold' }}>
                {tx('质量改进反思建议', 'Quality reflexions & recommendations')}
              </h4>
              <div className="growth-proposal-list" style={{ display: 'grid', gap: '12px' }}>
                {learningSummary.items.filter(item => item.quality_findings && item.quality_findings.length > 0).slice(0, 3).map((item) => (
                  <div key={`reflexion-${item.path}`} className="materials-panel" style={{ padding: '16px', borderLeft: '4px solid #ef4444' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
                      <strong style={{ color: '#27335f', fontSize: '14px' }}>{item.name}</strong>
                      <span className="materials-tile-pill skill-score-pill poor">{tx('健康度：', 'Health: ')}{item.score}%</span>
                    </div>
                    <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '8px' }}>
                      {tx('AI 质量门岗检测到以下问题：', 'The AI quality gate detected the following issues:')}
                    </p>
                    <div style={{ background: '#fef2f2', padding: '10px', borderRadius: '8px', border: '1px solid #fee2e2' }}>
                      {item.quality_findings?.map((f, i) => (
                        <div key={i} style={{ fontSize: '12px', color: '#b91c1c', marginBottom: i < item.quality_findings!.length - 1 ? '6px' : '0' }}>
                          <strong>• {f.title}:</strong> {f.message} {f.path ? ` (${f.path})` : ''}
                        </div>
                      ))}
                    </div>
                    <div style={{ marginTop: '12px', display: 'flex', gap: '8px' }}>
                      {item.primary_path && (
                        <button
                          type="button"
                          className="btn btn-sm"
                          onClick={() => navigate(dataFileEditorRoute(item.primary_path || '', activeTeamID))}
                        >
                          {tx('前往编辑 SKILL.md', 'Edit SKILL.md')}
                        </button>
                      )}
                      <button
                        type="button"
                        className="btn btn-sm"
                        onClick={() => openBundleDetail(item.path.replace(/^\/skills\/?/, ''))}
                      >
                        {tx('查看包文件夹', 'View bundle folder')}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
          {learningSummary.items.length > 0 && (
            <div className="skill-learning-list">
              {learningSummary.items.slice(0, 8).map((item) => (
                <div key={item.path} className={`skill-learning-item is-${item.status}`}>
                  <div className="skill-learning-item-head">
                    <div>
                      <strong>{item.name}</strong>
                      <small>{item.path}</small>
                    </div>
                    <em>{learningStatusLabel(item.status, tx)} · {item.score}</em>
                  </div>
                  <div className="skill-learning-badges">
                    {item.has_manifest && <span>{tx('有 manifest', 'manifest')}</span>}
                    {item.has_scripts && <span>{tx('脚本', 'scripts')}</span>}
                    {item.has_dependencies && <span>{tx('依赖', 'deps')}</span>}
                    {item.has_external_refs && <span>{tx('外部引用', 'external refs')}</span>}
                    {item.assigned_agents?.length ? <span>{item.assigned_agents.join(' / ')}</span> : <span>{tx('未分配', 'unassigned')}</span>}
                    <span>{qualityStatusLabel(item.quality_status, tx)}</span>
                  </div>
                  {item.quality_findings?.length ? (
                    <p>{item.quality_findings.slice(0, 2).map((finding) => `${finding.title}: ${finding.message}`).join('；')}</p>
                  ) : item.recommendations?.length ? (
                    <p>{item.recommendations.slice(0, 2).join('；')}</p>
                  ) : (
                    <p>{tx('元数据和分配状态较完整。', 'Metadata and assignment state look complete.')}</p>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>
      )}

      {(isBundleView || activeTab === 'library') && (
        <GitHubTreeList
          key={activeTeamID || 'personal'}
          rootPath="/skills"
          rootLabel={tx('技能', 'Skills')}
          title={tx('技能文件', 'Skill files')}
          description={tx('按文件夹层级浏览所有技能包。', 'Browse all skill bundles by folder.')}
          actionHref="/?type=Skill"
          actionLabel={tx('在首页查看', 'View on Home')}
          loadNode={scopedGetNode}
          fileRoute={(path) => dataFileEditorRoute(path, activeTeamID)}
        />
      )}

      {!isBundleView && activeTab === 'assignments' && assignmentState && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('Agent 分配', 'Agent assignments')}</h3>
              <p className="materials-section-copy">{tx('按 Agent 选择可用 Skill。Claude Code 写入 ~/.claude/skills，Codex 写入 ~/.agents/skills；Cursor 和 Gemini CLI 只保存分配并生成导出包。', 'Choose skills per agent. Claude Code writes to ~/.claude/skills, Codex writes to ~/.agents/skills, while Cursor and Gemini CLI keep assignments and export packages only.')}</p>
            </div>
            <div className="materials-actions">
              <button className="btn btn-sm" disabled={!assignmentChanged || assignmentSaving} onClick={resetSkillAssignments}>
                {tx('还原', 'Reset')}
              </button>
              <button className="btn btn-sm btn-primary" disabled={!canWriteCurrentScope || !assignmentChanged || assignmentSaving} onClick={() => { void saveSkillAssignments() }}>
                {assignmentSaving ? tx('保存中...', 'Saving...') : tx('保存分配', 'Save')}
              </button>
            </div>
          </div>

          {assignmentError && <div className="alert alert-warn">{assignmentError}</div>}
          {assignmentMessage && <div className="alert alert-success">{assignmentMessage}</div>}

          <div className="skill-assignment-grid">
            {assignmentState.agents.map((agent) => {
              const saved = savedAssignmentLookup[agent.id] || new Set<string>()
              const draft = draftAssignmentLookup[agent.id] || new Set<string>()
              const diff = diffSkillPaths(saved, draft)
              const selectedCount = draft.size
              const statusLabel = agent.supports_apply
                ? tx('可自动同步', 'Auto sync')
                : agent.export_supported
                  ? tx('可分配、可导出、暂不自动写入', 'Assignable, exportable, no auto-write')
                  : tx('可分配', 'Assignable')
              return (
                <div key={agent.id} className="materials-panel skill-assignment-card">
                  <div className="skill-assignment-card-head">
                    <div>
                      <strong>{agent.name}</strong>
                      <span>{agent.install_path_hint || agent.platform}</span>
                    </div>
                    <em>{tx(`${selectedCount} 个`, `${selectedCount}`)}</em>
                  </div>
                  <div className="skill-assignment-diff">
                    <span>{statusLabel}</span>
                    {agent.auto_apply_reason ? <span>{agent.auto_apply_reason}</span> : null}
                  </div>
                  <div className="skill-assignment-options">
                    {sortedSkills.length === 0 ? (
                      <p className="dashboard-empty-copy">{tx('暂无可分配 Skill。', 'No skills available.')}</p>
                    ) : (
                      sortedSkills.map((skill) => {
                        const skillPath = normalizeSkillPath(skill.bundlePath || skill.path)
                        return (
                          <label key={`${agent.id}-${skillPath}`} className="skill-assignment-option">
                            <input
                              type="checkbox"
                              disabled={!canWriteCurrentScope}
                              checked={draft.has(skillPath)}
                              onChange={() => toggleSkillAssignment(agent.id, skillPath)}
                            />
                            <span>
                              <strong>{skill.name}</strong>
                              <small>{skillPath}</small>
                            </span>
                          </label>
                        )
                      })
                    )}
                  </div>
                  {(diff.added.length > 0 || diff.removed.length > 0) && (
                    <div className="skill-assignment-diff">
                      {diff.added.length > 0 && (
                        <span>{tx(`新增 ${diff.added.map((item) => skillNameByPath[item] || item).join('、')}`, `Add ${diff.added.map((item) => skillNameByPath[item] || item).join(', ')}`)}</span>
                      )}
                      {diff.removed.length > 0 && (
                        <span>{tx(`移除 ${diff.removed.map((item) => skillNameByPath[item] || item).join('、')}`, `Remove ${diff.removed.map((item) => skillNameByPath[item] || item).join(', ')}`)}</span>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>

          {localMode && (
            <div className="materials-panel skill-local-sync-panel">
              <div className="materials-section-head">
                <div>
                  <h3 className="materials-section-title">{tx('本地同步', 'Local sync')}</h3>
                  <p className="materials-section-copy">{tx('预览会列出新增、更新、冲突和导出项。只有 Claude Code 与 Codex 会写本地目录，且只更新由 Vola 管理的 Skill；Cursor / Gemini CLI 只导出包。', 'Preview lists additions, updates, conflicts, and export items. Only Claude Code and Codex write local folders, and only Vola-managed skills are updated; Cursor / Gemini CLI export packages only.')}</p>
                </div>
                <div className="materials-actions">
                  <button
                    className="btn btn-sm"
                    disabled={assignmentChanged || Boolean(skillSyncBusy)}
                    onClick={() => { void runLocalSkillSync('preview') }}
                  >
                    {skillSyncBusy === 'preview' ? tx('预览中...', 'Previewing...') : tx('预览同步', 'Preview')}
                  </button>
                  <button
                    className="btn btn-sm btn-primary"
                    disabled={assignmentChanged || Boolean(skillSyncBusy)}
                    onClick={() => { void runLocalSkillSync('apply') }}
                  >
                    {skillSyncBusy === 'apply' ? tx('应用中...', 'Applying...') : tx('应用 Claude/Codex', 'Apply Claude/Codex')}
                  </button>
                  <button
                    className="btn btn-sm"
                    disabled={assignmentChanged || Boolean(skillSyncBusy)}
                    onClick={() => { void runLocalSkillSync('cleanup') }}
                  >
                    {skillSyncBusy === 'cleanup' ? tx('清理中...', 'Cleaning...') : tx('清理已取消分配', 'Clean unassigned')}
                  </button>
                </div>
              </div>

              {assignmentChanged && (
                <div className="alert alert-warn">{tx('分配有未保存修改，保存后再同步到本地。', 'Save assignment changes before syncing locally.')}</div>
              )}
              {skillSyncError && <div className="alert alert-warn">{skillSyncError}</div>}
              {skillSyncMessage && <div className="alert alert-success">{skillSyncMessage}</div>}

              {skillSyncPreview && (
                <div className="data-sync-preview skill-local-sync-preview">
                  <div className="data-sync-preview-sections">
                    {skillSyncPreview.agents.map((agent) => (
                      <details key={agent.agent_id} className="data-sync-preview-section" open={agent.summary.conflict > 0 || agent.summary.add > 0 || agent.summary.update > 0 || agent.summary.removable > 0 || agent.summary.export > 0}>
                        <summary className="data-sync-preview-summary">
                          <span>
                            <strong>{agent.name}</strong>
                            <small>{agent.target_root || agent.export_file_name || agent.support_status}</small>
                          </span>
                          <span>{agent.supported ? syncAgentSummaryText(agent, tx) : tx(`可导出 ${agent.summary.export} 个`, `${agent.summary.export} export`)}</span>
                        </summary>
                        {agent.message && <p className="dashboard-empty-copy">{agent.message}</p>}
                        {agent.auto_apply_reason && <p className="dashboard-empty-copy">{agent.auto_apply_reason}</p>}
                        {agent.directory_rules?.length ? (
                          <div className="data-sync-preview-list">
                            {agent.directory_rules.map((rule, index) => (
                              <div key={`${agent.agent_id}-rule-${index}`} className="data-sync-preview-entry">
                                <span className="preview-action preview-action-skip">{tx('规则', 'rule')}</span>
                                <span className="skill-conversion-report-text">{rule}</span>
                              </div>
                            ))}
                          </div>
                        ) : null}
                        {agent.detected_roots?.length ? (
                          <div className="data-sync-preview-list">
                            {agent.detected_roots.map((root, index) => (
                              <div key={`${agent.agent_id}-root-${index}`} className="data-sync-preview-entry">
                                <span className={root.exists ? 'preview-action preview-action-update' : 'preview-action preview-action-skip'}>{root.exists ? tx('已发现', 'found') : tx('未发现', 'missing')}</span>
                                <span className="skill-local-sync-path">{root.path}</span>
                                <small>{root.role || root.message || ''}{root.exists ? ` · ${root.is_dir ? tx('目录', 'directory') : tx('文件', 'file')}` : root.message ? ` · ${root.message}` : ''}</small>
                              </div>
                            ))}
                          </div>
                        ) : null}
                        {agent.errors?.length ? (
                          <div className="alert alert-warn">
                            {agent.errors.join('；')}
                          </div>
                        ) : null}
                        {agent.summary.conflict > 0 ? (
                          <div className="alert alert-warn">
                            {tx('有同名本地目录或文件冲突。应用时 Vola 不会覆盖未由它管理的内容。', 'Conflicts detected. Vola will not overwrite content it does not manage.')}
                          </div>
                        ) : null}
                        {(agent.summary.sync_risk || 0) > 0 ? (
                          <div className="alert alert-warn">
                            {tx(
                              `有 ${agent.summary.sync_risk} 个 Skill 未通过质量门槛，其中阻断 ${agent.summary.blocked || 0}，需审查 ${agent.summary.manual_required || 0}。`,
                              `${agent.summary.sync_risk} skills need quality review: ${agent.summary.blocked || 0} blocked, ${agent.summary.manual_required || 0} manual review.`,
                            )}
                          </div>
                        ) : null}
                        {agent.changes.length === 0 ? (
                          <p className="dashboard-empty-copy">{tx('没有需要处理的本地变化。', 'No local changes to process.')}</p>
                        ) : (
                          <div className="data-sync-preview-list">
                            {agent.changes.filter((change) => change.action !== 'marker').slice(0, 24).map((change, index) => (
                              <div key={`${agent.agent_id}-${change.target_path}-${index}`} className={`data-sync-preview-entry ${change.action === 'conflict' ? 'is-danger' : ''}`}>
                                <span className={syncActionClass(change.action)}>{syncActionLabel(change.action, tx)}</span>
                                <span className="skill-local-sync-path">{change.skill_path}{change.rel_path ? `/${change.rel_path}` : ''}</span>
                                {change.verification_required ? <small>{change.verification_message || tx(`需验证：${change.verification_status || 'required'}`, `verification: ${change.verification_status || 'required'}`)}</small> : null}
                                {change.reason ? <small>{change.reason}</small> : null}
                              </div>
                            ))}
                            {agent.changes.filter((change) => change.action !== 'marker').length > 24 && (
                              <p className="dashboard-empty-copy">{tx('只显示前 24 条变化。', 'Showing the first 24 changes only.')}</p>
                            )}
                          </div>
                        )}
                        {agent.export_available && (
                          <div className="materials-actions">
                            <button
                              className="btn btn-sm"
                              disabled={assignmentChanged || Boolean(skillExportBusy)}
                              onClick={() => { void downloadLocalSkillExport(agent.agent_id) }}
                            >
                              {skillExportBusy === agent.agent_id ? tx('生成中...', 'Creating...') : tx('下载导出包', 'Download export')}
                            </button>
                          </div>
                        )}
                      </details>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </section>
      )}

      {skills.length > 0 && (
        <section className="materials-section">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('Skill 转换', 'Skill conversion')}</h3>
              <p className="materials-section-copy">{tx('生成 Claude Code / Codex 互通副本，并附带脚本、依赖、外部引用和 MCP 配置检查报告。', 'Create a Claude Code / Codex compatible copy with a report for scripts, dependencies, external references, and MCP config.')}</p>
            </div>
            <div className="materials-actions">
              <button
                className="btn btn-sm"
                disabled={!conversionSourcePath || conversionSourcePlatform === conversionTargetPlatform || Boolean(conversionBusy)}
                onClick={() => { void runSkillConversion('preview') }}
              >
                {conversionBusy === 'preview' ? tx('预览中...', 'Previewing...') : tx('预览转换', 'Preview')}
              </button>
              <button
                className="btn btn-sm btn-primary"
                disabled={!canWriteCurrentScope || !conversionSourcePath || conversionSourcePlatform === conversionTargetPlatform || Boolean(conversionBusy)}
                onClick={() => { void runSkillConversion('apply') }}
              >
                {conversionBusy === 'apply' ? tx('生成中...', 'Creating...') : tx('生成转换副本', 'Create copy')}
              </button>
            </div>
          </div>

          <div className="materials-panel skill-conversion-panel">
            <div className="skill-conversion-grid">
              <div className="form-group">
                <label htmlFor="skill-conversion-source">{tx('源 Skill', 'Source skill')}</label>
                <CustomSelect
                  value={conversionSourcePath}
                  onChange={(val) => {
                    const next = val
                    setConversionSourcePath(next)
                    setConversionTargetPath(defaultConversionTargetPath(next, conversionTargetPlatform))
                    setConversionPreview(null)
                    setConversionMessage('')
                    setConversionError('')
                  }}
                  options={sortedSkills.map((skill) => {
                    const skillPath = normalizeSkillPath(skill.bundlePath || skill.path)
                    return { value: skillPath, label: skill.name }
                  })}
                  ariaLabel={tx('源 Skill', 'Source skill')}
                />
              </div>
              <div className="form-group">
                <label htmlFor="skill-conversion-source-platform">{tx('源平台', 'Source platform')}</label>
                <CustomSelect
                  value={conversionSourcePlatform}
                  onChange={(val) => {
                    const next = val as 'claude-code' | 'codex'
                    setConversionSourcePlatform(next)
                    setConversionTargetPlatform(next === 'claude-code' ? 'codex' : 'claude-code')
                    setConversionTargetPath(defaultConversionTargetPath(conversionSourcePath, next === 'claude-code' ? 'codex' : 'claude-code'))
                    setConversionPreview(null)
                  }}
                  options={[
                    { value: 'claude-code', label: 'Claude Code' },
                    { value: 'codex', label: 'Codex' },
                  ]}
                  ariaLabel={tx('源平台', 'Source platform')}
                />
              </div>
              <div className="form-group">
                <label htmlFor="skill-conversion-target-platform">{tx('目标平台', 'Target platform')}</label>
                <CustomSelect
                  value={conversionTargetPlatform}
                  onChange={(val) => {
                    const next = val as 'claude-code' | 'codex'
                    setConversionTargetPlatform(next)
                    if (next === conversionSourcePlatform) {
                      setConversionSourcePlatform(next === 'claude-code' ? 'codex' : 'claude-code')
                    }
                    setConversionTargetPath(defaultConversionTargetPath(conversionSourcePath, next))
                    setConversionPreview(null)
                  }}
                  options={[
                    { value: 'codex', label: 'Codex' },
                    { value: 'claude-code', label: 'Claude Code' },
                  ]}
                  ariaLabel={tx('目标平台', 'Target platform')}
                />
              </div>
              <div className="form-group skill-conversion-grid-wide">
                <label htmlFor="skill-conversion-target-path">{tx('目标路径', 'Target path')}</label>
                <input
                  id="skill-conversion-target-path"
                  type="text"
                  value={conversionTargetPath}
                  onChange={(event) => {
                    setConversionTargetPath(event.target.value)
                    setConversionPreview(null)
                  }}
                  placeholder="/skills/example-codex"
                />
              </div>
              <label className="skill-conversion-checkbox">
                <input
                  type="checkbox"
                  checked={conversionOverwrite}
                  onChange={(event) => {
                    setConversionOverwrite(event.target.checked)
                    setConversionPreview(null)
                  }}
                />
                <span>{tx('允许覆盖目标路径已有文件', 'Allow overwriting existing files at target path')}</span>
              </label>
            </div>

            {conversionSourcePlatform === conversionTargetPlatform && (
              <div className="alert alert-warn">{tx('源平台和目标平台不能相同。', 'Source and target platform must be different.')}</div>
            )}
            {conversionError && <div className="alert alert-warn">{conversionError}</div>}
            {conversionMessage && <div className="alert alert-success">{conversionMessage}</div>}

            {conversionPreview && (
              <div className="data-sync-preview skill-conversion-preview">
                <div className="data-sync-preview-sections">
                  <details className="data-sync-preview-section" open>
                    <summary className="data-sync-preview-summary">
                      <span>
                        <strong>{conversionPreview.source_platform} → {conversionPreview.target_platform}</strong>
                        <small>{conversionPreview.source_path} → {conversionPreview.target_path}</small>
                      </span>
                      <span>{tx(
                        `转换 ${conversionPreview.summary.converted} / 复制 ${conversionPreview.summary.copied} / 冲突 ${conversionPreview.summary.conflicts}`,
                        `convert ${conversionPreview.summary.converted} / copy ${conversionPreview.summary.copied} / conflicts ${conversionPreview.summary.conflicts}`,
                      )}</span>
                    </summary>
                    <div className="data-sync-preview-list">
                      {conversionPreview.files.slice(0, 28).map((file) => (
                        <div key={`${file.target_path}-${file.action}`} className={`data-sync-preview-entry ${file.action === 'conflict' ? 'is-danger' : ''}`}>
                          <span className={conversionActionClass(file.action)}>{conversionActionLabel(file.action, tx)}</span>
                          <span className="skill-local-sync-path">{file.rel_path}</span>
                          {file.reason ? <small>{file.reason}</small> : null}
                        </div>
                      ))}
                      {conversionPreview.files.length > 28 && (
                        <p className="dashboard-empty-copy">{tx('只显示前 28 个文件。', 'Showing the first 28 files only.')}</p>
                      )}
                    </div>
                  </details>

                  {[
                    { title: tx('可自动处理', 'Automatic'), items: conversionPreview.auto_items || [] },
                    { title: tx('需要处理', 'Needs review'), items: conversionPreview.manual_items || [] },
                    { title: tx('暂不自动转换', 'Not auto-converted'), items: conversionPreview.unsupported || [] },
                    { title: tx('提示', 'Notes'), items: conversionPreview.warnings || [] },
                  ].filter((section) => section.items.length > 0).map((section) => (
                    <details key={section.title} className="data-sync-preview-section" open>
                      <summary className="data-sync-preview-summary">
                        <strong>{section.title}</strong>
                        <span>{section.items.length}</span>
                      </summary>
                      <div className="data-sync-preview-list">
                        {section.items.slice(0, 12).map((item, index) => (
                          <div key={`${section.title}-${item.code}-${index}`} className="data-sync-preview-entry">
                            <span className="preview-action preview-action-skip">{item.code}</span>
                            <span className="skill-conversion-report-text">{item.path ? `${item.path}: ` : ''}{item.message}</span>
                          </div>
                        ))}
                      </div>
                    </details>
                  ))}
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {showNewForm && (
        <div className="materials-panel form-card">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('新建技能', 'New skill')}</h3>
              <p className="materials-section-copy">{tx('创建一个新的技能包，并直接进入 ', 'Create a new skill bundle and jump straight into the ')}<code>SKILL.md</code>{tx(' 编辑器。', ' editor.')}</p>
            </div>
          </div>
          <form onSubmit={handleCreateSkill}>
            <div className="form-group">
              <label htmlFor="skill-name">{tx('技能名称', 'Skill name')}</label>
              <input
                id="skill-name"
                type="text"
                value={newBundleName}
                onChange={(event) => setNewBundleName(event.target.value)}
                placeholder={tx('例如：meeting-notes', 'For example: meeting-notes')}
                disabled={creating || !canWriteCurrentScope}
                autoFocus
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={creating || !canWriteCurrentScope}>
                {creating ? tx('创建中...', 'Creating...') : tx('创建', 'Create')}
              </button>
              <button type="button" className="btn" onClick={() => setShowNewForm(false)} disabled={creating}>
                {tx('取消', 'Cancel')}
              </button>
            </div>
          </form>
        </div>
      )}

      <section className="materials-section">
        <div className="materials-section-head">
          <div>
            <h3 className="materials-section-title">{tx('技能包', 'Skill Bundles')}</h3>
            <p className="materials-section-copy">{tx('统一按时间或名称浏览技能包卡片。', 'Browse skill bundle cards by time or name.')}</p>
          </div>
          <MaterialsSectionToolbar
            count={filteredSkills.length}
            sortKey={sortKey}
            sortOptions={sortOptions}
            sortDir={sortDir}
            onSortKeyChange={(value) => setSortKey(value as MaterialsSortKey)}
            onSortDirToggle={() => setSortDir((value) => (value === 'desc' ? 'asc' : 'desc'))}
          >
            <button className="btn btn-sm materials-toolbar-control" onClick={() => setShowNewForm((value) => !value)}>
              {showNewForm ? tx('取消新建', 'Close form') : tx('新建技能', 'New skill')}
            </button>
            <button
              className="btn btn-sm materials-toolbar-control is-danger"
              disabled={!canDeleteSelection}
              onClick={() => {
                if (selectedDeletePath) void requestDelete([selectedDeletePath])
              }}
            >
              {tx('删除', 'Delete')}
            </button>
          </MaterialsSectionToolbar>
        </div>

        {(skillSourceOptions.length > 1 || sourceFilter !== 'all') && (
          <SourceFilterBar options={skillSourceOptions} value={sourceFilter} onChange={setSourceFilter} />
        )}

        {activeTeamID ? (
          <div className="team-skill-share-panel">
            <div>
              <strong>{tx('团队共享 Skill', 'Shared team skills')}</strong>
              <p>{tx('成员上传到团队空间的 Skill 会在这里汇总；Vola 会提示哪些还没安装到个人空间、哪些有团队新版，也会保留发布状态。', 'Skills uploaded to this team appear here. Vola shows what is not installed personally, what has a newer team version, and the publication state.')}</p>
            </div>
            <div className="team-skill-share-stats">
              <span>{tx(`已发布 ${teamPublicationSummary.published}`, `${teamPublicationSummary.published} published`)}</span>
              <span>{tx(`草稿 ${teamPublicationSummary.draft}`, `${teamPublicationSummary.draft} drafts`)}</span>
              <span>{tx(`待安装 ${teamInstallSummary.not_installed}`, `${teamInstallSummary.not_installed} to install`)}</span>
              <span>{tx(`可更新 ${teamInstallSummary.update_available}`, `${teamInstallSummary.update_available} updates`)}</span>
              <span>{tx(`已安装 ${teamInstallSummary.installed + teamInstallSummary.personal_newer}`, `${teamInstallSummary.installed + teamInstallSummary.personal_newer} installed`)}</span>
              {teamInstallSummary.update_available > 0 ? (
                <button
                  type="button"
                  className="btn btn-sm btn-primary"
                  disabled={bulkUpdatingTeamSkills}
                  onClick={() => { void updateAllTeamSkillCopies() }}
                >
                  {bulkUpdatingTeamSkills ? tx('更新中...', 'Updating...') : tx('更新全部个人副本', 'Update all copies')}
                </button>
              ) : null}
            </div>
          </div>
        ) : null}

        {filteredSkills.length === 0 ? (
          <div className="empty-state">
            <p>{tx('还没有技能内容', 'No skills yet')}</p>
            <p className="empty-hint">{tx('导入或创建技能包后，会在这里看到对应文件夹。', 'Imported or newly created skill bundles will appear here.')}</p>
          </div>
        ) : (
          <div className="materials-grid">
            {filteredSkills.map((skill) => {
              const bundlePath = normalizeHubPath(skill.bundlePath)
              const learningItem = learningSummary?.items?.find(
                (item) => normalizeHubPath(item.path) === bundlePath
              )
              const tile = buildSkillBundleTileModel(skill, locale, learningItem)
              const publication = activeTeamID
                ? teamSkillPublicationByPath[normalizeSkillPath(skill.bundlePath || skill.path)] || defaultTeamSkillPublication(skill)
                : null
              const installStatus = activeTeamID ? teamSkillInstallStatus(skill, personalSkillCopies, teamSkillSubscriptionByPath) : null
              const installActionLabel = installStatus ? teamSkillInstallActionLabel(installStatus, tx) : ''
              const installActionBusy = copyingSkillPath === normalizeSkillPath(skill.bundlePath)
              const installActionOverwrite = installStatus === 'update_available'

              const statusPill = tile.learningStatus ? (
                <span className={`materials-tile-pill skill-status-${tile.learningStatus}`}>
                  {learningStatusLabel(tile.learningStatus, tx)}
                </span>
              ) : null

              const scoreClass = tile.learningScore !== undefined
                ? (tile.learningScore >= 80 ? 'good' : tile.learningScore >= 60 ? 'medium' : 'poor')
                : ''
              const scorePill = tile.learningScore !== undefined ? (
                <span className={`materials-tile-pill skill-score-pill ${scoreClass}`}>
                  {tx('健康度: ', 'Health: ')}{tile.learningScore}%
                </span>
              ) : null

              const agentPills = tile.assignedAgents?.map((agentId) => (
                <span key={agentId} className="materials-tile-pill skill-agents-pill">
                  {agentId}
                </span>
              ))

              const teamInstallPill = installStatus ? (
                <span className={`materials-tile-pill team-skill-install-pill is-${installStatus.replace('_', '-')}`}>
                  {teamSkillInstallLabel(installStatus, tx)}
                </span>
              ) : null
              const teamPublicationPill = publication ? (
                <span className={`materials-tile-pill team-skill-publication-pill is-${publication.status}`}>
                  {teamSkillPublicationLabel(publication, tx)}
                </span>
              ) : null
              const reviewLabel = publication ? teamSkillReviewLabel(publication, tx) : ''
              const teamReviewPill = publication && reviewLabel ? (
                <span className={`materials-tile-pill team-skill-review-pill is-${publication.review_status || 'none'}`}>
                  {reviewLabel}
                </span>
              ) : null

              const customExtraPills = (
                <>
                  {tile.source ? <span className="materials-tile-pill materials-source-pill">{sourceLabel(tile.source, locale)}</span> : null}
                  {teamPublicationPill}
                  {teamReviewPill}
                  {teamInstallPill}
                  {statusPill}
                  {scorePill}
                  {agentPills}
                </>
              )

              const qualityClass = tile.qualityStatus
                ? `skill-quality-${tile.qualityStatus}`
                : ''
              const customSubtitle = installStatus || tile.qualityStatus ? (
                <div className="skill-tile-subtitle">
                  {installStatus ? <span>{teamSkillInstallHint(installStatus, tx)}</span> : null}
                  {tile.qualityStatus ? (
                    <span className={qualityClass}>
                      {tx('质量评级: ', 'Quality: ')}{qualityStatusLabel(tile.qualityStatus, tx)}
                    </span>
                  ) : null}
                </div>
              ) : tile.subtitle

              const customDescription = (
                <div>
                  <div style={{ wordBreak: 'break-all' }}>{tile.description}</div>
                  {tile.qualityFindings && tile.qualityFindings.length > 0 && (
                    <div className="skill-tile-warnings">
                      ⚠️ {tile.qualityFindings[0].title}: {tile.qualityFindings[0].message}
                      {tile.qualityFindings.length > 1 && ` (+${tile.qualityFindings.length - 1} ${tx('更多问题', 'more issues')})`}
                    </div>
                  )}
                </div>
              )

              return (
                <FileMaterialsTile
                  key={skill.path}
                  node={tile.node}
                  subtitle={customSubtitle}
                  description={customDescription}
                  extraPills={customExtraPills}
                  actions={installActionLabel ? (
                    <button
                      type="button"
                      className="btn btn-sm btn-primary"
                      disabled={Boolean(copyingSkillPath)}
                      onClick={() => { void handleCopySkillAction(skill.bundlePath, installActionOverwrite) }}
                    >
                      {installActionBusy ? tx('处理中...', 'Working...') : installActionLabel}
                    </button>
                  ) : undefined}
                  path={tile.path}
                  footerStart={tile.footerStart}
                  footerEnd={tile.footerEnd}
                  selected={selectedBundlePath === tile.node.path}
                  menuOpen={isMenuOpen(skill.bundlePath)}
                  menuButtonAriaLabel={tx(`打开 ${skill.name} 的工具菜单`, `Open tools menu for ${skill.name}`)}
                  menuPanel={(
                    <ResourceActionMenu
                      items={[
                        {
                          key: 'open',
                          label: tx('进入 bundle', 'Open bundle'),
                          onSelect: () => {
                            closeMenu()
                            openBundleDetail(skill.bundleId)
                          },
                        },
                        {
                          key: 'download',
                          label: tx('下载 ZIP', 'Download ZIP'),
                          onSelect: () => {
                            void handleDownloadZip(skill.bundlePath)
                          },
                        },
                        ...(activeTeamID
                          ? [{
                              key: 'copy-to-personal',
                              label: installActionLabel || teamSkillInstallLabel(installStatus || 'installed', tx),
                              disabled: Boolean(copyingSkillPath) || !installActionLabel,
                              onSelect: () => {
                                closeMenu()
                                if (installActionLabel) void handleCopySkillAction(skill.bundlePath, installActionOverwrite)
                              },
                            }]
                          : []),
                        ...(installStatus !== 'not_installed'
                          ? [{
                              key: 'rollback-version',
                              label: tx('历史版本回滚...', 'Rollback to backup...'),
                              onSelect: () => {
                                closeMenu()
                                void handleOpenRollback(normalizeSkillPath(skill.bundlePath))
                              },
                            }]
                          : []),
                        ...(activeTeamID && selectedTeam?.can_manage_members
                          ? [
                              ...(publication?.review_status === 'requested'
                                ? [
                                    {
                                      key: 'approve-review',
                                      label: teamSkillPublishingPath === normalizeSkillPath(skill.bundlePath) ? tx('处理中...', 'Working...') : tx('通过审查', 'Approve review'),
                                      disabled: Boolean(teamSkillPublishingPath),
                                      onSelect: () => {
                                        closeMenu()
                                        void resolveTeamSkillReview(skill.bundlePath, 'approved')
                                      },
                                    },
                                    {
                                      key: 'request-changes',
                                      label: tx('要求修改', 'Request changes'),
                                      disabled: Boolean(teamSkillPublishingPath),
                                      onSelect: () => {
                                        closeMenu()
                                        void resolveTeamSkillReview(skill.bundlePath, 'changes_requested')
                                      },
                                    },
                                  ]
                                : []),
                              ...(publication?.status === 'published' && publication.visibility === 'team'
                                ? [{
                                    key: 'draft-publication',
                                    label: teamSkillPublishingPath === normalizeSkillPath(skill.bundlePath) ? tx('处理中...', 'Working...') : tx('转为草稿', 'Move to draft'),
                                    disabled: Boolean(teamSkillPublishingPath),
                                    onSelect: () => {
                                      closeMenu()
                                      void updateTeamSkillPublication(skill.bundlePath, 'draft')
                                    },
                                  }]
                                : [{
                                    key: 'publish-skill',
                                    label: teamSkillPublishingPath === normalizeSkillPath(skill.bundlePath) ? tx('处理中...', 'Working...') : tx('发布给团队', 'Publish to team'),
                                    disabled: Boolean(teamSkillPublishingPath),
                                    onSelect: () => {
                                      closeMenu()
                                      void updateTeamSkillPublication(skill.bundlePath, 'published')
                                    },
                                  }]),
                              {
                                key: 'archive-publication',
                                label: tx('归档团队共享', 'Archive team share'),
                                tone: 'danger' as const,
                                disabled: Boolean(teamSkillPublishingPath) || publication?.status === 'archived',
                                onSelect: () => {
                                  closeMenu()
                                  void updateTeamSkillPublication(skill.bundlePath, 'archived')
                                },
                              },
                            ]
                          : []),
                        ...(activeTeamID && selectedTeam?.can_write && !selectedTeam?.can_manage_members && publication?.status !== 'published'
                          ? [{
                              key: 'request-review',
                              label: teamSkillPublishingPath === normalizeSkillPath(skill.bundlePath) ? tx('处理中...', 'Working...') : tx('提交审查', 'Request review'),
                              disabled: Boolean(teamSkillPublishingPath) || publication?.review_status === 'requested',
                              onSelect: () => {
                                closeMenu()
                                void requestTeamSkillReview(skill.bundlePath)
                              },
                            }]
                          : []),
                        {
                          key: 'select',
                          label: selectedBundlePath === tile.node.path ? tx('取消选中', 'Unselect') : tx('加入选择', 'Select'),
                          onSelect: () => {
                            closeMenu()
                            setSelectedBundlePath((value) => (value === tile.node.path ? null : tile.node.path))
                          },
                        },
                        ...(!skill.read_only && canWriteCurrentScope
                          ? [{
                              key: 'delete',
                              label: tx('删除', 'Delete'),
                              tone: 'danger' as const,
                              onSelect: () => {
                                closeMenu()
                                void requestDelete([skill.bundlePath])
                              },
                            }]
                          : []),
                      ]}
                    />
                  )}
                  onMenuToggle={() => toggleMenu(skill.bundlePath)}
                  onSelect={() => setSelectedBundlePath(tile.node.path)}
                  onOpen={() => openBundleDetail(skill.bundleId)}
                />
              )
            })}
          </div>
        )}
      </section>

      <ResourceConfirmDialog
        open={Boolean(deleteDialog)}
        kicker={tx('删除确认', 'Delete confirmation')}
        title={deleteDialog?.nonEmptyDirectories.length ? tx('这些目录不是空的', 'These folders are not empty') : tx('确认删除选中条目', 'Confirm deletion')}
        description={deleteDialog?.nonEmptyDirectories.length
          ? tx('确认后会递归删除其中所有可写文件和文件夹。只读内容不会被删除，可能会继续保留。', 'Continuing will recursively delete all writable files and folders inside. Read-only content will not be deleted and may remain in place.')
          : tx('这个操作会删除选中的技能文件或 bundle，且不可撤销。', 'This will delete the selected skill file or bundle and cannot be undone.')}
        cancelLabel={tx('取消', 'Cancel')}
        confirmLabel={deleteSubmitting ? tx('删除中...', 'Deleting...') : tx('确认删除', 'Delete')}
        tone="danger"
        submitting={deleteSubmitting}
        onCancel={closeDeleteDialog}
        onConfirm={() => void confirmDelete()}
      />

      {/* Diff Preview Modal */}
      {showDiffModal && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)',
          backdropFilter: 'blur(8px)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 1000,
        }}>
          <div className="card" style={{
            width: '80%',
            maxWidth: '900px',
            maxHeight: '80vh',
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'var(--weui-BG-2, #fff)',
            borderRadius: '12px',
            overflow: 'hidden',
            boxShadow: '0 8px 32px rgba(0,0,0,0.25)',
            border: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
          }}>
            <div className="card-header" style={{
              padding: '16px 24px',
              borderBottom: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}>
              <h4 style={{ margin: 0, fontWeight: 'bold' }}>{tx('版本更新差异对比', 'Version Update Diff')}</h4>
              <button className="btn btn-sm btn-icon" onClick={() => setShowDiffModal(false)}>✕</button>
            </div>
            
            <div className="card-body" style={{
              padding: '24px',
              overflowY: 'auto',
              flex: 1,
            }}>
              <p style={{ marginBottom: '16px', color: 'var(--weui-FG-1, rgba(0,0,0,0.6))' }}>
                {tx('请在覆盖您的本地个人副本前，仔细查看以下与团队最新版本的代码差异：', 'Please carefully inspect the code differences before overwriting your local copy:')}
              </p>
              
              <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
                {diffFiles.map((file) => {
                  return (
                    <div key={file.rel_path} style={{
                      border: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
                      borderRadius: '8px',
                      overflow: 'hidden',
                    }}>
                      <div style={{
                        padding: '8px 16px',
                        backgroundColor: 'var(--weui-BG-1, #f7f7f7)',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        fontSize: '14px',
                      }}>
                        <strong style={{ fontFamily: 'monospace' }}>{file.rel_path}</strong>
                        <span className={`pill ${
                          file.status === 'added' ? 'is-success' :
                          file.status === 'modified' ? 'is-warning' :
                          file.status === 'deleted' ? 'is-danger' : ''
                        }`} style={{ fontSize: '12px' }}>
                          {file.status === 'added' ? tx('新增', 'Added') :
                           file.status === 'modified' ? tx('修改', 'Modified') :
                           file.status === 'deleted' ? tx('删除', 'Deleted') : tx('未变', 'Unchanged')}
                        </span>
                      </div>
                      
                      {file.status === 'modified' && (
                        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px', padding: '12px', fontSize: '12px', fontFamily: 'monospace', overflowX: 'auto' }}>
                          <div style={{ padding: '8px', backgroundColor: 'rgba(250,81,81,0.05)', border: '1px solid rgba(250,81,81,0.2)', borderRadius: '4px' }}>
                            <div style={{ color: 'var(--weui-RED, #fa5151)', fontWeight: 'bold', marginBottom: '4px' }}>{tx('您的旧副本', 'Your Copy')}</div>
                            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{file.target_content}</pre>
                          </div>
                          <div style={{ padding: '8px', backgroundColor: 'rgba(7,193,96,0.05)', border: '1px solid rgba(7,193,96,0.2)', borderRadius: '4px' }}>
                            <div style={{ color: 'var(--weui-BRAND, #07c160)', fontWeight: 'bold', marginBottom: '4px' }}>{tx('团队最新版', 'Team Version')}</div>
                            <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{file.source_content}</pre>
                          </div>
                        </div>
                      )}

                      {file.status === 'added' && (
                        <div style={{ padding: '12px', fontSize: '12px', fontFamily: 'monospace', backgroundColor: 'rgba(7,193,96,0.05)', borderTop: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))', overflowX: 'auto' }}>
                          <pre style={{ margin: 0, whiteSpace: 'pre-wrap', color: 'var(--weui-BRAND, #07c160)' }}>{file.source_content}</pre>
                        </div>
                      )}

                      {file.status === 'deleted' && (
                        <div style={{ padding: '12px', fontSize: '12px', fontFamily: 'monospace', backgroundColor: 'rgba(250,81,81,0.05)', borderTop: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))', overflowX: 'auto' }}>
                          <pre style={{ margin: 0, whiteSpace: 'pre-wrap', color: 'var(--weui-RED, #fa5151)' }}>{file.target_content}</pre>
                        </div>
                      )}
                    </div>
                  )
                })}
                {diffFiles.length === 0 && (
                  <div style={{ textAlign: 'center', padding: '32px', color: 'var(--weui-FG-1, rgba(0,0,0,0.6))' }}>
                    {tx('两个版本完全一致，无需更新。', 'Both versions are identical. No update needed.')}
                  </div>
                )}
              </div>
            </div>
            
            <div className="card-footer" style={{
              padding: '16px 24px',
              borderTop: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
              display: 'flex',
              justifyContent: 'flex-end',
              gap: '12px',
            }}>
              <button className="btn" onClick={() => setShowDiffModal(false)}>{tx('取消', 'Cancel')}</button>
              {diffFiles.length > 0 && (
                <button className="btn btn-primary" onClick={async () => {
                  setShowDiffModal(false)
                  await copyTeamSkillToPersonal(diffSourceSkillPath, true)
                }}>{tx('确认覆盖更新', 'Overwrite & Update')}</button>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Rollback Modal */}
      {showRollbackModal && (
        <div style={{
          position: 'fixed',
          top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)',
          backdropFilter: 'blur(8px)',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          zIndex: 1000,
        }}>
          <div className="card" style={{
            width: '80%',
            maxWidth: '500px',
            maxHeight: '60vh',
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'var(--weui-BG-2, #fff)',
            borderRadius: '12px',
            overflow: 'hidden',
            boxShadow: '0 8px 32px rgba(0,0,0,0.25)',
            border: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
          }}>
            <div className="card-header" style={{
              padding: '16px 24px',
              borderBottom: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
            }}>
              <h4 style={{ margin: 0, fontWeight: 'bold' }}>{tx('选择历史备份回滚', 'Rollback to backup')}</h4>
              <button className="btn btn-sm btn-icon" onClick={() => setShowRollbackModal(false)}>✕</button>
            </div>
            
            <div className="card-body" style={{
              padding: '24px',
              overflowY: 'auto',
              flex: 1,
            }}>
              <p style={{ marginBottom: '16px', color: 'var(--weui-FG-1, rgba(0,0,0,0.6))', fontSize: '14px' }}>
                {tx('选择该技能在此前更新时自动保存的备份，可一键将其还原覆盖回本地个人空间：', 'Select a previously auto-saved backup to restore to your personal space:')}
              </p>
              
              <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                {backupsList.map((backup) => {
                  const dateStr = backup.name.replace('-backup.zip', '')
                  let formattedDate = dateStr
                  if (dateStr.length >= 15) {
                    formattedDate = `${dateStr.substring(0,4)}-${dateStr.substring(4,6)}-${dateStr.substring(6,8)} ${dateStr.substring(9,11)}:${dateStr.substring(11,13)}:${dateStr.substring(13,15)} UTC`
                  }
                  return (
                    <button
                      key={backup.path}
                      className="btn"
                      style={{
                        width: '100%',
                        textAlign: 'left',
                        padding: '12px 16px',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        border: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
                        borderRadius: '6px',
                      }}
                      disabled={rollingBack}
                      onClick={() => {
                        setRollingBack(true)
                        void handleRollbackExecute(backup.path).finally(() => setRollingBack(false))
                      }}
                    >
                      <span>{formattedDate}</span>
                      <small style={{ color: 'var(--weui-LINK, #576b95)' }}>{tx('点击回退', 'Restore')}</small>
                    </button>
                  )
                })}
                {backupsList.length === 0 && (
                  <div style={{ textAlign: 'center', padding: '32px', color: 'var(--weui-FG-1, rgba(0,0,0,0.6))', fontSize: '14px' }}>
                    {tx('当前技能暂无历史备份包。', 'No history backups available.')}
                  </div>
                )}
              </div>
            </div>
            
            <div className="card-footer" style={{
              padding: '16px 24px',
              borderTop: '1px solid var(--weui-FG-3, rgba(0,0,0,0.1))',
              display: 'flex',
              justifyContent: 'flex-end',
            }}>
              <button className="btn" disabled={rollingBack} onClick={() => setShowRollbackModal(false)}>{tx('关闭', 'Close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
