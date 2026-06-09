# Codex Console 与最初目标差距审计

更新日期：2026-06-08

本文记录当前实现距离最初讨论目标的差距。判断依据以当前仓库文件为准，不把聊天记忆当作完成证据。

## 最初目标

Vola 的方向不是大型 Agent 执行平台，也不是单一 memory MCP，而是个人 Agent 数据层。Codex、Claude、ChatGPT、Cursor 等工具产生的记忆、项目、技能、授权、执行记录和交付物，应能进入同一个可备份、可迁移、可审计的位置。

Codex Console 在这个方向里承担本地 Codex 工作资料的整理入口，核心对象包括：

- Codex Threads
- Goals
- Automations
- Runs
- Artifacts
- Memory Sync
- Hooks
- Handover Summary
- Skill Candidates

用户体验上的要求是：默认页面让普通用户看得懂 Vola 对后续 AI 有什么帮助，例如提示词如何写得更清楚、哪些长期记忆能提升准确度、下一个 Agent 应该先看什么，并尽量减少操作；专业用户可以继续进入详情、审查、编辑和同步。

## 当前总判断

不能说已经完成最初目标。当前更接近一个可用于桌面端试用的 Codex Console Lite，加上较完整的 Memory Sync 基础流程和 Skill Candidate 资产化入口。

按“个人 Agent 数据层”的完整目标看，当前约完成 78% 左右。按“Codex Console 可展示并整理本机 Codex 资料”看，当前约完成 90% 左右。差距主要不在扫描和展示，而在 people 目标、真实本地同步验证、候选 Skill 状态流转、自动化健康摘要和更细的 run/artifact 处置。

## 模块差距表

| 模块 | 当前状态 | 已有证据 | 主要缺口 | 下一步 |
| --- | --- | --- | --- | --- |
| 产品定位 | 基本完成 | README、页面和 `docs/agent-data-hub-iteration-plan.zh-CN.md` 已转向 Agent 个人数据 Hub | 公开表达还需要继续压到首页、onboarding、桌面首屏 | 保持同一套产品表达 |
| 桌面端接入 | 主体可用 | `desktop/src/lib.rs` 启动 Go sidecar，前端通过 Tauri command 获取 `/api` 根地址 | 桌面端还没有专门的系统托盘、后台状态、更新、崩溃恢复体验 | 先保持现状，后面再做桌面体验强化 |
| Codex Threads | 展示可用 | `GET /api/local/codex-console` 返回 threads；前端有 Threads tab | 还不能按重要性自动归纳用户真正关心的主题 | 增加项目级摘要和搜索 |
| Goals | 初版可用 | 后端从线程中提取 goal 线索，前端可展示 | 目标状态、预算、完成/阻塞原因仍是弱提取，不能覆盖全部 Codex 目标事件 | 继续读取 session 中更结构化的 goal 事件 |
| Automations | 只读可用 | 能读取 `.codex/automations/**/automation.toml` 并展示名称、状态、schedule、prompt，默认页会提示自动化和 Hook 只进入风险审查 | 没有最近运行结果、下次执行时间、失败提醒，也不能同步或编辑 | 先做自动化健康摘要，再考虑编辑入口 |
| Runs | 基础审计可用 | 能统计 tool call、browser/computer 线索、approval、error，并展示部分事件 | 时间线不够细，审批结果、工具输出、失败原因和截图交付物还没有完整串起来 | 做 run timeline 详情页 |
| Artifacts | 索引资产初版可用 | 能从 session 内容识别 HTML、Markdown、图片、PDF、PPT、文档、表格等路径，生成用途说明和给下个 Agent 的提示，并保存 `/platforms/codex/console/artifacts.json` | 还不能预览、打开文件、收藏，也没有“这是最终交付物”的判断 | 做预览、打开、收藏和最终交付标记 |
| Memory Candidates | 可用 | Codex / Chronicle 记忆已经进入候选列表，默认页会解释“记录了什么”和“为什么有用” | 还缺更强的项目推荐、people 目标和批量审查说明 | 增加 people 目标和项目推荐 |
| Memory Sync 写入 profile | 较完整 | `POST /api/local/codex-console/memory-sync` 可写入 profile；同步后标记 `synced`；单条候选可编辑后写入 | 没有 people 目标 | 增加 people 目标 |
| Memory Sync 写入 project | 可用 | `target:"project"` 会写入 project context，并用 marker 防重复 | 还没有根据线程自动推荐目标项目 | 增加项目推荐和批量规则 |
| Memory Review | 可用 | review 状态保存到 `/platforms/codex/console/memory-review.json` | 缺少普通用户能看懂的 review queue 文案和筛选 | 优化审查队列 |
| Memory Conflict | 基础完整 | profile conflict 支持保留已有、采用候选、两者共存、手工合并 | 还没有 people 目标下的冲突处理，也没有批量冲突队列 | 增加 people 冲突处理和批量队列 |
| Handover Summary | 项目资产初版可用 | 按 project 聚合 threads、runs、artifacts、memory candidates，并能编辑后保存到 `/projects/<project>/handover.md` | 摘要仍偏模板化，还没有版本历史查看和更自然的项目建议 | 增加历史版本和普通用户摘要 |
| Hooks | 审查初版 | 能展示 hook 文件、shebang、环境变量、风险信号、写入路径提示 | 触发时机、读取路径、执行日志、允许策略还没有 | 深化 Hook Review，但继续不自动启用 |
| Skill Candidates | 候选资产初版可用 | 能从成功 run 生成 `SKILL.md` 草稿，前端可查看、编辑、复制，保存到 `/skills/_candidates/<slug>/`，并加入 Codex、Claude Code、Cursor、Gemini CLI 分配后生成同步或导出预览 | 还没有真实本地同步验证，也没有 ready / archived 状态 | 增加同步验证和状态流转 |
| 授权 / Vault | Codex 导入有基础 | agent export payload 包含 sensitive findings 和 vault candidates | Codex Console 里还没有把授权、secret 风险和操作建议讲清楚 | 做授权风险摘要 |
| 多平台数据层 | 底座已有 | Vola 已有 Claude/Codex Skill 导入、转换、分配、本地同步、备份；Codex Console 候选 Skill 已能写入多 Agent 分配表，并生成 Codex/Claude Code 同步预览和 Cursor/Gemini CLI 导出预览 | 还没有从 Console 触发后的真实本地同步验证 | 做同步验证 |
| 团队 Skill / Agent 共享 | 管理流程初版可用 | 团队 Skills 页可显示未安装、团队有新版、已安装、个人副本较新，并能安装或更新个人副本；团队 Skill 有发布状态、团队可见性、个人订阅记录和审查历史；团队资料页可看订阅报表、更新通知和 Agent 配方审查；团队 Agent 可保存、发布并安装为个人 Agent 配置对象 | 更新检查已有接口和团队通知文件，但还没有接入服务启动后的常驻调度，也没有个人收件箱推送；团队 Agent 不是 Vola 内置执行器 | 接入调度和成员通知 |

## 距离“完整链路”还差什么

最初想要的完整链路可以拆成五步：

1. 读取本机 Codex 资料。
2. 把资料整理成普通用户能看懂的摘要。
3. 把有价值的内容变成 Vola 资产。
4. 让这些资产进入备份、迁移、审计和跨 Agent 复用。
5. 需要行动时，尽量由 Vola 自动整理建议，用户只在高风险处审查。

当前第 1 步基本完成，第 2 步已从数量看板改向“AI 使用改进台”，默认页开始展示提示词建议、Vola 已整理内容、长期记忆价值和专业入口；第 3 步里 Memory Sync 较完整，Skill Candidate 已能保存为 Hub 候选资产并进入多 Agent 分配预览，Handover 已能保存为可编辑项目文件，Artifact 已能保存为带用途说明的 Console 索引资产；第 4 步对 memory review、handover 文件、候选 Skill 文件、Agent 分配表和 artifact registry 已经成立，对 Automation 还没成立；第 5 步目前已有普通用户可读的提示词/记忆/交接建议，但真正自动处理仍主要集中在 Memory 和资产草稿生成。

团队 Skill 共享已经进入第 4 步：团队 Skill 能从团队空间安装到个人空间，个人副本可以继续进入 Agent 分配、本地同步和导出；发布状态、团队可见性、订阅记录、审查历史、管理员订阅报表和团队更新通知已具备基础能力。更新检查现在是接口和页面按钮，还没有接入服务启动后的常驻调度。

因此，最关键的差距不是页面样式，而是这些候选内容还没有全部进入 Vola 的资产生命周期。

## 建议推进顺序

### 1. Skill Candidate 进入 Hub

已完成前三步。Codex Console 现在可以把成功线程生成的 Skill Candidate 保存到 Hub 候选区，写入 `SKILL.md`、`candidate.vola.json` 和 `manifest.vola.json`，也可以把已保存候选加入 Codex、Claude Code、Cursor、Gemini CLI 分配表，并生成本地同步或导出预览。

已新增：

- `POST /api/local/codex-console/skill-candidates/save`
- 保存到 `/skills/_candidates/<slug>/SKILL.md`
- 同时写入 `candidate.vola.json` 和 `manifest.vola.json`
- 记录候选来源：thread、project、source path、tool calls、artifacts、confidence、signals、saved_at
- 前端只在专业详情里提供草稿编辑、保存、多 Agent 分配预览和 Skill 分配页入口

继续需要：

- 对候选 Skill 执行真实本地同步验证。
- 支持把候选状态从 draft 改为 ready / archived。

### 2. Handover 保存为项目文件

已完成第一步。Codex Console 现在可以把项目交接摘要保存为：

- `/projects/<project>/handover.md`
- 内容包含最近 threads、runs、artifacts、memory candidates 和审查注意事项
- 文件内保留 `Manual notes` 区域；重复保存会更新自动生成部分，并保留手写备注
- 保存后 Console 会显示 `saved` 状态、路径和保存时间

仍需继续：

- 页面内编辑 handover。
- 保存历史版本。
- 面向普通用户生成更自然的项目关注摘要。

### 3. Artifact Registry

已完成第一步。Codex Console 现在可以把 session 中识别到的交付物保存为 Console 资产：

- 保存到 `/platforms/codex/console/artifacts.json`
- 记录 `vola.codex-artifact-registry/v1` 版本、来源平台、保存时间、项目数和交付物数
- 每条 artifact 保留 name、kind、project、thread、source path 和 detail
- 每条 artifact 增加 `role`、`handoff_note` 和 `agent_instruction`，让下一个 Agent 知道它是交接文档、视觉证据、预览输出还是结构化数据
- Artifact 详情页会显示“给下一个 Agent 的用法”和可复制提示词
- Artifact tab 支持搜索、项目过滤和用途过滤
- registry 增加 `project_summaries`，包含每个项目的 role 分布和优先交付物
- Console 再次读取时会显示 saved 状态、路径和保存时间

仍需继续：

- 支持打开文件、复制 artifact 路径和预览 HTML、图片、PDF、PPT 等交付物。
- 增加“最终交付物 / 中间文件 / 日志引用”的人工标记；当前只有自动 role。

### 4. Memory Sync 增加 people 和编辑

Memory Sync 目前最接近可用。写入前编辑已完成一部分：

- `memory-sync` 请求支持 `content_overrides`。
- 记忆详情页支持在写入前编辑候选内容。
- 保存到 profile memory 或 project context 的内容使用编辑后的文本。
- 后端测试覆盖编辑内容写入 profile memory，并确认原始候选文本没有被写入。
- Profile conflict 的 merge 处理支持 `merged_content`，记忆详情页会提供“合并后写入内容”编辑框。
- 后端测试覆盖编辑后的冲突合并文本写入 profile memory。

仍然还差：

- people 目标
- 候选解释：为什么建议存、可能影响什么 Agent

### 5. Automations 健康摘要

目前只展示 `.codex/automations` 数量和配置。下一步应该让用户知道：

- 哪些自动化在运行
- 最近有没有失败
- 下次大概什么时候执行
- 是否有需要人工处理的结果

## 当前不建议做

- 不从 Vola 自动启用 hook。
- 不自动安装 Skill Candidate。
- 不把所有 Codex 线程全量写入长期记忆。
- 不把 Vola 做成 Codex Desktop 的替代执行界面。
- 不把团队 Agent 做成 Vola 内置运行平台；当前保存的是可发布、可安装的 Agent 配置对象和共享资料。

这些边界仍然符合最初定位：Vola 管数据和资产，不抢 Codex 的执行位置。

## 验证状态

本次审计后已继续实现 Skill Candidate 保存到 Hub 候选区、Handover 保存到项目文件、Artifact Registry 保存到 Console 索引资产，并把默认页从数量看板改成 AI 使用改进台。

已查看的当前证据：

- `internal/api/codex_console.go`
- `internal/api/router.go`
- `web/src/pages/CodexConsolePage.tsx`
- `web/src/api.ts`
- `web/src/App.tsx`
- `docs/agent-data-hub-iteration-plan.zh-CN.md`
- `docs/codex-console-product-decision-log.zh-CN.md`

新增实现证据：

- `POST /api/local/codex-console/skill-candidates/save`
- `/skills/_candidates/<slug>/SKILL.md`
- `/skills/_candidates/<slug>/candidate.vola.json`
- `/skills/_candidates/<slug>/manifest.vola.json`
- 前端 Skill 草稿详情里的“保存为 Vola Skill 草稿”
- `POST /api/local/codex-console/handovers/save`
- `/projects/<project>/handover.md`
- 前端项目交接详情里的“保存为项目交接文件”
- `POST /api/local/codex-console/artifacts/save`
- `/platforms/codex/console/artifacts.json`
- 前端交付物详情里的“保存交付物索引 / 更新交付物索引”

本次已验证：

- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'`
- `GOCACHE=/private/tmp/vola-go-cache go test ./internal/platforms ./internal/api`
- `npm --prefix web run build`
- `./node_modules/.bin/tauri build --bundles app`（`desktop/`）
- `npm --prefix desktop run check:runtime -- --require-new`
- 桌面新包运行时检查显示只运行 `/desktop/target/release/bundle/macos/Vola.app` 和它的 sidecar，没有旧 `src-tauri` app。
- 用户侧桌面截图显示新版 Codex Console 已切到提示词建议、长期记忆、交接材料方向；发现三列 memory 卡片拥挤后，已重新改为单条记忆价值摘要和更紧凑的专业入口。
- runtime 检查脚本已修正为精确匹配 executable 路径，避免把 Codex Computer Use 里包含 `Vola.app` 字样的历史上下文误列为 Vola 进程。
- `git diff --check`

本次继续验证：

- 用户再次指出默认页仍像数据看板、价值不够明确后，默认页已继续调整为“AI 使用改进台”。
- 首页不再显示工作区统计；工作区只在专业视图出现。
- 提示词建议改为纵向建议列表，包含建议、可直接使用的写法和依据，不再提供普通用户容易误解的操作按钮。
- 长期记忆卡片改为最多展示两条高价值候选，路径和 Markdown 会清理成可读摘要，并显示“为什么有用”。
- 右侧专业入口从数字改为状态词，减少普通用户被原始数据量干扰。
- `npm --prefix web run build` 已通过。第一次失败是权限问题：沙箱不能写 `/Users/zhongmoshu/Desktop/work/Vola/web` 下的 Vite 临时文件；外部权限重跑后通过。
- `./node_modules/.bin/tauri build --bundles app` 已通过，生成新的 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`。
- `npm --prefix desktop run build` 的 `.app` 编译和 bundle 已完成，但 DMG 打包阶段 `bundle_dmg.sh` 失败；桌面验证改用 `.app` bundle。
- 运行时检查脚本已能识别旧 `src-tauri` app 的 `Contents/MacOS/app`，不再漏掉旧窗口。
- Computer Use 验证必须传新 `.app` 完整路径；传应用名 `Vola` 会重新拉起旧 `/src-tauri/.../vola.app`。
- 最终桌面读取证据：新 app bundle id 为 `cn.vola.desktop`，页面 URL 为 `tauri://localhost/codex-console`，首页没有 `The string did not match the expected pattern.`，Codex Console 默认页显示提示词建议、长期记忆价值摘要和交接材料。
- 默认页布局已改为单列阅读流。桌面端新包读取到 `tauri://localhost/codex-console` 后，专业资料区显示在主内容下方，不再挤压提示词建议和长期记忆摘要。
- `memory-sync` 已支持编辑后写入。新增测试 `TestSQLiteSharedServerLocalCodexConsoleMemorySyncEditedContent` 覆盖编辑内容保存到 Vola profile memory。
- `memory-conflict/resolve` 已支持编辑后的 merge 内容。新增测试 `TestSQLiteSharedServerLocalCodexConsoleMemoryConflictResolveEditedMerge` 覆盖自定义合并内容保存到 Vola profile memory。
- Artifact registry 已从纯列表增强为接手材料。`TestSQLiteSharedServerLocalCodexConsoleArtifactRegistrySave` 已覆盖 `role`、`handoff_note`、`agent_instruction` 写入 registry。
- Artifact registry 已增加项目摘要。`TestSQLiteSharedServerLocalCodexConsoleArtifactRegistrySave` 已覆盖 `project_summaries`、role 统计和优先交付物。
- 本次验证命令：`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsole'`、`GOCACHE=/private/tmp/vola-go-cache go test ./internal/platforms ./internal/api`、`npm --prefix web run build`、`./node_modules/.bin/tauri build --bundles app`、`npm --prefix desktop run check:runtime -- --require-new`、`git diff --check`。

桌面端复盘：

- 本次再次看到 `The string did not match the expected pattern.` 时，实际同时运行了旧 `/src-tauri/.../vola.app` 和新 `/desktop/.../Vola.app`。旧进程已结束。
- 桌面构建顺序已改为先构建 `web/dist`，再同步到 `internal/web/dist` 并构建 sidecar。
- 新增 runtime 检查脚本，后续桌面验证前必须确认没有旧 `src-tauri` 进程。
- 本次验证中，Computer Use 使用 `app: "Vola"` 又拉起了旧 `/src-tauri/.../vola.app`；已关闭旧窗口。后续读取桌面端必须使用完整路径，不能用应用名。

2026-06-08 继续审计：

- 默认页已从“主内容 + 右侧专业详情”改为单阅读区。桌面端截图证据显示页面顺序为：可复制给下次 AI 的提示词、提示词改进建议、长期记忆价值摘要、下一个 Agent 接手材料、专业入口。
- 普通用户默认不再看到工作区统计、原始线程计数或右侧调试式数据栏；这些内容仍可从专业入口进入。
- 长期记忆默认页只展示最多两条候选，并说明“为什么有用”，避免把 `memories/...`、`/Users/...` 和大段 Markdown 原文直接给普通用户。
- 专业入口继续存在，但显示状态词而不是裸数字。
- 今日验证：
  - TypeScript 检查通过：`node typescript/bin/tsc -p web/tsconfig.json --noEmit --pretty false`。
  - Vite build 通过：`node web/node_modules/vite/bin/vite.js build`。因本机 arm64 Node 与 Rollup 原生包签名冲突，本次使用 x64 Node 和 `@rollup/rollup-darwin-x64`。
  - sidecar 更新通过：`node desktop/scripts/build-backend-sidecar.mjs`。
  - Tauri `.app` 打包通过：`node desktop/node_modules/@tauri-apps/cli/tauri.js build --bundles app --config '{"build":{"beforeBuildCommand":""}}'`。
  - runtime 检查通过，当前运行的是 `/desktop/target/release/bundle/macos/Vola.app` 和同路径 sidecar。
  - Computer Use 使用完整 `.app` 路径读取到 `tauri://localhost/codex-console`，页面没有 `The string did not match the expected pattern.`，也没有截图中那种卡片重叠。
  - `git diff --check` 通过。

2026-06-08 项目交接继续推进：

- `POST /api/local/codex-console/handovers/save` 新增 `content_override`。前端可提交专业用户编辑后的 Markdown。
- 后端保存编辑内容时会自动确保存在 `<!-- codex-console-handover:<id> -->` marker，避免用户删掉隐藏标记后 Console 无法识别 saved 状态。
- Console handover summary 新增 `version` 和 `saved_content`。专业详情页可以加载已保存的交接正文，也能显示当前 FileTree 版本号。
- Handover 详情页新增“交接文件内容”编辑框和“恢复当前草稿”。保存后写入 `/projects/<project>/handover.md`，FileTree 版本记录继续保留。
- 已验证：
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsoleHandoverSave`
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsole`
  - TypeScript 检查
  - Vite build

已有文档记录显示，前一轮还通过：

- `go test ./internal/platforms ./internal/api`
- `npm run build`（`web/`）
- `git diff --check`
- desktop app-only 构建和桌面页面验证

2026-06-08 Skill 草稿可编辑后保存：

- `POST /api/local/codex-console/skill-candidates/save` 新增 `draft_override`。用户在专业详情区编辑 `SKILL.md` 草稿后，保存内容会写入 `/skills/_candidates/<slug>/SKILL.md`。
- 保存响应、candidate metadata 和 Console 再次读取结果会返回 `edited`，用于区分纯生成草稿和人工改过的草稿。
- Console 读取已保存候选时，会回读 Hub 里的 `SKILL.md` 正文，避免刷新后又显示旧的生成草稿。
- 前端 Skill Candidate 详情页已把只读 `<pre>` 改成可编辑文本区，复制草稿和保存动作都使用当前编辑内容。已保存草稿也能继续编辑后覆盖保存，但仍不会自动安装到 Codex 或分配给任何 Agent。
- 已验证：
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave`
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsole`
  - TypeScript 检查：`node typescript/bin/tsc -p web/tsconfig.json --noEmit --pretty false`
  - Vite build：`node web/node_modules/vite/bin/vite.js build`

2026-06-08 Skill 草稿多 Agent 分配预览：

- 前端 Skill Candidate 专业详情区新增 Agent 目标选择：Codex、Claude Code、Cursor、Gemini CLI。
- 分配接口继续只写 Vola 的 `/settings/agent-skill-assignments.json`，不会写入 `~/.codex/skills`、`~/.claude/skills`、`.cursor/rules` 或 `GEMINI.md`。
- 预览结果会按 Agent 分组展示：Codex / Claude Code 显示本地同步计划，Cursor / Gemini CLI 显示导出包预览。
- 已验证：
  - TypeScript 检查通过。
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave$'` 通过。首次在沙箱内失败，原因是 `httptest` 不能绑定本地端口；非沙箱权限重跑通过。
  - Vite build 通过。首次在仓库根目录运行失败，原因是 Vite 找不到 `index.html`；切到 `web/` 目录重跑通过。
  - 新桌面 `.app` 使用完整路径打开后，`tauri://localhost/codex-console` 无 pattern 错误；Skill 草稿区显示 Codex、Claude Code、Cursor、Gemini CLI 四个目标，Claude Code 勾选交互正常。
- 未执行：真实候选的“加入所选 Agent 分配并预览”按钮，避免向当前本机 Hub 写入测试分配。
  - sidecar 更新：`node desktop/scripts/build-backend-sidecar.mjs`
  - desktop `.app` 构建：`node desktop/node_modules/@tauri-apps/cli/tauri.js build --bundles app --config '{"build":{"beforeBuildCommand":""}}'`
  - runtime 检查：`node desktop/scripts/check-desktop-runtime.mjs --require-new`
  - Computer Use 使用完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app` 读取桌面窗口，URL 为 `tauri://localhost/codex-console`，Skill 草稿页显示可编辑 `SKILL.md` 文本区、“恢复当前草稿”、“保存为 Vola Skill 草稿”，并显示“不会自动安装到 Codex 或启用给 Agent”。
  - 复盘：使用应用名 `Vola` 仍会拉起旧 `/src-tauri/.../vola.app`，旧包会出现 `The string did not match the expected pattern.`。桌面验证必须继续使用完整 `.app` 路径。

2026-06-08 Skill 草稿进入 Codex 分配预览：

- 新增 `POST /api/local/codex-console/skill-candidates/assign-preview`。请求默认分配到 Codex，也支持传入 `agent_ids` 和测试用 `target_roots`。
- 接口会确认候选已经保存为 Hub Skill，然后把 `/skills/_candidates/<slug>` 写入 Vola 的 `/settings/agent-skill-assignments.json`。
- 接口返回 `sync_preview`，复用现有 `/api/local/skills/sync/preview` 的计划结构，展示新增、更新、冲突、目标目录和变更文件。
- 该接口只写 Vola 分配表，不写 `~/.codex/skills`。真正写本地目录仍需要到 Skill 分配页执行本地同步。
- 前端 Skill Candidate 详情页新增“Agent 分配预览”区域。已保存草稿可点击“加入 Codex 分配并预览”，页面显示 Codex 目标目录、预览新增/更新/冲突数量和前几条文件变化，并提供 Skill 分配页入口。
- 已验证：
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsoleSkillCandidateSave`
  - `GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run TestSQLiteSharedServerLocalCodexConsole`
  - TypeScript 检查：`node typescript/bin/tsc -p web/tsconfig.json --noEmit --pretty false`
  - Vite build：`node web/node_modules/vite/bin/vite.js build`

2026-06-08 团队 Skill 共享文章吸收：

- 已读取文章《我折腾了好久的Skills团队共享，终于有产品替我做出来了。》，核心结论是团队 Skills / Agent 需要共享、安装、更新和角色权限，而不是只靠 zip 或链接传播。
- 已新增调研记录：`docs/team-skill-sharing-article-notes.zh-CN.md`。
- 已确认 Vola 已有团队、成员角色、团队 Skill 列表、团队文件树、团队 Skill 复制到个人空间、team scope 分配表、本地同步和导出能力。
- Skills 页团队范围现在会对比个人空间同路径 Skill，并展示未安装、团队有新版、已安装、个人副本较新。
- 团队 Skill 卡片现在可以直接安装到个人空间；团队版本较新时可以更新个人副本。更新复用现有 `copy-to-personal` 接口的 `overwrite` 参数。
- 新增团队 Skill 发布记录：`/settings/team-skill-publications.json`。管理员可发布、转草稿或归档；普通成员只看到 `published + team` 内容。
- 发布可见性不仅限制列表和文件树，也限制团队 Skill 复制到个人空间与 Skill 转换预览，避免成员按已知路径读取草稿内容。
- 新增个人团队 Skill 订阅记录：`/settings/team-skill-subscriptions.json`。复制到个人空间后记录来源团队、路径、文件指纹和更新时间，页面加载时检查团队源文件是否已有新版。
- 新增团队 Agent 对象：`/team/agents/<slug>/agent.vola.json`。管理员发布后，成员可安装到个人 `/agents/<slug>/agent.vola.json` 和 README。
- 团队资料库模板保留 `/team/agents/README.md`，用于说明团队 Agent 配置对象、默认 Skill、模型、权限和目标工具配置。
- 新增团队审查历史：`/settings/team-skill-review-history.json`，Skill / Agent 的提交审查、通过、要求修改、发布和归档会留下记录。
- 新增管理员订阅报表：团队资料页展示每个团队 Skill 在成员侧的未安装、已安装、可更新和来源缺失状态。
- 新增更新检查接口与团队通知文件：`/settings/team-skill-update-notifications.json`。管理员可手动触发检查，看到需要处理的成员更新提醒。
- 已补测试覆盖团队 Skill 覆盖更新个人副本、订阅更新检查、发布可见性、审查历史、订阅报表、团队更新通知、团队 Agent 发布和安装。
- 已验证：
  - TypeScript 检查：`cd web && node node_modules/typescript/bin/tsc -p tsconfig.json --noEmit --pretty false`
  - Vite build：`cd web && node node_modules/vite/bin/vite.js build`
  - Web embed 同步：`rm -rf internal/web/dist && cp -r web/dist internal/web/dist`
  - 后端目标测试：`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api -run 'TestSQLiteSharedServer(TeamSkillUploadAndCopyToPersonal|TeamSkillPublicationVisibility|TeamAgentPublicationAndInstall|LocalCodexConsoleSkillCandidateSave)$'`
  - 后端包全量测试：`GOCACHE=/private/tmp/vola-go-cache go test ./internal/api`
  - sidecar 更新：`node desktop/scripts/build-backend-sidecar.mjs`
  - desktop `.app` 构建：`cd desktop && node node_modules/@tauri-apps/cli/tauri.js build --bundles app --config '{"build":{"beforeBuildCommand":""}}'`
  - runtime 检查：`node desktop/scripts/check-desktop-runtime.mjs --require-new`
- 构建复盘：
  - Vite 必须在 `web/` 目录执行。直接在仓库根目录执行会找不到 `index.html`。
  - 桌面端必须在 `desktop/` 目录构建。仓库根目录会命中旧 `src-tauri` 项目，并因旧 updater 公钥缺少 `TAURI_SIGNING_PRIVATE_KEY` 构建失败；这不是本次要验证的桌面包。
  - 桌面运行检查必须使用完整路径 `/Users/zhongmoshu/Desktop/work/Vola/desktop/target/release/bundle/macos/Vola.app`，避免打开旧包。
