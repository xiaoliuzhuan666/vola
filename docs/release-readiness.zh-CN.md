# Vola Release Readiness

更新时间：2026-06-05 13:04 CST

## 评估结论

当前 checkout 可以进入内部演示评审；小范围 self-hosted release candidate 可以继续准备，但不能写成已经完成生产验收。

本次验证证明的是：

- 本地 SQLite 临时环境可启动，关键页面可渲染，CLI smoke 可通过。
- 构建链路可通过，内嵌前端产物已刷新。
- Compose 的 Postgres 容器内连接端口已修正，宿主机端口变更不会影响 `server -> db` 内部连接。
- Rename 兼容审计通过：公开页面和 CLI 文档以 Vola / `neu` 为主，`vola`、`vol`、`neudrive`、`xlzdrive`、`NEUDRIVE_*`、`X-NeuDrive-*` 作为兼容入口保留并有测试覆盖。

本次没有证明的是：

- prod-like Postgres 数据库已经验证。
- Postgres dump 和临时库恢复已经完成。
- 真实 GitHub Backup、WebDAV 或 S3-compatible provider 已经上传、删除或恢复。
- 真实生产环境 `/api/ops/status` 健康。

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

- 修正 `docker-compose.yml`：`server` 容器连接 Postgres 时固定使用 `db:5432`，`POSTGRES_PORT` 只控制宿主机端口映射。
- 更新 `docs/deployment-reliability.zh-CN.md`：说明 Compose 内部端口和宿主机端口的区别。
- 完成 rename 兼容审计：README、CLI 文档、安装脚本、CLI 页面和部署说明都明确推荐 `neu`，并把旧入口写成兼容项。
- 更新 CLI help 渲染和 smoke 覆盖：`neudrive`、`vol`、`xlzdrive` 入口会显示自己的命令名，同时提示推荐入口是 `neu`。
- API source header 新增 `X-Vola-Platform` / `X-Vola-Source`，旧 `X-NeuDrive-Platform` / `X-NeuDrive-Source` 继续可用；webhook 同时发送 `X-Vola-*` 和 `X-NeuDrive-*`。
- Aliyun Flow 文档明确 `VOLA_IMAGE` 是新部署使用的镜像变量；脚本继续打印 `NEUDRIVE_IMAGE` 只是为了旧自动化兼容。
- 更新本文件：把验证记录改为 2026-06-05 的 rename 兼容审计结果。

## Verified

以下项目已在当前 checkout 实际执行：

| 项目 | 结果 | 证据摘要 |
| --- | --- | --- |
| 工作区状态 | 通过 | 开始前 `git status --short --branch` 显示当前分支 `codex/xlzdrive-desktop-app...xiaoliuzhuan-ssh/main` 且已有未提交改动；本次未 commit / push。 |
| 指定文件阅读 | 通过 | 已阅读 README、中文 README、命名调研、CLI/setup 文档、release readiness、Makefile、`cmd/vola`/`cmd/neu`/`cmd/vol`/`cmd/neudrive`、`internal/cli`、安装/测试脚本、env example、`web/src`、`internal/web/dist` 生成规则。 |
| 命名残留分类 | 通过 | 公开展示需改项已处理；兼容入口、历史迁移记录、SDK 旧 export、部署旧变量和测试 fixture 均保留。 |
| `go test ./...` | 通过 | 全仓 Go 测试通过；`internal/api`、`internal/cli`、`internal/services` 含新增兼容测试。 |
| `tools/test-neudrive-cli.sh` | 通过 | safe-smoke 总计 8 项，passed 8，failed 0；新增 `neu`、`vola`、`vol`、`neudrive`、`xlzdrive` help 渲染检查。 |
| `cd web && npm run build` | 通过 | `tsc && vite build` 成功；仍有 Vite chunk 大小 warning，非失败。 |
| `make build` | 通过 | `npm ci && npm run build` 成功，刷新 `internal/web/dist`，生成 `bin/vola`、`bin/vol`、`bin/neu`、`bin/neudrive`。 |
| `bin/* help` | 通过 | `bin/neu help`、`bin/vola help`、`bin/vol help`、`bin/neudrive help` 均成功，且都包含 `Recommended command name: neu.` 和对应命令名的 `status` 示例。 |
| `/api/health` | 通过 | 临时 SQLite 服务 `GET /api/health` 返回 HTTP 200，`service=vola`、`status=ok`、`storage=sqlite`。 |
| 浏览器 `/` | 通过 | 标题 `Home — Vola`，可见 Vola，未出现 neuDrive/NEUDRIVE/X-NeuDrive，route error 0，console error 0。 |
| 浏览器 `/cli` | 通过 | 标题 `Command Line Tools — Vola`，可见 `neu` 和兼容入口说明，未出现旧品牌误导，console error 0。 |
| 浏览器 `/setup/cli` | 通过 | 标题 `Setup Guide — Vola`，CLI setup 页面可渲染，未出现旧品牌误导，console error 0。 |
| 浏览器 `/guides/cli` 与 `/integrations/cli` | 通过 | 公开 CLI 指南和集成页可渲染；`/integrations/cli` 可见兼容入口说明；两页均未出现旧品牌误导，console error 0。 |

## Unverified

这些项目本次没有真实执行，不能写成通过：

- prod-like Postgres 验证：没有提供专用 Postgres `DATABASE_URL` / `VOLA_TEST_DB`。
- Postgres dump：没有生产或 prod-like 数据库凭据，未执行 `pg_dump`。
- 临时库恢复：没有 dump 文件和临时 Postgres 库，未执行 `pg_restore`。
- 真实 GitHub Backup：没有真实 GitHub App / repo 凭据，未创建仓库、同步或推送。
- 真实 WebDAV / S3-compatible provider：没有真实 provider 凭据，未验证上传、远端删除和保留策略。
- 真实生产环境：没有生产域名、生产 admin token、生产数据库和外部备份目标，未验证线上 `/api/health` 或 `/api/ops/status`。
- 目标 Agent 真实运行：没有在 Claude Code、Codex、Cursor、Gemini CLI 中逐平台执行真实 Skill。

## Blocked

- `docker build -t vola:rc .` 未通过：本机 Docker daemon 未运行。`docker build` 和 `docker info` 都失败，错误为无法连接 `unix:///Users/zhongmoshu/.docker/run/docker.sock`。
- 真实外部备份、GitHub App、生产数据库和生产 admin token 都需要凭据；本次按要求没有接触真实 secret。

## Human Follow-Up

进入小范围 self-hosted 试用前，建议由运维或项目负责人补齐：

1. 启动 Docker Desktop 或可用 Docker daemon 后重跑 `docker build -t vola:rc .`。
2. 准备 prod-like Postgres，执行 migrations，启动服务并记录 `/api/health`、`/api/ops/status`。
3. 生成一次 Postgres dump，并恢复到临时库验证登录、Skills、Memory、Projects、Vault scope。
4. 至少配置一个离开服务器的备份目标：GitHub Backup、WebDAV 或 S3-compatible。
5. 用真实 provider 验证上传、历史记录、恢复预览、恢复应用和保留策略。
6. 准备测试账号、测试团队和测试资料；不要用生产用户隐私数据做演示。

## Release 判断

| 场景 | 判断 |
| --- | --- |
| 内部演示评审 | 可以进入；使用本地 SQLite 临时环境或准备好的 self-hosted 环境演示，演示时说明备份和生产恢复边界。 |
| 小范围 self-hosted | 可以作为 RC 继续准备；正式放入真实用户数据前，需要补齐 Postgres、外部备份和恢复演练。 |
| 公开 SaaS 生产 | 不建议直接开放；仍缺真实恢复演练、生产运维状态、外部备份验证和告警链路。 |

## 不建议承诺

- 不承诺 GitHub Backup 是完整数据库灾备。
- 不承诺 WebDAV / S3 zip 可单独恢复整套服务。
- 不承诺 SQLite 可作为生产主存储。
- 不承诺全平台 Skill 自动同步。
- 不承诺复杂 Skill 转换后无需人工检查即可运行。
- 不承诺当前版本已经完成公开 SaaS 大规模生产验收。
