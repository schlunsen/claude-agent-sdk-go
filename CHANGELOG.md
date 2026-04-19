# Changelog

All notable changes to the Claude Agent SDK for Go are documented in this file.

## [0.8.2] - 2026-04-19

### Fixed
- `middleware/rtk.TrackingDBPath()` returned the wrong filename
  (`tracking.db`) — rtk 0.37.1 actually stores its SQLite database as
  `history.db` (verified against a running sandbox at
  `~/.local/share/rtk/history.db`). All platform branches now return
  `history.db`, so callers doing `os.Stat(TrackingDBPath())` get a
  true positive when rtk has recorded any commands.
- Updated the 3 platform-path tests to assert the corrected filename.

## [0.8.1] - 2026-04-19

### Added
- `middleware/rtk` stats helpers for surfacing RTK compression savings
  in UIs and dashboards:
  - `Gain(ctx, opts...)` shells out to `rtk gain --format json` and
    decodes into typed Go structs (`GainExport`, `GainSummary`,
    `DayStats`, `WeekStats`, `MonthStats`). Options:
    `WithProject`, `WithDaily`, `WithWeekly`, `WithMonthly`, `WithAll`,
    `WithGainBinary`, `WithGainEnv`.
  - `TrackingDBPath()` returns the platform-specific path to rtk's
    SQLite tracking database (macOS `~/Library/Application Support/rtk`,
    Linux `$XDG_DATA_HOME/rtk` with `~/.local/share/rtk` fallback,
    Windows `%APPDATA%/rtk`). Callers who want per-invocation rows
    (which `rtk gain` does not export as JSON) can open the DB directly.
  - `IsInstalled(binary)` thin `exec.LookPath` wrapper for cheap
    health-check style calls.
  - `ErrRTKNotInstalled` sentinel and `*GainError` with preserved
    stderr for actionable error handling.
- 12 additional tests covering Gain argv construction, all breakdown
  flags, non-zero exits, empty/invalid JSON, missing binary, custom
  binary names, and platform-specific tracking DB paths.

### Notes
- `rtk gain --history`, `--failures`, and `--quota` remain text-only
  upstream; these have no JSON export path and are not wrapped.

## [0.8.0] - 2026-04-19

### Added
- `middleware/rtk` package: optional `PreToolUse` hook that wraps Bash
  commands with the [RTK](https://github.com/rtk-ai/rtk) CLI proxy to
  compress command output by 60-90% before it reaches the model.
  - Options: `WithBinary`, `WithCommands` (zero-args is a safe no-op,
    not a wipe), `WithAddedCommands`, `WithBlocked`, `WithUltraCompact`,
    `OnlyIfInstalled` (graceful degradation when rtk isn't on PATH).
  - Shell-aware rewriter tracks quotes, escapes, backticks, and
    parenthesis depth (incl. `$(...)` command substitution and
    subshells), so operators inside nested contexts do not split.
  - Wrapper-flag-value heuristic handles `sudo -u deploy git …`,
    `nice -n 10 git …`, `env -i PATH=/bin git …` correctly while
    refusing to swallow arguments of bare commands like `echo git`.
  - 27 unit tests incl. lossless segment round-trip fidelity.
- `examples/with_rtk/main.go` demonstrating RTK middleware usage.
- New **Middleware** section in README documenting the extensibility
  pattern and the `middleware/rtk` subpackage.

## [0.7.0] - 2026-04-01

### Added
- 9 new Client control methods: `SetModel`, `Interrupt`, `SetPermissionMode`, `StopTask`, `RewindFiles`, `ReconnectMcpServer`, `ToggleMcpServer`, `GetMcpStatus`, `GetServerInfo`
- `RateLimitEvent` and `RateLimitInfo` message types for rate limit monitoring
- `AssistantMessageError` struct for error details on assistant messages
- Missing fields on `AssistantMessage`: `Error`, `Usage`, `MessageID`, `StopReason`, `SessionID`, `UUID`
- Missing fields on `ResultMessage`: `StopReason`, `StructuredOutput`, `ModelUsage`, `PermissionDenials`, `UUID`
- Missing fields on `UserMessage`: `UUID`, `ToolUseResult`
- `PermissionModeDontAsk` constant
- CLI arg serialization for `AllowedTools`, `DisallowedTools`, `MaxTurns`, `AddDirs`, `Settings`, `McpServers`, `ContinueConversation`

### Fixed
- **Agents delivery**: Removed `--agents` CLI flag, agents now sent via `initialize` control protocol (matching Python/TypeScript SDK)
- **Output format**: Changed `--output-format` to `--json-schema` to match Python SDK
- **Thinking config**: Changed from full JSON `--thinking` to extracting budget as `--max-thinking-tokens` to match Python SDK
- **File checkpointing**: Changed from CLI flag to `CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING` env var to match Python SDK
- **Environment**: Filter `CLAUDECODE` env vars to prevent subprocess nesting bugs
- **Entrypoint**: Changed from `"agent"` to `"sdk-go"` for proper SDK identification

## [0.6.1] - 2026-03-31

### Fixed
- CLI arg serialization for v0.6.0 options fields — `ThinkingConfig`, `Effort`, `FallbackModel`, `OutputFormat`, `Sandbox`, `EnableFileCheckpointing`, and `SystemPromptFile` now actually get passed to the Claude CLI subprocess (were previously dead code)
- Updated `VERSION` file from `0.5.1` to `0.6.0`
- Updated `SDKVersion` constant from `0.1.0` to `0.6.0`

### Added
- Comprehensive tests for all new CLI argument serialization paths
- `--system-prompt-file` support in `buildCommandArgs()` for file-based system prompts

## [0.6.0] - 2026-03-31

### Added
- Skills support for agent definitions (`Skills []string` field)
- Memory field for agent definitions (`Memory *string`)
- MCP servers field for agent definitions (`McpServers []interface{}`)
- Four new hook event types: `PostToolUseFailure`, `SubagentStart`, `Notification`, `PermissionRequest`
- Subagent context fields (`agent_id`, `agent_type`) on hook inputs
- `tool_use_id` field on `PreToolUseHookInput` and `PostToolUseHookInput`
- `agent_transcript_path` field on `SubagentStopHookInput`
- `HookMatcher.Timeout` field with control protocol serialization
- `ThinkingConfig` type with `NewThinkingAdaptive()`, `NewThinkingEnabled()`, `NewThinkingDisabled()` constructors
- `EffortLevel` type (low/medium/high/max)
- `SandboxSettings` struct for Bash sandbox configuration
- `SystemPromptFile` struct for file-based system prompts
- New `ClaudeAgentOptions` fields: `FallbackModel`, `Thinking`, `Effort`, `Sandbox`, `OutputFormat`, `EnableFileCheckpointing`
- Builder methods: `WithFallbackModel`, `WithThinking`, `WithEffort`, `WithSandbox`, `WithOutputFormat`, `WithEnableFileCheckpointing`, `WithSystemPromptFile`

### Fixed
- Flaky `TestSDKMCPServer_HandleListTools` test caused by non-deterministic map iteration order on Go 1.25

## [0.2.9] - 2025-12-07

### Added
- Beta API feature support via `WithBetas()` and `WithBeta()` builder methods
- Support for Anthropic beta APIs like extended context windows (`context-1m-2025-08-07`)
- Pass beta feature flags to Claude Code CLI via `--betas` flag
- Comprehensive example demonstrating beta feature usage (`examples/with_betas/`)
- Extensive unit tests for betas functionality in both options and transport layers

### Details
- Implements feature parity with Python SDK v0.1.12+ for beta support
- Full CLI flag generation testing for `--betas` argument passing
- Supports multiple simultaneous beta features
- Method chaining support for fluent API usage
- Closes #23

## [0.2.2] - 2025-10-19

### Added
- Permission mode support with proper CLI flag passing
- Verbose logging option that can be enabled via `ClaudeAgentOptions.Verbose`
- System prompt support via `--system-prompt` flag to Claude CLI
- Permission prompt tool flag (`--permission-prompt-tool stdio`) for control protocol

### Fixed
- Control request handling for CLI-initiated requests without `request_id`
  - SDK now automatically generates request IDs for CLI-initiated control requests
  - Fixes permission callbacks that were failing silently
- Request ID parsing from top-level field in control_request messages
  - CLI sends `request_id` at top level, not inside request object
  - Fixes issue where control responses weren't matched to requests
  - Permission approvals are now properly recognized by CLI
- Client now properly passes options to transport layer
- Control protocol initialization and bidirectional communication

### Changed
- Enhanced control request logging for better debugging
- Updated `SubprocessCLITransport` to accept and use `ClaudeAgentOptions`
- Improved `SystemMessage` type with `RequestID` field for control protocol

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
- Go 1.24+

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
- Fixed interactive client connection hang and added verbose logging (#10)
  - Made verbose logging configurable via `CLAUDE_AGENT_VERBOSE` environment variable
  - Fixed Client.Connect() to wait for control protocol initialization
  - Added stderr logging to file at `~/.claude/agents_server/cli_stderr.log`
  - Improved error handling in control protocol initialization

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