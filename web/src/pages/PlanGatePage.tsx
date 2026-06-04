import { Link, useNavigate } from 'react-router-dom'
import { useI18n } from '../i18n'

export default function PlanGatePage() {
  const { tx } = useI18n()
  const navigate = useNavigate()
  const continueFree = () => {
    localStorage.removeItem('vola.postSignupIntent')
    localStorage.setItem('vola.planGateSeen', '1')
    navigate('/onboarding', { replace: true })
  }

  return (
    <div className="page app-narrow-page">
      <section className="plan-gate">
        <div className="plan-gate-head">
          <p className="materials-kicker">{tx('接入准备', 'Setup readiness')}</p>
          <h2>{tx('当前部署暂不展示价格。', 'Pricing is hidden in this deployment.')}</h2>
          <p>
            {tx(
              '你可以先完成账号、MCP 接入、数据导入和备份设置。存储限制和账号策略以当前部署配置为准。',
              'You can continue with account setup, MCP connection, imports, and backup settings. Storage limits and account policy follow this deployment.',
            )}
          </p>
          <div className="alert alert-warn">
            {tx('价格与订阅入口已暂时关闭；页面会直接引导你进入接入向导。', 'Pricing and subscription entry points are currently disabled; continue to the setup guide.')}
          </div>
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
            <button className="btn btn-primary btn-block" type="button" onClick={continueFree}>
              {tx('进入接入向导', 'Open setup guide')}
            </button>
          </article>
        </div>
        <p className="login-note"><Link to="/">{tx('稍后再选', 'Decide later')}</Link></p>
      </section>
    </div>
  )
}
