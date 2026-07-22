//go:build !linux

package daemon

import "net"

func unixPeerIdentity(net.Conn) ([]byte, bool) { return nil, false }
