# 技能名称：本地 MCP 客户端健壮性诊断与装配管理 (local-mcp-management-robustness)

本技能用于在本地开发环境中，辅助开发人员及 AI 助手（如 Cursor, Claude Code, Trae, Codebuddy, Workbuddy 等）并发安全地装配 MCP 服务，诊断共享 HTTP MCP 的网络存活状态与延迟，清理失效配置，处理主配置自愈、安全日志脱敏排查、账号登录无感静默续期以及团队技能防越权 Zip Slip 解压穿越。

## 适用场景

1. **环境连通性诊断**：当 AI 客户端报错无法连接到团队的某个共享 MCP，或怀疑本地配置文件冲突时。
2. **并发修改 mcp.json 配置**：当多个客户端需要并发读写，需要获取进程锁，防止文件内容被损坏。
3. **主配置与 mcp.json 损坏自愈**：主配置 `config.json` 或 IDE 的 `mcp.json` 因磁盘满等极端崩溃损坏后的一键自动还原与恢复。
4. **日志隐私脱敏加固**：保障在排查诊断本地服务时，API Keys、Password、Bearer 授权 Token 不在日志中泄露。
5. **SSE 重连历史重放补发**：处理断网恢复后瞬间变更事件的定向补发对齐。
6. **账号登录无感静默续期**：当 API 访问或 SSE 连接遭遇 401 凭证过期时，系统自动使用本地 Refresh Token 换取新 Token 并无感重试。
7. **团队技能回滚 Zip Slip 安全防线**：在技能从备份 zip 解压还原时，安全拦截并过滤包含目录穿越（如 `../`）的恶意文件，严防越权写穿沙箱。

## 使用指南

直接让您的 AI 助手调用此技能下的诊断工具，或根据以下规程进行排查：

### 1. 自动损坏故障自愈 (Self-Healing)
* **config.json / mcp.json 恢复规程**：
  * Vola 本地客户端会在每次成功保存配置后自动镜像备份一份 `.vola.bak` 文件。
  * 当客户端加载配置发生 JSON 语法格式损坏（例如断电写了一半）时，系统已内建故障自动恢复逻辑，会自动从最近的备份文件还原覆盖，完成自愈。
  * 用户或 AI 助手如需手动干预，可在同级目录下直接将 `.vola.bak` 覆盖原 JSON 文件。

### 2. 敏感凭证日志脱敏规程
* **安全排查规范**：
  * 系统已在 slog 日志处理管道中挂载了 `ReplaceAttr` 全局敏感字段拦截过滤。
  * 凡是键名为 `token`、`password`、`secret`、`jwt`、`authorization` 等，以及值带有 `Bearer ` 头的日志输出，均会自动被替换为 `"[MASKED]"`。
  * AI 助手在审查本地日志以排查 daemon 运行问题时，无需担心在 log 中暴露出团队的私有敏感凭证。

### 3. 运行连通性与延迟诊断脚本 (diagnose-mcp.sh)
本技能附带了一个轻量级诊断脚本 `diagnose-mcp.sh`，直接运行即可自动输出本机所装配的 Cursor、Trae、Codebuddy 和 Workbuddy 等编辑器的本地状态，并并发测出所有配置的团队共享 HTTP MCP 服务器是否在线以及延迟毫秒数。

### 4. 账号登录 401 拦截无感自愈规程
* **客户端静默刷新**：
  * 当客户端因 Access Token 过期收到 401 Unauthorized 错误时，系统将在后台自动截获。
  * 守护进程会使用本地 `config.json` 的 `RefreshToken` 向 `/api/auth/refresh` 发起刷新。
  * 获得新 Token 后，将自动写回本地 `config.json` 并使用新凭证自动重试先前失败的请求（如团队列表拉取或 SSE 重建），用户无须重新登录即可无感维持在线状态。

### 5. 团队技能安全解包与 Zip Slip 防御校验规程
* **沙箱前缀保护规程**：
  * 在团队对所订阅的共享 Skill 进行回滚或更新时，若解包的 Zip 文件中含有恶意的相对路径（`../`）或绝对路径穿越攻击，解压算法会运用 `path.Clean` 彻底清洗路径，并核对拼接后的路径是否符合技能物理路径沙箱前缀。
  * 任何跨越沙箱前缀的威胁文件将被立即跳过，而其余合规的技能文件仍将被安全还原并记录，从根本上防止外部恶意包覆盖宿主机的敏感关键文件。

---

## 环境变量要求

* `VOLA_CONFIG`：Vola 本地客户端配置文件路径。
* `HOME`：当前用户主目录。

---

## 技能定义元数据 (manifest.vola.json)

```json
{
  "name": "local-mcp-management-robustness",
  "version": "1.2.0",
  "description": "Diagnose local MCP client connections, currency locks, handle automatic backups & restoration, log masking, SSE event replay, silent token rotation, and zip slip protection.",
  "entrypoint": "diagnose-mcp.sh",
  "target_agents": ["claude-code", "codex", "cursor", "gemini-cli"],
  "permissions": [
    "read-local-config",
    "write-local-config"
  ]
}
```
