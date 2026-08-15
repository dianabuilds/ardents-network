package route

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"
)

const maximumRoleAttachments = 16

func serveNodeCapacity(ctx context.Context, input Actor, ready func(Evidence), observation Evidence) (Evidence, error) {
	return serveCapacity(ctx, input, ready, observation, carryNodeConnection)
}

func carryNodeConnection(ctx context.Context, input Actor, upstream net.Conn, observation Evidence,
	admit func() bool) (Evidence, error) {
	defer upstream.Close()
	if err := configureCarrierLiveness(upstream); err != nil {
		return observation, fmt.Errorf("configure upstream Carrier liveness: %w", err)
	}
	cancelUpstream := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer cancelUpstream()
	deadline := time.Now().Add(input.Deadline)
	if err := upstream.SetDeadline(deadline); err != nil {
		return observation, fmt.Errorf("set upstream setup deadline: %w", err)
	}
	securedUpstream := tls.Server(upstream, serverTLS(input.Certificate, input.UpstreamPin))
	if err := securedUpstream.HandshakeContext(ctx); err != nil {
		return observation, fmt.Errorf("authenticate upstream: %w", err)
	}
	if err := acceptLegBinding(securedUpstream, input.NetworkID, input.EpochDigest, input.NodeID); err != nil {
		return observation, fmt.Errorf("accept authenticated leg binding: %w", err)
	}
	if !admit() {
		return observation, errAttachmentCapacity
	}
	downstream, err := dialTLS(ctx, input.NextAddress, input.Certificate, input.NextPin, input.Deadline)
	if err != nil {
		return observation, fmt.Errorf("authenticate next role: %w", err)
	}
	defer downstream.Close()
	cancelDownstream := context.AfterFunc(ctx, func() { _ = downstream.Close() })
	defer cancelDownstream()
	if err := downstream.SetDeadline(deadline); err != nil {
		return observation, fmt.Errorf("set downstream setup deadline: %w", err)
	}
	if err := confirmLegBinding(downstream, input.NetworkID, input.EpochDigest, input.NextNodeID); err != nil {
		return observation, fmt.Errorf("confirm next authenticated leg binding: %w", err)
	}
	if err := errors.Join(securedUpstream.SetDeadline(time.Time{}), downstream.SetDeadline(time.Time{})); err != nil {
		return observation, fmt.Errorf("clear %s authenticated leg setup deadlines: %w", input.Role, err)
	}
	observation.PeerAuthenticated = true
	forward, reverse, relayErr := relayOpaque(securedUpstream, downstream)
	observation.OpaqueBytes, observation.OpaqueDigest = forward.count, forward.digest
	observation.ReverseOpaqueBytes, observation.ReverseOpaqueDigest = reverse.count, reverse.digest
	if relayErr != nil && ctx.Err() != nil {
		return observation, ctx.Err()
	}
	if relayErr != nil && !benignStreamError(relayErr) {
		return observation, relayErr
	}
	return observation, nil
}
