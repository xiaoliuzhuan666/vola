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
