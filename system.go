package agentctlsdk

import "context"

type SystemResource struct {
	transport *Transport
}

func (r *SystemResource) Health(ctx context.Context) (string, error) {
	return r.transport.GetText(ctx, "/health", nil)
}

func (r *SystemResource) Metrics(ctx context.Context) (string, error) {
	return r.transport.GetText(ctx, "/metrics", nil)
}
