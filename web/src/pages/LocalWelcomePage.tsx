import { useState, type FormEvent } from 'react'
import { api } from '../api'
import { useI18n } from '../i18n'

const PRODUCT_NAME = 'Vola'

export default function LocalWelcomePage() {
  const { tx } = useI18n()
  const [showCloudLogin, setShowCloudLogin] = useState(false)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Handle Offline / Local Mode Bootstrap
  const handleStartLocal = async () => {
    if (loading) return
    setLoading(true)
    setError('')
    try {
      const res = await api.bootstrapLocalOwnerToken()
      if (res && res.token) {
        localStorage.setItem('token', res.token)
        localStorage.removeItem('refresh_token')
        // Reload to let checkAuth in App.tsx fetch user info
        window.location.replace('/')
      } else {
        setError(tx('获取本地访问 Token 失败，请检查 Vola 后台引擎状态。', 'Failed to bootstrap local token. Please check background engine status.'))
        setLoading(false)
      }
    } catch (err: any) {
      console.error('Bootstrap local owner failed:', err)
      setError(err?.message || tx('连接本地服务出错，请确保 Vola 守护进程正在运行。', 'Error connecting to local service. Ensure the Vola daemon is running.'))
      setLoading(false)
    }
  }

  // Handle Cloud Account Login (for multi-device sync or team collab)
  const handleCloudLogin = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (loading) return
    setLoading(true)
    setError('')
    try {
      const resp = await api.login({ email, password })
      localStorage.setItem('token', resp.access_token)
      localStorage.setItem('refresh_token', resp.refresh_token)
      window.location.replace('/')
    } catch (err: any) {
      setError(err?.message || tx('云端账户登录失败，请检查您的邮箱和密码。', 'Cloud login failed. Please verify your email and password.'))
      setLoading(false)
    }
  }

  return (
    <div className="local-welcome-wrapper">
      <div className="local-welcome-card">
        {/* Logo / Lockup */}
        <div className="local-welcome-header">
          <img className="local-welcome-logo" src="/vola-mark.svg" alt="" aria-hidden="true" />
          <h2>{PRODUCT_NAME}</h2>
          <span className="badge-desktop">{tx('本地单机版', 'Local Edition')}</span>
        </div>

        {error && <div className="form-error-banner">{error}</div>}

        {!showCloudLogin ? (
          /* Normal State: Offline / Local声明 */
          <div className="local-welcome-main-view">
            <h3 className="local-welcome-title">
              {tx('欢迎使用本地工作台', 'Welcome to your Local Workspace')}
            </h3>
            <p className="local-welcome-desc">
              {tx(
                'Vola 已在您的计算机上安全启动。您的所有数据、文件、AI 技能和上下文均保存在本地，100% 隐私且无须联网即可使用。',
                'Vola is running securely on your machine. All your files, AI skills, and context are stored locally, 100% private, and available completely offline.'
              )}
            </p>
            <div className="local-welcome-sync-note">
              <span>{tx('Codex / Claude Code 可自动同步团队 Skill 和 MCP。', 'Codex / Claude Code can auto-sync team Skills and MCP.')}</span>
              <span>{tx('Cursor / Gemini CLI 使用导出包或手工整理。', 'Cursor / Gemini CLI use exports or manual setup.')}</span>
            </div>

            <button
              className="btn btn-primary btn-block btn-lg btn-glow"
              type="button"
              disabled={loading}
              onClick={handleStartLocal}
            >
              {loading ? tx('连接本地引擎中...', 'Connecting...') : tx('一键进入本地单机版', 'Start Offline Workspace')}
            </button>

            <div className="local-welcome-divider">
              <span>{tx('或者', 'OR')}</span>
            </div>

            <button
              className="btn btn-outline btn-block"
              type="button"
              onClick={() => setShowCloudLogin(true)}
            >
              {tx('登录云端账号进行多端同步', 'Sign In to Enable Cloud Sync')}
            </button>
          </div>
        ) : (
          /* Cloud Login Form State */
          <div className="local-welcome-cloud-view">
            <h3 className="local-welcome-title">
              {tx('登录 Vola 官方云服务', 'Sign In to Vola Cloud')}
            </h3>
            <p className="local-welcome-desc">
              {tx(
                '使用您的官方云账号登录，将您本地的配置、技能和文件同步至其他设备，或与团队成员进行实时协作。',
                'Sign in with your official account to sync your local configs, skills, and files across devices or collaborate with your team.'
              )}
            </p>

            <form onSubmit={handleCloudLogin}>
              <div className="form-group">
                <label className="form-label">{tx('邮箱地址', 'Email Address')}</label>
                <input
                  className="form-control"
                  type="email"
                  placeholder="name@example.com"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>

              <div className="form-group">
                <label className="form-label">{tx('登录密码', 'Password')}</label>
                <input
                  className="form-control"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>

              <div className="form-actions-stack">
                <button className="btn btn-primary btn-block" type="submit" disabled={loading}>
                  {loading ? tx('登录中...', 'Signing in...') : tx('安全登录并同步', 'Sign In & Sync')}
                </button>
                <button
                  className="btn btn-secondary btn-block"
                  type="button"
                  onClick={() => setShowCloudLogin(false)}
                >
                  {tx('返回本地模式', 'Back to Offline Mode')}
                </button>
              </div>
            </form>
          </div>
        )}

        <div className="local-welcome-footer">
          <span>{tx('本地节点：', 'Local Node: ')}<code>127.0.0.1:42690</code></span>
        </div>
      </div>
    </div>
  )
}
