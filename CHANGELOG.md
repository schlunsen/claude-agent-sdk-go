# Changelog

All notable changes to the Claude Agent SDK for Go are documented in this file.

## [0.1.0] - 2025-10-18

### Initial Release - Complete Port from Python SDK

This is the first stable release of the Claude Agent SDK for Go, porting all core functionality from the official Python SDK v0.1.3.

#### Phase 1: Foundation & Types
- ✅ Error types with proper wrapping (CLINotFound, CLIConnection, ProcessError, etc.)
- ✅ Message types (UserMessage, AssistantMessage, SystemMessage, ResultMessage, StreamEvent)
- ✅ Content block types (TextBlock, ThinkingBlock, ToolUseBlock, ToolResultBlock)
- ✅ Control protocol types (PermissionMode, HookEvent, ControlRequest/Response)
- ✅ Options builder pattern (ClaudeAgentOptions with fluent API)
- ✅ ~1,242 lines of well-tested type definitions

#### Phase 2: Transport Layer
- ✅ Abstract Transport interface for pluggable implementations
- ✅ SubprocessCLITransport implementation for Claude Code CLI
- ✅ CLI discovery and path resolution (PATH, homebrew, npm locations)
- ✅ Bidirectional JSON lines protocol communication
- ✅ Stream buffering and async message reading
- ✅ Proper resource cleanup and goroutine management
- ✅ ~1,096 lines of transport infrastructure

#### Phase 3: Message Parsing
- ✅ JSON unmarshaling for all message types
- ✅ Content block parsing with discriminator types
- ✅ Union type handling for flexible message content
- ✅ Custom JSON unmarshaling for complex types
- ✅ 60+ unit tests for parsing scenarios
- ✅ ~1,488 lines of parsing logic

#### Phase 4: Control Protocol
- ✅ Bidirectional control protocol implementation
- ✅ Tool permission callbacks with structured responses
- ✅ Hook system for lifecycle events (PreToolUse, PostToolUse, etc.)
- ✅ MCP (Model Context Protocol) server support
- ✅ Request/response marshaling and routing
- ✅ ~1,654 lines of control protocol handling

#### Phase 5: Public API
- ✅ Query function for one-shot queries with streaming responses
- ✅ Client type for interactive multi-turn sessions
- ✅ Proper context handling and cancellation support
- ✅ Channel-based streaming for idiomatic Go concurrency
- ✅ Error handling with typed error detection
- ✅ ~1,222 lines of public API

#### Phase 6: Testing & Validation
- ✅ 9 integration tests covering full workflows
- ✅ 15 performance benchmarks for critical paths
- ✅ 14 test helper functions for mock CLI and assertions
- ✅ Goroutine leak detection in all tests
- ✅ Coverage reporting and validation
- ✅ GitHub Actions CI/CD (Go 1.20, 1.21, 1.22)
- ✅ 60%+ code coverage across packages
- ✅ ~2,079 lines of test code

#### Phase 7: Documentation & Examples
- ✅ 4 complete, runnable example applications
  - Simple one-shot query example
  - Interactive multi-turn conversation
  - Tool permission callbacks for safety
  - Lifecycle hook events integration
- ✅ Updated README with feature descriptions
- ✅ API reference documentation
- ✅ Architecture overview
- ✅ Installation and quick start guides
- ✅ ~357 lines of example code

#### Phase 8: Polish & Release
- ✅ Version file (0.1.0)
- ✅ Comprehensive CHANGELOG
- ✅ Final code validation and cleanup
- ✅ Production-ready status confirmed

### Features

#### Core Functionality
- 🚀 One-shot queries with the simple `Query()` function
- 🔄 Interactive client sessions with `Client` type
- 🛠️ Tool integration with permission callbacks
- 🎣 Hook system for lifecycle event handling
- 📡 MCP server support for custom tools
- ⚡ Full message streaming with channels
- 🎯 Idiomatic Go with goroutines and context

#### Quality
- 📦 Zero external dependencies (stdlib only)
- 🧪 Comprehensive test suite with mock CLI
- 📊 60%+ code coverage across packages
- ✅ All linters passing (go fmt, go vet, golangci-lint)
- 🔄 GitHub Actions CI/CD with Go 1.20, 1.21, 1.22
- 📝 Extensive documentation and examples

#### Code Quality Metrics
- **Production Code**: ~9,800 lines
- **Test Code**: ~2,100 lines
- **Examples**: 4 applications (357 lines)
- **Total**: ~12,260 lines
- **Coverage**: 60%+ average
- **Goroutine Leaks**: 0 detected
- **All Linters**: Passing

### Supported Go Versions
- Go 1.20+
- Go 1.21
- Go 1.22

### Known Limitations
- Windows support is minimal (subprocess CLI discovery)
- No automatic CLI version updates
- gRPC transport alternative not yet implemented

### Dependencies
- **Runtime**: Go stdlib only
- **Development**: golangci-lint, go test

### Breaking Changes
None - this is the first release.

### Bug Fixes
- Fixed CLI invocation command flags to use correct protocol format (#9)
  - Changed from `agent --stdio` to `--print --input-format=stream-json --output-format=stream-json --verbose`
  - Updated query message structure to match Python SDK format with nested message object
  - Added `parent_tool_use_id` and `session_id` fields to protocol messages
- Added support for nested message format in AssistantMessage parsing
  - Handle nested `message.content` format from Claude CLI responses
  - Extract model field from nested message structure
  - Fall back to top-level content for backward compatibility

### Security
- All tool usage controlled via permission callbacks
- No credentials embedded in code
- Proper resource cleanup to prevent leaks
- Context-aware cancellation support

### Contributors
- Rasmus Schlunsen (https://github.com/schlunsen)

### Acknowledgments
- Official [Claude Agent SDK for Python](https://github.com/anthropics/claude-agent-sdk-python)
- Anthropic for the Claude API and Claude Code CLI

---

For detailed implementation information, see:
- [GO_PORT_PLAN.md](./GO_PORT_PLAN.md) - Implementation plan
- [PHASE_6_SUMMARY.md](./PHASE_6_SUMMARY.md) - Testing phase details
- [README.md](./README.md) - User documentation
- [tests/README.md](./tests/README.md) - Testing documentation
