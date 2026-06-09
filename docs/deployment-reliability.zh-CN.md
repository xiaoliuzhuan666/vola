# Vola 部署可靠性与恢复手册

本文面向 hosted / self-hosted 部署，重点说明数据在哪里、备份在哪里、服务异常时怎么确认和恢复。

## 数据分层

| 类型 | 位置 | 说明 |
| --- | --- | --- |
| 主存储 | Postgres 或本地 SQLite | profile、memory、projects、skills、vault 元数据、token、连接配置都以主存储为准 |
| Git mirror 工作目录 | `GIT_MIRROR_HOSTED_ROOT` 或本地配置路径 | 把可见 Hub 文件树转成 Git 历史，用于 GitHub Backup |
| GitHub 备份仓库 | 用户配置的私有仓库 | 保存可见文件树的版本历史，不包含原始 secret |
| WebDAV / S3-compatible 目标 | 坚果云 WebDAV、OSS、R2、MinIO、S3 等 | 上传 Vola 导出 zip，适合放在服务器之外 |
| 配置与 secret | `.env`、K8s Secret、Vault 字段 | `JWT_SECRET`、`VAULT_MASTER_KEY`、GitHub OAuth/App secret、备份目标密码必须单独保存 |

单台服务器、单个 PVC、单个数据库实例都不能算高可用备份。生产环境至少需要一个离开当前服务器的备份目标。

## 部署前检查

1. `PUBLIC_BASE_URL` 是最终访问域名，OAuth 回调和 GitHub App 都依赖它。
2. `JWT_SECRET` 和 `VAULT_MASTER_KEY` 已记录在安全位置，恢复时必须使用原值。
3. Postgres 数据卷有持久化配置。
4. hosted Git mirror 设置了 `GIT_MIRROR_HOSTED_ROOT=/data/git-mirrors`，并挂载 PVC。
5. GitHub Backup 或 WebDAV / S3-compatible 外部目标至少配置一个。
6. 已记录一份恢复演练时间、备份对象名和负责人。

## K8s 持久化

`deploy/k8s/app.yaml` 已包含 `vola-git-mirrors` PVC，并挂载到 `/data/git-mirrors`。

`deploy/prod/deploy.sh` 会把 `GIT_MIRROR_HOSTED_ROOT` 写入 `vola-config`。未显式设置时使用：

```bash
GIT_MIRROR_HOSTED_ROOT=/data/git-mirrors
```

如果生产集群没有默认 StorageClass，需要在 PVC 里补充 `storageClassName`，或由运维提前创建同名 PVC。

## Docker Compose 持久化

`docker-compose.yml` 使用 Postgres 作为主存储，并给 hosted Git mirror 挂载 `gitmirrors` volume：

```yaml
DATABASE_URL=postgres://...@db:5432/vola?sslmode=disable
GIT_MIRROR_HOSTED_ROOT=/data/git-mirrors
```

`POSTGRES_PORT` 只用于宿主机端口映射。`server` 容器访问同一个 Compose 网络里的 `db` 服务时固定使用容器内端口 `5432`，避免把宿主机端口改成 `5433`、`15432` 等值后导致服务连错端口。

这个 volume 只保存可见文件树的 Git working tree；完整服务恢复仍然需要 Postgres 备份、原 `JWT_SECRET`、原 `VAULT_MASTER_KEY`，以及至少一个离开当前服务器的备份目标。

## 命名兼容配置

新部署统一使用 `vola.env`、`VOLA_ENV_FILE`、`VOLA_HOST_PORT` 和 `VOLA_*` 配置名。部分部署脚本仍会读取 `neudrive.env`、`NEUDRIVE_ENV_FILE`、`NEUDRIVE_HOST_PORT`，这是为了让旧服务器自动化继续可用，不是新部署推荐命名。

API source header 推荐使用 `X-Vola-Platform` / `X-Vola-Source`。旧客户端仍可继续发送 `X-NeuDrive-Platform` / `X-NeuDrive-Source`。

## 账号和容量

默认每用户存储额度由 `USER_STORAGE_QUOTA_BYTES` 控制。可以在管理接口里为单个用户设置独立额度，覆盖全局默认值。详细操作见：

- [`docs/account-storage-mobile-sync.zh-CN.md`](./account-storage-mobile-sync.zh-CN.md)

## Postgres 备份

建议每天生成一份逻辑备份，并放到服务器之外，例如对象存储、另一台机器或受控备份系统。

备份命令示例：

```bash
mkdir -p ~/apps/vola/backups/postgres
pg_dump "$DATABASE_URL" \
  --format=custom \
  --no-owner \
  --file "~/apps/vola/backups/postgres/vola-$(date -u +%Y%m%dT%H%M%SZ).dump"
```

恢复到新库示例：

```bash
createdb vola_restore
pg_restore \
  --dbname "postgres://USER:PASSWORD@HOST:5432/vola_restore?sslmode=disable" \
  --clean \
  --if-exists \
  --no-owner \
  vola-YYYYMMDDTHHMMSSZ.dump
```

恢复演练不要直接对生产库执行。先恢复到临时库，确认服务能启动、用户能登录、Skills / Memory / Projects 能读取，再决定正式迁移。

## 外部备份目标

页面路径：`GitHub 备份`。

可用目标：

- GitHub：保存可见 Hub 文件树的 Git 版本历史。
- WebDAV：适合坚果云、Nextcloud、Synology 等支持 WebDAV 的网盘。
- S3-compatible：适合 AWS S3、Cloudflare R2、阿里云 OSS、MinIO 等。

WebDAV / S3-compatible 目标上传的是 Vola 导出 zip。它适合做离开服务器的恢复包，但它不替代数据库备份，因为账号、token、连接关系、备份目标配置等仍在数据库里。

外部目标可以开启自动备份计划。每个目标可设置间隔小时数，后台任务默认每小时检查一次到期目标，到期后生成 Vola 导出 zip 并上传到对应目标。手动和自动上传都会写入备份历史，记录触发来源、对象名、大小、耗时和错误。

每个外部目标可以设置保留策略：保留最近 N 份、保留 N 天，或者两者都不启用。清理只基于 Vola 历史记录里成功上传的 `vola-export-*.zip` 对象，不会扫描或删除第三方文件，也不会删除最近一次成功备份。真实 WebDAV / S3 provider 的删除兼容性仍需要在上线前验证。

恢复入口分两步：在 `GitHub 备份` 页面上传 Vola 导出 zip，先查看 Skills、Memory、Projects、Vault、Roles、Inbox 等分类和风险提示；确认后再选择“跳过已有文件”或“覆盖已有文件”应用恢复。恢复应用会拒绝包含路径穿越的 ZIP，并写回 Hub 文件树。Vault 恢复只写回导出包里的范围清单，secret 原值仍需要数据库备份或密钥系统。

## 运维状态接口

管理员 token 可访问：

```bash
curl -fsS "$PUBLIC_BASE_URL/api/ops/status" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

返回内容包括：

- 主存储类型和本地/hosted 模式。
- Git mirror 执行模式、远端仓库、最近同步/推送状态。
- WebDAV / S3-compatible 目标数量、启用数量、最近成功上传、最近错误。
- 外部备份目标的自动备份开关、间隔和最近自动执行时间。
- 外部备份目标的保留策略、最近备份运行历史、最近运行状态和错误。
- 当前检查结果：`ok`、`warning`、`critical`。
- 相关文档路径。

`/api/health` 仍只用于服务存活检查，不能证明备份有效。备份是否有效看 `/api/ops/status` 和真实恢复演练。

## 告警建议

至少监控这些条件：

- `/api/health` 非 2xx。
- `/api/ops/status` 的 `status` 为 `critical`。
- `backup.last_error` 非空。
- `backup.last_run_status = failed`。
- `git_mirror.last_error` 或 `git_mirror.last_push_error` 非空。
- 超过 24 小时没有新的 Postgres 备份文件。
- 超过 24 小时没有新的外部备份对象。

## 恢复演练清单

每次大版本上线后或每月至少做一次：

1. 找到最近一份 Postgres dump。
2. 找到最近一个 WebDAV / S3-compatible zip 或 GitHub Backup 仓库提交。
3. 在临时环境恢复 Postgres。
4. 使用原 `VAULT_MASTER_KEY` 和 `JWT_SECRET` 启动服务。
5. 如果需要恢复可见文件树，从 GitHub Backup clone，或在 `GitHub 备份` 页面上传导出 zip，先预览，再选择跳过或覆盖策略应用恢复。
6. 登录后台检查 Skills、Memory、Projects、Vault scope 列表和 GitHub Backup 页面。
7. 调用 `/api/ops/status`，记录返回状态、最近备份对象名、最近运行状态和检查时间。
8. 删除临时环境中的敏感数据。

## 不能替代的事项

- GitHub Backup 不保存原始 secret。
- WebDAV / S3 zip 不替代数据库级备份。
- PVC 只解决 Pod 重建后的文件保留，不解决服务器或集群损坏。
- 换机器恢复时，`VAULT_MASTER_KEY` 必须和原环境一致，否则加密字段无法解密。
