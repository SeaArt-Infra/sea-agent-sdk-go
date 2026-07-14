package seaagentsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHooksResourceUsesAPIKeyScopedRoutes(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		call   func(*HooksResource) error
	}{
		{
			name:   "register",
			method: http.MethodPost,
			path:   "/agent-v2/v1/hooks/register",
			call: func(resource *HooksResource) error {
				_, err := resource.Register(context.Background(), hookTestRequest())
				return err
			},
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/agent-v2/v1/hooks",
			call: func(resource *HooksResource) error {
				_, err := resource.Update(context.Background(), hookTestRequest())
				return err
			},
		},
		{
			name:   "delete",
			method: http.MethodDelete,
			path:   "/agent-v2/v1/hooks",
			call: func(resource *HooksResource) error {
				_, err := resource.Delete(context.Background())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan hookRequestRecord, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				record := hookRequestRecord{method: r.Method, path: r.URL.Path}
				if tt.method != http.MethodDelete {
					record.err = json.NewDecoder(r.Body).Decode(&record.body)
				}
				seen <- record
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"id":"hook_1"}}`))
			}))
			defer server.Close()

			client := NewClient(ClientOptions{Endpoint: server.URL, HTTPClient: server.Client()})
			if err := tt.call(client.Hooks); err != nil {
				t.Fatal(err)
			}
			record := <-seen
			if record.err != nil {
				t.Fatal(record.err)
			}
			if record.method != tt.method {
				t.Fatalf("method = %s, want %s", record.method, tt.method)
			}
			if record.path != tt.path {
				t.Fatalf("path = %s, want %s", record.path, tt.path)
			}
			if tt.method != http.MethodDelete && (len(record.body) != 3 || record.body["name"] == nil || record.body["endpoint"] == nil || record.body["description"] == nil) {
				t.Fatalf("unexpected hook body: %#v", record.body)
			}
		})
	}
}

type hookRequestRecord struct {
	method string
	path   string
	body   map[string]any
	err    error
}

func hookTestRequest() HookRequest {
	return HookRequest{
		Name:        "production-line-hook",
		Endpoint:    "https://example.com/agent-hook",
		Description: "Receives multimodal charge reservation events.",
	}
}
