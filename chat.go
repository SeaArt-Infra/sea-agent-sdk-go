package seaagentsdk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	defaultMaxReconnects     = 3
	defaultReconnectDelay    = 250 * time.Millisecond
	defaultMaxReconnectDelay = 5 * time.Second
)

var (
	errStreamEndedBeforeTerminal = errors.New("stream ended before a terminal event")
	errStreamTerminalReached     = errors.New("stream terminal event reached")
)

type streamProcessingError struct {
	err error
}

func (e *streamProcessingError) Error() string { return e.err.Error() }
func (e *streamProcessingError) Unwrap() error { return e.err }

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
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	if payload.RequestID == "" {
		if requestID, ok := payload.ExtraBody["request_id"].(string); ok && strings.TrimSpace(requestID) != "" {
			payload.RequestID = strings.TrimSpace(requestID)
		} else {
			payload.RequestID = newStreamRequestID()
		}
	}
	body := chatCompletionBody(payload)
	body["stream"] = true
	body["request_id"] = payload.RequestID

	initial := func() error {
		if handlers.Transport == StreamTransportWS {
			return r.transport.webSocketWithHeaders(ctx, "/v1/chat/completions/ws", nil, body, payload.Headers, processorWebSocketCallback(processor))
		}
		return r.transport.requestStreamWithHeadersCallback(ctx, "POST", "/v1/chat/completions", nil, body, payload.Headers, processorSSECallback(processor))
	}
	replay := func(runID string, afterSeq int64) error {
		return r.streamExistingOnce(ctx, runID, afterSeq, payload.Headers, handlers.Transport, processor)
	}
	return runStreamWithResume(ctx, handlers, processor, initial, replay)
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
	processor.RunID = chatID
	processor.LastSeq = int64(options.AfterSeq)
	initial := func() error {
		return r.streamExistingOnce(ctx, chatID, int64(options.AfterSeq), nil, handlers.Transport, processor)
	}
	replay := func(runID string, afterSeq int64) error {
		return r.streamExistingOnce(ctx, runID, afterSeq, nil, handlers.Transport, processor)
	}
	return runStreamWithResume(ctx, handlers, processor, initial, replay)
}

func (r *ChatResource) streamExistingOnce(ctx context.Context, chatID string, afterSeq int64, headers map[string]string, transport StreamTransport, processor *ChatStreamProcessor) error {
	query := QueryParams{"after_seq": afterSeq}
	path := "/v1/chats/" + url.PathEscape(chatID)
	if transport == StreamTransportWS {
		return r.transport.webSocketWithHeaders(ctx, path+"/ws", query, nil, headers, processorWebSocketCallback(processor))
	}
	return r.transport.requestStreamWithHeadersCallback(ctx, "GET", path+"/stream", query, nil, headers, processorSSECallback(processor))
}

func processorSSECallback(processor *ChatStreamProcessor) func(string) error {
	return func(chunk string) error {
		processor.WriteSSEChunk(chunk)
		if processor.Terminal {
			return errStreamTerminalReached
		}
		return nil
	}
}

func processorWebSocketCallback(processor *ChatStreamProcessor) func(string) error {
	return func(message string) error {
		if err := processor.WriteWebSocketMessage(message); err != nil {
			return &streamProcessingError{err: err}
		}
		if processor.Terminal {
			return errStreamTerminalReached
		}
		return nil
	}
}

func runStreamWithResume(
	ctx context.Context,
	handlers ChatStreamHandlers,
	processor *ChatStreamProcessor,
	initial func() error,
	replay func(string, int64) error,
) (string, error) {
	reconnects := 0
	connect := initial
	for {
		err := connect()
		if processor.Terminal {
			processor.DiscardSSEBuffer()
			return processor.Text(), nil
		}
		processor.DiscardSSEBuffer()

		if ctxErr := ctx.Err(); ctxErr != nil {
			return processor.Text(), ctxErr
		}
		if handlers.DisableAutoResume {
			return processor.Text(), err
		}

		cause := err
		if cause == nil {
			cause = errStreamEndedBeforeTerminal
		}
		if !isRetryableStreamError(cause) {
			return processor.Text(), cause
		}

		maxReconnects := handlers.MaxReconnects
		if maxReconnects == 0 {
			maxReconnects = defaultMaxReconnects
		}
		if reconnects >= maxReconnects {
			return processor.Text(), fmt.Errorf("stream did not reach a terminal event after %d reconnects: %w", reconnects, cause)
		}

		reconnects++
		delay := reconnectDelay(handlers, reconnects)
		info := ChatStreamReconnectInfo{
			Attempt:  reconnects,
			RunID:    processor.RunID,
			AfterSeq: processor.LastSeq,
			Delay:    delay,
			Err:      cause,
		}
		if handlers.OnReconnect != nil {
			handlers.OnReconnect(info)
		}
		if err := waitForReconnect(ctx, delay); err != nil {
			return processor.Text(), err
		}

		if processor.RunID != "" {
			connect = func() error { return replay(processor.RunID, processor.LastSeq) }
		} else {
			connect = initial
		}
	}
}

func reconnectDelay(handlers ChatStreamHandlers, attempt int) time.Duration {
	delay := handlers.ReconnectDelay
	if delay == 0 {
		delay = defaultReconnectDelay
	}
	maxDelay := handlers.MaxReconnectDelay
	if maxDelay == 0 {
		maxDelay = defaultMaxReconnectDelay
	}
	if delay < 0 {
		delay = 0
	}
	if maxDelay < delay {
		maxDelay = delay
	}
	for i := 1; i < attempt && delay < maxDelay; i++ {
		if delay > maxDelay/2 {
			return maxDelay
		}
		delay *= 2
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

func waitForReconnect(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryableStreamError(err error) bool {
	var processingErr *streamProcessingError
	if errors.As(err, &processingErr) || errors.Is(err, context.Canceled) {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 408 || httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	return true
}

func newStreamRequestID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err == nil {
		return "sdk_" + hex.EncodeToString(raw)
	}
	return fmt.Sprintf("sdk_%d", time.Now().UnixNano())
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
