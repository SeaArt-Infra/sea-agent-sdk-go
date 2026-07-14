package seaagentsdk

import "context"

type HooksResource struct {
	transport *Transport
}

func (r *HooksResource) Register(ctx context.Context, payload HookRequest) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/hooks/register", payload, &result)
	return result, err
}

func (r *HooksResource) Update(ctx context.Context, payload HookRequest) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/hooks", payload, &result)
	return result, err
}

func (r *HooksResource) Delete(ctx context.Context) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/hooks", nil, &result)
	return result, err
}
