package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/schlunsen/claude-agent-sdk-go/internal"
	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/internal/transport"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// Client provides bidirectional communication with Claude Code CLI for interactive sessions.
//
// Unlike the Query function which is designed for one-shot interactions, Client maintains
// a persistent connection and supports multiple query/response cycles, permission callbacks,
// hooks, and full control protocol features.
//
// Lifecycle:
//  1. Create client with NewClient()
//  2. Connect with Connect()
//  3. Send queries with Query()
//  4. Receive responses with ReceiveResponse()
//  5. Repeat steps 3-4 as needed
//  6. Clean up with Close()
//
// Example usage:
//
//	ctx := context.Background()
//	opts := types.NewClaudeAgentOptions().
//	    WithModel("claude-3-5-sonnet-latest").
//	    WithPermissionMode(types.PermissionModeAcceptEdits)
//
//	client, err := NewClient(ctx, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close(ctx)
//
//	// Connect to Claude
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// First query
//	if err := client.Query(ctx, "List files in current directory"); err != nil {
//	    log.Fatal(err)
//	}
//
//	for msg := range client.ReceiveResponse(ctx) {
//	    // Process messages
//	}
//
//	// Second query in same session
//	if err := client.Query(ctx, "Create a new file"); err != nil {
//	    log.Fatal(err)
//	}
//
//	for msg := range client.ReceiveResponse(ctx) {
//	    // Process messages
//	}
//
// Thread Safety:
//
// Client is not thread-safe. All methods should be called from the same goroutine,
// or you must provide your own synchronization.
type Client struct {
	options   *types.ClaudeAgentOptions
	transport transport.Transport
	query     *internal.Query
	logger    *log.Logger

	mu        sync.Mutex
	connected bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewClient creates a new interactive client with the given options.
//
// This does not establish a connection; you must call Connect() before sending queries.
//
// Parameters:
//   - ctx: Parent context for the client lifecycle
//   - options: Configuration options (nil uses defaults)
//
// Returns:
//   - A new Client instance
//   - An error if the CLI cannot be found or options are invalid
func NewClient(ctx context.Context, options *types.ClaudeAgentOptions) (*Client, error) {
	// Use default options if not provided
	if options == nil {
		options = types.NewClaudeAgentOptions()
	}

	// Validate permission callback configuration
	if options.CanUseTool != nil && options.PermissionPromptToolName != nil {
		return nil, fmt.Errorf("can_use_tool callback cannot be used with permission_prompt_tool_name")
	}

	// If CanUseTool is provided, automatically set PermissionPromptToolName to "stdio"
	if options.CanUseTool != nil && options.PermissionPromptToolName == nil {
		stdio := "stdio"
		options.PermissionPromptToolName = &stdio
	}

	// Find CLI path
	cliPath := ""
	if options.CLIPath != nil {
		cliPath = *options.CLIPath
	} else {
		var err error
		cliPath, err = transport.FindCLI()
		if err != nil {
			return nil, err
		}
	}

	// Determine working directory
	cwd := ""
	if options.CWD != nil {
		cwd = *options.CWD
	}

	// Prepare environment
	env := make(map[string]string)
	if options.Env != nil {
		for k, v := range options.Env {
			env[k] = v
		}
	}

	// Create client context
	clientCtx, cancel := context.WithCancel(ctx)

	// Create logger
	logger := log.NewLogger(options.Verbose)

	// Determine resume session ID from options
	resumeID := ""
	if options.Resume != nil && *options.Resume != "" {
		resumeID = *options.Resume
	}

	// Create subprocess transport with optional resume and options
	transportInst := transport.NewSubprocessCLITransport(cliPath, cwd, env, logger, resumeID, options)

	return &Client{
		options:   options,
		transport: transportInst,
		logger:    logger,
		connected: false,
		ctx:       clientCtx,
		cancel:    cancel,
	}, nil
}

// Connect establishes a connection to Claude Code CLI in streaming mode.
//
// This must be called before sending any queries. The connection uses streaming mode
// which enables full control protocol support including permissions, hooks, and
// bidirectional communication.
//
// Returns an error if:
//   - Already connected
//   - CLI subprocess fails to start
//   - Initialization fails
//
// Example:
//
//	if err := client.Connect(ctx); err != nil {
//	    if types.IsCLIConnectionError(err) {
//	        log.Fatal("Failed to connect:", err)
//	    }
//	    log.Fatal(err)
//	}
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return types.NewControlProtocolError("client already connected")
	}

	c.logger.Info("Connecting to Claude CLI...")

	// Connect transport
	if err := c.transport.Connect(ctx); err != nil {
		c.logger.Error("Failed to connect transport: %v", err)
		return types.NewCLIConnectionErrorWithCause("failed to connect to Claude CLI", err)
	}
	c.logger.Debug("Transport connected successfully")

	// Create query handler in streaming mode
	c.query = internal.NewQuery(ctx, c.transport, c.options, c.logger, true)
	c.logger.Debug("Query handler created")

	// Start message processing
	if err := c.query.Start(ctx); err != nil {
		c.logger.Error("Failed to start message processing: %v", err)
		_ = c.transport.Close(ctx)
		return err
	}
	c.logger.Debug("Message processing started")

	// Initialize control protocol
	if _, err := c.query.Initialize(ctx); err != nil {
		c.logger.Error("Failed to initialize control protocol: %v", err)
		_ = c.query.Stop(ctx)
		_ = c.transport.Close(ctx)
		return types.NewControlProtocolErrorWithCause("failed to initialize control protocol", err)
	}
	c.logger.Debug("Control protocol initialized")

	c.connected = true
	c.logger.Info("Successfully connected to Claude")
	return nil
}

// Query sends a prompt to Claude in the current session.
//
// This returns immediately after sending the prompt. Use ReceiveResponse() to
// get the response messages.
//
// Multiple calls to Query() can be made in sequence to have a multi-turn conversation.
// Each query/response cycle should be completed before sending the next query.
//
// Parameters:
//   - ctx: Context for cancellation
//   - prompt: The text prompt to send
//
// Returns an error if:
//   - Not connected (call Connect() first)
//   - Write to CLI fails
//   - Context is cancelled
//
// Example:
//
//	if err := client.Query(ctx, "What files are in this directory?"); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Now receive the response
//	for msg := range client.ReceiveResponse(ctx) {
//	    // Process messages
//	}
func (c *Client) Query(ctx context.Context, prompt string) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return types.NewCLIConnectionError("not connected - call Connect() first")
	}
	c.mu.Unlock()

	// Validate prompt
	if prompt == "" {
		return fmt.Errorf("prompt cannot be empty")
	}

	// Build query message
	queryMsg := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
		"parent_tool_use_id": nil,
		"session_id":         "default",
	}

	// Marshal and send
	data, err := json.Marshal(queryMsg)
	if err != nil {
		return types.NewControlProtocolErrorWithCause("failed to marshal query", err)
	}

	if err := c.transport.Write(ctx, string(data)); err != nil {
		return err
	}

	return nil
}

// QueryWithContent sends a structured content query (text + images) to Claude.
//
// This method allows sending messages with mixed content types (text and images),
// following the Claude API's content block format. Unlike Query() which only accepts
// plain text, this method accepts an array of content blocks.
//
// Content blocks can be:
//   - Text blocks: map[string]interface{}{"type": "text", "text": "..."}
//   - Image blocks: map[string]interface{}{"type": "image", "source": {...}}
//
// Example usage:
//
//	content := []interface{}{
//	    map[string]interface{}{
//	        "type": "text",
//	        "text": "What's in this image?",
//	    },
//	    map[string]interface{}{
//	        "type": "image",
//	        "source": map[string]interface{}{
//	            "type":       "base64",
//	            "media_type": "image/png",
//	            "data":       "iVBORw0KG...",
//	        },
//	    },
//	}
//
//	if err := client.QueryWithContent(ctx, content); err != nil {
//	    log.Fatal(err)
//	}
//
//	for msg := range client.ReceiveResponse(ctx) {
//	    // Process messages
//	}
func (c *Client) QueryWithContent(ctx context.Context, content interface{}) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return types.NewCLIConnectionError("not connected - call Connect() first")
	}
	c.mu.Unlock()

	// Validate content
	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	// Build query message with structured content
	queryMsg := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": content, // This can be a string or []ContentBlock
		},
		"parent_tool_use_id": nil,
		"session_id":         "default",
	}

	// Marshal and send
	data, err := json.Marshal(queryMsg)
	if err != nil {
		return types.NewControlProtocolErrorWithCause("failed to marshal query", err)
	}

	if err := c.transport.Write(ctx, string(data)); err != nil {
		return err
	}

	return nil
}

// ReceiveResponse returns a channel of response messages from Claude.
//
// This should be called after Query() to receive the response. The channel will
// receive messages until a ResultMessage is received, then it will be closed.
//
// The channel yields:
//   - UserMessage: Messages from the user (echoed back)
//   - AssistantMessage: Claude's text responses and tool uses
//   - SystemMessage: System notifications and control messages
//   - ResultMessage: Final result with cost/usage info (last message)
//
// The channel is closed when:
//   - A ResultMessage is received
//   - An error occurs
//   - The context is cancelled
//
// Example:
//
//	for msg := range client.ReceiveResponse(ctx) {
//	    switch m := msg.(type) {
//	    case *types.AssistantMessage:
//	        for _, block := range m.Content {
//	            if tb, ok := block.(*types.TextBlock); ok {
//	                fmt.Println("Claude:", tb.Text)
//	            }
//	        }
//	    case *types.ResultMessage:
//	        fmt.Printf("Done. Cost: $%.4f\n", *m.TotalCostUSD)
//	    }
//	}
func (c *Client) ReceiveResponse(ctx context.Context) <-chan types.Message {
	outputChan := make(chan types.Message, 10)

	go func() {
		defer close(outputChan)

		c.mu.Lock()
		if !c.connected || c.query == nil {
			c.mu.Unlock()
			return
		}
		messagesChan := c.query.GetMessages(ctx)
		c.mu.Unlock()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-messagesChan:
				if !ok {
					// Messages channel closed
					return
				}

				// Forward message to output
				select {
				case outputChan <- msg:
					// Check if this is a result message (end of response)
					if _, isResult := msg.(*types.ResultMessage); isResult {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return outputChan
}

// Close gracefully terminates the Claude session and cleans up resources.
//
// This should be called when you're done with the client, typically using defer:
//
//	client, err := NewClient(ctx, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close(ctx)
//
// After Close() is called, the client cannot be reused. Create a new client if needed.
//
// Returns an error if cleanup fails, but the client is marked as disconnected regardless.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.logger.Info("Closing Claude connection...")

	var errs []error

	// Stop query handler
	if c.query != nil {
		if err := c.query.Stop(ctx); err != nil {
			c.logger.Warning("Error stopping query handler: %v", err)
			errs = append(errs, err)
		}
		c.query = nil
	}

	// Close transport
	if c.transport != nil {
		if err := c.transport.Close(ctx); err != nil {
			c.logger.Warning("Error closing transport: %v", err)
			errs = append(errs, err)
		}
	}

	// Cancel context
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	c.connected = false
	c.logger.Debug("Connection closed")

	// Return first error if any
	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// IsConnected returns true if the client is currently connected to Claude.
//
// This can be used to check connection state before calling methods that require
// an active connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}
