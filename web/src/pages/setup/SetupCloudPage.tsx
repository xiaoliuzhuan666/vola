import { useEffect, useMemo, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { installCliTools, isTauri, type CliToolsInstallResult } from '../../api'
import { useSetup, type CloudPlatformTab } from '../SetupPage'
import { useI18n } from '../../i18n'

type CliGuide = {
  key: CloudPlatformTab
  label: string
  rankLabel?: string
  localPlatform: string
  localCommand: string
  localCopyKey: string
  localHint: string
  remoteCommand: string
  remoteCopyKey: string
  remoteCopyLabel?: string
  remoteIntro: string
  afterCopy: string[]
  verifyCommand?: string
  verifyCopyKey?: string
  troubleshooting: string[]
  advancedConfig?: {
    title: string
    content: string
    copyKey: string
  }
}

function commandClassName(content: string) {
  return content.trim().startsWith('{') ? 'mcp-copy-row config-row' : 'mcp-copy-row'
}

const sourceInstallCommand = `cd /path/to/Vola
./tools/install-vola.sh`

export default function SetupCloudPage() {
  const { tx } = useI18n()
  const location = useLocation()
  const [installingCli, setInstallingCli] = useState(false)
  const [cliInstallResult, setCliInstallResult] = useState<CliToolsInstallResult | null>(null)
  const [cliInstallError, setCliInstallError] = useState('')
  const {
    baseUrl,
    isDesktopRuntime,
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
      key: 'codex',
      label: 'Codex',
      rankLabel: tx('推荐', 'Recommended'),
      localPlatform: 'codex',
      localCommand: 'neu connect codex',
      localCopyKey: 'cli-local-codex',
      localHint: tx('这条命令会配置 Codex 的本地 MCP，并安装 Vola 管理的 Codex Skill。', 'This command configures Codex local MCP and installs the Vola-managed Codex skill.'),
      remoteCommand: `${codexCloudCommand} && ${codexLoginCommand}`,
      remoteCopyKey: 'cli-codex-cloud',
      remoteIntro: tx('把 Vola 添加到 Codex，并立即发起浏览器授权。', 'Add Vola to Codex and start browser authorization immediately.'),
      afterCopy: [
        tx('打开终端，粘贴命令并回车。', 'Open Terminal, paste the command, and press Enter.'),
        tx('浏览器打开后，登录 Vola 并批准授权。', 'When the browser opens, sign in to Vola and approve access.'),
      ],
      verifyCommand: codexStatusCommand,
      verifyCopyKey: 'cli-codex-check',
      troubleshooting: [
        tx('浏览器没有打开时，复制终端里的授权链接到浏览器。', 'If the browser does not open, copy the authorization link from Terminal into your browser.'),
        tx('看不到 vola 时，重新运行上面的命令。', 'If vola is not listed, run the command above again.'),
      ],
    },
    {
      key: 'claude',
      label: 'Claude Code',
      rankLabel: tx('其次', 'Second'),
      localPlatform: 'claude',
      localCommand: 'neu connect claude',
      localCopyKey: 'cli-local-claude',
      localHint: tx('这条命令会启动本地服务、创建访问凭证，并把 Vola 写入 Claude Code。', 'This command starts the local service, creates the access credential, and adds Vola to Claude Code.'),
      remoteCommand: claudeCloudCommand,
      remoteCopyKey: 'cli-claude-cloud',
      remoteIntro: tx('把 Vola 添加到 Claude Code 的用户级 MCP 配置。', 'Add Vola to the user-level MCP config in Claude Code.'),
      afterCopy: [
        tx('打开终端，粘贴命令并回车。', 'Open Terminal, paste the command, and press Enter.'),
        tx('打开 Claude Code，输入 /mcp，选择 vola 后完成授权。', 'Open Claude Code, type /mcp, choose vola, and finish authorization.'),
      ],
      verifyCommand: '/mcp',
      verifyCopyKey: 'cli-claude-check',
      troubleshooting: [
        tx('浏览器没有打开时，复制 Claude Code 给出的授权链接到浏览器。', 'If the browser does not open, copy the authorization link from Claude Code into your browser.'),
        tx('授权后还在等待时，把浏览器地址栏里的 callback URL 粘回 Claude Code。', 'If it still waits after authorization, paste the callback URL from the browser address bar back into Claude Code.'),
      ],
    },
    {
      key: 'gemini',
      label: 'Gemini CLI',
      localPlatform: 'gemini',
      localCommand: 'neu connect gemini',
      localCopyKey: 'cli-local-gemini',
      localHint: tx('这条命令会把本地 Vola MCP 写入 Gemini CLI。', 'This command writes the local Vola MCP config into Gemini CLI.'),
      remoteCommand: geminiCloudCommand,
      remoteCopyKey: 'cli-gemini-cloud',
      remoteIntro: tx('把 Vola 添加到 Gemini CLI 的远程 MCP 列表。', 'Add Vola to Gemini CLI as a remote MCP server.'),
      afterCopy: [
        tx('打开终端，粘贴命令并回车。', 'Open Terminal, paste the command, and press Enter.'),
        tx('打开 Gemini CLI，输入 /mcp auth vola 后完成授权。', 'Open Gemini CLI, type /mcp auth vola, and finish authorization.'),
      ],
      verifyCommand: '/mcp',
      verifyCopyKey: 'cli-gemini-check',
      troubleshooting: [
        tx('如果你用了别的 server 名称，把授权命令里的 vola 换成对应名称。', 'If you used another server name, replace vola in the auth command with that name.'),
      ],
    },
    {
      key: 'cursor',
      label: 'Cursor Agent',
      localPlatform: 'cursor',
      localCommand: 'neu connect cursor',
      localCopyKey: 'cli-local-cursor',
      localHint: tx('这条命令会写入 Cursor 的 MCP 配置。', 'This command writes the MCP config for Cursor.'),
      remoteCommand: cursorAgentLoginCommand,
      remoteCopyKey: 'cli-cursor-login',
      remoteIntro: tx('先保存下面的 MCP 配置，再运行授权命令。', 'Save the MCP config below first, then run the authorization command.'),
      afterCopy: [
        tx('把配置保存到 ~/.cursor/mcp.json。', 'Save the config to ~/.cursor/mcp.json.'),
        tx('打开终端，运行授权命令并完成浏览器授权。', 'Open Terminal, run the authorization command, and finish browser authorization.'),
      ],
      verifyCommand: cursorAgentStatusCommand,
      verifyCopyKey: 'cli-cursor-check',
      troubleshooting: [
        tx('需要查看工具列表时，运行 cursor-agent mcp list-tools vola。', 'To inspect the tools, run cursor-agent mcp list-tools vola.'),
      ],
      advancedConfig: {
        title: '~/.cursor/mcp.json',
        content: JSON.stringify({ mcpServers: { vola: { url: `${baseUrl}/mcp` } } }, null, 2),
        copyKey: 'cli-cursor-json',
      },
    },
  ], [
    baseUrl,
    claudeCloudCommand,
    codexCloudCommand,
    codexLoginCommand,
    codexStatusCommand,
    cursorAgentLoginCommand,
    cursorAgentStatusCommand,
    geminiCloudCommand,
    tx,
  ])

  const activeGuide = guides.find((guide) => guide.key === cloudPlatform) || guides[0]
  const featuredGuides = guides.filter((guide) => guide.rankLabel)
  const otherGuides = guides.filter((guide) => !guide.rankLabel)
  const useLocalCommand = cloudModeNeedsPublicUrl
  const primaryCommand = useLocalCommand ? activeGuide.localCommand : activeGuide.remoteCommand
  const primaryCopyKey = useLocalCommand ? activeGuide.localCopyKey : activeGuide.remoteCopyKey
  const primaryCopyLabel = useLocalCommand
    ? tx('复制连接命令', 'Copy connect command')
    : activeGuide.remoteCopyLabel || tx('复制这段命令', 'Copy this command')
  const primaryIntro = useLocalCommand ? activeGuide.localHint : activeGuide.remoteIntro
  const handleInstallCli = async () => {
    setInstallingCli(true)
    setCliInstallError('')
    try {
      const result = await installCliTools()
      setCliInstallResult(result)
    } catch (err: any) {
      setCliInstallError(err?.message || tx('安装 neu 失败', 'Failed to install neu'))
    } finally {
      setInstallingCli(false)
    }
  }

  return (
    <section className="setup-wizard mcp-setup-wizard">
      {cloudModeNeedsPublicUrl && (
        <div className="alert alert-warn cli-simple-alert">
          {isDesktopRuntime
              ? tx('桌面版会使用本机服务。终端里还没有 neu 时，先安装命令行工具，再执行连接命令。', 'The desktop app uses a local service. If Terminal does not have neu yet, install the command line tool before running the connect command.')
            : tx('这个页面现在不是公网 HTTPS 地址。使用本地连接命令；部署到 HTTPS 域名后，再回来复制远程 MCP 命令。', 'This page is not on a public HTTPS address. Use the local connect command. After deploying to an HTTPS, come back for the remote MCP command.')}
        </div>
      )}

      <article className="wizard-card mcp-step-card cli-hero-card">
        <div className="cli-hero-copy">
          <span className="wizard-step-label">{useLocalCommand ? tx('本地连接', 'Local connection') : tx('远程 MCP', 'Remote MCP')}</span>
          <h3>{tx('推荐用 Codex 连接 Vola', 'Connect Vola with Codex first')}</h3>
          <p>
            {useLocalCommand
              ? tx('先装好 neu，再复制连接命令。团队已经在用 Claude Code 时，选择第二个选项即可。', 'Install neu, then copy the connect command. Teams already using Claude Code can choose the second option.')
              : tx('Codex 会添加 MCP 并打开浏览器授权。Claude Code 是第二推荐。', 'Codex adds the MCP server and opens browser authorization. Claude Code is the second recommendation.')}
          </p>
        </div>

        <div className="mcp-platform-tabs cli-tool-tabs cli-featured-tabs" role="tablist" aria-label={tx('推荐 CLI 工具', 'Recommended CLI tools')}>
          {featuredGuides.map((guide) => (
            <button
              key={guide.key}
              type="button"
              role="tab"
              aria-selected={cloudPlatform === guide.key}
              className={cloudPlatform === guide.key ? 'active' : ''}
              onClick={() => setCloudPlatform(guide.key)}
            >
              <span className="cli-tool-name">{guide.label}</span>
              {guide.rankLabel && <span className="cli-tool-rank">{guide.rankLabel}</span>}
            </button>
          ))}
        </div>

        <details className="cli-other-tools" open={!activeGuide.rankLabel}>
          <summary>
            <span>{tx('其他工具', 'Other tools')}</span>
            <small>{otherGuides.map((guide) => guide.label).join(' / ')}</small>
          </summary>
          <div className="mcp-platform-tabs cli-tool-tabs cli-other-tool-tabs" role="tablist" aria-label={tx('其他 CLI 工具', 'Other CLI tools')}>
            {otherGuides.map((guide) => (
              <button
                key={guide.key}
                type="button"
                role="tab"
                aria-selected={cloudPlatform === guide.key}
                className={cloudPlatform === guide.key ? 'active' : ''}
                onClick={() => setCloudPlatform(guide.key)}
              >
                <span className="cli-tool-name">{guide.label}</span>
              </button>
            ))}
          </div>
        </details>

        <div className="cli-primary-step" role="tabpanel">
          {useLocalCommand && (
            <div className="cli-install-inline">
              <div className="cli-mini-step-label">{tx('步骤 1', 'Step 1')}</div>
              <div className="cli-install-inline-main">
                <div>
                  <strong>{tx('安装 neu 命令', 'Install the neu command')}</strong>
                  <p>
                    {tx(
                      '终端提示 command not found 时点这里。安装后当前终端可能还需要执行 source ~/.zshrc，页面会给出命令。',
                      'Use this when Terminal says command not found. The current terminal may still need source ~/.zshrc after installation; this page will show the command.',
                    )}
                  </p>
                </div>
                <div className="cli-install-inline-actions">
                  {isTauri ? (
                    <button className="btn btn-primary" type="button" disabled={installingCli} onClick={handleInstallCli}>
                      {installingCli ? tx('安装中...', 'Installing...') : tx('一键安装', 'Install')}
                    </button>
                  ) : (
                    <button className="btn btn-primary" type="button" onClick={() => copyToClipboard(sourceInstallCommand, 'cli-source-install')}>
                      {copied === 'cli-source-install' ? tx('已复制', 'Copied') : tx('复制安装命令', 'Copy install command')}
                    </button>
                  )}
                  <Link className="btn btn-outline" to="/cli">{tx('命令行工具页', 'Command Line page')}</Link>
                </div>
              </div>
              {cliInstallResult && (
                <div className="alert alert-success cli-install-inline-result">
                  <span>{tx(`已安装到 ${cliInstallResult.install_dir}`, `Installed into ${cliInstallResult.install_dir}`)}</span>
                  {cliInstallResult.shell_reload_command && (
                    <span>
                      {tx('当前终端继续报错时执行：', 'If the current terminal still errors, run:')}
                      <code>{cliInstallResult.shell_reload_command}</code>
                      <button className="btn btn-sm" type="button" onClick={() => copyToClipboard(cliInstallResult.shell_reload_command || '', 'cli-source-reload')}>
                        {copied === 'cli-source-reload' ? tx('已复制', 'Copied') : tx('复制', 'Copy')}
                      </button>
                    </span>
                  )}
                </div>
              )}
              {cliInstallError && <div className="alert alert-error">{cliInstallError}</div>}
            </div>
          )}

          <div className="cli-primary-head">
            <div>
              {useLocalCommand && <div className="cli-mini-step-label">{tx('步骤 2', 'Step 2')}</div>}
              <div className="cli-primary-title-row">
                <strong>{activeGuide.label}</strong>
                {activeGuide.rankLabel && <span>{activeGuide.rankLabel}</span>}
              </div>
              <p>{primaryIntro}</p>
            </div>
            {!useLocalCommand && <code>{baseUrl}/mcp</code>}
          </div>

          {activeGuide.advancedConfig && !useLocalCommand && (
            <div className="cli-secondary-copy">
              <div>
                <span>{activeGuide.advancedConfig.title}</span>
                <p>{tx('Cursor 需要先保存这段配置。', 'Cursor needs this config saved first.')}</p>
              </div>
              <div className={commandClassName(activeGuide.advancedConfig.content)}>
                <code>{activeGuide.advancedConfig.content}</code>
                <button className="btn btn-primary" onClick={() => copyToClipboard(activeGuide.advancedConfig?.content || '', activeGuide.advancedConfig?.copyKey || '')}>
                  {copied === activeGuide.advancedConfig.copyKey ? tx('已复制', 'Copied') : tx('复制配置', 'Copy config')}
                </button>
              </div>
            </div>
          )}

          <div className={commandClassName(primaryCommand)}>
            <code>{primaryCommand}</code>
            <button className="btn btn-primary" onClick={() => copyToClipboard(primaryCommand, primaryCopyKey)}>
              {copied === primaryCopyKey ? tx('已复制', 'Copied') : primaryCopyLabel}
            </button>
          </div>

          <ol className="cli-plain-steps">
            {(useLocalCommand
              ? [
                tx('打开新的终端窗口，粘贴命令并回车。', 'Open a new Terminal window, paste the command, and press Enter.'),
                tx('看到 Connected 后，重新打开对应 AI 工具。', 'When you see Connected, reopen the AI tool.'),
                tx('在对话里输入 Vola 命令，例如让它读取 profile 或 memory。', 'In chat, use a Vola command, for example asking it to read profile or memory.'),
              ]
              : activeGuide.afterCopy
            ).map((item) => <li key={item}>{item}</li>)}
          </ol>
        </div>
      </article>

      <div className="cli-secondary-grid">
        <details className="wizard-card mcp-step-card cli-disclosure">
          <summary>
            <span>{tx('完成后检查连接', 'Check connection after setup')}</span>
            <small>{useLocalCommand ? tx('可选', 'Optional') : activeGuide.verifyCommand || tx('可选', 'Optional')}</small>
          </summary>
          <div className="cli-disclosure-body">
            <p>
              {useLocalCommand
                ? tx('不确定是否成功时，运行平台状态命令。', 'If you are not sure it worked, run the platform status command.')
                : tx('授权完成后，可以运行下面的检查命令。', 'After authorization, you can run the check command below.')}
            </p>
            <div className="mcp-copy-row">
              <code>{useLocalCommand ? `neu platform show ${activeGuide.localPlatform}` : activeGuide.verifyCommand || cursorAgentStatusCommand}</code>
              <button
                className="btn btn-primary"
                onClick={() => copyToClipboard(
                  useLocalCommand ? `neu platform show ${activeGuide.localPlatform}` : activeGuide.verifyCommand || cursorAgentStatusCommand,
                  useLocalCommand ? `cli-local-check-${activeGuide.key}` : activeGuide.verifyCopyKey || 'cli-check',
                )}
              >
                {copied === (useLocalCommand ? `cli-local-check-${activeGuide.key}` : activeGuide.verifyCopyKey || 'cli-check') ? tx('已复制', 'Copied') : tx('复制命令', 'Copy command')}
              </button>
            </div>
          </div>
        </details>

        <details className="wizard-card mcp-step-card cli-disclosure">
          <summary>
            <span>{tx('遇到问题再看', 'If something fails')}</span>
            <small>{tx('命令找不到、没有弹浏览器', 'Command not found, no browser opened')}</small>
          </summary>
          <div className="cli-disclosure-body">
            <ul className="cli-note-list">
              {useLocalCommand && (
                <>
                  <li>{tx('终端提示找不到 neu 时，先到“命令行工具”页安装 CLI。', 'If Terminal says neu is not found, install the CLI from the Command Line Tools page first.')}</li>
                  <li>{tx('已经安装但当前终端还报错时，打开新的终端窗口，或执行命令行工具页显示的 source 命令。', 'If it is installed but the current Terminal still errors, open a new Terminal window or run the source command shown on the Command Line Tools page.')}</li>
                  <li>{tx('不想安装 CLI 时，可以去“本地模式”页手动复制配置。', 'If you do not want to install the CLI, use the Local Mode page and copy the config manually.')}</li>
                </>
              )}
              {activeGuide.troubleshooting.map((note) => <li key={note}>{note}</li>)}
            </ul>
            {useLocalCommand && (
              <div className="cli-help-actions">
                <Link className="btn btn-outline" to="/cli">{tx('打开命令行工具', 'Open Command Line Tools')}</Link>
                <Link className="btn btn-outline" to="/setup/local">{tx('打开本地模式', 'Open Local Mode')}</Link>
              </div>
            )}
          </div>
        </details>
      </div>
    </section>
  )
}
