package seaagentsdk

import "context"

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
