//go:build linux

package route

import (
	"bytes"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/terminal"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// The lane owner still verifies the framing accepted around terminal bodies.
func TestClosedTerminalBodiesRetainOuterLaneFraming(t *testing.T) {
	nonce, target := [32]byte{1}, [32]byte{2}
	lookup, err := terminal.EncodeDescriptorLookup(nonce, target)
	if err != nil {
		t.Fatal(err)
	}
	publish, err := terminal.EncodeDescriptorPublication(nonce, bytes.Repeat([]byte{3}, reachability.MaximumPrivateDescriptorSize))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{lookup, publish} {
		raw, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindOperation, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		frame, err := ardp.ReadFrame(bytes.NewReader(raw))
		if err != nil || !bytes.Equal(frame.Body, body) {
			t.Fatalf("Descriptor outer lane changed: %v", err)
		}
	}
	join, err := terminal.EncodeJoinRequest(terminal.JoinRequest{Nonce: nonce, Secret: target, Context: [32]byte{3}, Side: 1,
		Deadline: time.Unix(0x01020304, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: join})
	if err != nil || len(frame) != 4112 {
		t.Fatalf("JOIN outer lane changed: %v", err)
	}
}
