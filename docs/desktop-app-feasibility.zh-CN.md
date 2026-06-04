# Vola 桌面端可行性评估

## 判断

可行。线上网页不能扫描用户电脑目录，这是浏览器安全边界；桌面端应用运行在用户本机，可以在用户授权和系统权限允许的范围内读取本地文件。Vola 当前已经有本地模式和本地 App Data 扫描接口，所以桌面端不需要重写数据解析，只需要负责启动本地服务、打开窗口、处理权限与生命周期。

## cockpit-tools 的实现参考

`cockpit-tools` 使用 Tauri 2：

- 前端是 React/Vite。
- 本地能力集中在 Rust/Tauri 层，通过 `invoke` 暴露给页面。
- Tauri 配置里声明窗口、插件、能力权限、打包资源和 updater。
- 本地读取逻辑按平台判断路径，例如 macOS 的 `~/Library/Application Support/...`、Windows 的 `%APPDATA%`、Linux 的 `$XDG_CONFIG_HOME` 或 `~/.config`。
- 敏感操作放在本机进程里执行，网页端只拿结构化结果。

这套思路适合 Vola，但 Vola 已经有 Go 后端、SQLite、本地导入和 MCP 接口，没必要把扫描器搬到 Rust。

## cockpit-tools 的更新与窗口参考

这次对照 `jlcodes99/cockpit-tools` 当前主线后，能确认它的桌面发布链路有两部分值得参考：

- 自动更新：Tauri 配置里开启 `bundle.createUpdaterArtifacts`，配置 updater `pubkey` 和 GitHub Release 的 `latest.json` endpoint；前端用 `@tauri-apps/plugin-updater` 的 `check()` 检测版本，下载完成后用 `@tauri-apps/plugin-process` 触发重启。
- 桌面窗口：主窗口是 1280 x 800，最小 900 x 600，默认居中打开。

Vola 现在不能直接把 updater 配置写死。自动覆盖安装需要一套真实发布资产：Tauri 签名密钥、公开校验公钥、GitHub Release 中的 `latest.json`、各平台安装包和签名文件。缺其中任一项，用户点击更新就会变成失败弹窗。当前可先做窗口体验修正，并把 updater 作为发布链路任务接入。

## Vola 推荐架构

第一阶段采用“桌面壳 + Go sidecar”：

- Tauri 负责桌面窗口、应用生命周期和打包。
- Go sidecar 运行 `vola server --local-mode --storage sqlite`。
- 页面继续使用现有 Web UI。
- 访问入口改成 `http://127.0.0.1:<port>/?local_token=...&desktop=1`，由本地服务签发 owner token。
- 本地扫描继续走 `/api/local/platform/preview-task` 和 `/api/local/platform/import`。

这样做的好处：

- 保留现有鉴权、SQLite、团队资料、Skill、会话导入逻辑。
- 不重复维护 Rust 和 Go 两套扫描规则。
- 后续可以逐步把“选择目录”“系统托盘”“自动启动”“原生通知”加到 Tauri 层。

## 能扫描什么

桌面端可以扫描：

- `~/.claude` 下的 Claude Code 资料、会话、skills、commands、agents、memory。
- `~/.codex` 和 `~/.agents/skills` 下的 Codex 资料、会话、skills、rules、memories、automations。
- 后续可以扩展 Cursor、Windsurf、Gemini CLI 等本地配置目录。

但仍然不应该“静默扫全盘”。合理设计是：

- 默认只扫描明确支持的 AI 工具目录。
- 页面展示扫描来源、数量、敏感项提示。
- 用户点击导入后才写入 Vola。
- 如果要读取任意目录，必须让用户选择目录，并保留清晰的来源记录。

## 当前分支实现状态

已创建 `codex/vola-desktop-app` 分支，并加入第一版 Tauri 桌面工程：

- `desktop/tauri.conf.json`
- `desktop/Cargo.toml`
- `desktop/src/lib.rs`
- `desktop/scripts/build-backend-sidecar.mjs`
- `src-tauri/tauri.conf.json` 的主窗口已调到 1280 x 820，最小 900 x 600，并默认居中，避免通过 `make desktop-dev` 打开的旧入口仍是 800 x 600。

第一版目标是验证“桌面窗口启动 Vola 本地服务并进入可扫描页面”这条链路。当前已经加入 Tauri updater 配置、发布 workflow 和桌面端检查更新入口；GitHub Actions secret 仍需在远端仓库配置。还没有做 macOS 公证、托盘常驻和任意目录选择器。

## 当前验证状态

已通过：

- `cd desktop && npm install`
- `cd desktop && npm run build:backend`
- 授权运行 `desktop/sidecars/vola-x86_64-apple-darwin server --local-mode --listen 127.0.0.1:42735 --storage sqlite --public-base-url http://127.0.0.1:42735`
- `GET http://127.0.0.1:42735/api/health`
- `POST http://127.0.0.1:42735/api/local/owner-token`
- `POST /api/local/platform/preview-task` 扫描 Claude Code：成功，真实返回 1 条 profile rule、4 个 projects、1 个 bundles、44 条 conversations、82 个 agent artifacts。
- `POST /api/local/platform/preview-task` 扫描 Codex：成功，真实返回 3 条 profile rules、55 条 memories、25 个 projects、12 个 bundles、160 条 conversations、139 个 agent artifacts。
- `go test ./internal/platforms -run 'TestScanCodexJSONLLinesSkipsOversizedLine|TestParseCodexSessionFileSkipsOversizedConversation|TestSummarizeCodexSkippedSessionsLimitsExamples|TestPreviewImportCodexIncludesBundlesAndConversations|TestScanLocalCodexMigrationBuildsStructuredInventory' -count=1`
- `cd src-tauri && cargo check`：清理旧 `target` 缓存后通过。旧缓存里曾残留 `/Users/zhongmoshu/Desktop/work/neuDrive/...` 绝对路径，导致第一次检查失败。

这次验证还修了一个真实问题：本机 Codex 会话里存在 16 MB 以上的 JSONL 文件，原扫描逻辑会因为 `bufio.Scanner: token too long` 失败。现在逻辑会跳过超大的会话全文，只保留会话文件在 inventory 里的来源记录，并在预览 notes 里给一条汇总说明，避免页面被几十条长路径刷屏。

未通过：

- `cd desktop && cargo check`
- `cd desktop && /opt/homebrew/bin/cargo check --offline`

原因不是 Vola 业务代码已经明确失败，而是当前机器有两套 Homebrew Rust，并且 Tauri 依赖没有完整下载：

- `/usr/local/bin/rustc` 是 `1.81.0`，不支持部分新依赖使用的 Rust 2024 edition。
- `/opt/homebrew/bin/rustc` 已升级到 `1.95.0`，但 Cargo 在刷新 `crates.io` 索引时多次卡住或遇到 DNS 失败。
- 离线检查明确缺 `futures-macro`，它来自 `tauri v2.0.4 -> futures-util v0.3.32`。

下一次继续验证时建议直接使用：

```bash
cd /Users/zhongmoshu/Desktop/work/Vola/desktop
PATH=/opt/homebrew/bin:$PATH /opt/homebrew/bin/cargo check
PATH=/opt/homebrew/bin:$PATH npm run build
```

通过这两步之前，不要把桌面端描述成已经完成或已经可发布。
