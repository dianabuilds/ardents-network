package main

import (
	"context"
	"errors"
)

// The trusted UI owns these original file descriptions until any interrupt
// has finished. The caller supplies an interruptible close for its own I/O.
func withDocumentPresentation(ctx context.Context, closeIO, present func() error) (outcome error) {
	var closeErr error
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(closed)
		closeErr = closeIO()
	})
	defer func() {
		if stop() {
			closeErr = closeIO()
		} else {
			<-closed
		}
		outcome = errors.Join(outcome, closeErr, ctx.Err())
	}()
	return present()
}
