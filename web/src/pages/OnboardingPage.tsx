import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { api } from '../api'
import { useI18n } from '../i18n'

type PlatformKey = 'claude' | 'chatgpt' | 'cursor' | 'windsurf' | 'other'
type McpTabKey = Exclude<PlatformKey, 'other'>

type SandboxMessage = {
  id: string
  sender: 'user' | 'agent' | 'system'
  text: string
}

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

  // 新手交互状态
  const [connections, setConnections] = useState<any[]>([])
  const [demoImported, setDemoImported] = useState(false)
  const [importingDemo, setImportingDemo] = useState(false)

  // 沙盒对话状态
  const [sandboxInput, setSandboxInput] = useState('')
  const [isSandboxTyping, setIsSandboxTyping] = useState(false)
  const [sandboxMessages, setSandboxMessages] = useState<SandboxMessage[]>([
    {
      id: '1',
      sender: 'agent',
      text: tx(
        '这是一个模拟 AI。加载示例数据后，它会尝试读取 Vola 里的 profile 和 skills。',
        'This is a simulated AI. After loading sample data, it will try to read profile and skills from Vola.'
      )
    }
  ])

  const mcpURL = `${window.location.origin}/mcp`
  const profileTestPrompt = tx('请读取我的 Vola profile，并总结我的工作偏好。', 'Read my Vola profile and summarize what you know about my working preferences.')
  const projectTestPrompt = tx('请读取当前项目，并把项目背景、开发约定和常用命令保存到 Vola。', 'Please read the current project and save its background, development conventions, and common commands to Vola.')

  useEffect(() => {
    setActiveTab(tabFromPlatform(selectedPlatform))
  }, [selectedPlatform])

  // 3秒轮询检测连接状态
  useEffect(() => {
    let active = true
    const checkConnections = async () => {
      try {
        const conns = await api.getConnections()
        if (active) {
          setConnections(conns)
        }
      } catch (err) {
        console.error("Failed to load connections in onboarding:", err)
      }
    }
    void checkConnections()
    const timer = setInterval(checkConnections, 3000)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [])

  const isCurrentPlatformConnected = useMemo(() => {
    return connections.some((c) => (c.platform || '').toLowerCase() === activeTab.toLowerCase())
  }, [connections, activeTab])

  // 一键导入 Demo 预设
  const handleImportDemo = async () => {
    if (importingDemo) return
    try {
      setImportingDemo(true)

      // 写入个人偏好
      await api.writeTree('/memory/profile/preferences.md', {
        content: `---\ntitle: 个人偏好设置\ntype: preferences\n---\n\n# 我的开发与交互偏好\n\n## 语言与技术栈\n- 主要编程语言：TypeScript, React, Golang\n- 常用框架：Vite, TailwindCSS\n\n## 交互偏好\n- 喜欢简洁（KISS）的代码风格。\n- 编写 AI 技能包或脚本时，倾向于添加详细的中文注释。`
      })

      // 写入 Git 提交助手技能包
      await api.writeTree('/skills/git-commit-helper/SKILL.md', {
        content: `---\nname: Git Commit Helper\ndescription: 帮助规范化生成 Git Commit Message\n---\n\n# Git Commit Helper\n\n## Use when\n需要按照规范自动编写简洁 of Git Commit 信息时使用。\n\n## Avoid when\n单次提交包含多个不相关的大规模修改时，应手动分批提交。\n\n## Instructions\n- 提交类型包括：feat, fix, docs, style, refactor, test, chore。\n- 使用第一人称现在时，例如 "Add feature X" 而不是 "Added feature X"。`
      })

      setDemoImported(true)
    } catch (err) {
      console.error("Failed to import demo data:", err)
    } finally {
      setImportingDemo(false)
    }
  }

  // 发送沙盒消息
  const handleSendSandbox = (e: FormEvent) => {
    e.preventDefault()
    const trimmed = sandboxInput.trim()
    if (!trimmed || isSandboxTyping) return

    const userMsg: SandboxMessage = { id: Date.now().toString(), sender: 'user', text: trimmed }
    setSandboxMessages((prev) => [...prev, userMsg])
    setSandboxInput('')
    setIsSandboxTyping(true)

    // 模拟 AI 进行工具调用和文本生成
    setTimeout(() => {
      const logMsg: SandboxMessage = {
        id: (Date.now() + 1).toString(),
        sender: 'system',
        text: demoImported
          ? tx(
              '[MCP 工具调用] 成功读取 /memory/profile/preferences.md 和 /skills/git-commit-helper',
              '[MCP Tool Call] Successfully read /memory/profile/preferences.md and /skills/git-commit-helper'
            )
          : tx(
              '[MCP 工具调用] 尝试读取 /memory/profile/preferences.md，未找到相关用户数据',
              '[MCP Tool Call] Attempted to read /memory/profile/preferences.md, no user data found'
            ),
      }
      setSandboxMessages((prev) => [...prev, logMsg])

      setTimeout(() => {
        const replyText = demoImported
          ? tx(
              `我已通过 Vola MCP 接口读取到您的偏好！
您是一位偏好 TypeScript、React 和 Golang 的开发者，注重编写简洁（KISS）的代码，并且喜欢在编写 AI 技能包或脚本时添加详细的中文注释。

我还发现了一个您的专属技能包：[Git Commit Helper]。今后您在 Git 提交代码时我可以自动帮您规范排版！`,
              `I have read your preferences via the Vola MCP interface!
You are a developer who prefers TypeScript, React, and Golang, focuses on writing clean (KISS) code, and likes adding detailed comments.

I also found your skill package: [Git Commit Helper]. I can help format your Git commits in the future!`
            )
          : tx(
              '我刚才尝试读取你的 Vola Hub，但目前没有找到示例资料。展开下方示例区并加载数据后，可以再次测试。',
              'I tried reading your Vola Hub, but did not find sample data. Open the sample area below, load data, and try again.'
            )

        const agentMsg: SandboxMessage = {
          id: (Date.now() + 2).toString(),
          sender: 'agent',
          text: replyText,
        }
        setSandboxMessages((prev) => [...prev, agentMsg])
        setIsSandboxTyping(false)
      }, 1000)
    }, 800)
  }

  const mcpGuides = useMemo(() => [
    {
      key: 'claude' as const,
      label: 'Claude',
      title: tx('连接 Claude', 'Connect Claude'),
      path: 'Claude → Settings → Connectors',
      steps: [
        tx('打开 Claude 设置里的 Connectors 页面。', 'Open the Connectors page in Claude settings.'),
        tx('添加 custom connector，名称填写 Vola。', 'Add a custom connector and name it Vola.'),
        tx('粘贴第 2 步里的 MCP URL，然后保存。', 'Paste the MCP URL from Step 2, then save.'),
      ],
      settingsUrl: 'https://claude.ai/settings/connectors',
      settingsLabel: tx('打开 Claude Connectors', 'Open Claude Connectors'),
    },
    {
      key: 'chatgpt' as const,
      label: 'ChatGPT',
      title: tx('连接 ChatGPT', 'Connect ChatGPT'),
      path: 'ChatGPT → Settings → Apps → Advanced Settings',
      steps: [
        tx('打开 Settings → Apps → Advanced Settings。', 'Open Settings → Apps → Advanced Settings.'),
        tx('开启 Developer mode 后创建 app。', 'Enable Developer mode, then create an app.'),
        tx('MCP Server URL 填写第 2 步里的地址，Authentication 保持 OAuth，然后保存。', 'Paste the Step 2 address into MCP Server URL, keep Authentication as OAuth, then save.'),
      ],
      settingsUrl: 'https://chatgpt.com/apps#settings/Connectors/Advanced',
      settingsLabel: tx('打开 ChatGPT App Settings', 'Open ChatGPT App Settings'),
    },
    {
      key: 'cursor' as const,
      label: 'Cursor',
      title: tx('连接 Cursor', 'Connect Cursor'),
      path: 'Cursor → Settings → Tools & MCPs → Add Custom MCP',
      steps: [
        tx('打开 Settings → Tools & MCPs → Add Custom MCP。', 'Open Settings → Tools & MCPs → Add Custom MCP.'),
        tx('复制第 2 步里的 Cursor MCP config，粘贴到 Cursor 的 MCP 配置里。', 'Copy the Cursor MCP config from Step 2 and paste it into Cursor’s MCP settings.'),
        tx('保存配置后点击 Connect、Authenticate 或 Open，按浏览器提示完成 Vola 授权。', 'After saving, click Connect, Authenticate, or Open, then finish Vola authorization in the browser.'),
      ],
    },
    {
      key: 'windsurf' as const,
      label: 'Windsurf',
      title: tx('连接 Windsurf', 'Connect Windsurf'),
      path: 'Windsurf → Settings → Cascade → MCP Servers',
      steps: [
        tx('打开 Settings → Cascade → MCP Servers。', 'Open Settings → Cascade → MCP Servers.'),
        tx('复制第 2 步里的 Windsurf MCP config，粘贴到 Windsurf 的 MCP Servers 配置里。', 'Copy the Windsurf MCP config from Step 2 and paste it into Windsurf’s MCP Servers settings.'),
        tx('保存配置后点击 Connect、Authenticate 或 Open，按浏览器提示完成 Vola 授权。', 'After saving, click Connect, Authenticate, or Open, then finish Vola authorization in the browser.'),
      ],
    },
  ], [tx])

  const activeGuide = mcpGuides.find((guide) => guide.key === activeTab) || mcpGuides[0]
  const isEditorGuide = activeTab === 'cursor' || activeTab === 'windsurf'
  const mcpPayload = activeTab === 'cursor'
    ? JSON.stringify({ mcpServers: { vola: { url: mcpURL } } }, null, 2)
    : activeTab === 'windsurf'
      ? JSON.stringify({ mcpServers: { vola: { serverUrl: mcpURL } } }, null, 2)
      : mcpURL
  const step2Copy = isEditorGuide
    ? tx('复制配置，粘贴到编辑器的 MCP 设置里。', 'Copy the config and paste it into the editor’s MCP settings.')
    : tx('复制这个 MCP URL，粘贴到平台设置里。', 'Copy this MCP URL and paste it into the platform settings.')
  const copyButtonLabel = isEditorGuide ? tx('复制配置', 'Copy config') : tx('复制 URL', 'Copy URL')
  const copiedButtonLabel = isEditorGuide ? tx('已复制配置', 'Config copied') : tx('已复制', 'Copied')
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
      // clip permission blocked
    }
    setCopied(key)
    window.setTimeout(() => setCopied(''), 1600)
  }

  return (
    <div className="page onboarding-page">
      <section className="setup-wizard mcp-setup-wizard">
        <article className="wizard-card mcp-step-card">
          <div className="wizard-step-label">{tx('第 1 步：选择工具', 'Step 1: Choose a tool')}</div>
          <h3>{activeTab === 'claude' ? tx('从 Claude 开始连接', 'Start with Claude') : tx(`连接 ${activeGuide.label}`, `Connect ${activeGuide.label}`)}</h3>
          <p>{tx('第一次只需要连一个 AI 工具。连接后，它会读取你授权的 profile、memory、projects、skills 和 vault 权限。', 'Connect one AI tool first. After connecting, it can read authorized profile, memory, projects, skills, and vault access.')}</p>

          <div className="mcp-recommended-route">
            <div>
              <span>{tx('推荐入口', 'Recommended')}</span>
              <strong>{activeGuide.title}</strong>
              <small>{activeGuide.path}</small>
            </div>
            {activeGuide.settingsUrl && (
              <a className="btn btn-outline" href={activeGuide.settingsUrl} target="_blank" rel="noreferrer">
                {activeGuide.settingsLabel}
              </a>
            )}
          </div>

          <div className="mcp-platform-guide" role="tabpanel">
            <div className="mcp-guide-heading">
              <strong>{activeGuide.title}</strong>
              <span>{activeGuide.path}</span>
            </div>
            <ol>
              {activeGuide.steps.map((item) => <li key={item}>{item}</li>)}
            </ol>
          </div>

          <details className="mcp-other-clients" open={activeTab !== 'claude'}>
            <summary>{tx('其他工具', 'Other tools')}</summary>
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
          </details>

          <p className="mcp-other-note">
            {tx('其他 MCP Client 也可以使用第 2 步里的 URL。', 'Other MCP clients can also use the URL from Step 2.')}
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
          <p>{tx('保存连接后，在新对话里发送这句话。', 'After saving the connection, send this in a new chat.')}</p>
          <div className="mcp-copy-row prompt-row">
            <code>{activeTestPrompt}</code>
            <button className="btn btn-primary" onClick={() => { void copyText(activeTestPrompt, 'prompt') }}>
              {copied === 'prompt' ? tx('已复制', 'Copied') : tx('复制提示词', 'Copy prompt')}
            </button>
          </div>

          <div className="mcp-connection-note">
            <span
              className={`nav-item-badge ${isCurrentPlatformConnected ? 'badge-online' : 'badge-offline'}`}
            >
              {isCurrentPlatformConnected ? tx('已连接', 'Linked') : tx('等待连接', 'Waiting')}
            </span>
            <span>
              {isCurrentPlatformConnected
                ? tx(`已检测到来自 ${activeGuide.label} 的 MCP 连接。`, `Detected an active MCP connection from ${activeGuide.label}.`)
                : tx(`等待你在 ${activeGuide.label} 中保存配置并授权。`, `Waiting for config and authorization in ${activeGuide.label}.`)}
            </span>
          </div>

          <div className="mcp-step-actions">
            <Link to="/" className="btn btn-primary">{tx('打开数据概览', 'Open Home')}</Link>
            <Link to="/connections" className="btn btn-outline">{tx('查看已连接应用', 'View connected apps')}</Link>
          </div>
        </article>
      </section>

      <section className="mcp-advanced-panel">
        <details>
          <summary>
            <span>{tx('示例数据和网页沙盒', 'Sample data and web sandbox')}</span>
            <small>{tx('不连接外部客户端时再打开', 'Open this when you want to try without an external client')}</small>
          </summary>

          <div className="mcp-advanced-body">
            <div className="mcp-demo-card">
              <h4>{tx('加载示例数据', 'Load sample data')}</h4>
              <p>
                {tx('写入一份开发偏好和 Git Commit Helper skill，供下方网页沙盒读取。', 'Write a development preference profile and Git Commit Helper skill for the sandbox below.')}
              </p>
              <button
                type="button"
                className={`btn btn-sm ${demoImported ? 'btn-outline' : 'btn-primary'}`}
                disabled={importingDemo}
                onClick={handleImportDemo}
              >
                {importingDemo ? tx('导入中...', 'Importing...') : demoImported ? tx('Demo 数据已加载', 'Demo data loaded') : tx('加载 Demo 数据', 'Load demo data')}
              </button>
              {demoImported && (
                <div className="demo-import-hint">
                  <span>OK</span> {tx('已向 /memory 和 /skills 写入示例数据。', 'Example data written to /memory and /skills.')}
                </div>
              )}
            </div>

            <div className="sandbox-card">
              <div className="sandbox-header">
                <div className="sandbox-title">
                  <span aria-hidden="true" />
                  <strong>{tx('Vola Playground', 'Vola Playground')}</strong>
                </div>
                <span>{tx('模拟 AI 读取 Vola 资料', 'Simulate an AI reading Vola data')}</span>
              </div>

              <div className="sandbox-body">
                {sandboxMessages.map((msg) => {
                  if (msg.sender === 'system') {
                    return (
                      <div key={msg.id} className="mock-mcp-log">
                        {msg.text}
                      </div>
                    )
                  }
                  return (
                    <div key={msg.id} className={`mock-chat-bubble ${msg.sender}`}>
                      {msg.text}
                    </div>
                  )
                })}
                {isSandboxTyping && (
                  <div className="mock-chat-bubble agent is-typing">
                    {tx('AI 正在读取 MCP 数据...', 'AI is reading MCP data...')}
                  </div>
                )}
              </div>

              <form onSubmit={handleSendSandbox} className="sandbox-input-area">
                <input
                  type="text"
                  value={sandboxInput}
                  onChange={(e) => setSandboxInput(e.target.value)}
                  placeholder={tx('例如：总结我的开发偏好', 'For example: summarize my dev preferences')}
                  disabled={isSandboxTyping}
                />
                <button type="submit" className="btn btn-primary" disabled={isSandboxTyping || !sandboxInput.trim()}>
                  {tx('发送', 'Send')}
                </button>
              </form>
            </div>
          </div>
        </details>
      </section>
    </div>
  )
}
