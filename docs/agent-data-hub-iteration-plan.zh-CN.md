# Vola Agent 个人数据 Hub 迭代计划

## 背景

客户反馈集中在一个问题：如果 Vola 只被理解成 Skill 备份工具，GitHub、WebDAV、坚果云或 skillstash 这类方案会显得更直接。Vola 更适合承担 Agent 个人数据 Hub 的角色，统一管理 AI 工具可使用的 profile、memory、projects、conversations、skills、vault 和连接权限。

这个计划用于记录后续迭代方向。页面调整保持现有视觉风格，不引入新的设计系统，不改已有布局基调。

## 当前进度

- 阶段 1 已完成：产品表达已调整为 Agent 个人数据 Hub。
- 阶段 2 已完成：GitHub Backup 页面已增加数据位置、GitHub 远端仓库、WebDAV / S3-compatible 外部备份目标和恢复说明；OSS、R2 可通过 S3-compatible endpoint 接入。
- 阶段 3 已完成：Skill 导入会生成资产清单，能识别脚本、依赖、资源、外部 Claude tools/plugins 引用，并支持浏览器上传缺失的外部引用文件。
- 阶段 4 已完成主体能力：已加入 Agent 与 Skill 分配表，Skills 页可按 Agent 勾选 Skill；Claude Code / Codex 支持本地同步预览、应用和清理；Cursor / Gemini CLI 显示为“可分配、可导出、暂不自动写入”，并提供本地目标探测和导出包入口。
- 阶段 5 已完成：已加入 Claude Code / Codex 之间的 Skill 转换预览和生成副本能力，并输出可自动处理项、脚本、依赖、外部引用、MCP 配置、plugin/hook 风险报告；简单 Skill 可继续进入本地同步。
- 团队 Skill / Agent 共享已完成一段完整资产流程：团队 Skills 页能对比个人空间同路径 Skill，显示未安装、团队有新版、已安装、个人副本较新，并提供安装到个人或更新个人副本；团队 Skill 有发布状态、团队可见性和个人订阅记录；团队 Agent 可保存为 `/team/agents/<slug>/agent.vola.json`，管理员发布后成员可安装到个人 `/agents/<slug>/`。
- 阶段 6 主体完成：已补生产部署可靠性，包括 K8s Git mirror 持久卷、管理员运维状态接口、备份恢复手册和页面状态摘要。
- 阶段 7 主体完成：外部备份已有自动计划、运行历史、最近失败展示、保留策略、恢复预览和恢复应用入口；真实恢复演练和告警通道仍需继续。
- 阶段 8 进行中：Codex Console 已能展示 Codex Threads、Goals、Automations、Runs、Artifacts、Hooks、Memory 候选、Handover Summary 和候选 Skill 草稿，支持把选中的 Codex / Chronicle 记忆同步到 Vola profile memory 或 project context，并保存 Memory Review 状态；Hooks 已加入基础风险分析，profile memory conflict 已支持处理动作；Handover 已能保存为项目文件，候选 Skill 已能保存为 Hub 草稿并写入 manifest，也能分配给 Codex、Claude Code、Cursor、Gemini CLI 生成同步或导出预览。

## 整体完成度快照（2026-05-10）

面向内部演示或小范围 self-hosted 试用，当前约完成 85%。面向生产 SaaS，当前约完成 72% 左右。

已具备的部分：

- Agent 个人数据 Hub 的产品表达已经成立，覆盖 profile、memory、projects、skills、vault、connections、GitHub Backup 和外部备份目标。
- Skill 资产完整性已有 manifest，能识别脚本、依赖、资源、外部 Claude tools/plugins 引用。
- 多 Agent 分配、Claude Code / Codex 本地同步、Cursor / Gemini CLI 导出包、Claude / Codex 转换预览与生成副本已经可用。
- 团队 Skills 和团队 Agent 对象已有资料库入口。团队 Skill 可安装到个人空间后继续进入 Agent 分配、本地同步和导出流程；团队 Agent 发布后可安装为个人 Agent 配置对象，记录默认 Skill、目标工具、模型、权限和需要审批的动作。
- GitHub、WebDAV、S3-compatible / OSS / R2 备份目标已经进入页面和 API，外部 ZIP 备份具备自动计划、历史记录、保留策略和恢复应用入口。
- hosted 部署的 Git mirror PVC、部署脚本环境变量、`/api/ops/status`、恢复手册已经补齐。

主要短板：

- Cursor、Gemini CLI 暂不自动写入本地配置；当前采用分配表、目标探测、同步预览和导出包，避免误改用户全局配置。
- 转换后的 Skill 尚未在目标 Agent 运行时做自动验证；复杂 Skill 的 plugin、MCP、hooks 仍以报告形式提示，不自动改平台配置。
- 恢复能力仍未做真实演练；ZIP 恢复已经能应用到 Hub，但还需要在临时环境做完整恢复记录。
- 运维状态接口还不是完整 dashboard；已有备份历史、最近失败、保留策略状态，仍缺告警通道和恢复演练结果。
- Docker build 链路此前受 registry mirror 403 影响，生产镜像构建仍需要再次验证。
- Codex Console 仍缺 people 目标写入、hook 触发时机审查、候选 Skill 的真实本地同步验证和状态流转。
- 团队共享仍缺多人审批历史、团队管理员视角的全员订阅报表、后台自动检查更新和通知。团队 Agent 当前是可安装配置对象，不是 Vola 内置执行器。

## 阶段 1：产品定位改清楚

目标：

- 把产品表达从“Skill 备份/同步”调整为“Agent 个人数据 Hub”。
- 明确 Vola 管理的不只是 Skill，也包括 profile、memory、projects、conversations、vault、MCP/API 访问和权限。
- 在首页、README、onboarding、登录后入口页里保持同一套表达。

交付：

- README 中明确产品定位和能力边界。
- 公开首页首屏、能力模块、页脚文案统一到 Agent 个人数据 Hub。
- 登录后的连接入口页强调“把 Agent 接到同一份个人数据”。

验收：

- 用户不会把 Vola 只理解成 Skill 备份工具。
- 页面样式不变，只调整文案和信息表达。
- 前端构建通过，页面能正常打开。

## 阶段 2：外部备份变成一等功能

目标：

- 让用户知道数据主存储、备份目标和最近一次备份状态。
- GitHub Backup 保持现有能力，同时支持 WebDAV、S3-compatible、OSS、R2 等备份目标。
- 把“自己的服务器也可能挂”的担心转化成产品能力。

交付：

- GitHub Backup 页面增加存储状态说明、最近备份位置、恢复入口说明。
- 新增外部备份目标抽象：GitHub、WebDAV、S3-compatible。
- 部署文档增加 `GIT_MIRROR_HOSTED_ROOT`、Postgres 备份和恢复演练。

验收：

- 用户能看懂当前数据存在何处，备份到何处。
- 没有外部备份时，页面明确提示风险。
- GitHub 与 WebDAV / S3-compatible 至少一种远端备份可以从页面完成配置、同步、恢复说明。

## 阶段 3：Skill 资产完整性

目标：

- 复杂 Skill 不只保存 `SKILL.md`，还要保存脚本、依赖、资源文件和外部工具引用。
- 明确 `~/.claude/tools`、`~/.claude/plugins`、Skill 内 `scripts/` 等路径的处理规则。

交付：

- Skill manifest：记录入口文件、脚本、依赖文件、环境变量、支持平台。
- 导入时扫描 Skill 外部引用，提示哪些路径已保存，哪些路径需要用户确认。
- 对 Python、Node、shell 脚本增加依赖文件识别，例如 `requirements.txt`、`pyproject.toml`、`package.json`。

验收：

- `~/.claude/skills/<skill>/scripts/*.py` 能完整保存和导出。
- `SKILL.md` 引用 `~/.claude/tools/foo.py` 时，导入预览能提示并可选择纳入。
- 大文件、二进制、secret 风险有明确提示。

当前进展（2026-05-09）：

- 已增加 `manifest.vola.json`，Skill zip 导入后会在每个 Skill 目录生成资产清单。
- 资产清单会记录入口文件、脚本、依赖文件、资源、二进制文件、环境变量和支持平台线索。
- 上传结果页面已展示完整性检查，能看到脚本数量、依赖文件数量、外部引用数量，以及大文件、二进制、secret 风险提示。
- `SKILL.md` 等文本引用 `~/.claude/tools/...` 或 `~/.claude/plugins/...` 时，会记录为外部引用；浏览器上传 zip 时会提示当前 zip 未包含。
- 本地 Claude 迁移扫描会读取可访问且小于 256 KB 的外部 tools/plugins 文件，并保存到 Skill 下的 `external/claude-tools/` 或 `external/claude-plugins/`。
- Agent 本地导入和 Codex / Claude bundle 导入也会写入 `manifest.vola.json`。
- 浏览器上传 zip 后，如果 `SKILL.md` 引用了未包含的 `~/.claude/tools/...` 或 `~/.claude/plugins/...` 文件，页面可继续选择本地文件上传到 Skill 的 `external/` 目录，并刷新 `manifest.vola.json`。

## 阶段 4：多 Agent Skill 管理

目标：

- 支持一个 Skill Hub 分发到 Claude Code、Codex、Cursor、Gemini 等 Agent。
- 每个 Agent 可以选择启用不同 Skill。
- 同步前能看到差异。

交付：

- Skill assignment 页面：按 Agent 选择 Skill。
- 本地同步命令：预览、应用、清理未管理 Skill。
- 与现有 GitHub Backup 联动，保存版本历史。

验收：

- 用户能看到每个 Agent 当前有哪些 Skill。
- 用户能把一个 Skill 分配到多个 Agent。
- 同步前能看到新增、更新、缺失和冲突。

当前进展（2026-05-11）：

- 新增 `/api/skills/assignments`，分配表保存到 `/settings/agent-skill-assignments.json`，会进入 Hub 文件树和 Git 备份历史。
- Skills 页新增 Agent 分配面板，默认覆盖 Claude Code、Codex、Cursor、Gemini CLI。
- 页面可按 Agent 勾选 Skill，保存前展示新增/移除差异。
- Agent 分配卡片展示支持状态：Claude Code / Codex 是可自动同步；Cursor / Gemini CLI 是可分配、可导出、暂不自动写入。
- 新增 `docs/agent-skill-targets.zh-CN.md`，记录四类 Agent 的目录规则、导出包规则和安全边界。
- 新增 `/api/local/skills/sync/preview`：读取 Claude Code / Codex 本地 Skill 目录，展示新增、更新、本地多出、冲突、可清理项；Cursor / Gemini CLI 展示本地目标探测、目录规则和可导出项。
- 新增 `/api/local/skills/sync/apply`：按已保存分配表写入 Claude Code / Codex 目标目录，并写入 `.vola-managed.json` 标记。
- 新增 `/api/local/skills/sync/cleanup`：只删除带 `.vola-managed.json` 且已取消分配的 Skill 目录，不删除用户自己放在本地的普通 Skill。
- 新增 `/api/local/skills/sync/export`：按 Agent 生成导出包，包含完整 Skill 目录、scripts、依赖、assets、external Claude tools/plugins 和 manifest。
- Skills 页新增本地同步操作区，提供“预览同步”“应用到本地”“清理未管理 Skill”三个动作。
- 页面在 Cursor / Gemini CLI 的本地同步预览中展示不能自动写入的原因，并提供下载导出包按钮。
- Claude Code / Codex 本地应用遇到同名但没有 `.vola-managed.json` 的目录时会显示冲突，不会覆盖用户手工目录。

团队共享继续推进（2026-06-08）：

- 已根据团队 Skill 共享文章整理调研记录：`docs/team-skill-sharing-article-notes.zh-CN.md`。
- Vola 已有团队 Skill 列表、团队文件树、团队成员角色、团队 Skill 复制到个人空间、team scope 分配表、本地同步和导出。
- Skills 页在团队范围下会读取个人空间同路径 Skill，显示未安装、团队有新版、已安装、个人副本较新。
- 团队 Skill 卡片新增安装 / 更新动作：未安装时写入个人空间；团队版本较新时使用 overwrite 更新个人副本。
- 团队 Skill 新增发布记录：`/settings/team-skill-publications.json`，管理员可发布给团队、转草稿或归档；普通成员只看到 `published + team` 内容。
- 个人空间新增团队 Skill 订阅记录：`/settings/team-skill-subscriptions.json`，记录来源团队、源路径、目标路径、源文件指纹、安装时间和更新时间。页面加载时会重新检查团队源文件指纹，判断是否有团队新版。
- 团队 Agent 新增结构化对象：`/team/agents/<slug>/agent.vola.json`，记录默认 Skill、目标工具、模型、权限、需要人工审批的动作和维护人。管理员发布后，成员可以安装到个人 `/agents/<slug>/agent.vola.json`。
- 团队资料库模板保留 `/team/agents/README.md`，用于说明团队共享 Agent 的使用方式。
- 团队审查历史新增 `/settings/team-skill-review-history.json`，记录 Skill / Agent 的提交审查、通过、要求修改、发布和归档动作。
- 团队资料页新增管理员订阅报表，按成员展示团队 Skill 的未安装、已安装、可更新和来源缺失状态。
- 更新检查新增后台接口和团队通知文件 `/settings/team-skill-update-notifications.json`，管理员可手动触发检查并看到成员需要更新的 Skill。

仍需观察：

- Cursor、Gemini CLI 仍未做自动写入；除非后续有可靠且不会误改项目配置的目录规范，否则继续保持导出包方案。
- 目标 Agent 的真实运行效果仍需人工试用确认；当前验证范围是文件结构、转换结果和本地同步安全规则。
- 更新检查目前是接口和页面按钮，还没有接入服务启动后的常驻调度，也没有把通知写入个人收件箱。
- 团队 Agent 是可安装配置对象，不是 Vola 内置执行器。运行仍交给 Codex、Claude Code、Cursor、Gemini CLI 等工具。

## 阶段 5：Claude / Codex 转换

目标：

- 从“复制目录”升级到“平台适配”。
- 转换后给出可运行性检查结果。

交付：

- Claude Skill 到 Codex Skill 的转换规则。
- Codex Skill 到 Claude Skill 的转换规则。
- 转换报告：已转换、需要手工处理、不支持项。
- 针对脚本、依赖、hooks、plugins、MCP 配置输出风险说明。

验收：

- 简单 Skill 能直接转换并安装到目标 Agent。
- 复杂 Skill 不能直接使用时，报告能指出具体文件和原因。
- 不会静默丢弃脚本、依赖或权限要求。

当前进展（2026-05-11）：

- 新增 `/api/skills/convert/preview`，可预览 Claude Code → Codex、Codex → Claude Code 的转换结果。
- 新增 `/api/skills/convert/apply`，在 Hub 内生成转换后的 Skill 副本，例如 `/skills/release-helper-codex`。
- 转换会保留 `SKILL.md`、脚本、依赖、资源、外部引用文件，并重新生成 `manifest.vola.json`。
- Claude 外部工具引用如 `~/.claude/tools/helper.py` 会在转换到 Codex 时改写为 Skill 内相对路径 `external/claude-tools/helper.py`。
- 转换报告拆分为可自动处理、需要处理、暂不自动转换、提示四类；可自动处理项会说明 `SKILL.md`、文件树、脚本、依赖、assets、已纳入外部引用的处理结果。
- 转换报告会提示环境变量、脚本运行时、依赖安装、MCP 配置、hooks、Codex plugin 元数据等需要人工确认的项目。
- Skills 页新增 “Skill 转换” 面板，可选择源 Skill、源平台、目标平台、目标路径，支持预览和生成转换副本。
- 已补测试验证简单 Claude Code Skill 转为 Codex 后，可以分配给 Codex 并写入 Codex 本地 Skill 目录，目录内包含 `SKILL.md`、`manifest.vola.json` 和 `.vola-managed.json`。

仍需观察：

- 转换副本已经能进入第 4 阶段的 Agent 分配和本地同步，但还没有做目标 Agent 的实际运行自测。
- plugin 安装、MCP server 注册、hooks 启用仍只生成报告，不自动改本地平台配置。

## 阶段 6：部署可靠性

目标：

- 让 hosted / self-hosted 部署有清晰的数据安全路径。
- 避免用户误以为单台服务器就是可靠备份。

交付：

- K8s 配置增加 Git mirror volume 示例。
- Postgres 定时备份方案和恢复演练文档。
- 对象存储或 WebDAV 备份方案。
- 健康检查、后台任务、备份失败告警说明。

验收：

- 新部署能按文档完成备份目标配置。
- 能从备份恢复一份可用的用户数据。
- 页面和文档都明确区分主存储、缓存、备份、导出包。

当前进展（2026-05-10）：

- 新增管理员接口 `/api/ops/status`，汇总主存储、本地/hosted 模式、Git mirror、GitHub 远端仓库、WebDAV / S3-compatible 目标、最近成功备份和最近错误。
- GitHub Backup 页面在“数据位置与恢复”区域显示生产状态摘要，包括 Git 最近记录、外部目标数量和最近外部上传。
- `deploy/k8s/app.yaml` 增加 `vola-git-mirrors` PVC，并挂载到 `/data/git-mirrors`。
- `deploy/prod/deploy.sh` 会把 `GIT_MIRROR_HOSTED_ROOT` 写入 `vola-config`，未配置时默认 `/data/git-mirrors`。
- 新增 `docs/deployment-reliability.zh-CN.md`，记录数据分层、Postgres 备份、外部备份、状态接口、告警建议和恢复演练清单。

## 阶段 7：备份自动化与恢复入口

目标：

- WebDAV / S3-compatible 目标支持自动备份计划，减少依赖手工点击。
- 页面提供备份 ZIP 恢复入口，先能识别包内容和风险，再进入正式恢复。
- 运维检查能看见自动备份配置、最近执行时间和失败状态。
- 为后续真实恢复演练、保留策略、告警通道打基础。

交付：

- 备份目标增加 `auto_backup_enabled`、`auto_backup_interval_hours`、`last_auto_backup_at`。
- 备份目标增加 `retention_keep_last`、`retention_keep_days`，只按 Vola 生成的备份对象执行远端清理，不处理第三方对象。
- 新增备份运行历史，记录手动/自动触发、目标、对象名、大小、耗时、成功或失败信息。
- Scheduler 增加外部备份自动任务，按目标间隔调用现有导出 ZIP 上传能力。
- GitHub Backup 页面在外部备份目标里增加自动备份开关和间隔设置。
- 新增 `/api/backup/restore/preview`，支持上传 Vola 导出 zip 并返回识别分类、文件数量、字节数和风险提示。
- 新增 `/api/backup/restore/apply`，支持预览后按“跳过已有文件”或“覆盖已有文件”写回 Hub 文件树。
- 页面新增恢复预览和应用区，展示 Skills、Memory、Projects、Vault 等分类结果以及恢复结果。

验收：

- 新建 WebDAV / S3-compatible 目标时可以启用自动备份并设置间隔。
- 后台任务只处理到期且启用的目标，失败会记录到备份目标状态。
- 手动和自动备份都会写入运行历史，失败也能在历史和 `/api/ops/status` 中看到。
- 保留策略不会清理最近成功备份，也不会处理非 `vola-export-*.zip` 对象。
- 用户上传导出 ZIP 后，页面能识别是否为 Vola 备份包，并看到包含哪些数据。
- 当前阶段不会自动覆盖生产数据；应用恢复必须由用户点击触发，并选择跳过或覆盖策略。

当前进展（2026-05-10）：

- 已增加备份目标自动计划字段和 Postgres 迁移 `022_backup_automation.sql`，SQLite 启动时也会补齐字段。
- 已增加 Scheduler 的 `RunExternalBackups` 任务，默认每小时检查一次到期的外部备份目标。
- 已增加恢复预览 API 和页面入口，可读取 Vola 导出 zip 的 Skills、Memory、Projects、Vault、Roles、Inbox 等分类。
- 已增加 Postgres 迁移 `023_backup_runs_and_retention.sql`，记录备份运行历史和保留策略字段；SQLite 启动时也会补齐表和字段。
- 已增加 `/api/backup/runs`，可查询手动和自动备份运行历史。
- 已增加恢复应用 API 和页面入口，支持跳过已有文件、覆盖已有文件，并拒绝包含路径穿越的 ZIP；页面会展示恢复结果。
- GitHub Backup 页面已显示备份历史、最近失败、自动备份状态、保留策略、恢复预览和恢复应用入口。
- 已补 API 测试覆盖自动计划字段保存、自动任务成功/失败记录、恢复 ZIP 预览、恢复应用 skip/overwrite、路径穿越拒绝、手动备份历史成功/失败记录和 ops status 历史摘要。
- 多 Agent Skill 分配表、manifest、转换报告和本地同步导出包都属于 Hub 文件和导出内容的一部分，会随 GitHub Backup、外部 ZIP 备份和恢复入口一起进入备份边界。

验证结果（2026-05-10）：

- `env GOPROXY=https://goproxy.cn,direct GOCACHE=/private/tmp/vola-go-cache /usr/local/bin/go test ./...` 通过。
- `npm run build`（`web/`）通过；Vite 仍提示部分 chunk 超过 500 KB，这是体积提示，不是构建失败。
- `git diff --check` 通过。
- 未启动 dev server 做浏览器点击验证；本次以 API 测试和前端构建验证为准。

多 Agent Skill 验证结果（2026-05-11）：

- `env GOPROXY=https://goproxy.cn,direct GOCACHE=/private/tmp/vola-go-cache /usr/local/bin/go test ./...` 通过。
- `npm run build`（`web/`）通过；Vite 仍提示部分 chunk 超过 500 KB，这是体积提示，不是构建失败。
- `git diff --check` 通过。
- 已新增/更新测试覆盖 manifest 生成、Claude external tools/plugins 纳入、Claude/Codex 转换报告、分配表保存、Cursor/Gemini CLI 导出包预览、本地应用、未标记目录冲突保护、cleanup 标记规则和转换后 Codex 本地同步。
- 未启动 dev server 做浏览器点击验证；本次以 API 测试和前端构建验证为准。

## 阶段 8：Codex Console 与 Memory Sync

目标：

- 把 Codex Desktop 已经产生的本地资料整理成 Vola 的个人 Agent 数据层视图。
- 让用户能按 thread、goal、run、artifact、automation、memory candidate、hook risk asset 回看工作记录。
- Codex / Chronicle 记忆先进入候选区，用户确认后再写入 Vola memory。
- hooks、plugins、MCP server 继续只做报告和审查，不自动启用。

交付：

- 新增 Codex Console 页面和 `/api/local/codex-console`。
- 新增 `/api/local/codex-console/memory-sync`，把选中或全部 memory candidates 写入 `/memory/profile` 或 project context。
- 新增 `/api/local/codex-console/memory-review`，保存 accepted、ignored、deferred、synced 状态。
- 新增 `/api/local/codex-console/handovers/save`，把项目交接摘要保存为 `/projects/<project>/handover.md`。
- 后端响应包含 Threads、Goals、Automations、Runs、Artifacts、Hooks、Memory Candidates、Sensitive Findings、Vault Candidates。
- Artifacts 使用稳定 ID，页面只展示前 300 条，避免真实数据量过大时影响渲染。
- Hooks 从 Codex skill bundle 的 `hooks/` 文件识别，状态为 `manual_required`。
- Hooks 基础风险分析展示 shebang、环境变量、风险信号、写入路径提示和风险等级。
- Memory Review 状态保存到 `/platforms/codex/console/memory-review.json`，会进入 Hub 文件树和备份边界。
- Profile memory 已有不同来源内容时，Memory candidate 会显示 possible conflict。
- Profile memory conflict 支持保留已有、采用候选、两者共存和合并；普通 profile sync 遇冲突会跳过。
- Handover Summary 按 project 聚合 threads、runs、artifacts、memory candidates，并展示最近活动和关键条目；可保存为项目 handover 文件，重复保存会保留 `Manual notes`。
- Skill Candidates 从无错误且有工具调用的 Codex run 中生成候选草稿，可在页面编辑后保存为 Hub 草稿；已支持加入 Codex、Claude Code、Cursor、Gemini CLI 分配并生成同步或导出预览，当前仍不自动安装、不写入本地 Agent skills。
- 新增产品判断记录：`docs/codex-console-product-decision-log.zh-CN.md`。
- 新增开源项目调研记录：`docs/codex-console-open-source-research.zh-CN.md`。

当前进展（2026-06-08）：

- 本机桌面页当前显示：289 threads、69 goals、2 automations、260 runs、4344 artifacts、62 memory candidates、36 handovers、24 skill candidates、0 hooks。
- 真实本机没有发现 hook 风险资产；测试 fixture 已覆盖有 hook 文件时的响应。
- Memory Sync 已验证能把 Chronicle 候选写入 Vola profile memory。
- Memory Review 已支持接受、忽略、延后、重新待审；同步成功后会自动标记为 `synced`。
- Memory Sync 已支持 `target:"project"`，会追加到 `/projects/<project>/context.md`，重复候选会跳过。
- Profile possible conflict 已能在候选列表和详情里提示。
- Profile conflict resolve 已支持四种处理动作：保留已有、采用候选、两者共存、合并；处理结果会写入 Memory Review 状态。
- Hook Review 已显示基础详情：风险等级、shebang、环境变量、风险信号和写入路径提示；当前仍不执行、不安装、不改 Codex 配置。
- Handover Summary 已加入 Handovers tab，可按项目查看 thread、run、artifact 和 memory candidate 摘要；专业详情区可保存为 `/projects/<project>/handover.md`。
- Artifact Registry 已能把 Console 识别到的交付物保存为 `/platforms/codex/console/artifacts.json`，记录项目、线程、来源路径和保存时间；目前仍是索引资产，不提供预览或打开文件。
- Skill Candidates 已加入前端 tab 和详情页，可查看名称、项目、置信度、工具调用、交付物、信号、来源路径和 `SKILL.md` 候选草稿，支持编辑、复制和保存到 `/skills/_candidates/<slug>/`，写入 `SKILL.md`、`candidate.vola.json` 和 `manifest.vola.json`。
- 当前定位确认：Vola 是个人 Agent 数据层，Codex Console 是本地工作记录模块，不替代 Codex Desktop。
- 桌面端已从旧的 `src-tauri` 路径确认迁移到 `desktop/` 工程；Tauri 壳加载 `web/dist`，并启动 Go sidecar。
- 桌面端 sidecar 构建已改为生成 `desktop/sidecars/vola-<target-triple>`，避免写入仓库根目录 `bin/vola`。
- 桌面前端已通过 Tauri command 获取本机 sidecar API 根地址；返回值带 `/api`，桌面首页和 Codex Console 能正常请求本地 API。
- 桌面构建流程已补资源同步检查：release 构建先生成 `web/dist`，再复制到 `internal/web/dist` 并构建 Go sidecar，避免 sidecar 内嵌旧前端。
- 新增 `npm --prefix desktop run check:runtime -- --require-new`，用于检查是否仍有旧 `src-tauri` Vola 进程。旧进程存在时必须先退出再验证桌面端。
- Codex Console 默认页已从双栏看板改为单列阅读流：主内容展示可复制给下次 AI 的提示词、Vola 已整理好的内容、提示词建议和长期记忆价值；专业资料入口默认收起，避免普通用户先看到原始数量。
- Memory Sync 支持写入前编辑候选内容，前端通过记忆详情页编辑，后端通过 `content_overrides` 写入编辑后的文本。
- Profile memory conflict 支持手工编辑合并结果，前端通过“合并后写入内容”编辑框提交，后端通过 `merged_content` 写入最终 profile memory。
- Artifact registry 不再只是交付物列表；每条 artifact 会生成 `role`、`handoff_note` 和 `agent_instruction`，详情页提供“给下一个 Agent 的用法”和可复制提示词。
- Artifact tab 支持搜索、项目过滤和用途过滤；registry 增加 `project_summaries`，用于让下一个 Agent 先看项目级交付物摘要。
- Skill Candidate 详情页已把 `SKILL.md` 草稿从只读预览改为可编辑文本区。保存接口支持 `draft_override`，保存后 Console 会回读 Hub 里的编辑版 `SKILL.md`，metadata 和保存响应会标记 `edited`。
- Skill Candidate 已接入 Agent 分配链路。`/api/local/codex-console/skill-candidates/assign-preview` 会把已保存候选加入 Vola 的 Agent 分配表，并返回 Codex / Claude Code 本地同步预览或 Cursor / Gemini CLI 导出预览；该接口不会写入本地 Agent 目录。

仍需继续：

- Memory 目标扩展：people；Handover 已支持页面编辑并依赖 FileTree 版本记录，后续还需要版本历史查看入口；候选 Skill 草稿已支持页面编辑和多 Agent 分配预览，后续还需要真实本地同步验证和 ready / archived 状态。
- 记忆编辑：普通候选写入前编辑和 profile conflict 最终合并文本编辑已完成；people 目标仍未实现。
- Hook Review 深化：触发时机、结构化命令、读取路径和执行日志。
- 候选 Skill 的真实本地同步验证和状态流转。

验证结果（2026-06-07）：

- `go test ./internal/platforms ./internal/api` 通过。
- `npm run build`（`web/`）通过；Vite 仍提示部分 chunk 超过 500 KB。
- `git diff --check` 通过。
- `http://127.0.0.1:3001/codex-console?local_token=...` 返回 200。
- `/api/local/codex-console` 使用 Bearer token 返回 200。
- `/api/local/codex-console/memory-review` 已验证路由和候选 ID 校验；测试覆盖 ignored 状态持久化和 sync 后自动标记 `synced`。
- `/api/local/codex-console/memory-sync` 已补测试覆盖 project target 写入和重复同步跳过。
- `GET /api/local/codex-console` 已补测试覆盖 profile possible conflict 提示。
- `GET /api/local/codex-console` 已补测试覆盖 Hook Review 基础详情，包括风险等级、shebang、环境变量、风险信号和写入路径提示。
- `/api/local/codex-console/memory-conflict/resolve` 已补测试覆盖保留已有、采用候选、两者共存和合并；普通 profile sync 遇冲突会跳过。
- `GET /api/local/codex-console` 已补测试覆盖 Handover Summary，fixture 中的 `vola` 项目会生成 thread/run 摘要。
- `/api/local/codex-console/handovers/save` 已补测试覆盖保存到 `/projects/vola/handover.md`、重复保存保留 `Manual notes`、Console 回读 saved 状态和路径。
- `/api/local/codex-console/artifacts/save` 已补测试覆盖保存到 `/platforms/codex/console/artifacts.json`、registry 文件内容、artifact 项目的 thread/source 关联，以及 Console 回读 saved 状态和路径。
- Artifact 路径识别已兼容 Codex JSONL 工具输出里的转义换行，测试 fixture 覆盖 `docs/import-plan.md` 这种相对 Markdown 交付物。
- `GET /api/local/codex-console` 已补测试覆盖 Skill Candidates，fixture 中无错误且有工具调用的 run 会生成候选 Skill 草稿。
- `npm run build`（`web/`）已验证 Skill Candidates 前端类型和页面组件。
- Browser 打开 `http://127.0.0.1:3001/codex-console?local_token=...`，Skill Candidates tab 显示 23 条候选；详情区显示 `SKILL.md` 草稿、复制草稿和复制来源路径按钮；Console 0 errors。
- `cd desktop && npm run build` 通过，生成 `desktop/target/release/bundle/macos/Vola.app` 和 `desktop/target/release/bundle/dmg/Vola_0.1.0_aarch64.dmg`。
- 使用新构建的 macOS 桌面包验证：首页没有 `The string did not match the expected pattern.` 黄条，统计和文件树显示正常。
- 使用新构建的 macOS 桌面包验证：`Codex Console` 页面显示 285 threads、69 goals、259 runs、4515 artifacts、62 memory candidates、36 handovers、23 skill candidates。
- 本次新增 Skill Candidate 保存功能后，`./node_modules/.bin/tauri build --bundles app` 通过，生成 `desktop/target/release/bundle/macos/Vola.app`。
- Computer Use 打开新 `desktop/target/release/bundle/macos/Vola.app` 后，Codex Console 显示 289 threads、69 goals、260 runs、4344 artifacts、62 memory candidates、36 handovers、24 skill candidates，并在 Skill 草稿详情中显示“保存为 Vola Skill 草稿”按钮。
- 为避免向真实 Hub 写入随机测试草稿，本次桌面 UI 只验证按钮和状态可见；实际写入由 `TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave` 覆盖。
- 用户反馈纯数量看板和全量记录列表不够有意义后，Codex Console 默认视图已继续改为 `AI 使用改进台`：优先展示提示词改进建议、长期记忆能提升什么准确度、下一个 Agent 应该先看什么；原始记录仍在后续 tab。
- 用户侧桌面截图发现新版 memory 三列卡片在真实数据下会被长路径挤乱；已改为单条记忆价值摘要、提示词建议纵向列表和紧凑专业入口。
- 提示词建议已增加数据依据：会根据最近 Codex 记录中的桌面端、Tauri、Vola.app、验证、build、screenshot、失败 run、交付物和记忆候选信号展示对应建议，减少静态模板感。
- 桌面 runtime 检查脚本已改为精确匹配新桌面 app 和 sidecar executable，避免误把 Codex Computer Use 的历史上下文当成 Vola 进程。
- Codex Console 摘要和预览已过滤 Unicode replacement character，桌面页面不再直接展示 `�` 乱码。
- 用户再次指出默认页仍有太多无效数据后，首页继续降级所有冷数据：工作区统计只在专业视图显示；提示词建议不再带操作按钮；长期记忆只展示高价值候选、可读摘要和“为什么有用”；右侧专业入口用状态词替代裸数字。
- 本次 `npm --prefix web run build` 已通过。首次失败是因为沙箱不能写 Vola `web` 目录下的 Vite 临时文件，外部权限重跑后通过。
- 默认页的长期记忆摘要已继续改为价值说明：例如“这条记录描述了某项目背景和处理范围”“这条记录来自 Codex 记忆扩展，确认后可合并进 Vola 记忆”，避免普通用户看到原始路径和扩展正文。
- 桌面验证时发现 Computer Use 用应用名 `Vola` 会拉起旧 `/src-tauri/.../vola.app`；后续桌面读取必须使用新 `.app` 完整路径。
- `desktop/scripts/check-desktop-runtime.mjs` 已能拦住旧 `src-tauri` app 的 `Contents/MacOS/app` 和旧 sidecar。
- 新 `.app` 验证路径：`/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`；bundle id `cn.vola.desktop`；Codex Console 页面 URL `tauri://localhost/codex-console`。
- 新桌面包首页没有 `The string did not match the expected pattern.`；Codex Console 显示新版 AI 使用改进台。
- Handover 保存功能新增后，`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'` 和 `GOCACHE=/private/tmp/vola-go-cache go test ./internal/platforms ./internal/api` 通过。
- `npm --prefix web run build` 通过；Vite chunk size warning 仍存在。
- `./node_modules/.bin/tauri build --bundles app` 在 `desktop/` 下通过，生成 `desktop/target/release/bundle/macos/Vola.app`。
- `npm --prefix desktop run check:runtime -- --require-new` 通过，确认当前只运行新 `desktop` app，没有旧 `src-tauri` app。
- Computer Use 打开新 `desktop/target/release/bundle/macos/Vola.app` 后，首页没有 `The string did not match the expected pattern.`，Codex Console 正常加载，项目交接详情显示“保存为项目交接文件”按钮。
- 默认页布局再次调整后，桌面新包读取到 `tauri://localhost/codex-console`，专业资料区位于主内容下方，不再挤压提示词建议和长期记忆摘要。
- `TestSQLiteSharedServerLocalCodexConsoleMemorySyncEditedContent` 已覆盖编辑后的记忆内容写入 Vola profile memory。
- `TestSQLiteSharedServerLocalCodexConsoleMemoryConflictResolveEditedMerge` 已覆盖编辑后的冲突合并内容写入 Vola profile memory。
- `TestSQLiteSharedServerLocalCodexConsoleArtifactRegistrySave` 已覆盖 artifact 交接字段写入 registry。
- `TestSQLiteSharedServerLocalCodexConsoleArtifactRegistrySave` 已覆盖 artifact 项目摘要、role 统计和优先交付物写入 registry。
- 本次新增验证：`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'`、`GOCACHE=/private/tmp/vola-go-cache go test ./internal/platforms ./internal/api`、`npm --prefix web run build`、`./node_modules/.bin/tauri build --bundles app`、`npm --prefix desktop run check:runtime -- --require-new`、`git diff --check`。
- 桌面验证复盘补充：Computer Use 使用应用名 `Vola` 会拉起旧 `/src-tauri/.../vola.app`；本次已再次复现并关闭旧窗口。后续只能使用新 `.app` 完整路径读取桌面端。
- 2026-06-08 桌面端再次验证：默认页显示“Vola 已经替你整理好的内容”，包含项目接手材料、交付物用途说明、长期记忆候选、可复用流程草稿、自动化和 Hook 风险；专业资料和编辑入口默认折叠。Skill 草稿专业页仍显示 `Agent 分配预览` 区域，未保存草稿时按钮禁用，不会写入 `~/.codex/skills`。

长期仍需继续：

- 告警通道：备份失败或超过指定时间无成功备份时通知管理员。
- 真实恢复演练：从 Postgres dump + 外部 ZIP/GitHub 仓库恢复到临时环境并记录结果。
- WebDAV / S3-compatible 保留策略的远端删除已按历史对象名实现，但仍需要对坚果云、R2、OSS、MinIO 等真实服务做兼容性验证。

2026-06-08 Codex Console 默认页调整：

- 默认页改成单列阅读流，移除右侧专业详情栏，避免提示词建议和长期记忆摘要被挤压。
- 首页主任务改为“让后续 AI 更准”：先给可复制提示词，再给提示词改进建议、长期记忆价值和接手材料。
- 专业入口保留在底部，显示状态词，不再让普通用户先看数量。
- 桌面端已用新 `.app` 完整路径验证：`tauri://localhost/codex-console` 正常打开，没有旧的 pattern 错误，也没有卡片重叠。
- 本机验证注意事项：arm64 Node + Rollup 原生包会触发 macOS 签名限制；本次前端 build 用 x64 Node，Tauri build 用 arm64 Node，并通过 `--config '{"build":{"beforeBuildCommand":""}}'` 避免重复执行前端 build。

2026-06-08 项目交接编辑：

- Handover 详情页新增 Markdown 编辑框。用户可以把自动交接摘要改成真正给下一个 Agent 用的说明，再保存到项目资料。
- 保存接口支持 `content_override`，后端写入编辑后的内容，并返回 FileTree `version`。
- Console GET 会回读已保存的 `handover.md` 内容，下一次打开详情页可以继续从已保存正文编辑。
- 后端测试覆盖编辑内容保存、marker 自动保留、版本号递增和 Console 回读。

2026-06-08 Skill Candidate 多 Agent 分配预览：

- Skill Candidate 专业详情区新增 Agent 目标选择：Codex、Claude Code、Cursor、Gemini CLI。
- 默认仍选 Codex，专业用户可以勾选多个目标；按钮改为“加入所选 Agent 分配并预览”。
- 预览结果按 Agent 分组展示。Codex / Claude Code 显示本地同步计划；Cursor / Gemini CLI 显示导出包预览和不自动写入说明。
- 后端测试扩展为四个 Agent 同时分配，断言写入 `/settings/agent-skill-assignments.json`，并确认预览不会写入 Codex / Claude Code 临时本地目录。
- 已验证：TypeScript 检查、Vite build、`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave$'`、桌面 `.app` 中多 Agent 选择器显示和 Claude Code 勾选。
- 未执行：真实候选的分配按钮点击，避免向当前本机 Hub 写入测试分配。
