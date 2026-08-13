package route

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func serveNode(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	observation := Evidence{Schema: observationSchema, Kind: "complete", Role: input.Role,
		PID: os.Getpid(), ManifestDigest: input.ManifestDigest, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest,
		NodeID: input.NodeID, PreviousPin: input.UpstreamPin, NextNodeID: input.NextNodeID}
	if err := validateNode(input); err != nil {
		return observation, err
	}
	listener, err := net.Listen("tcp", input.ListenAddress)
	if err != nil {
		return observation, fmt.Errorf("listen for %s: %w", input.Role, err)
	}
	defer listener.Close()
	_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(input.Deadline))
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	if ready != nil {
		ready(Evidence{Schema: observationSchema, Kind: "ready", Role: input.Role, PID: os.Getpid(),
			NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeID,
			PreviousPin: input.UpstreamPin, NextNodeID: input.NextNodeID})
	}
	upstream, err := listener.Accept()
	if err != nil {
		return observation, contextError(ctx, err)
	}
	defer upstream.Close()
	cancelUpstream := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer cancelUpstream()
	deadline := time.Now().Add(input.Deadline)
	_ = upstream.SetDeadline(deadline)
	securedUpstream := tls.Server(upstream, serverTLS(input.Certificate, input.UpstreamPin))
	if err := securedUpstream.HandshakeContext(ctx); err != nil {
		return observation, fmt.Errorf("authenticate upstream: %w", err)
	}
	if err := acceptLegBinding(securedUpstream, input.NetworkID, input.EpochDigest, input.NodeID); err != nil {
		return observation, fmt.Errorf("accept authenticated leg binding: %w", err)
	}
	downstream, err := dialTLS(ctx, input.NextAddress, input.Certificate, input.NextPin, input.Deadline)
	if err != nil {
		return observation, fmt.Errorf("authenticate next role: %w", err)
	}
	defer downstream.Close()
	cancelDownstream := context.AfterFunc(ctx, func() { _ = downstream.Close() })
	defer cancelDownstream()
	_ = downstream.SetDeadline(deadline)
	if err := confirmLegBinding(downstream, input.NetworkID, input.EpochDigest, input.NextNodeID); err != nil {
		return observation, fmt.Errorf("confirm next authenticated leg binding: %w", err)
	}
	observation.PeerAuthenticated = true
	count, digest, err := relayOpaque(securedUpstream, downstream)
	observation.OpaqueBytes, observation.OpaqueDigest = count, digest
	if err != nil {
		return observation, err
	}
	return observation, nil
}

func validateNode(input Actor) error {
	if !emptyPlan(input.Plan) || input.PublisherPin != [32]byte{} ||
		!emptyCertificate(input.ClientCertificate) || !emptyCertificate(input.ServiceCertificate) {
		return errors.New("node received information outside its role-local duty")
	}
	roleOK := false
	for _, role := range routeRoles {
		roleOK = roleOK || input.Role == role
	}
	if !roleOK || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} || input.NodeID == [32]byte{} ||
		input.UpstreamPin == [32]byte{} || input.NextNodeID == [32]byte{} || input.NextPin == [32]byte{} {
		return errors.New("role-local carrier duty is invalid")
	}
	if err := validateEndpoint(input.ListenAddress); err != nil {
		return err
	}
	if err := validateEndpoint(input.NextAddress); err != nil {
		return err
	}
	if err := validateCertificate(input.Certificate); err != nil {
		return err
	}
	return validateDeadline(input.Deadline)
}

func relayOpaque(upstream, downstream net.Conn) (uint64, [32]byte, error) {
	hash := sha256.New()
	type copyResult struct {
		count int64
		err   error
	}
	results := make(chan copyResult, 2)
	copyDirection := func(destination, source net.Conn, record bool) {
		writer := io.Writer(destination)
		if record {
			writer = io.MultiWriter(destination, hash)
		}
		count, err := io.Copy(writer, io.LimitReader(source, 64<<10))
		if closer, ok := destination.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
		results <- copyResult{count, err}
	}
	go copyDirection(downstream, upstream, true)
	go copyDirection(upstream, downstream, false)
	first, second := <-results, <-results
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	if first.err != nil || second.err != nil {
		return uint64(first.count), digest, errors.Join(first.err, second.err)
	}
	return uint64(first.count), digest, nil
}

func contextError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
