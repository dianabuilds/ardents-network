//go:build linux

package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// openAuthorizedAttachment keeps setup cancellation tied to the actual local
// peer. Clients send no data until ACCEPT, so an early byte or disconnection
// refuses setup rather than becoming an unbounded optimistic input queue.
func (server *server) openAuthorizedAttachment(local *net.UnixConn, request Request) (Stream, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(server.ctx)
	timer := time.AfterFunc(15*time.Second, cancel)
	var mu sync.Mutex
	finished := false
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		var unexpected [1]byte
		n, err := local.Read(unexpected[:])
		mu.Lock()
		ready := finished
		mu.Unlock()
		var networkErr net.Error
		if n != 0 || !ready || !errors.As(err, &networkErr) || !networkErr.Timeout() {
			cancel()
		}
	}()
	stream, openErr := server.owner.Open(ctx, request)
	mu.Lock()
	finished = true
	mu.Unlock()
	if err := local.SetReadDeadline(time.Now()); err != nil {
		cancel()
		_ = local.Close()
	}
	<-monitorDone
	if !timer.Stop() {
		cancel()
	}
	if openErr != nil || stream == nil || ctx.Err() != nil {
		cancel()
		if stream != nil {
			_ = stream.Close()
		}
		return nil, nil, errors.Join(openErr, ctx.Err(), errors.New("local Application setup did not complete"))
	}
	if err := local.SetReadDeadline(time.Time{}); err != nil {
		cancel()
		_ = stream.Close()
		return nil, nil, err
	}
	return stream, cancel, nil
}
