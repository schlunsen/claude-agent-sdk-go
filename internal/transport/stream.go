package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

const (
	// DefaultMaxBufferSize is the default maximum size of a single JSON line
	// from the CLI (32MB). Override it per client with
	// ClaudeAgentOptions.WithMaxBufferSize.
	//
	// It used to be 1MB, which real sessions exceed routinely: a tool result
	// carrying an image or attachment arrives as one line, and 4-5MB lines are
	// ordinary. Exceeding the limit is no longer fatal (see LineTooLongError),
	// but each overflow still drops a message, so the default leaves generous
	// headroom over observed traffic.
	DefaultMaxBufferSize = 32 * 1024 * 1024

	// readChunkSize is the underlying read buffer. Longer lines are assembled
	// from several reads.
	readChunkSize = 64 * 1024
)

// LineTooLongError reports a line longer than the reader's maximum size.
//
// The oversized line has already been consumed when this is returned, so the
// next ReadLine resumes with the line after it: an oversized message costs that
// one message, not the stream. The reader used to be built on bufio.Scanner,
// which stops for good after its first ErrTooLong, so a single large message
// ended the whole session.
type LineTooLongError struct {
	// Size is the number of bytes read for the dropped line, including its
	// line terminator.
	Size int
	// MaxSize is the limit that was exceeded.
	MaxSize int
}

func (e *LineTooLongError) Error() string {
	return fmt.Sprintf("JSON line exceeded maximum buffer size (%d bytes, limit %d)", e.Size, e.MaxSize)
}

// JSONLineReader reads newline-delimited JSON from a stream.
type JSONLineReader struct {
	reader  *bufio.Reader
	maxSize int
}

// NewJSONLineReader creates a JSONLineReader with DefaultMaxBufferSize.
func NewJSONLineReader(r io.Reader) *JSONLineReader {
	return NewJSONLineReaderWithSize(r, DefaultMaxBufferSize)
}

// NewJSONLineReaderWithSize creates a JSONLineReader that rejects lines longer
// than maxSize bytes. A non-positive maxSize selects DefaultMaxBufferSize.
func NewJSONLineReaderWithSize(r io.Reader, maxSize int) *JSONLineReader {
	if maxSize <= 0 {
		maxSize = DefaultMaxBufferSize
	}
	return &JSONLineReader{
		reader:  bufio.NewReaderSize(r, readChunkSize),
		maxSize: maxSize,
	}
}

// ReadLine returns the next line, without its "\n" or "\r\n" terminator. The
// returned slice belongs to the caller.
//
// A line longer than the maximum size is consumed and discarded, and ReadLine
// returns a *LineTooLongError; the next call continues with the following line.
// ReadLine returns io.EOF when the stream ends.
func (r *JSONLineReader) ReadLine() ([]byte, error) {
	var (
		line    []byte
		size    int
		tooLong bool
	)
	for {
		chunk, err := r.reader.ReadSlice('\n')
		size += len(chunk)

		// Stop buffering once the line is certainly too long, but keep reading
		// to its end so the next call starts on a line boundary. The +2 leaves
		// room for a "\r\n" terminator, which is trimmed before the exact check.
		if !tooLong {
			if len(line)+len(chunk) > r.maxSize+2 {
				tooLong = true
				line = nil
			} else {
				line = append(line, chunk...)
			}
		}

		switch {
		case err == bufio.ErrBufferFull:
			continue // the line carries on in the next chunk
		case err == io.EOF:
			if size == 0 {
				return nil, io.EOF
			}
			// Otherwise this is a final line with no trailing newline.
		case err != nil:
			return nil, err
		}

		if !tooLong {
			line = trimLineEnding(line)
			tooLong = len(line) > r.maxSize
		}
		if tooLong {
			return nil, &LineTooLongError{Size: size, MaxSize: r.maxSize}
		}
		return line, nil
	}
}

// trimLineEnding drops a trailing "\n" and then a trailing "\r", as
// bufio.ScanLines does.
func trimLineEnding(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	return bytes.TrimSuffix(b, []byte("\r"))
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
