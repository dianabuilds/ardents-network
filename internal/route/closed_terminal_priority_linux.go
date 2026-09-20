//go:build linux

package route

import (
	"crypto/tls"
	"net"
	"sync"
)

func beginClosedTerminalWrite(parent net.Conn) func() {
	for {
		secured, ok := parent.(*tls.Conn)
		if !ok {
			break
		}
		parent = secured.NetConn()
	}
	switch prioritized := parent.(type) {
	case *closedSourceLane:
		return prioritized.beginTerminalWrite()
	case *closedRoleChildStream:
		return prioritized.beginTerminalWrite()
	case *ClosedOuterBridgeLane:
		return prioritized.beginTerminalWrite()
	default:
		return func() {}
	}
}

func (lane *ClosedOuterBridgeLane) beginTerminalWrite() func() {
	inner := lane.lane
	inner.mu.Lock()
	inner.terminalWriters++
	inner.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			inner.mu.Lock()
			inner.terminalWriters--
			inner.mu.Unlock()
		})
	}
}
