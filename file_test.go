package rollout

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileBufferWrite(t *testing.T) {
	buf := new(bytes.Buffer)

	b := FileBuffer{
		w: NewWriterSize(buf, 10),
	}

	b.Write([]byte("1234567890"))
	assert.Zero(t, buf.Len(), "should be empty because input is buffered")

	b.Write([]byte("12345"))
	assert.Equal(t, 15, buf.Len(), "data should be buffered")

	b.Write([]byte("abcdefghijklmno"))
	assert.Equal(t, 30, buf.Len(), "buffered data should write to underlying writer in one time")
}

func TestFileBufferFlush(t *testing.T) {
	buf := new(bytes.Buffer)
	b := FileBuffer{
		w: NewWriterSize(buf, 10),
	}

	b.Write([]byte("123456789"))
	b.Flush()

	assert.Equal(t, 9, buf.Len(), "data should be write to writer after flushing")
}
