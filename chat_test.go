package seaagentsdk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChatCompletionBodySupportsMultimodalMessages(t *testing.T) {
	body := chatCompletionBody(ChatCompletionRequest{
		AgentID: "agent_1",
		Messages: []ChatMessage{{
			Role: "user",
			Content: []ChatContentPart{
				TextChatContent("描述这张图片"),
				ImageURLChatContent("https://image.cdn2.seaart.me/a.png"),
			},
		}},
	})

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"content":[`) {
		t.Fatalf("content was not encoded as parts array: %s", raw)
	}
	if !strings.Contains(raw, `"text":"描述这张图片"`) {
		t.Fatalf("text part missing: %s", raw)
	}
	if !strings.Contains(raw, `"image_url":{"url":"https://image.cdn2.seaart.me/a.png"}`) {
		t.Fatalf("image_url part missing: %s", raw)
	}
}

func TestChatCompletionBodyKeepsStringMessages(t *testing.T) {
	body := chatCompletionBody(ChatCompletionRequest{
		AgentID:  "agent_1",
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	if !strings.Contains(string(data), `"content":"hello"`) {
		t.Fatalf("string content changed: %s", string(data))
	}
}

func TestChatCompletionBodyIncludesSkillIDs(t *testing.T) {
	body := chatCompletionBody(ChatCompletionRequest{
		AgentID:  "agent_1",
		SkillIDs: []string{"11111111-1111-1111-1111-111111111111"},
		Messages: []ChatMessage{{Role: "user", Content: "hello"}},
	})

	values, ok := body["skill_ids"].([]string)
	if !ok {
		t.Fatalf("skill_ids type = %T, want []string", body["skill_ids"])
	}
	if len(values) != 1 || values[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("skill_ids = %#v", values)
	}
}
