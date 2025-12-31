package main

import (
	"context"
	"fmt"
	"log"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// toolsArrayExample demonstrates using tools as an array of specific tool names.
func toolsArrayExample(ctx context.Context) {
	fmt.Println("=== Tools Array Example ===")
	fmt.Println("Setting tools=['Read', 'Glob', 'Grep']")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithToolsList("Read", "Glob", "Grep").
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "What tools do you have available? Just list them briefly.", opts)
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	for msg := range messages {
		switch m := msg.(type) {
		case *types.SystemMessage:
			if m.Subtype == "init" {
				if tools, ok := m.Data["tools"].([]any); ok {
					fmt.Printf("Tools from system message: %v\n\n", tools)
				}
			}
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
	fmt.Println()
}

// toolsEmptyArrayExample demonstrates using an empty tools array to disable all tools.
func toolsEmptyArrayExample(ctx context.Context) {
	fmt.Println("=== Tools Empty Array Example ===")
	fmt.Println("Setting tools=[] (disables all built-in tools)")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithNoTools().
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "What tools do you have available? Just list them briefly.", opts)
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	for msg := range messages {
		switch m := msg.(type) {
		case *types.SystemMessage:
			if m.Subtype == "init" {
				if tools, ok := m.Data["tools"].([]any); ok {
					fmt.Printf("Tools from system message: %v\n\n", tools)
				}
			}
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
	fmt.Println()
}

// toolsPresetExample demonstrates using the tools preset for all default Claude Code tools.
func toolsPresetExample(ctx context.Context) {
	fmt.Println("=== Tools Preset Example ===")
	fmt.Println("Setting tools={'type': 'preset', 'preset': 'claude_code'}")
	fmt.Println()

	opts := types.NewClaudeAgentOptions().
		WithToolsPreset(types.ToolsPreset{Type: "preset", Preset: "claude_code"}).
		WithMaxTurns(1)

	messages, err := claude.Query(ctx, "What tools do you have available? Just list them briefly.", opts)
	if err != nil {
		log.Printf("Query failed: %v", err)
		return
	}

	for msg := range messages {
		switch m := msg.(type) {
		case *types.SystemMessage:
			if m.Subtype == "init" {
				if tools, ok := m.Data["tools"].([]any); ok {
					fmt.Printf("Tools from system message (%d tools): %v...\n\n", len(tools), tools[:min(5, len(tools))])
				}
			}
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
	fmt.Println()
}

func main() {
	ctx := context.Background()

	toolsArrayExample(ctx)
	toolsEmptyArrayExample(ctx)
	toolsPresetExample(ctx)
}
