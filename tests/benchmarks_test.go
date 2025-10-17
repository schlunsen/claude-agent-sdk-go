package tests

import (
	"context"
	"testing"
	"time"

	claude "github.com/schlunsen/claude-agent-sdk-go"
	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
)

// BenchmarkQuery_SimpleMessage benchmarks a simple Query call.
func BenchmarkQuery_SimpleMessage(b *testing.B) {
	ctx := context.Background()

	// Create mock CLI once for all iterations
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Benchmark response"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(&testing.T{}, messages)
	if err != nil {
		b.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msgChan, err := claude.Query(ctx, "benchmark test", opts)
		if err != nil {
			b.Fatalf("Query() failed: %v", err)
		}

		// Drain channel
		for range msgChan {
		}
	}
}

// BenchmarkClient_QueryCycle benchmarks a complete client query cycle.
func BenchmarkClient_QueryCycle(b *testing.B) {
	ctx := context.Background()

	// Create mock CLI
	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Response"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(&testing.T{}, messages)
	if err != nil {
		b.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	opts := types.NewClaudeAgentOptions().
		WithCLIPath(mockCLI.Path).
		WithPermissionMode(types.PermissionModeBypassPermissions)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client, err := claude.NewClient(ctx, opts)
		if err != nil {
			b.Fatalf("NewClient() failed: %v", err)
		}

		if err := client.Connect(ctx); err != nil {
			_ = client.Close(ctx)
			continue // Connection might fail with mock CLI
		}

		if err := client.Query(ctx, "test"); err != nil {
			_ = client.Close(ctx)
			continue
		}

		// Drain messages
		for range client.ReceiveResponse(ctx) {
		}

		_ = client.Close(ctx)
	}
}

// BenchmarkMessageParsing benchmarks message parsing.
func BenchmarkMessageParsing(b *testing.B) {
	// Sample message data
	testMessages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-3"}`,
		`{"type":"user","content":"User message"}`,
		`{"type":"system","subtype":"info","data":{"message":"Info"}}`,
		`{"type":"result","output":"success"}`,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, msgJSON := range testMessages {
			_, err := types.UnmarshalMessage([]byte(msgJSON))
			if err != nil {
				b.Fatalf("UnmarshalMessage() failed: %v", err)
			}
		}
	}
}

// BenchmarkMessageParsing_Parallel benchmarks parallel message parsing.
func BenchmarkMessageParsing_Parallel(b *testing.B) {
	testMessage := `{"type":"assistant","content":[{"type":"text","text":"Hello world"}],"model":"claude-3"}`

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := types.UnmarshalMessage([]byte(testMessage))
			if err != nil {
				b.Fatalf("UnmarshalMessage() failed: %v", err)
			}
		}
	})
}

// BenchmarkContentBlockParsing benchmarks content block parsing.
func BenchmarkContentBlockParsing(b *testing.B) {
	testBlocks := []string{
		`{"type":"text","text":"Sample text"}`,
		`{"type":"tool_use","id":"1","name":"Bash","input":{"command":"ls"}}`,
		`{"type":"tool_result","tool_use_id":"1","content":"output"}`,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, blockJSON := range testBlocks {
			_, err := types.UnmarshalContentBlock([]byte(blockJSON))
			if err != nil {
				b.Fatalf("UnmarshalContentBlock() failed: %v", err)
			}
		}
	}
}

// BenchmarkOptionsBuilder benchmarks options builder pattern.
func BenchmarkOptionsBuilder(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		opts := types.NewClaudeAgentOptions().
			WithModel("claude-3-5-sonnet-latest").
			WithMaxTurns(10).
			WithEnvVar("KEY1", "value1").
			WithEnvVar("KEY2", "value2").
			WithEnvVar("KEY3", "value3").
			WithPermissionMode(types.PermissionModeBypassPermissions)

		_ = opts
	}
}

// BenchmarkClient_Create benchmarks client creation.
func BenchmarkClient_Create(b *testing.B) {
	ctx := context.Background()

	// Use echo as a simple CLI
	opts := types.NewClaudeAgentOptions().WithCLIPath("/bin/echo")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client, err := claude.NewClient(ctx, opts)
		if err == nil {
			_ = client.Close(ctx)
		}
	}
}

// BenchmarkClient_Connect benchmarks client connection.
func BenchmarkClient_Connect(b *testing.B) {
	ctx := context.Background()

	// Create mock CLI
	mockCLI, err := CreateMockCLI(&testing.T{}, "echo")
	if err != nil {
		b.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		client, err := claude.NewClient(ctx, opts)
		if err != nil {
			continue
		}

		if err := client.Connect(ctx); err != nil {
			_ = client.Close(ctx)
			continue
		}
		_ = client.Close(ctx)
	}
}

// BenchmarkContextCreation benchmarks context creation overhead.
func BenchmarkContextCreation(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cancel()
		_ = ctx
	}
}

// BenchmarkChannelOperations benchmarks channel send/receive.
func BenchmarkChannelOperations(b *testing.B) {
	ch := make(chan types.Message, 100)

	// Sample message
	result := "test"
	msg := &types.ResultMessage{
		Type:   "result",
		Result: &result,
	}

	b.ResetTimer()

	// Producer
	go func() {
		for i := 0; i < b.N; i++ {
			ch <- msg
		}
		close(ch)
	}()

	// Consumer
	count := 0
	for range ch {
		count++
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns.
func BenchmarkMemoryAllocation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate typical allocation pattern
		blocks := make([]types.ContentBlock, 0, 10)
		for j := 0; j < 10; j++ {
			block := &types.TextBlock{
				Type: "text",
				Text: "Sample text content",
			}
			blocks = append(blocks, block)
		}
		_ = blocks
	}
}

// BenchmarkQuery_WithAllocation tracks allocations for Query.
func BenchmarkQuery_WithAllocation(b *testing.B) {
	ctx := context.Background()

	messages := []string{
		`{"type":"assistant","content":[{"type":"text","text":"Response"}],"model":"claude-3"}`,
		`{"type":"result","output":"done"}`,
	}

	mockCLI, err := CreateMockCLIWithMessages(&testing.T{}, messages)
	if err != nil {
		b.Fatalf("Failed to create mock CLI: %v", err)
	}
	defer mockCLI.Cleanup()

	opts := types.NewClaudeAgentOptions().WithCLIPath(mockCLI.Path)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		msgChan, err := claude.Query(ctx, "test", opts)
		if err != nil {
			b.Fatalf("Query() failed: %v", err)
		}

		for range msgChan {
		}
	}
}

// BenchmarkParseJSON benchmarks raw JSON parsing.
func BenchmarkParseJSON(b *testing.B) {
	jsonData := []byte(`{"type":"assistant","content":[{"type":"text","text":"Hello world, this is a test message"}],"model":"claude-3-5-sonnet-latest","id":"msg_123","role":"assistant"}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := types.UnmarshalMessage(jsonData)
		if err != nil {
			b.Fatalf("UnmarshalMessage() failed: %v", err)
		}
	}
}

// BenchmarkErrorCreation benchmarks error creation.
func BenchmarkErrorCreation(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := types.NewCLINotFoundError("CLI not found in PATH")
		_ = err
	}
}

// BenchmarkErrorWrapping benchmarks error wrapping.
func BenchmarkErrorWrapping(b *testing.B) {
	baseErr := types.NewCLINotFoundError("base error")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := types.NewCLIConnectionErrorWithCause("connection failed", baseErr)
		_ = err
	}
}
