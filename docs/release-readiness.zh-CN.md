# neuDrive Release Readiness

更新时间：2026-05-13

## 评估范围

本次评估面向当前工作区，把 neuDrive 整理为可评审、可演示、可部署的 release candidate。重点检查：

- Agent 个人数据 Hub 的产品表达是否清楚。
- README、部署文档、备份恢复文档、阶段计划是否一致。
- 本地开发、测试、构建、部署命令是否可执行或有明确限制。
- GitHub Backup、WebDAV / S3-compatible 备份、Skill 导入、Skill 分配、Claude / Codex 转换、`/api/ops/status` 是否有可验证说明。

## 工作区状态

`AGENTS.md`：工作区根目录没有该文件；本次按用户消息中提供的 AGENTS 约束执行。

已有改动按功能域分为：

| 类型 | 文件或目录 | 处理建议 |
| --- | --- | --- |
| 产品与文档 | `README.md`、`README.zh-CN.md`、`docs/github-backup*.md`、`docs/deployment-reliability.zh-CN.md`、`docs/agent-data-hub-iteration-plan.zh-CN.md`、`docs/agent-skill-targets.zh-CN.md` | 纳入评审 |
| 部署配置 | `deploy/k8s/app.yaml`、`deploy/prod/README.md`、`deploy/prod/deploy.sh`、`docker-compose.yml`、`neudrive.env.example` | 纳入评审 |
| 备份恢复 | `internal/api/backup_*`、`internal/backups/`、`internal/storage/sqlite/backup_targets.go`、`migrations/021_*`、`022_*`、`023_*`、`internal/jobs/scheduler.go` | 纳入评审 |
| Skill Hub | `internal/api/local_skill_sync.go`、`skill_assignments.go`、`skill_conversion.go`、`internal/skillsarchive/manifest.go`、`web/src/pages/data/DataSkillsPage.tsx`、`web/src/pages/SkillsImportPage.tsx` | 纳入评审 |
| 页面状态 | `web/src/api.ts`、`web/src/pages/GitMirrorPage.tsx`、`DashboardPage.tsx`、`OnboardingPage.tsx`、`PublicPages.tsx`、`web/src/index.css` | 纳入评审 |
| 测试 | `internal/api/sqlite_shared_test.go`、`internal/platforms/claude_migration_test.go`、`internal/skillsarchive/archive_test.go` | 纳入评审 |
| 截图与阶段记录 | `neudrive-local-home.png`、`stage3-skill-manifest-upload.png`、`stage4-*.png`、`sync-backup-*.png`、`stage4-*.md` | 建议作为演示证据单独归档，不直接混入 release commit |
| 临时输出 | `.playwright-mcp/`、`console-stage4*.txt` | 建议归档或忽略；本次不删除 |

## 当前能力

### Agent 个人数据 Hub

- 支持 profile、memory、projects、conversations、skills、vault、connections 等数据面。
- 提供 Web UI、HTTP API、MCP 工具和 CLI 接入路径。
- 通过 scoped token 和 trust level 控制 Agent 访问范围。

### Skill 管理

- Skill 导入会生成 `manifest.neudrive.json`。
- manifest 记录入口文件、脚本、依赖文件、assets、二进制、环境变量、外部 Claude tools/plugins 引用。
- Claude Code / Codex 支持本地同步预览、应用和清理。
- Cursor / Gemini CLI 当前是“可分配、可导出、暂不自动写入”。
- Claude Code / Codex 支持转换预览和生成副本；复杂 Skill 的 MCP、plugin、hook 只生成报告。

### 备份与恢复

- GitHub Backup 保存用户可见文件树的 Git 版本历史。
- WebDAV / S3-compatible / OSS / R2 通过 S3-compatible endpoint 上传 neuDrive 导出 zip。
- 外部备份目标支持手动运行、自动计划、运行历史、最近失败、保留策略。
- ZIP 恢复支持预览、跳过已有文件、覆盖已有文件，并拒绝路径穿越。
- Postgres dump 仍是完整服务恢复的必要条件。

### 运维状态

- `/api/health` 用于服务存活检查。
- `/api/ops/status` 用于管理员查看主存储、Git mirror、GitHub Backup、外部备份、最近成功和最近错误。
- 页面已在 GitHub Backup 区域展示备份历史、最近失败、自动备份状态、保留策略、恢复预览和恢复结果。

## 部署前检查

部署前必须确认：

1. `PUBLIC_BASE_URL` 是最终访问域名。
2. `JWT_SECRET` 和 `VAULT_MASTER_KEY` 已保存在安全位置，恢复时继续使用原值。
3. `DATABASE_URL` 指向持久化 Postgres；不要把单机 SQLite 当成生产主存储。
4. `GIT_MIRROR_HOSTED_ROOT` 已配置并挂载可写持久卷，推荐 `/data/git-mirrors`。
5. Hosted GitHub Backup 需要 `GITHUB_APP_CLIENT_ID`、`GITHUB_APP_CLIENT_SECRET`、`GITHUB_APP_SLUG`。
6. 至少配置一个离开当前服务器的备份目标：GitHub、WebDAV 或 S3-compatible。
7. 至少完成一次 Postgres dump。
8. 至少在临时环境做一次恢复演练。
9. `/api/ops/status` 返回不是 `critical`。
10. 明确记录外部备份目标的保留策略和最近成功对象名。

## 本地开发命令

```bash
cp neudrive.env.example neudrive.env
set -a; source neudrive.env; set +a
go run ./cmd/neudrive server --listen :8080
cd web && npm run dev
```

前端开发服务：`http://localhost:3000`

后端 API：`http://localhost:8080`

## 构建与测试命令

```bash
go test ./...
cd web && npm run build
make build
docker build -t neudrive:rc .
```

如果 Docker build 因 registry、镜像下载或网络问题失败，不应写成构建成功；记录失败摘要即可。

## 演示脚本

### 首页

1. 打开 `http://localhost:3000/`。
2. 检查首屏是否表达为 Agent 个人数据 Hub。
3. 检查 GitHub Backup、Skills、Vault、MCP、CLI 等入口没有夸大承诺。

### Dashboard

1. 使用本地 owner token 或登录流程进入后台。
2. 打开 Dashboard。
3. 检查 Profile、Memory、Projects、Skills、Connections 等模块是否能看到。

### GitHub Backup

1. 打开 `GitHub 备份` 页面。
2. 检查数据位置说明、GitHub 远端仓库状态、WebDAV / S3-compatible 目标。
3. 检查备份历史、最近失败、自动备份、保留策略、恢复预览和恢复应用入口。
4. 不在演示中承诺 GitHub Backup 等同数据库备份。

### Skills

1. 打开 `Skills` 页面。
2. 导入一个包含 `SKILL.md`、scripts、requirements、assets 的 Skill。
3. 查看 manifest 和风险提示。
4. 在 Agent 分配表中分配给 Claude Code、Codex、Cursor、Gemini CLI。
5. 预览本地同步：Claude / Codex 可写入；Cursor / Gemini CLI 可导出但不自动写入。
6. 预览 Claude Code 与 Codex 的转换报告。

### 运维状态

```bash
curl -fsS "$PUBLIC_BASE_URL/api/ops/status" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

演示时说明：`/api/ops/status` 只能说明当前配置和最近运行状态，不能替代真实恢复演练。

## 已完成

- 产品表达已从 Skill 备份工具扩展为 Agent 个人数据 Hub。
- GitHub Backup 与 WebDAV / S3-compatible 外部备份进入页面和 API。
- 外部备份支持自动计划、运行历史、最近失败、保留策略。
- ZIP 恢复支持预览、跳过已有文件、覆盖已有文件。
- Skill 导入支持 manifest 和外部 Claude tools/plugins 纳入。
- Agent Skill 分配覆盖 Claude Code、Codex、Cursor、Gemini CLI。
- Claude Code / Codex 支持本地同步与转换。
- `/api/ops/status` 能展示备份和运维状态。
- K8s 和 prod 脚本已对齐 Git mirror PVC 和 GitHub App 环境变量。
- Docker Compose 已配置 Postgres 主存储和 Git mirror 持久 volume。

## 已知限制

- GitHub Backup 不保存 secret 明文，不能替代数据库备份。
- WebDAV / S3-compatible zip 不包含完整账号、session、billing、连接 token 状态。
- Vault 恢复只恢复范围清单，secret 原值依赖数据库备份或密钥系统。
- Cursor / Gemini CLI 不自动写入本地配置，只提供分配、预览和导出包。
- MCP、plugin、hook 不自动启用，转换报告只提示人工处理项。
- 目标 Agent 的 Skill 真实运行效果还没有逐平台自动化验证。
- 外部备份保留策略的远端删除需要对坚果云、R2、OSS、MinIO 等真实服务做兼容性验证。
- 告警通道尚未接入，当前只能通过页面和 `/api/ops/status` 查看异常。
- 多副本 hosted 部署需要额外处理 Git mirror worker 与共享卷并发问题。

## 不建议承诺

- 不建议承诺“全平台 Skill 自动同步”。
- 不建议承诺“GitHub Backup 就是完整灾备”。
- 不建议承诺“WebDAV / S3 zip 可以单独恢复整个服务”。
- 不建议承诺“复杂 Skill 转换后无需人工检查即可运行”。
- 不建议承诺“当前版本已适合公开 SaaS 大规模生产”。

## 验证记录

2026-05-13 验证结果：

- 文档和前端文案检查：README、部署可靠性文档、GitHub Backup 文档、Skill 目标规则和主要页面未发现“全平台 Skill 自动同步”“GitHub Backup 等同完整灾备”“SQLite 可作为生产主存储”这类承诺；价格页已把“自动同步”改为“托管同步/自动备份”，避免被误读为所有 Agent Skill 自动写入。
- API / 前端入口检查：`internal/api/router.go`、`web/src/api.ts`、`web/src/pages/GitMirrorPage.tsx`、`web/src/pages/data/DataSkillsPage.tsx` 对应到了 `/api/backup/*`、`/api/ops/status`、Skill 分配、本地同步、转换预览和导出入口。
- 部署配置检查：K8s 和 prod 脚本已有 `GIT_MIRROR_HOSTED_ROOT=/data/git-mirrors` 与持久卷；本次把 `docker-compose.yml` 也对齐为 `gitmirrors:/data/git-mirrors`，并在 `docs/deployment-reliability.zh-CN.md` 增加 Compose 持久化说明。
- `go test ./...`：通过。
- `cd web && npm run build`：通过。Vite 仍提示 `WorkbenchCodeEditor` 和主入口 chunk 超过 500 KB，这是体积提醒，不是构建失败。
- `make build`：通过。已刷新 `internal/web/dist`，并生成 `bin/neudrive`、`bin/neu`。
- 本地服务验证：首次在普通沙箱启动 `./bin/neudrive server --storage sqlite --sqlite-path /private/tmp/neudrive-rc-20260513.db --listen 127.0.0.1:42701 --local-mode ...` 失败，错误为 `listen tcp 127.0.0.1:42701: bind: operation not permitted`；获得权限后同一服务启动成功。
- `curl -fsS http://127.0.0.1:42701/api/health`：通过，返回 `storage: sqlite`。
- 首页静态 HTML 验证：`curl -fsS http://127.0.0.1:42701/` 返回 `Personal data hub for AI agents — neuDrive`，description 为 Agent 个人数据 Hub 表达。
- 页面渲染验证：内置浏览器检查两次超时，随后用 Playwright 打开 `/`、`/team`、`/sync-backup`、`/skills`，页面标题和关键内容可见，没有 route error；`/sync-backup` 和 `/skills` 控制台 error 数为 0。
- `/api/ops/status`：不带 token 的 GET 返回 401；使用本地 owner admin token 调用返回 `status: warning`，原因是临时 SQLite 环境没有配置 GitHub Backup 或 WebDAV / S3-compatible 目标。这符合本地临时验证环境预期，不代表生产状态。
- 未执行 `docker build -t neudrive:rc .`；本次没有验证 Docker 镜像构建。
- 仍未验证：Postgres dump 与临时库恢复、真实 GitHub 仓库同步、真实 WebDAV / S3-compatible provider 上传/删除兼容性、生产配置下的 `/api/ops/status`、目标 Agent 的 Skill 真实运行效果。

2026-05-12 验证结果：

- 已合入 `origin/main` 的 `2f255a1 Hide browser extension from user-facing surfaces`，并处理 README、Dashboard、PublicPages 的交集改动。
- `go test ./...`：通过。
- `cd web && npm run build`：通过。Vite 提示 `WorkbenchCodeEditor` 和主入口 chunk 超过 500 KB，这是体积提醒，不是构建失败。
- `make build`：通过。已用最后一次前端构建刷新 `internal/web/dist`，并生成 `bin/neudrive`、`bin/neu`。
- `docker build -t neudrive:rc .`：通过，最终镜像 sha256 为 `d4af768d960d0ad792c2161ad94fdb0b3650a53da625e4d1812bc06746c476a0`。
- 本地服务验证：用 SQLite 临时库启动，`curl -fsS http://127.0.0.1:42690/api/health` 返回 `storage: sqlite`。
- 首页静态 HTML 验证：`curl -fsS http://127.0.0.1:42690/` 返回 `Personal data hub for AI agents — neuDrive`，description 已更新为 Agent 个人数据 Hub 表达。
- 页面冒烟验证：Playwright 打开 `/`、`/team`、`/sync-backup`、`/skills`，页面可渲染，没有 route error，控制台没有 error。
- `/api/ops/status`：直接不带 token `curl` 返回 401，符合鉴权预期；真实生产配置下的管理员状态未验证。
- browser extension 对外展示检查：README、主要 docs、`web/src`、`internal/web` 中未发现公开展示文案残留；执行计划文档中的相关词只用于记录本次合入检查背景。

## 当前判断

| 场景 | 分数 | 判断 |
| --- | --- | --- |
| 内部演示 | 88 / 100 | 可用；需避免承诺自动化验证尚未覆盖的目标 Agent 运行效果 |
| 小范围 self-hosted | 78 / 100 | 可试用；前提是配置 Postgres、外部备份和恢复演练 |
| 公开 SaaS | 62 / 100 | 不建议直接开放；仍缺告警、真实恢复演练记录、多副本可靠性和更完整的运营保护 |
