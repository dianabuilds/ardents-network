//go:build linux

package route

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func TestClosedDescriptorOperationsHaveExactTerminalFraming(t *testing.T) {
	nonce, target := [32]byte{1}, [32]byte{2}
	lookup, err := EncodeClosedDescriptorLookup(nonce, target)
	if err != nil {
		t.Fatal(err)
	}
	proof := bytes.Repeat([]byte{3}, reachability.MaximumPrivateDescriptorSize)
	publish, err := EncodeClosedDescriptorPublication(nonce, proof)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range [][]byte{lookup, publish} {
		raw, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameOperation, Body: body})
		if err != nil {
			t.Fatal(err)
		}
		frame, err := ReadClosedLaneFrame(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		request, err := DecodeClosedDescriptorRequest(frame.Body)
		if err != nil || request.Nonce != nonce {
			t.Fatalf("terminal request: %+v %v", request, err)
		}
		if body[0] == 2 && (len(body) != 4096 || request.Target != target || len(request.Descriptor) != 0) {
			t.Fatal("lookup contract changed")
		}
		if body[0] == 6 && (len(body) != 16384 || request.Target != [32]byte{} || !bytes.Equal(request.Descriptor, proof)) {
			t.Fatal("publication proof lost")
		}
	}
	for status := uint8(0); status <= 4; status++ {
		payload := proof
		if status != 0 {
			payload = nil
		}
		body, err := EncodeClosedDescriptorResult(nonce, status, payload)
		if err != nil || len(body) != 16384 {
			t.Fatalf("unequal public result size: %v", err)
		}
		got, decoded, err := DecodeClosedDescriptorResult(body, nonce)
		if err != nil || got != status || !bytes.Equal(decoded, payload) {
			t.Fatalf("result changed: %v", err)
		}
		if _, _, err := DecodeClosedDescriptorResult(body, [32]byte{9}); err == nil {
			t.Fatal("result accepted different nonce")
		}
	}
}

func TestClosedDescriptorRejectsMalformedOperationAndResult(t *testing.T) {
	nonce, target := [32]byte{1}, [32]byte{2}
	lookup, err := EncodeClosedDescriptorLookup(nonce, target)
	if err != nil {
		t.Fatal(err)
	}
	publish, err := EncodeClosedDescriptorPublication(nonce, []byte{3})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		body   []byte
		mutate func([]byte)
	}{
		{"lookup padding", lookup, func(b []byte) { b[65] = 1 }},
		{"lookup target", lookup, func(b []byte) { clear(b[33:65]) }},
		{"nonce", lookup, func(b []byte) { clear(b[1:33]) }},
		{"unknown operation", lookup, func(b []byte) { b[0] = 8 }},
		{"wrong small operation", lookup, func(b []byte) { b[0] = 6 }},
		{"publication padding", publish, func(b []byte) { b[36] = 1 }},
		{"publication too large", publish, func(b []byte) { binary.BigEndian.PutUint16(b[33:35], 15001) }},
		{"publication length overflow", publish, func(b []byte) { binary.BigEndian.PutUint16(b[33:35], 65535) }},
		{"publication empty", publish, func(b []byte) { clear(b[33:35]) }},
		{"wrong large operation", publish, func(b []byte) { b[0] = 2 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := bytes.Clone(test.body)
			test.mutate(body)
			if _, err := DecodeClosedDescriptorRequest(body); err == nil {
				t.Fatal("malformed operation accepted")
			}
		})
	}
	for _, body := range [][]byte{nil, lookup[:4095], append(bytes.Clone(lookup), 0), publish[:16383], append(bytes.Clone(publish), 0)} {
		if _, err := DecodeClosedDescriptorRequest(body); err == nil {
			t.Fatal("incorrect request length accepted")
		}
	}
	result, err := EncodeClosedDescriptorResult(nonce, 0, []byte{3})
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[0] ^= 1 }, func(b []byte) { b[32] = 5 }, func(b []byte) { b[32] = 1 }, func(b []byte) { b[38] = 1 },
		func(b []byte) { binary.BigEndian.PutUint32(b[33:37], 15001) }, func(b []byte) { binary.BigEndian.PutUint32(b[33:37], ^uint32(0)) },
	} {
		body := bytes.Clone(result)
		mutate(body)
		if _, _, err := DecodeClosedDescriptorResult(body, nonce); err == nil {
			t.Fatal("malformed result accepted")
		}
	}
	for _, body := range [][]byte{nil, result[:16383], append(bytes.Clone(result), 0)} {
		if _, _, err := DecodeClosedDescriptorResult(body, nonce); err == nil {
			t.Fatal("incorrect response length accepted")
		}
	}
	if _, err := EncodeClosedDescriptorPublication(nonce, make([]byte, 15001)); err == nil {
		t.Fatal("oversized proof admitted")
	}
	if _, err := EncodeClosedDescriptorResult(nonce, 1, []byte{3}); err == nil {
		t.Fatal("refusal carried proof")
	}
}
