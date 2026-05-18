package seaagentsdk

import "context"

type AgentsResource struct {
	transport *Transport
}

func (r *AgentsResource) Register(ctx context.Context, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/agents/register", payload, &result)
	return result, err
}

func (r *AgentsResource) Update(ctx context.Context, agentID string, payload any) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/agents/"+urlEscape(agentID), payload, &result)
	return result, err
}

func (r *AgentsResource) Delete(ctx context.Context, agentID string, options DeleteOptions) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/agents/"+urlEscape(agentID), nil, &result)
	return result, err
}

func (r *AgentsResource) List(ctx context.Context, options AgentListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/agents", QueryParams{
		"search":   options.Search,
		"status":   options.Status,
		"owner_id": options.OwnerID,
		"category": options.Category,
		"limit":    options.Limit,
		"offset":   options.Offset,
	}, &result)
	return result, err
}

func (r *AgentsResource) Capabilities(ctx context.Context, agentID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/agents/"+urlEscape(agentID)+"/capabilities", nil, &result)
	return result, err
}
