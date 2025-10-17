package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/schlunsen/claude-agent-sdk-go/internal"
	"github.com/schlunsen/claude-agent-sdk-go/internal/transport"
	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
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
//   - All messages have been received (including the final ResultMessage)
//   - An error occurs
//   - The context is cancelled
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

	// Create subprocess transport
	transportInst := transport.NewSubprocessCLITransport(cliPath, cwd, env)

	// Connect to CLI
	if err := transportInst.Connect(ctx); err != nil {
		return nil, types.NewCLIConnectionErrorWithCause("failed to connect to Claude CLI", err)
	}

	// Create query handler (non-streaming mode)
	queryHandler := internal.NewQuery(ctx, transportInst, options, false)

	// Start message processing
	if err := queryHandler.Start(ctx); err != nil {
		_ = transportInst.Close(ctx)
		return nil, err
	}

	// Build the query message to send to CLI
	// For non-streaming mode, we send a single message with the prompt
	queryMsg := map[string]interface{}{
		"type":    "user",
		"content": prompt,
	}

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
					// Messages channel closed
					return
				}

				// Forward message to output
				select {
				case outputChan <- msg:
					// Check if this is a result message (end of query)
					if _, isResult := msg.(*types.ResultMessage); isResult {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return outputChan, nil
}
