import { useEffect, useMemo } from 'react'
import { useLocation } from 'react-router-dom'
import { useSetup, type CloudPlatformTab } from '../SetupPage'
import { useI18n } from '../../i18n'

type CliStep = {
  title: string
  copy: string
  content?: string
  copyKey?: string
  copyLabel?: string
}

type CliGuide = {
  key: CloudPlatformTab
  label: string
  steps: CliStep[]
  notes: string[]
}

export default function SetupCloudPage() {
  const { tx } = useI18n()
  const location = useLocation()
  const {
    baseUrl,
    cloudModeNeedsPublicUrl,
    cloudPlatform,
    setCloudPlatform,
    copied,
    copyToClipboard,
    claudeCloudCommand,
    codexCloudCommand,
    geminiCloudCommand,
    cursorAgentStatusCommand,
    codexLoginCommand,
    cursorAgentLoginCommand,
    geminiAuthCommand,
    codexStatusCommand,
  } = useSetup()

  useEffect(() => {
    const next = (location.state as { cloudPlatform?: CloudPlatformTab } | null)?.cloudPlatform
    if (next === 'claude' || next === 'codex' || next === 'gemini' || next === 'cursor') {
      setCloudPlatform(next)
    }
  }, [location.state, setCloudPlatform])

  const guides = useMemo<CliGuide[]>(() => [
    {
      key: 'claude',
      label: 'Claude Code',
      steps: [
        {
          title: tx('添加远程 MCP Server', 'Add remote MCP server'),
          copy: tx('在终端运行这个命令，Vola 会作为全局 MCP Server 出现在 Claude Code 中。', 'Run this command in your terminal. Vola will appear as a global MCP server in Claude Code.'),
          content: claudeCloudCommand,
          copyKey: 'cli-claude-add',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('在 Claude Code 中发起授权', 'Start authorization in Claude Code'),
          copy: tx('打开 Claude Code，运行 /mcp，选择 vola，然后开始认证。', 'Open Claude Code, run /mcp, choose vola, then start authentication.'),
          content: '/mcp',
          copyKey: 'cli-claude-auth',
          copyLabel: tx('复制 /mcp', 'Copy /mcp'),
        },
        {
          title: tx('完成浏览器授权', 'Finish browser authorization'),
          copy: tx('浏览器会打开 Vola 授权页；完成登录和批准后，Claude Code 会自动保存并刷新凭证。', 'The browser opens Vola authorization. After sign-in and approval, Claude Code saves and refreshes credentials automatically.'),
        },
      ],
      notes: [
        tx('如果浏览器没有自动打开，就手动复制 Claude Code 提供的授权链接。', 'If the browser does not open automatically, manually copy the authorization link shown by Claude Code.'),
        tx('如果网页授权完成后 CLI 仍在等待，把浏览器地址栏里的完整 callback URL 粘回 Claude Code。', 'If the CLI still waits after web auth completes, paste the full callback URL from the browser address bar back into Claude Code.'),
      ],
    },
    {
      key: 'codex',
      label: 'Codex',
      steps: [
        {
          title: tx('添加远程 MCP Server', 'Add remote MCP server'),
          copy: tx('运行 add 命令后，Vola 会写入 Codex 的用户级 MCP 配置，可在多个工作区复用。', 'After running the add command, Vola is written into Codex user-level MCP config and can be reused across workspaces.'),
          content: codexCloudCommand,
          copyKey: 'cli-codex-add',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('发起授权', 'Start authorization'),
          copy: tx('运行 login 命令后，浏览器会打开 Vola 授权页面。', 'Run the login command and the browser will open Vola authorization.'),
          content: codexLoginCommand,
          copyKey: 'cli-codex-login',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('确认连接状态', 'Confirm connection status'),
          copy: tx('授权完成后运行 list 命令，确认 vola 已连接。', 'After authorization, run the list command to confirm vola is connected.'),
          content: codexStatusCommand,
          copyKey: 'cli-codex-list',
          copyLabel: tx('复制命令', 'Copy command'),
        },
      ],
      notes: [
        tx('如果浏览器没有自动打开，就手动复制终端里提供的授权链接继续完成授权。', 'If the browser does not open automatically, manually copy the authorization link shown in the terminal to continue.'),
      ],
    },
    {
      key: 'gemini',
      label: 'Gemini CLI',
      steps: [
        {
          title: tx('添加远程 MCP Server', 'Add remote MCP server'),
          copy: tx('运行 add 命令时需要带 --transport http。', 'Run the add command with --transport http.'),
          content: geminiCloudCommand,
          copyKey: 'cli-gemini-add',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('在 Gemini 中发起授权', 'Start authorization in Gemini'),
          copy: tx('打开 Gemini，运行授权命令；如果使用了别的 server 名称，把 vola 换成你的名称。', 'Open Gemini and run the auth command. If you used a different server name, replace vola with your own name.'),
          content: geminiAuthCommand,
          copyKey: 'cli-gemini-auth',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('开始使用 Vola 工具', 'Start using Vola tools'),
          copy: tx('授权完成后，可以继续用 /mcp 查看状态，或直接开始调用 Vola 工具。', 'After authorization, use /mcp to inspect status, or immediately start calling Vola tools.'),
          content: '/mcp',
          copyKey: 'cli-gemini-status',
          copyLabel: tx('复制 /mcp', 'Copy /mcp'),
        },
      ],
      notes: [
        tx('Gemini CLI 当前已验证 Remote MCP + OAuth 可用。', 'Gemini CLI has been verified with Remote MCP + OAuth.'),
      ],
    },
    {
      key: 'cursor',
      label: 'Cursor Agent',
      steps: [
        {
          title: tx('配置 MCP JSON', 'Configure MCP JSON'),
          copy: tx('把这个配置加入项目目录的 .cursor/mcp.json，或用户目录的 ~/.cursor/mcp.json。', 'Add this config to .cursor/mcp.json in the project directory, or ~/.cursor/mcp.json in the user directory.'),
          content: JSON.stringify({ mcpServers: { vola: { url: `${baseUrl}/mcp` } } }, null, 2),
          copyKey: 'cli-cursor-json',
          copyLabel: tx('复制配置', 'Copy config'),
        },
        {
          title: tx('发起授权', 'Start authorization'),
          copy: tx('运行 login 命令后，Cursor Agent 会读取配置，并在浏览器中打开 Vola 授权。', 'Run the login command. Cursor Agent reads the config and opens Vola authorization in the browser.'),
          content: cursorAgentLoginCommand,
          copyKey: 'cli-cursor-login',
          copyLabel: tx('复制命令', 'Copy command'),
        },
        {
          title: tx('确认连接状态', 'Confirm connection status'),
          copy: tx('授权完成后运行 list 命令检查状态。', 'After authorization, run the list command to inspect status.'),
          content: cursorAgentStatusCommand,
          copyKey: 'cli-cursor-list',
          copyLabel: tx('复制命令', 'Copy command'),
        },
      ],
      notes: [
        tx('需要查看工具时，可以继续运行 cursor-agent mcp list-tools vola。', 'To view tools, you can also run cursor-agent mcp list-tools vola.'),
      ],
    },
  ], [
    baseUrl,
    claudeCloudCommand,
    codexCloudCommand,
    codexLoginCommand,
    codexStatusCommand,
    cursorAgentLoginCommand,
    cursorAgentStatusCommand,
    geminiAuthCommand,
    geminiCloudCommand,
    tx,
  ])

  const activeGuide = guides.find((guide) => guide.key === cloudPlatform) || guides[0]
  const [firstStep, ...remainingSteps] = activeGuide.steps

  return (
    <section className="setup-wizard mcp-setup-wizard">
      {cloudModeNeedsPublicUrl && (
        <div className="alert alert-warn">
          {tx('当前地址是 ', 'Current address: ')}<code>{baseUrl}</code>{tx('。CLI Apps 需要一个可公开访问的 HTTPS Hub URL；如果你现在在本地开发，建议先用本地模式，或通过公网域名 / 隧道暴露这个 Hub。', '. CLI apps need a publicly reachable HTTPS Hub URL. If you are developing locally, use local mode first or expose this Hub through a public domain / tunnel.')}
        </div>
      )}

      <article className="wizard-card mcp-step-card">
        <div className="wizard-step-label">{tx('第 1 步', 'Step 1')}</div>
        <h3>{tx('选择 CLI 工具', 'Choose a CLI tool')}</h3>
        <div className="mcp-platform-tabs" role="tablist" aria-label={tx('CLI 工具', 'CLI tools')}>
          {guides.map((guide) => (
            <button
              key={guide.key}
              type="button"
              role="tab"
              aria-selected={cloudPlatform === guide.key}
              className={cloudPlatform === guide.key ? 'active' : ''}
              onClick={() => setCloudPlatform(guide.key)}
            >
              {guide.label}
            </button>
          ))}
        </div>
        {firstStep && (
          <div className="cli-primary-step" role="tabpanel">
            <p>{firstStep.copy}</p>
            {firstStep.content && firstStep.copyKey && (
              <div className={firstStep.content.trim().startsWith('{') ? 'mcp-copy-row config-row' : 'mcp-copy-row'}>
                <code>{firstStep.content}</code>
                <button className="btn btn-primary" onClick={() => copyToClipboard(firstStep.content || '', firstStep.copyKey || '')}>
                  {copied === firstStep.copyKey ? tx('已复制 ✓', 'Copied ✓') : firstStep.copyLabel || tx('复制', 'Copy')}
                </button>
              </div>
            )}
          </div>
        )}
      </article>

      {remainingSteps.map((step, index) => (
        <article className="wizard-card mcp-step-card" key={`${activeGuide.key}:${step.title}`}>
          <div className="wizard-step-label">Step {index + 2}</div>
          <h3>{step.title}</h3>
          <p>{step.copy}</p>
          {step.content && step.copyKey && (
            <div className={step.content.trim().startsWith('{') ? 'mcp-copy-row config-row' : 'mcp-copy-row'}>
              <code>{step.content}</code>
              <button className="btn btn-primary" onClick={() => copyToClipboard(step.content || '', step.copyKey || '')}>
                {copied === step.copyKey ? tx('已复制 ✓', 'Copied ✓') : step.copyLabel || tx('复制', 'Copy')}
              </button>
            </div>
          )}
        </article>
      ))}

      {activeGuide.notes.length > 0 && (
        <article className="wizard-card mcp-step-card">
          <div className="wizard-step-label">{tx('提示', 'Notes')}</div>
          <ul className="cli-note-list">
            {activeGuide.notes.map((note) => <li key={note}>{note}</li>)}
          </ul>
        </article>
      )}
    </section>
  )
}
