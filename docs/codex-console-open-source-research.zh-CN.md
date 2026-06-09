# Codex Console 与个人 Agent 数据层开源项目调研

更新日期：2026-06-06

研究对象：Vola 是否应该从“Agent 个人数据 Hub + MCP + 导入/同步”，继续扩展到更清晰的 Codex Desktop 控制台能力。

后续产品判断、完成度估算和推进记录见 `docs/codex-console-product-decision-log.zh-CN.md`。

## 资料边界

本报告使用三类资料：

- 用户提供的 X 帖摘录：主要观点包括把长期记忆放在本地文件系统、用 AGENTS.md 约束工作方式、把 Skills / Connectors 做成可复用流程、重视 Codex Desktop 的浏览器和电脑操作能力。
- Vola 当前仓库代码和文档：`README.zh-CN.md`、`docs/agent-data-hub-iteration-plan.zh-CN.md`、`docs/agent-skill-targets.zh-CN.md`、`internal/platforms/platforms.go`、`internal/platforms/codex_migration_test.go`、`internal/api/local_platform_codex_test.go`、`internal/storage/sqlite/client_agent_export.go`。
- 公开网页和 GitHub 项目页：包括 OpenAI Codex use cases、Chronicle 文档、Agentify Desktop、Awareness-Local、AI Workspace、cortex-hub、MCP Gateway / Context Forge / ToolHive / Gate22、Mission Control、Hermes Studio、OpenHands、Cline、Roo Code 等。

X 原帖链接 `https://x.com/jxnlco/status/2057153744630890620` 通过公开网页读取时没有返回正文。因此，本报告把用户贴出的摘录作为线索，不把无法读取的原帖正文当作证据。OpenAI 官方资料可以确认的方向包括：Chronicle、Goals、Computer Use、Skills、browser-based workflows、automation use cases 等。

## 判断

开源世界已经出现很多相近方向，但没有一个项目完整覆盖 Vola 应该做的事。

相近项目大致分为五类：

1. 本地记忆和共享上下文；
2. MCP 网关、工具注册、权限和审计；
3. 浏览器 / 桌面 / 网页会话控制；
4. Agent 工作台和编排 dashboard；
5. 多 Agent 配置、Skill、规则分发。

Vola 的机会不在“做另一个 OpenHands / Cline / Mission Control”，也不在“只做 MCP Gateway”。更清楚的方向是：

> Vola 是个人 Agent 数据层。Codex、Claude、ChatGPT、Cursor 等工具可以把 profile、memory、projects、conversations、skills、automations、artifacts 和执行记录接到同一个可备份、可迁移、可审计的位置。

在这个定位下，Codex Console 可以先从 Vola 里的 Codex 数据视图开始：读取、归档、关联、筛选、生成可复用记忆和交接资料。等数据模型和权限边界清楚后，再考虑更深的本地控制。

## Vola 当前已经有什么

Vola 已经不是空白接入。代码里能看到 Codex 相关能力已经覆盖了本地资料导入和 Skill 同步的主体路径。

### 已有 Codex 数据源识别

`internal/platforms/platforms.go` 的 Codex adapter 已经把这些路径作为可发现来源：

- `~/.codex/config.toml`
- `~/.codex/AGENTS.md`
- `~/.codex/rules`
- `~/.codex/memories`
- `~/.agents/skills`
- `~/.codex/skills`
- `~/.codex/automations`
- `~/.codex/auth.json`
- `~/.codex/sessions`
- `~/.codex/history.jsonl`
- `~/.codex/session_index.jsonl`
- `~/.codex/archived_sessions`

这说明 Vola 已经把 Codex 看成一个本地 Agent 平台，而不只是一个 MCP 客户端。

### 已有导入结果

`internal/platforms/codex_migration_test.go` 和 `internal/api/local_platform_codex_test.go` 里已经验证了：

- profile rules 可以导入；
- memory items 可以导入；
- projects 可以从 Codex session 的工作目录中归纳；
- Codex skills / bundled skills 可以进入 Vola skills；
- Codex sessions 可以归档为 conversations；
- automations 可以解析 schedule；
- plugin metadata 可以进入 tools；
- auth / token 相关内容会作为敏感项或 vault candidate 处理；
- 导入后会生成 `/platforms/codex/agent/automations.json`、`tools.json`、`connections.json`、`sensitive-findings.json`、`vault-candidates.json`。

`internal/storage/sqlite/client_agent_export.go` 也已经定义了 `AgentExportPayload`，字段覆盖 `profile_rules`、`memory_items`、`projects`、`automations`、`tools`、`connections`、`archives`、`sensitive_findings`、`vault_candidates`、`CodexInventory` 等。

### 仍然缺少的产品对象

当前 Vola 更像“把 Codex 本地资料导入 Vola 文件树”，还没有把 Codex Desktop 的运行过程做成一等对象：

- Thread：线程状态、工作区、分支、最近活动、关联目标；
- Goal：目标、预算、状态、完成证据、阻塞原因；
- Run：工具调用、命令、浏览器动作、电脑操作、审批、失败记录；
- Automation：不只是导入 TOML，而是可筛选、可审计、可关联线程；
- Artifact：截图、HTML、PDF、PPT、报告、生成文件；
- Chronicle Memory：`~/.codex/memories_extensions/chronicle` 下的本地 Markdown 记忆，目前 Vola adapter 还没有把它列为独立来源；
- Memory Candidate：从线程中提取的稳定事实，需要人工接受后进入 memory / projects / people / skills。

这也解释了用户反馈里的问题：Vola 已经能接 Codex，但用户进入 Vola 后还不容易一眼看懂“连接 Codex 后我能做什么”。

## 开源项目分组

### 1. 本地记忆与共享上下文

| 项目 | 方向 | 对 Vola 的启发 |
| --- | --- | --- |
| [Awareness-Local](https://github.com/edwin-hao-ai/Awareness-Local) | 本地优先 AI agent memory，Markdown、SQLite FTS5 / embedding、MCP、Web dashboard | Vola 的 memory 不应只做聊天摘要，应该能保留文件化、可查、可迁移的本地记忆。 |
| [AI Workspace](https://github.com/lee-to/ai-workspace) | 多项目共享文件、notes、目录，通过 MCP 给 Claude Code / Cursor / Windsurf 等使用 | 值得借鉴“显式共享目录”和“不要默认暴露敏感路径”的思路。 |
| [cortex-hub](https://github.com/lktiep/cortex-hub) | 自托管 Agent Memory + Code Intelligence，一个 MCP endpoint 给多个 Agent 使用 | 和 Vola 的“一处资料，多 Agent 读取”很接近，但 Vola 应保持更强的数据资产和备份叙事。 |
| [Dory](https://dory.deeflect.com/) | 本地 Markdown corpus，通过 MCP 给 Claude、Codex、Hermes 等使用 | 说明“Markdown + MCP + local-first”正在成为用户可理解的默认形态。 |
| [Origin](https://useorigin.app/) | Local-first memory for AI work，强调 decisions、sessions、lessons、project context | 说明用户真正需要的是跨工具延续项目经验，而不是单次聊天记忆。 |
| [mem0](https://github.com/mem0ai/mem0)、[Graphiti](https://github.com/getzep/graphiti)、[Zep](https://github.com/getzep/zep) | 记忆引擎、知识图谱、长期记忆 API | 适合借鉴记忆分层、实体关系、时间线变化；不适合作为 Vola 的直接产品对标。 |

这类项目会直接影响 Vola 的对外表达。用户会问：“我已经有 memory MCP 了，为什么还需要 Vola？”答案应该是：Vola 管的不只是记忆，还包括 skills、projects、conversations、vault、agent connections、备份和跨平台迁移。

### 2. MCP 网关、工具管理与权限审计

| 项目 | 方向 | 对 Vola 的启发 |
| --- | --- | --- |
| [MCP Gateway Registry](https://github.com/agentic-community/mcp-gateway-registry) | MCP server / agent / skills registry，强调 OAuth、动态发现和治理 | Vola 不应只把 MCP 当接入方式，也要把 Agent 能访问什么、何时访问、留下什么记录讲清楚。 |
| [IBM mcp-context-forge](https://github.com/IBM/mcp-context-forge) | MCP Gateway / registry / gateway 管理 | 企业侧会把 MCP 当基础设施，Vola 应避免只用“MCP Hub”做差异化。 |
| [ToolHive](https://github.com/stacklok/toolhive) | MCP server 运行、安全和管理 | 对 self-hosted 和本地安全边界有参考价值。 |
| [Gate22](https://github.com/aipotheosis-labs/gate22) | MCP 权限、网关、访问控制 | 说明“工具权限层”会越来越拥挤，Vola 应把个人数据所有权放在更前面。 |

这类项目的存在说明：MCP 管理会变成基础设施能力。Vola 可以做 MCP，但产品叙事不能停在 MCP。

### 3. 浏览器、桌面和网页会话控制

| 项目 | 方向 | 对 Vola 的启发 |
| --- | --- | --- |
| [Agentify Desktop](https://github.com/agentify-sh/desktop) | 让 Codex / Claude Code / OpenCode 通过 MCP 控制已登录的 ChatGPT、Claude、Gemini、Grok、Perplexity 等网页会话 | 这是最接近“Codex Desktop 深度控制台”的开源方向之一。Vola 可以借鉴本地会话、tab key、文件上传、artifact 保存，但不要直接变成网页会话控制器。 |
| [QuickDesk](https://github.com/barry-ran/QuickDesk) | AI computer use + MCP | 说明桌面控制的价值在“可观察、可回放、可授权”。 |
| [Windows-MCP](https://github.com/CursorTouch/Windows-MCP) | Windows 桌面控制 MCP | 对 Windows 端未来支持有参考意义，但和 Vola 当前 Mac / Web / local Hub 的主线不同。 |

这类项目解决“让 Agent 做动作”。Vola 更适合解决“动作之后留下什么证据、沉淀成什么记忆、哪些内容可以被下一个 Agent 继续使用”。

### 4. Agent 工作台和运行 dashboard

| 项目 | 方向 | 对 Vola 的启发 |
| --- | --- | --- |
| [Mission Control](https://github.com/builderz-labs/mission-control) | 自托管 Agent orchestration dashboard，tasks、agents、skills、logs、tokens、memory、security、cron、alerts | 证明 dashboard 会很快变拥挤。Vola 不宜照搬全功能控制台，应保留“个人数据层”的清晰边界。 |
| [Hermes Studio](https://github.com/JPeetz/Hermes-Studio) | Hermes Agent Web UI，chat、memory、skills、terminal、approvals、cron、多 Agent 编排 | 可借鉴 audit trail、approval timeline、cron 管理，但不应把 Vola 改成 Hermes 的专用 UI。 |
| [Accomplish](https://github.com/accomplish-ai/accomplish) | Agent 工作台、任务、审批、memory、multi-agent UI | 可借鉴任务与审批视图。 |
| [OpenHands](https://github.com/OpenHands/OpenHands)、[Cline](https://github.com/cline/cline)、[Roo Code](https://github.com/RooCodeInc/Roo-Code)、[Archon](https://github.com/coleam00/Archon) | 执行型 coding agent / IDE agent / agentic workflow | Vola 应服务这些 agent，而不是正面竞争执行能力。 |

这类项目的共同点是“让 Agent 跑任务”。Vola 应该在它们之下和旁边，保存可迁移数据、技能资产和可审计记录。

### 5. 多 Agent 配置、Skill 和规则分发

| 项目 | 方向 | 对 Vola 的启发 |
| --- | --- | --- |
| [gaal](https://getgaal.com/) | 一个 YAML 管 Claude Code、Cursor、Codex 等多种 Agent 的 MCP、skills、rules、slash commands | 这和 Vola 的多 Agent skill 分发非常接近。Vola 的优势应是 Hub、UI、备份、转换报告和权限，不只是写配置文件。 |
| [skill-depot](https://github.com/Ruhal-Doshi/skill-depot) | local-first memory / skill system，Markdown + MCP | 说明 Skills 作为本地可复用资产会越来越常见，Vola 的 manifest、转换和分配能力需要讲得更明确。 |
| [Arcweld](https://arcweld.ai/) | Shared workspace for AI tools，通过 MCP 给多工具共享文件和审计轨迹 | 与 Vola 的 projects / team library 有重叠，值得关注团队协作表达。 |

这一类项目会带来最直接的竞争感。Vola 需要把“同步配置”和“管理个人 Agent 数据资产”区分开：前者是文件写入，后者还包括导入、转换、备份、权限、审计和交接。

## Jason 这段内容对 Vola 的意义

用户给的摘录里，真正值得 Vola 吸收的是三条趋势。

### 1. 长期记忆应该由用户拥有

把 Obsidian vault、Markdown、AGENTS.md、projects、people、notes 放在本地，本质上是在反对“记忆锁在某个平台里”。这和 Vola 的主张一致：profile、memory、projects、skills 和 private data 要跟着人走，不跟某一个客户端走。

Vola 要继续强化这个点：

- 数据能导出；
- 数据能备份；
- 数据能被多个 Agent 读取；
- 数据变更有来源和时间；
- 敏感内容进入 vault 和 scoped token，而不是普通 memory。

### 2. Codex Desktop 的价值不只是写代码

OpenAI 公开 use cases 已经把 Follow a goal、Computer Use、Save workflows as skills、browser-based games、automation workflows 放在 Codex 场景里。Chronicle 文档也说明，它是 Codex app on macOS 的 opt-in research preview，用屏幕上下文生成本地 Markdown memories，并提醒用户注意 rate limit、prompt injection、权限和未加密本地记忆这些风险。

这对 Vola 意味着：只支持 Codex CLI 的 MCP 接入还不够。用户会希望 Vola 展示 Codex 线程、目标、自动化和执行证据，而不是只看到“已连接 Codex”。

### 3. Skills / Connectors 会变成工作方式的包装

用户不想每次重新教 Agent 做同一件事。Jason 的做法是把成功流程沉淀成 Skills / Connectors。Vola 已经有 Skill 导入、manifest、Claude / Codex 转换、分配和本地同步，这条线很对。

下一步需要补的是：从 Codex 线程中提取“这次确实有用的流程”，生成候选 Skill，并给出文件、脚本、依赖、适用 Agent、验证记录。

## 建议新增模块：Codex Console

Codex Console 的定位：

> Vola 中面向 Codex 用户的本地工作记录、目标、自动化、交付物和记忆候选视图。

它不需要替代 Codex Desktop，也不需要一开始就承担远程操控 Codex 的主界面职责。它负责把 Codex 做过的事整理成可迁移、可审计、可复用的 Vola 数据。

### 一等对象

| 对象 | 数据来源 | Vola 中的价值 |
| --- | --- | --- |
| Threads | `~/.codex/session_index.jsonl`、`~/.codex/sessions/**/*.jsonl`、Codex thread metadata | 让用户按项目、时间、状态查看 Codex 线程。 |
| Goals | Codex 线程里的 goal 状态、预算、完成/阻塞记录；目前需要从 session events 和 Vola 导入结果中抽取 | 把长任务从聊天里提出来，形成可检查的任务资产。 |
| Automations | `~/.codex/automations/**/automation.toml` | 展示定时任务、heartbeat、状态、prompt、最近归档结果。 |
| Runs | session JSONL 中的 tool call、command、browser/computer 操作、approval、error | 形成执行时间线和审计证据。 |
| Artifacts | 线程生成的文件、截图、HTML、PPT、PDF、报告 | 把交付物挂回项目、线程和 memory。 |
| Chronicle Memories | `~/.codex/memories_extensions/chronicle/**/*.md` | 让用户查看 Codex 从屏幕上下文沉淀出的本地记忆，并决定哪些进入 Vola memory。 |
| Memory Candidates | 线程里明确确认的长期事实、项目规则、用户偏好、流程经验 | 人工接受后写入 memory / projects / skills。 |

### 页面结构

建议先做四个视图：

1. **Overview**：Codex 连接状态、最近线程、活跃目标、自动化数量、最近 artifact。
2. **Threads**：按项目、cwd、日期、模型、状态筛选 Codex 线程，打开后展示摘要和关键事件。
3. **Automations**：展示 Codex automations 的 kind、schedule、status、prompt、来源路径，并关联产生的线程或交付物。
4. **Memory Review**：从线程中提取候选事实，用户选择写入 profile、memory、project、skill 或忽略。

这些视图可以先只读。只读版已经能形成很大价值，也比较安全。

### 数据路径建议

可以在 Vola 文件树里新增或规范以下路径：

```text
/platforms/codex/console/
  threads/index.json
  threads/<thread-id>/summary.md
  threads/<thread-id>/events.jsonl
  threads/<thread-id>/artifacts.json
  goals/index.json
  automations/index.json
  runs/<run-id>/timeline.jsonl
  memory-candidates/<date>-<thread-id>.md
```

已有 `/platforms/codex/agent/*.json` 可以继续保存原始 agent import 结果。`console/` 更偏产品化视图，适合页面消费。

## 分阶段路线

### 阶段 1：Codex Console Lite

目标：不新增远程控制，只做只读整理。

交付：

- Codex import 后生成 console index；
- Threads 页面展示 session summary、cwd、时间、关键 tool events；
- Automations 页面读取并展示 `.codex/automations`；
- Chronicle memories 如存在，作为只读来源进入 Memory Review；
- Memory Review 页面展示候选事实，人工确认后写入 Vola memory / projects；
- README / Setup / Dashboard 文案明确“连接 Codex 后能查看线程、自动化、记忆候选和技能资产”。

验收：

- 不读取或保存 `auth.json` 里的 token 明文；
- 不自动执行 Codex、浏览器或电脑操作；
- 导入结果可删除、可重新生成；
- 页面展示来源路径和导入时间。

### 阶段 2：执行时间线与 artifact 管理

目标：让 Codex 做过的事能被复查和复用。

交付：

- 解析 session JSONL 里的 tool_call / tool_result / command / file artifact；
- 把 HTML、截图、文档、报告挂到 thread 和 project；
- 支持按项目生成 handover summary；
- 支持把成功流程转为候选 Skill。

验收：

- timeline 能看出输入、动作、输出和失败；
- artifact 有文件路径、来源线程、生成时间；
- 候选 Skill 不直接安装，先进入 review。

### 阶段 3：受控操作入口

目标：在 Vola 中发起有限 Codex 操作，但仍以审计和权限为中心。

交付：

- 从 Vola 创建 Codex task draft；
- 选择 workspace、goal、skill、memory scope；
- 生成可复制到 Codex 的 prompt 或通过安全 adapter 发起；
- 操作结果回写到 console timeline。

验收：

- 默认只生成草稿，不直接控制桌面；
- 任何真实电脑操作、浏览器操作、外部账号操作都需要显式授权；
- 每次操作都有 scope、source、time、result。

## 文案建议

Dashboard 上 Codex 卡片不要只写“连接 Codex CLI”。建议改成更具体的用户收益：

> 连接 Codex 后，Vola 可以整理 Codex 的本地记忆、项目线程、自动化、Skills 和交付物。你可以把确认过的长期事实写入 Vola memory，把可复用流程沉淀成 Skill，并把 Codex 工作记录和项目交接资料一起备份。

Setup / README 里可以增加一段：

> Codex 接入分两层：MCP 连接让 Codex 读取 Vola 数据；Codex Console 让 Vola 读取和整理 Codex 本地工作记录。前者用于给 Codex 上下文，后者用于把 Codex 做过的事沉淀回个人数据 Hub。

## 不建议做的事

| 不建议 | 原因 |
| --- | --- |
| 把 Vola 做成 OpenHands / Cline 的替代品 | 执行型 Agent 工作台已经很拥挤，Vola 的优势是数据所有权、迁移、备份和权限。 |
| 直接宣传“控制 Codex Desktop” | 如果没有官方稳定 API 或用户授权流程，容易把本地文件导入和桌面控制混在一起。 |
| 只讲 MCP Gateway | MCP 网关类项目很多，差异化不足。 |
| 自动把线程总结写入长期 memory | 线程里有推测、失败和临时方案，需要人工确认。 |
| 自动启用 plugin、hook、MCP server | Vola 现有 Skill 转换已经把这些列为人工处理项，这个边界应保留。 |

## 推荐优先级

最高优先级：

- Codex Console Lite：线程、自动化、artifact、memory candidates；
- Dashboard / Setup 的 Codex 价值表达；
- 导入结果的来源、敏感项和删除能力；
- 从 Codex 线程生成项目交接摘要。

中优先级：

- 从成功线程生成候选 Skill；
- timeline / audit trail；
- artifacts 与 projects 的关联；
- team library 中共享 Codex playbook。

后续再看：

- 从 Vola 发起 Codex task；
- 手机端查看 Codex Console；
- 跨机器同步 Codex Console；
- 更深的桌面 / 浏览器控制。

## 资料链接

- OpenAI Codex use cases: https://developers.openai.com/codex/use-cases
- OpenAI Codex Chronicle: https://developers.openai.com/codex/memories/chronicle
- Awareness-Local: https://github.com/edwin-hao-ai/Awareness-Local
- AI Workspace: https://github.com/lee-to/ai-workspace
- cortex-hub: https://github.com/lktiep/cortex-hub
- Dory: https://dory.deeflect.com/
- Origin: https://useorigin.app/
- mem0: https://github.com/mem0ai/mem0
- Graphiti: https://github.com/getzep/graphiti
- Zep: https://github.com/getzep/zep
- MCP Gateway Registry: https://github.com/agentic-community/mcp-gateway-registry
- IBM mcp-context-forge: https://github.com/IBM/mcp-context-forge
- ToolHive: https://github.com/stacklok/toolhive
- Gate22: https://github.com/aipotheosis-labs/gate22
- Agentify Desktop: https://github.com/agentify-sh/desktop
- QuickDesk: https://github.com/barry-ran/QuickDesk
- Windows-MCP: https://github.com/CursorTouch/Windows-MCP
- Mission Control: https://github.com/builderz-labs/mission-control
- Hermes Studio: https://github.com/JPeetz/Hermes-Studio
- Accomplish: https://github.com/accomplish-ai/accomplish
- OpenHands: https://github.com/OpenHands/OpenHands
- Cline: https://github.com/cline/cline
- Roo Code: https://github.com/RooCodeInc/Roo-Code
- Archon: https://github.com/coleam00/Archon
- gaal: https://getgaal.com/
- skill-depot: https://github.com/Ruhal-Doshi/skill-depot
- Arcweld: https://arcweld.ai/
