//go:build !linux

package administration

import (
	"net"
	"time"
)

func (*server) handleByteStream(*net.UnixConn, []byte, time.Time) bool { return false }
