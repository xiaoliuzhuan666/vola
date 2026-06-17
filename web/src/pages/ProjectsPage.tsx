import { useCallback, useEffect, useMemo, useState, type DragEvent, type FormEvent } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import {
  api,
  type FileNode,
  type LocalLibraryImportResponse,
  type LocalLibraryMarkdownCandidate,
  type LocalLibraryProjectCandidate,
  type LocalLibraryScanResponse,
  type LocalPlatformImportSummary,
  type ProjectContextPack,
  type ProjectMaterial,
  type ProjectRepositoryExport,
  type ProjectRepositoryExportApplyResult,
} from '../api'
import GitHubTreeList from '../components/GitHubTreeList'
import MaterialsSectionToolbar from '../components/MaterialsSectionToolbar'
import MaterialsTile from '../components/MaterialsTile'
import FileMaterialsTile from '../components/FileMaterialsTile'
import ResourceActionMenu from '../components/ResourceActionMenu'
import ResourceConfirmDialog from '../components/ResourceConfirmDialog'
import SourceFilterBar from '../components/SourceFilterBar'
import useResourceCardMenu from '../hooks/useResourceCardMenu'
import useTreeDeleteDialog from '../hooks/useTreeDeleteDialog'
import { useI18n } from '../i18n'
import {
  buildFileTileModel,
  buildSourceFilterOptions,
  bundleBrowsePath,
  bundleCapabilityLabel,
  bundleRelativeDirFromPath,
  dataFileEditorRoute,
  dataProjectBundleRoute,
  fileNodeSource,
  getMaterialsSortOptions,
  isTextLikeFile,
  matchesSourceFilter,
  normalizeBundleRelativeDir,
  projectSource,
  sortMaterialsItems,
  sourceLabel,
  type MaterialsSortDir,
  type MaterialsSortKey,
} from './data/DataShared'

interface Project {
  name: string
  status: string
  description?: string
  primary_path?: string
  log_path?: string
  capabilities?: string[]
  last_activity?: string
  updated_at?: string
  context_md?: string
  metadata?: Record<string, any>
}

export default function ProjectsPage() {
  const { locale, tx } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const params = useParams()
  const projectName = (params.projectName || '').trim()
  const query = useMemo(() => new URLSearchParams(location.search), [location.search])
  const currentRelativeDir = normalizeBundleRelativeDir(query.get('dir'))
  const currentBundlePath = projectName ? `/projects/${projectName}` : ''
  const currentBrowsePath = currentBundlePath ? bundleBrowsePath(currentBundlePath, currentRelativeDir) : ''
  const isBundleView = Boolean(currentBundlePath)

  const [projects, setProjects] = useState<Project[]>([])
  const [bundleNode, setBundleNode] = useState<FileNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedProjectName, setSelectedProjectName] = useState<string | null>(null)
  const [selectedEntryPath, setSelectedEntryPath] = useState<string | null>(null)
  const [showNewForm, setShowNewForm] = useState(false)
  const [newName, setNewName] = useState('')
  const [creating, setCreating] = useState(false)
  const [sortKey, setSortKey] = useState<MaterialsSortKey>('updated_at')
  const [sortDir, setSortDir] = useState<MaterialsSortDir>('desc')
  const [sourceFilter, setSourceFilter] = useState('all')
  const [archiveTarget, setArchiveTarget] = useState<Project | null>(null)
  const [archiveSubmitting, setArchiveSubmitting] = useState(false)
  const [localModeAvailable, setLocalModeAvailable] = useState(false)
  const [localLibraryScan, setLocalLibraryScan] = useState<LocalLibraryScanResponse | null>(null)
  const [localLibraryImport, setLocalLibraryImport] = useState<LocalLibraryImportResponse | null>(null)
  const [localLibraryScanning, setLocalLibraryScanning] = useState(false)
  const [localLibraryImporting, setLocalLibraryImporting] = useState(false)
  const [localLibraryNotice, setLocalLibraryNotice] = useState('')
  const [codexImport, setCodexImport] = useState<LocalPlatformImportSummary | null>(null)
  const [codexImporting, setCodexImporting] = useState(false)
  const [projectMaterials, setProjectMaterials] = useState<ProjectMaterial[]>([])
  const [projectContextPacks, setProjectContextPacks] = useState<ProjectContextPack[]>([])
  const [repositoryExport, setRepositoryExport] = useState<ProjectRepositoryExport | null>(null)
  const [repositoryApply, setRepositoryApply] = useState<ProjectRepositoryExportApplyResult | null>(null)
  const [projectWorkspaceLoading, setProjectWorkspaceLoading] = useState(false)
  const [materialSaving, setMaterialSaving] = useState(false)
  const [packBuilding, setPackBuilding] = useState(false)
  const [repoExporting, setRepoExporting] = useState(false)
  const [repoApplying, setRepoApplying] = useState(false)
  const [copySourcePath, setCopySourcePath] = useState('')
  const [repoExportDraft, setRepoExportDraft] = useState({
    repositoryRoot: '',
    repositoryDir: 'docs/ai-context',
    overwrite: true,
  })
  const [materialDraft, setMaterialDraft] = useState({
    title: '',
    sourceUrl: '',
    repositoryPath: '',
    tags: '',
    content: '',
  })
  const [packDraft, setPackDraft] = useState({
    title: '',
    purpose: '',
    repositoryDir: 'docs/ai-context/context-packs',
  })
  const [selectedMaterialPaths, setSelectedMaterialPaths] = useState<string[]>([])
  const [selectedPackPaths, setSelectedPackPaths] = useState<string[]>([])
  const { activeMenuId, closeMenu, isMenuOpen, toggleMenu } = useResourceCardMenu()

  const loadProjectWorkspace = useCallback(async (name: string) => {
    if (!name) return
    setProjectWorkspaceLoading(true)
    try {
      const [materials, packs] = await Promise.all([
        api.getProjectMaterials(name),
        api.getProjectContextPacks(name),
      ])
      setProjectMaterials(materials)
      setProjectContextPacks(packs)
      setRepositoryApply(null)
      setSelectedMaterialPaths((paths) => paths.filter((path) => materials.some((item) => item.path === path)))
      setSelectedPackPaths((paths) => paths.filter((path) => packs.some((item) => item.path === path)))
    } catch (err: any) {
      setError(err.message || tx('加载项目资料失败', 'Failed to load project material'))
    } finally {
      setProjectWorkspaceLoading(false)
    }
  }, [tx])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      if (isBundleView) {
        const node = await api.getTree(currentBrowsePath)
        setBundleNode(node)
        setProjects([])
        setRepositoryExport(null)
        closeMenu()
        setSelectedEntryPath(null)
        void loadProjectWorkspace(projectName)
        return
      }

      const data = await api.getProjects()
      setProjects(data || [])
      setBundleNode(null)
      setProjectMaterials([])
      setProjectContextPacks([])
      setRepositoryExport(null)
      closeMenu()
      setSelectedProjectName(null)
    } catch (err: any) {
      setError(err.message || tx('加载项目失败', 'Failed to load projects'))
    } finally {
      setLoading(false)
    }
  }, [closeMenu, currentBrowsePath, isBundleView, loadProjectWorkspace, projectName, tx])

  const {
    closeDialog: closeDeleteDialog,
    confirmDelete,
    dialog: deleteDialog,
    requestDelete,
    submitting: deleteSubmitting,
  } = useTreeDeleteDialog({ tx, onDeleted: load })

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    let active = true
    api.getPublicConfig()
      .then((config) => {
        if (active) setLocalModeAvailable(!!config.local_mode)
      })
      .catch(() => {
        if (active) setLocalModeAvailable(false)
      })
    return () => {
      active = false
    }
  }, [])

  const currentBundleContext = bundleNode?.bundle_context
  const bundleEntries = bundleNode?.children || []
  const selectedProject = selectedProjectName
    ? projects.find((project) => project.name === selectedProjectName) || null
    : null
  const selectedDeletePath = isBundleView ? selectedEntryPath : null
  const canDeleteSelection = Boolean(isBundleView && selectedDeletePath)

  const projectCapabilities = (project: Project) => {
    const raw = Array.isArray(project.capabilities)
      ? project.capabilities.map((value) => `${value || ''}`.trim().toLowerCase()).filter(Boolean)
      : []
    return Array.from(new Set(raw.length > 0 ? raw : ['context', 'logs']))
  }

  const projectDescription = (project: Project) =>
    project.description || tx('这个项目 bundle 还没有补充描述。', 'This project bundle does not have a description yet.')

  const handleCreate = async (event: FormEvent) => {
    event.preventDefault()
    if (!newName.trim()) return

    setCreating(true)
    setError('')
    try {
      await api.createProject(newName.trim())
      setNewName('')
      setShowNewForm(false)
      await load()
    } catch (err: any) {
      setError(err.message || tx('新建项目失败', 'Failed to create project'))
    } finally {
      setCreating(false)
    }
  }

  const requestArchive = (project: Project) => {
    closeMenu()
    setArchiveTarget(project)
  }

  const closeArchiveDialog = () => {
    if (archiveSubmitting) return
    setArchiveTarget(null)
  }

  const handleArchive = async (project: Project) => {
    setArchiveSubmitting(true)
    try {
      await api.archiveProject(project.name)
      setArchiveTarget(null)
      await load()
    } catch (err: any) {
      setError(err.message || tx('归档项目失败', 'Failed to archive project'))
    } finally {
      setArchiveSubmitting(false)
    }
  }

  const handleLocalLibraryScan = async () => {
    setLocalLibraryScanning(true)
    setLocalLibraryNotice('')
    setLocalLibraryImport(null)
    setError('')
    try {
      const scan = await api.scanLocalLibrary({ max_markdown: 5000, max_projects: 500 })
      setLocalLibraryScan(scan)
    } catch (err: any) {
      setError(err.message || tx('扫描本地资料失败', 'Failed to scan local material'))
    } finally {
      setLocalLibraryScanning(false)
    }
  }

  const handleCodexImport = async () => {
    setCodexImporting(true)
    setLocalLibraryNotice('')
    setError('')
    try {
      const response = await api.importLocalPlatform({ platform: 'codex', mode: 'agent' })
      setCodexImport(response.data)
      const agent = response.data.agent
      setLocalLibraryNotice(tx(
        `Codex 已整理到 Vola：${agent?.profile_categories || 0} 组偏好、${agent?.memory_items || 0} 条记忆、${agent?.projects || 0} 个项目、${agent?.bundles || 0} 个 Skill、${agent?.conversations || 0} 个会话。`,
        `Codex imported into Vola: ${agent?.profile_categories || 0} profile groups, ${agent?.memory_items || 0} memory items, ${agent?.projects || 0} projects, ${agent?.bundles || 0} skills, ${agent?.conversations || 0} conversations.`,
      ))
      await load()
    } catch (err: any) {
      setError(err.message || tx('Codex 导入失败', 'Codex import failed'))
    } finally {
      setCodexImporting(false)
    }
  }

  const handleLocalLibraryImport = async () => {
    setLocalLibraryImporting(true)
    setLocalLibraryNotice('')
    setError('')
    try {
      const result = await api.importLocalLibrary({ max_markdown: 5000, max_projects: 500 })
      setLocalLibraryImport(result)
      setLocalLibraryNotice(tx(
        `已导入 ${result.stats.projects_shown} 个项目候选和 ${result.stats.markdown_shown} 个 Markdown 索引。`,
        `Imported ${result.stats.projects_shown} project candidates and ${result.stats.markdown_shown} Markdown index rows.`,
      ))
      await load()
    } catch (err: any) {
      setError(err.message || tx('导入本地资料索引失败', 'Failed to import local material index'))
    } finally {
      setLocalLibraryImporting(false)
    }
  }

  const parseTags = (value: string) =>
    value.split(',').map((item) => item.trim()).filter(Boolean)

  const resetMaterialDraft = () => {
    setMaterialDraft({ title: '', sourceUrl: '', repositoryPath: '', tags: '', content: '' })
  }

  const handleSaveMaterial = async (event: FormEvent) => {
    event.preventDefault()
    if (!projectName || !materialDraft.content.trim()) return
    setMaterialSaving(true)
    setError('')
    try {
      await api.saveProjectMaterial(projectName, {
        title: materialDraft.title.trim() || 'Project material',
        content: materialDraft.content,
        source_url: materialDraft.sourceUrl.trim() || undefined,
        repository_path: materialDraft.repositoryPath.trim() || undefined,
        tags: parseTags(materialDraft.tags),
        source_type: materialDraft.sourceUrl.trim() ? 'url' : 'markdown',
      })
      resetMaterialDraft()
      await loadProjectWorkspace(projectName)
      await load()
    } catch (err: any) {
      setError(err.message || tx('保存资料失败', 'Failed to save material'))
    } finally {
      setMaterialSaving(false)
    }
  }

  const handleCopyMaterial = async (sourcePath?: string) => {
    const path = (sourcePath || copySourcePath).trim()
    if (!projectName || !path) return
    setMaterialSaving(true)
    setError('')
    try {
      await api.copyProjectMaterial(projectName, {
        source_path: path,
        repository_path: materialDraft.repositoryPath.trim() || undefined,
        source_url: materialDraft.sourceUrl.trim() || undefined,
        tags: parseTags(materialDraft.tags),
      })
      setCopySourcePath('')
      await loadProjectWorkspace(projectName)
      await load()
    } catch (err: any) {
      setError(err.message || tx('复制资料失败', 'Failed to copy material'))
    } finally {
      setMaterialSaving(false)
    }
  }

  const handleMaterialDrop = async (event: DragEvent<HTMLElement>) => {
    event.preventDefault()
    const path = event.dataTransfer.getData('text/plain')
    if (path) await handleCopyMaterial(path)
  }

  const handleBuildContextPack = async (event: FormEvent) => {
    event.preventDefault()
    if (!projectName) return
    setPackBuilding(true)
    setError('')
    try {
      const pack = await api.buildProjectContextPack(projectName, {
        title: packDraft.title.trim() || `${projectName} AI context`,
        purpose: packDraft.purpose.trim() || undefined,
        repository_dir: packDraft.repositoryDir.trim() || undefined,
        material_paths: selectedMaterialPaths.length > 0 ? selectedMaterialPaths : undefined,
        include_context: true,
        include_recent_logs: true,
      })
      setSelectedPackPaths([pack.path])
      setPackDraft((draft) => ({ ...draft, title: '', purpose: '' }))
      await loadProjectWorkspace(projectName)
      await load()
    } catch (err: any) {
      setError(err.message || tx('生成上下文包失败', 'Failed to build context pack'))
    } finally {
      setPackBuilding(false)
    }
  }

  const handleBuildRepositoryExport = async () => {
    if (!projectName) return
    setRepoExporting(true)
    setError('')
    try {
      const result = await api.buildProjectRepositoryExport(projectName, {
        repository_dir: repoExportDraft.repositoryDir.trim() || 'docs/ai-context',
        material_paths: selectedMaterialPaths.length > 0 ? selectedMaterialPaths : undefined,
        pack_paths: selectedPackPaths.length > 0 ? selectedPackPaths : undefined,
        include_index: true,
      })
      setRepositoryExport(result)
      setRepositoryApply(null)
    } catch (err: any) {
      setError(err.message || tx('生成仓库资料失败', 'Failed to build repository export'))
    } finally {
      setRepoExporting(false)
    }
  }

  const handleApplyRepositoryExport = async () => {
    if (!projectName || !repoExportDraft.repositoryRoot.trim()) return
    setRepoApplying(true)
    setError('')
    try {
      const result = await api.applyProjectRepositoryExport(projectName, {
        repository_root: repoExportDraft.repositoryRoot.trim(),
        repository_dir: repoExportDraft.repositoryDir.trim() || 'docs/ai-context',
        material_paths: selectedMaterialPaths.length > 0 ? selectedMaterialPaths : undefined,
        pack_paths: selectedPackPaths.length > 0 ? selectedPackPaths : undefined,
        include_index: true,
        overwrite: repoExportDraft.overwrite,
      })
      setRepositoryApply(result)
      setRepositoryExport({
        project: result.project,
        repository_dir: result.repository_dir,
        generated_at: result.generated_at,
        files: result.files.map((file) => ({
          path: file.path,
          content: '',
          source: file.source,
        })),
      })
    } catch (err: any) {
      setError(err.message || tx('写入仓库失败', 'Failed to write repository files'))
    } finally {
      setRepoApplying(false)
    }
  }

  const toggleMaterialPath = (path: string) => {
    setSelectedMaterialPaths((current) => (
      current.includes(path) ? current.filter((item) => item !== path) : [...current, path]
    ))
  }

  const togglePackPath = (path: string) => {
    setSelectedPackPaths((current) => (
      current.includes(path) ? current.filter((item) => item !== path) : [...current, path]
    ))
  }

  const openProjectBundle = useCallback((name: string, relativeDir = '') => {
    closeMenu()
    navigate(dataProjectBundleRoute(name, relativeDir))
  }, [closeMenu, navigate])

  const openFileEditor = useCallback((path: string) => {
    closeMenu()
    navigate(dataFileEditorRoute(path))
  }, [closeMenu, navigate])

  const openBundleFolder = useCallback((path: string) => {
    if (!projectName) return
    openProjectBundle(projectName, bundleRelativeDirFromPath(currentBundlePath, path))
  }, [currentBundlePath, openProjectBundle, projectName])

  const handleDownloadZip = useCallback(async (path: string) => {
    closeMenu()
    try {
      await api.downloadTreeZip(path)
    } catch (err: any) {
      setError(err.message || tx('下载 ZIP 失败', 'Failed to download ZIP'))
    }
  }, [closeMenu, tx])

  const formatTime = (ts?: string) => {
    if (!ts) return '-'
    try {
      return new Date(ts).toLocaleString(locale === 'zh-CN' ? 'zh-CN' : 'en-US')
    } catch {
      return ts
    }
  }

  const localMarkdownCategoryLabel = (category: string) => {
    switch (category) {
      case 'skill':
        return 'Skill'
      case 'agent-instructions':
        return tx('Agent 说明', 'Agent instructions')
      case 'codex-note':
        return tx('Codex 资料', 'Codex material')
      case 'general-playbook':
        return tx('通用方案', 'General')
      case 'learning-note':
        return tx('学习笔记', 'Learning')
      case 'sensitive-metadata':
        return tx('敏感文件名', 'Sensitive name')
      default:
        return tx('项目笔记', 'Project note')
    }
  }

  const renderLocalProjectPreview = (project: LocalLibraryProjectCandidate) => (
    <div key={project.path} className="project-bundle-path-row">
      <span className="project-bundle-path-label">{project.name}</span>
      <code className="project-bundle-path-value">{project.path}</code>
    </div>
  )

  const renderLocalMarkdownPreview = (doc: LocalLibraryMarkdownCandidate) => (
    <div key={doc.path} className="project-bundle-path-row">
      <span className="project-bundle-path-label">
        {doc.title} · {localMarkdownCategoryLabel(doc.category)}
      </span>
      <code className="project-bundle-path-value">{doc.path}</code>
    </div>
  )

  const sortOptions = getMaterialsSortOptions(locale)
  const getProjectLastActivity = (project: Project) => project.last_activity || project.updated_at
  const sortedProjects = useMemo(
    () =>
      sortMaterialsItems({
        items: projects,
        sortKey,
        sortDir,
        getName: (project) => project.name,
        getUpdatedAt: (project) => getProjectLastActivity(project),
      }),
    [projects, sortDir, sortKey],
  )
  const filteredProjects = useMemo(
    () => sortedProjects.filter((project) => matchesSourceFilter(projectSource(project), sourceFilter)),
    [sortedProjects, sourceFilter],
  )
  const sourceOptions = useMemo(
    () => buildSourceFilterOptions(projects, projectSource, locale),
    [locale, projects],
  )

  const sortedBundleEntries = useMemo(
    () =>
      sortMaterialsItems({
        items: bundleEntries,
        sortKey,
        sortDir,
        getName: (entry) => entry.name,
        getUpdatedAt: (entry) => entry.updated_at || entry.created_at,
        groupComparator: (left, right) => {
          const leftPriority = left.name === 'context.md' ? -2 : left.name === 'log.jsonl' ? -1 : 0
          const rightPriority = right.name === 'context.md' ? -2 : right.name === 'log.jsonl' ? -1 : 0
          if (leftPriority !== rightPriority) return leftPriority - rightPriority
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

  const relativeSegments = currentRelativeDir.split('/').filter(Boolean)

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (archiveTarget || deleteDialog || activeMenuId) return
      if (event.key === 'Escape') {
        if (isBundleView) setSelectedEntryPath(null)
        else setSelectedProjectName(null)
        return
      }
      if (isBundleView && event.key === 'Delete' && selectedDeletePath) {
        event.preventDefault()
        void requestDelete([selectedDeletePath])
        return
      }
      if (!isBundleView && event.key === 'Delete' && selectedProject?.status === 'active') {
        event.preventDefault()
        requestArchive(selectedProject)
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
        if (isTextLikeFile(entry.name, entry.mime_type)) {
          openFileEditor(entry.path)
        }
        return
      }
      if (!isBundleView && selectedProject) {
        openProjectBundle(selectedProject.name)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [
    activeMenuId,
    archiveTarget,
    bundleEntries,
    deleteDialog,
    isBundleView,
    openBundleFolder,
    openFileEditor,
    openProjectBundle,
    requestDelete,
    selectedDeletePath,
    selectedEntryPath,
    selectedProject,
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
              <button className="btn-text" onClick={() => navigate('/data/projects')}>{tx('项目', 'Projects')}</button>
              {currentBundleContext ? (
                <>
                  <span className="breadcrumbs-sep">/</span>
                  <button className="btn-text" onClick={() => openProjectBundle(projectName)}>{currentBundleContext.name}</button>
                </>
              ) : null}
              {relativeSegments.map((segment, index) => {
                const relative = relativeSegments.slice(0, index + 1).join('/')
                return (
                  <span key={relative}>
                    <span className="breadcrumbs-sep">/</span>
                    <button className="btn-text" onClick={() => openProjectBundle(projectName, relative)}>{segment}</button>
                  </span>
                )
              })}
            </nav>
            <div className="materials-kicker">Vola Data</div>
            <h2 className="materials-title">{currentBundleContext?.name || projectName}</h2>
            <p className="materials-subtitle">
              {tx('项目详情现在按 project bundle 浏览模式展示，深入子目录时也会保留上下文。', 'Project details now use a project-bundle browsing view, with context preserved while you drill into subfolders.')}
            </p>
          </div>
        </section>

        {error && <div className="alert alert-error">{error}</div>}
        {!error && !currentBundleContext && (
          <div className="alert alert-warn">{tx('没有找到这个 project bundle。', 'This project bundle could not be found.')}</div>
        )}

        {currentBundleContext ? (
          <GitHubTreeList
            rootPath={currentBundlePath}
            rootLabel={currentBundleContext.name}
            initialPath={currentBrowsePath || currentBundlePath}
            title={tx('项目文件', 'Project files')}
            description={tx('按 GitHub 文件列表样式浏览这个项目的所有文件。', 'Browse every file in this project with the GitHub-style file list.')}
            actionHref="/?type=Project"
            actionLabel={tx('在首页查看', 'View on Home')}
            onFileDragStart={(entry, event) => {
              if (!entry.name.toLowerCase().endsWith('.md')) return
              event.dataTransfer.setData('text/plain', entry.path)
              event.dataTransfer.effectAllowed = 'copy'
            }}
            fileMenuItems={(entry) => (
              !entry.is_dir && entry.name.toLowerCase().endsWith('.md')
                ? [{
                    key: 'copy-material',
                    label: tx('复制到项目资料', 'Copy to project material'),
                    onSelect: () => void handleCopyMaterial(entry.path),
                  }]
                : []
            )}
          />
        ) : null}

        {currentBundleContext ? (
          <section
            className="materials-panel project-context-workbench"
            onDragOver={(event) => {
              event.preventDefault()
              event.dataTransfer.dropEffect = 'copy'
            }}
            onDrop={(event) => void handleMaterialDrop(event)}
          >
            <div className="materials-section-head">
              <div>
                <h3 className="materials-section-title">{tx('项目资料', 'Project material')}</h3>
                <p className="materials-section-copy">{tx('右键 Markdown 文件或拖到这里，保存成这个项目的 AI 资料；需要协作时再写入项目仓库。', 'Right-click a Markdown file or drop it here to save it as AI material for this project, then write it into the repo for collaboration.')}</p>
              </div>
              <div className="form-actions">
                <button className="btn btn-sm materials-toolbar-control" disabled={projectWorkspaceLoading} onClick={() => void loadProjectWorkspace(projectName)}>
                  {projectWorkspaceLoading ? tx('刷新中...', 'Refreshing...') : tx('刷新', 'Refresh')}
                </button>
                <button className="btn btn-sm btn-primary materials-toolbar-control" disabled={repoExporting} onClick={() => void handleBuildRepositoryExport()}>
                  {repoExporting ? tx('生成中...', 'Creating...') : tx('生成仓库文件', 'Build repo files')}
                </button>
              </div>
            </div>

            <div className="project-repo-export-bar">
              <div className="form-group">
                <label htmlFor="project-repo-root">{tx('本机仓库根目录', 'Local repo root')}</label>
                <input
                  id="project-repo-root"
                  value={repoExportDraft.repositoryRoot}
                  onChange={(event) => setRepoExportDraft((draft) => ({ ...draft, repositoryRoot: event.target.value }))}
                  placeholder="/Users/me/work/app"
                />
              </div>
              <div className="form-group">
                <label htmlFor="project-repo-dir">{tx('仓库内目录', 'Repo directory')}</label>
                <input
                  id="project-repo-dir"
                  value={repoExportDraft.repositoryDir}
                  onChange={(event) => setRepoExportDraft((draft) => ({ ...draft, repositoryDir: event.target.value }))}
                  placeholder="docs/ai-context"
                />
              </div>
              <label className="project-repo-overwrite">
                <input
                  type="checkbox"
                  checked={repoExportDraft.overwrite}
                  onChange={(event) => setRepoExportDraft((draft) => ({ ...draft, overwrite: event.target.checked }))}
                />
                <span>{tx('覆盖已有文件', 'Overwrite files')}</span>
              </label>
              <button
                className="btn btn-primary"
                type="button"
                disabled={repoApplying || !repoExportDraft.repositoryRoot.trim()}
                onClick={() => void handleApplyRepositoryExport()}
              >
                {repoApplying ? tx('写入中...', 'Writing...') : tx('写入本机仓库', 'Write to repo')}
              </button>
            </div>

            <div className="project-context-grid">
              <form className="project-context-card" onSubmit={handleSaveMaterial}>
                <div className="data-record-title">{tx('保存 Markdown', 'Save Markdown')}</div>
                <div className="form-row project-context-form-row">
                  <div className="form-group">
                    <label htmlFor="project-material-title">{tx('标题', 'Title')}</label>
                    <input
                      id="project-material-title"
                      value={materialDraft.title}
                      onChange={(event) => setMaterialDraft((draft) => ({ ...draft, title: event.target.value }))}
                      placeholder={tx('后端接口说明', 'Backend API notes')}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="project-material-url">{tx('原文链接', 'Source URL')}</label>
                    <input
                      id="project-material-url"
                      value={materialDraft.sourceUrl}
                      onChange={(event) => setMaterialDraft((draft) => ({ ...draft, sourceUrl: event.target.value }))}
                      placeholder="https://..."
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="project-material-repo-path">{tx('仓库路径', 'Repo path')}</label>
                    <input
                      id="project-material-repo-path"
                      value={materialDraft.repositoryPath}
                      onChange={(event) => setMaterialDraft((draft) => ({ ...draft, repositoryPath: event.target.value }))}
                      placeholder="docs/ai-context/materials/backend-api.md"
                    />
                  </div>
                </div>
                <div className="form-group">
                  <label htmlFor="project-material-tags">{tx('标签', 'Tags')}</label>
                  <input
                    id="project-material-tags"
                    value={materialDraft.tags}
                    onChange={(event) => setMaterialDraft((draft) => ({ ...draft, tags: event.target.value }))}
                    placeholder={tx('backend, api', 'backend, api')}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="project-material-content">Markdown</label>
                  <textarea
                    id="project-material-content"
                    value={materialDraft.content}
                    onChange={(event) => setMaterialDraft((draft) => ({ ...draft, content: event.target.value }))}
                    placeholder={tx('粘贴 Markdown 内容，或从上方文件列表右键复制。', 'Paste Markdown, or copy from the file list above.')}
                    rows={7}
                  />
                </div>
                <div className="form-actions">
                  <button className="btn btn-primary" type="submit" disabled={materialSaving || !materialDraft.content.trim()}>
                    {materialSaving ? tx('保存中...', 'Saving...') : tx('保存资料', 'Save material')}
                  </button>
                  <button className="btn" type="button" onClick={resetMaterialDraft} disabled={materialSaving}>
                    {tx('清空', 'Clear')}
                  </button>
                </div>
              </form>

              <div className="project-context-card">
                <div className="data-record-title">{tx('从 Vola 文件复制', 'Copy from Vola file')}</div>
                <div className="form-group">
                  <label htmlFor="project-copy-source">{tx('文件路径', 'File path')}</label>
                  <input
                    id="project-copy-source"
                    value={copySourcePath}
                    onChange={(event) => setCopySourcePath(event.target.value)}
                    placeholder="/projects/demo/docs/backend.md"
                  />
                </div>
                <div className="form-actions">
                  <button className="btn btn-primary" type="button" disabled={materialSaving || !copySourcePath.trim()} onClick={() => void handleCopyMaterial()}>
                    {materialSaving ? tx('复制中...', 'Copying...') : tx('复制到资料', 'Copy to material')}
                  </button>
                </div>
                <div className="project-drop-zone">
                  {tx('把 Markdown 文件卡片拖到这里', 'Drop Markdown file cards here')}
                </div>
              </div>

              <form className="project-context-card" onSubmit={handleBuildContextPack}>
                <div className="data-record-title">{tx('AI 上下文包', 'AI context pack')}</div>
                <div className="form-group">
                  <label htmlFor="project-pack-title">{tx('标题', 'Title')}</label>
                  <input
                    id="project-pack-title"
                    value={packDraft.title}
                    onChange={(event) => setPackDraft((draft) => ({ ...draft, title: event.target.value }))}
                    placeholder={`${projectName} AI context`}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="project-pack-purpose">{tx('用途', 'Purpose')}</label>
                  <input
                    id="project-pack-purpose"
                    value={packDraft.purpose}
                    onChange={(event) => setPackDraft((draft) => ({ ...draft, purpose: event.target.value }))}
                    placeholder={tx('给 Codex 开始后端联调前读取', 'Read before Codex starts backend integration')}
                  />
                </div>
                <div className="form-group">
                  <label htmlFor="project-pack-repo-dir">{tx('仓库目录', 'Repo dir')}</label>
                  <input
                    id="project-pack-repo-dir"
                    value={packDraft.repositoryDir}
                    onChange={(event) => setPackDraft((draft) => ({ ...draft, repositoryDir: event.target.value }))}
                  />
                </div>
                <div className="form-actions">
                  <button className="btn btn-primary" type="submit" disabled={packBuilding}>
                    {packBuilding ? tx('生成中...', 'Creating...') : tx('生成上下文包', 'Build context pack')}
                  </button>
                </div>
              </form>
            </div>

            <div className="project-context-lists">
              <div className="project-context-card">
                <div className="data-record-head">
                  <div>
                    <div className="data-record-title">{tx('已保存资料', 'Saved material')}</div>
                    <div className="data-record-secondary">{tx(`${selectedMaterialPaths.length || projectMaterials.length} 个会进入下一份上下文包`, `${selectedMaterialPaths.length || projectMaterials.length} will be used in the next context pack`)}</div>
                  </div>
                </div>
                {projectMaterials.length === 0 ? (
                  <div className="dashboard-file-empty">{tx('还没有项目资料。', 'No project material yet.')}</div>
                ) : (
                  <div className="data-record-list">
                    {projectMaterials.map((item) => (
                      <button
                        key={item.path}
                        type="button"
                        className={`project-context-list-row${selectedMaterialPaths.includes(item.path) ? ' is-selected' : ''}`}
                        onClick={() => toggleMaterialPath(item.path)}
                      >
                        <span>{item.title}</span>
                        <code>{item.repository_path || item.path}</code>
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div className="project-context-card">
                <div className="data-record-title">{tx('上下文包', 'Context packs')}</div>
                {projectContextPacks.length === 0 ? (
                  <div className="dashboard-file-empty">{tx('还没有上下文包。', 'No context packs yet.')}</div>
                ) : (
                  <div className="data-record-list">
                    {projectContextPacks.map((item) => (
                      <button
                        key={item.path}
                        type="button"
                        className={`project-context-list-row${selectedPackPaths.includes(item.path) ? ' is-selected' : ''}`}
                        onClick={() => togglePackPath(item.path)}
                      >
                        <span>{item.title}</span>
                        <code>{item.repository_path || item.path}</code>
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div className="project-context-card">
                <div className="data-record-title">{tx('仓库文件', 'Repository files')}</div>
                {!repositoryExport ? (
                  <div className="dashboard-file-empty">{tx('生成或写入后会显示可提交到 Git 的文件。', 'Generated or written files ready for Git will appear here.')}</div>
                ) : (
                  <div className="project-bundle-paths">
                    {repositoryExport.files.map((file) => (
                      <div key={file.path} className="project-bundle-path-row">
                        <span className="project-bundle-path-label">{tx('文件', 'File')}</span>
                        <code className="project-bundle-path-value">{file.path}</code>
                      </div>
                    ))}
                  </div>
                )}
                {repositoryApply ? (
                  <div className="project-repo-apply-result">
                    <div className="data-record-secondary">
                      {tx(
                        `写入 ${repositoryApply.files.filter((file) => file.status === 'written').length} 个，跳过 ${repositoryApply.files.filter((file) => file.status === 'skipped').length} 个，失败 ${repositoryApply.files.filter((file) => file.status === 'error').length} 个。`,
                        `Written ${repositoryApply.files.filter((file) => file.status === 'written').length}, skipped ${repositoryApply.files.filter((file) => file.status === 'skipped').length}, failed ${repositoryApply.files.filter((file) => file.status === 'error').length}.`,
                      )}
                    </div>
                    <div className="project-bundle-paths">
                      {repositoryApply.files.map((file) => (
                        <div key={`${file.path}:${file.status}`} className="project-bundle-path-row">
                          <span className={`project-bundle-path-label repo-status-${file.status}`}>{file.status}</span>
                          <code className="project-bundle-path-value">{file.target_path || file.path}</code>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </section>
        ) : null}

        {currentBundleContext ? (
          <section className="materials-section">
            <div className="materials-section-head">
              <div>
                <h3 className="materials-section-title">{tx('Bundle 内容', 'Bundle contents')}</h3>
                <p className="materials-section-copy">{tx('项目内的文件和目录现在都在这个视图里继续浏览。', 'Continue browsing project files and directories from this bundle view.')}</p>
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
              <div className="empty-state">
                <p>{tx('这个项目目录还没有内容', 'This project folder is empty')}</p>
              </div>
            ) : (
              <div className="materials-grid">
                {filteredBundleEntries.map((entry) => {
                  const tile = buildFileTileModel({
                    node: entry,
                    variant: 'bundle-entry',
                    bundleLabel: currentBundleContext.name,
                    locale,
                  })
                  const editable = isTextLikeFile(entry.name, entry.mime_type)
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
                            ...((entry.is_dir || editable)
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
                            ...(!entry.is_dir && entry.name.toLowerCase().endsWith('.md')
                              ? [{
                                  key: 'copy-material',
                                  label: tx('复制到项目资料', 'Copy to project material'),
                                  onSelect: () => {
                                    closeMenu()
                                    void handleCopyMaterial(entry.path)
                                  },
                                }]
                              : []),
                            {
                              key: 'select',
                              label: selectedEntryPath === entry.path ? tx('取消选中', 'Unselect') : tx('加入选择', 'Select'),
                              onSelect: () => {
                                closeMenu()
                                setSelectedEntryPath((value) => (value === entry.path ? null : entry.path))
                              },
                            },
                            {
                              key: 'delete',
                              label: tx('删除', 'Delete'),
                              tone: 'danger' as const,
                              onSelect: () => {
                                closeMenu()
                                void requestDelete([entry.path])
                              },
                            },
                          ]}
                        />
                      )}
                      onMenuToggle={() => toggleMenu(entry.path)}
                      onSelect={() => setSelectedEntryPath(entry.path)}
                      onOpen={entry.is_dir ? () => openBundleFolder(entry.path) : (editable ? () => openFileEditor(entry.path) : undefined)}
                      draggable={!entry.is_dir && entry.name.toLowerCase().endsWith('.md')}
                      onDragStart={(event) => {
                        event.dataTransfer.setData('text/plain', entry.path)
                        event.dataTransfer.effectAllowed = 'copy'
                      }}
                      onContextMenu={(event) => {
                        event.preventDefault()
                        toggleMenu(entry.path)
                      }}
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
            ? tx('确认后会递归删除其中所有可写文件和文件夹。', 'Continuing will recursively delete all writable files and folders inside.')
            : tx('这个操作会删除选中的项目文件或目录，且不可撤销。', 'This will delete the selected project file or directory and cannot be undone.')}
          cancelLabel={tx('取消', 'Cancel')}
          confirmLabel={deleteSubmitting ? tx('删除中...', 'Deleting...') : tx('确认删除', 'Delete')}
          tone="danger"
          submitting={deleteSubmitting}
          onCancel={closeDeleteDialog}
          onConfirm={() => void confirmDelete()}
        />

        <ResourceConfirmDialog
          open={Boolean(archiveTarget)}
          kicker={tx('危险操作', 'Danger zone')}
          title={archiveTarget ? tx(`确认归档项目 “${archiveTarget.name}”`, `Archive project "${archiveTarget.name}"`) : ''}
          description={tx('归档不会删除项目文件，但项目会退出活跃状态。你之后仍然可以继续在文件树里查看它。', 'Archiving does not delete project files, but the project leaves the active state. You can still view it in the file tree afterward.')}
          cancelLabel={tx('取消', 'Cancel')}
          confirmLabel={archiveSubmitting ? tx('归档中...', 'Archiving...') : tx('确认归档', 'Archive')}
          tone="danger"
          submitting={archiveSubmitting}
          onCancel={closeArchiveDialog}
          onConfirm={() => {
            if (archiveTarget) void handleArchive(archiveTarget)
          }}
        />
      </div>
    )
  }

  return (
    <div className="page materials-page">
      <section className="materials-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">Vola Data</div>
          <h2 className="materials-title">{tx('项目', 'Projects')}</h2>
          <p className="materials-subtitle">{tx('把项目看成一组长期上下文卡片。点开后会进入 project bundle 视图。', 'Treat projects as long-lived context cards. Opening one jumps into the project bundle view.')}</p>
        </div>
      </section>

      {error && <div className="alert alert-error">{error}</div>}

      {localModeAvailable && (
        <div className="materials-panel form-card">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('Codex 与本地资料整理', 'Codex and local material')}</h3>
              <p className="materials-section-copy">
                {tx('把当前机器上的 Codex 配置、skills、会话和项目文档整理到 Vola。索引用来找资料；Codex 导入会放进记忆、项目、技能和会话模块。', 'Organize Codex config, skills, conversations, and project docs from this machine into Vola. The index helps you find material; Codex import places recognized items into memory, projects, skills, and conversations.')}
              </p>
            </div>
            <div className="form-actions">
              <button className="btn btn-sm materials-toolbar-control" onClick={() => void handleLocalLibraryScan()} disabled={localLibraryScanning || localLibraryImporting || codexImporting}>
                {localLibraryScanning ? tx('扫描中...', 'Scanning...') : tx('预览资料索引', 'Preview index')}
              </button>
              <button className="btn btn-sm materials-toolbar-control" onClick={() => void handleLocalLibraryImport()} disabled={localLibraryScanning || localLibraryImporting || codexImporting}>
                {localLibraryImporting ? tx('生成中...', 'Creating...') : tx('生成项目索引', 'Create project index')}
              </button>
              <button className="btn btn-sm btn-primary materials-toolbar-control" onClick={() => void handleCodexImport()} disabled={localLibraryScanning || localLibraryImporting || codexImporting}>
                {codexImporting ? tx('导入中...', 'Importing...') : tx('导入 Codex 资料', 'Import Codex')}
              </button>
            </div>
          </div>

          {localLibraryNotice && <div className="alert alert-success" style={{ marginTop: 12 }}>{localLibraryNotice}</div>}

          {localLibraryScan && (
            <div className="data-record-list" style={{ marginTop: 16 }}>
              <div className="data-record-item">
                <div className="data-record-title">{tx('扫描结果', 'Scan result')}</div>
                <div className="data-record-secondary">
                  {tx(
                    `${localLibraryScan.stats.roots_scanned} 个目录，${localLibraryScan.stats.projects_found} 个项目候选，${localLibraryScan.stats.markdown_found} 个 Markdown。`,
                    `${localLibraryScan.stats.roots_scanned} roots, ${localLibraryScan.stats.projects_found} project candidates, ${localLibraryScan.stats.markdown_found} Markdown files.`,
                  )}
                </div>
                {localLibraryScan.stats.sensitive_files > 0 && (
                  <div className="data-record-secondary">
                    {tx(
                      `${localLibraryScan.stats.sensitive_files} 个 Markdown 文件名看起来可能含账号或密钥，只导入路径和元数据。`,
                      `${localLibraryScan.stats.sensitive_files} Markdown filenames look account- or key-related; only path metadata is imported.`,
                    )}
                  </div>
                )}
                <div className="project-bundle-paths" style={{ marginTop: 10 }}>
                  {localLibraryScan.roots.map((root) => (
                    <div key={root.path} className="project-bundle-path-row">
                      <span className="project-bundle-path-label">{root.scanned ? tx('已扫描', 'Scanned') : tx('未扫描', 'Not scanned')}</span>
                      <code className="project-bundle-path-value">{root.path}</code>
                    </div>
                  ))}
                </div>
              </div>

              {localLibraryScan.projects.length > 0 && (
                <div className="data-record-item">
                  <div className="data-record-title">{tx('项目候选', 'Project candidates')}</div>
                  <div className="project-bundle-paths">
                    {localLibraryScan.projects.slice(0, 6).map(renderLocalProjectPreview)}
                  </div>
                </div>
              )}

              {localLibraryScan.markdown.length > 0 && (
                <div className="data-record-item">
                  <div className="data-record-title">{tx('Markdown 候选', 'Markdown candidates')}</div>
                  <div className="project-bundle-paths">
                    {localLibraryScan.markdown.slice(0, 6).map(renderLocalMarkdownPreview)}
                  </div>
                </div>
              )}
            </div>
          )}

          {localLibraryImport && (
            <div className="project-bundle-paths" style={{ marginTop: 12 }}>
              <div className="project-bundle-path-row">
                <span className="project-bundle-path-label">{tx('索引项目', 'Index project')}</span>
                <code className="project-bundle-path-value">/projects/{localLibraryImport.project_name}/</code>
              </div>
              <div className="materials-actions" style={{ marginTop: 12 }}>
                <button className="btn btn-sm" onClick={() => openProjectBundle(localLibraryImport.project_name)}>{tx('打开索引项目', 'Open index project')}</button>
              </div>
            </div>
          )}

          {codexImport?.agent && (
            <div className="codex-import-destinations">
              <Link to="/memory" className="codex-import-destination">
                <strong>{codexImport.agent.profile_categories + codexImport.agent.memory_items}</strong>
                <span>{tx('记忆与偏好', 'Memory & profile')}</span>
              </Link>
              <Link to="/data/projects" className="codex-import-destination">
                <strong>{codexImport.agent.projects}</strong>
                <span>{tx('项目', 'Projects')}</span>
              </Link>
              <Link to="/skills" className="codex-import-destination">
                <strong>{codexImport.agent.bundles}</strong>
                <span>Skills</span>
              </Link>
              <Link to="/data/conversations" className="codex-import-destination">
                <strong>{codexImport.agent.conversations}</strong>
                <span>{tx('会话', 'Conversations')}</span>
              </Link>
            </div>
          )}
        </div>
      )}

      <GitHubTreeList
        rootPath="/projects"
        rootLabel={tx('项目', 'Projects')}
        title={tx('项目文件', 'Project files')}
        description={tx('按文件夹层级浏览所有 Project bundle。', 'Browse all project bundles by folder.')}
        actionHref="/?type=Project"
        actionLabel={tx('在首页查看', 'View on Home')}
      />

      {showNewForm && (
        <div className="materials-panel form-card">
          <div className="materials-section-head">
            <div>
              <h3 className="materials-section-title">{tx('新建项目', 'New project')}</h3>
              <p className="materials-section-copy">{tx('创建一个新的项目空间，用来整理任务上下文、日志和相关资料。', 'Create a new project space to organize task context, logs, and supporting material.')}</p>
            </div>
          </div>
          <form onSubmit={handleCreate}>
            <div className="form-group">
              <label htmlFor="proj-name">{tx('项目名称', 'Project name')}</label>
              <input
                id="proj-name"
                type="text"
                value={newName}
                onChange={(event) => setNewName(event.target.value)}
                placeholder={tx('例如：blog-redesign', 'For example: blog-redesign')}
                disabled={creating}
                autoFocus
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={creating}>
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
            <h3 className="materials-section-title">{tx('项目库', 'Project Library')}</h3>
            <p className="materials-section-copy">{tx('统一浏览项目卡片，打开后进入 project bundle。', 'Browse project cards here, then open one into its project bundle.')}</p>
          </div>
          <MaterialsSectionToolbar
            count={filteredProjects.length}
            sortKey={sortKey}
            sortOptions={sortOptions}
            sortDir={sortDir}
            onSortKeyChange={(value) => setSortKey(value as MaterialsSortKey)}
            onSortDirToggle={() => setSortDir((value) => (value === 'desc' ? 'asc' : 'desc'))}
          >
            <button className="btn btn-sm materials-toolbar-control" onClick={() => setShowNewForm((value) => !value)}>
              {showNewForm ? tx('取消新建', 'Close form') : tx('新建项目', 'New project')}
            </button>
            <button
              className="btn btn-sm materials-toolbar-control is-danger"
              disabled={selectedProject?.status !== 'active'}
              onClick={() => {
                if (selectedProject) requestArchive(selectedProject)
              }}
            >
              {tx('归档项目', 'Archive project')}
            </button>
          </MaterialsSectionToolbar>
        </div>

        {(sourceOptions.length > 1 || sourceFilter !== 'all') && (
          <SourceFilterBar options={sourceOptions} value={sourceFilter} onChange={setSourceFilter} />
        )}

        {filteredProjects.length === 0 ? (
          <div className="empty-state">
            <p>{tx('暂无项目', 'No projects yet')}</p>
            <p className="empty-hint">{tx('项目帮助 Agent 组织不同任务的上下文和进度。', 'Projects help agents organize context and progress across different tasks.')}</p>
          </div>
        ) : (
          <div className="materials-grid materials-grid-wide">
            {filteredProjects.map((project) => {
              const source = projectSource(project)
              return (
                <MaterialsTile
                  key={project.name}
                  iconClassName="icon-folder"
                  title={project.name}
                  titleActionAriaLabel={tx(`打开项目 ${project.name} 的 bundle 目录`, `Open the ${project.name} bundle folder`)}
                  subtitle={`${tx('项目 bundle', 'Project bundle')} · ${project.status}`}
                  description={projectDescription(project)}
                  path={`/projects/${project.name}/`}
                  pills={(
                    <>
                      {source ? (
                        <span className="materials-tile-pill materials-source-pill">
                          {sourceLabel(source, locale)}
                        </span>
                      ) : null}
                      {projectCapabilities(project).map((capability) => (
                        <span key={capability} className="materials-tile-pill">
                          {bundleCapabilityLabel(capability, locale)}
                        </span>
                      ))}
                    </>
                  )}
                  footerStart={tx('最后活动', 'Last activity')}
                  footerEnd={formatTime(getProjectLastActivity(project))}
                  selected={selectedProjectName === project.name}
                  menuOpen={isMenuOpen(project.name)}
                  menuButtonAriaLabel={tx(`打开项目 ${project.name} 的工具菜单`, `Open tools menu for project ${project.name}`)}
                  menuPanel={(
                    <ResourceActionMenu
                      items={[
                        {
                          key: 'bundle',
                          label: tx('打开 bundle', 'Open bundle'),
                          onSelect: () => {
                            closeMenu()
                            openProjectBundle(project.name)
                          },
                        },
                        {
                          key: 'open',
                          label: tx('打开 context', 'Open context'),
                          onSelect: () => {
                            closeMenu()
                            navigate(dataFileEditorRoute(project.primary_path || `/projects/${project.name}/context.md`))
                          },
                        },
                        {
                          key: 'log',
                          label: tx('打开日志', 'Open log'),
                          onSelect: () => {
                            closeMenu()
                            navigate(dataFileEditorRoute(project.log_path || `/projects/${project.name}/log.jsonl`))
                          },
                        },
                        {
                          key: 'select',
                          label: selectedProjectName === project.name ? tx('取消选中', 'Unselect') : tx('加入选择', 'Select'),
                          onSelect: () => {
                            closeMenu()
                            setSelectedProjectName((value) => (value === project.name ? null : project.name))
                          },
                        },
                        ...(project.status === 'active'
                          ? [{
                              key: 'archive',
                              label: tx('归档项目', 'Archive project'),
                              tone: 'danger' as const,
                              onSelect: () => requestArchive(project),
                            }]
                          : []),
                      ]}
                    />
                  )}
                  onMenuToggle={() => toggleMenu(project.name)}
                  onSelect={() => setSelectedProjectName((value) => (value === project.name ? null : project.name))}
                  onOpen={() => openProjectBundle(project.name)}
                >
                  <div className="project-bundle-paths">
                    <div className="project-bundle-path-row">
                      <span className="project-bundle-path-label">{tx('主文件', 'Primary')}</span>
                      <code className="project-bundle-path-value">{project.primary_path || `/projects/${project.name}/context.md`}</code>
                    </div>
                    <div className="project-bundle-path-row">
                      <span className="project-bundle-path-label">{tx('Log', 'Log')}</span>
                      <code className="project-bundle-path-value">{project.log_path || `/projects/${project.name}/log.jsonl`}</code>
                    </div>
                  </div>
                </MaterialsTile>
              )
            })}
          </div>
        )}
      </section>

      <ResourceConfirmDialog
        open={Boolean(archiveTarget)}
        kicker={tx('危险操作', 'Danger zone')}
        title={archiveTarget ? tx(`确认归档项目 “${archiveTarget.name}”`, `Archive project "${archiveTarget.name}"`) : ''}
        description={tx('归档不会删除项目文件，但项目会退出活跃状态。你之后仍然可以在文件树里查看它。', 'Archiving does not delete project files, but the project leaves the active state. You can still view it in the file tree later.')}
        cancelLabel={tx('取消', 'Cancel')}
        confirmLabel={archiveSubmitting ? tx('归档中...', 'Archiving...') : tx('确认归档', 'Archive')}
        tone="danger"
        submitting={archiveSubmitting}
        onCancel={closeArchiveDialog}
        onConfirm={() => {
          if (archiveTarget) void handleArchive(archiveTarget)
        }}
      />
    </div>
  )
}
