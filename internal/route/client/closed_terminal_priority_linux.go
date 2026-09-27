//go:build linux

package client

import (
	"crypto/tls"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
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
	case *route.ClosedOuterBridgeLane:
		return prioritized.BeginTerminalWrite()
	default:
		return func() {}
	}
}
