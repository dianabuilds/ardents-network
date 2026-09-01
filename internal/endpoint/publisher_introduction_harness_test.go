package endpoint

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// introductionTestHarness is the Endpoint package's behavioral peer. It
// proves the participant boundary without exposing a Node role starter.
type introductionTestHarness struct {
	listener net.Listener
	done     chan struct{}

	mu         sync.Mutex
	connection net.Conn
	once       sync.Once
}

func startIntroductionTestHarness(address string, certificate tls.Certificate, network, digest, nodeID [32]byte,
	epoch uint64, deadline time.Time, admit route.EndpointTransitBindingAdmitter,
) (*introductionTestHarness, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	harness := &introductionTestHarness{listener: listener, done: make(chan struct{})}
	go harness.serve(certificate, network, digest, nodeID, epoch, deadline, admit)
	return harness, nil
}

func (harness *introductionTestHarness) serve(certificate tls.Certificate, network, digest, nodeID [32]byte,
	epoch uint64, deadline time.Time, admit route.EndpointTransitBindingAdmitter,
) {
	defer close(harness.done)
	raw, err := harness.listener.Accept()
	if err != nil {
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	accepted, err := route.AcceptEndpointTransitAttachment(ctx, raw, route.EndpointTransitAttachmentAcceptance{
		NetworkID: network, Digest: digest, TransitNodeID: nodeID, Epoch: epoch, TransitRole: route.IntroductionRole,
		Deadline: deadline, AdmissionDeadline: deadline, Certificate: certificate, Admit: admit,
	})
	cancel()
	if err != nil {
		_ = raw.Close()
		return
	}
	harness.mu.Lock()
	harness.connection = accepted.Connection
	harness.mu.Unlock()
	record, err := route.ReadIntroductionControlRecord(accepted.Connection)
	if err != nil || record.Registration == nil {
		_ = accepted.Connection.Close()
		return
	}
	registration := record.Registration
	if err := route.WriteIntroductionSlotReady(accepted.Connection, route.IntroductionSlotReady{
		Reachability: registration.Reachability, JoinHandle: registration.JoinHandle, NotAfter: registration.NotAfter,
	}); err != nil {
		_ = accepted.Connection.Close()
		return
	}
	_ = accepted.Connection.SetDeadline(registration.NotAfter)
	buffer := make([]byte, 1)
	_, _ = accepted.Connection.Read(buffer)
	_ = accepted.Connection.Close()
}

func (harness *introductionTestHarness) Close() error {
	if harness == nil {
		return nil
	}
	var result error
	harness.once.Do(func() {
		result = harness.listener.Close()
		if errors.Is(result, net.ErrClosed) {
			result = nil
		}
		harness.mu.Lock()
		connection := harness.connection
		harness.mu.Unlock()
		if connection != nil {
			result = errors.Join(result, connection.Close())
			if errors.Is(result, net.ErrClosed) {
				result = nil
			}
		}
		<-harness.done
	})
	return result
}
