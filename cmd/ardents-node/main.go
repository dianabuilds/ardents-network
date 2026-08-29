package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), nodeTerminationSignals()...)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run(ctx context.Context, arguments []string, output io.Writer) error {
	if len(arguments) > 0 && arguments[0] == "contributor" {
		return runContributor(ctx, arguments[1:], output)
	}
	if len(arguments) != 3 || arguments[1] != "--config" {
		return errors.New("usage: ardents-node (source|node) --config PATH | contributor ACTION")
	}
	if arguments[0] == "node" {
		return runNode(ctx, arguments[2], output)
	}
	if arguments[0] != "source" {
		return errors.New("usage: ardents-node (source|node) --config PATH | contributor ACTION")
	}
	events := newEventOutput(output)
	store, err := openSource(arguments[2], events.append)
	if err != nil {
		return err
	}
	snapshot, err := store.Current()
	if err == nil {
		err = events.encode(map[string]any{
			"schema": "ardents-source-event-v1", "kind": "source-ready",
			"generation": snapshot.Generation, "epoch": snapshot.Epoch,
		})
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
