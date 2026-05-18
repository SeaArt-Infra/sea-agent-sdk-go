package seaagentsdk

import "context"

type ToolsResource struct {
	transport *Transport
}

func (r *ToolsResource) Register(ctx context.Context, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/tools/register", payload, &result)
	return result, err
}

func (r *ToolsResource) List(ctx context.Context, options ToolListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/tools", QueryParams{
		"search":   options.Search,
		"status":   options.Status,
		"public":   options.Public,
		"provider": options.Provider,
		"limit":    options.Limit,
		"offset":   options.Offset,
	}, &result)
	return result, err
}

func (r *ToolsResource) Get(ctx context.Context, toolID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/tools/"+urlEscape(toolID), nil, &result)
	return result, err
}

func (r *ToolsResource) Update(ctx context.Context, toolID string, payload any) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/tools/"+urlEscape(toolID), payload, &result)
	return result, err
}

func (r *ToolsResource) Delete(ctx context.Context, toolID string, options DeleteOptions) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/tools/"+urlEscape(toolID), nil, &result)
	return result, err
}

func (r *ToolsResource) Resolve(ctx context.Context, toolID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/tools/"+urlEscape(toolID)+"/resolve", nil, &result)
	return result, err
}
