package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// AddDirsExample demonstrates adding additional directories to Claude's context.
func main() {
	ctx := context.Background()

	fmt.Println("=== Add Directories Example ===")
	fmt.Println("Adding extra directories for Claude to access")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithAddDirs("../other-project", "/tmp/shared-resources").
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "List the directories you have access to.", opts)
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
