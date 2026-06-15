# Vola Release Readiness

更新时间：2026-06-14 19:09 CST

## 评估结论

当前 checkout 的本地构建、安全依赖审计、Docker 镜像、Compose + Postgres 临时环境、公开页 SEO 复查和 `driver.sunningfun.cn` 生产备份链路复查已通过。内部演示评审可以继续；当前这台 self-hosted 生产实例可以进入小范围试用，但不能写成已经完成公开 SaaS 大规模生产验收。

2026-06-14 补充：`https://driver.sunningfun.cn` 生产实例已完成 GitHub Backup、COS 外部备份、`/api/ops/status`、`/api/ops/instance-status`、页面实例状态卡片和一次临时环境恢复演练。恢复演练覆盖 `pg_dump`、`pg_restore`、临时 Vola 服务启动、Vola zip 导出、restore preview 和 restore apply。详细记录见 `docs/production-backup-configuration-runbook-2026-06-14.zh-CN.md`。

仍不能写成公开 SaaS 大规模生产验收已完成，因为自动备份还没有跨 24 小时观察，最近 COS 对象还没有直接下载后做 restore apply，生产 secret 解密也没有在临时环境用原 `VAULT_MASTER_KEY` 复验。

本次验证证明的是：

- 本地 SQLite 临时环境可启动，关键页面可渲染，CLI smoke 可通过。
- 构建链路可通过，内嵌前端产物已刷新。
- 前端安全依赖已升级，`npm audit --registry=https://registry.npmjs.org --audit-level=moderate` 显示 `found 0 vulnerabilities`。
- Compose 的 Postgres 容器内连接端口已修正，宿主机端口变更不会影响 `server -> db` 内部连接。
- Docker 镜像可在本机完成构建；临时 Compose 环境使用 Postgres 启动后，`/api/health` 返回 `storage=postgres`、`status=ok`。
- 生产镜像 `vola:staging-20260614-ops-instance-connfix` 已部署到 `driver.sunningfun.cn`，`/api/health` 返回 `status=ok`。
- 生产 `/api/ops/instance-status` 返回 `status=ok`，实例检查项全为 `ok`。
- 生产 `/sync-backup` 页面显示实例级备份状态，console error 为 0。
- 生产 `/api/connections` 历史 NULL 字段扫描问题已修复，接口返回 HTTP 200。
- 公开页服务端 HTML、运行时 canonical、robots 和 sitemap 已统一到 `https://www.vola.ai`。
- 配置加载会拒绝公开域名搭配开发默认 `JWT_SECRET`、`VAULT_MASTER_KEY` 或 `vola_dev` 数据库密码，降低误用本地 compose 默认值上线的风险。
- Rename 兼容审计通过：公开页面和 CLI 文档以 Vola / `neu` 为主，`vola`、`vol`、`neudrive`、`xlzdrive`、`NEUDRIVE_*`、`X-NeuDrive-*` 作为兼容入口保留并有测试覆盖。

本次没有证明的是：

- 自动备份已经跨 24 小时稳定触发。
- 最近 COS 对象直接下载后执行 restore apply。
- 使用原生产 `VAULT_MASTER_KEY` 在临时环境复验旧 secret 解密。
- 目标 Agent 真实运行：没有在 Claude Code、Codex、Cursor、Gemini CLI 中逐平台执行真实 Skill。

## 当前发布边界

Vola 的产品表达保持为：Agent 个人数据 Hub。它集中管理 profile、memory、projects、conversations、skills、vault 和 Agent 访问权限。

需要继续保持这些边界：

- GitHub Backup 只备份用户可见文件树，不包含 secret 明文，也不能替代 Postgres 备份。
- WebDAV / S3-compatible 目标上传的是 Vola export zip，适合离开服务器保存恢复包，不覆盖账号、session、billing、连接 token 等完整内部状态。
- SQLite 适合本地模式和临时验证；生产或长期 self-hosted 主存储应使用持久化 Postgres。
- Skill 自动写入只覆盖 Claude Code 和 Codex；Cursor 与 Gemini CLI 当前是可分配、可预览、可导出，不自动修改本地配置。
- Claude / Codex Skill 转换会保留脚本、依赖、assets 和外部引用；MCP、plugin、hook 仍需要人工检查。
- Team Library 当前按小团队共享资料库验收，不等同企业级组织管理、SSO、审批或审计产品。

## 本次修改

- 升级前端构建与路由相关依赖，修复 Vite 8 对结构化数据 URL 的构建要求，安全审计清零。
- 修正公开页后端 SEO 重写逻辑：canonical、Open Graph URL 和 structured data 从 `www.vola.cn` 改为 `www.vola.ai`，并同步 `internal/web` 测试。
- 增加公开域名部署安全检查：`PUBLIC_BASE_URL` 指向非本地地址时，拒绝开发默认 JWT、Vault master key 和 `vola_dev` 数据库密码。
- 更新团队 Skill 分享分析文档：后台更新检查已经接入服务调度，会写入团队提醒和成员个人通知文件。
- 修正 `docker-compose.yml`：`server` 容器连接 Postgres 时固定使用 `db:5432`，`POSTGRES_PORT` 只控制宿主机端口映射。
- 更新 `docs/deployment-reliability.zh-CN.md`：说明 Compose 内部端口和宿主机端口的区别。
- 完成 rename 兼容审计：README、CLI 文档、安装脚本、CLI 页面和部署说明都明确推荐 `neu`，并把旧入口写成兼容项。
- 更新 CLI help 渲染和 smoke 覆盖：`neudrive`、`vol`、`xlzdrive` 入口会显示自己的命令名，同时提示推荐入口是 `neu`。
- API source header 新增 `X-Vola-Platform` / `X-Vola-Source`，旧 `X-NeuDrive-Platform` / `X-NeuDrive-Source` 继续可用；webhook 同时发送 `X-Vola-*` 和 `X-NeuDrive-*`。
- Aliyun Flow 文档明确 `VOLA_IMAGE` 是新部署使用的镜像变量；脚本继续打印 `NEUDRIVE_IMAGE` 只是为了旧自动化兼容。
- 更新本文件：补充 2026-06-14 生产备份、恢复演练和实例状态验证结果。
- 新增实例级 `/api/ops/instance-status`，让管理员看整台实例的 GitHub Backup / 外部备份状态。
- `GitMirrorPage` 已优先显示实例级备份状态卡片。
- 修复 `/api/connections` 读取历史连接记录时遇到 `api_key_hash` 或 `api_key_prefix` 为 NULL 导致 HTTP 500 的问题。

## Verified

以下项目已在当前 checkout 实际执行：

| 项目 | 结果 | 证据摘要 |
| --- | --- | --- |
| 工作区状态 | 通过 | 开始前 `git status --short --branch` 显示当前分支 `codex/xlzdrive-desktop-app...xiaoliuzhuan-ssh/main` 且已有未提交改动；本次未 commit / push。 |
| 指定文件阅读 | 通过 | 已阅读 README、中文 README、命名调研、CLI/setup 文档、release readiness、Makefile、`cmd/vola`/`cmd/neu`/`cmd/vol`/`cmd/neudrive`、`internal/cli`、安装/测试脚本、env example、`web/src`、`internal/web/dist` 生成规则。 |
| 命名残留分类 | 通过 | 公开展示需改项已处理；兼容入口、历史迁移记录、SDK 旧 export、部署旧变量和测试 fixture 均保留。 |
| `go test ./...` | 通过 | 全仓 Go 测试通过；`internal/api`、`internal/cli`、`internal/services` 含新增兼容测试。 |
| `go test ./internal/config` | 通过 | 覆盖公开域名部署安全检查；本地地址允许开发默认值，公开地址会拒绝开发默认 secret 和 `vola_dev` 数据库密码。 |
| `npm audit --registry=https://registry.npmjs.org --audit-level=moderate` | 通过 | 安全审计返回 `found 0 vulnerabilities`。 |
| `tools/test-neudrive-cli.sh` | 通过 | safe-smoke 总计 8 项，passed 8，failed 0；新增 `neu`、`vola`、`vol`、`neudrive`、`xlzdrive` help 渲染检查。 |
| `cd web && npm run build` | 通过 | `tsc && vite build` 成功；仍有 Vite chunk 大小 warning，非失败。 |
| 生产镜像 | 通过 | `vola:staging-20260614-ops-instance-connfix` 架构为 `linux/amd64`，镜像 ID `sha256:1bc920eec3d9cab1e39f3460dd1a7df73a94dbe5612a8917cabc7031cb2d69c1`。 |
| 生产 `/api/health` | 通过 | `https://driver.sunningfun.cn/api/health` 返回 `service=vola`、`status=ok`、`storage=postgres`。 |
| 生产 `/api/ops/instance-status` | 通过 | 返回 `status=ok`；`users_total=2`，`users_with_git_backup=1`，`users_with_external_backup=1`，`users_with_remote_backup_artifact=1`，`users_with_critical_backup_status=0`。 |
| 生产 `/api/connections` | 通过 | 短效 admin token 调用返回 HTTP 200；修复前该接口因历史 NULL 字段返回 500。 |
| 生产浏览器 `/sync-backup` | 通过 | 标题 `GitHub Backup — Vola`；可见 `实例备份状态正常`、`实例用户：1/2`、`外部备份用户：1/2`；console error 0。 |
| `make build` | 通过 | `npm ci && npm run build` 成功，刷新 `internal/web/dist`，生成 `bin/vola`、`bin/vol`、`bin/neu`、`bin/neudrive`。 |
| `docker build -t vola:local-readiness .` | 通过 | 当前 checkout 可构建 Linux runtime 镜像。 |
| 临时 Compose Postgres 环境 | 通过 | `COMPOSE_PROJECT_NAME=vola_readiness PORT=18080 POSTGRES_PORT=55432 docker compose up -d --build --force-recreate` 后，`db` 为 healthy，`server` 启动成功。 |
| `bin/* help` | 通过 | `bin/neu help`、`bin/vola help`、`bin/vol help`、`bin/neudrive help` 均成功，且都包含 `Recommended command name: neu.` 和对应命令名的 `status` 示例。 |
| `/api/health` | 通过 | 临时 SQLite 服务 `GET /api/health` 返回 HTTP 200，`service=vola`、`status=ok`、`storage=sqlite`。 |
| 临时 Compose `/api/health` | 通过 | `GET http://127.0.0.1:18080/api/health` 返回 HTTP 200，`service=vola`、`status=ok`、`storage=postgres`。 |
| 公开页 SEO | 通过 | 服务端 HTML canonical、Open Graph URL、structured data 和 sitemap 均指向 `https://www.vola.ai`；仓库公开页源码和生成产物中未发现 `vola.cn` 残留。 |
| 浏览器 `/` | 通过 | 标题 `Home — Vola`，可见 Vola，未出现 neuDrive/NEUDRIVE/X-NeuDrive，route error 0，console error 0。 |
| 临时 Compose 浏览器 `/` | 通过 | 标题 `Agent 个人数据 Hub — Vola`，H1 `把 AI 工作资料放在你名下`，`#root` 有内容，console error 0。 |
| 浏览器 `/cli` | 通过 | 标题 `Command Line Tools — Vola`，可见 `neu` 和兼容入口说明，未出现旧品牌误导，console error 0。 |
| 浏览器 `/setup/cli` | 通过 | 标题 `Setup Guide — Vola`，CLI setup 页面可渲染，未出现旧品牌误导，console error 0。 |
| 浏览器 `/guides/cli` 与 `/integrations/cli` | 通过 | 公开 CLI 指南和集成页可渲染；`/integrations/cli` 可见兼容入口说明；两页均未出现旧品牌误导，console error 0。 |

## Unverified

这些项目本次没有真实执行，不能写成通过：

- 自动备份跨 24 小时观察：当前只确认目标配置、手动上传和后台任务未到期跳过。
- 直接 COS 对象恢复：还没有把最近对象 `vola/neudrive-export-20260614-090115Z.zip` 直接下载后执行 restore apply。
- 生产 secret 解密复验：临时恢复环境没有使用原生产 `VAULT_MASTER_KEY`。
- 目标 Agent 真实运行：没有在 Claude Code、Codex、Cursor、Gemini CLI 中逐平台执行真实 Skill。

## Blocked

- 当前本机 Docker 构建、临时 Compose 验证、生产部署和生产备份状态验证已通过。
- 剩余验证需要等待时间窗口或使用受控 secret：自动备份跨 24 小时观察、直接 COS 对象恢复、原 `VAULT_MASTER_KEY` 解密复验。

## Human Follow-Up

继续扩大 self-hosted 试用前，建议由运维或项目负责人补齐：

1. 观察下一次自动备份是否按 24 小时计划产生新对象和历史记录。
2. 从 COS 直接下载最近对象，做一次 restore preview/apply。
3. 在临时恢复环境使用原 `VAULT_MASTER_KEY` 和 `JWT_SECRET`，复验旧 secret 解密和登录。
4. 把恢复演练脚本化，减少人工步骤。
5. 准备测试账号、测试团队和测试资料；不要用生产用户隐私数据做演示。

## Release 判断

| 场景 | 判断 |
| --- | --- |
| 内部演示评审 | 可以进入；使用本地 SQLite 临时环境或准备好的 self-hosted 环境演示，演示时说明备份和生产恢复边界。 |
| 小范围 self-hosted | 当前 `driver.sunningfun.cn` 实例可以进入小范围试用；需要继续观察自动备份并补直接 COS 恢复验证。 |
| 公开 SaaS 生产 | 不建议直接开放；仍缺自动备份长期观察、告警链路、原 secret 解密复验和更完整的多用户恢复演练。 |

## 不建议承诺

- 不承诺 GitHub Backup 是完整数据库灾备。
- 不承诺 WebDAV / S3 zip 可单独恢复整套服务。
- 不承诺 SQLite 可作为生产主存储。
- 不承诺全平台 Skill 自动同步。
- 不承诺复杂 Skill 转换后无需人工检查即可运行。
- 不承诺当前版本已经完成公开 SaaS 大规模生产验收。
