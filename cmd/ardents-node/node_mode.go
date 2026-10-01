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
	runtime, err := readNodePlan(path)
	if err != nil {
		return err
	}
	return runNodeRuntime(ctx, runtime, output)
}

func runNodeRuntime(ctx context.Context, runtime nodeRuntime, output io.Writer) error {
	boundedOutput, ok := output.(*os.File)
	if !ok {
		return errors.New("node lifecycle output does not support write deadlines")
	}
	store, err := state.Open(runtime.state)
	if err != nil {
		return err
	}
	if _, currentErr := store.Current(); errors.Is(currentErr, state.ErrNoCurrentGeneration) {
		if _, refreshErr := store.Refresh(ctx); refreshErr != nil {
			return errors.Join(refreshErr, store.Close())
		}
	}
	runtime.node.Current = store.CurrentNodeDuty
	runtime.node.CurrentClosedProfile = func() (state.ClosedProfileView, bool) {
		profile, currentErr := store.CurrentClosedProfile()
		return profile, currentErr == nil
	}
	runtime.node.CurrentClosedRoute = store.CurrentClosedRoute
	runtime.node.Emit = nodeEventEmitter(boundedOutput, runtime.diagnosticDirectory)
	_, runErr := node.Run(ctx, runtime.node)
	return errors.Join(runErr, store.Close())
}
