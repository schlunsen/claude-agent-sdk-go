package transport

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/schlunsen/claude-agent-sdk-go/internal/log"
	"github.com/schlunsen/claude-agent-sdk-go/types"
)

// An oversized line must cost that one message, not the stream. The reader
// used to be built on bufio.Scanner, which stops permanently after its first
// ErrTooLong, so one large tool result ended the whole session.
func TestJSONLineReaderRecoversAfterOversizedLine(t *testing.T) {
	const maxSize = 64
	input := `{"type":"a"}` + "\n" +
		`{"data":"` + strings.Repeat("x", 500) + `"}` + "\n" +
		`{"type":"b"}` + "\n"
	r := NewJSONLineReaderWithSize(strings.NewReader(input), maxSize)

	if line, err := r.ReadLine(); err != nil || string(line) != `{"type":"a"}` {
		t.Fatalf("first line = %q, %v", line, err)
	}

	_, err := r.ReadLine()
	var tooLong *LineTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("oversized line: error = %v, want *LineTooLongError", err)
	}
	if tooLong.MaxSize != maxSize || tooLong.Size <= maxSize {
		t.Errorf("LineTooLongError = %+v, want MaxSize %d and Size > %d", tooLong, maxSize, maxSize)
	}

	if line, err := r.ReadLine(); err != nil || string(line) != `{"type":"b"}` {
		t.Fatalf("the stream must continue after an oversized line: got %q, %v", line, err)
	}
	if _, err := r.ReadLine(); err != io.EOF {
		t.Fatalf("want io.EOF at end of stream, got %v", err)
	}
}

// Lines longer than the internal read buffer but within the limit must come
// back intact; the reader assembles them from several reads.
func TestJSONLineReaderAssemblesLongLines(t *testing.T) {
	long := `{"data":"` + strings.Repeat("y", readChunkSize*3) + `"}`
	r := NewJSONLineReaderWithSize(strings.NewReader(long+"\n"+`{"n":2}`+"\n"), readChunkSize*4)

	if line, err := r.ReadLine(); err != nil || string(line) != long {
		t.Fatalf("long line: err = %v, got %d bytes, want %d", err, len(line), len(long))
	}
	if line, err := r.ReadLine(); err != nil || string(line) != `{"n":2}` {
		t.Fatalf("next line = %q, %v", line, err)
	}
}

// The limit applies to the line content, not its terminator.
func TestJSONLineReaderSizeBoundary(t *testing.T) {
	const maxSize = 16
	exact := strings.Repeat("a", maxSize)
	over := strings.Repeat("b", maxSize+1)
	r := NewJSONLineReaderWithSize(strings.NewReader(exact+"\r\n"+over+"\n"), maxSize)

	if line, err := r.ReadLine(); err != nil || string(line) != exact {
		t.Fatalf("a CRLF-terminated line of exactly the limit must be accepted: %q, %v", line, err)
	}
	var tooLong *LineTooLongError
	if _, err := r.ReadLine(); !errors.As(err, &tooLong) {
		t.Fatalf("a line one byte over the limit must be rejected, got %v", err)
	}
}

func TestJSONLineReaderFinalLineWithoutNewline(t *testing.T) {
	r := NewJSONLineReaderWithSize(strings.NewReader(`{"a":1}`+"\n"+`{"b":2}`), 64)
	for _, want := range []string{`{"a":1}`, `{"b":2}`} {
		if line, err := r.ReadLine(); err != nil || string(line) != want {
			t.Fatalf("got %q, %v, want %q", line, err, want)
		}
	}
	if _, err := r.ReadLine(); err != io.EOF {
		t.Fatalf("want io.EOF, got %v", err)
	}

	// An oversized final line is reported, then the stream ends cleanly.
	r = NewJSONLineReaderWithSize(strings.NewReader(strings.Repeat("z", 100)), 64)
	var tooLong *LineTooLongError
	if _, err := r.ReadLine(); !errors.As(err, &tooLong) {
		t.Fatalf("oversized final line: want *LineTooLongError, got %v", err)
	}
	if _, err := r.ReadLine(); err != io.EOF {
		t.Fatalf("want io.EOF after the oversized final line, got %v", err)
	}
}

// ClaudeAgentOptions.MaxBufferSize existed but the transport never read it,
// so WithMaxBufferSize had no effect.
func TestTransportHonoursMaxBufferSizeOption(t *testing.T) {
	logger := log.NewLogger(false)

	tr := NewSubprocessCLITransport("claude", "", nil, logger, "", nil)
	if got := tr.maxBufferSize(); got != DefaultMaxBufferSize {
		t.Errorf("no options: maxBufferSize() = %d, want %d", got, DefaultMaxBufferSize)
	}

	size := 4096
	tr = NewSubprocessCLITransport("claude", "", nil, logger, "", &types.ClaudeAgentOptions{MaxBufferSize: &size})
	if got := tr.maxBufferSize(); got != size {
		t.Errorf("WithMaxBufferSize(%d): maxBufferSize() = %d", size, got)
	}
}

// Close cancels the command's context, and the command runs under
// exec.CommandContext, so Close kills the CLI itself. That kill used to come
// back as "subprocess exited with error (exit code: -1)" on every normal close.
func TestCloseDoesNotReportItsOwnKill(t *testing.T) {
	cli, err := FindMockCLI(t)
	if err != nil {
		t.Skip("no mock CLI available")
	}
	tr := NewSubprocessCLITransport(cli, "", nil, log.NewLogger(false), "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	if err := tr.Close(ctx); err != nil {
		t.Fatalf("Close() must not report the shutdown it performed itself, got: %v", err)
	}
}

// A real failure must still be reported: only the kill Close performs is
// treated as a clean shutdown.
func TestCloseStillReportsRealExitFailures(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	path := filepath.Join(t.TempDir(), "failing-cli")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tr := NewSubprocessCLITransport(path, "", nil, log.NewLogger(false), "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	// Wait until the CLI has exited on its own before closing. Its stdout hits
	// EOF when it exits, and the reader closes the message channel on EOF. A
	// sleep is not enough: on a cold start Close can still win the race and
	// kill the script, which is exactly the shutdown Close is right to ignore.
	msgs := tr.ReadMessages(ctx)
	for open := true; open; {
		select {
		case _, open = <-msgs:
		case <-ctx.Done():
			t.Fatal("the CLI did not exit on its own")
		}
	}

	err := tr.Close(ctx)
	if err == nil || !strings.Contains(err.Error(), "exit code: 3") {
		t.Fatalf("Close() must still report a CLI that exited with status 3, got: %v", err)
	}
}
