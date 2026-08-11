package nativecircuit

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const attachedCopyBufferBytes = 32 * 1024

func runEndpointUserAttached(ctx context.Context, transport net.Conn, trust endpointTrust, nonce handle, application net.Conn, setupVerified func() error) (endpointObservation, error) {
	if transport == nil || application == nil || trust.Roots == nil {
		return endpointObservation{}, errors.New("attached User endpoint is incomplete")
	}
	defer transport.Close()
	defer application.Close()
	secured := tls.Client(transport, userStreamTLSConfig(trust))
	if err := secured.HandshakeContext(ctx); err != nil {
		return endpointObservation{}, fmt.Errorf("authenticate exact Target/Instance: %w", candidatePeerReadFailure(err))
	}
	observation, err := observeEndpointTLS(secured.ConnectionState())
	if err != nil {
		return observation, errors.Join(errCandidateContractFailure, err)
	}
	applyAttachedDeadline(ctx, secured, application)
	if err := exchangeUserCanary(secured, nonce); err != nil {
		return observation, err
	}
	if setupVerified != nil {
		if err := setupVerified(); err != nil {
			return observation, err
		}
	}
	return proxyAttached(ctx, secured, application, observation)
}

func runEndpointServiceAttached(ctx context.Context, transport net.Conn, certificate tls.Certificate, nonce handle, application net.Conn) (endpointObservation, error) {
	if transport == nil || application == nil || len(certificate.Certificate) == 0 {
		return endpointObservation{}, errors.New("attached Service endpoint is incomplete")
	}
	defer transport.Close()
	defer application.Close()
	secured := tls.Server(transport, &tls.Config{
		Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{endpointALPN}, SessionTicketsDisabled: true,
	})
	if err := secured.HandshakeContext(ctx); err != nil {
		return endpointObservation{}, fmt.Errorf("active Instance TLS handshake: %w", candidatePeerReadFailure(err))
	}
	observation, err := observeEndpointTLS(secured.ConnectionState())
	if err != nil {
		return observation, errors.Join(errCandidateContractFailure, err)
	}
	applyAttachedDeadline(ctx, secured, application)
	if err := exchangeServiceCanary(secured, nonce); err != nil {
		return observation, err
	}
	return proxyAttached(ctx, secured, application, observation)
}

type attachedCopyResult struct {
	bytes int64
	err   error
}

func proxyAttached(ctx context.Context, route, application net.Conn, observation endpointObservation) (endpointObservation, error) {
	completed := make(chan attachedCopyResult, 2)
	copyStream := func(destination io.Writer, source io.Reader) {
		buffer := make([]byte, attachedCopyBufferBytes)
		count, err := io.CopyBuffer(destination, source, buffer)
		completed <- attachedCopyResult{bytes: count, err: err}
	}
	go copyStream(route, application)
	go copyStream(application, route)
	var first attachedCopyResult
	select {
	case <-ctx.Done():
		_ = route.Close()
		_ = application.Close()
		return observation, ctx.Err()
	case first = <-completed:
	}
	_ = route.Close()
	_ = application.Close()
	second := <-completed
	observation.ApplicationBytes = int(first.bytes + second.bytes)
	observation.ApplicationBytesVerified = benignAttachedClose(first.err)
	observation.QueueHighWaterBytes = attachedCopyBufferBytes
	if !observation.ApplicationBytesVerified {
		return observation, first.err
	}
	if !benignAttachedClose(second.err) {
		return observation, second.err
	}
	return observation, nil
}

func benignAttachedClose(err error) bool {
	return err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe)
}

func applyAttachedDeadline(ctx context.Context, connections ...net.Conn) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}
	for _, connection := range connections {
		_ = connection.SetDeadline(deadline)
	}
}
