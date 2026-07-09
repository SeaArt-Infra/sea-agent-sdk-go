package seaagentsdk

import "net/http"

type QueryParams map[string]any

type Config struct {
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	UserID   string `yaml:"userId"`
}

type ClientOptions struct {
	Endpoint   string
	APIKey     string
	Headers    map[string]string
	HTTPClient *http.Client
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
	Public         *bool
	Provider       string
	Category       string
	Limit          int
	Offset         int
}

type ToolListOptions struct {
	Search         string
	Status         string
	SourceKind     string
	OwnerID        string
	Public         *bool
	Provider       string
	Category       string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

type SkillListOptions struct {
	Search         string
	Status         string
	SourceKind     string
	OwnerID        string
	Public         *bool
	Provider       string
	Category       string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

type AgentListOptions struct {
	Search         string
	Status         string
	OwnerID        string
	Category       string
	IncludeDeleted bool
	Limit          int
	Offset         int
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

type ChatContentURL struct {
	URL string `json:"url"`
}

type ChatContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *ChatContentURL `json:"image_url,omitempty"`
	VideoURL *ChatContentURL `json:"video_url,omitempty"`
}

func TextChatContent(text string) ChatContentPart {
	return ChatContentPart{Type: "text", Text: text}
}

func ImageURLChatContent(url string) ChatContentPart {
	return ChatContentPart{Type: "image_url", ImageURL: &ChatContentURL{URL: url}}
}

func VideoURLChatContent(url string) ChatContentPart {
	return ChatContentPart{Type: "video_url", VideoURL: &ChatContentURL{URL: url}}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ChatCompletionRequest struct {
	RequestID   string            `json:"request_id,omitempty"`
	AgentID     string            `json:"agent_id,omitempty"`
	Category    string            `json:"category,omitempty"`
	AgentConfig map[string]any    `json:"agent_config,omitempty"`
	SkillIDs    []string          `json:"skill_ids,omitempty"`
	Messages    []ChatMessage     `json:"messages"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Headers     map[string]string `json:"-"`
	ExtraBody   map[string]any    `json:"-"`
}

type ChatRunOptions struct {
	RequestID   string
	AgentID     string
	Category    string
	AgentConfig map[string]any
	SkillIDs    []string
	Message     string
	Messages    []ChatMessage
	Metadata    map[string]any
	Headers     map[string]string
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
