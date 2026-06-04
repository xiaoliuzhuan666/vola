import { useState } from 'react'
import { useI18n } from '../i18n'

type CopyKey = 'login' | 'daily' | 'import' | 'backup'

const loginCommand = `neu login --api-base https://your-vola.example`

const dailyCommands = `neu status
neu platform ls
neu connect claude
neu browse`

const importCommands = `neu import claude --dry-run
neu import codex --dry-run
neu import skill ./demo-skill
neu import memory ./notes`

const backupCommands = `neu git init --output ./vola-export/git-mirror
neu git pull
neu sync pull --format archive --output vola-backup.ndrvz`

function CommandBlock({
  title,
  description,
  command,
  copyKey,
  copied,
  onCopy,
}: {
  title: string
  description: string
  command: string
  copyKey: CopyKey
  copied: CopyKey | ''
  onCopy: (key: CopyKey, command: string) => void
}) {
  const { tx } = useI18n()
  return (
    <article className="cli-command-item">
      <div className="cli-command-item-head">
        <div>
          <h3 className="card-title">{title}</h3>
          <p className="materials-section-copy">{description}</p>
        </div>
        <button className="btn btn-sm" type="button" onClick={() => onCopy(copyKey, command)}>
          {copied === copyKey ? tx('已复制', 'Copied') : tx('复制', 'Copy')}
        </button>
      </div>
      <pre className="cli-command-code"><code>{command}</code></pre>
    </article>
  )
}

export default function CommandLineToolsPage() {
  const { tx } = useI18n()
  const [copied, setCopied] = useState<CopyKey | ''>('')

  const copyCommand = async (key: CopyKey, command: string) => {
    try {
      await navigator.clipboard?.writeText(command)
      setCopied(key)
      window.setTimeout(() => setCopied((current) => current === key ? '' : current), 1500)
    } catch {
      setCopied('')
    }
  }

  return (
    <div className="page materials-page cli-tools-page">
      <section className="materials-hero cli-tools-hero">
        <div className="materials-hero-copy">
          <div className="materials-kicker">neu CLI</div>
          <h2 className="materials-title">{tx('命令行工具', 'Command Line Tools')}</h2>
          <p className="materials-subtitle">
            {tx(
              '使用 neu 在终端里登录官方云服务、检查本地运行状态、连接 AI 工具、导入本地数据，并把 Vola 内容备份出来。',
              'Use neu from your terminal to sign in to the hosted service, check local status, connect AI tools, import local data, and back up Vola content.',
            )}
          </p>
        </div>
      </section>

      <section className="materials-panel cli-command-panel">
        <div className="cli-command-panel-head">
          <div>
            <h3 className="card-title">{tx('常用命令', 'Common commands')}</h3>
            <p className="materials-section-copy">
              {tx('从安装、登录到导入和备份，按需要复制对应命令。', 'Copy the command you need, from install and login to import and backup.')}
            </p>
          </div>
        </div>
        <div className="cli-command-list">
        <CommandBlock
          title={tx('登录 Vola 服务', 'Sign in to Vola')}
          description={tx('把示例地址替换成你的部署域名；登录完成后 CLI 会保存当前 profile。', 'Replace the example URL with your deployment domain. After sign-in, the CLI stores the current profile.')}
          command={loginCommand}
          copyKey="login"
          copied={copied}
          onCopy={copyCommand}
        />
        <CommandBlock
          title={tx('日常检查和连接', 'Daily checks and connection')}
          description={tx('检查本地运行状态，查看可连接的平台，并快速配置 Claude 等工具。', 'Check local status, list available platforms, and quickly configure tools such as Claude.')}
          command={dailyCommands}
          copyKey="daily"
          copied={copied}
          onCopy={copyCommand}
        />
        <CommandBlock
          title={tx('导入本地数据', 'Import local data')}
          description={tx('在本地模式下扫描 Claude、Codex、skills、memory 等内容；正式导入前推荐先 dry-run。', 'In local mode, scan Claude, Codex, skills, memory, and related content. Run dry-run before writing data.')}
          command={importCommands}
          copyKey="import"
          copied={copied}
          onCopy={copyCommand}
        />
        <CommandBlock
          title={tx('备份和同步', 'Backup and sync')}
          description={tx('初始化 GitHub 备份、刷新 Git mirror，或拉取 archive 备份包。', 'Initialize GitHub backup, refresh the Git mirror, or pull an archive backup bundle.')}
          command={backupCommands}
          copyKey="backup"
          copied={copied}
          onCopy={copyCommand}
        />
        </div>
      </section>
    </div>
  )
}
