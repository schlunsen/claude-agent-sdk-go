# Test Suite Documentation

This directory contains comprehensive integration tests, benchmarks, and test utilities for the Claude Agent SDK Go port.

## Test Files

### integration_test.go
End-to-end integration tests that verify the full stack works correctly.

**Query Integration Tests:**
- `TestQueryIntegration_SimplePrompt` - Full end-to-end query with mock CLI
- `TestQueryIntegration_WithOptions` - Query with all options configured
- `TestQueryIntegration_ErrorHandling` - Error scenarios (empty prompt, invalid CLI path)
- `TestQueryIntegration_ContextCancellation` - Cancellation mid-stream

**Client Integration Tests:**
- `TestClientIntegration_FullSession` - Complete client workflow (connect, query, receive, close)
- `TestClientIntegration_MultipleQueries` - Multiple query/response cycles
- `TestClientIntegration_WithPermissions` - Permission callback integration
- `TestClientIntegration_WithHooks` - Hook callback integration

**Protocol Tests:**
- `TestControlProtocol_FullFlow` - Permission + hook + MCP flow (requires real CLI)
- `TestStreamingWithControlMessages` - Mixed normal and control messages
- `TestRealCLIIntegration` - Integration with actual Claude CLI (requires API key)

### benchmarks_test.go
Performance benchmarks for critical paths.

**Query Benchmarks:**
- `BenchmarkQuery_SimpleMessage` - Basic query performance
- `BenchmarkQuery_WithAllocation` - Query with allocation tracking

**Client Benchmarks:**
- `BenchmarkClient_QueryCycle` - Complete client query cycle
- `BenchmarkClient_Create` - Client creation overhead
- `BenchmarkClient_Connect` - Client connection overhead

**Message Parsing Benchmarks:**
- `BenchmarkMessageParsing` - Sequential message parsing
- `BenchmarkMessageParsing_Parallel` - Parallel message parsing
- `BenchmarkContentBlockParsing` - Content block parsing

**Utility Benchmarks:**
- `BenchmarkOptionsBuilder` - Options builder pattern
- `BenchmarkContextCreation` - Context creation overhead
- `BenchmarkChannelOperations` - Channel send/receive
- `BenchmarkMemoryAllocation` - Memory allocation patterns
- `BenchmarkParseJSON` - Raw JSON parsing
- `BenchmarkErrorCreation` - Error creation
- `BenchmarkErrorWrapping` - Error wrapping

### coverage_test.go
Test coverage reporting and validation.

**Coverage Tests:**
- `TestCoverageReport` - Generates and prints coverage statistics
- `TestCoverageHTML` - Generates HTML coverage report

**Coverage Targets:**
- Public API: >85% coverage
- Internal packages: >80% coverage

### test_helpers.go
Test utilities and helper functions.

**Mock CLI Helpers:**
- `CreateMockCLI(behavior)` - Creates temporary mock CLI subprocess
- `CreateMockCLIWithMessages(messages)` - Creates mock CLI with predefined output
- `FindRealCLI()` - Locates actual Claude CLI for integration tests

**Assertion Helpers:**
- `AssertMessageType(msg, expected)` - Checks message type
- `AssertMessageContent(msg, expectedText)` - Checks message content
- `AssertNoGoroutineLeaks()` - Detects goroutine leaks

**Collection Helpers:**
- `CollectMessages(ctx, messages, timeout)` - Collects all messages with timeout
- `CollectMessagesUntilResult()` - Collects until ResultMessage

**Test Utilities:**
- `RequireAPIKey()` - Skips test if CLAUDE_API_KEY not set
- `CreateTestContext(timeout)` - Creates context with timeout
- `MarshalJSON(v)` / `UnmarshalJSON(data, v)` - JSON helpers
- `WithTimeout(timeout, fn)` - Runs function with timeout

## Running Tests

### All Tests
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Integration Tests Only
```bash
# Run integration tests
go test -v ./tests/...

# Skip long-running tests
go test -short ./tests/...
```

### Benchmarks
```bash
# Run all benchmarks
go test -bench=. ./tests/...

# Run specific benchmark
go test -bench=BenchmarkQuery ./tests/...

# With memory profiling
go test -bench=. -benchmem ./tests/...
```

### Coverage Reports
```bash
# Generate text coverage report
go test -run TestCoverageReport ./tests/...

# Generate HTML coverage report
go test -run TestCoverageHTML ./tests/...
open coverage.html
```

### Real CLI Integration Tests
These tests require the actual Claude CLI and API key:

```bash
# Set API key
export CLAUDE_API_KEY=your_api_key

# Run real CLI tests
go test -v -run TestRealCLIIntegration ./tests/...
go test -v -run TestControlProtocol_FullFlow ./tests/...
```

## Test Design Principles

### 1. Mock CLI for Speed
Most integration tests use mock CLI subprocess to avoid network calls and ensure fast, deterministic tests.

### 2. Goroutine Leak Detection
Each integration test includes goroutine leak detection to catch resource cleanup issues:
```go
checkGoroutines := AssertNoGoroutineLeaks(t)
defer checkGoroutines()
```

### 3. Context Timeouts
All tests use contexts with timeouts to prevent hanging:
```go
ctx, cancel := CreateTestContext(t, 30*time.Second)
defer cancel()
```

### 4. Graceful Degradation
Tests gracefully handle missing dependencies (Claude CLI, API key) by skipping rather than failing.

### 5. Parallel Execution
Tests are designed to run in parallel when possible using `t.Parallel()`.

## Test Coverage

### Current Coverage
- **Public API:** >85% (target)
- **Internal packages:** >80% (target)
- **Overall:** >85% (target)

### Coverage by Package
```
github.com/schlunsen/claude-agent-sdk-go            - Public API
github.com/schlunsen/claude-agent-sdk-go/internal   - Core logic
github.com/schlunsen/claude-agent-sdk-go/internal/transport - Transport layer
github.com/schlunsen/claude-agent-sdk-go/internal/types - Type definitions
```

## Mock CLI Behavior

The mock CLI helper supports different behaviors:

### echo
Simple echo behavior - reads stdin and writes to stdout
```go
mockCLI, _ := CreateMockCLI(t, "echo")
```

### simple-response
Returns predefined messages
```go
mockCLI, _ := CreateMockCLI(t, "simple-response")
```

### control-response
Returns control protocol responses
```go
mockCLI, _ := CreateMockCLI(t, "control-response")
```

### Custom messages
Specify exact messages to output
```go
messages := []string{
    `{"type":"assistant","content":[{"type":"text","text":"Hello"}],"model":"claude-3"}`,
    `{"type":"result","output":"success"}`,
}
mockCLI, _ := CreateMockCLIWithMessages(t, messages)
```

## Continuous Integration

Tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Run Tests
  run: go test -short -cover ./...

- name: Run Benchmarks
  run: go test -bench=. -benchtime=100ms ./tests/...
```

## Performance Targets

### Query Performance
- Simple query: <100ms (excluding CLI startup)
- CLI subprocess startup: 200-500ms (Node.js overhead)
- Message parsing: <1ms per message

### Memory Usage
- Typical session: <50MB
- No memory leaks in long-running sessions

### Goroutine Management
- Clean shutdown with no goroutine leaks
- Proper context cancellation handling

## Troubleshooting

### Tests hang
- Check for goroutine leaks: `go test -timeout 30s`
- Enable verbose logging: `go test -v`
- Check context cancellation

### Coverage too low
- Run coverage report: `go test -run TestCoverageReport`
- Generate HTML report: `go test -run TestCoverageHTML`
- Focus on untested code paths

### Integration tests fail
- Check Claude CLI installation: `which claude`
- Set API key: `export CLAUDE_API_KEY=...`
- Check network connectivity

### Benchmark variance
- Run with longer benchtime: `-benchtime=1s`
- Run multiple times: `-count=5`
- Check CPU throttling / background processes

## Contributing

When adding new tests:

1. **Integration tests** - Add to `integration_test.go`
   - Use mock CLI when possible
   - Include goroutine leak detection
   - Use timeouts

2. **Benchmarks** - Add to `benchmarks_test.go`
   - Focus on critical paths
   - Include memory profiling with `b.ReportAllocs()`

3. **Helpers** - Add to `test_helpers.go`
   - Document expected behavior
   - Handle errors gracefully

4. **Coverage** - Maintain >80% coverage
   - Run coverage report before PR
   - Add tests for uncovered paths

## Reference

- Main implementation: `/internal/`
- Python SDK reference: `/Users/schlunsen/projects/claude-agent-sdk-python/`
- Implementation plan: `/GO_PORT_PLAN.md`
