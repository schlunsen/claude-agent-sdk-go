package internal

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// TestParseMessage_UserMessage tests parsing of user messages.
func TestParseMessage_UserMessage(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		wantType    string
		checkResult func(t *testing.T, msg types.Message)
	}{
		{
			name:     "simple string content",
			input:    userMessageSimple,
			wantErr:  false,
			wantType: "user",
			checkResult: func(t *testing.T, msg types.Message) {
				userMsg, ok := msg.(*types.UserMessage)
				if !ok {
					t.Errorf("expected *types.UserMessage, got %T", msg)
					return
				}
				if userMsg.Type != "user" {
					t.Errorf("expected type 'user', got '%s'", userMsg.Type)
				}
				contentStr, ok := userMsg.Content.(string)
				if !ok {
					t.Errorf("expected content to be string, got %T", userMsg.Content)
					return
				}
				if contentStr != "Hello Claude" {
					t.Errorf("expected content 'Hello Claude', got '%s'", contentStr)
				}
			},
		},
		{
			name:     "content blocks with tool result",
			input:    userMessageComplex,
			wantErr:  false,
			wantType: "user",
			checkResult: func(t *testing.T, msg types.Message) {
				userMsg, ok := msg.(*types.UserMessage)
				if !ok {
					t.Errorf("expected *types.UserMessage, got %T", msg)
					return
				}
				blocks, ok := userMsg.Content.([]types.ContentBlock)
				if !ok {
					t.Errorf("expected content to be []ContentBlock, got %T", userMsg.Content)
					return
				}
				if len(blocks) != 2 {
					t.Errorf("expected 2 content blocks, got %d", len(blocks))
				}
				if userMsg.ParentToolUseID == nil || *userMsg.ParentToolUseID != "parent_456" {
					t.Errorf("expected parent_tool_use_id 'parent_456', got %v", userMsg.ParentToolUseID)
				}
			},
		},
		{
			name:     "only text content",
			input:    userMessageOnlyText,
			wantErr:  false,
			wantType: "user",
			checkResult: func(t *testing.T, msg types.Message) {
				userMsg, ok := msg.(*types.UserMessage)
				if !ok {
					t.Errorf("expected *types.UserMessage, got %T", msg)
					return
				}
				contentStr, ok := userMsg.Content.(string)
				if !ok {
					t.Errorf("expected content to be string, got %T", userMsg.Content)
					return
				}
				if contentStr != "Simple text content" {
					t.Errorf("expected content 'Simple text content', got '%s'", contentStr)
				}
			},
		},
		{
			name:     "content blocks array",
			input:    userMessageContentBlocks,
			wantErr:  false,
			wantType: "user",
			checkResult: func(t *testing.T, msg types.Message) {
				userMsg, ok := msg.(*types.UserMessage)
				if !ok {
					t.Errorf("expected *types.UserMessage, got %T", msg)
					return
				}
				blocks, ok := userMsg.Content.([]types.ContentBlock)
				if !ok {
					t.Errorf("expected content to be []ContentBlock, got %T", userMsg.Content)
					return
				}
				if len(blocks) != 2 {
					t.Errorf("expected 2 content blocks, got %d", len(blocks))
				}
			},
		},
		{
			name:     "extra fields ignored (forward compat)",
			input:    userMessageExtraFields,
			wantErr:  false,
			wantType: "user",
			checkResult: func(t *testing.T, msg types.Message) {
				userMsg, ok := msg.(*types.UserMessage)
				if !ok {
					t.Errorf("expected *types.UserMessage, got %T", msg)
					return
				}
				if userMsg.Type != "user" {
					t.Errorf("expected type 'user', got '%s'", userMsg.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if msg.GetMessageType() != tt.wantType {
					t.Errorf("expected message type %s, got %s", tt.wantType, msg.GetMessageType())
				}
				if tt.checkResult != nil {
					tt.checkResult(t, msg)
				}
			}
		})
	}
}

// TestParseMessage_AssistantMessage tests parsing of assistant messages.
func TestParseMessage_AssistantMessage(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		wantType    string
		checkResult func(t *testing.T, msg types.Message)
	}{
		{
			name:     "simple text content",
			input:    assistantMessageText,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if len(assistantMsg.Content) != 1 {
					t.Errorf("expected 1 content block, got %d", len(assistantMsg.Content))
					return
				}
				textBlock, ok := assistantMsg.Content[0].(*types.TextBlock)
				if !ok {
					t.Errorf("expected *types.TextBlock, got %T", assistantMsg.Content[0])
					return
				}
				if textBlock.Text != "Hi there! How can I help you today?" {
					t.Errorf("unexpected text content: %s", textBlock.Text)
				}
				if assistantMsg.Model != "claude-sonnet-4-5-20250929" {
					t.Errorf("expected model 'claude-sonnet-4-5-20250929', got '%s'", assistantMsg.Model)
				}
			},
		},
		{
			name:     "tool use content",
			input:    assistantMessageToolUse,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if len(assistantMsg.Content) != 2 {
					t.Errorf("expected 2 content blocks, got %d", len(assistantMsg.Content))
					return
				}
				toolUseBlock, ok := assistantMsg.Content[1].(*types.ToolUseBlock)
				if !ok {
					t.Errorf("expected *types.ToolUseBlock, got %T", assistantMsg.Content[1])
					return
				}
				if toolUseBlock.Name != "calculator" {
					t.Errorf("expected tool name 'calculator', got '%s'", toolUseBlock.Name)
				}
			},
		},
		{
			name:     "thinking block content",
			input:    assistantMessageThinking,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if len(assistantMsg.Content) < 1 {
					t.Errorf("expected at least 1 content block, got %d", len(assistantMsg.Content))
					return
				}
				thinkingBlock, ok := assistantMsg.Content[0].(*types.ThinkingBlock)
				if !ok {
					t.Errorf("expected *types.ThinkingBlock, got %T", assistantMsg.Content[0])
					return
				}
				if !strings.Contains(thinkingBlock.Thinking, "step by step") {
					t.Errorf("unexpected thinking content: %s", thinkingBlock.Thinking)
				}
			},
		},
		{
			name:     "mixed content blocks",
			input:    assistantMessageMixed,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if len(assistantMsg.Content) != 3 {
					t.Errorf("expected 3 content blocks, got %d", len(assistantMsg.Content))
				}
			},
		},
		{
			name:     "all block types",
			input:    assistantMessageAllBlocks,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if len(assistantMsg.Content) != 3 {
					t.Errorf("expected 3 content blocks, got %d", len(assistantMsg.Content))
				}
			},
		},
		{
			name:     "extra fields ignored (forward compat)",
			input:    assistantMessageExtraFields,
			wantErr:  false,
			wantType: "assistant",
			checkResult: func(t *testing.T, msg types.Message) {
				assistantMsg, ok := msg.(*types.AssistantMessage)
				if !ok {
					t.Errorf("expected *types.AssistantMessage, got %T", msg)
					return
				}
				if assistantMsg.Type != "assistant" {
					t.Errorf("expected type 'assistant', got '%s'", assistantMsg.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if msg.GetMessageType() != tt.wantType {
					t.Errorf("expected message type %s, got %s", tt.wantType, msg.GetMessageType())
				}
				if tt.checkResult != nil {
					tt.checkResult(t, msg)
				}
			}
		})
	}
}

// TestParseMessage_SystemMessage tests parsing of system messages.
func TestParseMessage_SystemMessage(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		wantSubtype string
	}{
		{
			name:        "metadata system message",
			input:       systemMessageMetadata,
			wantErr:     false,
			wantSubtype: "metadata",
		},
		{
			name:        "warning system message",
			input:       systemMessageWarning,
			wantErr:     false,
			wantSubtype: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				systemMsg, ok := msg.(*types.SystemMessage)
				if !ok {
					t.Errorf("expected *types.SystemMessage, got %T", msg)
					return
				}
				if systemMsg.Subtype != tt.wantSubtype {
					t.Errorf("expected subtype %s, got %s", tt.wantSubtype, systemMsg.Subtype)
				}
			}
		})
	}
}

// TestParseMessage_ResultMessage tests parsing of result messages.
func TestParseMessage_ResultMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantErr  bool
		isError  bool
		hasUsage bool
	}{
		{
			name:     "success result",
			input:    resultMessageSuccess,
			wantErr:  false,
			isError:  false,
			hasUsage: true,
		},
		{
			name:     "error result",
			input:    resultMessageError,
			wantErr:  false,
			isError:  true,
			hasUsage: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				resultMsg, ok := msg.(*types.ResultMessage)
				if !ok {
					t.Errorf("expected *types.ResultMessage, got %T", msg)
					return
				}
				if resultMsg.IsError != tt.isError {
					t.Errorf("expected is_error %v, got %v", tt.isError, resultMsg.IsError)
				}
				if tt.hasUsage && resultMsg.Usage == nil {
					t.Errorf("expected usage to be present")
				}
				if !tt.hasUsage && resultMsg.Usage != nil {
					t.Errorf("expected usage to be nil")
				}
			}
		})
	}
}

// TestParseMessage_StreamEvent tests parsing of stream events.
func TestParseMessage_StreamEvent(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantErr   bool
		eventType string
	}{
		{
			name:      "message_start event",
			input:     streamEventMessageStart,
			wantErr:   false,
			eventType: "message_start",
		},
		{
			name:      "content_block_delta event",
			input:     streamEventContentBlockDelta,
			wantErr:   false,
			eventType: "content_block_delta",
		},
		{
			name:      "message_delta event",
			input:     streamEventMessageDelta,
			wantErr:   false,
			eventType: "message_delta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				streamEvent, ok := msg.(*types.StreamEvent)
				if !ok {
					t.Errorf("expected *types.StreamEvent, got %T", msg)
					return
				}
				if streamEvent.Event == nil {
					t.Errorf("expected event to be present")
					return
				}
				evtType, ok := streamEvent.Event["type"].(string)
				if !ok || evtType != tt.eventType {
					t.Errorf("expected event type %s, got %v", tt.eventType, evtType)
				}
			}
		})
	}
}

// TestParseMessage_InvalidJSON tests error handling for invalid JSON.
func TestParseMessage_InvalidJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "malformed JSON",
			input:   invalidJSONMalformed,
			wantErr: true,
		},
		{
			name:    "missing type field",
			input:   invalidJSONMissingType,
			wantErr: true,
		},
		{
			name:    "unknown message type",
			input:   invalidJSONUnknownType,
			wantErr: true,
		},
		{
			name:    "empty bytes",
			input:   invalidJSONEmptyBytes,
			wantErr: true,
		},
		{
			name:    "null type field",
			input:   invalidJSONNullType,
			wantErr: true,
		},
		{
			name:    "number type field",
			input:   invalidJSONNumberType,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				// Verify error is one of the expected types
				if !types.IsMessageParseError(err) && !types.IsJSONDecodeError(err) {
					t.Errorf("expected MessageParseError or JSONDecodeError, got %T", err)
				}
			}
		})
	}
}

// TestParseContentBlock_TextBlock tests parsing text blocks.
func TestParseContentBlock_TextBlock(t *testing.T) {
	block, err := ParseContentBlock(textBlockJSON)
	if err != nil {
		t.Fatalf("ParseContentBlock() error = %v", err)
	}

	textBlock, ok := block.(*types.TextBlock)
	if !ok {
		t.Fatalf("expected *types.TextBlock, got %T", block)
	}

	if textBlock.Type != "text" {
		t.Errorf("expected type 'text', got '%s'", textBlock.Type)
	}
	if textBlock.Text != "This is a text block" {
		t.Errorf("unexpected text: %s", textBlock.Text)
	}
}

// TestParseContentBlock_ToolUseBlock tests parsing tool use blocks.
func TestParseContentBlock_ToolUseBlock(t *testing.T) {
	block, err := ParseContentBlock(toolUseBlockJSON)
	if err != nil {
		t.Fatalf("ParseContentBlock() error = %v", err)
	}

	toolUseBlock, ok := block.(*types.ToolUseBlock)
	if !ok {
		t.Fatalf("expected *types.ToolUseBlock, got %T", block)
	}

	if toolUseBlock.Type != "tool_use" {
		t.Errorf("expected type 'tool_use', got '%s'", toolUseBlock.Type)
	}
	if toolUseBlock.Name != "calculator" {
		t.Errorf("expected name 'calculator', got '%s'", toolUseBlock.Name)
	}
	if toolUseBlock.ID != "toolu_block_123" {
		t.Errorf("expected id 'toolu_block_123', got '%s'", toolUseBlock.ID)
	}
}

// TestParseContentBlock_ToolResultBlock tests parsing tool result blocks.
func TestParseContentBlock_ToolResultBlock(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		wantErr     bool
		wantIsError *bool
	}{
		{
			name:        "simple result",
			input:       toolResultBlockJSON,
			wantErr:     false,
			wantIsError: nil,
		},
		{
			name:        "error result",
			input:       toolResultBlockJSONWithError,
			wantErr:     false,
			wantIsError: boolPtr(true),
		},
		{
			name:        "complex content",
			input:       toolResultBlockJSONComplex,
			wantErr:     false,
			wantIsError: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := ParseContentBlock(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContentBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				toolResultBlock, ok := block.(*types.ToolResultBlock)
				if !ok {
					t.Errorf("expected *types.ToolResultBlock, got %T", block)
					return
				}
				if toolResultBlock.Type != "tool_result" {
					t.Errorf("expected type 'tool_result', got '%s'", toolResultBlock.Type)
				}
				if tt.wantIsError != nil {
					if toolResultBlock.IsError == nil {
						t.Errorf("expected is_error to be %v, got nil", *tt.wantIsError)
					} else if *toolResultBlock.IsError != *tt.wantIsError {
						t.Errorf("expected is_error %v, got %v", *tt.wantIsError, *toolResultBlock.IsError)
					}
				}
			}
		})
	}
}

// TestParseContentBlock_ThinkingBlock tests parsing thinking blocks.
func TestParseContentBlock_ThinkingBlock(t *testing.T) {
	block, err := ParseContentBlock(thinkingBlockJSON)
	if err != nil {
		t.Fatalf("ParseContentBlock() error = %v", err)
	}

	thinkingBlock, ok := block.(*types.ThinkingBlock)
	if !ok {
		t.Fatalf("expected *types.ThinkingBlock, got %T", block)
	}

	if thinkingBlock.Type != "thinking" {
		t.Errorf("expected type 'thinking', got '%s'", thinkingBlock.Type)
	}
	if !strings.Contains(thinkingBlock.Thinking, "think about this") {
		t.Errorf("unexpected thinking content: %s", thinkingBlock.Thinking)
	}
}

// TestParseContentBlock_InvalidBlocks tests error handling for invalid content blocks.
func TestParseContentBlock_InvalidBlocks(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "empty bytes",
			input:   []byte{},
			wantErr: true,
		},
		{
			name:    "missing type field",
			input:   contentBlockMissingType,
			wantErr: true,
		},
		{
			name:    "unknown type",
			input:   contentBlockUnknownType,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   contentBlockMalformed,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseContentBlock(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseContentBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseContentBlocks_Multiple tests parsing multiple content blocks.
func TestParseContentBlocks_Multiple(t *testing.T) {
	rawBlocks := make([]json.RawMessage, len(multipleContentBlocks))
	for i, block := range multipleContentBlocks {
		rawBlocks[i] = json.RawMessage(block)
	}

	blocks, err := ParseContentBlocks(rawBlocks)
	if err != nil {
		t.Fatalf("ParseContentBlocks() error = %v", err)
	}

	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}

	// Verify types
	if _, ok := blocks[0].(*types.TextBlock); !ok {
		t.Errorf("expected block 0 to be *types.TextBlock, got %T", blocks[0])
	}
	if _, ok := blocks[1].(*types.ThinkingBlock); !ok {
		t.Errorf("expected block 1 to be *types.ThinkingBlock, got %T", blocks[1])
	}
	if _, ok := blocks[2].(*types.ToolUseBlock); !ok {
		t.Errorf("expected block 2 to be *types.ToolUseBlock, got %T", blocks[2])
	}
}

// TestParseContentBlocks_Empty tests parsing empty content blocks array.
func TestParseContentBlocks_Empty(t *testing.T) {
	blocks, err := ParseContentBlocks([]json.RawMessage{})
	if err != nil {
		t.Fatalf("ParseContentBlocks() error = %v", err)
	}

	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

// TestParseContentBlocks_WithError tests error handling in batch parsing.
func TestParseContentBlocks_WithError(t *testing.T) {
	rawBlocks := []json.RawMessage{
		json.RawMessage(`{"type": "text", "text": "Valid"}`),
		json.RawMessage(`{"type": "invalid_type"}`),
		json.RawMessage(`{"type": "text", "text": "Also valid"}`),
	}

	_, err := ParseContentBlocks(rawBlocks)
	if err == nil {
		t.Error("expected error for invalid block, got nil")
	}

	// Error should mention the index
	if !strings.Contains(err.Error(), "index 1") {
		t.Errorf("error should mention index 1, got: %v", err)
	}
}

// TestExtractType tests the type extraction helper.
func TestExtractType(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantType string
		wantErr  bool
	}{
		{
			name:     "valid user type",
			input:    []byte(`{"type": "user", "content": "test"}`),
			wantType: "user",
			wantErr:  false,
		},
		{
			name:     "valid assistant type",
			input:    []byte(`{"type": "assistant", "content": []}`),
			wantType: "assistant",
			wantErr:  false,
		},
		{
			name:     "missing type",
			input:    []byte(`{"content": "test"}`),
			wantType: "",
			wantErr:  true,
		},
		{
			name:     "invalid JSON",
			input:    []byte(`{invalid`),
			wantType: "",
			wantErr:  true,
		},
		{
			name:     "type is number",
			input:    []byte(`{"type": 123}`),
			wantType: "",
			wantErr:  true,
		},
		{
			name:     "type is null",
			input:    []byte(`{"type": null}`),
			wantType: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, err := extractType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotType != tt.wantType {
				t.Errorf("extractType() = %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

// TestTruncateString tests the string truncation helper.
func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "no truncation needed",
			input:  "short",
			maxLen: 10,
			want:   "short",
		},
		{
			name:   "exact length",
			input:  "exact",
			maxLen: 5,
			want:   "exact",
		},
		{
			name:   "truncation needed",
			input:  "this is a very long string",
			maxLen: 10,
			want:   "this is a ...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// BenchmarkParseMessage_User benchmarks parsing a user message.
func BenchmarkParseMessage_User(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ParseMessage(userMessageSimple)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseMessage_Assistant benchmarks parsing an assistant message.
func BenchmarkParseMessage_Assistant(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ParseMessage(assistantMessageText)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseMessage_AssistantComplex benchmarks parsing a complex assistant message.
func BenchmarkParseMessage_AssistantComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ParseMessage(assistantMessageMixed)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseContentBlock benchmarks parsing a single content block.
func BenchmarkParseContentBlock(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ParseContentBlock(textBlockJSON)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseContentBlocks benchmarks parsing multiple content blocks.
func BenchmarkParseContentBlocks(b *testing.B) {
	rawBlocks := make([]json.RawMessage, len(multipleContentBlocks))
	for i, block := range multipleContentBlocks {
		rawBlocks[i] = json.RawMessage(block)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseContentBlocks(rawBlocks)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}
