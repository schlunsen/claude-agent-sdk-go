package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// InteractiveClient demonstrates the interactive Client for multi-turn conversations.
// This allows back-and-forth conversation with Claude while maintaining session state.
func main() {
	ctx := context.Background()

	// Create options for the interactive client
	opts := types.NewClaudeAgentOptions().
		WithModel("claude-3-5-sonnet-latest").
		WithPermissionMode(types.PermissionModeBypassPermissions)

	// Create client
	client, err := claude.NewClient(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Connect to Claude
	fmt.Println("Connecting to Claude...")
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		_ = client.Close(ctx)
	}()

	fmt.Println("Connected! Type your questions (press Ctrl+C to exit)")
	fmt.Println("---")

	// Interactive loop
	reader := bufio.NewReader(os.Stdin)
	for {
		// Read user input
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			log.Fatalf("Error reading input: %v", err)
		}

		prompt := strings.TrimSpace(input)
		if prompt == "" {
			continue
		}

		// Send query to Claude
		if err := client.Query(ctx, prompt); err != nil {
			fmt.Printf("Error sending query: %v\n", err)
			continue
		}

		// Receive and print responses
		foundResponse := false
		for msg := range client.ReceiveResponse(ctx) {
			msgType := msg.GetMessageType()

			switch msgType {
			case "assistant":
				if !foundResponse {
					fmt.Print("Claude: ")
					foundResponse = true
				}
				if assistantMsg, ok := msg.(*types.AssistantMessage); ok {
					for _, block := range assistantMsg.Content {
						if textBlock, ok := block.(*types.TextBlock); ok {
							fmt.Print(textBlock.Text)
						}
					}
				}
			case "result":
				fmt.Println()
				fmt.Println()
			}
		}
	}

	fmt.Println("\nGoodbye!")
}
