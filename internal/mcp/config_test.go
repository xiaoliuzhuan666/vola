package mcp

import "testing"

func TestGenerateStdioEnvConfig(t *testing.T) {
	cfg := GenerateStdioEnvConfig("vola-mcp", "")

	mcpServers, ok := cfg["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcpServers map, got %T", cfg["mcpServers"])
	}

	server, ok := mcpServers["vola"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected vola server map, got %T", mcpServers["vola"])
	}

	args, ok := server["args"].([]string)
	if !ok {
		t.Fatalf("expected args slice, got %T", server["args"])
	}
	if len(args) != 2 || args[0] != "--token-env" || args[1] != DefaultTokenEnvVar {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestGenerateHTTPOAuthConfig(t *testing.T) {
	cfg := GenerateHTTPOAuthConfig("https://vola.ai")

	mcpServers, ok := cfg["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcpServers map, got %T", cfg["mcpServers"])
	}

	server, ok := mcpServers["vola"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected vola server map, got %T", mcpServers["vola"])
	}

	if server["type"] != "http" {
		t.Fatalf("expected type=http, got %v", server["type"])
	}
	if server["url"] != "https://vola.ai/mcp" {
		t.Fatalf("unexpected url: %v", server["url"])
	}
	if _, exists := server["headers"]; exists {
		t.Fatal("oauth config should not include static headers")
	}
}

func TestGenerateHTTPBearerConfig(t *testing.T) {
	cfg := GenerateHTTPBearerConfig("https://vola.ai", "ndt_test")

	mcpServers, ok := cfg["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcpServers map, got %T", cfg["mcpServers"])
	}

	server, ok := mcpServers["vola"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected vola server map, got %T", mcpServers["vola"])
	}

	headers, ok := server["headers"].(map[string]string)
	if !ok {
		t.Fatalf("expected headers map, got %T", server["headers"])
	}

	if headers["Authorization"] != "Bearer ndt_test" {
		t.Fatalf("unexpected authorization header: %v", headers["Authorization"])
	}
}

func TestGenerateHTTPConfigAlias(t *testing.T) {
	cfg := GenerateHTTPConfig("https://vola.ai", "ndt_alias")

	mcpServers := cfg["mcpServers"].(map[string]interface{})
	server := mcpServers["vola"].(map[string]interface{})
	headers := server["headers"].(map[string]string)

	if headers["Authorization"] != "Bearer ndt_alias" {
		t.Fatalf("unexpected authorization header: %v", headers["Authorization"])
	}
}
