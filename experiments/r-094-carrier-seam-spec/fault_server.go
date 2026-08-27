//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/quic-go/quic-go"
)

type faultReady func(string)

type faultPathObservation struct {
	remoteStart string
	remoteEnd   string
}

func serveFaultProfile(ctx context.Context, profile profileID, listen string, certificate tls.Certificate,
	expectedClient [32]byte, binding route.LegBinding, ready faultReady, path *faultPathObservation) error {
	switch profile {
	case tcpProfile:
		return serveFaultTCP(ctx, listen, certificate, expectedClient, binding, ready, path)
	case quicProfile:
		return serveFaultQUIC(ctx, listen, certificate, expectedClient, binding, ready, path)
	default:
		return errIncompatible
	}
}

func serveFaultTCP(ctx context.Context, listen string, certificate tls.Certificate, expectedClient [32]byte,
	binding route.LegBinding, ready faultReady, path *faultPathObservation) error {
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		if err := tcpListener.SetDeadline(binding.NotAfter); err != nil {
			return err
		}
	}
	ready(listener.Addr().String())
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer connection.Close()
	path.remoteStart = connection.RemoteAddr().String()
	path.remoteEnd = path.remoteStart
	secured := tls.Server(connection, serverTLS(certificate, expectedClient))
	if err := secured.SetDeadline(binding.NotAfter); err != nil {
		return err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		return err
	}
	if err := verifyTransportState(secured.ConnectionState(), expectedClient); err != nil {
		return err
	}
	if err := route.AcceptNodeLegBinding(secured, binding); err != nil {
		return err
	}
	if err := faultExchangeServer(secured); err != nil {
		return err
	}
	path.remoteEnd = connection.RemoteAddr().String()
	return secured.Close()
}

func serveFaultQUIC(ctx context.Context, listen string, certificate tls.Certificate, expectedClient [32]byte,
	binding route.LegBinding, ready faultReady, path *faultPathObservation) error {
	listener, err := quic.ListenAddr(listen, serverTLS(certificate, expectedClient), quicServerConfig())
	if err != nil {
		return err
	}
	defer listener.Close()
	ready(listener.Addr().String())
	connection, err := listener.Accept(ctx)
	if err != nil {
		return err
	}
	path.remoteStart = connection.RemoteAddr().String()
	path.remoteEnd = path.remoteStart
	stream, err := connection.AcceptStream(ctx)
	if err != nil {
		_ = connection.CloseWithError(1, "r094-fault-stream")
		return err
	}
	lane := &quicLane{stream: stream, connection: connection}
	if err := lane.SetDeadline(binding.NotAfter); err != nil {
		_ = lane.abort()
		return err
	}
	if err := verifyTransportState(connection.ConnectionState().TLS, expectedClient); err != nil {
		_ = lane.abort()
		return err
	}
	if err := route.AcceptNodeLegBinding(lane, binding); err != nil {
		_ = lane.abort()
		return err
	}
	if err := faultExchangeServer(lane); err != nil {
		path.remoteEnd = connection.RemoteAddr().String()
		_ = lane.abort()
		return err
	}
	path.remoteEnd = connection.RemoteAddr().String()
	if err := stream.Close(); err != nil {
		_ = lane.abort()
		return err
	}
	select {
	case <-connection.Context().Done():
		return nil
	case <-ctx.Done():
		_ = connection.CloseWithError(1, "r094-fault-deadline")
		return ctx.Err()
	case <-time.After(time.Until(binding.NotAfter)):
		_ = connection.CloseWithError(1, "r094-fault-deadline")
		return context.DeadlineExceeded
	}
}
