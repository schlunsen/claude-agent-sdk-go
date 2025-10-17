package claude

import (
	"context"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

func TestNewClient_NilOptions(t *testing.T) {
	ctx := context.Background()

	client, err := NewClient(ctx, nil)
	if err == nil {
		// CLI might be installed - that's OK, just clean up
		if client != nil {
			_ = client.Close(ctx)
		}
		return
	}

	// Should get CLINotFoundError
	if !types.IsCLINotFoundError(err) {
		t.Logf("Expected CLINotFoundError but got: %v", err)
	}
}

func TestNewClient_InvalidCLIPath(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/nonexistent/path/to/claude")

	client, err := NewClient(ctx, opts)
	if err != nil {
		// Expected - CLI path doesn't exist
		// However, NewClient doesn't validate the path, only Connect does
		// So we might get a client back
		if client != nil {
			_ = client.Close(ctx)
		}
	}
}

func TestNewClient_ConflictingPermissionOptions(t *testing.T) {
	ctx := context.Background()

	// Create a dummy callback
	canUseTool := func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
		return types.PermissionResultAllow{Behavior: "allow"}, nil
	}

	// This should fail because both are set
	promptTool := "cli"
	opts := types.NewClaudeAgentOptions().
		WithCLIPath("/bin/echo").
		WithCanUseTool(canUseTool).
		WithPermissionPromptToolName(promptTool)

	_, err := NewClient(ctx, opts)
	if err == nil {
		t.Fatal("expected error for conflicting permission options")
	}

	if err.Error() != "can_use_tool callback cannot be used with permission_prompt_tool_name" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestClient_ConnectBeforeQuery(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Try to query without connecting
	err = client.Query(ctx, "test")
	if err == nil {
		t.Fatal("expected error when querying without connecting")
	}

	if !types.IsCLIConnectionError(err) {
		t.Errorf("expected CLIConnectionError, got: %T - %v", err, err)
	}
}

func TestClient_EmptyPrompt(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Without connecting, should get connection error first
	err = client.Query(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty prompt without connection")
	}

	// Should be connection error since we haven't connected
	if !types.IsCLIConnectionError(err) {
		t.Errorf("expected CLIConnectionError when not connected, got: %v", err)
	}
}

func TestClient_IsConnected(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Should not be connected initially
	if client.IsConnected() {
		t.Error("client should not be connected before Connect()")
	}

	// After close, should not be connected
	_ = client.Close(ctx)
	if client.IsConnected() {
		t.Error("client should not be connected after Close()")
	}
}

func TestClient_DoubleConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// First connect attempt (will likely fail with /bin/echo)
	err1 := client.Connect(ctx)

	// Second connect attempt
	err2 := client.Connect(ctx)

	// If first connect succeeded, second should fail with "already connected"
	if err1 == nil && err2 == nil {
		t.Error("expected error on second Connect() call")
	}

	// If second connect got an error, check if it's the right one
	if err2 != nil && types.IsControlProtocolError(err2) {
		// Good - got the expected error
		if err2.Error() != "client already connected" {
			t.Logf("Got control protocol error but unexpected message: %v", err2)
		}
	}
}

func TestClient_CloseIdempotent(t *testing.T) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}

	// Close multiple times should not panic or error
	err1 := client.Close(ctx)
	err2 := client.Close(ctx)
	err3 := client.Close(ctx)

	// All should succeed (or at least not panic)
	_ = err1
	_ = err2
	_ = err3
}

func TestClient_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	client, err := NewClient(ctx, opts)
	if err != nil {
		t.Skip("Could not create client")
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Cancel context before operations
	cancel()

	// Operations should respect cancellation
	err = client.Connect(ctx)
	// May fail due to cancellation or other reasons - just ensure no panic
	_ = err
}

// TestClient_Integration is an integration test that requires Claude CLI to be installed.
func TestClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	client, err := NewClient(ctx, opts)
	if err != nil {
		if types.IsCLINotFoundError(err) {
			t.Skip("Claude CLI not installed")
		}
		t.Fatal(err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Connect
	if err := client.Connect(ctx); err != nil {
		if types.IsCLIConnectionError(err) {
			t.Skip("Could not connect to Claude CLI")
		}
		t.Fatal(err)
	}

	// First query
	if err := client.Query(ctx, "What is 2+2? Reply with just the number."); err != nil {
		t.Fatal(err)
	}

	// Receive response
	var messageCount int
	for msg := range client.ReceiveResponse(ctx) {
		messageCount++
		if _, ok := msg.(*types.ResultMessage); ok {
			break
		}
	}

	if messageCount == 0 {
		t.Fatal("expected at least one message")
	}

	t.Logf("First query received %d messages", messageCount)

	// Second query in same session
	if err := client.Query(ctx, "What is 3+3? Reply with just the number."); err != nil {
		t.Fatal(err)
	}

	// Receive second response
	messageCount = 0
	for msg := range client.ReceiveResponse(ctx) {
		messageCount++
		if _, ok := msg.(*types.ResultMessage); ok {
			break
		}
	}

	if messageCount == 0 {
		t.Fatal("expected at least one message in second query")
	}

	t.Logf("Second query received %d messages", messageCount)
}

// TestClient_MultipleQueries tests multiple query/response cycles
func TestClient_MultipleQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	client, err := NewClient(ctx, opts)
	if err != nil {
		if types.IsCLINotFoundError(err) {
			t.Skip("Claude CLI not installed")
		}
		t.Fatal(err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	if err := client.Connect(ctx); err != nil {
		if types.IsCLIConnectionError(err) {
			t.Skip("Could not connect to Claude CLI")
		}
		t.Fatal(err)
	}

	// Send 3 queries in sequence
	queries := []string{
		"Say 'first'",
		"Say 'second'",
		"Say 'third'",
	}

	for i, prompt := range queries {
		if err := client.Query(ctx, prompt); err != nil {
			t.Fatalf("Query %d failed: %v", i+1, err)
		}

		// Receive response
		gotResult := false
		for msg := range client.ReceiveResponse(ctx) {
			if _, ok := msg.(*types.ResultMessage); ok {
				gotResult = true
				break
			}
		}

		if !gotResult {
			t.Fatalf("Query %d did not receive ResultMessage", i+1)
		}

		t.Logf("Query %d completed", i+1)
	}
}

// BenchmarkClient benchmarks the Client type
func BenchmarkClient_Create(b *testing.B) {
	ctx := context.Background()
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, err := NewClient(ctx, opts)
		if err == nil {
			_ = client.Close(ctx)
		}
	}
}
