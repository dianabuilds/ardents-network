package connection

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestExchangeContinuityReturnsOnlyVerifiedPeerState(t *testing.T) {
	t.Parallel()
	client, publisher := net.Pipe()
	defer client.Close()
	defer publisher.Close()
	key, context, exporter := [32]byte{1}, [32]byte{2}, [32]byte{3}
	results := make(chan struct {
		peer ContinuityPeer
		err  error
	}, 2)
	for _, input := range []ContinuityExchange{
		{Key: key, Context: context, ExporterCommitment: exporter, Role: RoleClient, Generation: 2, SendBase: 4, SendEnd: 9, ReceiveNext: 7},
		{Key: key, Context: context, ExporterCommitment: exporter, Role: RolePublisher, Generation: 2, SendBase: 7, SendEnd: 11, ReceiveNext: 9},
	} {
		carrier := client
		if input.Role == RolePublisher {
			carrier = publisher
		}
		go func(carrier net.Conn, input ContinuityExchange) {
			peer, err := ExchangeContinuity(t.Context(), carrier, input)
			results <- struct {
				peer ContinuityPeer
				err  error
			}{peer, err}
		}(carrier, input)
	}
	for range 2 {
		result := <-results
		if result.err != nil || result.peer.PeerNonce == [32]byte{} || result.peer.LocalNonce == [32]byte{} ||
			result.peer.PeerNonce == result.peer.LocalNonce {
			t.Fatalf("continuity exchange failed: peer=%+v err=%v", result.peer, result.err)
		}
	}
}

func TestExchangeContinuityRejectsMalformedPeer(t *testing.T) {
	t.Parallel()
	client, publisher := net.Pipe()
	defer client.Close()
	defer publisher.Close()
	go func() {
		_, _ = Read(publisher)
		_ = Write(publisher, Record{Terminal: &Terminal{AttachmentGeneration: 1}})
	}()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err := ExchangeContinuity(ctx, client, ContinuityExchange{Key: [32]byte{1}, Context: [32]byte{2},
		ExporterCommitment: [32]byte{3}, Role: RoleClient, Generation: 1})
	if !errors.Is(err, ErrContinuityViolation) {
		t.Fatalf("malformed peer was accepted: %v", err)
	}
}
