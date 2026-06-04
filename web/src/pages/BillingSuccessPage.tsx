import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import { useI18n } from '../i18n'

export default function BillingSuccessPage() {
  const { tx } = useI18n()

  useEffect(() => {
    localStorage.removeItem('vola.postSignupIntent')
  }, [])

  return (
    <div className="page materials-page">
      <div className="billing-success-card">
        <div className="status-banner">
          <span className="status-icon status-ok">&#10003;</span>
          <span className="status-text">{tx('已返回 Vola', 'Returned to Vola')}</span>
        </div>

        <h2>{tx('当前部署暂不展示价格和订阅入口。', 'Pricing and subscription entry points are hidden in this deployment.')}</h2>
        <p className="page-subtitle">
          {tx(
            '你可以继续完成 AI 工具接入、资料导入和备份设置。',
            'You can continue with AI tool setup, data import, and backup configuration.',
          )}
        </p>

        <div className="billing-actions">
          <Link to="/onboarding" className="btn btn-primary">
            {tx('继续接入向导', 'Continue onboarding')}
          </Link>
          <Link to="/settings/profile" className="btn">
            {tx('查看个人资料', 'Open profile')}
          </Link>
        </div>
      </div>
    </div>
  )
}
