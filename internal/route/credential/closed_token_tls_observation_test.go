//go:build linux

package credential

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// The synchronous client owns these buffers; the listener uses its actual TLS
// connection and admission state. This records plaintext above verified TLS,
// not encrypted packets or the receiver's transient handshake state.
type issuerTLSObservationConn struct {
	net.Conn
	sent, received []byte
}

func (connection *issuerTLSObservationConn) Write(p []byte) (int, error) {
	n, err := connection.Conn.Write(p)
	if len(connection.sent)+n > 128<<10 {
		return n, errors.New("issuer TLS send observation exceeded bound")
	}
	connection.sent = append(connection.sent, p[:n]...)
	return n, err
}
func (connection *issuerTLSObservationConn) Read(p []byte) (int, error) {
	n, err := connection.Conn.Read(p)
	if len(connection.received)+n > 128<<10 {
		return n, errors.New("issuer TLS receive observation exceeded bound")
	}
	connection.received = append(connection.received, p[:n]...)
	return n, err
}
func issuerObservedFrames(t *testing.T, raw []byte) []route.ClosedLaneFrame {
	t.Helper()
	input := bytes.NewReader(raw)
	var frames []route.ClosedLaneFrame
	for input.Len() > 0 {
		frame, err := route.ReadClosedLaneFrame(input)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, frame)
	}
	return frames
}
func checkIssuerTLSObservation(t *testing.T, issuer *ClosedTokenIssuer, listener *ClosedTokenListener, carrier route.CarrierProfile,
	expectedServer [32]byte, connection *issuerTLSObservationConn, operation, result []byte, before issuerStateObservation) {
	t.Helper()
	requests, responses := issuerObservedFrames(t, connection.sent), issuerObservedFrames(t, connection.received)
	if len(requests) != 3 || requests[0].Kind != 1 || requests[1].Kind != 3 || requests[2].Kind != 10 || !bytes.Equal(requests[2].Body, operation) || len(responses) != 2 || responses[0].Kind != 5 || responses[1].Kind != 11 || !bytes.Equal(responses[1].Body, result) {
		t.Fatal("incomplete actual issuer TLS protocol observation")
	}
	outcome, err := route.DecodeClosedIssuanceResult(result, [32]byte{71})
	if err != nil || outcome.Status != 0 {
		t.Fatalf("TLS issuer did not report successful issuance: %v", err)
	}
	batch, err := DecodeClosedTokenBatchResult(outcome.Payload)
	if err != nil || batch.Status != ClosedTokenIssued || len(batch.Signatures) != 1 {
		t.Fatalf("TLS issuer omitted actual signature: %v", err)
	}
	after := observeClosedIssuer(t, issuer, "after actual TLS issuance and listener drain")
	if len(before.Reservations) != 0 || len(after.Reservations) != 1 || after.Reservations[0].Count != 1 || listener.Active() != 0 {
		t.Fatal("TLS issuer did not retain exactly one debit and join its connection")
	}
	evidence := struct {
		Carrier                     route.CarrierProfile
		ExpectedServer              [32]byte
		LocalAddress, RemoteAddress string
		Sent, Received              []byte
		Before, After               issuerStateObservation
		Limit                       string
	}{carrier, expectedServer, connection.LocalAddr().String(), connection.RemoteAddr().String(), connection.sent, connection.received, before, after,
		"real direct role TLS13 with pinned server and exact plaintext frames plus issuer state; State/provisioning fixtures; transient TLS/controller/combined Endpoint observations absent; incomplete P3"}
	if output := os.Getenv("ARDENTS_ISSUER_OBSERVATIONS"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("issuer evidence directory must be absolute")
		}
		raw, err := json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, "tls-"+string(carrier)+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("captured actual pinned role TLS request/result, durable debit and joined listener; incomplete P3")
}
