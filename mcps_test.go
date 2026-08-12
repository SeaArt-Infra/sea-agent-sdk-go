package seaagentsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMcpsResourceUsesMCPManagementRoutes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		call   func(*McpsResource) error
	}{
		{
			name: "register", method: http.MethodPost, path: "/agent-v2/v1/mcps/register",
			call: func(resource *McpsResource) error {
				_, err := resource.Register(context.Background(), mcpSDKPayload())
				return err
			},
		},
		{
			name: "list", method: http.MethodGet, path: "/agent-v2/v1/mcps",
			call: func(resource *McpsResource) error {
				_, err := resource.List(context.Background(), MCPListOptions{Status: "active", IncludeDeleted: true})
				return err
			},
		},
		{
			name: "get", method: http.MethodGet, path: "/agent-v2/v1/mcps/mcp%2F1",
			call: func(resource *McpsResource) error {
				_, err := resource.Get(context.Background(), "mcp/1")
				return err
			},
		},
		{
			name: "update", method: http.MethodPut, path: "/agent-v2/v1/mcps/mcp-1",
			call: func(resource *McpsResource) error {
				_, err := resource.Update(context.Background(), "mcp-1", mcpSDKPayload())
				return err
			},
		},
		{
			name: "delete", method: http.MethodDelete, path: "/agent-v2/v1/mcps/mcp-1",
			call: func(resource *McpsResource) error {
				_, err := resource.Delete(context.Background(), "mcp-1")
				return err
			},
		},
		{
			name: "tools", method: http.MethodGet, path: "/agent-v2/v1/mcps/mcp-1/tools",
			call: func(resource *McpsResource) error {
				_, err := resource.Tools(context.Background(), "mcp-1")
				return err
			},
		},
		{
			name: "call", method: http.MethodPost, path: "/agent-v2/v1/mcps/mcp-1/call",
			call: func(resource *McpsResource) error {
				_, err := resource.Call(context.Background(), "mcp-1", map[string]any{"name": "search", "arguments": map[string]any{"query": "hello"}})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seen := make(chan mcpSDKRequestRecord, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				record := mcpSDKRequestRecord{method: request.Method, path: request.URL.EscapedPath(), query: request.URL.Query()}
				if request.Method == http.MethodPost || request.Method == http.MethodPut {
					record.err = json.NewDecoder(request.Body).Decode(&record.body)
				}
				seen <- record
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"data":{}}`))
			}))
			defer server.Close()

			client := NewClient(ClientOptions{Endpoint: server.URL, HTTPClient: server.Client()})
			if err := test.call(client.Mcps); err != nil {
				t.Fatal(err)
			}
			record := <-seen
			if record.err != nil {
				t.Fatal(record.err)
			}
			if record.method != test.method || record.path != test.path {
				t.Fatalf("request = %s %s, want %s %s", record.method, record.path, test.method, test.path)
			}
			if test.name == "list" && record.query.Get("status") != "active" || test.name == "list" && record.query.Get("include_deleted") != "true" {
				t.Fatalf("list query = %s", record.query.Encode())
			}
		})
	}
}

type mcpSDKRequestRecord struct {
	method string
	path   string
	query  url.Values
	body   map[string]any
	err    error
}

func mcpSDKPayload() map[string]any {
	return map[string]any{
		"name":       "sea-search",
		"server_url": "https://mcp.example.com/mcp",
		"transport":  "streamable-http",
	}
}
