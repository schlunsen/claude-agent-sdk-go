package types

import "context"

// Transport defines the interface for communicating with Claude Code CLI.
// This is exposed publicly so that callers can implement custom transports
// or mock the transport layer in tests.
//
// The default implementation (SubprocessCLITransport) spawns the Claude CLI
// as a subprocess and communicates via stdin/stdout JSON lines.
type Transport interface {
	// Connect establishes a connection to the Claude Code CLI.
	// For subprocess transports, this starts the process and prepares I/O pipes.
	Connect(ctx context.Context) error

	// Close terminates the connection and cleans up resources.
	Close(ctx context.Context) error

	// Write sends a JSON message to the CLI.
	// The data should be a complete JSON object (trailing newline is added automatically).
	Write(ctx context.Context, data string) error

	// ReadMessages returns a channel of incoming messages from the CLI.
	// The channel is closed when the process exits or an error occurs.
	ReadMessages(ctx context.Context) <-chan Message

	// IsReady returns true if the transport is connected and ready for communication.
	IsReady() bool

	// GetError returns any error that occurred during transport operation.
	GetError() error
}
