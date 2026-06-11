package seaagentsdk

import "fmt"

type Client struct {
	Endpoint  string
	APIKey    string
	Transport *Transport
	System    *SystemResource
	Catalog   *CatalogResource
	Tools     *ToolsResource
	Skills    *SkillsResource
	Agents    *AgentsResource
	Hooks     *HooksResource
	Chat      *ChatResource
}

func NewClient(options ClientOptions) *Client {
	endpoint := normalizeAgentGatewayEndpoint(options.Endpoint)
	transport := NewTransport(endpoint, options.APIKey, options.Headers, options.HTTPClient)

	client := &Client{
		Endpoint:  endpoint,
		APIKey:    options.APIKey,
		Transport: transport,
	}

	client.System = &SystemResource{transport: transport}
	client.Catalog = &CatalogResource{transport: transport}
	client.Tools = &ToolsResource{transport: transport}
	client.Skills = &SkillsResource{transport: transport}
	client.Agents = &AgentsResource{transport: transport}
	client.Hooks = &HooksResource{transport: transport}
	client.Chat = &ChatResource{transport: transport}

	return client
}

func NewClientFromConfig(path string) (*Client, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is not configured. Expected ~/.seaagent/config.yaml or a custom config path")
	}

	return NewClient(ClientOptions{
		Endpoint: cfg.Endpoint,
		APIKey:   cfg.APIKey,
		Headers:  headersFromConfig(cfg),
	}), nil
}

func headersFromConfig(cfg Config) map[string]string {
	if cfg.UserID == "" {
		return nil
	}
	return map[string]string{"X-User-ID": cfg.UserID}
}
