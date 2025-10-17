package transport

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/schlunsen/claude-agent-sdk-go/internal/types"
)

const (
	// SDKVersion is the version identifier for this SDK
	SDKVersion = "0.1.0"
)

// SubprocessCLITransport implements Transport using a Claude Code CLI subprocess.
// It manages the subprocess lifecycle, stdin/stdout/stderr pipes, and message streaming.
type SubprocessCLITransport struct {
	cliPath string
	cwd     string
	env     map[string]string

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
func NewSubprocessCLITransport(cliPath, cwd string, env map[string]string) *SubprocessCLITransport {
	return &SubprocessCLITransport{
		cliPath:  cliPath,
		cwd:      cwd,
		env:      env,
		messages: make(chan types.Message, 10), // Buffered channel for smooth streaming
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

	// Create cancellable context
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Build command: claude agent --stdio
	t.cmd = exec.CommandContext(t.ctx, t.cliPath, "agent", "--stdio")

	// Set working directory if provided
	if t.cwd != "" {
		t.cmd.Dir = t.cwd
	}

	// Set up environment variables
	// Start with current environment
	t.cmd.Env = os.Environ()

	// Add SDK-specific variables
	t.cmd.Env = append(t.cmd.Env, "CLAUDE_CODE_ENTRYPOINT=agent")
	t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("CLAUDE_AGENT_SDK_VERSION=%s", SDKVersion))

	// Add custom environment variables
	for key, value := range t.env {
		t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("%s=%s", key, value))
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
		return types.NewCLIConnectionErrorWithCause("failed to start subprocess", err)
	}

	// Create JSON line writer for stdin
	t.writer = NewJSONLineWriter(t.stdin)

	// Launch message reader loop in goroutine
	go t.messageReaderLoop(t.ctx)

	// Mark as ready
	t.ready = true

	return nil
}

// messageReaderLoop reads JSON lines from stdout and parses them into messages.
// It runs in a goroutine and sends messages to the messages channel.
// It respects context cancellation and closes the messages channel when done.
func (t *SubprocessCLITransport) messageReaderLoop(ctx context.Context) {
	defer close(t.messages)

	reader := NewJSONLineReader(t.stdout)

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read next JSON line
		line, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				// Normal end of stream
				return
			}

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
			// Store parse error but continue reading
			t.OnError(err)
			continue
		}

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

	// Write JSON line (includes newline and flush)
	if err := t.writer.WriteLine(data); err != nil {
		t.ready = false
		t.err = types.NewCLIConnectionErrorWithCause("failed to write to subprocess stdin", err)
		return t.err
	}

	return nil
}

// ReadMessages returns a channel of incoming messages from the subprocess.
// The channel is closed when the subprocess exits or an error occurs.
func (t *SubprocessCLITransport) ReadMessages(ctx context.Context) <-chan types.Message {
	return t.messages
}

// Close terminates the subprocess and cleans up all resources.
// It attempts to gracefully shut down the subprocess with a timeout.
func (t *SubprocessCLITransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmd == nil {
		return nil // Not connected
	}

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
func (t *SubprocessCLITransport) readStderr(ctx context.Context) {
	if t.stderr == nil {
		return
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

		// Could log or handle stderr here
		// For now, we just read and discard
		_ = line
	}
}
