import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type FileNode } from '../../api'
import GitHubTreeList from '../../components/GitHubTreeList'
import { useI18n } from '../../i18n'
import { dataFileEditorRoute, formatDateTime, sourceLabel } from './DataShared'

function isConversation(node: FileNode) {
  return node.bundle_context?.kind === 'conversation' || node.path.startsWith('/conversations/')
}

function titleFor(node: FileNode) {
  return String(node.metadata?.conversation_title || node.bundle_context?.name || node.name)
}

function transcriptPath(node: FileNode) {
  return node.is_dir ? `${node.path.replace(/\/+$/, '')}/conversation.md` : node.path
}

function summaryFor(node: FileNode, tx: (zh: string, en: string) => string) {
  const summary = node.metadata?.summary || node.metadata?.description || node.bundle_context?.description
  if (typeof summary === 'string' && summary.trim()) return summary.trim()
  return tx('可复用为记忆、项目上下文或跨 Agent 提示。', 'Ready to reuse as memory, project context, or a replay prompt.')
}

export default function DataConversationsPage() {
  const { locale, tx } = useI18n()
  const [items, setItems] = useState<FileNode[]>([])
  const [selected, setSelected] = useState<FileNode | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const snapshot = await api.getTreeSnapshot('/conversations')
      const conversations = snapshot.entries
        .filter(isConversation)
        .filter((entry) => entry.is_dir || entry.name.endsWith('.md'))
        .sort((a, b) => new Date(b.updated_at || b.created_at || 0).getTime() - new Date(a.updated_at || a.created_at || 0).getTime())
      setItems(conversations)
      setSelected((current) => current || conversations[0] || null)
    } catch (err: any) {
      setError(err?.message || tx('加载会话失败', 'Failed to load conversations'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const statusFor = (node: FileNode) => {
    const hasSummary = Boolean(node.metadata?.summary || node.metadata?.description || node.bundle_context?.description)
    return hasSummary ? tx('已有摘要', 'Has summary') : tx('已导入', 'Imported')
  }

  const copyReplayPrompt = async (node: FileNode) => {
    const prompt = `Replay this Vola conversation into the current agent as reusable context: ${transcriptPath(node)}. Extract the key decisions, preferences, and next actions before answering.`
    await navigator.clipboard?.writeText(prompt)
    setCopied(node.path)
    window.setTimeout(() => setCopied(''), 1600)
  }

  const convertToMemory = async (node: FileNode) => {
    const safeName = titleFor(node).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48) || 'conversation-memory'
    try {
      await api.writeTree(`/memory/${safeName}.md`, {
        content: `# ${titleFor(node)}\n\nSource conversation: ${transcriptPath(node)}\n\n${summaryFor(node, tx)}\n`,
        mimeType: 'text/markdown',
        metadata: { source: 'conversation', source_path: node.path },
        minTrustLevel: node.min_trust_level || node.bundle_context?.min_trust_level || 3,
      })
      setError(tx('已创建 Memory 条目。', 'Memory entry created.'))
    } catch (err: any) {
      setError(err?.message || tx('创建 Memory 失败', 'Failed to create memory'))
    }
  }

  const addToProject = async (node: FileNode) => {
    const projectName = window.prompt(tx('输入项目名称', 'Project name'))
    if (!projectName) return
    try {
      await api.appendProjectLog(projectName, {
        source: 'vola',
        action: 'link_conversation',
        summary: `${titleFor(node)} · ${transcriptPath(node)}`,
        tags: ['conversation'],
      })
      setError(tx('已添加到项目记录。', 'Added to project log.'))
    } catch (err: any) {
      setError(err?.message || tx('添加到项目失败', 'Failed to add to project'))
    }
  }

  const deleteConversation = async (node: FileNode) => {
    if (!window.confirm(tx(`删除会话 ${titleFor(node)}？`, `Delete conversation ${titleFor(node)}?`))) return
    try {
      await api.deleteTree(node.path)
      setSelected(null)
      await load()
    } catch (err: any) {
      setError(err?.message || tx('删除失败', 'Delete failed'))
    }
  }

  const sourceCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const item of items) {
      const source = String(item.bundle_context?.source || item.source || item.metadata?.source || 'unknown')
      counts.set(source, (counts.get(source) || 0) + 1)
    }
    return Array.from(counts.entries())
  }, [items])

  if (loading) return <div className="page-loading">{tx('加载中...', 'Loading...')}</div>

  return (
    <div className="page conversations-page">
      <div className="page-header compact-header">
        <div>
          <h2>{tx('会话', 'Conversations')}</h2>
          <p className="page-subtitle">
            {tx('已导入的 AI 会话可以变成记忆、项目上下文或可复用参考。', 'Imported AI conversations that can become memory, project context or reusable references.')}
          </p>
        </div>
        <div className="page-actions">
          <Link to="/imports/claude-export" className="btn btn-primary">{tx('导入会话', 'Import conversations')}</Link>
        </div>
      </div>

      {error && <div className={error.includes('失败') || error.toLowerCase().includes('failed') ? 'alert alert-warn' : 'alert alert-success'}>{error}</div>}

      <GitHubTreeList
        rootPath="/conversations"
        rootLabel={tx('会话', 'Conversations')}
        title={tx('会话文件', 'Conversation files')}
        description={tx('按来源和导入批次浏览所有会话文件。', 'Browse all conversation files by source and import batch.')}
        actionHref="/?type=Conversation"
        actionLabel={tx('在首页查看', 'View on Home')}
      />

      <section className="conversation-layout">
        <div className="conversation-list-panel">
          <div className="data-summary-grid compact">
            <div className="data-summary-card"><span>{tx('总数', 'Total')}</span><strong>{items.length}</strong><small>{tx('会话', 'conversations')}</small></div>
            {sourceCounts.slice(0, 3).map(([source, count]) => (
              <div key={source} className="data-summary-card"><span>{sourceLabel(source, locale)}</span><strong>{count}</strong><small>{tx('来源', 'source')}</small></div>
            ))}
          </div>
          <table className="data-table conversation-table">
            <thead>
              <tr>
                <th>{tx('标题', 'Title')}</th>
                <th>{tx('来源', 'Source')}</th>
                <th>{tx('日期', 'Date')}</th>
                <th>{tx('状态', 'Status')}</th>
                <th>{tx('操作', 'Actions')}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((node) => (
                <tr key={node.path} className={selected?.path === node.path ? 'is-selected' : ''} onClick={() => setSelected(node)}>
                  <td><strong>{titleFor(node)}</strong><small>{summaryFor(node, tx)}</small></td>
                  <td>{sourceLabel(String(node.bundle_context?.source || node.source || node.metadata?.source || 'unknown'), locale)}</td>
                  <td>{formatDateTime(node.updated_at || node.created_at, locale)}</td>
                  <td>{statusFor(node)}</td>
                  <td><button className="btn-text" onClick={(event) => { event.stopPropagation(); setSelected(node) }}>{tx('打开', 'Open')}</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <aside className="conversation-detail-panel">
          {selected ? (
            <>
              <h3>{titleFor(selected)}</h3>
              <p>{summaryFor(selected, tx)}</p>
              <dl className="preview-meta">
                <div><dt>{tx('来源', 'Source')}</dt><dd>{sourceLabel(String(selected.bundle_context?.source || selected.source || selected.metadata?.source || 'unknown'), locale)}</dd></div>
                <div><dt>{tx('转录文件', 'Transcript')}</dt><dd><code>{transcriptPath(selected)}</code></dd></div>
                <div><dt>{tx('状态', 'Status')}</dt><dd>{statusFor(selected)}</dd></div>
                <div><dt>{tx('更新时间', 'Updated')}</dt><dd>{formatDateTime(selected.updated_at || selected.created_at, locale)}</dd></div>
              </dl>
              <div className="drawer-actions">
                <Link className="btn btn-primary" to={dataFileEditorRoute(transcriptPath(selected))}>{tx('打开转录', 'Open transcript')}</Link>
                <button className="btn btn-outline" onClick={() => { void copyReplayPrompt(selected) }}>{copied === selected.path ? tx('已复制', 'Copied') : tx('Replay 到另一个 Agent', 'Replay into another agent')}</button>
                <button className="btn btn-outline" onClick={() => { void convertToMemory(selected) }}>{tx('转为记忆', 'Convert to memory')}</button>
                <button className="btn btn-outline" onClick={() => { void addToProject(selected) }}>{tx('添加到项目', 'Add to project')}</button>
                <button className="btn btn-danger" onClick={() => { void deleteConversation(selected) }}>{tx('删除', 'Delete')}</button>
              </div>
            </>
          ) : (
            <div className="empty-action-state">
              <p>{tx('还没有会话。', 'No conversations yet.')}</p>
              <Link className="btn btn-primary" to="/imports/claude-export">{tx('导入会话', 'Import conversations')}</Link>
            </div>
          )}
        </aside>
      </section>
    </div>
  )
}
