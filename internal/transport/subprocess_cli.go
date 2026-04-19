package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

const (
	// SDKVersion is the version identifier for this SDK
	SDKVersion = "0.8.2"
)

// SubprocessCLITransport implements Transport using a Claude Code CLI subprocess.
// It manages the subprocess lifecycle, stdin/stdout/stderr pipes, and message streaming.
type SubprocessCLITransport struct {
	cliPath         string
	cwd             string
	env             map[string]string
	logger          *log.Logger
	resumeSessionID string                    // Optional session ID to resume conversation
	options         *types.ClaudeAgentOptions // Options for CLI configuration

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	ctx    context.Context
	cancel context.CancelFunc

	// Message streaming
	messages chan types.Message

	// Writer for stdin
	writer *JSONLineWriter

	// Error tracking
	mu    sync.Mutex
	err   error
	ready bool
}

// NewSubprocessCLITransport creates a new transport instance.
// The cliPath should point to the claude binary.
// The cwd is the working directory for the subprocess (empty string uses current directory).
// The env map contains additional environment variables to set for the subprocess.
// The logger is used for debug/diagnostic output.
// The resumeSessionID is an optional session ID to resume a previous conversation.
// The options contains configuration for the CLI.
func NewSubprocessCLITransport(cliPath, cwd string, env map[string]string, logger *log.Logger, resumeSessionID string, options *types.ClaudeAgentOptions) *SubprocessCLITransport {
	return &SubprocessCLITransport{
		cliPath:         cliPath,
		cwd:             cwd,
		env:             env,
		logger:          logger,
		resumeSessionID: resumeSessionID,
		options:         options,
		messages:        make(chan types.Message, 10), // Buffered channel for smooth streaming
	}
}

// Connect starts the Claude Code CLI subprocess and establishes communication pipes.
// It launches the subprocess with "agent --stdio" arguments and sets up the environment.
func (t *SubprocessCLITransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd != nil {
		return nil // Already connected
	}

	t.logger.Debug("Starting Claude CLI subprocess: %s", t.cliPath)

	// Create cancellable context
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Build command arguments
	args := t.buildCommandArgs()

	// Log the full command for debugging
	t.logger.Debug("Claude CLI command: %s %v", t.cliPath, args)

	// Create command with arguments
	t.cmd = exec.CommandContext(t.ctx, t.cliPath, args...) // #nosec G204 -- cliPath is validated via findCLI()

	// Set working directory if provided
	if t.cwd != "" {
		t.cmd.Dir = t.cwd
	}

	// Set up environment variables
	// Start with current environment, filtering out CLAUDECODE vars to prevent nesting
	for _, envVar := range os.Environ() {
		if !strings.HasPrefix(envVar, "CLAUDECODE") {
			t.cmd.Env = append(t.cmd.Env, envVar)
		}
	}

	// Add SDK-specific variables
	t.cmd.Env = append(t.cmd.Env, "CLAUDE_CODE_ENTRYPOINT=sdk-go")
	t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("CLAUDE_AGENT_SDK_VERSION=%s", SDKVersion))

	// Add model environment variable if specified in options (ANTHROPIC_MODEL)
	// This is critical - both CLI flag and env var should be set for maximum compatibility
	if t.options != nil && t.options.Model != nil {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("ANTHROPIC_MODEL=%s", *t.options.Model))
		t.logger.Debug("Setting ANTHROPIC_MODEL environment variable: %s", *t.options.Model)
	} else {
		t.logger.Debug("ANTHROPIC_MODEL not set (using CLI default)")
	}

	// Add base URL environment variable if specified in options (ANTHROPIC_BASE_URL)
	// If not set, Claude CLI will use default Anthropic API endpoint
	if t.options != nil && t.options.BaseURL != nil {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s", *t.options.BaseURL))
		t.logger.Debug("Setting ANTHROPIC_BASE_URL environment variable: %s", *t.options.BaseURL)
	} else {
		t.logger.Debug("ANTHROPIC_BASE_URL not set (using default Anthropic API)")
	}

	// Add file checkpointing environment variable if enabled
	if t.options != nil && t.options.EnableFileCheckpointing {
		t.cmd.Env = append(t.cmd.Env, "CLAUDE_CODE_ENABLE_SDK_FILE_CHECKPOINTING=true")
		t.logger.Debug("Enabling file checkpointing via environment variable")
	}

	// Add custom environment variables (these can override the above if needed)
	for key, value := range t.env {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("%s=%s", key, value))
		t.logger.Debug("Setting custom environment variable: %s=%s", key, value)
	}

	// Set up pipes
	var err error

	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return types.NewCLIConnectionErrorWithCause("failed to create stdin pipe", err)
	}

	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return types.NewCLIConnectionErrorWithCause("failed to create stdout pipe", err)
	}

	t.stderr, err = t.cmd.StderrPipe()
	if err != nil {
		return types.NewCLIConnectionErrorWithCause("failed to create stderr pipe", err)
	}

	// Start the process
	if err := t.cmd.Start(); err != nil {
		t.logger.Error("Failed to start subprocess: %v", err)
		return types.NewCLIConnectionErrorWithCause("failed to start subprocess", err)
	}
	t.logger.Debug("CLI subprocess started successfully (PID: %d)", t.cmd.Process.Pid)

	// Create JSON line writer for stdin
	t.writer = NewJSONLineWriter(t.stdin)

	// Launch message reader loop in goroutine
	go t.messageReaderLoop(t.ctx)

	// Launch stderr reader for debugging
	go t.readStderr(t.ctx)

	// Mark as ready
	t.ready = true
	t.logger.Debug("Transport ready for communication")

	return nil
}

// messageReaderLoop reads JSON lines from stdout and parses them into messages.
// It runs in a goroutine and sends messages to the messages channel.
// It respects context cancellation and closes the messages channel when done.
func (t *SubprocessCLITransport) messageReaderLoop(ctx context.Context) {
	defer close(t.messages)

	t.logger.Debug("Message reader loop started")
	reader := NewJSONLineReader(t.stdout)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			t.logger.Debug("Message reader loop stopped: context cancelled")
			return
		default:
		}

		// Read next JSON line
		line, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				t.logger.Debug("Message reader loop stopped: EOF from CLI")
				// Normal end of stream
				return
			}

			t.logger.Error("Failed to read from CLI stdout: %v", err)
			// Store error and return
			t.OnError(types.NewJSONDecodeErrorWithCause(
				"failed to read JSON line from subprocess",
				string(line),
				err,
			))
			return
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse JSON into message
		msg, err := types.UnmarshalMessage(line)
		if err != nil {
			t.logger.Warning("Failed to parse message from CLI: %v", err)
			// Store parse error but continue reading
			t.OnError(err)
			continue
		}

		t.logger.Debug("Received message from CLI: type=%s", msg.GetMessageType())

		// Send message to channel (respect context cancellation)
		select {
		case <-ctx.Done():
			return
		case t.messages <- msg:
			// Message sent successfully
		}
	}
}

// Write sends a JSON message to the subprocess stdin.
// The data should be a complete JSON string (newline will be added automatically).
func (t *SubprocessCLITransport) Write(ctx context.Context, data string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return types.NewCLIConnectionError("transport is not ready for writing")
	}

	if t.writer == nil {
		return types.NewCLIConnectionError("stdin writer not initialized")
	}

	t.logger.Debug("Sending message to CLI stdin")

	// Write JSON line (includes newline and flush)
	if err := t.writer.WriteLine(data); err != nil {
		t.ready = false
		t.err = types.NewCLIConnectionErrorWithCause("failed to write to subprocess stdin", err)
		t.logger.Error("Failed to write to CLI stdin: %v", err)
		return t.err
	}

	return nil
}

// ReadMessages returns a channel of incoming messages from the subprocess.
// The channel is closed when the subprocess exits or an error occurs.
func (t *SubprocessCLITransport) ReadMessages(ctx context.Context) <-chan types.Message {
	return t.messages
}

// buildCommandArgs builds the command line arguments for the CLI subprocess.
// This is extracted into a separate method to allow for testing.
func (t *SubprocessCLITransport) buildCommandArgs() []string {
	args := []string{
		"--input-format=stream-json",
		"--output-format=stream-json",
		"--verbose",
	}

	// Add permission prompt tool if specified
	if t.options != nil && t.options.PermissionPromptToolName != nil {
		args = append(args, "--permission-prompt-tool", *t.options.PermissionPromptToolName)
		t.logger.Debug("Setting permission prompt tool: %s", *t.options.PermissionPromptToolName)
	}

	// Add permission mode if specified
	if t.options != nil && t.options.PermissionMode != nil {
		args = append(args, "--permission-mode", string(*t.options.PermissionMode))
		t.logger.Debug("Setting permission mode: %s", string(*t.options.PermissionMode))
	}

	// Add system prompt - always pass the flag to match Python SDK behavior
	// When nil, pass empty string to prevent unintended Claude Code defaults
	if t.options != nil {
		if t.options.SystemPrompt == nil {
			// Default to empty system prompt when not specified
			args = append(args, "--system-prompt", "")
			t.logger.Debug("Setting empty system prompt (default)")
		} else if promptStr, ok := t.options.SystemPrompt.(string); ok {
			// Handle string prompt
			args = append(args, "--system-prompt", promptStr)
			t.logger.Debug("Setting system prompt: %s", promptStr)
		} else if preset, ok := t.options.SystemPrompt.(types.SystemPromptPreset); ok {
			// Handle preset case - append to default Claude Code prompt
			if preset.Append != nil {
				args = append(args, "--append-system-prompt", *preset.Append)
				t.logger.Debug("Appending to system prompt preset: %s", *preset.Append)
			}
		} else if file, ok := t.options.SystemPrompt.(types.SystemPromptFile); ok {
			// Handle file-based system prompt
			args = append(args, "--system-prompt-file", file.Path)
			t.logger.Debug("Setting system prompt from file: %s", file.Path)
		}
	} else {
		// No options provided, use empty system prompt
		args = append(args, "--system-prompt", "")
		t.logger.Debug("Setting empty system prompt (no options)")
	}

	// Add model if specified
	if t.options != nil && t.options.Model != nil {
		args = append(args, "--model", *t.options.Model)
		t.logger.Debug("Setting model: %s", *t.options.Model)
	}

	// Add --resume flag if resuming a conversation
	if t.resumeSessionID != "" {
		args = append(args, "--resume", t.resumeSessionID)
		t.logger.Debug("Resuming Claude CLI conversation with session ID: %s", t.resumeSessionID)
	}

	// Add --fork-session flag if forking a resumed session
	if t.options != nil && t.options.ForkSession {
		args = append(args, "--fork-session")
		t.logger.Debug("Forking resumed session to new session ID")
	}

	// Add permission bypass flags if enabled
	if t.options != nil {
		// Must set allow flag first (acts as safety switch)
		if t.options.AllowDangerouslySkipPermissions {
			args = append(args, "--allow-dangerously-skip-permissions")
			t.logger.Debug("Allowing permission bypass (safety switch enabled)")

			// Only add skip flag if allow flag is also set
			if t.options.DangerouslySkipPermissions {
				args = append(args, "--dangerously-skip-permissions")
				t.logger.Debug("DANGER: Bypassing all permissions - use only in sandboxed environments!")
			}
		}
	}

	// Add extended thinking configuration
	// Python SDK converts ThinkingConfig to just the token count via --max-thinking-tokens
	if t.options != nil && t.options.Thinking != nil {
		if t.options.Thinking.Type == "enabled" && t.options.Thinking.BudgetTokens != nil {
			args = append(args, "--max-thinking-tokens", fmt.Sprintf("%d", *t.options.Thinking.BudgetTokens))
			t.logger.Debug("Setting max thinking tokens: %d", *t.options.Thinking.BudgetTokens)
		} else {
			// For "adaptive" or "disabled", don't pass any thinking flag - CLI handles it
			t.logger.Debug("Thinking type is %s, CLI will handle defaults", t.options.Thinking.Type)
		}
	} else if t.options != nil && t.options.MaxThinkingTokens != nil {
		// Deprecated fallback: use MaxThinkingTokens if Thinking is not set
		args = append(args, "--max-thinking-tokens", fmt.Sprintf("%d", *t.options.MaxThinkingTokens))
		t.logger.Debug("Setting max thinking tokens (deprecated): %d", *t.options.MaxThinkingTokens)
	}

	// Add effort level if specified
	if t.options != nil && t.options.Effort != nil {
		args = append(args, "--effort", string(*t.options.Effort))
		t.logger.Debug("Setting effort level: %s", string(*t.options.Effort))
	}

	// Add fallback model if specified
	if t.options != nil && t.options.FallbackModel != nil {
		args = append(args, "--fallback-model", *t.options.FallbackModel)
		t.logger.Debug("Setting fallback model: %s", *t.options.FallbackModel)
	}

	// Add output format if specified (JSON schema for structured output)
	// Python SDK extracts just the schema from output_format and passes it as --json-schema
	if t.options != nil && t.options.OutputFormat != nil {
		outputFormatJSON, err := json.Marshal(t.options.OutputFormat)
		if err != nil {
			t.logger.Warning("Failed to marshal output format to JSON: %v", err)
		} else {
			args = append(args, "--json-schema", string(outputFormatJSON))
			t.logger.Debug("Setting JSON schema: %s", string(outputFormatJSON))
		}
	}

	// Add sandbox configuration if specified
	if t.options != nil && t.options.Sandbox != nil {
		sandboxJSON, err := json.Marshal(t.options.Sandbox)
		if err != nil {
			t.logger.Warning("Failed to marshal sandbox config to JSON: %v", err)
		} else {
			args = append(args, "--sandbox", string(sandboxJSON))
			t.logger.Debug("Setting sandbox config: %s", string(sandboxJSON))
		}
	}

	// Note: File checkpointing is handled via environment variable in Connect()

	// Add budget limit if specified
	if t.options != nil && t.options.MaxBudgetUSD != nil {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", *t.options.MaxBudgetUSD))
		t.logger.Debug("Setting max budget: $%.2f USD", *t.options.MaxBudgetUSD)
	}

	// Add beta feature flags if specified
	if t.options != nil && len(t.options.Betas) > 0 {
		for _, beta := range t.options.Betas {
			args = append(args, "--betas", beta)
			t.logger.Debug("Adding beta feature flag: %s", beta)
		}
	}

	// Add plugin directories
	if t.options != nil && len(t.options.Plugins) > 0 {
		for _, plugin := range t.options.Plugins {
			if plugin.Type == "local" {
				args = append(args, "--plugin-dir", plugin.Path)
				t.logger.Debug("Adding plugin directory: %s", plugin.Path)
			} else {
				// This shouldn't happen if NewPluginConfig is used, but handle it anyway
				t.logger.Warning("Skipping unsupported plugin type: %s", plugin.Type)
			}
		}
	}

	// Add allowed tools if specified (comma-separated)
	if t.options != nil && len(t.options.AllowedTools) > 0 {
		args = append(args, "--allowedTools", joinStrings(t.options.AllowedTools, ","))
		t.logger.Debug("Setting allowed tools: %s", joinStrings(t.options.AllowedTools, ","))
	}

	// Add disallowed tools if specified (comma-separated)
	if t.options != nil && len(t.options.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", joinStrings(t.options.DisallowedTools, ","))
		t.logger.Debug("Setting disallowed tools: %s", joinStrings(t.options.DisallowedTools, ","))
	}

	// Add max turns if specified
	if t.options != nil && t.options.MaxTurns != nil {
		args = append(args, "--max-turns", fmt.Sprintf("%d", *t.options.MaxTurns))
		t.logger.Debug("Setting max turns: %d", *t.options.MaxTurns)
	}

	// Add additional directories (one flag per dir)
	if t.options != nil && len(t.options.AddDirs) > 0 {
		for _, dir := range t.options.AddDirs {
			args = append(args, "--add-dir", dir)
			t.logger.Debug("Adding directory: %s", dir)
		}
	}

	// Add settings file path if specified
	if t.options != nil && t.options.Settings != nil {
		args = append(args, "--settings", *t.options.Settings)
		t.logger.Debug("Setting settings file: %s", *t.options.Settings)
	}

	// Add MCP servers configuration (marshaled as JSON)
	if t.options != nil && t.options.McpServers != nil {
		mcpJSON, err := json.Marshal(t.options.McpServers)
		if err != nil {
			t.logger.Warning("Failed to marshal MCP servers config to JSON: %v", err)
		} else {
			args = append(args, "--mcp-config", string(mcpJSON))
			t.logger.Debug("Setting MCP config: %s", string(mcpJSON))
		}
	}

	// Add continue conversation flag
	if t.options != nil && t.options.ContinueConversation {
		args = append(args, "--continue")
		t.logger.Debug("Continuing conversation")
	}

	// Add setting sources if specified (enables local slash commands, CLAUDE.md, etc.)
	if t.options != nil && len(t.options.SettingSources) > 0 {
		sources := make([]string, len(t.options.SettingSources))
		for i, src := range t.options.SettingSources {
			sources[i] = string(src)
		}
		args = append(args, "--setting-sources", joinStrings(sources, ","))
		t.logger.Debug("Setting sources: %s", joinStrings(sources, ","))
	}

	// Note: Agents are sent via the initialize control protocol, not CLI flags

	// Add subagent execution configuration if specified
	if t.options != nil && t.options.SubagentExecution != nil {
		subagentJSON := make(map[string]interface{})

		if t.options.SubagentExecution.MultiInvocation != "" {
			subagentJSON["multi_invocation"] = string(t.options.SubagentExecution.MultiInvocation)
		}
		if t.options.SubagentExecution.MaxConcurrent > 0 {
			subagentJSON["max_concurrent"] = t.options.SubagentExecution.MaxConcurrent
		}
		if t.options.SubagentExecution.ErrorHandling != "" {
			subagentJSON["error_handling"] = string(t.options.SubagentExecution.ErrorHandling)
		}

		if len(subagentJSON) > 0 {
			subagentJSONBytes, err := json.Marshal(subagentJSON)
			if err != nil {
				t.logger.Warning("Failed to marshal subagent execution config to JSON: %v", err)
			} else {
				args = append(args, "--subagent-execution", string(subagentJSONBytes))
				t.logger.Debug("Subagent execution configuration: %s", string(subagentJSONBytes))
			}
		}
	}

	return args
}

// joinStrings joins strings with a separator (avoiding strings import)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// Close terminates the subprocess and cleans up all resources.
// It attempts to gracefully shut down the subprocess with a timeout.
func (t *SubprocessCLITransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd == nil {
		return nil // Not connected
	}

	t.logger.Debug("Closing CLI subprocess...")
	t.ready = false

	// Cancel the context to stop goroutines
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	// Close stdin to signal end of input
	if t.stdin != nil {
		_ = t.stdin.Close()
		t.stdin = nil
	}

	// Wait for process to exit (with context timeout)
	done := make(chan error, 1)
	go func() {
		done <- t.cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// Timeout - kill the process
		if t.cmd.Process != nil {
			_ = t.cmd.Process.Kill()
		}
		<-done // Wait for Wait() to return
		return types.NewProcessError("subprocess did not exit gracefully, killed")

	case err := <-done:
		// Process exited
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return types.NewProcessErrorWithCode(
					"subprocess exited with error",
					exitErr.ExitCode(),
				)
			}
			return types.NewProcessErrorWithCause("subprocess exited with error", err)
		}
		return nil
	}
}

// OnError stores an error that occurred during transport operation.
// This allows errors from the reading loop to be retrieved later.
func (t *SubprocessCLITransport) OnError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.err == nil {
		t.err = err
	}
}

// IsReady returns true if the transport is ready for communication.
func (t *SubprocessCLITransport) IsReady() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.ready
}

// GetError returns any error that occurred during transport operation.
// This is useful for checking if an error occurred in the reading loop.
func (t *SubprocessCLITransport) GetError() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.err
}

// readStderr reads stderr output in a goroutine for debugging.
// This is a helper function for monitoring subprocess errors.
// It also parses known error patterns and stores them as typed errors.
func (t *SubprocessCLITransport) readStderr(ctx context.Context) {
	if t.stderr == nil {
		return
	}

	// Determine if file logging is enabled via StderrLogFile option
	var logFile *os.File
	if t.options != nil && t.options.StderrLogFile != nil {
		// Resolve log file path
		logPath := *t.options.StderrLogFile
		if logPath == "" {
			// Use default location
			homeDir, _ := os.UserHomeDir()
			logPath = fmt.Sprintf("%s/.claude/agents_server/cli_stderr.log", homeDir)
		}

		// Create parent directory if it doesn't exist
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0750); err != nil { // #nosec G301 -- log directory only needs owner+group access
			fmt.Fprintf(os.Stderr,
				"[SDK] Failed to create stderr log directory %s: %v\n"+
					"Stderr file logging disabled. To fix, create directory:\n"+
					"  mkdir -p %s\n",
				logDir, err, logDir)
		} else {
			// Try to open log file
			var err error
			logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G302 G304 -- logPath is constructed from known safe paths
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"[SDK] Failed to open stderr log file %s: %v\n"+
						"Stderr file logging disabled. Possible fixes:\n"+
						"  1. Ensure directory exists: mkdir -p %s\n"+
						"  2. Check file permissions: chmod 644 %s\n"+
						"  3. Use custom path: opts.WithCustomStderrLogFile(\"/path/to/file.log\")\n",
					logPath, err, logDir, logPath)
			} else {
				t.logger.Debug("Stderr file logging enabled: %s", logPath)
			}
		}
	}

	// Ensure cleanup if file was opened
	if logFile != nil {
		defer func() {
			_ = logFile.Close()
		}()
	}

	reader := NewJSONLineReader(t.stderr)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadLine()
		if err != nil {
			return
		}

		// Process stderr output
		if len(line) > 0 {
			stderrText := string(line)

			// Write to log file if enabled and file is open
			if logFile != nil {
				_, _ = fmt.Fprintf(logFile, "[Claude CLI stderr]: %s\n", stderrText)
				_ = logFile.Sync() // Flush to disk immediately
			}

			// Call stderr callback if configured (for runtime control)
			if t.options != nil && t.options.Stderr != nil {
				t.options.Stderr(stderrText)
			}

			// Parse known error patterns and create typed errors
			t.parseStderrError(stderrText)
		}
	}
}

// parseStderrError parses stderr text for known error patterns and stores typed errors.
func (t *SubprocessCLITransport) parseStderrError(stderrText string) {
	// Check for "No conversation found with session ID:" error
	if matched, sessionID := extractSessionNotFoundError(stderrText); matched {
		// Create typed error
		err := types.NewSessionNotFoundError(
			sessionID,
			"Claude CLI could not find this conversation. It may have been deleted or the CLI was reinstalled.",
		)

		// Store error for retrieval
		t.OnError(err)

		// Log it
		t.logger.Error("Claude session not found: %s", sessionID)
	}
}

// extractSessionNotFoundError checks if the stderr text contains a session not found error.
// Returns (true, sessionID) if matched, (false, "") otherwise.
func extractSessionNotFoundError(stderrText string) (bool, string) {
	// Pattern: "No conversation found with session ID: <uuid>"
	// Example: "No conversation found with session ID: 8587b432-e504-42c8-b9a7-e3fd0b4b2c60"
	const pattern = "No conversation found with session ID:"

	if idx := findSubstring(stderrText, pattern); idx >= 0 {
		// Extract session ID after the pattern
		sessionIDStart := idx + len(pattern)
		if sessionIDStart < len(stderrText) {
			// Trim whitespace and extract the session ID
			remaining := stderrText[sessionIDStart:]
			sessionID := trimWhitespace(remaining)
			// Session ID is the first token (UUID format)
			if len(sessionID) > 0 {
				// Take everything up to the first whitespace or end of string
				endIdx := 0
				for endIdx < len(sessionID) && !isWhitespace(rune(sessionID[endIdx])) {
					endIdx++
				}
				sessionID = sessionID[:endIdx]
				return true, sessionID
			}
		}
	}

	return false, ""
}

// Helper functions for string parsing

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimWhitespace(s string) string {
	start := 0
	for start < len(s) && isWhitespace(rune(s[start])) {
		start++
	}
	end := len(s)
	for end > start && isWhitespace(rune(s[end-1])) {
		end--
	}
	return s[start:end]
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
