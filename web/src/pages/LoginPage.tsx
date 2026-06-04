import { useEffect, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, type AuthProvider } from '../api'
import { useI18n } from '../i18n'
import { PublicShell } from './PublicPages'

const PRODUCT_NAME = 'Vola'

export default function LoginPage() {
  const { tx } = useI18n()
  const [providers, setProviders] = useState<AuthProvider[]>([])
  const [error, setError] = useState('')
  const [loadingAction, setLoadingAction] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  useEffect(() => {
    document.title = tx(`登录 — ${PRODUCT_NAME}`, `Log in — ${PRODUCT_NAME}`)
  }, [tx])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    setError(params.get('error') || '')
    api.getAuthProviders().then((items) => setProviders(items || [])).catch(() => setProviders([]))
  }, [])

  const githubProvider = providers.find((provider) => provider.id === 'github')
  const pocketProvider = providers.find((provider) => provider.kind === 'oidc')
  const githubEnabled = !!githubProvider?.enabled
  const pocketEnabled = !!pocketProvider?.enabled
  const busy = loadingAction !== ''
  const hasProvider = githubEnabled || pocketEnabled

  const redirectTarget = () => {
    const params = new URLSearchParams(window.location.search)
    return sanitizeLoginRedirect(params.get('redirect'))
  }

  const handleEmailLogin = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (busy) return
    setLoadingAction('email')
    setError('')
    try {
      const resp = await api.login({ email, password })
      localStorage.setItem('token', resp.access_token)
      localStorage.setItem('refresh_token', resp.refresh_token)
      window.location.assign(redirectTarget())
    } catch (err: any) {
      setError(err?.message || tx('登录失败，请检查邮箱和密码。', 'Sign-in failed. Check your email and password.'))
      setLoadingAction('')
    }
  }

  const handleProviderAction = async (provider: AuthProvider | undefined, loadingKey: string) => {
    if (!provider?.enabled) return
    setLoadingAction(loadingKey)
    setError('')
    try {
      const resp = await api.startAuthProvider(provider.id, redirectTarget(), 'login')
      window.location.assign(resp.authorization_url)
    } catch (err: any) {
      setError(err?.message || tx('启动登录失败', 'Failed to start sign-in'))
      setLoadingAction('')
    }
  }

  return (
    <PublicShell>
      <main className="auth-split">
        <section className="auth-copy">
          <p className="public-kicker">{tx('欢迎回来', 'Welcome back')}</p>
          <h1>{tx('回到你的 AI 工作资料台。', 'Return to your AI work-data desk.')}</h1>
          <p>{tx(`继续管理 ${PRODUCT_NAME} 里的个人资料、项目上下文、技能库和访问凭证。`, `Continue managing profile data, project context, skills, and access credentials in ${PRODUCT_NAME}.`)}</p>
        </section>
        <section className="auth-card">
          <h1 className="login-title">{tx(`登录 ${PRODUCT_NAME}`, `Log in to ${PRODUCT_NAME}`)}</h1>
          <p className="login-desc">{tx('使用邮箱密码登录；第三方登录会按当前部署配置显示。', 'Use email and password; third-party sign-in appears when enabled by this deployment.')}</p>
          {error && <div className="alert alert-warn">{error}</div>}

          <form className="login-form" onSubmit={handleEmailLogin}>
            <label className="form-field">
              <span>{tx('邮箱', 'Email')}</span>
              <input
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </label>
            <label className="form-field">
              <span>{tx('密码', 'Password')}</span>
              <input
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                minLength={8}
                required
              />
            </label>
            <button type="submit" className="btn btn-primary btn-block" disabled={busy}>
              {loadingAction === 'email' ? tx('登录中...', 'Signing in...') : tx('登录', 'Log in')}
            </button>
          </form>

          {hasProvider && (
            <>
              <div className="auth-divider"><span>{tx('或使用第三方账号', 'Or continue with a provider')}</span></div>
              <div className="login-actions">
                {githubEnabled && (
                  <button
                    type="button"
                    className="btn btn-outline btn-block"
                    onClick={() => { void handleProviderAction(githubProvider, 'github') }}
                    disabled={busy}
                  >
                    {loadingAction === 'github' ? tx('跳转中...', 'Redirecting...') : tx('使用 GitHub 登录', 'Continue with GitHub')}
                  </button>
                )}
                {pocketEnabled && (
                  <button
                    type="button"
                    className="btn btn-outline btn-block"
                    onClick={() => { void handleProviderAction(pocketProvider, 'pocket') }}
                    disabled={busy}
                  >
                    {loadingAction === 'pocket' ? tx('跳转中...', 'Redirecting...') : tx('使用邮箱身份服务登录', 'Continue with email provider')}
                  </button>
                )}
              </div>
            </>
          )}

          <p className="login-note">
            {tx('还没有账户？', 'No account yet?')} <Link to="/signup">{tx('免费创建账号', 'Create free account')}</Link>
          </p>
        </section>
      </main>
    </PublicShell>
  )
}

function sanitizeLoginRedirect(raw: string | null): string {
  const redirect = (raw || '').trim()
  if (!redirect) return '/'
  try {
    const target = redirect.startsWith('/') ? new URL(redirect, window.location.origin) : new URL(redirect)
    if (target.origin !== window.location.origin) return '/'
    if (target.pathname === '/login' || target.pathname === '/signup') return '/'
    if (isStaticAssetPath(target.pathname)) return '/'
    return `${target.pathname}${target.search}${target.hash}`
  } catch {
    return '/'
  }
}

function isStaticAssetPath(pathname: string) {
  if (pathname.startsWith('/assets/')) return true
  if (pathname === '/favicon.ico' || pathname.startsWith('/favicon-') || pathname === '/apple-touch-icon.png') return true
  return pathname === '/robots.txt' || pathname === '/sitemap.xml'
}
