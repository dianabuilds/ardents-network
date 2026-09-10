//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
)

// showPublishedLink owns one explicit destination presentation. The value is
// never added to ordinary diagnostics, a file, or a background runtime event.
func showPublishedLink(ctx context.Context, socket string, output io.WriteCloser) (outcome error) {
	if ctx == nil || output == nil {
		return errTextInput
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var once sync.Once
	var closeErr error
	closeOutput := func() { once.Do(func() { closeErr = output.Close() }) }
	interrupted := make(chan struct{})
	stop := context.AfterFunc(bounded, func() { defer close(interrupted); closeOutput() })
	defer func() {
		closeOutput()
		if !stop() {
			<-interrupted
		}
		outcome = errors.Join(outcome, closeErr, bounded.Err())
	}()
	link, err := administration.RequestPublishedLink(bounded, socket)
	if err != nil {
		return err
	}
	_, err = io.WriteString(output, link+"\n")
	return err
}
