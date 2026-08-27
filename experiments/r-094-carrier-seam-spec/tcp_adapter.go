//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func openTCP(ctx context.Context, attempt carrierAttempt) (transportResult, error) {
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", attempt.Endpoint)
	if err != nil {
		return transportResult{}, err
	}
	if tcpConnection, ok := connection.(*net.TCPConn); ok {
		_ = tcpConnection.SetWriteBuffer(16 << 10)
	}
	secured := tls.Client(connection, clientTLS(attempt.Certificate, attempt.ExpectedPeer))
	if err := secured.SetDeadline(attempt.Deadline); err != nil {
		_ = connection.Close()
		return transportResult{}, err
	}
	if err := secured.HandshakeContext(ctx); err != nil {
		_ = connection.Close()
		return transportResult{}, err
	}
	return transportResult{lane: secured, state: secured.ConnectionState(), close: connection.Close}, nil
}

func startTCPPeer(parent context.Context, certificate tls.Certificate, expectedClient [32]byte,
	binding route.LegBinding, mode peerMode) (*peerRuntime, error) {
	ctx, cancel := context.WithCancel(parent)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	var slot closeSlot
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		slot.Set(func() { _ = connection.Close() })
		if tcpConnection, ok := connection.(*net.TCPConn); ok {
			_ = tcpConnection.SetReadBuffer(16 << 10)
		}
		secured := tls.Server(connection, serverTLS(certificate, expectedClient))
		defer secured.Close()
		if deadlineErr := secured.SetDeadline(binding.NotAfter); deadlineErr != nil {
			done <- deadlineErr
			return
		}
		if handshakeErr := secured.HandshakeContext(ctx); handshakeErr != nil {
			done <- handshakeErr
			return
		}
		done <- servePeer(ctx, secured, binding, mode)
	}()
	return &peerRuntime{Endpoint: listener.Addr().String(), done: done, stop: func() {
		cancel()
		_ = listener.Close()
		slot.Close()
	}}, nil
}

func startTCPStall(parent context.Context) (*peerRuntime, error) {
	ctx, cancel := context.WithCancel(parent)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	var slot closeSlot
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		slot.Set(func() { _ = connection.Close() })
		<-ctx.Done()
		_ = connection.Close()
		done <- ctx.Err()
	}()
	return &peerRuntime{Endpoint: listener.Addr().String(), done: done, stop: func() {
		cancel()
		_ = listener.Close()
		slot.Close()
	}}, nil
}

var _ deadlineLane = (*tls.Conn)(nil)
