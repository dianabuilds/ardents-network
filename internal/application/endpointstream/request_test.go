//go:build linux

package endpointstream

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
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{name: "malformed magic", raw: []byte("AAI2\x02\x00\x01x")},
		{name: "truncated destination", raw: []byte("AAI4\x02\x00\x02x")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeRequest(test.raw); err == nil {
				t.Fatal("noncanonical request was accepted")
			}
		})
	}
}

func TestRequestUsesOneByteStreamGrammar(t *testing.T) {
	for _, test := range []struct {
		wire string
	}{
		{"AAI4\x02\x00\x01x"},
	} {
		want := Request{Destination: TargetLink, Value: "x"}
		raw, err := EncodeRequest(want)
		if err != nil || string(raw) != test.wire {
			t.Fatalf("encode = %q, %v", raw, err)
		}
		got, err := DecodeRequest(raw)
		if err != nil || got != want {
			t.Fatalf("decode = %+v, %v", got, err)
		}
	}
	if _, err := DecodeRequest([]byte("AAI3\x02\x00\x01x")); err == nil {
		t.Fatal("retired request magic was accepted")
	}
}
