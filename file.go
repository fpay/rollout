package rollout

import (
	"os"
	"sync"
	"time"
)

// FileBuffer is a thread safe file writer with buffer. It is used to reduce disk IO.
// Guarantee atomic in single process writing situation.
type FileBuffer struct {
	f     *os.File
	timer *time.Timer

	mux sync.RWMutex
	w   *BufferWriter
}

// NewFileBuffer creates a new FileBuffer instance.
func NewFileBuffer(dest string, size int, interval time.Duration) (Buffer, error) {
	f, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	b := FileBuffer{
		w: NewWriterSize(f, size),
		f: f,
	}

	b.flushAtInterval(interval)

	return &b, nil
}

// Write writes contents of p into the buffer.
func (b *FileBuffer) Write(p []byte) (int, error) {
	b.mux.Lock()
	defer b.mux.Unlock()

	return b.w.Write(p)
}

// Flush writes buffered data to file.
func (b *FileBuffer) Flush() error {
	b.mux.Lock()
	defer b.mux.Unlock()

	return b.w.Flush()
}

// Close stops timer, flushes data, and closes the file.
func (b *FileBuffer) Close() error {
	b.mux.Lock()
	defer b.mux.Unlock()

	if b.timer != nil {
		b.timer.Stop()
	}

	if b.f != nil {
		b.w.Flush()
		return b.f.Close()
	}

	return nil
}

// flushAtInterval starts a timer, it will call Flush method every interval.
func (b *FileBuffer) flushAtInterval(interval time.Duration) {
	b.mux.Lock()
	defer b.mux.Unlock()

	b.timer = time.AfterFunc(interval, func() {
		var flush bool

		b.mux.RLock()
		flush = b.w.Buffered() > 0
		b.mux.RUnlock()

		if flush {
			b.Flush()
		}
		b.flushAtInterval(interval)
	})
}
