# Production Deploy

Recommended server layout:

```text
~/apps/vola/
  bin/
    deploy-main
    status
  config/
    vola.env
  k8s/
  logs/
  repo/
```

`repo/` is a clean git checkout of `git@github.com:agi-bar/vola.git`.
`config/vola.env` is copied from `repo/vola.env.example` and holds the
real deployment settings and secrets.

`bin/deploy-main` is a thin wrapper that:

1. moves into `repo/`
2. updates to the latest `origin/main`
3. runs `deploy/prod/deploy.sh`
4. writes a timestamped log file under `logs/`

`deploy/prod/deploy.sh` builds the Docker image directly inside the minikube
Docker daemon, syncs the tracked manifests into `k8s/`, creates or updates the
runtime secrets/config from `config/vola.env`, applies the Kubernetes
manifests, updates the deployment image, waits for rollout, and verifies the
public healthcheck.

The production manifests mount `/data/git-mirrors` as a persistent volume and
set `GIT_MIRROR_HOSTED_ROOT` through the `vola-config` ConfigMap. The
healthcheck only proves that the HTTP service is alive; backup readiness should
be checked through the admin-only `/api/ops/status` endpoint and a real restore
drill.

Useful commands from the server:

```bash
cp ~/apps/vola/repo/vola.env.example ~/apps/vola/config/vola.env
vim ~/apps/vola/config/vola.env
~/apps/vola/bin/deploy-main
~/apps/vola/bin/status
```

Required settings for the release candidate:

- `POSTGRES_PASSWORD`
- `JWT_SECRET`
- `VAULT_MASTER_KEY`
- `PUBLIC_BASE_URL`
- `GIT_MIRROR_HOSTED_ROOT`，production 默认写入 `/data/git-mirrors`
- `INSTANCE_ADMIN_USER_IDS`，逗号分隔的实例管理员用户 UUID，用于整站用户管理和实例级运维状态
- `GITHUB_APP_CLIENT_ID`、`GITHUB_APP_CLIENT_SECRET`、`GITHUB_APP_SLUG`，用于 hosted GitHub Backup 的 GitHub App 授权
- `GITHUB_CLIENT_ID`、`GITHUB_CLIENT_SECRET`，用于通用 GitHub OAuth 路径；如果只开放 GitHub App user backup，可以留空
- `USER_STORAGE_QUOTA_BYTES`，默认每用户存储额度；现在示例默认 100MB，单个账号额度可以通过 admin API 设置
- `OBJECT_STORAGE_BACKEND=cos`、`TENCENT_COS_BUCKET`、`TENCENT_COS_REGION`、`TENCENT_COS_SECRET_ID`、`TENCENT_COS_SECRET_KEY`，用于把二进制文件存到腾讯 Lighthouse COS；Secret 不要提交到仓库

## Backup and Restore

Before opening a production instance to users:

1. Keep `JWT_SECRET` and `VAULT_MASTER_KEY` in a separate secure place.
2. Configure at least one remote backup destination: GitHub Backup, WebDAV, or
   S3-compatible storage.
3. Run a Postgres logical backup and copy it off the server.
4. Run one restore drill in a temporary environment.
5. Check `/api/ops/status` with an admin token and record the result.

Detailed runbook:

- [`docs/deployment-reliability.zh-CN.md`](../../docs/deployment-reliability.zh-CN.md)
- [`docs/account-storage-mobile-sync.zh-CN.md`](../../docs/account-storage-mobile-sync.zh-CN.md)

## Bundle Sync Prod-like 验收

上线或大版本同步改动后，建议在这台 prod-like 机器上额外跑一次 Bundle Sync 验收。

推荐顺序：

1. 部署最新 `origin/main`
2. 在管理后台生成一个 `both` Sync Token
3. 用匿名 fixture 跑 `export -> preview -> push -> pull -> diff`
4. 再用一套真实 `.ndrvz` 跑同样流程
5. 单独验证一次 archive `resume`
6. 单独验证一次 `mirror` 删除边界

完整 Runbook 见：

- [`docs/sync-prodlike-acceptance.md`](../../docs/sync-prodlike-acceptance.md)
- [`docs/sync.md`](../../docs/sync.md)
