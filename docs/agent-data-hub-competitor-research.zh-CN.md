# Agent 数据 Hub 相似产品调研与优化建议

更新日期：2026-06-17

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

最新核心原则见 `docs/core-principles.zh-CN.md`：

> Vola 是给个人和小团队用的 Agent 资料中心：把 profile、memory、projects、skills、MCP、prompt 和 playbook 放在一个私有 Hub 里，再安全同步到 Codex、Claude Code 等本机工具。

## 相似产品分组

| 类别 | 代表产品 | 他们解决的问题 | 对 Vola 的启发 |
| --- | --- | --- | --- |
| 长期记忆 / 共享上下文 | [Mem0](https://github.com/mem0ai/mem0)、[Zep](https://github.com/getzep/zep)、[Graphiti](https://github.com/getzep/graphiti)、[Basic Memory](https://github.com/basicmachines-co/basic-memory) | 给 agent 提供长期记忆、知识图谱、Markdown 或 API 形式的可复用上下文 | Vola 的 memory 需要强调“用户拥有、可备份、可迁移”，但 Vola 还要管理 skills、projects、conversations、vault 和连接权限 |
| MCP 网关 / 工具治理 | [IBM Context Forge](https://github.com/IBM/mcp-context-forge)、[ToolHive](https://github.com/stacklok/toolhive)、[MCP Gateway Registry](https://github.com/agentic-community/mcp-gateway-registry)、[Gate22](https://github.com/aipotheosis-labs/gate22) | 管理 MCP server、registry、权限、安全、审计和运行环境 | MCP 管理会基础设施化，Vola 不能只讲“MCP Hub”，需要把个人数据资产和授权记录放到更前面 |
| 浏览器 / 桌面控制 | [Agentify Desktop](https://github.com/agentify-sh/desktop)、[QuickDesk](https://github.com/barry-ran/QuickDesk)、[Windows-MCP](https://github.com/CursorTouch/Windows-MCP) | 让 agent 控制已登录网页会话或桌面应用 | 这类产品负责“让 agent 做动作”。Vola 更应该负责动作之后留下的资料、记录、记忆和可复用 skill |
| Agent 工作台 | [OpenHands](https://github.com/OpenHands/OpenHands)、[Cline](https://github.com/cline/cline)、[Roo Code](https://github.com/RooCodeInc/Roo-Code)、[Archon](https://github.com/coleam00/Archon)、[Hermes Studio](https://github.com/JPeetz/Hermes-Studio) | 任务执行、代码修改、终端、审批、多 agent 协作、运行记录 | Vola 应服务这些 agent，而不是把自己做成另一个执行平台 |
| 多 Agent 配置 / Skill 分发 | [gaal](https://getgaal.com/)、[skill-depot](https://github.com/Ruhal-Doshi/skill-depot)、[Arcweld](https://arcweld.ai/) | 用统一配置或 workspace 管理多个 AI 工具的 rules、skills、MCP、文件和共享上下文 | 这组最接近 Vola。Vola 需要把导入、转换、权限、备份、审计和迁移讲清楚，和“写配置文件”拉开距离 |

## 2026-06-17 补充：cc-switch、SkillHub 与 Vola 的差异

本次补充聚焦用户近期提到的 cc-switch、SkillHub、SkillHub MCP 等项目。

| 项目 | 主要定位 | 值得关注的能力 | 和 Vola 的区别 |
| --- | --- | --- | --- |
| [cc-switch](https://github.com/farion1231/cc-switch) | 多 AI 编程工具的桌面管理器，覆盖 Claude Code、Claude Desktop、Codex、Gemini CLI、OpenCode、OpenClaw、Hermes 等 | provider presets、本地代理、热切换、MCP / Skills 管理、system tray、使用量和成本统计、配置备份与跨设备同步 | cc-switch 重点管理工具配置、模型 provider、代理和切换体验。Vola 重点管理 Agent 资料资产：profile、memory、projects、skills、MCP、prompt、playbook，以及它们如何安全进入本机工具 |
| [SkillHub.club](https://www.skillhub.club/) / [SkillHub Desktop](https://github.com/skillhub-club/skillhub-desktop) | 面向 Agent Skill 的公开发现、安装和桌面管理 | Skill 搜索、目录浏览、一键安装、多工具同步、集合、AI 辅助创建和改写、命令面板 | SkillHub 更像公开 Skill 目录和桌面安装器。Vola 不以公开市场为主，而是保存个人和小团队的私有资料、团队经验、权限和备份 |
| [iflytek SkillHub](https://github.com/iflytek/skillhub) | 面向组织内部的自托管企业 Skill registry | 发布与版本、namespace、角色、review、全局推广、audit log、CLI、对象存储 | iflytek SkillHub 是企业级私有 registry。Vola 的 Team Library 当前只服务小团队资料共享，不承诺企业治理平台能力 |
| [skillhub-club/mcp-server](https://github.com/skillhub-club/mcp-server) | 通过 MCP 搜索、发现、安装 SkillHub 里的 Agent Skills | 让 Agent 通过 MCP 获取 Skill 发现和安装能力 | 它把外部 Skill 目录暴露给 Agent。Vola 的 MCP 重点是读写用户自己的 Hub 资料和团队资产 |
| [artuntan/skillhub-mcp](https://github.com/artuntan/skillhub-mcp) | AI 资源推荐 MCP server | 推荐 skills、tools、agents、rules、MCP servers 等资源 | 它更像外部资源推荐器。Vola 可以借鉴“按问题推荐资源”，但安装和同步仍要经过用户确认和安全路径 |

这些项目说明市场已经把几个方向分开了：

- provider / 模型 / 代理管理会继续由 cc-switch 这类工具做深；
- 公开 Skill 发现会由 SkillHub.club 这类目录型产品推进；
- 企业 Skill registry 会有 namespace、review、audit 和合规要求；
- MCP server 可以成为资源发现入口；
- Vola 更适合守住个人和小团队的私有 Agent 资料中心定位。

## 可以吸收到 Vola 的做法

| 来源 | 可吸收做法 | Vola 中的产品形态 |
| --- | --- | --- |
| cc-switch | 首次启动自动检测本机工具，展示每个工具的配置状态 | 连接页显示 Codex、Claude Code、Cursor、Gemini CLI 的可用状态、配置路径和最近一次同步结果 |
| cc-switch | 配置修改前有备份和状态提示 | 继续坚持 `SafeUpdateMcpConfig`、配置锁、同步预览、冲突提示和 Vola 管理标记 |
| cc-switch | system tray 和快速切换降低日常操作成本 | 桌面版可以把“打开 Hub”“刷新 Codex”“刷新 Claude Code”“查看同步状态”放进常驻入口 |
| SkillHub.club / Desktop | Skill 搜索、标签、集合和命令面板 | Vola 可为个人和团队 Skill 增加标签、集合、推荐目录和搜索排序 |
| SkillHub.club / Desktop | 一条命令或一个按钮安装 Skill | Team Library 中保留“安装到个人空间”“同步到 Codex / Claude Code”的直接入口 |
| SkillHub.club | Skill Stack / 套装概念 | Vola 可提供小团队入门资料包，例如“研发团队评审包”“客服知识包”“发布流程包”，内容放在私有 Hub 内 |
| iflytek SkillHub | 发布状态、版本、tag、namespace | Team Library 可继续强化 `draft`、`published`、`archived`、版本说明、负责人和更新提醒 |
| iflytek SkillHub | CLI-first 和安装说明清楚 | `neu` 可以提供更短的引导命令，例如检测、连接、测试、同步四个状态放在同一条状态输出里 |
| SkillHub MCP | 通过 MCP 做资源推荐 | Vola 可以增加“推荐可导入资源”视图，但只生成预览和说明，不自动安装第三方 MCP server |

## 降低使用成本的产品建议

这些建议不要求改变 Vola 的安全边界，适合排进后续迭代：

1. **首页只推荐一个主路径**

   新用户默认看到“连接 Codex”，旁边给出“连接 Claude Code”。Cursor 和 Gemini CLI 作为导出型平台展示，不放在同等优先级。

2. **连接后直接给测试指令**

   成功连接后显示一条可复制的句子：`请读取我的 Vola profile、skills 和最近项目上下文，并告诉我已经能访问哪些资料。`

3. **把状态拆成用户能理解的几项**

   页面和 CLI 分别显示：`neu` 是否安装、Hub 是否运行、账号是否登录、Codex / Claude Code 是否已连接、团队 Skill / MCP 是否有待同步。

4. **团队资产路径可视化**

   Team Library 显示“团队 Skill / MCP -> 个人空间或本机配置 -> Codex / Claude Code”这条路径。成员不需要知道背后是否重新执行了 `neu connect codex` 或 `neu connect claude`。

5. **导出型平台明确提示**

   Cursor 和 Gemini CLI 卡片直接显示“可导出，不自动改配置”，并提供导出包、说明文件和目标路径建议。

6. **空状态给模板**

   空团队资料库默认给 `/team/mcp`、`/team/prompts`、`/team/playbooks`、`/skills/<name>` 的模板。用户不用自己设计目录。

7. **同步前后都有可读结果**

   同步前显示新增、更新、冲突；同步后显示写入了哪里、哪些内容没有处理、下一步在 Codex / Claude Code 里如何验证。

8. **保留高级入口，但视觉降级**

   GitHub Backup、外部备份、Skill 转换、Codex Console、MCP Gateway 等能力仍保留，但首次使用优先完成连接和测试。

9. **用角色化资料包帮助起步**

   小团队可从“研发团队评审包”“发布流程包”“客户支持知识包”开始，再按自己的团队经验改。资料包仍保存到私有 Hub，不变成公开市场。

10. **所有自动动作都能解释**

   凡是写本机目录或配置的动作，都要说明目标平台、目标路径、是否由 Vola 管理、遇到冲突时为什么停下。

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
- [cc-switch](https://github.com/farion1231/cc-switch)
- [SkillHub.club](https://www.skillhub.club/)
- [SkillHub Desktop](https://github.com/skillhub-club/skillhub-desktop)
- [iflytek SkillHub](https://github.com/iflytek/skillhub)
- [SkillHub MCP Server](https://github.com/skillhub-club/mcp-server)
- [skillhub-mcp](https://github.com/artuntan/skillhub-mcp)

### 相关公开资料

- [OpenAI Codex use cases](https://developers.openai.com/codex/use-cases)
- [OpenAI Codex Chronicle](https://developers.openai.com/codex/memories/chronicle)
