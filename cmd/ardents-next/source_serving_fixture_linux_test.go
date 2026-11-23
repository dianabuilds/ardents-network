//go:build linux

package main

import (
	"context"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/network/source/serving"
	"github.com/dianabuilds/ardents-network/internal/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func acquireSourceInputs(ctx context.Context, path string) (sourceInputs, servingdeclaration.ServingDeclaration, error) {
	return serving.AcquireInputs(ctx, path, nil)
}

func acquireObservedSourceInputs(ctx context.Context, path string, observe func(string, *os.File)) (sourceInputs, servingdeclaration.ServingDeclaration, error) {
	return serving.AcquireInputs(ctx, path, observe)
}

func sourceServingInputs(config *state.Config, plan servingdeclaration.ServingDeclaration, inputs sourceInputs, check func() error) error {
	return serving.ConfigureInputs(config, plan, inputs, check)
}

type sourceLifetime interface {
	Current() (state.Snapshot, error)
	Wait(context.Context) error
	Close() error
}

func superviseSource(ctx context.Context, owner sourceLifetime, check func() error, deny context.CancelFunc, out, diagnostic io.Writer, joinInputs func() error) error {
	return serving.Supervise(ctx, owner, check, deny, sourceEvents(out, diagnostic), joinInputs)
}
