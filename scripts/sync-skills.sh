#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# 定位 Vola 的配置与运行态目录
if [ -n "$VOLA_CONFIG" ]; then
    CONFIG_PATH="$VOLA_CONFIG"
    CONFIG_DIR=$(dirname "$CONFIG_PATH")
elif [ -n "$NEUDRIVE_CONFIG" ]; then
    CONFIG_PATH="$NEUDRIVE_CONFIG"
    CONFIG_DIR=$(dirname "$CONFIG_PATH")
else
    CONFIG_DIR="$HOME/.config/vola"
    if [ ! -d "$CONFIG_DIR" ] && [ -d "$HOME/Library/Application Support/Vola" ]; then
        CONFIG_DIR="$HOME/Library/Application Support/Vola"
    fi
    # macOS legacy 兼容性检查
    if [ ! -d "$CONFIG_DIR" ] && [ -d "$HOME/Library/Application Support/NeuDrive" ]; then
        CONFIG_DIR="$HOME/Library/Application Support/NeuDrive"
    fi
fi

if [ -z "${CONFIG_PATH:-}" ]; then
    CONFIG_PATH="$CONFIG_DIR/config.json"
fi
RUNTIME_PATH="$CONFIG_DIR/runtime.json"

echo "==== Vola Skill 本地同步脚本 ===="

# 1. 尝试检测或拉起 daemon
check_daemon() {
    if [ -f "$RUNTIME_PATH" ]; then
        API_BASE=$(grep -o '"api_base"[[:space:]]*:[[:space:]]*"[^"]*"' "$RUNTIME_PATH" | head -n 1 | cut -d'"' -f4 || true)
        if [ -n "$API_BASE" ] && curl -s -f --max-time 2 "$API_BASE/api/health" > /dev/null 2>&1; then
            return 0
        fi
    fi
    return 1
}

if ! check_daemon; then
    echo "未检测到运行中的 Vola 本地守护进程。尝试启动..."
    if command -v neu >/dev/null 2>&1; then
        # 通过 CLI 的 status 自动拉起 daemon
        neu status > /dev/null 2>&1 || true
    elif command -v vola >/dev/null 2>&1; then
        vola status > /dev/null 2>&1 || true
    elif [ -f "./bin/neu" ]; then
        ./bin/neu status > /dev/null 2>&1 || true
    elif [ -f "./bin/vola" ]; then
        ./bin/vola status > /dev/null 2>&1 || true
    else
        # 尝试通过 go run 启动（如果在开发源码目录下）
        if [ -f "./cmd/vola/main.go" ]; then
            echo "在源码目录下，正在通过 go 启动本地开发服务..."
            if [ -f "./bin/vola" ]; then
                ./bin/vola status > /dev/null 2>&1 || true
            fi
        fi
    fi
fi

# 再次检查
if ! check_daemon; then
    echo "❌ 错误: 无法启动或连接到本地 Vola 守护进程 (Daemon)。"
    echo "请先在后台运行 Vola 服务。您可以使用以下命令启动："
    echo "    neu status  (自动在后台启动守护进程)"
    echo "或者运行开发服务器："
    echo "    go run ./cmd/vola server --local-mode"
    exit 1
fi

echo "✅ 成功连接到本地 daemon: $API_BASE"

# 2. 提取管理员 Token
if [ ! -f "$CONFIG_PATH" ]; then
    echo "❌ 错误: 找不到配置文件 config.json，路径为: $CONFIG_PATH"
    echo "请先运行 'neu status' 初始化配置。"
    exit 1
fi

OWNER_TOKEN=$(grep -o '"owner_token"[[:space:]]*:[[:space:]]*"[^"]*"' "$CONFIG_PATH" | head -n 1 | cut -d'"' -f4 || true)
if [ -z "$OWNER_TOKEN" ]; then
    echo "❌ 错误: 配置文件中未包含 owner_token。"
    exit 1
fi

# 3. 构造同步 Payload
TEAM_ID="$1"
AGENT_IDS='["claude-code", "codex", "cursor"]' # 默认同步 Claude Code、Codex 和 Cursor

if [ -n "$TEAM_ID" ]; then
    echo "正在发起团队 Skill 同步，团队 ID: $TEAM_ID ..."
    PAYLOAD=$(printf '{"agent_ids": %s, "team_id": "%s", "ack_quality_review": true}' "$AGENT_IDS" "$TEAM_ID")
else
    echo "正在发起个人/默认空间 Skill 同步..."
    PAYLOAD=$(printf '{"agent_ids": %s, "ack_quality_review": true}' "$AGENT_IDS")
fi

# 4. 调用本地同步与清理 API
echo "正在应用本地同步计划..."
RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d "$PAYLOAD" \
  "$API_BASE/api/local/skills/sync/apply")

echo "正在应用本地清理计划..."
CLEANUP_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -d "$PAYLOAD" \
  "$API_BASE/api/local/skills/sync/cleanup")

# 5. 输出格式化结果
# 简易输出提取，也可以通过 jq 格式化（如果安装了 jq）
if command -v jq >/dev/null 2>&1; then
    echo "=== 同步结果 ==="
    echo "$RESPONSE" | jq '.'
    echo "=== 清理结果 ==="
    echo "$CLEANUP_RESPONSE" | jq '.'
else
    echo "=== 同步结果 ==="
    echo "$RESPONSE"
    echo "=== 清理结果 ==="
    echo "$CLEANUP_RESPONSE"
fi

echo "========================================"
echo "🎉 同步与清理处理完成！请检查 Agent 本地 Skills 目录。"
