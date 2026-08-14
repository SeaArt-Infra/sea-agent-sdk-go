package seaagentsdk

import (
	"context"
	"net/http"
)

// MCPProtocolVersion is the streamable-HTTP revision spoken by the gateway's
// standard MCP endpoint.
const MCPProtocolVersion = "2025-03-26"

// McpsResource manages independently registered MCP servers and proxies their
// tools/list and tools/call operations through agent-gateway.
type McpsResource struct {
	transport *Transport
}

type MCPListOptions struct {
	Search         string
	Status         string
	Public         *bool
	Provider       string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

func (r *McpsResource) Register(ctx context.Context, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/mcps/register", payload, &result)
	return result, err
}

func (r *McpsResource) List(ctx context.Context, options MCPListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/mcps", QueryParams{
		"search":          options.Search,
		"status":          options.Status,
		"public":          options.Public,
		"provider":        options.Provider,
		"include_deleted": options.IncludeDeleted,
		"limit":           options.Limit,
		"offset":          options.Offset,
	}, &result)
	return result, err
}

func (r *McpsResource) Get(ctx context.Context, mcpID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/mcps/"+urlEscape(mcpID), nil, &result)
	return result, err
}

func (r *McpsResource) Update(ctx context.Context, mcpID string, payload any) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/mcps/"+urlEscape(mcpID), payload, &result)
	return result, err
}

func (r *McpsResource) Delete(ctx context.Context, mcpID string) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/mcps/"+urlEscape(mcpID), nil, &result)
	return result, err
}

func (r *McpsResource) Tools(ctx context.Context, mcpID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/mcps/"+urlEscape(mcpID)+"/tools", nil, &result)
	return result, err
}

func (r *McpsResource) Call(ctx context.Context, mcpID string, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/mcps/"+urlEscape(mcpID)+"/call", payload, &result)
	return result, err
}

// MCPConnectionInfo carries everything a standard MCP client needs to reach a
// registered server through the gateway's streamable-HTTP proxy endpoint.
type MCPConnectionInfo struct {
	// URL is the gateway endpoint to connect to, not the upstream server URL.
	URL string
	// Headers include gateway authentication. Upstream registry credentials are
	// injected by the gateway and never appear here.
	Headers map[string]string
}

// ConnectionInfo returns the endpoint and headers for talking MCP to a
// registered server through the gateway.
//
// The SDK deliberately stops here rather than implementing the protocol: the
// gateway endpoint is standard streamable-HTTP, so pair this with an official
// MCP SDK client instead of a hand-rolled JSON-RPC layer.
//
//	info, err := client.Mcps.ConnectionInfo("mcp-id")
//	session, err := mcp.NewClient(impl, nil).Connect(ctx, &mcp.StreamableClientTransport{
//	    Endpoint:   info.URL,
//	    HTTPClient: info.HTTPClient(nil),
//	}, nil)
func (r *McpsResource) ConnectionInfo(mcpID string) (*MCPConnectionInfo, error) {
	endpoint, err := r.transport.buildURL("/v1/mcps/"+urlEscape(mcpID)+"/mcp", nil)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"MCP-Protocol-Version": MCPProtocolVersion,
	}
	// Reuse the transport's own auth and custom-header rules so the proxy path
	// cannot drift from the REST paths.
	for key, values := range r.transport.buildHeaders("application/json, text/event-stream", true, nil) {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return &MCPConnectionInfo{URL: endpoint, Headers: headers}, nil
}

// HTTPClient returns an *http.Client that attaches Headers to every request,
// which is how the official Go MCP SDK expects transport-level auth to be
// supplied (its transports take an HTTPClient, not a header map).
func (i *MCPConnectionInfo) HTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{}
	}
	clone := *base
	inner := clone.Transport
	if inner == nil {
		inner = http.DefaultTransport
	}
	headers := make(map[string]string, len(i.Headers))
	for key, value := range i.Headers {
		headers[key] = value
	}
	clone.Transport = mcpHeaderRoundTripper{base: inner, headers: headers}
	return &clone
}

type mcpHeaderRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t mcpHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	for key, value := range t.headers {
		// Set, not Add: a caller-supplied duplicate must not shadow gateway auth.
		clone.Header.Set(key, value)
	}
	return t.base.RoundTrip(clone)
}
