package agentctlsdk

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ChatStreamProcessor struct {
	handlers ChatStreamHandlers
	buffer   strings.Builder
	text     strings.Builder
}

func NewChatStreamProcessor(handlers ChatStreamHandlers) *ChatStreamProcessor {
	return &ChatStreamProcessor{handlers: handlers}
}

func (p *ChatStreamProcessor) WriteSSEChunk(chunk string) {
	p.buffer.WriteString(chunk)
	parts := splitSSEBlocks(p.buffer.String())
	if len(parts) == 0 {
		return
	}

	p.buffer.Reset()
	last := parts[len(parts)-1]
	complete := parts[:len(parts)-1]

	if !strings.HasSuffix(last, "\n\n") && !strings.HasSuffix(last, "\r\n\r\n") {
		p.buffer.WriteString(last)
	} else {
		complete = parts
	}

	for _, block := range complete {
		for _, event := range ParseSSE(block) {
			p.handleEvent(event)
		}
	}
}

func (p *ChatStreamProcessor) WriteWebSocketMessage(message string) error {
	event, err := ParseWebSocketEvent(message)
	if err != nil {
		return err
	}
	p.handleEvent(event)
	return nil
}

func (p *ChatStreamProcessor) End() string {
	if p.buffer.Len() > 0 {
		for _, event := range ParseSSE(p.buffer.String()) {
			p.handleEvent(event)
		}
		p.buffer.Reset()
	}
	return p.text.String()
}

func (p *ChatStreamProcessor) handleEvent(event ChatStreamEvent) {
	if p.handlers.OnEvent != nil {
		p.handlers.OnEvent(event)
	}
	delta := TextFromStreamEvent(event)
	if delta == "" {
		return
	}
	p.text.WriteString(delta)
	if p.handlers.OnTextDelta != nil {
		p.handlers.OnTextDelta(delta, event)
	}
}

func ParseSSE(text string) []ChatStreamEvent {
	blocks := strings.FieldsFunc(text, func(r rune) bool { return false })
	_ = blocks

	var events []ChatStreamEvent
	for _, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		lines := strings.Split(block, "\n")
		eventName := "message"
		var dataLines []string

		for _, line := range lines {
			line = strings.TrimSuffix(line, "\r")
			switch {
			case strings.HasPrefix(line, "event:"):
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimLeft(strings.TrimPrefix(line, "data:"), " "))
			}
		}

		if len(dataLines) == 0 {
			continue
		}

		dataText := strings.Join(dataLines, "\n")
		var data any = dataText
		if err := json.Unmarshal([]byte(dataText), &data); err != nil {
			data = dataText
		}

		events = append(events, ChatStreamEvent{
			Event: eventName,
			Data:  data,
		})
	}

	return events
}

func ParseWebSocketEvent(message string) (ChatStreamEvent, error) {
	var parsed any
	if err := json.Unmarshal([]byte(message), &parsed); err != nil {
		return ChatStreamEvent{Event: "message", Data: message}, nil
	}

	object, ok := parsed.(map[string]any)
	if !ok {
		return ChatStreamEvent{Event: "message", Data: parsed}, nil
	}

	eventName, _ := object["event"].(string)
	if eventName == "" {
		eventName = "message"
	}

	if eventName == "error" {
		code, _ := object["code"].(string)
		errorText, _ := object["error"].(string)
		if errorText == "" {
			errorText = fmt.Sprint(object)
		}
		if code != "" {
			return ChatStreamEvent{}, fmt.Errorf("%s: %s", code, errorText)
		}
		return ChatStreamEvent{}, fmt.Errorf("%s", errorText)
	}

	return ChatStreamEvent{
		Event: eventName,
		Data:  object["data"],
	}, nil
}

func TextFromStreamEvent(event ChatStreamEvent) string {
	if event.Event == "response.text.delta" || event.Event == "response.output_text.delta" {
		return stringField(event.Data, "delta")
	}
	if event.Event == "chat.response" || event.Event == "message.delta" {
		if value := stringField(event.Data, "content"); value != "" {
			return value
		}
		if value := stringField(event.Data, "text"); value != "" {
			return value
		}
		return stringField(event.Data, "delta")
	}
	return ""
}

func stringField(data any, field string) string {
	object, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := object[field].(string)
	return value
}

func splitSSEBlocks(text string) []string {
	if text == "" {
		return nil
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	var blocks []string
	start := 0
	for {
		idx := strings.Index(text[start:], "\n\n")
		if idx < 0 {
			blocks = append(blocks, text[start:])
			break
		}
		end := start + idx + 2
		blocks = append(blocks, text[start:end])
		start = end
		if start >= len(text) {
			break
		}
	}
	return blocks
}
