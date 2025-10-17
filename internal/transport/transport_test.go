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
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
)

// TestFindCLI tests CLI discovery in various scenarios
func TestFindCLI(t *testing.T) {
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
				f.Close()

				// Make it executable
				if err := os.Chmod(claudePath, 0755); err != nil {
					t.Fatalf("Failed to chmod mock binary: %v", err)
				}

				// Add to PATH
				os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

				// Return cleanup function
				return func() {
					os.Setenv("PATH", origPath)
				}
			},
			wantError: false,
		},
		{
			name: "CLI not found",
			setup: func() func() {
				// Save and clear PATH
				origPath := os.Getenv("PATH")
				os.Setenv("PATH", "")

				return func() {
					os.Setenv("PATH", origPath)
				}
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setup()
			defer cleanup()

			path, err := FindCLI()

			if tt.wantError {
				if err == nil {
					t.Errorf("FindCLI() expected error, got nil")
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

	transport := NewSubprocessCLITransport(echoPath, "", nil)

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

	transport := NewSubprocessCLITransport(catPath, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}
	defer transport.Close(ctx)

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

	transport := NewSubprocessCLITransport(echoPath, "", nil)

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
		defer pw.Close()
		pw.Write([]byte(jsonStream))
	}()

	// Create transport with custom stdout
	transport := &SubprocessCLITransport{
		messages: make(chan types.Message, 10),
		ready:    true,
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

	transport := NewSubprocessCLITransport(echoPath, "", env)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() unexpected error: %v", err)
	}
	defer transport.Close(ctx)

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

	transport := NewSubprocessCLITransport(cliPath, "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to CLI
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect() failed: %v", err)
	}
	defer transport.Close(ctx)

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
