# AI 开发团队的多项目需求连续性与接手机制调研

更新日期：2026-05-25

研究对象：多人分别负责项目、开发大量依赖 AI、模型与客户端不统一、历史需求资料不足、项目需要被他人继续维护的团队

适用范围：工程项目的需求理解、变更执行、交接和验收；不代替产品负责人对新业务决策的确认

## 核心判断

团队不能靠“大家都使用同一种模型”解决需求偏差。即使模型相同，会话上下文、提示词、读取顺序和代码版本不同，也会给出不同解释。

真正可执行的办法是把 AI 从“需求解释的裁判”改为“读取项目事实并提交变更的执行者”：

1. 每个项目在 Git 仓库内保存一份版本化的项目事实，包括目标、术语、功能规格、接口契约、决策记录、验收场景和已知未知项。
2. 每个功能变更携带独立的变更包，写明改变哪些现有规格、为何改变、如何验证、哪些内容明确不改变。
3. AI 只可依据标有来源的资料陈述现状；资料冲突或缺失时必须标记 `UNKNOWN` 或 `CONFLICT`，不能自行发明业务目标。
4. 需求文件不以“写完”为完成条件，只有验收场景、自动检查或人工验收证据与规格对应，才可以进入发布判断。
5. 无需每次举行宣讲会议，但必须有一名能对业务意图作异步确认的人。完全没有负责确认的人，最多能保持代码行为一致，无法证明产品目标正确。

对当前问题的工具建议：

| 场景 | 建议 |
| --- | --- |
| 大量已有项目、资料欠缺、需要低成本开始接手 | 选 2 个项目试行 **OpenSpec 风格的当前规格 + 变更包 + 归档**，并加入本报告规定的验收与审批规则 |
| 已经使用 Spec Kit 的项目 | **继续使用，不要为了换工具迁移文档**；增加需求编号、证据、未知项、交接记录，并固定执行 `clarify`、`checklist`、`analyze` 质量流程 |
| 新项目或影响范围很大的新能力 | Spec Kit 适合提供较完整的需求、设计、任务流程 |
| 每位成员使用不同 Agent 或模型 | 用 `AGENTS.md` 与 Agent Skills 统一读取顺序和工作步骤；它们不能充当需求真相 |
| 团队资料传播与跨 Agent 读取 | Vola 目前可保存 playbook、prompt、Skill 和索引；项目正式规格仍应首先进入对应代码仓库与 PR 历史 |

## 问题到底出在哪里

### 1. 聊天记录不是需求资产

AI 会话里经常存在这些内容：

- 临时解释，没有进入仓库；
- 仅对某次实现有效的设想；
- 已被代码修改推翻但仍在历史消息里的旧要求；
- 模型根据代码或界面自行推断出的业务含义；
- 通过多轮聊天逐渐改变、却没有明确记录的验收条件。

后续维护者把历史对话喂给另一个模型，得到的是“对一组混合材料的再次解释”，不是可靠的项目基准。

### 2. 代码只能证明现状，不能独自证明意图

已有代码、测试和线上行为很重要，但它们只能说明某个版本怎样运行。它们无法单独回答：

- 当前行为是不是历史缺陷；
- 某项限制是产品规则还是临时技术处理；
- 某个字段未来是否必须保持兼容；
- 当两个实现冲突时，哪一个符合商业目标。

因此，接手时应同时记录两列事实：

| 类型 | 回答的问题 | 可用依据 |
| --- | --- | --- |
| `INTENDED` 期望行为 | 产品要求是什么 | 已批准规格、决策记录、明确的验收说明 |
| `OBSERVED` 已有行为 | 当前系统实际怎样运行 | 发布版本、接口、数据结构、测试、日志、界面检查 |

`INTENDED` 缺失时，AI 可以整理 `OBSERVED`，但不得把它改名为需求。

### 3. 更换模型只是放大已有缺口

不同模型产生差异，并不是首要问题。首要问题是团队没有把业务规则、禁止变化的边界和验收条件保存为可检查资料。只要输入允许多种解释，任何模型都可能选择其中一种。

## 本次调研依据

### 仓库现状

本次阅读了 Vola 现有资料，得到以下与方案相关的事实：

| 已有能力或边界 | 依据 | 对本议题的意义 |
| --- | --- | --- |
| 产品定位是跨 Agent 的个人数据 Hub，保存 profile、memory、projects、skills 等资料 | `README.zh-CN.md` | 可以帮助不同 Agent 访问资料，但不等于拥有需求治理制度 |
| Team Library 当前支持 `/team/mcp`、`/team/prompts`、`/team/playbooks` 与团队 `/skills` | `docs/team-ai-library.zh-CN.md` | 适合分发团队模板、审阅流程和读取 Skill |
| Team Library 目前不是企业审批、审计或完整项目协作平台 | `README.zh-CN.md`、`docs/team-ai-library.zh-CN.md`、`docs/launch-test-checklist.zh-CN.md` | 需求批准和合并规则仍应依赖仓库 PR 与负责人 |
| 项目资料与结构化日志已有 `/projects` 数据面 | `docs/reference.zh-CN.md`、`web/src/pages/ProjectsPage.tsx` | 可保存项目摘要或交接材料，但不应绕过代码仓库的规格历史 |
| 已有模型学习与 proposal 研究强调来源、模型信息和审查 | `docs/model-provider-learning-engine-research.zh-CN.md` | 可复用到未来的规格检查或差异提示，不应自动改需求 |

当前仓库未发现 `.specify`、`openspec`、`.kiro` 或既有 `specs` 管理目录。因此，本报告属于治理方案与采用建议，不表示 Vola 已具备需求审批功能。

### 外部方案与标准资料

资料检索时间为 2026-05-25。优先使用项目官方文档、官方发布信息、标准或论文原文。

| 资料 | 当前可核实信息 | 本报告使用方式 |
| --- | --- | --- |
| GitHub Spec Kit | 官方 release 最新为 `v0.8.13`，发布于 2026-05-21；README 说明支持 30+ Agent 集成，包含 `constitution`、`specify`、`plan`、`tasks`、`implement` 以及 `clarify`、`checklist`、`analyze` | 评估继续使用 Spec Kit 的条件与质量步骤 |
| OpenSpec | 官方 release 最新为 `v1.3.1`，发布于 2026-04-21；README 将其定位为适合 brownfield 的轻量 spec layer，基本流程为 proposal / apply / archive，扩展流程包含 verify 与 onboard，支持 25+ 工具 | 作为已有项目低成本试行候选 |
| Kiro Specs | 官方文档（页面更新时间 2026-05-05）说明规格产生 `requirements.md`、`design.md`、`tasks.md`，并提供需求分析能力 | 作为 IDE 内集成参考，不选作混合客户端团队的统一前提 |
| AGENTS.md | 官方站点将其定义为用于指导 coding agents 的开放 Markdown 格式，并支持嵌套目录说明 | 用于保存项目读取顺序、命令、禁止操作和验证要求 |
| Agent Skills Specification | 官方规格规定 `SKILL.md` 与可选 `scripts/`、`references/`、`assets/`，并采用按需加载 | 用于传播可复用工作流程，不作为某一项目的业务规格 |
| EARS | Mavin 等提出以事件、状态和系统响应形成约束化自然语言需求 | 用于编写可检验的功能要求 |
| ISO/IEC/IEEE 29148:2018 | 标准覆盖需求工程过程和需求信息项 | 用于确定需求文件应具备的完整性、可验证性等性质 |
| Gherkin / Cucumber | 官方参考定义 Given / When / Then 等场景结构 | 用于验收场景书写和自动化映射 |
| OpenAPI 与 Pact | 官方资料分别提供 API 描述规范和消费者契约验证方式 | 用于将接口要求变成机器可检查资料 |
| GitHub protected branches / rulesets | 官方文档支持必需 PR 审阅、CODEOWNERS 审阅与必需状态检查 | 用于让规格和验证规则真正影响合并 |

此外，Thoughtworks Technology Radar Volume 34（2026-04）将 **spec-driven development** 置于 `Assess`，既认可需求、规格、计划和任务的结构化价值，也提醒规格资料会产生上下文膨胀和与代码不同步的风险。这正是本报告不主张“文档越多越好”、而强调证据和维护责任的原因。

## 方案比较

### Spec Kit：可以继续，但需要改变使用方法

Spec Kit 并未过时。官方在 2026-05-21 仍发布新版，且支持多种 Agent 接入。它的优点是：

- 项目原则、功能规格、技术计划、任务和执行阶段清晰；
- `clarify` 可在计划前发现未说明的问题；
- `checklist` 可检查规格的完整性、清晰度和一致性；
- `analyze` 可在实现前检查各份资料之间是否互相矛盾；
- 可用 preset 统一团队模板。

它不能单独解决的问题是：

- 官方哲学明确写有对高级模型能力的较强依赖，模型仍参与解释；
- 资料若没有负责人更新，会和代码一起发生偏离；
- 对没有历史规格的已有项目，不能直接生成“真实业务意图”；
- 社区 extensions 与 presets 由第三方维护，官方说明并未审查、背书或支持其代码。

适用决策：

| 情况 | 用法 |
| --- | --- |
| 既有项目已经保存 Spec Kit 文件 | 保留原体系；把本报告的证据、未知项、验收映射加入项目模板 |
| 新建系统或一次重大产品改造 | 使用 Spec Kit 的完整流程 |
| 小规模持续修复、历史项目刚开始整理 | 不必强迫所有项目立即创建大量阶段文件；先用轻量变更包试行 |

### OpenSpec：更适合从已有项目开始试行

OpenSpec 的官方描述直接面向 brownfield 项目，其结构适合处理“当前已有行为”与“这次要改变什么”的差别：

```text
openspec/
  specs/                     # 当前认可的规格
  changes/
    <change-name>/
      proposal.md            # 为什么改变
      specs/                 # 本次对规格的增加、修改或移除
      design.md              # 技术决定
      tasks.md               # 执行清单
    archive/                 # 已完成并归档的变更
```

优点：

- 日常修改只创建本次相关资料，不要求先写完整产品百科；
- 适合从一个真实功能开始逐渐恢复项目规格；
- 官方 README 明确面向多种 AI assistants，不要求统一模型；
- 扩展工作流已有 `verify` 和 `onboard` 概念。

限制：

- 轻量也意味着团队要自行增加审批规则、证据文件和 CI 检查；
- “AI 生成了一份 spec”仍不是批准，必须由责任人或既有明确证据确认；
- 不应在试行前把它当成跨所有仓库的协作平台。

建议：对于目前资料不足、同时又要继续迭代的已有项目，将 OpenSpec 用作两项目试行的目录和工作流基础，比立即要求全部项目采用完整阶段流程更容易执行。

### Kiro Specs：可借鉴结构，不作为统一依赖

Kiro 官方把规格组织为 `requirements.md`、`design.md` 和 `tasks.md`，并提供 requirements analysis，结构合理。它适合已在 Kiro 内工作的成员。

团队当前的问题是模型和客户端不统一。统一机制应让 Codex、Claude、Cursor、IDE 和未来客户端都能读取普通 Git 文件。因此可以借用 Kiro 的三类资料结构，但不应要求每个人切换同一 IDE 才能参与项目。

### AGENTS.md 与 Agent Skills：规定工作方式，不规定产品真相

两者都适合团队使用，但承担不同职责：

| 文件类型 | 应保存什么 | 不应保存什么 |
| --- | --- | --- |
| `AGENTS.md` | 资料读取顺序、构建测试命令、敏感文件限制、规格变更要求、PR 检查要求 | 某个功能的全部业务规则、会持续变化的需求正文 |
| `SKILL.md` | 接手审阅流程、生成验收证据流程、API 契约检查流程、报告模板与脚本 | 某个仓库当前批准的功能状态 |

只把需求藏在某人的 Skill 或某个 Agent memory 里，下一位维护者仍然无法确认它是否是该项目的正式要求。

## 推荐机制：项目事实与变更制度

### 1. 要保存的六类资料

| 资料 | 解决什么问题 | 是否需要进代码仓库 | 修改责任 |
| --- | --- | --- | --- |
| 项目说明与术语表 | 防止相同词被不同模型理解成不同对象 | 是 | 项目负责人 |
| 当前规格 `specs` | 定义现阶段认可的功能和约束 | 是 | 业务负责人批准，工程人员维护 |
| 变更包 `changes` | 记录本次改动与不改动内容 | 是 | 当前任务负责人 |
| 决策记录 `decisions` / ADR | 解释为何选择某项行为或架构 | 是 | 作出决定的人 |
| 验收证据 `evidence` | 说明哪些要求已经验证，哪些还没有 | 是 | 实现与验收负责人 |
| 交接摘要 `handover` | 帮助下一位维护者找到当前状态与风险 | 是；跨项目索引可同步到 Hub | 离开项目或完成里程碑的人 |

重要规则：聊天原文、AI memory 和个人笔记可以作为线索，但只有进入上述资料并被审阅的内容，才属于团队可依赖的项目事实。

### 2. 来源等级

AI 接手项目时必须按下表标注依据，不能把所有材料混在一个摘要里：

| 等级 | 类型 | 能证明什么 | 处理冲突方式 |
| --- | --- | --- | --- |
| A | 已批准的规格、决策记录、验收批准 | 期望行为 | 同级冲突必须请求责任人决定 |
| B | 自动验收测试、OpenAPI/Pact 契约、已发布界面或接口检查记录 | 实际已验证行为 | 与 A 不同则记为偏差，不直接改变 A |
| C | 代码实现、数据库迁移、配置、日志 | 当前实现线索 | 生成待确认项并以测试核验 |
| D | issue、PR 讨论、聊天、AI 会话总结、个人 memory | 背景与线索 | 不可直接升级为需求 |

项目没有 A 类资料时，第一项工作不是实现新功能，而是根据 B/C/D 创建“现状规格草案”和“待确认清单”。高影响未知项被确认前，不执行会改变业务规则的修改。

### 3. 推荐目录

已有项目试行建议采用下列 Git 目录。它遵循 OpenSpec 的当前规格/变更思路，同时增加决策、证据和交接所需内容：

```text
AGENTS.md
openspec/
  project.md                 # 目标、边界、负责人、业务对象
  glossary.md                # 术语及禁止混用的词
  specs/
    <domain>/
      spec.md                # 已认可功能规格，含 REQ 编号
      contract.openapi.yaml  # 有 API 时保存接口契约
  decisions/
    ADR-0001-<topic>.md
  changes/
    <yyyy-mm-dd>-<change>/
      proposal.md
      requirements.md
      design.md
      tasks.md
      unknowns.md
      evidence.md
    archive/
  handover/
    current.md
    <yyyy-mm-dd>-<milestone>.md
```

若项目已采用 Spec Kit，则保留 `.specify` 与 `specs` 结构，不需要改名为 `openspec`；仅需要把同等信息加入其模板和变更资料。

### 4. 当前规格的写法

每条需求必须可被引用、可被检验，并写明依据：

```markdown
## REQ-AUTH-003 外部登录失败的提示

- 状态：Approved
- 负责人：<name-or-role>
- 来源：ADR-0012；PR #123；验收记录 2026-05-20
- 适用范围：Web 登录页
- 不适用范围：管理员后台登录

### Requirement

WHEN 外部身份提供方返回授权拒绝，
THE SYSTEM SHALL 显示用户可理解的取消提示，
AND SHALL NOT 创建新的登录 session。

### Acceptance Scenarios

Scenario: 用户取消授权
  Given 用户未登录且打开外部登录流程
  When 身份提供方返回 access_denied
  Then 页面显示“登录已取消”
  And 系统没有创建新的 session

### Verification

- 自动检查：`<test command or test id>`
- 人工检查：`<page/action/result>`
- 最近验证：未验证 / YYYY-MM-DD 通过 / YYYY-MM-DD 失败
```

这里采用 EARS 风格的 `WHEN ... THE SYSTEM SHALL ...` 以及 Gherkin 场景。模型可以协助起草，但 `Status: Approved` 只能在负责人确认或现有批准资料能够明确支持时写入。

### 5. 变更包的最低要求

一次会影响用户可见行为、数据、接口、权限或发布结果的修改，都需要变更包。小型文案修正可由项目自行约定简化规则。

`proposal.md` 必须包含：

```markdown
# Change: <标题>

## Why
为什么需要修改；引用问题、反馈或已批准目标。

## Affected Requirements
- Modified: REQ-...
- Added: REQ-...
- Removed: none / REQ-...

## Explicit Non-Goals
- 这次不会改变什么。

## Approval
- Business owner: Pending / Approved by ... at ...
- Technical reviewer: Pending / Approved by ... at ...
```

`unknowns.md` 必须包含：

```markdown
| ID | 问题 | 影响 | 当前证据 | 决定者 | 状态 |
| --- | --- | --- | --- | --- | --- |
| U-001 | ... | Blocker / High / Low | ... | ... | Open / Resolved |
```

`evidence.md` 必须包含：

```markdown
| REQ ID | 检查方法 | 结果 | 日期 | 执行者或 Agent | 证据位置 |
| --- | --- | --- | --- | --- | --- |
| REQ-... | automated / manual / contract | Pass / Fail / Not run | ... | ... | ... |
```

若 `Blocker` 或 `High` 未知项仍是 `Open`，变更不可被描述为业务验收完成。

## 无人宣讲时的接手流程

这个流程不要求原负责人召开说明会，但会把必要确认变成可审阅资料。

### 阶段 A：获取事实，不实施业务改动

接手人及其 AI 读取：

1. `AGENTS.md`、README、构建与测试入口；
2. 当前规格、变更包、决策记录、交接记录；
3. 发布和验收记录；
4. API/schema/migration 与相关测试；
5. 最后才使用聊天记录、历史 issue 或个人 memory 作为背景。

输出一份接手报告：

| 输出 | 内容 |
| --- | --- |
| 功能地图 | 功能域、入口、主要数据、外部依赖 |
| 规格覆盖表 | 每个重要行为是否有批准规格、验收证据 |
| 不一致列表 | 规格、代码、测试、线上记录之间的差异 |
| 未知项列表 | 资料不足、无法判断产品意图的问题 |
| 建议读取路径 | 下一次 Agent 需要读取的文件和命令 |

### 阶段 B：恢复当前基准

没有规格的旧项目，不要一次性让 AI 为全仓库生成大量“需求”。优先选取：

- 正在迭代的功能；
- 权限、付费、数据删除、外部同步等风险较高的功能；
- 用户投诉或回归频率高的功能；
- 跨项目复用的接口。

对每个选中的功能：

1. 根据可观察行为编写 `OBSERVED` 草案；
2. 收集已有测试、接口和发布检查记录；
3. 列出无法由实现证明的 `INTENDED` 问题；
4. 由负责角色异步批准、修订或明确标为暂不确认；
5. 将批准后的内容标记为当前规格。

### 阶段 C：开始正常变更

只有当前任务涉及的高影响行为拥有基准，才进入 proposal、实现与验收。这样既不会因补全全部历史而停滞，也不会让新修改继续扩大资料缺口。

## 日常开发流程

### 一次需求变更

| 步骤 | 人与 AI 的动作 | 必须得到的资料 |
| --- | --- | --- |
| 需求提出 | 人说明目标，AI 查找相关规格和代码现状 | `proposal.md` 草案、受影响 REQ |
| 歧义处理 | AI 列 `unknowns`，责任人处理高影响问题 | 已更新 `unknowns.md` 和批准记录 |
| 技术设计 | AI/工程师写实现方案、数据和兼容影响 | `design.md`、必要 ADR |
| 任务执行 | AI 按已批准范围修改代码和测试 | `tasks.md` 状态、代码 diff |
| 需求验证 | 执行自动检查、契约检查和必要人工操作 | `evidence.md` |
| 审阅合并 | CODEOWNER/负责人审阅规格和行为修改，CI 必须通过 | PR 审阅和状态检查 |
| 归档交接 | 当前规格更新，记录当前限制和后续问题 | archive 与 `handover/current.md` |

### 合并要求

团队可以在 GitHub 或相同能力的平台配置以下要求：

| 修改内容 | 必需审阅 | 必需检查 |
| --- | --- | --- |
| `openspec/specs/**` 或产品要求文件 | 产品/领域 CODEOWNER | 规格格式、REQ 引用、未知项状态 |
| API/schema/data migration | 领域负责人 + 技术 reviewer | 单元/集成测试、OpenAPI/Pact 或迁移验证 |
| 权限、计费、数据删除、备份恢复 | 两位明确 reviewer | 自动检查 + 人工验收证据 |
| `AGENTS.md` / Team Skill | 工程负责人 | 指令安全检查、相关项目试用 |

GitHub 官方 ruleset/branch protection 已能要求 PR 审阅、CODEOWNERS 审阅和状态检查。若仓库允许 AI 直接向主分支写入，上面的资料就只是建议，无法成为团队制度。

## 如何控制模型解释差异

### 1. 要求结构化读取结果

每位 AI 接手时都输出同一 JSON 或表格结构，而不是自由摘要：

```json
{
  "project": "<name>",
  "revision": "<git-commit>",
  "requirements": [
    {
      "id": "REQ-...",
      "statement": "<verbatim-or-short-paraphrase>",
      "status": "approved|observed|unknown|conflict",
      "sources": ["<path>#<section>"],
      "acceptance": ["<scenario-or-check>"],
      "unresolved_questions": []
    }
  ]
}
```

比较两个 Agent 时，比较的是 `status`、`sources`、受影响 REQ 和待确认问题，不比较写作风格。

### 2. 双模型只用于发现问题，不用于多数表决

对于支付、权限、同步、删除、隐私或上线前关键功能，可让两个不同模型分别执行规格审阅：

- 两者都必须引用资料路径；
- 任一模型报告 `conflict` 或高影响 `unknown` 时进入人工决定；
- 两者答案相同，只说明解释一致，不证明业务决定正确；
- 禁止用“两个模型都认为应该如此”替代责任人批准。

### 3. 让机器验证可机器验证的要求

| 要求类型 | 推荐形式 |
| --- | --- |
| REST API 请求响应、错误码、字段兼容 | OpenAPI + contract/integration test |
| 前端用户流程 | Gherkin 场景 + E2E 或明确人工验收步骤 |
| 权限和数据隔离 | 授权测试 + 负向测试 |
| 数据迁移、导入导出、备份恢复 | 固定样本 + round-trip 验证记录 |
| 技术选择原因与舍弃方案 | ADR |
| 业务策略、优先级、不可观察规则 | 负责人批准的规格 |

模型最容易造成问题的部分往往正是不能仅用代码检查的业务判断，因此这部分要保留清晰的批准责任。

## 团队角色与责任

AI 全程参与开发，并不意味着可以移除责任分工。角色可以很轻量，也可以一人承担多个角色，但不能缺失。

| 角色 | 最少职责 |
| --- | --- |
| Domain owner | 批准或拒绝业务意图、非目标和高影响未知项 |
| Maintainer | 维护代码、规格位置、测试入口和交接资料 |
| Reviewer | 检查此次改动是否覆盖受影响要求及验证结果 |
| Agent | 查找资料、生成草案、实现已批准任务、执行检查、报告不一致 |
| Tool/library owner | 维护团队模板、Skill、CI 检查与 Vola 索引 |

可接受的低会议制度是：责任人不做口头宣讲，但必须审阅规格变更 PR 或在变更包中留下决定记录。

## Vola 在这套机制中的位置

### 目前可以使用的部分

Vola 当前 Team Library 适合保存跨项目通用内容：

```text
/team/playbooks/requirements/README.md
/team/playbooks/requirements/project-intake.md
/team/playbooks/requirements/spec-review.md
/team/prompts/requirements/structured-read.json.md
/skills/project-intake/SKILL.md
/skills/spec-evidence-review/SKILL.md
```

用途：

- 让 Claude Code、Codex 等可读取相同的接手步骤；
- 保存需求模板、审阅 checklist、结构化输出 schema；
- 通过备份与 MCP 访问让团队成员能取用流程材料；
- 将跨项目索引或阶段性接手摘要作为查找入口。

### 不应现在承诺的部分

根据现有文档，Team Library 当前不等同于：

- 项目规格的审批系统；
- 组织级审计记录；
- PR 必需审阅与 CI 阻断；
- 完整的多人项目管理页面；
- 所有 Agent 本地配置自动安装。

因此，项目的正式 `specs`、`changes`、`decisions`、`evidence` 应存在项目 Git 仓库；Vola 负责传播方法、检索资料和保存团队共享模板，而不是替代仓库历史与代码审阅。

### 未来值得增加的产品能力

如果 Vola 后续要服务这种协作场景，优先考虑以下能力：

| 能力 | 价值 | 风险限制 |
| --- | --- | --- |
| Project Contract 模板库 | 分发规格、接手、验收模板 | 模板不是批准结果 |
| 项目索引与交接看板 | 展示项目负责人、规格位置、最后验证时间、未解决项数量 | 需要从仓库同步真实状态 |
| Spec review Skill 与结构化输出 schema | 各 Agent 以相同格式报告不一致 | 不自动改变规格 |
| PR/CI 状态引用 | 在 Hub 里查看验收是否完成 | 权限和来源必须清楚 |
| proposal 来源与模型记录 | 看出哪项建议由何种材料和模型生成 | 未批准 proposal 不进入当前规格 |

这个方向与现有“资料、Skill、来源、proposal 需要审查”的产品研究相容，但属于后续产品工作，不能当成本次已经具备的功能。

## 试行计划

### 第 1 至 2 周：选项目与制定规则

选择两个项目：

- 一个仍在频繁修改、由原维护者可异步确认的项目；
- 一个即将由另一名成员接手、历史文档不足的项目。

完成事项：

1. 确定 domain owner 与 maintainer。
2. 决定每个项目使用现有 Spec Kit 还是 OpenSpec 风格目录；同一项目不要同时运行两套普通变更流程。
3. 增加 `AGENTS.md` 中的规格读取和验证说明。
4. 创建项目说明、术语表、接手报告和首批高影响规格。
5. 配置 PR 审阅和 CI 必需检查。

### 第 3 至 6 周：用真实变更检验制度

每个试行项目至少处理三次真实变更，要求：

- 有受影响 REQ；
- 有明确非目标；
- 有 unknowns 处理记录；
- 有 evidence 记录；
- 有一次由不同 Agent 读取同一变更包并比较输出。

观察指标：

| 指标 | 目标 |
| --- | --- |
| 新变更中包含受影响 REQ 的比例 | 100%（纳入规则的变更） |
| 高影响未知项在合并时仍处于 Open 的数量 | 0 |
| 需求变化同时更新验收证据的比例 | 100% |
| 新接手者定位关键功能依据所需时间 | 比试行前下降 |
| 双 Agent 审阅发现的冲突 | 被记录并处理，而不是被忽略 |

### 第 7 至 12 周：选择团队默认方式

根据两项目试行结果决定：

| 观察结果 | 决定 |
| --- | --- |
| OpenSpec 资料足够、成员维护负担可接受 | 将其作为已有项目默认变更目录 |
| 重大需求的歧义仍较多，需要更完整前置资料 | 对高影响能力采用 Spec Kit 流程 |
| 文件持续未维护、PR 仍绕过审阅 | 暂停扩大范围，先修订权限和责任安排 |
| Vola 模板与 Skill 被频繁使用 | 将模板库和结构化审阅 Skill 纳入正式迭代计划 |

## Spec Kit 是否继续使用的明确回答

可以继续使用。它仍在活跃发布，也具备跨 Agent 接入和需求质量检查步骤。

但不能把“安装 Spec Kit”当成保证需求不偏离的答案。你们当前缺少的不是另一个生成文档的命令，而是以下团队规则：

1. 业务规格与代码一起版本化；
2. 已有行为、期望行为和未知项明确区分；
3. 变更必须引用受影响需求和验收证据；
4. 合并必须经过相应责任人和自动检查；
5. AI 使用统一读取与报告格式，不自行批准业务解释。

如果以前的 Spec Kit 资料仍在项目中，应保留并完善；如果大多数项目已经没有有效资料，建议用 OpenSpec 风格在两个已有项目中试行当前规格与变更包制度，并在重大新能力上继续保留 Spec Kit 作为更完整的工作流选择。

## 给团队的制度草案

可以直接作为内部规则评审的第一版：

```markdown
# AI 开发项目需求连续性规则 v0.1

1. 每个在维护项目必须声明负责人、规格目录、验证命令和交接文件位置。
2. AI 会话、memory、聊天记录和个人 Skill 不是正式业务需求来源。
3. 影响用户行为、数据、接口、权限或发布的改动必须包含变更资料：
   - 目的与非目标；
   - 受影响 REQ；
   - 未知项；
   - 验收证据。
4. 没有批准规格的旧功能可以记录为 OBSERVED；没有责任人确认，不得写成 APPROVED INTENDED。
5. 高影响未知项未处理时，不合并会改变相关业务行为的修改。
6. 所有 Agent 接手报告必须引用文件路径与版本，并区分 approved、observed、unknown、conflict。
7. 产品要求变化由 domain owner 审阅；自动测试通过不能替代业务批准。
8. AGENTS.md 与 Skill 规定工作步骤；正式规格和证据保存在项目仓库。
9. 团队 Hub 可以分发模板、流程与索引；不得被宣传为未经实现的审批或审计系统。
10. 每月抽查一次规格与实现是否仍一致，并记录未验证内容。
```

## 参考资料

### 官方工具与产品资料

- GitHub Spec Kit repository and README: <https://github.com/github/spec-kit>
- GitHub Spec Kit latest release checked on 2026-05-25: <https://github.com/github/spec-kit/releases/tag/v0.8.13>
- GitHub Spec Kit documentation: <https://github.github.com/spec-kit/>
- OpenSpec repository and README: <https://github.com/Fission-AI/OpenSpec>
- OpenSpec latest release checked on 2026-05-25: <https://github.com/Fission-AI/OpenSpec/releases/tag/v1.3.1>
- Kiro Specs documentation: <https://kiro.dev/docs/specs/>
- AGENTS.md open format: <https://agents.md/>
- Agent Skills Specification: <https://agentskills.io/specification>

### 需求、契约与治理资料

- ISO/IEC/IEEE 29148:2018 Requirements Engineering: <https://www.iso.org/standard/72089.html>
- Mavin et al., *Easy Approach to Requirements Syntax (EARS)*: <https://alistairmavin.com/ears/>
- Cucumber Gherkin Reference: <https://cucumber.io/docs/gherkin/reference/>
- OpenAPI Specification: <https://spec.openapis.org/oas/latest.html>
- Pact documentation, consumer-driven contracts: <https://docs.pact.io/>
- GitHub documentation, protected branches and required reviews/checks: <https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches>

### 研究与行业观察

- Thoughtworks Technology Radar, Volume 34, April 2026, *spec-driven development* (`Assess`): <https://www.thoughtworks.com/radar/techniques/spec-driven-development>
- Adibhatla et al., *Spec Kit Agents: Goal-Directed Agents for Structured, Traceable Specification-Driven Development*, arXiv, 2026: <https://arxiv.org/abs/2604.05278>
- Heiss et al., *A Context-Grounded Approach to Specification-Driven Development*, accepted at ASE 2026 NIER: <https://arxiv.org/abs/2602.00180>

### 本仓库已读取资料

- `README.zh-CN.md`
- `docs/team-ai-library.zh-CN.md`
- `docs/agent-data-hub-iteration-plan.zh-CN.md`
- `docs/model-provider-learning-engine-research.zh-CN.md`
- `docs/launch-test-checklist.zh-CN.md`
- `docs/reference.zh-CN.md`

## 已验证与未验证

已验证：

- 通过官方 GitHub release API 核实了 Spec Kit `v0.8.13`（2026-05-21）与 OpenSpec `v1.3.1`（2026-04-21）的最新 release 信息。
- 阅读了 Spec Kit、OpenSpec、Kiro Specs、AGENTS.md 与 Agent Skills 的官方公开说明。
- 阅读了 Vola 与 Team Library、Projects、学习 proposal 方向有关的现有文档和前端路径线索。

未验证：

- 未在 Vola 仓库安装或运行 Spec Kit、OpenSpec 或 Kiro。
- 未创建 CI、CODEOWNERS、Team Library 内容或新的产品功能。
- 未用两种实际 Agent 对同一项目执行接手评测；该项属于试行阶段工作。
- 未证明 Vola 当前已经能管理项目规格审批或组织审计；现有文档明确没有这项承诺。
