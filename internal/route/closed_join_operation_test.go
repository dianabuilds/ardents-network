//go:build linux

package route

import (
	"bytes"
	"testing"
	"time"
)

func TestClosedJoinOperationMatchesApprovedWire(t *testing.T) {
	request := ClosedJoinRequest{Nonce: [32]byte{0x11}, Secret: [32]byte{0x22}, Side: 2, Context: [32]byte{0x33}, Deadline: time.Unix(0x01020304, 0).UTC()}
	raw, err := EncodeClosedJoinRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	// Independent exact offsets: operation + nonce + secret + side + context +
	// big-endian u64 seconds; the remainder belongs to this same 4096-byte body.
	expected := make([]byte, 4096)
	expected[0], expected[1], expected[33], expected[65], expected[66] = 5, 0x11, 0x22, 2, 0x33
	copy(expected[98:106], []byte{0, 0, 0, 0, 1, 2, 3, 4})
	if !bytes.Equal(raw, expected) {
		t.Fatal("JOIN differs from approved transcript")
	}
	decoded, err := DecodeClosedJoinRequest(expected)
	if err != nil || decoded != request {
		t.Fatalf("decode exact transcript: %+v %v", decoded, err)
	}
	frame, err := EncodeClosedLaneFrame(ClosedLaneFrame{Kind: closedFrameOperation, Lane: 1, Body: expected})
	if err != nil || len(frame) != 4112 {
		t.Fatalf("JOIN lane framing: %v", err)
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { b[0] = 4 }, func(b []byte) { clear(b[1:33]) }, func(b []byte) { clear(b[33:65]) },
		func(b []byte) { b[65] = 0 }, func(b []byte) { b[65] = 3 }, func(b []byte) { clear(b[66:98]) },
		func(b []byte) { clear(b[98:106]) }, func(b []byte) { b[98] = 0x80 }, func(b []byte) { b[106] = 1 }, func(b []byte) { b[4095] = 1 },
	} {
		changed := bytes.Clone(expected)
		mutate(changed)
		if _, err := DecodeClosedJoinRequest(changed); err == nil {
			t.Fatal("invalid JOIN accepted")
		}
	}
	for _, changed := range [][]byte{nil, raw[:4095], append(bytes.Clone(raw), 0)} {
		if _, err := DecodeClosedJoinRequest(changed); err == nil {
			t.Fatal("wrong JOIN size accepted")
		}
	}
	request.Deadline = request.Deadline.Add(time.Nanosecond)
	if _, err := EncodeClosedJoinRequest(request); err == nil {
		t.Fatal("fractional deadline silently truncated")
	}
}

func TestClosedJoinResultsRequireLocalNonceAndEmptyPayload(t *testing.T) {
	nonce := [32]byte{7}
	for status := uint8(0); status <= 4; status++ {
		raw, err := EncodeClosedJoinResult(nonce, status)
		if err != nil || len(raw) != 16384 {
			t.Fatalf("result: %v", err)
		}
		if got, err := DecodeClosedJoinResult(raw, nonce); err != nil || got != status {
			t.Fatalf("local result: %d %v", got, err)
		}
		if _, err := DecodeClosedJoinResult(raw, [32]byte{8}); err == nil {
			t.Fatal("foreign nonce accepted")
		}
		for _, offset := range []int{33, 36, 37, 16383} {
			changed := bytes.Clone(raw)
			changed[offset] = 1
			if _, err := DecodeClosedJoinResult(changed, nonce); err == nil {
				t.Fatal("nonempty length or padding accepted")
			}
		}
		if _, err := DecodeClosedJoinResult(raw[:len(raw)-1], nonce); err == nil {
			t.Fatal("short result accepted")
		}
	}
	if _, err := EncodeClosedJoinResult(nonce, 5); err == nil {
		t.Fatal("unsupported result status accepted")
	}
	if _, err := EncodeClosedJoinResult([32]byte{}, 0); err == nil {
		t.Fatal("zero nonce accepted")
	}
}
