package route

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

func servePublisher(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	observation := Evidence{Schema: observationSchema, Kind: "complete", Role: "publisher", PID: os.Getpid(),
		ManifestDigest: input.ManifestDigest, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest,
		NodeID: input.NodeID, PreviousPin: input.UpstreamPin}
	if err := validatePublisher(input); err != nil {
		return observation, err
	}
	listener, err := net.Listen("tcp", input.ListenAddress)
	if err != nil {
		return observation, fmt.Errorf("listen for publisher: %w", err)
	}
	defer listener.Close()
	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(input.Deadline))
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	if ready != nil {
		ready(Evidence{Schema: observationSchema, Kind: "ready", Role: "publisher", PID: os.Getpid(),
			NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeID, PreviousPin: input.UpstreamPin})
	}
	connection, err := listener.Accept()
	if err != nil {
		return observation, contextError(ctx, err)
	}
	defer connection.Close()
	cancelConnection := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer cancelConnection()
	deadline := time.Now().Add(input.Deadline)
	_ = connection.SetDeadline(deadline)
	outer := tls.Server(connection, serverTLS(input.Certificate, input.UpstreamPin))
	if err := outer.HandshakeContext(ctx); err != nil {
		return observation, fmt.Errorf("authenticate responder: %w", err)
	}
	if err := acceptLegBinding(outer, input.NetworkID, input.EpochDigest, input.NodeID); err != nil {
		return observation, fmt.Errorf("accept authenticated responder leg: %w", err)
	}
	if input.RawAttachment {
		observation.PeerAuthenticated = true
		defer input.Stream.Close()
		forward, reverse, streamErr := relayOpaque(outer, input.Stream)
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
	observation.PeerAuthenticated = true
	if input.Stream != nil {
		defer input.Stream.Close()
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

func validatePublisher(input Actor) error {
	if !emptyPlan(input.Plan) || input.PublisherPin != [32]byte{} ||
		!emptyCertificate(input.ClientCertificate) || input.NextNodeID != [32]byte{} || input.NextAddress != "" ||
		input.NextPin != [32]byte{} {
		return errors.New("publisher received information outside its role-local duty")
	}
	if input.Role != "publisher" || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.UpstreamPin == [32]byte{} {
		return errors.New("publisher responder identity is required")
	}
	if err := validateEndpoint(input.ListenAddress); err != nil {
		return err
	}
	if err := validateCertificate(input.Certificate); err != nil {
		return err
	}
	if input.RawAttachment {
		if input.Stream == nil || !emptyCertificate(input.ServiceCertificate) {
			return errors.New("raw publisher attachment duty is invalid")
		}
	} else if err := validateCertificate(input.ServiceCertificate); err != nil {
		return err
	}
	return validateDeadline(input.Deadline)
}
