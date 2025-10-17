package types

import (
	"encoding/json"
	"fmt"
)

// ContentBlock is an interface for all content block types.
// Content blocks can be text, thinking, tool use, or tool result blocks.
type ContentBlock interface {
	GetType() string
	isContentBlock()
}

// TextBlock represents a text content block from Claude.
type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetType returns the type of the content block.
func (t *TextBlock) GetType() string {
	return t.Type
}

func (t *TextBlock) isContentBlock() {}

// ThinkingBlock represents a thinking content block from Claude.
// This contains Claude's internal reasoning and signature.
type ThinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// GetType returns the type of the content block.
func (t *ThinkingBlock) GetType() string {
	return t.Type
}

func (t *ThinkingBlock) isContentBlock() {}

// ToolUseBlock represents a tool use request from Claude.
type ToolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// GetType returns the type of the content block.
func (t *ToolUseBlock) GetType() string {
	return t.Type
}

func (t *ToolUseBlock) isContentBlock() {}

// ToolResultBlock represents the result of a tool execution.
type ToolResultBlock struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content,omitempty"`  // Can be string or []map[string]interface{}
	IsError   *bool       `json:"is_error,omitempty"` // Pointer to distinguish between false and not set
}

// GetType returns the type of the content block.
func (t *ToolResultBlock) GetType() string {
	return t.Type
}

func (t *ToolResultBlock) isContentBlock() {}

// UnmarshalContentBlock unmarshals a JSON content block into the appropriate type.
func UnmarshalContentBlock(data []byte) (ContentBlock, error) {
	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeCheck); err != nil {
		return nil, NewJSONDecodeErrorWithCause("failed to determine content block type", string(data), err)
	}

	switch typeCheck.Type {
	case "text":
		var block TextBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal text block", string(data), err)
		}
		return &block, nil
	case "thinking":
		var block ThinkingBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal thinking block", string(data), err)
		}
		return &block, nil
	case "tool_use":
		var block ToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal tool_use block", string(data), err)
		}
		return &block, nil
	case "tool_result":
		var block ToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal tool_result block", string(data), err)
		}
		return &block, nil
	default:
		return nil, NewMessageParseErrorWithType("unknown content block type", typeCheck.Type)
	}
}

// Message is an interface for all message types from Claude.
type Message interface {
	GetMessageType() string
	isMessage()
}

// UserMessage represents a message from the user.
type UserMessage struct {
	Type            string      `json:"type"`
	Content         interface{} `json:"content"` // Can be string or []ContentBlock
	ParentToolUseID *string     `json:"parent_tool_use_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *UserMessage) GetMessageType() string {
	return m.Type
}

func (m *UserMessage) isMessage() {}

// UnmarshalJSON implements custom unmarshaling for UserMessage to handle content union type.
func (m *UserMessage) UnmarshalJSON(data []byte) error {
	type Alias UserMessage
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Try to unmarshal as string first
	var contentStr string
	if err := json.Unmarshal(aux.Content, &contentStr); err == nil {
		m.Content = contentStr
		return nil
	}

	// Try to unmarshal as array of content blocks
	var contentArr []json.RawMessage
	if err := json.Unmarshal(aux.Content, &contentArr); err == nil {
		blocks := make([]ContentBlock, len(contentArr))
		for i, rawBlock := range contentArr {
			block, err := UnmarshalContentBlock(rawBlock)
			if err != nil {
				return err
			}
			blocks[i] = block
		}
		m.Content = blocks
		return nil
	}

	return fmt.Errorf("content must be string or array of content blocks")
}

// AssistantMessage represents a message from Claude assistant.
type AssistantMessage struct {
	Type            string         `json:"type"`
	Content         []ContentBlock `json:"content"`
	Model           string         `json:"model"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *AssistantMessage) GetMessageType() string {
	return m.Type
}

func (m *AssistantMessage) isMessage() {}

// UnmarshalJSON implements custom unmarshaling for AssistantMessage to handle content blocks.
func (m *AssistantMessage) UnmarshalJSON(data []byte) error {
	type Alias AssistantMessage
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Unmarshal content blocks
	m.Content = make([]ContentBlock, len(aux.Content))
	for i, rawBlock := range aux.Content {
		block, err := UnmarshalContentBlock(rawBlock)
		if err != nil {
			return err
		}
		m.Content[i] = block
	}

	return nil
}

// MarshalJSON implements custom marshaling for AssistantMessage to handle content blocks.
func (m *AssistantMessage) MarshalJSON() ([]byte, error) {
	type Alias AssistantMessage
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	})
}

// SystemMessage represents a system message with metadata.
type SystemMessage struct {
	Type    string                 `json:"type"`
	Subtype string                 `json:"subtype"`
	Data    map[string]interface{} `json:"data"`
}

// GetMessageType returns the type of the message.
func (m *SystemMessage) GetMessageType() string {
	return m.Type
}

func (m *SystemMessage) isMessage() {}

// ResultMessage represents a result message with cost and usage information.
type ResultMessage struct {
	Type          string                 `json:"type"`
	Subtype       string                 `json:"subtype"`
	DurationMs    int                    `json:"duration_ms"`
	DurationAPIMs int                    `json:"duration_api_ms"`
	IsError       bool                   `json:"is_error"`
	NumTurns      int                    `json:"num_turns"`
	SessionID     string                 `json:"session_id"`
	TotalCostUSD  *float64               `json:"total_cost_usd,omitempty"`
	Usage         map[string]interface{} `json:"usage,omitempty"`
	Result        *string                `json:"result,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *ResultMessage) GetMessageType() string {
	return m.Type
}

func (m *ResultMessage) isMessage() {}

// StreamEvent represents a stream event for partial message updates during streaming.
type StreamEvent struct {
	Type            string                 `json:"type"`
	UUID            string                 `json:"uuid"`
	SessionID       string                 `json:"session_id"`
	Event           map[string]interface{} `json:"event"` // The raw Anthropic API stream event
	ParentToolUseID *string                `json:"parent_tool_use_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *StreamEvent) GetMessageType() string {
	return m.Type
}

func (m *StreamEvent) isMessage() {}

// UnmarshalMessage unmarshals a JSON message into the appropriate message type.
func UnmarshalMessage(data []byte) (Message, error) {
	var typeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeCheck); err != nil {
		return nil, NewJSONDecodeErrorWithCause("failed to determine message type", string(data), err)
	}

	switch typeCheck.Type {
	case "user":
		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal user message", string(data), err)
		}
		return &msg, nil
	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal assistant message", string(data), err)
		}
		return &msg, nil
	case "system":
		var msg SystemMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal system message", string(data), err)
		}
		return &msg, nil
	case "result":
		var msg ResultMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal result message", string(data), err)
		}
		return &msg, nil
	case "stream_event":
		var msg StreamEvent
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal stream event", string(data), err)
		}
		return &msg, nil
	default:
		return nil, NewMessageParseErrorWithType("unknown message type", typeCheck.Type)
	}
}
