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

	"github.com/dianabuilds/ardents-network/internal/route"
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
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			fixture := newPrivateRecipientNetworkFixture(t, carrier, route.ClosedPurposeIntroduction, 3)
			capture := new(introductionTranscript)
			fixture.observe = func(connection net.Conn) net.Conn { return &introductionTranscriptConn{connection, capture} }
			request := route.ClosedRegistrationRequest{Revision: 1, Expiry: time.Now().UTC().Add(30 * time.Second).Truncate(time.Second)}
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
			withdrawal := route.ClosedRegistrationRequest{Slot: request.Slot, Revision: request.Revision, Withdraw: true}
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
			hello, err := route.DecodeClosedHello(requests[0].Body)
			if err != nil || hello.Purpose != route.ClosedPurposeIntroduction || hello.RecipientNodeID != fixture.receiver.NodeID {
				t.Fatal("capture missed receiver binding")
			}
			if len(requests[1].Body) != 355 || requests[1].Body[0] != 3 || !bytes.Equal(requests[1].Body[1:], fixture.tokens[0]) {
				t.Fatal("capture missed actual admitted token")
			}
			decoded, err := route.DecodeClosedRegistrationRequest(requests[2].Body)
			if err != nil || decoded != request {
				t.Fatal("capture missed visible registration fields")
			}
			decoded, err = route.DecodeClosedRegistrationRequest(requests[3].Body)
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
				Carrier        route.CarrierProfile
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
func decodeIntroductionTrace(t *testing.T, raw []byte) []route.ClosedLaneFrame {
	t.Helper()
	input := bytes.NewReader(raw)
	var frames []route.ClosedLaneFrame
	for input.Len() != 0 {
		frame, err := route.ReadClosedLaneFrame(input)
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
