package types

import (
	"errors"
	"testing"
)

// TestCLINotFoundError tests CLINotFoundError creation and methods.
func TestCLINotFoundError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewCLINotFoundError("CLI not found")
		if err.Error() != "CLI not found" {
			t.Errorf("expected 'CLI not found', got '%s'", err.Error())
		}
	})

	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("path not found")
		err := NewCLINotFoundErrorWithCause("CLI not found", cause)
		if err.Unwrap() != cause {
			t.Error("expected unwrap to return cause")
		}
	})

	t.Run("errors.Is", func(t *testing.T) {
		err := NewCLINotFoundError("test")
		target := &CLINotFoundError{}
		if !errors.Is(err, target) {
			t.Error("expected errors.Is to return true for CLINotFoundError")
		}
	})
}

// TestProcessError tests ProcessError creation and methods.
func TestProcessError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewProcessError("process failed")
		if err.Error() != "process failed" {
			t.Errorf("expected 'process failed', got '%s'", err.Error())
		}
	})

	t.Run("error with exit code", func(t *testing.T) {
		err := NewProcessErrorWithCode("process failed", 1)
		expected := "process failed (exit code: 1)"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})
}

// TestJSONDecodeError tests JSONDecodeError creation and methods.
func TestJSONDecodeError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewJSONDecodeError("invalid JSON")
		if err.Error() != "invalid JSON" {
			t.Errorf("expected 'invalid JSON', got '%s'", err.Error())
		}
	})

	t.Run("error with raw data", func(t *testing.T) {
		err := NewJSONDecodeErrorWithRaw("invalid JSON", "{broken")
		if !containsSubstring(err.Error(), "invalid JSON") {
			t.Error("expected error message to contain 'invalid JSON'")
		}
		if !containsSubstring(err.Error(), "{broken") {
			t.Error("expected error message to contain raw data")
		}
	})
}

// TestMessageParseError tests MessageParseError creation and methods.
func TestMessageParseError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewMessageParseError("failed to parse")
		if err.Error() != "failed to parse" {
			t.Errorf("expected 'failed to parse', got '%s'", err.Error())
		}
	})

	t.Run("error with type", func(t *testing.T) {
		err := NewMessageParseErrorWithType("failed to parse", "user_message")
		if !containsSubstring(err.Error(), "user_message") {
			t.Error("expected error message to contain message type")
		}
	})
}

// TestPermissionDeniedError tests PermissionDeniedError creation and methods.
func TestPermissionDeniedError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewPermissionDeniedError("permission denied")
		if err.Error() != "permission denied" {
			t.Errorf("expected 'permission denied', got '%s'", err.Error())
		}
	})

	t.Run("error with tool name", func(t *testing.T) {
		err := NewPermissionDeniedErrorWithTool("permission denied", "Bash")
		if !containsSubstring(err.Error(), "Bash") {
			t.Error("expected error message to contain tool name")
		}
	})

	t.Run("error with reason", func(t *testing.T) {
		err := NewPermissionDeniedErrorWithReason("permission denied", "Bash", "unsafe operation")
		if !containsSubstring(err.Error(), "unsafe operation") {
			t.Error("expected error message to contain reason")
		}
	})
}

// Helper function to check if a string contains a substring.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
