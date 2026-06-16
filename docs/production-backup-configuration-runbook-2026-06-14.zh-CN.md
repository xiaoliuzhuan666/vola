# Vola 生产备份配置与验证记录

日期：2026-06-14

这份记录只写本次会话里已经做过、已经复查过的内容，不写 secret 原文，不把没做过的恢复动作写成完成。

## 范围

- 仓库：`/Users/zhongmoshu/Desktop/work/Vola`
- 生产地址：`https://driver.sunningfun.cn`
- 生产主机：`family-growth-tencent`
- 当前容器：
  - `neudrive-server`
  - `neudrive-postgres`
- 当前应用镜像：`vola:staging-20260614-ops-instance-connfix`

## 本地私密资料保存位置

以下文件已经保存在本机，不应提交到仓库：

- `/Users/zhongmoshu/Desktop/vola-cos-backup-credentials.md`
- `/Users/zhongmoshu/Desktop/vola-github-backup-credentials.md`

2026-06-14 复查结果：两个文件权限都是 `600`。

## 这次实际完成了什么

### 1. GitHub Backup

已完成的生产配置：

- GitHub App 名称：`Vola Backup`
- GitHub App slug：`vola-backup`
- Callback URL：`https://driver.sunningfun.cn/api/git-mirror/github-app/callback`
- 当前安装账号：`xiaoliuzhuan666`
- 当前备份仓库：`https://github.com/xiaoliuzhuan666/vola-backup.git`
- 当前模式：`github_app_user`
- 远端分支：`main`
- 自动 commit：开启
- 自动 push：开启

生产环境里已经设置：

- `PUBLIC_BASE_URL=https://driver.sunningfun.cn`
- `GIT_MIRROR_HOSTED_ROOT=/data/git-mirrors`
- `GITHUB_APP_CLIENT_ID` 已设置
- `GITHUB_APP_CLIENT_SECRET` 已设置
- `GITHUB_APP_SLUG=vola-backup`
- `INSTANCE_ADMIN_USER_IDS` 需要包含负责运维验收的用户 UUID，用于访问 `/api/ops/instance-status`

### 2. 外部备份目标

已完成的生产配置：

- 目标名称：`Tencent COS Backup`
- 目标类型：S3-compatible
- 用途：Vola 导出 zip 的离机备份
- 自动备份：开启
- 间隔：24 小时
- 保留策略：最近 7 份

当前最近一次成功对象：

- `vola/neudrive-export-20260614-090115Z.zip`

### 3. 服务与状态

2026-06-14 复查结果：

- `GET /api/health` 返回 `service=vola`、`status=ok`、`storage=postgres`
- 对已配置备份的生产用户调用 `/api/ops/status` 返回 `status=ok`
- 实例级 `/api/ops/instance-status` 返回 `status=ok`
- GitHub Backup、外部备份目标、remote backup artifact 三项检查都为 `ok`

### 4. 会话里额外确认过的动作

这些动作不是刚刚重新跑的，但在同一会话里已经实际做过：

- GitHub App 浏览器授权成功
- 创建或复用私有仓库 `xiaoliuzhuan666/vola-backup`
- 成功触发 Git mirror sync 与 push
- COS 真实上传成功
- 私有对象签名下载验证过
- 恢复预览验证过
- 保留策略删除验证过

## 当前复查快照

### 健康检查

```json
{"ok":true,"data":{"service":"vola","status":"ok","storage":"postgres","time":"2026-06-14T11:06:20Z"}}
```

### `/api/ops/status` 摘要

复查时间：`2026-06-14T09:57:40Z`

```json
{
  "status": "ok",
  "storage": "postgres",
  "local_mode": false,
  "public_url": "https://driver.sunningfun.cn",
  "git_mirror": {
    "execution_mode": "hosted",
    "hosted_root_set": true,
    "remote_url": "https://github.com/xiaoliuzhuan666/vola-backup.git",
    "auto_push_enabled": true,
    "sync_state": "idle",
    "last_synced_at": "2026-06-14T09:45:25Z",
    "last_push_at": "2026-06-14T09:45:32Z",
    "github_app_connected": true
  },
  "backup": {
    "targets_configured": 1,
    "enabled_targets": 1,
    "targets_with_last_backup": 1,
    "last_successful_backup_at": "2026-06-14T09:01:15Z",
    "last_backup_object": "vola/neudrive-export-20260614-090115Z.zip",
    "last_run_status": "success"
  }
}
```

### `/api/ops/instance-status` 摘要

复查时间：`2026-06-14T11:06:45Z`

```json
{
  "status": "ok",
  "users_total": 2,
  "users_with_git_backup": 1,
  "users_with_external_backup": 1,
  "users_with_remote_backup_artifact": 1,
  "users_with_critical_backup_status": 0,
  "checks": [
    {"id": "server", "status": "ok"},
    {"id": "instance_users", "status": "ok"},
    {"id": "instance_backup_errors", "status": "ok"},
    {"id": "instance_git_mirror_remote", "status": "ok"},
    {"id": "instance_external_backup_targets", "status": "ok"},
    {"id": "instance_remote_backup_artifact", "status": "ok"}
  ]
}
```

### 生产部署记录

- 本地镜像：`vola:staging-20260614-ops-instance-connfix`
- 镜像 ID：`sha256:1bc920eec3d9cab1e39f3460dd1a7df73a94dbe5612a8917cabc7031cb2d69c1`
- 架构：`linux/amd64`
- 生产 env 备份：
  - `/opt/neudrive/config/neudrive.env.bak-image-20260614T105510Z`
  - `/opt/neudrive/config/neudrive.env.bak-image-connfix-20260614T110608Z`
- 部署命令：`docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f /opt/neudrive/deploy/tencent/docker-compose.yml up -d --no-deps server`
- 部署后 `GET /api/health`：`status=ok`、`storage=postgres`
- 部署后 `/api/ops/instance-status`：`status=ok`
- 部署后 `/api/connections`：`HTTP 200`
- 浏览器验证 `/sync-backup`：
  - 标题：`GitHub Backup — Vola`
  - 页面显示：`实例备份状态正常`
  - 页面显示：`实例用户：1/2 个已有远端备份记录`
  - 页面显示：`外部备份用户：1/2`
  - console error：0

回滚时可把 `NEUDRIVE_IMAGE` 改回上一个镜像 `vola:staging-20260614-ops-instance`，或使用上面的 env 备份文件恢复，再执行同一条 Compose 命令。

### 容器状态

复查时生产相关容器：

- `neudrive-server`：`Up`，镜像 `vola:staging-20260614-ops-instance-connfix`
- `neudrive-postgres`：`Up (healthy)`

### 临时 admin token 清理

本次复查里多次插入过短效 `admin` token，用来读取 `/api/ops/status`、`/api/ops/instance-status`、`/api/connections` 和浏览器页面。

复查结果：

- 这些 token 都已经撤销
- 没有保留到本地文件

## 事实表

| Requirement or behavior | Status | Sources | Verification | Unknowns or conflicts |
| --- | --- | --- | --- | --- |
| 生产服务可访问 | OBSERVED | `curl https://driver.sunningfun.cn/api/health` | 返回 `status=ok`、`storage=postgres` | 无 |
| Hosted Git mirror 已配置 | OBSERVED | `PUBLIC_BASE_URL`、`GIT_MIRROR_HOSTED_ROOT`、`/api/ops/status` | `hosted_root_set=true` | 无 |
| GitHub App 用户授权可用 | OBSERVED | 同会话操作记录，`/api/ops/status` | `github_app_connected=true` | GitHub App 权限如果后续变更，用户需要重新批准 |
| GitHub Backup 仓库已连通并有 push 记录 | OBSERVED | `/api/ops/status`、`local_git_mirrors` 查询 | `last_push_at=2026-06-14T09:45:32Z` | 当前备份归属于一个具体用户，不是实例级全局状态 |
| 外部备份目标已配置并有真实上传 | OBSERVED | `/api/ops/status`、同会话操作记录 | `targets_with_last_backup=1`，最近对象名已记录 | 当前只配置了 1 个目标 |
| 保留策略已生效 | OBSERVED | `/api/ops/status` recent runs | 较早对象出现 `remote_deleted_at` | 只验证了当前策略，不代表以后改策略仍正确 |
| 实例管理员配置 | REQUIRED | `INSTANCE_ADMIN_USER_IDS` | 控制 `/api/admin/users` 和 `/api/ops/instance-status` | 真实用户 UUID 不写入仓库文档 |
| 实例级运维状态接口 | OBSERVED | 生产：`/api/ops/instance-status`、`GitMirrorPage` | 本地测试通过；生产接口返回 `status=ok`；页面显示实例状态卡片 | `/api/ops/status` 仍是当前用户状态，不能当作实例级结论 |
| 恢复预览可用 | OBSERVED | 同会话操作记录 | 已对真实备份包做 preview | 这次复查没有重跑 |
| 恢复 apply 在临时环境完成 | OBSERVED | 本地临时 Postgres + 本地临时 Vola 服务 | 早期演练：preview 识别 47 个文件；apply 写入 20 个、跳过 27 个 | 早期演练使用的是恢复后临时服务导出的 Vola zip |
| COS 最近对象直接恢复完成 | OBSERVED | `tools/restore-drill.sh --object ...` | 直接下载 COS 对象；preview 识别 47 个文件；apply 写入 36 个、跳过 11 个、错误 0 个；`file_tree` live rows 为 67 | 这是文件树 restore preview/apply 验证，不等同于数据库级全量恢复 |
| Postgres dump 与临时库恢复完成 | OBSERVED | `pg_dump`、`pg_restore`、临时 Vola `/api/health` | 早期演练 dump 230565 bytes；2026-06-15 复验 dump 245047 bytes；临时服务返回 `storage=postgres` | 临时环境使用随机 `JWT_SECRET` |
| 使用原 `VAULT_MASTER_KEY` 验证旧 secret 解密 | OBSERVED | 生产只读 dump、原生产 `VAULT_MASTER_KEY`、`tools/restore-drill.sh --vault-master-key-file ...` | 真实候选 Vault 记录 2 条；DB 层探针返回 `plaintext_bytes=80` | 只记录脱敏 scope 和明文字节数，不记录 secret 原文 |
| Vault 异常 nonce 不再导致服务 panic | OBSERVED | `internal/vault/vault.go`、`internal/vault/vault_test.go` | `Decrypt` 对 nil / 非法 nonce 长度返回 `ErrDecryptFailed`；新增短 nonce 测试 | 还需要完整回归测试确认所有调用侧按错误处理 |
| 自动备份调度已经跨 24 小时观察过 | NOT RUN | `/api/ops/status` | 目前只确认了手动成功和目标配置 | 自动任务是否按 24 小时稳定触发，还要等时间验证 |
| 当前实例的二进制对象存储已经切到 COS | NOT CONFIGURED | 容器环境变量复查 | `OBJECT_STORAGE_BACKEND=db`，`TENCENT_COS_BUCKET` 等未设置 | 现在 COS 用在外部备份，不是运行时对象存储 |

## 实操流程

### A. GitHub Backup 配置流程

1. 在 GitHub 创建 GitHub App。
2. 配置 Callback URL：`https://driver.sunningfun.cn/api/git-mirror/github-app/callback`
3. 给 App 打开仓库权限：
   - `Administration: write`
   - `Contents: write`
4. 在生产环境写入：
   - `GITHUB_APP_CLIENT_ID`
   - `GITHUB_APP_CLIENT_SECRET`
   - `GITHUB_APP_SLUG`
   - `INSTANCE_ADMIN_USER_IDS`
5. 重启 `neudrive-server`。
6. 用浏览器在 `GitHub Backup` 页面点击：
   - `连接 GitHub`
   - `创建私有备份仓库`
   - `立即同步`
7. 用 `/api/ops/status` 确认：
   - `git_mirror_remote=ok`
   - `remote_backup_artifact=ok`

### B. COS 外部备份配置流程

1. 在对象存储侧准备 bucket 和访问凭据。
2. 在 Vola 的 `GitHub Backup` 页面新增外部备份目标。
3. 类型选 `S3-compatible`。
4. 填入 bucket、region、endpoint、prefix 和密钥。
5. 打开：
   - `enabled`
   - `auto_backup_enabled`
6. 设置：
   - `auto_backup_interval_hours=24`
   - `retention_keep_last=7`
7. 手动执行一次上传。
8. 用 `/api/ops/status` 和备份历史确认最近对象名、状态、错误字段。

## 这次发现的几个容易误解点

### 1. `/api/ops/status` 不是实例级全局状态

它是按当前 token 所属用户计算的。

如果你拿了另一个没有配置 GitHub Backup / 外部备份的用户 token 去看，会得到 `warning`，哪怕系统里另一个用户已经配置好了备份。

这会误导运维排查。

生产环境已新增 `/api/ops/instance-status`。它会遍历用户账号，汇总实例里多少用户已有 GitHub Backup、外部备份上传和远端备份对象，并在 `subjects` 里显示每个用户自己的状态。管理员应优先用这个接口看整台实例。

### 2. COS 现在是“外部备份目标”，不是“运行时对象存储”

当前生产容器环境里：

- `OBJECT_STORAGE_BACKEND=db`
- `TENCENT_COS_BUCKET` 未设置

所以当前状态是：

- Hub 的主存储：Postgres
- 运行时二进制对象：仍在数据库
- COS：只用于外部备份 zip

如果目标是“上传文件直接进 COS”，还需要单独切换对象存储配置。

### 3. 生产主机上有一组误建但未清理的 Docker 资源

当前仍能看到：

- volumes：`tencent_gitmirrors`、`tencent_pgdata`
- network：`tencent_default`

正式使用的是：

- volumes：`neudrive_gitmirrors`、`neudrive_pgdata`
- network：`neudrive_default`

这些误建资源这次没有删除，原因是避免在生产机器上多做一步无必要操作。

## 现在能不能说“都验证完了”

核心备份恢复链路已经验证到临时环境，但还不能说所有事项都验证完。

现在已经验证到的，是下面这条链路：

1. 生产服务在线
2. GitHub Backup 已连通
3. COS 外部备份已上传
4. `/api/ops/status` 对实际配置用户返回 `ok`
5. 生产 Postgres dump 可以恢复到临时 Postgres
6. 临时 Vola 服务可以基于恢复库启动
7. Vola 导出 zip 可以在临时用户上执行 restore preview/apply
8. 生产 `/api/ops/instance-status` 返回 `ok`
9. 生产 `/sync-backup` 页面显示实例状态卡片，且没有 console error

## 2026-06-14 公开注册关闭与实例管理员加固记录

这次又补了两层控制：

- `INSTANCE_ADMIN_USER_IDS` 只允许指定用户访问 `/api/admin/users` 和 `/api/ops/instance-status`
- `VOLA_ENABLE_PUBLIC_REGISTRATION=0` 关闭公开注册，`/api/auth/register` 和第三方 `signup` 入口都返回 `403`

本地验证：

- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/config` 通过
- `go test ./internal/api` 的公开注册和实例管理员相关用例通过
- `GOCACHE=/private/tmp/vola-go-cache go test ./...` 通过
- `npm --prefix web run build` 通过
- `npm --prefix web run test` 通过
- `docker compose -f docker-compose.yml config --services` 通过
- 腾讯 Compose 用 dummy env 执行配置解析通过
- `bash -n deploy/prod/deploy.sh deploy/tencent/pull-and-deploy.sh` 通过
- `git diff --check` 通过

生产验证结果：

- `GET /api/config` 返回 `public_registration_enabled=false`
- `POST /api/auth/register` 返回 `403`
- `GET /login` 页面显示“公开注册已关闭，请联系管理员创建账号”
- `GET /signup` 页面显示“注册已关闭 / 联系管理员”
- 公开页的注册按钮都改为按 `public_registration_enabled` 跳转到登录或注册
- `GET /api/health` 返回 `status=ok`、`storage=postgres`
- `neudrive-server` 使用新镜像运行，并继续绑定在 `127.0.0.1:18080`
- 同机其它服务端口 `127.0.0.1:18090`、`127.0.0.1:3005`、`127.0.0.1:3006` 仍在 loopback 上监听

这次发布用的新镜像：

- `vola:staging-20260614-registration-lock-amd64`
- 镜像 ID：`sha256:d12e9a93c329ca211758da388e36bc81d0ae9c2fc1ce75126862e9eb8b824db8`
- 架构：`linux/amd64`

远端备份文件：

- `/opt/neudrive/config/neudrive.env.bak-registration-lock-20260614T125408Z`
- `/opt/neudrive/deploy/tencent/docker-compose.yml.bak-registration-lock-20260614T125408Z`

回滚方式：

1. 把远端 env 或 Compose 还原到上面的备份文件，或把镜像变量改回上一版。
2. 执行 `docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f /opt/neudrive/deploy/tencent/docker-compose.yml up -d --no-deps server`。
3. 再查 `/api/health`、`/api/config` 和相关页面。

这次验证没覆盖的只有一项：

- 还没做“把公开注册重新打开”的回归验证，这次是只验证关闭路径。

还没验证完的是：

1. 自动备份跨 24 小时观察

## 2026-06-14 恢复演练记录

演练方式：

1. 从生产 `neudrive-postgres` 执行 `pg_dump --format=custom --no-owner --no-acl`。
2. 本机启动独立临时 Postgres 容器。
3. 用 `pg_restore` 恢复到 `vola_restore`。
4. 本机启动独立临时 Vola 服务，连接该临时库。
5. 给恢复出的源用户插入短效 `admin` token，调用 `/api/export/zip` 导出 Vola zip。
6. 创建演练用户，插入短效 `admin` token。
7. 对演练用户执行 `/api/backup/restore/preview` 和 `/api/backup/restore/apply`。
8. 撤销临时 token，删除临时容器和临时文件。

结果：

- dump 大小：230565 bytes
- 临时库恢复后数量：
  - `users=2`
  - `file_tree=496`
  - `backup_targets=1`
  - `local_git_mirrors=1`
- 临时服务健康检查：
  - `service=vola`
  - `status=ok`
  - `storage=postgres`
- 导出的 Vola zip：40183 bytes
- restore preview：
  - `recognized=true`
  - `total_files=47`
  - 分类包括 `identity`、`inbox`、`memory_profile`、`projects`、`roles`、`scratch`、`skills`、`vault`
  - 警告：包含 Vault 范围；有 1 个 unknown 文件
- restore apply：
  - `recognized=true`
  - `mode=skip`
  - `applied=20`
  - `skipped=27`
  - `overwritten=0`
  - 演练用户恢复后 `file_tree` 有 35 行
- 临时 source token 和 restore token 都已撤销
- 临时 Postgres 容器和 `/private/tmp/vola-restore-drill-*` 已清理

第一次临时服务启动失败，原因是使用了 Docker 容器内网 IP。本机进程改用 Docker 映射端口后演练通过。

## 2026-06-15 COS 对象直接恢复演练

这次补上了“直接从 COS 最近对象恢复”的验证，不再使用临时服务重新导出的 zip。

对象：

- `vola/neudrive-export-20260614-090115Z.zip`
- 下载大小：40376 bytes

工具：

- 新增 `tools/download-s3-backup.py`：从本地 COS 凭据 md 读取字段，使用 S3 SigV4 发起 `GET Object`，不打印 secret。
- 新增 `tools/restore-drill.sh`：启动临时 Postgres 和临时 Vola，创建临时 owner token，执行 restore preview/apply，最后清理临时资源。

命令：

```bash
tools/restore-drill.sh \
  --object vola/neudrive-export-20260614-090115Z.zip \
  --postgres-image pgvector/pgvector:pg16
```

结果：

- COS 对象下载成功。
- 临时 Postgres 容器启动成功。
- 临时 Vola 服务启动在 `127.0.0.1` 本地端口。
- restore preview：
  - `recognized=true`
  - `total_files=47`
  - `total_bytes=88408`
  - 分类包括 `identity`、`inbox`、`memory_profile`、`projects`、`roles`、`scratch`、`skills`、`unknown`、`vault`
- restore apply：
  - `recognized=true`
  - `mode=skip`
  - `applied=36`
  - `skipped=11`
  - `overwritten=0`
  - `errors=0`
- 临时库恢复后 `file_tree` live rows 为 67。
- 脚本结束后已撤销临时 token，并清理临时 Vola 进程和临时 Postgres 容器。

第一次下载失败过一次，COS 返回 `SignatureDoesNotMatch`。原因是凭据 md 的 JSON 示例里 `s3_secret_access_key` 是占位说明，脚本错误地优先读取了占位值。已修正为跳过 `<...>` 形式的占位值后读取真实 `SecretKey`。

同日复跑结果一致：

- `tools/restore-drill.sh --object vola/neudrive-export-20260614-090115Z.zip --postgres-image pgvector/pgvector:pg16` 通过。
- 下载大小仍为 40376 bytes。
- restore preview 仍识别 47 个文件，分类包括 `identity`、`inbox`、`memory_profile`、`projects`、`roles`、`scratch`、`skills`、`unknown`、`vault`。
- restore apply 结果仍为 `mode=skip`、`applied=36`、`skipped=11`、`errors=0`。
- `file_tree` live rows 仍为 67。

2026-06-15 同时做了生产只读查询：

- `GET /api/health` 返回 `status=ok`、`storage=postgres`
- `backup_targets` 显示 `auto_backup_enabled=true`、`auto_backup_interval_hours=24`
- 最新 `backup_runs` 仍是 2026-06-14 09:01 UTC 的手动成功备份
- `last_auto_backup_at` 为空

所以 24 小时自动备份观察目前是“未到期”，不能写成已通过。

## 2026-06-15 Vault 旧 secret 解密复验状态

目标是确认：恢复后的生产数据库 dump 在使用原生产 `VAULT_MASTER_KEY` 时，旧 Vault secret 可以解密。

2026-06-15 14:12 CST 已完成复验，结论为通过。

此前卡住的原因：

- 之前的临时 dump `/private/tmp/vola-prod-20260615.dump` 已不存在。
- 本地 COS 凭据文档不包含 `VAULT_MASTER_KEY`，也没有可用于解密生产 Vault 数据的 key 文件。
- 2026-06-15 14:00 CST 重新验证 SSH 到 `family-growth-tencent` 已恢复，`ssh -o BatchMode=yes -o ConnectTimeout=15 family-growth-tencent 'echo ssh-ok'` 返回 `ssh-ok`。
- SSH 恢复只解决了主机访问问题；复验还需要重新生成生产 dump，并临时使用原生产 `VAULT_MASTER_KEY`。

复验期间发现了一个服务端问题：

- 早前 dump 演练里，调用 `/api/vault/auth.github` 曾返回 HTTP 500。
- 服务日志里出现 `crypto/cipher: incorrect nonce length given to GCM`。
- 当时恢复出的临时库里，部分 Vault 记录 `nonce` 长度为 1；真实加密形态的候选记录 `nonce` 长度为 12。

对应修正：

- `internal/vault/vault.go` 的 `Decrypt` 现在会检查 nil Vault、nil GCM 和 nonce 长度，异常时返回 `ErrDecryptFailed`，不再让 AES-GCM panic。
- `internal/vault/vault_test.go` 新增短 nonce 测试。
- 新增 `tools/probe-vault-decrypt.go`，只从 `vault_entries` 里选 `octet_length(nonce)=12` 且有密文的记录做解密探针。
- `tools/restore-drill.sh` 已支持 `--vault-master-key-file`，在有 dump 和原 key 时自动执行 DB 层 Vault 解密探针。

后续要完成这项，需要：

```bash
tools/restore-drill.sh \
  --dump /path/to/vola-prod.dump \
  --object vola/neudrive-export-20260614-090115Z.zip \
  --postgres-image pgvector/pgvector:pg16 \
  --vault-master-key-file /path/to/vault-master-key.txt
```

脚本输出应只记录脱敏后的 scope 和明文字节数，不记录 secret 原文。

本次复验执行：

- 从生产 `neudrive-postgres` 使用 `pg_dump -Fc --no-owner --no-acl` 生成本地临时 dump，大小 245047 bytes。
- 从生产 env 读取 `VAULT_MASTER_KEY` 到本地临时文件；未打印 key 值，复验后已删除。
- 生产库 `vault_entries` 总数 21；其中 `octet_length(nonce)=12` 且 `encrypted_data` 非空的真实候选记录 2 条；异常 nonce 记录 19 条。
- 执行：

```bash
tools/restore-drill.sh \
  --dump /private/tmp/vola-vault-verify-umAra0/vola-prod-20260615T061240Z.dump \
  --object vola/neudrive-export-20260614-090115Z.zip \
  --postgres-image pgvector/pgvector:pg16 \
  --vault-master-key-file /private/tmp/vola-vault-verify-umAra0/vault-master-key-20260615T061240Z.txt
```

结果：

- COS 对象下载：40376 bytes。
- restore preview：`recognized=true`、`total_files=47`、`total_bytes=88408`。
- restore apply：`mode=skip`、`applied=31`、`skipped=16`、`overwritten=0`、`errors=0`。
- 临时库 `file_tree` live rows：554。
- `/api/vault/scopes` 元数据探针返回 `vault scopes count=5`。
- DB 层解密探针返回：`vault decrypt ok scope=auth.github.git_mirror.app_user_refresh_token plaintext_bytes=80`。
- 未输出 secret 明文、token、生产 UUID 或 `VAULT_MASTER_KEY`。

清理复查：

- 本地临时 dump 和 key 文件已删除。
- `docker ps -a --filter name=vola-restore-drill` 没有残留临时容器。
- `/private/tmp` 下没有 `vola-restore-drill-*` 或 `vola-vault-verify-*` 临时目录。
- 复验后生产 `/api/health` 仍返回 `status=ok`、`storage=postgres`。

## 2026-06-15 本地验证补充

这轮对当前脚本和 Vault 修正做了本地验证：

- `gofmt -w internal/vault/vault.go internal/vault/vault_test.go tools/probe-vault-decrypt.go` 已执行。
- `bash -n tools/restore-drill.sh` 通过。
- `python3 -m py_compile tools/download-s3-backup.py` 通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/vault` 通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteBackupTargetsPersistAutomationSchedule|TestSQLiteBackupRestorePreviewReadsNeuDriveZip|TestSQLiteBackupRestoreApplySupportsSkipOverwriteAndRejectsUnsafePaths|TestSQLiteBackupRunHistoryRecordsManualSuccessAndFailure|TestSQLiteBackupRunHistoryRecordsAutomaticSuccessAndFailure|TestSQLiteSharedServerOpsStatusReportsBackupReadiness|TestSQLiteSharedServerOpsInstanceStatusAggregatesBackupReadiness|TestInstanceAdminAPIsRejectNonOwnerAdminToken|TestHostedInstanceAdminAPIsAllowConfiguredAdminUser|TestTeamsSupportMultipleMembershipsAndIsolatedSkills'` 通过。
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/backups` 通过。
- `tools/restore-drill.sh --object vola/neudrive-export-20260614-090115Z.zip --postgres-image pgvector/pgvector:pg16` 通过。
- `git diff --check` 通过。
- `ssh -o BatchMode=yes -o ConnectTimeout=10 family-growth-tencent 'echo ssh-ok'` 早前失败，错误为 `Connection timed out during banner exchange`。
- 2026-06-15 14:00 CST 复查 `ssh -o BatchMode=yes -o ConnectTimeout=15 family-growth-tencent 'echo ssh-ok'` 通过。

本轮 COS 对象恢复脚本输出：

- 下载对象：`vola/neudrive-export-20260614-090115Z.zip`
- 下载大小：40376 bytes
- restore preview：`recognized=true`、`total_files=47`、`total_bytes=88408`
- restore apply：`mode=skip`、`applied=36`、`skipped=11`、`overwritten=0`、`errors=0`
- 临时库 `file_tree` live rows：67

清理复查：

- `docker ps -a --filter name=vola-restore-drill` 没有残留临时容器。
- `/private/tmp` 下没有 `vola-restore-drill-*` 临时目录。

仍未验证：

- 24 小时自动备份触发。原因是自动备份观察点还没有新的到期触发记录；已创建本地 Codex 后续检查 `vola-24h-backup-observation`，到点后只做只读检查并继续记录结果。

## 2026-06-15 公开注册重新开启和自注册验证

目标：默认每用户 100MB 云端资料额度不变，并允许同事自己创建账号使用。

生产改动：

- 备份 env：`/opt/neudrive/config/neudrive.env.bak-public-registration-20260615T055803Z`
- 备份 Compose：`/opt/neudrive/deploy/tencent/docker-compose.yml.bak-public-registration-20260615T055803Z`
- 设置 `VOLA_ENABLE_PUBLIC_REGISTRATION=1`
- 保持 `USER_STORAGE_QUOTA_BYTES=100MB`
- 只重建 `neudrive-server`，没有重启 Postgres：

```bash
ssh family-growth-tencent '
  cd /opt/neudrive/deploy/tencent &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml up -d --no-deps server
'
```

生产复查结果：

- SSH：`ssh -o BatchMode=yes -o ConnectTimeout=15 family-growth-tencent 'echo ssh-ok'` 通过。
- `/api/health`：`status=ok`，`storage=postgres`。
- `/api/config`：`public_registration_enabled=true`，`github_app_enabled=true`，`storage=postgres`。
- 远程 env 脱敏检查：
  - `PUBLIC_BASE_URL=https://driver.sunningfun.cn`
  - `USER_STORAGE_QUOTA_BYTES=100MB`
  - `VOLA_ENABLE_PUBLIC_REGISTRATION=1`
  - `OBJECT_STORAGE_BACKEND=db`
  - `INSTANCE_ADMIN_USER_IDS` 已配置，未记录 UUID。
- `docker compose ps`：
  - `neudrive-server` 使用 `vola:staging-20260614-registration-lock-amd64`，端口 `127.0.0.1:18080->8080/tcp`。
  - `neudrive-postgres` 保持运行并为 healthy。
- 共用端口仍绑定在 loopback：
  - `127.0.0.1:18080`
  - `127.0.0.1:18090`
  - `127.0.0.1:3005`
  - `127.0.0.1:3006`

自注册验证：

- 使用临时邮箱和 slug 调用 `POST /api/auth/register`，返回 HTTP 201。
- 注册响应包含 access token 和 refresh token；文档和终端输出均未记录 token。
- 使用返回的 access token 调用 `GET /api/auth/me`，返回 HTTP 200，slug 和 email 与临时账号一致。
- 使用同一 token 调用 `GET /api/tree/`，返回 HTTP 200，说明新账号创建后可进入受保护的个人资料接口。
- 数据库里该用户 `storage_quota_bytes IS NULL`，表示继承全局默认额度 `USER_STORAGE_QUOTA_BYTES=100MB`。
- 临时账号已从 `users` 删除；删除前关联记录为 credentials=1、sessions=1、roles=1；删除后按 email 或 slug 查询用户数为 0。

页面验证：

- 浏览器打开 `https://driver.sunningfun.cn/signup`。
- 页面标题为 `注册 — Vola`。
- 表单显示邮箱、密码、账户名、显示名称。
- 按钮显示“创建账号”。
- 未出现“注册已关闭”或“联系管理员创建账号”的提示。

回滚方式：

```bash
ssh family-growth-tencent '
  cp /opt/neudrive/config/neudrive.env.bak-public-registration-20260615T055803Z /opt/neudrive/config/neudrive.env &&
  cd /opt/neudrive/deploy/tencent &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml up -d --no-deps server &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml ps
'
```

也可以只把 `/opt/neudrive/config/neudrive.env` 中的 `VOLA_ENABLE_PUBLIC_REGISTRATION` 改回 `0`，再执行同一条 `docker compose ... up -d --no-deps server`。

注意：

- 100MB 是每个账号的 Vola file tree / 用户资料默认额度，不是 COS bucket 总容量。
- 公开注册打开后，同事可以自己创建账号；但它同时带来垃圾注册和账号数量增长风险。当前容量限制约束单账号资料大小，不约束账号数量和注册频率。

## 给其他用户用，够不够简单

### 对普通使用者

在“运维已经预先配好”的前提下，GitHub Backup 这条路径算比较简单：

1. 连接 GitHub
2. 创建私有备份仓库
3. 立即同步

这个流程对普通用户是能用的。

### 对部署者

还不算简单。

部署者至少要理解这些东西：

- `PUBLIC_BASE_URL`
- `JWT_SECRET`
- `VAULT_MASTER_KEY`
- `GIT_MIRROR_HOSTED_ROOT`
- `INSTANCE_ADMIN_USER_IDS`
- `VOLA_ENABLE_PUBLIC_REGISTRATION`
- GitHub App 三个环境变量
- 外部备份目标参数
- `/api/ops/status` 是当前用户状态，`/api/ops/instance-status` 才是实例状态

### 用户数据隔离结论

当前系统已经做了应用层用户数据隔离，主要依据是：

- 常规文件树读取和列表按 `user_id` 查询，且继续受 `min_trust_level` 限制，代码位置包括 `internal/services/filetree_service.go`。
- Vault scope 列表、读取、写入和删除都按 `user_id` 与 scope 查询，secret 原文不会进入导出包，代码位置包括 `internal/services/vault_service.go`。
- Team Library 通过 `team_members` 判断成员关系；成员可以读授权团队资料，viewer 写入会返回 `403`，代码位置包括 `internal/services/team_service.go`。
- 测试覆盖了非实例管理员访问 `/api/admin/users`、`/api/ops/instance-status` 返回 `403`，也覆盖了 Bob 不能读取 owner 的个人 Skill，以及 viewer 不能写团队文件。

边界也要写清楚：

- 这是应用层隔离，不是 Postgres Row Level Security。当前没有看到 `CREATE POLICY` 或 `ENABLE ROW LEVEL SECURITY`。
- 使用同一个数据库账号连接 Postgres 时，数据库本身不会替应用兜住跨用户 SQL 错误。后续如果要做更强的多租户安全，可以评估数据库 RLS、只读运维账号、更多跨用户负向用例和审计告警。
- 实例管理员本来就有整站运维能力，所以 `INSTANCE_ADMIN_USER_IDS` 必须只放可信用户 UUID。

### 当前最值得补的地方

1. 给腾讯 COS 做一套更直接的预设文案，不要只显示成泛化的 S3-compatible。
2. 如果后面打算承接文件类内容，考虑把对象存储从 `db` 切到 COS，而不是只做外部备份。
3. 把 `/api/ops/instance-status` 做成独立运维页面，减少进入 GitHub Backup 页面才能看到实例状态的依赖。
4. 增加自动备份到期后的定时观测记录，至少覆盖一次 24 小时自动触发。
5. 增加跨用户数据隔离专项测试清单，覆盖文件树、Vault、连接配置、备份目标、Team Library 和实例管理员接口。

## 本次复查用到的命令

```bash
curl -fsS https://driver.sunningfun.cn/api/health
curl -fsS https://driver.sunningfun.cn/api/ops/instance-status
curl -fsS https://driver.sunningfun.cn/api/connections
ssh family-growth-tencent 'docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"'
ssh family-growth-tencent 'docker exec neudrive-postgres psql -U neudrive -d neudrive -Atc "SELECT user_id, count(*), max(last_backup_at) FROM backup_targets GROUP BY user_id;"'
ssh family-growth-tencent 'docker exec neudrive-postgres psql -U neudrive -d neudrive -Atc "SELECT user_id, remote_url, last_synced_at, last_push_at FROM local_git_mirrors;"'
ssh family-growth-tencent 'docker volume ls'
ssh family-growth-tencent 'docker network ls'
```

`/api/ops/status`、`/api/ops/instance-status`、`/api/connections` 和浏览器复查都使用了临时短效 `admin` token，读完即撤销。

## 2026-06-15 首次体验简化改动生产发布

目标：发布首页和 MCP 接入页的首次体验简化改动，让新用户优先连接第一个 AI 工具，并把示例数据与网页沙盒放到较低优先级区域。

本地发布前检查：

- `npm --prefix web run build`：通过，包含 `tsc --noEmit` 和 Vite production build。
- `rm -rf internal/web/dist && cp -R web/dist internal/web/dist`：已同步前端产物到 Go embed 目录。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`：通过。
- `docker compose config --services`：通过，服务为 `db`、`server`。
- `env VOLA_IMAGE=vola:test POSTGRES_PASSWORD=dummy JWT_SECRET=dummy VAULT_MASTER_KEY=... docker compose -f deploy/tencent/docker-compose.yml config --services`：通过，服务为 `db`、`server`。
- `bash -n deploy/prod/deploy.sh deploy/tencent/pull-and-deploy.sh`：通过。
- `git diff --check`：通过。
- `docs/agent-data-hub-competitor-research.zh-CN.md` 尾部空格检查：通过。

发布前线上状态：

- `https://driver.sunningfun.cn/api/health` 返回 HTTP 502。
- `docker ps` 没有 `neudrive-server` 和 `neudrive-postgres`。
- `/opt/neudrive` 存在，`/opt/vola` 不存在。
- 同机已有 `family-growth-api` 和 `family-growth-api-harmony`，端口分别绑定在 `127.0.0.1:3005` 和 `127.0.0.1:3006`。

镜像：

- 新镜像：`vola:release-202606152057-ux-843c3f60b242-dirty`
- 新镜像 ID：`sha256:0661efe82fb53da2b9254aae2d176bdbc501eaf04dba369c89e18c0da4cfcf60`
- 架构：`linux/amd64`
- 构建命令：

```bash
docker build --platform linux/amd64 -t vola:release-202606152057-ux-843c3f60b242-dirty .
```

- 上传命令：

```bash
docker save vola:release-202606152057-ux-843c3f60b242-dirty | ssh family-growth-tencent 'docker load'
```

远端改动：

- 备份目录：`/opt/neudrive/backups/deploy-20260615T130436Z`
- 备份 env：`/opt/neudrive/backups/deploy-20260615T130436Z/neudrive.env`
- 备份 Compose：`/opt/neudrive/backups/deploy-20260615T130436Z/docker-compose.yml`
- 旧镜像记录：`/opt/neudrive/backups/deploy-20260615T130436Z/previous-image.txt`
- 旧镜像：`vola:staging-20260614-registration-lock-amd64`
- 旧镜像 ID：`sha256:d12e9a93c329ca211758da388e36bc81d0ae9c2fc1ce75126862e9eb8b824db8`
- 修改项：仅更新 `/opt/neudrive/config/neudrive.env` 里的 `NEUDRIVE_IMAGE`。

执行命令：

```bash
ssh family-growth-tencent '
  cd /opt/neudrive/deploy/tencent &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml up -d db server &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml ps
'
```

生产复查结果：

- 服务器本机 `http://127.0.0.1:18080/api/health`：HTTP 200，`status=ok`，`storage=postgres`。
- 公网 `https://driver.sunningfun.cn/api/health`：HTTP 200，`status=ok`，`storage=postgres`。
- 公网 `https://driver.sunningfun.cn/api/config`：HTTP 200，`public_registration_enabled=true`，`github_app_enabled=true`，`storage=postgres`。
- `http://driver.sunningfun.cn/`：HTTP 301 跳转到 HTTPS。
- `https://driver.sunningfun.cn/`：HTTP 200。
- 浏览器打开首页：标题为 `Personal data hub for AI agents — Vola`。
- 浏览器打开 `/setup/mcp`：未登录状态跳转到 `/login?redirect=%2Fsetup%2Fmcp`，页面标题为 `登录 — Vola`。
- 浏览器控制台：0 个 error，0 个 warning。
- `docker compose ps`：
  - `neudrive-server` 使用 `vola:release-202606152057-ux-843c3f60b242-dirty`，端口 `127.0.0.1:18080->8080/tcp`。
  - `neudrive-postgres` 使用 `postgres:16-alpine`，状态为 healthy。
- 端口检查：
  - `127.0.0.1:18080` 已绑定给 Vola。
  - `127.0.0.1:3005` 和 `127.0.0.1:3006` 仍绑定给 family-growth 服务。
  - 本次检查未观察到 `127.0.0.1:18090` 监听。
- `docker ps`：`family-growth-api` 与 `family-growth-api-harmony` 仍为 healthy。
- `docker compose logs --tail=120 server`：未看到启动错误；`RunExternalBackups` 本轮执行结果为 `succeeded=1`。

未验证：

- 没有使用生产账号登录验证 `/setup/mcp` 内页；线上未登录访问会跳转登录页。
- 没有重新做自注册临时账号创建和清理；本轮只读取 `/api/config` 确认公开注册开关仍为开启状态。
- 没有调用实例管理员接口；本次改动不涉及实例管理员权限。
- 没有做数据库恢复演练；本次没有数据库迁移或数据结构改动。

过程记录：

- 第一次远端备份脚本在读取旧镜像字段时因为 shell 变量展开失败退出，发生在修改 env 前；随后换用 `grep | sed` 读取旧镜像并重新执行成功。

回滚方式：

```bash
ssh family-growth-tencent '
  cp /opt/neudrive/backups/deploy-20260615T130436Z/neudrive.env /opt/neudrive/config/neudrive.env &&
  cp /opt/neudrive/backups/deploy-20260615T130436Z/docker-compose.yml /opt/neudrive/deploy/tencent/docker-compose.yml &&
  cd /opt/neudrive/deploy/tencent &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml up -d db server &&
  docker compose -p neudrive --env-file /opt/neudrive/config/neudrive.env -f docker-compose.yml ps
'
```

如果只回滚镜像，也可以把 `/opt/neudrive/config/neudrive.env` 中的 `NEUDRIVE_IMAGE` 改回 `vola:staging-20260614-registration-lock-amd64`，再执行同一条 `docker compose ... up -d db server`。
