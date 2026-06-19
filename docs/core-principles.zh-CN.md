# Vola 核心原则

更新日期：2026-06-17

## 核心定位

> Vola 是给个人和小团队用的 Agent 资料中心：把 profile、memory、projects、skills、MCP、prompt 和 playbook 放在一个私有 Hub 里，再安全同步到 Codex、Claude Code 等本机工具。

这句话是当前产品表达和功能取舍的基准。Vola 的重点不是替用户运行任务，也不是做公开 Skill 市场，而是让个人和小团队把 Agent 会反复用到的资料、经验、连接配置和权限放在同一个可备份、可迁移、可授权的位置。

## 我们管理什么

| 资产 | 说明 |
| --- | --- |
| `profile` | 稳定偏好、沟通习惯、做事原则 |
| `memory` | 可复用的长期记忆、项目经验、关系信息 |
| `projects` | 项目上下文、交接资料、阶段记录 |
| `skills` | 可复用 Agent Skill，包含脚本、依赖、资源和转换报告 |
| `MCP` | Vola 自身 MCP 接入、团队 MCP 说明、可安全同步的本地配置 |
| `prompt` | 可复用提示词模板 |
| `playbook` | 团队 AI 使用经验、评审流程、协作方法 |

## 产品原则

1. **资料归用户和团队所有**

   Vola 保存的是用户和小团队的 Agent 资产。平台连接、MCP、CLI 和本地同步都是使用方式，不是产品全部。

2. **先服务个人，再服务小团队**

   个人空间解决“我的上下文如何跨工具使用”。Team Library 解决“团队 Skill、MCP、prompt、playbook 如何共享给成员”。当前不把产品写成企业级组织管理、SSO、审批或审计平台。

3. **安全同步优先于覆盖配置**

   Codex 和 Claude Code 可以通过现有安全路径同步本地 Skill 和团队 MCP。写入必须使用既有路径规则、配置锁和 Vola 管理标记；同名非 Vola 管理目录不能覆盖。Cursor 和 Gemini CLI 当前保持导出、预览和手工处理。

4. **不自动安装第三方 MCP server**

   Vola 可以保存说明、健康检查、同步预览和本机刷新入口，但不替用户静默安装、注册或启用第三方 server、hook、plugin。

5. **让新用户先完成一个动作**

   首次使用不要求用户理解所有数据模型。默认推荐先连接 Codex，其次连接 Claude Code；连接后给出一条可复制的测试指令，再引导导入资料、同步团队资产和设置备份。

6. **高级能力留在后面**

   GitHub Backup、外部备份、团队资料、Skill 转换、Codex Console 都很重要，但首次体验要先让用户确认：本机工具已经能访问 Vola。

7. **公开生态可借鉴，不改变边界**

   Vola 可以吸收 cc-switch 的配置状态可见性、SkillHub 的发现和安装体验、企业 SkillHub 的版本和命名空间经验，但不能因此变成 provider manager、公开市场或企业治理平台。

## 平台边界

| 平台 | Vola 当前处理方式 |
| --- | --- |
| Codex | 推荐入口；支持 Vola MCP、团队 MCP 本机刷新、Skill 自动同步到 `~/.agents/skills` |
| Claude Code | 第二推荐入口；支持 Vola MCP、团队 MCP 本机刷新、Skill 自动同步到 `~/.claude/skills` |
| Cursor | 可连接 MCP，可分配 Skill，可预览和导出；不自动改本机配置 |
| Gemini CLI | 可连接 MCP，可分配 Skill，可预览和导出；不自动改本机配置 |
| Claude / ChatGPT Web | 适合读取 profile、memory、projects、skills 等 Hub 资料；本地 Skill 写入不适用 |

## 使用成本原则

每个入口都应回答这几个问题：

- 我现在应该点哪个按钮或运行哪条命令？
- 连接后我发哪句话确认可用？
- 我的团队资料会同步到哪个本机工具？
- 哪些平台能自动同步，哪些平台只能导出？
- 失败时是缺少 `neu`、Hub 未启动、未登录，还是平台配置不可写？

面向新用户，页面和 CLI 应优先提供：

- 默认推荐路径：Codex 优先，Claude Code 其次；
- 连接状态：`neu`、Hub、账号、平台配置、本地同步分别显示；
- 团队资产路径：团队 Skill / MCP -> 个人空间或本机配置 -> Codex / Claude Code 可用；
- 导出型平台提示：Cursor / Gemini CLI 明确显示“导出或手工处理”；
- 示例资料：空团队也能看到推荐目录和一份可复制模板；
- 恢复入口：同步前能预览，发生冲突时能看懂原因。

## 不做什么

- 不把 Vola 做成另一个 Coding Agent 或任务执行平台；
- 不把 Vola 只做成 MCP gateway；
- 不把 Vola 做成公开 Skill 市场；
- 不把 Team Library 夸大成企业级 SSO、审批、审计平台；
- 不为了自动化而越过本机安全路径、配置锁和用户确认。

