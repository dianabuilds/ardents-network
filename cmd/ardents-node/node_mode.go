package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
	"github.com/dianabuilds/ardents-network/internal/nodelifecycle"
)

func runNode(ctx context.Context, path string, output io.Writer) error {
	boundedOutput, ok := output.(*os.File)
	if !ok {
		return errors.New("node lifecycle output does not support write deadlines")
	}
	runtime, err := readNodePlan(path)
	if err != nil {
		return err
	}
	store, err := networkstate.Open(runtime.state)
	if err != nil {
		return err
	}
	runtime.node.Current = store.Current
	runtime.node.Emit = nodelifecycle.EventEmitter(boundedOutput)
	_, runErr := nodelifecycle.Run(ctx, runtime.node)
	if closeErr := store.Close(); runErr == nil {
		runErr = closeErr
	}
	return runErr
}
