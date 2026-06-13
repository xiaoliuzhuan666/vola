# 技能名称：本地 MCP 客户端健壮性诊断与装配管理 (local-mcp-management-robustness)

本技能用于在本地开发环境中，辅助开发人员及 AI 助手（如 Cursor, Claude Code, Trae 等）并发安全地装配 MCP 服务，诊断共享 HTTP MCP 的网络存活状态与延迟，清理失效配置，并提供一键式备份与灾备恢复。

## 适用场景

1. **环境诊断**：当 AI 客户端报错无法连接到团队的某个共享 MCP，或怀疑本地配置文件冲突时。
2. **安全修改配置**：当需要在本地 `mcp.json` 中并发读写，需要获取进程锁，防止文件内容被损坏。
3. **标签高精裁剪**：项目级的按需配置装配，减少 AI 提示词 Token 的无谓损耗。
4. **备份与灾备恢复**：首次修改前的冷备份，以及当本地配置损坏时的一键还原。

## 使用指南

直接让您的 AI 助手调用此技能下的诊断工具，或根据以下规程进行排查：

### 1. 并发安全地读写或修改配置文件
* **规范**：在修改 IDE 配置文件（如 `~/.cursor/mcp.json` 或 `~/.trae/mcp.json`）前，必须创建同级目录下的锁文件 `.mcp.json.lock`。
* **锁文件校验**：尝试创建排他性锁，成功后再写入，写入完毕后自动释放锁。
* **原子覆盖**：不要直接覆盖，先写到临时的 `.tmp` 文件，通过重命名原子替换原配置。

### 2. 运行诊断脚本 (diagnose-mcp.sh)
本技能附带了一个轻量级诊断脚本 `diagnose-mcp.sh`，用于诊断当前已配置的团队 HTTP MCP 在线状态。

### 3. 一键还原备份
如果配置由于 IDE 异常发生损坏，在目录下查找 `mcp.json.vola.bak` 文件，通过本技能的一键还原规程将其覆盖回 `mcp.json` 即可完成自愈。

---

## 环境变量要求

* `VOLA_CONFIG`：Vola 本地客户端配置文件路径。
* `HOME`：当前用户主目录。

---

## 技能定义元数据 (manifest.vola.json)

```json
{
  "name": "local-mcp-management-robustness",
  "version": "1.0.0",
  "description": "Diagnose local MCP client connections, concurrency locks, and handle automatic backups & restoration.",
  "entrypoint": "diagnose-mcp.sh",
  "target_agents": ["claude-code", "codex", "cursor", "gemini-cli"],
  "permissions": [
    "read-local-config",
    "write-local-config"
  ]
}
```
