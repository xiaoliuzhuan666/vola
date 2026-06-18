# Vola Skill / MCP / 同步能力核对

日期：2026-06-16

## 结论

Vola 当前已经基本满足“个人管理本地 Skill 和 MCP、小团队共享 Skill / MCP、成员一键更新、跨设备迁移与备份”的产品方向。更准确的定位是：

> Vola 是个人和小团队的 Agent 资料 Hub：个人统一管理 profile、memory、projects、skills、MCP 和备份；团队统一共享 Skill、MCP 配置、Agent 配方、prompt 和 playbook；成员优先通过 Codex，其次通过 Claude Code 连接和更新。

当前适合对外表达为“小团队 Agent 资料与 Skill/MCP 共享中心”。不要写成企业级组织管理、SSO、全量审计或全平台自动安装产品。

## 能力矩阵

| 能力 | 当前状态 | 说明 |
| --- | --- | --- |
| 个人 Skill 管理 | 已支持 | Skill 统一保存在 Hub 的 `/skills`，可创建、导入、编辑、预览和导出。 |
| Skill 分配到 Agent | 已支持 | 支持 Codex、Claude Code、Cursor、Gemini CLI 的分配表。 |
| Codex 本地 Skill 自动写入 | 已支持 | 分配后写入 `~/.agents/skills`，只更新带 `.vola-managed.json` 标记的目录。 |
| Claude Code 本地 Skill 自动写入 | 已支持 | 分配后写入 `~/.claude/skills`，安全规则与 Codex 一致。 |
| Cursor / Gemini Skill | 部分支持 | 可分配、预览、导出 zip，不自动修改本地配置。 |
| Claude / Codex Skill 转换 | 已支持 | Hub 内可预览和生成互通副本；脚本、依赖、assets、二进制资源和外部引用会保留。 |
| 个人 MCP 连接 | 已支持 | `neu connect codex` 和 `neu connect claude` 会配置 Vola MCP；Web MCP Hub 支持 Claude Desktop 注册。 |
| 第三方 MCP 聚合 | 已支持 | Hub 可保存 `/settings/mcp-servers.json`，MCP Gateway 会装载启用的 Stdio MCP 并做工具名前缀隔离。 |
| 团队 Skill 发布 | 已支持 | 团队可发布、归档、审查 Skill，成员可查看团队可见 Skill。 |
| 团队 Skill 一键装配 | 已支持 | 成员可把团队 Skill 安装到个人 `/skills`。 |
| 团队 Skill 一键更新 | 已支持 | 使用指纹判断更新，成员可同步新版；覆盖前会保存个人旧版本备份。 |
| 团队 Skill 更新报表 | 已支持 | 管理员可查看成员安装状态、待更新数量和更新提醒。 |
| 团队 Skill diff / rollback | 已支持 | 支持查看团队版本与个人副本差异，并从备份恢复。 |
| 团队 Skill 标签、集合、资料包 | 已支持 | Skills 页面提供标签过滤、集合卡片和研发评审/客户支持/团队工作流资料包；资料包只写入当前 Hub。 |
| 团队 Skill 版本与发布说明 | 已支持 | 团队 Skill 发布记录支持 `version`、`release_note`、发布状态和审查状态展示。 |
| 团队 MCP 共享 | 已支持 | 团队可发布 stdio/http MCP，支持审查、归档和 HTTP 健康检查。 |
| 团队 MCP 本地连接同步 | 已支持 | 团队页和 MCP Hub 提供“同步到 Codex / Claude Code”入口；本地刷新复用现有连接和安全写入流程，会拉取已发布团队 MCP 并更新客户端配置。 |
| 本机工具状态检测 | 已支持 | `GET /api/local/tools/status` 返回 Codex、Claude Code、Cursor、Gemini CLI 的安装、连接、同步模式、下一步动作和资源推荐预览。 |
| 资源推荐预览 | 已支持 | MCP Hub 展示 Skill / MCP / 私有资源索引建议；仅生成说明，不自动安装第三方 MCP server。 |
| 多端同步 | 已支持迁移和备份，不是实时同步 | 通过云端 Hub、Bundle Sync、GitHub Backup 和外部备份目标完成跨设备迁移、备份与恢复。 |
| 本地团队模拟 | 已支持演示 | 本地团队、成员、Skill 和审查记录只保存在当前设备。真实多人共享需要云端账号。 |

## 推荐连接顺序

1. Codex：作为默认推荐。原因是 `neu connect codex` 可以配置 Vola MCP、写入团队 MCP，并且 Vola Skill 默认写入 `~/.agents/skills`。
2. Claude Code：作为第二推荐。适合团队已经把 Claude Code 作为主力工具的场景。
3. Claude / ChatGPT Web：适合验证 profile、memory 和轻量资料读取。
4. Cursor / Gemini CLI：当前适合通过 MCP 连接和 Skill 导出包使用，不作为 Skill 自动写入的首选。

## 已确认的实现位置

| 领域 | 代码位置 |
| --- | --- |
| Skill 分配表 | `internal/api/skill_assignments.go` |
| 本地 Skill 同步 | `internal/api/local_skill_sync.go` |
| Skill 页面 | `web/src/pages/data/DataSkillsPage.tsx` |
| 团队 Skill 发布、订阅、更新提醒 | `internal/api/team_skill_assets.go` |
| 团队 Skill 安装到个人空间 | `internal/api/skill_copy.go` |
| 团队 Skill diff / rollback | `internal/api/skill_diff_rollback.go` |
| 团队 MCP 资产 | `internal/api/team_mcp.go` |
| 本地 MCP 客户端注册 | `internal/api/mcp_client_registry.go` |
| MCP 健康检查 | `internal/api/mcp_health.go` |
| 本机工具状态检测 | `internal/api/local_tool_status.go` |
| 本机平台连接刷新 | `internal/api/local_platform_connection.go` |
| Codex / Claude / Cursor 等平台连接 | `internal/platforms/platforms.go` |
| MCP Gateway 第三方 MCP 聚合 | `internal/mcp/gateway.go` |
| Bundle Sync | `internal/api/sync.go`、`internal/services/sync_service.go` |
| GitHub Backup | `internal/api/git_mirror.go`、`internal/localgitsync` |

## 还可以继续简化的地方

- 连接页已新增“先安装 neu 命令，再连接 Codex / Claude Code”的两步入口。桌面版可一键把 `neu` 安装到本机，避免用户直接复制 `neu connect codex` 后遇到 `command not found`。
- 在团队 MCP 区域增加“同步到本机”按钮，直接触发当前平台的连接更新，而不是要求用户知道需要重新执行 `neu connect codex` 或 `neu connect claude`。这一项现在已经完成。
- 在侧边栏保留“本机同步”常驻入口。MCP Hub 会展示 Codex、Claude Code、Cursor、Gemini CLI 的同步差异和下一步动作。
- 在首页和连接页把“Codex 推荐、Claude Code 其次”固定成默认排序，减少用户第一次选择成本。
- 在 Skill 更新通知里给成员提供一个直接按钮：查看差异、同步新版、需要时恢复旧版。
- 在 MCP Hub 里统一展示 Codex、Claude Code、Claude Desktop、Cursor、Gemini 的注册状态；当前 Web 一键注册主要覆盖 Claude Desktop，团队 MCP 刷新则由 Team Library / MCP Hub 提供入口。
- 把“本地团队模拟”和“真实团队云端同步”的差别放到团队页首屏，避免用户误解本地演示数据会自动同步给同事。

## 验证记录

本次核对基于代码阅读和后端测试。

已执行并通过：

```bash
go test ./internal/platforms ./internal/api ./internal/services ./internal/mcp
```

未执行：

- 没有在真实 Codex / Claude Code 客户端内逐项安装团队 MCP。
- 没有在多台设备上做云端账号同步验证。
- 没有对 Cursor / Gemini 的导出包做人工导入演练。
