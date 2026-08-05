package seaagentsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSkillsResourcePreservesMCPServerBindings(t *testing.T) {
	payload := map[string]any{
		"name": "mcp-research",
		"config": map[string]any{
			"mcp_servers": []string{"11111111-1111-4111-8111-111111111111"},
		},
	}
	expectedPayload, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]any
	if err := json.Unmarshal(expectedPayload, &expected); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		method string
		path   string
		call   func(*SkillsResource) error
	}{
		{
			name: "register", method: http.MethodPost, path: "/agent-v2/v1/skills/register",
			call: func(resource *SkillsResource) error {
				_, err := resource.Register(context.Background(), payload)
				return err
			},
		},
		{
			name: "update", method: http.MethodPut, path: "/agent-v2/v1/skills/skill-1",
			call: func(resource *SkillsResource) error {
				_, err := resource.Update(context.Background(), "skill-1", payload)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			type requestRecord struct {
				method string
				path   string
				body   map[string]any
				err    error
			}
			received := make(chan requestRecord, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				record := requestRecord{method: request.Method, path: request.URL.EscapedPath()}
				record.err = json.NewDecoder(request.Body).Decode(&record.body)
				received <- record
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"data":{}}`))
			}))
			defer server.Close()

			client := NewClient(ClientOptions{Endpoint: server.URL, HTTPClient: server.Client()})
			if err := test.call(client.Skills); err != nil {
				t.Fatal(err)
			}
			record := <-received
			if record.err != nil {
				t.Fatal(record.err)
			}
			if record.method != test.method || record.path != test.path {
				t.Fatalf("request = %s %s, want %s %s", record.method, record.path, test.method, test.path)
			}
			if !reflect.DeepEqual(record.body, expected) {
				t.Fatalf("request body = %#v, want %#v", record.body, expected)
			}
		})
	}
}
