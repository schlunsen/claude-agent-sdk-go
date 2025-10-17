package transport

import (
	"bufio"
	"io"

	"github.com/schlunsen/claude-agent-sdk-go/types"
)

const (
	// DefaultMaxBufferSize is the default maximum size for JSON line buffer (1MB)
	DefaultMaxBufferSize = 1024 * 1024
)

// JSONLineReader reads JSON lines from an input stream with buffering.
// Each call to ReadLine returns the next complete JSON line (without newline).
type JSONLineReader struct {
	scanner *bufio.Scanner
	maxSize int
}

// NewJSONLineReader creates a new JSONLineReader with the default buffer size.
func NewJSONLineReader(r io.Reader) *JSONLineReader {
	return NewJSONLineReaderWithSize(r, DefaultMaxBufferSize)
}

// NewJSONLineReaderWithSize creates a new JSONLineReader with a custom max buffer size.
func NewJSONLineReaderWithSize(r io.Reader, maxSize int) *JSONLineReader {
	scanner := bufio.NewScanner(r)

	// Set up buffer with max size
	buf := make([]byte, 0, 64*1024) // Initial 64KB buffer
	scanner.Buffer(buf, maxSize)

	return &JSONLineReader{
		scanner: scanner,
		maxSize: maxSize,
	}
}

// ReadLine reads the next JSON line from the stream.
// Returns the raw JSON bytes (without newline) or an error.
// Returns io.EOF when the stream ends.
func (r *JSONLineReader) ReadLine() ([]byte, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			// Check if it's a buffer overflow error
			if err == bufio.ErrTooLong {
				return nil, types.NewJSONDecodeErrorWithRaw(
					"JSON line exceeded maximum buffer size",
					"",
				)
			}
			return nil, err
		}
		// No error, just EOF
		return nil, io.EOF
	}

	return r.scanner.Bytes(), nil
}

// JSONLineWriter writes JSON lines to an output stream with buffering.
// Each call to WriteLine writes the data followed by a newline and flushes.
type JSONLineWriter struct {
	writer *bufio.Writer
}

// NewJSONLineWriter creates a new JSONLineWriter with default buffer size.
func NewJSONLineWriter(w io.Writer) *JSONLineWriter {
	return &JSONLineWriter{
		writer: bufio.NewWriter(w),
	}
}

// WriteLine writes a JSON line to the stream with a trailing newline.
// The data is written to the buffer and then immediately flushed.
func (w *JSONLineWriter) WriteLine(data string) error {
	if _, err := w.writer.WriteString(data); err != nil {
		return err
	}

	if _, err := w.writer.WriteString("\n"); err != nil {
		return err
	}

	return w.writer.Flush()
}

// Flush flushes any buffered data to the underlying writer.
func (w *JSONLineWriter) Flush() error {
	return w.writer.Flush()
}
