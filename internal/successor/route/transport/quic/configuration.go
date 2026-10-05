package quic

import (
	"github.com/quic-go/quic-go"
	"time"
)

func nodeConfig() *quic.Config {
	return &quic.Config{Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, KeepAlivePeriod: time.Second, MaxIncomingStreams: -1, MaxIncomingUniStreams: -1,
		InitialPacketSize: 1200, InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false }, EnableDatagrams: false, Allow0RTT: false}
}
