package types

import "fmt"

// CLINotFoundError indicates that the Claude Code CLI binary could not be found.
// This typically occurs when the CLI is not installed or not in PATH.
type CLINotFoundError struct {
	Message string
	Cause   error
}

// Error returns the error message, implementing the error interface.
func (e *CLINotFoundError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Is checks if the target error is a CLINotFoundError.
func (e *CLINotFoundError) Is(target error) bool {
	_, ok := target.(*CLINotFoundError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *CLINotFoundError) Unwrap() error {
	return e.Cause
}

// NewCLINotFoundError creates a new CLINotFoundError with the given message.
func NewCLINotFoundError(message string) *CLINotFoundError {
	return &CLINotFoundError{Message: message}
}

// NewCLINotFoundErrorWithCause creates a new CLINotFoundError with the given message and cause.
func NewCLINotFoundErrorWithCause(message string, cause error) *CLINotFoundError {
	return &CLINotFoundError{Message: message, Cause: cause}
}

// CLIConnectionError indicates a failure to connect to the Claude Code CLI process.
// This can occur due to subprocess startup failures, pipe creation errors, or
// communication protocol issues.
type CLIConnectionError struct {
	Message string
	Cause   error
}

// Error returns the error message, implementing the error interface.
func (e *CLIConnectionError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Is checks if the target error is a CLIConnectionError.
func (e *CLIConnectionError) Is(target error) bool {
	_, ok := target.(*CLIConnectionError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *CLIConnectionError) Unwrap() error {
	return e.Cause
}

// NewCLIConnectionError creates a new CLIConnectionError with the given message.
func NewCLIConnectionError(message string) *CLIConnectionError {
	return &CLIConnectionError{Message: message}
}

// NewCLIConnectionErrorWithCause creates a new CLIConnectionError with the given message and cause.
func NewCLIConnectionErrorWithCause(message string, cause error) *CLIConnectionError {
	return &CLIConnectionError{Message: message, Cause: cause}
}

// ProcessError indicates an error with the Claude Code CLI subprocess.
// This includes unexpected termination, non-zero exit codes, or signal interruption.
type ProcessError struct {
	Message  string
	ExitCode int
	Cause    error
}

// Error returns the error message, implementing the error interface.
func (e *ProcessError) Error() string {
	msg := e.Message
	if e.ExitCode != 0 {
		msg = fmt.Sprintf("%s (exit code: %d)", msg, e.ExitCode)
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

// Is checks if the target error is a ProcessError.
func (e *ProcessError) Is(target error) bool {
	_, ok := target.(*ProcessError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *ProcessError) Unwrap() error {
	return e.Cause
}

// NewProcessError creates a new ProcessError with the given message.
func NewProcessError(message string) *ProcessError {
	return &ProcessError{Message: message}
}

// NewProcessErrorWithCode creates a new ProcessError with the given message and exit code.
func NewProcessErrorWithCode(message string, exitCode int) *ProcessError {
	return &ProcessError{Message: message, ExitCode: exitCode}
}

// NewProcessErrorWithCause creates a new ProcessError with the given message and cause.
func NewProcessErrorWithCause(message string, cause error) *ProcessError {
	return &ProcessError{Message: message, Cause: cause}
}

// JSONDecodeError indicates a failure to parse JSON data from the CLI.
// This can occur when the CLI sends malformed JSON or when the JSON structure
// doesn't match the expected schema.
type JSONDecodeError struct {
	Message string
	Raw     string // The raw JSON that failed to parse
	Cause   error
}

// Error returns the error message, implementing the error interface.
func (e *JSONDecodeError) Error() string {
	msg := e.Message
	if e.Raw != "" {
		// Truncate raw data if too long
		rawSnippet := e.Raw
		if len(rawSnippet) > 100 {
			rawSnippet = rawSnippet[:100] + "..."
		}
		msg = fmt.Sprintf("%s (raw: %s)", msg, rawSnippet)
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

// Is checks if the target error is a JSONDecodeError.
func (e *JSONDecodeError) Is(target error) bool {
	_, ok := target.(*JSONDecodeError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *JSONDecodeError) Unwrap() error {
	return e.Cause
}

// NewJSONDecodeError creates a new JSONDecodeError with the given message.
func NewJSONDecodeError(message string) *JSONDecodeError {
	return &JSONDecodeError{Message: message}
}

// NewJSONDecodeErrorWithRaw creates a new JSONDecodeError with the given message and raw JSON.
func NewJSONDecodeErrorWithRaw(message string, raw string) *JSONDecodeError {
	return &JSONDecodeError{Message: message, Raw: raw}
}

// NewJSONDecodeErrorWithCause creates a new JSONDecodeError with the given message, raw JSON, and cause.
func NewJSONDecodeErrorWithCause(message string, raw string, cause error) *JSONDecodeError {
	return &JSONDecodeError{Message: message, Raw: raw, Cause: cause}
}

// MessageParseError indicates a failure to parse a message from the CLI.
// This differs from JSONDecodeError in that the JSON may be valid but the message
// structure is invalid or unexpected (e.g., missing required fields, wrong types).
type MessageParseError struct {
	Message     string
	MessageType string // The type of message that failed to parse
	Cause       error
}

// Error returns the error message, implementing the error interface.
func (e *MessageParseError) Error() string {
	msg := e.Message
	if e.MessageType != "" {
		msg = fmt.Sprintf("%s (type: %s)", msg, e.MessageType)
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

// Is checks if the target error is a MessageParseError.
func (e *MessageParseError) Is(target error) bool {
	_, ok := target.(*MessageParseError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *MessageParseError) Unwrap() error {
	return e.Cause
}

// NewMessageParseError creates a new MessageParseError with the given message.
func NewMessageParseError(message string) *MessageParseError {
	return &MessageParseError{Message: message}
}

// NewMessageParseErrorWithType creates a new MessageParseError with the given message and message type.
func NewMessageParseErrorWithType(message string, messageType string) *MessageParseError {
	return &MessageParseError{Message: message, MessageType: messageType}
}

// NewMessageParseErrorWithCause creates a new MessageParseError with the given message, message type, and cause.
func NewMessageParseErrorWithCause(message string, messageType string, cause error) *MessageParseError {
	return &MessageParseError{Message: message, MessageType: messageType, Cause: cause}
}

// ControlProtocolError indicates a violation of the control protocol between
// the SDK and CLI. This includes invalid request/response sequences, unexpected
// control messages, or protocol version mismatches.
type ControlProtocolError struct {
	Message string
	Cause   error
}

// Error returns the error message, implementing the error interface.
func (e *ControlProtocolError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Is checks if the target error is a ControlProtocolError.
func (e *ControlProtocolError) Is(target error) bool {
	_, ok := target.(*ControlProtocolError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *ControlProtocolError) Unwrap() error {
	return e.Cause
}

// NewControlProtocolError creates a new ControlProtocolError with the given message.
func NewControlProtocolError(message string) *ControlProtocolError {
	return &ControlProtocolError{Message: message}
}

// NewControlProtocolErrorWithCause creates a new ControlProtocolError with the given message and cause.
func NewControlProtocolErrorWithCause(message string, cause error) *ControlProtocolError {
	return &ControlProtocolError{Message: message, Cause: cause}
}

// PermissionDeniedError indicates that a permission request was denied.
// This occurs when the user or permission callback denies a tool use request,
// or when a permission check fails.
type PermissionDeniedError struct {
	Message  string
	ToolName string // The tool that was denied
	Reason   string // Optional reason for denial
	Cause    error
}

// Error returns the error message, implementing the error interface.
func (e *PermissionDeniedError) Error() string {
	msg := e.Message
	if e.ToolName != "" {
		msg = fmt.Sprintf("%s (tool: %s)", msg, e.ToolName)
	}
	if e.Reason != "" {
		msg = fmt.Sprintf("%s - %s", msg, e.Reason)
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

// Is checks if the target error is a PermissionDeniedError.
func (e *PermissionDeniedError) Is(target error) bool {
	_, ok := target.(*PermissionDeniedError)
	return ok
}

// Unwrap returns the wrapped error.
func (e *PermissionDeniedError) Unwrap() error {
	return e.Cause
}

// NewPermissionDeniedError creates a new PermissionDeniedError with the given message.
func NewPermissionDeniedError(message string) *PermissionDeniedError {
	return &PermissionDeniedError{Message: message}
}

// NewPermissionDeniedErrorWithTool creates a new PermissionDeniedError with the given message and tool name.
func NewPermissionDeniedErrorWithTool(message string, toolName string) *PermissionDeniedError {
	return &PermissionDeniedError{Message: message, ToolName: toolName}
}

// NewPermissionDeniedErrorWithReason creates a new PermissionDeniedError with the given message, tool name, and reason.
func NewPermissionDeniedErrorWithReason(message string, toolName string, reason string) *PermissionDeniedError {
	return &PermissionDeniedError{Message: message, ToolName: toolName, Reason: reason}
}

// NewPermissionDeniedErrorWithCause creates a new PermissionDeniedError with the given message and cause.
func NewPermissionDeniedErrorWithCause(message string, cause error) *PermissionDeniedError {
	return &PermissionDeniedError{Message: message, Cause: cause}
}
