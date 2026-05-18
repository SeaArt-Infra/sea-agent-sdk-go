# sea-agent-sdk-go

基于当前 `agentctl` CLI 项目整理出的 Go SDK，用于调用 `agent-gateway` 的注册、查询、聊天、SSE 流式响应和 WebSocket 流式响应接口。

## 安装

```bash
go get github.com/SeaVerseAI/sea-agent-sdk-go
```

## 初始化

```go
package main

import (
	"context"
	"fmt"
	"os"

	seaagentsdk "github.com/SeaVerseAI/sea-agent-sdk-go"
)

func main() {
	client := seaagentsdk.NewClient(seaagentsdk.ClientOptions{
		Endpoint: "http://127.0.0.1:8080",
		APIKey:   os.Getenv("AGENT_GATEWAY_API_KEY"),
		Headers: map[string]string{
			"X-User-ID": "production-line-123",
		},
	})

	health, err := client.System.Health(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("%#v\n", health)
}
```

也可以复用 CLI 的默认配置文件：

```go
client, err := seaagentsdk.NewClientFromConfig("")
if err != nil {
	panic(err)
}
```

默认读取 `~/.agentctl/config.yaml`，格式与 CLI 一致：

```yaml
endpoint: http://127.0.0.1:8080
apiKey: sa-xxxxxxxx
```

`X-User-ID` 用于 `tools`、`skills`、`agents` 的注册和更新接口，`agent-gateway` 会用它写入 provider、owner 和操作人字段。也可以通过 `ClientOptions.Headers` 配置其他全局请求头。

## 基础示例

查询工具列表：

```go
ctx := context.Background()

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

普通非流式聊天：

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID: "production-line-123:web_assistant:v1",
	Message: "Search recent AI news and summarize the top 3 items.",
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", result)
```

使用多轮消息：

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	AgentID: "production-line-123:web_assistant:v1",
	Messages: []seaagentsdk.ChatMessage{
		{Role: "system", Content: "Answer in concise Chinese."},
		{Role: "user", Content: "Fetch https://example.com and explain what it is."},
	},
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", result)
```

带请求元数据和自定义 Header 的聊天：

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	RequestID: "req_123",
	AgentID:   "production-line-123:web_assistant:v1",
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
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", result)
```

`request_id`、`category`、`metadata` 会进入 `agent-gateway` 的 chat 请求体；自定义 Headers 会透传给 agent-worker，SSE 和 WebSocket 创建聊天时都支持。

## SSE 流式聊天

SSE 是默认流式传输方式，底层使用 HTTP `text/event-stream`，适合大多数 HTTP 网关和代理场景。

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	seaagentsdk "github.com/SeaVerseAI/sea-agent-sdk-go"
)

func main() {
	client := seaagentsdk.NewClient(seaagentsdk.ClientOptions{
		Endpoint: "http://127.0.0.1:8080",
		APIKey:   os.Getenv("AGENT_GATEWAY_API_KEY"),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	text, err := client.Chat.RunStream(
		ctx,
		seaagentsdk.ChatRunOptions{
			AgentID: "production-line-123:web_assistant:v1",
			Message: "Fetch https://example.com and summarize it in one paragraph.",
		},
		seaagentsdk.ChatStreamHandlers{
			Transport: seaagentsdk.StreamTransportSSE,
			OnTextDelta: func(delta string, event seaagentsdk.ChatStreamEvent) {
				fmt.Print(delta)
			},
			OnEvent: func(event seaagentsdk.ChatStreamEvent) {
				// 可用于记录日志、统计指标、处理工具调用事件等。
				_ = event
			},
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("\n\nFinal text:", text)
}
```

## WebSocket 流式聊天

如果调用方希望使用持久连接，或者运行环境已经统一管理 WebSocket 生命周期，可以将 `Transport` 切换为 `StreamTransportWS`。

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

text, err := client.Chat.RunStream(
	ctx,
	seaagentsdk.ChatRunOptions{
		AgentID: "production-line-123:web_assistant:v1",
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
if err != nil {
	panic(err)
}

fmt.Println("\n\nFinal text:", text)
```

## 订阅已有 Chat

如果 Chat 由其他进程、浏览器页面或 CLI 创建，可以通过 Chat ID 继续订阅后续事件。`AfterSeq` 用于从指定事件序号之后恢复。

SSE：

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

WebSocket：

```go
chatID := "chat_xxxxxxxxxxxxx"

text, err := client.Chat.Stream(
	context.Background(),
	chatID,
	seaagentsdk.ChatStreamHandlers{
		Transport: seaagentsdk.StreamTransportWS,
		OnTextDelta: func(delta string, event seaagentsdk.ChatStreamEvent) {
			fmt.Print(delta)
		},
	},
	seaagentsdk.ChatEventsOptions{
		AfterSeq: 10,
	},
)
if err != nil {
	panic(err)
}

fmt.Println("\n\nReceived text:", text)
```

## 使用内联 Agent 配置

如果不想引用已注册的 Agent ID，可以直接传入 `AgentConfig`。`temperature`、`max_turns`、`timeout` 等运行时字段会由 `agent-gateway` 透传给 agent-worker：

```go
result, err := client.Chat.Run(ctx, seaagentsdk.ChatRunOptions{
	Category: "fabric",
	AgentConfig: map[string]any{
		"agent": map[string]any{
			"name":          "inline-assistant",
			"model":         "gpt-4.1-mini",
			"temperature":   0.2,
			"max_turns":     6,
			"timeout":       120,
			"system_prompt": "Answer in Chinese and keep the answer brief.",
		},
	},
	Message: "Explain what agent-gateway does.",
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", result)
```

如果 Agent 需要由 `agent-gateway` 自动拉起 sandbox，可以在 `AgentConfig` 中声明 `runtime.sandbox.sandbox_template`。当前支持的模板枚举为 `react-game` 和 `react-web`：

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
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", result)
```

## 注册 Tool、Skill 和 Agent

`agent-gateway` 现在由服务端生成 `tool_key`、`skill_key`、`agent_key`，其中 Agent key 形如 `owner_id:name:version`。注册和创建 payload 中不要传这些字段，否则会返回 `400`。

注册工具：

```go
tool, err := client.Tools.Register(ctx, map[string]any{
	"name":        "search_web",
	"version":     "v1",
	"description": "Search public web pages.",
	"transport":   "http",
	"endpoint":    "https://example.com/tools/search",
	"method":      "POST",
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
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", tool)
```

注册技能：

```go
skill, err := client.Skills.Register(ctx, map[string]any{
	"name":        "web_research",
	"version":     "v1",
	"description": "Research a topic with web tools.",
	"instruction": "Search, compare sources, and summarize findings.",
	"required_tools": []map[string]any{
		{"ref": "production-line-123:search_web:v1"},
	},
	"enabled": true,
	"public":  false,
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", skill)
```

注册 Agent：

```go
agent, err := client.Agents.Register(ctx, map[string]any{
	"name":          "web_assistant",
	"version":       "v1",
	"category":      "fabric",
	"system_prompt": "You are a web research assistant.",
	"skills":        []string{"production-line-123:web_research:v1"},
	"config": map[string]any{
		"temperature": 0.2,
		"max_turns":   6,
	},
	"enabled": true,
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", agent)
```

## 注册 Hook endpoint

```go
hook, err := client.Hooks.Register(ctx, map[string]any{
	"name":        "production-line-hook",
	"endpoint":    "https://example.com/agent-hook",
	"description": "Receives Agent Worker events for the configured API key.",
	"metadata":    map[string]any{},
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", hook)
```

Hook 使用 `ClientOptions.APIKey` 作为 `Authorization: Bearer ...`，payload 中不要传 `api_key`。Worker 固定用 `POST` 调用 endpoint，业务方按事件 payload 中的 `event_id` 自行过滤。

## 资源接口

- `client.System.Health(ctx)`
- `client.System.Metrics(ctx)`
- `client.Catalog.List(ctx, options)`
- `client.Tools.Register(ctx, payload)`
- `client.Tools.List(ctx, options)`
- `client.Tools.Get(ctx, toolID)`
- `client.Tools.Update(ctx, toolID, payload)`
- `client.Tools.Delete(ctx, toolID, options)`
- `client.Tools.Resolve(ctx, toolID)`
- `client.Skills.Register(ctx, payload)`
- `client.Skills.List(ctx, options)`
- `client.Skills.Get(ctx, skillID)`
- `client.Skills.Update(ctx, skillID, payload)`
- `client.Skills.Delete(ctx, skillID, options)`
- `client.Agents.Register(ctx, payload)`
- `client.Agents.List(ctx, options)`
- `client.Agents.Update(ctx, agentID, payload)`
- `client.Agents.Delete(ctx, agentID, options)`
- `client.Agents.Capabilities(ctx, agentID)`
- `client.Hooks.Register(ctx, payload)`
- `client.Hooks.List(ctx, options)`
- `client.Hooks.Get(ctx, hookID)`
- `client.Hooks.Update(ctx, hookID, payload)`
- `client.Hooks.Delete(ctx, hookID)`
- `client.Chat.CreateCompletion(ctx, payload)`
- `client.Chat.StreamCompletion(ctx, payload, handlers)`
- `client.Chat.Run(ctx, options)`
- `client.Chat.RunStream(ctx, options, handlers)`
- `client.Chat.Get(ctx, chatID)`
- `client.Chat.Events(ctx, chatID, options)`
- `client.Chat.Stream(ctx, chatID, handlers, options)`
- `client.Chat.Cancel(ctx, chatID)`

## 调试

设置任一环境变量后，SDK 会打印发出的 HTTP 和 WebSocket 请求：

```bash
export SEAAGENT_DEBUG=1
# 或
export AGENTCTL_DEBUG=1
```
