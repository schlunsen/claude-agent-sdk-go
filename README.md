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
- 📡 **MCP support** - Model Context Protocol servers (external and SDK-based) + **[NEW] Factory function** for simplified server creation (Issue #24)
- ⚡ **Streaming** - Full message streaming with partial outputs
- 🎯 **Idiomatic Go** - Uses goroutines, channels, and context for natural concurrency
- 📦 **Zero dependencies** - Core SDK uses only Go stdlib (except test examples)

### Comparison with Python SDK

Coming from Python? We have comprehensive guides:
- **[📊 Feature Parity Guide](docs/FEATURE_PARITY.md)** - See what's implemented and how it compares
- **[🏗️ Architecture Differences](docs/ARCHITECTURE.md)** - Understand async/await vs goroutines
- **[🔄 Migration Guide](docs/MIGRATION_FROM_PYTHON.md)** - Python → Go code examples
- **[📝 Side-by-side examples](docs/examples/python_vs_go/)** - Compare implementations

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
- **Authentication** (choose one method):
  - **API Key** (Pay-as-you-go): Set `CLAUDE_API_KEY` environment variable
  - **OAuth Token** (Max subscription): Set `CLAUDE_CODE_OAUTH_TOKEN` environment variable

## Authentication

The SDK supports two authentication methods depending on your Anthropic subscription plan:

### Method 1: API Key (Pay-as-you-go Plans)

For standard API usage with pay-as-you-go billing:

```bash
export CLAUDE_API_KEY=your-api-key-here
```

Get your API key from the [Anthropic Console](https://console.anthropic.com/).

### Method 2: OAuth Token (Max Subscription Plans)

For Anthropic Max subscription plans that include Claude Code:

1. **Setup OAuth token** with Claude Code CLI:
   ```bash
   claude setup-token
   ```
   This will open a browser window to authenticate and generate your OAuth token.

2. **Export the token** to your environment:
   ```bash
   export CLAUDE_CODE_OAUTH_TOKEN=your-oauth-token-here
   ```

3. **Use the SDK** - The Claude Code CLI will automatically use the OAuth token when available.

**Note**: OAuth tokens are tied to your Max subscription and provide access to Claude Code features. API keys are for direct API access with usage-based billing.

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

## Debugging and Stderr Logging

The SDK provides flexible options for capturing and logging stderr output from the Claude CLI subprocess.

### SDK-Managed File Logging (Simple)

For basic use cases, enable automatic file logging with SDK-managed file handling:

```go
// Use default location: ~/.claude/agents_server/cli_stderr.log
opts := NewClaudeAgentOptions().
	WithDefaultStderrLogFile()

// Or use custom path
opts := NewClaudeAgentOptions().
	WithCustomStderrLogFile("/tmp/claude-stderr.log")
```

The SDK will:
- Create parent directories if they don't exist
- Log all stderr output with timestamps
- Provide helpful error messages if logging fails
- Continue execution even if file logging fails

### Runtime Control with Stderr Callback (Advanced)

For runtime control over what gets logged, use the `Stderr` callback:

```go
type App struct {
	enableLogging atomic.Bool
	logFile       *os.File
}

func (app *App) setup() {
	opts := NewClaudeAgentOptions().
		WithStderr(func(line string) {
			// Full runtime control over stderr handling
			if app.enableLogging.Load() {
				fmt.Fprintf(app.logFile, "[stderr] %s\n", line)
			}

			// Can also filter, transform, or send to monitoring
			if strings.Contains(line, "ERROR") {
				app.sendAlert(line)
			}
		})
}

// Toggle logging at runtime
func (app *App) EnableLogging()  { app.enableLogging.Store(true) }
func (app *App) DisableLogging() { app.enableLogging.Store(false) }
```

### Combined Approach

You can use both SDK-managed file logging AND a custom callback:

```go
opts := NewClaudeAgentOptions().
	WithDefaultStderrLogFile().  // SDK logs to file
	WithStderr(func(line string) {
		// Your custom logic (monitoring, filtering, etc.)
		if strings.Contains(line, "CRITICAL") {
			alert.Send(line)
		}
	})
```

**Benefits:**
- **Simple things simple**: Just enable file logging for basic debugging
- **Complex things possible**: Use callbacks for filtering, monitoring, alerts
- **Non-intrusive**: Disabled by default, no files created unless requested
- **Helpful errors**: Clear messages if directory creation or file opening fails

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `CLAUDE_API_KEY` | Claude API key for pay-as-you-go plans (choose one auth method) |
| `CLAUDE_CODE_OAUTH_TOKEN` | OAuth token for Max subscription plans (choose one auth method) |
| `CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK` | Skip CLI version validation (dev only) |
| Custom variables | Passed to CLI process via `WithEnv()` |

**Authentication**: Set either `CLAUDE_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN`, not both. See [Authentication](#authentication) section for details.

**Note:** For stderr logging, use the options-based approach (`WithDefaultStderrLogFile()` or `WithStderr()`) instead of environment variables. See [Debugging and Stderr Logging](#debugging-and-stderr-logging).

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

- **`examples/with_rtk/main.go`** - Reduce token usage with the RTK CLI proxy (`middleware/rtk`)
  ```bash
  cd examples/with_rtk && go run main.go
  ```

## Middleware

Optional, opt-in packages under `middleware/` that implement common hook
patterns so you don't have to write them yourself.

### `middleware/rtk` — token-saving Bash wrapper

[RTK](https://github.com/rtk-ai/rtk) is a CLI proxy that compresses the
output of noisy commands (`git`, `npm`, `cargo`, `kubectl`, `pytest`, …)
by 60-90% before it reaches the LLM. The middleware installs a
`PreToolUse` hook that rewrites `git status` → `rtk git status`
transparently. The agent sees the compressed output; nothing else changes.

```go
import (
    "github.com/schlunsen/claude-agent-sdk-go/types"
    "github.com/schlunsen/claude-agent-sdk-go/middleware/rtk"
)

opts := types.NewClaudeAgentOptions().
    WithHook(types.HookEventPreToolUse, rtk.Hook(
        rtk.WithUltraCompact(true), // pass "-u" to rtk
        rtk.OnlyIfInstalled(),      // no-op if rtk is not on PATH
    ))
```

Only the `Bash` tool is affected; built-in `Read` / `Grep` / `Glob`
bypass the hook. See `middleware/rtk/doc.go` for all options
(`WithCommands`, `WithAddedCommands`, `WithBlocked`, `WithBinary`).

Surface compression stats in your UI or dashboard with the
`Gain` helper (wraps `rtk gain --format json`):

```go
export, err := rtk.Gain(ctx,
    rtk.WithAll(),                 // include daily/weekly/monthly breakdowns
    rtk.WithProject("/path/to/project"),
)
if errors.Is(err, rtk.ErrRTKNotInstalled) {
    // rtk not on PATH — fall back silently
} else if err != nil {
    log.Printf("rtk gain: %v", err)
} else {
    log.Printf("saved %d tokens (%.1f%%)",
        export.Summary.TotalSaved, export.Summary.AvgSavingsPct)
}
```

`rtk.TrackingDBPath()` returns the platform-specific location of
rtk's SQLite tracking database for callers who need per-invocation
rows (which `rtk gain` only exposes as aggregates).

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
