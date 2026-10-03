package issuer

import (
	"context"
	"crypto/tls"
	"errors"
	admissionissuer "github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	nodeouter "github.com/dianabuilds/ardents-network/internal/node/outer"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// nodeHandler owns one State-authenticated outer Carrier. It
// creates no peer, route or fallback: every child terminates at this issuer.
func nodeHandler(config Config, certificate tls.Certificate, issuer *admissionissuer.ClosedTokenIssuer, spends *spending.Ledger, limits *route.ClosedDutyLimits, recordRelease func(error)) admissionissuer.ClosedNodeBootstrapHandler {
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
		var childReleases releaseErrors
		outerErr := nodeouter.Serve(ctx, carrier.Connection, outer, func(childContext context.Context, lane *route.ClosedOuterBridgeLane) {
			admitted := func(connection net.Conn, hello ardp.Frame) (*route.ClosedAdmission, error) {
				exporter, err := routecarrier.ClosedRoleTLSExporter(connection)
				if err != nil {
					return nil, err
				}
				channel, err := route.NewClosedAdmissionChannel(receiver, spends, limits, exporter, config.VerifyAdmission(receiver), config.Now)
				if err != nil {
					return nil, err
				}
				return issuer.ServeAdmittedAfterHello(childContext, connection, channel, hello, lane)
			}
			childReleases.record(serveClosedIssuerInner(childContext, lane, certificate, deadline, carrier.NodeKey, serve, admitted))
		})
		// Publish one joined Carrier outcome. A secondary inner-TLS error must
		// not win the listener's first-failure slot before its causal physical
		// writer result has arrived from the joined outer owner.
		recordRelease(errors.Join(outerErr, childReleases.result()))
	}
}

func serveClosedIssuerInner(ctx context.Context, lane *route.ClosedOuterBridgeLane, certificate tls.Certificate, deadline time.Time, adjacency [32]byte, serve func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error, admitted func(net.Conn, ardp.Frame) (*route.ClosedAdmission, error)) (cleanupErr error) {
	status := byte(1)
	var admission *route.ClosedAdmission
	defer func() { cleanupErr = errors.Join(cleanupErr, admission.Release()) }()
	defer func() { cleanupErr = errors.Join(cleanupErr, lane.CloseWithStatus(status)) }()
	secured, err := routecarrier.AcceptClosedRoleTLS(ctx, lane, certificate, deadline)
	if err != nil {
		return
	}
	defer func() {
		if err := secured.CloseWrite(); err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
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
		var operationErr error
		admission, operationErr = admitted(secured, helloFrame)
		if operationErr == nil {
			status = 0
		}
	}
	return
}
