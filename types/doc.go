// Package types provides type definitions for the Claude Agent SDK.
//
// This package contains all public types used when interacting with Claude Code CLI,
// including messages, content blocks, errors, control protocol types, and configuration options.
//
// # Message Types
//
// Messages represent communication between the user and Claude:
//
//   - UserMessage: Messages from the user to Claude
//   - AssistantMessage: Claude's responses with content blocks
//   - SystemMessage: System notifications and metadata
//   - ResultMessage: Final result with cost/usage info
//   - StreamEvent: Partial message updates during streaming
//
// Example:
//
//	for msg := range messages {
//	    switch m := msg.(type) {
//	    case *types.AssistantMessage:
//	        for _, block := range m.Content {
//	            if tb, ok := block.(*types.TextBlock); ok {
//	                fmt.Println("Claude:", tb.Text)
//	            }
//	        }
//	    case *types.ResultMessage:
//	        fmt.Printf("Done. Cost: $%.4f\n", *m.TotalCostUSD)
//	    }
//	}
//
// # Content Blocks
//
// Content blocks represent different types of content in messages:
//
//   - TextBlock: Plain text content
//   - ThinkingBlock: Claude's internal reasoning
//   - ToolUseBlock: Tool invocation requests
//   - ToolResultBlock: Results from tool execution
//
// # Error Types
//
// The SDK provides typed errors for specific failure scenarios:
//
//   - CLINotFoundError: Claude Code CLI binary not found
//   - CLIConnectionError: Failed to connect to CLI process
//   - ProcessError: CLI subprocess errors (exit codes, crashes)
//   - JSONDecodeError: Invalid JSON from CLI
//   - MessageParseError: Valid JSON but invalid message structure
//   - ControlProtocolError: Control protocol violations
//   - PermissionDeniedError: Permission request denied
//
// Use the Is* helper functions for error checking:
//
//	if types.IsCLINotFoundError(err) {
//	    log.Fatal("Please install Claude Code CLI: npm install -g @anthropic-ai/claude-code")
//	}
//
// # Configuration
//
// ClaudeAgentOptions provides a fluent builder API for configuration:
//
//	opts := types.NewClaudeAgentOptions().
//	    WithModel("claude-3-5-sonnet-latest").
//	    WithAllowedTools("Bash", "Write", "Read").
//	    WithPermissionMode(types.PermissionModeAcceptEdits).
//	    WithCanUseTool(func(ctx context.Context, toolName string, input map[string]interface{}, permCtx ToolPermissionContext) (interface{}, error) {
//	        // Custom permission logic
//	        return &PermissionResultAllow{Behavior: "allow"}, nil
//	    })
//
// # Control Protocol
//
// The control protocol enables bidirectional communication with the CLI:
//
//   - Permission callbacks: Control which tools Claude can use
//   - Hook system: React to lifecycle events (PreToolUse, PostToolUse, etc.)
//   - MCP servers: Define custom tools via Model Context Protocol
//
// # Hook Events
//
// Available hook events:
//
//   - HookEventPreToolUse: Before tool execution
//   - HookEventPostToolUse: After tool execution
//   - HookEventUserPromptSubmit: When user submits a prompt
//   - HookEventStop: When session stops
//   - HookEventSubagentStop: When a subagent stops
//   - HookEventPreCompact: Before context compaction
//
// Example hook:
//
//	opts.WithHook(types.HookEventPreToolUse, types.HookMatcher{
//	    Hooks: []types.HookCallbackFunc{
//	        func(ctx context.Context, input interface{}, toolUseID *string, hookCtx types.HookContext) (interface{}, error) {
//	            preToolInput := input.(*types.PreToolUseHookInput)
//	            log.Printf("Tool %s about to execute", preToolInput.ToolName)
//	            return &types.SyncHookJSONOutput{}, nil
//	        },
//	    },
//	})
//
// # Permission Modes
//
// Permission modes control how tool permissions are handled:
//
//   - PermissionModeDefault: Ask user for each tool use
//   - PermissionModeAcceptEdits: Auto-allow file edits
//   - PermissionModePlan: Plan mode (review before execution)
//   - PermissionModeBypassPermissions: Allow all tools (use with caution)
//
// # MCP Server Configuration
//
// MCP servers can be configured in multiple ways:
//
//   - McpStdioServerConfig: External server via stdio
//   - McpSSEServerConfig: External server via Server-Sent Events
//   - McpHTTPServerConfig: External server via HTTP
//   - McpSdkServerConfig: In-process SDK server
//
// # Thread Safety
//
// Types in this package are generally safe for concurrent reads, but mutable
// operations (e.g., modifying ClaudeAgentOptions after creation) are not thread-safe.
// Use appropriate synchronization if sharing instances across goroutines.
package types
