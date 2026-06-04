# Vola 可配置模型提供方 + 每日学习任务 + 成长提案系统调研

更新日期：2026-05-24

## 这件事该怎么定义

桌面版现在已经不适合继续被包装成“本地资料浏览器”。更准确的产品目标是：

> 一个会持续整理 Agent 知识、发现可沉淀经验、推荐相关 Skill、提醒用户把经验变成能力资产的个人 AI 工具箱。

这里的“成长”不是让模型参数自己训练，也不是每天自动乱改用户的 Skill。更可靠的定义是：

1. 每天扫描：知道本地 Skill、memory、project、conversation 发生了什么变化。
2. 结构化理解：把变化整理成 skill map、能力地图、问题清单。
3. 模型分析：在用户授权的模型提供方上生成学习报告和改进建议。
4. 成长提案：把“建议改什么”做成可审查、可应用、可回滚的 proposal。
5. 用户确认：默认只写报告和提案，不直接改主 Skill。

所以需要模型能力，但不应该内置一个固定模型服务。正确架构是：**Learning Engine + 可配置 Model Provider + Proposal Review**。

## 外部项目现状

### 1. Letta / MemGPT：长期记忆不是聊天记录堆叠

Letta 把 agent 做成有状态实体，记忆、消息、推理、工具调用都会持久化；核心记忆会进入上下文，agent 也能通过工具修改自己的 memory blocks。

可借鉴点：

- 记忆要分层：profile、project、skill、daily learning、proposal 不应该混在一个目录。
- 记忆要可编辑：模型只能提出修改建议，真正写入要经过受控 API。
- 共享记忆要谨慎：团队级记忆、个人级记忆、Agent 级配置要有作用域。

对 Vola 的启发：

- `/memory/learning/...` 是对的，但还不够，需要补 `/memory/proposals/...` 和 `/memory/capability-map/...`。
- Skill 自己不应该直接承载所有学习结果，否则会越改越乱。

### 2. Mem0：生产化记忆引擎的关键是作用域、检索和评估

Mem0 OSS 的重点是自托管、可替换 LLM / embedding / vector store / reranker，并且强调用户掌控数据和 provider。它默认支持本地开发、自定义组件、服务模式和库模式。

可借鉴点：

- 模型、embedding、vector store 都要可配置。
- 记忆写入和记忆检索要分开。
- 记忆不是只有“新增”，还要支持更新、冲突处理、评估。

对 Vola 的启发：

- Model Provider 设置页不要只存一个 API Key，要有用途分类：summary model、proposal model、embedding model。
- 每日学习任务不一定都调用大模型：扫描和打分可本地完成，只有摘要和 proposal 需要模型。
- 未来如果引入向量检索，可以从 SQLite FTS 起步，不急着强依赖 Qdrant / pgvector。

### 3. Graphiti / Zep：适合动态知识，不适合第一阶段直接全量引入

Graphiti 的价值是 temporal knowledge graph：实体、关系、时间有效性、episode provenance。它很适合处理会变化的知识，例如“某个 Skill 原来支持 Claude，后来也支持 Codex”“某条项目经验来自哪次会话”。

可借鉴点：

- 事实变更不要覆盖旧事实，要保留时间和来源。
- 每个派生结论都要能回到原始 episode。
- 动态知识适合增量构建，不适合每天全量重算。

对 Vola 的启发：

- 第一阶段不建议直接上图数据库，成本太高。
- 可以先实现“轻量 provenance”：每条 learning note / proposal 记录 input_paths、source_entries、generated_by、model、prompt_version。
- 后续再考虑把 skill、project、agent、proposal 的关系导成图。

### 4. LangGraph：学习任务必须可恢复、可回放

LangGraph 的 persistence 提供 thread、checkpoint、state history、replay、update_state 等能力。它说明长任务不应该只留最后结果，而是要能看到每一步。

可借鉴点：

- 学习任务要有 run id。
- 每一步要记录状态：scan、summarize、propose、validate、await_review、applied。
- 失败后可以从上一步继续，不要重头跑。

对 Vola 的启发：

- 新增 `learning_runs` 概念，先存在文件树或 SQLite。
- 每日任务输出不只是 markdown，还要有 machine-readable JSON。
- UI 上要能看到最近一次任务成功/失败，以及失败在哪一步。

### 5. Aider：先做 map，再让模型理解

Aider 的 repo map 思路很适合 Vola。它不是把整个仓库塞给模型，而是先压缩成结构化索引。

可借鉴点：

- 先做 Skill Map：入口、描述、when_to_use、manifest、脚本、依赖、分配、更新时间、验证状态。
- 模型看 map，不直接看全量文件。
- 只有生成具体 proposal 时，再读取相关原文。

对 Vola 的启发：

- 每日学习任务的第一步应该是 `BuildSkillMap`。
- 这个 map 可以同时服务推荐、日报、能力地图和后续搜索。

### 6. OpenHands / Voyager / Reflexion：成长来自“可复用技能”和“失败反思”

OpenHands 把 skill 作为可复用 markdown 资产。Voyager 的长期学习靠不断扩张 skill library。Reflexion 强调把失败、反馈、自我批评转成文本经验。

可借鉴点：

- Skill 必须是资产，不只是文档。
- 失败记录比成功记录更有学习价值。
- 新 Skill 不能直接入主库，要先作为候选，经过验证后再启用。

对 Vola 的启发：

- 成长提案类型至少要有四种：
  - `improve_skill`：补 description、when_to_use、验证步骤。
  - `split_skill`：一个 Skill 过大，拆成多个候选。
  - `new_skill`：从项目经验或会话中提炼新 Skill。
  - `archive_or_review`：长期未使用、信息不足或风险较高的 Skill。
- 提案默认写入 `/memory/proposals/skills/YYYY-MM-DD/*.json` 和对应 markdown。

### 7. OpenEvolve / EvoAgentX：可以借流程，不要照搬野心

这些项目强调生成候选方案、评估、再优化。它们适合算法和工作流搜索，但对桌面知识工具来说，风险是复杂度过高。

可借鉴点：

- 所有自动改进都要有 evaluation gate。
- 可以一次生成多个候选 proposal，让用户选。
- 没有验证的 proposal 不能直接应用。

对 Vola 的启发：

- 第一阶段不要做自动改写。
- 第二阶段可以做“半自动应用”：只允许修改 frontmatter、补充建议段落、创建候选文件。
- 第三阶段再引入验证任务，例如脚本存在性、依赖可用性、本地同步预览通过。

### 8. LiteLLM / Dify / Open WebUI：模型提供方要独立于业务逻辑

LiteLLM 的价值是把不同 provider 统一成类似 OpenAI 的调用接口，并支持路由、fallback、成本统计、budget。Dify 和 Open WebUI 的价值是 provider 配置产品化：用户可以选择 OpenAI、Anthropic、Gemini、Ollama、OpenAI-compatible endpoint。

可借鉴点：

- Provider 配置应该是独立模块，不嵌在学习任务里。
- 要支持 OpenAI-compatible endpoint，因为本地模型、LiteLLM proxy、企业网关都可以走这个入口。
- 要记录用量、错误、最后一次验证时间。

对 Vola 的启发：

- 第一阶段支持四类 provider 足够：
  - OpenAI-compatible：`base_url`、`api_key`、`model`。
  - OpenAI：作为预置模板，本质也可走 OpenAI-compatible。
  - Anthropic：单独 adapter。
  - Gemini：单独 adapter。
  - Ollama：本地默认候选，走 `http://localhost:11434`。
- 配置要有 `test connection`，不能让用户填完后等到夜里任务才失败。

## 对 Vola 的差异化判断

竞品很多，差异不要做成“我们也有一个 Agent”。更好的差异是：

### 1. 不做复杂 Agent 控制台，做知识资产管家

用户真正想要的是：

- 我现在有哪些可用 Skill？
- 新需求来了应该用哪个？
- 哪些经验值得沉淀成 Skill？
- 哪些 Skill 质量差、不该同步给 Agent？
- 哪些修改是 AI 建议的，我能不能看懂并确认？

这比“自动执行一堆任务”更贴近桌面版价值。

### 2. 本地优先，但模型可换

本地扫描、本地记录、本地审批是基础；模型只是增强。

这样能覆盖三类用户：

- 不配模型：也能看到 Skill 健康度和推荐。
- 自带云模型 Key：得到高质量总结和提案。
- 本地模型：保护隐私，但质量可接受时再启用。

### 3. 提案优先，不默认自动改

这是和很多“自进化”工具拉开距离的地方。Vola 应该强调：

- AI 可以建议。
- 用户能审查。
- 应用有边界。
- 所有变更有来源和回滚点。

### 4. 和 Skill 同步链路打通

Vola 已经有 Claude Code / Codex 同步和 Cursor / Gemini CLI 导出边界。成长系统要服务这条链路：

- 未验证 Skill 不建议同步。
- 有外部引用的 Skill 标记风险。
- 没有 when_to_use 的 Skill 不进入高优先推荐。
- 新需求匹配不到 Skill 时，生成候选 Skill 提案。

## 建议的数据结构

### Model Provider

建议路径：

- `/settings/model-providers.json`

建议结构：

```json
{
  "version": "vola.model-providers/v1",
  "default_summary_provider_id": "openai-main",
  "default_proposal_provider_id": "openai-main",
  "providers": [
    {
      "id": "openai-main",
      "type": "openai-compatible",
      "name": "OpenAI",
      "base_url": "https://api.openai.com/v1",
      "api_key_ref": "vault://model/openai-main",
      "models": {
        "summary": "<user-configured-summary-model>",
        "proposal": "<user-configured-proposal-model>"
      },
      "enabled": true,
      "last_verified_at": null
    }
  ]
}
```

API Key 不写进文件树明文，放 Vault。

模型名不在产品里写死。不同供应商的模型名、端点和参数会变，配置页只保存用户选择或输入的值，并通过 `POST /api/model-providers/test` 做连接验证。

### Learning Run

建议路径：

- `/memory/learning/runs/YYYY-MM-DD/run.json`
- `/memory/learning/runs/YYYY-MM-DD/report.md`

建议字段：

```json
{
  "id": "2026-05-24-skill-learning",
  "status": "completed",
  "started_at": "2026-05-24T02:00:00Z",
  "finished_at": "2026-05-24T02:01:30Z",
  "steps": [
    {"name": "scan", "status": "completed"},
    {"name": "summarize", "status": "completed"},
    {"name": "propose", "status": "completed"}
  ],
  "input_paths": ["/skills", "/memory/profile", "/projects"],
  "model": {
    "provider_id": "openai-main",
    "model": "<user-configured-summary-model>"
  },
  "outputs": {
    "report_path": "/memory/learning/runs/2026-05-24/report.md",
    "proposal_dir": "/memory/proposals/skills/2026-05-24"
  }
}
```

### Growth Proposal

建议路径：

- `/memory/proposals/skills/YYYY-MM-DD/<proposal-id>.json`
- `/memory/proposals/skills/YYYY-MM-DD/<proposal-id>.md`

建议字段：

```json
{
  "id": "proposal-skill-docker-001",
  "type": "improve_skill",
  "status": "pending_review",
  "target_path": "/skills/docker-delivery/SKILL.md",
  "risk": "low",
  "reason": "Skill has scripts and dependencies but lacks verification steps.",
  "suggested_changes": [
    {
      "kind": "append_section",
      "heading": "Verification",
      "content": "Run docker compose config and a dry-run build before applying."
    }
  ],
  "source_paths": [
    "/skills/docker-delivery/SKILL.md",
    "/skills/docker-delivery/manifest.vola.json"
  ],
  "created_by": {
    "kind": "learning_engine",
    "model_provider_id": "openai-main",
    "prompt_version": "skill-growth-proposal-v1"
  }
}
```

## 产品页面建议

### 设置页：模型提供方

用户需要看到：

- Provider 类型：OpenAI-compatible / Anthropic / Gemini / Ollama。
- Base URL。
- Model for summary。
- Model for proposal。
- API Key 状态：已配置 / 未配置，不展示明文。
- 测试连接按钮。
- 最近一次调用错误。

### Skills 页：学习摘要

当前已经有基础 UI，下一步补：

- 最近一次学习任务状态。
- 今日报告入口。
- 待审成长提案数量。
- “生成学习报告”手动按钮。
- “只扫描不调用模型”模式提示。

### Proposal Review 页

用户需要能做三件事：

- 看懂：为什么建议改、改哪里、风险是什么。
- 选择：接受、忽略、稍后、编辑后接受。
- 应用：只应用受支持的 change kind。

第一阶段只支持低风险 change kind：

- append_section
- update_frontmatter_field
- create_candidate_skill
- add_verification_note

不支持直接重写全文。

## 分阶段计划

### 阶段 1：Provider 配置和模型调用基础

目标：让桌面版可以配置模型，但不强依赖模型。

任务：

1. 新增模型 provider 数据模型和 API：
   - `GET /api/model-providers`
   - `PUT /api/model-providers`
   - `POST /api/model-providers/test`
2. API Key 存 Vault，配置文件只存 `api_key_ref`。
3. 实现 provider adapters：
   - OpenAI-compatible
   - Ollama
   - Anthropic
   - Gemini
4. 设置页增加 provider 配置 UI。
5. 后端增加统一 `ModelService.GenerateJSON` 和 `ModelService.GenerateText`。

验收：

- 不配置模型时，现有 Skill 学习摘要仍可用。
- 配置 OpenAI-compatible 或 Ollama 后，可以测试连接。
- 连接失败会显示明确错误。

### 阶段 2：每日学习任务升级

目标：从“统计摘要”升级成“可读学习报告”。

任务：

1. 新增 `LearningRunService`。
2. 每天任务流程改成：
   - BuildSkillMap
   - ScanRecentMemory
   - GenerateLearningReport
   - WriteRunJSON
   - WriteReportMarkdown
3. 没有模型时只写结构化报告。
4. 有模型时生成更高质量的“今日学习报告”。
5. Skills 页显示最近 learning run 状态。

验收：

- `/memory/learning/runs/YYYY-MM-DD/run.json` 存在。
- `/memory/learning/runs/YYYY-MM-DD/report.md` 可打开。
- 日更失败不会影响主服务启动。

### 阶段 3：成长提案系统

目标：把模型输出变成可审查 proposal，不直接改 Skill。

任务：

1. 新增 `GrowthProposalService`。
2. 定义 proposal JSON schema。
3. 支持四类 proposal：
   - improve_skill
   - new_skill
   - split_skill
   - archive_or_review
4. 新增 API：
   - `GET /api/growth-proposals`
   - `POST /api/growth-proposals/{id}/accept`
   - `POST /api/growth-proposals/{id}/dismiss`
   - `POST /api/growth-proposals/{id}/apply`
5. 新增 Proposal Review UI。

验收：

- 每日任务能生成 pending proposal。
- 用户可以接受或忽略。
- 应用只处理白名单 change kind。
- 应用后保留 source 和 run id。

### 阶段 4：需求路由和候选 Skill

目标：用户有新需求时，不只是搜已有 Skill，还能建议创建候选 Skill。

任务：

1. 升级 `/api/skills/learning-recommend`：
   - 先返回已有 Skill 候选。
   - 如果匹配不足，生成 `new_skill` proposal。
2. Skills 页的推荐输入框显示：
   - 可直接使用
   - 需要补充后使用
   - 建议新建
3. 候选 Skill 写入 `/skills/_candidates/...` 或 proposal 目录，默认不进入同步。

验收：

- 输入一个新需求，能返回已有 Skill 或候选 Skill 提案。
- 候选 Skill 不会自动同步到 Claude / Codex。

### 阶段 5：验证和质量门槛

目标：让“成长”变得可靠。

任务：

1. 给每个 Skill 增加 verification status。
2. 本地同步前展示验证状态。
3. 对含脚本、依赖、外部引用的 Skill 要求预览验证。
4. proposal 应用前做静态检查。
5. 学习报告统计：
   - 本周新增 proposal
   - 已接受 proposal
   - 被忽略 proposal
   - 同步前风险项

验收：

- 高风险 proposal 不能一键应用。
- 未验证 Skill 在 UI 上有明确标记。
- 同步预览能使用验证状态排序。

当前实现状态（2026-05-24）：

- 已增加 Skill 质量门槛评估，输出 `quality_status`、`quality_stats` 和 `quality_findings`。
- 已自动检查：入口文件、manifest 文件完整性、脚本文件可读性、脚本运行时识别、依赖文件可读性、`package.json` JSON 解析、外部引用是否纳入、环境变量提示、MCP 配置、plugin 元数据、hook 文件。
- 已区分三类非通过结果：`blocked` 阻断同步前处理，`manual_required` 需要人工审查，`warning` 作为提醒。
- Skills 页学习摘要已展示阻断项、需审查项和提醒项；同步预览会显示具体质量提示。
- 当前不会在后台自动执行任意 Skill 脚本，也不会自动调用 Claude Code / Codex 做真实请求；复杂 Skill 会要求目标 Agent 试用，并把 MCP/plugin/hook 标记为需审查。

## 推荐优先级

第一优先级：

1. Model Provider 设置。
2. Learning Run 文件结构。
3. 模型生成学习报告。

第二优先级：

1. Growth Proposal JSON schema。
2. Proposal Review UI。
3. 低风险 proposal 应用。

第三优先级：

1. 候选 Skill 自动生成。
2. 验证门槛。
3. 轻量 provenance 和能力地图。

## 不建议现在做的事

- 不要内置唯一云模型服务，否则会引入成本、隐私和供应商绑定问题。
- 不要默认自动改写 Skill 主文件。
- 不要第一阶段引入图数据库。
- 不要把它做成全功能 Agent 平台。
- 不要把“自学习”包装成模型自己训练。

## 参考资料

- Letta Stateful Agents: https://docs.letta.com/guides/core-concepts/stateful-agents
- Mem0 OSS Overview: https://docs.mem0.ai/open-source/overview
- Graphiti: https://github.com/getzep/graphiti
- LangGraph Persistence: https://docs.langchain.com/oss/python/langgraph/persistence
- LiteLLM Proxy Quick Start: https://docs.litellm.ai/docs/proxy/quick_start
- Aider Repo Map: https://aider.chat/docs/repomap.html
- OpenHands Skills: https://docs.openhands.dev/overview/skills/overview
- Reflexion: https://github.com/noahshinn/reflexion
- Voyager: https://voyager.minedojo.org/
- OpenEvolve: https://github.com/algorithmicsuperintelligence/openevolve
- EvoAgentX: https://github.com/EvoAgentX/EvoAgentX
