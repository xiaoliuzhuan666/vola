[English](README.md) | 简体中文

# neuDrive

<p align="center">
  <img src="docs/assets/neudrive-logo.png" alt="neuDrive logo" width="320" />
</p>

**Agent 个人数据 Hub**

> 一个地方，保存 Claude、ChatGPT、Codex、Cursor 等 Agent 可使用的个人资料、记忆、项目上下文、技能和访问权限。

---

## 这是什么

neuDrive 给每个人一个 Agent 数据 Hub。Claude、ChatGPT、Codex、Cursor、Copilot、飞书、Kimi、智谱等 Agent 可以通过这个 Hub 读取被授权的 profile、memory、projects、conversations、skills 和 vault 资料，减少在每个平台重复维护上下文。

你可以自己部署 neuDrive，运行属于自己的 Hub。我也提供了一个已经部署好的版本，可以直接在 [https://www.neudrive.ai](https://www.neudrive.ai) 体验。使用兑换码 `VIVO50` 可以获得三个月免费服务；三个月到期后，你可以选择继续续费、继续使用可用的托管方案，或者切换到自己部署的版本。

**你的 profile、memory、projects、skills 和私密资料跟着人走，不跟平台走。**

## 功能特性

- **Agent 个人数据 Hub**：集中保存 profile、memory、projects、conversations、skills 和 Agent 通信记录，让上下文跟着你跨工具流动。
- **跨平台 AI 接入**：通过 hosted OAuth、Remote MCP 或本地 adapter 连接 Claude、ChatGPT、Cursor、Windsurf、Codex CLI、Gemini CLI、飞书和自定义 MCP 客户端。
- **记忆、项目和技能迁移**：从 Agent 工具导入 skills、项目上下文、profile/preferences、会话和 notes，也可以通过 CLI、API、Bundle Sync 导出或恢复。
- **Team Library 第一阶段**：小团队可以把内部 Skill、MCP 配置说明、提示词和 AI 使用技巧放进团队资料库，并进入备份、搜索和 MCP 访问链路。[查看文档](docs/team-ai-library.zh-CN.md)
- **秘密和信任控制**：secrets 放在统一 vault 中，通过 scoped token 和 trust level 控制每个 Agent 能访问什么。
- **Agent 协作**：Agent 可以互发消息、写入项目日志、交接任务，不需要你在工具之间手动复制上下文。
- **面向开发者的数据接口**：提供 canonical virtual tree、typed HTTP API、MCP tools、文件树读写和 `snapshot/changes` 同步接口。
- **GitHub Backup**：把 neuDrive 可见文件树同步到私有 GitHub 仓库，保留可恢复版本历史。[查看文档](docs/github-backup.zh-CN.md)
- **Hosted 或自托管**：可以直接使用 [neudrive.ai](https://www.neudrive.ai)，也可以部署自己的 Hub，使用本地或远端存储。

## 当前边界

- GitHub Backup 备份的是用户可见文件树，不包含 secret 明文，也不能替代 Postgres 备份。
- WebDAV / S3-compatible 备份上传的是 neuDrive 导出 zip，适合离开当前服务器保存恢复包；账号、连接、billing、session 等仍以数据库备份为准。
- Skill 自动写入当前只对 Claude Code 和 Codex 开放：Claude Code 写 `~/.claude/skills`，Codex 写 `~/.codex/skills`，且只更新带 `.neudrive-managed.json` 的目录；Cursor、Gemini CLI 可分配、可预览、可导出，但 neuDrive 不自动修改它们的本地配置。详细边界见 [多 Agent Skill 目标规则](docs/agent-skill-targets.zh-CN.md)。
- Claude / Codex Skill 转换会保留脚本、依赖、assets、二进制资源和外部引用文件；MCP、plugin、hook 只生成报告，需要用户手动配置。
- Hosted OAuth、ChatGPT Apps、Claude Connectors 等能力受对应平台账号计划、灰度和平台规则影响。
- Team Library 当前按小团队共享资料库设计，不等同于企业级组织管理、审计报表、SSO 或审批流。

本仓库里的 hosted service 示例统一使用：

- Hub 地址：`https://www.neudrive.ai`
- MCP 地址：`https://www.neudrive.ai/mcp`

## 从这里开始

按你的接入方式选第一个合适的入口：

1. **Web / Desktop Apps**：最快接入 Claude、ChatGPT、Cursor、Windsurf 等图形界面，并使用官方云服务 + 浏览器授权。[查看文档](docs/setup.zh-CN.md#web-and-desktop-apps)
2. **CLI Apps**：使用 Claude Code、Codex CLI、Gemini CLI、Cursor Agent，通过 Remote HTTP MCP + OAuth 接入。[查看文档](docs/setup.zh-CN.md#cli-apps)
3. **本地模式**：仓库内本地开发、局域网环境，或者当前还没有公网 HTTPS 地址。[查看文档](docs/setup.zh-CN.md#local-mode)
4. **高级模式 / GPT Actions / Adapters**：通用 MCP 客户端、自定义 GPT、Feishu / webhook 等更进阶的接法。[查看文档](docs/setup.zh-CN.md#advanced-mode)

## Web / Desktop Apps

如果你是从 Claude 网页版、ChatGPT、Cursor 或 Windsurf 这类图形界面发起连接，先看这一节。

### Claude Connectors

1. 登录 Claude 网页应用，进入 `Settings -> Connectors -> Go to Customize`。
2. 点击 `Add custom connector`。
3. 把 `Remote MCP Server URL` 填成 `https://www.neudrive.ai/mcp`。
4. 保存后点击 `Connect`。
5. 浏览器会跳转到 neuDrive 的登录与授权页；完成后回到 Claude。

### ChatGPT Apps

1. 登录 ChatGPT，进入 `Settings -> Apps`。
2. 在 `Advanced settings` 里点击 `Create app`。
3. 把 `MCP Server URL` 填成 `https://www.neudrive.ai/mcp`。
4. 按提示完成 neuDrive 登录和授权。

如果你的账号里暂时看不到 `Apps` 入口，通常意味着当前计划或灰度范围还没有开放这一功能。

完成授权后，建议马上打开一个**新的对话**，然后直接发出导入指令，例如：

- `请将我的 skills、projects 和 profile 导入到 neuDrive。`
- `请读取我在 neuDrive 中已有的 profile、skills 和最近的项目上下文，并告诉我里面已经有什么内容。`

Cursor、Windsurf 和更完整的接入变体见：[Web / Desktop Apps](docs/setup.zh-CN.md#web-and-desktop-apps)

## 本地 CLI 快速开始

先在本地安装 CLI：

```bash
git clone https://github.com/agi-bar/neudrive.git
cd neudrive
./tools/install-neudrive.sh
```

安装完成后，默认使用 `neu`；兼容别名 `neudrive` 也仍然可用。

```bash
neu status         # 检查 daemon、本地存储和当前 target 是否就绪
neu platform ls    # 查看已发现的平台 adapter 和连接状态
neu connect claude # 安装 / 配置 Claude 集成
neu browse         # 在浏览器里打开本地 Hub
```

接着在已连接好的客户端里打开一个**新的对话**，直接说一句：`请把这个工作区里有用的 skills、项目上下文和 profile/preferences 导入到 neuDrive。`

详细 CLI 使用见：[CLI 使用手册](docs/cli.zh-CN.md)

## 开发、测试与构建

本地开发常用命令：

```bash
cp neudrive.env.example neudrive.env
set -a; source neudrive.env; set +a
go run ./cmd/neudrive server --listen :8080
cd web && npm run dev
```

前端开发服务器默认访问 `http://localhost:3000`，并把 API 代理到 `http://localhost:8080`。

Release candidate 验证建议至少执行：

```bash
go test ./...
cd web && npm run build
make build
```

`make build` 会重新生成嵌入式前端产物并构建 `bin/neudrive` 与 `bin/neu`。部署前检查清单见：[Release Readiness](docs/release-readiness.zh-CN.md)

## 登录官方云服务

如果你希望登录官方云 Hub，用于 CLI、Claude 等接入流程，可以直接运行：

```bash
neu login
```

这个命令会拉起浏览器登录流程，把官方云 profile 保存到本地，并自动把当前 target 切到这个 profile。

## 文档索引

先看这些：

- [接入说明](docs/setup.zh-CN.md)
- [GitHub Backup 指南](docs/github-backup.zh-CN.md)
- [小范围上线测试清单](docs/launch-test-checklist.zh-CN.md)
- [CLI 使用手册](docs/cli.zh-CN.md)
- [详细参考](docs/reference.zh-CN.md)

英文文档：

- [README](README.md)
- [Setup Guide](docs/setup.md)
- [GitHub Backup Guide](docs/github-backup.md)
- [CLI Guide](docs/cli.md)
- [Reference](docs/reference.md)

更多资料：

- [Token 管理](docs/setup.zh-CN.md#token-management)
- [团队 AI 资料库](docs/team-ai-library.zh-CN.md)
- [Bundle Sync 指南](docs/sync.md)
- [SDK / HTTP API](docs/reference.zh-CN.md#sdk)
- [产品设计文档](docs/design.md)
- [Prod-like 验收 Runbook](docs/sync-prodlike-acceptance.md)
- [安全与资源审计](docs/sync-audit.md)
- [CLI 测试矩阵](docs/cli-test-matrix.md)
