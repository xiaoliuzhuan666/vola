# 多 Agent Skill 目标规则

本文记录 Vola 当前对 Claude Code、Codex、Cursor、Gemini CLI 的 Skill 同步边界。

## 支持状态

| Agent | 当前状态 | 本地写入 | 导出包 |
| --- | --- | --- | --- |
| Claude Code | 可自动同步 | `~/.claude/skills` | 支持 |
| Codex | 可自动同步 | `~/.agents/skills` | 支持 |
| Cursor | 可分配、可导出、暂不自动写入 | 不自动写入 | 支持 |
| Gemini CLI | 可分配、可导出、暂不自动写入 | 不自动写入 | 支持 |

## Claude Code

- 默认目标目录是 `~/.claude/skills`。
- 只有已经分配给 Claude Code 的 Skill 会被写入目标目录。
- Vola 会在写入的 Skill 目录内创建 `.vola-managed.json`。
- 预览和应用只会更新 Vola 管理过的目录；如果目标目录里已经存在同名但没有标记的 Skill，会显示冲突，不会覆盖。
- 清理只删除带 `.vola-managed.json` 且已经取消分配的 Skill。

## Codex

- 默认目标目录是 `~/.agents/skills`。
- 行为与 Claude Code 一致：只写分配 Skill，只更新带 `.vola-managed.json` 的目录。
- 从 Claude Code 转换到 Codex 的简单 Skill，可以进入 `/skills/<name>-codex`，再通过本地同步写入 Codex Skill 目录。

## Cursor

- 当前状态是“可分配、可导出、暂不自动写入”。
- Vola 会记录 Cursor 的 Skill 分配，并在本地同步预览里生成导出项。
- 导出包包含完整 Skill 文件夹：`SKILL.md`、scripts、依赖文件、assets、`external/claude-tools`、`external/claude-plugins`、`manifest.vola.json`。
- 如果某个项目使用 `.cursor/rules`，用户需要手动阅读导出包，把适合 Cursor 的内容整理到项目规则里。
- Vola 不自动修改 Cursor 全局配置或项目配置。

## Gemini CLI

- 当前状态是“可分配、可导出、暂不自动写入”。
- Vola 会记录 Gemini CLI 的 Skill 分配，并在本地同步预览里生成导出项。
- 导出包同样保留完整 Skill 文件夹和 manifest。
- 用户需要手动把可用内容引用或整理到自己项目已有的 `GEMINI.md` 或其他 Gemini CLI 指南文件中。
- Vola 不自动修改 `GEMINI.md` 或 Gemini CLI 配置。

## 转换边界

- Claude Code 和 Codex 之间支持 Hub 内转换预览和生成副本。
- 转换不会丢弃脚本、依赖文件、assets、二进制资源、外部 Claude tools/plugins 文件。
- `~/.claude/tools/...` 这类已纳入包内的引用，在转换到 Codex 时会改写为 `external/claude-tools/...`。
- `~/.claude/plugins/...` 这类已纳入包内的引用，在转换到 Codex 时会改写为 `external/claude-plugins/...`。
- hooks、MCP、plugin 配置会出现在转换报告里，由用户手动处理。它们会作为文件复制到转换副本中，但 Vola 不会自动注册、安装或启用。
- Vola 不自动安装 MCP server，不自动启用 plugin，不自动修改 hook。

## 安全规则

- 自动写入只允许写 Agent 对应的 Vola 管理目录。
- `.vola-managed.json` 是更新和清理的判断依据。
- 同名目录如果不是 Vola 管理，预览会显示冲突，应用时不会写入。
- 清理只处理已取消分配且带 Vola 标记的目录，不删除用户手工创建的 Skill。

## 团队资产到本机

- Codex 和 Claude Code 支持团队 Skill 自动同步到本地管理目录。
- Codex 和 Claude Code 的团队 MCP 可通过本机同步入口刷新到当前机器的客户端配置。
- Cursor 和 Gemini CLI 只做导出预览或导出包，不自动写本机配置。
- 同名但不是 Vola 管理的本机目录不会被覆盖。
- Team Library 和 MCP Hub 会显示每个平台的状态、同步模式、配置路径和下一步动作；状态接口为 `GET /api/local/tools/status`。
- 团队 Skill 可以带版本号、发布说明、标签、发布状态和审查状态；这些信息只影响展示和同步判断，不绕过本机写入规则。

## 当前验证证据

以下测试覆盖当前声明的主路径：

- `internal/api/sqlite_shared_test.go`
  - 导入复杂 Skill zip 后生成 `manifest.vola.json`，识别 scripts、requirements、package、assets、二进制、`external/claude-tools`、`external/claude-plugins`、环境变量提示。
  - Cursor 和 Gemini CLI 的分配会出现在本地同步预览中，但只有 `export` 项，不产生本地写入计划；导出包保留完整 Skill 文件夹。
  - Claude Code / Codex 本地应用只写目标管理目录，并用 `.vola-managed.json` 标记；同名非管理目录会返回冲突，不覆盖。
  - Claude Code / Codex 转换报告区分自动处理项、需要手动处理项和暂不自动转换项；MCP、plugin、hook 进入报告，不自动启用。
- `internal/platforms/claude_migration_test.go`
  - Claude Code 本地扫描会把被引用的 `~/.claude/tools/...` 和 `~/.claude/plugins/...` 文件纳入 Skill 包，并保留依赖文件与二进制 assets。
- `internal/storage/sqlite/client_agent_export_claude_test.go`
  - Claude Code / Codex agent export 导入会写入便携 manifest，保留脚本、依赖、二进制 assets、外部引用和 Codex plugin 元数据。

建议验证命令：

```bash
go test ./internal/skillsarchive ./internal/platforms ./internal/api ./internal/storage/sqlite
```
