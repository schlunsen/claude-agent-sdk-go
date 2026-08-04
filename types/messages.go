package types

import (
	"encoding/json"
	"fmt"
)

// SystemMessageSubtype constants for common system message subtypes
const (
	SystemSubtypeInit        = "init"
	SystemSubtypeWarning     = "warning"
	SystemSubtypeError       = "error"
	SystemSubtypeInfo        = "info"
	SystemSubtypeDebug       = "debug"
	SystemSubtypeSessionEnd  = "session_end"
	SystemSubtypeSessionInfo = "session_info"
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

// ServerToolName constants for known server-side tool names.
const (
	ServerToolNameAdvisor       = "advisor"
	ServerToolNameWebSearch     = "web_search"
	ServerToolNameWebFetch      = "web_fetch"
	ServerToolNameCodeExecution = "code_execution"
)

// ServerToolUseBlock represents a server-side tool use request.
// These are tools that execute on the server (e.g., web_search, web_fetch, advisor).
type ServerToolUseBlock struct {
	Type  string                 `json:"type"` // "server_tool_use"
	ID    string                 `json:"id"`
	Name  string                 `json:"name"` // e.g., "web_search", "web_fetch", "advisor"
	Input map[string]interface{} `json:"input"`
}

// GetType returns the type of the content block.
func (t *ServerToolUseBlock) GetType() string {
	return t.Type
}

func (t *ServerToolUseBlock) isContentBlock() {}

// ServerToolResultBlock represents the result of a server-side tool execution.
type ServerToolResultBlock struct {
	Type      string      `json:"type"` // "server_tool_result"
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content,omitempty"` // Can be string or structured content
}

// GetType returns the type of the content block.
func (t *ServerToolResultBlock) GetType() string {
	return t.Type
}

func (t *ServerToolResultBlock) isContentBlock() {}

// DeferredToolUse represents a tool use that has been deferred by a PreToolUse hook.
// When a PreToolUse hook returns "defer", the tool invocation is paused and a
// DeferredToolUse is emitted so the caller can decide how to proceed.
type DeferredToolUse struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

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
	case "server_tool_use":
		var block ServerToolUseBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal server_tool_use block", string(data), err)
		}
		return &block, nil
	case "server_tool_result":
		var block ServerToolResultBlock
		if err := json.Unmarshal(data, &block); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal server_tool_result block", string(data), err)
		}
		return &block, nil
	default:
		// Forward compatibility: return unknown content blocks as raw maps
		// rather than erroring, to handle new block types from future CLI versions
		return nil, NewMessageParseErrorWithType("unknown content block type", typeCheck.Type)
	}
}

// Message is an interface for all message types from Claude.
type Message interface {
	GetMessageType() string
	ShouldDisplayToUser() bool
	isMessage()
}

// UserMessage represents a message from the user.
type UserMessage struct {
	Type            string                 `json:"type"`
	Content         interface{}            `json:"content"` // Can be string or []ContentBlock
	ParentToolUseID *string                `json:"parent_tool_use_id,omitempty"`
	UUID            *string                `json:"uuid,omitempty"`
	ToolUseResult   map[string]interface{} `json:"tool_use_result,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *UserMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns true for user messages (always display).
func (m *UserMessage) ShouldDisplayToUser() bool {
	return true
}

func (m *UserMessage) isMessage() {}

// UnmarshalJSON implements custom unmarshaling for UserMessage to handle content union type.
func (m *UserMessage) UnmarshalJSON(data []byte) error {
	type Alias UserMessage
	aux := &struct {
		Content json.RawMessage            `json:"content"`
		Message map[string]json.RawMessage `json:"message"` // Handle nested message format from CLI
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var contentRaw json.RawMessage

	// Check if content is in nested message.content (Claude CLI format)
	if aux.Message != nil {
		if content, ok := aux.Message["content"]; ok {
			contentRaw = content
		}
		// Also extract parent_tool_use_id from nested message if present
		if parentToolUseID, ok := aux.Message["parent_tool_use_id"]; ok {
			var id string
			if err := json.Unmarshal(parentToolUseID, &id); err == nil {
				m.ParentToolUseID = &id
			}
		}
	}

	// Fall back to top-level content if nested not found
	if contentRaw == nil && aux.Content != nil {
		contentRaw = aux.Content
	}

	// If we still don't have content, that's an error
	if contentRaw == nil {
		return fmt.Errorf("missing content field")
	}

	// Try to unmarshal as string first
	var contentStr string
	if err := json.Unmarshal(contentRaw, &contentStr); err == nil {
		m.Content = contentStr
		return nil
	}

	// Try to unmarshal as array of content blocks
	var contentArr []json.RawMessage
	if err := json.Unmarshal(contentRaw, &contentArr); err == nil {
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

// AssistantMessageError represents an error within an assistant message.
type AssistantMessageError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// AssistantMessage represents a message from Claude assistant.
type AssistantMessage struct {
	Type            string                 `json:"type"`
	Content         []ContentBlock         `json:"content"`
	Model           string                 `json:"model"`
	ParentToolUseID *string                `json:"parent_tool_use_id,omitempty"`
	Error           *AssistantMessageError `json:"error,omitempty"`
	Usage           map[string]interface{} `json:"usage,omitempty"`
	MessageID       *string                `json:"message_id,omitempty"`
	StopReason      *string                `json:"stop_reason,omitempty"`
	SessionID       *string                `json:"session_id,omitempty"`
	UUID            *string                `json:"uuid,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *AssistantMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns true for assistant messages (always display).
func (m *AssistantMessage) ShouldDisplayToUser() bool {
	return true
}

func (m *AssistantMessage) isMessage() {}

// UnmarshalJSON implements custom unmarshaling for AssistantMessage to handle content blocks.
func (m *AssistantMessage) UnmarshalJSON(data []byte) error {
	type Alias AssistantMessage
	aux := &struct {
		Content []json.RawMessage          `json:"content"`
		Message map[string]json.RawMessage `json:"message"` // Handle nested message format from CLI
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var contentBlocks []json.RawMessage

	// Check if content is in nested message.content (Claude CLI format)
	if aux.Message != nil {
		if contentRaw, ok := aux.Message["content"]; ok {
			var nested []json.RawMessage
			if err := json.Unmarshal(contentRaw, &nested); err == nil {
				contentBlocks = nested
			}
		}
		// Also extract model from nested message if present
		if modelRaw, ok := aux.Message["model"]; ok {
			var model string
			if err := json.Unmarshal(modelRaw, &model); err == nil {
				m.Model = model
			}
		}
	}

	// Fall back to top-level content if nested not found
	if contentBlocks == nil && aux.Content != nil {
		contentBlocks = aux.Content
	}

	// Unmarshal content blocks
	m.Content = make([]ContentBlock, len(contentBlocks))
	for i, rawBlock := range contentBlocks {
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
	Type      string                 `json:"type"`
	Subtype   string                 `json:"subtype,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Response  map[string]interface{} `json:"response,omitempty"`   // For control_response messages
	Request   map[string]interface{} `json:"request,omitempty"`    // For control_request messages
	RequestID string                 `json:"request_id,omitempty"` // For control_request/control_response messages (top-level field)
}

// GetMessageType returns the type of the message.
func (m *SystemMessage) GetMessageType() string {
	return m.Type
}

func (m *SystemMessage) isMessage() {}

// IsInit returns true if this is a system init message.
func (m *SystemMessage) IsInit() bool {
	return m.Subtype == SystemSubtypeInit
}

// IsWarning returns true if this is a system warning message.
func (m *SystemMessage) IsWarning() bool {
	return m.Subtype == SystemSubtypeWarning
}

// IsError returns true if this is a system error message.
func (m *SystemMessage) IsError() bool {
	return m.Subtype == SystemSubtypeError
}

// IsInfo returns true if this is a system info message.
func (m *SystemMessage) IsInfo() bool {
	return m.Subtype == SystemSubtypeInfo
}

// IsDebug returns true if this is a system debug message.
func (m *SystemMessage) IsDebug() bool {
	return m.Subtype == SystemSubtypeDebug
}

// ShouldDisplayToUser returns true if this system message should be shown to the user.
// By default, init and debug messages are not shown to users.
func (m *SystemMessage) ShouldDisplayToUser() bool {
	return m.Subtype != SystemSubtypeInit && m.Subtype != SystemSubtypeDebug
}

// TaskNotificationStatus constants for task notification statuses.
const (
	TaskNotificationStatusCompleted = "completed"
	TaskNotificationStatusFailed    = "failed"
	TaskNotificationStatusStopped   = "stopped"
)

// TaskUpdatedStatus constants for statuses reported inside a task_updated patch.
// "pending"/"running"/"paused" are non-terminal; "completed"/"failed"/"killed"
// are terminal. Note task_updated reports the raw "killed"; the CLI maps that
// to "stopped" only when it emits a task_notification.
const (
	TaskUpdatedStatusPending   = "pending"
	TaskUpdatedStatusRunning   = "running"
	TaskUpdatedStatusPaused    = "paused"
	TaskUpdatedStatusCompleted = "completed"
	TaskUpdatedStatusFailed    = "failed"
	TaskUpdatedStatusKilled    = "killed"
)

// IsTerminalTaskStatus reports whether a task status means the task has
// finished and should be cleared from any "active task" tracking. It spans
// both lifecycle vocabularies: TaskNotificationMessage reports "stopped" (the
// CLI's mapped form of a killed task) while TaskUpdatedMessage reports the raw
// "killed". Consumers should treat the status of a TaskNotificationMessage and
// a TaskUpdatedMessage the same way.
func IsTerminalTaskStatus(status string) bool {
	switch status {
	case TaskNotificationStatusCompleted, TaskNotificationStatusFailed,
		TaskNotificationStatusStopped, TaskUpdatedStatusKilled:
		return true
	default:
		return false
	}
}

// TaskUsage represents usage statistics for a task.
type TaskUsage struct {
	TotalTokens int `json:"total_tokens"`
	ToolUses    int `json:"tool_uses"`
	DurationMs  int `json:"duration_ms"`
}

// TaskBudget represents the API-side task budget in tokens.
type TaskBudget struct {
	MaxTokens  int     `json:"max_tokens"`
	MaxCostUSD float64 `json:"max_cost_usd,omitempty"`
}

// TaskStartedMessage is emitted when a task starts execution.
type TaskStartedMessage struct {
	Type      string `json:"type"` // "task_started"
	TaskID    string `json:"task_id"`
	SessionID string `json:"session_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *TaskStartedMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for task started messages (internal).
func (m *TaskStartedMessage) ShouldDisplayToUser() bool {
	return false
}

func (m *TaskStartedMessage) isMessage() {}

// TaskProgressMessage is emitted while a task is in progress.
type TaskProgressMessage struct {
	Type      string                 `json:"type"` // "task_progress"
	TaskID    string                 `json:"task_id"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *TaskProgressMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for task progress messages (internal).
func (m *TaskProgressMessage) ShouldDisplayToUser() bool {
	return false
}

func (m *TaskProgressMessage) isMessage() {}

// TaskNotificationMessage is emitted when a task completes, fails, or is stopped.
type TaskNotificationMessage struct {
	Type      string                 `json:"type"` // "task_notification"
	TaskID    string                 `json:"task_id"`
	Status    string                 `json:"status"` // "completed", "failed", "stopped"
	SessionID string                 `json:"session_id,omitempty"`
	Usage     *TaskUsage             `json:"usage,omitempty"`
	Error     *string                `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *TaskNotificationMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns true for task notification messages.
func (m *TaskNotificationMessage) ShouldDisplayToUser() bool {
	return true
}

func (m *TaskNotificationMessage) isMessage() {}

// TaskUpdatedMessage is emitted when a background task's state changes.
//
// The CLI emits system/task_updated events as a task moves through its
// lifecycle. Patch carries the changed fields (e.g. "status", "end_time");
// when the patched status is terminal (see IsTerminalTaskStatus) the task has
// finished.
//
// Lifecycle note: a background task's terminal state can arrive *only* as a
// TaskUpdatedMessage with no accompanying TaskNotificationMessage — for
// example a task stopped via StopTask reports status "killed" here, and the
// matching notification is sometimes suppressed. Consumers that track active
// task IDs should therefore clear them on a terminal status (see
// IsTerminalTaskStatus) from *either* a TaskNotificationMessage or a
// TaskUpdatedMessage.
type TaskUpdatedMessage struct {
	Type      string                 `json:"type"` // "system"
	Subtype   string                 `json:"subtype"`
	TaskID    string                 `json:"task_id"`
	Patch     map[string]interface{} `json:"patch,omitempty"`
	Status    string                 `json:"-"` // Derived from Patch["status"]; empty when the patch carries no status
	SessionID string                 `json:"session_id,omitempty"`
	UUID      string                 `json:"uuid,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *TaskUpdatedMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for task updated messages (internal).
func (m *TaskUpdatedMessage) ShouldDisplayToUser() bool {
	return false
}

func (m *TaskUpdatedMessage) isMessage() {}

// IsTerminal reports whether this update marks the task as finished.
func (m *TaskUpdatedMessage) IsTerminal() bool {
	return IsTerminalTaskStatus(m.Status)
}

// UnmarshalJSON implements custom unmarshaling for TaskUpdatedMessage,
// deriving Status from the patch. Parsed defensively: the patch may omit
// status/uuid/session_id and parsing must never fail on a lifecycle event.
func (m *TaskUpdatedMessage) UnmarshalJSON(data []byte) error {
	type Alias TaskUpdatedMessage
	aux := (*Alias)(m)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	// Terminal-ness is derived from patch.status; the CLI is assumed to set it
	// on terminal transitions. A patch that carries only end_time/result/error
	// (no status) is left non-terminal (Status == "") — the full patch is
	// still preserved on Patch for callers that need more.
	if m.Patch != nil {
		if status, ok := m.Patch["status"].(string); ok {
			m.Status = status
		}
	}
	return nil
}

// HookEventMessage is emitted when include_hook_events is enabled.
// It provides visibility into hook lifecycle events during execution.
type HookEventMessage struct {
	Type      string                 `json:"type"` // "hook_event"
	Event     string                 `json:"event"`
	HookName  string                 `json:"hook_name,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *HookEventMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for hook event messages (diagnostic).
func (m *HookEventMessage) ShouldDisplayToUser() bool {
	return false
}

func (m *HookEventMessage) isMessage() {}

// MirrorErrorMessage is emitted when a SessionStore.Append() call fails.
// This allows the caller to detect transcript mirroring failures.
type MirrorErrorMessage struct {
	Type    string      `json:"type"` // "mirror_error"
	Key     interface{} `json:"key,omitempty"`
	Error   string      `json:"error"`
	Message string      `json:"message,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *MirrorErrorMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns true for mirror error messages (indicates data loss).
func (m *MirrorErrorMessage) ShouldDisplayToUser() bool {
	return true
}

func (m *MirrorErrorMessage) isMessage() {}

// ResultMessage represents a result message with cost and usage information.
type ResultMessage struct {
	Type              string                 `json:"type"`
	Subtype           string                 `json:"subtype"`
	DurationMs        int                    `json:"duration_ms"`
	DurationAPIMs     int                    `json:"duration_api_ms"`
	IsError           bool                   `json:"is_error"`
	NumTurns          int                    `json:"num_turns"`
	SessionID         string                 `json:"session_id"`
	TotalCostUSD      *float64               `json:"total_cost_usd,omitempty"`
	Usage             map[string]interface{} `json:"usage,omitempty"`
	Result            *string                `json:"result,omitempty"`
	StopReason        *string                `json:"stop_reason,omitempty"`
	StructuredOutput  interface{}            `json:"structured_output,omitempty"`
	ModelUsage        map[string]interface{} `json:"model_usage,omitempty"`
	PermissionDenials []interface{}          `json:"permission_denials,omitempty"`
	UUID              *string                `json:"uuid,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *ResultMessage) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for result messages (metadata only).
func (m *ResultMessage) ShouldDisplayToUser() bool {
	return false
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

// ShouldDisplayToUser returns false for stream events (internal only).
func (m *StreamEvent) ShouldDisplayToUser() bool {
	return false
}

func (m *StreamEvent) isMessage() {}

// RateLimitStatus represents the rate limit state.
type RateLimitStatus string

const (
	RateLimitStatusAllowed        RateLimitStatus = "allowed"
	RateLimitStatusAllowedWarning RateLimitStatus = "allowed_warning"
	RateLimitStatusRejected       RateLimitStatus = "rejected"
)

// RateLimitType represents the type of rate limit.
type RateLimitType string

const (
	RateLimitTypeFiveHour       RateLimitType = "five_hour"
	RateLimitTypeSevenDay       RateLimitType = "seven_day"
	RateLimitTypeSevenDayOpus   RateLimitType = "seven_day_opus"
	RateLimitTypeSevenDaySonnet RateLimitType = "seven_day_sonnet"
	RateLimitTypeOverage        RateLimitType = "overage"
)

// RateLimitInfo contains rate limit state information.
type RateLimitInfo struct {
	Status                RateLimitStatus        `json:"status"`
	ResetsAt              *string                `json:"resets_at,omitempty"`
	RateLimitType         RateLimitType          `json:"rate_limit_type"`
	Utilization           *float64               `json:"utilization,omitempty"`
	OverageStatus         *string                `json:"overage_status,omitempty"`
	OverageResetsAt       *string                `json:"overage_resets_at,omitempty"`
	OverageDisabledReason *string                `json:"overage_disabled_reason,omitempty"`
	Raw                   map[string]interface{} `json:"raw,omitempty"`
}

// RateLimitEvent is emitted when rate limit state changes.
type RateLimitEvent struct {
	Type          string         `json:"type"` // "rate_limit"
	RateLimitInfo *RateLimitInfo `json:"rate_limit_info"`
	UUID          *string        `json:"uuid,omitempty"`
	SessionID     *string        `json:"session_id,omitempty"`
}

// GetMessageType returns the type of the message.
func (m *RateLimitEvent) GetMessageType() string {
	return m.Type
}

// ShouldDisplayToUser returns false for rate limit events (metadata only).
func (m *RateLimitEvent) ShouldDisplayToUser() bool {
	return false
}

func (m *RateLimitEvent) isMessage() {}

// UnmarshalMessage unmarshals a JSON message into the appropriate message type.
func UnmarshalMessage(data []byte) (Message, error) {
	var typeCheck struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
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
	case "system", "control_request", "control_response":
		// Task lifecycle events arrive from the CLI as system messages with
		// dedicated subtypes; route them to their typed messages so consumers
		// can track task state without inspecting raw system data.
		if typeCheck.Type == "system" {
			switch typeCheck.Subtype {
			case "task_started":
				var msg TaskStartedMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task started message", string(data), err)
				}
				return &msg, nil
			case "task_progress":
				var msg TaskProgressMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task progress message", string(data), err)
				}
				return &msg, nil
			case "task_notification":
				var msg TaskNotificationMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task notification message", string(data), err)
				}
				return &msg, nil
			case "task_updated":
				var msg TaskUpdatedMessage
				if err := json.Unmarshal(data, &msg); err != nil {
					return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task updated message", string(data), err)
				}
				return &msg, nil
			}
		}
		// system, control_request, and control_response are all SystemMessage types
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
	case "rate_limit":
		var msg RateLimitEvent
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal rate limit event", string(data), err)
		}
		return &msg, nil
	case "task_started":
		var msg TaskStartedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task started message", string(data), err)
		}
		return &msg, nil
	case "task_progress":
		var msg TaskProgressMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task progress message", string(data), err)
		}
		return &msg, nil
	case "task_notification":
		var msg TaskNotificationMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task notification message", string(data), err)
		}
		return &msg, nil
	case "task_updated":
		var msg TaskUpdatedMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal task updated message", string(data), err)
		}
		return &msg, nil
	case "hook_event":
		var msg HookEventMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal hook event message", string(data), err)
		}
		return &msg, nil
	case "mirror_error":
		var msg MirrorErrorMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal mirror error message", string(data), err)
		}
		return &msg, nil
	default:
		// Forward compatibility: skip unknown message types from future CLI versions
		// rather than returning an error. Return a SystemMessage with the raw type.
		var msg SystemMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, NewJSONDecodeErrorWithCause("failed to unmarshal unknown message type", string(data), err)
		}
		return &msg, nil
	}
}
