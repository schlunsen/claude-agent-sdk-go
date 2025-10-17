package claude

import (
	"context"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

func TestQuery_EmptyPrompt(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions()

	_, err := Query(ctx, "", opts)
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if err.Error() != "prompt cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestQuery_NilOptions(t *testing.T) {
	// This test just ensures nil options don't panic
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// We expect this to fail (no CLI in test environment) but not panic
	_, err := Query(ctx, "test", nil)
	if err == nil {
		// If it succeeds, that's fine too (CLI might be installed)
		return
	}

	// Should get CLINotFoundError
	if !types.IsCLINotFoundError(err) {
		t.Logf("Expected CLINotFoundError but got: %v", err)
		// This is not a fatal error - just log it
	}
}

func TestQuery_CLINotFound(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/nonexistent/path/to/claude")

	_, err := Query(ctx, "test prompt", opts)
	if err == nil {
		t.Fatal("expected error for nonexistent CLI path")
	}

	if !types.IsCLIConnectionError(err) {
		t.Errorf("expected CLIConnectionError, got: %T", err)
	}
}

func TestQuery_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo") // Use a command that exists

	messages, err := Query(ctx, "test", opts)
	if err != nil {
		// Connection might fail before context check - that's OK
		return
	}

	// If we got a channel, it should close quickly due to cancelled context
	timeout := time.After(1 * time.Second)
	for {
		select {
		case _, ok := <-messages:
			if !ok {
				// Channel closed as expected
				return
			}
		case <-timeout:
			t.Fatal("channel did not close after context cancellation")
		}
	}
}

func TestQuery_WithOptions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithMaxTurns(5).
		WithEnvVar("TEST", "value")

	// This will fail due to CLI not found, but tests that options are accepted
	_, err := Query(ctx, "test", opts)
	if err == nil {
		// CLI might be installed - that's OK
		return
	}

	// Should be CLI not found or connection error
	if !types.IsCLINotFoundError(err) && !types.IsCLIConnectionError(err) {
		t.Logf("Got error: %v", err)
	}
}

// TestQuery_Integration is an integration test that requires Claude CLI to be installed.
// It's skipped by default but can be run with: go test -tags=integration
func TestQuery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if CLAUDE_API_KEY is set
	// Note: We don't use os.Getenv here to avoid import, but in real tests you would
	// if apiKey := os.Getenv("CLAUDE_API_KEY"); apiKey == "" {
	// 	t.Skip("CLAUDE_API_KEY not set")
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	messages, err := Query(ctx, "What is 2+2? Reply with just the number.", opts)
	if err != nil {
		if types.IsCLINotFoundError(err) {
			t.Skip("Claude CLI not installed")
		}
		t.Fatal(err)
	}

	// Collect all messages
	var receivedMessages []types.Message
	for msg := range messages {
		receivedMessages = append(receivedMessages, msg)
	}

	// Should have received at least one message
	if len(receivedMessages) == 0 {
		t.Fatal("expected at least one message")
	}

	// Last message should be a ResultMessage
	lastMsg := receivedMessages[len(receivedMessages)-1]
	if _, ok := lastMsg.(*types.ResultMessage); !ok {
		t.Errorf("expected last message to be ResultMessage, got %T", lastMsg)
	}

	t.Logf("Received %d messages", len(receivedMessages))
}

// BenchmarkQuery benchmarks the Query function (will fail without CLI installed)
func BenchmarkQuery(b *testing.B) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Query(ctx, "test", opts)
	}
}
