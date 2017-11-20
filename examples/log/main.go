package main

import (
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/jerray/rollout"
)

func main() {
	w := rollout.New(rollout.Options{
		Rotation: rollout.RotateDaily,
		Template: "test-{{.Time}}.log",
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
	w.Close()
}
