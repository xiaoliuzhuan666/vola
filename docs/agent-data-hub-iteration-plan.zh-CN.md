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
- 阶段 6 主体完成：已补生产部署可靠性，包括 K8s Git mirror 持久卷、管理员运维状态接口、备份恢复手册和页面状态摘要。
- 阶段 7 主体完成：外部备份已有自动计划、运行历史、最近失败展示、保留策略、恢复预览和恢复应用入口；真实恢复演练和告警通道仍需继续。

## 整体完成度快照（2026-05-10）

面向内部演示或小范围 self-hosted 试用，当前约完成 85%。面向生产 SaaS，当前约完成 72% 左右。

已具备的部分：

- Agent 个人数据 Hub 的产品表达已经成立，覆盖 profile、memory、projects、skills、vault、connections、GitHub Backup 和外部备份目标。
- Skill 资产完整性已有 manifest，能识别脚本、依赖、资源、外部 Claude tools/plugins 引用。
- 多 Agent 分配、Claude Code / Codex 本地同步、Cursor / Gemini CLI 导出包、Claude / Codex 转换预览与生成副本已经可用。
- GitHub、WebDAV、S3-compatible / OSS / R2 备份目标已经进入页面和 API，外部 ZIP 备份具备自动计划、历史记录、保留策略和恢复应用入口。
- hosted 部署的 Git mirror PVC、部署脚本环境变量、`/api/ops/status`、恢复手册已经补齐。

主要短板：

- Cursor、Gemini CLI 暂不自动写入本地配置；当前采用分配表、目标探测、同步预览和导出包，避免误改用户全局配置。
- 转换后的 Skill 尚未在目标 Agent 运行时做自动验证；复杂 Skill 的 plugin、MCP、hooks 仍以报告形式提示，不自动改平台配置。
- 恢复能力仍未做真实演练；ZIP 恢复已经能应用到 Hub，但还需要在临时环境做完整恢复记录。
- 运维状态接口还不是完整 dashboard；已有备份历史、最近失败、保留策略状态，仍缺告警通道和恢复演练结果。
- Docker build 链路此前受 registry mirror 403 影响，生产镜像构建仍需要再次验证。

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

仍需观察：

- Cursor、Gemini CLI 仍未做自动写入；除非后续有可靠且不会误改项目配置的目录规范，否则继续保持导出包方案。
- 目标 Agent 的真实运行效果仍需人工试用确认；当前验证范围是文件结构、转换结果和本地同步安全规则。

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

仍需继续：

- 告警通道：备份失败或超过指定时间无成功备份时通知管理员。
- 真实恢复演练：从 Postgres dump + 外部 ZIP/GitHub 仓库恢复到临时环境并记录结果。
- WebDAV / S3-compatible 保留策略的远端删除已按历史对象名实现，但仍需要对坚果云、R2、OSS、MinIO 等真实服务做兼容性验证。
