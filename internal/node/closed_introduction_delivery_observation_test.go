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
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeIntroduction, 3, 1)
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
			registrationTrace.mu.Lock()
			registered := observedChannel{bytes.Clone(registrationTrace.sent), bytes.Clone(registrationTrace.received)}
			registrationTrace.mu.Unlock()
			sentFrames, readFrames := decodeIntroductionTrace(t, registered.Sent), decodeIntroductionTrace(t, registered.Received)
			if len(sentFrames) != 6 || len(readFrames) != 7 {
				t.Fatal("registration transcript missed a delivery or cleanup")
			}
			evidence := struct {
				Carrier      route.CarrierProfile
				Registration observedChannel
				Submissions  []observedChannel
				Durable      map[string][]byte
				Limit        string
			}{carrier, registered, submissions, captureIntroductionFiles(t, fixture.admissionRoot), "protocol boundary only; no receiving heap or other-role/privacy verdict"}
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
