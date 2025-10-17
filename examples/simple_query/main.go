package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
)

// SimpleQuery demonstrates the one-shot Query function.
// This is the simplest way to use the SDK - just send a prompt and get responses.
func main() {
	ctx := context.Background()

	// Create options with a simple model
	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest")

	// Simple query
	fmt.Println("Sending query: 'What is 2 + 2?'")
	fmt.Println("---")

	messages, err := claude.Query(ctx, "What is 2 + 2?", opts)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	// Process messages from the channel
	for msg := range messages {
		msgType := msg.GetMessageType()

		switch msgType {
		case "assistant":
			if assistantMsg, ok := msg.(*types.AssistantMessage); ok {
				for _, block := range assistantMsg.Content {
					if textBlock, ok := block.(*types.TextBlock); ok {
						fmt.Printf("Claude: %s\n", textBlock.Text)
					}
				}
			}
		case "result":
			fmt.Println("---")
			fmt.Println("Query completed successfully")
		}
	}
}
