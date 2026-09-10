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

// Every producer of a session reader is itself a counted accepted handler.
// Its Add therefore precedes the last Done, even when cancellation races an
// outgoing handshake. A timeout waiting for drained does not release roots:
// this sole owner retains them until the complete tree actually joins.
func (server *closedForwardingServer) finishShutdown() {
	server.workers.Wait()
	server.sessions.mu.Lock()
	sessionErr := server.sessions.cleanupErr
	server.sessions.mu.Unlock()
	server.drainErr = errors.Join(server.outgoingErr, sessionErr, server.spends.Close())
	close(server.drained)
}
