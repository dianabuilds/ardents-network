package ardp_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestDescriptorCanonicalOperationBytes(t *testing.T) {
	nonce, target := [32]byte{0x81}, [32]byte{0x42}
	proof := bytes.Repeat([]byte{0xa5}, 15000)
	lookup := make([]byte, 4096)
	lookup[0], lookup[1], lookup[33] = 2, 0x81, 0x42
	publish := make([]byte, 16384)
	publish[0], publish[1], publish[33], publish[34] = 6, 0x81, 0x3a, 0x98
	copy(publish[35:], proof)
	for _, test := range []struct {
		name    string
		request ardp.DescriptorRequest
		wire    []byte
	}{
		{"lookup", ardp.DescriptorRequest{Operation: ardp.DescriptorLookup, Nonce: nonce, Target: target}, lookup},
		{"publish-maximum", ardp.DescriptorRequest{Operation: ardp.DescriptorPublish, Nonce: nonce, Proof: proof}, publish},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := ardp.EncodeDescriptorRequest(test.request)
			if err != nil || !bytes.Equal(encoded, test.wire) {
				t.Fatalf("canonical operation differs: %v", err)
			}
			decoded, err := ardp.DecodeDescriptorRequest(test.wire)
			if err != nil || decoded.Operation != test.request.Operation || decoded.Nonce != nonce || decoded.Target != test.request.Target || !bytes.Equal(decoded.Proof, test.request.Proof) {
				t.Fatalf("independent wire refused or changed: %+v %v", decoded, err)
			}
			if len(decoded.Proof) != 0 {
				test.wire[35] = 0x17
				if decoded.Proof[0] != 0x17 {
					t.Fatal("decoder's documented borrowed lifetime changed")
				}
			}
		})
	}
}

func TestDescriptorRefusesMalformedOperations(t *testing.T) {
	lookup := make([]byte, 4096)
	lookup[0], lookup[1], lookup[33] = 2, 1, 2
	publish := make([]byte, 16384)
	publish[0], publish[1], publish[34], publish[35] = 6, 1, 1, 3
	for _, test := range []struct {
		name   string
		base   []byte
		change func([]byte) []byte
	}{
		{"truncated", lookup, func(b []byte) []byte { return b[:33] }},
		{"trailing", lookup, func(b []byte) []byte { return append(b, 0) }},
		{"second-operation-padding", lookup, func(b []byte) []byte { b[65] = 6; return b }},
		{"unknown-operation", lookup, func(b []byte) []byte { b[0] = 1; return b }},
		{"zero-nonce", lookup, func(b []byte) []byte { b[1] = 0; return b }},
		{"zero-target", lookup, func(b []byte) []byte { b[33] = 0; return b }},
		{"lookup-large-body", lookup, func(b []byte) []byte { return append(b, make([]byte, 12288)...) }},
		{"publish-small-body", publish, func(b []byte) []byte { return b[:4096] }},
		{"empty-proof", publish, func(b []byte) []byte { b[34] = 0; return b }},
		{"over-proof-bound", publish, func(b []byte) []byte { binary.BigEndian.PutUint16(b[33:35], 15001); return b }},
		{"max-u16-proof", publish, func(b []byte) []byte { b[33], b[34] = 255, 255; return b }},
		{"publication-padding", publish, func(b []byte) []byte { b[16383] = 1; return b }},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := test.change(bytes.Clone(test.base))
			if _, err := ardp.DecodeDescriptorRequest(body); err == nil {
				t.Fatal("malformed operation accepted")
			}
		})
	}
	for _, request := range []ardp.DescriptorRequest{
		{Operation: ardp.DescriptorLookup, Nonce: [32]byte{1}, Target: [32]byte{2}, Proof: []byte{3}},
		{Operation: ardp.DescriptorPublish, Nonce: [32]byte{1}, Target: [32]byte{2}, Proof: []byte{3}},
		{Operation: ardp.DescriptorPublish, Nonce: [32]byte{1}, Proof: make([]byte, 15001)},
	} {
		if _, err := ardp.EncodeDescriptorRequest(request); err == nil {
			t.Fatal("ambiguous or oversized input accepted")
		}
	}
}

func TestDescriptorResultOperationAndNonceBinding(t *testing.T) {
	nonce := [32]byte{0x81}
	for _, operation := range []ardp.DescriptorOperation{ardp.DescriptorLookup, ardp.DescriptorPublish} {
		for status := uint8(0); status <= 4; status++ {
			var proof []byte
			if operation == ardp.DescriptorLookup && status == 0 {
				proof = []byte{0xa5}
			}
			wire := make([]byte, 16384)
			wire[0], wire[32] = 0x81, status
			if len(proof) != 0 {
				wire[36], wire[37] = 1, 0xa5
			}
			encoded, err := ardp.EncodeDescriptorResult(operation, nonce, status, proof)
			if err != nil || !bytes.Equal(encoded, wire) {
				t.Fatalf("canonical RESULT differs: %v", err)
			}
			got, borrowed, err := ardp.DecodeDescriptorResult(wire, operation, nonce)
			if err != nil || got != status || !bytes.Equal(borrowed, proof) {
				t.Fatalf("independent RESULT refused: %v", err)
			}
			if _, _, err = ardp.DecodeDescriptorResult(wire, operation, [32]byte{0x82}); err == nil {
				t.Fatal("foreign request nonce accepted")
			}
			wire[16383] = 1
			if _, _, err = ardp.DecodeDescriptorResult(wire, operation, nonce); err == nil {
				t.Fatal("RESULT padding accepted")
			}
		}
	}
	for _, test := range []struct {
		operation ardp.DescriptorOperation
		status    uint8
		length    uint32
	}{
		{ardp.DescriptorLookup, 0, 0},  // An empty success cannot represent a lookup.
		{ardp.DescriptorPublish, 0, 1}, // Store acknowledgement cannot carry a proof.
		{ardp.DescriptorLookup, 1, 1},
		{ardp.DescriptorLookup, 5, 0},
		{ardp.DescriptorOperation(1), 0, 0},
		{ardp.DescriptorLookup, 0, 15001},
		{ardp.DescriptorLookup, 0, ^uint32(0)},
	} {
		wire := make([]byte, 16384)
		wire[0], wire[32] = 0x81, test.status
		binary.BigEndian.PutUint32(wire[33:37], test.length)
		if _, _, err := ardp.DecodeDescriptorResult(wire, test.operation, nonce); err == nil {
			t.Fatalf("invalid result accepted: %+v", test)
		}
	}
}

func FuzzDescriptorOperationDecoders(f *testing.F) {
	lookup := make([]byte, 4096)
	lookup[0], lookup[1], lookup[33] = 2, 1, 2
	publish := make([]byte, 16384)
	publish[0], publish[1], publish[34], publish[35] = 6, 1, 1, 3
	f.Add(lookup)
	f.Add(publish)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = ardp.DecodeDescriptorRequest(body)
		for _, operation := range []ardp.DescriptorOperation{ardp.DescriptorLookup, ardp.DescriptorPublish} {
			_, _, _ = ardp.DecodeDescriptorResult(body, operation, [32]byte{1})
		}
	})
}
