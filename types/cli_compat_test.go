package types

import (
	"encoding/json"
	"testing"
)

// These payloads are shapes the Claude CLI really emits, taken from
// ~/.claude/wee/logs where each one aborted a message parse with
// "[SDK WARNING] Failed to parse message from CLI".

// A tool_result whose tool_use_result is a bare string. The struct field was
// map[string]interface{}, so the whole user message failed to unmarshal and the
// tool result never reached the consumer.
func TestUserMessageToolUseResultAsString(t *testing.T) {
	raw := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Exit code 1\nnode: command not found"}]},"tool_use_result":"Exit code 1\nnode: command not found"}`

	var msg UserMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("string tool_use_result must parse, got: %v", err)
	}
	if msg.ToolUseResultText == nil {
		t.Fatal("expected ToolUseResultText to be populated for a string payload")
	}
	if *msg.ToolUseResultText != "Exit code 1\nnode: command not found" {
		t.Errorf("ToolUseResultText = %q", *msg.ToolUseResultText)
	}
	if msg.ToolUseResult != nil {
		t.Errorf("ToolUseResult should stay nil for a string payload, got %v", msg.ToolUseResult)
	}
	blocks, ok := msg.Content.([]ContentBlock)
	if !ok || len(blocks) != 1 {
		t.Fatalf("expected 1 content block, got %#v", msg.Content)
	}
}

// The object form must keep working exactly as before.
func TestUserMessageToolUseResultAsObject(t *testing.T) {
	raw := `{"type":"user","message":{"role":"user","content":"hi"},"tool_use_result":{"stdout":"ok","exit_code":0}}`

	var msg UserMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("object tool_use_result must parse, got: %v", err)
	}
	if msg.ToolUseResult == nil {
		t.Fatal("expected ToolUseResult map to be populated")
	}
	if msg.ToolUseResult["stdout"] != "ok" {
		t.Errorf("ToolUseResult[stdout] = %v", msg.ToolUseResult["stdout"])
	}
	if msg.ToolUseResultText != nil {
		t.Errorf("ToolUseResultText should stay nil for an object payload, got %q", *msg.ToolUseResultText)
	}
}

// Synthetic assistant messages carry "error" as a bare string rather than the
// documented {type, message} object.
func TestAssistantMessageErrorAsString(t *testing.T) {
	raw := `{"type":"assistant","message":{"model":"<synthetic>","role":"assistant","content":[{"type":"text","text":"x"}]},"error":"Prompt is too long"}`

	var msg AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("string error must parse, got: %v", err)
	}
	if msg.Error == nil {
		t.Fatal("expected Error to be populated")
	}
	if msg.Error.Message != "Prompt is too long" {
		t.Errorf("Error.Message = %q", msg.Error.Message)
	}
}

func TestAssistantMessageErrorAsObject(t *testing.T) {
	raw := `{"type":"assistant","message":{"model":"m","role":"assistant","content":[]},"error":{"type":"overloaded_error","message":"Overloaded"}}`

	var msg AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("object error must parse, got: %v", err)
	}
	if msg.Error == nil || msg.Error.Type != "overloaded_error" || msg.Error.Message != "Overloaded" {
		t.Fatalf("Error = %#v", msg.Error)
	}
}

// Model fallback introduces a "fallback" content block. Unknown block types
// must not abort the message — the surrounding text is still wanted.
func TestUnknownContentBlockIsPreserved(t *testing.T) {
	raw := `{"type":"assistant","message":{"model":"claude-opus-5","role":"assistant","content":[{"type":"fallback","from":{"model":"claude-fable-5-1"},"to":{"model":"claude-opus-5"}},{"type":"text","text":"after"}]}}`

	var msg AssistantMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unknown content block must not abort the message, got: %v", err)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(msg.Content))
	}
	unknown, ok := msg.Content[0].(*UnknownBlock)
	if !ok {
		t.Fatalf("first block = %T, want *UnknownBlock", msg.Content[0])
	}
	if unknown.GetType() != "fallback" {
		t.Errorf("unknown.GetType() = %q, want fallback", unknown.GetType())
	}
	if len(unknown.Raw) == 0 {
		t.Error("expected the raw payload to be preserved")
	}
	text, ok := msg.Content[1].(*TextBlock)
	if !ok || text.Text != "after" {
		t.Fatalf("second block = %#v, want the text block", msg.Content[1])
	}
}
