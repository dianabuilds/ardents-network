package prefix

import (
	"context"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/route/channel"
	"github.com/dianabuilds/ardents-network/internal/route/role"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	roletls "github.com/dianabuilds/ardents-network/internal/route/transport/tls"
)

// openRoleLane opens one lower lane on the original Interior Session,
// addressed to the exact recipient duty, then arms the setup interruption
// that closes only that lane when ctx ends. The caller retains the lane and
// that interruption; neither supplies role authority.
func (p *Prefix) openRoleLane(ctx, caller context.Context, recipient network.RetainedDuty, purpose ardp.Purpose, end time.Time) (*framing.Lane, prefixSetupInterruption, error) {
	lane, err := p.interior.Open(ctx, caller, ardp.EncodeOpen(ardp.Open{RecipientNodeID: recipient.NodeID, RecipientDutyGeneration: recipient.RecordGeneration, Purpose: uint8(purpose), Deadline: end}, false))
	if err != nil {
		return nil, prefixSetupInterruption{}, err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); _ = lane.Close() })
	return lane, prefixSetupInterruption{stop: stop, done: done}, nil
}

// presentRoleLane authenticates the lane against the freshly observed member
// key, retains its connection and presents the role HELLO. setup bounds the
// handshake and the retained connection; end bounds the HELLO. A connection is
// returned only once it has been retained, including after a later failure, so
// its owner closes it on every path.
func (p *Prefix) presentRoleLane(ctx, caller context.Context, lane *framing.Lane, authority role.Authority, member network.Member, purpose ardp.Purpose, end, setup time.Time, bootstrap bool) (net.Conn, error) {
	secured, err := roletls.OpenRole(ctx, lane, member.PublicKey, framing.MinDeadline(setup, time.Now().Add(10*time.Second)))
	if err != nil {
		return nil, err
	}
	conn := transport.Retain(secured)
	if err := conn.SetDeadline(setup); err != nil {
		return conn, err
	}
	hello, err := authority.FreshHello(end, purpose, false)
	if err != nil {
		return conn, err
	}
	return conn, presentPrefixRole(ctx, caller, conn, authority, hello, p.config.Present, bootstrap)
}
