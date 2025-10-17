package internal

import (
	"encoding/json"
	"fmt"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// ParseMessage parses a JSON byte slice into a typed Message.
// Returns the appropriate message type based on the "type" field discriminator.
// Handles: user, assistant, system, result, stream_event
func ParseMessage(data []byte) (types.Message, error) {
	if len(data) == 0 {
		return nil, types.NewMessageParseError("cannot parse empty message data")
	}

	// Use the existing UnmarshalMessage from types package
	msg, err := types.UnmarshalMessage(data)
	if err != nil {
		// If it's already a proper error type, return it
		if types.IsMessageParseError(err) || types.IsJSONDecodeError(err) {
			return nil, err
		}
		// Otherwise wrap it
		msgType, _ := extractType(data)
		return nil, types.NewMessageParseErrorWithCause(
			fmt.Sprintf("failed to parse message: %v", err),
			msgType,
			err,
		)
	}

	return msg, nil
}

// ParseContentBlock parses a single content block JSON into ContentBlock interface.
// Handles: text, tool_use, tool_result, thinking (if present)
func ParseContentBlock(data []byte) (types.ContentBlock, error) {
	if len(data) == 0 {
		return nil, types.NewMessageParseError("cannot parse empty content block data")
	}

	// Use the existing UnmarshalContentBlock from types package
	block, err := types.UnmarshalContentBlock(data)
	if err != nil {
		// If it's already a proper error type, return it
		if types.IsMessageParseError(err) || types.IsJSONDecodeError(err) {
			return nil, err
		}
		// Otherwise wrap it
		blockType, _ := extractType(data)
		return nil, types.NewMessageParseErrorWithCause(
			fmt.Sprintf("failed to parse content block: %v", err),
			blockType,
			err,
		)
	}

	return block, nil
}

// ParseContentBlocks parses multiple content blocks.
// Each block must have a "type" field discriminator.
func ParseContentBlocks(blocks []json.RawMessage) ([]types.ContentBlock, error) {
	if len(blocks) == 0 {
		return []types.ContentBlock{}, nil
	}

	result := make([]types.ContentBlock, 0, len(blocks))
	for i, rawBlock := range blocks {
		block, err := ParseContentBlock(rawBlock)
		if err != nil {
			// Wrap with index for better error context
			return nil, fmt.Errorf("failed to parse content block at index %d: %w", i, err)
		}
		result = append(result, block)
	}

	return result, nil
}

// parseUserMessage parses a user message from JSON bytes.
// nolint:unused
func parseUserMessage(data []byte) (*types.UserMessage, error) {
	var msg types.UserMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal user message",
			truncateString(string(data), 200),
			err,
		)
	}
	return &msg, nil
}

// parseAssistantMessage parses an assistant message from JSON bytes.
// nolint:unused
func parseAssistantMessage(data []byte) (*types.AssistantMessage, error) {
	var msg types.AssistantMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal assistant message",
			truncateString(string(data), 200),
			err,
		)
	}
	return &msg, nil
}

// parseSystemMessage parses a system message from JSON bytes.
// nolint:unused
func parseSystemMessage(data []byte) (*types.SystemMessage, error) {
	var msg types.SystemMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal system message",
			truncateString(string(data), 200),
			err,
		)
	}
	return &msg, nil
}

// parseResultMessage parses a result message from JSON bytes.
// nolint:unused
func parseResultMessage(data []byte) (*types.ResultMessage, error) {
	var msg types.ResultMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal result message",
			truncateString(string(data), 200),
			err,
		)
	}
	return &msg, nil
}

// parseStreamEvent parses a stream event from JSON bytes.
// nolint:unused
func parseStreamEvent(data []byte) (*types.StreamEvent, error) {
	var msg types.StreamEvent
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal stream event",
			truncateString(string(data), 200),
			err,
		)
	}
	return &msg, nil
}

// parseTextBlock parses a text block from JSON bytes.
// nolint:unused
func parseTextBlock(data []byte) (*types.TextBlock, error) {
	var block types.TextBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal text block",
			truncateString(string(data), 200),
			err,
		)
	}
	return &block, nil
}

// parseToolUseBlock parses a tool use block from JSON bytes.
// nolint:unused
func parseToolUseBlock(data []byte) (*types.ToolUseBlock, error) {
	var block types.ToolUseBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal tool_use block",
			truncateString(string(data), 200),
			err,
		)
	}
	return &block, nil
}

// parseToolResultBlock parses a tool result block from JSON bytes.
// nolint:unused
func parseToolResultBlock(data []byte) (*types.ToolResultBlock, error) {
	var block types.ToolResultBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal tool_result block",
			truncateString(string(data), 200),
			err,
		)
	}
	return &block, nil
}

// parseThinkingBlock parses a thinking block from JSON bytes.
// nolint:unused
func parseThinkingBlock(data []byte) (*types.ThinkingBlock, error) {
	var block types.ThinkingBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return nil, types.NewJSONDecodeErrorWithCause(
			"failed to unmarshal thinking block",
			truncateString(string(data), 200),
			err,
		)
	}
	return &block, nil
}

// extractType extracts the "type" field value from raw JSON.
// Returns empty string if type field is missing or invalid.
func extractType(data []byte) (string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON for type extraction: %w", err)
	}

	typeVal, ok := m["type"]
	if !ok {
		return "", fmt.Errorf("missing type field")
	}

	typeStr, ok := typeVal.(string)
	if !ok {
		return "", fmt.Errorf("type field is not a string")
	}

	return typeStr, nil
}

// truncateString truncates a string to maxLen characters for logging.
// Adds "..." suffix if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
