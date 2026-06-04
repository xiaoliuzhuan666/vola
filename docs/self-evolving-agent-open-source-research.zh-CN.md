# Vola 自进化 / 自学习开源项目调研

## 结论先说

现在开源圈里真正成熟的，不是“模型自己变聪明”这件事，而是四层东西叠起来：

1. 记忆分层
2. 状态持久化
3. 技能/工作流复用
4. 基于失败和反馈的再生成

对 Vola 来说，最值钱的不是把桌面端做成一个更大的 Agent 控制台，而是把它做成一个会持续整理、总结、推荐、验证的本机学习入口。用户要的是简单工具能完成实际需求，不是一个功能越来越杂的壳。

## 我看了哪些项目

### 1. 记忆层

**Letta / MemGPT**

- [Letta memory overview](https://docs.letta.com/guides/agents/memory)
- [MemGPT paper](https://arxiv.org/abs/2310.08560)

它的核心不是“聊天记录多存一点”，而是把记忆拆成层：in-context memory、out-of-context memory、archival memory。它还强调工具驱动的 memory management，让 agent 决定该记什么、什么时候取回来。

对 Vola 的启发很直接：不要把所有信息都塞进一个大“资料库”。profile、memory、projects、skills 应该是不同层，读写规则也要不同。

**Mem0**

- [Open source overview](https://docs.mem0.ai/open-source/overview)
- [Memory types](https://docs.mem0.ai/core-concepts/memory-types)
- [Memory evaluation](https://docs.mem0.ai/core-concepts/memory-evaluation)

Mem0 做得比较像“生产可用的记忆引擎”。它把记忆拆成 conversation / session / user / organizational 四层，还把写入和检索拆开，支持实体作用域、冲突处理、向量 + 图的混合存储。

对 Vola 最值得抄的，是“按实体和作用域存记忆”，以及“写入和检索分离”。这比单纯做全文搜索更像真正可用的学习系统。

**Graphiti / Zep**

- [Graphiti GitHub](https://github.com/getzep/graphiti)
- [Zep temporal knowledge graph paper](https://arxiv.org/abs/2501.13956)

Graphiti 的重点是 temporal knowledge graph。它不是只记事实，而是记事实之间的关系、时间和变化。这个方向很适合做“项目演进”“skill 变化”“用户偏好变化”。

对 Vola 来说，它更像一层“记忆结构化引擎”，适合把零散资料变成能推理的关系网。

**LangGraph**

- [Persistence](https://docs.langchain.com/oss/python/langgraph/persistence)
- [Memory](https://docs.langchain.com/oss/python/langgraph/memory)

LangGraph 的长处是 checkpoint、thread、replay、update_state 这一套。它解决的不是“记什么”，而是“怎么持续跑、怎么恢复、怎么回放”。

对 Vola 的借鉴点是：学习过程也应该可回放、可恢复、可审计。扫描、导入、转换、应用这些动作，不要只留结果，要留 checkpoint。

### 2. 任务 / 代码层

**Aider**

- [Repo map](https://aider.chat/docs/repomap.html)

Aider 很关键的一点，是它会先把整个代码库压成一个 repo map，再把关键符号和定义喂给模型。它不是靠“多塞上下文”工作，而是靠“先做结构化压缩”工作。

对 Vola 来说，这一招可以直接借到 skill / project 扫描里：先生成“skill map”，再做总结、推荐和迁移，而不是把整个目录原样丢给模型。

**OpenHands**

- [Skills overview](https://docs.openhands.dev/overview/skills/overview)
- [OpenHands skills README](https://github.com/OpenHands/OpenHands/blob/main/skills/README.md)

OpenHands 的 skill 系统很实用：技能是可共享的 markdown 文件，能按关键词或上下文触发，还能放进组织级 registry。它把“知识”做成了可复用、可分发、可约束的文件。

对 Vola 最有价值的是：技能不是页面上的标签，而是有结构、有注册、有触发规则的资产。

### 3. 反思 / 技能层

**Reflexion**

- [Paper](https://arxiv.org/abs/2303.11366)
- [GitHub](https://github.com/noahshinn/reflexion)

Reflexion 的核心是 verbal reinforcement learning：把失败、反馈、自我批评写成文本，再喂回下一轮。它不靠重新训练模型，而是靠反思链路提升下一次表现。

这对 Vola 很有用，因为它说明“自学习”不一定要动模型参数。对产品来说，更现实的是：把失败轨迹、用户修正、导入冲突、验证结果都变成可复用的反思记录。

**Voyager**

- [Paper](https://arxiv.org/abs/2305.16291)
- [Official site](https://voyager.minedojo.org/)

Voyager 的三个组件很值得看：automatic curriculum、ever-growing skill library、iterative prompting with feedback / self-verification。它不是靠一次成功，而是靠持续探索、成功后把技能沉淀进库里。

这和 Vola 很像：你不是要一个“能看文件的桌面壳”，而是要一个“能把本地 skill 变成可复用技能库”的系统。

**EvoSkill / EvoSkills**

- [EvoSkill GitHub](https://github.com/sentient-agi/EvoSkill)
- [EvoSkill paper](https://arxiv.org/abs/2603.02766)
- [EvoSkills paper](https://arxiv.org/abs/2604.01687)

EvoSkill 比 Voyager 更贴近 Vola：它直接从失败轨迹里发现和合成可复用 skill，再做评估，最后才保留。它强调 transferable skills，不是只修单次任务。

这正好对应 Vola 最该有的能力：从本地 skill 扫描、使用历史、失败记录里，提炼出“这个 skill 到底能不能复用”的判断。

### 4. 演化 / 工作流层

**OpenEvolve**

- [GitHub](https://github.com/algorithmicsuperintelligence/openevolve)

OpenEvolve 走的是 AlphaEvolve 那条路：LLM + 演化搜索 + 多轮变异 + 评估。它更像一个“候选方案生成器”，而不是一个聊天机器人。

它适合借鉴的点是：所有“自进化”都必须有评估门禁，不然只是随机试错。

**A-Evolve**

- [GitHub](https://github.com/A-EVO-Lab/a-evolve)
- [Position paper](https://arxiv.org/abs/2602.00359)

A-Evolve 的观点更激进：把演化本身做成 autonomous evolver agent。它的价值不在“更会聊”，而在“会持续改自己”。

对 Vola 来说，可以借它的流程观，但不要照搬它的野心。桌面产品更需要“可解释的学习记录”，不是不可控的自动改写。

**EvoAgentX**

- [Official site](https://www.evoagentx.org/)
- [GitHub](https://github.com/EvoAgentX/EvoAgentX)

EvoAgentX 的重点是自动生成、执行、评估、优化 agentic workflows。它更像一个自我迭代的工作流工厂。

对 Vola 的启发是：你可以把“学习”做成工作流，而不是做成静态页面。比如“扫描 -> 归纳 -> 生成候选技能 -> 评估 -> 通过才入库”。

## 这些项目真正可以借到什么

### 1. 记忆不要只有一层

最该借的是 Mem0 / Letta / Graphiti 这条线。

- 短期上下文和长期记忆要分开
- 用户级、团队级、Agent 级要分开
- 事实、偏好、过程、结论要分开
- 写入、检索、更新、删除要分开

这会直接影响 Vola 的数据模型，而不是只影响 UI。

### 2. 学习过程要可回放

LangGraph 的 checkpoint / replay / update_state 很适合做学习审计。

Vola 里每次扫描 skill、导入项目、转换 skill、应用本地写入，都应该留下：

- 输入是什么
- 发生了什么
- 输出是什么
- 哪一步失败了
- 哪一步被用户接受了

这样“自学习”才不是一句空话。

### 3. 先压缩，再理解

Aider 的 repo map 思路很适合 Vola 的 skill 扫描。

不要把整个 skill 目录直接当上下文扔进去。先做结构化摘要：

- 入口文件
- 脚本
- 依赖
- 外部引用
- 支持的平台
- 最近变更
- 可复用程度

这比“扫描更多文件”更有价值。

### 4. 技能要能注册、触发、版本化

OpenHands、Voyager、EvoSkill 的共同点是，技能不是散的文本，而是带有明确边界的资产。

Vola 很适合把 skill 做成：

- manifest
- scripts
- dependencies
- external refs
- supported agents
- verification result

再往上走，才是“自动推荐给哪个 Agent 用”。

### 5. 失败轨迹比成功样例更值钱

Reflexion 和 EvoSkill 都在说明一件事：真正有用的提升，很多时候来自失败后的总结，而不是成功后的自夸。

Vola 可以把失败轨迹沉淀成：

- 冲突记录
- 反思卡片
- 规则修订建议
- 待验证 skill patch

这是比“加一个聊天页”更实在的自学习能力。

## 对 Vola 的产品建议

### 1. 别把桌面端做成大杂烩

你现在这版桌面端如果只是“扫本地 skill”，确实会显得单薄；但补法不是无限加功能，而是把它收成一个清晰的学习入口。

建议前台只保留四个主动作：

1. 扫描
2. 总结
3. 推荐
4. 应用

### 2. 自学习放后台，前台保持简单

用户不需要看见一堆“AI 很聪明”的模块，他们只想知道：

- 这次扫到了什么
- 哪些 skill 值得复用
- 哪些内容过期了
- 下一步该做什么

### 3. 先别把边界外的东西做成默认入口

你仓库文档里已经把 `devices / roles / inbox / collaboration` 收成设计预研了，当前稳定能力还是 `profile / memory / projects / skills / tree / token / sync`。这个收口是对的。

现在最该做的不是扩入口，而是把现有入口做得更准、更省事。

### 4. 产品语言要换

不要把 Vola 讲成“自进化 AI 平台”。那会很虚。

更适合的说法是：

- 持续整理你的 Agent 知识
- 把本地 skill 变成可复用资产
- 让经验可沉淀、可比较、可验证
- 让下一次使用比上一次更省事

## 我建议你下一步优先做的事

1. 先做一个“本地 skill 学习摘要页”
2. 给每个 skill 补上版本、来源、用途、最近使用、最近验证
3. 做失败轨迹到反思卡片的转化
4. 做 skill 候选晋级流程，只有通过验证才进主库
5. 把桌面端收成“扫描 + 学习 + 应用”三段，不再向外扩面

## 参考资料

- [Letta memory overview](https://docs.letta.com/guides/agents/memory)
- [MemGPT paper](https://arxiv.org/abs/2310.08560)
- [Mem0 open-source overview](https://docs.mem0.ai/open-source/overview)
- [Mem0 memory types](https://docs.mem0.ai/core-concepts/memory-types)
- [Graphiti GitHub](https://github.com/getzep/graphiti)
- [LangGraph persistence](https://docs.langchain.com/oss/python/langgraph/persistence)
- [LangGraph memory](https://docs.langchain.com/oss/python/langgraph/memory)
- [Aider repo map](https://aider.chat/docs/repomap.html)
- [OpenHands skills overview](https://docs.openhands.dev/overview/skills/overview)
- [Reflexion paper](https://arxiv.org/abs/2303.11366)
- [Voyager paper](https://arxiv.org/abs/2305.16291)
- [EvoSkill GitHub](https://github.com/sentient-agi/EvoSkill)
- [OpenEvolve GitHub](https://github.com/algorithmicsuperintelligence/openevolve)
- [A-Evolve GitHub](https://github.com/A-EVO-Lab/a-evolve)
- [EvoAgentX GitHub](https://github.com/EvoAgentX/EvoAgentX)
