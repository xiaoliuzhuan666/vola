# neuDrive 发布执行清单

更新时间：2026-05-14

本文记录本次 release candidate 的执行状态。目标是把当前工作区整理成一份可审、可构建、可部署、可恢复演练的发布包。

## 上线测试策略

本次按“个人版先上线测试，小团队只作为资料库试用”执行。

| 场景 | 本次处理方式 | 不承诺内容 |
| --- | --- | --- |
| 个人使用 | 作为主要测试对象，验证账号、容量、Hub 数据、Skill、同步、备份和恢复 | 不承诺公开 SaaS 大规模开放 |
| 小团队使用 | 只按 Team Library 验收：团队资料、团队 Skill、成员角色和团队文件树 | 不承诺企业级组织管理、审计报表、SSO、审批流 |
| 多 Agent Skill | Claude Code / Codex 验证自动同步；Cursor / Gemini CLI 验证导出包 | 不承诺全平台自动写入本地配置 |
| 备份恢复 | 验证 Postgres dump、真实外部备份目标和临时环境恢复 | 不承诺 GitHub Backup 或 ZIP 单独恢复完整服务 |

可执行清单见：[neuDrive 小范围上线测试清单](launch-test-checklist.zh-CN.md)。

## 当前快照

- 当前分支：`main`
- 远端状态：已合入 `origin/main`，当前不再落后。
- 已合入远端提交：`2f255a1 Hide browser extension from user-facing surfaces`
- 远端提交影响文件：`README.md`、`README.zh-CN.md`、`docs/setup*.md`、`docs/reference.zh-CN.md`、`web/src/pages/DashboardPage.tsx`、`web/src/pages/DeveloperAccessPage.tsx`、`web/src/pages/PublicPages.tsx`
- 已处理交集文件：`README.md`、`README.zh-CN.md`、`web/src/pages/DashboardPage.tsx`、`web/src/pages/PublicPages.tsx`

已确认 README、Dashboard、PublicPages、服务端 SEO 和主要 docs 中没有把 browser extension 对外展示文案重新带回。`docs/release-execution-plan.zh-CN.md` 中保留的 `browser extension` 只是在描述本次合入检查背景。

## 应纳入 Release 的改动

| 分组 | 文件范围 | 发布说明 |
| --- | --- | --- |
| 产品表达与文档 | `README.md`、`README.zh-CN.md`、`docs/github-backup*.md`、`docs/deployment-reliability.zh-CN.md`、`docs/release-readiness.zh-CN.md`、`docs/team-ai-library.zh-CN.md`、`docs/agent-*.zh-CN.md` | Agent 个人数据 Hub、备份边界、Skill 目标和团队资料库说明 |
| 团队资料库 | `web/src/App.tsx`、`web/src/pages/TeamLibraryPage.tsx`、`internal/api/teams.go`、`internal/api/team_scope.go`、`internal/services/team_service.go`、`internal/storage/sqlite/teams.go` | 新增 `/team` 页面和侧边栏入口；支持团队、成员角色和团队文件树；按资料库试用验收 |
| 备份恢复 | `internal/api/backup_*`、`internal/backups/`、`internal/storage/sqlite/backup_targets.go`、`migrations/021_*`、`022_*`、`023_*`、`internal/jobs/scheduler.go` | GitHub Backup、WebDAV / S3-compatible 外部备份、恢复预览、运行历史、保留策略 |
| Skill Hub | `internal/api/local_skill_sync.go`、`internal/api/skill_assignments.go`、`internal/api/skill_conversion.go`、`internal/skillsarchive/manifest.go`、`web/src/pages/data/DataSkillsPage.tsx`、`web/src/pages/SkillsImportPage.tsx` | Skill manifest、Agent 分配、本地同步、Claude / Codex 转换 |
| 本地导入 | `internal/api/local_platform*.go`、`internal/platforms/claude_migration.go`、`internal/storage/sqlite/client_agent_export_*.go` | Claude / Codex 本地资料导入与归档能力 |
| 部署配置 | `deploy/k8s/app.yaml`、`deploy/prod/README.md`、`deploy/prod/deploy.sh`、`docker-compose.yml` | Git mirror PVC、生产环境变量、外部备份所需目录 |
| 前端状态页 | `web/src/api.ts`、`web/src/pages/GitMirrorPage.tsx`、`web/src/pages/DashboardPage.tsx`、`web/src/pages/OnboardingPage.tsx`、`web/src/pages/PublicPages.tsx`、`web/src/index.css` | 备份状态、导入状态、公开页面和产品入口 |
| 测试 | `internal/api/sqlite_shared_test.go`、`internal/platforms/claude_migration_test.go`、`internal/skillsarchive/archive_test.go`、`internal/backups/upload_test.go` | SQLite、Skill archive、导入、备份上传测试 |
| 嵌入式前端产物 | `internal/web/dist/**` | 只保留最后一次 `make build` 生成的产物 |

## 不应直接纳入 Release 的内容

| 文件或目录 | 处理方式 |
| --- | --- |
| `.playwright-mcp/` | 浏览器验证临时输出，建议放入本地归档或加入忽略规则，不随 release commit |
| `console-stage4*.txt` | 临时 console 输出，不随 release commit |
| `neudrive-local-home.png`、`stage3-*.png`、`stage4-*.png`、`sync-backup-*.png` | 可作为演示证据单独归档，不随默认 release commit |
| `stage4-*.md` | 阶段记录可归档；如果要保留，需要放到明确的 docs 目录并说明用途 |
| 多轮 `internal/web/dist/assets/*` hash 产物 | 最后一次构建后统一确认，不保留中间状态 |

## 执行顺序

1. 合入远端提交 `2f255a1`，处理 README、Dashboard、PublicPages 的冲突风险。
2. 完成文件分组，确认临时文件不进入发布提交。
3. 用最终源码执行 `npm run build` 和 `make build`，让 `internal/web/dist` 只代表最后一次构建。
4. 在生产同构环境确认环境变量和持久卷。
5. 完成 Postgres dump 和恢复演练。
6. 至少验证一个离开服务器的备份目标。
7. 执行完整构建测试与页面冒烟验证。
8. 更新 `docs/release-readiness.zh-CN.md` 的真实验证记录。
9. 按 `docs/launch-test-checklist.zh-CN.md` 完成个人账号和 Team Library 第一阶段验收。

## 生产配置检查项

| 配置 | 发布前状态要求 |
| --- | --- |
| `PUBLIC_BASE_URL` | 必须是最终 HTTPS 域名 |
| `DATABASE_URL` | 必须指向持久化 Postgres |
| `JWT_SECRET` | 必须保存在安全位置，恢复时继续使用原值 |
| `VAULT_MASTER_KEY` | 必须保存在安全位置，恢复时继续使用原值 |
| `GIT_MIRROR_HOSTED_ROOT` | hosted 模式推荐 `/data/git-mirrors`，并挂载可写 PVC |
| `GITHUB_APP_CLIENT_ID` | Hosted GitHub Backup 必填 |
| `GITHUB_APP_CLIENT_SECRET` | Hosted GitHub Backup 必填，不写入 Git |
| `GITHUB_APP_SLUG` | Hosted GitHub Backup 必填 |
| 外部备份目标密钥 | 只进环境变量、K8s Secret 或部署平台 secret，不写入 Git |

## 本轮已验证

- `go test ./...` 通过。
- `cd web && npm run build` 通过；Vite 仍提示部分 chunk 超过 500 KB，是体积提醒，不是构建失败。
- `make build` 通过；已重新生成 `internal/web/dist`，并生成 `bin/neudrive`、`bin/neu`。
- `docker build -t neudrive:rc .` 通过。
- Team Library 核心 API 用例通过：同一用户多团队成员关系、团队 Skill 读取、个人 Skill 隔离、member 写入和 viewer 禁写。
- 文件树读取行为已覆盖：不存在的文件路径返回 404，不再作为空目录返回。
- 本地 SQLite 服务启动通过，`/api/health` 返回 `storage: sqlite`。
- `/`、`/team`、`/sync-backup`、`/skills` 页面冒烟验证通过，Playwright 控制台没有 error。
- `/api/ops/status` 不带 token 返回 401，符合鉴权预期。
- 首页静态 HTML、服务端 SEO 默认值、公开页文案已对齐 Agent 个人数据 Hub 表达。

## 本轮未验证

- 生产同构环境启动。
- Postgres dump 和临时库恢复。
- GitHub Backup 真实仓库同步。
- WebDAV / S3-compatible 真实 provider 上传、恢复预览和保留策略。
- `/api/ops/status` 在真实生产配置下的状态。
- 目标 Agent 的 Skill 真实运行效果。
- Team Library 第一阶段浏览器完整流程验收。

## 发布判断

当前代码已经具备内部演示和 release candidate 评审条件。下一步适合进入“个人版 + Team Library 第一阶段”的小范围上线测试；仍不适合直接公开发布。公开发布前必须完成真实环境配置、Postgres 恢复演练、外部备份 provider 验证、生产状态检查和更完整的运营保护。
