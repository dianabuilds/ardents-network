package outer

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// Serve owns an accepted outer channel and its inner handlers until their
// cancellation and cleanup have joined. It returns physical write and close failures
// to the receiving role, which retains the accepted connection's final result.
func Serve(ctx context.Context, connection net.Conn, handshake *route.ClosedOuterHandshake, serve func(context.Context, *route.ClosedOuterBridgeLane)) (closeErr error) {
	defer handshake.Close()
	var closeOnce sync.Once
	var physicalCloseErr error
	closePhysical := func() error {
		closeOnce.Do(func() { physicalCloseErr = connection.Close() })
		return physicalCloseErr
	}
	writer := &writer{connection: connection, closeConnection: closePhysical}
	bridge, err := route.NewClosedOuterBridge(handshake, writer.update, writer.write)
	if err != nil {
		return
	}
	childContext, cancel := context.WithCancel(ctx)
	interrupt := func() {
		_ = connection.SetDeadline(time.Now())
		_ = closePhysical()
	}
	interrupted := make(chan struct{})
	stop := context.AfterFunc(childContext, func() { defer close(interrupted); interrupt() })
	var children sync.WaitGroup
	defer func() {
		cancel()
		interrupt()
		bridge.Close()
		children.Wait()
		if !stop() {
			<-interrupted
		}
		closeErr = errors.Join(physicalCloseErr, writer.result())
	}()
	if connection.SetReadDeadline(time.Now().Add(10*time.Second)) != nil {
		return
	}
	first := true
	for {
		frame, err := ardp.ReadFrame(connection)
		if err != nil {
			return
		}
		lane, err := bridge.Accept(frame)
		if err != nil {
			return
		}
		if first {
			hello, err := ardp.DecodeHello(frame.Body)
			if err != nil || connection.SetDeadline(hello.Deadline) != nil {
				return
			}
			first = false
		}
		if lane != nil {
			children.Go(func() { serve(childContext, lane) })
		}
	}
}
