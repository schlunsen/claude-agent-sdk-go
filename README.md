# Claude Agent SDK for Go

[![Tests](https://github.com/schlunsen/claude-agent-sdk-go/workflows/Tests/badge.svg)](https://github.com/schlunsen/claude-agent-sdk-go/actions)
[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/schlunsen/claude-agent-sdk-go)](https://github.com/schlunsen/claude-agent-sdk-go/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/schlunsen/claude-agent-sdk-go)](https://goreportcard.com/report/github.com/schlunsen/claude-agent-sdk-go)

**Unofficial community port** of the [official Python SDK](https://github.com/anthropics/claude-agent-sdk-python)

⚠️ This is **not affiliated with or endorsed by Anthropic**. Use at your own risk.

A Go SDK for building multi-turn AI agent applications with Claude via the Claude Code CLI. Build agentic workflows, interact with tools, manage permissions, and stream responses with full control over execution.

## Features

- 🚀 **One-shot queries** - Simple `Query()` function for single interactions
- 🔄 **Interactive sessions** - Bidirectional control protocol with `Client` for complex workflows
- 🛠️ **Tool integration** - Permission callbacks and tool use controls
- 🎣 **Hook system** - Respond to lifecycle events (PreToolUse, PostToolUse, etc.)
- 📡 **MCP support** - Model Context Protocol servers (external and SDK-based)
- ⚡ **Streaming** - Full message streaming with partial outputs
- 🎯 **Idiomatic Go** - Uses goroutines, channels, and context for natural concurrency
- 📦 **Zero dependencies** - Core SDK uses only Go stdlib (except test examples)

## Status

✅ **Production Ready - v0.1.0 Released**

- [x] Phase 1: Foundation & Types (100%)
- [x] Phase 2: Transport Layer (100%)
- [x] Phase 3: Message Parsing (100%)
- [x] Phase 4: Control Protocol (100%)
- [x] Phase 5: Public API (100%)
- [x] Phase 6: Testing & Validation (100%)
- [x] Phase 7: Documentation & Examples (100%)
- [x] Phase 8: Polish & Release (100%)

**Code Statistics:**
- Production code: ~9,800 lines
- Test code: ~2,100 lines
- Examples: 4 working demonstrations
- Test coverage: 60%+ across all packages
- CI/CD: GitHub Actions (Go 1.24, 1.25)

## Quick Start

### Installation

```bash
go get github.com/schlunsen/claude-agent-sdk-go
```

### Basic Usage

#### One-Shot Query

```go
package main

import (
	"context"
	"fmt"

	sdk "github.com/schlunsen/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	// Simple query
	messages, err := sdk.Query(ctx, "What is 2 + 2?", nil)
	if err != nil {
		panic(err)
	}

	for msg := range messages {
		fmt.Println(msg)
	}
}
```

#### Interactive Client

```go
package main

import (
	"context"
	"fmt"

	sdk "github.com/schlunsen/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	options := sdk.NewClaudeAgentOptions().
		WithModel("claude-opus-4-20250514").
		WithAllowedTools("Bash", "Write", "Read")

	client, err := sdk.NewClient(options)
	if err != nil {
		panic(err)
	}

	// Connect and start session
	if err := client.Connect(ctx); err != nil {
		panic(err)
	}
	defer client.Close(ctx)

	// Send query
	if err := client.Query(ctx, "List the files in the current directory"); err != nil {
		panic(err)
	}

	// Receive streaming responses
	for msg := range client.ReceiveResponse(ctx) {
		fmt.Println(msg)
	}
}
```

#### With Permission Callbacks

```go
package main

import (
	"context"
	"fmt"

	sdk "github.com/schlunsen/claude-agent-sdk-go"
)

func main() {
	ctx := context.Background()

	options := sdk.NewClaudeAgentOptions().
		WithModel("claude-opus-4-20250514").
		WithAllowedTools("Bash", "Write").
		WithPermissionCallback(func(ctx context.Context, toolName string, input interface{}) (bool, error) {
			// Approve or deny tool usage
			fmt.Printf("Tool %s requested. Allow? (y/n): ", toolName)
			// ... prompt user or implement custom logic
			return true, nil
		})

	// Use with client or query
	messages, err := sdk.Query(ctx, "Delete all files in /tmp", options)
	if err != nil {
		panic(err)
	}

	for msg := range messages {
		fmt.Println(msg)
	}
}
```

## Requirements

- **Go 1.24+**
- **Claude Code CLI** installed globally:
  ```bash
  npm install -g @anthropic-ai/claude-code
  ```
- **Valid Claude API key** (via `CLAUDE_API_KEY` environment variable)

## Architecture

The SDK is organized into logical layers:

```
User Application
    ↓
Public API (Query, Client)
    ↓
Internal Orchestration
    ↓
Query (Control Protocol)
    ↓
Message Parser
    ↓
Transport (Abstract)
    ↓
SubprocessCLITransport
    ↓
Claude Code CLI (Node.js)
```

### Key Layers

- **Transport**: Manages subprocess communication and JSON lines protocol
- **Query**: Implements bidirectional control protocol (permissions, hooks, MCP)
- **Message Parser**: Converts JSON to Go types
- **Public API**: User-facing `Query()` function and `Client` type

## API Reference

### Query Function (One-Shot)

```go
func Query(
	ctx context.Context,
	prompt string,
	options *ClaudeAgentOptions,
) (<-chan Message, error)
```

Executes a single query and streams responses as a channel of `Message`.

**Example:**
```go
messages, err := Query(ctx, "What's the weather?", nil)
for msg := range messages {
	// Process each message
}
```

### Client Type (Interactive)

```go
type Client interface {
	Connect(ctx context.Context) error
	Query(ctx context.Context, prompt string) error
	ReceiveResponse(ctx context.Context) <-chan Message
	Close(ctx context.Context) error
}
```

**Lifecycle:**
1. `Connect()` - Establish session
2. `Query()` - Send prompt (repeatable)
3. `ReceiveResponse()` - Get streaming responses
4. `Close()` - Cleanup

### Options Builder

```go
options := NewClaudeAgentOptions().
	WithModel("claude-opus-4-20250514").
	WithAllowedTools("Bash", "Write", "Read").
	WithSystemPrompt("You are a helpful assistant.").
	WithPermissionCallback(func(ctx context.Context, tool string, input interface{}) (bool, error) {
		// Custom permission logic
		return true, nil
	}).
	WithHook("PreToolUse", func(ctx context.Context, event interface{}) (HookDecision, error) {
		// Pre-tool-use hook
		return HookAllow, nil
	})
```

### Message Types

All responses from Claude are `Message` types:

```go
type Message interface {
	Type() string
	// UserMessage, AssistantMessage, SystemMessage, ResultMessage, etc.
}
```

**Message Content:**
```go
type ContentBlock interface {
	// TextBlock, ToolUseBlock, ToolResultBlock, etc.
}
```

## Control Protocol

The SDK uses a bidirectional control protocol to handle:

### 1. Tool Permissions

When Claude attempts to use a tool, the SDK can intercept and make a decision:

```go
WithPermissionCallback(func(ctx context.Context, toolName string, input interface{}) (bool, error) {
	if toolName == "Bash" && isRiskyCommand(input) {
		return false, nil  // Deny
	}
	return true, nil  // Allow
})
```

### 2. Hooks

Respond to lifecycle events:

```go
WithHook("PreToolUse", func(ctx context.Context, event interface{}) (HookDecision, error) {
	// Called before each tool use
	// Return: HookAllow, HookDeny, or HookBlock
	return HookAllow, nil
})

WithHook("PostToolUse", func(ctx context.Context, event interface{}) (HookDecision, error) {
	// Called after tool completes
	return HookAllow, nil
})
```

### 3. MCP Servers

Define custom tools via SDK MCP servers:

```go
// TODO: Implement custom MCP server support
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `CLAUDE_API_KEY` | Claude API key (required) |
| `CLAUDE_AGENT_VERBOSE` | Enable verbose debug logging to file at `~/.claude/agents_server/cli_stderr.log` |
| `CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK` | Skip CLI version validation (dev only) |
| Custom variables | Passed to CLI process via `WithEnv()` |

## Error Handling

The SDK provides typed errors for better handling:

```go
import "errors"
import "github.com/schlunsen/claude-agent-sdk-go/types"

messages, err := Query(ctx, "...", nil)
if err != nil {
	switch {
	case types.IsCLINotFoundError(err):
		fmt.Println("Claude Code CLI not installed")
	case types.IsCLIConnectionError(err):
		fmt.Println("Failed to connect to CLI")
	default:
		fmt.Printf("Error: %v\n", err)
	}
}
```

## Comparison with Python SDK

| Feature | Python | Go |
|---------|--------|-----|
| One-shot queries | ✅ | ✅ |
| Interactive client | ✅ | ✅ |
| Tool permissions | ✅ | ✅ |
| Hook system | ✅ | ✅ |
| MCP servers | ✅ | ✅ |
| Streaming | ✅ | ✅ |
| CLI discovery | ✅ | ✅ |
| Error types | ✅ | ✅ |

**Key Differences:**
- **Concurrency**: Go uses channels + goroutines instead of async/await
- **Context**: All operations require explicit `context.Context`
- **Builder pattern**: Go uses fluent API for options (vs Python's dataclass)
- **Message iteration**: Channels instead of async generators

## Examples

See `examples/` directory for complete, runnable examples:

- **`examples/simple_query/main.go`** - Basic one-shot query
  ```bash
  cd examples/simple_query && go run main.go
  ```

- **`examples/interactive_client/main.go`** - Multi-turn conversation with prompt
  ```bash
  cd examples/interactive_client && go run main.go
  ```

- **`examples/with_permissions/main.go`** - Tool permission callbacks for safety
  ```bash
  cd examples/with_permissions && go run main.go
  ```

- **`examples/with_hooks/main.go`** - Hook lifecycle events (PreToolUse, PostToolUse)
  ```bash
  cd examples/with_hooks && go run main.go
  ```

## Development

### Prerequisites

```bash
go 1.24+
```

### Build

```bash
make build
```

### Run Tests

```bash
# Run unit tests only (recommended for development)
make test

# Run ALL tests including integration tests (spawns Claude processes)
make test-all

# Run only integration tests (requires CLAUDE_API_KEY)
make test-integration
```

**Note:** By default, `make test` runs in short mode and skips integration tests to avoid spawning Claude CLI processes. Use `make test-all` only when you explicitly want to test against the real Claude CLI.

### Lint & Format

```bash
make lint
make fmt
```

## Known Limitations

- No automatic CLI version updates
- Limited Windows support
- No gRPC transport alternative

## Contributing

Contributions welcome! Please note this is an unofficial port. If you find issues or want to contribute:

1. Fork the repo
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a PR

## License

This project is licensed under the same license as the [official Python SDK](https://github.com/anthropics/claude-agent-sdk-python). See `LICENSE` file.

## Disclaimer

⚠️ **This is an unofficial community port** and is not affiliated with Anthropic. Use at your own risk.

- Always review code before granting tool permissions
- Be cautious with sensitive operations (file deletion, network access, etc.)
- Test thoroughly in development environments first
- The Go port may have different behavior than the Python SDK

## Support

For issues with:

- **This Go SDK**: Open an issue on [GitHub](https://github.com/schlunsen/claude-agent-sdk-go/issues)
- **Claude Code CLI**: See [official docs](https://claude.com)
- **Claude API**: Contact [Anthropic support](https://support.anthropic.com)

## Resources

- [Official Python SDK](https://github.com/anthropics/claude-agent-sdk-python)
- [Claude Code Documentation](https://claude.com/docs)
- [Claude API Documentation](https://docs.anthropic.com)

---

**Status**: ✅ Production Ready - v0.1.0 | **Go Version**: 1.24+ | **Last Updated**: October 2025
