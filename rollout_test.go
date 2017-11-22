package rollout

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRolloutDefaultOptions(t *testing.T) {
	r := New(Options{})

	assert.Equal(t, defaultBufferSize, r.bufferSize, "default buffer size should match")
	assert.Equal(t, RotateDaily, r.interval, "default rotation interval should be daily")
	assert.Equal(t, 10*time.Second, r.flushInterval, "default flushing interval should be 10s")
	assert.Equal(t, defaultTimeFormat, r.timeFormat, "default time format should match")
}

type MockBuffer struct {
	mock.Mock
}

func (m *MockBuffer) Write(p []byte) (n int, err error) {
	m.On("Write", p).Return(len(p), nil)
	m.MethodCalled("Write", p)
	return len(p), nil
}

func (m *MockBuffer) Close() error {
	m.On("Close").Return(nil)
	m.MethodCalled("Close")
	return nil
}

func (m *MockBuffer) Flush() error {
	m.On("Flush").Return(nil)
	m.MethodCalled("Flush")
	return nil
}

func NewMockBuffer(dest string, size int, interval time.Duration) (Buffer, error) {
	return &MockBuffer{}, nil
}

func TestRolloutWrite(t *testing.T) {
	clock := func() Clock {
		now := time.Now()
		return func() time.Time {
			now = now.Add(time.Second)
			return now
		}
	}()

	r := New(Options{
		Clock:      clock,
		BufferFunc: NewMockBuffer,
		Rotation:   RotateSecondly,
	})

	p := []byte("any data")
	r.Write(p)
	mb := r.buf.Buffer.(*MockBuffer)
	mb.AssertCalled(t, "Write", p)

	p = []byte("any data")
	r.Write(p)
	mb2 := r.buf.Buffer.(*MockBuffer)
	mb2.AssertCalled(t, "Write", p)

	// old buffer should be closed
	mb.AssertCalled(t, "Close")

	r = New(Options{
		BufferFunc: func(dest string, size int, interval time.Duration) (Buffer, error) {
			return nil, errors.New("test")
		},
	})
	n, err := r.Write(p)
	assert.Error(t, err, "write should return error")
	assert.Zero(t, n, "write byte should be zero")
}

func TestRolloutFlush(t *testing.T) {
	r := New(Options{
		BufferFunc: NewMockBuffer,
	})

	r.Flush()
	r.Write([]byte("any"))
	mb := r.buf.Buffer.(*MockBuffer)
	r.Flush()
	r.Flush()
	mb.AssertNumberOfCalls(t, "Flush", 2)
}

func TestRolloutClose(t *testing.T) {
	r := New(Options{
		BufferFunc: NewMockBuffer,
	})
	assert.Nil(t, r.buf, "buf should be nil if there is no write")
	r.Close()

	r = New(Options{
		BufferFunc: NewMockBuffer,
	})
	r.Write([]byte("any"))
	mb := r.buf.Buffer.(*MockBuffer)
	r.Close()
	r.Close()
	mb.AssertNumberOfCalls(t, "Close", 1)

	assert.True(t, r.closed, "closed flag should be true")

	_, err := r.Write([]byte("test data"))
	assert.Equal(t, ErrClosed, err, "write to closed writer should return error")
}

func TestRolloutPosition(t *testing.T) {
	cases := []struct {
		time     time.Time
		interval int
		position int
	}{
		{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), 1, 0},
		{time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), RotateMinutely, 0},
		{time.Date(2017, time.November, 11, 14, 9, 27, 0, time.UTC), RotateMinutely, 25173489},
		{time.Date(2017, time.November, 11, 14, 9, 27, 0, time.UTC), RotateDaily, 17481},
		{time.Date(2017, time.November, 22, 0, 0, 0, 0, time.UTC), RotateDaily, 17492},
		{time.Date(2017, time.November, 22, 0, 0, 0, 0, time.Local), RotateDaily, 17492},
	}

	for _, c := range cases {
		r := New(Options{
			Rotation: c.interval,
		})

		actual := r.position(c.time)
		assert.Equal(t, c.position, actual, "position should match")
	}
}

func TestRolloutDestination(t *testing.T) {
	cases := []struct {
		root     string
		template string
		format   string
		time     time.Time
		expect   string
	}{
		{"", "test-{{.Time}}.log", "2006-01-02", time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), "test-1970-01-01.log"},
		{"/var/log", "{{.Time}}.log", "2006-01-02", time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC), "/var/log/1970-01-01.log"},
		{"/var/log", "test-{{.Time}}.log", "2006-01-02", time.Date(2017, time.November, 11, 14, 15, 0, 0, time.UTC), "/var/log/test-2017-11-11.log"},
		{"", "test-{{.Time}}.log", "2006-01-02 15:04", time.Date(2017, time.November, 11, 14, 15, 0, 0, time.UTC), "test-2017-11-11 14:15.log"},
		{"", "test-{{.Time}}.log", "2006-01-02", time.Date(2017, time.November, 22, 0, 0, 0, 0, time.Local), "test-2017-11-22.log"},
	}

	for _, c := range cases {
		r := New(Options{
			Template:   c.template,
			TimeFormat: c.format,
			Root:       c.root,
		})
		actual := r.destination(c.time)
		assert.Equal(t, c.expect, actual, "destination should match")
	}
}
