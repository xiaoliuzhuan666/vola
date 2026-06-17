package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agi-bar/vola/internal/services"
	"github.com/google/uuid"
)

type MCPServersConfig struct {
	Version string            `json:"version"`
	Servers []ConfigMCPServer `json:"servers"`
}

type ConfigMCPServer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	Transport   string            `json:"transport,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status,omitempty"`
	Visibility  string            `json:"visibility,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

type ManagedServer struct {
	Config ConfigMCPServer
	Cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser

	// JSON-RPC 状态管理
	nextID    int64
	pendingMu sync.Mutex
	pending   map[int64]chan jsonRPCResponse
	readerWG  sync.WaitGroup
	active    bool

	tools []MCPTool
}

type MCPGateway struct {
	fileTree *services.FileTreeService
	ownerID  interface{} // 兼容 uuid.UUID

	mu      sync.RWMutex
	servers map[string]*ManagedServer
	running bool
}

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func NewGateway(fileTree *services.FileTreeService, ownerID interface{}) *MCPGateway {
	return &MCPGateway{
		fileTree: fileTree,
		ownerID:  ownerID,
		servers:  make(map[string]*ManagedServer),
	}
}

// Start 自动读取文件树中的配置并启动所有 enabled 的外部 MCP Server 进程
func (g *MCPGateway) Start(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running {
		return nil
	}

	slog.Info("Starting MCP Gateway...")
	g.running = true

	// 1. 读取配置文件
	cfg, err := g.loadConfig(ctx)
	if err != nil {
		slog.Warn("Failed to load /settings/mcp-servers.json config, starting with empty gateway", "error", err)
		return nil
	}

	// 2. 依次启动进程
	for _, sc := range cfg.Servers {
		if !sc.Enabled {
			continue
		}
		s := &ManagedServer{
			Config:  sc,
			pending: make(map[int64]chan jsonRPCResponse),
		}
		if err := g.startServer(s); err != nil {
			slog.Error("Failed to start managed MCP Server", "id", sc.ID, "error", err)
			continue
		}
		g.servers[sc.ID] = s
	}

	return nil
}

// Stop 强杀所有的托管子进程，清理生命周期
func (g *MCPGateway) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.running {
		return
	}
	slog.Info("Stopping MCP Gateway...")
	g.running = false

	for id, s := range g.servers {
		g.stopServer(s)
		delete(g.servers, id)
	}
}

// MergeTools 将 Vola 原生的 Tools 列表与托管的第三方 MCP 的 Tools 列表合并输出
func (g *MCPGateway) MergeTools(native []MCPTool) []MCPTool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	out := make([]MCPTool, len(native))
	copy(out, native)

	for _, s := range g.servers {
		if !s.active {
			continue
		}
		for _, tool := range s.tools {
			// 为外部 Tool 名称加上前缀隔离，防范与原生的重名冲突
			prefixedTool := tool
			prefixedTool.Name = fmt.Sprintf("%s__%s", s.Config.ID, tool.Name)
			out = append(out, prefixedTool)
		}
	}
	return out
}

// IsExternalTool 检测某个 ToolCall 是否属于被代理路由的外部第三方 Tool
func (g *MCPGateway) IsExternalTool(name string) (string, string, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for id, s := range g.servers {
		prefix := id + "__"
		if s.active && len(name) > len(prefix) && name[:len(prefix)] == prefix {
			return id, name[len(prefix):], true
		}
	}
	return "", "", false
}

// CallExternalTool 透明代理转发 JSON-RPC tools/call 请求到对应的子进程并返回
func (g *MCPGateway) CallExternalTool(serverID, originalToolName string, args map[string]interface{}) (string, bool) {
	g.mu.RLock()
	s, ok := g.servers[serverID]
	g.mu.RUnlock()

	if !ok || !s.active {
		return fmt.Sprintf("error: managed MCP server %s not found or inactive", serverID), true
	}

	params := map[string]interface{}{
		"name":      originalToolName,
		"arguments": args,
	}

	resp, err := s.call("tools/call", params, 10*time.Second)
	if err != nil {
		return fmt.Sprintf("error calling tool: %v", err), true
	}

	if resp.Error != nil {
		return fmt.Sprintf("MCP Server Error (code=%d): %s", resp.Error.Code, resp.Error.Message), true
	}

	// 转换结果
	var resultStruct struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal(resp.Result, &resultStruct); err != nil {
		return fmt.Sprintf("error decoding MCP response: %v, raw: %s", err, string(resp.Result)), true
	}

	if len(resultStruct.Content) > 0 {
		return resultStruct.Content[0].Text, resultStruct.IsError
	}
	return "", resultStruct.IsError
}

func (g *MCPGateway) startServer(s *ManagedServer) error {
	cmd := exec.Command(s.Config.Command, s.Config.Args...)

	// 拼接环境变量
	env := os.Environ()
	for k, v := range s.Config.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	s.Cmd = cmd
	s.Stdin = stdin
	s.Stdout = stdout
	s.Stderr = stderr
	s.active = true

	// 启动错误流日志记录器
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			slog.Debug("Managed MCP Server stderr", "id", s.Config.ID, "log", scanner.Text())
		}
	}()

	// 启动数据流读取器协程
	s.readerWG.Add(1)
	go s.runReader()

	// 协议初始化握手
	if err := g.initializeProtocol(s); err != nil {
		g.stopServer(s)
		return fmt.Errorf("protocol initialize handshake failed: %w", err)
	}

	// 发现可用的 Tools
	if err := g.discoverTools(s); err != nil {
		g.stopServer(s)
		return fmt.Errorf("tools discovery failed: %w", err)
	}

	slog.Info("Managed MCP Server fully integrated", "id", s.Config.ID, "tools_count", len(s.tools))
	return nil
}

func (g *MCPGateway) stopServer(s *ManagedServer) {
	s.active = false
	if s.Stdin != nil {
		s.Stdin.Close()
	}
	if s.Cmd != nil {
		_ = s.Cmd.Process.Kill()
		_ = s.Cmd.Wait()
	}
	s.readerWG.Wait()
	slog.Info("Stopped managed MCP Server", "id", s.Config.ID)
}

func (s *ManagedServer) runReader() {
	defer s.readerWG.Done()
	scanner := bufio.NewScanner(s.Stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Error("Failed to parse JSON-RPC line from child stdio", "id", s.Config.ID, "error", err, "raw", string(line))
			continue
		}

		// 根据 ID 唤醒挂起的请求 channel
		var idVal int64
		switch v := resp.ID.(type) {
		case float64:
			idVal = int64(v)
		case int64:
			idVal = v
		default:
			continue
		}

		s.pendingMu.Lock()
		ch, exists := s.pending[idVal]
		if exists {
			delete(s.pending, idVal)
		}
		s.pendingMu.Unlock()

		if exists {
			ch <- resp
			close(ch)
		}
	}
}

func (s *ManagedServer) call(method string, params interface{}, timeout time.Duration) (jsonRPCResponse, error) {
	if !s.active {
		return jsonRPCResponse{}, errors.New("server not active")
	}

	id := atomic.AddInt64(&s.nextID, 1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return jsonRPCResponse{}, err
	}

	ch := make(chan jsonRPCResponse, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()

	// 写入 Stdin
	_, err = s.Stdin.Write(append(data, '\n'))
	if err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return jsonRPCResponse{}, err
	}

	// 等待并超时处理
	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(timeout):
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return jsonRPCResponse{}, fmt.Errorf("timeout waiting for response (id=%d)", id)
	}
}

func (s *ManagedServer) notify(method string, params interface{}) error {
	if !s.active {
		return errors.New("server not active")
	}
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = s.Stdin.Write(append(data, '\n'))
	return err
}

func (g *MCPGateway) initializeProtocol(s *ManagedServer) error {
	initParams := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "vola-gateway",
			"version": "1.0.0",
		},
	}
	resp, err := s.call("initialize", initParams, 5*time.Second)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("server initialize error: %s", resp.Error.Message)
	}

	// 发送 notifications/initialized 通知
	return s.notify("notifications/initialized", map[string]interface{}{})
}

func (g *MCPGateway) discoverTools(s *ManagedServer) error {
	resp, err := s.call("tools/list", nil, 5*time.Second)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var toolsResp struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &toolsResp); err != nil {
		return err
	}
	s.tools = toolsResp.Tools
	return nil
}

func (g *MCPGateway) loadConfig(ctx context.Context) (*MCPServersConfig, error) {
	if g.fileTree == nil {
		return nil, errors.New("filetree not available")
	}

	var uid uuid.UUID
	switch v := g.ownerID.(type) {
	case uuid.UUID:
		uid = v
	case string:
		var err error
		uid, err = uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid owner ID string format: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported owner ID type: %T", g.ownerID)
	}

	entry, err := g.fileTree.Read(ctx, uid, "/settings/mcp-servers.json", 100) // 100 代表 Full TrustLevel
	if err != nil {
		return nil, err
	}

	var cfg MCPServersConfig
	if err := json.Unmarshal([]byte(entry.Content), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
