//go:build linux

package connection

import "testing"

func TestRequestRetainsReservedNameWithoutSelectingIt(t *testing.T) {
	for _, request := range []Request{{Destination: TargetLink, Value: "ardents-target:v1:abc"}, {Destination: Name, Value: "reserved"}} {
		raw, err := EncodeRequest(request)
		if err != nil {
			t.Fatalf("encode %+v: %v", request, err)
		}
		got, err := DecodeRequest(raw)
		if err != nil || got != request {
			t.Fatalf("decode %+v = %+v, %v", request, got, err)
		}
	}
}

func TestRequestRejectsUnknownAndNoncanonicalDestination(t *testing.T) {
	if _, err := EncodeRequest(Request{Destination: 3, Value: "target"}); err == nil {
		t.Fatal("unknown destination was accepted")
	}
	if _, err := DecodeRequest([]byte("AAI3\x02\x00\x02x")); err == nil {
		t.Fatal("truncated request was accepted")
	}
}
