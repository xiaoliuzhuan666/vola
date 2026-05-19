import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { useI18n } from '../i18n'

type PlatformKey = 'claude' | 'chatgpt' | 'cursor' | 'windsurf' | 'other'
type McpTabKey = Exclude<PlatformKey, 'other'>

function normalizePlatform(raw?: string): PlatformKey | '' {
  const value = (raw || '').toLowerCase()
  if (value === 'claude' || value === 'chatgpt' || value === 'cursor' || value === 'windsurf') return value
  if (value === 'other' || value === 'mcp' || value === 'api') return 'other'
  return ''
}

function tabFromPlatform(platform: PlatformKey | ''): McpTabKey {
  if (platform === 'chatgpt' || platform === 'cursor' || platform === 'windsurf') return platform
  return 'claude'
}

export default function OnboardingPage() {
  const { tx } = useI18n()
  const params = useParams()
  const location = useLocation()
  const selectedPlatform = normalizePlatform(params.platform || (location.state as { platform?: string } | null)?.platform)
  const [activeTab, setActiveTab] = useState<McpTabKey>(tabFromPlatform(selectedPlatform))
  const [copied, setCopied] = useState('')
  const mcpURL = `${window.location.origin}/mcp`
  const profileTestPrompt = 'Read my neuDrive profile and summarize what you know about my working preferences.'
  const projectTestPrompt = tx('请读取当前项目，并把项目背景、开发约定和常用命令保存到 neuDrive。', 'Please read the current project and save its background, development conventions, and common commands to neuDrive.')

  useEffect(() => {
    setActiveTab(tabFromPlatform(selectedPlatform))
  }, [selectedPlatform])

  const mcpGuides = useMemo(() => [
    {
      key: 'claude' as const,
      label: 'Claude',
      title: tx('在 Claude 添加 Connector', 'Add a Connector in Claude'),
      path: 'Claude → Settings → Connectors',
      steps: [
        tx('打开 Claude 设置里的 Connectors 页面。', 'Open the Connectors page in Claude settings.'),
        tx('选择添加新的 custom connector，名称填写 neuDrive。', 'Add a new custom connector and name it neuDrive.'),
        tx('把第 2 步里的 MCP URL 粘贴到 URL 字段，然后保存。', 'Paste the MCP URL from Step 2 into the URL field, then save.'),
      ],
      settingsUrl: 'https://claude.ai/settings/connectors',
      settingsLabel: tx('打开 Claude Connectors', 'Open Claude Connectors'),
    },
    {
      key: 'chatgpt' as const,
      label: 'ChatGPT',
      title: tx('在 ChatGPT 创建 App', 'Create an App in ChatGPT'),
      path: 'ChatGPT → Settings → Apps → Advanced Settings',
      steps: [
        tx('打开 Settings → Apps → Advanced Settings。', 'Open Settings → Apps → Advanced Settings.'),
        tx('如果 Developer mode 还没有开启，先开启 Developer mode。', 'If Developer mode is not enabled yet, turn it on first.'),
        tx('点击 Create app，MCP Server URL 填写第 2 步里的地址，Authentication 保持 OAuth，然后保存。', 'Click Create app, paste the Step 2 address into MCP Server URL, keep Authentication as OAuth, then save.'),
      ],
      settingsUrl: 'https://chatgpt.com/apps#settings/Connectors/Advanced',
      settingsLabel: tx('打开 ChatGPT App Settings', 'Open ChatGPT App Settings'),
    },
    {
      key: 'cursor' as const,
      label: 'Cursor',
      title: tx('在 Cursor 添加 Custom MCP', 'Add a Custom MCP in Cursor'),
      path: 'Cursor → Settings → Tools & MCPs → Add Custom MCP',
      steps: [
        tx('打开 Settings → Tools & MCPs → Add Custom MCP。', 'Open Settings → Tools & MCPs → Add Custom MCP.'),
        tx('复制第 2 步里的 Cursor MCP config，粘贴到 Cursor 的 MCP 配置里。', 'Copy the Cursor MCP config from Step 2 and paste it into Cursor’s MCP settings.'),
        tx('保存配置后点击 Connect、Authenticate 或 Open，按浏览器提示完成 neuDrive 授权。', 'After saving, click Connect, Authenticate, or Open, then finish neuDrive authorization in the browser.'),
      ],
    },
    {
      key: 'windsurf' as const,
      label: 'Windsurf',
      title: tx('在 Windsurf 添加 MCP Server', 'Add an MCP Server in Windsurf'),
      path: 'Windsurf → Settings → Cascade → MCP Servers',
      steps: [
        tx('打开 Settings → Cascade → MCP Servers。', 'Open Settings → Cascade → MCP Servers.'),
        tx('复制第 2 步里的 Windsurf MCP config，粘贴到 Windsurf 的 MCP Servers 配置里。', 'Copy the Windsurf MCP config from Step 2 and paste it into Windsurf’s MCP Servers settings.'),
        tx('保存配置后点击 Connect、Authenticate 或 Open，按浏览器提示完成 neuDrive 授权。', 'After saving, click Connect, Authenticate, or Open, then finish neuDrive authorization in the browser.'),
      ],
    },
  ], [tx])
  const activeGuide = mcpGuides.find((guide) => guide.key === activeTab) || mcpGuides[0]
  const isEditorGuide = activeTab === 'cursor' || activeTab === 'windsurf'
  const mcpPayload = activeTab === 'cursor'
    ? JSON.stringify({ mcpServers: { neudrive: { url: mcpURL } } }, null, 2)
    : activeTab === 'windsurf'
      ? JSON.stringify({ mcpServers: { neudrive: { serverUrl: mcpURL } } }, null, 2)
      : mcpURL
  const step2Copy = isEditorGuide
    ? tx('复制对应 MCP 配置，粘贴到编辑器的 MCP 设置里。', 'Copy the matching MCP config and paste it into the editor’s MCP settings.')
    : tx('在平台的 MCP Server URL 字段里粘贴这个地址。', 'Paste this address into the platform’s MCP Server URL field.')
  const copyButtonLabel = isEditorGuide ? tx('复制配置', 'Copy config') : tx('复制 URL', 'Copy URL')
  const copiedButtonLabel = isEditorGuide ? tx('已复制配置 ✓', 'Config copied ✓') : tx('已复制 ✓', 'Copied ✓')
  const activeTestPrompt = isEditorGuide ? projectTestPrompt : profileTestPrompt

  const copyText = async (value: string, key: string) => {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(value)
      } else {
        const textarea = document.createElement('textarea')
        textarea.value = value
        textarea.setAttribute('readonly', 'true')
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }
    } catch {
      // Still show UI feedback so the setup flow does not feel broken if clipboard permission is blocked.
    }
    setCopied(key)
    window.setTimeout(() => setCopied(''), 1600)
  }

  return (
    <div className="page onboarding-page">
      <section className="setup-wizard mcp-setup-wizard">
        <article className="wizard-card mcp-step-card">
          <div className="wizard-step-label">{tx('第 1 步，共 3 步', 'Step 1 of 3')}</div>
          <h3>{tx('把平台接到个人数据 Hub', 'Connect the platform to your data hub')}</h3>
          <p>{tx('选择你正在使用的平台，然后把它接到同一份 profile、memory、projects、skills 和 vault 权限。', 'Choose your platform, then connect it to the same profile, memory, projects, skills, and vault access.')}</p>
          <div className="mcp-platform-tabs" role="tablist" aria-label={tx('平台接入入口', 'Platform setup entry')}>
            {mcpGuides.map((guide) => (
              <button
                key={guide.key}
                role="tab"
                aria-selected={activeTab === guide.key}
                className={activeTab === guide.key ? 'active' : ''}
                onClick={() => setActiveTab(guide.key)}
              >
                {guide.label}
              </button>
            ))}
          </div>
          <div className="mcp-platform-guide" role="tabpanel">
            <div className="mcp-guide-heading">
              <strong>{activeGuide.title}</strong>
              <span>{activeGuide.path}</span>
            </div>
            <ol>
              {activeGuide.steps.map((item) => <li key={item}>{item}</li>)}
            </ol>
            {activeGuide.settingsUrl && (
              <a className="btn btn-outline" href={activeGuide.settingsUrl} target="_blank" rel="noreferrer">
                {activeGuide.settingsLabel}
              </a>
            )}
          </div>
          <p className="mcp-other-note">
            {tx('如果使用其他 MCP Client，在该工具的 MCP Server 设置里添加第 2 步里的 URL 即可。', 'For another MCP client, add the URL from Step 2 in that tool’s MCP Server settings.')}
          </p>
        </article>

        <article className="wizard-card mcp-step-card">
          <div className="wizard-step-label">{tx('第 2 步，共 3 步', 'Step 2 of 3')}</div>
          <h3>{tx('添加 MCP Server', 'Add MCP Server')}</h3>
          <p>{step2Copy}</p>
          <div className={isEditorGuide ? 'mcp-copy-row config-row' : 'mcp-copy-row'}>
            <code>{mcpPayload}</code>
            <button className="btn btn-primary" onClick={() => { void copyText(mcpPayload, 'mcp') }}>
              {copied === 'mcp' ? copiedButtonLabel : copyButtonLabel}
            </button>
          </div>
        </article>

        <article className="wizard-card mcp-step-card">
          <div className="wizard-step-label">{tx('第 3 步，共 3 步', 'Step 3 of 3')}</div>
          <h3>{tx('在对话中测试数据读取', 'Test data access in chat')}</h3>
          <p>{tx('连接保存后，在对应 AI 工具的新对话里发送这句话，确认它能读取被授权的 neuDrive 资料。', 'After saving the connection, send this in a new chat in that AI tool to confirm it can read authorized neuDrive data.')}</p>
          <div className="mcp-copy-row prompt-row">
            <code>{activeTestPrompt}</code>
            <button className="btn btn-primary" onClick={() => { void copyText(activeTestPrompt, 'prompt') }}>
              {copied === 'prompt' ? tx('已复制 ✓', 'Copied ✓') : tx('复制提示词', 'Copy prompt')}
            </button>
          </div>
          <div className="mcp-step-actions">
            <Link to="/" className="btn btn-primary">{tx('打开概览', 'Open Home')}</Link>
            <Link to="/connections" className="btn btn-outline">{tx('查看已连接应用', 'View connected apps')}</Link>
          </div>
        </article>
      </section>
    </div>
  )
}
