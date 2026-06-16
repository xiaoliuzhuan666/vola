# Agent 数据 Hub 相似产品调研与优化建议

更新日期：2026-06-15

## 调研范围

这份文档关注和 Vola 功能相近的公开项目与产品。信息来自项目官网、官方文档、GitHub 仓库，以及仓库内已有研究文档 `docs/codex-console-open-source-research.zh-CN.md`。

本次只把能打开到官方站点、官方文档或 GitHub 仓库的对象写入重点样本。搜索结果里出现但无法确认主页、仓库或维护状态的名称，暂不作为确定对标对象。

## Vola 的位置

Vola 更适合被理解为个人 Agent 数据 Hub：

- 多个 AI 工具共用同一份 profile、memory、projects、skills 和 vault 权限；
- 用户能导入、整理、备份、迁移自己的 Agent 数据资产；
- MCP、API 和本地同步是接入方式，不是产品全部；
- Vola 不应正面竞争 OpenHands、Cline、Roo Code 这类执行型 coding agent，也不应只停留在 MCP gateway。

更清楚的表达是：

> 把 Claude、ChatGPT、Codex、Cursor 等 AI 工具接到同一份个人长期上下文。

## 相似产品分组

| 类别 | 代表产品 | 他们解决的问题 | 对 Vola 的启发 |
| --- | --- | --- | --- |
| 长期记忆 / 共享上下文 | [Mem0](https://github.com/mem0ai/mem0)、[Zep](https://github.com/getzep/zep)、[Graphiti](https://github.com/getzep/graphiti)、[Basic Memory](https://github.com/basicmachines-co/basic-memory) | 给 agent 提供长期记忆、知识图谱、Markdown 或 API 形式的可复用上下文 | Vola 的 memory 需要强调“用户拥有、可备份、可迁移”，但 Vola 还要管理 skills、projects、conversations、vault 和连接权限 |
| MCP 网关 / 工具治理 | [IBM Context Forge](https://github.com/IBM/mcp-context-forge)、[ToolHive](https://github.com/stacklok/toolhive)、[MCP Gateway Registry](https://github.com/agentic-community/mcp-gateway-registry)、[Gate22](https://github.com/aipotheosis-labs/gate22) | 管理 MCP server、registry、权限、安全、审计和运行环境 | MCP 管理会基础设施化，Vola 不能只讲“MCP Hub”，需要把个人数据资产和授权记录放到更前面 |
| 浏览器 / 桌面控制 | [Agentify Desktop](https://github.com/agentify-sh/desktop)、[QuickDesk](https://github.com/barry-ran/QuickDesk)、[Windows-MCP](https://github.com/CursorTouch/Windows-MCP) | 让 agent 控制已登录网页会话或桌面应用 | 这类产品负责“让 agent 做动作”。Vola 更应该负责动作之后留下的资料、记录、记忆和可复用 skill |
| Agent 工作台 | [OpenHands](https://github.com/OpenHands/OpenHands)、[Cline](https://github.com/cline/cline)、[Roo Code](https://github.com/RooCodeInc/Roo-Code)、[Archon](https://github.com/coleam00/Archon)、[Hermes Studio](https://github.com/JPeetz/Hermes-Studio) | 任务执行、代码修改、终端、审批、多 agent 协作、运行记录 | Vola 应服务这些 agent，而不是把自己做成另一个执行平台 |
| 多 Agent 配置 / Skill 分发 | [gaal](https://getgaal.com/)、[skill-depot](https://github.com/Ruhal-Doshi/skill-depot)、[Arcweld](https://arcweld.ai/) | 用统一配置或 workspace 管理多个 AI 工具的 rules、skills、MCP、文件和共享上下文 | 这组最接近 Vola。Vola 需要把导入、转换、权限、备份、审计和迁移讲清楚，和“写配置文件”拉开距离 |

## 需要借鉴的地方

1. **默认从一个入口开始**

   Basic Memory、gaal 这类产品的可理解性来自“一个目录”或“一个配置”。Vola 现在能力更完整，但首页和接入页会让用户同时看到 AI 连接、资料导入、备份、Demo、沙盒、连接状态等多条路线。首次使用应只推荐一个动作：连接第一个 AI 工具。

2. **把连接后的第一句话写出来**

   Memory 和 MCP 项目经常用一个可复制的命令或 prompt 建立信心。Vola 也应该在接入页给出连接后可直接发送的一句话，例如“请读取我的 Vola profile，并总结我的工作偏好”。用户不用理解所有数据模型，也能马上知道是否接通。

3. **把高级能力延后**

   示例数据、网页沙盒、备份、团队共享、Codex Console 都有价值，但不应该在首次连接之前争抢注意力。首屏只回答三件事：连接哪个工具、复制什么、发哪句话测试。

4. **强调数据所有权，而不是工具数量**

   同类项目大多在讲 memory、MCP 或 agent 执行。Vola 的差异是“用户的 Agent 数据资产在一处，并且能备份、迁移和授权”。连接数量可以做状态，不应成为首屏标题。

5. **把 Vola 当数据层，不当执行平台**

   OpenHands、Cline、Roo Code、Hermes Studio 已经覆盖“让 agent 执行任务”。Vola 首页不宜像控制台总览那样把所有模块摊开，更应该让用户感受到：先把常用 AI 工具接进来，它们之后共享同一份长期上下文。

## 应避免的方向

- 不把 Vola 改成另一个 coding agent 工作台；
- 不把 MCP gateway 当唯一卖点；
- 不在首次进入时同时展示所有功能入口；
- 不让 Demo 数据和沙盒压过真实连接路径；
- 不用“导入资料”和“设置备份”与“连接第一个 AI 工具”并列竞争。

## 对当前项目的优化建议

### 首页

- 首屏标题从“三件事”改为“连接第一个 AI 工具，共享长期上下文”；
- 主操作只保留一个：进入 MCP 接入页；
- 导入资料、备份同步变成次级链接；
- 常用连接里只强提示推荐入口，其他工具保留但视觉降级；
- 状态区保留 AI 工具、Hub 资料、工作模式三项，作为背景信息，不再推动用户同时处理三件事。

### 接入页

- 默认推荐 Claude，因为路径短、用户更容易理解 MCP Connector；
- ChatGPT、Cursor、Windsurf 放进“其他工具”区域；
- 复制 URL / 配置和测试 prompt 仍保留三步，但每步文案更短；
- Demo 数据和网页沙盒移到页面下方的可展开区域；
- 连接状态以轻量提示呈现，不再占用大块视觉空间。

### 对外表达

建议持续使用这类句子：

- “把多个 AI 工具接到同一份个人长期上下文。”
- “Profile、memory、projects、skills 和 vault 权限由用户管理。”
- “Vola 不是替你执行任务的 agent，它是 agent 共用的数据层。”

不建议继续把首屏写成：

- “先把 AI、资料和备份三件事理顺。”
- “完成下面三步，再进入高级配置。”
- “连接、导入、备份同等重要。”

## 本次前端调整范围

本次代码调整只处理首次体验：

- 首页首屏文案和主操作；
- 首页常用连接的推荐状态；
- MCP 接入页的默认路径、其他工具入口、复制提示和测试提示词；
- Demo 数据与沙盒位置。

不在本次改动中处理：

- 后端权限模型；
- MCP 协议实现；
- 本地平台写入规则；
- 备份与恢复逻辑；
- Codex Console 数据模型。

## 参考项目索引

### 长期记忆 / 共享上下文

- [Mem0](https://github.com/mem0ai/mem0)
- [Zep](https://github.com/getzep/zep)
- [Graphiti](https://github.com/getzep/graphiti)
- [Basic Memory](https://github.com/basicmachines-co/basic-memory)
- [Origin](https://useorigin.app/)
- [Dory](https://dory.deeflect.com/)
- [AI Workspace](https://github.com/lee-to/ai-workspace)
- [Awareness-Local](https://github.com/edwin-hao-ai/Awareness-Local)
- [cortex-hub](https://github.com/lktiep/cortex-hub)

### MCP 网关 / 工具治理

- [IBM Context Forge](https://github.com/IBM/mcp-context-forge)
- [ToolHive](https://github.com/stacklok/toolhive)
- [MCP Gateway Registry](https://github.com/agentic-community/mcp-gateway-registry)
- [Gate22](https://github.com/aipotheosis-labs/gate22)

### 浏览器 / 桌面控制

- [Agentify Desktop](https://github.com/agentify-sh/desktop)
- [QuickDesk](https://github.com/barry-ran/QuickDesk)
- [Windows-MCP](https://github.com/CursorTouch/Windows-MCP)

### Agent 工作台

- [OpenHands](https://github.com/OpenHands/OpenHands)
- [Cline](https://github.com/cline/cline)
- [Roo Code](https://github.com/RooCodeInc/Roo-Code)
- [Archon](https://github.com/coleam00/Archon)
- [Hermes Studio](https://github.com/JPeetz/Hermes-Studio)
- [Accomplish](https://github.com/accomplish-ai/accomplish)
- [Mission Control](https://github.com/builderz-labs/mission-control)

### 多 Agent 配置 / Skill 分发

- [gaal](https://getgaal.com/)
- [skill-depot](https://github.com/Ruhal-Doshi/skill-depot)
- [Arcweld](https://arcweld.ai/)

### 相关公开资料

- [OpenAI Codex use cases](https://developers.openai.com/codex/use-cases)
- [OpenAI Codex Chronicle](https://developers.openai.com/codex/memories/chronicle)
