//go:build linux

package node

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
)

// Captures exact plaintext bytes at the transmitting role-TLS boundary and
// actual durable receiving files. This is component evidence, not complete P3:
// receiver heap, transport metadata, provisioning and combined observers remain
// outside this capture. Successful receiving ACKs anchor the exchange.
type introductionTranscript struct {
	mu             sync.Mutex
	sent, received []byte
}
type introductionTranscriptConn struct {
	net.Conn
	capture *introductionTranscript
}

func (connection *introductionTranscriptConn) Write(p []byte) (int, error) {
	n, err := connection.Conn.Write(p)
	connection.capture.mu.Lock()
	defer connection.capture.mu.Unlock()
	if len(connection.capture.sent)+n > 128<<10 {
		return n, errors.New("capture send bound exceeded")
	}
	connection.capture.sent = append(connection.capture.sent, p[:n]...)
	return n, err
}
func (connection *introductionTranscriptConn) Read(p []byte) (int, error) {
	n, err := connection.Conn.Read(p)
	connection.capture.mu.Lock()
	defer connection.capture.mu.Unlock()
	if len(connection.capture.received)+n > 128<<10 {
		return n, errors.New("capture receive bound exceeded")
	}
	connection.capture.received = append(connection.capture.received, p[:n]...)
	return n, err
}

func TestClosedIntroductionRegistrationObservation(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, ardp.PurposeIntroduction, 3)
			capture := new(introductionTranscript)
			fixture.observe = func(connection net.Conn) net.Conn { return &introductionTranscriptConn{connection, capture} }
			request := terminal.RegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)}
			if _, err := rand.Read(request.Nonce[:]); err != nil {
				t.Fatal(err)
			}
			if _, err := rand.Read(request.Slot[:]); err != nil {
				t.Fatal(err)
			}
			connection, closeCarrier, status := registerIntroductionFixture(t, fixture, 0, request)
			defer closeCarrier()
			if status != 0 {
				t.Fatal("registration positive control refused")
			}
			before := captureIntroductionFiles(t, fixture.admissionRoot)
			withdrawal := terminal.RegistrationRequest{Slot: request.Slot, Revision: request.Revision, Withdraw: true}
			if _, err := rand.Read(withdrawal.Nonce[:]); err != nil {
				t.Fatal(err)
			}
			if sendRegistrationFixture(t, connection, withdrawal) != 0 {
				t.Fatal("withdrawal positive control refused")
			}
			closeCarrier()
			after := captureIntroductionFiles(t, fixture.admissionRoot)
			capture.mu.Lock()
			sent, received := bytes.Clone(capture.sent), bytes.Clone(capture.received)
			capture.mu.Unlock()
			requests := decodeIntroductionTrace(t, sent)
			responses := decodeIntroductionTrace(t, received)
			if len(requests) != 4 || requests[0].Kind != 1 || requests[1].Kind != 2 || requests[2].Kind != 10 || requests[3].Kind != 10 || len(responses) != 3 || responses[0].Kind != 5 || responses[1].Kind != 11 || responses[2].Kind != 11 {
				t.Fatal("missing or extra protocol bytes in capture")
			}
			hello, err := ardp.DecodeHello(requests[0].Body)
			if err != nil || hello.Purpose != ardp.PurposeIntroduction || hello.RecipientNodeID != fixture.receiver.NodeID {
				t.Fatal("capture missed receiver binding")
			}
			if len(requests[1].Body) != 355 || requests[1].Body[0] != 3 || !bytes.Equal(requests[1].Body[1:], fixture.tokens[0]) {
				t.Fatal("capture missed actual admitted token")
			}
			decoded, err := terminal.DecodeRegistrationRequest(requests[2].Body)
			if err != nil || decoded != request {
				t.Fatal("capture missed visible registration fields")
			}
			decoded, err = terminal.DecodeRegistrationRequest(requests[3].Body)
			if err != nil || decoded != withdrawal {
				t.Fatal("capture missed owning withdrawal")
			}
			// Absence of a raw identifier is not an unlinkability verdict: explicitly
			// locate its linkable SHA256 floor in the actual receiving files.
			digest := sha256.Sum256(request.Slot[:])
			found := false
			for name, raw := range before {
				if bytes.Contains(raw, request.Slot[:]) {
					t.Fatal("durable slot retained raw identifier")
				}
				if bytes.Contains(raw, digest[:]) {
					found = true
					if !bytes.Equal(raw, after[name]) {
						t.Fatal("withdrawal changed retained slot floor")
					}
				}
			}
			if !found {
				t.Fatal("capture missed positive hashed-slot floor control")
			}
			evidence := struct {
				Carrier        routecarrier.CarrierProfile
				Sent, Received []byte
				Before, After  map[string][]byte
				Limit          string
			}{carrier, sent, received, before, after, "role TLS boundary and durable files only; incomplete P3"}
			if output := os.Getenv("ARDENTS_INTRODUCTION_OBSERVATIONS"); output != "" {
				if !filepath.IsAbs(output) {
					t.Fatal("capture output must be absolute")
				}
				raw, err := json.Marshal(evidence)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(output, string(carrier)+".json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("captured %d sent bytes, %d received bytes, %d durable files before/after withdrawal", len(sent), len(received), len(before))
		})
	}
}
func decodeIntroductionTrace(t *testing.T, raw []byte) []ardp.Frame {
	t.Helper()
	input := bytes.NewReader(raw)
	var frames []ardp.Frame
	for input.Len() != 0 {
		frame, err := ardp.ReadFrame(input)
		if err != nil {
			t.Fatalf("truncated capture: %v", err)
		}
		frames = append(frames, frame)
	}
	return frames
}
func captureIntroductionFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatal("unexpected receiving root entry")
		}
		file, err := os.Open(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(file, 1<<20+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil || len(raw) > 1<<20 {
			t.Fatalf("incomplete receiving file capture: %v / %v", readErr, closeErr)
		}
		out[entry.Name()] = raw
	}
	return out
}

// Two real admitted submissions expose the receiving Introduction's remapping
// of channel nonces while preserving the opaque capsule. This is a protocol
// boundary observation, not recipient decryption or a complete P3 verdict.
func TestClosedIntroductionDeliveryObservation(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, ardp.PurposeIntroduction, 3, 1)
			registrationTrace := new(introductionTranscript)
			fixture.observe = func(connection net.Conn) net.Conn { return &introductionTranscriptConn{connection, registrationTrace} }
			request := terminal.RegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)}
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
				submitter.receiver.ExpectedPurpose = ardp.PurposeSubmission
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
				capsule := introductioncapsule.Capsule{Slot: request.Slot, Revision: request.Revision, Expiry: end, Ciphertext: make([]byte, 360)}
				for _, value := range [][]byte{nonce[:], capsule.DeliveryNonce[:], capsule.Encapsulation[:], capsule.Ciphertext} {
					if _, err := rand.Read(value); err != nil {
						t.Fatal(err)
					}
				}
				operation, err := introductioncapsule.EncodeSubmission(nonce, capsule)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(connection, ardp.Frame{Kind: 10, Body: operation}); err != nil {
					t.Fatal(err)
				}
				delivered, err := ardp.ReadFrame(registration)
				if err != nil {
					t.Fatal(err)
				}
				if delivered.Kind != 10 || delivered.Lane != uint32(2*(index+1)) {
					t.Fatal("unexpected receiving delivery lane")
				}
				forwarded, received, err := introductioncapsule.DecodeSubmission(delivered.Body)
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
				result, err := terminal.EncodeDescriptorResult(forwarded, 0, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err := ardp.WriteFrame(registration, ardp.Frame{Kind: 11, Lane: delivered.Lane, Body: result}); err != nil {
					t.Fatal(err)
				}
				closed, err := ardp.ReadFrame(registration)
				if err != nil || closed.Kind != 9 || closed.Lane != delivered.Lane || !bytes.Equal(closed.Body, []byte{0}) {
					t.Fatalf("delivery CLOSE: %v", err)
				}
				reply, err := ardp.ReadFrame(connection)
				if err != nil || reply.Kind != 11 || reply.Lane != 0 {
					t.Fatalf("submission reply: %v", err)
				}
				verdict, proof, err := terminal.DecodeDescriptorResult(reply.Body, nonce)
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
			withdrawal := terminal.RegistrationRequest{Slot: request.Slot, Revision: request.Revision, Withdraw: true}
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
				Carrier      routecarrier.CarrierProfile
				Registration observedChannel
				Submissions  []observedChannel
				Durable      map[string][]byte
				Limit        string
			}{carrier, registered, submissions, captureIntroductionFiles(t, fixture.admissionRoot), "observed registration, delivery, replies, withdrawal and durable artifacts; transient receiver state, Node supervision and other roles unobserved; incomplete P3"}
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
