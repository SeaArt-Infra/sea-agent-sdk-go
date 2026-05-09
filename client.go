package agentctlsdk

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
	Chat      *ChatResource
}

func NewClient(options ClientOptions) *Client {
	transport := NewTransport(options.Endpoint, options.APIKey, options.HTTPClient)

	client := &Client{
		Endpoint:  options.Endpoint,
		APIKey:    options.APIKey,
		Transport: transport,
	}

	client.System = &SystemResource{transport: transport}
	client.Catalog = &CatalogResource{transport: transport}
	client.Tools = &ToolsResource{transport: transport}
	client.Skills = &SkillsResource{transport: transport}
	client.Agents = &AgentsResource{transport: transport}
	client.Chat = &ChatResource{transport: transport}

	return client
}

func NewClientFromConfig(path string) (*Client, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is not configured. Expected ~/.agentctl/config.yaml or a custom config path")
	}

	return NewClient(ClientOptions{
		Endpoint: cfg.Endpoint,
		APIKey:   cfg.APIKey,
	}), nil
}
