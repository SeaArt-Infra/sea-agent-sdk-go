package seaagentsdk

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
	err := r.transport.PostJSONWithHeaders(ctx, "/v1/chat/completions", body, payload.Headers, &result)
	return result, err
}

func (r *ChatResource) StreamCompletion(ctx context.Context, payload ChatCompletionRequest, handlers ChatStreamHandlers) (string, error) {
	processor := NewChatStreamProcessor(handlers)
	body := chatCompletionBody(payload)
	body["stream"] = true

	var err error
	if handlers.Transport == StreamTransportWS {
		err = r.transport.WebSocketWithHeaders(ctx, "/v1/chat/completions/ws", nil, body, payload.Headers, func(message string) {
			if wsErr := processor.WriteWebSocketMessage(message); wsErr != nil && err == nil {
				err = wsErr
			}
		})
	} else {
		err = r.transport.PostStreamWithHeaders(ctx, "/v1/chat/completions", body, payload.Headers, processor.WriteSSEChunk)
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
		RequestID:   options.RequestID,
		AgentID:     options.AgentID,
		Category:    options.Category,
		AgentConfig: options.AgentConfig,
		SkillIDs:    options.SkillIDs,
		Messages:    messages,
		Metadata:    options.Metadata,
		Stream:      stream,
		Headers:     options.Headers,
		ExtraBody:   options.ExtraBody,
	}
}

func chatCompletionBody(payload ChatCompletionRequest) map[string]any {
	body := map[string]any{
		"messages": payload.Messages,
		"stream":   payload.Stream,
	}
	if payload.RequestID != "" {
		body["request_id"] = payload.RequestID
	}
	if payload.AgentID != "" {
		body["agent_id"] = payload.AgentID
	}
	if payload.Category != "" {
		body["category"] = payload.Category
	}
	if payload.AgentConfig != nil {
		body["agent_config"] = payload.AgentConfig
	}
	if len(payload.SkillIDs) > 0 {
		body["skill_ids"] = payload.SkillIDs
	}
	if payload.Metadata != nil {
		body["metadata"] = payload.Metadata
	}
	for key, value := range payload.ExtraBody {
		body[key] = value
	}
	return body
}
