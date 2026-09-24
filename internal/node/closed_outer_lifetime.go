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
	serveClosedOuterObserved(ctx, connection, outer, serve, nil)
}

func serveClosedOuterObserved(ctx context.Context, connection net.Conn, outer *route.ClosedOuterHandshake, serve func(context.Context, *route.ClosedOuterBridgeLane), observe func(string)) {
	defer outer.Close()
	writer := &closedOuterWriter{connection: connection}
	bridge, err := route.NewClosedOuterBridge(outer, writer.update, writer.write)
	if err != nil {
		if observe != nil {
			observe("outer-bridge-" + closedRouteDiagnosticCause(err))
		}
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
	reason := ""
	childStarted := false
	defer func() {
		cancel()
		interrupt()
		bridge.Close()
		children.Wait()
		if !stop() {
			<-interrupted
		}
		if reason != "" && observe != nil {
			observe(reason)
		}
	}()
	if connection.SetReadDeadline(time.Now().Add(10*time.Second)) != nil {
		reason = "outer-read-deadline-other"
		return
	}
	first := true
	for {
		frame, err := route.ReadClosedLaneFrame(connection)
		if err != nil {
			if !childStarted && ctx.Err() == nil {
				reason = "outer-read-" + closedRouteDiagnosticCause(err)
			}
			return
		}
		lane, err := bridge.Accept(frame)
		if err != nil {
			if !childStarted && ctx.Err() == nil {
				reason = "outer-accept-" + closedRouteDiagnosticCause(err)
			}
			return
		}
		if first {
			hello, err := route.DecodeClosedHello(frame.Body)
			if err != nil || connection.SetDeadline(hello.Deadline) != nil {
				if err != nil {
					reason = "outer-hello-invalid"
				} else {
					reason = "outer-deadline-other"
				}
				return
			}
			first = false
		}
		if lane != nil {
			childStarted = true
			children.Go(func() { serve(childContext, lane) })
		}
	}
}
