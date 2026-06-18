# Vola 桌面端发布与自动更新

## 当前状态

已接入 Tauri updater 的仓库配置：

- `src-tauri/tauri.conf.json` 开启 `bundle.createUpdaterArtifacts`，并配置 GitHub Release `latest.json` endpoint。
- `src-tauri` 已注册 `tauri-plugin-updater` 和 `tauri-plugin-process`。
- `web/src/pages/InfoPage.tsx` 在桌面端显示“检查更新”入口，可下载并安装签名更新包，安装后提示重启。
- `.github/workflows/release.yml` 会在 tag 发布时构建 CLI 二进制、桌面安装包、签名 updater artifacts、`latest.json` 和 `SHA256SUMS.txt`，并发布到 GitHub Release。
- 桌面端 Release 资产会带平台前缀，例如 `macos-aarch64-vola.app.tar.gz`，避免不同 matrix 产物重名，并让 `latest.json` 能识别平台。

## 本机签名 key

本机已生成 updater 签名 key：

- 私钥：`~/.tauri/vola-updater.key`
- 公钥：`~/.tauri/vola-updater.key.pub`

私钥不能提交到仓库。当前公钥已经写入 `src-tauri/tauri.conf.json`。

## GitHub Actions secret

远端仓库需要配置：

- `TAURI_SIGNING_PRIVATE_KEY`：填入 `~/.tauri/vola-updater.key` 的完整内容。
- `TAURI_SIGNING_PRIVATE_KEY_PASSWORD`：本次 key 未设置密码，可以不配置或留空。

当前已通过 GitHub 网页给 `xiaoliuzhuan666/vola` 配置 `TAURI_SIGNING_PRIVATE_KEY`。后续如需轮换 key，安装并登录 GitHub CLI 后可执行：

```bash
gh secret set TAURI_SIGNING_PRIVATE_KEY --repo xiaoliuzhuan666/vola < ~/.tauri/vola-updater.key
```

## 发布流程

1. 确认 `src-tauri/tauri.conf.json` 中的 `version` 是要发布的版本。
2. 提交并推送代码。
3. 创建匹配版本的 tag，例如 `v0.1.0`。
4. GitHub Actions 的 `Release` workflow 会自动构建并发布 Release。
5. Release 里必须包含 `latest.json`；桌面端 updater endpoint 指向：

```text
https://github.com/xiaoliuzhuan666/vola/releases/latest/download/latest.json
```

## 验证

发布后检查：

- GitHub Release 是非 draft、非 prerelease。
- Release 资产中包含 `latest.json`。
- `latest.json` 的 `version` 与桌面端版本一致。
- `latest.json.platforms` 至少包含当前系统对应平台，并且对应资产 URL 和 signature 存在。
- 已安装旧版桌面端点击“检查更新”后能下载、安装并在重启后切到新版本。

## 2026-06-16 v0.1.3 GitHub 打包记录

本次按 GitHub tag 发布流程重新打包桌面端：

- 提交：`2f520532e07be957fd044f4844ec3ba30bf16fcf`
- tag：`v0.1.3`
- Release workflow run：`27597924824`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.3`
- workflow 结果：success

本地发版前验证：

- `npm --prefix web run build`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`：通过。
- `docker compose config --services`：通过。
- `deploy/tencent/docker-compose.yml` 使用 dummy 必填环境变量做 `config --services`：通过。
- `bash -n deploy/prod/deploy.sh deploy/tencent/pull-and-deploy.sh`：通过。
- `git diff --check`：通过。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.3`。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.3_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.3_x64.dmg`
- `linux-x86_64-vola_0.1.3_amd64.AppImage`
- `windows-x86_64-vola_0.1.3_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.3`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，未影响本次打包。后续可以把相关 action 或 runner 环境升级到 Node.js 24。

## 2026-06-16 v0.1.4 GitHub 打包记录

本次按 GitHub tag 发布流程重新打包桌面端，并把连接流程简化、能力核对文档和桌面端版本一并发布：

- 提交：`2d20176248f3064cda323811110a769541007561`
- tag：`v0.1.4`
- Release workflow run：`27603282948`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27603282948`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.4`
- workflow 结果：success

本地发版前验证：

- `npm --prefix web run build`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`：通过。
- `docker compose config --services`：通过。
- `bash -n deploy/prod/deploy.sh deploy/tencent/pull-and-deploy.sh`：通过。
- `make build`：通过。
- `git diff --check`：通过。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.4`。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.4_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.4_x64.dmg`
- `linux-x86_64-vola_0.1.4_amd64.AppImage`
- `windows-x86_64-vola_0.1.4_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.4`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/upload-artifact@v4`，未影响本次打包。后续可以把相关 action 或 runner 环境升级到 Node.js 24。

## 2026-06-16 v0.1.5 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点是让不会安装 CLI 的用户可以在桌面版里一键安装 `neu`，并把连接页调整为“先安装 neu，再连接 Codex / Claude Code”。

本地发版前验证：

- `cargo check --manifest-path src-tauri/Cargo.toml`：通过。
- `npm --prefix web run typecheck`：通过。
- `npm --prefix web run build`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`：通过。
- `make build`：通过。
- `git diff --check`：通过。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.5`。

发布完成后回填：

- 提交：`88bfb812c77848226d633df1e4f4ddd27dd0337e`
- tag：`v0.1.5`
- Release workflow run：`27606558923`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27606558923`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.5`
- workflow 结果：success

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.5_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.5_x64.dmg`
- `linux-x86_64-vola_0.1.5_amd64.AppImage`
- `windows-x86_64-vola_0.1.5_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.5`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/download-artifact@v4` 和 `softprops/action-gh-release@v2`，未影响本次打包。

## 2026-06-16 v0.1.6 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点修复新版桌面包打开后停在“本地数据服务暂时没有响应”的问题，并让桌面端首页直接进入 Codex Console。

问题含义：

- 桌面窗口已经打开，但前端没有连上本地 Vola 数据服务。
- 本机复查发现一个旧 runtime 地址可能指向已经停止的端口；同时旧的包内 `vola` 后端程序没有包含最新 Codex Console API，导致 `/api/local/codex-console` 不可用。

本次修复：

- 桌面端启动后优先使用当前进程刚启动的 API 地址，不再优先信任旧 runtime 文件。
- 旧 runtime 文件只作为备用来源，并且必须通过 `/api/config` 连通检查。
- 桌面端自动选择 `42690..42719` 中可用端口启动本地服务。
- 桌面端启动本地服务时显式传入 `--listen` 和 `--public-base-url`。
- 打包环境优先使用 `.app/Contents/Resources/bin/vola`，避免误用仓库里的旧 `./bin/vola`。
- Tauri `beforeBuildCommand` 现在会在构建前重新编译包内 Go 后端，避免本机和 GitHub Actions 打出来的桌面包带上旧后端。
- 桌面端本地模式打开后直接进入 `tauri://localhost/codex-console`，侧边栏首项为 Codex Console。

本地发版前验证：

- `cargo fmt --manifest-path src-tauri/Cargo.toml`：通过。
- `npm --prefix web run build`：通过。
- `cargo check --manifest-path src-tauri/Cargo.toml`：通过。
- `TAURI_ENV_TARGET_TRIPLE=aarch64-apple-darwin node scripts/tauri-web-command.mjs build`：通过，生成 `src-tauri/bin/vola`，文件类型为 macOS arm64 Mach-O。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`：通过。
- `git diff --check`：通过。
- 本机 Tauri `.app` 构建：通过，产物包含 `vola.app` 和签名 updater 包 `vola.app.tar.gz`。
- 包内后端程序检查：`vola.app/Contents/Resources/bin/vola` 为 macOS arm64 Mach-O，时间为 2026-06-16 22:08:02 CST。
- 直接启动新 `.app`：通过，进程从 `vola.app/Contents/Resources/bin/vola server --local-mode --listen 127.0.0.1:42690 --storage sqlite --public-base-url http://127.0.0.1:42690` 拉起后端。
- `GET /api/config`：通过，返回 `local_mode: true`。
- `POST /api/local/owner-token` 后带 token 请求 `GET /api/local/codex-console`：通过，返回 `threads: 397`、`skill_candidates: 33`、`memory_candidates: 62`、`overview: true`。
- Computer Use 验证桌面窗口：通过，当前 URL 为 `tauri://localhost/codex-console`，页面显示 Codex Console，不再显示“本地数据服务暂时没有响应”。

发布完成后回填：

- 提交：`7378e574ecd4b569f97e4c60cffa27b3f5c671ee`
- tag：`v0.1.6`
- Release workflow run：`27624170386`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27624170386`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.6`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`：已上传，`latest.json` 版本为 `0.1.6`，包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64` 四个平台的签名和下载地址。
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.6_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.6_x64.dmg`
- `linux-x86_64-vola_0.1.6_amd64.AppImage`
- `windows-x86_64-vola_0.1.6_x64-setup.exe`

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/upload-artifact@v4`，未影响本次打包。

## 2026-06-17 v0.1.7 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点是让团队 Skill / MCP 更容易同步到本机 Codex 和 Claude Code。

本次更新：

- Team Library 新增本机同步入口：团队 Skill 可预览后应用到本机，Cursor 和 Gemini CLI 继续导出。
- Team Library 的团队 MCP 区域新增“同步到 Codex / Claude Code”入口。
- MCP Hub 新增本机同步入口，可刷新 Codex 和 Claude Code 本机连接。
- 后端新增 `POST /api/local/platform/connection/refresh`，复用现有连接、安全路径和配置锁机制，只允许 Codex 与 Claude Code 自动刷新。
- 修复开发环境 `/mcp` 代理误匹配 `/mcp-hub` 的问题。

本地发版前验证：

- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api ./internal/platforms ./internal/mcp`：通过。
- `npm --prefix web run build`：通过。
- `git diff --check`：通过。
- 本地浏览器验证 `/team` 和 `/mcp-hub`：通过，console error 为 0。

发布完成后回填：

- 提交：`f9947a42a62dc01640e87fd4b0d6543b4001a283`
- tag：`v0.1.7`
- Release workflow run：`27655621463`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27655621463`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.7`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.7_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.7_x64.dmg`
- `linux-x86_64-vola_0.1.7_amd64.AppImage`
- `windows-x86_64-vola_0.1.7_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.7`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/upload-artifact@v4`，未影响本次打包。

## 2026-06-17 v0.1.8 GitHub 打包记录

本次按 GitHub tag 发布流程重新打包桌面端，重点是继续优化小团队 Agent 资料与 Skill/MCP 共享中心，把 Team Library 和 MCP Hub 的本机同步入口做得更清楚，并保留 Cursor / Gemini CLI 的导出边界。

本次更新：

- 团队 Skill / MCP 页面继续强化本机同步状态与入口。
- 文档补充 Vola 的核心定位：个人和小团队的私有 Agent 资料中心，安全同步到 Codex、Claude Code 等本机工具。
- 继续说明 Cursor / Gemini CLI 以预览和导出为主，不自动改本机配置。

发布完成后回填：

- 提交：`18d807455f0a37716027dcc8cd5a5fddee62d73e`
- tag：`v0.1.8`
- Release workflow run：`27666209036`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27666209036`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.8`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.8_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.8_x64.dmg`
- `linux-x86_64-vola_0.1.8_amd64.AppImage`
- `windows-x86_64-vola_0.1.8_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.8`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/download-artifact@v4`、`actions/upload-artifact@v4` 或 `softprops/action-gh-release@v2`，未影响本次打包。

## 2026-06-17 v0.1.9 GitHub 打包记录

本次按 GitHub tag 发布流程重新打包桌面端，重点修复本地 App Data 导入 Codex/Claude Code 资料时，超大 profile rules 写入单条 profile memory 触发 `memory.UpsertProfile: content exceeds maximum size of 65536 bytes` 的问题。

本次修复：

- Codex/Claude Code 导入到 Vola 时，如果 agent profile rules 小于 64 KiB，仍按原逻辑写入 `/memory/profile/<platform>-agent.md`。
- 如果 profile rules 超过 64 KiB，完整原文改存到 `/platforms/<platform>/agent/profile-rules.md`。
- profile memory 只保存短摘要、原始大小和完整归档路径，避免超过 profile 单条大小限制。
- 旧 SQLite client 导入路径和本地 HTTP API 导入路径都使用同样策略。
- 导入结果会返回 profile path 和 archive path，方便前端或调用方展示真实写入位置。

本地发版前验证：

- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api ./internal/platforms ./internal/mcp`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/storage/sqlite`：通过。
- `npm --prefix web run build`：通过。第一次在临时发布目录执行时因未安装依赖出现 `tsc: command not found`，执行 `npm ci --prefix web` 后重试通过。
- `git diff --check`：通过。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.9`。

发布完成后回填：

- 提交：`e16b7e8d9960606d263a52ea2e2cbead76e96a5e`
- tag：`v0.1.9`
- Release workflow run：`27675563202`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27675563202`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.9`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.9_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.9_x64.dmg`
- `linux-x86_64-vola_0.1.9_amd64.AppImage`
- `windows-x86_64-vola_0.1.9_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.9`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/upload-artifact@v4`，未影响本次打包。
- 本次 tag 基于 `v0.1.8` 发布线创建，避免丢失上一版团队资产本机同步功能；没有把主工作区中的其他未发布改动带入本次发布。

## 2026-06-17 v0.1.11 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点是把 Vola 项目资料变成更适合 AI 协作的日常工作入口：个人/团队资料继续保存在 Vola，需要协作时可以生成到代码仓库的 `docs/ai-context` 目录。

本次更新：

- 项目页新增“项目资料”工作台，可粘贴 Markdown，也可从 Vola 文件树复制 Markdown。
- 项目文件列表里的 Markdown 支持右键“复制到项目资料”，文件卡片也支持拖到项目资料区。
- 支持生成 AI 上下文包，把项目说明、近期记录和选中的资料合成协作用 Markdown。
- 支持生成仓库资料列表，并在本地模式写入指定仓库目录，默认路径为 `docs/ai-context`。
- 后端和 MCP 新增项目资料、上下文包、仓库导出相关接口，Codex 可直接读取和使用同一批项目知识。
- 仓库写入增加路径安全检查：要求仓库目录已存在，拒绝写出仓库范围，拒绝通过 symlink 写入或穿越目录。
- 项目资料区域统一了模块间距，并修正表单输入被通用布局覆盖后挤在一行的问题。

本地发版前验证：

- `npm --prefix web run typecheck`：通过。
- `npm --prefix web run build`：通过。
- `rsync -a --delete web/dist/ internal/web/dist/`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/web ./internal/hubpath ./internal/services ./internal/api ./internal/mcp`：通过。
- `git diff --check`：通过。
- 本地浏览器验证 `/data/projects/ai-dev`：通过，项目资料工作台可见，`backend-api.md` 右键菜单显示“复制到项目资料”。
- 本地浏览器验证仓库写入：通过，测试仓库生成 `docs/ai-context/README.md`、`docs/ai-context/materials/backend-api.md`、`docs/ai-context/context-packs/backend-handoff.md`。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.11`。

发布完成后回填：

- 提交：`122949166bd0daae29aa5a07968e777c493d596d`
- tag：`v0.1.11`
- Release workflow run：`27704806347`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27704806347`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.11`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.11_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.11_x64.dmg`
- `linux-x86_64-vola_0.1.11_amd64.AppImage`
- `windows-x86_64-vola_0.1.11_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.11`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

## 2026-06-18 v0.1.12 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点处理三个用户反馈：项目资料入口太复杂、Codex 本地导入偶发 SQLite 锁冲突、桌面安装包仍显示旧图标。

本次更新：

- 项目资料工作台改成一个主入口：粘贴 Markdown 或拖入本机 `.md` 文件即可保存到 Vola。
- 标题、来源链接、Git 路径、标签和手动 Vola 路径移入高级区域，降低首次使用时的表单负担。
- 仓库同步独立成“同步到项目仓库”区域，默认只需要填写本机仓库根目录；仓库内目录、覆盖开关和上下文包信息放入高级区域。
- 已选资料和上下文包可一键复制路径，便于直接贴给 Codex 或同事。
- Codex / Claude Code 本地导入增加串行导入锁和 SQLite busy/locked 重试；仍然繁忙时返回 409，并显示“Vola 正在保存本地资料，请稍等几秒再试”。
- 更新 `src-tauri/icons`、`desktop/icons`、`web/public` 和浏览器扩展 `extension/icon*.png` 图标资源；Tauri 安装包实际引用的 `src-tauri/icons/32x32.png`、`128x128.png`、`icon.icns`、`icon.ico` 已替换。

本地发版前验证：

- `npm --prefix web run typecheck`：通过。
- `npm --prefix web run build`：通过。
- `rsync -a --delete web/dist/ internal/web/dist/`：通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api ./internal/storage/sqlite`：通过。
- `cargo check --manifest-path src-tauri/Cargo.toml`：通过。
- `TAURI_ENV_TARGET_TRIPLE=aarch64-apple-darwin node scripts/tauri-web-command.mjs build`：通过，生成 `src-tauri/bin/vola`，文件类型为 macOS arm64 Mach-O。
- 本机 Tauri macOS 构建：`.app`、`.dmg` 和 updater tar 包已生成；最后 updater 签名阶段因本机未配置 `TAURI_SIGNING_PRIVATE_KEY` 失败。正式 GitHub Release workflow 使用仓库 secret 完成签名。
- 包内图标检查：`vola.app/Contents/Resources/icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致，`Info.plist` 指向 `icon.icns`，版本为 `0.1.12`。
- 本地浏览器验证 `/data/projects/ui-check`：通过，项目资料主入口改为粘贴 / 拖入 Markdown；桌面和 390px 移动宽度没有控件重叠。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.12`。
- 桌面安装包图标配置复查：`src-tauri/tauri.conf.json` 的 bundle icon 列表指向 `src-tauri/icons`，对应文件已更新。

发布完成后回填：

- 提交：`f6d18239e77995c7a2b9df225983f9da7cfad97e`
- tag：`v0.1.12`
- Release workflow run：`27728076043`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27728076043`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.12`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-vola.app.tar.gz`
- `macos-aarch64-vola_0.1.12_aarch64.dmg`
- `macos-x86_64-vola.app.tar.gz`
- `macos-x86_64-vola_0.1.12_x64.dmg`
- `linux-x86_64-vola_0.1.12_amd64.AppImage`
- `windows-x86_64-vola_0.1.12_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.12`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。

注意：

- v0.1.12 已经确认包内图标文件是新图标，但正式包的 `productName` 仍是小写 `vola`，GitHub Release 资产名也仍是 `vola_0.1.12...`。这会让 macOS / Windows 的应用缓存和历史包更容易混在一起，因此继续发布 v0.1.13 修正桌面包身份。

## 2026-06-18 v0.1.13 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，专门处理安装包身份仍沿用 Tauri 默认值的问题。

本次更新：

- `src-tauri/tauri.conf.json` 的 `productName` 从 `vola` 改为 `Vola`。
- `src-tauri/Cargo.toml` 的包名从默认 `app` 改为 `vola-desktop`，描述和作者同步改成 Vola 项目信息。
- 桌面版本升到 `0.1.13`。

本地发版前验证：

- `cargo check --manifest-path src-tauri/Cargo.toml`：通过。
- `TAURI_ENV_TARGET_TRIPLE=aarch64-apple-darwin node scripts/tauri-web-command.mjs build`：通过，包含 `npm run typecheck` 和 `vite build`。
- 本机 Tauri macOS app 构建：生成 `src-tauri/target/aarch64-apple-darwin/release/bundle/macos/Vola.app` 和 `Vola.app.tar.gz`；最后 updater 签名阶段因本机未配置 `TAURI_SIGNING_PRIVATE_KEY` 失败。正式 GitHub Release workflow 使用仓库 secret 完成签名。
- 包内信息检查：`CFBundleDisplayName` 为 `Vola`，`CFBundleName` 为 `Vola`，`CFBundleExecutable` 为 `vola-desktop`，版本为 `0.1.13`。
- 包内图标检查：`Vola.app/Contents/Resources/icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致。
- `git diff --check`：通过。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.13`。

发布完成后回填：

- 提交：`2d867b77491ec04470bea735a7bebdd6f8528419`
- tag：`v0.1.13`
- Release workflow run：`27731081581`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27731081581`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.13`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-Vola.app.tar.gz`
- `macos-aarch64-Vola_0.1.13_aarch64.dmg`
- `macos-x86_64-Vola.app.tar.gz`
- `macos-x86_64-Vola_0.1.13_x64.dmg`
- `linux-x86_64-Vola_0.1.13_amd64.AppImage`
- `windows-x86_64-Vola_0.1.13_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.13`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。
- macOS updater URL 已指向 `macos-aarch64-Vola.app.tar.gz` 和 `macos-x86_64-Vola.app.tar.gz`。

远端 macOS updater 包复查：

- 下载 `macos-aarch64-Vola.app.tar.gz` 后，压缩包内顶层目录为 `Vola.app/`。
- 包内包含 `Vola.app/Contents/MacOS/vola-desktop` 和 `Vola.app/Contents/Resources/icon.icns`。

注意：

- GitHub Actions 出现 Node.js 20 deprecation 警告，来源为 `actions/upload-artifact@v4`、`actions/download-artifact@v4` 和 `softprops/action-gh-release@v2`，未影响本次打包。

## 2026-06-18 v0.1.14 GitHub 打包记录

本次准备按 GitHub tag 发布流程重新打包桌面端，重点是让正式 GitHub Release 使用新的蓝色 Vola icon。

本次更新：

- `web/public/vola-mark.svg`、`web/public/favicon.svg` 和 `web/public/vola-app-icon.png` 换成明亮蓝紫色系。
- `src-tauri/icons` 重新生成，确保 GitHub Release 的 macOS、Windows、Linux 桌面包使用新版图标。
- `desktop/icons` 同步生成新版图标，避免另一条本地桌面打包路线继续使用旧资源。
- 新增 `docs/vola-app-icon-design.zh-CN.md`，记录 icon 视觉方案、配色、资源生成链路和 DMG 验证方法。
- 桌面版本升到 `0.1.14`。

本地发版前验证：

- `git diff --check`：通过。
- `npm ci`（`web/`）：通过。第一次执行 `npm --prefix web run build` 因干净 worktree 未安装依赖失败，报 `tsc: command not found`；安装依赖后重试通过。
- `TAURI_ENV_TARGET_TRIPLE=aarch64-apple-darwin node scripts/tauri-web-command.mjs build`：通过，包含 `npm run typecheck`、`vite build` 和后端 sidecar 生成。
- `cargo check --manifest-path src-tauri/Cargo.toml`：第一次因干净 worktree 还没有 `src-tauri/bin/vola` 失败，报 `resource path bin/vola doesn't exist`；生成 sidecar 后重试通过。
- 本机 Tauri macOS app / dmg 构建：`cd src-tauri && ../web/node_modules/.bin/tauri build --bundles app dmg --no-sign` 通过，生成 `Vola.app`、`Vola_0.1.14_aarch64.dmg` 和 updater tar 包。
- 本地签名检查：`codesign --verify --deep --strict --verbose=2 src-tauri/target/release/bundle/macos/Vola.app` 失败，报 `code has no resources but signature indicates they must be present`。本地构建使用了 `--no-sign`，正式 GitHub Release workflow 使用仓库 secret 完成签名和 updater 签名。
- 包内信息检查：`CFBundleDisplayName` 为 `Vola`，`CFBundleName` 为 `Vola`，`CFBundleExecutable` 为 `vola-desktop`，版本为 `0.1.14`。
- 包内图标检查：`Vola.app/Contents/Resources/icon.icns`、DMG 外层 `icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致。
- DMG 内部检查：挂载 `Vola_0.1.14_aarch64.dmg` 后，`Vola.app/Contents/Resources/icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致，版本为 `0.1.14`。
- `web/public/vola-app-icon.png` 和 `web/dist/vola-app-icon.png` 的 SHA-256 一致。
- `src-tauri/tauri.conf.json`、`src-tauri/Cargo.toml`、`src-tauri/Cargo.lock` 版本均为 `0.1.14`。

发布完成后回填：

- 提交：`d840149dbcce54bbb519e4ebb0794115ac7982a1`
- tag：`v0.1.14`
- Release workflow run：`27736356747`
- Actions 页面：`https://github.com/xiaoliuzhuan666/vola/actions/runs/27736356747`
- Release 页面：`https://github.com/xiaoliuzhuan666/vola/releases/tag/v0.1.14`
- workflow 结果：通过，`Desktop macos-aarch64`、`Desktop macos-x86_64`、`Desktop linux-x86_64`、`Desktop windows-x86_64` 和 `Publish GitHub release` 均为 success。

GitHub Release 资产：

- `latest.json`
- `macos-aarch64-Vola.app.tar.gz`
- `macos-aarch64-Vola_0.1.14_aarch64.dmg`
- `macos-x86_64-Vola.app.tar.gz`
- `macos-x86_64-Vola_0.1.14_x64.dmg`
- `linux-x86_64-Vola_0.1.14_amd64.AppImage`
- `windows-x86_64-Vola_0.1.14_x64-setup.exe`

`latest.json` 复查：

- `version` 为 `0.1.14`。
- `platforms` 包含 `darwin-aarch64`、`darwin-x86_64`、`linux-x86_64`、`windows-x86_64`。
- 四个平台记录均包含资产 URL 和 signature。
- macOS updater URL 已指向 `macos-aarch64-Vola.app.tar.gz` 和 `macos-x86_64-Vola.app.tar.gz`。
- 下载 `macos-aarch64-Vola_0.1.14_aarch64.dmg` 后，包内 `Vola.app/Contents/Resources/icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致，版本为 `0.1.14`。
- 下载 `macos-aarch64-Vola.app.tar.gz` 后，包内 `Vola.app/Contents/Resources/icon.icns` 与 `src-tauri/icons/icon.icns` 的 SHA-256 一致，版本为 `0.1.14`。
