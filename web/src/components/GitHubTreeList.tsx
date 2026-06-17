import { useEffect, useMemo, useState, type DragEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode } from '../api'
import { useI18n, type AppLocale } from '../i18n'
import { dataFileEditorRoute, formatDateTime, sourceLabel } from '../pages/data/DataShared'
import ResourceActionMenu, { type ResourceActionMenuItem } from './ResourceActionMenu'

interface GitHubTreeListProps {
  rootPath?: string
  rootLabel?: string
  path?: string
  initialPath?: string
  title: string
  description?: string
  actionHref?: string
  actionLabel?: string
  className?: string
  loadNode?: (path: string) => Promise<FileNode>
  fileRoute?: (path: string) => string
  onPathChange?: (path: string) => void
  onFileDragStart?: (entry: FileNode, event: DragEvent<HTMLElement>) => void
  fileMenuItems?: (entry: FileNode) => ResourceActionMenuItem[]
}

const rootPath = '/'

function normalizePath(path?: string) {
  if (!path || path === rootPath) return rootPath
  return `/${path.replace(/^\/+|\/+$/g, '')}`
}

function pathParts(path: string) {
  return normalizePath(path).split('/').filter(Boolean)
}

function joinPath(parts: string[]) {
  return parts.length === 0 ? rootPath : `/${parts.join('/')}`
}

function parentPath(path: string, root: string) {
  const parts = pathParts(path)
  const next = joinPath(parts.slice(0, -1))
  if (root !== rootPath && !next.startsWith(root)) return root
  return next
}

function relativeParts(root: string, path: string) {
  const normalizedRoot = normalizePath(root)
  const normalizedPath = normalizePath(path)
  if (normalizedRoot === rootPath) return pathParts(normalizedPath)
  return normalizedPath.replace(normalizedRoot, '').split('/').filter(Boolean)
}

function fileTypeLabel(node: FileNode, locale: AppLocale) {
  if (node.is_dir) return locale === 'zh-CN' ? '文件夹' : 'Folder'
  const path = node.path.toLowerCase()
  if (path.startsWith('/conversations/')) return locale === 'zh-CN' ? '会话' : 'Conversation'
  if (path.startsWith('/memory/')) return locale === 'zh-CN' ? '记忆' : 'Memory'
  if (path.startsWith('/skills/')) return locale === 'zh-CN' ? '技能' : 'Skill'
  if (path.startsWith('/projects/')) return locale === 'zh-CN' ? '项目' : 'Project'
  if (path.startsWith('/vault/')) return 'Vault'
  return locale === 'zh-CN' ? '文件' : 'File'
}

function formatBytes(value?: number) {
  const bytes = Number(value || 0)
  if (!Number.isFinite(bytes) || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  const kib = bytes / 1024
  if (kib < 1024) return `${kib.toFixed(kib >= 10 ? 0 : 1)} KiB`
  const mib = kib / 1024
  return `${mib.toFixed(mib >= 10 ? 0 : 1)} MiB`
}

function isMissingTreeError(err: any) {
  const status = Number(err?.status || err?.code)
  const message = `${err?.message || err || ''}`.toLowerCase()
  return status === 404 || message.includes('not found') || message.includes('file not found')
}

export default function GitHubTreeList({
  rootPath: rawRootPath = rootPath,
  rootLabel,
  path,
  initialPath,
  title,
  description,
  actionHref,
  actionLabel,
  className = '',
  loadNode,
  fileRoute,
  onPathChange,
  onFileDragStart,
  fileMenuItems,
}: GitHubTreeListProps) {
  const { locale, tx } = useI18n()
  const normalizedRootPath = normalizePath(rawRootPath)
  const [internalPath, setInternalPath] = useState(normalizePath(initialPath || normalizedRootPath))
  const [node, setNode] = useState<FileNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [openMenuPath, setOpenMenuPath] = useState('')
  const currentPath = normalizePath(path || internalPath)
  const displayRootLabel = rootLabel || (normalizedRootPath === rootPath ? tx('根目录', 'Root') : normalizedRootPath.replace(/^\/+/, ''))

  useEffect(() => {
    if (path) return
    setInternalPath(normalizePath(initialPath || normalizedRootPath))
  }, [initialPath, normalizedRootPath, path])

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      setError('')
      try {
        const next = await (loadNode ? loadNode(currentPath) : api.getTree(currentPath))
        if (!cancelled) setNode(next)
      } catch (err: any) {
        if (!cancelled) {
          if (currentPath === normalizedRootPath && isMissingTreeError(err)) {
            setNode({
              path: normalizedRootPath,
              name: displayRootLabel,
              is_dir: true,
              children: [],
            })
          } else {
            setError(err?.message || tx('加载文件失败', 'Failed to load files'))
          }
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [currentPath, displayRootLabel, loadNode, normalizedRootPath, tx])

  const entries = useMemo(() => {
    const children = node?.children || []
    return [...children].sort((a, b) => {
      if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
      return a.name.localeCompare(b.name)
    })
  }, [node])

  const setPath = (nextPath: string) => {
    const normalized = normalizePath(nextPath)
    if (!path) setInternalPath(normalized)
    onPathChange?.(normalized)
    setOpenMenuPath('')
  }

  const currentRelativeParts = relativeParts(normalizedRootPath, currentPath)
  const showParent = currentPath !== normalizedRootPath

  return (
    <section className={`dashboard-file-panel github-tree-list ${className}`.trim()}>
      <div className="dashboard-section-head">
        <div>
          <h3>{title}</h3>
          {description && <p>{description}</p>}
          <div className="dashboard-breadcrumb" aria-label={tx('当前位置', 'Current path')}>
            <button onClick={() => setPath(normalizedRootPath)}>{displayRootLabel}</button>
            {currentRelativeParts.map((part, index) => {
              const rootParts = normalizedRootPath === rootPath ? [] : pathParts(normalizedRootPath)
              const to = joinPath([...rootParts, ...currentRelativeParts.slice(0, index + 1)])
              return (
                <span key={to}>
                  <span>/</span>
                  <button onClick={() => setPath(to)}>{part}</button>
                </span>
              )
            })}
          </div>
        </div>
        {actionHref && <Link to={actionHref} className="dashboard-card-link">{actionLabel || tx('打开', 'Open')}</Link>}
      </div>

      {error && <div className="alert alert-warn">{error}</div>}

      <div className="dashboard-github-file-list" aria-busy={loading}>
        {showParent && (
          <button className="dashboard-file-row" onClick={() => setPath(parentPath(currentPath, normalizedRootPath))}>
            <span className="dashboard-file-icon">↩</span>
            <span className="dashboard-file-name">..</span>
            <span className="dashboard-file-kind">{tx('上一级', 'Parent')}</span>
            <span className="dashboard-file-source">-</span>
            <span className="dashboard-file-updated">-</span>
          </button>
        )}

        {entries.map((entry) => {
          const nextPath = normalizePath(entry.path || `${currentPath.replace(/\/+$/, '')}/${entry.name}`)
          const source = entry.source || entry.metadata?.source || entry.bundle_context?.source || 'system'
          const menuItems = fileMenuItems?.(entry) || []
          const menu = menuItems.length > 0 ? (
            <span className="dashboard-file-menu-wrap" onClick={(event) => event.preventDefault()}>
              <button
                type="button"
                className="materials-tile-menu-button"
                aria-label={tx(`打开 ${entry.name} 的工具菜单`, `Open tools menu for ${entry.name}`)}
                onClick={(event) => {
                  event.preventDefault()
                  event.stopPropagation()
                  setOpenMenuPath((value) => (value === nextPath ? '' : nextPath))
                }}
              >
                ⋮
              </button>
              {openMenuPath === nextPath ? (
                <span className="dashboard-file-menu-panel" onClick={(event) => event.stopPropagation()}>
                  <ResourceActionMenu items={menuItems} />
                </span>
              ) : null}
            </span>
          ) : (
            <span />
          )
          if (entry.is_dir) {
            return (
              <button key={nextPath} className="dashboard-file-row" onClick={() => setPath(nextPath)}>
                <span className="dashboard-file-icon">▸</span>
                <span className="dashboard-file-name is-folder">{entry.name}</span>
                <span className="dashboard-file-kind">{fileTypeLabel(entry, locale)}</span>
                <span className="dashboard-file-source">{sourceLabel(String(source), locale)}</span>
                <span className="dashboard-file-updated">{formatDateTime(entry.updated_at || entry.created_at, locale)}</span>
                {menu}
              </button>
            )
          }
          return (
            <Link
              key={nextPath}
              to={fileRoute ? fileRoute(nextPath) : dataFileEditorRoute(nextPath)}
              className="dashboard-file-row"
              draggable={Boolean(onFileDragStart)}
              onDragStart={(event) => onFileDragStart?.(entry, event)}
              onContextMenu={(event) => {
                if (menuItems.length === 0) return
                event.preventDefault()
                setOpenMenuPath(nextPath)
              }}
            >
              <span className="dashboard-file-icon">•</span>
              <span className="dashboard-file-name">{entry.name}</span>
              <span className="dashboard-file-kind">{fileTypeLabel(entry, locale)} · {formatBytes(entry.size || entry.content?.length)}</span>
              <span className="dashboard-file-source">{sourceLabel(String(source), locale)}</span>
              <span className="dashboard-file-updated">{formatDateTime(entry.updated_at || entry.created_at, locale)}</span>
              {menu}
            </Link>
          )
        })}

        {!loading && entries.length === 0 && (
          <div className="dashboard-file-empty">{tx('这个目录暂时没有文件。', 'This folder is empty.')}</div>
        )}
        {loading && <div className="dashboard-file-empty">{tx('加载中...', 'Loading...')}</div>}
      </div>
    </section>
  )
}
