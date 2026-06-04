import { Link, useLocation } from 'react-router-dom'
import { useI18n } from '../i18n'
import { billingReasonMessage } from './BillingShared'

export default function BillingPage() {
  const { tx } = useI18n()
  const location = useLocation()
  const reason = new URLSearchParams(location.search).get('reason')
  const bannerMessage = billingReasonMessage(reason, tx)

  return (
    <div className="page app-narrow-page">
      <section className="plan-gate">
        <div className="plan-gate-head">
          <p className="materials-kicker">{tx('账户与容量', 'Account and storage')}</p>
          <h2>{tx('当前部署暂不展示价格和订阅入口。', 'Pricing and subscription entry points are hidden in this deployment.')}</h2>
          <p>
            {tx(
              '你可以继续使用已启用的数据管理、接入、导入导出和备份功能。存储限制与账号策略以当前部署配置为准。',
              'You can continue using enabled data management, setup, import, export, and backup features. Storage limits and account policy follow this deployment.',
            )}
          </p>
          {bannerMessage && <div className="alert alert-warn">{bannerMessage}</div>}
        </div>

        <div className="plan-gate-grid">
          <article className="plan-option-card featured">
            <h3>{tx('继续配置 Vola', 'Continue configuring Vola')}</h3>
            <p>{tx('连接 Claude、ChatGPT、Cursor、Windsurf 或命令行工具。', 'Connect Claude, ChatGPT, Cursor, Windsurf, or command-line tools.')}</p>
            <ul>
              <li>{tx('账号登录与个人资料', 'Account sign-in and profile')}</li>
              <li>{tx('Memory、项目、会话数据', 'Memory, projects, and conversations')}</li>
              <li>{tx('Skills Library 和 Team Library', 'Skills Library and Team Library')}</li>
              <li>{tx('导入、同步和备份', 'Import, sync, and backup')}</li>
            </ul>
            <Link className="btn btn-primary btn-block" to="/onboarding">
              {tx('进入接入向导', 'Open setup guide')}
            </Link>
          </article>
        </div>
      </section>
    </div>
  )
}
