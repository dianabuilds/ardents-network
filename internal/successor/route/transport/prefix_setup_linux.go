//go:build linux

package transport

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// interruptPrefixOpening owns setup-only cancellation. Its stop joins a
// started callback and retains both physical failures before the connection
// can transfer or its reservation can return.
func interruptPrefixOpening(ctx context.Context, conn net.Conn) func() error {
	done := make(chan struct{})
	var result error
	stop := context.AfterFunc(ctx, func() {
		defer close(done)
		result = errors.Join(conn.SetDeadline(time.Now()), conn.Close())
	})
	var once sync.Once
	return func() error {
		once.Do(func() {
			if !stop() {
				<-done
			}
		})
		return result
	}
}
