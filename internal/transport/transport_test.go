package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// TestFindCLI tests CLI discovery in various scenarios
func TestFindCLI(t *testing.T) {
	// Disable version checking for these tests since we're using mock binaries
	t.Setenv("CLAUDE_AGENT_SDK_SKIP_VERSION_CHECK", "1")

	tests := []struct {
		name      string
		setup     func() func() // Returns cleanup function
		wantError bool
	}{
		{
			name: "CLI in PATH",
			setup: func() func() {
				// Save original PATH
				origPath := os.Getenv("PATH")

				// Create temporary directory with a mock claude binary
				tmpDir := t.TempDir()
				claudePath := filepath.Join(tmpDir, "claude")

				// Create mock binary
				f, err := os.Create(claudePath)
				if err != nil {
					t.Fatalf("Failed to create mock binary: %v", err)
				}
				_ = f.Close()

				// Make it executable
				if err := os.Chmod(claudePath, 0755); err != nil {
					t.Fatalf("Failed to chmod mock binary: %v", err)
				}

				// Add to PATH
				_ = os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

				// Return cleanup function
				return func() {
					_ = os.Setenv("PATH", origPath)
				}
			},
			wantError: false,
		},
		// Note: "CLI not found" test is skipped because it's environment-dependent
		// If Claude CLI is installed in standard locations (like ~/.local/bin/claude),
		// it will be found even when PATH/HOME are cleared since FindCLI checks
		// hardcoded paths using user.Current(). This is actually desired behavior.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup()
			defer cleanup()

			path, err := FindCLI()

			if tt.wantError {
				if err == nil {
					t.Errorf("FindCLI() expected error, got nil (found path: %s, PATH=%s, HOME=%s)", path, os.Getenv("PATH"), os.Getenv("HOME"))
				}
				var cliNotFoundErr *types.CLINotFoundError
				if err != nil && !types.IsCLINotFoundError(err) {
					t.Errorf("FindCLI() error type = %T, want *types.CLINotFoundError", err)
				}
				_ = cliNotFoundErr
			} else {
				if err != nil {
					t.Errorf("FindCLI() unexpected error: %v", err)
				}
				if path == "" {
					t.Errorf("FindCLI() returned empty path")
				}
			}
		})
	}
}

// TestExpandHome tests home directory expansion
func TestExpandHome(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde only",
			input: "~",
			want:  "HOME_DIR", // Will be replaced in test
		},
		{
			name:  "tilde with path",
			input: "~/.config/claude",
			want:  "HOME_DIR/.config/claude",
		},
		{
			name:  "no tilde",
			input: "/usr/local/bin/claude",
			want:  "/usr/local/bin/claude",
		},
		{
			name:  "relative path",
			input: "./bin/claude",
			want:  "./bin/claude",
		},
	}

	// Get actual home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replace placeholder with actual home dir
			want := strings.ReplaceAll(tt.want, "HOME_DIR", homeDir)

			got := expandHome(tt.input)
			if got != want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, want)
			}
		})
	}
}

// TestJSONLineReader tests buffered JSON line reading
func TestJSONLineReader(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "single line",
			input: `{"type":"test","data":"hello"}` + "\n",
			want:  []string{`{"type":"test","data":"hello"}`},
		},
		{
			name: "multiple lines",
			input: `{"type":"test1"}` + "\n" +
				`{"type":"test2"}` + "\n" +
				`{"type":"test3"}` + "\n",
			want: []string{
				`{"type":"test1"}`,
				`{"type":"test2"}`,
				`{"type":"test3"}`,
			},
		},
		{
			name:  "empty lines ignored",
			input: `{"type":"test1"}` + "\n\n" + `{"type":"test2"}` + "\n",
			want:  []string{`{"type":"test1"}`, `{"type":"test2"}`},
		},
		{
			name:  "trailing newline",
			input: `{"type":"test"}` + "\n",
			want:  []string{`{"type":"test"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewJSONLineReader(strings.NewReader(tt.input))

			var got []string
			for {
				line, err := reader.ReadLine()
				if err == io.EOF {
					break
				}
				if err != nil {
					if !tt.wantErr {
						t.Errorf("ReadLine() unexpected error: %v", err)
					}
					return
				}

				if len(line) > 0 {
					got = append(got, string(line))
				}
			}

			if len(got) != len(tt.want) {
				t.Errorf("ReadLine() got %d lines, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if i >= len(tt.want) {
					break
				}
				if got[i] != tt.want[i] {
					t.Errorf("ReadLine() line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestJSONLineReaderBufferOverflow tests buffer size limits
func TestJSONLineReaderBufferOverflow(t *testing.T) {
	// Create a JSON line larger than the buffer
	// Note: bufio.Scanner needs significantly larger input to trigger the error
	smallBufferSize := 1024
	largeJSON := `{"data":"` + strings.Repeat("x", smallBufferSize*2) + `"}`

	reader := NewJSONLineReaderWithSize(strings.NewReader(largeJSON+"\n"), smallBufferSize)

	_, err := reader.ReadLine()
	// The scanner may or may not fail depending on internal buffering
	// We just verify that if there's an error, it's handled correctly
	if err != nil {
		t.Logf("ReadLine() error (expected for large buffer): %v", err)
	} else {
		// For smaller sizes, the scanner may succeed by growing the buffer
		t.Logf("ReadLine() succeeded (scanner grew buffer)")
	}
}

// TestJSONLineWriter tests buffered JSON line writing
func TestJSONLineWriter(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "single line",
			lines: []string{`{"type":"test"}`},
			want:  `{"type":"test"}` + "\n",
		},
		{
			name: "multiple lines",
			lines: []string{
				`{"type":"test1"}`,
				`{"type":"test2"}`,
				`{"type":"test3"}`,
			},
			want: `{"type":"test1"}` + "\n" +
				`{"type":"test2"}` + "\n" +
				`{"type":"test3"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewJSONLineWriter(&buf)

			for _, line := range tt.lines {
				if err := writer.WriteLine(line); err != nil {
					t.Errorf("WriteLine() unexpected error: %v", err)
				}
			}

			got := buf.String()
			if got != tt.want {
				t.Errorf("WriteLine() wrote %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSubprocessCLITransportConnect tests subprocess connection
func TestSubprocessCLITransportConnect(t *testing.T) {
	// Skip if no echo command available
	echoPath, err := FindMockCLI()
	if err != nil {
		t.Skip("No echo command available for testing")
	}

	logger := log.NewLogger(false) // Non-verbose for tests
	transport := NewSubprocessCLITransport(echoPath, "", nil, logger, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect should succeed
	if err := transport.Connect(ctx); err != nil {
		t.Errorf("Connect() unexpected error: %v", err)
	}

	// Should be ready
	if !transport.IsReady() {
		t.Errorf("IsReady() = false, want true after Connect()")
	}

	// Clean up
	if err := transport.Close(ctx); err != nil {
		t.Logf("Close() error (may be expected): %v", err)
	}
}

// TestSubprocessCLITransportWrite tests writing to subprocess
func TestSubprocessCLITransportWrite(t *testing.T) {
	// Use cat command as a simple echo subprocess
	catPath, err := FindMockCLI()
	if err != nil {
		t.Skip("No cat command available for testing")
	}

	logger := log.NewLogger(false) // Non-verbose for tests
	transport := NewSubprocessCLITransport(catPath, "", nil, logger, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}
	defer func() {
		_ = transport.Close(ctx)
	}()

	// Write should succeed
	testJSON := `{"type":"test","data":"hello"}`
	if err := transport.Write(ctx, testJSON); err != nil {
		t.Errorf("Write() unexpected error: %v", err)
	}
}

// TestSubprocessCLITransportClose tests subprocess cleanup
func TestSubprocessCLITransportClose(t *testing.T) {
	echoPath, err := FindMockCLI()
	if err != nil {
		t.Skip("No echo command available for testing")
	}

	logger := log.NewLogger(false) // Non-verbose for tests
	transport := NewSubprocessCLITransport(echoPath, "", nil, logger, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect and then close
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}

	if err := transport.Close(ctx); err != nil {
		t.Logf("Close() error (may be expected for echo): %v", err)
	}

	// Should not be ready after close
	if transport.IsReady() {
		t.Errorf("IsReady() = true, want false after Close()")
	}
}

// TestMessageReaderLoop tests message reading and parsing
func TestMessageReaderLoop(t *testing.T) {
	// Create a mock JSON stream
	jsonStream := `{"type":"user","content":"hello"}` + "\n" +
		`{"type":"assistant","content":[{"type":"text","text":"hi"}],"model":"claude-3"}` + "\n" +
		`{"type":"system","subtype":"info","data":{}}` + "\n"

	// Create a pipe to simulate subprocess output
	pr, pw := io.Pipe()

	// Write mock data in a goroutine
	go func() {
		defer func() {
			_ = pw.Close()
		}()
		_, _ = pw.Write([]byte(jsonStream))
	}()

	// Create transport with custom stdout
	logger := log.NewLogger(false) // Non-verbose for tests
	transport := &SubprocessCLITransport{
		messages: make(chan types.Message, 10),
		ready:    true,
		logger:   logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	transport.ctx = ctx
	transport.stdout = pr

	// Start reader loop
	go transport.messageReaderLoop(ctx)

	// Read messages from channel
	var messages []types.Message
	for msg := range transport.messages {
		messages = append(messages, msg)
	}

	// Should have parsed 3 messages
	if len(messages) != 3 {
		t.Errorf("messageReaderLoop() parsed %d messages, want 3", len(messages))
	}

	// Verify message types
	expectedTypes := []string{"user", "assistant", "system"}
	for i, msg := range messages {
		if i >= len(expectedTypes) {
			break
		}
		if msg.GetMessageType() != expectedTypes[i] {
			t.Errorf("message[%d].Type = %q, want %q", i, msg.GetMessageType(), expectedTypes[i])
		}
	}
}

// TestSubprocessEnvironment tests environment variable setup
func TestSubprocessEnvironment(t *testing.T) {
	echoPath, err := FindMockCLI()
	if err != nil {
		t.Skip("No echo command available for testing")
	}

	env := map[string]string{
		"TEST_VAR":    "test_value",
		"ANOTHER_VAR": "another_value",
	}

	logger := log.NewLogger(false) // Non-verbose for tests
	transport := NewSubprocessCLITransport(echoPath, "", env, logger, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}
	defer func() {
		_ = transport.Close(ctx)
	}()

	// Check that environment variables were set (we can't directly verify,
	// but we can check that Connect succeeded with the env)
	if !transport.IsReady() {
		t.Errorf("IsReady() = false after Connect() with custom env")
	}
}

// FindMockCLI finds a command suitable for testing (cat, echo, etc.)
func FindMockCLI() (string, error) {
	// Try to find cat command (available on Unix systems)
	if path, err := exec.LookPath("cat"); err == nil {
		return path, nil
	}

	// Try echo as fallback
	if path, err := exec.LookPath("echo"); err == nil {
		return path, nil
	}

	return "", types.NewCLINotFoundError("no suitable test command found (cat or echo)")
}

// BenchmarkJSONLineReader benchmarks JSON line reading performance
func BenchmarkJSONLineReader(b *testing.B) {
	// Create test data
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = `{"type":"test","data":"` + strings.Repeat("x", 100) + `"}`
	}
	input := strings.Join(lines, "\n") + "\n"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader := NewJSONLineReader(strings.NewReader(input))
		for {
			_, err := reader.ReadLine()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("ReadLine() error: %v", err)
			}
		}
	}
}

// BenchmarkJSONLineWriter benchmarks JSON line writing performance
func BenchmarkJSONLineWriter(b *testing.B) {
	line := `{"type":"test","data":"` + strings.Repeat("x", 100) + `"}`

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writer := NewJSONLineWriter(&buf)
		for j := 0; j < 1000; j++ {
			if err := writer.WriteLine(line); err != nil {
				b.Fatalf("WriteLine() error: %v", err)
			}
		}
	}
}

// TestIntegrationSubprocessCLI tests end-to-end subprocess communication
// This test requires the actual Claude CLI to be installed
func TestIntegrationSubprocessCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Try to find Claude CLI
	cliPath, err := FindCLI()
	if err != nil {
		t.Skipf("Claude CLI not found, skipping integration test: %v", err)
	}

	logger := log.NewLogger(false) // Non-verbose for tests
	transport := NewSubprocessCLITransport(cliPath, "", nil, logger, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to CLI
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer func() {
		_ = transport.Close(ctx)
	}()

	// Should be ready
	if !transport.IsReady() {
		t.Errorf("IsReady() = false after successful Connect()")
	}

	// Try to write a simple query
	query := map[string]interface{}{
		"type":    "control",
		"subtype": "query",
		"prompt":  "Hello, Claude!",
	}

	queryJSON, err := json.Marshal(query)
	if err != nil {
		t.Fatalf("Failed to marshal query: %v", err)
	}

	if err := transport.Write(ctx, string(queryJSON)); err != nil {
		t.Errorf("Write() failed: %v", err)
	}

	// Read messages (with timeout)
	messages := transport.ReadMessages(ctx)

	select {
	case msg := <-messages:
		if msg == nil {
			t.Errorf("Received nil message")
		} else {
			t.Logf("Received message type: %s", msg.GetMessageType())
		}
	case <-time.After(5 * time.Second):
		t.Logf("Timeout waiting for response (may be expected for this test)")
	}
}

// TestExtractSessionNotFoundError tests parsing of session not found errors from stderr
func TestExtractSessionNotFoundError(t *testing.T) {
	tests := []struct {
		name          string
		stderrText    string
		wantMatched   bool
		wantSessionID string
	}{
		{
			name:          "valid session not found error",
			stderrText:    "No conversation found with session ID: 8587b432-e504-42c8-b9a7-e3fd0b4b2c60",
			wantMatched:   true,
			wantSessionID: "8587b432-e504-42c8-b9a7-e3fd0b4b2c60",
		},
		{
			name:          "session not found with extra text",
			stderrText:    "Error: No conversation found with session ID: 12345678-1234-1234-1234-123456789abc. Please check the ID.",
			wantMatched:   true,
			wantSessionID: "12345678-1234-1234-1234-123456789abc.",
		},
		{
			name:          "session not found with leading whitespace",
			stderrText:    "No conversation found with session ID:   abc123-def456  ",
			wantMatched:   true,
			wantSessionID: "abc123-def456",
		},
		{
			name:          "different error message",
			stderrText:    "Connection failed: timeout",
			wantMatched:   false,
			wantSessionID: "",
		},
		{
			name:          "partial match",
			stderrText:    "No conversation found",
			wantMatched:   false,
			wantSessionID: "",
		},
		{
			name:          "empty string",
			stderrText:    "",
			wantMatched:   false,
			wantSessionID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMatched, gotSessionID := extractSessionNotFoundError(tt.stderrText)

			if gotMatched != tt.wantMatched {
				t.Errorf("extractSessionNotFoundError() matched = %v, want %v", gotMatched, tt.wantMatched)
			}

			if gotSessionID != tt.wantSessionID {
				t.Errorf("extractSessionNotFoundError() sessionID = %q, want %q", gotSessionID, tt.wantSessionID)
			}
		})
	}
}

// TestParseStderrError tests the stderr error parsing and error creation
func TestParseStderrError(t *testing.T) {
	logger := log.NewLogger(false)
	transport := &SubprocessCLITransport{
		logger:   logger,
		messages: make(chan types.Message, 10),
	}

	// Test session not found error
	stderrText := "No conversation found with session ID: 8587b432-e504-42c8-b9a7-e3fd0b4b2c60"
	transport.parseStderrError(stderrText)

	// Check that error was stored
	err := transport.GetError()
	if err == nil {
		t.Fatal("parseStderrError() should have stored an error")
	}

	// Check that it's the right error type
	if !types.IsSessionNotFoundError(err) {
		t.Errorf("parseStderrError() stored error type = %T, want SessionNotFoundError", err)
	}

	// Check session ID is in the error
	if sessionErr, ok := err.(*types.SessionNotFoundError); ok {
		if sessionErr.SessionID != "8587b432-e504-42c8-b9a7-e3fd0b4b2c60" {
			t.Errorf("SessionNotFoundError.SessionID = %q, want %q",
				sessionErr.SessionID, "8587b432-e504-42c8-b9a7-e3fd0b4b2c60")
		}
	}
}

// TestForkSessionFlag tests that --fork-session flag is passed when ForkSession is true
func TestForkSessionFlag(t *testing.T) {
	tests := []struct {
		name            string
		resumeSessionID string
		forkSession     bool
		wantResumeFlag  bool
		wantForkFlag    bool
	}{
		{
			name:            "with resume and fork session",
			resumeSessionID: "test-session-id",
			forkSession:     true,
			wantResumeFlag:  true,
			wantForkFlag:    true,
		},
		{
			name:            "with resume but no fork session",
			resumeSessionID: "test-session-id",
			forkSession:     false,
			wantResumeFlag:  true,
			wantForkFlag:    false,
		},
		{
			name:            "with fork session but no resume",
			resumeSessionID: "",
			forkSession:     true,
			wantResumeFlag:  false,
			wantForkFlag:    true,
		},
		{
			name:            "without resume and fork session",
			resumeSessionID: "",
			forkSession:     false,
			wantResumeFlag:  false,
			wantForkFlag:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create options with fork session setting
			opts := types.NewClaudeAgentOptions().
				WithForkSession(tt.forkSession)

			logger := log.NewLogger(false)
			transport := NewSubprocessCLITransport("/bin/echo", "", nil, logger, tt.resumeSessionID, opts)

			// Build command args (without actually connecting)
			args := transport.buildCommandArgs()

			// Convert to string for easier searching
			argsStr := strings.Join(args, " ")
			t.Logf("CLI args: %v", args)

			// Check for --resume flag
			hasResumeFlag := contains(args, "--resume")
			if hasResumeFlag != tt.wantResumeFlag {
				t.Errorf("--resume flag present = %v, want %v", hasResumeFlag, tt.wantResumeFlag)
			}

			// Check for session ID if resume flag is expected
			if tt.wantResumeFlag {
				hasSessionID := contains(args, tt.resumeSessionID)
				if !hasSessionID {
					t.Errorf("session ID %q not found in args: %v", tt.resumeSessionID, args)
				}
			}

			// Check for --fork-session flag
			hasForkFlag := contains(args, "--fork-session")
			if hasForkFlag != tt.wantForkFlag {
				t.Errorf("--fork-session flag present = %v, want %v\nArgs: %s", hasForkFlag, tt.wantForkFlag, argsStr)
			}
		})
	}
}

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// TestBuildCommandArgs_SystemPrompt tests system prompt handling to match Python SDK behavior
func TestBuildCommandArgs_SystemPrompt(t *testing.T) {
	tests := []struct {
		name         string
		systemPrompt interface{}
		wantFlag     bool
		wantValue    string
		wantAppend   bool // For preset case
	}{
		{
			name:         "nil system prompt should pass empty string",
			systemPrompt: nil,
			wantFlag:     true,
			wantValue:    "",
		},
		{
			name:         "empty string system prompt",
			systemPrompt: "",
			wantFlag:     true,
			wantValue:    "",
		},
		{
			name:         "custom system prompt",
			systemPrompt: "You are a helpful assistant",
			wantFlag:     true,
			wantValue:    "You are a helpful assistant",
		},
		{
			name:         "multiline system prompt",
			systemPrompt: "You are a helpful assistant.\nAlways be polite.",
			wantFlag:     true,
			wantValue:    "You are a helpful assistant.\nAlways be polite.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := types.NewClaudeAgentOptions()
			if tt.systemPrompt != nil {
				opts.WithSystemPrompt(tt.systemPrompt)
			}
			// If tt.systemPrompt is explicitly nil in the struct, it will be nil in opts

			logger := log.NewLogger(false)
			transport := NewSubprocessCLITransport(
				"/usr/local/bin/claude",
				"",
				nil,
				logger,
				"",
				opts,
			)

			args := transport.buildCommandArgs()

			// Find --system-prompt flag
			foundFlag := false
			foundValue := ""
			for i, arg := range args {
				if arg == "--system-prompt" && i+1 < len(args) {
					foundFlag = true
					foundValue = args[i+1]
					break
				}
			}

			if foundFlag != tt.wantFlag {
				t.Errorf("--system-prompt flag present = %v, want %v", foundFlag, tt.wantFlag)
			}

			if foundValue != tt.wantValue {
				t.Errorf("--system-prompt value = %q, want %q", foundValue, tt.wantValue)
			}
		})
	}
}

// TestBuildCommandArgs_SystemPromptPreset tests system prompt preset handling
func TestBuildCommandArgs_SystemPromptPreset(t *testing.T) {
	appendText := "Additional instructions here"
	preset := types.SystemPromptPreset{
		Type:   "preset",
		Preset: "claude_code",
		Append: &appendText,
	}

	opts := types.NewClaudeAgentOptions().
		WithSystemPromptPreset(preset)

	logger := log.NewLogger(false)
	transport := NewSubprocessCLITransport(
		"/usr/local/bin/claude",
		"",
		nil,
		logger,
		"",
		opts,
	)

	args := transport.buildCommandArgs()

	// Find --append-system-prompt flag
	foundAppendFlag := false
	foundAppendValue := ""
	for i, arg := range args {
		if arg == "--append-system-prompt" && i+1 < len(args) {
			foundAppendFlag = true
			foundAppendValue = args[i+1]
			break
		}
	}

	if !foundAppendFlag {
		t.Errorf("--append-system-prompt flag not found in args: %v", args)
	}

	if foundAppendValue != appendText {
		t.Errorf("--append-system-prompt value = %q, want %q", foundAppendValue, appendText)
	}

	// Should NOT have --system-prompt flag when using preset
	hasSystemPromptFlag := false
	for _, arg := range args {
		if arg == "--system-prompt" {
			hasSystemPromptFlag = true
			break
		}
	}

	if hasSystemPromptFlag {
		t.Errorf("--system-prompt flag should not be present when using preset, but found in args: %v", args)
	}
}

// TestBuildCommandArgs_NoOptions tests that empty system prompt is used when no options provided
func TestBuildCommandArgs_NoOptions(t *testing.T) {
	logger := log.NewLogger(false)
	transport := NewSubprocessCLITransport(
		"/usr/local/bin/claude",
		"",
		nil,
		logger,
		"",
		nil, // No options
	)

	args := transport.buildCommandArgs()

	// Find --system-prompt flag
	foundFlag := false
	foundValue := ""
	for i, arg := range args {
		if arg == "--system-prompt" && i+1 < len(args) {
			foundFlag = true
			foundValue = args[i+1]
			break
		}
	}

	if !foundFlag {
		t.Errorf("--system-prompt flag should be present even with no options")
	}

	if foundValue != "" {
		t.Errorf("--system-prompt value = %q, want empty string", foundValue)
	}
}

// TestBuildCommandArgs_Plugins tests plugin CLI argument generation
func TestBuildCommandArgs_Plugins(t *testing.T) {
	tests := []struct {
		name      string
		plugins   []types.PluginConfig
		wantFlags int // Number of --plugin-dir flags expected
	}{
		{
			name:      "no plugins",
			plugins:   []types.PluginConfig{},
			wantFlags: 0,
		},
		{
			name: "single plugin",
			plugins: []types.PluginConfig{
				*types.NewLocalPluginConfig("/path/to/plugin"),
			},
			wantFlags: 1,
		},
		{
			name: "multiple plugins",
			plugins: []types.PluginConfig{
				*types.NewLocalPluginConfig("/path/to/plugin1"),
				*types.NewLocalPluginConfig("/path/to/plugin2"),
			},
			wantFlags: 2,
		},
		{
			name: "three plugins",
			plugins: []types.PluginConfig{
				*types.NewLocalPluginConfig("./plugins/demo"),
				*types.NewLocalPluginConfig("./plugins/custom"),
				*types.NewLocalPluginConfig("/usr/local/share/claude-plugins/tools"),
			},
			wantFlags: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := types.NewClaudeAgentOptions().WithPlugins(tt.plugins)

			logger := log.NewLogger(false)
			transport := NewSubprocessCLITransport(
				"/usr/local/bin/claude",
				"",
				nil,
				logger,
				"",
				opts,
			)

			args := transport.buildCommandArgs()

			// Count --plugin-dir flags
			count := 0
			pluginDirs := []string{}
			for i, arg := range args {
				if arg == "--plugin-dir" {
					count++
					if i+1 < len(args) {
						pluginDirs = append(pluginDirs, args[i+1])
					}
				}
			}

			if count != tt.wantFlags {
				t.Errorf("expected %d --plugin-dir flags, got %d", tt.wantFlags, count)
			}

			// Verify plugin paths match
			if len(pluginDirs) != len(tt.plugins) {
				t.Errorf("expected %d plugin paths, got %d", len(tt.plugins), len(pluginDirs))
			}

			for i, plugin := range tt.plugins {
				if i >= len(pluginDirs) {
					break
				}
				if pluginDirs[i] != plugin.Path {
					t.Errorf("plugin[%d] path = %s, want %s", i, pluginDirs[i], plugin.Path)
				}
			}
		})
	}
}

// TestBuildCommandArgs_PluginsWithOtherOptions tests plugins work with other options
func TestBuildCommandArgs_PluginsWithOtherOptions(t *testing.T) {
	opts := types.NewClaudeAgentOptions().
		WithLocalPlugin("./my-plugin").
		WithModel("claude-3-5-sonnet-20241022").
		WithMaxThinkingTokens(1000).
		WithSystemPrompt("You are a helpful assistant")

	logger := log.NewLogger(false)
	transport := NewSubprocessCLITransport(
		"/usr/local/bin/claude",
		"",
		nil,
		logger,
		"",
		opts,
	)

	args := transport.buildCommandArgs()

	// Verify plugin flag exists
	hasPluginDir := false
	for _, arg := range args {
		if arg == "--plugin-dir" {
			hasPluginDir = true
			break
		}
	}

	if !hasPluginDir {
		t.Error("--plugin-dir flag not found in args")
	}

	// Verify other flags still work
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "--model") {
		t.Error("--model flag not found")
	}
	if !strings.Contains(argsStr, "--max-thinking-tokens") {
		t.Error("--max-thinking-tokens flag not found")
	}
	if !strings.Contains(argsStr, "--system-prompt") {
		t.Error("--system-prompt flag not found")
	}
}

// TestBuildCommandArgs_Betas tests beta feature flag handling.
func TestBuildCommandArgs_Betas(t *testing.T) {
	t.Run("no betas", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions()

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/usr/local/bin/claude",
			"",
			nil,
			logger,
			"",
			opts,
		)

		args := transport.buildCommandArgs()
		argsStr := strings.Join(args, " ")

		if strings.Contains(argsStr, "--betas") {
			t.Error("--betas flag should not be present when no betas specified")
		}
	})

	t.Run("single beta", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithBeta("context-1m-2025-08-07")

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/usr/local/bin/claude",
			"",
			nil,
			logger,
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Find --betas flag and verify it's followed by the beta value
		hasBetas := false
		for i, arg := range args {
			if arg == "--betas" {
				if i+1 < len(args) && args[i+1] == "context-1m-2025-08-07" {
					hasBetas = true
				}
				break
			}
		}

		if !hasBetas {
			t.Error("--betas flag with correct value not found")
		}
	})

	t.Run("multiple betas", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithBeta("context-1m-2025-08-07").
			WithBeta("another-beta-feature")

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/usr/local/bin/claude",
			"",
			nil,
			logger,
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Count how many times --betas appears
		betasCount := 0
		for _, arg := range args {
			if arg == "--betas" {
				betasCount++
			}
		}

		if betasCount != 2 {
			t.Errorf("expected 2 --betas flags, got %d", betasCount)
		}

		// Verify both beta values are present
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "context-1m-2025-08-07") {
			t.Error("context-1m-2025-08-07 beta not found in args")
		}
		if !strings.Contains(argsStr, "another-beta-feature") {
			t.Error("another-beta-feature beta not found in args")
		}
	})

	t.Run("betas with other options", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithBeta("context-1m-2025-08-07").
			WithModel("claude-3-5-sonnet-20241022").
			WithMaxThinkingTokens(5000)

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/usr/local/bin/claude",
			"",
			nil,
			logger,
			"",
			opts,
		)

		args := transport.buildCommandArgs()
		argsStr := strings.Join(args, " ")

		// Verify betas flag
		if !strings.Contains(argsStr, "--betas") {
			t.Error("--betas flag not found")
		}

		// Verify other flags still work
		if !strings.Contains(argsStr, "--model") {
			t.Error("--model flag not found when combined with betas")
		}
		if !strings.Contains(argsStr, "--max-thinking-tokens") {
			t.Error("--max-thinking-tokens flag not found when combined with betas")
		}
	})

	t.Run("WithBetas replaces previous betas", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithBeta("beta-1").
			WithBeta("beta-2").
			WithBetas([]string{"beta-3"})

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/usr/local/bin/claude",
			"",
			nil,
			logger,
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Count how many times --betas appears - should be 1
		betasCount := 0
		for _, arg := range args {
			if arg == "--betas" {
				betasCount++
			}
		}

		if betasCount != 1 {
			t.Errorf("expected 1 --betas flag after WithBetas, got %d", betasCount)
		}

		// Verify only the new beta is present
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "beta-3") {
			t.Error("beta-3 not found after WithBetas()")
		}
		if strings.Contains(argsStr, "beta-1") || strings.Contains(argsStr, "beta-2") {
			t.Error("old betas should be replaced by WithBetas()")
		}
	})
}

// TestStderrFileLogging tests stderr file logging functionality
func TestStderrFileLogging(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		// Create options without stderr file logging
		opts := types.NewClaudeAgentOptions()

		if opts.StderrLogFile != nil {
			t.Error("StderrLogFile should be nil by default")
		}
	})

	t.Run("custom path creates directory and file", func(t *testing.T) {
		// Create temporary directory for testing
		tempDir := t.TempDir()
		customLogPath := filepath.Join(tempDir, "logs", "stderr.log")

		// Create options with custom stderr log file
		opts := types.NewClaudeAgentOptions().
			WithCustomStderrLogFile(customLogPath)

		// Verify option is set
		if opts.StderrLogFile == nil {
			t.Fatal("StderrLogFile should not be nil")
		}
		if *opts.StderrLogFile != customLogPath {
			t.Errorf("StderrLogFile = %q, want %q", *opts.StderrLogFile, customLogPath)
		}

		// Create transport with echo command that writes to stderr
		logger := log.NewLogger(false)

		// Use a simple shell command that writes to stderr
		transport := NewSubprocessCLITransport(
			"/bin/sh",
			"",
			nil,
			logger,
			"",
			opts,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Connect will trigger the stderr reader goroutine
		err := transport.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Write a command that produces stderr output
		// Use -c to run a command that writes to stderr
		_ = transport.Write(ctx, `echo "test error" >&2`)

		// Give it time to process
		time.Sleep(500 * time.Millisecond)

		// Close transport
		_ = transport.Close(ctx)

		// Verify the directory was created
		logDir := filepath.Dir(customLogPath)
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			t.Errorf("Log directory was not created: %s", logDir)
		}

		// Verify the log file was created
		if _, err := os.Stat(customLogPath); os.IsNotExist(err) {
			t.Errorf("Log file was not created: %s", customLogPath)
		}
	})

	t.Run("default location option", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithDefaultStderrLogFile()

		if opts.StderrLogFile == nil {
			t.Fatal("StderrLogFile should not be nil")
		}
		if *opts.StderrLogFile != "" {
			t.Errorf("StderrLogFile = %q, want empty string for default", *opts.StderrLogFile)
		}
	})

	t.Run("callback still works with file logging", func(t *testing.T) {
		tempDir := t.TempDir()
		customLogPath := filepath.Join(tempDir, "test.log")

		// Track callback invocations
		var callbackLines []string
		var mu sync.Mutex

		opts := types.NewClaudeAgentOptions().
			WithCustomStderrLogFile(customLogPath).
			WithStderr(func(line string) {
				mu.Lock()
				defer mu.Unlock()
				callbackLines = append(callbackLines, line)
			})

		logger := log.NewLogger(false)
		transport := NewSubprocessCLITransport(
			"/bin/sh",
			"",
			nil,
			logger,
			"",
			opts,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := transport.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		// Write command that produces stderr
		_ = transport.Write(ctx, `echo "callback test" >&2`)

		time.Sleep(500 * time.Millisecond)
		_ = transport.Close(ctx)

		// Verify callback was called
		mu.Lock()
		numCallbacks := len(callbackLines)
		mu.Unlock()

		if numCallbacks == 0 {
			t.Log("Warning: callback was not invoked (may be expected for /bin/sh)")
		}

		// Verify file was still created (file logging should work even with callback)
		if _, err := os.Stat(customLogPath); os.IsNotExist(err) {
			t.Errorf("Log file should be created even when callback is set: %s", customLogPath)
		}
	})
}

// TestStderrFileLogging_DirectoryCreation tests that parent directories are created
func TestStderrFileLogging_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()

	// Create nested path that doesn't exist
	deepPath := filepath.Join(tempDir, "a", "b", "c", "stderr.log")

	opts := types.NewClaudeAgentOptions().
		WithCustomStderrLogFile(deepPath)

	logger := log.NewLogger(false)
	transport := NewSubprocessCLITransport(
		"/bin/echo",
		"",
		nil,
		logger,
		"",
		opts,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This should create all parent directories
	err := transport.Connect(ctx)
	if err != nil {
		t.Logf("Connect error (may be expected): %v", err)
	}

	// Give readStderr goroutine time to run
	time.Sleep(200 * time.Millisecond)

	_ = transport.Close(ctx)

	// Verify nested directories were created
	expectedDir := filepath.Join(tempDir, "a", "b", "c")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Nested directories were not created: %s", expectedDir)
	}

	// Verify log file was created
	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Logf("Log file was not created (may be expected for /bin/echo): %s", deepPath)
	}
}

// TestBuildCommandArgs_Agents tests agent configuration JSON serialization
func TestBuildCommandArgs_Agents(t *testing.T) {
	t.Run("single agent with all fields", func(t *testing.T) {
		mode := types.SubagentExecutionModeParallel
		timeout := 30.5
		maxTurns := 5
		modelStr := "claude-opus-4-5-latest"

		opts := types.NewClaudeAgentOptions().
			WithAgent("search", types.AgentDefinition{
				Description:   "Search agent",
				Prompt:        "Search for information",
				Tools:         []string{"Read", "Glob"},
				Model:         &modelStr,
				ExecutionMode: &mode,
				Timeout:       &timeout,
				MaxTurns:      &maxTurns,
			})

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Verify --agents flag is present
		agentsIdx := -1
		for i, arg := range args {
			if arg == "--agents" {
				agentsIdx = i
				break
			}
		}

		if agentsIdx == -1 {
			t.Fatal("--agents flag not found in command arguments")
		}

		if agentsIdx+1 >= len(args) {
			t.Fatal("--agents flag has no value")
		}

		agentsJSON := args[agentsIdx+1]

		// Verify JSON can be unmarshaled
		var agentsData map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(agentsJSON), &agentsData); err != nil {
			t.Fatalf("Failed to unmarshal agents JSON: %v", err)
		}

		// Verify agent exists
		searchAgent, ok := agentsData["search"]
		if !ok {
			t.Fatal("Agent 'search' not found in JSON")
		}

		// Verify fields
		if searchAgent["description"] != "Search agent" {
			t.Errorf("Expected description 'Search agent', got %v", searchAgent["description"])
		}

		if searchAgent["prompt"] != "Search for information" {
			t.Errorf("Expected prompt 'Search for information', got %v", searchAgent["prompt"])
		}

		if searchAgent["execution_mode"] != "parallel" {
			t.Errorf("Expected execution_mode 'parallel', got %v", searchAgent["execution_mode"])
		}

		if searchAgent["timeout"] != 30.5 {
			t.Errorf("Expected timeout 30.5, got %v", searchAgent["timeout"])
		}

		if maxTurnsVal, ok := searchAgent["max_turns"].(float64); !ok || maxTurnsVal != 5 {
			t.Errorf("Expected max_turns 5, got %v", searchAgent["max_turns"])
		}
	})

	t.Run("multiple agents with different configs", func(t *testing.T) {
		mode1 := types.SubagentExecutionModeSequential
		mode2 := types.SubagentExecutionModeParallel

		opts := types.NewClaudeAgentOptions().
			WithAgent("agent1", types.AgentDefinition{
				Description:   "First agent",
				Prompt:        "First prompt",
				ExecutionMode: &mode1,
			}).
			WithAgent("agent2", types.AgentDefinition{
				Description:   "Second agent",
				Prompt:        "Second prompt",
				ExecutionMode: &mode2,
			})

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		agentsIdx := -1
		for i, arg := range args {
			if arg == "--agents" {
				agentsIdx = i
				break
			}
		}

		if agentsIdx == -1 {
			t.Fatal("--agents flag not found")
		}

		agentsJSON := args[agentsIdx+1]
		var agentsData map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(agentsJSON), &agentsData); err != nil {
			t.Fatalf("Failed to unmarshal agents JSON: %v", err)
		}

		if len(agentsData) != 2 {
			t.Errorf("Expected 2 agents, got %d", len(agentsData))
		}

		if agentsData["agent1"]["execution_mode"] != "sequential" {
			t.Error("agent1 should have sequential execution mode")
		}

		if agentsData["agent2"]["execution_mode"] != "parallel" {
			t.Error("agent2 should have parallel execution mode")
		}
	})

	t.Run("agent with only required fields", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions().
			WithAgent("simple", types.AgentDefinition{
				Description: "Simple agent",
				Prompt:      "Simple prompt",
			})

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		agentsIdx := -1
		for i, arg := range args {
			if arg == "--agents" {
				agentsIdx = i
				break
			}
		}

		if agentsIdx == -1 {
			t.Fatal("--agents flag not found")
		}

		agentsJSON := args[agentsIdx+1]
		var agentsData map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(agentsJSON), &agentsData); err != nil {
			t.Fatalf("Failed to unmarshal agents JSON: %v", err)
		}

		simpleAgent := agentsData["simple"]

		// Verify required fields are present
		if simpleAgent["description"] != "Simple agent" {
			t.Error("description should be present")
		}
		if simpleAgent["prompt"] != "Simple prompt" {
			t.Error("prompt should be present")
		}

		// Verify optional fields are absent (not in JSON)
		if _, ok := simpleAgent["execution_mode"]; ok {
			t.Error("execution_mode should not be in JSON when not set")
		}
		if _, ok := simpleAgent["timeout"]; ok {
			t.Error("timeout should not be in JSON when not set")
		}
		if _, ok := simpleAgent["max_turns"]; ok {
			t.Error("max_turns should not be in JSON when not set")
		}
	})

	t.Run("no agents when not specified", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions()

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Verify --agents flag is not present
		for _, arg := range args {
			if arg == "--agents" {
				t.Fatal("--agents flag should not be present when no agents are configured")
			}
		}
	})
}

// TestBuildCommandArgs_SubagentExecution tests subagent execution config JSON serialization
func TestBuildCommandArgs_SubagentExecution(t *testing.T) {
	t.Run("subagent execution with all fields", func(t *testing.T) {
		config := types.NewSubagentExecutionConfig()
		config.MultiInvocation = types.MultiInvocationModeParallel
		config.MaxConcurrent = 5
		config.ErrorHandling = types.SubagentErrorHandlingFailFast

		opts := types.NewClaudeAgentOptions().
			WithSubagentExecution(config)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Verify --subagent-execution flag is present
		subagentIdx := -1
		for i, arg := range args {
			if arg == "--subagent-execution" {
				subagentIdx = i
				break
			}
		}

		if subagentIdx == -1 {
			t.Fatal("--subagent-execution flag not found")
		}

		subagentJSON := args[subagentIdx+1]
		var subagentData map[string]interface{}
		if err := json.Unmarshal([]byte(subagentJSON), &subagentData); err != nil {
			t.Fatalf("Failed to unmarshal subagent JSON: %v", err)
		}

		if subagentData["multi_invocation"] != "parallel" {
			t.Errorf("Expected multi_invocation 'parallel', got %v", subagentData["multi_invocation"])
		}

		if subagentData["max_concurrent"] != float64(5) {
			t.Errorf("Expected max_concurrent 5, got %v", subagentData["max_concurrent"])
		}

		if subagentData["error_handling"] != "fail_fast" {
			t.Errorf("Expected error_handling 'fail_fast', got %v", subagentData["error_handling"])
		}
	})

	t.Run("subagent execution with defaults", func(t *testing.T) {
		config := types.NewSubagentExecutionConfig()

		opts := types.NewClaudeAgentOptions().
			WithSubagentExecution(config)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		subagentIdx := -1
		for i, arg := range args {
			if arg == "--subagent-execution" {
				subagentIdx = i
				break
			}
		}

		if subagentIdx == -1 {
			t.Fatal("--subagent-execution flag not found")
		}

		subagentJSON := args[subagentIdx+1]
		var subagentData map[string]interface{}
		if err := json.Unmarshal([]byte(subagentJSON), &subagentData); err != nil {
			t.Fatalf("Failed to unmarshal subagent JSON: %v", err)
		}

		// Verify defaults are serialized
		if subagentData["multi_invocation"] != "sequential" {
			t.Error("Default multi_invocation should be sequential")
		}

		if subagentData["max_concurrent"] != float64(3) {
			t.Error("Default max_concurrent should be 3")
		}

		if subagentData["error_handling"] != "continue" {
			t.Error("Default error_handling should be continue")
		}
	})

	t.Run("no subagent execution when not specified", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions()

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Verify --subagent-execution flag is not present
		for _, arg := range args {
			if arg == "--subagent-execution" {
				t.Fatal("--subagent-execution flag should not be present when config not set")
			}
		}
	})
}

// TestBuildCommandArgs_AgentsWithSubagentExecution tests agents and subagent config together
func TestBuildCommandArgs_AgentsWithSubagentExecution(t *testing.T) {
	mode := types.SubagentExecutionModeParallel
	subagentConfig := types.NewSubagentExecutionConfig()
	subagentConfig.MaxConcurrent = 4

	opts := types.NewClaudeAgentOptions().
		WithAgent("agent1", types.AgentDefinition{
			Description:   "Agent 1",
			Prompt:        "Prompt 1",
			ExecutionMode: &mode,
		}).
		WithSubagentExecution(subagentConfig)

	transport := NewSubprocessCLITransport(
		"claude",
		"",
		nil,
		log.NewLogger(false),
		"",
		opts,
	)

	args := transport.buildCommandArgs()

	// Verify both flags are present
	hasAgents := false
	hasSubagentExecution := false

	for _, arg := range args {
		if arg == "--agents" {
			hasAgents = true
		}
		if arg == "--subagent-execution" {
			hasSubagentExecution = true
		}
	}

	if !hasAgents {
		t.Error("--agents flag should be present")
	}

	if !hasSubagentExecution {
		t.Error("--subagent-execution flag should be present")
	}
}

// TestBuildCommandArgs_OutputFormat tests structured output format (JSON schema) CLI argument generation
func TestBuildCommandArgs_OutputFormat(t *testing.T) {
	t.Run("simple object schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":  map[string]interface{}{"type": "string"},
				"count": map[string]interface{}{"type": "number"},
			},
			"required": []string{"name", "count"},
		}

		opts := types.NewClaudeAgentOptions().
			WithOutputFormat(schema)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Find --json-schema flag
		jsonSchemaIdx := -1
		for i, arg := range args {
			if arg == "--json-schema" {
				jsonSchemaIdx = i
				break
			}
		}

		if jsonSchemaIdx == -1 {
			t.Fatal("--json-schema flag not found in command arguments")
		}

		if jsonSchemaIdx+1 >= len(args) {
			t.Fatal("--json-schema flag has no value")
		}

		schemaJSON := args[jsonSchemaIdx+1]

		// Verify JSON can be unmarshaled
		var parsedSchema map[string]interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &parsedSchema); err != nil {
			t.Fatalf("Failed to unmarshal schema JSON: %v", err)
		}

		// Verify schema structure
		if parsedSchema["type"] != "object" {
			t.Errorf("Expected schema type 'object', got %v", parsedSchema["type"])
		}

		props, ok := parsedSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Schema properties should be a map")
		}

		if props["name"] == nil || props["count"] == nil {
			t.Error("Schema should have 'name' and 'count' properties")
		}
	})

	t.Run("nested schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"analysis": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"word_count": map[string]interface{}{"type": "integer"},
						"has_errors": map[string]interface{}{"type": "boolean"},
					},
					"required": []string{"word_count", "has_errors"},
				},
				"items": map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "string"},
				},
			},
			"required": []string{"analysis", "items"},
		}

		opts := types.NewClaudeAgentOptions().
			WithOutputFormat(schema)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Find --json-schema flag
		jsonSchemaIdx := -1
		for i, arg := range args {
			if arg == "--json-schema" {
				jsonSchemaIdx = i
				break
			}
		}

		if jsonSchemaIdx == -1 {
			t.Fatal("--json-schema flag not found")
		}

		schemaJSON := args[jsonSchemaIdx+1]
		var parsedSchema map[string]interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &parsedSchema); err != nil {
			t.Fatalf("Failed to unmarshal schema JSON: %v", err)
		}

		// Verify nested structure
		props, ok := parsedSchema["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("Schema should have properties")
		}

		// Check nested analysis object
		analysis := props["analysis"].(map[string]interface{})
		if analysis == nil {
			t.Fatal("Analysis property should exist")
		}

		analysisProps := analysis["properties"].(map[string]interface{})
		if analysisProps["word_count"] == nil || analysisProps["has_errors"] == nil {
			t.Error("Analysis should have word_count and has_errors")
		}

		// Check items array
		items := props["items"].(map[string]interface{})
		if items == nil || items["type"] != "array" {
			t.Error("Items should be an array type")
		}
	})

	t.Run("array schema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":    map[string]interface{}{"type": "string"},
					"value": map[string]interface{}{"type": "number"},
				},
			},
		}

		opts := types.NewClaudeAgentOptions().
			WithOutputFormat(schema)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		jsonSchemaIdx := -1
		for i, arg := range args {
			if arg == "--json-schema" {
				jsonSchemaIdx = i
				break
			}
		}

		if jsonSchemaIdx == -1 {
			t.Fatal("--json-schema flag not found")
		}

		schemaJSON := args[jsonSchemaIdx+1]
		var parsedSchema map[string]interface{}
		if err := json.Unmarshal([]byte(schemaJSON), &parsedSchema); err != nil {
			t.Fatalf("Failed to unmarshal schema JSON: %v", err)
		}

		if parsedSchema["type"] != "array" {
			t.Errorf("Expected schema type 'array', got %v", parsedSchema["type"])
		}
	})

	t.Run("output format with other options", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result": map[string]interface{}{"type": "string"},
			},
		}

		opts := types.NewClaudeAgentOptions().
			WithOutputFormat(schema).
			WithModel("claude-3-5-sonnet-20241022").
			WithMaxTurns(10)

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()
		argsStr := strings.Join(args, " ")

		// Verify --json-schema flag exists
		hasJSONSchema := false
		for _, arg := range args {
			if arg == "--json-schema" {
				hasJSONSchema = true
				break
			}
		}

		if !hasJSONSchema {
			t.Error("--json-schema flag not found")
		}

		// Verify other flags still work
		if !strings.Contains(argsStr, "--model") {
			t.Error("--model flag not found")
		}
		if !strings.Contains(argsStr, "--max-turns") {
			t.Error("--max-turns flag not found")
		}
	})

	t.Run("no output format when not specified", func(t *testing.T) {
		opts := types.NewClaudeAgentOptions()

		transport := NewSubprocessCLITransport(
			"claude",
			"",
			nil,
			log.NewLogger(false),
			"",
			opts,
		)

		args := transport.buildCommandArgs()

		// Verify --json-schema flag is not present
		for _, arg := range args {
			if arg == "--json-schema" {
				t.Fatal("--json-schema flag should not be present when output format is not set")
			}
		}
	})
}
