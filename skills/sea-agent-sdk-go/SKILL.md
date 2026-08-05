---
name: sea-agent-sdk-go
description: Integrate Go services with SeaArt Agent Gateway through the official sea-agent-sdk-go. Use for catalog lookup, Tool, MCP Server, Skill, Agent, Hook, chat completion, SSE or WebSocket streaming, chat replay, and cancellation in Go.
---

# SeaAgent Go SDK

Use `github.com/SeaArt-Infra/sea-agent-sdk-go` for Agent Gateway work in Go. Prefer its client and stream helpers over hand-written HTTP or SSE code.

## Workflow

1. Inspect `go.mod` and use Go 1.24.3 or newer.
2. Add the SDK with `go get github.com/SeaArt-Infra/sea-agent-sdk-go`.
3. Create one `NewClient` with the gateway endpoint, API key, and any global headers.
4. Use the resource on the client that matches the operation.
5. Run a focused Go test or `go test ./...` after changing the integration.

The SDK appends `/agent-v2` when the configured endpoint does not already contain it. Store the API key outside source control. Send `X-User-ID` for Tool, MCP Server, Skill, and Agent writes when the gateway requires owner or operator metadata.

## Create A Client

```go
client := seaagentsdk.NewClient(seaagentsdk.ClientOptions{
    Endpoint: os.Getenv("AGENT_GATEWAY_ENDPOINT"),
    APIKey:   os.Getenv("AGENT_GATEWAY_API_KEY"),
    Headers: map[string]string{
        "X-User-ID": userID,
    },
})
```

Use `NewClientFromConfig("")` only when the service intentionally shares `~/.seaagent/config.yaml`.

## Run And Stream Chat

Use `Message` for a single user turn and `Messages` for a multi-turn or multimodal request. Do not set both `AgentConfig` and `SkillIDs`; `SkillIDs` add temporary Skills to an Agent run.

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
    AgentID: agentID,
    Message: "Summarize this request.",
})
if err != nil {
    return err
}
_ = result
```

Use SSE by default. Use WebSocket only when the caller needs a persistent connection or manages a WebSocket lifecycle.

```go
text, err := client.Chat.RunStream(ctx, seaagentsdk.ChatRunOptions{
    AgentID: agentID,
    Message: "Explain the result as it arrives.",
}, seaagentsdk.ChatStreamHandlers{
    Transport: seaagentsdk.StreamTransportSSE,
    OnTextDelta: func(delta string, _ seaagentsdk.ChatStreamEvent) {
        fmt.Print(delta)
    },
})
if err != nil {
    return err
}
fmt.Println(text)
```

Preserve the default reconnect behavior unless product requirements demand a different retry policy. Use `Chat.Events`, `Chat.Stream`, or `Chat.Cancel` to replay, resume, or cancel an existing chat.

## Per-Chat Reasoning

Use the top-level `ChatRunOptions.ReasoningEffort` option only to override the
selected Agent for this run. Leave it empty when the caller did not choose a
level so the Agent and Fabric defaults remain effective. The supported platform
values are `off`, `on`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max`, and
`ultra`; use the exported `ReasoningEffort*` constants and only select values
verified for the Agent's model route. Do not send provider-specific thinking
fields through `ExtraBody`.

## Agent Default Reasoning

To save a default level on an Agent, set `model.reasoning_effort` in the
concise registration payload. A chat without `ChatRunOptions.ReasoningEffort`
uses that default; an explicit chat value applies only to that chat. Full
create and update payloads use `model_config.reasoning_effort` instead.

## Select Resources

| Task | Client resource |
| --- | --- |
| Health or metrics | `System` |
| Resolved catalog entries | `Catalog` |
| Tool registration and resolution | `Tools` |
| MCP Server registration and tool proxying | `Mcps` |
| Skill registration and listing | `Skills` |
| Agent registration and inspection | `Agents` |
| Multimodal charge reservation hook | `Hooks` |
| Chat, streaming, replay, cancellation | `Chat` |

## Manage MCP Servers

Use `client.Mcps` for `Register`, `List`, `Get`, `Update`, `Delete`, `Tools`, and `Call`. Registration and updates accept `streamable-http` or legacy `sse` transports; `Call` accepts `{ "name": ..., "arguments": ..., "timeout_ms": ... }`. Include both `X-User-ID` and `X-Flag: 1` for MCP mutations. Gateway never returns stored upstream header values, only `header_keys`; access to a private server's `Tools` and `Call` operations requires its owner or `X-Admin-Access: 1`.

Pass list filters through the corresponding option struct. Keep custom gateway fields in `ExtraBody` only when the SDK has no typed option for them. Put request-specific HTTP headers in `ChatRunOptions.Headers`, not in the JSON body.

## Agent Skill Preload

Agent registration keeps `skills` as an array of Skill UUIDs. Add the UUID of
a short instruction needed on every run to `pre_skills` as well: gateway
injects it into the resolved system prompt and avoids the initial Worker
`read_file` call for its `SKILL.md`. Skills only in `skills` remain
progressively loaded by Worker. `pre_skills` must be a duplicate-free subset
of `skills`; every bound Skill keeps its tool bindings.

## Medium-Term Memory Policy

For a registered Agent, use optional `config.memory_policy` in a concise
registration payload or `agent_config.memory_policy` in a low-level
create/update payload. Omit it for the normal persistent-session behavior;
use it to restrict a particular Agent:

```go
"config": map[string]any{
	"memory_policy": map[string]any{
		"medium_term": map[string]any{"recall": false, "learn": false},
	},
},
```

For a complete persistent session, `medium_term.recall` and
`medium_term.learn` both default to `true`. `recall` retrieves relevant
semantic memory as background context; `learn` queues a qualifying completed
run for asynchronous extraction rather than saving it synchronously. Both
default to `false` for ephemeral runs (no `metadata.session_id`) and are forced
off by a missing memory scope, user opt-out, or Worker
`MEMORY_MEDIUM_TERM_ENABLED=false`. Agent policy and request-level
`memory_policy` only restrict; pass a request-level override through
`ChatRunOptions.ExtraBody`. Long-term recall and writes remain disabled by
default.

## Verify And Protect Data

Run `go test ./...` from the module root. Verify a health check or a non-streaming chat before adding streaming UI behavior. Do not expose gateway API keys in browser code, commits, logs, errors, or telemetry. Redact complete prompts and raw Tool output from diagnostic logs.
