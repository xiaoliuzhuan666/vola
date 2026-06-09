# 团队 Skill / Agent 共享文章调研记录

更新日期：2026-06-08

文章：<https://mp.weixin.qq.com/s/ylu0WvLJGNTZiim-NQmPQQ>

标题：《我折腾了好久的Skills团队共享，终于有产品替我做出来了。》

## 文章重点

文章讨论的是小团队怎样让 Skills 和 Agent 在团队里流通。作者认为很多团队现在还在用 zip、GitHub 链接或手工安装的方式传 Skill，成员不知道自己装的是哪个版本，更新也要靠人工反复发送。

文章里提到的 Accio Work 企业版能力包括：

- 团队工作空间。
- 成员管理和角色权限：所有者、管理员、普通成员。
- 团队资产管理：团队共享 Skills 和 Agent。
- 管理员可以把团队 Skill 设置为全员可见。
- 成员能看到“未安装”的团队 Skill，点击加号安装到自己的空间。
- 团队 Skill 支持一键更新，作者还希望后续能登录时自动检查更新。
- 普通成员可以推荐 Agent 到团队空间，但默认对其他人不可见；管理员设置全员可见后，其他成员可以直接使用。

## 对 Vola 的判断

这篇文章和 Vola 的方向相近，但 Vola 不应该变成 Accio Work 这类完整 Agent 协作平台。更合适的吸收方式是：

- Vola 负责团队 Skills、Agent 配方、权限、来源、版本和备份。
- Codex、Claude、Cursor、Gemini CLI 等工具负责实际运行。
- 团队 Skill 可以进入 Vola 的资产生命周期：上传、安装到个人空间、更新个人副本、分配给目标 Agent、同步或导出。
- 团队 Agent 先以“Agent 配方”形式保存：说明、默认 Skill、模型、权限、审批动作、目标工具配置方式。Vola 暂不直接运行或托管 Agent。

## 当前已有能力

代码审计确认，Vola 已有这些底座：

- 团队与成员接口：`/api/teams`、`/api/teams/{team}/members`。
- 团队 Skill 列表：`/api/teams/{team}/skills`。
- 团队文件树：`/api/teams/{team}/tree/*`。
- Agent 访问团队资料：`/agent/teams`、`/agent/teams/{team}/skills`。
- 团队 Skill 可以复制到个人空间：`POST /api/skills/copy-to-personal`。
- Skills 页已支持个人 / 团队范围切换。
- Skill 分配表已支持 team scope，团队 Skill 可分配给 Codex、Claude Code、Cursor、Gemini CLI。
- 本地 Skill 同步和导出支持 team scope。

## 本次吸收并实现

本次实现这些能力：

1. 团队 Skill 安装 / 更新状态

   团队 Skills 页面现在会自动读取个人空间同路径 Skill，给每个团队 Skill 标记：

   - 未安装到个人
   - 团队有新版
   - 已安装到个人
   - 个人副本较新

   用户在团队 Skill 卡片上可以直接“安装到个人”或“更新个人副本”。更新时会调用已有复制接口的 `overwrite` 模式。复制完成后页面会刷新个人副本状态。

2. 团队 Skill 发布与可见性

   新增团队发布记录文件：`/settings/team-skill-publications.json`。每个团队 Skill 可以记录：

   - `draft`
   - `published`
   - `archived`
   - `private`
   - `team`

   普通成员只能看到 `published + team` 的团队 Skill。团队所有者和管理员仍可以看到草稿和归档内容，用于审核、重新发布或查历史。普通成员不能把 Skill 发布给全员，也不能归档团队共享 Skill。

   后端读取也按同一规则处理：普通成员即使知道草稿 Skill 的路径，也不能通过复制到个人空间或 Skill 转换预览读取草稿内容。

3. 团队 Skill 订阅与自动检查更新

   新增个人订阅记录文件：`/settings/team-skill-subscriptions.json`。当用户把团队 Skill 安装到个人空间时，Vola 会记录团队来源、源路径、目标路径、文件数量、字节数、安装时间和团队源文件指纹。

   页面加载团队 Skills 时会读取订阅记录，并重新计算团队源文件指纹。如果团队源文件已变化，会显示“团队有新版”，用户可以直接更新个人副本。

   管理员可以在团队资料页查看全员订阅报表，按成员确认每个团队 Skill 的未安装、已安装、可更新和来源缺失状态。更新检查会写入 `/settings/team-skill-update-notifications.json`，用于保留团队级提醒。

4. 团队 Agent 对象

   新增团队 Agent 对象：`/team/agents/<slug>/agent.vola.json`，同时生成对应 README。对象记录说明、默认 Skill、目标 Agent、模型、权限、需要人工审批的动作和维护人。

   团队 Agent 支持 `draft`、`published`、`archived` 和 `private`、`team` 可见性。管理员发布后，成员可以安装到个人空间，生成 `/agents/<slug>/agent.vola.json` 与 `/agents/<slug>/README.md`。

   Skill 和 Agent 的审查动作会写入 `/settings/team-skill-review-history.json`。成员创建团队资产时可提交审查，管理员可通过或要求修改。

5. 团队 Agent 配方目录

   团队资料库模板新增 `/team/agents/README.md`，用于保存团队共享 Agent 的说明、默认 Skill、模型、权限建议、需要人工审批的动作，以及 Codex / Claude / Cursor / Gemini CLI 的配置说明。

## 仍未实现

- 审查历史已可记录，但还没有多管理员投票、评论线程或必须多人通过的策略。
- 更新检查已有后端接口和团队通知文件，但还没有接入服务启动后的常驻调度，也没有把提醒写入成员个人收件箱。
- 团队 Agent 是可安装的配置对象，不是 Vola 内置执行器。真正运行仍由 Codex、Claude Code、Cursor、Gemini CLI 等工具负责。

## 后续建议

下一阶段更值得做的是团队审查与通知：

- 管理员看到更清晰的待审查 Skill / Agent 队列。
- 审查记录增加评论线程和多人通过策略。
- 成员登录时看到“可安装 / 可更新”的提醒。
- 更新检查接入明确的调度周期。
- 已安装的个人副本继续进入 Agent 分配、本地同步或导出包。

这个方向和 Vola 的“个人 / 团队 Agent 数据层”定位一致，也不会把产品推向执行型 Agent 平台。
