import { FormEvent, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, type AuthProvider } from '../api'
import { useI18n } from '../i18n'
import { PublicShell } from './PublicPages'

export default function LoginPage() {
  const { tx } = useI18n()
  const navigate = useNavigate()
  const [providers, setProviders] = useState<AuthProvider[]>([])
  const [error, setError] = useState('')
  const [loadingAction, setLoadingAction] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  useEffect(() => {
    document.title = tx('登录 — neuDrive', 'Log in — neuDrive')
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

  const redirectTarget = () => {
    const params = new URLSearchParams(window.location.search)
    return sanitizeLoginRedirect(params.get('redirect'))
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

  const handleEmailLogin = async (event: FormEvent) => {
    event.preventDefault()
    if (busy) return
    setLoadingAction('email')
    setError('')
    try {
      const response = await api.login({ email, password })
      localStorage.setItem('token', response.access_token)
      localStorage.setItem('refresh_token', response.refresh_token)
      window.location.assign(redirectTarget())
    } catch (err: any) {
      setError(err?.message || tx('登录失败', 'Login failed'))
      setLoadingAction('')
    }
  }

  return (
    <PublicShell>
      <main className="auth-split">
        <section className="auth-copy">
          <p className="public-kicker">{tx('欢迎回来', 'Welcome back')}</p>
          <h1>{tx('回到你的 AI 记忆、文件和技能层。', 'Return to your AI memory, files, and skills layer.')}</h1>
          <p>{tx('登录后继续管理 neuDrive 里的数据、接入方式和开发者访问。', 'Sign in to manage your neuDrive data, integrations, and developer access.')}</p>
        </section>
        <section className="auth-card">
          <h1 className="login-title">{tx('登录 neuDrive', 'Log in to neuDrive')}</h1>
          <p className="login-desc">{tx('使用已有账号进入产品。', 'Use your existing account to enter the product.')}</p>
          {error && <div className="alert alert-warn">{error}</div>}

          <div className="login-actions">
            <button
              type="button"
              className="btn btn-primary btn-block"
              onClick={() => { void handleProviderAction(githubProvider, 'github') }}
              disabled={busy || !githubEnabled}
            >
              {loadingAction === 'github' ? tx('跳转中...', 'Redirecting...') : tx('使用 GitHub 登录', 'Continue with GitHub')}
            </button>
            <button
              type="button"
              className="btn btn-outline btn-block"
              onClick={() => { void handleProviderAction(pocketProvider, 'pocket') }}
              disabled={busy || !pocketEnabled}
            >
              {loadingAction === 'pocket' ? tx('跳转中...', 'Redirecting...') : tx('使用 Pocket ID 登录', 'Continue with Pocket ID')}
            </button>
          </div>

          <div className="auth-divider"><span>{tx('或使用邮箱', 'or use email')}</span></div>

          <form className="login-form" onSubmit={handleEmailLogin}>
            <input className="input" type="email" required placeholder="you@example.com" value={email} onChange={(event) => setEmail(event.target.value)} />
            <input className="input" type="password" required placeholder={tx('密码', 'Password')} value={password} onChange={(event) => setPassword(event.target.value)} />
            <button className="btn btn-outline btn-block" disabled={busy}>
              {loadingAction === 'email' ? tx('登录中...', 'Signing in...') : tx('登录', 'Log in')}
            </button>
          </form>

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
  if (pathname === '/logo-mark.png' || pathname === '/logo-social.png') return true
  return pathname === '/robots.txt' || pathname === '/sitemap.xml'
}
