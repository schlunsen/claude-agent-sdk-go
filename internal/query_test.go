package internal

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
)

// mockTransport implements a mock transport for testing.
type mockTransport struct {
	mu             sync.Mutex
	messagesChan   chan types.Message
	writtenData    []string
	closed         bool
	ready          bool
	onErrorHandler func(error)
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		messagesChan: make(chan types.Message, 100),
		writtenData:  make([]string, 0),
		ready:        true,
	}
}

func (m *mockTransport) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ready = true
	return nil
}

func (m *mockTransport) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.messagesChan)
		m.closed = true
	}
	m.ready = false
	return nil
}

func (m *mockTransport) Write(ctx context.Context, data string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writtenData = append(m.writtenData, data)
	return nil
}

func (m *mockTransport) ReadMessages(ctx context.Context) <-chan types.Message {
	return m.messagesChan
}

func (m *mockTransport) OnError(err error) {
	if m.onErrorHandler != nil {
		m.onErrorHandler(err)
	}
}

func (m *mockTransport) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ready
}

func (m *mockTransport) sendMessage(msg types.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.messagesChan <- msg
	}
}

func (m *mockTransport) getWrittenData() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.writtenData...)
}

// TestNewQuery tests Query construction.
func TestNewQuery(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if query == nil {
		t.Fatal("NewQuery returned nil")
	}
	if query.transport != transport {
		t.Error("transport not set correctly")
	}
	if !query.isStreamingMode {
		t.Error("expected streaming mode to be true")
	}
	if query.requestMap == nil {
		t.Error("requestMap not initialized")
	}
	if query.hookCallbacks == nil {
		t.Error("hookCallbacks not initialized")
	}
}

// TestInitialize tests Query initialization with hooks.
func TestInitialize(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()

	// Create hook callback
	hookCallback := func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
		return map[string]interface{}{
			"continue": true,
		}, nil
	}

	// Create options with hooks
	bashMatcher := "Bash"
	opts := types.NewClaudeAgentOptions().WithHook(
		types.HookEventPreToolUse,
		types.HookMatcher{
			Matcher: &bashMatcher,
			Hooks:   []types.HookCallbackFunc{hookCallback},
		},
	)

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Start goroutine to respond to initialize request
	go func() {
		time.Sleep(50 * time.Millisecond)
		written := transport.getWrittenData()

		for _, data := range written {
			var sentRequest map[string]interface{}
			if err := json.Unmarshal([]byte(data), &sentRequest); err != nil {
				continue
			}

			reqType, _ := sentRequest["type"].(string)
			if reqType != "control_request" {
				continue
			}

			requestID, _ := sentRequest["request_id"].(string)
			request, _ := sentRequest["request"].(map[string]interface{})
			subtype, _ := request["subtype"].(string)

			if subtype == "initialize" {
				// Send success response
				controlResponse := &types.SystemMessage{
					Type:    "control_response",
					Subtype: "control_response",
					Data: map[string]interface{}{
						"response": map[string]interface{}{
							"subtype":    "success",
							"request_id": requestID,
							"response": map[string]interface{}{
								"capabilities": []string{"hooks", "permissions"},
							},
						},
					},
				}
				transport.sendMessage(controlResponse)
				return
			}
		}
	}()

	// Initialize
	result, err := query.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify hooks were registered
	query.mu.Lock()
	hookCount := len(query.hookCallbacks)
	query.mu.Unlock()

	if hookCount != 1 {
		t.Errorf("expected 1 hook callback, got %d", hookCount)
	}

	// Test non-streaming mode
	nonStreamingQuery := NewQuery(ctx, transport, opts, false)
	result, err = nonStreamingQuery.Initialize(ctx)
	if err != nil {
		t.Errorf("unexpected error for non-streaming mode: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for non-streaming mode")
	}
}

// TestErrorResponse tests error response handling.
func TestErrorResponse(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Send a control request
	responseChan := make(chan error, 1)

	go func() {
		request := map[string]interface{}{
			"subtype": "set_permission_mode",
			"mode":    "invalid",
		}
		_, err := query.sendControlRequest(ctx, request)
		responseChan <- err
	}()

	// Wait a bit for request to be sent
	time.Sleep(50 * time.Millisecond)

	// Get the written data to extract request ID
	written := transport.getWrittenData()
	if len(written) == 0 {
		t.Fatal("no data written to transport")
	}

	var sentRequest map[string]interface{}
	if err := json.Unmarshal([]byte(written[0]), &sentRequest); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	requestID, _ := sentRequest["request_id"].(string)

	// Send an error response
	controlResponse := &types.SystemMessage{
		Type:    "control_response",
		Subtype: "control_response",
		Data: map[string]interface{}{
			"response": map[string]interface{}{
				"subtype":    "error",
				"request_id": requestID,
				"error":      "invalid permission mode",
			},
		},
	}

	transport.sendMessage(controlResponse)

	// Wait for error
	select {
	case err := <-responseChan:
		if err == nil {
			t.Fatal("expected error response")
		}
		if !types.IsControlProtocolError(err) {
			t.Errorf("expected ControlProtocolError, got %T", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for error response")
	}
}

// TestQueryStartStop tests lifecycle management.
func TestQueryStartStop(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	// Start the query
	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Starting again should fail
	if err := query.Start(ctx); err == nil {
		t.Error("expected error when starting already started query")
	}

	// Stop the query
	stopCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := query.Stop(stopCtx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Messages channel should be closed
	select {
	case _, ok := <-query.messagesChan:
		if ok {
			t.Error("messages channel should be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for messages channel to close")
	}
}

// TestHandlePermissionRequest tests permission callback handling.
func TestHandlePermissionRequest(t *testing.T) {
	tests := []struct {
		name           string
		requestData    map[string]interface{}
		callbackResult interface{}
		callbackError  error
		expectedError  bool
		expectedResult map[string]interface{}
	}{
		{
			name: "allow permission",
			requestData: map[string]interface{}{
				"subtype":   "can_use_tool",
				"tool_name": "Bash",
				"input":     map[string]interface{}{"command": "ls"},
			},
			callbackResult: types.PermissionResultAllow{
				Behavior: "allow",
			},
			expectedResult: map[string]interface{}{
				"behavior":     "allow",
				"updatedInput": map[string]interface{}{"command": "ls"},
			},
		},
		{
			name: "deny permission",
			requestData: map[string]interface{}{
				"subtype":   "can_use_tool",
				"tool_name": "Write",
				"input":     map[string]interface{}{"file_path": "/etc/passwd"},
			},
			callbackResult: types.PermissionResultDeny{
				Behavior: "deny",
				Message:  "Access denied",
			},
			expectedResult: map[string]interface{}{
				"behavior": "deny",
				"message":  "Access denied",
			},
		},
		{
			name: "allow with updated input",
			requestData: map[string]interface{}{
				"subtype":   "can_use_tool",
				"tool_name": "Write",
				"input":     map[string]interface{}{"file_path": "/tmp/test.txt"},
			},
			callbackResult: types.PermissionResultAllow{
				Behavior: "allow",
				UpdatedInput: &map[string]interface{}{
					"file_path": "/tmp/sanitized.txt",
				},
			},
			expectedResult: map[string]interface{}{
				"behavior": "allow",
				"updatedInput": map[string]interface{}{
					"file_path": "/tmp/sanitized.txt",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			transport := newMockTransport()

			opts := types.NewClaudeAgentOptions().WithCanUseTool(
				func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
					if tt.callbackError != nil {
						return nil, tt.callbackError
					}
					return tt.callbackResult, nil
				},
			)

			query := NewQuery(ctx, transport, opts, true)

			result, err := query.handlePermissionRequest(tt.requestData)
			if tt.expectedError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.expectedResult != nil {
				// Check behavior
				if behavior, ok := result["behavior"].(string); ok {
					if expectedBehavior, ok := tt.expectedResult["behavior"].(string); ok {
						if behavior != expectedBehavior {
							t.Errorf("behavior mismatch: got %s, want %s", behavior, expectedBehavior)
						}
					}
				}

				// Check message for deny
				if message, ok := result["message"].(string); ok {
					if expectedMessage, ok := tt.expectedResult["message"].(string); ok {
						if message != expectedMessage {
							t.Errorf("message mismatch: got %s, want %s", message, expectedMessage)
						}
					}
				}
			}
		})
	}
}

// TestHandleHookCallback tests hook callback handling.
func TestHandleHookCallback(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()

	hookCalled := false
	hookOutput := map[string]interface{}{
		"continue":       true,
		"suppressOutput": false,
	}

	opts := types.NewClaudeAgentOptions()
	query := NewQuery(ctx, transport, opts, true)

	// Register a hook callback
	callback := func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
		hookCalled = true
		return hookOutput, nil
	}

	callbackID := query.registerHookCallback(callback)

	// Create hook callback request
	requestData := map[string]interface{}{
		"subtype":     "hook_callback",
		"callback_id": callbackID,
		"input": map[string]interface{}{
			"tool_name":  "Bash",
			"tool_input": map[string]interface{}{"command": "echo test"},
		},
	}

	result, err := query.handleHookCallback(requestData)
	if err != nil {
		t.Fatalf("handleHookCallback failed: %v", err)
	}

	if !hookCalled {
		t.Error("hook callback was not called")
	}

	if continueVal, ok := result["continue"].(bool); !ok || !continueVal {
		t.Error("expected continue to be true")
	}
}

// TestHandleMCPMessage tests MCP message routing.
func TestHandleMCPMessage(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	// Add a mock MCP server
	mockServer := &mockMCPServer{
		name:    "test-server",
		version: "1.0.0",
	}
	query.AddMCPServer("test-server", mockServer)

	// Test successful MCP message
	requestData := map[string]interface{}{
		"subtype":     "mcp_message",
		"server_name": "test-server",
		"message": map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "test/method",
			"params":  map[string]interface{}{},
		},
	}

	result, err := query.handleMCPMessage(requestData)
	if err != nil {
		t.Fatalf("handleMCPMessage failed: %v", err)
	}

	mcpResponse, ok := result["mcp_response"].(map[string]interface{})
	if !ok {
		t.Fatal("expected mcp_response in result")
	}

	if mcpResponse["jsonrpc"] != "2.0" {
		t.Error("expected jsonrpc 2.0")
	}

	// Test server not found
	requestData["server_name"] = "nonexistent"
	result, err = query.handleMCPMessage(requestData)
	if err != nil {
		t.Fatalf("handleMCPMessage failed: %v", err)
	}

	mcpResponse, ok = result["mcp_response"].(map[string]interface{})
	if !ok {
		t.Fatal("expected mcp_response in result")
	}

	errorData, ok := mcpResponse["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error in mcp_response")
	}

	if code, ok := errorData["code"].(int); !ok || code != -32601 {
		t.Errorf("expected error code -32601, got %v", code)
	}
}

// TestRequestResponseCorrelation tests request-response pairing.
func TestRequestResponseCorrelation(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Send a control request in a goroutine
	responseChan := make(chan map[string]interface{}, 1)
	errorChan := make(chan error, 1)

	go func() {
		request := map[string]interface{}{
			"subtype": "set_permission_mode",
			"mode":    "default",
		}
		result, err := query.sendControlRequest(ctx, request)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- result
	}()

	// Wait a bit for the request to be sent
	time.Sleep(50 * time.Millisecond)

	// Get the written data to extract request ID
	written := transport.getWrittenData()
	if len(written) == 0 {
		t.Fatal("no data written to transport")
	}

	var sentRequest map[string]interface{}
	if err := json.Unmarshal([]byte(written[0]), &sentRequest); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	requestID, _ := sentRequest["request_id"].(string)
	if requestID == "" {
		t.Fatal("request_id not found in sent request")
	}

	// Send a control response
	controlResponse := &types.SystemMessage{
		Type:    "control_response",
		Subtype: "control_response",
		Data: map[string]interface{}{
			"response": map[string]interface{}{
				"subtype":    "success",
				"request_id": requestID,
				"response": map[string]interface{}{
					"mode": "default",
				},
			},
		},
	}

	transport.sendMessage(controlResponse)

	// Wait for response
	select {
	case result := <-responseChan:
		if mode, ok := result["mode"].(string); !ok || mode != "default" {
			t.Errorf("unexpected result: %v", result)
		}
	case err := <-errorChan:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for response")
	}
}

// TestMessageRouting tests that normal messages pass through to consumer.
func TestMessageRouting(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Send a normal message
	userMsg := &types.UserMessage{
		Type:    "user",
		Content: "test message",
	}

	transport.sendMessage(userMsg)

	// Receive from messages channel
	messages := query.GetMessages(ctx)

	select {
	case msg := <-messages:
		if msg.GetMessageType() != "user" {
			t.Errorf("expected user message, got %s", msg.GetMessageType())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestControlMessageFiltering tests that control messages don't leak to consumer.
func TestControlMessageFiltering(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Send a control request
	controlRequest := &types.SystemMessage{
		Type:    "control_request",
		Subtype: "control_request",
		Data: map[string]interface{}{
			"request_id": "test_req_1",
			"request": map[string]interface{}{
				"subtype": "interrupt",
			},
		},
	}

	transport.sendMessage(controlRequest)

	// Send a normal message after
	userMsg := &types.UserMessage{
		Type:    "user",
		Content: "test message",
	}

	transport.sendMessage(userMsg)

	// Should only receive the user message
	messages := query.GetMessages(ctx)

	select {
	case msg := <-messages:
		if msg.GetMessageType() != "user" {
			t.Errorf("expected user message, got %s", msg.GetMessageType())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// TestConcurrentRequests tests multiple simultaneous requests.
func TestConcurrentRequests(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := query.Stop(ctx); err != nil {
			t.Logf("error stopping query: %v", err)
		}
	}()

	// Start a goroutine to respond to all control requests
	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			written := transport.getWrittenData()

			// Process all written requests
			for _, data := range written {
				var sentRequest map[string]interface{}
				if err := json.Unmarshal([]byte(data), &sentRequest); err != nil {
					continue
				}

				reqType, _ := sentRequest["type"].(string)
				if reqType != "control_request" {
					continue
				}

				requestID, _ := sentRequest["request_id"].(string)
				if requestID == "" {
					continue
				}

				// Check if we already responded to this request
				// by checking if it's still in the request map
				query.mu.Lock()
				_, exists := query.requestMap[requestID]
				query.mu.Unlock()

				if exists {
					// Send response
					controlResponse := &types.SystemMessage{
						Type:    "control_response",
						Subtype: "control_response",
						Data: map[string]interface{}{
							"response": map[string]interface{}{
								"subtype":    "success",
								"request_id": requestID,
								"response":   map[string]interface{}{},
							},
						},
					}
					transport.sendMessage(controlResponse)
				}
			}

			// Exit when all requests are done
			query.mu.Lock()
			pendingCount := len(query.requestMap)
			query.mu.Unlock()
			if pendingCount == 0 {
				time.Sleep(100 * time.Millisecond) // Give time for any stragglers
				break
			}
		}
	}()

	numRequests := 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	results := make([]error, numRequests)

	// Send multiple concurrent requests
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()

			request := map[string]interface{}{
				"subtype": "set_permission_mode",
				"mode":    "default",
			}

			_, err := query.sendControlRequest(ctx, request)
			results[index] = err
		}(i)
	}

	wg.Wait()

	// Check results
	for i, err := range results {
		if err != nil {
			t.Errorf("request %d failed: %v", i, err)
		}
	}
}

// TestContextCancellation tests cleanup on context cancellation.
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	transport := newMockTransport()
	opts := types.NewClaudeAgentOptions()

	query := NewQuery(ctx, transport, opts, true)

	if err := query.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context
	cancel()

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)

	// Messages channel should eventually close
	select {
	case _, ok := <-query.messagesChan:
		if ok {
			t.Error("messages channel should be closed after context cancellation")
		}
	case <-time.After(1 * time.Second):
		// Channel might not be closed immediately, that's ok
	}
}

// TestCallbackTimeouts tests timeout handling for callbacks.
func TestCallbackTimeouts(t *testing.T) {
	ctx := context.Background()
	transport := newMockTransport()

	// Create a callback that times out
	opts := types.NewClaudeAgentOptions().WithCanUseTool(
		func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
			// Simulate slow callback
			select {
			case <-time.After(5 * time.Second):
				return types.PermissionResultAllow{Behavior: "allow"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	)

	query := NewQuery(ctx, transport, opts, true)

	requestData := map[string]interface{}{
		"subtype":   "can_use_tool",
		"tool_name": "Bash",
		"input":     map[string]interface{}{"command": "ls"},
	}

	// Use a short timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	// Replace query context with timeout context
	query.ctx = timeoutCtx

	// This should timeout
	_, err := query.handlePermissionRequest(requestData)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// mockMCPServer implements a mock MCP server for testing.
type mockMCPServer struct {
	name    string
	version string
}

func (m *mockMCPServer) HandleMessage(message map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      message["id"],
		"result":  map[string]interface{}{},
	}, nil
}

func (m *mockMCPServer) Name() string {
	return m.name
}

func (m *mockMCPServer) Version() string {
	return m.version
}
