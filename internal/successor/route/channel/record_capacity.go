package channel

import (
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// RecordWriteLimit projects immutable framing capacity, not live credit or
// authority. Subtract this lane's frame header from its original parent's TLS
// payload bound so nested records do not create extra small BYTES frames.
func (l *Lane) RecordWriteLimit() int {
	if l == nil || l.s == nil {
		return 0
	}
	return min(ardp.MaximumBodySize, transport.TLSWriteLimit(l.s.conn)-ardp.HeaderSize)
}
