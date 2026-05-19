[English](github-backup.md) | 简体中文

# GitHub Backup 指南

GitHub Backup 会把用户在 neuDrive 里可见的文件树同步到一个 Git 仓库。它的用途是提供可恢复的版本历史：skills、memory 文件、project 笔记和其他公开 Hub 文件都可以备份到 GitHub，并且可以用普通 Git 工具查看。

## 存储模型

neuDrive 里的数据分三层：

- 主存储：Hosted 部署通常是 Postgres，本地模式通常是 SQLite。Hub 的当前状态先写在这里。
- Git working tree：neuDrive 把用户可见文件树写成 Git 仓库工作副本，用于生成版本历史。
- 远端备份：GitHub Backup 会把 Git working tree 推送到 GitHub；WebDAV、S3-compatible、OSS、R2 目标会上传 neuDrive 导出 zip，让备份离开当前机器或服务器。

GitHub Backup 不能替代数据库备份。它保存的是用户可见文件树，适合恢复 skills、team 资料、memory 文件、project 笔记等内容；账号、连接、billing、vault scope 元数据和 secret 仍需要依赖数据库备份或服务配置恢复。

当前可用的远端备份有两类：GitHub 版本历史，以及 WebDAV / S3-compatible 导出包上传。R2、OSS 等对象存储需要使用兼容 S3 Signature V4 的 endpoint。

## 会备份什么

GitHub Backup 会保持 neuDrive 里看到的路径结构，例如：

```text
skills/...
team/...
memory/...
project/...
```

Secrets 不会导出。账号内部元数据、连接记录、vault scope 元数据、billing 状态和服务实现细节也不会写进备份仓库。

## 恢复方式

如果已经同步到 GitHub，可以先 clone 远端仓库查看历史版本：

```bash
git clone https://github.com/<owner>/neudrive-backup.git
```

恢复单个 Skill、memory 文件或 project 文件时，可以从 Git 历史里取回对应路径，再通过 neuDrive 的导入或同步命令写回 Hub。完整服务恢复还需要数据库备份，因为 GitHub Backup 不包含账号内部数据和 secret。

本地文件树重新导入可以使用同步命令：

```bash
neu sync push --source ./recovered-neudrive-files
```

执行前请先确认目录里只包含希望写回 neuDrive 的文件。

如果备份到了 WebDAV 或 S3-compatible 目标，下载对应的 `neudrive-export-*.zip`。这个 zip 是 neuDrive 导出包，包含可恢复的文件树、profile、memory、projects、roles、inbox 和 vault scope 元数据；不包含 secret 明文。

恢复流程：

1. 从 WebDAV 或对象存储下载 zip。
2. 在 `GitHub Backup` 页面上传 zip，先执行恢复预览，确认分类、文件数量、风险提示。
3. 选择应用策略：`跳过已有文件` 适合追加恢复，`覆盖已有文件` 适合以备份包为准恢复同名文件。
4. 点击应用恢复，把 zip 中可识别的 Skills、Memory、Projects、Roles、Inbox、Identity 和 Vault scope 清单写回 Hub 文件树。

完整服务恢复仍然需要数据库备份。GitHub / WebDAV / S3-compatible 备份主要覆盖用户可见数据和可移植导出包，不覆盖登录 session、连接 token、billing 状态和 secret 明文。

恢复应用会拒绝包含路径穿越的 ZIP。Vault 恢复只写回范围清单，不恢复 secret 原值；secret 原值需要数据库备份或密钥系统。

## WebDAV / S3-compatible 外部备份

在 `GitHub Backup` 页面下方的 `外部备份目标` 区域可以新增目标。

WebDAV 目标需要：

- WebDAV 目录 URL，例如坚果云、Nextcloud 或自建 WebDAV 的目录地址。
- 用户名。
- 密码或应用密码。保存后不会在页面回显。

同步时 neuDrive 会生成 `neudrive-export-YYYYMMDD-HHMMSSZ.zip`，使用 WebDAV `PUT` 上传。如果填写了多级对象路径，服务会尝试用 `MKCOL` 创建目录。

S3-compatible 目标需要：

- Endpoint，例如 Cloudflare R2、MinIO、支持 S3 API 的 OSS endpoint。
- Bucket。
- Region。R2 可以用 `auto`；其他服务按对应存储服务要求填写。
- Prefix，可选。
- Access key ID 和 Secret access key。Secret 保存后不会在页面回显。
- URL 样式。默认使用 path-style：`<endpoint>/<bucket>/<prefix>/<object>`；如果服务要求 virtual-hosted style，可以关闭 Path-style URL。

S3-compatible 上传使用 AWS Signature V4，对象名同样是 `neudrive-export-YYYYMMDD-HHMMSSZ.zip`。

外部目标可开启自动备份和保留策略：

- 自动备份：按目标设置的小时间隔，由后台任务定时生成并上传导出 zip。
- 备份历史：记录每次手动或自动运行的触发来源、对象名、大小、耗时和错误。
- 保留策略：可设置保留最近 N 份或保留 N 天。清理只处理 neuDrive 历史记录中的 `neudrive-export-*.zip`，不会处理第三方文件，也不会删除最近一次成功备份。

## Hosted 模式

Hosted 部署建议只使用 GitHub App user 授权。

用户流程：

1. 打开 `GitHub Backup`。
2. 点击 `Connect GitHub`。
3. 授权完成后，neuDrive 会在当前 GitHub 用户下创建或复用一个私有仓库 `neudrive-backup`。
4. 点击 `Sync now`，把当前 neuDrive 文件树写入 Git working tree，并推送到 `origin/main`。

Hosted 模式会把普通用户界面保持得很简单：

- 认证方式固定为 `github_app_user`。
- Remote name 固定为 `origin`。
- 目标分支固定为 `main`。
- 自动 commit 和自动 push 默认开启。
- 手动同步默认 60 秒内最多触发一次。

手动同步冷却时间可以用这个环境变量调整：

```bash
GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS=60
```

设置为 `0` 可以关闭冷却时间。

## Hosted 部署配置

Hosted Git working tree 的路径规则是：

```text
$GIT_MIRROR_HOSTED_ROOT/<user_id>
```

应用自身在 hosted 模式下没有内建 `GIT_MIRROR_HOSTED_ROOT` 默认值。如果没有配置，同步会返回：

```text
GIT_MIRROR_HOSTED_ROOT is not configured
```

`deploy/prod/deploy.sh` 会在生产 ConfigMap 中默认写入 `/data/git-mirrors`。如果不用该脚本部署，需要自己设置这个环境变量并挂载可写目录。

Kubernetes 推荐挂载方式：

```yaml
env:
  - name: GIT_MIRROR_HOSTED_ROOT
    value: /data/git-mirrors

volumeMounts:
  - name: git-mirror-data
    mountPath: /data/git-mirrors

volumes:
  - name: git-mirror-data
    persistentVolumeClaim:
      claimName: neudrive-git-mirrors
```

同步代码会自动创建每个用户自己的目录，但服务进程必须对挂载的父目录有写权限。多副本部署时，建议使用 RWX volume，或者确保只有一个 pod 运行 Git mirror sync worker 指向同一个 root。

Hosted GitHub App 授权还需要这些环境变量：

```bash
GITHUB_APP_CLIENT_ID=...
GITHUB_APP_CLIENT_SECRET=...
GITHUB_APP_SLUG=...
PUBLIC_BASE_URL=https://your-neudrive-host
JWT_SECRET=...
```

GitHub App 还必须申请这些 repository 权限：

- Administration: read and write。neuDrive 创建私有 `neudrive-backup` 仓库时会调用 GitHub 的 `POST /user/repos` API，这个权限是必需的。
- Contents: read and write。后续把备份内容 push 到仓库时需要这个权限。

在 GitHub 里修改 App 权限后，用户需要批准新的权限，或重新连接 GitHub，然后再重试创建仓库。

## 本地模式

本地部署可以选择三种认证方式：

- 本机 Git 凭证：使用当前机器的 SSH key 或 credential helper。GitHub SSH 地址使用 `git@github.com:owner/repo.git`。
- GitHub token：适合 HTTPS 仓库 URL。
- GitHub App user：和 hosted 模式一样走 GitHub 用户授权。

本地模式默认不限制手动 `Sync now` 的频率。如果你也希望本地模式有冷却时间，可以设置 `GIT_MIRROR_MANUAL_SYNC_COOLDOWN_SECONDS`。

对应的 CLI 命令：

```bash
neu git init --output ./neudrive-export/git-mirror
neu git pull
neu git auth github-app --device
```

## 远端更新与冲突

neuDrive 把 mirror 当作备份目标。如果远端分支上有本地 mirror 没有的提交，普通 push 会被阻止，并显示 remote conflict。你需要先确认远端改动；如果确定要以 neuDrive 当前 mirror 为准，可以在 UI 里执行 overwrite。overwrite 使用 `--force-with-lease`，避免误覆盖已经再次变化的远端分支。

## 常见问题

- `GIT_MIRROR_HOSTED_ROOT is not configured`：配置这个环境变量，并挂载一个可写目录。
- hosted root 下权限不足：用 `securityContext.fsGroup` 或 initContainer 修正 PVC 权限。
- 本机 Git 凭证填写了 GitHub HTTPS URL：改用 GitHub token 模式，或者把仓库地址改成 `git@github.com:owner/repo.git`。
- 备份里没有导入的文件：确认当前 neuDrive 用户真的拥有这些文件。系统内置 skills 即使用户自己的 `file_tree` 为空，也可能在页面里可见。
