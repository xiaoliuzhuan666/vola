package mcp

import (
	"testing"

	"github.com/google/uuid"
)

func TestMCPGateway_MergeTools(t *testing.T) {
	gateway := NewGateway(nil, uuid.New())
	gateway.running = true

	// 模拟拉起一个 ManagedServer，并填充 Tools
	serverID := "filesystem"
	gateway.servers[serverID] = &ManagedServer{
		Config: ConfigMCPServer{
			ID:      serverID,
			Name:    "Filesystem Server",
			Enabled: true,
		},
		active: true,
		tools: []MCPTool{
			{
				Name:        "read_file",
				Description: "Read a file from disk",
			},
		},
	}

	nativeTools := []MCPTool{
		{
			Name:        "read_profile",
			Description: "Read profile memory",
		},
	}

	merged := gateway.MergeTools(nativeTools)

	// 期望 merged 包含 2 个 Tools：read_profile 和 filesystem__read_file
	if len(merged) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(merged))
	}

	foundNative := false
	foundExternal := false
	for _, tool := range merged {
		if tool.Name == "read_profile" {
			foundNative = true
		} else if tool.Name == "filesystem__read_file" {
			foundExternal = true
		}
	}

	if !foundNative {
		t.Error("native tool 'read_profile' not found in merged list")
	}
	if !foundExternal {
		t.Error("prefixed external tool 'filesystem__read_file' not found in merged list")
	}
}

func TestMCPGateway_IsExternalTool(t *testing.T) {
	gateway := NewGateway(nil, uuid.New())
	gateway.running = true

	serverID := "filesystem"
	gateway.servers[serverID] = &ManagedServer{
		Config: ConfigMCPServer{
			ID:      serverID,
			Enabled: true,
		},
		active: true,
	}

	// 测试正确的外部 Tool 格式
	sID, origName, ok := gateway.IsExternalTool("filesystem__read_file")
	if !ok {
		t.Fatal("expected IsExternalTool to return true")
	}
	if sID != "filesystem" {
		t.Errorf("expected serverID=filesystem, got %s", sID)
	}
	if origName != "read_file" {
		t.Errorf("expected originalToolName=read_file, got %s", origName)
	}

	// 测试不匹配的 Tool 格式
	_, _, ok = gateway.IsExternalTool("read_profile")
	if ok {
		t.Fatal("expected IsExternalTool to return false for native tool")
	}

	// 测试禁用的外部服务 Tool 格式
	gateway.servers[serverID].active = false
	_, _, ok = gateway.IsExternalTool("filesystem__read_file")
	if ok {
		t.Fatal("expected IsExternalTool to return false for inactive server")
	}
}
