---
name: sea-agent-sdk-go
description: Integrate Go services with SeaArt Agent Gateway through the official sea-agent-sdk-go. Use for catalog lookup, Tool, Skill, Agent, Hook, chat completion, SSE or WebSocket streaming, chat replay, and cancellation in Go.
---

# SeaAgent Go SDK

Use `github.com/SeaArt-Infra/sea-agent-sdk-go` for Agent Gateway work in Go. Prefer its client and stream helpers over hand-written HTTP or SSE code.

## Workflow

1. Inspect `go.mod` and use Go 1.24.3 or newer.
2. Add the SDK with `go get github.com/SeaArt-Infra/sea-agent-sdk-go`.
3. Create one `NewClient` with the gateway endpoint, API key, and any global headers.
4. Use the resource on the client that matches the operation.
5. Run a focused Go test or `go test ./...` after changing the integration.

The SDK appends `/agent-v2` when the configured endpoint does not already contain it. Store the API key outside source control. Send `X-User-ID` for Tool, Skill, and Agent writes when the gateway requires owner or operator metadata.

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

## Select Resources

| Task | Client resource |
| --- | --- |
| Health or metrics | `System` |
| Resolved catalog entries | `Catalog` |
| Tool registration and resolution | `Tools` |
| Skill registration and listing | `Skills` |
| Agent registration and inspection | `Agents` |
| Multimodal charge reservation hook | `Hooks` |
| Chat, streaming, replay, cancellation | `Chat` |

Pass list filters through the corresponding option struct. Keep custom gateway fields in `ExtraBody` only when the SDK has no typed option for them. Put request-specific HTTP headers in `ChatRunOptions.Headers`, not in the JSON body.

## Verify And Protect Data

Run `go test ./...` from the module root. Verify a health check or a non-streaming chat before adding streaming UI behavior. Do not expose gateway API keys in browser code, commits, logs, errors, or telemetry. Redact complete prompts and raw Tool output from diagnostic logs.
