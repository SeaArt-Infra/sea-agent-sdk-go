# sea-agent-sdk-go

> Beta: SDK APIs and `agent-gateway` behavior may still change with gateway versions.

Go SDK for `agent-gateway`. It wraps the gateway APIs for catalog lookup, resource registration, chat completion, SSE streaming, WebSocket streaming, chat replay, and hook management.

## Available Resources

| Resource | Client field | What it does |
| --- | --- | --- |
| System | `client.System` | Health and metrics checks |
| Catalog | `client.Catalog` | List resolved catalog entries |
| Tools | `client.Tools` | Register, list, update, delete, and resolve tools |
| Skills | `client.Skills` | Register, list, update, and delete skills |
| Agents | `client.Agents` | Register, list, update, delete, and inspect agents |
| Hooks | `client.Hooks` | Manage the multimodal charge reservation hook |
| Chat | `client.Chat` | Run chat, stream chat, replay events, and cancel chats |

## How It Works

1. Create a `Client` with an agent-gateway endpoint and optional API key.
2. The SDK normalizes the endpoint to include `/agent-v2` when needed.
3. Each resource helper sends gateway-compatible HTTP requests with global and per-request headers.
4. Chat helpers can either return a full response or process SSE/WebSocket events through callbacks.
5. Streaming helpers automatically resume transient disconnects from the last delivered event sequence.

`X-User-ID` is required for `tools`, `skills`, and `agents` write operations when the gateway needs provider, owner, or operator metadata.

## Quick Start

Install the module:

```bash
go get github.com/SeaArt-Infra/sea-agent-sdk-go
```

The current Go module path is `github.com/SeaArt-Infra/sea-agent-sdk-go`.

Requires Go 1.24.3 or newer, matching the version declared in `go.mod`.

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

`endpoint` may be the gateway base URL or a URL that already includes `/agent-v2`. The SDK appends `/agent-v2` before sending requests when it is missing. Non-streaming requests use a default timeout of 180 seconds. Pass a custom `ClientOptions.HTTPClient` to override it.

## Listing Resources

List APIs pass gateway filters through SDK option structs. Common filters are `Search`, `Status`, `Provider`, `Public`, `Limit`, and `Offset`. Compatibility filters include `SourceKind`, `OwnerID`, and `Category`.

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

Use `SkillIDs` to temporarily mount extra Skills for a registered Agent run when it needs one-off capabilities without changing its saved configuration. Agent Gateway accepts at most 20 active, visible Skill UUIDs, merges them after the Agent's own Skills, dedupes repeated IDs, rejects `SkillIDs` when `AgentConfig` is used, and only lets Skill runtime config fill Agent defaults that are unset.

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID:  "33333333-3333-4333-8333-333333333333",
	SkillIDs: []string{"11111111-1111-1111-1111-111111111111"},
	Message:  "Use the extra skill for this run.",
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

`request_id`, `category`, and `metadata` are sent in the chat body. Custom headers are forwarded when the SDK creates non-streaming, SSE, or WebSocket chat requests. Use `ExtraBody` for gateway fields that are not yet exposed as first-class SDK options.

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
			fmt.Printf("[%d] %s", event.Seq, delta)
		},
		OnEvent: func(event seaagentsdk.ChatStreamEvent) {
			// Record metrics or inspect tool-call events here.
			_ = event
		},
		OnReconnect: func(info seaagentsdk.ChatStreamReconnectInfo) {
			log.Printf("resuming run=%s after_seq=%d (attempt %d)", info.RunID, info.AfterSeq, info.Attempt)
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

### Worker Stream Event Format

`agent-gateway` forwards worker stream events as SSE blocks or WebSocket messages. The SDK normalizes both transports into `ChatStreamEvent{ID, Seq, Event, Data}`. `ID` preserves the protocol event ID and `Seq` contains its numeric sequence, or zero for heartbeat and unnumbered events. Use `OnTextDelta` for assistant text and `OnEvent` for all raw lifecycle, tool, skill, and terminal events.

SSE frames use the standard event/data envelope:

```text
event: response.text.delta
data: {"type":"response.text.delta","response_id":"run_xxx","item_id":"item_run_xxx_msg","output_index":0,"content_index":0,"delta":"hello"}
id: 12
```

WebSocket frames carry the same payload under `data`:

```json
{
  "id": "12",
  "event": "response.text.delta",
  "data": {
    "type": "response.text.delta",
    "response_id": "run_xxx",
    "item_id": "item_run_xxx_msg",
    "output_index": 0,
    "content_index": 0,
    "delta": "hello"
  }
}
```

Common worker event sequence:

| Event | When it appears | Important fields in `Data` |
| --- | --- | --- |
| `response.created` | Run accepted and response object created | `type`, `response.id`, `response.status`, `response.model`, `response.metadata` |
| `response.in_progress` | Run enters processing | `type`, `response.id`, `response.status` |
| `response.output_item.added` | Assistant message item or tool call item starts | `response_id`, `output_index`, `item.type`, `item.id`, `item.status`; tool calls also include `item.call_id`, `item.name` |
| `response.content_part.added` | Assistant text content part starts | `response_id`, `item_id`, `output_index`, `content_index`, `part.type` |
| `chat.delta` | Legacy assistant text chunk | `content`, `text`, or `delta` |
| `response.text.delta` | Assistant text token/chunk | `response_id`, `item_id`, `output_index`, `content_index`, `delta` |
| `response.function_call_arguments.done` | Tool call arguments are finalized | `response_id`, `item_id`, `call_id`, `name`, `arguments` as a JSON string |
| `fabric.tool.started` | Worker starts a tool call | `tool.id`, `tool.call_id`, `tool.name`, `tool.status`, `tool.arguments` |
| `fabric.tool.completed` | Worker finishes a tool call | `tool.status`, `tool.output`, `tool.output_text`, `tool.output_type`; structured tool protocols may add `tool.structured_content`, `tool.protocol_type`, `tool.tool_response` |
| `fabric.skill.started` | Worker loads a skill through a `read_file` tool call | `skill.id`, `skill.name`, `skill.status`, `skill.path` |
| `fabric.skill.completed` | Skill file load completes | `skill.status`, `skill.output`, `skill.output_text`, `skill.path` |
| `response.text.done` | Assistant final text is known | `response_id`, `item_id`, `content_index`, `text` |
| `response.content_part.done` | Assistant text content part completes | `part.type`, `part.text` |
| `response.output_item.done` | Assistant message or function call output item completes | `item.type`, `item.status`, `item.content` for messages; `item.call_id`, `item.arguments`, `item.output` for tool calls |
| `response.completed` | Run completed successfully | `response.id`, `response.status`, `response.usage`, `response.elapsed_ms`, `response.metadata`, `response.output` |
| `response.failed` | Run failed | `response.status`, `response.error.type`, `response.error.code`, `response.error.message` |
| `response.cancelled` | Run was cancelled | `response.status`, `response.cancel_reason` |

The SDK accumulates returned text from `response.text.delta`. It also keeps compatibility with `chat.delta`, legacy `response.output_text.delta`, `chat.response`, and `message.delta` text events. Tool, skill, usage, metadata, and terminal details are not passed to `OnTextDelta`; inspect them in `OnEvent`.

The recognized terminal events are `response.completed`, `response.failed`, `response.cancelled`, `response.canceled`, `chat.response`, `chat.completed`, `chat.failed`, and `chat.cancelled`. The SDK stops reading as soon as one is delivered; events after a terminal event are ignored.

### Automatic Stream Resume

`RunStream`, `StreamCompletion`, and `Stream` automatically resume an interrupted stream until a terminal event is received. The SDK records `run_id` from `chat.created` and the last delivered `Seq`, then reconnects through the chat replay endpoint with `after_seq`. Duplicate replayed sequence numbers are not delivered to callbacks. If a create stream disconnects before `chat.created`, the SDK retries creation with the same `request_id`; when the caller did not provide one, the SDK generates it for that call. Request-specific headers are also forwarded to replay requests.

By default, the SDK makes up to three reconnect attempts with exponential delays from 250 milliseconds to 5 seconds. Customize the policy through `ChatStreamHandlers`:

```go
handlers := seaagentsdk.ChatStreamHandlers{
	MaxReconnects:     5,
	ReconnectDelay:    500 * time.Millisecond,
	MaxReconnectDelay: 10 * time.Second,
	OnReconnect: func(info seaagentsdk.ChatStreamReconnectInfo) {
		log.Printf("attempt=%d run=%s after_seq=%d delay=%s err=%v",
			info.Attempt, info.RunID, info.AfterSeq, info.Delay, info.Err)
	},
}
```

| Handler field | Default | Behavior |
| --- | --- | --- |
| `DisableAutoResume` | `false` | Set to `true` when the caller owns reconnection |
| `MaxReconnects` | `3` | Reconnect attempts after the initial connection; zero selects the default |
| `ReconnectDelay` | `250ms` | Initial reconnect delay |
| `MaxReconnectDelay` | `5s` | Maximum exponential-backoff delay |
| `OnReconnect` | `nil` | Receives `ChatStreamReconnectInfo{Attempt, RunID, AfterSeq, Delay, Err}` |

Context cancellation, explicit WebSocket error events, and non-transient HTTP errors stop immediately; HTTP 408, 429, 5xx, premature EOF, and network errors are eligible for reconnection. Incomplete SSE frames from a broken connection are discarded before replay. The default stream client has no total response timeout, so use the Go context to set the desired stream deadline. A custom `ClientOptions.HTTPClient` and its timeout are preserved for both normal and streaming requests.

Automatic resume covers connection loss while the current process is running. Persist `RunID` and `event.Seq`, then call `Stream` with `AfterSeq`, when recovery must survive a process restart.

## Replay an Existing Chat

If another SDK client or application created the chat, subscribe by chat ID. `AfterSeq` starts from events after the specified sequence number. If that connection is interrupted, the SDK continues automatically from the last delivered event.

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

Register the single hook endpoint owned by the configured API key:

```go
request := seaagentsdk.HookRequest{
	Name:        "production-line-hook",
	Endpoint:    "https://example.com/agent-hook",
	Description: "Receives multimodal charge reservation events for the configured API key.",
}
hook, err := client.Hooks.Register(ctx, request)
updated, err := client.Hooks.Update(ctx, request)
deleted, err := client.Hooks.Delete(ctx)
```

Hook management requests use `ClientOptions.APIKey` as `Authorization: Bearer ...`; registration and update request fields are `name`, `endpoint`, and `description`. One API key owns at most one active Hook. Registration creates a Hook and returns `409 Conflict` when one is already active; after deletion, the same API key can register again.

### Callback event: `multimodal.charge.reserve`

The Worker sends this event with fixed `POST` immediately before submitting a multimodal model operation. Callback `metadata` is copied from the individual chat request:

```json
{
  "event_id": "evt_...",
  "event": "multimodal.charge.reserve",
  "run_id": "run_...",
  "metadata": {},
  "data": {
    "operation_id": "op_...",
    "tool_name": "generate",
    "model": "model-name",
    "modality": "multimodal",
    "cost": "0.035",
    "currency": "USD"
  }
}
```

For this event, the endpoint must synchronously return an HTTP success status and a top-level JSON object. Approval returns `{"approved":true}`. Rejection returns `{"approved":false}` and can include `code` and `message`, for example `{"approved":false,"code":"insufficient_balance","message":"Balance is insufficient"}`.

## API Reference

| Area | Methods |
| --- | --- |
| System | `Health(ctx)`, `Metrics(ctx)` |
| Catalog | `List(ctx, options)` |
| Tools | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, toolID)`, `Update(ctx, toolID, payload)`, `Delete(ctx, toolID)`, `Resolve(ctx, toolID)` |
| Skills | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, skillID)`, `Update(ctx, skillID, payload)`, `Delete(ctx, skillID)` |
| Agents | `Register(ctx, payload)`, `List(ctx, options)`, `Get(ctx, agentID)`, `Update(ctx, agentID, payload)`, `Delete(ctx, agentID)`, `Capabilities(ctx, agentID)` |
| Hooks | `Register(ctx, payload)`, `Update(ctx, payload)`, `Delete(ctx)` |
| Chat | `CreateCompletion(ctx, payload)`, `StreamCompletion(ctx, payload, handlers)`, `Run(ctx, options)`, `RunStream(ctx, options, handlers)`, `Get(ctx, chatID)`, `Events(ctx, chatID, options)`, `Stream(ctx, chatID, handlers, options)`, `Cancel(ctx, chatID)` |

## Next Steps

- Start with `Chat.Run` for non-streaming requests.
- Use `Chat.RunStream` with SSE for most streaming integrations.
- Use `Chat.Stream` with `AfterSeq` to subscribe to an existing chat or resume after a process restart; active-process disconnects are handled automatically.
- Register tools, skills, and agents with UUID-based references only.
