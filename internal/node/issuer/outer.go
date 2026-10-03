package issuer

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	nodeouter "github.com/dianabuilds/ardents-network/internal/node/outer"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// nodeHandler owns one State-authenticated outer Carrier. It
// creates no peer, route or fallback: every child terminates at this issuer.
func nodeHandler(config Config, certificate tls.Certificate, issuer *credential.ClosedTokenIssuer, spends *spending.Ledger, limits *route.ClosedDutyLimits, recordRelease func(error)) credential.ClosedNodeBootstrapHandler {
	return func(ctx context.Context, carrier routecarrier.ClosedSharedCarrier, serve func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error) {
		defer func() { recordRelease(carrier.Connection.Close()) }()
		updated, err := config.CurrentDuty()
		if err != nil {
			return
		}
		receiver, available := config.Authority.Receiver(updated, ardp.PurposeIssuer, config.Now())
		if !available {
			return
		}
		deadline := receiver.NotAfter
		outer, err := route.NewClosedOuterHandshake(route.ClosedOuterReceiver{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration,
			StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest, NodeID: receiver.NodeID, RecordDigest: receiver.RecordDigest,
			DutyGeneration: receiver.DutyGeneration, RoleDomain: receiver.RoleDomain, Subrole: receiver.Subrole, Deadline: deadline}, limits, config.Now)
		if err != nil {
			return
		}
		recordRelease(nodeouter.Serve(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
			admitted := func(connection net.Conn, hello ardp.Frame) error {
				exporter, err := routecarrier.ClosedRoleTLSExporter(connection)
				if err != nil {
					return err
				}
				channel, err := route.NewClosedAdmissionChannel(receiver, spends, limits, exporter, config.VerifyAdmission(receiver), config.Now)
				if err != nil {
					return err
				}
				operationErr, releaseErr := issuer.ServeAdmittedAfterHello(childContext, connection, channel, hello, lane)
				recordRelease(releaseErr)
				return operationErr
			}
			serveClosedIssuerInner(childContext, lane, certificate, deadline, carrier.NodeKey, serve, admitted)
		}))
	}
}

func serveClosedIssuerInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, certificate tls.Certificate, deadline time.Time, adjacency [32]byte, serve func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error, admitted func(net.Conn, ardp.Frame) error) {
	status := byte(1)
	defer func() { _ = lane.CloseWithStatus(status) }()
	secured, err := routecarrier.AcceptClosedRoleTLS(ctx, lane, certificate, deadline)
	if err != nil {
		return
	}
	defer func() {
		if secured.CloseWrite() != nil {
			status = 1
		}
	}()
	if err := lane.BeginInnerHello(); err != nil {
		return
	}
	helloFrame, err := ardp.ReadFrame(secured)
	if err != nil {
		return
	}
	if helloFrame.Kind != 1 || helloFrame.Lane != 0 {
		return
	}
	hello, err := ardp.DecodeHello(helloFrame.Body)
	if err != nil || lane.Activate(hello) != nil {
		return
	}
	switch lane.Restriction() {
	case route.ClosedChildIssuerBootstrap:
		if serve(ctx, secured, adjacency, hello) == nil {
			status = 0
		}
	case route.ClosedChildOrdinary:
		if admitted(secured, helloFrame) == nil {
			status = 0
		}
	}
}
