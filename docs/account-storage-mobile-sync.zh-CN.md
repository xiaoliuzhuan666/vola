# 账号、存储额度与手机同步说明

本文记录 Vola 自部署时的账号开通、存储额度、手机端同步和腾讯云 COS 备份建议。

## 账号与存储额度

全局默认额度由环境变量控制：

```bash
USER_STORAGE_QUOTA_BYTES=100MB
```

支持纯字节数，也支持 `100MB`、`1GB`、`10GiB` 这类写法。值为 `0` 表示不限制。

单个账号可以设置独立额度：

- `null`：继承全局默认额度。
- `0`：该账号不限制。
- 大于 `0`：该账号使用指定字节数额度。

额度统计范围是 Vola 文件树里的用户可见数据，包括 Skills、Memory、Projects、Inbox、Vault scope 元数据等；目录本身不计入容量。二进制文件即使存到腾讯 Lighthouse COS，也按文件大小计入用户额度。账号、session、token、连接配置、billing 状态仍在数据库里，不属于这个文件树容量。

## 腾讯 Lighthouse COS 文件存储

默认情况下，二进制文件保存在数据库 `file_blobs.data`。生产环境可以把二进制文件放到腾讯 Lighthouse COS / 腾讯 COS，数据库只保存对象 key、大小和 hash：

```bash
OBJECT_STORAGE_BACKEND=cos
TENCENT_COS_BUCKET=your-bucket-1250000000
TENCENT_COS_REGION=ap-guangzhou
TENCENT_COS_ENDPOINT=
TENCENT_COS_SECRET_ID=your-secret-id
TENCENT_COS_SECRET_KEY=your-secret-key
TENCENT_COS_PREFIX=vola
TENCENT_COS_PATH_STYLE=0
```

`TENCENT_COS_ENDPOINT` 留空时会使用 `https://cos.<region>.myqcloud.com`。新开通的 COS/Lighthouse COS 建议保持 `TENCENT_COS_PATH_STYLE=0`，使用 bucket 子域名访问。不要把 SecretId、SecretKey 写进仓库。

需要注意的是，COS 文件存储只替代二进制内容本身，Postgres 仍然是账号、权限、文件树、同步历史和存储额度的主数据库。生产备份仍需要 Postgres 备份，COS 不能单独恢复整个系统。

## 管理账号 API

这些接口需要 admin scoped token。自部署时可以先用本地 owner token 或后台生成的 admin token 操作。

列出账号：

```bash
curl -fsS "$PUBLIC_BASE_URL/api/admin/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

创建账号并指定额度：

```bash
curl -fsS "$PUBLIC_BASE_URL/api/admin/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "change-this-password",
    "display_name": "Demo User",
    "slug": "demo-user",
    "storage_quota_bytes": 1073741824
  }'
```

修改账号额度：

```bash
curl -fsS "$PUBLIC_BASE_URL/api/admin/users/<USER_ID>/quota" \
  -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"storage_quota_bytes": 2147483648}'
```

改回继承全局额度：

```bash
curl -fsS "$PUBLIC_BASE_URL/api/admin/users/<USER_ID>/quota" \
  -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"storage_quota_bytes": null}'
```

## 手机端同步方式

手机端不应该理解成“把云端文件直接写进 iOS/Android 系统目录”。手机 App 有沙箱限制，也不适合让第三方 Agent 随便改本地文件。更合理的方式是云端为主、手机缓存为辅：

1. 手机端通过 Vola 登录拿到 JWT 或 scoped token。
2. 首次打开时调用云端 API 读取数据，例如 `/api/tree/snapshot`、`/api/dashboard/stats`、`/api/memory/profile`。
3. 需要离线时，把读取到的文件树和 cursor 缓存在手机本地 SQLite 或 App 私有目录。
4. 下次启动用 `/api/tree/changes?cursor=...` 读取增量变更。
5. 手机上新增或修改的数据通过 `/api/tree/*` 写回，或通过 Bundle Sync 的 import session 上传。
6. 如果同一份数据在多端同时修改，以服务端版本、checksum、sync history 做冲突提示。

如果用户在手机里的 ChatGPT、Claude 等 App 里使用 Vola，通常不需要把数据同步到手机文件系统。那些 Agent 通过云端 MCP / OAuth 访问 Vola，数据仍然保存在服务器里。

## 腾讯云 COS 是否需要

Docker 部署本身不依赖 COS。服务运行需要的是：

- Postgres 持久化数据卷。
- `GIT_MIRROR_HOSTED_ROOT` 对应的持久目录。
- 正确保存 `JWT_SECRET` 和 `VAULT_MASTER_KEY`。

COS 更适合作为离开当前服务器的备份目标。建议生产环境至少配置一个外部备份目标，COS、GitHub Backup、WebDAV、R2、OSS、MinIO 都可以。

腾讯云 COS 接入建议：

```json
{
  "kind": "s3",
  "name": "Tencent COS Backup",
  "enabled": true,
  "s3_endpoint": "https://cos.ap-guangzhou.myqcloud.com",
  "s3_bucket": "your-bucket-name",
  "s3_region": "ap-guangzhou",
  "s3_prefix": "vola",
  "s3_access_key_id": "AKID...",
  "s3_path_style": false,
  "auto_backup_enabled": true,
  "auto_backup_interval_hours": 24,
  "retention_keep_last": 7
}
```

注意事项：

- 腾讯云 COS 按存储容量、请求、流量等计费；上线前看当前账号的免费额度和计费规则。
- COS 的 S3 兼容接入应优先使用 virtual-hosted-style，所以 `s3_path_style` 建议设为 `false`。
- 上线前必须用真实 COS bucket 做一次上传、保留策略清理、恢复预览验证；本地接收器验收不能证明真实 COS 兼容。

参考文档：

- 腾讯云 COS 按量计费：`https://cloud.tencent.com/document/product/436/36522`
- 腾讯云 COS 免费额度：`https://cloud.tencent.com/document/product/436/6240`
- 腾讯云 COS 使用 AWS S3 SDK 访问：`https://cloud.tencent.com/document/product/436/37421`
