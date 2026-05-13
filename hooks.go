package agentctlsdk

import "context"

type HooksResource struct {
	transport *Transport
}

func (r *HooksResource) Register(ctx context.Context, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/hooks/register", payload, &result)
	return result, err
}

func (r *HooksResource) List(ctx context.Context, options HookListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/hooks", QueryParams{
		"search": options.Search,
		"limit":  options.Limit,
		"offset": options.Offset,
	}, &result)
	return result, err
}

func (r *HooksResource) Get(ctx context.Context, hookID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/hooks/"+urlEscape(hookID), nil, &result)
	return result, err
}

func (r *HooksResource) Update(ctx context.Context, hookID string, payload any) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/hooks/"+urlEscape(hookID), payload, &result)
	return result, err
}

func (r *HooksResource) Delete(ctx context.Context, hookID string) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/hooks/"+urlEscape(hookID), nil, &result)
	return result, err
}
