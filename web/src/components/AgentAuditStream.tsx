import { useCallback, useEffect, useState } from 'react'
import { api } from '../api'
import { useI18n } from '../i18n'

type AgentActivity = {
  id?: string
  ID?: string
  action?: string
  Action?: string
  path?: string
  Path?: string
  metadata?: Record<string, any>
  Metadata?: Record<string, any>
  created_at?: string
  CreatedAt?: string
}

function activityID(activity: AgentActivity, index: number) {
  return activity.id || activity.ID || `${activity.action || activity.Action || 'activity'}-${index}`
}

function activityAction(activity: AgentActivity) {
  return String(activity.action || activity.Action || 'activity')
}

function activityPath(activity: AgentActivity) {
  return String(activity.path || activity.Path || '/')
}

function activityMetadata(activity: AgentActivity) {
  return activity.metadata || activity.Metadata || {}
}

function activityCreatedAt(activity: AgentActivity) {
  return String(activity.created_at || activity.CreatedAt || '')
}

export default function AgentAuditStream() {
  const { tx } = useI18n()
  const [activities, setActivities] = useState<AgentActivity[]>([])
  const [loading, setLoading] = useState(true)
  const [available, setAvailable] = useState(true)

  const loadActivities = useCallback(async () => {
    try {
      const data = await api.getDashboardActivities()
      setActivities(Array.isArray(data) ? data : [])
      setAvailable(true)
    } catch {
      setAvailable(false)
      setActivities([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadActivities()
    const timer = setInterval(() => {
      void loadActivities()
    }, 15000)
    return () => clearInterval(timer)
  }, [loadActivities])

  const formatTime = (timeStr: string) => {
    if (!timeStr) return '-'
    const date = new Date(timeStr)
    if (Number.isNaN(date.getTime())) return '-'
    return date.toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
  }

  return (
    <section className="agent-audit-panel" aria-busy={loading}>
      <div className="dashboard-section-head compact">
        <div>
          <h3>{tx('Agent 活动记录', 'Agent Activity')}</h3>
          <p>{tx('显示最近的外部读取、写入和安全审查记录。', 'Recent external reads, writes, and safety checks.')}</p>
        </div>
        <span className={available ? 'agent-audit-status is-ready' : 'agent-audit-status'}>
          {available ? tx('可用', 'Available') : tx('暂无数据', 'No data')}
        </span>
      </div>

      {loading && activities.length === 0 ? (
        <div className="agent-audit-empty">{tx('正在读取活动记录...', 'Loading activity...')}</div>
      ) : !available ? (
        <div className="agent-audit-empty">
          {tx('当前没有可用的活动记录。', 'Activity history is not available right now.')}
        </div>
      ) : activities.length === 0 ? (
        <div className="agent-audit-empty">
          {tx('还没有外部 Agent 访问记录。', 'No external agent activity yet.')}
        </div>
      ) : (
        <div className="agent-audit-list">
          {activities.map((activity, index) => {
            const meta = activityMetadata(activity)
            const risk = String(meta.risk_level || 'LOW').toUpperCase()
            const accessed = String(meta.accessed_resource || activityPath(activity))
            const agent = String(meta.agent_name || tx('未知 Agent', 'Unknown agent'))
            const tone = risk === 'BLOCKED' ? 'bad' : risk === 'HIGH' ? 'warn' : 'ok'

            return (
              <article className="agent-audit-item" key={activityID(activity, index)}>
                <div>
                  <strong>{agent}</strong>
                  <code>{activityAction(activity)} {accessed === '/' ? '/' : `/${accessed.replace(/^\/+/, '')}`}</code>
                </div>
                <div>
                  <span className={`codex-console-pill tone-${tone}`}>{risk}</span>
                  <small>{formatTime(activityCreatedAt(activity))}</small>
                </div>
              </article>
            )
          })}
        </div>
      )}
    </section>
  )
}
