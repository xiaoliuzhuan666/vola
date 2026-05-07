import { useI18n } from '../../i18n'
import { useSetup } from '../SetupPage'
import { SetupCodeBlock, SetupSection } from './SetupShared'

export default function SetupAdaptersPage() {
  const { tx } = useI18n()
  const { baseUrl, cloudModeNeedsPublicUrl, copied, copyToClipboard } = useSetup()
  const callbackUrl = `${baseUrl}/api/adapters/feishu/<your-slug>/events`

  return (
    <SetupSection
      icon={<>&#128279;</>}
      title={tx('适配器', 'Adapters')}
      description={tx('通过 webhook / bot / 事件回调接入飞书、钉钉、Slack 等工作区平台。', 'Connect workspace platforms such as Feishu, DingTalk, and Slack through webhooks, bots, and event callbacks.')}
      highlight
    >
      {cloudModeNeedsPublicUrl && (
        <div className="alert alert-warn">
          {tx('当前地址是 ', 'Current address: ')}<code>{baseUrl}</code>{tx('。飞书回调必须使用可公开访问的 HTTPS 地址；如果你现在在本地开发，请先切到公网域名或隧道地址。', '. Feishu callbacks require a publicly reachable HTTPS address. If you are developing locally, switch to a public domain or tunnel first.')}
        </div>
      )}

      <h4 className="setup-platform-title">Feishu Bot Adapter</h4>
      <p className="setup-note setup-note-first">
        {tx('完成请求网址校验后，飞书发来的事件会进入 neuDrive 的结构化事件记录。', 'After request URL verification, Feishu events are written into neuDrive structured event records.')}
      </p>

      <SetupCodeBlock
        label="Feishu Callback URL"
        content={callbackUrl}
        copied={copied}
        copyKey="adapter-feishu-callback"
        onCopy={copyToClipboard}
      />

      <SetupCodeBlock
        label="Server Environment Variables"
        content={[
          'FEISHU_APP_ID=replace-with-your-app-id',
          'FEISHU_APP_SECRET=replace-with-your-app-secret',
          'FEISHU_VERIFICATION_TOKEN=replace-with-your-verification-token',
          'FEISHU_ENCRYPT_KEY=replace-with-your-encrypt-key',
        ].join('\n')}
        copied={copied}
        copyKey="adapter-feishu-env"
        onCopy={copyToClipboard}
      />

      <ol className="setup-steps">
        <li>{tx('在飞书开放平台创建一个自建应用，并启用 ', 'Create a custom app in Feishu Open Platform and enable ')}<code>{tx('机器人能力', 'Bot capability')}</code>{tx('。', '.')}</li>
        <li>{tx('进入 ', 'Open ')}<code>{tx('事件与回调', 'Events and callbacks')}</code>{tx('，订阅 ', ' and subscribe to ')}<code>{tx('消息与群组 - 接收消息 v2.0', 'Messages and groups - receive messages v2.0')}</code>{tx('。', '.')}</li>
        <li>{tx('订阅方式选择 ', 'Set subscription delivery to ')}<code>{tx('将事件发送至开发者服务器', 'Send events to developer server')}</code>{tx('。', '.')}</li>
        <li>{tx('请求网址填写 ', 'Set the request URL to ')}<code>{callbackUrl}</code>{tx('，把其中的 ', ', replacing ')}<code>&lt;your-slug&gt;</code>{tx(' 换成 neuDrive 用户 slug。', ' with your neuDrive user slug.')}</li>
        <li>{tx('把飞书应用里的 ', 'Map the Feishu app values ')}<code>App ID</code>{tx('、', ', ')}<code>App Secret</code>{tx('、', ', and ')}<code>Verification Token</code>{tx(' 配到服务端环境变量 ', ' to the server environment variables ')}<code>FEISHU_APP_ID</code>{tx('、', ', ')}<code>FEISHU_APP_SECRET</code>{tx('、', ', and ')}<code>FEISHU_VERIFICATION_TOKEN</code>{tx('。', '.')}</li>
        <li>{tx('推荐同时开启加密推送，并把 ', 'We also recommend enabling encrypted delivery and setting ')}<code>Encrypt Key</code>{tx(' 配到 ', ' as ')}<code>FEISHU_ENCRYPT_KEY</code>{tx('；neuDrive 已支持飞书要求的签名校验和事件解密。', '; neuDrive already supports Feishu signature verification and event decryption.')}</li>
        <li>{tx('保存配置后，飞书会先发起 ', 'After saving, Feishu sends a ')}<code>challenge</code>{tx(' 验证；验证通过后，后续消息事件就会被 neuDrive 接收。', ' verification request first. After it passes, neuDrive receives future message events.')}</li>
      </ol>

      <p className="setup-note">
        {tx('飞书消息会写入 neuDrive 的事件记录；配置 ', 'Feishu messages are written into neuDrive event records. After ')}<code>FEISHU_APP_ID</code> / <code>FEISHU_APP_SECRET</code>{tx(' 后，neuDrive 会自动回复一条确认消息。文本消息会提取正文，非文本消息会保留结构化内容。', ' are configured, neuDrive can automatically reply with a confirmation message. Text messages extract the body; non-text messages keep structured content.')}
      </p>
    </SetupSection>
  )
}
