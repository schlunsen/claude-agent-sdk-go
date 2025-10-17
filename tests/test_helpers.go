package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// MockCLI represents a mock Claude CLI subprocess for testing.
type MockCLI struct {
	Path       string
	ScriptPath string
	Cleanup    func()
}

// CreateMockCLI creates a temporary mock Claude CLI for testing.
// Returns the CLI path, cleanup function, and error.
func CreateMockCLI(t *testing.T, behavior string) (*MockCLI, error) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create script based on OS
	var scriptPath, cliPath string
	var scriptContent string

	if runtime.GOOS == "windows" {
		// Windows batch script
		scriptPath = filepath.Join(tmpDir, "mock-claude.bat")
		cliPath = scriptPath

		switch behavior {
		case "echo":
			scriptContent = `@echo off
type CON
`
		case "simple-response":
			scriptContent = `@echo off
echo {"type":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-3"}
echo {"type":"result","output":"success"}
`
		default:
			scriptContent = "@echo off\n"
		}
	} else {
		// Unix shell script
		scriptPath = filepath.Join(tmpDir, "mock-claude.sh")
		cliPath = scriptPath

		switch behavior {
		case "echo":
			scriptContent = `#!/bin/sh
cat
`
		case "simple-response":
			scriptContent = `#!/bin/sh
echo '{"type":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-3"}'
echo '{"type":"result","output":"success"}'
`
		case "control-response":
			scriptContent = `#!/bin/sh
# Read input and send control response
while IFS= read -r line; do
  echo "$line" >&2
  echo '{"type":"control_response","response":{"subtype":"success","request_id":"req_1","response":{}}}'
done
`
		default:
			scriptContent = "#!/bin/sh\n"
		}
	}

	// Write script
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return nil, fmt.Errorf("failed to write mock script: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return &MockCLI{
		Path:       cliPath,
		ScriptPath: scriptPath,
		Cleanup:    cleanup,
	}, nil
}

// CreateMockCLIWithMessages creates a mock CLI that outputs predefined messages.
func CreateMockCLIWithMessages(t *testing.T, messages []string) (*MockCLI, error) {
	t.Helper()

	tmpDir := t.TempDir()
	var scriptPath, cliPath string
	var scriptContent string

	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(tmpDir, "mock-claude.bat")
		cliPath = scriptPath
		scriptContent = "@echo off\n"
		for _, msg := range messages {
			scriptContent += fmt.Sprintf("echo %s\n", msg)
		}
	} else {
		scriptPath = filepath.Join(tmpDir, "mock-claude.sh")
		cliPath = scriptPath
		scriptContent = "#!/bin/sh\n"
		for _, msg := range messages {
			// Escape single quotes in message
			escaped := strings.ReplaceAll(msg, "'", "'\\''")
			scriptContent += fmt.Sprintf("echo '%s'\n", escaped)
		}
	}

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return nil, fmt.Errorf("failed to write mock script: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return &MockCLI{
		Path:       cliPath,
		ScriptPath: scriptPath,
		Cleanup:    cleanup,
	}, nil
}

// AssertMessageType checks if a message has the expected type.
func AssertMessageType(t *testing.T, msg types.Message, expected string) {
	t.Helper()

	if msg == nil {
		t.Fatalf("message is nil, expected type %s", expected)
	}

	actual := msg.GetMessageType()
	if actual != expected {
		t.Errorf("message type = %q, want %q", actual, expected)
	}
}

// AssertMessageContent checks if a message contains expected content.
func AssertMessageContent(t *testing.T, msg types.Message, expectedText string) {
	t.Helper()

	if msg == nil {
		t.Fatal("message is nil")
	}

	// Try to get content from assistant message
	if assMsg, ok := msg.(*types.AssistantMessage); ok {
		if len(assMsg.Content) > 0 {
			if textBlock, ok := assMsg.Content[0].(*types.TextBlock); ok {
				if !strings.Contains(textBlock.Text, expectedText) {
					t.Errorf("message text = %q, want to contain %q", textBlock.Text, expectedText)
				}
				return
			}
		}
	}

	// Try to get content from user message
	if userMsg, ok := msg.(*types.UserMessage); ok {
		content := fmt.Sprintf("%v", userMsg.Content)
		if !strings.Contains(content, expectedText) {
			t.Errorf("message content = %q, want to contain %q", content, expectedText)
		}
		return
	}

	t.Errorf("message type %T does not contain expected text content", msg)
}

// CollectMessages collects all messages from a channel with timeout.
func CollectMessages(ctx context.Context, t *testing.T, messages <-chan types.Message, timeout time.Duration) []types.Message {
	t.Helper()

	var collected []types.Message
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				// Channel closed
				return collected
			}
			collected = append(collected, msg)

			// Check if we got a result message (end of stream)
			if _, isResult := msg.(*types.ResultMessage); isResult {
				return collected
			}

		case <-timeoutCtx.Done():
			// Timeout - return what we have
			return collected
		}
	}
}

// AssertNoGoroutineLeaks checks for goroutine leaks.
// Returns the current goroutine count before test.
func AssertNoGoroutineLeaks(t *testing.T) func() {
	t.Helper()

	beforeCount := runtime.NumGoroutine()

	return func() {
		// Give goroutines time to clean up
		time.Sleep(100 * time.Millisecond)

		afterCount := runtime.NumGoroutine()

		// Allow some tolerance (e.g., test framework goroutines)
		tolerance := 2
		if afterCount > beforeCount+tolerance {
			t.Errorf("goroutine leak detected: before=%d, after=%d (diff=%d)",
				beforeCount, afterCount, afterCount-beforeCount)

			// Print goroutine stack for debugging
			buf := make([]byte, 1<<16)
			stackSize := runtime.Stack(buf, true)
			t.Logf("Goroutine stacks:\n%s", buf[:stackSize])
		}
	}
}

// FindRealCLI tries to find the actual Claude CLI for integration tests.
// Returns path and error. Skips test if not found.
func FindRealCLI(t *testing.T) string {
	t.Helper()

	// Check if CLAUDE_CLI_PATH is set
	if cliPath := os.Getenv("CLAUDE_CLI_PATH"); cliPath != "" {
		if _, err := os.Stat(cliPath); err == nil {
			return cliPath
		}
	}

	// Try to find in PATH
	if cliPath, err := exec.LookPath("claude"); err == nil {
		return cliPath
	}

	// Try common installation paths
	commonPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".npm-global", "bin", "claude"),
		"/usr/local/bin/claude",
		"/opt/homebrew/bin/claude",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	t.Skip("Claude CLI not found - set CLAUDE_CLI_PATH or install Claude CLI")
	return ""
}

// RequireAPIKey checks if CLAUDE_API_KEY is set, skips test if not.
func RequireAPIKey(t *testing.T) {
	t.Helper()

	if os.Getenv("CLAUDE_API_KEY") == "" {
		t.Skip("CLAUDE_API_KEY not set - skipping integration test")
	}
}

// CreateTestContext creates a context with timeout for tests.
func CreateTestContext(t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()

	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Register cleanup
	t.Cleanup(cancel)

	return ctx, cancel
}

// MarshalJSON is a helper to marshal JSON for tests.
func MarshalJSON(t *testing.T, v interface{}) string {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	return string(data)
}

// UnmarshalJSON is a helper to unmarshal JSON for tests.
func UnmarshalJSON(t *testing.T, data string, v interface{}) {
	t.Helper()

	if err := json.Unmarshal([]byte(data), v); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
}

// CopyEnv creates a copy of environment variables for testing.
func CopyEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// WithTimeout runs a function with a timeout, failing the test if it times out.
func WithTimeout(t *testing.T, timeout time.Duration, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatalf("operation timed out after %v", timeout)
	}
}

// ReadAll reads all data from a reader with timeout.
func ReadAll(t *testing.T, r io.Reader, timeout time.Duration) []byte {
	t.Helper()

	done := make(chan []byte)
	errChan := make(chan error)

	go func() {
		data, err := io.ReadAll(r)
		if err != nil {
			errChan <- err
			return
		}
		done <- data
	}()

	select {
	case data := <-done:
		return data
	case err := <-errChan:
		t.Fatalf("failed to read: %v", err)
		return nil
	case <-time.After(timeout):
		t.Fatalf("read timed out after %v", timeout)
		return nil
	}
}
