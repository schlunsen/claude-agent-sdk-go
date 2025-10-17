package types

import (
	"encoding/json"
	"testing"
)

// TestTextBlockMarshaling tests JSON marshaling/unmarshaling of TextBlock.
func TestTextBlockMarshaling(t *testing.T) {
	original := &TextBlock{
		Type: "text",
		Text: "Hello, world!",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal TextBlock: %v", err)
	}

	// Unmarshal back
	var decoded TextBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TextBlock: %v", err)
	}

	if decoded.Type != original.Type || decoded.Text != original.Text {
		t.Errorf("unmarshaled TextBlock doesn't match original")
	}
}

// TestToolUseBlockMarshaling tests JSON marshaling/unmarshaling of ToolUseBlock.
func TestToolUseBlockMarshaling(t *testing.T) {
	original := &ToolUseBlock{
		Type: "tool_use",
		ID:   "test-123",
		Name: "Bash",
		Input: map[string]interface{}{
			"command": "ls -la",
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal ToolUseBlock: %v", err)
	}

	// Unmarshal back
	var decoded ToolUseBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ToolUseBlock: %v", err)
	}

	if decoded.Type != original.Type || decoded.ID != original.ID || decoded.Name != original.Name {
		t.Errorf("unmarshaled ToolUseBlock doesn't match original")
	}
}

// TestUnmarshalContentBlock tests unmarshaling of different content block types.
func TestUnmarshalContentBlock(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantType string
	}{
		{
			name:     "text block",
			json:     `{"type":"text","text":"Hello"}`,
			wantType: "text",
		},
		{
			name:     "tool_use block",
			json:     `{"type":"tool_use","id":"123","name":"Bash","input":{}}`,
			wantType: "tool_use",
		},
		{
			name:     "tool_result block",
			json:     `{"type":"tool_result","tool_use_id":"123"}`,
			wantType: "tool_result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := UnmarshalContentBlock([]byte(tt.json))
			if err != nil {
				t.Fatalf("UnmarshalContentBlock failed: %v", err)
			}
			if block.GetType() != tt.wantType {
				t.Errorf("expected type %s, got %s", tt.wantType, block.GetType())
			}
		})
	}
}

// TestUserMessageMarshaling tests JSON marshaling/unmarshaling of UserMessage.
func TestUserMessageMarshaling(t *testing.T) {
	t.Run("string content", func(t *testing.T) {
		original := &UserMessage{
			Type:    "user",
			Content: "Hello, Claude!",
		}

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("failed to marshal UserMessage: %v", err)
		}

		var decoded UserMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("failed to unmarshal UserMessage: %v", err)
		}

		if str, ok := decoded.Content.(string); !ok || str != "Hello, Claude!" {
			t.Errorf("content doesn't match: got %v", decoded.Content)
		}
	})
}

// TestResultMessageMarshaling tests JSON marshaling/unmarshaling of ResultMessage.
func TestResultMessageMarshaling(t *testing.T) {
	costUSD := 0.05
	result := "Success"
	original := &ResultMessage{
		Type:          "result",
		Subtype:       "query_complete",
		DurationMs:    5000,
		DurationAPIMs: 4500,
		IsError:       false,
		NumTurns:      3,
		SessionID:     "session-123",
		TotalCostUSD:  &costUSD,
		Result:        &result,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal ResultMessage: %v", err)
	}

	var decoded ResultMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ResultMessage: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("session ID doesn't match")
	}
	if decoded.TotalCostUSD == nil || *decoded.TotalCostUSD != *original.TotalCostUSD {
		t.Errorf("total cost doesn't match")
	}
}
