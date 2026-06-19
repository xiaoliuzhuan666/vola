# Vola 小范围上线测试清单

更新时间：2026-05-14

本清单按当前版本的真实能力制定。测试目标是先验证个人版工作流，再让小团队以资料库方式试用；不把当前版本宣传成完整团队协作平台，也不直接开放公开 SaaS。

## 测试范围

### 本次纳入

- 个人账号登录、容量配置和基础资料管理。
- Profile、Memory、Projects、Conversations、Skills、Vault、Connections 的 Web UI、API、MCP 和 CLI 入口。
- Claude Code / Codex Skill 导入、分配、本地同步、转换预览和生成副本。
- Cursor / Gemini CLI Skill 分配、预览和导出包。
- GitHub Backup、WebDAV / S3-compatible 外部备份目标、备份历史、恢复预览和恢复应用。
- Team Library 第一阶段：创建团队、添加成员、按角色读写团队资料、团队 Skill 与 `/team/mcp`、`/team/prompts`、`/team/playbooks`。

### 本次不纳入

- 公开 SaaS 大规模注册。
- 企业级组织架构、审批流、审计报表、统一管理员后台。
- Cursor / Gemini CLI 自动修改本地配置。
- MCP server、plugin、hook 自动安装或自动启用。
- GitHub Backup 或 WebDAV / S3 zip 替代数据库备份。
- 多副本 hosted 部署下 Git mirror worker 并发调度。

## 上线前必须完成

1. 生产环境使用持久化 Postgres，`DATABASE_URL` 指向正式数据库。
2. `PUBLIC_BASE_URL` 使用最终 HTTPS 域名。
3. `JWT_SECRET` 和 `VAULT_MASTER_KEY` 保存到部署平台 secret 或受控密钥系统，恢复时继续使用原值。
4. 公开域名部署不能使用开发默认 `JWT_SECRET`、`VAULT_MASTER_KEY` 或 `vola_dev` 数据库密码；服务启动时会拒绝这类组合。
5. `GIT_MIRROR_HOSTED_ROOT` 指向持久目录，推荐 `/data/git-mirrors`。
6. 设置默认容量 `USER_STORAGE_QUOTA_BYTES`，配置 `INSTANCE_ADMIN_USER_IDS`，并准备实例管理员的 admin token。
7. 至少配置一个离开服务器的备份目标：GitHub Backup、WebDAV 或 S3-compatible。
8. 完成一次 Postgres dump，并在临时库做恢复演练。
9. 使用真实外部备份目标完成上传、历史记录、恢复预览和恢复应用验证。
10. 管理员调用 `/api/ops/status`，返回状态不能是 `critical`。
11. 准备测试账号、测试团队和测试资料，不使用生产用户隐私数据做演示。

## 个人账号验收

| 项目 | 通过标准 |
| --- | --- |
| 创建账号 | admin API 可以创建账号，账号能登录 Web UI |
| 容量 | 默认容量生效，单账号容量可以改成独立值或继承默认值 |
| Dashboard | Profile、Memory、Projects、Skills、Connections 等模块能打开 |
| 文件树 | 能创建、读取、编辑、删除测试资料，容量统计随写入变化 |
| Vault | 能看到 scope 列表，不能在导出包里泄露 secret 明文 |
| MCP | Agent 可以读取授权范围内的文件、Skill 和资料 |
| CLI | `neu status`、bundle export / preview / push / pull / diff 走通 |
| 同步历史 | 成功和失败任务都有记录，失败摘要不包含原始敏感内容 |

## Skill 验收

| 项目 | 通过标准 |
| --- | --- |
| 复杂 Skill 导入 | `SKILL.md`、scripts、依赖文件、assets、外部 Claude tools/plugins 引用进入 manifest |
| Claude Code 同步 | 只写 `~/.claude/skills` 下 Vola 管理的目录 |
| Codex 同步 | 只写 `~/.codex/skills` 下 Vola 管理的目录 |
| 冲突保护 | 同名但没有 `.vola-managed.json` 的目录不会被覆盖 |
| Cursor / Gemini CLI | 只出现导出项，不自动改本地配置 |
| Claude / Codex 转换 | 简单 Skill 可生成副本，复杂 Skill 的 MCP、plugin、hook 显示人工处理提示 |
| 真实运行 | 至少选 1 个简单 Skill 分别在 Claude Code 和 Codex 里试用一次 |

## Team Library 验收

Team Library 只按“小团队共享资料库”验收。

| 项目 | 通过标准 |
| --- | --- |
| 创建团队 | 当前账号成为 owner，团队有独立 hub user |
| 添加成员 | owner/admin 可按用户 slug 添加成员 |
| 角色 | owner/admin 可管理成员；member 可写团队资料；viewer 只读 |
| 多团队 | 同一个用户可加入多个团队，并能在页面选择团队 |
| 团队文件树 | `/team/mcp`、`/team/prompts`、`/team/playbooks` 可创建和读取 |
| 团队 Skill | 团队 `/skills` 与个人 `/skills` 分开 |
| MCP 访问 | Agent 传 `scope=team` 和 `team` / `team_id` 可读取团队资料 |
| 备份 | GitHub Backup 和外部 ZIP 包含团队资料与团队 Skill |

需要注意：这不是企业版团队空间。当前不验收组织级审批、审计报表、SSO 统一管理、跨团队权限矩阵和企业管理员后台。

## 备份与恢复验收

| 项目 | 通过标准 |
| --- | --- |
| Postgres dump | 生成 dump 文件并记录路径、时间、负责人 |
| 临时库恢复 | 临时环境能使用原 `JWT_SECRET` 和 `VAULT_MASTER_KEY` 启动 |
| 外部备份上传 | GitHub / WebDAV / S3-compatible 至少一个真实目标上传成功 |
| 备份历史 | 手动和自动备份能在页面或 API 中看到 |
| 恢复预览 | 上传 Vola export zip 后能识别 Skills、Memory、Projects、Vault 等分类 |
| 恢复应用 | 在临时环境验证跳过已有文件和覆盖已有文件 |
| 保留策略 | 对真实 provider 验证保留最近 N 份，不处理第三方文件 |
| 运维状态 | `/api/ops/status` 显示最近成功对象名，没有最近失败错误 |

## 发布判断

可以进入小范围测试的条件：

- `go test ./...` 通过。
- `cd web && npm run build` 通过。
- `make build` 通过，并刷新嵌入式前端产物。
- 生产同构环境能启动，`/api/health` 返回 ok。
- 个人账号主流程通过。
- Team Library 第一阶段主流程通过。
- 真实外部备份和临时环境恢复通过。
- `/api/ops/status` 不是 `critical`。

必须暂停的情况：

- 生产环境仍使用 SQLite 当主存储。
- 没有 Postgres dump 或没有恢复演练记录。
- 没有任何离开服务器的备份目标。
- `/api/ops/status` 为 `critical`。
- 登录、容量、文件树写入、Skill 导入或备份恢复中任一主流程失败。
