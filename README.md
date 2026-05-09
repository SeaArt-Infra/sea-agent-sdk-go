# agentctl-sdk-go

基于当前 `agentctl` CLI 项目整理出的 Go SDK，用于调用 agent-gateway 的注册、查询和聊天接口。

## 安装

```bash
go get github.com/seaart-infra/agentctl-sdk-go
```

## 初始化

```go
package main

import (
	"context"
	"fmt"

	agentctlsdk "github.com/seaart-infra/agentctl-sdk-go"
)

func main() {
	client := agentctlsdk.NewClient(agentctlsdk.ClientOptions{
		Endpoint: "http://127.0.0.1:8080",
		APIKey:   "sa-xxxxxxxx",
	})

	health, err := client.System.Health(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Println(health)
}
```

也可以复用 CLI 的默认配置文件：

```go
client, err := agentctlsdk.NewClientFromConfig("")
if err != nil {
	panic(err)
}
```

默认读取 `~/.agentctl/config.yaml`，格式与 CLI 一致：

```yaml
endpoint: http://127.0.0.1:8080
apiKey: sa-xxxxxxxx
```

## 示例

```go
ctx := context.Background()

client := agentctlsdk.NewClient(agentctlsdk.ClientOptions{
	Endpoint: "http://127.0.0.1:8080",
	APIKey:   "sa-xxxxxxxx",
})

tools, err := client.Tools.List(ctx, agentctlsdk.ToolListOptions{
	Provider: "web-tools-mcp",
	Status:   "active",
})
if err != nil {
	panic(err)
}

fmt.Printf("%#v\n", tools)
```

流式聊天：

```go
text, err := client.Chat.RunStream(
	context.Background(),
	agentctlsdk.ChatRunOptions{
		AgentID: "web_assistant:v1",
		Message: "Fetch https://example.com",
	},
	agentctlsdk.ChatStreamHandlers{
		Transport: agentctlsdk.StreamTransportSSE,
		OnTextDelta: func(delta string, event agentctlsdk.ChatStreamEvent) {
			fmt.Print(delta)
		},
	},
)
if err != nil {
	panic(err)
}

fmt.Println("\nFinal text:", text)
```
