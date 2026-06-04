import { useEffect, useState } from 'react'
import { api } from '../api'
import { useI18n } from '../i18n'

export default function AgentAuditStream() {
  const { tx } = useI18n()
  const [activities, setActivities] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  const loadActivities = async () => {
    try {
      // We will define this API in api.ts as well.
      const data = await api.getDashboardActivities()
      setActivities(data || [])
    } catch (err) {
      console.warn('Failed to load agent activities', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void loadActivities()
    const timer = setInterval(() => {
      void loadActivities()
    }, 4500) // Poll every 4.5 seconds for snappy updates
    return () => clearInterval(timer)
  }, [])

  const getRiskStyles = (risk: string) => {
    switch (String(risk).toUpperCase()) {
      case 'BLOCKED':
        return {
          bg: 'rgba(239, 68, 68, 0.08)',
          border: 'rgba(239, 68, 68, 0.2)',
          color: '#ef4444',
          label: tx('已阻断', 'Blocked'),
        }
      case 'HIGH':
        return {
          bg: 'rgba(245, 158, 11, 0.08)',
          border: 'rgba(245, 158, 11, 0.2)',
          color: '#d97706',
          label: tx('高危警告', 'High Risk'),
        }
      default:
        return {
          bg: 'rgba(99, 102, 241, 0.06)',
          border: 'rgba(99, 102, 241, 0.1)',
          color: '#6366f1',
          label: tx('只读安全', 'Safe Read'),
        }
    }
  }

  const formatTime = (timeStr: string) => {
    try {
      const date = new Date(timeStr)
      return date.toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      })
    } catch {
      return '-'
    }
  }

  return (
    <div className="dashboard-summary-panel" style={{ marginTop: '24px', padding: '20px', background: '#fff', borderRadius: '18px', border: '1px solid rgba(65, 77, 136, 0.08)', boxShadow: '0 10px 30px rgba(65, 77, 136, 0.02)' }}>
      <div className="dashboard-section-head compact" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px', borderBottom: '1px solid rgba(65, 77, 136, 0.06)', paddingBottom: '10px' }}>
        <h3 style={{ margin: 0, fontSize: '14.5px', fontWeight: 800, color: '#12192d', display: 'flex', alignItems: 'center', gap: '6px' }}>
          <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" strokeWidth="2.5" fill="none" strokeLinecap="round" strokeLinejoin="round" style={{ color: '#6366f1' }}>
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          {tx('Agent 行为审计防火墙', 'Agent Activity Firewall')}
        </h3>
        <span style={{
          width: '8px',
          height: '8px',
          borderRadius: '50%',
          background: '#10b981',
          display: 'inline-block',
          boxShadow: '0 0 8px #10b981',
          animation: 'pulse 2s infinite',
        }} />
      </div>

      {loading && activities.length === 0 ? (
        <div style={{ padding: '24px 0', textAlign: 'center', fontSize: '12px', color: '#506074' }}>
          {tx('正在建立安全监听...', 'Establishing secure listener...')}
        </div>
      ) : activities.length === 0 ? (
        <div style={{ padding: '36px 12px', textAlign: 'center', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '10px' }}>
          <span style={{ fontSize: '24px' }}>🛡️</span>
          <div style={{ fontSize: '13px', fontWeight: 600, color: '#12192d' }}>{tx('全天候主动防御中', 'Active Defense Active')}</div>
          <div style={{ fontSize: '11.5px', color: '#506074', lineHeight: '1.5', maxWidth: '180px', margin: '0 auto' }}>
            {tx('暂无 Agent 外部请求。当有 AI 工具尝试读取或写入数据时，安全日志流会实时在这里展示。', 'No external agent requests yet. Realtime safety logs will appear here.')}
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '10px', maxHeight: '380px', overflowY: 'auto', paddingRight: '4px' }}>
          {activities.map((act) => {
            const meta = act.Metadata || {}
            const risk = String(meta.risk_level || 'LOW')
            const styles = getRiskStyles(risk)
            const accessed = String(meta.accessed_resource || act.Path || '')
            const agent = String(meta.agent_name || 'Unknown Agent')

            return (
              <div
                key={act.ID}
                style={{
                  padding: '12px',
                  background: '#f8fafc',
                  border: '1px solid rgba(65, 77, 136, 0.05)',
                  borderRadius: '12px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '6px',
                  transition: 'all 0.2s',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }}>
                  <span style={{ fontSize: '12.5px', fontWeight: 700, color: '#12192d', textOverflow: 'ellipsis', overflow: 'hidden', whiteSpace: 'nowrap', maxWidth: '110px' }}>
                    {agent}
                  </span>
                  <span style={{
                    fontSize: '10px',
                    fontWeight: 700,
                    padding: '2px 6px',
                    borderRadius: '999px',
                    background: styles.bg,
                    border: `1px solid ${styles.border}`,
                    color: styles.color,
                  }}>
                    {styles.label}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '6px' }}>
                  <code style={{
                    fontSize: '11px',
                    color: '#506074',
                    background: 'rgba(65, 77, 136, 0.04)',
                    padding: '2px 6px',
                    borderRadius: '4px',
                    wordBreak: 'break-all',
                    maxWidth: '150px',
                    textOverflow: 'ellipsis',
                    overflow: 'hidden',
                    whiteSpace: 'nowrap',
                  }}>
                    {act.Action} {accessed === '/' ? '/' : `/${accessed}`}
                  </code>
                  <span style={{ fontSize: '10px', color: '#94a3b8', flexShrink: 0 }}>
                    {formatTime(act.CreatedAt)}
                  </span>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
