# Vola 小范围上线验收记录

验收日期：2026-05-18

## 结论

**当前结论：被外部条件阻塞，暂不建议直接进入小范围测试。**

代码、页面和本地临时环境已经完成一轮验收，当前没有发现“文档写得比实现更强”的新增问题；但按 `docs/launch-test-checklist.zh-CN.md` 的发布条件，仍缺少生产同构证据：

- 真实 Postgres 主存储启动与恢复演练。
- 至少一个真实离开服务器的备份目标。
- 真实 GitHub Backup 推送。
- 真实 WebDAV / S3-compatible provider 上传与删除兼容性。
- 生产配置下的 `/api/ops/status` 结果。
- Team Library 第一阶段的完整主流程验收。
- Claude Code / Codex 中至少一个真实 Skill 的实际运行验证。

这些项不是本地 SQLite 冒烟测试能替代的内容。补齐前，只能说明当前版本具备继续做小范围上线准备的基础，不能说明已经满足小范围测试放量条件。

## 本次范围

本次按以下文件和功能做验收：

- `README.zh-CN.md`
- `docs/release-readiness.zh-CN.md`
- `docs/launch-test-checklist.zh-CN.md`
- `docs/deployment-reliability.zh-CN.md`
- `docs/github-backup.zh-CN.md`
- Agent 个人数据 Hub 的文案边界
- GitHub Backup、WebDAV / S3-compatible、恢复预览、恢复应用、`/api/ops/status`
- Skill 导入、manifest、Agent 分配、本地同步、Claude / Codex 转换、Cursor / Gemini CLI 导出边界
- Team Library 第一阶段表达
- 部署配置中的 Postgres、`JWT_SECRET`、`VAULT_MASTER_KEY`、`GIT_MIRROR_HOSTED_ROOT`、`USER_STORAGE_QUOTA_BYTES`、外部备份目标

验收开始前执行了 `git status --short --branch`。当前工作区已有大量修改、删除和新增文件，全部按用户已有工作处理；本次只新增本记录，不还原、不删除、不格式化其他文件。

## 已验证

### 1. 文档和页面文案

- README 仍以 **Agent 个人数据 Hub** 为主表达。
- README、部署可靠性文档、GitHub Backup 文档都明确写到：
  - GitHub Backup 只覆盖用户可见文件树。
  - WebDAV / S3-compatible 上传的是 Vola 导出 zip。
  - 完整恢复仍需要数据库备份，尤其是 Postgres dump，以及原 `JWT_SECRET`、`VAULT_MASTER_KEY`。
  - SQLite 只适合本地模式，不应当作生产主存储。
- Team Library 仍按“小团队共享资料库”表达，未写成企业协作平台；页面文案也明确排除了审计、SSO、审批流。
- Skills 页面与 `docs/agent-skill-targets.zh-CN.md` 一致：
  - Claude Code 写 `~/.claude/skills`
  - Codex 写 `~/.codex/skills`
  - Cursor / Gemini CLI 只保留分配和导出包，不自动写入本地配置

### 2. 页面 / API 与真实实现对应

已对照以下实现：

- `internal/api/router.go`
- `internal/api/backup_restore.go`
- `internal/api/backup_targets.go`
- `internal/api/ops_status.go`
- `internal/backups/service.go`
- `internal/backups/webdav.go`
- `internal/backups/s3.go`
- `web/src/api.ts`
- `web/src/pages/GitMirrorPage.tsx`
- `web/src/pages/data/DataSkillsPage.tsx`
- `web/src/pages/TeamLibraryPage.tsx`

对应关系成立：

- `/api/backup/targets`
- `/api/backup/runs`
- `/api/backup/restore/preview`
- `/api/backup/restore/apply`
- `/api/ops/status`
- Skill assignments / local sync / export / conversion
- Team Library 页面和团队 API

### 3. 部署配置

已检查：

- `docker-compose.yml`
- `vola.env.example`
- `deploy/prod/README.md`

可确认：

- Compose 使用 Postgres 作为主存储。
- 已配置 `JWT_SECRET`、`VAULT_MASTER_KEY`、`GIT_MIRROR_HOSTED_ROOT`、`USER_STORAGE_QUOTA_BYTES`。
- `gitmirrors` volume 已挂载到 `/data/git-mirrors`。
- 生产文档要求离开服务器的备份目标，并把 Postgres dump、密钥和恢复演练单独列为必须项。

### 4. 构建与测试

执行结果：

- `git status --short --branch`：已执行，确认工作区有大量已有改动。
- `GOCACHE=/private/tmp/vola-go-cache go test ./...`
  - 首次失败，原文为：
    - `httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted`
    - `listen tcp 127.0.0.1:0: bind: operation not permitted`
  - 获得本机端口监听权限后重试，通过。
- `cd web && npm run build`：通过。
- `make build`：通过，已刷新嵌入式前端产物并生成 `bin/vola`、`bin/neu`。
- `docker build -t vola:launch-acceptance-20260518 .`：通过，镜像 ID 为 `0d78e3f39811`。

Vite 仍提示两个 chunk 大于 500 KB，这是体积提醒，不是构建失败。

### 5. 本地 SQLite 临时服务

使用临时 SQLite 服务验证：

- `GET /api/health`：通过，返回 `storage: sqlite`。
- 首页 HTML：通过，`title` 为 `Personal data hub for AI agents — Vola`。
- 页面渲染：
  - `/`
  - `/team`
  - `/sync-backup`
  - `/skills`
  - 四个页面均能渲染，浏览器控制台 error 数为 0。
- `/api/ops/status`
  - 无 token 返回 401。
  - 使用本地 owner token 返回 `status: warning`。
  - warning 原因是本地临时环境没有 GitHub Backup 仓库；这符合临时环境，不代表生产环境。

### 6. 备份与恢复链路

在临时 SQLite 环境中验证：

- `POST /api/backup/restore/preview`
  - 能识别 Vola export zip。
  - 返回分类：`identity`、`scratch`、`skills`、`unknown`。
- `POST /api/backup/restore/apply?mode=skip`
  - 成功执行。
  - 本次结果：`applied: 2`、`skipped: 27`、`overwritten: 0`。
- WebDAV 本地接收器
  - 收到 `PUT /webdav/vola-export-20260518-115920Z.zip`
  - 带 Basic auth
- S3-compatible 本地接收器
  - 收到 path-style `PUT /bucket/acceptance/vola-export-20260518-115948Z.zip`
  - 带 AWS4 签名
- `GET /api/backup/runs`
  - 能看到 WebDAV 和 S3 两条 success run。
- 再次调用 `/api/ops/status`
  - 外部备份目标状态为 ok
  - `remote_backup_artifact` 为 ok
  - 整体仍为 `warning`，只剩 GitHub Backup 仓库未配置

## 未验证

- 真实 Postgres 主存储启动、迁移、写入和恢复。
- Postgres dump 生成与临时库恢复。
- 真实 GitHub Backup 创建仓库、同步和推送。
- 真实 WebDAV / S3-compatible provider 的上传、远端删除、保留策略兼容性。
- 真实生产配置下的 `/api/ops/status`。
- Team Library 第一阶段完整主流程：
  - 创建团队
  - 添加成员
  - 角色读写
  - 多团队切换
  - 团队 Skill
  - Agent `scope=team`
  - 团队资料进入真实备份
- 复杂 Skill 从导入到 `manifest.vola.json`、Agent 分配、本地同步、转换报告的本次实际操作复验。
- Claude Code / Codex 中真实执行一个 Skill。
- 多副本 hosted 部署下 Git mirror worker 与共享卷并发行为。

## 阻塞项

### 外部条件阻塞

要把结论从“被外部条件阻塞”推进到“可以进入小范围测试”，还需要：

1. 一套生产同构或 prod-like Postgres 环境。
2. 可用的真实 GitHub 凭据和专用备份仓库。
3. 可用的真实 WebDAV 或 S3-compatible provider 凭据。
4. 一次可记录的 Postgres dump 与恢复演练。
5. 可用于真实 Skill 运行验证的 Claude Code / Codex 环境。

### 环境限制原文

- Go 测试首次在普通沙箱中失败：
  - `httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted`
  - `listen tcp 127.0.0.1:0: bind: operation not permitted`
- 本机临时接收器首次在普通沙箱中失败：
  - `Error: listen EPERM: operation not permitted 127.0.0.1:42818`

这两类问题在获得本机监听权限后都能继续验证，不属于代码失败。

## 不建议承诺

- 不建议承诺“已经可以直接进入小范围测试”。
- 不建议承诺“GitHub Backup 就是完整数据库灾备”。
- 不建议承诺“WebDAV / S3-compatible zip 可以单独恢复整套服务”。
- 不建议承诺“Cursor / Gemini CLI 已支持本地自动同步”。
- 不建议承诺“SQLite 已可作为生产主存储”。
- 不建议承诺“Team Library 已等同企业协作平台”。
- 不建议承诺“复杂 Skill 转换后无需人工检查即可运行”。

## 下一步建议

1. 在 prod-like Postgres 环境执行：
   - 服务启动
   - 登录
   - 文件树写入
   - Postgres dump
   - 临时库恢复
2. 用真实 provider 复跑：
   - GitHub Backup
   - WebDAV 或 S3-compatible 上传
   - 保留策略删除
   - 恢复预览
   - 恢复应用
3. 跑 Team Library 第一阶段完整清单，不只做页面渲染。
4. 选一个简单 Skill，在 Claude Code 与 Codex 各真实运行一次。
5. 完成后再补一份基于 prod-like / 真实 provider 的验收记录；到那时才适合给出“可以进入小范围测试”的结论。
