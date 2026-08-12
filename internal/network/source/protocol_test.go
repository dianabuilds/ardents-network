package source

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestRequestFrozenBytes(t *testing.T) {
	t.Parallel()
	request := Message{Operation: "by-digest", MaterialIndex: 7}
	for index := range request.NetworkDigest {
		request.NetworkDigest[index] = byte(index)
		request.ObjectDigest[index] = byte(0x80 + index)
	}
	var encoded bytes.Buffer
	if err := writeRequest(&encoded, request); err != nil {
		t.Fatal(err)
	}
	want := "415244483351310002" +
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f" +
		"808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f" +
		"00000007"
	if hex.EncodeToString(encoded.Bytes()) != want {
		t.Fatalf("request bytes=%x", encoded.Bytes())
	}
	decoded, err := readRequest(bytes.NewReader(encoded.Bytes()))
	if err != nil || decoded.Operation != request.Operation || decoded.NetworkDigest != request.NetworkDigest ||
		decoded.ObjectDigest != request.ObjectDigest || decoded.MaterialIndex != request.MaterialIndex ||
		decoded.Status != "" || decoded.Payload != nil {
		t.Fatalf("decoded request=%+v err=%v", decoded, err)
	}
}

func TestResponseRejectsNonOKObject(t *testing.T) {
	t.Parallel()
	response := Message{Status: "not-found", ObjectDigest: [32]byte{1}}
	if err := writeResponse(new(bytes.Buffer), response); err == nil {
		t.Fatal("non-OK response carried an object")
	}
	var raw [45]byte
	copy(raw[:8], "ARDH3S1\x00")
	raw[8], raw[9] = notFoundStatus, 1
	if _, err := readResponse(bytes.NewReader(raw[:])); err == nil {
		t.Fatal("non-OK wire response carried an object")
	}
}
