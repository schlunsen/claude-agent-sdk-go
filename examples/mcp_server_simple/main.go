package main

import (
	"context"
	"fmt"

	sdk "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

func main() {
	ctx := context.Background()

	// Create an MCP server using the factory function
	// This eliminates ~80% of boilerplate code compared to manual implementation
	calculator, err := types.NewSDKMCPServer("calculator",
		// Add tool: addition
		types.Tool{
			Name:        "add",
			Description: "Add two numbers together",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{"type": "number", "description": "First number"},
					"b": map[string]interface{}{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				a, _ := args["a"].(float64)
				b, _ := args["b"].(float64)
				return map[string]any{
					"result":    a + b,
					"operation": "addition",
				}, nil
			},
		},
		// Add tool: multiplication
		types.Tool{
			Name:        "multiply",
			Description: "Multiply two numbers together",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"a": map[string]interface{}{"type": "number", "description": "First number"},
					"b": map[string]interface{}{"type": "number", "description": "Second number"},
				},
				"required": []string{"a", "b"},
			},
			Handler: func(ctx context.Context, args map[string]any) (any, error) {
				a, _ := args["a"].(float64)
				b, _ := args["b"].(float64)
				return map[string]any{
					"result":    a * b,
					"operation": "multiplication",
				}, nil
			},
		},
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create MCP server: %v", err))
	}

	// Configure Claude with the MCP server
	options := types.NewClaudeAgentOptions().
		WithModel("claude-sonnet-4-20250514").
		WithMcpServers(map[string]interface{}{
			"calculator": calculator,
		}).
		WithAllowedTools("mcp__calculator__add", "mcp__calculator__multiply").
		WithSystemPrompt(`You are a helpful calculator assistant. You have access to a calculator tool with two operations:
- add(a, b): adds two numbers
- multiply(a, b): multiplies two numbers

Use these tools to help the user with mathematical calculations.`)

	// Query Claude with a calculation request
	prompt := "What is 15 * 23? Also, what is the sum of 100 and 50?"

	fmt.Printf("User: %s\n\n", prompt)

	messages, err := sdk.Query(ctx, prompt, options)
	if err != nil {
		panic(fmt.Sprintf("query failed: %v", err))
	}

	// Process the response stream
	for msg := range messages {
		switch m := msg.(type) {
		case *types.AssistantMessage:
			// Process assistant response
			for _, block := range m.Content {
				switch c := block.(type) {
				case *types.TextBlock:
					fmt.Printf("Assistant: %s\n", c.Text)
				case *types.ToolUseBlock:
					fmt.Printf("🔧 Using tool: %s\n", c.Name)
					fmt.Printf("   Input: %v\n", c.Input)
				}
			}
		case *types.ResultMessage:
			// Final result with cost information
			fmt.Printf("\n📊 Result: %s\n", m.Type)
			if m.TotalCostUSD != nil {
				fmt.Printf("   Total cost: $%.4f\n", *m.TotalCostUSD)
			}
		}
	}
}
