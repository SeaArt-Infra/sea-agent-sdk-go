package agentctlsdk

import "net/http"

type QueryParams map[string]any

type Config struct {
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
}

type ClientOptions struct {
	Endpoint   string
	APIKey     string
	HTTPClient *http.Client
}

type DeleteOptions struct {
	OperatorID string
}

type PaginationOptions struct {
	Limit  int
	Offset int
}

type CatalogListOptions struct {
	CapabilityType string
	Search         string
	Status         string
	SourceKind     string
	OwnerID        string
	Provider       string
	Category       string
	Limit          int
	Offset         int
}

type ToolListOptions struct {
	Search     string
	Status     string
	SourceKind string
	OwnerID    string
	Provider   string
	Category   string
	Limit      int
	Offset     int
}

type SkillListOptions struct {
	Search     string
	Status     string
	SourceKind string
	OwnerID    string
	Provider   string
	Category   string
	Limit      int
	Offset     int
}

type AgentListOptions struct {
	Search   string
	Status   string
	OwnerID  string
	Category string
	Limit    int
	Offset   int
}

type HookListOptions struct {
	Search string
	Limit  int
	Offset int
}

type ChatEventsOptions struct {
	AfterSeq int
	Limit    int
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	AgentID     string         `json:"agent_id,omitempty"`
	AgentConfig map[string]any `json:"agent_config,omitempty"`
	Messages    []ChatMessage  `json:"messages"`
	Stream      bool           `json:"stream,omitempty"`
	ExtraBody   map[string]any `json:"-"`
}

type ChatRunOptions struct {
	AgentID     string
	AgentConfig map[string]any
	Message     string
	Messages    []ChatMessage
	ExtraBody   map[string]any
}

type StreamTransport string

const (
	StreamTransportSSE StreamTransport = "sse"
	StreamTransportWS  StreamTransport = "ws"
)

type ChatStreamEvent struct {
	Event string
	Data  any
}

type ChatStreamHandlers struct {
	Transport   StreamTransport
	OnEvent     func(ChatStreamEvent)
	OnTextDelta func(string, ChatStreamEvent)
}
