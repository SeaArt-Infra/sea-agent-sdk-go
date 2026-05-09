package agentctlsdk

import (
	"context"
	"net/url"
)

type ChatResource struct {
	transport *Transport
}

func (r *ChatResource) CreateCompletion(ctx context.Context, payload ChatCompletionRequest) (any, error) {
	body := chatCompletionBody(payload)
	var result any
	err := r.transport.PostJSON(ctx, "/v1/chat/completions", body, &result)
	return result, err
}

func (r *ChatResource) StreamCompletion(ctx context.Context, payload ChatCompletionRequest, handlers ChatStreamHandlers) (string, error) {
	processor := NewChatStreamProcessor(handlers)
	body := chatCompletionBody(payload)
	body["stream"] = true

	var err error
	if handlers.Transport == StreamTransportWS {
		err = r.transport.WebSocket(ctx, "/v1/chat/completions/ws", nil, body, func(message string) {
			if wsErr := processor.WriteWebSocketMessage(message); wsErr != nil && err == nil {
				err = wsErr
			}
		})
	} else {
		err = r.transport.PostStream(ctx, "/v1/chat/completions", body, processor.WriteSSEChunk)
	}
	if err != nil {
		return "", err
	}
	return processor.End(), nil
}

func (r *ChatResource) Run(ctx context.Context, options ChatRunOptions) (any, error) {
	return r.CreateCompletion(ctx, buildRunPayload(options, false))
}

func (r *ChatResource) RunStream(ctx context.Context, options ChatRunOptions, handlers ChatStreamHandlers) (string, error) {
	return r.StreamCompletion(ctx, buildRunPayload(options, true), handlers)
}

func (r *ChatResource) Get(ctx context.Context, chatID string) (any, error) {
	var result any
	err := r.transport.GetJSON(ctx, "/v1/chats/"+url.PathEscape(chatID), nil, &result)
	return result, err
}

func (r *ChatResource) Events(ctx context.Context, chatID string, options ChatEventsOptions) (any, error) {
	var result any
	limit := options.Limit
	if limit == 0 {
		limit = 100
	}

	err := r.transport.GetJSON(ctx, "/v1/chats/"+url.PathEscape(chatID)+"/events", QueryParams{
		"after_seq": options.AfterSeq,
		"limit":     limit,
	}, &result)
	return result, err
}

func (r *ChatResource) Stream(ctx context.Context, chatID string, handlers ChatStreamHandlers, options ChatEventsOptions) (string, error) {
	processor := NewChatStreamProcessor(handlers)
	query := QueryParams{
		"after_seq": options.AfterSeq,
	}

	var err error
	if handlers.Transport == StreamTransportWS {
		err = r.transport.WebSocket(ctx, "/v1/chats/"+url.PathEscape(chatID)+"/ws", query, nil, func(message string) {
			if wsErr := processor.WriteWebSocketMessage(message); wsErr != nil && err == nil {
				err = wsErr
			}
		})
	} else {
		err = r.transport.GetStream(ctx, "/v1/chats/"+url.PathEscape(chatID)+"/stream", query, processor.WriteSSEChunk)
	}

	if err != nil {
		return "", err
	}
	return processor.End(), nil
}

func (r *ChatResource) Cancel(ctx context.Context, chatID string) (any, error) {
	var result any
	err := r.transport.PostJSON(ctx, "/v1/chats/"+url.PathEscape(chatID)+"/cancel", nil, &result)
	return result, err
}

func buildRunPayload(options ChatRunOptions, stream bool) ChatCompletionRequest {
	messages := options.Messages
	if len(messages) == 0 {
		messages = []ChatMessage{{Role: "user", Content: options.Message}}
	}

	return ChatCompletionRequest{
		AgentID:     options.AgentID,
		AgentConfig: options.AgentConfig,
		Messages:    messages,
		Stream:      stream,
		ExtraBody:   options.ExtraBody,
	}
}

func chatCompletionBody(payload ChatCompletionRequest) map[string]any {
	body := map[string]any{
		"messages": payload.Messages,
		"stream":   payload.Stream,
	}
	if payload.AgentID != "" {
		body["agent_id"] = payload.AgentID
	}
	if payload.AgentConfig != nil {
		body["agent_config"] = payload.AgentConfig
	}
	for key, value := range payload.ExtraBody {
		body[key] = value
	}
	return body
}
