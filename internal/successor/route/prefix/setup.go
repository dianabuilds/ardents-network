package prefix

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// prefixSetupInterruption owns exactly one decision to stop or join a setup
// callback. AfterFunc's false result can also mean previously stopped; repeating
// that decision must never wait for a callback that the first stop prevented.
type prefixSetupInterruption struct {
	stop func() bool
	done <-chan struct{}
	once sync.Once
	ran  bool
}

func (i *prefixSetupInterruption) join() bool {
	i.once.Do(func() {
		if i.stop != nil && !i.stop() {
			i.ran = true
			<-i.done
		}
	})
	return i.ran
}

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
	interruption := prefixSetupInterruption{stop: stop, done: done}
	return func() error {
		interruption.join()
		return result
	}
}
