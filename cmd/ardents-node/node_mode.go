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
	stopClockObservation := func() error { return nil }
	if runtime.clockObservation != "" {
		stopClockObservation, err = startContributorClockObservation(ctx, runtime.clockObservation, contributorClockObservationInterval)
		if err != nil {
			return err
		}
	}
	store, err := state.Open(runtime.state)
	if err != nil {
		return errors.Join(err, stopClockObservation())
	}
	if _, currentErr := store.Current(); errors.Is(currentErr, state.ErrNoCurrentGeneration) {
		if _, refreshErr := store.Refresh(ctx); refreshErr != nil {
			_ = store.Close()
			return refreshErr
		}
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
	return errors.Join(runErr, store.Close(), stopClockObservation())
}
