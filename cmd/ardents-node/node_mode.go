package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node"
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
	store, err := state.Open(runtime.state)
	if err != nil {
		return err
	}
	runtime.node.Current = func() (node.DutyView, error) {
		view, currentErr := store.CurrentNodeDuty()
		if currentErr != nil {
			return nil, currentErr
		}
		return view, nil
	}
	runtime.node.Emit = nodeEventEmitter(boundedOutput, runtime.diagnosticDirectory)
	_, runErr := node.Run(ctx, runtime.node)
	if closeErr := store.Close(); runErr == nil {
		runErr = closeErr
	}
	return runErr
}
