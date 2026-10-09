package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/schlunsen/claude-agent-sdk-go/internal"
	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/internal/transport"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// Query executes a single Claude query in non-streaming mode and returns a channel of messages.
// This is the simplest way to interact with Claude for one-off questions or batch processing.
//
// The function:
//   - Finds and connects to Claude Code CLI
//   - Sends the prompt in non-streaming mode (--print flag)
//   - Streams response messages to the returned channel
//   - Automatically cleans up resources when done
//
// The returned channel is read-only and will be closed when:
//   - The CLI has finished the run and exited
//   - An error occurs
//   - The context is cancelled
//
// A ResultMessage ends a turn, not necessarily the run: a background subagent
// can wake the session for follow-up turns, whose messages (and their own
// ResultMessage) arrive after the first one. The CLI bounds how long it waits
// for background work with CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS (10 minutes by
// default).
//
// Error handling:
//   - Connection errors are returned immediately
//   - Parse errors during message reading are sent to options.OnError callback if provided
//   - Context cancellation is respected throughout
//
// Example usage:
//
//	ctx := context.Background()
//	opts := types.NewClaudeAgentOptions().WithModel("claude-3-5-sonnet-latest")
//	messages, err := Query(ctx, "What is 2+2?", opts)
//	if err != nil {
//	    if types.IsCLINotFoundError(err) {
//	        log.Fatal("Claude CLI not installed")
//	    }
//	    log.Fatal(err)
//	}
//
//	for msg := range messages {
//	    switch m := msg.(type) {
//	    case *types.AssistantMessage:
//	        for _, block := range m.Content {
//	            if tb, ok := block.(*types.TextBlock); ok {
//	                fmt.Println(tb.Text)
//	            }
//	        }
//	    case *types.ResultMessage:
//	        fmt.Printf("Cost: $%.4f\n", *m.TotalCostUSD)
//	    }
//	}
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - prompt: The text prompt to send to Claude
//   - options: Configuration options (nil uses defaults)
//
// Returns:
//   - A read-only channel of Message types
//   - An error if connection or initialization fails
func Query(ctx context.Context, prompt string, options *types.ClaudeAgentOptions) (<-chan types.Message, error) {
	// Use default options if not provided
	if options == nil {
		options = types.NewClaudeAgentOptions()
	}

	// Validate prompt
	if prompt == "" {
		return nil, fmt.Errorf("prompt cannot be empty")
	}

	// Find Claude CLI path
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

	// Create transport with --print flag for non-streaming mode
	env := make(map[string]string)
	if options.Env != nil {
		for k, v := range options.Env {
			env[k] = v
		}
	}

	// Create logger with verbosity from options
	verbose := options != nil && options.Verbose
	logger := log.NewLogger(verbose)

	// Determine resume session ID from options
	resumeID := ""
	if options.Resume != nil && *options.Resume != "" {
		resumeID = *options.Resume
	}

	// Create subprocess transport with optional resume and options
	transportInst := transport.NewSubprocessCLITransport(cliPath, cwd, env, logger, resumeID, options)

	// Connect to CLI
	if err := transportInst.Connect(ctx); err != nil {
		return nil, types.NewCLIConnectionErrorWithCause("failed to connect to Claude CLI", err)
	}

	// Create query handler (non-streaming mode)
	queryHandler := internal.NewQuery(ctx, transportInst, options, logger, false)

	// Start message processing
	if err := queryHandler.Start(ctx); err != nil {
		_ = transportInst.Close(ctx)
		return nil, err
	}

	// Use resume ID as session ID, or default if not resuming
	sessionID := "default-session"
	if resumeID != "" {
		sessionID = resumeID
	}

	// Build the query message to send to CLI
	// Format matches Python SDK: type, message{role,content}, parent_tool_use_id, session_id
	queryMsg := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": prompt,
		},
		"parent_tool_use_id": nil,
		"session_id":         sessionID,
	}

	internal.StampUserMessage(queryMsg, options.VerbatimPrompts)

	// Marshal and send
	data, err := json.Marshal(queryMsg)
	if err != nil {
		_ = queryHandler.Stop(ctx)
		_ = transportInst.Close(ctx)
		return nil, types.NewControlProtocolErrorWithCause("failed to marshal query", err)
	}

	if err := transportInst.Write(ctx, string(data)); err != nil {
		_ = queryHandler.Stop(ctx)
		_ = transportInst.Close(ctx)
		return nil, err
	}

	// Close stdin once the run is over, so the CLI exits when it is done.
	// With hooks or CanUseTool this waits for the CLI to report the session
	// idle, since follow-up turns may still send control requests that need
	// a reply; otherwise stdin closes now.
	go func() {
		if err := queryHandler.WaitForRunEndAndEndInput(ctx); err != nil && ctx.Err() == nil {
			logger.Debug("Failed to end CLI input: %v", err)
		}
	}()

	// Create output channel for user
	outputChan := make(chan types.Message, 10)

	// Start goroutine to read messages and forward to output channel
	go func() {
		defer close(outputChan)
		defer func() {
			_ = queryHandler.Stop(ctx)
			_ = transportInst.Close(ctx)
		}()

		messagesChan := queryHandler.GetMessages(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-messagesChan:
				if !ok {
					// The CLI exited: the run is over
					return
				}

				// Forward message to output. A result does not end the
				// stream: follow-up turns may still come (see above).
				select {
				case outputChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return outputChan, nil
}
