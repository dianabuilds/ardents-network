package node

import (
	"context"
	"errors"
)

// Register this worker before starting the shared Wait. It interrupts outgoing
// reads, writes and HELLOs without taking session locks. Joining inside Carrier
// Close would deadlock: a reader's exact-lease invalidation needs the pool lock
// held while the pool closes its physical transports.
func (server *closedForwardingServer) closeOutgoing(ctx context.Context) {
	defer server.workers.Done()
	<-ctx.Done()
	server.outgoingErr = server.pool.Close()
}

// Join every accepted producer before the session owner waits for its readers.
// A producer can publish a late successful handshake, but no reader can be
// added after joinedResult starts its final Wait. A timeout waiting for drained
// does not release roots: this sole owner retains them until both layers join.
func (server *closedForwardingServer) finishShutdown() {
	server.workers.Wait()
	sessionErr := server.sessions.joinedResult()
	var hostErr error
	if server.host != nil {
		hostErr = server.host.Close()
	}
	server.drainErr = errors.Join(server.outgoingErr, sessionErr, server.spends.Close(), hostErr)
	close(server.drained)
}
