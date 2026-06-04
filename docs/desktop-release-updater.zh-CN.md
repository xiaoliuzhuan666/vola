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
