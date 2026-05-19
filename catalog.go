package seaagentsdk

import "context"

type CatalogResource struct {
	transport *Transport
}

func (r *CatalogResource) List(ctx context.Context, options CatalogListOptions) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/catalog", QueryParams{
		"capability_type": options.CapabilityType,
		"search":          options.Search,
		"status":          options.Status,
		"public":          options.Public,
		"provider":        options.Provider,
		"limit":           options.Limit,
		"offset":          options.Offset,
	}, &result)
	return result, err
}
