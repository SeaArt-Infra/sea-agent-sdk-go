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
		"source_kind":     options.SourceKind,
		"owner_id":        options.OwnerID,
		"public":          options.Public,
		"provider":        options.Provider,
		"category":        options.Category,
		"limit":           options.Limit,
		"offset":          options.Offset,
	}, &result)
	return result, err
}
