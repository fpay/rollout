package rollout

import (
	"io"
)

// Buffer interface defines buffer's common behaviors used by Rollout. A Buffer must implement
// io.WriteCloser interface. A Flush method is used to flush data to underlying writer before
// Rollout closes.
type Buffer interface {
	io.WriteCloser

	// Flush writes any data in the buffer to underlying writer.
	Flush() error
}

type BufferWriter struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

// NewWriterSize returns a new Writer whose buffer has at least the specified
// size. If the argument io.Writer is already a Writer with large enough
// size, it returns the underlying Writer.
func NewWriterSize(w io.Writer, size int) *BufferWriter {
	// Is it already a Writer?
	b, ok := w.(*BufferWriter)
	if ok && len(b.buf) >= size {
		return b
	}
	if size <= 0 {
		size = defaultBufferSize
	}
	return &BufferWriter{
		buf: make([]byte, size),
		wr:  w,
	}
}

// Flush writes any buffered data to the underlying io.Writer.
func (b *BufferWriter) Flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}
	n, err := b.wr.Write(b.buf[0:b.n])
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

// Available returns how many bytes are unused in the buffer.
func (b *BufferWriter) Available() int { return len(b.buf) - b.n }

// Buffered returns the number of bytes that have been written into the current buffer.
func (b *BufferWriter) Buffered() int { return b.n }

// Write writes the contents of p into the buffer.
// It returns the number of bytes written.
// If nn < len(p), it also returns an error explaining
// why the write is short.
func (b *BufferWriter) Write(p []byte) (nn int, err error) {
	if len(p) > b.Available() && b.err == nil {
		var n int
		if b.Buffered() == 0 {
			// Large write, empty buffer.
			// Write directly from p to avoid copy.
			nn, b.err = b.wr.Write(p)
		} else {
			for len(p) > 0 {
				n = copy(b.buf[b.n:], p)
				b.n += n
				b.Flush()
				nn += n
				p = p[n:]
			}
		}
		if b.err != nil {
			return nn, b.err
		}
		return nn, nil
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}
