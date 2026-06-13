#!/usr/bin/env bash

# local-mcp-management-robustness 诊断脚本
# 用于排查本地主流 AI 编辑器的 MCP 装配状态与远程共享 HTTP MCP 的连通性

set -euo pipefail

echo "============================================="
echo "   Vola 本地 MCP 客户端连通性与健壮性诊断工具"
echo "============================================="

# 1. 检测常见 IDE 的 mcp.json 位置
IDE_PATHS=(
  "Cursor|$HOME/.cursor/mcp.json"
  "Trae|$HOME/.trae/mcp.json"
  "Codebuddy|$HOME/.codebuddy/mcp.json"
  "Workbuddy|$HOME/.workbuddy/mcp.json"
)

found_config=false

for item in "${IDE_PATHS[@]}"; do
  name="${item%%|*}"
  path="${item#*|}"
  
  if [ -f "$path" ]; then
    echo "✔ 发现 $name 配置文件: $path"
    found_config=true
    
    # 检查是否有备份文件
    bak_path="${path}.vola.bak"
    if [ -f "$bak_path" ]; then
      echo "  ↳ [备份状态]: 已存在冷备文件 $bak_path"
    else
      echo "  ↳ [备份状态]: ⚠️ 未检测到备份。Vola 守护进程将在下次连接修改时自动冷备。"
    fi
    
    # 从 JSON 中解析出 team-mcp- 开头的 HTTP 服务的 URL 并探测连通性
    echo "  ↳ [装配团队 MCP 检测]:"
    # 用简单的 grep/sed 抓取 "team-mcp-" 相关的 URL (避免强依赖 jq)
    urls=$(grep -o '"url": *"[^"]*"' "$path" | sed -E 's/"url": *"//;s/"//' || true)
    
    if [ -z "$urls" ]; then
      echo "    无正在运行的共享 HTTP MCP。"
    else
      for url in $urls; do
        if [[ "$url" =~ ^http ]]; then
          echo -n "    - 正在探测 $url ... "
          start_time=$(date +%s%N)
          if curl -s -I -m 3 "$url" > /dev/null; then
            end_time=$(date +%s%N)
            # 计算延迟毫秒
            latency=$(( (end_time - start_time) / 1000000 ))
            echo -e "\033[32m【在线】\033[0m 延时: ${latency}ms"
          else
            # 尝试 GET
            if curl -s -m 3 "$url" > /dev/null; then
              end_time=$(date +%s%N)
              latency=$(( (end_time - start_time) / 1000000 ))
              echo -e "\033[32m【在线】\033[0m 延时: ${latency}ms"
            else
              echo -e "\033[31m【离线】\033[0m (3s 超时或服务已关闭)"
            fi
          fi
        fi
      done
    fi
  else
    echo "✖ 未发现 $name 配置文件"
  fi
done

if [ "$found_config" = false ]; then
  echo "⚠️ 提示: 本地未检测到任何主流 AI 编辑器 (Cursor/Trae/Codebuddy/Workbuddy) 的装配目录配置。"
fi

echo "============================================="
echo "诊断结束。可通过 'vola status' 或前台 UI 呼吸灯面板查看更详细的心跳信息。"
