package agentctlsdk

import "context"

type SkillsResource struct {
	transport *Transport
}

func (r *SkillsResource) Register(ctx context.Context, payload any) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/skills/register", payload, &result)
	return result, err
}

func (r *SkillsResource) List(ctx context.Context, options SkillListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/skills", QueryParams{
		"search":      options.Search,
		"status":      options.Status,
		"source_kind": options.SourceKind,
		"owner_id":    options.OwnerID,
		"provider":    options.Provider,
		"category":    options.Category,
		"limit":       options.Limit,
		"offset":      options.Offset,
	}, &result)
	return result, err
}

func (r *SkillsResource) Get(ctx context.Context, skillID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/skills/"+urlEscape(skillID), nil, &result)
	return result, err
}

func (r *SkillsResource) Update(ctx context.Context, skillID string, payload any) (any, error) {
	var result any
	err := r.transport.PutJSON(ctx, "/v1/skills/"+urlEscape(skillID), payload, &result)
	return result, err
}

func (r *SkillsResource) Delete(ctx context.Context, skillID string, options DeleteOptions) (any, error) {
	var result any
	err := r.transport.DeleteJSON(ctx, "/v1/skills/"+urlEscape(skillID), QueryParams{
		"operator_id": options.OperatorID,
	}, &result)
	return result, err
}
