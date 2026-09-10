package node

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// serveClosedOuter owns the transport, every allocated inner handler, and
// their receiving-duty reservations until cancellation and cleanup have joined.
func serveClosedOuter(ctx context.Context, connection net.Conn, outer *route.ClosedOuterHandshake, serve func(context.Context, *route.ClosedOuterBridgeLane)) {
	defer outer.Close()
	writer := &closedOuterWriter{connection: connection}
	bridge, err := route.NewClosedOuterBridge(outer, writer.update, writer.write)
	if err != nil {
		return
	}
	childContext, cancel := context.WithCancel(ctx)
	interrupt := func() {
		_ = connection.SetDeadline(time.Now())
		_ = connection.Close()
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
	}()
	if connection.SetReadDeadline(time.Now().Add(10*time.Second)) != nil {
		return
	}
	first := true
	for {
		frame, err := route.ReadClosedLaneFrame(connection)
		if err != nil {
			return
		}
		lane, err := bridge.Accept(frame)
		if err != nil {
			return
		}
		if first {
			hello, err := route.DecodeClosedHello(frame.Body)
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
