package networkstate

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestSourceRequestFrozenBytes(t *testing.T) {
	t.Parallel()
	request := sourceRequest{opcode: sourceByDigest}
	for index := range request.networkDigest {
		request.networkDigest[index] = byte(index)
		request.objectDigest[index] = byte(0x80 + index)
	}
	var encoded bytes.Buffer
	if err := writeSourceRequest(&encoded, request); err != nil {
		t.Fatal(err)
	}
	want := "415244483351310002" +
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f"
	if hex.EncodeToString(encoded.Bytes()) != want {
		t.Fatalf("request bytes=%x", encoded.Bytes())
	}
	decoded, err := readSourceRequest(bytes.NewReader(encoded.Bytes()))
	if err != nil || decoded != request {
		t.Fatalf("decoded request=%+v err=%v", decoded, err)
	}
}

func TestSourceResponseRejectsNonOKObject(t *testing.T) {
	t.Parallel()
	response := sourceResponse{status: sourceNotFound, objectDigest: [32]byte{1}}
	if err := writeSourceResponse(new(bytes.Buffer), response); err == nil {
		t.Fatal("non-OK response carried an object")
	}
	var raw [45]byte
	copy(raw[:8], "ARDH3S1\x00")
	raw[8], raw[9] = sourceNotFound, 1
	if _, err := readSourceResponse(bytes.NewReader(raw[:])); err == nil {
		t.Fatal("non-OK wire response carried an object")
	}
}

func TestByDigestResponseRejectsDifferentObject(t *testing.T) {
	t.Parallel()
	requested, returned := [32]byte{1}, [32]byte{2}
	if err := validateByDigestResponse(requested, returned); err == nil {
		t.Fatal("BY_DIGEST accepted an object other than its exact selector")
	}
}

func TestProtocolStatusesKeepDistinctTerminalOutcomes(t *testing.T) {
	t.Parallel()
	want := [...]byte{sourceOutcomeNotFound, sourceOutcomeBusy, sourceOutcomeBadRequest, sourceOutcomeInternal}
	for status := byte(1); status <= sourceInternal; status++ {
		if got := classifySourceOutcome(sourceStatusErrors[status]); got != want[status-1] {
			t.Fatalf("status %d outcome=%d want=%d", status, got, want[status-1])
		}
	}
}
