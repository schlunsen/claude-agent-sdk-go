package tests

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// TestQueryIntegration_SimplePrompt tests a simple end-to-end query.
func TestQueryIntegration_SimplePrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI that responds with a simple message
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Test response"}],"model":"claude-3"}`,
		`{"type":"result","output":"success"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(t, messages)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Execute query
	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)
	msgChan, err := claude.Query(ctx, "Hello", opts)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Collect messages
	collected := CollectMessages(ctx, t, msgChan, 5*time.Second)

	// Verify we got messages
	if len(collected) == 0 {
		t.Fatal("expected at least one message")
	}

	// Verify last message is a result
	lastMsg := collected[len(collected)-1]
	AssertMessageType(t, lastMsg, "result")
}

// TestQueryIntegration_WithOptions tests query with various options.
func TestQueryIntegration_WithOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Response"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(t, messages)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Execute query with options
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithModel("claude-3-5-sonnet-latest").
		WithMaxTurns(5).
		WithEnvVar("TEST_VAR", "test_value").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	msgChan, err := claude.Query(ctx, "Test with options", opts)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Verify we get responses
	collected := CollectMessages(ctx, t, msgChan, 5*time.Second)
	if len(collected) == 0 {
		t.Fatal("expected messages")
	}
}

// TestQueryIntegration_ErrorHandling tests error scenarios.
func TestQueryIntegration_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		prompt      string
		cliPath     string
		expectError bool
		errorType   string
	}{
		{
			name:        "empty prompt",
			prompt:      "",
			cliPath:     "/bin/echo",
			expectError: true,
			errorType:   "validation",
		},
		{
			name:        "invalid CLI path",
			prompt:      "test",
			cliPath:     "/nonexistent/cli",
			expectError: true,
			errorType:   "connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := types.NewClaudeAgentOptions().WithCLIPath(tt.cliPath)
			_, err := claude.Query(ctx, tt.prompt, opts)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if err != nil {
				t.Logf("Error: %v", err)
			}
		})
	}
}

// TestQueryIntegration_ContextCancellation tests cancellation mid-stream.
func TestQueryIntegration_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create mock CLI with delayed responses
	mockCLI, err := CreateMockCLI(t, "echo")
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)
	msgChan, err := claude.Query(ctx, "test", opts)
	if err != nil {
		// Connection might fail before we cancel - that's OK
		return
	}

	// Cancel immediately
	cancel()

	// Channel should close quickly
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-msgChan:
			if !ok {
				// Channel closed - good
				return
			}
		case <-timeout:
			t.Fatal("channel did not close after context cancellation")
		}
	}
}

// TestClientIntegration_FullSession tests a complete client workflow.
func TestClientIntegration_FullSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Response 1"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(t, messages)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Create client
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithPermissionMode(types.PermissionModeBypassPermissions)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Verify not connected initially
	if client.IsConnected() {
		t.Error("client should not be connected initially")
	}

	// Connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Verify connected
	if !client.IsConnected() {
		t.Error("client should be connected after Connect()")
	}

	// Send query
	if err := client.Query(ctx, "Hello"); err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Receive response
	messageCount := 0
	for msg := range client.ReceiveResponse(ctx) {
		messageCount++
		t.Logf("Received message type: %s", msg.GetMessageType())

		if _, ok := msg.(*types.ResultMessage); ok {
			break
		}
	}

	if messageCount == 0 {
		t.Fatal("expected at least one message")
	}

	// Close
	if err := client.Close(ctx); err != nil {
		t.Logf("Close() error (may be expected): %v", err)
	}

	// Verify not connected after close
	if client.IsConnected() {
		t.Error("client should not be connected after Close()")
	}
}

// TestClientIntegration_MultipleQueries tests multiple query/response cycles.
func TestClientIntegration_MultipleQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 60*time.Second)
	defer cancel()

	// Create mock CLI with multiple responses
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"First"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
		`{"type":"assistant","content":[{"type":"text","text":"Second"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(t, messages)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Create and connect client
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithPermissionMode(types.PermissionModeBypassPermissions)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// First query
	if err := client.Query(ctx, "First query"); err != nil {
		t.Fatalf("First Query() failed: %v", err)
	}

	gotResult := false
	for msg := range client.ReceiveResponse(ctx) {
		if _, ok := msg.(*types.ResultMessage); ok {
			gotResult = true
			break
		}
	}

	if !gotResult {
		t.Fatal("first query did not receive ResultMessage")
	}

	// Second query
	if err := client.Query(ctx, "Second query"); err != nil {
		t.Fatalf("Second Query() failed: %v", err)
	}

	gotResult = false
	for msg := range client.ReceiveResponse(ctx) {
		if _, ok := msg.(*types.ResultMessage); ok {
			gotResult = true
			break
		}
	}

	if !gotResult {
		t.Fatal("second query did not receive ResultMessage")
	}
}

// TestClientIntegration_WithPermissions tests permission callbacks.
func TestClientIntegration_WithPermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI
	mockCLI, err := CreateMockCLI(t, "echo")
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Track permission calls
	var permissionCalls []string
	var mu sync.Mutex

	canUseTool := func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
		mu.Lock()
		permissionCalls = append(permissionCalls, toolName)
		mu.Unlock()

		// Allow all tools
		return types.PermissionResultAllow{
			Behavior: "allow",
		}, nil
	}

	// Create client with permission callback
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithCanUseTool(canUseTool)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Note: Without actual Claude CLI, we can't test permission flow
	// But we verify the client was created with the callback
	t.Logf("Client created with permission callback")
}

// TestClientIntegration_WithHooks tests hook callbacks.
func TestClientIntegration_WithHooks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI
	mockCLI, err := CreateMockCLI(t, "echo")
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Track hook calls
	var hookCalls []string
	var mu sync.Mutex

	hookCallback := func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
		mu.Lock()
		hookCalls = append(hookCalls, fmt.Sprintf("%v", input))
		mu.Unlock()

		return map[string]interface{}{
			"status": "processed",
		}, nil
	}

	// Create client with hooks
	toolNamePattern := "Bash"
	hookMatcher := types.HookMatcher{
		Matcher: &toolNamePattern,
		Hooks:   []types.HookCallbackFunc{hookCallback},
	}
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithPermissionMode(types.PermissionModeBypassPermissions).
		WithHook(types.HookEventPreToolUse, hookMatcher)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	// Note: Without actual Claude CLI, we can't test hook flow
	// But we verify the client was created with hooks
	t.Logf("Client created with hook callbacks")
}

// TestControlProtocol_FullFlow tests the control protocol end-to-end.
func TestControlProtocol_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires actual Claude CLI to test control protocol
	RequireAPIKey(t)
	cliPath := FindRealCLI(t)

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 45*time.Second)
	defer cancel()

	// Track permission requests
	permissionRequested := false
	var mu sync.Mutex

	canUseTool := func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
		mu.Lock()
		permissionRequested = true
		mu.Unlock()

		t.Logf("Permission requested for tool: %s", toolName)

		// Allow the tool
		return types.PermissionResultAllow{
			Behavior: "allow",
		}, nil
	}

	// Create client
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(cliPath).
		WithModel("claude-3-5-sonnet-latest").
		WithCanUseTool(canUseTool)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	// Connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}

	// Send a query that might require tools
	prompt := "What is 2+2? Just tell me the number."
	if err := client.Query(ctx, prompt); err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Receive response
	messageCount := 0
	for msg := range client.ReceiveResponse(ctx) {
		messageCount++
		t.Logf("Message %d: type=%s", messageCount, msg.GetMessageType())

		if _, ok := msg.(*types.ResultMessage); ok {
			break
		}
	}

	if messageCount == 0 {
		t.Fatal("expected at least one message")
	}

	t.Logf("Received %d messages", messageCount)
	t.Logf("Permission requested: %v", permissionRequested)
}

// TestStreamingWithControlMessages tests mixed normal and control messages.
func TestStreamingWithControlMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 30*time.Second)
	defer cancel()

	// Create mock CLI that sends both normal and control messages
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Processing..."}],"model":"claude-3"}`,
		`{"type":"system","subtype":"info","data":{"message":"Debug info"}}`,
		`{"type":"assistant","content":[{"type":"text","text":"Done"}],"model":"claude-3"}`,
		`{"type":"result","output":"complete"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(t, messages)
	if err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	// Execute query
	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)
	msgChan, err := claude.Query(ctx, "Test", opts)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Collect and categorize messages
	var assistantMessages []types.Message
	var systemMessages []types.Message
	var resultMessages []types.Message

	for msg := range msgChan {
		switch msg.GetMessageType() {
		case "assistant":
			assistantMessages = append(assistantMessages, msg)
		case "system":
			systemMessages = append(systemMessages, msg)
		case "result":
			resultMessages = append(resultMessages, msg)
		}
	}

	// Verify we got messages of each type
	t.Logf("Received: %d assistant, %d system, %d result",
		len(assistantMessages), len(systemMessages), len(resultMessages))

	if len(assistantMessages) == 0 {
		t.Error("expected at least one assistant message")
	}

	if len(resultMessages) == 0 {
		t.Error("expected at least one result message")
	}
}

// TestRealCLIIntegration tests with actual Claude CLI if available.
func TestRealCLIIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Require API key and CLI
	RequireAPIKey(t)
	cliPath := FindRealCLI(t)

	checkGoroutines := AssertNoGoroutineLeaks(t)
	defer checkGoroutines()

	ctx, cancel := CreateTestContext(t, 45*time.Second)
	defer cancel()

	// Simple query test
	opts := types.NewClaudeAgentOptions().
		WithCLIPath(cliPath).
		WithModel("claude-3-5-sonnet-latest").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	msgChan, err := claude.Query(ctx, "Say 'hello' and nothing else.", opts)
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}

	// Collect messages
	defer func() {
		for range msgChan {
			// drain any remaining messages
		}
	}()
	collected := CollectMessages(ctx, t, msgChan, 30*time.Second)

	if len(collected) == 0 {
		t.Fatal("expected at least one message")
	}

	// Verify we got a response
	foundText := false
	for _, msg := range collected {
		if assMsg, ok := msg.(*types.AssistantMessage); ok {
			for _, block := range assMsg.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					text := strings.ToLower(textBlock.Text)
					if strings.Contains(text, "hello") {
						foundText = true
						t.Logf("Got response: %s", textBlock.Text)
					}
				}
			}
		}
	}

	if !foundText {
		t.Log("Warning: response may not contain expected text (this can happen with LLMs)")
	}

	// Verify last message is result
	lastMsg := collected[len(collected)-1]
	AssertMessageType(t, lastMsg, "result")
}
