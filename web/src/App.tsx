import { Component, Suspense, lazy, useCallback, useEffect, useState, useMemo, type ErrorInfo, type ReactNode } from 'react'
import { Navigate, NavLink, Outlet, Route, Routes, useLocation, useNavigate, useParams } from 'react-router-dom'
import { api, isTauri, type PublicConfig } from './api'
import LanguageToggle from './components/LanguageToggle'
import { useI18n } from './i18n'

const LoginPage = lazy(() => import('./pages/LoginPage'))
const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const ConnectionsPage = lazy(() => import('./pages/ConnectionsPage'))
const InfoPage = lazy(() => import('./pages/InfoPage'))
const ProjectsPage = lazy(() => import('./pages/ProjectsPage'))
const SetupPage = lazy(() => import('./pages/SetupPage'))
const OAuthAuthorizePage = lazy(() => import('./pages/OAuthAuthorizePage'))
const SetupWebAppsPage = lazy(() => import('./pages/setup/SetupWebAppsPage'))
const SetupCloudPage = lazy(() => import('./pages/setup/SetupCloudPage'))
const SetupLocalPage = lazy(() => import('./pages/setup/SetupLocalPage'))
const SetupAdvancedPage = lazy(() => import('./pages/setup/SetupAdvancedPage'))
const SetupGptActionsPage = lazy(() => import('./pages/setup/SetupGptActionsPage'))
const DataFileEditorPage = lazy(() => import('./pages/data/DataFileEditorPage'))
const DataSkillsPage = lazy(() => import('./pages/data/DataSkillsPage'))
const DataMemoryPage = lazy(() => import('./pages/data/DataMemoryPage'))
const DataConversationsPage = lazy(() => import('./pages/data/DataConversationsPage'))
const GrowthProposalsPage = lazy(() => import('./pages/GrowthProposalsPage'))
const ClaudeImportPage = lazy(() => import('./pages/ClaudeImportPage'))
const SystemSettingsPage = lazy(() => import('./pages/SystemSettingsPage'))
const TeamLibraryPage = lazy(() => import('./pages/TeamLibraryPage'))
const SyncLoginPage = lazy(() => import('./pages/SyncLoginPage'))
const SkillsImportPage = lazy(() => import('./pages/SkillsImportPage'))
const GitMirrorPage = lazy(() => import('./pages/GitMirrorPage'))
const ClaudeMigrationPage = lazy(() => import('./pages/ClaudeMigrationPage'))
const OnboardingPage = lazy(() => import('./pages/OnboardingPage'))
const DeveloperAccessPage = lazy(() => import('./pages/DeveloperAccessPage'))
const CommandLineToolsPage = lazy(() => import('./pages/CommandLineToolsPage'))
const McpHubPage = lazy(() => import('./pages/McpHubPage'))
const CodexConsolePage = lazy(() => import('./pages/CodexConsolePage'))
const LocalWelcomePage = lazy(() => import('./pages/LocalWelcomePage'))
const MarketingHomePage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.MarketingHomePage })))
const IntegrationsPage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.IntegrationsPage })))
const IntegrationDetailPage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.IntegrationDetailPage })))
const DocsLandingPage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.DocsLandingPage })))
const GuidePage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.GuidePage })))
const SignupPage = lazy(() => import('./pages/PublicPages').then((module) => ({ default: module.SignupPage })))
const PrivacyPage = lazy(() => import('./pages/PublicPages').then(({ LegalPage }) => ({ default: () => <LegalPage kind="privacy" /> })))
const TermsPage = lazy(() => import('./pages/PublicPages').then(({ LegalPage }) => ({ default: () => <LegalPage kind="terms" /> })))

const PRODUCT_NAME = 'Vola'
const STALE_CHUNK_RELOAD_KEY = 'vola.staleChunkReloadAt'

type NavIconName =
  | 'home'
  | 'team'
  | 'developer'
  | 'import'
  | 'mcp'
  | 'backup'
  | 'cli'
  | 'growth'
  | 'account'
  | 'settings'
  | 'skills'
  | 'memory'

const navIconPaths: Record<NavIconName, ReactNode> = {
  home: (
    <>
      <path d="M6.2 8.6c0-1.36 2.6-2.46 5.8-2.46s5.8 1.1 5.8 2.46-2.6 2.46-5.8 2.46-5.8-1.1-5.8-2.46Z" />
      <path d="M6.2 8.6v5.3c0 1.36 2.6 2.46 5.8 2.46s5.8-1.1 5.8-2.46V8.6" />
      <path d="M8.4 13.8c1.02.5 2.25.75 3.6.75s2.58-.25 3.6-.75" />
    </>
  ),
  team: (
    <>
      <path d="M8.3 7.3h7.4a2 2 0 0 1 2 2v6.1a2 2 0 0 1-2 2H8.3a2 2 0 0 1-2-2V9.3a2 2 0 0 1 2-2Z" />
      <path d="M9.2 10.4h5.6" />
      <path d="M9.2 13h4.1" />
      <path d="M17.7 10.1 20 8.7v7.3l-2.3-1.4" />
    </>
  ),
  developer: (
    <>
      <path d="M6.1 7h11.8a1.9 1.9 0 0 1 1.9 1.9v7.6a1.9 1.9 0 0 1-1.9 1.9H6.1a1.9 1.9 0 0 1-1.9-1.9V8.9A1.9 1.9 0 0 1 6.1 7Z" />
      <path d="m8.1 10.2 2.1 1.8-2.1 1.8" />
      <path d="M12.3 13.8h3.6" />
    </>
  ),
  import: (
    <>
      <path d="M12 5.2v7.7" />
      <path d="m8.8 10 3.2 3.1 3.2-3.1" />
      <path d="M5.6 15h12.8" />
      <path d="M7.2 6.8h9.6" />
      <path d="M6.2 15v2.1c0 1 .7 1.7 1.8 1.7h8c1.1 0 1.8-.7 1.8-1.7V15" />
    </>
  ),
  mcp: (
    <>
      <path d="M12 7.1v4.7" />
      <path d="m8 14.4 4-2.6 4 2.6" />
      <circle cx="12" cy="5.8" r="2" />
      <circle cx="6.7" cy="15.5" r="2" />
      <circle cx="17.3" cy="15.5" r="2" />
      <path d="M8.7 15.5h6.6" />
    </>
  ),
  backup: (
    <>
      <path d="M7.1 9.2A5.5 5.5 0 0 1 17.8 12h.4a3.05 3.05 0 0 1-.2 6.1H7.4a3.85 3.85 0 0 1-.3-7.7" />
      <path d="m9.4 14.1 2.6-2.5 2.6 2.5" />
      <path d="M12 11.7v5.3" />
    </>
  ),
  cli: (
    <>
      <rect x="4.8" y="6.5" width="14.4" height="11.4" rx="2.3" />
      <path d="m8.1 10.1 2.2 1.9-2.2 1.9" />
      <path d="M12.5 14h3.4" />
      <path d="M4.8 9h14.4" />
    </>
  ),
  growth: (
    <>
      <path d="M5.8 16.7c4.2-.6 7.2-3.3 8-7.4" />
      <path d="M14 9h4.2v4.2" />
      <path d="M6.3 8.5 7 6.4l.7 2.1 2.1.7-2.1.7L7 12l-.7-2.1-2.1-.7 2.1-.7Z" />
      <path d="M16.7 16.5 17.2 15l.5 1.5 1.5.5-1.5.5-.5 1.5-.5-1.5-1.5-.5 1.5-.5Z" />
    </>
  ),
  account: (
    <>
      <circle cx="10" cy="9" r="3" />
      <path d="M5.4 18.2c.8-3 2.3-4.5 4.6-4.5 1.4 0 2.5.5 3.4 1.5" />
      <path d="M15.4 9.2a3.6 3.6 0 0 1 4.4 3.5 3.6 3.6 0 0 1-3.6 3.6h-3.1a2.4 2.4 0 0 1-.2-4.8" />
    </>
  ),
  settings: (
    <>
      <path d="M6.4 8.1h11.2" />
      <path d="M6.4 15.9h11.2" />
      <path d="M6.4 12h11.2" />
      <circle cx="9" cy="8.1" r="1.7" />
      <circle cx="15.2" cy="12" r="1.7" />
      <circle cx="11.4" cy="15.9" r="1.7" />
    </>
  ),
  skills: (
    <>
      <path d="M7.2 5.6h9.6a1.7 1.7 0 0 1 1.7 1.7v9.4a1.7 1.7 0 0 1-1.7 1.7H7.2a1.7 1.7 0 0 1-1.7-1.7V7.3a1.7 1.7 0 0 1 1.7-1.7Z" />
      <path d="M8.8 9.2h6.4" />
      <path d="M8.8 12h4.8" />
      <path d="M8.8 14.8h6.4" />
      <path d="M16.5 5.6v12.8" />
    </>
  ),
  memory: (
    <>
      <path d="M8.1 7.3a3.4 3.4 0 0 1 6.4-1.6 3.3 3.3 0 0 1 3.7 3.3 3.4 3.4 0 0 1-.8 2.2 3.6 3.6 0 0 1-1.7 6.7H8.3a3.6 3.6 0 0 1-1.7-6.7A3.4 3.4 0 0 1 8.1 7.3Z" />
      <path d="M9.4 11.4h5.2" />
      <path d="M10.2 14.1h3.6" />
    </>
  ),
}

function NavIcon({ name }: { name: NavIconName }) {
  return (
    <span className={`nav-icon nav-icon-${name}`} aria-hidden="true">
      <svg viewBox="0 0 24 24" focusable="false">
        {navIconPaths[name]}
      </svg>
    </span>
  )
}

function isLocalOwnerUser(user: any, localMode: boolean) {
  if (!localMode || !user) return false
  return (user.slug === 'local' || user.display_name === 'Local Owner') && !user.email
}

function userDisplayName(user: any, localMode: boolean, tx: (zh: string, en: string) => string) {
  if (!user && localMode) return tx('本地用户', 'Local User')
  if (isLocalOwnerUser(user, localMode)) return tx('本地用户', 'Local User')
  return String(user?.display_name || user?.name || user?.email || user?.slug || tx('用户', 'User'))
}

function userDisplayHint(user: any, localMode: boolean, tx: (zh: string, en: string) => string) {
  if (!user && localMode) return tx('本地单机模式', 'Local-only mode')
  if (isLocalOwnerUser(user, localMode)) return tx('本地单机模式', 'Local-only mode')
  if (user?.email) return String(user.email)
  return localMode ? tx('云端账户', 'Cloud account') : tx('账户菜单', 'Account menu')
}

function isChunkLoadError(error: unknown) {
  const message = error instanceof Error ? error.message : String(error || '')
  return /Failed to fetch dynamically imported module|Importing a module script failed|Loading chunk|vite:preloadError/i.test(message)
}

function reloadForFreshAssets() {
  if (typeof window === 'undefined') return false
  const now = Date.now()
  const lastReload = Number(window.sessionStorage.getItem(STALE_CHUNK_RELOAD_KEY) || '0')
  if (Number.isFinite(lastReload) && now - lastReload < 60_000) return false
  window.sessionStorage.setItem(STALE_CHUNK_RELOAD_KEY, String(now))
  const nextURL = new URL(window.location.href)
  nextURL.searchParams.set('__nd_reload', String(now))
  window.location.replace(nextURL.toString())
  return true
}

if (typeof window !== 'undefined') {
  window.addEventListener('vite:preloadError', (event) => {
    event.preventDefault()
    reloadForFreshAssets()
  })
  window.addEventListener('unhandledrejection', (event) => {
    if (isChunkLoadError(event.reason) && reloadForFreshAssets()) {
      event.preventDefault()
    }
  })
}

class RouteErrorBoundary extends Component<{ children: ReactNode; fallback: ReactNode }, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: unknown, _errorInfo: ErrorInfo) {
    if (isChunkLoadError(error)) reloadForFreshAssets()
  }

  render() {
    if (this.state.failed) return this.props.fallback
    return this.props.children
  }
}

function LegacyOnboardingRedirect() {
  const { platform } = useParams()
  if (platform === 'codex' || platform === 'gemini' || platform === 'claude-code') return <Navigate to="/setup/cli" replace />
  if (platform === 'browser') return <Navigate to="/setup/web-apps" replace />
  return <Navigate to="/setup/mcp" replace />
}

function LegacyDataFilesRedirect() {
  const location = useLocation()
  const wildcard = useParams()['*']
  const params = new URLSearchParams(location.search)
  params.delete('access')
  if (wildcard) {
    const decoded = decodeURIComponent(wildcard).replace(/^\/+/, '')
    if (decoded) {
      params.set('path', `/${decoded}`)
      params.delete('type')
      params.delete('source')
      params.delete('q')
    }
  }
  const search = params.toString()
  return <Navigate to={`/${search ? `?${search}` : ''}`} replace />
}

function App() {
  const [user, setUser] = useState<any>(null)
  const [connectionCount, setConnectionCount] = useState<number | null>(null)
  const [publicConfig, setPublicConfig] = useState<PublicConfig>({})
  const [loading, setLoading] = useState(true)
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const [localConfig, setLocalConfig] = useState<any>(null)
  const [teams, setTeams] = useState<any[]>([])
  const [workspaceModalOpen, setWorkspaceModalOpen] = useState(false)
  const [selectedTeamID, setSelectedTeamID] = useState<string>('')
  const [desktopStartupError, setDesktopStartupError] = useState(false)
  const { tx } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const systemSettingsEnabled = !!publicConfig?.system_settings_enabled
  const localMode = !!publicConfig?.local_mode
  const desktopConsoleHome = isTauri && localMode
  const publicRegistrationEnabled = publicConfig?.public_registration_enabled !== false
  const currentUser = user || null
  const currentUserName = userDisplayName(currentUser, localMode, tx)

  const fetchWorkspaceInfo = useCallback(async () => {
    if (!localMode || !currentUser) return
    try {
      const cfg = await api.getLocalConfig()
      setLocalConfig(cfg)
      const list = await api.getTeams()
      setTeams(list)
    } catch (err) {
      console.error('Failed to load workspace info:', err)
    }
  }, [localMode, currentUser])

  useEffect(() => {
    void fetchWorkspaceInfo()
  }, [fetchWorkspaceInfo])

  const currentActiveTeam = useMemo(() => {
    if (!localConfig) return null
    try {
      const parsed = JSON.parse(localConfig.raw)
      const activeTeamID = parsed.profiles?.[parsed.current_profile || 'default']?.active_team_id || ''
      if (!activeTeamID) return null
      return teams.find((t: any) => t.id === activeTeamID) || null
    } catch {
      return null
    }
  }, [localConfig, teams])

  const currentUserHint = useMemo(() => {
    if (localMode && currentActiveTeam) {
      const roleStr = currentActiveTeam.role === 'owner' ? tx('所有者', 'Owner') :
                      currentActiveTeam.role === 'admin' ? tx('管理员', 'Admin') :
                      currentActiveTeam.role === 'member' ? tx('成员', 'Member') :
                      tx('观察员', 'Viewer')
      return `${tx('团队：', 'Team: ')}${currentActiveTeam.name} (${roleStr})`
    }
    return userDisplayHint(currentUser, localMode, tx)
  }, [currentUser, localMode, currentActiveTeam, tx])

  useEffect(() => {
    if (workspaceModalOpen) {
      const currentActiveTeamID = currentActiveTeam?.id || ''
      setSelectedTeamID(currentActiveTeamID)
    }
  }, [workspaceModalOpen, currentActiveTeam])

  const handleConfirmWorkspaceSwitch = async () => {
    try {
      await api.updateLocalActiveWorkspace({ active_team_id: selectedTeamID })
      setWorkspaceModalOpen(false)
      window.location.reload()
    } catch (err) {
      console.error('Failed to switch workspace:', err)
    }
  }

  const currentUserInitial = currentUserName.slice(0, 1).toUpperCase()
  const importsHomePath = localMode ? '/imports/local-apps' : '/imports/claude-export'
  const homePath = desktopConsoleHome ? '/codex-console' : '/'

  const checkAuth = useCallback(async () => {
    const clearAuthParamsFromURL = () => {
      const nextURL = new URL(window.location.href)
      nextURL.searchParams.delete('auth_token')
      nextURL.searchParams.delete('auth_refresh')
      nextURL.searchParams.delete('local_token')
      const next = `${nextURL.pathname}${nextURL.search}${nextURL.hash}`
      window.history.replaceState({}, '', next || nextURL.pathname)
    }

    const params = new URLSearchParams(window.location.search)
    const authToken = params.get('auth_token')
    const authRefresh = params.get('auth_refresh')
    const localToken = params.get('local_token')
    setDesktopStartupError(false)
    if (authToken) {
      localStorage.setItem('token', authToken)
      if (authRefresh) localStorage.setItem('refresh_token', authRefresh)
      else localStorage.removeItem('refresh_token')
      clearAuthParamsFromURL()
    }
    if (!authToken && localToken && !localStorage.getItem('token')) {
      localStorage.setItem('token', localToken)
      localStorage.removeItem('refresh_token')
      clearAuthParamsFromURL()
    } else if (localToken) {
      clearAuthParamsFromURL()
    }

    let cfg: PublicConfig = {}
    let retries = 0
    const maxRetries = 20 // Up to 10 seconds total wait time
    while (true) {
      try {
        cfg = await api.getPublicConfig()
        setPublicConfig(cfg || {})
        break
      } catch (err) {
        console.error("Failed to fetch public config:", err)
        if (isTauri && retries < maxRetries) {
          retries++
          await new Promise((resolve) => setTimeout(resolve, 500))
          continue
        }
        if (isTauri) setDesktopStartupError(true)
        setPublicConfig({})
        break
      }
    }

    const bootstrapLocalOwner = async (): Promise<string | null> => {
      if (!cfg?.local_mode) return null
      try {
        const created = await api.bootstrapLocalOwnerToken()
        if (!created?.token) return null
        localStorage.setItem('token', created.token)
        localStorage.removeItem('refresh_token')
        return created.token
      } catch {
        return null
      }
    }

    const loadUser = async () => {
      const me = await api.getMe()
      setUser(me)
      return me
    }

    let token = localStorage.getItem('token')
    if (!token && localToken) {
      localStorage.setItem('token', localToken)
      localStorage.removeItem('refresh_token')
      token = localToken
    }
    if (!token) token = await bootstrapLocalOwner() || ''
    if (!token) {
      setUser(null)
      setLoading(false)
      return
    }

    try {
      await loadUser()
    } catch {
      localStorage.removeItem('token')
      localStorage.removeItem('refresh_token')
      let nextToken = ''
      if (localToken && localToken !== token) {
        localStorage.setItem('token', localToken)
        nextToken = localToken
      } else {
        nextToken = await bootstrapLocalOwner() || ''
      }
      if (!nextToken) {
        setUser(null)
        setLoading(false)
        return
      }
      try {
        await loadUser()
      } catch {
        localStorage.removeItem('token')
        localStorage.removeItem('refresh_token')
        setUser(null)
      }
    }
    setLoading(false)
  }, [])

  useEffect(() => {
    void checkAuth()
  }, [checkAuth])

  useEffect(() => {
    const currentURL = new URL(window.location.href)
    if (currentURL.searchParams.has('__nd_reload')) {
      currentURL.searchParams.delete('__nd_reload')
      window.history.replaceState({}, '', `${currentURL.pathname}${currentURL.search}${currentURL.hash}`)
    }
    const timer = window.setTimeout(() => {
      window.sessionStorage.removeItem(STALE_CHUNK_RELOAD_KEY)
    }, 60_000)
    return () => window.clearTimeout(timer)
  }, [])

  useEffect(() => {
    if (!user) return
    const isSignupReturn = localStorage.getItem('vola.postSignupIntent') === '1'
    if (!isSignupReturn) return
    if (location.pathname.startsWith('/onboarding')) return
    navigate('/onboarding', { replace: true })
  }, [location.pathname, navigate, user])

  useEffect(() => {
    if (!currentUser) return
    const path = location.pathname
    const pageParams = new URLSearchParams(location.search)
    const isProjectDocsPage = path.startsWith('/codex-console') && pageParams.get('view') === 'context_index'
    const pageTitle =
      path === '/' ? 'Home' :
      path.startsWith('/plan') ? 'Get Started' :
      path.startsWith('/onboarding') ? 'Get Started' :
      path.startsWith('/connections') ? 'Connections' :
      path.startsWith('/sync-backup') || path.startsWith('/git-mirror') ? 'GitHub Backup' :
      path.startsWith('/settings/billing') || path.startsWith('/billing') ? 'Account' :
      path.startsWith('/settings/developer-access') || path.startsWith('/settings/developer') ? 'Developer Access' :
      path.startsWith('/settings/security') ? 'System Settings' :
      path.startsWith('/settings/profile') || path.startsWith('/info') ? 'My Profile' :
      path.startsWith('/team') ? 'Team AI Library' :
      path.startsWith('/cli') ? 'Command Line Tools' :
      isProjectDocsPage ? 'Project Docs' :
      path.startsWith('/codex-console') ? 'Codex Console' :
      path.startsWith('/setup') ? 'Setup Guide' :
      path.startsWith('/data/conversations') ? 'Conversations' :
      path.startsWith('/data/projects') || path.startsWith('/projects') ? 'Projects' :
      path.startsWith('/data') ? 'Data Explorer' :
      path.startsWith('/memory') ? 'Memory' :
      path.startsWith('/growth-proposals') ? 'Growth Proposals' :
      path.startsWith('/skills') ? 'Skills' :
      path.startsWith('/imports/local-apps') || path.startsWith('/imports/claude') || path.startsWith('/imports/codex') ? 'Local App Data Import' :
      path.startsWith('/imports') ? 'Data Imports' :
      PRODUCT_NAME
    document.title = pageTitle === PRODUCT_NAME ? PRODUCT_NAME : `${pageTitle} — ${PRODUCT_NAME}`
  }, [location.pathname, location.search, currentUser])

  useEffect(() => {
    if (!currentUser) {
      setConnectionCount(null)
      return
    }
    let active = true
    const fetchConnections = async () => {
      try {
        const conns = await api.getConnections()
        if (active) setConnectionCount(conns.length)
      } catch {
        if (active) setConnectionCount(0)
      }
    }
    void fetchConnections()
    const timer = setInterval(() => {
      void fetchConnections()
    }, 15000)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [currentUser])

  useEffect(() => {
    setUserMenuOpen(false)
  }, [location.pathname])

  const routeFallback = (
    <div className="loading-screen">
      <div className="loading-spinner" />
      <p>{tx('页面加载中...', 'Loading page...')}</p>
    </div>
  )

  const routeErrorFallback = (
    <div className="loading-screen">
      <p>{tx('页面资源已更新，请刷新后继续。', 'The page assets were updated. Refresh to continue.')}</p>
      <button
        className="btn btn-primary"
        type="button"
        onClick={() => {
          window.sessionStorage.removeItem(STALE_CHUNK_RELOAD_KEY)
          window.location.reload()
        }}
      >
        {tx('刷新页面', 'Refresh page')}
      </button>
    </div>
  )

  const handleLogout = async () => {
    await api.logout()
    if (!localMode) {
      setUser(null)
      navigate('/login')
      return
    }
    try {
      const created = await api.bootstrapLocalOwnerToken()
      if (created?.token) {
        localStorage.setItem('token', created.token)
        localStorage.removeItem('refresh_token')
        setUser(await api.getMe())
      } else {
        setUser(null)
      }
    } catch {
      setUser(null)
    }
    navigate('/', { replace: true })
  }

  if (loading) {
    return (
      <div className="loading-screen">
        <div className="loading-spinner" />
        {isTauri ? (
          <>
            <h2 style={{ fontSize: '18px', fontWeight: 850, color: '#12192d', marginTop: '16px', marginBottom: '4px' }}>{PRODUCT_NAME}</h2>
            <p style={{ fontSize: '13px', color: '#506074' }}>{tx('正在连接本地 Vola 数据引擎...', 'Connecting to local Vola engine...')}</p>
          </>
        ) : (
          <p>{tx('加载中...', 'Loading...')}</p>
        )}
      </div>
    )
  }

  if (location.pathname === '/oauth/authorize') {
    return <RouteErrorBoundary key={location.key} fallback={routeErrorFallback}><Suspense fallback={routeFallback}><OAuthAuthorizePage /></Suspense></RouteErrorBoundary>
  }

  if (location.pathname === '/import/skills') {
    return <RouteErrorBoundary key={location.key} fallback={routeErrorFallback}><Suspense fallback={routeFallback}><SkillsImportPage /></Suspense></RouteErrorBoundary>
  }

  if (location.pathname.startsWith('/guides/') || location.pathname.startsWith('/integrations/') || location.pathname === '/privacy' || location.pathname === '/terms') {
    return (
      <RouteErrorBoundary key={location.key} fallback={routeErrorFallback}>
      <Suspense fallback={routeFallback}>
        <Routes>
          <Route path="/guides/:platform" element={<GuidePage />} />
          <Route path="/integrations/:platform" element={<IntegrationDetailPage />} />
          <Route path="/privacy" element={<PrivacyPage />} />
          <Route path="/terms" element={<TermsPage />} />
        </Routes>
      </Suspense>
      </RouteErrorBoundary>
    )
  }

  if (!user && !localMode && isTauri) {
    return (
      <div className="loading-screen">
        {!desktopStartupError && <div className="loading-spinner" />}
        <h2 style={{ fontSize: '18px', fontWeight: 850, color: '#12192d', marginTop: '16px', marginBottom: '4px' }}>{PRODUCT_NAME}</h2>
        <p style={{ fontSize: '13px', color: '#506074' }}>
          {desktopStartupError
            ? tx('本地数据服务暂时没有响应，请重试或重新打开 Vola。', 'The local data service is not responding. Try again or reopen Vola.')
            : tx('正在准备桌面控制台...', 'Preparing desktop console...')}
        </p>
        {desktopStartupError && (
          <button
            className="btn btn-primary"
            type="button"
            onClick={() => {
              setLoading(true)
              void checkAuth()
            }}
          >
            {tx('重试', 'Retry')}
          </button>
        )}
      </div>
    )
  }

  if (!user && !localMode) {
    const protectedSignupRedirect = `/signup?redirect=${encodeURIComponent(location.pathname + location.search)}`
    const protectedLoginRedirect = `/login?redirect=${encodeURIComponent(location.pathname + location.search)}`
    const signupTarget = publicRegistrationEnabled ? protectedSignupRedirect : protectedLoginRedirect
    return (
      <RouteErrorBoundary key={location.key} fallback={routeErrorFallback}>
      <Suspense fallback={routeFallback}>
        <Routes>
          <Route path="/" element={<MarketingHomePage />} />
          <Route path="/pricing" element={<Navigate to="/" replace />} />
          <Route path="/integrations" element={<IntegrationsPage />} />
          <Route path="/integrations/:platform" element={<IntegrationDetailPage />} />
          <Route path="/docs" element={<DocsLandingPage />} />
          <Route path="/guides/:platform" element={<GuidePage />} />
          <Route path="/privacy" element={<PrivacyPage />} />
          <Route path="/terms" element={<TermsPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/signup" element={<SignupPage publicRegistrationEnabled={publicRegistrationEnabled} />} />
          <Route path="/onboarding/*" element={<Navigate to={signupTarget} replace />} />
          <Route path="/plan" element={<Navigate to={signupTarget} replace />} />
          <Route path="*" element={<Navigate to={protectedLoginRedirect} replace />} />
        </Routes>
      </Suspense>
      </RouteErrorBoundary>
    )
  }

  const showGitHubBackup = localMode || !!publicConfig?.github_enabled || !!publicConfig?.github_app_enabled
  const contextIndexParams = new URLSearchParams(location.search)
  const isContextIndexRoute =
    location.pathname.startsWith('/codex-console') &&
    contextIndexParams.get('view') === 'context_index'
  const isMcpRoute = location.pathname.startsWith('/mcp-hub') || location.pathname.startsWith('/setup/mcp')
  const isMoreNavRoute =
    location.pathname.startsWith('/imports') ||
    location.pathname.startsWith('/memory') ||
    location.pathname.startsWith('/growth-proposals') ||
    location.pathname.startsWith('/sync-backup') ||
    location.pathname.startsWith('/git-mirror') ||
    location.pathname.startsWith('/settings/profile') ||
    location.pathname.startsWith('/settings/developer-access') ||
    location.pathname.startsWith('/settings/developer') ||
    location.pathname.startsWith('/cli') ||
    (location.pathname.startsWith('/codex-console') && !desktopConsoleHome) ||
    location.pathname.startsWith('/settings/security')
  const isSyncLoginRoute = location.pathname === '/sync/login'
  const isLegacySyncLoginRoute =
    location.pathname === '/data/sync' &&
    new URLSearchParams(location.search).get('cli_login') === '1'

  if (isLegacySyncLoginRoute) {
    return <Navigate to={`/sync/login${location.search}`} replace />
  }

  if (isSyncLoginRoute) {
    return <RouteErrorBoundary key={location.key} fallback={routeErrorFallback}><Suspense fallback={routeFallback}><SyncLoginPage systemSettingsEnabled={systemSettingsEnabled} /></Suspense></RouteErrorBoundary>
  }

  return (
    <div className="app-layout">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <div className="sidebar-brand-lockup">
            <img className="sidebar-brand-logo" src="/vola-mark.svg" alt="" aria-hidden="true" />
            <h1>{PRODUCT_NAME}</h1>
          </div>
        </div>

        <nav className="sidebar-nav">
          <div className="sidebar-group-header">{tx('常用', 'Main')}</div>

          <NavLink to={homePath} end className={({ isActive }) => isActive && !isContextIndexRoute ? 'nav-item active' : 'nav-item'}>
            <NavIcon name="home" />
            <span>{desktopConsoleHome ? 'Codex Console' : tx('今日概览', 'Today')}</span>
          </NavLink>

          <NavLink to="/skills" className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'}>
            <NavIcon name="skills" />
            <span>Skill</span>
          </NavLink>

          <NavLink to={localMode ? '/mcp-hub' : '/setup/mcp'} className={() => isMcpRoute ? 'nav-item active' : 'nav-item'}>
            <NavIcon name="mcp" />
            <span>MCP</span>
            {connectionCount !== null && (
              connectionCount > 0 ? (
                <span className="nav-item-badge badge-online" title={tx('AI 已就绪', 'AI Connected')}>
                  {tx('已连', 'Linked')}
                </span>
              ) : (
                <span className="nav-item-badge badge-offline" title={tx('未连接 AI', 'No AI Connected')}>
                  {tx('未连', 'Offline')}
                </span>
              )
            )}
          </NavLink>

          <NavLink to="/team" className={({ isActive }) => isActive ? 'nav-item active' : 'nav-item'}>
            <NavIcon name="team" />
            <span>{tx('团队', 'Team')}</span>
          </NavLink>

          {localMode && (
            <NavLink to="/codex-console?view=context_index" className={() => isContextIndexRoute ? 'nav-item active' : 'nav-item'}>
              <NavIcon name="memory" />
              <span>{tx('项目文档', 'Project Docs')}</span>
            </NavLink>
          )}

          <div className="sidebar-divider" />
          <div className="nav-group">
            <details open={isMoreNavRoute || undefined}>
              <summary className={isMoreNavRoute ? 'nav-item nav-item-button group-active' : 'nav-item nav-item-button'}>
                <NavIcon name="settings" />
                <span>{tx('更多', 'More')}</span>
                <span className="nav-group-caret">›</span>
              </summary>
              <div className="nav-submenu">
                {localMode && (
                  <NavLink to={importsHomePath} className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                    {tx('导入资料', 'Import Data')}
                  </NavLink>
                )}
                <NavLink to="/memory" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                  {tx('记忆', 'Memory')}
                </NavLink>
                <NavLink to="/growth-proposals" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                  {tx('优化建议', 'Suggestions')}
                </NavLink>
                {localMode && (
                  <NavLink to="/settings/profile#cloud-account" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                    {tx('云端账号', 'Cloud Account')}
                  </NavLink>
                )}
                {showGitHubBackup && (
                  <NavLink to="/sync-backup" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                    {tx('GitHub 备份', 'GitHub Backup')}
                  </NavLink>
                )}
                <NavLink to="/cli" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                  {tx('命令行工具', 'Command Line')}
                </NavLink>
                {localMode && !desktopConsoleHome && (
                  <NavLink to="/codex-console" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                    Codex Console
                  </NavLink>
                )}
                {systemSettingsEnabled && (
                  <NavLink to="/settings/security" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                    {tx('系统设置', 'System Settings')}
                  </NavLink>
                )}
                <NavLink to="/settings/developer-access" className={({ isActive }) => isActive ? 'nav-subitem active' : 'nav-subitem'}>
                  {tx('开发者访问', 'Developer Access')}
                </NavLink>
              </div>
            </details>
          </div>
        </nav>

        <div className="sidebar-footer">
          <LanguageToggle compact />
          <div className="sidebar-user-menu-wrap">
            <button
              className="sidebar-user-button"
              type="button"
              aria-haspopup="menu"
              aria-expanded={userMenuOpen}
              onClick={() => setUserMenuOpen((open) => !open)}
            >
              <span className="sidebar-user-avatar">{currentUserInitial}</span>
              <span className="user-info">
                <span className="user-name">{currentUserName}</span>
                <span className="user-menu-hint">{currentUserHint}</span>
              </span>
              <span className="user-menu-chevron">⌄</span>
            </button>
            {userMenuOpen && (
              <div className="sidebar-user-menu" role="menu">
                <NavLink to="/settings/profile" role="menuitem" onClick={() => setUserMenuOpen(false)}>
                  {tx('个人资料', 'Profile')}
                </NavLink>
                {localMode && (
                  <button type="button" role="menuitem" onClick={() => { setWorkspaceModalOpen(true); setUserMenuOpen(false); }}>
                    {tx('切换空间', 'Switch Workspace')}
                  </button>
                )}
                <button type="button" role="menuitem" onClick={() => { void handleLogout() }}>
                  {tx('退出', 'Sign out')}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>

      <main className="main-content">
        <RouteErrorBoundary key={location.pathname} fallback={routeErrorFallback}>
        <Suspense fallback={routeFallback}>
          <Routes>
            <Route path="/" element={desktopConsoleHome ? <Navigate to="/codex-console" replace /> : <DashboardPage systemSettingsEnabled={systemSettingsEnabled} localMode={localMode} />} />
            <Route path="/plan" element={<Navigate to="/onboarding" replace />} />
            <Route path="/onboarding" element={<Navigate to="/setup/mcp" replace />} />
            <Route path="/onboarding/:platform" element={<LegacyOnboardingRedirect />} />
            <Route path="/connections" element={<ConnectionsPage />} />
            <Route path="/sync-backup" element={<GitMirrorPage />} />
            <Route path="/git-mirror" element={<Navigate to="/sync-backup" replace />} />
            <Route path="/billing" element={<Navigate to="/onboarding" replace />} />
            <Route path="/billing/success" element={<Navigate to="/onboarding" replace />} />
            <Route path="/settings" element={<Navigate to="/settings/profile" replace />} />
            <Route path="/settings/profile" element={<InfoPage />} />
            <Route path="/settings/billing" element={<Navigate to="/settings/profile" replace />} />
            <Route path="/settings/developer-access" element={<DeveloperAccessPage />} />
            <Route path="/settings/security" element={systemSettingsEnabled ? <SystemSettingsPage /> : <Navigate to="/settings/profile" replace />} />
            <Route path="/settings/developer" element={<Navigate to="/settings/developer-access" replace />} />
            <Route path="/team" element={<TeamLibraryPage />} />
            <Route path="/cli" element={<CommandLineToolsPage />} />
            <Route path="/codex-console" element={localMode ? <CodexConsolePage /> : <Navigate to="/" replace />} />
            <Route path="/mcp-hub" element={<McpHubPage />} />
            <Route path="/command-line-tools" element={<Navigate to="/cli" replace />} />

            <Route path="/setup/mcp" element={<OnboardingPage />} />
            <Route path="/setup" element={<SetupPage />}>
              <Route index element={<Navigate to="/setup/mcp" replace />} />
              <Route path="web-apps" element={<SetupWebAppsPage />} />
              <Route path="cli" element={<SetupCloudPage />} />
              <Route path="cloud" element={<Navigate to="/setup/cli" replace />} />
              <Route path="adapters" element={<Navigate to="/setup/web-apps" replace />} />
              <Route path="local" element={<SetupLocalPage />} />
              <Route path="advanced" element={<SetupAdvancedPage />} />
              <Route path="gpt-actions" element={<SetupGptActionsPage />} />
              <Route path="tokens" element={<Navigate to="/settings/developer-access" replace />} />
            </Route>
            <Route path="/setup/tokens" element={<Navigate to="/settings/developer-access" replace />} />

            <Route path="/connections/import/claude" element={<ClaudeImportPage />} />
            <Route path="/data" element={<Outlet />}>
              <Route index element={<Navigate to="/" replace />} />
              <Route path="files/edit/*" element={<DataFileEditorPage />} />
              <Route path="files/browse/*" element={<LegacyDataFilesRedirect />} />
              <Route path="files/recent" element={<Navigate to="/" replace />} />
              <Route path="files/*" element={<LegacyDataFilesRedirect />} />
              <Route path="projects" element={<ProjectsPage />} />
              <Route path="projects/:projectName" element={<ProjectsPage />} />
              <Route path="conversations" element={<DataConversationsPage />} />
              <Route path="conversations/*" element={<DataConversationsPage />} />
              <Route path="skills" element={<Navigate to="/skills" replace />} />
              <Route path="skills/:bundleKey" element={<Navigate to="/skills" replace />} />
              <Route path="memory" element={<Navigate to="/memory" replace />} />
              <Route path="profile" element={<Navigate to="/settings/profile" replace />} />
              <Route path="roles" element={<Navigate to="/" replace />} />
              <Route path="inbox" element={<Navigate to="/" replace />} />
              <Route path="settings" element={<Navigate to="/settings/profile" replace />} />
              <Route path="sync" element={<Navigate to="/sync-backup" replace />} />
            </Route>
            <Route path="/memory" element={<DataMemoryPage />} />
            <Route path="/skills" element={<DataSkillsPage />} />
            <Route path="/skills/:bundleKey" element={<DataSkillsPage />} />
            <Route path="/growth-proposals" element={<GrowthProposalsPage />} />

            <Route path="/imports" element={<Navigate to={importsHomePath} replace />} />
            <Route path="/imports/local-apps" element={localMode ? <ClaudeMigrationPage localMode={localMode} officialExportPath="/imports/claude-export" /> : <Navigate to="/" replace />} />
            <Route path="/imports/claude" element={<Navigate to={localMode ? "/imports/local-apps?platform=claude" : "/"} replace />} />
            <Route path="/imports/codex" element={<Navigate to={localMode ? "/imports/local-apps?platform=codex" : "/"} replace />} />
            <Route path="/imports/claude-export" element={<ClaudeImportPage localMode={localMode} />} />

            <Route path="/login" element={<Navigate to="/" replace />} />
            <Route path="/signup" element={<Navigate to="/" replace />} />
            <Route path="/pricing" element={<Navigate to="/" replace />} />
            <Route path="/integrations" element={<Navigate to="/connections" replace />} />
            <Route path="/docs" element={<Navigate to="/onboarding" replace />} />
            <Route path="/info" element={<Navigate to="/settings/profile" replace />} />
            <Route path="/projects" element={<Navigate to="/data/projects" replace />} />
            <Route path="/collaborations" element={<Navigate to="/team" replace />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
        </RouteErrorBoundary>
      </main>
      {workspaceModalOpen && (
        <div className="modal-backdrop" onClick={() => setWorkspaceModalOpen(false)}>
          <div className="modal-container workspace-modal" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3>{tx('切换工作空间', 'Switch Workspace')}</h3>
              <button className="modal-close-btn" onClick={() => setWorkspaceModalOpen(false)}>×</button>
            </div>
            <div className="modal-body">
              <p className="modal-desc">{tx('选择要在本地激活的团队或个人工作空间：', 'Select the team or personal workspace to activate locally:')}</p>
              <div className="workspace-list">
                {/* 个人空间 */}
                <div
                  className={`workspace-item ${!selectedTeamID ? 'active' : ''}`}
                  onClick={() => setSelectedTeamID('')}
                >
                  <div className="workspace-icon personal-icon">👤</div>
                  <div className="workspace-info">
                    <div className="workspace-name">{tx('个人空间', 'Personal Workspace')}</div>
                    <div className="workspace-role">{tx('默认空间', 'Default')}</div>
                  </div>
                  <div className="workspace-radio">
                    <input
                      type="radio"
                      checked={!selectedTeamID}
                      onChange={() => setSelectedTeamID('')}
                    />
                  </div>
                </div>

                {/* 团队空间列表 */}
                {teams.map((t: any) => (
                  <div
                    key={t.id}
                    className={`workspace-item ${selectedTeamID === t.id ? 'active' : ''}`}
                    onClick={() => setSelectedTeamID(t.id)}
                  >
                    <div className="workspace-icon team-icon">👥</div>
                    <div className="workspace-info">
                      <div className="workspace-name">{t.name}</div>
                      <div className="workspace-role">
                        {t.role === 'owner' ? tx('所有者', 'Owner') :
                         t.role === 'admin' ? tx('管理员', 'Admin') :
                         t.role === 'member' ? tx('成员', 'Member') :
                         tx('观察员', 'Viewer')}
                      </div>
                    </div>
                    <div className="workspace-radio">
                      <input
                        type="radio"
                        checked={selectedTeamID === t.id}
                        onChange={() => setSelectedTeamID(t.id)}
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
            <div className="modal-footer">
              <button
                className="btn btn-secondary"
                onClick={() => setWorkspaceModalOpen(false)}
              >
                {tx('取消', 'Cancel')}
              </button>
              <button
                className="btn btn-primary"
                onClick={handleConfirmWorkspaceSwitch}
              >
                {tx('确认切换', 'Switch')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
