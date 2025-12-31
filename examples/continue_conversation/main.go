package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// ContinueConversationExample demonstrates continuing a previous conversation.
func main() {
	ctx := context.Background()

	fmt.Println("=== Continue Conversation Example ===")
	fmt.Println("Using --continue to resume the last conversation")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithContinueConversation(true).
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "What were we just talking about?", opts)
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
