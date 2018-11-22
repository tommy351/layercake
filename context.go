package main

import (
	"context"
	"os"
	"os/signal"
)

func newContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		for {
			<-ch
			cancel()
		}
	}()

	return ctx
}
