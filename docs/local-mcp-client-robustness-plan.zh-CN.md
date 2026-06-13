# Vola 本地客户端装配健壮性与团队协作实时化增强计划与落地记录

为了使团队在使用 Cursor, Trae, Codebuddy, Workbuddy 等 IDE 客户端时具有工业级的稳定性与零感知的实时协作体验，Vola 的本地守护进程（Daemon）、Go 后端服务和 React 前端 UI 进行了以下核心维度的优化与落地。

---

## 1. 核心技术方案与落地详情

### 1.1 mcp.json 与 config.json 并发安全锁、冷备灾备与原子性写入
1. **轻量级跨平台进程锁**：
   * 在读写各 IDE 配置文件（如 `~/.cursor/mcp.json`）之前，尝试在同级目录下创建临时锁定文件 `.mcp.json.lock`。
   * 采用带重试的非阻塞创建模式（`os.O_CREATE|os.O_EXCL`），单次等待 100ms，最大重试 3 秒。
   * 成功获取锁后再进行文件的读取与写入，并在完成或异常退出时通过 `defer os.Remove` 自动释放锁，避免多客户端并发写入竞态导致损坏。
2. **首次自动安全冷备**：
   * 在首次对配置执行修改前，若同级目录下不存在备份 `mcp.json.vola.bak`，则对原文件执行一次完整物理备份。
   * 在 `Disconnect` 时，如果清理后文件无其他配置，提供一键将 `.vola.bak` 恢复的灾备恢复机制。
3. **主配置与 mcp.json 损坏自愈 (Self-Healing) [NEW]**：
   * **全局配置自愈**：在 `SaveConfig` 后自动将配置冷备至 `config.json.vola.bak`。加载配置若解析出错或文件为空，自动恢复备份内容并物理覆盖写入主配置，实现秒级自愈。
   * **mcp.json 自愈**：在 `safeUpdateMcpConfig` 中增加反序列化解析错误拦截，若由于异常断电导致 JSON 写坏，自动从 `.vola.bak` 物理恢复回滚。
4. **原子性覆盖写入 (Atomic Write)**：
   * 写入配置时不直接操作目标文件，先写入 `.tmp` 文件，再调用操作系统的原子级重命名 `os.Rename` 替换原配置，彻底杜绝写出一半文件导致 JSON 格式损毁的现象。

### 1.2 SQLite 存储层多路并发读取优化 [NEW]
1. **解除单连接池限制**：
   * 移除了 `sqlite.Open` 中限制单连接并发的 `db.SetMaxOpenConns(1)` 约束，重构为最大 10 个活跃连接 `db.SetMaxOpenConns(10)` 与 5 个空闲连接 `db.SetMaxIdleConns(5)`。
   * 结合 SQLite 的 WAL 模式，实现了多客户端/多协程并发读取本地数据库的高吞吐，写锁冲突依靠 DSN 内的 `busy_timeout(5000)` 安全阻塞等待。

### 1.3 共享 HTTP MCP 本地存活度与延时检测 (Health Check)
1. **常驻存活探测器**：
   * 在 Vola 本地守护进程中启动常驻健康检查协程，每 30 秒轮询检测本机关接的所有远程 `http` 团队 MCP 终点。
   * 发送轻量级请求（3 秒超时 `HEAD` 或 `GET`），计算往返延时并检测服务是否在线，结果缓存在本地守护进程的 `sync.Map` 中。
2. **健康度 API 端点**：
   * 新增 API：`GET /api/local/mcp/health`，向前端实时返回已装配 HTTP MCP 的在线状态（`online` / `offline`）和延迟（`latency_ms`）。
3. **前端可视化健康微章与呼吸灯**：
   * 在 `TeamLibraryPage.tsx` 和 `DashboardPage.tsx` 的团队 MCP 列表及配置详情中展示检测面板。离线服务伴有红色呼吸发光效果徽章，在线服务标注绿色微章并显示延迟。
4. **失效键值清理**：
   * 在每次检测时，自动比对最新配置，对从配置中删除或解绑的已下线服务从 `mcpHealthCache` 内存缓存中执行 `Delete` 彻底清除。
5. **探测自适应退避与按需唤醒**：
   * **连续失败退避**：对连续检测失败的离线远程 HTTP MCP 服务，自动递增其检测间隔周期（连续失败 1 次时每 2 个周期检查一次；失败 2 次时每 4 个周期检查一次；失败 3 次以上时每 8 个周期），降低离线节点造成的 DNS 解析负荷与 TCP 超时浪费。
   * **Dashboard 强制唤醒**：在 API 请求被调用且距上一次刷新超过 5 秒时，会在后台自动重置退避并触发一次全量刷新检查。

### 1.4 安全日志属性脱敏与 Panic 凭证掩码拦截 [NEW]
1. **slog 日志拦截掩码 (ReplaceAttr)**：
   * 在 `Init` 流程中配置 `ReplaceAttr` 处理逻辑。
   * 检测键名为 `token`、`password`、`secret`、`jwt`、`authorization` 等字段的 KV 属性，将其脱敏替换为 `"[MASKED]"`。
   * 对所有包含 `Bearer ` 头的字符串属性值进行掩码拦截清洗，杜绝任何 API Key 进入日志文件。

### 1.5 基于 SSE 的协作更新秒级实时推送与瞬间重连重放 [UPDATE]
1. **服务端事件流端点**：
   * 在中心 Go 服务端提供 `GET /api/teams/{team}/events` SSE 协议端点。涉及该团队的 Skill 发布、更新、或是团队 MCP 的变动事件，通过事件广播机制下发至客户端。
2. **多协程连接池生命周期解耦**：
   * 解除全局阻塞，为每个 team 分配独立的生命周期协程，单独管理连接建立、异常捕获和重连。
3. **指数退避重连与看门狗自愈**：
   * 增加指数退避等待逻辑（从 1s 开始，每次失败翻倍，最大为 60s），防止由于服务端波动或网络中断引发高频重试风暴。
   * 启动 45s 数据流看门狗（Watchdog），在超时间隔无有效数据时主动断流熔断，强制唤醒读取协程抛出 `EOF` 并自愈重连。
4. **增量事件瞬间重放 (Event Replay Log) [NEW]**：
   * **服务端定长缓存**：在 `EventBroker` 中加入定长历史变更队列（保留 5 分钟），客户端重连时通过携带 Query 参数 `?last_seen_ms` 指明自己看到的数据时间戳。
   * **瞬间增量补发**：服务端自动提取并定向重放补发这期间错失的所有变更事件，消除瞬间网络抖动造成的通知盲区。

### 1.6 高精度项目级装配与沙箱穿越防御 [UPDATE]
1. **装配连接结构扩展与 Tags 过滤**：
   * 允许在连接特定 IDE 时配置仅同步含有特定 Tags 的团队 MCP。
2. **项目级局部装配**：
   * 支持读取开发目录下的 `.volarc` 配置，依据项目标签执行局部高精装配，防止全局配置文件冗余，降低提示词 Token 消耗。
3. **路径规范化防穿越 (Path Traversal 防御) [NEW]**：
   * 在加载 `.volarc` 时，通过 `filepath.EvalSymlinks` 展开真实路径，强力拦截并阻断命中系统核心敏感前缀（如 `/etc/`，`/var/`，`/private/` 等）的路径加载，确保沙箱安全性。

---

## 2. 自动化测试与系统验证

为了验证上述机制，项目新编写并运行了极具代表性的单元与集成测试：

1. **文件读写锁及原子写入测试**：[platforms_lock_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/platforms/platforms_lock_test.go)
   * `TestMcpConfigSafeAtomicWriteAndLock`：验证在高并发并发写入下，配置文件没有出现损坏，并且生成了 `.vola.bak` 备份文件。
2. **SQLite 存储并发读取性能测试 [NEW]**：[store_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/storage/sqlite/store_test.go)
   * `TestSQLiteWALModeReadConcurrency`：并发 10 个 goroutines 进行高频读取，验证 WAL 连接池性能，无 database locked 锁死现象。
3. **配置损坏崩溃与自愈测试 [NEW]**：[platforms_lock_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/platforms/platforms_lock_test.go)
   * `TestMcpConfigSelfHealing`：手动将配置文件改写成乱码损坏状态，验证 Connect 触发时自动从 `.vola.bak` 恢复完好配置并写回。
4. **安全日志脱敏掩码测试 [NEW]**：[logger_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/logger/logger_test.go)
   * `TestLogMaskingSecurity`：验证 `token`/`password`/`secret`/`authorization` 以及 `Bearer ` 头信息输出时被成功替换为 `"[MASKED]"`。
5. **SSE 事件断连历史重放测试 [NEW]**：[sse_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/api/sse_test.go)
   * `TestSSEEventReplayOnReconnect`：模拟客户端在断开期间云端产生变更，重连时发送时间戳立刻定向补发这期间错失的事件。
6. **项目装配路径沙箱防穿越测试 [NEW]**：[platforms_lock_test.go](file:///Users/zhongmoshu/Desktop/work/Vola/internal/platforms/platforms_lock_test.go)
   * `TestVolarcPathSandboxing`：向 `.volarc` 植入系统敏感路径软链接，测试 `loadLocalVolarc` 成功阻断加载。
