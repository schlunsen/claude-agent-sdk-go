// Package claude provides a Go SDK for interacting with Claude through Claude Code CLI.
//
// The SDK provides two main ways to interact with Claude:
//
// 1. Query function for simple, one-shot interactions:
//
//	ctx := context.Background()
//	opts := types.NewClaudeAgentOptions().WithModel("claude-3-5-sonnet-latest")
//	messages, err := Query(ctx, "What is 2+2?", opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for msg := range messages {
//	    if assistantMsg, ok := msg.(*types.AssistantMessage); ok {
//	        for _, block := range assistantMsg.Content {
//	            if textBlock, ok := block.(*types.TextBlock); ok {
//	                fmt.Println(textBlock.Text)
//	            }
//	        }
//	    }
//	}
//
// 2. Client type for interactive, bidirectional conversations:
//
//	ctx := context.Background()
//	opts := types.NewClaudeAgentOptions()
//	client, err := NewClient(ctx, opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close(ctx)
//
//	if err := client.Connect(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	if err := client.Query(ctx, "Hello, Claude!"); err != nil {
//	    log.Fatal(err)
//	}
//
//	for msg := range client.ReceiveResponse(ctx) {
//	    // Process response messages
//	    if resultMsg, ok := msg.(*types.ResultMessage); ok {
//	        fmt.Printf("Cost: $%.4f\n", *resultMsg.TotalCostUSD)
//	        break
//	    }
//	}
//
// Query vs Client:
//
// Use Query when:
//   - You have a simple, one-off question
//   - You know all inputs upfront
//   - You don't need bidirectional communication
//   - You want fire-and-forget style interactions
//
// Use Client when:
//   - You need interactive conversations with follow-ups
//   - You want to send messages based on responses
//   - You need multiple query/response cycles in one session
//   - You need full control protocol support (permissions, hooks)
//
// Error Handling:
//
// The SDK provides typed errors for common failure scenarios:
//
//	if err := client.Connect(ctx); err != nil {
//	    if types.IsCLINotFoundError(err) {
//	        log.Fatal("Claude CLI not installed. Run: npm install -g @anthropic-ai/claude-code")
//	    }
//	    if types.IsCLIConnectionError(err) {
//	        log.Fatal("Failed to connect to Claude CLI:", err)
//	    }
//	    log.Fatal("Unexpected error:", err)
//	}
//
// Configuration:
//
// Use ClaudeAgentOptions to configure the SDK:
//
//	opts := types.NewClaudeAgentOptions().
//	    WithModel("claude-3-5-sonnet-latest").
//	    WithCWD("/path/to/project").
//	    WithPermissionMode(types.PermissionModeAcceptEdits).
//	    WithMaxTurns(10)
//
// Permission Callbacks:
//
// Control tool execution with permission callbacks:
//
//	opts := types.NewClaudeAgentOptions().
//	    WithCanUseTool(func(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
//	        // Allow safe tools automatically
//	        if toolName == "Read" {
//	            return types.PermissionResultAllow{
//	                Behavior: "allow",
//	            }, nil
//	        }
//	        // Deny dangerous tools
//	        return types.PermissionResultDeny{
//	            Behavior: "deny",
//	            Message:  "Tool not allowed",
//	        }, nil
//	    })
//
// Hooks:
//
// React to events during Claude's execution:
//
//	opts := types.NewClaudeAgentOptions().
//	    WithHook(types.HookEventPreToolUse, types.HookMatcher{
//	        Matcher: stringPtr("Write"),
//	        Hooks: []types.HookCallbackFunc{
//	            func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
//	                // Log or modify tool inputs before execution
//	                return map[string]interface{}{
//	                    "continue": true,
//	                }, nil
//	            },
//	        },
//	    })
//
// Context Cancellation:
//
// All operations respect context cancellation for clean shutdown:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	messages, err := Query(ctx, "Long running task", opts)
//	if err != nil {
//	    if err == context.DeadlineExceeded {
//	        log.Println("Query timed out")
//	    }
//	}
//
// For more examples and detailed usage, see the examples/ directory.
package claude
