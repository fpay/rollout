package rollout

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"
)

const (
	defaultBufferSize    = 4096
	defaultDestTamplate  = "rollout-{{.Time}}.log"
	defaultTimeFormat    = "2006-01-02"
	defaultFlushInterval = 10
	defaultKeeps         = 30

	// RotateSecondly rotate every second
	RotateSecondly = 1

	// RotateMinutely rorate every minute
	RotateMinutely = 60 * RotateSecondly

	// RotateHourly rotate every hour
	RotateHourly = 60 * RotateMinutely

	// RotateDaily rotate every day
	RotateDaily = 24 * RotateHourly

	// RotateWeekly rotate every week
	RotateWeekly = 7 * RotateDaily
)

var (
	defaultClock = time.Now

	host string
	pid  int

	ErrClosed = errors.New("write stream closed")
)

func init() {
	host = getHostname()
	pid = os.Getpid()
}

func getHostname() string {
	var b []byte
	name, err := os.Hostname()
	if err == nil {
		b = []byte(name)
	} else {
		b = make([]byte, 1024)
		rand.Read(b)
	}
	h := sha1.New()
	h.Write(b)

	return hex.EncodeToString(h.Sum(nil))
}

// Clock function used to get time. Mostly for testing purpose.
type Clock func() time.Time

// BufferFunc is function to generate a new Buffer.
type BufferFunc func(dest string, size int, interval time.Duration) (Buffer, error)

// Options is data for create Rollout instance.
type Options struct {

	// Template is a template string for output destination name. Useable variables are `Host`, `Pid` and `Time`.
	// You can change time format by providing `TimeFormat` option.
	// In the situation of multiple processes, it is highly recommended to add `{{.Pid}}` in the template to avoid
	// writing conflicts. If you run multiple processes in docker in the same machine, and they all write to the
	// same directory in the host, add `{{.Host}}` in the template.
	Template string

	// TimeFormat is format string for `Template`'s Time field value. Default is "2016-01-02".
	TimeFormat string

	// Root is prefix of output destination name. In the built-in file buffer, it is treated as file directory.
	Root string

	// Rotation is the frequency how often write to a new destination. Default is RotateDaily.
	Rotation int

	// Keeps is how many destination copies will be retained. Default is 30.
	Keeps int

	// BufferSize is the size of underlying buffer. Default is 4096.
	BufferSize int

	// Flush is the interval for buffer automaticly flushing. Default is 10.
	Flush int

	// BufferFunc is a function generating new buffer. Default value is the built-in NewFileBuffer.
	BufferFunc BufferFunc

	// Clock is function to get current time.
	Clock Clock
}

// Rollout is an io.WriteCloser. It is used for writing logs to rolling files.
// Output Buffer is an interface, so you can define your own Buffer and BufferFunc
// to use another underlying writer other than built-in file buffer.
type Rollout struct {
	bufferSize    int
	bufferFunc    BufferFunc
	clock         Clock
	flushInterval time.Duration
	interval      int
	root          string
	template      *template.Template
	timeFormat    string
	keeps         int
	zoneOffset    int

	mux    sync.RWMutex
	buf    *rolloutBuffer
	closed bool
}

// New creates Rollout instance.
func New(options Options) *Rollout {
	if options.Rotation <= 0 {
		options.Rotation = RotateDaily
	}

	if options.Template == "" {
		options.Template = defaultDestTamplate
	}

	if options.TimeFormat == "" {
		options.TimeFormat = defaultTimeFormat
	}

	if options.BufferSize <= 0 {
		options.BufferSize = defaultBufferSize
	}

	if options.Flush <= 0 {
		options.Flush = defaultFlushInterval
	}

	if options.Clock == nil {
		options.Clock = defaultClock
	}

	if options.BufferFunc == nil {
		options.BufferFunc = NewFileBuffer
	}

	tpl := template.New("package.rollout.filename")
	tpl, err := tpl.Parse(options.Template)
	if err != nil {
		tpl, _ = tpl.Parse(defaultDestTamplate)
	}

	r := Rollout{
		interval:      options.Rotation,
		root:          options.Root,
		template:      tpl,
		timeFormat:    options.TimeFormat,
		bufferSize:    options.BufferSize,
		bufferFunc:    options.BufferFunc,
		flushInterval: time.Duration(options.Flush) * time.Second,
		clock:         options.Clock,
		keeps:         options.Keeps,
	}

	_, r.zoneOffset = options.Clock().Zone()

	return &r
}

type rolloutBuffer struct {
	Buffer
	pos int
}

// Write writes the contents of p into the buffer. It returns an error if its status
// is closed or it fails to create the logging file.
func (r *Rollout) Write(p []byte) (n int, err error) {
	r.mux.RLock()
	if r.closed {
		r.mux.RUnlock()
		return 0, ErrClosed
	}
	r.mux.RUnlock()

	r.mux.Lock()
	defer r.mux.Unlock()

	now := r.clock()
	pos := r.position(now)

	if r.buf == nil || r.buf.pos != pos {
		buf, err := r.bufferFunc(r.destination(now), r.bufferSize, r.flushInterval)
		if err != nil {
			return 0, err
		}

		var old *rolloutBuffer
		old, r.buf = r.buf, &rolloutBuffer{buf, pos}

		if old != nil {
			old.Close()
		}
	}

	return r.buf.Write(p)
}

// Flush writes buffered data to current file.
func (r *Rollout) Flush() error {
	r.mux.RLock()
	defer r.mux.RUnlock()

	if r.buf == nil {
		return nil
	}
	return r.buf.Flush()
}

// Close the writer. There may be data present in current buffer when main goroutine
// quits. Such data will lost if you don't flush it to the underlying writer. Close
// will flushes any data in the buffer to current logging file and then closes the file
// descriptor. So make sure Rollout is closed before main goroutine quits.
func (r *Rollout) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	if r.buf == nil {
		return nil
	}
	return r.buf.Close()
}

// Rotate TODO: delete old files
// func (r *Rollout) Rotate() error {
// 	return nil
// }

func (r *Rollout) position(t time.Time) int {
	timestamp := int(t.Unix())
	if r.interval >= RotateDaily {
		timestamp += r.zoneOffset
	}
	return timestamp / r.interval
}

func (r *Rollout) destination(t time.Time) string {
	buf := new(bytes.Buffer)
	r.template.Execute(buf, map[string]interface{}{
		"Pid":  pid,
		"Host": host,
		"Time": t.Format(r.timeFormat),
	})
	return filepath.Join(r.root, buf.String())
}
