//go:build linux

package node

import (
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedIntroductionRegistrationOwnsSlotUntilExpiry(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeIntroduction, 3)
			request := route.ClosedRegistrationRequest{Nonce: [32]byte{101}, Slot: [32]byte{102}, Revision: 1, Expiry: time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)}
			first, closeFirst, status := registerIntroductionFixture(t, fixture, 0, request)
			defer closeFirst()
			if status != 0 {
				t.Fatal("valid Publication registration refused")
			}
			_, closeDuplicate, status := registerIntroductionFixture(t, fixture, 1, request)
			closeDuplicate()
			if status != 1 {
				t.Fatal("duplicate slot replaced its owning channel")
			}
			withdraw := route.ClosedRegistrationRequest{Nonce: [32]byte{103}, Slot: request.Slot, Revision: request.Revision, Withdraw: true}
			if status := sendRegistrationFixture(t, first, withdraw); status != 0 {
				t.Fatal("owning withdrawal refused")
			}
			closeFirst()
			_, closeReclaim, status := registerIntroductionFixture(t, fixture, 2, request)
			closeReclaim()
			if status != 1 {
				t.Fatal("withdrawn slot was reclaimed within its expiry")
			}
			request.Slot[0]++
			request.Nonce[0]++
			request.Expiry = time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)
			_, closeLost, status := registerIntroductionFixture(t, fixture, 3, request)
			if status != 0 {
				t.Fatal("fresh slot refused")
			}
			closeLost()
			_, closeAfterLoss, status := registerIntroductionFixture(t, fixture, 4, request)
			closeAfterLoss()
			if status != 1 {
				t.Fatal("channel loss made old slot reclaimable")
			}
			if _, closeReplay, err := fixture.openTerminal(t.Context(), fixture.tokens[3], 3); err == nil {
				closeReplay()
				t.Fatal("spent Publication token admitted")
			}
			if _, closeWrong, err := fixture.openTerminal(t.Context(), fixture.tokens[5], 1); err == nil {
				closeWrong()
				t.Fatal("wrong admission class accepted")
			}
			request.Slot[0]++
			request.Nonce[0]++
			request.Expiry = time.Now().UTC().Add(5 * time.Second).Truncate(time.Second)
			valid, closeValid, status := registerIntroductionFixture(t, fixture, 5, request)
			defer closeValid()
			if status != 0 {
				t.Fatal("wrong-class attempt spent a valid class-3 token")
			}
			withdraw.Nonce = request.Nonce // Replaying the request nonce must close the registration.
			withdraw.Slot, withdraw.Revision = request.Slot, request.Revision
			body, err := route.EncodeClosedRegistrationRequest(withdraw)
			if err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(valid, route.ClosedLaneFrame{Kind: 10, Body: body}); err != nil {
				t.Fatal(err)
			}
			if frame, err := route.ReadClosedLaneFrame(valid); err == nil {
				t.Fatalf("replayed nonce accepted: %+v", frame)
			}
		})
	}
}

func registerIntroductionFixture(t *testing.T, fixture *resolutionNetworkFixture, token int, request route.ClosedRegistrationRequest) (net.Conn, func(), uint8) {
	t.Helper()
	connection, closeCarrier, err := fixture.openTerminal(t.Context(), fixture.tokens[token], 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeCarrier)
	return connection, closeCarrier, sendRegistrationFixture(t, connection, request)
}

func sendRegistrationFixture(t *testing.T, connection net.Conn, request route.ClosedRegistrationRequest) uint8 {
	t.Helper()
	body, err := route.EncodeClosedRegistrationRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Body: body}); err != nil {
		t.Fatal(err)
	}
	frame, err := route.ReadClosedLaneFrame(connection)
	if err != nil || frame.Kind != 11 || frame.Lane != 0 {
		t.Fatalf("registration result: %v", err)
	}
	status, proof, err := route.DecodeClosedDescriptorResult(frame.Body, request.Nonce)
	if err != nil || len(proof) != 0 {
		t.Fatalf("registration result payload: %v", err)
	}
	return status
}
