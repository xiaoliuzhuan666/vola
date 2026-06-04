# Vola prod-like 同步与备份验收记录

日期：2026-05-13

## 环境

- 工作区：`/Users/zhongmoshu/Desktop/work/Vola`
- Commit：`2f255a154bcdd5ffc1328ddddb8edc5160e3eb12`
- 分支状态：`main...origin/main`
- 主存储：本地 SQLite
- 服务地址：`http://127.0.0.1:42731`
- SQLite 数据库：`/private/tmp/vola-prodlike-20260513/acceptance.db`
- 验收产物目录：`/private/tmp/vola-prodlike-20260513`
- 管理员 token：本地 owner token，仅记录前缀 `ndt_82a321`
- Sync token：`both` 权限，仅记录前缀 `ndt_6f822`
- 生产数据：未连接
- 真实 GitHub / WebDAV / S3 凭据：未提供

首次启动服务失败，原因是 `VAULT_MASTER_KEY` 不是 64 位 hex。改成验收专用 64 位 hex 后，普通沙箱启动仍失败，错误为 `listen tcp 127.0.0.1:42731: bind: operation not permitted`；获得本机端口绑定权限后启动成功。

`/api/health` 返回：

```json
{"ok":true,"data":{"service":"vola","status":"ok","storage":"sqlite"}}
```

## 执行命令

以下验收命令已成功执行：

```bash
/private/tmp/vola-prodlike-20260513/neu sync export --source /private/tmp/vola-prodlike-20260513/fixtures/roundtrip-src --format archive -o /private/tmp/vola-prodlike-20260513/roundtrip.ndrvz
/private/tmp/vola-prodlike-20260513/neu sync preview --bundle /private/tmp/vola-prodlike-20260513/roundtrip.ndrvz --api-base http://127.0.0.1:42731 --token <sync-token>
/private/tmp/vola-prodlike-20260513/neu sync push --bundle /private/tmp/vola-prodlike-20260513/roundtrip.ndrvz --transport auto --api-base http://127.0.0.1:42731 --token <sync-token>
/private/tmp/vola-prodlike-20260513/neu sync pull --format archive -o /private/tmp/vola-prodlike-20260513/roundtrip-pulled.ndrvz --api-base http://127.0.0.1:42731 --token <sync-token>
/private/tmp/vola-prodlike-20260513/neu sync diff --left /private/tmp/vola-prodlike-20260513/roundtrip.ndrvz --right /private/tmp/vola-prodlike-20260513/roundtrip-pulled.ndrvz --format json
/private/tmp/vola-prodlike-20260513/neu sync resume --bundle /private/tmp/vola-prodlike-20260513/resume-large.ndrvz --api-base http://127.0.0.1:42731 --token <sync-token>
go test ./...
cd web && npm run build
make build
```

`make build` 按仓库 Makefile 执行了 `npm ci`、前端构建、`rm -rf internal/web/dist`、复制 `web/dist` 到 `internal/web/dist`，并重新构建 `bin/vola` 和 `bin/neu`。

## 验收矩阵

| 项目 | 结果 | 证据 |
| --- | --- | --- |
| 本地服务健康检查 | yes | `/api/health` 返回 `ok`，`storage: sqlite`。 |
| Sync token 具备 push 和 pull 权限 | yes | `/api/tokens/sync` 返回 scopes：`read:bundle`、`write:bundle`。 |
| Preview 不写 history | yes | preview 前 `neu sync history` 返回 `[]`，preview 后仍为 `[]`。 |
| Archive push 写入 history | yes | history 出现 import job `5ee12842-6567-492e-b3de-dff386952690`，transport 为 `archive`，status 为 `succeeded`。 |
| Archive pull 写入 history | yes | history 出现 export job `59be9ee4-6fbd-4b6c-a585-ceeb73008bca`，transport 为 `archive`，status 为 `succeeded`。 |
| Round-trip 内容 diff | yes | `neu sync diff` 返回 `equal: true`，2 个 skill、5 个文件均 unchanged。 |
| Round-trip archive 字节级 hash | no | 源 archive SHA256 为 `365e3336...368c9ac`；拉回 archive SHA256 为 `1211951...ef683`。manifest 内文件 hash 一致，但 archive 级 hash 因重新导出时 `created_at`、`source`、`archive_sha256` 变化而不同。 |
| 二进制文件 round-trip | yes | `assets/sample.bin` 在两份 manifest 中 SHA256 均为 `5e62a906ba05040729ff6e2b3e3f7d744671a3c64f375cc2159b29483ed29bc1`。 |
| 中断 archive 上传后 resume | yes | Session `b26552df-89cc-4eec-8fd9-51386ac05189` 共 2 个 part，先只上传 part 0，再执行 `neu sync resume` 上传 part 1 并 commit 成功。 |
| Resume history | yes | Job `eb6cc58d-4766-4f4c-a776-bf223bebcf3f` 状态为 `succeeded`。 |
| Resume sidecar 文件清理 | yes | `/private/tmp/vola-prodlike-20260513/resume-large.ndrvz.session.json` 在 resume 后已不存在。 |
| 服务端 session parts 清理 | yes | resumed session 的 `sync_session_parts` 计数为 `0`。 |
| Mirror preview 删除边界 | yes | `mirror-alpha.ndrvz` preview 只报告 2 个 delete，均在 `/skills/acceptance-alpha` 下：`assets/sample.bin`、`references/notes.md`。 |
| Mirror apply 删除边界 | yes | apply 后导出显示 `acceptance-alpha` 只剩 `SKILL.md`；`acceptance-beta` 仍保留 `SKILL.md` 和 `scripts/run.sh`。 |
| Profile category 不受 skills mirror 影响 | yes | skills-only mirror 后，`profile.preferences` 仍为 `acceptance profile should survive skills mirror`。 |
| 失败 job 记录错误摘要 | yes | 构造坏 bundle 后 import 返回 HTTP 400；history job `2937d6af-670c-42b5-b59f-94d3d410d18c` 记录错误 `skill "acceptance-bad" missing SKILL.md`。 |
| 失败 job 不回显原始 bundle 内容 | yes | history JSON 中未出现 `SECRET_RAW_BUNDLE_SHOULD_NOT_APPEAR`。 |
| WebDAV 上传代码路径 | yes，本地接收器 | 本地接收器收到 WebDAV `PUT`，带 Basic auth，对象为 `vola-export-20260513-031523Z.zip`，大小 34577 bytes。 |
| S3-compatible 上传代码路径 | yes，本地接收器 | 本地接收器收到 S3 path-style `PUT`，带 AWS Signature V4 auth，对象为 `acceptance/vola-export-20260513-031627Z.zip`，大小 34610 bytes。 |
| 真实 WebDAV/S3 provider 兼容性 | 未验证 | 未提供真实 WebDAV/S3 凭据。本地接收器结果不能证明真实 provider 兼容性。 |
| Backup run history | yes | `/api/backup/runs?limit=10` 返回两条 success run：WebDAV 和 S3。 |
| 恢复预览 | yes | 对 WebDAV ZIP 调用 `/api/backup/restore/preview` 返回 `recognized: true`，35 个文件，分类包括 `identity`、`memory_profile`、`skills`、`vault`、`unknown`。 |
| Restore apply | 未执行 | 本次只验证恢复预览，未执行恢复应用。 |
| `/api/ops/status` 无 token | yes | 返回 HTTP 401。 |
| `/api/ops/status` 带管理员 token | yes | 返回 `storage: sqlite`、`local_mode: true`、Git mirror 状态、2 个 backup targets、2 个 enabled targets、2 条最近成功 backup run，且无 last backup error。 |
| GitHub Backup 真实 push | 未验证 | 未提供 GitHub token、GitHub App session 或专用 GitHub repo。ops status 正确显示 GitHub backup repository 未配置。 |
| 浏览器路由检查 | yes | Headless Chrome 打开 `/sync-backup`、`/skills`、`/data`，均返回 200，无 route error 文本，无 console error。 |
| Go tests | yes | `go test ./...` 通过。 |
| Web build | yes | `cd web && npm run build` 通过。Vite 仍提示部分 chunk 超过 500 KB。 |
| Embedded build | yes | `make build` 通过，并刷新 `internal/web/dist`、`bin/vola`、`bin/neu`。 |

## Ops Status 快照

管理员 token 调用 `/api/ops/status` 的摘要：

```json
{
  "status": "warning",
  "storage": "sqlite",
  "local_mode": true,
  "git_mirror": {
    "service_configured": true,
    "execution_mode": "local",
    "sync_state": "idle",
    "github_app_connected": false,
    "github_token_set": false
  },
  "backup": {
    "service_configured": true,
    "targets_configured": 2,
    "enabled_targets": 2,
    "targets_with_last_backup": 2,
    "history_count": 2,
    "last_run_status": "success",
    "last_error": ""
  }
}
```

整体状态为 `warning`，原因是临时环境没有配置 GitHub Backup。

## 失败摘要

- 第一次服务启动失败：`VAULT_MASTER_KEY` 不是 64 位 hex，已改用验收专用 64 位 hex。
- 普通沙箱禁止本机端口绑定：服务和本地备份接收器都需要本机 bind 权限。
- CLI 在普通沙箱下无法访问本机服务：`neu sync history` 初次失败，错误为 `connect: operation not permitted`；获得权限后 sync 命令正常执行。
- Round-trip archive 文件 SHA256 不一致。内容级 diff 和 manifest 文件级 SHA256 均通过。
- 仓库 Playwright 依赖提示浏览器缓存缺失；浏览器路由检查改用本机已安装的 Google Chrome。

## 残余风险

- 没有 GitHub 凭据和专用验收 repo，真实 GitHub Backup 未验证。
- 没有真实 WebDAV/S3-compatible provider 凭据，真实 provider 上传和删除兼容性未验证。本地接收器只证明 Vola 会生成请求并记录 history。
- 本次只使用 SQLite，没有执行 Postgres 与生产 migrations 验收。
- 只执行恢复预览，没有执行 restore apply。
- committed sync session 因成功后清理了 parts，再查询 session 会显示 `missing_parts`。服务端清理是正确的，但这个响应形态容易误导运维人员。
- Vite 仍有 chunk size warning，主要涉及 `WorkbenchCodeEditor` 和主入口 chunk。
