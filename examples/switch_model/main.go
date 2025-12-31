package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// SwitchModelExample demonstrates switching models during a conversation
// using the Client.SetModel() method to change models mid-session.
func main() {
	ctx := context.Background()

	fmt.Println("=== Switch Model During Conversation Example ===")
	fmt.Println()

	// Create client with initial model (haiku - fast and cheap)
	opts := types.NewClaudeAgentOptions().
		WithModel("haiku").
		WithMaxTurns(1)

	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func() { _ = client.Close(ctx) }()

	// Connect to Claude
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// First query: Use haiku (fast, cheap) for simple question
	fmt.Println("--- Turn 1: Using haiku model ---")
	if err := client.Query(ctx, "What is the capital of France? Answer in one word."); err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*types.AssistantMessage); ok {
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					fmt.Printf("Claude (haiku): %s\n", textBlock.Text)
				}
			}
		}
	}
	fmt.Println()

	// Switch to sonnet model for more complex task
	fmt.Println("--- Turn 2: Switching to sonnet model ---")
	if err := client.SetModel(ctx, "sonnet"); err != nil {
		log.Fatalf("Failed to switch model: %v", err)
	}

	if err := client.Query(ctx, "Now tell me 3 interesting facts about that city."); err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*types.AssistantMessage); ok {
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					fmt.Printf("Claude (sonnet): %s\n", textBlock.Text)
				}
			}
		}
	}
	fmt.Println()

	// Switch to opus model for summarization
	fmt.Println("--- Turn 3: Switching to opus model ---")
	if err := client.SetModel(ctx, "opus"); err != nil {
		log.Fatalf("Failed to switch model: %v", err)
	}

	if err := client.Query(ctx, "Summarize our conversation so far."); err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	for msg := range client.ReceiveResponse(ctx) {
		if m, ok := msg.(*types.AssistantMessage); ok {
			for _, block := range m.Content {
				if textBlock, ok := block.(*types.TextBlock); ok {
					fmt.Printf("Claude (opus): %s\n", textBlock.Text)
				}
			}
		}
	}
	fmt.Println()

	fmt.Println("=== Conversation Complete ===")
}
