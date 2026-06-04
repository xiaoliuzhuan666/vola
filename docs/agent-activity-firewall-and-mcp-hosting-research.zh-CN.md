# Agent 行为实时审计防火墙与本地 MCP 一键托管设计调研报告

---

## 1. 深度安全隐私合规层：Agent 行为审计与主动防御防火墙 (P0)

### 1.1 技术痛点与第一性原理剖析
在 Vola/Vola 目前的设计中，外部 Agent (如 Cursor, Claude, ChatGPT) 通过 `X-API-Key` 或 Bearer Token 对 `/agent/` 路由段进行数据交互。鉴权成功后，Agent 可以静默下载整个项目树镜像、读取 Profile 和 Memory、检索本地文件等。
在当前大模型（LLMs）具备自主规划和代码执行能力（Agentic AI）的背景下，这种静默访问带来了极高的隐私泄露隐患：
1. **静默越权**：用户不知道 Agent 在什么时间、读取了哪些具体文件（例如敏感的密码文件、秘钥环境配置文件 `.env` 等）。
2. **凭证窃取**：当 Agent 扫描整个代码仓库时，可能会顺带将本地的 SSH Key、AWS 凭证以及用户的浏览器 Cookie 历史作为上下文发送到第三方模型的公有云端，导致核心数字资产泄露。

为了解决这个问题，我们需要引入一个 **“Agent 行为实时审计与主动确认防火墙 (Agent Activity Audit & Consent Firewall)”**。

---

### 1.2 架构与实现路径设计 (Go 后端)

我们需要在 Go 服务端的 `internal/api` 中实现一个**请求劫持与审计拦截中间件**。

#### A. 数据库审计日志表设计 (`agent_audit_logs`)
每次 Agent 访问 `/agent/` 的路由，我们都需要在 SQLite 中记录以下审计字段：
```sql
CREATE TABLE agent_audit_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    connection_id TEXT,         -- 标识来自哪一个接入 of Agent 客户端
    token_id TEXT,              -- 或者是来自哪一个 ScopedToken
    agent_name TEXT,            -- 从 Connection 元数据中提取
    request_method TEXT,        -- GET, PUT, POST
    request_path TEXT,          -- 具体的 API 访问路径
    accessed_resource TEXT,     -- 提取出的资源（例如文件路径 /projects/xxx/main.go）
    status_code INTEGER,        -- 200, 403, 500
    risk_level TEXT,            -- LOW, MEDIUM, HIGH (依据敏感路径判定)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### B. API 请求劫持与中间件设计
我们在 `router.go` 中的 `/agent/` 路由组内，引入 `s.agentAuditMiddleware`：
```go
func (s *Server) agentAuditMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 提取调用源元数据
        conn := connectionFromCtx(r.Context())
        token := scopedTokenFromCtx(r.Context())

        agentName := "Unknown Agent"
        var connID, tokenID string
        if conn != nil {
            agentName = conn.Name
            connID = conn.ID
        } else if token != nil {
            agentName = "Scoped Token: " + token.Name
            tokenID = token.ID
        }

        // 2. 识别访问的具体资源
        accessedResource := r.URL.Path
        if strings.HasPrefix(accessedResource, "/agent/tree/") {
            accessedResource = strings.TrimPrefix(accessedResource, "/agent/tree/")
        }

        // 3. 实时风险路径识别 (PII / Credentials 防火墙)
        riskLevel := "LOW"
        isSensitive := false
        sensitivePaths := []string{".env", "id_rsa", "config.json", "credentials", "session", "cookie", "vault"}
        for _, sp := range sensitivePaths {
            if strings.Contains(strings.ToLower(accessedResource), sp) {
                riskLevel = "HIGH"
                isSensitive = true
                break
            }
        }

        // 4. HIGH 级别敏感拦截（主动确认弹窗防火墙）
        if isSensitive && s.isLocalMode() {
            // 通过 Tauri RPC 发送主动确认请求到桌面端 UI
            authorized := s.requestUserConsent(r.Context(), agentName, r.Method, accessedResource)
            if !authorized {
                respondForbidden(w, "Access blocked by Vola Security Firewall (PII/Credential protection)")
                // 记录被阻断的审计
                s.logAgentActivity(r.Context(), connID, tokenID, agentName, r.Method, r.URL.Path, accessedResource, 403, "BLOCKED_HIGH")
                return
            }
        }

        // 5. 往下传导执行
        ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        next.ServeHTTP(ww, r)

        // 6. 异步归档审计日志
        go s.logAgentActivity(r.Context(), connID, tokenID, agentName, r.Method, r.URL.Path, accessedResource, ww.statusCode, riskLevel)
    })
}
```

---

### 1.3 前端 UI 实时监控面板设计 (React 前端)

为了让用户感知审计活动，我们将在 Dashboard 中增加一个**“Agent 实时审计流 (Agent Activity Log Stream)”**：
- **实时流动看板**：通过 EventSource (SSE) 或 WebSocket 长连接，当 Go 中间件有请求通过时，前端看板实时流动更新。
- **状态卡片与风险标签**：
  - **LOW**：🟢 绿色标签，表示常规只读（如 `Cursor 读取了概览信息`）。
  - **HIGH**：🟡 黄色标签，表示敏感文件（如 `Claude 读取了 .env 配置文件`）。
  - **BLOCKED**：🔴 红色标签，表示被防火墙主动阻断。
- **一键切断（Kill Switch）**：可以在审计流上直接一键“撤销该 Agent 授权”，瞬间使该 Token 失效。

---

## 2. 技能资产化：本地 Skill 一键发布为本地 MCP 服务 (P1)

### 2.1 本地 MCP 协议网关的核心原理
在 Vola 目前的架构中，`MCP 控制中心` 主要负责管理本地运行的外部 MCP 进程（作为 Client 调用外部服务器）。
而在 P1 阶段中，我们要让 **Vola 本身充当一个本地的 MCP Host Server**！
因为 Vola 已经持有了用户的全量本地数据，且用户在 UI 上定义了非常多好用的 Skills（包含代码、命令行、提示词）。
通过把 Vola 本身发布为一个标准的 MCP Server，可以让外部所有兼容 MCP 的 AI 客户端（如 Claude Code CLI、Cursor, VS Code）直接连接它！

```
+------------------+             Stdio / HTTP (MCP)             +-------------------------+
|                  | <----------------------------------------> |                         |
|  Claude Code /   |                                            |  Vola Daemon        |
|  Cursor Editor   |              mcp.list_tools()              |  (MCP Server Gateway)   |
|                  |                                            |                         |
+------------------+                                            +-------------------------+
                                                                             |
                                                                   +---------+---------+
                                                                   |                   |
                                                           Execute Local Skill  Read local data
```

---

### 2.2 本地一键挂载实现路径
当 Vola 收到 MCP `list_tools` 握手时：
1. **自动发布 Skills 为 Tools**：Vola 会自动把用户在 `skills/` 目录中定义的自定义 Skill，映射为一个标准的 MCP `tool`：
   - 描述、输入参数声明（使用已有的 schema）都会自动转换成 JSON Schema。
2. **本地进程桥接**：
   - 当 Agent 通过 MCP 协议调用 `call_tool(tool_name, arguments)` 时，Vola 后端本地自动解析并执行该 Skill（如运行一段 JavaScript 或 CLI 命令），再把输出通过标准的 JSON RPC 协议吐回给客户端。

---

## 3. 本地向量化 RAG 网关设计 (P2)

### 3.1 本地轻量级嵌入与索引
- **Go 内置向量库**：在 `internal/services` 中，通过 sqlite 嵌入扩展或 pure-go 向量搜索实现极轻量级的词嵌入和向量检索，无需复杂的外部 Python 服务。
- **分块策略（Chunking）**：对于 `/projects` 下的用户代码或 conversations 备份，在本地进行按文件类型（如 `.go`, `.tsx`, `.md`）的 AST 树/语义段切片。

---

## 4. 商业化闭环与多团队组织协作层 (P3)

- **协同锁（Distributed Locking）**：基于 SQLite 的事务机制或 Go 本地 sync.Mutex，在云端同步协同上设计文件临时租赁锁。当 `Teammate A` 打开并编辑 `/team/mcp/README.md` 时，向 `Teammate B` 表明该文件正被锁定，保障文件写入一致性。
- **精细化 RBAC 角色网格**：基于用户 Connection 携带的 scoped scopes，设计符合部门划分的数据隔离控制矩阵（ACL）。
