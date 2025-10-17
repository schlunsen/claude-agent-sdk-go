package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// WithHooks demonstrates the hook system for responding to lifecycle events.
// Hooks are called at various points during query execution to allow custom logic.
func main() {
	ctx := context.Background()

	// Create hook matchers for PreToolUse and PostToolUse events
	preToolHook := types.HookMatcher{
		Hooks: []types.HookCallbackFunc{preToolUseHook},
	}

	postToolHook := types.HookMatcher{
		Hooks: []types.HookCallbackFunc{postToolUseHook},
	}

	// Create options with multiple hooks
	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithAllowedTools("Bash", "Read", "Write").
		WithHook(types.HookEventPreToolUse, preToolHook).
		WithHook(types.HookEventPostToolUse, postToolHook).
		WithPermissionMode(types.PermissionModeBypassPermissions)

	fmt.Println("Query: 'What is the current directory?'")
	fmt.Println("---")

	// Send query
	messages, err := claude.Query(ctx, "What is the current directory?", opts)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// Process messages
	for msg := range messages {
		msgType := msg.GetMessageType()

		switch msgType {
		case "assistant":
			if assistantMsg, ok := msg.(*types.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					switch block := block.(type) {
					case *types.TextBlock:
						fmt.Printf("[Message] %s\n", block.Text)
					case *types.ToolUseBlock:
						fmt.Printf("[Tool] Tool call received: %s\n", block.Name)
					}
				}
			}
		case "result":
			fmt.Println("---")
			fmt.Println("[Done] Query completed")
		}
	}
}

// preToolUseHook is called before Claude uses a tool
func preToolUseHook(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
	fmt.Println("[Hook] PreToolUse triggered")

	// You could implement custom logic here:
	// - Log tool usage for auditing
	// - Block certain operations
	// - Alert users about dangerous operations

	return map[string]interface{}{
		"continue": true,
	}, nil
}

// postToolUseHook is called after a tool execution completes
func postToolUseHook(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
	fmt.Println("[Hook] PostToolUse triggered")

	// You could implement custom logic here:
	// - Log tool results
	// - Validate outputs
	// - Clean up resources
	// - Chain additional operations

	return map[string]interface{}{
		"continue": true,
	}, nil
}
