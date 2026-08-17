package route

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

func servePublisherCapacity(ctx context.Context, input Actor, ready func(Evidence), observation Evidence) (Evidence, error) {
	return serveCapacity(ctx, input, ready, observation, carryPublisherConnection)
}

func carryPublisherConnection(ctx context.Context, input Actor, connection net.Conn, observation Evidence,
	admit func() bool) (Evidence, error) {
	defer connection.Close()
	if err := configureCarrierLiveness(connection); err != nil {
		return observation, fmt.Errorf("configure responder Carrier liveness: %w", err)
	}
	cancelConnection := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer cancelConnection()
	if err := connection.SetDeadline(time.Now().Add(input.Deadline)); err != nil {
		return observation, fmt.Errorf("set responder setup deadline: %w", err)
	}
	outer := tls.Server(connection, serverTLS(input.Certificate, input.UpstreamPin))
	if err := outer.HandshakeContext(ctx); err != nil {
		return observation, fmt.Errorf("authenticate responder: %w", err)
	}
	if err := acceptLegBinding(outer, input.NetworkID, input.EpochDigest, input.NodeID); err != nil {
		return observation, fmt.Errorf("accept authenticated responder leg: %w", err)
	}
	if !admit() {
		return observation, errAttachmentCapacity
	}
	if input.RawAttachment {
		if err := outer.SetDeadline(time.Time{}); err != nil {
			return observation, fmt.Errorf("clear publisher raw Attachment setup deadline: %w", err)
		}
		observation.PeerAuthenticated = true
		stream, owned, openErr := publisherAttachmentStream(ctx, input.Stream)
		if openErr != nil {
			return observation, fmt.Errorf("open publisher attachment stream: %w", openErr)
		}
		forward, reverse, streamErr := relayOpaque(outer, stream)
		if owned {
			streamErr = errors.Join(streamErr, stream.Close())
		}
		observation.OpaqueBytes, observation.OpaqueDigest = forward.count, forward.digest
		observation.ReverseOpaqueBytes, observation.ReverseOpaqueDigest = reverse.count, reverse.digest
		if streamErr != nil && !benignStreamError(streamErr) {
			return observation, fmt.Errorf("carry raw publisher attachment: %w", streamErr)
		}
		return observation, nil
	}
	inner := tls.Server(outer, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{input.ServiceCertificate}, SessionTicketsDisabled: true})
	if err := inner.HandshakeContext(ctx); err != nil {
		return observation, fmt.Errorf("accept end-to-end canary session: %w", err)
	}
	if err := inner.SetDeadline(time.Time{}); err != nil {
		return observation, fmt.Errorf("clear publisher end-to-end setup deadline: %w", err)
	}
	observation.PeerAuthenticated = true
	if input.Stream != nil {
		forward, reverse, streamErr := relayOpaque(inner, input.Stream)
		observation.OpaqueBytes, observation.OpaqueDigest = forward.count, forward.digest
		observation.ReverseOpaqueBytes, observation.ReverseOpaqueDigest = reverse.count, reverse.digest
		if streamErr != nil && !benignStreamError(streamErr) {
			return observation, fmt.Errorf("carry bounded publisher stream: %w", streamErr)
		}
		return observation, nil
	}
	value, err := readCanary(inner)
	if err != nil {
		return observation, fmt.Errorf("read canary: %w", err)
	}
	observation.CanaryLength, observation.CanaryDigest = uint32(len(value)), sha256.Sum256(value)
	if err := writeReceipt(inner, value); err != nil {
		return observation, fmt.Errorf("write canary receipt: %w", err)
	}
	return observation, nil
}

func publisherAttachmentStream(ctx context.Context, stream io.ReadWriteCloser) (io.ReadWriteCloser, bool, error) {
	source, ok := stream.(attachmentStreamSource)
	if !ok {
		return stream, false, nil
	}
	opened, err := source.OpenAttachment(ctx)
	return opened, true, err
}
