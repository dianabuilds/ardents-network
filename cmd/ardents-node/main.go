package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) != 3 || arguments[0] != "source" || arguments[1] != "--config" {
		return errors.New("usage: ardents-node source --config PATH")
	}
	store, err := openSource(arguments[2])
	if err != nil {
		return err
	}
	snapshot, err := store.Current()
	if err == nil {
		err = json.NewEncoder(output).Encode(struct {
			Schema     string `json:"schema"`
			Kind       string `json:"kind"`
			Generation string `json:"generation"`
			Epoch      uint64 `json:"epoch"`
		}{"ardents-h3-s1-source-event-v1", "source-ready", snapshot.Generation, snapshot.Epoch})
	}
	if err != nil {
		_ = store.Close()
		return err
	}
	waitErr := store.Wait(ctx)
	closeErr := store.Close()
	if waitErr != nil {
		return waitErr
	}
	return closeErr
}
