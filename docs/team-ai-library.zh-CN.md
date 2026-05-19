# 团队 AI 资料库

neuDrive 当前已经能承载小团队内部 Skill、MCP 配置、提示词和 AI 使用技巧，但不同内容要放在不同位置。本阶段按“团队资料库”使用，不按完整企业协作平台使用。

## 支持情况

| 类型 | 当前支持 | 推荐位置 | 说明 |
| --- | --- | --- | --- |
| 团队 Skill | 支持 | `/skills/<name>/...` | 当前团队的 `SKILL.md` 会进入团队 Skills、MCP `list_skills`、Agent 分配、本地同步和 Claude / Codex 转换 |
| MCP 配置 | 支持保存和共享说明 | `/team/mcp/...` | 适合保存 server URL、stdio command、环境变量名、负责人、安全说明；当前不会自动安装第三方 MCP server |
| 提示词 | 支持 | `/team/prompts/...` | 适合保存 reusable prompt；如果有脚本、资源、触发规则，建议升级成 Skill |
| AI 使用技巧 | 支持 | `/team/playbooks/...` | 适合保存模型选择经验、评审流程、协作方法、团队约定 |

## 能用上的现有能力

- 团队与成员：可以创建团队，把用户加入团队，并使用 owner、admin、member、viewer 角色控制成员管理和写入权限。
- 团队空间：团队资料通过独立的 hub user 保存，不和成员个人空间混在一起。
- 文件树：团队的 `/team/...` 和 `/skills/...` 都走同一套 Hub 文件树。
- 搜索：`/team/...` 文件可通过文件树读取和普通搜索访问；`/skills/...` 还会出现在 Skills 列表。
- MCP：Agent 传 `scope=team` 和 `team` / `team_id` 后，可以用 `read_file` 读取团队 `/team/...`，用 `list_skills` / `read_skill` 读取团队 `/skills/...`。
- 备份：GitHub Backup 和外部 ZIP 备份都会包含 `/skills/...` 与 `/team/...` 下的文件。
- 权限：scoped token、trust level 和现有文件访问控制仍然生效。

## 当前边界

- 这是 Team Library，不是企业级组织平台。
- 已有团队、成员和基础角色，但还没有组织层级、审批流、审计报表、SSO 统一管理和企业管理员后台。
- 前端主要围绕团队资料和成员管理，没有完整项目协作管理页。
- MCP 配置当前只保存说明和样例，不自动改团队成员本机的 MCP 配置。
- Prompt 没有独立 prompt registry；短模板放 `/team/prompts`，可运行包放 `/skills`。
- Cursor / Gemini CLI 对团队 Skill 仍按导出包处理，不自动写本地配置。

## 第一阶段目录

页面入口：`Team Library`

建议目录：

```text
/team/README.md
/team/mcp/README.md
/team/prompts/README.md
/team/playbooks/README.md
/skills/<team-skill>/SKILL.md
```

这套目录能直接进入现有备份链路。上线测试时按“小团队共享资料库”验收，不按完整团队协作平台验收。
