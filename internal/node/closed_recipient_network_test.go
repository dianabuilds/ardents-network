//go:build linux

package node

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/dianabuilds/ardents-network/internal/route"
	"net"
	"time"
)

func (fixture *resolutionNetworkFixture) openTerminal(ctx context.Context, token []byte, class uint8) (net.Conn, func(), error) {
	end := minResolutionTime(time.Now().UTC().Add(90*time.Second).Truncate(time.Second), fixture.profile.NotAfter)
	outer, err := route.OpenClosedNodeCarrier(ctx, route.ClosedNodeCarrierRequest{CarrierProfile: fixture.carrier, Endpoint: fixture.endpoint, Certificate: fixture.certificate, ExpectedPeerKey: fixture.serverKey, Deadline: end})
	if err != nil {
		return nil, nil, fmt.Errorf("outer dial: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = outer.Close()
		}
	}()
	hello := route.ClosedHello{NetworkID: fixture.profile.NetworkID, StateGeneration: fixture.profile.StateGeneration, StateDigest: fixture.profile.StateDigest,
		ProfileDigest: fixture.profile.Digest, RecipientNodeID: fixture.receiver.NodeID, RecipientDutyGeneration: fixture.receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, Deadline: end}
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, nil, err
	}
	body, err := route.EncodeClosedHello(hello)
	if err != nil {
		return nil, nil, err
	}
	if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		return nil, nil, err
	}
	accepted, err := route.ReadClosedLaneFrame(outer)
	if err != nil || accepted.Kind != 5 {
		return nil, nil, fmt.Errorf("outer accept: kind=%d error=%v", accepted.Kind, err)
	}
	open, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: fixture.receiver.NodeID, NextDutyGeneration: fixture.receiver.DutyGeneration, Purpose: fixture.receiver.ExpectedPurpose, Deadline: end}, route.ClosedChildOrdinary)
	if err != nil {
		return nil, nil, err
	}
	if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: open}); err != nil {
		return nil, nil, err
	}
	inner, err := route.OpenClosedRoleTLS(ctx, &outerTestInnerConn{outer: outer, lane: 1}, fixture.serverKey, end)
	if err != nil {
		return nil, nil, fmt.Errorf("inner TLS: %w", err)
	}
	hello.Purpose = fixture.receiver.ExpectedPurpose
	if _, err := rand.Read(hello.ChannelNonce[:]); err != nil {
		return nil, nil, err
	}
	body, err = route.EncodeClosedHello(hello)
	if err != nil {
		return nil, nil, err
	}
	if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
		return nil, nil, err
	}
	admit := append([]byte{class}, token...)
	defer clear(admit)
	if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 2, Body: admit}); err != nil {
		return nil, nil, err
	}
	accepted, err = route.ReadClosedLaneFrame(inner)
	if err != nil {
		return nil, nil, fmt.Errorf("Control admission: %w", err)
	}
	if status, credit, err := route.DecodeClosedAcceptFrame(accepted); err != nil || status != 0 || credit != 64<<10 {
		return nil, nil, fmt.Errorf("Control accept: status=%d credit=%d error=%v", status, credit, err)
	}

	success = true
	return inner, func() { _ = outer.Close() }, nil
}
