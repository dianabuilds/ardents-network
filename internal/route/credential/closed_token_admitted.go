package credential

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// ServeAdmittedAfterHello consumes genuine class-1 receiving admission on an
// ordinary Node child, then serves exactly one fixed issuer operation. The
// outer lane independently checks this connection's TLS exporter and sealed
// admission before its pending lifetime can be extended.
func (issuer *ClosedTokenIssuer) ServeAdmittedAfterHello(ctx context.Context, connection net.Conn,
	channel *route.ClosedAdmissionChannel, helloFrame ardp.Frame, lane *route.ClosedOuterBridgeLane) error {
	if issuer == nil || ctx == nil || connection == nil || channel == nil || lane == nil ||
		lane.Restriction() != route.ClosedChildOrdinary || ctx.Err() != nil {
		return errors.New("closed admitted issuer channel is unavailable")
	}
	hello, err := ardp.DecodeHello(helloFrame.Body)
	if err != nil || !issuer.acceptsBootstrapHello(hello) {
		return errors.New("closed admitted issuer HELLO is unavailable")
	}
	if _, err := channel.Accept(helloFrame); err != nil {
		return err
	}
	admit, err := ardp.ReadFrame(connection)
	if err != nil || admit.Kind != 2 || admit.Lane != 0 || len(admit.Body) != 355 || admit.Body[0] != 1 {
		return errors.New("closed issuer requires Control admission")
	}
	lease, err := channel.Accept(admit)
	if err != nil {
		return err
	}
	defer lease.Release()
	if err := lane.Admit(&lease, connection); err != nil {
		return err
	}
	if err := connection.SetDeadline(lease.Deadline); err != nil {
		return err
	}
	accepted, err := ardp.AcceptFrame(0, 64<<10)
	if err != nil {
		return err
	}
	// Account all ARDP request/response frames, including admission. Exactly
	// one operation/result is permitted; no loop or arbitrary destination.
	used := uint64(16 + len(helloFrame.Body) + 16 + len(admit.Body))
	write := func(frame ardp.Frame) error {
		used += uint64(16 + len(frame.Body))
		if used > lease.Bytes || ctx.Err() != nil || !issuer.clock().UTC().Before(lease.Deadline) {
			return errors.New("closed admitted issuer reserve ended")
		}
		return ardp.WriteFrame(connection, frame)
	}
	if err := write(accepted); err != nil {
		return err
	}
	operation, err := ardp.ReadFrame(connection)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 || len(operation.Body) != 16<<10 {
		return errors.New("closed admitted issuer operation is invalid")
	}
	used += uint64(16 + len(operation.Body))
	if used > lease.Bytes || ctx.Err() != nil || !issuer.clock().UTC().Before(lease.Deadline) {
		return errors.New("closed admitted issuer reserve ended")
	}
	result, err := issuer.issueTerminalOperation(operation.Body, closedIssuanceAdmitted)
	if err != nil {
		return err
	}
	return write(ardp.Frame{Kind: 11, Body: result})
}
