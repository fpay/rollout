# Rollout

Rollout is an io.WriteCloser. It is mainly used for writing logs to rolling files.

## Features

1. Rotate by time interval.
2. Customized rotation interval.
3. Customized output destination (file name if you are using built-in FileBuffer).
4. Write to buffer first to reduce IO.
5. Thread safe.

## Install

```sh
go get github.com/jerray/rollout
```

## Usage

```go
func main() {
	w := rollout.New(rollout.Options{
		Rotation: rollout.RotateDaily,
		Template: "rollout-{{.Time}}.log",
	})

	log.SetOutput(w)

	for i := 0; i < 5; i++ {
		go func(i int) {
			for {
				log.Printf("%d - %s\n", i, "OK")
				time.Sleep(1 * time.Second)
			}
		}(i)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

    // Make sure any data in buffer is flushed to file before main goroutine quits.
	w.Close()
}
```

## License

MIT
