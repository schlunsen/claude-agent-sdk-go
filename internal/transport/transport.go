package transport

import (
	"context"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// Transport defines the interface for communicating with Claude Code CLI subprocess.
// This is a low-level transport interface that handles raw I/O with the Claude process.
// The Query class builds on top of this to implement the control protocol and message routing.
type Transport interface {
	// Connect establishes connection to Claude Code CLI subprocess.
	// For subprocess transports, this starts the process and prepares stdin/stdout/stderr pipes.
	Connect(ctx context.Context) error

	// Close terminates subprocess and cleans up resources.
	// This should gracefully shut down the subprocess and clean up all goroutines.
	Close(ctx context.Context) error

	// Write sends a JSON message to the subprocess stdin.
	// The data should be a complete JSON line (without the trailing newline - it will be added).
	Write(ctx context.Context, data string) error

	// ReadMessages returns a channel of incoming messages from subprocess stdout.
	// The channel is closed when the subprocess exits or an error occurs.
	// Messages are parsed from JSON lines and returned as Message interface types.
	ReadMessages(ctx context.Context) <-chan types.Message

	// OnError is called when an error occurs in the reading loop.
	// Implementations can use this to store errors for later retrieval.
	OnError(err error)

	// IsReady checks if the transport is ready for communication.
	// Returns true if the subprocess is running and ready to send/receive messages.
	IsReady() bool
}
