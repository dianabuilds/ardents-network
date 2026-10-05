package ardp_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestJoinRequestMatchesIndependentWire(t *testing.T) {
	// Protocol oracle: operation, local nonce, secret, side, context, u64
	// deadline, then exactly 3990 zero bytes. No encoder supplies this input.
	prefix, err := hex.DecodeString("05" + strings.Repeat("11", 32) + strings.Repeat("22", 32) + "02" + strings.Repeat("33", 32) + "0000000071234567")
	if err != nil {
		t.Fatal(err)
	}
	wire := append(prefix, make([]byte, 3990)...)
	var nonce, secret, context [32]byte
	copy(nonce[:], bytes.Repeat([]byte{0x11}, 32))
	copy(secret[:], bytes.Repeat([]byte{0x22}, 32))
	copy(context[:], bytes.Repeat([]byte{0x33}, 32))
	want := ardp.JoinRequest{Nonce: nonce, Secret: secret, Side: 2, Context: context, Deadline: time.Unix(0x71234567, 0).UTC()}
	got, err := ardp.DecodeJoinRequest(wire)
	if err != nil || got != want {
		t.Fatal("independent JOIN request refused or changed", err)
	}
	encoded, err := ardp.EncodeJoinRequest(want)
	if err != nil || !bytes.Equal(encoded, wire) {
		t.Fatal("JOIN output differs from canonical bytes", err)
	}
}

func TestJoinResultIsFixedEmptyAndBoundToLocalNonce(t *testing.T) {
	var nonce [32]byte
	nonce[0], nonce[31] = 0x41, 0x99
	for status := uint8(0); status <= 4; status++ {
		wire := make([]byte, 16384)
		wire[0], wire[31], wire[32] = 0x41, 0x99, status
		encoded, err := ardp.EncodeJoinResult(nonce, status)
		if err != nil || !bytes.Equal(encoded, wire) {
			t.Fatal("JOIN RESULT changed size, nonce, status or empty payload", err)
		}
		actual, err := ardp.DecodeJoinResult(wire, nonce)
		if err != nil || actual != status {
			t.Fatal("independent RESULT refused", err)
		}
		for _, offset := range []int{0, 31, 33, 34, 35, 16383} {
			altered := bytes.Clone(wire)
			altered[offset] ^= 1
			if _, err := ardp.DecodeJoinResult(altered, nonce); err == nil {
				t.Fatalf("accepted wrong nonce, payload length or padding at %d", offset)
			}
		}
	}
	if _, err := ardp.EncodeJoinResult(nonce, 5); err == nil {
		t.Fatal("unknown result status accepted")
	}
}

func TestJoinRequestRejectsMalformedAndConcatenatedOperations(t *testing.T) {
	// Sparse literal fields keep the decoder checks independent of its encoder.
	wire := make([]byte, 4096)
	wire[0], wire[1], wire[33], wire[65], wire[66], wire[105] = 5, 1, 2, 1, 3, 1
	if _, err := ardp.DecodeJoinRequest(wire); err != nil {
		t.Fatal("valid independent control", err)
	}
	for name, change := range map[string]func([]byte) []byte{
		"short":           func(b []byte) []byte { return b[:4095] },
		"concatenated":    func(b []byte) []byte { return append(b, b...) },
		"wrong-operation": func(b []byte) []byte { b[0] = 4; return b },
		"zero-nonce":      func(b []byte) []byte { b[1] = 0; return b },
		"zero-secret":     func(b []byte) []byte { b[33] = 0; return b },
		"zero-context":    func(b []byte) []byte { b[66] = 0; return b },
		"zero-side":       func(b []byte) []byte { b[65] = 0; return b },
		"unknown-side":    func(b []byte) []byte { b[65] = 3; return b },
		"zero-deadline":   func(b []byte) []byte { b[105] = 0; return b },
		"overflow":        func(b []byte) []byte { b[98] = 0x80; return b },
		"padding-first":   func(b []byte) []byte { b[106] = 1; return b },
		"padding-last":    func(b []byte) []byte { b[4095] = 1; return b },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ardp.DecodeJoinRequest(change(bytes.Clone(wire))); err == nil {
				t.Fatal("invalid JOIN accepted")
			}
		})
	}
	request, err := ardp.DecodeJoinRequest(wire)
	if err != nil {
		t.Fatal(err)
	}
	request.Deadline = request.Deadline.Add(time.Nanosecond)
	if _, err := ardp.EncodeJoinRequest(request); err == nil {
		t.Fatal("fractional deadline silently rounded")
	}
}
