# Codex Console 产品判断与推进记录

更新日期：2026-06-08

本文记录 Vola 在 Codex Console、个人 Agent 数据层、记忆同步和 hook 风险资产方向上的产品判断。相关开源项目调研见 `docs/codex-console-open-source-research.zh-CN.md`，当前差距审计见 `docs/codex-console-goal-gap-audit.zh-CN.md`。

## 当前定位

Vola 不适合追 Dify、OpenHands、Cline 这类执行型 Agent 平台，也不适合只做 Awareness-Local 那种 memory MCP。更清楚的定位是：

> Vola 是个人 Agent 数据层。Codex、Claude、ChatGPT、Cursor 等工具可以把记忆、项目、技能、授权、执行记录和交付物接到同一个可备份、可迁移、可审计的位置。

Codex Console 是这个定位下的本地工作记录模块。它读取 Codex 已经生成的本地资料，把线程、目标、自动化、执行时间线、交付物、记忆候选和 hook 风险资产整理给用户看。它不替代 Codex Desktop，也不默认从 Vola 发起桌面或浏览器操作。

## 本次对话脉络

本轮讨论从“Vola 是否要补 Codex Desktop 方向”开始。经过开源项目调研和本地代码查看后，形成了几个产品判断：

- Vola 不追 Dify、OpenHands、Cline 这类执行型 Agent 平台。
- Vola 也不只做 Awareness-Local 这类 memory MCP。
- Vola 更适合成为个人 Agent 数据层，服务 Codex、Claude、ChatGPT、Cursor 等工具。
- Codex Console 是这个数据层里的本地工作记录视图，而不是新的 Agent 执行入口。

随后把 Codex Console 拆成几个对象：Threads、Goals、Automations、Runs、Artifacts、Memory Sync、Hooks。讨论中优先确认了 Memory Sync 的边界：Codex 和 Chronicle 记忆先作为候选展示，用户选择后再写入 Vola memory，不自动把线程总结或原始大文件写进长期记忆。

在“记忆方案、hook 方案的完成度还有多少，是否还有可以吸收的点”这个问题上，结论是：Memory Sync 已经具备从候选写入 Vola profile memory 的基础流程；随后补上了 review 状态，支持接受、忽略、延后和已同步。推进后，Memory Sync 已支持写入 project context，并能在 profile 目标上提示 possible conflict。现在普通候选支持写入前编辑，profile conflict 支持保留已有、采用候选、两者共存和手工合并；Codex Console 也已按项目生成 Handover Summary，Handover 已能保存为可编辑项目文件，并能从成功 Codex 线程生成候选 Skill 草稿再保存到 Hub 候选区。Skill 草稿已支持编辑后保存，并能加入 Codex、Claude Code、Cursor、Gemini CLI 分配表生成同步或导出预览。people 目标、真实本地同步验证和 ready / archived 状态仍未实现。hook 方向目前只应做 inventory 和风险审查，不自动启用。

本轮实现沿着这个判断继续推进：把 Artifacts 和 Hooks 加入 Codex Console，把 hook 作为 `manual_required` 风险资产展示；继续推进后，又把 Memory Review 状态写入 Hub 文件树，让候选记忆有可备份、可迁移的 review 记录。

2026-06-08 继续查看团队 Skill 共享文章《我折腾了好久的Skills团队共享，终于有产品替我做出来了。》。文章的核心不是执行 Agent，而是团队 Skills / Agent 如何低成本共享、安装、更新和授权。Vola 已吸收其中适合数据层的部分：团队 Skills 页现在能显示未安装、团队有新版、已安装、个人副本较新，并能安装到个人空间或更新个人副本；团队资料库新增 `/team/agents`，用于保存团队 Agent 配方。Vola 仍不直接运行团队 Agent，也不自动启用 hook、plugin 或 MCP。

## 事实边界

| 内容 | 状态 | 来源 | 验证 |
| --- | --- | --- | --- |
| “Vola 是个人 Agent 数据层” | APPROVED | 本次产品讨论 | 已写入本文 |
| Codex Console 的对象包括 Threads、Goals、Automations、Runs、Artifacts、Memory Sync、Hooks | APPROVED | 本次产品讨论 | 后端响应和前端 tab 已覆盖 |
| Codex / Chronicle 记忆先作为候选，再由用户同步到 Vola memory | APPROVED | 本次产品讨论 | API 测试和页面验证已覆盖 |
| Hook 只展示风险资产，不执行、不安装、不改配置 | APPROVED | 本次产品讨论 | 测试 fixture 覆盖 `manual_required` |
| 候选 Skill 可编辑、保存为 Hub 草稿、加入多 Agent 分配预览，但不自动安装、不写入本地 Agent skills | APPROVED | 本次产品讨论 | API 测试、前端构建和桌面选择器点击已覆盖；真实分配写入未在本机 UI 执行 |
| 团队 Skill 共享应做安装、更新、权限和备份，不做完整执行平台 | APPROVED | 微信文章调研与本次实现 | 已加入团队 Skill 安装 / 更新状态和 `/team/agents` 配方目录 |
| 原始 X 帖正文 | UNKNOWN | 公开网页读取没有返回正文 | 调研报告只把用户摘录作为线索 |
| 未来是否从 Vola 发起 Codex task | UNKNOWN | 当前没有产品批准 | 暂不实现，只保留 task draft 方向 |

## 调研结论

开源项目大致分成五类：

- 本地记忆和共享上下文：Awareness-Local、AI Workspace、cortex-hub、Dory、Origin、mem0、Graphiti、Zep。
- MCP 网关、工具注册、权限和审计：MCP Gateway Registry、IBM mcp-context-forge、ToolHive、Gate22。
- 浏览器、桌面和网页会话控制：Agentify Desktop、QuickDesk、Windows-MCP。
- Agent 工作台和运行 dashboard：Mission Control、Hermes Studio、Accomplish、OpenHands、Cline、Roo Code、Archon。
- 多 Agent 配置、Skill 和规则分发：gaal、skill-depot、Arcweld。

这些项目证明了几个趋势：

- 记忆要由用户拥有，Markdown、本地文件和可迁移存储会越来越重要。
- MCP 管理会变成基础设施能力，仅靠 MCP Hub 很难形成差异。
- 执行型 Agent 工作台已经拥挤，Vola 应服务这些工具，而不是变成它们的替代品。
- Skills、Connectors、hooks 会成为工作方式的包装，但自动启用会带来权限和副作用风险。
- 团队协作场景会需要 Skills / Agent 的共享、安装状态、更新提醒和角色权限；这些更适合成为 Vola 的资产层能力。

因此，Vola 的重点应该是：数据所有权、来源记录、备份、迁移、权限、审计和跨 Agent 复用。

## 当前完成度判断

面向内部演示和小范围 self-hosted 试用，Vola 作为个人 Agent 数据层大约完成 78%。

分模块看：

| 模块 | 完成度 | 说明 |
| --- | ---: | --- |
| Agent 数据层叙事 | 80% | README、页面和功能已经覆盖 profile、memory、projects、skills、vault、connections、备份。 |
| Codex 本地导入 | 85% | 已能扫描 Codex config、AGENTS.md、rules、memories、Chronicle、sessions、automations、skills、plugins、auth 敏感项。 |
| Codex Console Lite | 94% | 已有 Threads、Runs、Automations、Artifacts、Artifact Registry、Hooks、Memory 候选、Handover Summary、Handover 保存、候选 Skill 草稿、多 Agent 分配预览和工作区概览。 |
| Memory Sync | 92% | 已能从 Codex Console 写入 `/memory/profile` 和 project context，保存 accepted、ignored、deferred、synced 状态，支持普通候选写入前编辑和 profile conflict 手工合并；还缺 people 目标。 |
| Run / Artifact 审计 | 78% | 已能从 session JSONL 识别工具调用、浏览器/电脑线索、错误和 artifact 引用，能保存带用途说明的 artifact registry，并支持项目和用途过滤；还缺预览、打开文件、最终交付标记和更细事件时间线。 |
| Hook 方案 | 50% | 已能把 Skill bundle 里的 hook 文件作为风险资产展示，并显示 shebang、环境变量、风险信号、写入路径提示和风险等级；仍不自动启用，也没有执行日志。 |
| Skill Candidate | 82% | 已能从成功 Codex run 生成 `SKILL.md` 候选草稿，支持页面编辑、保存到 `/skills/_candidates/<slug>/`、写入来源 metadata 和 `manifest.vola.json`，并加入 Codex、Claude Code、Cursor、Gemini CLI 分配表生成同步或导出预览；还缺真实本地同步验证和 ready / archived 状态。 |
| 受控操作入口 | 15% | 目前不从 Vola 发起 Codex task，只保留未来生成 task draft 的方向。 |

## 已实现内容

截至 2026-06-07，Codex Console 已覆盖：

- Threads：从 `.codex/sessions/**/*.jsonl` 和 session index 中整理线程、项目、时间、消息数、工具事件。
- Goals：从用户消息中识别 `/goal`、`目标:`、`objective:` 等目标线索。
- Runs：统计 tool call、tool result、browser/computer 线索、approval、error，并展示部分事件。
- Automations：读取 `.codex/automations/**/automation.toml`，展示 kind、status、schedule、prompt 和来源路径。
- Artifacts：从 attachment 和 tool result 中提取 HTML、Markdown、图片、PDF、PPT、文档、表格、JSON 等文件引用。
- Artifact Registry：可把 Console 中识别到的交付物保存到 `/platforms/codex/console/artifacts.json`，记录项目、线程、来源路径、用途说明、给下一个 Agent 的提示和保存时间；这是 Vola 资产化的初版，不等同于文件预览或最终交付判断。
- Memory Candidates：展示 Codex memory 和 Chronicle memory 候选。
- Memory Sync：支持同步选中候选或全部候选到 `/memory/profile`，也支持写入 project context。
- Memory Review：支持把候选标记为 accepted、ignored、deferred、synced，状态文件保存到 `/platforms/codex/console/memory-review.json`。
- Conflict Hint / Resolve：同一 profile category 已有不同来源内容时，候选会显示 `possible` 冲突提示，并提供保留已有、采用候选、两者共存和手工合并。
- Hooks：从 Codex skill bundle 文件树中识别 `hooks/` 下的文件，作为 `manual_required` 风险资产展示，并提供基础风险详情。
- Handover Summary：按 project 聚合 threads、runs、artifacts、memory candidates，展示计数、最近活动、摘要和关键条目；专业详情区可编辑并保存为 `/projects/<project>/handover.md`。
- Skill Candidates：从无错误且有工具调用的 Codex run 生成候选 Skill，页面展示名称、项目、置信度、工具调用、交付物、信号、来源路径和 `SKILL.md` 草稿；专业详情区可编辑并保存为 Vola Skill 草稿，写入 `/skills/_candidates/<slug>/SKILL.md`、`candidate.vola.json` 和 `manifest.vola.json`，也可加入 Codex、Claude Code、Cursor、Gemini CLI 分配表生成同步或导出预览，但不自动安装。

Memory Sync 的规则：

- 普通 Codex memory 写入 `codex-*` profile category。
- Chronicle memory 写入 `codex-chronicle-*` profile category。
- Project target 写入 `/projects/<project>/context.md`，会追加带 marker 的 `Codex memory` 段落；重复同步同一候选会跳过。
- Profile target 如果遇到同一 category 已有不同来源内容，会在候选上提示 possible conflict；普通 sync 会跳过该候选，必须通过 conflict resolve 处理。
- Conflict resolve 支持保留已有、采用候选、两者共存和手工合并；两者共存会写入 `原分类-codex` 这类独立 profile category。
- 单条候选写入前可以编辑正文；编辑只影响这次同步，不改 Codex 原始记录。
- 内容为空会跳过。
- 超过 64KB 的候选会跳过，避免把过大的线程或原始 memory 直接写入 profile。
- 写入来源标记为 `agent:codex`，request source 使用 `codex-console`。
- 同步成功后，对应候选会标记为 `synced`，并记录写入的 profile memory path。

Hook 规则：

- Vola 只展示 hook 文件，不执行，不安装，不修改 Agent 全局配置。
- Hook 状态显示为 `manual_required`。
- 页面展示 hook kind、所属 bundle、来源路径、文件摘要、shebang、环境变量、风险信号、写入路径提示和风险等级。
- 后续如果要支持启用，也应先生成安装说明或 task draft，由用户明确执行。

## 吸收进来的方案

### 来自记忆类项目

已经吸收：

- 本地文件优先。
- Markdown 记忆可以作为可迁移资产。
- 记忆写入需要来源和时间。
- Chronicle 这类自动生成的记忆必须先作为候选展示。
- Memory Review 队列：accepted、ignored、deferred、synced。
- 记忆目标类型：profile、project。
- Profile possible conflict 提示。
- Profile conflict resolve：保留已有、采用候选、两者共存、合并。

还值得继续吸收：

- 记忆目标类型：people、skill candidate。
- 自定义合并编辑器：让用户手动编辑最终内容。
- SQLite FTS 搜索：先做关键词搜索，再考虑 embedding。
- 记忆来源时间线：展示来自哪个 thread、Chronicle 文件或导入动作。

### 来自执行型 Agent dashboard

已经吸收：

- Run 时间线。
- 工具调用统计。
- 浏览器、电脑操作、approval、error 的可见化线索。
- Artifact 引用回到 thread 和 project。
- 按项目生成 handover summary。

还值得继续吸收：

- 更完整的 timeline 展开。
- 失败事件和审批结果筛选。
- 候选 Skill 的人工编辑、保存为 Hub Skill、分配给 Agent 和本地同步验证。

### 来自 hook / skill 方案

已经吸收：

- hook 是 Skill 资产的一部分。
- hook 有权限、副作用和触发时机风险。
- hook 不应自动启用。
- hook、MCP、plugin 配置应进入转换和导入报告。
- Hook Review 基础版：展示 shebang、环境变量、风险等级、远程脚本管道、破坏性删除、网络拉取、权限变更和写入路径提示。

还值得继续吸收：

- Hook Inventory：所有 hook 文件集中展示。
- Hook Review 深化：展示触发时机、结构化命令、读取路径和更细的副作用说明。
- Hook Timeline：未来有执行日志后，把结果挂到 Run。
- Hook Templates：从成功流程生成候选 hook 或 preflight。

## 当前不做的事

- 不把 Vola 做成 OpenHands、Cline、Roo Code 的替代品。
- 不宣传“直接控制 Codex Desktop”。
- 不自动把线程总结写入长期 memory。
- 不自动启用 plugin、hook、MCP server。
- 不读取或保存 `auth.json` 里的 token 明文。
- 不把 raw memory 或大体积线程直接塞进 profile memory。

## 下一步建议

近期优先级：

1. People 目标：等仓库里有稳定模型后再写入，不临时造路径。
2. 自定义合并编辑器：profile conflict 目前支持自动合并，还不能手工编辑最终内容。
3. Artifact Registry 深化：已有索引保存；下一步做按项目过滤、预览、打开文件、复制路径和最终交付标记。
4. Hook Review 深化：识别触发时机、结构化命令、读取路径和更细的副作用说明。
5. 候选 Skill 编辑与分配：已能保存为 Hub 草稿，下一步是页面编辑、分配给 Agent、本地同步预览和验证。

中期方向：

1. 候选 Skill 进入 Hub Skill 后，继续走 Agent 分配、本地同步和转换报告。
2. Run timeline 增加更完整事件展开。
3. Codex task draft：在 Vola 中选择 workspace、goal、skill、memory scope，生成可复制到 Codex 的任务草稿。
4. Memory 搜索和冲突处理。
5. Console 数据导出到 `/platforms/codex/console/`，用于备份和跨机器迁移。

## 验证记录

2026-06-06 已验证：

- `go test ./internal/platforms ./internal/api`
- `cd web && npm run build`
- Playwright 打开 `http://127.0.0.1:3001/codex-console`，Memory tab 同步选中候选后页面显示“已同步 1 条到长期记忆”，Console 0 errors。
- Memory Review 继续推进后，`GET /api/local/codex-console` 可返回 `memory_review_required`、`memory_accepted`、`memory_ignored`、`memory_deferred`、`memory_synced` 统计和候选 `review_status`。
- `POST /api/local/codex-console/memory-review` 已验证路由和候选 ID 校验；测试覆盖 ignored 状态持久化和 sync 后自动标记 `synced`。
- `POST /api/local/codex-console/memory-sync` 已验证 `target:"project"`，会写入 project context，并对重复候选跳过。
- `GET /api/local/codex-console` 已补测试覆盖 profile possible conflict 提示。
- Hook Review 基础详情已补测试覆盖：风险等级、shebang、环境变量、风险信号和写入路径提示。
- Hook Review 继续推进后，`go test ./internal/platforms ./internal/api`、`npm run build` 和 `git diff --check` 通过；Playwright 打开 Codex Console 页面时 Console 0 errors、2 个 React Router 开发环境 warning。
- Memory conflict resolve 已补测试覆盖：普通 sync 遇 profile conflict 会跳过；resolve 支持保留已有、采用候选、两者共存和合并。
- Handover Summary 已补测试覆盖：fixture 中的 `vola` 项目会生成 handover summary，包含 thread、run 和摘要内容。
- Skill Candidates 已补测试覆盖：fixture 中无错误且有工具调用的 run 会生成候选 Skill 草稿；前端 `Skill Candidates` tab 构建通过。
- Browser 打开 `http://127.0.0.1:3001/codex-console?local_token=...` 后，Skill Candidates tab 显示 23 条候选；详情区显示 `SKILL.md` 草稿、复制草稿和复制来源路径按钮；Console 0 errors。

`npm run build` 仍有 Vite chunk size warning，不影响构建结果。

2026-06-07 桌面端验证与修正：

- 发现此前主要验证的是 Web 页面，不能代表 Vola Desktop。真实桌面包加载的是 `tauri://localhost` 下的静态前端，并由 Go sidecar 提供本地 API。
- `desktop/scripts/build-backend-sidecar.mjs` 已改为直接构建到 `desktop/sidecars/vola-<target-triple>`，避免覆盖仓库根目录已有的 `bin/vola`。
- `desktop/src/lib.rs` 已新增 `get_api_base` Tauri command，由桌面壳把 sidecar 的 API 根地址传给前端。
- `desktop/tauri.conf.json` 已开启 `withGlobalTauri`，让前端可以在 Tauri WebView 中调用桌面 command。
- 前端 `web/src/api.ts` 已能识别 `tauri:` runtime，并在请求前等待桌面返回 API 地址。
- 桌面端初次仍出现 `The string did not match the expected pattern.`，原因是桌面壳返回 `http://127.0.0.1:<port>`，前端实际需要的是 `http://127.0.0.1:<port>/api`；已改为返回带 `/api` 的根地址。
- 使用新构建的 `desktop/target/release/bundle/macos/Vola.app` 验证：桌面首页不再出现黄色错误条，统计和文件树正常显示；侧边栏显示 `Codex Console`。
- 在同一个新桌面包内打开 `tauri://localhost/codex-console`，页面显示 285 threads、69 goals、259 runs、4515 artifacts、62 memory candidates、36 handovers、23 skill candidates。
- `cd desktop && npm run build` 已通过，产物包括 `desktop/target/release/bundle/macos/Vola.app` 和 `desktop/target/release/bundle/dmg/Vola_0.1.0_aarch64.dmg`。构建中仍有 Vite chunk size warning，不影响产物生成。
- release build 曾因 Tauri release 缓存残留旧项目路径 `/Users/zhongmoshu/Desktop/work/neuDrive/...` 失败；清理 `desktop/target/release/build` 中相关自动生成缓存后重建通过。

2026-06-07 看板含义调整：

- 用户反馈 Codex Console 不能只展示数量，也不应该默认铺开全量记录；普通用户看不懂 Threads、Runs、Artifacts 数字背后该做什么。
- 产品判断改为：Codex Console 默认展示“AI 使用改进”，即提示词可以怎么写得更清楚、哪些长期记忆能提升后续 AI 准确度、下一个 Agent 应该先看什么。
- 数字概览保留，但降级为专业查看入口；完整 Threads / Runs / Automations / Artifacts / Hooks 仍保留在后续标签里，用于追溯。
- 能自动处理的范围只放在 Memory：接受、延后、忽略、同步到 profile/project、处理 profile conflict。Hook、Skill 草稿、失败 Run 继续只做审查和打开详情，不自动启用或安装。
- Codex Console 返回给页面的摘要和预览已过滤 Unicode replacement character，避免 Codex session 中的坏字符在桌面端显示成乱码。
- 继续调整后，默认页不再以 threads、artifacts、runs 这类冷数字为主；首屏改为提示词建议、长期记忆价值说明、交接上下文和可复制给下次 AI 的提示词模板。
- 用户侧桌面截图发现长期记忆三列卡片会被长路径和正文挤乱。随后已改为单条价值摘要，提示词建议改为纵向列表，右侧专业入口改为紧凑列表，避免普通用户被大量数字和路径压住。
- 提示词建议继续从真实 Codex 数据里提取依据：最近记录里出现桌面端 / Tauri / Vola.app、验证 / build / screenshot、失败 run、交付物引用或记忆候选时，页面会显示对应建议和“依据”说明。
- runtime 检查脚本曾把 Codex Computer Use 的长命令行误判为 Vola 进程，因为命令行文本里包含 `Vola.app`。已改为使用 `ps -ax -o pid=,command=`，并要求命令行开头精确匹配新桌面 app executable 或 sidecar executable。

2026-06-07 Skill Candidate 保存为 Hub 草稿：

- 新增 `POST /api/local/codex-console/skill-candidates/save`，按 candidate ID 从当前 Codex Console 数据中找到候选 Skill。
- 保存目标为 `/skills/_candidates/<slug>/`，其中 `<slug>` 使用候选名称加短 ID，避免同名线程互相覆盖。
- 写入 `SKILL.md`、`candidate.vola.json` 和 `manifest.vola.json`。metadata 记录 source platform、thread、project、source path、tool calls、artifacts、confidence、signals 和 saved_at。
- 前端只在 Skill 草稿专业详情区提供“保存为 Vola Skill 草稿”，普通简报不增加新操作。
- 已验证 `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'` 通过。测试会读取保存后的三个 Hub 文件，并确认 Console 再次读取时能看到候选的 draft 状态和路径。
- 已执行 `./node_modules/.bin/tauri build --bundles app`，生成新的 `desktop/target/release/bundle/macos/Vola.app`。
- Computer Use 打开该新桌面包后，Codex Console 的 Skill 草稿详情显示“保存为 Vola Skill 草稿”按钮。为避免写入真实 Hub 测试数据，桌面 UI 未点击保存。

2026-06-07 Handover 保存为项目文件：

- 新增 `POST /api/local/codex-console/handovers/save`，按 handover ID 从当前 Codex Console 数据中找到项目交接摘要。
- 保存目标为 `/projects/<project>/handover.md`。
- 文件内容包含项目状态、最近 threads、runs、artifacts、memory candidates、审查注意事项和 `Manual notes` 区域。
- 重复保存会更新自动生成部分，并保留已有 `Manual notes` 手写备注。
- `GET /api/local/codex-console` 会回读已保存状态，返回 handover 的 `status`、`path` 和 `saved_at`。
- 前端只在项目交接专业详情区提供“保存为项目交接文件”按钮，普通简报不增加新操作。
- 已验证 `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'` 通过。测试会读取保存后的 `/projects/vola/handover.md`，并确认重复保存保留手写备注、Console 再次读取时能看到 saved 状态和路径。

2026-06-07 桌面端 `The string did not match the expected pattern.` 复盘：

- 这次不是 `get_api_base` 代码又退回旧值。`desktop/src/lib.rs` 当前仍返回带 `/api` 的 sidecar API 根地址。
- 实际发现同一台机器同时运行了两个 Vola：旧 `/Users/zhongmoshu/Desktop/work/Vola/src-tauri/.../vola.app` 和新 `/Users/zhongmoshu/Desktop/work/Vola/desktop/.../Vola.app`。旧 app 带旧 sidecar，容易让 macOS 切到旧窗口，看起来像已修过的问题回来了。
- 已结束旧 `src-tauri` 进程，只保留新 `desktop/target/release/bundle/macos/Vola.app`。
- 还发现桌面构建脚本存在资源同步风险：`desktop/scripts/build-backend-sidecar.mjs` 只构建 Go sidecar，没有把 `web/dist` 同步到 `internal/web/dist`；Go sidecar 内嵌的是 `internal/web/dist`。这会让 sidecar 可能带旧前端资源。
- 已把 `desktop/tauri.conf.json` 的 release 构建顺序改为：先 `web` build，再构建 sidecar。
- 已让 `desktop/scripts/build-backend-sidecar.mjs` 在 Go build 前把 `web/dist` 复制到 `internal/web/dist`。
- 已新增 `npm --prefix desktop run check:runtime -- --require-new`，用于验证桌面端只运行新 `desktop` app；如果旧 `src-tauri` app 还在运行，脚本会失败。
- 已重新构建 `desktop/target/release/bundle/macos/Vola.app`，并用 Computer Use 打开新包验证：首页没有该错误，Codex Console 正常加载，项目交接详情显示“保存为项目交接文件”按钮。

2026-06-07 Artifact Registry 保存为 Vola 资产：

- 新增 `POST /api/local/codex-console/artifacts/save`，把当前 Codex Console 识别到的交付物保存到 `/platforms/codex/console/artifacts.json`。
- 保存文件格式为 `vola.codex-artifact-registry/v1`，包含来源平台、保存时间、artifact_count、project_count、projects 和 artifacts 列表。
- `GET /api/local/codex-console` 会回读 artifact registry 状态，返回 `status`、`path`、`saved_at`、`artifact_count` 和 `project_count`。
- 前端只在交付物专业详情区显示“保存交付物索引 / 更新交付物索引”，普通简报不增加新操作。
- 为兼容 Codex JSONL 工具输出里的转义换行，artifact 路径识别已把 `\\n`、`\\r`、`\\t` 作为路径前缀处理；测试 fixture 覆盖 `docs/import-plan.md` 这种相对 Markdown 交付物。
- 已验证 `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'` 通过。测试会读取保存后的 registry 文件，并确认 Console 再次读取时能看到 saved 状态、路径和计数。

2026-06-07 默认页二次调整：

- 用户再次反馈：默认页如果主要展示数量、路径、原始记录，普通用户仍然不知道能做什么。
- 产品判断更新为：Codex Console 首页优先回答“下次 AI 怎么更准”，而不是“本机有多少条 Codex 数据”。
- 默认页现在按三类价值组织：
  - 下次提示词怎么写，减少测错端、漏验收和交付物说明不清。
  - 哪些长期记忆值得确认，以及为什么会提升后续 AI 准确度。
  - 下一个 Agent 应该先读项目交接、交付物索引还是 Skill 草稿。
- 首页不再展示工作区统计；工作区聚合只在专业视图出现。
- 长期记忆默认页会把路径和 Markdown 片段转成可读摘要，原始内容留在专业详情里。
- 右侧专业入口不再突出数字，改成“可查看 / 可接手 / 待审查 / 可追溯”等状态词。
- 已验证 `npm --prefix web run build` 通过；首次构建因沙箱不能写 Vola `web` 目录临时文件失败，使用外部权限重跑通过。

2026-06-07 桌面验证复盘补充：

- 仅用 `app: "Vola"` 做 Computer Use 验证不可靠；macOS 会按旧 bundle 记录拉起 `/Users/zhongmoshu/Desktop/work/Vola/src-tauri/.../vola.app`。
- 以后桌面验证必须指定完整路径：`/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`。
- `desktop/scripts/check-desktop-runtime.mjs` 已放宽旧进程识别，能拦住旧 `src-tauri` app 的 `Contents/MacOS/app` 和旧 sidecar。
- 最终桌面验证使用的是新 bundle id `cn.vola.desktop`，进程路径为 `/desktop/target/release/bundle/macos/Vola.app/Contents/MacOS/vola-desktop`。
- 新桌面包首页没有 `The string did not match the expected pattern.`。
- 新桌面包 Codex Console 能显示新版默认页：提示词建议、长期记忆价值摘要、下一个 Agent 接手材料、可复制提示词和状态化专业入口。
- 记忆候选摘要继续调整为“这条记录描述了什么 / 为什么有用”，不再在默认页直接露出 `memories/...`、`/Users/...` 或扩展说明原文。

2026-06-07 默认页布局和 Memory Sync 编辑：

- 用户继续反馈：页面仍像数据看板，右侧区域挤压主阅读区，普通用户不知道能做什么。
- 默认页布局改为单列阅读流：提示词建议、长期记忆价值、下一个 Agent 接手材料按顺序展示；可复制提示词、自动处理边界和专业资料入口放在下方，不再挤压主内容。
- “当前项目”显示为完整标签，例如 `当前项目：upnexis`，避免只出现孤立项目名。
- 长文本卡片增加换行和宽度保护，减少真实 Codex 记忆、项目名、路径摘要把卡片挤乱。
- Memory Sync 增加 `content_overrides`：专业用户可以在写入前编辑候选记忆文本，保存的是编辑后的内容，Codex 原始记录不变。
- 仍然不自动把候选记忆写入长期记忆。这个限制保留，因为错误长期记忆会影响后续 AI 判断。
- 验证结果：`go test ./internal/platforms ./internal/api`、`npm --prefix web run build`、`./node_modules/.bin/tauri build --bundles app`、`git diff --check` 均通过。
- 桌面验证使用新包完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`；`npm --prefix desktop run check:runtime -- --require-new` 显示只运行新桌面 app 和 sidecar。Computer Use 再次证明不能用应用名 `Vola`，它会拉起旧 `src-tauri` app。

2026-06-07 Memory conflict 手工合并：

- Profile memory 与 Codex 候选相似时，专业详情区现在会生成“合并后写入内容”草稿。
- 用户可以编辑最终文本后点击“合并并保存”；后端通过 `merged_content` 写入编辑后的 profile memory。
- 没有传 `merged_content` 的旧请求仍使用原自动合并格式，保持兼容。
- 这项能力只处理冲突候选，不改变 Codex 原始记录，也不自动写入普通候选。
- 已补 `TestSQLiteSharedServerLocalCodexConsoleMemoryConflictResolveEditedMerge`，确认编辑后的合并文本会写入 Vola profile memory，旧自动合并分区不会混入最终内容。

2026-06-07 Artifact 变成接手材料：

- 用户最初不想要冷数字或原始记录列表，因此交付物索引不能只保存 `artifacts.json`。
- 每个 Codex artifact 现在会生成 `role`、`handoff_note` 和 `agent_instruction`。
- `role` 会把文件分成交接文档、视觉证据、预览输出、结构化数据、执行证据、附件或项目文件。
- `handoff_note` 解释这个交付物应该怎么被使用；`agent_instruction` 可以直接复制给下一个 Agent。
- Artifact 详情页显示“给下一个 Agent 的用法”和可复制提示词，减少用户自己判断 `.md`、HTML、截图、JSON 到底该怎么交接。
- 保存到 `/platforms/codex/console/artifacts.json` 的 registry 也会包含这些交接字段，方便后续 Agent 不打开 UI 也能读取。

2026-06-07 Artifact 按项目和用途缩小范围：

- Artifact tab 增加搜索、项目过滤和用途过滤，避免用户在几千条交付物里查找。
- registry 增加 `project_summaries`，每个项目记录 artifact 数量、role 计数和最多 5 个优先交付物。
- 优先交付物按交接文档、预览输出、视觉证据、结构化数据、执行证据、附件、项目文件排序。
- 这让下一个 Agent 可以先读项目摘要，再决定是否打开完整 artifacts 列表。

2026-06-08 默认页从看板改成阅读流：

- 用户再次反馈：页面仍像数据看板，右侧区域和冷数据让普通用户不知道能做什么。
- 决策更新：Codex Console 首页不再同时展示专业详情栏；默认页只回答三件事：
  - 下次给 AI 的开场提示词怎么写。
  - 最近提示词哪里可以改进。
  - 哪些长期记忆和交接材料能让后续 AI 更准。
- 专业模块入口保留在页面底部，用“可查看 / 可接手 / 待审查 / 可追溯”等状态词，不再突出裸数字。
- 桌面端验证使用完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`。新窗口 URL 为 `tauri://localhost/codex-console`，未出现 `The string did not match the expected pattern.`。
- 构建复盘：当前本机 arm64 Node 会因为 hardened runtime 拒绝加载 ad-hoc 签名的 Rollup 原生包；本次用 x64 Node + `@rollup/rollup-darwin-x64` 完成 Vite build，再用 arm64 Tauri CLI 打 `.app`，避免前端构建和 Tauri CLI 的原生包架构互相冲突。

2026-06-08 项目交接可编辑：

- 项目交接不再只是自动摘要和保存按钮。专业详情区现在提供 Markdown 编辑框，默认内容来自已保存的 `handover.md`；未保存时由当前 Codex 线程、执行记录、交付物和记忆候选生成草稿。
- 保存接口新增 `content_override`。用户编辑后的内容会写入 `/projects/<project>/handover.md`，后端会保证保留 Codex handover marker，方便 Console 下次识别。
- 保存响应和 Console 详情会返回 FileTree `version`。Vola 的 FileTree 已保留版本记录，因此交接文件每次保存都有可追溯版本。
- 这保持了产品边界：普通首页只告诉用户“下一个 Agent 该先看交接材料”，专业用户才进入编辑和保存。

2026-06-08 Skill 草稿可编辑：

- Skill Candidate 不再只是生成 `SKILL.md` 后让用户复制或保存。专业详情区现在提供可编辑文本区，用户可以修改用途说明、步骤和注意事项后再保存到 Hub。
- 保存接口新增 `draft_override`。保存编辑版会写入 `/skills/_candidates/<slug>/SKILL.md`，同时更新 `candidate.vola.json`、`manifest.vola.json`，并标记 `edited`。
- Console 再次读取候选时会优先显示 Hub 中已保存的 `SKILL.md`，避免编辑后刷新又看到旧生成草稿。
- 这项能力仍然停在“整理成 Vola 资产”这一层：不会自动安装到 Codex、本地 Agent skills。后续阶段已经让已保存草稿进入 Vola 分配表并生成本地同步预览，但真正写入本地 Codex 仍需用户在 Skill 分配页执行。
- 已验证 `TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave`，覆盖编辑版写入、metadata `edited`、manifest 中 `SKILL.md` 哈希和 Console 回读编辑版草稿。
- 桌面端验证必须使用完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`。本次用应用名 `Vola` 仍会误拉起旧 `/src-tauri/.../vola.app`，旧包会出现 `The string did not match the expected pattern.`；这个现象不能作为新桌面包结果。

2026-06-08 Skill 草稿进入 Codex 分配预览：

- Skill Candidate 已从“保存为 Hub 草稿”推进到“进入 Agent 分配表并查看本地同步计划”。
- 新接口 `POST /api/local/codex-console/skill-candidates/assign-preview` 会把已保存候选加入 Vola 的 Agent 分配表，并返回 Codex 本地同步预览。
- 预览复用现有 Local Skill Sync 计划结构，能看到目标目录、新增、更新、冲突和变更文件。
- 该动作仍不写 `~/.codex/skills`，也不自动启用给 Codex。它只改变 Vola Hub 里的分配表；真正写本地目录仍由 Skill 分配页的本地同步操作完成。
- 前端在 Skill Candidate 详情页增加“Agent 分配预览”区域，按钮文案为“加入 Codex 分配并预览”，结果卡片明确说明这里只是预览。

2026-06-08 Skill 草稿进入多 Agent 分配预览：

- Skill Candidate 详情页新增 Agent 目标选择：Codex、Claude Code、Cursor、Gemini CLI。
- `assign-preview` 请求会携带所选 `agent_ids`；后端测试覆盖四个目标同时写入 Vola 分配表。
- 预览结果按 Agent 分组展示。Codex / Claude Code 显示本地同步计划；Cursor / Gemini CLI 显示导出包预览和不自动写入原因。
- 页面按钮已从“加入 Codex 分配并预览”改为“加入所选 Agent 分配并预览”。
- 该能力仍只改变 Hub 分配表并展示预览，不写任何本机 Agent 目录。
- 已验证 TypeScript、Vite build、`TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave` 和桌面 `.app` 多 Agent 选择器点击。真实分配按钮未在本机 UI 执行，避免写入当前 Hub。

2026-06-08 桌面默认页价值化调整：

- 用户指出默认页仍有无效数据和布局错乱后，首页从“提示词建议 + 记忆 + 接手材料 + 专业入口”再次调整为更清楚的阅读流。
- 新增“Vola 已经替你整理好的内容”区，把项目接手材料、交付物用途说明、长期记忆候选、可复用流程草稿、自动化和 Hook 风险用状态词展示，不再用裸数量作为默认信息。
- 长期记忆默认区只展示一条最值得看的候选，说明它记录了什么、为什么能让后续 AI 更准；完整候选列表留给专业入口。
- 专业资料和编辑入口默认折叠。普通用户不用先看 Threads、Runs、Artifacts 等专业模块；需要审查时可以展开进入。
- 桌面端验证使用完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`，URL 为 `tauri://localhost/codex-console`。页面没有 `The string did not match the expected pattern.`，也未看到卡片重叠。
- Skill 草稿专业页仍可打开，`Agent 分配预览` 区域保留；未保存草稿时按钮禁用，符合“不自动写入本地 Codex”的边界。
- 验证命令：TypeScript 检查、Vite build、desktop sidecar build、Tauri `.app` build、`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole(SkillCandidateSave|)$'` 和 `git diff --check`。
