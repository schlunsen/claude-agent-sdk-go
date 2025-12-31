package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// FallbackModelExample demonstrates using a fallback model when the primary model is unavailable.
func main() {
	ctx := context.Background()

	fmt.Println("=== Fallback Model Example ===")
	fmt.Println("Setting model='opus' with fallback_model='sonnet'")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithModel("opus").
		WithFallbackModel("sonnet").
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "What is 2 + 2?", opts)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for msg := range messages {
		switch m := msg.(type) {
		case *types.AssistantMessage:
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					fmt.Printf("Claude: %s\n", textBlock.Text)
				}
			}
		case *types.ResultMessage:
			if m.TotalCostUSD != nil {
				fmt.Printf("\nCost: $%.4f\n", *m.TotalCostUSD)
			}
		}
	}
}
