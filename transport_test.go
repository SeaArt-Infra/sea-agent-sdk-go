package seaagentsdk

import (
	"net/http"
	"testing"
	"time"
)

func TestDefaultStreamClientHasNoTotalTimeout(t *testing.T) {
	transport := NewTransport("http://127.0.0.1:8080", "", nil, nil)
	if transport.httpClient.Timeout != 60*time.Second {
		t.Fatalf("HTTP timeout = %s, want 60s", transport.httpClient.Timeout)
	}
	if transport.streamHTTPClient.Timeout != 0 {
		t.Fatalf("stream timeout = %s, want no total timeout", transport.streamHTTPClient.Timeout)
	}

	custom := &http.Client{Timeout: 2 * time.Minute}
	transport = NewTransport("http://127.0.0.1:8080", "", nil, custom)
	if transport.streamHTTPClient != custom {
		t.Fatal("custom HTTP client was not preserved for streams")
	}
}

func TestNormalizeAgentGatewayEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "base URL",
			endpoint: "http://127.0.0.1:8080",
			want:     "http://127.0.0.1:8080/agent-v2",
		},
		{
			name:     "base URL with trailing slash",
			endpoint: "http://127.0.0.1:8080/",
			want:     "http://127.0.0.1:8080/agent-v2",
		},
		{
			name:     "existing agent-v2 prefix",
			endpoint: "http://127.0.0.1:8080/agent-v2",
			want:     "http://127.0.0.1:8080/agent-v2",
		},
		{
			name:     "existing agent-v2 prefix with trailing slash",
			endpoint: "http://127.0.0.1:8080/agent-v2/",
			want:     "http://127.0.0.1:8080/agent-v2/",
		},
		{
			name:     "base path",
			endpoint: "https://example.com/api",
			want:     "https://example.com/api/agent-v2",
		},
		{
			name:     "query preserved",
			endpoint: "https://example.com?debug=1",
			want:     "https://example.com/agent-v2?debug=1",
		},
		{
			name:     "empty",
			endpoint: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAgentGatewayEndpoint(tt.endpoint); got != tt.want {
				t.Fatalf("normalizeAgentGatewayEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestBuildURLAddsAgentV2Fallback(t *testing.T) {
	transport := NewTransport("http://127.0.0.1:8080", "", nil, nil)
	got, err := transport.buildURL("/v1/tools", QueryParams{"limit": 20})
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:8080/agent-v2/v1/tools?limit=20"
	if got != want {
		t.Fatalf("buildURL() = %q, want %q", got, want)
	}

	transport = NewTransport("http://127.0.0.1:8080/agent-v2", "", nil, nil)
	got, err = transport.buildURL("/v1/tools", nil)
	if err != nil {
		t.Fatal(err)
	}
	want = "http://127.0.0.1:8080/agent-v2/v1/tools"
	if got != want {
		t.Fatalf("buildURL() = %q, want %q", got, want)
	}
}
