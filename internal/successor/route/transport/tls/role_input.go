package tls

import (
	"bufio"
	"net"
)

// roleInput gives the TLS reader one bounded ciphertext buffer. Its small
// record-header reads consume bytes already fetched for this same TLS channel,
// rather than each returning separate credit through the forwarding lanes.
// There is no background read, delay, authority cache or enlarged lane credit.
// The raw owner still controls deadlines, interruption and physical retirement.
type roleInput struct {
	net.Conn
	input *bufio.Reader
}

func newRoleInput(raw net.Conn) *roleInput {
	return &roleInput{Conn: raw, input: bufio.NewReaderSize(raw, 16<<10)}
}

func (conn *roleInput) Read(body []byte) (int, error) { return conn.input.Read(body) }

// NetConn preserves traversal to the original framing retirement witness.
func (conn *roleInput) NetConn() net.Conn { return conn.Conn }
