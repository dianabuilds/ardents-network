package transport

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
)

// TLSWriteLimit projects only physical record capacity through the original
// connection. It grants no credit or authority and performs no I/O. A framed
// lower stream accounts for its own headers before returning its payload limit.
func TLSWriteLimit(conn net.Conn) int {
	for conn != nil {
		switch current := conn.(type) {
		case *tls.Conn:
			if current.ConnectionState().Version == tls.VersionTLS13 {
				return min(16<<10, TLSWriteLimit(current.NetConn())) - 5 - 1 - 16
			}
			return 16 << 10
		case interface{ RecordWriteLimit() int }:
			return min(16<<10, current.RecordWriteLimit())
		case interface{ NetConn() net.Conn }:
			conn = current.NetConn()
		default:
			return 16 << 10
		}
	}
	return 16 << 10
}

// Write keeps each selected TLS 1.3 record within one 16 KiB Route BYTES
// frame: five header bytes, one inner content-type byte and a 16-byte AEAD tag.
// All bytes are written immediately; no traffic is retained for a later call.
// Serializing the complete call preserves TLS Write's concurrent-call ordering.
func (c *retainedConn) Write(body []byte) (int, error) {
	secured, ok := c.Conn.(*tls.Conn)
	if !ok || secured.ConnectionState().Version != tls.VersionTLS13 || len(body) == 0 {
		return c.Conn.Write(body)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	limit := TLSWriteLimit(secured)
	if limit <= 0 {
		return 0, errors.New("nested TLS record capacity unavailable")
	}
	written := 0
	for len(body) != 0 {
		part := body[:min(len(body), limit)]
		n, err := secured.Write(part)
		written += n
		if err != nil {
			return written, err
		}
		if n != len(part) {
			return written, io.ErrShortWrite
		}
		body = body[n:]
	}
	return written, nil
}
