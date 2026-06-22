# sea-agent-sdk-go

> Beta: SDK API 和 agent-gateway 行为仍可能随网关版本调整。

Go SDK for `agent-gateway`. It wraps the gateway APIs for catalog lookup, resource registration, chat completion, SSE streaming, WebSocket streaming, chat replay, and hook management.

## Available Resources

| Resource | Client field | What it does |
| --- | --- | --- |
| System | `client.System` | Health and metrics checks |
| Catalog | `client.Catalog` | List resolved catalog entries |
| Tools | `client.Tools` | Register, list, update, delete, and resolve tools |
| Skills | `client.Skills` | Register, list, update, and delete skills |
| Agents | `client.Agents` | Register, list, update, delete, and inspect agents |
| Hooks | `client.Hooks` | Register and manage worker event hook endpoints |
| Chat | `client.Chat` | Run chat, stream chat, replay events, and cancel chats |

## How It Works

1. Create a `Client` with an agent-gateway endpoint and optional API key.
2. The SDK normalizes the endpoint to include `/agent-v2` when needed.
3. Each resource helper sends gateway-compatible HTTP requests with global and per-request headers.
4. Chat helpers can either return a full response or process SSE/WebSocket events through callbacks.

`X-User-ID` is required for `tools`, `skills`, and `agents` write operations when the gateway needs provider, owner, or operator metadata. `NewClientFromConfig` maps `userId` from the CLI config to `X-User-ID`.

## Quick Start

Install the module:

```bash
go get github.com/SeaArt-Infra/sea-agent-sdk-go
```

The current Go module path is `github.com/SeaArt-Infra/sea-agent-sdk-go`.

Create a client and run a chat request:

```go
package main

import (
	"context"
	"fmt"
	"os"

	seaagentsdk "github.com/SeaArt-Infra/sea-agent-sdk-go"
)

func main() {
	ctx := context.Background()
	client := seaagentsdk.NewClient(seaagentsdk.ClientOptions{
		Endpoint: "http://127.0.0.1:8080",
		APIKey:   os.Getenv("AGENT_GATEWAY_API_KEY"),
		Headers: map[string]string{
			"X-User-ID": "production-line-123",
		},
	})

	result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
		AgentID: "33333333-3333-4333-8333-333333333333",
		Message: "Search recent AI news and summarize the top 3 items.",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", result)
}
```

Check gateway health:

```go
health, err := client.System.Health(context.Background())
if err != nil {
	panic(err)
}

fmt.Println(health)
```

## Configuration

Pass options directly:

```go
client := seaagentsdk.NewClient(seaagentsdk.ClientOptions{
	Endpoint: "http://127.0.0.1:8080",
	APIKey:   os.Getenv("AGENT_GATEWAY_API_KEY"),
	Headers: map[string]string{
		"X-User-ID": "production-line-123",
	},
})
```

Or reuse the CLI config:

```go
client, err := seaagentsdk.NewClientFromConfig("")
if err != nil {
	panic(err)
}
```

By default, the SDK reads `~/.seaagent/config.yaml`:

```yaml
endpoint: http://127.0.0.1:8080
apiKey: sa-xxxxxxxx
userId: production-line-123
```

`endpoint` may be the gateway base URL or a URL that already includes `/agent-v2`. The SDK appends `/agent-v2` before sending requests when it is missing.

## Listing Resources

List APIs follow CLI and gateway filters. Common filters are `Search`, `Status`, `Provider`, `Public`, `Limit`, and `Offset`. Compatibility filters include `SourceKind`, `OwnerID`, and `Category`.

```go
tools, err := client.Tools.List(ctx, seaagentsdk.ToolListOptions{
	Provider: "web-tools-mcp",
	Status:   "active",
	Limit:    20,
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", tools)
```

Pagination follows the gateway behavior: `Limit` defaults to 20 when omitted or `<= 0`, the gateway caps values above 200, and `Offset` starts at 0.

## Chat Requests

Use `Message` for the common single-user-message case:

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID: "33333333-3333-4333-8333-333333333333",
	Message: "Fetch https://example.com and explain what it is.",
})
```

Use `Messages` for multi-turn conversations:

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID: "33333333-3333-4333-8333-333333333333",
	Messages: []seaagentsdk.ChatMessage{
		{Role: "system", Content: "Answer in concise Chinese."},
		{Role: "user", Content: "Fetch https://example.com and explain what it is."},
	},
})
```

Use OpenAI-style content parts for multimodal messages:

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID: "33333333-3333-4333-8333-333333333333",
	Messages: []seaagentsdk.ChatMessage{
		{
			Role: "user",
			Content: []seaagentsdk.ChatContentPart{
				seaagentsdk.TextChatContent("Describe this image."),
				seaagentsdk.ImageURLChatContent("https://example.com/image.png"),
			},
		},
	},
})
```

Attach request metadata and per-request headers when gateway or worker tracing needs them:

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	RequestID: "req_123",
	AgentID:   "33333333-3333-4333-8333-333333333333",
	Category:  "fabric",
	Message:   "Summarize this request context.",
	Metadata: map[string]any{
		"session_id": "sess_123",
		"user_id":    "user_456",
		"trace_id":   "trace_789",
	},
	Headers: map[string]string{
		"X-Trace-ID": "trace_789",
	},
})
```

`request_id`, `category`, and `metadata` are sent in the chat body. Custom headers are forwarded when the SDK creates non-streaming, SSE, or WebSocket chat requests.

## Streaming

SSE is the default stream transport and works well with most HTTP gateways and proxies:

```go
text, err := client.Chat.RunStream(
	ctx,
	seaagentsdk.ChatRunOptions{
		AgentID: "33333333-3333-4333-8333-333333333333",
		Message: "Fetch https://example.com and summarize it in one paragraph.",
	},
	seaagentsdk.ChatStreamHandlers{
		Transport: seaagentsdk.StreamTransportSSE,
		OnTextDelta: func(delta string, event seaagentsdk.ChatStreamEvent) {
			fmt.Print(delta)
		},
		OnEvent: func(event seaagentsdk.ChatStreamEvent) {
			// Record metrics or inspect tool-call events here.
			_ = event
		},
	},
)
if err != nil {
	panic(err)
}

fmt.Println("\n\nFinal text:", text)
```

Switch to WebSocket when the caller wants a persistent connection or already manages WebSocket lifecycle:

```go
text, err := client.Chat.RunStream(
	ctx,
	seaagentsdk.ChatRunOptions{
		AgentID: "33333333-3333-4333-8333-333333333333",
		Message: "Tell me what tools you can use, then answer with a short plan.",
	},
	seaagentsdk.ChatStreamHandlers{
		Transport: seaagentsdk.StreamTransportWS,
		OnTextDelta: func(delta string, event seaagentsdk.ChatStreamEvent) {
			fmt.Print(delta)
		},
		OnEvent: func(event seaagentsdk.ChatStreamEvent) {
			if event.Event == "error" {
				fmt.Printf("stream error event: %#v\n", event.Data)
			}
		},
	},
)
```

## Replay an Existing Chat

If another process, browser page, or CLI command created the chat, subscribe by chat ID. `AfterSeq` resumes from events after the specified sequence number.

```go
chatID := "chat_xxxxxxxxxxxxx"

text, err := client.Chat.Stream(
	context.Background(),
	chatID,
	seaagentsdk.ChatStreamHandlers{
		Transport: seaagentsdk.StreamTransportSSE,
		OnTextDelta: func(delta string, event seaagentsdk.ChatStreamEvent) {
			fmt.Print(delta)
		},
	},
	seaagentsdk.ChatEventsOptions{
		AfterSeq: 0,
	},
)
if err != nil {
	panic(err)
}

fmt.Println("\n\nReceived text:", text)
```

Use `StreamTransportWS` with the same API to replay over WebSocket.

## Inline Agent Config

Pass `AgentConfig` when the request should not reference a registered agent. Runtime fields such as `temperature`, `max_turns`, and `timeout` are forwarded by `agent-gateway` to the worker.

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	Category: "fabric",
	AgentConfig: map[string]any{
		"agent": map[string]any{
			"name":             "inline-assistant",
			"model":            "gpt-4.1-mini",
			"reasoning_effort": "medium",
			"temperature":      0.2,
			"max_turns":        6,
			"timeout":          120,
			"system_prompt":    "Answer in Chinese and keep the answer brief.",
		},
	},
	Message: "Explain what agent-gateway does.",
})
```

Declare a sandbox template when the gateway should start a sandbox for the inline agent. Supported template values are `react-game` and `react-web`.

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	Category: "fabric",
	AgentConfig: map[string]any{
		"agent": map[string]any{
			"name":          "inline-sandbox-agent",
			"model":         "gpt-4.1-mini",
			"system_prompt": "Build and modify React apps inside the sandbox.",
		},
		"runtime": map[string]any{
			"sandbox": map[string]any{
				"sandbox_template": "react-game",
			},
		},
	},
	Message: "Create a small React game.",
})
```

## Register Tools, Skills, and Agents

`agent-gateway` uses server-generated UUID `id` values as resource identities. Registry lookup and association should use UUIDs; do not send removed `tool_key`, `skill_key`, or `agent_key` fields.

Register an HTTP tool:

```go
tool, err := client.Tools.Register(ctx, map[string]any{
	"name":         "search_web",
	"description":  "Search public web pages.",
	"runtime_type": "http",
	"endpoint":     "https://example.com/tools/search",
	"service_name": "example",
	"method":       "POST",
	"parameters": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
		"required": []string{"query"},
	},
	"enabled": true,
	"public":  false,
})
```

`service_name` is a top-level tool field beside `name`. It identifies the backing service shared by tools on the same server. If omitted, the gateway derives it from the endpoint host prefix; builtin and no-endpoint tools default to `deepagent`. Do not put `service_name` in metadata/config, and do not send `inject_user_credentials` in user-facing registration payloads.

Register a skill:

```go
skill, err := client.Skills.Register(ctx, map[string]any{
	"name":        "web-research",
	"description": "Research a topic with web tools.",
	"instruction": "Search, compare sources, and summarize findings.",
	"required_tools": []string{
		"22222222-2222-4222-8222-222222222222",
	},
	"enabled": true,
	"public":  false,
})
```

When `required_tools` or `optional_tools` contains registered HTTP Tool UUID strings, the gateway normalizes them to `{"type":"http","ref":"<tool-uuid>"}`. Use object entries when you need non-default tool types:

```go
"required_tools": []map[string]any{
	{"type": "http", "ref": "22222222-2222-4222-8222-222222222222"},
	{"type": "builtin", "ref": "seaart:generate_image"},
	{"type": "mcp", "ref": "filesystem:read_file", "server": "mcp-filesystem"},
},
```

`type` is required and must be `http`, `http_batch`, `builtin`, or `mcp`. MCP entries also require `server`. Do not use Tool `name` or old `tool_key` values as `ref`.

Register an agent:

```go
agent, err := client.Agents.Register(ctx, map[string]any{
	"name":          "web_assistant",
	"category":      "fabric",
	"system_prompt": "You are a web research assistant.",
	"skills":        []string{"11111111-1111-4111-8111-111111111111"},
	"config": map[string]any{
		"temperature": 0.2,
		"max_turns":   6,
	},
	"enabled": true,
})
```

## Skill Runtime Rules

| Field | Rule |
| --- | --- |
| `name` | Must match `^[a-z0-9-]+$`; use lowercase letters, numbers, and hyphens only |
| `description` | Required; keep it short because the gateway writes it to inline `SKILL.md` frontmatter |
| `instruction` | Required; full Markdown body for the skill |
| `required_tools` / `optional_tools` | Use UUID refs for registered HTTP, HTTP Batch, and registered builtin tools |

When an agent runs with a registered skill, the gateway assembles an inline skill document:

```md
---
name: web-research
description: Research a topic with web tools.
---

Search, compare sources, and summarize findings.
```

## Hook Endpoints

Register a hook endpoint for worker events:

```go
hook, err := client.Hooks.Register(ctx, map[string]any{
	"name":        "production-line-hook",
	"endpoint":    "https://example.com/agent-hook",
	"description": "Receives Agent Worker events for the configured API key.",
	"metadata":    map[string]any{},
})
```

Hooks use `ClientOptions.APIKey` as `Authorization: Bearer ...`; do not send `api_key` in the payload. Worker calls use `POST`, and the receiver should filter by `event_id` in the event payload when needed.

## API Reference

| Area | Methods |
| --- | --- |
| System | `Health(ctx)`, `Metrics(ctx)` |
| Catalog | `List(ctx, options)` |
| Tools | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, toolID)`, `Update(ctx, toolID, payload)`, `Delete(ctx, toolID)`, `Resolve(ctx, toolID)` |
| Skills | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, skillID)`, `Update(ctx, skillID, payload)`, `Delete(ctx, skillID)` |
| Agents | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, agentID)`, `Update(ctx, agentID, payload)`, `Delete(ctx, agentID)`, `Capabilities(ctx, agentID)` |
| Hooks | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, hookID)`, `Update(ctx, hookID, payload)`, `Delete(ctx, hookID)` |
| Chat | `CreateCompletion(ctx, payload)`, `StreamCompletion(ctx, payload, handlers)`, `Run(ctx, options)`, `RunStream(ctx, options, handlers)`, `Get(ctx, chatID)`, `Events(ctx, chatID, options)`, `Stream(ctx, chatID, handlers, options)`, `Cancel(ctx, chatID)` |

## Debugging

Set `SEAAGENT_DEBUG=1` to print outgoing HTTP and WebSocket requests:

```bash
export SEAAGENT_DEBUG=1
```

## Next Steps

- Start with `Chat.Run` for non-streaming requests.
- Use `Chat.RunStream` with SSE for most streaming integrations.
- Use `Chat.Stream` with `AfterSeq` to resume an existing chat.
- Register tools, skills, and agents with UUID-based references only.
