//go:build linux

package node

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Two real admitted submissions expose the receiving Introduction's remapping
// of channel nonces while preserving the opaque capsule. This is a protocol
// boundary observation, not recipient decryption or a complete P3 verdict.
func TestClosedIntroductionDeliveryObservation(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			var receiver *closedIntroductionServer
			fixture := newPrivateRecipientNetworkFixtureWithStart(t, carrier, route.ClosedPurposeIntroduction, 3, observedIntroductionStart(&receiver), 1)
			observations := []introductionReceiverObservation{observeIntroductionReceiver(t, receiver, "startup")}
			registrationTrace := new(introductionTranscript)
			fixture.observe = func(connection net.Conn) net.Conn { return &introductionTranscriptConn{connection, registrationTrace} }
			request := route.ClosedRegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)}
			for _, value := range [][]byte{request.Nonce[:], request.Slot[:]} {
				if _, err := rand.Read(value); err != nil {
					t.Fatal(err)
				}
			}
			registration, closeRegistration, status := registerIntroductionFixture(t, fixture, 0, request)
			defer closeRegistration()
			if status != 0 {
				t.Fatal("real registration refused")
			}
			observations = append(observations, observeIntroductionReceiver(t, receiver, "registered"))
			if slots := observations[len(observations)-1].Slots; len(slots) != 1 || slots[0].Request != request || !slots[0].Active || !slots[0].TLSHandshakeComplete {
				t.Fatal("receiving snapshot missed actual registration")
			}
			var previous [32]byte
			type observedChannel struct{ Sent, Received []byte }
			submissions := make([]observedChannel, 0, 2)
			for index := 0; index < 2; index++ {
				submitter := *fixture
				submitter.receiver.ExpectedPurpose = route.ClosedPurposeSubmission
				trace := new(introductionTranscript)
				submitter.observe = func(connection net.Conn) net.Conn { return &introductionTranscriptConn{connection, trace} }
				connection, closeSubmission, err := submitter.openTerminal(t.Context(), fixture.supplementary[1][index], 1)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(closeSubmission)
				end := time.Now().UTC().Add(8 * time.Second).Truncate(time.Second)
				if err := connection.SetDeadline(end); err != nil {
					t.Fatal(err)
				}
				if err := registration.SetDeadline(end); err != nil {
					t.Fatal(err)
				}
				var nonce [32]byte
				capsule := route.ClosedIntroductionCapsule{Slot: request.Slot, Revision: request.Revision, Expiry: end, Ciphertext: make([]byte, 360)}
				for _, value := range [][]byte{nonce[:], capsule.DeliveryNonce[:], capsule.Encapsulation[:], capsule.Ciphertext} {
					if _, err := rand.Read(value); err != nil {
						t.Fatal(err)
					}
				}
				operation, err := route.EncodeClosedIntroductionSubmission(nonce, capsule)
				if err != nil {
					t.Fatal(err)
				}
				if err := route.WriteClosedLaneFrame(connection, route.ClosedLaneFrame{Kind: 10, Body: operation}); err != nil {
					t.Fatal(err)
				}
				delivered, err := route.ReadClosedLaneFrame(registration)
				if err != nil {
					t.Fatal(err)
				}
				if delivered.Kind != 10 || delivered.Lane != uint32(2*(index+1)) {
					t.Fatal("unexpected receiving delivery lane")
				}
				forwarded, received, err := route.DecodeClosedIntroductionSubmission(delivered.Body)
				if err != nil {
					t.Fatal(err)
				}
				if forwarded == nonce || forwarded == capsule.DeliveryNonce || forwarded == previous {
					t.Fatal("Introduction reused a channel nonce")
				}
				pending := observeIntroductionReceiver(t, receiver, "delivery awaiting acknowledgement")
				if len(pending.Slots) != 1 || pending.Slots[0].InFlight != 1 || len(pending.Slots[0].Pending) != 1 || pending.Slots[0].Pending[0].Nonce != forwarded || pending.Slots[0].Pending[0].Lane != delivered.Lane || pending.Slots[0].Pending[0].Acknowledged || pending.Slots[0].Pending[0].End != capsule.Expiry {
					t.Fatal("receiving snapshot missed actual pending delivery")
				}
				observations = append(observations, pending)
				previous = forwarded
				if !bytes.Equal(operation[33:], delivered.Body[33:]) || received.DeliveryNonce != capsule.DeliveryNonce || !bytes.Equal(received.Ciphertext, capsule.Ciphertext) {
					t.Fatal("Introduction changed opaque capsule")
				}
				result, err := route.EncodeClosedDescriptorResult(forwarded, 0, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := route.WriteClosedLaneFrame(registration, route.ClosedLaneFrame{Kind: 11, Lane: delivered.Lane, Body: result}); err != nil {
					t.Fatal(err)
				}
				closed, err := route.ReadClosedLaneFrame(registration)
				if err != nil || closed.Kind != 9 || closed.Lane != delivered.Lane || !bytes.Equal(closed.Body, []byte{0}) {
					t.Fatalf("delivery CLOSE: %v", err)
				}
				reply, err := route.ReadClosedLaneFrame(connection)
				if err != nil || reply.Kind != 11 || reply.Lane != 0 {
					t.Fatalf("submission reply: %v", err)
				}
				verdict, proof, err := route.DecodeClosedDescriptorResult(reply.Body, nonce)
				if err != nil || verdict != 0 || len(proof) != 0 {
					t.Fatalf("submission nonce handback: %v", err)
				}
				closeSubmission()
				settled := observeIntroductionReceiver(t, receiver, "delivery completed")
				if len(settled.Slots) != 1 || settled.Slots[0].InFlight != 0 || len(settled.Slots[0].Pending) != 0 {
					t.Fatal("completed submission retained pending receiving state")
				}
				observations = append(observations, settled)
				trace.mu.Lock()
				sent, read := bytes.Clone(trace.sent), bytes.Clone(trace.received)
				trace.mu.Unlock()
				sentFrames, readFrames := decodeIntroductionTrace(t, sent), decodeIntroductionTrace(t, read)
				if len(sentFrames) != 3 || len(readFrames) != 2 || sentFrames[1].Kind != 2 || sentFrames[1].Body[0] != 1 || !bytes.Equal(sentFrames[2].Body, operation) || !bytes.Equal(readFrames[1].Body, reply.Body) {
					t.Fatal("incomplete submission observation")
				}
				submissions = append(submissions, observedChannel{sent, read})
			}
			withdrawal := route.ClosedRegistrationRequest{Slot: request.Slot, Revision: request.Revision, Withdraw: true}
			if _, err := rand.Read(withdrawal.Nonce[:]); err != nil {
				t.Fatal(err)
			}
			if sendRegistrationFixture(t, registration, withdrawal) != 0 {
				t.Fatal("owning withdrawal refused")
			}
			closeRegistration()
			withdrawn := observeIntroductionReceiver(t, receiver, "withdrawn")
			if len(withdrawn.Slots) != 1 || withdrawn.Slots[0].Active || !withdrawn.Slots[0].Done || withdrawn.Slots[0].InFlight != 0 || len(withdrawn.Slots[0].Pending) != 0 || withdrawn.Slots[0].Request != request {
				t.Fatal("withdrawal failed to retain only inactive replay-protection state")
			}
			observations = append(observations, withdrawn)
			registrationTrace.mu.Lock()
			registered := observedChannel{bytes.Clone(registrationTrace.sent), bytes.Clone(registrationTrace.received)}
			registrationTrace.mu.Unlock()
			sentFrames, readFrames := decodeIntroductionTrace(t, registered.Sent), decodeIntroductionTrace(t, registered.Received)
			if len(sentFrames) != 6 || len(readFrames) != 7 {
				t.Fatal("registration transcript missed a delivery or cleanup")
			}
			evidence := struct {
				Carrier        route.CarrierProfile
				Registration   observedChannel
				Submissions    []observedChannel
				Durable        map[string][]byte
				ReceiverStates []introductionReceiverObservation
				Limit          string
			}{carrier, registered, submissions, captureIntroductionFiles(t, fixture.admissionRoot), observations, "actual receiver slot state and protocol; transient admission/TLS internals, Node supervision and other roles unobserved; incomplete P3"}
			if output := os.Getenv("ARDENTS_INTRODUCTION_OBSERVATIONS"); output != "" {
				if !filepath.IsAbs(output) {
					t.Fatal("capture output must be absolute")
				}
				raw, err := json.Marshal(evidence)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(output, "delivery-"+string(carrier)+".json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Log("captured two admitted submissions, independent hop nonces, unchanged opaque capsules and complete replies/CLOSE/withdrawal")
		})
	}
}
