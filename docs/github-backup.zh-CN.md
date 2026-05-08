[English](github-backup.md) | 简体中文

# GitHub Backup 指南

GitHub Backup 会把用户在 neuDrive 里可见的文件树同步到一个 Git 仓库。它的用途是提供可恢复的版本历史：skills、memory 文件、project 笔记和其他公开 Hub 文件都可以备份到 GitHub，并且可以用普通 Git 工具查看。

## 会备份什么

GitHub Backup 会保持 neuDrive 里看到的路径结构，例如：

```text
skills/...
memory/...
project/...
```

Secrets 不会导出。账号内部元数据、连接记录、vault scope 元数据、billing 状态和服务实现细节也不会写进备份仓库。

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

`GIT_MIRROR_HOSTED_ROOT` 在 hosted 模式下没有内建默认值。如果没有配置，同步会返回：

```text
GIT_MIRROR_HOSTED_ROOT is not configured
```

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
      claimName: neudrive-git-mirror
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
