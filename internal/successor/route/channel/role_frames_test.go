package channel

import (
	"bytes"
	"encoding/hex"
	"io"
	"net"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These checks establish channel handshake framing only. No successful
// Network authority, token spend or receiving Admission is supplied here.
func TestRoleAcceptanceCanonicalBytesAndExactCredit(t *testing.T) {
	want, err := hex.DecodeString("415244500003050000000000000000050000010000")
	if err != nil {
		t.Fatal(err)
	}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	done := make(chan error, 1)
	go func() { done <- Accept(local) }()
	actual := make([]byte, len(want))
	if _, err := io.ReadFull(peer, actual); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil || !bytes.Equal(actual, want) {
		t.Fatal("role acceptance wire changed", actual, err)
	}
	for _, change := range []string{"exact", "credit", "refusal"} {
		t.Run(change, func(t *testing.T) {
			local, peer := net.Pipe()
			defer local.Close()
			defer peer.Close()
			wire := bytes.Clone(want)
			if change == "credit" {
				wire[len(wire)-1] = 1
			}
			if change == "refusal" {
				wire[16] = 1
				clear(wire[17:])
			}
			go func() { _, err := peer.Write(wire); done <- err }()
			err := Accepted(local)
			if writeErr := <-done; writeErr != nil {
				t.Fatal(writeErr)
			}
			if (err == nil) != (change == "exact") {
				t.Fatal("wrong role acceptance outcome", err)
			}
		})
	}
}

func TestRoleHelloRefusesAnotherValidControlKind(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	done := make(chan error, 1)
	go func() { done <- Accept(peer) }()
	if _, err := ReadHello(local); err == nil {
		t.Fatal("ACCEPT substituted for HELLO")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The rejected control was syntactically valid, so this checks role stage
	// ordering rather than only the codec's invalid-header refusal.
	if !ardp.ValidFrame(ardp.Frame{Kind: ardp.KindAccept, Body: []byte{0, 0, 1, 0, 0}}) {
		t.Fatal("negative control is not a valid acceptance frame")
	}
}
