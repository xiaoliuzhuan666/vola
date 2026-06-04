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
  const { tx, locale } = useI18n()
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
        '你好！我是模拟的 AI 助手。连接 Vola 后，我能自动读取你的 Vola 个人资产（Profile/Skills）。在下方输入框和我聊聊试试吧！',
        'Hello! I am a simulated AI assistant. After connecting to Vola, I can automatically read your Vola assets. Try chatting with me below!'
      )
    }
  ])

  const mcpURL = `${window.location.origin}/mcp`
  const profileTestPrompt = 'Read my Vola profile and summarize what you know about my working preferences.'
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

      // 写入个人偏好偏好
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
              '我刚才尝试读取了你的 Vola Hub，但它目前还是空的哦。建议您在第 1 步里点击“一键导入 Demo 预设”，导入数据后我就可以根据您的编程偏好给出专属回复了！',
              'I just tried reading your Vola Hub, but it is currently empty. I suggest clicking "One-click Import Demo Set" in Step 1. After importing, I can give customized responses based on your preferences!'
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
      title: tx('在 Claude 添加 Connector', 'Add a Connector in Claude'),
      path: 'Claude → Settings → Connectors',
      steps: [
        tx('打开 Claude 设置里的 Connectors 页面。', 'Open the Connectors page in Claude settings.'),
        tx('选择添加新的 custom connector，名称填写 Vola。', 'Add a new custom connector and name it Vola.'),
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
        tx('保存配置后点击 Connect、Authenticate 或 Open，按浏览器提示完成 Vola 授权。', 'After saving, click Connect, Authenticate, or Open, then finish Vola authorization in the browser.'),
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
      // clip permission blocked
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

          {/* 一键导入 Demo 预设数据 */}
          <div style={{ marginTop: '20px', padding: '16px', background: 'rgba(65, 77, 136, 0.04)', borderRadius: '12px', border: '1px dashed rgba(65, 77, 136, 0.15)' }}>
            <h4 style={{ margin: '0 0 8px 0', fontSize: '14px', color: '#27335f', fontWeight: 'bold' }}>
              💡 {tx('一键加载示例数据集以体验 (推荐)', 'One-click Import Demo Set (Recommended)')}
            </h4>
            <p style={{ margin: '0 0 12px 0', fontSize: '12px', color: 'var(--color-text-secondary)' }}>
              {tx('我们为你准备了包含 TypeScript 开发偏好以及 Git 规范化提交的示例 Skill。点击导入后，网页沙盒能根据偏好进行对话应答。', 'We prepared a profile template and Git format skill bundle for you. Clicking import allows the sandbox simulator to read these preferences.')}
            </p>
            <button
              type="button"
              className={`btn btn-sm ${demoImported ? 'btn-outline' : 'btn-primary'}`}
              disabled={importingDemo}
              onClick={handleImportDemo}
            >
              {importingDemo ? tx('导入中...', 'Importing...') : demoImported ? tx('已成功加载 Demo 数据 ✓', 'Demo Data Loaded ✓') : tx('一键加载 Demo 数据集', 'One-click Load Demo Data')}
            </button>
            {demoImported && (
              <div className="demo-import-hint">
                <span>✓</span> {tx('已自动向 /memory 和 /skills 写入示例数据。', 'Example data written to /memory and /skills successfully.')}
              </div>
            )}
          </div>

          <p className="mcp-other-note" style={{ marginTop: '16px' }}>
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
          <p>{tx('连接保存后，在对应 AI 工具的新对话里发送这句话，确认它能读取被授权的 Vola 资料。', 'After saving the connection, send this in a new chat in that AI tool to confirm it can read authorized Vola data.')}</p>
          <div className="mcp-copy-row prompt-row">
            <code>{activeTestPrompt}</code>
            <button className="btn btn-primary" onClick={() => { void copyText(activeTestPrompt, 'prompt') }}>
              {copied === 'prompt' ? tx('已复制 ✓', 'Copied ✓') : tx('复制提示词', 'Copy prompt')}
            </button>
          </div>

          {/* 实时连接状态检测灯 */}
          <div style={{ marginTop: '20px', padding: '12px 16px', background: 'rgba(255, 255, 255, 0.6)', borderRadius: '12px', border: '1px solid rgba(22, 182, 204, 0.15)', display: 'flex', alignItems: 'center', gap: '10px' }}>
            <span
              className={`nav-item-badge ${isCurrentPlatformConnected ? 'badge-online' : 'badge-offline'}`}
              style={{ padding: '4px 8px', borderRadius: '10px', fontSize: '11px' }}
            >
              {isCurrentPlatformConnected ? tx('连接就绪 Linked', 'Linked') : tx('等待连接 Waiting', 'Waiting')}
            </span>
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
              {isCurrentPlatformConnected
                ? tx(`已成功检测到来自 ${activeGuide.label} 的活跃 MCP 连接！🎉`, `Successfully detected active MCP connection from ${activeGuide.label}! 🎉`)
                : tx(`等待您在 ${activeGuide.label} 中保存配置并授权...`, `Waiting for config and authorization in ${activeGuide.label}...`)}
            </span>
          </div>

          <div className="mcp-step-actions" style={{ marginTop: '24px' }}>
            <Link to="/" className="btn btn-primary">{tx('打开数据概览', 'Open Home')}</Link>
            <Link to="/connections" className="btn btn-outline">{tx('查看已连接应用', 'View connected apps')}</Link>
          </div>
        </article>
      </section>

      {/* 网页版沙盒对话模拟器 */}
      <section className="sandbox-card" style={{ maxWidth: '800px', margin: '32px auto 0' }}>
        <div className="sandbox-header">
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ display: 'inline-block', width: '8px', height: '8px', borderRadius: '50%', background: '#10b981' }}></span>
            <strong style={{ color: '#27335f', fontSize: '14px' }}>{tx('Vola Playground 沙盒对话模拟', 'Vola Playground Chat Sandbox')}</strong>
          </div>
          <span style={{ fontSize: '11px', color: 'var(--color-text-secondary)' }}>
            {tx('无需配置外部客户端，即可在此模拟对话流转', 'Simulate data integration here instantly without external apps')}
          </span>
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
            <div className="mock-chat-bubble agent" style={{ opacity: 0.6 }}>
              {tx('AI 正在思考并调用 MCP 接口...', 'AI is thinking and invoking MCP tools...')}
            </div>
          )}
        </div>

        <form onSubmit={handleSendSandbox} className="sandbox-input-area">
          <input
            type="text"
            value={sandboxInput}
            onChange={(e) => setSandboxInput(e.target.value)}
            placeholder={tx('向模拟 AI 发送消息测试 Vola 读取，例如："总结我的开发偏好"...', 'Send msg to test Vola reads, e.g., "Summarize my dev preferences"...')}
            disabled={isSandboxTyping}
          />
          <button type="submit" className="btn btn-primary" disabled={isSandboxTyping || !sandboxInput.trim()}>
            {tx('发送', 'Send')}
          </button>
        </form>
      </section>
    </div>
  )
}
