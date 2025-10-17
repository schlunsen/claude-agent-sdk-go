# Claude Agent SDK for Go - Development Guide

## Quick Start

### Prerequisites
- Go 1.20+
- Claude Code CLI installed: `npm install -g @anthropic-ai/claude-code`
- Valid `CLAUDE_API_KEY` environment variable

### Build & Test

```bash
# Format code
make fmt

# Run linters
make lint

# Build the SDK
make build

# Run all tests
make test

# Generate coverage report
make coverage

# Clean build artifacts
make clean

# Build and verify examples
make examples
```

## Project Structure

```
claude-agent-sdk-go/
├── README.md                    # User-facing documentation
├── CLAUDE.md                    # This file - dev guidelines
├── GO_PORT_PLAN.md             # Detailed implementation plan
├── go.mod                       # Module definition
├── Makefile                     # Build targets
├── LICENSE                      # MIT License
├── .gitignore                   # Git ignore rules
├── types/                       # Public type definitions (exported)
│   ├── messages.go              # Message types and content blocks
│   ├── messages_test.go         # Message type tests
│   ├── control.go               # Control protocol types
│   ├── control_test.go          # Control protocol tests
│   ├── options.go               # ClaudeAgentOptions builder
│   ├── errors.go                # Error definitions
│   ├── errors_test.go           # Error tests
│   └── doc.go                   # Package documentation
├── internal/                    # Private packages (not exported)
│   ├── transport/               # Transport abstraction
│   │   ├── transport.go         # Transport interface
│   │   └── subprocess_cli.go    # CLI subprocess implementation
│   ├── message_parser.go        # JSON parsing
│   ├── query.go                 # Control protocol handler
│   └── client.go                # Internal client orchestration
├── client.go                    # Public Client type
├── query.go                     # Public Query function
├── doc.go                       # Package documentation
├── examples/                    # Runnable examples
│   ├── simple_query/main.go
│   ├── interactive_client/main.go
│   ├── with_permissions/main.go
│   └── with_hooks/main.go
└── tests/                       # Test files
    ├── *_test.go
    └── testdata/
```

## Reference Implementation

The Python SDK source is available at `/Users/schlunsen/projects/claude-agent-sdk-python` for reference:
- `src/claude_agent_sdk/` - Main implementation
- `src/claude_agent_sdk/_internal/` - Internal modules
- `tests/` - Test patterns and fixtures

Use this as your reference for behavior, message formats, and edge cases while implementing the Go port.

## Development Workflow

### 1. Before Making Changes

Always work on a feature branch, never directly on `main`:

```bash
# Create and switch to feature branch
git checkout -b phase-1/error-types
git checkout -b phase-2/transport-layer
git checkout -b fix/json-parsing
git checkout -b chore/update-makefile
```

Branch naming:
- Features: `phase-N/description` (e.g., `phase-1/error-types`)
- Fixes: `fix/description` (e.g., `fix/goroutine-leak`)
- Chores: `chore/description` (e.g., `chore/update-deps`)

### 2. Making Changes

- Keep changes focused and atomic
- Follow Go style guidelines (see Coding Standards below)
- Add tests for new functionality
- Reference the Python SDK for behavior
- Implement in phase order (types → transport → parser → protocol → API)

### 3. Testing Locally

Before pushing, run the full test suite:

```bash
# Format code
make fmt

# Check for issues
make lint

# Build
make build

# Run tests
make test

# Run with coverage
make coverage
```

All checks should pass before pushing.

### 4. Committing Changes

Keep commits small and focused - one clear change per commit:

```bash
# Good - multiple small commits
git commit -m "Phase 1: Add error types"
git commit -m "Phase 1: Add error wrapping helpers"
git commit -m "Phase 1: Add unit tests for errors"

# Bad - huge commit mixing concerns
git commit -m "Phase 1: Add error types, message parsing, and transport"
```

Commit message format - reference the phase being worked on:

```
Phase 1: Add error types and helpers

- Implement CLINotFoundError, CLIConnectionError, ProcessError
- Add error wrapping with errors.Is() support
- Add test coverage for error creation
```

```
Phase 2: Implement subprocess transport layer

- Add Transport interface abstraction
- Implement SubprocessCLITransport with CLI discovery
- Add JSON lines reading/writing with buffering
- Handle process lifecycle and stream cleanup
```

### 5. Push Frequently

Push early and often - don't wait until everything is "complete":

```bash
# After every 2-3 commits
git push -u origin phase-1/error-types

# Subsequent pushes
git push
```

This ensures:
- Your work is backed up on GitHub
- Progress is visible to others
- You get feedback early

### 6. Create a Pull Request

When a feature, fix, or chore is done:

1. **Ensure all checks pass**:
   ```bash
   make fmt
   make lint
   make build
   make test
   ```

2. **Push to GitHub**:
   ```bash
   git push origin phase-1/error-types
   ```

3. **Create PR with clear description**:
   - Title: Match commit message style
   - Description: Explain what, why, and how
   - Reference the implementation phase
   - Link related issues if any

4. **Example PR**:
   ```
   Title: Phase 1: Add error types and helpers

   ## Description
   Implements all error types defined in the implementation plan with proper
   error wrapping and typed checking via errors.Is().

   ## Changes
   - Add CLINotFoundError, CLIConnectionError, ProcessError types
   - Implement error wrapping with context preservation
   - Add error constructor functions
   - Add comprehensive unit tests (>90% coverage)

   ## Testing
   - All new tests pass
   - Existing tests still pass
   - Ready for Phase 2 which depends on these types
   ```

5. **Request review and address feedback** with additional commits to the same branch

6. **Merge when approved**

## Coding Standards

### Naming Conventions
- **Packages**: lowercase, short (`types`, `transport`)
- **Types**: PascalCase (`Message`, `Client`, `ClaudeAgentOptions`)
- **Functions**: camelCase (`query`, `connect`, `receiveResponse`)
- **Constants**: UPPER_SNAKE_CASE (`DefaultTimeout`, `MaxBufferSize`)
- **Private**: lowercase starting letter (`newTransport`, `parseMessage`)

### Comments

Every exported type and function needs a comment:

```go
// Query executes a single prompt and streams responses.
// Returns a channel of messages that can be read until closed.
func Query(ctx context.Context, prompt string, options *ClaudeAgentOptions) (<-chan Message, error)

// Message represents a response from Claude.
type Message interface {
    Type() string
}
```

### Error Messages

Be specific and helpful:

```go
// Bad
return fmt.Errorf("error")

// Good
return fmt.Errorf("failed to find Claude Code CLI: searched PATH and common install locations")
```

### Code Organization

**Public APIs** (exported from root):
- `func Query()` in `query.go`
- `type Client` in `client.go`
- `type ClaudeAgentOptions` builder in `options.go`

**Internal Implementation** (in `internal/`):
- Never export internal packages
- Put implementation details here
- Examples: transport, message parser, control protocol

**Interfaces**:
- Define interfaces where needed for abstraction
- Use `interface{}` sparingly - prefer explicit types
- Example: `type Transport interface { ... }`

### JSON Marshaling

For message types with unions, implement custom marshaling:

```go
// Use struct tags for simple types
type TextBlock struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

// Use MarshalJSON for complex types (unions, discriminators)
func (m Message) MarshalJSON() ([]byte, error) {
    // Custom marshaling logic
}
```

### Context Usage

All I/O operations must accept `context.Context`:

```go
// Good
func (t *Transport) Write(ctx context.Context, data string) error
func (c *Client) Connect(ctx context.Context) error

// Bad - no context
func (t *Transport) Write(data string) error
```

This enables:
- Cancellation: `ctx.Done()`
- Timeouts: `context.WithTimeout()`
- Deadlines: `context.WithDeadline()`

### Goroutine Management

When spawning goroutines, tie them to context cancellation:

```go
// Good - respects context cancellation
go func() {
    select {
    case <-ctx.Done():
        return
    case msg := <-messages:
        // Process message
    }
}()

// Bad - goroutine leak if ctx is cancelled
go func() {
    for msg := range messages {
        // No way to stop this
    }
}()
```

### Concurrency Patterns

**Channel for streaming:**
```go
messages := make(chan Message)
go readLoop(ctx, messages)
for msg := range messages {
    // Process
}
```

**Mutex for shared state:**
```go
type Query struct {
    mu sync.Mutex
    requestMap map[string]ControlRequest
}
```

## Testing Guidelines

### Unit Tests
- Test single functions/methods
- Use table-driven tests for multiple cases
- Mock external dependencies

### Integration Tests
- Test end-to-end flows
- Use fixtures from Python SDK
- Test happy path and error cases

### Coverage Target
- Aim for >80% code coverage
- Focus on critical paths and error handling
- Don't obsess over 100%

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test -v ./types/...

# Run with coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

### Test Files

Place tests next to the code they test:

```
types/
├── errors.go
├── errors_test.go        # Tests for errors.go
├── messages.go
└── messages_test.go      # Tests for messages.go
```

## Common Tasks

### Add a New Message Type

1. Define struct in `types/messages.go`
2. Add JSON tags for marshaling
3. Add tests in `types/messages_test.go` or appropriate `*_test.go` file
4. Update docs if user-facing

### Add a New Hook

1. Define hook type in `types/control.go`
2. Implement routing in `internal/query.go`
3. Add to options builder in `types/options.go`
4. Test with integration test

### Add a New Example

1. Create `examples/my_example/main.go`
2. Include `package main` with runnable code
3. Add to README's examples section
4. Verify it builds: `make examples`

## Debugging

### Enable Debug Logging

Print debug info during development:

```go
log.Printf("DEBUG: %+v", value)
```

### Use Go Debugger

Install Delve:
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Run tests with debugger:
```bash
dlv test ./types -- -test.run TestName
```

### Inspect Subprocess Communication

Set environment variable to see verbose output:
```bash
CLAUDE_CODE_VERBOSE=1 go test -v ./...
```

### Use Go Profiler

```bash
# CPU profile
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

## Performance Considerations

- **Subprocess startup**: ~200-500ms (mostly Node.js startup)
- **Message parsing**: JSON parsing with bufio is fast
- **Goroutine overhead**: Minimal for message reading loop
- **Memory**: Should stay <50MB for typical sessions

Benchmark critical paths:
```bash
go test -bench=. -benchmem ./...
```

## Git Workflow Summary

1. **Create feature branch**: `git checkout -b phase-1/error-types`
2. **Make changes** with tests
3. **Commit frequently**: Small, focused commits
4. **Push often**: After every 2-3 commits
5. **Create PR** when feature is complete
6. **Address review** feedback with additional commits
7. **Merge** and delete branch

Key commands:
```bash
# Create and switch to branch
git checkout -b phase-1/error-types

# See status
git status

# Stage changes
git add types/

# Commit
git commit -m "Phase 1: Add error types"

# Push (first time)
git push -u origin phase-1/error-types

# Push (subsequent)
git push

# Create PR on GitHub
# (Visit GitHub web UI and create PR from your branch)
```

## Dependencies

Core dependencies (none for now - using Go stdlib):
- `encoding/json` (stdlib)
- `os` and `os/exec` (stdlib)
- `io` and `bufio` (stdlib)
- `context` (stdlib)
- `errors` (stdlib, Go 1.20+)

Keep it stdlib-only to avoid bloating the SDK with dependencies.

## Troubleshooting

### "Claude Code CLI not found"

```bash
# Install CLI
npm install -g @anthropic-ai/claude-code

# Or set custom path
export CLAUDE_CLI_PATH=/path/to/claude
```

### Tests hang or timeout
- Check context cancellation
- Look for goroutine leaks with `go.uber.org/goleak`
- Ensure mock subprocess cleanup
- Use timeout: `go test -timeout 30s ./...`

### Build fails on different OS
- Use `runtime.GOOS` for OS-specific code
- Test on Linux, macOS, Windows if possible
- Avoid hardcoded paths (use `filepath.Join`)
- Use cross-compilation: `GOOS=linux GOARCH=amd64 go build ./...`

### Linter issues
```bash
# See what golangci-lint is complaining about
golangci-lint run ./...

# Auto-fix some issues
gofmt -w ./internal/
```

## References

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go)
- Python SDK: `/Users/schlunsen/projects/claude-agent-sdk-python/src/claude_agent_sdk/`
- Implementation plan details: `GO_PORT_PLAN.md`

---

**Status**: 🚧 In Development | **Go Version**: 1.20+ | **Last Updated**: October 2024
