//go:build linux

package node

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func sendJoinFixture(t *testing.T, fixture *resolutionNetworkFixture, token int, side uint8) (net.Conn, [32]byte) {
	t.Helper()
	connection, closeConnection, err := fixture.openTerminal(t.Context(), fixture.tokens[token], 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeConnection)
	request := route.ClosedJoinRequest{Nonce: [32]byte{byte(token + 1), 101}, Secret: [32]byte{102}, Context: [32]byte{103}, Side: side, Deadline: time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)}
	raw, err := route.EncodeClosedJoinRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Lane: 1, Body: raw}); err != nil {
		t.Fatal(err)
	}
	return connection, request.Nonce
}

func TestClosedJoinNodePairsThroughBothCarriers(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeDataJoin, 2)
			first, firstNonce := sendJoinFixture(t, fixture, 0, 1)
			type resultRead struct {
				frame route.ClosedLaneFrame
				err   error
			}
			firstResult := make(chan resultRead, 1)
			readerDone := make(chan struct{})
			t.Cleanup(func() {
				if err := first.(*tls.Conn).NetConn().(*outerTestInnerConn).outer.SetDeadline(time.Now()); err != nil && !errors.Is(err, net.ErrClosed) {
					t.Error(err)
				}
				select {
				case <-readerDone:
				case <-time.After(3 * time.Second):
					t.Error("first-result reader did not join")
				}
			})
			go func() {
				defer close(readerDone)
				frame, err := route.ReadClosedLaneFrame(first)
				firstResult <- resultRead{frame, err}
			}()
			select {
			case result := <-firstResult:
				t.Fatalf("unpaired JOIN produced result: %v", result.err)
			case <-time.After(25 * time.Millisecond):
			}
			duplicate, duplicateNonce := sendJoinFixture(t, fixture, 1, 1)
			refusal, err := route.ReadClosedLaneFrame(duplicate)
			if err != nil {
				t.Fatal(err)
			}
			if status, err := route.DecodeClosedJoinResult(refusal.Body, duplicateNonce); err != nil || status != 1 {
				t.Fatal("duplicate side not refused locally")
			}
			// Duplicate refusal establishes the first reservation exists at the owner.
			select {
			case result := <-firstResult:
				t.Fatalf("reserved unpaired JOIN acknowledged: %v", result.err)
			default:
			}
			second, secondNonce := sendJoinFixture(t, fixture, 2, 2)
			result, err := route.ReadClosedLaneFrame(second)
			if err != nil {
				t.Fatal(err)
			}
			if status, err := route.DecodeClosedJoinResult(result.Body, secondNonce); err != nil || status != 0 || result.Lane != 1 {
				t.Fatal("second JOIN did not activate")
			}
			select {
			case result := <-firstResult:
				if result.err != nil {
					t.Fatal(result.err)
				}
				if status, err := route.DecodeClosedJoinResult(result.frame.Body, firstNonce); err != nil || status != 0 || result.frame.Lane != 1 {
					t.Fatal("first JOIN lost local nonce")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("first JOIN did not pair")
			}
			transfer := func(from, to net.Conn, kind uint8, body []byte) {
				t.Helper()
				if err := route.WriteClosedLaneFrame(from, route.ClosedLaneFrame{Kind: kind, Lane: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
				frame, err := route.ReadClosedLaneFrame(to)
				if err != nil {
					t.Fatal(err)
				}
				if frame.Kind != kind || frame.Lane != 1 || !bytes.Equal(frame.Body, body) {
					t.Fatal("Node changed framed data")
				}
			}
			// Actual time crosses the original JOIN deadline on each Carrier; the
			// established data lane must retain its longer original class-2 admission.
			wait := time.NewTimer(11 * time.Second)
			defer wait.Stop()
			select {
			case <-wait.C:
			case <-t.Context().Done():
				t.Fatal(t.Context().Err())
			}
			transfer(first, second, 6, []byte("opaque Service TLS fixture bytes"))
			transfer(first, second, 8, nil)
			transfer(second, first, 6, []byte("reverse remains live after EOF"))
			transfer(second, first, 9, []byte{0})
			for _, connection := range []net.Conn{first, second} {
				outer := connection.(*tls.Conn).NetConn().(*outerTestInnerConn).outer
				if err := outer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					t.Fatal(err)
				}
				for {
					terminal, err := route.ReadClosedLaneFrame(outer)
					if err != nil {
						t.Fatal(err)
					}
					if terminal.Kind == 9 {
						if terminal.Lane != 1 || len(terminal.Body) != 1 || terminal.Body[0] != 0 {
							t.Fatalf("successful JOIN emitted outer refusal: %+v", terminal)
						}
						break
					}
				}
			}
		})
	}
}
