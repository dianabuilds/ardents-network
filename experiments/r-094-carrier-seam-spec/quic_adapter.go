//go:build ignore

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/quic-go/quic-go"
)

type quicLane struct {
	stream     *quic.Stream
	connection *quic.Conn
}

func (lane *quicLane) Read(buffer []byte) (int, error)   { return lane.stream.Read(buffer) }
func (lane *quicLane) Write(buffer []byte) (int, error)  { return lane.stream.Write(buffer) }
func (lane *quicLane) SetDeadline(value time.Time) error { return lane.stream.SetDeadline(value) }
func (lane *quicLane) Close() error {
	lane.stream.CancelRead(0)
	return errors.Join(lane.stream.Close(), lane.connection.CloseWithError(0, "r094-close"))
}

func (lane *quicLane) abort() error {
	lane.stream.CancelRead(1)
	lane.stream.CancelWrite(1)
	return lane.connection.CloseWithError(1, "r094-abort")
}

func openQUIC(ctx context.Context, attempt carrierAttempt) (transportResult, error) {
	connection, err := quic.DialAddr(ctx, attempt.Endpoint,
		clientTLS(attempt.Certificate, attempt.ExpectedPeer), quicClientConfig())
	if err != nil {
		return transportResult{}, err
	}
	stream, err := connection.OpenStreamSync(ctx)
	if err != nil {
		_ = connection.CloseWithError(0, "r094-open-failed")
		return transportResult{}, err
	}
	lane := &quicLane{stream: stream, connection: connection}
	if err := lane.SetDeadline(attempt.Deadline); err != nil {
		_ = lane.Close()
		return transportResult{}, err
	}
	return transportResult{lane: lane, state: connection.ConnectionState().TLS, close: lane.Close}, nil
}

func startQUICPeer(parent context.Context, certificate tls.Certificate, expectedClient [32]byte,
	binding route.LegBinding, mode peerMode) (*peerRuntime, error) {
	ctx, cancel := context.WithCancel(parent)
	listener, err := quic.ListenAddr("127.0.0.1:0", serverTLS(certificate, expectedClient), quicServerConfig())
	if err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	var slot closeSlot
	go func() {
		connection, acceptErr := listener.Accept(ctx)
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		slot.Set(func() { _ = connection.CloseWithError(0, "r094-peer-stop") })
		stream, streamErr := connection.AcceptStream(ctx)
		if streamErr != nil {
			done <- streamErr
			return
		}
		lane := &quicLane{stream: stream, connection: connection}
		if deadlineErr := lane.SetDeadline(binding.NotAfter); deadlineErr != nil {
			_ = lane.abort()
			done <- deadlineErr
			return
		}
		serveErr := servePeer(ctx, lane, binding, mode)
		if serveErr == nil && mode == peerNormal {
			if closeErr := stream.Close(); closeErr != nil {
				_ = lane.abort()
				done <- closeErr
				return
			}
			<-connection.Context().Done()
			done <- nil
			return
		}
		_ = lane.abort()
		done <- serveErr
	}()
	return &peerRuntime{Endpoint: listener.Addr().String(), done: done, stop: func() {
		cancel()
		_ = listener.Close()
		slot.Close()
	}}, nil
}

func startQUICStall(parent context.Context) (*peerRuntime, error) {
	ctx, cancel := context.WithCancel(parent)
	packet, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	go func() {
		buffer := make([]byte, 2048)
		for {
			_ = packet.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			if _, _, readErr := packet.ReadFrom(buffer); readErr != nil {
				select {
				case <-ctx.Done():
					done <- ctx.Err()
					return
				default:
				}
			}
		}
	}()
	return &peerRuntime{Endpoint: packet.LocalAddr().String(), done: done, stop: func() {
		cancel()
		_ = packet.Close()
	}}, nil
}

func quicClientConfig() *quic.Config {
	return &quic.Config{
		Versions: []quic.Version{quic.Version1}, HandshakeIdleTimeout: time.Second,
		MaxIdleTimeout: 5 * time.Second, MaxIncomingStreams: -1, MaxIncomingUniStreams: -1,
		InitialPacketSize:          1200,
		InitialStreamReceiveWindow: 32 << 10, MaxStreamReceiveWindow: 32 << 10,
		InitialConnectionReceiveWindow: 64 << 10, MaxConnectionReceiveWindow: 64 << 10,
		AllowConnectionWindowIncrease: func(*quic.Conn, uint64) bool { return false },
		EnableDatagrams:               false, Allow0RTT: false,
	}
}

func quicServerConfig() *quic.Config {
	config := quicClientConfig()
	config.MaxIncomingStreams = 1
	return config
}

var _ deadlineLane = (*quicLane)(nil)
