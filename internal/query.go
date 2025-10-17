package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/schlunsen/claude-agent-sdk-go/internal/transport"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// Query manages bidirectional control message handling.
// It orchestrates message routing between the transport and application callbacks,
// handling permissions, hooks, and MCP message routing.
type Query struct {
	// Transport and lifecycle
	transport transport.Transport
	ctx       context.Context
	cancel    context.CancelFunc

	// Request tracking
	mu                 sync.Mutex
	requestMap         map[string]chan responseResult
	nextRequestID      int64
	hookCallbacks      map[string]types.HookCallbackFunc
	nextHookCallbackID int64

	// Callbacks
	canUseTool types.CanUseToolFunc
	hooks      map[types.HookEvent][]types.HookMatcher
	mcpServers map[string]types.MCPServer

	// Message handling
	messagesChan     chan types.Message
	stopChan         chan struct{}
	readLoopDone     chan struct{}
	started          bool
	initialized      bool
	initializeResult map[string]interface{}
	isStreamingMode  bool
}

// responseResult wraps the response or error from a control request.
type responseResult struct {
	response map[string]interface{}
	err      error
}

// NewQuery creates a new Query handler.
func NewQuery(ctx context.Context, transport transport.Transport, opts *types.ClaudeAgentOptions, isStreamingMode bool) *Query {
	queryCtx, cancel := context.WithCancel(ctx)

	q := &Query{
		transport:       transport,
		ctx:             queryCtx,
		cancel:          cancel,
		requestMap:      make(map[string]chan responseResult),
		hookCallbacks:   make(map[string]types.HookCallbackFunc),
		messagesChan:    make(chan types.Message, 100),
		stopChan:        make(chan struct{}),
		readLoopDone:    make(chan struct{}),
		isStreamingMode: isStreamingMode,
		mcpServers:      make(map[string]types.MCPServer),
	}

	if opts != nil {
		q.canUseTool = opts.CanUseTool
		q.hooks = opts.Hooks
	}

	return q
}

// Initialize sends initialization control request if in streaming mode.
func (q *Query) Initialize(ctx context.Context) (map[string]interface{}, error) {
	if !q.isStreamingMode {
		return nil, nil
	}

	if q.initialized {
		return q.initializeResult, nil
	}

	// Build hooks configuration
	hooksConfig := make(map[string]interface{})
	if q.hooks != nil {
		for event, matchers := range q.hooks {
			if len(matchers) == 0 {
				continue
			}

			eventHooks := make([]map[string]interface{}, 0, len(matchers))
			for _, matcher := range matchers {
				callbackIDs := make([]string, 0, len(matcher.Hooks))
				for _, callback := range matcher.Hooks {
					callbackID := q.registerHookCallback(callback)
					callbackIDs = append(callbackIDs, callbackID)
				}

				hookConfig := map[string]interface{}{
					"hookCallbackIds": callbackIDs,
				}
				if matcher.Matcher != nil {
					hookConfig["matcher"] = *matcher.Matcher
				}
				eventHooks = append(eventHooks, hookConfig)
			}
			hooksConfig[string(event)] = eventHooks
		}
	}

	// Send initialize request
	request := map[string]interface{}{
		"subtype": "initialize",
	}
	if len(hooksConfig) > 0 {
		request["hooks"] = hooksConfig
	}

	result, err := q.sendControlRequest(ctx, request)
	if err != nil {
		return nil, types.NewControlProtocolErrorWithCause("initialization failed", err)
	}

	q.initialized = true
	q.initializeResult = result
	return result, nil
}

// Start begins the control message handling loop.
func (q *Query) Start(ctx context.Context) error {
	q.mu.Lock()
	if q.started {
		q.mu.Unlock()
		return types.NewControlProtocolError("query already started")
	}
	q.started = true
	q.mu.Unlock()

	// Start message reading loop
	go q.messageLoop()

	return nil
}

// Stop gracefully stops the query handler.
func (q *Query) Stop(ctx context.Context) error {
	// Signal stop
	select {
	case <-q.stopChan:
		// Already stopped
		return nil
	default:
		close(q.stopChan)
	}

	// Cancel context to stop all operations
	q.cancel()

	// Wait for read loop to complete
	select {
	case <-q.readLoopDone:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Close message channel
	close(q.messagesChan)

	return nil
}

// GetMessages returns a channel for consuming normal (non-control) messages.
func (q *Query) GetMessages(ctx context.Context) <-chan types.Message {
	return q.messagesChan
}

// messageLoop reads messages from transport and routes them.
func (q *Query) messageLoop() {
	defer close(q.readLoopDone)

	messages := q.transport.ReadMessages(q.ctx)

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-q.stopChan:
			return
		case msg, ok := <-messages:
			if !ok {
				// Channel closed - transport has stopped
				return
			}

			// Route message based on type
			if err := q.routeMessage(msg); err != nil {
				// Log error but continue processing
				// In a production system, we might want to report this via an error channel
				continue
			}
		}
	}
}

// routeMessage routes a message to the appropriate handler.
func (q *Query) routeMessage(msg types.Message) error {
	// Check message type
	msgType := msg.GetMessageType()

	// Handle control responses
	if msgType == "control_response" {
		if sysMsg, ok := msg.(*types.SystemMessage); ok {
			return q.handleControlResponse(sysMsg)
		}
		return types.NewControlProtocolError("invalid control_response message type")
	}

	// Handle control requests
	if msgType == "control_request" {
		if sysMsg, ok := msg.(*types.SystemMessage); ok {
			go q.handleControlRequest(sysMsg)
			return nil
		}
		return types.NewControlProtocolError("invalid control_request message type")
	}

	// Regular message - send to consumer
	select {
	case q.messagesChan <- msg:
		return nil
	case <-q.ctx.Done():
		return q.ctx.Err()
	}
}

// handleControlResponse handles a control response message.
func (q *Query) handleControlResponse(msg *types.SystemMessage) error {
	// Parse response
	responseData, ok := msg.Data["response"].(map[string]interface{})
	if !ok {
		return types.NewControlProtocolError("invalid control response format")
	}

	requestID, ok := responseData["request_id"].(string)
	if !ok {
		return types.NewControlProtocolError("missing request_id in control response")
	}

	// Find pending request
	q.mu.Lock()
	responseChan, exists := q.requestMap[requestID]
	if exists {
		delete(q.requestMap, requestID)
	}
	q.mu.Unlock()

	if !exists {
		// Orphaned response - might be a timeout or duplicate
		return nil
	}

	// Check for error response
	subtype, _ := responseData["subtype"].(string)
	if subtype == "error" {
		errMsg, _ := responseData["error"].(string)
		if errMsg == "" {
			errMsg = "unknown control protocol error"
		}
		select {
		case responseChan <- responseResult{err: types.NewControlProtocolError(errMsg)}:
		case <-q.ctx.Done():
		}
		return nil
	}

	// Success response
	response, _ := responseData["response"].(map[string]interface{})
	select {
	case responseChan <- responseResult{response: response}:
	case <-q.ctx.Done():
	}

	return nil
}

// handleControlRequest handles an incoming control request from CLI.
func (q *Query) handleControlRequest(msg *types.SystemMessage) {
	requestID, _ := msg.Data["request_id"].(string)
	requestData, _ := msg.Data["request"].(map[string]interface{})

	if requestID == "" || requestData == nil {
		q.sendErrorResponse(requestID, "invalid control request format")
		return
	}

	subtype, _ := requestData["subtype"].(string)

	var response map[string]interface{}
	var err error

	switch subtype {
	case "can_use_tool":
		response, err = q.handlePermissionRequest(requestData)
	case "hook_callback":
		response, err = q.handleHookCallback(requestData)
	case "mcp_message":
		response, err = q.handleMCPMessage(requestData)
	case "interrupt":
		// Handle interrupt - just acknowledge for now
		response = make(map[string]interface{})
	case "set_permission_mode":
		// Handle permission mode change - acknowledge for now
		response = make(map[string]interface{})
	default:
		err = types.NewControlProtocolError("unsupported control request subtype: " + subtype)
	}

	if err != nil {
		q.sendErrorResponse(requestID, err.Error())
		return
	}

	q.sendSuccessResponse(requestID, response)
}

// handlePermissionRequest handles a permission request for tool use.
func (q *Query) handlePermissionRequest(requestData map[string]interface{}) (map[string]interface{}, error) {
	if q.canUseTool == nil {
		return nil, types.NewControlProtocolError("canUseTool callback is not provided")
	}

	toolName, _ := requestData["tool_name"].(string)
	input, _ := requestData["input"].(map[string]interface{})
	suggestions, _ := requestData["permission_suggestions"].([]interface{})

	if toolName == "" || input == nil {
		return nil, types.NewControlProtocolError("missing tool_name or input in permission request")
	}

	// Build permission context
	permissionUpdates := make([]types.PermissionUpdate, 0)
	for _, s := range suggestions {
		if suggestionMap, ok := s.(map[string]interface{}); ok {
			// Parse suggestion into PermissionUpdate
			// This is a simplified version - production code should handle all fields
			suggestionJSON, _ := json.Marshal(suggestionMap)
			var update types.PermissionUpdate
			if err := json.Unmarshal(suggestionJSON, &update); err == nil {
				permissionUpdates = append(permissionUpdates, update)
			}
		}
	}

	ctx := types.ToolPermissionContext{
		Suggestions: permissionUpdates,
	}

	// Call permission callback
	result, err := q.canUseTool(q.ctx, toolName, input, ctx)
	if err != nil {
		return nil, err
	}

	// Convert result to response format
	response := make(map[string]interface{})

	switch r := result.(type) {
	case types.PermissionResultAllow:
		response["behavior"] = "allow"
		if r.UpdatedInput != nil {
			response["updatedInput"] = *r.UpdatedInput
		} else {
			response["updatedInput"] = input
		}
		if len(r.UpdatedPermissions) > 0 {
			response["updatedPermissions"] = r.UpdatedPermissions
		}

	case *types.PermissionResultAllow:
		response["behavior"] = "allow"
		if r.UpdatedInput != nil {
			response["updatedInput"] = *r.UpdatedInput
		} else {
			response["updatedInput"] = input
		}
		if len(r.UpdatedPermissions) > 0 {
			response["updatedPermissions"] = r.UpdatedPermissions
		}

	case types.PermissionResultDeny:
		response["behavior"] = "deny"
		if r.Message != "" {
			response["message"] = r.Message
		}
		if r.Interrupt {
			response["interrupt"] = r.Interrupt
		}

	case *types.PermissionResultDeny:
		response["behavior"] = "deny"
		if r.Message != "" {
			response["message"] = r.Message
		}
		if r.Interrupt {
			response["interrupt"] = r.Interrupt
		}

	default:
		return nil, types.NewControlProtocolError("permission callback returned invalid type")
	}

	return response, nil
}

// handleHookCallback handles a hook callback request.
func (q *Query) handleHookCallback(requestData map[string]interface{}) (map[string]interface{}, error) {
	callbackID, _ := requestData["callback_id"].(string)
	input := requestData["input"]
	toolUseID, _ := requestData["tool_use_id"].(*string)

	if callbackID == "" {
		return nil, types.NewControlProtocolError("missing callback_id in hook callback request")
	}

	// Find callback
	q.mu.Lock()
	callback, exists := q.hookCallbacks[callbackID]
	q.mu.Unlock()

	if !exists {
		return nil, types.NewControlProtocolError("no hook callback found for ID: " + callbackID)
	}

	// Build hook context
	hookCtx := types.HookContext{}

	// Call hook callback
	hookOutput, err := callback(q.ctx, input, toolUseID, hookCtx)
	if err != nil {
		return nil, err
	}

	// Convert hook output to response
	// The callback should return a map[string]interface{} representing the hook output
	response, ok := hookOutput.(map[string]interface{})
	if !ok {
		return nil, types.NewControlProtocolError("hook callback must return map[string]interface{}")
	}

	return response, nil
}

// handleMCPMessage handles an MCP message request.
func (q *Query) handleMCPMessage(requestData map[string]interface{}) (map[string]interface{}, error) {
	serverName, _ := requestData["server_name"].(string)
	message, _ := requestData["message"].(map[string]interface{})

	if serverName == "" || message == nil {
		return nil, types.NewControlProtocolError("missing server_name or message in MCP request")
	}

	// Find MCP server
	q.mu.Lock()
	server, exists := q.mcpServers[serverName]
	q.mu.Unlock()

	if !exists {
		// Return JSONRPC error response
		messageID := message["id"]
		return map[string]interface{}{
			"mcp_response": map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      messageID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": fmt.Sprintf("Server '%s' not found", serverName),
				},
			},
		}, nil
	}

	// Route message to MCP server
	mcpResponse, err := server.HandleMessage(message)
	if err != nil {
		// Return JSONRPC error response
		messageID := message["id"]
		return map[string]interface{}{
			"mcp_response": map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      messageID,
				"error": map[string]interface{}{
					"code":    -32603,
					"message": err.Error(),
				},
			},
		}, nil
	}

	return map[string]interface{}{
		"mcp_response": mcpResponse,
	}, nil
}

// sendControlRequest sends a control request to CLI and waits for response.
func (q *Query) sendControlRequest(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	if !q.isStreamingMode {
		return nil, types.NewControlProtocolError("control requests require streaming mode")
	}

	// Generate unique request ID
	requestID := q.generateRequestID()

	// Create response channel
	responseChan := make(chan responseResult, 1)
	q.mu.Lock()
	q.requestMap[requestID] = responseChan
	q.mu.Unlock()

	// Build control request
	controlRequest := map[string]interface{}{
		"type":       "control_request",
		"request_id": requestID,
		"request":    request,
	}

	// Marshal and send
	data, err := json.Marshal(controlRequest)
	if err != nil {
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, types.NewControlProtocolErrorWithCause("failed to marshal control request", err)
	}

	if err := q.transport.Write(ctx, string(data)); err != nil {
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, types.NewControlProtocolErrorWithCause("failed to send control request", err)
	}

	// Wait for response with timeout
	select {
	case result := <-responseChan:
		if result.err != nil {
			return nil, result.err
		}
		return result.response, nil
	case <-ctx.Done():
		q.mu.Lock()
		delete(q.requestMap, requestID)
		q.mu.Unlock()
		return nil, ctx.Err()
	}
}

// sendSuccessResponse sends a success control response.
func (q *Query) sendSuccessResponse(requestID string, response map[string]interface{}) {
	controlResponse := map[string]interface{}{
		"type": "control_response",
		"response": map[string]interface{}{
			"subtype":    "success",
			"request_id": requestID,
			"response":   response,
		},
	}

	data, err := json.Marshal(controlResponse)
	if err != nil {
		return
	}

	_ = q.transport.Write(q.ctx, string(data))
}

// sendErrorResponse sends an error control response.
func (q *Query) sendErrorResponse(requestID string, errorMsg string) {
	controlResponse := map[string]interface{}{
		"type": "control_response",
		"response": map[string]interface{}{
			"subtype":    "error",
			"request_id": requestID,
			"error":      errorMsg,
		},
	}

	data, err := json.Marshal(controlResponse)
	if err != nil {
		return
	}

	_ = q.transport.Write(q.ctx, string(data))
}

// generateRequestID generates a unique request ID.
func (q *Query) generateRequestID() string {
	id := atomic.AddInt64(&q.nextRequestID, 1)
	return fmt.Sprintf("req_%d", id)
}

// registerHookCallback registers a hook callback and returns its ID.
func (q *Query) registerHookCallback(callback types.HookCallbackFunc) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	id := atomic.AddInt64(&q.nextHookCallbackID, 1)
	callbackID := fmt.Sprintf("hook_%d", id)
	q.hookCallbacks[callbackID] = callback
	return callbackID
}

// AddMCPServer adds an MCP server for handling MCP messages.
func (q *Query) AddMCPServer(name string, server types.MCPServer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.mcpServers[name] = server
}

// matchesToolName checks if a tool name matches a matcher pattern.
// nolint:unused
func matchesToolName(toolName string, pattern *string) bool {
	if pattern == nil || *pattern == "" {
		return true // No pattern means match all
	}

	// Use regex for pattern matching
	regex, err := regexp.Compile(*pattern)
	if err != nil {
		return false
	}

	return regex.MatchString(toolName)
}
