package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// WithPermissions demonstrates how to control tool usage with permission callbacks.
// This example uses the CanUseTool callback to intercept tool requests.
func main() {
	ctx := context.Background()

	// Create options with permission callback
	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithAllowedTools("Bash", "Read", "Write").
		WithCanUseTool(permissionHandler)

	fmt.Println("Query: 'List files in the current directory using bash'")
	fmt.Println("---")

	// Send query with tool permission control
	messages, err := claude.Query(ctx, "List files in the current directory using bash", opts)
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
						fmt.Printf("Text: %s\n", block.Text)
					case *types.ToolUseBlock:
						fmt.Printf("Tool Use: %s (%s)\n", block.Name, block.ID)
						// Pretty print the input
						inputJSON, _ := json.MarshalIndent(block.Input, "", "  ")
						fmt.Printf("Input: %s\n", string(inputJSON))
					}
				}
			}
		case "result":
			fmt.Println("---")
			fmt.Println("Query completed")
		}
	}
}

// permissionHandler is called whenever Claude wants to use a tool.
// It receives the tool name, input parameters, and permission context.
func permissionHandler(ctx context.Context, toolName string, input map[string]interface{}, permCtx types.ToolPermissionContext) (interface{}, error) {
	fmt.Printf("\n[Permission Request] Tool: %s\n", toolName)

	// Pretty print the input
	inputJSON, _ := json.MarshalIndent(input, "", "  ")
	fmt.Printf("Input: %s\n", string(inputJSON))

	// For this example, we'll demonstrate different approval policies
	switch toolName {
	case "Bash":
		// Check if it's a risky command
		if cmd, ok := input["command"].(string); ok {
			if isRiskyCommand(cmd) {
				fmt.Println("Decision: DENIED (risky command)")
				return &types.PermissionResultDeny{
					Behavior:  "deny",
					Message:   "This command is too risky",
					Interrupt: false,
				}, nil
			}
		}
		fmt.Println("Decision: APPROVED")
		return &types.PermissionResultAllow{
			Behavior: "allow",
		}, nil

	case "Read":
		// Always allow read operations
		fmt.Println("Decision: APPROVED")
		return &types.PermissionResultAllow{
			Behavior: "allow",
		}, nil

	case "Write":
		// Always deny write operations for safety
		fmt.Println("Decision: DENIED (write operations disabled)")
		return &types.PermissionResultDeny{
			Behavior:  "deny",
			Message:   "Write operations are disabled",
			Interrupt: false,
		}, nil

	default:
		// Deny unknown tools
		fmt.Println("Decision: DENIED (unknown tool)")
		return &types.PermissionResultDeny{
			Behavior:  "deny",
			Message:   "Unknown tool",
			Interrupt: false,
		}, nil
	}
}

// isRiskyCommand checks if a bash command is potentially dangerous
func isRiskyCommand(cmd string) bool {
	// Simple heuristic - in production, use proper parsing
	riskyKeywords := []string{"rm -rf", "mkfs", "dd if=/dev"}
	for _, keyword := range riskyKeywords {
		if strings.Contains(cmd, keyword) {
			return true
		}
	}
	return false
}
