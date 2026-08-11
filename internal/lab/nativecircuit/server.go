package nativecircuit

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"sync"
)

type rendezvousAttachment struct {
	JoinToken     handle `json:"join_token"`
	AttemptHandle handle `json:"attempt_handle"`
}

type introductionRegistrationFrame struct {
	Slot handle `json:"slot"`
}

type introductionDeliveryFrame struct {
	Slot   handle `json:"slot"`
	Sealed []byte `json:"sealed"`
}

func serveRelay(ctx context.Context, listener net.Listener, certificate tls.Certificate, allowedNext []string, connections int) error {
	return serveRelayObserved(ctx, listener, certificate, allowedNext, connections, nil)
}

func serveRelayObserved(ctx context.Context, listener net.Listener, certificate tls.Certificate, allowedNext []string, connections int, observed func(string)) error {
	return serveNodeConnections(ctx, listener, certificate, connections, func(connection net.Conn) error {
		next, err := handleRelayConnection(ctx, connection, allowedNext)
		if err == nil && observed != nil {
			observed("route.next_address=" + next)
		}
		return err
	})
}

func serveRendezvous(ctx context.Context, listener net.Listener, certificate tls.Certificate, connections int) error {
	return serveRendezvousObserved(ctx, listener, certificate, connections, nil)
}

func serveRendezvousObserved(ctx context.Context, listener net.Listener, certificate tls.Certificate, connections int, observed func(string)) error {
	manager := newRendezvousManager()
	return serveNodeConnections(ctx, listener, certificate, connections, func(connection net.Conn) error {
		request, err := readFrame(connection)
		if err != nil || request.Type != frameRendezvousRegister && request.Type != frameRendezvousAttach {
			return errors.New("rendezvous expected a register or attach frame")
		}
		var attachment rendezvousAttachment
		if err := decodeFixedJSON(request.Payload, &attachment); err != nil {
			return err
		}
		side := "user"
		if request.Type == frameRendezvousAttach {
			side = "service"
		}
		if observed != nil {
			observed("rendezvous." + side + ".attempt_handle")
			observed("rendezvous.join_token")
		}
		return manager.join(ctx, side, attachment.JoinToken, attachment.AttemptHandle, connection)
	})
}

func serveIntroduction(ctx context.Context, listener net.Listener, certificate tls.Certificate, connections int) error {
	return serveIntroductionObserved(ctx, listener, certificate, connections, nil)
}

func serveIntroductionObserved(ctx context.Context, listener net.Listener, certificate tls.Certificate, connections int, observed func(string)) error {
	manager := newIntroductionManager()
	return serveNodeConnections(ctx, listener, certificate, connections, func(connection net.Conn) error {
		request, err := readFrame(connection)
		if err != nil {
			return err
		}
		switch request.Type {
		case frameIntroductionRegister:
			var registration introductionRegistrationFrame
			if err := decodeFixedJSON(request.Payload, &registration); err != nil {
				return err
			}
			if observed != nil {
				observed("introduction.opaque_slot")
			}
			return manager.register(ctx, registration.Slot, connection)
		case frameIntroductionDeliver:
			var delivery introductionDeliveryFrame
			if err := decodeFixedJSON(request.Payload, &delivery); err != nil {
				return err
			}
			if observed != nil {
				observed("introduction.opaque_slot")
				observed("introduction.sealed_invitation")
			}
			if err := manager.deliver(ctx, delivery.Slot, delivery.Sealed); err != nil {
				return err
			}
			return writeFrame(connection, frame{Type: frameIntroductionAcknowledge, Payload: []byte("accepted")})
		default:
			return errors.New("introduction received an invalid state transition")
		}
	})
}

func serveNodeConnections(ctx context.Context, listener net.Listener, certificate tls.Certificate, expected int, handler func(net.Conn) error) error {
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	var wait sync.WaitGroup
	errorsSeen := make(chan error, max(expected, 1))
	accepted := 0
	for expected == 0 || accepted < expected {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return err
		}
		accepted++
		wait.Add(1)
		go func() {
			defer wait.Done()
			defer connection.Close()
			secured := tls.Server(connection, &tls.Config{
				Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
				CurvePreferences: []tls.CurveID{tls.X25519}, NextProtos: []string{nodeALPN}, SessionTicketsDisabled: true,
			})
			if err := secured.HandshakeContext(ctx); err != nil {
				errorsSeen <- err
				return
			}
			stopConnection := context.AfterFunc(ctx, func() { _ = secured.Close() })
			defer stopConnection()
			if err := handler(secured); err != nil && ctx.Err() == nil {
				errorsSeen <- err
			}
		}()
	}
	wait.Wait()
	close(errorsSeen)
	var result error
	for err := range errorsSeen {
		result = errors.Join(result, err)
	}
	return result
}

func decodeFixedJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.More() {
		return errors.New("native circuit control frame has invalid encoding")
	}
	return nil
}
